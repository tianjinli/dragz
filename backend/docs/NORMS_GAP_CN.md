# 后端规范差距与约定

[English](NORMS_GAP.md) | **中文**

本文记录已拍板约定与整改边界。组件清单见 [COMPONENTS_CN.md](COMPONENTS_CN.md)，分层约定见 [CONVENTIONS_CN.md](CONVENTIONS_CN.md)。

## 已拍板约定

1. **数据访问**：使用 **sqlc**，不引入 ent。当前 **单 engine：`postgresql`**。
2. **i18n**：用户可见文案一律走 locale key（`errors.*` / `common.*`），禁止在 `fail`/`failData`/`HTTPError.Message` 中写中英文字面量。
3. **多库扩展**：本轮不实现第二数据库；但 logic/httpapi **不得**直接依赖 `internal/store/sqlc` 生成代码，须通过仓储 interface，便于日后增加 MySQL 等实现。

## 多库扩展 Playbook（Backlog）

换库 = 换 store 实现 + DDL/queries，不换业务层。

1. 新增 `migrations/<driver>/` DDL。
2. `sqlc.yaml` 增加对应 `engine`，生成到 `internal/store/<driver>/sqlc`（或等价路径）。
3. 实现同一套 `internal/store` 仓储 interface。
4. `Database.Driver` + `store.Open(driver, dsn)` 注册驱动。
5. CI 增加该库 smoke（登录、用户 CRUD）。

### 具体的阻塞点（2026-10-30 盘点，动手前必须先解决）

这些不是"换个驱动名"就能过的，按工作量从大到小：

| 阻塞点 | 位置 | 说明 |
|--------|------|------|
| 占位符方言 | `queries/*.sql` | 全部是 `$1` 形式。MySQL 要 `?`，等于每个查询文件都要一份方言版本，sqlc 不会自动转换 |
| 迁移引擎的 SQL | `plugins/*/migrations/*.sql`、`migrations/001_init.sql` | `BIGSERIAL`、`TIMESTAMPTZ`、`JSONB`、`ON CONFLICT` 都是 PG 专有 |
| 迁移串行化 | `store.AcquireMigrationLock` | 用 `pg_advisory_lock`。MySQL 要换 `GET_LOCK()`，语义（超时、连接绑定、随连接释放）不完全等价 |
| 可空 JSON 列 | `store/sqlc_helpers.go` 的 `nullRawArray`、`pqtype.NullRawMessage` | 绑定 `database/sql` 的 pq 类型，MySQL 侧要用 `sql.NullString` |
| 菜单环检测 | `menu_parent_is_descendant`（schema 函数 + 递归 CTE） | sqlc 分析器不支持递归 CTE，MySQL 8 的 CTE 写法也不同，需要在应用层重写或换实现 |
| 大小写与排序规则 | 全部查询 | PG 标识符折叠为小写、默认区分大小写排序；MySQL 在 Windows 上默认不区分，唯一索引行为会变 |

结论：这是一次"平行实现 + 双份 SQL"的工程，不是配置项。在真正需要之前保持单 engine，
并用 `Database.Driver` 的白名单在启动时挡住误配置（`config.Validate` 与 `store.Open` 各挡一次）。

## 本轮已做 / 进行中

| 项 | 状态 |
|----|------|
| 清理旧 `job-worker` / `jobworker`；统一为 `cmd/jobs-runner` + `internal/jobs/runner` | 已完成 |
| Docker Compose 仅挂载 `001_init.sql`（含 `token_version`）；既有 volume 若缺列需一次性 ALTER 或重建 | 已完成 |
| `token_version`（JWT 失效）、`GET /api/ready`、密钥校验 / `admin-api.example.yaml` | 已完成 |
| `Database.Driver`（默认 postgres） | 已完成 |
| i18n：httpapi `fail`/`failData`/`HTTPError.Message` 已全部改 key；zh/en locale 对齐；auth/lockout/feature + 其余 handler | 已完成 |
| sqlc 单 engine、schema 对齐、仓储 interface | **已完成** |
| 全量 fail 文案微调、MySQL 实现 | Backlog |

### sqlc 迁移进度

- **Schema**：`schema/tables.sql` 自 `migrations/001_init.sql` DDL 同步（含 `token_version`、`menu_parent_is_descendant`）。
- **已迁 sqlc**：users / refresh_tokens / auth_rbac / jobs / settings / oauth / dictionaries / roles·menus·menu_buttons / organizations·positions / notices / files / webhooks / audit（含列表筛选） / data_scope。可选筛选统一用 `sqlc.narg`（见 `ListRolesFiltered` 模式）。
- **菜单环检测**：sqlc 分析器不支持递归 CTE，故用 schema 函数 `menu_parent_is_descendant`（仅在 `001_init.sql` / `schema/tables.sql`）；查询 `MenuParentIsDescendant`。
- **边界**：httpapi **不** import `internal/store/sqlc`；`repos.go` 扩展 `AuthStore` / `UserStore` / `JobStore` / `SettingsStore`。业务 SQL 仅在 `queries/`。

## 密钥

- 仓库内 `etc/*.yaml` 仅用于本地开发；生产必须通过环境/挂载注入。
- 示例见 `etc/admin-api.example.yaml`。
- 启动时若检测到已知默认密钥且未显式允许开发模式，将拒绝启动（见 `config.Validate`）。
