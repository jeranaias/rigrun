# Installation Guide

Complete installation instructions for rigrun across all supported platforms.

---

## Table of Contents

- [System Requirements](#system-requirements)
- [Installing Ollama](#installing-ollama)
- [Installing rigrun](#installing-rigrun)
- [GPU Setup](#gpu-setup)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

---

## System Requirements

### Minimum Requirements

| Component | Requirement |
|-----------|-------------|
| **OS** | Windows 10+, macOS 10.15+, Linux (kernel 4.x+) |
| **RAM** | 8GB (16GB recommended) |
| **Storage** | 10GB free for models |
| **CPU** | Any modern x86_64 or ARM64 processor |

### Recommended GPU

| GPU Type | VRAM | Performance |
|----------|------|-------------|
| **NVIDIA** | 6GB+ | Excellent (CUDA) |
| **AMD** | 6GB+ | Good (ROCm, Linux only) |
| **Apple Silicon** | 8GB+ unified | Excellent (Metal) |
| **Intel Arc** | 6GB+ | Good (experimental) |
| **None (CPU)** | N/A | Slow but functional |

---

## Installing Ollama

Ollama is required for local inference. Install it before rigrun.

### macOS

**Option 1: Official Installer (Recommended)**
1. Download from https://ollama.com/download
2. Open the `.dmg` file
3. Drag Ollama to Applications
4. Launch Ollama (starts background service)

**Option 2: Homebrew**
```bash
brew install ollama
```

**Start Ollama:**
Ollama runs automatically after installation. To start manually:
```bash
ollama serve
```

**Verify:**
```bash
ollama --version
ollama list  # Should show empty list initially
```

### Linux

**Ubuntu/Debian:**
```bash
curl -fsSL https://ollama.com/install.sh | sh
```

This installs Ollama and sets up a systemd service.

**Start Service:**
```bash
sudo systemctl start ollama
sudo systemctl enable ollama  # Start on boot
```

**Check Status:**
```bash
sudo systemctl status ollama
```

**Manual Start (Alternative):**
```bash
ollama serve
```

**Verify:**
```bash
ollama --version
curl http://localhost:11434/api/tags  # Should return JSON
```

### Windows

**Installation:**
1. Download installer from https://ollama.com/download
2. Run `OllamaSetup.exe`
3. Follow installation wizard
4. Ollama starts as a Windows service automatically

**Verify:**
```powershell
ollama --version
ollama list
```

**Troubleshooting Windows:**
If Ollama doesn't start:
```powershell
# Check service status
Get-Service Ollama

# Start service manually
Start-Service Ollama
```

---

## Installing rigrun

### Method 1: Pre-Built Binaries (Recommended)

**1. Download Release**

Visit https://github.com/rigrun/rigrun/releases

Download the appropriate file for your platform:
- **macOS (Intel)**: `rigrun-x86_64-apple-darwin.tar.gz`
- **macOS (Apple Silicon)**: `rigrun-aarch64-apple-darwin.tar.gz`
- **Linux (x86_64)**: `rigrun-x86_64-unknown-linux-gnu.tar.gz`
- **Linux (ARM64)**: `rigrun-aarch64-unknown-linux-gnu.tar.gz`
- **Windows**: `rigrun-x86_64-pc-windows-msvc.zip`

**2. Extract**

**macOS/Linux:**
```bash
tar -xzf rigrun-*.tar.gz
```

**Windows:**
Right-click the `.zip` file and select "Extract All"

**3. Install to PATH**

**macOS/Linux:**
```bash
sudo mv rigrun /usr/local/bin/
sudo chmod +x /usr/local/bin/rigrun
```

**Windows:**
1. Move `rigrun.exe` to `C:\Program Files\rigrun\`
2. Add to PATH:
   - Open "System Properties" → "Environment Variables"
   - Edit "Path" under "System variables"
   - Add `C:\Program Files\rigrun\`
   - Click OK and restart terminal

**4. Verify:**
```bash
rigrun --version
rigrun --help
```

### Method 2: Cargo (Build from crates.io)

**Prerequisites:**
Install Rust: https://rustup.rs

**Install:**
```bash
cargo install rigrun
```

This downloads, compiles, and installs rigrun to `~/.cargo/bin/`

**Verify:**
```bash
rigrun --version
```

### Method 3: Build from Source

**1. Clone Repository:**
```bash
git clone https://github.com/rigrun/rigrun.git
cd rigrun
```

**2. Build:**
```bash
cargo build --release
```

**3. Install:**
```bash
# The binary is at target/release/rigrun
sudo cp target/release/rigrun /usr/local/bin/
```

**4. Verify:**
```bash
rigrun --version
```

---

## GPU Setup

rigrun auto-detects GPUs, but some platforms need driver configuration.

### NVIDIA (CUDA)

**Windows:**
1. Download NVIDIA drivers: https://www.nvidia.com/drivers
2. Install CUDA Toolkit: https://developer.nvidia.com/cuda-downloads
3. Reboot

**Linux:**
```bash
# Ubuntu/Debian
sudo apt update
sudo apt install nvidia-driver-535 nvidia-cuda-toolkit

# Reboot
sudo reboot

# Verify
nvidia-smi
```

**Verify CUDA:**
```bash
nvidia-smi
nvcc --version
```

### AMD (ROCm - Linux Only)

**Ubuntu 22.04:**
```bash
# Add ROCm repository
wget https://repo.radeon.com/amdgpu-install/latest/ubuntu/jammy/amdgpu-install_*.deb
sudo apt install ./amdgpu-install_*.deb

# Install ROCm
sudo amdgpu-install --usecase=rocm

# Add user to video and render groups
sudo usermod -a -G video,render $USER

# Reboot
sudo reboot

# Verify
rocm-smi
```

**Supported GPUs:**
- RX 6000 series (RDNA 2)
- RX 7000 series (RDNA 3)
- RX 9000 series (RDNA 4)

**RDNA 4 Special Configuration:**

For RX 9070/9070 XT:
```bash
# Set GFX version override
export HSA_OVERRIDE_GFX_VERSION=11.0.0
echo 'export HSA_OVERRIDE_GFX_VERSION=11.0.0' >> ~/.bashrc

# May need ollama-for-amd fork
# Run rigrun gpu-setup for specific instructions
rigrun gpu-setup
```

### Apple Silicon (M1/M2/M3)

**No additional setup needed.**

Metal acceleration is built into macOS. Ensure you're running:
- macOS 12 (Monterey) or later
- Latest system updates

**Verify:**
```bash
system_profiler SPDisplaysDataType | grep "Chipset Model"
```

Should show "Apple M1/M2/M3" or similar.

### Intel Arc

**Windows/Linux:**

Intel Arc GPU support is experimental.

**Install Drivers:**
- Windows: https://www.intel.com/content/www/us/en/download/785597/
- Linux: Follow Intel's Linux GPU driver guide

**Note**: May require specific Ollama builds or manual configuration.

---

## Verification

### 1. Check Ollama

```bash
ollama --version
ollama list
```

Should show version and model list (empty initially).

### 2. Check rigrun

```bash
rigrun --version
rigrun --help
```

### 3. Start rigrun (First Run)

```bash
rigrun
```

Expected output:
```
✓ GPU: NVIDIA RTX 3080 (10GB)
# or
✓ GPU: Apple M1 Pro (16GB unified memory)
# or
! GPU: None detected (CPU mode)

[↓] Downloading qwen2.5-coder:7b (4.2 GB)...
✓ Model ready
✓ Server: http://localhost:8787

Today: 0 queries | Saved: $0.00

Ready!
```

### 4. Test API

In another terminal:
```bash
curl http://localhost:8787/health
```

Expected:
```json
{
  "status": "ok",
  "version": "0.1.0"
}
```

### 5. Test Inference

```bash
curl http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

If you see a response with generated text, installation is successful!

---

## Post-Installation Configuration

### Set Default Model

```bash
rigrun config --model qwen2.5-coder:14b
```

### Change Port

```bash
rigrun config --port 8080
```

### Add OpenRouter (Optional)

For cloud fallback:
```bash
rigrun config --openrouter-key sk-or-v1-xxxxx
```

### Check Configuration

```bash
rigrun config --show
```

---

## Troubleshooting Installation

### Ollama Issues

**"Command not found: ollama"**

- **macOS**: Restart terminal after installation
- **Linux**: Ensure `/usr/local/bin` is in PATH
- **Windows**: Restart terminal or add Ollama to PATH manually

**"Cannot connect to Ollama"**

Check if service is running:
```bash
# Linux
sudo systemctl status ollama

# macOS/Windows
ollama serve
```

### rigrun Issues

**"Command not found: rigrun"**

Verify installation location:
```bash
# Should be in PATH
which rigrun  # Unix
where rigrun  # Windows
```

Add to PATH if needed:
```bash
# macOS/Linux - add to ~/.bashrc or ~/.zshrc
export PATH="$HOME/.cargo/bin:$PATH"

# Windows - use System Properties → Environment Variables
```

**"Failed to compile rigrun"**

If using `cargo install`:
```bash
# Update Rust
rustup update stable

# Try again with verbose output
cargo install rigrun -v
```

### GPU Issues

**"GPU not detected"**

Run diagnostics:
```bash
rigrun gpu-setup
```

Check drivers:
```bash
# NVIDIA
nvidia-smi

# AMD
rocm-smi

# Intel
xpu-smi  # If available
```

**"Out of memory" errors**

Use a smaller model:
```bash
rigrun config --model qwen2.5-coder:3b
ollama pull qwen2.5-coder:3b
```

### Port Conflicts

**"Port 8787 already in use"**

Find what's using it:
```bash
# macOS/Linux
lsof -i :8787

# Windows
netstat -ano | findstr :8787
```

Use different port:
```bash
rigrun config --port 8080
rigrun
```

---

## Uninstallation

### Remove rigrun

**If installed via binary:**
```bash
sudo rm /usr/local/bin/rigrun  # Unix
# Or delete from C:\Program Files\rigrun\ on Windows
```

**If installed via cargo:**
```bash
cargo uninstall rigrun
```

**Remove config and data:**
```bash
rm -rf ~/.rigrun  # Unix/macOS
# Or C:\Users\<USERNAME>\.rigrun on Windows
```

### Remove Ollama

**macOS:**
```bash
# Stop service
ollama stop

# Remove app
rm -rf /Applications/Ollama.app
rm -rf ~/.ollama
```

**Linux:**
```bash
# Stop service
sudo systemctl stop ollama
sudo systemctl disable ollama

# Remove package
sudo apt remove ollama  # If installed via package manager
# or delete binary
sudo rm /usr/local/bin/ollama

# Remove data
rm -rf ~/.ollama
```

**Windows:**
1. Uninstall via "Add or Remove Programs"
2. Delete `C:\Users\<USERNAME>\.ollama` if needed

---

## Next Steps

- **[Getting Started Guide](getting-started.md)** - Make your first API call
- **[Configuration Guide](configuration.md)** - Configure rigrun
- **[API Reference](api-reference.md)** - Learn the API
- **[Troubleshooting](troubleshooting.md)** - Fix common issues

---

## Additional Resources

- **Ollama Documentation**: https://github.com/ollama/ollama/tree/main/docs
- **CUDA Installation**: https://docs.nvidia.com/cuda/cuda-installation-guide-linux/
- **ROCm Installation**: https://rocm.docs.amd.com/
- **rigrun GitHub**: https://github.com/rigrun/rigrun
