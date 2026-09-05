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

# sqlite-vec / some packages need C headers; slim golang images omit them.
# Keep PATH explicit: apt-get can run in a context where /usr/local/go/bin
# is not inherited by `bash -lc` depending on image entrypoint.
$quoted = ($args | ForEach-Object {
    "'" + ($_ -replace "'", "'\''") + "'"
}) -join ' '
$shell = @"
set -e
export PATH="/usr/local/go/bin:`$PATH"
apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq libsqlite3-dev >/dev/null
go $quoted
"@

Write-Host "docker $Image go $($args -join ' ')  (workdir=$src; +libsqlite3-dev)"
docker run --rm `
  -v "${src}:/src" `
  -v weknora-go-mod-cache:/go/pkg/mod `
  -v weknora-go-build-cache:/root/.cache/go-build `
  -w /src `
  -e CGO_ENABLED=1 `
  $Image `
  bash -lc $shell
