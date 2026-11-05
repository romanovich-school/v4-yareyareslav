param([string]$Base = 'http://localhost:8080')

$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$resultDir = Join-Path $PSScriptRoot 'results/4'
New-Item -ItemType Directory -Force $resultDir | Out-Null
$utf8 = New-Object System.Text.UTF8Encoding($false)

function Compose {
    $ErrorActionPreference = 'Continue'
    & docker compose -f compose/docker-compose.yml @args
    if ($LASTEXITCODE -ne 0) { throw "docker compose failed: $args" }
}

function Save-Text($Name, $Lines) {
    [IO.File]::WriteAllLines((Join-Path $resultDir $Name), [string[]]$Lines, $utf8)
}

function Register($Role) {
    $body = @{ email = "lab4-$Role-$([guid]::NewGuid())@test.local"; password = 'lab4-password'; role = $Role } | ConvertTo-Json
    (Invoke-RestMethod -Method Post -Uri "$Base/auth/register" -ContentType 'application/json' -Body $body).access_token
}

function Save-Logs($Suffix, $Since, $Until) {
    foreach ($service in @('gateway', 'orders-svc', 'catalog-svc')) {
        $lines = Compose logs --no-color --no-log-prefix --since $Since --until $Until $service
        Save-Text "$service-$Suffix.log" $lines
    }
}

Push-Location (Join-Path $PSScriptRoot 'load')
try {
    & go build -buildvcs=false -o (Join-Path $resultDir 'load.exe') .
    if ($LASTEXITCODE -ne 0) { throw 'load build failed' }
} finally { Pop-Location }

$seller = Register 'seller'
$buyer = Register 'buyer'
$listing = Invoke-RestMethod -Method Post -Uri "$Base/listings" -ContentType 'application/json' -Headers @{ Authorization = "Bearer $seller" } -Body '{"title":"lab4-trace","price":10,"stock":1}'
$rid = [guid]::NewGuid().ToString()
$traceStart = [DateTime]::UtcNow.ToString('o')
$response = Invoke-WebRequest -UseBasicParsing -Method Post -Uri "$Base/orders" -ContentType 'application/json' -Headers @{ Authorization = "Bearer $buyer"; 'Idempotency-Key' = [guid]::NewGuid().ToString(); 'X-Request-ID' = $rid } -Body (@{ listing_id = $listing.id; qty = 1 } | ConvertTo-Json)
Save-Text 'trace-response.txt' @("request_id=$rid", "status=$($response.StatusCode)", $response.Content)
$traceLines = foreach ($service in @('gateway', 'orders-svc', 'catalog-svc')) {
    foreach ($line in (Compose logs --no-color --no-log-prefix --since $traceStart $service)) {
        if ($line.Contains($rid)) { "${service}: $line" }
    }
}
Save-Text 'trace.log' $traceLines
Get-Content (Join-Path $resultDir 'trace-response.txt')
$loadProcess = $null
try {
    $loadSince = [DateTime]::UtcNow.ToString('o')
    $loadOutput = Join-Path $resultDir 'load-30s.txt'
    $loadProcess = Start-Process -FilePath (Join-Path $resultDir 'load.exe') -ArgumentList @('-base', $Base, '-mode', 'orders', '-n', '1000000', '-c', '10', '-buyers', '1', '-stock', '1000000', '-duration', '30s') -WindowStyle Hidden -PassThru -RedirectStandardOutput $loadOutput -RedirectStandardError (Join-Path $resultDir 'load-30s.stderr')
    $setupTimer = [Diagnostics.Stopwatch]::StartNew()
    while (-not (Select-String -Path $loadOutput -Pattern '^load started at ' -Quiet)) {
        if ($loadProcess.HasExited -or $setupTimer.Elapsed.TotalSeconds -gt 30) { throw 'load failed to start' }
        Start-Sleep -Milliseconds 100
    }
    $startedLine = Get-Content $loadOutput | Where-Object { $_ -match '^load started at ' }
    $loadStart = [DateTime]::Parse($startedLine.Substring('load started at '.Length)).ToUniversalTime()
    while (([DateTime]::UtcNow - $loadStart).TotalSeconds -lt 8) { Start-Sleep -Milliseconds 100 }
    $stopStart = [DateTime]::UtcNow.ToString('o')
    Compose stop -t 0 catalog-svc
    $stoppedAt = [DateTime]::UtcNow.ToString('o')
    Write-Output "catalog stopped at $stoppedAt"
    while (([DateTime]::UtcNow - $loadStart).TotalSeconds -lt 18) { Start-Sleep -Milliseconds 100 }
    Compose start catalog-svc
    $restartedAt = [DateTime]::UtcNow.ToString('o')
    Write-Output "catalog restarted at $restartedAt"
    $loadProcess.WaitForExit()
    if ($loadProcess.ExitCode -ne 0) { throw '30-second load failed' }
    $loadUntil = [DateTime]::UtcNow.ToString('o')
    Save-Logs '30s' $loadSince $loadUntil
    Save-Text 'load-window.txt' @("since=$loadSince", "stop_started=$stopStart", "catalog_stopped=$stoppedAt", "catalog_restarted=$restartedAt", "until=$loadUntil")
    Get-Content $loadOutput

    $beforeKafkaTimer = [Diagnostics.Stopwatch]::StartNew()
    do {
        $beforeKafkaPending = Compose exec -T postgres psql -U app -d orders -tAc 'select count(*) from outbox where published_at is null'
        if ([int]$beforeKafkaPending -eq 0) { break }
        if ($beforeKafkaTimer.Elapsed.TotalSeconds -gt 60) { throw 'outbox did not drain before Kafka experiment' }
        Start-Sleep -Milliseconds 500
    } while ($true)
    Save-Text 'outbox-before-kafka.txt' @("pending=$beforeKafkaPending")
    Compose stop kafka
    $kafkaSince = [DateTime]::UtcNow.ToString('o')
    $kafkaTimer = [Diagnostics.Stopwatch]::StartNew()
    & (Join-Path $resultDir 'load.exe') -base $Base -mode orders -n 300 -c 20 -buyers 1 -stock 1000 | Tee-Object -FilePath (Join-Path $resultDir 'load-kafka-down.txt')
    if ($LASTEXITCODE -ne 0) { throw 'Kafka-down load failed' }
    while ($kafkaTimer.Elapsed.TotalSeconds -lt 30) { Start-Sleep -Milliseconds 100 }
    $kafkaUntil = [DateTime]::UtcNow.ToString('o')
    Save-Logs 'kafka-down' $kafkaSince $kafkaUntil
    Save-Text 'kafka-window.txt' @("since=$kafkaSince", "until=$kafkaUntil", "seconds=$(([DateTime]::Parse($kafkaUntil) - [DateTime]::Parse($kafkaSince)).TotalSeconds)")
    Compose exec -T postgres psql -U app -d orders -c 'select count(*) from outbox where published_at is null' | Tee-Object -FilePath (Join-Path $resultDir 'outbox-before-recovery.txt')
} finally {
    if ($loadProcess -and -not $loadProcess.HasExited) { Stop-Process -Id $loadProcess.Id }
    Compose start kafka catalog-svc
}

$drainTimer = [Diagnostics.Stopwatch]::StartNew()
do {
    $pending = Compose exec -T postgres psql -U app -d orders -tAc 'select count(*) from outbox where published_at is null'
    if ([int]$pending -eq 0) { break }
    if ($drainTimer.Elapsed.TotalSeconds -gt 60) { throw 'outbox did not drain in 60 seconds' }
    Start-Sleep -Milliseconds 500
} while ($true)
Save-Text 'outbox-after-recovery.txt' @("pending=$pending", "drain_seconds=$($drainTimer.Elapsed.TotalSeconds)")
Write-Output "outbox pending after recovery=$pending"

$kafkaLoad = Get-Content (Join-Path $resultDir 'load-kafka-down.txt')
$kafkaListingLine = $kafkaLoad | Where-Object { $_ -match '^listing (\S+) stock=' } | Select-Object -First 1
$kafkaListing = ($kafkaListingLine -split ' ')[1]
$consumerTimer = [Diagnostics.Stopwatch]::StartNew()
do {
    $finalListing = Invoke-RestMethod -Uri "$Base/listings/$kafkaListing"
    if ($finalListing.stock -eq 700 -and $finalListing.sold -eq 300) { break }
    if ($consumerTimer.Elapsed.TotalSeconds -gt 60) { throw 'catalog did not apply all 300 events in 60 seconds' }
    Start-Sleep -Milliseconds 500
} while ($true)
Save-Text 'listing-after-recovery.json' @($finalListing | ConvertTo-Json -Compress)

$httpOrders = @(Get-Content (Join-Path $resultDir 'gateway-30s.log') | ForEach-Object { $_ | ConvertFrom-Json } | Where-Object { $_.msg -eq 'HTTP request' -and $_.method -eq 'POST' -and $_.path -eq '/orders' })
$unavailable = @(Get-Content (Join-Path $resultDir 'orders-svc-30s.log') | Where-Object { $_ -match 'for /marketplace\.orders\.v1\.OrdersService/Create completed with Unavailable' -and $_ -match 'reserve listing: rpc error: code = Unavailable' } | ForEach-Object { ($_ -split '"')[1] } | Sort-Object -Unique)
$http503 = @($httpOrders | Where-Object { $_.status -eq 503 })
$matched = @($http503 | Where-Object { $_.request_id -in $unavailable })
Save-Text 'catalog-unavailable-ids.txt' $unavailable
$searchTimer = [Diagnostics.Stopwatch]::StartNew()
$outboxLog = Get-Content (Join-Path $resultDir 'orders-svc-kafka-down.log')
$attempts = @($outboxLog | Where-Object { $_ -match 'outbox publish attempt batch_size=' })
$sizes = @($attempts | ForEach-Object { if ($_ -match 'batch_size=(\d+)') { [int]$Matches[1] } } | Sort-Object)
$failed = @($attempts | Where-Object { $_ -match 'success=false' }).Count
$warnings = @($outboxLog | Where-Object { $_ -match 'WARN: outbox publication failed:' }).Count
if ($sizes.Count -eq 0) { throw 'no outbox attempts recorded' }
$middle = [int][Math]::Floor($sizes.Count / 2)
$median = if ($sizes.Count % 2) { $sizes[$middle] } else { ($sizes[$middle - 1] + $sizes[$middle]) / 2 }
$searchTimer.Stop()
$summary = @(
    "http_orders=$($httpOrders.Count)",
    "http_201=$(@($httpOrders | Where-Object { $_.status -eq 201 }).Count)",
    "http_503=$($http503.Count)",
    "catalog_unavailable_unique=$($unavailable.Count)",
    "matched_http503_unavailable=$($matched.Count)",
    "orders_deadline_exceeded=$(@(Get-Content (Join-Path $resultDir 'orders-svc-30s.log') | Where-Object { $_ -match 'for /marketplace\.orders\.v1\.OrdersService/Create completed with DeadlineExceeded' }).Count)",
    "outbox_attempts=$($attempts.Count)",
    "outbox_failed=$failed",
    "outbox_success=$($attempts.Count - $failed)",
    "outbox_warnings=$warnings",
    "outbox_batch_sizes=$($sizes -join ',')",
    "outbox_batch_median=$median",
    "outbox_search_ms=$([Math]::Round($searchTimer.Elapsed.TotalMilliseconds, 3))"
)
Save-Text 'summary.txt' $summary
$summary
