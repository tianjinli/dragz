#!/usr/bin/env bash
# Bootstrap a fresh Ubuntu 24.04 node for Dragz.
# Run ON the server as root:
#   sudo bash bootstrap-server.sh
#
# Installs Docker (Ubuntu packages; the preinstalled snap cannot bind-mount
# /data), containerd registry mirrors, k3s (Traefik + servicelb, stable channel
# from the China mirror), Helm and cert-manager.
set -euo pipefail

log() { echo -e "\n==> $*"; }

if [ "$(id -u)" -ne 0 ]; then
  echo "Please run as root: sudo bash bootstrap-server.sh" >&2
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

log "Installing base packages"
apt-get update -y
apt-get install -y curl ca-certificates

log "Installing Docker (Ubuntu packages, unconfined)"
if command -v snap >/dev/null 2>&1 && snap list docker >/dev/null 2>&1; then
  snap remove --purge docker || true
fi
if ! command -v docker >/dev/null 2>&1; then
  apt-get install -y docker.io docker-compose-v2
fi
systemctl enable --now docker
usermod -aG docker ubuntu || true

log "Configuring Docker Hub mirror"
mkdir -p /etc/docker
install -m 0644 "$SCRIPT_DIR/../daemon.json" /etc/docker/daemon.json
systemctl restart docker
docker --version
docker compose version

log "Configuring containerd registry mirrors"
mkdir -p /etc/rancher/k3s
install -m 0644 "$SCRIPT_DIR/../registries.yaml" /etc/rancher/k3s/registries.yaml

log "Installing k3s (stable, China mirror)"
if ! command -v k3s >/dev/null 2>&1; then
  curl -sfL https://rancher-mirror.rancher.cn/k3s/k3s-install.sh \
    | INSTALL_K3S_MIRROR=cn INSTALL_K3S_EXEC="--write-kubeconfig-mode 644" sh -
else
  systemctl restart k3s
fi
kubectl wait --for=condition=Ready node --all --timeout=180s
kubectl get nodes -o wide

log "Installing Helm (get.helm.sh)"
if ! command -v helm >/dev/null 2>&1; then
  HELM_VERSION="${HELM_VERSION:-v3.16.4}"
  curl -fsSL "https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz" -o /tmp/helm.tgz
  tar -xzf /tmp/helm.tgz -C /tmp linux-amd64/helm
  install -m 0755 /tmp/linux-amd64/helm /usr/local/bin/helm
  rm -rf /tmp/helm.tgz /tmp/linux-amd64
fi
helm version --short

log "Installing cert-manager"
helm repo add jetstack https://charts.jetstack.io >/dev/null 2>&1 || true
helm repo update jetstack >/dev/null
helm upgrade -i cert-manager jetstack/cert-manager \
  -n cert-manager --create-namespace \
  --set crds.enabled=true \
  --set startupapicheck.enabled=false \
  --timeout 10m
kubectl -n cert-manager rollout status deploy/cert-manager --timeout=300s
kubectl -n cert-manager rollout status deploy/cert-manager-cainjector --timeout=300s
kubectl -n cert-manager rollout status deploy/cert-manager-webhook --timeout=300s

log "Creating host data directories"
mkdir -p /data/dragz/pg /data/dragz/redis /data/dragz/uploads /data/dragz/backup
chmod 0777 /data/dragz/uploads
ls -ld /data/dragz/*

log "Bootstrap complete"
echo "Node IP(s): $(hostname -I)"
