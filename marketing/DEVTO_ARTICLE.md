# Building a Local-First LLM Router: A Technical Deep Dive

*How rigrun routes queries through cache, local GPU, and cloud fallback - with real usage numbers*

---

## The Wake-Up Call

It was 2 AM on a Tuesday when I checked my OpenAI dashboard. $487.23 for the month. My side project, a simple AI coding assistant, was burning through my runway faster than I could say "token limit exceeded."

I stared at my idle RTX 3080. I had spent $800 on this card for gaming, but it was collecting dust while I paid OpenAI to do work it could handle locally.

That's when I decided to build **rigrun** - a local-first LLM router that intelligently decides where to send each request: cache, local GPU, or cloud.

**Spoiler**: It worked for my use case. Your results will depend on your query patterns and hardware.

---

## The Problem: False Dichotomy

Most developers face a bad choice:

### Option 1: Cloud-Only (OpenAI, Anthropic, etc.)
**Pros:**
- Easy to use
- Excellent quality
- No setup required

**Cons:**
- Expensive ($500+/month for moderate use)
- Privacy concerns
- Latency from API calls
- Vendor lock-in

### Option 2: Local-Only (Ollama, llama.cpp, etc.)
**Pros:**
- Free (after hardware cost)
- Complete privacy
- No rate limits
- Fast (on good hardware)

**Cons:**
- Quality ceiling (7-14B models ≠ GPT-4)
- Requires GPU investment
- Model management overhead
- No fallback for hard queries

### What if we could have both?

---

## The Solution: Smart Three-Tier Routing

rigrun implements a simple but powerful routing strategy:

```
┌─────────────────┐
│     Request     │
└────────┬────────┘
         │
    ┌────▼─────┐
    │  Cache?  │
    └──┬───┬───┘
       │   │ Miss
   Hit │   │
       │   │
       │   └─────────┐
       │             │
       │        ┌────▼─────┐
       │        │  Local?  │
       │        └──┬───┬───┘
       │           │   │ Fail
       │       OK  │   │
       │           │   │
       │           │   └─────────┐
       │           │             │
       │           │        ┌────▼─────┐
       │           │        │  Cloud?  │
       │           │        └────┬─────┘
       │           │             │
       └───────────┴─────────────┴─────→ Response
```

### Tier 1: Semantic Cache
Before doing any inference, check if we've seen a similar query recently.

**Key insight**: "What is recursion?" and "Explain recursion to me" should hit the same cache.

**Implementation**:
```rust
async fn check_cache(query: &str) -> Option<CachedResponse> {
    let query_embedding = embed(query).await?;

    for (cached_query, response) in cache.iter() {
        let similarity = cosine_similarity(
            &query_embedding,
            &cached_query.embedding
        );

        if similarity > 0.85 && !cached_query.is_expired() {
            return Some(response.clone());
        }
    }

    None
}
```

**Results**:
- Hit rate: 32% (after warmup)
- Latency: 12ms (P50)
- Cost: $0

### Tier 2: Local GPU
If cache misses, try the local GPU via Ollama.

**Model Selection Strategy**:
```rust
fn recommend_model(vram: usize) -> &'static str {
    match vram {
        0..=6 => "qwen2.5-coder:3b",
        6..=8 => "qwen2.5-coder:7b",
        8..=16 => "qwen2.5-coder:14b",
        16..=24 => "deepseek-coder-v2:16b",
        _ => "llama3.3:70b",
    }
}
```

**Implementation**:
```rust
async fn try_local(query: &str, model: &str) -> Result<Response> {
    let request = OllamaRequest {
        model: model.to_string(),
        prompt: query.to_string(),
        stream: false,
    };

    // Timeout after 30 seconds to avoid hanging
    timeout(
        Duration::from_secs(30),
        ollama_client.generate(request)
    ).await?
}
```

**Results**:
- Success rate: 58% of total queries
- Latency: 180ms (P50, 7B model)
- Cost: $0
- Quality: "Good enough" for 80% of tasks

### Tier 3: Cloud Fallback
If local fails or times out, fall back to cloud.

**Why OpenRouter instead of direct OpenAI**:
- Better pricing
- Multiple providers (fallback of fallback)
- Transparent pricing

**Implementation**:
```rust
async fn fallback_to_cloud(query: &str) -> Result<Response> {
    let request = OpenRouterRequest {
        model: "anthropic/claude-3-haiku".to_string(), // Cheapest good model
        messages: vec![Message {
            role: "user".to_string(),
            content: query.to_string(),
        }],
    };

    openrouter_client.complete(request).await
}
```

**Results**:
- Frequency: 10% of queries
- Latency: 890ms (P50)
- Cost: $0.15 per 1M tokens (Claude 3 Haiku)

---

## The Results: My Numbers (Your Mileage Will Vary)

I tracked 30 days of usage on my coding assistant project. These numbers are specific to my use case (mostly code generation and simple Q&A).

### Query Distribution
```
Total queries: 4,782

Tier Breakdown:
├─ Cache:  1,534 queries (32.1%) → $0.00
├─ Local:  2,789 queries (58.3%) → $0.00
└─ Cloud:    459 queries ( 9.6%) → $47.32
```

### My Cost Comparison
| Approach | Monthly Cost | Notes |
|----------|--------------|-------|
| **100% Cloud (estimated)** | ~$400-500 | Depends on model/provider |
| **rigrun (my setup)** | $47.32 | 90% handled locally |

**Important caveats:**
- These numbers assume most of your queries are simple enough for local models
- Complex reasoning tasks will hit cloud more often
- Cache hit rate depends on how repetitive your queries are
- Your hardware affects local inference quality and speed

### Latency Distribution
```
                Cache    Local    Cloud
P50:            12ms     180ms    890ms
P90:            38ms     350ms    1,450ms
P99:            45ms     420ms    2,100ms
```

**Takeaway**: Caching provides the biggest latency win when queries are similar enough to hit.

---

## Building rigrun: Technical Deep Dive

### Architecture Overview

```
┌─────────────────────────────────────────────────┐
│                  rigrun Server                  │
│                  (Port 8787)                    │
├─────────────────────────────────────────────────┤
│                                                 │
│  ┌───────────────┐  ┌──────────────────────┐  │
│  │ Semantic      │  │  Request Router      │  │
│  │ Cache Engine  │  │  (async/await)       │  │
│  │               │  │                      │  │
│  │ Vector Store  │  │  • Priority logic    │  │
│  │ TTL Manager   │  │  • Error handling    │  │
│  │ Persistence   │  │  • Cost tracking     │  │
│  └───────────────┘  └──────────────────────┘  │
│                                                 │
│  ┌───────────────┐  ┌──────────────────────┐  │
│  │ Ollama Client │  │  OpenRouter Client   │  │
│  │ (Local GPU)   │  │  (Cloud Fallback)    │  │
│  └───────────────┘  └──────────────────────┘  │
│                                                 │
└─────────────────────────────────────────────────┘
         ▲                                ▲
         │                                │
    Ollama API                     OpenRouter API
  (localhost:11434)              (openrouter.ai)
```

### Technology Stack

**Language**: Rust
- Why? Performance, memory safety, excellent async support

**Framework**: Axum
- Modern, fast HTTP framework built on Tokio

**Async Runtime**: Tokio
- Industry-standard async runtime

**Serialization**: Serde
- Type-safe JSON handling

**Dependencies**:
```toml
[dependencies]
axum = "0.7"
tokio = { version = "1", features = ["full"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
reqwest = { version = "0.11", features = ["json"] }
thiserror = "1"
```

### Core Router Implementation

```rust
use axum::{Router, routing::post, Json, Extension};
use serde::{Deserialize, Serialize};
use std::sync::Arc;

#[derive(Deserialize)]
struct ChatRequest {
    model: String,
    messages: Vec<Message>,
    temperature: Option<f32>,
}

#[derive(Serialize)]
struct ChatResponse {
    id: String,
    object: String,
    choices: Vec<Choice>,
    usage: Usage,
}

async fn chat_completions(
    Extension(state): Extension<Arc<AppState>>,
    Json(req): Json<ChatRequest>,
) -> Result<Json<ChatResponse>, AppError> {
    let query = extract_query(&req.messages);

    // Tier 1: Check cache
    if let Some(cached) = state.cache.get(&query).await? {
        state.metrics.record_cache_hit();
        return Ok(Json(cached));
    }

    // Tier 2: Try local
    match state.ollama.complete(&req).await {
        Ok(response) => {
            state.cache.set(&query, &response).await?;
            state.metrics.record_local_hit();
            return Ok(Json(response));
        }
        Err(e) => {
            log::warn!("Local inference failed: {}", e);
        }
    }

    // Tier 3: Fallback to cloud
    let response = state.openrouter.complete(&req).await?;
    state.cache.set(&query, &response).await?;
    state.metrics.record_cloud_hit(calculate_cost(&response));

    Ok(Json(response))
}
```

### Semantic Caching Implementation

The most interesting part: semantic similarity matching.

```rust
use ndarray::Array1;

struct CacheEntry {
    query: String,
    embedding: Array1<f32>,
    response: ChatResponse,
    expires_at: SystemTime,
}

struct SemanticCache {
    entries: Vec<CacheEntry>,
    embedding_client: EmbeddingClient,
    ttl: Duration,
}

impl SemanticCache {
    async fn get(&self, query: &str) -> Option<ChatResponse> {
        let query_embedding = self.embedding_client
            .embed(query)
            .await
            .ok()?;

        for entry in &self.entries {
            // Skip expired entries
            if SystemTime::now() > entry.expires_at {
                continue;
            }

            // Calculate cosine similarity
            let similarity = cosine_similarity(
                &query_embedding,
                &entry.embedding
            );

            // Threshold: 0.85 (tuned empirically)
            if similarity > 0.85 {
                return Some(entry.response.clone());
            }
        }

        None
    }

    async fn set(&mut self, query: &str, response: &ChatResponse) -> Result<()> {
        let embedding = self.embedding_client.embed(query).await?;

        self.entries.push(CacheEntry {
            query: query.to_string(),
            embedding,
            response: response.clone(),
            expires_at: SystemTime::now() + self.ttl,
        });

        // Cleanup expired entries periodically
        self.cleanup_expired();

        Ok(())
    }
}

fn cosine_similarity(a: &Array1<f32>, b: &Array1<f32>) -> f32 {
    let dot_product = a.dot(b);
    let norm_a = a.dot(a).sqrt();
    let norm_b = b.dot(b).sqrt();
    dot_product / (norm_a * norm_b)
}
```

**Tuning the similarity threshold**:
- Too high (>0.95): Miss similar queries
- Too low (<0.75): Match unrelated queries
- Sweet spot: **0.85** (validated with 1000+ query pairs)

### GPU Auto-Detection

Cross-platform GPU detection was tricky. Here's how I solved it:

```rust
#[cfg(target_os = "windows")]
fn detect_gpu() -> Result<GpuInfo> {
    use std::process::Command;

    // Try nvidia-smi first
    if let Ok(output) = Command::new("nvidia-smi")
        .arg("--query-gpu=name,memory.total")
        .arg("--format=csv,noheader")
        .output()
    {
        return parse_nvidia_output(&output);
    }

    // Try AMD (Windows)
    // ... AMD detection logic

    // Default to CPU
    Ok(GpuInfo::Cpu)
}

#[cfg(target_os = "macos")]
fn detect_gpu() -> Result<GpuInfo> {
    use std::process::Command;

    let output = Command::new("system_profiler")
        .arg("SPDisplaysDataType")
        .output()?;

    if String::from_utf8_lossy(&output.stdout).contains("Apple") {
        return Ok(GpuInfo::AppleSilicon);
    }

    Ok(GpuInfo::Cpu)
}
```

### Error Handling Strategy

Rust's `Result` type makes error handling explicit:

```rust
use thiserror::Error;

#[derive(Error, Debug)]
enum RouterError {
    #[error("Cache error: {0}")]
    Cache(#[from] CacheError),

    #[error("Ollama unavailable: {0}")]
    Ollama(String),

    #[error("Cloud provider error: {0}")]
    Cloud(#[from] reqwest::Error),

    #[error("Invalid request: {0}")]
    InvalidRequest(String),
}

// Convert to HTTP responses
impl IntoResponse for RouterError {
    fn into_response(self) -> Response {
        let (status, message) = match self {
            RouterError::Cache(e) => (
                StatusCode::INTERNAL_SERVER_ERROR,
                format!("Cache error: {}", e)
            ),
            RouterError::Ollama(e) => (
                StatusCode::BAD_GATEWAY,
                format!("Local inference unavailable: {}", e)
            ),
            RouterError::Cloud(e) => (
                StatusCode::BAD_GATEWAY,
                format!("Cloud fallback failed: {}", e)
            ),
            RouterError::InvalidRequest(e) => (
                StatusCode::BAD_REQUEST,
                e
            ),
        };

        (status, Json(json!({ "error": message }))).into_response()
    }
}
```

---

## Benchmarking Methodology

### Test Setup
- **Hardware**: Ryzen 5600X, RTX 3080 (10GB), 32GB RAM
- **OS**: Ubuntu 22.04
- **Local Model**: qwen2.5-coder:14b (8.2GB VRAM)
- **Cloud Model**: Claude 3 Haiku (via OpenRouter)
- **Queries**: Mix of code generation, Q&A, creative writing

### Metrics Collected
```rust
struct Metrics {
    total_queries: AtomicU64,
    cache_hits: AtomicU64,
    local_hits: AtomicU64,
    cloud_hits: AtomicU64,
    total_cost: AtomicU64, // In cents
    latencies: Mutex<Vec<Duration>>,
}

impl Metrics {
    fn cache_hit_rate(&self) -> f64 {
        let total = self.total_queries.load(Ordering::Relaxed) as f64;
        let hits = self.cache_hits.load(Ordering::Relaxed) as f64;
        if total == 0.0 { 0.0 } else { hits / total }
    }

    fn avg_latency(&self) -> Duration {
        let latencies = self.latencies.lock().unwrap();
        let sum: Duration = latencies.iter().sum();
        sum / latencies.len() as u32
    }
}
```

### Quality Evaluation
I manually rated 100 responses on a scale of 1-5:

| Query Type | Local Quality | Fallback Rate |
|------------|---------------|---------------|
| Simple code | 4.8/5 | 2% |
| Complex code | 4.2/5 | 15% |
| Explanations | 4.6/5 | 5% |
| Creative | 4.0/5 | 20% |
| Math/Logic | 3.8/5 | 35% |

**Conclusion**: Local models excel at common dev tasks but struggle with complex reasoning.

---

## Lessons Learned

### What Worked

**1. Semantic caching is a game-changer**
32% cache hit rate = 1/3 of queries served instantly for free. The investment in embedding similarity was worth it.

**2. Most queries don't need GPT-4**
"Write hello world in Python" doesn't require frontier models. Local 7B models handle 80% of dev queries just fine.

**3. Rust was the right choice**
Low-latency, memory-safe, single binary distribution. Compilation times were painful, but runtime performance was worth it.

**4. Auto-detection reduces friction**
Users hate configuration. Auto-detecting GPU and recommending models made onboarding smooth.

### What Didn't Work

**1. First attempt at caching (exact match)**
Obvious in retrospect. "What is X?" and "Explain X" are semantically identical but string-different.

**2. No timeout on local inference**
Early versions would hang indefinitely on OOM errors. Learned to always set timeouts.

**3. Not tracking costs initially**
Built metrics in v2. Huge mistake - knowing ROI is critical for user motivation.

**4. Underestimating model quality gaps**
7B models are not 70B models. Setting expectations correctly is important.

### Unexpected Challenges

**1. Windows + NVIDIA + Ollama**
CUDA setup on Windows is painful. Spent 2 days debugging driver issues.

**2. Embedding API latency**
Initially used OpenAI embeddings (50ms). Switching to local embeddings (5ms) was a 10x speedup.

**3. Cache invalidation**
"There are only two hard things in Computer Science..." Cache TTL is tricky to tune.

**4. OpenRouter rate limits**
Hit rate limits during testing. Learned to respect them and handle 429s gracefully.

---

## Roadmap & Future Work

### v1.1 (Next Month)
- [ ] Docker image (most requested feature)
- [ ] Web UI for monitoring
- [ ] Prometheus metrics export
- [ ] Better error messages

### v1.2 (Q2 2026)
- [ ] Embeddings endpoint support
- [ ] Fine-tuning workflows
- [ ] Multi-user support
- [ ] Budget controls per user

### v2.0 (Q3 2026)
- [ ] Automatic quality detection (trigger fallback on low confidence)
- [ ] A/B testing framework
- [ ] Custom routing strategies
- [ ] Enterprise features (SSO, audit logs)

### Research Ideas
- Can we predict which queries need cloud before inference?
- Active learning: Fine-tune local model on cloud responses
- Multi-GPU load balancing
- Speculative execution (try local + cloud in parallel)

---

## How to Try It

### Quick Start (5 minutes)

```bash
# 1. Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# 2. Install rigrun
cargo install rigrun

# 3. Run
rigrun
```

That's it! Server starts on `http://localhost:8787`

### Integration Example (Python)

```python
from openai import OpenAI

# Before: Expensive
# client = OpenAI(api_key=os.getenv("OPENAI_API_KEY"))

# After: 90% cheaper
client = OpenAI(
    base_url="http://localhost:8787/v1",
    api_key="unused"
)

response = client.chat.completions.create(
    model="auto",
    messages=[
        {"role": "user", "content": "Write a Python function to reverse a string"}
    ]
)

print(response.choices[0].message.content)
```

### Check Your Savings

```bash
curl http://localhost:8787/stats
```

```json
{
  "total_queries": 147,
  "cache_hits": 47,
  "local_hits": 85,
  "cloud_hits": 15,
  "estimated_savings": "$23.45",
  "cache_hit_rate": 0.32
}
```

---

## Conclusion

**Key Takeaways**:

1. **Local-first can work for certain use cases** - Simple queries often don't need cloud models
2. **Semantic caching helps** - If you ask similar questions, you can avoid redundant work
3. **Smart routing is a trade-off** - You trade some quality for cost savings
4. **This is not magic** - Results depend heavily on your specific usage patterns

**Who might benefit from rigrun**:
- Developers with GPUs sitting idle
- Side projects where cost matters more than peak quality
- Privacy-sensitive applications where local processing is preferred
- Anyone curious about local LLM inference

**Who probably shouldn't use this**:
- Production apps requiring consistent GPT-4-level quality
- Users without GPUs (CPU inference is painfully slow)
- Enterprise-scale deployments
- Anyone who needs guaranteed response quality

---

## Links

- **GitHub**: https://github.com/rigrun/rigrun
- **Documentation**: https://github.com/rigrun/rigrun/tree/main/docs
- **Discord**: [Coming soon]
- **Twitter**: [@rigrun] (share your setup!)

---

## Discussion

I'd love to hear:
- What's your current LLM API spend?
- Would this work for your use case?
- What features would make this useful for you?
- What am I missing?

Drop a comment below or open an issue on GitHub!

---

*An experiment in local-first LLM routing. Feedback welcome.*

---

## Tags
`#ai` `#llm` `#opensource` `#rust` `#devops` `#costsavings` `#machinelearning` `#selfhosted` `#privacy`
