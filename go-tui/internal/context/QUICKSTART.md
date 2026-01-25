# Quick Start: Smart Context Truncation

Get started with Smart Context Truncation in 5 minutes!

## 🚀 Minimal Example

```go
package main

import (
    "context"
    "fmt"

    "github.com/jeranaias/rigrun-tui/internal/context"
    "github.com/jeranaias/rigrun-tui/internal/model"
    "github.com/jeranaias/rigrun-tui/internal/ollama"
)

func main() {
    // 1. Create your conversation
    conv := model.NewConversation()
    conv.SystemPrompt = "You are a helpful assistant"

    // Add messages...
    for i := 0; i < 60; i++ {
        conv.AddUserMessage("User message " + fmt.Sprint(i))
        conv.AddAssistantMessage()
    }

    // 2. Create Ollama client
    client := ollama.NewClient()

    // 3. Create summarizer
    summarizer := context.NewLLMSummarizer(&context.SummarizerConfig{
        Client: client,
        Model:  "qwen2.5-coder:7b", // Fast model
    })

    // 4. Create truncator
    truncator := context.NewConversationTruncator(&context.TruncatorConfig{
        MaxFullMessages:  20,  // Keep 20 recent
        SummaryThreshold: 50,  // Trigger at 50
        Summarizer:       summarizer,
    })

    // 5. Truncate!
    result, err := truncator.Truncate(context.Background(), conv)
    if err != nil {
        panic(err)
    }

    // 6. Use the result
    if result.WasTruncated {
        fmt.Println("✓ Truncated:", result.SummaryInfo())
        fmt.Println("Summary:", result.Summary)
    }

    // 7. Get messages for LLM
    messages := result.ToOllamaMessages()
    fmt.Printf("Sending %d messages to LLM (was %d)\n",
        len(messages), result.TotalMessages)
}
```

## 📦 Even Simpler: Managed Mode

```go
// Create once
manager := context.NewTruncatedConversationManager(conv, truncator)

// Use everywhere
messages, err := manager.GetMessagesForLLM(ctx)

// Check status
if manager.IsTruncated() {
    fmt.Println("Summary:", manager.GetSummary())
}
```

## 🎯 Three Steps Integration

### Step 1: Setup (Once)

```go
// In your initialization code
var (
    ollamaClient *ollama.Client
    truncator    *context.ConversationTruncator
)

func init() {
    ollamaClient = ollama.NewClient()

    summarizer := context.NewLLMSummarizer(&context.SummarizerConfig{
        Client: ollamaClient,
        Model:  "qwen2.5-coder:7b",
    })

    truncator = context.NewConversationTruncator(&context.TruncatorConfig{
        MaxFullMessages:  20,
        SummaryThreshold: 50,
        Summarizer:       summarizer,
    })
}
```

### Step 2: Use Before LLM Call

```go
func sendToLLM(conv *model.Conversation) {
    // Check if truncation needed
    if truncator.ShouldTruncate(conv) {
        result, _ := truncator.Truncate(context.Background(), conv)
        messages := result.ToOllamaMessages()

        // Show indicator in UI
        showTruncationIndicator(result.SummaryInfo())
    } else {
        messages := conv.ToOllamaMessages()
    }

    // Send to LLM...
}
```

### Step 3: Show Indicator (Optional)

```go
func showTruncationIndicator(info string) {
    // In your UI code
    statusBar.SetText("🗜️ " + info)

    // Or in terminal
    fmt.Println("💡 " + info)
}
```

## 🔧 Common Configurations

### For Quick Chats
```go
config := &context.TruncatorConfig{
    MaxFullMessages:  25,  // Keep more context
    SummaryThreshold: 60,  // Trigger later
    Summarizer:       summarizer,
}
```

### For Long Sessions
```go
config := &context.TruncatorConfig{
    MaxFullMessages:  15,  // Aggressive truncation
    SummaryThreshold: 40,  // Trigger earlier
    Summarizer:       summarizer,
}
```

### Fallback Mode (No LLM)
```go
config := &context.TruncatorConfig{
    MaxFullMessages:  20,
    SummaryThreshold: 50,
    Summarizer:       context.NewSimpleSummarizer(), // No LLM needed
}
```

## 💡 Pro Tips

### 1. Check First, Truncate Only When Needed

```go
if truncator.ShouldTruncate(conv) {
    // Only truncate if needed
    result, _ := truncator.Truncate(ctx, conv)
    // ...
}
```

### 2. Estimate Benefit

```go
saved, percent := context.EstimateTruncationBenefit(conv, truncator)
fmt.Printf("Will save ~%d tokens (%.1f%%)\n", saved, percent)
```

### 3. Handle Errors Gracefully

```go
result, err := truncator.Truncate(ctx, conv)
if err != nil {
    // Fall back to original conversation
    messages := conv.ToOllamaMessages()
} else {
    messages := result.ToOllamaMessages()
}
```

### 4. Stream Summaries for Better UX

```go
summarizer := context.NewStreamingSummarizer(config)

summary, _ := summarizer.SummarizeStream(ctx, messages, func(chunk string) {
    // Update UI in real-time
    ui.AppendToSummary(chunk)
})
```

## 🎨 UI Integration Examples

### Terminal UI

```go
if result.WasTruncated {
    fmt.Println()
    fmt.Println("╭─────────────────────────────────╮")
    fmt.Printf("│ 🗜️  Conversation Summarized     │\n")
    fmt.Printf("│ %s              │\n", result.SummaryInfo())
    fmt.Println("╰─────────────────────────────────╯")
    fmt.Println()
}
```

### Status Bar

```go
if manager.IsTruncated() {
    statusLine := fmt.Sprintf("📝 %d msgs | 🗜️ %s",
        len(conv.Messages),
        manager.GetTruncationInfo())
    ui.SetStatus(statusLine)
}
```

### Dialog/Modal

```go
if userWantsToSeeSummary {
    dialog.Show(
        "Previous Conversation",
        manager.GetSummary(),
        dialog.InfoIcon,
    )
}
```

## 🧪 Testing Your Integration

```go
func TestTruncation(t *testing.T) {
    // Create test conversation
    conv := model.NewConversation()
    for i := 0; i < 60; i++ {
        conv.AddUserMessage("Test")
        conv.AddAssistantMessage()
    }

    // Create truncator
    truncator := context.NewConversationTruncator(&context.TruncatorConfig{
        MaxFullMessages:  20,
        SummaryThreshold: 50,
        Summarizer:       context.NewSimpleSummarizer(),
    })

    // Test truncation
    result, err := truncator.Truncate(context.Background(), conv)

    // Verify
    if err != nil {
        t.Fatal(err)
    }
    if !result.WasTruncated {
        t.Error("Expected truncation")
    }
    if len(result.RecentMessages) != 20 {
        t.Error("Expected 20 recent messages")
    }
}
```

## 📚 Next Steps

1. Read [TRUNCATION_README.md](TRUNCATION_README.md) for full documentation
2. Check [example_integration.go](example_integration.go) for more patterns
3. Review [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) for architecture details
4. Run benchmarks: `go test ./internal/context/ -bench=.`

## 🆘 Troubleshooting

### "Summaries too long"

```go
// Use lower NumPredict
client.ChatWithOptions(ctx, model, msgs, &ollama.Options{
    NumPredict: 300, // Limit to 300 tokens
})
```

### "Too much truncated"

```go
// Keep more messages
config.MaxFullMessages = 30
```

### "Triggers too early"

```go
// Increase threshold
config.SummaryThreshold = 75
```

### "LLM unavailable"

```go
// Use simple summarizer
config.Summarizer = context.NewSimpleSummarizer()
```

## ✅ Checklist

- [ ] Imported context package
- [ ] Created Ollama client
- [ ] Created summarizer
- [ ] Created truncator
- [ ] Added truncation before LLM calls
- [ ] Added UI indicator
- [ ] Tested with >50 messages
- [ ] Handles errors gracefully

Done! You're ready to use Smart Context Truncation! 🎉
