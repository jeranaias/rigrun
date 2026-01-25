# RIGRUN RUST → GO PORTING BATTLE PLAN

## 8 PHASES × 4 AGENTS = 32 TOTAL AGENTS

**Base Directory**: `C:\rigrun\go-tui`
**Reference**: `C:\rigrun2-ratatui-archive`

---

## PHASE 1: CORE ROUTING + TYPES (4 Agents)

### Agent 1-1: Types and Enums
**Files to create**: `internal/router/types.go`
- Define `Tier` type with constants (Cache, Local, Cloud, Haiku, Sonnet, Opus, Gpt4o)
- Define `QueryComplexity` type (Trivial, Simple, Moderate, Complex, Expert)
- Define `QueryType` type (Lookup, Explanation, CodeGeneration, Refactoring, Architecture, Debugging, Review, Planning, General)
- Define `RoutingDecision` struct
- Define `QueryResult` struct
- Add String() methods for all types
- Reference: `C:\rigrun2-ratatui-archive\src\types.rs` and `src\router\mod.rs`

### Agent 1-2: Query Classification
**Files to create**: `internal/router/classify.go`
- Implement `ClassifyComplexity(query string) QueryComplexity`
- Implement `ClassifyType(query string) QueryType`
- Use keyword matching from Rust version
- Word count thresholds: <5 trivial, 5-10 simple, 10-15 moderate, 15+ complex
- Keywords: "architect", "design pattern", "trade-off" → Expert
- Keywords: "explain", "compare", "analyze", "implement", "code" → Complex
- Reference: `C:\rigrun2-ratatui-archive\src\router\mod.rs` lines 255-298

### Agent 1-3: Routing Logic
**Files to create**: `internal/router/router.go`
- Implement `RouteQuery(query string, maxTier *Tier) Tier`
- Implement `RouteQueryDetailed(query string, maxTier *Tier) RoutingDecision`
- Implement `(t Tier) Escalate() *Tier` for tier escalation
- Implement `(c QueryComplexity) MinTier() Tier`
- Reference: `C:\rigrun2-ratatui-archive\src\router\mod.rs` lines 300-360

### Agent 1-4: Cost Calculation
**Files to create**: `internal/router/cost.go`
- Implement `(t Tier) InputCostPer1K() float64`
- Implement `(t Tier) OutputCostPer1K() float64`
- Implement `(t Tier) CalculateCostCents(inputTokens, outputTokens int) float64`
- Implement `(t Tier) TypicalLatencyMs() int`
- Implement `(t Tier) Name() string`
- Pricing: Haiku $0.025/1K, Sonnet $0.3/1K, Opus $1.5/1K, GPT-4o $0.25/1K
- Reference: `C:\rigrun2-ratatui-archive\src\router\mod.rs` lines 19-107

---

## PHASE 2: CACHING SYSTEM (4 Agents)

### Agent 2-1: Exact-Match Cache
**Files to create**: `internal/cache/exact.go`
- Implement `ExactCache` struct with `map[string]CacheEntry`
- Implement `CacheEntry` with Response, ExpiresAt, CreatedAt
- Implement `Get(key string) (string, bool)`
- Implement `Set(key, response string, ttl time.Duration)`
- Implement TTL expiration check
- Use `sync.RWMutex` for thread safety
- Reference: `C:\rigrun2-ratatui-archive\src\cache\mod.rs`

### Agent 2-2: Cache Persistence
**Files to create**: `internal/cache/storage.go`
- Implement `SaveToFile(path string) error`
- Implement `LoadFromFile(path string) error`
- JSON format for cache entries
- Location: `~/.rigrun/cache.json`
- Implement LRU eviction when cache exceeds max size
- Reference: `C:\rigrun2-ratatui-archive\src\cache\mod.rs`

### Agent 2-3: Semantic Cache Foundation
**Files to create**: `internal/cache/semantic.go`
- Define `SemanticCache` interface
- Implement basic cosine similarity function
- Implement `FindSimilar(query string, threshold float64) (string, float64, bool)`
- Stub embedding generation (will call Ollama /api/embeddings later)
- Default threshold: 0.92
- Reference: `C:\rigrun2-ratatui-archive\src\cache\semantic.rs`

### Agent 2-4: Cache Manager
**Files to create**: `internal/cache/manager.go`
- Implement `CacheManager` that combines exact + semantic
- Implement `Lookup(query string) (string, bool, CacheHitType)`
- Implement `Store(query, response string)`
- Implement `Stats() CacheStats` (hits, misses, hit rate)
- Implement background cleanup goroutine for expired entries
- Reference: `C:\rigrun2-ratatui-archive\src\cache\mod.rs`

---

## PHASE 3: GPU DETECTION (4 Agents)

### Agent 3-1: NVIDIA Detection
**Files to create**: `internal/detect/nvidia.go`
- Implement `DetectNvidia() (*GpuInfo, error)`
- Call `nvidia-smi --query-gpu=name,memory.total,driver_version --format=csv`
- Parse output to extract GPU name, VRAM, driver version
- Validate driver recency
- Reference: `C:\rigrun2-ratatui-archive\src\detect\mod.rs`

### Agent 3-2: AMD Detection
**Files to create**: `internal/detect/amd.go`
- Implement `DetectAMD() (*GpuInfo, error)`
- Call `rocm-smi --showmeminfo vram --showproductname`
- Detect AMD architecture (RDNA, RDNA2, RDNA3, GCN)
- Check HSA_OVERRIDE_GFX_VERSION
- Parse VRAM from output
- Reference: `C:\rigrun2-ratatui-archive\src\detect\mod.rs`

### Agent 3-3: Model Recommendation
**Files to create**: `internal/detect/recommend.go`
- Implement `RecommendModel(vramGB int) string`
- 8GB → qwen2.5-coder:7b
- 16GB → qwen2.5-coder:14b
- 24GB+ → larger models
- Implement `EstimateModelVRAM(modelName string) int`
- Implement `WillModelFit(modelName string, availableVRAM int) bool`
- Reference: `C:\rigrun2-ratatui-archive\src\detect\mod.rs`

### Agent 3-4: GPU Utilities
**Files to create**: `internal/detect/detect.go`
- Implement `DetectGPU() (*GpuInfo, error)` - tries NVIDIA, then AMD, then CPU
- Implement `DetectGPUCached() (*GpuInfo, error)` - 5 minute cache
- Implement `GpuInfo` struct (Name, Type, VRAM, Driver, Architecture)
- Implement `GpuType` enum (NVIDIA, AMD, AppleSilicon, IntelArc, CPU)
- Implement `GetGPUMemoryUsage() (used, total int, error)`
- Reference: `C:\rigrun2-ratatui-archive\src\detect\mod.rs`

---

## PHASE 4: TOOLS + AGENTIC LOOP (4 Agents)

### Agent 4-1: Tool Interface and Registry
**Files to create**: `internal/tools/types.go`, `internal/tools/registry.go`
- Define `Tool` interface with `Name()`, `Description()`, `Schema()`, `Execute()`
- Define `ToolResult` struct
- Define `PermissionLevel` enum (Low, Medium, High, Critical)
- Implement `Registry` struct to hold all tools
- Implement `GetTool(name string) Tool`
- Implement `ListTools() []ToolInfo`
- Reference: `C:\rigrun2-ratatui-archive\src\tools\registry.rs`

### Agent 4-2: File Tools (read, write, edit)
**Files to create**: `internal/tools/file.go`
- Implement `ReadTool` - read file contents with line numbers
- Implement `WriteTool` - create/overwrite files
- Implement `EditTool` - find and replace in files
- All tools implement the Tool interface
- Add path validation (no system paths)
- Reference: `C:\rigrun2-ratatui-archive\src\tools\builtin\read.rs`, `write.rs`, `edit.rs`

### Agent 4-3: Search Tools (glob, grep, bash)
**Files to create**: `internal/tools/search.go`, `internal/tools/bash.go`
- Implement `GlobTool` - find files by pattern using filepath.Glob
- Implement `GrepTool` - search file contents using regexp
- Implement `BashTool` - execute shell commands
- Add safety validation for bash (no dangerous commands)
- Reference: `C:\rigrun2-ratatui-archive\src\tools\builtin\glob.rs`, `grep.rs`, `bash.rs`

### Agent 4-4: Agentic Loop
**Files to create**: `internal/tools/executor.go`, `internal/tools/agentic.go`
- Implement `Executor` that runs tools with permission checks
- Implement `AgenticLoop` that:
  - Sends message to model
  - Parses tool calls from response
  - Executes tools
  - Formats results back to model
  - Loops until model says "done" or max iterations
- Handle streaming during tool execution
- Reference: `C:\rigrun2-ratatui-archive\src\tools\agentic_loop.rs`, `executor.rs`

---

## PHASE 5: CLI COMMANDS (4 Agents)

### Agent 5-1: CLI Structure
**Files to modify**: `main.go`
**Files to create**: `internal/cli/cli.go`
- Refactor main.go to use command routing
- Default command: TUI (no args)
- Add flag parsing: --paranoid, --skip-banner, --quiet, --verbose, --model
- Implement command dispatcher
- Reference: `C:\rigrun2-ratatui-archive\src\main.rs` lines 326-500

### Agent 5-2: Ask and Chat Commands
**Files to create**: `internal/cli/ask.go`, `internal/cli/chat.go`
- Implement `rigrun ask "question"` - single query with routing
- Implement `rigrun ask "question" --model X` - specific model
- Implement `rigrun ask "question" --file code.rs` - include file
- Implement `rigrun chat` - legacy interactive mode (non-TUI)
- Show tier selection and cost after each query
- Reference: `C:\rigrun2-ratatui-archive\src\main.rs`

### Agent 5-3: Status, Config, Doctor Commands
**Files to create**: `internal/cli/status.go`, `internal/cli/config.go`, `internal/cli/doctor.go`
- Implement `rigrun status` - show current stats, model, GPU
- Implement `rigrun config show` - display config
- Implement `rigrun config set KEY VALUE`
- Implement `rigrun doctor` - run health checks
- Implement `rigrun doctor --fix` - auto-fix issues
- Reference: `C:\rigrun2-ratatui-archive\src\main.rs`, `src\health\mod.rs`

### Agent 5-4: Setup and Cache Commands
**Files to create**: `internal/cli/setup.go`, `internal/cli/cache.go`
- Implement `rigrun setup` - first-run wizard
- Implement `rigrun setup --quick` - minimal setup
- Implement `rigrun cache stats` - show cache metrics
- Implement `rigrun cache clear` - clear all cache
- Reference: `C:\rigrun2-ratatui-archive\src\main.rs`, `src\firstrun\mod.rs`

---

## PHASE 6: SECURITY/COMPLIANCE (4 Agents)

### Agent 6-1: Session Manager
**Files to create**: `internal/security/session.go`
- Implement `SessionManager` struct
- 15-minute session timeout (STIG AC-12)
- Warning at 13 minutes
- Implement `StartSession()`, `RefreshSession()`, `IsExpired() bool`
- Implement timeout callback for re-authentication
- Use `time.Timer` for timeout tracking
- Reference: `C:\rigrun2-ratatui-archive\src\security\session_manager.rs`

### Agent 6-2: DoD Consent Banner
**Files to create**: `internal/security/banner.go`
- Implement exact DoD IS warning banner text
- Implement `ShowBanner() error` - display and require acknowledgment
- Implement `LogBannerAcknowledgment()` - audit trail
- Handle --skip-banner flag (for CI, but log it)
- Reference: `C:\rigrun2-ratatui-archive\src\consent_banner.rs`

### Agent 6-3: Audit Logging
**Files to create**: `internal/security/audit.go`
- Implement `AuditLogger` struct
- Log to `~/.rigrun/audit.log`
- Format: `YYYY-MM-DD HH:MM:SS | TIER | "query..." | tokens | cost`
- Implement secret redaction (API keys, passwords, tokens)
- Regex patterns for: OpenAI keys, GitHub tokens, AWS keys, Bearer tokens
- Reference: `C:\rigrun2-ratatui-archive\src\audit.rs`

### Agent 6-4: Classification UI
**Files to create**: `internal/security/classification.go`
- Implement classification banner rendering
- Levels: UNCLASSIFIED, CUI, SECRET, TOP SECRET
- Top and bottom banners with appropriate colors
- CUI designations: NOFORN, IMCON, etc.
- Reference: `C:\rigrun2-ratatui-archive\src\classification_ui.rs`

---

## PHASE 7: CLOUD CLIENT + SERVER API (4 Agents)

### Agent 7-1: OpenRouter Client
**Files to create**: `internal/cloud/client.go`
- Implement `OpenRouterClient` struct
- Implement `Chat(messages []Message) (Response, error)`
- Implement `ChatStream(messages []Message, callback func(token string))`
- Handle API key from config
- Reference: `C:\rigrun2-ratatui-archive\src\cloud\mod.rs`

### Agent 7-2: OpenRouter Streaming
**Files to create**: `internal/cloud/stream.go`
- Implement SSE parsing for streaming responses
- Handle partial JSON chunks
- Implement retry logic with exponential backoff
- Handle rate limiting (429 responses)
- Reference: `C:\rigrun2-ratatui-archive\src\cloud\mod.rs`

### Agent 7-3: HTTP Server Setup
**Files to create**: `internal/server/server.go`
- Implement HTTP server on port 8787
- Implement `/v1/chat/completions` endpoint (OpenAI compatible)
- Implement `/v1/models` endpoint
- Implement `/stats` endpoint
- Reference: `C:\rigrun2-ratatui-archive\src\server\mod.rs`

### Agent 7-4: Server Security
**Files to create**: `internal/server/middleware.go`
- Implement Bearer token authentication
- Implement CORS middleware
- Implement rate limiting
- Implement request logging
- Session timeout enforcement
- Reference: `C:\rigrun2-ratatui-archive\src\server\mod.rs`

---

## PHASE 8: INTEGRATION + TESTING (4 Agents)

### Agent 8-1: Router Tests
**Files to create**: `internal/router/router_test.go`, `internal/router/classify_test.go`
- Test all complexity classifications
- Test all query type classifications
- Test routing decisions
- Test cost calculations
- Test tier escalation
- Reference: `C:\rigrun2-ratatui-archive\src\router\mod.rs` tests section

### Agent 8-2: Integration Tests
**Files to create**: `internal/integration_test.go`
- End-to-end routing test (cache → local → cloud)
- Tool execution tests
- Cache hit/miss tests
- Session timeout tests

### Agent 8-3: TUI Integration
**Files to modify**: `internal/ui/chat/model.go`, `internal/ui/chat/view.go`
- Integrate router into TUI
- Display tier selection reasoning
- Show cost/tokens in status bar
- Add routing mode indicator
- Tool result rendering in chat

### Agent 8-4: Polish and Documentation
**Files to create**: `README.md` updates, `internal/config/config.go`
- Implement unified config loading (TOML or JSON)
- Config file at `~/.rigrun/config.toml`
- Update README with new features
- Add inline documentation
- Cleanup unused code

---

## EXECUTION ORDER

```
PHASE 1 ──────────────────────────────────────────────────────────
  [Agent 1-1: Types]     [Agent 1-2: Classify]
  [Agent 1-3: Router]    [Agent 1-4: Cost]
                              ↓
PHASE 2 ──────────────────────────────────────────────────────────
  [Agent 2-1: Exact]     [Agent 2-2: Storage]
  [Agent 2-3: Semantic]  [Agent 2-4: Manager]
                              ↓
PHASE 3 ──────────────────────────────────────────────────────────
  [Agent 3-1: NVIDIA]    [Agent 3-2: AMD]
  [Agent 3-3: Recommend] [Agent 3-4: Utilities]
                              ↓
PHASE 4 ──────────────────────────────────────────────────────────
  [Agent 4-1: Registry]  [Agent 4-2: File Tools]
  [Agent 4-3: Search]    [Agent 4-4: Agentic]
                              ↓
PHASE 5 ──────────────────────────────────────────────────────────
  [Agent 5-1: CLI]       [Agent 5-2: Ask/Chat]
  [Agent 5-3: Status]    [Agent 5-4: Setup]
                              ↓
PHASE 6 ──────────────────────────────────────────────────────────
  [Agent 6-1: Session]   [Agent 6-2: Banner]
  [Agent 6-3: Audit]     [Agent 6-4: Classification]
                              ↓
PHASE 7 ──────────────────────────────────────────────────────────
  [Agent 7-1: Client]    [Agent 7-2: Stream]
  [Agent 7-3: Server]    [Agent 7-4: Security]
                              ↓
PHASE 8 ──────────────────────────────────────────────────────────
  [Agent 8-1: Tests]     [Agent 8-2: Integration]
  [Agent 8-3: TUI]       [Agent 8-4: Polish]
```

---

## SUCCESS CRITERIA

After all 8 phases:

- [ ] `rigrun` launches TUI (default)
- [ ] `rigrun ask "question"` uses intelligent routing
- [ ] `rigrun status` shows GPU, model, stats
- [ ] `rigrun config show` displays config
- [ ] `rigrun doctor` runs health checks
- [ ] Queries route to Cache/Local/Cloud based on complexity
- [ ] Cost tracking shows savings vs Opus
- [ ] DoD banner shows on first run
- [ ] Session times out after 15 minutes
- [ ] All queries logged to audit.log
- [ ] Tools work (read, write, edit, glob, grep, bash)
- [ ] Agentic loop executes multi-step tasks

---

**Total: 32 Agents across 8 Phases**
**Estimated Time: 8 sequential phases, ~10-15 minutes per phase**
