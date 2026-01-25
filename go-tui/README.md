# rigrun

**Your GPU. Your data. Your beautiful terminal.**

A stunning AI coding assistant that lives in your terminal, runs on YOUR hardware, and looks incredible doing it.

```
    ██████╗ ██╗ ██████╗ ██████╗ ██╗   ██╗███╗   ██╗
    ██╔══██╗██║██╔════╝ ██╔══██╗██║   ██║████╗  ██║
    ██████╔╝██║██║  ███╗██████╔╝██║   ██║██╔██╗ ██║
    ██╔══██╗██║██║   ██║██╔══██╗██║   ██║██║╚██╗██║
    ██║  ██║██║╚██████╔╝██║  ██║╚██████╔╝██║ ╚████║
    ╚═╝  ╚═╝╚═╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝
```

---

## The Experience

Just type `rigrun` and you're in:

- **30fps streaming** - Buttery smooth response rendering
- **Syntax highlighting** - Code looks beautiful, always
- **Vim keybindings** - Your muscle memory just works
- **Responsive design** - Adapts to any terminal size
- **Dark/light themes** - Easy on the eyes, day or night
- **Progress indicators** - Always know what's happening

Everything you need is inside the TUI. No flags to memorize. No commands to look up.

---

## Get Started

### 1. Download & Run the Installer

The installer is also a beautiful TUI experience:

**Windows:**
```powershell
# Download from GitHub Releases
Invoke-WebRequest -Uri "https://github.com/jeranaias/rigrun/releases/latest/download/rigrun-installer-windows-amd64.exe" -OutFile rigrun-installer.exe
.\rigrun-installer.exe
```

**macOS/Linux:**
```bash
curl -LO https://github.com/jeranaias/rigrun/releases/latest/download/rigrun-installer
chmod +x rigrun-installer
./rigrun-installer
```

The installer walks you through everything:
1. System check (GPU, Ollama, disk space)
2. Ollama setup if needed
3. Model selection with recommendations
4. Configuration creation
5. Launch rigrun!

### 2. Or Build From Source

```bash
git clone https://github.com/jeranaias/rigrun.git
cd rigrun
go build -o rigrun .
ollama pull qwen2.5-coder:14b
./rigrun
```

### 3. Start Using It

```bash
rigrun
```

That's it. Everything else happens inside.

---

## Inside the TUI

### Slash Commands

Type `/` to access everything:

| Command | What It Does |
|---------|--------------|
| `/help` | Show all available commands |
| `/clear` | Fresh start, clear the chat |
| `/new` | Start a new conversation |
| `/save` | Save this conversation |
| `/load` | Load a previous conversation |
| `/export` | Export as JSON, Markdown, or text |
| `/model` | Switch AI models on the fly |
| `/config` | View and change settings |
| `/tools` | Manage agentic tools |
| `/status` | System status at a glance |
| `/audit` | View security audit log |
| `/security` | Security status summary |
| `/classify` | Set classification level |
| `/quit` | Exit gracefully |

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Send your message |
| `Ctrl+C` | Stop generation or quit |
| `Ctrl+L` | Clear the screen |
| `Ctrl+P` | Command palette |
| `Tab` | Auto-complete files and commands |
| `Up/Down` | Browse your history |
| `PageUp/Down` | Scroll through long responses |
| `/` | Open slash command menu |
| `Esc` | Cancel current action |

### Context Mentions

Pull in context with `@`:

```
@file:src/main.go explain this code
@clipboard paste and analyze
@git show recent changes
@codebase summarize the project
@error what went wrong?
```

---

## Agentic Mode

rigrun can DO things, not just talk about them. Inside the TUI, just ask:

```
Find all TODO comments in this project and summarize them
```

```
Create a Python script that fetches weather data
```

```
Search the web for Go 1.22 release notes
```

rigrun will use its tools:

| Tool | What It Does | Permission |
|------|--------------|------------|
| **Read** | Read any file | Automatic |
| **Glob** | Find files by pattern | Automatic |
| **Grep** | Search file contents | Automatic |
| **Write** | Create/update files | Asks first |
| **Edit** | Modify files | Asks first |
| **Bash** | Run commands | Asks first |
| **WebSearch** | Search the internet | Automatic |
| **WebFetch** | Fetch web pages | Automatic |

Manage tools with `/tools` in the TUI.

---

## Smart Routing

rigrun automatically picks the best model for each conversation:

| Your Query | Where It Runs | Cost |
|------------|---------------|------|
| Quick questions | Your GPU | Free |
| Code completion | Your GPU | Free |
| Repeated questions | Instant cache | Free |
| Complex analysis | Cloud (if enabled) | ~$0.01 |

You stay in control:
- `/mode local` - Force everything local
- `/mode cloud` - Allow cloud when needed
- `/config paranoid_mode true` - Block all cloud forever

---

## Security Built In

Perfect for enterprise and government. Set classification with `/classify`:

- `UNCLASSIFIED`
- `CUI` (Controlled Unclassified)
- `CONFIDENTIAL`
- `SECRET`
- `TOP SECRET`

With caveats: `/classify SECRET --caveat NOFORN`

### Air-Gapped Mode

Complete network isolation:

```bash
rigrun --no-network
```

Or set permanently in the TUI: `/config offline_mode true`

### Full IL5 Compliance

All NIST 800-53 controls implemented:
- Session timeouts (AC-12)
- Audit logging (AU-2, AU-3)
- Encrypted backups (CP-9)
- Account lockout (AC-7)
- Role-based access (AC-5, AC-6)
- And 40+ more controls

Access everything through the TUI with `/audit`, `/security`, `/backup`, etc.

---

## Configuration

Everything is configurable from inside the TUI with `/config`.

Config file lives at: `~/.rigrun/config.toml`

```toml
[local]
ollama_url = "http://localhost:11434"
ollama_model = "qwen2.5-coder:14b"

[routing]
default_mode = "local"
paranoid_mode = false

[cloud]
openrouter_key = ""  # Optional

[security]
session_timeout = 1800
audit_enabled = true
classification = "UNCLASSIFIED"

[ui]
theme = "dark"
show_cost = true
show_tokens = true
```

---

## Recommended Models

| Model | VRAM | Best For |
|-------|------|----------|
| `qwen2.5-coder:7b` | 4GB | Fast daily coding |
| `qwen2.5-coder:14b` | 9GB | Best local quality |
| `llama3.2:8b` | 5GB | General purpose |
| `codestral:22b` | 13GB | Premium analysis |

Switch models anytime with `/model qwen2.5-coder:14b`

---

## CLI Commands (For Scripting)

While the TUI is the main experience, CLI commands exist for automation:

```bash
# One-shot question
rigrun ask "What is a mutex?"

# With a file
rigrun ask "Review this:" --file main.go

# Agentic mode
rigrun ask --agentic "Find all bugs in this code"

# System commands
rigrun status
rigrun doctor
rigrun test all
```

Full CLI reference: `rigrun help`

---

## Troubleshooting

### Inside the TUI

- `/doctor` - Run diagnostics
- `/status` - Check system state
- `/tools list` - Verify tools are working

### From Terminal

```bash
rigrun doctor    # Full diagnostics
rigrun status    # Quick status check
rigrun test all  # Run self-tests
```

### Common Issues

**Ollama not running:**
```bash
ollama serve
```

**Model not found:**
```bash
ollama pull qwen2.5-coder:14b
```

---

## Feedback & Support

We're actively looking for testers!

- **Bug Reports**: [GitHub Issues](https://github.com/jeranaias/rigrun/issues)
- **Feature Requests**: [GitHub Discussions](https://github.com/jeranaias/rigrun/discussions)

---

## Development

```bash
# Build
go build -o rigrun .

# Build installer
go build -o rigrun-installer ./cmd/installer

# Test
go test ./...
```

---

## License

AGPL-3.0 - Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge

---

**Built with [Ollama](https://ollama.ai), [Charm](https://charm.sh), and love for the terminal.**
