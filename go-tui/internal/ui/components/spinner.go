// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// SPINNER MODEL
// =============================================================================

// Spinner is a customizable loading spinner component.
type Spinner struct {
	// Core spinner from bubbles
	spinner spinner.Model

	// Configuration
	style     SpinnerStyle
	message   string
	detail    string
	startTime time.Time

	// State
	isActive  bool
	showTimer bool
}

// SpinnerStyle defines the visual style for the spinner.
type SpinnerStyle int

const (
	SpinnerBraille SpinnerStyle = iota // Default braille spinner
	SpinnerDots                        // Classic dots
	SpinnerLine                        // Line rotation
	SpinnerPulse                       // Pulsing circle
	SpinnerBlock                       // Block animation
)

// NewSpinner creates a new spinner with default ASCII-compatible settings.
func NewSpinner() Spinner {
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: []string{"|", "/", "-", "\\"},
		FPS:    time.Second / 10,
	}

	return Spinner{
		spinner:   s,
		style:     SpinnerLine,
		message:   "Loading",
		showTimer: true,
	}
}

// NewSpinnerWithStyle creates a spinner with a specific style.
func NewSpinnerWithStyle(style SpinnerStyle) Spinner {
	s := NewSpinner()
	s.SetStyle(style)
	return s
}

// NewThinkingSpinner creates a spinner specifically for "Thinking..." state.
func NewThinkingSpinner() Spinner {
	s := NewSpinner()
	s.message = "Thinking"
	s.showTimer = true
	return s
}

// NewLoadingSpinner creates a spinner for model loading.
func NewLoadingSpinner() Spinner {
	s := NewSpinner()
	s.message = "Loading model"
	s.showTimer = false
	return s
}

// =============================================================================
// STYLE CONFIGURATION
// =============================================================================

// SetStyle changes the spinner animation style (ASCII-compatible styles).
func (s *Spinner) SetStyle(style SpinnerStyle) {
	s.style = style

	switch style {
	case SpinnerBraille:
		// Use line spinner as ASCII-safe alternative to braille
		s.spinner.Spinner = spinner.Spinner{
			Frames: []string{"|", "/", "-", "\\"},
			FPS:    time.Second / 10,
		}
	case SpinnerDots:
		s.spinner.Spinner = spinner.Spinner{
			Frames: []string{".  ", ".. ", "...", " ..", "  .", "   "},
			FPS:    time.Second / 6,
		}
	case SpinnerLine:
		s.spinner.Spinner = spinner.Spinner{
			Frames: []string{"|", "/", "-", "\\"},
			FPS:    time.Second / 10,
		}
	case SpinnerPulse:
		// ASCII alternative to pulsing circles
		s.spinner.Spinner = spinner.Spinner{
			Frames: []string{"( )", "(o)", "(O)", "(o)"},
			FPS:    time.Second / 8,
		}
	case SpinnerBlock:
		// ASCII alternative to block animation
		s.spinner.Spinner = spinner.Spinner{
			Frames: []string{"[    ]", "[=   ]", "[==  ]", "[=== ]", "[====]", "[ ===]", "[  ==]", "[   =]"},
			FPS:    time.Second / 15,
		}
	}
}

// SetMessage sets the text displayed next to the spinner.
func (s *Spinner) SetMessage(msg string) {
	s.message = msg
}

// SetDetail sets additional detail text below the spinner.
func (s *Spinner) SetDetail(detail string) {
	s.detail = detail
}

// SetShowTimer enables or disables the elapsed time display.
func (s *Spinner) SetShowTimer(show bool) {
	s.showTimer = show
}

// =============================================================================
// STATE MANAGEMENT
// =============================================================================

// Start activates the spinner and records the start time.
func (s *Spinner) Start() tea.Cmd {
	s.isActive = true
	s.startTime = time.Now()
	return s.spinner.Tick
}

// Stop deactivates the spinner.
func (s *Spinner) Stop() {
	s.isActive = false
}

// IsActive returns whether the spinner is currently running.
func (s *Spinner) IsActive() bool {
	return s.isActive
}

// GetElapsed returns the duration since the spinner started.
func (s *Spinner) GetElapsed() time.Duration {
	if s.startTime.IsZero() {
		return 0
	}
	return time.Since(s.startTime)
}

// =============================================================================
// BUBBLE TEA INTERFACE
// =============================================================================

// Init initializes the spinner.
func (s Spinner) Init() tea.Cmd {
	return nil
}

// Update handles messages for the spinner.
func (s Spinner) Update(msg tea.Msg) (Spinner, tea.Cmd) {
	if !s.isActive {
		return s, nil
	}

	var cmd tea.Cmd
	s.spinner, cmd = s.spinner.Update(msg)
	return s, cmd
}

// View renders the spinner.
func (s Spinner) View() string {
	if !s.isActive {
		return ""
	}

	// Spinner character
	spinnerView := lipgloss.NewStyle().
		Foreground(styles.Purple).
		Render(s.spinner.View())

	// Message text
	messageView := lipgloss.NewStyle().
		Foreground(styles.TextSecondary).
		Render(s.message)

	// Animated dots
	dotsView := lipgloss.NewStyle().
		Foreground(styles.Purple).
		Render("...")

	result := spinnerView + " " + messageView + dotsView

	// Add timer if enabled
	if s.showTimer && !s.startTime.IsZero() {
		elapsed := time.Since(s.startTime)
		timerView := lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Render(" (" + formatElapsed(elapsed) + ")")
		result += timerView
	}

	// Add detail if present
	if s.detail != "" {
		detailView := lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			PaddingLeft(2).
			Render(s.detail)
		result += "\n" + detailView
	}

	return result
}

// =============================================================================
// THINKING INDICATOR
// =============================================================================

// ThinkingIndicator is a specialized spinner for the "Thinking..." state.
type ThinkingIndicator struct {
	spinner   Spinner
	startTime time.Time
	detail    string
}

// NewThinkingIndicator creates a new thinking indicator.
func NewThinkingIndicator() ThinkingIndicator {
	return ThinkingIndicator{
		spinner: NewThinkingSpinner(),
	}
}

// Start begins the thinking animation.
func (t *ThinkingIndicator) Start() tea.Cmd {
	t.startTime = time.Now()
	return t.spinner.Start()
}

// Stop ends the thinking animation.
func (t *ThinkingIndicator) Stop() {
	t.spinner.Stop()
}

// SetDetail sets the detail text (e.g., "Processing context...")
func (t *ThinkingIndicator) SetDetail(detail string) {
	t.detail = detail
	t.spinner.SetDetail(detail)
}

// IsActive returns whether thinking is active.
func (t *ThinkingIndicator) IsActive() bool {
	return t.spinner.IsActive()
}

// GetElapsed returns time spent thinking.
func (t *ThinkingIndicator) GetElapsed() time.Duration {
	if t.startTime.IsZero() {
		return 0
	}
	return time.Since(t.startTime)
}

// Update handles messages.
func (t ThinkingIndicator) Update(msg tea.Msg) (ThinkingIndicator, tea.Cmd) {
	var cmd tea.Cmd
	t.spinner, cmd = t.spinner.Update(msg)
	return t, cmd
}

// View renders the thinking indicator.
func (t ThinkingIndicator) View() string {
	return t.spinner.View()
}

// =============================================================================
// MODEL LOADING SPINNER
// =============================================================================

// ModelLoadingSpinner shows loading state for model initialization.
type ModelLoadingSpinner struct {
	spinner   Spinner
	modelName string
}

// NewModelLoadingSpinner creates a model loading spinner.
func NewModelLoadingSpinner(modelName string) ModelLoadingSpinner {
	s := NewSpinner()
	s.SetMessage("Loading " + modelName)
	s.SetShowTimer(false)

	return ModelLoadingSpinner{
		spinner:   s,
		modelName: modelName,
	}
}

// Start begins the loading animation.
func (m *ModelLoadingSpinner) Start() tea.Cmd {
	return m.spinner.Start()
}

// Stop ends the loading animation.
func (m *ModelLoadingSpinner) Stop() {
	m.spinner.Stop()
}

// Update handles messages.
func (m ModelLoadingSpinner) Update(msg tea.Msg) (ModelLoadingSpinner, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// View renders the loading spinner.
func (m ModelLoadingSpinner) View() string {
	if !m.spinner.IsActive() {
		return ""
	}

	// Container box for model loading
	content := m.spinner.View()

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Purple).
		Padding(1, 2).
		Render(content)

	return box
}

// =============================================================================
// INLINE SPINNER
// =============================================================================

// InlineSpinner is a minimal spinner for inline use.
type InlineSpinner struct {
	spinner spinner.Model
	active  bool
}

// NewInlineSpinner creates a minimal inline ASCII-compatible spinner.
func NewInlineSpinner() InlineSpinner {
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: []string{"|", "/", "-", "\\"},
		FPS:    time.Second / 10,
	}
	return InlineSpinner{spinner: s}
}

// Start begins the spinner.
func (i *InlineSpinner) Start() tea.Cmd {
	i.active = true
	return i.spinner.Tick
}

// Stop ends the spinner.
func (i *InlineSpinner) Stop() {
	i.active = false
}

// Update handles messages.
func (i InlineSpinner) Update(msg tea.Msg) (InlineSpinner, tea.Cmd) {
	if !i.active {
		return i, nil
	}
	var cmd tea.Cmd
	i.spinner, cmd = i.spinner.Update(msg)
	return i, cmd
}

// View renders just the spinner character.
func (i InlineSpinner) View() string {
	if !i.active {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(styles.Purple).
		Render(i.spinner.View())
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// formatElapsed formats a duration for display.
func formatElapsed(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds < 60 {
		return formatSpinnerInt(seconds) + "s"
	}
	minutes := seconds / 60
	secs := seconds % 60
	return formatSpinnerInt(minutes) + "m " + formatSpinnerInt(secs) + "s"
}

// formatSpinnerInt converts an int to string without fmt.
func formatSpinnerInt(n int) string {
	if n == 0 {
		return "0"
	}
	if n == -9223372036854775808 { // math.MinInt64
		return "-9223372036854775808"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
