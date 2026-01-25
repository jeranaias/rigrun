# rigrun Test Suite Architecture

> **Vision:** A comprehensive test suite that validates rigrun as "the fastest, smartest, most private LLM router with true semantic caching" - covering security, compliance, performance, and functionality across both Rust and Go implementations.

---

## Test Categories Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        RIGRUN TEST PYRAMID                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│                           ┌─────────────┐                                   │
│                           │  E2E Tests  │  (10%)                            │
│                           │  Full Flow  │  Cross-language integration       │
│                           └──────┬──────┘                                   │
│                                  │                                          │
│                    ┌─────────────┴─────────────┐                            │
│                    │   Integration Tests       │  (20%)                     │
│                    │   Multi-component flows   │                            │
│                    └─────────────┬─────────────┘                            │
│                                  │                                          │
│          ┌───────────────────────┴───────────────────────┐                  │
│          │              Security & Compliance            │  (25%)           │
│          │   Classification, IL5, NIST 800-53, STIG     │                  │
│          └───────────────────────┬───────────────────────┘                  │
│                                  │                                          │
│   ┌──────────────────────────────┴──────────────────────────────┐           │
│   │                      Unit Tests                              │  (45%)   │
│   │   Per-module, per-function, pure logic validation           │           │
│   └─────────────────────────────────────────────────────────────┘           │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 1. Unit Tests

### 1.1 Rust Server (`src/`)

| Module | Test File | Coverage Target | Key Test Cases |
|--------|-----------|-----------------|----------------|
| `router/` | `router_test.rs` | 95% | Classification routing, tier selection, complexity scoring |
| `cache/semantic.rs` | `semantic_test.rs` | 90% | Embedding similarity, threshold matching, LRU eviction |
| `cache/embeddings.rs` | `embeddings_test.rs` | 85% | Vector generation, dimension validation |
| `cache/vector_index.rs` | `vector_index_test.rs` | 90% | HNSW search, index persistence, concurrent access |
| `local/` | `ollama_test.rs` | 80% | Model detection, streaming, error handling |
| `cloud/` | `openrouter_test.rs` | 85% | API calls, retry logic, rate limiting |
| `detect/` | `gpu_detect_test.rs` | 75% | GPU detection, VRAM parsing, model recommendations |
| `audit/` | `audit_test.rs` | 95% | Log integrity, secret redaction, rotation |
| `security/` | `session_test.rs` | 95% | Timeout, lockout, RBAC |
| `classification_ui/` | `classification_test.rs` | 100% | All 5 levels, CUI markings, routing enforcement |
| `consent_banner/` | `consent_test.rs` | 100% | DoD banner text, acknowledgment persistence |
| `stats/` | `stats_test.rs` | 90% | Cost calculation, savings tracking, aggregation |
| `health/` | `health_test.rs` | 85% | Component checks, degraded states |
| `errors/` | `errors_test.rs` | 90% | IL5 error format, no info leakage |

### 1.2 Go TUI (`go-tui/internal/`)

| Package | Test File | Coverage Target | Key Test Cases |
|---------|-----------|-----------------|----------------|
| `router/` | `router_test.go` | 95% | Routing decisions, tier escalation, classification |
| `cache/` | `cache_test.go` | 90% | Exact + semantic lookup, eviction, persistence |
| `ollama/` | `ollama_test.go` | 80% | Client operations, streaming, model switching |
| `cloud/` | `cloud_test.go` | 85% | OpenRouter API, cost tracking, model mapping |
| `tools/` | `tools_test.go` | 90% | All 6 tools, permission levels, sandboxing |
| `commands/` | `commands_test.go` | 85% | All 50+ slash commands, argument parsing |
| `cli/` | `cli_test.go` | 90% | **ask, chat, intel commands**, flag parsing |
| `security/` | `security_test.go` | 95% | Audit logging, classification enforcement |
| `session/` | `session_test.go` | 90% | Timeout, activity tracking, persistence |
| `config/` | `config_test.go` | 85% | Load/save, validation, env overrides |
| `detect/` | `detect_test.go` | 75% | GPU detection, model recommendations |
| `context/` | `context_test.go` | 85% | @ mention parsing, file/git/clipboard expansion |
| `model/` | `model_test.go` | 80% | Conversation, message types, serialization |
| `storage/` | `storage_test.go` | 85% | Conversation persistence, search, cleanup |
| `util/` | `util_test.go` | 90% | String helpers, atomic operations |
| `ui/chat/` | `chat_test.go` | 70% | State transitions, key handling |
| `ui/components/` | `components_test.go` | 70% | Rendering, theming |

---

## 2. Security & Compliance Tests

### 2.1 Classification Routing (CRITICAL - 100% Required)

```
tests/security/
├── classification_routing_test.go     # Go implementation
├── classification_routing_test.rs     # Rust implementation
├── testdata/
│   ├── unclassified_queries.json      # 500+ UNCLASSIFIED test cases
│   ├── cui_queries.json               # 200+ CUI test cases
│   ├── fouo_queries.json              # 100+ FOUO test cases
│   ├── secret_queries.json            # 100+ SECRET test cases
│   └── top_secret_queries.json        # 50+ TOP SECRET test cases
└── adversarial/
    ├── injection_attempts.json        # 100+ prompt injection attempts
    ├── boundary_bypass.json           # 50+ classification bypass attempts
    └── encoding_attacks.json          # Unicode, base64, etc.
```

**Test Matrix:**

| Classification | Expected Route | Cloud Allowed | Audit Required | Test Count |
|----------------|---------------|---------------|----------------|------------|
| UNCLASSIFIED | Any | Yes | Optional | 500+ |
| CUI | Local Only | **NEVER** | **ALWAYS** | 200+ |
| FOUO | Local Only | **NEVER** | **ALWAYS** | 100+ |
| SECRET | Local Only | **NEVER** | **ALWAYS** | 100+ |
| TOP SECRET | Local Only | **NEVER** | **ALWAYS** | 50+ |

**Adversarial Tests (53+ scenarios from README claim):**
- Prompt injection to bypass classification
- Unicode homoglyphs to confuse parsers
- Context manipulation attacks
- Tier escalation attempts
- Air-gap bypass attempts

### 2.2 IL5/DoD STIG Compliance

```
tests/compliance/
├── il5_controls_test.go
├── nist_800_53_test.go
├── stig_checklist_test.go
└── controls/
    ├── ac_access_control.json         # AC-1 through AC-25
    ├── au_audit.json                  # AU-1 through AU-16
    ├── cm_config_mgmt.json            # CM-1 through CM-11
    ├── ia_identification.json         # IA-1 through IA-12
    ├── sc_system_comms.json           # SC-1 through SC-45
    └── si_system_integrity.json       # SI-1 through SI-17
```

**NIST 800-53 Control Mapping:**

| Control | Description | Test Cases |
|---------|-------------|------------|
| AC-4 | Information Flow Enforcement | Classification routing, no cloud for CUI+ |
| AC-5 | Separation of Duties | RBAC validation |
| AC-6 | Least Privilege | Tool permissions, API access |
| AC-8 | System Use Notification | Consent banner display |
| AC-12 | Session Termination | Timeout enforcement (15-30 min) |
| AU-2 | Audit Events | All query logging |
| AU-6 | Audit Review | Log integrity, tamper detection |
| AU-9 | Protection of Audit Info | HMAC signing, secure storage |
| SC-7 | Boundary Protection | Air-gap mode, network blocking |
| SC-8 | Transmission Confidentiality | TLS enforcement |
| SC-13 | Cryptographic Protection | FIPS 140-2 algorithms |
| SC-17 | PKI Certificates | Certificate validation |
| SI-11 | Error Handling | No sensitive info in errors |

### 2.3 Brute Force Tests (909 scenarios from README claim)

```
tests/security/brute_force/
├── tier_escalation_test.go            # Force cloud routing of classified content
├── cache_poisoning_test.go            # Inject malicious cached responses
├── session_hijacking_test.go          # Token replay, session fixation
├── rate_limit_bypass_test.go          # Overwhelm defenses
└── scenarios/
    ├── batch_001_200.json             # Scenarios 1-200
    ├── batch_201_400.json             # Scenarios 201-400
    ├── batch_401_600.json             # Scenarios 401-600
    ├── batch_601_800.json             # Scenarios 601-800
    └── batch_801_909.json             # Scenarios 801-909
```

---

## 3. Integration Tests

### 3.1 Rust Server Integration

```
tests/integration/
├── server_integration_test.rs
├── cache_routing_test.rs
├── cloud_fallback_test.rs
├── classification_flow_test.rs
└── full_pipeline_test.rs
```

**Key Integration Flows:**
1. Query → Classification → Cache Check → Route → Response
2. Cache Miss → Local Ollama → Store Response → Return
3. Local Failure → Cloud Fallback (UNCLASSIFIED only) → Return
4. Session Start → Consent → Activity → Timeout → Lock

### 3.2 Go TUI Integration

```
go-tui/internal/
├── integration_test.go                # Already exists, expand
├── ac4_e2e_test.go                    # Already exists, expand
├── concurrency_test.go                # Already exists, expand
└── cli_integration_test.go            # NEW: Test ask/chat/intel flows
```

**CLI Command Integration Tests:**

| Command | Test Scenarios |
|---------|---------------|
| `rigrun ask "question"` | Single query, routing, output format |
| `rigrun ask --agentic "task"` | Tool use, iteration limits, cloud fallback |
| `rigrun ask --model X "question"` | Model selection, openrouter/auto |
| `rigrun chat` | Interactive session, history, commands |
| `rigrun intel "company"` | Web search, synthesis, cost tracking |

---

## 4. End-to-End Tests

### 4.1 Cross-Language Integration

```
tests/e2e/
├── rust_go_interop_test.go            # Go TUI talks to Rust server
├── full_stack_test.sh                 # Shell script orchestrating both
├── docker_compose_test.yaml           # Containerized E2E
└── scenarios/
    ├── developer_workflow.json        # Typical dev usage
    ├── government_workflow.json       # IL5 compliance flow
    └── airgap_workflow.json           # Fully offline operation
```

### 4.2 IDE Integration Tests

```
tests/e2e/ide/
├── vscode_test.js                     # VS Code extension testing
├── cursor_test.js                     # Cursor integration
├── jetbrains_test.kt                  # IntelliJ plugin
└── neovim_test.lua                    # Neovim configuration
```

---

## 5. Performance & Benchmark Tests

### 5.1 Benchmarks

```
benchmarks/
├── cache_benchmark.rs                 # Semantic cache performance
├── routing_benchmark.rs               # Classification speed
├── embedding_benchmark.rs             # Vector generation
├── search_benchmark.rs                # HNSW search at scale
├── throughput_benchmark.rs            # Requests/second
└── memory_benchmark.rs                # Memory footprint
```

**Performance Targets:**

| Metric | Target | Test Method |
|--------|--------|-------------|
| Cache lookup (exact) | <1ms | 10,000 lookups |
| Cache lookup (semantic) | <10ms | 10,000 lookups with embeddings |
| Embedding generation | <50ms | Per-query embedding |
| Classification | <5ms | Pattern matching + heuristics |
| Local inference (first token) | <500ms | Ollama cold start |
| Cloud fallback | <2s | OpenRouter round-trip |
| Memory (idle) | <50MB | Server with empty cache |
| Memory (100k cache) | <500MB | Full semantic cache |
| Binary size | <20MB | Release build |

### 5.2 Load Tests

```
tests/load/
├── concurrent_users_test.go           # 100+ simultaneous users
├── sustained_load_test.go             # 1 hour continuous load
├── spike_test.go                      # Sudden traffic bursts
└── recovery_test.go                   # Behavior after overload
```

---

## 6. Platform & Compatibility Tests

### 6.1 Cross-Platform Matrix

| Platform | Rust Server | Go TUI | GPU Detection | Test Priority |
|----------|-------------|--------|---------------|---------------|
| Windows 11 | ✓ | ✓ | NVIDIA, AMD, Intel | High |
| Windows 10 | ✓ | ✓ | NVIDIA, AMD | High |
| macOS ARM | ✓ | ✓ | Apple Silicon | High |
| macOS Intel | ✓ | ✓ | None | Medium |
| Ubuntu 22.04 | ✓ | ✓ | NVIDIA, AMD | High |
| Ubuntu 20.04 | ✓ | ✓ | NVIDIA | Medium |
| Debian 12 | ✓ | ✓ | NVIDIA | Medium |
| Alpine (Docker) | ✓ | ✓ | None | High |

### 6.2 GPU Compatibility Tests

```
tests/gpu/
├── nvidia_cuda_test.go
├── amd_rocm_test.go
├── apple_metal_test.go
├── intel_arc_test.go
└── no_gpu_fallback_test.go
```

---

## 7. Test Infrastructure

### 7.1 CI/CD Pipeline

```yaml
# .github/workflows/test.yml
name: Full Test Suite

on: [push, pull_request]

jobs:
  unit-tests:
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]
    steps:
      - run: cargo test --all
      - run: go test ./...

  security-tests:
    steps:
      - run: cargo test --test security_tests
      - run: go test -tags=security ./...

  integration-tests:
    needs: unit-tests
    steps:
      - run: docker-compose up -d
      - run: cargo test --test integration_tests -- --ignored
      - run: go test -tags=integration ./...

  e2e-tests:
    needs: integration-tests
    steps:
      - run: ./tests/e2e/run_all.sh

  benchmarks:
    if: github.ref == 'refs/heads/main'
    steps:
      - run: cargo bench
      - run: go test -bench=. ./...
```

### 7.2 Test Data Management

```
testdata/
├── queries/
│   ├── classification/                # Queries by classification level
│   ├── complexity/                    # Queries by complexity tier
│   └── adversarial/                   # Attack scenarios
├── fixtures/
│   ├── config/                        # Test configurations
│   ├── cache/                         # Pre-populated cache states
│   └── audit/                         # Sample audit logs
├── golden/
│   ├── responses/                     # Expected API responses
│   └── outputs/                       # Expected CLI outputs
└── mocks/
    ├── ollama/                        # Mock Ollama responses
    └── openrouter/                    # Mock OpenRouter responses
```

### 7.3 Test Utilities

```go
// go-tui/internal/testutil/testutil.go
package testutil

// MockOllamaServer creates a local mock Ollama for testing
func MockOllamaServer() *httptest.Server

// MockOpenRouterServer creates a mock OpenRouter API
func MockOpenRouterServer() *httptest.Server

// ClassificationTestCase represents a classification test scenario
type ClassificationTestCase struct {
    Query          string
    Classification string
    ExpectedTier   string
    ShouldBlock    bool
}

// LoadClassificationTests loads test cases from JSON
func LoadClassificationTests(path string) []ClassificationTestCase

// AssertNoCloudLeak verifies no classified data reached cloud
func AssertNoCloudLeak(t *testing.T, auditLog string)
```

---

## 8. Coverage Requirements

### Minimum Coverage Gates

| Category | Coverage | Enforcement |
|----------|----------|-------------|
| Security-critical code | 100% | Block merge if below |
| Classification routing | 100% | Block merge if below |
| Audit logging | 95% | Warning if below |
| Core routing logic | 90% | Warning if below |
| CLI commands | 85% | Informational |
| UI components | 70% | Informational |
| Overall codebase | 80% | Target |

### Coverage Commands

```bash
# Rust coverage
cargo tarpaulin --out Html --output-dir coverage/

# Go coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage/index.html

# Combined report
./scripts/coverage_report.sh
```

---

## 9. Test Commands Reference

```bash
# Run all tests
make test

# Run specific categories
make test-unit
make test-integration
make test-security
make test-e2e
make test-bench

# Run with coverage
make test-coverage

# Run classification tests only (CRITICAL)
make test-classification

# Run adversarial tests
make test-adversarial

# Run IL5 compliance tests
make test-compliance

# Run specific module tests
cargo test router::
go test ./internal/cli/...

# Run with verbose output
cargo test -- --nocapture
go test -v ./...

# Run benchmarks
cargo bench
go test -bench=. -benchmem ./...
```

---

## 10. Implementation Priority

### Phase 1: Foundation (Week 1-2)
1. ✅ Fix existing failing tests
2. Add CLI tests (`internal/cli/`) for ask, chat, intel
3. Add classification routing tests (100% coverage)
4. Set up test infrastructure (mocks, fixtures)

### Phase 2: Security (Week 3-4)
1. Implement all 909 brute force test scenarios
2. Implement all 53 adversarial attack tests
3. Add NIST 800-53 control validation
4. Add IL5 compliance test suite

### Phase 3: Integration (Week 5-6)
1. Cross-language E2E tests (Rust server + Go TUI)
2. IDE integration tests
3. Docker/container tests
4. CI/CD pipeline setup

### Phase 4: Performance (Week 7-8)
1. Benchmark suite for all critical paths
2. Load testing infrastructure
3. Memory profiling
4. Regression detection

### Phase 5: Polish (Week 9-10)
1. Coverage gap analysis
2. Documentation for test writing
3. Test data expansion
4. Continuous improvement process

---

## Summary

This test suite validates rigrun's core promise:

> **"The only LLM router built for DoD/IL5 classification requirements"**

With:
- **100% classification routing accuracy** - validated by 1000+ test cases
- **Zero cloud leakage for CUI+** - validated by 909 brute force + 53 adversarial tests
- **IL5 compliance** - validated by NIST 800-53 control mapping
- **Cross-platform reliability** - validated on Windows, macOS, Linux
- **Performance guarantees** - validated by comprehensive benchmarks

Every claim in the README becomes a test. Every security guarantee becomes enforced.
