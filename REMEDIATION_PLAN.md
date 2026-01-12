# rigrun Remediation Plan

## Executive Summary

Six independent reviews identified critical issues across security, performance, code quality, UX, and semantic cache implementation. This document provides a phased approach to address all findings.

**Current State:**
| Area | Score | Status |
|------|-------|--------|
| Security | 42/100 | NOT PRODUCTION READY |
| Code Quality | 45/100 | HIGH TECHNICAL DEBT |
| Performance | 35/100 | WILL NOT SCALE |
| UX/Documentation | 62/100 | NEEDS IMPROVEMENT |
| Semantic Cache | 32/100 | ALGORITHM FLAWS |
| Business Viability | 22/100 | OPEN SOURCE FOCUS |

**Target State (Post-Remediation):**
| Area | Target | Timeline |
|------|--------|----------|
| Security | 75/100 | Phase 1-2 |
| Code Quality | 70/100 | Phase 2-3 |
| Performance | 70/100 | Phase 2-3 |
| UX/Documentation | 80/100 | Phase 3-4 |
| Semantic Cache | 70/100 | Phase 2 |

---

## Phase 1: Security Hardening (CRITICAL)
**Timeline: Immediate**
**Effort: 40-60 hours**
**Priority: BLOCKING - Must complete before any production use**

### 1.1 Stop Logging Sensitive Data
**Issue:** API keys, passwords, PII logged in plaintext to `~/.rigrun/audit.log`
**Risk:** CRITICAL - Data breach, credential theft
**Files:** `src/audit.rs`, `src/server/mod.rs`

**Tasks:**
- [ ] Implement secret detection regex patterns:
  ```
  sk-[a-zA-Z0-9]{20,}     # OpenAI keys
  sk-or-[a-zA-Z0-9]{20,}  # OpenRouter keys
  sk-ant-[a-zA-Z0-9]{20,} # Anthropic keys
  Bearer [a-zA-Z0-9-._~+/]+=* # Bearer tokens
  password[=:]["'][^"']+["'] # Password patterns
  ```
- [ ] Redact matched patterns before logging: `sk-or-v1-abc123...` → `sk-or-***REDACTED***`
- [ ] Add configurable `audit_log_redaction: bool` setting (default: true)
- [ ] Truncate query preview to 30 chars (currently 50)
- [ ] Add log rotation: max 100MB, 7 days retention
- [ ] Document what gets logged in privacy docs

**Acceptance Criteria:**
- No API keys appear in audit.log after 100 test queries containing secrets
- Log files auto-rotate at size/age limits
- Privacy documentation updated

---

### 1.2 Add Basic Authentication
**Issue:** API has NO authentication - anyone can use it
**Risk:** HIGH - Open proxy abuse, unauthorized access
**Files:** `src/server/mod.rs`

**Tasks:**
- [ ] Add `--api-key` flag to server startup
- [ ] Add `api_key` to config.json
- [ ] Implement Bearer token middleware for axum
- [ ] Protect sensitive endpoints:
  - `POST /v1/chat/completions` - require auth
  - `GET /stats` - require auth
  - `GET /cache/stats` - require auth
  - `GET /cache/semantic` - require auth
  - `GET /health` - no auth (for load balancers)
  - `GET /v1/models` - no auth (discovery)
- [ ] Return 401 Unauthorized with helpful message
- [ ] Add `--no-auth` flag for local development (with warning)

**Acceptance Criteria:**
- Requests without valid Bearer token return 401
- Config file supports API key
- Warning displayed if running without auth on 0.0.0.0

---

### 1.3 Encrypt Sensitive Storage
**Issue:** Config and cache stored as plaintext JSON
**Risk:** HIGH - API keys readable from disk
**Files:** `src/cache/mod.rs`, `src/config.rs`

**Tasks:**
- [ ] Use system keyring for API keys (keyring crate):
  - macOS: Keychain
  - Windows: Credential Manager
  - Linux: Secret Service (libsecret)
- [ ] Fallback to encrypted file if keyring unavailable
- [ ] Encrypt cache file at rest (AES-256-GCM)
- [ ] Add HMAC for cache integrity verification
- [ ] Migrate existing plaintext configs on upgrade

**Acceptance Criteria:**
- API keys not visible in config.json
- Cache file is encrypted binary, not readable JSON
- Graceful fallback if keyring unavailable

---

### 1.4 Add Security Headers
**Issue:** No security headers on HTTP responses
**Risk:** MEDIUM - XSS, clickjacking if UI added later
**Files:** `src/server/mod.rs`

**Tasks:**
- [ ] Add tower middleware for security headers:
  ```
  X-Content-Type-Options: nosniff
  X-Frame-Options: DENY
  X-XSS-Protection: 1; mode=block
  Content-Security-Policy: default-src 'none'
  Cache-Control: no-store
  ```
- [ ] Add CORS configuration (default: same-origin)
- [ ] Add `--cors-origin` flag for cross-origin access

**Acceptance Criteria:**
- All responses include security headers
- CORS properly configured

---

### 1.5 Rate Limiting Improvements
**Issue:** Current rate limiting easily bypassed (IP-only, high burst)
**Risk:** MEDIUM - DoS, resource exhaustion
**Files:** `src/server/mod.rs`

**Tasks:**
- [ ] Reduce burst size from 60 to 20
- [ ] Add per-API-key rate limiting (if auth enabled)
- [ ] Add cost-based limiting (weight by estimated tokens)
- [ ] Add global rate limit (all clients combined)
- [ ] Implement exponential backoff for repeat offenders
- [ ] Add `X-RateLimit-*` headers to responses

**Acceptance Criteria:**
- Burst limited to 20 requests
- Rate limit headers in responses
- Authenticated users get higher limits

---

### 1.6 Connection Limits
**Issue:** No limit on concurrent connections
**Risk:** MEDIUM - Slowloris attacks, resource exhaustion
**Files:** `src/server/mod.rs`

**Tasks:**
- [ ] Add max concurrent connections limit (default: 100)
- [ ] Add connection timeout (default: 30s)
- [ ] Reduce request timeout from 120s to 60s
- [ ] Add `--max-connections` flag

**Acceptance Criteria:**
- Server rejects connections above limit with 503
- Idle connections closed after timeout

---

## Phase 2: Performance & Algorithm Fixes
**Timeline: After Phase 1**
**Effort: 60-80 hours**
**Priority: HIGH - Required for any real usage**

### 2.1 Fix Lock Contention (CRITICAL)
**Issue:** Write lock held during 60-second Ollama embedding calls
**Risk:** CRITICAL - Serializes ALL requests, 10x latency under load
**Files:** `src/server/mod.rs`, `src/cache/semantic.rs`

**Tasks:**
- [ ] Refactor cache access pattern:
  ```rust
  // BEFORE (broken):
  let mut cache = state.cache.write().await;
  let result = cache.get(query).await; // Holds lock during I/O!

  // AFTER (correct):
  let embedding = generate_embedding(query).await; // No lock
  let cache = state.cache.read().await;
  let result = cache.search_with_embedding(&embedding);
  drop(cache);
  if result.is_none() {
      let mut cache = state.cache.write().await;
      cache.store(query, embedding, response).await;
  }
  ```
- [ ] Separate read lock for lookups, write lock only for stores
- [ ] Move embedding generation completely outside lock
- [ ] Add embedding cache (separate from response cache)
- [ ] Implement request coalescing for identical queries

**Acceptance Criteria:**
- 100 concurrent requests complete without timeout
- p99 latency < 5s under load
- No lock held during network I/O

---

### 2.2 Replace O(n) Vector Search
**Issue:** Brute-force linear scan - 800ms at 100k entries
**Risk:** CRITICAL - Slower than just calling the LLM
**Files:** `src/cache/vector_index.rs`

**Tasks:**
- [ ] Integrate HNSW library (hnswlib-rs or instant-distance)
- [ ] Configure HNSW parameters:
  - M: 16 (connections per node)
  - ef_construction: 200 (build quality)
  - ef_search: 50 (search quality)
- [ ] Add SIMD optimization for cosine similarity
- [ ] Pre-normalize embeddings (store unit vectors)
- [ ] Benchmark: must be <10ms for 100k vectors

**Implementation:**
```rust
// Replace VectorIndex internals with HNSW
use instant_distance::{Builder, Search};

pub struct VectorIndex {
    hnsw: HnswMap<String, [f32; 768]>,
    // ...
}

impl VectorIndex {
    pub fn search(&self, query: &[f32]) -> Option<(String, f32)> {
        let mut search = Search::default();
        let result = self.hnsw.search(query, &mut search);
        // O(log n) instead of O(n)
    }
}
```

**Acceptance Criteria:**
- Search latency < 10ms for 100k vectors
- Memory overhead < 2x vs brute force
- All existing tests pass

---

### 2.3 Raise Similarity Threshold
**Issue:** 0.80 threshold = 15-25% false positive rate
**Risk:** HIGH - Wrong answers returned from cache
**Files:** `src/server/mod.rs`, `src/cache/semantic.rs`

**Tasks:**
- [ ] Raise default threshold from 0.80 to 0.92
- [ ] Add configurable threshold in config.json
- [ ] Add `--similarity-threshold` flag
- [ ] Create evaluation dataset:
  - 100 query pairs that SHOULD match
  - 100 query pairs that should NOT match
  - Calculate precision/recall at different thresholds
- [ ] Log similarity scores for analysis
- [ ] Add threshold validation (0.70 - 0.99)

**Test Cases for Threshold:**
| Query 1 | Query 2 | Expected | Threshold |
|---------|---------|----------|-----------|
| "What is Python?" | "What is Java?" | NO MATCH | Should fail at 0.92 |
| "How to reverse a string" | "Reverse a string in Python" | MATCH | Should pass at 0.92 |
| "Delete all files" | "List all files" | NO MATCH | Should fail at 0.92 |

**Acceptance Criteria:**
- False positive rate < 5% on evaluation dataset
- Configurable threshold documented
- Warning if threshold < 0.85

---

### 2.4 Fix Cache Invalidation
**Issue:** Expired cache entries leave orphaned vectors = memory leak
**Risk:** HIGH - Unbounded memory growth
**Files:** `src/cache/semantic.rs`, `src/cache/vector_index.rs`

**Tasks:**
- [ ] Auto-sync vector index when exact cache expires
- [ ] Add TTL tracking to vector index entries
- [ ] Implement periodic background cleanup (every 5 minutes)
- [ ] Add metrics for orphaned vectors
- [ ] Add `rigrun cache clear` command
- [ ] Add `rigrun cache stats` with vector/entry count

**Implementation:**
```rust
// In cleanup_expired()
pub fn cleanup_expired(&mut self) -> usize {
    let removed = self.exact_cache.cleanup_expired();
    self.sync_vector_index(); // Always sync after cleanup
    removed
}

// Background task
tokio::spawn(async move {
    loop {
        tokio::time::sleep(Duration::from_secs(300)).await;
        cache.write().await.cleanup_expired();
    }
});
```

**Acceptance Criteria:**
- Vector count equals cache entry count (±10%)
- Memory stable after 24h continuous use
- Cleanup runs automatically

---

### 2.5 Batch Stats Persistence
**Issue:** Disk write on EVERY request
**Risk:** MEDIUM - I/O bottleneck, disk wear
**Files:** `src/stats/mod.rs`, `src/server/mod.rs`

**Tasks:**
- [ ] Buffer stats in memory
- [ ] Persist every 30 seconds OR every 100 queries
- [ ] Use async file I/O (tokio::fs)
- [ ] Add graceful shutdown to flush pending stats
- [ ] Remove sync disk writes from hot path

**Acceptance Criteria:**
- No disk writes during request handling
- Stats survive server restart
- Graceful shutdown flushes buffer

---

### 2.6 Add Query Validation
**Issue:** No length limits, silent truncation
**Risk:** MEDIUM - Incorrect cache hits, wasted compute
**Files:** `src/cache/semantic.rs`, `src/cache/embeddings.rs`

**Tasks:**
- [ ] Add minimum query length (10 characters)
- [ ] Add maximum query length (8000 characters / ~2000 tokens)
- [ ] Reject empty/whitespace-only queries
- [ ] Log warning for truncated queries
- [ ] Add token counting (tiktoken-rs or similar)
- [ ] Return error for oversized queries (don't silently truncate)

**Acceptance Criteria:**
- Empty queries rejected with clear error
- Oversized queries rejected (not truncated)
- Token count logged for debugging

---

## Phase 3: Code Quality & Reliability
**Timeline: After Phase 2**
**Effort: 40-60 hours**
**Priority: MEDIUM - Required for maintainability**

### 3.1 Fix Error Handling
**Issue:** `.expect()` panics in hot paths, silent failures
**Risk:** HIGH - Server crashes, data loss
**Files:** Multiple

**Tasks:**
- [ ] Audit all `.expect()` calls (8+ found):
  - `src/server/mod.rs:187` - governor config
  - `src/server/mod.rs:961, 963, 980` - signal handlers
  - `src/local/mod.rs:239, 219, 221` - HTTP client
- [ ] Replace with proper Result propagation
- [ ] Add context to all errors (anyhow::Context)
- [ ] Implement graceful degradation for non-critical failures
- [ ] Add error recovery for stats persistence

**Acceptance Criteria:**
- Zero `.expect()` in library code
- All errors have context
- Server doesn't crash on config errors

---

### 3.2 Enable Integration Tests
**Issue:** All integration tests are `#[ignore]`
**Risk:** HIGH - Regressions go undetected
**Files:** `tests/integration_tests.rs`

**Tasks:**
- [ ] Remove `#[ignore]` from all tests
- [ ] Add test fixtures (mock Ollama responses)
- [ ] Set up CI pipeline (GitHub Actions)
- [ ] Add test coverage reporting
- [ ] Target: 60% code coverage

**CI Pipeline:**
```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: dtolnay/rust-toolchain@stable
      - run: cargo test --all-features
      - run: cargo clippy -- -D warnings
      - run: cargo audit
```

**Acceptance Criteria:**
- All tests run in CI
- No `#[ignore]` attributes
- CI blocks merge on failure

---

### 3.3 Deduplicate Types
**Issue:** 3 Message types, 2 Tier enums
**Risk:** MEDIUM - Inconsistency, bugs
**Files:** `src/server/mod.rs`, `src/local/mod.rs`, `src/cloud/mod.rs`, `src/router/mod.rs`, `src/stats/mod.rs`

**Tasks:**
- [ ] Create `src/types.rs` with canonical types
- [ ] Define single `Message` struct
- [ ] Define single `Tier` enum with all variants
- [ ] Update all modules to use common types
- [ ] Add `From` implementations for conversions

**Acceptance Criteria:**
- Single source of truth for Message and Tier
- No duplicate type definitions

---

### 3.4 Fix Concurrency Bugs
**Issue:** Race conditions in cache and stats
**Risk:** HIGH - Data corruption, inconsistent state
**Files:** `src/server/mod.rs`, `src/stats/mod.rs`

**Tasks:**
- [ ] Add loom testing for cache operations
- [ ] Fix stats update atomicity (both session and all-time)
- [ ] Implement proper cache check-and-set
- [ ] Add version numbers to cache entries
- [ ] Use DashMap for concurrent cache access

**Acceptance Criteria:**
- No data corruption under concurrent load
- Stats always consistent
- Loom tests pass

---

### 3.5 Reduce Clone Overhead
**Issue:** Excessive cloning of strings and embeddings
**Risk:** LOW - Performance overhead
**Files:** Multiple

**Tasks:**
- [ ] Replace `String` with `Arc<str>` for cache keys
- [ ] Pre-allocate embedding vectors
- [ ] Use `Cow<str>` for temporary strings
- [ ] Profile with flamegraph, fix hot spots

**Acceptance Criteria:**
- 20% reduction in allocations (measured)
- No unnecessary clones in hot path

---

## Phase 4: UX & Documentation
**Timeline: After Phase 3**
**Effort: 30-40 hours**
**Priority: MEDIUM - Required for adoption**

### 4.1 Improve First-Run Experience
**Issue:** Silent GB downloads, confusing wizard
**Risk:** HIGH - 60% user abandonment
**Files:** `src/main.rs`, `src/setup.rs`

**Tasks:**
- [ ] Show total download size before model pull:
  ```
  Downloading qwen2.5-coder:7b (4.2 GB)
  This is a ONE-TIME download. Future starts are instant.
  [████████████░░░░░░░░] 2.1 GB / 4.2 GB (50%)
  ```
- [ ] Check Ollama BEFORE wizard (not after)
- [ ] Offer to install Ollama automatically
- [ ] Move OpenRouter setup to AFTER first successful query
- [ ] Add "skip for now" option for all optional steps
- [ ] Show clear "what's next" after setup

**Acceptance Criteria:**
- User knows what's downloading and how big
- Can complete setup without OpenRouter
- Clear next steps shown

---

### 4.2 Add `rigrun doctor` Command
**Issue:** Users can't diagnose problems
**Risk:** MEDIUM - Support burden
**Files:** `src/main.rs` (new command)

**Tasks:**
- [ ] Check Ollama installation and version
- [ ] Check Ollama service status
- [ ] Check GPU detection (CUDA/ROCm/Metal)
- [ ] Check available VRAM
- [ ] Check installed models
- [ ] Check rigrun config validity
- [ ] Check port availability
- [ ] Check cache health
- [ ] Test embedding generation
- [ ] Show clear pass/fail for each check

**Output Example:**
```
rigrun doctor

[✓] Ollama installed (v0.5.1)
[✓] Ollama service running
[✓] GPU detected: NVIDIA RTX 3080 (10GB VRAM)
[✓] Model available: qwen2.5-coder:7b
[✓] Config valid
[✓] Port 8787 available
[✓] Cache healthy (1,234 entries)
[✓] Embeddings working (nomic-embed-text)

All checks passed! rigrun is ready to use.
```

**Acceptance Criteria:**
- Single command shows all system status
- Clear error messages for each failure
- Actionable fix suggestions

---

### 4.3 Improve Error Messages
**Issue:** Cryptic errors without solutions
**Risk:** MEDIUM - User frustration
**Files:** Multiple

**Tasks:**
- [ ] Audit all error messages
- [ ] Add "Possible causes" section
- [ ] Add "Try these fixes" section
- [ ] Add documentation links
- [ ] Use consistent error format

**Error Template:**
```
[✗] Failed to connect to Ollama

Possible causes:
  - Ollama service not running
  - Ollama installed but not started
  - Wrong Ollama URL in config

Try these fixes:
  1. Start Ollama: ollama serve
  2. Check status: rigrun doctor
  3. Verify URL: rigrun config show

Need help? https://github.com/jeranaias/rigrun/issues
```

**Acceptance Criteria:**
- All errors have causes and fixes
- Links to relevant docs
- Consistent formatting

---

### 4.4 Restructure Documentation
**Issue:** Information scattered across files
**Risk:** MEDIUM - Users can't find answers
**Files:** `docs/`, `README.md`

**Tasks:**
- [ ] Create single "5-Minute Quickstart" in README
- [ ] Lead with pre-built binaries (not cargo)
- [ ] Reorganize docs/:
  ```
  docs/
  ├── getting-started.md (first-run guide)
  ├── installation.md (all methods)
  ├── configuration.md (all settings)
  ├── api-reference.md (complete API)
  ├── troubleshooting.md (common issues)
  ├── security.md (auth, privacy)
  └── contributing.md
  ```
- [ ] Add "What is rigrun?" section for non-technical users
- [ ] Add architecture diagram
- [ ] Add cost savings calculator

**Acceptance Criteria:**
- README gets user to working setup in 5 minutes
- All docs organized logically
- No duplicate information

---

### 4.5 Simplify CLI
**Issue:** 13 top-level commands is overwhelming
**Risk:** LOW - Usability friction
**Files:** `src/main.rs`

**Tasks:**
- [ ] Group related commands:
  ```
  rigrun                 # Start server (main command)
  rigrun ask "..."       # Quick query
  rigrun chat            # Interactive mode
  rigrun status          # Show stats
  rigrun config [...]    # All config operations
  rigrun setup [...]     # All setup operations (ide, gpu)
  rigrun cache [...]     # All cache operations (stats, clear)
  rigrun doctor          # Diagnose issues
  ```
- [ ] Reduce from 13 to 8 top-level commands
- [ ] Add command aliases (e.g., `rigrun s` = `rigrun status`)
- [ ] Improve --help output with examples

**Acceptance Criteria:**
- Fewer top-level commands
- Logical grouping
- Examples in help text

---

## Phase 5: Future Hardening
**Timeline: Ongoing**
**Effort: Variable**
**Priority: LOW - Nice to have**

### 5.1 Add TLS Support
- [ ] Integrate rustls for HTTPS
- [ ] Auto-generate self-signed certs
- [ ] Support custom certs
- [ ] Add `--tls` and `--cert` flags

### 5.2 Add Observability
- [ ] Prometheus metrics endpoint
- [ ] Structured logging (JSON)
- [ ] Request tracing (OpenTelemetry)
- [ ] Dashboard template (Grafana)

### 5.3 Add Multi-Model Embeddings
- [ ] Support multiple embedding models
- [ ] A/B test model quality
- [ ] Fallback chain for embedding failures

### 5.4 Optimize Binary Size
- [ ] Audit dependencies for bloat
- [ ] Remove unused features
- [ ] Target: <10MB binary

### 5.5 Add Streaming Support
- [ ] SSE for chat completions
- [ ] Chunked transfer encoding
- [ ] Stream-aware caching

---

## Dependency Audit

### Add to CI:
```toml
# Cargo.toml
[dev-dependencies]
cargo-audit = "0.18"
```

```yaml
# GitHub Actions
- run: cargo audit
```

### Reduce Attack Surface:
- [ ] Use minimal tokio features: `["rt", "net", "sync", "time"]`
- [ ] Remove `blocking` feature from reqwest
- [ ] Pin exact versions in Cargo.lock
- [ ] Enable `cargo-deny` for license/security checks

---

## Success Metrics

### Phase 1 Complete When:
- [ ] No secrets in logs (manual audit)
- [ ] Authentication works end-to-end
- [ ] Security headers present
- [ ] Rate limiting improved

### Phase 2 Complete When:
- [ ] 100 concurrent requests succeed
- [ ] Search < 10ms at 100k vectors
- [ ] False positive rate < 5%
- [ ] No memory leaks after 24h

### Phase 3 Complete When:
- [ ] Zero panics in production code
- [ ] 60%+ test coverage
- [ ] CI pipeline green
- [ ] No duplicate types

### Phase 4 Complete When:
- [ ] 80% of new users complete setup
- [ ] `rigrun doctor` catches common issues
- [ ] Docs are comprehensive and organized

---

## Estimated Total Effort

| Phase | Hours | Priority |
|-------|-------|----------|
| Phase 1: Security | 40-60 | CRITICAL |
| Phase 2: Performance | 60-80 | HIGH |
| Phase 3: Code Quality | 40-60 | MEDIUM |
| Phase 4: UX/Docs | 30-40 | MEDIUM |
| Phase 5: Future | Variable | LOW |
| **Total** | **170-240 hours** | - |

At 20 hours/week part-time: **8-12 weeks**
At 40 hours/week full-time: **4-6 weeks**

---

## Risk Assessment

### If We Don't Fix Phase 1:
- API keys will leak
- Cache poisoning will occur
- Users will be exposed to attacks
- **DO NOT SHIP WITHOUT PHASE 1**

### If We Don't Fix Phase 2:
- Performance degrades with usage
- False positives frustrate users
- Memory grows unbounded
- **Usable only for demos**

### If We Don't Fix Phase 3:
- Bugs accumulate
- Maintenance becomes painful
- Contributors avoid the codebase
- **Technical debt compounds**

### If We Don't Fix Phase 4:
- Users abandon during setup
- Support burden increases
- Adoption stalls
- **Good software nobody uses**

---

## Conclusion

This remediation plan addresses all critical findings from the six independent reviews. The phased approach ensures:

1. **Security first** - No production use until Phase 1 complete
2. **Performance second** - Usable at scale after Phase 2
3. **Quality third** - Maintainable after Phase 3
4. **Polish fourth** - Adoptable after Phase 4

The total effort is significant (170-240 hours) but the alternative is shipping broken software that will harm users and damage reputation.

**Recommendation:** Complete Phase 1 before any public announcement. Complete Phase 2 before any production recommendations. Complete Phase 3-4 for long-term success.

---

*Document Version: 1.0*
*Created: 2026-01-12*
*Based on: Security Audit, Code Review, Business Analysis, Performance Review, UX Review, Algorithm Review*
