// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package commands provides the slash command system for the TUI.
package commands

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// =============================================================================
// COMPLETER
// =============================================================================

// Completer handles tab completion for commands and arguments.
type Completer struct {
	registry *Registry

	// Callbacks for dynamic completion
	// These are set by the application to provide context-specific completions
	ModelsFn   func() []string           // Returns available models
	SessionsFn func() []SessionInfo      // Returns saved sessions
	ToolsFn    func() []string           // Returns available tools
	ConfigFn   func() []string           // Returns config keys
	FilesFn    func(prefix string) []string // Returns matching files
}

// NewCompleter creates a new completer with the given registry.
func NewCompleter(registry *Registry) *Completer {
	return &Completer{
		registry: registry,
	}
}

// GetCommand returns a command by name from the completer's registry.
func (c *Completer) GetCommand(name string) *Command {
	if c.registry == nil {
		return nil
	}
	return c.registry.Get(name)
}

// Complete returns completions for the given input at the cursor position.
func (c *Completer) Complete(input string, cursorPos int) []Completion {
	// If cursor is not at end, use the portion up to cursor
	if cursorPos < len(input) {
		input = input[:cursorPos]
	}

	input = strings.TrimSpace(input)

	// Not a command - check for file mention completion
	if !strings.HasPrefix(input, "/") {
		return c.completeMentions(input)
	}

	// Parse the input to determine what we're completing
	parts := splitCommandLine(input)
	if len(parts) == 0 {
		return c.completeCommands("")
	}

	// Still typing the command name?
	if len(parts) == 1 && !strings.HasSuffix(input, " ") {
		return c.completeCommands(parts[0])
	}

	// Completing an argument
	cmd := c.registry.Get(parts[0])
	if cmd == nil {
		return nil
	}

	// Determine which argument we're completing
	argIndex := len(parts) - 2 // -1 for command, -1 for 0-based index
	if strings.HasSuffix(input, " ") {
		argIndex++
	}

	partial := ""
	if !strings.HasSuffix(input, " ") && len(parts) > 1 {
		partial = parts[len(parts)-1]
	}

	return c.completeArg(cmd, argIndex, partial)
}

// =============================================================================
// COMMAND COMPLETION
// =============================================================================

// completeCommands returns completions for command names.
func (c *Completer) completeCommands(partial string) []Completion {
	var completions []Completion

	partial = strings.ToLower(partial)

	for _, cmd := range c.registry.All() {
		if cmd.Hidden {
			continue
		}

		// Check main name
		if strings.HasPrefix(strings.ToLower(cmd.Name), partial) {
			score := calculateScore(cmd.Name, partial)
			completions = append(completions, Completion{
				Value:       cmd.Name,
				Display:     cmd.Name,
				Description: cmd.Description,
				Score:       score,
			})
		}

		// Check aliases
		for _, alias := range cmd.Aliases {
			if strings.HasPrefix(strings.ToLower(alias), partial) {
				score := calculateScore(alias, partial)
				completions = append(completions, Completion{
					Value:       alias,
					Display:     alias + " -> " + cmd.Name,
					Description: cmd.Description,
					Score:       score - 10, // Slightly lower score for aliases
				})
			}
		}
	}

	// Sort by score (descending), then alphabetically
	sortCompletions(completions)

	return completions
}

// =============================================================================
// ARGUMENT COMPLETION
// =============================================================================

// completeArg returns completions for a command argument.
func (c *Completer) completeArg(cmd *Command, argIndex int, partial string) []Completion {
	if argIndex < 0 || argIndex >= len(cmd.Args) {
		return nil
	}

	arg := cmd.Args[argIndex]

	switch arg.Type {
	case ArgTypeModel:
		return c.completeModels(partial)
	case ArgTypeSession:
		return c.completeSessions(partial)
	case ArgTypeFile:
		return c.completeFiles(partial)
	case ArgTypeEnum:
		return c.completeEnum(arg.Values, partial)
	case ArgTypeTool:
		return c.completeTools(partial)
	case ArgTypeConfig:
		return c.completeConfig(partial)
	case ArgTypeString:
		// Check if there's a custom completer
		if arg.Completer != nil {
			values := arg.Completer()
			return c.completeFromList(values, partial)
		}
		return nil
	default:
		return nil
	}
}

// completeModels returns completions for model names.
func (c *Completer) completeModels(partial string) []Completion {
	var models []string
	if c.ModelsFn != nil {
		models = c.ModelsFn()
	} else {
		// Default models for testing
		models = []string{
			"qwen2.5-coder:14b",
			"qwen2.5-coder:7b",
			"codestral:22b",
			"llama3.1:8b",
			"deepseek-coder:6.7b",
		}
	}

	return c.completeFromList(models, partial)
}

// completeSessions returns completions for session IDs.
func (c *Completer) completeSessions(partial string) []Completion {
	if c.SessionsFn == nil {
		return nil
	}

	sessions := c.SessionsFn()
	var completions []Completion

	partial = strings.ToLower(partial)

	for _, session := range sessions {
		idMatch := strings.HasPrefix(strings.ToLower(session.ID), partial)
		titleMatch := strings.Contains(strings.ToLower(session.Title), partial)

		if idMatch || titleMatch {
			score := calculateScore(session.ID, partial)
			if titleMatch && !idMatch {
				score -= 5
			}

			display := session.ID
			if session.Title != "" {
				display = session.ID + " - " + truncate(session.Title, 30)
			}

			completions = append(completions, Completion{
				Value:       session.ID,
				Display:     display,
				Description: session.Preview,
				Score:       score,
			})
		}
	}

	sortCompletions(completions)
	return completions
}

// completeFiles returns completions for file paths.
func (c *Completer) completeFiles(partial string) []Completion {
	// Use custom function if provided
	if c.FilesFn != nil {
		paths := c.FilesFn(partial)
		return c.completeFromList(paths, partial)
	}

	// Default file completion
	return c.defaultFileCompletion(partial)
}

// defaultFileCompletion provides basic file path completion.
func (c *Completer) defaultFileCompletion(partial string) []Completion {
	var completions []Completion

	// Handle empty partial
	if partial == "" {
		partial = "."
	}

	// Get the directory and prefix
	dir := filepath.Dir(partial)
	prefix := filepath.Base(partial)
	if strings.HasSuffix(partial, string(os.PathSeparator)) {
		dir = partial
		prefix = ""
	}

	// Read directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	prefix = strings.ToLower(prefix)

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(strings.ToLower(name), prefix) {
			continue
		}

		// Skip hidden files unless partial starts with .
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}

		path := filepath.Join(dir, name)
		if entry.IsDir() {
			path += string(os.PathSeparator)
		}

		score := calculateScore(name, prefix)
		// Boost directories
		if entry.IsDir() {
			score += 5
		}

		desc := ""
		if info, err := entry.Info(); err == nil {
			if entry.IsDir() {
				desc = "directory"
			} else {
				desc = formatFileSize(info.Size())
			}
		}

		completions = append(completions, Completion{
			Value:       path,
			Display:     name,
			Description: desc,
			Score:       score,
		})
	}

	sortCompletions(completions)

	// Limit results
	if len(completions) > 20 {
		completions = completions[:20]
	}

	return completions
}

// completeEnum returns completions for enum values.
func (c *Completer) completeEnum(values []string, partial string) []Completion {
	return c.completeFromList(values, partial)
}

// completeTools returns completions for tool names.
func (c *Completer) completeTools(partial string) []Completion {
	var tools []string
	if c.ToolsFn != nil {
		tools = c.ToolsFn()
	} else {
		// Default tools
		tools = []string{
			"Read", "Write", "Edit", "Glob", "Grep", "Bash",
		}
	}

	return c.completeFromList(tools, partial)
}

// completeConfig returns completions for config keys.
func (c *Completer) completeConfig(partial string) []Completion {
	var keys []string
	if c.ConfigFn != nil {
		keys = c.ConfigFn()
	} else {
		// Default config keys
		keys = []string{
			"model", "mode", "temperature", "max_tokens",
			"timeout", "autosave", "theme",
		}
	}

	return c.completeFromList(keys, partial)
}

// completeFromList returns completions from a list of strings.
func (c *Completer) completeFromList(values []string, partial string) []Completion {
	var completions []Completion

	partial = strings.ToLower(partial)

	for _, value := range values {
		if strings.HasPrefix(strings.ToLower(value), partial) {
			score := calculateScore(value, partial)
			completions = append(completions, Completion{
				Value:       value,
				Display:     value,
				Description: "",
				Score:       score,
			})
		}
	}

	sortCompletions(completions)
	return completions
}

// =============================================================================
// MENTION COMPLETION
// =============================================================================

// completeMentions handles completion for @ mentions.
func (c *Completer) completeMentions(input string) []Completion {
	// Find the last @ in the input
	lastAt := strings.LastIndex(input, "@")
	if lastAt == -1 {
		return nil
	}

	// Get the partial mention
	partial := input[lastAt:]

	// Check if we're completing a file mention
	if strings.HasPrefix(partial, "@file:") {
		pathPart := strings.TrimPrefix(partial, "@file:")
		// Handle quoted paths
		pathPart = strings.Trim(pathPart, "\"")
		files := c.completeFiles(pathPart)

		// Prepend @file: to the values
		for i := range files {
			files[i].Value = "@file:" + files[i].Value
			files[i].Display = "@file:" + files[i].Display
		}
		return files
	}

	// Complete mention types
	mentionTypes := []struct {
		value string
		desc  string
	}{
		{"@file:", "Include file content"},
		{"@clipboard", "Include clipboard content"},
		{"@git", "Include recent git info"},
		{"@codebase", "Include directory structure"},
		{"@error", "Include last error message"},
	}

	var completions []Completion
	partial = strings.ToLower(partial)

	for _, m := range mentionTypes {
		if strings.HasPrefix(strings.ToLower(m.value), partial) {
			score := calculateScore(m.value, partial)
			completions = append(completions, Completion{
				Value:       m.value,
				Display:     m.value,
				Description: m.desc,
				Score:       score,
			})
		}
	}

	sortCompletions(completions)
	return completions
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// calculateScore calculates a match score for completion ranking.
// Higher score = better match.
func calculateScore(value, partial string) int {
	value = strings.ToLower(value)
	partial = strings.ToLower(partial)

	score := 100

	// Exact match
	if value == partial {
		return score + 100
	}

	// Prefix match bonus
	if strings.HasPrefix(value, partial) {
		score += 50
		// Bonus for shorter completions
		score += 20 - len(value)
	}

	// Length penalty
	score -= len(value) / 2

	return score
}

// sortCompletions sorts completions by score (descending), then alphabetically.
func sortCompletions(completions []Completion) {
	sort.Slice(completions, func(i, j int) bool {
		if completions[i].Score != completions[j].Score {
			return completions[i].Score > completions[j].Score
		}
		return completions[i].Value < completions[j].Value
	})
}

// truncate truncates a string to maxLen characters.
// Uses rune-based truncation to handle Unicode correctly.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

// formatFileSize formats a file size in human-readable form.
func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return formatSizeNum(float64(size)/GB) + " GB"
	case size >= MB:
		return formatSizeNum(float64(size)/MB) + " MB"
	case size >= KB:
		return formatSizeNum(float64(size)/KB) + " KB"
	default:
		return formatSizeInt(size) + " B"
	}
}

func formatSizeNum(f float64) string {
	whole := int64(f)
	frac := int64((f - float64(whole)) * 10)
	if frac == 0 {
		return formatSizeInt(whole)
	}
	return formatSizeInt(whole) + "." + formatSizeInt(frac)
}

func formatSizeInt(n int64) string {
	if n == 0 {
		return "0"
	}

	var digits []byte
	negative := n < 0
	if negative {
		n = -n
	}

	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

// =============================================================================
// COMPLETION NAVIGATION
// =============================================================================

// CompletionState holds the state for navigating completions.
type CompletionState struct {
	// Original input before completion
	OriginalInput string

	// Current completions
	Completions []Completion

	// Selected index (-1 for none)
	Selected int

	// Visible indicates if completions should be shown
	Visible bool
}

// NewCompletionState creates a new completion state.
func NewCompletionState() *CompletionState {
	return &CompletionState{
		Selected: -1,
	}
}

// Update updates the completion state with new completions.
func (cs *CompletionState) Update(input string, completions []Completion) {
	cs.OriginalInput = input
	cs.Completions = completions
	cs.Selected = 0 // Changed from -1 to auto-select first
	cs.Visible = len(completions) > 0
}

// Next moves to the next completion.
func (cs *CompletionState) Next() {
	if len(cs.Completions) == 0 {
		return
	}
	cs.Selected = (cs.Selected + 1) % len(cs.Completions)
}

// Prev moves to the previous completion.
func (cs *CompletionState) Prev() {
	if len(cs.Completions) == 0 {
		return
	}
	cs.Selected--
	if cs.Selected < 0 {
		cs.Selected = len(cs.Completions) - 1
	}
}

// Accept returns the selected completion value, or empty if none selected.
func (cs *CompletionState) Accept() string {
	if cs.Selected < 0 || cs.Selected >= len(cs.Completions) {
		if len(cs.Completions) > 0 {
			return cs.Completions[0].Value
		}
		return ""
	}
	return cs.Completions[cs.Selected].Value
}

// Clear clears the completion state.
func (cs *CompletionState) Clear() {
	cs.OriginalInput = ""
	cs.Completions = nil
	cs.Selected = -1
	cs.Visible = false
}

// GetSelected returns the currently selected completion, or nil.
func (cs *CompletionState) GetSelected() *Completion {
	if cs.Selected < 0 || cs.Selected >= len(cs.Completions) {
		return nil
	}
	return &cs.Completions[cs.Selected]
}
