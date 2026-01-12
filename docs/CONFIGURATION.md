# rigrun Configuration Guide

This guide covers all configuration options for rigrun, including CLI commands, configuration files, and environment variables.

---

## Configuration File

### Location

rigrun stores its configuration in a JSON file at:

**Unix/Linux/macOS:**
```
~/.rigrun/config.json
```

**Windows:**
```
C:\Users\<USERNAME>\.rigrun\config.json
```

The directory is created automatically on first run.

### Format

The configuration file is JSON format with the following structure:

```json
{
  "openrouter_key": "sk-or-v1-xxxxx",
  "model": "qwen2.5-coder:14b",
  "port": 8787,
  "first_run_complete": true
}
```

### Configuration Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `openrouter_key` | string | OpenRouter API key for cloud fallback | `null` |
| `model` | string | Default local model to use | Auto-detected based on GPU |
| `port` | int | Server port | `8787` |
| `first_run_complete` | boolean | Whether first-run setup completed | `false` |

---

## CLI Configuration Commands

### View Current Configuration

```bash
rigrun config --show
```

Output example:
```
=== RigRun Configuration ===

  OpenRouter Key: sk-or-v1...
  Model:          qwen2.5-coder:14b
  Port:           8787

Config file: /home/user/.rigrun/config.json
```

### Set OpenRouter API Key

```bash
rigrun config --openrouter-key sk-or-v1-xxxxx
```

This enables cloud fallback routing for complex queries. Get your key at https://openrouter.ai/keys

### Set Default Model

```bash
rigrun config --model qwen2.5-coder:14b
```

Overrides the auto-detected model recommendation. Available models:

| VRAM | Recommended Model |
|------|-------------------|
| <6GB | `qwen2.5-coder:3b` |
| 6-8GB | `qwen2.5-coder:7b` |
| 8-16GB | `qwen2.5-coder:14b` |
| 16-24GB | `deepseek-coder-v2:16b` |
| 24GB+ | `llama3.3:70b` |

### Set Server Port

```bash
rigrun config --port 8080
```

Changes the port rigrun listens on. Useful when port 8787 is already in use.

### Multiple Options

You can set multiple options in one command:

```bash
rigrun config --model qwen2.5-coder:7b --port 9000
```

---

## Environment Variables

### OPENROUTER_API_KEY

If set, rigrun will use this as the OpenRouter API key (can be overridden by config file):

**Unix/Linux/macOS:**
```bash
export OPENROUTER_API_KEY=sk-or-v1-xxxxx
rigrun
```

**Windows (PowerShell):**
```powershell
$env:OPENROUTER_API_KEY="sk-or-v1-xxxxx"
rigrun
```

**Windows (CMD):**
```cmd
set OPENROUTER_API_KEY=sk-or-v1-xxxxx
rigrun
```

---

## Model Management

### List Available Models

```bash
rigrun models
```

Shows:
- Available models for download
- Currently downloaded models (marked with ✓)
- Recommended model for your GPU

### Download a Model

```bash
rigrun pull qwen2.5-coder:14b
```

Downloads the specified model via Ollama. Large models may take several minutes depending on your internet connection.

### Check Downloaded Models

```bash
ollama list
```

Lists all models currently downloaded on your system.

### Remove a Model

```bash
ollama rm qwen2.5-coder:14b
```

Frees up disk space by removing unused models.

---

## Port Configuration

### Default Port

rigrun uses port **8787** by default. This can be changed via:

1. **Config file:**
   ```json
   {
     "port": 8080
   }
   ```

2. **CLI command:**
   ```bash
   rigrun config --port 8080
   ```

### Port Conflicts

If port 8787 is already in use, rigrun will:
1. Detect the conflict
2. Check if another rigrun instance is running
3. Attempt to stop it (if it's rigrun)
4. Search for next available port (8788, 8789, etc.)

You can manually specify a different port to avoid conflicts.

### Finding Process Using a Port

**Unix/Linux/macOS:**
```bash
lsof -i :8787
```

**Windows (PowerShell):**
```powershell
netstat -ano | findstr :8787
```

---

## Cloud Provider Configuration

### OpenRouter Setup

OpenRouter provides access to multiple cloud LLM providers through a single API.

#### 1. Get an API Key

1. Visit https://openrouter.ai
2. Sign up or log in
3. Navigate to https://openrouter.ai/keys
4. Create a new API key
5. Copy the key (starts with `sk-or-v1-`)

#### 2. Configure rigrun

```bash
rigrun config --openrouter-key sk-or-v1-xxxxx
```

#### 3. Verify Setup

```bash
rigrun config --show
```

Should show your OpenRouter key (partially masked).

### Model Routing

When OpenRouter is configured, rigrun automatically routes queries:

- **Simple queries** → Local GPU
- **Complex queries** → Cloud (OpenRouter auto-selects best model)
- **Explicit cloud** → Use `model: "cloud"`, `model: "haiku"`, etc.

### Cost Tracking

rigrun tracks:
- Local queries (free)
- Cloud queries (cost estimated)
- Money saved vs all-cloud approach

View stats:
```bash
rigrun status
```

Or programmatically:
```bash
curl http://localhost:8787/stats
```

---

## Cache Configuration

### Cache Location

Response cache is stored at:

**Unix/Linux/macOS:**
```
~/.rigrun/cache/
```

**Windows:**
```
C:\Users\<USERNAME>\.rigrun\cache\
```

### Cache TTL (Time-To-Live)

Default: **24 hours**

Cache entries expire after 24 hours. This is currently hardcoded but ensures fresh responses while maximizing cache hits.

### Cache Statistics

View cache performance:

```bash
curl http://localhost:8787/cache/stats
```

Response:
```json
{
  "entries": 128,
  "total_lookups": 500,
  "hits": 320,
  "misses": 180,
  "hit_rate_percent": 64.0,
  "ttl_hours": 24
}
```

### Clear Cache

To clear the cache, delete the cache directory:

**Unix/Linux/macOS:**
```bash
rm -rf ~/.rigrun/cache
```

**Windows (PowerShell):**
```powershell
Remove-Item -Recurse -Force "$env:USERPROFILE\.rigrun\cache"
```

The cache will be recreated automatically on next request.

---

## Advanced Configuration

### GPU Setup

For AMD GPUs, especially RDNA 4, you may need special configuration:

```bash
rigrun gpu-setup
```

This wizard will:
- Detect your GPU
- Check for driver/ROCm issues
- Provide specific fix commands
- Suggest optimal models

### Environment Variables for GPU

**AMD (ROCm):**
```bash
export HSA_OVERRIDE_GFX_VERSION=11.0.0
```

**NVIDIA (CUDA):**
```bash
export CUDA_VISIBLE_DEVICES=0
```

Set these before starting rigrun if you have multiple GPUs.

### Logging

rigrun uses the `tracing` crate for logging. Control verbosity with:

```bash
RUST_LOG=debug rigrun
```

Levels: `error`, `warn`, `info`, `debug`, `trace`

---

## Configuration Examples

### Example 1: Local-Only Setup

No cloud fallback, just local inference:

```json
{
  "model": "qwen2.5-coder:14b",
  "port": 8787
}
```

```bash
# Don't set openrouter_key
rigrun
```

### Example 2: Hybrid with Cloud Fallback

Local-first with cloud for complex queries:

```json
{
  "openrouter_key": "sk-or-v1-xxxxx",
  "model": "qwen2.5-coder:7b",
  "port": 8787
}
```

```bash
rigrun config --openrouter-key sk-or-v1-xxxxx
rigrun
```

### Example 3: Development Setup

Different port, smaller model:

```json
{
  "model": "qwen2.5-coder:3b",
  "port": 9000
}
```

```bash
rigrun config --model qwen2.5-coder:3b --port 9000
rigrun
```

### Example 4: Production Setup

Larger model, default port, cloud fallback:

```json
{
  "openrouter_key": "sk-or-v1-xxxxx",
  "model": "deepseek-coder-v2:16b",
  "port": 8787
}
```

With systemd service (Linux):

```bash
# Install rigrun
cargo install rigrun

# Create service file
sudo nano /etc/systemd/system/rigrun.service
```

Service file:
```ini
[Unit]
Description=rigrun Local LLM Router
After=network.target

[Service]
Type=simple
User=your-username
Environment="OPENROUTER_API_KEY=sk-or-v1-xxxxx"
ExecStart=/home/your-username/.cargo/bin/rigrun
Restart=always

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable rigrun
sudo systemctl start rigrun
sudo systemctl status rigrun
```

### Example 5: Multi-User Setup

On a shared server, users can run rigrun on different ports:

**User 1:**
```bash
rigrun config --port 8787
rigrun
```

**User 2:**
```bash
rigrun config --port 8788
rigrun
```

**User 3:**
```bash
rigrun config --port 8789
rigrun
```

Each user gets their own:
- Config file: `~/.rigrun/config.json`
- Cache: `~/.rigrun/cache/`
- Stats: `~/.rigrun/stats.json`

---

## Troubleshooting Configuration

### Config Not Loading

If changes aren't taking effect:

1. **Check file location:**
   ```bash
   rigrun config --show
   ```
   This shows the config file path.

2. **Verify JSON syntax:**
   ```bash
   cat ~/.rigrun/config.json | jq .
   ```
   (Requires `jq` installed)

3. **Restart rigrun:**
   Config is loaded at startup. Stop (Ctrl+C) and restart.

### OpenRouter Key Not Working

1. **Verify key format:**
   Should start with `sk-or-v1-`

2. **Check account credits:**
   Visit https://openrouter.ai/credits

3. **Test key manually:**
   ```bash
   curl https://openrouter.ai/api/v1/auth/key \
     -H "Authorization: Bearer sk-or-v1-xxxxx"
   ```

### Model Not Found

1. **Check model is downloaded:**
   ```bash
   ollama list
   ```

2. **Download if missing:**
   ```bash
   rigrun pull qwen2.5-coder:14b
   ```

3. **Verify model name:**
   ```bash
   rigrun models
   ```

### Port Already in Use

1. **Find conflicting process:**
   ```bash
   lsof -i :8787  # Unix
   netstat -ano | findstr :8787  # Windows
   ```

2. **Change port:**
   ```bash
   rigrun config --port 8080
   ```

3. **Or stop conflicting process:**
   ```bash
   kill -9 <PID>  # Unix
   taskkill /F /PID <PID>  # Windows
   ```

---

## Configuration Best Practices

1. **Start with defaults**
   - Let rigrun auto-detect your GPU and model
   - Add cloud fallback later if needed

2. **Use version control for configs**
   - Keep a backup of your config
   - Document custom settings

3. **Monitor cache effectiveness**
   ```bash
   curl http://localhost:8787/cache/stats
   ```
   - Aim for >50% hit rate
   - Adjust caching strategy if needed

4. **Track costs**
   ```bash
   rigrun status
   ```
   - Monitor daily spending
   - Adjust model routing if costs too high

5. **Keep models updated**
   ```bash
   ollama list
   ollama pull qwen2.5-coder:14b
   ```

---

For more information:
- [Quick Start Guide](QUICKSTART.md)
- [API Reference](API.md)
- [Main README](../README.md)
