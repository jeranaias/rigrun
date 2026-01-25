// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides the visual UI components for rigrun TUI.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// ACTIVE CONTEXT TYPES
// =============================================================================

// ContextItem represents a single context item (@mention) in the active context.
type ContextItem struct {
	Type       string // "file", "git", "clipboard", "codebase", "error", "url"
	Path       string // For file mentions
	DisplayName string // Short display name (e.g., "main.go" from "@file:src/main.go")
	Tokens     int    // Estimated token count
	IsPinned   bool   // If true, persists across messages
	IsExpanded bool   // For future UI expansion features
}

// ActiveContext tracks the current context state for display.
type ActiveContext struct {
	Items       []ContextItem
	Pinned      []ContextItem // Persist across messages
	TotalTokens int
}

// NewActiveContext creates a new empty ActiveContext.
func NewActiveContext() *ActiveContext {
	return &ActiveContext{
		Items:       []ContextItem{},
		Pinned:      []ContextItem{},
		TotalTokens: 0,
	}
}

// AddItem adds a context item to the active context.
func (ac *ActiveContext) AddItem(item ContextItem) {
	// Check for duplicates
	for _, existing := range ac.Items {
		if existing.Type == item.Type && existing.Path == item.Path {
			return // Already exists
		}
	}
	ac.Items = append(ac.Items, item)
	ac.TotalTokens += item.Tokens
}

// RemoveItem removes a context item by index.
func (ac *ActiveContext) RemoveItem(index int) {
	if index < 0 || index >= len(ac.Items) {
		return
	}
	item := ac.Items[index]
	ac.TotalTokens -= item.Tokens
	ac.Items = append(ac.Items[:index], ac.Items[index+1:]...)
}

// PinItem marks an item as pinned.
func (ac *ActiveContext) PinItem(index int) {
	if index < 0 || index >= len(ac.Items) {
		return
	}
	ac.Items[index].IsPinned = true
	ac.Pinned = append(ac.Pinned, ac.Items[index])
	// Remove from Items to avoid duplication
	ac.Items = append(ac.Items[:index], ac.Items[index+1:]...)
}

// UnpinItem removes an item from the pinned list.
func (ac *ActiveContext) UnpinItem(index int) {
	if index < 0 || index >= len(ac.Pinned) {
		return
	}
	item := ac.Pinned[index]
	item.IsPinned = false
	ac.Pinned = append(ac.Pinned[:index], ac.Pinned[index+1:]...)

	// Also update in main items list if present
	for i := range ac.Items {
		if ac.Items[i].Path == item.Path && ac.Items[i].Type == item.Type {
			ac.Items[i].IsPinned = false
		}
	}
}

// Clear removes all non-pinned items.
func (ac *ActiveContext) Clear() {
	// Recalculate total from pinned items only
	ac.TotalTokens = 0
	for _, item := range ac.Pinned {
		ac.TotalTokens += item.Tokens
	}
	ac.Items = make([]ContextItem, len(ac.Pinned))
	copy(ac.Items, ac.Pinned)
}

// GetActiveItems returns only the non-pinned items.
func (ac *ActiveContext) GetActiveItems() []ContextItem {
	active := []ContextItem{}
	for _, item := range ac.Items {
		if !item.IsPinned {
			active = append(active, item)
		}
	}
	return active
}

// HasItems returns true if there are any context items.
func (ac *ActiveContext) HasItems() bool {
	return len(ac.Items) > 0
}

// =============================================================================
// CONTEXT BAR COMPONENT
// =============================================================================

// ContextBar renders the active context indicator.
type ContextBar struct {
	context  *ActiveContext
	width    int
	expanded bool // For future expansion feature
}

// NewContextBar creates a new context bar component.
func NewContextBar() *ContextBar {
	return &ContextBar{
		context:  NewActiveContext(),
		width:    80,
		expanded: false,
	}
}

// SetContext updates the active context.
func (cb *ContextBar) SetContext(ctx *ActiveContext) {
	cb.context = ctx
}

// SetWidth updates the width of the context bar.
func (cb *ContextBar) SetWidth(width int) {
	cb.width = width
}

// SetExpanded toggles the expanded view.
func (cb *ContextBar) SetExpanded(expanded bool) {
	cb.expanded = expanded
}

// RenderCompact renders a compact context indicator for status bar.
// Format: "Context: @file:main.go +2.5k | @git +500 | Total: ~3k tokens"
func (cb *ContextBar) RenderCompact() string {
	if cb.context == nil || !cb.context.HasItems() {
		return ""
	}

	parts := []string{}

	// Limit to first 2-3 items to keep it compact
	maxItems := 3
	itemCount := len(cb.context.Items)
	displayCount := itemCount
	if displayCount > maxItems {
		displayCount = maxItems
	}

	for i := 0; i < displayCount; i++ {
		item := cb.context.Items[i]
		parts = append(parts, cb.formatItemCompact(item))
	}

	// Add "... +N more" if there are more items
	if itemCount > maxItems {
		remaining := itemCount - maxItems
		parts = append(parts, fmt.Sprintf("... +%d more", remaining))
	}

	// Add total
	totalStr := formatTokenCount(cb.context.TotalTokens)

	result := strings.Join(parts, " | ")
	if result != "" {
		result = "Context: " + result + " | Total: ~" + totalStr
	}

	// Add MaxWidth constraint to prevent overflow on narrow terminals
	maxWidth := cb.width - 4
	if maxWidth < 10 {
		maxWidth = 10
	}

	resultRunes := []rune(result)
	if len(resultRunes) > maxWidth {
		result = string(resultRunes[:maxWidth-3]) + "..."
	}

	return lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Render(result)
}

// RenderInline renders an inline context indicator (super compact for narrow spaces).
// Format: "@file +2.5k | @git +500"
func (cb *ContextBar) RenderInline() string {
	if cb.context == nil || !cb.context.HasItems() {
		return ""
	}

	parts := []string{}

	// Show only first 2 items for inline
	maxItems := 2
	itemCount := len(cb.context.Items)
	displayCount := itemCount
	if displayCount > maxItems {
		displayCount = maxItems
	}

	for i := 0; i < displayCount; i++ {
		item := cb.context.Items[i]
		parts = append(parts, cb.formatItemInline(item))
	}

	// Add indicator if more items exist
	if itemCount > maxItems {
		parts = append(parts, "...")
	}

	result := strings.Join(parts, " | ")

	return lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Render(result)
}

// RenderExpanded renders the expanded context view with all items.
// This is displayed when user hovers or presses a key to expand.
func (cb *ContextBar) RenderExpanded() string {
	if cb.context == nil || !cb.context.HasItems() {
		return lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Render("No active context")
	}

	var lines []string

	// Header
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Purple).
		Render("Active Context")
	lines = append(lines, header)
	lines = append(lines, strings.Repeat("-", 40))

	// Active items (non-pinned)
	activeItems := cb.context.GetActiveItems()
	if len(activeItems) > 0 {
		for i, item := range activeItems {
			lines = append(lines, cb.formatItemExpanded(item, i, false))
		}
	}

	// Separator if both active and pinned exist
	if len(activeItems) > 0 && len(cb.context.Pinned) > 0 {
		lines = append(lines, strings.Repeat("-", 40))
	}

	// Pinned items
	if len(cb.context.Pinned) > 0 {
		pinnedHeader := lipgloss.NewStyle().
			Bold(true).
			Foreground(styles.Amber).
			Render("[PIN] Pinned:")
		lines = append(lines, pinnedHeader)

		for i, item := range cb.context.Pinned {
			lines = append(lines, cb.formatItemExpanded(item, i, true))
		}
	}

	// Footer with totals
	lines = append(lines, strings.Repeat("-", 40))
	totalLine := fmt.Sprintf("Total: ~%s tokens", formatTokenCount(cb.context.TotalTokens))

	// Add estimated cost if using cloud (assume Sonnet tier for estimation)
	// This is a rough estimate - actual cost depends on routing tier
	estimatedCost := float64(cb.context.TotalTokens) / 1000.0 * 0.3 // 0.3 cents per 1K tokens (Sonnet input)
	if estimatedCost > 0.01 {
		costStr := formatContextCost(estimatedCost)
		totalLine += fmt.Sprintf(" | Est. cost: ~%s (if cloud)", costStr)
	}

	lines = append(lines, lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Render(totalLine))

	// Combine all lines
	content := strings.Join(lines, "\n")

	// Wrap in a border
	width := cb.width - 4
	if width < 10 {
		width = 10
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Purple).
		Padding(1, 2).
		Width(width).
		Render(content)

	return box
}

// =============================================================================
// FORMATTING HELPERS
// =============================================================================

// formatItemCompact formats a context item for compact display.
// Format: "@file:main.go +2.5k"
func (cb *ContextBar) formatItemCompact(item ContextItem) string {
	icon := getContextIcon(item.Type)
	name := item.DisplayName
	if name == "" {
		name = item.Path
	}
	// Truncate long names
	if len([]rune(name)) > 15 {
		nameRunes := []rune(name)
		name = string(nameRunes[:12]) + "..."
	}

	tokenStr := formatTokenCount(item.Tokens)

	// Add pin indicator (ASCII)
	pinIndicator := ""
	if item.IsPinned {
		pinIndicator = "[PIN] "
	}

	return lipgloss.NewStyle().
		Foreground(getContextColor(item.Type)).
		Render(fmt.Sprintf("%s%s%s +%s", pinIndicator, icon, name, tokenStr))
}

// formatItemInline formats a context item for inline display (super compact).
// Format: "@file +2.5k"
func (cb *ContextBar) formatItemInline(item ContextItem) string {
	icon := getContextIcon(item.Type)
	tokenStr := formatTokenCount(item.Tokens)
	name := item.DisplayName
	if name == "" {
		name = item.Type
	}

	return lipgloss.NewStyle().
		Foreground(getContextColor(item.Type)).
		Render(fmt.Sprintf("%s%s +%s", icon, name, tokenStr))
}

// formatItemExpanded formats a context item for expanded display.
// Format: "[F] main.go         2,500 tokens [x]"
func (cb *ContextBar) formatItemExpanded(item ContextItem, index int, isPinned bool) string {
	icon := getContextIconEmoji(item.Type)
	name := item.DisplayName
	if name == "" {
		name = item.Path
	}

	// Pad name to align token counts
	namePadded := padRight(name, 20)

	tokenStr := formatTokenCountLong(item.Tokens)

	// Action button indicator
	actionBtn := "[x]"
	if isPinned {
		actionBtn = "[-]" // Different icon for unpinning
	}

	nameStyle := lipgloss.NewStyle().Foreground(getContextColor(item.Type))
	tokenStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	actionStyle := lipgloss.NewStyle().Foreground(styles.Rose)

	return fmt.Sprintf("%s %s %s %s",
		nameStyle.Render(icon),
		nameStyle.Render(namePadded),
		tokenStyle.Render(tokenStr),
		actionStyle.Render(actionBtn))
}

// getContextIcon returns the icon for a context type (compact).
func getContextIcon(contextType string) string {
	switch contextType {
	case "file":
		return "@file:"
	case "git":
		return "@git"
	case "clipboard":
		return "@clipboard"
	case "codebase":
		return "@codebase"
	case "error":
		return "@error"
	case "url":
		return "@url:"
	default:
		return "@"
	}
}

// getContextIconEmoji returns the ASCII icon for a context type (expanded view).
func getContextIconEmoji(contextType string) string {
	switch contextType {
	case "file":
		return "[F]"
	case "git":
		return "[G]"
	case "clipboard":
		return "[C]"
	case "codebase":
		return "[CB]"
	case "error":
		return "[!]"
	case "url":
		return "[U]"
	default:
		return "[@]"
	}
}

// getContextColor returns the color for a context type.
func getContextColor(contextType string) lipgloss.AdaptiveColor {
	switch contextType {
	case "file":
		return styles.Cyan
	case "git":
		return styles.Emerald
	case "clipboard":
		return styles.Purple
	case "codebase":
		return styles.Amber
	case "error":
		return styles.Rose
	case "url":
		return styles.Cyan
	default:
		return styles.TextMuted
	}
}

// formatTokenCount formats a token count compactly.
// Returns "2.5k" for 2500, "500" for 500, etc.
func formatTokenCount(tokens int) string {
	if tokens < 1000 {
		return fmt.Sprintf("%d tok", tokens)
	}
	if tokens < 10000 {
		return fmt.Sprintf("%.1fk tok", float64(tokens)/1000.0)
	}
	return fmt.Sprintf("%dk tok", tokens/1000)
}

// formatTokenCountLong formats a token count with full details.
// Returns "2,500 tokens" with comma separators.
func formatTokenCountLong(tokens int) string {
	return fmt.Sprintf("%s tokens", formatNumberWithCommas(tokens))
}

// formatContextCost formats a cost in cents for display.
func formatContextCost(cents float64) string {
	if cents < 1.0 {
		return fmt.Sprintf("%.2fc", cents)
	}
	if cents < 100.0 {
		return fmt.Sprintf("%.1fc", cents)
	}
	// Convert to dollars for larger amounts
	return fmt.Sprintf("$%.2f", cents/100.0)
}

// formatNumberWithCommas formats a number with thousand separators.
func formatNumberWithCommas(n int) string {
	if n < 0 {
		return "-" + formatNumberWithCommas(-n)
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	s := fmt.Sprintf("%d", n)
	var result []rune
	count := 0

	// Process from right to left
	for i := len(s) - 1; i >= 0; i-- {
		if count > 0 && count%3 == 0 {
			result = append([]rune{','}, result...)
		}
		result = append([]rune{rune(s[i])}, result...)
		count++
	}

	return string(result)
}

// padRight pads a string to the specified length with spaces.
func padRight(s string, length int) string {
	runes := []rune(s)
	if len(runes) >= length {
		return string(runes[:length])
	}
	padding := strings.Repeat(" ", length-len(runes))
	return s + padding
}

// =============================================================================
// HELPER FUNCTIONS FOR CREATING CONTEXT ITEMS
// =============================================================================

// CreateContextItemFromMention creates a ContextItem from a mention string and token estimate.
func CreateContextItemFromMention(mentionType, path string, tokens int) ContextItem {
	displayName := path

	// Extract filename for file mentions
	if mentionType == "file" && path != "" {
		// Get last part of path (filename)
		parts := strings.Split(path, "/")
		if len(parts) > 0 {
			displayName = parts[len(parts)-1]
		}
		// Also handle Windows paths
		parts = strings.Split(displayName, "\\")
		if len(parts) > 0 {
			displayName = parts[len(parts)-1]
		}
	}

	return ContextItem{
		Type:        mentionType,
		Path:        path,
		DisplayName: displayName,
		Tokens:      tokens,
		IsPinned:    false,
		IsExpanded:  false,
	}
}
