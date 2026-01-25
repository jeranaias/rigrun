// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package benchmark provides model benchmarking capabilities for rigrun.
package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// =============================================================================
// RESULT TYPES
// =============================================================================

// Result contains the complete benchmark results for a model.
type Result struct {
	ModelName        string         `json:"model_name"`
	StartTime        time.Time      `json:"start_time"`
	EndTime          time.Time      `json:"end_time"`
	Duration         time.Duration  `json:"duration"`
	Tests            []TestResult   `json:"tests"`
	AvgTTFT          time.Duration  `json:"avg_ttft"`
	AvgTokensPerSec  float64        `json:"avg_tokens_per_sec"`
	AvgQualityScore  float64        `json:"avg_quality_score"`
	PassedTests      int            `json:"passed_tests"`
	FailedTests      int            `json:"failed_tests"`
}

// TestResult contains the result of a single test.
type TestResult struct {
	Name          string        `json:"name"`
	Type          TestType      `json:"type"`
	Status        TestStatus    `json:"status"`
	StartTime     time.Time     `json:"start_time"`
	EndTime       time.Time     `json:"end_time"`
	Duration      time.Duration `json:"duration"`
	TTFT          time.Duration `json:"ttft"`           // Time to first token
	TokensPerSec  float64       `json:"tokens_per_sec"` // Generation speed
	TokenCount    int           `json:"token_count"`
	QualityScore  float64       `json:"quality_score"` // 0-100
	Response      string        `json:"response"`
	Error         string        `json:"error,omitempty"`
}

// TestStatus indicates the outcome of a test.
type TestStatus string

const (
	TestStatusPending TestStatus = "pending"
	TestStatusRunning TestStatus = "running"
	TestStatusPassed  TestStatus = "passed"
	TestStatusFailed  TestStatus = "failed"
)

// Comparison holds results from comparing multiple models.
type Comparison struct {
	Models    []string           `json:"models"`
	Results   map[string]*Result `json:"results"`
	StartTime time.Time          `json:"start_time"`
	EndTime   time.Time          `json:"end_time"`
	Duration  time.Duration      `json:"duration"`
}

// =============================================================================
// RESULT STORAGE
// =============================================================================

// Storage handles saving and loading benchmark results.
type Storage struct {
	dir string
}

// NewStorage creates a new storage instance.
// By default, results are stored in ~/.rigrun/benchmarks/
func NewStorage() (*Storage, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	benchmarkDir := filepath.Join(homeDir, ".rigrun", "benchmarks")
	if err := os.MkdirAll(benchmarkDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create benchmark directory: %w", err)
	}

	return &Storage{dir: benchmarkDir}, nil
}

// NewStorageWithDir creates a storage instance with a custom directory.
func NewStorageWithDir(dir string) (*Storage, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}
	return &Storage{dir: dir}, nil
}

// Save saves a benchmark result to disk.
func (s *Storage) Save(result *Result) error {
	// Generate filename with timestamp and model name
	timestamp := time.Now().Format("20060102-150405.000")
	filename := fmt.Sprintf("%s_%s.json", sanitizeFilename(result.ModelName), timestamp)
	path := filepath.Join(s.dir, filename)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write result: %w", err)
	}

	return nil
}

// SaveComparison saves a model comparison to disk.
func (s *Storage) SaveComparison(comparison *Comparison) error {
	timestamp := time.Now().Format("20060102-150405.000")
	filename := fmt.Sprintf("comparison_%s.json", timestamp)
	path := filepath.Join(s.dir, filename)

	data, err := json.MarshalIndent(comparison, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal comparison: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write comparison: %w", err)
	}

	return nil
}

// Load loads a benchmark result from disk.
func (s *Storage) Load(filename string) (*Result, error) {
	path := filepath.Join(s.dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read result: %w", err)
	}

	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return &result, nil
}

// LoadComparison loads a comparison from disk.
func (s *Storage) LoadComparison(filename string) (*Comparison, error) {
	path := filepath.Join(s.dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read comparison: %w", err)
	}

	var comparison Comparison
	if err := json.Unmarshal(data, &comparison); err != nil {
		return nil, fmt.Errorf("failed to unmarshal comparison: %w", err)
	}

	return &comparison, nil
}

// List returns all benchmark result files.
func (s *Storage) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	files := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			files = append(files, entry.Name())
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		infoI, errI := os.Stat(filepath.Join(s.dir, files[i]))
		infoJ, errJ := os.Stat(filepath.Join(s.dir, files[j]))
		if errI != nil || errJ != nil {
			return false // Keep original order if error
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})

	return files, nil
}

// GetLatestForModel returns the most recent result for a specific model.
func (s *Storage) GetLatestForModel(modelName string) (*Result, error) {
	files, err := s.List()
	if err != nil {
		return nil, err
	}

	// Find the first file that matches the model name
	sanitized := sanitizeFilename(modelName)
	for _, file := range files {
		if strings.HasPrefix(file, sanitized+"_") {
			return s.Load(file)
		}
	}

	return nil, fmt.Errorf("no results found for model: %s", modelName)
}

// sanitizeFilename removes characters that aren't safe for filenames.
func sanitizeFilename(name string) string {
	// Replace common separators with underscores
	safe := name
	replacements := map[rune]rune{
		':':  '_',
		'/':  '_',
		'\\': '_',
		' ':  '_',
		'*':  '_',
		'?':  '_',
		'<':  '_',
		'>':  '_',
		'|':  '_',
		'"':  '_',
	}

	result := make([]rune, 0, len(safe))
	for _, r := range safe {
		if replacement, ok := replacements[r]; ok {
			result = append(result, replacement)
		} else {
			result = append(result, r)
		}
	}

	return string(result)
}

// =============================================================================
// RESULT ANALYSIS
// =============================================================================

// GetBestModel returns the model with the best overall performance from a comparison.
func (c *Comparison) GetBestModel() (string, *Result) {
	var bestModel string
	var bestResult *Result
	var bestScore float64

	for model, result := range c.Results {
		// Calculate composite score (weighted average)
		// Speed: 40%, Quality: 40%, Latency: 20%
		score := 0.0

		if result.AvgTokensPerSec > 0 {
			score += result.AvgTokensPerSec * 0.4
		}

		if result.AvgQualityScore > 0 {
			score += result.AvgQualityScore * 0.4
		}

		if result.AvgTTFT > 0 {
			// Lower is better for latency, so invert it
			latencyScore := 1000.0 / float64(result.AvgTTFT.Milliseconds())
			score += latencyScore * 0.2
		}

		if score > bestScore {
			bestScore = score
			bestModel = model
			bestResult = result
		}
	}

	return bestModel, bestResult
}

// GetFastestModel returns the model with the highest tokens/sec.
func (c *Comparison) GetFastestModel() (string, *Result) {
	var fastest string
	var fastestResult *Result
	var highestSpeed float64

	for model, result := range c.Results {
		if result.AvgTokensPerSec > highestSpeed {
			highestSpeed = result.AvgTokensPerSec
			fastest = model
			fastestResult = result
		}
	}

	return fastest, fastestResult
}

// GetLowestLatencyModel returns the model with the lowest TTFT.
func (c *Comparison) GetLowestLatencyModel() (string, *Result) {
	var lowest string
	var lowestResult *Result
	var lowestTTFT time.Duration = time.Hour * 24 // Start with a very high value

	for model, result := range c.Results {
		if result.AvgTTFT > 0 && result.AvgTTFT < lowestTTFT {
			lowestTTFT = result.AvgTTFT
			lowest = model
			lowestResult = result
		}
	}

	return lowest, lowestResult
}

// GetHighestQualityModel returns the model with the best quality score.
func (c *Comparison) GetHighestQualityModel() (string, *Result) {
	var highest string
	var highestResult *Result
	var highestQuality float64

	for model, result := range c.Results {
		if result.AvgQualityScore > highestQuality {
			highestQuality = result.AvgQualityScore
			highest = model
			highestResult = result
		}
	}

	return highest, highestResult
}

// =============================================================================
// SUMMARY GENERATION
// =============================================================================

// Summary returns a text summary of the benchmark result.
func (r *Result) Summary() string {
	return fmt.Sprintf(
		"Model: %s\n"+
			"Duration: %s\n"+
			"Tests: %d passed, %d failed\n"+
			"Avg TTFT: %s\n"+
			"Avg Speed: %s\n"+
			"Avg Quality: %s",
		r.ModelName,
		FormatDuration(r.Duration),
		r.PassedTests,
		r.FailedTests,
		FormatTTFT(r.AvgTTFT),
		FormatTokensPerSec(r.AvgTokensPerSec),
		FormatQualityScore(r.AvgQualityScore),
	)
}

// ComparisonSummary returns a text summary of the model comparison.
func (c *Comparison) ComparisonSummary() string {
	best, bestResult := c.GetBestModel()
	fastest, fastestResult := c.GetFastestModel()
	lowestLatency, lowestLatencyResult := c.GetLowestLatencyModel()
	highestQuality, highestQualityResult := c.GetHighestQualityModel()

	summary := fmt.Sprintf("Benchmark Comparison Summary\n")
	summary += fmt.Sprintf("Models tested: %d\n", len(c.Models))
	summary += fmt.Sprintf("Total duration: %s\n\n", FormatDuration(c.Duration))

	if bestResult != nil {
		summary += fmt.Sprintf("Best Overall: %s (Speed: %s, Quality: %s)\n",
			best,
			FormatTokensPerSec(bestResult.AvgTokensPerSec),
			FormatQualityScore(bestResult.AvgQualityScore))
	}

	if fastestResult != nil {
		summary += fmt.Sprintf("Fastest: %s (%s)\n",
			fastest,
			FormatTokensPerSec(fastestResult.AvgTokensPerSec))
	}

	if lowestLatencyResult != nil {
		summary += fmt.Sprintf("Lowest Latency: %s (%s)\n",
			lowestLatency,
			FormatTTFT(lowestLatencyResult.AvgTTFT))
	}

	if highestQualityResult != nil {
		summary += fmt.Sprintf("Highest Quality: %s (%s)\n",
			highestQuality,
			FormatQualityScore(highestQualityResult.AvgQualityScore))
	}

	return summary
}
