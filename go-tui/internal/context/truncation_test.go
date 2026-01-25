// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package context

import (
	"context"
	"testing"

	"github.com/jeranaias/rigrun-tui/internal/model"
)

// =============================================================================
// MOCK SUMMARIZER
// =============================================================================

// mockSummarizer is a test summarizer that returns predictable summaries.
type mockSummarizer struct {
	summary string
	err     error
}

func (m *mockSummarizer) Summarize(ctx context.Context, messages []*model.Message) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.summary, nil
}

// =============================================================================
// TRUNCATION TESTS
// =============================================================================

func TestConversationTruncator_BelowThreshold(t *testing.T) {
	// Create a conversation with fewer messages than threshold
	conv := model.NewConversation()
	conv.SystemPrompt = "You are a helpful assistant"

	// Create 25 exchanges (50 messages total, which is at the threshold)
	// Need to be below threshold, so create 20 exchanges (40 messages)
	for i := 0; i < 20; i++ {
		conv.AddUserMessage("Message " + string(rune(i+'0')))
		conv.AddAssistantMessage()
	}

	// Create truncator with threshold of 50
	config := &TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
		Summarizer:       &mockSummarizer{summary: "Test summary"},
	}
	truncator := NewConversationTruncator(config)

	// Truncate
	result, err := truncator.Truncate(context.Background(), conv)
	if err != nil {
		t.Fatalf("Truncate failed: %v", err)
	}

	// Should not be truncated (40 < 50)
	if result.WasTruncated {
		t.Error("Expected no truncation for conversation below threshold")
	}

	if len(result.RecentMessages) != 40 { // 20 user + 20 assistant
		t.Errorf("Expected 40 messages, got %d", len(result.RecentMessages))
	}

	if result.HasSummary() {
		t.Error("Expected no summary for conversation below threshold")
	}
}

func TestConversationTruncator_AboveThreshold(t *testing.T) {
	// Create a conversation with more messages than threshold
	conv := model.NewConversation()
	conv.SystemPrompt = "You are a helpful assistant"

	for i := 0; i < 60; i++ {
		conv.AddUserMessage("Message " + string(rune(i+'0')))
		conv.AddAssistantMessage()
	}

	// Create truncator
	config := &TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
		Summarizer:       &mockSummarizer{summary: "Test summary of old messages"},
	}
	truncator := NewConversationTruncator(config)

	// Truncate
	result, err := truncator.Truncate(context.Background(), conv)
	if err != nil {
		t.Fatalf("Truncate failed: %v", err)
	}

	// Should be truncated (120 > 50)
	if !result.WasTruncated {
		t.Error("Expected truncation for conversation above threshold")
	}

	// Should keep 20 most recent messages
	if len(result.RecentMessages) != 20 {
		t.Errorf("Expected 20 recent messages, got %d", len(result.RecentMessages))
	}

	// Should have a summary
	if !result.HasSummary() {
		t.Error("Expected summary for truncated conversation")
	}

	if result.Summary != "Test summary of old messages" {
		t.Errorf("Expected summary to be 'Test summary of old messages', got '%s'", result.Summary)
	}

	// Check summary range
	expectedStart := 0
	expectedEnd := 100 // 120 total - 20 kept = 100 summarized
	if result.SummaryRange[0] != expectedStart || result.SummaryRange[1] != expectedEnd {
		t.Errorf("Expected summary range [%d, %d], got [%d, %d]",
			expectedStart, expectedEnd, result.SummaryRange[0], result.SummaryRange[1])
	}
}

func TestConversationTruncator_PreservesSystemPrompt(t *testing.T) {
	conv := model.NewConversation()
	conv.SystemPrompt = "Custom system prompt"

	for i := 0; i < 60; i++ {
		conv.AddUserMessage("Message")
		conv.AddAssistantMessage()
	}

	config := &TruncatorConfig{
		MaxFullMessages:  10,
		SummaryThreshold: 50,
		Summarizer:       &mockSummarizer{summary: "Summary"},
	}
	truncator := NewConversationTruncator(config)

	result, err := truncator.Truncate(context.Background(), conv)
	if err != nil {
		t.Fatalf("Truncate failed: %v", err)
	}

	if result.SystemPrompt != "Custom system prompt" {
		t.Errorf("Expected system prompt to be preserved, got '%s'", result.SystemPrompt)
	}
}

func TestConversationTruncator_NoSummarizer(t *testing.T) {
	conv := model.NewConversation()

	for i := 0; i < 60; i++ {
		conv.AddUserMessage("Message")
		conv.AddAssistantMessage()
	}

	// Create truncator without summarizer
	config := &TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
		Summarizer:       nil, // No summarizer
	}
	truncator := NewConversationTruncator(config)

	result, err := truncator.Truncate(context.Background(), conv)
	if err != nil {
		t.Fatalf("Truncate failed: %v", err)
	}

	// Should still truncate, but use simple summary
	if !result.WasTruncated {
		t.Error("Expected truncation even without summarizer")
	}

	// Should have a fallback summary
	if !result.HasSummary() {
		t.Error("Expected fallback summary")
	}

	// Fallback summary should mention message count
	if result.Summary == "" {
		t.Error("Expected non-empty fallback summary")
	}
}

func TestConversationTruncator_ShouldTruncate(t *testing.T) {
	config := &TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
	}
	truncator := NewConversationTruncator(config)

	tests := []struct {
		name      string
		msgCount  int
		shouldTruncate bool
	}{
		{"Below threshold", 30, false},
		{"At threshold", 50, false},
		{"Above threshold", 51, true},
		{"Well above threshold", 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv := model.NewConversation()
			for i := 0; i < tt.msgCount; i++ {
				conv.AddUserMessage("Message")
			}

			result := truncator.ShouldTruncate(conv)
			if result != tt.shouldTruncate {
				t.Errorf("ShouldTruncate() = %v, want %v", result, tt.shouldTruncate)
			}
		})
	}
}

func TestTruncateResult_Methods(t *testing.T) {
	result := &TruncateResult{
		SystemPrompt:   "System",
		Summary:        "Test summary",
		SummaryRange:   [2]int{0, 50},
		RecentMessages: make([]*model.Message, 20),
		WasTruncated:   true,
		TotalMessages:  70,
		TokensSaved:    1000,
	}

	// Test HasSummary
	if !result.HasSummary() {
		t.Error("HasSummary() should return true")
	}

	// Test SummaryInfo
	info := result.SummaryInfo()
	if info == "" {
		t.Error("SummaryInfo() should return non-empty string")
	}

	// Test GetFullMessageCount
	if result.GetFullMessageCount() != 20 {
		t.Errorf("GetFullMessageCount() = %d, want 20", result.GetFullMessageCount())
	}

	// Test GetSummarizedMessageCount
	if result.GetSummarizedMessageCount() != 50 {
		t.Errorf("GetSummarizedMessageCount() = %d, want 50", result.GetSummarizedMessageCount())
	}
}

func TestTruncateResult_ToOllamaMessages(t *testing.T) {
	// Create some test messages
	msg1 := model.NewUserMessage("Hello")
	msg2 := model.NewAssistantMessage()
	msg2.Content = "Hi there"
	msg2.IsStreaming = false

	result := &TruncateResult{
		SystemPrompt:   "You are helpful",
		Summary:        "Previous chat summary",
		RecentMessages: []*model.Message{msg1, msg2},
		WasTruncated:   true,
	}

	messages := result.ToOllamaMessages()

	// Should have: system prompt + summary + 2 messages = 4 total
	if len(messages) != 4 {
		t.Errorf("Expected 4 messages, got %d", len(messages))
	}

	// Check first is system prompt
	if messages[0].Role != "system" || messages[0].Content != "You are helpful" {
		t.Error("First message should be system prompt")
	}

	// Check second is summary
	if messages[1].Role != "system" {
		t.Error("Second message should be summary (as system message)")
	}

	// Check third is user message
	if messages[2].Role != "user" || messages[2].Content != "Hello" {
		t.Error("Third message should be user message")
	}

	// Check fourth is assistant message
	if messages[3].Role != "assistant" || messages[3].Content != "Hi there" {
		t.Error("Fourth message should be assistant message")
	}
}

func TestEstimateTruncationBenefit(t *testing.T) {
	conv := model.NewConversation()

	// Add messages (each ~25 chars = ~6 tokens)
	for i := 0; i < 60; i++ {
		conv.AddUserMessage("This is a test message!") // ~6 tokens
		conv.AddAssistantMessage()
	}

	config := &TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
	}
	truncator := NewConversationTruncator(config)

	tokensSaved, percentReduction := EstimateTruncationBenefit(conv, truncator)

	// Should save some tokens
	if tokensSaved <= 0 {
		t.Errorf("Expected positive tokens saved, got %d", tokensSaved)
	}

	// Should have a positive percentage
	if percentReduction <= 0 {
		t.Errorf("Expected positive percent reduction, got %f", percentReduction)
	}

	// For conversation below threshold
	conv2 := model.NewConversation()
	for i := 0; i < 30; i++ {
		conv2.AddUserMessage("Message")
	}

	tokensSaved2, percentReduction2 := EstimateTruncationBenefit(conv2, truncator)
	if tokensSaved2 != 0 || percentReduction2 != 0 {
		t.Error("Expected no benefit for conversation below threshold")
	}
}

func TestDefaultTruncatorConfig(t *testing.T) {
	config := DefaultTruncatorConfig()

	if config.MaxFullMessages != 20 {
		t.Errorf("Expected MaxFullMessages = 20, got %d", config.MaxFullMessages)
	}

	if config.SummaryThreshold != 50 {
		t.Errorf("Expected SummaryThreshold = 50, got %d", config.SummaryThreshold)
	}
}

func TestNewConversationTruncator_WithDefaults(t *testing.T) {
	// Test with nil config
	truncator := NewConversationTruncator(nil)
	if truncator.maxFullMessages != 20 {
		t.Errorf("Expected default maxFullMessages = 20, got %d", truncator.maxFullMessages)
	}
	if truncator.summaryThreshold != 50 {
		t.Errorf("Expected default summaryThreshold = 50, got %d", truncator.summaryThreshold)
	}

	// Test with zero values
	config := &TruncatorConfig{
		MaxFullMessages:  0,
		SummaryThreshold: 0,
	}
	truncator2 := NewConversationTruncator(config)
	if truncator2.maxFullMessages != 20 {
		t.Errorf("Expected default maxFullMessages = 20, got %d", truncator2.maxFullMessages)
	}
	if truncator2.summaryThreshold != 50 {
		t.Errorf("Expected default summaryThreshold = 50, got %d", truncator2.summaryThreshold)
	}
}
