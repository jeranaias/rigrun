// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package session provides session management with DoD compliance timeout.
package session

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// CONFIG TESTS
// =============================================================================

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Timeout != 15*time.Minute {
		t.Errorf("Default Timeout = %v, want 15m", cfg.Timeout)
	}
	if cfg.WarningBefore != 2*time.Minute {
		t.Errorf("Default WarningBefore = %v, want 2m", cfg.WarningBefore)
	}
	if !cfg.AutoSaveEnabled {
		t.Error("Default AutoSaveEnabled should be true")
	}
	if cfg.AutoSaveInterval != 30*time.Second {
		t.Errorf("Default AutoSaveInterval = %v, want 30s", cfg.AutoSaveInterval)
	}
}

// =============================================================================
// MANAGER CREATION TESTS
// =============================================================================

func TestNewManager(t *testing.T) {
	cfg := DefaultConfig()
	m := NewManager(cfg)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	// Check session ID format
	if !strings.HasPrefix(m.SessionID(), "sess_") {
		t.Errorf("SessionID should start with 'sess_', got %q", m.SessionID())
	}

	// Check times are set
	if m.StartTime().IsZero() {
		t.Error("StartTime should not be zero")
	}
}

// =============================================================================
// SESSION STATE TESTS
// =============================================================================

func TestManager_SessionID(t *testing.T) {
	m := NewManager(DefaultConfig())
	id1 := m.SessionID()
	id2 := m.SessionID()

	if id1 != id2 {
		t.Error("SessionID should be consistent")
	}
	if id1 == "" {
		t.Error("SessionID should not be empty")
	}
}

func TestManager_Duration(t *testing.T) {
	m := NewManager(DefaultConfig())
	time.Sleep(10 * time.Millisecond)

	duration := m.Duration()
	if duration < 10*time.Millisecond {
		t.Errorf("Duration should be at least 10ms, got %v", duration)
	}
}

func TestManager_IdleTime(t *testing.T) {
	m := NewManager(DefaultConfig())
	time.Sleep(10 * time.Millisecond)

	idle := m.IdleTime()
	if idle < 10*time.Millisecond {
		t.Errorf("IdleTime should be at least 10ms, got %v", idle)
	}

	// Record activity and check idle resets
	m.RecordActivity()
	idle = m.IdleTime()
	if idle > 5*time.Millisecond {
		t.Errorf("IdleTime should be near zero after RecordActivity, got %v", idle)
	}
}

func TestManager_RemainingTime(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout = 100 * time.Millisecond
	m := NewManager(cfg)

	remaining := m.RemainingTime()
	if remaining > 100*time.Millisecond || remaining < 90*time.Millisecond {
		t.Errorf("RemainingTime should be close to timeout, got %v", remaining)
	}

	// Wait for timeout
	time.Sleep(110 * time.Millisecond)
	remaining = m.RemainingTime()
	if remaining != 0 {
		t.Errorf("RemainingTime should be 0 after timeout, got %v", remaining)
	}
}

// =============================================================================
// ACTIVITY TRACKING TESTS
// =============================================================================

func TestManager_RecordActivity(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout = 50 * time.Millisecond
	cfg.WarningBefore = 20 * time.Millisecond
	m := NewManager(cfg)

	// Wait until warning threshold
	time.Sleep(35 * time.Millisecond)

	// Manually check that we're in warning zone
	if !m.ShouldShowWarning() {
		t.Log("Not yet in warning zone, waiting more...")
	}

	// Record activity should reset idle time
	m.RecordActivity()

	remaining := m.RemainingTime()
	if remaining < 40*time.Millisecond {
		t.Errorf("RemainingTime should be near timeout after RecordActivity, got %v", remaining)
	}
}

func TestManager_DirtyState(t *testing.T) {
	m := NewManager(DefaultConfig())

	if m.IsDirty() {
		t.Error("New session should not be dirty")
	}

	m.MarkDirty()
	if !m.IsDirty() {
		t.Error("Session should be dirty after MarkDirty")
	}

	m.MarkClean()
	if m.IsDirty() {
		t.Error("Session should not be dirty after MarkClean")
	}
}

// =============================================================================
// TIMEOUT TESTS
// =============================================================================

func TestManager_IsExpired(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout = 50 * time.Millisecond
	m := NewManager(cfg)

	if m.IsExpired() {
		t.Error("New session should not be expired")
	}

	time.Sleep(60 * time.Millisecond)

	if !m.IsExpired() {
		t.Error("Session should be expired after timeout")
	}
}

func TestManager_ShouldShowWarning(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout = 100 * time.Millisecond
	cfg.WarningBefore = 30 * time.Millisecond
	m := NewManager(cfg)

	// Should not show warning initially
	if m.ShouldShowWarning() {
		t.Error("Should not show warning initially")
	}

	// Wait until warning threshold (70ms)
	time.Sleep(75 * time.Millisecond)

	if !m.ShouldShowWarning() {
		t.Error("Should show warning after threshold")
	}

	// Calling again should return false (already shown)
	m.mu.Lock()
	m.warningShown = true
	m.mu.Unlock()

	if m.ShouldShowWarning() {
		t.Error("Should not show warning again after already shown")
	}
}

func TestManager_ShouldAutoSave(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoSaveEnabled = true
	cfg.AutoSaveInterval = 20 * time.Millisecond
	m := NewManager(cfg)

	// Not dirty - should not save
	if m.ShouldAutoSave() {
		t.Error("Should not auto-save when not dirty")
	}

	// Mark dirty
	m.MarkDirty()

	// Wait for interval
	time.Sleep(25 * time.Millisecond)

	if !m.ShouldAutoSave() {
		t.Error("Should auto-save when dirty and interval elapsed")
	}
}

// =============================================================================
// CALLBACK TESTS
// =============================================================================

func TestManager_TimeoutCallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout = 30 * time.Millisecond
	m := NewManager(cfg)

	called := false
	m.SetTimeoutCallback(func() {
		called = true
	})

	time.Sleep(40 * time.Millisecond)
	m.Check()

	if !called {
		t.Error("Timeout callback should have been called")
	}
}

func TestManager_WarningCallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout = 50 * time.Millisecond
	cfg.WarningBefore = 20 * time.Millisecond
	m := NewManager(cfg)

	called := false
	var remainingTime time.Duration
	m.SetWarningCallback(func(remaining time.Duration) {
		called = true
		remainingTime = remaining
	})

	// Wait until warning threshold
	time.Sleep(35 * time.Millisecond)
	m.Check()

	if !called {
		t.Error("Warning callback should have been called")
	}
	if remainingTime <= 0 {
		t.Error("Remaining time should be positive")
	}
}

func TestManager_AutoSaveCallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoSaveEnabled = true
	cfg.AutoSaveInterval = 20 * time.Millisecond
	m := NewManager(cfg)

	called := false
	m.SetAutoSaveCallback(func() error {
		called = true
		return nil
	})

	m.MarkDirty()
	time.Sleep(25 * time.Millisecond)
	m.Check()

	if !called {
		t.Error("AutoSave callback should have been called")
	}

	// Should be marked clean after successful save
	if m.IsDirty() {
		t.Error("Session should be clean after successful auto-save")
	}
}

// =============================================================================
// CONFIGURATION TESTS
// =============================================================================

func TestManager_SetTimeout(t *testing.T) {
	m := NewManager(DefaultConfig())

	m.SetTimeout(5 * time.Minute)

	// Verify by checking remaining time
	remaining := m.RemainingTime()
	if remaining > 5*time.Minute {
		t.Errorf("RemainingTime should be <= 5m after SetTimeout, got %v", remaining)
	}
}

func TestManager_SetWarningTime(t *testing.T) {
	m := NewManager(DefaultConfig())
	m.SetWarningTime(1 * time.Minute)
	// Just verify no panic - internal state
}

func TestManager_SetAutoSaveEnabled(t *testing.T) {
	m := NewManager(DefaultConfig())

	m.SetAutoSaveEnabled(false)
	m.MarkDirty()

	if m.ShouldAutoSave() {
		t.Error("Should not auto-save when disabled")
	}
}

func TestManager_SetAutoSaveInterval(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoSaveInterval = 1 * time.Hour
	m := NewManager(cfg)

	m.SetAutoSaveInterval(10 * time.Millisecond)
	m.MarkDirty()
	time.Sleep(15 * time.Millisecond)

	if !m.ShouldAutoSave() {
		t.Error("Should auto-save after new interval")
	}
}

// =============================================================================
// STATUS TESTS
// =============================================================================

func TestManager_GetStatus(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout = 100 * time.Millisecond
	m := NewManager(cfg)

	m.MarkDirty()
	time.Sleep(10 * time.Millisecond)

	status := m.GetStatus()

	if status.SessionID == "" {
		t.Error("Status.SessionID should not be empty")
	}
	if status.Duration < 10*time.Millisecond {
		t.Error("Status.Duration should be at least 10ms")
	}
	if status.IdleTime < 10*time.Millisecond {
		t.Error("Status.IdleTime should be at least 10ms")
	}
	if status.RemainingTime <= 0 || status.RemainingTime > 100*time.Millisecond {
		t.Error("Status.RemainingTime should be reasonable")
	}
	if !status.IsDirty {
		t.Error("Status.IsDirty should be true")
	}
	if status.IsExpired {
		t.Error("Status.IsExpired should be false")
	}
}

// =============================================================================
// FORMAT TESTS
// =============================================================================

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{5 * time.Minute, "5m"},
		{5*time.Minute + 30*time.Second, "5m 30s"},
	}

	for _, tc := range tests {
		got := FormatDuration(tc.input)
		if got != tc.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// =============================================================================
// CONCURRENCY TESTS
// =============================================================================

func TestManager_ConcurrentAccess(t *testing.T) {
	m := NewManager(DefaultConfig())

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = m.SessionID()
				_ = m.Duration()
				_ = m.IdleTime()
				_ = m.RemainingTime()
				_ = m.IsExpired()
				_ = m.IsDirty()
				m.RecordActivity()
				m.MarkDirty()
				m.MarkClean()
			}
		}()
	}
	wg.Wait()
}

// =============================================================================
// CHECK INTEGRATION TEST
// =============================================================================

func TestManager_Check_Integration(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Timeout = 100 * time.Millisecond
	cfg.WarningBefore = 30 * time.Millisecond
	cfg.AutoSaveEnabled = true
	cfg.AutoSaveInterval = 20 * time.Millisecond
	m := NewManager(cfg)

	warningCalled := false
	timeoutCalled := false
	autoSaveCalled := false

	m.SetWarningCallback(func(remaining time.Duration) {
		warningCalled = true
	})
	m.SetTimeoutCallback(func() {
		timeoutCalled = true
	})
	m.SetAutoSaveCallback(func() error {
		autoSaveCalled = true
		return nil
	})

	// Initial check - nothing should trigger
	result := m.Check()
	if !result {
		t.Error("Check should return true initially")
	}

	// Mark dirty and wait for auto-save
	m.MarkDirty()
	time.Sleep(25 * time.Millisecond)
	m.Check()
	if !autoSaveCalled {
		t.Error("Auto-save should have been called")
	}

	// Wait for warning threshold
	time.Sleep(50 * time.Millisecond)
	m.Check()
	if !warningCalled {
		t.Error("Warning should have been called")
	}

	// Wait for timeout
	time.Sleep(30 * time.Millisecond)
	result = m.Check()
	if result {
		t.Error("Check should return false after timeout")
	}
	if !timeoutCalled {
		t.Error("Timeout should have been called")
	}
}
