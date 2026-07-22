#Requires -Version 5.1
<#
.SYNOPSIS
  Live API smoke against a running stack (docker compose / start.sh).

.DESCRIPTION
  Checks health, login/me, events, geo dry-run, ingest, maintenance backfill.
  Requires BASE_URL reachable (default http://127.0.0.1).

.EXAMPLE
  .\scripts\smoke-api.ps1
  .\scripts\smoke-api.ps1 -BaseUrl http://127.0.0.1 -User admin -Password admin
#>
param(
  [string]$BaseUrl = "http://127.0.0.1",
  [string]$User = "admin",
  [string]$Password = "admin",
  [string]$Bearer = ""
)

$ErrorActionPreference = "Stop"
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$failed = 0

function Invoke-Smoke {
  param(
    [string]$Name,
    [string]$Method,
    [string]$Path,
    [hashtable]$Headers = @{},
    [string]$Body = $null,
    [string]$ContentType = "application/json",
    [int[]]$Expect = @(200)
  )
  $uri = "$BaseUrl$Path"
  try {
    $params = @{
      Uri             = $uri
      Method          = $Method
      WebSession      = $session
      UseBasicParsing = $true
      TimeoutSec      = 30
      Headers         = $Headers
    }
    if ($null -ne $Body) {
      $params.Body = $Body
      $params.ContentType = $ContentType
    }
    $resp = Invoke-WebRequest @params
    if ($Expect -notcontains [int]$resp.StatusCode) {
      Write-Host "FAIL $Name -> $($resp.StatusCode) (want $($Expect -join ','))"
      $script:failed++
      return $null
    }
    Write-Host "OK   $Name -> $($resp.StatusCode)"
    return $resp
  } catch {
    $code = 0
    if ($_.Exception.Response) {
      $code = [int]$_.Exception.Response.StatusCode
    }
    if ($Expect -contains $code) {
      Write-Host "OK   $Name -> $code"
      return $null
    }
    Write-Host "FAIL $Name -> $($_.Exception.Message)"
    $script:failed++
    return $null
  }
}

Write-Host "Smoke against $BaseUrl"
Invoke-Smoke -Name "health" -Method GET -Path "/api/health" | Out-Null

$loginBody = (@{ username = $User; password = $Password } | ConvertTo-Json -Compress)
Invoke-Smoke -Name "login" -Method POST -Path "/api/auth/login" -Body $loginBody | Out-Null
Invoke-Smoke -Name "me" -Method GET -Path "/api/auth/me" | Out-Null

$csrf = ($session.Cookies.GetCookies($BaseUrl) | Where-Object { $_.Name -eq "nm_csrf" } | Select-Object -First 1).Value
if (-not $csrf) {
  Write-Host "WARN csrf cookie missing - mutating calls may fail"
}

$hdr = @{ "X-CSRF-Token" = "$csrf"; Origin = $BaseUrl }
if ($Bearer) { $hdr["Authorization"] = "Bearer $Bearer" }

Invoke-Smoke -Name "events" -Method GET -Path '/api/events?period=1h&group_by=country' | Out-Null

$csv = "Network,Country,Region,City,Latitude,Longitude`n1.2.3.0/24,RU,Moscow,Moscow,55.75,37.61`n"
Invoke-Smoke -Name "geo-dry-run" -Method POST -Path '/upload-geo?dry_run=1' -Headers $hdr -Body $csv -ContentType "text/csv" | Out-Null

Invoke-Smoke -Name "ingest" -Method POST -Path "/api/ingest" -Headers $hdr -Body "src=1.1.1.1 dst=8.8.8.8 action=allow`n" -ContentType "text/plain" | Out-Null

Invoke-Smoke -Name "maintenance" -Method POST -Path "/api/system/maintenance/backfill" -Headers $hdr -Expect @(202) | Out-Null

if ($failed -gt 0) {
  Write-Host "SMOKE FAILED ($failed)"
  exit 1
}
Write-Host "SMOKE PASSED"
exit 0
