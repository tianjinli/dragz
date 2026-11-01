#!/usr/bin/env bash
# Deploy the Dragz application stack onto k3s.
# Run ON the server as root from the repo root:
#   sudo bash deploy/k3s/scripts/deploy-app.sh
#
# Idempotent: safe to re-run after pushing new images.
set -euo pipefail

REPO_DIR="${REPO_DIR:-/opt/dragz}"
ENV_FILE="${ENV_FILE:-$REPO_DIR/deploy/.env.prod}"
K8S_DIR="$REPO_DIR/deploy/k3s"
NS=dragz-prod

if [ ! -f "$ENV_FILE" ]; then
  echo "Missing $ENV_FILE" >&2
  exit 1
fi

cd "$REPO_DIR"
set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

APP_TAG="${APP_TAG:-v1.0.1}"
NODE_IP="${DB_BIND_IP:-$(hostname -I | awk '{print $1}')}"
PG_HOST="dragz-postgres.${NS}.svc.cluster.local"
REDIS_HOST="dragz-redis.${NS}.svc.cluster.local"
DSN="postgres://${PG_USER}:${PG_PASSWORD}@${PG_HOST}:5432/${PG_DATABASE}?sslmode=disable"

echo "==> Starting host infrastructure (Postgres + Redis), node IP ${NODE_IP}"
docker compose --env-file "$ENV_FILE" -f deploy/docker-compose.prod-infra.yml up -d

echo "==> Namespace + middleware"
kubectl apply -f "$K8S_DIR/namespace.yaml"

echo "==> ConfigMap"
kubectl -n "$NS" create configmap dragz-config \
  --from-file="$K8S_DIR/config/" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "==> Application secret"
kubectl -n "$NS" create secret generic dragz-app-secrets \
  --from-literal=ADMIN_DB_DSN="$DSN" \
  --from-literal=JOBS_DB_DSN="$DSN" \
  --from-literal=REDIS_ADDR="${REDIS_HOST}:6379" \
  --from-literal=REDIS_PASSWORD="${REDIS_PASSWORD}" \
  --from-literal=ADMIN_AUTH_SECRET="${ADMIN_AUTH_SECRET}" \
  --from-literal=CONFIG_MASTER_KEY="${CONFIG_MASTER_KEY}" \
  --from-literal=BOOTSTRAP_ADMIN_USERNAME="${BOOTSTRAP_ADMIN_USERNAME}" \
  --from-literal=BOOTSTRAP_ADMIN_PASSWORD="${BOOTSTRAP_ADMIN_PASSWORD}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "==> Registry pull secret"
kubectl -n "$NS" create secret docker-registry hub-registry \
  --docker-server="${REGISTRY_SERVER}" \
  --docker-username="${REGISTRY_USERNAME}" \
  --docker-password="${REGISTRY_PASSWORD}" \
  --dry-run=client -o yaml | kubectl apply -f -

if [ -n "${ALIDNS_ACCESS_KEY_ID:-}" ] && [ -n "${ALIDNS_ACCESS_KEY_SECRET:-}" ]; then
  echo "==> AliDNS secret (cert-manager DNS-01)"
  kubectl -n cert-manager create secret generic alidns-secret \
    --from-literal=access-key-id="${ALIDNS_ACCESS_KEY_ID}" \
    --from-literal=access-key-secret="${ALIDNS_ACCESS_KEY_SECRET}" \
    --dry-run=client -o yaml | kubectl apply -f -
else
  echo "==> AliDNS keys not set, skipping alidns-secret (DNS-01 needs it)"
fi

echo "==> External DB endpoints"
sed "s/__NODE_IP__/${NODE_IP}/g" "$K8S_DIR/external-db.yaml.tpl" | kubectl apply -f -

echo "==> TLS issuer + certificate"
kubectl apply -f "$K8S_DIR/tls.yaml"

echo "==> Deployments (tag ${APP_TAG})"
kubectl apply -f "$K8S_DIR/admin-api.yaml" \
              -f "$K8S_DIR/jobs-runner.yaml" \
              -f "$K8S_DIR/admin-web.yaml" \
              -f "$K8S_DIR/ingress.yaml"

for d in admin-api jobs-runner admin-web; do
  kubectl -n "$NS" set image "deployment/$d" "$d=${REGISTRY_SERVER}/dragz/${d}:${APP_TAG}"
done

echo "==> Waiting for rollouts"
for d in admin-api admin-web jobs-runner; do
  kubectl -n "$NS" rollout status "deploy/$d" --timeout=240s
done

echo "==> Status"
kubectl -n "$NS" get pods,svc,ingress,certificate
echo
echo "Health checks:"
echo "  curl -s https://diy.cszqq.top/api/health"
echo "  curl -s https://diy.cszqq.top/healthz"
