# rigrun

**Your GPU, your data, your interface - beautiful.**

A stunning terminal UI for local LLM chat with intelligent query routing and full DoD IL5 compliance.

```
           ██████╗ ██╗ ██████╗ ██████╗ ██╗   ██╗███╗   ██╗
           ██╔══██╗██║██╔════╝ ██╔══██╗██║   ██║████╗  ██║
           ██████╔╝██║██║  ███╗██████╔╝██║   ██║██╔██╗ ██║
           ██╔══██╗██║██║   ██║██╔══██╗██║   ██║██║╚██╗██║
           ██║  ██║██║╚██████╔╝██║  ██║╚██████╔╝██║ ╚████║
           ╚═╝  ╚═╝╚═╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝
```

## Features

- **Beautiful TUI** - World-class terminal interface built with Bubble Tea and Lip Gloss
- **Intelligent Routing** - Automatically routes queries to optimal tier (cache/local/cloud)
- **Local-First** - Runs on your GPU with Ollama for privacy and zero API costs
- **Cloud Fallback** - Seamlessly escalates to cloud models via OpenRouter when needed
- **Semantic Caching** - Instant responses for semantically similar queries
- **DoD IL5 Compliance** - Full NIST 800-53 control implementation
- **Agentic Tools** - Read, write, edit files, web search, execute commands with permission controls
- **Offline Mode** - Air-gapped operation with complete network isolation

## Quick Start

### Option 1: Interactive Installer (Recommended)

The easiest way to get started is with the interactive installer:

```bash
# Build and run the installer
go build -o rigrun-installer.exe ./cmd/installer
./rigrun-installer

# Or use the shell scripts
# Linux/macOS:
./scripts/install.sh

# Windows PowerShell:
.\scripts\install.ps1
```

The installer will:
- Check system requirements
- Set up Ollama if needed
- Download a recommended AI model
- Create your configuration

For non-interactive environments, use text mode:
```bash
./rigrun-installer --text
```

### Option 2: Manual Installation

#### Prerequisites

1. Install [Ollama](https://ollama.ai) and pull a model:
   ```bash
   ollama pull qwen2.5-coder:14b
   ```

2. Download the latest release or build from source:
   ```bash
   go build -o rigrun.exe .
   ```

3. Run rigrun:
   ```bash
   rigrun
   ```

## Intelligent Routing

rigrun automatically routes queries to the optimal tier based on complexity:

| Tier | Description | Cost | Use Case |
|------|-------------|------|----------|
| **Cache** | Instant responses for repeated/similar queries | Free | Semantic/exact cache hits |
| **Local** | Your GPU via Ollama | Free | Most queries |
| **Cloud** | OpenRouter auto-selection | $$ | Complex tasks needing best models |
| **Haiku** | Claude 3 Haiku | $ | Quick cloud fallback |
| **Sonnet** | Claude 3.5 Sonnet | $$ | Balanced quality/cost |
| **Opus** | Claude 3 Opus | $$$ | Expert-level reasoning |

### How Routing Works

1. **Cache Check** - First checks semantic and exact cache for instant hits
2. **Complexity Analysis** - Classifies query complexity (trivial/simple/moderate/complex/expert)
3. **Tier Selection** - Routes to minimum tier that can handle the complexity
4. **Paranoid Mode** - Optional mode that blocks all cloud requests

---

## CLI Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `rigrun` | Start interactive TUI (default) |
| `rigrun ask "question"` | Quick one-shot question |
| `rigrun chat` | Start chat session without full TUI |
| `rigrun status` | Show system status (Ollama, GPU, models) |
| `rigrun config show` | Display current configuration |
| `rigrun config set <key> <value>` | Set a configuration value |
| `rigrun config reset` | Reset to default configuration |
| `rigrun doctor` | Run diagnostics |
| `rigrun cache stats` | Show cache statistics |
| `rigrun cache clear` | Clear the cache |
| `rigrun setup` | Interactive setup wizard |
| `rigrun --version` | Show version information |
| `rigrun --help` | Show help |
| `rigrun --no-network` | Start in offline mode (air-gapped) |

### DoD IL5 Security Commands

#### Authentication (IA-2)

| Command | Description |
|---------|-------------|
| `rigrun auth status` | Show authentication status |
| `rigrun auth login` | Authenticate with API key |
| `rigrun auth logout` | End current session |
| `rigrun auth validate` | Validate configured API key |
| `rigrun auth sessions` | List active sessions |
| `rigrun auth mfa status` | Show MFA status |

#### Access Control (AC-5/AC-6/AC-7)

| Command | Description |
|---------|-------------|
| `rigrun rbac status` | Show current user role and permissions |
| `rigrun rbac assign <user> <role>` | Assign role (admin only) |
| `rigrun rbac revoke <user> <role>` | Revoke role (admin only) |
| `rigrun rbac list-roles` | List all available roles |
| `rigrun rbac list-users` | List users and their roles |
| `rigrun rbac check <permission>` | Check if permission is granted |
| `rigrun lockout status` | Show account lockout status |
| `rigrun lockout unlock <user>` | Unlock a locked account |
| `rigrun lockout history` | Show lockout history |

#### Audit (AU-6/AU-9/AU-11)

| Command | Description |
|---------|-------------|
| `rigrun audit view [lines]` | View recent audit entries |
| `rigrun audit export <format>` | Export audit log (json/csv) |
| `rigrun audit search <query>` | Search audit entries |
| `rigrun audit retention` | Show retention policy |

#### Encryption (SC-28/SC-13)

| Command | Description |
|---------|-------------|
| `rigrun encrypt init` | Initialize encryption (creates master key) |
| `rigrun encrypt config` | Encrypt config file sensitive fields |
| `rigrun encrypt cache` | Encrypt cache database |
| `rigrun encrypt audit` | Encrypt audit logs |
| `rigrun encrypt status` | Show encryption status |
| `rigrun encrypt rotate` | Rotate master key |
| `rigrun crypto status` | Show cryptographic controls status |
| `rigrun crypto ciphers` | List approved cipher suites |
| `rigrun crypto fips` | Show FIPS compliance status |
| `rigrun crypto pki` | Show PKI certificate status |

#### Network Security (SC-7/SC-8/AC-17)

| Command | Description |
|---------|-------------|
| `rigrun boundary status` | Show boundary protection status |
| `rigrun boundary policy` | Show current network policy |
| `rigrun boundary allow <host>` | Add host to allowlist |
| `rigrun boundary block <host>` | Block a host |
| `rigrun boundary unblock <host>` | Unblock a host |
| `rigrun boundary connections` | List active connections |
| `rigrun boundary enforce <on\|off>` | Enable/disable egress filtering |
| `rigrun transport status` | Show TLS configuration |
| `rigrun transport verify` | Verify transport security |
| `rigrun transport ciphers` | List allowed ciphers |
| `rigrun transport sessions` | List active remote sessions |
| `rigrun transport terminate <id>` | Terminate remote session |
| `rigrun transport policy` | Show transport policy |

#### Backup & Recovery (CP-9/CP-10)

| Command | Description |
|---------|-------------|
| `rigrun backup create [type]` | Create backup (config/cache/audit/full) |
| `rigrun backup restore <id>` | Restore from backup |
| `rigrun backup list` | List all backups |
| `rigrun backup verify <id>` | Verify backup integrity |
| `rigrun backup delete <id>` | Delete a backup |
| `rigrun backup schedule` | Show/set backup schedule |
| `rigrun backup status` | Show backup system status |

#### Vulnerability Management (RA-5)

| Command | Description |
|---------|-------------|
| `rigrun vuln scan` | Run full vulnerability scan |
| `rigrun vuln list` | List all vulnerabilities |
| `rigrun vuln show <id>` | Show vulnerability details |
| `rigrun vuln export <format>` | Export vulnerability report |
| `rigrun vuln schedule` | Show/set scan schedule |
| `rigrun vuln status` | Show scanner status |

#### Security Testing (SA-11)

| Command | Description |
|---------|-------------|
| `rigrun sectest run` | Run all security tests |
| `rigrun sectest static` | Run static analysis |
| `rigrun sectest deps` | Check dependency vulnerabilities |
| `rigrun sectest fuzz` | Run fuzz tests |
| `rigrun sectest report` | Generate security test report |
| `rigrun sectest history` | Show test history |
| `rigrun sectest status` | Show testing status |

#### Integrity Verification (SI-7)

| Command | Description |
|---------|-------------|
| `rigrun verify status` | Show integrity verification status |
| `rigrun verify software` | Verify software integrity |
| `rigrun verify firmware` | Verify firmware integrity |
| `rigrun verify information` | Verify information integrity |

#### Training & Compliance (AT-2/AT-3/PS-6/PL-4)

| Command | Description |
|---------|-------------|
| `rigrun training status` | Show training compliance status |
| `rigrun training required [role]` | List required training |
| `rigrun training complete <courseID> <score>` | Record completion |
| `rigrun training history` | Show training history |
| `rigrun training expiring` | Show expiring certifications |
| `rigrun training report` | Generate training report |
| `rigrun training courses` | List available courses |
| `rigrun agreements list` | List all access agreements |
| `rigrun agreements show <id>` | Show agreement details |
| `rigrun agreements sign <id>` | Sign an agreement |
| `rigrun agreements status` | Show agreement status |
| `rigrun agreements check` | Check for unsigned agreements |
| `rigrun rules show` | Show rules of behavior |
| `rigrun rules acknowledge` | Acknowledge rules |
| `rigrun rules status` | Show acknowledgment status |

#### Maintenance (MA-4/MA-5)

| Command | Description |
|---------|-------------|
| `rigrun maintenance status` | Show maintenance mode status |
| `rigrun maintenance enable` | Enter maintenance mode |
| `rigrun maintenance disable` | Exit maintenance mode |

#### Configuration Management (CM-5/CM-6)

| Command | Description |
|---------|-------------|
| `rigrun configmgmt status` | Show configuration management status |
| `rigrun configmgmt baseline` | Show configuration baseline |
| `rigrun configmgmt changes` | List configuration changes |

#### Continuous Monitoring (CA-7)

| Command | Description |
|---------|-------------|
| `rigrun conmon status` | Show continuous monitoring status |
| `rigrun conmon alerts` | List active alerts |
| `rigrun conmon metrics` | Show security metrics |

#### Session Management (AC-12)

| Command | Description |
|---------|-------------|
| `rigrun session list` | List active sessions |
| `rigrun session show <id>` | Show session details |
| `rigrun session export <id>` | Export session |
| `rigrun session delete <id>` | Delete a session |
| `rigrun session delete-all` | Delete all sessions |
| `rigrun session stats` | Show session statistics |

#### Classification (AC-3/SC-16)

| Command | Description |
|---------|-------------|
| `rigrun classify status` | Show classification status |
| `rigrun classify set <level>` | Set classification level |

#### Incident Response (IR-*)

| Command | Description |
|---------|-------------|
| `rigrun incident status` | Show incident response status |
| `rigrun incident create` | Create new incident |
| `rigrun incident list` | List incidents |

#### Media Sanitization (MP-6)

| Command | Description |
|---------|-------------|
| `rigrun sanitize status` | Show sanitization status |
| `rigrun sanitize run` | Run sanitization |

---

## TUI Features

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Ctrl+C` | Cancel generation / Quit |
| `Ctrl+L` | Clear conversation |
| `Ctrl+P` | Open command palette |
| `Tab` | Complete command/file path |
| `Up/Down` | Scroll through history |
| `PageUp/PageDown` | Scroll viewport |
| `/` | Open slash command menu |

### Slash Commands

Type `/` to see available commands:

#### Help & Meta

| Command | Aliases | Description |
|---------|---------|-------------|
| `/help` | `/h`, `/?` | Show help |
| `/quit` | `/q`, `/exit` | Exit application |
| `/version` | `/ver` | Show version info |

#### Session Management

| Command | Aliases | Description |
|---------|---------|-------------|
| `/clear` | `/c` | Clear conversation |
| `/new` | `/n` | Start new conversation |
| `/save [name]` | `/s` | Save conversation |
| `/load <id>` | `/l`, `/resume` | Load conversation |
| `/list` | `/sessions` | List saved sessions |
| `/export [format]` | `/e` | Export (json/md/txt) |
| `/history [n]` | `/hist` | Show last N messages |
| `/copy` | - | Copy last response to clipboard |

#### Security & Compliance

| Command | Aliases | Description |
|---------|---------|-------------|
| `/audit [lines]` | - | Show recent audit log entries |
| `/security` | `/sec` | Show security status summary |
| `/classify [level]` | `/cls` | Show/set classification level |
| `/consent` | - | Show consent status |

#### Configuration

| Command | Aliases | Description |
|---------|---------|-------------|
| `/config [key]` | `/cfg` | Show configuration |
| `/model <name>` | `/m` | Switch model |
| `/mode <mode>` | - | Switch routing mode |
| `/streaming [on\|off]` | `/stream` | Toggle streaming |

#### Tools & System

| Command | Aliases | Description |
|---------|---------|-------------|
| `/tools [action]` | `/t` | Tool management (list/enable/disable) |
| `/doctor` | `/diag` | Run system diagnostics |
| `/models [action]` | - | Model management (list/info) |

#### Status & Information

| Command | Aliases | Description |
|---------|---------|-------------|
| `/status` | - | Show current status |
| `/cache [action]` | - | Cache management (stats/clear) |
| `/gpu` | - | Show GPU status |
| `/tokens` | `/tok` | Show token usage |
| `/context` | `/ctx` | Show context window info |

### Context Mentions

Include context in your messages with @ mentions:

| Mention | Description |
|---------|-------------|
| `@file:path/to/file` | Include file contents |
| `@clipboard` | Include clipboard contents |
| `@git` | Include git context (recent commits, diff) |
| `@codebase` | Include codebase summary |
| `@error` | Include last error |

Example:
```
@file:src/main.go What does this code do?
```

---

## Agentic Tools

rigrun supports agentic tool use for code assistance:

| Tool | Permission | Description |
|------|------------|-------------|
| **Read** | Auto | Read file contents |
| **Glob** | Auto | Find files by pattern |
| **Grep** | Auto | Search file contents with regex |
| **Write** | Ask | Write/create file contents |
| **Edit** | Ask | Edit file with search/replace |
| **Bash** | Ask | Execute shell commands |
| **WebFetch** | Auto | Fetch web content with SSRF protection |
| **WebSearch** | Auto | DuckDuckGo search (no API key needed) |

### Permission Levels

- **Auto**: Always allowed, no prompt
- **Ask**: Requires user confirmation
- **Never**: Always blocked

### Tool-Supported Models

For best tool calling results, use models with native function calling support:

| Model | Tool Support |
|-------|--------------|
| `llama3.1:8b+` | Excellent |
| `llama3.2:3b+` | Good |
| `qwen2.5:7b+` | Excellent |
| `qwen2.5-coder:7b+` | Excellent |
| `mistral:7b+` | Good |
| `mixtral:8x7b` | Excellent |
| `command-r:35b` | Excellent |

---

## DoD IL5 Compliance Features

rigrun implements comprehensive NIST 800-53 Rev 5 security controls for DoD IL5 environments.

### NIST 800-53 Control Summary

| Control Family | Controls | Implementation |
|----------------|----------|----------------|
| **Access Control (AC)** | AC-3, AC-5, AC-6, AC-7, AC-12, AC-17 | RBAC, separation of duties, least privilege, lockout, session management, remote access |
| **Awareness & Training (AT)** | AT-2, AT-3 | Security awareness training, role-based training |
| **Audit & Accountability (AU)** | AU-2, AU-3, AU-6, AU-9, AU-11 | Audit events, review, protection, retention |
| **Security Assessment (CA)** | CA-7 | Continuous monitoring |
| **Configuration Mgmt (CM)** | CM-5, CM-6 | Change control, configuration settings |
| **Contingency Planning (CP)** | CP-9, CP-10 | Backup, recovery |
| **Identification & Auth (IA)** | IA-2, IA-7 | User authentication, cryptographic auth |
| **Incident Response (IR)** | IR-4, IR-5, IR-6 | Incident handling, monitoring, reporting |
| **Maintenance (MA)** | MA-4, MA-5 | Nonlocal maintenance, personnel |
| **Media Protection (MP)** | MP-6 | Media sanitization |
| **Personnel Security (PS)** | PS-6 | Access agreements |
| **Planning (PL)** | PL-4 | Rules of behavior |
| **Risk Assessment (RA)** | RA-5 | Vulnerability scanning |
| **System & Services (SA)** | SA-11 | Developer security testing |
| **System & Comm (SC)** | SC-7, SC-8, SC-12, SC-13, SC-17, SC-28 | Boundary protection, transmission security, key management, crypto protection, PKI, encryption at rest |
| **System & Info Integrity (SI)** | SI-7 | Software/firmware integrity |

### Classification Banners

Enable classification banners in config:

```toml
[security]
banner_enabled = true
classification = "CUI"  # UNCLASSIFIED, CUI, CONFIDENTIAL, SECRET, TOP SECRET
```

### Session Management

- Configurable session timeout (AC-12)
- Automatic session locking after inactivity
- Session persistence with audit trail
- Remote session termination capability

### Audit Logging

When enabled, rigrun logs:
- All queries and responses
- Tool invocations and results
- Session events (start, timeout, end)
- Security events (auth, access control)
- Configuration changes

### Offline Mode (SC-7)

For air-gapped environments with complete network isolation:

```bash
# Enable via CLI flag
rigrun --no-network

# Enable via environment variable
export RIGRUN_OFFLINE=1

# Enable via config
rigrun config set routing.offline_mode true
```

When offline mode is enabled:
- All cloud API access is blocked
- WebFetch and WebSearch tools are disabled
- Only local Ollama models can be used
- Network egress is restricted to localhost only

### Paranoid Mode

Block all cloud requests while allowing local network:

```bash
rigrun config set paranoid_mode true
# or
export RIGRUN_PARANOID=1
```

---

## Configuration

Configuration file: `~/.rigrun/config.toml`

### Sample Configuration

```toml
# rigrun configuration file

version = "1.0.0"
default_model = "qwen2.5-coder:14b"

[routing]
default_mode = "local"      # local, cloud, hybrid
max_tier = "opus"           # Maximum tier to use
paranoid_mode = false       # Block all cloud requests
offline_mode = false        # Air-gapped mode (blocks all external network)

[local]
ollama_url = "http://localhost:11434"
ollama_model = "qwen2.5-coder:14b"

[cloud]
openrouter_key = ""
default_model = "anthropic/claude-3.5-sonnet"

[security]
session_timeout_secs = 3600
audit_enabled = true
banner_enabled = false
classification = "UNCLASSIFIED"

[cache]
enabled = true
ttl_hours = 24
max_size = 10000
semantic_enabled = true
semantic_threshold = 0.92

[ui]
theme = "dark"              # dark, light, auto
show_cost = true
show_tokens = true
compact_mode = false
```

### Environment Variables

Override configuration with environment variables:

| Variable | Description |
|----------|-------------|
| `RIGRUN_MODEL` | Override default model |
| `RIGRUN_OPENROUTER_KEY` | Override OpenRouter API key |
| `RIGRUN_PARANOID` | Set to "1" or "true" for paranoid mode |
| `RIGRUN_OFFLINE` | Set to "1" or "true" for offline mode |
| `RIGRUN_OLLAMA_URL` | Override Ollama URL |
| `RIGRUN_MODE` | Override routing mode |
| `RIGRUN_MAX_TIER` | Override max tier |
| `RIGRUN_CLASSIFICATION` | Override classification |

---

## Architecture

```
rigrun
├── cmd/                    # CLI entry points
├── internal/
│   ├── cli/               # CLI command handlers (50+ commands)
│   ├── config/            # Configuration management
│   ├── ollama/            # Ollama client
│   ├── cloud/             # OpenRouter client
│   ├── router/            # Query routing logic
│   ├── cache/             # Exact + semantic caching
│   ├── session/           # Session management
│   ├── security/          # NIST 800-53 control implementations
│   │   ├── auth.go        # IA-2 Authentication
│   │   ├── rbac.go        # AC-5/AC-6 Role-based access
│   │   ├── lockout.go     # AC-7 Account lockout
│   │   ├── audit.go       # AU-2/AU-3 Audit logging
│   │   ├── auditprotect.go # AU-9 Audit protection
│   │   ├── auditreview.go # AU-6 Audit review
│   │   ├── encrypt.go     # SC-28 Encryption at rest
│   │   ├── keystore.go    # SC-12 Key management
│   │   ├── crypto.go      # SC-13 Cryptographic protection
│   │   ├── pki.go         # SC-17 PKI certificates
│   │   ├── boundary.go    # SC-7 Boundary protection
│   │   ├── transport.go   # SC-8 Transmission security
│   │   ├── integrity.go   # SI-7 Integrity verification
│   │   ├── backup.go      # CP-9/CP-10 Backup/recovery
│   │   ├── vulnscan.go    # RA-5 Vulnerability scanning
│   │   ├── sectest.go     # SA-11 Security testing
│   │   ├── training.go    # AT-2/AT-3 Training
│   │   ├── agreements.go  # PS-6 Access agreements
│   │   ├── configmgmt.go  # CM-5/CM-6 Config management
│   │   ├── maintenance.go # MA-4/MA-5 Maintenance
│   │   ├── conmon.go      # CA-7 Continuous monitoring
│   │   ├── incident.go    # IR-* Incident response
│   │   └── sanitize.go    # MP-6 Media sanitization
│   ├── offline/           # Air-gapped mode controls
│   ├── tools/             # Agentic tool system
│   │   ├── read.go        # File reading
│   │   ├── glob.go        # File pattern matching
│   │   ├── grep.go        # Content search
│   │   ├── write.go       # File writing
│   │   ├── edit.go        # File editing
│   │   ├── bash.go        # Command execution
│   │   ├── web.go         # WebFetch with SSRF protection
│   │   ├── duckduckgo.go  # WebSearch via DuckDuckGo
│   │   ├── executor.go    # Tool execution engine
│   │   ├── agentic.go     # Agentic loop management
│   │   └── ollama.go      # Tool-to-Ollama conversion
│   ├── context/           # @ mention parsing
│   ├── commands/          # Slash command system
│   ├── model/             # Data models
│   ├── detect/            # GPU detection
│   └── ui/
│       ├── chat/          # Chat view (16+ slash commands)
│       ├── components/    # UI components
│       └── styles/        # Theming
└── pkg/
    ├── markdown/          # Markdown rendering
    └── syntax/            # Syntax highlighting
```

---

## Development

### Building

```bash
# Build
go build -o rigrun.exe .

# Build with version info
go build -ldflags "-X main.Version=1.0.0 -X main.GitCommit=$(git rev-parse HEAD)" -o rigrun.exe .

# Run tests
go test ./...

# Run with race detector
go run -race . chat
```

### Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Styling
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [BurntSushi/toml](https://github.com/BurntSushi/toml) - TOML parsing

---

## Troubleshooting

### Ollama not running

```bash
# Check if Ollama is running
ollama list

# Start Ollama service
ollama serve
```

### Model not found

```bash
# Pull the model
ollama pull qwen2.5-coder:14b

# Or use a different model
rigrun config set default_model llama3.2:8b
```

### Tools not working

```bash
# Check tool status
rigrun --help tools

# In TUI
/tools list
```

### Run diagnostics

```bash
rigrun doctor
```

---

## License

MIT License - Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge

## Acknowledgments

- [Ollama](https://ollama.ai) - Local LLM runtime
- [OpenRouter](https://openrouter.ai) - Cloud LLM gateway
- [Charm](https://charm.sh) - Beautiful TUI libraries
- [NIST](https://csrc.nist.gov/publications/detail/sp/800-53/rev-5/final) - SP 800-53 Rev 5 Security Controls
