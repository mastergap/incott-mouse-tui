# Build bin\incott.exe on Windows with plain PowerShell. See docs\BUILDING.md.
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go not found. Install with: winget install GoLang.Go"
}
$cc = (go env CC)
if (-not (Get-Command $cc -ErrorAction SilentlyContinue)) {
    throw "C compiler '$cc' not found. Install: winget install BrechtSanders.WinLibs.POSIX.UCRT (or set CC to your gcc.exe)."
}

$version = (git describe --tags --always --dirty 2>$null)
if (-not $version) { $version = 'dev' }

$env:CGO_ENABLED = '1'
go build -trimpath -ldflags "-s -w -X main.version=$version" -o bin\incott.exe .\cmd\incott
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Built bin\incott.exe ($version)"
