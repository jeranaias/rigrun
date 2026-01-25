# rigrun

**Stop paying $20/month to send your code to someone else's servers.**

```
    ██████╗ ██╗ ██████╗ ██████╗ ██╗   ██╗███╗   ██╗
    ██╔══██╗██║██╔════╝ ██╔══██╗██║   ██║████╗  ██║
    ██████╔╝██║██║  ███╗██████╔╝██║   ██║██╔██╗ ██║
    ██╔══██╗██║██║   ██║██╔══██╗██║   ██║██║╚██╗██║
    ██║  ██║██║╚██████╔╝██║  ██║╚██████╔╝██║ ╚████║
    ╚═╝  ╚═╝╚═╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝
```

rigrun is an AI coding assistant that runs **on your hardware first**. A beautiful terminal interface. Intelligent query routing. Full DoD IL5 security compliance. Agentic file operations. And it's free.

---

## ⚡ Quick Start

**Two steps:**

1. **Install Ollama** → [ollama.ai](https://ollama.ai) (one-click installer)
2. **Install rigrun** → [Download](https://github.com/jeranaias/rigrun/releases/latest) and run

```bash
# Already have Go? One command:
go install github.com/jeranaias/rigrun@latest

# Then:
rigrun
```

That's it. Type a question, get an answer. Your code never leaves your machine.

→ [Full setup guide](#getting-started) | [Model recommendations](#recommended-models)

---

## Table of Contents

- [Quick Start](#-quick-start)
- [The Problem](#the-problem)
- [The Solution](#the-solution)
- [Why rigrun Over Alternatives](#why-rigrun-over-the-alternatives)
- [Who Is This For](#who-is-this-for)
- [Features Deep Dive](#features-deep-dive)
- [Getting Started](#getting-started)
- [The TUI Experience](#the-tui-experience)
- [Agentic Tools](#agentic-tools)
- [Intelligent Routing](#intelligent-routing)
- [Security & Compliance](#security--compliance)
- [Configuration](#configuration)
- [CLI Reference](#cli-reference)
- [Troubleshooting](#troubleshooting)
- [Architecture](#architecture)
- [FAQ](#faq)
- [Contributing](#contributing)

---

## The Problem

You're a developer. You use AI coding assistants every day. And you're probably:

- **Paying $20-40/month** for GitHub Copilot, ChatGPT Plus, or Claude Pro
- **Sending your proprietary code** to servers you don't control
- **Waiting on rate limits** when you need help most
- **Stuck without internet** on flights, in secure facilities, or just bad WiFi
- **Worried about data retention** policies you never read
- **Context switching** between your editor, browser, and terminal
- **Losing conversation history** every time you close a tab

Meanwhile, you have a GPU sitting in your machine doing nothing while you code.

---

## The Solution

rigrun is an AI coding assistant that runs **on your hardware first**.

Your GPU handles 90%+ of queries instantly, for free, with your data never leaving your machine. When you need the absolute best models for complex problems, rigrun can optionally route to cloud APIs - but only when YOU decide, and you see exactly what it costs.

**One command:**
```bash
rigrun
```

A beautiful terminal interface opens. Start typing. Your local model responds in milliseconds. Save conversations. Search your history. Include files with `@file:`. Have the AI read, write, and edit code for you. All in one place, all private, all free.

---

## Why rigrun Over the Alternatives?

### vs. ChatGPT Plus / Claude Pro

| Feature | ChatGPT Plus | Claude Pro | rigrun |
|---------|--------------|------------|--------|
| Monthly cost | $20 | $20 | **$0** |
| Your code sent to cloud | Yes, always | Yes, always | **No (local first)** |
| Works offline | No | No | **Yes** |
| Rate limits | Yes (40/3hr) | Yes (varies) | **None** |
| Response latency | 500ms-2s | 500ms-2s | **50-200ms local** |
| Works in secure facilities | No | No | **Yes** |
| Conversation history | Limited | Limited | **Unlimited local** |
| File context | Manual paste | Manual paste | **@file: mentions** |
| Agentic file ops | No | No | **Yes** |
| Session management | Per-tab | Per-tab | **Persistent sessions** |
| Keyboard-first | No | No | **Yes** |
| Syntax highlighting | Basic | Good | **Full terminal colors** |

### vs. GitHub Copilot

| Feature | Copilot Individual | Copilot Business | rigrun |
|---------|-------------------|------------------|--------|
| Monthly cost | $10 | $19 | **$0** |
| IDE lock-in | VSCode/JetBrains | VSCode/JetBrains | **Any terminal** |
| Telemetry | Extensive | Extensive | **Zero** |
| Works offline | No | No | **Yes** |
| Full conversation context | No (line-by-line) | Limited | **Yes** |
| Multi-file reasoning | No | Limited | **Yes** |
| Agentic file operations | No | Limited | **Full toolkit** |
| Custom model choice | No | No | **Yes** |
| Self-hostable | No | No | **Yes** |
| Security compliance | Trust GitHub | Trust GitHub | **IL5 certified** |

### vs. Cursor / Windsurf / Other AI IDEs

| Feature | Cursor Pro | Windsurf | rigrun |
|---------|------------|----------|--------|
| Monthly cost | $20 | $15 | **$0** |
| Requires specific IDE | Yes (Cursor fork) | Yes (VSCode fork) | **No** |
| Works with any editor | No | No | **Yes (terminal)** |
| Local-first | No | No | **Yes** |
| Works offline | No | No | **Yes** |
| Data privacy | Cloud-dependent | Cloud-dependent | **Complete** |
| Agentic tools | Yes | Yes | **Yes** |
| Security compliance | No | No | **IL5 certified** |

### vs. Running Ollama Directly

| Feature | Raw Ollama CLI | rigrun |
|---------|----------------|--------|
| Beautiful TUI | No | **Yes (30fps streaming)** |
| Conversation history | Manual | **Automatic persistence** |
| Session management | None | **Full (save/load/export)** |
| Smart model routing | None | **Cache → Local → Cloud** |
| Agentic tools | None | **8 tools (read/write/grep/bash/web)** |
| Cloud fallback | None | **Optional (OpenRouter)** |
| Context mentions | None | **@file, @git, @clipboard, @codebase** |
| Slash commands | None | **20+ commands** |
| Syntax highlighting | None | **Full support** |
| Progress indicators | Basic | **Animated spinners** |
| Security compliance | None | **IL5 certified** |
| Cost tracking | None | **Per-query and cumulative** |

---

## Who Is This For?

### Developers Who Value Privacy

Your code is your competitive advantage. Every query to ChatGPT, Claude, or Copilot sends your proprietary logic, architecture decisions, and business context to third-party servers with data retention policies you probably haven't read.

With rigrun, your code **never leaves your machine** unless you explicitly choose cloud routing. When you do use cloud, you see exactly what's being sent and to which provider. Complete transparency, complete control.

### Developers Who Are Tired of Subscriptions

The subscription creep is real:
- ChatGPT Plus: $20/month
- Claude Pro: $20/month
- GitHub Copilot: $10-19/month
- Cursor: $20/month

That's $50-80/month if you want the best of everything. rigrun costs **$0/month** for unlimited local queries. For the rare cases where you need GPT-4 or Claude Opus, you pay per-query via OpenRouter - typically **$1-5/month** for heavy users instead of $50+.

### Developers Who Work Offline

- **Flights**: 6 hours without AI assistance? Not anymore.
- **Coffee shops**: Bad WiFi doesn't kill your productivity.
- **Secure facilities**: SCIFs, classified networks, air-gapped systems.
- **Rural areas**: Satellite internet latency? Irrelevant.
- **VPN restrictions**: Company VPN blocks AI services? Local works.

rigrun with a local model works **completely offline**. Zero network required.

### Enterprise and Government Teams

rigrun implements **43 NIST 800-53 security controls** required for DoD IL5 certification:

- **AC-7**: Account lockout after failed attempts
- **AC-12**: Session timeout and termination
- **AU-2/AU-3**: Comprehensive audit logging
- **AU-9**: Tamper-evident log protection (HMAC)
- **CP-9/CP-10**: Encrypted backup and recovery
- **SC-7**: Network boundary protection
- **SC-13**: FIPS 140-2 cryptographic controls
- **SC-28**: Encryption at rest (AES-256-GCM)

This isn't security theater. These are the same controls required for systems handling CUI and classified information. Run `rigrun test security` to verify compliance.

### Developers Who Want to Use Their Hardware

You bought that RTX 4090, RX 7900, or M3 Max. It mostly sits idle while you code. rigrun puts it to work:

- **14B models**: Run comfortably on 8-10GB VRAM, handle 90% of coding tasks
- **7B models**: Run on 4GB VRAM, great for quick questions
- **22B+ models**: Premium quality on high-end GPUs
- **CPU fallback**: Works without a GPU, just slower

Your hardware, your inference, your data.

---

## Features Deep Dive

### Beautiful Terminal Interface

rigrun isn't a command-line afterthought. It's a **proper terminal application** built with [Bubble Tea](https://github.com/charmbracelet/bubbletea):

- **30fps streaming**: Responses render smoothly, character by character
- **Syntax highlighting**: Code blocks in any language, properly colored
- **Responsive design**: Adapts to terminal resize, any size works
- **Dark and light themes**: Easy on the eyes, matches your terminal
- **Progress indicators**: Animated spinners for all async operations
- **Status bar**: Model, cost, tokens - always visible
- **Vim keybindings**: Optional j/k navigation, : commands
- **Mouse support**: Click to position, scroll to navigate

### Intelligent Query Routing

Not all queries need GPT-4. rigrun automatically routes to the optimal tier:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Cache     │ ──▶ │   Local     │ ──▶ │   Cloud     │
│  (instant)  │     │  (your GPU) │     │ (optional)  │
│    FREE     │     │    FREE     │     │  $0.01-0.10 │
└─────────────┘     └─────────────┘     └─────────────┘
```

**Cache tier**: Semantic similarity matching. Asked "How do I reverse a string in Python?" before? Instant response from cache.

**Local tier**: Your Ollama model handles it. Sub-200ms response. Free. Private.

**Cloud tier**: Complex queries that need frontier models. You approve each escalation. Pay per query, not per month.

Configure routing with `/mode local`, `/mode cloud`, or `/mode auto`.

### Agentic File Operations

rigrun can interact with your codebase, not just talk about it:

| Tool | Permission | Description |
|------|------------|-------------|
| **Read** | Auto | Read any file in allowed directories |
| **Glob** | Auto | Find files by pattern (`**/*.go`, `src/**/*.ts`) |
| **Grep** | Auto | Search file contents with regex |
| **Write** | Ask | Create new files or overwrite existing |
| **Edit** | Ask | Search and replace within files |
| **Bash** | Ask | Execute shell commands |
| **WebSearch** | Auto | Search via DuckDuckGo (no API key) |
| **WebFetch** | Auto | Fetch web pages (SSRF protected) |

**Permission levels**:
- **Auto**: Always allowed, no prompt
- **Ask**: Shows you the operation, you approve/deny
- **Never**: Blocked (destructive commands like `rm -rf`)

Example session:
```
You: Find all TODO comments in this project and create a markdown summary

rigrun: I'll search for TODOs across the codebase.
[Uses Grep tool - automatic]

Found 47 TODO comments across 12 files. Here's the breakdown:
- src/auth/: 8 TODOs (security-related)
- src/api/: 15 TODOs (endpoint implementations)
...

Would you like me to create TODO.md with this summary?

You: Yes

rigrun: [Uses Write tool - asks permission]
Create file: TODO.md
Content: [shows preview]
Allow? (y/n)
```

### Context Mentions

Include context inline with `@` mentions:

| Mention | What It Does |
|---------|--------------|
| `@file:path/to/file` | Include file contents in context |
| `@file:src/*.go` | Include multiple files by glob pattern |
| `@clipboard` | Include current clipboard contents |
| `@git` | Include recent git history and diff |
| `@git:diff` | Include just the current diff |
| `@git:log` | Include recent commit messages |
| `@codebase` | Include project structure summary |
| `@error` | Include last error from tool execution |
| `@selection` | Include selected text (from IDE integration) |

Examples:
```
@file:src/auth/login.go explain this authentication flow

@git:diff review these changes before I commit

@codebase what's the overall architecture here?

@clipboard the user reported this error, what's wrong?
```

### Session Management

Conversations persist across restarts:

| Command | What It Does |
|---------|--------------|
| `/save [name]` | Save current conversation |
| `/load <id>` | Load a previous conversation |
| `/list` | List all saved sessions |
| `/export [format]` | Export as JSON, Markdown, or plain text |
| `/history [n]` | Show last N messages |
| `/new` | Start fresh conversation |
| `/clear` | Clear current but don't delete |

Sessions are stored in `~/.rigrun/sessions/` with full message history, tool calls, and metadata.

### Cost Tracking

Every query shows exactly what it cost:

```
─────────────────────────────────────────────
Tier: Local | Tokens: 847 | Cost: $0.00 | Saved: $0.12 vs Cloud
```

Track cumulative spending:
- `/status` - Current session costs
- `rigrun audit stats` - Historical cost analysis

---

## Getting Started

### Prerequisites

1. **Ollama** - The local LLM runtime
   - [Download Ollama](https://ollama.ai) (Windows, macOS, Linux)
   - Takes 30 seconds to install

2. **A GPU** (recommended but not required)
   - NVIDIA: CUDA support (most cards)
   - AMD: ROCm or Vulkan (RX 6000/7000 series)
   - Apple: Metal (M1/M2/M3)
   - CPU-only works, just slower

### Installation

#### Option 1: Download Installer (Recommended)

Download from [GitHub Releases](https://github.com/jeranaias/rigrun/releases/latest):

| Platform | File |
|----------|------|
| Windows | `rigrun-installer-windows-amd64.exe` |
| macOS Intel | `rigrun-installer-darwin-amd64` |
| macOS Apple Silicon | `rigrun-installer-darwin-arm64` |
| Linux | `rigrun-installer-linux-amd64` |

Run the installer - it's also a beautiful TUI that guides you through:
1. System requirements check
2. Ollama verification/setup
3. Model selection and download
4. Configuration creation
5. First launch

#### Option 2: Build From Source

```bash
# Clone
git clone https://github.com/jeranaias/rigrun.git
cd rigrun

# Build
go build -o rigrun .

# Pull a model
ollama pull qwen2.5-coder:14b

# Run
./rigrun
```

#### Option 3: Go Install

```bash
go install github.com/jeranaias/rigrun@latest
```

### First Run

```bash
rigrun
```

The TUI opens. Type a question. Get an answer. That's it.

### Recommended Models

| Model | VRAM | Speed | Quality | Best For |
|-------|------|-------|---------|----------|
| `qwen2.5-coder:7b` | 4GB | Fast | Good | Quick questions, low VRAM |
| `qwen2.5-coder:14b` | 9GB | Medium | Excellent | **Daily driver** |
| `deepseek-coder-v2:16b` | 10GB | Medium | Excellent | Complex understanding |
| `codestral:22b` | 13GB | Slower | Premium | Best local quality |
| `llama3.1:8b` | 5GB | Fast | Good | General purpose |
| `llama3.1:70b` | 40GB | Slow | Frontier | If you have the VRAM |

Pull with: `ollama pull qwen2.5-coder:14b`

Switch anytime with: `/model qwen2.5-coder:14b`

---

## The TUI Experience

### Slash Commands

Type `/` to see all commands, or use these directly:

#### Session Commands
| Command | Aliases | Description |
|---------|---------|-------------|
| `/help` | `/h`, `/?` | Show all commands |
| `/clear` | `/c` | Clear conversation |
| `/new` | `/n` | Start new conversation |
| `/save [name]` | `/s` | Save conversation |
| `/load <id>` | `/l`, `/resume` | Load conversation |
| `/list` | `/sessions` | List saved sessions |
| `/export [format]` | `/e` | Export (json/md/txt) |
| `/history [n]` | `/hist` | Show recent messages |
| `/copy` | - | Copy last response |

#### Model & Routing
| Command | Aliases | Description |
|---------|---------|-------------|
| `/model <name>` | `/m` | Switch AI model |
| `/mode <mode>` | - | Set routing (local/cloud/auto) |
| `/models` | - | List available models |
| `/streaming [on\|off]` | `/stream` | Toggle streaming |

#### Configuration
| Command | Aliases | Description |
|---------|---------|-------------|
| `/config [key]` | `/cfg` | View/set configuration |
| `/theme [dark\|light]` | - | Switch theme |

#### Tools & Status
| Command | Aliases | Description |
|---------|---------|-------------|
| `/tools [action]` | `/t` | Manage tools (list/enable/disable) |
| `/status` | - | System status |
| `/doctor` | `/diag` | Run diagnostics |
| `/cache [action]` | - | Cache stats/clear |
| `/tokens` | `/tok` | Token usage |
| `/context` | `/ctx` | Context window info |
| `/gpu` | - | GPU status |

#### Security (IL5)
| Command | Aliases | Description |
|---------|---------|-------------|
| `/audit [lines]` | - | View audit log |
| `/security` | `/sec` | Security status |
| `/classify [level]` | `/cls` | Set classification |
| `/consent` | - | Consent banner status |

#### Meta
| Command | Aliases | Description |
|---------|---------|-------------|
| `/version` | `/ver` | Version info |
| `/quit` | `/q`, `/exit` | Exit |
| `/retry` | - | Retry last failed message |
| `/tips` | - | Show usage tips |

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Shift+Enter` | New line (multi-line input) |
| `Ctrl+C` | Cancel generation / Quit |
| `Ctrl+L` | Clear screen |
| `Ctrl+P` | Command palette |
| `Tab` | Auto-complete file paths |
| `Up` | Previous message from history |
| `Down` | Next message from history |
| `PageUp` | Scroll response up |
| `PageDown` | Scroll response down |
| `Home` | Scroll to top |
| `End` | Scroll to bottom |
| `/` | Open slash command menu |
| `Esc` | Cancel current operation |

#### Vim Mode (Optional)
Enable with `/config vim_mode true`:
| Key | Action |
|-----|--------|
| `j` | Scroll down |
| `k` | Scroll up |
| `g` | Go to top |
| `G` | Go to bottom |
| `:` | Command mode |
| `:w` | Save session |
| `:q` | Quit |

---

## Agentic Tools

### Tool Reference

#### Read
Read file contents into context.
```
Arguments:
  file_path: string (required) - Absolute or relative path

Example: {"name": "Read", "arguments": {"file_path": "src/main.go"}}
```

#### Glob
Find files matching a pattern.
```
Arguments:
  pattern: string (required) - Glob pattern (e.g., "**/*.ts")
  path: string (optional) - Base directory

Example: {"name": "Glob", "arguments": {"pattern": "**/*.test.js"}}
```

#### Grep
Search file contents with regex.
```
Arguments:
  pattern: string (required) - Regex pattern
  path: string (optional) - Directory to search
  glob: string (optional) - File pattern filter
  output_mode: string (optional) - "content", "files_with_matches", "count"

Example: {"name": "Grep", "arguments": {"pattern": "TODO|FIXME", "glob": "**/*.go"}}
```

#### Write
Create or overwrite a file. **Requires permission.**
```
Arguments:
  file_path: string (required) - Path to write
  content: string (required) - File contents

Example: {"name": "Write", "arguments": {"file_path": "test.py", "content": "print('hello')"}}
```

#### Edit
Search and replace in a file. **Requires permission.**
```
Arguments:
  file_path: string (required) - File to edit
  old_string: string (required) - Text to find
  new_string: string (required) - Replacement text

Example: {"name": "Edit", "arguments": {"file_path": "config.js", "old_string": "debug: false", "new_string": "debug: true"}}
```

#### Bash
Execute a shell command. **Requires permission.**
```
Arguments:
  command: string (required) - Shell command to run
  timeout: number (optional) - Timeout in ms (default: 30000)

Example: {"name": "Bash", "arguments": {"command": "go test ./..."}}

Blocked commands: rm -rf /, format, mkfs, dd if=, shutdown, reboot
```

#### WebSearch
Search the web via DuckDuckGo (no API key required).
```
Arguments:
  query: string (required) - Search query
  num_results: number (optional) - Max results (default: 5)

Example: {"name": "WebSearch", "arguments": {"query": "golang context best practices"}}
```

#### WebFetch
Fetch a web page and extract content.
```
Arguments:
  url: string (required) - URL to fetch

Example: {"name": "WebFetch", "arguments": {"url": "https://go.dev/doc/effective_go"}}

Security: SSRF protection blocks internal/private IPs
```

### Tool Security

Tools operate within a security sandbox:

- **Allowed paths**: Current directory, home directory, temp directory
- **Blocked paths**: System directories, other users' directories
- **Blocked commands**: Destructive operations (rm -rf /, format, etc.)
- **Network protection**: SSRF protection, private IP blocking
- **Permission escalation**: Write/Edit/Bash always require approval

Configure tool permissions in `~/.rigrun/config.toml`:
```toml
[tools]
read_permission = "auto"      # auto, ask, never
write_permission = "ask"      # auto, ask, never
bash_permission = "ask"       # auto, ask, never
web_permission = "auto"       # auto, ask, never
```

---

## Intelligent Routing

### How It Works

Every query goes through the routing pipeline:

1. **Classification**: Analyze query complexity (trivial/simple/moderate/complex/expert)
2. **Cache check**: Look for semantic or exact match
3. **Tier selection**: Route to minimum capable tier
4. **Execution**: Run query, track cost
5. **Cache update**: Store for future hits

### Routing Modes

| Mode | Behavior |
|------|----------|
| `local` | Always use local model, never cloud |
| `cloud` | Always use cloud (for testing) |
| `auto` | Smart routing based on complexity |
| `hybrid` | Local first, cloud fallback on failure |

Set with `/mode <mode>` or `rigrun config set routing.default_mode <mode>`

### Complexity Classification

| Level | Examples | Default Tier |
|-------|----------|--------------|
| Trivial | "What is 2+2?", "Hello" | Cache/Local |
| Simple | "What is a mutex?", "Fix this typo" | Local |
| Moderate | "Explain this function", "Write a test" | Local |
| Complex | "Design a system for...", "Analyze architecture" | Local/Cloud |
| Expert | "Novel algorithm for...", "Security audit" | Cloud |

### Cloud Providers

rigrun uses [OpenRouter](https://openrouter.ai) for cloud access, giving you:
- Claude 3.5 Sonnet, Claude 3 Opus
- GPT-4, GPT-4 Turbo
- Llama 3.1 405B
- Mixtral, Command R+
- And 100+ other models

Set your API key:
```bash
rigrun config set openrouter_key sk-or-...
```

### Cost Control

- **Paranoid mode**: Block ALL cloud requests
  ```bash
  rigrun config set paranoid_mode true
  ```

- **Max tier**: Limit how high routing can escalate
  ```bash
  rigrun config set routing.max_tier sonnet  # Never use Opus
  ```

- **Per-query approval**: In auto mode, you approve cloud escalations

---

## Security & Compliance

### NIST 800-53 Controls

rigrun implements 43 security controls across these families:

#### Access Control (AC)
| Control | Description | Implementation |
|---------|-------------|----------------|
| AC-3 | Access Enforcement | Role-based access control |
| AC-5 | Separation of Duties | Distinct admin/operator/user roles |
| AC-6 | Least Privilege | Minimal default permissions |
| AC-7 | Unsuccessful Logon Attempts | 3 attempts, 15-min lockout |
| AC-8 | System Use Notification | DoD consent banner |
| AC-12 | Session Termination | Configurable timeout (default 30min) |
| AC-17 | Remote Access | Secure API authentication |

#### Audit & Accountability (AU)
| Control | Description | Implementation |
|---------|-------------|----------------|
| AU-2 | Audit Events | All queries, tools, sessions logged |
| AU-3 | Content of Audit Records | Timestamp, user, action, result |
| AU-5 | Response to Failures | Alert on audit failure |
| AU-6 | Audit Review | `/audit` command, SIEM export |
| AU-9 | Protection of Audit Info | HMAC tamper detection |
| AU-11 | Audit Record Retention | Configurable retention policy |

#### Contingency Planning (CP)
| Control | Description | Implementation |
|---------|-------------|----------------|
| CP-9 | System Backup | Encrypted backup command |
| CP-10 | System Recovery | Verified restore process |

#### Identification & Authentication (IA)
| Control | Description | Implementation |
|---------|-------------|----------------|
| IA-2 | User Identification | API key authentication |
| IA-7 | Cryptographic Auth | PKI certificate support |

#### System & Communications (SC)
| Control | Description | Implementation |
|---------|-------------|----------------|
| SC-7 | Boundary Protection | Network isolation modes |
| SC-8 | Transmission Confidentiality | TLS 1.3 required |
| SC-12 | Key Management | Secure key storage |
| SC-13 | Cryptographic Protection | FIPS 140-2 algorithms |
| SC-17 | PKI Certificates | Certificate pinning |
| SC-28 | Protection at Rest | AES-256-GCM encryption |

#### System & Information Integrity (SI)
| Control | Description | Implementation |
|---------|-------------|----------------|
| SI-7 | Software Integrity | Binary verification |

### Classification Levels

Set classification with CLI or TUI:

```bash
rigrun classify set UNCLASSIFIED
rigrun classify set CUI
rigrun classify set CONFIDENTIAL
rigrun classify set SECRET
rigrun classify set SECRET --caveat NOFORN
rigrun classify set TOP_SECRET
```

Classification affects:
- Banner display
- Routing restrictions (CUI+ forced local)
- Audit requirements
- Network policies

### Air-Gapped Operation

For completely isolated environments:

```bash
rigrun --no-network
# or
rigrun config set security.offline_mode true
```

This blocks:
- All cloud API calls
- WebSearch and WebFetch tools
- Any network egress except localhost Ollama

### Verify Compliance

```bash
# Run security self-tests
rigrun test security

# Verify integrity
rigrun verify all

# Check compliance status
rigrun sectest run
```

---

## Configuration

### Config File

Location: `~/.rigrun/config.toml`

```toml
# ============================================================
# rigrun Configuration
# ============================================================

# General settings
[general]
default_model = "qwen2.5-coder:14b"
default_mode = "local"

# Local Ollama settings
[local]
ollama_url = "http://localhost:11434"
ollama_model = "qwen2.5-coder:14b"

# Cloud settings (optional)
[cloud]
openrouter_key = ""                           # Your OpenRouter API key
cloud_model = "anthropic/claude-3.5-sonnet"   # Default cloud model

# Routing behavior
[routing]
default_mode = "local"     # local, cloud, auto, hybrid
max_tier = "opus"          # Maximum tier: haiku, sonnet, opus
paranoid_mode = false      # true = block ALL cloud requests
offline_mode = false       # true = block ALL network (air-gapped)

# Security settings
[security]
session_timeout = 1800     # Seconds (30 min default, IL5 requires <=30)
audit_enabled = true       # Enable audit logging
banner_enabled = false     # Show classification banner
classification = "UNCLASSIFIED"

# Cache settings
[cache]
enabled = true
ttl_hours = 24             # Cache entry lifetime
max_size = 10000           # Max cached entries
semantic_enabled = true    # Enable semantic similarity matching
semantic_threshold = 0.92  # Similarity threshold (0-1)

# UI settings
[ui]
theme = "dark"             # dark, light
vim_mode = false           # Enable Vim keybindings
show_cost = true           # Show cost in status bar
show_tokens = true         # Show token count
compact_mode = false       # Reduce padding

# Tool permissions
[tools]
read_permission = "auto"
write_permission = "ask"
bash_permission = "ask"
web_permission = "auto"
```

### Environment Variables

Override any config with environment variables:

| Variable | Description |
|----------|-------------|
| `RIGRUN_MODEL` | Override default model |
| `RIGRUN_OPENROUTER_KEY` | Set OpenRouter API key |
| `RIGRUN_PARANOID` | `1` or `true` for paranoid mode |
| `RIGRUN_OFFLINE` | `1` or `true` for offline mode |
| `RIGRUN_OLLAMA_URL` | Override Ollama URL |
| `RIGRUN_MODE` | Override routing mode |
| `RIGRUN_MAX_TIER` | Override max tier |
| `RIGRUN_CLASSIFICATION` | Override classification |
| `RIGRUN_THEME` | Override theme |

### CLI Configuration

```bash
# View all config
rigrun config show

# Set a value
rigrun config set paranoid_mode true
rigrun config set ollama_model llama3.1:8b

# Reset to defaults
rigrun config reset
```

---

## CLI Reference

### Core Commands

```bash
rigrun                      # Start TUI (default)
rigrun ask "question"       # One-shot question
rigrun ask --file main.go "review"  # Include file
rigrun ask --agentic "task" # Enable tools
rigrun chat                 # Chat mode without full TUI
rigrun status               # System status
rigrun doctor               # Diagnostics
rigrun help                 # Show help
```

### Session Commands

```bash
rigrun session list                    # List sessions
rigrun session show <id>               # Show session details
rigrun session export <id> --format md # Export session
rigrun session delete <id> --confirm   # Delete session
rigrun session stats                   # Session statistics
```

### Security Commands

```bash
rigrun audit show --lines 50           # View audit log
rigrun audit export --format json      # Export for SIEM
rigrun audit verify                    # Verify log integrity
rigrun backup create full              # Create backup
rigrun backup restore <id> --confirm   # Restore backup
rigrun classify set SECRET             # Set classification
rigrun consent accept                  # Accept DoD banner
rigrun lockout status                  # Check lockout
rigrun test security                   # Run security tests
rigrun verify all                      # Verify integrity
```

### Maintenance Commands

```bash
rigrun cache stats                     # Cache statistics
rigrun cache clear                     # Clear cache
rigrun maintenance start --operator ID # Enter maintenance mode
rigrun maintenance end                 # Exit maintenance
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--paranoid` | Block all cloud requests |
| `--no-network`, `--offline` | Air-gapped mode |
| `--skip-banner` | Skip consent banner |
| `-q`, `--quiet` | Minimal output |
| `-v`, `--verbose` | Debug output |
| `--model NAME` | Override model |
| `--json` | JSON output for scripting |

---

## Troubleshooting

### Common Issues

#### Ollama Not Running

```bash
# Check if running
ollama list

# Start it
ollama serve
```

#### Model Not Found

```bash
# List available models
ollama list

# Pull the model you need
ollama pull qwen2.5-coder:14b
```

#### GPU Not Detected

```bash
# Check GPU status
rigrun status

# For AMD on Windows, you may need:
# Set OLLAMA_VULKAN=1 before starting Ollama
```

#### Slow Responses

- Try a smaller model: `qwen2.5-coder:7b`
- Check GPU utilization with `rigrun gpu`
- Ensure Ollama is using GPU, not CPU

#### Cloud Not Working

```bash
# Verify API key
rigrun config show | grep openrouter

# Test connection
rigrun test connectivity
```

### Diagnostics

```bash
# Full system check
rigrun doctor

# Self-tests
rigrun test all

# Security tests
rigrun test security

# Verbose mode for debugging
rigrun -v ask "test"
```

---

## Architecture

```
rigrun/
├── main.go                    # Entry point, TUI model
├── cmd/
│   └── installer/             # TUI installer
├── internal/
│   ├── cli/                   # 50+ CLI command handlers
│   │   ├── ask.go            # Ask command with agentic loop
│   │   ├── audit_cmd.go      # Audit log commands
│   │   ├── backup_cmd.go     # Backup/restore commands
│   │   ├── suggest.go        # Typo suggestions
│   │   └── ...
│   ├── config/               # Configuration management
│   ├── ollama/               # Ollama API client
│   ├── cloud/                # OpenRouter API client
│   ├── router/               # Query routing logic
│   │   ├── classifier.go     # Query complexity analysis
│   │   └── tier.go           # Tier selection
│   ├── cache/                # Semantic + exact caching
│   ├── session/              # Session persistence
│   ├── security/             # NIST 800-53 implementations
│   │   ├── audit.go          # AU-2/AU-3 audit logging
│   │   ├── auditprotect.go   # AU-9 tamper protection
│   │   ├── backup.go         # CP-9/CP-10 backup
│   │   ├── boundary.go       # SC-7 network boundary
│   │   ├── crypto.go         # SC-13 cryptographic controls
│   │   ├── encrypt.go        # SC-28 encryption at rest
│   │   ├── lockout.go        # AC-7 account lockout
│   │   ├── rbac.go           # AC-5/AC-6 role-based access
│   │   └── ...
│   ├── tools/                # Agentic tool implementations
│   │   ├── read.go           # Read tool
│   │   ├── glob.go           # Glob tool
│   │   ├── grep.go           # Grep tool
│   │   ├── write.go          # Write tool
│   │   ├── edit.go           # Edit tool
│   │   ├── bash.go           # Bash tool
│   │   ├── web.go            # WebFetch tool
│   │   ├── duckduckgo.go     # WebSearch tool
│   │   ├── executor.go       # Tool execution engine
│   │   ├── agentic.go        # Agentic loop orchestration
│   │   └── security.go       # Tool sandboxing
│   ├── context/              # @ mention parsing
│   ├── commands/             # Slash command registry
│   ├── model/                # Data models
│   ├── detect/               # GPU detection
│   ├── offline/              # Air-gapped mode
│   └── ui/                   # TUI components
│       ├── chat/             # Chat view
│       ├── components/       # Reusable UI components
│       └── styles/           # Theming
└── pkg/
    ├── markdown/             # Markdown rendering
    └── syntax/               # Syntax highlighting
```

### Key Design Decisions

1. **Local-first**: All queries try local before cloud
2. **Privacy by default**: No telemetry, no data collection
3. **Security as architecture**: IL5 controls baked in, not bolted on
4. **TUI-centric**: Full experience in terminal, CLI for automation
5. **Tool safety**: Sandbox by default, permission escalation for writes

---

## FAQ

**Q: Is my code really private?**

A: Yes. With local models, queries never leave your machine - they go to Ollama on localhost. When you enable cloud, you explicitly see what's sent. No telemetry, no data collection, no training on your queries.

**Q: How does quality compare to ChatGPT/Claude?**

A: For 90% of coding tasks (explaining code, writing functions, debugging, refactoring), a good 14B model like Qwen 2.5 Coder produces equivalent results. For the 10% that need frontier models, you can route to cloud.

**Q: What if local gives a bad answer?**

A: Ask it to try again (often works), rephrase your question, use `/mode cloud` for that query, or switch to a larger local model.

**Q: Can I use this at work?**

A: Yes. The IL5 security controls make rigrun suitable for enterprise and government. Run `rigrun test security` to verify compliance, and check with your security team about specific requirements.

**Q: What GPUs are supported?**

A: Anything Ollama supports:
- NVIDIA: CUDA (most cards)
- AMD: ROCm on Linux, Vulkan on Windows
- Apple: Metal (M1/M2/M3)
- CPU-only: Works, just slower

**Q: Is this affiliated with Anthropic/OpenAI/any AI company?**

A: No. rigrun is independent open-source software. It uses Ollama (open-source) for local inference and OpenRouter (API aggregator) for optional cloud access.

**Q: Why AGPL license?**

A: To ensure improvements stay open-source. You can use rigrun for any purpose. If you modify and distribute it, you share your changes under the same license.

**Q: How do I report bugs or request features?**

A: [GitHub Issues](https://github.com/jeranaias/rigrun/issues) for bugs, [GitHub Discussions](https://github.com/jeranaias/rigrun/discussions) for features and questions.

---

## Contributing

rigrun is open source and welcomes contributions:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes
4. Run tests: `go test ./...`
5. Commit: `git commit -m 'Add amazing feature'`
6. Push: `git push origin feature/amazing-feature`
7. Open a Pull Request

### Development Setup

```bash
git clone https://github.com/jeranaias/rigrun.git
cd rigrun
go mod download
go build -o rigrun .
```

### Code Style

- Go standard formatting (`gofmt`)
- Clear comments on exported functions
- Tests for new features
- No breaking changes to config without migration path

---

## License

**AGPL-3.0** - Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge

You can:
- Use rigrun for any purpose (personal, commercial, government)
- Modify the source code
- Distribute copies

You must:
- Share source code of modifications if you distribute
- License modifications under AGPL-3.0
- Preserve copyright notices

---

## Acknowledgments

- [Ollama](https://ollama.ai) - Local LLM runtime
- [OpenRouter](https://openrouter.ai) - Cloud LLM gateway
- [Charm](https://charm.sh) - Beautiful TUI libraries (Bubble Tea, Lip Gloss, Bubbles)
- [NIST](https://csrc.nist.gov) - SP 800-53 Security Controls

---

<p align="center">
<b>Your GPU. Your data. Your terminal.<br>
No subscriptions. No telemetry. No compromises.</b>
</p>

```bash
rigrun
```
