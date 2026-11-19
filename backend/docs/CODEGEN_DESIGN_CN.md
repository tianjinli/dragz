# 代码生成器 gen-crud 设计（MVP）

[English](CODEGEN_DESIGN.md)（待补） | **中文**

本文是 `cmd/gen-crud`（CRUD 代码生成器）的详细设计。定位是 `new-plugin` 的进阶：
`new-plugin` 生成"一个空插件"，`gen-crud` 生成"一个功能完整、开箱即用的 CRUD 插件"。

**最终目的：让各类业务——CRM、商城、工单、进销存……——都以插件形式快速接入**。
gen-crud 是业务插件的流水线：写一份 spec 得到一个完整业务模块，模块之间经
eventbus 协作而互不 import。补齐短信/邮件等基础功能只是它的第一个使用场景。
约定背景见 [PLUGIN_CN.md](PLUGIN_CN.md) 与 [CONVENTIONS_CN.md](CONVENTIONS_CN.md)。

## 1. 目标与非目标

**目标（MVP）**

1. 一份 YAML 资源 spec，一键生成：插件迁移 SQL → store → handler → 路由 →
   菜单/权限声明 → 前端 CRUD 页 → page-map → 中英语言包 → `all.go` 注册。
2. 生成物直接通过全部门禁：`go build/vet/test`、`gofmt`、`check-imports.sh`、
   `check_fail_i18n.sh`、`go run ./cmd/gen-api`、前端 `typecheck/lint/check:i18n/test/build`。
3. 生成的表默认携带 `tenant_id`（SaaS 阶段一的预埋，见 §8）。
4. **生成物天生可协作**：增/删/改/状态变更自动发布 `<plugin>.<resource>.<action>`
   事件（§6.6）——业务插件（CRM、商城）之间不 import，只消费彼此的事件。
5. 生成器自己吃自己的狗粮：M3 起用 gen-crud 批量产出后续基础功能插件（短信模板、
   邮件模板等），并以一个真实业务样板（CRM 客户模块或商城商品模块）验证业务
   接入路径。

**非目标（明确不做）**

- 不做热加载 / 运行时生成：注册永远是编译期的 `plugins/all/all.go`。
- 不做表单设计器 UI：MVP 只有 CLI + spec 文件（芋道式表单设计器留给 M4 之后）。
- 不生成树表、字典关联、文件上传等复杂控件（M1–M2 只覆盖平表 CRUD；
  控件扩展点见 §15）。
- 不改变现有权限模型：权限码仍是 `sys_menu` 按钮行，路由表仍是唯一真源。

## 2. 输入：资源 spec

每个待生成的资源对应一个 YAML 文件，存放于仓库任意位置（推荐
`backend/tools/specs/<plugin>-<resource>.yaml`，方便 review 与回放）。

```yaml
# 生成目标
plugin: demo                      # 插件 ID，^[a-z][a-z0-9]{2,15}$，目录名/前缀
resource: order                   # 资源名，snake_case；API 路径与前端命名由此派生
name_cn: 订单                     # 菜单/按钮中文名
name_en: Order                    # 菜单英文名
menu_parent: ""                   # ParentCode，空 = 顶级
menu_icon: "lucide:package"       # 菜单图标
table: demo_order                 # 表名；为空时派生为 <plugin>_<resource>

fields:
  - column: order_no
    type: string                  # string | int | bigint | bool | timestamptz | text | decimal
    label_cn: 订单号
    label_en: Order No
    required: true
    max_len: 64
    unique: true                  # 影响迁移唯一约束（与 tenant_id 组成复合唯一，见 §8）
    searchable: true              # 进入列表搜索区
    listable: true                # 进入列表列
  - column: amount
    type: decimal
    label_cn: 金额
    label_en: Amount
    required: true
  - column: status
    type: int
    label_cn: 状态
    label_en: Status
    required: true
    status_field: true            # 生成 PUT /:id/status 端点与前端状态开关
    dict: demo_order_status       # 可选：关联字典 code，前端渲染 Tag，表单渲染 Select
  - column: remark
    type: text
    label_cn: 备注
    label_en: Remark
    form_widget: textarea         # input | textarea | select | switch | date
    required: false
    listable: false

soft_delete: true                 # 生成 deleted_at 列 + 软删语义（默认 true）
tenant: true                      # 预埋 tenant_id（默认 true，见 §8）
```

**校验规则**（生成前全部检查，失败即退出、不写任何文件）：

- `plugin` 满足 `plugin.ValidID`；`resource` 匹配 `^[a-z][a-z0-9_]{1,30}$`。
- 至少一个 `required + listable` 字段；`status_field` 至多一个且必须是 `int`。
- `table` 在目标库中不得已存在（仅提示性检查，最终由迁移幂等兜底）。
- 字段 column 匹配 `^[a-z][a-z0-9_]{0,62}$`，不得叫 `id/tenant_id/created_at/
  updated_at/deleted_at`（这些由生成器统一产出）。

## 3. 生成物全景

```
backend/plugins/<plugin>/
  plugin.go            # Meta（含 Menus、DropTables）+ Routes() + Migrations()
  migrations/001_init.sql
  response.go          # 插件本地响应封装（ok/fail，见 §6.3）
  store.go             # 纯 database/sql CRUD（经 ctx.DB，见 §6.4）
  handlers.go          # 六个标准 handler（见 §6.5）
  store_test.go        # 基础单测骨架（成功路径 + 唯一冲突）
frontend/admin/src/pages/<plugin>/index.tsx    # ResourceCrudPage 薄配置页
frontend/admin/src/router/page-map.tsx         # 插入 Component 映射（复用锚点）
frontend/admin/src/router/page-map.test.tsx    # backendComponents 追加
frontend/admin/src/locales/zh.json             # pages.<plugin> + 命名空间字段文案
frontend/admin/src/locales/en.json             # 同 key 集合（check:i18n 强制）
backend/plugins/all/all.go                     # 经 // scaffold:insert 锚点注册
```

生成完成后打印下一步提示：`go build ./... && go run ./cmd/gen-api && 启用插件`。

## 4. 生成的 API 契约（严格对齐 ResourceCrudPage）

前端 `ResourceCrudPage` 按资源根路径调用以下六种请求，生成的路由必须逐字节匹配，
这样前端页只需提供 `columns/fields` 配置、零手写请求层：

| Method | Path | Permission | 说明 |
|---|---|---|---|
| GET    | `/api/<plugin>/<resource>` | `<plugin>:<resource>:query`  | 分页列表，query: `page/page_size/q` + searchable 字段 |
| GET    | `/api/<plugin>/<resource>/:id` | `<plugin>:<resource>:query`  | 详情（`loadDetailOnEdit` 用） |
| POST   | `/api/<plugin>/<resource>` | `<plugin>:<resource>:create` | 新增 |
| PUT    | `/api/<plugin>/<resource>/:id` | `<plugin>:<resource>:update` | 全量更新 |
| PUT    | `/api/<plugin>/<resource>/:id/status` | `<plugin>:<resource>:update` | 状态切换（仅 `status_field` 存在时生成） |
| DELETE | `/api/<plugin>/<resource>/:id` | `<plugin>:<resource>:delete` | 删除（soft_delete 时置 deleted_at） |

统一响应包络 `{"code":0,"message":"ok","data":...}`；列表 data 为
`{"items":[...],"total":n}`；错误经 `fail` 语义返回 locale key（见 §10）。

## 5. 迁移模板（001_init.sql）

生成一个文件、包含全部要素（插件迁移 checksum 只增不改，所以首版必须一次写对）：

```sql
CREATE TABLE demo_order (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL DEFAULT 0,          -- §8：永远生成
  order_no TEXT NOT NULL DEFAULT '',
  amount NUMERIC(18,4) NOT NULL DEFAULT 0,
  status INT NOT NULL DEFAULT 0,
  remark TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ                        -- soft_delete: true 时生成
);

CREATE UNIQUE INDEX demo_order_order_no_key     -- unique 字段 → 复合唯一
  ON demo_order (tenant_id, order_no) WHERE deleted_at IS NULL;
CREATE INDEX demo_order_tenant_created_idx
  ON demo_order (tenant_id, created_at DESC);   -- 列表排序主索引
CREATE INDEX demo_order_q_idx ON demo_order
  USING gin (to_tsvector('simple', order_no))   -- 仅 searchable 的 string 字段
  WHERE deleted_at IS NULL;
```

类型映射：`string→TEXT`、`text→TEXT`、`int→INT`、`bigint→BIGINT`、
`bool→BOOLEAN`、`timestamptz→TIMESTAMPTZ`、`decimal→NUMERIC(18,4)`。
PG 专有语法与 `NORMS_GAP_CN.md` 的单 engine 决策一致，无需方言处理。

`Meta.DropTables` 声明该表——卸载即回到"从未安装"，重装可全量重放。

## 6. 后端模板逐个说明

### 6.1 plugin.go

- `Meta`：ID/Name/Version("0.1.0")/Description/DependsOn(["system"])/Order(200)/
  Builtin(false)。
- `Meta.Menus`：一个页面菜单（Code = `<plugin>_<resource>_page`，Component =
  `/<plugin>/index`）。权限按钮不进 Menus——由路由表启动时自动同步（§9）。
- `Routes()`：按 §4 表格逐条生成，全部带 `Permission` 与 `MenuCode`；
  `status` 路由复用 update 码。更新/删除/新增路由默认走审计中间件（`Audit: nil`）。
- `Migrations()`：与 example 插件相同的 embed 读法。

### 6.2 注册（all.go）

复用 `new-plugin` 已验证的 `patchAllGo` 逻辑：import 块追加 +
`// scaffold:insert` 锚点插入。该函数从 `cmd/new-plugin/main.go` 提取为共享包
`internal/pluginscaffold`（仅依赖标准库 + internal/plugin，符合三层依赖方向），
`new-plugin` 与 `gen-crud` 共用。

### 6.3 response.go（插件本地，约 60 行）

插件不可 import `internal/httpapi`（example 的注释已明确"这是设计使然"）。
生成一份本地封装，语义与 httpapi 的 `ok/fail` 对齐：

- `ok(w, r, data)` → 200 包络。
- `fail(w, r, status, code, key, args...)` → 走请求语言（`Accept-Language`，
  复用 `internal/pkg/i18n` 的公共入口——该包属于 `internal/pkg`，插件可依赖）
  解析 locale key，未知 key 原样输出并记日志。
- 错误码沿用 `internal/pkg/errors` 的 `codes.go` 语义（400/404/409 常量）。

### 6.4 store.go（纯 database/sql，零 sqlc 泄漏）

- 方法集：`List(PageQuery) (items, total, err)`、`Get(id)`、`Create(in)`、
  `Update(id, in)`、`UpdateStatus(id, status)`、`Delete(id)`。
- 全部经 `ctx.DB`（`*store.DB`，database/sql 语义，`BeginTx` 可用）执行，
  SQL 以具名常量写在生成文件内。**不生成 `queries/*.sql`、不触碰 sqlc**：
  插件表是插件私有的，核心 sqlc schema 快照不掺业务插件表，避免
  `schema/tables.sql` 与插件迁移互相牵制。
- `List`：动态 WHERE 拼接只用白名单列名（spec 校验后是字面量集合，无注入面）；
  软删时恒带 `deleted_at IS NULL`；排序固定 `created_at DESC, id DESC`。
- 唯一冲突错误转译为 `errors.CodeConflict`（pg `unique_violation` → 409）。

### 6.5 handlers.go

每个 handler 的骨架与 core `httpapi` 处理器一致：解析参数 → 校验 → 调 store →
写包络。校验规则由 spec 派生（required/max_len/类型），错误消息全部是
`validation.*` 既有 key（`required`、`max_len` 等），**不新增后端文案 key**，
因此 `check_fail_i18n.sh` 无需为新插件扩展。

### 6.6 事件广播（业务插件协作的唯一通道）

架构铁律：插件之间不互相 import，跨插件协作只有 eventbus 一条路。因此
**gen-crud 把事件发布做进模板**，每个生成的业务模块出厂就带标准事件契约，
CRM 订阅商城的 `mall.order.paid`、商城订阅 CRM 的 `crm.customer.created`
都不需要任何基础层改造：

| 动作 | 事件名 | Payload |
|---|---|---|
| 新增 | `<plugin>.<resource>.created` | `{"id": n, "record": {...}}` |
| 更新 | `<plugin>.<resource>.updated` | `{"id": n, "record": {...}}` |
| 状态变更 | `<plugin>.<resource>.status_changed` | `{"id": n, "from": x, "to": y}` |
| 删除 | `<plugin>.<resource>.deleted` | `{"id": n}` |

- 发布点在 handler 成功写库之后（与 example 插件的 `example.pinged` 同一写法，
  `ctx.Bus.Publish`），事件名与 §6.6 表格逐字一致——事件名即契约，消费方按名
  订阅，不存在注册表。
- spec 可加 `events: false` 关闭（纯配置类插件不需要广播）。
- 后续想跨副本投递（商城多实例部署）时，把 in-process Bus 升级为"本地 +
  asynq 转发"对生成物透明——Publish 签名不变，这正是 Bus 抽象的意义。

这条同时是"进阶功能后期不回头改"的关键预埋：会员插件将来订阅
`mall.order.paid` 发积分，支付插件广播 `payment.succeeded`，全部基于本节
契约，无需动任何既有插件。

## 7. 前端生成物

页面即 `ResourceCrudPage` 薄配置（与前端 README 的用法逐字对应）：

```tsx
export function DemoPage() {
  return (
    <ResourceCrudPage
      titleKey="pages.demo.title"
      endpoint="/api/demo/order"       // 与 §4 契约一致
      namespace="demo.order"
      columns={...}                     // 由 listable 字段生成
      fields={...}                      // 由 form_widget/required 派生控件
      createCode="demo:order:create"
      updateCode="demo:order:update"
      deleteCode="demo:order:delete"
      paged
    />
  );
}
```

- `status_field` 存在时加 `cascadeStatus`（走 `PUT /:id/status`）。
- `dict` 字段渲染 `Select`/`Tag`，字典值经 `/api/system/dictionary-values/:code` 拉取。
- 语言包：`pages.<plugin>.{title,intro}` + `<plugin>.<resource>.<field>label` 等
  字段文案，zh/en 同步生成，key 集合一致（check:i18n 门禁）。
- page-map / page-map.test.tsx 插入逻辑与 `new-plugin.scaffoldFrontend` 相同，
  一并提取进 `internal/pluginscaffold` 供两端复用。

## 8. tenant_id 预埋

沿用已拍板的"SaaS 多租户：共享库 + 行级隔离，阶段一显式 tenant_id"路线：

1. 生成器**永远**输出 `tenant_id BIGINT NOT NULL DEFAULT 0` 列。
2. 唯一约束一律做成 `(tenant_id, <unique_col>)` 复合唯一；普通索引把
   `tenant_id` 放在最左。
3. store 的 WHERE 恒拼 `tenant_id = $n`，MVP 阶段实参恒为 0——语义上就是
   "所有数据都属于默认租户"，行为与不带该列完全一致，但表结构与查询已经
   租户化，阶段二只需把租户值接进上下文（JWT → ctx → store 实参），
   **对生成物零改造**。
4. spec 里 `tenant: false` 仅用于确知永不参与多租户的场景（如纯静态表），
   默认 true。

## 9. 权限码与菜单

- 权限码 = `<plugin>:<resource>:<action>`，通过 `Route.Permission` 声明；
  启动时由 menuperm 自动同步为 `sys_menu` 按钮行，挂在
  `MenuCode = <plugin>_<resource>_page` 下，无需生成任何菜单 SQL。
- 按钮中文名由 `MenuName` 显式给出（查询/新增/编辑/删除/改状态五个标准词），
  英文名走既有翻译管理机制。
- 生成的插件出厂即暗（`Builtin: false`）：编译进来 → 管理员在插件中心启用 →
  菜单/按钮/路由三件套同时出现；停用即 404。

## 10. i18n

- 后端：handlers 返回的错误一律是既有 `validation.*` / `errors.*` key；
  生成的 `response.go` 负责 key → 文案解析。MVP 不给插件引入新 locale 包。
- 前端：静态文案进 `zh.json`/`en.json` 的 `pages.*` 与插件命名空间，两份
  key 集合由 `check:i18n` 强制一致；生成器同时写入两份，天然对齐。

## 11. 实现结构

```
backend/cmd/gen-crud/
  main.go            # CLI：-spec <file> [-dry-run] [-dir <repo root>]
  spec.go            # YAML 解析 + 全量校验
  model.go           # spec → 内部模型（列类型映射、派生名、SQL 片段）
  gen_backend.go     # plugin.go / migrations / response.go / store.go / handlers.go
  gen_frontend.go    # 页面 / page-map / locales（调 internal/pluginscaffold）
  gen_register.go    # all.go 注册
  templates/         # go:embed 的 text/template，一文件一模板
  golden_test.go     # 固定 spec → 快照比对（防模板回归）
backend/internal/pluginscaffold/
  allgo.go           # patchAllGo（自 new-plugin 提取）
  frontend.go        # 前端脚手架（自 new-plugin 提取）
```

- 模板渲染后统一 `gofmt`（Go 文件）与 `JSON 缩进重排`（locale 文件）再落盘。
- `-dry-run` 只打印将要创建/修改的文件清单，不写盘。
- 幂等性：目标文件已存在时默认报错退出（生成器不覆盖手改过的代码）；
  `-force` 仅对 `migrations/` 以外的文件允许覆盖并提示 diff。

## 12. 质量门禁与测试

| 层 | 保障 |
|---|---|
| 生成器自身 | `golden_test.go`：3 个固定 spec（最小/含状态/含字典）快照比对 |
| 生成物编译 | CI 冒烟：跑一次 gen-crud → `go build ./...` → 前端 `pnpm build` |
| 依赖方向 | 生成物只 import `internal/{app,plugin,pkg/*}` 与 `platform/*`，`check-imports.sh` 全绿 |
| 路由真源 | 生成后跑 `go run ./cmd/gen-api` 同步 openapi.json（生成器打印提醒） |
| i18n | zh/en key 对齐由 `check_fail_i18n.sh`（后端）与 `check:i18n`（前端）双门禁 |
| 运行时 | `store_test.go` 骨架覆盖 CRUD 成功路径与唯一冲突；插件启用后手测六端点 |

## 13. 里程碑

| 阶段 | 内容 | 验收 |
|---|---|---|
| M1 | spec 解析 + 后端五件套生成（含 §6.6 事件广播；手写前端先跑通） | 生成的插件启用后六端点全通，事件可在下游订阅 |
| M2 | 前端页面/page-map/locales 生成 + `internal/pluginscaffold` 提取 | 生成后零手改过全部门禁 |
| M3 | dogfood：用 gen-crud 生成短信模板管理、邮件模板管理两个基础插件 | 两个插件进主干，生成器无 bug 修复 patch |
| M4 | **业务样板**：以 CRM 客户模块（客户/联系人/跟进记录，含关联下拉与主子表雏形）或商城商品模块（商品/分类树/图片字段）为验收样板，反推 spec 扩展字段 | 业务样板手改量 < 20 行，spec 扩展点落定 |
| M5 | 增强：`--from-db` 反向内省、树表/主子表/导入导出/文件上传控件 | 按需排期，由 M4 暴露的真实需求驱动 |

## 14. dogfood 验收（M3/M4 的意义）

M3（短信/邮件模板管理）验证生成器对"基础功能"的覆盖：如果还需要大量手改，
说明模板抽象不对。验收标准：生成物手改量 < 20 行/插件（通常只有 Description
文案），且这两个插件的迭代不再触碰生成器核心。

M4（CRM/商城业务样板）验证生成器对"真实业务"的覆盖——这才是它的最终目的。
业务域一定会暴露平表 CRUD 盖不住的形态（关联、主子表、树、状态机流转），
样板的作用就是把它们收集成 spec 扩展字段清单，M5 逐个模板化，而不是提前
凭空设计。**接入一个新业务的成本预期 = 写 1–3 份 spec + 挂菜单启用插件。**

## 15. 风险与开放问题

1. **模板漂移**：ResourceCrudPage props 与路由契约若演进，生成物会过期。
   缓解：§4 契约表作为"双源契约"写进本文档与前端 README，改动必须同步；
   golden 测试在 PR 里暴露任何一侧的漂移。
2. **插件本地 store vs sqlc**：插件表绕开了 sqlc 体系，长期可能形成两套数据
   访问风格。若后续插件数量增长，可在 sqlc.yaml 增加 per-plugin `gen` 条目
   生成到 `plugins/<id>/store/sqlc`（sqlc 支持多 entry），本设计预留了这条路，
   MVP 先不引入。
3. **openapi 同步**：gen-crud 不自动跑 `gen-api`，避免生成器嵌套调用 Go 工具；
   打印提醒 + CI 校验 openapi.json 是否最新。
4. **卸载语义**：`DropTables` 只含 spec 里的主表；M5 的关联表（如多表资源）
   需要在 spec 显式声明 `extra_tables`，否则卸载留孤儿表（沿用 PLUGIN 文档
   自检清单的口径）。
5. **事件 payload 版本化**：业务插件长期演进，`record` 结构会变，订阅方可能
   跨版本存活。MVP 约定 payload 只增不改字段（新字段可选），破坏性变更改
   事件名加版本后缀（`mall.order.paid.v2`）。是否需要 schema 注册表，等出现
   3 个以上真实消费方再议。
6. **业务插件的依赖面**：CRM/商城必然用到文件上传（商品图、附件）与字典，
   这些在 system/core 侧但受 FeatureKey 控制。生成的业务插件不声明对
   feature 的依赖——路由按需加 `FeatureKey`，启用插件后管理员在功能开关里
   自行打开，缺依赖时表现为该端点 403/禁用而非整插件失效。
7. **主子表事务边界**（M5）：订单/订单明细这类生成物，事务写在 store 层
   `BeginTx`，事件只在最外层提交成功后发一次——模板设计时固定这个语义，
   不留给使用者选择。
