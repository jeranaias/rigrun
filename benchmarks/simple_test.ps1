# Simple semantic cache benchmark
$queries = @(
    # Seed queries (will populate cache)
    "How does recursion work?",
    "What is a binary search tree?",
    "How do I make an HTTP request in Python?",
    "Explain async/await",
    "How do I write unit tests?",

    # Exact repeats (should be exact hits)
    "How does recursion work?",
    "What is a binary search tree?",
    "How do I make an HTTP request in Python?",
    "Explain async/await",
    "How do I write unit tests?",

    # Semantic variations (should be semantic hits)
    "Explain recursion to me",
    "Describe a BST data structure",
    "How to call an API in Python?",
    "What is async await in JavaScript?",
    "How do I test my code?"
)

$results = @()
Write-Host "Running 20 queries..."
Write-Host ""
Write-Host "Model  | Latency | Query"
Write-Host "-------|---------|------"

foreach ($q in $queries) {
    $body = @{model="auto"; messages=@(@{role="user"; content=$q})} | ConvertTo-Json -Depth 3
    $start = Get-Date
    $response = Invoke-RestMethod -Uri "http://localhost:8787/v1/chat/completions" -Method Post -Body $body -ContentType "application/json"
    $latency = ((Get-Date) - $start).TotalMilliseconds
    $results += @{query=$q; model=$response.model; latency=$latency}
    $shortQuery = if ($q.Length -gt 45) { $q.Substring(0, 42) + "..." } else { $q }
    Write-Host "$($response.model.PadRight(6)) | $([int]$latency)ms".PadRight(9) "| $shortQuery"
}

# Get stats
Write-Host ""
$stats = Invoke-RestMethod -Uri "http://localhost:8787/cache/semantic"
Write-Host "=== Cache Stats ==="
Write-Host "Total lookups:  $($stats.total_lookups)"
Write-Host "Exact hits:     $($stats.exact_hits)"
Write-Host "Semantic hits:  $($stats.semantic_hits)"
Write-Host "Misses:         $($stats.misses)"
Write-Host "Embed failures: $($stats.embedding_failures)"
Write-Host ""
Write-Host "TOTAL HIT RATE: $($stats.total_hit_rate)%"
