// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// status.go - Status command implementation for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Command: status
// Short:   Display comprehensive system status
// Aliases: s, info
//
// Examples:
//   rigrun status                 Show system status
//   rigrun s                      Show status (short alias)
//   rigrun status --json          Status in JSON format (SIEM)
//
// Status Sections:
//   System:    GPU detection, Ollama status, loaded model
//   Routing:   Default mode, max tier, paranoid mode, cloud key
//   Session:   Query counts, local/cloud split, cache hits, cost/savings
//   Cache:     Total entries, hit rate, exact/semantic hits
//
// Output Fields:
//   GPU        Detected GPU name and VRAM (or "CPU mode")
//   Ollama     Running status and version
//   Model      Current/configured model and load status
//   Default    Routing mode (Local/Cloud/Hybrid)
//   Max Tier   Maximum allowed cloud tier
//   Paranoid   Whether cloud requests are blocked
//   Cloud Key  Whether OpenRouter API key is configured
//   Queries    Total queries this session
//   Local      Local model queries (count and percentage)
//   Cloud      Cloud API queries (count and percentage)
//   Cache Hits Cache-served queries
//   Cost       Estimated cost this session
//   Saved      Estimated savings from cache and local
//   Entries    Total cache entries
//   Hit Rate   Cache hit percentage
//
// Flags:
//   --json              Output in JSON format
//
// JSON Output (for SIEM):
//   Provides structured output suitable for log aggregation
//   and security information event management systems.
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/cache"
	"github.com/jeranaias/rigrun-tui/internal/detect"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
)

// =============================================================================
// STYLES
// =============================================================================

var (
	// Title style for the header
	statusTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	// Section header style
	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")). // White
			MarginTop(1)

	// Label style for field names
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")). // Light gray
			Width(14)

	// Value styles
	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")) // White

	valueGreenStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")) // Green

	valueYellowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")) // Yellow

	valueRedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red

	valueDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Dim

	// Separator line
	statusSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// =============================================================================
// SESSION STATS (in-memory tracking)
// =============================================================================

// SessionStats tracks statistics for the current session.
type SessionStats struct {
	Queries   int
	Local     int
	Cloud     int
	CacheHits int
	Cost      float64
	Saved     float64
}

// Global session stats (would be in a session manager in production)
var sessionStats = SessionStats{}

// =============================================================================
// HANDLE STATUS
// =============================================================================

// HandleStatus handles the "status" command.
// Displays comprehensive system status including GPU, Ollama, routing, and cache info.
// Supports JSON output for IL5 SIEM integration (AU-6, SI-4).
func HandleStatus(args Args) error {
	// JSON output mode for SIEM integration
	if args.JSON {
		return handleStatusJSON(args)
	}

	// Print header
	separator := strings.Repeat("=", 41)
	fmt.Println()
	fmt.Println(statusTitleStyle.Render("rigrun Status"))
	fmt.Println(statusSeparatorStyle.Render(separator))
	fmt.Println()

	// System section
	fmt.Println(sectionStyle.Render("System"))
	fmt.Println(formatGPUStatus())
	fmt.Println(formatOllamaStatus())
	fmt.Println(formatModelStatus())
	fmt.Println()

	// Routing section
	fmt.Println(sectionStyle.Render("Routing"))
	fmt.Println(formatRoutingStatus(args))
	fmt.Println()

	// Session section
	fmt.Println(sectionStyle.Render("Session"))
	fmt.Println(formatSessionStats())
	fmt.Println()

	// Cache section
	fmt.Println(sectionStyle.Render("Cache"))
	fmt.Println(formatCacheStats())
	fmt.Println()

	return nil
}

// handleStatusJSON outputs status information in JSON format for SIEM integration.
func handleStatusJSON(args Args) error {
	// Collect system info
	systemInfo := collectSystemInfo()

	// Collect routing info
	routingInfo := collectRoutingInfo(args)

	// Collect session info
	sessionInfo := collectSessionInfo()

	// Collect cache info
	cacheInfo := collectCacheInfo()

	// Build response
	data := StatusData{
		System:  systemInfo,
		Routing: routingInfo,
		Session: sessionInfo,
		Cache:   cacheInfo,
	}

	resp := NewJSONResponse("status", data)
	return resp.Print()
}

// collectSystemInfo gathers system information for JSON output.
func collectSystemInfo() StatusSystemInfo {
	info := StatusSystemInfo{}

	// GPU info
	gpu, err := detect.DetectGPUCached()
	if err != nil || gpu == nil || gpu.Type == detect.GpuTypeCPU {
		info.GPU = "None detected (CPU mode)"
		info.GPUType = "cpu"
	} else {
		info.GPU = gpu.Name
		info.GPUType = gpu.Type.String()
		info.VRAMGB = int(gpu.VramGB)
	}

	// Ollama status
	client := ollama.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.CheckRunning(ctx); err != nil {
		info.Ollama = "not_running"
	} else {
		info.Ollama = "running"
		info.OllamaVer = getOllamaVersion()
	}

	// Model info
	loadedModels, err := detect.GetOllamaLoadedModels()
	if err == nil && len(loadedModels) > 0 {
		info.Model = loadedModels[0]
		info.ModelStatus = "loaded"
	} else {
		cfg, configErr := LoadConfig()
		if configErr == nil && cfg.Local.OllamaModel != "" {
			info.Model = cfg.Local.OllamaModel
		} else if configErr == nil && cfg.DefaultModel != "" {
			info.Model = cfg.DefaultModel
		} else {
			if gpu != nil {
				rec := detect.RecommendModel(int(gpu.VramGB) * 1024)
				info.Model = rec.ModelName
			} else {
				info.Model = "qwen2.5-coder:7b"
			}
		}

		// Check availability
		models, listErr := client.ListModels(ctx)
		if listErr == nil {
			available := false
			for _, m := range models {
				if m.Name == info.Model || strings.HasPrefix(m.Name, info.Model+":") {
					available = true
					break
				}
			}
			if available {
				info.ModelStatus = "available"
			} else {
				info.ModelStatus = "not_downloaded"
			}
		} else {
			info.ModelStatus = "unknown"
		}
	}

	return info
}

// collectRoutingInfo gathers routing configuration for JSON output.
func collectRoutingInfo(args Args) StatusRoutingInfo {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = DefaultConfig()
	}

	info := StatusRoutingInfo{
		DefaultMode:  cfg.Routing.DefaultMode,
		MaxTier:      cfg.Routing.MaxTier,
		ParanoidMode: args.Paranoid || cfg.Routing.ParanoidMode,
		CloudKeySet:  cfg.Cloud.OpenRouterKey != "",
	}

	if info.DefaultMode == "" {
		info.DefaultMode = "local"
	}
	if info.MaxTier == "" {
		info.MaxTier = "opus"
	}

	return info
}

// collectSessionInfo gathers session statistics for JSON output.
func collectSessionInfo() StatusSessionInfo {
	stats := sessionStats
	return StatusSessionInfo{
		Queries:    stats.Queries,
		Local:      stats.Local,
		Cloud:      stats.Cloud,
		CacheHits:  stats.CacheHits,
		CostCents:  stats.Cost * 100,
		SavedCents: stats.Saved * 100,
	}
}

// collectCacheInfo gathers cache statistics for JSON output.
func collectCacheInfo() StatusCacheInfo {
	manager := cache.Default()
	stats := manager.Stats()
	hitRate := manager.HitRate()

	return StatusCacheInfo{
		Entries:      manager.ExactCacheSize() + manager.SemanticCacheSize(),
		HitRate:      hitRate,
		ExactHits:    stats.ExactHits,
		SemanticHits: stats.SemanticHits,
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// formatGPUStatus returns a formatted string describing the GPU status.
func formatGPUStatus() string {
	gpu, err := detect.DetectGPUCached()

	var gpuStr string
	if err != nil || gpu == nil || gpu.Type == detect.GpuTypeCPU {
		gpuStr = valueDimStyle.Render("None detected (CPU mode)")
	} else {
		gpuStr = valueGreenStyle.Render(fmt.Sprintf("%s (%dGB VRAM)", gpu.Name, gpu.VramGB))
	}

	return fmt.Sprintf("  %s%s", labelStyle.Render("GPU:"), gpuStr)
}

// formatOllamaStatus returns a formatted string describing Ollama status.
func formatOllamaStatus() string {
	client := ollama.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := client.CheckRunning(ctx)

	var statusStr string
	if err != nil {
		statusStr = valueRedStyle.Render("Not running")
	} else {
		// Try to get version from ollama --version
		version := getOllamaVersion()
		if version != "" {
			statusStr = valueGreenStyle.Render(fmt.Sprintf("Running (v%s)", version))
		} else {
			statusStr = valueGreenStyle.Render("Running")
		}
	}

	return fmt.Sprintf("  %s%s", labelStyle.Render("Ollama:"), statusStr)
}

// formatModelStatus returns a formatted string describing the model status.
func formatModelStatus() string {
	client := ollama.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Get loaded models
	loadedModels, err := detect.GetOllamaLoadedModels()

	var modelStr string
	if err != nil || len(loadedModels) == 0 {
		// No models loaded - show default/configured model
		cfg, configErr := LoadConfig()
		var modelName string
		if configErr == nil && cfg.Local.OllamaModel != "" {
			modelName = cfg.Local.OllamaModel
		} else if configErr == nil && cfg.DefaultModel != "" {
			modelName = cfg.DefaultModel
		} else {
			gpu, _ := detect.DetectGPUCached()
			if gpu != nil {
				// VramGB is in GB, RecommendModel expects MB
				rec := detect.RecommendModel(int(gpu.VramGB) * 1024)
				modelName = rec.ModelName
			} else {
				modelName = "qwen2.5-coder:7b"
			}
		}

		// Check if model is available
		models, listErr := client.ListModels(ctx)
		available := false
		if listErr == nil {
			for _, m := range models {
				if m.Name == modelName || strings.HasPrefix(m.Name, modelName+":") {
					available = true
					break
				}
			}
		}

		if available {
			modelStr = valueStyle.Render(fmt.Sprintf("%s (available)", modelName))
		} else {
			modelStr = valueYellowStyle.Render(fmt.Sprintf("%s (not downloaded)", modelName))
		}
	} else {
		// Show first loaded model
		modelStr = valueGreenStyle.Render(fmt.Sprintf("%s (loaded)", loadedModels[0]))
	}

	return fmt.Sprintf("  %s%s", labelStyle.Render("Model:"), modelStr)
}

// formatRoutingStatus returns formatted routing configuration info.
func formatRoutingStatus(args Args) string {
	var lines []string

	// Load config for routing info
	cfg, err := LoadConfig()
	if err != nil {
		cfg = DefaultConfig()
	}

	// Default routing mode
	defaultMode := cfg.Routing.DefaultMode
	if defaultMode == "" {
		defaultMode = "local"
	}
	defaultRouting := fmt.Sprintf("%s mode", strings.Title(defaultMode))
	if args.Paranoid || cfg.Routing.ParanoidMode {
		defaultRouting = "Local only (paranoid mode)"
	} else if cfg.Routing.DefaultMode == "cloud" && cfg.Cloud.OpenRouterKey == "" {
		defaultRouting = "Local only (no cloud key)"
	}
	lines = append(lines, fmt.Sprintf("  %s%s",
		labelStyle.Render("Default:"),
		valueStyle.Render(defaultRouting)))

	// Max tier
	maxTier := cfg.Routing.MaxTier
	if maxTier == "" {
		maxTier = "opus"
	}
	lines = append(lines, fmt.Sprintf("  %s%s",
		labelStyle.Render("Max Tier:"),
		valueStyle.Render(maxTier)))

	// Paranoid mode
	paranoidStr := "No"
	if args.Paranoid || cfg.Routing.ParanoidMode {
		paranoidStr = valueYellowStyle.Render("Yes")
	}
	lines = append(lines, fmt.Sprintf("  %s%s",
		labelStyle.Render("Paranoid:"),
		valueStyle.Render(paranoidStr)))

	// OpenRouter key status
	keyStatus := "Not configured"
	if cfg.Cloud.OpenRouterKey != "" {
		keyStatus = valueGreenStyle.Render("Configured")
	}
	lines = append(lines, fmt.Sprintf("  %s%s",
		labelStyle.Render("Cloud Key:"),
		valueStyle.Render(keyStatus)))

	return strings.Join(lines, "\n")
}

// formatSessionStats returns formatted session statistics.
func formatSessionStats() string {
	var lines []string

	// For now use global session stats (in production this would come from session manager)
	stats := sessionStats

	// Queries
	lines = append(lines, fmt.Sprintf("  %s%s",
		labelStyle.Render("Queries:"),
		valueStyle.Render(fmt.Sprintf("%d", stats.Queries))))

	// Local with percentage
	localPct := 0
	if stats.Queries > 0 {
		localPct = stats.Local * 100 / stats.Queries
	}
	lines = append(lines, fmt.Sprintf("  %s%s",
		labelStyle.Render("Local:"),
		valueGreenStyle.Render(fmt.Sprintf("%d (%d%%)", stats.Local, localPct))))

	// Cloud with percentage
	cloudPct := 0
	if stats.Queries > 0 {
		cloudPct = stats.Cloud * 100 / stats.Queries
	}
	lines = append(lines, fmt.Sprintf("  %s%s",
		labelStyle.Render("Cloud:"),
		valueYellowStyle.Render(fmt.Sprintf("%d (%d%%)", stats.Cloud, cloudPct))))

	// Cache hits
	lines = append(lines, fmt.Sprintf("  %s%s",
		labelStyle.Render("Cache Hits:"),
		valueStyle.Render(fmt.Sprintf("%d", stats.CacheHits))))

	// Cost (in cents)
	lines = append(lines, fmt.Sprintf("  %s%s",
		labelStyle.Render("Cost:"),
		valueStyle.Render(fmt.Sprintf("%.2f¢", stats.Cost*100))))

	// Saved (in cents)
	lines = append(lines, fmt.Sprintf("  %s%s",
		labelStyle.Render("Saved:"),
		valueGreenStyle.Render(fmt.Sprintf("%.2f¢", stats.Saved*100))))

	return strings.Join(lines, "\n")
}

// formatCacheStats returns formatted cache statistics.
func formatCacheStats() string {
	var lines []string

	// Get cache manager stats
	manager := cache.Default()
	stats := manager.Stats()
	hitRate := manager.HitRate()

	// Total entries
	totalEntries := manager.ExactCacheSize() + manager.SemanticCacheSize()
	lines = append(lines, fmt.Sprintf("  %s%s",
		labelStyle.Render("Entries:"),
		valueStyle.Render(fmt.Sprintf("%d", totalEntries))))

	// Hit rate as percentage
	hitRatePct := int(hitRate * 100)
	hitRateStr := fmt.Sprintf("%d%%", hitRatePct)
	if hitRatePct >= 50 {
		hitRateStr = valueGreenStyle.Render(hitRateStr)
	} else if hitRatePct >= 20 {
		hitRateStr = valueYellowStyle.Render(hitRateStr)
	} else {
		hitRateStr = valueDimStyle.Render(hitRateStr)
	}
	lines = append(lines, fmt.Sprintf("  %s%s",
		labelStyle.Render("Hit Rate:"),
		hitRateStr))

	// Optional: show breakdown if verbose
	if stats.TotalLookups > 0 {
		lines = append(lines, fmt.Sprintf("  %s%s",
			labelStyle.Render("Exact Hits:"),
			valueDimStyle.Render(fmt.Sprintf("%d", stats.ExactHits))))
		lines = append(lines, fmt.Sprintf("  %s%s",
			labelStyle.Render("Semantic:"),
			valueDimStyle.Render(fmt.Sprintf("%d", stats.SemanticHits))))
	}

	return strings.Join(lines, "\n")
}

// getOllamaVersion attempts to get the Ollama version from CLI.
func getOllamaVersion() string {
	// Try to execute ollama --version
	cmd := exec.Command("ollama", "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Parse version string - typically "ollama version 0.5.4"
	parts := strings.Fields(string(output))
	if len(parts) > 0 {
		// Return the last part which should be the version number
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return ""
}

// =============================================================================
// CONFIG HELPERS (shared with config.go)
// =============================================================================

// getConfigDir returns the rigrun config directory path.
func getConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}

	configDir := filepath.Join(home, ".rigrun")

	// Create if doesn't exist
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return "", fmt.Errorf("could not create config directory: %w", err)
		}
	}

	return configDir, nil
}
