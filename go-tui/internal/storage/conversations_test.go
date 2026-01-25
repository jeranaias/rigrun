// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package storage provides conversation persistence for rigrun TUI.
package storage

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// CONVERSATION STORE TESTS
// =============================================================================

func TestNewConversationStoreWithDir(t *testing.T) {
	tempDir := t.TempDir()

	store, err := NewConversationStoreWithDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	if store.BaseDir != tempDir {
		t.Errorf("BaseDir = %q, want %q", store.BaseDir, tempDir)
	}
	if store.MaxConversations != 100 {
		t.Errorf("MaxConversations = %d, want 100", store.MaxConversations)
	}
}

func TestConversationStore_SaveAndLoad(t *testing.T) {
	store, err := NewConversationStoreWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Create a conversation
	conv := &StoredConversation{
		Model: "test-model",
		Messages: []StoredMessage{
			{ID: "msg1", Role: "user", Content: "Hello", Timestamp: time.Now()},
			{ID: "msg2", Role: "assistant", Content: "Hi there!", Timestamp: time.Now()},
		},
	}

	// Save
	id, err := store.Save(conv)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if id == "" {
		t.Error("Expected non-empty ID")
	}
	if !strings.HasPrefix(id, "conv_") {
		t.Errorf("ID should start with 'conv_', got %q", id)
	}

	// Load
	loaded, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != id {
		t.Errorf("Loaded ID = %q, want %q", loaded.ID, id)
	}
	if loaded.Model != "test-model" {
		t.Errorf("Loaded Model = %q, want %q", loaded.Model, "test-model")
	}
	if len(loaded.Messages) != 2 {
		t.Errorf("Loaded Messages count = %d, want 2", len(loaded.Messages))
	}
}

func TestConversationStore_LoadNotFound(t *testing.T) {
	store, err := NewConversationStoreWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	_, err = store.Load("nonexistent-id")
	if !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("Expected ErrConversationNotFound, got %v", err)
	}
}

func TestConversationStore_Delete(t *testing.T) {
	store, err := NewConversationStoreWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Save a conversation
	conv := &StoredConversation{
		Messages: []StoredMessage{
			{Role: "user", Content: "Test"},
		},
	}
	id, _ := store.Save(conv)

	// Delete it
	err = store.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	_, err = store.Load(id)
	if !errors.Is(err, ErrConversationNotFound) {
		t.Error("Conversation should not exist after delete")
	}
}

func TestConversationStore_DeleteNotFound(t *testing.T) {
	store, err := NewConversationStoreWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	err = store.Delete("nonexistent-id")
	if !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("Expected ErrConversationNotFound, got %v", err)
	}
}

func TestConversationStore_List(t *testing.T) {
	store, err := NewConversationStoreWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Empty list
	metas, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(metas) != 0 {
		t.Errorf("Expected empty list, got %d items", len(metas))
	}

	// Add conversations
	for i := 0; i < 3; i++ {
		conv := &StoredConversation{
			Messages: []StoredMessage{
				{Role: "user", Content: "Message " + string(rune('A'+i))},
			},
		}
		store.Save(conv)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// List again
	metas, err = store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(metas) != 3 {
		t.Errorf("Expected 3 items, got %d", len(metas))
	}

	// Verify sorted by most recent first
	for i := 0; i < len(metas)-1; i++ {
		if metas[i].UpdatedAt.Before(metas[i+1].UpdatedAt) {
			t.Error("List should be sorted by most recent first")
		}
	}
}

func TestConversationStore_LoadByIndex(t *testing.T) {
	store, err := NewConversationStoreWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Add conversations
	var lastID string
	for i := 0; i < 3; i++ {
		conv := &StoredConversation{
			Messages: []StoredMessage{
				{Role: "user", Content: "Message " + string(rune('A'+i))},
			},
		}
		lastID, _ = store.Save(conv)
		time.Sleep(10 * time.Millisecond)
	}

	// Load by index 0 (most recent)
	loaded, err := store.LoadByIndex(0)
	if err != nil {
		t.Fatalf("LoadByIndex failed: %v", err)
	}
	if loaded.ID != lastID {
		t.Errorf("LoadByIndex(0) should return most recent conversation")
	}

	// Invalid index
	_, err = store.LoadByIndex(100)
	if !errors.Is(err, ErrConversationNotFound) {
		t.Errorf("Expected ErrConversationNotFound for invalid index, got %v", err)
	}
}

func TestConversationStore_Search(t *testing.T) {
	store, err := NewConversationStoreWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Add conversations with different content
	store.Save(&StoredConversation{
		Summary: "About Rust programming",
		Messages: []StoredMessage{
			{Role: "user", Content: "Tell me about Rust"},
		},
	})
	store.Save(&StoredConversation{
		Summary: "About Go programming",
		Messages: []StoredMessage{
			{Role: "user", Content: "Tell me about Go"},
		},
	})

	// Search for "Rust"
	results, err := store.Search("rust")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'rust', got %d", len(results))
	}

	// Search for "programming"
	results, err = store.Search("programming")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'programming', got %d", len(results))
	}
}

func TestConversationStore_SearchMessages(t *testing.T) {
	store, err := NewConversationStoreWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Add conversations
	store.Save(&StoredConversation{
		Messages: []StoredMessage{
			{Role: "user", Content: "How do I implement a binary tree?"},
			{Role: "assistant", Content: "Here's how to implement a binary tree..."},
		},
	})
	store.Save(&StoredConversation{
		Messages: []StoredMessage{
			{Role: "user", Content: "What is a hash map?"},
		},
	})

	// Search message content
	results, err := store.SearchMessages("binary tree")
	if err != nil {
		t.Fatalf("SearchMessages failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

func TestConversationStore_Clear(t *testing.T) {
	store, err := NewConversationStoreWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Add conversations
	for i := 0; i < 3; i++ {
		store.Save(&StoredConversation{
			Messages: []StoredMessage{{Role: "user", Content: "Test"}},
		})
	}

	// Clear all
	err = store.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify empty
	metas, _ := store.List()
	if len(metas) != 0 {
		t.Errorf("Expected empty store after Clear, got %d items", len(metas))
	}
}

func TestConversationStore_EnforceLimit(t *testing.T) {
	store, err := NewConversationStoreWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	store.MaxConversations = 3

	// Add more than limit
	for i := 0; i < 5; i++ {
		store.Save(&StoredConversation{
			Messages: []StoredMessage{{Role: "user", Content: "Test " + string(rune('A'+i))}},
		})
		time.Sleep(10 * time.Millisecond)
	}

	// Verify limit enforced
	metas, _ := store.List()
	if len(metas) > 3 {
		t.Errorf("Expected at most 3 conversations, got %d", len(metas))
	}
}

// =============================================================================
// STORED CONVERSATION TESTS
// =============================================================================

func TestStoredConversation_GenerateSummary(t *testing.T) {
	store, _ := NewConversationStoreWithDir(t.TempDir())

	conv := &StoredConversation{
		Messages: []StoredMessage{
			{Role: "user", Content: "This is a very long message that should be truncated to fifty characters maximum"},
		},
	}

	id, _ := store.Save(conv)
	loaded, _ := store.Load(id)

	if len(loaded.Summary) > 50 {
		t.Errorf("Summary should be truncated to 50 chars, got %d", len(loaded.Summary))
	}
	if !strings.HasSuffix(loaded.Summary, "...") {
		t.Error("Truncated summary should end with '...'")
	}
}

func TestStoredConversation_ExportMarkdown(t *testing.T) {
	conv := &StoredConversation{
		ID:        "test-123",
		CreatedAt: time.Now(),
		Messages: []StoredMessage{
			{Role: "user", Content: "Hello", Timestamp: time.Now()},
			{Role: "assistant", Content: "Hi!", Timestamp: time.Now()},
		},
	}

	md := conv.ExportMarkdown()

	if !strings.Contains(md, "# Session test-123") {
		t.Error("Markdown should contain session ID header")
	}
	if !strings.Contains(md, "**User**") {
		t.Error("Markdown should contain User role")
	}
	if !strings.Contains(md, "**Assistant**") {
		t.Error("Markdown should contain Assistant role")
	}
}

func TestStoredConversation_ExportJSON(t *testing.T) {
	conv := &StoredConversation{
		ID:    "test-123",
		Model: "test-model",
	}

	data, err := conv.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	if !strings.Contains(string(data), `"id": "test-123"`) {
		t.Error("JSON should contain conversation ID")
	}
}

func TestStoredConversation_GetPreview(t *testing.T) {
	conv := &StoredConversation{
		Messages: []StoredMessage{
			{Role: "system", Content: "You are a helpful assistant"},
			{Role: "user", Content: "What is Go?"},
		},
	}

	preview := conv.GetPreview()
	if preview != "What is Go?" {
		t.Errorf("GetPreview should return first user message, got %q", preview)
	}
}

func TestStoredConversation_MessageCount(t *testing.T) {
	conv := &StoredConversation{
		Messages: []StoredMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
			{Role: "user", Content: "How are you?"},
		},
	}

	if conv.MessageCount() != 3 {
		t.Errorf("MessageCount() = %d, want 3", conv.MessageCount())
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"hi", 2, "hi"},
		{"hello", 3, "hel"},
		{"", 5, ""},
		{"test", 0, ""},
	}

	for _, tc := range tests {
		got := truncateString(tc.input, tc.maxLen)
		if got != tc.want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
		}
	}
}

func TestFormatPadded(t *testing.T) {
	tests := []struct {
		input string
		width int
		want  string
	}{
		{"hi", 5, "hi   "},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello world"},
		{"", 3, "   "},
	}

	for _, tc := range tests {
		got := formatPadded(tc.input, tc.width)
		if got != tc.want {
			t.Errorf("formatPadded(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
		}
	}
}

func TestFormatSessionList(t *testing.T) {
	// Empty list
	result := FormatSessionList(nil)
	if result != "No sessions found." {
		t.Errorf("Expected 'No sessions found.' for empty list")
	}

	// Non-empty list
	sessions := []ConversationMeta{
		{ID: "conv_123", CreatedAt: time.Now(), MessageCount: 5, Preview: "Hello"},
	}
	result = FormatSessionList(sessions)
	if !strings.Contains(result, "Sessions:") {
		t.Error("Result should contain 'Sessions:' header")
	}
	if !strings.Contains(result, "conv_123") {
		t.Error("Result should contain session ID")
	}
}

// =============================================================================
// ERROR TESTS
// =============================================================================

func TestConversationError_Is(t *testing.T) {
	err1 := &ConversationError{Message: "test error"}
	err2 := &ConversationError{Message: "test error"}
	err3 := &ConversationError{Message: "different error"}

	if !errors.Is(err1, err2) {
		t.Error("Same message errors should match")
	}
	if errors.Is(err1, err3) {
		t.Error("Different message errors should not match")
	}
}

// =============================================================================
// UNICODE TESTS
// =============================================================================

func TestConversationStore_UnicodeContent(t *testing.T) {
	store, err := NewConversationStoreWithDir(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	conv := &StoredConversation{
		Summary: "日本語のテスト",
		Messages: []StoredMessage{
			{Role: "user", Content: "こんにちは世界!"},
			{Role: "assistant", Content: "Hello! 你好! Bonjour!"},
		},
	}

	id, err := store.Save(conv)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Messages[0].Content != "こんにちは世界!" {
		t.Error("Unicode content should be preserved")
	}
}
