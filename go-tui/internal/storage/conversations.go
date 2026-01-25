// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package storage provides conversation persistence for rigrun TUI.
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// STORED CONVERSATION TYPE
// =============================================================================

// StoredConversation represents a persisted conversation.
type StoredConversation struct {
	// Identity
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Messages
	Messages []StoredMessage `json:"messages"`

	// Context tracking
	TokensUsed int      `json:"tokens_used,omitempty"`
	Mentions   []string `json:"mentions,omitempty"`
}

// StoredMessage represents a persisted message.
type StoredMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // "user", "assistant", "system", "tool"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`

	// Statistics (for assistant messages)
	TokenCount   int     `json:"token_count,omitempty"`
	DurationMs   int64   `json:"duration_ms,omitempty"`
	TokensPerSec float64 `json:"tokens_per_sec,omitempty"`
	TTFTMs       int64   `json:"ttft_ms,omitempty"`

	// Tool information
	ToolName   string `json:"tool_name,omitempty"`
	ToolInput  string `json:"tool_input,omitempty"`
	ToolResult string `json:"tool_result,omitempty"`
	IsSuccess  bool   `json:"is_success,omitempty"`
}

// ConversationMeta contains metadata for listing conversations.
type ConversationMeta struct {
	ID           string    `json:"id"`
	Summary      string    `json:"summary"`
	Model        string    `json:"model"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
	Preview      string    `json:"preview"` // First user message truncated
}

// =============================================================================
// CONVERSATION STORE
// =============================================================================

// ConversationStore handles conversation persistence.
type ConversationStore struct {
	// BaseDir is the directory for storing conversations
	// Default: ~/.rigrun/conversations/
	BaseDir string

	// MaxConversations limits stored conversations (0 = unlimited)
	MaxConversations int
}

// NewConversationStore creates a new conversation store.
func NewConversationStore() (*ConversationStore, error) {
	// Get user home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	baseDir := filepath.Join(homeDir, ".rigrun", "conversations")

	// Ensure directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}

	return &ConversationStore{
		BaseDir:          baseDir,
		MaxConversations: 100,
	}, nil
}

// NewConversationStoreWithDir creates a store with a custom directory.
func NewConversationStoreWithDir(baseDir string) (*ConversationStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}

	return &ConversationStore{
		BaseDir:          baseDir,
		MaxConversations: 100,
	}, nil
}

// =============================================================================
// SAVE OPERATIONS
// =============================================================================

// Save persists a conversation and returns its ID.
func (s *ConversationStore) Save(conv *StoredConversation) (string, error) {
	// Generate ID if not set
	if conv.ID == "" {
		conv.ID = generateConversationID()
	}

	// Auto-generate summary if not set
	if conv.Summary == "" {
		conv.Summary = s.generateSummary(conv)
	}

	// Update timestamp
	conv.UpdatedAt = time.Now()
	if conv.CreatedAt.IsZero() {
		conv.CreatedAt = conv.UpdatedAt
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return "", err
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	filePath := s.filePath(conv.ID)
	if err := util.AtomicWriteFile(filePath, data, 0644); err != nil {
		return "", err
	}

	// Enforce max conversations limit
	if s.MaxConversations > 0 {
		s.enforceLimit()
	}

	return conv.ID, nil
}

// generateSummary creates a summary from the first user message.
func (s *ConversationStore) generateSummary(conv *StoredConversation) string {
	for _, msg := range conv.Messages {
		if msg.Role == "user" && msg.Content != "" {
			content := msg.Content
			// Truncate to 50 characters using rune-based truncation for Unicode safety
			runes := []rune(content)
			if len(runes) > 50 {
				content = string(runes[:47]) + "..."
			}
			// Remove newlines
			content = strings.ReplaceAll(content, "\n", " ")
			content = strings.ReplaceAll(content, "\r", "")
			return content
		}
	}
	return "New conversation"
}

// enforceLimit removes oldest conversations if over limit.
func (s *ConversationStore) enforceLimit() {
	metas, err := s.List()
	if err != nil || len(metas) <= s.MaxConversations {
		return
	}

	// Sort by updated time (oldest first)
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].UpdatedAt.Before(metas[j].UpdatedAt)
	})

	// Delete excess
	excess := len(metas) - s.MaxConversations
	for i := 0; i < excess; i++ {
		s.Delete(metas[i].ID)
	}
}

// =============================================================================
// LOAD OPERATIONS
// =============================================================================

// Load retrieves a conversation by ID.
func (s *ConversationStore) Load(id string) (*StoredConversation, error) {
	filePath := s.filePath(id)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}

	var conv StoredConversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, err
	}

	return &conv, nil
}

// LoadByIndex loads a conversation by its index in the list (0 = most recent).
func (s *ConversationStore) LoadByIndex(index int) (*StoredConversation, error) {
	metas, err := s.List()
	if err != nil {
		return nil, err
	}

	if index < 0 || index >= len(metas) {
		return nil, ErrConversationNotFound
	}

	return s.Load(metas[index].ID)
}

// =============================================================================
// LIST OPERATIONS
// =============================================================================

// List returns all saved conversations (most recent first).
func (s *ConversationStore) List() ([]ConversationMeta, error) {
	entries, err := os.ReadDir(s.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ConversationMeta{}, nil
		}
		return nil, err
	}

	var metas []ConversationMeta

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Extract ID from filename
		id := strings.TrimSuffix(entry.Name(), ".json")

		// Load the conversation to get metadata
		conv, err := s.Load(id)
		if err != nil {
			continue // Skip corrupted files
		}

		// Get first user message as preview
		preview := ""
		for _, msg := range conv.Messages {
			if msg.Role == "user" {
				preview = msg.Content
				// Use rune-based truncation for Unicode safety
				previewRunes := []rune(preview)
				if len(previewRunes) > 80 {
					preview = string(previewRunes[:77]) + "..."
				}
				break
			}
		}

		metas = append(metas, ConversationMeta{
			ID:           conv.ID,
			Summary:      conv.Summary,
			Model:        conv.Model,
			CreatedAt:    conv.CreatedAt,
			UpdatedAt:    conv.UpdatedAt,
			MessageCount: len(conv.Messages),
			Preview:      preview,
		})
	}

	// Sort by updated time (most recent first)
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].UpdatedAt.After(metas[j].UpdatedAt)
	})

	return metas, nil
}

// Search finds conversations matching a query string.
func (s *ConversationStore) Search(query string) ([]ConversationMeta, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var results []ConversationMeta

	for _, meta := range all {
		// Search in summary and preview
		if strings.Contains(strings.ToLower(meta.Summary), query) ||
			strings.Contains(strings.ToLower(meta.Preview), query) {
			results = append(results, meta)
		}
	}

	return results, nil
}

// =============================================================================
// DELETE OPERATIONS
// =============================================================================

// Delete removes a conversation by ID.
func (s *ConversationStore) Delete(id string) error {
	filePath := s.filePath(id)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return ErrConversationNotFound
		}
		return err
	}

	return nil
}

// Clear removes all saved conversations.
func (s *ConversationStore) Clear() error {
	entries, err := os.ReadDir(s.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			filePath := filepath.Join(s.BaseDir, entry.Name())
			os.Remove(filePath)
		}
	}

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// filePath returns the file path for a conversation ID.
func (s *ConversationStore) filePath(id string) string {
	return filepath.Join(s.BaseDir, id+".json")
}

// generateConversationID creates a unique conversation ID.
func generateConversationID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "conv_" + hex.EncodeToString(bytes)
}

// =============================================================================
// ERRORS
// =============================================================================

// ErrConversationNotFound is returned when a conversation doesn't exist.
// Use errors.Is(err, ErrConversationNotFound) to check for this error.
var ErrConversationNotFound = &ConversationError{Message: "conversation not found"}

// ConversationError represents a conversation-related error.
// It implements the error interface and can be compared using errors.Is.
type ConversationError struct {
	Message string
}

// Error implements the error interface.
func (e *ConversationError) Error() string {
	return e.Message
}

// Is implements errors.Is support for comparing conversation errors.
func (e *ConversationError) Is(target error) bool {
	t, ok := target.(*ConversationError)
	if !ok {
		return false
	}
	return e.Message == t.Message
}

// =============================================================================
// SESSION LIST FORMATTING
// =============================================================================

// FormatSessionList formats a list of sessions for display in a table format.
// Returns a human-readable string with session ID, creation time, message count, and preview.
func FormatSessionList(sessions []ConversationMeta) string {
	if len(sessions) == 0 {
		return "No sessions found."
	}

	var sb strings.Builder
	sb.WriteString("Sessions:\n")
	sb.WriteString("-----------------------------------------------------\n")
	sb.WriteString(formatPadded("ID", 12) + " " + formatPadded("Created", 20) + " " + formatPadded("Messages", 8) + " Preview\n")
	sb.WriteString("-----------------------------------------------------\n")

	for _, s := range sessions {
		preview := truncateString(s.Preview, 30)
		// Format: ID (truncated to 12 chars), Created time, Message count, Preview
		idStr := s.ID
		if len(idStr) > 12 {
			idStr = idStr[:12]
		}
		createdStr := s.CreatedAt.Format("2006-01-02 15:04")
		msgCountStr := util.IntToStr(s.MessageCount)

		sb.WriteString(formatPadded(idStr, 12) + " " +
			formatPadded(createdStr, 20) + " " +
			formatPadded(msgCountStr, 8) + " " +
			preview + "\n")
	}
	return sb.String()
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated.
// Uses rune-based truncation for proper Unicode handling.
func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// formatPadded pads a string to the specified width with spaces.
func formatPadded(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	padding := width - len(runes)
	for i := 0; i < padding; i++ {
		s += " "
	}
	return s
}


// =============================================================================
// SESSION SEARCH (MESSAGE CONTENT)
// =============================================================================

// SearchMessages searches conversations by message content.
// Returns conversations where any message contains the query string (case-insensitive).
func (s *ConversationStore) SearchMessages(query string) ([]ConversationMeta, error) {
	if query == "" {
		return s.List()
	}

	query = strings.ToLower(query)
	all, err := s.List()
	if err != nil {
		return nil, err
	}

	var results []ConversationMeta

	for _, meta := range all {
		// Load full conversation to search message content
		conv, err := s.Load(meta.ID)
		if err != nil {
			continue
		}

		// Search in all messages
		for _, msg := range conv.Messages {
			if strings.Contains(strings.ToLower(msg.Content), query) {
				results = append(results, meta)
				break // Found a match, move to next conversation
			}
		}
	}

	return results, nil
}

// =============================================================================
// SESSION EXPORT
// =============================================================================

// ExportMarkdown exports the conversation as a Markdown formatted string.
// Includes session metadata, timestamps, and all messages with role labels.
func (c *StoredConversation) ExportMarkdown() string {
	var sb strings.Builder
	sb.WriteString("# Session " + c.ID + "\n\n")
	sb.WriteString("Created: " + c.CreatedAt.Format(time.RFC3339) + "\n\n")
	sb.WriteString("---\n\n")

	for _, msg := range c.Messages {
		role := "**User**"
		if msg.Role == "assistant" {
			role = "**Assistant**"
		} else if msg.Role == "system" {
			role = "**System**"
		} else if msg.Role == "tool" {
			role = "**Tool**"
		}
		sb.WriteString(role + " (" + msg.Timestamp.Format("15:04") + "):\n\n")
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n---\n\n")
	}

	return sb.String()
}

// ExportJSON exports the conversation as a pretty-printed JSON byte array.
// Returns an error if JSON marshaling fails.
func (c *StoredConversation) ExportJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// GetPreview returns a preview string from the first user message.
// Returns empty string if no user messages exist.
func (c *StoredConversation) GetPreview() string {
	for _, msg := range c.Messages {
		if msg.Role == "user" && msg.Content != "" {
			return truncateString(msg.Content, 80)
		}
	}
	return ""
}

// MessageCount returns the number of messages in the conversation.
func (c *StoredConversation) MessageCount() int {
	return len(c.Messages)
}
