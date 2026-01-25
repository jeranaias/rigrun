// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package benchmark provides model benchmarking capabilities for rigrun.
package benchmark

import (
	"context"
	"fmt"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/ollama"
)

// =============================================================================
// BENCHMARK RUNNER
// =============================================================================

// Runner executes benchmarks on models.
// Note: Runner is not thread-safe and should not be used concurrently
// from multiple goroutines.
type Runner struct {
	client *ollama.Client
}

// NewRunner creates a new benchmark runner.
func NewRunner(client *ollama.Client) *Runner {
	return &Runner{client: client}
}

// Run executes the full benchmark suite on a model.
func (r *Runner) Run(ctx context.Context, modelName string) (*Result, error) {
	result := &Result{
		ModelName: modelName,
		StartTime: time.Now(),
		Tests:     make([]TestResult, 0),
	}

	// Get all standard tests
	tests := GetStandardTests()

	for _, test := range tests {
		testResult, err := r.runTest(ctx, modelName, test)
		if err != nil {
			// Record failed test
			testResult = TestResult{
				Name:   test.Name,
				Error:  err.Error(),
				Status: TestStatusFailed,
			}
		}
		result.Tests = append(result.Tests, testResult)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Compute aggregate metrics
	result.computeAggregates()

	return result, nil
}

// runTest executes a single benchmark test.
func (r *Runner) runTest(ctx context.Context, modelName string, test Test) (TestResult, error) {
	// Check for context cancellation before starting
	select {
	case <-ctx.Done():
		return TestResult{Name: test.Name, Status: TestStatusFailed, Error: "Context cancelled"}, ctx.Err()
	default:
	}

	// Validate test input
	if test.Prompt == "" {
		return TestResult{Name: test.Name, Status: TestStatusFailed, Error: "Empty prompt"}, fmt.Errorf("test prompt is empty")
	}

	testResult := TestResult{
		Name:      test.Name,
		Type:      test.Type,
		Status:    TestStatusRunning,
		StartTime: time.Now(),
	}

	// Prepare messages
	messages := []ollama.Message{
		{Role: "user", Content: test.Prompt},
	}

	// Set up streaming callback to measure TTFT
	var firstTokenTime time.Time
	var totalTokens int
	var responseContent string
	var streamErr error
	streamStart := time.Now()

	callback := func(chunk ollama.StreamChunk) {
		if chunk.Error != nil {
			streamErr = chunk.Error
			return
		}

		// Record first token time (measured from first non-empty content chunk)
		if firstTokenTime.IsZero() && chunk.Content != "" {
			firstTokenTime = time.Now()
		}

		responseContent += chunk.Content

		if chunk.Done {
			totalTokens = chunk.CompletionTokens
		}
	}

	// Execute the test with streaming
	err := r.client.ChatStream(ctx, modelName, messages, callback)
	if err == nil && streamErr != nil {
		err = streamErr
	}
	streamEnd := time.Now()

	if err != nil {
		testResult.Status = TestStatusFailed
		testResult.Error = err.Error()
		return testResult, err
	}

	// Calculate metrics
	testResult.EndTime = streamEnd
	testResult.Duration = streamEnd.Sub(streamStart)

	if !firstTokenTime.IsZero() {
		testResult.TTFT = firstTokenTime.Sub(streamStart)
	}

	testResult.TokenCount = totalTokens

	if totalTokens > 0 && testResult.Duration > 0 {
		testResult.TokensPerSec = float64(totalTokens) / testResult.Duration.Seconds()
	}
	// Note: If token count is unavailable (totalTokens == 0), TokensPerSec will be 0

	// Evaluate quality if evaluator is provided
	if test.Evaluator != nil {
		testResult.QualityScore = test.Evaluator(responseContent)
	}

	testResult.Response = responseContent
	testResult.Status = TestStatusPassed

	return testResult, nil
}

// RunComparison runs benchmarks on multiple models and compares them.
// Returns a comparison even if individual models fail. Returns an error only
// if all models fail to run.
func (r *Runner) RunComparison(ctx context.Context, modelNames []string) (*Comparison, error) {
	comparison := &Comparison{
		Models:    make([]string, len(modelNames)),
		Results:   make(map[string]*Result),
		StartTime: time.Now(),
	}

	copy(comparison.Models, modelNames)

	successCount := 0
	for _, modelName := range modelNames {
		result, err := r.Run(ctx, modelName)
		if err != nil {
			// Still record the result even if there were errors
			comparison.Results[modelName] = result
		} else {
			comparison.Results[modelName] = result
			successCount++
		}
	}

	comparison.EndTime = time.Now()
	comparison.Duration = comparison.EndTime.Sub(comparison.StartTime)

	// Return error if all models failed
	if successCount == 0 {
		return comparison, fmt.Errorf("all models failed to run")
	}

	return comparison, nil
}

// =============================================================================
// RESULT COMPUTATION
// =============================================================================

// computeAggregates calculates aggregate metrics from individual tests.
func (r *Result) computeAggregates() {
	var totalTTFT time.Duration
	var totalTokensPerSec float64
	var totalQuality float64
	var ttftCount, tpsCount, qualityCount int

	for _, test := range r.Tests {
		if test.Status != TestStatusPassed {
			continue
		}

		if test.TTFT > 0 {
			totalTTFT += test.TTFT
			ttftCount++
		}

		if test.TokensPerSec > 0 {
			totalTokensPerSec += test.TokensPerSec
			tpsCount++
		}

		if test.QualityScore >= 0 {
			totalQuality += test.QualityScore
			qualityCount++
		}
	}

	if ttftCount > 0 {
		r.AvgTTFT = totalTTFT / time.Duration(ttftCount)
	}

	if tpsCount > 0 {
		r.AvgTokensPerSec = totalTokensPerSec / float64(tpsCount)
	}

	if qualityCount > 0 {
		r.AvgQualityScore = totalQuality / float64(qualityCount)
	}

	// Count passed/failed tests
	for _, test := range r.Tests {
		if test.Status == TestStatusPassed {
			r.PassedTests++
		} else if test.Status == TestStatusFailed {
			r.FailedTests++
		}
	}
}

// =============================================================================
// FORMATTING HELPERS
// =============================================================================

// FormatTTFT formats time to first token for display.
func FormatTTFT(d time.Duration) string {
	if d == 0 {
		return "N/A"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// FormatTokensPerSec formats tokens per second for display.
func FormatTokensPerSec(tps float64) string {
	if tps == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%.1f t/s", tps)
}

// FormatQualityScore formats quality score for display.
func FormatQualityScore(score float64) string {
	if score == 0 {
		return "N/A"
	}
	return fmt.Sprintf("%.1f%%", score)
}

// FormatDuration formats duration for display.
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "N/A"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}
