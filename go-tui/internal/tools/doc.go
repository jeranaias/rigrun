// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
//
// This package implements all tool executors for the agentic capabilities,
// including file operations, shell commands, and web interactions.
// All tools implement proper timeout, validation, and resource cleanup.
//
// # Key Types
//
//   - Tool: Tool definition with name, description, and parameters
//   - Executor: Interface for executing tools
//   - Result: Tool execution result with output and status
//   - RiskLevel: Security risk level for tool operations
//
// # Available Tools
//
// File Tools:
//   - Read: Read file contents with security validation
//   - Write: Write files with path validation
//   - Edit: Edit files with diff preview
//
// Search Tools:
//   - Glob: File pattern matching
//   - Grep: Content search with regex
//
// System Tools:
//   - Bash: Shell command execution (restricted)
//
// Web Tools:
//   - WebFetch: Fetch and process web content
//   - WebSearch: DuckDuckGo search
//
// # Security
//
// All tools implement comprehensive security validation:
//   - Path traversal prevention
//   - Sensitive file protection
//   - Shell command restrictions
//   - TLS for web requests
package tools
