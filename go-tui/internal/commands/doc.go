// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package commands provides the slash command system for the TUI.
//
// This package handles parsing and executing slash commands in the chat
// interface, providing autocomplete and command registration.
//
// # Key Types
//
//   - Registry: Command registry with all available commands
//   - Handler: Command handler interface
//   - ParseResult: Parsed command with name and arguments
//   - Completer: Tab completion for commands and arguments
//
// # Built-in Commands
//
//   - /help: Show available commands
//   - /model: Switch models
//   - /clear: Clear conversation
//   - /export: Export conversation
//   - /index: Manage codebase index
//   - /benchmark: Run model benchmarks
//
// # Usage
//
// Parse and execute a command:
//
//	result := commands.Parse(input)
//	if result.IsCommand {
//	    return registry.Execute(ctx, result)
//	}
//
// Get completions:
//
//	completions := completer.Complete("/mo")
//	// Returns ["/model"]
package commands
