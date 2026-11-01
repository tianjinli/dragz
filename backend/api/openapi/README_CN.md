# OpenAPI 生成

[English](README.md) | **中文**

接口契约在 `api/desc/all.api`。用 go-zero 工具链生成文档：

```bash
go install github.com/zeromicro/go-zero/tools/goctl@latest
goctl api swagger -api api/desc/all.api -dir api/openapi
```

运行期路由仍注册在 `internal/httpapi/routes.go`；`.api` 文件是 AI 与前端协作时的契约真源。
