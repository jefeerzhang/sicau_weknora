# Run Go tests for WeKnora using the official golang Docker image (CGO on).
# Usage (from repo root or any checkout):
#   .\scripts\docker-go-test.ps1
#   .\scripts\docker-go-test.ps1 -Worktree .worktrees\sync-upstream-v0.8.0 -Packages "./internal/handler/"
#   .\scripts\docker-go-test.ps1 -Run "Invitation|TenantMember"

param(
    [string]$Worktree = ".",
    [string]$Packages = "./internal/application/service/ ./internal/handler/ ./internal/router/ ./internal/types/",
    [string]$Run = "",
    [string]$Image = "golang:1.26",
    [string]$Timeout = "15m"
)

$ErrorActionPreference = "Stop"
$src = (Resolve-Path $Worktree).Path -replace '\\', '/'
$args = @("test") + ($Packages -split '\s+' | Where-Object { $_ }) + @("-count=1", "-timeout", $Timeout)
if ($Run) { $args += @("-run", $Run) }

Write-Host "docker $Image go $($args -join ' ')  (workdir=$src)"
docker run --rm `
  -v "${src}:/src" `
  -v weknora-go-mod-cache:/go/pkg/mod `
  -v weknora-go-build-cache:/root/.cache/go-build `
  -w /src `
  -e CGO_ENABLED=1 `
  $Image `
  go @args
