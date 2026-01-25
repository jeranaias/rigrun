# Installing rigrun

<p align="center">
  <img src="docs/logo.png" alt="rigrun" width="400">
</p>

<p align="center">
  <strong>The AI coding assistant that respects your terminal</strong>
</p>

<p align="center">
  <a href="#download">Download</a> •
  <a href="#quick-install">Quick Install</a> •
  <a href="#manual-install">Manual Install</a> •
  <a href="#requirements">Requirements</a> •
  <a href="#first-run">First Run</a>
</p>

---

## Download

**The easiest way to install rigrun is to download the installer:**

| Platform | Download |
|----------|----------|
| **Windows** | [rigrun-installer.exe](https://github.com/jeranaias/rigrun-tui/releases/latest/download/rigrun-installer.exe) |
| **macOS** | [rigrun-installer-darwin](https://github.com/jeranaias/rigrun-tui/releases/latest/download/rigrun-installer-darwin) |
| **Linux** | [rigrun-installer-linux](https://github.com/jeranaias/rigrun-tui/releases/latest/download/rigrun-installer-linux) |

The installer is a beautiful, guided experience that:
- ✦ Checks your system requirements
- ✦ Sets up Ollama (if needed)
- ✦ Downloads a recommended AI model
- ✦ Creates your configuration
- ✦ Offers to **Launch rigrun** immediately

After installation, choose "Launch rigrun now" to open a terminal and experience rigrun's interactive welcome tutorial.

---

## Quick Install (CLI)

### macOS / Linux

```bash
curl -fsSL https://rigrun.dev/install.sh | bash
```

Or with wget:

```bash
wget -qO- https://rigrun.dev/install.sh | bash
```

### Windows (PowerShell)

```powershell
iwr -useb https://rigrun.dev/install.ps1 | iex
```

### Homebrew (macOS)

```bash
brew tap jeranaias/rigrun
brew install rigrun
```

---

## What the Installer Does

The installer provides a guided setup experience:

1. **✓ Checks your system** - OS, Go, Ollama, network, disk space
2. **✓ Sets up Ollama** - Guides you through installation if needed
3. **✓ Downloads a model** - Choose from recommended AI models
4. **✓ Creates configuration** - Sensible defaults, ready to customize
5. **✓ Adds to PATH** - Run `rigrun` from anywhere

All in under 60 seconds.

---

## Manual Install

### From Source

```bash
# Clone the repository
git clone https://github.com/jeranaias/rigrun-tui.git
cd rigrun-tui

# Build
go build -o rigrun .

# Move to your PATH
mv rigrun ~/.local/bin/

# Create config directory
mkdir -p ~/.rigrun
```

### Pre-built Binaries

Download from [Releases](https://github.com/jeranaias/rigrun-tui/releases):

| Platform | Architecture | Download |
|----------|--------------|----------|
| macOS    | Apple Silicon | [rigrun-darwin-arm64.tar.gz](https://github.com/jeranaias/rigrun-tui/releases/latest/download/rigrun-darwin-arm64.tar.gz) |
| macOS    | Intel | [rigrun-darwin-amd64.tar.gz](https://github.com/jeranaias/rigrun-tui/releases/latest/download/rigrun-darwin-amd64.tar.gz) |
| Linux    | x86_64 | [rigrun-linux-amd64.tar.gz](https://github.com/jeranaias/rigrun-tui/releases/latest/download/rigrun-linux-amd64.tar.gz) |
| Linux    | ARM64 | [rigrun-linux-arm64.tar.gz](https://github.com/jeranaias/rigrun-tui/releases/latest/download/rigrun-linux-arm64.tar.gz) |
| Windows  | x86_64 | [rigrun-windows-amd64.zip](https://github.com/jeranaias/rigrun-tui/releases/latest/download/rigrun-windows-amd64.zip) |

---

## Requirements

### Required

- **Ollama** - Local LLM runtime
  - Install from [ollama.ai](https://ollama.ai)
  - Start with: `ollama serve`

### Recommended

- **Go 1.21+** - Only needed for building from source
- **Git** - For version control features
- **A modern terminal** - With Unicode and 256-color support

### Supported Platforms

| Platform | Status | Notes |
|----------|--------|-------|
| macOS 12+ | ✅ Full support | Apple Silicon recommended |
| Linux (glibc) | ✅ Full support | Ubuntu 20.04+, Debian 11+ |
| Linux (musl) | ✅ Full support | Alpine 3.14+ |
| Windows 10+ | ✅ Full support | Windows Terminal recommended |
| WSL2 | ✅ Full support | Same as Linux |

---

## First Run

After installation, start rigrun:

```bash
rigrun
```

You'll see an interactive tutorial that teaches you:

1. **💬 Ask Anything** - Natural language questions about code
2. **📁 File Context** - Use `@file:path` to include files
3. **⌨️ Command Palette** - Press `Ctrl+P` for fuzzy search
4. **❓ Help** - Type `/help` for all commands

### Quick Tips

```
Ctrl+P      Open command palette
@file:      Include file in context
/help       Show help
/save       Save conversation
/model      Switch models
```

---

## Configuration

After installation, your config is at:

- **macOS/Linux**: `~/.rigrun/config.toml`
- **Windows**: `%USERPROFILE%\.rigrun\config.toml`

### Essential Settings

```toml
[ollama]
url = "http://localhost:11434"
model = "qwen2.5-coder:7b"

[ui]
vim_mode = false      # Enable vim keybindings
show_costs = true     # Show token costs

[routing]
default_mode = "auto" # auto, local, or cloud
```

See [Configuration Guide](docs/configuration.md) for all options.

---

## Verify Installation

```bash
# Check version
rigrun --version

# Run diagnostics
rigrun doctor

# Test connection to Ollama
rigrun --check-ollama
```

---

## Troubleshooting

### "Command not found: rigrun"

Your PATH may not include the install directory. Add it:

```bash
# macOS/Linux
export PATH="$PATH:$HOME/.local/bin"

# Or add to ~/.bashrc or ~/.zshrc permanently
echo 'export PATH="$PATH:$HOME/.local/bin"' >> ~/.bashrc
```

### "Connection refused" to Ollama

Make sure Ollama is running:

```bash
ollama serve
```

### "Model not found"

Download a model first:

```bash
ollama pull qwen2.5-coder:7b
```

### Need more help?

- Run `rigrun doctor` for diagnostics
- Check [FAQ](docs/faq.md)
- Open an [issue](https://github.com/jeranaias/rigrun-tui/issues)

---

## Uninstall

### macOS/Linux

```bash
rm ~/.local/bin/rigrun
rm -rf ~/.rigrun
```

### Windows

```powershell
Remove-Item $env:USERPROFILE\.local\bin\rigrun.exe
Remove-Item -Recurse $env:USERPROFILE\.rigrun
```

### Homebrew

```bash
brew uninstall rigrun
```

---

## Next Steps

- 📖 [User Guide](docs/user-guide.md) - Complete documentation
- 🎯 [Tutorials](docs/tutorials/) - Step-by-step guides
- ⌨️ [Keyboard Shortcuts](docs/shortcuts.md) - All keybindings
- 🔒 [Security Guide](docs/security.md) - IL5/DoD compliance

---

<p align="center">
  <strong>Ready to code? Let's go!</strong>
</p>

```bash
rigrun
```
