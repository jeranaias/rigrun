// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides the visual UI components for rigrun TUI.
// Each component is designed to be STUNNING and professionally polished.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// HEADER COMPONENT - Gorgeous title bar with rigrun branding
// =============================================================================

// Mode represents the routing mode (LOCAL/CLOUD/AUTO)
type Mode int

const (
	ModeLocal Mode = iota
	ModeCloud
	ModeAuto
)

// String returns the display string for the mode
func (m Mode) String() string {
	switch m {
	case ModeLocal:
		return "LOCAL"
	case ModeCloud:
		return "CLOUD"
	case ModeAuto:
		return "AUTO"
	default:
		return "UNKNOWN"
	}
}

// Header represents the gorgeous title bar component
type Header struct {
	Title       string // Main title (default: "rigrun")
	ModelName   string // Current model name
	Mode        Mode   // Routing mode
	Width       int    // Available width
	OfflineMode bool   // IL5 SC-7: True when --no-network flag is active
	theme       *styles.Theme
}

// NewHeader creates a new Header component with default values
func NewHeader(theme *styles.Theme) *Header {
	return &Header{
		Title:     "rigrun",
		ModelName: "",
		Mode:      ModeLocal,
		Width:     80,
		theme:     theme,
	}
}

// SetWidth updates the header width
func (h *Header) SetWidth(width int) {
	h.Width = width
}

// SetModel updates the current model name
func (h *Header) SetModel(model string) {
	h.ModelName = model
}

// SetMode updates the routing mode
func (h *Header) SetMode(mode Mode) {
	h.Mode = mode
}

// SetOfflineMode updates the offline mode state (IL5 SC-7)
func (h *Header) SetOfflineMode(offline bool) {
	h.OfflineMode = offline
}

// View renders the header component with beautiful styling
func (h *Header) View() string {
	// Ensure minimum width
	width := h.Width
	if width < 40 {
		width = 40
	}

	// Calculate inner width (accounting for borders and padding)
	innerWidth := width - 6

	// ==========================================================================
	// BUILD THE GORGEOUS HEADER
	// ==========================================================================

	// Brand title with elegant styling
	brandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Cyan)

	// Decorative accent
	accentStyle := lipgloss.NewStyle().
		Foreground(styles.Purple)

	// Create the brand with decorative elements
	brand := accentStyle.Render("< ") +
		brandStyle.Render(h.Title) +
		accentStyle.Render(" >")

	// Subtitle line with model and mode
	subtitleParts := []string{}

	if h.ModelName != "" {
		modelStyle := lipgloss.NewStyle().
			Foreground(styles.TextSecondary)
		subtitleParts = append(subtitleParts, modelStyle.Render(h.ModelName))
	}

	// Mode indicator with color coding
	modeStyle := h.getModeStyle()
	modeIndicator := modeStyle.Render("[" + h.Mode.String() + "]")
	subtitleParts = append(subtitleParts, modeIndicator)

	// IL5 SC-7: Offline mode badge (prominent red background)
	if h.OfflineMode {
		offlineBadge := lipgloss.NewStyle().
			Background(lipgloss.Color("#FF0000")). // Red background
			Foreground(lipgloss.Color("#FFFFFF")). // White text
			Bold(true).
			Padding(0, 1).
			Render("OFFLINE")
		subtitleParts = append(subtitleParts, offlineBadge)
	}

	subtitle := strings.Join(subtitleParts, " ")

	// Center the content
	brandLine := lipgloss.NewStyle().
		Width(innerWidth).
		Align(lipgloss.Center).
		Render(brand)

	subtitleLine := lipgloss.NewStyle().
		Width(innerWidth).
		Align(lipgloss.Center).
		Foreground(styles.TextMuted).
		Render(subtitle)

	// Combine lines
	content := lipgloss.JoinVertical(lipgloss.Center, brandLine, subtitleLine)

	// Apply the gorgeous border and styling
	headerBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Purple).
		Background(styles.SurfaceDim).
		Padding(0, 2).
		Width(width)

	return headerBox.Render(content)
}

// ViewCompact renders a compact single-line header for narrow terminals
func (h *Header) ViewCompact() string {
	// Compact format: < rigrun > | model | [MODE] | [OFFLINE]
	brandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Cyan)

	accentStyle := lipgloss.NewStyle().
		Foreground(styles.Purple)

	brand := accentStyle.Render("<") +
		brandStyle.Render(h.Title) +
		accentStyle.Render(">")

	parts := []string{brand}

	if h.ModelName != "" {
		modelStyle := lipgloss.NewStyle().
			Foreground(styles.TextMuted)
		parts = append(parts, modelStyle.Render(h.ModelName))
	}

	modeStyle := h.getModeStyle()
	parts = append(parts, modeStyle.Render("["+h.Mode.String()+"]"))

	// IL5 SC-7: Offline mode badge
	if h.OfflineMode {
		offlineBadge := lipgloss.NewStyle().
			Background(lipgloss.Color("#FF0000")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Render("[OFFLINE]")
		parts = append(parts, offlineBadge)
	}

	separator := lipgloss.NewStyle().
		Foreground(styles.Overlay).
		Render(" | ")

	return strings.Join(parts, separator)
}

// ViewFancy renders an extra fancy header with ASCII art flourishes
func (h *Header) ViewFancy() string {
	width := h.Width
	if width < 60 {
		return h.View()
	}

	innerWidth := width - 6

	// Top decorative line
	topDeco := h.createDecorativeLine(innerWidth)

	// Brand with sparkles
	brandStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Cyan)

	sparkleStyle := lipgloss.NewStyle().
		Foreground(styles.Purple)

	brand := sparkleStyle.Render("* ") +
		brandStyle.Render(h.Title) +
		sparkleStyle.Render(" *")

	// Tagline
	taglineStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)
	tagline := taglineStyle.Render("Local LLM Router")

	// Model info
	modelLine := ""
	if h.ModelName != "" {
		modelStyle := lipgloss.NewStyle().
			Foreground(styles.TextSecondary)
		modelLine = modelStyle.Render("Model: " + h.ModelName)
	}

	// Mode badge
	modeStyle := h.getModeStyle()
	modeBadge := modeStyle.Render(" " + h.Mode.String() + " ")

	// IL5 SC-7: Offline mode badge (fancy version)
	var offlineBadge string
	if h.OfflineMode {
		offlineBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#FF0000")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1).
			Render("OFFLINE MODE - SC-7 Boundary Protection")
	}

	// Bottom decorative line
	bottomDeco := h.createDecorativeLine(innerWidth)

	// Center everything
	centerStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Align(lipgloss.Center)

	lines := []string{
		centerStyle.Render(topDeco),
		centerStyle.Render(brand),
		centerStyle.Render(tagline),
	}

	if modelLine != "" {
		lines = append(lines, centerStyle.Render(modelLine))
	}

	lines = append(lines, centerStyle.Render(modeBadge))

	// IL5 SC-7: Add offline badge if active
	if offlineBadge != "" {
		lines = append(lines, centerStyle.Render(offlineBadge))
	}

	lines = append(lines, centerStyle.Render(bottomDeco))

	content := lipgloss.JoinVertical(lipgloss.Center, lines...)

	// Apply border
	headerBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(styles.Purple).
		Background(styles.SurfaceDim).
		Padding(0, 2).
		Width(width)

	return headerBox.Render(content)
}

// getModeStyle returns the appropriate style for the current mode
func (h *Header) getModeStyle() lipgloss.Style {
	switch h.Mode {
	case ModeLocal:
		return lipgloss.NewStyle().
			Foreground(styles.Emerald).
			Bold(true)
	case ModeCloud:
		return lipgloss.NewStyle().
			Foreground(styles.Amber).
			Bold(true)
	case ModeAuto:
		return lipgloss.NewStyle().
			Foreground(styles.Purple).
			Bold(true)
	default:
		return lipgloss.NewStyle().
			Foreground(styles.TextMuted)
	}
}

// createDecorativeLine creates a decorative line with gradual fade
func (h *Header) createDecorativeLine(width int) string {
	if width < 10 {
		return ""
	}

	// Create a nice decorative pattern
	// Format: -----< * >-----
	sideLen := (width - 7) / 2
	if sideLen < 3 {
		sideLen = 3
	}

	lineStyle := lipgloss.NewStyle().
		Foreground(styles.Overlay)

	accentStyle := lipgloss.NewStyle().
		Foreground(styles.Purple)

	side := strings.Repeat("-", sideLen)

	return lineStyle.Render(side) +
		accentStyle.Render("< * >") +
		lineStyle.Render(side)
}

// =============================================================================
// GRADIENT TITLE (for terminals with true color support)
// =============================================================================

// GradientTitle creates a beautiful gradient text effect
// Note: This works best in terminals with true color support
func GradientTitle(text string, startColor, endColor lipgloss.Color) string {
	if len(text) == 0 {
		return ""
	}

	// For short text, just use the start color
	if len(text) < 3 {
		return lipgloss.NewStyle().Foreground(startColor).Render(text)
	}

	// Build gradient character by character
	var result strings.Builder
	chars := []rune(text)
	n := len(chars)

	for i, char := range chars {
		// Calculate interpolation factor
		t := float64(i) / float64(n-1)

		// Interpolate colors (simplified - works for hex colors)
		color := interpolateColor(startColor, endColor, t)

		style := lipgloss.NewStyle().Foreground(color)
		result.WriteString(style.Render(string(char)))
	}

	return result.String()
}

// interpolateColor interpolates between two colors
// This is a simplified version that works for the gradient effect
func interpolateColor(start, end lipgloss.Color, t float64) lipgloss.Color {
	// Extract RGB values from hex colors
	startHex := string(start)
	endHex := string(end)

	// Handle # prefix
	if len(startHex) > 0 && startHex[0] == '#' {
		startHex = startHex[1:]
	}
	if len(endHex) > 0 && endHex[0] == '#' {
		endHex = endHex[1:]
	}

	// Parse hex colors (default to white if parsing fails)
	sr, sg, sb := parseHexColor(startHex)
	er, eg, eb := parseHexColor(endHex)

	// Interpolate each channel
	r := uint8(float64(sr) + t*(float64(er)-float64(sr)))
	g := uint8(float64(sg) + t*(float64(eg)-float64(sg)))
	b := uint8(float64(sb) + t*(float64(eb)-float64(sb)))

	// Format as hex color
	return lipgloss.Color(formatHexColor(r, g, b))
}

// parseHexColor parses a hex color string into RGB components
func parseHexColor(hex string) (r, g, b uint8) {
	if len(hex) < 6 {
		return 255, 255, 255 // Default to white
	}

	// Parse each component
	r = parseHexByte(hex[0:2])
	g = parseHexByte(hex[2:4])
	b = parseHexByte(hex[4:6])
	return
}

// parseHexByte parses a two-character hex string into a byte
func parseHexByte(s string) uint8 {
	if len(s) != 2 {
		return 255
	}

	var result uint8
	for _, c := range s {
		result *= 16
		switch {
		case c >= '0' && c <= '9':
			result += uint8(c - '0')
		case c >= 'a' && c <= 'f':
			result += uint8(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			result += uint8(c - 'A' + 10)
		default:
			return 255
		}
	}
	return result
}

// formatHexColor formats RGB values as a hex color string
func formatHexColor(r, g, b uint8) string {
	const hexChars = "0123456789ABCDEF"
	return "#" +
		string(hexChars[r>>4]) + string(hexChars[r&0xF]) +
		string(hexChars[g>>4]) + string(hexChars[g&0xF]) +
		string(hexChars[b>>4]) + string(hexChars[b&0xF])
}
