// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides the visual UI components for rigrun TUI.
package components

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/model"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// MESSAGE BUBBLE COMPONENT - Stunning message bubbles
// =============================================================================

// MessageBubble represents a styled message bubble
type MessageBubble struct {
	Message       *model.Message
	Width         int
	IsLatest      bool
	ShowTimestamp bool
	ShowStats     bool
	Streaming     bool
	theme         *styles.Theme
}

// NewMessageBubble creates a new MessageBubble
func NewMessageBubble(msg *model.Message, theme *styles.Theme) *MessageBubble {
	if msg == nil {
		// Return a safe default or handle gracefully
		return &MessageBubble{
			Message:   &model.Message{Role: "system", Content: ""},
			Width:     80,
			theme:     theme,
		}
	}
	return &MessageBubble{
		Message:       msg,
		Width:         80,
		IsLatest:      false,
		ShowTimestamp: true,
		ShowStats:     true,
		Streaming:     msg.IsStreaming,
		theme:         theme,
	}
}

// SetWidth sets the bubble width
func (b *MessageBubble) SetWidth(width int) {
	b.Width = width
}

// SetIsLatest marks this as the latest message
func (b *MessageBubble) SetIsLatest(latest bool) {
	b.IsLatest = latest
}

// View renders the message bubble
func (b *MessageBubble) View() string {
	switch b.Message.Role {
	case model.RoleUser:
		return b.renderUserBubble()
	case model.RoleAssistant:
		return b.renderAssistantBubble()
	case model.RoleSystem:
		return b.renderSystemBubble()
	case model.RoleTool:
		return b.renderToolBubble()
	default:
		return b.renderGenericBubble()
	}
}

// ==========================================================================
// USER BUBBLE - Blue tones, right-aligned feel
// ==========================================================================

func (b *MessageBubble) renderUserBubble() string {
	content := b.Message.GetDisplayContent()
	if content == "" {
		content = "..."
	}

	// Word wrap the content
	maxContentWidth := b.Width - 12 // Account for margins and padding
	if maxContentWidth < 20 {
		maxContentWidth = 20
	}
	wrappedContent := wordWrap(content, maxContentWidth)

	// Calculate actual content width (for the bubble)
	contentWidth := minInt(maxLineWidth(wrappedContent)+4, b.Width-8)

	// User bubble style - Beautiful blue tones
	bubbleStyle := lipgloss.NewStyle().
		Foreground(styles.UserBubbleFg).
		Background(styles.UserBubbleBg).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.UserBubbleBorder).
		Padding(0, 2).
		Width(contentWidth)

	bubble := bubbleStyle.Render(wrappedContent)

	// Role indicator - subtle, not bold
	roleStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)
	roleIndicator := roleStyle.Render("you")

	// Timestamp (dimmed)
	timestamp := ""
	if b.ShowTimestamp {
		timestamp = b.renderTimestamp()
	}

	// Build the header (role + timestamp)
	headerParts := []string{roleIndicator}
	if timestamp != "" {
		headerParts = append(headerParts, timestamp)
	}
	header := strings.Join(headerParts, " ")

	// Right-align the bubble with left margin
	leftMargin := b.Width - contentWidth - 4
	if leftMargin < 0 {
		leftMargin = 0
	}

	marginStyle := lipgloss.NewStyle().MarginLeft(leftMargin)

	// Assemble: header above, bubble below (right-aligned)
	headerLine := marginStyle.Render(header)
	bubbleLine := marginStyle.Render(bubble)

	return lipgloss.JoinVertical(lipgloss.Right, headerLine, bubbleLine)
}

// ==========================================================================
// ASSISTANT BUBBLE - Purple/violet tones, left-aligned
// ==========================================================================

func (b *MessageBubble) renderAssistantBubble() string {
	content := b.Message.GetDisplayContent()

	// Show cursor for streaming messages
	if b.Streaming {
		content = content + b.renderStreamingCursor()
	}

	if content == "" {
		content = "..."
	}

	// Word wrap the content
	maxContentWidth := b.Width - 12
	if maxContentWidth < 20 {
		maxContentWidth = 20
	}
	wrappedContent := wordWrap(content, maxContentWidth)

	// Calculate actual content width
	contentWidth := minInt(maxLineWidth(wrappedContent)+4, b.Width-8)

	// Assistant bubble style - Beautiful purple/violet tones
	bubbleStyle := lipgloss.NewStyle().
		Foreground(styles.AssistantBubbleFg).
		Background(styles.AssistantBubbleBg).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.AssistantBubbleBorder).
		Padding(0, 2).
		Width(contentWidth).
		MarginRight(4)

	bubble := bubbleStyle.Render(wrappedContent)

	// Role indicator - subtle, not bold
	roleStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)
	roleIndicator := roleStyle.Render("assistant")

	// Add model badge if available
	modelBadge := ""
	// Model info would come from message metadata in production

	// Timestamp
	timestamp := ""
	if b.ShowTimestamp {
		timestamp = b.renderTimestamp()
	}

	// Build header
	headerParts := []string{roleIndicator}
	if modelBadge != "" {
		headerParts = append(headerParts, modelBadge)
	}
	if timestamp != "" {
		headerParts = append(headerParts, timestamp)
	}
	header := strings.Join(headerParts, " ")

	// Statistics line (for completed messages)
	statsLine := ""
	if b.ShowStats && !b.Streaming && b.Message.TotalDuration > 0 {
		statsLine = b.renderStats()
	}

	// Assemble
	result := lipgloss.JoinVertical(lipgloss.Left, header, bubble)
	if statsLine != "" {
		result = lipgloss.JoinVertical(lipgloss.Left, result, statsLine)
	}

	return result
}

// ==========================================================================
// SYSTEM BUBBLE - Amber/yellow tones, centered
// ==========================================================================

func (b *MessageBubble) renderSystemBubble() string {
	content := b.Message.GetDisplayContent()
	if content == "" {
		content = "System message"
	}

	// Word wrap
	maxContentWidth := b.Width - 20
	if maxContentWidth < 30 {
		maxContentWidth = 30
	}
	wrappedContent := wordWrap(content, maxContentWidth)

	// Calculate bubble width
	contentWidth := minInt(maxLineWidth(wrappedContent)+4, b.Width-16)

	// System bubble style - Amber/yellow tones, centered with double border
	bubbleStyle := lipgloss.NewStyle().
		Foreground(styles.SystemBubbleFg).
		Background(styles.SystemBubbleBg).
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(styles.SystemBubbleBorder).
		Padding(0, 2).
		Width(contentWidth).
		Align(lipgloss.Center)

	bubble := bubbleStyle.Render(wrappedContent)

	// Center the bubble
	centerStyle := lipgloss.NewStyle().
		Width(b.Width).
		Align(lipgloss.Center)

	// System label - subtle styling
	labelStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)
	icon := labelStyle.Render("system")

	// Timestamp
	timestamp := ""
	if b.ShowTimestamp {
		timestamp = b.renderTimestamp()
	}

	header := icon
	if timestamp != "" {
		header = icon + " " + timestamp
	}

	return lipgloss.JoinVertical(
		lipgloss.Center,
		centerStyle.Render(header),
		centerStyle.Render(bubble),
	)
}

// ==========================================================================
// TOOL BUBBLE - Emerald for success, Rose for error
// ==========================================================================

func (b *MessageBubble) renderToolBubble() string {
	content := b.Message.ToolResult
	if content == "" {
		content = b.Message.GetDisplayContent()
	}

	// Truncate very long tool output
	maxLines := 20
	lines := strings.Split(content, "\n")
	truncated := false
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}
	content = strings.Join(lines, "\n")
	if truncated {
		content += "\n... (output truncated)"
	}

	// Word wrap
	maxContentWidth := b.Width - 10
	if maxContentWidth < 30 {
		maxContentWidth = 30
	}
	wrappedContent := wordWrap(content, maxContentWidth)

	// ACCESSIBILITY: Choose style based on success/error with high contrast colors
	var bubbleStyle lipgloss.Style
	var iconStyle lipgloss.Style
	var icon string

	if b.Message.IsSuccess {
		// ACCESSIBILITY: High contrast green with checkmark symbol for colorblind users
		bubbleStyle = lipgloss.NewStyle().
			Foreground(styles.ToolSuccessFg).
			Background(styles.ToolSuccessBg).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(styles.SuccessHighContrast).
			BorderLeft(true).
			PaddingLeft(2)

		iconStyle = lipgloss.NewStyle().
			Foreground(styles.SuccessHighContrast).
			Bold(true)
		icon = styles.StatusIndicators.Success // Checkmark for success
	} else {
		// ACCESSIBILITY: High contrast red with X mark symbol for colorblind users
		bubbleStyle = lipgloss.NewStyle().
			Foreground(styles.ToolErrorFg).
			Background(styles.ToolErrorBg).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(styles.ErrorHighContrast).
			BorderLeft(true).
			PaddingLeft(2)

		iconStyle = lipgloss.NewStyle().
			Foreground(styles.ErrorHighContrast).
			Bold(true)
		icon = styles.StatusIndicators.Error // X mark for error
	}

	bubble := bubbleStyle.Render(wrappedContent)

	// Tool name header
	toolNameStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary).
		Bold(true)

	toolName := b.Message.ToolName
	if toolName == "" {
		toolName = "Tool"
	}

	header := iconStyle.Render(icon) + " " + toolNameStyle.Render(toolName)

	// Timestamp
	if b.ShowTimestamp {
		header += " " + b.renderTimestamp()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, bubble)
}

// ==========================================================================
// GENERIC BUBBLE - Fallback for unknown roles
// ==========================================================================

func (b *MessageBubble) renderGenericBubble() string {
	content := b.Message.GetDisplayContent()
	if content == "" {
		content = "..."
	}

	// Word wrap
	maxContentWidth := b.Width - 10
	if maxContentWidth < 20 {
		maxContentWidth = 20
	}
	if maxContentWidth > b.Width-2 {
		maxContentWidth = b.Width - 2
	}
	wrappedContent := wordWrap(content, maxContentWidth)

	// Simple style
	bubbleStyle := lipgloss.NewStyle().
		Foreground(styles.TextPrimary).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Overlay).
		Padding(0, 2)

	return bubbleStyle.Render(wrappedContent)
}

// ==========================================================================
// HELPER METHODS
// ==========================================================================

// renderTimestamp renders a dimmed timestamp
func (b *MessageBubble) renderTimestamp() string {
	timestampStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)

	ts := b.Message.Timestamp
	if ts.IsZero() {
		return ""
	}

	// Format: "12:34 PM" or "Jan 5, 12:34 PM"
	now := time.Now()
	var formatted string

	if ts.Year() == now.Year() && ts.YearDay() == now.YearDay() {
		// Same day - just show time
		formatted = formatTime(ts)
	} else {
		// Different day - show date and time
		formatted = formatDate(ts) + ", " + formatTime(ts)
	}

	return timestampStyle.Render(formatted)
}

// renderStats renders message statistics
func (b *MessageBubble) renderStats() string {
	statsStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		PaddingLeft(2)

	stats := b.Message.FormatStats()
	if stats == "" {
		return ""
	}

	return statsStyle.Render(stats)
}

// renderStreamingCursor renders the streaming cursor animation
func (b *MessageBubble) renderStreamingCursor() string {
	cursorStyle := lipgloss.NewStyle().
		Foreground(styles.Purple).
		Blink(true)

	return cursorStyle.Render("_")
}

// ==========================================================================
// UTILITY FUNCTIONS
// ==========================================================================

// wordWrap wraps text to fit within the specified width
func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for lineIdx, line := range lines {
		if lineIdx > 0 {
			result.WriteString("\n")
		}

		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}

		currentLine := words[0]

		for _, word := range words[1:] {
			if runeLen(currentLine)+1+runeLen(word) <= width {
				currentLine += " " + word
			} else {
				result.WriteString(currentLine)
				result.WriteString("\n")
				currentLine = word
			}
		}

		result.WriteString(currentLine)
	}

	return result.String()
}

// maxLineWidth returns the width of the longest line in runes (characters).
// This correctly handles Unicode text where len() would return byte count.
func maxLineWidth(text string) int {
	maxWidth := 0
	for _, line := range strings.Split(text, "\n") {
		lineWidth := runeLen(line)
		if lineWidth > maxWidth {
			maxWidth = lineWidth
		}
	}
	return maxWidth
}

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// runeLen returns the number of runes (characters) in a string.
// This correctly handles Unicode text where len() would return byte count.
func runeLen(s string) int {
	return len([]rune(s))
}

// formatTime formats a time as "3:04 PM"
func formatTime(t time.Time) string {
	hour := t.Hour()
	minute := t.Minute()
	ampm := "AM"

	if hour >= 12 {
		ampm = "PM"
		if hour > 12 {
			hour -= 12
		}
	}
	if hour == 0 {
		hour = 12
	}

	minuteStr := util.IntToString(minute)
	if minute < 10 {
		minuteStr = "0" + minuteStr
	}

	return util.IntToString(hour) + ":" + minuteStr + " " + ampm
}

// formatDate formats a date as "Jan 5"
func formatDate(t time.Time) string {
	months := []string{
		"Jan", "Feb", "Mar", "Apr", "May", "Jun",
		"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
	}

	month := months[t.Month()-1]
	day := t.Day()

	return month + " " + util.IntToString(day)
}

// =============================================================================
// MESSAGE LIST COMPONENT - For rendering multiple messages
// =============================================================================

// MessageList represents a list of message bubbles
type MessageList struct {
	Messages      []*model.Message
	Width         int
	ShowTimestamps bool
	ShowStats     bool
	theme         *styles.Theme
}

// NewMessageList creates a new MessageList
func NewMessageList(theme *styles.Theme) *MessageList {
	return &MessageList{
		Messages:      []*model.Message{},
		Width:         80,
		ShowTimestamps: true,
		ShowStats:     true,
		theme:         theme,
	}
}

// SetMessages sets the messages to display
func (ml *MessageList) SetMessages(messages []*model.Message) {
	ml.Messages = messages
}

// SetWidth sets the list width
func (ml *MessageList) SetWidth(width int) {
	ml.Width = width
}

// View renders all messages
func (ml *MessageList) View() string {
	if len(ml.Messages) == 0 {
		// Empty state
		emptyStyle := lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Italic(true).
			Width(ml.Width).
			Align(lipgloss.Center).
			Padding(2, 0)

		return emptyStyle.Render("No messages yet. Start a conversation!")
	}

	var bubbles []string

	for i, msg := range ml.Messages {
		bubble := NewMessageBubble(msg, ml.theme)
		bubble.SetWidth(ml.Width)
		bubble.ShowTimestamp = ml.ShowTimestamps
		bubble.ShowStats = ml.ShowStats
		bubble.SetIsLatest(i == len(ml.Messages)-1)

		bubbles = append(bubbles, bubble.View())
	}

	// Add spacing between messages
	separator := "\n"

	return strings.Join(bubbles, separator)
}
