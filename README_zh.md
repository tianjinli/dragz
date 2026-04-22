# Dragz

[English Version](./README.md) | [中文版](./README_zh.md)

一个基于 Gin 和 Zap 构建的 Go 语言 RESTful 领域驱动设计（DDD）管理模板。

## 🚀 功能特性

### 🏛️ 架构设计
- **领域驱动设计 (DDD)**: 采用经典的 DDD 分层架构，逻辑清晰、易于扩展。
- **依赖注入 (Wire)**: 使用 Google Wire 进行编译时依赖注入，提高代码可维护性。
- **RESTful API**: 基于 Gin 框架构建的高性能 RESTful 接口。

### 🔐 认证与安全
- **JWT 身份验证**: 支持双 Token（Access Token & Refresh Token）机制，保障登录安全。
- **自定义 Claims**: 提供统一的 Claims 存储 and 获取机制。
- **认证中间件**: 灵活的中间件支持，轻松保护受限路由。

### 🌐 国际化 (i18n)
- **多语言支持**: 内置 `en_US`, `zh_CN`, `zh_TW` 语言包。
- **错误代码映射**: 统一的错误处理，支持根据客户端语言自动返回翻译后的错误信息。
- **嵌入式资源**: 语言包文件通过 `go:embed` 嵌入到二进制文件中，部署更便捷。

### 🛠️ 基础设施与工具
- **配置管理 (Viper)**: 支持本地 YAML 文件配置，并预留了 Nacos/Etcd 等配置中心接入能力。
- **数据库集成 (GORM)**:
    - 支持 MySQL 和 PostgreSQL。
    - 支持多数据源配置。
    - 支持数据库操作代理（通过 SSH 隧道）。
- **Redis 缓存**: 内置 Redis 支持，适用于会话管理、限流等场景。
- **SSH 隧道 (SSHTunnel)**:
    - **SSH Expose**: 将本地服务通过 SSH 隧道暴露到远程服务器。
    - **SSH Forward**: 端口转发功能。
    - **Socks5 Proxy**: 支持通过远程机器作为 Socks5 代理进行网络请求（如支付接口调用）。
- **二维码服务**: 基于 WebSocket (Melody) 的扫码登录/交互功能，支持自动过期和连接管理。
- **验证码服务**: 预留 Captcha 接口，易于扩展多种验证码实现。

### 📦 部署与运维
- **容器化支持**: 提供 Dockerfile 和 docker-compose 配置。
- **Kubernetes 支持**: 完整的 Manifests 文件（PVC, ConfigMap, Deployment, Ingress 等）。
- **数据库迁移 (Atlas)**: 集成 Atlas 工具进行版本化数据库架构管理。
- **Swagger 文档**: 集成 Swagger UI，自动生成 API 交互文档。

## 📁 目录结构

```text
├── assets/             # 静态资源、Swagger 文档、默认配置
├── atlas/              # 数据库迁移脚本与配置
├── cmd/                # 程序入口与 Wire 注入代码
├── internal/           # 核心逻辑 (DDD 分层)
│   ├── app/            # 应用层 (Controller, Router, Middleware)
│   ├── bootstrap/      # 启动初始化逻辑 (Config, DB, Redis, Tunnel)
│   ├── entity/         # 领域实体
│   ├── i18n/           # 国际化语言包与错误定义
│   ├── infra/          # 基础设施实现 (JWT, QRcode, Translate)
│   ├── repo/           # 数据仓储层
│   └── service/        # 领域服务层
├── pkg/                # 公共工具包与核心抽象
└── manifest/           # Kubernetes 部署文件
```

## 🛠️ 快速开始

1. **安装依赖**:
   ```bash
   go mod download
   ```

2. **依赖注入**:
   ```bash
   wire gen ./cmd
   ```

3. **运行服务**:
   ```bash
   go run cmd/dragz/main.go
   ```

## 📜 开源协议
本项目采用 [Apache 2.0](LICENSE) 协议。
