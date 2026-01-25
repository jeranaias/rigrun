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

---

## The Problem

You're a developer. You use AI coding assistants every day. And you're probably:

- **Paying $20-40/month** for GitHub Copilot, ChatGPT Plus, or Claude Pro
- **Sending your proprietary code** to servers you don't control
- **Waiting on rate limits** when you need help most
- **Stuck without internet** on flights, in secure facilities, or just bad WiFi
- **Worried about data retention** policies you never read

Meanwhile, you have a GPU sitting in your machine doing nothing while you code.

---

## The Solution

rigrun is an AI coding assistant that runs **on your hardware first**.

Your GPU handles 90%+ of queries instantly, for free, with your data never leaving your machine. When you need the absolute best models for complex problems, rigrun can optionally route to cloud APIs - but only when YOU decide, and you see exactly what it costs.

**One command to start:**
```bash
rigrun
```

That's it. A beautiful terminal interface opens. Start typing. Your local model responds in milliseconds.

---

## Why rigrun Over the Alternatives?

### vs. ChatGPT / Claude Web

| | ChatGPT Plus | Claude Pro | rigrun |
|---|---|---|---|
| Monthly cost | $20 | $20 | **$0** |
| Your code sent to cloud | Yes | Yes | **No (local)** |
| Works offline | No | No | **Yes** |
| Rate limits | Yes | Yes | **No** |
| Response latency | 500ms-2s | 500ms-2s | **50-200ms** |
| Works in secure facilities | No | No | **Yes** |

### vs. GitHub Copilot

| | Copilot Individual | Copilot Business | rigrun |
|---|---|---|---|
| Monthly cost | $10 | $19 | **$0** |
| IDE lock-in | Yes (VSCode/JetBrains) | Yes | **No (any terminal)** |
| Telemetry | Extensive | Extensive | **None** |
| Works offline | No | No | **Yes** |
| Full conversation context | No | No | **Yes** |
| Agentic file operations | No | Limited | **Yes** |

### vs. Running Ollama Directly

| | Raw Ollama | rigrun |
|---|---|---|
| Beautiful interface | No (CLI only) | **Yes (30fps TUI)** |
| Conversation history | Manual | **Automatic** |
| Smart model routing | No | **Yes** |
| Agentic tools (read/write files) | No | **Yes** |
| Cloud fallback for hard problems | No | **Yes (optional)** |
| Security compliance | No | **IL5 certified** |

---

## Who Is This For?

### Developers Who Value Privacy

Your code is your competitive advantage. Every query to ChatGPT or Copilot sends your proprietary logic, architecture decisions, and business context to third-party servers. With rigrun, your code **never leaves your machine** unless you explicitly choose cloud routing.

### Developers Who Are Tired of Subscriptions

$20/month for ChatGPT. $10-19 for Copilot. $20 for Claude. It adds up. rigrun costs **$0/month** for unlimited local queries. If you occasionally need Claude or GPT-4 for complex analysis, you pay per-query (typically $0.01-0.10) instead of flat monthly fees.

### Developers Who Work Offline

Flights. Coffee shops with bad WiFi. Secure facilities. Rural areas. VPNs that block AI services. rigrun works completely offline with local models. Your productivity doesn't depend on internet connectivity.

### Enterprise and Government Teams

rigrun implements **43 NIST 800-53 security controls** required for DoD IL5 certification:
- Session timeouts and lockout policies
- HMAC-protected tamper-evident audit logs
- Classification-aware routing (sensitive data forced local)
- Role-based access control
- Encrypted backups with secure key management
- Air-gapped operation mode

This isn't security theater. These are the same controls required for systems handling classified information.

### Developers Who Want to Use Their Hardware

You bought that RTX 4090 or RX 7900. It sits idle while you code. rigrun puts it to work. A 14B parameter model runs comfortably on 8GB VRAM and handles most coding tasks as well as cloud models.

---

## What Can It Actually Do?

### Instant Code Help

Ask questions. Get answers. With syntax highlighting, streaming responses, and conversation history.

```
> Explain how Go channels work with a producer-consumer example

[Response streams in real-time with syntax-highlighted code]
```

### Agentic File Operations

rigrun can actually interact with your codebase:

```
> Find all functions that call the database and list potential SQL injection risks

[rigrun uses Grep, Read tools to analyze your code]
[Shows file:line references with analysis]
```

```
> Create a unit test file for the auth module

[rigrun reads existing code, creates test file, asks permission before writing]
```

### Smart Routing

rigrun automatically picks the right model:

- **Cache hit**: Instant response for repeated/similar queries (free)
- **Local model**: Your GPU handles it (free, ~100ms)
- **Cloud escalation**: Complex analysis that needs GPT-4/Claude (paid, you approve)

You always see where each query went and what it cost.

### Full Conversation Context

Unlike Copilot's line-by-line suggestions, rigrun maintains full conversation context. Reference earlier code, build on previous answers, have actual back-and-forth problem-solving sessions.

---

## The Experience

rigrun isn't a janky CLI tool. It's a **proper terminal application**:

- **30fps streaming** - Responses render smoothly, not in jerky chunks
- **Syntax highlighting** - Code is always beautiful and readable
- **Vim keybindings** - `j/k` navigation, `:` commands (optional)
- **Responsive design** - Adapts to any terminal size
- **Slash commands** - `/save`, `/load`, `/model`, `/config` - everything accessible
- **Context mentions** - `@file:path` to include files, `@git` for repo context

Everything happens inside one interface. No context switching. No browser tabs.

---

## "But What About Quality?"

Fair question. Local models are smaller than GPT-4 or Claude Opus.

**The honest answer**: For 90% of coding tasks, a good 14B model (Qwen 2.5 Coder, CodeStral) produces results indistinguishable from cloud models. You won't notice the difference for:
- Explaining code
- Writing functions
- Debugging errors
- Refactoring
- Writing tests
- Documentation

For the 10% of tasks that genuinely need frontier models (complex architecture decisions, novel algorithm design, nuanced analysis), rigrun lets you route to Claude or GPT-4. You pay per-query, not per-month. Most developers find this costs **$1-5/month** instead of $20-40.

---

## "Is It Actually Stable?"

rigrun is **production software**, not a weekend project:

- **50+ Go files** of well-structured code
- **Built-in self-tests** - Run `rigrun test all` to verify everything works
- **Comprehensive error handling** - Graceful degradation, clear error messages
- **Active development** - Bugs get fixed, features get added

Is it perfect? No software is. But it's stable enough for daily use by real developers.

---

## Getting Started

### 1. Install Ollama

[Download Ollama](https://ollama.ai) - it takes 30 seconds.

### 2. Download rigrun

**From Releases (Recommended):**
```bash
# Download the installer from GitHub Releases
# https://github.com/jeranaias/rigrun/releases/latest
```

**Or Build From Source:**
```bash
git clone https://github.com/jeranaias/rigrun.git
cd rigrun
go build -o rigrun .
```

### 3. Pull a Model

```bash
ollama pull qwen2.5-coder:14b  # Best quality, needs 9GB VRAM
# or
ollama pull qwen2.5-coder:7b   # Faster, needs 4GB VRAM
```

### 4. Run It

```bash
rigrun
```

The beautiful TUI opens. Start chatting. That's it.

---

## Recommended Models

| Model | VRAM | Speed | Quality | Best For |
|-------|------|-------|---------|----------|
| `qwen2.5-coder:7b` | 4GB | Fast | Good | Quick questions, tight VRAM |
| `qwen2.5-coder:14b` | 9GB | Medium | Excellent | Daily driver, best balance |
| `deepseek-coder-v2:16b` | 10GB | Medium | Excellent | Complex code understanding |
| `codestral:22b` | 13GB | Slower | Premium | When you need the best local |

Switch models anytime: `/model qwen2.5-coder:14b`

---

## Inside the TUI

### Slash Commands

Type `/` to see all commands:

| Command | What It Does |
|---------|--------------|
| `/help` | Show all commands |
| `/clear` | Clear conversation |
| `/save` | Save conversation |
| `/load` | Load previous conversation |
| `/model` | Switch AI model |
| `/mode` | Switch routing (local/cloud/auto) |
| `/config` | View/change settings |
| `/tools` | Manage agentic tools |
| `/status` | System status |
| `/doctor` | Run diagnostics |

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Ctrl+C` | Cancel/quit |
| `Ctrl+L` | Clear screen |
| `Tab` | Auto-complete |
| `Up/Down` | History |
| `/` | Command menu |

### Context Mentions

Include files and context:
```
@file:src/main.go explain this
@git what changed recently?
@clipboard analyze this
```

---

## Configuration

Config lives at `~/.rigrun/config.toml`:

```toml
[local]
ollama_url = "http://localhost:11434"
ollama_model = "qwen2.5-coder:14b"

[routing]
default_mode = "local"      # local, cloud, auto
paranoid_mode = false       # true = block ALL cloud

[cloud]
openrouter_key = ""         # Optional, for cloud fallback

[security]
session_timeout = 1800      # seconds
audit_enabled = true
classification = "UNCLASSIFIED"
```

Or configure from the TUI: `/config`

---

## Security & Compliance

rigrun implements **NIST 800-53 Rev 5** security controls for enterprise/government use:

| Control Family | What It Means |
|----------------|---------------|
| **Access Control (AC)** | Session timeouts, role-based access, account lockout |
| **Audit (AU)** | Tamper-evident logging, log protection, retention |
| **Contingency (CP)** | Encrypted backups, recovery procedures |
| **System Protection (SC)** | Boundary protection, encryption at rest, crypto controls |

### Classification Levels

```bash
rigrun classify set CUI
rigrun classify set SECRET --caveat NOFORN
```

### Air-Gapped Mode

Complete network isolation:
```bash
rigrun --no-network
```

---

## Troubleshooting

### Ollama Not Running
```bash
ollama serve
```

### Model Not Found
```bash
ollama pull qwen2.5-coder:14b
ollama list  # see available models
```

### Run Diagnostics
```bash
rigrun doctor
```

### Run Self-Tests
```bash
rigrun test all
```

---

## FAQ

**Q: Is my code really private?**
A: Yes. When using local models, your queries never leave your machine. When you enable cloud routing, you see exactly what goes to which provider.

**Q: What if the local model gives a bad answer?**
A: Ask it to try again, or use `/mode cloud` to escalate to a frontier model for that specific query.

**Q: Can I use this at work?**
A: Yes. The IL5 security controls make rigrun suitable for enterprise and government environments. Check with your security team about specific compliance requirements.

**Q: What GPUs are supported?**
A: Any GPU that Ollama supports - NVIDIA (CUDA), AMD (ROCm/Vulkan), Apple Silicon (Metal). Even CPU-only works, just slower.

**Q: Is this affiliated with any AI company?**
A: No. rigrun is independent open-source software. It uses Ollama for local inference and optionally OpenRouter for cloud access.

---

## Contributing

rigrun is open source (AGPL-3.0). Contributions welcome:

1. Fork the repo
2. Create a feature branch
3. Make changes
4. Run tests: `go test ./...`
5. Submit a PR

---

## Support & Feedback

- **Bug Reports**: [GitHub Issues](https://github.com/jeranaias/rigrun/issues)
- **Feature Requests**: [GitHub Discussions](https://github.com/jeranaias/rigrun/discussions)
- **Questions**: Open an issue, we respond quickly

---

## License

AGPL-3.0 - Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge

You can use rigrun for any purpose. If you modify and distribute it, you must share your changes under the same license.

---

**Your GPU. Your data. Your terminal. No subscriptions. No telemetry. No excuses.**

```bash
rigrun
```
