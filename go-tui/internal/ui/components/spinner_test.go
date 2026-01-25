// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// SPINNER TESTS
// =============================================================================

func TestNewSpinner(t *testing.T) {
	s := NewSpinner()

	// Default style is now SpinnerLine (ASCII-compatible)
	if s.style != SpinnerLine {
		t.Errorf("NewSpinner() style = %v, want %v", s.style, SpinnerLine)
	}

	if s.message != "Loading" {
		t.Errorf("NewSpinner() message = %q, want %q", s.message, "Loading")
	}

	if !s.showTimer {
		t.Error("NewSpinner() showTimer should be true")
	}

	if s.isActive {
		t.Error("NewSpinner() should not be active initially")
	}
}

func TestNewSpinnerWithStyle(t *testing.T) {
	tests := []struct {
		name  string
		style SpinnerStyle
	}{
		{"Braille", SpinnerBraille},
		{"Dots", SpinnerDots},
		{"Line", SpinnerLine},
		{"Pulse", SpinnerPulse},
		{"Block", SpinnerBlock},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSpinnerWithStyle(tc.style)
			if s.style != tc.style {
				t.Errorf("NewSpinnerWithStyle(%v) style = %v, want %v", tc.style, s.style, tc.style)
			}
		})
	}
}

func TestNewThinkingSpinner(t *testing.T) {
	s := NewThinkingSpinner()

	if s.message != "Thinking" {
		t.Errorf("NewThinkingSpinner() message = %q, want %q", s.message, "Thinking")
	}

	if !s.showTimer {
		t.Error("NewThinkingSpinner() showTimer should be true")
	}
}

func TestNewLoadingSpinner(t *testing.T) {
	s := NewLoadingSpinner()

	if s.message != "Loading model" {
		t.Errorf("NewLoadingSpinner() message = %q, want %q", s.message, "Loading model")
	}

	if s.showTimer {
		t.Error("NewLoadingSpinner() showTimer should be false")
	}
}

func TestSpinnerSetStyle(t *testing.T) {
	s := NewSpinner()

	styles := []SpinnerStyle{
		SpinnerBraille,
		SpinnerDots,
		SpinnerLine,
		SpinnerPulse,
		SpinnerBlock,
	}

	for _, style := range styles {
		s.SetStyle(style)
		if s.style != style {
			t.Errorf("SetStyle(%v) did not set style correctly", style)
		}
	}
}

func TestSpinnerSetMessage(t *testing.T) {
	s := NewSpinner()
	msg := "Custom message"
	s.SetMessage(msg)

	if s.message != msg {
		t.Errorf("SetMessage(%q) message = %q, want %q", msg, s.message, msg)
	}
}

func TestSpinnerSetDetail(t *testing.T) {
	s := NewSpinner()
	detail := "Processing..."
	s.SetDetail(detail)

	if s.detail != detail {
		t.Errorf("SetDetail(%q) detail = %q, want %q", detail, s.detail, detail)
	}
}

func TestSpinnerSetShowTimer(t *testing.T) {
	s := NewSpinner()

	s.SetShowTimer(false)
	if s.showTimer {
		t.Error("SetShowTimer(false) did not disable timer")
	}

	s.SetShowTimer(true)
	if !s.showTimer {
		t.Error("SetShowTimer(true) did not enable timer")
	}
}

func TestSpinnerStartStop(t *testing.T) {
	s := NewSpinner()

	// Should not be active initially
	if s.IsActive() {
		t.Error("Spinner should not be active initially")
	}

	// Start spinner
	cmd := s.Start()
	if !s.IsActive() {
		t.Error("Start() should activate spinner")
	}
	if cmd == nil {
		t.Error("Start() should return a non-nil command")
	}

	// Check that start time was set
	if s.startTime.IsZero() {
		t.Error("Start() should set startTime")
	}

	// Stop spinner
	s.Stop()
	if s.IsActive() {
		t.Error("Stop() should deactivate spinner")
	}
}

func TestSpinnerGetElapsed(t *testing.T) {
	s := NewSpinner()

	// Before start, elapsed should be 0
	if s.GetElapsed() != 0 {
		t.Error("GetElapsed() should return 0 before Start()")
	}

	// After start, elapsed should be > 0
	s.Start()
	time.Sleep(10 * time.Millisecond)
	elapsed := s.GetElapsed()
	if elapsed == 0 {
		t.Error("GetElapsed() should return non-zero after Start()")
	}
}

func TestSpinnerInit(t *testing.T) {
	s := NewSpinner()
	cmd := s.Init()
	if cmd != nil {
		t.Error("Init() should return nil")
	}
}

func TestSpinnerUpdate(t *testing.T) {
	s := NewSpinner()

	// Update when inactive should return nil command
	updated, cmd := s.Update(tea.KeyMsg{})
	if cmd != nil {
		t.Error("Update() should return nil command when inactive")
	}

	// Start spinner
	s.Start()

	// Update when active should process messages
	updated, cmd = s.Update(tea.KeyMsg{})
	if updated.isActive != s.isActive {
		t.Error("Update() should maintain active state")
	}
}

func TestSpinnerView(t *testing.T) {
	s := NewSpinner()

	// View when inactive should return empty string
	view := s.View()
	if view != "" {
		t.Errorf("View() when inactive = %q, want empty string", view)
	}

	// Start spinner
	s.Start()

	// View when active should return non-empty string
	view = s.View()
	if view == "" {
		t.Error("View() when active should return non-empty string")
	}

	// View should contain message
	if !strings.Contains(view, s.message) {
		t.Errorf("View() = %q, should contain message %q", view, s.message)
	}
}

func TestSpinnerViewWithDetail(t *testing.T) {
	s := NewSpinner()
	s.SetDetail("Processing files...")
	s.Start()

	view := s.View()
	if !strings.Contains(view, s.detail) {
		t.Errorf("View() = %q, should contain detail %q", view, s.detail)
	}
}

// =============================================================================
// THINKING INDICATOR TESTS
// =============================================================================

func TestNewThinkingIndicator(t *testing.T) {
	ti := NewThinkingIndicator()

	if ti.spinner.message != "Thinking" {
		t.Errorf("NewThinkingIndicator() message = %q, want %q", ti.spinner.message, "Thinking")
	}
}

func TestThinkingIndicatorStartStop(t *testing.T) {
	ti := NewThinkingIndicator()

	// Should not be active initially
	if ti.IsActive() {
		t.Error("ThinkingIndicator should not be active initially")
	}

	// Start
	cmd := ti.Start()
	if !ti.IsActive() {
		t.Error("Start() should activate ThinkingIndicator")
	}
	if cmd == nil {
		t.Error("Start() should return a non-nil command")
	}

	// Stop
	ti.Stop()
	if ti.IsActive() {
		t.Error("Stop() should deactivate ThinkingIndicator")
	}
}

func TestThinkingIndicatorSetDetail(t *testing.T) {
	ti := NewThinkingIndicator()
	detail := "Analyzing code..."
	ti.SetDetail(detail)

	if ti.detail != detail {
		t.Errorf("SetDetail(%q) detail = %q, want %q", detail, ti.detail, detail)
	}

	if ti.spinner.detail != detail {
		t.Errorf("SetDetail(%q) did not update spinner detail", detail)
	}
}

func TestThinkingIndicatorGetElapsed(t *testing.T) {
	ti := NewThinkingIndicator()

	// Before start, elapsed should be 0
	if ti.GetElapsed() != 0 {
		t.Error("GetElapsed() should return 0 before Start()")
	}

	// After start, elapsed should be > 0
	ti.Start()
	time.Sleep(10 * time.Millisecond)
	elapsed := ti.GetElapsed()
	if elapsed == 0 {
		t.Error("GetElapsed() should return non-zero after Start()")
	}
}

func TestThinkingIndicatorUpdate(t *testing.T) {
	ti := NewThinkingIndicator()
	ti.Start()

	updated, cmd := ti.Update(tea.KeyMsg{})
	if updated.IsActive() != ti.IsActive() {
		t.Error("Update() should maintain active state")
	}
	_ = cmd // cmd may be nil or a spinner tick
}

func TestThinkingIndicatorView(t *testing.T) {
	ti := NewThinkingIndicator()

	// View when inactive should be empty
	view := ti.View()
	if view != "" {
		t.Error("View() when inactive should return empty string")
	}

	// Start and check view
	ti.Start()
	view = ti.View()
	if view == "" {
		t.Error("View() when active should return non-empty string")
	}
}

// =============================================================================
// MODEL LOADING SPINNER TESTS
// =============================================================================

func TestNewModelLoadingSpinner(t *testing.T) {
	modelName := "qwen2.5-coder:14b"
	mls := NewModelLoadingSpinner(modelName)

	if mls.modelName != modelName {
		t.Errorf("NewModelLoadingSpinner(%q) modelName = %q, want %q", modelName, mls.modelName, modelName)
	}

	expectedMsg := "Loading " + modelName
	if mls.spinner.message != expectedMsg {
		t.Errorf("NewModelLoadingSpinner() message = %q, want %q", mls.spinner.message, expectedMsg)
	}

	if mls.spinner.showTimer {
		t.Error("NewModelLoadingSpinner() should not show timer")
	}
}

func TestModelLoadingSpinnerStartStop(t *testing.T) {
	mls := NewModelLoadingSpinner("test-model")

	// Start
	cmd := mls.Start()
	if !mls.spinner.IsActive() {
		t.Error("Start() should activate ModelLoadingSpinner")
	}
	if cmd == nil {
		t.Error("Start() should return a non-nil command")
	}

	// Stop
	mls.Stop()
	if mls.spinner.IsActive() {
		t.Error("Stop() should deactivate ModelLoadingSpinner")
	}
}

func TestModelLoadingSpinnerUpdate(t *testing.T) {
	mls := NewModelLoadingSpinner("test-model")
	mls.Start()

	updated, cmd := mls.Update(tea.KeyMsg{})
	if updated.spinner.IsActive() != mls.spinner.IsActive() {
		t.Error("Update() should maintain active state")
	}
	_ = cmd
}

func TestModelLoadingSpinnerView(t *testing.T) {
	mls := NewModelLoadingSpinner("test-model")

	// View when inactive should be empty
	view := mls.View()
	if view != "" {
		t.Error("View() when inactive should return empty string")
	}

	// Start and check view
	mls.Start()
	view = mls.View()
	if view == "" {
		t.Error("View() when active should return non-empty string")
	}

	// View should be a box (contains border characters or content)
	if len(view) < 10 {
		t.Error("View() should return a styled box with content")
	}
}

// =============================================================================
// INLINE SPINNER TESTS
// =============================================================================

func TestNewInlineSpinner(t *testing.T) {
	is := NewInlineSpinner()

	if is.active {
		t.Error("NewInlineSpinner() should not be active initially")
	}
}

func TestInlineSpinnerStartStop(t *testing.T) {
	is := NewInlineSpinner()

	// Start
	cmd := is.Start()
	if !is.active {
		t.Error("Start() should activate InlineSpinner")
	}
	if cmd == nil {
		t.Error("Start() should return a non-nil command")
	}

	// Stop
	is.Stop()
	if is.active {
		t.Error("Stop() should deactivate InlineSpinner")
	}
}

func TestInlineSpinnerUpdate(t *testing.T) {
	is := NewInlineSpinner()

	// Update when inactive should return nil command
	updated, cmd := is.Update(tea.KeyMsg{})
	if cmd != nil {
		t.Error("Update() should return nil command when inactive")
	}

	// Start and update
	is.Start()
	updated, cmd = is.Update(tea.KeyMsg{})
	if updated.active != is.active {
		t.Error("Update() should maintain active state")
	}
}

func TestInlineSpinnerView(t *testing.T) {
	is := NewInlineSpinner()

	// View when inactive should be empty
	view := is.View()
	if view != "" {
		t.Error("View() when inactive should return empty string")
	}

	// Start and check view
	is.Start()
	view = is.View()
	if view == "" {
		t.Error("View() when active should return non-empty string")
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestFormatElapsed(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"0 seconds", 0, "0s"},
		{"5 seconds", 5 * time.Second, "5s"},
		{"30 seconds", 30 * time.Second, "30s"},
		{"59 seconds", 59 * time.Second, "59s"},
		{"1 minute", 60 * time.Second, "1m 0s"},
		{"1 minute 30 seconds", 90 * time.Second, "1m 30s"},
		{"2 minutes 45 seconds", 165 * time.Second, "2m 45s"},
		{"10 minutes", 600 * time.Second, "10m 0s"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatElapsed(tc.duration)
			if got != tc.want {
				t.Errorf("formatElapsed(%v) = %q, want %q", tc.duration, got, tc.want)
			}
		})
	}
}

func TestFormatSpinnerInt(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{123, "123"},
		{-1, "-1"},
		{-123, "-123"},
		{-9223372036854775808, "-9223372036854775808"}, // MinInt64
	}

	for _, tc := range tests {
		got := formatSpinnerInt(tc.input)
		if got != tc.want {
			t.Errorf("formatSpinnerInt(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestSpinnerDoubleStart(t *testing.T) {
	s := NewSpinner()

	// First start
	cmd1 := s.Start()
	time1 := s.startTime

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Second start should update start time
	cmd2 := s.Start()
	time2 := s.startTime

	if time1 == time2 {
		t.Error("Double Start() should update start time")
	}

	if cmd1 == nil || cmd2 == nil {
		t.Error("Start() should always return a command")
	}
}

func TestSpinnerStopWhenNotActive(t *testing.T) {
	s := NewSpinner()

	// Stopping when not active should not panic
	s.Stop()

	if s.IsActive() {
		t.Error("Stop() should ensure spinner is not active")
	}
}

func TestSpinnerViewWithTimer(t *testing.T) {
	s := NewSpinner()
	s.SetShowTimer(true)
	s.Start()

	// Wait a bit for elapsed time
	time.Sleep(100 * time.Millisecond)

	view := s.View()
	if view == "" {
		t.Error("View() should return non-empty string")
	}

	// View should contain elapsed time indicator (parentheses for timer)
	if !strings.Contains(view, "(") || !strings.Contains(view, ")") {
		t.Error("View() with timer should contain elapsed time in parentheses")
	}
}

func TestSpinnerViewWithoutTimer(t *testing.T) {
	s := NewSpinner()
	s.SetShowTimer(false)
	s.Start()

	view := s.View()
	if view == "" {
		t.Error("View() should return non-empty string")
	}

	// View should NOT contain timer parentheses
	if strings.Contains(view, "(") && strings.Contains(view, ")") {
		t.Error("View() without timer should not contain elapsed time")
	}
}
