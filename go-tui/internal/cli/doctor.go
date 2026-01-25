// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// doctor.go - Doctor command implementation for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Command: doctor [subcommand]
// Short:   Run system health checks and diagnostics
// Aliases: diag, diagnose
//
// Subcommands:
//   (default)           Run all health checks
//   fix                 Run checks and attempt auto-fixes
//
// Examples:
//   rigrun doctor                Run all health checks
//   rigrun doctor --json         Health check results in JSON (SIEM)
//   rigrun doctor fix            Run checks and attempt auto-fixes
//
// Health Checks Performed:
//   1. Ollama Installed   - Checks if Ollama CLI is available
//   2. Ollama Running     - Checks if Ollama server is responding
//   3. Model Available    - Checks if configured model is downloaded
//   4. GPU Detected       - Checks for GPU acceleration support
//   5. Config Valid       - Validates configuration file
//   6. Cache Writable     - Checks cache directory permissions
//   7. OpenRouter Config  - Checks cloud API key (optional)
//
// Status Symbols:
//   [green checkmark]   Pass  - Check successful
//   [yellow warning]    Warn  - Non-critical issue detected
//   [red X]             Fail  - Critical issue detected
//
// Flags:
//   --json              Output in JSON format
//
// Auto-Fix Examples:
//   - Missing Ollama:    Suggests installation command
//   - Ollama not running: Suggests "ollama serve"
//   - Missing model:     Suggests "ollama pull <model>"
//   - Invalid config:    Suggests "rigrun config reset"
//
// Exit Codes:
//   0   All checks passed
//   1   One or more checks failed
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/detect"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
)

// =============================================================================
// DOCTOR STYLES
// =============================================================================

var (
	// Doctor title style
	doctorTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	// Check pass style (green checkmark)
	checkPassStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true)

	// Check warn style (yellow warning)
	checkWarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true)

	// Check fail style (red X)
	checkFailStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	// Check message style
	checkMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255"))

	// Fix suggestion style
	fixStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true).
			PaddingLeft(2)

	// Summary style
	summaryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))
)

// =============================================================================
// HEALTH CHECK TYPES
// =============================================================================

// CheckStatus represents the status of a health check.
type CheckStatus int

const (
	// CheckPass indicates the check passed successfully.
	CheckPass CheckStatus = iota
	// CheckWarn indicates the check passed with warnings.
	CheckWarn
	// CheckFail indicates the check failed.
	CheckFail
)

// String returns the string representation of the check status.
func (s CheckStatus) String() string {
	switch s {
	case CheckPass:
		return "Pass"
	case CheckWarn:
		return "Warn"
	case CheckFail:
		return "Fail"
	default:
		return "Unknown"
	}
}

// Symbol returns the Unicode symbol for the check status.
func (s CheckStatus) Symbol() string {
	switch s {
	case CheckPass:
		return checkPassStyle.Render("[OK]")
	case CheckWarn:
		return checkWarnStyle.Render("[!!]")
	case CheckFail:
		return checkFailStyle.Render("[FAIL]")
	default:
		return "?"
	}
}

// HealthCheck represents a single health check result.
type HealthCheck struct {
	Name    string
	Status  CheckStatus
	Message string
	Fix     string // Suggested fix command or instruction
}

// Render returns a formatted string representation of the health check.
func (c *HealthCheck) Render() string {
	result := fmt.Sprintf("%s %s", c.Status.Symbol(), checkMsgStyle.Render(c.Message))
	if c.Status != CheckPass && c.Fix != "" {
		result += "\n" + fixStyle.Render("-> "+c.Fix)
	}
	return result
}

// allowedFixCommands defines a whitelist of permitted fix commands.
// Each key is a fix pattern, and the value is the safe command to execute.
// This prevents command injection by only allowing predefined commands.
var allowedFixCommands = map[string][]string{
	// Ollama installation and service management
	"brew install ollama":                {"brew", "install", "ollama"},
	"ollama serve":                       {"ollama", "serve"},

	// Windows Vulkan environment variable
	"setx OLLAMA_VULKAN 1":              {"setx", "OLLAMA_VULKAN", "1"},

	// Ollama install script (Linux/macOS)
	"curl -fsSL https://ollama.ai/install.sh | sh": {"sh", "-c", "curl -fsSL https://ollama.ai/install.sh | sh"},
}

// isAllowedFixCommand checks if a fix command matches a whitelisted pattern.
// Returns the safe command arguments if allowed, nil otherwise.
func isAllowedFixCommand(fixCmd string) []string {
	// Normalize the command string
	normalized := strings.TrimSpace(fixCmd)

	// Check for exact match in whitelist
	if args, ok := allowedFixCommands[normalized]; ok {
		return args
	}

	// Check for ollama pull commands (dynamic model names)
	if strings.HasPrefix(normalized, "ollama pull ") {
		modelName := strings.TrimPrefix(normalized, "ollama pull ")
		modelName = strings.TrimSpace(modelName)

		// Validate model name format: alphanumeric, dash, underscore, colon, dot only
		// This prevents command injection through model names
		for _, ch := range modelName {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			     (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' ||
			     ch == ':' || ch == '.') {
				return nil
			}
		}

		return []string{"ollama", "pull", modelName}
	}

	// Check for rigrun config reset
	if normalized == "rigrun config reset" {
		return []string{"rigrun", "config", "reset"}
	}

	// Check for rigrun config set (with validation)
	if strings.HasPrefix(normalized, "rigrun config set ") {
		// Don't auto-execute config set commands as they contain user data
		// These should be manual only
		return nil
	}

	return nil
}

// TryFix attempts to automatically fix the issue if possible.
// Uses a whitelist approach to prevent command injection vulnerabilities.
func (c *HealthCheck) TryFix() error {
	if c.Fix == "" || c.Status == CheckPass {
		return nil
	}

	// Extract the actual command from the Fix string
	fixCmd := c.Fix

	// Check for "Run:" prefix and extract command
	if strings.HasPrefix(fixCmd, "Run: ") {
		fixCmd = strings.TrimPrefix(fixCmd, "Run: ")
	} else if strings.HasPrefix(fixCmd, "Restart Ollama: ") {
		// Special case for "Restart Ollama: ollama serve"
		fixCmd = strings.TrimPrefix(fixCmd, "Restart Ollama: ")
	} else {
		// Not an auto-fixable command (manual instructions only)
		return fmt.Errorf("manual fix required: %s", c.Fix)
	}

	fixCmd = strings.TrimSpace(fixCmd)

	// Check if command is in whitelist
	args := isAllowedFixCommand(fixCmd)
	if args == nil {
		return fmt.Errorf("fix command not permitted by security policy: %s", fixCmd)
	}

	fmt.Printf("  Attempting fix: %s\n", fixCmd)

	// Execute the whitelisted command
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fix failed: %w", err)
	}

	return nil
}

// =============================================================================
// HANDLE DOCTOR
// =============================================================================

// HandleDoctor handles the "doctor" command.
// Runs system health checks and optionally attempts auto-fixes.
// Supports JSON output for IL5 SIEM integration (AU-6, SI-4).
func HandleDoctor(args Args) error {
	// Run all checks
	checks := runAllChecks()

	// Count results
	passed := 0
	warned := 0
	failed := 0
	for _, check := range checks {
		switch check.Status {
		case CheckPass:
			passed++
		case CheckWarn:
			warned++
		case CheckFail:
			failed++
		}
	}

	// JSON output mode for SIEM integration
	if args.JSON {
		return handleDoctorJSON(checks, passed, warned, failed)
	}

	// Human-readable output
	separator := strings.Repeat("=", 41)
	fmt.Println()
	fmt.Println(doctorTitleStyle.Render("rigrun Doctor"))
	fmt.Println(separatorStyle.Render(separator))
	fmt.Println()

	// Display results
	for _, check := range checks {
		fmt.Println(check.Render())
	}

	// Summary line
	fmt.Println()
	fmt.Println(separatorStyle.Render(strings.Repeat("-", 41)))

	summaryParts := []string{
		fmt.Sprintf("%d passed", passed),
	}
	if warned > 0 {
		summaryParts = append(summaryParts, checkWarnStyle.Render(fmt.Sprintf("%d warning", warned)))
	}
	if failed > 0 {
		summaryParts = append(summaryParts, checkFailStyle.Render(fmt.Sprintf("%d failed", failed)))
	}

	fmt.Println(summaryStyle.Render(strings.Join(summaryParts, ", ")))
	fmt.Println()

	// Auto-fix if requested
	if args.Subcommand == "fix" && (warned > 0 || failed > 0) {
		fmt.Println(doctorTitleStyle.Render("Attempting Auto-Fix..."))
		fmt.Println()

		for _, check := range checks {
			if check.Status != CheckPass && check.Fix != "" {
				if err := check.TryFix(); err != nil {
					fmt.Printf("  %s Could not fix %s: %s\n",
						checkWarnStyle.Render("[!!]"),
						check.Name,
						err)
				} else {
					fmt.Printf("  %s Fixed %s\n",
						checkPassStyle.Render("[OK]"),
						check.Name)
				}
			}
		}
		fmt.Println()
	}

	// Return error if there are failures
	if failed > 0 {
		return fmt.Errorf("%d health check(s) failed", failed)
	}

	return nil
}

// handleDoctorJSON outputs doctor results in JSON format for SIEM integration.
func handleDoctorJSON(checks []*HealthCheck, passed, warned, failed int) error {
	// Convert checks to JSON-friendly format
	jsonChecks := make([]DoctorCheck, 0, len(checks))
	for _, check := range checks {
		status := "pass"
		switch check.Status {
		case CheckWarn:
			status = "warn"
		case CheckFail:
			status = "fail"
		}

		jsonChecks = append(jsonChecks, DoctorCheck{
			Name:    check.Name,
			Status:  status,
			Message: check.Message,
			Fix:     check.Fix,
		})
	}

	data := DoctorData{
		Checks: jsonChecks,
		Summary: DoctorSummary{
			Passed:  passed,
			Warned:  warned,
			Failed:  failed,
			Healthy: failed == 0,
		},
	}

	resp := NewJSONResponse("doctor", data)

	// If there are failures, mark as unsuccessful but still output data
	if failed > 0 {
		errMsg := fmt.Sprintf("%d health check(s) failed", failed)
		resp.Success = false
		resp.Error = &errMsg
	}

	return resp.Print()
}

// =============================================================================
// HEALTH CHECK FUNCTIONS
// =============================================================================

// runAllChecks runs all health checks and returns the results.
func runAllChecks() []*HealthCheck {
	var checks []*HealthCheck

	// 1. Check Ollama installed
	checks = append(checks, checkOllamaInstalled())

	// 2. Check Ollama running
	checks = append(checks, checkOllamaRunning())

	// 3. Check model available
	checks = append(checks, checkModelAvailable())

	// 4. Check GPU detected
	checks = append(checks, checkGPUDetected())

	// 5. Check config valid
	checks = append(checks, checkConfigValid())

	// 6. Check cache writable
	checks = append(checks, checkCacheWritable())

	// 7. Check OpenRouter configured (optional)
	checks = append(checks, checkOpenRouterConfigured())

	return checks
}

// checkOllamaInstalled checks if Ollama CLI is installed.
func checkOllamaInstalled() *HealthCheck {
	check := &HealthCheck{
		Name: "Ollama Installed",
	}

	// Try to run ollama --version
	cmd := exec.Command("ollama", "--version")
	output, err := cmd.Output()

	if err != nil {
		check.Status = CheckFail
		check.Message = "Ollama not installed"
		if runtime.GOOS == "windows" {
			check.Fix = "Download from https://ollama.ai/download"
		} else if runtime.GOOS == "darwin" {
			check.Fix = "Run: brew install ollama"
		} else {
			check.Fix = "Run: curl -fsSL https://ollama.ai/install.sh | sh"
		}
		return check
	}

	// Parse version
	versionStr := strings.TrimSpace(string(output))
	parts := strings.Fields(versionStr)
	version := "unknown"
	if len(parts) > 0 {
		version = parts[len(parts)-1]
	}

	check.Status = CheckPass
	check.Message = fmt.Sprintf("Ollama installed (v%s)", version)
	return check
}

// checkOllamaRunning checks if Ollama server is running.
func checkOllamaRunning() *HealthCheck {
	check := &HealthCheck{
		Name: "Ollama Running",
	}

	// Try HTTP check to 127.0.0.1:11434 (explicit IPv4 to avoid IPv6 issues on Windows)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:11434", nil)
	if err != nil {
		check.Status = CheckFail
		check.Message = "Could not create request"
		check.Fix = "Run: ollama serve"
		return check
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		check.Status = CheckFail
		check.Message = "Ollama server not running"
		check.Fix = "Run: ollama serve"
		return check
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		check.Status = CheckWarn
		check.Message = fmt.Sprintf("Ollama returned status %d", resp.StatusCode)
		check.Fix = "Restart Ollama: ollama serve"
		return check
	}

	check.Status = CheckPass
	check.Message = "Ollama running"
	return check
}

// checkModelAvailable checks if the configured model is available.
func checkModelAvailable() *HealthCheck {
	check := &HealthCheck{
		Name: "Model Available",
	}

	// Load config to get model name
	cfg, err := LoadConfig()
	if err != nil {
		cfg = DefaultConfig()
	}

	modelName := cfg.Local.OllamaModel
	if modelName == "" {
		modelName = cfg.DefaultModel
	}
	if modelName == "" {
		// Use recommended model based on GPU
		gpu, _ := detect.DetectGPUCached()
		if gpu != nil {
			// VramGB is in GB, RecommendModel expects MB
			rec := detect.RecommendModel(int(gpu.VramGB) * 1024)
			modelName = rec.ModelName
		} else {
			modelName = "qwen2.5-coder:7b"
		}
	}

	// Check if model is in Ollama's list
	client := ollama.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	models, err := client.ListModels(ctx)
	if err != nil {
		check.Status = CheckWarn
		check.Message = fmt.Sprintf("Could not check model: %s", err)
		check.Fix = fmt.Sprintf("Run: ollama pull %s", modelName)
		return check
	}

	// Check if model exists
	found := false
	for _, m := range models {
		if m.Name == modelName || strings.HasPrefix(m.Name, modelName+":") {
			found = true
			break
		}
	}

	if !found {
		check.Status = CheckWarn
		check.Message = fmt.Sprintf("Model not downloaded: %s", modelName)
		check.Fix = fmt.Sprintf("Run: ollama pull %s", modelName)
		return check
	}

	check.Status = CheckPass
	check.Message = fmt.Sprintf("Model available: %s", modelName)
	return check
}

// checkGPUDetected checks if a GPU is detected.
func checkGPUDetected() *HealthCheck {
	check := &HealthCheck{
		Name: "GPU Detected",
	}

	gpu, err := detect.DetectGPU()
	if err != nil {
		check.Status = CheckWarn
		check.Message = fmt.Sprintf("GPU detection failed: %s", err)
		check.Fix = "Check GPU drivers are installed"
		return check
	}

	if gpu == nil || gpu.Type == detect.GpuTypeCPU {
		check.Status = CheckWarn
		check.Message = "No GPU detected - running in CPU mode"
		check.Fix = "Install GPU drivers for acceleration"
		return check
	}

	check.Status = CheckPass
	check.Message = fmt.Sprintf("GPU detected: %s", gpu.Name)

	// Check for AMD RDNA 4 specific issues
	if gpu.Type == detect.GpuTypeAmd {
		arch := detect.DetectAmdArchitecture(gpu.Name)
		if arch == detect.AmdArchRdna4 {
			// Check if OLLAMA_VULKAN is set
			if os.Getenv("OLLAMA_VULKAN") != "1" {
				check.Status = CheckWarn
				check.Message = fmt.Sprintf("GPU detected: %s (RDNA 4 requires Vulkan)", gpu.Name)
				if runtime.GOOS == "windows" {
					check.Fix = "Run: setx OLLAMA_VULKAN 1"
				} else {
					check.Fix = "Set: export OLLAMA_VULKAN=1"
				}
			}
		}
	}

	return check
}

// checkConfigValid checks if the configuration file is valid.
func checkConfigValid() *HealthCheck {
	check := &HealthCheck{
		Name: "Config Valid",
	}

	configPath := ConfigPath()
	if configPath == "" {
		check.Status = CheckWarn
		check.Message = "Could not determine config path"
		return check
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		check.Status = CheckPass
		check.Message = "Config valid (using defaults)"
		return check
	}

	// Try to load config
	_, err := LoadConfig()
	if err != nil {
		check.Status = CheckFail
		check.Message = fmt.Sprintf("Config invalid: %s", err)
		check.Fix = "Run: rigrun config reset"
		return check
	}

	check.Status = CheckPass
	check.Message = "Config valid"
	return check
}

// checkCacheWritable checks if the cache directory is writable.
func checkCacheWritable() *HealthCheck {
	check := &HealthCheck{
		Name: "Cache Writable",
	}

	// Get cache directory
	cacheDir, err := getCacheDir()
	if err != nil {
		check.Status = CheckFail
		check.Message = fmt.Sprintf("Could not determine cache directory: %s", err)
		return check
	}

	// Ensure directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		check.Status = CheckFail
		check.Message = fmt.Sprintf("Could not create cache directory: %s", err)
		check.Fix = fmt.Sprintf("Create manually: mkdir -p %s", cacheDir)
		return check
	}

	// Try to write a test file
	testFile := filepath.Join(cacheDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		check.Status = CheckFail
		check.Message = fmt.Sprintf("Cache directory not writable: %s", err)
		check.Fix = fmt.Sprintf("Check permissions: chmod 755 %s", cacheDir)
		return check
	}

	// Clean up test file
	os.Remove(testFile)

	check.Status = CheckPass
	check.Message = "Cache directory writable"
	return check
}

// checkOpenRouterConfigured checks if OpenRouter API key is configured.
func checkOpenRouterConfigured() *HealthCheck {
	check := &HealthCheck{
		Name: "OpenRouter Configured",
	}

	cfg, err := LoadConfig()
	if err != nil {
		cfg = DefaultConfig()
	}

	if cfg.Cloud.OpenRouterKey == "" {
		check.Status = CheckWarn
		check.Message = "OpenRouter not configured (cloud routing disabled)"
		check.Fix = "Run: rigrun config set openrouter_key YOUR_KEY"
		return check
	}

	// Validate key format
	if !strings.HasPrefix(cfg.Cloud.OpenRouterKey, "sk-or-") {
		check.Status = CheckWarn
		check.Message = "OpenRouter key format may be invalid"
		check.Fix = "Get key from https://openrouter.ai/keys"
		return check
	}

	check.Status = CheckPass
	check.Message = "OpenRouter configured"
	return check
}

// =============================================================================
// HELPERS
// =============================================================================

// getCacheDir returns the rigrun cache directory path.
func getCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}

	cacheDir := filepath.Join(home, ".rigrun", "cache")
	return cacheDir, nil
}
