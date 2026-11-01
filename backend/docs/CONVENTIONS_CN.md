# 后端约定（对齐 Simple Admin）

[English](CONVENTIONS.md) | **中文**

## 分层

- `internal/httpapi` —— 薄处理器（解析请求、写响应）
- `internal/httpapi/logic` —— 业务编排；已迁移模块的处理器不得直接调用 `store`
- `internal/store` —— 通过仓储接口做数据访问；PostgreSQL 实现可以使用 `internal/store/sqlc`
- `internal/pkg` —— 共享的错误、i18n、ctx key、casbin 包装

**铁律：** `logic` 与 `httpapi` 不得 import `internal/store/sqlc`（生成代码必须待在仓储接口后面）。

## 文案与 i18n

**铁律：Go 代码里不写用户可见的字面量。** 每个标签、消息和错误都走 message ID，
并且两份语言文件都要有它。

- store 返回 `internal/store/errors.go` 里的哨兵错误
- HTTP 映射用 `internal/pkg/errors/mapper.go`，返回的是 **message key**，不是散文
- `fail` / `failData` 只接受 i18n key；本地化发生在 `localized()` 里
- 语言文件：`internal/pkg/i18n/locale/{zh,en}.json`；**每个 key 必须在两边都存在**。
  `scripts/check_fail_i18n.sh` 校验传给 `fail`/`failData` 的 `errors.*`
  key、传给 `i18n.StoredMessage` 的 ID 以及 `login_log.reason.*` 的值在 `zh`
  和 `en` 中都存在，所以拼错也不可能把裸 key 发给客户端。
- 优先用 `errors.*` key；带参数的文案用 go-i18n 的 `TemplateData`（例如 `{{.Remaining}}`）
- 放进响应体的东西一律用 `localized(r, ...)`；它要求请求上带着 translator
  （凡经 `kernel.Register` 注册的路由都有）。

### 写入数据库的文案

生命周期长于请求的行（登录日志的失败原因、插件最近一次错误）也不能存渲染好的中文/英文：

- 存 message ID：`"login_log.reason.invalid_credentials"`。
- 带模板数据时存 `i18n.StoredMessage(id, data)`；它会在文本列里写入
  `i18n:{"id":...,"data":{...}}`。
- 读取时用 `localizedStored(r, value)` 渲染。旧版本写下的纯文本原样返回，
  所以这次切换不需要数据迁移。
- 日志、`fmt.Errorf` 包装和其他运维诊断信息保持英文（ASCII），永不本地化 ——
  它们是给开发者看的，不是给用户看的。

### 三层文本：界面文案 / 预设数据 / 管理员内容

同一个句子可能来自三个不同的地方，改文案前先判断它属于哪一层：

1. **界面文案** —— 随二进制发布的 chrome（按钮、表单标签、提示）。走 message key，
   两份语言文件 `locale/{zh,en}.json` 都写，`check_fail_i18n.sh` 校验。
   **不进翻译管理**：翻译管理页没有、也不该有界面文案的入口，改这类文字就是改代码、
   发版本。
2. **预设数据** —— 迁移或插件种子创建的菜单、权限按钮、角色、部门、定时任务、存储
   配置。中文是**默认语言**，直接存在各自的文本列里；英文写在 `sys_translation`
   （`(entity_key, 'en')` 唯一），由「系统设置 → 翻译管理」维护，`entity_key` 用
   `menu.<code>` / `perm.<code>` / `role.<code>[.remark]` / `dept.<code>` /
   `job.<code>` / `storage.<type>`。填中文原文后，英文列由免费机器翻译自动生成
   （`POST /api/system/translations/translate`，provider 取自 `Translate.Providers`，
   无需 API Key），生成结果仍可手改；新增时英文为空会由服务端补译，所以管理员
   只需要写中文。两列还各有一个**翻译**按钮，按需把另一边重新翻一遍并覆盖已有
   内容——自动那一遍只填空的目标列，不会盖掉已经写好的译文。该服务尽力而为：
   连不上时手写英文即可，功能不受影响。
   清空英文列会删掉对应译文行；删除接口没有删到东西时返回 **404**，
   因为列表列的是预设实体，某一行可能比它的译文活得久，静默返回成功
   会让管理员继续看到一段已经不存在的文字。
   读取时 `localizePreset` 按 `i18n.Lang(r)` 覆盖，
   缺译回退中文，中文请求零查询。
3. **管理员内容** —— 字典类型与条目、配置项名称/描述、公告正文、用户数据：管理员
   随手新增，没有稳定的实体编码，也不该被语言切换改名，原文就是最终值。

判断：**随二进制发布 → key；种子写进库的业务名 → 原文 + 译文表；管理员随时新增 →
纯原文。**

`internal/menuperm` 生成的按钮文案（动作模板 + 名词表）属于第 2 层：同步时把英文
模板写进 `sys_translation`（`ON CONFLICT DO NOTHING`，不覆盖管理员已改的译文）。

### 语言范围：中文 + 英文

产品当前只交付中文和英文两套界面，所以可翻译语言也只有英语一门：

- `internal/pkg/i18n/languages.go` 的 `targetLanguages` 是**唯一真源**（今天只有
  `"en"`），`Accept-Language` 解析与翻译管理的写入校验都读它；
- 前端语言列表 `frontend/admin/src/constants/locales.ts` 必须与它一致
  （`zh-CN` / `en-US`），主语言标签就是发出去的 `Accept-Language`；
- 未注册的语言请求按中文返回，写入 `lang` 非英语时 400。

真要加第三门语言：注册表加一项 → `locales.ts` 加一项 → 翻译页补该语言的录入入口；
界面文案包要另做，`locale/*.json` 与前端 `locales/*.json` 各补一份，否则该语种界面
按中文显示。**数据可翻译与界面可翻译是两件事**，别用翻译管理去填界面文案。

前端文案约定见
[`frontend/admin/README_CN.md`](../../frontend/admin/README_CN.md#国际化i18n)。

## 数据访问（sqlc）

- 引擎：目前**只支持 postgresql**（`sqlc.yaml`）
- **业务 SQL 只写在 `queries/*.sql` 里** —— 跑 `sqlc generate`；不要在
  `internal/store/*.go` 里内嵌 `SELECT`/`INSERT`/`UPDATE`/`WITH` 字符串字面量
  （生成的 `sqlc/` 除外）
- 表结构源要与 `migrations/` 保持一致（见 `schema/`）
- Docker Compose 首次初始化 Postgres 时挂载 `001_system_init.sql`（系统基线），
  `token_version` 和 `menu_parent_is_descendant` 都在 `001_system_init` 里。
  已存在的命名卷不会重跑初始化 —— 早于这些对象的库需要手工执行一次
  `ALTER` / `CREATE FUNCTION`（或者重建卷）；权威 DDL 见 `schema/tables.sql`。
  `migrations/` 只保留参与建库的脚本（001 / 009）；退役的业务脚本已从代码树移除，
  它们当年 seed 的行由 `internal/store/plugin.go` 里的 `retiredModuleStatements`
  在启动时删除。
- 新增另一种数据库：见 [NORMS_GAP_CN.md](NORMS_GAP_CN.md) 的 playbook —— 不要重写业务逻辑

### 软删除与业务键

- 删除一律是软删除：业务表都有 `deleted_at`，所有读取都带 `deleted_at IS NULL`。
- 所以业务键（`code`、`username` 等）**只在存活行之间唯一**：用
  `CREATE UNIQUE INDEX uk_x ON t (key) WHERE deleted_at IS NULL`，**不要**用
  `UNIQUE` 表约束。普通约束会让已删除行继续占着键，「删掉再建一条一样的」就会
  报唯一键冲突。Postgres 没法把这种部分唯一写成表约束，所以这些索引和其它
  `CREATE INDEX` 放在一起；存量库由 `internal/store/plugin.go` 的
  `bootstrapStatements` 先 DROP 旧约束再建索引。
- 对 SQL 的影响：这些表上的 `ON CONFLICT (key)` 不再能推断出索引，要么写
  `ON CONFLICT (key) WHERE deleted_at IS NULL`，要么「先复活再插入」
  （见 `queries/menu_buttons.sql`）。
- `sys_file` 保留普通 `UNIQUE (storage_id, object_key)`：它的删除是带恢复的回收站，
  恢复回来的文件不能和期间被复用的 key 撞车。
- 种子数据是**显式 id** 插入的（`migrations/001_system_init.sql`），序列因此停在
  初始值；`seedStatements` 每次启动把所有自增序列拉到 `MAX(id)` 之上，否则 UI 建的
  第一条就会撞种子行（`sys_x_pkey`）。

## 鉴权

- Casbin 策略：`(roleId, permissionCode, access)`
- 启动时重新加载；角色授权变更后经 `app.Context.AfterRoleGrantsChanged` 重载；
  插件**启用和停用**时都要重载（停用插件的码必须立刻离开 enforcer）。
- 权限缓存在 Redis 里，按用户 id 作 key
- 访问 JWT 携带 `token_version`；强制下线 / 改密会踢 DB 版本号，使已签发的访问令牌失效

## 权限按钮（接口权限）

每个权限码都是 `sys_menu` 里 `type = 'button'` 的一行。没有独立的权限表，也没有第二棵授权树。

- **一码一按钮。** `sys_menu.code` 存权限串（`system:user:create`）。
  `sys_menu.permission` 已删除：它从来没被填过，也从来没被读过。页面码
  （`system_user`）和权限码不可能撞车，因为冒号是 `validateMenu` 里的严格判别符：
  按钮码必须含冒号，其他类型必须不含。
- **路由表是唯一真源。** `plugin.Route.Permission` / `AnyOf` 声明权限码；
  启动同步（`store.SyncPermissionMenus`，由 `internal/menuperm` 驱动）在所有插件
  迁移跑完后，为每个码 upsert 一个按钮。所以新增一条路由，权限就可以被授予，不需要改 seed。
- **归属与可编辑。** `parent_id`、`method`、`path`、`is_sensitive`、`plugin_id`、
  `status` 和 `code` 归同步所有。管理员只能改 `name` 和 `sort`；
  `store.lockManagedMenuFields` 会覆盖其余字段，`UpdateMenuStatus` / `DeleteMenu`
  直接拒绝这些行。
- **`sys_menu.managed` 是归属标记。** 路由表声明的每个码，同步都写 `1`；
  管理后台写入 `0`（列默认值）。只有 `managed = 1` 的行会被锁定，也只有它们会被退役，
  这正是管理员手工建一个权限按钮、下次启动不会被删掉的原因。手工按钮的码后来出现在
  路由表里时，会被同步接管（`managed` 翻成 1），因为码归路由表所有。手工按钮就是普通行：
  编辑、停用、删除都可用，其 type 被钉死为 `button`，所以永远不会变成页面。
  它的码是前端或外部服务可以校验的东西；它本身不授予任何接口，因为守卫读的是路由声明的码。
- **父级解析是推导出来的。** `system:dict:item:create` 沿
  `system_dict_item` → `system_dict` 往上走，取存在为真实菜单的最长候选，
  所以只有例外才需要 `Route.MenuCode`。*核心*路由解析不到会启动失败；
  插件路由会跳过，直到它的迁移建出页面（启用时会重跑同步）。
- **授权跟着码走。** `/api/auth/codes` 和 Casbin 策略查询都读
  `sys_role_menu JOIN sys_menu WHERE type = 'button'`，并按
  `(plugin_id = '' OR 插件已启用)` 过滤，和菜单树完全一致。停用插件的码随页面一起消失。
- **退役是软删除。** 同步所有的按钮，路由消失时会被软删除并停用，但它的
  `sys_role_menu` 行保留：加回路由就能恢复授权。upsert 会同时重置 `deleted_at`
  和 `status`，因为 `ListUserPermissionCodes` 过滤 `status = 1` ——
  不重置状态的话，授权回来时挂的是一个已死的按钮。
- **历史升级。** `store.migrateLegacyAPIPermissions` 把 `sys_api_permission`
  拷成按钮、把 `sys_role_api_permission` 拷进 `sys_role_menu`，然后在一个事务里
  drop 掉三张旧表。它由 `to_regclass('public.sys_api_permission')` 守卫，
  所以只跑一次，之后是 no-op。属于已删除模块的码也会被拷过去，随后被
  `Seed` 的退役模块清理再删掉，清理紧跟在 `Bootstrap` 之后。
- **前端门控刻意做得粗。** 菜单/公告/钩子/接口用单个 `system:xxx:manage` 码
  覆盖新增、修改和删除，因为路由表就是这么声明的。不要在 UI 里发明
  `:create` / `:delete` 字面量；它们永远不会被授予。

## 接口契约

- 手写 `api/desc/all.api`；不需要 goctl 代码生成
- 入口：`api.RegisterRoutes`
- 存活：`GET /api/health`；就绪：`GET /api/ready`（DB + Redis）

## 定时任务（asynq）

- **配置**：PostgreSQL 里的 `sys_scheduled_job` 仍是管理后台的唯一真源
- **执行**：`cmd/jobs-runner` 跑 asynq Server + Scheduler（Redis 队列）
- **API 进程**：`admin-api` 只经 asynq Client 入队手动执行和 reconcile 请求；
  没有 15 秒一次的 DB 轮询
- **日志**：`sys_job_log` 由 jobs-runner 在每次尝试后写入
- **同步**：jobs-runner 的 Reconciler 每 60 秒加载一次已启用的任务，
  CRUD 之后触发的 `scheduler:reconcile` 任务也会触发
- **特性开关**：`feature.scheduler.enabled` 关闭注册与执行

## 配置

- `Database.Driver` 默认 `postgres`（目前唯一支持的驱动）
- 生产环境密钥不得使用开发默认值；见 `config.Validate` 和 `etc/admin-api.example.yaml`

## 插件（运行时启停）

分层：`plugins/ → platform/ → kernel/`。依赖只能单向；
`scripts/check-imports.sh` 强制执行。

- **注册是编译期的。** `plugins/all/all.go` 是唯一的开关。没有热加载：
  启用插件改变的是行为，不是二进制。
- **import 边界。** `internal/pluginstate` 是中立的边界包（只依赖标准库），
  它之所以存在，是因为 `internal/plugin` import 了 `internal/app`；
  `app` 和 `store` 永远不得 import `internal/plugin`。
- **出厂即暗。** 新编译进来的非内置插件以 `enabled = is_builtin` 插入，
  所以它默认停用、迁移不跑，直到管理员在 系统设置 → 插件中心 里启用。
- **内置插件不能停用。** `system` 在插件目录同步、内存快照以及
  `UPDATE ... WHERE is_builtin = FALSE` 守卫里都被强制为启用。
  因此 `/api/health` 和 `/api/ready` 永远不会 404。
- **停用意味着 404，不是 403/503。** 路由闸门位于认证**外层**，所以匿名调用者拿到的响应
  与路径不存在时完全一样。`rest.WithNotFoundHandler` 对未匹配路径渲染同一个信封。
- **菜单在服务端裁剪。** `sys_menu.plugin_id`（`''` = 核心）在两个递归菜单查询里
  与 `sys_plugin.enabled` join，所以停用插件会带走整棵子树，也没有孤儿被提升成根。
  两个查询还都过滤 `type <> 'button'`，所以权限按钮永远不会进入侧边栏。
- **停用和卸载是两种操作。** `Disable` 只翻开关：不跑 DDL、不丢数据、即时可回。
  `Uninstall` 是破坏性的，只对已停用的插件开放；它在一个事务里 drop 掉
  `Meta.DropTables`、插件的 `sys_menu` 行（页面和按钮）、它的 `sys_plugin_migration`
  行和 `sys_plugin` 行。插件仍编译在二进制里，所以下次启动会以停用状态重新插入，
  启用时重放迁移 `001`。表名是被插值进 `DROP TABLE` 的，所以任何 DDL 执行前
  都要用 `^[a-z][a-z0-9_]{0,62}$` 校验 —— schema 限定、引号和大小写都会被拒绝。
- **每个副本都有自己的快照。** `Enable`/`Disable` 只重载处理请求的那个副本；
  `admin-api` 还跑着 `plugincenter.Watch`，每 5 秒重读一次 `sys_plugin`，
  让其他副本不重启也能收敛。最坏情况下陈旧一个周期。轮询是刻意的选择：
  go-zero 的 redis 客户端没有提供 subscriber，而每插件一行的查询比多一条连接
  加重连/丢消息处理更划算。

### 启动 DDL 与迁移

- 插件表（`sys_plugin`、`sys_plugin_migration`）和 `sys_menu` 的
  `plugin_id`、`method`、`path`、`is_sensitive` 列，由 `store.PluginRepository.Bootstrap`
  在每次启动时执行的**运行时 bootstrap DDL** 创建。这是刻意的：Postgres 镜像只在
  空 PGDATA 上跑 `migrations/*.sql`，所以新增一个迁移文件会让每个已存在的库
  都要手工 `psql` 一次。
- 其余一切归 `queries/*.sql`（业务 SQL）或插件自己的 `migrations/NNN_*.sql`。
- **移除一个模块**意味着三件事：删它的代码、删它的 seed 文件、
  把它的行加进 `internal/store/plugin.go` 的 `retiredModuleStatements`。
  最后一步才是清理已经初始化过的库 —— 删 seed 文件只影响空卷。

### 插件迁移

- 位置：`plugins/<id>/migrations/NNN_name.sql`，用 `go:embed` 内嵌。
  `NNN` 恰好三位数字，所以字典序即数字序。
- **只增不改。** 每个已应用文件的 SHA-256 会被记录；改动已应用的文件会让下次启用
  直接失败关闭（`ErrChecksumMismatch`）。修复方式是加一个新文件。
  破窗手段是还原原文件或手工删掉记账行。
- **一个文件 = 一个事务**，连同它的 `sys_plugin_migration` 行。
  失败只回滚该文件，所以下次启用从这个文件续上。
- 一个文件可以含**多条语句**（不带绑定参数执行，这让 pgx 走 simple protocol），
  但不得含 `$1` 这类占位符。
- 插件自己的菜单用 ID **1000+**，并且必须显式设置 `plugin_id`。
- 所有 schema 变更都由会话级 `pg_advisory_lock` 串行化
  （bootstrap、seed、目录同步、启动迁移、启用、停用）。
- **`admin-api` 是唯一的迁移执行者。** `jobs-runner` 必须在它完成启动之后再启动。

### 停用

停用只翻开关：从不跑 DDL，从不删行。表和迁移记账都留着，
所以重新启用是即时且无损的。卸载（drop）和 down 迁移不在范围内。

### 已知限制：多副本

启用状态快照是每进程一份。当 `admin-api` 有多个副本时，在 A 副本上的开关
会让 B 副本继续用陈旧的闸门判定，直到重启（菜单读的是数据库，所以是一致的，
这让不一致变得可见）。目前假设单副本；升级路径是 Redis pub/sub 失效通道，
或者由闸门读取一个短 TTL 的 Redis key。
