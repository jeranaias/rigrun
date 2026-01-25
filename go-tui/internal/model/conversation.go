// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package model contains the data structures for conversations and messages.
package model

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/ollama"
)

// MaxMessages is the maximum number of messages to keep in conversation history.
// When exceeded, old messages are pruned to prevent unbounded memory growth.
const MaxMessages = 1000

// =============================================================================
// CONVERSATION TYPE
// =============================================================================

// Conversation holds a complete chat conversation with history and metadata.
type Conversation struct {
	// Identity
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Messages
	Messages []*Message `json:"messages"`

	// Model configuration
	Model string `json:"model"`

	// Context tracking
	TokensUsed     int     `json:"tokens_used"`
	MaxTokens      int     `json:"max_tokens"`
	ContextPercent float64 `json:"-"` // Computed, not persisted

	// System prompt (optional)
	SystemPrompt string `json:"system_prompt,omitempty"`
}

// NewConversation creates a new conversation with a generated ID.
func NewConversation() *Conversation {
	return &Conversation{
		ID:        generateConversationID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  make([]*Message, 0),
		MaxTokens: 128000, // Default context window
	}
}

// NewConversationWithModel creates a new conversation with a specific model.
func NewConversationWithModel(model string) *Conversation {
	conv := NewConversation()
	conv.Model = model
	return conv
}

// =============================================================================
// MESSAGE MANAGEMENT
// =============================================================================

// AddMessage adds a message to the conversation.
func (c *Conversation) AddMessage(msg *Message) {
	c.Messages = append(c.Messages, msg)
	c.UpdatedAt = time.Now()
	c.updateTokenEstimate()
	c.updateTitle()
	c.pruneOldMessages()
}

// AddUserMessage creates and adds a user message.
func (c *Conversation) AddUserMessage(content string) *Message {
	msg := NewUserMessage(content)
	c.AddMessage(msg)
	return msg
}

// AddAssistantMessage creates and adds a streaming assistant message.
func (c *Conversation) AddAssistantMessage() *Message {
	msg := NewAssistantMessage()
	c.AddMessage(msg)
	return msg
}

// AddSystemMessage creates and adds a system message.
func (c *Conversation) AddSystemMessage(content string) *Message {
	msg := NewSystemMessage(content)
	c.AddMessage(msg)
	return msg
}

// AddToolMessage creates and adds a tool result message.
func (c *Conversation) AddToolMessage(toolName string, result string, success bool) *Message {
	msg := NewToolMessage(toolName, result, success)
	c.AddMessage(msg)
	return msg
}

// GetLastMessage returns the most recent message, or nil if empty.
func (c *Conversation) GetLastMessage() *Message {
	if len(c.Messages) == 0 {
		return nil
	}
	return c.Messages[len(c.Messages)-1]
}

// GetLastAssistantMessage returns the most recent assistant message.
func (c *Conversation) GetLastAssistantMessage() *Message {
	for i := len(c.Messages) - 1; i >= 0; i-- {
		if c.Messages[i].Role == RoleAssistant {
			return c.Messages[i]
		}
	}
	return nil
}

// GetLastUserMessage returns the most recent user message.
func (c *Conversation) GetLastUserMessage() *Message {
	for i := len(c.Messages) - 1; i >= 0; i-- {
		if c.Messages[i].Role == RoleUser {
			return c.Messages[i]
		}
	}
	return nil
}

// AppendToLast appends a token to the last (streaming) message.
func (c *Conversation) AppendToLast(token string) {
	last := c.GetLastMessage()
	if last != nil && last.IsStreaming {
		last.AppendToken(token)
	}
}

// FinalizeLast finalizes the last streaming message with statistics.
func (c *Conversation) FinalizeLast(stats *Statistics) {
	last := c.GetLastMessage()
	if last != nil && last.IsStreaming {
		last.FinalizeStream(stats)
		c.updateTokenEstimate()
	}
}

// ClearHistory removes all messages from the conversation.
func (c *Conversation) ClearHistory() {
	c.Messages = make([]*Message, 0)
	c.TokensUsed = 0
	c.ContextPercent = 0
	c.UpdatedAt = time.Now()
}

// RemoveMessage removes a message by ID.
func (c *Conversation) RemoveMessage(id string) bool {
	for i, msg := range c.Messages {
		if msg.ID == id {
			c.Messages = append(c.Messages[:i], c.Messages[i+1:]...)
			c.UpdatedAt = time.Now()
			c.updateTokenEstimate()
			return true
		}
	}
	return false
}

// GetMessageByID returns a message by its ID.
func (c *Conversation) GetMessageByID(id string) *Message {
	for _, msg := range c.Messages {
		if msg.ID == id {
			return msg
		}
	}
	return nil
}

// MessageCount returns the number of messages.
func (c *Conversation) MessageCount() int {
	return len(c.Messages)
}

// IsEmpty returns true if there are no messages.
func (c *Conversation) IsEmpty() bool {
	return len(c.Messages) == 0
}

// =============================================================================
// OLLAMA CONVERSION
// =============================================================================

// ToOllamaMessages converts the conversation to Ollama message format.
func (c *Conversation) ToOllamaMessages() []ollama.Message {
	messages := make([]ollama.Message, 0, len(c.Messages)+1)

	// Add system prompt if present
	if c.SystemPrompt != "" {
		messages = append(messages, ollama.NewSystemMessage(c.SystemPrompt))
	}

	// Add conversation messages
	for _, msg := range c.Messages {
		// Skip tool messages in the standard format
		// (they would need special handling for function calling)
		if msg.Role == RoleTool {
			continue
		}

		var ollamaRole string
		switch msg.Role {
		case RoleUser:
			ollamaRole = "user"
		case RoleAssistant:
			ollamaRole = "assistant"
		case RoleSystem:
			ollamaRole = "system"
		default:
			continue
		}

		content := msg.GetDisplayContent()
		if content != "" {
			messages = append(messages, ollama.Message{
				Role:    ollamaRole,
				Content: content,
			})
		}
	}

	return messages
}

// ToOllamaMessagesWithOverride converts the conversation to Ollama message format,
// but overrides the last user message content with the provided content.
// This is used for @ mention expansion where the UI shows the original message
// but the LLM receives the expanded context.
func (c *Conversation) ToOllamaMessagesWithOverride(lastUserContent string) []ollama.Message {
	messages := make([]ollama.Message, 0, len(c.Messages)+1)

	// Add system prompt if present
	if c.SystemPrompt != "" {
		messages = append(messages, ollama.NewSystemMessage(c.SystemPrompt))
	}

	// Find the index of the last user message
	lastUserIdx := -1
	for i := len(c.Messages) - 1; i >= 0; i-- {
		if c.Messages[i].Role == RoleUser {
			lastUserIdx = i
			break
		}
	}

	// Add conversation messages
	for i, msg := range c.Messages {
		// Skip tool messages in the standard format
		if msg.Role == RoleTool {
			continue
		}

		var ollamaRole string
		switch msg.Role {
		case RoleUser:
			ollamaRole = "user"
		case RoleAssistant:
			ollamaRole = "assistant"
		case RoleSystem:
			ollamaRole = "system"
		default:
			continue
		}

		// Use override content for the last user message
		var content string
		if i == lastUserIdx && lastUserContent != "" {
			content = lastUserContent
		} else {
			content = msg.GetDisplayContent()
		}

		if content != "" {
			messages = append(messages, ollama.Message{
				Role:    ollamaRole,
				Content: content,
			})
		}
	}

	return messages
}

// GetHistory returns the message history for display.
func (c *Conversation) GetHistory() []*Message {
	return c.Messages
}

// =============================================================================
// TOKEN TRACKING
// =============================================================================

// EstimateTokens estimates the total token count of the conversation.
func (c *Conversation) EstimateTokens() int {
	total := 0

	// System prompt tokens
	if c.SystemPrompt != "" {
		total += (len(c.SystemPrompt) + 3) / 4
	}

	// Message tokens
	for _, msg := range c.Messages {
		total += msg.EstimateTokens()
		// Add overhead for message structure (~4 tokens per message)
		total += 4
	}

	return total
}

// updateTokenEstimate updates the token usage and context percentage.
func (c *Conversation) updateTokenEstimate() {
	c.TokensUsed = c.EstimateTokens()
	if c.MaxTokens > 0 {
		c.ContextPercent = float64(c.TokensUsed) / float64(c.MaxTokens) * 100
	}
}

// GetContextPercent returns the percentage of context window used.
func (c *Conversation) GetContextPercent() float64 {
	return c.ContextPercent
}

// IsContextNearLimit returns true if context usage is above 75%.
func (c *Conversation) IsContextNearLimit() bool {
	return c.ContextPercent >= 75
}

// IsContextCritical returns true if context usage is above 90%.
func (c *Conversation) IsContextCritical() bool {
	return c.ContextPercent >= 90
}

// SetMaxTokens updates the maximum context window.
func (c *Conversation) SetMaxTokens(max int) {
	c.MaxTokens = max
	c.updateTokenEstimate()
}

// =============================================================================
// TITLE MANAGEMENT
// =============================================================================

// updateTitle auto-generates a title from the first user message if not set.
func (c *Conversation) updateTitle() {
	if c.Title != "" {
		return
	}

	// Find the first user message
	for _, msg := range c.Messages {
		if msg.Role == RoleUser {
			c.Title = msg.Preview(50)
			return
		}
	}
}

// SetTitle manually sets the conversation title.
func (c *Conversation) SetTitle(title string) {
	c.Title = title
	c.UpdatedAt = time.Now()
}

// GetTitle returns the conversation title or a default.
func (c *Conversation) GetTitle() string {
	if c.Title != "" {
		return c.Title
	}
	return "New Conversation"
}

// =============================================================================
// SERIALIZATION HELPERS
// =============================================================================

// Preview returns a short preview of the conversation.
func (c *Conversation) Preview() string {
	if len(c.Messages) == 0 {
		return "Empty conversation"
	}

	first := c.GetLastUserMessage()
	if first == nil {
		first = c.Messages[0]
	}

	return first.Preview(100)
}

// GetMeta returns metadata about the conversation.
func (c *Conversation) GetMeta() ConversationMeta {
	return ConversationMeta{
		ID:           c.ID,
		Title:        c.GetTitle(),
		Model:        c.Model,
		MessageCount: len(c.Messages),
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
		Preview:      c.Preview(),
	}
}

// ConversationMeta holds lightweight metadata for listing.
type ConversationMeta struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Model        string    `json:"model"`
	MessageCount int       `json:"message_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Preview      string    `json:"preview"`
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// generateConversationID creates a unique conversation ID.
func generateConversationID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "conv_" + hex.EncodeToString(bytes)
}

// Clone creates a deep copy of the conversation.
func (c *Conversation) Clone() *Conversation {
	clone := &Conversation{
		ID:           c.ID,
		Title:        c.Title,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
		Model:        c.Model,
		TokensUsed:   c.TokensUsed,
		MaxTokens:    c.MaxTokens,
		SystemPrompt: c.SystemPrompt,
		Messages:     make([]*Message, len(c.Messages)),
	}

	for i, msg := range c.Messages {
		// Messages are value types so this creates a copy
		msgCopy := *msg
		clone.Messages[i] = &msgCopy
	}

	return clone
}

// pruneOldMessages removes old messages when conversation history exceeds MaxMessages.
// Keeps the system prompt message (if any) and the most recent MaxMessages messages.
func (c *Conversation) pruneOldMessages() {
	if len(c.Messages) <= MaxMessages {
		return
	}

	// Find system prompt messages to preserve
	var systemMessages []*Message
	var otherMessages []*Message
	for _, msg := range c.Messages {
		if msg.Role == RoleSystem {
			systemMessages = append(systemMessages, msg)
		} else {
			otherMessages = append(otherMessages, msg)
		}
	}

	// If we have more than MaxMessages non-system messages, keep only the last MaxMessages
	if len(otherMessages) > MaxMessages {
		// Keep system messages + last MaxMessages non-system messages
		keepCount := MaxMessages
		startIdx := len(otherMessages) - keepCount
		otherMessages = otherMessages[startIdx:]
	}

	// Rebuild messages: system messages first, then conversation messages
	c.Messages = make([]*Message, 0, len(systemMessages)+len(otherMessages))
	c.Messages = append(c.Messages, systemMessages...)
	c.Messages = append(c.Messages, otherMessages...)
}
