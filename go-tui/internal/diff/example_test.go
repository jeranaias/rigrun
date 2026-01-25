// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package diff provides diff computation and formatting for file changes.
package diff_test

import (
	"fmt"

	"github.com/jeranaias/rigrun-tui/internal/diff"
)

func ExampleComputeDiff() {
	// Original file content
	oldContent := "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n"

	// Modified file content
	newContent := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}\n"

	// Compute the diff
	d := diff.ComputeDiff("main.go", oldContent, newContent)

	// Display summary
	fmt.Println(d.Summary())

	// Output:
	// Modified +3 -1
}

func ExampleFormatUnifiedDiff() {
	oldContent := "line1\nline2\nline3"
	newContent := "line1\nmodified\nline3"

	d := diff.ComputeDiff("file.txt", oldContent, newContent)

	// Generate unified diff format
	unified := diff.FormatUnifiedDiff(d)
	fmt.Println(unified)

	// Output:
	// --- a/file.txt
	// +++ b/file.txt
	// @@ -1,3 +1,3 @@
	//  line1
	// -line2
	// +modified
	//  line3
}

func ExampleDiff_Summary_newFile() {
	// New file (empty old content)
	d := diff.ComputeDiff("newfile.txt", "", "line1\nline2")

	fmt.Println(d.Summary())
	fmt.Println("File mode:", d.Stats.FileMode)

	// Output:
	// New file +2
	// File mode: new
}

func ExampleDiff_Summary_deletedFile() {
	// Deleted file (empty new content)
	d := diff.ComputeDiff("oldfile.txt", "line1\nline2", "")

	fmt.Println(d.Summary())
	fmt.Println("File mode:", d.Stats.FileMode)

	// Output:
	// File deleted -2
	// File mode: deleted
}

func ExampleDiffLineType_Prefix() {
	// Show diff line prefixes
	fmt.Println("Added:", diff.DiffLineAdded.Prefix())
	fmt.Println("Removed:", diff.DiffLineRemoved.Prefix())
	fmt.Println("Context:", diff.DiffLineContext.Prefix())

	// Output:
	// Added: +
	// Removed: -
	// Context:
}

func ExampleDiff_Hunks() {
	oldContent := "line1\nline2\nline3\nline4\nline5"
	newContent := "line1\nmodified2\nline3\nmodified4\nline5"

	d := diff.ComputeDiff("file.txt", oldContent, newContent)

	// Iterate through hunks
	for i, hunk := range d.Hunks {
		fmt.Printf("Hunk %d: @@ -%d,%d +%d,%d @@\n",
			i+1, hunk.OldStart, hunk.OldCount, hunk.NewStart, hunk.NewCount)
	}

	// Output will show the hunks created by grouping changes
}
