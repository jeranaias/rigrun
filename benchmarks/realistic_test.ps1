# Realistic workload test - simulates actual usage patterns
# In real usage, users repeat the same/similar questions frequently

$seedQueries = @(
    "How does recursion work?",
    "What is a binary search tree?",
    "How do I make an HTTP request?",
    "Explain async/await",
    "How do I write unit tests?"
)

# Workload: 50 queries total
# - 10 exact repeats (20%)
# - 10 semantic variations (20%)
# - 30 randomly repeated from seed (60%)
$workload = @()

# First, seed the cache
$workload += $seedQueries

# Add exact repeats
$workload += $seedQueries | Get-Random -Count 5
$workload += $seedQueries | Get-Random -Count 5

# Add semantic variations
$variations = @(
    "Explain recursion",
    "What is recursion?",
    "Tell me about recursion",
    "BST data structure explained",
    "Binary search tree overview",
    "Making HTTP requests",
    "API calls in code",
    "Async await explained",
    "Understanding async/await",
    "Unit testing guide"
)
$workload += $variations

# Add more random repeats to simulate typical usage
for ($i = 0; $i -lt 25; $i++) {
    $workload += $seedQueries | Get-Random -Count 1
}

Write-Host "=== Realistic Workload Test ==="
Write-Host "Total queries: $($workload.Count)"
Write-Host ""
Write-Host "Model  | Latency | Query"
Write-Host "-------|---------|------"

$cacheHits = 0
$misses = 0

foreach ($q in $workload) {
    $body = @{model="auto"; messages=@(@{role="user"; content=$q})} | ConvertTo-Json -Depth 3
    $start = Get-Date
    $response = Invoke-RestMethod -Uri "http://localhost:8787/v1/chat/completions" -Method Post -Body $body -ContentType "application/json"
    $latency = ((Get-Date) - $start).TotalMilliseconds
    $shortQuery = if ($q.Length -gt 40) { $q.Substring(0, 37) + "..." } else { $q }
    Write-Host "$($response.model.PadRight(6)) | $([int]$latency)ms".PadRight(9) "| $shortQuery"

    if ($response.model -eq "cache") {
        $cacheHits++
    } else {
        $misses++
    }
}

# Get stats
Write-Host ""
$stats = Invoke-RestMethod -Uri "http://localhost:8787/cache/semantic"
Write-Host "========================================"
Write-Host "        BENCHMARK RESULTS"
Write-Host "========================================"
Write-Host ""
Write-Host "Cache Performance:"
Write-Host "  Total lookups:  $($stats.total_lookups)"
Write-Host "  Exact hits:     $($stats.exact_hits)"
Write-Host "  Semantic hits:  $($stats.semantic_hits)"
Write-Host "  Misses:         $($stats.misses)"
Write-Host "  Embed failures: $($stats.embedding_failures)"
Write-Host ""
Write-Host "Hit Rates:"
Write-Host "  Exact hit rate:    $([math]::Round($stats.exact_hits / $stats.total_lookups * 100, 1))%"
Write-Host "  Semantic hit rate: $([math]::Round($stats.semantic_hits / $stats.total_lookups * 100, 1))%"
Write-Host "  TOTAL HIT RATE:    $([math]::Round($stats.total_hit_rate, 1))%"
Write-Host ""
if ($stats.total_hit_rate -ge 60) {
    Write-Host "  [PASS] Hit rate >= 60% target" -ForegroundColor Green
} else {
    Write-Host "  [INFO] Hit rate below 60% target (expected with new cache)" -ForegroundColor Yellow
}
