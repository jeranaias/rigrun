# rigrun CLI Reference

Quick reference for all rigrun commands.

## Quick Start

```bash
rigrun                          # Start server
rigrun ask "your question"      # Quick query
rigrun chat                     # Interactive mode
rigrun s                        # Status (shortcut)
```

---

## All Commands

### Server Operations

#### Start Server (default)
```bash
rigrun                          # Start on default port (8787)
rigrun --paranoid               # Block ALL cloud requests
```

### Query Operations

#### Ask (Quick Query)
```bash
rigrun ask "What is Rust?"
rigrun ask "Explain closures" --model qwen2.5-coder:7b
```

#### Chat (Interactive)
```bash
rigrun chat
rigrun chat --model qwen2.5-coder:14b
```

Direct prompt also works:
```bash
rigrun "your question here"
echo "question" | rigrun
```

### Status & Diagnostics

#### Status (alias: `s`)
```bash
rigrun status                   # Full status
rigrun s                        # Shortcut
```

Shows:
- Server running status
- GPU info and VRAM usage
- Model configuration
- Today's query stats
- Money saved

#### Doctor
```bash
rigrun doctor
```

Checks:
- Ollama installation
- Ollama service status
- Config validity
- Port availability

### Configuration

#### Show Config
```bash
rigrun config show
rigrun config                   # Default: show
```

#### Set OpenRouter Key
```bash
rigrun config set-key sk-or-v1-xxx
```

#### Set Model
```bash
rigrun config set-model qwen2.5-coder:7b
```

#### Set Port
```bash
rigrun config set-port 8080
```

### Setup Operations

#### IDE Integration
```bash
rigrun setup ide
```

Detects and configures:
- VS Code
- Cursor
- JetBrains IDEs
- Neovim

#### GPU Setup
```bash
rigrun setup gpu
```

Shows:
- GPU detection
- VRAM usage
- Driver status
- Setup guidance

### Cache Operations

#### Cache Stats
```bash
rigrun cache stats
```

Shows:
- Number of cached entries
- Cache size on disk
- Cache location

#### Clear Cache
```bash
rigrun cache clear
```

Removes all cached queries.

#### Export Cache
```bash
rigrun cache export
rigrun cache export --output ./backups
```

Exports:
- Cache data
- Audit log
- Statistics

### Model Management

#### List Models (alias: `m`)
```bash
rigrun models                   # Full list
rigrun m                        # Shortcut
```

Shows:
- Available models
- Downloaded models (✓)
- Recommended for your GPU

#### Pull Model
```bash
rigrun pull qwen2.5-coder:7b
rigrun pull deepseek-coder-v2:16b
```

Downloads a model from Ollama.

---

## Global Flags

```bash
--paranoid                      # Block all cloud requests
--help                          # Show help
--version                       # Show version
```

---

## Legacy Commands (Hidden)

These still work but are hidden from help:

```bash
rigrun examples                 # CLI examples
rigrun background               # Run as background process
rigrun stop                     # Stop background server
rigrun ide-setup                # → rigrun setup ide
rigrun gpu-setup                # → rigrun setup gpu
rigrun export                   # → rigrun cache export
```

---

## Command Hierarchy

```
rigrun
├── (default)           Start server
├── ask <question>      Quick query
├── chat                Interactive mode
├── status (s)          Show stats
├── config              Configuration
│   ├── show
│   ├── set-key
│   ├── set-model
│   └── set-port
├── setup               Setup operations
│   ├── ide
│   └── gpu
├── cache               Cache operations
│   ├── stats
│   ├── clear
│   └── export
├── doctor              Diagnose issues
├── models (m)          List models
└── pull <model>        Download model
```

---

## Common Workflows

### First Time Setup
```bash
rigrun                          # Runs first-time wizard
# Follow prompts to configure
```

### Daily Usage
```bash
rigrun                          # Start server (once)
rigrun s                        # Check status
rigrun ask "question"           # Quick queries
rigrun chat                     # Long conversations
```

### Troubleshooting
```bash
rigrun doctor                   # Run diagnostics
rigrun status                   # Check GPU/model
rigrun setup gpu                # GPU setup help
```

### Configuration
```bash
rigrun config show              # View current config
rigrun config set-model qwen2.5-coder:14b
rigrun config set-key sk-or-xxx
```

### Maintenance
```bash
rigrun cache stats              # Check cache size
rigrun cache clear              # Clear if needed
rigrun cache export             # Backup data
```

---

## Tips

1. **Use aliases**: `rigrun s` instead of `rigrun status`
2. **Direct prompts**: `rigrun "question"` for quick one-offs
3. **Pipe input**: `echo "question" | rigrun`
4. **Check help**: Add `--help` to any command
5. **Paranoid mode**: Use `--paranoid` for local-only operation

---

## Examples

### Development Workflow
```bash
# Start server once
rigrun

# In another terminal, quick queries
rigrun ask "how do I handle errors in rust?"
rigrun ask "explain async/await"

# Or interactive mode
rigrun chat
```

### System Maintenance
```bash
# Check everything is working
rigrun doctor

# View configuration
rigrun config show

# Check cache size
rigrun cache stats

# Export data
rigrun cache export --output ~/backups
```

### Model Management
```bash
# See what's available
rigrun models

# Download a model
rigrun pull qwen2.5-coder:14b

# Configure as default
rigrun config set-model qwen2.5-coder:14b

# Verify
rigrun status
```

---

## Getting Help

```bash
rigrun --help                   # Main help
rigrun <command> --help         # Command help
rigrun config --help            # Subcommand group help
rigrun config set-key --help    # Specific subcommand help
rigrun doctor                   # Diagnose issues
```

---

## API Usage

When server is running:

```bash
# Health check
curl http://localhost:8787/health

# Chat completion
curl http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "hi"}]
  }'

# List models
curl http://localhost:8787/v1/models

# Get stats
curl http://localhost:8787/stats
```

---

For full documentation, see: https://github.com/jeranaias/rigrun
