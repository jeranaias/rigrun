// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
//
// This file contains all rendering logic for the chat interface, including:
//   - Main view rendering (renderChat)
//   - Message rendering (user, assistant, system, tool messages)
//   - UI components (header, status bar, input area, search bar)
//   - Code block processing and syntax highlighting
//   - Search term highlighting with Unicode support
//   - Context bars and routing information display
//
// All helper functions for formatting and text utilities have been moved to utils.go.
package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/model"
	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/ui/components"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// MAIN RENDER
// =============================================================================

// renderChat renders the complete chat view.
// Layout: header (1 line) + [search bar (1 line)] + messages (viewport) + input (3 lines) + status (1 line)
// Total height must equal m.height exactly to prevent overflow/underflow.
//
// COUPLING WARNING: The viewport height is pre-calculated in handleResize() (model.go)
// using conservative constant estimates. This function measures actual heights with
// lipgloss.Height() and has a fallback if there's a mismatch. If you change the height
// of any component here, also update the constants in handleResize().
func (m Model) renderChat() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// If help overlay is active, render it instead of normal UI
	if m.showHelp {
		return m.renderHelpOverlay()
	}

	// Build fixed-height components first to calculate available space
	header := m.renderHeader()
	input := m.renderInput()
	status := m.renderStatusBar()

	// Render search bar if in search mode
	var searchBar string
	if m.searchMode {
		searchBar = m.renderSearchBar()
	}

	// Render progress indicator if active
	var progressBar string
	if m.IsShowingProgress() {
		progressBar = m.renderProgressIndicator()
	}

	// Calculate exact heights
	headerHeight := lipgloss.Height(header)
	inputHeight := lipgloss.Height(input)
	statusHeight := lipgloss.Height(status)
	searchBarHeight := lipgloss.Height(searchBar)
	progressBarHeight := lipgloss.Height(progressBar)

	// Calculate available height for messages viewport
	// This MUST match the viewport's configured height
	availableHeight := m.height - headerHeight - inputHeight - statusHeight - searchBarHeight - progressBarHeight
	if availableHeight < 1 {
		availableHeight = 1
	}

	// Get viewport content - viewport should already be sized correctly
	// via SetHeight in the Update function. We trust the viewport's height.
	messages := m.viewport.View()

	// Verify viewport height matches available space to catch sizing bugs
	viewportRenderedHeight := lipgloss.Height(messages)
	if viewportRenderedHeight != availableHeight {
		// Viewport height mismatch - force correct height to prevent layout breakage
		// This is a fallback; the root cause should be fixed in Update()
		messages = lipgloss.NewStyle().
			Height(availableHeight).
			MaxHeight(availableHeight).
			Width(m.width).
			Render(messages)
	}

	// Stack vertically - order is critical:
	// 1. Header at top
	// 2. Search bar (if in search mode)
	// 3. Progress bar (if showing progress)
	// 4. Messages area (scrollable viewport)
	// 5. Input area (separator + input + char count)
	// 6. Status bar at bottom
	var baseView string

	// Build the appropriate stack based on what's visible
	if m.IsShowingProgress() && m.searchMode {
		baseView = lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			searchBar,
			progressBar,
			messages,
			input,
			status,
		)
	} else if m.IsShowingProgress() {
		baseView = lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			progressBar,
			messages,
			input,
			status,
		)
	} else if m.searchMode {
		baseView = lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			searchBar,
			messages,
			input,
			status,
		)
	} else {
		baseView = lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			messages,
			input,
			status,
		)
	}

	// Render command palette overlay on top if visible
	if m.commandPalette != nil && m.commandPalette.IsVisible() {
		m.commandPalette.SetSize(m.width, m.height)
		paletteView := m.commandPalette.View()
		// Layer palette on top of base view
		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Left, lipgloss.Top,
			baseView+"\n"+paletteView,
		)
	}

	// Render tutorial overlay if visible (highest priority overlay)
	if m.IsTutorialVisible() && m.tutorial != nil {
		m.tutorial.SetSize(m.width, m.height)
		tutorialView := m.tutorial.View()
		// Layer tutorial on top of base view
		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Left, lipgloss.Top,
			baseView+"\n"+tutorialView,
		)
	}

	// Render non-blocking error toasts as overlay (lazygit-inspired)
	// Toasts appear in bottom-right corner and don't block UI interaction
	if m.HasToasts() {
		toasts := m.GetToasts()
		toastOverlay := components.RenderToastStack(toasts, m.width, m.height)
		// Overlay toasts on the base view
		return m.overlayToasts(baseView, toastOverlay)
	}

	return baseView
}

// overlayToasts renders toasts on top of the base view.
// Toasts are positioned in the bottom-right corner without blocking interaction.
func (m Model) overlayToasts(baseView, toastView string) string {
	// Split base view into lines
	baseLines := strings.Split(baseView, "\n")
	toastLines := strings.Split(toastView, "\n")

	// Calculate toast dimensions
	toastHeight := len(toastLines)
	toastWidth := 0
	for _, line := range toastLines {
		if w := lipgloss.Width(line); w > toastWidth {
			toastWidth = w
		}
	}

	// Overlay toast in bottom-right corner
	// Start overlaying from (height - toastHeight - 2) to leave space for status bar
	startRow := m.height - toastHeight - 2
	if startRow < 0 {
		startRow = 0
	}

	// Build the result by overlaying toast lines on base lines
	result := make([]string, len(baseLines))
	for i, baseLine := range baseLines {
		toastLineIdx := i - startRow
		if toastLineIdx >= 0 && toastLineIdx < len(toastLines) {
			// This row has toast content - position it to the right
			toastLine := toastLines[toastLineIdx]
			if lipgloss.Width(toastLine) > 0 {
				baseWidth := lipgloss.Width(baseLine)
				toastLineWidth := lipgloss.Width(toastLine)

				// Pad base line to full width if needed
				if baseWidth < m.width-toastLineWidth-1 {
					baseLine = baseLine + strings.Repeat(" ", m.width-toastLineWidth-1-baseWidth)
				}

				// Truncate base line to make room for toast
				if baseWidth > m.width-toastLineWidth-1 {
					// Need to truncate - find where to cut
					cutPoint := m.width - toastLineWidth - 1
					if cutPoint > 0 {
						baseLine = truncateToWidth(baseLine, cutPoint)
					}
				}

				// Combine base and toast
				result[i] = baseLine + toastLine
			} else {
				result[i] = baseLine
			}
		} else {
			result[i] = baseLine
		}
	}

	return strings.Join(result, "\n")
}

// truncateToWidth truncates a string to fit within a given visible width.
func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}

	currentWidth := 0
	var result strings.Builder

	for _, r := range s {
		runeWidth := lipgloss.Width(string(r))
		if currentWidth+runeWidth > width {
			break
		}
		result.WriteRune(r)
		currentWidth += runeWidth
	}

	return result.String()
}

// renderSearchBar renders the search input bar with match count and navigation hints.
// The bar is styled with an amber background to visually distinguish it from the main content.
func (m Model) renderSearchBar() string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	// Search input
	searchInputView := m.searchInput.View()

	// Match count indicator
	var matchInfo string
	if m.searchQuery != "" {
		if len(m.searchMatches) == 0 {
			matchInfo = lipgloss.NewStyle().
				Foreground(styles.Rose).
				Render(" No matches")
		} else {
			matchInfo = lipgloss.NewStyle().
				Foreground(styles.Emerald).
				Render(fmt.Sprintf(" %d/%d", m.searchMatchIndex+1, len(m.searchMatches)))
		}
	}

	// Help text
	helpText := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Render(" | Enter/Down=next | Up=prev | Esc=close")

	// Combine search bar content
	searchContent := searchInputView + matchInfo + helpText

	// Create search bar with amber background to stand out
	searchBarStyle := lipgloss.NewStyle().
		Background(styles.SurfaceDim).
		Foreground(styles.Amber).
		Width(width).
		Padding(0, 1)

	return searchBarStyle.Render(searchContent)
}

// =============================================================================
// HEADER
// =============================================================================

// renderHeader renders the title bar with model name, status indicator, and offline mode badge.
// The header uses a dimmed surface background and is always 1 line high.
func (m Model) renderHeader() string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	// Title
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.Purple).
		Render("rigrun")

	// Model info
	modelInfo := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Render(" | " + m.modelName)

	// Status indicator
	var statusIcon string
	switch m.state {
	case StateStreaming:
		statusIcon = lipgloss.NewStyle().
			Foreground(styles.Emerald).
			Render(" " + styles.AnimationStatusIndicators.Connected)
	case StateError:
		statusIcon = lipgloss.NewStyle().
			Foreground(styles.Rose).
			Render(" " + styles.StatusIndicators.Error)
	default:
		statusIcon = lipgloss.NewStyle().
			Foreground(styles.Cyan).
			Render(" " + styles.StatusIndicators.Success)
	}

	// IL5 SC-7: Offline mode badge
	var offlineBadge string
	if m.offlineMode {
		offlineBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#FF0000")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1).
			Render("OFFLINE")
	}

	// Combine header content
	headerContent := title + modelInfo + statusIcon
	if offlineBadge != "" {
		headerContent = headerContent + " " + offlineBadge
	}

	// Create header bar
	header := lipgloss.NewStyle().
		Background(styles.SurfaceDim).
		Width(width).
		Padding(0, 1).
		Render(headerContent)

	return header
}

// =============================================================================
// MESSAGES
// =============================================================================

// renderMessages renders all messages in the conversation with appropriate styling.
// Returns an empty state message if the conversation is empty or nil.
func (m *Model) renderMessages() string {
	if m.conversation == nil || m.conversation.IsEmpty() {
		return m.renderEmptyState()
	}

	var parts []string
	messages := m.conversation.GetHistory()

	for i, msg := range messages {
		rendered := m.renderMessage(msg, i == len(messages)-1, i)
		parts = append(parts, rendered)
	}

	// Add thinking indicator if streaming
	if m.state == StateStreaming && m.isThinking {
		parts = append(parts, m.renderThinking())
	}

	// Join with consistent vertical spacing for readability
	// Use single blank line between messages for clean, professional look
	return strings.Join(parts, "\n")
}

// renderMessage renders a single message based on its role.
// Delegates to role-specific rendering functions for proper styling and layout.
func (m *Model) renderMessage(msg *model.Message, isLast bool, msgIndex int) string {
	switch msg.Role {
	case model.RoleUser:
		return m.renderUserMessage(msg, msgIndex)
	case model.RoleAssistant:
		return m.renderAssistantMessage(msg, isLast, msgIndex)
	case model.RoleSystem:
		return m.renderSystemMessage(msg, msgIndex)
	case model.RoleTool:
		return m.renderToolMessage(msg, msgIndex)
	default:
		return m.highlightSearchTerms(msg.GetDisplayContent(), msgIndex)
	}
}

// highlightSearchTerms highlights search terms in the given text.
// If in search mode and the text contains matches, wraps them in highlight styling.
// This function handles Unicode text correctly by working with runes for position
// calculations, ensuring proper handling of multi-byte characters (Chinese, emoji, etc.).
func (m *Model) highlightSearchTerms(text string, msgIndex int) string {
	if !m.searchMode || m.searchQuery == "" {
		return text
	}

	// Check if this message has any matches
	hasMatches := false
	for _, match := range m.searchMatches {
		if match.MessageIndex == msgIndex {
			hasMatches = true
			break
		}
	}
	if !hasMatches {
		return text
	}

	// Convert to runes for proper Unicode handling
	// This ensures we work with character positions, not byte positions
	textRunes := []rune(text)
	queryRunes := []rune(strings.ToLower(m.searchQuery))
	queryLen := len(queryRunes)

	if queryLen == 0 {
		return text
	}

	var result strings.Builder
	lastEnd := 0 // rune position

	for i := 0; i <= len(textRunes)-queryLen; i++ {
		// Check for case-insensitive match at position i
		matched := true
		for j := 0; j < queryLen; j++ {
			// Compare lowercase runes
			textRuneLower := []rune(strings.ToLower(string(textRunes[i+j])))[0]
			if textRuneLower != queryRunes[j] {
				matched = false
				break
			}
		}

		if matched {
			matchStart := i
			matchEnd := i + queryLen

			// Write text before match (from lastEnd to matchStart)
			if matchStart > lastEnd {
				result.WriteString(string(textRunes[lastEnd:matchStart]))
			}

			// Check if this is the current match (for special highlighting)
			// StartPos in searchMatches is stored as RUNE position for Unicode safety
			isCurrentMatch := false
			for idx, match := range m.searchMatches {
				if match.MessageIndex == msgIndex && match.StartPos == matchStart && idx == m.searchMatchIndex {
					isCurrentMatch = true
					break
				}
			}

			// Highlight the match
			matchText := string(textRunes[matchStart:matchEnd])
			if isCurrentMatch {
				// Current match: bright background with dark text for contrast
				highlighted := lipgloss.NewStyle().
					Background(styles.Amber).
					Foreground(styles.TextInverse).
					Bold(true).
					Render(matchText)
				result.WriteString(highlighted)
			} else {
				// Other matches: subtle highlight
				highlighted := lipgloss.NewStyle().
					Background(styles.SurfaceDim).
					Foreground(styles.Amber).
					Render(matchText)
				result.WriteString(highlighted)
			}

			lastEnd = matchEnd
			i = matchEnd - 1 // -1 because loop will increment
		}
	}

	// Write remaining text after last match
	if lastEnd < len(textRunes) {
		result.WriteString(string(textRunes[lastEnd:]))
	}

	return result.String()
}

// renderUserMessage renders a user message with blue styling and right alignment.
// User messages have rounded borders and are right-aligned to distinguish them from assistant messages.
func (m *Model) renderUserMessage(msg *model.Message, msgIndex int) string {
	maxWidth := m.width - 8
	if maxWidth > m.width-2 {
		maxWidth = m.width - 2 // Never exceed terminal
	}
	if maxWidth < 10 {
		maxWidth = 10 // Minimum takes precedence
	}

	content := msg.GetDisplayContent()

	// Apply search highlighting if in search mode
	content = m.highlightSearchTerms(content, msgIndex)

	// User bubble style - blue tones, right-aligned feel
	bubble := lipgloss.NewStyle().
		Foreground(styles.UserBubbleFg).
		Background(styles.UserBubbleBg).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.UserBubbleBorder).
		Padding(0, 2).
		MaxWidth(maxWidth)

	// Wrap content with safe width
	wrapWidth := maxWidth - 4
	if wrapWidth < 10 {
		wrapWidth = 10
	}
	rendered := bubble.Render(wrapText(content, wrapWidth))

	// Add margin to push right (user messages align right)
	marginLeft := m.width - lipgloss.Width(rendered) - 4
	if marginLeft < 0 {
		marginLeft = 0
	}

	withMargin := lipgloss.NewStyle().
		MarginLeft(marginLeft).
		MarginTop(1).
		MarginBottom(1).
		Render(rendered)

	// No role label needed - right alignment and color indicate user message
	return withMargin
}

// renderAssistantMessage renders an assistant message with purple styling.
// Includes code block processing, streaming cursor, statistics, and routing info.
func (m *Model) renderAssistantMessage(msg *model.Message, isLast bool, msgIndex int) string {
	maxWidth := m.width - 8
	if maxWidth > m.width-2 {
		maxWidth = m.width - 2 // Never exceed terminal
	}
	if maxWidth < 10 {
		maxWidth = 10 // Minimum takes precedence
	}

	content := msg.GetDisplayContent()

	// Skip rendering if no content yet (prevents empty bubble)
	if strings.TrimSpace(content) == "" && !msg.IsStreaming {
		return ""
	}

	// Apply search highlighting if in search mode (before adding cursor)
	content = m.highlightSearchTerms(content, msgIndex)

	// Add streaming cursor if this is the last message and streaming
	if msg.IsStreaming && m.state == StateStreaming {
		if content == "" {
			content = "_" // Show just cursor when no content yet
		} else {
			content += lipgloss.NewStyle().
				Foreground(styles.Purple).
				Blink(true).
				Render("_")
		}
	}

	// Process code blocks in the content
	processedContent := m.renderContentWithCodeBlocks(content, maxWidth)

	// Add statistics line if complete
	var statsLine string
	if !msg.IsStreaming && msg.TotalDuration > 0 {
		statsLine = "\n" + m.renderStats(msg)
	}

	// Add routing info line if available
	var routingLine string
	if !msg.IsStreaming && msg.RoutingTier != "" {
		routingLine = "\n" + m.renderRoutingInfo(msg)
	}

	// Wrap in consistent margin for clean spacing
	result := processedContent + statsLine + routingLine
	return lipgloss.NewStyle().
		MarginTop(1).
		MarginBottom(1).
		MarginLeft(2).
		Render(result)
}

// renderContentWithCodeBlocks processes content and renders code blocks separately.
func (m *Model) renderContentWithCodeBlocks(content string, maxWidth int) string {
	// Calculate safe wrap width to avoid negative values
	wrapWidth := maxWidth - 4
	if wrapWidth < 10 {
		wrapWidth = 10
	}

	// Check if content has code blocks
	if !strings.Contains(content, "```") {
		// No code blocks - render as normal assistant bubble
		bubble := lipgloss.NewStyle().
			Foreground(styles.AssistantBubbleFg).
			Background(styles.AssistantBubbleBg).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(styles.AssistantBubbleBorder).
			Padding(0, 2).
			MaxWidth(maxWidth)
		return bubble.Render(wrapText(content, wrapWidth))
	}

	// Has code blocks - split and render each part
	var parts []string
	lines := strings.Split(content, "\n")
	var currentText []string
	var inCodeBlock bool
	var codeLines []string
	var language string

	textBubble := lipgloss.NewStyle().
		Foreground(styles.AssistantBubbleFg).
		Background(styles.AssistantBubbleBg).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.AssistantBubbleBorder).
		Padding(0, 2).
		MaxWidth(maxWidth)

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block - render it
				if len(currentText) > 0 {
					text := strings.Join(currentText, "\n")
					if strings.TrimSpace(text) != "" {
						parts = append(parts, textBubble.Render(wrapText(text, wrapWidth)))
					}
					currentText = nil
				}

				// Render code block
				code := strings.Join(codeLines, "\n")
				cb := components.NewCodeBlock(language, code)
				cb.SetMaxWidth(maxWidth)
				parts = append(parts, cb.Render())

				codeLines = nil
				language = ""
				inCodeBlock = false
			} else {
				// Start of code block
				if len(currentText) > 0 {
					text := strings.Join(currentText, "\n")
					if strings.TrimSpace(text) != "" {
						parts = append(parts, textBubble.Render(wrapText(text, wrapWidth)))
					}
					currentText = nil
				}

				language = strings.TrimPrefix(line, "```")
				language = strings.TrimSpace(language)
				inCodeBlock = true
			}
		} else if inCodeBlock {
			codeLines = append(codeLines, line)
		} else {
			currentText = append(currentText, line)
		}
	}

	// Handle remaining content
	if len(currentText) > 0 {
		text := strings.Join(currentText, "\n")
		if strings.TrimSpace(text) != "" {
			parts = append(parts, textBubble.Render(wrapText(text, wrapWidth)))
		}
	}

	// Handle unclosed code block
	if inCodeBlock {
		if len(codeLines) > 0 {
			// Render as code block
			code := strings.Join(codeLines, "\n")
			cb := components.NewCodeBlock(language, code)
			cb.SetMaxWidth(maxWidth)
			parts = append(parts, cb.Render())
		} else {
			// Just an opening marker with no content - render as text
			text := "```" + language
			if strings.TrimSpace(text) != "" {
				parts = append(parts, textBubble.Render(text))
			}
		}
	}

	return strings.Join(parts, "\n")
}

// renderSystemMessage renders a system message with amber styling.
func (m *Model) renderSystemMessage(msg *model.Message, msgIndex int) string {
	maxWidth := m.width - 8
	if maxWidth > m.width-2 {
		maxWidth = m.width - 2 // Never exceed terminal
	}
	if maxWidth < 10 {
		maxWidth = 10 // Minimum takes precedence
	}

	content := msg.GetDisplayContent()

	// Apply search highlighting if in search mode
	content = m.highlightSearchTerms(content, msgIndex)

	// System bubble style - amber tones, centered
	// Double border indicates system message
	bubble := lipgloss.NewStyle().
		Foreground(styles.SystemBubbleFg).
		Background(styles.SystemBubbleBg).
		BorderStyle(lipgloss.DoubleBorder()).
		BorderForeground(styles.SystemBubbleBorder).
		Padding(0, 2).
		MaxWidth(maxWidth).
		Align(lipgloss.Center)

	// Calculate safe wrap width to avoid negative values
	wrapWidth := maxWidth - 4
	if wrapWidth < 10 {
		wrapWidth = 10
	}
	rendered := bubble.Render(wrapText(content, wrapWidth))

	// Wrap with consistent margins
	return lipgloss.NewStyle().
		MarginTop(1).
		MarginBottom(1).
		Render(rendered)
}

// renderToolMessage renders a tool result message.
func (m *Model) renderToolMessage(msg *model.Message, msgIndex int) string {
	maxWidth := m.width - 8
	if maxWidth > m.width-2 {
		maxWidth = m.width - 2 // Never exceed terminal
	}
	if maxWidth < 10 {
		maxWidth = 10 // Minimum takes precedence
	}

	content := msg.GetDisplayContent()

	// Apply search highlighting if in search mode
	content = m.highlightSearchTerms(content, msgIndex)

	var bubble lipgloss.Style
	if msg.IsSuccess {
		// Success style - emerald
		bubble = lipgloss.NewStyle().
			Foreground(styles.ToolSuccessFg).
			Background(styles.ToolSuccessBg).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(styles.Emerald).
			BorderLeft(true).
			PaddingLeft(2).
			MaxWidth(maxWidth)
	} else {
		// Error style - rose
		bubble = lipgloss.NewStyle().
			Foreground(styles.ToolErrorFg).
			Background(styles.ToolErrorBg).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(styles.Rose).
			BorderLeft(true).
			PaddingLeft(2).
			MaxWidth(maxWidth)
	}

	// Tool label with ASCII icon
	var icon string
	if msg.IsSuccess {
		icon = "[OK]"
	} else {
		icon = "[ERR]"
	}

	roleLabel := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true).
		Render(icon + " " + msg.ToolName)

	// Calculate safe wrap width to avoid negative values
	wrapWidth := maxWidth - 4
	if wrapWidth < 10 {
		wrapWidth = 10
	}
	rendered := bubble.Render(wrapText(content, wrapWidth))

	// Wrap with consistent margins
	result := roleLabel + "\n" + rendered
	return lipgloss.NewStyle().
		MarginTop(1).
		MarginBottom(1).
		MarginLeft(2).
		Render(result)
}

// renderStats renders the statistics line for a message.
func (m *Model) renderStats(msg *model.Message) string {
	stats := msg.FormatStats()
	if stats == "" {
		return ""
	}

	return lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true).
		PaddingLeft(2).
		Render(stats)
}

// renderRoutingInfo renders routing information for a message.
// Shows tier used (icon + name), token count, and cost if cloud tier.
// Example: "* Cloud (Sonnet) - 234 tokens - 0.05c"
// Cache hits show: "⚡ Cached (Exact)"
func (m *Model) renderRoutingInfo(msg *model.Message) string {
	if msg.RoutingTier == "" {
		return ""
	}

	// Parse tier from string
	tier := parseTier(msg.RoutingTier)

	// Get icon for tier - use lightning bolt for cache hits
	icon, ok := components.TierIcons[tier]
	if !ok {
		icon = "?"
	}
	// Override icon for cache hits with lightning bolt
	if tier == router.TierCache {
		icon = "⚡"
	}

	// Build routing info string
	var parts []string

	// Tier icon and name
	tierStyle := m.getTierStyle(tier)
	tierStr := tierStyle.Render(icon + " " + msg.RoutingTier)
	parts = append(parts, tierStr)

	// Token count if available
	if msg.TokenCount > 0 {
		tokenStr := lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Render(fmt.Sprintf("%d tokens", msg.TokenCount))
		parts = append(parts, tokenStr)
	}

	// Cost if paid tier
	if tier.IsPaid() && msg.RoutingCost > 0 {
		costStr := lipgloss.NewStyle().
			Foreground(styles.Amber).
			Render(formatCost(msg.RoutingCost))
		parts = append(parts, costStr)
	}

	// Join with separator and right-align
	separator := lipgloss.NewStyle().
		Foreground(styles.Overlay).
		Render(" - ")

	info := strings.Join(parts, separator)

	// Right-align the routing info
	width := m.width
	if width <= 0 {
		width = 80
	}
	routingWidth := width - 4
	if routingWidth < 20 {
		routingWidth = 20
	}

	rightAligned := lipgloss.NewStyle().
		Width(routingWidth).
		Align(lipgloss.Right).
		Foreground(styles.TextMuted).
		Render(info)

	return rightAligned
}

// getTierStyle returns the appropriate style for a routing tier.
func (m *Model) getTierStyle(tier router.Tier) lipgloss.Style {
	switch tier {
	case router.TierCache:
		return lipgloss.NewStyle().Foreground(styles.Cyan).Bold(true)
	case router.TierLocal:
		return lipgloss.NewStyle().Foreground(styles.Emerald).Bold(true)
	case router.TierCloud, router.TierHaiku:
		return lipgloss.NewStyle().Foreground(styles.Amber).Bold(true)
	case router.TierSonnet:
		return lipgloss.NewStyle().Foreground(styles.Purple).Bold(true)
	case router.TierOpus:
		return lipgloss.NewStyle().Foreground(styles.Rose).Bold(true)
	case router.TierGpt4o:
		return lipgloss.NewStyle().Foreground(styles.Cyan).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(styles.TextMuted)
	}
}

// parseTier converts a tier string back to a Tier constant.
func parseTier(s string) router.Tier {
	if strings.HasPrefix(s, "Cache") {
		return router.TierCache
	}
	if strings.HasPrefix(s, "Local") {
		return router.TierLocal
	}
	if strings.HasPrefix(s, "Haiku") {
		return router.TierHaiku
	}
	if strings.HasPrefix(s, "Sonnet") {
		return router.TierSonnet
	}
	if strings.HasPrefix(s, "Opus") {
		return router.TierOpus
	}
	if strings.HasPrefix(s, "GPT-4o") {
		return router.TierGpt4o
	}
	if strings.HasPrefix(s, "Cloud") {
		return router.TierCloud
	}
	return router.TierLocal
}

// renderThinking renders the thinking indicator.
func (m *Model) renderThinking() string {
	return m.renderStreamingIndicator()
}

// renderStreamingIndicator renders the animated streaming indicator.
// Uses a spinner animation with ASCII-compatible frames.
func (m *Model) renderStreamingIndicator() string {
	// Use the built-in spinner if available
	spinner := m.spinner.View()

	// ASCII-compatible spinner frames (fallback if spinner not available)
	frames := []string{"|", "/", "-", "\\"}
	frameIndex := int(time.Now().UnixMilli()/100) % len(frames)
	frame := frames[frameIndex]

	// Use spinner view if it has content, otherwise use our frame
	if spinner == "" {
		spinner = frame
	}

	thinkingStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary)

	streamingStyle := lipgloss.NewStyle().
		Foreground(styles.Purple)

	text := thinkingStyle.Render("Thinking")
	dots := streamingStyle.Render("...")

	return streamingStyle.Render(spinner) + " " + text + dots
}

// renderEmptyState renders the empty conversation state with a welcoming interface.
// Shows: welcome message, current model, quick tips, example prompts, and help hint.
func (m *Model) renderEmptyState() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	emptyWidth := width - 8
	if emptyWidth < 40 {
		emptyWidth = 40 // Minimum for readable content
	}
	if emptyWidth > 80 {
		emptyWidth = 80 // Cap width for readability
	}

	var sb strings.Builder

	// Welcome header with model name
	welcomeStyle := lipgloss.NewStyle().
		Foreground(styles.Purple).
		Bold(true).
		Align(lipgloss.Center).
		Width(emptyWidth)
	sb.WriteString(welcomeStyle.Render("Welcome to rigrun"))
	sb.WriteString("\n\n")

	// Current model info
	modelStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary).
		Align(lipgloss.Center).
		Width(emptyWidth)
	modelName := m.modelName
	if modelName == "" {
		modelName = "No model selected"
	}
	sb.WriteString(modelStyle.Render("Model: " + modelName))
	sb.WriteString("\n\n")

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(styles.Overlay).
		Align(lipgloss.Center).
		Width(emptyWidth)
	sb.WriteString(sepStyle.Render(strings.Repeat("-", 40)))
	sb.WriteString("\n\n")

	// Quick tips section
	tipsHeaderStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan).
		Bold(true)
	sb.WriteString(tipsHeaderStyle.Render("Quick Tips"))
	sb.WriteString("\n\n")

	tipStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted)
	keyStyle := lipgloss.NewStyle().
		Foreground(styles.Amber).
		Bold(true)

	tips := []struct {
		key  string
		desc string
	}{
		{"Type a message", "Start chatting with the AI"},
		{"?", "Show keyboard shortcuts"},
		{"/help", "List available commands"},
		{"Ctrl+P", "Open command palette"},
		{"Ctrl+R", "Cycle routing mode (local/cloud/auto)"},
		{"@file:path", "Include file content in your message"},
	}

	for _, tip := range tips {
		line := fmt.Sprintf("  %s  %s",
			keyStyle.Render(fmt.Sprintf("%-16s", tip.key)),
			tipStyle.Render(tip.desc))
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Example prompts section
	examplesHeaderStyle := lipgloss.NewStyle().
		Foreground(styles.Emerald).
		Bold(true)
	sb.WriteString(examplesHeaderStyle.Render("Try asking"))
	sb.WriteString("\n\n")

	exampleStyle := lipgloss.NewStyle().
		Foreground(styles.TextSecondary).
		Italic(true)

	examples := []string{
		"\"Explain how goroutines work in Go\"",
		"\"Write a function to parse JSON\"",
		"\"Help me debug this error: @error\"",
		"\"Review this file: @file:main.go\"",
	}

	for _, example := range examples {
		sb.WriteString("  " + exampleStyle.Render(example))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Help hint at bottom
	hintStyle := lipgloss.NewStyle().
		Foreground(styles.Overlay).
		Align(lipgloss.Center).
		Width(emptyWidth)
	sb.WriteString(hintStyle.Render("Press ? for help | Ctrl+Q to quit"))

	// Wrap everything in a centered container
	containerStyle := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(width - 4).
		Padding(2, 0)

	return containerStyle.Render(sb.String())
}

// =============================================================================
// INPUT AREA
// =============================================================================

// renderInput renders the input area with focus ring indicator.
// The border color changes based on vim mode following lazygit's focus styling pattern:
// - Insert mode: bright green border (active editing)
// - Normal mode: dim gray border (navigation mode)
// - Command mode: amber border (command input)
// This provides clear visual feedback about the current editing mode.
func (m Model) renderInput() string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	// Determine focus state and border color based on vim mode
	var borderColor lipgloss.AdaptiveColor
	var modeLabel string
	var modeLabelStyle lipgloss.Style

	if m.vimHandler != nil && m.vimHandler.Enabled() {
		switch m.vimHandler.Mode() {
		case VimModeInsert:
			// Insert mode: bright green border - user is actively typing
			borderColor = styles.FocusRingInsert
			modeLabel = " INSERT "
			modeLabelStyle = lipgloss.NewStyle().
				Background(styles.FocusRingInsert).
				Foreground(styles.TextInverse).
				Bold(true)
		case VimModeNormal:
			// Normal mode: dim border - user is navigating
			borderColor = styles.FocusRingDim
			modeLabel = " NORMAL "
			modeLabelStyle = lipgloss.NewStyle().
				Background(styles.FocusRingDim).
				Foreground(styles.TextInverse).
				Bold(true)
		case VimModeVisual:
			// Visual mode: purple border - selection
			borderColor = styles.Purple
			modeLabel = " VISUAL "
			modeLabelStyle = lipgloss.NewStyle().
				Background(styles.Purple).
				Foreground(styles.TextInverse).
				Bold(true)
		case VimModeCommand:
			// Command mode: amber border - command input
			borderColor = styles.FocusRingCommand
			modeLabel = " COMMAND "
			modeLabelStyle = lipgloss.NewStyle().
				Background(styles.FocusRingCommand).
				Foreground(styles.TextInverse).
				Bold(true)
		default:
			borderColor = styles.FocusRing
			modeLabel = ""
		}
	} else if m.input.Focused() {
		// Non-vim mode, but input is focused
		borderColor = styles.FocusRing
		modeLabel = ""
	} else {
		// Input not focused
		borderColor = styles.FocusRingDim
		modeLabel = ""
	}

	// Render the mode label badge
	var modeBadge string
	if modeLabel != "" {
		modeBadge = modeLabelStyle.Render(modeLabel)
	}

	// Create top border with mode indicator integrated
	// Format: ─────[ INSERT ]─────────────────
	borderChar := "\u2500" // Unicode horizontal line
	if modeLabel != "" {
		// Calculate border segments around mode label
		labelWidth := lipgloss.Width(modeBadge)
		leftBorderWidth := 3
		rightBorderWidth := width - leftBorderWidth - labelWidth - 2
		if rightBorderWidth < 0 {
			rightBorderWidth = 0
		}

		leftBorder := lipgloss.NewStyle().
			Foreground(borderColor).
			Render(strings.Repeat(borderChar, leftBorderWidth))
		rightBorder := lipgloss.NewStyle().
			Foreground(borderColor).
			Render(strings.Repeat(borderChar, rightBorderWidth))

		modeBadge = leftBorder + modeBadge + rightBorder
	} else {
		// No mode label, just a colored border line
		modeBadge = lipgloss.NewStyle().
			Foreground(borderColor).
			Render(strings.Repeat(borderChar, width))
	}

	// Input view - the textinput handles its own prompt
	// In vim command mode, show command buffer instead
	var inputView string
	if m.vimHandler != nil && m.vimHandler.Mode() == VimModeCommand {
		cmdBuffer := m.vimHandler.GetCommandBuffer()
		inputView = lipgloss.NewStyle().
			Foreground(styles.Amber).
			Render(cmdBuffer)
	} else {
		inputView = m.input.View()
	}

	// Status indicator for streaming
	var statusIndicator string
	if m.state == StateStreaming {
		statusIndicator = lipgloss.NewStyle().
			Foreground(styles.Amber).
			Render(" (streaming...)")
	}

	// Combine input on one line with padding
	inputLineWidth := width - 4
	if inputLineWidth < 10 {
		inputLineWidth = 10
	}

	// Build input line content (no extra prompt - textinput has it)
	inputContent := inputView + statusIndicator

	// Add subtle left border indicator for vim mode
	var leftIndicator string
	if m.vimHandler != nil && m.vimHandler.Enabled() {
		switch m.vimHandler.Mode() {
		case VimModeInsert:
			leftIndicator = lipgloss.NewStyle().
				Foreground(styles.FocusRingInsert).
				Render("\u2503 ") // Bold vertical bar
		case VimModeNormal:
			leftIndicator = lipgloss.NewStyle().
				Foreground(styles.FocusRingDim).
				Render("\u2502 ") // Light vertical bar
		case VimModeCommand:
			leftIndicator = lipgloss.NewStyle().
				Foreground(styles.FocusRingCommand).
				Render("\u2503 ") // Bold vertical bar
		default:
			leftIndicator = "  "
		}
	} else {
		leftIndicator = "  "
	}

	inputLine := lipgloss.NewStyle().
		Width(inputLineWidth).
		Render(leftIndicator + inputContent)

	// Character count - right aligned, subtle
	charCount := m.renderCharCount()

	// Build the input area: mode border, input line, char count
	// Use fixed height of 3 to prevent layout shift when typing
	result := lipgloss.JoinVertical(
		lipgloss.Left,
		modeBadge,
		inputLine,
		charCount,
	)

	// Force exact height of 3 lines to prevent shrinking when user types
	return lipgloss.NewStyle().
		Height(3).
		MaxHeight(3).
		Width(width).
		Render(result)
}

// renderCharCount renders the character count indicator.
func (m Model) renderCharCount() string {
	count := len([]rune(m.input.Value()))
	max := m.input.CharLimit

	// Prevent division by zero
	if max <= 0 {
		max = 1
	}

	// Determine color based on usage
	var style lipgloss.Style
	percent := float64(count) / float64(max) * 100

	if percent >= 90 {
		style = lipgloss.NewStyle().Foreground(styles.Rose)
	} else if percent >= 75 {
		style = lipgloss.NewStyle().Foreground(styles.Amber)
	} else {
		style = lipgloss.NewStyle().Foreground(styles.TextMuted)
	}

	countStr := formatInt(count) + " / " + formatInt(max)

	// Use stored width, ensure minimum
	width := m.width
	if width <= 0 {
		width = 80
	}
	charCountWidth := width - 4
	if charCountWidth < 10 {
		charCountWidth = 10
	}

	return lipgloss.NewStyle().
		Width(charCountWidth).
		Align(lipgloss.Right).
		Padding(0, 2).
		Render(style.Render(countStr))
}

// =============================================================================
// STATUS BAR
// =============================================================================

// renderStatusBar renders the bottom status bar.
// Format: qwen2.5-coder:14b | Cloud | 1,234 tok | $0.45 | Saved: $12.30 | Ctrl+R=mode
// Responsive: adapts content based on terminal width with smart truncation.
// Guarantees content NEVER exceeds terminal width - no overflow, no wrapping.
func (m Model) renderStatusBar() string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	sep := lipgloss.NewStyle().Foreground(styles.Overlay).Render(" | ")

	// Maximum available width for content (excluding padding from Padding(0, 1))
	maxContentWidth := width - 4
	if maxContentWidth < 20 {
		maxContentWidth = 20
	}

	// IL5 SC-7: Offline mode badge
	var offlineBadge string
	if m.offlineMode {
		offlineBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("#FF0000")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1).
			Render("OFFLINE")
	}

	// Routing mode indicator with icon
	routingMode := m.routingMode
	if routingMode == "" {
		routingMode = "hybrid"
	}
	// IL5 SC-7: In offline mode, show "local-only" and disable cloud indicators
	if m.offlineMode {
		routingMode = "local-only"
	}
	modeStyle := lipgloss.NewStyle().Bold(true)
	var modeIcon string
	switch routingMode {
	case "local":
		modeStyle = modeStyle.Foreground(styles.Emerald)
		modeIcon = "@"
	case "local-only":
		modeStyle = modeStyle.Foreground(styles.Rose)
		modeIcon = "@"
	case "cloud":
		modeStyle = modeStyle.Foreground(styles.Amber)
		modeIcon = "*"
	default: // "hybrid"
		modeStyle = modeStyle.Foreground(styles.Purple)
		modeIcon = "~"
	}

	// Prepare all possible left section components (will progressively remove if needed)
	// Components ordered by removal priority (last = removed first):
	// 1. Saved amount (lowest priority - remove first)
	// 2. Cost
	// 3. Token count
	// 4. Full mode text (fall back to icon only)
	// 5. Model name truncation (increasingly aggressive)

	var savedStr string
	var costStr string
	var tokenStr string

	if m.sessionStats != nil {
		stats := m.sessionStats.GetStats()
		if stats.TotalQueries > 0 {
			// Token count
			tokenCount := stats.TotalInputTokens + stats.TotalOutputTokens
			tokenStr = lipgloss.NewStyle().
				Foreground(styles.TextMuted).
				Render(fmt.Sprintf("%s tok", formatNumberWithCommas(tokenCount)))

			// Cost
			if stats.TotalCostCents > 0.001 {
				costStr = lipgloss.NewStyle().
					Foreground(styles.Amber).
					Render(formatCost(stats.TotalCostCents))
			}

			// Saved
			if stats.TotalSavedCents > 0.001 {
				savedStr = lipgloss.NewStyle().
					Foreground(styles.Emerald).
					Render("Saved: " + formatCost(stats.TotalSavedCents))
			}
		}
	}

	// Right section components (vim mode + context bar + context cost + shortcuts)
	var vimModeIndicator string
	if m.vimHandler != nil && m.vimHandler.Enabled() {
		vimModeStr := m.vimHandler.ModeString()
		// Style vim mode based on current mode
		var vimStyle lipgloss.Style
		switch m.vimHandler.Mode() {
		case VimModeNormal:
			vimStyle = lipgloss.NewStyle().Foreground(styles.Cyan).Bold(true)
		case VimModeInsert:
			vimStyle = lipgloss.NewStyle().Foreground(styles.Emerald).Bold(true)
		case VimModeVisual:
			vimStyle = lipgloss.NewStyle().Foreground(styles.Purple).Bold(true)
		case VimModeCommand:
			vimStyle = lipgloss.NewStyle().Foreground(styles.Amber).Bold(true)
		default:
			vimStyle = lipgloss.NewStyle().Foreground(styles.TextMuted)
		}
		vimModeIndicator = vimStyle.Render(vimModeStr)
	}

	contextBarCompact := m.renderContextBarCompact()
	contextBarFull := m.renderContextBarWithTier()
	contextCostInfo := m.renderContextCostInfo()
	// Help hint styled to be subtle but noticeable - always show in some form
	shortcutsShort := lipgloss.NewStyle().Foreground(styles.TextMuted).Render("? | ^C")
	shortcutsFull := lipgloss.NewStyle().Foreground(styles.TextMuted).Render("?=help | ^R=mode | ^C=stop")

	// Helper function to build and measure status bar with given components
	buildStatusBar := func(modelName string, showFullMode, showTokens, showCost, showSaved, useFullContext, showContextCost, useFullShortcuts bool) (left, right string, totalWidth int) {
		// IL5 SC-7: Start with offline badge if active
		var leftParts []string
		if offlineBadge != "" {
			leftParts = append(leftParts, offlineBadge)
		}

		modelStr := lipgloss.NewStyle().
			Foreground(styles.TextSecondary).
			Render(modelName)
		leftParts = append(leftParts, modelStr)

		var modeStr string
		if showFullMode {
			modeStr = modeStyle.Render(modeIcon + " " + strings.ToUpper(routingMode))
		} else {
			modeStr = modeStyle.Render(modeIcon)
		}
		leftParts = append(leftParts, modeStr)

		// IL5 SC-7: Show "Cloud: disabled" in offline mode
		if m.offlineMode {
			cloudDisabled := lipgloss.NewStyle().
				Foreground(styles.TextMuted).
				Italic(true).
				Render("Cloud: disabled")
			leftParts = append(leftParts, cloudDisabled)
		}

		left = strings.Join(leftParts, sep)

		if showTokens && tokenStr != "" {
			left += sep + tokenStr
		}
		if showCost && costStr != "" {
			left += sep + costStr
		}
		if showSaved && savedStr != "" {
			left += sep + savedStr
		}

		var contextBar, shortcuts string
		var rightParts []string

		// Add vim mode indicator if enabled
		if vimModeIndicator != "" {
			rightParts = append(rightParts, vimModeIndicator)
		}

		if useFullContext {
			contextBar = contextBarFull
		} else {
			contextBar = contextBarCompact
		}
		rightParts = append(rightParts, contextBar)

		// Add context cost info if available
		if showContextCost && contextCostInfo != "" {
			rightParts = append(rightParts, contextCostInfo)
		}

		if useFullShortcuts {
			shortcuts = shortcutsFull
		} else {
			shortcuts = shortcutsShort
		}
		rightParts = append(rightParts, shortcuts)

		right = strings.Join(rightParts, "  ")

		leftWidth := lipgloss.Width(left)
		rightWidth := lipgloss.Width(right)
		totalWidth = leftWidth + rightWidth + 1 // +1 for minimum spacing

		return left, right, totalWidth
	}

	// Try configurations from most complete to most minimal
	// Each step removes one element or truncates more aggressively
	modelName := m.modelName

	type statusConfig struct {
		modelMaxLen      int
		showFullMode     bool
		showTokens       bool
		showCost         bool
		showSaved        bool
		useFullContext   bool
		showContextCost  bool
		useFullShortcuts bool
	}

	// Only show context cost if we have @mentions in input
	hasContextMentions := m.contextTokenEstimate > 0

	configurations := []statusConfig{
		// Full configuration
		{40, true, true, true, true, true, hasContextMentions, true},
		// Remove saved
		{40, true, true, true, false, true, hasContextMentions, true},
		// Remove cost
		{40, true, true, false, false, true, hasContextMentions, true},
		// Remove tokens
		{40, true, false, false, false, true, hasContextMentions, true},
		// Use compact shortcuts
		{40, true, false, false, false, true, hasContextMentions, false},
		// Remove context cost
		{40, true, false, false, false, true, false, false},
		// Use compact context
		{40, true, false, false, false, false, false, false},
		// Use icon-only mode
		{40, false, false, false, false, false, false, false},
		// Truncate model name to 25
		{25, false, false, false, false, false, false, false},
		// Truncate model name to 18
		{18, false, false, false, false, false, false, false},
		// Truncate model name to 12
		{12, false, false, false, false, false, false, false},
		// Truncate model name to 8
		{8, false, false, false, false, false, false, false},
		// Minimal - truncate model to 5
		{5, false, false, false, false, false, false, false},
	}

	var finalLeft, finalRight string
	for _, cfg := range configurations {
		truncatedModel := modelName
		// Use rune-based truncation to handle Unicode correctly
		modelRunes := []rune(truncatedModel)
		if len(modelRunes) > cfg.modelMaxLen {
			truncatedModel = string(modelRunes[:cfg.modelMaxLen]) + ".."
		}

		left, right, totalWidth := buildStatusBar(
			truncatedModel,
			cfg.showFullMode,
			cfg.showTokens,
			cfg.showCost,
			cfg.showSaved,
			cfg.useFullContext,
			cfg.showContextCost,
			cfg.useFullShortcuts,
		)

		if totalWidth <= maxContentWidth {
			finalLeft = left
			finalRight = right
			break
		}
	}

	// Fallback: if still too wide after all configurations, use absolute minimum
	if finalLeft == "" {
		modeStr := modeStyle.Render(modeIcon)
		finalLeft = modeStr
		finalRight = shortcutsShort
	}

	// Calculate padding - guaranteed to fit now since we checked width above
	leftWidth := lipgloss.Width(finalLeft)
	rightWidth := lipgloss.Width(finalRight)
	padding := maxContentWidth - leftWidth - rightWidth
	if padding < 0 {
		padding = 0
	}

	// Build status bar
	status := lipgloss.NewStyle().
		Background(styles.SurfaceDim).
		Width(width).
		Padding(0, 1).
		Render(finalLeft + strings.Repeat(" ", padding) + finalRight)

	return status
}

// renderContextBar renders the context usage bar.
func (m Model) renderContextBar() string {
	percent := m.GetContextPercent()
	barWidth := 10

	filled := int(percent / 100 * float64(barWidth))
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	// Determine color
	var color lipgloss.AdaptiveColor
	if percent >= 90 {
		color = styles.Rose
	} else if percent >= 75 {
		color = styles.Amber
	} else {
		color = styles.Cyan
	}

	bar := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("#", filled)) +
		lipgloss.NewStyle().Foreground(styles.Overlay).Render(strings.Repeat("-", empty))

	label := lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Ctx: ")

	return label + bar
}

// renderContextBarWithTier renders context bar with color based on current tier.
func (m Model) renderContextBarWithTier() string {
	percent := m.GetContextPercent()
	barWidth := 10

	filled := int(percent / 100 * float64(barWidth))
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	// Determine color based on tier if we have routing info
	var color lipgloss.AdaptiveColor
	if m.lastRouting != nil {
		switch m.lastRouting.Tier {
		case router.TierCache:
			color = styles.Cyan
		case router.TierLocal:
			color = styles.Emerald
		case router.TierCloud, router.TierHaiku:
			color = styles.Amber
		case router.TierSonnet:
			color = styles.Purple
		case router.TierOpus, router.TierGpt4o:
			color = styles.Rose
		default:
			color = styles.Cyan
		}
	} else {
		// Default color based on context percentage
		if percent >= 90 {
			color = styles.Rose
		} else if percent >= 75 {
			color = styles.Amber
		} else {
			color = styles.Cyan
		}
	}

	bar := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("#", filled)) +
		lipgloss.NewStyle().Foreground(styles.Overlay).Render(strings.Repeat("-", empty))

	label := lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Ctx: ")

	return label + "[" + bar + "]"
}

// renderContextBarCompact renders a compact context bar for very narrow terminals.
func (m Model) renderContextBarCompact() string {
	percent := m.GetContextPercent()
	barWidth := 5 // Smaller bar for narrow terminals

	filled := int(percent / 100 * float64(barWidth))
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	// Determine color based on context percentage
	var color lipgloss.AdaptiveColor
	if percent >= 90 {
		color = styles.Rose
	} else if percent >= 75 {
		color = styles.Amber
	} else {
		color = styles.Cyan
	}

	bar := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("#", filled)) +
		lipgloss.NewStyle().Foreground(styles.Overlay).Render(strings.Repeat("-", empty))

	return "[" + bar + "]"
}

// =============================================================================
// HELP OVERLAY
// =============================================================================

// renderHelpOverlay renders context-sensitive keyboard shortcuts help overlay.
// Following lazygit's pattern, only shows keybindings that work in the current context.
// This is displayed when the user presses '?' to toggle help.
func (m Model) renderHelpOverlay() string {
	width := m.width
	height := m.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	// Determine the context that was active BEFORE help was shown
	// When help opens, we show keys for the previous context (what user was doing)
	var activeContext HelpContext
	if m.searchMode {
		activeContext = ContextSearch
	} else if m.state == StateError {
		activeContext = ContextError
	} else if m.state == StateStreaming {
		activeContext = ContextStreaming
	} else if m.inputMode {
		activeContext = ContextInput
	} else {
		activeContext = ContextNormal
	}

	// Get help items filtered by context and grouped by category
	groupedItems := GetHelpItemsByCategory(activeContext)
	categoryOrder := GetCategoryOrder()

	// Build help content
	var sb strings.Builder

	// Header with context indicator - styled to stand out
	contextName := GetContextDisplayName(activeContext)
	sb.WriteString(fmt.Sprintf("Keys available now (%s)\n", contextName))
	sb.WriteString(strings.Repeat("\u2500", 35) + "\n\n") // Unicode horizontal line

	// Render items grouped by category in preferred order
	hasContent := false
	for _, category := range categoryOrder {
		items, exists := groupedItems[category]
		if !exists || len(items) == 0 {
			continue
		}

		hasContent = true
		// Category header
		categoryStyle := lipgloss.NewStyle().
			Foreground(styles.Cyan).
			Bold(true)
		sb.WriteString(categoryStyle.Render(string(category)) + "\n")

		// Items in this category
		for _, item := range items {
			keyStyle := lipgloss.NewStyle().Foreground(styles.Amber)
			descStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				keyStyle.Render(fmt.Sprintf("%-14s", item.Key)),
				descStyle.Render(item.Desc)))
		}
		sb.WriteString("\n")
	}

	// Add vim mode section if vim is enabled and relevant to context
	if m.vimHandler != nil && m.vimHandler.Enabled() {
		vimItems := GetVimHelpItems()
		var relevantVimItems []HelpItem
		for _, item := range vimItems {
			for _, ctx := range item.Contexts {
				if ctx == activeContext {
					relevantVimItems = append(relevantVimItems, item)
					break
				}
			}
		}

		if len(relevantVimItems) > 0 {
			categoryStyle := lipgloss.NewStyle().
				Foreground(styles.Cyan).
				Bold(true)
			sb.WriteString(categoryStyle.Render("Vim") + "\n")

			for _, item := range relevantVimItems {
				keyStyle := lipgloss.NewStyle().Foreground(styles.Amber)
				descStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
				sb.WriteString(fmt.Sprintf("  %s  %s\n",
					keyStyle.Render(fmt.Sprintf("%-14s", item.Key)),
					descStyle.Render(item.Desc)))
			}
			sb.WriteString("\n")
		}
	}

	// If no items for this context, show a helpful message
	if !hasContent {
		sb.WriteString("  No specific keybindings for this mode.\n\n")
	}

	// Current state indicator
	sb.WriteString(strings.Repeat("\u2500", 35) + "\n")
	stateStyle := lipgloss.NewStyle().Foreground(styles.TextMuted).Italic(true)

	// Show current mode info
	var modeInfo string
	switch activeContext {
	case ContextInput:
		modeInfo = "Input mode - type your message"
	case ContextNormal:
		modeInfo = "Normal mode - navigate with j/k"
	case ContextStreaming:
		modeInfo = "Streaming - Esc or C-c to cancel"
	case ContextSearch:
		modeInfo = "Search mode - n/N to navigate"
	case ContextError:
		modeInfo = "Error - Esc or Enter to dismiss"
	default:
		modeInfo = "Press ? to toggle help"
	}
	sb.WriteString(stateStyle.Render(modeInfo) + "\n")

	// Multi-line mode indicator if in input mode
	if activeContext == ContextInput {
		if m.multiLineMode {
			sb.WriteString(stateStyle.Render("Multi-line: ON (C-Enter to send)") + "\n")
		}
	}

	// Close hint
	sb.WriteString("\n")
	closeStyle := lipgloss.NewStyle().Foreground(styles.Overlay)
	sb.WriteString(closeStyle.Render("Press ? or Esc to close"))

	content := sb.String()

	// Calculate overlay dimensions - slightly wider for better formatting
	contentWidth := 55
	if contentWidth > width-4 {
		contentWidth = width - 4
	}

	contentLines := strings.Count(content, "\n") + 1
	contentHeight := contentLines + 2 // +2 for padding
	if contentHeight > height-4 {
		contentHeight = height - 4
	}

	// Create help box style with subtle background
	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Purple).
		Foreground(styles.TextPrimary).
		Background(styles.Surface).
		Padding(1, 2).
		Width(contentWidth).
		MaxHeight(contentHeight)

	helpBox := helpStyle.Render(content)

	// Center the help box
	helpWidth := lipgloss.Width(helpBox)
	helpHeight := lipgloss.Height(helpBox)

	marginLeft := (width - helpWidth) / 2
	if marginLeft < 0 {
		marginLeft = 0
	}
	marginTop := (height - helpHeight) / 2
	if marginTop < 0 {
		marginTop = 0
	}

	// Create centered overlay
	centered := lipgloss.NewStyle().
		MarginLeft(marginLeft).
		MarginTop(marginTop).
		Render(helpBox)

	return centered
}

// =============================================================================
// CONTEXT COST DISPLAY
// =============================================================================

// renderContextCostInfo renders the context cost information when @mentions are detected.
// Uses the new ContextBar component to show active @mentions with token counts.
// Format examples:
//   - "@file:main.go +2.5k | @git +500"
//   - "Context: @file:main.go +2.5k | Total: ~3k tokens"
func (m Model) renderContextCostInfo() string {
	// Only show if we have active context with mentions
	if m.activeContext == nil || !m.activeContext.HasItems() {
		return ""
	}

	// Create context bar component
	contextBar := components.NewContextBar()
	contextBar.SetContext(m.activeContext)
	contextBar.SetWidth(m.width)

	// Render inline version for status bar (super compact)
	result := contextBar.RenderInline()

	// Add cost estimate for cloud routing if we have cost estimate
	cost := m.contextCostEstimate
	if cost > 0.001 { // Only show if cost is meaningful
		costStr := formatCost(cost)
		costStyle := lipgloss.NewStyle().Foreground(styles.Amber)
		result += " " + costStyle.Render("~"+costStr)
	}

	return result
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================
// All formatting and text utilities have been moved to utils.go for better organization.
// This section is intentionally left empty as a marker for potential future view-specific helpers.

// =============================================================================
// PROGRESS INDICATOR RENDERING
// =============================================================================

// renderProgressIndicator renders the progress indicator for agentic loops
func (m Model) renderProgressIndicator() string {
	if m.progressIndicator == nil {
		return ""
	}

	// Set width for rendering without mutating the model's indicator
	// Create a copy to avoid mutating the shared state
	indicator := *m.progressIndicator
	indicator.Width = m.width - 4

	// Render the progress indicator
	return indicator.Render()
}
