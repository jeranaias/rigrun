# rigrun_agent.ps1 - An agent that uses rigrun to think and build code
# This is rigrun building rigrun FOR REAL

param(
    [Parameter(Mandatory=$true)]
    [string]$Task
)

$RIGRUN_URL = "http://localhost:8787/v1/chat/completions"

function Ask-Rigrun {
    param([string]$Prompt, [string]$SystemPrompt = "You are a helpful coding assistant. Be concise.")

    $body = @{
        model = "local"  # Use local model via rigrun
        messages = @(
            @{ role = "system"; content = $SystemPrompt }
            @{ role = "user"; content = $Prompt }
        )
    } | ConvertTo-Json -Depth 3

    try {
        $response = Invoke-RestMethod -Uri $RIGRUN_URL -Method POST -Body $body -ContentType "application/json" -TimeoutSec 300
        return $response.choices[0].message.content
    } catch {
        Write-Host "ERROR calling rigrun: $($_.Exception.Message)" -ForegroundColor Red
        return $null
    }
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host " rigrun Agent - Building with rigrun" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Task: $Task" -ForegroundColor Yellow
Write-Host ""

# Step 1: Ask rigrun to plan the task
Write-Host "[1/4] Asking rigrun to plan..." -ForegroundColor Cyan
$plan = Ask-Rigrun -Prompt "I need to: $Task

Create a brief plan with 3-5 steps. Be specific and actionable." -SystemPrompt "You are a software architect. Create concise plans."

if ($plan) {
    Write-Host "Plan from rigrun:" -ForegroundColor Green
    Write-Host $plan
    Write-Host ""
}

# Step 2: Ask rigrun to write the code
Write-Host "[2/4] Asking rigrun to write code..." -ForegroundColor Cyan
$code = Ask-Rigrun -Prompt "Based on this plan:
$plan

Write the code to implement this. Output ONLY the code, no explanations.
Task: $Task" -SystemPrompt "You are a Rust expert. Write clean, working code. Output only code."

if ($code) {
    Write-Host "Code from rigrun:" -ForegroundColor Green
    Write-Host $code
    Write-Host ""
}

# Step 3: Ask rigrun to review
Write-Host "[3/4] Asking rigrun to review..." -ForegroundColor Cyan
$review = Ask-Rigrun -Prompt "Review this code for bugs and improvements:
$code

List any issues found." -SystemPrompt "You are a code reviewer. Be critical but constructive."

if ($review) {
    Write-Host "Review from rigrun:" -ForegroundColor Green
    Write-Host $review
    Write-Host ""
}

# Step 4: Get final stats
Write-Host "[4/4] Final rigrun stats:" -ForegroundColor Cyan
$stats = Invoke-RestMethod -Uri "http://localhost:8787/stats"
Write-Host "  Session Queries: $($stats.session.queries)"
Write-Host "  Local: $($stats.session.local_queries) | Cloud: $($stats.session.cloud_queries)"
Write-Host "  Money Saved: `$$([math]::Round($stats.today.saved_usd, 2))" -ForegroundColor Green
Write-Host ""
Write-Host "Done! rigrun completed the task using local LLM." -ForegroundColor Cyan
