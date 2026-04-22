# Dragz

[English Version](./README.md) | [中文版](./README_zh.md)

A RESTful DDD (Domain-Driven Design) Admin Boilerplate for Go, built with Gin and Zap.

## 🚀 Features

### 🏛️ Architecture Design
- **Domain-Driven Design (DDD)**: Adopts a classic DDD layered architecture, ensuring logic clarity and high scalability.
- **Dependency Injection (Wire)**: Uses Google Wire for compile-time dependency injection, improving code maintainability.
- **RESTful API**: Built with the Gin framework for high-performance RESTful interfaces.

### 🔐 Authentication & Security
- **JWT Authentication**: Supports double Token (Access Token & Refresh Token) mechanism to ensure login security.
- **Custom Claims**: Provides a unified mechanism for storing and retrieving Claims.
- **Auth Middleware**: Flexible middleware support for protecting restricted routes easily.

### 🌐 Internationalization (i18n)
- **Multi-language Support**: Built-in language packs for `en_US`, `zh_CN`, and `zh_TW`.
- **Error Code Mapping**: Unified error handling, automatically returning translated error messages based on client language.
- **Embedded Resources**: Language pack files are embedded in the binary via `go:embed` for easier deployment.

### 🛠️ Infrastructure & Tools
- **Configuration Management (Viper)**: Supports local YAML configurations and reserves capability for configuration centers like Nacos/Etcd.
- **Database Integration (GORM)**:
    - Supports MySQL and PostgreSQL.
    - Supports multiple data source configurations.
    - Supports database operation proxy (via SSH tunnel).
- **Redis Cache**: Built-in Redis support for session management, rate limiting, and more.
- **SSH Tunnel (SSHTunnel)**:
    - **SSH Expose**: Expose local services to a remote server via SSH tunnel.
    - **SSH Forward**: Port forwarding functionality.
    - **Socks5 Proxy**: Supports using a remote machine as a Socks5 proxy for network requests (e.g., payment API calls).
- **QR Code Service**: QR code login/interaction functionality based on WebSocket (Melody), supporting auto-expiration and connection management.
- **Captcha Service**: Reserved Captcha interface, easy to extend with various implementations.

### 📦 Deployment & DevOps
- **Containerization Support**: Provides Dockerfile and docker-compose configurations.
- **Kubernetes Support**: Complete Manifest files (PVC, ConfigMap, Deployment, Ingress, etc.).
- **Database Migration (Atlas)**: Integrated with Atlas tool for versioned database schema management.
- **Swagger Documentation**: Integrated with Swagger UI to automatically generate interactive API documentation.

## 📁 Directory Structure

```text
├── assets/             # Static resources, Swagger docs, default configs
├── atlas/              # Database migration scripts and configs
├── cmd/                # Entry points and Wire injection code
├── internal/           # Core logic (DDD layers)
│   ├── app/            # Application layer (Controller, Router, Middleware)
│   ├── bootstrap/      # Initialization logic (Config, DB, Redis, Tunnel)
│   ├── entity/         # Domain entities
│   ├── i18n/           # I18n language packs and error definitions
│   ├── infra/          # Infrastructure implementations (JWT, QRcode, Translate)
│   ├── repo/           # Data repository layer
│   └── service/        # Domain service layer
├── pkg/                # Public utility packages and core abstractions
└── manifest/           # Kubernetes deployment files
```

## 🛠️ Quick Start

1. **Install Dependencies**:
   ```bash
   go mod download
   ```

2. **Dependency Injection**:
   ```bash
   wire gen ./cmd
   ```

3. **Run Service**:
   ```bash
   go run cmd/dragz/main.go
   ```

## 📜 License
This project is licensed under the [Apache 2.0](LICENSE) License.
