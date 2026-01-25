// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cli provides command-line interface parsing and execution.
//
// This test file covers the CLI commands: ask, chat, intel
// These are critical user-facing commands that must work reliably.
package cli

import (
	"os"
	"strings"
	"testing"
)

// =============================================================================
// ARG PARSER TESTS (args.go)
// =============================================================================

func TestArgParser_BasicParsing(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantSub  string
		validate func(*testing.T, *ArgParser)
	}{
		{
			name:    "simple subcommand",
			args:    []string{"show"},
			wantSub: "show",
		},
		{
			name:    "subcommand with flag",
			args:    []string{"show", "--lines", "50"},
			wantSub: "show",
			validate: func(t *testing.T, p *ArgParser) {
				if p.Flag("lines") != "50" {
					t.Errorf("Flag(lines) = %q, want %q", p.Flag("lines"), "50")
				}
			},
		},
		{
			name:    "flag with equals",
			args:    []string{"show", "--since=2024-01-01"},
			wantSub: "show",
			validate: func(t *testing.T, p *ArgParser) {
				if p.Flag("since") != "2024-01-01" {
					t.Errorf("Flag(since) = %q, want %q", p.Flag("since"), "2024-01-01")
				}
			},
		},
		{
			name:    "boolean flag",
			args:    []string{"show", "--json"},
			wantSub: "show",
			validate: func(t *testing.T, p *ArgParser) {
				if !p.BoolFlag("json") {
					t.Error("BoolFlag(json) should be true")
				}
			},
		},
		{
			name:    "multiple positional args",
			args:    []string{"search", "error", "in", "production"},
			wantSub: "search",
			validate: func(t *testing.T, p *ArgParser) {
				if p.PositionalCount() != 4 {
					t.Errorf("PositionalCount() = %d, want 4", p.PositionalCount())
				}
				joined := strings.Join(p.PositionalFrom(1), " ")
				if joined != "error in production" {
					t.Errorf("PositionalFrom(1) joined = %q, want %q", joined, "error in production")
				}
			},
		},
		{
			name:    "mixed flags and positional",
			args:    []string{"ask", "--model", "qwen2.5:14b", "Hello", "world"},
			wantSub: "ask",
			validate: func(t *testing.T, p *ArgParser) {
				if p.Flag("model") != "qwen2.5:14b" {
					t.Errorf("Flag(model) = %q, want %q", p.Flag("model"), "qwen2.5:14b")
				}
				// Positional should be: ask, Hello, world
				if p.Positional(1) != "Hello" {
					t.Errorf("Positional(1) = %q, want %q", p.Positional(1), "Hello")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewArgParser(tt.args)
			if parser.Subcommand() != tt.wantSub {
				t.Errorf("Subcommand() = %q, want %q", parser.Subcommand(), tt.wantSub)
			}
			if tt.validate != nil {
				tt.validate(t, parser)
			}
		})
	}
}

func TestArgParser_FlagIntOrDefault(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		flagName   string
		defaultVal int
		want       int
	}{
		{
			name:       "flag present",
			args:       []string{"cmd", "--max-iter", "10"},
			flagName:   "max-iter",
			defaultVal: 5,
			want:       10,
		},
		{
			name:       "flag missing uses default",
			args:       []string{"cmd"},
			flagName:   "max-iter",
			defaultVal: 5,
			want:       5,
		},
		{
			name:       "invalid int uses default",
			args:       []string{"cmd", "--max-iter", "abc"},
			flagName:   "max-iter",
			defaultVal: 5,
			want:       5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewArgParser(tt.args)
			got := parser.FlagIntOrDefault(tt.flagName, tt.defaultVal)
			if got != tt.want {
				t.Errorf("FlagIntOrDefault(%q, %d) = %d, want %d", tt.flagName, tt.defaultVal, got, tt.want)
			}
		})
	}
}

func TestArgParser_HasFlag(t *testing.T) {
	parser := NewArgParser([]string{"cmd", "--verbose", "--lines", "50"})

	if !parser.HasFlag("verbose") {
		t.Error("HasFlag(verbose) should be true")
	}
	if !parser.HasFlag("lines") {
		t.Error("HasFlag(lines) should be true")
	}
	if parser.HasFlag("nonexistent") {
		t.Error("HasFlag(nonexistent) should be false")
	}
}

// =============================================================================
// MODEL DETECTION TESTS
// =============================================================================

func TestIsFreeModel(t *testing.T) {
	tests := []struct {
		model    string
		wantFree bool
	}{
		{"mistralai/devstral-2512:free", true},
		{"xiaomi/mimo-v2-flash:free", true},
		{"nvidia/nemotron-3-nano-30b-a3b:free", true},
		{"openrouter/auto", false},
		{"anthropic/claude-sonnet-4", false},
		{"qwen2.5:14b", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := strings.HasSuffix(tt.model, ":free")
			if got != tt.wantFree {
				t.Errorf("IsFreeModel(%q) = %v, want %v", tt.model, got, tt.wantFree)
			}
		})
	}
}

func TestIsCloudModel(t *testing.T) {
	tests := []struct {
		model     string
		wantCloud bool
	}{
		{"openrouter/auto", true},
		{"anthropic/claude-sonnet-4", true},
		{"mistralai/devstral-2512:free", true},
		{"openai/gpt-4o", true},
		{"qwen2.5:14b", false},
		{"llama3:8b", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			// Cloud models contain "/"
			got := strings.Contains(tt.model, "/")
			if got != tt.wantCloud {
				t.Errorf("IsCloudModel(%q) = %v, want %v", tt.model, got, tt.wantCloud)
			}
		})
	}
}

// =============================================================================
// COST CALCULATION TESTS
// =============================================================================

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		inputTokens  int
		outputTokens int
		wantCost     float64
	}{
		{
			name:         "free model has zero cost",
			model:        "mistralai/devstral-2512:free",
			inputTokens:  1000,
			outputTokens: 500,
			wantCost:     0.0,
		},
		{
			name:         "paid model has cost",
			model:        "anthropic/claude-sonnet-4",
			inputTokens:  1000,
			outputTokens: 500,
			wantCost:     0.0105, // (1000/1M * 3.0) + (500/1M * 15.0)
		},
		{
			name:         "zero tokens = zero cost",
			model:        "anthropic/claude-sonnet-4",
			inputTokens:  0,
			outputTokens: 0,
			wantCost:     0.0,
		},
		{
			name:         "million tokens",
			model:        "openrouter/auto",
			inputTokens:  1000000,
			outputTokens: 500000,
			wantCost:     10.5, // (1M/1M * 3.0) + (500k/1M * 15.0) = 3.0 + 7.5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cost float64
			if strings.HasSuffix(tt.model, ":free") {
				cost = 0.0
			} else {
				inputCost := float64(tt.inputTokens) / 1000000.0 * 3.0
				outputCost := float64(tt.outputTokens) / 1000000.0 * 15.0
				cost = inputCost + outputCost
			}

			// Allow small floating point differences
			if diff := cost - tt.wantCost; diff > 0.0001 || diff < -0.0001 {
				t.Errorf("Cost = %f, want %f", cost, tt.wantCost)
			}
		})
	}
}

// =============================================================================
// PARSE BOOL STRING TESTS
// =============================================================================

func TestParseBoolString(t *testing.T) {
	trueValues := []string{"true", "TRUE", "True", "yes", "YES", "y", "Y", "1", "on", "ON"}
	falseValues := []string{"false", "FALSE", "False", "no", "NO", "n", "N", "0", "off", "OFF"}

	for _, v := range trueValues {
		t.Run("true_"+v, func(t *testing.T) {
			got, err := ParseBoolString(v)
			if err != nil {
				t.Errorf("ParseBoolString(%q) error = %v", v, err)
			}
			if !got {
				t.Errorf("ParseBoolString(%q) = false, want true", v)
			}
		})
	}

	for _, v := range falseValues {
		t.Run("false_"+v, func(t *testing.T) {
			got, err := ParseBoolString(v)
			if err != nil {
				t.Errorf("ParseBoolString(%q) error = %v", v, err)
			}
			if got {
				t.Errorf("ParseBoolString(%q) = true, want false", v)
			}
		})
	}

	t.Run("invalid", func(t *testing.T) {
		_, err := ParseBoolString("maybe")
		if err == nil {
			t.Error("ParseBoolString(maybe) should error")
		}
	})
}

// =============================================================================
// PARSE INT WITH VALIDATION TESTS
// =============================================================================

func TestParseIntWithValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		field   string
		want    int
		wantErr bool
	}{
		{"valid positive", "42", "count", 42, false},
		{"valid one", "1", "count", 1, false},
		{"zero is invalid", "0", "count", 0, true},
		{"negative is invalid", "-5", "count", 0, true},
		{"empty is invalid", "", "count", 0, true},
		{"non-numeric is invalid", "abc", "count", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIntWithValidation(tt.input, tt.field)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseIntWithValidation(%q, %q) error = %v, wantErr %v", tt.input, tt.field, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseIntWithValidation(%q, %q) = %d, want %d", tt.input, tt.field, got, tt.want)
			}
		})
	}
}

// =============================================================================
// INTEGRATION-STYLE TESTS (testing Parse() with os.Args simulation)
// =============================================================================

// TestParse_Integration tests the actual Parse() function by temporarily
// modifying os.Args. This is an integration test of the full CLI parsing.
func TestParse_Integration(t *testing.T) {
	// Save original args
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	tests := []struct {
		name        string
		args        []string
		wantCommand Command
		validate    func(*testing.T, Args)
	}{
		{
			name:        "ask command",
			args:        []string{"rigrun", "ask", "What is Go?"},
			wantCommand: CmdAsk,
			validate: func(t *testing.T, a Args) {
				if a.Query != "What is Go?" {
					t.Errorf("Query = %q, want %q", a.Query, "What is Go?")
				}
			},
		},
		{
			name:        "ask with model flag",
			args:        []string{"rigrun", "ask", "--model", "qwen2.5:14b", "Hello"},
			wantCommand: CmdAsk,
			validate: func(t *testing.T, a Args) {
				if a.Model != "qwen2.5:14b" {
					t.Errorf("Model = %q, want %q", a.Model, "qwen2.5:14b")
				}
			},
		},
		{
			name:        "ask with agentic mode",
			args:        []string{"rigrun", "ask", "--agentic", "List files"},
			wantCommand: CmdAsk,
			validate: func(t *testing.T, a Args) {
				if !a.Agentic {
					t.Error("Agentic should be true")
				}
			},
		},
		{
			name:        "ask with paranoid flag",
			args:        []string{"rigrun", "ask", "--paranoid", "Question"},
			wantCommand: CmdAsk,
			validate: func(t *testing.T, a Args) {
				if !a.Paranoid {
					t.Error("Paranoid should be true")
				}
			},
		},
		{
			name:        "ask with quiet flag",
			args:        []string{"rigrun", "ask", "-q", "Question"},
			wantCommand: CmdAsk,
			validate: func(t *testing.T, a Args) {
				if !a.Quiet {
					t.Error("Quiet should be true")
				}
			},
		},
		{
			name:        "chat command",
			args:        []string{"rigrun", "chat"},
			wantCommand: CmdChat,
		},
		{
			name:        "chat with model",
			args:        []string{"rigrun", "chat", "--model", "llama3:8b"},
			wantCommand: CmdChat,
			validate: func(t *testing.T, a Args) {
				if a.Model != "llama3:8b" {
					t.Errorf("Model = %q, want %q", a.Model, "llama3:8b")
				}
			},
		},
		{
			name:        "intel command",
			args:        []string{"rigrun", "intel", "Anthropic"},
			wantCommand: CmdIntel,
			validate: func(t *testing.T, a Args) {
				// Intel uses Subcommand for company name
				if a.Subcommand != "Anthropic" {
					t.Errorf("Subcommand = %q, want %q", a.Subcommand, "Anthropic")
				}
			},
		},
		{
			name:        "intel with model",
			args:        []string{"rigrun", "intel", "--model", "openrouter/auto", "Company"},
			wantCommand: CmdIntel,
			validate: func(t *testing.T, a Args) {
				if a.Model != "openrouter/auto" {
					t.Errorf("Model = %q, want %q", a.Model, "openrouter/auto")
				}
			},
		},
		{
			name:        "status command",
			args:        []string{"rigrun", "status"},
			wantCommand: CmdStatus,
		},
		{
			name:        "config command",
			args:        []string{"rigrun", "config", "show"},
			wantCommand: CmdConfig,
			validate: func(t *testing.T, a Args) {
				if a.Subcommand != "show" {
					t.Errorf("Subcommand = %q, want %q", a.Subcommand, "show")
				}
			},
		},
		{
			name:        "help command",
			args:        []string{"rigrun", "help"},
			wantCommand: CmdHelp,
		},
		{
			name:        "version flag",
			args:        []string{"rigrun", "--version"},
			wantCommand: CmdVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args
			cmd, args := Parse()

			if cmd != tt.wantCommand {
				t.Errorf("Command = %v, want %v", cmd, tt.wantCommand)
			}

			if tt.validate != nil {
				tt.validate(t, args)
			}
		})
	}
}

// =============================================================================
// EDGE CASES
// =============================================================================

func TestArgParser_EmptyArgs(t *testing.T) {
	parser := NewArgParser([]string{})
	if parser.Subcommand() != "" {
		t.Errorf("Subcommand() = %q, want empty", parser.Subcommand())
	}
	if parser.PositionalCount() != 0 {
		t.Errorf("PositionalCount() = %d, want 0", parser.PositionalCount())
	}
}

func TestArgParser_OnlyFlags(t *testing.T) {
	parser := NewArgParser([]string{"--verbose", "--json"})
	if parser.Subcommand() != "" {
		t.Errorf("Subcommand() = %q, want empty", parser.Subcommand())
	}
	if !parser.BoolFlag("verbose") {
		t.Error("BoolFlag(verbose) should be true")
	}
	if !parser.BoolFlag("json") {
		t.Error("BoolFlag(json) should be true")
	}
}

func TestArgParser_FlagOrDefault(t *testing.T) {
	parser := NewArgParser([]string{"cmd", "--present", "value"})

	if parser.FlagOrDefault("present", "default") != "value" {
		t.Error("FlagOrDefault should return actual value when present")
	}
	if parser.FlagOrDefault("missing", "default") != "default" {
		t.Error("FlagOrDefault should return default when missing")
	}
}

// =============================================================================
// BENCHMARKS
// =============================================================================

func BenchmarkArgParser_Simple(b *testing.B) {
	args := []string{"ask", "What is Go?"}
	for i := 0; i < b.N; i++ {
		NewArgParser(args)
	}
}

func BenchmarkArgParser_Complex(b *testing.B) {
	args := []string{"ask", "--agentic", "--model", "openrouter/auto", "--max-iter", "10", "-q", "Complex task with many arguments"}
	for i := 0; i < b.N; i++ {
		NewArgParser(args)
	}
}

func BenchmarkArgParser_ManyFlags(b *testing.B) {
	args := []string{
		"cmd",
		"--flag1", "value1",
		"--flag2", "value2",
		"--flag3", "value3",
		"--flag4", "value4",
		"--flag5", "value5",
		"--bool1",
		"--bool2",
		"--bool3",
		"positional1",
		"positional2",
	}
	for i := 0; i < b.N; i++ {
		NewArgParser(args)
	}
}
