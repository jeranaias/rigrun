// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// WELCOME SCREEN MODEL
// =============================================================================

// Welcome is the welcome screen component.
type Welcome struct {
	// Display info
	version     string
	modelName   string
	gpuName     string
	mode        string
	offlineMode bool // IL5 SC-7: True when --no-network flag is active

	// Dimensions
	width  int
	height int

	// Theme
	theme *styles.Theme
}

// NewWelcome creates a new welcome screen.
func NewWelcome(theme *styles.Theme) Welcome {
	return Welcome{
		version:   "dev",
		modelName: "qwen2.5-coder:14b",
		gpuName:   "", // Set externally via SetGPUName()
		mode:      "auto",
		theme:     theme,
	}
}

// SetVersion sets the version string.
func (w *Welcome) SetVersion(version string) {
	w.version = version
}

// SetModelName sets the model name.
func (w *Welcome) SetModelName(name string) {
	w.modelName = name
}

// SetGPUName sets the GPU name.
func (w *Welcome) SetGPUName(name string) {
	w.gpuName = name
}

// SetMode sets the routing mode.
func (w *Welcome) SetMode(mode string) {
	w.mode = mode
}

// SetOfflineMode sets the offline mode state (IL5 SC-7).
func (w *Welcome) SetOfflineMode(offline bool) {
	w.offlineMode = offline
}

// SetSize updates the dimensions.
func (w *Welcome) SetSize(width, height int) {
	w.width = width
	w.height = height
}

// =============================================================================
// BUBBLE TEA INTERFACE
// =============================================================================

// Init initializes the welcome screen.
func (w Welcome) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (w Welcome) Update(msg tea.Msg) (Welcome, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height
	}
	return w, nil
}

// View renders the welcome screen.
// Responsive: adapts to terminal size, minimum 80x24 supported.
func (w Welcome) View() string {
	width := w.width
	if width == 0 {
		width = 80
	}
	height := w.height
	if height == 0 {
		height = 24
	}

	// Build the IS warning banner first
	warningBanner := w.renderISWarning(width, height)
	warningHeight := lipgloss.Height(warningBanner)

	// Calculate remaining height for content
	remainingHeight := height - warningHeight

	// Calculate box width - responsive to terminal width
	boxWidth := 62
	if width < 70 {
		boxWidth = width - 8
	}
	if boxWidth < 40 {
		boxWidth = 40
	}
	if boxWidth > width-4 {
		boxWidth = width - 4
	}

	// Adjust padding for narrow terminals
	horizontalPadding := 4
	verticalPadding := 1
	if width < 70 {
		horizontalPadding = 2
	}

	// Box overhead: 2 (border top/bottom) + 2*verticalPadding
	boxOverhead := 2 + 2*verticalPadding

	// Available lines for content inside the box
	availableContentLines := remainingHeight - boxOverhead

	// Build the content based on available space
	// Calculate what we can fit
	// Logo: 6 lines (without leading newline)
	// Version: 1 line
	// System info: 3 lines (Model, GPU, Mode)
	// Press key: 1 line
	// Spacing between sections: 1 line each (use single newlines when tight)

	var content string
	var contentLines int

	// Minimum viable: logo(6) + spacing(1) + version(1) + spacing(1) + sysinfo(3) + spacing(1) + presskey(1) = 14 lines
	// Compact: logo(6) + version(1) + sysinfo(3) + presskey(1) + 3 single newlines = 14 lines
	// Very compact: compact logo(4) + version(1) + sysinfo(3) + presskey(1) + 3 newlines = 12 lines

	if availableContentLines >= 18 {
		// Full layout with double newlines
		content = w.renderLogo()
		content += "\n\n" + w.renderVersion()
		content += "\n\n" + w.renderSystemInfo()
		content += "\n\n" + w.renderPressKey()
		contentLines = 6 + 2 + 1 + 2 + 3 + 2 + 1 // 17
	} else if availableContentLines >= 14 {
		// Compact: single newlines between sections
		content = w.renderLogo()
		content += "\n" + w.renderVersion()
		content += "\n" + w.renderSystemInfo()
		content += "\n" + w.renderPressKey()
		contentLines = 6 + 1 + 1 + 1 + 3 + 1 + 1 // 14
	} else if availableContentLines >= 10 {
		// Very compact: use compact logo, minimal spacing
		content = w.renderLogoCompact()
		content += "\n" + w.renderVersion()
		content += "\n" + w.renderSystemInfo()
		content += "\n" + w.renderPressKey()
		contentLines = 3 + 1 + 1 + 1 + 3 + 1 + 1 // 11
	} else {
		// Ultra compact: minimal content
		content = w.renderLogoCompact()
		content += "\n" + w.renderSystemInfoCompact()
		content += "\n" + w.renderPressKey()
		contentLines = 3 + 1 + 1 + 1 + 1 // 7
	}

	// If still too tight, remove vertical padding
	if contentLines+boxOverhead > remainingHeight {
		verticalPadding = 0
		boxOverhead = 2
	}

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(styles.Purple).
		Padding(verticalPadding, horizontalPadding).
		Width(boxWidth).
		Align(lipgloss.Center).
		Render(content)

	boxHeight := lipgloss.Height(box)

	// CRITICAL: Don't center if box is taller than available space
	// Instead, align to top to ensure banner is visible
	var centered string
	if boxHeight >= remainingHeight {
		// Box is too tall - align top, let it overflow at bottom rather than cutting top
		centered = lipgloss.Place(
			width, remainingHeight,
			lipgloss.Center, lipgloss.Top,
			box,
		)
	} else {
		// Box fits - center it vertically
		centered = lipgloss.Place(
			width, remainingHeight,
			lipgloss.Center, lipgloss.Center,
			box,
		)
	}

	// Combine warning banner on top
	return warningBanner + centered
}

// =============================================================================
// RENDER HELPERS
// =============================================================================

// renderISWarning renders the government IS warning banner.
// Responsive: uses narrow/compact format based on terminal dimensions.
func (w Welcome) renderISWarning(width, height int) string {
	// Create amber/gold warning style
	bannerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFB000")). // Amber/Gold
		Bold(true)

	var warningText string

	// For 80x24 terminals, we need a compact banner (6 lines max)
	// to leave enough room for the content box
	// Banner height budget: height <= 24 needs compact (6 lines)
	//                       height <= 30 needs narrow (10 lines)
	//                       height > 30 can use full (18 lines)

	if height <= 24 {
		// Compact banner for small terminals - 6 lines total
		// This leaves 24 - 6 = 18 lines for content
		if width >= 78 {
			warningText = `+============================================================================+
|  U.S. GOVERNMENT SYSTEM - Authorized use only. By using this system, you   |
|  consent to monitoring. Unauthorized use may result in penalties.          |
+============================================================================+`
		} else {
			warningText = `+========================================================+
|  U.S. GOVT SYSTEM - Authorized use only.               |
|  Consent to monitoring. Unauthorized use prohibited.   |
+========================================================+`
		}
	} else if height <= 30 {
		// Medium banner for medium terminals
		if width >= 78 {
			warningText = `+============================================================================+
|              U.S. GOVERNMENT INFORMATION SYSTEM                            |
+----------------------------------------------------------------------------+
|  You are accessing a U.S. Government information system. This includes     |
|  this computer, network, and all connected devices and storage media.      |
|  Unauthorized use may result in disciplinary action, civil and criminal    |
|  penalties. By using this system, you consent to monitoring.               |
+============================================================================+`
		} else {
			warningText = `+========================================================+
|        U.S. GOVERNMENT INFORMATION SYSTEM              |
+--------------------------------------------------------+
|  This is a U.S. Government system for authorized       |
|  use only. Unauthorized use may result in civil        |
|  and criminal penalties. By using this system you      |
|  consent to monitoring. No expectation of privacy.     |
+========================================================+`
		}
	} else if width >= 100 {
		// Full-width banner for wide terminals (97 chars)
		warningText = `+=================================================================================================+
|                         U.S. GOVERNMENT INFORMATION SYSTEM                                      |
+-------------------------------------------------------------------------------------------------+
|  You are accessing a U.S. Government information system, which includes:                        |
|  (1) this computer, (2) this computer network, (3) all computers connected to this network,     |
|  and (4) all devices and storage media attached to this network or to a computer on this        |
|  network. This information system is provided for U.S. Government-authorized use only.          |
|                                                                                                 |
|  Unauthorized or improper use of this system may result in disciplinary action, as well as      |
|  civil and criminal penalties.                                                                  |
|                                                                                                 |
|  By using this information system, you understand and consent to the following:                 |
|  - You have no reasonable expectation of privacy regarding any communications or data           |
|    transiting or stored on this information system.                                             |
|  - At any time, the government may monitor, intercept, search and seize any communication       |
|    or data transiting or stored on this information system.                                     |
|  - Any communications or data transiting or stored on this information system may be            |
|    disclosed or used for any U.S. Government-authorized purpose.                                |
+=================================================================================================+`
	} else if width >= 78 {
		// Narrow banner for 80-column terminals (76 chars inner)
		warningText = `+============================================================================+
|              U.S. GOVERNMENT INFORMATION SYSTEM                            |
+----------------------------------------------------------------------------+
|  You are accessing a U.S. Government information system. This includes     |
|  this computer, network, and all connected devices and storage media.      |
|  This system is provided for U.S. Government-authorized use only.          |
|                                                                            |
|  Unauthorized use may result in disciplinary action, civil and criminal    |
|  penalties. By using this system, you consent to monitoring, interception, |
|  search, and seizure of any data. You have no expectation of privacy.      |
+============================================================================+`
	} else {
		// Compact banner for very narrow terminals (fits in 60 chars)
		warningText = `+========================================================+
|        U.S. GOVERNMENT INFORMATION SYSTEM              |
+--------------------------------------------------------+
|  This is a U.S. Government system for authorized       |
|  use only. Unauthorized use may result in civil        |
|  and criminal penalties. By using this system you      |
|  consent to monitoring. No expectation of privacy.     |
+========================================================+`
	}

	// Center the banner in available width
	centeredStyle := bannerStyle.
		Width(width).
		Align(lipgloss.Center)

	return centeredStyle.Render(warningText) + "\n"
}

// renderLogo renders the ASCII art logo (6 lines).
// Responsive: uses compact or simple logo for narrow terminals.
func (w Welcome) renderLogo() string {
	logoStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)

	// Full ASCII art is ~50 chars wide, needs ~54 with box padding
	if w.width >= 60 {
		// Full ASCII art logo - 6 lines using pure ASCII characters
		logo := `  ____  _  ____  ____  _   _ _   _
 |  _ \| |/ ___||  _ \| | | | \ | |
 | |_) | | |  _ | |_) | | | |  \| |
 |  _ <| | |_| ||  _ <| |_| | |\  |
 |_| \_\_|\____||_| \_\\___/|_| \_|
                                   `
		return logoStyle.Render(logo)
	}

	// For narrow terminals, use compact logo
	return w.renderLogoCompact()
}

// renderLogoCompact renders a compact text-based logo (3 lines).
func (w Welcome) renderLogoCompact() string {
	logoStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)

	if w.width >= 40 {
		// Compact box logo for narrow terminals - 3 lines
		// Uses standard ASCII box drawing for maximum compatibility
		return logoStyle.Render(`+--------------------+
|      rigrun        |
+--------------------+`)
	}

	// Simple text logo for very narrow terminals - 1 line
	return logoStyle.Render("rigrun - Local LLM Router")
}

// renderVersion renders the version subtitle.
func (w Welcome) renderVersion() string {
	return lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true).
		Render("Local LLM Router v" + w.version)
}

// renderSystemInfo renders model, GPU, and mode info (3-4 lines).
func (w Welcome) renderSystemInfo() string {
	labelStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary).
		Width(8)

	valueStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)

	lines := []string{
		labelStyle.Render("Model: ") + valueStyle.Render(w.modelName),
		labelStyle.Render("GPU:   ") + valueStyle.Render(w.gpuName),
		labelStyle.Render("Mode:  ") + w.renderModeIndicator(),
	}

	// IL5 SC-7: Add prominent offline mode indicator
	if w.offlineMode {
		offlineBadge := lipgloss.NewStyle().
			Background(lipgloss.Color("#FF0000")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1).
			Render("OFFLINE MODE - SC-7 Boundary Protection")
		lines = append(lines, "")
		lines = append(lines, offlineBadge)
		// Add explanation
		offlineNote := lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Italic(true).
			Render("Network restricted to localhost Ollama only")
		lines = append(lines, offlineNote)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderSystemInfoCompact renders a single-line system info (1 line).
func (w Welcome) renderSystemInfoCompact() string {
	valueStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)

	return valueStyle.Render(w.modelName) + " | " + w.renderModeIndicator()
}

// renderModeIndicator renders the mode with appropriate color.
func (w Welcome) renderModeIndicator() string {
	var modeStyle lipgloss.Style
	mode := w.mode
	displayMode := mode

	// IL5 SC-7: In offline mode, show "local-only" instead of regular mode
	if w.offlineMode {
		modeStyle = lipgloss.NewStyle().Foreground(styles.Rose).Bold(true)
		displayMode = "local-only"
	} else {
		switch mode {
		case "local":
			modeStyle = lipgloss.NewStyle().Foreground(styles.Emerald).Bold(true)
		case "cloud":
			modeStyle = lipgloss.NewStyle().Foreground(styles.Amber).Bold(true)
		case "auto", "hybrid":
			modeStyle = lipgloss.NewStyle().Foreground(styles.Purple).Bold(true)
			displayMode = "auto (OpenRouter)"
		default:
			modeStyle = lipgloss.NewStyle().Foreground(styles.TextSecondary)
		}
	}

	return modeStyle.Render(displayMode)
}

// renderQuickStart renders the quick start tips.
func (w Welcome) renderQuickStart() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary).
		Bold(true)

	bulletStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)

	tipStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary)

	tips := []string{
		bulletStyle.Render("-") + tipStyle.Render(" Type a message and press Enter"),
		bulletStyle.Render("-") + tipStyle.Render(" Use /help to see all commands"),
		bulletStyle.Render("-") + tipStyle.Render(" Use @file:path to include context"),
		bulletStyle.Render("-") + tipStyle.Render(" Press Ctrl+C to stop generation"),
	}

	title := titleStyle.Render("Quick Start:")

	return title + "\n" + lipgloss.JoinVertical(lipgloss.Left, tips...)
}

// renderQuickStartCompact renders a condensed quick start for small terminals.
func (w Welcome) renderQuickStartCompact() string {
	tipStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary)

	return tipStyle.Render("Type /help for commands, Ctrl+C to stop")
}

// renderPressKey renders the "press any key" prompt.
func (w Welcome) renderPressKey() string {
	return lipgloss.NewStyle().
		Foreground(styles.Purple).
		Render("Press any key to continue...")
}

// =============================================================================
// ALTERNATE LOGO STYLES
// =============================================================================

// CompactLogo returns a smaller logo for narrow terminals (3 lines).
func CompactLogo() string {
	return lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true).
		Render(`+--------------------+
|      rigrun        |
+--------------------+`)
}

// SimpleLogo returns a minimal text logo.
func SimpleLogo() string {
	return lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true).
		Render("rigrun - Local LLM Router")
}

// =============================================================================
// KEYBOARD SHORTCUT HELP
// =============================================================================

// KeyboardShortcuts returns a formatted list of keyboard shortcuts.
func KeyboardShortcuts() string {
	keyStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true).
		Width(12)

	descStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary)

	shortcuts := []struct {
		key  string
		desc string
	}{
		{"Enter", "Send message"},
		{"Ctrl+C", "Cancel/Quit"},
		{"Ctrl+L", "Clear screen"},
		{"Ctrl+P", "Command palette"},
		{"Up/Down", "Scroll messages"},
		{"Tab", "Tab completion"},
		{"Esc", "Dismiss/Cancel"},
		{"PgUp/PgDn", "Page scroll"},
	}

	lines := make([]string, len(shortcuts))
	for i, s := range shortcuts {
		lines[i] = keyStyle.Render(s.key) + descStyle.Render(s.desc)
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(styles.TextPrimary).
		Bold(true)

	return titleStyle.Render("Keyboard Shortcuts") + "\n" +
		lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// =============================================================================
// WELCOME OVERLAY
// =============================================================================

// WelcomeOverlay creates a centered welcome overlay for use over other content.
func WelcomeOverlay(width, height int, version string) string {
	w := NewWelcome(nil)
	w.SetVersion(version)
	w.SetSize(width, height)

	// Create a semi-transparent background effect
	overlay := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(w.View())

	return overlay
}
