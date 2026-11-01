// Package platform hosts platform capabilities shared by plugins: auth,
// rbac, file, audit, job, dict, eventbus. Platform packages are themselves
// implemented as built-in plugins (added in later phases).
//
// 铁律（scripts/check-imports.sh 强制）：
//   - platform 不得 import plugins（kernel 同样不得）；
//   - plugins 之间禁止互相 import，跨插件通信走 platform 接口或事件总线。
//
// P0 阶段此包尚无实现；openapi / eventbus / feature 等在 P3 落地。
package platform
