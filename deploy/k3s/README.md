# Dragz on k3s (Tencent Cloud, China mainland)

**English** | [中文](README_CN.md)

Single-node k3s deployment for `diy.cszqq.top` on `111.230.232.233` (Ubuntu 24.04,
2 vCPU / 4 GB). PostgreSQL and Redis run on the host via Docker Compose, outside
of k3s, so the cluster can be rebuilt without touching data.

```
diy.cszqq.top (Traefik :80/:443)
├── /api/*    -> svc/admin-api:5280        (admin-api, go-zero)
└── /*        -> svc/admin-web:80          (nginx + built SPA)
jobs-runner   -> asynq worker (no Service)

host docker compose: postgres:16-alpine + redis:7.4-alpine
  bound to 10.1.0.9 (private IP) only, reached from pods via
  selector-less Services + Endpoints in external-db.yaml.tpl
```

## Files

| Path | Purpose |
| --- | --- |
| `config/{admin-api,jobs-runner}.yaml` | go-zero config, mounted as ConfigMap |
| `external-db.yaml.tpl` | Services + Endpoints pointing at the node private IP |
| `admin-api.yaml` / `jobs-runner.yaml` / `admin-web.yaml` | workloads |
| `ingress.yaml` | one Ingress per path (`/api` -> admin-api, `/` -> admin-web) |
| `tls.yaml` | ClusterIssuer (Let's Encrypt DNS-01) + Certificate |
| `registries.yaml` | containerd mirrors (Docker Hub / quay / ghcr) |
| `daemon.json` | host Docker Hub mirror for Postgres/Redis pulls |
| `scripts/bootstrap-server.sh` | installs docker, k3s, helm, cert-manager, mirrors |
| `scripts/deploy-app.sh` | applies every manifest, creates secrets, starts compose |
| `build-and-push.ps1` | local build + push to `hub.cszqq.top/dragz/*` |
| `deploy.ps1` | end-to-end: build/push, upload, bootstrap, deploy |

## First deploy

1. Fill `deploy/.env.prod` (gitignored). Required keys are documented in
   `deploy/.env.prod.example`. `CONFIG_MASTER_KEY` must be identical for
   admin-api and jobs-runner.
2. Make sure `diy.cszqq.top` A record points at `111.230.232.233`.
3. From the repo root:

   ```powershell
   powershell -File deploy/k3s/deploy.ps1
   ```

   The server only accepts the `ubuntu` user with the provided key
   (`sudo` is passwordless); `deploy.ps1` handles that.

## China-network notes (important)

The server cannot reach GitHub or Docker Hub directly. The bootstrap script
works around this:

- **kernel/k3s binary**: installed with `INSTALL_K3S_MIRROR=cn`
  (`rancher-mirror.rancher.cn`).
- **Helm**: downloaded from `get.helm.sh` (reachable); the `get-helm-3` script
  on `raw.githubusercontent.com` is not.
- **containerd images**: `/etc/rancher/k3s/registries.yaml` mirrors
  `docker.io` -> `mirror.ccs.tencentyun.com`, `quay.io` -> `quay.m.daocloud.io`,
  `ghcr.io` -> `ghcr.m.daocloud.io`.
- **host Docker**: `/etc/docker/daemon.json` mirrors Docker Hub so the
  Postgres/Redis compose stack can pull.
- **snap docker is removed**: it is AppArmor-confined and cannot bind-mount
  `/data/dragz`. Use the apt `docker.io` + `docker-compose-v2` packages.

## Images

Preferred: push to `hub.cszqq.top/dragz/*` and let k3s pull with the
`hub-registry` pull secret.

Offline fallback (used when the registry is unavailable):

```bash
docker save -o dragz-images.tar <4 images>
scp dragz-images.tar ubuntu@111.230.232.233:/tmp/
ssh ubuntu@111.230.232.233 "sudo k3s ctr -n k8s.io images import /tmp/dragz-images.tar"
```

`imagePullPolicy: IfNotPresent` uses the imported image without contacting the
registry.

## TLS

`tls.yaml` creates a `letsencrypt-prod` ClusterIssuer and a `dragz-tls`
Certificate using **DNS-01 via Alibaba Cloud DNS**, because HTTP-01 is
unusable here: overseas resolvers still see a stale record pointing at
`43.174.224.0/24` (Alibaba Singapore), which serves a DNSPod block page.

Prerequisites (one-time):

```bash
# AliDNS webhook (Aliyun image, reachable in mainland)
helm repo add alidns https://wjiec.github.io/alidns-webhook
helm upgrade --install alidns-webhook alidns/alidns-webhook \
  -n cert-manager \
  --set groupName=acme.cszqq.top \
  --set image.repository=registry.cn-hangzhou.aliyuncs.com/wjiec/alidns-webhook \
  --set image.tag=latest

# credentials (RAM user with DNS permission for the zone);
# deploy-app.sh creates this automatically when ALIDNS_ACCESS_KEY_ID and
# ALIDNS_ACCESS_KEY_SECRET are set in deploy/.env.prod
kubectl -n cert-manager create secret generic alidns-secret \
  --from-literal=access-key-id='<id>' \
  --from-literal=access-key-secret='<secret>'
```

Check and force-retry:

```bash
sudo k3s kubectl -n dragz-prod get certificate
sudo k3s kubectl -n dragz-prod describe certificate dragz-tls
```

If an attempt failed while DNS/credentials were wrong, delete the terminal
resources and the secret so a fresh order starts (cert-manager will recreate
the secret on issuance):

```bash
sudo k3s kubectl -n dragz-prod delete certificaterequest,order,challenge --all
sudo k3s kubectl -n dragz-prod delete certificate dragz-tls
sudo k3s kubectl -n dragz-prod delete secret dragz-tls
sudo k3s kubectl apply -f /opt/dragz/deploy/k3s/tls.yaml
```

## Upgrading an existing `qinghe` deployment

Everything was renamed to `dragz`: namespace `qinghe-prod` -> `dragz-prod`,
containers `qinghe-postgres`/`qinghe-redis` -> `dragz-*`, host data
`/data/qinghe` -> `/data/dragz`, repo dir `/opt/qinghe` -> `/opt/dragz`,
registry path `hub.cszqq.top/qinghe/*` -> `hub.cszqq.top/dragz/*`.

**The manifests alone do not move any data.** On an existing server run this
first, or Postgres will start on an empty directory and look like total data
loss:

```bash
# 1. stop the old stack, move data, then let the new paths take over
cd /opt/qinghe && sudo docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod-infra.yml down
sudo mv /data/qinghe /data/dragz
sudo mv /opt/qinghe /opt/dragz

# 2. rename the database role/database so .env.prod can drop "qinghe"
#    (start the stack once with the OLD .env.prod values, then:)
sudo docker exec dragz-postgres psql -U qinghe -d postgres \
  -c 'ALTER ROLE qinghe RENAME TO dragz;' \
  -c 'ALTER DATABASE qinghe RENAME TO dragz;'
# then update deploy/.env.prod: PG_DATABASE=dragz, PG_USER=dragz

# 3. rotate the seeded admin password hash (optional, keeps the account usable)
sudo docker exec dragz-postgres psql -U dragz -d dragz -c \
  "UPDATE sys_user SET password_hash = '\$2a\$10\$GsG.n57BtMaOq9w2dqZPqOzZGbROiaqmwpX0MOLQPot9RKwMvQKZ.', must_change_password = 1 WHERE username = 'admin';"

# 4. re-apply into the new namespace, then drop the old one
sudo bash /opt/dragz/deploy/k3s/scripts/deploy-app.sh
sudo k3s kubectl delete namespace qinghe-prod
```

Images must be rebuilt and pushed under the new path (`build-and-push.ps1`
already targets `hub.cszqq.top/dragz/*`); `deploy-app.sh` pins
`${REGISTRY_SERVER}/dragz/${d}` and will fail to pull the old tags otherwise.

## Upgrading a deployment that still has `sys_api_permission`

The API-permission tables were folded into menu buttons: `sys_api_permission`,
`sys_role_api_permission` and `sys_menu_api_permission` are dropped, and every
permission code becomes a `sys_menu` row with `type = 'button'`.

The conversion runs by itself on the first boot of the new image
(`store.migrateLegacyAPIPermissions`: backfill, copy grants, then drop the three
tables in one transaction), so **no manual SQL is needed**. Two things to know:

- **Single replica only.** The old pods still query `sys_api_permission` and will
  return 500 once the first new pod has dropped it. Scale `admin-api` to one
  replica (or accept a short window with `--max-surge 0`) for the rollout, then
  scale back up.
- **Check the counts first** if you want a paper trail:

  ```bash
  sudo docker exec dragz-postgres psql -U dragz -d dragz -c \
    'SELECT role_id, count(*) FROM sys_role_api_permission GROUP BY 1 ORDER BY 1;'
  ```

  After the rollout the same numbers must be matched by

  ```sql
  SELECT rm.role_id, count(*) FROM sys_role_menu rm
  JOIN sys_menu m ON m.id = rm.menu_id AND m.type = 'button'
  GROUP BY 1 ORDER BY 1;
  ```

  Grants only grow: codes belonging to modules that no longer exist (API Management
  itself) are copied and then removed again by the retired-module cleanup.

## Operations

```bash
# status
sudo k3s kubectl -n dragz-prod get pods,svc,ingress,certificate
# logs
sudo k3s kubectl -n dragz-prod logs deploy/admin-api --tail=100 -f
# host data layer
cd /opt/dragz && sudo docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod-infra.yml ps
# database backup
sudo docker exec dragz-postgres pg_dump -U dragz dragz > /data/dragz/backup/dragz-$(date +%F).sql
```

To ship a new version: bump `APP_TAG` in `deploy/.env.prod`, run
`build-and-push.ps1`, then `deploy-app.sh` (or `deploy.ps1 -SkipBootstrap`).

## Storage / CDN

The default upload target is local (`./data/uploads`, backed by
`/data/dragz/uploads` on the node). To switch to OSS + CDN
(`img.diy.cszqq.top`), configure a `storage_type=s3` entry in the admin UI
(Settings -> Storage). No code or manifest change is required.
