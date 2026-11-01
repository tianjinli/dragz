# Build and push Dragz images to the container registry.
# Run from anywhere:  pwsh -File deploy/k3s/build-and-push.ps1 [-Tag v1.0.1]
param(
    [string]$Tag = ""
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$envFile = Join-Path $root "deploy\.env.prod"
if (-not (Test-Path $envFile)) { throw "Missing $envFile" }

Get-Content $envFile | ForEach-Object {
    if ($_ -match '^\s*([A-Za-z_][A-Za-z0-9_]*)=(.*)$') {
        Set-Item -Path "Env:$($matches[1])" -Value $matches[2].Trim()
    }
}

if (-not $Tag) { $Tag = $env:APP_TAG }
if (-not $Tag) { throw "No tag. Pass -Tag or set APP_TAG in deploy/.env.prod" }

$registry = $env:REGISTRY_SERVER
if (-not $registry) { throw "REGISTRY_SERVER missing" }

$dockerConfig = Join-Path $env:TEMP "dragz-docker"
New-Item -ItemType Directory -Force -Path $dockerConfig | Out-Null
$env:DOCKER_CONFIG = $dockerConfig

Write-Host "==> Logging in to $registry" -ForegroundColor Cyan
$env:REGISTRY_PASSWORD | docker login $registry -u $env:REGISTRY_USERNAME --password-stdin
if ($LASTEXITCODE -ne 0) { throw "docker login failed" }

Write-Host "==> Building admin frontend" -ForegroundColor Cyan
pnpm --dir (Join-Path $root "frontend\admin") build:prod
if ($LASTEXITCODE -ne 0) { throw "frontend build failed" }

$backend = Join-Path $root "backend"

Write-Host "==> Building images (tag $Tag)" -ForegroundColor Cyan
docker build -f deploy/Dockerfile.admin-web.prod -t "$registry/dragz/admin-web:$Tag" $root
docker build -f backend/Dockerfile -t "$registry/dragz/admin-api:$Tag" $backend
docker build -f backend/Dockerfile.jobs-runner -t "$registry/dragz/jobs-runner:$Tag" $backend

Write-Host "==> Pushing images" -ForegroundColor Cyan
docker push "$registry/dragz/admin-web:$Tag"
docker push "$registry/dragz/admin-api:$Tag"
docker push "$registry/dragz/jobs-runner:$Tag"

Write-Host "==> Done: $registry/dragz/*:$Tag" -ForegroundColor Green
