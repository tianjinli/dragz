# Dragz 部署到 k3s（腾讯云，中国大陆）

[English](README.md) | **中文**

`diy.cszqq.top` 的单节点 k3s 部署，主机 `111.230.232.233`（Ubuntu 24.04，
2 vCPU / 4 GB）。PostgreSQL 和 Redis 用 Docker Compose 跑在宿主机上、位于 k3s 之外，
这样重建集群不会碰到数据。

```
diy.cszqq.top (Traefik :80/:443)
├── /api/*    -> svc/admin-api:5280        (admin-api, go-zero)
└── /*        -> svc/admin-web:80          (nginx + 构建好的 SPA)
jobs-runner   -> asynq worker（不暴露 Service）

宿主机 docker compose: postgres:16-alpine + redis:7.4-alpine
  只绑定 10.1.0.9（内网 IP），Pod 通过 external-db.yaml.tpl 里的
  无 selector 的 Service + Endpoints 访问
```

## 文件

| 路径 | 用途 |
| --- | --- |
| `config/{admin-api,jobs-runner}.yaml` | go-zero 配置，以 ConfigMap 挂载 |
| `external-db.yaml.tpl` | 指向节点内网 IP 的 Service + Endpoints |
| `admin-api.yaml` / `jobs-runner.yaml` / `admin-web.yaml` | 工作负载 |
| `ingress.yaml` | 按路径各一个 Ingress（`/api` -> admin-api，`/` -> admin-web） |
| `tls.yaml` | ClusterIssuer（Let's Encrypt DNS-01）+ Certificate |
| `registries.yaml` | containerd 镜像加速（Docker Hub / quay / ghcr） |
| `daemon.json` | 宿主机 Docker Hub 加速，用于拉 Postgres/Redis |
| `scripts/bootstrap-server.sh` | 安装 docker、k3s、helm、cert-manager、镜像加速 |
| `scripts/deploy-app.sh` | 应用全部清单、创建 Secret、拉起 compose |
| `build-and-push.ps1` | 本地构建并推送到 `hub.cszqq.top/dragz/*` |
| `deploy.ps1` | 端到端：构建/推送、上传、初始化、部署 |

## 首次部署

1. 填写 `deploy/.env.prod`（已 gitignore）。必需项见
   `deploy/.env.prod.example`。`CONFIG_MASTER_KEY` 在 admin-api 和
   jobs-runner 里必须一致。
2. 确认 `diy.cszqq.top` 的 A 记录指向 `111.230.232.233`。
3. 在仓库根目录：

   ```powershell
   powershell -File deploy/k3s/deploy.ps1
   ```

   服务器只接受 `ubuntu` 用户 + 指定私钥登录（`sudo` 免密）；`deploy.ps1`
   已经处理好这一点。

## 国内网络注意事项（重要）

服务器无法直连 GitHub 和 Docker Hub。bootstrap 脚本这样绕过：

- **kernel/k3s 二进制**：用 `INSTALL_K3S_MIRROR=cn` 安装
  （`rancher-mirror.rancher.cn`）。
- **Helm**：从 `get.helm.sh` 下载（可访问）；`raw.githubusercontent.com`
  上的 `get-helm-3` 脚本不可访问。
- **containerd 镜像**：`/etc/rancher/k3s/registries.yaml` 把
  `docker.io` 镜像到 `mirror.ccs.tencentyun.com`、`quay.io` 到
  `quay.m.daocloud.io`、`ghcr.io` 到 `ghcr.m.daocloud.io`。
- **宿主机 Docker**：`/etc/docker/daemon.json` 加速 Docker Hub，
  让 Postgres/Redis 的 compose 栈能拉镜像。
- **移除 snap 版 docker**：它受 AppArmor 限制，无法 bind-mount
  `/data/dragz`。用 apt 的 `docker.io` + `docker-compose-v2`。

## 镜像

首选：推送到 `hub.cszqq.top/dragz/*`，k3s 用 `hub-registry` pull secret 拉取。

离线兜底（仓库不可用时）：

```bash
docker save -o dragz-images.tar <4 个镜像>
scp dragz-images.tar ubuntu@111.230.232.233:/tmp/
ssh ubuntu@111.230.232.233 "sudo k3s ctr -n k8s.io images import /tmp/dragz-images.tar"
```

`imagePullPolicy: IfNotPresent` 会直接用导入的镜像，不访问仓库。

## TLS

`tls.yaml` 创建一个 `letsencrypt-prod` ClusterIssuer 和一个 `dragz-tls`
Certificate，走**阿里云 DNS 的 DNS-01**，因为这里 HTTP-01 不可用：
海外解析器看到的仍是解析到 `43.174.224.0/24`（阿里云新加坡）的陈旧记录，
那边只会返回 DNSPod 的拦截页。

前置条件（一次性）：

```bash
# AliDNS webhook（阿里云镜像，大陆可访问）
helm repo add alidns https://wjiec.github.io/alidns-webhook
helm upgrade --install alidns-webhook alidns/alidns-webhook \
  -n cert-manager \
  --set groupName=acme.cszqq.top \
  --set image.repository=registry.cn-hangzhou.aliyuncs.com/wjiec/alidns-webhook \
  --set image.tag=latest

# 凭据（对该 zone 有 DNS 权限的 RAM 用户）；
# deploy/.env.prod 里设置了 ALIDNS_ACCESS_KEY_ID 和
# ALIDNS_ACCESS_KEY_SECRET 时，deploy-app.sh 会自动创建
kubectl -n cert-manager create secret generic alidns-secret \
  --from-literal=access-key-id='<id>' \
  --from-literal=access-key-secret='<secret>'
```

查看与强制重试：

```bash
sudo k3s kubectl -n dragz-prod get certificate
sudo k3s kubectl -n dragz-prod describe certificate dragz-tls
```

如果某次尝试失败是因为 DNS/凭据还不对，删掉终态资源和 secret，让订单重新开始
（签发时 cert-manager 会重建 secret）：

```bash
sudo k3s kubectl -n dragz-prod delete certificaterequest,order,challenge --all
sudo k3s kubectl -n dragz-prod delete certificate dragz-tls
sudo k3s kubectl -n dragz-prod delete secret dragz-tls
sudo k3s kubectl apply -f /opt/dragz/deploy/k3s/tls.yaml
```

## 从已有的 `qinghe` 部署升级

所有东西都改名成了 `dragz`：命名空间 `qinghe-prod` -> `dragz-prod`，
容器 `qinghe-postgres`/`qinghe-redis` -> `dragz-*`，宿主机数据
`/data/qinghe` -> `/data/dragz`，仓库目录 `/opt/qinghe` -> `/opt/dragz`，
镜像路径 `hub.cszqq.top/qinghe/*` -> `hub.cszqq.top/dragz/*`。

**只改清单不会搬任何数据。** 在已有服务器上必须先跑下面的步骤，
否则 Postgres 会在空目录上启动，看起来就像数据全丢了：

```bash
# 1. 停掉旧栈，搬数据，再让新路径接管
cd /opt/qinghe && sudo docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod-infra.yml down
sudo mv /data/qinghe /data/dragz
sudo mv /opt/qinghe /opt/dragz

# 2. 重命名数据库角色/库，这样 .env.prod 才能去掉 "qinghe"
#    （先用旧的 .env.prod 把栈启动一次，然后：）
sudo docker exec dragz-postgres psql -U qinghe -d postgres \
  -c 'ALTER ROLE qinghe RENAME TO dragz;' \
  -c 'ALTER DATABASE qinghe RENAME TO dragz;'
# 然后更新 deploy/.env.prod：PG_DATABASE=dragz, PG_USER=dragz

# 3. 轮换 seed 出来的管理员密码哈希（可选，保证账号可用）
sudo docker exec dragz-postgres psql -U dragz -d dragz -c \
  "UPDATE sys_user SET password_hash = '\$2a\$10\$GsG.n57BtMaOq9w2dqZPqOzZGbROiaqmwpX0MOLQPot9RKwMvQKZ.', must_change_password = 1 WHERE username = 'admin';"

# 4. 重新部署到新命名空间，然后删掉旧的
sudo bash /opt/dragz/deploy/k3s/scripts/deploy-app.sh
sudo k3s kubectl delete namespace qinghe-prod
```

镜像必须按新路径重建并推送（`build-and-push.ps1` 已经指向
`hub.cszqq.top/dragz/*`）；`deploy-app.sh` 钉死了
`${REGISTRY_SERVER}/dragz/${d}`，否则拉不到旧 tag。

## 升级还带 `sys_api_permission` 的部署

接口权限表已经合并进菜单按钮：`sys_api_permission`、
`sys_role_api_permission` 和 `sys_menu_api_permission` 被删除，
每个权限码变成一行 `sys_menu`，`type = 'button'`。

转换在新镜像第一次启动时自动执行
（`store.migrateLegacyAPIPermissions`：回填、拷贝授权，然后在一个事务里删掉三张表），
所以**不需要手工 SQL**。有两点要知道：

- **只能单副本。** 旧 Pod 仍在查询 `sys_api_permission`，第一个新 Pod 删掉它之后
  就会返 500。发布期间把 `admin-api` 缩到 1 个副本（或者接受用
  `--max-surge 0` 换来的一小段窗口），发布完再扩回去。
- **先核对数量**，如果你想留个凭证：

  ```bash
  sudo docker exec dragz-postgres psql -U dragz -d dragz -c \
    'SELECT role_id, count(*) FROM sys_role_api_permission GROUP BY 1 ORDER BY 1;'
  ```

  发布后同样的数字必须能被这条匹配上：

  ```sql
  SELECT rm.role_id, count(*) FROM sys_role_menu rm
  JOIN sys_menu m ON m.id = rm.menu_id AND m.type = 'button'
  GROUP BY 1 ORDER BY 1;
  ```

  授权只会变多：属于已不存在模块的码（接口管理本身）会被拷过去，
  随后被退役模块清理删掉。

## 运维

```bash
# 状态
sudo k3s kubectl -n dragz-prod get pods,svc,ingress,certificate
# 日志
sudo k3s kubectl -n dragz-prod logs deploy/admin-api --tail=100 -f
# 宿主机数据层
cd /opt/dragz && sudo docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod-infra.yml ps
# 数据库备份
sudo docker exec dragz-postgres pg_dump -U dragz dragz > /data/dragz/backup/dragz-$(date +%F).sql
```

发布新版本：改 `deploy/.env.prod` 里的 `APP_TAG`，跑
`build-and-push.ps1`，再跑 `deploy-app.sh`（或 `deploy.ps1 -SkipBootstrap`）。

## 存储 / CDN

默认上传目标是本地（`./data/uploads`，落在节点的
`/data/dragz/uploads`）。要切到 OSS + CDN
（`img.diy.cszqq.top`），在管理后台配置一条 `storage_type=s3`
（设置 -> 存储）。不需要改代码或清单。
