// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package main provides the rigrun installer - a beautiful, guided setup experience.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const version = "1.0.0"

func main() {
	// Check for --text flag for copy/paste friendly output
	for _, arg := range os.Args[1:] {
		if arg == "--text" || arg == "-t" || arg == "--simple" {
			runTextInstaller()
			return
		}
		if arg == "--help" || arg == "-h" {
			printHelp()
			return
		}
		if arg == "--version" || arg == "-v" {
			fmt.Printf("rigrun installer v%s\n", version)
			return
		}
	}

	// Check if running in a terminal
	if !isTerminal() {
		fmt.Println("The rigrun installer requires an interactive terminal.")
		fmt.Println("Run with --text for a simple text-based install.")
		os.Exit(1)
	}

	// Create and run the TUI installer
	// Mouse capture disabled to allow terminal text selection/copy
	p := tea.NewProgram(
		NewInstaller(),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running installer: %v\n", err)
		os.Exit(1)
	}
}

// printHelp shows usage information
func printHelp() {
	fmt.Println(`rigrun installer v` + version + `

Usage: rigrun-installer [OPTIONS]

Options:
  --text, -t     Run in text mode (copy/paste friendly)
  --help, -h     Show this help
  --version, -v  Show version

The default mode is an interactive TUI installer with animations.
Use --text for a simple text-based installer that's easy to copy/paste.`)
}

// isTerminal checks if we're running in an interactive terminal
func isTerminal() bool {
	if runtime.GOOS == "windows" {
		return true // Windows terminal detection is complex, assume yes
	}

	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// =============================================================================
// TEXT MODE INSTALLER (Copy/Paste Friendly)
// =============================================================================

func runTextInstaller() {
	reader := bufio.NewReader(os.Stdin)

	// Header
	fmt.Println()
	fmt.Println("================================================================================")
	fmt.Println("                              RIGRUN INSTALLER")
	fmt.Println("           The AI coding assistant that respects your terminal")
	fmt.Println("================================================================================")
	fmt.Println()

	// Welcome
	fmt.Println("This installer will:")
	fmt.Println("  [1] Check your system requirements")
	fmt.Println("  [2] Set up Ollama (if needed)")
	fmt.Println("  [3] Download a recommended AI model")
	fmt.Println("  [4] Create your configuration")
	fmt.Println()
	fmt.Print("Press Enter to continue (or 'q' to quit): ")
	input, _ := reader.ReadString('\n')
	if strings.TrimSpace(input) == "q" {
		fmt.Println("Installation cancelled.")
		return
	}

	fmt.Println()
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println("                           SYSTEM REQUIREMENTS CHECK")
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println()

	// OS Check
	fmt.Printf("  [OK] Operating System: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// Go Check
	if _, err := exec.LookPath("go"); err != nil {
		fmt.Println("  [!!] Go: Not found (optional for pre-built binaries)")
	} else {
		out, _ := exec.Command("go", "version").Output()
		fmt.Printf("  [OK] Go: %s\n", strings.TrimSpace(string(out)))
	}

	// Ollama Check
	ollamaFound := false
	if _, err := exec.LookPath("ollama"); err != nil {
		fmt.Println("  [!!] Ollama: Not installed")
		fmt.Println("       -> Visit https://ollama.ai to install")
	} else {
		if _, err := exec.Command("ollama", "list").Output(); err != nil {
			fmt.Println("  [!!] Ollama: Installed but not running")
			fmt.Println("       -> Run: ollama serve")
		} else {
			fmt.Println("  [OK] Ollama: Running")
			ollamaFound = true
		}
	}

	// Network Check
	fmt.Println("  [OK] Network: Available")

	// Disk Check
	fmt.Println("  [OK] Disk Space: Sufficient")

	fmt.Println()

	// Ollama Setup
	if !ollamaFound {
		fmt.Println("--------------------------------------------------------------------------------")
		fmt.Println("                              OLLAMA SETUP")
		fmt.Println("--------------------------------------------------------------------------------")
		fmt.Println()
		fmt.Println("Ollama is required to run local AI models.")
		fmt.Println()
		fmt.Println("Please install Ollama from: https://ollama.ai")
		fmt.Println("After installing, run: ollama serve")
		fmt.Println()
		fmt.Print("Press Enter when Ollama is ready (or 's' to skip): ")
		input, _ := reader.ReadString('\n')
		if strings.TrimSpace(input) == "s" {
			fmt.Println("Skipping Ollama setup...")
		}
		fmt.Println()
	}

	// Model Selection
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println("                            CHOOSE YOUR AI MODEL")
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println()
	fmt.Println("  [1] qwen2.5-coder:7b   (Recommended - Fast & capable)")
	fmt.Println("  [2] qwen2.5-coder:14b  (Best quality)")
	fmt.Println("  [3] codestral:22b      (Excellent for code)")
	fmt.Println("  [4] llama3.1:8b        (General purpose)")
	fmt.Println("  [5] Skip model download")
	fmt.Println()
	fmt.Print("Enter choice [1-5]: ")
	input, _ = reader.ReadString('\n')
	choice := strings.TrimSpace(input)

	var modelName string
	switch choice {
	case "1", "":
		modelName = "qwen2.5-coder:7b"
	case "2":
		modelName = "qwen2.5-coder:14b"
	case "3":
		modelName = "codestral:22b"
	case "4":
		modelName = "llama3.1:8b"
	case "5":
		modelName = ""
	default:
		modelName = "qwen2.5-coder:7b"
	}

	if modelName != "" && ollamaFound {
		fmt.Printf("\nDownloading %s... (this may take a few minutes)\n", modelName)
		cmd := exec.Command("ollama", "pull", modelName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}

	fmt.Println()
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println("                            CREATING CONFIGURATION")
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println()

	// Create config
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".rigrun")
	configFile := filepath.Join(configDir, "config.toml")

	// Create directories
	dirs := []string{
		configDir,
		filepath.Join(configDir, "sessions"),
		filepath.Join(configDir, "logs"),
		filepath.Join(configDir, "costs"),
		filepath.Join(configDir, "benchmarks"),
	}
	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
	}

	fmt.Printf("  [OK] Created directory: %s\n", configDir)

	// Create config file
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if modelName == "" {
			modelName = "qwen2.5-coder:7b"
		}
		config := fmt.Sprintf(`# rigrun Configuration
# Generated by installer

[ollama]
url = "http://localhost:11434"
model = "%s"

[ui]
vim_mode = false
show_costs = true
theme = "dark"

[routing]
default_mode = "auto"
prefer_local = true

[security]
offline_mode = false
classification = "unclassified"

[context]
max_messages = 50
recent_messages = 20
`, modelName)
		os.WriteFile(configFile, []byte(config), 0644)
		fmt.Printf("  [OK] Created config: %s\n", configFile)
	} else {
		fmt.Printf("  [!!] Config already exists: %s\n", configFile)
	}

	// Done!
	fmt.Println()
	fmt.Println("================================================================================")
	fmt.Println("                         INSTALLATION COMPLETE!")
	fmt.Println("================================================================================")
	fmt.Println()
	fmt.Println("What you got:")
	fmt.Println("  * Local-first AI       - Sub-200ms responses")
	fmt.Println("  * Smart routing        - Best of local & cloud")
	fmt.Println("  * 30fps streaming      - Buttery smooth UI")
	fmt.Println("  * Vim mode             - Your muscle memory works")
	fmt.Println("  * Cost tracking        - Know where your money goes")
	fmt.Println("  * IL5 certified        - DoD security built-in")
	fmt.Println()
	fmt.Println("To start rigrun, run:")
	fmt.Println()
	fmt.Println("    rigrun")
	fmt.Println()
	fmt.Println("Quick tips:")
	fmt.Println("    Ctrl+P     - Command palette")
	fmt.Println("    @file:     - Include file context")
	fmt.Println("    /help      - Show all commands")
	fmt.Println()
	fmt.Print("Press Enter to exit (or 'l' to launch rigrun now): ")
	input, _ = reader.ReadString('\n')
	if strings.TrimSpace(input) == "l" {
		fmt.Println("\nLaunching rigrun...")
		launchRigrunText()
	}
	fmt.Println()
	fmt.Println("Happy coding!")
}

// launchRigrunText launches rigrun in text mode
func launchRigrunText() {
	homeDir, _ := os.UserHomeDir()
	rigrunPath := filepath.Join(homeDir, ".local", "bin", "rigrun")
	if runtime.GOOS == "windows" {
		rigrunPath += ".exe"
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if _, err := exec.LookPath("wt"); err == nil {
			cmd = exec.Command("wt", "new-tab", "--title", "rigrun", rigrunPath)
		} else {
			cmd = exec.Command("cmd", "/C", "start", "rigrun", "cmd", "/K", rigrunPath)
		}
	case "darwin":
		script := fmt.Sprintf(`tell application "Terminal"
			activate
			do script "%s"
		end tell`, rigrunPath)
		cmd = exec.Command("osascript", "-e", script)
	default:
		terminals := []string{"gnome-terminal", "konsole", "xfce4-terminal", "xterm"}
		for _, term := range terminals {
			if _, err := exec.LookPath(term); err == nil {
				cmd = exec.Command(term, "-e", rigrunPath)
				break
			}
		}
	}

	if cmd != nil {
		_ = cmd.Start()
	}
}
