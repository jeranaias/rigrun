# Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
# SPDX-License-Identifier: AGPL-3.0-or-later

$queries = @(
    "What is recursion in programming?",
    "Explain how a for loop works",
    "What is a variable in code?",
    "How do functions work in programming?",
    "What is an array data structure?",
    "Explain object oriented programming",
    "What is inheritance in OOP?",
    "How does polymorphism work?",
    "What is encapsulation?",
    "Explain abstraction in software"
)

Write-Host "Firing 10 queries at rigrun..." -ForegroundColor Cyan

foreach ($i in 0..($queries.Count - 1)) {
    $q = $queries[$i]
    $body = @{
        model = "local"
        messages = @(@{ role = "user"; content = $q })
    } | ConvertTo-Json -Depth 3

    Write-Host "[$($i+1)/10] $q" -ForegroundColor Yellow
    $start = Get-Date

    try {
        $response = Invoke-RestMethod -Uri "http://localhost:8787/v1/chat/completions" -Method POST -Body $body -ContentType "application/json" -TimeoutSec 120
        $elapsed = ((Get-Date) - $start).TotalSeconds
        $answer = $response.choices[0].message.content
        if ($answer.Length -gt 80) { $answer = $answer.Substring(0, 80) + "..." }
        Write-Host "  [$([math]::Round($elapsed, 1))s] $answer" -ForegroundColor Green
    } catch {
        Write-Host "  ERROR: $($_.Exception.Message)" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Done! Check rigrun stats:" -ForegroundColor Cyan
$stats = Invoke-RestMethod -Uri "http://localhost:8787/stats"
Write-Host "  Session Queries: $($stats.session.queries)"
Write-Host "  Local: $($stats.session.local_queries) | Cloud: $($stats.session.cloud_queries)"
Write-Host "  Today Saved: `$$($stats.today.saved_usd) | Spent: `$$($stats.today.spent_usd)"
