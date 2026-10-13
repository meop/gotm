[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$GotmRepo = 'meop/gotm'
$GotmBin = "$env:USERPROFILE\.gotm\bin"
$Releases = "https://github.com/$GotmRepo/releases"

$IsArm64 = [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture -eq `
  [System.Runtime.InteropServices.Architecture]::Arm64
$GoArch = if ($IsArm64) { 'arm64' } else { 'amd64' }

# every request is a plain page or release download: no GitHub API, so no rate
# limit and no token. the newest release is where the release page's "latest"
# address redirects to
Write-Host "Fetching latest gotm release: github.com/$GotmRepo"
$latest = Invoke-WebRequest -ErrorAction Stop -UseBasicParsing -Method Head -Uri "$Releases/latest"
$resolved = if ($latest.BaseResponse.ResponseUri) { $latest.BaseResponse.ResponseUri } else { $latest.BaseResponse.RequestMessage.RequestUri }
$Tag = ([string]$resolved).TrimEnd('/').Split('/')[-1]
if ($Tag -notmatch '^v\d') {
  Write-Error "Could not read the latest release from $Releases/latest"
  exit 1
}
Write-Host "  version: $Tag"

$Asset = "gotm-$($Tag.TrimStart('v'))-windows-$GoArch.zip"
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  Write-Host "  downloading $Releases/download/$Tag/$Asset"
  Invoke-WebRequest -ErrorAction Stop -UseBasicParsing -Uri "$Releases/download/$Tag/$Asset" -OutFile "$tmp\$Asset"
  Invoke-WebRequest -ErrorAction Stop -UseBasicParsing -Uri "$Releases/download/$Tag/SHA256SUMS" -OutFile "$tmp\SHA256SUMS"

  # the archive is checked against the release's own checksum manifest before
  # anything in it is used
  $want = (Get-Content "$tmp\SHA256SUMS" | Where-Object { $_ -match " $([regex]::Escape($Asset))$" } | Select-Object -First 1) -split '\s+' | Select-Object -First 1
  $got = (Get-FileHash -Algorithm SHA256 "$tmp\$Asset").Hash.ToLower()
  if (-not $want -or $want.ToLower() -ne $got) {
    Write-Error "$Asset does not match its checksum (want: $want, got: $got)"
    exit 1
  }

  Expand-Archive "$tmp\$Asset" -DestinationPath "$tmp\unpacked" -Force
  $exe = Get-ChildItem "$tmp\unpacked" -Recurse -Filter 'gotm.exe' | Select-Object -First 1
  if (-not $exe) {
    Write-Error "gotm.exe not found in $Asset"
    exit 1
  }
  New-Item -ItemType Directory -Path $GotmBin -Force | Out-Null
  Copy-Item $exe.FullName "$GotmBin\gotm.exe" -Force
  Write-Host "  installed $GotmBin\gotm.exe"
} finally {
  Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host ''
Write-Host 'Refer to the project README for how to activate gotm in your shell.'
