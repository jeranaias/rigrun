// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// SESSION TIMEOUT OVERLAY - IL5 AC-12 COMPLIANCE
// =============================================================================

// SessionTimeoutOverlay displays a warning when the session is about to expire.
// This implements DoD STIG AC-12 (Session Termination) requirements.
type SessionTimeoutOverlay struct {
	// State
	visible       bool
	timeRemaining time.Duration
	expired       bool

	// Configuration
	warningThreshold time.Duration // Default: 2 minutes

	// Dimensions
	width  int
	height int
}

// NewSessionTimeoutOverlay creates a new session timeout overlay.
func NewSessionTimeoutOverlay() SessionTimeoutOverlay {
	return SessionTimeoutOverlay{
		visible:          false,
		warningThreshold: 2 * time.Minute,
	}
}

// =============================================================================
// CONFIGURATION
// =============================================================================

// SetSize sets the overlay dimensions.
func (o *SessionTimeoutOverlay) SetSize(width, height int) {
	o.width = width
	o.height = height
}

// SetWarningThreshold sets when to show the warning (default: 2 minutes).
func (o *SessionTimeoutOverlay) SetWarningThreshold(threshold time.Duration) {
	o.warningThreshold = threshold
}

// =============================================================================
// STATE MANAGEMENT
// =============================================================================

// Show displays the overlay with the given time remaining.
func (o *SessionTimeoutOverlay) Show(remaining time.Duration) {
	o.visible = true
	o.timeRemaining = remaining
	o.expired = remaining <= 0
}

// Hide hides the overlay.
func (o *SessionTimeoutOverlay) Hide() {
	o.visible = false
	o.expired = false
}

// UpdateTime updates the countdown timer.
func (o *SessionTimeoutOverlay) UpdateTime(remaining time.Duration) {
	o.timeRemaining = remaining
	if remaining <= 0 {
		o.expired = true
	}
}

// IsVisible returns whether the overlay is currently visible.
func (o *SessionTimeoutOverlay) IsVisible() bool {
	return o.visible
}

// IsExpired returns whether the session has expired.
func (o *SessionTimeoutOverlay) IsExpired() bool {
	return o.expired
}

// TimeRemaining returns the current time remaining.
func (o *SessionTimeoutOverlay) TimeRemaining() time.Duration {
	return o.timeRemaining
}

// =============================================================================
// BUBBLE TEA INTERFACE
// =============================================================================

// SessionTimeoutTickMsg signals a countdown tick for the session timeout overlay.
type SessionTimeoutTickMsg struct {
	Time time.Time
}

// SessionTimeoutWarningMsg signals the session is about to expire.
type SessionTimeoutWarningMsg struct {
	TimeRemaining time.Duration
}

// SessionExpiredMsg signals the session has expired and TUI should exit.
type SessionExpiredMsg struct{}

// SessionExtendedMsg signals the user extended their session by pressing a key.
type SessionExtendedMsg struct{}

// Init initializes the overlay (no-op for overlays).
func (o SessionTimeoutOverlay) Init() tea.Cmd {
	return nil
}

// Update handles messages for the overlay.
func (o SessionTimeoutOverlay) Update(msg tea.Msg) (SessionTimeoutOverlay, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		o.width = msg.Width
		o.height = msg.Height

	case tea.KeyMsg:
		// Any key press while warning is visible extends the session
		if o.visible && !o.expired {
			o.Hide()
			return o, func() tea.Msg {
				return SessionExtendedMsg{}
			}
		}

	case SessionTimeoutTickMsg:
		if o.visible {
			// Update remaining time (caller should handle actual timing)
			if o.timeRemaining <= 0 {
				o.expired = true
			}
		}
	}

	return o, nil
}

// View renders the session timeout overlay.
func (o SessionTimeoutOverlay) View() string {
	if !o.visible {
		return ""
	}

	if o.expired {
		return o.viewExpired()
	}
	return o.viewWarning()
}

// =============================================================================
// RENDER METHODS
// =============================================================================

// viewWarning renders the warning overlay before timeout.
func (o SessionTimeoutOverlay) viewWarning() string {
	width := o.width
	if width == 0 {
		width = 60
	}
	height := o.height
	if height == 0 {
		height = 24
	}

	// Calculate max content width
	maxWidth := width - 8
	if maxWidth < 40 {
		maxWidth = 40
	}
	if maxWidth > 60 {
		maxWidth = 60
	}

	// Format remaining time as M:SS
	timeStr := formatTimeRemaining(o.timeRemaining)

	// Build content
	var parts []string

	// Warning icon and title
	titleStyle := lipgloss.NewStyle().
		Foreground(styles.Amber).
		Bold(true)
	parts = append(parts, titleStyle.Render(styles.StatusIndicators.Warning+" Session Timeout Warning"))

	// Empty line
	parts = append(parts, "")

	// Main message with countdown
	timeStyle := lipgloss.NewStyle().
		Foreground(styles.Amber).
		Bold(true)

	msgStyle := lipgloss.NewStyle().
		Foreground(styles.TextPrimary).
		Width(maxWidth - 4).
		Align(lipgloss.Center)

	parts = append(parts, msgStyle.Render(
		"Session will expire in "+timeStyle.Render(timeStr)))

	// Empty line
	parts = append(parts, "")

	// Instruction
	hintStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary).
		Italic(true).
		Align(lipgloss.Center)
	parts = append(parts, hintStyle.Render("Press any key to continue working"))

	// Empty line
	parts = append(parts, "")

	// Compliance notice
	complianceStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Align(lipgloss.Center)
	parts = append(parts, complianceStyle.Render("DoD STIG AC-12 Session Termination"))

	content := lipgloss.JoinVertical(lipgloss.Center, parts...)

	// Create warning box with amber/yellow border
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(styles.Amber).
		Padding(1, 3).
		Width(maxWidth).
		Align(lipgloss.Center)

	box := boxStyle.Render(content)

	// Center the box
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		box,
		lipgloss.WithWhitespaceBackground(styles.SurfaceDim),
	)
}

// viewExpired renders the expired session message.
func (o SessionTimeoutOverlay) viewExpired() string {
	width := o.width
	if width == 0 {
		width = 60
	}
	height := o.height
	if height == 0 {
		height = 24
	}

	// Calculate max content width
	maxWidth := width - 8
	if maxWidth < 40 {
		maxWidth = 40
	}
	if maxWidth > 60 {
		maxWidth = 60
	}

	// Build content
	var parts []string

	// Error icon and title
	titleStyle := lipgloss.NewStyle().
		Foreground(styles.Rose).
		Bold(true)
	parts = append(parts, titleStyle.Render(styles.StatusIndicators.Error+" Session Expired"))

	// Empty line
	parts = append(parts, "")

	// Main message
	msgStyle := lipgloss.NewStyle().
		Foreground(styles.TextPrimary).
		Width(maxWidth - 4).
		Align(lipgloss.Center)
	parts = append(parts, msgStyle.Render(
		"Your session has timed out due to inactivity."))

	// Empty line
	parts = append(parts, "")

	// Exit notice
	exitStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary).
		Align(lipgloss.Center)
	parts = append(parts, exitStyle.Render("Application will exit automatically."))

	// Empty line
	parts = append(parts, "")

	// Compliance notice
	complianceStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Align(lipgloss.Center)
	parts = append(parts, complianceStyle.Render("DoD STIG AC-12 Compliance"))

	content := lipgloss.JoinVertical(lipgloss.Center, parts...)

	// Create expired box with rose/red border
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(styles.Rose).
		Padding(1, 3).
		Width(maxWidth).
		Align(lipgloss.Center)

	box := boxStyle.Render(content)

	// Center the box
	return lipgloss.Place(
		width, height,
		lipgloss.Center, lipgloss.Center,
		box,
		lipgloss.WithWhitespaceBackground(styles.SurfaceDim),
	)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// formatTimeRemaining formats a duration as M:SS for display.
func formatTimeRemaining(d time.Duration) string {
	if d < 0 {
		return "0:00"
	}

	totalSecs := int(d.Seconds())
	mins := totalSecs / 60
	secs := totalSecs % 60

	return fmt.Sprintf("%d:%02d", mins, secs)
}

// =============================================================================
// SESSION TIMEOUT CONFIGURATION CONSTANTS
// =============================================================================

const (
	// DefaultSessionTimeout is the default session timeout per DoD STIG (30 minutes).
	// Note: Per the plan, configurable from 15-30 minutes.
	DefaultSessionTimeout = 30 * time.Minute

	// MinSessionTimeout is the minimum allowed session timeout (15 minutes per DoD STIG).
	MinSessionTimeout = 15 * time.Minute

	// MaxSessionTimeout is the maximum allowed session timeout (30 minutes per DoD STIG).
	MaxSessionTimeout = 30 * time.Minute

	// DefaultWarningThreshold is when to show the warning overlay (2 minutes before timeout).
	DefaultWarningThreshold = 2 * time.Minute
)

// ValidateSessionTimeout clamps the timeout to the valid DoD STIG range.
func ValidateSessionTimeout(timeout time.Duration) time.Duration {
	if timeout < MinSessionTimeout {
		return MinSessionTimeout
	}
	if timeout > MaxSessionTimeout {
		return MaxSessionTimeout
	}
	return timeout
}
