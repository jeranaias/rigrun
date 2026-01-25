// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

/*
Package chat provides the main chat view component for the rigrun TUI application.

The chat package implements a complete terminal-based chat interface using the
Bubble Tea framework. It provides an interactive, real-time conversation
experience with LLM models through Ollama.

# Key Components

## Model (model.go)

The Model struct is the central Bubble Tea model that maintains all chat state:
  - Conversation history and message management
  - Input handling with multi-line support
  - Viewport for message scrolling
  - Streaming state for real-time LLM responses
  - Routing configuration (local, cloud, hybrid modes)

## View Rendering (view.go)

Rendering logic for the complete chat interface:
  - Header with model name and status indicators
  - Message bubbles with role-specific styling (user, assistant, system, tool)
  - Code block syntax highlighting
  - Search term highlighting with Unicode support
  - Status bar with context usage, cost tracking, and routing mode

## Update Loop (update.go)

Handles all Bubble Tea messages and user interactions:
  - Keyboard input processing
  - Stream token handling
  - Ollama connection management
  - Window resize handling
  - Command execution

## Streaming (streaming.go)

Optimized streaming implementation for smooth LLM responses:
  - StreamingBuffer for batched token rendering
  - Flicker-free updates at capped frame rates
  - Thread-safe streaming state management

## Commands (commands.go)

Slash command handler registry supporting:
  - /help - Show available commands
  - /clear - Clear conversation
  - /save, /load - Conversation persistence
  - /model - Model switching
  - /mode - Routing mode (local, cloud, hybrid)
  - /export - Export conversation to file
  - /audit - View security audit log (IL5)

## Vim Navigation (vim.go)

Optional Vim-style modal editing:
  - Normal mode for navigation (j/k scroll, gg/G jump)
  - Insert mode for text input
  - Visual mode for selection
  - Command mode for :commands

# Usage

Create a new chat model and run it as a Bubble Tea program:

	client := ollama.NewClient("http://localhost:11434")
	model := chat.New(chat.Options{
		Client:    client,
		ModelName: "qwen2.5-coder:14b",
	})
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}

# Security Features (IL5 Compliance)

The chat package includes DoD IL5 compliance features:
  - Offline mode for air-gapped environments
  - Audit logging of all interactions
  - Security status commands
  - Session timeout handling
*/
package chat
