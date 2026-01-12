# rigrun Benchmark Results Template

This document serves as a template for benchmark results. After running the benchmark suite, actual results will be generated in the `results/` directory.

## Quick Summary

| Metric | Target | Your Result | Status |
|--------|--------|-------------|--------|
| Semantic Cache Hit Rate | 60%+ | ___ % | [ ] |
| Exact Cache Hit Rate | 95%+ | ___ % | [ ] |
| Cache Latency | < 5ms | ___ ms | [ ] |
| Cost Savings vs GPT-4 | 90%+ | ___ % | [ ] |

## Detailed Results

### Cache Performance

```
+---------------------------+----------+-----------+
| Metric                    | Exact    | Semantic  |
+---------------------------+----------+-----------+
| Total Queries             |          |           |
| Cache Hits                |          |           |
| Cache Misses              |          |           |
| Hit Rate                  |     %    |      %    |
| Avg Hit Latency (ms)      |          |           |
| Avg Miss Latency (ms)     |          |           |
+---------------------------+----------+-----------+
```

### Latency Distribution

```
Latency Histogram (Cache Hits)
==============================

   0-1ms   |############################################|
   1-2ms   |####################|
   2-5ms   |#######|
   5-10ms  |##|
  10-50ms  ||
  50-100ms ||
   >100ms  ||

Latency Histogram (Cache Misses - Local LLM)
============================================

   0-500ms   |#########|
 500-1000ms  |############################|
1000-1500ms  |#####################|
1500-2000ms  |############|
2000-3000ms  |#####|
   >3000ms   |##|
```

### Semantic Cache Group Analysis

Shows hit rate per semantic similarity group:

```
+-----------------------------+----------+------+-----------+
| Group                       | Queries  | Hits | Hit Rate  |
+-----------------------------+----------+------+-----------+
| recursion_explanation       |          |      |       %   |
| debugging_help              |          |      |       %   |
| function_creation           |          |      |       %   |
| code_explanation            |          |      |       %   |
| async_programming           |          |      |       %   |
| api_requests                |          |      |       %   |
| error_handling              |          |      |       %   |
| database_queries            |          |      |       %   |
| testing_code                |          |      |       %   |
| git_operations              |          |      |       %   |
| performance_optimization    |          |      |       %   |
| data_structures             |          |      |       %   |
| authentication              |          |      |       %   |
| refactoring                 |          |      |       %   |
| regex_patterns              |          |      |       %   |
+-----------------------------+----------+------+-----------+
| TOTAL                       |          |      |       %   |
+-----------------------------+----------+------+-----------+
```

### Throughput Analysis

```
Queries Per Second (QPS) by Phase
=================================

Warmup:        [##########] ___ qps
Baseline:      [##########] ___ qps
Exact Cache:   [##########] ___ qps
Semantic:      [##########] ___ qps
Mixed:         [##########] ___ qps
Stress Test:   [##########] ___ qps
```

## Cost Savings Analysis

### Assumptions

| Parameter | Value | Notes |
|-----------|-------|-------|
| GPT-4 Input Price | $0.03/1K tokens | OpenAI pricing as of 2024 |
| GPT-4 Output Price | $0.06/1K tokens | OpenAI pricing as of 2024 |
| Avg Tokens/Query | 500 | Input + Output combined |
| Local LLM Cost | $0.00/query | After hardware investment |
| Cache Hit Cost | $0.00/query | Zero marginal cost |

### Monthly Cost Projection

```
Usage Tier: Light (10,000 queries/month)
========================================

Without rigrun (100% GPT-4):
  Input tokens:  5,000,000 @ $0.03/1K = $150.00
  Output tokens: 5,000,000 @ $0.06/1K = $300.00
  TOTAL: $450.00/month

With rigrun (___% cache hit rate):
  Cache hits: ___ queries = $0.00
  Local LLM:  ___ queries = $0.00
  Cloud fallback: ___ queries = $___.00
  TOTAL: $___.00/month

  SAVINGS: $___.00/month (___%)
  ====================================


Usage Tier: Medium (100,000 queries/month)
==========================================

Without rigrun (100% GPT-4):
  TOTAL: $4,500.00/month

With rigrun:
  TOTAL: $___.00/month

  SAVINGS: $___/month (___%)
  ====================================


Usage Tier: Heavy (1,000,000 queries/month)
===========================================

Without rigrun (100% GPT-4):
  TOTAL: $45,000.00/month

With rigrun:
  TOTAL: $___/month

  SAVINGS: $___/month (___%)
  ====================================
```

### ROI Calculator

```
Hardware Investment: $___
Monthly Savings: $___
Payback Period: ___ months

Break-even Analysis
===================
Month 1:  Cumulative savings: $___
Month 3:  Cumulative savings: $___
Month 6:  Cumulative savings: $___
Month 12: Cumulative savings: $___
```

## Memory Efficiency

### Cache Memory Usage

```
+---------------------------+----------------+
| Metric                    | Value          |
+---------------------------+----------------+
| Cache Entries             |                |
| Memory Per Entry (avg)    |           KB   |
| Total Cache Memory        |           MB   |
| Vector Index Size         |           MB   |
| Embedding Dimensions      |                |
+---------------------------+----------------+
```

### Comparison with Competitors

```
Memory Usage for 10,000 Cached Responses
========================================

rigrun:
  [#######                              ] ___ MB

LiteLLM (with Redis):
  [##################                   ] ~150 MB

Custom Solution (in-memory):
  [#########################            ] ~200 MB

Note: rigrun uses efficient IndexMap with LRU eviction
and compressed embeddings for minimal memory footprint.
```

## Performance Charts

### Cache Hit Rate Over Time

```
Hit Rate %
100|
 90|                              ....
 80|                    ..........
 70|            ........
 60| -------- target line --------
 50|    ......
 40| ...
 30|
 20|
 10|
  0+---+---+---+---+---+---+---+---+---+---+
    0  10  20  30  40  50  60  70  80  90 100
                    Queries
```

### Latency Comparison

```
Latency (ms)
2000|
    | ###
1500| ###
    | ###
1000| ###
    | ###  ###
 500| ###  ###
    | ###  ###
   5| ***  ###  ***
   0+-----+-----+-----+
     Cache Local Cloud

*** = Cache hits (sub-5ms)
### = LLM inference
```

## Reproducibility

### Test Environment

```yaml
Hardware:
  CPU: [Your CPU]
  RAM: [Your RAM] GB
  GPU: [Your GPU] ([VRAM] GB)
  Storage: [SSD/HDD]

Software:
  OS: Windows [Version]
  PowerShell: [Version]
  Rust: [Version]
  Ollama: [Version]
  rigrun: [Version]

Configuration:
  Model: qwen2.5-coder:7b
  Cache TTL: 24 hours
  Max Cache Entries: 10,000
  Similarity Threshold: 0.85
```

### Commands to Reproduce

```powershell
# 1. Clone and build
git clone https://github.com/rigrun/rigrun
cd rigrun
cargo build --release

# 2. Pull required models
ollama pull qwen2.5-coder:7b
ollama pull nomic-embed-text

# 3. Run benchmarks
cd benchmarks
.\run_benchmark.ps1 -Iterations 3

# 4. View results
Get-Content results\latest_results.json | ConvertFrom-Json
```

## Conclusion

### Key Findings

1. **Semantic Cache Effectiveness**: [ ] Achieved / [ ] Not achieved 60%+ hit rate
2. **Exact Cache Performance**: [ ] Achieved / [ ] Not achieved 95%+ hit rate
3. **Latency Target**: [ ] Achieved / [ ] Not achieved <5ms cache latency
4. **Cost Savings**: Estimated ___% reduction in LLM API costs

### Recommendations

Based on benchmark results:

1. **Threshold Tuning**: If semantic hit rate is low, consider adjusting similarity threshold
2. **Cache Size**: Current max entries (10,000) is [ ] adequate / [ ] should be increased
3. **TTL Setting**: 24-hour TTL is [ ] appropriate / [ ] should be adjusted

---

*This template will be populated with actual data after running `.\run_benchmark.ps1`*
*Results are saved to `results/benchmark_results_<timestamp>.md`*
