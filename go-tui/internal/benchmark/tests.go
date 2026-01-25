// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package benchmark provides model benchmarking capabilities for rigrun.
package benchmark

import (
	"strings"
)

// =============================================================================
// TEST DEFINITIONS
// =============================================================================

// Test represents a single benchmark test.
type Test struct {
	Name        string
	Type        TestType
	Prompt      string
	Evaluator   QualityEvaluator
	Description string
}

// TestType categorizes the type of test.
type TestType string

const (
	TestTypeSpeed       TestType = "speed"
	TestTypeLatency     TestType = "latency"
	TestTypeCompletion  TestType = "completion"
	TestTypeExplanation TestType = "explanation"
)

// QualityEvaluator is a function that scores the quality of a response (0-100).
type QualityEvaluator func(response string) float64

// =============================================================================
// STANDARD TEST SUITE
// =============================================================================

// GetStandardTests returns the standard benchmark test suite.
func GetStandardTests() []Test {
	return []Test{
		// Latency test - measures TTFT with simple prompt
		{
			Name:        "Latency Test",
			Type:        TestTypeLatency,
			Prompt:      "Say 'Hello'",
			Description: "Measures time to first token with minimal prompt",
			Evaluator: func(response string) float64 {
				// Simple check - should contain "hello" (case insensitive)
				lower := strings.ToLower(response)
				if strings.Contains(lower, "hello") {
					return 100.0
				}
				return 50.0
			},
		},

		// Speed test - measures tokens/sec with longer generation
		{
			Name:        "Speed Test",
			Type:        TestTypeSpeed,
			Prompt:      "Write a haiku about programming.",
			Description: "Measures token generation speed with creative task",
			Evaluator: func(response string) float64 {
				// Check if response looks like a haiku (has line breaks)
				lines := strings.Split(strings.TrimSpace(response), "\n")
				if len(lines) >= 3 {
					return 100.0
				}
				// Partial credit if it generated something
				if len(response) > 10 {
					return 70.0
				}
				return 30.0
			},
		},

		// Code completion test - measures accuracy on code tasks
		{
			Name:        "Code Completion Test",
			Type:        TestTypeCompletion,
			Prompt:      "Complete this function:\n\ndef fibonacci(n):\n    # Calculate the nth Fibonacci number",
			Description: "Measures accuracy on code completion",
			Evaluator: func(response string) float64 {
				lower := strings.ToLower(response)
				score := 0.0

				// Check for key elements of a fibonacci implementation
				if strings.Contains(lower, "return") {
					score += 20
				}
				if strings.Contains(lower, "if") || strings.Contains(lower, "while") || strings.Contains(lower, "for") {
					score += 20
				}
				// Check for fibonacci-related terms
				if strings.Contains(lower, "fib") || strings.Contains(lower, "n-1") || strings.Contains(lower, "n-2") {
					score += 30
				}
				// Check for recursion or iteration patterns
				if strings.Contains(lower, "fibonacci(") || (strings.Contains(lower, "for") && strings.Contains(lower, "range")) {
					score += 30
				}

				return score
			},
		},

		// Explanation test - measures coherence in explanations
		{
			Name:        "Explanation Test",
			Type:        TestTypeExplanation,
			Prompt:      "Explain what a REST API is in simple terms.",
			Description: "Measures coherence and clarity in explanations",
			Evaluator: func(response string) float64 {
				lower := strings.ToLower(response)
				score := 0.0

				// Check for key REST API concepts
				keywords := []string{"api", "http", "request", "response", "rest"}
				for _, kw := range keywords {
					if strings.Contains(lower, kw) {
						score += 15
					}
				}

				// Check for structure (paragraphs or sentences)
				sentences := strings.Count(response, ".")
				if sentences >= 3 {
					score += 15
				}

				// Check for coherence indicators
				if strings.Contains(lower, "essentially") || strings.Contains(lower, "basically") ||
					strings.Contains(lower, "in other words") || strings.Contains(lower, "for example") {
					score += 10
				}

				// Ensure score doesn't exceed 100
				if score > 100 {
					score = 100
				}

				return score
			},
		},

		// Instruction following test
		{
			Name:        "Instruction Following Test",
			Type:        TestTypeCompletion,
			Prompt:      "List exactly 3 programming languages. Format: 1. Language",
			Description: "Measures ability to follow specific instructions",
			Evaluator: func(response string) float64 {
				score := 0.0

				// Check for numbered list format
				if strings.Contains(response, "1.") {
					score += 25
				}
				if strings.Contains(response, "2.") {
					score += 25
				}
				if strings.Contains(response, "3.") {
					score += 25
				}

				// Check that it doesn't list more than 3
				if !strings.Contains(response, "4.") {
					score += 25
				}

				return score
			},
		},
	}
}

// =============================================================================
// CUSTOM TEST BUILDERS
// =============================================================================

// NewSpeedTest creates a custom speed test with a given prompt.
func NewSpeedTest(name, prompt string) Test {
	return Test{
		Name:        name,
		Type:        TestTypeSpeed,
		Prompt:      prompt,
		Description: "Custom speed test",
		Evaluator: func(response string) float64 {
			// Simple evaluator - just check if something was generated
			if len(response) > 20 {
				return 100.0
			}
			return float64(len(response)) * 5.0 // Scale to 100
		},
	}
}

// NewLatencyTest creates a custom latency test with a given prompt.
func NewLatencyTest(name, prompt string) Test {
	return Test{
		Name:        name,
		Type:        TestTypeLatency,
		Prompt:      prompt,
		Description: "Custom latency test",
		Evaluator: func(response string) float64 {
			// Any response is valid for latency tests
			if len(response) > 0 {
				return 100.0
			}
			return 0.0
		},
	}
}

// NewCodeTest creates a custom code completion test.
func NewCodeTest(name, prompt string, expectedKeywords []string) Test {
	return Test{
		Name:        name,
		Type:        TestTypeCompletion,
		Prompt:      prompt,
		Description: "Custom code completion test",
		Evaluator: func(response string) float64 {
			if len(expectedKeywords) == 0 {
				return 100.0
			}

			lower := strings.ToLower(response)
			found := 0
			for _, kw := range expectedKeywords {
				if strings.Contains(lower, strings.ToLower(kw)) {
					found++
				}
			}

			return float64(found) / float64(len(expectedKeywords)) * 100.0
		},
	}
}

// =============================================================================
// TEST SUITE HELPERS
// =============================================================================

// FilterTestsByType returns only tests of a specific type.
func FilterTestsByType(tests []Test, testType TestType) []Test {
	filtered := make([]Test, 0)
	for _, test := range tests {
		if test.Type == testType {
			filtered = append(filtered, test)
		}
	}
	return filtered
}

// GetQuickTestSuite returns a minimal test suite for quick benchmarking.
func GetQuickTestSuite() []Test {
	allTests := GetStandardTests()
	// Return just the first test of each type
	quick := make([]Test, 0)
	seen := make(map[TestType]bool)

	for _, test := range allTests {
		if !seen[test.Type] {
			quick = append(quick, test)
			seen[test.Type] = true
		}
	}

	return quick
}
