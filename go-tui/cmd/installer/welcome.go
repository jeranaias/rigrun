// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// FIRST RUN WELCOME EXPERIENCE
// =============================================================================

// WelcomeScreen shows the first-run experience after installation
type WelcomeScreen struct {
	width       int
	height      int
	step        int
	typing      string
	typeTarget  string
	typeIndex   int
	showCursor  bool
	tips        []Tip
	currentTip  int
}

// Tip is an interactive tutorial tip
type Tip struct {
	Title       string
	Description string
	Example     string
	Icon        string
}

// NewWelcomeScreen creates the first-run welcome
func NewWelcomeScreen() *WelcomeScreen {
	return &WelcomeScreen{
		tips: []Tip{
			{
				Title:       "Ask Anything",
				Description: "Just type naturally. rigrun understands code questions.",
				Example:     "What does this function do?",
				Icon:        "[Chat]",
			},
			{
				Title:       "Reference Files",
				Description: "Use @file: to include file contents in your question.",
				Example:     "@file:main.go explain this code",
				Icon:        "[File]",
			},
			{
				Title:       "Command Palette",
				Description: "Press Ctrl+P to fuzzy search all commands instantly.",
				Example:     "Ctrl+P -> type 'save' -> Enter",
				Icon:        "[Keys]",
			},
			{
				Title:       "Get Help",
				Description: "Type /help for a quick reference of all features.",
				Example:     "/help",
				Icon:        "[Help]",
			},
			{
				Title:       "You're Ready!",
				Description: "Start coding with AI. The terminal is yours.",
				Example:     "Let's go!",
				Icon:        "[Go!]",
			},
		},
	}
}

// Init initializes the welcome screen
func (w *WelcomeScreen) Init() tea.Cmd {
	return tea.Batch(
		w.blinkCursor(),
		w.startTyping(w.tips[0].Example),
	)
}

type cursorBlinkMsg struct{}
type welcomeTypeMsg struct {
	target string
	index  int
}

func (w *WelcomeScreen) blinkCursor() tea.Cmd {
	return tea.Tick(530*time.Millisecond, func(t time.Time) tea.Msg {
		return cursorBlinkMsg{}
	})
}

func (w *WelcomeScreen) startTyping(text string) tea.Cmd {
	w.typeTarget = text
	w.typeIndex = 0
	w.typing = ""
	return w.typeTick(text, 1)
}

func (w *WelcomeScreen) typeTick(target string, index int) tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return welcomeTypeMsg{target: target, index: index}
	})
}

// Update handles messages
func (w *WelcomeScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return w, tea.Quit
		case "enter", " ", "n":
			if w.currentTip < len(w.tips)-1 {
				w.currentTip++
				return w, w.startTyping(w.tips[w.currentTip].Example)
			}
			return w, tea.Quit
		case "p", "b":
			if w.currentTip > 0 {
				w.currentTip--
				return w, w.startTyping(w.tips[w.currentTip].Example)
			}
		}

	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height

	case cursorBlinkMsg:
		w.showCursor = !w.showCursor
		return w, w.blinkCursor()

	case welcomeTypeMsg:
		if msg.target == w.typeTarget && msg.index <= len(msg.target) {
			w.typing = msg.target[:msg.index]
			w.typeIndex = msg.index
			if msg.index < len(msg.target) {
				return w, w.typeTick(msg.target, msg.index+1)
			}
		}
	}

	return w, nil
}

// View renders the welcome screen
func (w *WelcomeScreen) View() string {
	var s strings.Builder

	// Header
	header := lipgloss.NewStyle().
		Foreground(brandPrimary).
		Bold(true).
		MarginBottom(1).
		Render("  Welcome to rigrun!")

	s.WriteString("\n\n")
	s.WriteString(header)
	s.WriteString("\n\n")

	// Current tip
	tip := w.tips[w.currentTip]

	// Icon and title
	titleLine := fmt.Sprintf("  %s  %s", tip.Icon, highlightStyle.Render(tip.Title))
	s.WriteString(titleLine)
	s.WriteString("\n\n")

	// Description
	s.WriteString(dimStyle.Render("     " + tip.Description))
	s.WriteString("\n\n")

	// Example with typing effect
	exampleBox := w.renderExampleBox(tip)
	s.WriteString(exampleBox)
	s.WriteString("\n\n")

	// Progress dots
	s.WriteString("  ")
	for i := range w.tips {
		if i == w.currentTip {
			s.WriteString(highlightStyle.Render("*"))
		} else if i < w.currentTip {
			s.WriteString(successStyle.Render("*"))
		} else {
			s.WriteString(dimStyle.Render("o"))
		}
		s.WriteString(" ")
	}
	s.WriteString("\n\n")

	// Navigation
	nav := dimStyle.Render("  ENTER: Next  |  P: Previous  |  Q: Start using rigrun")
	s.WriteString(nav)

	return w.centerVertically(s.String())
}

func (w *WelcomeScreen) renderExampleBox(tip Tip) string {
	// Fake terminal prompt
	prompt := dimStyle.Render("  > ")

	// Typing text
	typed := highlightStyle.Render(w.typing)

	// Cursor
	cursor := ""
	if w.showCursor && w.typeIndex >= len(w.typeTarget) {
		cursor = "_"
	} else if w.showCursor {
		cursor = "|"
	}

	// Box
	content := prompt + typed + dimStyle.Render(cursor)

	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(brandPrimary).
		Padding(1, 2).
		Width(50).
		Render(content)

	return "     " + box
}

func (w *WelcomeScreen) centerVertically(content string) string {
	if w.height == 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	padding := (w.height - len(lines)) / 3
	if padding < 0 {
		padding = 0
	}

	var s strings.Builder
	for i := 0; i < padding; i++ {
		s.WriteString("\n")
	}
	s.WriteString(content)
	return s.String()
}
