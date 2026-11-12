#requires -version 5.1
<#
.SYNOPSIS
  Downloads the Windows W0 toolchain and native libraries used by Tingyu.

.DESCRIPTION
  Fetches, with SHA-256 verification:
    * Go 1.27.3 for windows-amd64 (the version client/go.mod requires)
    * libmpv-2.dll from the shinchiro/mpv-winbuild-cmake release 20261101
    * mygo-sqlite3.dll from the MyGo sqlite-3.53.4-1 release

  Go is extracted under -Tools (default G:\tingyu-windows-tools, outside the
  repository). libmpv and SQLite are staged next to the executable under
  client\build, which .gitignore already excludes.

  Nothing is installed system-wide and nothing is added to PATH.

.EXAMPLE
  powershell -ExecutionPolicy Bypass -File .\scripts\windows-setup.ps1
#>
param(
  [string]$Tools = "G:\tingyu-windows-tools",
  [switch]$Force
)
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$client = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$build = Join-Path $client "build"
$mpvDir = Join-Path $build "mpv"
New-Item -ItemType Directory -Force -Path $Tools, $build, $mpvDir | Out-Null

$GoVersion = "1.27.3"
$GoZip = "go$GoVersion.windows-amd64.zip"
$GoURL = "https://go.dev/dl/$GoZip"
$GoSHA = "F26081F438319757F8A1B46D7D0B2C5EC7583D4038E485E0AA725CFDE7885FFF"

$MpvArchive = "mpv-dev-x86_64-20261101-git-48fca90e8c.7z"
$MpvURL = "https://github.com/shinchiro/mpv-winbuild-cmake/releases/download/20261101/$MpvArchive"
$MpvArchiveSHA = "30D3F9FA121CE24638610A9AE52F8957743F9DEA416DD37DC4FF7E43589EF843"
$MpvDLLSHA = "F40293DC5F4012F3F7BCC79DE2B9BFB66309B87DBD33A47EE49EBFF716EB8154"

$SQLiteDLL = "mygo-sqlite3-windows-amd64.dll"
$SQLiteURL = "https://github.com/egoist/mygo/releases/download/sqlite-3.53.4-1/$SQLiteDLL"
$SQLiteSHA = "A9FA4B1EAE82D8F9826D6B3CAC20E337EBB29B5E859EAB3D2852D65B323224F4"

function Get-File([string]$Url, [string]$Dest, [string]$SHA256) {
  if ((Test-Path $Dest) -and -not $Force) {
    Write-Host "have    $Dest"
  } else {
    Write-Host "fetch   $Url"
    Invoke-WebRequest -Uri $Url -OutFile $Dest -UseBasicParsing -TimeoutSec 900
  }
  $actual = (Get-FileHash $Dest -Algorithm SHA256).Hash
  if ($SHA256 -and $actual -ne $SHA256) {
    throw "SHA-256 mismatch for $Dest`: got $actual, want $SHA256"
  }
  Write-Host "verify  $Dest $actual"
}

# 1. Go
$goRoot = Join-Path $Tools "go"
if (-not (Test-Path (Join-Path $goRoot "bin\go.exe")) -or $Force) {
  $zip = Join-Path $Tools $GoZip
  Get-File $GoURL $zip $GoSHA
  if (Test-Path $goRoot) { Remove-Item -Recurse -Force $goRoot }
  Write-Host "extract $zip -> $Tools"
  Expand-Archive -Path $zip -DestinationPath $Tools -Force
}
& (Join-Path $goRoot "bin\go.exe") version

# 2. 7zr (standalone 7-Zip extractor; mpv ships .7z)
$sevenZip = Join-Path $Tools "7zr.exe"
if (-not (Test-Path $sevenZip) -or $Force) {
  Write-Host "fetch   https://www.7-zip.org/a/7zr.exe"
  Invoke-WebRequest -Uri "https://www.7-zip.org/a/7zr.exe" -OutFile $sevenZip -UseBasicParsing -TimeoutSec 300
}

# 3. libmpv
$mpvArchive = Join-Path $Tools $MpvArchive
Get-File $MpvURL $mpvArchive $MpvArchiveSHA
$mpvDLL = Join-Path $mpvDir "libmpv-2.dll"
if (-not (Test-Path $mpvDLL) -or $Force) {
  & $sevenZip e $mpvArchive "-o$mpvDir" "libmpv-2.dll" -y | Out-Null
  if ($LASTEXITCODE -ne 0) { throw "7zr failed to extract libmpv-2.dll" }
}
$actual = (Get-FileHash $mpvDLL -Algorithm SHA256).Hash
if ($actual -ne $MpvDLLSHA) { throw "libmpv-2.dll SHA-256 $actual, want $MpvDLLSHA" }
Write-Host "verify  $mpvDLL $actual"
(Get-Item $mpvDLL).VersionInfo | Format-List FileVersion, ProductVersion, FileDescription, LegalCopyright

# 4. SQLite for MyGo
$sqliteDest = Join-Path $build "mygo-sqlite3.dll"
Get-File $SQLiteURL $sqliteDest $SQLiteSHA

Write-Host ""
Write-Host "toolchain ready:"
Write-Host "  go:     $goRoot\bin\go.exe"
Write-Host "  libmpv: $mpvDLL"
Write-Host "  sqlite: $sqliteDest"
Write-Host "next:     powershell -ExecutionPolicy Bypass -File .\scripts\windows-build.ps1 -Accept"
