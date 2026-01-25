// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"strings"
	"testing"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/telemetry"
)

func TestNewCostDashboard(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := telemetry.NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	dashboard := NewCostDashboard(tracker)
	if dashboard == nil {
		t.Fatal("dashboard is nil")
	}

	if dashboard.view != ViewSummary {
		t.Errorf("default view: got %v, want ViewSummary", dashboard.view)
	}
}

func TestCostDashboard_SetView(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := telemetry.NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	dashboard := NewCostDashboard(tracker)

	tests := []struct {
		name string
		view DashboardView
	}{
		{"summary", ViewSummary},
		{"history", ViewHistory},
		{"breakdown", ViewBreakdown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dashboard.SetView(tt.view)
			if dashboard.view != tt.view {
				t.Errorf("view: got %v, want %v", dashboard.view, tt.view)
			}
		})
	}
}

func TestCostDashboard_SetSize(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := telemetry.NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	dashboard := NewCostDashboard(tracker)
	dashboard.SetSize(100, 50)

	if dashboard.width != 100 {
		t.Errorf("width: got %d, want 100", dashboard.width)
	}
	if dashboard.height != 50 {
		t.Errorf("height: got %d, want 50", dashboard.height)
	}
}

func TestCostDashboard_RenderSummary(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := telemetry.NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	// Record some test queries
	tracker.RecordQuery("cache", 100, 300, 10*time.Millisecond, "test cache query")
	tracker.RecordQuery("local", 200, 600, 500*time.Millisecond, "test local query")
	tracker.RecordQuery("cloud", 500, 1500, 2*time.Second, "test cloud query")

	dashboard := NewCostDashboard(tracker)
	dashboard.SetView(ViewSummary)

	output := dashboard.View()

	// Check for expected content
	expectedStrings := []string{
		"Cost Dashboard",
		"Session ID",
		"Started",
		"Duration",
		"Cost Summary",
		"Total Cost",
		"Savings",
		"Efficiency",
		"Token Usage by Tier",
		"Cache",
		"Local",
		"Cloud",
		"Top 5 Most Expensive Queries",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing expected string: %s", expected)
		}
	}
}

func TestCostDashboard_RenderHistory(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := telemetry.NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	// Record and save some queries
	tracker.RecordQuery("cache", 100, 300, 10*time.Millisecond, "test")
	if err := tracker.SaveCurrentSession(); err != nil {
		t.Fatalf("SaveCurrentSession failed: %v", err)
	}

	dashboard := NewCostDashboard(tracker)
	dashboard.SetView(ViewHistory)

	output := dashboard.View()

	expectedStrings := []string{
		"Cost History",
		"Total Cost",
		"Total Saved",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing expected string: %s", expected)
		}
	}
}

func TestCostDashboard_RenderBreakdown(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := telemetry.NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	// Record queries across different tiers
	tracker.RecordQuery("cache", 100, 300, 10*time.Millisecond, "test1")
	tracker.RecordQuery("local", 200, 600, 500*time.Millisecond, "test2")
	tracker.RecordQuery("cloud", 500, 1500, 2*time.Second, "test3")

	if err := tracker.SaveCurrentSession(); err != nil {
		t.Fatalf("SaveCurrentSession failed: %v", err)
	}

	dashboard := NewCostDashboard(tracker)
	dashboard.SetView(ViewBreakdown)

	output := dashboard.View()

	expectedStrings := []string{
		"Cost Breakdown",
		"Last 30 Days",
		"Cache",
		"Local",
		"Cloud",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing expected string: %s", expected)
		}
	}
}

func TestCostDashboard_RenderBar(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := telemetry.NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	dashboard := NewCostDashboard(tracker)

	tests := []struct {
		name     string
		value    int
		maxWidth int
	}{
		{"zero", 0, 10},
		{"half", 5, 10},
		{"full", 10, 10},
		{"overflow", 15, 10},
		{"negative", -5, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := dashboard.renderBar(tt.value, tt.maxWidth, "green")
			// Just check that we got something back
			if bar == "" {
				t.Error("bar should not be empty")
			}
			// Check that it contains bar characters (ASCII: # for filled, - for empty)
			if !strings.Contains(bar, "#") && !strings.Contains(bar, "-") {
				t.Error("bar should contain bar characters")
			}
		})
	}
}

func TestCostDashboard_TierColor(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := telemetry.NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	dashboard := NewCostDashboard(tracker)

	tests := []struct {
		tier      string
		wantColor string
	}{
		{"cache", "2"},
		{"local", "12"},
		{"cloud", "11"},
		{"unknown", "7"},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			color := dashboard.tierColor(tt.tier)
			if color != tt.wantColor {
				t.Errorf("color: got %s, want %s", color, tt.wantColor)
			}
		})
	}
}

func TestCostDashboard_FormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{3*time.Hour + 15*time.Minute, "3h 15m"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatCostDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatCostDuration(%v): got %s, want %s", tt.duration, got, tt.want)
			}
		})
	}
}

func TestCostDashboard_EmptySession(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := telemetry.NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	dashboard := NewCostDashboard(tracker)

	// Should not panic with empty session
	output := dashboard.View()
	if output == "" {
		t.Error("output should not be empty")
	}

	// Check for "No tokens used" message
	if !strings.Contains(output, "No tokens used yet") {
		t.Error("should show 'No tokens used yet' message")
	}
}

func TestCostDashboard_LargeNumberOfQueries(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := telemetry.NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	// Record many queries
	for i := 0; i < 100; i++ {
		tracker.RecordQuery("cloud", 100, 300, time.Second, "test query")
	}

	dashboard := NewCostDashboard(tracker)
	output := dashboard.View()

	// Should still render without issues
	if output == "" {
		t.Error("output should not be empty")
	}

	// Should show top 5 queries only
	lines := strings.Split(output, "\n")
	queryLines := 0
	inTopQueries := false
	for _, line := range lines {
		if strings.Contains(line, "Top 5 Most Expensive Queries") {
			inTopQueries = true
			continue
		}
		if inTopQueries && strings.HasPrefix(strings.TrimSpace(line), "1.") {
			queryLines++
		}
		if inTopQueries && (strings.TrimSpace(line) == "" || strings.Contains(line, "===")) {
			break
		}
	}

	// Should show at most 5 queries in summary
	if queryLines > 5 {
		t.Errorf("should show at most 5 queries, got %d", queryLines)
	}
}

// stripANSI removes ANSI escape codes from a string for testing.
func stripANSI(s string) string {
	var result []rune
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result = append(result, r)
	}
	return string(result)
}
