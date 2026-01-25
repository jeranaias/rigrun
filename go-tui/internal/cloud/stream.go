// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cloud

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// STREAMING: Robust SSE parsing with error handling

// =============================================================================
// STREAMING CONSTANTS
// =============================================================================

// MaxChunkSize is the maximum allowed size for a single SSE chunk (64KB)
const MaxChunkSize = 64 * 1024

// =============================================================================
// STREAMING TYPES
// =============================================================================

// StreamChunk represents a single chunk from the OpenRouter streaming response.
type StreamChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
			Role    string `json:"role,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error error `json:"-"` // Error field for channel-based streaming
}

// GetContent returns the content from the first choice's delta.
func (c *StreamChunk) GetContent() string {
	if len(c.Choices) > 0 {
		return c.Choices[0].Delta.Content
	}
	return ""
}

// GetRole returns the role from the first choice's delta.
func (c *StreamChunk) GetRole() string {
	if len(c.Choices) > 0 {
		return c.Choices[0].Delta.Role
	}
	return ""
}

// IsDone returns true if the stream has finished.
func (c *StreamChunk) IsDone() bool {
	if len(c.Choices) > 0 {
		return c.Choices[0].FinishReason != ""
	}
	return false
}

// GetFinishReason returns the finish reason if streaming is complete.
func (c *StreamChunk) GetFinishReason() string {
	if len(c.Choices) > 0 {
		return c.Choices[0].FinishReason
	}
	return ""
}

// HasError returns true if the chunk contains an error.
func (c *StreamChunk) HasError() bool {
	return c.Error != nil
}

// StreamCallback is the function type called for each received chunk.
type StreamCallback func(chunk StreamChunk)

// CompletionRequest represents a generic completion request for streaming.
type CompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream"`
}

// StreamStats holds statistics collected during streaming.
type StreamStats struct {
	FirstTokenTime time.Duration
	TotalTime      time.Duration
	TokenCount     int
	Model          string
}

// StreamError represents an error that occurred during streaming,
// preserving any partial content received before the error.
type StreamError struct {
	Partial string // Content received before error
	Err     error
}

// Error implements the error interface.
func (e *StreamError) Error() string {
	if e.Partial != "" {
		return fmt.Sprintf("stream error (partial content received: %d chars): %v", len(e.Partial), e.Err)
	}
	return fmt.Sprintf("stream error: %v", e.Err)
}

// Unwrap returns the underlying error.
func (e *StreamError) Unwrap() error {
	return e.Err
}

// =============================================================================
// SSE READER
// =============================================================================

// SSEReader parses Server-Sent Events from a stream.
type SSEReader struct {
	reader *bufio.Reader
}

// NewSSEReader creates a new SSE reader from an io.Reader.
func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{
		reader: bufio.NewReader(r),
	}
}

// ReadEvent reads the next SSE event from the stream.
// Returns the event type, data, and any error.
// The event type is typically empty for OpenRouter responses.
// Returns io.EOF when the stream ends.
func (s *SSEReader) ReadEvent() (string, []byte, error) {
	var eventType string
	var dataLines [][]byte

	for {
		line, err := s.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				// If we have data, return it before EOF
				if len(dataLines) > 0 {
					return eventType, bytes.Join(dataLines, []byte("\n")), nil
				}
				return "", nil, io.EOF
			}
			return "", nil, err
		}

		// Trim trailing newline and carriage return
		line = bytes.TrimRight(line, "\r\n")

		// Empty line signals end of event
		if len(line) == 0 {
			if len(dataLines) > 0 {
				return eventType, bytes.Join(dataLines, []byte("\n")), nil
			}
			continue
		}

		// Parse field
		if bytes.HasPrefix(line, []byte("event:")) {
			eventType = string(bytes.TrimSpace(line[6:]))
		} else if bytes.HasPrefix(line, []byte("data:")) {
			data := bytes.TrimSpace(line[5:])
			dataLines = append(dataLines, data)
		} else if bytes.HasPrefix(line, []byte("data: ")) {
			data := line[6:]
			dataLines = append(dataLines, data)
		}
		// Ignore other fields (id:, retry:, comments starting with :)
	}
}

// =============================================================================
// STREAMING CHAT
// =============================================================================

// ChatStream performs a streaming chat completion request.
// The callback is called for each chunk received.
// Supports context cancellation.
func (c *OpenRouterClient) ChatStream(ctx context.Context, messages []ChatMessage, callback StreamCallback) error {
	if !c.IsConfigured() {
		return ErrNotConfigured
	}

	url := c.baseURL + "/chat/completions"

	reqBody := ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// PERFORMANCE: Use shared streaming client with connection pooling (timeout handled via context)
	// SECURITY: TLS 1.2+ enforced via shared client configuration
	resp, err := sharedStreamingClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return c.handleErrorResponse(resp.StatusCode, body)
	}

	return c.processStream(ctx, resp.Body, callback)
}

// processStream reads and processes the SSE stream.
func (c *OpenRouterClient) processStream(ctx context.Context, body io.Reader, callback StreamCallback) error {
	reader := NewSSEReader(body)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, data, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		// Check for [DONE] signal
		if bytes.Equal(data, []byte("[DONE]")) {
			return nil
		}

		// Parse the chunk
		var chunk StreamChunk
		if err := json.Unmarshal(data, &chunk); err != nil {
			// Skip malformed chunks
			continue
		}

		callback(chunk)

		// Check if finished
		if chunk.IsDone() {
			return nil
		}
	}
}

// ChatStreamWithStats performs a streaming chat and collects statistics.
func (c *OpenRouterClient) ChatStreamWithStats(ctx context.Context, messages []ChatMessage, callback StreamCallback) (*StreamStats, error) {
	stats := &StreamStats{}
	startTime := time.Now()
	var firstTokenTime time.Time
	tokenCount := 0

	wrappedCallback := func(chunk StreamChunk) {
		content := chunk.GetContent()
		if content != "" {
			tokenCount++
			if firstTokenTime.IsZero() {
				firstTokenTime = time.Now()
				stats.FirstTokenTime = firstTokenTime.Sub(startTime)
			}
		}
		if chunk.Model != "" {
			stats.Model = chunk.Model
		}
		callback(chunk)
	}

	err := c.ChatStream(ctx, messages, wrappedCallback)

	stats.TotalTime = time.Since(startTime)
	stats.TokenCount = tokenCount

	return stats, err
}

// =============================================================================
// STREAMING WITH RETRY
// =============================================================================

// streamWithRetry performs a streaming request with retry logic.
// Retries on connection errors but not on 4xx errors.
func (c *OpenRouterClient) streamWithRetry(ctx context.Context, req *http.Request, callback StreamCallback) error {
	var lastErr error
	var accumulated strings.Builder

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		// Apply backoff delay after first attempt
		if attempt > 0 {
			delay := c.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		// Clone the request body for retry
		var bodyBytes []byte
		if req.Body != nil {
			bodyBytes, _ = io.ReadAll(req.Body)
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// Create new request for retry
		newReq := req.Clone(ctx)
		if bodyBytes != nil {
			newReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// PERFORMANCE: Use shared streaming client with connection pooling
		// SECURITY: TLS 1.2+ enforced via shared client configuration
		resp, err := sharedStreamingClient.Do(newReq)
		if err != nil {
			lastErr = err
			continue
		}

		// Don't retry on 4xx errors
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return c.handleErrorResponse(resp.StatusCode, body)
		}

		// Check for other error responses
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = c.handleErrorResponse(resp.StatusCode, body)
			continue
		}

		// Wrap callback to accumulate content
		wrappedCallback := func(chunk StreamChunk) {
			accumulated.WriteString(chunk.GetContent())
			callback(chunk)
		}

		err = c.processStream(ctx, resp.Body, wrappedCallback)
		resp.Body.Close()

		if err != nil {
			// Connection error during streaming
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				lastErr = &StreamError{
					Partial: accumulated.String(),
					Err:     err,
				}
				continue
			}
			return err
		}

		return nil
	}

	// All retries exhausted
	if lastErr != nil {
		if accumulated.Len() > 0 {
			return &StreamError{
				Partial: accumulated.String(),
				Err:     fmt.Errorf("max retries exceeded: %w", lastErr),
			}
		}
		return fmt.Errorf("max retries exceeded: %w", lastErr)
	}
	return errors.New("max retries exceeded")
}

// ChatStreamWithRetry performs a streaming chat with retry logic.
func (c *OpenRouterClient) ChatStreamWithRetry(ctx context.Context, messages []ChatMessage, callback StreamCallback) error {
	if !c.IsConfigured() {
		return ErrNotConfigured
	}

	url := c.baseURL + "/chat/completions"

	reqBody := ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   true,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	return c.streamWithRetry(ctx, req, callback)
}

// =============================================================================
// RATE LIMIT HANDLING
// =============================================================================

// handleRateLimit handles rate limit responses by parsing Retry-After
// and waiting the appropriate time before returning.
func (c *OpenRouterClient) handleRateLimit(resp *http.Response) error {
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return ErrRateLimited
	}

	// Try to parse as seconds
	seconds, err := strconv.Atoi(retryAfter)
	if err == nil {
		return &RateLimitError{
			RetryAfter: time.Duration(seconds) * time.Second,
		}
	}

	// Try to parse as HTTP date
	t, err := http.ParseTime(retryAfter)
	if err == nil {
		return &RateLimitError{
			RetryAfter: time.Until(t),
		}
	}

	return ErrRateLimited
}

// RateLimitError represents a rate limit error with retry information.
type RateLimitError struct {
	RetryAfter time.Duration
}

// Error implements the error interface.
func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("rate limited, retry after %v", e.RetryAfter)
	}
	return "rate limited"
}

// Is allows RateLimitError to be compared with ErrRateLimited.
func (e *RateLimitError) Is(target error) bool {
	return target == ErrRateLimited
}

// =============================================================================
// ACCUMULATED RESPONSE
// =============================================================================

// ChatStreamAccumulate performs a streaming chat but returns the full response
// at the end. This is useful for simple use cases where you want streaming
// for progress but need the complete response.
func (c *OpenRouterClient) ChatStreamAccumulate(ctx context.Context, messages []ChatMessage) (string, error) {
	var accumulated strings.Builder

	err := c.ChatStream(ctx, messages, func(chunk StreamChunk) {
		accumulated.WriteString(chunk.GetContent())
	})

	if err != nil {
		// Check if it's a stream error with partial content
		var streamErr *StreamError
		if errors.As(err, &streamErr) && streamErr.Partial != "" {
			return streamErr.Partial, err
		}
		return accumulated.String(), err
	}

	return accumulated.String(), nil
}

// =============================================================================
// TOKEN COUNTER WRAPPER
// =============================================================================

// WithTokenCounter wraps a callback to count tokens as they arrive.
// The counter is incremented for each chunk that contains content.
func WithTokenCounter(callback StreamCallback, counter *int) StreamCallback {
	return func(chunk StreamChunk) {
		if chunk.GetContent() != "" {
			*counter++
		}
		callback(chunk)
	}
}

// =============================================================================
// STREAM ACCUMULATOR
// =============================================================================

// StreamAccumulator collects streaming chunks and builds a complete response.
type StreamAccumulator struct {
	Content      strings.Builder
	TokenCount   int
	Model        string
	FinishReason string
	StartTime    time.Time
	FirstTokenAt time.Time
	Done         bool
	Error        error
}

// NewStreamAccumulator creates a new accumulator.
func NewStreamAccumulator() *StreamAccumulator {
	return &StreamAccumulator{
		StartTime: time.Now(),
	}
}

// Add processes a new chunk.
func (a *StreamAccumulator) Add(chunk StreamChunk) {
	content := chunk.GetContent()
	if content != "" {
		a.TokenCount++
		if a.FirstTokenAt.IsZero() {
			a.FirstTokenAt = time.Now()
		}
		a.Content.WriteString(content)
	}

	if chunk.Model != "" {
		a.Model = chunk.Model
	}

	if chunk.IsDone() {
		a.Done = true
		a.FinishReason = chunk.GetFinishReason()
	}
}

// GetContent returns the accumulated content.
func (a *StreamAccumulator) GetContent() string {
	return a.Content.String()
}

// GetStats returns the collected statistics.
func (a *StreamAccumulator) GetStats() *StreamStats {
	var ttft time.Duration
	if !a.FirstTokenAt.IsZero() {
		ttft = a.FirstTokenAt.Sub(a.StartTime)
	}

	return &StreamStats{
		FirstTokenTime: ttft,
		TotalTime:      time.Since(a.StartTime),
		TokenCount:     a.TokenCount,
		Model:          a.Model,
	}
}

// Callback returns a StreamCallback that accumulates to this accumulator.
func (a *StreamAccumulator) Callback() StreamCallback {
	return func(chunk StreamChunk) {
		a.Add(chunk)
	}
}

// =============================================================================
// CHANNEL-BASED STREAMING
// =============================================================================

// ChatStreamChan performs a streaming chat and returns a channel of chunks.
// The channel is closed when streaming is complete or an error occurs.
// Errors are available via the returned error channel.
func (c *OpenRouterClient) ChatStreamChan(ctx context.Context, messages []ChatMessage) (<-chan StreamChunk, <-chan error) {
	chunkChan := make(chan StreamChunk, 64)
	errChan := make(chan error, 1)

	go func() {
		defer close(chunkChan)
		defer close(errChan)

		err := c.ChatStream(ctx, messages, func(chunk StreamChunk) {
			select {
			case chunkChan <- chunk:
			case <-ctx.Done():
				return
			}
		})

		if err != nil {
			select {
			case errChan <- err:
			case <-ctx.Done():
			}
		}
	}()

	return chunkChan, errChan
}

// =============================================================================
// CHANNEL-BASED COMPLETION STREAMING
// =============================================================================

// StreamCompletion performs a streaming completion request and returns a channel of chunks.
// This function provides robust SSE parsing with proper error handling.
// Errors are delivered through the StreamChunk.Error field.
func (c *OpenRouterClient) StreamCompletion(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	if !c.IsConfigured() {
		return nil, ErrNotConfigured
	}

	chunks := make(chan StreamChunk, 100)

	go func() {
		defer close(chunks)

		resp, err := c.sendStreamRequest(ctx, req)
		if err != nil {
			chunks <- StreamChunk{Error: err}
			return
		}
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			select {
			case <-ctx.Done():
				chunks <- StreamChunk{Error: ctx.Err()}
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err == io.EOF {
				return
			}
			if err != nil {
				chunks <- StreamChunk{Error: fmt.Errorf("read error: %w", err)}
				return
			}

			// Parse SSE
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				data = strings.TrimSpace(data)

				if data == "[DONE]" {
					return
				}

				var chunk StreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					chunks <- StreamChunk{Error: fmt.Errorf("parse error: %w", err)}
					continue // Don't abort on single parse error
				}

				chunks <- chunk
			}
		}
	}()

	return chunks, nil
}

// sendStreamRequest sends the streaming HTTP request and returns the response.
func (c *OpenRouterClient) sendStreamRequest(ctx context.Context, req *CompletionRequest) (*http.Response, error) {
	url := c.baseURL + "/chat/completions"

	// Ensure stream is enabled
	req.Stream = true

	// Use client's model if not specified in request
	if req.Model == "" {
		req.Model = c.model
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	httpReq.Header.Set("Connection", "keep-alive")

	// PERFORMANCE: Use shared streaming client with connection pooling
	// SECURITY: TLS 1.2+ enforced via shared client configuration
	resp, err := sharedStreamingClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, c.handleErrorResponse(resp.StatusCode, body)
	}

	return resp, nil
}

// =============================================================================
// BUFFER MANAGEMENT
// =============================================================================

// readChunk reads a complete SSE event from the reader with buffer size limits.
// Returns the complete event data or an error if the chunk exceeds MaxChunkSize.
func (c *OpenRouterClient) readChunk(reader *bufio.Reader) (string, error) {
	var buf strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return buf.String(), err
		}

		buf.WriteString(line)
		if buf.Len() > MaxChunkSize {
			return "", fmt.Errorf("chunk too large: %d bytes", buf.Len())
		}

		if line == "\n" { // End of SSE event
			return buf.String(), nil
		}
	}
}

// =============================================================================
// STREAMING WITH RECONNECTION
// =============================================================================

// ReconnectState tracks the state for reconnection logic.
type ReconnectState struct {
	LastEventID    string
	AccumulatedLen int
	RetryCount     int
	MaxRetries     int
}

// StreamWithReconnect performs a streaming completion with automatic reconnection
// for dropped connections. It tracks progress for resume capability if supported by the API.
func (c *OpenRouterClient) StreamWithReconnect(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	if !c.IsConfigured() {
		return nil, ErrNotConfigured
	}

	chunks := make(chan StreamChunk, 100)

	go func() {
		defer close(chunks)

		state := &ReconnectState{
			MaxRetries: c.maxRetries,
		}

		for state.RetryCount <= state.MaxRetries {
			// Check context before attempting connection
			select {
			case <-ctx.Done():
				chunks <- StreamChunk{Error: ctx.Err()}
				return
			default:
			}

			err := c.streamWithState(ctx, req, chunks, state)
			if err == nil {
				// Completed successfully
				return
			}

			// Check if error is retryable
			if !isStreamRetryable(err) {
				chunks <- StreamChunk{Error: err}
				return
			}

			// Context errors are not retryable
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				chunks <- StreamChunk{Error: err}
				return
			}

			// Apply backoff before retry
			state.RetryCount++
			if state.RetryCount > state.MaxRetries {
				chunks <- StreamChunk{Error: fmt.Errorf("max reconnection attempts exceeded: %w", err)}
				return
			}

			delay := c.calculateBackoff(state.RetryCount)
			select {
			case <-ctx.Done():
				chunks <- StreamChunk{Error: ctx.Err()}
				return
			case <-time.After(delay):
				// Continue to retry
			}
		}
	}()

	return chunks, nil
}

// streamWithState performs a single streaming attempt with state tracking.
func (c *OpenRouterClient) streamWithState(ctx context.Context, req *CompletionRequest, chunks chan<- StreamChunk, state *ReconnectState) error {
	resp, err := c.sendStreamRequest(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Use buffer-safe reading
		eventData, err := c.readChunk(reader)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		// Parse SSE lines from the event
		lines := strings.Split(eventData, "\n")
		for _, line := range lines {
			// Track event ID for reconnection
			if strings.HasPrefix(line, "id: ") {
				state.LastEventID = strings.TrimPrefix(line, "id: ")
				state.LastEventID = strings.TrimSpace(state.LastEventID)
			}

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				data = strings.TrimSpace(data)

				if data == "[DONE]" {
					return nil
				}

				var chunk StreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					chunks <- StreamChunk{Error: fmt.Errorf("parse error: %w", err)}
					continue // Don't abort on single parse error
				}

				// Track accumulated content length for progress
				state.AccumulatedLen += len(chunk.GetContent())

				chunks <- chunk
			}
		}
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// isStreamRetryable determines if a streaming error should trigger a retry.
func isStreamRetryable(err error) bool {
	// Don't retry on context cancellation
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Don't retry on client errors (4xx)
	var orErr *OpenRouterError
	if errors.As(err, &orErr) {
		if orErr.Status >= 400 && orErr.Status < 500 {
			return false
		}
		return orErr.Status >= 500
	}

	// Rate limiting can be retried
	if errors.Is(err, ErrRateLimited) {
		return true
	}

	// Network errors are retryable
	return true
}
