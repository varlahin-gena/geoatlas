#Requires -Version 5.1
<#
.SYNOPSIS
  Load-test GeoAtlas syslog connectors from Windows (UDP/TCP).

.DESCRIPTION
  Sends CEF traffic in parallel to the three current connectors:
    - Default syslog  :514  both  -> TCP + UDP
    - Osnovnoy        :5515 udp
    - Zapasnoy        :5516 tcp

  Every event uses at least one IP from the fixed gray (enterprise) nets:
    10.0.0.0-10.0.255.254   (Moskva / Roga_i_kopyta)
    10.27.0.0-10.27.255.254  (Nizhniy Novgorod / Tyap_lyap)
    10.28.0.0-10.28.255.254  (Novosibirsk / Bez_problem)
    10.25.0.0-10.25.255.254  (Tyumen / Zelenoglazoe_taksi)
    10.30.0.0-10.30.255.254  (Omsk / Papa_Karlo)
  The other side is either another gray net or a public IPv4 sampled from GeoIP CSV.

.EXAMPLE
  .\load-test-connectors.ps1

.EXAMPLE
  .\load-test-connectors.ps1 -EpsPerSender 2000 -DurationSec 300

.EXAMPLE
  .\load-test-connectors.ps1 -SkipDefaultTcp -EpsPerSender 5000 -DurationSec 60

.EXAMPLE
  # Find capacity ceiling: ramp EPS until drops/queue pressure (needs API token)
  .\load-test-connectors.ps1 -FindThreshold -ApiToken $env:GA_API_TOKEN

.EXAMPLE
  .\load-test-connectors.ps1 -FindThreshold -StartEpsPerSender 250 -MaxEpsPerSender 8000 -StepEpsPerSender 250 -StepSec 45 -ApiToken '...'

.EXAMPLE
  # 1h steady soak at ~10k total EPS (2500 x 4 connectors), sample every 60s
  .\load-test-connectors.ps1 -Soak -EpsPerSender 2500 -DurationSec 3600 -WorkersPerConnector 4 -Batch 50 -ApiToken $token

.EXAMPLE
  .\load-test-connectors.ps1 -DryRun
#>
param(
  [string]$HostAddress = '138.16.225.160',
  [string]$GeoCsv = 'C:\Users\varla\Downloads\geoip_new.csv',
  [int]$EpsPerSender = 100,
  [int]$DurationSec = 60,
  [int]$Batch = 100,
  [int]$RangeSample = 2500,
  [int]$CsvMaxLines = 0,
  [int]$WorkersPerConnector = 1,   # raise to 2..4 to push past ~1.5k client EPS
  [int]$LinePoolSize = 4000,       # prebuilt lines (fast path; avoids PS string gen per event)
  [switch]$SkipDefaultTcp,
  [switch]$SkipDefaultUdp,
  [switch]$SkipMainUdp,
  [switch]$SkipBackupTcp,
  [switch]$DryRun,

  # --- steady soak (-Soak): constant EPS, continuous sockets, periodic samples ---
  [switch]$Soak,
  [int]$SampleSec = 60,
  [string]$SoakLog = '',

  # --- capacity ramp (-FindThreshold) ---
  [switch]$FindThreshold,
  [int]$StartEpsPerSender = 250,
  [int]$MaxEpsPerSender = 10000,
  [int]$StepEpsPerSender = 250,
  [int]$StepSec = 45,
  # 0 = no pause between steps (recommended for capacity tests — avoids cache/queue drain).
  [int]$SettleSec = 0,
  [string]$ThresholdLog = '',
  [string]$ApiBase = '',
  [string]$ApiToken = '',
  [double]$FailDropPerSec = 1,
  [double]$FailQueuePct = 75,
  [double]$FailLagSec = 60
)

$ErrorActionPreference = 'Stop'

$Connectors = @(
  @{ Name = 'Default syslog'; Id = 'default'; Port = 514;  Proto = 'tcp'; Skip = [bool]$SkipDefaultTcp }
  @{ Name = 'Default syslog'; Id = 'default'; Port = 514;  Proto = 'udp'; Skip = [bool]$SkipDefaultUdp }
  @{ Name = 'Osnovnoy';        Id = 'id_1515'; Port = 5515; Proto = 'udp'; Skip = [bool]$SkipMainUdp }
  @{ Name = 'Zapasnoy';        Id = 'id_5516'; Port = 5516; Proto = 'tcp'; Skip = [bool]$SkipBackupTcp }
) | Where-Object { -not $_.Skip }

if ($Connectors.Count -eq 0) { throw 'All connectors skipped via -Skip* flags' }
if ($FindThreshold -and $Soak) { throw 'Use either -FindThreshold or -Soak, not both' }
if ($EpsPerSender -lt 1) { throw 'EpsPerSender must be >= 1' }
if ($DurationSec -lt 1) { throw 'DurationSec must be >= 1' }
if ($Batch -lt 1) { throw 'Batch must be >= 1' }
if ($WorkersPerConnector -lt 1) { throw 'WorkersPerConnector must be >= 1' }
if ($LinePoolSize -lt 100) { throw 'LinePoolSize must be >= 100' }
if ($Soak -and $SampleSec -lt 5) { throw 'SampleSec must be >= 5' }

function ConvertTo-UInt32Ip([string]$ip) {
  $parts = $ip.Split('.') | ForEach-Object { [uint32]$_ }
  return ($parts[0] -shl 24) -bor ($parts[1] -shl 16) -bor ($parts[2] -shl 8) -bor $parts[3]
}
function ConvertFrom-UInt32Ip([uint32]$n) {
  return '{0}.{1}.{2}.{3}' -f (($n -shr 24) -band 255), (($n -shr 16) -band 255), (($n -shr 8) -band 255), ($n -band 255)
}

function New-RangeObj([string]$spec) {
  $a, $b = $spec -split '-', 2
  $start = ConvertTo-UInt32Ip $a.Trim()
  $end = ConvertTo-UInt32Ip $b.Trim()
  if ($end -lt $start) { return $null }
  $size = [uint64]($end - $start) + 1
  return [pscustomobject]@{
    Spec = $spec
    Start = $start
    End = $end
    Size = $size
    IsPrivate10 = $spec.StartsWith('10.')
  }
}

# Fixed gray/enterprise nets from the uploaded GeoIP (must be used in every event).
$GraySpecs = @(
  '10.0.0.0-10.0.255.254',    # Moskva / Roga_i_kopyta
  '10.27.0.0-10.27.255.254',  # Nizhniy Novgorod / Tyap_lyap
  '10.28.0.0-10.28.255.254',  # Novosibirsk / Bez_problem
  '10.25.0.0-10.25.255.254',  # Tyumen / Zelenoglazoe_taksi
  '10.30.0.0-10.30.255.254'   # Omsk / Papa_Karlo
)
$GrayRanges = @($GraySpecs | ForEach-Object { New-RangeObj $_ } | Where-Object { $_ })
if ($GrayRanges.Count -ne 5) { throw 'Failed to parse fixed gray ranges' }

function Read-GeoRangesSample {
  param(
    [string]$Path,
    [int]$SampleSize,
    [int]$MaxLines
  )
  if (-not (Test-Path -LiteralPath $Path)) {
    Write-Warning "GeoCSV not found: $Path - public peers will also be gray-to-gray only"
    return @()
  }

  $fi = Get-Item -LiteralPath $Path
  Write-Host ("reading public peers from GeoIP CSV: {0} ({1:N1} MiB), sample={2}" -f $fi.FullName, ($fi.Length / 1MB), $SampleSize)

  # Keep public IPv4 only (gray nets are fixed separately). Skip 10.x / IPv6.
  $headQuota = [Math]::Min($SampleSize, [Math]::Max(100, [int]($SampleSize * 0.25)))
  $estLines = if ($MaxLines -gt 0) { [double]$MaxLines } else {
    [Math]::Max(1000.0, $fi.Length / 90.0)
  }
  $remainSlots = [Math]::Max(1, $SampleSize - $headQuota)
  $stride = [Math]::Max(1, [int][Math]::Floor($estLines / [Math]::Max(1, $remainSlots * 3)))

  $sample = New-Object System.Collections.Generic.List[object]
  $seen = 0
  $kept = 0
  $phase = 'head'
  $reader = New-Object System.IO.StreamReader($Path, [Text.Encoding]::UTF8, $true, 1MB)
  try {
    $null = $reader.ReadLine()
    while ($true) {
      $line = $reader.ReadLine()
      if ($null -eq $line) { break }
      $seen++
      if ($MaxLines -gt 0 -and $seen -gt $MaxLines) { break }

      $comma = $line.IndexOf([char]',')
      if ($comma -gt 0) {
        $net = $line.Substring(0, $comma).Trim().Trim('"')
        if ($net -match '^\d+\.\d+\.\d+\.\d+-\d+\.\d+\.\d+\.\d+$' -and -not $net.StartsWith('10.')) {
          $obj = New-RangeObj $net
          if ($null -ne $obj) {
            $sample.Add($obj)
            $kept++
            if ($kept -ge $SampleSize) { break }
          }
        }
      }

      if ($phase -eq 'head') {
        if ($kept -ge $headQuota) { $phase = 'stride' }
        continue
      }

      for ($s = 1; $s -lt $stride; $s++) {
        if ($null -eq $reader.ReadLine()) { break }
        $seen++
        if ($MaxLines -gt 0 -and $seen -gt $MaxLines) { break }
      }
      if ($MaxLines -gt 0 -and $seen -gt $MaxLines) { break }
    }
  } finally {
    $reader.Close()
  }

  Write-Host ("  scanned~{0} lines, public peers={1} (headQuota={2}, stride={3})" -f $seen, $sample.Count, $headQuota, $stride)
  return $sample.ToArray()
}

$PublicRanges = @(Read-GeoRangesSample -Path $GeoCsv -SampleSize $RangeSample -MaxLines $CsvMaxLines)
Write-Host ("ranges: gray={0} public_peers={1}" -f $GrayRanges.Count, $PublicRanges.Count)
foreach ($g in $GrayRanges) { Write-Host ("  gray: {0}" -f $g.Spec) }

$Actions = @('accept','allow','deny','drop','close')
$Protos = @('TCP','UDP')
$DstPorts = @(443,80,53,22,8080,8443,25,3389,445,993)

# Expand logical connectors into parallel workers (rate split across workers).
$LogicalConnectors = @($Connectors)
$Connectors = foreach ($c in $LogicalConnectors) {
  for ($w = 1; $w -le $WorkersPerConnector; $w++) {
    [pscustomobject]@{
      Name = $c.Name
      Id = $c.Id
      Port = $c.Port
      Proto = $c.Proto
      Worker = $w
    }
  }
}
Write-Host ("workers: {0} logical x {1} = {2} senders" -f `
  $LogicalConnectors.Count, $WorkersPerConnector, $Connectors.Count)

function Get-RandomIpFromRange($range) {
  if ($range.Size -eq 1) { return ConvertFrom-UInt32Ip $range.Start }
  $max = [int64][Math]::Min([int64]$range.Size, [int64][int]::MaxValue)
  $offset = [uint64](Get-Random -Maximum $max)
  return ConvertFrom-UInt32Ip ([uint32]($range.Start + $offset))
}

# Always touch at least one gray net: gray->public | public->gray | gray->gray
function Get-GrayAwarePair {
  $mode = Get-Random -Maximum 3
  if ($PublicRanges.Count -eq 0) { $mode = 2 }
  switch ($mode) {
    0 { # gray -> public
      $srcRange = $GrayRanges[(Get-Random -Maximum $GrayRanges.Count)]
      $dstRange = $PublicRanges[(Get-Random -Maximum $PublicRanges.Count)]
    }
    1 { # public -> gray
      $srcRange = $PublicRanges[(Get-Random -Maximum $PublicRanges.Count)]
      $dstRange = $GrayRanges[(Get-Random -Maximum $GrayRanges.Count)]
    }
    default { # gray -> gray (different nets when possible)
      $srcRange = $GrayRanges[(Get-Random -Maximum $GrayRanges.Count)]
      do { $dstRange = $GrayRanges[(Get-Random -Maximum $GrayRanges.Count)] } while ($dstRange.Spec -eq $srcRange.Spec -and $GrayRanges.Count -gt 1)
    }
  }
  return [pscustomobject]@{
    Src = (Get-RandomIpFromRange $srcRange)
    Dst = (Get-RandomIpFromRange $dstRange)
  }
}

function New-CefLine {
  $p = Get-GrayAwarePair
  $src = $p.Src
  $dst = $p.Dst
  $spt = Get-Random -Minimum 10000 -Maximum 60000
  $dpt = $DstPorts[(Get-Random -Maximum $DstPorts.Count)]
  $act = $Actions[(Get-Random -Maximum $Actions.Count)]
  $proto = $Protos[(Get-Random -Maximum $Protos.Count)]
  $inB = Get-Random -Minimum 40 -Maximum 5000
  $outB = Get-Random -Minimum 20 -Maximum 2000
  # Short kv form — enough for parser; much faster than full CEF templates.
  return "src=$src dst=$dst spt=$spt dpt=$dpt act=$act proto=$proto rule=LoadTest out=$outB in=$inB cn1=1 cn2=1"
}

Write-Host ("building line pool: {0}..." -f $LinePoolSize)
$LinePool = New-Object string[] $LinePoolSize
for ($i = 0; $i -lt $LinePoolSize; $i++) {
  $LinePool[$i] = New-CefLine
}
Write-Host '  line pool ready'

if (-not $ApiBase) { $ApiBase = "http://$HostAddress" }

if ($DryRun) {
  Write-Host '== dry-run =='
  if ($FindThreshold) {
    Write-Host ("mode=FindThreshold start={0} max={1} step={2} stepSec={3} logical={4} workers={5}" -f `
      $StartEpsPerSender, $MaxEpsPerSender, $StepEpsPerSender, $StepSec, $LogicalConnectors.Count, $WorkersPerConnector)
    Write-Host ("  total EPS range: {0} .. {1}" -f `
      ($StartEpsPerSender * $LogicalConnectors.Count), ($MaxEpsPerSender * $LogicalConnectors.Count))
    Write-Host ("  api={0} token={1}" -f $ApiBase, $(if ($ApiToken) { 'set' } else { 'MISSING (client-only ramp)' }))
  } elseif ($Soak) {
    Write-Host ("mode=Soak duration={0}s ({1:N1}h) eps/logical={2} total~{3} sample={4}s workers={5}" -f `
      $DurationSec, ($DurationSec / 3600.0), $EpsPerSender,
      ($EpsPerSender * $LogicalConnectors.Count), $SampleSec, $WorkersPerConnector)
  } else {
    Write-Host ("host={0} duration={1}s eps/logical={2} logical={3} total~{4} EPS" -f `
      $HostAddress, $DurationSec, $EpsPerSender, $LogicalConnectors.Count, ($EpsPerSender * $LogicalConnectors.Count))
  }
  $LogicalConnectors | ForEach-Object {
    Write-Host ("  - {0} ({1}) :{2}/{3}" -f $_.Name, $_.Id, $_.Port, $_.Proto)
  }
  Write-Host '--- sample lines ---'
  1..3 | ForEach-Object { New-CefLine }
  exit 0
}

$senderScript = {
  param(
    [string]$HostAddress,
    [int]$Port,
    [string]$Proto,
    [string]$Name,
    [string]$Id,
    [int]$Eps,
    [int]$DurationSec,
    [int]$Batch,
    [string[]]$LinePool,
    [hashtable]$Control  # continuous ramp: EpsWorker, Stop, Sent (synced)
  )

  $ErrorActionPreference = 'Stop'
  $sent = 0L
  $errors = 0L
  $label = '{0}:{1}/{2}' -f $HostAddress, $Port, $Proto
  $poolN = $LinePool.Length
  $idx = Get-Random -Maximum $poolN
  $sw = [Diagnostics.Stopwatch]::StartNew()
  $continuous = $null -ne $Control
  $deadlineMs = if ($continuous) { [long]::MaxValue } else { $DurationSec * 1000L }
  $epsNow = if ($continuous) { [Math]::Max(1, [int]$Control['EpsWorker']) } else { [Math]::Max(1, $Eps) }
  $batchIntervalMs = ($Batch / [double]$epsNow) * 1000.0
  $nextDueMs = 0.0

  try {
    if ($Proto -eq 'tcp') {
      $client = New-Object System.Net.Sockets.TcpClient
      $client.NoDelay = $true
      $client.SendBufferSize = 1024 * 1024
      $client.Connect($HostAddress, $Port)
      $stream = $client.GetStream()
      $writer = New-Object System.IO.StreamWriter($stream, [Text.Encoding]::UTF8, 256KB)
      $writer.NewLine = "`n"
      $writer.AutoFlush = $false
      try {
        while ($true) {
          if ($continuous) {
            if ([bool]$Control['Stop']) { break }
            $newEps = [Math]::Max(1, [int]$Control['EpsWorker'])
            if ($newEps -ne $epsNow) {
              $epsNow = $newEps
              $batchIntervalMs = ($Batch / [double]$epsNow) * 1000.0
              $nextDueMs = [double]$sw.ElapsedMilliseconds
            }
          } elseif ($sw.ElapsedMilliseconds -ge $deadlineMs) {
            break
          }

          for ($i = 0; $i -lt $Batch; $i++) {
            $writer.WriteLine($LinePool[$idx])
            $idx++
            if ($idx -ge $poolN) { $idx = 0 }
          }
          $writer.Flush()
          $sent += $Batch
          if ($continuous) {
            [System.Threading.Monitor]::Enter($Control)
            try { $Control['Sent'] = [long]$Control['Sent'] + $Batch }
            finally { [System.Threading.Monitor]::Exit($Control) }
          }
          $nextDueMs += $batchIntervalMs
          $wait = [int]($nextDueMs - $sw.ElapsedMilliseconds)
          if ($wait -gt 1) { Start-Sleep -Milliseconds $wait }
        }
      } finally {
        $writer.Flush(); $writer.Dispose(); $stream.Dispose(); $client.Close()
      }
    } else {
      $udp = New-Object System.Net.Sockets.UdpClient
      $udp.Client.SendBufferSize = 1024 * 1024
      $udp.Connect($HostAddress, $Port)
      $enc = [Text.Encoding]::UTF8
      $bytePool = New-Object 'object[]' $poolN
      for ($b = 0; $b -lt $poolN; $b++) {
        $bytePool[$b] = $enc.GetBytes($LinePool[$b] + "`n")
      }
      try {
        while ($true) {
          if ($continuous) {
            if ([bool]$Control['Stop']) { break }
            $newEps = [Math]::Max(1, [int]$Control['EpsWorker'])
            if ($newEps -ne $epsNow) {
              $epsNow = $newEps
              $batchIntervalMs = ($Batch / [double]$epsNow) * 1000.0
              $nextDueMs = [double]$sw.ElapsedMilliseconds
            }
          } elseif ($sw.ElapsedMilliseconds -ge $deadlineMs) {
            break
          }

          for ($i = 0; $i -lt $Batch; $i++) {
            $bytes = $bytePool[$idx]
            [void]$udp.Send($bytes, $bytes.Length)
            $idx++
            if ($idx -ge $poolN) { $idx = 0 }
          }
          $sent += $Batch
          if ($continuous) {
            [System.Threading.Monitor]::Enter($Control)
            try { $Control['Sent'] = [long]$Control['Sent'] + $Batch }
            finally { [System.Threading.Monitor]::Exit($Control) }
          }
          $nextDueMs += $batchIntervalMs
          $wait = [int]($nextDueMs - $sw.ElapsedMilliseconds)
          if ($wait -gt 1) { Start-Sleep -Milliseconds $wait }
        }
      } finally {
        $udp.Close()
      }
    }
  } catch {
    $errors++
    return [pscustomobject]@{
      Name = $Name; Id = $Id; Port = $Port; Proto = $Proto
      Sent = $sent; Errors = $errors; ElapsedSec = $sw.Elapsed.TotalSeconds
      EpsActual = 0; Error = $_.Exception.Message; Target = $label
    }
  }

  $sw.Stop()
  $epsActual = if ($sw.Elapsed.TotalSeconds -gt 0) { [math]::Round($sent / $sw.Elapsed.TotalSeconds, 1) } else { 0 }
  return [pscustomobject]@{
    Name = $Name; Id = $Id; Port = $Port; Proto = $Proto
    Sent = $sent; Errors = $errors; ElapsedSec = [math]::Round($sw.Elapsed.TotalSeconds, 1)
    EpsActual = $epsActual; Error = ''; Target = $label
  }
}

function Invoke-ConnectorBurst {
  param(
    [int]$EpsPerLogical,   # target EPS per logical connector (tcp/udp endpoint)
    [int]$Seconds
  )
  # Split logical rate across WorkersPerConnector.
  $epsWorker = [Math]::Max(1, [int][Math]::Ceiling($EpsPerLogical / [double]$WorkersPerConnector))
  $pool = [RunspaceFactory]::CreateRunspacePool(1, [Math]::Max(4, $Connectors.Count))
  $pool.Open()
  $jobs = @()
  foreach ($c in $Connectors) {
    $ps = [PowerShell]::Create()
    $ps.RunspacePool = $pool
    [void]$ps.AddScript($senderScript).AddParameters(@{
      HostAddress = $HostAddress
      Port = [int]$c.Port
      Proto = [string]$c.Proto
      Name = [string]$c.Name
      Id = [string]$c.Id
      Eps = $epsWorker
      DurationSec = $Seconds
      Batch = $Batch
      LinePool = $LinePool
      Control = $null
    })
    $jobs += [pscustomobject]@{ PS = $ps; Handle = $ps.BeginInvoke(); Meta = $c }
  }
  $results = @()
  foreach ($j in $jobs) {
    try {
      $out = $j.PS.EndInvoke($j.Handle)
      if ($out) { $results += $out }
    } catch {
      $results += [pscustomobject]@{
        Name = $j.Meta.Name; Id = $j.Meta.Id; Port = $j.Meta.Port; Proto = $j.Meta.Proto
        Sent = 0; Errors = 1; ElapsedSec = 0; EpsActual = 0
        Error = $_.Exception.Message; Target = ('{0}:{1}/{2}' -f $HostAddress, $j.Meta.Port, $j.Meta.Proto)
      }
    } finally {
      $j.PS.Dispose()
    }
  }
  $pool.Close(); $pool.Dispose()
  return $results
}

function Get-IngestStatsSnapshot {
  param([int]$TimeoutSec = 20, [int]$Retries = 3)
  if (-not $ApiToken) { return $null }
  $url = ($ApiBase.TrimEnd('/') + '/api/ingest/stats')
  for ($attempt = 1; $attempt -le $Retries; $attempt++) {
    try {
      $resp = Invoke-WebRequest -Uri $url -Headers @{ Authorization = "Bearer $ApiToken" } `
        -TimeoutSec $TimeoutSec -UseBasicParsing
      return ($resp.Content | ConvertFrom-Json)
    } catch {
      $msg = $_.Exception.Message
      $code = $null
      if ($_.Exception.Response) {
        try { $code = [int]$_.Exception.Response.StatusCode } catch { }
      }
      if ($code -eq 401 -or $code -eq 403) {
        Write-Warning ("ingest/stats HTTP {0}: bad/expired token (not a timeout)" -f $code)
        return $null
      }
      Write-Warning ("ingest/stats attempt {0}/{1} failed: {2}" -f $attempt, $Retries, $msg)
      if ($attempt -lt $Retries) { Start-Sleep -Seconds (2 * $attempt) }
    }
  }
  return $null
}

function Show-BurstSummary {
  param($results, [string]$Title)
  $totalSent = ($results | Measure-Object -Property Sent -Sum).Sum
  $totalErr = ($results | Measure-Object -Property Errors -Sum).Sum
  $avgElapsed = ($results | Measure-Object -Property ElapsedSec -Average).Average
  $aggEps = if ($avgElapsed -gt 0) { [math]::Round($totalSent / $avgElapsed, 1) } else { 0 }
  Write-Host ("{0}: sent={1} errors={2} ~{3} EPS aggregate" -f $Title, $totalSent, $totalErr, $aggEps)
  return [pscustomobject]@{ Sent = $totalSent; Errors = $totalErr; EpsActual = $aggEps; ElapsedSec = $avgElapsed }
}

# --------------- Soak: constant EPS, continuous sockets, timed samples ---------------
if ($Soak) {
  $nLogical = $LogicalConnectors.Count
  $totalTarget = $EpsPerSender * $nLogical
  $epsWorker = [Math]::Max(1, [int][Math]::Ceiling($EpsPerSender / [double]$WorkersPerConnector))
  if (-not $SoakLog) {
    $SoakLog = Join-Path $PSScriptRoot ("soak-{0:yyyyMMdd-HHmmss}.csv" -f (Get-Date))
  }

  Write-Host ''
  Write-Host '== SOAK (steady EPS) =='
  Write-Host ("host={0} duration={1}s ({2:N2}h) total~{3} EPS ({4}/logical x {5})" -f `
    $HostAddress, $DurationSec, ($DurationSec / 3600.0), $totalTarget, $EpsPerSender, $nLogical)
  Write-Host ("threads={0} (workers/conn={1}) batch={2} sample={3}s" -f `
    $Connectors.Count, $WorkersPerConnector, $Batch, $SampleSec)
  Write-Host ("log={0}" -f $SoakLog)
  Write-Host 'MODE: fixed rate, sockets stay open for the whole soak (stable stream).'
  foreach ($c in $LogicalConnectors) {
    Write-Host ("  -> {0} ({1}) :{2}/{3}" -f $c.Name, $c.Id, $c.Port, $c.Proto)
  }

  if ($ApiToken) {
    Write-Host ("probing {0}/api/ingest/stats ..." -f $ApiBase.TrimEnd('/'))
    $probe = Get-IngestStatsSnapshot -TimeoutSec 30 -Retries 4
    if ($null -eq $probe) {
      Write-Warning 'API stats unreachable at start - continuing; samples may lack server metrics.'
      $ApiToken = ''
    } else {
      Write-Host ("api ok: state={0} received={1} dropped={2}" -f `
        $probe.state, $probe.received_total, $probe.dropped_total)
    }
  } else {
    Write-Warning 'No -ApiToken: soak will log client EPS only. Pass token for recv/drop/queue.'
  }

  $control = [hashtable]::Synchronized(@{
    EpsWorker = $epsWorker
    Stop = $false
    Sent = [long]0
  })
  $rsPool = [RunspaceFactory]::CreateRunspacePool(1, [Math]::Max(4, $Connectors.Count))
  $rsPool.Open()
  $jobs = @()
  foreach ($c in $Connectors) {
    $ps = [PowerShell]::Create()
    $ps.RunspacePool = $rsPool
    [void]$ps.AddScript($senderScript).AddParameters(@{
      HostAddress = $HostAddress
      Port = [int]$c.Port
      Proto = [string]$c.Proto
      Name = [string]$c.Name
      Id = [string]$c.Id
      Eps = $epsWorker
      DurationSec = 0
      Batch = $Batch
      LinePool = $LinePool
      Control = $control
    })
    $jobs += [pscustomobject]@{ PS = $ps; Handle = $ps.BeginInvoke(); Meta = $c }
  }

  'utc,elapsed_s,client_eps,recv_eps,drop_eps,queue_pct,lag_s,auth_delta,received_total,dropped_total,queue_depth,verdict' |
    Set-Content -LiteralPath $SoakLog -Encoding UTF8

  Write-Host ("soak workers started: {0}; warming 3s..." -f $jobs.Count)
  Start-Sleep -Seconds 3

  $samples = New-Object System.Collections.Generic.List[object]
  $soakSw = [Diagnostics.Stopwatch]::StartNew()
  $prevSnap = Get-IngestStatsSnapshot -TimeoutSec 15 -Retries 2
  [System.Threading.Monitor]::Enter($control)
  try { $prevSent = [long]$control['Sent'] } finally { [System.Threading.Monitor]::Exit($control) }
  $prevT = $soakSw.Elapsed.TotalSeconds
  $failHits = 0

  try {
    while ($soakSw.Elapsed.TotalSeconds -lt $DurationSec) {
      $left = $DurationSec - $soakSw.Elapsed.TotalSeconds
      $sleepFor = [Math]::Min($SampleSec, [Math]::Max(1, [int][Math]::Ceiling($left)))
      Start-Sleep -Seconds $sleepFor

      $nowT = $soakSw.Elapsed.TotalSeconds
      $dt = [Math]::Max(0.001, $nowT - $prevT)
      [System.Threading.Monitor]::Enter($control)
      try { $curSent = [long]$control['Sent'] } finally { [System.Threading.Monitor]::Exit($control) }
      $clientEps = [math]::Round(($curSent - $prevSent) / $dt, 1)
      $snap = Get-IngestStatsSnapshot -TimeoutSec 15 -Retries 2

      $recvEps = $null; $dropEps = $null; $queuePct = $null; $lag = $null; $authDelta = $null
      $recvTotal = $null; $dropTotal = $null; $qDepth = $null
      $verdict = 'ok'; $reason = ''

      if ($null -ne $prevSnap -and $null -ne $snap) {
        $recvEps = [math]::Round(([double]$snap.received_total - [double]$prevSnap.received_total) / $dt, 1)
        $dropDelta = [double]$snap.dropped_total - [double]$prevSnap.dropped_total
        $bufDelta = 0.0
        if ($null -ne $snap.buffer_drops_total -and $null -ne $prevSnap.buffer_drops_total) {
          $bufDelta = [double]$snap.buffer_drops_total - [double]$prevSnap.buffer_drops_total
        }
        $dropEps = [math]::Round(($dropDelta + $bufDelta) / $dt, 2)
        $recvTotal = $snap.received_total
        $dropTotal = $snap.dropped_total
        $qDepth = $snap.queue_depth
        $qCap = [double]$snap.queue_capacity
        if ($qCap -gt 0) { $queuePct = [math]::Round(100.0 * [double]$qDepth / $qCap, 1) }
        if ($recvEps -gt 0) {
          $backlog = [double]$snap.received_total - [double]$snap.inserted_total
          if ($backlog -lt 0) { $backlog = 0 }
          $lag = [math]::Round($backlog / $recvEps, 1)
        }
        if ($null -ne $snap.auth_rejected_total -and $null -ne $prevSnap.auth_rejected_total) {
          $authDelta = [double]$snap.auth_rejected_total - [double]$prevSnap.auth_rejected_total
        }
        if ($dropEps -gt $FailDropPerSec) { $verdict = 'warn'; $reason = ("drop/s={0}" -f $dropEps); $failHits++ }
        elseif ($null -ne $queuePct -and $queuePct -ge $FailQueuePct) { $verdict = 'warn'; $reason = ("queue={0}%" -f $queuePct); $failHits++ }
        elseif ($null -ne $lag -and $lag -gt $FailLagSec) { $verdict = 'warn'; $reason = ("lag={0}s" -f $lag); $failHits++ }
        else { $failHits = 0 }
      }

      $row = [pscustomobject]@{
        ElapsedSec = [math]::Round($nowT, 1)
        ClientEps = $clientEps
        RecvEps = $recvEps
        DropEps = $dropEps
        QueuePct = $queuePct
        LagSec = $lag
        Verdict = $verdict
        Reason = $reason
      }
      $samples.Add($row)

      # InvariantCulture: RU locale uses ',' as decimal → breaks CSV columns
      $inv = [System.Globalization.CultureInfo]::InvariantCulture
      $n1 = { param($v) if ($null -eq $v) { '' } else { ([double]$v).ToString('0.0', $inv) } }
      $n2 = { param($v) if ($null -eq $v) { '' } else { ([double]$v).ToString('0.00', $inv) } }
      $n0 = { param($v) if ($null -eq $v) { '' } else { ([double]$v).ToString('0', $inv) } }
      $line = '{0},{1},{2},{3},{4},{5},{6},{7},{8},{9},{10},{11}' -f @(
        [DateTime]::UtcNow.ToString('o', $inv),
        (& $n1 $row.ElapsedSec),
        (& $n1 $clientEps),
        (& $n1 $recvEps),
        (& $n2 $dropEps),
        (& $n1 $queuePct),
        (& $n1 $lag),
        (& $n0 $authDelta),
        $(if ($null -ne $recvTotal) { ([long]$recvTotal).ToString($inv) } else { '' }),
        $(if ($null -ne $dropTotal) { ([long]$dropTotal).ToString($inv) } else { '' }),
        $(if ($null -ne $qDepth) { ([long]$qDepth).ToString($inv) } else { '' }),
        $verdict
      )
      Add-Content -LiteralPath $SoakLog -Value $line -Encoding UTF8

      Write-Host ("  t={0,6:N0}s client={1,8} recv={2,8} drop={3,7} queue={4,6}% lag={5,5} => {6} {7}" -f `
        $nowT, $clientEps,
        $(if ($null -ne $recvEps) { $recvEps } else { '?' }),
        $(if ($null -ne $dropEps) { $dropEps } else { '?' }),
        $(if ($null -ne $queuePct) { $queuePct } else { '?' }),
        $(if ($null -ne $lag) { $lag } else { '?' }),
        $verdict, $reason)

      $prevSnap = $snap
      $prevSent = $curSent
      $prevT = $nowT
    }
  } finally {
    $control['Stop'] = $true
    foreach ($j in $jobs) {
      try { [void]$j.PS.EndInvoke($j.Handle) } catch { }
      finally { $j.PS.Dispose() }
    }
    $rsPool.Close(); $rsPool.Dispose()
    Write-Host 'soak workers stopped'
  }

  Write-Host ''
  Write-Host '== soak summary =='
  $avgClient = ($samples | Measure-Object -Property ClientEps -Average).Average
  $minClient = ($samples | Measure-Object -Property ClientEps -Minimum).Minimum
  $maxClient = ($samples | Measure-Object -Property ClientEps -Maximum).Maximum
  $recvSamples = @($samples | Where-Object { $null -ne $_.RecvEps })
  Write-Host ("target total EPS : {0}" -f $totalTarget)
  Write-Host ("client EPS       : avg={0:N1} min={1:N1} max={2:N1} (jitter={3:N1})" -f `
    $avgClient, $minClient, $maxClient, ($maxClient - $minClient))
  if ($recvSamples.Count -gt 0) {
    $avgRecv = ($recvSamples | Measure-Object -Property RecvEps -Average).Average
    $minRecv = ($recvSamples | Measure-Object -Property RecvEps -Minimum).Minimum
    $maxRecv = ($recvSamples | Measure-Object -Property RecvEps -Maximum).Maximum
    $avgDrop = ($recvSamples | Where-Object { $null -ne $_.DropEps } | Measure-Object -Property DropEps -Average).Average
    $maxQueue = ($recvSamples | Where-Object { $null -ne $_.QueuePct } | Measure-Object -Property QueuePct -Maximum).Maximum
    $warnN = @($samples | Where-Object { $_.Verdict -ne 'ok' }).Count
    Write-Host ("server recv EPS  : avg={0:N1} min={1:N1} max={2:N1}" -f $avgRecv, $minRecv, $maxRecv)
    Write-Host ("drop/s avg       : {0:N2}   max queue%={1}" -f $avgDrop, $maxQueue)
    Write-Host ("warn samples     : {0} / {1}" -f $warnN, $samples.Count)
  }
  Write-Host ("samples          : {0}  log={1}" -f $samples.Count, $SoakLog)
  Write-Host 'Watch UI /system charts over the same window for CPU/lag correlation.'
  exit 0
}

# --------------- FindThreshold: stepped ramp until SLO breach ---------------
if ($FindThreshold) {
  if ($StartEpsPerSender -lt 1) { throw 'StartEpsPerSender must be >= 1' }
  if ($MaxEpsPerSender -lt $StartEpsPerSender) { throw 'MaxEpsPerSender must be >= StartEpsPerSender' }
  if ($StepEpsPerSender -lt 1) { throw 'StepEpsPerSender must be >= 1' }
  if ($StepSec -lt 5) { throw 'StepSec must be >= 5' }
  if (-not $ThresholdLog) {
    $ThresholdLog = Join-Path $PSScriptRoot ("threshold-{0:yyyyMMdd-HHmmss}.csv" -f (Get-Date))
  }
  $inv = [System.Globalization.CultureInfo]::InvariantCulture

  $nLogical = $LogicalConnectors.Count
  Write-Host ''
  Write-Host '== FIND THRESHOLD (capacity ramp) =='
  Write-Host ("host={0} logical={1} workers/conn={2} threads={3} step={4}s settle={5}s" -f `
    $HostAddress, $nLogical, $WorkersPerConnector, $Connectors.Count, $StepSec, $SettleSec)
  Write-Host ("eps/logical: {0} .. {1} step {2}  => total ~{3} .. {4} EPS" -f `
    $StartEpsPerSender, $MaxEpsPerSender, $StepEpsPerSender,
    ($StartEpsPerSender * $nLogical), ($MaxEpsPerSender * $nLogical))
  Write-Host ("fail if: drop/s>{0} OR queue>={1}% OR lag>{2}s" -f $FailDropPerSec, $FailQueuePct, $FailLagSec)
  Write-Host ("log={0}" -f $ThresholdLog)
  Write-Host 'MODE: continuous ramp (connections stay open; rate changes in-place, no settle/reconnect gap).'
  Write-Host 'NOTE: if client EPS plateaus with drop/s=0, that is CLIENT limit (not server).'
  foreach ($c in $LogicalConnectors) {
    Write-Host ("  -> {0} ({1}) :{2}/{3}" -f $c.Name, $c.Id, $c.Port, $c.Proto)
  }
  'utc,step,eps_per_logical,target_eps,client_eps,recv_eps,drop_eps,queue_pct,lag_s,auth_delta,received_total,dropped_total,queue_depth,verdict,reason' |
    Set-Content -LiteralPath $ThresholdLog -Encoding UTF8
  if (-not $ApiToken) {
    Write-Warning 'No -ApiToken: cannot auto-detect drops. Watch UI /system Pipeline; script ramps blindly.'
    Write-Host '  Token: docker exec backend printenv API_AUTH_TOKEN'
  } else {
    Write-Host ("probing {0}/api/ingest/stats ..." -f $ApiBase.TrimEnd('/'))
    $probe = Get-IngestStatsSnapshot -TimeoutSec 30 -Retries 4
    if ($null -eq $probe) {
      Write-Warning @"
API stats unreachable (timeout or overload) - NOT necessarily a bad token.
Continuing ramp WITHOUT auto drop detection. Watch UI /system Pipeline.
"@
      $ApiToken = ''
    } else {
      Write-Host ("api ok: state={0} received={1} dropped={2}" -f `
        $probe.state, $probe.received_total, $probe.dropped_total)
    }
  }
  Write-Host ''

  # Continuous control plane: workers keep sockets open and only change pace.
  $eps = $StartEpsPerSender
  $epsWorker0 = [Math]::Max(1, [int][Math]::Ceiling($eps / [double]$WorkersPerConnector))
  $control = [hashtable]::Synchronized(@{
    EpsWorker = $epsWorker0
    Stop = $false
    Sent = [long]0
  })
  $rsPool = [RunspaceFactory]::CreateRunspacePool(1, [Math]::Max(4, $Connectors.Count))
  $rsPool.Open()
  $jobs = @()
  foreach ($c in $Connectors) {
    $ps = [PowerShell]::Create()
    $ps.RunspacePool = $rsPool
    [void]$ps.AddScript($senderScript).AddParameters(@{
      HostAddress = $HostAddress
      Port = [int]$c.Port
      Proto = [string]$c.Proto
      Name = [string]$c.Name
      Id = [string]$c.Id
      Eps = $epsWorker0
      DurationSec = 0
      Batch = $Batch
      LinePool = $LinePool
      Control = $control
    })
    $jobs += [pscustomobject]@{ PS = $ps; Handle = $ps.BeginInvoke(); Meta = $c }
  }
  Write-Host ("continuous workers started: {0} threads (no reconnect between steps)" -f $jobs.Count)
  # brief warm-up so sockets are up before first sample
  Start-Sleep -Seconds 2

  $steps = New-Object System.Collections.Generic.List[object]
  $lastGood = $null
  $firstBad = $null
  $clientPlateauHits = 0

  try {
    while ($eps -le $MaxEpsPerSender) {
      $totalTarget = $eps * $nLogical
      $epsWorker = [Math]::Max(1, [int][Math]::Ceiling($eps / [double]$WorkersPerConnector))
      $control['EpsWorker'] = $epsWorker
      Write-Host ("---- step eps/logical={0} total~{1} EPS for {2}s (continuous) ----" -f $eps, $totalTarget, $StepSec)

      $before = Get-IngestStatsSnapshot
      [System.Threading.Monitor]::Enter($control)
      try { $sent0 = [long]$control['Sent'] } finally { [System.Threading.Monitor]::Exit($control) }
      $t0 = [Diagnostics.Stopwatch]::StartNew()

      Start-Sleep -Seconds $StepSec
      # optional settle ONLY if explicitly requested (default 0)
      if ($SettleSec -gt 0) {
        Write-Host ("  settle {0}s (drains queues - not recommended for ceiling tests)..." -f $SettleSec)
        Start-Sleep -Seconds $SettleSec
      }

      [System.Threading.Monitor]::Enter($control)
      try { $sent1 = [long]$control['Sent'] } finally { [System.Threading.Monitor]::Exit($control) }
      $after = Get-IngestStatsSnapshot
      $dt = [Math]::Max(0.001, $t0.Elapsed.TotalSeconds)
      $clientEps = [math]::Round(($sent1 - $sent0) / $dt, 1)
      $client = [pscustomobject]@{ EpsActual = $clientEps; Sent = ($sent1 - $sent0); Errors = 0 }
      Write-Host ("  client: sent={0} ~{1} EPS aggregate" -f $client.Sent, $client.EpsActual)

      $dropPerSec = $null
      $recvPerSec = $null
      $queuePct = $null
      $lag = $null
      $authDelta = $null
      $verdict = 'ok'
      $reason = ''
      $clientLimited = ($client.EpsActual -lt ($totalTarget * 0.5)) -and ($totalTarget -ge 500)

      if ($null -ne $before -and $null -ne $after) {
        $dropDelta = [double]$after.dropped_total - [double]$before.dropped_total
        $bufDropDelta = 0.0
        if ($null -ne $after.buffer_drops_total -and $null -ne $before.buffer_drops_total) {
          $bufDropDelta = [double]$after.buffer_drops_total - [double]$before.buffer_drops_total
        }
        $recvDelta = [double]$after.received_total - [double]$before.received_total
        if ($null -ne $after.auth_rejected_total -and $null -ne $before.auth_rejected_total) {
          $authDelta = [double]$after.auth_rejected_total - [double]$before.auth_rejected_total
        }
        $dropPerSec = [math]::Round(($dropDelta + $bufDropDelta) / $dt, 2)
        $recvPerSec = [math]::Round($recvDelta / $dt, 1)
        $qDepth = [double]($after.queue_depth)
        $qCap = [double]($after.queue_capacity)
        if ($qCap -gt 0) { $queuePct = [math]::Round(100.0 * $qDepth / $qCap, 1) }
        if ($recvPerSec -gt 0) {
          $backlog = [double]$after.received_total - [double]$after.inserted_total
          if ($backlog -lt 0) { $backlog = 0 }
          $lag = [math]::Round($backlog / $recvPerSec, 1)
        }
        $circuitOpen = $false
        if ($null -ne $after.circuit_open) { $circuitOpen = [bool]$after.circuit_open }

        $serverStressed = ($dropPerSec -gt $FailDropPerSec) -or `
          ($null -ne $queuePct -and $queuePct -ge $FailQueuePct) -or `
          ($null -ne $lag -and $lag -gt $FailLagSec) -or `
          $circuitOpen

        if ($authDelta -gt 100) {
          $verdict = 'fail'; $reason = ("auth_rejected +{0}" -f $authDelta)
        } elseif ($serverStressed) {
          if ($circuitOpen) { $verdict = 'fail'; $reason = 'circuit_open' }
          elseif ($dropPerSec -gt $FailDropPerSec) { $verdict = 'fail'; $reason = ("drop/s={0}" -f $dropPerSec) }
          elseif ($null -ne $queuePct -and $queuePct -ge $FailQueuePct) { $verdict = 'fail'; $reason = ("queue={0}%" -f $queuePct) }
          else { $verdict = 'fail'; $reason = ("lag={0}s" -f $lag) }
        } elseif ($clientLimited) {
          $clientPlateauHits++
          if ($clientPlateauHits -ge 2) {
            $verdict = 'client_limit'
            $reason = ("client~{0} EPS << target {1} (server healthy)" -f $client.EpsActual, $totalTarget)
          } else {
            $verdict = 'ok'
            $reason = 'client catching up'
          }
        }

        Write-Host ("  server: recv/s={0} drop/s={1} queue={2}% lag={3}s auth_delta={4} => {5} {6}" -f `
          $recvPerSec, $dropPerSec,
          $(if ($null -ne $queuePct) { $queuePct } else { '?' }),
          $(if ($null -ne $lag) { $lag } else { '?' }),
          $(if ($null -ne $authDelta) { $authDelta } else { '?' }),
          $verdict, $reason)
      } else {
        if ($clientLimited) {
          $clientPlateauHits++
          $verdict = $(if ($clientPlateauHits -ge 2) { 'client_limit' } else { 'ok' })
          $reason = ("client~{0} << target {1}" -f $client.EpsActual, $totalTarget)
        }
        Write-Host ("  server: (no stats) => {0} {1}" -f $verdict, $reason)
      }

      $recvTotal = $null; $dropTotal = $null; $qDepthOut = $null
      if ($null -ne $after) {
        $recvTotal = $after.received_total
        $dropTotal = $after.dropped_total
        $qDepthOut = $after.queue_depth
      }
      $row = [pscustomobject]@{
        EpsPerLogical = $eps
        TotalTargetEps = $totalTarget
        ClientEps = $client.EpsActual
        RecvPerSec = $recvPerSec
        DropPerSec = $dropPerSec
        QueuePct = $queuePct
        LagSec = $lag
        Verdict = $verdict
        Reason = $reason
      }
      $steps.Add($row)

      $n1 = { param($v) if ($null -eq $v) { '' } else { ([double]$v).ToString('0.0', $inv) } }
      $n2 = { param($v) if ($null -eq $v) { '' } else { ([double]$v).ToString('0.00', $inv) } }
      $csvReason = ($reason -replace ',', ';')
      $csvLine = '{0},{1},{2},{3},{4},{5},{6},{7},{8},{9},{10},{11},{12},{13},{14}' -f @(
        [DateTime]::UtcNow.ToString('o', $inv),
        $steps.Count,
        $eps,
        $totalTarget,
        (& $n1 $client.EpsActual),
        (& $n1 $recvPerSec),
        (& $n2 $dropPerSec),
        (& $n1 $queuePct),
        (& $n1 $lag),
        (& $n1 $authDelta),
        $(if ($null -ne $recvTotal) { ([long]$recvTotal).ToString($inv) } else { '' }),
        $(if ($null -ne $dropTotal) { ([long]$dropTotal).ToString($inv) } else { '' }),
        $(if ($null -ne $qDepthOut) { ([long]$qDepthOut).ToString($inv) } else { '' }),
        $verdict,
        $csvReason
      )
      Add-Content -LiteralPath $ThresholdLog -Value $csvLine -Encoding UTF8

      if ($verdict -eq 'ok') {
        $lastGood = $row
        $eps += $StepEpsPerSender
        continue
      }
      $firstBad = $row
      break
    }
  } finally {
    $control['Stop'] = $true
    foreach ($j in $jobs) {
      try { [void]$j.PS.EndInvoke($j.Handle) } catch { }
      finally { $j.PS.Dispose() }
    }
    $rsPool.Close(); $rsPool.Dispose()
    Write-Host 'continuous workers stopped'
  }

  Write-Host ''
  Write-Host '== threshold report =='
  $steps | Format-Table EpsPerLogical, TotalTargetEps, ClientEps, RecvPerSec, DropPerSec, QueuePct, LagSec, Verdict, Reason -AutoSize
  Write-Host ("CSV for charts: {0}" -f $ThresholdLog)

  $maxRecv = ($steps | Where-Object { $null -ne $_.RecvPerSec } | Measure-Object -Property RecvPerSec -Maximum).Maximum
  $maxClient = ($steps | Measure-Object -Property ClientEps -Maximum).Maximum

  if ($firstBad -and $firstBad.Verdict -eq 'client_limit') {
    Write-Host 'RESULT: CLIENT GENERATOR LIMIT (not GeoAtlas ceiling)'
    Write-Host ("  max client send ~{0} EPS; max server recv ~{1} EPS; drops stayed ~0" -f $maxClient, $maxRecv)
    Write-Host '  Retry with more parallelism, e.g.:'
    Write-Host '    -WorkersPerConnector 4 -Batch 200 -FindThreshold ...'
    Write-Host '  Or run generators from several PCs / closer network.'
  } elseif ($firstBad -and $firstBad.Verdict -eq 'fail') {
    if ($lastGood) {
      Write-Host ("LAST STABLE (server): recv/s~{0} at target~{1} (logical {2}/conn)" -f `
        $lastGood.RecvPerSec, $lastGood.TotalTargetEps, $lastGood.EpsPerLogical)
      Write-Host ("FIRST SERVER FAIL at target~{0}: {1}" -f $firstBad.TotalTargetEps, $firstBad.Reason)
      Write-Host ("EST. SERVER CEILING: between recv~{0} and target~{1} EPS" -f $lastGood.RecvPerSec, $firstBad.TotalTargetEps)
    } else {
      Write-Host ("FAILED on first step: {0}" -f $firstBad.Reason)
    }
  } elseif ($lastGood) {
    Write-Host ("No server failure up to target~{0} EPS (max recv/s~{1}). Raise -MaxEpsPerSender / -WorkersPerConnector." -f `
      ($MaxEpsPerSender * $nLogical), $maxRecv)
  }
  Write-Host ''
  Write-Host 'Compare measured recv/s with install profile expected_eps_max on /system Overview.'
  exit 0
}

# --------------- fixed-rate burst ---------------
$totalTargetEps = $EpsPerSender * $LogicalConnectors.Count
Write-Host ''
Write-Host ("== load-test-connectors host={0} duration={1}s eps/logical={2} logical={3} threads={4} total~{5} EPS ==" -f `
  $HostAddress, $DurationSec, $EpsPerSender, $LogicalConnectors.Count, $Connectors.Count, $totalTargetEps)
foreach ($c in $LogicalConnectors) {
  Write-Host ("  -> {0} ({1}) :{2}/{3}" -f $c.Name, $c.Id, $c.Port, $c.Proto)
}
Write-Host ''
Write-Host ("sending for {0}s..." -f $DurationSec)
$results = Invoke-ConnectorBurst -EpsPerLogical $EpsPerSender -Seconds $DurationSec
Write-Host ''
Write-Host '== results =='
$results | Format-Table Name, Id, Port, Proto, Sent, EpsActual, Errors, ElapsedSec, Error -AutoSize
[void](Show-BurstSummary $results 'Total')
Write-Host ''
Write-Host 'Check on server:'
Write-Host '  UI -> /system -> Pipeline (EPS / drops / per-connector)'
Write-Host ''
Write-Host 'Steady 1h soak (~10k EPS, stable stream):'
Write-Host '  .\load-test-connectors.ps1 -Soak -EpsPerSender 2500 -DurationSec 3600 -WorkersPerConnector 4 -Batch 50 -ApiToken $token'
Write-Host 'Capacity ramp (continuous):'
Write-Host '  .\load-test-connectors.ps1 -FindThreshold -ApiToken $token -WorkersPerConnector 4 -Batch 200 -SettleSec 0'

