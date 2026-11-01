# OpenAPI generation

**English** | [中文](README_CN.md)

The API contract lives in `api/desc/all.api`. Generate documentation with go-zero tooling:

```bash
go install github.com/zeromicro/go-zero/tools/goctl@latest
goctl api swagger -api api/desc/all.api -dir api/openapi
```

The runtime routes remain registered in `internal/httpapi/routes.go`; the `.api` file is the contract source of truth for AI and frontend collaboration.
