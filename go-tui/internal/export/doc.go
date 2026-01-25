// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package export provides conversation export functionality for rigrun TUI.
//
// This package supports exporting conversations to various formats with
// styling, metadata, and optional opening in external applications.
//
// # Key Types
//
//   - Format: Export format enumeration (JSON, Markdown, HTML)
//   - Exporter: Main export interface
//   - Options: Export configuration options
//
// # Supported Formats
//
//   - JSON: Machine-readable with full metadata
//   - Markdown: Human-readable with formatting
//   - HTML: Styled for viewing in browsers
//
// # Usage
//
// Export a conversation:
//
//	exporter := export.New(export.Options{
//	    Format: export.FormatMarkdown,
//	    Open:   true,
//	})
//	path, err := exporter.Export(conversation)
//
// Export to specific file:
//
//	err := exporter.ExportTo(conversation, "output.md")
package export
