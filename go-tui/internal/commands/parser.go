// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package commands provides the slash command system for the TUI.
package commands

import (
	"strings"
	"unicode"
)

// =============================================================================
// PARSE RESULT
// =============================================================================

// ParseResult contains the result of parsing user input.
type ParseResult struct {
	// IsCommand is true if the input starts with /
	IsCommand bool

	// Command is the matched command (nil if not found)
	Command *Command

	// CommandName is the raw command name (e.g., "/help")
	CommandName string

	// Args are the parsed arguments
	Args []string

	// RawInput is the original input string
	RawInput string

	// RawArgs is the unparsed arguments portion
	RawArgs string

	// Error if command not found or parsing failed
	Error error
}

// =============================================================================
// PARSER
// =============================================================================

// Parser handles parsing of slash commands and their arguments.
type Parser struct {
	registry *Registry
}

// NewParser creates a new parser with the given registry.
func NewParser(registry *Registry) *Parser {
	return &Parser{registry: registry}
}

// Parse parses user input and returns the parse result.
// Returns IsCommand=false if the input doesn't start with /
func (p *Parser) Parse(input string) ParseResult {
	input = strings.TrimSpace(input)

	result := ParseResult{
		RawInput: input,
	}

	// Check if this is a command
	if !strings.HasPrefix(input, "/") {
		result.IsCommand = false
		return result
	}

	result.IsCommand = true

	// Extract command name and arguments
	parts := splitCommandLine(input)
	if len(parts) == 0 {
		return result
	}

	result.CommandName = parts[0]
	if len(parts) > 1 {
		result.Args = parts[1:]
		// Reconstruct raw args
		idx := strings.Index(input, result.CommandName)
		if idx >= 0 {
			afterCmd := input[idx+len(result.CommandName):]
			result.RawArgs = strings.TrimSpace(afterCmd)
		}
	}

	// Look up the command
	result.Command = p.registry.Get(result.CommandName)

	return result
}

// ParseArgs parses a raw argument string into individual arguments.
// Handles quoted strings with spaces.
func ParseArgs(input string) []string {
	return splitCommandLine(input)
}

// =============================================================================
// ARGUMENT PARSING
// =============================================================================

// splitCommandLine splits a command line into tokens, respecting quotes.
// Supports both single and double quotes for arguments with spaces.
func splitCommandLine(input string) []string {
	var tokens []string
	var current strings.Builder
	var inSingleQuote, inDoubleQuote bool

	for i := 0; i < len(input); i++ {
		char := rune(input[i])

		switch {
		case char == '\'' && !inDoubleQuote:
			// Toggle single quote mode
			inSingleQuote = !inSingleQuote
			// Don't include the quote in the token

		case char == '"' && !inSingleQuote:
			// Toggle double quote mode
			inDoubleQuote = !inDoubleQuote
			// Don't include the quote in the token

		case char == '\\' && i+1 < len(input) && (inDoubleQuote || inSingleQuote):
			// Escape sequence inside quotes
			next := rune(input[i+1])
			if next == '"' || next == '\'' || next == '\\' {
				current.WriteRune(next)
				i++ // Skip the next character
			} else {
				current.WriteRune(char)
			}

		case unicode.IsSpace(char) && !inSingleQuote && !inDoubleQuote:
			// Space outside quotes - end current token
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}

		default:
			// Regular character
			current.WriteRune(char)
		}
	}

	// Don't forget the last token
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// IsCommand returns true if the input appears to be a command.
func IsCommand(input string) bool {
	return strings.HasPrefix(strings.TrimSpace(input), "/")
}

// ExtractCommandName extracts just the command name from input.
// e.g., "/model qwen2.5" -> "/model"
func ExtractCommandName(input string) string {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "/") {
		return ""
	}

	// Find end of command name (first space or end of string)
	end := strings.IndexFunc(input, unicode.IsSpace)
	if end == -1 {
		return input
	}
	return input[:end]
}

// GetPartialCommand returns the partial command being typed.
// Returns empty string if not in command mode.
func GetPartialCommand(input string) string {
	if !strings.HasPrefix(input, "/") {
		return ""
	}

	// Find end of command (first space)
	end := strings.IndexFunc(input, unicode.IsSpace)
	if end == -1 {
		// Still typing command name
		return input
	}

	// Command is complete, return empty
	return ""
}

// GetPartialArg returns the partial argument being typed.
// Returns the arg index and partial text.
func GetPartialArg(input string) (int, string) {
	parts := splitCommandLine(input)
	if len(parts) <= 1 {
		return 0, ""
	}

	// Check if we're in the middle of an argument or starting a new one
	trimmed := strings.TrimSpace(input)
	if strings.HasSuffix(trimmed, " ") || strings.HasSuffix(trimmed, "\"") || strings.HasSuffix(trimmed, "'") {
		// Starting a new argument
		return len(parts) - 1, ""
	}

	// In the middle of an argument
	return len(parts) - 2, parts[len(parts)-1]
}

// ValidateArgs validates arguments against a command's argument definitions.
func ValidateArgs(cmd *Command, args []string) error {
	if cmd == nil {
		return nil
	}

	// Check required arguments
	for i, argDef := range cmd.Args {
		if argDef.Required && i >= len(args) {
			return &ValidationError{
				Command:  cmd.Name,
				Arg:      argDef.Name,
				Message:  "required argument missing",
				Expected: argDef.Description,
			}
		}

		// Validate enum values
		if i < len(args) && argDef.Type == ArgTypeEnum && len(argDef.Values) > 0 {
			valid := false
			for _, v := range argDef.Values {
				if strings.EqualFold(args[i], v) {
					valid = true
					break
				}
			}
			if !valid {
				return &ValidationError{
					Command:  cmd.Name,
					Arg:      argDef.Name,
					Message:  "invalid value",
					Got:      args[i],
					Expected: strings.Join(argDef.Values, ", "),
				}
			}
		}
	}

	return nil
}

// =============================================================================
// VALIDATION ERROR
// =============================================================================

// ValidationError represents an argument validation error.
type ValidationError struct {
	Command  string
	Arg      string
	Message  string
	Got      string
	Expected string
}

func (e *ValidationError) Error() string {
	msg := e.Command + ": " + e.Message
	if e.Arg != "" {
		msg += " for argument '" + e.Arg + "'"
	}
	if e.Got != "" {
		msg += " (got: " + e.Got + ")"
	}
	if e.Expected != "" {
		msg += " - expected: " + e.Expected
	}
	return msg
}
