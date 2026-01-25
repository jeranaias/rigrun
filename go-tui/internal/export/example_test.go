// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package export_test

import (
	"fmt"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/export"
	"github.com/jeranaias/rigrun-tui/internal/storage"
)

// ExampleExportMarkdown demonstrates exporting a conversation to Markdown format.
func ExampleExportMarkdown() {
	// Create a sample conversation
	conv := &storage.StoredConversation{
		ID:        "conv_example123",
		Summary:   "My First Chat",
		Model:     "qwen2.5-coder:14b",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages: []storage.StoredMessage{
			{
				ID:        "msg_1",
				Role:      "user",
				Content:   "How do I write a Hello World program in Python?",
				Timestamp: time.Now(),
			},
			{
				ID:      "msg_2",
				Role:    "assistant",
				Content: "Here's a simple Hello World program in Python:\n\n```python\nprint(\"Hello, World!\")\n```\n\nThis single line prints the message to the console.",
				Timestamp: time.Now(),
				TokenCount:   45,
				DurationMs:   1234,
				TokensPerSec: 36.5,
				TTFTMs:       156,
			},
		},
		TokensUsed: 60,
	}

	// Set up export options
	opts := export.DefaultOptions()
	opts.OutputDir = "./examples"
	opts.OpenAfterExport = false // Don't auto-open in example

	// Export to Markdown
	path, err := export.ExportMarkdown(conv, opts)
	if err != nil {
		fmt.Printf("Export failed: %v\n", err)
		return
	}

	fmt.Printf("Exported to: %s\n", path)
	// Output would be something like:
	// Exported to: ./examples/conversation_My_First_Chat_20260124_143052.md
}

// ExampleExportHTML demonstrates exporting a conversation to HTML format.
func ExampleExportHTML() {
	// Create a sample conversation
	conv := &storage.StoredConversation{
		ID:        "conv_example456",
		Summary:   "Debugging Rust Code",
		Model:     "qwen2.5-coder:14b",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages: []storage.StoredMessage{
			{
				ID:        "msg_1",
				Role:      "user",
				Content:   "Why is my Rust code not compiling?",
				Timestamp: time.Now(),
			},
			{
				ID:      "msg_2",
				Role:    "assistant",
				Content: "I'd be happy to help debug your Rust code! However, I don't see any code in your message. Please share:\n\n1. The code that's not compiling\n2. The error message you're getting\n\nFor example:\n\n```rust\nfn main() {\n    println!(\"Hello, world!\");\n}\n```",
				Timestamp:    time.Now(),
				TokenCount:   78,
				DurationMs:   2100,
				TokensPerSec: 37.1,
				TTFTMs:       189,
			},
		},
		TokensUsed: 95,
	}

	// Set up export options with light theme
	opts := export.DefaultOptions()
	opts.OutputDir = "./examples"
	opts.Theme = "light"
	opts.OpenAfterExport = false

	// Export to HTML
	path, err := export.ExportHTML(conv, opts)
	if err != nil {
		fmt.Printf("Export failed: %v\n", err)
		return
	}

	fmt.Printf("Exported to: %s\n", path)
	// Output would be something like:
	// Exported to: ./examples/conversation_Debugging_Rust_Code_20260124_143052.html
}

// ExampleExportToFile demonstrates using a custom exporter.
func ExampleExportToFile() {
	// Create a sample conversation
	conv := &storage.StoredConversation{
		ID:        "conv_example789",
		Summary:   "Quick Question",
		Model:     "qwen2.5-coder:14b",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages: []storage.StoredMessage{
			{
				ID:        "msg_1",
				Role:      "user",
				Content:   "What's the capital of France?",
				Timestamp: time.Now(),
			},
			{
				ID:        "msg_2",
				Role:      "assistant",
				Content:   "The capital of France is Paris.",
				Timestamp: time.Now(),
			},
		},
	}

	// Export with custom options
	opts := &export.Options{
		OutputDir:         "./examples/json",
		OpenAfterExport:   false,
		IncludeMetadata:   true,
		IncludeTimestamps: true,
	}

	// Create JSON exporter
	exporter := export.NewJSONExporter(opts)

	path, err := export.ExportToFile(conv, exporter, opts)
	if err != nil {
		fmt.Printf("Export failed: %v\n", err)
		return
	}

	fmt.Printf("Exported to: %s\n", path)
	// Output would be something like:
	// Exported to: ./examples/json/conversation_Quick_Question_20260124_143052.json
}
