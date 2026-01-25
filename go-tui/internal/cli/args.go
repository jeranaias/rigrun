// args.go - Unified argument parsing for all CLI commands in rigrun.
//
// This file eliminates the duplication of argument parsing across
// 23+ command files. Each command previously had its own custom parsing
// logic with inconsistent handling of flags, subcommands, and values.
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// =============================================================================
// ARG PARSER - UNIFIED ARGUMENT PARSING FOR ALL COMMANDS
// =============================================================================

// ArgParser provides unified argument parsing for CLI commands.
// It handles multiple flag formats consistently:
//   - Long flags: --flag value or --flag=value
//   - Short flags: -f value
//   - Boolean flags: --flag (no value needed)
//   - Positional arguments: arguments without flags
//   - Subcommands: first positional argument
type ArgParser struct {
	subcommand string            // First positional arg (e.g., "show", "list", "clear")
	flags      map[string]string // String flags (--key=value)
	boolFlags  map[string]bool   // Boolean flags (--confirm)
	positional []string          // All positional arguments including subcommand
	raw        []string          // Original raw arguments
}

// NewArgParser creates a new argument parser from raw arguments.
// It automatically parses flags in multiple formats and separates positional args.
//
// Supported flag formats:
//
//	--flag value     Long flag with space-separated value
//	--flag=value     Long flag with equals sign
//	-f value         Short flag with space-separated value
//	--flag           Boolean flag (no value)
//
// Example:
//
//	args := NewArgParser([]string{"show", "--lines", "50", "--since=2024-01-01", "--json"})
//	args.Subcommand()        // "show"
//	args.Flag("lines")       // "50"
//	args.Flag("since")       // "2024-01-01"
//	args.BoolFlag("json")    // true
func NewArgParser(raw []string) *ArgParser {
	parser := &ArgParser{
		flags:      make(map[string]string),
		boolFlags:  make(map[string]bool),
		positional: make([]string, 0),
		raw:        raw,
	}

	// Parse arguments
	i := 0
	for i < len(raw) {
		arg := raw[i]

		// Check if it's a flag
		if strings.HasPrefix(arg, "-") {
			// Handle --flag=value format
			if strings.Contains(arg, "=") {
				parts := strings.SplitN(arg, "=", 2)
				flagName := strings.TrimLeft(parts[0], "-")
				flagValue := parts[1]

				// Boolean flags can be explicit: --json=true, --json=false
				if flagValue == "true" || flagValue == "false" {
					parser.boolFlags[flagName] = flagValue == "true"
				} else {
					parser.flags[flagName] = flagValue
				}
				i++
				continue
			}

			// Extract flag name (remove leading dashes)
			flagName := strings.TrimLeft(arg, "-")

			// Check if next arg is a value (not a flag and not end of args)
			if i+1 < len(raw) && !strings.HasPrefix(raw[i+1], "-") {
				// This is a flag with a value
				parser.flags[flagName] = raw[i+1]
				i += 2
			} else {
				// This is a boolean flag
				parser.boolFlags[flagName] = true
				i++
			}
		} else {
			// Positional argument
			parser.positional = append(parser.positional, arg)
			i++
		}
	}

	// First positional is the subcommand
	if len(parser.positional) > 0 {
		parser.subcommand = parser.positional[0]
	}

	return parser
}

// Subcommand returns the first positional argument (subcommand).
// Returns empty string if no positional arguments.
//
// Example: "audit show" -> "show"
func (p *ArgParser) Subcommand() string {
	return p.subcommand
}

// Flag returns the value of a string flag.
// Returns empty string if flag not found.
// Supports both long and short flag names.
//
// Example:
//
//	args.Flag("lines")      // --lines 50
//	args.Flag("l")          // -l 50
//	args.Flag("since")      // --since=2024-01-01
func (p *ArgParser) Flag(name string) string {
	// Try exact match first
	if val, ok := p.flags[name]; ok {
		return val
	}

	// Try without dashes
	name = strings.TrimLeft(name, "-")
	if val, ok := p.flags[name]; ok {
		return val
	}

	return ""
}

// FlagOrDefault returns the flag value or a default if not found.
func (p *ArgParser) FlagOrDefault(name, defaultValue string) string {
	if val := p.Flag(name); val != "" {
		return val
	}
	return defaultValue
}

// FlagInt returns the flag value as an integer.
// Returns 0 and error if flag is not a valid integer.
func (p *ArgParser) FlagInt(name string) (int, error) {
	val := p.Flag(name)
	if val == "" {
		return 0, fmt.Errorf("flag %s not found", name)
	}
	return strconv.Atoi(val)
}

// FlagIntOrDefault returns the flag value as an integer or a default.
// Returns default if flag not found or not a valid integer.
func (p *ArgParser) FlagIntOrDefault(name string, defaultValue int) int {
	val, err := p.FlagInt(name)
	if err != nil {
		return defaultValue
	}
	return val
}

// BoolFlag returns the value of a boolean flag.
// Returns false if flag not found.
// Supports both long and short flag names.
//
// Example:
//
//	args.BoolFlag("json")    // --json
//	args.BoolFlag("confirm") // --confirm
//	args.BoolFlag("y")       // -y
func (p *ArgParser) BoolFlag(name string) bool {
	// Try exact match first
	if val, ok := p.boolFlags[name]; ok {
		return val
	}

	// Try without dashes
	name = strings.TrimLeft(name, "-")
	if val, ok := p.boolFlags[name]; ok {
		return val
	}

	return false
}

// Positional returns the positional argument at the given index.
// Returns empty string if index out of bounds.
// Index 0 is the subcommand.
//
// Example: "audit show --lines 50"
//
//	args.Positional(0)  // "show" (subcommand)
//	args.Positional(1)  // "" (no second positional arg)
func (p *ArgParser) Positional(index int) string {
	if index < 0 || index >= len(p.positional) {
		return ""
	}
	return p.positional[index]
}

// PositionalFrom returns all positional arguments starting from index.
// Useful for joining remaining args into a query/message.
//
// Example: "audit search error in production"
//
//	args.PositionalFrom(1)  // []string{"search", "error", "in", "production"}
//	strings.Join(args.PositionalFrom(1), " ")  // "search error in production"
func (p *ArgParser) PositionalFrom(index int) []string {
	if index < 0 || index >= len(p.positional) {
		return []string{}
	}
	return p.positional[index:]
}

// PositionalCount returns the number of positional arguments.
func (p *ArgParser) PositionalCount() int {
	return len(p.positional)
}

// HasFlag returns true if the flag exists (either as string or bool flag).
func (p *ArgParser) HasFlag(name string) bool {
	name = strings.TrimLeft(name, "-")
	_, hasString := p.flags[name]
	_, hasBool := p.boolFlags[name]
	return hasString || hasBool
}

// Raw returns the original raw arguments.
func (p *ArgParser) Raw() []string {
	return p.raw
}

// =============================================================================
// HELPER FUNCTIONS FOR COMMON ARG PATTERNS
// =============================================================================

// ParseIntWithValidation parses an integer from a string and validates it's positive.
// Returns the integer and nil error if valid, or 0 and error if invalid.
func ParseIntWithValidation(s string, fieldName string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("%s is required", fieldName)
	}

	val, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", fieldName, err)
	}

	if val <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %d", fieldName, val)
	}

	return val, nil
}

// ParseBoolString parses a boolean from various string representations.
// Accepts: true/false, yes/no, y/n, 1/0, on/off (case-insensitive)
func ParseBoolString(s string) (bool, error) {
	s = strings.ToLower(strings.TrimSpace(s))

	switch s {
	case "true", "yes", "y", "1", "on":
		return true, nil
	case "false", "no", "n", "0", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", s)
	}
}

// JoinPositionalArgs joins positional arguments from the given index into a single string.
// This is useful for commands that accept multi-word queries or messages.
//
// Example: "audit search error in production" -> "error in production"
func JoinPositionalArgs(parser *ArgParser, startIndex int) string {
	return strings.Join(parser.PositionalFrom(startIndex), " ")
}
