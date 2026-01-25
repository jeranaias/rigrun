# rigrun Architecture Documentation

> Comprehensive technical documentation for the rigrun project
> Last Updated: 2026-01-24

---

## Table of Contents

1. [Repository Overview](#1-repository-overview)
2. [Directory Structure](#2-directory-structure)
3. [Rust Implementation](#3-rust-implementation-src)
4. [Go TUI Architecture](#4-go-tui-architecture-go-tui)
5. [Security Architecture](#5-security-architecture)
6. [Key Features by Location](#6-key-features-by-location)
7. [Architecture Diagrams](#7-architecture-diagrams)
8. [Configuration Reference](#8-configuration-reference)
9. [CLI Commands Reference](#9-cli-commands-reference)
10. [Build and Installation](#10-build-and-installation)

---

## 1. Repository Overview

### What is rigrun?

**rigrun** is a classification-aware LLM router designed for secure environments, with the tagline: **"Your GPU first, cloud when needed."**

It is the only LLM router built for DoD/IL5 classification requirements, intelligently routing queries based on data classification levels to ensure classified content NEVER touches cloud APIs while maintaining full OpenAI compatibility for unclassified workloads.

### Two Implementations

rigrun provides two implementations:

| Implementation | Language | Purpose | Status |
|----------------|----------|---------|--------|
| **Rust Server** (`src/`) | Rust | Production-ready OpenAI-compatible API server with CLI | Production |
| **Go TUI** (`go-tui/`) | Go | Modern interactive terminal interface using Charm ecosystem | Production |

### Core Features

- **Classification-Aware Routing**: Automatic detection and enforcement of data classification (UNCLASSIFIED, CUI, FOUO, SECRET, TOP SECRET)
- **Three-Tier Routing**: Cache (instant, free) -> Local GPU (fast, free) -> Cloud (pay only when needed)
- **NIST 800-53 Compliance**: Full implementation of security controls for IL5 authorization
- **DoD STIG Compliance**: Session timeouts, audit logging, consent banners, secret redaction
- **Semantic Caching**: Context-aware deduplication using embeddings (40-60% hit rate)
- **GPU Auto-Detection**: NVIDIA (CUDA), AMD (ROCm), Apple Silicon (Metal), Intel Arc
- **OpenAI-Compatible API**: Drop-in replacement for existing codebases
- **Agentic Tool System**: Built-in tools for file operations, code editing, and shell execution
- **Air-Gap/Paranoid Mode**: Zero external network connections when enabled

### Target Audience

- **Defense Contractors**: Handle CUI and classified workloads with AI assistance
- **Government Agencies**: Meet IL5 requirements while enabling modern AI workflows
- **Cleared Facilities**: Use AI without risking security violations
- **Enterprise Teams**: Self-hosted AI for compliance requirements
- **Security-Conscious Developers**: Local-first AI that respects data privacy

---

## 2. Directory Structure

### Complete Directory Tree

```
rigrun/
|
|-- .github/                    # GitHub CI/CD configuration
|   |-- ISSUE_TEMPLATE/         # Issue templates
|   |-- workflows/              # GitHub Actions workflows
|   |   |-- ci.yml              # Continuous integration (Rust + Go tests)
|   |   |-- release.yml         # Release automation
|   |-- FUNDING.yml             # Sponsorship configuration
|   |-- PULL_REQUEST_TEMPLATE.md
|
|-- agent-prompts/              # AI agent prompt templates
|
|-- archive/                    # Archived files and legacy code
|
|-- benchmarks/                 # Performance benchmarks and test queries
|
|-- docs/                       # Additional documentation
|   |-- api-reference.md        # OpenAI-compatible API documentation
|   |-- CLAUDE_CODE_USAGE.md    # Claude Code integration guide
|   |-- COMPETITIVE_INTEL_SYSTEM_DESIGN.md
|   |-- configuration.md        # Configuration options
|   |-- contributing.md         # Contribution guidelines
|   |-- error-messages.md       # Error handling reference
|   |-- GETTING_STARTED.md      # Quickstart guide
|   |-- GPU_COMPATIBILITY.md    # GPU setup documentation
|   |-- index.html              # Documentation site
|   |-- installation.md         # Installation instructions
|   |-- INTEL_*.md              # Competitive intelligence documentation
|   |-- RACE_DETECTION.md       # Concurrency testing guide
|   |-- security.md             # Security and compliance guide
|   |-- troubleshooting.md      # Troubleshooting guide
|   |-- UX_2.0_DESIGN_SPEC.md   # UI/UX design specification
|
|-- examples/                   # Example code
|   |-- error_demo.rs           # Rust error handling example
|   |-- intel/                  # Competitive intelligence examples
|       |-- example_module_company_overview.go
|       |-- example_orchestrator.go
|
|-- go-tui/                     # Go TUI implementation (see Section 4)
|
|-- installer/                  # Installation packages
|   |-- build.bat               # Windows installer build script
|
|-- local-rag-rust-server/      # Local RAG server (separate project)
|
|-- marketing/                  # Marketing materials and pitch decks
|
|-- scripts/                    # Utility scripts
|
|-- src/                        # Rust implementation (see Section 3)
|
|-- target/                     # Rust build artifacts (gitignored)
|
|-- tests/                      # Integration tests
|
|-- Root Files:
|   |-- .dockerignore           # Docker ignore patterns
|   |-- .gitignore              # Git ignore patterns
|   |-- ask / ask.bat           # Quick-ask shell scripts
|   |-- Cargo.toml              # Rust dependencies
|   |-- Cargo.lock              # Locked Rust dependency versions
|   |-- CHANGELOG.md            # Version history
|   |-- CLI_REFERENCE.md        # CLI command documentation
|   |-- CONTRIBUTING.md         # Contribution guidelines
|   |-- docker-compose.yml      # Docker Compose configuration
|   |-- Dockerfile              # Container definition
|   |-- install.ps1             # Windows PowerShell installer
|   |-- install.sh              # Unix/macOS installer
|   |-- LICENSE                 # MIT License
|   |-- LICENSE-COMMERCIAL.md   # Commercial license terms
|   |-- NOTICE                  # Attribution notices
|   |-- README.md               # Main project documentation
```

### Root Level Files

| File | Purpose |
|------|---------|
| `README.md` | Main project documentation, quickstart guide, and feature overview |
| `ARCHITECTURE.md` | This file - comprehensive technical architecture documentation |
| `Cargo.toml` / `Cargo.lock` | Rust dependencies and build configuration |
| `LICENSE` / `LICENSE-COMMERCIAL.md` | MIT and commercial license terms |
| `CHANGELOG.md` | Version history and release notes |
| `CLI_REFERENCE.md` | Full CLI command documentation |
| `CONTRIBUTING.md` | Contribution guidelines for developers |
| `install.sh` / `install.ps1` | Cross-platform installation scripts |
| `Dockerfile` / `docker-compose.yml` | Container deployment configuration |
| `ask` / `ask.bat` | Quick-ask convenience scripts |

---

## 3. Rust Implementation (`src/`)

The Rust implementation provides the production-ready OpenAI-compatible API server with full security features.

### Module Overview

```
src/
|-- lib.rs                  # Library root - re-exports all public types
|-- main.rs                 # Binary entry point with CLI parsing
|
|-- audit.rs                # Audit logging (NIST AU-*)
|-- background.rs           # Background task management
|-- classification_ui.rs    # CUI marking per DoDI 5200.48
|-- cli_session.rs          # CLI session with timeout
|-- colors.rs               # Terminal color utilities
|-- consent_banner.rs       # DoD IL5 consent banner
|-- conversation_store.rs   # Conversation persistence
|-- error.rs                # Error formatting utilities
|-- status_indicator.rs     # Live status display
|-- types.rs                # Shared type definitions (Tier, Message, etc.)
|-- utils.rs                # Utility functions (mask_sensitive, etc.)
|
|-- cache/                  # Response caching subsystem
|   |-- mod.rs              # Cache module root
|   |-- embeddings.rs       # Embedding generation for semantic cache
|   |-- semantic.rs         # Semantic similarity caching
|   |-- vector_index.rs     # Vector similarity search
|
|-- cli/                    # CLI completion and slash commands
|   |-- mod.rs              # CLI module root
|   |-- completer.rs        # Tab completion for commands
|   |-- input.rs            # Interactive input handling
|
|-- cloud/                  # Cloud provider integration
|   |-- mod.rs              # OpenRouter client for cloud fallback
|
|-- context/                # Smart context management
|   |-- mod.rs              # @ mention support (@file, @git, @codebase, @error)
|
|-- detect/                 # GPU detection and model recommendations
|   |-- mod.rs              # NVIDIA, AMD, Apple Silicon, Intel Arc detection
|
|-- download/               # Model download management
|   |-- mod.rs              # Download module root
|   |-- manager.rs          # Download progress and state
|   |-- state.rs            # Download state tracking
|   |-- types.rs            # Download-related types
|
|-- errors/                 # IL5-compliant error handling
|   |-- mod.rs              # NIST 800-53 SI-11 compliant error responses
|
|-- firstrun/               # First-run setup wizard
|   |-- mod.rs              # Module root
|   |-- wizard.rs           # Interactive setup wizard
|
|-- health/                 # System health checks
|   |-- mod.rs              # Startup health check, diagnostics
|
|-- local/                  # Local LLM integration
|   |-- mod.rs              # Ollama client with streaming
|
|-- router/                 # Query routing engine
|   |-- mod.rs              # Classification-based routing logic
|
|-- security/               # DoD STIG security controls
|   |-- mod.rs              # Security module root
|   |-- locks.rs            # File locking for atomic operations
|   |-- session_manager.rs  # Session timeout (AC-11, AC-12)
|
|-- server/                 # HTTP server
|   |-- mod.rs              # OpenAI-compatible API endpoints
|
|-- setup/                  # Configuration wizard
|   |-- mod.rs              # Setup wizard and config generation
|
|-- stats/                  # Cost tracking and analytics
|   |-- mod.rs              # Real-time savings calculations
```

### Key Rust Types

| Module | Key Types | Purpose |
|--------|-----------|---------|
| `types` | `Tier`, `Message`, `StreamingConfig` | Core data structures |
| `router` | `QueryComplexity`, `QueryType`, `RoutingDecision` | Routing logic |
| `cache` | `QueryCache`, `CachedResponse`, `CacheStats` | Response caching |
| `local` | `OllamaClient`, `OllamaResponse` | Local LLM communication |
| `cloud` | `OpenRouterClient` | Cloud API integration |
| `detect` | `GpuInfo`, `GpuType`, `ModelRecommendation` | Hardware detection |
| `server` | `Server` | HTTP API server |
| `stats` | `StatsTracker`, `SessionStats`, `SavingsSummary` | Cost analytics |
| `audit` | `AuditLogger`, `AuditEntry` | Security audit logging |
| `security` | `Session`, `SessionConfig`, `SessionManager` | STIG compliance |
| `classification_ui` | `ClassificationLevel`, `CuiDesignation` | CUI markings |

### Tier System (Rust)

```rust
pub enum Tier {
    Cache,   // Semantic cache - instant, free
    Local,   // Ollama - fast, free
    Cloud,   // OpenRouter auto-selection
    Haiku,   // Claude 3 Haiku - fast, cheap
    Sonnet,  // Claude 3 Sonnet - balanced
    Opus,    // Claude 3 Opus - powerful
    Gpt4o,   // GPT-4o
}
```

---

## 4. Go TUI Architecture (`go-tui/`)

The Go implementation provides a modern terminal user interface using the Charm ecosystem (Bubble Tea, Lipgloss, Glamour).

### Directory Structure

```
go-tui/
|-- go.mod                      # Go module definition
|-- go.sum                      # Dependency checksums
|-- main.go                     # Entry point (if present)
|
|-- cmd/
|   |-- installer/              # Standalone installer application
|       |-- main.go             # Installer entry point
|       |-- installer.go        # Installation logic
|       |-- welcome.go          # Welcome screen
|
|-- internal/
    |-- ac4_e2e_test.go         # AC-4 information flow E2E tests
    |-- concurrency_test.go     # Concurrency safety tests
    |-- integration_test.go     # Full integration test suite
    |
    |-- benchmark/              # Performance benchmarking
    |   |-- benchmark.go        # Benchmark runner
    |   |-- results.go          # Result formatting
    |   |-- tests.go            # Benchmark test definitions
    |
    |-- cache/                  # Query caching system
    |   |-- exact.go            # Hash-based exact matching
    |   |-- manager.go          # Cache manager orchestration
    |   |-- manager_optimized.go # Performance-optimized manager
    |   |-- semantic.go         # Embedding-based semantic matching
    |   |-- storage.go          # Persistent cache storage
    |
    |-- cli/                    # CLI command handlers
    |   |-- agreements_cmd.go   # User agreements
    |   |-- args.go             # Argument parsing
    |   |-- ask.go              # Ask command handler
    |   |-- audit_cmd.go        # Audit commands
    |   |-- auth_cmd.go         # Authentication commands
    |   |-- backup_cmd.go       # Backup/restore
    |   |-- boundary_cmd.go     # Boundary protection
    |   |-- cache_cmd.go        # Cache management
    |   |-- chat.go             # Interactive chat
    |   |-- classify_cmd.go     # Classification commands
    |   |-- cli.go              # CLI router
    |   |-- config.go           # Configuration commands
    |   |-- configmgmt_cmd.go   # Config management
    |   |-- confirm.go          # Confirmation dialogs
    |   |-- conmon_cmd.go       # Connection monitoring
    |   |-- consent.go          # Consent banner handling
    |   |-- crypto_cmd.go       # Crypto commands
    |   |-- doctor.go           # System diagnostics
    |   |-- encrypt_cmd.go      # Encryption commands
    |   |-- errors.go           # Error handling
    |   |-- helpers.go          # CLI utilities
    |   |-- incident_cmd.go     # Incident management
    |   |-- intel_cmd.go        # Intel system commands
    |   |-- json_output.go      # JSON output formatting
    |   |-- lockout_cmd.go      # Account lockout
    |   |-- maintenance_cmd.go  # Maintenance mode
    |   |-- rbac_cmd.go         # RBAC commands
    |   |-- sanitize_cmd.go     # Data sanitization
    |   |-- sectest_cmd.go      # Security testing
    |   |-- session_cmd.go      # Session management
    |   |-- setup.go            # Setup wizard
    |   |-- status.go           # Status display
    |   |-- styles.go           # CLI styling
    |   |-- terminal.go         # Terminal utilities
    |   |-- test.go             # Test commands
    |   |-- training_cmd.go     # Security training
    |   |-- transport_cmd.go    # Transport security
    |   |-- verify_cmd.go       # Verification commands
    |   |-- vulnscan_cmd.go     # Vulnerability scanning
    |
    |-- cloud/                  # Cloud provider integration
    |   |-- client.go           # OpenRouter client
    |   |-- stream.go           # Streaming response handling
    |
    |-- commands/               # TUI slash command system
    |   |-- benchmark_handler.go # Benchmark command
    |   |-- completion.go       # Tab completion
    |   |-- handlers.go         # Command handlers
    |   |-- index_cmd.go        # Index commands
    |   |-- parser.go           # Command parsing
    |   |-- registry.go         # Command registration
    |
    |-- config/                 # Configuration management
    |   |-- config.go           # Config loading, saving, validation
    |
    |-- context/                # Smart context management
    |   |-- cache.go            # Context caching
    |   |-- example_integration.go # Integration example
    |   |-- expander.go         # Context expansion
    |   |-- fetchers.go         # File/git/codebase fetchers
    |   |-- parser.go           # @ mention parsing
    |   |-- summarizer.go       # Context summarization
    |   |-- truncation.go       # Context truncation for token limits
    |
    |-- detect/                 # GPU detection
    |   |-- amd.go              # AMD GPU detection
    |   |-- detect.go           # Main detection logic
    |   |-- recommend.go        # Model recommendations
    |
    |-- diff/                   # Inline diff system
    |   |-- diff.go             # Diff generation and display
    |   |-- README.md           # Diff system documentation
    |
    |-- export/                 # Conversation export
    |   |-- converter.go        # Format conversion
    |   |-- export.go           # Export orchestration
    |   |-- html.go             # HTML export
    |   |-- json.go             # JSON export
    |   |-- markdown.go         # Markdown export
    |   |-- README.md           # Export documentation
    |
    |-- index/                  # Codebase indexing
    |   |-- index.go            # Index management
    |   |-- parser.go           # Code parsing
    |   |-- schema.go           # Index schema
    |   |-- search.go           # Index search
    |   |-- watcher.go          # File system watching
    |   |-- README.md           # Index documentation
    |
    |-- model/                  # Data models
    |   |-- conversation.go     # Conversation structure
    |   |-- message.go          # Message types
    |   |-- models.go           # Model definitions
    |
    |-- offline/                # Offline operation
    |   |-- offline.go          # Offline mode management
    |
    |-- ollama/                 # Ollama integration
    |   |-- client.go           # Ollama API client
    |   |-- start_unix.go       # Unix-specific startup
    |   |-- start_windows.go    # Windows-specific startup
    |   |-- stream.go           # Streaming responses
    |   |-- stream_optimized.go # Performance-optimized streaming
    |   |-- types.go            # Ollama-specific types
    |
    |-- plan/                   # Task planning system
    |   |-- executor.go         # Plan execution
    |   |-- generator.go        # Plan generation
    |   |-- plan.go             # Plan data structures
    |   |-- README.md           # Planning documentation
    |
    |-- router/                 # Query routing
    |   |-- classify.go         # Query classification
    |   |-- cost.go             # Cost calculation
    |   |-- router.go           # Routing logic
    |   |-- types.go            # Router types
    |
    |-- security/               # IL5 Security Controls (see Section 5)
    |
    |-- server/                 # HTTP server
    |   |-- middleware.go       # Security middleware
    |   |-- server.go           # OpenAI-compatible API
    |
    |-- session/                # Session management
    |   |-- manager.go          # DoD STIG session timeout
    |
    |-- storage/                # Persistence layer
    |   |-- conversations.go    # Conversation storage
    |
    |-- tasks/                  # Background task system
    |   |-- queue.go            # Task queue
    |   |-- runner.go           # Task runner
    |   |-- task.go             # Task definitions
    |   |-- ARCHITECTURE.md     # Task system documentation
    |   |-- README.md           # Task system guide
    |
    |-- telemetry/              # Cost tracking and telemetry
    |   |-- cost.go             # Cost calculation
    |   |-- cost_storage.go     # Cost persistence
    |   |-- example_integration.go # Integration example
    |   |-- README.md           # Telemetry documentation
    |
    |-- tools/                  # Agentic tool system
    |   |-- agentic.go          # Agentic orchestration
    |   |-- bash.go             # Bash command execution
    |   |-- definitions.go      # Tool definitions
    |   |-- diff_integration.go # Diff tool integration
    |   |-- diff_wrapper.go     # Diff wrapper
    |   |-- duckduckgo.go       # DuckDuckGo search
    |   |-- edit.go             # File editing tool
    |   |-- executor.go         # Tool execution engine
    |   |-- file.go             # File operations
    |   |-- ollama.go           # Ollama tool calls
    |   |-- read.go             # File reading tool
    |   |-- search.go           # Search tools (Glob, Grep)
    |   |-- security.go         # Tool security validation
    |   |-- web.go              # Web fetching tool
    |   |-- write.go            # File writing tool
    |   |-- SECURITY_FIXES.md   # Security documentation
    |
    |-- ui/                     # TUI components
    |   |-- chat/               # Main chat interface
    |   |   |-- cancel.go       # Request cancellation
    |   |   |-- commands.go     # Slash command handling
    |   |   |-- completion.go   # Tab completion
    |   |   |-- export.go       # Export functionality
    |   |   |-- input.go        # Input handling
    |   |   |-- keys.go         # Keyboard shortcuts
    |   |   |-- messages.go     # Message types
    |   |   |-- model.go        # Bubble Tea model (73KB main file)
    |   |   |-- security_commands.go # Security slash commands
    |   |   |-- streaming.go    # Streaming response display
    |   |   |-- update.go       # State updates
    |   |   |-- utils.go        # Utility functions
    |   |   |-- view.go         # View rendering
    |   |   |-- view_completion.go # Completion view
    |   |   |-- viewport_optimizer.go # Viewport performance
    |   |   |-- vim.go          # Vim mode support
    |   |   |-- VIM_MODE.md     # Vim mode documentation
    |   |
    |   |-- components/         # Reusable UI components
    |   |   |-- benchmark_view.go # Benchmark display
    |   |   |-- classification_banner.go # Classification banners
    |   |   |-- codeblock.go    # Code block rendering
    |   |   |-- completion.go   # Completion dropdown
    |   |   |-- consent_banner.go # DoD consent banner
    |   |   |-- context_bar.go  # Context display bar
    |   |   |-- cost_dashboard.go # Cost tracking display
    |   |   |-- diff_viewer.go  # Diff visualization
    |   |   |-- error.go        # Error display
    |   |   |-- error_patterns.go # Error pattern recognition
    |   |   |-- fuzzy.go        # Fuzzy matching
    |   |   |-- header.go       # Header component
    |   |   |-- helpers.go      # Component utilities
    |   |   |-- input.go        # Input component
    |   |   |-- message.go      # Message display
    |   |   |-- palette.go      # Command palette
    |   |   |-- permission.go   # Permission dialogs
    |   |   |-- plan_view.go    # Plan display
    |   |   |-- progress.go     # Progress indicators
    |   |   |-- session_timeout_overlay.go # Timeout warning
    |   |   |-- spinner.go      # Loading spinner
    |   |   |-- statusbar.go    # Status bar
    |   |   |-- task_list.go    # Task list display
    |   |   |-- toolresult.go   # Tool result display
    |   |   |-- tutorial.go     # Interactive tutorial
    |   |   |-- viewport.go     # Scrollable viewport
    |   |   |-- welcome.go      # Welcome screen
    |   |
    |   |-- styles/             # Theme and styling
    |       |-- theme.go        # Color themes
    |
    |-- util/                   # General utilities
        |-- atomic.go           # Atomic operations
        |-- convert.go          # Type conversions
        |-- string.go           # String utilities
```

### Built-in Agentic Tools

| Tool | Risk Level | Permission | Description |
|------|------------|------------|-------------|
| `Read` | Low | Auto | Read file contents with line numbers |
| `Write` | Medium | Ask | Write content to a file |
| `Edit` | Medium | Ask | Edit file with string replacement |
| `Glob` | Low | Auto | Find files by pattern |
| `Grep` | Low | Auto | Search file contents with regex |
| `Bash` | Critical | Ask | Execute shell commands |
| `WebSearch` | Low | Auto | DuckDuckGo search (no API key) |
| `WebFetch` | Low | Auto | Fetch and parse web pages |

---

## 5. Security Architecture

rigrun implements comprehensive security controls for IL5 (Impact Level 5) authorization per NIST 800-53 Rev 5.

### Security Package Structure (`go-tui/internal/security/`)

```
security/
|-- security.go             # Package documentation and re-exports
|
|-- Root Security Modules:
|   |-- agreements.go       # User agreements and acknowledgments
|   |-- audit.go            # Audit logging (AU-2, AU-3)
|   |-- audit_hmac_key.go   # HMAC key management for audit integrity
|   |-- audit_hmac_key_acl_unix.go    # Unix ACL support
|   |-- audit_hmac_key_acl_windows.go # Windows ACL support
|   |-- auditprotect.go     # Audit protection (AU-9)
|   |-- auditreview.go      # Audit review (AU-6)
|   |-- auth.go             # Authentication (IA-2, IA-5)
|   |-- backup.go           # Backup and recovery (CP-9)
|   |-- banner.go           # DoD consent banner (AC-8)
|   |-- boundary.go         # Boundary protection (SC-7)
|   |-- classification.go   # Classification enforcement (AC-4)
|   |-- configmgmt.go       # Configuration management (CM-5, CM-6)
|   |-- conmon.go           # Continuous monitoring (CA-7)
|   |-- crypto.go           # Cryptographic operations (SC-13)
|   |-- encrypt.go          # Encryption at rest (SC-28)
|   |-- incident.go         # Incident response (IR-4, IR-5)
|   |-- integrity.go        # Integrity verification (SI-7)
|   |-- keystore.go         # Key storage management
|   |-- keystore_unix.go    # Unix key storage
|   |-- keystore_windows.go # Windows key storage
|   |-- lockout.go          # Account lockout (AC-7)
|   |-- maintenance.go      # Maintenance mode (MA-2)
|   |-- pki.go              # PKI certificate management (SC-17)
|   |-- rbac.go             # Role-based access control (AC-5, AC-6)
|   |-- remoteaccess.go     # Remote access control (AC-17)
|   |-- rulesofbehavior.go  # Rules of behavior (PL-4)
|   |-- sanitize.go         # Data sanitization (MP-6)
|   |-- sectest.go          # Security testing (CA-8)
|   |-- session.go          # Session management (AC-11, AC-12)
|   |-- spillage.go         # Spillage handling (IR-9)
|   |-- training.go         # Security awareness (AT-2)
|   |-- transport.go        # Transport security (SC-8)
|   |-- vulnscan.go         # Vulnerability scanning (RA-5)
|
|-- Subpackages:
|   |-- access/             # Access control subpackage
|   |   |-- doc.go          # Package documentation
|   |   |-- lockout.go      # Account lockout (AC-7)
|   |   |-- rbac.go         # RBAC enforcement (AC-5, AC-6)
|   |
|   |-- audit/              # Audit subpackage
|   |   |-- doc.go          # Package documentation
|   |   |-- hmac.go         # HMAC integrity protection
|   |   |-- logger.go       # Thread-safe audit logging
|   |   |-- protect.go      # Audit protection (AU-9)
|   |   |-- review.go       # Audit review (AU-6)
|   |
|   |-- auth/               # Authentication subpackage
|   |   |-- doc.go          # Package documentation
|   |   |-- manager.go      # Authentication manager
|   |   |-- session.go      # Session tokens
|   |
|   |-- classification/     # Classification subpackage
|   |   |-- doc.go          # Package documentation
|   |   |-- enforcer.go     # Classification enforcement (AC-4)
|   |
|   |-- crypto/             # Cryptographic subpackage
|   |   |-- doc.go          # Package documentation
|   |   |-- encrypt.go      # Encryption operations
|   |   |-- fips.go         # FIPS 140-2 compliance
|   |   |-- pki.go          # PKI operations
|   |
|   |-- network/            # Network security subpackage
|       |-- doc.go          # Package documentation
|       |-- boundary.go     # Network boundary (SC-7)
|       |-- transport.go    # Transport security (SC-8)
```

### NIST 800-53 Control Mapping

| Control Family | Control ID | Description | Implementation |
|----------------|------------|-------------|----------------|
| **Access Control (AC)** | | | |
| | AC-4 | Information Flow Enforcement | `classification.go`, `classification/enforcer.go` |
| | AC-5 | Separation of Duties | `rbac.go`, `access/rbac.go` |
| | AC-6 | Least Privilege | `rbac.go`, `access/rbac.go` |
| | AC-7 | Unsuccessful Logon Attempts | `lockout.go`, `access/lockout.go` |
| | AC-8 | System Use Notification | `banner.go` |
| | AC-11 | Session Lock | `session.go`, `auth/session.go` |
| | AC-12 | Session Termination | `session.go`, `auth/session.go` |
| | AC-17 | Remote Access | `remoteaccess.go` |
| **Audit (AU)** | | | |
| | AU-2 | Audit Events | `audit.go`, `audit/logger.go` |
| | AU-3 | Content of Audit Records | `audit.go`, `audit/logger.go` |
| | AU-5 | Response to Audit Failures | `audit/logger.go` |
| | AU-6 | Audit Review/Analysis | `auditreview.go`, `audit/review.go` |
| | AU-9 | Protection of Audit Info | `auditprotect.go`, `audit/protect.go` |
| **Identification (IA)** | | | |
| | IA-2 | Identification/Authentication | `auth.go`, `auth/manager.go` |
| | IA-2(1) | Multi-factor Authentication | `auth.go` |
| | IA-5 | Authenticator Management | `auth.go` |
| | IA-7 | Cryptographic Module Auth | `crypto.go`, `crypto/fips.go` |
| **System/Comms (SC)** | | | |
| | SC-7 | Boundary Protection | `boundary.go`, `network/boundary.go` |
| | SC-8 | Transmission Confidentiality | `transport.go`, `network/transport.go` |
| | SC-13 | Cryptographic Protection | `crypto.go`, `crypto/encrypt.go` |
| | SC-17 | PKI Certificates | `pki.go`, `crypto/pki.go` |
| | SC-28 | Protection at Rest | `encrypt.go` |
| **Incident Response (IR)** | | | |
| | IR-4 | Incident Handling | `incident.go` |
| | IR-5 | Incident Monitoring | `incident.go` |
| | IR-9 | Information Spillage | `spillage.go` |
| **Config Mgmt (CM)** | | | |
| | CM-5 | Access Restrictions for Change | `configmgmt.go` |
| | CM-6 | Configuration Settings | `configmgmt.go` |
| **Security Assessment (CA)** | | | |
| | CA-7 | Continuous Monitoring | `conmon.go` |
| | CA-8 | Penetration Testing | `sectest.go` |
| **Contingency (CP)** | | | |
| | CP-9 | System Backup | `backup.go` |
| **Risk Assessment (RA)** | | | |
| | RA-5 | Vulnerability Scanning | `vulnscan.go` |
| **Awareness Training (AT)** | | | |
| | AT-2 | Security Awareness | `training.go` |
| **Maintenance (MA)** | | | |
| | MA-2 | Controlled Maintenance | `maintenance.go` |
| **Planning (PL)** | | | |
| | PL-4 | Rules of Behavior | `rulesofbehavior.go` |
| **Media Protection (MP)** | | | |
| | MP-6 | Media Sanitization | `sanitize.go` |
| **System Integrity (SI)** | | | |
| | SI-7 | Software Integrity | `integrity.go` |
| | SI-11 | Error Handling | `errors/mod.rs` (Rust) |

### Classification Levels

| Level | Description | Routing |
|-------|-------------|---------|
| `UNCLASSIFIED` | Public information | Cloud or Local (cost-optimized) |
| `CUI` | Controlled Unclassified Information | Local Only (NIST 800-171) |
| `FOUO` | For Official Use Only | Local Only |
| `SECRET` | Classified information | Local Only (air-gapped) |
| `TOP_SECRET` | Highly classified | Local Only (maximum security) |

---

## 6. Key Features by Location

### Classification-Based Routing

| Feature | Rust Location | Go Location |
|---------|---------------|-------------|
| Classification detection | `src/router/mod.rs` | `internal/router/classify.go` |
| Routing enforcement | `src/router/mod.rs` | `internal/router/router.go` |
| Classification UI | `src/classification_ui.rs` | `internal/security/classification.go` |
| Banner display | `src/consent_banner.rs` | `internal/security/banner.go` |

### Caching System

| Feature | Rust Location | Go Location |
|---------|---------------|-------------|
| Exact match cache | `src/cache/mod.rs` | `internal/cache/exact.go` |
| Semantic cache | `src/cache/semantic.rs` | `internal/cache/semantic.go` |
| Cache manager | `src/cache/mod.rs` | `internal/cache/manager.go` |
| Embeddings | `src/cache/embeddings.rs` | `internal/cache/semantic.go` |

### LLM Integration

| Feature | Rust Location | Go Location |
|---------|---------------|-------------|
| Ollama client | `src/local/mod.rs` | `internal/ollama/client.go` |
| OpenRouter client | `src/cloud/mod.rs` | `internal/cloud/client.go` |
| Streaming responses | `src/local/mod.rs` | `internal/ollama/stream.go` |
| Model recommendations | `src/detect/mod.rs` | `internal/detect/recommend.go` |

### Security Features

| Feature | Rust Location | Go Location |
|---------|---------------|-------------|
| Session timeout | `src/security/session_manager.rs` | `internal/security/session.go` |
| Audit logging | `src/audit.rs` | `internal/security/audit.go` |
| Secret redaction | `src/utils.rs` | `internal/security/audit.go` |
| HMAC integrity | `src/audit.rs` | `internal/security/audit_hmac_key.go` |
| RBAC | - | `internal/security/rbac.go` |
| Account lockout | - | `internal/security/lockout.go` |

### User Interface

| Feature | Rust Location | Go Location |
|---------|---------------|-------------|
| CLI parsing | `src/main.rs` | `internal/cli/cli.go` |
| Status display | `src/status_indicator.rs` | `internal/cli/status.go` |
| Interactive chat | `src/cli_session.rs` | `internal/ui/chat/model.go` |
| Slash commands | `src/cli/mod.rs` | `internal/commands/registry.go` |
| Tab completion | `src/cli/completer.rs` | `internal/commands/completion.go` |

### Context System

| Feature | Rust Location | Go Location |
|---------|---------------|-------------|
| @ mention parsing | `src/context/mod.rs` | `internal/context/parser.go` |
| File fetching | `src/context/mod.rs` | `internal/context/fetchers.go` |
| Context expansion | `src/context/mod.rs` | `internal/context/expander.go` |
| Truncation | - | `internal/context/truncation.go` |

### Cost Tracking

| Feature | Rust Location | Go Location |
|---------|---------------|-------------|
| Cost calculation | `src/router/mod.rs` | `internal/router/cost.go` |
| Stats tracking | `src/stats/mod.rs` | `internal/telemetry/cost.go` |
| Cost display | `src/status_indicator.rs` | `internal/ui/components/cost_dashboard.go` |

---

## 7. Architecture Diagrams

### System Architecture

```
+------------------------------------------------------------------+
|                         rigrun System                             |
+------------------------------------------------------------------+
|                                                                   |
|  User Interface Layer                                             |
|  +------------------+     +---------------------------+           |
|  |   Go TUI         |     |    Rust HTTP Server       |           |
|  | (Bubble Tea)     |     | (OpenAI-compatible API)   |           |
|  +--------+---------+     +-------------+-------------+           |
|           |                             |                         |
|           +-------------+---------------+                         |
|                         |                                         |
|  Security Layer         v                                         |
|  +-----------------------------------------------------------+   |
|  |                 Security Controls                          |   |
|  |  - Classification Enforcement (AC-4)                       |   |
|  |  - Authentication (IA-2)                                   |   |
|  |  - Session Management (AC-11/12)                           |   |
|  |  - Audit Logging (AU-*)                                    |   |
|  +----------------------------+------------------------------+   |
|                               |                                   |
|  Routing Layer                v                                   |
|  +-----------------------------------------------------------+   |
|  |                    Query Router                            |   |
|  |  - Classify content (UNCLASSIFIED -> TOP_SECRET)           |   |
|  |  - Classify complexity (Trivial -> Expert)                 |   |
|  |  - Select tier based on classification + complexity        |   |
|  +----------+----------------+----------------+--------------+   |
|             |                |                |                   |
|  Backend    v                v                v                   |
|  +----------+--+    +--------+------+    +---+------------+      |
|  |    Cache    |    |    Local      |    |    Cloud       |      |
|  |    Tier     |    |    Tier       |    |    Tier        |      |
|  +------+------+    +-------+-------+    +-------+--------+      |
|         |                   |                    |                |
|         v                   v                    v                |
|  +------+------+    +-------+-------+    +-------+--------+      |
|  | Exact Match |    |    Ollama     |    |  OpenRouter    |      |
|  | + Semantic  |    |    Server     |    |     API        |      |
|  +-------------+    +---------------+    +----------------+      |
|                     | qwen2.5-coder |    | Claude, GPT-4o |      |
|                     | deepseek, etc |    | Haiku, Sonnet  |      |
|                     +---------------+    +----------------+      |
|                                                                   |
+------------------------------------------------------------------+
```

### Data Flow: Classification-Aware Routing

```
User Query
    |
    v
+-------------------+
| Parse & Validate  |
+-------------------+
    |
    v
+-------------------+
| Classification    |
| Detection         |---> Contains CUI/SECRET markers?
+-------------------+          |
    |                    +-----+-----+
    |                    |           |
    | UNCLASSIFIED       | YES       |
    |                    v           |
    |             +-------------+    |
    |             | LOCAL ONLY  |<---+
    |             | (Enforced)  |
    |             +------+------+
    |                    |
    v                    v
+-------------------+    +-------------------+
| Complexity        |    | Ollama Local LLM  |
| Classification    |    +-------------------+
+-------------------+             |
    |                             v
    +---> Trivial ------------> Cache Lookup
    |                              |
    |                        Hit?--+-> Return Cached
    |                              |
    |                        Miss--+-> Local Tier
    |
    +---> Simple/Moderate ------> Local Tier (Ollama)
    |                                  |
    |                            Success?--+-> Cache & Return
    |                                      |
    |                            Fail?-----+-> Escalate (if allowed)
    |
    +---> Complex/Expert -------> Cloud Tier (if UNCLASSIFIED)
                                       |
                                  Response
                                       |
                                       v
                               +---------------+
                               | Cache Result  |
                               | Track Cost    |
                               | Audit Log     |
                               +---------------+
                                       |
                                       v
                               Return to User
```

### TUI Component Hierarchy

```
+----------------------------------------------------------+
|                     main.go                               |
|                 (Bubble Tea Program)                      |
+----------------------------------------------------------+
                          |
                          v
+----------------------------------------------------------+
|                    App Model                              |
|  - State: conversation, input, settings                  |
|  - Update: handle messages, key events                   |
|  - View: render components                               |
+----------------------------------------------------------+
          |           |           |           |
          v           v           v           v
    +---------+ +---------+ +---------+ +-----------+
    |  Header | | Message | |  Input  | | StatusBar |
    |Component| |  List   | |Component| | Component |
    +---------+ +---------+ +---------+ +-----------+
                    |
                    v
    +------------------------------------------+
    |           Message Components              |
    | - User messages                          |
    | - Assistant messages (with streaming)    |
    | - Tool results                           |
    | - Code blocks (syntax highlighted)       |
    | - Error displays                         |
    +------------------------------------------+
```

### Tool Execution Flow

```
LLM Response with Tool Call
          |
          v
+-------------------+
| ParseToolCalls()  |
| - JSON format     |
| - Extract params  |
+-------------------+
          |
          v
+-------------------+
| Tool Registry     |
| - Lookup tool     |
| - Validate exists |
+-------------------+
          |
          v
+-------------------+
| Security Check    |
| - RiskLevel       |
| - PermissionLevel |
| - Path validation |
| - Command safety  |
+-------------------+
          |
     +----+----+
     |         |
     v         v
  Denied    Approved
     |         |
     v         v
  Return    +-------------------+
  Error     | Validate Params   |
            | - Type checking   |
            | - Path sanitizing |
            +-------------------+
                    |
                    v
            +-------------------+
            | Execute Tool      |
            | - Read/Write/etc  |
            | - Timeout: 10min  |
            +-------------------+
                    |
                    v
            +-------------------+
            | Record in History |
            | - Duration        |
            | - Success/Fail    |
            | - Audit log       |
            +-------------------+
                    |
                    v
            Return Result to LLM
```

---

## 8. Configuration Reference

### Config File Locations

```
~/.rigrun/config.toml    (primary, TOML format)
~/.rigrun/config.json    (fallback, JSON format)
```

### Configuration Sections

#### General Settings

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `version` | string | "1.0.0" | Config version |
| `default_model` | string | "qwen2.5-coder:14b" | Default local model |

#### Routing (`[routing]`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `default_mode` | string | "hybrid" | "local", "cloud", or "hybrid" |
| `max_tier` | string | "opus" | Maximum tier to route to |
| `paranoid_mode` | bool | false | Block all cloud requests |

#### Local (`[local]`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `ollama_url` | string | "http://localhost:11434" | Ollama server URL |
| `ollama_model` | string | "qwen2.5-coder:14b" | Default Ollama model |

#### Cloud (`[cloud]`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `openrouter_key` | string | "" | OpenRouter API key |
| `default_model` | string | "anthropic/claude-3.5-sonnet" | Default cloud model |

#### Security (`[security]`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `session_timeout_secs` | int | 900 | Session timeout (15 min DoD STIG) |
| `audit_enabled` | bool | true | Enable audit logging |
| `banner_enabled` | bool | false | Show DoD consent banner |
| `classification` | string | "UNCLASSIFIED" | Default classification |

#### Cache (`[cache]`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enabled` | bool | true | Enable caching |
| `ttl_hours` | int | 24 | Cache entry lifetime |
| `max_size` | int | 10000 | Maximum cache entries |
| `semantic_enabled` | bool | true | Enable semantic caching |
| `semantic_threshold` | float | 0.92 | Similarity threshold |

#### UI (`[ui]`)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `theme` | string | "dark" | "dark", "light", or "auto" |
| `show_cost` | bool | true | Display cost information |
| `show_tokens` | bool | true | Display token counts |
| `compact_mode` | bool | false | Use compact layout |

### Environment Variables

| Variable | Overrides | Description |
|----------|-----------|-------------|
| `RIGRUN_MODEL` | `default_model` | Model override |
| `RIGRUN_OPENROUTER_KEY` | `cloud.openrouter_key` | API key |
| `RIGRUN_PARANOID` | `routing.paranoid_mode` | "1" or "true" |
| `RIGRUN_OLLAMA_URL` | `local.ollama_url` | Ollama URL |
| `RIGRUN_MODE` | `routing.default_mode` | Routing mode |
| `RIGRUN_MAX_TIER` | `routing.max_tier` | Maximum tier |
| `RIGRUN_CLASSIFICATION` | `security.classification` | Classification level |

---

## 9. CLI Commands Reference

### Core Commands

| Command | Description |
|---------|-------------|
| `rigrun` | Start TUI interface (default) |
| `rigrun ask "question"` | Single query mode |
| `rigrun ask --agentic "task"` | Agentic mode with tool use |
| `rigrun chat` | Interactive chat mode |
| `rigrun status` | Show system status |
| `rigrun config [show\|set]` | Configuration management |
| `rigrun setup` | First-run wizard |
| `rigrun doctor` | System diagnostics |
| `rigrun cache [stats\|clear]` | Cache management |
| `rigrun models` | List available models |
| `rigrun pull <model>` | Download a model |

### Security Commands (Go TUI)

| Command | NIST Control | Description |
|---------|--------------|-------------|
| `/audit` | AU-6 | Review audit logs |
| `/compliance` | CA-7 | Compliance status |
| `/incidents` | IR-5 | Incident monitoring |
| `/rbac` | AC-5, AC-6 | Role-based access |
| `/permissions` | AC-6 | Permission management |
| `/config` | CM-5, CM-6 | Configuration management |
| `/save`, `/load` | AU-9, CP-9 | Session persistence |
| `/export` | AU-9 | Export conversations |

### Global Flags

| Flag | Description |
|------|-------------|
| `--paranoid` | Block all cloud requests |
| `--air-gapped` | Zero external connections |
| `--skip-banner` | Skip DoD consent banner |
| `-q, --quiet` | Minimal output |
| `-v, --verbose` | Debug output |
| `--model NAME` | Override default model |

---

## 10. Build and Installation

### Rust Implementation

```bash
# Prerequisites: Rust 1.70+, Ollama

# Development build
cargo build

# Release build (optimized, ~6.4 MB)
cargo build --release

# Run tests
cargo test

# Install from source
cargo install --path .

# Install from crates.io
cargo install rigrun
```

### Go Implementation

```bash
# Prerequisites: Go 1.22+, Ollama

cd go-tui

# Download dependencies
go mod download

# Build
go build -o rigrun ./main.go

# Run tests
go test ./...

# Run with race detector
go test -race ./...
```

### Cross-Platform Installation

#### macOS/Linux

```bash
# Using install script
curl -fsSL https://raw.githubusercontent.com/rigrun/rigrun/main/install.sh | bash

# Or manually
wget https://github.com/rigrun/rigrun/releases/latest/download/rigrun-linux-amd64
chmod +x rigrun-linux-amd64
sudo mv rigrun-linux-amd64 /usr/local/bin/rigrun
```

#### Windows

```powershell
# Using PowerShell script
irm https://raw.githubusercontent.com/rigrun/rigrun/main/install.ps1 | iex
```

### Docker

```bash
# Build image
docker build -t rigrun .

# Run container
docker run -p 8787:8787 rigrun

# With docker-compose
docker-compose up -d
```

---

## Appendix: CI/CD Workflows

### GitHub Actions (`.github/workflows/`)

| Workflow | Purpose |
|----------|---------|
| `ci.yml` | Continuous integration: Rust tests + clippy + fmt, Go tests on Linux/macOS/Windows |
| `release.yml` | Automated releases with cross-compilation |

### CI Pipeline

1. **Rust Tests**: `cargo check`, `cargo test`, `cargo fmt --check`, `cargo clippy`
2. **Go Tests**: `go build`, `go test ./...`, race detection
3. **Cross-Platform**: Ubuntu, macOS, Windows runners
4. **Security**: Token permissions restricted, dependabot enabled

---

*Generated: 2026-01-24*
*rigrun version: 0.2.0*
