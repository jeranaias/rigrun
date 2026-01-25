// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package context provides the @ mention system for including context in messages.
package context

import (
	"strings"
)

// =============================================================================
// EXPANDER
// =============================================================================

// Expander handles expanding @ mentions into full context for LLM messages.
type Expander struct {
	parser  *Parser
	fetcher *Fetcher
}

// NewExpander creates a new context expander.
func NewExpander(fetcher *Fetcher) *Expander {
	if fetcher == nil {
		fetcher = NewFetcher(nil)
	}
	return &Expander{
		parser:  NewParser(),
		fetcher: fetcher,
	}
}

// =============================================================================
// EXPANSION RESULT
// =============================================================================

// ExpansionResult contains the result of expanding a message.
type ExpansionResult struct {
	// OriginalMessage is the original user message
	OriginalMessage string

	// ExpandedMessage is the message with context prepended
	ExpandedMessage string

	// CleanMessage is the message with mentions removed (for display)
	CleanMessage string

	// Mentions are the parsed mentions
	Mentions []Mention

	// Summary describes what context was included
	Summary MentionSummary

	// Errors contains any errors that occurred during expansion
	Errors []MentionError
}

// MentionError represents an error fetching a mention.
type MentionError struct {
	Mention Mention
	Error   error
}

// HasErrors returns true if there were any errors during expansion.
func (r *ExpansionResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// ErrorSummary returns a summary of errors.
func (r *ExpansionResult) ErrorSummary() string {
	if len(r.Errors) == 0 {
		return ""
	}

	var parts []string
	for _, e := range r.Errors {
		parts = append(parts, e.Mention.Raw+": "+e.Error.Error())
	}
	return strings.Join(parts, "; ")
}

// =============================================================================
// EXPANSION METHODS
// =============================================================================

// Expand expands all @ mentions in the message.
// Returns the expanded message with context prepended and the clean message.
func (e *Expander) Expand(message string) *ExpansionResult {
	result := &ExpansionResult{
		OriginalMessage: message,
	}

	// Parse mentions
	mentions, cleanMessage := e.parser.Parse(message)
	result.Mentions = mentions
	result.CleanMessage = cleanMessage

	// If no mentions, return original message
	if len(mentions) == 0 {
		result.ExpandedMessage = message
		return result
	}

	// Fetch content for each mention
	mentions = e.fetcher.FetchAll(mentions)
	result.Mentions = mentions

	// Create summary
	result.Summary = Summarize(mentions)

	// Collect errors
	for _, m := range mentions {
		if m.Error != nil {
			result.Errors = append(result.Errors, MentionError{
				Mention: m,
				Error:   m.Error,
			})
		}
	}

	// Build expanded message
	result.ExpandedMessage = e.buildExpandedMessage(mentions, cleanMessage)

	return result
}

// buildExpandedMessage constructs the full message with context.
func (e *Expander) buildExpandedMessage(mentions []Mention, userMessage string) string {
	var sb strings.Builder

	// Add context header
	hasContext := false
	for _, m := range mentions {
		if m.Content != "" {
			hasContext = true
			break
		}
	}

	if hasContext {
		sb.WriteString("<context>\n")

		// Add each mention's content
		for _, m := range mentions {
			if m.Content == "" {
				continue
			}

			// Section header based on mention type
			sb.WriteString("\n<")
			sb.WriteString(m.Type.String())
			if m.Type == MentionFile && m.Path != "" {
				sb.WriteString(" path=\"")
				sb.WriteString(m.Path)
				sb.WriteString("\"")
			}
			sb.WriteString(">\n")

			sb.WriteString(m.Content)

			sb.WriteString("\n</")
			sb.WriteString(m.Type.String())
			sb.WriteString(">\n")
		}

		sb.WriteString("\n</context>\n\n")
	}

	// Add user message
	sb.WriteString(userMessage)

	return sb.String()
}

// =============================================================================
// QUICK EXPAND FUNCTIONS
// =============================================================================

// QuickExpand is a convenience function for one-off expansion.
func QuickExpand(message string) *ExpansionResult {
	expander := NewExpander(nil)
	return expander.Expand(message)
}

// QuickExpandWithConfig is a convenience function with custom config.
func QuickExpandWithConfig(message string, config *FetcherConfig) *ExpansionResult {
	fetcher := NewFetcher(config)
	expander := NewExpander(fetcher)
	return expander.Expand(message)
}

// =============================================================================
// FORMATTING OPTIONS
// =============================================================================

// FormatOptions controls how expanded messages are formatted.
type FormatOptions struct {
	// UseXMLTags wraps context in XML tags (default: true)
	UseXMLTags bool

	// IncludeLineNumbers for file content (default: true)
	IncludeLineNumbers bool

	// MaxContentLength truncates content longer than this (0 = no limit)
	MaxContentLength int

	// Separator between context sections
	Separator string
}

// DefaultFormatOptions returns the default formatting options.
func DefaultFormatOptions() *FormatOptions {
	return &FormatOptions{
		UseXMLTags:         true,
		IncludeLineNumbers: true,
		MaxContentLength:   0,
		Separator:          "\n---\n",
	}
}

// ExpandWithOptions expands mentions with custom formatting.
func (e *Expander) ExpandWithOptions(message string, opts *FormatOptions) *ExpansionResult {
	if opts == nil {
		opts = DefaultFormatOptions()
	}

	result := e.Expand(message)

	// Apply formatting options
	if opts.MaxContentLength > 0 {
		result.ExpandedMessage = truncateContent(result.ExpandedMessage, opts.MaxContentLength)
	}

	return result
}

// truncateContent truncates content to the specified length.
// Uses rune-based truncation to handle Unicode correctly.
func truncateContent(content string, maxLen int) string {
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[:maxLen-3]) + "..."
}

// =============================================================================
// PREVIEW GENERATION
// =============================================================================

// GeneratePreview creates a preview of what context will be included.
// This is useful for showing the user what @mentions will expand to.
func (e *Expander) GeneratePreview(message string) string {
	mentions, _ := e.parser.Parse(message)

	if len(mentions) == 0 {
		return "No context mentions found"
	}

	var sb strings.Builder
	sb.WriteString("Context to include:\n")

	for _, m := range mentions {
		sb.WriteString("  - ")
		sb.WriteString(m.Raw)

		switch m.Type {
		case MentionFile:
			sb.WriteString(" (file: ")
			sb.WriteString(m.Path)
			sb.WriteString(")")
		case MentionGit:
			if m.Range != "" {
				sb.WriteString(" (range: ")
				sb.WriteString(m.Range)
				sb.WriteString(")")
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// =============================================================================
// CONTEXT SIZE ESTIMATION
// =============================================================================

// EstimateContextSize estimates the token count for expanded context.
// Uses rough approximation of 4 characters per token.
// WARNING: This does full I/O to fetch file contents. For UI updates,
// use EstimateContextSizeFast() instead.
func (e *Expander) EstimateContextSize(message string) int {
	result := e.Expand(message)
	return (len(result.ExpandedMessage) + 3) / 4
}

// EstimateContextSizeFast estimates token count WITHOUT doing I/O.
// Uses heuristics for typical content sizes. Safe to call on every keystroke.
// Returns estimated tokens based on mention types without fetching content.
func (e *Expander) EstimateContextSizeFast(message string) int {
	mentions, cleanMsg := e.parser.Parse(message)

	// Start with the clean message size
	estimatedChars := len(cleanMsg)

	// Add estimates per mention type (without fetching content)
	for _, m := range mentions {
		switch m.Type {
		case MentionFile:
			// Typical source file: ~5KB = ~1250 tokens
			estimatedChars += 5000
		case MentionClipboard:
			// Typical clipboard: ~500 chars = ~125 tokens
			estimatedChars += 500
		case MentionGit:
			// Git context (commits + status + diff): ~2KB = ~500 tokens
			estimatedChars += 2000
		case MentionCodebase:
			// Codebase tree summary: ~10KB = ~2500 tokens
			estimatedChars += 10000
		case MentionLastError:
			// Error message: ~200 chars = ~50 tokens
			estimatedChars += 200
		case MentionURL:
			// Web page content: ~3KB = ~750 tokens
			estimatedChars += 3000
		}
	}

	return (estimatedChars + 3) / 4
}

// ContextSizeInfo provides detailed size information.
type ContextSizeInfo struct {
	// TotalChars is the total character count
	TotalChars int

	// EstimatedTokens is the estimated token count
	EstimatedTokens int

	// MessageChars is the user message character count
	MessageChars int

	// ContextChars is the context character count
	ContextChars int

	// BreakdownByType shows tokens per mention type
	BreakdownByType map[MentionType]int
}

// GetContextSizeInfo returns detailed size information.
func (e *Expander) GetContextSizeInfo(message string) ContextSizeInfo {
	result := e.Expand(message)

	info := ContextSizeInfo{
		TotalChars:      len(result.ExpandedMessage),
		EstimatedTokens: (len(result.ExpandedMessage) + 3) / 4,
		MessageChars:    len(result.CleanMessage),
		ContextChars:    len(result.ExpandedMessage) - len(result.CleanMessage),
		BreakdownByType: make(map[MentionType]int),
	}

	for _, m := range result.Mentions {
		if m.Content != "" {
			tokens := (len(m.Content) + 3) / 4
			info.BreakdownByType[m.Type] += tokens
		}
	}

	return info
}

// =============================================================================
// STREAMING SUPPORT
// =============================================================================

// StreamingExpander supports incremental expansion for long messages.
type StreamingExpander struct {
	*Expander
	mentions []Mention
	fetched  int
}

// NewStreamingExpander creates a streaming expander.
func NewStreamingExpander(fetcher *Fetcher) *StreamingExpander {
	return &StreamingExpander{
		Expander: NewExpander(fetcher),
	}
}

// Start begins streaming expansion by parsing mentions.
func (se *StreamingExpander) Start(message string) []Mention {
	se.mentions, _ = se.parser.Parse(message)
	se.fetched = 0
	return se.mentions
}

// FetchNext fetches the next mention's content.
// Returns the mention, and true if there are more to fetch.
func (se *StreamingExpander) FetchNext() (*Mention, bool) {
	if se.fetched >= len(se.mentions) {
		return nil, false
	}

	m := &se.mentions[se.fetched]
	se.fetcher.Fetch(m)
	se.fetched++

	return m, se.fetched < len(se.mentions)
}

// Progress returns the current progress (0.0 to 1.0).
func (se *StreamingExpander) Progress() float64 {
	if len(se.mentions) == 0 {
		return 1.0
	}
	return float64(se.fetched) / float64(len(se.mentions))
}

// Complete returns the final expansion result.
func (se *StreamingExpander) Complete(originalMessage string) *ExpansionResult {
	_, cleanMessage := se.parser.Parse(originalMessage)

	result := &ExpansionResult{
		OriginalMessage: originalMessage,
		Mentions:        se.mentions,
		CleanMessage:    cleanMessage,
		Summary:         Summarize(se.mentions),
	}

	for _, m := range se.mentions {
		if m.Error != nil {
			result.Errors = append(result.Errors, MentionError{
				Mention: m,
				Error:   m.Error,
			})
		}
	}

	result.ExpandedMessage = se.buildExpandedMessage(se.mentions, cleanMessage)
	return result
}
