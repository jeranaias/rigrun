// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package detect provides GPU detection functionality for rigrun.
// This file contains additional AMD-specific utilities extending the base detection in detect.go.
package detect

import (
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// amdDetectTimeout is the default timeout for AMD detection operations.
// CANCELLATION: Context enables timeout and cancellation
const amdDetectTimeout = 10 * time.Second

// =============================================================================
// PERFORMANCE: Pre-compiled regex (compiled once at startup)
// =============================================================================

var (
	// Numeric extraction pattern for AMD memory parsing
	amdNumericRegex = regexp.MustCompile(`(\d+)`)
)

// =============================================================================
// AMD MEMORY USAGE
// =============================================================================

// AmdMemoryInfo contains detailed AMD GPU memory information.
type AmdMemoryInfo struct {
	UsedMB       int     // Used memory in MB
	TotalMB      int     // Total memory in MB
	FreeMB       int     // Free memory in MB
	UsagePercent float64 // Memory usage as a percentage
}

// GetAMDMemoryUsage retrieves current AMD GPU memory usage.
// Returns usedMB, totalMB, and an error if detection fails.
func GetAMDMemoryUsage() (usedMB, totalMB int, err error) {
	return GetAMDMemoryUsageWithContext(context.Background())
}

// GetAMDMemoryUsageWithContext retrieves current AMD GPU memory usage with context support.
// CANCELLATION: Context enables timeout and cancellation
func GetAMDMemoryUsageWithContext(ctx context.Context) (usedMB, totalMB int, err error) {
	// Create timeout context if none is set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, amdDetectTimeout)
		defer cancel()
	}

	if runtime.GOOS == "windows" {
		return getAmdMemoryUsageWindowsWithContext(ctx)
	}
	return getAmdMemoryUsageLinuxWithContext(ctx)
}

// getAmdMemoryUsageLinux gets AMD memory usage on Linux using rocm-smi.
func getAmdMemoryUsageLinux() (usedMB, totalMB int, err error) {
	return getAmdMemoryUsageLinuxWithContext(context.Background())
}

// getAmdMemoryUsageLinuxWithContext gets AMD memory usage on Linux with context support.
// CANCELLATION: Context enables timeout and cancellation
func getAmdMemoryUsageLinuxWithContext(ctx context.Context) (usedMB, totalMB int, err error) {
	cmd := exec.CommandContext(ctx, "rocm-smi", "--showmeminfo", "vram")
	output, err := cmd.Output()
	if err != nil {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return 0, 0, ctx.Err()
		default:
		}
		return 0, 0, err
	}

	stdout := string(output)
	lines := strings.Split(stdout, "\n")

	for _, line := range lines {
		lineLower := strings.ToLower(line)

		// Parse total VRAM using pre-compiled amdNumericRegex
		if strings.Contains(lineLower, "total") {
			matches := amdNumericRegex.FindAllString(line, -1)
			for _, match := range matches {
				val, parseErr := strconv.Atoi(match)
				if parseErr == nil && val > 0 {
					if val > 1000000000 {
						totalMB = val / (1024 * 1024) // Bytes to MB
					} else if val > 1000000 {
						totalMB = val / 1024 // KB to MB
					} else {
						totalMB = val // Already in MB
					}
					break
				}
			}
		}

		// Parse used VRAM using pre-compiled amdNumericRegex
		if strings.Contains(lineLower, "used") {
			matches := amdNumericRegex.FindAllString(line, -1)
			for _, match := range matches {
				val, parseErr := strconv.Atoi(match)
				if parseErr == nil && val > 0 {
					if val > 1000000000 {
						usedMB = val / (1024 * 1024) // Bytes to MB
					} else if val > 1000000 {
						usedMB = val / 1024 // KB to MB
					} else {
						usedMB = val // Already in MB
					}
					break
				}
			}
		}
	}

	return usedMB, totalMB, nil
}

// getAmdMemoryUsageWindows gets AMD memory usage on Windows.
// Note: AMD doesn't provide real-time memory usage easily on Windows.
func getAmdMemoryUsageWindows() (usedMB, totalMB int, err error) {
	return getAmdMemoryUsageWindowsWithContext(context.Background())
}

// getAmdMemoryUsageWindowsWithContext gets AMD memory usage on Windows with context support.
// CANCELLATION: Context enables timeout and cancellation
func getAmdMemoryUsageWindowsWithContext(ctx context.Context) (usedMB, totalMB int, err error) {
	psScript := `
$gpu = Get-CimInstance Win32_VideoController | Where-Object { $_.Name -like '*AMD*' -or $_.Name -like '*Radeon*' } | Select-Object -First 1
if ($gpu) {
    $totalMB = [Math]::Round($gpu.AdapterRAM / 1MB, 0)
    Write-Output "$totalMB,0"
}
`
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", psScript)
	output, err := cmd.Output()
	if err != nil {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return 0, 0, ctx.Err()
		default:
		}
		return 0, 0, err
	}

	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) < 2 {
		return 0, 0, nil
	}

	total, _ := strconv.Atoi(parts[0])
	used, _ := strconv.Atoi(parts[1]) // Used is typically 0 on Windows

	return used, total, nil
}

// GetAMDDetailedMemory retrieves detailed AMD GPU memory information.
func GetAMDDetailedMemory() (*AmdMemoryInfo, error) {
	usedMB, totalMB, err := GetAMDMemoryUsage()
	if err != nil {
		return nil, err
	}

	freeMB := totalMB - usedMB
	if freeMB < 0 {
		freeMB = 0
	}

	var usagePercent float64
	if totalMB > 0 {
		usagePercent = float64(usedMB) / float64(totalMB) * 100.0
	}

	return &AmdMemoryInfo{
		UsedMB:       usedMB,
		TotalMB:      totalMB,
		FreeMB:       freeMB,
		UsagePercent: usagePercent,
	}, nil
}

// =============================================================================
// HSA OVERRIDE
// =============================================================================

// CheckHSAOverride checks if HSA_OVERRIDE_GFX_VERSION is set.
// This environment variable is important for ROCm compatibility with unsupported GPUs.
// Returns the value and whether it's set.
func CheckHSAOverride() (string, bool) {
	value := os.Getenv("HSA_OVERRIDE_GFX_VERSION")
	return value, value != ""
}

// GetRecommendedHSAOverride returns the recommended HSA_OVERRIDE_GFX_VERSION for a GPU.
// This helps unsupported AMD GPUs work with ROCm/Ollama.
func GetRecommendedHSAOverride(gpuName string) (string, bool) {
	arch := DetectAmdArchitecture(gpuName)

	switch arch {
	case AmdArchRdna4:
		// RDNA 4 needs gfx1100 override until ROCm adds native support
		return "11.0.0", true

	case AmdArchRdna3:
		nameLower := strings.ToLower(gpuName)
		// RX 7600 and some variants may need override
		if strings.Contains(nameLower, "7600") || strings.Contains(nameLower, "7700") {
			return "11.0.0", true
		}
		return "", false

	case AmdArchRdna2:
		nameLower := strings.ToLower(gpuName)
		// Lower-end RDNA 2 may need override
		if strings.Contains(nameLower, "6600") || strings.Contains(nameLower, "6500") ||
			strings.Contains(nameLower, "6400") {
			return "10.3.0", true
		}
		return "", false

	case AmdArchRdna1:
		// RDNA 1 may need gfx1010 override
		return "10.1.0", true

	case AmdArchVega:
		// Vega may need gfx900 override
		return "9.0.0", true

	default:
		return "", false
	}
}

// =============================================================================
// GFX VERSION DETECTION
// =============================================================================

// DetectAMDGfxVersion returns the GFX version string for a GPU.
// This is useful for ROCm compatibility and Vulkan backend selection.
func DetectAMDGfxVersion(gpuName string) string {
	arch := DetectAmdArchitecture(gpuName)
	nameLower := strings.ToLower(gpuName)

	switch arch {
	case AmdArchRdna4:
		return "gfx1200"

	case AmdArchRdna3:
		// Different RDNA 3 chips have different GFX versions
		if strings.Contains(nameLower, "7900") {
			return "gfx1100"
		} else if strings.Contains(nameLower, "7800") || strings.Contains(nameLower, "7700") {
			return "gfx1101"
		} else if strings.Contains(nameLower, "7600") {
			return "gfx1102"
		}
		return "gfx1100"

	case AmdArchRdna2:
		// Different RDNA 2 chips
		if strings.Contains(nameLower, "6900") || strings.Contains(nameLower, "6800") {
			return "gfx1030"
		} else if strings.Contains(nameLower, "6700") {
			return "gfx1031"
		}
		return "gfx1032"

	case AmdArchRdna1:
		return "gfx1010"

	case AmdArchVega:
		if strings.Contains(nameLower, "radeon vii") || strings.Contains(nameLower, "radeon 7") {
			return "gfx906"
		}
		return "gfx900"

	default:
		return "unknown"
	}
}

// =============================================================================
// PUBLIC AVAILABILITY CHECKS
// =============================================================================

// IsAMDAvailable checks if AMD GPU detection tools are available.
func IsAMDAvailable() bool {
	return IsAMDAvailableWithContext(context.Background())
}

// IsAMDAvailableWithContext checks if AMD GPU detection tools are available with context support.
// CANCELLATION: Context enables timeout and cancellation
func IsAMDAvailableWithContext(ctx context.Context) bool {
	// Create timeout context if none is set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, amdDetectTimeout)
		defer cancel()
	}

	if runtime.GOOS == "windows" {
		// On Windows, check for HIP SDK
		if hipPath := os.Getenv("HIP_PATH"); hipPath != "" {
			if _, err := os.Stat(hipPath); err == nil {
				return true
			}
		}

		// Check common installation paths
		hipPaths := []string{
			"C:\\Program Files\\AMD\\ROCm",
			"C:\\Program Files\\AMD\\HIP SDK",
		}
		for _, path := range hipPaths {
			if _, err := os.Stat(path); err == nil {
				return true
			}
		}

		// Check if any AMD GPU is present via WMI (doesn't require ROCm)
		cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
			"$gpu = Get-CimInstance Win32_VideoController | Where-Object { $_.Name -like '*AMD*' -or $_.Name -like '*Radeon*' } | Select-Object -First 1; if ($gpu) { 'true' }")
		output, err := cmd.Output()
		return err == nil && strings.TrimSpace(string(output)) == "true"
	}

	// On Linux, check for rocm-smi
	cmd := exec.CommandContext(ctx, "rocm-smi", "--version")
	if err := cmd.Run(); err == nil {
		return true
	}

	// Also check for ROCm installation path
	if _, err := os.Stat("/opt/rocm"); err == nil {
		return true
	}

	return false
}

// CheckROCmInstalled checks if ROCm/HIP is installed on the system.
// This is a public wrapper around the internal checks.
func CheckROCmInstalled() bool {
	if runtime.GOOS == "linux" {
		return checkRocmInstalled()
	}
	if runtime.GOOS == "windows" {
		return checkHipInstalled()
	}
	return false
}

// =============================================================================
// AMD VRAM INFERENCE (Extended)
// =============================================================================

// InferAMDVramFromModel infers VRAM size in GB from the AMD GPU model name.
// This extends the internal inferAmdVramFromModel with additional models.
func InferAMDVramFromModel(gpuName string) uint32 {
	// Use the internal function first
	vram := inferAmdVramFromModel(gpuName)
	if vram > 0 {
		return vram
	}

	// Additional models not covered by the base function
	nameLower := strings.ToLower(gpuName)

	// RX 8000 series (RDNA 4) - future proofing
	if strings.Contains(nameLower, "8800 xt") || strings.Contains(nameLower, "8900 xt") {
		return 16
	} else if strings.Contains(nameLower, "8700 xt") || strings.Contains(nameLower, "8800") {
		return 12
	}

	return 0
}

// =============================================================================
// AMD SETUP GUIDANCE
// =============================================================================

// GetAMDSetupCommands returns platform-specific setup commands for AMD GPU support.
func GetAMDSetupCommands() []string {
	if runtime.GOOS == "linux" {
		return []string{
			"# Install ROCm for AMD GPU support:",
			"wget https://repo.radeon.com/amdgpu-install/latest/ubuntu/jammy/amdgpu-install_6.0.60000-1_all.deb",
			"sudo apt install ./amdgpu-install_6.0.60000-1_all.deb",
			"sudo amdgpu-install --usecase=rocm",
		}
	}
	if runtime.GOOS == "windows" {
		return []string{
			"# AMD GPU support on Windows requires the HIP SDK",
			"# Download from: https://www.amd.com/en/developer/resources/rocm-hub/hip-sdk.html",
			"# After installation, Ollama should detect your AMD GPU",
		}
	}
	return []string{}
}

// GetAMDSetupLinks returns helpful documentation links for AMD GPU setup.
func GetAMDSetupLinks() []string {
	if runtime.GOOS == "linux" {
		return []string{
			"https://rocm.docs.amd.com/projects/install-on-linux/en/latest/",
			"https://ollama.com/blog/amd-preview",
		}
	}
	return []string{
		"https://www.amd.com/en/developer/resources/rocm-hub/hip-sdk.html",
	}
}
