// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"strings"
	"testing"

	"github.com/jeranaias/rigrun-tui/internal/diff"
)

func TestNewDiffViewer(t *testing.T) {
	d := &diff.Diff{
		FilePath: "test.txt",
		Stats: diff.DiffStats{
			Additions: 1,
			Deletions: 1,
			FileMode:  "modified",
		},
	}

	viewer := NewDiffViewer(d)

	if viewer.diff != d {
		t.Error("Diff not set correctly")
	}

	if viewer.approved {
		t.Error("Should not be approved initially")
	}

	if viewer.rejected {
		t.Error("Should not be rejected initially")
	}

	if !viewer.showHelp {
		t.Error("Should show help by default")
	}
}

func TestDiffViewer_Approve(t *testing.T) {
	viewer := NewDiffViewer(&diff.Diff{})

	viewer.Approve()

	if !viewer.IsApproved() {
		t.Error("Should be approved")
	}

	if viewer.IsRejected() {
		t.Error("Should not be rejected")
	}
}

func TestDiffViewer_Reject(t *testing.T) {
	viewer := NewDiffViewer(&diff.Diff{})

	viewer.Reject()

	if viewer.IsApproved() {
		t.Error("Should not be approved")
	}

	if !viewer.IsRejected() {
		t.Error("Should be rejected")
	}
}

func TestDiffViewer_ApproveAfterReject(t *testing.T) {
	viewer := NewDiffViewer(&diff.Diff{})

	viewer.Reject()
	viewer.Approve()

	if !viewer.IsApproved() {
		t.Error("Should be approved")
	}

	if viewer.IsRejected() {
		t.Error("Should not be rejected after approving")
	}
}

func TestDiffViewer_SetSize(t *testing.T) {
	viewer := NewDiffViewer(&diff.Diff{})

	viewer.SetSize(100, 50)

	if viewer.width != 100 {
		t.Errorf("Expected width 100, got %d", viewer.width)
	}

	if viewer.height != 50 {
		t.Errorf("Expected height 50, got %d", viewer.height)
	}
}

func TestDiffViewer_ScrollUp(t *testing.T) {
	viewer := NewDiffViewer(&diff.Diff{})
	viewer.scrollPos = 10

	viewer.ScrollUp(5)

	if viewer.scrollPos != 5 {
		t.Errorf("Expected scroll position 5, got %d", viewer.scrollPos)
	}

	// Should not go negative
	viewer.ScrollUp(10)

	if viewer.scrollPos != 0 {
		t.Errorf("Expected scroll position 0, got %d", viewer.scrollPos)
	}
}

func TestDiffViewer_ScrollDown(t *testing.T) {
	viewer := NewDiffViewer(&diff.Diff{})

	viewer.ScrollDown(5)

	if viewer.scrollPos != 5 {
		t.Errorf("Expected scroll position 5, got %d", viewer.scrollPos)
	}
}

func TestDiffViewer_View_NoDiff(t *testing.T) {
	viewer := NewDiffViewer(nil)

	view := viewer.View()

	if !strings.Contains(view, "No diff available") {
		t.Error("Should show 'No diff available' when diff is nil")
	}
}

func TestDiffViewer_View_NewFile(t *testing.T) {
	d := diff.ComputeDiff("test.txt", "", "line1\nline2\nline3")
	viewer := NewDiffViewer(d)

	view := viewer.View()

	// Should contain file path
	if !strings.Contains(view, "test.txt") {
		t.Error("View should contain file path")
	}

	// Should show "New file" mode
	if !strings.Contains(view, "New file") {
		t.Error("View should indicate new file")
	}

	// Should show additions count
	if !strings.Contains(view, "+3") {
		t.Error("View should show +3 additions")
	}
}

func TestDiffViewer_View_Modified(t *testing.T) {
	oldContent := "line1\nline2\nline3"
	newContent := "line1\nmodified\nline3"

	d := diff.ComputeDiff("test.txt", oldContent, newContent)
	viewer := NewDiffViewer(d)

	view := viewer.View()

	// Should contain file path
	if !strings.Contains(view, "test.txt") {
		t.Error("View should contain file path")
	}

	// Should show modified mode
	if !strings.Contains(view, "Modified") {
		t.Error("View should indicate modified file")
	}

	// Should show stats
	if !strings.Contains(view, "+1") {
		t.Error("View should show +1 addition")
	}

	if !strings.Contains(view, "-1") {
		t.Error("View should show -1 deletion")
	}
}

func TestDiffViewer_View_Approved(t *testing.T) {
	d := diff.ComputeDiff("test.txt", "old", "new")
	viewer := NewDiffViewer(d)

	viewer.Approve()
	view := viewer.View()

	// Should show approved status
	if !strings.Contains(view, "approved") {
		t.Error("View should show approved status")
	}
}

func TestDiffViewer_View_Rejected(t *testing.T) {
	d := diff.ComputeDiff("test.txt", "old", "new")
	viewer := NewDiffViewer(d)

	viewer.Reject()
	view := viewer.View()

	// Should show rejected status
	if !strings.Contains(view, "rejected") {
		t.Error("View should show rejected status")
	}
}

func TestDiffViewer_View_Help(t *testing.T) {
	d := diff.ComputeDiff("test.txt", "old", "new")
	viewer := NewDiffViewer(d)

	view := viewer.View()

	// Should show help when not approved/rejected
	if !strings.Contains(view, "Approve") {
		t.Error("View should show approval help")
	}

	if !strings.Contains(view, "Reject") {
		t.Error("View should show rejection help")
	}
}

func TestMinDiffInt(t *testing.T) {
	tests := []struct {
		a        int
		b        int
		expected int
	}{
		{5, 10, 5},
		{10, 5, 5},
		{5, 5, 5},
		{-5, 10, -5},
		{-10, -5, -10},
	}

	for _, tt := range tests {
		result := minDiffInt(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("minDiffInt(%d, %d) = %d, expected %d", tt.a, tt.b, result, tt.expected)
		}
	}
}
