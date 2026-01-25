// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package context provides the @ mention system for including context in messages.
package context

import (
	"regexp"
	"sort"
	"strings"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// MENTION TYPES
// =============================================================================

// MentionType indicates the type of @ mention.
type MentionType int

const (
	MentionFile       MentionType = iota // @file:path
	MentionClipboard                     // @clipboard
	MentionGit                           // @git or @git:range
	MentionCodebase                      // @codebase
	MentionLastError                     // @error
	MentionURL                           // @url:https://...
)

// String returns the string representation of the mention type.
func (t MentionType) String() string {
	switch t {
	case MentionFile:
		return "file"
	case MentionClipboard:
		return "clipboard"
	case MentionGit:
		return "git"
	case MentionCodebase:
		return "codebase"
	case MentionLastError:
		return "error"
	case MentionURL:
		return "url"
	default:
		return "unknown"
	}
}

// =============================================================================
// MENTION STRUCT
// =============================================================================

// Mention represents a parsed @ mention in user input.
type Mention struct {
	// Type of mention
	Type MentionType

	// Raw is the original text (e.g., "@file:src/main.go")
	Raw string

	// Path for file mentions
	Path string

	// Range for git mentions (e.g., "HEAD~3")
	Range string

	// URL for URL mentions
	URL string

	// Content is populated after fetching
	Content string

	// Error if fetching failed
	Error error

	// Start and End positions in the original input
	Start int
	End   int
}

// IsResolved returns true if the mention has been resolved (content fetched).
func (m *Mention) IsResolved() bool {
	return m.Content != "" || m.Error != nil
}

// HasError returns true if there was an error fetching the mention.
func (m *Mention) HasError() bool {
	return m.Error != nil
}

// =============================================================================
// PARSER
// =============================================================================

// Parser parses @ mentions from user input.
type Parser struct {
	// Patterns for matching different mention types
	patterns map[MentionType]*regexp.Regexp
}

// NewParser creates a new mention parser.
func NewParser() *Parser {
	return &Parser{
		patterns: map[MentionType]*regexp.Regexp{
			// @file:path or @file:"path with spaces"
			MentionFile: regexp.MustCompile(`@file:(?:"([^"]+)"|'([^']+)'|(\S+))`),

			// @clipboard
			MentionClipboard: regexp.MustCompile(`@clipboard\b`),

			// @git or @git:range
			MentionGit: regexp.MustCompile(`@git(?::(\S+))?\b`),

			// @codebase
			MentionCodebase: regexp.MustCompile(`@codebase\b`),

			// @error
			MentionLastError: regexp.MustCompile(`@error\b`),

			// @url:https://... or @url:"https://..."
			MentionURL: regexp.MustCompile(`@url:(?:"([^"]+)"|'([^']+)'|(\S+))`),
		},
	}
}

// Parse extracts all @ mentions from the input string.
// Returns the list of mentions and the remaining text with mentions removed.
func (p *Parser) Parse(input string) ([]Mention, string) {
	var mentions []Mention

	// Track positions to remove from the text
	var removals []removal

	// Parse file mentions
	for _, match := range p.patterns[MentionFile].FindAllStringSubmatchIndex(input, -1) {
		raw := input[match[0]:match[1]]

		// Extract path from capture groups (quoted or unquoted)
		var path string
		// BUGFIX: Add bounds check to prevent index out of bounds panic
		for i := 2; i <= 6 && i+1 < len(match); i += 2 {
			if match[i] != -1 {
				path = input[match[i]:match[i+1]]
				break
			}
		}

		mentions = append(mentions, Mention{
			Type:  MentionFile,
			Raw:   raw,
			Path:  path,
			Start: match[0],
			End:   match[1],
		})
		removals = append(removals, removal{match[0], match[1]})
	}

	// Parse clipboard mentions
	for _, match := range p.patterns[MentionClipboard].FindAllStringIndex(input, -1) {
		mentions = append(mentions, Mention{
			Type:  MentionClipboard,
			Raw:   input[match[0]:match[1]],
			Start: match[0],
			End:   match[1],
		})
		removals = append(removals, removal{match[0], match[1]})
	}

	// Parse git mentions
	for _, match := range p.patterns[MentionGit].FindAllStringSubmatchIndex(input, -1) {
		raw := input[match[0]:match[1]]

		gitRange := ""
		if match[2] != -1 {
			gitRange = input[match[2]:match[3]]
		}

		mentions = append(mentions, Mention{
			Type:  MentionGit,
			Raw:   raw,
			Range: gitRange,
			Start: match[0],
			End:   match[1],
		})
		removals = append(removals, removal{match[0], match[1]})
	}

	// Parse codebase mentions
	for _, match := range p.patterns[MentionCodebase].FindAllStringIndex(input, -1) {
		mentions = append(mentions, Mention{
			Type:  MentionCodebase,
			Raw:   input[match[0]:match[1]],
			Start: match[0],
			End:   match[1],
		})
		removals = append(removals, removal{match[0], match[1]})
	}

	// Parse error mentions
	for _, match := range p.patterns[MentionLastError].FindAllStringIndex(input, -1) {
		mentions = append(mentions, Mention{
			Type:  MentionLastError,
			Raw:   input[match[0]:match[1]],
			Start: match[0],
			End:   match[1],
		})
		removals = append(removals, removal{match[0], match[1]})
	}

	// Parse URL mentions
	for _, match := range p.patterns[MentionURL].FindAllStringSubmatchIndex(input, -1) {
		raw := input[match[0]:match[1]]

		// Extract URL from capture groups
		var url string
		for i := 2; i <= 6; i += 2 {
			if match[i] != -1 {
				url = input[match[i]:match[i+1]]
				break
			}
		}

		mentions = append(mentions, Mention{
			Type:  MentionURL,
			Raw:   raw,
			URL:   url,
			Start: match[0],
			End:   match[1],
		})
		removals = append(removals, removal{match[0], match[1]})
	}

	// Build the remaining text (with mentions removed)
	remaining := removeMentions(input, removals)

	return mentions, remaining
}

// ParseMentionTypes returns a list of all mention type prefixes.
func ParseMentionTypes() []string {
	return []string{
		"@file:",
		"@clipboard",
		"@git",
		"@codebase",
		"@error",
		"@url:",
	}
}

// =============================================================================
// HELPER TYPES
// =============================================================================

// removal represents a range to remove from input text.
type removal struct {
	start, end int
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// removeMentions removes the specified ranges from the input string.
func removeMentions(input string, removals []removal) string {
	if len(removals) == 0 {
		return input
	}

	// PERFORMANCE: O(n log n) standard library sort
	// Sort removals by start position (descending) to process from end
	sort.Slice(removals, func(i, j int) bool {
		return removals[i].start > removals[j].start
	})

	result := input
	for _, r := range removals {
		// Remove the mention and any trailing whitespace
		end := r.end
		for end < len(result) && result[end] == ' ' {
			end++
		}
		result = result[:r.start] + result[end:]
	}

	// Clean up multiple spaces
	result = strings.Join(strings.Fields(result), " ")
	return strings.TrimSpace(result)
}

// HasMentions returns true if the input contains any @ mentions.
func HasMentions(input string) bool {
	return strings.Contains(input, "@file:") ||
		strings.Contains(input, "@clipboard") ||
		strings.Contains(input, "@git") ||
		strings.Contains(input, "@codebase") ||
		strings.Contains(input, "@error") ||
		strings.Contains(input, "@url:")
}

// GetMentionAtPosition returns the mention at the given cursor position, if any.
func GetMentionAtPosition(input string, pos int) *Mention {
	parser := NewParser()
	mentions, _ := parser.Parse(input)

	for i := range mentions {
		if pos >= mentions[i].Start && pos <= mentions[i].End {
			return &mentions[i]
		}
	}

	return nil
}

// HighlightMentions returns the input with mentions highlighted (for display).
// The highlighter function is called for each mention to wrap it with styling.
func HighlightMentions(input string, highlighter func(mention string) string) string {
	if highlighter == nil {
		return input
	}

	parser := NewParser()
	mentions, _ := parser.Parse(input)

	if len(mentions) == 0 {
		return input
	}

	// PERFORMANCE: O(n log n) standard library sort
	// Sort mentions by start position (descending) to replace from end
	sort.Slice(mentions, func(i, j int) bool {
		return mentions[i].Start > mentions[j].Start
	})

	result := input
	for _, m := range mentions {
		highlighted := highlighter(m.Raw)
		result = result[:m.Start] + highlighted + result[m.End:]
	}

	return result
}

// =============================================================================
// MENTION SUMMARY
// =============================================================================

// MentionSummary provides a summary of mentions in user input.
type MentionSummary struct {
	TotalCount int
	FileCount  int
	Files      []string
	HasClipboard bool
	HasGit       bool
	GitRange     string
	HasCodebase  bool
	HasError     bool
	HasURL       bool
	URLs         []string
}

// Summarize creates a summary of the given mentions.
func Summarize(mentions []Mention) MentionSummary {
	summary := MentionSummary{
		TotalCount: len(mentions),
	}

	for _, m := range mentions {
		switch m.Type {
		case MentionFile:
			summary.FileCount++
			summary.Files = append(summary.Files, m.Path)
		case MentionClipboard:
			summary.HasClipboard = true
		case MentionGit:
			summary.HasGit = true
			if m.Range != "" {
				summary.GitRange = m.Range
			}
		case MentionCodebase:
			summary.HasCodebase = true
		case MentionLastError:
			summary.HasError = true
		case MentionURL:
			summary.HasURL = true
			summary.URLs = append(summary.URLs, m.URL)
		}
	}

	return summary
}

// FormatSummary returns a formatted string describing the mentions.
func (s *MentionSummary) FormatSummary() string {
	if s.TotalCount == 0 {
		return ""
	}

	var parts []string

	if s.FileCount > 0 {
		if s.FileCount == 1 {
			parts = append(parts, "1 file")
		} else {
			parts = append(parts, util.IntToStr(s.FileCount)+" files")
		}
	}

	if s.HasClipboard {
		parts = append(parts, "clipboard")
	}

	if s.HasGit {
		if s.GitRange != "" {
			parts = append(parts, "git:"+s.GitRange)
		} else {
			parts = append(parts, "git")
		}
	}

	if s.HasCodebase {
		parts = append(parts, "codebase")
	}

	if s.HasError {
		parts = append(parts, "error")
	}

	if s.HasURL {
		if len(s.URLs) == 1 {
			parts = append(parts, "1 URL")
		} else {
			parts = append(parts, util.IntToStr(len(s.URLs))+" URLs")
		}
	}

	return strings.Join(parts, ", ")
}

