# GPU Compatibility Guide for rigrun

> **Note:** Essential GPU setup information is now included in the [**GETTING_STARTED.md**](GETTING_STARTED.md) guide. This document contains additional detailed GPU compatibility information and advanced configuration options.

---

This guide covers GPU compatibility for running local LLM inference with Ollama as rigrun's local backend.

## Quick Reference by GPU Series

| GPU Series | Support Level | Notes |
|------------|---------------|-------|
| **NVIDIA RTX 40/30 Series** | Full | Works out of the box |
| **NVIDIA RTX 20 Series** | Full | Works out of the box |
| **AMD RDNA 4 (RX 9000)** | Vulkan | Set `OLLAMA_VULKAN=1` |
| **AMD RDNA 3 (RX 7000)** | Partial | ROCm on Linux, Vulkan on Windows |
| **AMD RDNA 2 (RX 6000)** | Partial | May need HSA override |
| **Intel Arc** | Limited | Experimental support |
| **Apple Silicon** | Full | Works out of the box |

## AMD RDNA 4 (RX 9070 XT, RX 9070, RX 9060 XT)

**Status**: Works with Vulkan backend

AMD's RDNA 4 architecture (gfx1200/gfx1201) is not yet supported by ROCm/HIP on Windows, but **Ollama's Vulkan backend provides full GPU acceleration**.

### Setup (Windows)

**Option 1: Use the startup script**
```batch
scripts\start_ollama_vulkan.bat
```

**Option 2: Set environment variable permanently**
```batch
scripts\set_ollama_vulkan_permanent.bat
```
(Requires running as Administrator)

**Option 3: Manual**
```batch
set OLLAMA_VULKAN=1
ollama serve
```

### Performance (RX 9070 XT - 16GB VRAM)

Tested performance with Vulkan backend:

| Model Size | Response Time | Notes |
|------------|---------------|-------|
| 3B | ~2-3 seconds | Fits entirely in VRAM |
| 14B | ~10-15 seconds | Fits entirely in VRAM |
| 22B | ~15-20 seconds | Sweet spot for 16GB |
| 32B | ~30-45 seconds | Uses iGPU shared memory |

### Multi-GPU Feature

Ollama with Vulkan can split models across multiple GPUs, including the iGPU:
- **dGPU (RX 9070 XT)**: 16GB VRAM
- **iGPU**: Accesses shared system RAM (32-64GB available)

This allows running 32B+ models by splitting layers between dGPU and iGPU.

### Vulkan Features Detected

On RX 9070 XT:
- `fp16`: Enabled
- `bf16`: Enabled
- `KHR_coopmat`: Enabled (cooperative matrix operations)

## Model Recommendations by VRAM

### 8GB VRAM (RTX 3060, RX 7600)

| Use Case | Model | Quantization | Size |
|----------|-------|--------------|------|
| General | Qwen 2.5 7B | Q4_K_M | 5.6GB |
| Coding | Qwen 2.5 Coder 7B | Q4_K_M | 5.6GB |
| Fast | Phi-3 Mini 3.8B | Q4_K_M | 2.5GB |

### 12GB VRAM (RTX 4070, RX 7700 XT)

| Use Case | Model | Quantization | Size |
|----------|-------|--------------|------|
| General | Qwen 3 8B | Q5_K_M | 8.5GB |
| Coding | Qwen 2.5 Coder 14B | Q4_K_M | 11GB |
| Reasoning | Phi-4 14B | Q4_K_M | 10.5GB |

### 16GB VRAM (RTX 4060 Ti 16GB, RX 9070 XT, RX 7800 XT)

| Use Case | Model | Quantization | Size |
|----------|-------|--------------|------|
| General | Qwen 3 14B | Q5_K_M | 14GB |
| Coding | Codestral 22B | Q4_K_M | 15.5GB |
| Reasoning | DeepSeek-R1 Distill 14B | Q5_K_M | 14GB |
| Maximum | Qwen 3 32B | Q4_K_M | 15.5GB |

### 24GB VRAM (RTX 3090, RTX 4090)

| Use Case | Model | Quantization | Size |
|----------|-------|--------------|------|
| General | Qwen 3 32B | Q5_K_M | 22GB |
| Coding | DeepSeek-Coder-V2 | Q4_K_M | 23.5GB |
| Reasoning | QwQ 32B | Q5_K_M | 22GB |
| Maximum | Llama 3.3 70B | Q4_K_M | 23.5GB |

## Quantization Guide

| Level | Quality | VRAM Reduction | When to Use |
|-------|---------|----------------|-------------|
| Q4_K_M | 92-95% | 75% | VRAM limited |
| Q5_K_M | 97-99% | 80% | **Recommended** |
| Q6_K | 99%+ | 87% | Quality priority |
| Q8_0 | 99%+ | 50% | Maximum quality |

**Recommendation**: Q5_K_M is the sweet spot for most users.

## Troubleshooting

### Ollama not using GPU

1. Check GPU is detected: `ollama run --verbose`
2. For RDNA 4: Ensure `OLLAMA_VULKAN=1` is set
3. Restart Ollama server after setting env vars

### Model too large for VRAM

1. Use lower quantization (Q4_K_M instead of Q5_K_M)
2. Reduce context length: `/set parameter num_ctx 4096`
3. On systems with iGPU: Ollama will automatically split layers

### AMD GPU not detected (Windows)

For RDNA 4 (RX 9000 series):
- ROCm/HIP not supported yet
- Use Vulkan backend: `set OLLAMA_VULKAN=1`

For RDNA 2/3 (RX 6000/7000 series):
- Install AMD Adrenalin drivers
- May need HSA override: `set HSA_OVERRIDE_GFX_VERSION=11.0.0`

## Verification

To verify GPU acceleration is working:

```batch
set OLLAMA_DEBUG=INFO
set OLLAMA_VULKAN=1
ollama serve
```

Look for output like:
```
library=Vulkan name=Vulkan0 description="AMD Radeon RX 9070 XT" type=discrete total="15.9 GiB"
```

## Sources

- [Ollama GPU Documentation](https://docs.ollama.com/gpu)
- [AMD ROCm Compatibility Matrix](https://rocm.docs.amd.com/en/latest/compatibility/compatibility-matrix.html)
- [Ollama GitHub Issues](https://github.com/ollama/ollama/issues)
