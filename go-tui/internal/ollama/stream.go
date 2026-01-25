// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ollama provides the HTTP client for communicating with Ollama API.
package ollama

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"
)

// =============================================================================
// STREAM READER
// =============================================================================

// StreamReader handles line-by-line JSON parsing of streaming responses.
type StreamReader struct {
	reader      *bufio.Reader
	// PERFORMANCE: strings.Builder avoids quadratic allocations
	accumulator strings.Builder
	tokenCount  int
	model       string
	firstToken  bool
	startTime   time.Time
	decoder     *json.Decoder // Reusable JSON decoder for optimization
}

// NewStreamReader creates a new stream reader from an io.Reader.
func NewStreamReader(r io.Reader) *StreamReader {
	return &StreamReader{
		reader:     bufio.NewReader(r),
		firstToken: true,
		startTime:  time.Now(),
	}
}

// Process reads the stream and calls the callback for each chunk.
// Blocks until the stream is complete or the context is cancelled.
func (s *StreamReader) Process(ctx context.Context, callback StreamCallback) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			chunk, err := s.readChunk()
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}

			if chunk != nil {
				callback(*chunk)
				if chunk.Done {
					return nil
				}
			}
		}
	}
}

// readChunk reads and parses a single line from the stream.
func (s *StreamReader) readChunk() (*StreamChunk, error) {
	line, err := s.reader.ReadBytes('\n')
	if err != nil {
		if err == io.EOF && len(line) == 0 {
			return nil, io.EOF
		}
		// Try to process the last line even on EOF
		if len(line) == 0 {
			return nil, err
		}
	}

	// Skip empty lines
	if len(line) == 0 {
		return nil, nil
	}

	// Parse the JSON response - use a generic struct to capture tool_calls
	var response struct {
		Model              string    `json:"model"`
		CreatedAt          time.Time `json:"created_at"`
		Message            struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		Done               bool      `json:"done"`
		DoneReason         string    `json:"done_reason,omitempty"`
		TotalDuration      int64     `json:"total_duration,omitempty"`
		LoadDuration       int64     `json:"load_duration,omitempty"`
		PromptEvalCount    int       `json:"prompt_eval_count,omitempty"`
		PromptEvalDuration int64     `json:"prompt_eval_duration,omitempty"`
		EvalCount          int       `json:"eval_count,omitempty"`
		EvalDuration       int64     `json:"eval_duration,omitempty"`
	}
	if err := json.Unmarshal(line, &response); err != nil {
		// Skip malformed lines
		return nil, nil
	}

	// Track the model
	if response.Model != "" {
		s.model = response.Model
	}

	// Extract content
	content := response.Message.Content
	if content != "" {
		s.accumulator.WriteString(content)
		s.tokenCount++
	}

	// Build the chunk
	chunk := &StreamChunk{
		Content:     content,
		ToolCalls:   response.Message.ToolCalls,
		Done:        response.Done,
		Model:       s.model,
		DoneReason:  response.DoneReason,
	}

	// Record first token timing
	if s.firstToken && content != "" {
		s.firstToken = false
	}

	// On completion, extract statistics
	if response.Done {
		chunk.TotalDuration = time.Duration(response.TotalDuration)
		chunk.LoadDuration = time.Duration(response.LoadDuration)
		chunk.PromptEvalDuration = time.Duration(response.PromptEvalDuration)
		chunk.EvalDuration = time.Duration(response.EvalDuration)
		chunk.PromptTokens = response.PromptEvalCount
		chunk.CompletionTokens = response.EvalCount
	}

	return chunk, nil
}

// GetAccumulated returns all accumulated content.
func (s *StreamReader) GetAccumulated() string {
	return s.accumulator.String()
}

// GetTokenCount returns the number of tokens received.
func (s *StreamReader) GetTokenCount() int {
	return s.tokenCount
}

// GetModel returns the model name from the stream.
func (s *StreamReader) GetModel() string {
	return s.model
}

// =============================================================================
// STREAM STATISTICS
// =============================================================================

// StreamStats holds statistics collected during streaming.
type StreamStats struct {
	// Timing
	StartTime          time.Time
	FirstTokenTime     time.Time
	EndTime            time.Time

	// Durations (from Ollama response)
	TotalDuration      time.Duration
	LoadDuration       time.Duration
	PromptEvalDuration time.Duration
	EvalDuration       time.Duration

	// Token counts
	PromptTokens       int
	CompletionTokens   int

	// Computed
	TTFT              time.Duration // Time to first token
	TokensPerSecond   float64
}

// NewStreamStats creates a new StreamStats with start time set.
func NewStreamStats() *StreamStats {
	return &StreamStats{
		StartTime: time.Now(),
	}
}

// RecordFirstToken marks the time of first token arrival.
func (s *StreamStats) RecordFirstToken() {
	if s.FirstTokenTime.IsZero() {
		s.FirstTokenTime = time.Now()
		s.TTFT = s.FirstTokenTime.Sub(s.StartTime)
	}
}

// Finalize computes final statistics from the last chunk.
func (s *StreamStats) Finalize(chunk StreamChunk) {
	s.EndTime = time.Now()
	s.TotalDuration = chunk.TotalDuration
	s.LoadDuration = chunk.LoadDuration
	s.PromptEvalDuration = chunk.PromptEvalDuration
	s.EvalDuration = chunk.EvalDuration
	s.PromptTokens = chunk.PromptTokens
	s.CompletionTokens = chunk.CompletionTokens

	// Calculate tokens per second
	if s.EvalDuration > 0 {
		seconds := s.EvalDuration.Seconds()
		s.TokensPerSecond = float64(s.CompletionTokens) / seconds
	}
}

// Format returns a formatted string representation.
func (s *StreamStats) Format() string {
	totalSec := s.TotalDuration.Seconds()
	ttftMs := s.TTFT.Milliseconds()

	return formatStatsDuration(totalSec) + " | " +
		formatStatsInt(s.CompletionTokens) + " tokens | " +
		formatStatsFloat(s.TokensPerSecond) + " tok/s | " +
		"TTFT " + formatStatsInt(int(ttftMs)) + "ms"
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// formatStatsInt formats an integer without using fmt.
func formatStatsInt(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

// formatStatsFloat formats a float with one decimal place.
func formatStatsFloat(f float64) string {
	whole := int(f)
	frac := int((f - float64(whole)) * 10)
	if frac < 0 {
		frac = -frac
	}
	return formatStatsInt(whole) + "." + formatStatsInt(frac)
}

// formatStatsDuration formats seconds as a nice duration string.
func formatStatsDuration(seconds float64) string {
	if seconds < 1 {
		ms := int(seconds * 1000)
		return formatStatsInt(ms) + "ms"
	}
	return formatStatsFloat(seconds) + "s"
}

// =============================================================================
// STREAM ACCUMULATOR
// =============================================================================

// StreamAccumulator collects streaming chunks and builds statistics.
type StreamAccumulator struct {
	// PERFORMANCE: strings.Builder avoids quadratic allocations
	content       strings.Builder
	Stats         *StreamStats
	Done          bool
	Error         error
}

// NewStreamAccumulator creates a new accumulator.
func NewStreamAccumulator() *StreamAccumulator {
	return &StreamAccumulator{
		Stats: NewStreamStats(),
	}
}

// Add processes a new chunk.
func (a *StreamAccumulator) Add(chunk StreamChunk) {
	if chunk.Error != nil {
		a.Error = chunk.Error
		a.Done = true
		return
	}

	// Record first token
	if chunk.Content != "" && a.content.Len() == 0 {
		a.Stats.RecordFirstToken()
	}

	// Accumulate content
	a.content.WriteString(chunk.Content)

	// Check if done
	if chunk.Done {
		a.Done = true
		a.Stats.Finalize(chunk)
	}
}

// GetContent returns the accumulated content.
func (a *StreamAccumulator) GetContent() string {
	return a.content.String()
}

// IsDone returns whether streaming is complete.
func (a *StreamAccumulator) IsDone() bool {
	return a.Done
}

// GetError returns any error that occurred.
func (a *StreamAccumulator) GetError() error {
	return a.Error
}

// GetStats returns the collected statistics.
func (a *StreamAccumulator) GetStats() *StreamStats {
	return a.Stats
}
