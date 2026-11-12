#requires -version 5.1
<#
.SYNOPSIS
  Builds the Tingyu Windows client and the W0 acceptance harness.

.DESCRIPTION
  Produces:
    build\tingyu.exe          the real client (same main.go / app.Main as macOS)
    build\w0accept.exe        the SQLite + libmpv W0 check
    build\libmpv-2.dll        expected next to the executable (see windows-setup.ps1)
    build\mygo-sqlite3.dll    expected next to the executable

  Use -Accept to run w0accept.exe after building. Use -Run to start the client
  with a throwaway data directory, a generated tone, scan it and play it, then
  quit (a smoke run; see docs/windows-w0.md for the recorded evidence).

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\scripts\windows-build.ps1 -Accept
#>
param(
  [string]$Go = "",
  [string]$Tools = "G:\tingyu-windows-tools",
  [switch]$Accept,
  [switch]$Test,
  [switch]$Run,
  [switch]$Window
)
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$client = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $client
$build = Join-Path $client "build"
New-Item -ItemType Directory -Force -Path $build | Out-Null

if (-not $Go) {
  if ($env:GO) { $Go = $env:GO }
  elseif (Get-Command go.exe -ErrorAction SilentlyContinue) { $Go = (Get-Command go.exe).Source }
  elseif (Test-Path (Join-Path $Tools "go\bin\go.exe")) { $Go = Join-Path $Tools "go\bin\go.exe" }
}
if (-not $Go -or -not (Test-Path $Go)) {
  throw "Go 1.27.3 not found. Run scripts\windows-setup.ps1, install Go, or pass -Go <path>."
}
& $Go version

$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

Write-Host "build   tingyu.exe"
& $Go build -trimpath -o (Join-Path $build "tingyu.exe") .
if ($LASTEXITCODE -ne 0) { throw "go build tingyu.exe failed" }

Write-Host "build   w0accept.exe"
& $Go build -trimpath -o (Join-Path $build "w0accept.exe") ./cmd/w0accept
if ($LASTEXITCODE -ne 0) { throw "go build w0accept.exe failed" }

if ($Test) {
  Write-Host "test    go test ./..."
  & $Go test ./...
  if ($LASTEXITCODE -ne 0) { throw "go test failed" }
}

$mpv = Join-Path $build "mpv\libmpv-2.dll"
$sqlite = Join-Path $build "mygo-sqlite3.dll"
foreach ($dep in @($mpv, $sqlite)) {
  if (-not (Test-Path $dep)) { throw "missing $dep; run scripts\windows-setup.ps1" }
}

if ($Accept) {
  Write-Host "accept  w0accept"
  $evidence = Join-Path $build "w0-evidence\w0accept"
  & (Join-Path $build "w0accept.exe") -mpv $mpv -dir $evidence
  if ($LASTEXITCODE -ne 0) { throw "w0accept failed" }
}

if ($Run) {
  $data = Join-Path $build "w0-data"
  $media = Join-Path $build "w0-media"
  New-Item -ItemType Directory -Force -Path $data, $media, (Join-Path $build "w0-evidence") | Out-Null
  & (Join-Path $build "w0accept.exe") -wav (Join-Path $media "w0-tone.wav") | Out-Null
  $env:TINGYU_DATA_DIR = $data
  $env:TINGYU_AUTO_SCAN_DIR = $media
  $env:TINGYU_AUTO_PLAY_INDEX = "0"
  $env:TINGYU_AUTO_QUIT_MS = "15000"
  $env:TINGYU_STARTUP_LOG = Join-Path $build "w0-evidence\app-run.log"
  Write-Host "run     tingyu.exe (data=$data, media=$media, quits in 15 s)"
  & (Join-Path $build "tingyu.exe")
  Get-Content $env:TINGYU_STARTUP_LOG -Encoding UTF8 | Select-String -Pattern "library:|scan:|play:|sound after|quit" | ForEach-Object { $_.Line }
}

if ($Window) {
  Write-Host "window  windows-window-test.ps1"
  & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot "windows-window-test.ps1")
  if ($LASTEXITCODE -ne 0) { throw "window test failed" }
}

Write-Host ""
Get-ChildItem $build -File | Select-Object Name, Length | Format-Table -AutoSize
