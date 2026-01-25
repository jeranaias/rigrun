// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package model contains the data structures for conversations and messages.
package model

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

// =============================================================================
// ROLE TYPE
// =============================================================================

// Role represents the sender of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// String returns the string representation of the role.
func (r Role) String() string {
	return string(r)
}

// DisplayName returns a human-readable name for the role.
func (r Role) DisplayName() string {
	switch r {
	case RoleUser:
		return "You"
	case RoleAssistant:
		return "Assistant"
	case RoleSystem:
		return "System"
	case RoleTool:
		return "Tool"
	default:
		return string(r)
	}
}

// =============================================================================
// MESSAGE TYPE
// =============================================================================

// Message represents a single message in a conversation.
type Message struct {
	// Identity
	ID        string    `json:"id"`
	Role      Role      `json:"role"`
	Timestamp time.Time `json:"timestamp"`

	// Content
	Content string `json:"content"`

	// Streaming state (not persisted)
	// PERFORMANCE: strings.Builder avoids quadratic allocations during streaming
	IsStreaming     bool           `json:"-"`
	streamContent   strings.Builder `json:"-"` // Content being streamed, merged into Content when done

	// Token statistics
	TokenCount int `json:"token_count,omitempty"`

	// For tool messages
	ToolName   string `json:"tool_name,omitempty"`
	ToolInput  string `json:"tool_input,omitempty"`
	ToolResult string `json:"tool_result,omitempty"`
	IsSuccess  bool   `json:"is_success,omitempty"`

	// Performance metrics (for assistant messages)
	TTFT          time.Duration `json:"ttft_ns,omitempty"`           // Time to first token
	TotalDuration time.Duration `json:"total_duration_ns,omitempty"` // Total generation time
	TokensPerSec  float64       `json:"tokens_per_sec,omitempty"`    // Generation speed

	// Context information
	ContextMentions []string `json:"context_mentions,omitempty"` // @file:, @git, etc.
	ContextInfo     string   `json:"context_info,omitempty"`     // Summary of expanded context (e.g., "2 files, git, codebase")

	// Routing information (for assistant messages)
	RoutingTier  string  `json:"routing_tier,omitempty"`  // Tier used: Cache, Local, Cloud, Haiku, Sonnet, Opus
	RoutingCost  float64 `json:"routing_cost,omitempty"`  // Cost in cents for this message
	CacheHitType string  `json:"cache_hit_type,omitempty"` // Cache hit type: None, Exact, Semantic
}

// NewMessage creates a new message with a generated ID.
func NewMessage(role Role, content string) *Message {
	return &Message{
		ID:        generateID(),
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewUserMessage creates a new user message.
func NewUserMessage(content string) *Message {
	return NewMessage(RoleUser, content)
}

// NewAssistantMessage creates a new assistant message.
func NewAssistantMessage() *Message {
	return &Message{
		ID:          generateID(),
		Role:        RoleAssistant,
		Timestamp:   time.Now(),
		IsStreaming: true,
	}
}

// NewSystemMessage creates a new system message.
func NewSystemMessage(content string) *Message {
	return NewMessage(RoleSystem, content)
}

// NewToolMessage creates a new tool result message.
func NewToolMessage(toolName string, result string, success bool) *Message {
	msg := NewMessage(RoleTool, result)
	msg.ToolName = toolName
	msg.ToolResult = result
	msg.IsSuccess = success
	return msg
}

// =============================================================================
// MESSAGE METHODS
// =============================================================================

// AppendToken appends a token to a streaming message.
func (m *Message) AppendToken(token string) {
	if m.IsStreaming {
		m.streamContent.WriteString(token)
	}
}

// FinalizeStream completes streaming and sets statistics.
func (m *Message) FinalizeStream(stats *Statistics) {
	if !m.IsStreaming {
		return
	}

	m.Content = m.streamContent.String()
	m.streamContent.Reset()
	m.IsStreaming = false

	if stats != nil {
		m.TTFT = stats.TTFT
		m.TotalDuration = stats.TotalDuration
		m.TokenCount = stats.CompletionTokens
		m.TokensPerSec = stats.TokensPerSecond
	}
}

// GetDisplayContent returns the content to display (streaming or final).
func (m *Message) GetDisplayContent() string {
	if m.IsStreaming {
		return m.streamContent.String()
	}
	return m.Content
}

// Preview returns a truncated preview of the message content.
// Uses rune-based truncation to handle Unicode correctly.
func (m *Message) Preview(maxLen int) string {
	content := m.GetDisplayContent()
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[:maxLen-3]) + "..."
}

// IsEmpty returns true if the message has no content.
func (m *Message) IsEmpty() bool {
	return len(m.Content) == 0 && m.streamContent.Len() == 0
}

// EstimateTokens gives a rough estimate of token count.
// Uses the approximation of ~4 characters per token.
func (m *Message) EstimateTokens() int {
	content := m.GetDisplayContent()
	return (len(content) + 3) / 4
}

// FormatStats returns a formatted string of message statistics.
func (m *Message) FormatStats() string {
	if m.Role != RoleAssistant || m.TotalDuration == 0 {
		return ""
	}

	// Format: "2.5s | 128 tokens | 51 tok/s | TTFT 234ms"
	totalSec := m.TotalDuration.Seconds()
	ttftMs := m.TTFT.Milliseconds()

	return formatDuration(totalSec) + " | " +
		formatInt(m.TokenCount) + " tokens | " +
		formatFloat64(m.TokensPerSec) + " tok/s | " +
		"TTFT " + formatInt(int(ttftMs)) + "ms"
}

// =============================================================================
// STATISTICS TYPE
// =============================================================================

// Statistics holds timing and token count information for a generation.
type Statistics struct {
	// Timestamps
	StartTime      time.Time
	FirstTokenTime time.Time
	EndTime        time.Time

	// Token counts
	PromptTokens     int
	CompletionTokens int

	// Derived metrics (computed on Finalize)
	TTFT            time.Duration
	TotalDuration   time.Duration
	TokensPerSecond float64
}

// NewStatistics creates a new Statistics with the start time set.
func NewStatistics() *Statistics {
	return &Statistics{
		StartTime: time.Now(),
	}
}

// RecordFirstToken records when the first token was received.
func (s *Statistics) RecordFirstToken() {
	if s.FirstTokenTime.IsZero() {
		s.FirstTokenTime = time.Now()
		s.TTFT = s.FirstTokenTime.Sub(s.StartTime)
	}
}

// Finalize computes the final statistics.
func (s *Statistics) Finalize(tokenCount int) {
	s.EndTime = time.Now()
	s.CompletionTokens = tokenCount
	s.TotalDuration = s.EndTime.Sub(s.StartTime)

	if s.TotalDuration > 0 {
		s.TokensPerSecond = float64(tokenCount) / s.TotalDuration.Seconds()
	}
}

// Format returns a formatted string of the statistics.
func (s *Statistics) Format() string {
	totalSec := s.TotalDuration.Seconds()
	ttftMs := s.TTFT.Milliseconds()

	return formatDuration(totalSec) + " | " +
		formatInt(s.CompletionTokens) + " tokens | " +
		formatFloat64(s.TokensPerSecond) + " tok/s | " +
		"TTFT " + formatInt(int(ttftMs)) + "ms"
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// generateID creates a unique message ID.
func generateID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "msg_" + hex.EncodeToString(bytes)
}

// formatInt formats an integer without using fmt.
// Handles negative numbers and zero correctly.
//
// BUG FIX: Added math.MinInt64 handling to prevent overflow.
// On negation, -math.MinInt64 overflows because math.MaxInt64 < |math.MinInt64|.
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}

	// Handle math.MinInt64 edge case to prevent overflow on negation
	// math.MinInt64 = -9223372036854775808, and -(-9223372036854775808) overflows
	if n == -9223372036854775808 { // math.MinInt64 on 64-bit systems
		return "-9223372036854775808"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

// formatFloat64 formats a float with one decimal place.
// NOTE: This is a simplified formatter that truncates (does not round).
// It handles one decimal place only for display purposes.
//
// LIMITATIONS:
// - No rounding: 45.99 -> "45.9" (not "46.0")
// - Single decimal precision only: 45.95 -> "45.9"
// - Does not handle NaN, Inf, or values outside int range
// - For precise formatting, use fmt.Sprintf("%.1f", f) instead
//
// TODO: Consolidate with view.go's formatFloat64 into shared utility.
func formatFloat64(f float64) string {
	// Handle special cases
	if f != f { // NaN check
		return "NaN"
	}
	if f > 9223372036854775807 { // Larger than MaxInt64
		return "Inf"
	}
	if f < -9223372036854775808 { // Smaller than MinInt64
		return "-Inf"
	}

	whole := int(f)

	// Calculate fractional part - abs() handles negative numbers correctly
	absF := f
	if f < 0 {
		absF = -f
	}
	absWhole := whole
	if whole < 0 {
		absWhole = -whole
	}
	frac := int((absF - float64(absWhole)) * 10)

	return formatInt(whole) + "." + formatInt(frac)
}

// formatDuration formats seconds as a nice duration string.
func formatDuration(seconds float64) string {
	if seconds < 1 {
		ms := int(seconds * 1000)
		return formatInt(ms) + "ms"
	}
	return formatFloat64(seconds) + "s"
}
