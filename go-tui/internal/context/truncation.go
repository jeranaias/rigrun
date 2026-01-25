// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package context provides conversation truncation and summarization.
package context

import (
	"context"
	"fmt"

	"github.com/jeranaias/rigrun-tui/internal/model"
)

// =============================================================================
// TRUNCATION TYPES
// =============================================================================

// ConversationTruncator manages context window optimization through smart truncation.
// It keeps recent messages in full while summarizing older messages to maintain
// performance as conversations grow.
type ConversationTruncator struct {
	// maxFullMessages is the number of recent messages to keep in full
	maxFullMessages int

	// summaryThreshold is the message count that triggers summarization
	summaryThreshold int

	// summarizer creates summaries of message groups
	summarizer Summarizer
}

// TruncateResult holds the truncated conversation structure.
// This represents an optimized conversation that fits within context limits
// while preserving key information through summarization.
type TruncateResult struct {
	// SystemPrompt is the original system prompt (always preserved)
	SystemPrompt string

	// Summary is the LLM-generated summary of old messages
	Summary string

	// SummaryRange indicates which message indices were summarized [start, end)
	SummaryRange [2]int

	// RecentMessages contains the most recent N messages in full
	RecentMessages []*model.Message

	// WasTruncated indicates if truncation occurred
	WasTruncated bool

	// TotalMessages is the original message count before truncation
	TotalMessages int

	// TokensSaved is an estimate of tokens saved through summarization
	TokensSaved int
}

// =============================================================================
// TRUNCATOR CONFIGURATION
// =============================================================================

// TruncatorConfig holds configuration for the conversation truncator.
type TruncatorConfig struct {
	// MaxFullMessages is the number of recent messages to keep (default: 20)
	MaxFullMessages int

	// SummaryThreshold triggers summarization when exceeded (default: 50)
	SummaryThreshold int

	// Summarizer is the summarization implementation
	Summarizer Summarizer
}

// DefaultTruncatorConfig returns default configuration.
func DefaultTruncatorConfig() *TruncatorConfig {
	return &TruncatorConfig{
		MaxFullMessages:  20,
		SummaryThreshold: 50,
		Summarizer:       nil, // Must be set by caller
	}
}

// =============================================================================
// CONSTRUCTOR
// =============================================================================

// NewConversationTruncator creates a new conversation truncator.
func NewConversationTruncator(config *TruncatorConfig) *ConversationTruncator {
	if config == nil {
		config = DefaultTruncatorConfig()
	}

	// Apply defaults for zero values
	if config.MaxFullMessages <= 0 {
		config.MaxFullMessages = 20
	}
	if config.SummaryThreshold <= 0 {
		config.SummaryThreshold = 50
	}

	return &ConversationTruncator{
		maxFullMessages:  config.MaxFullMessages,
		summaryThreshold: config.SummaryThreshold,
		summarizer:       config.Summarizer,
	}
}

// =============================================================================
// TRUNCATION METHODS
// =============================================================================

// Truncate optimizes a conversation for the context window.
// It keeps the system prompt, recent messages in full, and summarizes old messages.
func (ct *ConversationTruncator) Truncate(ctx context.Context, conv *model.Conversation) (*TruncateResult, error) {
	result := &TruncateResult{
		SystemPrompt:  conv.SystemPrompt,
		TotalMessages: len(conv.Messages),
		WasTruncated:  false,
	}

	// If conversation is below threshold, no truncation needed
	if len(conv.Messages) <= ct.summaryThreshold {
		result.RecentMessages = conv.Messages
		return result, nil
	}

	// Determine split point
	splitIndex := len(conv.Messages) - ct.maxFullMessages
	if splitIndex < 0 {
		splitIndex = 0
	}

	// Keep recent messages
	result.RecentMessages = conv.Messages[splitIndex:]

	// Summarize old messages if any
	if splitIndex > 0 {
		oldMessages := conv.Messages[:splitIndex]

		// Generate summary if summarizer is available
		if ct.summarizer != nil {
			summary, err := ct.summarizer.Summarize(ctx, oldMessages)
			if err != nil {
				// If summarization fails, fall back to simple truncation
				result.Summary = fmt.Sprintf("Previous conversation (%d messages)", len(oldMessages))
			} else {
				result.Summary = summary
			}
		} else {
			// No summarizer available, use simple message count
			result.Summary = fmt.Sprintf("Previous conversation (%d messages)", len(oldMessages))
		}

		result.SummaryRange = [2]int{0, splitIndex}
		result.WasTruncated = true

		// Estimate tokens saved
		result.TokensSaved = ct.estimateTokensSaved(oldMessages, result.Summary)
	}

	return result, nil
}

// ShouldTruncate returns true if the conversation should be truncated.
func (ct *ConversationTruncator) ShouldTruncate(conv *model.Conversation) bool {
	return len(conv.Messages) > ct.summaryThreshold
}

// estimateTokensSaved calculates approximately how many tokens were saved.
func (ct *ConversationTruncator) estimateTokensSaved(messages []*model.Message, summary string) int {
	// Calculate original token count
	originalTokens := 0
	for _, msg := range messages {
		originalTokens += msg.EstimateTokens()
	}

	// Calculate summary token count
	summaryTokens := (len(summary) + 3) / 4

	// Return the difference (can be negative if summary is longer)
	saved := originalTokens - summaryTokens
	if saved < 0 {
		return 0
	}
	return saved
}

// =============================================================================
// RESULT METHODS
// =============================================================================

// HasSummary returns true if the result contains a summary.
func (tr *TruncateResult) HasSummary() bool {
	return tr.Summary != ""
}

// SummaryInfo returns a human-readable description of the summary.
func (tr *TruncateResult) SummaryInfo() string {
	if !tr.WasTruncated {
		return ""
	}

	messageCount := tr.SummaryRange[1] - tr.SummaryRange[0]
	return fmt.Sprintf("Summarized %d messages (saved ~%d tokens)", messageCount, tr.TokensSaved)
}

// GetFullMessageCount returns the number of full (non-summarized) messages.
func (tr *TruncateResult) GetFullMessageCount() int {
	return len(tr.RecentMessages)
}

// GetSummarizedMessageCount returns the number of summarized messages.
func (tr *TruncateResult) GetSummarizedMessageCount() int {
	if !tr.WasTruncated {
		return 0
	}
	return tr.SummaryRange[1] - tr.SummaryRange[0]
}

// =============================================================================
// CONVERSION TO OLLAMA FORMAT
// =============================================================================

// ToOllamaMessages converts the truncated result to Ollama message format.
// This includes the system prompt, summary (if present), and recent messages.
func (tr *TruncateResult) ToOllamaMessages() []struct {
	Role    string
	Content string
} {
	messages := make([]struct {
		Role    string
		Content string
	}, 0)

	// Add system prompt if present
	if tr.SystemPrompt != "" {
		messages = append(messages, struct {
			Role    string
			Content string
		}{
			Role:    "system",
			Content: tr.SystemPrompt,
		})
	}

	// Add summary as a system message if present
	if tr.HasSummary() {
		summaryContent := fmt.Sprintf("Previous conversation summary:\n\n%s\n\n---\n\nRecent conversation continues below:", tr.Summary)
		messages = append(messages, struct {
			Role    string
			Content string
		}{
			Role:    "system",
			Content: summaryContent,
		})
	}

	// Add recent messages
	for _, msg := range tr.RecentMessages {
		// Skip tool messages in the standard format
		if msg.Role == model.RoleTool {
			continue
		}

		var role string
		switch msg.Role {
		case model.RoleUser:
			role = "user"
		case model.RoleAssistant:
			role = "assistant"
		case model.RoleSystem:
			role = "system"
		default:
			continue
		}

		content := msg.GetDisplayContent()
		if content != "" {
			messages = append(messages, struct {
				Role    string
				Content string
			}{
				Role:    role,
				Content: content,
			})
		}
	}

	return messages
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// EstimateTruncationBenefit estimates the benefit of truncating a conversation.
// Returns the estimated tokens saved and the percentage reduction.
func EstimateTruncationBenefit(conv *model.Conversation, truncator *ConversationTruncator) (tokensSaved int, percentReduction float64) {
	if len(conv.Messages) <= truncator.summaryThreshold {
		return 0, 0.0
	}

	splitIndex := len(conv.Messages) - truncator.maxFullMessages
	if splitIndex <= 0 {
		return 0, 0.0
	}

	// Calculate current tokens
	currentTokens := conv.EstimateTokens()

	// Estimate tokens after truncation (rough estimate)
	oldMessages := conv.Messages[:splitIndex]
	oldTokens := 0
	for _, msg := range oldMessages {
		oldTokens += msg.EstimateTokens()
	}

	// Assume summary is ~10% of original size
	summaryTokens := oldTokens / 10
	tokensSaved = oldTokens - summaryTokens

	if currentTokens > 0 {
		percentReduction = float64(tokensSaved) / float64(currentTokens) * 100
	}

	return tokensSaved, percentReduction
}
