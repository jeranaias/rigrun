// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// terminal.go - Terminal detection and handling for rigrun CLI.
//
// USABILITY: TTY detection for proper terminal handling
//
// This file provides utilities for detecting terminal capabilities:
// - TTY detection for stdin/stdout
// - Terminal width detection for text wrapping
// - Color output control based on TTY and NO_COLOR
// - ANSI escape sequence management
//
// These utilities ensure proper behavior in different environments:
// - Interactive terminals (full colors, prompts)
// - Piped output (no colors, no prompts)
// - CI/CD environments (respects NO_COLOR)
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"os"
	"strings"
	"sync"

	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// =============================================================================
// TTY DETECTION
// USABILITY: TTY detection for proper terminal handling
// =============================================================================

// IsTTY returns true if stdin is a terminal.
// Use this to determine if interactive prompts are possible.
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// IsStdoutTTY returns true if stdout is a terminal.
// Use this to determine if colored output should be used.
func IsStdoutTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// IsStderrTTY returns true if stderr is a terminal.
func IsStderrTTY() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// =============================================================================
// TERMINAL WIDTH DETECTION
// USABILITY: TTY detection for proper terminal handling
// =============================================================================

const (
	// DefaultTerminalWidth is the fallback width when detection fails
	DefaultTerminalWidth = 80

	// MinTerminalWidth is the minimum width we'll use for wrapping
	MinTerminalWidth = 40
)

// GetTerminalWidth returns the current terminal width.
// Returns DefaultTerminalWidth (80) if width cannot be determined.
func GetTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return DefaultTerminalWidth
	}
	if width < MinTerminalWidth {
		return MinTerminalWidth
	}
	return width
}

// GetTerminalSize returns both width and height of the terminal.
// Returns defaults (80x24) if size cannot be determined.
func GetTerminalSize() (width, height int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 || h <= 0 {
		return DefaultTerminalWidth, 24
	}
	return w, h
}

// WrapText wraps text to fit within the specified width.
// If maxWidth is 0 or negative, uses GetTerminalWidth().
// Preserves existing newlines and handles word boundaries.
func WrapText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = GetTerminalWidth()
	}

	// Leave some margin for readability
	if maxWidth > 10 {
		maxWidth -= 2
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		// If line fits, use it as-is
		if len(line) <= maxWidth {
			result.WriteString(line)
			continue
		}

		// Word wrap the line
		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}

		currentLine := words[0]
		for _, word := range words[1:] {
			if len(currentLine)+1+len(word) <= maxWidth {
				currentLine += " " + word
			} else {
				result.WriteString(currentLine)
				result.WriteString("\n")
				currentLine = word
			}
		}
		result.WriteString(currentLine)
	}

	return result.String()
}

// =============================================================================
// COLOR OUTPUT CONTROL
// USABILITY: TTY detection for proper terminal handling
// =============================================================================

var (
	// colorsEnabled caches the color support decision
	colorsEnabled     bool
	colorsEnabledOnce sync.Once
)

// ColorsEnabled returns true if colored output should be used.
// Respects NO_COLOR environment variable and TTY detection.
// See https://no-color.org/ for the NO_COLOR specification.
func ColorsEnabled() bool {
	colorsEnabledOnce.Do(func() {
		// NO_COLOR takes precedence (any non-empty value disables colors)
		if os.Getenv("NO_COLOR") != "" {
			colorsEnabled = false
			return
		}

		// FORCE_COLOR overrides TTY detection
		if os.Getenv("FORCE_COLOR") != "" {
			colorsEnabled = true
			return
		}

		// Check if stdout is a TTY
		colorsEnabled = IsStdoutTTY()
	})
	return colorsEnabled
}

// ForceColorsEnabled allows overriding color detection (for testing).
// This should only be used in tests.
func ForceColorsEnabled(enabled bool) {
	colorsEnabled = enabled
	// Reset the once so it doesn't re-compute
	colorsEnabledOnce = sync.Once{}
	colorsEnabledOnce.Do(func() {
		colorsEnabled = enabled
	})
}

// GetColorProfile returns the appropriate termenv color profile.
// Returns Ascii (no colors) for non-TTY or when NO_COLOR is set.
func GetColorProfile() termenv.Profile {
	if !ColorsEnabled() {
		return termenv.Ascii
	}
	// Let termenv auto-detect the best profile for this terminal
	return termenv.ColorProfile()
}

// =============================================================================
// INTERACTIVE INPUT HELPERS
// USABILITY: TTY detection for proper terminal handling
// =============================================================================

// CanPrompt returns true if interactive prompts are possible.
// Prompts require stdin to be a TTY.
func CanPrompt() bool {
	return IsTTY()
}

// RequiresTTY returns an error if stdin is not a terminal.
// Use this at the start of commands that require interactive input.
func RequiresTTY(operation string) error {
	if !IsTTY() {
		return &TTYRequiredError{Operation: operation}
	}
	return nil
}

// TTYRequiredError is returned when an operation requires a TTY but none is available.
type TTYRequiredError struct {
	Operation string
}

func (e *TTYRequiredError) Error() string {
	if e.Operation != "" {
		return "stdin is not a terminal; cannot " + e.Operation + " interactively"
	}
	return "stdin is not a terminal; interactive input not available"
}

// =============================================================================
// TERMINAL CAPABILITY DETECTION
// =============================================================================

// TerminalCapabilities describes what the current terminal supports.
type TerminalCapabilities struct {
	IsTTY            bool
	IsStdoutTTY      bool
	IsStderrTTY      bool
	ColorsEnabled    bool
	Width            int
	Height           int
	ColorProfile     termenv.Profile
	SupportsUnicode  bool
	Supports256Color bool
	SupportsTrueColor bool
}

// GetTerminalCapabilities returns information about the current terminal.
func GetTerminalCapabilities() TerminalCapabilities {
	width, height := GetTerminalSize()
	profile := GetColorProfile()

	return TerminalCapabilities{
		IsTTY:            IsTTY(),
		IsStdoutTTY:      IsStdoutTTY(),
		IsStderrTTY:      IsStderrTTY(),
		ColorsEnabled:    ColorsEnabled(),
		Width:            width,
		Height:           height,
		ColorProfile:     profile,
		SupportsUnicode:  profile != termenv.Ascii,
		Supports256Color: profile >= termenv.ANSI256,
		SupportsTrueColor: profile >= termenv.TrueColor,
	}
}
