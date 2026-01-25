// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package export

import (
	"encoding/json"
	"fmt"

	"github.com/jeranaias/rigrun-tui/internal/storage"
)

// =============================================================================
// JSON EXPORTER
// =============================================================================

// JSONExporter exports conversations to JSON format.
// NOTE: JSON exports always include the complete conversation data structure
// and do not respect filtering options. This ensures the exported JSON is a
// faithful representation of the stored conversation that can be re-imported.
type JSONExporter struct {
	// Options are accepted but currently not used for filtering.
	// JSON exports always include complete data.
	options *Options
}

// NewJSONExporter creates a new JSON exporter.
// The options parameter is accepted for consistency with other exporters,
// but JSON exports always include complete conversation data.
func NewJSONExporter(opts *Options) *JSONExporter {
	if opts == nil {
		opts = DefaultOptions()
	}
	return &JSONExporter{options: opts}
}

// Export converts a conversation to JSON format.
// NOTE: This always exports the complete conversation regardless of options.
func (e *JSONExporter) Export(conv *storage.StoredConversation) ([]byte, error) {
	// Validate conversation data
	if conv == nil {
		return nil, fmt.Errorf("conversation is nil")
	}

	return json.MarshalIndent(conv, "", "  ")
}

// FileExtension returns the file extension for JSON.
func (e *JSONExporter) FileExtension() string {
	return ".json"
}

// MimeType returns the MIME type for JSON.
func (e *JSONExporter) MimeType() string {
	return "application/json"
}
