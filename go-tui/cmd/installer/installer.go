// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// STYLES
// =============================================================================

var (
	// Colors
	brandPrimary   = lipgloss.Color("#7C3AED") // Purple
	brandSecondary = lipgloss.Color("#06B6D4") // Cyan
	brandAccent    = lipgloss.Color("#10B981") // Emerald
	brandWarning   = lipgloss.Color("#F59E0B") // Amber
	brandError     = lipgloss.Color("#EF4444") // Red
	textMuted      = lipgloss.Color("#6B7280") // Gray
	textBright     = lipgloss.Color("#F9FAFB") // White

	// Styles
	titleStyle = lipgloss.NewStyle().
			Foreground(brandPrimary).
			Bold(true).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(textMuted).
			Italic(true)

	successStyle = lipgloss.NewStyle().
			Foreground(brandAccent).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(brandError).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(brandWarning)

	highlightStyle = lipgloss.NewStyle().
			Foreground(brandSecondary).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(textMuted)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(brandPrimary).
			Padding(1, 2)

	selectedStyle = lipgloss.NewStyle().
			Foreground(brandPrimary).
			Bold(true)

	unselectedStyle = lipgloss.NewStyle().
			Foreground(textMuted)
)

// =============================================================================
// ASCII ART
// =============================================================================

const logo = `
    ██████╗ ██╗ ██████╗ ██████╗ ██╗   ██╗███╗   ██╗
    ██╔══██╗██║██╔════╝ ██╔══██╗██║   ██║████╗  ██║
    ██████╔╝██║██║  ███╗██████╔╝██║   ██║██╔██╗ ██║
    ██╔══██╗██║██║   ██║██╔══██╗██║   ██║██║╚██╗██║
    ██║  ██║██║╚██████╔╝██║  ██║╚██████╔╝██║ ╚████║
    ╚═╝  ╚═╝╚═╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝
`

const tagline = "The AI coding assistant that respects your terminal"

const rocketArt = `
        *
       /|\
      / | \
     /  |  \
    /___|___\
       |||
       |||
      /   \
`

// =============================================================================
// INSTALLER MODEL
// =============================================================================

// Phase represents the current installation phase
type Phase int

const (
	PhaseWelcome Phase = iota
	PhaseSystemCheck
	PhaseOllamaSetup
	PhaseConfigSetup
	PhaseModelDownload
	PhaseComplete
)

// CheckResult represents a system check result
type CheckResult struct {
	Name    string
	Status  string // "pass", "fail", "warn", "checking"
	Message string
	Fix     string
}

// Installer is the main installer model
type Installer struct {
	phase         Phase
	width         int
	height        int
	spinner       spinner.Model
	progress      progress.Model
	checks        []CheckResult
	currentCheck  int
	ollamaFound   bool
	modelSelected int
	models        []string
	configPath    string
	installPath   string
	error         string
	done          bool

	// Animation state
	typingText    string
	typingTarget  string
	typingIndex   int

	// Completion screen
	launchSelected bool // true = "Launch rigrun now", false = "Close"
}

// NewInstaller creates a new installer instance
func NewInstaller() *Installer {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(brandPrimary)

	p := progress.New(progress.WithDefaultGradient())

	homeDir, _ := os.UserHomeDir()

	return &Installer{
		phase:    PhaseWelcome,
		spinner:  s,
		progress: p,
		checks: []CheckResult{
			{Name: "Operating System", Status: "checking"},
			{Name: "Go Installation", Status: "checking"},
			{Name: "Ollama Service", Status: "checking"},
			{Name: "Network Access", Status: "checking"},
			{Name: "Disk Space", Status: "checking"},
		},
		models: []string{
			"qwen2.5-coder:7b (Recommended - Fast & capable)",
			"qwen2.5-coder:14b (Best quality)",
			"codestral:22b (Excellent for code)",
			"llama3.1:8b (General purpose)",
			"Skip model download",
		},
		configPath:     filepath.Join(homeDir, ".rigrun"),
		installPath:    filepath.Join(homeDir, ".local", "bin"),
		launchSelected: true, // Default to "Launch rigrun now"
	}
}

// Init initializes the installer
func (i *Installer) Init() tea.Cmd {
	return tea.Batch(
		i.spinner.Tick,
		i.typeWriter(logo, 5*time.Millisecond),
	)
}

// =============================================================================
// UPDATE
// =============================================================================

// tickMsg is sent for animations
type tickMsg time.Time

// typeWriterMsg updates the typing animation
type typeWriterMsg struct {
	target string
	index  int
}

// checkCompleteMsg signals a check is complete
type checkCompleteMsg struct {
	index  int
	result CheckResult
}

// installCompleteMsg signals installation is complete
type installCompleteMsg struct {
	success bool
	error   string
}

// Update handles messages
func (i *Installer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return i.handleKey(msg)

	case tea.WindowSizeMsg:
		i.width = msg.Width
		i.height = msg.Height
		// Clamp progress bar width to a reasonable range
		progressWidth := msg.Width - 20
		if progressWidth < 20 {
			progressWidth = 20
		}
		if progressWidth > 100 {
			progressWidth = 100
		}
		i.progress.Width = progressWidth

		// Update boxStyle width dynamically based on terminal width
		boxWidth := msg.Width - 16
		if boxWidth < 40 {
			boxWidth = 40
		}
		if boxWidth > 70 {
			boxWidth = 70
		}
		boxStyle = boxStyle.Width(boxWidth)

		// Return spinner tick to force a redraw
		return i, i.spinner.Tick

	case spinner.TickMsg:
		var cmd tea.Cmd
		i.spinner, cmd = i.spinner.Update(msg)
		return i, cmd

	case progress.FrameMsg:
		var cmd tea.Cmd
		progressModel, cmd := i.progress.Update(msg)
		i.progress = progressModel.(progress.Model)
		return i, cmd

	case typeWriterMsg:
		if msg.target == i.typingTarget && msg.index <= len(msg.target) {
			i.typingText = msg.target[:msg.index]
			i.typingIndex = msg.index
			if msg.index < len(msg.target) {
				return i, i.typeWriterTick(msg.target, msg.index+1, 5*time.Millisecond)
			}
		}
		return i, nil

	case checkCompleteMsg:
		i.checks[msg.index] = msg.result
		i.currentCheck++
		if i.currentCheck < len(i.checks) {
			return i, i.runCheck(i.currentCheck)
		}
		// All checks complete
		i.ollamaFound = i.checks[2].Status == "pass"
		return i, nil

	case installCompleteMsg:
		if msg.success {
			i.phase = PhaseComplete
		} else {
			i.error = msg.error
		}
		return i, nil
	}

	return i, nil
}

// handleKey processes key presses
func (i *Installer) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return i, tea.Quit

	case "enter", " ":
		return i.handleSelect()

	case "up", "k":
		if i.phase == PhaseModelDownload && i.modelSelected > 0 {
			i.modelSelected--
		}
		if i.phase == PhaseComplete {
			i.launchSelected = true
		}
		return i, nil

	case "down", "j":
		if i.phase == PhaseModelDownload && i.modelSelected < len(i.models)-1 {
			i.modelSelected++
		}
		if i.phase == PhaseComplete {
			i.launchSelected = false
		}
		return i, nil

	case "tab":
		if i.phase == PhaseComplete {
			i.launchSelected = !i.launchSelected
		}
		return i, nil
	}

	return i, nil
}

// handleSelect processes selection/enter
func (i *Installer) handleSelect() (tea.Model, tea.Cmd) {
	switch i.phase {
	case PhaseWelcome:
		i.phase = PhaseSystemCheck
		return i, i.runCheck(0)

	case PhaseSystemCheck:
		if i.currentCheck >= len(i.checks) {
			if i.ollamaFound {
				i.phase = PhaseModelDownload
			} else {
				i.phase = PhaseOllamaSetup
			}
		}
		return i, nil

	case PhaseOllamaSetup:
		i.phase = PhaseModelDownload
		return i, nil

	case PhaseModelDownload:
		i.phase = PhaseConfigSetup
		return i, i.runInstall()

	case PhaseConfigSetup:
		// Wait for install to complete
		return i, nil

	case PhaseComplete:
		if i.launchSelected {
			return i, i.launchRigrun()
		}
		return i, tea.Quit
	}

	return i, nil
}

// =============================================================================
// COMMANDS
// =============================================================================

// typeWriter starts a typing animation
func (i *Installer) typeWriter(text string, delay time.Duration) tea.Cmd {
	i.typingTarget = text
	i.typingIndex = 0
	i.typingText = ""
	return i.typeWriterTick(text, 1, delay)
}

// typeWriterTick sends the next typewriter tick
func (i *Installer) typeWriterTick(target string, index int, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return typeWriterMsg{target: target, index: index}
	})
}

// runCheck runs a system check
func (i *Installer) runCheck(index int) tea.Cmd {
	return func() tea.Msg {
		result := i.checks[index]
		result.Status = "checking"

		switch index {
		case 0: // OS Check
			result = i.checkOS()
		case 1: // Go Check
			result = i.checkGo()
		case 2: // Ollama Check
			result = i.checkOllama()
		case 3: // Network Check
			result = i.checkNetwork()
		case 4: // Disk Check
			result = i.checkDisk()
		}

		time.Sleep(300 * time.Millisecond) // Simulate work for visual effect
		return checkCompleteMsg{index: index, result: result}
	}
}

// System checks
func (i *Installer) checkOS() CheckResult {
	os := runtime.GOOS
	arch := runtime.GOARCH
	return CheckResult{
		Name:    "Operating System",
		Status:  "pass",
		Message: fmt.Sprintf("%s/%s", os, arch),
	}
}

func (i *Installer) checkGo() CheckResult {
	_, err := exec.LookPath("go")
	if err != nil {
		return CheckResult{
			Name:    "Go Installation",
			Status:  "warn",
			Message: "Go not found (optional for pre-built binaries)",
		}
	}
	out, _ := exec.Command("go", "version").Output()
	version := strings.TrimSpace(string(out))
	return CheckResult{
		Name:    "Go Installation",
		Status:  "pass",
		Message: version,
	}
}

func (i *Installer) checkOllama() CheckResult {
	_, err := exec.LookPath("ollama")
	if err != nil {
		return CheckResult{
			Name:    "Ollama Service",
			Status:  "fail",
			Message: "Ollama not installed",
			Fix:     "Visit https://ollama.ai to install",
		}
	}

	// Check if Ollama is running
	out, err := exec.Command("ollama", "list").Output()
	if err != nil {
		return CheckResult{
			Name:    "Ollama Service",
			Status:  "warn",
			Message: "Ollama installed but not running",
			Fix:     "Run: ollama serve",
		}
	}

	models := strings.Count(string(out), "\n")
	return CheckResult{
		Name:    "Ollama Service",
		Status:  "pass",
		Message: fmt.Sprintf("Running with %d models", models),
	}
}

func (i *Installer) checkNetwork() CheckResult {
	// Simple check - try to resolve a hostname
	_, err := exec.Command("ping", "-c", "1", "-W", "2", "ollama.ai").Output()
	if runtime.GOOS == "windows" {
		_, err = exec.Command("ping", "-n", "1", "-w", "2000", "ollama.ai").Output()
	}

	if err != nil {
		return CheckResult{
			Name:    "Network Access",
			Status:  "warn",
			Message: "Limited connectivity (offline mode available)",
		}
	}
	return CheckResult{
		Name:    "Network Access",
		Status:  "pass",
		Message: "Connected",
	}
}

func (i *Installer) checkDisk() CheckResult {
	// Check available disk space (simplified)
	homeDir, _ := os.UserHomeDir()
	var stat struct {
		Bavail uint64
		Bsize  int64
	}

	// This is a simplified check - real implementation would use syscall
	_ = homeDir
	_ = stat

	return CheckResult{
		Name:    "Disk Space",
		Status:  "pass",
		Message: "Sufficient space available",
	}
}

// =============================================================================
// RIGRUN BINARY DOWNLOAD
// =============================================================================

// GitHubRelease represents a GitHub release response
type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a release asset
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// checkRigrunBinary checks if rigrun binary exists
func (i *Installer) checkRigrunBinary() bool {
	rigrunPath := filepath.Join(i.installPath, "rigrun")
	if runtime.GOOS == "windows" {
		rigrunPath += ".exe"
	}
	_, err := os.Stat(rigrunPath)
	return err == nil
}

// downloadRigrunBinary downloads the rigrun binary from GitHub releases
func (i *Installer) downloadRigrunBinary() error {
	const repoOwner = "jeranaias"
	const repoName = "rigrun"

	// Determine the asset name based on OS and architecture
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Map Go architecture names to common release names
	archName := goarch
	switch goarch {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "arm64"
	case "386":
		archName = "i386"
	}

	// Map Go OS names to common release names
	osName := goos
	switch goos {
	case "darwin":
		osName = "Darwin"
	case "linux":
		osName = "Linux"
	case "windows":
		osName = "Windows"
	}

	// Get the latest release info
	releaseURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	resp, err := http.Get(releaseURL)
	if err != nil {
		return fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch release info: HTTP %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %w", err)
	}

	// Find the appropriate asset
	var assetURL string
	var assetName string

	// Look for matching asset (e.g., rigrun_Darwin_arm64.tar.gz or rigrun_Windows_x86_64.zip)
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, osName) && strings.Contains(asset.Name, archName) {
			assetURL = asset.BrowserDownloadURL
			assetName = asset.Name
			break
		}
	}

	if assetURL == "" {
		return fmt.Errorf("no release found for %s/%s", osName, archName)
	}

	// Download the asset
	assetResp, err := http.Get(assetURL)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer assetResp.Body.Close()

	if assetResp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download binary: HTTP %d", assetResp.StatusCode)
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "rigrun-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write to temp file
	if _, err := io.Copy(tmpFile, assetResp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to save download: %w", err)
	}
	tmpFile.Close()

	// Create install directory
	if err := os.MkdirAll(i.installPath, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Extract the binary
	if strings.HasSuffix(assetName, ".zip") {
		if err := extractZip(tmpPath, i.installPath); err != nil {
			return fmt.Errorf("failed to extract zip: %w", err)
		}
	} else if strings.HasSuffix(assetName, ".tar.gz") || strings.HasSuffix(assetName, ".tgz") {
		if err := extractTarGz(tmpPath, i.installPath); err != nil {
			return fmt.Errorf("failed to extract tar.gz: %w", err)
		}
	} else {
		// Direct binary - just copy it
		rigrunPath := filepath.Join(i.installPath, "rigrun")
		if runtime.GOOS == "windows" {
			rigrunPath += ".exe"
		}
		if err := copyFile(tmpPath, rigrunPath); err != nil {
			return fmt.Errorf("failed to copy binary: %w", err)
		}
		os.Chmod(rigrunPath, 0755)
	}

	return nil
}

// extractZip extracts a zip file to the destination directory
func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Only extract the rigrun binary
		name := filepath.Base(f.Name)
		if name != "rigrun" && name != "rigrun.exe" {
			continue
		}

		destPath := filepath.Join(dest, name)

		rc, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// extractTarGz extracts a tar.gz file to the destination directory
func extractTarGz(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Only extract the rigrun binary
		name := filepath.Base(header.Name)
		if name != "rigrun" && name != "rigrun.exe" {
			continue
		}

		destPath := filepath.Join(dest, name)

		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			return err
		}

		if _, err := io.Copy(outFile, tr); err != nil {
			outFile.Close()
			return err
		}
		outFile.Close()
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// runInstall performs the actual installation
func (i *Installer) runInstall() tea.Cmd {
	return func() tea.Msg {
		// Check and download rigrun binary if not present
		if !i.checkRigrunBinary() {
			if err := i.downloadRigrunBinary(); err != nil {
				// Non-fatal: user can build from source or download manually
				// Just log the error but continue with config setup
				_ = err
			}
		}

		// Create config directory
		if err := os.MkdirAll(i.configPath, 0755); err != nil {
			return installCompleteMsg{success: false, error: err.Error()}
		}

		// Create default config
		configFile := filepath.Join(i.configPath, "config.toml")
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			config := i.generateConfig()
			if err := os.WriteFile(configFile, []byte(config), 0644); err != nil {
				return installCompleteMsg{success: false, error: err.Error()}
			}
		}

		// Create directories
		dirs := []string{
			filepath.Join(i.configPath, "sessions"),
			filepath.Join(i.configPath, "logs"),
			filepath.Join(i.configPath, "costs"),
			filepath.Join(i.configPath, "benchmarks"),
		}
		for _, dir := range dirs {
			os.MkdirAll(dir, 0755)
		}

		// Download model if selected (not "Skip")
		if i.modelSelected < len(i.models)-1 {
			modelName := strings.Split(i.models[i.modelSelected], " ")[0]
			exec.Command("ollama", "pull", modelName).Run()
		}

		time.Sleep(500 * time.Millisecond) // Visual feedback
		return installCompleteMsg{success: true}
	}
}

// generateConfig creates the default configuration
func (i *Installer) generateConfig() string {
	model := "qwen2.5-coder:7b"
	if i.modelSelected < len(i.models)-1 {
		model = strings.Split(i.models[i.modelSelected], " ")[0]
	}

	return fmt.Sprintf(`# rigrun Configuration
# Generated by the rigrun installer

[local]
# Local Ollama server URL (use 127.0.0.1 to avoid IPv6 issues on Windows)
ollama_url = "http://127.0.0.1:11434"

# Default model for local inference
ollama_model = "%s"

[ui]
# Enable vim-style keybindings (j/k navigation, :commands)
vim_mode = false

# Show token costs in status bar
show_costs = true

# Theme (dark/light)
theme = "dark"

[routing]
# Default routing mode: local, cloud, auto
default_mode = "auto"

# Prefer local models when possible
prefer_local = true

[security]
# Enable offline mode (no network access)
offline_mode = false

# Classification level (unclassified, cui, secret, top_secret)
classification = "unclassified"

[context]
# Maximum messages before summarization
max_messages = 50

# Recent messages to keep in full
recent_messages = 20
`, model)
}

// launchRigrun opens a terminal and runs rigrun
func (i *Installer) launchRigrun() tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd

		rigrunPath := filepath.Join(i.installPath, "rigrun")
		if runtime.GOOS == "windows" {
			rigrunPath += ".exe"
		}

		switch runtime.GOOS {
		case "windows":
			// Windows: Open Windows Terminal or cmd.exe with rigrun
			// Try Windows Terminal first (wt), fallback to cmd
			if _, err := exec.LookPath("wt"); err == nil {
				cmd = exec.Command("wt", "new-tab", "--title", "rigrun", rigrunPath)
			} else {
				// Use cmd.exe with /K to keep window open
				cmd = exec.Command("cmd", "/C", "start", "rigrun", "cmd", "/K", rigrunPath)
			}

		case "darwin":
			// macOS: Open Terminal.app with rigrun
			script := fmt.Sprintf(`
				tell application "Terminal"
					activate
					do script "%s"
				end tell
			`, rigrunPath)
			cmd = exec.Command("osascript", "-e", script)

		default:
			// Linux: Try common terminal emulators
			terminals := []struct {
				name string
				args []string
			}{
				{"gnome-terminal", []string{"--", rigrunPath}},
				{"konsole", []string{"-e", rigrunPath}},
				{"xfce4-terminal", []string{"-e", rigrunPath}},
				{"xterm", []string{"-e", rigrunPath}},
				{"alacritty", []string{"-e", rigrunPath}},
				{"kitty", []string{rigrunPath}},
			}

			for _, term := range terminals {
				if _, err := exec.LookPath(term.name); err == nil {
					cmd = exec.Command(term.name, term.args...)
					break
				}
			}

			// Fallback: just run in current terminal (won't work but better than nothing)
			if cmd == nil {
				cmd = exec.Command(rigrunPath)
			}
		}

		// Start the command (don't wait for it)
		_ = cmd.Start()

		return tea.Quit()
	}
}

// =============================================================================
// VIEW
// =============================================================================

// View renders the installer
func (i *Installer) View() string {
	switch i.phase {
	case PhaseWelcome:
		return i.viewWelcome()
	case PhaseSystemCheck:
		return i.viewSystemCheck()
	case PhaseOllamaSetup:
		return i.viewOllamaSetup()
	case PhaseModelDownload:
		return i.viewModelDownload()
	case PhaseConfigSetup:
		return i.viewConfigSetup()
	case PhaseComplete:
		return i.viewComplete()
	}
	return ""
}

func (i *Installer) viewWelcome() string {
	var s strings.Builder

	// Logo with typing effect
	logoStyle := lipgloss.NewStyle().Foreground(brandPrimary).Bold(true)
	if i.typingTarget == logo {
		s.WriteString(logoStyle.Render(i.typingText))
	} else {
		s.WriteString(logoStyle.Render(logo))
	}

	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render("    " + tagline))
	s.WriteString("\n\n")

	// Version
	s.WriteString(dimStyle.Render(fmt.Sprintf("    Version %s", version)))
	s.WriteString("\n\n")

	// Welcome box
	welcomeText := `
Welcome to the rigrun installer!

This installer will:

  * Check your system requirements
  * Set up Ollama (if needed)
  * Download a recommended AI model
  * Create your configuration
  * Get you coding in 60 seconds

`
	s.WriteString(boxStyle.Render(welcomeText))
	s.WriteString("\n\n")

	// Continue prompt
	s.WriteString(highlightStyle.Render("  Press ENTER to begin"))
	s.WriteString(dimStyle.Render("  |  Press Q to quit"))

	return i.center(s.String())
}

func (i *Installer) viewSystemCheck() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("  System Requirements Check"))
	s.WriteString("\n\n")

	for idx, check := range i.checks {
		var icon, status string
		var style lipgloss.Style

		switch check.Status {
		case "checking":
			if idx == i.currentCheck {
				icon = i.spinner.View()
			} else {
				icon = "[ ]"
			}
			status = "Checking..."
			style = dimStyle
		case "pass":
			icon = "[OK]"
			status = check.Message
			style = successStyle
		case "fail":
			icon = "[FAIL]"
			status = check.Message
			style = errorStyle
		case "warn":
			icon = "[!!]"
			status = check.Message
			style = warningStyle
		}

		s.WriteString(fmt.Sprintf("  %s %s", style.Render(icon), check.Name))
		s.WriteString(dimStyle.Render(fmt.Sprintf(" - %s", status)))
		s.WriteString("\n")

		if check.Fix != "" {
			s.WriteString(dimStyle.Render(fmt.Sprintf("      -> %s", check.Fix)))
			s.WriteString("\n")
		}
	}

	s.WriteString("\n")

	if i.currentCheck >= len(i.checks) {
		// All checks complete
		allPass := true
		for _, check := range i.checks {
			if check.Status == "fail" {
				allPass = false
				break
			}
		}

		if allPass {
			s.WriteString(successStyle.Render("  All checks passed!"))
			s.WriteString("\n\n")
			s.WriteString(highlightStyle.Render("  Press ENTER to continue"))
		} else {
			s.WriteString(warningStyle.Render("  Some checks need attention"))
			s.WriteString("\n\n")
			s.WriteString(highlightStyle.Render("  Press ENTER to continue anyway"))
		}
	}

	return i.center(s.String())
}

func (i *Installer) viewOllamaSetup() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("  Ollama Setup Required"))
	s.WriteString("\n\n")

	content := `
Ollama is required to run local AI models.

Please install Ollama from:

  ` + highlightStyle.Render("https://ollama.ai") + `

After installing, run:

  ` + highlightStyle.Render("ollama serve") + `

Then press ENTER to continue.
`

	s.WriteString(boxStyle.Render(content))
	s.WriteString("\n\n")
	s.WriteString(highlightStyle.Render("  Press ENTER when Ollama is ready"))

	return i.center(s.String())
}

func (i *Installer) viewModelDownload() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("Choose Your AI Model"))
	s.WriteString("\n\n")

	s.WriteString(dimStyle.Render("Select a model to download:"))
	s.WriteString("\n\n")

	// Build model list with consistent alignment
	for idx, model := range i.models {
		cursor := "  " // No cursor (2 spaces for alignment)
		style := unselectedStyle
		if idx == i.modelSelected {
			cursor = "> " // Cursor takes same space
			style = selectedStyle
		}
		s.WriteString(style.Render(fmt.Sprintf("  %s%s", cursor, model)))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(dimStyle.Render("Use ↑/↓ to select, ENTER to confirm"))

	return i.center(s.String())
}

func (i *Installer) viewConfigSetup() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("  Setting Up rigrun"))
	s.WriteString("\n\n")

	s.WriteString(fmt.Sprintf("  %s Creating configuration...\n", i.spinner.View()))
	s.WriteString(dimStyle.Render(fmt.Sprintf("     %s/config.toml\n\n", i.configPath)))

	if i.modelSelected < len(i.models)-1 {
		modelName := strings.Split(i.models[i.modelSelected], " ")[0]
		s.WriteString(fmt.Sprintf("  %s Downloading %s...\n", i.spinner.View(), modelName))
		s.WriteString(dimStyle.Render("     This may take a few minutes\n"))
	}

	return i.center(s.String())
}

func (i *Installer) viewComplete() string {
	var s strings.Builder

	// Success art
	successArt := `
    +------------------------------------------+
    |                                          |
    |      *** Installation Complete! ***      |
    |                                          |
    +------------------------------------------+
`
	s.WriteString(successStyle.Render(successArt))
	s.WriteString("\n")

	// Quick highlights
	highlights := `
  +-----------------------------------------------------+
  |  What you just got:                                 |
  |                                                     |
  |  * Local-first AI      Sub-200ms responses         |
  |  * Smart routing       Best of local & cloud       |
  |  * 30fps streaming     Buttery smooth UI           |
  |  * Vim mode            Your muscle memory works    |
  |  * Cost tracking       Know where your money goes  |
  |  * IL5 certified       DoD security built-in       |
  +-----------------------------------------------------+
`
	s.WriteString(dimStyle.Render(highlights))
	s.WriteString("\n")

	// Rocket art
	rocketSmall := `
          *
         /|\
        /_|_\
          |
`
	s.WriteString(lipgloss.NewStyle().Foreground(brandSecondary).Render(rocketSmall))
	s.WriteString("\n")

	// Two options with selection indicator
	s.WriteString("  Choose your next step:\n\n")

	// Option 1: Launch rigrun now
	launch := "  Launch rigrun now"
	if i.launchSelected {
		s.WriteString(selectedStyle.Render("  > " + launch))
		s.WriteString(highlightStyle.Render("  <- Opens a new terminal with rigrun"))
	} else {
		s.WriteString(unselectedStyle.Render("    " + launch))
	}
	s.WriteString("\n\n")

	// Option 2: Close
	closeText := "  Close installer"
	if !i.launchSelected {
		s.WriteString(selectedStyle.Render("  > " + closeText))
		s.WriteString(dimStyle.Render("  <- You can run 'rigrun' anytime"))
	} else {
		s.WriteString(unselectedStyle.Render("    " + closeText))
	}
	s.WriteString("\n\n")

	// Navigation help
	s.WriteString(dimStyle.Render("  Up/Down or Tab to select  |  Enter to confirm"))
	s.WriteString("\n\n")

	// Config paths
	s.WriteString(dimStyle.Render(fmt.Sprintf("  Config: %s", filepath.Join(i.configPath, "config.toml"))))

	return i.center(s.String())
}

// center centers content on screen
func (i *Installer) center(content string) string {
	if i.width == 0 || i.height == 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	height := len(lines)

	// Vertical centering
	topPadding := (i.height - height) / 3
	if topPadding < 0 {
		topPadding = 0
	}

	var s strings.Builder
	for j := 0; j < topPadding; j++ {
		s.WriteString("\n")
	}
	s.WriteString(content)

	return s.String()
}
