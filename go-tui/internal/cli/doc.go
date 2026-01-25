// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cli provides command-line interface parsing and execution for rigrun.
//
// This package implements all CLI commands for the rigrun TUI application,
// providing both interactive and non-interactive modes with comprehensive
// NIST 800-53 compliance features for DoD IL5 environments.
//
// # Key Types
//
//   - Command: Enumeration of all available CLI commands
//   - Args: Parsed command-line arguments with global and command-specific flags
//   - OutputFormatter: JSON and text output formatting for SIEM integration
//
// # Usage
//
// Parse and execute commands:
//
//	args := cli.ParseArgs(os.Args[1:])
//	switch args.Cmd {
//	case cli.CmdAsk:
//	    return cli.RunAsk(args)
//	case cli.CmdChat:
//	    return cli.RunChat(args)
//	// ... other commands
//	}
//
// # Commands Overview
//
// Core Commands:
//   - ask: Single question query
//   - chat: Interactive chat session
//   - status: System status display
//   - config: Configuration management
//   - setup: First-run wizard
//
// Security Commands (NIST 800-53):
//   - audit: Audit log management (AU-5, AU-6, AU-9, AU-11)
//   - auth: Authentication (IA-2)
//   - rbac: Role-based access control (AC-5, AC-6)
//   - encrypt: Data encryption (SC-28)
//   - classify: Classification levels
//   - consent: System use notification (AC-8)
//
// All commands support --json flag for SIEM integration.
package cli
