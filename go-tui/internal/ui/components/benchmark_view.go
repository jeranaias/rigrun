// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/benchmark"
)

// =============================================================================
// BENCHMARK VIEW
// =============================================================================

// BenchmarkView renders benchmark results in the TUI.
type BenchmarkView struct {
	width  int
	height int
}

// NewBenchmarkView creates a new benchmark view.
func NewBenchmarkView(width, height int) *BenchmarkView {
	return &BenchmarkView{
		width:  width,
		height: height,
	}
}

// SetSize updates the view dimensions.
func (v *BenchmarkView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// RenderResult renders a single benchmark result.
func (v *BenchmarkView) RenderResult(result *benchmark.Result) string {
	if result == nil {
		return "No benchmark result available"
	}

	var b strings.Builder

	// Header
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#A78BFA")). // Purple
		PaddingBottom(1)

	b.WriteString(titleStyle.Render(fmt.Sprintf("Benchmark Results: %s", result.ModelName)))
	b.WriteString("\n\n")

	// Summary metrics
	metricsStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#45475A")). // Overlay
		Padding(1, 2)

	metrics := v.formatMetrics(result)
	b.WriteString(metricsStyle.Render(metrics))
	b.WriteString("\n\n")

	// Test results
	b.WriteString(v.formatTestResults(result))

	return b.String()
}

// RenderComparison renders a comparison of multiple models.
func (v *BenchmarkView) RenderComparison(comparison *benchmark.Comparison) string {
	var b strings.Builder

	// Header
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#A78BFA")).
		PaddingBottom(1)

	b.WriteString(titleStyle.Render("Model Comparison"))
	b.WriteString("\n\n")

	// Summary
	summaryStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#45475A")).
		Padding(1, 2)

	b.WriteString(summaryStyle.Render(comparison.ComparisonSummary()))
	b.WriteString("\n\n")

	// Comparison table
	b.WriteString(v.formatComparisonTable(comparison))

	return b.String()
}

// formatMetrics formats the summary metrics for a result.
func (v *BenchmarkView) formatMetrics(result *benchmark.Result) string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Width(20)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#34D399"))

	// Duration
	b.WriteString(labelStyle.Render("Duration:"))
	b.WriteString(valueStyle.Render(benchmark.FormatDuration(result.Duration)))
	b.WriteString("\n")

	// Tests
	b.WriteString(labelStyle.Render("Tests:"))
	testSummary := fmt.Sprintf("%d passed, %d failed", result.PassedTests, result.FailedTests)
	if result.FailedTests > 0 {
		testSummary = lipgloss.NewStyle().Foreground(lipgloss.Color("#FB7185")).Render(testSummary)
	} else {
		testSummary = valueStyle.Render(testSummary)
	}
	b.WriteString(testSummary)
	b.WriteString("\n\n")

	// Performance metrics
	b.WriteString(labelStyle.Render("Avg TTFT:"))
	b.WriteString(valueStyle.Render(benchmark.FormatTTFT(result.AvgTTFT)))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Avg Speed:"))
	b.WriteString(valueStyle.Render(benchmark.FormatTokensPerSec(result.AvgTokensPerSec)))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Avg Quality:"))
	b.WriteString(valueStyle.Render(benchmark.FormatQualityScore(result.AvgQualityScore)))
	b.WriteString("\n")

	return b.String()
}

// formatTestResults formats individual test results.
func (v *BenchmarkView) formatTestResults(result *benchmark.Result) string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#A78BFA"))

	b.WriteString(headerStyle.Render("Test Results"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("-", v.width-4))
	b.WriteString("\n\n")

	for _, test := range result.Tests {
		b.WriteString(v.formatTestResult(test))
		b.WriteString("\n")
	}

	return b.String()
}

// formatTestResult formats a single test result.
func (v *BenchmarkView) formatTestResult(test benchmark.TestResult) string {
	var b strings.Builder

	// Status indicator (ASCII)
	var statusIcon string
	var statusColor lipgloss.Color
	switch test.Status {
	case benchmark.TestStatusPassed:
		statusIcon = "[OK]"
		statusColor = lipgloss.Color("#34D399")
	case benchmark.TestStatusFailed:
		statusIcon = "[X]"
		statusColor = lipgloss.Color("#FB7185")
	case benchmark.TestStatusRunning:
		statusIcon = "[.]"
		statusColor = lipgloss.Color("#FBBF24")
	default:
		statusIcon = "[ ]"
		statusColor = lipgloss.Color("#6C7086")
	}

	statusStyle := lipgloss.NewStyle().Foreground(statusColor)
	nameStyle := lipgloss.NewStyle().Bold(true)

	b.WriteString(statusStyle.Render(statusIcon))
	b.WriteString(" ")
	b.WriteString(nameStyle.Render(test.Name))
	b.WriteString("\n")

	if test.Status == benchmark.TestStatusFailed {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FB7185")).
			Italic(true).
			PaddingLeft(2)
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", test.Error)))
		b.WriteString("\n")
		return b.String()
	}

	// Metrics
	metricStyle := lipgloss.NewStyle().PaddingLeft(2)
	b.WriteString(metricStyle.Render(fmt.Sprintf(
		"TTFT: %s | Speed: %s | Quality: %s | Duration: %s",
		benchmark.FormatTTFT(test.TTFT),
		benchmark.FormatTokensPerSec(test.TokensPerSec),
		benchmark.FormatQualityScore(test.QualityScore),
		benchmark.FormatDuration(test.Duration),
	)))
	b.WriteString("\n")

	return b.String()
}

// formatComparisonTable creates a comparison table for multiple models.
func (v *BenchmarkView) formatComparisonTable(comparison *benchmark.Comparison) string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#A78BFA"))

	b.WriteString(headerStyle.Render("Detailed Comparison"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("=", v.width-4))
	b.WriteString("\n\n")

	// Table header
	header := fmt.Sprintf("%-25s | %-12s | %-12s | %-12s | %-8s",
		"Model", "Avg TTFT", "Avg Speed", "Avg Quality", "Tests")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("-", v.width-4))
	b.WriteString("\n")

	// Table rows
	for _, modelName := range comparison.Models {
		result, ok := comparison.Results[modelName]
		if !ok {
			continue
		}

		row := fmt.Sprintf("%-25s | %-12s | %-12s | %-12s | %d/%d",
			truncate(modelName, 25),
			benchmark.FormatTTFT(result.AvgTTFT),
			benchmark.FormatTokensPerSec(result.AvgTokensPerSec),
			benchmark.FormatQualityScore(result.AvgQualityScore),
			result.PassedTests,
			result.PassedTests+result.FailedTests,
		)

		// Highlight best values
		if result.FailedTests == 0 {
			row = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Render(row)
		}

		b.WriteString(row)
		b.WriteString("\n")
	}

	return b.String()
}

// RenderProgress renders benchmark progress during execution.
func (v *BenchmarkView) RenderProgress(modelName string, currentTest int, totalTests int, testName string) string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#A78BFA"))

	b.WriteString(titleStyle.Render(fmt.Sprintf("Benchmarking: %s", modelName)))
	b.WriteString("\n\n")

	// Progress bar
	progressStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#45475A")).
		Padding(1, 2)

	progress := fmt.Sprintf("Test %d of %d: %s", currentTest, totalTests, testName)
	b.WriteString(progressStyle.Render(progress))
	b.WriteString("\n\n")

	// Progress percentage
	var percentage float64
	if totalTests == 0 {
		percentage = 0
	} else {
		percentage = float64(currentTest) / float64(totalTests) * 100
	}
	barWidth := 40
	filled := int(float64(barWidth) * percentage / 100)

	bar := strings.Repeat("#", filled) + strings.Repeat("-", barWidth-filled)
	b.WriteString(fmt.Sprintf("[%s] %.0f%%\n", bar, percentage))

	return b.String()
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// truncate truncates a string to a maximum length.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// RenderSystemMessage formats a benchmark system message.
func RenderBenchmarkMessage(message string) string {
	msgStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#45475A")).
		Padding(1, 2).
		Foreground(lipgloss.Color("#6C7086"))

	return msgStyle.Render(message)
}

// RenderError formats a benchmark error message.
func RenderBenchmarkError(err error) string {
	errorStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FB7185")).
		Padding(1, 2).
		Foreground(lipgloss.Color("#FB7185")).
		Bold(true)

	return errorStyle.Render(fmt.Sprintf("Benchmark Error: %s", err.Error()))
}
