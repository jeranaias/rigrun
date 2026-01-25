# Cost Tracking and Analytics

This package provides comprehensive cost tracking and analytics for rigrun's intelligent routing system.

## Features

### Cost Tracker (`cost.go`)

The `CostTracker` tracks token usage and costs across sessions:

- **Session-based tracking**: Each session tracks costs independently
- **Multi-tier support**: Tracks usage across cache, local, and cloud tiers
- **Top queries**: Maintains top 10 most expensive queries per session
- **Savings calculation**: Compares actual costs vs all-Opus pricing
- **Persistent storage**: All cost data is saved to disk

#### Usage

```go
// Create a cost tracker
tracker, err := telemetry.NewCostTracker("~/.rigrun/costs")
if err != nil {
    log.Fatal(err)
}

// Record a query
tracker.RecordQuery(
    "cloud",           // tier
    500,              // input tokens
    1500,             // output tokens
    2*time.Second,    // duration
    "Explain caching" // prompt (truncated to 100 chars)
)

// Get current session stats
session := tracker.GetCurrentSession()
fmt.Printf("Total cost: $%.4f\n", session.TotalCost)
fmt.Printf("Savings: $%.4f\n", session.Savings)

// Get historical trends
trends := tracker.GetTrends(7) // Last 7 days
fmt.Printf("Total cost (7d): $%.4f\n", trends.TotalCost)
```

### Cost Storage (`cost_storage.go`)

The `CostStorage` handles persistent storage of cost data:

- **JSON-based storage**: Human-readable format
- **Date-range filtering**: Efficient session listing
- **Automatic cleanup**: Optional deletion of old data
- **Size tracking**: Monitor storage usage

#### Storage Location

Cost data is stored in `~/.rigrun/costs/` by default. Each session is saved as:

```
~/.rigrun/costs/20240315-143022.json
```

The filename format is `YYYYMMDD-HHMMSS.json` based on session start time.

#### File Format

```json
{
  "id": "20240315-143022",
  "start_time": "2024-03-15T14:30:22Z",
  "end_time": "2024-03-15T15:30:22Z",
  "cache_tokens": {
    "input": 100,
    "output": 300
  },
  "local_tokens": {
    "input": 200,
    "output": 600
  },
  "cloud_tokens": {
    "input": 500,
    "output": 1500
  },
  "total_cost": 0.0234,
  "savings": 0.1456,
  "top_queries": [
    {
      "timestamp": "2024-03-15T14:35:22Z",
      "prompt": "Write a complex algorithm...",
      "tier": "cloud",
      "input_tokens": 500,
      "output_tokens": 1500,
      "cost": 0.0234,
      "duration": 2000000000
    }
  ]
}
```

## Cost Dashboard

The cost dashboard provides three views:

### 1. Summary View (Default)

Shows current session statistics:
- Session ID and duration
- Total cost and savings
- Cost efficiency percentage
- Token usage breakdown by tier
- Top 5 most expensive queries

**Command**: `/cost` or `/cost summary`

### 2. History View

Shows cost trends over time:
- Last 7 days by default
- Total cost and savings
- Daily breakdown with bar chart
- Query counts per day

**Command**: `/cost history`

### 3. Breakdown View

Shows detailed tier breakdown:
- Last 30 days by default
- Cost distribution (cache/local/cloud)
- Percentage breakdown
- Total savings vs all-Opus

**Command**: `/cost breakdown`

## Integration

### Adding to Your Application

```go
import "github.com/jeranaias/rigrun-tui/internal/telemetry"

// Initialize cost tracker
tracker, err := telemetry.NewCostTracker("")
if err != nil {
    log.Fatal(err)
}

// Add to command context
ctx := commands.NewContext(cfg, ollama, store, session, cache)
ctx.CostTracker = tracker

// Record queries after each inference
tracker.RecordQuery(tier, inputTokens, outputTokens, duration, prompt)

// Periodically save session
go func() {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        tracker.SaveCurrentSession()
    }
}()
```

### Displaying the Dashboard

```go
import "github.com/jeranaias/rigrun-tui/internal/ui/components"

// Create dashboard
dashboard := components.NewCostDashboard(tracker)
dashboard.SetSize(width, height)

// Set view based on command
switch view {
case "history":
    dashboard.SetView(components.ViewHistory)
case "breakdown":
    dashboard.SetView(components.ViewBreakdown)
default:
    dashboard.SetView(components.ViewSummary)
}

// Render
output := dashboard.View()
fmt.Println(output)
```

## Pricing Model

The cost tracker uses the following pricing (per 1K tokens):

### Cache Tier (Free)
- Input: $0.00
- Output: $0.00

### Local Tier (Free)
- Input: $0.00
- Output: $0.00

### Cloud Tier
- Input: $0.03 (0.03 cents = $0.0003)
- Output: $0.15 (0.15 cents = $0.0015)

### Reference: Opus (for savings calculation)
- Input: $1.50 (1.5 cents = $0.015)
- Output: $7.50 (7.5 cents = $0.075)

All costs are calculated in cents then converted to dollars for display.

## Testing

Run tests with:

```bash
go test ./internal/telemetry/...
go test ./internal/ui/components/cost_dashboard_test.go
```

## Performance Considerations

- **Memory usage**: Top queries are capped at 10 per session
- **Storage**: Each session file is typically < 5KB
- **Concurrency**: All operations are thread-safe with RWMutex
- **Cleanup**: Use `DeleteBefore()` to remove old sessions

## Future Enhancements

Possible improvements:
- Export to CSV/Excel
- Cost alerts and budgets
- Per-user cost tracking
- Model-specific cost breakdown
- Real-time cost streaming
- Integration with billing systems
