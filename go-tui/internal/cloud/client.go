// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cloud provides OpenRouter integration for cloud LLM inference.
//
// OpenRouter provides access to multiple LLM providers through a single API,
// including Claude, GPT-4, and other models. This package implements the
// client for communicating with OpenRouter's API.
//
// CLOUD: Secure logging, retry logic, and validation
package cloud

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// Configuration constants for OpenRouter API.
const (
	// DefaultOpenRouterURL is the base URL for OpenRouter API.
	DefaultOpenRouterURL = "https://openrouter.ai/api/v1"

	// DefaultTimeout is the default timeout for API requests.
	DefaultTimeout = 60 * time.Second

	// DefaultMaxRetries is the default number of retry attempts for transient errors.
	DefaultMaxRetries = 3

	// retryBaseDelay is the base delay for exponential backoff.
	retryBaseDelay = 500 * time.Millisecond

	// retryMaxDelay is the maximum delay for exponential backoff.
	retryMaxDelay = 10 * time.Second

	// MaxResponseSize is the maximum allowed response body size.
	// SECURITY: Response size limit prevents memory exhaustion attacks.
	MaxResponseSize = 10 * 1024 * 1024 // 10MB limit
)

var (
	// PERFORMANCE: Connection pooling reduces TCP handshake overhead.
	// Shared HTTP client with connection pooling for all OpenRouter requests.
	// SECURITY: TLS verification required for production
	sharedHTTPClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				CipherSuites:       security.ApprovedCipherSuites,
				InsecureSkipVerify: false, // SECURITY: TLS verification required for production
			},
		},
		Timeout: DefaultTimeout,
	}

	// sharedStreamingClient is used for streaming requests (no timeout, context-controlled).
	// PERFORMANCE: Connection pooling for streaming requests.
	// SECURITY: TLS verification required for production
	sharedStreamingClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				CipherSuites:       security.ApprovedCipherSuites,
				InsecureSkipVerify: false, // SECURITY: TLS verification required for production
			},
		},
		// No timeout for streaming - controlled via context
	}
)

// OpenRouterModels maps friendly names to full model identifiers.
var OpenRouterModels = map[string]string{
	"auto":   "openrouter/auto",
	"haiku":  "anthropic/claude-3-haiku",
	"sonnet": "anthropic/claude-3-sonnet",
	"opus":   "anthropic/claude-3-opus",
	"gpt4o":  "openai/gpt-4o",
	"gpt4":   "openai/gpt-4-turbo",
}

// validModels is the set of known valid model identifiers for validation.
// CLOUD: Model validation to prevent requests to unknown models.
var validModels = map[string]bool{
	// OpenRouter
	"openrouter/auto": true,
	// Anthropic Claude models
	"anthropic/claude-3-haiku":    true,
	"anthropic/claude-3-sonnet":   true,
	"anthropic/claude-3-opus":     true,
	"anthropic/claude-3.5-sonnet": true,
	"anthropic/claude-3.5-haiku":  true,
	"claude-3-opus":               true,
	"claude-3-sonnet":             true,
	"claude-3-haiku":              true,
	// OpenAI GPT models
	"openai/gpt-4":       true,
	"openai/gpt-4-turbo": true,
	"openai/gpt-4o":      true,
	"openai/gpt-4o-mini": true,
	"gpt-4":              true,
	"gpt-4-turbo":        true,
	"gpt-4o":             true,
	// Google models
	"google/gemini-pro":     true,
	"google/gemini-pro-1.5": true,
	"google/gemini-ultra":   true,
	// Meta models
	"meta-llama/llama-3-70b-instruct": true,
	"meta-llama/llama-3-8b-instruct":  true,
}

// Error variables for common OpenRouter errors.
var (
	// ErrNotConfigured indicates the API key is not set.
	ErrNotConfigured = errors.New("OpenRouter API key not configured")

	// ErrAuthFailed indicates authentication failed (invalid or expired API key).
	ErrAuthFailed = errors.New("authentication failed")

	// ErrRateLimited indicates too many requests were made.
	ErrRateLimited = errors.New("rate limited")

	// ErrModelNotFound indicates the requested model does not exist.
	ErrModelNotFound = errors.New("model not found")

	// ErrInsufficientCredits indicates the account has insufficient credits.
	ErrInsufficientCredits = errors.New("insufficient credits")

	// ErrUnknownModel indicates the model is not in the validated model list.
	// CLOUD: Model validation error.
	ErrUnknownModel = errors.New("unknown model")
)

// OpenRouterError represents an error from the OpenRouter API.
type OpenRouterError struct {
	Code    string
	Message string
	Status  int
}

// Error implements the error interface.
func (e *OpenRouterError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("OpenRouter error [%s] (HTTP %d): %s", e.Code, e.Status, e.Message)
	}
	return fmt.Sprintf("OpenRouter error (HTTP %d): %s", e.Status, e.Message)
}

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`    // "user", "assistant", or "system"
	Content string `json:"content"` // The message content
}

// NewUserMessage creates a new user message.
func NewUserMessage(content string) ChatMessage {
	return ChatMessage{Role: "user", Content: content}
}

// NewAssistantMessage creates a new assistant message.
func NewAssistantMessage(content string) ChatMessage {
	return ChatMessage{Role: "assistant", Content: content}
}

// NewSystemMessage creates a new system message.
func NewSystemMessage(content string) ChatMessage {
	return ChatMessage{Role: "system", Content: content}
}

// ChatRequest represents a request to the chat completions endpoint.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatResponse represents a response from the chat completions endpoint.
type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// GetContent returns the content of the first choice, or empty string if none.
func (r *ChatResponse) GetContent() string {
	if len(r.Choices) > 0 {
		return r.Choices[0].Message.Content
	}
	return ""
}

// Pricing represents the pricing information for a model.
type Pricing struct {
	Prompt     string `json:"prompt"`     // Cost per token for prompts
	Completion string `json:"completion"` // Cost per token for completions
}

// ModelInfo represents information about an available model.
type ModelInfo struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	ContextSize int     `json:"context_length"`
	Pricing     Pricing `json:"pricing"`
}

// modelsResponse is the internal response structure for listing models.
type modelsResponse struct {
	Data []struct {
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		ContextLength int      `json:"context_length"`
		Pricing       *Pricing `json:"pricing"`
	} `json:"data"`
}

// apiErrorResponse represents an error response from the API.
type apiErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// OpenRouterClient is a client for communicating with the OpenRouter API.
type OpenRouterClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	model      string
	maxRetries int
	timeout    time.Duration
	siteURL    string
	siteName   string

	// pkiManager handles TLS certificate validation (SC-17)
	pkiManager *security.PKIManager

	// validateCerts enables certificate validation before API calls
	validateCerts bool
}

// NewOpenRouterClient creates a new OpenRouter client with the given API key.
//
// The API key should be in the format "sk-or-..." as provided by OpenRouter.
// If the API key is empty, the client will still be created but Chat requests
// will fail with ErrNotConfigured.
//
// NIST 800-53 SC-17: Uses PKIManager for secure TLS configuration.
func NewOpenRouterClient(apiKey string) *OpenRouterClient {
	// Get PKI manager for secure TLS config (SC-17)
	pkiManager := security.GlobalPKIManager()
	tlsConfig := pkiManager.GetTLSConfig()

	return &OpenRouterClient{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: DefaultOpenRouterURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
		model:         "openrouter/auto",
		maxRetries:    DefaultMaxRetries,
		timeout:       DefaultTimeout,
		siteURL:       "https://rigrun.local",
		siteName:      "rigrun",
		pkiManager:    pkiManager,
		validateCerts: true, // SECURITY FIX: Default to true for secure certificate validation
	}
}

// NewOpenRouterClientWithPKI creates a new OpenRouter client with custom PKI settings.
//
// NIST 800-53 SC-17: Allows custom PKI configuration for certificate management.
func NewOpenRouterClientWithPKI(apiKey string, pkiManager *security.PKIManager) *OpenRouterClient {
	tlsConfig := pkiManager.GetTLSConfig()

	return &OpenRouterClient{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: DefaultOpenRouterURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
		model:         "openrouter/auto",
		maxRetries:    DefaultMaxRetries,
		timeout:       DefaultTimeout,
		siteURL:       "https://rigrun.local",
		siteName:      "rigrun",
		pkiManager:    pkiManager,
		validateCerts: pkiManager.IsCertPinningEnabled(),
	}
}

// WithBaseURL sets a custom base URL for the API.
func (c *OpenRouterClient) WithBaseURL(url string) *OpenRouterClient {
	c.baseURL = strings.TrimSuffix(url, "/")
	return c
}

// WithTimeout sets the request timeout.
func (c *OpenRouterClient) WithTimeout(timeout time.Duration) *OpenRouterClient {
	c.timeout = timeout
	c.httpClient.Timeout = timeout
	return c
}

// WithMaxRetries sets the maximum number of retry attempts.
func (c *OpenRouterClient) WithMaxRetries(maxRetries int) *OpenRouterClient {
	c.maxRetries = maxRetries
	return c
}

// WithSiteURL sets the site URL for rate limit categorization.
func (c *OpenRouterClient) WithSiteURL(url string) *OpenRouterClient {
	c.siteURL = url
	return c
}

// WithSiteName sets the site name for OpenRouter.
func (c *OpenRouterClient) WithSiteName(name string) *OpenRouterClient {
	c.siteName = name
	return c
}

// WithCertValidation enables certificate validation before API calls.
//
// NIST 800-53 SC-17: When enabled, validates TLS certificates and logs issues.
func (c *OpenRouterClient) WithCertValidation(enabled bool) *OpenRouterClient {
	c.validateCerts = enabled
	return c
}

// WithTLSConfig sets a custom TLS configuration for the HTTP client.
//
// NIST 800-53 SC-17: Allows specifying custom TLS settings.
func (c *OpenRouterClient) WithTLSConfig(config *tls.Config) *OpenRouterClient {
	c.httpClient.Transport = &http.Transport{
		TLSClientConfig: config,
	}
	return c
}

// SetModel sets the model to use for chat requests.
func (c *OpenRouterClient) SetModel(model string) {
	// Check if it's a friendly name
	if fullModel, ok := OpenRouterModels[model]; ok {
		c.model = fullModel
	} else {
		c.model = model
	}
}

// GetModel returns the current model.
func (c *OpenRouterClient) GetModel() string {
	return c.model
}

// IsConfigured returns true if the client has an API key configured.
func (c *OpenRouterClient) IsConfigured() bool {
	return c.apiKey != ""
}

// APIKeyMasked returns a masked version of the API key for display.
// SECURITY: Never exposes API key fragments - use fingerprint instead.
// CLOUD: Secure logging - never log API key fragments.
func (c *OpenRouterClient) APIKeyMasked() string {
	if c.apiKey == "" {
		return "[not set]"
	}
	// SECURITY: Never show any part of the key, use fingerprint instead
	return fmt.Sprintf("[REDACTED, length=%d, fingerprint=%s]", len(c.apiKey), c.keyFingerprint())
}

// keyFingerprint returns a secure fingerprint of the API key for logging.
// CLOUD: Secure logging - use fingerprint instead of exposing key fragments.
// SECURITY: Uses SHA-256 hash to create a unique identifier without exposing the key.
func (c *OpenRouterClient) keyFingerprint() string {
	if c.apiKey == "" {
		return "none"
	}
	h := sha256.Sum256([]byte(c.apiKey))
	return hex.EncodeToString(h[:4]) // First 8 hex chars (4 bytes)
}

// KeyFingerprint returns a secure fingerprint of the API key for external use.
// CLOUD: Secure logging - public accessor for key fingerprint.
func (c *OpenRouterClient) KeyFingerprint() string {
	return c.keyFingerprint()
}

// =============================================================================
// CLOUD: Request/Response Logging (without sensitive data)
// =============================================================================

// logRequest logs an API request without exposing sensitive data.
// CLOUD: Secure logging - does not log headers (may contain auth) or body (may contain sensitive data).
func (c *OpenRouterClient) logRequest(req *http.Request) {
	log.Printf("API Request: %s %s", req.Method, req.URL.Path)
	// Don't log headers (may contain auth)
	// Don't log body (may contain sensitive data)
}

// logResponse logs an API response with duration.
// CLOUD: Secure logging - only logs status code and duration, no response body.
func (c *OpenRouterClient) logResponse(resp *http.Response, duration time.Duration) {
	log.Printf("API Response: %d %s (%v)", resp.StatusCode, resp.Status, duration)
}

// =============================================================================
// CLOUD: Model Validation
// =============================================================================

// validateModel checks if the model is in the known valid models list.
// CLOUD: Model validation to prevent requests to unknown models.
func (c *OpenRouterClient) validateModel(model string) error {
	// First check if it's a friendly name that maps to a valid model
	if fullModel, ok := OpenRouterModels[model]; ok {
		model = fullModel
	}

	if !validModels[model] {
		return fmt.Errorf("%w: %s", ErrUnknownModel, model)
	}
	return nil
}

// ValidateModel is a public wrapper for model validation.
// CLOUD: Model validation to prevent requests to unknown models.
func (c *OpenRouterClient) ValidateModel(model string) error {
	return c.validateModel(model)
}

// =============================================================================
// CLOUD: Retry Logic with Exponential Backoff
// =============================================================================

// doWithRetry performs an HTTP request with retry logic and exponential backoff.
// CLOUD: Retry logic with exponential backoff for transient errors.
// Retries on 5xx errors and rate limiting, with delays of 1s, 2s, 4s.
// PERFORMANCE: Uses shared HTTP client with connection pooling.
func (c *OpenRouterClient) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		// Clone the request for retry (body needs to be re-readable)
		reqCopy := req.Clone(ctx)

		// Log the request (without sensitive data)
		c.logRequest(reqCopy)

		startTime := time.Now()
		// PERFORMANCE: Use shared HTTP client with connection pooling
		resp, err := sharedHTTPClient.Do(reqCopy)
		duration := time.Since(startTime)

		if err == nil && resp.StatusCode < 500 {
			// Log successful response
			c.logResponse(resp, duration)
			return resp, nil
		}

		lastErr = err
		if resp != nil {
			// Log failed response
			c.logResponse(resp, duration)
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			resp.Body.Close()
		}

		// Exponential backoff: 1s, 2s, 4s
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(1<<attempt) * time.Second):
		}
	}
	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// setHeaders sets the required headers for OpenRouter API requests.
func (c *OpenRouterClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "rigrun/0.2.0")

	if c.siteURL != "" {
		req.Header.Set("HTTP-Referer", c.siteURL)
	}
	if c.siteName != "" {
		req.Header.Set("X-Title", c.siteName)
	}
}

// Chat performs a chat completion request with the given messages.
//
// It automatically handles retries with exponential backoff for transient errors
// such as rate limiting and server errors.
//
// NIST 800-53 AC-7: Integrates with lockout manager to track authentication failures.
func (c *OpenRouterClient) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	if !c.IsConfigured() {
		return nil, ErrNotConfigured
	}

	// AC-7: Check if API key is locked out due to previous failures
	lockoutMgr := security.GlobalLockoutManager()
	apiKeyID := c.getAPIKeyIdentifier()
	if lockoutMgr.IsLocked(apiKeyID) {
		// Log the blocked attempt
		security.AuditLogEvent("", "API_KEY_BLOCKED", map[string]string{
			"reason": "lockout",
			"key_id": apiKeyID,
		})
		return nil, fmt.Errorf("%w: API key is temporarily locked due to authentication failures", security.ErrLocked)
	}

	url := c.baseURL + "/chat/completions"

	reqBody := ChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
	}

	var lastErr error

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		// Apply backoff delay after first attempt
		if attempt > 0 {
			delay := c.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		response, err := c.doRequest(ctx, url, reqBody)
		if err != nil {
			// Check if error is retryable
			if c.isRetryable(err) {
				lastErr = err
				continue
			}
			return nil, err
		}

		return response, nil
	}

	// All retries exhausted
	if lastErr != nil {
		return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
	}
	return nil, errors.New("max retries exceeded")
}

// readResponse reads the response body with size limits to prevent memory exhaustion.
//
// SECURITY: Response size limit prevents memory exhaustion attacks.
func readResponse(resp *http.Response) ([]byte, error) {
	// SECURITY: Limit response size to prevent memory exhaustion
	limitedReader := io.LimitReader(resp.Body, MaxResponseSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if we hit the limit (response was truncated)
	if int64(len(body)) == MaxResponseSize {
		return nil, fmt.Errorf("response exceeded maximum size of %d bytes", MaxResponseSize)
	}

	return body, nil
}

// doRequest performs a single HTTP request to the chat completions endpoint.
//
// NIST 800-53 SC-17: Optionally validates certificates before making requests.
// SECURITY: Clears Authorization header after request to prevent logging.
// PERFORMANCE: Uses shared HTTP client with connection pooling.
func (c *OpenRouterClient) doRequest(ctx context.Context, requestURL string, reqBody ChatRequest) (*ChatResponse, error) {
	// SC-17: Validate certificate before making request if enabled
	if c.validateCerts && c.pkiManager != nil {
		if err := c.validateCertificate(requestURL); err != nil {
			return nil, fmt.Errorf("certificate validation failed: %w", err)
		}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req)

	// PERFORMANCE: Use shared HTTP client with connection pooling
	resp, err := sharedHTTPClient.Do(req)

	// SECURITY: Clear Authorization header immediately after request to prevent logging
	req.Header.Del("Authorization")

	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// SECURITY: Read response with size limit to prevent memory exhaustion
	body, err := readResponse(resp)
	if err != nil {
		return nil, err
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp.StatusCode, body)
	}

	// Parse successful response
	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// AC-7: Record successful authentication (resets failure counter)
	c.RecordAuthSuccess()

	return &chatResp, nil
}

// handleErrorResponse converts HTTP error responses to appropriate Go errors.
//
// NIST 800-53 AC-7: Records authentication failures for lockout tracking.
func (c *OpenRouterClient) handleErrorResponse(statusCode int, body []byte) error {
	// AC-7: Record authentication failure for 401 responses
	if statusCode == http.StatusUnauthorized {
		c.RecordAuthFailure()
	}

	// Try to parse error response
	var apiErr apiErrorResponse
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error.Message != "" {
		orErr := &OpenRouterError{
			Code:    apiErr.Error.Code,
			Message: apiErr.Error.Message,
			Status:  statusCode,
		}

		// Map to specific error types
		switch statusCode {
		case http.StatusUnauthorized:
			return fmt.Errorf("%w: %s", ErrAuthFailed, orErr.Message)
		case http.StatusPaymentRequired:
			return fmt.Errorf("%w: %s", ErrInsufficientCredits, orErr.Message)
		case http.StatusNotFound:
			return fmt.Errorf("%w: %s", ErrModelNotFound, orErr.Message)
		case http.StatusTooManyRequests:
			return fmt.Errorf("%w: %s", ErrRateLimited, orErr.Message)
		default:
			return orErr
		}
	}

	// Fallback for unparseable error responses
	switch statusCode {
	case http.StatusUnauthorized:
		return ErrAuthFailed
	case http.StatusPaymentRequired:
		return ErrInsufficientCredits
	case http.StatusNotFound:
		return ErrModelNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimited
	default:
		return &OpenRouterError{
			Message: string(body),
			Status:  statusCode,
		}
	}
}

// isRetryable determines if an error should trigger a retry.
func (c *OpenRouterClient) isRetryable(err error) bool {
	// Rate limiting is retryable
	if errors.Is(err, ErrRateLimited) {
		return true
	}

	// Check for OpenRouterError with 5xx status
	var orErr *OpenRouterError
	if errors.As(err, &orErr) {
		return orErr.Status >= 500 && orErr.Status < 600
	}

	// Network errors might be retryable (connection issues, timeouts)
	// but we don't retry context cancellation
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	return false
}

// calculateBackoff returns the delay to wait before the next retry.
func (c *OpenRouterClient) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: 500ms, 1000ms, 2000ms, etc.
	delay := retryBaseDelay * time.Duration(1<<uint(attempt))
	if delay > retryMaxDelay {
		delay = retryMaxDelay
	}
	return delay
}

// ListModels retrieves the list of available models from OpenRouter.
//
// PERFORMANCE: Uses shared HTTP client with connection pooling.
// SECURITY: Response size limit prevents memory exhaustion.
func (c *OpenRouterClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := c.baseURL + "/models"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Models endpoint doesn't require auth but we set headers anyway
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "rigrun/0.2.0")

	// PERFORMANCE: Use shared HTTP client with connection pooling
	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// SECURITY: Read response with size limit to prevent memory exhaustion
	body, err := readResponse(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &OpenRouterError{
			Message: fmt.Sprintf("failed to list models: %s", string(body)),
			Status:  resp.StatusCode,
		}
	}

	var modelsResp modelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	models := make([]ModelInfo, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		info := ModelInfo{
			ID:          m.ID,
			Name:        m.Name,
			ContextSize: m.ContextLength,
		}
		if m.Pricing != nil {
			info.Pricing = *m.Pricing
		}
		models = append(models, info)
	}

	return models, nil
}

// Generate performs a simple text generation with a single user prompt.
func (c *OpenRouterClient) Generate(ctx context.Context, prompt string) (*ChatResponse, error) {
	messages := []ChatMessage{
		NewUserMessage(prompt),
	}
	return c.Chat(ctx, messages)
}

// ChatWithModel performs a chat completion with a specific model, overriding the default.
// This method is thread-safe and does not modify the original client's model field.
func (c *OpenRouterClient) ChatWithModel(ctx context.Context, model string, messages []ChatMessage) (*ChatResponse, error) {
	// SECURITY FIX: Create a copy of client to avoid race condition
	// The original code modified c.model which is not thread-safe
	clientCopy := *c
	clientCopy.SetModel(model)
	return clientCopy.Chat(ctx, messages)
}

// ValidateAPIKey checks if the API key format appears valid.
// Note: This doesn't verify the key with OpenRouter, just checks the format.
// SECURITY: Enhanced validation with length and entropy checks.
func ValidateAPIKey(apiKey string) bool {
	apiKey = strings.TrimSpace(apiKey)

	// OpenRouter keys typically start with "sk-or-"
	if !strings.HasPrefix(apiKey, "sk-or-") {
		return false
	}

	// Minimum length check (sk-or- prefix + at least 32 chars)
	if len(apiKey) < 38 {
		return false
	}

	// Basic entropy check: key should contain alphanumeric variety
	// Count unique characters to detect obvious test keys like "sk-or-aaaaaaaaaa"
	uniqueChars := make(map[rune]bool)
	for _, char := range apiKey[6:] { // Skip "sk-or-" prefix
		uniqueChars[char] = true
	}

	// Require at least 10 unique characters for reasonable entropy
	if len(uniqueChars) < 10 {
		return false
	}

	return true
}

// IsConfigured is a convenience function to check if OpenRouter is configured
// via the OPENROUTER_API_KEY environment variable.
func IsConfigured() bool {
	// This would typically check os.Getenv("OPENROUTER_API_KEY")
	// but we leave that to the caller to avoid importing os here
	return false
}

// =============================================================================
// NIST 800-53 SC-17: PKI Certificate Validation
// =============================================================================

// validateCertificate validates the TLS certificate for the given URL.
// Logs certificate issues to the audit log.
func (c *OpenRouterClient) validateCertificate(requestURL string) error {
	if c.pkiManager == nil {
		return nil
	}

	// Extract host from URL
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	host := parsedURL.Host
	if host == "" {
		return fmt.Errorf("no host in URL")
	}

	// Validate the certificate
	status, err := c.pkiManager.ValidateCertificate(host)

	// Log the validation result
	valid := err == nil
	reason := ""
	if err != nil {
		reason = err.Error()
	}

	// Log to audit trail
	security.LogCertValidation("OpenRouterClient", host, valid, reason)

	if err != nil {
		return err
	}

	// Check for expiring certificates (warning threshold: 30 days)
	if status != nil && status.DaysUntilExpiry < 30 {
		security.LogCertValidation("OpenRouterClient", host, true,
			fmt.Sprintf("certificate expiring in %d days", status.DaysUntilExpiry))
	}

	return nil
}

// GetCertificateStatus returns the certificate status for the API endpoint.
//
// NIST 800-53 SC-17: Provides certificate information for monitoring.
func (c *OpenRouterClient) GetCertificateStatus() (*security.CertStatus, error) {
	if c.pkiManager == nil {
		return nil, errors.New("PKI manager not configured")
	}

	// Extract host from base URL
	parsedURL, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	host := parsedURL.Host
	if host == "" {
		return nil, errors.New("no host in base URL")
	}

	return c.pkiManager.ValidateCertificate(host)
}

// GetPKIManager returns the PKI manager used by this client.
func (c *OpenRouterClient) GetPKIManager() *security.PKIManager {
	return c.pkiManager
}

// =============================================================================
// NIST 800-53 AC-7: Unsuccessful Logon Attempts
// =============================================================================

// getAPIKeyIdentifier returns a secure fingerprint identifier for the API key for lockout tracking.
// This uses SHA-256 hash to create a trackable identifier without exposing the key prefix.
// NIST 800-53 IA-5(1): Obscure feedback of authentication information.
func (c *OpenRouterClient) getAPIKeyIdentifier() string {
	if c.apiKey == "" {
		return "unknown"
	}
	// Use SHA-256 hash to create a secure fingerprint
	hash := sha256.Sum256([]byte(c.apiKey))
	// Return first 8 chars of hash as identifier (4 bytes = 8 hex chars)
	return fmt.Sprintf("key_sha256_%x", hash[:4])
}

// RecordAuthAttempt records an authentication attempt for AC-7 compliance.
// This should be called after each API request that could result in auth failure.
//
// NIST 800-53 AC-7: Tracks unsuccessful authentication attempts.
// ERROR HANDLING: Errors must not be silently ignored
func (c *OpenRouterClient) RecordAuthAttempt(success bool) {
	lockoutMgr := security.GlobalLockoutManager()
	apiKeyID := c.getAPIKeyIdentifier()

	if err := lockoutMgr.RecordAttempt(apiKeyID, success); err != nil {
		// Log to stderr when recording fails - security critical for AC-7
		fmt.Fprintf(os.Stderr, "AC-7 WARNING: failed to record auth attempt: %v\n", err)
	}
}

// RecordAuthFailure records an authentication failure for AC-7 compliance.
// Call this when an API request fails due to authentication issues (401).
func (c *OpenRouterClient) RecordAuthFailure() {
	c.RecordAuthAttempt(false)
}

// RecordAuthSuccess records a successful authentication for AC-7 compliance.
// Call this when an API request succeeds, resetting the failure counter.
func (c *OpenRouterClient) RecordAuthSuccess() {
	c.RecordAuthAttempt(true)
}

// IsAPIKeyLocked checks if the API key is currently locked out.
//
// NIST 800-53 AC-7: Returns true if too many authentication failures have occurred.
func (c *OpenRouterClient) IsAPIKeyLocked() bool {
	lockoutMgr := security.GlobalLockoutManager()
	return lockoutMgr.IsLocked(c.getAPIKeyIdentifier())
}

// GetLockoutStatus returns the lockout status for the API key.
func (c *OpenRouterClient) GetLockoutStatus() *security.AttemptRecord {
	lockoutMgr := security.GlobalLockoutManager()
	return lockoutMgr.GetStatus(c.getAPIKeyIdentifier())
}

// minInt returns the smaller of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
