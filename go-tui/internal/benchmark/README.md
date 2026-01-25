# Model Benchmarking

The benchmark package provides comprehensive model benchmarking capabilities for rigrun, helping users understand which models perform best for their use cases.

## Features

- Standard test suite for measuring model performance
- Metrics: speed (tokens/sec), latency (TTFT), quality scores
- Single model benchmarking
- Multi-model comparison
- Results storage and history
- Beautiful TUI display

## Usage

### Basic Benchmarking

Run a benchmark on a single model:

```
/benchmark qwen2.5-coder:14b
```

### Model Comparison

Compare multiple models:

```
/benchmark qwen2.5-coder:14b llama3.1:8b mistral:7b
```

## Test Suite

The standard test suite includes:

### 1. Latency Test
- **Purpose**: Measures time to first token (TTFT)
- **Prompt**: Simple greeting
- **Metric**: Milliseconds to first token

### 2. Speed Test
- **Purpose**: Measures token generation speed
- **Prompt**: Creative task (haiku)
- **Metric**: Tokens per second

### 3. Code Completion Test
- **Purpose**: Measures accuracy on coding tasks
- **Prompt**: Complete a Fibonacci function
- **Metric**: Quality score (0-100)

### 4. Explanation Test
- **Purpose**: Measures coherence and clarity
- **Prompt**: Explain REST API
- **Metric**: Quality score (0-100)

### 5. Instruction Following Test
- **Purpose**: Measures ability to follow specific instructions
- **Prompt**: List exactly 3 programming languages
- **Metric**: Quality score (0-100)

## Metrics

### Time to First Token (TTFT)
- Lower is better
- Indicates model latency and responsiveness
- Important for interactive use cases

### Tokens Per Second
- Higher is better
- Indicates generation speed
- Important for long-form content generation

### Quality Score
- Scale: 0-100
- Measures task-specific accuracy
- Based on keyword matching and structural analysis

## Results Storage

Benchmark results are automatically saved to:
```
~/.rigrun/benchmarks/
```

Files are named with the pattern:
```
{model-name}_{timestamp}.json
```

Comparison results:
```
comparison_{timestamp}.json
```

## Interpreting Results

### For Interactive Chat
Prioritize **low TTFT** (fast response start)

### For Content Generation
Prioritize **high tokens/sec** (fast overall generation)

### For Accuracy
Prioritize **high quality scores** on relevant test types

### Best Overall
The system calculates a composite score:
- Speed: 40%
- Quality: 40%
- Latency: 20%

## Example Output

```
Benchmark Results: qwen2.5-coder:14b
=====================================

Duration:         2m 15s
Tests:            5 passed, 0 failed

Avg TTFT:         245ms
Avg Speed:        42.3 t/s
Avg Quality:      87.5%

Test Results
------------

✓ Latency Test
  TTFT: 245ms | Speed: 38.2 t/s | Quality: 100.0% | Duration: 2.5s

✓ Speed Test
  TTFT: 198ms | Speed: 45.6 t/s | Quality: 100.0% | Duration: 8.2s

✓ Code Completion Test
  TTFT: 312ms | Speed: 41.8 t/s | Quality: 85.0% | Duration: 18.5s
```

## Comparison Output

```
Model Comparison
================

Models tested: 3
Total duration: 6m 30s

Best Overall: qwen2.5-coder:14b (Speed: 42.3 t/s, Quality: 87.5%)
Fastest: mistral:7b (48.1 t/s)
Lowest Latency: llama3.1:8b (189ms)
Highest Quality: qwen2.5-coder:14b (87.5%)

Detailed Comparison
-------------------
Model                     | Avg TTFT     | Avg Speed    | Avg Quality  | Tests
qwen2.5-coder:14b        | 245ms        | 42.3 t/s     | 87.5%        | 5/5
llama3.1:8b              | 189ms        | 39.7 t/s     | 82.3%        | 5/5
mistral:7b               | 234ms        | 48.1 t/s     | 78.9%        | 5/5
```

## Architecture

### Package Structure

```
benchmark/
├── benchmark.go      # Core benchmarking logic and runner
├── tests.go          # Standard test suite definitions
├── results.go        # Result types and storage
└── README.md         # This file
```

### Key Components

1. **Runner**: Executes benchmarks on models
2. **Test**: Defines individual test prompts and evaluators
3. **Result**: Contains benchmark results and metrics
4. **Storage**: Saves/loads results from disk

### Integration

The benchmark feature integrates with:
- **Commands**: `/benchmark` command in command registry
- **Ollama Client**: Uses existing client for model interaction
- **UI Components**: `benchmark_view.go` for result display

## Extending

### Custom Tests

Create custom tests using the builders:

```go
// Custom speed test
speedTest := benchmark.NewSpeedTest(
    "my-test",
    "Write a detailed explanation...",
)

// Custom code test with keywords
codeTest := benchmark.NewCodeTest(
    "my-code-test",
    "Implement quicksort in Python",
    []string{"def", "quicksort", "partition", "return"},
)
```

### Custom Evaluators

Define custom quality evaluators:

```go
evaluator := func(response string) float64 {
    // Your custom scoring logic
    score := 0.0
    if strings.Contains(response, "expected_keyword") {
        score += 50.0
    }
    // ... more checks
    return score
}
```

## Performance Notes

- Benchmarks run with 10-minute timeout
- Each test streams responses for accurate TTFT measurement
- Results are saved asynchronously
- Failed tests don't stop the suite

## Troubleshooting

### "Ollama not available"
- Ensure Ollama is running: `ollama serve`
- Check connection: `ollama list`

### "Model not found"
- Pull the model first: `ollama pull qwen2.5-coder:14b`
- Check available models: `ollama list`

### Slow benchmarks
- Normal: Each test generates a response
- Expected time: 1-3 minutes per model
- Multiple models: Run sequentially

### Low quality scores
- Quality is task-specific
- Based on keyword matching
- Not a perfect measure of model capability
- Use for relative comparison

## Future Enhancements

Potential improvements:
- [ ] Custom test creation via UI
- [ ] Historical trend analysis
- [ ] Cost estimation (cloud models)
- [ ] Parallel test execution
- [ ] More sophisticated quality metrics
- [ ] Test suite profiles (coding, writing, chat, etc.)
- [ ] Export results to CSV/Markdown
- [ ] Benchmark leaderboard
