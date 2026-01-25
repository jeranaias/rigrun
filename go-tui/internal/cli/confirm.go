// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// confirm.go - Unified confirmation handling for all CLI commands in rigrun.
//
// USABILITY: TTY detection for proper terminal handling
//
// This file eliminates the duplication of confirmation logic across
// 23+ command files. Some commands used --confirm flag, some used
// interactive prompts, some used both inconsistently.
//
// This standardizes on a single pattern:
//   1. If --confirm flag is present, proceed without prompting
//   2. If --json mode, require --confirm flag (no interactive prompts in JSON mode)
//   3. If stdin is not a TTY, require --confirm flag (can't prompt)
//   4. Otherwise, show interactive prompt for confirmation
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// =============================================================================
// UNIFIED CONFIRMATION HANDLING
// =============================================================================

// USABILITY: Options struct improves API clarity
// ConfirmationOptions provides a clear, self-documenting API for confirmation functions.
// This replaces multiple boolean parameters that were unclear at call sites.
//
// Before (unclear):
//
//	RequireConfirmation(true, "delete", false)  // What do true/false mean?
//
// After (self-documenting):
//
//	RequireConfirmationWithOpts("delete", ConfirmationOptions{ConfirmFlag: true})
type ConfirmationOptions struct {
	// ConfirmFlag indicates if --confirm flag was passed (skip interactive prompt)
	ConfirmFlag bool
	// JSONMode indicates if --json flag was passed (requires ConfirmFlag for destructive actions)
	JSONMode bool
}

// RequireConfirmationWithOpts checks if the user has confirmed a destructive action.
// USABILITY: Options struct improves API clarity
//
// Confirmation flow:
//  1. If opts.ConfirmFlag is true (--confirm), return true immediately
//  2. If opts.JSONMode is true, return error (JSON mode requires --confirm flag)
//  3. If stdin is not a TTY, return error (can't prompt)
//  4. Otherwise, show interactive prompt and wait for user input
//
// Parameters:
//
//	action - description of the action (e.g., "delete all sessions")
//	opts   - confirmation options struct for clear, self-documenting API
//
// Returns:
//
//	bool  - true if confirmed, false if cancelled
//	error - non-nil if confirmation is required but not provided
//
// Example:
//
//	confirmed, err := RequireConfirmationWithOpts("delete all audit logs", ConfirmationOptions{
//	    ConfirmFlag: confirmFlag,
//	    JSONMode:    jsonMode,
//	})
func RequireConfirmationWithOpts(action string, opts ConfirmationOptions) (bool, error) {
	return RequireConfirmation(opts.ConfirmFlag, action, opts.JSONMode)
}

// RequireConfirmationWithDetailsOpts is like RequireConfirmationWithOpts but shows
// additional details before prompting.
// USABILITY: Options struct improves API clarity
//
// Parameters:
//
//	action  - description of the action (e.g., "delete session")
//	details - map of detail labels to values (e.g., {"Session ID": "abc123"})
//	opts    - confirmation options struct for clear, self-documenting API
//
// Example:
//
//	details := map[string]string{
//	    "Session ID": session.ID,
//	    "Created":    session.CreatedAt.String(),
//	}
//	confirmed, err := RequireConfirmationWithDetailsOpts("delete this session", details, ConfirmationOptions{
//	    ConfirmFlag: confirmFlag,
//	    JSONMode:    jsonMode,
//	})
func RequireConfirmationWithDetailsOpts(action string, details map[string]string, opts ConfirmationOptions) (bool, error) {
	return RequireConfirmationWithDetails(opts.ConfirmFlag, action, details, opts.JSONMode)
}

// ConfirmDangerousActionWithOpts is a specialized confirmation for highly dangerous operations
// that require typing a specific phrase to confirm (e.g., "DELETE ALL").
// USABILITY: Options struct improves API clarity
//
// Parameters:
//
//	action        - description of the action
//	confirmPhrase - phrase user must type to confirm (e.g., "DELETE ALL")
//	opts          - confirmation options struct for clear, self-documenting API
//
// Example:
//
//	confirmed, err := ConfirmDangerousActionWithOpts("permanently delete all audit logs", "DELETE ALL", ConfirmationOptions{
//	    ConfirmFlag: confirmFlag,
//	    JSONMode:    jsonMode,
//	})
func ConfirmDangerousActionWithOpts(action, confirmPhrase string, opts ConfirmationOptions) (bool, error) {
	return ConfirmDangerousAction(opts.ConfirmFlag, action, confirmPhrase, opts.JSONMode)
}

// RequireConfirmation checks if the user has confirmed a destructive action.
// It implements a consistent confirmation pattern across all CLI commands.
//
// Deprecated: Use RequireConfirmationWithOpts for clearer API.
// This function is kept for backward compatibility.
//
// Confirmation flow:
//  1. If confirmFlag is true (--confirm), return true immediately
//  2. If jsonMode is true, return error (JSON mode requires --confirm flag)
//  3. Otherwise, show interactive prompt and wait for user input
//
// Parameters:
//
//	confirmFlag - true if --confirm flag was passed
//	action      - description of the action (e.g., "delete all sessions")
//	jsonMode    - true if --json flag was passed
//
// Returns:
//
//	bool  - true if confirmed, false if cancelled
//	error - non-nil if confirmation is required but not provided (JSON mode)
//
// Example:
//
//	confirmed, err := RequireConfirmation(confirmFlag, "delete all audit logs", jsonMode)
//	if err != nil {
//	    return err  // JSON mode without --confirm
//	}
//	if !confirmed {
//	    fmt.Println("Cancelled.")
//	    return nil
//	}
//	// Proceed with destructive action
func RequireConfirmation(confirmFlag bool, action string, jsonMode bool) (bool, error) {
	// If --confirm flag is present, proceed without prompting
	if confirmFlag {
		return true, nil
	}

	// In JSON mode, --confirm flag is required (no interactive prompts)
	if jsonMode {
		return false, fmt.Errorf("confirmation required: use --confirm flag for destructive actions in JSON mode")
	}

	// USABILITY: TTY detection for proper terminal handling
	// Can't prompt if stdin is not a TTY (e.g., piped input, cron jobs, CI/CD)
	if !IsTTY() {
		return false, fmt.Errorf("confirmation required but stdin is not a terminal; use --confirm flag")
	}

	// Show interactive prompt
	fmt.Println()
	fmt.Printf("Are you sure you want to %s? [y/N]: ", action)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	// Parse response
	response := strings.ToLower(strings.TrimSpace(input))
	confirmed := response == "y" || response == "yes"

	return confirmed, nil
}

// RequireConfirmationWithDetails is like RequireConfirmation but shows
// additional details before prompting.
//
// Deprecated: Use RequireConfirmationWithDetailsOpts for clearer API.
// This function is kept for backward compatibility.
//
// Parameters:
//
//	confirmFlag - true if --confirm flag was passed
//	action      - description of the action (e.g., "delete session")
//	details     - map of detail labels to values (e.g., {"Session ID": "abc123", "Created": "2024-01-01"})
//	jsonMode    - true if --json flag was passed
//
// Example:
//
//	details := map[string]string{
//	    "Session ID": session.ID,
//	    "Created":    session.CreatedAt.String(),
//	    "Queries":    fmt.Sprintf("%d", len(session.Messages)),
//	}
//	confirmed, err := RequireConfirmationWithDetails(confirmFlag, "delete this session", details, jsonMode)
func RequireConfirmationWithDetails(confirmFlag bool, action string, details map[string]string, jsonMode bool) (bool, error) {
	// If --confirm flag is present, proceed without prompting
	if confirmFlag {
		return true, nil
	}

	// In JSON mode, --confirm flag is required
	if jsonMode {
		return false, fmt.Errorf("confirmation required: use --confirm flag for destructive actions in JSON mode")
	}

	// USABILITY: TTY detection for proper terminal handling
	// Can't prompt if stdin is not a TTY (e.g., piped input, cron jobs, CI/CD)
	if !IsTTY() {
		return false, fmt.Errorf("confirmation required but stdin is not a terminal; use --confirm flag")
	}

	// Show details
	fmt.Println()
	fmt.Println(WarningStyle.Render("WARNING: Destructive Action"))
	fmt.Println(RenderSeparator(50))
	fmt.Println()

	// Display details in consistent format
	for label, value := range details {
		fmt.Printf("  %s%s\n", RenderLabel(label+":", 20), value)
	}

	fmt.Println()
	fmt.Println(ErrorStyle.Render("This action cannot be undone."))
	fmt.Println()
	fmt.Printf("Are you sure you want to %s? [y/N]: ", action)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	// Parse response
	response := strings.ToLower(strings.TrimSpace(input))
	confirmed := response == "y" || response == "yes"

	return confirmed, nil
}

// ConfirmDangerousAction is a specialized confirmation for highly dangerous operations
// that require typing a specific phrase to confirm (e.g., "DELETE ALL").
//
// Deprecated: Use ConfirmDangerousActionWithOpts for clearer API.
// This function is kept for backward compatibility.
//
// Parameters:
//
//	confirmFlag    - true if --confirm flag was passed
//	action         - description of the action
//	confirmPhrase  - phrase user must type to confirm (e.g., "DELETE ALL")
//	jsonMode       - true if --json flag was passed
//
// Example:
//
//	confirmed, err := ConfirmDangerousAction(confirmFlag, "permanently delete all audit logs", "DELETE ALL", jsonMode)
func ConfirmDangerousAction(confirmFlag bool, action, confirmPhrase string, jsonMode bool) (bool, error) {
	// If --confirm flag is present, proceed without prompting
	if confirmFlag {
		return true, nil
	}

	// In JSON mode, --confirm flag is required
	if jsonMode {
		return false, fmt.Errorf("confirmation required: use --confirm flag for destructive actions in JSON mode")
	}

	// USABILITY: TTY detection for proper terminal handling
	// Can't prompt if stdin is not a TTY (e.g., piped input, cron jobs, CI/CD)
	if !IsTTY() {
		return false, fmt.Errorf("confirmation required but stdin is not a terminal; use --confirm flag")
	}

	// Show danger warning
	fmt.Println()
	fmt.Println(ErrorStyle.Render("[!!] DANGER: HIGHLY DESTRUCTIVE ACTION [!!]"))
	fmt.Println(RenderSeparator(50))
	fmt.Println()
	fmt.Printf("You are about to: %s\n", ErrorStyle.Render(action))
	fmt.Println()
	fmt.Println(ErrorStyle.Render("THIS ACTION CANNOT BE UNDONE."))
	fmt.Println()
	fmt.Printf("To confirm, type '%s' (without quotes): ", confirmPhrase)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	// Check if user typed the exact phrase
	response := strings.TrimSpace(input)
	confirmed := response == confirmPhrase

	if !confirmed {
		fmt.Println()
		fmt.Println(DimStyle.Render("Confirmation phrase did not match. Cancelled."))
		fmt.Println()
	}

	return confirmed, nil
}

// =============================================================================
// HELPER FUNCTIONS FOR COMMON CONFIRMATION PATTERNS
// =============================================================================

// ShowCancellationMessage displays a standard cancellation message.
// Use this after RequireConfirmation returns false.
func ShowCancellationMessage() {
	fmt.Println()
	fmt.Println(DimStyle.Render("Cancelled."))
	fmt.Println()
}

// ShowConfirmationRequired displays a message when --confirm flag is needed.
// Use this in JSON mode or when showing usage for destructive commands.
func ShowConfirmationRequired(command string) {
	fmt.Println()
	fmt.Println(WarningStyle.Render("Confirmation required for destructive actions."))
	fmt.Println()
	fmt.Printf("To proceed, run:\n  %s --confirm\n", command)
	fmt.Println()
}

// PromptYesNo prompts the user with a yes/no question.
// Returns true for yes, false for no.
// Returns false if stdin is not a TTY (cannot prompt).
// This is for simple yes/no prompts that are not destructive confirmations.
//
// Example:
//
//	if PromptYesNo("Enable audit logging?") {
//	    // Enable it
//	}
func PromptYesNo(question string) bool {
	// USABILITY: TTY detection for proper terminal handling
	if !IsTTY() {
		return false
	}

	fmt.Printf("%s [y/N]: ", question)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response := strings.ToLower(strings.TrimSpace(input))
	return response == "y" || response == "yes"
}

// PromptChoice prompts the user to choose from a list of options.
// Returns the index of the selected option (0-based).
// Returns -1 if cancelled, invalid input, or stdin is not a TTY.
//
// Example:
//
//	options := []string{"Delete", "Archive", "Export"}
//	choice := PromptChoice("What would you like to do?", options)
//	if choice == 0 {
//	    // Delete
//	}
func PromptChoice(question string, options []string) int {
	// USABILITY: TTY detection for proper terminal handling
	if !IsTTY() {
		return -1
	}

	fmt.Println()
	fmt.Println(question)
	fmt.Println()

	for i, option := range options {
		fmt.Printf("  %d) %s\n", i+1, option)
	}

	fmt.Println()
	fmt.Printf("Enter choice (1-%d): ", len(options))

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return -1
	}

	response := strings.TrimSpace(input)
	var choice int
	_, err = fmt.Sscanf(response, "%d", &choice)
	if err != nil || choice < 1 || choice > len(options) {
		return -1
	}

	return choice - 1 // Convert to 0-based index
}
