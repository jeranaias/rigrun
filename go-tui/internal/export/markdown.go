// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package export

import (
	"fmt"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/storage"
)

// =============================================================================
// MARKDOWN EXPORTER
// =============================================================================

// MarkdownExporter exports conversations to Markdown format.
type MarkdownExporter struct {
	options *Options
}

// NewMarkdownExporter creates a new Markdown exporter.
func NewMarkdownExporter(opts *Options) *MarkdownExporter {
	if opts == nil {
		opts = DefaultOptions()
	}
	return &MarkdownExporter{options: opts}
}

// Export converts a conversation to Markdown format.
func (e *MarkdownExporter) Export(conv *storage.StoredConversation) ([]byte, error) {
	// Validate conversation data
	if conv == nil {
		return nil, fmt.Errorf("conversation is nil")
	}
	if len(conv.Messages) == 0 {
		return nil, fmt.Errorf("conversation has no messages")
	}
	if conv.CreatedAt.IsZero() {
		return nil, fmt.Errorf("conversation has invalid creation timestamp")
	}

	var sb strings.Builder

	// YAML frontmatter with metadata
	if e.options.IncludeMetadata {
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("title: %s\n", escapeYAML(conv.Summary)))
		sb.WriteString(fmt.Sprintf("model: %s\n", conv.Model))
		sb.WriteString(fmt.Sprintf("date: %s\n", conv.CreatedAt.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("updated: %s\n", conv.UpdatedAt.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("messages: %d\n", len(conv.Messages)))
		if conv.TokensUsed > 0 {
			sb.WriteString(fmt.Sprintf("tokens: %d\n", conv.TokensUsed))
		}
		sb.WriteString(fmt.Sprintf("exported: %s\n", time.Now().Format(time.RFC3339)))
		sb.WriteString("generator: rigrun-tui\n")
		sb.WriteString("---\n\n")
	}

	// Title
	sb.WriteString(fmt.Sprintf("# %s\n\n", escapeMarkdown(conv.Summary)))

	// Metadata section
	if e.options.IncludeMetadata {
		sb.WriteString("## Session Information\n\n")
		sb.WriteString(fmt.Sprintf("- **Model**: %s\n", conv.Model))
		sb.WriteString(fmt.Sprintf("- **Created**: %s\n", formatTimestamp(conv.CreatedAt)))
		sb.WriteString(fmt.Sprintf("- **Last Updated**: %s\n", formatTimestamp(conv.UpdatedAt)))
		sb.WriteString(fmt.Sprintf("- **Messages**: %d\n", len(conv.Messages)))
		if conv.TokensUsed > 0 {
			sb.WriteString(fmt.Sprintf("- **Tokens Used**: %d\n", conv.TokensUsed))
		}
		if len(conv.Mentions) > 0 {
			sb.WriteString(fmt.Sprintf("- **Context**: %s\n", strings.Join(conv.Mentions, ", ")))
		}
		sb.WriteString("\n---\n\n")
	}

	// Conversation messages
	sb.WriteString("## Conversation\n\n")

	for i, msg := range conv.Messages {
		// Role label with timestamp
		roleLabel := e.formatRoleLabel(msg.Role)
		if e.options.IncludeTimestamps {
			sb.WriteString(fmt.Sprintf("### %s <sub>%s</sub>\n\n",
				roleLabel,
				formatShortTimestamp(msg.Timestamp)))
		} else {
			sb.WriteString(fmt.Sprintf("### %s\n\n", roleLabel))
		}

		// Message content
		content := msg.Content
		if content == "" && msg.Role == "tool" {
			content = e.formatToolMessage(&msg)
		}

		// Write content with proper code block handling
		sb.WriteString(e.formatMessageContent(content))
		sb.WriteString("\n\n")

		// Statistics for assistant messages
		if msg.Role == "assistant" && e.options.IncludeMetadata {
			stats := e.formatMessageStats(&msg)
			if stats != "" {
				sb.WriteString(stats)
				sb.WriteString("\n\n")
			}
		}

		// Add separator between messages (except last)
		if i < len(conv.Messages)-1 {
			sb.WriteString("---\n\n")
		}
	}

	// Footer
	sb.WriteString("\n---\n\n")
	sb.WriteString(fmt.Sprintf("*Exported from rigrun TUI on %s*\n",
		time.Now().Format("January 2, 2006 at 3:04 PM")))

	return []byte(sb.String()), nil
}

// FileExtension returns the file extension for Markdown.
func (e *MarkdownExporter) FileExtension() string {
	return ".md"
}

// MimeType returns the MIME type for Markdown.
func (e *MarkdownExporter) MimeType() string {
	return "text/markdown"
}

// =============================================================================
// FORMATTING HELPERS
// =============================================================================

// formatRoleLabel returns a formatted label for the message role.
func (e *MarkdownExporter) formatRoleLabel(role string) string {
	// Check for empty role
	if role == "" {
		return "Unknown"
	}

	switch role {
	case "user":
		return "[User]"
	case "assistant":
		return "[Assistant]"
	case "system":
		return "[System]"
	case "tool":
		return "[Tool]"
	default:
		// Replace deprecated strings.Title with proper title casing
		if len(role) > 0 {
			runes := []rune(role)
			return strings.ToUpper(string(runes[0])) + string(runes[1:])
		}
		return role
	}
}

// formatMessageContent formats the message content with proper code block handling.
func (e *MarkdownExporter) formatMessageContent(content string) string {
	// Content is already in markdown format - just ensure proper spacing
	content = strings.TrimSpace(content)

	// If content contains code blocks, ensure they're properly formatted
	// The content should already have proper code fences from the TUI
	return content
}

// formatToolMessage formats a tool message with input/output.
func (e *MarkdownExporter) formatToolMessage(msg *storage.StoredMessage) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("**Tool**: `%s`\n\n", msg.ToolName))

	if msg.ToolInput != "" {
		sb.WriteString("**Input**:\n```\n")
		sb.WriteString(msg.ToolInput)
		sb.WriteString("\n```\n\n")
	}

	if msg.ToolResult != "" {
		status := "[OK]"
		if !msg.IsSuccess {
			status = "[FAIL]"
		}
		sb.WriteString(fmt.Sprintf("**Result** %s:\n```\n", status))
		sb.WriteString(msg.ToolResult)
		sb.WriteString("\n```\n")
	}

	return sb.String()
}

// formatMessageStats formats statistics for a message.
func (e *MarkdownExporter) formatMessageStats(msg *storage.StoredMessage) string {
	if msg.TokenCount == 0 && msg.DurationMs == 0 {
		return ""
	}

	var parts []string

	if msg.TokenCount > 0 {
		parts = append(parts, fmt.Sprintf("Tokens: %d", msg.TokenCount))
	}

	if msg.DurationMs > 0 {
		parts = append(parts, fmt.Sprintf("Duration: %s", formatDuration(msg.DurationMs)))
	}

	if msg.TTFTMs > 0 {
		parts = append(parts, fmt.Sprintf("TTFT: %s", formatDuration(msg.TTFTMs)))
	}

	if msg.TokensPerSec > 0 {
		parts = append(parts, fmt.Sprintf("Speed: %s", formatTokensPerSec(msg.TokensPerSec)))
	}

	if len(parts) == 0 {
		return ""
	}

	return fmt.Sprintf("<sub>Stats: %s</sub>", strings.Join(parts, " | "))
}

// =============================================================================
// ESCAPING HELPERS
// =============================================================================

// escapeMarkdown escapes special Markdown characters in plain text.
func escapeMarkdown(s string) string {
	// Only escape characters that would break formatting in titles/headings
	s = strings.ReplaceAll(s, "#", "\\#")
	s = strings.ReplaceAll(s, "*", "\\*")
	s = strings.ReplaceAll(s, "_", "\\_")
	s = strings.ReplaceAll(s, "[", "\\[")
	s = strings.ReplaceAll(s, "]", "\\]")
	return s
}

// escapeYAML escapes special YAML characters in values.
func escapeYAML(s string) string {
	// Quote if contains special characters (including backslash)
	if strings.ContainsAny(s, ":#|>@`\"'[]{}!%&*\n\r\\") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		// Escape special characters including newlines and backslashes
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		s = strings.ReplaceAll(s, "\n", "\\n")
		s = strings.ReplaceAll(s, "\r", "\\r")
		return fmt.Sprintf("\"%s\"", s)
	}
	return s
}
