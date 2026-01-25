// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ollama provides the HTTP client for communicating with Ollama API.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// =============================================================================
// ERROR TYPES
// =============================================================================

// ClientError represents an error from the Ollama client.
type ClientError struct {
	Type    ErrorType
	Message string
	Cause   error
}

func (e *ClientError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *ClientError) Unwrap() error {
	return e.Cause
}

// ErrorType categorizes client errors for handling.
type ErrorType int

const (
	ErrTypeUnknown ErrorType = iota
	ErrTypeNotRunning
	ErrTypeTimeout
	ErrTypeModelNotFound
	ErrTypeContextExceeded
	ErrTypeConnection
	ErrTypeInvalidResponse
)

// Sentinel errors for easy checking.
var (
	ErrNotRunning      = &ClientError{Type: ErrTypeNotRunning, Message: "Ollama is not running"}
	ErrTimeout         = &ClientError{Type: ErrTypeTimeout, Message: "request timed out"}
	ErrModelNotFound   = &ClientError{Type: ErrTypeModelNotFound, Message: "model not found"}
	ErrContextExceeded = &ClientError{Type: ErrTypeContextExceeded, Message: "context window exceeded"}
)

// =============================================================================
// CLIENT CONFIGURATION
// =============================================================================

// ClientConfig holds configuration options for the Ollama client.
type ClientConfig struct {
	// BaseURL is the Ollama API base URL (default: http://127.0.0.1:11434)
	// Note: Uses explicit IPv4 address instead of localhost to avoid IPv6 resolution issues on Windows
	BaseURL string

	// Timeout for non-streaming requests (default: 30s)
	Timeout time.Duration

	// StreamTimeout for establishing streaming connections (default: 5s)
	StreamTimeout time.Duration

	// DefaultModel to use if none specified (default: "qwen2.5-coder:14b")
	DefaultModel string

	// MaxRetries for transient failures (default: 3)
	MaxRetries int

	// RetryDelay between retries (default: 1s)
	RetryDelay time.Duration
}

// DefaultConfig returns the default client configuration.
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		BaseURL:       "http://127.0.0.1:11434",
		Timeout:       30 * time.Second,
		StreamTimeout: 5 * time.Second,
		DefaultModel:  "qwen2.5-coder:14b",
		MaxRetries:    3,
		RetryDelay:    1 * time.Second,
	}
}

// =============================================================================
// CLIENT
// =============================================================================

// Client handles communication with the Ollama API.
// It provides methods for health checks, model management, and chat operations.
//
// The Client is thread-safe for concurrent use.
//
// Example:
//
//	client := ollama.NewClient()
//	if err := client.EnsureRunning(ctx); err != nil {
//	    log.Fatal("Ollama not available:", err)
//	}
//	resp, err := client.Chat(ctx, "qwen2.5-coder:7b", messages)
type Client struct {
	config     *ClientConfig
	httpClient *http.Client
}

// NewClient creates a new Ollama client with default configuration.
func NewClient() *Client {
	return NewClientWithConfig(DefaultConfig())
}

// NewClientWithConfig creates a new Ollama client with custom configuration.
func NewClientWithConfig(config *ClientConfig) *Client {
	if config == nil {
		config = DefaultConfig()
	}

	// Fill in defaults for any zero values
	if config.BaseURL == "" {
		config.BaseURL = "http://127.0.0.1:11434"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.StreamTimeout == 0 {
		config.StreamTimeout = 5 * time.Second
	}
	if config.DefaultModel == "" {
		config.DefaultModel = "qwen2.5-coder:14b"
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// =============================================================================
// HEALTH CHECK
// =============================================================================

// CheckRunning verifies that Ollama is reachable and running.
func (c *Client) CheckRunning(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.BaseURL, nil)
	if err != nil {
		return &ClientError{Type: ErrTypeConnection, Message: "failed to create request", Cause: err}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return ErrNotRunning
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ClientError{
			Type:    ErrTypeConnection,
			Message: "unexpected status from Ollama: " + resp.Status,
		}
	}

	return nil
}

// StartOllama attempts to start the Ollama server if it's not running.
// Returns nil if Ollama is already running or was successfully started.
// The actual start logic is platform-specific (see start_windows.go and start_unix.go).
func (c *Client) StartOllama(ctx context.Context) error {
	// First check if already running
	if err := c.CheckRunning(ctx); err == nil {
		return nil // Already running
	}

	// Use platform-specific implementation to start Ollama
	// This handles finding the executable, setting proper process attributes,
	// and waiting for Ollama to become ready
	return c.startOllamaProcess(ctx)
}

// EnsureRunning checks if Ollama is running, and starts it if not.
// This is a convenience method that combines CheckRunning and StartOllama.
func (c *Client) EnsureRunning(ctx context.Context) error {
	if err := c.CheckRunning(ctx); err == nil {
		return nil
	}
	return c.StartOllama(ctx)
}

// =============================================================================
// MODEL OPERATIONS
// =============================================================================

// ListModels retrieves all available models from Ollama.
func (c *Client) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.BaseURL+"/api/tags", nil)
	if err != nil {
		return nil, &ClientError{Type: ErrTypeConnection, Message: "failed to create request", Cause: err}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, ErrNotRunning
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &ClientError{
			Type:    ErrTypeInvalidResponse,
			Message: "failed to list models: " + resp.Status,
		}
	}

	var result ListModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &ClientError{Type: ErrTypeInvalidResponse, Message: "failed to decode response", Cause: err}
	}

	return result.Models, nil
}

// GetModel retrieves information about a specific model.
func (c *Client) GetModel(ctx context.Context, name string) (*ShowModelResponse, error) {
	reqBody := ShowModelRequest{Name: name}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &ClientError{Type: ErrTypeInvalidResponse, Message: "failed to marshal request", Cause: err}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/api/show", bytes.NewReader(body))
	if err != nil {
		return nil, &ClientError{Type: ErrTypeConnection, Message: "failed to create request", Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, ErrNotRunning
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrModelNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &ClientError{
			Type:    ErrTypeInvalidResponse,
			Message: "failed to get model: " + resp.Status,
		}
	}

	var result ShowModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &ClientError{Type: ErrTypeInvalidResponse, Message: "failed to decode response", Cause: err}
	}

	return &result, nil
}

// =============================================================================
// CHAT OPERATIONS
// =============================================================================

// Chat sends a chat request and returns the complete response (non-streaming).
func (c *Client) Chat(ctx context.Context, model string, messages []Message) (*ChatResponse, error) {
	if model == "" {
		model = c.config.DefaultModel
	}

	reqBody := ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &ClientError{Type: ErrTypeInvalidResponse, Message: "failed to marshal request", Cause: err}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, &ClientError{Type: ErrTypeConnection, Message: "failed to create request", Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, ErrNotRunning
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrModelNotFound
	}

	if resp.StatusCode != http.StatusOK {
		// Try to read error message
		var ollamaErr OllamaError
		if err := json.NewDecoder(resp.Body).Decode(&ollamaErr); err == nil && ollamaErr.Error != "" {
			return nil, &ClientError{
				Type:    ErrTypeInvalidResponse,
				Message: ollamaErr.Error,
			}
		}
		return nil, &ClientError{
			Type:    ErrTypeInvalidResponse,
			Message: "chat request failed: " + resp.Status,
		}
	}

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &ClientError{Type: ErrTypeInvalidResponse, Message: "failed to decode response", Cause: err}
	}

	return &result, nil
}

// ChatWithOptions sends a chat request with custom options.
func (c *Client) ChatWithOptions(ctx context.Context, model string, messages []Message, opts *Options) (*ChatResponse, error) {
	if model == "" {
		model = c.config.DefaultModel
	}

	reqBody := ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
		Options:  opts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &ClientError{Type: ErrTypeInvalidResponse, Message: "failed to marshal request", Cause: err}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, &ClientError{Type: ErrTypeConnection, Message: "failed to create request", Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, ErrNotRunning
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrModelNotFound
	}

	if resp.StatusCode != http.StatusOK {
		var ollamaErr OllamaError
		if err := json.NewDecoder(resp.Body).Decode(&ollamaErr); err == nil && ollamaErr.Error != "" {
			return nil, &ClientError{
				Type:    ErrTypeInvalidResponse,
				Message: ollamaErr.Error,
			}
		}
		return nil, &ClientError{
			Type:    ErrTypeInvalidResponse,
			Message: "chat request failed: " + resp.Status,
		}
	}

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &ClientError{Type: ErrTypeInvalidResponse, Message: "failed to decode response", Cause: err}
	}

	return &result, nil
}

// =============================================================================
// STREAMING CHAT
// =============================================================================

// StreamCallback is called for each chunk received during streaming.
type StreamCallback func(chunk StreamChunk)

// ChatStream sends a streaming chat request and calls the callback for each chunk.
// The callback is called synchronously in the order chunks are received.
// Returns when streaming is complete or an error occurs.
func (c *Client) ChatStream(ctx context.Context, model string, messages []Message, callback StreamCallback) error {
	if model == "" {
		model = c.config.DefaultModel
	}

	reqBody := ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return &ClientError{Type: ErrTypeInvalidResponse, Message: "failed to marshal request", Cause: err}
	}

	// Use a client without timeout for streaming (we handle timeout via context)
	// SECURITY: TLS not required - Ollama runs locally on localhost (127.0.0.1) over HTTP
	// TLS configuration would not apply to this local HTTP connection
	streamClient := &http.Client{}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return &ClientError{Type: ErrTypeConnection, Message: "failed to create request", Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := streamClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return ErrTimeout
		}
		return ErrNotRunning
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrModelNotFound
	}

	if resp.StatusCode != http.StatusOK {
		var ollamaErr OllamaError
		if err := json.NewDecoder(resp.Body).Decode(&ollamaErr); err == nil && ollamaErr.Error != "" {
			return &ClientError{
				Type:    ErrTypeInvalidResponse,
				Message: ollamaErr.Error,
			}
		}
		return &ClientError{
			Type:    ErrTypeInvalidResponse,
			Message: "stream request failed: " + resp.Status,
		}
	}

	// Create stream reader and process
	reader := NewStreamReader(resp.Body)
	return reader.Process(ctx, callback)
}

// ChatStreamChan sends a streaming chat request and returns a channel of chunks.
// The channel is closed when streaming is complete or an error occurs.
// Errors are delivered as chunks with the Error field set.
func (c *Client) ChatStreamChan(ctx context.Context, model string, messages []Message) <-chan StreamChunk {
	ch := make(chan StreamChunk)

	go func() {
		defer close(ch)

		err := c.ChatStream(ctx, model, messages, func(chunk StreamChunk) {
			select {
			case ch <- chunk:
			case <-ctx.Done():
				return
			}
		})

		if err != nil {
			select {
			case ch <- StreamChunk{Error: err, Done: true}:
			case <-ctx.Done():
			}
		}
	}()

	return ch
}

// ChatStreamWithTools sends a streaming chat request with tool definitions.
// The callback is called for each chunk received, including tool calls.
func (c *Client) ChatStreamWithTools(ctx context.Context, model string, messages []Message, tools []Tool, callback StreamCallback) error {
	if model == "" {
		model = c.config.DefaultModel
	}

	reqBody := ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
		Tools:    tools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return &ClientError{Type: ErrTypeInvalidResponse, Message: "failed to marshal request", Cause: err}
	}

	// Use a client without timeout for streaming (we handle timeout via context)
	// SECURITY: TLS not required - Ollama runs locally on localhost (127.0.0.1) over HTTP
	// TLS configuration would not apply to this local HTTP connection
	streamClient := &http.Client{}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return &ClientError{Type: ErrTypeConnection, Message: "failed to create request", Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := streamClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return ErrTimeout
		}
		return ErrNotRunning
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrModelNotFound
	}

	if resp.StatusCode != http.StatusOK {
		var ollamaErr OllamaError
		if err := json.NewDecoder(resp.Body).Decode(&ollamaErr); err == nil && ollamaErr.Error != "" {
			return &ClientError{
				Type:    ErrTypeInvalidResponse,
				Message: ollamaErr.Error,
			}
		}
		return &ClientError{
			Type:    ErrTypeInvalidResponse,
			Message: "stream request failed: " + resp.Status,
		}
	}

	// Create stream reader and process
	reader := NewStreamReader(resp.Body)
	return reader.Process(ctx, callback)
}

// =============================================================================
// EMBEDDINGS
// =============================================================================

// GenerateEmbedding creates an embedding vector for the given text.
func (c *Client) GenerateEmbedding(ctx context.Context, model string, text string) ([]float64, error) {
	if model == "" {
		model = c.config.DefaultModel
	}

	reqBody := EmbeddingRequest{
		Model:  model,
		Prompt: text,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &ClientError{Type: ErrTypeInvalidResponse, Message: "failed to marshal request", Cause: err}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, &ClientError{Type: ErrTypeConnection, Message: "failed to create request", Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, ErrNotRunning
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrModelNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &ClientError{
			Type:    ErrTypeInvalidResponse,
			Message: "embedding request failed: " + resp.Status,
		}
	}

	var result EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, &ClientError{Type: ErrTypeInvalidResponse, Message: "failed to decode response", Cause: err}
	}

	return result.Embedding, nil
}

// =============================================================================
// UTILITY METHODS
// =============================================================================

// GetConfig returns the client configuration.
func (c *Client) GetConfig() *ClientConfig {
	return c.config
}

// SetModel updates the default model.
func (c *Client) SetModel(model string) {
	c.config.DefaultModel = model
}

// GetModel returns the current default model.
func (c *Client) GetDefaultModel() string {
	return c.config.DefaultModel
}

// ModelExists checks if a model is available locally.
// Returns true if the model exists, false otherwise.
func (c *Client) ModelExists(ctx context.Context, model string) bool {
	_, err := c.GetModel(ctx, model)
	return err == nil
}

// IsModelNotFound checks if an error is a model not found error.
func IsModelNotFound(err error) bool {
	var clientErr *ClientError
	if errors.As(err, &clientErr) {
		return clientErr.Type == ErrTypeModelNotFound
	}
	return errors.Is(err, ErrModelNotFound)
}

// IsNotRunning checks if an error indicates Ollama is not running.
func IsNotRunning(err error) bool {
	var clientErr *ClientError
	if errors.As(err, &clientErr) {
		return clientErr.Type == ErrTypeNotRunning
	}
	return errors.Is(err, ErrNotRunning)
}

// IsTimeout checks if an error is a timeout error.
func IsTimeout(err error) bool {
	var clientErr *ClientError
	if errors.As(err, &clientErr) {
		return clientErr.Type == ErrTypeTimeout
	}
	return errors.Is(err, ErrTimeout)
}

// Helper to drain response body
func drainAndClose(r io.ReadCloser) {
	io.Copy(io.Discard, r)
	r.Close()
}
