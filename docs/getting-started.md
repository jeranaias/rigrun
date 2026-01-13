# Getting Started with rigrun

Welcome to rigrun - a self-hosted LLM router that intelligently routes requests through semantic cache, local GPU inference (via Ollama), and cloud fallback (via OpenRouter). This guide will get you from zero to your first API call in under 10 minutes.

---

## What is rigrun?

**For Developers:**
rigrun is an OpenAI-compatible API router that reduces LLM costs by 90% through intelligent request routing. It's a drop-in replacement for OpenAI/Claude APIs that runs on your hardware.

**For Non-Technical Users:**
rigrun lets you run AI models on your own computer instead of paying cloud services. It's like having your own private ChatGPT that saves money and keeps your data private.

**How It Works:**
1. **Cache First**: Instant responses for similar questions (40-60% hit rate)
2. **Local GPU Second**: Free inference on your hardware (30-50% of requests)
3. **Cloud Fallback**: Only pay for complex queries that need cloud power (10% of requests)

**Result**: 90% cost reduction vs cloud-only solutions like OpenAI or Anthropic.

---

## Before You Begin

### System Requirements

**Minimum:**
- Operating System: Windows 10+, macOS 10.15+, or Linux
- RAM: 8GB (16GB recommended)
- Storage: 10GB free space for models
- Internet: Required for initial setup and cloud fallback

**GPU (Optional but Recommended):**
- NVIDIA GPU (6GB+ VRAM) with CUDA
- AMD GPU (6GB+ VRAM) with ROCm
- Apple Silicon (M1/M2/M3) with Metal
- Intel Arc GPU

**Note**: You can run rigrun on CPU-only, but GPU provides 10-20x faster inference.

### What You'll Install

1. **Ollama** - Local LLM runtime (handles GPU inference)
2. **rigrun** - Router that orchestrates cache, local, and cloud
3. **A Model** - e.g., qwen2.5-coder:7b (auto-downloaded)
4. **OpenRouter Key** (optional) - For cloud fallback only

---

## Quick Start (5 Minutes)

### Step 1: Install Ollama

Ollama is the local inference engine that runs models on your GPU.

**macOS:**
```bash
brew install ollama
# Or download from https://ollama.com/download
```

**Linux:**
```bash
curl -fsSL https://ollama.com/install.sh | sh
```

**Windows:**
Download installer from https://ollama.com/download

Verify installation:
```bash
ollama --version
```

### Step 2: Install rigrun

**Option A: Pre-built Binary (Recommended)**
1. Go to https://github.com/rigrun/rigrun/releases
2. Download the binary for your platform
3. Extract and move to your PATH:

```bash
# macOS/Linux
tar -xzf rigrun-*.tar.gz
sudo mv rigrun /usr/local/bin/

# Windows
# Extract to C:\Program Files\rigrun\
# Add to PATH via System Properties
```

**Option B: Install via Cargo**
```bash
# Requires Rust toolchain
cargo install rigrun
```

Verify installation:
```bash
rigrun --help
```

### Step 3: First Run

Start rigrun (this auto-configures everything):
```bash
rigrun
```

**What Happens:**
1. Detects your GPU (NVIDIA, AMD, Apple Silicon, or CPU)
2. Recommends optimal model based on VRAM
3. Downloads the model (one-time, may take 5-10 minutes)
4. Starts API server on http://localhost:8787

**Example Output:**
```
✓ GPU: NVIDIA RTX 3080 (10GB)
[↓] Downloading qwen2.5-coder:7b (4.2 GB)...
✓ Model ready
✓ Server: http://localhost:8787

Today: 0 queries | Saved: $0.00

Ready! Try:
  curl localhost:8787/v1/chat/completions -H "Content-Type: application/json" \
    -d '{"model":"auto","messages":[{"role":"user","content":"Hello!"}]}'
```

### Step 4: Make Your First Request

**Using cURL:**
```bash
curl http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Explain what rigrun does in one sentence."}]
  }'
```

**Using Python:**
```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8787/v1",
    api_key="unused"  # Local - no auth needed
)

response = client.chat.completions.create(
    model="auto",
    messages=[{"role": "user", "content": "Say hello!"}]
)

print(response.choices[0].message.content)
```

**Expected Response:**
```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1700000000,
  "model": "auto",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "rigrun is a local-first LLM router that reduces AI costs..."
    },
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 15,
    "completion_tokens": 25,
    "total_tokens": 40
  }
}
```

Congratulations! You just made your first local LLM request through rigrun.

---

## Understanding the Routing

rigrun uses a three-tier routing system:

### Tier 1: Semantic Cache (Fastest, Free)

The cache recognizes similar queries:
- "What is recursion?" = "Explain recursion to me"
- "How to reverse string?" = "Reverse a string in Python"

**Performance**: <10ms response time
**Cost**: $0

### Tier 2: Local GPU (Fast, Free)

Runs models on your GPU via Ollama:
- Good for code completion, Q&A, simple tasks
- Quality comparable to GPT-3.5

**Performance**: 1-3s response time
**Cost**: $0 (after GPU purchase)

### Tier 3: Cloud Fallback (Slower, Paid)

Routes to OpenRouter for complex queries:
- Advanced reasoning, long contexts
- Access to GPT-4, Claude Opus, etc.

**Performance**: 3-10s response time
**Cost**: Pay-per-use (typically $0.001-0.03 per request)

---

## Next Steps

### 1. Check Your System Status

```bash
rigrun status
```

Shows:
- Server status and port
- GPU info and VRAM usage
- Today's query stats and savings

### 2. View API Stats

```bash
curl http://localhost:8787/stats
curl http://localhost:8787/cache/stats
```

Monitor cache hit rates and routing decisions.

### 3. Optional: Add Cloud Fallback

For complex queries, configure OpenRouter:

1. Get API key: https://openrouter.ai/keys
2. Configure rigrun:
```bash
rigrun config --openrouter-key sk-or-v1-xxxxx
```
3. Restart rigrun

Now rigrun will automatically route complex queries to cloud when needed.

### 4. Configure Your IDE

Use rigrun as the backend for coding assistants:

```bash
rigrun ide-setup
```

Supports VS Code, Cursor, JetBrains, and Neovim.

### 5. Try Interactive Chat

```bash
rigrun chat
```

Quick terminal-based chat for testing.

---

## Common First-Run Issues

### Ollama Not Running

**Error**: "Failed to connect to Ollama"

**Fix**:
```bash
# Start Ollama manually
ollama serve

# Then in another terminal
rigrun
```

### Model Download Fails

**Error**: "Failed to download model"

**Causes**:
- No internet connection
- Insufficient disk space (need 5-10GB)
- Corporate firewall blocking download

**Fix**:
```bash
# Download manually
ollama pull qwen2.5-coder:7b

# Then start rigrun
rigrun
```

### Port Already in Use

**Error**: "Port 8787 is already in use"

**Fix**:
```bash
# Use a different port
rigrun config --port 8080

# Then start on new port
rigrun
```

Access at http://localhost:8080 instead.

### GPU Not Detected

**Error**: "No GPU detected (CPU mode)"

**Check**:
```bash
# NVIDIA
nvidia-smi

# AMD (Linux)
rocm-smi

# Apple Silicon
system_profiler SPDisplaysDataType
```

**Fix**:
1. Install GPU drivers (see [installation.md](installation.md))
2. Restart system
3. Run `rigrun gpu-setup` for diagnosis

---

## Understanding Cost Savings

### Example: Individual Developer

**Usage**: 10M tokens/month (about 1,000 queries/day)

**Without rigrun (100% OpenAI GPT-4):**
- Cost: $300/month
- Annual: $3,600

**With rigrun (90% local, 10% cloud):**
- Cache hits (60%): $0
- Local GPU (30%): $0
- Cloud (10%): $30/month
- Annual: $360

**Savings: $3,240/year (91% reduction)**

### ROI Calculation

A $1,500 gaming GPU pays for itself in 5 months vs all-cloud costs.

---

## Where to Go Next

- **[Installation Guide](installation.md)** - Detailed installation for all platforms
- **[Configuration Guide](configuration.md)** - All configuration options
- **[API Reference](api-reference.md)** - Complete API documentation
- **[Troubleshooting](troubleshooting.md)** - Solutions to common problems

---

## Getting Help

- **GitHub Issues**: https://github.com/rigrun/rigrun/issues
- **Discussions**: https://github.com/rigrun/rigrun/discussions
- **Documentation**: https://github.com/rigrun/rigrun/tree/main/docs

Welcome to the rigrun community!
