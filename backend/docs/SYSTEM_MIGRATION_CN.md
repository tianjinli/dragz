# system 插件与 internal/httpapi 迁移清单

[English](SYSTEM_MIGRATION.md) | **中文**

`plugins/system` 目前通过 `httpapi.Routes()` 暴露核心路由，属于**过渡态**（文档称 P6）。目标：核心业务也以插件契约声明，handlers 迁出 `internal/httpapi` 的「上帝包」形态。

## 当前边界

| 区域 | 现状 | 目标 |
|------|------|------|
| 路由表 | `plugins/system` → `httpapi.Routes()` | 逐步把路由声明留在 system（或拆子包），handlers 按域分文件 |
| 中间件 | `httpapi.Hooks` | 可留在 httpapi 或迁到 platform；kernel 只认 `plugin.Hooks` |
| 业务编排 | 部分已在 `httpapi/logic` | handlers 不直接调 store；新代码走 logic |
| 豁免 | 仅 `plugins/system` 可 import `internal/httpapi` | 豁免在迁移完成后删除 |

## 建议搬迁顺序（不阻塞 P0–P1）

1. 鉴权 / 会话相关 handlers  
2. 用户 / 角色 / 部门  
3. 菜单 / 权限同步周边  
4. 通知 / 文件 / 任务等可选能力  
5. 删除 system → httpapi 的 import 豁免，并更新 `check-imports.sh`

## 停用闸门（已实现，勿回退）

- 顺序：`Localize → Gate → Authenticate → …`  
- 插件停用后业务路由必须 **404**（与路径不存在相同），**禁止**先认证再 401。  
- 实现：`internal/httpapi/plugin.go` 的 `pluginGate`；测试见 `plugin_test.go` / `kernel/router_test.go`。

## 与本阶段关系

P0–P1 **不要求**完成上述搬迁；本文件只固定边界与验收口径，避免二次开发继续往 httpapi 堆无归属代码。
