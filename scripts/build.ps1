$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$env:GOCACHE = Join-Path $repoRoot ".gocache"
$env:GOMODCACHE = Join-Path $repoRoot ".gomodcache"
$goexe = (go env GOEXE).Trim()
$output = Join-Path $repoRoot ("aicommit" + $goexe)

go build -o $output .
