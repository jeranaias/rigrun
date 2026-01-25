// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCostTracker_NewCostTracker(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	if tracker == nil {
		t.Fatal("tracker is nil")
	}

	if tracker.currentID == "" {
		t.Error("currentID should not be empty")
	}

	session := tracker.GetCurrentSession()
	if session == nil {
		t.Fatal("current session is nil")
	}

	if session.ID != tracker.currentID {
		t.Errorf("session ID mismatch: got %s, want %s", session.ID, tracker.currentID)
	}
}

func TestCostTracker_RecordQuery(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	tests := []struct {
		name         string
		tier         string
		inputTokens  int
		outputTokens int
		duration     time.Duration
		prompt       string
	}{
		{
			name:         "cache query",
			tier:         "cache",
			inputTokens:  100,
			outputTokens: 300,
			duration:     10 * time.Millisecond,
			prompt:       "What is the capital of France?",
		},
		{
			name:         "local query",
			tier:         "local",
			inputTokens:  200,
			outputTokens: 600,
			duration:     500 * time.Millisecond,
			prompt:       "Explain how caching works",
		},
		{
			name:         "cloud query",
			tier:         "cloud",
			inputTokens:  500,
			outputTokens: 1500,
			duration:     2 * time.Second,
			prompt:       "Write a complex algorithm for data processing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker.RecordQuery(tt.tier, tt.inputTokens, tt.outputTokens, tt.duration, tt.prompt)
		})
	}

	session := tracker.GetCurrentSession()
	if session == nil {
		t.Fatal("session is nil")
	}

	// Check cache tokens
	if session.CacheTokens.Input != 100 {
		t.Errorf("cache input tokens: got %d, want 100", session.CacheTokens.Input)
	}
	if session.CacheTokens.Output != 300 {
		t.Errorf("cache output tokens: got %d, want 300", session.CacheTokens.Output)
	}

	// Check local tokens
	if session.LocalTokens.Input != 200 {
		t.Errorf("local input tokens: got %d, want 200", session.LocalTokens.Input)
	}
	if session.LocalTokens.Output != 600 {
		t.Errorf("local output tokens: got %d, want 600", session.LocalTokens.Output)
	}

	// Check cloud tokens
	if session.CloudTokens.Input != 500 {
		t.Errorf("cloud input tokens: got %d, want 500", session.CloudTokens.Input)
	}
	if session.CloudTokens.Output != 1500 {
		t.Errorf("cloud output tokens: got %d, want 1500", session.CloudTokens.Output)
	}

	// Check top queries
	if len(session.TopQueries) != 3 {
		t.Errorf("top queries count: got %d, want 3", len(session.TopQueries))
	}
}

func TestCostTracker_TopQueries(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	// Record 15 queries with varying costs
	for i := 0; i < 15; i++ {
		inputTokens := (i + 1) * 100
		outputTokens := (i + 1) * 300
		tracker.RecordQuery("cloud", inputTokens, outputTokens, time.Second, "test query")
	}

	session := tracker.GetCurrentSession()
	if len(session.TopQueries) > 10 {
		t.Errorf("top queries should be capped at 10, got %d", len(session.TopQueries))
	}

	// Verify queries are sorted by cost (descending)
	for i := 0; i < len(session.TopQueries)-1; i++ {
		if session.TopQueries[i].Cost < session.TopQueries[i+1].Cost {
			t.Errorf("queries not sorted by cost: query %d ($%.4f) < query %d ($%.4f)",
				i, session.TopQueries[i].Cost, i+1, session.TopQueries[i+1].Cost)
		}
	}
}

func TestCostTracker_GetHistory(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	// Record some queries
	tracker.RecordQuery("cache", 100, 300, time.Millisecond, "test1")
	tracker.RecordQuery("local", 200, 600, time.Second, "test2")

	// Save current session
	if err := tracker.SaveCurrentSession(); err != nil {
		t.Fatalf("SaveCurrentSession failed: %v", err)
	}

	// Get history
	now := time.Now()
	history := tracker.GetHistory(now.AddDate(0, 0, -1), now.AddDate(0, 0, 1))

	if len(history) != 1 {
		t.Errorf("history count: got %d, want 1", len(history))
	}

	if len(history) > 0 {
		session := history[0]
		if session.ID != tracker.currentID {
			t.Errorf("session ID mismatch: got %s, want %s", session.ID, tracker.currentID)
		}
	}
}

func TestCostTracker_GetTrends(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	// Record some queries
	tracker.RecordQuery("cache", 100, 300, time.Millisecond, "test1")
	tracker.RecordQuery("local", 200, 600, time.Second, "test2")
	tracker.RecordQuery("cloud", 500, 1500, 2*time.Second, "test3")

	// Save session
	if err := tracker.SaveCurrentSession(); err != nil {
		t.Fatalf("SaveCurrentSession failed: %v", err)
	}

	trends := tracker.GetTrends(7)
	if trends == nil {
		t.Fatal("trends is nil")
	}

	if trends.Days != 7 {
		t.Errorf("trends days: got %d, want 7", trends.Days)
	}

	if trends.TotalCost <= 0 {
		t.Error("total cost should be greater than 0")
	}

	if trends.TotalSaved <= 0 {
		t.Error("total saved should be greater than 0")
	}

	if len(trends.TierBreakdown) != 3 {
		t.Errorf("tier breakdown count: got %d, want 3", len(trends.TierBreakdown))
	}
}

func TestCostTracker_EndSession(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	oldID := tracker.currentID

	// Record a query
	tracker.RecordQuery("cache", 100, 300, time.Millisecond, "test")

	// End session
	if err := tracker.EndSession(); err != nil {
		t.Fatalf("EndSession failed: %v", err)
	}

	// Check new session created
	if tracker.currentID == oldID {
		t.Error("session ID should change after EndSession")
	}

	// Check old session saved
	session, err := tracker.storage.Load(oldID)
	if err != nil {
		t.Fatalf("failed to load saved session: %v", err)
	}

	if session.ID != oldID {
		t.Errorf("loaded session ID: got %s, want %s", session.ID, oldID)
	}

	if session.EndTime.IsZero() {
		t.Error("session end time should be set")
	}
}

func TestCostTracker_PromptTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	longPrompt := string(make([]byte, 200))
	for i := range longPrompt {
		longPrompt = longPrompt[:i] + "x" + longPrompt[i+1:]
	}

	tracker.RecordQuery("cache", 100, 300, time.Millisecond, longPrompt)

	session := tracker.GetCurrentSession()
	if len(session.TopQueries) == 0 {
		t.Fatal("no queries recorded")
	}

	recordedPrompt := session.TopQueries[0].Prompt
	if len(recordedPrompt) > 103 { // 100 chars + "..."
		t.Errorf("prompt not truncated: length %d", len(recordedPrompt))
	}
}

func TestCostTracker_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewCostTracker(tmpDir)
	if err != nil {
		t.Fatalf("NewCostTracker failed: %v", err)
	}

	// Simulate concurrent query recording
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				tracker.RecordQuery("cache", 10, 30, time.Millisecond, "concurrent test")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	session := tracker.GetCurrentSession()
	totalQueries := len(session.TopQueries)

	// Should have at least some queries (capped at 10 for top queries)
	if totalQueries == 0 {
		t.Error("no queries recorded from concurrent access")
	}
}

func TestCostStorage_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewCostStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewCostStorage failed: %v", err)
	}

	session := &SessionCost{
		ID:        "test-session",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(time.Hour),
		CacheTokens: TokenCount{
			Input:  100,
			Output: 300,
		},
		TotalCost: 0.05,
		Savings:   0.10,
		TopQueries: []QueryCost{
			{
				Timestamp:    time.Now(),
				Prompt:       "test query",
				Tier:         "cache",
				InputTokens:  100,
				OutputTokens: 300,
				Cost:         0.0,
				Duration:     10 * time.Millisecond,
			},
		},
	}

	// Save
	if err := storage.Save(session); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := storage.Load(session.ID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != session.ID {
		t.Errorf("ID mismatch: got %s, want %s", loaded.ID, session.ID)
	}

	if loaded.TotalCost != session.TotalCost {
		t.Errorf("TotalCost mismatch: got %.4f, want %.4f", loaded.TotalCost, session.TotalCost)
	}

	if len(loaded.TopQueries) != len(session.TopQueries) {
		t.Errorf("TopQueries count mismatch: got %d, want %d", len(loaded.TopQueries), len(session.TopQueries))
	}
}

func TestCostStorage_List(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewCostStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewCostStorage failed: %v", err)
	}

	// Create test sessions
	now := time.Now()
	sessions := []*SessionCost{
		{ID: now.AddDate(0, 0, -2).Format("20060102-150405"), StartTime: now.AddDate(0, 0, -2)},
		{ID: now.AddDate(0, 0, -1).Format("20060102-150405"), StartTime: now.AddDate(0, 0, -1)},
		{ID: now.Format("20060102-150405"), StartTime: now},
	}

	for _, s := range sessions {
		s.TopQueries = make([]QueryCost, 0)
		if err := storage.Save(s); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// List all
	from := now.AddDate(0, 0, -7)
	to := now.AddDate(0, 0, 1)
	ids, err := storage.List(from, to)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("List count: got %d, want 3", len(ids))
	}
}

func TestCostStorage_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewCostStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewCostStorage failed: %v", err)
	}

	session := &SessionCost{
		ID:         "delete-test",
		StartTime:  time.Now(),
		TopQueries: make([]QueryCost, 0),
	}

	// Save
	if err := storage.Save(session); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Delete
	if err := storage.Delete(session.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Try to load (should fail)
	_, err = storage.Load(session.ID)
	if err == nil {
		t.Error("Load should fail after Delete")
	}
}

func TestCostStorage_DefaultDirectory(t *testing.T) {
	storage, err := NewCostStorage("")
	if err != nil {
		t.Fatalf("NewCostStorage with empty dir failed: %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	expectedDir := filepath.Join(homeDir, ".rigrun", "costs")
	if storage.dir != expectedDir {
		t.Errorf("default dir: got %s, want %s", storage.dir, expectedDir)
	}
}
