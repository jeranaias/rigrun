// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package chat

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeranaias/rigrun-tui/internal/commands"
	"github.com/jeranaias/rigrun-tui/internal/export"
)

// =============================================================================
// EXPORT HANDLERS
// =============================================================================

// handleExportConversation handles the export conversation message.
func (m Model) handleExportConversation(msg commands.ExportConversationMsg) (tea.Model, tea.Cmd) {
	format := msg.Format

	// Show status message
	m.conversation.AddSystemMessage(fmt.Sprintf("Exporting conversation to %s format...", format))
	m.updateViewport()

	// Export asynchronously
	return m, func() tea.Msg {
		// Set up export options
		opts := export.DefaultOptions()
		opts.OutputDir = "./exports"
		opts.OpenAfterExport = true
		opts.IncludeMetadata = true
		opts.IncludeTimestamps = true
		opts.Theme = "dark" // Could be made configurable

		// Export the conversation
		path, err := export.ExportModelConversation(m.conversation, format, opts)

		return commands.ExportCompleteMsg{
			Path:  path,
			Error: err,
		}
	}
}

// handleExportComplete handles the export completion message.
func (m Model) handleExportComplete(msg commands.ExportCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.conversation.AddSystemMessage(fmt.Sprintf("[FAIL] Export failed: %v", msg.Error))
	} else {
		m.conversation.AddSystemMessage(fmt.Sprintf("[OK] Successfully exported to: %s", msg.Path))
	}
	m.updateViewport()
	return m, nil
}
