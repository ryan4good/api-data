param(
  [Parameter(Mandatory = $true)][string]$HostName,
  [Parameter(Mandatory = $true, ValueFromRemainingArguments = $true)][string[]]$RemoteCommand
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
  $verboseArgs = @()
  if ($env:DEPLOY_SSH_VERBOSE -eq "1") {
    $verboseArgs += "-v"
  }
  ssh @verboseArgs -o StrictHostKeyChecking=accept-new -o NumberOfPasswordPrompts=1 "root@$HostName" ($RemoteCommand -join " ")
} finally {
  Remove-Item -LiteralPath $ask -Force -ErrorAction SilentlyContinue
}
