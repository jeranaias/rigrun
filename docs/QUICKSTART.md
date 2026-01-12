# rigrun Quickstart Guide

rigrun lets you run a local AI endpoint powered by [Ollama](https://ollama.com) (for fast, GPU-accelerated models) with optional cloud fallback via [OpenRouter](https://openrouter.ai).

This guide walks you from zero to your first successful request.

---

## 1. Prerequisites

### 1.1 Install Rust (via `rustup`)

rigrun is written in Rust. To build or install it via `cargo`, you need:

- Rust toolchain (stable)
- `cargo` (included with `rustup`)

**Install on macOS / Linux / WSL:**

```bash
curl https://sh.rustup.rs -sSf | sh
# Follow the on-screen instructions, then restart your terminal
```

**Install on Windows:**

1. Download the Rust installer:
   https://www.rust-lang.org/tools/install
2. Run the installer and select the default options.
3. Open a new *Developer Command Prompt* or *PowerShell* so `cargo` is in your `PATH`.

Verify your installation:

```bash
rustc --version
cargo --version
```

You should see version numbers, not "command not found".

---

### 1.2 Install Ollama

rigrun talks to a *local* Ollama server to run models. Install Ollama first.

#### macOS

1. Go to: https://ollama.com/download
2. Download the macOS installer (`.dmg`).
3. Open it and drag **Ollama** into `Applications`.
4. Run Ollama once so it can start its background service.

Or via Homebrew:

```bash
brew install ollama
ollama serve  # optional, usually started automatically
```

#### Linux

Ollama provides a convenient install script:

```bash
curl -fsSL https://ollama.com/install.sh | sh
```

Then start the service (if not already running):

```bash
ollama serve
```

> Keep this terminal open, or configure Ollama as a system service per their docs.

#### Windows

1. Go to: https://ollama.com/download
2. Download the Windows installer.
3. Run the installer and follow the prompts.
4. After installation, Ollama should start as a background service.

---

### 1.3 GPU Drivers (optional but recommended)

You can run models on CPU only, but GPU acceleration is much faster.

#### NVIDIA (CUDA)

- Install the latest NVIDIA driver suitable for your GPU.
- Install CUDA (or at least the driver/runtime) compatible with your driver.
- Reboot after installation.

Helpful link:
https://developer.nvidia.com/cuda-downloads

#### AMD (ROCm)

- Ensure your GPU is supported by ROCm.
- Install ROCm following the official guide for your distro:
  https://rocm.docs.amd.com
- Add any needed environment variables from the ROCm docs.
- Reboot.

#### Apple Silicon (M1/M2/M3 etc.)

- No extra driver install needed; Apple's Metal API is built in.
- Use the latest macOS version for best performance.

> If your GPU is not correctly configured, Ollama (and therefore rigrun) will fall back to CPU.

---

## 2. Installing rigrun

You have three options:

### 2.1 Install via `cargo`

Fastest if you already have Rust.

```bash
cargo install rigrun
```

This will download and compile rigrun and place the binary in `~/.cargo/bin` (or your cargo bin directory).

Verify:

```bash
rigrun --help
```

You should see the CLI help output.

---

### 2.2 Download from Releases

If you don't want to build from source:

1. Visit the releases page:
   `https://github.com/rigrun/rigrun/releases`
2. Download the appropriate binary for your platform (e.g. `rigrun-x86_64-unknown-linux-gnu.tar.gz`).
3. Extract it:

   ```bash
   tar -xzf rigrun-*.tar.gz
   ```

4. Move it somewhere in your `PATH`, for example:

   ```bash
   sudo mv rigrun /usr/local/bin/
   ```

5. Test:

   ```bash
   rigrun --help
   ```

---

### 2.3 Build from Source

1. Clone the repository:

   ```bash
   git clone https://github.com/rigrun/rigrun.git
   cd rigrun
   ```

2. Build in release mode:

   ```bash
   cargo build --release
   ```

3. The compiled binary will be at:

   ```bash
   target/release/rigrun
   ```

4. Optionally, add it to your `PATH`:

   ```bash
   sudo cp target/release/rigrun /usr/local/bin/
   ```

---

## 3. First Run Walkthrough

Once Ollama is running and rigrun is installed, you're ready for your first run.

Run:

```bash
rigrun
```

What happens:

### 3.1 GPU Detection

On startup, rigrun will:

1. Detect available hardware (NVIDIA, AMD, Apple Silicon, Intel Arc).
2. Check VRAM capacity.
3. Determine if GPU acceleration is available.

You may see output like:

```text
✓ GPU: NVIDIA RTX 4090 (24GB)
# or
! GPU: None detected (CPU mode)
```

If you see a warning about GPUs, check the **Common Issues** section below.

---

### 3.2 Model Download (Ollama Pull)

If the recommended model is not yet downloaded, rigrun will automatically fetch it via Ollama.

Typical output:

```text
[↓] Downloading qwen2.5-coder:14b...
pulling manifest
pulling 1a2b3c4d...
verifying sha256 digest
writing manifest
✓ Model ready
```

Notes:

- The download can be **several GB**, depending on the model.
- You only need to download each model **once**; subsequent runs are instant.

You can also pre-pull a model yourself:

```bash
ollama pull qwen2.5-coder:7b
```

and then run `rigrun`.

---

### 3.3 Server Startup

After hardware checks and model setup, rigrun starts its HTTP server.

You'll see something like:

```text
✓ Server: http://localhost:8787

Today: 0 queries | Saved: $0.00

Ready! Try:
  curl localhost:8787/v1/chat/completions -H "Content-Type: application/json" -d '{"model":"auto","messages":[{"role":"user","content":"hi"}]}'
```

From now on:

- Keep this terminal window open; it's your rigrun server.
- Use the printed URL (`http://localhost:8787` by default) in your API requests.

> If the port is already in use, see **Common Issues - Port already in use**.

---

## 4. Making Your First Query

rigrun exposes an **OpenAI-compatible** chat completions endpoint at:

```text
http://localhost:8787/v1/chat/completions
```

### 4.1 cURL Example

```bash
curl http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [
      {"role": "user", "content": "Say hello in one short sentence."}
    ]
  }'
```

### 4.2 Python Example

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8787/v1",
    api_key="unused"  # local inference doesn't need auth
)

response = client.chat.completions.create(
    model="auto",
    messages=[{"role": "user", "content": "Explain what rigrun does."}]
)

print(response.choices[0].message.content)
```

### 4.3 JavaScript Example

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://localhost:8787/v1',
  apiKey: 'unused',
});

const response = await client.chat.completions.create({
  model: 'auto',
  messages: [
    { role: 'user', content: 'Give me a fun fact about Rust.' }
  ]
});

console.log(response.choices[0].message.content);
```

---

### 4.4 Expected Response Format

A typical response looks like:

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1712345678,
  "model": "auto",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! I'm running locally through rigrun and Ollama."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 15,
    "total_tokens": 25
  }
}
```

Key fields:

- `choices[0].message.content` - the actual text answer.
- `usage` - token usage (helpful for monitoring and optimization).

---

## 5. Setting up an OpenRouter Key (Optional)

rigrun can optionally use **OpenRouter** as a *cloud fallback* provider.

### 5.1 Why Cloud Fallback?

You might want to use OpenRouter when:

- Your local GPU is too small/slow for complex queries.
- You need access to advanced proprietary models (GPT-4, Claude, etc.).
- You want automatic fallback when the local model can't handle a query.

OpenRouter acts as a unified gateway to many cloud models.

---

### 5.2 Get an OpenRouter API Key

1. Go to: https://openrouter.ai
2. Sign in or create an account.
3. Navigate to your API keys:
   `https://openrouter.ai/keys`
4. Create a new key and copy it somewhere safe.

---

### 5.3 Configure rigrun with Your OpenRouter Key

Use the rigrun CLI:

```bash
rigrun config --openrouter-key sk-or-v1-xxxxx
```

You should see a confirmation message:

```text
✓ OpenRouter API key set
```

Notes:

- The config is stored in `~/.rigrun/config.json`.
- Restart rigrun if it's already running to pick up the new config.

Once configured, rigrun can automatically route complex queries to cloud models when needed.

---

## 6. Common Issues and Solutions

### 6.1 "Ollama not found" or "Cannot connect to Ollama"

**Symptoms:**

- rigrun logs something like:
  ```text
  ✗ Ollama is not running. Please start it with: ollama serve
  ```

**Fix:**

1. Check that Ollama is installed:

   ```bash
   ollama --version
   ```

   - If this fails, (re)install Ollama (see **Prerequisites**).

2. Start Ollama manually:

   ```bash
   ollama serve
   ```

   Leave this running, and then in another terminal run:

   ```bash
   rigrun
   ```

---

### 6.2 "Port already in use"

**Symptoms:**

- rigrun log shows:

  ```text
  ! Port 8787 is already in use
  ```

**Fix:**

1. Find what's using that port:

   - macOS / Linux:

     ```bash
     lsof -i :8787
     ```

   - Windows (PowerShell):

     ```powershell
     netstat -ano | findstr :8787
     ```

2. Either stop the other service or change rigrun's port:

   ```bash
   rigrun config --port 9000
   ```

   Then use `http://localhost:9000` in your requests.

---

### 6.3 Model Download Fails

**Symptoms:**

- During first run, logs show:

  ```text
  [↓] Downloading model...
  ✗ Failed to download model
  ```

**Possible Causes & Fixes:**

1. **No internet connection:**
   - Check your connection and retry.

2. **Insufficient disk space:**
   - LLM models are large (several GB). Free up space and try again.

3. **Corporate proxy/firewall:**
   - Configure your system/http proxy for Ollama.

4. Retry the pull manually:

   ```bash
   ollama pull qwen2.5-coder:7b
   ```

   Once it succeeds, restart `rigrun`.

---

### 6.4 GPU Not Detected

**Symptoms:**

- rigrun logs say "No GPU detected (CPU mode)".
- Performance is slower than expected.

**Fix:**

1. Confirm your GPU is visible to the OS:

   - NVIDIA (Linux/Windows):

     ```bash
     nvidia-smi
     ```

   - Windows: check **Device Manager → Display adapters**.
   - macOS: **About This Mac → System Report → Graphics/Displays**.

2. Confirm you installed the correct drivers:

   - NVIDIA: use official drivers + CUDA runtime/toolkit.
   - AMD: ensure ROCm version matches your OS and GPU.
   - Apple: update macOS to the latest stable version.

3. Restart your machine after driver installation.

4. Run rigrun's GPU setup wizard:

   ```bash
   rigrun gpu-setup
   ```

   This will diagnose GPU issues and provide specific fix steps.

---

### 6.5 CUDA / ROCm Issues

**Symptoms:**

- Errors mentioning CUDA, cuDNN, or ROCm libraries.
- Models running on CPU despite having GPU.

**Fix:**

1. Ensure version compatibility:
   - CUDA version must match your driver expectations.
   - ROCm version must match your GPU and OS version.

2. For AMD RDNA 4 GPUs:
   - May need the ollama-for-amd fork
   - Run `rigrun gpu-setup` for specific instructions

3. Environment variables:
   - Check `LD_LIBRARY_PATH` on Linux (for CUDA/ROCm libraries).
   - For AMD, may need `HSA_OVERRIDE_GFX_VERSION` set.

4. Verify Ollama alone works with your GPU before involving rigrun:

   ```bash
   ollama run qwen2.5-coder:7b
   ```

---

## 7. Next Steps

Now that you have rigrun running and answering basic prompts, here are ideas for what to explore next.

### 7.1 IDE Integration

Use rigrun as a local backend for coding assistants:

```bash
rigrun ide-setup
```

This interactive wizard will configure:

- **VS Code** - Copilot/Continue extension
- **Cursor** - Custom model endpoint
- **JetBrains** (IntelliJ, PyCharm, WebStorm, etc.) - AI Assistant
- **Neovim** - Copilot.lua / codecompanion.nvim

The wizard auto-generates configurations using your local AI!

---

### 7.2 Advanced Configuration

rigrun offers several configuration options:

```bash
# View current configuration
rigrun config --show

# Change default model
rigrun config --model deepseek-coder-v2:16b

# Change server port
rigrun config --port 8080

# Set OpenRouter key for cloud fallback
rigrun config --openrouter-key sk-or-xxx
```

See [CONFIGURATION.md](CONFIGURATION.md) for all available options.

---

### 7.3 Monitoring and Stats

To understand how rigrun is being used:

```bash
rigrun status
```

This shows:

- Server status and port
- GPU information and VRAM usage
- Currently loaded models
- Today's query statistics
- Cost savings

You can also access stats programmatically:

```bash
curl http://localhost:8787/stats
curl http://localhost:8787/cache/stats
```

---

### 7.4 Interactive Chat

For quick testing and experimentation:

```bash
rigrun chat
```

This starts an interactive chat session in your terminal. Type your queries and get instant responses. Type 'exit' or press Ctrl+C to quit.

---

You now have a working rigrun setup, backed by Ollama and optionally OpenRouter.
From here, you can plug it into apps, scripts, and IDEs just like you would with any OpenAI-compatible API—except it's running under your control, locally and privately.

For more information:
- [API Reference](API.md)
- [Configuration Guide](CONFIGURATION.md)
- [Main README](../README.md)
