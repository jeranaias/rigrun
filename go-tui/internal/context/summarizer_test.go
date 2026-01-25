// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package context

import (
	"context"
	"strings"
	"testing"

	"github.com/jeranaias/rigrun-tui/internal/model"
)

// =============================================================================
// SIMPLE SUMMARIZER TESTS
// =============================================================================

func TestSimpleSummarizer_EmptyMessages(t *testing.T) {
	summarizer := NewSimpleSummarizer()
	summary, err := summarizer.Summarize(context.Background(), []*model.Message{})

	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	if summary != "" {
		t.Errorf("Expected empty summary for no messages, got '%s'", summary)
	}
}

func TestSimpleSummarizer_SingleMessage(t *testing.T) {
	summarizer := NewSimpleSummarizer()
	messages := []*model.Message{
		model.NewUserMessage("Hello, how are you?"),
	}

	summary, err := summarizer.Summarize(context.Background(), messages)
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	// Should mention 1 message
	if !strings.Contains(summary, "1 messages") {
		t.Errorf("Expected summary to mention 1 message, got '%s'", summary)
	}

	// Should include the message content
	if !strings.Contains(summary, "Hello") {
		t.Errorf("Expected summary to include message content, got '%s'", summary)
	}
}

func TestSimpleSummarizer_MultipleMessages(t *testing.T) {
	summarizer := NewSimpleSummarizer()

	messages := []*model.Message{
		model.NewUserMessage("How do I implement feature X?"),
		model.NewAssistantMessage(),
		model.NewUserMessage("Can you show me an example?"),
		model.NewAssistantMessage(),
	}

	// Set assistant message content
	messages[1].Content = "You can implement it like this..."
	messages[1].IsStreaming = false
	messages[3].Content = "Sure, here's an example..."
	messages[3].IsStreaming = false

	summary, err := summarizer.Summarize(context.Background(), messages)
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	// Should mention total count
	if !strings.Contains(summary, "4 messages") {
		t.Errorf("Expected summary to mention 4 messages, got '%s'", summary)
	}

	// Should mention user and assistant counts
	if !strings.Contains(summary, "2 from user") {
		t.Errorf("Expected summary to mention user count, got '%s'", summary)
	}

	if !strings.Contains(summary, "2 from assistant") {
		t.Errorf("Expected summary to mention assistant count, got '%s'", summary)
	}

	// Should include first message
	if !strings.Contains(summary, "feature X") {
		t.Errorf("Expected summary to include first message topic, got '%s'", summary)
	}
}

func TestSimpleSummarizer_LongMessages(t *testing.T) {
	summarizer := NewSimpleSummarizer()

	// Create a very long message (more than 100 chars)
	longMessage := strings.Repeat("This is a long message. ", 10)

	messages := []*model.Message{
		model.NewUserMessage(longMessage),
	}

	summary, err := summarizer.Summarize(context.Background(), messages)
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	// Should truncate long messages in summary
	if strings.Contains(summary, strings.Repeat("This is a long message. ", 10)) {
		t.Error("Expected summary to truncate long messages")
	}

	// Should have ellipsis for truncated content
	if !strings.Contains(summary, "...") {
		t.Error("Expected truncation indicator (...) in summary")
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestExtractKeyTopics(t *testing.T) {
	tests := []struct {
		name     string
		messages []*model.Message
		expected []string
	}{
		{
			name: "Code files",
			messages: []*model.Message{
				model.NewUserMessage("Can you help me with main.go?"),
			},
			expected: []string{"code files"},
		},
		{
			name: "Debugging",
			messages: []*model.Message{
				model.NewUserMessage("I'm getting an error in my code"),
			},
			expected: []string{"debugging"},
		},
		{
			name: "Implementation",
			messages: []*model.Message{
				model.NewUserMessage("How do I implement this feature?"),
			},
			expected: []string{"implementation"},
		},
		{
			name: "Testing",
			messages: []*model.Message{
				model.NewUserMessage("How should I test this function?"),
			},
			expected: []string{"testing"},
		},
		{
			name: "Multiple topics",
			messages: []*model.Message{
				model.NewUserMessage("I need to implement a test for main.go"),
			},
			expected: []string{"code files", "implementation", "testing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topics := ExtractKeyTopics(tt.messages)

			// Check that expected topics are present
			for _, expectedTopic := range tt.expected {
				found := false
				for _, topic := range topics {
					if topic == expectedTopic {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected topic '%s' not found in %v", expectedTopic, topics)
				}
			}
		})
	}
}

func TestExtractKeyTopics_IgnoresAssistantMessages(t *testing.T) {
	messages := []*model.Message{
		model.NewAssistantMessage(),
	}
	messages[0].Content = "I found a bug in your code"
	messages[0].IsStreaming = false

	topics := ExtractKeyTopics(messages)

	// Should not extract topics from assistant messages
	if len(topics) != 0 {
		t.Errorf("Expected no topics from assistant messages, got %v", topics)
	}
}

// =============================================================================
// LLM SUMMARIZER TESTS (mock-based)
// =============================================================================

func TestNewLLMSummarizer_DefaultModel(t *testing.T) {
	summarizer := NewLLMSummarizer(&SummarizerConfig{})

	if summarizer.model != "qwen2.5-coder:7b" {
		t.Errorf("Expected default model 'qwen2.5-coder:7b', got '%s'", summarizer.model)
	}
}

func TestNewLLMSummarizer_CustomModel(t *testing.T) {
	config := &SummarizerConfig{
		Model: "custom-model",
	}
	summarizer := NewLLMSummarizer(config)

	if summarizer.model != "custom-model" {
		t.Errorf("Expected custom model, got '%s'", summarizer.model)
	}
}

func TestLLMSummarizer_BuildPrompt(t *testing.T) {
	summarizer := NewLLMSummarizer(&SummarizerConfig{})

	messages := []*model.Message{
		model.NewUserMessage("How do I fix this bug?"),
		model.NewAssistantMessage(),
		model.NewUserMessage("Thanks!"),
	}
	messages[1].Content = "Try checking the logs"
	messages[1].IsStreaming = false

	prompt := summarizer.buildSummarizationPrompt(messages)

	// Should contain instructions
	if !strings.Contains(prompt, "Summarize") {
		t.Error("Prompt should contain summarization instructions")
	}

	// Should contain message content
	if !strings.Contains(prompt, "bug") {
		t.Error("Prompt should contain user message content")
	}

	if !strings.Contains(prompt, "logs") {
		t.Error("Prompt should contain assistant message content")
	}

	// Should have role indicators
	if !strings.Contains(prompt, "User:") {
		t.Error("Prompt should have User: indicator")
	}

	if !strings.Contains(prompt, "Assistant:") {
		t.Error("Prompt should have Assistant: indicator")
	}
}

func TestLLMSummarizer_BuildPrompt_TruncatesLongMessages(t *testing.T) {
	summarizer := NewLLMSummarizer(&SummarizerConfig{})

	// Create a very long message (> 2000 chars)
	longMessage := strings.Repeat("x", 3000)

	messages := []*model.Message{
		model.NewUserMessage(longMessage),
	}

	prompt := summarizer.buildSummarizationPrompt(messages)

	// Should truncate to around 2000 chars
	if strings.Contains(prompt, strings.Repeat("x", 3000)) {
		t.Error("Expected long messages to be truncated in prompt")
	}

	// Should have truncation indicator
	if !strings.Contains(prompt, "[truncated]") {
		t.Error("Expected truncation indicator in prompt")
	}
}

// =============================================================================
// STREAMING SUMMARIZER TESTS
// =============================================================================

func TestNewStreamingSummarizer_DefaultModel(t *testing.T) {
	summarizer := NewStreamingSummarizer(&SummarizerConfig{})

	if summarizer.model != "qwen2.5-coder:7b" {
		t.Errorf("Expected default model 'qwen2.5-coder:7b', got '%s'", summarizer.model)
	}
}

func TestNewStreamingSummarizer_CustomModel(t *testing.T) {
	config := &SummarizerConfig{
		Model: "custom-model",
	}
	summarizer := NewStreamingSummarizer(config)

	if summarizer.model != "custom-model" {
		t.Errorf("Expected custom model, got '%s'", summarizer.model)
	}
}

// =============================================================================
// INTEGRATION-STYLE TESTS
// =============================================================================

func TestSummarizerSystemPrompt(t *testing.T) {
	// Verify the system prompt is well-formed
	if summarizerSystemPrompt == "" {
		t.Error("Summarizer system prompt should not be empty")
	}

	// Should mention key aspects
	expectations := []string{
		"summarizer",
		"concise",
		"technical",
		"bullet",
	}

	lowerPrompt := strings.ToLower(summarizerSystemPrompt)
	for _, expected := range expectations {
		if !strings.Contains(lowerPrompt, expected) {
			t.Errorf("System prompt should mention '%s'", expected)
		}
	}
}

// =============================================================================
// BENCHMARKS
// =============================================================================

func BenchmarkSimpleSummarizer(b *testing.B) {
	summarizer := NewSimpleSummarizer()

	messages := make([]*model.Message, 50)
	for i := 0; i < 50; i++ {
		if i%2 == 0 {
			messages[i] = model.NewUserMessage("This is a test message")
		} else {
			msg := model.NewAssistantMessage()
			msg.Content = "This is a response"
			msg.IsStreaming = false
			messages[i] = msg
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = summarizer.Summarize(context.Background(), messages)
	}
}

func BenchmarkExtractKeyTopics(b *testing.B) {
	messages := make([]*model.Message, 50)
	for i := 0; i < 50; i++ {
		messages[i] = model.NewUserMessage("How do I implement a test for main.go to debug this error?")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExtractKeyTopics(messages)
	}
}
