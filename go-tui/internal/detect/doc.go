// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package detect provides GPU detection and model recommendation for rigrun.
//
// This package detects available GPUs on the system, retrieves their specifications,
// and provides diagnostic information for GPU acceleration setup. It also recommends
// appropriate AI models based on available VRAM.
//
// # Key Types
//
//   - GpuInfo: Information about a detected GPU including type, name, and VRAM
//   - GpuType: Enumeration of supported GPU types (NVIDIA, AMD, Apple Silicon, Intel Arc)
//   - ModelRecommendation: Suggested models based on GPU capabilities
//
// # Supported GPU Types
//
//   - NVIDIA (via nvidia-smi)
//   - AMD (via rocm-smi on Linux, WMI/Registry on Windows)
//   - Apple Silicon (via system_profiler on macOS)
//   - Intel Arc (via intel_gpu_top)
//
// # Usage
//
//	ctx := context.Background()
//	gpus, err := detect.DetectGPUs(ctx)
//	if err != nil {
//		log.Fatal(err)
//	}
//	for _, gpu := range gpus {
//		fmt.Printf("%s: %s (%d GB VRAM)\n", gpu.Type, gpu.Name, gpu.VRAM/1024)
//	}
//
//	// Get model recommendations
//	recommendations := detect.RecommendModels(gpus)
package detect
