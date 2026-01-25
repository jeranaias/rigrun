// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides the visual UI components for rigrun TUI.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/jeranaias/rigrun-tui/internal/model"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// CHAT VIEWPORT COMPONENT - Scrollable chat area with indicators
// =============================================================================

// ChatViewport represents a scrollable chat viewport with proper scroll tracking
type ChatViewport struct {
	viewport    viewport.Model
	messages    []*model.Message
	width       int
	height      int
	ready       bool
	autoScroll  bool  // Auto-scroll to bottom on new content
	theme       *styles.Theme
	messageList *MessageList

	// Scroll position tracking for proper scroll behavior
	scrollY    int // Current scroll position (line offset)
	maxScrollY int // Maximum scroll position (total lines - visible height)
}

// NewChatViewport creates a new ChatViewport
func NewChatViewport(theme *styles.Theme) *ChatViewport {
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle()

	return &ChatViewport{
		viewport:    vp,
		messages:    []*model.Message{},
		width:       80,
		height:      20,
		ready:       false,
		autoScroll:  true,
		theme:       theme,
		messageList: NewMessageList(theme),
	}
}

// SetSize updates the viewport dimensions
func (cv *ChatViewport) SetSize(width, height int) {
	cv.width = width
	cv.height = height
	cv.viewport.Width = width - 2  // Account for scroll indicator
	cv.viewport.Height = height
	cv.messageList.SetWidth(width - 4) // Account for padding
	cv.ready = true

	// Re-render content with new size
	cv.updateContent()
}

// SetMessages updates the messages to display
func (cv *ChatViewport) SetMessages(messages []*model.Message) {
	cv.messages = messages
	cv.messageList.SetMessages(messages)
	cv.updateContent()

	// Auto-scroll to bottom if enabled
	if cv.autoScroll {
		cv.ScrollToBottom()
	}
}

// AppendMessage adds a message to the list
func (cv *ChatViewport) AppendMessage(msg *model.Message) {
	cv.messages = append(cv.messages, msg)
	cv.messageList.SetMessages(cv.messages)
	cv.updateContent()

	if cv.autoScroll {
		cv.ScrollToBottom()
	}
}

// UpdateLastMessage updates the content of the last message (for streaming)
func (cv *ChatViewport) UpdateLastMessage() {
	cv.updateContent()

	if cv.autoScroll {
		cv.ScrollToBottom()
	}
}

// updateContent re-renders the message content and updates scroll tracking
func (cv *ChatViewport) updateContent() {
	content := cv.messageList.View()

	// Wrap content for proper width calculation
	wrappedContent := wrapContentForViewport(content, cv.width-2)
	cv.viewport.SetContent(wrappedContent)

	// Update scroll position tracking
	lines := strings.Count(wrappedContent, "\n") + 1
	cv.maxScrollY = maxInt0(0, lines-cv.height)

	// Sync scrollY with viewport's actual position
	cv.scrollY = cv.viewport.YOffset

	// Ensure scrollY is within bounds
	if cv.scrollY > cv.maxScrollY {
		cv.scrollY = cv.maxScrollY
	}
	if cv.scrollY < 0 {
		cv.scrollY = 0
	}
}

// ScrollToBottom scrolls to the bottom of the viewport
func (cv *ChatViewport) ScrollToBottom() {
	cv.viewport.GotoBottom()
	cv.scrollY = cv.maxScrollY
	cv.autoScroll = true
}

// ScrollToTop scrolls to the top of the viewport
func (cv *ChatViewport) ScrollToTop() {
	cv.viewport.GotoTop()
	cv.scrollY = 0
	cv.autoScroll = false
}

// ScrollUp scrolls up by the specified number of lines
func (cv *ChatViewport) ScrollUp(lines int) {
	cv.autoScroll = false // User took control - disable auto-scroll
	cv.scrollY = maxInt0(0, cv.scrollY-lines)
	cv.viewport.SetYOffset(cv.scrollY)
}

// ScrollDown scrolls down by the specified number of lines
func (cv *ChatViewport) ScrollDown(lines int) {
	cv.scrollY = minInt(cv.maxScrollY, cv.scrollY+lines)
	cv.viewport.SetYOffset(cv.scrollY)

	// Re-enable auto-scroll if at bottom
	if cv.scrollY >= cv.maxScrollY {
		cv.autoScroll = true
	}
}

// PageUp scrolls up by one page
func (cv *ChatViewport) PageUp() {
	cv.autoScroll = false // User took control
	cv.scrollY = maxInt0(0, cv.scrollY-cv.height)
	cv.viewport.SetYOffset(cv.scrollY)
}

// PageDown scrolls down by one page
func (cv *ChatViewport) PageDown() {
	cv.scrollY = minInt(cv.maxScrollY, cv.scrollY+cv.height)
	cv.viewport.SetYOffset(cv.scrollY)

	// Re-enable auto-scroll if at bottom
	if cv.scrollY >= cv.maxScrollY {
		cv.autoScroll = true
	}
}

// AtTop returns true if the viewport is at the top
func (cv *ChatViewport) AtTop() bool {
	return cv.viewport.AtTop()
}

// AtBottom returns true if the viewport is at the bottom
func (cv *ChatViewport) AtBottom() bool {
	return cv.viewport.AtBottom()
}

// ScrollPercent returns the scroll position as a percentage
func (cv *ChatViewport) ScrollPercent() float64 {
	return cv.viewport.ScrollPercent()
}

// EnableAutoScroll enables automatic scrolling to bottom
func (cv *ChatViewport) EnableAutoScroll() {
	cv.autoScroll = true
}

// DisableAutoScroll disables automatic scrolling
func (cv *ChatViewport) DisableAutoScroll() {
	cv.autoScroll = false
}

// Update handles viewport updates with proper scroll tracking
func (cv *ChatViewport) Update(msg tea.Msg) (*ChatViewport, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			cv.ScrollUp(1)
			return cv, nil
		case "down", "j":
			cv.ScrollDown(1)
			// Re-enable auto-scroll if at bottom
			if cv.scrollY >= cv.maxScrollY {
				cv.autoScroll = true
			}
			return cv, nil
		case "pgup":
			cv.ScrollUp(cv.height)
			cv.autoScroll = false
			return cv, nil
		case "pgdn", "pgdown":
			cv.ScrollDown(cv.height)
			// Re-enable auto-scroll if at bottom
			if cv.scrollY >= cv.maxScrollY {
				cv.autoScroll = true
			}
			return cv, nil
		case "home", "g":
			cv.ScrollToTop()
			cv.autoScroll = false
			return cv, nil
		case "end", "G":
			cv.ScrollToBottom()
			cv.autoScroll = true
			return cv, nil
		}

	case tea.MouseMsg:
		// Handle mouse wheel scrolling with smooth behavior
		switch msg.Type {
		case tea.MouseWheelUp:
			cv.ScrollUp(3)
			return cv, nil
		case tea.MouseWheelDown:
			cv.ScrollDown(3)
			return cv, nil
		}
	}

	// Let the underlying viewport handle any other messages
	cv.viewport, cmd = cv.viewport.Update(msg)

	// Sync our scroll tracking with viewport's actual position
	cv.scrollY = cv.viewport.YOffset

	return cv, cmd
}

// View renders the viewport with scroll indicators
func (cv *ChatViewport) View() string {
	if !cv.ready {
		return ""
	}

	// Main viewport content
	viewportContent := cv.viewport.View()

	// Scroll indicators
	topIndicator := cv.renderTopIndicator()
	bottomIndicator := cv.renderBottomIndicator()

	// Build the complete view
	var result strings.Builder

	// Top indicator
	if topIndicator != "" {
		result.WriteString(topIndicator)
		result.WriteString("\n")
	}

	// Viewport content
	result.WriteString(viewportContent)

	// Bottom indicator
	if bottomIndicator != "" {
		result.WriteString("\n")
		result.WriteString(bottomIndicator)
	}

	return result.String()
}

// ViewWithBorder renders the viewport with a decorative border
func (cv *ChatViewport) ViewWithBorder() string {
	content := cv.View()

	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Overlay).
		Width(cv.width)

	return borderStyle.Render(content)
}

// ==========================================================================
// SCROLL INDICATORS
// ==========================================================================

// renderTopIndicator renders the "more above" indicator
func (cv *ChatViewport) renderTopIndicator() string {
	if cv.AtTop() {
		return ""
	}

	indicatorStyle := lipgloss.NewStyle().
		Foreground(styles.Purple).
		Width(cv.width).
		Align(lipgloss.Center)

	// Fancy indicator with arrows
	arrowStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan)

	textStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)

	indicator := arrowStyle.Render("^") + " " +
		textStyle.Render("scroll up for more") + " " +
		arrowStyle.Render("^")

	return indicatorStyle.Render(indicator)
}

// renderBottomIndicator renders the "more below" indicator with scroll position
func (cv *ChatViewport) renderBottomIndicator() string {
	if cv.AtBottom() {
		return ""
	}

	indicatorStyle := lipgloss.NewStyle().
		Foreground(styles.Purple).
		Width(cv.width).
		Align(lipgloss.Center)

	// Fancy indicator with arrows
	arrowStyle := lipgloss.NewStyle().
		Foreground(styles.Cyan)

	textStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)

	// Add scroll position indicator
	posStyle := lipgloss.NewStyle().
		Foreground(styles.Purple).
		Bold(true)

	scrollPos := ""
	if cv.maxScrollY > 0 {
		scrollPos = posStyle.Render(fmt.Sprintf(" [%d/%d] ", cv.scrollY+1, cv.maxScrollY+1))
	}

	indicator := arrowStyle.Render("v") + scrollPos +
		textStyle.Render("scroll down for more") + " " +
		arrowStyle.Render("v")

	return indicatorStyle.Render(indicator)
}

// =============================================================================
// SCROLL BAR COMPONENT - Beautiful vertical scroll bar
// =============================================================================

// ScrollBar represents a vertical scroll bar
type ScrollBar struct {
	Height       int
	ScrollPos    float64 // 0.0 to 1.0
	ContentRatio float64 // visible / total
	theme        *styles.Theme
}

// NewScrollBar creates a new ScrollBar
func NewScrollBar(theme *styles.Theme) *ScrollBar {
	return &ScrollBar{
		Height:       20,
		ScrollPos:    0.0,
		ContentRatio: 1.0,
		theme:        theme,
	}
}

// SetHeight sets the scroll bar height
func (sb *ScrollBar) SetHeight(height int) {
	sb.Height = height
}

// SetPosition sets the scroll position (0.0 to 1.0)
func (sb *ScrollBar) SetPosition(pos float64) {
	if pos < 0 {
		pos = 0
	}
	if pos > 1 {
		pos = 1
	}
	sb.ScrollPos = pos
}

// SetContentRatio sets the visible/total content ratio
func (sb *ScrollBar) SetContentRatio(ratio float64) {
	if ratio < 0.1 {
		ratio = 0.1
	}
	if ratio > 1 {
		ratio = 1
	}
	sb.ContentRatio = ratio
}

// View renders the scroll bar
func (sb *ScrollBar) View() string {
	if sb.Height <= 0 || sb.ContentRatio >= 1.0 {
		// No scrolling needed - show faded track
		trackStyle := lipgloss.NewStyle().
			Foreground(styles.Overlay)
		return trackStyle.Render(strings.Repeat("|", sb.Height))
	}

	// Calculate thumb size and position
	thumbSize := int(float64(sb.Height) * sb.ContentRatio)
	if thumbSize < 1 {
		thumbSize = 1
	}
	if thumbSize > sb.Height {
		thumbSize = sb.Height
	}

	// Calculate thumb position
	scrollableTrack := sb.Height - thumbSize
	thumbPos := int(float64(scrollableTrack) * sb.ScrollPos)
	if thumbPos < 0 {
		thumbPos = 0
	}
	if thumbPos > scrollableTrack {
		thumbPos = scrollableTrack
	}

	// Build the scroll bar
	var result strings.Builder

	trackStyle := lipgloss.NewStyle().
		Foreground(styles.Overlay)

	thumbStyle := lipgloss.NewStyle().
		Foreground(styles.Purple)

	for i := 0; i < sb.Height; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			result.WriteString(thumbStyle.Render("#"))
		} else {
			result.WriteString(trackStyle.Render("|"))
		}
		if i < sb.Height-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// =============================================================================
// VIEWPORT WITH SCROLLBAR - Combined viewport and scroll bar
// =============================================================================

// ViewportWithScrollbar combines the chat viewport with a scroll bar
type ViewportWithScrollbar struct {
	viewport  *ChatViewport
	scrollbar *ScrollBar
	width     int
	height    int
	theme     *styles.Theme
}

// NewViewportWithScrollbar creates a combined viewport and scrollbar
func NewViewportWithScrollbar(theme *styles.Theme) *ViewportWithScrollbar {
	return &ViewportWithScrollbar{
		viewport:  NewChatViewport(theme),
		scrollbar: NewScrollBar(theme),
		width:     80,
		height:    20,
		theme:     theme,
	}
}

// SetSize updates dimensions
func (vws *ViewportWithScrollbar) SetSize(width, height int) {
	vws.width = width
	vws.height = height
	vws.viewport.SetSize(width-3, height) // Reserve space for scrollbar
	vws.scrollbar.SetHeight(height)
}

// SetMessages updates messages
func (vws *ViewportWithScrollbar) SetMessages(messages []*model.Message) {
	vws.viewport.SetMessages(messages)
	vws.updateScrollbar()
}

// AppendMessage appends a message
func (vws *ViewportWithScrollbar) AppendMessage(msg *model.Message) {
	vws.viewport.AppendMessage(msg)
	vws.updateScrollbar()
}

// UpdateLastMessage updates the last message
func (vws *ViewportWithScrollbar) UpdateLastMessage() {
	vws.viewport.UpdateLastMessage()
	vws.updateScrollbar()
}

// updateScrollbar updates the scrollbar position
func (vws *ViewportWithScrollbar) updateScrollbar() {
	vws.scrollbar.SetPosition(vws.viewport.ScrollPercent())

	// Use actual content height from the viewport for accurate scrollbar sizing
	totalContent := vws.viewport.viewport.TotalLineCount()
	if totalContent > 0 {
		ratio := float64(vws.height) / float64(totalContent)
		vws.scrollbar.SetContentRatio(ratio)
	}
}

// Update handles updates
func (vws *ViewportWithScrollbar) Update(msg tea.Msg) (*ViewportWithScrollbar, tea.Cmd) {
	var cmd tea.Cmd
	vws.viewport, cmd = vws.viewport.Update(msg)
	vws.updateScrollbar()
	return vws, cmd
}

// View renders the viewport with scrollbar
func (vws *ViewportWithScrollbar) View() string {
	viewportView := vws.viewport.View()
	scrollbarView := vws.scrollbar.View()

	// Join horizontally
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		viewportView,
		" ",
		scrollbarView,
	)
}

// Accessor methods for the embedded viewport
func (vws *ViewportWithScrollbar) ScrollToBottom() {
	vws.viewport.ScrollToBottom()
	vws.updateScrollbar()
}

func (vws *ViewportWithScrollbar) ScrollToTop() {
	vws.viewport.ScrollToTop()
	vws.updateScrollbar()
}

func (vws *ViewportWithScrollbar) EnableAutoScroll() {
	vws.viewport.EnableAutoScroll()
}

func (vws *ViewportWithScrollbar) DisableAutoScroll() {
	vws.viewport.DisableAutoScroll()
}

// =============================================================================
// CONTENT WRAPPING WITH RUNEWIDTH SUPPORT
// =============================================================================

// wrapContentForViewport wraps content to fit within the specified width,
// using go-runewidth for proper Unicode and wide character handling.
// This ensures Asian characters, emojis, and other wide characters are handled correctly.
func wrapContentForViewport(content string, width int) string {
	if width <= 0 {
		return content
	}

	var wrapped strings.Builder
	for _, line := range strings.Split(content, "\n") {
		// Check if line already fits
		lineWidth := runewidth.StringWidth(line)
		if lineWidth <= width {
			if wrapped.Len() > 0 {
				wrapped.WriteByte('\n')
			}
			wrapped.WriteString(line)
			continue
		}

		// Wrap long lines using word boundaries when possible
		wrappedLine := wordWrapWithRunewidth(line, width)
		if wrapped.Len() > 0 {
			wrapped.WriteByte('\n')
		}
		wrapped.WriteString(wrappedLine)
	}

	return wrapped.String()
}

// wordWrapWithRunewidth wraps a single line to the specified width,
// using runewidth for proper character width calculation.
// It tries to break at word boundaries when possible.
func wordWrapWithRunewidth(line string, width int) string {
	if width <= 0 {
		return line
	}

	runes := []rune(line)
	if len(runes) == 0 {
		return ""
	}

	var result strings.Builder
	var currentLine strings.Builder
	currentWidth := 0
	lastSpaceIdx := -1

	for i, r := range runes {
		charWidth := runewidth.RuneWidth(r)

		// Track last space position for word-boundary breaks
		if r == ' ' {
			lastSpaceIdx = i
		}

		// Check if adding this character would exceed width
		if currentWidth+charWidth > width {
			// Try to break at word boundary
			if lastSpaceIdx > 0 && currentLine.Len() > 0 {
				// Write up to the last space
				lineStr := currentLine.String()
				lineRunes := []rune(lineStr)

				// Calculate rune index for the space
				runeIdx := 0
				byteIdx := 0
				for byteIdx < len(lineStr) && runeIdx < lastSpaceIdx-(i-len(lineRunes)) {
					// Get byte size of current rune for proper indexing
					size := len(string(lineRunes[runeIdx]))
					byteIdx += size
					runeIdx++
				}

				// Simplified approach: write current line and start new line
				if result.Len() > 0 {
					result.WriteByte('\n')
				}
				result.WriteString(strings.TrimRight(lineStr, " "))

				// Start new line with remaining content
				currentLine.Reset()
				currentLine.WriteRune(r)
				currentWidth = charWidth
				lastSpaceIdx = -1
			} else {
				// No good break point, force break at current position
				if currentLine.Len() > 0 {
					if result.Len() > 0 {
						result.WriteByte('\n')
					}
					result.WriteString(currentLine.String())
					currentLine.Reset()
				}
				currentLine.WriteRune(r)
				currentWidth = charWidth
				lastSpaceIdx = -1
			}
		} else {
			currentLine.WriteRune(r)
			currentWidth += charWidth
		}
	}

	// Write remaining content
	if currentLine.Len() > 0 {
		if result.Len() > 0 {
			result.WriteByte('\n')
		}
		result.WriteString(currentLine.String())
	}

	return result.String()
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// maxInt0 returns the maximum of two integers (renamed to avoid conflicts)
// Used for scroll position calculations
func maxInt0(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GetScrollPosition returns the current scroll position as a formatted string
// for display in the UI (e.g., "[15/100]")
func (cv *ChatViewport) GetScrollPosition() string {
	if cv.maxScrollY <= 0 {
		return ""
	}
	return fmt.Sprintf("[%d/%d]", cv.scrollY+1, cv.maxScrollY+1)
}

// GetScrollY returns the current Y scroll offset
func (cv *ChatViewport) GetScrollY() int {
	return cv.scrollY
}

// GetMaxScrollY returns the maximum Y scroll offset
func (cv *ChatViewport) GetMaxScrollY() int {
	return cv.maxScrollY
}
