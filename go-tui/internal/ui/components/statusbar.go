// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides the visual UI components for rigrun TUI.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/model"
	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// TierIcons maps router tiers to display icons for status bar
var TierIcons = map[router.Tier]string{
	router.TierCache:  "z",  // Lightning for cache (fast)
	router.TierLocal:  "@",  // Home for local
	router.TierAuto:   "A",  // Auto for OpenRouter auto-routing
	router.TierCloud:  "*",  // Cloud
	router.TierHaiku:  "#",  // Bamboo for Haiku
	router.TierSonnet: "~",  // Scroll for Sonnet
	router.TierOpus:   "&",  // Theater for Opus
	router.TierGpt4o:  "$",  // Dollar for GPT-4o
}

// =============================================================================
// STATUS BAR COMPONENT - Beautiful bottom status bar
// =============================================================================

// Status represents the current application status
type Status int

const (
	StatusReady Status = iota
	StatusStreaming
	StatusThinking
	StatusLoading
	StatusError
	StatusIdle
)

// String returns the display string for the status
func (s Status) String() string {
	switch s {
	case StatusReady:
		return "Ready"
	case StatusStreaming:
		return "Streaming..."
	case StatusThinking:
		return "Thinking..."
	case StatusLoading:
		return "Loading..."
	case StatusError:
		return "Error"
	case StatusIdle:
		return "Idle"
	default:
		return "Unknown"
	}
}

// Icon returns an icon for the status
// ACCESSIBILITY: Uses distinct shapes alongside colors for colorblind users
func (s Status) Icon() string {
	switch s {
	case StatusReady:
		return styles.StatusIndicators.Success // Checkmark for ready
	case StatusStreaming:
		return "~"
	case StatusThinking:
		return styles.StatusIndicators.Pending // Empty circle for pending
	case StatusLoading:
		return styles.StatusIndicators.Pending // Empty circle for loading
	case StatusError:
		return styles.StatusIndicators.Error // X mark for error
	case StatusIdle:
		return "-"
	default:
		return "?"
	}
}

// StatusBar represents the beautiful bottom status bar
type StatusBar struct {
	Mode          Mode   // LOCAL/CLOUD/AUTO
	ModelName     string // Current model
	GPUStatus     string // GPU info (e.g., "RTX 4090" or "")
	GPUActive     bool   // Whether GPU is active
	TokenCount    int    // Tokens used in current context
	MaxTokens     int    // Maximum context tokens
	Status        Status // Current status
	Width         int    // Available width
	ShowShortcuts bool   // Show keyboard shortcuts
	theme         *styles.Theme

	// Router integration
	RoutingMode    string               // "cloud", "local", "hybrid"
	CurrentTier    router.Tier          // Current routing tier being used
	SessionStats   *router.SessionStats // Session statistics for cost tracking
	TotalCostCents float64              // Total cost in cents
	TotalSaved     float64              // Total savings vs Opus in cents

	// IL5 SC-7: Offline mode indicator
	OfflineMode bool // True when --no-network flag is active
}

// NewStatusBar creates a new StatusBar component
func NewStatusBar(theme *styles.Theme) *StatusBar {
	return &StatusBar{
		Mode:           ModeAuto,
		ModelName:      "",
		GPUStatus:      "",
		GPUActive:      false,
		TokenCount:     0,
		MaxTokens:      4096,
		Status:         StatusReady,
		Width:          80,
		ShowShortcuts:  true,
		theme:          theme,
		RoutingMode:    "auto",
		CurrentTier:    router.TierAuto,
		SessionStats:   nil,
		TotalCostCents: 0.0,
		TotalSaved:     0.0,
	}
}

// SetWidth updates the status bar width
func (s *StatusBar) SetWidth(width int) {
	s.Width = width
}

// SetTokenUsage updates the token count display
func (s *StatusBar) SetTokenUsage(used, max int) {
	s.TokenCount = used
	s.MaxTokens = max
}

// SetGPU updates the GPU status
func (s *StatusBar) SetGPU(name string, active bool) {
	s.GPUStatus = name
	s.GPUActive = active
}

// SetStatus updates the current status
func (s *StatusBar) SetStatus(status Status) {
	s.Status = status
}

// SetRoutingMode updates the routing mode display
func (s *StatusBar) SetRoutingMode(mode string) {
	s.RoutingMode = mode
}

// SetCurrentTier updates the current tier being used
func (s *StatusBar) SetCurrentTier(tier router.Tier) {
	s.CurrentTier = tier
}

// SetSessionStats sets the session statistics for cost tracking
func (s *StatusBar) SetSessionStats(stats *router.SessionStats) {
	s.SessionStats = stats
	if stats != nil {
		snapshot := stats.GetStats()
		s.TotalCostCents = snapshot.TotalCostCents
		s.TotalSaved = snapshot.TotalSavedCents
	}
}

// UpdateFromSessionStats refreshes cost/savings from session stats
func (s *StatusBar) UpdateFromSessionStats() {
	if s.SessionStats != nil {
		snapshot := s.SessionStats.GetStats()
		s.TotalCostCents = snapshot.TotalCostCents
		s.TotalSaved = snapshot.TotalSavedCents
	}
}

// SetOfflineMode updates the offline mode state (IL5 SC-7)
func (s *StatusBar) SetOfflineMode(offline bool) {
	s.OfflineMode = offline
}

// SetModel updates the model name with optional tier lookup.
// If the model is found in the registry, displays the friendly name with tier icon.
func (s *StatusBar) SetModel(modelName string) {
	if info, ok := model.GetModelInfo(modelName); ok {
		// Use display name with tier icon
		s.ModelName = fmt.Sprintf("%s %s", info.TierIcon(), info.Name)
		// Update max tokens from model info
		if info.MaxTokens > 0 {
			s.MaxTokens = info.MaxTokens
		}
	} else {
		// Unknown model - display as-is
		s.ModelName = modelName
	}
}

// SetModelWithTier updates the model name with explicit tier display.
// Displays: "icon ModelName [Tier]"
func (s *StatusBar) SetModelWithTier(modelName string) {
	if info, ok := model.GetModelInfo(modelName); ok {
		s.ModelName = fmt.Sprintf("%s %s [%s]", info.TierIcon(), info.Name, info.Tier)
		if info.MaxTokens > 0 {
			s.MaxTokens = info.MaxTokens
		}
	} else {
		s.ModelName = modelName
	}
}

// GetModelInfo returns information about the current model if available.
// Returns nil if the model is not in the registry.
func (s *StatusBar) GetModelInfo() *model.ModelInfo {
	if info, ok := model.GetModelInfo(s.ModelName); ok {
		return &info
	}
	return nil
}

// View renders the status bar
func (s *StatusBar) View() string {
	// Choose layout based on width
	if s.Width < 60 {
		return s.viewNarrow()
	}
	if s.Width < 100 {
		return s.viewMedium()
	}
	return s.viewWide()
}

// viewNarrow renders a compact status bar for narrow terminals
// Format: [MODE|GPU|OFF] ContextBar Status
func (s *StatusBar) viewNarrow() string {
	parts := []string{}

	// IL5 SC-7: Offline indicator takes precedence
	if s.OfflineMode {
		offlineStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("#FF0000")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)
		parts = append(parts, offlineStyle.Render("OFF"))
	} else {
		// Mode indicator (compact)
		modeStyle := s.getModeStyle()
		modeChar := string([]rune(s.Mode.String())[0]) // First letter only
		parts = append(parts, modeStyle.Render(modeChar))
	}

	// ACCESSIBILITY: GPU indicator with high contrast and shape indicator
	if s.GPUActive {
		gpuStyle := lipgloss.NewStyle().Foreground(styles.SuccessHighContrast).Bold(true)
		parts = append(parts, gpuStyle.Render(styles.StatusIndicators.Success))
	}

	// Combine mode section
	modeSection := "[" + strings.Join(parts, "|") + "]"

	// Context bar (smaller)
	contextBar := s.renderContextBarSmall()

	// Status
	statusStyle := s.getStatusStyle()
	statusText := statusStyle.Render(s.Status.Icon())

	// Join with spaces
	separator := lipgloss.NewStyle().Foreground(styles.Overlay).Render(" ")

	result := modeSection + separator + contextBar + separator + statusText

	// Apply background
	return lipgloss.NewStyle().
		Background(styles.SurfaceDim).
		Foreground(styles.TextSecondary).
		Width(s.Width).
		Render(result)
}

// viewMedium renders a medium-width status bar
// Format: MODE | model | GPU | Context: ContextBar | Status | [OFFLINE]
func (s *StatusBar) viewMedium() string {
	separator := lipgloss.NewStyle().
		Foreground(styles.Overlay).
		Render(" | ")

	parts := []string{}

	// IL5 SC-7: Offline badge (shown first when active)
	if s.OfflineMode {
		offlineBadge := lipgloss.NewStyle().
			Background(lipgloss.Color("#FF0000")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Render(" OFFLINE ")
		parts = append(parts, offlineBadge)
	}

	// Mode (show "local-only" in offline mode)
	// ACCESSIBILITY: Uses high contrast colors for colorblind users
	modeStyle := s.getModeStyle()
	modeText := s.Mode.String()
	if s.OfflineMode {
		modeText = "local-only"
		modeStyle = lipgloss.NewStyle().Foreground(styles.ErrorHighContrast).Bold(true)
	}
	parts = append(parts, modeStyle.Render(modeText))

	// Model (truncated if needed)
	if s.ModelName != "" {
		modelName := s.ModelName
		// Use rune-based truncation to handle Unicode correctly
		modelRunes := []rune(modelName)
		if len(modelRunes) > 15 {
			modelName = string(modelRunes[:12]) + "..."
		}
		modelStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)
		parts = append(parts, modelStyle.Render(modelName))
	}

	// GPU
	if s.GPUStatus != "" {
		gpuStyle := s.getGPUStyle()
		gpuIcon := s.getGPUIcon()
		parts = append(parts, gpuStyle.Render(gpuIcon+" "+s.GPUStatus))
	}

	// Context bar with label
	contextLabel := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Render("Ctx:")
	contextBar := s.renderContextBar()
	parts = append(parts, contextLabel+" "+contextBar)

	// Status
	statusStyle := s.getStatusStyle()
	parts = append(parts, statusStyle.Render(s.Status.String()))

	result := strings.Join(parts, separator)

	// Apply background
	return lipgloss.NewStyle().
		Background(styles.SurfaceDim).
		Foreground(styles.TextSecondary).
		Padding(0, 1).
		Width(s.Width).
		Render(result)
}

// viewWide renders a full-featured status bar for wide terminals
// Format: [OFFLINE] qwen2.5-coder:14b | local-only | Cloud: disabled | 1,234 tok
func (s *StatusBar) viewWide() string {
	// Left section: Model, Routing Mode, Tier
	leftParts := []string{}

	// IL5 SC-7: Offline badge (shown first, prominent)
	if s.OfflineMode {
		offlineBadge := lipgloss.NewStyle().
			Background(lipgloss.Color("#FF0000")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1).
			Render("OFFLINE")
		leftParts = append(leftParts, offlineBadge)
	}

	// Model name
	if s.ModelName != "" {
		modelStyle := lipgloss.NewStyle().Foreground(styles.TextSecondary)
		leftParts = append(leftParts, modelStyle.Render(s.ModelName))
	}

	// Routing mode with tier icon
	// In offline mode, show "local-only" and "Cloud: disabled"
	routingMode := s.RoutingMode
	if routingMode == "" {
		routingMode = "auto"
	}
	if s.OfflineMode {
		routingMode = "local-only"
	}
	tierIcon, ok := TierIcons[s.CurrentTier]
	if !ok {
		tierIcon = "?"
	}
	// ACCESSIBILITY: Uses high contrast colors for colorblind users
	modeStyle := s.getRoutingModeStyle(routingMode)
	if s.OfflineMode {
		modeStyle = lipgloss.NewStyle().Foreground(styles.ErrorHighContrast).Bold(true)
		tierIcon = "@" // Local icon
	}
	modeBadge := modeStyle.Render(tierIcon + " " + strings.ToUpper(routingMode))
	leftParts = append(leftParts, modeBadge)

	// Cloud status indicator (disabled in offline mode)
	if s.OfflineMode {
		cloudDisabled := lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Italic(true).
			Render("Cloud: disabled")
		leftParts = append(leftParts, cloudDisabled)
	}

	// Token count
	tokenStr := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Render(fmt.Sprintf("%s tok", formatNumber(s.TokenCount)))
	leftParts = append(leftParts, tokenStr)

	// Session cost (if any)
	if s.TotalCostCents > 0 {
		costStr := lipgloss.NewStyle().
			Foreground(styles.Amber).
			Render(s.formatCost(s.TotalCostCents))
		leftParts = append(leftParts, costStr)
	}

	// ACCESSIBILITY: Savings display (if any) with high contrast green
	if s.TotalSaved > 0 {
		savedStr := lipgloss.NewStyle().
			Foreground(styles.SuccessHighContrast).
			Bold(true).
			Render(styles.StatusIndicators.Success + " Saved: " + s.formatCost(s.TotalSaved))
		leftParts = append(leftParts, savedStr)
	}

	leftSep := lipgloss.NewStyle().Foreground(styles.Overlay).Render(" | ")
	leftSection := strings.Join(leftParts, leftSep)

	// Center section: Context bar with tier-based coloring
	contextLabel := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Render("Ctx: ")
	contextBar := s.renderContextBarWithTier()
	contextPercent := s.renderContextPercent()
	centerSection := contextLabel + contextBar + " " + contextPercent

	// Right section: Status and shortcuts
	rightParts := []string{}

	// Status
	statusStyle := s.getStatusStyle()
	rightParts = append(rightParts, statusStyle.Render(s.Status.String()))

	// Keyboard shortcuts (if enabled) - updated to include Ctrl+R
	if s.ShowShortcuts {
		shortcuts := s.renderShortcutsWithRouting()
		rightParts = append(rightParts, shortcuts)
	}

	rightSection := strings.Join(rightParts, " ")

	// Calculate spacing
	leftWidth := lipgloss.Width(leftSection)
	centerWidth := lipgloss.Width(centerSection)
	rightWidth := lipgloss.Width(rightSection)
	totalContent := leftWidth + centerWidth + rightWidth

	// Add spacing between sections
	spacing := s.Width - totalContent - 4 // Account for padding
	if spacing < 4 {
		spacing = 4
	}

	leftSpace := strings.Repeat(" ", spacing/2)
	rightSpace := strings.Repeat(" ", spacing-spacing/2)

	result := leftSection + leftSpace + centerSection + rightSpace + rightSection

	// Apply styled border for wide view
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(styles.Overlay).
		Background(styles.SurfaceDim).
		Foreground(styles.TextSecondary).
		Padding(0, 1).
		Width(s.Width).
		Render(result)
}

// ==========================================================================
// HELPER RENDER METHODS
// ==========================================================================

// renderModeBadge renders a styled mode badge
func (s *StatusBar) renderModeBadge() string {
	modeStyle := s.getModeStyle().
		Padding(0, 1)

	return modeStyle.Render(s.Mode.String())
}

// renderGPUBadge renders a styled GPU badge
func (s *StatusBar) renderGPUBadge() string {
	gpuStyle := s.getGPUStyle()
	icon := s.getGPUIcon()

	return gpuStyle.Render(icon + " " + s.GPUStatus)
}

// renderContextBar renders the context usage bar
// Format: [##########] (10 blocks)
func (s *StatusBar) renderContextBar() string {
	percent := 0.0
	if s.MaxTokens > 0 {
		percent = float64(s.TokenCount) / float64(s.MaxTokens) * 100
	}

	filled := int(percent / 10)
	if filled > 10 {
		filled = 10
	}
	empty := 10 - filled

	// Choose color based on percentage
	barColor := styles.Cyan
	if percent >= 90 {
		barColor = styles.Rose
	} else if percent >= 75 {
		barColor = styles.Amber
	} else if percent >= 50 {
		barColor = styles.Emerald
	}

	filledStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyStyle := lipgloss.NewStyle().Foreground(styles.Overlay)

	filledPart := filledStyle.Render(strings.Repeat("#", filled))
	emptyPart := emptyStyle.Render(strings.Repeat("-", empty))

	return "[" + filledPart + emptyPart + "]"
}

// renderContextBarSmall renders a smaller context bar for narrow view
// Format: [####--] (6 blocks)
func (s *StatusBar) renderContextBarSmall() string {
	percent := 0.0
	if s.MaxTokens > 0 {
		percent = float64(s.TokenCount) / float64(s.MaxTokens) * 100
	}

	filled := int(percent / 100 * 6)
	if filled > 6 {
		filled = 6
	}
	empty := 6 - filled

	// Choose color based on percentage
	barColor := styles.Cyan
	if percent >= 90 {
		barColor = styles.Rose
	} else if percent >= 75 {
		barColor = styles.Amber
	}

	filledStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyStyle := lipgloss.NewStyle().Foreground(styles.Overlay)

	return filledStyle.Render(strings.Repeat("#", filled)) +
		emptyStyle.Render(strings.Repeat("-", empty))
}

// renderContextPercent renders the context percentage with token counts
func (s *StatusBar) renderContextPercent() string {
	percent := 0.0
	if s.MaxTokens > 0 {
		percent = float64(s.TokenCount) / float64(s.MaxTokens) * 100
	}

	// Choose color based on percentage
	color := styles.TextMuted
	if percent >= 90 {
		color = styles.Rose
	} else if percent >= 75 {
		color = styles.Amber
	}

	percentStyle := lipgloss.NewStyle().Foreground(color)

	// Format: 2,048/4,096 (50%)
	return percentStyle.Render(
		formatNumber(s.TokenCount) + "/" + formatNumber(s.MaxTokens) +
			" (" + formatPercent(percent) + ")",
	)
}

// renderShortcuts renders keyboard shortcut hints
func (s *StatusBar) renderShortcuts() string {
	keyStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted)

	shortcuts := []string{
		keyStyle.Render("^P") + descStyle.Render("cmds"),
		keyStyle.Render("^C") + descStyle.Render("stop"),
	}

	return strings.Join(shortcuts, " ")
}

// getModeStyle returns the style for the current mode
func (s *StatusBar) getModeStyle() lipgloss.Style {
	switch s.Mode {
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

// getStatusStyle returns the style for the current status
// ACCESSIBILITY: Uses high contrast colors with bold for colorblind users
func (s *StatusBar) getStatusStyle() lipgloss.Style {
	switch s.Status {
	case StatusReady:
		// ACCESSIBILITY: High contrast green with bold
		return lipgloss.NewStyle().Foreground(styles.SuccessHighContrast).Bold(true)
	case StatusStreaming, StatusThinking:
		// ACCESSIBILITY: High contrast blue with bold
		return lipgloss.NewStyle().Foreground(styles.InfoHighContrast).Bold(true)
	case StatusLoading:
		// ACCESSIBILITY: High contrast amber with bold
		return lipgloss.NewStyle().Foreground(styles.WarningHighContrast).Bold(true)
	case StatusError:
		// ACCESSIBILITY: High contrast red with bold
		return lipgloss.NewStyle().Foreground(styles.ErrorHighContrast).Bold(true)
	case StatusIdle:
		return lipgloss.NewStyle().Foreground(styles.TextMuted)
	default:
		return lipgloss.NewStyle().Foreground(styles.TextMuted)
	}
}

// getGPUStyle returns the style for GPU status
// ACCESSIBILITY: Uses high contrast colors with bold for colorblind users
func (s *StatusBar) getGPUStyle() lipgloss.Style {
	if s.GPUActive {
		return lipgloss.NewStyle().Foreground(styles.SuccessHighContrast).Bold(true)
	}
	return lipgloss.NewStyle().Foreground(styles.TextMuted)
}

// getGPUIcon returns an icon for GPU status
// ACCESSIBILITY: Uses distinct shapes alongside colors for colorblind users
func (s *StatusBar) getGPUIcon() string {
	if s.GPUActive {
		return styles.StatusIndicators.Success // Checkmark for active
	}
	return styles.StatusIndicators.Pending // Empty circle for inactive
}

// ==========================================================================
// HELPER FUNCTIONS (using shared helpers from helpers.go)
// ==========================================================================

// formatNumber formats a number with thousand separators
func formatNumber(n int) string {
	return fmtNumber(n)
}

// formatPercent formats a percentage with one decimal place
func formatPercent(p float64) string {
	return fmtPercent(p)
}

// ==========================================================================
// ROUTING-SPECIFIC METHODS
// ==========================================================================

// getRoutingModeStyle returns the style for a routing mode string
// ACCESSIBILITY: Uses high contrast colors for colorblind users
func (s *StatusBar) getRoutingModeStyle(mode string) lipgloss.Style {
	switch mode {
	case "local":
		return lipgloss.NewStyle().
			Foreground(styles.SuccessHighContrast).
			Bold(true)
	case "cloud":
		return lipgloss.NewStyle().
			Foreground(styles.WarningHighContrast).
			Bold(true)
	case "auto", "hybrid":
		return lipgloss.NewStyle().
			Foreground(styles.Purple).
			Bold(true)
	default:
		return lipgloss.NewStyle().
			Foreground(styles.TextMuted)
	}
}

// getTierStyle returns the style for a specific routing tier
// ACCESSIBILITY: Uses high contrast colors for colorblind users
func (s *StatusBar) getTierStyle(tier router.Tier) lipgloss.Style {
	switch tier {
	case router.TierCache:
		return lipgloss.NewStyle().Foreground(styles.InfoHighContrast).Bold(true)
	case router.TierLocal:
		return lipgloss.NewStyle().Foreground(styles.SuccessHighContrast).Bold(true)
	case router.TierAuto:
		return lipgloss.NewStyle().Foreground(styles.Purple).Bold(true)
	case router.TierCloud, router.TierHaiku:
		return lipgloss.NewStyle().Foreground(styles.WarningHighContrast).Bold(true)
	case router.TierSonnet:
		return lipgloss.NewStyle().Foreground(styles.Purple).Bold(true)
	case router.TierOpus:
		return lipgloss.NewStyle().Foreground(styles.ErrorHighContrast).Bold(true)
	case router.TierGpt4o:
		return lipgloss.NewStyle().Foreground(styles.InfoHighContrast).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(styles.TextMuted)
	}
}

// renderContextBarWithTier renders context bar with color based on current tier
// ACCESSIBILITY: Uses high contrast colors for colorblind users
func (s *StatusBar) renderContextBarWithTier() string {
	percent := 0.0
	if s.MaxTokens > 0 {
		percent = float64(s.TokenCount) / float64(s.MaxTokens) * 100
	}

	filled := int(percent / 10)
	if filled > 10 {
		filled = 10
	}
	empty := 10 - filled

	// Choose color based on current tier with high contrast for accessibility
	barColor := styles.InfoHighContrast
	switch s.CurrentTier {
	case router.TierCache:
		barColor = styles.InfoHighContrast
	case router.TierLocal:
		barColor = styles.SuccessHighContrast
	case router.TierAuto:
		barColor = styles.Purple
	case router.TierCloud, router.TierHaiku:
		barColor = styles.WarningHighContrast
	case router.TierSonnet:
		barColor = styles.Purple
	case router.TierOpus, router.TierGpt4o:
		barColor = styles.ErrorHighContrast
	}

	// ACCESSIBILITY: Override with warning colors if context is getting full
	if percent >= 90 {
		barColor = styles.ErrorHighContrast
	} else if percent >= 75 {
		barColor = styles.WarningHighContrast
	}

	filledStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyStyle := lipgloss.NewStyle().Foreground(styles.Overlay)

	filledPart := filledStyle.Render(strings.Repeat("#", filled))
	emptyPart := emptyStyle.Render(strings.Repeat("-", empty))

	return "[" + filledPart + emptyPart + "]"
}

// renderShortcutsWithRouting renders shortcuts including Ctrl+R for routing
func (s *StatusBar) renderShortcutsWithRouting() string {
	keyStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted)

	shortcuts := []string{
		keyStyle.Render("^R") + descStyle.Render("mode"),
		keyStyle.Render("^P") + descStyle.Render("cmds"),
		keyStyle.Render("^C") + descStyle.Render("stop"),
	}

	return strings.Join(shortcuts, " ")
}

// formatCost formats a cost in cents for display
// Returns format like "0.05c" for cents or "$1.23" for dollars
func (s *StatusBar) formatCost(cents float64) string {
	if cents < 1.0 {
		return fmt.Sprintf("%.2fc", cents)
	} else if cents < 100.0 {
		return fmt.Sprintf("%.1fc", cents)
	}
	// Convert to dollars for larger amounts
	return fmt.Sprintf("$%.2f", cents/100.0)
}

// GetTierIcon returns the icon for a given tier
func GetTierIcon(tier router.Tier) string {
	if icon, ok := TierIcons[tier]; ok {
		return icon
	}
	return "?"
}
