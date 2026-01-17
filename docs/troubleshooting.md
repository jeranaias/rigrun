# Troubleshooting Guide

> **Note:** Common issues and solutions are now included in the [**GETTING_STARTED.md**](GETTING_STARTED.md) guide. For quick answers, check the "Common Issues" section there first. This document contains additional detailed troubleshooting information.

---

Solutions to common problems when using rigrun.

---

## Table of Contents

- [Ollama Connection Issues](#ollama-connection-issues)
- [Model Problems](#model-problems)
- [GPU Issues](#gpu-issues)
- [Server Startup Failures](#server-startup-failures)
- [API Request Errors](#api-request-errors)
- [Cache Issues](#cache-issues)
- [OpenRouter Problems](#openrouter-problems)
- [Performance Issues](#performance-issues)
- [Network Problems](#network-problems)
- [Configuration Issues](#configuration-issues)

---

## Ollama Connection Issues

### Failed to Connect to Ollama

**Symptoms:**
```
[✗] Failed to connect to Ollama
Cannot connect to Ollama at http://localhost:11434
```

**Possible Causes:**
- Ollama service not running
- Ollama not installed
- Wrong Ollama URL in config
- Firewall blocking connection

**Solutions:**

**1. Check if Ollama is installed:**
```bash
ollama --version
```

If not installed, see [installation.md](installation.md).

**2. Start Ollama service:**
```bash
# macOS/Linux
ollama serve

# Linux (systemd)
sudo systemctl start ollama
sudo systemctl status ollama

# Windows
# Ollama runs as a service automatically
# Check: Get-Service Ollama
```

**3. Verify Ollama is responding:**
```bash
curl http://localhost:11434/api/tags
```

Should return JSON with model list.

**4. Check firewall:**
```bash
# Linux
sudo ufw status
sudo ufw allow 11434

# macOS
# System Preferences → Security & Privacy → Firewall
```

**5. Check Ollama URL in config:**
```bash
rigrun config --show
# If custom Ollama URL is set, verify it's correct
```

---

## Model Problems

### Model Not Found

**Symptoms:**
```
[✗] Model not found: qwen2.5-coder:7b
```

**Solutions:**

**1. List downloaded models:**
```bash
ollama list
```

**2. Pull the model:**
```bash
ollama pull qwen2.5-coder:7b
```

**3. Wait for download to complete:**
Large models take 5-15 minutes depending on internet speed.

**4. Verify model after download:**
```bash
ollama list
# Should show the model
```

**5. Test model directly:**
```bash
ollama run qwen2.5-coder:7b "Hello"
```

### Model Download Fails

**Symptoms:**
```
[✗] Failed to download model
pulling manifest: connection timeout
```

**Possible Causes:**
- No internet connection
- Insufficient disk space
- Corporate firewall/proxy
- Ollama service overloaded

**Solutions:**

**1. Check internet connection:**
```bash
ping ollama.com
```

**2. Check disk space:**
```bash
# Unix/macOS
df -h ~

# Windows
Get-PSDrive C
```

Models require 3-10GB depending on size.

**3. Check Ollama disk usage:**
```bash
# Unix/macOS
du -sh ~/.ollama

# Windows
# Check C:\Users\<USERNAME>\.ollama
```

**4. Configure proxy (if needed):**
```bash
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
ollama pull qwen2.5-coder:7b
```

**5. Try a smaller model first:**
```bash
ollama pull qwen2.5-coder:1.5b
rigrun config --model qwen2.5-coder:1.5b
```

### Model Runs Out of Memory

**Symptoms:**
```
[✗] Request failed: out of memory
```

**Solutions:**

**1. Check available VRAM:**
```bash
# NVIDIA
nvidia-smi

# AMD
rocm-smi

# Apple Silicon
vm_stat | grep free
```

**2. Use smaller model:**
```bash
# Current model too large, switch to smaller
rigrun config --model qwen2.5-coder:3b
ollama pull qwen2.5-coder:3b
```

**3. Close other GPU applications:**
- Close games, video editors, other ML workloads
- Check GPU usage: `nvidia-smi` or `rocm-smi`

**4. Reduce context length:**
Use shorter prompts and conversations.

**5. Enable CPU offloading (Ollama):**
```bash
# Set environment variable for Ollama
export OLLAMA_GPU_LAYERS=20  # Reduce layers on GPU
ollama serve
```

---

## GPU Issues

### GPU Not Detected

**Symptoms:**
```
! GPU: None detected (CPU mode)
```

**Solutions:**

**1. Run GPU diagnostics:**
```bash
rigrun gpu-setup
```

This wizard diagnoses GPU issues and provides specific fixes.

**2. Verify GPU is working:**

**NVIDIA:**
```bash
nvidia-smi
# Should show GPU name and driver version
```

**AMD:**
```bash
rocm-smi
# Should show GPU info
```

**Apple Silicon:**
```bash
system_profiler SPDisplaysDataType
# Should show "Apple M1/M2/M3"
```

**3. Install/update drivers:**

See [installation.md#gpu-setup](installation.md#gpu-setup) for driver installation.

**4. Reboot after driver installation:**
```bash
sudo reboot
```

**5. Check Ollama can see GPU:**
```bash
ollama run llama3.2 "test"
# Watch output - should mention GPU
```

### NVIDIA CUDA Issues

**Symptoms:**
- "CUDA not found"
- "CUDA driver version mismatch"
- GPU detected but not used

**Solutions:**

**1. Check CUDA installation:**
```bash
nvcc --version
nvidia-smi
```

Version should match (e.g., both show CUDA 12.x).

**2. Install CUDA toolkit:**
```bash
# Ubuntu
sudo apt install nvidia-cuda-toolkit

# Or download from:
# https://developer.nvidia.com/cuda-downloads
```

**3. Update NVIDIA drivers:**
```bash
# Ubuntu
sudo ubuntu-drivers autoinstall
sudo reboot
```

**4. Verify driver-CUDA compatibility:**
Check https://docs.nvidia.com/cuda/cuda-toolkit-release-notes/

**5. Set CUDA paths:**
```bash
export PATH=/usr/local/cuda/bin:$PATH
export LD_LIBRARY_PATH=/usr/local/cuda/lib64:$LD_LIBRARY_PATH
```

Add to `~/.bashrc` or `~/.zshrc` to persist.

### AMD ROCm Issues

**Symptoms:**
- "ROCm not found"
- GPU shows but models run slowly
- HSA errors

**Solutions:**

**1. Verify ROCm installation:**
```bash
rocm-smi
rocminfo
```

**2. Check GPU is supported:**
Not all AMD GPUs support ROCm. Check:
https://rocm.docs.amd.com/en/latest/release/gpu_os_support.html

**3. For RDNA 4 (RX 9070/9070 XT):**
```bash
# Set GFX version override
export HSA_OVERRIDE_GFX_VERSION=11.0.0
echo 'export HSA_OVERRIDE_GFX_VERSION=11.0.0' >> ~/.bashrc
```

**4. For RDNA 4 (RX 9000 series), use Vulkan backend:**
```bash
# Set Vulkan environment variable
export OLLAMA_VULKAN=1
ollama serve

# Or make it permanent
echo 'export OLLAMA_VULKAN=1' >> ~/.bashrc
```

On Windows:
```batch
set OLLAMA_VULKAN=1
ollama serve
```

**5. Add user to groups:**
```bash
sudo usermod -a -G video,render $USER
# Logout and login again
```

---

## Server Startup Failures

### Port Already in Use

**Symptoms:**
```
[✗] Port 8787 is already in use
```

**Solutions:**

**1. Find what's using the port:**

**macOS/Linux:**
```bash
lsof -i :8787
```

**Windows:**
```powershell
netstat -ano | findstr :8787
```

**2. Kill the process:**

**macOS/Linux:**
```bash
kill -9 <PID>
```

**Windows:**
```powershell
taskkill /F /PID <PID>
```

**3. Or use a different port:**
```bash
rigrun config --port 8080
rigrun
```

### Server Crashes on Startup

**Symptoms:**
- rigrun exits immediately
- "Panic" or "fatal error" messages

**Solutions:**

**1. Run with debug logging:**
```bash
RUST_LOG=debug rigrun
```

Look for specific error messages.

**2. Check configuration file:**
```bash
cat ~/.rigrun/config.json
```

Ensure valid JSON. Fix syntax errors or delete file to reset:
```bash
rm ~/.rigrun/config.json
rigrun  # Will recreate with defaults
```

**3. Clear cache (may be corrupted):**
```bash
rm -rf ~/.rigrun/cache
rigrun
```

**4. Reinstall rigrun:**
```bash
cargo install rigrun --force
# Or download fresh binary
```

---

## API Request Errors

### 400 Bad Request

**Symptoms:**
```json
{
  "error": {
    "message": "Request must contain at least one message",
    "type": "invalid_request_error"
  }
}
```

**Solutions:**

**1. Verify request format:**
```bash
curl http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

Required fields:
- `model` (string)
- `messages` (array with at least one message)

**2. Check message format:**
Each message needs:
- `role`: "system", "user", or "assistant"
- `content`: non-empty string

**3. Validate JSON:**
Use a JSON validator or `jq`:
```bash
echo '{"model":"auto","messages":[...]}' | jq .
```

### 500 Internal Server Error

**Symptoms:**
```json
{
  "error": {
    "message": "Internal server error",
    "type": "server_error"
  }
}
```

**Solutions:**

**1. Check rigrun logs:**
Look at terminal where rigrun is running for error details.

**2. Check Ollama status:**
```bash
curl http://localhost:11434/api/tags
```

**3. Verify model is loaded:**
```bash
ollama list
```

**4. Restart rigrun:**
```bash
# Ctrl+C to stop
rigrun
```

**5. Report bug:**
If issue persists, report at:
https://github.com/rigrun/rigrun/issues

### Request Timeout

**Symptoms:**
```
[✗] Request timed out after 120 seconds
```

**Solutions:**

**1. Use smaller model:**
```bash
rigrun config --model qwen2.5-coder:3b
```

**2. Reduce prompt length:**
Shorter prompts = faster responses.

**3. Check system resources:**
```bash
# CPU usage
top  # Unix
Get-Process  # Windows

# Memory
free -h  # Linux
vm_stat  # macOS
```

**4. Close other applications:**
Free up RAM and VRAM.

**5. Switch to cloud for complex queries:**
```bash
# In request, use model: "cloud"
curl http://localhost:8787/v1/chat/completions \
  -d '{"model":"cloud","messages":[...]}'
```

---

## Cache Issues

### Cache Not Working

**Symptoms:**
- Cache hit rate always 0%
- Same query always goes to model

**Solutions:**

**1. Check cache stats:**
```bash
curl http://localhost:8787/cache/stats
```

**2. Verify cache directory exists:**
```bash
ls -la ~/.rigrun/cache/
```

**3. Check cache TTL:**
Default is 24 hours. Old entries expire.

**4. Clear and rebuild cache:**
```bash
rm -rf ~/.rigrun/cache
rigrun  # Cache will rebuild
```

### Low Cache Hit Rate

**Expected:** 40-60% for typical usage

**If lower:**

**1. Review query patterns:**
Cache works best for:
- Repeated questions
- Similar queries
- Common patterns

**2. Increase semantic similarity:**
Cache uses embedding similarity. Very different queries won't match.

**3. Check cache entries:**
```bash
curl http://localhost:8787/cache/stats
# Look at "entries" count
```

Few entries = not enough usage to build up cache.

---

## OpenRouter Problems

### OpenRouter Not Configured

**Symptoms:**
```
[✗] OpenRouter not configured
```

**Solutions:**

**1. Get API key:**
https://openrouter.ai/keys

**2. Configure rigrun:**
```bash
rigrun config --openrouter-key sk-or-v1-xxxxx
```

**3. Restart rigrun:**
```bash
# Ctrl+C, then
rigrun
```

**4. Verify:**
```bash
rigrun config --show
# Should show OpenRouter key (partially masked)
```

### Authentication Failed

**Symptoms:**
```
[✗] Authentication failed
Invalid API key
```

**Solutions:**

**1. Verify API key format:**
Should start with `sk-or-v1-`

**2. Check key at OpenRouter:**
https://openrouter.ai/keys

**3. Generate new key if expired:**
```bash
rigrun config --openrouter-key sk-or-v1-NEW_KEY
```

**4. Check account status:**
https://openrouter.ai/account

### Rate Limit Exceeded

**Symptoms:**
```
[✗] Rate limit exceeded
Too many requests
```

**Solutions:**

**1. Wait 60 seconds:**
Rate limits reset quickly.

**2. Add credits to account:**
https://openrouter.ai/credits

**3. Use local models more:**
```bash
# Force local for most queries
# Only explicitly use cloud when needed
```

**4. Check usage:**
https://openrouter.ai/activity

---

## Performance Issues

### Slow Response Times

**Solutions:**

**1. Check cache hit rate:**
```bash
curl http://localhost:8787/cache/stats
```

Target: >50% hit rate for best performance.

**2. Use appropriate model size:**
```bash
# Faster models for simple tasks
rigrun config --model qwen2.5-coder:3b

# Larger models only when needed
rigrun config --model qwen2.5-coder:14b
```

**3. Monitor GPU usage:**
```bash
# While making request
nvidia-smi -l 1  # NVIDIA
rocm-smi -d  # AMD
```

**4. Check system resources:**
```bash
top  # CPU and RAM usage
```

**5. Reduce concurrent requests:**
Too many simultaneous requests = slower responses.

### High Memory Usage

**Solutions:**

**1. Check loaded models:**
```bash
ollama list
```

**2. Unload unused models:**
```bash
ollama rm model-name
```

**3. Use smaller model:**
```bash
rigrun config --model qwen2.5-coder:3b
```

**4. Clear cache if too large:**
```bash
du -sh ~/.rigrun/cache/  # Check size
rm -rf ~/.rigrun/cache/  # Clear if needed
```

---

## Network Problems

### Cannot Connect to Server

**Symptoms:**
```
curl: (7) Failed to connect to localhost port 8787: Connection refused
```

**Solutions:**

**1. Check if rigrun is running:**
```bash
ps aux | grep rigrun  # Unix
Get-Process rigrun  # Windows
```

**2. Start rigrun:**
```bash
rigrun
```

**3. Verify port:**
```bash
rigrun config --show
# Check which port is configured
```

**4. Test with correct port:**
```bash
curl http://localhost:8787/health
```

### Firewall Blocking Connections

**Solutions:**

**Linux (ufw):**
```bash
sudo ufw allow 8787
```

**macOS:**
System Preferences → Security & Privacy → Firewall → Firewall Options → Allow rigrun

**Windows:**
```powershell
# Run as Administrator
New-NetFirewallRule -DisplayName "rigrun" -Direction Inbound -LocalPort 8787 -Protocol TCP -Action Allow
```

---

## Configuration Issues

### Config Not Loading

**Solutions:**

**1. Check file location:**
```bash
rigrun config --show
# Shows config file path
```

**2. Verify JSON syntax:**
```bash
cat ~/.rigrun/config.json | jq .
```

**3. Reset to defaults:**
```bash
rm ~/.rigrun/config.json
rigrun  # Recreates config
```

### Changes Not Taking Effect

**Solutions:**

**1. Restart rigrun:**
Config is loaded at startup.
```bash
# Stop (Ctrl+C), then start
rigrun
```

**2. Verify changes saved:**
```bash
rigrun config --show
```

**3. Check file permissions:**
```bash
ls -l ~/.rigrun/config.json
# Should be readable/writable by user
```

---

## Advanced Diagnostics

### Run System Check

```bash
rigrun doctor
```

This checks:
- Ollama installation and status
- GPU detection and drivers
- Model availability
- Port availability
- Cache health
- Configuration validity

### Collect Debug Information

For bug reports:

**1. Get versions:**
```bash
rigrun --version
ollama --version
rustc --version  # If built from source
```

**2. Get logs:**
```bash
RUST_LOG=debug rigrun > rigrun-debug.log 2>&1
```

**3. Get configuration:**
```bash
rigrun config --show > config-output.txt
```

**4. Get system info:**
```bash
# GPU
nvidia-smi > gpu-info.txt  # NVIDIA
rocm-smi > gpu-info.txt    # AMD

# OS
uname -a > system-info.txt  # Unix
systeminfo > system-info.txt  # Windows
```

**5. Report issue:**
Open issue at https://github.com/rigrun/rigrun/issues with collected information.

---

## Getting More Help

If issues persist:

- **GitHub Issues**: https://github.com/rigrun/rigrun/issues
- **Discussions**: https://github.com/rigrun/rigrun/discussions
- **Documentation**: https://github.com/rigrun/rigrun/tree/main/docs

When reporting issues, include:
- Operating system and version
- GPU type and drivers
- rigrun and Ollama versions
- Error messages and logs
- Steps to reproduce

---

## Related Documentation

- [Getting Started](getting-started.md) - Initial setup guide
- [Installation](installation.md) - Detailed installation
- [Configuration](configuration.md) - Configuration options
- [API Reference](api-reference.md) - API documentation
