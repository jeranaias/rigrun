// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// ERROR DISPLAY MODEL
// =============================================================================

// ErrorDisplay is a styled error message component.
type ErrorDisplay struct {
	// Error content
	category    ErrorCategory
	title       string
	message     string
	context     string   // Additional context about when/where the error occurred
	suggestions []string
	docsURL     string // Documentation URL for complex errors
	logHint     string // What to look for in logs

	// Display options
	dismissible   bool
	autoDismiss   time.Duration
	isToast       bool
	showCopyHint  bool // Show hint about copying error
	showDocsHint  bool // Show hint about opening docs
	logsPath      string // Path to log file (computed on creation)

	// State
	visible     bool
	createdAt   time.Time

	// Dimensions
	width  int
	height int
}

// NewErrorDisplay creates a new error display.
func NewErrorDisplay() ErrorDisplay {
	return ErrorDisplay{
		category:     CategoryUnknown,
		dismissible:  true,
		visible:      false,
		showCopyHint: true,
		logsPath:     getLogsPath(),
	}
}

// NewError creates an error display with title and message.
func NewError(title, message string) ErrorDisplay {
	return ErrorDisplay{
		category:     CategoryUnknown,
		title:        title,
		message:      message,
		dismissible:  true,
		visible:      true,
		createdAt:    time.Now(),
		showCopyHint: true,
		logsPath:     getLogsPath(),
	}
}

// NewErrorWithSuggestions creates an error with helpful suggestions.
func NewErrorWithSuggestions(title, message string, suggestions []string) ErrorDisplay {
	e := NewError(title, message)
	e.suggestions = suggestions
	return e
}

// NewEnhancedError creates an error with full enhanced details from a pattern.
func NewEnhancedError(pattern ErrorPattern, message string) ErrorDisplay {
	return ErrorDisplay{
		category:      pattern.Category,
		title:         pattern.Title,
		message:       message,
		suggestions:   pattern.Suggestions,
		docsURL:       pattern.DocsURL,
		logHint:       pattern.LogHint,
		dismissible:   true,
		visible:       true,
		createdAt:     time.Now(),
		showCopyHint:  true,
		showDocsHint:  pattern.DocsURL != "",
		logsPath:      getLogsPath(),
	}
}

// NewEnhancedErrorWithContext creates an error with additional context.
func NewEnhancedErrorWithContext(pattern ErrorPattern, message, context string) ErrorDisplay {
	e := NewEnhancedError(pattern, message)
	e.context = context
	return e
}

// NewToastError creates a dismissible toast-style error.
func NewToastError(message string) ErrorDisplay {
	return ErrorDisplay{
		title:       "Error",
		message:     message,
		dismissible: true,
		isToast:     true,
		visible:     true,
		createdAt:   time.Now(),
		autoDismiss: 5 * time.Second,
	}
}

// =============================================================================
// CONFIGURATION
// =============================================================================

// SetTitle sets the error title.
func (e *ErrorDisplay) SetTitle(title string) {
	e.title = title
}

// SetMessage sets the error message.
func (e *ErrorDisplay) SetMessage(message string) {
	e.message = message
}

// SetSuggestions sets the list of suggestions.
func (e *ErrorDisplay) SetSuggestions(suggestions []string) {
	e.suggestions = suggestions
}

// SetCategory sets the error category.
func (e *ErrorDisplay) SetCategory(category ErrorCategory) {
	e.category = category
}

// SetContext sets additional context about the error.
func (e *ErrorDisplay) SetContext(context string) {
	e.context = context
}

// SetDocsURL sets the documentation URL.
func (e *ErrorDisplay) SetDocsURL(url string) {
	e.docsURL = url
	e.showDocsHint = url != ""
}

// SetLogHint sets the hint about what to look for in logs.
func (e *ErrorDisplay) SetLogHint(hint string) {
	e.logHint = hint
}

// SetDismissible sets whether the error can be dismissed.
func (e *ErrorDisplay) SetDismissible(dismissible bool) {
	e.dismissible = dismissible
}

// SetAutoDismiss sets automatic dismissal duration.
func (e *ErrorDisplay) SetAutoDismiss(duration time.Duration) {
	e.autoDismiss = duration
}

// SetSize sets the display dimensions.
func (e *ErrorDisplay) SetSize(width, height int) {
	e.width = width
	e.height = height
}

// =============================================================================
// STATE MANAGEMENT
// =============================================================================

// Show displays the error.
func (e *ErrorDisplay) Show() {
	e.visible = true
	e.createdAt = time.Now()
}

// Hide hides the error.
func (e *ErrorDisplay) Hide() {
	e.visible = false
}

// IsVisible returns whether the error is visible.
func (e *ErrorDisplay) IsVisible() bool {
	return e.visible
}

// IsDismissible returns whether the error can be dismissed.
func (e *ErrorDisplay) IsDismissible() bool {
	return e.dismissible
}

// GetTitle returns the error title.
func (e *ErrorDisplay) GetTitle() string {
	return e.title
}

// GetMessage returns the error message.
func (e *ErrorDisplay) GetMessage() string {
	return e.message
}

// GetSuggestions returns the error suggestions.
func (e *ErrorDisplay) GetSuggestions() []string {
	return e.suggestions
}

// GetCategory returns the error category.
func (e *ErrorDisplay) GetCategory() ErrorCategory {
	return e.category
}

// ShouldAutoDismiss checks if auto-dismiss time has elapsed.
func (e *ErrorDisplay) ShouldAutoDismiss() bool {
	if e.autoDismiss == 0 {
		return false
	}
	return time.Since(e.createdAt) >= e.autoDismiss
}

// =============================================================================
// BUBBLE TEA INTERFACE
// =============================================================================

// Init initializes the error display.
func (e ErrorDisplay) Init() tea.Cmd {
	if e.autoDismiss > 0 {
		return tea.Tick(e.autoDismiss, func(t time.Time) tea.Msg {
			return ErrorAutoDismissMsg{}
		})
	}
	return nil
}

// ErrorAutoDismissMsg signals auto-dismissal.
type ErrorAutoDismissMsg struct{}

// ErrorCopyRequestMsg requests copying the error details to clipboard.
type ErrorCopyRequestMsg struct {
	Title   string
	Message string
	Context string
}

// ErrorDocsRequestMsg requests opening the documentation URL.
type ErrorDocsRequestMsg struct {
	URL string
}

// Update handles messages.
func (e ErrorDisplay) Update(msg tea.Msg) (ErrorDisplay, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		e.width = msg.Width
		e.height = msg.Height

	case tea.KeyMsg:
		if e.dismissible {
			switch msg.String() {
			case "esc", "enter", "q":
				e.Hide()
			case "c":
				// Copy error to clipboard - return a command to handle this
				// The parent component should handle this message
				return e, func() tea.Msg {
					return ErrorCopyRequestMsg{
						Title:   e.title,
						Message: e.message,
						Context: e.context,
					}
				}
			case "d":
				// Open documentation - return a command to handle this
				if e.docsURL != "" {
					return e, func() tea.Msg {
						return ErrorDocsRequestMsg{
							URL: e.docsURL,
						}
					}
				}
			}
		}

	case ErrorAutoDismissMsg:
		if e.autoDismiss > 0 {
			e.Hide()
		}
	}

	return e, nil
}

// View renders the error display.
func (e ErrorDisplay) View() string {
	if !e.visible {
		return ""
	}

	if e.isToast {
		return e.viewToast()
	}
	return e.viewBox()
}

// =============================================================================
// RENDER METHODS
// =============================================================================

// viewBox renders a full error box with enhanced information.
func (e ErrorDisplay) viewBox() string {
	width := e.width
	if width == 0 {
		width = 60
	}

	maxWidth := width - 8
	if maxWidth < 30 {
		maxWidth = 30
	}
	if maxWidth > 80 {
		maxWidth = 80
	}

	// Build content
	var parts []string

	// Title with category and icon
	categoryStr := string(e.category)
	if e.category == "" {
		categoryStr = string(CategoryUnknown)
	}

	// ACCESSIBILITY: Title with error icon (X mark) and high contrast red for colorblind users
	titleStyle := lipgloss.NewStyle().
		Foreground(styles.ErrorHighContrast).
		Bold(true)
	iconAndTitle := styles.StatusIndicators.Error + " " + e.title
	parts = append(parts, titleStyle.Render(iconAndTitle))
	parts = append(parts, "") // Spacer

	// Message
	if e.message != "" {
		messageStyle := lipgloss.NewStyle().
			Foreground(styles.TextPrimary).
			Width(maxWidth - 4)
		parts = append(parts, messageStyle.Render(e.message))
		parts = append(parts, "") // Spacer
	}

	// Context (if provided)
	if e.context != "" {
		contextLabelStyle := lipgloss.NewStyle().
			Foreground(styles.TextSecondary).
			Bold(true)
		parts = append(parts, contextLabelStyle.Render("Context:"))

		contextStyle := lipgloss.NewStyle().
			Foreground(styles.TextSecondary).
			Width(maxWidth - 4).
			Italic(true)
		parts = append(parts, contextStyle.Render(e.context))
		parts = append(parts, "") // Spacer
	}

	// Suggestions
	if len(e.suggestions) > 0 {
		suggestionTitle := lipgloss.NewStyle().
			Foreground(styles.InfoHighContrast).
			Bold(true).
			Render("Suggestions:")
		parts = append(parts, suggestionTitle)

		bulletStyle := lipgloss.NewStyle().
			Foreground(styles.Cyan)
		textStyle := lipgloss.NewStyle().
			Foreground(styles.TextSecondary)

		for _, suggestion := range e.suggestions {
			line := bulletStyle.Render("  * ") + textStyle.Render(suggestion)
			parts = append(parts, line)
		}
		parts = append(parts, "") // Spacer
	}

	// Documentation link (if available)
	if e.docsURL != "" {
		docsStyle := lipgloss.NewStyle().
			Foreground(styles.InfoHighContrast)
		docsLine := docsStyle.Render("[DOC] Docs: ") +
			lipgloss.NewStyle().Foreground(styles.TextSecondary).Render(e.docsURL)
		parts = append(parts, docsLine)
	}

	// Logs information
	if e.logsPath != "" {
		logsStyle := lipgloss.NewStyle().
			Foreground(styles.WarningHighContrast)
		logsLine := logsStyle.Render("[LOG] Logs: ") +
			lipgloss.NewStyle().Foreground(styles.TextSecondary).Render(e.logsPath)
		parts = append(parts, logsLine)

		// Log hint (if provided)
		if e.logHint != "" {
			logHintStyle := lipgloss.NewStyle().
				Foreground(styles.TextMuted).
				Italic(true).
				Width(maxWidth - 4)
			parts = append(parts, "   "+logHintStyle.Render("-> "+e.logHint))
		}
		parts = append(parts, "") // Spacer
	}

	// Action hints
	if e.dismissible {
		var hints []string
		hints = append(hints, "[Enter] Dismiss")
		if e.showCopyHint {
			hints = append(hints, "[c] Copy error")
		}
		if e.showDocsHint && e.docsURL != "" {
			hints = append(hints, "[d] Open docs")
		}

		hintStyle := lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Italic(true)
		parts = append(parts, hintStyle.Render(strings.Join(hints, "    ")))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Determine border color based on category
	borderColor := styles.ErrorHighContrast
	switch e.category {
	case CategoryNetwork, CategoryModel, CategoryTool:
		borderColor = styles.ErrorHighContrast
	case CategoryConfig, CategoryParse:
		borderColor = styles.WarningHighContrast
	case CategoryTimeout:
		borderColor = styles.InfoHighContrast
	case CategoryPermission:
		borderColor = styles.ErrorHighContrast
	case CategoryContext, CategoryResource:
		borderColor = styles.WarningHighContrast
	}

	// Create title for the box border
	borderTitle := " " + categoryStr + " Error "

	// ACCESSIBILITY: Create error box with high contrast border
	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true).
		Padding(1, 2).
		Width(maxWidth).
		Render(content)

	// Add category label to the top border
	boxWithTitle := addTitleToBox(box, borderTitle, borderColor)

	// Center if we have height
	if e.height > 0 {
		return lipgloss.Place(
			e.width, e.height,
			lipgloss.Center, lipgloss.Center,
			boxWithTitle,
		)
	}

	return boxWithTitle
}

// viewToast renders a compact toast-style error.
func (e ErrorDisplay) viewToast() string {
	width := e.width
	if width == 0 {
		width = 60
	}

	maxWidth := width - 4
	if maxWidth > 60 {
		maxWidth = 60
	}

	// ACCESSIBILITY: Icon and message with high contrast for colorblind users
	iconStyle := lipgloss.NewStyle().
		Foreground(styles.ErrorHighContrast).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(styles.TextPrimary)

	// ACCESSIBILITY: X mark symbol provides visual cue beyond color
	content := iconStyle.Render(styles.StatusIndicators.Error+" ") +
		messageStyle.Render(e.message)

	// ACCESSIBILITY: Toast container with high contrast border
	toast := lipgloss.NewStyle().
		Background(styles.SurfaceDim).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.ErrorHighContrast).
		Padding(0, 2).
		MaxWidth(maxWidth).
		Render(content)

	// Position at bottom right if we have dimensions
	if e.width > 0 && e.height > 0 {
		return lipgloss.Place(
			e.width, e.height,
			lipgloss.Right, lipgloss.Bottom,
			lipgloss.NewStyle().Margin(0, 2, 2, 0).Render(toast),
		)
	}

	return toast
}

// =============================================================================
// PREDEFINED ERROR TYPES
// =============================================================================

// ConnectionError creates an error for Ollama connection issues.
func ConnectionError() ErrorDisplay {
	return NewErrorWithSuggestions(
		"Connection Error",
		"Cannot connect to Ollama service.",
		[]string{
			"Start Ollama: ollama serve",
			"Check if Ollama is installed: ollama --version",
			"Verify Ollama is running on localhost:11434",
		},
	)
}

// ConnectionErrorWithDetails creates an error for Ollama connection issues with a specific error message.
func ConnectionErrorWithDetails(errMsg string) ErrorDisplay {
	return NewErrorWithSuggestions(
		"Connection Error",
		errMsg,
		[]string{
			"Start Ollama manually: ollama serve",
			"Check if Ollama is installed: ollama --version",
			"Verify Ollama is running on localhost:11434",
			"On Windows, check %LOCALAPPDATA%\\Programs\\Ollama\\",
		},
	)
}

// ModelNotFoundError creates an error for missing models.
func ModelNotFoundError(modelName string) ErrorDisplay {
	return NewErrorWithSuggestions(
		"Model Not Found",
		"The model '"+modelName+"' is not available.",
		[]string{
			"List available models: ollama list",
			"Pull the model: ollama pull " + modelName,
			"Check model name spelling",
		},
	)
}

// TimeoutError creates an error for request timeouts.
func TimeoutError() ErrorDisplay {
	return NewErrorWithSuggestions(
		"Request Timeout",
		"The request took too long to complete.",
		[]string{
			"Try again",
			"Check Ollama server load",
			"Consider using a smaller model",
		},
	)
}

// ContextExceededError creates an error for context window overflow.
func ContextExceededError() ErrorDisplay {
	return NewErrorWithSuggestions(
		"Context Exceeded",
		"The conversation has exceeded the model's context window.",
		[]string{
			"Start a new conversation: /new",
			"Clear history: /clear",
			"Remove some context with shorter messages",
		},
	)
}

// PermissionError creates an error for permission issues.
func PermissionError(operation string) ErrorDisplay {
	return NewErrorWithSuggestions(
		"Permission Denied",
		"The operation '"+operation+"' was denied.",
		[]string{
			"Grant permission when prompted",
			"Check file permissions",
			"Run with appropriate privileges",
		},
	)
}

// =============================================================================
// ERROR OVERLAY
// =============================================================================

// ErrorOverlay renders an error as a centered overlay.
func ErrorOverlay(width, height int, title, message string, suggestions []string) string {
	e := NewErrorWithSuggestions(title, message, suggestions)
	e.SetSize(width, height)
	return e.View()
}

// =============================================================================
// INLINE ERROR
// =============================================================================

// InlineError renders a minimal inline error message.
// ACCESSIBILITY: Uses X mark symbol and high contrast red for colorblind users.
func InlineError(message string) string {
	iconStyle := lipgloss.NewStyle().
		Foreground(styles.ErrorHighContrast).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(styles.ErrorHighContrast)

	return iconStyle.Render(styles.StatusIndicators.Error+" ") +
		messageStyle.Render(message)
}

// InlineWarning renders a minimal inline warning message.
// ACCESSIBILITY: Uses warning triangle symbol and high contrast amber for colorblind users.
func InlineWarning(message string) string {
	iconStyle := lipgloss.NewStyle().
		Foreground(styles.WarningHighContrast).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(styles.WarningHighContrast)

	return iconStyle.Render(styles.StatusIndicators.Warning+" ") +
		messageStyle.Render(message)
}

// InlineInfo renders a minimal inline info message.
// ACCESSIBILITY: Uses info circle symbol and high contrast blue for colorblind users.
func InlineInfo(message string) string {
	iconStyle := lipgloss.NewStyle().
		Foreground(styles.InfoHighContrast).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary)

	return iconStyle.Render(styles.StatusIndicators.Info+" ") +
		messageStyle.Render(message)
}

// InlineSuccess renders a minimal inline success message.
// ACCESSIBILITY: Uses checkmark symbol and high contrast green for colorblind users.
func InlineSuccess(message string) string {
	iconStyle := lipgloss.NewStyle().
		Foreground(styles.SuccessHighContrast).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(styles.SuccessHighContrast)

	return iconStyle.Render(styles.StatusIndicators.Success+" ") +
		messageStyle.Render(message)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// getLogsPath returns the path to the logs directory.
// On Windows: %USERPROFILE%\.rigrun\logs\rigrun.log
// On Unix: ~/.rigrun/logs/rigrun.log
func getLogsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.rigrun/logs/rigrun.log"
	}

	logsPath := filepath.Join(home, ".rigrun", "logs", "rigrun.log")
	// Convert to forward slashes for display consistency
	return filepath.ToSlash(logsPath)
}

// addTitleToBox adds a title to the top border of a box.
// This is a simple implementation that overlays the title on the first line.
func addTitleToBox(box, title string, color lipgloss.AdaptiveColor) string {
	lines := strings.Split(box, "\n")
	if len(lines) == 0 {
		return box
	}

	// Create styled title
	titleStyle := lipgloss.NewStyle().
		Foreground(color).
		Bold(true)
	styledTitle := titleStyle.Render(title)

	// Find the first line (the top border) and insert the title
	if len(lines) > 0 {
		firstLine := lines[0]
		// Simple replacement - insert title after the first border character
		if len(firstLine) > 4 {
			// Replace part of the border with the title
			prefix := string(firstLine[0:2])
			// Calculate where to place the title (near the start)
			lines[0] = prefix + styledTitle + string(firstLine[min(len(firstLine), 2+lipgloss.Width(styledTitle)):])
		}
	}

	return strings.Join(lines, "\n")
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
