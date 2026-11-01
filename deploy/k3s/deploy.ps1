# End-to-end deploy: build/push images, bootstrap the server, ship manifests, apply.
# Run from the repo root or anywhere:
#   pwsh -File deploy/k3s/deploy.ps1
#   pwsh -File deploy/k3s/deploy.ps1 -SkipBuild
param(
    [string]$Server = "111.230.232.233",
    [string]$RemoteUser = "ubuntu",
    [string]$KeyPath = "C:\Users\fish\Desktop\dragz-root.pem",
    [string]$Tag = "",
    [switch]$SkipBuild,
    [switch]$SkipBootstrap
)

$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path

if (-not (Test-Path $KeyPath)) { throw "Key not found: $KeyPath" }
$key = Join-Path $env:TEMP "dragz-deploy.pem"
Copy-Item -LiteralPath $KeyPath -Destination $key -Force
icacls $key /inheritance:r /grant:r "$($env:USERNAME):(R)" | Out-Null

$sshOpts = @(
    "-i", $key,
    "-o", "StrictHostKeyChecking=accept-new",
    "-o", "ConnectTimeout=20",
    "-o", "IdentitiesOnly=yes",
    "-o", "LogLevel=ERROR"
)

function Invoke-Remote([string]$Command) {
    & ssh @sshOpts "$RemoteUser@$Server" $Command
    if ($LASTEXITCODE -ne 0) { throw "Remote command failed (exit $LASTEXITCODE): $Command" }
}

if (-not $SkipBuild) {
    Write-Host "==> Building and pushing images" -ForegroundColor Cyan
    $buildArgs = @("-NoProfile", "-File", (Join-Path $PSScriptRoot "build-and-push.ps1"))
    if ($Tag) { $buildArgs += @("-Tag", $Tag) }
    & powershell.exe @buildArgs
    if ($LASTEXITCODE -ne 0) { throw "build-and-push failed" }
}

Write-Host "==> Preparing /opt/dragz on $Server" -ForegroundColor Cyan
Invoke-Remote "sudo mkdir -p /opt/dragz && sudo chown -R ${RemoteUser}:${RemoteUser} /opt/dragz"

Write-Host "==> Uploading deploy files" -ForegroundColor Cyan
& scp @sshOpts -r (Join-Path $root "deploy") "${RemoteUser}@${Server}:/opt/dragz/"
if ($LASTEXITCODE -ne 0) { throw "scp deploy failed" }

& ssh @sshOpts "$RemoteUser@$Server" "mkdir -p /opt/dragz/backend/migrations"
if ($LASTEXITCODE -ne 0) { throw "remote mkdir failed" }

& scp @sshOpts (Get-ChildItem (Join-Path $root "backend\migrations") -File | ForEach-Object { $_.FullName }) "${RemoteUser}@${Server}:/opt/dragz/backend/migrations/"
if ($LASTEXITCODE -ne 0) { throw "scp migrations failed" }

if (-not $SkipBootstrap) {
    Write-Host "==> Bootstrapping server (docker/k3s/cert-manager)" -ForegroundColor Cyan
    Invoke-Remote "sudo bash /opt/dragz/deploy/k3s/scripts/bootstrap-server.sh"
}

Write-Host "==> Deploying application" -ForegroundColor Cyan
Invoke-Remote "sudo bash /opt/dragz/deploy/k3s/scripts/deploy-app.sh"

Write-Host "==> Done" -ForegroundColor Green
