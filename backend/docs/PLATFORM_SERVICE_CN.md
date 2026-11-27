# 平台统一服务架构设计（v1）

> 第一版决策：服务配置归平台统一管理，租户不可配服务商、不可填 API Key，所有租户共享平台服务；用量按租户统计、额度按套餐限制、成本平台统一核算。BYOS（租户自带服务）仅在接口层预留，不做实现。

## 一、总体架构

```
租户业务代码（插件 / 核心模块）
        │  只依赖 Scene，不依赖供应商
        ▼
ServiceHub（internal/servicehub）—— 唯一收口
  1. Resolve(tenantID, scene)        场景 → 平台配置的供应商实例（按 priority 主备）
  2. QuotaCheck(tenantID, scene)     读 sys_usage_counter，超限返回专用错误
  3. RateLimit(tenantID, scene)      进程内令牌桶，防单租户打爆共享 Key
  4. Driver.Do(...)                  调用具体驱动
  5. Meter(...)                      写 sys_usage_record + upsert sys_usage_counter
        ▼
驱动实现（复用现有包）
  OSS    → internal/filestore.Provider（现有接口不变）
  SMS    → hub 内 smsDriver（阿里云/腾讯云实现）
  Mail   → hub 内 mailDriver（SMTP 实现）
  AI     → hub 内 aiDriver（OpenAI 兼容协议）
  Pay    → hub 内 payDriver（仅平台收款场景）
```

关键约束：

- **业务代码只写 Scene 字符串常量**（如 `oss.attachment`、`sms.verification_code`），永远不感知供应商。
- **Resolver 签名从第一天带 `tenantID`**。v1 忽略租户差异只查平台配置；将来开 BYOS 时在 Resolver 内加"租户覆盖 → 平台兜底"一步，调用方零改动。
- **计量、配额、限流全部发生在 Hub 内部**，业务代码不埋点，否则 BYOS 扩展时计量必漏。
- 平台配置变更后**热生效**：Hub 持有带版本号的缓存，管理端保存时递增版本触发重载，无需重启。

## 二、数据模型（migration `032_platform_service.sql`）

### 2.1 平台表（无 tenant_id，不走租户 RLS）

```sql
-- 服务商驱动目录（s3 / aliyun-sms / smtp / openai / wechat-pay ...）
CREATE TABLE IF NOT EXISTS sys_service_provider (
  id BIGSERIAL,
  code VARCHAR(64) NOT NULL,
  driver_type VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id),
  UNIQUE (code)
);

-- 场景实例：一个 scene 可有多行（priority 排序 = 主备切换顺序）
CREATE TABLE IF NOT EXISTS sys_platform_service (
  id BIGSERIAL,
  scene VARCHAR(128) NOT NULL,          -- oss.attachment / sms.verification_code / mail.transactional / ai.chat / pay.platform
  provider_id BIGINT NOT NULL REFERENCES sys_service_provider (id),
  config JSONB NOT NULL DEFAULT '{}',   -- 非敏感配置（endpoint、bucket、region、from 等）
  secret_enc TEXT NOT NULL DEFAULT '',  -- SecretBox 加密（enc:v1: 前缀），AccessKey/SecretKey/AppKey
  priority INT NOT NULL DEFAULT 100,    -- 数字小 = 主用；主用探活失败切下一行
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  last_health_at TIMESTAMPTZ NULL,
  last_health_ok BOOLEAN NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id),
  UNIQUE (scene, provider_id)
);
CREATE INDEX IF NOT EXISTS idx_platform_service_scene
  ON sys_platform_service (scene, priority) WHERE enabled;
```

平台表不启用租户 RLS（无 tenant_id）。访问边界靠两层保证：租户端 HTTP API 不暴露这两张表的任何路由；SQL 层这些表只出现在 `dragz_migrate` / `dragz_platform` 与应用角色的平台侧查询里。

### 2.2 用量表（带 tenant_id，走标准 RLS）

```sql
CREATE TABLE IF NOT EXISTS sys_usage_record (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  scene VARCHAR(128) NOT NULL,
  provider_id BIGINT NOT NULL,
  quantity BIGINT NOT NULL DEFAULT 1,   -- 调用次数 / 字节数 / Token 数，unit 区分
  unit VARCHAR(16) NOT NULL DEFAULT 'call',
  cost NUMERIC(12,6) NOT NULL DEFAULT 0,-- 平台侧成本（供应商单价 × 用量），供成本分析
  ref_type VARCHAR(64) NOT NULL DEFAULT '',
  ref_id VARCHAR(64) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_usage_record_tenant
  ON sys_usage_record (tenant_id, scene, created_at DESC);
-- + trg_fill_tenant_id / ENABLE & FORCE RLS / tenant_isolation policy（照抄 013 模式）

-- 日聚合，jobs-runner 定时任务从 sys_usage_record 汇总，兼做 counter 对账基准
CREATE TABLE IF NOT EXISTS sys_usage_daily (
  tenant_id BIGINT NOT NULL,
  day DATE NOT NULL,
  scene VARCHAR(128) NOT NULL,
  quantity BIGINT NOT NULL DEFAULT 0,
  cost NUMERIC(12,6) NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, day, scene)
);
```

### 2.3 复用既有表，不新建

| 既有表 | 用途 |
| --- | --- |
| `sys_plan_entitlement` | 套餐额度定义，`entitlement_key` 约定为 `service.<scene>.<period>`，如 `service.oss.attachment.monthly_bytes` |
| `sys_usage_counter` | 配额检查的实时计数：`(tenant_id, quota_key=service.<scene>, period_key=202611)`，Hub 内 `INSERT ... ON CONFLICT DO UPDATE SET used_value = used_value + $n RETURNING used_value` 一步完成累加+回读 |
| `sys_tenant_subscription` | 判断租户当前套餐，取 entitlement 额度 |

注意：`sys_usage_record` / `sys_usage_daily` 若在 `bootstrapStatements` 里同步建索引，裸写 `ON CONFLICT (col...)` 必须与唯一索引一致，避免启动 42P10（沿用既有教训）。

## 三、Go 层设计（`internal/servicehub`）

```go
type Scene string

const (
    SceneOSSAttachment  Scene = "oss.attachment"
    SceneSMSVerifyCode  Scene = "sms.verification_code"
    SceneMailTrans      Scene = "mail.transactional"
    SceneAIChat         Scene = "ai.chat"
    ScenePayPlatform    Scene = "pay.platform"
)

// 驱动契约：每类服务一个最小接口，OSS 直接复用 filestore.Provider
type SMSDriver interface {
    Send(ctx context.Context, to, templateID string, params map[string]string) error
}
type MailDriver interface {
    Send(ctx context.Context, m Mail) error
}
type AIDriver interface {
    Chat(ctx context.Context, req AIRequest) (AIResponse, error)
}
type PayDriver interface { // v1 仅平台收款
    CreateOrder(ctx context.Context, o PayOrder) (PayResult, error)
}

type Hub struct { /* resolver 缓存 + limiter + meter */ }

// 主入口：v1 tenantID 仅用于计量与配额，不影响解析结果
func (h *Hub) SMS(tenantID int64) SMSDriver
func (h *Hub) Mail(tenantID int64) MailDriver
func (h *Hub) AI(tenantID int64) AIDriver
func (h *Hub) OSS(tenantID int64) filestore.Provider // 包装现有 Provider，套上计量/配额
```

调用链示例（短信验证码）：

```go
err := h.SMS(tenantID).Send(ctx, phone, tplLogin, map[string]string{"code": code})
// Hub 内部：resolve(主备) → 限流 → 驱动发送 → 计量写库 → 返回
```

- **配额检查**：`period_key` 按场景周期（次数类用月 `202611`，字节类用月，AI Token 用月）。累加后 `used_value > limit` 则回滚本次计数并返回 `ErrQuotaExceeded`，HTTP 层映射为 `429 + code: SERVICE_QUOTA_EXCEEDED`，调用方据此渲染升级提示。
- **限流**：进程内令牌桶 `map[tenantID+scene]*ratelimit.Bucket`，v1 只兜底（如短信 5 QPS/租户），不做分布式限流。
- **主备切换**：Resolve 按 priority 取第一行；驱动 `Test()`/调用失败且 `last_health_ok=false` 时标记，下次 Resolve 跳过并告警（复用 018 的 outbox 通知机制即可）。
- **密钥**：`secret_enc` 走现有 `internal/security.SecretBox`，master key 来自启动配置；管理端 API 永不下发明文。

## 四、API 与菜单

**平台管理端**（新增路由组 `/api/platform/services`，仅平台管理员）：

- 服务商目录 CRUD + 场景实例 CRUD（config 表单按 driver_type 渲染）
- 「测试连接」按钮 → 调驱动 `Test()`，写 `last_health_*`
- 用量/成本查询（读 `sys_usage_daily`）

**租户端**：

- 不出现任何「服务配置」菜单

菜单 seed 写进 032 迁移（平台侧"平台管理 → 服务配置"二级菜单：存储/短信/邮件/AI/支付/认证，与租户端无新增）。

## 五、支付边界（v1 明确不做的事）

- `pay.platform`（租户买套餐）→ 平台商户号，正常实现。
- **租户业务收款（`pay.tenant_business`）v1 不开放**：平台账户代租户收终端用户资金属二清红线，不做"平台统一代收"的变体。表结构已通过 `scene` 天然隔离，将来合规路径是服务商/特约商户模式（平台做服务商，租户进件自己的商户号），届时再加 scene 与进件流程，本设计无需变更。

## 六、实施阶段

| 阶段 | 内容 | 验收 |
| --- | --- | --- |
| P1 | migration 032 + `internal/servicehub` 骨架 + OSS 场景迁移（filestore 配置源从 sys_config 切到 sys_platform_service，sys_config 旧键保留只读兼容一个版本） | 上传/下载走 Hub，计量落库，管理端可配 S3/local |
| P2 | 短信/邮件驱动 + 配额检查 + 限流 + 平台端 CRUD/测试 UI + `SERVICE_QUOTA_EXCEEDED` 错误码 | 超额租户收到 429，调用方显示提示 |
| P3 | jobs-runner 日聚合任务（sys_usage_daily + counter 对账修正）+ 成本分析页 + AI/支付场景接入 | 成本分析可按租户/场景出报表 |

预留但不实现：`sys_tenant_service`（租户覆盖表）+ Resolver 内先查租户后查平台的两段逻辑——只留接口注释标明扩展点。

## 七、风险清单

1. 共享 Key 被单租户打爆 → Hub 限流 + 供应商侧配额告警双保险。
2. 平台表无 RLS，靠"租户 API 不暴露路由"约束 → 新增路由时不得在租户鉴权组里注册 servicehub 管理接口。
3. 计量写库在请求路径上 → v1 同步 upsert 单行可接受；量级上来后改异步（缓冲 channel / outbox），Hub 内部切换，调用方无感。
4. 迁移期间 filestore 双配置源 → 兼容读一个版本后在 033 清理，避免长期双轨。
