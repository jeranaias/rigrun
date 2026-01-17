# Getting Started with rigrun

The classification-aware LLM router for secure environments. One guide, everything you need.

---

## Quick Start (5 minutes)

### 1. Install Ollama

**macOS:**
```bash
brew install ollama
```

**Linux:**
```bash
curl -fsSL https://ollama.com/install.sh | sh
```

**Windows:**
Download from https://ollama.com/download

### 2. Install rigrun

**Pre-built binary (recommended):**
```bash
# Download from https://github.com/rigrun/rigrun/releases
# Extract and add to PATH
```

**Or via Cargo:**
```bash
cargo install rigrun
```

### 3. Run Setup

```bash
rigrun
```

On first run, rigrun will:
- Detect your GPU
- Recommend the best model for your hardware
- Download it automatically
- Start the server

### 4. Use It

```bash
# Interactive chat
rigrun chat

# Quick question
rigrun ask "What is recursion?"

# API endpoint
curl http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Hello!"}]}'
```

**That's it!** For details, read on.

---

## Detailed Setup

### Prerequisites

Run these commands to verify your system is ready:

| Requirement | Verify Command | Minimum |
|-------------|----------------|---------|
| **Ollama** | `ollama --version` | Any recent version |
| **Disk Space** | 10GB free | For models |
| **RAM** | 8GB (16GB recommended) | For inference |
| **GPU** (optional) | See GPU section | 6GB+ VRAM |

### Installation by Platform

#### Windows

**Step 1: Install Ollama**
1. Download installer from https://ollama.com/download
2. Run `OllamaSetup.exe`
3. Ollama starts automatically as a Windows service

**Verify:**
```powershell
ollama --version
ollama list
```

**Step 2: Install rigrun**

Option A - Pre-built binary:
1. Download `rigrun-x86_64-pc-windows-msvc.zip` from [releases](https://github.com/rigrun/rigrun/releases)
2. Extract to `C:\Program Files\rigrun\`
3. Add to PATH: System Properties > Environment Variables > Edit Path > Add `C:\Program Files\rigrun\`
4. Restart terminal

Option B - Cargo:
```powershell
cargo install rigrun
```

**Verify:**
```powershell
rigrun --version
```

#### macOS

**Step 1: Install Ollama**
```bash
brew install ollama
# Or download from https://ollama.com/download
```

Ollama runs automatically after installation.

**Step 2: Install rigrun**

Option A - Pre-built binary:
```bash
# Download appropriate file:
# Intel Mac: rigrun-x86_64-apple-darwin.tar.gz
# Apple Silicon: rigrun-aarch64-apple-darwin.tar.gz

tar -xzf rigrun-*.tar.gz
sudo mv rigrun /usr/local/bin/
```

Option B - Cargo:
```bash
cargo install rigrun
```

#### Linux

**Step 1: Install Ollama**
```bash
curl -fsSL https://ollama.com/install.sh | sh
sudo systemctl enable ollama
sudo systemctl start ollama
```

**Verify:**
```bash
systemctl status ollama
curl http://localhost:11434/api/tags  # Should return JSON
```

**Step 2: Install rigrun**

Option A - Pre-built binary:
```bash
# Download:
# x86_64: rigrun-x86_64-unknown-linux-gnu.tar.gz
# ARM64: rigrun-aarch64-unknown-linux-gnu.tar.gz

tar -xzf rigrun-*.tar.gz
sudo mv rigrun /usr/local/bin/
sudo chmod +x /usr/local/bin/rigrun
```

Option B - Cargo:
```bash
cargo install rigrun
```

### First Run

Run rigrun for the first time:

```bash
rigrun
```

**What happens:**

1. **GPU Detection** - rigrun identifies your GPU (NVIDIA, AMD, Apple Silicon, Intel Arc, or CPU-only)

2. **Model Recommendation** - Based on your VRAM, rigrun suggests the optimal model:
   | VRAM | Recommended Model |
   |------|-------------------|
   | <6GB | `qwen2.5-coder:3b` |
   | 6-8GB | `qwen2.5-coder:7b` |
   | 9-16GB | `qwen2.5-coder:14b` |
   | 17GB+ | `deepseek-coder-v2:16b` |

3. **Model Download** - One-time download (takes 5-15 minutes depending on model size and internet speed)

4. **Server Start** - API server starts at http://localhost:8787

**Example first-run output:**
```
+==============================================================+
|                    Welcome to rigrun                         |
|            Local-First LLM Router for IL5 Compliance         |
+==============================================================+

[+] GPU: NVIDIA RTX 4070 (12GB)
[+] Recommended model: qwen2.5-coder:14b

[...] Downloading qwen2.5-coder:14b (8 GB)...
[########################################] 100%

[+] Model ready
[+] Server: http://localhost:8787

Ready!
```

### Configuration

Configuration is stored at:
- **Unix/macOS:** `~/.rigrun/config.json`
- **Windows:** `C:\Users\<USERNAME>\.rigrun\config.json`

**View configuration:**
```bash
rigrun config show
```

**Common settings:**
```bash
# Change default model
rigrun config set-model qwen2.5-coder:7b

# Change port (default: 8787)
rigrun config set-port 8080

# Add cloud fallback (optional)
rigrun config set-key sk-or-v1-xxxxx  # OpenRouter key
```

**Configuration file format:**
```json
{
  "openrouter_key": null,
  "model": "qwen2.5-coder:14b",
  "port": 8787,
  "first_run_complete": true
}
```

---

## GPU Setup

### NVIDIA (Works out of box)

NVIDIA GPUs with CUDA work automatically. Verify setup:

```bash
nvidia-smi
```

If CUDA isn't working:
```bash
# Ubuntu/Debian
sudo apt update
sudo apt install nvidia-driver-535 nvidia-cuda-toolkit
sudo reboot

# Verify
nvidia-smi
nvcc --version
```

### AMD RDNA 4 (RX 9070, RX 9070 XT) - Vulkan

RDNA 4 GPUs require the Vulkan backend:

**Windows:**
```batch
set OLLAMA_VULKAN=1
ollama serve
```

Or make it permanent by running `scripts\set_ollama_vulkan_permanent.bat` as Administrator.

**Linux:**
```bash
export OLLAMA_VULKAN=1
ollama serve

# Make permanent
echo 'export OLLAMA_VULKAN=1' >> ~/.bashrc
```

**Performance on RX 9070 XT (16GB):**
| Model | Response Time |
|-------|---------------|
| 3B | ~2-3 seconds |
| 14B | ~10-15 seconds |
| 22B | ~15-20 seconds |

### AMD RDNA 2/3 (RX 6000/7000 series)

These GPUs use ROCm on Linux. On Windows, use Vulkan backend.

**Linux:**
```bash
# Install ROCm
wget https://repo.radeon.com/amdgpu-install/latest/ubuntu/jammy/amdgpu-install_*.deb
sudo apt install ./amdgpu-install_*.deb
sudo amdgpu-install --usecase=rocm
sudo usermod -a -G video,render $USER
sudo reboot

# Verify
rocm-smi
```

**If model runs slowly:**
```bash
# Set HSA override
export HSA_OVERRIDE_GFX_VERSION=11.0.0
echo 'export HSA_OVERRIDE_GFX_VERSION=11.0.0' >> ~/.bashrc
```

### Apple Silicon (Works out of box)

M1/M2/M3/M4 Macs use Metal acceleration automatically. No configuration needed.

**Verify:**
```bash
system_profiler SPDisplaysDataType | grep "Chipset Model"
```

### CPU Only

rigrun works without a GPU, just slower. To force CPU mode:
```bash
rigrun setup --hardware cpu
```

**Tips for CPU-only:**
- Use smaller models (`qwen2.5-coder:3b`)
- Expect 10-30x slower inference than GPU
- 16GB+ RAM recommended

---

## Common Issues

### "Ollama not found"

**Symptoms:**
```
[X] Failed to connect to Ollama
Cannot connect to Ollama at http://localhost:11434
```

**Solutions:**

1. **Check if Ollama is installed:**
   ```bash
   ollama --version
   ```

2. **Start Ollama service:**
   ```bash
   # macOS/Linux
   ollama serve

   # Linux (systemd)
   sudo systemctl start ollama

   # Windows (PowerShell as Admin)
   Start-Service Ollama
   ```

3. **Verify it's running:**
   ```bash
   curl http://localhost:11434/api/tags
   ```

### "Model too large for VRAM"

**Symptoms:**
```
[X] Request failed: out of memory
```

**Solutions:**

1. **Use smaller model:**
   ```bash
   rigrun config set-model qwen2.5-coder:3b
   ollama pull qwen2.5-coder:3b
   ```

2. **Close other GPU applications** (games, video editors, other ML workloads)

3. **Check GPU memory usage:**
   ```bash
   # NVIDIA
   nvidia-smi

   # AMD
   rocm-smi
   ```

4. **Model size reference:**
   | Model | VRAM Required |
   |-------|---------------|
   | 3B | ~4GB |
   | 7B | ~6GB |
   | 14B | ~10GB |
   | 22B | ~16GB |
   | 32B | ~20GB |

### "Connection refused"

**Symptoms:**
```
curl: (7) Failed to connect to localhost port 8787: Connection refused
```

**Solutions:**

1. **Check if rigrun is running:**
   ```bash
   ps aux | grep rigrun  # Unix
   Get-Process rigrun    # Windows
   ```

2. **Start rigrun:**
   ```bash
   rigrun
   ```

3. **Check configured port:**
   ```bash
   rigrun config show
   ```

4. **Check for port conflicts:**
   ```bash
   # Unix
   lsof -i :8787

   # Windows
   netstat -ano | findstr :8787
   ```

5. **Use different port:**
   ```bash
   rigrun config set-port 8080
   rigrun
   ```

### GPU Not Detected

**Run diagnostics:**
```bash
rigrun setup gpu
```

**For NVIDIA:**
```bash
nvidia-smi
# Should show GPU name and driver version
```

**For AMD (RDNA 4):**
```bash
# Make sure Vulkan is enabled
export OLLAMA_VULKAN=1
ollama serve
```

**For AMD (RDNA 2/3 on Linux):**
```bash
rocm-smi
# If empty, check ROCm installation
```

---

## Next Steps

### CLI Reference

Full command reference:
```bash
rigrun                    # Start server
rigrun chat               # Interactive chat
rigrun ask "question"     # Quick query
rigrun status             # Show stats and GPU info
rigrun models             # List available models
rigrun pull <model>       # Download a model
rigrun config show        # View configuration
rigrun doctor             # Diagnose issues
```

See [CLI_REFERENCE.md](../CLI_REFERENCE.md) for complete documentation.

### API Documentation

rigrun exposes an OpenAI-compatible API:

```bash
# Health check
curl http://localhost:8787/health

# Chat completion
curl http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# List models
curl http://localhost:8787/v1/models

# Stats
curl http://localhost:8787/stats
```

**Python example:**
```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8787/v1",
    api_key="unused"
)

response = client.chat.completions.create(
    model="auto",
    messages=[{"role": "user", "content": "Explain recursion"}]
)
print(response.choices[0].message.content)
```

See [api-reference.md](api-reference.md) for complete API documentation.

### IDE Integration

Set up rigrun as your coding assistant backend:

```bash
rigrun setup ide
```

Supports:
- VS Code
- Cursor
- JetBrains (IntelliJ, PyCharm, WebStorm)
- Neovim

### Contributing

We welcome contributions!

```bash
git clone https://github.com/rigrun/rigrun
cd rigrun
cargo build
cargo test
```

See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

---

## Getting Help

- **Diagnose issues:** `rigrun doctor`
- **GitHub Issues:** https://github.com/rigrun/rigrun/issues
- **Discussions:** https://github.com/rigrun/rigrun/discussions

---

*This guide consolidates all rigrun documentation into one place. For the most up-to-date information, check the [GitHub repository](https://github.com/rigrun/rigrun).*
