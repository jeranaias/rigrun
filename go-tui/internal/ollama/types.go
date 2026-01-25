// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ollama provides the HTTP client for communicating with Ollama API.
package ollama

import "time"

// =============================================================================
// REQUEST TYPES
// =============================================================================

// Message represents a chat message in the conversation.
type Message struct {
	Role      string     `json:"role"`                 // "user", "assistant", "system", "tool"
	Content   string     `json:"content"`              // The message content
	ToolCalls []ToolCall `json:"tool_calls,omitempty"` // Tool calls requested by assistant
}

// ToolCall represents a tool invocation from the model.
type ToolCall struct {
	Function ToolFunction `json:"function"`
}

// ToolFunction contains the function name and arguments.
type ToolFunction struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ChatRequest is the request body for /api/chat endpoint.
type ChatRequest struct {
	Model    string    `json:"model"`              // Model name (e.g., "qwen2.5-coder:14b")
	Messages []Message `json:"messages"`           // Conversation history
	Stream   bool      `json:"stream"`             // Enable streaming (default: true)
	Format   string    `json:"format,omitempty"`   // Response format (e.g., "json")
	Options  *Options  `json:"options,omitempty"`  // Model parameters
	Template string    `json:"template,omitempty"` // Custom prompt template
	Context  []int     `json:"context,omitempty"`  // Previous context for continuations
	Tools    []Tool    `json:"tools,omitempty"`    // Available tools for function calling
}

// Tool represents a tool definition for function calling.
type Tool struct {
	Type     string       `json:"type"`     // Always "function"
	Function ToolSchema   `json:"function"` // Function definition
}

// ToolSchema defines a tool's interface.
type ToolSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

// ToolParameters defines the parameters schema for a tool.
type ToolParameters struct {
	Type       string                    `json:"type"` // "object"
	Properties map[string]ToolProperty   `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

// ToolProperty defines a single parameter property using JSON Schema.
type ToolProperty struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`    // Allowed values for string type
	Default     any      `json:"default,omitempty"` // Default value for the parameter
}

// Options contains model parameters for inference.
type Options struct {
	// Sampling parameters
	Temperature      float64 `json:"temperature,omitempty"`       // 0.0-2.0, default 0.8
	TopK             int     `json:"top_k,omitempty"`             // Default 40
	TopP             float64 `json:"top_p,omitempty"`             // 0.0-1.0, default 0.9
	RepeatPenalty    float64 `json:"repeat_penalty,omitempty"`    // Default 1.1
	PresencePenalty  float64 `json:"presence_penalty,omitempty"`  // Default 0.0
	FrequencyPenalty float64 `json:"frequency_penalty,omitempty"` // Default 0.0

	// Context parameters
	NumCtx    int `json:"num_ctx,omitempty"`    // Context window size, default 2048
	NumPredict int `json:"num_predict,omitempty"` // Max tokens to generate, -1 for unlimited

	// Performance parameters
	NumGPU     int `json:"num_gpu,omitempty"`     // Number of GPU layers to use
	NumThread  int `json:"num_thread,omitempty"`  // Number of threads for inference
	NumBatch   int `json:"num_batch,omitempty"`   // Batch size for prompt processing

	// Stopping
	Stop []string `json:"stop,omitempty"` // Stop sequences

	// Seed for reproducibility
	Seed int `json:"seed,omitempty"` // Random seed
}

// GenerateRequest is the request body for /api/generate endpoint.
type GenerateRequest struct {
	Model   string   `json:"model"`
	Prompt  string   `json:"prompt"`
	Stream  bool     `json:"stream"`
	System  string   `json:"system,omitempty"`
	Options *Options `json:"options,omitempty"`
	Context []int    `json:"context,omitempty"`
	Raw     bool     `json:"raw,omitempty"`
}

// EmbeddingRequest is the request body for /api/embeddings endpoint.
type EmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// =============================================================================
// RESPONSE TYPES
// =============================================================================

// ChatResponse is the response from /api/chat endpoint.
type ChatResponse struct {
	Model              string    `json:"model"`
	CreatedAt          time.Time `json:"created_at"`
	Message            Message   `json:"message"`
	Done               bool      `json:"done"`
	DoneReason         string    `json:"done_reason,omitempty"`
	Context            []int     `json:"context,omitempty"`
	TotalDuration      int64     `json:"total_duration,omitempty"`       // nanoseconds
	LoadDuration       int64     `json:"load_duration,omitempty"`        // nanoseconds
	PromptEvalCount    int       `json:"prompt_eval_count,omitempty"`    // number of tokens in prompt
	PromptEvalDuration int64     `json:"prompt_eval_duration,omitempty"` // nanoseconds
	EvalCount          int       `json:"eval_count,omitempty"`           // number of tokens generated
	EvalDuration       int64     `json:"eval_duration,omitempty"`        // nanoseconds
}

// GenerateResponse is the response from /api/generate endpoint.
type GenerateResponse struct {
	Model              string    `json:"model"`
	CreatedAt          time.Time `json:"created_at"`
	Response           string    `json:"response"`
	Done               bool      `json:"done"`
	DoneReason         string    `json:"done_reason,omitempty"`
	Context            []int     `json:"context,omitempty"`
	TotalDuration      int64     `json:"total_duration,omitempty"`
	LoadDuration       int64     `json:"load_duration,omitempty"`
	PromptEvalCount    int       `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64     `json:"prompt_eval_duration,omitempty"`
	EvalCount          int       `json:"eval_count,omitempty"`
	EvalDuration       int64     `json:"eval_duration,omitempty"`
}

// EmbeddingResponse is the response from /api/embeddings endpoint.
type EmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// =============================================================================
// MODEL TYPES
// =============================================================================

// ModelInfo contains information about a model.
type ModelInfo struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
	Digest     string    `json:"digest"`
	Details    ModelDetails `json:"details,omitempty"`
}

// ModelDetails contains detailed information about a model.
type ModelDetails struct {
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// ListModelsResponse is the response from /api/tags endpoint.
type ListModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

// ShowModelRequest is the request for /api/show endpoint.
type ShowModelRequest struct {
	Name string `json:"name"`
}

// ShowModelResponse is the response from /api/show endpoint.
type ShowModelResponse struct {
	License    string       `json:"license"`
	Modelfile  string       `json:"modelfile"`
	Parameters string       `json:"parameters"`
	Template   string       `json:"template"`
	Details    ModelDetails `json:"details"`
}

// =============================================================================
// STREAMING TYPES
// =============================================================================

// StreamChunk represents a single chunk from streaming response.
type StreamChunk struct {
	// Content from this chunk (for chat: message.content, for generate: response)
	Content string

	// Tool calls requested by the model (populated when model wants to call tools)
	ToolCalls []ToolCall

	// Timing information (only populated on final chunk)
	Done               bool
	DoneReason         string
	TotalDuration      time.Duration
	LoadDuration       time.Duration
	PromptEvalDuration time.Duration
	EvalDuration       time.Duration

	// Token counts (only populated on final chunk)
	PromptTokens     int
	CompletionTokens int

	// Model information
	Model string

	// Error if any occurred during streaming
	Error error
}

// =============================================================================
// ERROR TYPES
// =============================================================================

// OllamaError represents an error from the Ollama API.
type OllamaError struct {
	Error string `json:"error"`
}

// =============================================================================
// HELPER METHODS
// =============================================================================

// NewUserMessage creates a new user message.
func NewUserMessage(content string) Message {
	return Message{Role: "user", Content: content}
}

// NewAssistantMessage creates a new assistant message.
func NewAssistantMessage(content string) Message {
	return Message{Role: "assistant", Content: content}
}

// NewSystemMessage creates a new system message.
func NewSystemMessage(content string) Message {
	return Message{Role: "system", Content: content}
}

// NewToolResultMessage creates a tool result message.
func NewToolResultMessage(content string) Message {
	return Message{Role: "tool", Content: content}
}

// NewAssistantMessageWithTools creates an assistant message with tool calls.
func NewAssistantMessageWithTools(content string, toolCalls []ToolCall) Message {
	return Message{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
	}
}

// HasToolCalls returns true if the message contains tool calls.
func (m *Message) HasToolCalls() bool {
	return len(m.ToolCalls) > 0
}

// TokensPerSecond calculates the generation speed from a response.
func (r *ChatResponse) TokensPerSecond() float64 {
	if r.EvalDuration == 0 {
		return 0
	}
	seconds := float64(r.EvalDuration) / 1e9
	return float64(r.EvalCount) / seconds
}

// TTFT returns the time to first token (prompt evaluation time).
func (r *ChatResponse) TTFT() time.Duration {
	return time.Duration(r.PromptEvalDuration)
}

// TotalTime returns the total generation time.
func (r *ChatResponse) TotalTime() time.Duration {
	return time.Duration(r.TotalDuration)
}

// FormatSize formats the model size in human-readable form.
func (m *ModelInfo) FormatSize() string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case m.Size >= GB:
		return formatFloat(float64(m.Size)/GB) + " GB"
	case m.Size >= MB:
		return formatFloat(float64(m.Size)/MB) + " MB"
	case m.Size >= KB:
		return formatFloat(float64(m.Size)/KB) + " KB"
	default:
		return formatFloat(float64(m.Size)) + " B"
	}
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return string(rune(int64(f) + '0'))
	}
	// Simple formatting without fmt package
	whole := int64(f)
	frac := int64((f - float64(whole)) * 10)
	if frac == 0 {
		return string(rune(whole + '0'))
	}
	return string([]rune{rune(whole + '0'), '.', rune(frac + '0')})
}
