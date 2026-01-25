// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package context provides example integration of truncation with conversations.
// This file demonstrates how to use the truncation feature in practice.
package context

import (
	"context"
	"fmt"

	"github.com/jeranaias/rigrun-tui/internal/model"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
)

// =============================================================================
// EXAMPLE: BASIC TRUNCATION
// =============================================================================

// ExampleBasicTruncation shows how to truncate a conversation.
func ExampleBasicTruncation(conv *model.Conversation) {
	// Create a simple summarizer (no LLM required)
	summarizer := NewSimpleSummarizer()

	// Create truncator
	config := &TruncatorConfig{
		MaxFullMessages:  20,  // Keep 20 recent messages
		SummaryThreshold: 50,  // Trigger after 50 messages
		Summarizer:       summarizer,
	}
	truncator := NewConversationTruncator(config)

	// Check if truncation is needed
	if truncator.ShouldTruncate(conv) {
		result, err := truncator.Truncate(context.Background(), conv)
		if err != nil {
			fmt.Printf("Truncation error: %v\n", err)
			return
		}

		fmt.Printf("Truncation results:\n")
		fmt.Printf("  - Total messages: %d\n", result.TotalMessages)
		fmt.Printf("  - Summarized: %d messages\n", result.GetSummarizedMessageCount())
		fmt.Printf("  - Kept in full: %d messages\n", result.GetFullMessageCount())
		fmt.Printf("  - Tokens saved: ~%d\n", result.TokensSaved)
		fmt.Printf("  - Summary: %s\n", result.Summary)
	}
}

// =============================================================================
// EXAMPLE: LLM-BASED SUMMARIZATION
// =============================================================================

// ExampleLLMSummarization shows how to use LLM-based summarization.
func ExampleLLMSummarization(conv *model.Conversation, client *ollama.Client) error {
	// Create LLM summarizer
	summarizerConfig := &SummarizerConfig{
		Model:  "qwen2.5-coder:7b", // Use fast model for summaries
		Client: client,
	}
	summarizer := NewLLMSummarizer(summarizerConfig)

	// Create truncator with LLM summarizer
	truncatorConfig := &TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
		Summarizer:       summarizer,
	}
	truncator := NewConversationTruncator(truncatorConfig)

	// Truncate conversation
	result, err := truncator.Truncate(context.Background(), conv)
	if err != nil {
		return fmt.Errorf("truncation failed: %w", err)
	}

	// Use the truncated result for next LLM request
	if result.WasTruncated {
		fmt.Printf("Conversation truncated: %s\n", result.SummaryInfo())
	}

	return nil
}

// =============================================================================
// EXAMPLE: STREAMING SUMMARIZATION WITH PROGRESS
// =============================================================================

// ExampleStreamingSummarization shows how to use streaming summarization.
func ExampleStreamingSummarization(conv *model.Conversation, client *ollama.Client) error {
	// Create streaming summarizer
	config := &SummarizerConfig{
		Model:  "qwen2.5-coder:7b",
		Client: client,
	}
	summarizer := NewStreamingSummarizer(config)

	// Get messages to summarize (older messages)
	if len(conv.Messages) <= 50 {
		fmt.Println("No truncation needed")
		return nil
	}

	oldMessages := conv.Messages[:len(conv.Messages)-20]

	// Stream the summary with progress updates
	var summaryChunks []string
	summary, err := summarizer.SummarizeStream(context.Background(), oldMessages, func(chunk string) {
		summaryChunks = append(summaryChunks, chunk)
		// Update UI with streaming progress
		fmt.Print(chunk)
	})

	if err != nil {
		return fmt.Errorf("streaming summarization failed: %w", err)
	}

	fmt.Printf("\n\nFinal summary (%d chars):\n%s\n", len(summary), summary)
	return nil
}

// =============================================================================
// EXAMPLE: INTEGRATION WITH CONVERSATION MODEL
// =============================================================================

// TruncatedConversationManager manages conversation truncation lifecycle.
type TruncatedConversationManager struct {
	conversation  *model.Conversation
	truncator     *ConversationTruncator
	lastTruncated *TruncateResult
}

// NewTruncatedConversationManager creates a new manager.
func NewTruncatedConversationManager(conv *model.Conversation, truncator *ConversationTruncator) *TruncatedConversationManager {
	return &TruncatedConversationManager{
		conversation: conv,
		truncator:    truncator,
	}
}

// GetMessagesForLLM returns messages optimized for LLM context window.
// This automatically handles truncation when needed.
func (m *TruncatedConversationManager) GetMessagesForLLM(ctx context.Context) ([]ollama.Message, error) {
	// Check if truncation is needed
	if !m.truncator.ShouldTruncate(m.conversation) {
		// No truncation needed, use original conversion
		return m.conversation.ToOllamaMessages(), nil
	}

	// Truncate conversation
	result, err := m.truncator.Truncate(ctx, m.conversation)
	if err != nil {
		return nil, fmt.Errorf("failed to truncate conversation: %w", err)
	}

	// Store truncation result for reference
	m.lastTruncated = result

	// Convert truncated result to Ollama messages
	messages := result.ToOllamaMessages()

	// Convert to ollama.Message type
	ollamaMessages := make([]ollama.Message, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = ollama.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	return ollamaMessages, nil
}

// GetTruncationInfo returns information about the last truncation.
func (m *TruncatedConversationManager) GetTruncationInfo() string {
	if m.lastTruncated == nil || !m.lastTruncated.WasTruncated {
		return ""
	}
	return m.lastTruncated.SummaryInfo()
}

// IsTruncated returns true if the conversation is currently truncated.
func (m *TruncatedConversationManager) IsTruncated() bool {
	return m.lastTruncated != nil && m.lastTruncated.WasTruncated
}

// GetSummary returns the summary of truncated messages.
func (m *TruncatedConversationManager) GetSummary() string {
	if m.lastTruncated == nil {
		return ""
	}
	return m.lastTruncated.Summary
}

// =============================================================================
// EXAMPLE: FALLBACK HANDLING
// =============================================================================

// TruncateWithFallback attempts LLM summarization, falls back to simple on error.
func TruncateWithFallback(ctx context.Context, conv *model.Conversation, client *ollama.Client) (*TruncateResult, error) {
	// Try LLM summarization first
	llmSummarizer := NewLLMSummarizer(&SummarizerConfig{
		Model:  "qwen2.5-coder:7b",
		Client: client,
	})

	truncator := NewConversationTruncator(&TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
		Summarizer:       llmSummarizer,
	})

	result, err := truncator.Truncate(ctx, conv)
	if err != nil {
		// LLM summarization failed, fall back to simple summarizer
		fmt.Println("LLM summarization failed, using simple summarizer")

		simpleSummarizer := NewSimpleSummarizer()
		truncator.summarizer = simpleSummarizer

		result, err = truncator.Truncate(ctx, conv)
		if err != nil {
			return nil, fmt.Errorf("both LLM and simple summarization failed: %w", err)
		}
	}

	return result, nil
}

// =============================================================================
// EXAMPLE: PERFORMANCE MONITORING
// =============================================================================

// TruncationMetrics tracks truncation performance.
type TruncationMetrics struct {
	TotalTruncations   int
	TotalTokensSaved   int
	AverageTokensSaved int
	LargestSummary     int
}

// TrackTruncation adds truncation metrics.
func (m *TruncationMetrics) TrackTruncation(result *TruncateResult) {
	if !result.WasTruncated {
		return
	}

	m.TotalTruncations++
	m.TotalTokensSaved += result.TokensSaved

	if m.TotalTruncations > 0 {
		m.AverageTokensSaved = m.TotalTokensSaved / m.TotalTruncations
	}

	summaryTokens := (len(result.Summary) + 3) / 4
	if summaryTokens > m.LargestSummary {
		m.LargestSummary = summaryTokens
	}
}

// Report returns a formatted metrics report.
func (m *TruncationMetrics) Report() string {
	return fmt.Sprintf(
		"Truncation Metrics:\n"+
			"  Total truncations: %d\n"+
			"  Total tokens saved: %d\n"+
			"  Average tokens saved: %d\n"+
			"  Largest summary: %d tokens\n",
		m.TotalTruncations,
		m.TotalTokensSaved,
		m.AverageTokensSaved,
		m.LargestSummary,
	)
}

// =============================================================================
// EXAMPLE: SMART TRUNCATION STRATEGY
// =============================================================================

// SmartTruncationStrategy adjusts truncation based on conversation characteristics.
type SmartTruncationStrategy struct {
	baseConfig *TruncatorConfig
}

// NewSmartTruncationStrategy creates a new smart strategy.
func NewSmartTruncationStrategy(summarizer Summarizer) *SmartTruncationStrategy {
	return &SmartTruncationStrategy{
		baseConfig: &TruncatorConfig{
			MaxFullMessages:  20,
			SummaryThreshold: 50,
			Summarizer:       summarizer,
		},
	}
}

// GetTruncator returns a truncator optimized for the conversation.
func (s *SmartTruncationStrategy) GetTruncator(conv *model.Conversation) *ConversationTruncator {
	config := *s.baseConfig // Copy base config

	// Adjust based on conversation size
	messageCount := len(conv.Messages)

	if messageCount > 200 {
		// Very long conversation: keep fewer messages, more aggressive truncation
		config.MaxFullMessages = 15
		config.SummaryThreshold = 40
	} else if messageCount > 100 {
		// Long conversation: standard settings
		config.MaxFullMessages = 20
		config.SummaryThreshold = 50
	} else {
		// Normal conversation: keep more context
		config.MaxFullMessages = 25
		config.SummaryThreshold = 60
	}

	// Adjust based on context usage
	if conv.IsContextNearLimit() {
		// Near context limit: be more aggressive
		config.MaxFullMessages = max(10, config.MaxFullMessages-5)
		config.SummaryThreshold = max(30, config.SummaryThreshold-10)
	}

	return NewConversationTruncator(&config)
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
