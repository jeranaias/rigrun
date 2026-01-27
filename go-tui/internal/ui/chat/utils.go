// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
package chat

import (
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/jeranaias/rigrun-tui/internal/model"
)

// =============================================================================
// FORMATTING UTILITIES
// =============================================================================

// formatTimestamp formats a timestamp for display in chat messages.
// It uses smart formatting based on how recent the timestamp is:
//   - Today: just time (e.g., "15:04")
//   - This week: day and time (e.g., "Mon 15:04")
//   - Older: date and time (e.g., "Jan 2 15:04")
func formatTimestamp(t time.Time) string {
	now := time.Now()

	// Today: just time
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}

	// This week: day and time
	if now.Sub(t) < 7*24*time.Hour {
		return t.Format("Mon 15:04")
	}

	// Older: date and time
	return t.Format("Jan 2 15:04")
}

// formatBool formats a boolean as an enabled/disabled string.
// This is the canonical implementation for the chat package.
// For other formatting needs (yes/no), use fmt.Sprintf or implement separately.
func formatBool(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}

// formatFloat64 formats a float with one decimal place with proper rounding.
// Examples: 45.9 -> "45.9", 123.456 -> "123.5", -5.3 -> "-5.3"
func formatFloat64(f float64) string {
	// Handle special cases
	if f != f { // NaN check
		return "NaN"
	}
	if f > 9223372036854775807 { // Larger than MaxInt64
		return "Inf"
	}
	if f < -9223372036854775808 { // Smaller than MinInt64
		return "-Inf"
	}

	// Round to one decimal place by adding 0.05 (or -0.05 for negatives)
	negative := f < 0
	absF := f
	if negative {
		absF = -f
	}

	// Add 0.05 for rounding then multiply by 10 and truncate
	rounded := absF + 0.05
	whole := int(rounded)
	frac := int((rounded - float64(whole)) * 10)

	// Build the result
	result := formatInt(whole) + "." + formatInt(frac)
	if negative {
		result = "-" + result
	}
	return result
}

// formatInt formats an integer as a string without external dependencies.
// This is used throughout the chat package for number formatting.
func formatInt(n int) string {
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

// formatNumberWithCommas formats an integer with thousand separators.
// Example: 1234567 -> "1,234,567"
func formatNumberWithCommas(n int) string {
	if n == -9223372036854775808 { // math.MinInt64
		return "-9,223,372,036,854,775,808"
	}
	negative := n < 0
	if negative {
		n = -n
	}

	if n < 1000 {
		if negative {
			return "-" + formatInt(n)
		}
		return formatInt(n)
	}

	s := formatInt(n)
	result := ""
	count := 0

	for i := len(s) - 1; i >= 0; i-- {
		if count > 0 && count%3 == 0 {
			result = "," + result
		}
		result = string(s[i]) + result
		count++
	}

	if negative {
		result = "-" + result
	}
	return result
}

// formatCost formats a cost in cents for display.
// Returns format like "0.05c" for cents or "$1.23" for dollars.
func formatCost(cents float64) string {
	if cents < 1.0 {
		return formatFloat64(cents) + "c"
	} else if cents < 100.0 {
		return formatFloat64(cents) + "c"
	}
	// Convert to dollars for larger amounts
	return "$" + formatFloat64(cents/100.0)
}

// =============================================================================
// CLIPBOARD UTILITIES
// =============================================================================

// copyToClipboard copies the given text to the system clipboard.
// Returns an error if the clipboard is not available or the operation fails.
func copyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

// =============================================================================
// TEXT UTILITIES
// =============================================================================

// wordWrap wraps text to a maximum width, handling Unicode correctly.
// It preserves existing line breaks and intelligently breaks long lines at spaces.
// This is an alias for wrapText for API compatibility.
func wordWrap(text string, maxWidth int) string {
	return wrapText(text, maxWidth)
}

// calculateContentWidth calculates the safe content width for message rendering.
// It accounts for margins and padding to prevent content overflow.
// Parameters:
//   - totalWidth: the total available width (e.g., terminal width)
//   - margin: the margin on each side (default 2 characters each side = 4 total)
//
// Returns the content width that should be used for text wrapping.
// Returns minimum of 3 for extremely narrow widths.
func calculateContentWidth(totalWidth, margin int) int {
	contentWidth := totalWidth - margin
	if contentWidth < 3 {
		contentWidth = 3 // Minimum content width
	}
	return contentWidth
}

// wrapText wraps text to a maximum width, handling Unicode correctly.
// It preserves existing line breaks and intelligently breaks long lines at spaces.
func wrapText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		// Convert to runes to handle Unicode characters correctly
		runes := []rune(line)

		// Wrap long lines
		for len(runes) > maxWidth {
			// Find a good break point (look for space)
			breakPoint := maxWidth
			for j := maxWidth; j > 0; j-- {
				if runes[j] == ' ' {
					breakPoint = j
					break
				}
			}

			result.WriteString(string(runes[:breakPoint]))
			result.WriteString("\n")
			runes = []rune(strings.TrimLeft(string(runes[breakPoint:]), " "))
		}
		result.WriteString(string(runes))
	}

	return result.String()
}

// =============================================================================
// MESSAGE PREVIEW UTILITIES
// =============================================================================

// MaxPreviewLines is the maximum number of lines to show before truncating a message.
const MaxPreviewLines = 20

// renderMessagePreview renders a message with optional truncation for long content.
// If the message has more lines than MaxPreviewLines and expanded is false,
// it shows a preview with a "show more" indicator.
// Parameters:
//   - content: the message content to render
//   - expanded: if true, show full content; if false, truncate long messages
//
// Returns the content, possibly truncated with a "show more" indicator.
func renderMessagePreview(content string, expanded bool) string {
	lines := strings.Split(content, "\n")

	if len(lines) <= MaxPreviewLines || expanded {
		return content
	}

	preview := strings.Join(lines[:MaxPreviewLines], "\n")
	moreLines := len(lines) - MaxPreviewLines
	more := "\n... [" + formatInt(moreLines) + " more lines - press Enter to expand]"
	return preview + more
}

// isMessageTruncatable returns true if the message content would be truncated.
func isMessageTruncatable(content string) bool {
	lines := strings.Split(content, "\n")
	return len(lines) > MaxPreviewLines
}

// =============================================================================
// CONVERSATION UTILITIES
// =============================================================================

// formatHistory formats conversation history for display.
// Returns the last N messages formatted as a string.
func formatHistory(conv *model.Conversation, n int) string {
	messages := conv.GetHistory()
	totalMessages := len(messages)

	if totalMessages == 0 {
		return "No messages in conversation history"
	}

	start := 0
	if totalMessages > n {
		start = totalMessages - n
	}

	estimatedSize := (totalMessages - start) * 200
	var history strings.Builder
	history.Grow(estimatedSize)

	history.WriteString("Last ")
	history.WriteString(formatInt(totalMessages - start))
	history.WriteString(" messages (of ")
	history.WriteString(formatInt(totalMessages))
	history.WriteString(" total):\n\n")

	const maxHistorySize = 500000
	for i := start; i < totalMessages; i++ {
		if history.Len() > maxHistorySize {
			history.WriteString("\n[Output truncated - use /export for full history]")
			break
		}

		msg := messages[i]
		timeStr := msg.Timestamp.Format("15:04:05")

		roleStr := ""
		switch msg.Role {
		case model.RoleUser:
			roleStr = "You"
		case model.RoleAssistant:
			roleStr = "Assistant"
		case model.RoleSystem:
			roleStr = "System"
		case model.RoleTool:
			roleStr = "Tool"
		default:
			roleStr = string(msg.Role)
		}

		history.WriteString("[")
		history.WriteString(formatInt(i + 1))
		history.WriteString("] ")
		history.WriteString(timeStr)
		history.WriteString(" - ")
		history.WriteString(roleStr)
		history.WriteString(":\n")

		content := msg.GetDisplayContent()
		if len(content) > 100 {
			runes := []rune(content)
			if len(runes) > 100 {
				content = string(runes[:100]) + "..."
			}
		}
		history.WriteString(content)
		history.WriteString("\n\n")
	}

	return history.String()
}
