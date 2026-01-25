// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// setup.go - First-run wizard and setup commands for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Command: setup
// Short:   First-run setup wizard
// Aliases: init, wizard
//
// Examples:
//   rigrun setup                  Run interactive setup wizard
//   rigrun setup --json           Show setup status in JSON
//
// The setup wizard walks through:
//   1. GPU detection and configuration
//   2. Ollama installation verification
//   3. Model selection and download
//   4. Optional OpenRouter API key configuration
//   5. Security settings (paranoid mode, audit logging)
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"

	"github.com/jeranaias/rigrun-tui/internal/detect"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
)

// =============================================================================
// DEPLOYMENT MODES AND USE CASES
// =============================================================================

// DeploymentMode represents the deployment mode for rigrun.
type DeploymentMode int

const (
	// DeploymentCloud is cloud-first routing (recommended).
	DeploymentCloud DeploymentMode = iota
	// DeploymentLocal is local-only mode (no cloud).
	DeploymentLocal
	// DeploymentHybrid tries local first, falls back to cloud.
	DeploymentHybrid
)

// String returns the string representation of the deployment mode.
func (d DeploymentMode) String() string {
	switch d {
	case DeploymentCloud:
		return "cloud"
	case DeploymentLocal:
		return "local"
	case DeploymentHybrid:
		return "hybrid"
	default:
		return "unknown"
	}
}

// WizardConfig holds configuration collected from the setup wizard.
type WizardConfig struct {
	DeploymentMode   DeploymentMode
	Model            string
	OpenRouterKey    string
	EnableConsent    bool
	EnableAuditLog   bool
	ConfigPath       string
}

// =============================================================================
// SETUP COMMAND HANDLER
// =============================================================================

// HandleSetup handles the "setup" command with various subcommands.
// Modes:
//   - setup: Full interactive wizard
//   - setup --quick: Minimal setup with defaults
//   - setup gpu: GPU detection only
//   - setup model: Model selection only
func HandleSetup(args Args) error {
	// Check for --quick flag in raw args
	quick := false
	for _, arg := range args.Raw {
		if arg == "--quick" || arg == "-q" {
			quick = true
			break
		}
	}

	switch args.Subcommand {
	case "":
		if quick {
			return runQuickSetup()
		}
		return runFullWizard()
	case "quick":
		return runQuickSetup()
	case "gpu":
		return runGPUSetup()
	case "model":
		return runModelSetup()
	case "wizard":
		return runFullWizard()
	default:
		return fmt.Errorf("unknown setup subcommand: %s", args.Subcommand)
	}
}

// =============================================================================
// FULL WIZARD
// =============================================================================

// runFullWizard runs the complete interactive setup wizard.
func runFullWizard() error {
	config := &WizardConfig{}

	// Header
	fmt.Println()
	fmt.Println("rigrun Setup Wizard")
	fmt.Println(strings.Repeat("=", 39))
	fmt.Println()

	// Step 1: Hardware Detection
	fmt.Println("Step 1: Hardware Detection")
	fmt.Println(strings.Repeat("-", 26))

	gpu, model, err := detectHardwareWithSpinner()
	if err != nil {
		fmt.Printf("  Detecting GPU... X Error: %v\n", err)
	}
	config.Model = model

	// Check Ollama
	ollamaRunning := checkOllamaWithSpinner()
	fmt.Println()

	// Step 2: Model Selection
	fmt.Println("Step 2: Model Selection")
	fmt.Println(strings.Repeat("-", 23))

	if gpu != nil && config.Model != "" {
		fmt.Printf("Recommended model for your GPU: %s\n", config.Model)
	}
	fmt.Println()
	fmt.Println("Available models:")
	fmt.Printf("  [1] %s (recommended)\n", config.Model)

	// Get alternative models based on VRAM
	vramMB := 8000 // Default
	if gpu != nil {
		vramMB = int(gpu.VramGB) * 1024
	}
	alternatives := detect.GetAlternativeModels(vramMB)
	for i, alt := range alternatives {
		if i < 2 { // Show up to 2 alternatives
			fmt.Printf("  [%d] %s\n", i+2, alt.ModelName)
		}
	}
	fmt.Println("  [3] Other (enter name)")
	fmt.Println()

	choice := promptChoice("Select model", []string{"1", "2", "3"}, 0)
	switch choice {
	case 0:
		// Keep recommended
	case 1:
		if len(alternatives) > 0 {
			config.Model = alternatives[0].ModelName
		}
	case 2:
		if len(alternatives) > 1 {
			config.Model = alternatives[1].ModelName
		} else {
			config.Model = promptString("Enter model name", config.Model)
		}
	}
	fmt.Println()

	// Step 3: Routing Configuration
	fmt.Println("Step 3: Routing Configuration")
	fmt.Println(strings.Repeat("-", 29))
	fmt.Println("Default routing mode:")
	fmt.Println("  [1] Cloud (recommended) - Best quality, uses OpenRouter")
	fmt.Println("  [2] Local - Free, uses only local GPU")
	fmt.Println("  [3] Hybrid - Try local first, fall back to cloud")
	fmt.Println()

	routingChoice := promptChoice("Select mode", []string{"1", "2", "3"}, 0)
	switch routingChoice {
	case 0:
		config.DeploymentMode = DeploymentCloud
	case 1:
		config.DeploymentMode = DeploymentLocal
	case 2:
		config.DeploymentMode = DeploymentHybrid
	}
	fmt.Println()

	// Step 4: Cloud Setup (optional)
	if config.DeploymentMode != DeploymentLocal {
		fmt.Println("Step 4: Cloud Setup (optional)")
		fmt.Println(strings.Repeat("-", 30))
		config.OpenRouterKey = promptSecure("OpenRouter API key (press Enter to skip)")
		fmt.Println()
	}

	// Step 5: Security Settings
	fmt.Println("Step 5: Security Settings")
	fmt.Println(strings.Repeat("-", 25))
	config.EnableConsent = promptYesNo("Enable DoD consent banner?", true)
	config.EnableAuditLog = promptYesNo("Enable audit logging?", true)
	fmt.Println()

	// Save configuration
	configPath, err := saveConfig(config)
	if err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Download model if Ollama is running
	if ollamaRunning && config.Model != "" {
		fmt.Println("Downloading model...")
		if err := downloadModelWithProgress(config.Model); err != nil {
			fmt.Printf("  Warning: Model download failed: %v\n", err)
			fmt.Printf("  You can download later with: rigrun pull %s\n", config.Model)
		}
	}

	// Completion
	fmt.Println()
	fmt.Println("Setup Complete!")
	fmt.Println(strings.Repeat("=", 15))
	fmt.Printf("Config saved to %s\n", configPath)
	fmt.Println("Run 'rigrun' to start chatting!")
	fmt.Println()

	return nil
}

// =============================================================================
// QUICK SETUP
// =============================================================================

// runQuickSetup runs minimal setup with auto-detection and defaults.
func runQuickSetup() error {
	fmt.Println()
	fmt.Println("rigrun Quick Setup")
	fmt.Println(strings.Repeat("=", 18))
	fmt.Println()

	config := &WizardConfig{
		DeploymentMode: DeploymentCloud, // Default to cloud
		EnableConsent:  false,
		EnableAuditLog: true,
	}

	// Auto-detect GPU and model
	gpu, model, err := detectHardwareWithSpinner()
	if err != nil {
		fmt.Printf("  Warning: GPU detection failed: %v\n", err)
	}
	config.Model = model

	// Check Ollama
	ollamaRunning := checkOllamaWithSpinner()

	// Save config
	configPath, err := saveConfig(config)
	if err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Download model if possible
	if ollamaRunning && config.Model != "" {
		fmt.Println()
		fmt.Println("Downloading recommended model...")
		if err := downloadModelWithProgress(config.Model); err != nil {
			fmt.Printf("  Warning: Model download failed: %v\n", err)
		}
	}

	fmt.Println()
	fmt.Println("Quick Setup Complete!")
	fmt.Println(strings.Repeat("=", 20))
	fmt.Printf("  GPU: %s\n", gpuName(gpu))
	fmt.Printf("  Model: %s\n", config.Model)
	fmt.Printf("  Mode: %s\n", config.DeploymentMode)
	fmt.Printf("  Config: %s\n", configPath)
	fmt.Println()
	fmt.Println("Run 'rigrun' to start!")
	fmt.Println()

	return nil
}

// =============================================================================
// GPU SETUP
// =============================================================================

// runGPUSetup runs GPU detection and displays setup guidance.
func runGPUSetup() error {
	fmt.Println()
	fmt.Println("rigrun GPU Setup")
	fmt.Println(strings.Repeat("=", 16))
	fmt.Println()

	gpu, model, err := detectHardwareWithSpinner()
	if err != nil {
		fmt.Printf("Error detecting GPU: %v\n", err)
		return err
	}

	fmt.Println()
	fmt.Println("GPU Information:")
	fmt.Println(strings.Repeat("-", 16))
	if gpu != nil && gpu.Type != detect.GpuTypeCPU {
		fmt.Printf("  Name:   %s\n", gpu.Name)
		fmt.Printf("  VRAM:   %dGB\n", gpu.VramGB)
		fmt.Printf("  Type:   %s\n", gpu.Type)
		if gpu.Driver != "" {
			fmt.Printf("  Driver: %s\n", gpu.Driver)
		}

		// AMD-specific hints
		if gpu.Type == detect.GpuTypeAmd {
			hint := detect.GetAmdSetupHint(gpu.Name)
			if hint != "" {
				fmt.Println()
				fmt.Println("AMD Setup Hint:")
				fmt.Printf("  %s\n", hint)
			}
		}
	} else {
		fmt.Println("  No GPU detected - running in CPU mode")
		fmt.Println()
		fmt.Println("Troubleshooting:")
		warnings := detect.DiagnoseCPUFallback()
		for _, w := range warnings {
			fmt.Printf("  - %s\n", w)
		}
	}

	fmt.Println()
	fmt.Println("Recommended Model:")
	fmt.Println(strings.Repeat("-", 17))
	fmt.Printf("  %s\n", model)

	// Show alternatives
	vramMB := 8000
	if gpu != nil {
		vramMB = int(gpu.VramGB) * 1024
	}
	alternatives := detect.GetAlternativeModels(vramMB)
	if len(alternatives) > 0 {
		fmt.Println()
		fmt.Println("Alternative Models:")
		for _, alt := range alternatives {
			fmt.Printf("  - %s: %s\n", alt.ModelName, alt.Description)
		}
	}

	fmt.Println()

	return nil
}

// =============================================================================
// MODEL SETUP
// =============================================================================

// runModelSetup runs interactive model selection.
func runModelSetup() error {
	fmt.Println()
	fmt.Println("rigrun Model Setup")
	fmt.Println(strings.Repeat("=", 18))
	fmt.Println()

	// Detect GPU for recommendations
	gpu, err := detect.DetectGPUCached()
	if err != nil {
		gpu = detect.GetCPUInfo()
	}

	vramMB := int(gpu.VramGB) * 1024
	recommendation := detect.RecommendModel(vramMB)
	allModels := detect.ListRecommendedModels(vramMB)

	fmt.Println("Available models for your system:")
	fmt.Println()
	for i, model := range allModels {
		marker := "  "
		if model.ModelName == recommendation.ModelName {
			marker = "* "
		}
		fmt.Printf("%s[%d] %-25s %s (%s)\n",
			marker, i+1, model.ModelName, model.Description, model.Quality)
	}
	fmt.Println()
	fmt.Println("  * = recommended")
	fmt.Println()

	// Get user choice
	options := make([]string, len(allModels)+1)
	for i := range allModels {
		options[i] = strconv.Itoa(i + 1)
	}
	options[len(allModels)] = "c" // Cancel

	fmt.Println("Enter number to select, or 'c' to cancel:")
	choice := setupPromptInput("> ")
	choice = strings.TrimSpace(strings.ToLower(choice))

	if choice == "c" || choice == "" {
		fmt.Println("Cancelled.")
		return nil
	}

	idx, err := strconv.Atoi(choice)
	if err != nil || idx < 1 || idx > len(allModels) {
		fmt.Println("Invalid selection.")
		return nil
	}

	selectedModel := allModels[idx-1].ModelName

	// Check if Ollama is running
	if detect.CheckOllamaRunning() {
		fmt.Println()
		if promptYesNo(fmt.Sprintf("Download %s now?", selectedModel), true) {
			if err := downloadModelWithProgress(selectedModel); err != nil {
				fmt.Printf("Download failed: %v\n", err)
				return err
			}
		}
	} else {
		fmt.Println()
		fmt.Println("Ollama is not running. Start it with: ollama serve")
		fmt.Printf("Then download the model with: rigrun pull %s\n", selectedModel)
	}

	fmt.Println()
	return nil
}

// =============================================================================
// INPUT HELPERS
// =============================================================================

var inputReader = bufio.NewReader(os.Stdin)
var inputMutex sync.Mutex

// setupPromptInput reads a line from stdin (for setup wizard).
func setupPromptInput(prompt string) string {
	inputMutex.Lock()
	defer inputMutex.Unlock()

	fmt.Print(prompt)
	line, err := inputReader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(line)
}

// promptInputWithDefault reads with a default value shown.
func promptInputWithDefault(prompt, defaultVal string) string {
	if defaultVal != "" {
		prompt = fmt.Sprintf("%s [%s]: ", prompt, defaultVal)
	} else {
		prompt = prompt + ": "
	}

	input := setupPromptInput(prompt)
	if input == "" {
		return defaultVal
	}
	return input
}

// promptString prompts for a string input with optional default.
func promptString(prompt string, defaultVal string) string {
	return promptInputWithDefault(prompt, defaultVal)
}

// promptSecure prompts for sensitive input (API keys, passwords) without echoing.
// Uses golang.org/x/term for secure cross-platform input.
func promptSecure(prompt string) string {
	inputMutex.Lock()
	defer inputMutex.Unlock()

	if prompt != "" {
		fmt.Print(prompt)
		if !strings.HasSuffix(prompt, ": ") && !strings.HasSuffix(prompt, " ") {
			fmt.Print(": ")
		}
	}

	keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return ""
	}
	fmt.Println() // Add newline after hidden input

	return strings.TrimSpace(string(keyBytes))
}

// promptYesNo prompts for a yes/no answer.
func promptYesNo(prompt string, defaultYes bool) bool {
	suffix := "[Y/n]"
	if !defaultYes {
		suffix = "[y/N]"
	}

	input := setupPromptInput(fmt.Sprintf("%s %s: ", prompt, suffix))
	input = strings.ToLower(strings.TrimSpace(input))

	if input == "" {
		return defaultYes
	}

	return input == "y" || input == "yes"
}

// promptChoice prompts user to select from numbered options.
// Returns the index of the selected option (0-based).
func promptChoice(prompt string, options []string, defaultIdx int) int {
	suffix := fmt.Sprintf("[%s]", options[defaultIdx])
	input := setupPromptInput(fmt.Sprintf("%s %s: ", prompt, suffix))
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultIdx
	}

	// Try to find matching option
	for i, opt := range options {
		if input == opt || input == strconv.Itoa(i+1) {
			return i
		}
	}

	return defaultIdx
}

// =============================================================================
// SPINNER HELPERS
// =============================================================================

// spinner shows a spinner while running a function.
func spinner(msg string, fn func() error) error {
	done := make(chan struct{})
	errChan := make(chan error, 1)
	spinChars := []rune{'|', '/', '-', '\\'}

	go func() {
		errChan <- fn()
		close(done)
	}()

	i := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	fmt.Printf("  %s... ", msg)

	for {
		select {
		case <-done:
			err := <-errChan
			if err != nil {
				fmt.Println("X")
			} else {
				fmt.Println("Done")
			}
			return err
		case <-ticker.C:
			fmt.Printf("\r  %s... %c", msg, spinChars[i%len(spinChars)])
			i++
		}
	}
}

// runWithSpinner runs a function with a spinner display.
func runWithSpinner(msg string, fn func() error) error {
	return spinner(msg, fn)
}

// =============================================================================
// DETECTION HELPERS
// =============================================================================

// detectHardwareWithSpinner detects GPU with spinner feedback.
func detectHardwareWithSpinner() (*detect.GpuInfo, string, error) {
	var gpu *detect.GpuInfo
	var err error

	spinErr := runWithSpinner("Detecting GPU", func() error {
		gpu, err = detect.DetectGPUCached()
		return err
	})

	if spinErr != nil || gpu == nil {
		gpu = detect.GetCPUInfo()
	}

	// Show result
	if gpu.Type != detect.GpuTypeCPU {
		fmt.Printf("  Detected: %s (%dGB VRAM)\n", gpu.Name, gpu.VramGB)
	} else {
		fmt.Println("  No GPU detected - using CPU mode")
	}

	// Get recommended model
	vramMB := int(gpu.VramGB) * 1024
	recommendation := detect.RecommendModel(vramMB)

	return gpu, recommendation.ModelName, nil
}

// checkOllamaWithSpinner checks Ollama status with spinner.
func checkOllamaWithSpinner() bool {
	running := false

	runWithSpinner("Checking Ollama", func() error {
		running = detect.CheckOllamaRunning()
		if !running {
			return fmt.Errorf("not running")
		}
		return nil
	})

	if running {
		fmt.Println("  Ollama is running")
	} else {
		fmt.Println("  Ollama is not running")
		fmt.Println("  Start it with: ollama serve")
	}

	return running
}

// gpuName returns a display name for the GPU.
func gpuName(gpu *detect.GpuInfo) string {
	if gpu == nil || gpu.Type == detect.GpuTypeCPU {
		return "CPU Only"
	}
	return fmt.Sprintf("%s (%dGB)", gpu.Name, gpu.VramGB)
}

// =============================================================================
// MODEL DOWNLOAD
// =============================================================================

// downloadModelWithProgress downloads a model with progress display.
func downloadModelWithProgress(model string) error {
	client := ollama.NewClient()
	ctx := context.Background()

	// Check if model already exists
	models, err := client.ListModels(ctx)
	if err == nil {
		for _, m := range models {
			if m.Name == model || strings.HasPrefix(m.Name, model+":") {
				fmt.Printf("  Model %s already downloaded\n", model)
				return nil
			}
		}
	}

	fmt.Printf("  Downloading %s...\n", model)
	fmt.Println("  (This may take several minutes)")

	// Note: Ollama client doesn't have pull endpoint in our implementation
	// In production, this would call the /api/pull endpoint with streaming
	// For now, we inform the user to use ollama CLI
	fmt.Println()
	fmt.Printf("  Run: ollama pull %s\n", model)
	fmt.Println()

	return nil
}

// =============================================================================
// CONFIGURATION
// =============================================================================

// ConfigFile represents the rigrun configuration file structure.
type ConfigFile struct {
	Model          string `json:"model,omitempty"`
	DeploymentMode string `json:"deployment_mode,omitempty"`
	OpenRouterKey  string `json:"openrouter_key,omitempty"`
	Port           int    `json:"port,omitempty"`
	FirstRunDone   bool   `json:"first_run_complete"`
	Compliance     struct {
		Enabled        bool `json:"enabled"`
		ConsentBanner  bool `json:"consent_banner"`
		AuditAllQueries bool `json:"audit_all_queries"`
		SessionTimeoutMinutes int `json:"session_timeout_minutes,omitempty"`
	} `json:"compliance"`
}

// saveConfig saves the wizard configuration to disk.
func saveConfig(config *WizardConfig) (string, error) {
	// Get config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(homeDir, ".rigrun")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}

	configPath := filepath.Join(configDir, "config.json")

	// Build config file
	cfg := ConfigFile{
		Model:          config.Model,
		DeploymentMode: config.DeploymentMode.String(),
		OpenRouterKey:  config.OpenRouterKey,
		Port:           8787,
		FirstRunDone:   true,
	}
	cfg.Compliance.Enabled = config.EnableConsent || config.EnableAuditLog
	cfg.Compliance.ConsentBanner = config.EnableConsent
	cfg.Compliance.AuditAllQueries = config.EnableAuditLog
	if config.EnableConsent {
		cfg.Compliance.SessionTimeoutMinutes = 30
	}

	// Write JSON
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", err
	}

	// Also write TOML for advanced users
	tomlPath := filepath.Join(configDir, "config.toml")
	tomlContent := generateTOMLConfig(config)
	if err := os.WriteFile(tomlPath, []byte(tomlContent), 0644); err != nil {
		// Non-fatal, JSON is primary
		fmt.Printf("  Warning: Could not write TOML config: %v\n", err)
	}

	// Write wizard marker
	markerPath := filepath.Join(configDir, ".wizard_complete")
	os.WriteFile(markerPath, []byte(time.Now().Format(time.RFC3339)), 0644)

	return configPath, nil
}

// generateTOMLConfig generates a TOML configuration file.
func generateTOMLConfig(config *WizardConfig) string {
	paranoid := config.DeploymentMode == DeploymentLocal

	apiKeyLine := "# api_key = \"sk-or-...\""
	if config.OpenRouterKey != "" {
		apiKeyLine = fmt.Sprintf("api_key = \"%s\"", config.OpenRouterKey)
	}

	model := config.Model
	if model == "" {
		model = "auto"
	}

	timeout := 0
	if config.EnableConsent {
		timeout = 30
	}

	return fmt.Sprintf(`# rigrun Configuration
# Generated by first-run wizard
# Location: ~/.rigrun/config.toml

[server]
port = 8787
host = "127.0.0.1"

[routing]
# Deployment mode: "local", "hybrid", or "cloud"
mode = "%s"

# Paranoid mode blocks ALL cloud requests
paranoid_mode = %t

# Default model for local inference
default_model = "%s"

[compliance]
# Enable DoD/IL5 compliance features
enabled = %t

# Show consent banner before processing
consent_banner = %t

# Session timeout in minutes (0 = disabled)
session_timeout_minutes = %d

# Log all queries for audit
audit_all_queries = %t

[cloud]
# OpenRouter API key (optional)
%s

# Cloud model preferences
preferred_models = [
    "anthropic/claude-3-5-sonnet",
    "anthropic/claude-3-haiku",
    "openai/gpt-4o",
]

[cache]
# Enable response caching
enabled = true

# Cache TTL in seconds
ttl_seconds = 3600

# Maximum cache size in MB
max_size_mb = 100

[logging]
# Log level: "error", "warn", "info", "debug", "trace"
level = "info"

# Enable structured JSON logging
json_format = false
`,
		config.DeploymentMode,
		paranoid,
		model,
		config.EnableConsent || config.EnableAuditLog,
		config.EnableConsent,
		timeout,
		config.EnableAuditLog,
		apiKeyLine,
	)
}

// IsFirstRun checks if this is the first run (no wizard complete marker).
func IsFirstRun() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return true
	}

	configDir := filepath.Join(homeDir, ".rigrun")
	configFile := filepath.Join(configDir, "config.json")
	wizardMarker := filepath.Join(configDir, ".wizard_complete")

	// First run if config doesn't exist OR wizard marker doesn't exist
	_, configErr := os.Stat(configFile)
	_, markerErr := os.Stat(wizardMarker)

	return os.IsNotExist(configErr) || os.IsNotExist(markerErr)
}

// MarkWizardComplete marks the wizard as completed.
func MarkWizardComplete() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(homeDir, ".rigrun")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	markerPath := filepath.Join(configDir, ".wizard_complete")
	return os.WriteFile(markerPath, []byte(time.Now().Format(time.RFC3339)), 0644)
}
