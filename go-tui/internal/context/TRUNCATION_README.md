# Smart Context Truncation

## Overview

The Smart Context Truncation feature (Feature 4.1) ensures that long conversations don't slow down or fail by automatically summarizing old messages while keeping recent messages in full. This maintains constant performance as conversations grow.

## Features

### Core Capabilities

- **Automatic Truncation**: Triggers after ~50 messages
- **Intelligent Summarization**: LLM-based summaries that preserve key facts
- **Constant Performance**: Processing time stays consistent regardless of conversation length
- **Flexible Configuration**: Customizable thresholds and behavior
- **Fallback Support**: Simple summarization when LLM is unavailable

### What's Preserved in Summaries

- File paths and code locations discussed
- Key decisions made
- Errors encountered and their resolutions
- Important context needed to continue the conversation
- Implementation details and technical specifics

## Architecture

### Components

1. **ConversationTruncator** (`truncation.go`)
   - Manages truncation logic
   - Decides when to summarize
   - Coordinates between original and truncated messages

2. **Summarizer Interface** (`summarizer.go`)
   - Defines summarization contract
   - Implementations:
     - `LLMSummarizer`: Uses AI for intelligent summaries
     - `SimpleSummarizer`: Fallback for count-based summaries
     - `StreamingSummarizer`: Real-time summarization with progress

3. **TruncateResult** (`truncation.go`)
   - Contains truncated conversation
   - Tracks summarized vs. full messages
   - Provides conversion to Ollama format

## Usage

### Basic Usage

```go
import (
    "context"
    "github.com/jeranaias/rigrun-tui/internal/context"
    "github.com/jeranaias/rigrun-tui/internal/model"
    "github.com/jeranaias/rigrun-tui/internal/ollama"
)

// Create summarizer
client := ollama.NewClient()
summarizer := context.NewLLMSummarizer(&context.SummarizerConfig{
    Model:  "qwen2.5-coder:7b",
    Client: client,
})

// Create truncator
truncator := context.NewConversationTruncator(&context.TruncatorConfig{
    MaxFullMessages:  20,  // Keep 20 recent messages
    SummaryThreshold: 50,  // Summarize after 50 messages
    Summarizer:       summarizer,
})

// Check if truncation needed
if truncator.ShouldTruncate(conversation) {
    result, err := truncator.Truncate(ctx, conversation)
    if err != nil {
        // Handle error
    }

    // Use truncated result
    messages := result.ToOllamaMessages()
}
```

### With Automatic Management

```go
// Create manager that handles truncation automatically
manager := context.NewTruncatedConversationManager(conversation, truncator)

// Get optimized messages (automatically truncates if needed)
messages, err := manager.GetMessagesForLLM(ctx)

// Check if truncation occurred
if manager.IsTruncated() {
    fmt.Println("Truncation info:", manager.GetTruncationInfo())
    fmt.Println("Summary:", manager.GetSummary())
}
```

### Streaming Summarization

```go
// Create streaming summarizer
summarizer := context.NewStreamingSummarizer(&context.SummarizerConfig{
    Model:  "qwen2.5-coder:7b",
    Client: client,
})

// Stream summary with progress updates
summary, err := summarizer.SummarizeStream(ctx, oldMessages, func(chunk string) {
    // Update UI with streaming chunk
    fmt.Print(chunk)
})
```

### Fallback Pattern

```go
// Try LLM summarization, fall back to simple on error
result, err := context.TruncateWithFallback(ctx, conversation, client)
if err != nil {
    // Handle error
}
```

## Configuration

### TruncatorConfig

```go
type TruncatorConfig struct {
    // MaxFullMessages: Number of recent messages to keep in full
    // Default: 20
    MaxFullMessages int

    // SummaryThreshold: Message count that triggers summarization
    // Default: 50
    SummaryThreshold int

    // Summarizer: Implementation to use for creating summaries
    Summarizer Summarizer
}
```

### Recommended Settings

| Scenario | MaxFullMessages | SummaryThreshold | Model |
|----------|----------------|------------------|-------|
| Development chat | 20 | 50 | qwen2.5-coder:7b |
| Long debugging | 15 | 40 | qwen2.5-coder:7b |
| Quick questions | 25 | 60 | qwen2.5-coder:7b |
| Very long sessions | 10 | 30 | qwen2.5-coder:7b |

## Performance

### Benefits

- **Token Savings**: Typically 80-90% reduction in old message tokens
- **Constant Latency**: Response time doesn't increase with conversation length
- **Memory Efficiency**: Fewer tokens = less memory usage
- **Context Window**: Maximizes available space for actual content

### Example Metrics

```
Conversation: 120 messages (15,000 tokens)
After truncation:
  - Recent messages: 20 (2,500 tokens)
  - Summary: 1 (200 tokens)
  - Total: 2,700 tokens
  - Savings: 12,300 tokens (82% reduction)
```

## Integration with Conversation Model

The truncation system integrates seamlessly with the existing `model.Conversation`:

```go
// Existing conversation model (no changes needed)
conv := model.NewConversation()
conv.AddUserMessage("Hello")
conv.AddAssistantMessage()

// Add truncation on top
truncator := context.NewConversationTruncator(config)
result, _ := truncator.Truncate(ctx, conv)

// Original conversation unchanged
fmt.Println("Original:", len(conv.Messages))
fmt.Println("Truncated:", len(result.RecentMessages))
```

## UI Integration

### Display Truncation Indicator

```go
if result.WasTruncated {
    fmt.Printf("🗜️ Conversation summarized (%s)\n", result.SummaryInfo())
}
```

### Show Summary on Demand

```go
if result.HasSummary() {
    fmt.Println("Summary of previous conversation:")
    fmt.Println(result.Summary)
}
```

### Progress During Summarization

```go
var progress int
summarizer.SummarizeStream(ctx, messages, func(chunk string) {
    progress += len(chunk)
    fmt.Printf("\rSummarizing: %d chars", progress)
})
```

## Testing

### Run Tests

```bash
# All context tests
go test ./internal/context/...

# Just truncation tests
go test ./internal/context/truncation_test.go

# Just summarizer tests
go test ./internal/context/summarizer_test.go

# With coverage
go test ./internal/context/... -cover

# Verbose output
go test ./internal/context/... -v
```

### Test Coverage

- ✅ Below threshold (no truncation)
- ✅ Above threshold (with truncation)
- ✅ System prompt preservation
- ✅ No summarizer fallback
- ✅ Token estimation
- ✅ Ollama message conversion
- ✅ Simple summarization
- ✅ LLM prompt building
- ✅ Streaming summarization

## Error Handling

### Summarization Failures

```go
result, err := truncator.Truncate(ctx, conv)
if err != nil {
    // Truncation failed, use original conversation
    messages := conv.ToOllamaMessages()
} else {
    // Use truncated result
    messages := result.ToOllamaMessages()
}
```

### LLM Unavailable

The system automatically falls back to simple summarization:

```go
// If LLM summarization fails, gets simple message count summary
// Example: "Previous conversation (80 messages)"
```

## Advanced Usage

### Custom Summarizer

```go
type CustomSummarizer struct{}

func (s *CustomSummarizer) Summarize(ctx context.Context, messages []*model.Message) (string, error) {
    // Custom summarization logic
    return "Custom summary", nil
}

truncator := context.NewConversationTruncator(&context.TruncatorConfig{
    Summarizer: &CustomSummarizer{},
})
```

### Smart Strategy

```go
strategy := context.NewSmartTruncationStrategy(summarizer)

// Automatically adjusts based on conversation characteristics
truncator := strategy.GetTruncator(conversation)
```

### Metrics Tracking

```go
metrics := &context.TruncationMetrics{}

result, _ := truncator.Truncate(ctx, conv)
metrics.TrackTruncation(result)

fmt.Println(metrics.Report())
// Output:
// Truncation Metrics:
//   Total truncations: 5
//   Total tokens saved: 45000
//   Average tokens saved: 9000
//   Largest summary: 250 tokens
```

## Future Enhancements

Potential improvements for future versions:

1. **Semantic Chunking**: Group related messages before summarizing
2. **Importance Scoring**: Keep important older messages in full
3. **Multi-level Summaries**: Hierarchical summaries for very long conversations
4. **Cached Summaries**: Store summaries to avoid re-summarization
5. **User-triggered Summarization**: Manual "Summarize Now" button
6. **Summary Expansion**: Click to see original messages

## Troubleshooting

### Issue: Summaries are too long

**Solution**: Adjust the summarizer model temperature and max tokens:

```go
resp, err := client.ChatWithOptions(ctx, model, messages, &ollama.Options{
    Temperature: 0.3,  // Lower for more concise
    NumPredict:  300,  // Reduce max length
})
```

### Issue: Important context lost

**Solution**: Increase `MaxFullMessages`:

```go
config := &TruncatorConfig{
    MaxFullMessages: 30,  // Keep more messages
}
```

### Issue: Truncation happening too early

**Solution**: Increase `SummaryThreshold`:

```go
config := &TruncatorConfig{
    SummaryThreshold: 75,  // Wait longer before truncating
}
```

## API Reference

See inline documentation in:
- `truncation.go` - Core truncation logic
- `summarizer.go` - Summarization implementations
- `example_integration.go` - Usage examples

## License

Part of the rigrun Go TUI project.
