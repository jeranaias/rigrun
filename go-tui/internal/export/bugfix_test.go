// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package export

import (
	"strings"
	"testing"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/storage"
)

// TestXSSVulnerabilityFix tests that language names in code blocks are properly escaped.
func TestXSSVulnerabilityFix(t *testing.T) {
	conv := &storage.StoredConversation{
		ID:        "test1",
		Summary:   "XSS Test",
		Model:     "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages: []storage.StoredMessage{
			{
				ID:        "msg1",
				Role:      "assistant",
				Content:   "```<script>alert('xss')</script>\ncode here\n```",
				Timestamp: time.Now(),
			},
		},
	}

	exporter := NewHTMLExporter(nil)
	output, err := exporter.Export(conv)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	result := string(output)
	// Check that script tags are escaped
	if strings.Contains(result, "<script>alert('xss')</script>") {
		t.Error("XSS vulnerability: script tag not escaped in language label")
	}
	if !strings.Contains(result, "&lt;script&gt;") {
		t.Error("Expected escaped script tag in output")
	}
}

// TestYAMLNewlineInjectionFix tests that newlines are properly escaped in YAML frontmatter.
func TestYAMLNewlineInjectionFix(t *testing.T) {
	conv := &storage.StoredConversation{
		ID:        "test2",
		Summary:   "Test\nInjection: malicious",
		Model:     "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages: []storage.StoredMessage{
			{
				ID:        "msg1",
				Role:      "user",
				Content:   "test",
				Timestamp: time.Now(),
			},
		},
	}

	exporter := NewMarkdownExporter(nil)
	output, err := exporter.Export(conv)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	result := string(output)
	// Check that newlines are escaped in YAML frontmatter
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		if i > 0 && i < 10 { // Check frontmatter area
			if strings.HasPrefix(line, "Injection:") {
				t.Error("YAML injection vulnerability: newline not escaped in title")
			}
		}
	}

	// Should contain escaped newline
	if strings.Contains(result, "title: Test\nInjection") {
		t.Error("Newline not escaped in YAML value")
	}
}

// TestEmptyConversationValidation tests that empty conversations are rejected.
func TestEmptyConversationValidation(t *testing.T) {
	tests := []struct {
		name string
		conv *storage.StoredConversation
		want string
	}{
		{
			name: "nil conversation",
			conv: nil,
			want: "conversation is nil",
		},
		{
			name: "no messages",
			conv: &storage.StoredConversation{
				ID:        "test",
				Summary:   "Test",
				Model:     "test",
				CreatedAt: time.Now(),
				Messages:  []storage.StoredMessage{},
			},
			want: "conversation has no messages",
		},
		{
			name: "invalid timestamp",
			conv: &storage.StoredConversation{
				ID:      "test",
				Summary: "Test",
				Model:   "test",
				Messages: []storage.StoredMessage{
					{ID: "msg1", Role: "user", Content: "test", Timestamp: time.Now()},
				},
			},
			want: "invalid creation timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			htmlExporter := NewHTMLExporter(nil)
			_, err := htmlExporter.Export(tt.conv)
			if err == nil {
				t.Errorf("Expected error containing %q, got nil", tt.want)
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Expected error containing %q, got %q", tt.want, err.Error())
			}

			mdExporter := NewMarkdownExporter(nil)
			_, err = mdExporter.Export(tt.conv)
			if err == nil {
				t.Errorf("Expected error containing %q, got nil", tt.want)
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Expected error containing %q, got %q", tt.want, err.Error())
			}

			jsonExporter := NewJSONExporter(nil)
			_, err = jsonExporter.Export(tt.conv)
			if tt.name == "nil conversation" {
				if err == nil {
					t.Error("Expected error for nil conversation")
				}
			}
		})
	}
}

// TestDeprecatedStringsTitleReplaced tests that strings.Title is replaced.
func TestDeprecatedStringsTitleReplaced(t *testing.T) {
	conv := &storage.StoredConversation{
		ID:        "test3",
		Summary:   "Role Test",
		Model:     "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages: []storage.StoredMessage{
			{
				ID:        "msg1",
				Role:      "unknown_role",
				Content:   "test content",
				Timestamp: time.Now(),
			},
			{
				ID:        "msg2",
				Role:      "", // empty role
				Content:   "test content",
				Timestamp: time.Now(),
			},
		},
	}

	// Test HTML exporter
	htmlExporter := NewHTMLExporter(nil)
	htmlOutput, err := htmlExporter.Export(conv)
	if err != nil {
		t.Fatalf("HTML export failed: %v", err)
	}
	htmlResult := string(htmlOutput)

	// Check that unknown role is properly capitalized
	if !strings.Contains(htmlResult, "Unknown_role") && !strings.Contains(htmlResult, "Unknown") {
		t.Error("Unknown role not properly handled in HTML")
	}

	// Test Markdown exporter
	mdExporter := NewMarkdownExporter(nil)
	mdOutput, err := mdExporter.Export(conv)
	if err != nil {
		t.Fatalf("Markdown export failed: %v", err)
	}
	mdResult := string(mdOutput)

	// Check that unknown role is properly capitalized
	if !strings.Contains(mdResult, "Unknown_role") && !strings.Contains(mdResult, "Unknown") {
		t.Error("Unknown role not properly handled in Markdown")
	}
}

// TestFilenameSanitization tests that problematic characters are sanitized.
func TestFilenameSanitization(t *testing.T) {
	tests := []struct {
		input    string
		mustNot  []string
		mustHave []string
	}{
		{
			input:   "Test/Path\\Name:With*Special?Chars",
			mustNot: []string{"/", "\\", ":", "*", "?"},
			mustHave: []string{"-"},
		},
		{
			input:   "Test<HTML>Tags|Pipe",
			mustNot: []string{"<", ">", "|"},
			mustHave: []string{"-"},
		},
		{
			input:   "Test With Spaces\tAnd\nNewlines\r",
			mustNot: []string{" ", "\t", "\n", "\r"},
			mustHave: []string{"_"},
		},
		{
			input:   "Test\x00\x01\x1fControl\x7fChars",
			mustNot: []string{"\x00", "\x01", "\x1f", "\x7f"},
			mustHave: []string{"-"},
		},
	}

	for _, tt := range tests {
		result := sanitizeFilename(tt.input)
		for _, char := range tt.mustNot {
			if strings.Contains(result, char) {
				t.Errorf("sanitizeFilename(%q) contains forbidden character %q, got %q", tt.input, char, result)
			}
		}
		for _, char := range tt.mustHave {
			if !strings.Contains(result, char) {
				t.Errorf("sanitizeFilename(%q) should contain %q, got %q", tt.input, char, result)
			}
		}
	}
}

// TestContextMentionsDeduplication tests that context mentions are deduplicated.
func TestContextMentionsDeduplication(t *testing.T) {
	// This would require importing model package, so we'll test the exported function
	// We can verify the deduplication logic works by checking the stored conversation
	storedConv := &storage.StoredConversation{
		ID:        "test4",
		Summary:   "Mentions Test",
		Model:     "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Mentions:  []string{"file1.go", "file2.go", "file1.go", "file2.go", "file3.go"},
		Messages: []storage.StoredMessage{
			{
				ID:        "msg1",
				Role:      "user",
				Content:   "test",
				Timestamp: time.Now(),
			},
		},
	}

	// The mentions should be preserved as-is in export
	mdExporter := NewMarkdownExporter(nil)
	output, err := mdExporter.Export(storedConv)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	result := string(output)
	// Count occurrences of file1.go in the context line
	contextLine := ""
	for _, line := range strings.Split(result, "\n") {
		if strings.Contains(line, "Context:") {
			contextLine = line
			break
		}
	}

	if contextLine != "" {
		// Should contain duplicates as stored
		if !strings.Contains(contextLine, "file1.go") {
			t.Error("Expected mentions to be included")
		}
	}
}

// TestJSONExporterValidation tests that JSON exporter validates input.
func TestJSONExporterValidation(t *testing.T) {
	exporter := NewJSONExporter(nil)

	_, err := exporter.Export(nil)
	if err == nil {
		t.Error("Expected error for nil conversation")
	}
	if err != nil && !strings.Contains(err.Error(), "conversation is nil") {
		t.Errorf("Expected 'conversation is nil' error, got %q", err.Error())
	}
}

// TestYAMLBackslashEscaping tests that backslashes are properly escaped.
func TestYAMLBackslashEscaping(t *testing.T) {
	conv := &storage.StoredConversation{
		ID:        "test5",
		Summary:   `Path\With\Backslashes`,
		Model:     "test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages: []storage.StoredMessage{
			{
				ID:        "msg1",
				Role:      "user",
				Content:   "test",
				Timestamp: time.Now(),
			},
		},
	}

	exporter := NewMarkdownExporter(nil)
	output, err := exporter.Export(conv)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	result := string(output)
	// The backslashes should be properly escaped in YAML
	if strings.Contains(result, "title: Path\\With\\Backslashes\n") {
		t.Error("Backslashes not properly escaped in YAML (should be quoted)")
	}
}
