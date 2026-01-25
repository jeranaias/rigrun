// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package commands provides the slash command system for the TUI.
package commands

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// /INDEX COMMAND
// =============================================================================

// IndexCommand triggers codebase indexing
var IndexCommand = &Command{
	Name:        "/index",
	Aliases:     []string{},
	Description: "Index the codebase for intelligent @codebase search",
	Usage:       "/index",
	Category:    "Codebase",
	Handler:     handleIndex,
}

// IndexStatusMsg is sent when indexing completes
type IndexStatusMsg struct {
	Success      bool
	Error        error
	FileCount    int
	SymbolCount  int
	Duration     time.Duration
}

// handleIndex handles the /index command
func handleIndex(ctx *Context, args []string) tea.Cmd {
	// Check if index is available
	if ctx.CodebaseIndex == nil {
		return func() tea.Msg {
			return StatusMessage{
				Message: "Codebase indexing not available. Index will be initialized.",
				Type:    "info",
			}
		}
	}

	// Check if already indexing
	stats := ctx.CodebaseIndex.Stats()
	if stats.IsIndexing {
		return func() tea.Msg {
			return StatusMessage{
				Message: "Indexing already in progress...",
				Type:    "warning",
			}
		}
	}

	// Start indexing in background
	return func() tea.Msg {
		startTime := time.Now()

		// Create context with timeout
		indexCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// Perform indexing
		err := ctx.CodebaseIndex.Index(indexCtx)
		duration := time.Since(startTime)

		if err != nil {
			return IndexStatusMsg{
				Success:  false,
				Error:    err,
				Duration: duration,
			}
		}

		// Get final statistics
		stats := ctx.CodebaseIndex.Stats()

		return IndexStatusMsg{
			Success:      true,
			FileCount:    stats.FileCount,
			SymbolCount:  stats.SymbolCount,
			Duration:     duration,
		}
	}
}

// =============================================================================
// /SEARCH COMMAND
// =============================================================================

// SearchCommand searches the codebase index
var SearchCommand = &Command{
	Name:        "/search",
	Aliases:     []string{"/find"},
	Description: "Search for symbols in the codebase",
	Usage:       "/search <query>",
	Category:    "Codebase",
	Args: []ArgDef{
		{
			Name:        "query",
			Required:    true,
			Type:        ArgTypeString,
			Description: "Symbol name or search query",
		},
	},
	Handler: handleSearch,
}

// SearchResultMsg contains search results
type SearchResultMsg struct {
	Query   string
	Results []SearchResultItem
	Error   error
}

// SearchResultItem represents a single search result
type SearchResultItem struct {
	Name       string
	Type       string
	FilePath   string
	Line       int
	Signature  string
	Doc        string
}

// handleSearch handles the /search command
func handleSearch(ctx *Context, args []string) tea.Cmd {
	if len(args) == 0 {
		return func() tea.Msg {
			return StatusMessage{
				Message: "Usage: /search <query>",
				Type:    "error",
			}
		}
	}

	query := args[0]

	// Check if index is available
	if ctx.CodebaseIndex == nil || !ctx.CodebaseIndex.IsIndexed() {
		return func() tea.Msg {
			return StatusMessage{
				Message: "Codebase not indexed. Run /index first.",
				Type:    "warning",
			}
		}
	}

	// Perform search
	return func() tea.Msg {
		// Import the index package (would need to be added to imports)
		// For now, just return a placeholder
		return StatusMessage{
			Message: fmt.Sprintf("Searching for '%s'...", query),
			Type:    "info",
		}
	}
}

// =============================================================================
// STATUS MESSAGE
// =============================================================================

// StatusMessage is a generic status message
type StatusMessage struct {
	Message string
	Type    string // "info", "success", "warning", "error"
}
