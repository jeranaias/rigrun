# Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
# SPDX-License-Identifier: AGPL-3.0-or-later

$prompt = "Write a Python function to check if a number is prime."

$models = @("qwen2.5:14b", "mistral-small", "codestral:22b", "gemma2:27b")

Write-Host "============================================="
Write-Host "Model Benchmark - RX 9070 XT with Vulkan"
Write-Host "============================================="
Write-Host ""

foreach ($model in $models) {
    Write-Host "Testing $model..."
    $start = Get-Date

    $output = ollama run $model $prompt 2>&1

    $elapsed = (Get-Date) - $start
    Write-Host $output
    Write-Host ""
    Write-Host ">>> $model Time: $($elapsed.TotalSeconds) seconds"
    Write-Host "============================================="
    Write-Host ""
}

Write-Host "Benchmark complete!"
