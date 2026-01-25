// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package export

import (
	"fmt"

	"github.com/jeranaias/rigrun-tui/internal/model"
	"github.com/jeranaias/rigrun-tui/internal/storage"
)

// =============================================================================
// CONVERSION UTILITIES
// =============================================================================

// ConvertModelToStored converts a model.Conversation to storage.StoredConversation.
// This allows exporting active conversations that haven't been persisted yet.
func ConvertModelToStored(conv *model.Conversation) *storage.StoredConversation {
	if conv == nil {
		return nil
	}

	// Convert messages
	messages := make([]storage.StoredMessage, 0, len(conv.Messages))
	for _, msg := range conv.Messages {
		storedMsg := storage.StoredMessage{
			ID:        msg.ID,
			Role:      string(msg.Role),
			Content:   msg.GetDisplayContent(),
			Timestamp: msg.Timestamp,
		}

		// Copy statistics for assistant messages
		if msg.Role == model.RoleAssistant {
			storedMsg.TokenCount = msg.TokenCount
			storedMsg.DurationMs = msg.TotalDuration.Milliseconds()
			storedMsg.TokensPerSec = msg.TokensPerSec
			storedMsg.TTFTMs = msg.TTFT.Milliseconds()
		}

		// Copy tool information
		if msg.Role == model.RoleTool {
			storedMsg.ToolName = msg.ToolName
			storedMsg.ToolInput = msg.ToolInput
			storedMsg.ToolResult = msg.ToolResult
			storedMsg.IsSuccess = msg.IsSuccess
		}

		messages = append(messages, storedMsg)
	}

	// Extract context mentions (deduplicated)
	mentionsMap := make(map[string]bool)
	for _, msg := range conv.Messages {
		for _, mention := range msg.ContextMentions {
			mentionsMap[mention] = true
		}
	}
	mentions := make([]string, 0, len(mentionsMap))
	for mention := range mentionsMap {
		mentions = append(mentions, mention)
	}

	return &storage.StoredConversation{
		ID:         conv.ID,
		Summary:    conv.GetTitle(),
		Model:      conv.Model,
		CreatedAt:  conv.CreatedAt,
		UpdatedAt:  conv.UpdatedAt,
		Messages:   messages,
		TokensUsed: conv.TokensUsed,
		Mentions:   mentions,
	}
}

// ExportModelConversation exports a model.Conversation directly.
// This is a convenience function that combines conversion and export.
func ExportModelConversation(conv *model.Conversation, format string, opts *Options) (string, error) {
	storedConv := ConvertModelToStored(conv)
	if storedConv == nil {
		return "", fmt.Errorf("conversation is nil")
	}

	switch format {
	case "markdown", "md":
		return ExportMarkdown(storedConv, opts)
	case "html", "htm":
		return ExportHTML(storedConv, opts)
	case "json":
		exporter := NewJSONExporter(opts)
		return ExportToFile(storedConv, exporter, opts)
	default:
		return "", fmt.Errorf("unsupported export format: %s", format)
	}
}
