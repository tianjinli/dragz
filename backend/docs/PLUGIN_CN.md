# 插件开发指南

[English](PLUGIN.md) | **中文**

后端的业务功能以**编译期插件**的形式接入。插件只负责「声明自己贡献了什么」，装配、排序、开关、迁移全部由内核和平台层完成。这份文档说明怎么写一个插件，以及写完必须自检什么。

相关文档：[CONVENTIONS_CN.md](CONVENTIONS_CN.md)（分层与授权约定）、[../../README_CN.md](../../README_CN.md)（快速开始）。

## 1. 什么时候做成插件

做成插件，而不是往 `internal/` 里塞：

- 会**整块上下线**的功能（示例插件、某个业务模块）。
- 拥有**自己的表**和**自己的菜单页面**。
- 需要「出厂即暗、管理员打开才生效」的交付形态。

不要做成插件：跨插件共享的基础设施（那是 `platform/`）、框架契约本身（那是 `kernel/`）、以及需要被所有插件直接 import 的工具（放 `internal/pkg/`）。

## 2. 目录结构

```
plugins/<id>/
  plugin.go              Meta() + Routes()，可选 Migrations()
  migrations/
    001_init.sql         迁移文件，只增不改
  <其他实现文件>.go
```

`<id>` 必须匹配 `^[a-z][a-z0-9]{2,15}$`（`plugin.ValidID`）。它同时是目录名、表名前缀、配置命名空间和 `sys_menu.plugin_id` 的值。

## 3. 契约

最小契约只有两个方法：

```go
type Plugin interface {
    Meta() Meta
    Routes() []Route
}
```

需要建表时再实现可选能力 `Migrator`（`Migrations() []Migration`）——能力接口用类型断言收集，所以插件只选择自己需要的部分。

```go
type Plugin struct{}

func (Plugin) Meta() plugin.Meta {
    return plugin.Meta{
        ID: "example", Name: "示例插件", Version: "0.1.0",
        DependsOn: []string{"system"},   // 依赖的插件必须先初始化
        Order:     10,                   // 同层排序的 tiebreaker，无语义
        DropTables: []string{"sys_example_item"}, // 卸载时 DROP 的表
        Builtin:   false,                // true = 不可停用
    }
}
```

`DropTables` 只在插件**卸载**时使用（见第 7 节）。迁移是只增不改的，无法从中反推出建了哪些表，
所以建表插件必须自己声明；不声明就卸载，表会留在库里变成孤儿。名字必须是纯标识符
（`^[a-z][a-z0-9_]{0,62}$`），带 schema、带引号、大写的都会被拒绝，一条 DDL 都不会执行。

`Route` 的字段里，除了 `Method/Path/Handler` 与 go-zero 一一对应，其余都交给内核中间件：

| 字段 | 作用 |
|------|------|
| `Public` | 免认证（登录、健康检查、验证码）。仍然会走本地化和限流 |
| `Permission` | 单个权限码；留空表示「登录即可」 |
| `AnyOf` | 命中任一权限码即放行 |
| `FeatureKey` | 再套一层功能开关（在权限之后判断） |
| `Audit` | `nil` = 按默认规则记录写操作；`false` = 跳过审计 |
| `AuditPermission` | 覆盖审计行里记录的权限码 |
| `MenuCode` | 权限按钮挂载的页面码；留空则由权限码推导 |
| `MenuName` | 覆盖自动生成的中文按钮名 |

## 4. 注册

只有一个地方：`plugins/all/all.go`。

```go
func All() []plugin.Plugin {
    return []plugin.Plugin{
        system.Plugin{},
        example.Plugin{},
        // scaffold:insert
    }
}
```

加一个 import + 一个 slice 元素即可，`// scaffold:insert` 是脚手架自动追加的位置。**没有热加载**：启用/停用改变的是行为，不是二进制。

## 5. 迁移

- 位置：`plugins/<id>/migrations/NNN_name.sql`，用 `go:embed` 读进来。`NNN` 恰好三位，保证字典序等于数字序。
- **一个文件 = 一个事务**，和它的 `sys_plugin_migration` 记录一起提交。失败只回滚这一个文件，下次启用从它继续。
- **只增不改**：已应用文件的 SHA-256 会被记录并在每次运行时比对，改动会让启用失败（`ErrChecksumMismatch`）。修就用新文件向前修。
- 一个文件可以有多条语句（无绑定参数执行，pgx 走简单协议），但**不能出现 `$1` 这类占位符**。
- 所有 schema 变更由会话级 `pg_advisory_lock` 串行化；`admin-api` 是唯一的迁移执行者，`jobs-runner` 必须在它启动完成之后再起。

## 6. 菜单与权限

插件自己的菜单写在迁移里，注意三点：

1. **ID 用 1000+**，避开核心保留段（1–99、82–86）。
2. **必须显式写 `plugin_id`**，这是停用插件时菜单跟着消失的依据。
3. **不要 seed 权限码**。权限码由启动同步从 `Route.Permission` / `AnyOf` 生成，挂到 `MenuCode` 指定的页面上：

```sql
INSERT INTO sys_menu
  (id, parent_id, type, name, code, route_name, route_path, component,
   sort, status, visible, plugin_id)
VALUES
  (1000, 0, 'page', '示例插件', 'example_page', 'ExamplePage',
   '/example', '/example/index', 900, 1, 1, 'example')
ON CONFLICT DO NOTHING;
```

对应路由：

```go
{
    Method: http.MethodGet, Path: "/api/example/ping",
    Permission: "example:ping",
    MenuCode:   "example_page",   // 推导不出来时必须显式写
    MenuName:   "连通性检查",
    Handler:    pingHandler,
}
```

推导规则：把权限码最后一段当动作丢掉，剩下的部分连字符转下划线，从长到短取第一个真实存在的非 button 菜单。`example:ping` 的候选只有 `example`，而页面码是 `example_page`，所以要显式声明。

**核心插件的权限码解析不到页面会让启动失败**（这是编程错误）；**普通插件的会跳过并打日志**，等它的迁移把页面建出来，启用时再补。

## 7. 启用与停用

| 动作 | 发生什么 |
|------|----------|
| 编译进来 | 插进 `sys_plugin`，`enabled = is_builtin`，即**默认停用** |
| 启用 | 跑待执行迁移 → 同步权限按钮 → 翻 `enabled = true` → 重载路由闸门与 Casbin |
| 停用 | 只翻开关。不跑 DDL、不删数据、不退役按钮 |

停用的效果是**三层同时消失**：路由返回 404（与路径不存在逐字节相同）、菜单在 SQL 层被裁掉、权限码从 `/api/auth/codes` 和 Casbin 里消失。重新启用即时无损。

`system` 是 builtin，三处强制启用：catalog 同步、内存快照、`UPDATE ... WHERE is_builtin = FALSE`。所以 `/api/health` 和 `/api/ready` 永远不会 404。

**多副本**：每个副本各有一份内存快照。`admin-api` 启动后会跑一个 5 秒的 watcher
（`plugincenter.Watch`），定时重读 `sys_plugin`，所以 A 副本上的开关最迟一个周期后在 B 副本生效。
不做 Redis 订阅是因为 go-zero 的 redis 客户端没有 subscriber，而一张只有十几行的表的轮询
比多一条连接加重连/丢消息处理更划算；代价是那一个周期内 B 仍按旧状态放行。

## 7.1 卸载

卸载是**破坏性**操作，和停用严格分开：停用只是关灯，卸载是把灯拆了。

| 前置条件 | 说明 |
|----------|------|
| 非 builtin | `system` / `plugincenter` 不可卸载 |
| 已停用 | 未停用直接返回 409 `errors.plugin.enabled_cannot_uninstall` |
| 无已启用的依赖方 | 否则 409 `errors.plugin.dependency_in_use` |

满足后，以下动作在**一个事务**里完成，任何一步失败都整体回滚：

1. `DROP TABLE IF EXISTS` 掉 `Meta.DropTables` 里声明的表（不加 CASCADE，被别的插件外键引用就报错）
2. 删 `sys_plugin_migration` 里该插件的迁移记录
3. 删 `sys_role_menu` → `sys_menu` 里 `plugin_id = <id>` 的行（页面和按钮一起）
4. 删 `sys_plugin` 的目录行

插件仍然编译在二进制里，所以下次启动的 catalog 同步会把它作为**停用**重新插回来；
再启用就是从 `001` 重新跑一遍迁移，等于全新安装。这就是「卸载」的完整语义，不需要额外的
"已卸载" 状态位。

`DropTables` 漏声明不会报错，只会留下孤儿表——所以自检清单里有这一项。

## 8. 响应约定

插件**不能** import `internal/httpapi`（那是核心应用的内部实现，唯一豁免是 `plugins/system`，P6 之后会删掉）。自己写响应体，保持信封一致：

```go
func pingHandler(*app.Context) http.HandlerFunc {
    return func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"pong":true}}`))
    }
}
```

需要数据库、缓存、翻译器时，从 `*app.Context` 上取，不要自己建连接。

## 9. 自检清单

新增插件提交前：

- [ ] `plugins/all/all.go` 加了一行，`go build ./...` 通过
- [ ] `bash scripts/check-imports.sh` 通过（插件之间没有互相 import）
- [ ] 迁移文件名是 `NNN_name.sql`，没有 `$1`
- [ ] 菜单 ID ≥ 1000，`plugin_id` 写对了
- [ ] 建了表就把表名写进 `Meta.DropTables`
- [ ] 没有手工 seed 权限码
- [ ] `go run ./cmd/gen-api` 之后 `api/openapi/openapi.json` 有对应路径
- [ ] 路由基线 `internal/kernel/testdata/routes.golden` 更新了（含首行计数）
- [ ] 全新库启用一次、停用一次、再启用一次，菜单与权限码都符合预期
- [ ] 再卸载一次，确认表、菜单、迁移记录都没了，重新启用能回到初始状态

卸载走的是真实的 `DROP TABLE`，单元测试只覆盖到标识符校验。要跑真实 DDL，起一个一次性库：

```bash
docker run -d --rm --name dragz-verify-pg \
  -e POSTGRES_USER=verify -e POSTGRES_PASSWORD=verify -e POSTGRES_DB=verify \
  -p 25433:5432 postgres:16-alpine
DRAGZ_VERIFY_DSN='postgres://verify:verify@127.0.0.1:25433/verify?sslmode=disable' \
  go test ./internal/store -run TestUninstallPlugin -v
docker stop dragz-verify-pg
```

没设 `DRAGZ_VERIFY_DSN` 时这些用例自动 skip，所以 `go test ./...` 仍然是纯离线的。

## 10. 常见错误

| 现象 | 原因 |
|------|------|
| 启动报 `permission "x:y" has no owning menu` | 核心路由的权限码推导不到页面，补 `Route.MenuCode` |
| 启用失败 `ErrChecksumMismatch` | 改了一个已经应用过的迁移文件。加新文件，或手工恢复原文件 |
| 启用成功但按钮没出现 | 迁移里的页面 `plugin_id` 写错，或页面还没建出来就同步了 |
| 停用后接口返回 401 而不是 404 | 路由闸门被放到了认证内层。`Gate` 必须在 `Authenticate` 外层 |
| 菜单出现了但侧边栏不显示 | 菜单 `status != 1`、`visible = 0`，或父菜单被停用 |
| 插件按钮被重启"删掉"了 | 手工建的按钮 `managed = 0` 不会被退役；同步建的才会。检查是不是码被同步接管了 |
| 卸载后表还在 | `Meta.DropTables` 没声明，或名字不合法（大写/带 schema/带引号会被拒绝并整体回滚） |
| 卸载报 409 `enabled_cannot_uninstall` | 先停用再卸载；两步分开是为了不让"停服务"和"扔数据"变成一次点击 |
