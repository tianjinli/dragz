# 部署与运维脚本

[English](README.md) | **中文**

一键操作入口全部在 `deploy/`。所有脚本都是 bash，Windows 下用 Git Bash 执行，Linux/macOS 通用。

## 本地开发

| 目标 | 一条命令 |
|------|----------|
| 全栈启动（前端构建 + postgres/redis/api/worker/nginx） | `bash deploy/up.sh` |
| 只起后端和依赖（跳过前端构建） | `bash deploy/up.sh --api-only` |
| 只起 postgres + redis，后端用 `go run` 跑 | `bash deploy/dev-infra.sh` |
| 看状态 + 健康探针 | `bash deploy/ps.sh` |
| 跟随日志（可指定服务） | `bash deploy/logs.sh [admin-api]` |
| 停止（保留数据卷） | `bash deploy/down.sh` |
| 停止并清空数据库后重建 | `bash deploy/reset.sh` |

启动完成后：

- 管理后台 <http://127.0.0.1:5279>
- API 健康检查 <http://127.0.0.1:5280/api/health>
- 默认账号 `admin` / `Dragz!7xZ`（由 `migrations/001_system_init.sql` 种入，首次登录需改密）

`up.sh` 是幂等的：重复执行只会重建有变化的镜像层。

### 端口被占用

如果 5432 / 6379 / 5280 / 5279 已被别的服务占用（本机就长期被其他 Postgres/Redis 容器占用），用 override 文件改端口即可，不必改仓库里的 compose：

```bash
cp deploy/ports.override.example.yml deploy/ports.override.yml
DRAGZ_EXTRA_COMPOSE="$PWD/deploy/ports.override.yml" bash deploy/up.sh
```

`ports.override.example.yml` 里用的是 `!override`，它会**替换**基础文件的 `ports` 列表；普通合并是追加，冲突的端口映射仍然存在，起不来。

`up.sh` / `ps.sh` 会用 `docker compose port` 读回真实映射端口，健康探针和最终输出的 URL 会自动跟着变。

> 注意：容器之间仍走 compose 内部网络（`postgres:5432`、`redis:6379`），override 只影响宿主机端口。

## 生产（k3s）

生产链路保持不变，入口在 `deploy/k3s/`：

| 目标 | 命令 |
|------|------|
| 构建并推送镜像 | `pwsh deploy/k3s/build-and-push.ps1 -Tag vX.Y.Z` |
| 上传部署文件并滚动发布 | `bash deploy/k3s/deploy-manual.sh [key.pem]` |
| 宿主机 postgres/redis | `docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod-infra.yml up -d` |

生产密钥填在 `deploy/.env.prod`（gitignored），字段说明见 `deploy/.env.prod.example`。详细步骤见 [`k3s/README.md`](k3s/README_CN.md)。

## 约定

- 脚本共用 `lib.sh`，它只被 `source`，不单独执行。
- 所有 compose 调用都带 `--project-directory deploy`，因此**在哪个目录执行脚本都一样**。
- `curl` 一律加 `--noproxy '*'`：Windows 上系统代理会拦截 `127.0.0.1`，导致健康检查误报。
- 破坏性操作（`down.sh --volumes`、`reset.sh`）必须手动输入 `yes` 确认。

## 三个刻意的设计决定

1. **compose 固定 `name: dragz`**
   Compose 默认用目录名当项目名，`deploy/` 这个目录名很多项目都在用。`docker compose down` 是按项目标签删容器的，项目名撞车会顺手删掉别的项目的容器（本机就发生过一次）。显式命名后互不干扰。
2. **不使用 `--remove-orphans`**
   同理，孤儿清理按项目范围执行，跨项目会误删。代价是改过服务名后旧容器会残留，需要手动 `docker rm`。
3. **Postgres 初始化 SQL 打进镜像**（`deploy/Dockerfile.postgres`）
   原来是 bind mount `backend/migrations/*.sql`。Docker Desktop 的 File Sharing 不包含仓库路径时挂载直接失败（本机 `C:\Users\snail` 都没共享），一键启动就断了。打进镜像后不依赖任何宿主机路径配置，首次建库行为与挂载完全一致。

前端产物 `frontend/admin/dist` 在构建前会被脚本先清空：它是纯构建产物，而让打包器自己清目录在部分受管环境下会触发文件系统保护。
