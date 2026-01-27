// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
//
// This file implements non-blocking error toasts inspired by lazygit's popup/toast system.
// Unlike modal error dialogs, toasts appear in the bottom-right corner and auto-dismiss,
// allowing users to continue interacting with the UI while errors are displayed.
package components

import (
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// TOAST TYPES
// =============================================================================

// ToastKind represents the type of toast notification.
// Inspired by lazygit's types.ToastKind.
type ToastKind int

const (
	// ToastKindStatus is an informational toast (cyan color)
	ToastKindStatus ToastKind = iota
	// ToastKindError is an error toast (rose/red color)
	ToastKindError
	// ToastKindWarning is a warning toast (amber color)
	ToastKindWarning
	// ToastKindSuccess is a success toast (emerald color)
	ToastKindSuccess
)

// DefaultToastDuration is the default auto-dismiss duration for status toasts.
const DefaultToastDuration = 4 * time.Second

// ErrorToastDuration is the auto-dismiss duration for error toasts (longer to read).
const ErrorToastDuration = 8 * time.Second

// WarningToastDuration is the auto-dismiss duration for warning toasts.
const WarningToastDuration = 6 * time.Second

// =============================================================================
// ERROR TOAST
// =============================================================================

// ErrorToast represents a non-blocking error notification.
// Unlike modal errors, toasts appear in the corner and auto-dismiss.
type ErrorToast struct {
	ID          int           // Unique identifier for this toast
	Message     string        // The toast message
	Kind        ToastKind     // Type of toast (error, warning, success, status)
	CreatedAt   time.Time     // When the toast was created
	Duration    time.Duration // How long before auto-dismiss
	Dismissible bool          // Whether user can dismiss early
	ShowRetry   bool          // Whether to show a retry button
	RetryAction func()        // Function to call on retry (if ShowRetry)
}

// NewErrorToast creates a new error toast with default 8-second duration.
func NewErrorToast(message string) ErrorToast {
	return ErrorToast{
		ID:          generateToastID(),
		Message:     message,
		Kind:        ToastKindError,
		CreatedAt:   time.Now(),
		Duration:    ErrorToastDuration,
		Dismissible: true,
	}
}

// NewWarningToast creates a new warning toast with default 6-second duration.
func NewWarningToast(message string) ErrorToast {
	return ErrorToast{
		ID:          generateToastID(),
		Message:     message,
		Kind:        ToastKindWarning,
		CreatedAt:   time.Now(),
		Duration:    WarningToastDuration,
		Dismissible: true,
	}
}

// NewStatusToast creates a new status/info toast with default 4-second duration.
func NewStatusToast(message string) ErrorToast {
	return ErrorToast{
		ID:          generateToastID(),
		Message:     message,
		Kind:        ToastKindStatus,
		CreatedAt:   time.Now(),
		Duration:    DefaultToastDuration,
		Dismissible: true,
	}
}

// NewSuccessToast creates a new success toast with default 4-second duration.
func NewSuccessToast(message string) ErrorToast {
	return ErrorToast{
		ID:          generateToastID(),
		Message:     message,
		Kind:        ToastKindSuccess,
		CreatedAt:   time.Now(),
		Duration:    DefaultToastDuration,
		Dismissible: true,
	}
}

// NewRetryableErrorToast creates an error toast with a retry action.
func NewRetryableErrorToast(message string, retryAction func()) ErrorToast {
	toast := NewErrorToast(message)
	toast.ShowRetry = true
	toast.RetryAction = retryAction
	return toast
}

// IsExpired returns true if the toast should be dismissed.
func (t *ErrorToast) IsExpired() bool {
	return time.Since(t.CreatedAt) >= t.Duration
}

// TimeRemaining returns how much time is left before auto-dismiss.
func (t *ErrorToast) TimeRemaining() time.Duration {
	remaining := t.Duration - time.Since(t.CreatedAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// =============================================================================
// TOAST MANAGER
// =============================================================================

// ToastManager manages multiple toast notifications.
// Inspired by lazygit's status.StatusManager.
type ToastManager struct {
	toasts    []ErrorToast
	nextID    int
	maxToasts int
	mutex     sync.Mutex
}

// NewToastManager creates a new toast manager.
func NewToastManager() *ToastManager {
	return &ToastManager{
		toasts:    make([]ErrorToast, 0),
		nextID:    1,
		maxToasts: 5, // Maximum visible toasts at once
	}
}

// AddToast adds a new toast to the manager.
func (m *ToastManager) AddToast(toast ErrorToast) int {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Assign ID if not set
	if toast.ID == 0 {
		toast.ID = m.nextID
		m.nextID++
	}

	// Add to front of list (newest first)
	m.toasts = append([]ErrorToast{toast}, m.toasts...)

	// Trim to max toasts
	if len(m.toasts) > m.maxToasts {
		m.toasts = m.toasts[:m.maxToasts]
	}

	return toast.ID
}

// AddError is a convenience method to add an error toast.
func (m *ToastManager) AddError(message string) int {
	return m.AddToast(NewErrorToast(message))
}

// AddWarning is a convenience method to add a warning toast.
func (m *ToastManager) AddWarning(message string) int {
	return m.AddToast(NewWarningToast(message))
}

// AddStatus is a convenience method to add a status toast.
func (m *ToastManager) AddStatus(message string) int {
	return m.AddToast(NewStatusToast(message))
}

// AddSuccess is a convenience method to add a success toast.
func (m *ToastManager) AddSuccess(message string) int {
	return m.AddToast(NewSuccessToast(message))
}

// RemoveToast removes a toast by ID.
func (m *ToastManager) RemoveToast(id int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for i, toast := range m.toasts {
		if toast.ID == id {
			m.toasts = append(m.toasts[:i], m.toasts[i+1:]...)
			return
		}
	}
}

// TickToasts removes expired toasts and returns the remaining toasts.
// Should be called periodically (e.g., every 100ms).
func (m *ToastManager) TickToasts() []ErrorToast {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Filter out expired toasts
	active := make([]ErrorToast, 0, len(m.toasts))
	for _, toast := range m.toasts {
		if !toast.IsExpired() {
			active = append(active, toast)
		}
	}
	m.toasts = active

	return m.toasts
}

// GetToasts returns a copy of the current toasts.
func (m *ToastManager) GetToasts() []ErrorToast {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	result := make([]ErrorToast, len(m.toasts))
	copy(result, m.toasts)
	return result
}

// HasToasts returns true if there are any active toasts.
func (m *ToastManager) HasToasts() bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return len(m.toasts) > 0
}

// Clear removes all toasts.
func (m *ToastManager) Clear() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.toasts = make([]ErrorToast, 0)
}

// =============================================================================
// TOAST MESSAGES
// =============================================================================

// ToastTickMsg is sent periodically to update toast state.
type ToastTickMsg struct {
	Time time.Time
}

// ToastDismissMsg requests dismissing a specific toast.
type ToastDismissMsg struct {
	ID int
}

// ToastRetryMsg requests retrying the action associated with a toast.
type ToastRetryMsg struct {
	ID int
}

// ToastAddMsg requests adding a new toast.
type ToastAddMsg struct {
	Message string
	Kind    ToastKind
}

// NewToastTickMsg creates a toast tick message.
func NewToastTickMsg() ToastTickMsg {
	return ToastTickMsg{Time: time.Now()}
}

// ToastTickCmd returns a command that ticks toasts every 100ms.
func ToastTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return ToastTickMsg{Time: t}
	})
}

// =============================================================================
// TOAST RENDERING
// =============================================================================

// RenderToast renders a single toast notification.
func RenderToast(toast ErrorToast, width int) string {
	maxWidth := 60
	if width > 0 && width-8 < maxWidth {
		maxWidth = width - 8
	}
	if maxWidth < 30 {
		maxWidth = 30
	}

	// Determine colors based on toast kind
	var iconColor, borderColor lipgloss.AdaptiveColor
	var icon string

	switch toast.Kind {
	case ToastKindError:
		iconColor = styles.Rose
		borderColor = styles.Rose
		icon = styles.StatusIndicators.Error
	case ToastKindWarning:
		iconColor = styles.Amber
		borderColor = styles.Amber
		icon = styles.StatusIndicators.Warning
	case ToastKindSuccess:
		iconColor = styles.Emerald
		borderColor = styles.Emerald
		icon = styles.StatusIndicators.Success
	default: // ToastKindStatus
		iconColor = styles.Cyan
		borderColor = styles.Cyan
		icon = styles.StatusIndicators.Info
	}

	// Build content
	iconStyle := lipgloss.NewStyle().
		Foreground(iconColor).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(styles.TextPrimary).
		Width(maxWidth - 8)

	// Wrap message text
	message := toast.Message
	if len(message) > maxWidth-10 {
		// Simple word wrap
		message = wrapToastText(message, maxWidth-10)
	}

	content := iconStyle.Render(icon+" ") + messageStyle.Render(message)

	// Add dismiss hint for dismissible toasts
	if toast.Dismissible {
		hintStyle := lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Italic(true)

		var hints []string
		if toast.ShowRetry {
			hints = append(hints, "[r] Retry")
		}
		hints = append(hints, "[x] Dismiss")

		// Show time remaining
		remaining := toast.TimeRemaining()
		if remaining > 0 {
			secs := int(remaining.Seconds())
			if secs > 0 {
				hints = append(hints, formatSeconds(secs)+"s")
			}
		}

		content += "\n" + hintStyle.Render(strings.Join(hints, "  "))
	}

	// Create toast box
	toastStyle := lipgloss.NewStyle().
		Background(styles.SurfaceDim).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 2).
		MaxWidth(maxWidth)

	return toastStyle.Render(content)
}

// RenderToastStack renders multiple toasts stacked vertically.
// Toasts are positioned in the bottom-right corner.
func RenderToastStack(toasts []ErrorToast, width, height int) string {
	if len(toasts) == 0 {
		return ""
	}

	// Render each toast
	renderedToasts := make([]string, 0, len(toasts))
	for _, toast := range toasts {
		rendered := RenderToast(toast, width)
		renderedToasts = append(renderedToasts, rendered)
	}

	// Stack toasts vertically (newest at bottom)
	stack := lipgloss.JoinVertical(lipgloss.Right, renderedToasts...)

	// Position in bottom-right with margin
	positioned := lipgloss.NewStyle().
		MarginRight(2).
		MarginBottom(1).
		Render(stack)

	// Place at bottom-right of screen
	if width > 0 && height > 0 {
		return lipgloss.Place(
			width, height,
			lipgloss.Right, lipgloss.Bottom,
			positioned,
		)
	}

	return positioned
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// Global toast ID counter
var toastIDMutex sync.Mutex
var toastIDCounter int

// generateToastID generates a unique toast ID.
func generateToastID() int {
	toastIDMutex.Lock()
	defer toastIDMutex.Unlock()
	toastIDCounter++
	return toastIDCounter
}

// wrapToastText performs simple word wrapping for toast messages.
func wrapToastText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len() == 0 {
			currentLine.WriteString(word)
		} else if currentLine.Len()+1+len(word) <= maxWidth {
			currentLine.WriteString(" ")
			currentLine.WriteString(word)
		} else {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		}
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return strings.Join(lines, "\n")
}

// formatSeconds formats seconds as a string for countdown display.
func formatSeconds(secs int) string {
	if secs <= 0 {
		return "0"
	}
	// Simple int to string conversion
	result := ""
	for secs > 0 {
		digit := secs % 10
		result = string(rune('0'+digit)) + result
		secs /= 10
	}
	if result == "" {
		return "0"
	}
	return result
}
