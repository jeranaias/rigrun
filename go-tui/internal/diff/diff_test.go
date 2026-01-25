// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package diff provides diff computation and formatting for file changes.
package diff

import (
	"strings"
	"testing"
)

func TestComputeDiff_NewFile(t *testing.T) {
	oldContent := ""
	newContent := "line1\nline2\nline3"

	d := ComputeDiff("test.txt", oldContent, newContent)

	if d.Stats.FileMode != "new" {
		t.Errorf("Expected FileMode 'new', got '%s'", d.Stats.FileMode)
	}

	if d.Stats.Additions != 3 {
		t.Errorf("Expected 3 additions, got %d", d.Stats.Additions)
	}

	if d.Stats.Deletions != 0 {
		t.Errorf("Expected 0 deletions, got %d", d.Stats.Deletions)
	}
}

func TestComputeDiff_DeletedFile(t *testing.T) {
	oldContent := "line1\nline2\nline3"
	newContent := ""

	d := ComputeDiff("test.txt", oldContent, newContent)

	if d.Stats.FileMode != "deleted" {
		t.Errorf("Expected FileMode 'deleted', got '%s'", d.Stats.FileMode)
	}

	if d.Stats.Additions != 0 {
		t.Errorf("Expected 0 additions, got %d", d.Stats.Additions)
	}

	if d.Stats.Deletions != 3 {
		t.Errorf("Expected 3 deletions, got %d", d.Stats.Deletions)
	}
}

func TestComputeDiff_Modified(t *testing.T) {
	oldContent := "line1\nline2\nline3"
	newContent := "line1\nmodified\nline3\nline4"

	d := ComputeDiff("test.txt", oldContent, newContent)

	if d.Stats.FileMode != "modified" {
		t.Errorf("Expected FileMode 'modified', got '%s'", d.Stats.FileMode)
	}

	if d.Stats.Additions != 2 {
		t.Errorf("Expected 2 additions, got %d", d.Stats.Additions)
	}

	if d.Stats.Deletions != 1 {
		t.Errorf("Expected 1 deletion, got %d", d.Stats.Deletions)
	}
}

func TestComputeDiff_NoChanges(t *testing.T) {
	content := "line1\nline2\nline3"

	d := ComputeDiff("test.txt", content, content)

	if d.Stats.Additions != 0 {
		t.Errorf("Expected 0 additions, got %d", d.Stats.Additions)
	}

	if d.Stats.Deletions != 0 {
		t.Errorf("Expected 0 deletions, got %d", d.Stats.Deletions)
	}
}

func TestDiffLineType_String(t *testing.T) {
	tests := []struct {
		lineType DiffLineType
		expected string
	}{
		{DiffLineContext, "context"},
		{DiffLineAdded, "added"},
		{DiffLineRemoved, "removed"},
	}

	for _, tt := range tests {
		result := tt.lineType.String()
		if result != tt.expected {
			t.Errorf("Expected %s, got %s", tt.expected, result)
		}
	}
}

func TestDiffLineType_Prefix(t *testing.T) {
	tests := []struct {
		lineType DiffLineType
		expected string
	}{
		{DiffLineContext, " "},
		{DiffLineAdded, "+"},
		{DiffLineRemoved, "-"},
	}

	for _, tt := range tests {
		result := tt.lineType.Prefix()
		if result != tt.expected {
			t.Errorf("Expected '%s', got '%s'", tt.expected, result)
		}
	}
}

func TestFormatUnifiedDiff(t *testing.T) {
	oldContent := "line1\nline2\nline3"
	newContent := "line1\nmodified\nline3"

	d := ComputeDiff("test.txt", oldContent, newContent)
	unified := FormatUnifiedDiff(d)

	// Check header
	if !strings.Contains(unified, "--- a/test.txt") {
		t.Error("Missing old file header")
	}
	if !strings.Contains(unified, "+++ b/test.txt") {
		t.Error("Missing new file header")
	}

	// Check that it contains diff markers
	if !strings.Contains(unified, "@@") {
		t.Error("Missing hunk header")
	}
}

func TestDiff_Summary(t *testing.T) {
	tests := []struct {
		name       string
		oldContent string
		newContent string
		expected   string
	}{
		{
			name:       "new file",
			oldContent: "",
			newContent: "line1\nline2",
			expected:   "New file +2",
		},
		{
			name:       "deleted file",
			oldContent: "line1\nline2",
			newContent: "",
			expected:   "File deleted -2",
		},
		{
			name:       "modified file",
			oldContent: "line1\nline2\nline3",
			newContent: "line1\nmodified\nline3\nline4",
			expected:   "Modified +2 -1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := ComputeDiff("test.txt", tt.oldContent, tt.newContent)
			summary := d.Summary()
			if summary != tt.expected {
				t.Errorf("Expected summary '%s', got '%s'", tt.expected, summary)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "empty",
			content:  "",
			expected: []string{},
		},
		{
			name:     "single line no newline",
			content:  "line1",
			expected: []string{"line1"},
		},
		{
			name:     "single line with newline",
			content:  "line1\n",
			expected: []string{"line1"},
		},
		{
			name:     "multiple lines",
			content:  "line1\nline2\nline3",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "multiple lines with trailing newline",
			content:  "line1\nline2\nline3\n",
			expected: []string{"line1", "line2", "line3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitLines(tt.content)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d lines, got %d", len(tt.expected), len(result))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("Line %d: expected '%s', got '%s'", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestComputeLCS(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{
			name:     "identical",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "completely different",
			a:        []string{"a", "b", "c"},
			b:        []string{"x", "y", "z"},
			expected: []string{},
		},
		{
			name:     "partial match",
			a:        []string{"a", "b", "c", "d"},
			b:        []string{"a", "x", "c", "d"},
			expected: []string{"a", "c", "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeLCS(tt.a, tt.b)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected LCS length %d, got %d", len(tt.expected), len(result))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("LCS[%d]: expected '%s', got '%s'", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

func TestGroupIntoHunks(t *testing.T) {
	// Create a simple diff with changes
	diffLines := []DiffLine{
		{Type: DiffLineContext, Content: "line1", OldLine: 1, NewLine: 1},
		{Type: DiffLineContext, Content: "line2", OldLine: 2, NewLine: 2},
		{Type: DiffLineRemoved, Content: "old line", OldLine: 3, NewLine: 0},
		{Type: DiffLineAdded, Content: "new line", OldLine: 0, NewLine: 3},
		{Type: DiffLineContext, Content: "line4", OldLine: 4, NewLine: 4},
	}

	oldLines := []string{"line1", "line2", "old line", "line4"}
	newLines := []string{"line1", "line2", "new line", "line4"}

	hunks := groupIntoHunks(diffLines, oldLines, newLines)

	if len(hunks) == 0 {
		t.Error("Expected at least one hunk")
	}

	// Check that hunks contain the changes
	foundChange := false
	for _, hunk := range hunks {
		for _, line := range hunk.Lines {
			if line.Type == DiffLineRemoved || line.Type == DiffLineAdded {
				foundChange = true
				break
			}
		}
	}

	if !foundChange {
		t.Error("Hunks should contain changed lines")
	}
}
