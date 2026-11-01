#!/usr/bin/env bash
# Ship the already-pushed images to k3s: upload deploy files + migrations, run deploy-app.sh.
# Images must be built and pushed first (deploy/k3s/build-and-push.ps1).
#
#   bash deploy/k3s/deploy-manual.sh [path/to/key.pem]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SERVER="${SERVER:-ubuntu@111.230.232.233}"
KEY="${1:-$TEMP/dragz-deploy.pem}"

[[ -f "$KEY" ]] || { echo "Missing key: $KEY" >&2; exit 1; }
chmod 600 "$KEY"

# APP_TAG is read from the env file that gets uploaded with the deploy dir.
TAG="$(sed -n 's/^APP_TAG=//p' "$ROOT/deploy/.env.prod")"
echo "==> Deploying tag ${TAG:-<empty>} to $SERVER"
[[ -n "$TAG" ]] || { echo "APP_TAG missing in deploy/.env.prod" >&2; exit 1; }

OPTS=(-i "$KEY" -o StrictHostKeyChecking=accept-new -o IdentitiesOnly=yes -o ConnectTimeout=20 -o LogLevel=ERROR)

ssh "${OPTS[@]}" "$SERVER" 'sudo mkdir -p /opt/dragz && sudo chown -R ubuntu:ubuntu /opt/dragz && mkdir -p /opt/dragz/backend/migrations'
scp "${OPTS[@]}" -r "$ROOT/deploy" "$SERVER:/opt/dragz/"
scp "${OPTS[@]}" "$ROOT"/backend/migrations/*.sql "$SERVER:/opt/dragz/backend/migrations/"
ssh "${OPTS[@]}" "$SERVER" 'sudo bash /opt/dragz/deploy/k3s/scripts/deploy-app.sh'

echo "==> Rollout images"
ssh "${OPTS[@]}" "$SERVER" 'export KUBECONFIG=/etc/rancher/k3s/k3s.yaml; sudo kubectl -n dragz-prod get deploy -o custom-columns=NAME:.metadata.name,IMAGE:.spec.template.spec.containers[0].image'
