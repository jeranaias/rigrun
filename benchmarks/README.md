# rigrun Benchmark Suite

Comprehensive benchmarks to measure and prove rigrun's performance claims for semantic caching, latency improvements, and cost savings.

## Overview

This benchmark suite tests rigrun's three-tier caching architecture:
1. **Semantic Cache Layer** - Embedding-based similarity matching
2. **Exact Cache Layer** - Hash-based exact match caching
3. **Pass-through** - Requests that go to local/cloud LLM

## Performance Claims Being Tested

| Metric | Target | How We Measure |
|--------|--------|----------------|
| Semantic Cache Hit Rate | 60%+ | Similar queries hitting cache |
| Exact Cache Hit Rate | 30%+ | Identical queries hitting cache |
| Cache Latency | <5ms | Time to serve cached response |
| Local LLM Latency | <2000ms | Time for Ollama response |
| Cost Savings | 90%+ | Tokens saved vs all-cloud |

## Test Dataset

The `queries.json` file contains 100+ realistic developer queries organized into:

### Semantic Similarity Groups
Queries that express the same intent differently (SHOULD hit semantic cache):
- "What is recursion?" / "Explain recursion" / "How does recursion work?"
- "Fix this bug" / "Debug this code" / "What's wrong with this?"
- "Write a function" / "Create a method" / "Implement a function"

### Distinct Query Categories
Queries that should NOT match each other:
- Code explanation requests
- Debugging requests
- Code generation requests
- Refactoring requests
- Documentation requests

## Hardware Requirements

### Minimum (for valid benchmarks)
- **CPU**: 4+ cores (for concurrent request handling)
- **RAM**: 8GB (for embeddings and cache)
- **Storage**: 1GB free (for test data and results)

### Recommended (for production-realistic results)
- **CPU**: 8+ cores
- **RAM**: 16GB+
- **GPU**: NVIDIA with 8GB+ VRAM (for local LLM via Ollama)
- **Storage**: SSD with 10GB+ free

### Required Software
- Windows 10/11 or Windows Server 2019+
- PowerShell 5.1+ (included in Windows)
- Rust toolchain (for building rigrun)
- Ollama (for local LLM inference)

## Running Benchmarks

### Quick Start

```powershell
# Navigate to benchmarks directory
cd C:\rigrun\benchmarks

# Run the full benchmark suite
.\run_benchmark.ps1
```

### Options

```powershell
# Run with specific number of iterations
.\run_benchmark.ps1 -Iterations 3

# Run without starting rigrun (if already running)
.\run_benchmark.ps1 -SkipServerStart

# Run only semantic cache tests
.\run_benchmark.ps1 -TestType semantic

# Run only exact cache tests
.\run_benchmark.ps1 -TestType exact

# Verbose output for debugging
.\run_benchmark.ps1 -Verbose

# Specify custom server URL
.\run_benchmark.ps1 -ServerUrl "http://localhost:8080"
```

### Output Files

After running, results are saved to:
- `results/benchmark_results_<timestamp>.json` - Raw JSON data
- `results/benchmark_results_<timestamp>.md` - Formatted markdown report
- `results/latest_results.json` - Symlink to most recent run

## Benchmark Methodology

### Phase 1: Warm-up
1. Start rigrun server (if not already running)
2. Wait for Ollama connection
3. Run 10 warm-up queries to initialize caches

### Phase 2: Baseline (No Cache)
1. Clear all caches
2. Send each unique query once
3. Measure latency and token usage
4. Record as baseline metrics

### Phase 3: Exact Cache Test
1. Send identical queries again
2. Measure cache hit rate and latency
3. Calculate improvement vs baseline

### Phase 4: Semantic Cache Test
1. Send semantically similar queries (paraphrased)
2. Measure semantic cache hit rate
3. Compare latency to baseline
4. Validate similarity threshold effectiveness

### Phase 5: Mixed Workload
1. Simulate realistic usage pattern:
   - 40% repeated exact queries
   - 30% semantically similar queries
   - 30% new unique queries
2. Measure overall cache effectiveness
3. Calculate cost savings

### Phase 6: Stress Test (Optional)
1. Send concurrent requests (10, 50, 100 parallel)
2. Measure throughput and latency under load
3. Check for cache consistency

## Interpreting Results

### Cache Hit Rate
- **60%+ semantic**: Excellent - semantic matching working well
- **40-60% semantic**: Good - typical for varied workloads
- **<40% semantic**: Poor - may need threshold tuning

### Latency
- **Cache hit**: Should be <5ms (sub-millisecond typical)
- **Local LLM**: 500-2000ms depending on model/query
- **Cloud LLM**: 1000-5000ms depending on provider

### Cost Savings
Calculated as:
```
savings = (queries_from_cache * avg_tokens_per_query * cost_per_token) / total_cost_without_cache
```

Using OpenAI GPT-4 pricing ($0.03/1K input, $0.06/1K output) as baseline.

## Reproducing Results

To reproduce benchmark results:

1. **Environment Setup**
   ```powershell
   # Install Rust
   winget install Rustlang.Rustup

   # Install Ollama
   winget install Ollama.Ollama

   # Build rigrun
   cd C:\rigrun
   cargo build --release
   ```

2. **Pull Required Models**
   ```powershell
   ollama pull qwen2.5-coder:7b
   ollama pull nomic-embed-text  # For embeddings
   ```

3. **Run Benchmarks**
   ```powershell
   cd C:\rigrun\benchmarks
   .\run_benchmark.ps1 -Iterations 5
   ```

4. **Compare Results**
   Results are saved with timestamps for comparison across runs.

## Extending the Benchmark

### Adding Custom Queries
Edit `queries.json` to add new test cases:
```json
{
  "semantic_groups": [
    {
      "name": "your_group_name",
      "description": "What these queries test",
      "queries": [
        "First phrasing of the question",
        "Second phrasing (should match first)",
        "Third phrasing (should match first)"
      ]
    }
  ]
}
```

### Adding Custom Metrics
The benchmark script supports custom metric collection via hooks:
```powershell
# In run_benchmark.ps1, add to the metrics collection section
$customMetric = Measure-YourCustomMetric
$results.custom_metrics.your_metric = $customMetric
```

## Troubleshooting

### Server Won't Start
```powershell
# Check if port is in use
netstat -ano | findstr :8787

# Kill existing process
taskkill /PID <pid> /F
```

### Low Cache Hit Rate
1. Check similarity threshold in rigrun config
2. Verify embeddings are being generated (check logs)
3. Ensure queries are being stored properly

### High Latency
1. Check Ollama is running: `ollama list`
2. Verify GPU is being utilized: `nvidia-smi`
3. Check for memory pressure

## Contributing

Found an issue or want to add more test cases? Please:
1. Fork the repository
2. Add your changes
3. Run the full benchmark suite
4. Submit a PR with before/after results

## License

MIT - Same as rigrun main project
