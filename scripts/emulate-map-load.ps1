#Requires -Version 5.1
<#
.SYNOPSIS
  Emulate MSE syslog TCP to GeoAtlas (:514) — map load + anomaly triggers.

.DESCRIPTION
  IPs only from your GeoIP ranges (10.x + 1.0/1.1).
  By default injects demo anomaly patterns (safe margins for small/medium):
    - port_scan:       public 1.x src -> many dst_ports (touches 10.x)
    - lateral_fanout:  10.x src -> many other 10.x hosts (needs enterprise nets)
  Optional: horizontal_scan via -Recipes. Then optional background noise
  (-DurationSec / -Lines) unless -AnomalyOnly.
  Scanner runs every ~5 min. Mark 10.x as enterprise nets; open /anomalies.

.EXAMPLE
  # Presentation demo (anomaly burst only)
  .\emulate-map-load.ps1 -AnomalyOnly

.EXAMPLE
  .\emulate-map-load.ps1 -Eps 80 -DurationSec 60

.EXAMPLE
  .\emulate-map-load.ps1 -AnomalyOnly -Recipes port_scan,horizontal_scan,lateral_fanout
#>
param(
  [string]$HostAddress = '138.16.225.160',
  [int]$Port = 514,
  [int]$Eps = 50,
  [int]$DurationSec = 30,
  [int]$Lines = 0,
  [int]$Batch = 50,
  # Comma-separated: port_scan, horizontal_scan, lateral_fanout
  [string]$Recipes = 'port_scan,lateral_fanout',
  [switch]$AnomalyOnly,
  [switch]$NoAnomaly,
  [switch]$DryRun
)

$ErrorActionPreference = 'Stop'

$RangeSpecs = @(
  '10.0.0.0-10.0.255.254','10.1.0.0-10.1.255.254','10.2.0.0-10.2.255.254','10.3.0.0-10.3.255.254',
  '10.26.0.0-10.26.127.254','10.26.176.0-10.26.176.126','10.26.192.0-10.26.192.254',
  '10.27.0.0-10.27.255.254','10.28.0.0-10.28.255.254','10.25.0.0-10.25.255.254',
  '10.30.0.0-10.30.255.254','10.32.0.0-10.32.255.254',
  '10.68.0.0-10.68.255.254','10.69.0.0-10.69.255.254','10.70.0.0-10.70.255.254','10.71.0.0-10.71.255.254',
  '10.72.0.0-10.72.255.254','10.73.0.0-10.73.255.254','10.74.0.0-10.74.255.254','10.75.0.0-10.75.255.254',
  '10.76.0.0-10.76.255.254','10.78.0.0-10.78.255.254','10.79.0.0-10.79.255.254','10.81.0.0-10.81.255.254',
  '10.82.0.0-10.82.255.254','10.84.0.0-10.84.255.254','10.86.0.0-10.86.255.254','10.87.0.0-10.87.255.254',
  '10.88.0.0-10.88.255.254','10.91.0.0-10.91.255.254','10.92.0.0-10.92.255.254','10.93.0.0-10.93.255.254',
  '10.95.0.0-10.95.255.254','10.97.0.0-10.97.255.254','10.99.0.0-10.99.255.254','10.100.0.0-10.100.255.254',
  '10.107.0.0-10.107.255.254','10.108.0.0-10.108.255.254','10.109.0.0-10.109.255.254','10.110.0.0-10.110.255.254',
  '10.111.0.0-10.111.255.254','10.112.0.0-10.112.255.254','10.113.0.0-10.113.255.254','10.115.0.0-10.115.255.254',
  '10.117.0.0-10.117.255.254','10.120.0.0-10.120.255.254','10.127.0.0-10.127.255.254',
  '10.144.0.0-10.144.255.254','10.145.0.0-10.145.255.254','10.148.0.0-10.148.255.254','10.200.0.0-10.200.255.254',
  '1.0.0.0-1.0.0.255','1.0.1.0-1.0.1.255','1.0.2.0-1.0.3.255','1.0.4.0-1.0.7.255','1.0.8.0-1.0.15.255',
  '1.0.16.0-1.0.31.255','1.0.32.0-1.0.63.255','1.0.64.0-1.0.79.255','1.0.80.0-1.0.95.255','1.0.96.0-1.0.111.255',
  '1.0.112.0-1.0.127.255','1.0.128.0-1.0.143.255','1.0.144.0-1.0.159.255','1.0.160.0-1.0.163.255',
  '1.0.164.0-1.0.164.15','1.0.164.16-1.0.164.19','1.0.164.20-1.0.164.21','1.0.164.23-1.0.164.23',
  '1.0.164.24-1.0.164.31','1.0.164.32-1.0.164.63','1.0.164.64-1.0.164.127','1.0.164.128-1.0.164.255',
  '1.0.165.0-1.0.165.255','1.0.166.0-1.0.167.255','1.0.168.0-1.0.175.255','1.0.176.0-1.0.191.255',
  '1.0.192.0-1.0.207.255','1.0.208.0-1.0.215.255','1.0.216.0-1.0.223.255','1.0.224.0-1.0.239.255','1.0.240.0-1.0.255.255',
  '1.1.0.0-1.1.0.255','1.1.1.0-1.1.1.255','1.1.2.0-1.1.3.255','1.1.4.0-1.1.7.255','1.1.8.0-1.1.15.255'
)

function ConvertTo-UInt32Ip([string]$ip) {
  $parts = $ip.Split('.') | ForEach-Object { [uint32]$_ }
  return ($parts[0] -shl 24) -bor ($parts[1] -shl 16) -bor ($parts[2] -shl 8) -bor $parts[3]
}
function ConvertFrom-UInt32Ip([uint32]$n) {
  return '{0}.{1}.{2}.{3}' -f (($n -shr 24) -band 255), (($n -shr 16) -band 255), (($n -shr 8) -band 255), ($n -band 255)
}

$Ranges = foreach ($spec in $RangeSpecs) {
  $a, $b = $spec -split '-', 2
  $start = ConvertTo-UInt32Ip $a.Trim()
  $end = ConvertTo-UInt32Ip $b.Trim()
  if ($end -lt $start) { throw "bad range: $spec" }
  [pscustomobject]@{ Spec = $spec; Start = $start; End = $end; Size = [uint64]($end - $start) + 1; IsPrivate10 = $spec.StartsWith('10.') }
}
$PublicRanges = @($Ranges | Where-Object { -not $_.IsPrivate10 })
$PrivateRanges = @($Ranges | Where-Object { $_.IsPrivate10 })

$Actions = @('accept','allow','deny','drop','close')
$Protos = @('TCP','UDP')
$DstPorts = @(443,80,53,22,8080,8443,25,3389)

$script:rangeIdx = 0
$script:uniquePairs = New-Object 'System.Collections.Generic.HashSet[string]'
$script:sent = 0

function Get-RandomIpFromRange($range) {
  if ($range.Size -eq 1) { return ConvertFrom-UInt32Ip $range.Start }
  $max = [int64][Math]::Min([int64]$range.Size, [int64][int]::MaxValue)
  $offset = [uint64](Get-Random -Maximum $max)
  return ConvertFrom-UInt32Ip ([uint32]($range.Start + $offset))
}

function New-CefLine([string]$src, [string]$dst, [int]$spt, [int]$dpt, [string]$act, [string]$proto, [int]$vendor) {
  $protoNum = if ($proto -eq 'UDP') { 17 } else { 6 }
  $inB = Get-Random -Minimum 40 -Maximum 5000
  $outB = Get-Random -Minimum 20 -Maximum 2000
  $pk1 = Get-Random -Minimum 1 -Maximum 40
  $pk2 = Get-Random -Minimum 1 -Maximum 30
  switch ($vendor) {
    0 {
      $rt = [int64]([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())
      return "CEF:0|Usergate|UTM|6|traffic|firewall|1|rt=$rt deviceExternalId=utmcore@lab suser=user_lab act=$act cs1Label=Rule cs1=Allow trusted to untrusted src=$src spt=$spt cs2Label=Source Zone cs2=Trusted proto=$proto dst=$dst dpt=$dpt cs4Label=Destination Zone cs4=Untrusted in=$inB out=$outB cn1Label=Packets sent cn1=$pk1 cn2Label=Packets received cn2=$pk2"
    }
    1 {
      $app = switch ($dpt) { 53 { 'DNS' } 22 { 'SSH' } default { 'HTTPS' } }
      $et = [int][double]::Parse((Get-Date -UFormat %s))
      $eid = Get-Random -Minimum 1000 -Maximum 90000
      return "CEF: 0|Fortinet|Fortigate|v6.0.3|00013|traffic:forward $act|3|deviceExternalId=FGT-LAB FTNTFGTlogid=0000000013 cat=traffic:forward FTNTFGTsubtype=forward FTNTFGTlevel=notice FTNTFGTvd=root FTNTFGTeventtime=$et src=$src spt=$spt deviceInboundInterface=port12 dst=$dst dpt=$dpt deviceOutboundInterface=port11 externalId=$eid proto=$protoNum act=$act FTNTFGTpolicyid=1 app=$app out=$outB in=$inB FTNTFGTsentpkt=$pk1 FTNTFGTrcvdpkt=$pk2 FTNTFGTutmaction=allow"
    }
    default {
      return "src=$src dst=$dst spt=$spt dpt=$dpt act=$act proto=$proto rule=LabRule out=$outB in=$inB cn1=$pk1 cn2=$pk2"
    }
  }
}

function Next-NoisePair {
  $srcRange = $Ranges[$script:rangeIdx % $Ranges.Count]
  $script:rangeIdx++
  do { $dstRange = $Ranges[(Get-Random -Maximum $Ranges.Count)] } while ($dstRange.Spec -eq $srcRange.Spec -and $Ranges.Count -gt 1)
  $src = Get-RandomIpFromRange $srcRange
  $dst = Get-RandomIpFromRange $dstRange
  [void]$script:uniquePairs.Add(('{0}->{1}' -f $src, $dst))
  return [pscustomobject]@{ Src = $src; Dst = $dst }
}

function New-NoiseLine {
  $p = Next-NoisePair
  New-CefLine $p.Src $p.Dst (Get-Random -Minimum 10000 -Maximum 60000) `
    $DstPorts[(Get-Random -Maximum $DstPorts.Count)] `
    $Actions[(Get-Random -Maximum $Actions.Count)] `
    $Protos[(Get-Random -Maximum $Protos.Count)] `
    (Get-Random -Maximum 3)
}

# --- Anomaly recipes (margins vs thresholds.go) ---
# small:  port_scan 40p/80e; horizontal 30h/60e; lateral 20h/40e
# medium: port_scan 50p/100e; horizontal 40h/80e; lateral 25h/50e
# Demo volumes below clear small+medium; PUBLIC src for scans (include_private=false).

function Resolve-RecipeSet([string]$raw) {
  $allowed = @('port_scan', 'horizontal_scan', 'lateral_fanout')
  $parts = @($raw -split ',' | ForEach-Object { $_.Trim().ToLowerInvariant() } | Where-Object { $_ })
  if ($parts.Count -eq 0) { throw 'Recipes is empty; use port_scan,horizontal_scan,lateral_fanout' }
  $set = New-Object 'System.Collections.Generic.HashSet[string]'
  foreach ($p in $parts) {
    if ($allowed -notcontains $p) {
      throw ("Unknown recipe '{0}'. Allowed: {1}" -f $p, ($allowed -join ', '))
    }
    [void]$set.Add($p)
  }
  return $set
}

$RecipeSet = Resolve-RecipeSet $Recipes

function Get-AnomalyLines {
  $lines = New-Object System.Collections.Generic.List[string]

  # Fixed public attackers from your 1.x ranges (not RFC1918 — scans ignore private src by default)
  $attackerPort = '1.0.1.50'
  $attackerHoriz = '1.1.8.20'
  $victimPort = '10.0.10.10'          # enterprise target for port scan
  $horizBase = 10,0,1                 # scan 10.0.1.0/24
  # Must sit inside enterprise_nets (detector filters src AND dst). Prefer gray 10.0/16.
  $lateralSrc = '10.0.68.10'

  if ($RecipeSet.Contains('port_scan')) {
    Write-Host 'anomaly recipe: port_scan (1.0.1.50 -> 10.0.10.10, 100 ports x 2 = 200 events)'
    foreach ($port in 1..100) {
      1..2 | ForEach-Object {
        $lines.Add((New-CefLine $attackerPort $victimPort (10000 + $port) $port 'deny' 'TCP' 0))
      }
    }
  }

  if ($RecipeSet.Contains('horizontal_scan')) {
    Write-Host 'anomaly recipe: horizontal_scan (1.1.8.20 -> 10.0.1.1..60, 120 events)'
    foreach ($hostOctet in 1..60) {
      $dst = '{0}.{1}.{2}.{3}' -f $horizBase[0], $horizBase[1], $horizBase[2], $hostOctet
      1..2 | ForEach-Object {
        $lines.Add((New-CefLine $attackerHoriz $dst (20000 + $hostOctet) 445 'deny' 'TCP' 2))
      }
    }
  }

  if ($RecipeSet.Contains('lateral_fanout')) {
    # Detector requires BOTH src and dst in enterprise_nets. Stick to the usual gray
    # enterprise ranges (same as load-test): 10.0 / 10.25 / 10.27 / 10.28 / 10.30.
    # 40 unique hosts x 3 = 120 events (clears small 20/40 and medium 25/50).
    $latTargets = @(
      '10.0.2.11','10.0.3.12','10.0.4.13','10.0.5.14','10.0.6.15','10.0.7.16','10.0.8.17','10.0.9.18',
      '10.0.10.19','10.0.11.20','10.0.12.21','10.0.13.22','10.0.14.23','10.0.15.24','10.0.16.25','10.0.17.26',
      '10.25.10.19','10.25.11.20','10.25.12.21','10.25.13.22','10.25.14.23','10.25.15.24','10.25.16.25','10.25.17.26',
      '10.27.12.21','10.27.13.22','10.27.14.23','10.27.15.24','10.28.14.23','10.28.15.24','10.28.16.25','10.28.17.26',
      '10.30.16.25','10.30.17.26','10.30.18.27','10.30.19.28','10.30.20.29','10.30.21.30','10.30.22.31','10.30.23.32'
    )
    Write-Host ("anomaly recipe: lateral_fanout ({0} -> {1} gray enterprise hosts x 3 = {2} events)" -f `
      $lateralSrc, $latTargets.Count, ($latTargets.Count * 3))
    foreach ($dst in $latTargets) {
      1..3 | ForEach-Object {
        $lines.Add((New-CefLine $lateralSrc $dst (30000 + (Get-Random -Maximum 20000)) 445 'accept' 'TCP' 0))
      }
    }
  }

  return $lines
}

function Write-AnomalyChecklist {
  $selected = @($RecipeSet | Sort-Object)
  Write-Host 'Anomaly checklist:'
  Write-Host '  1) Enterprise nets must cover gray ranges used by lateral: 10.0/16, 10.25/16, 10.27/16, 10.28/16, 10.30/16'
  Write-Host '  2) Wait for scanner (~5 min, ANOMALY_SCAN_INTERVAL); lateral window is 15 min'
  Write-Host ("  3) Open /anomalies — expect: {0}" -f ($selected -join ', '))
  Write-Host '  4) Use «Разбор» / map deep-link on each alert'
  Write-Host '  5) If port_scan missing: confirm 1.x src is public (include_private=false is OK)'
  Write-Host '  6) If lateral missing: src+dst must both be inside enterprise_nets (not only dst)'
  Write-Host '  7) If peers empty in «Разбор»: GeoIP must have coords for BOTH src and dst (10.x often missing)'
  Write-Host '  8) Re-run same day: ANOMALY_SUPPRESS_HOURS=24 — use -Recipes lateral_fanout only, or change src IP'
  $bits = @()
  if ($RecipeSet.Contains('port_scan')) { $bits += 'port_scan src=1.0.1.50 dst=10.0.10.10' }
  if ($RecipeSet.Contains('horizontal_scan')) { $bits += 'horizontal src=1.1.8.20 net=10.0.1.0/24' }
  if ($RecipeSet.Contains('lateral_fanout')) { $bits += 'lateral src=10.0.68.10 (gray 10.0/16)' }
  Write-Host ("  Actors: {0}" -f ($bits -join '; '))
}

if ($DryRun) {
  Write-Host ("== dry-run: {0} ranges (public={1}, private10={2}) recipes={3} ==" -f `
    $Ranges.Count, $PublicRanges.Count, $PrivateRanges.Count, (($RecipeSet | Sort-Object) -join ','))
  if (-not $NoAnomaly) {
    $anom = Get-AnomalyLines
    Write-Host ("anomaly lines: {0}" -f $anom.Count)
    $anom | Select-Object -First 3
  }
  Write-Host '--- noise sample ---'
  1..3 | ForEach-Object { New-NoiseLine }
  Write-Host ''
  Write-AnomalyChecklist
  exit 0
}

$target = '{0}:{1}' -f $HostAddress, $Port
Write-Host ("== emulate-map-load TCP syslog: {0} eps={1} duration={2}s lines={3} anomaly={4} recipes={5} ==" -f `
  $target, $Eps, $DurationSec, $Lines, (-not $NoAnomaly), (($RecipeSet | Sort-Object) -join ','))

$client = New-Object System.Net.Sockets.TcpClient
try { $client.Connect($HostAddress, $Port) }
catch { throw ("Cannot open TCP {0}: {1}" -f $target, $_) }
$stream = $client.GetStream()
$writer = New-Object System.IO.StreamWriter($stream, [Text.Encoding]::UTF8, 65536)
$writer.NewLine = "`n"
$writer.AutoFlush = $false

function Send-TextLines([System.Collections.IEnumerable]$textLines) {
  foreach ($line in $textLines) {
    $writer.WriteLine([string]$line)
    $script:sent++
  }
  $writer.Flush()
}
function Send-NoiseBatch([int]$n) {
  for ($i = 0; $i -lt $n; $i++) { $writer.WriteLine((New-NoiseLine)) }
  $writer.Flush()
  $script:sent += $n
}

try {
  if (-not $NoAnomaly) {
    $anom = Get-AnomalyLines
    Write-Host ("sending anomaly burst: {0} lines..." -f $anom.Count)
    Send-TextLines $anom
    Write-Host ("anomaly burst done, sent={0}" -f $script:sent)
  }

  if (-not $AnomalyOnly) {
    if ($Lines -gt 0) {
      Write-Host ("noise burst: {0} lines" -f $Lines)
      $left = $Lines
      while ($left -gt 0) {
        $n = [Math]::Min($Batch, $left)
        Send-NoiseBatch $n
        $left -= $n
        Write-Host ("`r  sent={0} unique_pairs={1}" -f $script:sent, $script:uniquePairs.Count) -NoNewline
      }
      Write-Host ''
    } elseif ($DurationSec -gt 0) {
      if ($Eps -lt 1) { throw 'Eps must be >= 1' }
      Write-Host ("noise rate: ~{0} EPS for {1}s" -f $Eps, $DurationSec)
      $deadline = [DateTime]::UtcNow.AddSeconds($DurationSec)
      $delaySec = [Math]::Max(0.01, $Batch / [double]$Eps)
      while ([DateTime]::UtcNow -lt $deadline) {
        Send-NoiseBatch $Batch
        Start-Sleep -Milliseconds ([int]($delaySec * 1000))
        Write-Host ("`r  sent={0} unique_pairs={1}" -f $script:sent, $script:uniquePairs.Count) -NoNewline
      }
      Write-Host ''
    }
  }
} finally {
  $writer.Flush(); $writer.Dispose(); $stream.Dispose(); $client.Close()
}

Write-Host ("OK: sent ~{0} lines to {1} (unique noise pairs={2})." -f $script:sent, $target, $script:uniquePairs.Count)
Write-Host ''
Write-AnomalyChecklist
