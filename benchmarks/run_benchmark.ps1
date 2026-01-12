<#
.SYNOPSIS
    Benchmark suite for rigrun LLM router performance testing.

.DESCRIPTION
    Runs comprehensive benchmarks to measure:
    - Cache hit rates (exact vs semantic)
    - Latency improvements
    - Cost savings
    - Memory efficiency

.PARAMETER Iterations
    Number of times to repeat the full benchmark suite. Default: 1

.PARAMETER SkipServerStart
    Skip starting rigrun server (use if already running)

.PARAMETER TestType
    Run specific test type: 'all', 'semantic', 'exact', 'stress'. Default: 'all'

.PARAMETER ServerUrl
    rigrun server URL. Default: http://localhost:8787

.PARAMETER Verbose
    Enable verbose output

.EXAMPLE
    .\run_benchmark.ps1 -Iterations 3

.EXAMPLE
    .\run_benchmark.ps1 -SkipServerStart -TestType semantic
#>

param(
    [int]$Iterations = 1,
    [switch]$SkipServerStart,
    [ValidateSet('all', 'semantic', 'exact', 'stress')]
    [string]$TestType = 'all',
    [string]$ServerUrl = 'http://localhost:8787',
    [switch]$VerboseOutput
)

# Configuration
$ErrorActionPreference = 'Stop'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir
$QueriesFile = Join-Path $ScriptDir 'queries.json'
$ResultsDir = Join-Path $ScriptDir 'results'
$Timestamp = Get-Date -Format 'yyyyMMdd_HHmmss'

# Cost assumptions (per 1K tokens, in USD)
$CostPerKTokenInput = 0.03   # GPT-4 input pricing
$CostPerKTokenOutput = 0.06  # GPT-4 output pricing
$AvgTokensPerQuery = 500     # Average tokens per query/response

# Ensure results directory exists
if (-not (Test-Path $ResultsDir)) {
    New-Item -ItemType Directory -Path $ResultsDir -Force | Out-Null
}

# Initialize results structure
$Results = @{
    metadata = @{
        timestamp = $Timestamp
        iterations = $Iterations
        server_url = $ServerUrl
        test_type = $TestType
        platform = [System.Environment]::OSVersion.VersionString
        powershell_version = $PSVersionTable.PSVersion.ToString()
    }
    phases = @{
        warmup = @{ status = 'pending' }
        baseline = @{ status = 'pending' }
        exact_cache = @{ status = 'pending' }
        semantic_cache = @{ status = 'pending' }
        mixed_workload = @{ status = 'pending' }
        stress_test = @{ status = 'pending' }
    }
    summary = @{
        total_queries = 0
        cache_hits = 0
        semantic_hits = 0
        exact_hits = 0
        misses = 0
        avg_cache_latency_ms = 0
        avg_miss_latency_ms = 0
        estimated_cost_savings_usd = 0
        semantic_hit_rate = 0
        exact_hit_rate = 0
        total_hit_rate = 0
    }
}

# Helper Functions
function Write-Log {
    param([string]$Message, [string]$Level = 'INFO')
    $timestamp = Get-Date -Format 'HH:mm:ss'
    $color = switch ($Level) {
        'ERROR' { 'Red' }
        'WARN'  { 'Yellow' }
        'SUCCESS' { 'Green' }
        'DEBUG' { 'Gray' }
        default { 'White' }
    }
    Write-Host "[$timestamp] [$Level] $Message" -ForegroundColor $color
}

function Test-ServerHealth {
    param([string]$Url)
    try {
        $response = Invoke-RestMethod -Uri "$Url/health" -Method Get -TimeoutSec 5
        return $response.status -eq 'ok' -or $response.status -eq 'degraded'
    }
    catch {
        return $false
    }
}

function Clear-ServerCache {
    param([string]$Url)
    # Note: rigrun doesn't have a cache clear endpoint yet
    # For now, we restart the server or wait for TTL
    Write-Log "Cache clearing requested (manual restart may be needed for clean baseline)" -Level 'WARN'
}

function Send-ChatQuery {
    param(
        [string]$Url,
        [string]$Query,
        [string]$Model = 'auto'
    )

    $body = @{
        model = $Model
        messages = @(
            @{
                role = 'user'
                content = $Query
            }
        )
    } | ConvertTo-Json -Depth 10

    $startTime = Get-Date

    try {
        $response = Invoke-RestMethod -Uri "$Url/v1/chat/completions" `
            -Method Post `
            -Body $body `
            -ContentType 'application/json' `
            -TimeoutSec 120

        $endTime = Get-Date
        $latencyMs = ($endTime - $startTime).TotalMilliseconds

        return @{
            success = $true
            latency_ms = [math]::Round($latencyMs, 2)
            model = $response.model
            tokens = $response.usage.total_tokens
            response_length = $response.choices[0].message.content.Length
            is_cache_hit = $response.model -eq 'cache'
        }
    }
    catch {
        $endTime = Get-Date
        $latencyMs = ($endTime - $startTime).TotalMilliseconds

        return @{
            success = $false
            latency_ms = [math]::Round($latencyMs, 2)
            error = $_.Exception.Message
            is_cache_hit = $false
        }
    }
}

function Get-CacheStats {
    param([string]$Url)
    try {
        $stats = Invoke-RestMethod -Uri "$Url/cache/stats" -Method Get -TimeoutSec 5
        return $stats
    }
    catch {
        return @{
            entries = 0
            hits = 0
            misses = 0
            hit_rate_percent = 0
        }
    }
}

function Start-RigrunServer {
    param([string]$ProjectRoot)

    Write-Log "Building rigrun..."
    Push-Location $ProjectRoot
    try {
        $buildResult = & cargo build --release 2>&1
        if ($LASTEXITCODE -ne 0) {
            Write-Log "Build failed: $buildResult" -Level 'ERROR'
            return $false
        }

        Write-Log "Starting rigrun server..."
        $serverProcess = Start-Process -FilePath "$ProjectRoot\target\release\rigrun.exe" `
            -PassThru `
            -WindowStyle Hidden

        # Wait for server to be ready
        $maxWait = 30
        $waited = 0
        while ($waited -lt $maxWait) {
            Start-Sleep -Seconds 1
            $waited++
            if (Test-ServerHealth -Url $ServerUrl) {
                Write-Log "Server started successfully (PID: $($serverProcess.Id))" -Level 'SUCCESS'
                return $serverProcess
            }
        }

        Write-Log "Server failed to start within $maxWait seconds" -Level 'ERROR'
        return $false
    }
    finally {
        Pop-Location
    }
}

# Load test queries
Write-Log "Loading test queries from $QueriesFile"
$TestData = Get-Content $QueriesFile -Raw | ConvertFrom-Json

# ASCII Art Header
Write-Host @"

  ____  _       ____              ____                  _                          _
 |  _ \(_) __ _|  _ \ _   _ _ __ | __ )  ___ _ __   ___| |__  _ __ ___   __ _ _ __| | __
 | |_) | |/ _` | |_) | | | | '_ \|  _ \ / _ \ '_ \ / __| '_ \| '_ ` _ \ / _` | '__| |/ /
 |  _ <| | (_| |  _ <| |_| | | | | |_) |  __/ | | | (__| | | | | | | | | (_| | |  |   <
 |_| \_\_|\__, |_| \_\\__,_|_| |_|____/ \___|_| |_|\___|_| |_|_| |_| |_|\__,_|_|  |_|\_\
          |___/

"@ -ForegroundColor Cyan

Write-Host "Benchmark Suite v1.0.0" -ForegroundColor Cyan
Write-Host "=" * 70 -ForegroundColor Gray
Write-Host ""

# Check/Start Server
if (-not $SkipServerStart) {
    if (Test-ServerHealth -Url $ServerUrl) {
        Write-Log "Server already running at $ServerUrl" -Level 'SUCCESS'
    }
    else {
        $serverProcess = Start-RigrunServer -ProjectRoot $ProjectRoot
        if (-not $serverProcess) {
            Write-Log "Failed to start server. Use -SkipServerStart if server is running elsewhere." -Level 'ERROR'
            exit 1
        }
    }
}
else {
    if (-not (Test-ServerHealth -Url $ServerUrl)) {
        Write-Log "Server not responding at $ServerUrl" -Level 'ERROR'
        exit 1
    }
    Write-Log "Using existing server at $ServerUrl" -Level 'SUCCESS'
}

# Collect initial cache stats
$initialCacheStats = Get-CacheStats -Url $ServerUrl
Write-Log "Initial cache state: $($initialCacheStats.entries) entries, $($initialCacheStats.hit_rate_percent)% hit rate"

# ============================================================================
# PHASE 1: WARM-UP
# ============================================================================
Write-Host ""
Write-Host "=" * 70 -ForegroundColor Cyan
Write-Host "PHASE 1: WARM-UP" -ForegroundColor Cyan
Write-Host "=" * 70 -ForegroundColor Cyan

$warmupQueries = $TestData.stress_test_queries | Select-Object -First 5
$warmupResults = @()

foreach ($query in $warmupQueries) {
    Write-Log "Warm-up query: $($query.Substring(0, [Math]::Min(40, $query.Length)))..."
    $result = Send-ChatQuery -Url $ServerUrl -Query $query
    $warmupResults += $result
    if ($result.success) {
        Write-Log "  Latency: $($result.latency_ms)ms, Tokens: $($result.tokens)" -Level 'DEBUG'
    }
    else {
        Write-Log "  Failed: $($result.error)" -Level 'WARN'
    }
}

$Results.phases.warmup = @{
    status = 'completed'
    queries = $warmupResults.Count
    avg_latency_ms = [math]::Round(($warmupResults | Where-Object { $_.success } | Measure-Object -Property latency_ms -Average).Average, 2)
}
Write-Log "Warm-up complete: $($warmupResults.Count) queries, avg latency: $($Results.phases.warmup.avg_latency_ms)ms" -Level 'SUCCESS'

# ============================================================================
# PHASE 2: BASELINE (First pass - populating cache)
# ============================================================================
if ($TestType -eq 'all' -or $TestType -eq 'exact') {
    Write-Host ""
    Write-Host "=" * 70 -ForegroundColor Cyan
    Write-Host "PHASE 2: BASELINE (Populating Cache)" -ForegroundColor Cyan
    Write-Host "=" * 70 -ForegroundColor Cyan

    $baselineResults = @()
    $uniqueQueries = @()

    # Get first query from each semantic group
    foreach ($group in $TestData.semantic_groups) {
        $uniqueQueries += $group.queries[0]
    }

    # Add distinct queries
    foreach ($category in $TestData.distinct_queries) {
        $uniqueQueries += $category.queries | Select-Object -First 3
    }

    Write-Log "Running $($uniqueQueries.Count) unique queries to establish baseline..."

    $queryIndex = 0
    foreach ($query in $uniqueQueries) {
        $queryIndex++
        $displayQuery = if ($query.Length -gt 50) { $query.Substring(0, 50) + "..." } else { $query }
        Write-Log "[$queryIndex/$($uniqueQueries.Count)] $displayQuery"

        $result = Send-ChatQuery -Url $ServerUrl -Query $query
        $result.query = $query
        $baselineResults += $result

        if ($result.success) {
            $cacheStatus = if ($result.is_cache_hit) { "(CACHE)" } else { "(MISS)" }
            Write-Log "  $cacheStatus Latency: $($result.latency_ms)ms" -Level 'DEBUG'
        }
        else {
            Write-Log "  FAILED: $($result.error)" -Level 'WARN'
        }

        # Small delay to avoid overwhelming the server
        Start-Sleep -Milliseconds 100
    }

    $successfulBaseline = $baselineResults | Where-Object { $_.success }
    $Results.phases.baseline = @{
        status = 'completed'
        total_queries = $baselineResults.Count
        successful_queries = $successfulBaseline.Count
        cache_hits = ($successfulBaseline | Where-Object { $_.is_cache_hit }).Count
        avg_latency_ms = [math]::Round(($successfulBaseline | Measure-Object -Property latency_ms -Average).Average, 2)
        min_latency_ms = [math]::Round(($successfulBaseline | Measure-Object -Property latency_ms -Minimum).Minimum, 2)
        max_latency_ms = [math]::Round(($successfulBaseline | Measure-Object -Property latency_ms -Maximum).Maximum, 2)
    }

    Write-Log "Baseline complete: $($Results.phases.baseline.successful_queries)/$($Results.phases.baseline.total_queries) successful" -Level 'SUCCESS'
    Write-Log "  Avg latency: $($Results.phases.baseline.avg_latency_ms)ms, Cache hits: $($Results.phases.baseline.cache_hits)"
}

# ============================================================================
# PHASE 3: EXACT CACHE TEST
# ============================================================================
if ($TestType -eq 'all' -or $TestType -eq 'exact') {
    Write-Host ""
    Write-Host "=" * 70 -ForegroundColor Cyan
    Write-Host "PHASE 3: EXACT CACHE TEST (Repeated Queries)" -ForegroundColor Cyan
    Write-Host "=" * 70 -ForegroundColor Cyan

    $exactCacheResults = @()

    # Repeat the same queries - should all hit exact cache
    Write-Log "Repeating $($uniqueQueries.Count) queries (should hit exact cache)..."

    $queryIndex = 0
    foreach ($query in $uniqueQueries) {
        $queryIndex++
        $displayQuery = if ($query.Length -gt 50) { $query.Substring(0, 50) + "..." } else { $query }
        Write-Log "[$queryIndex/$($uniqueQueries.Count)] $displayQuery"

        $result = Send-ChatQuery -Url $ServerUrl -Query $query
        $result.query = $query
        $exactCacheResults += $result

        if ($result.success) {
            $cacheStatus = if ($result.is_cache_hit) { "(CACHE HIT)" } else { "(MISS)" }
            $level = if ($result.is_cache_hit) { 'SUCCESS' } else { 'WARN' }
            Write-Log "  $cacheStatus Latency: $($result.latency_ms)ms" -Level $level
        }

        Start-Sleep -Milliseconds 50
    }

    $successfulExact = $exactCacheResults | Where-Object { $_.success }
    $exactHits = ($successfulExact | Where-Object { $_.is_cache_hit }).Count
    $exactHitRate = if ($successfulExact.Count -gt 0) { [math]::Round(($exactHits / $successfulExact.Count) * 100, 2) } else { 0 }

    $Results.phases.exact_cache = @{
        status = 'completed'
        total_queries = $exactCacheResults.Count
        successful_queries = $successfulExact.Count
        cache_hits = $exactHits
        cache_misses = $successfulExact.Count - $exactHits
        hit_rate_percent = $exactHitRate
        avg_hit_latency_ms = [math]::Round(($successfulExact | Where-Object { $_.is_cache_hit } | Measure-Object -Property latency_ms -Average).Average, 2)
        avg_miss_latency_ms = [math]::Round(($successfulExact | Where-Object { -not $_.is_cache_hit } | Measure-Object -Property latency_ms -Average).Average, 2)
    }

    $hitRateColor = if ($exactHitRate -ge 90) { 'Green' } elseif ($exactHitRate -ge 60) { 'Yellow' } else { 'Red' }
    Write-Host "Exact cache hit rate: $exactHitRate%" -ForegroundColor $hitRateColor
    Write-Log "Exact cache test complete: $exactHits/$($successfulExact.Count) hits ($exactHitRate%)" -Level 'SUCCESS'
}

# ============================================================================
# PHASE 4: SEMANTIC CACHE TEST
# ============================================================================
if ($TestType -eq 'all' -or $TestType -eq 'semantic') {
    Write-Host ""
    Write-Host "=" * 70 -ForegroundColor Cyan
    Write-Host "PHASE 4: SEMANTIC CACHE TEST (Paraphrased Queries)" -ForegroundColor Cyan
    Write-Host "=" * 70 -ForegroundColor Cyan

    $semanticResults = @()

    Write-Log "Testing semantic similarity matching..."

    foreach ($group in $TestData.semantic_groups) {
        Write-Log "Group: $($group.name)" -Level 'DEBUG'

        # Skip the first query (already in cache from baseline)
        # Test subsequent paraphrased queries
        $paraphrased = $group.queries | Select-Object -Skip 1

        foreach ($query in $paraphrased) {
            $displayQuery = if ($query.Length -gt 50) { $query.Substring(0, 50) + "..." } else { $query }
            Write-Log "  Testing: $displayQuery"

            $result = Send-ChatQuery -Url $ServerUrl -Query $query
            $result.query = $query
            $result.group = $group.name
            $result.expected_match = $group.expected_match
            $semanticResults += $result

            if ($result.success) {
                $cacheStatus = if ($result.is_cache_hit) { "(SEMANTIC HIT)" } else { "(MISS)" }
                $level = if ($result.is_cache_hit) { 'SUCCESS' } else { 'DEBUG' }
                Write-Log "    $cacheStatus Latency: $($result.latency_ms)ms" -Level $level
            }

            Start-Sleep -Milliseconds 50
        }
    }

    $successfulSemantic = $semanticResults | Where-Object { $_.success }
    $semanticHits = ($successfulSemantic | Where-Object { $_.is_cache_hit }).Count
    $semanticHitRate = if ($successfulSemantic.Count -gt 0) { [math]::Round(($semanticHits / $successfulSemantic.Count) * 100, 2) } else { 0 }

    $Results.phases.semantic_cache = @{
        status = 'completed'
        total_queries = $semanticResults.Count
        successful_queries = $successfulSemantic.Count
        semantic_hits = $semanticHits
        semantic_misses = $successfulSemantic.Count - $semanticHits
        hit_rate_percent = $semanticHitRate
        avg_hit_latency_ms = [math]::Round(($successfulSemantic | Where-Object { $_.is_cache_hit } | Measure-Object -Property latency_ms -Average).Average, 2)
        avg_miss_latency_ms = [math]::Round(($successfulSemantic | Where-Object { -not $_.is_cache_hit } | Measure-Object -Property latency_ms -Average).Average, 2)
        by_group = @{}
    }

    # Calculate per-group hit rates
    foreach ($group in $TestData.semantic_groups) {
        $groupResults = $successfulSemantic | Where-Object { $_.group -eq $group.name }
        $groupHits = ($groupResults | Where-Object { $_.is_cache_hit }).Count
        $groupRate = if ($groupResults.Count -gt 0) { [math]::Round(($groupHits / $groupResults.Count) * 100, 2) } else { 0 }
        $Results.phases.semantic_cache.by_group[$group.name] = @{
            queries = $groupResults.Count
            hits = $groupHits
            hit_rate = $groupRate
        }
    }

    $hitRateColor = if ($semanticHitRate -ge 60) { 'Green' } elseif ($semanticHitRate -ge 40) { 'Yellow' } else { 'Red' }
    Write-Host "Semantic cache hit rate: $semanticHitRate%" -ForegroundColor $hitRateColor
    Write-Log "Semantic cache test complete: $semanticHits/$($successfulSemantic.Count) hits ($semanticHitRate%)" -Level 'SUCCESS'
}

# ============================================================================
# PHASE 5: MIXED WORKLOAD
# ============================================================================
if ($TestType -eq 'all') {
    Write-Host ""
    Write-Host "=" * 70 -ForegroundColor Cyan
    Write-Host "PHASE 5: MIXED WORKLOAD (Realistic Usage)" -ForegroundColor Cyan
    Write-Host "=" * 70 -ForegroundColor Cyan

    $mixedResults = @()
    $mixedQueries = @()

    # Build a mixed workload:
    # - 40% repeated exact queries
    # - 30% semantically similar
    # - 30% new unique queries

    # Repeated exact (40%)
    $repeatCount = [math]::Ceiling($uniqueQueries.Count * 0.4)
    $mixedQueries += $uniqueQueries | Get-Random -Count $repeatCount

    # Semantically similar (30%)
    $semanticCount = [math]::Ceiling($uniqueQueries.Count * 0.3)
    foreach ($group in ($TestData.semantic_groups | Get-Random -Count $semanticCount)) {
        $mixedQueries += $group.queries | Get-Random
    }

    # New unique (30%)
    foreach ($category in $TestData.distinct_queries) {
        $mixedQueries += $category.queries | Get-Random -Count 2
    }

    # Shuffle
    $mixedQueries = $mixedQueries | Get-Random -Count $mixedQueries.Count

    Write-Log "Running $($mixedQueries.Count) mixed queries..."

    $queryIndex = 0
    foreach ($query in $mixedQueries) {
        $queryIndex++
        $displayQuery = if ($query.Length -gt 45) { $query.Substring(0, 45) + "..." } else { $query }
        Write-Log "[$queryIndex/$($mixedQueries.Count)] $displayQuery"

        $result = Send-ChatQuery -Url $ServerUrl -Query $query
        $result.query = $query
        $mixedResults += $result

        if ($result.success) {
            $cacheStatus = if ($result.is_cache_hit) { "(HIT)" } else { "(MISS)" }
            Write-Log "  $cacheStatus $($result.latency_ms)ms" -Level 'DEBUG'
        }

        Start-Sleep -Milliseconds 50
    }

    $successfulMixed = $mixedResults | Where-Object { $_.success }
    $mixedHits = ($successfulMixed | Where-Object { $_.is_cache_hit }).Count
    $mixedHitRate = if ($successfulMixed.Count -gt 0) { [math]::Round(($mixedHits / $successfulMixed.Count) * 100, 2) } else { 0 }

    $Results.phases.mixed_workload = @{
        status = 'completed'
        total_queries = $mixedResults.Count
        successful_queries = $successfulMixed.Count
        cache_hits = $mixedHits
        cache_misses = $successfulMixed.Count - $mixedHits
        hit_rate_percent = $mixedHitRate
        avg_latency_ms = [math]::Round(($successfulMixed | Measure-Object -Property latency_ms -Average).Average, 2)
    }

    Write-Log "Mixed workload complete: $mixedHitRate% hit rate" -Level 'SUCCESS'
}

# ============================================================================
# PHASE 6: STRESS TEST
# ============================================================================
if ($TestType -eq 'all' -or $TestType -eq 'stress') {
    Write-Host ""
    Write-Host "=" * 70 -ForegroundColor Cyan
    Write-Host "PHASE 6: STRESS TEST (Rapid Fire)" -ForegroundColor Cyan
    Write-Host "=" * 70 -ForegroundColor Cyan

    $stressResults = @()
    $stressQueries = $TestData.stress_test_queries * 3  # Repeat 3x

    Write-Log "Sending $($stressQueries.Count) queries in rapid succession..."

    $stressStart = Get-Date
    foreach ($query in $stressQueries) {
        $result = Send-ChatQuery -Url $ServerUrl -Query $query
        $result.query = $query
        $stressResults += $result
        # No delay - stress test
    }
    $stressEnd = Get-Date
    $stressDuration = ($stressEnd - $stressStart).TotalSeconds

    $successfulStress = $stressResults | Where-Object { $_.success }
    $stressHits = ($successfulStress | Where-Object { $_.is_cache_hit }).Count
    $throughput = [math]::Round($stressResults.Count / $stressDuration, 2)

    $Results.phases.stress_test = @{
        status = 'completed'
        total_queries = $stressResults.Count
        successful_queries = $successfulStress.Count
        failed_queries = $stressResults.Count - $successfulStress.Count
        cache_hits = $stressHits
        duration_seconds = [math]::Round($stressDuration, 2)
        throughput_qps = $throughput
        avg_latency_ms = [math]::Round(($successfulStress | Measure-Object -Property latency_ms -Average).Average, 2)
    }

    Write-Log "Stress test complete: $throughput queries/sec, $($successfulStress.Count)/$($stressResults.Count) successful" -Level 'SUCCESS'
}

# ============================================================================
# SUMMARY
# ============================================================================
Write-Host ""
Write-Host "=" * 70 -ForegroundColor Green
Write-Host "BENCHMARK SUMMARY" -ForegroundColor Green
Write-Host "=" * 70 -ForegroundColor Green

# Get final cache stats
$finalCacheStats = Get-CacheStats -Url $ServerUrl

# Calculate summary metrics
$totalQueries = 0
$totalHits = 0

if ($Results.phases.exact_cache.status -eq 'completed') {
    $totalQueries += $Results.phases.exact_cache.successful_queries
    $totalHits += $Results.phases.exact_cache.cache_hits
}
if ($Results.phases.semantic_cache.status -eq 'completed') {
    $totalQueries += $Results.phases.semantic_cache.successful_queries
    $totalHits += $Results.phases.semantic_cache.semantic_hits
}
if ($Results.phases.mixed_workload.status -eq 'completed') {
    $totalQueries += $Results.phases.mixed_workload.successful_queries
    $totalHits += $Results.phases.mixed_workload.cache_hits
}

$overallHitRate = if ($totalQueries -gt 0) { [math]::Round(($totalHits / $totalQueries) * 100, 2) } else { 0 }

# Cost savings calculation
$tokensSaved = $totalHits * $AvgTokensPerQuery
$costSavedInput = ($tokensSaved / 1000) * $CostPerKTokenInput
$costSavedOutput = ($tokensSaved / 1000) * $CostPerKTokenOutput
$totalCostSaved = $costSavedInput + $costSavedOutput

$Results.summary = @{
    total_queries = $totalQueries
    cache_hits = $totalHits
    cache_misses = $totalQueries - $totalHits
    overall_hit_rate_percent = $overallHitRate
    exact_hit_rate_percent = if ($Results.phases.exact_cache.hit_rate_percent) { $Results.phases.exact_cache.hit_rate_percent } else { 0 }
    semantic_hit_rate_percent = if ($Results.phases.semantic_cache.hit_rate_percent) { $Results.phases.semantic_cache.hit_rate_percent } else { 0 }
    tokens_saved = $tokensSaved
    estimated_cost_saved_usd = [math]::Round($totalCostSaved, 4)
    final_cache_entries = $finalCacheStats.entries
    final_cache_hit_rate = $finalCacheStats.hit_rate_percent
}

# Display summary table
Write-Host ""
Write-Host "  Metric                    | Value" -ForegroundColor White
Write-Host "  --------------------------+------------------" -ForegroundColor Gray
Write-Host "  Total Queries             | $totalQueries" -ForegroundColor White
Write-Host "  Cache Hits                | $totalHits" -ForegroundColor White
Write-Host "  Cache Misses              | $($totalQueries - $totalHits)" -ForegroundColor White
Write-Host ""

$hitRateColor = if ($overallHitRate -ge 60) { 'Green' } elseif ($overallHitRate -ge 40) { 'Yellow' } else { 'Red' }
Write-Host "  Overall Hit Rate          | $overallHitRate%" -ForegroundColor $hitRateColor

if ($Results.phases.exact_cache.status -eq 'completed') {
    $exactColor = if ($Results.phases.exact_cache.hit_rate_percent -ge 90) { 'Green' } elseif ($Results.phases.exact_cache.hit_rate_percent -ge 60) { 'Yellow' } else { 'Red' }
    Write-Host "  Exact Cache Hit Rate      | $($Results.phases.exact_cache.hit_rate_percent)%" -ForegroundColor $exactColor
}

if ($Results.phases.semantic_cache.status -eq 'completed') {
    $semanticColor = if ($Results.phases.semantic_cache.hit_rate_percent -ge 60) { 'Green' } elseif ($Results.phases.semantic_cache.hit_rate_percent -ge 40) { 'Yellow' } else { 'Red' }
    Write-Host "  Semantic Cache Hit Rate   | $($Results.phases.semantic_cache.hit_rate_percent)%" -ForegroundColor $semanticColor
}

Write-Host ""
Write-Host "  Tokens Saved              | $tokensSaved" -ForegroundColor Cyan
Write-Host "  Est. Cost Saved (GPT-4)   | `$$([math]::Round($totalCostSaved, 4))" -ForegroundColor Cyan
Write-Host ""

# Target comparison
Write-Host "  TARGET COMPARISON" -ForegroundColor Yellow
Write-Host "  --------------------------+------------------" -ForegroundColor Gray

$targetSemanticHitRate = 60
$targetExactHitRate = 95
$targetCacheLatency = 5

$semanticStatus = if ($Results.phases.semantic_cache.hit_rate_percent -ge $targetSemanticHitRate) { "[PASS]" } else { "[FAIL]" }
$semanticStatusColor = if ($Results.phases.semantic_cache.hit_rate_percent -ge $targetSemanticHitRate) { 'Green' } else { 'Red' }
Write-Host "  Semantic Hit Rate >= 60%  | $semanticStatus $($Results.phases.semantic_cache.hit_rate_percent)%" -ForegroundColor $semanticStatusColor

$exactStatus = if ($Results.phases.exact_cache.hit_rate_percent -ge $targetExactHitRate) { "[PASS]" } else { "[FAIL]" }
$exactStatusColor = if ($Results.phases.exact_cache.hit_rate_percent -ge $targetExactHitRate) { 'Green' } else { 'Red' }
Write-Host "  Exact Hit Rate >= 95%     | $exactStatus $($Results.phases.exact_cache.hit_rate_percent)%" -ForegroundColor $exactStatusColor

if ($Results.phases.exact_cache.avg_hit_latency_ms) {
    $latencyStatus = if ($Results.phases.exact_cache.avg_hit_latency_ms -le $targetCacheLatency) { "[PASS]" } else { "[FAIL]" }
    $latencyStatusColor = if ($Results.phases.exact_cache.avg_hit_latency_ms -le $targetCacheLatency) { 'Green' } else { 'Red' }
    Write-Host "  Cache Latency <= 5ms      | $latencyStatus $($Results.phases.exact_cache.avg_hit_latency_ms)ms" -ForegroundColor $latencyStatusColor
}

Write-Host ""

# Save results
$resultsJsonPath = Join-Path $ResultsDir "benchmark_results_$Timestamp.json"
$resultsMdPath = Join-Path $ResultsDir "benchmark_results_$Timestamp.md"
$latestJsonPath = Join-Path $ResultsDir "latest_results.json"

$Results | ConvertTo-Json -Depth 10 | Out-File -FilePath $resultsJsonPath -Encoding UTF8
$Results | ConvertTo-Json -Depth 10 | Out-File -FilePath $latestJsonPath -Encoding UTF8

# Generate markdown report
$mdReport = @"
# rigrun Benchmark Results

**Date:** $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')
**Server:** $ServerUrl
**Platform:** $([System.Environment]::OSVersion.VersionString)

## Summary

| Metric | Value | Target | Status |
|--------|-------|--------|--------|
| Overall Hit Rate | $($Results.summary.overall_hit_rate_percent)% | 60%+ | $(if ($Results.summary.overall_hit_rate_percent -ge 60) { 'PASS' } else { 'FAIL' }) |
| Exact Cache Hit Rate | $($Results.summary.exact_hit_rate_percent)% | 95%+ | $(if ($Results.summary.exact_hit_rate_percent -ge 95) { 'PASS' } else { 'FAIL' }) |
| Semantic Cache Hit Rate | $($Results.summary.semantic_hit_rate_percent)% | 60%+ | $(if ($Results.summary.semantic_hit_rate_percent -ge 60) { 'PASS' } else { 'FAIL' }) |
| Tokens Saved | $($Results.summary.tokens_saved) | - | - |
| Est. Cost Saved | `$$($Results.summary.estimated_cost_saved_usd) | - | - |

## Phase Results

### Baseline
- Total Queries: $($Results.phases.baseline.total_queries)
- Avg Latency: $($Results.phases.baseline.avg_latency_ms)ms

### Exact Cache Test
- Queries: $($Results.phases.exact_cache.total_queries)
- Hits: $($Results.phases.exact_cache.cache_hits)
- Hit Rate: $($Results.phases.exact_cache.hit_rate_percent)%
- Avg Hit Latency: $($Results.phases.exact_cache.avg_hit_latency_ms)ms

### Semantic Cache Test
- Queries: $($Results.phases.semantic_cache.total_queries)
- Semantic Hits: $($Results.phases.semantic_cache.semantic_hits)
- Hit Rate: $($Results.phases.semantic_cache.hit_rate_percent)%
- Avg Hit Latency: $($Results.phases.semantic_cache.avg_hit_latency_ms)ms

### Mixed Workload
- Queries: $($Results.phases.mixed_workload.total_queries)
- Hit Rate: $($Results.phases.mixed_workload.hit_rate_percent)%

### Stress Test
- Queries: $($Results.phases.stress_test.total_queries)
- Throughput: $($Results.phases.stress_test.throughput_qps) qps
- Avg Latency: $($Results.phases.stress_test.avg_latency_ms)ms

## Cost Savings Projection

Based on GPT-4 pricing (`$0.03/1K input, `$0.06/1K output):

| Scale | Monthly Queries | Tokens Saved | Monthly Savings |
|-------|-----------------|--------------|-----------------|
| Light | 10,000 | $(10000 * $Results.summary.overall_hit_rate_percent / 100 * $AvgTokensPerQuery) | `$$([math]::Round(10000 * $Results.summary.overall_hit_rate_percent / 100 * $AvgTokensPerQuery / 1000 * 0.045, 2)) |
| Medium | 100,000 | $(100000 * $Results.summary.overall_hit_rate_percent / 100 * $AvgTokensPerQuery) | `$$([math]::Round(100000 * $Results.summary.overall_hit_rate_percent / 100 * $AvgTokensPerQuery / 1000 * 0.045, 2)) |
| Heavy | 1,000,000 | $(1000000 * $Results.summary.overall_hit_rate_percent / 100 * $AvgTokensPerQuery) | `$$([math]::Round(1000000 * $Results.summary.overall_hit_rate_percent / 100 * $AvgTokensPerQuery / 1000 * 0.045, 2)) |

---
*Generated by rigrun benchmark suite v1.0.0*
"@

$mdReport | Out-File -FilePath $resultsMdPath -Encoding UTF8

Write-Log "Results saved to:" -Level 'SUCCESS'
Write-Log "  JSON: $resultsJsonPath"
Write-Log "  Markdown: $resultsMdPath"

Write-Host ""
Write-Host "Benchmark complete!" -ForegroundColor Green
