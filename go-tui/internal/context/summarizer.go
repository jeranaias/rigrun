// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package context provides conversation summarization capabilities.
package context

import (
	"context"
	"fmt"
	"strings"

	"github.com/jeranaias/rigrun-tui/internal/model"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
)

// =============================================================================
// SUMMARIZER INTERFACE
// =============================================================================

// Summarizer creates concise summaries of message groups.
// Implementations should extract key information like:
// - Code locations discussed
// - Decisions made
// - Errors encountered
// - Problems solved
type Summarizer interface {
	// Summarize creates a summary of the given messages
	Summarize(ctx context.Context, messages []*model.Message) (string, error)
}

// =============================================================================
// LLM SUMMARIZER
// =============================================================================

// LLMSummarizer uses an LLM to create intelligent summaries of conversations.
type LLMSummarizer struct {
	client *ollama.Client
	model  string
}

// SummarizerConfig holds configuration for the LLM summarizer.
type SummarizerConfig struct {
	// Model to use for summarization (default: fastest available)
	Model string

	// Client is the Ollama client (required)
	Client *ollama.Client
}

// NewLLMSummarizer creates a new LLM-based summarizer.
func NewLLMSummarizer(config *SummarizerConfig) *LLMSummarizer {
	if config == nil {
		config = &SummarizerConfig{}
	}

	// Use fast model for summarization by default
	model := config.Model
	if model == "" {
		model = "qwen2.5-coder:7b" // Use smaller/faster model for summaries
	}

	return &LLMSummarizer{
		client: config.Client,
		model:  model,
	}
}

// =============================================================================
// SUMMARIZATION METHODS
// =============================================================================

// Summarize creates a concise summary of the message history.
// The summary preserves key facts, decisions, and context needed for continuation.
func (s *LLMSummarizer) Summarize(ctx context.Context, messages []*model.Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	// Build prompt for summarization
	prompt := s.buildSummarizationPrompt(messages)

	// Create messages for the summarization request
	ollamaMessages := []ollama.Message{
		{
			Role:    "system",
			Content: summarizerSystemPrompt,
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// Call LLM for summarization (non-streaming for simplicity)
	resp, err := s.client.ChatWithOptions(ctx, s.model, ollamaMessages, &ollama.Options{
		Temperature: 0.3, // Lower temperature for more focused summaries
		NumPredict:  500, // Limit summary length
	})

	if err != nil {
		return "", fmt.Errorf("summarization failed: %w", err)
	}

	summary := strings.TrimSpace(resp.Message.Content)
	if summary == "" {
		return "", fmt.Errorf("received empty summary from LLM")
	}

	return summary, nil
}

// buildSummarizationPrompt constructs the prompt for summarization.
func (s *LLMSummarizer) buildSummarizationPrompt(messages []*model.Message) string {
	var sb strings.Builder

	sb.WriteString("Summarize the following conversation. Focus on:\n")
	sb.WriteString("- Files and code locations discussed\n")
	sb.WriteString("- Key decisions made\n")
	sb.WriteString("- Errors encountered and how they were resolved\n")
	sb.WriteString("- Important context for continuing the conversation\n\n")
	sb.WriteString("Conversation:\n")
	sb.WriteString("---\n\n")

	// Add each message
	for i, msg := range messages {
		// Add role indicator
		switch msg.Role {
		case model.RoleUser:
			sb.WriteString("User: ")
		case model.RoleAssistant:
			sb.WriteString("Assistant: ")
		case model.RoleSystem:
			sb.WriteString("System: ")
		default:
			continue // Skip tool messages
		}

		// Add content
		content := msg.GetDisplayContent()

		// Truncate very long messages to avoid overwhelming the summarizer
		if len(content) > 2000 {
			content = content[:2000] + "...[truncated]"
		}

		sb.WriteString(content)
		sb.WriteString("\n")

		// Add separator between messages (except last)
		if i < len(messages)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// summarizerSystemPrompt is the system prompt for the summarization model.
const summarizerSystemPrompt = `You are a conversation summarizer. Your task is to create concise, informative summaries of technical conversations.

Guidelines:
- Extract key facts: file paths, function names, error messages, decisions
- Keep the summary under 300 words
- Use bullet points for clarity
- Preserve technical details that would be needed to continue the conversation
- Focus on actionable information and context
- Omit pleasantries and repetitive content

Format your summary in a clear, structured way that someone could read and immediately understand the context of the conversation.`

// =============================================================================
// SIMPLE SUMMARIZER (NO LLM)
// =============================================================================

// SimpleSummarizer creates basic summaries without using an LLM.
// This is a fallback when LLM summarization is unavailable or fails.
type SimpleSummarizer struct{}

// NewSimpleSummarizer creates a new simple summarizer.
func NewSimpleSummarizer() *SimpleSummarizer {
	return &SimpleSummarizer{}
}

// Summarize creates a simple count-based summary.
func (s *SimpleSummarizer) Summarize(ctx context.Context, messages []*model.Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	// Count message types
	userCount := 0
	assistantCount := 0

	// Extract first and last user messages for context
	var firstUserMsg, lastUserMsg string

	for _, msg := range messages {
		switch msg.Role {
		case model.RoleUser:
			userCount++
			if firstUserMsg == "" {
				content := msg.GetDisplayContent()
				if len(content) > 100 {
					firstUserMsg = content[:100] + "..."
				} else {
					firstUserMsg = content
				}
			}
			// Always update last
			content := msg.GetDisplayContent()
			if len(content) > 100 {
				lastUserMsg = content[:100] + "..."
			} else {
				lastUserMsg = content
			}
		case model.RoleAssistant:
			assistantCount++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Previous conversation: %d messages (%d from user, %d from assistant)\n\n",
		len(messages), userCount, assistantCount))

	if firstUserMsg != "" {
		sb.WriteString("Started with: ")
		sb.WriteString(firstUserMsg)
		sb.WriteString("\n")
	}

	if lastUserMsg != "" && lastUserMsg != firstUserMsg {
		sb.WriteString("Last topic: ")
		sb.WriteString(lastUserMsg)
	}

	return sb.String(), nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// ExtractKeyTopics attempts to extract key topics from messages without LLM.
// This is a simple heuristic-based approach.
func ExtractKeyTopics(messages []*model.Message) []string {
	topics := make(map[string]bool)

	// Look for common patterns in user messages
	for _, msg := range messages {
		if msg.Role != model.RoleUser {
			continue
		}

		content := strings.ToLower(msg.GetDisplayContent())

		// Extract file references
		if strings.Contains(content, ".go") || strings.Contains(content, ".ts") ||
		   strings.Contains(content, ".py") || strings.Contains(content, ".js") {
			topics["code files"] = true
		}

		// Extract error-related topics
		if strings.Contains(content, "error") || strings.Contains(content, "bug") {
			topics["debugging"] = true
		}

		// Extract implementation topics
		if strings.Contains(content, "implement") || strings.Contains(content, "create") {
			topics["implementation"] = true
		}

		// Extract testing topics
		if strings.Contains(content, "test") {
			topics["testing"] = true
		}
	}

	result := make([]string, 0, len(topics))
	for topic := range topics {
		result = append(result, topic)
	}

	return result
}

// =============================================================================
// STREAMING SUMMARIZER
// =============================================================================

// StreamingSummarizer creates summaries incrementally.
// This is useful for showing progress during summarization of long conversations.
type StreamingSummarizer struct {
	client *ollama.Client
	model  string
}

// NewStreamingSummarizer creates a new streaming summarizer.
func NewStreamingSummarizer(config *SummarizerConfig) *StreamingSummarizer {
	if config == nil {
		config = &SummarizerConfig{}
	}

	model := config.Model
	if model == "" {
		model = "qwen2.5-coder:7b"
	}

	return &StreamingSummarizer{
		client: config.Client,
		model:  model,
	}
}

// SummarizeStream creates a summary with streaming callback.
// The callback is called with each chunk of the summary as it's generated.
func (s *StreamingSummarizer) SummarizeStream(ctx context.Context, messages []*model.Message, callback func(chunk string)) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	// Build prompt
	llmSummarizer := &LLMSummarizer{client: s.client, model: s.model}
	prompt := llmSummarizer.buildSummarizationPrompt(messages)

	// Create messages for the request
	ollamaMessages := []ollama.Message{
		{
			Role:    "system",
			Content: summarizerSystemPrompt,
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// Collect the summary
	var summary strings.Builder

	// Stream the response
	err := s.client.ChatStream(ctx, s.model, ollamaMessages, func(chunk ollama.StreamChunk) {
		if chunk.Content != "" {
			summary.WriteString(chunk.Content)
			if callback != nil {
				callback(chunk.Content)
			}
		}
	})

	if err != nil {
		return "", fmt.Errorf("streaming summarization failed: %w", err)
	}

	result := strings.TrimSpace(summary.String())
	if result == "" {
		return "", fmt.Errorf("received empty summary from LLM")
	}

	return result, nil
}
