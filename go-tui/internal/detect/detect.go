// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package detect provides GPU detection utilities for rigrun.
//
// This package detects available GPUs on the system, retrieves their specifications,
// and provides diagnostic information for GPU acceleration setup.
//
// Supported GPU Types:
//   - NVIDIA (via nvidia-smi)
//   - AMD (via rocm-smi on Linux, WMI/Registry on Windows)
//   - Apple Silicon (via system_profiler on macOS)
//   - Intel Arc (via intel_gpu_top)
package detect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// gpuDetectTimeout is the default timeout for GPU detection operations.
// CANCELLATION: Context enables timeout and cancellation
const gpuDetectTimeout = 10 * time.Second

// =============================================================================
// GPU TYPE DEFINITIONS
// =============================================================================

// GpuType represents the type of GPU detected on the system.
type GpuType int

const (
	// GpuTypeCPU indicates no dedicated GPU found, CPU-only mode.
	GpuTypeCPU GpuType = iota
	// GpuTypeNvidia indicates an NVIDIA GPU (CUDA-capable).
	GpuTypeNvidia
	// GpuTypeAmd indicates an AMD GPU (ROCm-capable).
	GpuTypeAmd
	// GpuTypeAppleSilicon indicates Apple Silicon with integrated GPU (Metal-capable).
	GpuTypeAppleSilicon
	// GpuTypeIntel indicates an Intel Arc discrete GPU.
	GpuTypeIntel
)

// String returns the string representation of the GPU type.
func (t GpuType) String() string {
	switch t {
	case GpuTypeNvidia:
		return "NVIDIA"
	case GpuTypeAmd:
		return "AMD"
	case GpuTypeAppleSilicon:
		return "Apple Silicon"
	case GpuTypeIntel:
		return "Intel Arc"
	case GpuTypeCPU:
		return "CPU"
	default:
		return "Unknown"
	}
}

// =============================================================================
// GPU INFO
// =============================================================================

// GpuInfo contains information about a detected GPU.
type GpuInfo struct {
	// Name of the GPU (e.g., "NVIDIA RTX 4090")
	Name string
	// VramGB is the available VRAM in gigabytes
	VramGB uint32
	// Driver version if available
	Driver string
	// Type is the type of GPU
	Type GpuType
}

// String returns a formatted string representation of the GPU info.
func (g *GpuInfo) String() string {
	s := fmt.Sprintf("%s (%dGB VRAM)", g.Name, g.VramGB)
	if g.Driver != "" {
		s += fmt.Sprintf(" [Driver: %s]", g.Driver)
	}
	return s
}

// =============================================================================
// GPU CACHE
// =============================================================================

var (
	gpuCache         *GpuInfo
	gpuCacheTime     time.Time
	gpuCacheMu       sync.Mutex
	gpuCacheDuration = 5 * time.Minute
)

// DetectGPU detects available GPUs on the system.
//
// This function checks for various GPU types in the following order:
//  1. NVIDIA GPUs (via nvidia-smi)
//  2. AMD GPUs (via rocm-smi or Windows methods)
//  3. Apple Silicon (via system_profiler on macOS)
//  4. Intel Arc GPUs (via intel_gpu_top)
//
// If no GPU is found, returns a GpuInfo with Type=GpuTypeCPU.
func DetectGPU() (*GpuInfo, error) {
	return DetectGPUWithContext(context.Background())
}

// DetectGPUWithContext detects available GPUs with context support.
// CANCELLATION: Context enables timeout and cancellation
func DetectGPUWithContext(ctx context.Context) (*GpuInfo, error) {
	// Create timeout context if none is set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, gpuDetectTimeout)
		defer cancel()
	}

	// Try NVIDIA first (most common for ML workloads)
	if info := detectNvidiaWithContext(ctx); info != nil {
		return info, nil
	}

	// Try AMD (ROCm on Linux, WMI/Registry on Windows)
	if info := detectAmdWithContext(ctx); info != nil {
		return info, nil
	}

	// Try Apple Silicon (macOS only)
	if info := DetectAppleSiliconWithContext(ctx); info != nil {
		return info, nil
	}

	// Try Intel Arc
	if info := detectIntelArcWithContext(ctx); info != nil {
		return info, nil
	}

	// No GPU found, fall back to CPU mode
	return GetCPUInfoWithContext(ctx), nil
}

// DetectGPUCached returns cached GPU detection result if available and fresh.
// Otherwise performs full detection and caches the result.
//
// Cache TTL is 5 minutes. This is the preferred function for hot paths
// where GPU detection may be called multiple times.
func DetectGPUCached() (*GpuInfo, error) {
	gpuCacheMu.Lock()
	defer gpuCacheMu.Unlock()

	// Check if cache is still valid
	if gpuCache != nil && time.Since(gpuCacheTime) < gpuCacheDuration {
		return gpuCache, nil
	}

	// Cache miss or expired - perform fresh detection
	info, err := DetectGPU()
	if err != nil {
		return nil, err
	}

	// Update cache
	gpuCache = info
	gpuCacheTime = time.Now()

	return info, nil
}

// ClearGPUCache clears the GPU detection cache, forcing fresh detection on next call.
// Useful when hardware configuration may have changed (e.g., driver update).
func ClearGPUCache() {
	gpuCacheMu.Lock()
	defer gpuCacheMu.Unlock()
	gpuCache = nil
	gpuCacheTime = time.Time{}
}

// =============================================================================
// NVIDIA DETECTION
// =============================================================================

// detectNvidia detects NVIDIA GPUs using nvidia-smi.
func detectNvidia() *GpuInfo {
	return detectNvidiaWithContext(context.Background())
}

// detectNvidiaWithContext detects NVIDIA GPUs with context support.
// CANCELLATION: Context enables timeout and cancellation
func detectNvidiaWithContext(ctx context.Context) *GpuInfo {
	// Try nvidia-smi command locations in order
	paths := getNvidiaSmiPaths()

	var output []byte
	var err error

	for _, path := range paths {
		cmd := exec.CommandContext(ctx, path,
			"--query-gpu=name,memory.total,driver_version",
			"--format=csv,noheader,nounits")
		output, err = cmd.Output()
		if err == nil {
			break
		}
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}

	if err != nil || len(output) == 0 {
		return nil
	}

	// Parse output
	stdout := strings.TrimSpace(string(output))
	line := strings.Split(stdout, "\n")[0]
	line = strings.TrimSpace(line)

	// nvidia-smi outputs CSV with ", " as delimiter
	parts := strings.Split(line, ", ")
	if len(parts) < 3 {
		return nil
	}

	name := "NVIDIA " + strings.TrimSpace(parts[0])

	// Memory is in MiB, convert to GB
	vramMB, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return nil
	}
	vramGB := uint32(vramMB/1024.0 + 0.5) // Round

	driver := strings.TrimSpace(parts[2])

	return &GpuInfo{
		Name:   name,
		VramGB: vramGB,
		Driver: driver,
		Type:   GpuTypeNvidia,
	}
}

// getNvidiaSmiPaths returns possible paths for nvidia-smi based on OS.
func getNvidiaSmiPaths() []string {
	if runtime.GOOS == "windows" {
		return []string{
			"nvidia-smi",
			`C:\Windows\System32\nvidia-smi.exe`,
			`C:\Program Files\NVIDIA Corporation\NVSMI\nvidia-smi.exe`,
		}
	}
	return []string{"nvidia-smi"}
}

// =============================================================================
// AMD DETECTION
// =============================================================================

// detectAmd detects AMD GPUs.
func detectAmd() *GpuInfo {
	return detectAmdWithContext(context.Background())
}

// detectAmdWithContext detects AMD GPUs with context support.
// CANCELLATION: Context enables timeout and cancellation
func detectAmdWithContext(ctx context.Context) *GpuInfo {
	if runtime.GOOS == "windows" {
		return detectAmdWindowsWithContext(ctx)
	}
	return detectAmdLinuxWithContext(ctx)
}

// detectAmdLinux detects AMD GPUs using rocm-smi on Linux.
func detectAmdLinux() *GpuInfo {
	return detectAmdLinuxWithContext(context.Background())
}

// detectAmdLinuxWithContext detects AMD GPUs using rocm-smi with context support.
// CANCELLATION: Context enables timeout and cancellation
func detectAmdLinuxWithContext(ctx context.Context) *GpuInfo {
	cmd := exec.CommandContext(ctx, "rocm-smi", "--showproductname", "--showmeminfo", "vram")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	stdout := string(output)

	// Parse GPU name
	name := "AMD GPU"
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "Card series:") || strings.Contains(line, "GPU") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 1 {
				name = "AMD " + strings.TrimSpace(parts[1])
				break
			}
		}
	}

	// Parse VRAM using pre-compiled amdNumericRegex (defined in amd.go)
	vramGB := uint32(8) // Default
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "Total Memory") || strings.Contains(line, "VRAM Total") {
			// Extract numeric value using pre-compiled regex
			matches := amdNumericRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				val, err := strconv.ParseUint(matches[1], 10, 64)
				if err == nil {
					if val > 1_000_000_000 {
						// Bytes to GB
						vramGB = uint32(val / 1_073_741_824)
					} else if val > 1_000_000 {
						// MB to GB
						vramGB = uint32(val / 1024)
					} else {
						// Already in GB
						vramGB = uint32(val)
					}
				}
			}
			break
		}
	}

	// Try to get driver version
	driver := ""
	cmd = exec.CommandContext(ctx, "rocm-smi", "--showdriverversion")
	if output, err := cmd.Output(); err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			if strings.Contains(line, "Driver version") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) > 1 {
					driver = strings.TrimSpace(parts[1])
				}
				break
			}
		}
	}

	return &GpuInfo{
		Name:   name,
		VramGB: vramGB,
		Driver: driver,
		Type:   GpuTypeAmd,
	}
}

// detectAmdWindows detects AMD GPUs on Windows using WMI and registry.
func detectAmdWindows() *GpuInfo {
	return detectAmdWindowsWithContext(context.Background())
}

// detectAmdWindowsWithContext detects AMD GPUs on Windows with context support.
// CANCELLATION: Context enables timeout and cancellation
func detectAmdWindowsWithContext(ctx context.Context) *GpuInfo {
	// Get GPU name using PowerShell
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
		`$gpu = Get-CimInstance Win32_VideoController | Where-Object { $_.Name -like '*AMD*' -or $_.Name -like '*Radeon*' } | Select-Object -First 1; if ($gpu) { $gpu.Name }`)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	gpuName := strings.TrimSpace(string(output))
	if gpuName == "" {
		return nil
	}

	// Try to get VRAM from registry (more accurate for high-end GPUs)
	vramGB := getAmdVramFromRegistryWithContext(ctx)

	// If registry didn't work, try WMI
	if vramGB == 0 {
		vramGB = getAmdVramFromWMIWithContext(ctx)
	}

	// If still 0, try to infer from model name
	if vramGB == 0 {
		vramGB = inferAmdVramFromModel(gpuName)
	}

	// Default fallback
	if vramGB == 0 {
		vramGB = 8
	}

	// Validate - if WMI returned 4GB or less for a high-end GPU, it's likely wrong
	if vramGB <= 4 {
		if inferred := inferAmdVramFromModel(gpuName); inferred > vramGB {
			vramGB = inferred
		}
	}

	// Get driver version
	driver := ""
	cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
		`$gpu = Get-CimInstance Win32_VideoController | Where-Object { $_.Name -like '*AMD*' -or $_.Name -like '*Radeon*' } | Select-Object -First 1; if ($gpu) { $gpu.DriverVersion }`)
	if output, err := cmd.Output(); err == nil {
		driver = strings.TrimSpace(string(output))
	}

	return &GpuInfo{
		Name:   gpuName,
		VramGB: vramGB,
		Driver: driver,
		Type:   GpuTypeAmd,
	}
}

// getAmdVramFromRegistry tries to get AMD VRAM from Windows registry.
func getAmdVramFromRegistry() uint32 {
	return getAmdVramFromRegistryWithContext(context.Background())
}

// getAmdVramFromRegistryWithContext tries to get AMD VRAM from Windows registry with context support.
// CANCELLATION: Context enables timeout and cancellation
func getAmdVramFromRegistryWithContext(ctx context.Context) uint32 {
	script := `
try {
    $paths = @(
        "HKLM:\SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}\0000",
        "HKLM:\SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}\0001",
        "HKLM:\SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}\0002"
    )
    foreach ($path in $paths) {
        if (Test-Path $path) {
            $desc = (Get-ItemProperty -Path $path -Name "DriverDesc" -ErrorAction SilentlyContinue).DriverDesc
            if ($desc -like "*AMD*" -or $desc -like "*Radeon*") {
                $mem = (Get-ItemProperty -Path $path -Name "HardwareInformation.qwMemorySize" -ErrorAction SilentlyContinue)."HardwareInformation.qwMemorySize"
                if ($mem) {
                    $vramGB = [Math]::Round($mem / 1GB, 0)
                    Write-Output $vramGB
                    exit
                }
            }
        }
    }
    Write-Output "0"
} catch {
    Write-Output "0"
}`

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script)
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	val, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 32)
	if err != nil {
		return 0
	}
	return uint32(val)
}

// getAmdVramFromWMI tries to get AMD VRAM from WMI (has 4GB limitation).
func getAmdVramFromWMI() uint32 {
	return getAmdVramFromWMIWithContext(context.Background())
}

// getAmdVramFromWMIWithContext tries to get AMD VRAM from WMI with context support.
// CANCELLATION: Context enables timeout and cancellation
func getAmdVramFromWMIWithContext(ctx context.Context) uint32 {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
		`$gpu = Get-CimInstance Win32_VideoController | Where-Object { $_.Name -like '*AMD*' -or $_.Name -like '*Radeon*' } | Select-Object -First 1; if ($gpu -and $gpu.AdapterRAM) { [Math]::Round($gpu.AdapterRAM / 1GB, 0) }`)
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	val, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 32)
	if err != nil {
		return 0
	}
	return uint32(val)
}

// inferAmdVramFromModel infers VRAM size from AMD GPU model name.
func inferAmdVramFromModel(gpuName string) uint32 {
	name := strings.ToLower(gpuName)

	// RX 9000 series (RDNA 4) - 2025
	if strings.Contains(name, "9070 xt") {
		return 16
	} else if strings.Contains(name, "9070") {
		return 12
	}

	// RX 7000 series (RDNA 3)
	if strings.Contains(name, "7900 xtx") {
		return 24
	} else if strings.Contains(name, "7900 xt") || strings.Contains(name, "7900 gre") {
		return 20
	} else if strings.Contains(name, "7800 xt") {
		return 16
	} else if strings.Contains(name, "7700 xt") {
		return 12
	} else if strings.Contains(name, "7600 xt") {
		return 16
	} else if strings.Contains(name, "7600") {
		return 8
	}

	// Pro series (Workstation)
	if strings.Contains(name, "pro w7900") {
		return 48
	} else if strings.Contains(name, "pro w7800") {
		return 32
	} else if strings.Contains(name, "pro w7700") {
		return 16
	} else if strings.Contains(name, "pro w6800") {
		return 32
	}

	// RX 6000 series (RDNA 2)
	if strings.Contains(name, "6950 xt") || strings.Contains(name, "6900 xt") ||
		strings.Contains(name, "6800 xt") || strings.Contains(name, "6800") {
		return 16
	} else if strings.Contains(name, "6750 xt") || strings.Contains(name, "6700 xt") {
		return 12
	} else if strings.Contains(name, "6700") || strings.Contains(name, "6650 xt") {
		return 10
	} else if strings.Contains(name, "6600 xt") || strings.Contains(name, "6600") {
		return 8
	} else if strings.Contains(name, "6500 xt") || strings.Contains(name, "6400") {
		return 4
	}

	// RX 5000 series (RDNA 1)
	if strings.Contains(name, "5700 xt") || strings.Contains(name, "5700") {
		return 8
	} else if strings.Contains(name, "5600 xt") || strings.Contains(name, "5600") {
		return 6
	} else if strings.Contains(name, "5500 xt") {
		return 8
	}

	// Radeon VII / Vega
	if strings.Contains(name, "radeon vii") || strings.Contains(name, "radeon 7") {
		return 16
	}
	if strings.Contains(name, "vega 64") || strings.Contains(name, "vega 56") {
		return 8
	}

	return 0
}

// =============================================================================
// APPLE SILICON DETECTION
// =============================================================================

// DetectAppleSilicon detects Apple Silicon GPU on macOS.
//
// Uses system_profiler to detect Apple Silicon chips and their unified memory.
// Returns nil if not on macOS or no Apple Silicon is detected.
func DetectAppleSilicon() *GpuInfo {
	return DetectAppleSiliconWithContext(context.Background())
}

// DetectAppleSiliconWithContext detects Apple Silicon GPU with context support.
// CANCELLATION: Context enables timeout and cancellation
func DetectAppleSiliconWithContext(ctx context.Context) *GpuInfo {
	if runtime.GOOS != "darwin" {
		return nil
	}

	cmd := exec.CommandContext(ctx, "system_profiler", "SPDisplaysDataType", "-json")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	stdout := string(output)

	// Check if it's Apple Silicon
	if !strings.Contains(stdout, "Apple") {
		return nil
	}

	// Parse chip name
	name := "Apple Silicon"
	chipNames := []string{
		"M4 Ultra", "M4 Max", "M4 Pro", "M4",
		"M3 Ultra", "M3 Max", "M3 Pro", "M3",
		"M2 Ultra", "M2 Max", "M2 Pro", "M2",
		"M1 Ultra", "M1 Max", "M1 Pro", "M1",
	}
	for _, chip := range chipNames {
		if strings.Contains(stdout, chip) {
			name = "Apple " + chip
			break
		}
	}

	// Get unified memory using sysctl
	vramGB := uint32(8) // Default
	cmd = exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize")
	if output, err := cmd.Output(); err == nil {
		if bytes, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64); err == nil {
			// Apple Silicon can use up to ~75% of unified memory for GPU
			// Report the full memory as it's shared
			vramGB = uint32(bytes / 1_073_741_824)
		}
	}

	// Get macOS version as "driver"
	driver := ""
	cmd = exec.CommandContext(ctx, "sw_vers", "-productVersion")
	if output, err := cmd.Output(); err == nil {
		driver = "macOS " + strings.TrimSpace(string(output))
	}

	return &GpuInfo{
		Name:   name,
		VramGB: vramGB,
		Driver: driver,
		Type:   GpuTypeAppleSilicon,
	}
}

// =============================================================================
// INTEL ARC DETECTION
// =============================================================================

// detectIntelArc detects Intel Arc GPUs.
func detectIntelArc() *GpuInfo {
	return detectIntelArcWithContext(context.Background())
}

// detectIntelArcWithContext detects Intel Arc GPUs with context support.
// CANCELLATION: Context enables timeout and cancellation
func detectIntelArcWithContext(ctx context.Context) *GpuInfo {
	cmd := exec.CommandContext(ctx, "intel_gpu_top", "-L")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	stdout := strings.ToLower(string(output))

	// Check if it's an Arc GPU (discrete)
	if !strings.Contains(stdout, "arc") {
		return nil
	}

	// Parse GPU name and estimate VRAM
	name := "Intel Arc"
	vramGB := uint32(8) // Default

	if strings.Contains(stdout, "a770") {
		name = "Intel Arc A770"
		vramGB = 16
	} else if strings.Contains(stdout, "a750") {
		name = "Intel Arc A750"
		vramGB = 8
	} else if strings.Contains(stdout, "a580") {
		name = "Intel Arc A580"
		vramGB = 8
	} else if strings.Contains(stdout, "a380") {
		name = "Intel Arc A380"
		vramGB = 6
	} else if strings.Contains(stdout, "a310") {
		name = "Intel Arc A310"
		vramGB = 4
	}

	return &GpuInfo{
		Name:   name,
		VramGB: vramGB,
		Type:   GpuTypeIntel,
	}
}

// =============================================================================
// CPU FALLBACK
// =============================================================================

// GetCPUInfo returns GpuInfo for CPU-only mode.
// Estimates "VRAM" based on system RAM.
func GetCPUInfo() *GpuInfo {
	return GetCPUInfoWithContext(context.Background())
}

// GetCPUInfoWithContext returns GpuInfo for CPU-only mode with context support.
// CANCELLATION: Context enables timeout and cancellation
func GetCPUInfoWithContext(ctx context.Context) *GpuInfo {
	vramGB := uint32(0)

	// Try to get system RAM
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize")
		if output, err := cmd.Output(); err == nil {
			if bytes, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64); err == nil {
				// Use 50% of system RAM as "available" for CPU inference
				vramGB = uint32(bytes / 1_073_741_824 / 2)
			}
		}
	case "linux":
		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "MemTotal:") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						if kb, err := strconv.ParseUint(parts[1], 10, 64); err == nil {
							// Convert KB to GB, use 50%
							vramGB = uint32(kb / 1024 / 1024 / 2)
						}
					}
					break
				}
			}
		}
	case "windows":
		cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
			`[Math]::Round((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1GB / 2, 0)`)
		if output, err := cmd.Output(); err == nil {
			if val, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 32); err == nil {
				vramGB = uint32(val)
			}
		}
	}

	if vramGB == 0 {
		vramGB = 4 // Minimum fallback
	}

	return &GpuInfo{
		Name:   "CPU Only",
		VramGB: vramGB,
		Type:   GpuTypeCPU,
	}
}

// =============================================================================
// GPU STATUS REPORT
// =============================================================================

// GPUStatusReport contains comprehensive GPU status information.
type GPUStatusReport struct {
	// GPU is the detected GPU information
	GPU *GpuInfo
	// MemoryUsed in MB
	MemoryUsed int
	// MemoryTotal in MB
	MemoryTotal int
	// MemoryPercent is the percentage of VRAM used (0-100)
	MemoryPercent float64
	// IsAvailable indicates if GPU acceleration is available
	IsAvailable bool
	// Warnings contains any issues detected with GPU setup
	Warnings []string
}

// GetGPUStatusReport returns a comprehensive GPU status report.
func GetGPUStatusReport() GPUStatusReport {
	gpu, _ := DetectGPUCached()
	if gpu == nil {
		gpu = GetCPUInfo()
	}

	report := GPUStatusReport{
		GPU:         gpu,
		IsAvailable: gpu.Type != GpuTypeCPU,
		Warnings:    make([]string, 0),
	}

	// Get memory usage based on GPU type
	switch gpu.Type {
	case GpuTypeNvidia:
		usage := getNvidiaMemoryUsage()
		if usage != nil {
			report.MemoryUsed = usage.UsedMB
			report.MemoryTotal = usage.TotalMB
			if usage.TotalMB > 0 {
				report.MemoryPercent = float64(usage.UsedMB) / float64(usage.TotalMB) * 100
			}
		}
	case GpuTypeAmd:
		// AMD memory usage is harder to get reliably
		report.MemoryTotal = int(gpu.VramGB * 1024)
	default:
		report.MemoryTotal = int(gpu.VramGB * 1024)
	}

	// Check for issues
	report.Warnings = append(report.Warnings, DiagnoseCPUFallback()...)

	// Check VRAM usage
	if report.MemoryPercent > 90 {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("VRAM is nearly full (%.1f%% used) - may cause slowdowns", report.MemoryPercent))
	}

	return report
}

// GpuMemoryUsage contains real-time GPU memory usage.
type GpuMemoryUsage struct {
	TotalMB        int
	UsedMB         int
	FreeMB         int
	GPUUtilization int // 0-100
}

// getNvidiaMemoryUsage gets real-time VRAM usage from nvidia-smi.
func getNvidiaMemoryUsage() *GpuMemoryUsage {
	paths := getNvidiaSmiPaths()

	var output []byte
	var err error

	for _, path := range paths {
		cmd := exec.Command(path,
			"--query-gpu=memory.total,memory.used,memory.free,utilization.gpu",
			"--format=csv,noheader,nounits")
		output, err = cmd.Output()
		if err == nil {
			break
		}
	}

	if err != nil {
		return nil
	}

	line := strings.TrimSpace(string(output))
	line = strings.Split(line, "\n")[0]
	parts := strings.Split(line, ", ")

	if len(parts) < 3 {
		return nil
	}

	total, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	used, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	free, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
	util := 0
	if len(parts) > 3 {
		util, _ = strconv.Atoi(strings.TrimSpace(parts[3]))
	}

	return &GpuMemoryUsage{
		TotalMB:        total,
		UsedMB:         used,
		FreeMB:         free,
		GPUUtilization: util,
	}
}

// =============================================================================
// CPU FALLBACK DIAGNOSIS
// =============================================================================

// DiagnoseCPUFallback returns reasons why GPU might not be detected.
// Checks for common issues that cause CPU fallback.
func DiagnoseCPUFallback() []string {
	warnings := make([]string, 0)

	gpu, _ := DetectGPUCached()
	if gpu != nil && gpu.Type != GpuTypeCPU {
		// GPU is detected, no fallback
		return warnings
	}

	// Check for NVIDIA
	if !checkNvidiaSmiAvailable() {
		// Check if NVIDIA driver might be installed but nvidia-smi not in PATH
		if runtime.GOOS == "windows" {
			nvidiaDriverPath := `C:\Windows\System32\nvidia-smi.exe`
			if _, err := os.Stat(nvidiaDriverPath); os.IsNotExist(err) {
				warnings = append(warnings, "NVIDIA: nvidia-smi not found - NVIDIA drivers may not be installed")
			} else {
				warnings = append(warnings, "NVIDIA: nvidia-smi found but not in PATH")
			}
		} else {
			warnings = append(warnings, "NVIDIA: nvidia-smi not in PATH - NVIDIA drivers may not be installed")
		}
	}

	// Check for AMD/ROCm
	if runtime.GOOS == "linux" {
		if !checkRocmInstalled() {
			warnings = append(warnings, "AMD: ROCm not installed - run 'amdgpu-install --usecase=rocm' to enable AMD GPU support")
		}
	} else if runtime.GOOS == "windows" {
		if !checkHipInstalled() {
			warnings = append(warnings, "AMD: HIP SDK not detected - download from AMD developer portal for AMD GPU support")
		}
	}

	// Check for Intel Arc
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("intel_gpu_top"); err != nil {
			warnings = append(warnings, "Intel: intel_gpu_top not found - Intel GPU tools may not be installed")
		}
	}

	if len(warnings) == 0 {
		warnings = append(warnings, "No GPU detected - ensure GPU drivers are properly installed")
	}

	return warnings
}

// checkNvidiaSmiAvailable checks if nvidia-smi is available and working.
func checkNvidiaSmiAvailable() bool {
	paths := getNvidiaSmiPaths()
	for _, path := range paths {
		cmd := exec.Command(path, "--version")
		if err := cmd.Run(); err == nil {
			return true
		}
	}
	return false
}

// checkRocmInstalled checks if ROCm is installed on Linux.
func checkRocmInstalled() bool {
	// Check for rocm-smi
	if _, err := exec.LookPath("rocm-smi"); err == nil {
		return true
	}
	// Check for ROCm directory
	if _, err := os.Stat("/opt/rocm"); err == nil {
		return true
	}
	return false
}

// checkHipInstalled checks if HIP SDK is installed on Windows.
func checkHipInstalled() bool {
	// Check HIP_PATH environment variable
	if hipPath := os.Getenv("HIP_PATH"); hipPath != "" {
		if _, err := os.Stat(hipPath); err == nil {
			return true
		}
	}
	// Check common installation paths
	hipPaths := []string{
		`C:\Program Files\AMD\ROCm`,
		`C:\Program Files\AMD\HIP SDK`,
	}
	for _, path := range hipPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// =============================================================================
// OLLAMA INTEGRATION
// =============================================================================

// OllamaLoadedModel represents a model loaded in Ollama.
type OllamaLoadedModel struct {
	Name      string
	ID        string
	Size      string
	SizeBytes uint64
	Processor string
	Until     string
}

// GetOllamaLoadedModels retrieves models currently loaded in Ollama.
// Calls the Ollama API to get information about loaded models.
func GetOllamaLoadedModels() ([]string, error) {
	// Try using the API first (more reliable)
	models, err := getOllamaLoadedModelsFromAPI()
	if err == nil {
		return models, nil
	}

	// Fall back to CLI
	return getOllamaLoadedModelsFromCLI()
}

// getOllamaLoadedModelsFromAPI gets loaded models via Ollama HTTP API.
func getOllamaLoadedModelsFromAPI() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:11434/api/ps", nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama API returned status %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]string, len(result.Models))
	for i, m := range result.Models {
		models[i] = m.Name
	}

	return models, nil
}

// getOllamaLoadedModelsFromCLI gets loaded models via ollama ps command.
func getOllamaLoadedModelsFromCLI() ([]string, error) {
	cmd := exec.Command("ollama", "ps")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	models := make([]string, 0)

	// Skip header line
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// First column is the model name
		parts := strings.Fields(line)
		if len(parts) > 0 {
			models = append(models, parts[0])
		}
	}

	return models, nil
}

// =============================================================================
// AMD ARCHITECTURE DETECTION
// =============================================================================

// AmdArchitecture represents AMD GPU architecture generation.
type AmdArchitecture int

const (
	AmdArchUnknown AmdArchitecture = iota
	AmdArchVega                    // Vega (gfx900/gfx906)
	AmdArchRdna1                   // RDNA 1 (RX 5000 series, gfx1010/gfx1011/gfx1012)
	AmdArchRdna2                   // RDNA 2 (RX 6000 series, gfx1030/gfx1031/gfx1032)
	AmdArchRdna3                   // RDNA 3 (RX 7000 series, gfx1100/gfx1101/gfx1102)
	AmdArchRdna4                   // RDNA 4 (RX 9000 series, gfx1200/gfx1201)
)

func (a AmdArchitecture) String() string {
	switch a {
	case AmdArchVega:
		return "Vega"
	case AmdArchRdna1:
		return "RDNA 1"
	case AmdArchRdna2:
		return "RDNA 2"
	case AmdArchRdna3:
		return "RDNA 3"
	case AmdArchRdna4:
		return "RDNA 4"
	default:
		return "Unknown"
	}
}

// DetectAmdArchitecture detects AMD GPU architecture from GPU name.
func DetectAmdArchitecture(gpuName string) AmdArchitecture {
	name := strings.ToLower(gpuName)

	// RDNA 4 (RX 9000 series)
	if strings.Contains(name, "9070") || strings.Contains(name, "9080") || strings.Contains(name, "9090") ||
		strings.Contains(name, "gfx1200") || strings.Contains(name, "gfx1201") {
		return AmdArchRdna4
	}

	// RDNA 3 (RX 7000 series)
	if strings.Contains(name, "7900") || strings.Contains(name, "7800") || strings.Contains(name, "7700") ||
		strings.Contains(name, "7600") || strings.Contains(name, "7500") ||
		strings.Contains(name, "gfx1100") || strings.Contains(name, "gfx1101") || strings.Contains(name, "gfx1102") {
		return AmdArchRdna3
	}

	// RDNA 2 (RX 6000 series)
	if strings.Contains(name, "6900") || strings.Contains(name, "6800") || strings.Contains(name, "6700") ||
		strings.Contains(name, "6600") || strings.Contains(name, "6500") || strings.Contains(name, "6400") ||
		strings.Contains(name, "gfx1030") || strings.Contains(name, "gfx1031") || strings.Contains(name, "gfx1032") {
		return AmdArchRdna2
	}

	// RDNA 1 (RX 5000 series)
	if strings.Contains(name, "5700") || strings.Contains(name, "5600") || strings.Contains(name, "5500") ||
		strings.Contains(name, "gfx1010") || strings.Contains(name, "gfx1011") || strings.Contains(name, "gfx1012") {
		return AmdArchRdna1
	}

	// Vega
	if strings.Contains(name, "vega") || strings.Contains(name, "radeon vii") ||
		strings.Contains(name, "gfx900") || strings.Contains(name, "gfx906") {
		return AmdArchVega
	}

	return AmdArchUnknown
}

// GetAmdSetupHint returns setup hints for AMD GPUs based on architecture.
func GetAmdSetupHint(gpuName string) string {
	arch := DetectAmdArchitecture(gpuName)

	switch arch {
	case AmdArchRdna4:
		return "RDNA 4 GPU detected. Set OLLAMA_VULKAN=1 for GPU acceleration."
	case AmdArchRdna3, AmdArchRdna2:
		return "Ensure ROCm is installed for GPU acceleration."
	case AmdArchRdna1, AmdArchVega:
		version := getHsaOverrideVersion(arch)
		if version != "" {
			return fmt.Sprintf("Try setting HSA_OVERRIDE_GFX_VERSION=%s for GPU support.", version)
		}
	}

	return ""
}

// getHsaOverrideVersion returns recommended HSA_OVERRIDE_GFX_VERSION for architecture.
func getHsaOverrideVersion(arch AmdArchitecture) string {
	switch arch {
	case AmdArchRdna4:
		return "11.0.0"
	case AmdArchRdna1:
		return "10.1.0"
	case AmdArchVega:
		return "9.0.0"
	default:
		return ""
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// CheckOllamaAvailable checks if Ollama CLI is available.
func CheckOllamaAvailable() bool {
	cmd := exec.Command("ollama", "--version")
	return cmd.Run() == nil
}

// CheckOllamaRunning checks if Ollama server is running.
func CheckOllamaRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use explicit IPv4 address to avoid IPv6 resolution issues on Windows
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:11434", nil)
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GetNvidiaSmiPath returns the path to nvidia-smi if found.
func GetNvidiaSmiPath() string {
	paths := getNvidiaSmiPaths()
	for _, path := range paths {
		// Check if it's just a command name (will be found via PATH)
		if !filepath.IsAbs(path) {
			if foundPath, err := exec.LookPath(path); err == nil {
				return foundPath
			}
		} else {
			// It's an absolute path, check if file exists
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}
