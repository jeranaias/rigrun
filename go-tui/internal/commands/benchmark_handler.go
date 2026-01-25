// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package commands provides the slash command system for the TUI.
package commands

import (
	"context"
	"fmt"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeranaias/rigrun-tui/internal/benchmark"
)

// =============================================================================
// BENCHMARK MESSAGE TYPES
// =============================================================================

// BenchmarkStartMsg indicates a benchmark is starting.
type BenchmarkStartMsg struct {
	Models []string
}

// BenchmarkProgressMsg reports benchmark progress.
type BenchmarkProgressMsg struct {
	ModelName   string
	CurrentTest int
	TotalTests  int
	TestName    string
}

// BenchmarkCompleteMsg indicates benchmark completion.
type BenchmarkCompleteMsg struct {
	Result     *benchmark.Result
	Comparison *benchmark.Comparison
	Error      error
}

// =============================================================================
// BENCHMARK HANDLER
// =============================================================================

// HandleBenchmark runs benchmark tests on one or more models.
func HandleBenchmark(ctx *Context, args []string) tea.Cmd {
	if len(args) == 0 {
		return func() tea.Msg {
			return ErrorMsg{
				Title:   "Missing argument",
				Message: "/benchmark requires at least one model name",
				Tip:     "Usage: /benchmark <model> [model2 model3...]",
			}
		}
	}

	models := args

	// Return a message that triggers the benchmark
	return func() tea.Msg {
		return BenchmarkStartMsg{Models: models}
	}
}

// RunBenchmark executes the actual benchmark asynchronously.
// This is called from the main application after BenchmarkStartMsg is received.
func RunBenchmark(ctx *Context, models []string) tea.Cmd {
	return func() tea.Msg {
		// Check if Ollama client is available
		if ctx == nil || ctx.Ollama == nil {
			return BenchmarkCompleteMsg{
				Error: fmt.Errorf("Ollama client not available"),
			}
		}

		// Create benchmark runner
		runner := benchmark.NewRunner(ctx.Ollama)

		// Create context with timeout
		benchCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		// Run benchmark(s)
		if len(models) == 1 {
			// Single model benchmark
			result, err := runner.Run(benchCtx, models[0])
			if err != nil {
				return BenchmarkCompleteMsg{Error: err}
			}

			// Save result
			storage, err := benchmark.NewStorage()
			if err == nil {
				if saveErr := storage.Save(result); saveErr != nil {
					log.Printf("Failed to save benchmark result: %v", saveErr)
				}
			}

			return BenchmarkCompleteMsg{Result: result}
		}

		// Multiple model comparison
		comparison, err := runner.RunComparison(benchCtx, models)
		if err != nil {
			return BenchmarkCompleteMsg{Error: err}
		}

		// Save comparison
		storage, err := benchmark.NewStorage()
		if err == nil {
			if saveErr := storage.SaveComparison(comparison); saveErr != nil {
				log.Printf("Failed to save benchmark comparison: %v", saveErr)
			}
		}

		return BenchmarkCompleteMsg{Comparison: comparison}
	}
}
