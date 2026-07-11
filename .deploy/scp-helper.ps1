param(
  [Parameter(Mandatory = $true)][string]$HostName,
  [Parameter(Mandatory = $true)][string]$LocalPath,
  [Parameter(Mandatory = $true)][string]$RemotePath
)

$ErrorActionPreference = "Stop"

if (-not $env:DEPLOY_PASS) {
  throw "DEPLOY_PASS is required"
}

$ask = Join-Path $PSScriptRoot "askpass.cmd"
Set-Content -LiteralPath $ask -Value "@echo off`necho $env:DEPLOY_PASS" -Encoding ASCII

try {
  $env:SSH_ASKPASS = $ask
  $env:SSH_ASKPASS_REQUIRE = "force"
  $env:DISPLAY = ":0"
  scp -o StrictHostKeyChecking=accept-new -o NumberOfPasswordPrompts=1 $LocalPath "root@${HostName}:$RemotePath"
} finally {
  Remove-Item -LiteralPath $ask -Force -ErrorAction SilentlyContinue
}
