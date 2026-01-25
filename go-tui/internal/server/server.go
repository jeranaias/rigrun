// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package server provides an HTTP API server with OpenAI-compatible endpoints.
//
// Endpoints:
//   - POST /v1/chat/completions - OpenAI-compatible chat completions
//   - GET  /v1/models          - List available models
//   - GET  /health             - Health check
//   - GET  /stats              - Usage statistics
//   - GET  /cache/stats        - Cache statistics
//   - POST /cache/clear        - Clear cache
//
// Supports both streaming and non-streaming responses.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/cache"
	"github.com/jeranaias/rigrun-tui/internal/cloud"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/security"
)

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	// DefaultPort is the default port for the HTTP server.
	DefaultPort = 8787

	// DefaultOllamaTimeout is the timeout for local Ollama requests.
	DefaultOllamaTimeout = 15 * time.Second

	// MaxQueryLength is the maximum length for a query to prevent DoS.
	MaxQueryLength = 100000

	// MaxMessageCount is the maximum number of messages in a request.
	MaxMessageCount = 100

	// MaxRequestBodySize is the maximum size for request body to prevent DoS (1MB).
	MaxRequestBodySize = 1 * 1024 * 1024

	// MaxTokensLimit is the maximum value for max_tokens parameter.
	MaxTokensLimit = 128000

	// MinTemperature is the minimum value for temperature parameter.
	MinTemperature = 0.0

	// MaxTemperature is the maximum value for temperature parameter.
	MaxTemperature = 2.0

	// Version is the server version.
	Version = "0.2.0"
)

// validRoles defines the set of acceptable message roles.
// IL5 SECURITY: Validates message roles to prevent injection attacks.
var validRoles = map[string]bool{
	"user":      true,
	"assistant": true,
	"system":    true,
	"tool":      true,
}

// validateMessages validates that all message roles are acceptable.
// IL5 SECURITY: Prevents role injection attacks by enforcing a whitelist.
// Returns an error if any message has an invalid role.
func validateMessages(messages []ChatMessage) error {
	for i, msg := range messages {
		if !validRoles[msg.Role] {
			return fmt.Errorf("invalid role '%s' at message %d: must be one of user, assistant, system, tool", msg.Role, i)
		}
	}
	return nil
}

// ============================================================================
// SERVER STATS
// ============================================================================

// ServerStats tracks server usage statistics.
type ServerStats struct {
	TotalRequests  int64     `json:"total_requests"`
	CacheHits      int64     `json:"cache_hits"`
	LocalRequests  int64     `json:"local_requests"`
	CloudRequests  int64     `json:"cloud_requests"`
	TotalTokens    int64     `json:"total_tokens"`
	TotalCostCents float64   `json:"total_cost_cents"`
	StartTime      time.Time `json:"start_time"`
	mu             sync.Mutex
}

// NewServerStats creates a new ServerStats instance.
func NewServerStats() *ServerStats {
	return &ServerStats{
		StartTime: time.Now(),
	}
}

// RecordRequest records a new request in the stats.
func (s *ServerStats) RecordRequest(tier router.Tier, tokens int64, costCents float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	atomic.AddInt64(&s.TotalRequests, 1)
	atomic.AddInt64(&s.TotalTokens, tokens)
	s.TotalCostCents += costCents

	switch tier {
	case router.TierCache:
		atomic.AddInt64(&s.CacheHits, 1)
	case router.TierLocal:
		atomic.AddInt64(&s.LocalRequests, 1)
	default:
		atomic.AddInt64(&s.CloudRequests, 1)
	}
}

// GetStats returns a copy of the current stats.
func (s *ServerStats) GetStats() ServerStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	return ServerStats{
		TotalRequests:  atomic.LoadInt64(&s.TotalRequests),
		CacheHits:      atomic.LoadInt64(&s.CacheHits),
		LocalRequests:  atomic.LoadInt64(&s.LocalRequests),
		CloudRequests:  atomic.LoadInt64(&s.CloudRequests),
		TotalTokens:    atomic.LoadInt64(&s.TotalTokens),
		TotalCostCents: s.TotalCostCents,
		StartTime:      s.StartTime,
	}
}

// Uptime returns the server uptime duration.
func (s *ServerStats) Uptime() time.Duration {
	return time.Since(s.StartTime)
}

// ============================================================================
// SERVER
// ============================================================================

// Server is the HTTP API server with OpenAI-compatible endpoints.
type Server struct {
	port   int
	router *http.ServeMux
	server *http.Server

	ollama *ollama.Client
	cloud  *cloud.OpenRouterClient
	cache  *cache.CacheManager
	stats  *ServerStats
	auth   *AuthConfig

	// paranoidMode blocks all cloud requests when true (NIST SC-7 boundary protection)
	paranoidMode bool

	mu sync.RWMutex
}

// NewServer creates a new Server with the specified port.
// If port is 0, the default port (8787) is used.
func NewServer(port int) *Server {
	if port == 0 {
		port = DefaultPort
	}

	s := &Server{
		port:   port,
		router: http.NewServeMux(),
		ollama: ollama.NewClient(),
		cache:  cache.Default(),
		stats:  NewServerStats(),
		auth:   DefaultAuthConfig(),
	}

	s.setupRoutes()
	return s
}

// WithOllamaClient sets a custom Ollama client.
func (s *Server) WithOllamaClient(client *ollama.Client) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ollama = client
	return s
}

// WithCloudClient sets the OpenRouter cloud client.
func (s *Server) WithCloudClient(client *cloud.OpenRouterClient) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cloud = client
	return s
}

// WithCache sets a custom cache manager.
func (s *Server) WithCache(cm *cache.CacheManager) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = cm
	return s
}

// WithAuth sets the authentication configuration.
func (s *Server) WithAuth(config *AuthConfig) *Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auth = config
	return s
}

// Port returns the server port.
func (s *Server) Port() int {
	return s.port
}

// ============================================================================
// ROUTES
// ============================================================================

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() {
	// OpenAI-compatible endpoints
	s.router.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)
	s.router.HandleFunc("GET /v1/models", s.handleModels)

	// Health and stats endpoints
	s.router.HandleFunc("GET /health", s.handleHealth)
	s.router.HandleFunc("GET /stats", s.handleStats)

	// Cache management endpoints
	s.router.HandleFunc("GET /cache/stats", s.handleCacheStats)
	s.router.HandleFunc("POST /cache/clear", s.handleCacheClear)
}

// ============================================================================
// OPENAI-COMPATIBLE TYPES
// ============================================================================

// ChatMessage represents a message in the chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest is the OpenAI-compatible chat completion request.
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatChoice represents a single choice in the completion response.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Usage contains token usage information.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionResponse is the OpenAI-compatible chat completion response.
type ChatCompletionResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   Usage        `json:"usage"`
}

// StreamChunk represents a single chunk in a streaming response.
type StreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// ============================================================================
// CHAT COMPLETIONS HANDLER
// ============================================================================

// handleChatCompletions handles POST /v1/chat/completions.
func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	// CRITICAL IL5 FIX: Limit request body size to prevent DoS attacks
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)

	// Parse request
	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Check if error is due to request body too large
		if err.Error() == "http: request body too large" {
			s.writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("Request body exceeds maximum size of %d bytes", MaxRequestBodySize))
			return
		}
		// IL5 SECURITY: Log full details internally, return generic message to client
		log.Printf("Invalid request body: %v", err)
		s.writeError(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Validate request
	if len(req.Messages) == 0 {
		s.writeError(w, http.StatusBadRequest, "Request must contain at least one message")
		return
	}

	// IL5 SECURITY: Validate message roles
	if err := validateMessages(req.Messages); err != nil {
		// IL5 SECURITY: Log full validation error internally
		log.Printf("Message validation failed: %v", err)
		s.writeError(w, http.StatusBadRequest, "Invalid message format. Messages must have valid roles (user, assistant, system, tool)")
		return
	}

	if len(req.Messages) > MaxMessageCount {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Too many messages: maximum is %d", MaxMessageCount))
		return
	}

	for i, msg := range req.Messages {
		if len(msg.Content) > MaxQueryLength {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Message %d exceeds maximum length of %d", i, MaxQueryLength))
			return
		}
	}

	// CRITICAL IL5 FIX: Validate MaxTokens parameter
	if req.MaxTokens < 0 || req.MaxTokens > MaxTokensLimit {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("max_tokens must be between 1 and %d", MaxTokensLimit))
		return
	}

	// CRITICAL IL5 FIX: Validate Temperature parameter
	if req.Temperature < MinTemperature || req.Temperature > MaxTemperature {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("temperature must be between %.1f and %.1f", MinTemperature, MaxTemperature))
		return
	}

	// Handle streaming vs non-streaming
	if req.Stream {
		s.handleStreamingCompletion(w, r, req)
	} else {
		s.handleNonStreamingCompletion(w, r, req)
	}
}

// handleNonStreamingCompletion handles non-streaming chat completions.
func (s *Server) handleNonStreamingCompletion(w http.ResponseWriter, r *http.Request, req ChatCompletionRequest) {
	startTime := time.Now()
	ctx := r.Context()

	// Extract the last user message for cache lookup
	var cacheKey string
	if len(req.Messages) > 0 {
		cacheKey = req.Messages[len(req.Messages)-1].Content
	}

	// Check cache first
	s.mu.RLock()
	cacheManager := s.cache
	s.mu.RUnlock()

	if cacheManager != nil {
		if response, hitType := cacheManager.Lookup(cacheKey); hitType != cache.CacheHitNone {
			// Cache hit
			latencyMs := time.Since(startTime).Milliseconds()
			log.Printf("CACHE_HIT | type=%s query=%s latency=%dms", hitType.String(), truncateString(cacheKey, 50), latencyMs)

			s.stats.RecordRequest(router.TierCache, 0, 0)

			s.writeJSON(w, http.StatusOK, ChatCompletionResponse{
				ID:      generateResponseID(),
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   "cache",
				Choices: []ChatChoice{
					{
						Index: 0,
						Message: ChatMessage{
							Role:    "assistant",
							Content: response,
						},
						FinishReason: "stop",
					},
				},
				Usage: Usage{
					PromptTokens:     0,
					CompletionTokens: 0,
					TotalTokens:      0,
				},
			})
			return
		}
	}

	// Determine which tier to use
	tier := s.routeRequest(req.Model, cacheKey)

	// Execute the request based on tier
	var responseText string
	var promptTokens, completionTokens int
	var err error

	switch {
	case tier == router.TierLocal || tier == router.TierCache:
		responseText, promptTokens, completionTokens, err = s.executeLocalRequest(ctx, req)
		if err != nil {
			// Fall back to cloud if available
			s.mu.RLock()
			cloudClient := s.cloud
			s.mu.RUnlock()

			if cloudClient != nil && cloudClient.IsConfigured() {
				log.Printf("LOCAL_FALLBACK | error=%v falling_back_to_cloud", err)
				responseText, promptTokens, completionTokens, err = s.executeCloudRequest(ctx, req)
				tier = router.TierCloud
			}
		}

	default:
		// Cloud tiers
		responseText, promptTokens, completionTokens, err = s.executeCloudRequest(ctx, req)
	}

	if err != nil {
		// IL5 SECURITY: Log full details internally, return generic message to client
		log.Printf("REQUEST_ERROR | tier=%s error=%v", tier.String(), err)
		s.writeError(w, http.StatusInternalServerError, "Request processing failed. Please try again.")
		return
	}

	// Store in cache
	if cacheManager != nil && responseText != "" {
		cacheManager.Store(cacheKey, responseText, tier.String())
	}

	// Record stats
	totalTokens := promptTokens + completionTokens
	costCents := tier.CalculateCostCents(uint32(promptTokens), uint32(completionTokens))
	s.stats.RecordRequest(tier, int64(totalTokens), costCents)

	latencyMs := time.Since(startTime).Milliseconds()
	log.Printf("REQUEST_COMPLETE | tier=%s tokens=%d cost=%.4fc latency=%dms", tier.String(), totalTokens, costCents, latencyMs)

	// Return response
	s.writeJSON(w, http.StatusOK, ChatCompletionResponse{
		ID:      generateResponseID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []ChatChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: responseText,
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
		},
	})
}

// handleStreamingCompletion handles streaming chat completions.
func (s *Server) handleStreamingCompletion(w http.ResponseWriter, r *http.Request, req ChatCompletionRequest) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	ctx := r.Context()
	responseID := generateResponseID()
	created := time.Now().Unix()

	// Determine tier
	var cacheKey string
	if len(req.Messages) > 0 {
		cacheKey = req.Messages[len(req.Messages)-1].Content
	}
	tier := s.routeRequest(req.Model, cacheKey)

	// Convert messages to Ollama format
	ollamaMessages := make([]ollama.Message, len(req.Messages))
	for i, msg := range req.Messages {
		ollamaMessages[i] = ollama.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Stream from appropriate tier
	// PERFORMANCE: strings.Builder avoids quadratic allocations
	var totalContent strings.Builder
	var totalTokens int

	if tier == router.TierLocal || tier == router.TierCache {
		s.mu.RLock()
		ollamaClient := s.ollama
		s.mu.RUnlock()

		if ollamaClient != nil {
			model := ollamaClient.GetDefaultModel()

			// Send initial role chunk
			s.sendStreamChunk(w, flusher, StreamChunk{
				ID:      responseID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []struct {
					Index int `json:"index"`
					Delta struct {
						Role    string `json:"role,omitempty"`
						Content string `json:"content,omitempty"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				}{
					{
						Index: 0,
						Delta: struct {
							Role    string `json:"role,omitempty"`
							Content string `json:"content,omitempty"`
						}{
							Role: "assistant",
						},
						FinishReason: nil,
					},
				},
			})

			// Stream tokens
			err := ollamaClient.ChatStream(ctx, model, ollamaMessages, func(chunk ollama.StreamChunk) {
				if chunk.Error != nil {
					return
				}

				if chunk.Content != "" {
					totalContent.WriteString(chunk.Content)

					s.sendStreamChunk(w, flusher, StreamChunk{
						ID:      responseID,
						Object:  "chat.completion.chunk",
						Created: created,
						Model:   req.Model,
						Choices: []struct {
							Index int `json:"index"`
							Delta struct {
								Role    string `json:"role,omitempty"`
								Content string `json:"content,omitempty"`
							} `json:"delta"`
							FinishReason *string `json:"finish_reason"`
						}{
							{
								Index: 0,
								Delta: struct {
									Role    string `json:"role,omitempty"`
									Content string `json:"content,omitempty"`
								}{
									Content: chunk.Content,
								},
								FinishReason: nil,
							},
						},
					})
				}

				if chunk.Done {
					totalTokens = chunk.CompletionTokens
				}
			})

			if err != nil {
				log.Printf("STREAM_ERROR | tier=local error=%v", err)
			}
		}
	} else {
		// Cloud streaming not implemented - send error
		log.Printf("STREAM_NOT_SUPPORTED | tier=%s", tier.String())
		s.sendStreamChunk(w, flusher, StreamChunk{
			ID:      responseID,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   req.Model,
			Choices: []struct {
				Index int `json:"index"`
				Delta struct {
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			}{
				{
					Index: 0,
					Delta: struct {
						Role    string `json:"role,omitempty"`
						Content string `json:"content,omitempty"`
					}{
						Content: "Streaming is only supported for local models.",
					},
					FinishReason: nil,
				},
			},
		})
	}

	// Send final chunk with finish_reason
	finishReason := "stop"
	s.sendStreamChunk(w, flusher, StreamChunk{
		ID:      responseID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   req.Model,
		Choices: []struct {
			Index int `json:"index"`
			Delta struct {
				Role    string `json:"role,omitempty"`
				Content string `json:"content,omitempty"`
			} `json:"delta"`
			FinishReason *string `json:"finish_reason"`
		}{
			{
				Index: 0,
				Delta: struct {
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
				}{},
				FinishReason: &finishReason,
			},
		},
	})

	// Send [DONE] marker
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()

	// Record stats
	s.stats.RecordRequest(tier, int64(totalTokens), 0)

	// Store in cache
	s.mu.RLock()
	cacheManager := s.cache
	s.mu.RUnlock()

	if cacheManager != nil && totalContent.Len() > 0 {
		cacheManager.Store(cacheKey, totalContent.String(), tier.String())
	}
}

// sendStreamChunk sends a single SSE chunk.
func (s *Server) sendStreamChunk(w http.ResponseWriter, flusher http.Flusher, chunk StreamChunk) {
	data, err := json.Marshal(chunk)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// streamResponse sends chunks over SSE (generic helper).
func (s *Server) streamResponse(w http.ResponseWriter, chunks <-chan StreamChunk) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	for chunk := range chunks {
		s.sendStreamChunk(w, flusher, chunk)
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// ============================================================================
// REQUEST EXECUTION
// ============================================================================

// routeRequest determines which tier to use for a request.
// CRITICAL FIX: Include classification (default Unclassified for API) and paranoid_mode
// Classification enforcement ensures CUI+ data stays on-premise (NIST AC-4)
func (s *Server) routeRequest(model, query string) router.Tier {
	switch model {
	case "auto", "":
		// Default to Unclassified for API requests (TUI has its own classification handling)
		// Pass server's paranoid mode setting
		return router.RouteQuery(
			query,
			security.ClassificationUnclassified, // API defaults to UNCLASSIFIED
			s.paranoidMode,
			nil, // No tier limit
		)
	case "local", "cache":
		return router.TierLocal
	case "cloud", "haiku", "sonnet", "opus", "gpt4", "gpt4o":
		return router.TierCloud
	default:
		return router.TierLocal
	}
}

// executeLocalRequest executes a request using the local Ollama client.
func (s *Server) executeLocalRequest(ctx context.Context, req ChatCompletionRequest) (string, int, int, error) {
	s.mu.RLock()
	ollamaClient := s.ollama
	s.mu.RUnlock()

	if ollamaClient == nil {
		// IL5 SECURITY: Generic error message to avoid exposing configuration details
		return "", 0, 0, fmt.Errorf("local inference unavailable")
	}

	// Convert messages to Ollama format
	ollamaMessages := make([]ollama.Message, len(req.Messages))
	for i, msg := range req.Messages {
		ollamaMessages[i] = ollama.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, DefaultOllamaTimeout)
	defer cancel()

	// Execute chat request
	model := ollamaClient.GetDefaultModel()
	resp, err := ollamaClient.Chat(timeoutCtx, model, ollamaMessages)
	if err != nil {
		return "", 0, 0, err
	}

	return resp.Message.Content, resp.PromptEvalCount, resp.EvalCount, nil
}

// executeCloudRequest executes a request using the OpenRouter cloud client.
func (s *Server) executeCloudRequest(ctx context.Context, req ChatCompletionRequest) (string, int, int, error) {
	s.mu.RLock()
	cloudClient := s.cloud
	s.mu.RUnlock()

	if cloudClient == nil || !cloudClient.IsConfigured() {
		// IL5 SECURITY: Generic error message to avoid exposing configuration details
		return "", 0, 0, fmt.Errorf("cloud inference unavailable")
	}

	// Convert messages to cloud format
	cloudMessages := make([]cloud.ChatMessage, len(req.Messages))
	for i, msg := range req.Messages {
		cloudMessages[i] = cloud.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Execute chat request
	resp, err := cloudClient.Chat(ctx, cloudMessages)
	if err != nil {
		return "", 0, 0, err
	}

	return resp.GetContent(), resp.Usage.PromptTokens, resp.Usage.CompletionTokens, nil
}

// ============================================================================
// MODELS HANDLER
// ============================================================================

// ModelInfo represents information about an available model.
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelsResponse is the OpenAI-compatible models list response.
type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

// handleModels handles GET /v1/models.
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	models := []ModelInfo{
		{ID: "auto", Object: "model", Created: 0, OwnedBy: "rigrun"},
		{ID: "local", Object: "model", Created: 0, OwnedBy: "ollama"},
		{ID: "cache", Object: "model", Created: 0, OwnedBy: "rigrun"},
		{ID: "cloud", Object: "model", Created: 0, OwnedBy: "openrouter"},
		{ID: "haiku", Object: "model", Created: 0, OwnedBy: "anthropic"},
		{ID: "sonnet", Object: "model", Created: 0, OwnedBy: "anthropic"},
		{ID: "opus", Object: "model", Created: 0, OwnedBy: "anthropic"},
	}

	// Add local models from Ollama if available
	s.mu.RLock()
	ollamaClient := s.ollama
	s.mu.RUnlock()

	if ollamaClient != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		localModels, err := ollamaClient.ListModels(ctx)
		if err == nil {
			for _, m := range localModels {
				models = append(models, ModelInfo{
					ID:      m.Name,
					Object:  "model",
					Created: m.ModifiedAt.Unix(),
					OwnedBy: "ollama",
				})
			}
		}
	}

	s.writeJSON(w, http.StatusOK, ModelsResponse{
		Object: "list",
		Data:   models,
	})
}

// ============================================================================
// HEALTH HANDLER
// ============================================================================

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status       string  `json:"status"`
	Version      string  `json:"version"`
	OllamaStatus string  `json:"ollama_status"`
	CloudStatus  string  `json:"cloud_status"`
	CacheEnabled bool    `json:"cache_enabled"`
	CacheEntries int     `json:"cache_entries"`
	CacheHitRate float64 `json:"cache_hit_rate"`
}

// handleHealth handles GET /health.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := HealthResponse{
		Status:  "ok",
		Version: Version,
	}

	// Check Ollama status
	s.mu.RLock()
	ollamaClient := s.ollama
	cloudClient := s.cloud
	cacheManager := s.cache
	s.mu.RUnlock()

	if ollamaClient != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := ollamaClient.CheckRunning(ctx); err == nil {
			health.OllamaStatus = "ok"
		} else {
			health.OllamaStatus = "unavailable"
			health.Status = "degraded"
		}
	} else {
		health.OllamaStatus = "not_configured"
	}

	// Check cloud status
	if cloudClient != nil && cloudClient.IsConfigured() {
		health.CloudStatus = "configured"
	} else {
		health.CloudStatus = "not_configured"
	}

	// Check cache status
	if cacheManager != nil {
		health.CacheEnabled = cacheManager.IsEnabled()
		health.CacheEntries = cacheManager.ExactCacheSize()
		health.CacheHitRate = cacheManager.HitRate()
	}

	s.writeJSON(w, http.StatusOK, health)
}

// ============================================================================
// STATS HANDLER
// ============================================================================

// StatsResponse represents the usage statistics response.
type StatsResponse struct {
	TotalRequests  int64   `json:"total_requests"`
	CacheHits      int64   `json:"cache_hits"`
	LocalRequests  int64   `json:"local_requests"`
	CloudRequests  int64   `json:"cloud_requests"`
	TotalTokens    int64   `json:"total_tokens"`
	TotalCostCents float64 `json:"total_cost_cents"`
	UptimeSeconds  int64   `json:"uptime_seconds"`
	CacheHitRate   float64 `json:"cache_hit_rate"`
	CostSavings    float64 `json:"cost_savings_cents"`
}

// handleStats handles GET /stats.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.stats.GetStats()

	// Calculate cache hit rate
	var cacheHitRate float64
	if stats.TotalRequests > 0 {
		cacheHitRate = float64(stats.CacheHits) / float64(stats.TotalRequests) * 100
	}

	// Estimate cost savings (cache hits + local requests vs if all were cloud)
	// Using Opus pricing as baseline: $15/M input + $75/M output
	estimatedOpusCost := float64(stats.TotalTokens) * 0.001 * (1.5 + 7.5) / 2 // average of input/output
	costSavings := estimatedOpusCost - stats.TotalCostCents

	s.writeJSON(w, http.StatusOK, StatsResponse{
		TotalRequests:  stats.TotalRequests,
		CacheHits:      stats.CacheHits,
		LocalRequests:  stats.LocalRequests,
		CloudRequests:  stats.CloudRequests,
		TotalTokens:    stats.TotalTokens,
		TotalCostCents: stats.TotalCostCents,
		UptimeSeconds:  int64(stats.Uptime().Seconds()),
		CacheHitRate:   cacheHitRate,
		CostSavings:    costSavings,
	})
}

// ============================================================================
// CACHE HANDLERS
// ============================================================================

// CacheStatsResponse represents the cache statistics response.
type CacheStatsResponse struct {
	Enabled         bool    `json:"enabled"`
	ExactEntries    int     `json:"exact_entries"`
	SemanticEntries int     `json:"semantic_entries"`
	ExactHits       int     `json:"exact_hits"`
	SemanticHits    int     `json:"semantic_hits"`
	Misses          int     `json:"misses"`
	TotalLookups    int     `json:"total_lookups"`
	HitRate         float64 `json:"hit_rate_percent"`
}

// handleCacheStats handles GET /cache/stats.
func (s *Server) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cacheManager := s.cache
	s.mu.RUnlock()

	if cacheManager == nil {
		s.writeJSON(w, http.StatusOK, CacheStatsResponse{
			Enabled: false,
		})
		return
	}

	stats := cacheManager.Stats()
	s.writeJSON(w, http.StatusOK, CacheStatsResponse{
		Enabled:         cacheManager.IsEnabled(),
		ExactEntries:    cacheManager.ExactCacheSize(),
		SemanticEntries: cacheManager.SemanticCacheSize(),
		ExactHits:       stats.ExactHits,
		SemanticHits:    stats.SemanticHits,
		Misses:          stats.Misses,
		TotalLookups:    stats.TotalLookups,
		HitRate:         cacheManager.HitRate() * 100,
	})
}

// handleCacheClear handles POST /cache/clear.
func (s *Server) handleCacheClear(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cacheManager := s.cache
	s.mu.RUnlock()

	if cacheManager == nil {
		s.writeJSON(w, http.StatusOK, map[string]string{
			"status":  "error",
			"message": "Cache not configured",
		})
		return
	}

	cacheManager.Clear()
	log.Printf("CACHE_CLEARED | client_ip=%s", GetClientIP(r))

	s.writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "Cache cleared successfully",
	})
}

// ============================================================================
// SERVER LIFECYCLE
// ============================================================================

// Start starts the HTTP server.
func (s *Server) Start() error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)

	// Build middleware chain
	handler := Chain(
		RecoveryMiddleware(),
		SecurityHeadersMiddleware(),
		LoggingMiddleware(log.Default()),
		RateLimitMiddleware(DefaultRateLimiter()),
	)(s.router)

	// Apply auth middleware if enabled
	if s.auth != nil && s.auth.Enabled {
		handler = AuthMiddleware(s.auth)(handler)
	}

	s.server = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("SERVER_START | addr=%s version=%s", addr, Version)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	log.Printf("SERVER_SHUTDOWN | starting graceful shutdown")

	// Save cache before shutdown
	s.mu.RLock()
	cacheManager := s.cache
	s.mu.RUnlock()

	if cacheManager != nil {
		// Note: Cache persistence disabled - would require file I/O on ExactCache
		log.Printf("CACHE_STATS | entries=%d", cacheManager.ExactCacheSize())
	}

	return s.server.Shutdown(ctx)
}

// ============================================================================
// HELPERS
// ============================================================================

// writeJSON writes a JSON response.
func (s *Server) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    "invalid_request_error",
			"code":    status,
		},
	})
}

// generateResponseID generates a unique response ID.
func generateResponseID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "chatcmpl-" + hex.EncodeToString(bytes)
}

// truncateString truncates a string to the specified length.
// Uses rune-based truncation to handle Unicode correctly.
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
