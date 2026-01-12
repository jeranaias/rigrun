# rigrun Battle Plan: From Wrapper to Moat

> **Mission:** Transform rigrun from "a wrapper around Ollama" into "the fastest, smartest, most private LLM router with true semantic caching."

> **Enemy:** LiteLLM ($2M funded, Python, enterprise-focused)

> **Our Advantages:** Rust (speed), single-person (agility), open source (trust), local-first (privacy)

---

## Phase 1: Technical Differentiation (Weeks 1-2)

### 1.1 TRUE Semantic Caching (THE MOAT)

**Current State:** Exact string hashing - "What is recursion?" and "Explain recursion" are cache MISSES.

**Target State:** Embedding-based similarity - both queries hit the same cached response.

#### Implementation Plan

```
┌─────────────────────────────────────────────────────────────────┐
│                    SEMANTIC CACHE ARCHITECTURE                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   Query: "Explain recursion"                                     │
│              │                                                   │
│              ▼                                                   │
│   ┌──────────────────┐                                          │
│   │  Embedding Model  │  (all-MiniLM-L6-v2, local, 80MB)        │
│   │   via Ollama      │                                          │
│   └────────┬─────────┘                                          │
│            │                                                     │
│            ▼                                                     │
│   [0.23, 0.87, 0.12, ...]  (384-dim vector)                     │
│            │                                                     │
│            ▼                                                     │
│   ┌──────────────────┐                                          │
│   │  Vector Index     │  (HNSW algorithm, in-memory)            │
│   │  (hnswlib-rs)     │                                          │
│   └────────┬─────────┘                                          │
│            │                                                     │
│            ▼                                                     │
│   Similarity Search: Find vectors with cosine_sim > 0.85        │
│            │                                                     │
│            ├─── MATCH FOUND ──► Return cached response           │
│            │                                                     │
│            └─── NO MATCH ─────► Route to Local/Cloud             │
│                                       │                          │
│                                       ▼                          │
│                              Store response + embedding          │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

#### Technical Specifications

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Embedding Model | `nomic-embed-text` or `all-minilm` via Ollama | Already have Ollama, no new deps |
| Vector Dimensions | 384 or 768 | Standard sizes, good accuracy |
| Similarity Algorithm | HNSW (Hierarchical Navigable Small World) | O(log n) search, battle-tested |
| Rust Library | `hnsw_rs` or `instant-distance` | Pure Rust, no FFI |
| Similarity Threshold | 0.85 (configurable) | Balance precision/recall |
| Index Persistence | Memory-mapped file | Fast startup, survives restarts |

#### Files to Create/Modify

```
src/
├── cache/
│   ├── mod.rs              # Existing - add semantic mode toggle
│   ├── semantic.rs         # NEW - SemanticCache implementation
│   ├── embeddings.rs       # NEW - Embedding generation via Ollama
│   └── vector_index.rs     # NEW - HNSW index wrapper
├── server/
│   └── mod.rs              # Modify - route through semantic cache
└── main.rs                 # Modify - add --semantic-cache flag
```

#### API Changes

```rust
// New config options
pub struct Config {
    // ... existing fields ...
    pub semantic_cache_enabled: bool,      // Default: true
    pub similarity_threshold: f32,          // Default: 0.85
    pub embedding_model: String,            // Default: "nomic-embed-text"
    pub max_cache_entries: usize,           // Default: 50,000
}

// New cache stats
pub struct CacheStats {
    // ... existing fields ...
    pub semantic_hits: u64,
    pub exact_hits: u64,
    pub avg_similarity_score: f32,
}
```

#### Benchmarks to Publish

1. **Hit Rate Comparison**
   - Exact cache vs Semantic cache on 10,000 real queries
   - Target: 60%+ semantic hit rate vs <15% exact hit rate

2. **Latency Impact**
   - Embedding generation time (target: <50ms)
   - Vector search time (target: <5ms for 100k entries)
   - Total overhead: <100ms

3. **Memory Usage**
   - 100k entries = ~150MB vector index
   - Acceptable for most machines

---

### 1.2 Performance Optimization

#### Binary Size Reduction

| Current | Target | Method |
|---------|--------|--------|
| ~25MB | <15MB | Strip symbols, LTO, `opt-level=z` |
| N/A | <50MB Docker | Alpine base, multi-stage build |

#### Cargo.toml Optimizations

```toml
[profile.release]
opt-level = "z"      # Optimize for size
lto = true           # Link-time optimization
codegen-units = 1    # Better optimization
panic = "abort"      # Smaller binary
strip = true         # Strip symbols
```

#### Memory Footprint

| Current | Target | Method |
|---------|--------|--------|
| ~50MB idle | <20MB idle | Lazy loading, smaller buffers |
| Unbounded cache | Configurable LRU | Already implemented |

---

### 1.3 Code Assistant Specialization

#### Features for Developers

1. **Project-Aware Caching**
   ```rust
   // Cache key includes project context
   cache_key = hash(query + project_root + file_extension)
   ```

2. **Language-Specific Routing**
   ```rust
   // Route Rust questions to models good at Rust
   if detected_language == "rust" {
       prefer_model = "deepseek-coder";
   }
   ```

3. **Completion vs Chat Mode**
   ```rust
   // Different caching strategies
   if request.is_completion {
       // More aggressive caching, prefix matching
   } else {
       // Standard semantic caching
   }
   ```

4. **IDE Telemetry (opt-in)**
   ```rust
   // Track which completions were accepted
   // Use to improve routing decisions
   ```

---

## Phase 2: Distribution & Simplicity (Week 2-3)

### 2.1 One-Line Install

#### Current State
```bash
# Requires Rust installed
cargo install rigrun
```

#### Target State
```bash
# Just works
curl -fsSL https://rigrun.dev/install.sh | sh

# Or Windows
irm https://rigrun.dev/install.ps1 | iex
```

#### Release Artifacts

| Platform | Format | Size Target |
|----------|--------|-------------|
| Linux x64 | Binary + .deb + .rpm | <15MB |
| Linux ARM64 | Binary | <15MB |
| macOS Universal | Binary + .pkg | <15MB |
| Windows x64 | Binary + .msi | <15MB |
| Docker | Multi-arch image | <50MB |

### 2.2 Zero-Config Operation

#### Current State
- Requires Ollama pre-installed
- Manual model download
- Config file needed for cloud

#### Target State
```bash
rigrun
# Auto-detects Ollama or offers to install
# Auto-downloads recommended model for your VRAM
# Prompts for OpenRouter key only if you want cloud
# Server running in 60 seconds
```

### 2.3 Docker Support

```dockerfile
# Dockerfile
FROM alpine:3.19
COPY rigrun /usr/local/bin/
EXPOSE 8787
ENTRYPOINT ["rigrun"]
```

```yaml
# docker-compose.yml
services:
  rigrun:
    image: ghcr.io/jeranaias/rigrun:latest
    ports:
      - "8787:8787"
    volumes:
      - rigrun-cache:/root/.rigrun
    environment:
      - OPENROUTER_KEY=${OPENROUTER_KEY}
```

---

## Phase 3: Privacy Maximalism (Week 3-4)

### 3.1 Privacy Features

1. **Audit Log**
   ```
   ~/.rigrun/audit.log
   2024-01-15 10:23:45 | CACHE_HIT  | "What is..." | local
   2024-01-15 10:24:12 | CLOUD_SENT | "Complex..." | openrouter | $0.002
   ```

2. **Paranoid Mode**
   ```bash
   rigrun --paranoid  # Blocks ALL cloud requests
   ```

3. **Data Export**
   ```bash
   rigrun export --format json  # Export all your data
   rigrun purge                  # Delete everything
   ```

4. **Telemetry Declaration**
   ```
   rigrun collects: NOTHING
   rigrun phones home: NEVER
   rigrun sells data: ARE YOU KIDDING
   ```

### 3.2 Compliance Positioning

- [ ] Write GDPR compliance statement
- [ ] Write HIPAA considerations doc
- [ ] Create "rigrun for Enterprise" landing page
- [ ] Add SOC2-friendly audit logging

---

## Phase 4: Community & Content (Ongoing)

### 4.1 Build in Public

#### Daily Content
- Day 1: "Building true semantic caching for rigrun"
- Day 2: "Hit 60% cache rate with embeddings"
- Day 3: "Shrunk the binary from 25MB to 12MB"
- ...continue daily

#### Platforms
- Twitter/X: Daily updates, benchmarks, comparisons
- Reddit: Weekly deep-dives on r/LocalLLaMA, r/rust
- YouTube: "Building an LLM Router in Rust" series
- Dev.to/Hashnode: Technical blog posts

### 4.2 Comparison Content (SEO Play)

Create pages/posts for:
- "rigrun vs LiteLLM: Benchmarks"
- "rigrun vs GPTCache: Which is faster?"
- "Best local LLM router 2024"
- "How to reduce OpenAI costs by 80%"

### 4.3 Integration Partnerships

Reach out to:
- **Continue.dev** - "We'd love to be a recommended backend"
- **Ollama** - "Can we be listed as a companion tool?"
- **LocalAI** - "Integration partnership?"

---

## Phase 5: Monetization (Month 2+)

### 5.1 Revenue Streams (In Order of Priority)

1. **GitHub Sponsors / Ko-fi**
   - Target: $500/month
   - Requires: 1000+ stars, active community

2. **Consulting**
   - "Need custom LLM infrastructure? I built rigrun."
   - Target: $5-20k/year side income

3. **rigrun Cloud (Future)**
   - Hosted version for teams
   - $29/month per seat
   - Requires: Proven OSS traction first

4. **Enterprise Support**
   - SLA, priority support, custom features
   - $500-2000/month per company

### 5.2 Pricing Philosophy

> "rigrun is free forever for individuals. Companies that make money using rigrun should pay."

---

## Success Metrics

### Week 1
- [ ] Semantic caching implemented
- [ ] Benchmarks showing 60%+ hit rate
- [ ] Binary size <20MB

### Week 2
- [ ] Docker support live
- [ ] One-line install working
- [ ] First "rigrun vs LiteLLM" benchmark published

### Month 1
- [ ] 500+ GitHub stars
- [ ] 100+ daily active users
- [ ] Featured on r/LocalLLaMA

### Month 3
- [ ] 2000+ GitHub stars
- [ ] Continue.dev integration
- [ ] First paying sponsor

### Month 6
- [ ] 5000+ GitHub stars
- [ ] $500+/month in sponsorships
- [ ] Enterprise inquiry pipeline

---

## The Narrative

### One-Liner
> "rigrun: The LLM router with a memory."

### Elevator Pitch
> "LiteLLM routes your LLM requests. rigrun remembers them. Our semantic cache learns from every query, delivering 60%+ cache hits and cutting your API costs by 80% - all in a single 15MB binary that never phones home."

### Battle Cry
> "They have funding. We have focus. They route. We remember. They're Python. We're Rust. Let's fucking go."

---

## Immediate Next Steps

1. **Launch rigrun server** for dogfooding
2. **Implement semantic cache** using rigrun + Sonnets
3. **Benchmark and publish results**
4. **Ship v0.2.0 with semantic caching**
5. **Post on HN/Reddit with benchmarks**

---

*Last updated: Today*
*Status: READY TO EXECUTE*
