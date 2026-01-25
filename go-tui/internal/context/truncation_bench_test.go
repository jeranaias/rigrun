// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package context

import (
	"context"
	"testing"

	"github.com/jeranaias/rigrun-tui/internal/model"
)

// =============================================================================
// BENCHMARKS
// =============================================================================

func BenchmarkTruncation_SmallConversation(b *testing.B) {
	conv := createTestConversation(30)
	truncator := NewConversationTruncator(&TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
		Summarizer:       NewSimpleSummarizer(),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = truncator.Truncate(context.Background(), conv)
	}
}

func BenchmarkTruncation_MediumConversation(b *testing.B) {
	conv := createTestConversation(100)
	truncator := NewConversationTruncator(&TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
		Summarizer:       NewSimpleSummarizer(),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = truncator.Truncate(context.Background(), conv)
	}
}

func BenchmarkTruncation_LargeConversation(b *testing.B) {
	conv := createTestConversation(500)
	truncator := NewConversationTruncator(&TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
		Summarizer:       NewSimpleSummarizer(),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = truncator.Truncate(context.Background(), conv)
	}
}

func BenchmarkEstimateTruncationBenefit(b *testing.B) {
	conv := createTestConversation(200)
	truncator := NewConversationTruncator(&TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EstimateTruncationBenefit(conv, truncator)
	}
}

func BenchmarkToOllamaMessages(b *testing.B) {
	conv := createTestConversation(100)
	truncator := NewConversationTruncator(&TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
		Summarizer:       NewSimpleSummarizer(),
	})
	result, _ := truncator.Truncate(context.Background(), conv)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = result.ToOllamaMessages()
	}
}

func BenchmarkShouldTruncate(b *testing.B) {
	conv := createTestConversation(100)
	truncator := NewConversationTruncator(&TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = truncator.ShouldTruncate(conv)
	}
}

// =============================================================================
// COMPARISON BENCHMARKS
// =============================================================================

// BenchmarkWithoutTruncation shows baseline performance without truncation
func BenchmarkWithoutTruncation(b *testing.B) {
	conv := createTestConversation(200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = conv.ToOllamaMessages()
	}
}

// BenchmarkWithTruncation shows performance with truncation
func BenchmarkWithTruncation(b *testing.B) {
	conv := createTestConversation(200)
	truncator := NewConversationTruncator(&TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
		Summarizer:       NewSimpleSummarizer(),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, _ := truncator.Truncate(context.Background(), conv)
		_ = result.ToOllamaMessages()
	}
}

// =============================================================================
// TOKEN ESTIMATION BENCHMARKS
// =============================================================================

func BenchmarkEstimateTokens_Original(b *testing.B) {
	conv := createTestConversation(200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = conv.EstimateTokens()
	}
}

func BenchmarkEstimateTokens_Truncated(b *testing.B) {
	conv := createTestConversation(200)
	truncator := NewConversationTruncator(&TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
		Summarizer:       NewSimpleSummarizer(),
	})
	result, _ := truncator.Truncate(context.Background(), conv)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Estimate tokens in truncated result
		tokens := 0
		for _, msg := range result.RecentMessages {
			tokens += msg.EstimateTokens()
		}
		tokens += (len(result.Summary) + 3) / 4
		_ = tokens
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func createTestConversation(messageCount int) *model.Conversation {
	conv := model.NewConversation()
	conv.SystemPrompt = "You are a helpful coding assistant"

	for i := 0; i < messageCount/2; i++ {
		conv.AddUserMessage("This is a test message with some content about code and programming")
		msg := conv.AddAssistantMessage()
		msg.Content = "This is a response with some helpful information about the code you mentioned"
		msg.IsStreaming = false
	}

	return conv
}

func createTestMessages(count int) []*model.Message {
	messages := make([]*model.Message, count)
	for i := 0; i < count; i++ {
		if i%2 == 0 {
			messages[i] = model.NewUserMessage("Test user message with some content")
		} else {
			msg := model.NewAssistantMessage()
			msg.Content = "Test assistant response with helpful information"
			msg.IsStreaming = false
			messages[i] = msg
		}
	}
	return messages
}

// =============================================================================
// MEMORY BENCHMARKS
// =============================================================================

func BenchmarkMemoryAllocation_Original(b *testing.B) {
	b.ReportAllocs()

	conv := createTestConversation(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = conv.ToOllamaMessages()
	}
}

func BenchmarkMemoryAllocation_Truncated(b *testing.B) {
	b.ReportAllocs()

	conv := createTestConversation(100)
	truncator := NewConversationTruncator(&TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
		Summarizer:       NewSimpleSummarizer(),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, _ := truncator.Truncate(context.Background(), conv)
		_ = result.ToOllamaMessages()
	}
}
