// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

/*
Package components provides reusable UI components for the rigrun TUI application.

This package contains a collection of styled, interactive components built on
top of the Bubble Tea and Lip Gloss libraries. Each component is designed to
be visually polished and consistent with the rigrun design language.

# Core Components

## Input Components

InputArea (input.go) - Styled text input with character counter and multi-line support.
CompletionPopup (completion.go) - Tab completion popup for commands and @mentions.
CommandPalette (palette.go) - Fuzzy-searchable command palette (Ctrl+P).

## Display Components

Header (header.go) - Application header with model name, status, and offline badge.
StatusBar (statusbar.go) - Bottom status bar with routing mode, costs, and shortcuts.
MessageBubble (message.go) - Styled message bubbles for chat messages.
CodeBlock (codeblock.go) - Syntax-highlighted code blocks using Chroma.
DiffViewer (diff_viewer.go) - Side-by-side and unified diff display.

## Progress and Feedback

Spinner (spinner.go) - Animated spinner with customizable styles.
ProgressIndicator (progress.go) - Progress bars for agentic operations.
ErrorDisplay (error.go) - Smart error messages with suggestions.

## Security Components (IL5 Compliance)

ClassificationBanner (classification_banner.go) - Top/bottom banners showing data classification.
ConsentBanner (consent_banner.go) - DoD System Use Notification banner (NIST 800-53 AC-8).
SessionTimeoutOverlay (session_timeout_overlay.go) - Session timeout warning and logout.

## Specialized Views

Tutorial (tutorial.go) - Interactive onboarding tutorial overlay.
Welcome (welcome.go) - First-run welcome screen.
PlanView (plan_view.go) - Display and edit execution plans.
TaskList (task_list.go) - Task management view.
CostDashboard (cost_dashboard.go) - Cost tracking and analysis.
BenchmarkView (benchmark_view.go) - Performance benchmark results.

# Key Types

## Theme Integration

All components accept a *styles.Theme for consistent styling:

	theme := styles.NewTheme()
	header := components.NewHeader(theme)
	header.SetWidth(80)
	header.SetModel("qwen2.5-coder:14b")
	view := header.View()

## Bubble Tea Integration

Most components implement the Bubble Tea Model interface:

	type Component interface {
		Init() tea.Cmd
		Update(tea.Msg) (Component, tea.Cmd)
		View() string
	}

## Error Handling

The error components provide intelligent error display:

	display := components.NewErrorDisplay("Connection refused", components.ErrorCategoryNetwork)
	display.SetSuggestions([]string{"Check if Ollama is running", "Verify network connection"})
	view := display.Render(80)

# Helper Functions

The package includes shared helper functions in helpers.go:
  - toStr() - Integer to string conversion without fmt
  - formatDuration() - Human-readable duration formatting
  - truncateWithEllipsis() - Safe string truncation with Unicode support
*/
package components
