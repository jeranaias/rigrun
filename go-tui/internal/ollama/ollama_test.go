// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ollama provides the HTTP client for communicating with Ollama API.
package ollama

import (
	"testing"
	"time"
)

// =============================================================================
// MESSAGE TESTS
// =============================================================================

func TestNewUserMessage(t *testing.T) {
	msg := NewUserMessage("Hello")

	if msg.Role != "user" {
		t.Errorf("Role = %q, want 'user'", msg.Role)
	}

	if msg.Content != "Hello" {
		t.Errorf("Content = %q, want 'Hello'", msg.Content)
	}
}

func TestNewAssistantMessage(t *testing.T) {
	msg := NewAssistantMessage("Response")

	if msg.Role != "assistant" {
		t.Errorf("Role = %q, want 'assistant'", msg.Role)
	}

	if msg.Content != "Response" {
		t.Errorf("Content = %q, want 'Response'", msg.Content)
	}
}

func TestNewSystemMessage(t *testing.T) {
	msg := NewSystemMessage("You are a helpful assistant")

	if msg.Role != "system" {
		t.Errorf("Role = %q, want 'system'", msg.Role)
	}

	if msg.Content != "You are a helpful assistant" {
		t.Errorf("Content = %q", msg.Content)
	}
}

func TestNewToolResultMessage(t *testing.T) {
	msg := NewToolResultMessage("Tool output")

	if msg.Role != "tool" {
		t.Errorf("Role = %q, want 'tool'", msg.Role)
	}

	if msg.Content != "Tool output" {
		t.Errorf("Content = %q, want 'Tool output'", msg.Content)
	}
}

func TestNewAssistantMessageWithTools(t *testing.T) {
	toolCalls := []ToolCall{
		{
			Function: ToolFunction{
				Name:      "search",
				Arguments: map[string]interface{}{"query": "test"},
			},
		},
	}

	msg := NewAssistantMessageWithTools("Using tool", toolCalls)

	if msg.Role != "assistant" {
		t.Errorf("Role = %q, want 'assistant'", msg.Role)
	}

	if msg.Content != "Using tool" {
		t.Errorf("Content = %q, want 'Using tool'", msg.Content)
	}

	if len(msg.ToolCalls) != 1 {
		t.Errorf("ToolCalls length = %d, want 1", len(msg.ToolCalls))
	}
}

func TestMessage_HasToolCalls(t *testing.T) {
	// Without tool calls
	msg := NewAssistantMessage("Response")
	if msg.HasToolCalls() {
		t.Error("HasToolCalls should be false without tool calls")
	}

	// With tool calls
	msg = NewAssistantMessageWithTools("", []ToolCall{{Function: ToolFunction{Name: "test"}}})
	if !msg.HasToolCalls() {
		t.Error("HasToolCalls should be true with tool calls")
	}
}

// =============================================================================
// CHAT RESPONSE TESTS
// =============================================================================

func TestChatResponse_TokensPerSecond(t *testing.T) {
	tests := []struct {
		name         string
		evalCount    int
		evalDuration int64
		want         float64
	}{
		{"normal", 100, int64(time.Second), 100.0},
		{"zero duration", 100, 0, 0.0},
		{"fast", 1000, int64(100 * time.Millisecond), 10000.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := &ChatResponse{
				EvalCount:    tc.evalCount,
				EvalDuration: tc.evalDuration,
			}

			got := resp.TokensPerSecond()

			// Allow small floating point differences
			if tc.want != 0 && (got < tc.want*0.99 || got > tc.want*1.01) {
				t.Errorf("TokensPerSecond() = %f, want %f", got, tc.want)
			}
			if tc.want == 0 && got != 0 {
				t.Errorf("TokensPerSecond() = %f, want 0", got)
			}
		})
	}
}

func TestChatResponse_TTFT(t *testing.T) {
	resp := &ChatResponse{
		PromptEvalDuration: int64(500 * time.Millisecond),
	}

	ttft := resp.TTFT()

	if ttft != 500*time.Millisecond {
		t.Errorf("TTFT() = %v, want 500ms", ttft)
	}
}

func TestChatResponse_TotalTime(t *testing.T) {
	resp := &ChatResponse{
		TotalDuration: int64(2 * time.Second),
	}

	total := resp.TotalTime()

	if total != 2*time.Second {
		t.Errorf("TotalTime() = %v, want 2s", total)
	}
}

// =============================================================================
// MODEL INFO TESTS
// =============================================================================

func TestModelInfo_FormatSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0 B"},
		{512, "0.5 KB"},
		{1024, "1 KB"},
		{1024 * 1024, "1 MB"},
		{1024 * 1024 * 1024, "1 GB"},
		{2 * 1024 * 1024 * 1024, "2 GB"},
	}

	for _, tc := range tests {
		m := &ModelInfo{Size: tc.size}
		got := m.FormatSize()

		// Just check the unit is correct
		if tc.size < 1024 && got[len(got)-1] != 'B' {
			t.Errorf("FormatSize(%d) = %q, expected bytes", tc.size, got)
		}
		if tc.size >= 1024*1024*1024 && got[len(got)-2:] != "GB" {
			t.Errorf("FormatSize(%d) = %q, expected GB", tc.size, got)
		}
	}
}

// =============================================================================
// REQUEST TYPE TESTS
// =============================================================================

func TestChatRequest_Fields(t *testing.T) {
	req := ChatRequest{
		Model: "qwen2.5:14b",
		Messages: []Message{
			NewSystemMessage("Be helpful"),
			NewUserMessage("Hello"),
		},
		Stream: true,
		Options: &Options{
			Temperature: 0.7,
			NumCtx:      4096,
		},
	}

	if req.Model != "qwen2.5:14b" {
		t.Errorf("Model = %q", req.Model)
	}

	if len(req.Messages) != 2 {
		t.Errorf("Messages length = %d, want 2", len(req.Messages))
	}

	if !req.Stream {
		t.Error("Stream should be true")
	}

	if req.Options.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", req.Options.Temperature)
	}
}

func TestGenerateRequest_Fields(t *testing.T) {
	req := GenerateRequest{
		Model:  "llama3",
		Prompt: "Hello",
		Stream: true,
		System: "Be helpful",
		Options: &Options{
			Temperature: 0.5,
		},
	}

	if req.Model != "llama3" {
		t.Errorf("Model = %q", req.Model)
	}

	if req.Prompt != "Hello" {
		t.Errorf("Prompt = %q", req.Prompt)
	}

	if req.System != "Be helpful" {
		t.Errorf("System = %q", req.System)
	}
}

func TestEmbeddingRequest_Fields(t *testing.T) {
	req := EmbeddingRequest{
		Model:  "nomic-embed-text",
		Prompt: "Hello world",
	}

	if req.Model != "nomic-embed-text" {
		t.Errorf("Model = %q", req.Model)
	}

	if req.Prompt != "Hello world" {
		t.Errorf("Prompt = %q", req.Prompt)
	}
}

// =============================================================================
// OPTIONS TESTS
// =============================================================================

func TestOptions_Fields(t *testing.T) {
	opts := Options{
		Temperature:      0.8,
		TopK:             40,
		TopP:             0.9,
		RepeatPenalty:    1.1,
		PresencePenalty:  0.0,
		FrequencyPenalty: 0.0,
		NumCtx:           4096,
		NumPredict:       2048,
		NumGPU:           1,
		NumThread:        4,
		NumBatch:         512,
		Stop:             []string{"\n\n"},
		Seed:             42,
	}

	if opts.Temperature != 0.8 {
		t.Errorf("Temperature = %f", opts.Temperature)
	}

	if opts.NumCtx != 4096 {
		t.Errorf("NumCtx = %d", opts.NumCtx)
	}

	if opts.Seed != 42 {
		t.Errorf("Seed = %d", opts.Seed)
	}

	if len(opts.Stop) != 1 {
		t.Errorf("Stop length = %d", len(opts.Stop))
	}
}

// =============================================================================
// TOOL DEFINITION TESTS
// =============================================================================

func TestTool_Definition(t *testing.T) {
	tool := Tool{
		Type: "function",
		Function: ToolSchema{
			Name:        "search",
			Description: "Search the web",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"query": {
						Type:        "string",
						Description: "Search query",
					},
					"limit": {
						Type:        "integer",
						Description: "Max results",
						Default:     10,
					},
				},
				Required: []string{"query"},
			},
		},
	}

	if tool.Type != "function" {
		t.Errorf("Type = %q", tool.Type)
	}

	if tool.Function.Name != "search" {
		t.Errorf("Name = %q", tool.Function.Name)
	}

	if len(tool.Function.Parameters.Properties) != 2 {
		t.Errorf("Properties length = %d", len(tool.Function.Parameters.Properties))
	}

	if len(tool.Function.Parameters.Required) != 1 {
		t.Errorf("Required length = %d", len(tool.Function.Parameters.Required))
	}
}

func TestToolCall_Fields(t *testing.T) {
	tc := ToolCall{
		Function: ToolFunction{
			Name: "search",
			Arguments: map[string]interface{}{
				"query": "test query",
				"limit": 5,
			},
		},
	}

	if tc.Function.Name != "search" {
		t.Errorf("Function.Name = %q", tc.Function.Name)
	}

	if tc.Function.Arguments["query"] != "test query" {
		t.Errorf("Arguments['query'] = %v", tc.Function.Arguments["query"])
	}
}

// =============================================================================
// RESPONSE TYPE TESTS
// =============================================================================

func TestChatResponse_Fields(t *testing.T) {
	now := time.Now()
	resp := ChatResponse{
		Model:              "qwen2.5:14b",
		CreatedAt:          now,
		Message:            NewAssistantMessage("Hello!"),
		Done:               true,
		DoneReason:         "stop",
		TotalDuration:      int64(time.Second),
		PromptEvalCount:    10,
		PromptEvalDuration: int64(100 * time.Millisecond),
		EvalCount:          50,
		EvalDuration:       int64(900 * time.Millisecond),
	}

	if resp.Model != "qwen2.5:14b" {
		t.Errorf("Model = %q", resp.Model)
	}

	if !resp.Done {
		t.Error("Done should be true")
	}

	if resp.DoneReason != "stop" {
		t.Errorf("DoneReason = %q", resp.DoneReason)
	}

	if resp.PromptEvalCount != 10 {
		t.Errorf("PromptEvalCount = %d", resp.PromptEvalCount)
	}

	if resp.EvalCount != 50 {
		t.Errorf("EvalCount = %d", resp.EvalCount)
	}
}

func TestGenerateResponse_Fields(t *testing.T) {
	resp := GenerateResponse{
		Model:    "llama3",
		Response: "Generated text",
		Done:     true,
	}

	if resp.Model != "llama3" {
		t.Errorf("Model = %q", resp.Model)
	}

	if resp.Response != "Generated text" {
		t.Errorf("Response = %q", resp.Response)
	}

	if !resp.Done {
		t.Error("Done should be true")
	}
}

func TestEmbeddingResponse_Fields(t *testing.T) {
	resp := EmbeddingResponse{
		Embedding: []float64{0.1, 0.2, 0.3, 0.4, 0.5},
	}

	if len(resp.Embedding) != 5 {
		t.Errorf("Embedding length = %d, want 5", len(resp.Embedding))
	}

	if resp.Embedding[0] != 0.1 {
		t.Errorf("Embedding[0] = %f, want 0.1", resp.Embedding[0])
	}
}

// =============================================================================
// STREAMING TYPES TESTS
// =============================================================================

func TestStreamChunk_Fields(t *testing.T) {
	chunk := StreamChunk{
		Content:            "Hello",
		Done:               false,
		Model:              "qwen2.5:14b",
		TotalDuration:      time.Second,
		LoadDuration:       100 * time.Millisecond,
		PromptEvalDuration: 200 * time.Millisecond,
		EvalDuration:       700 * time.Millisecond,
		PromptTokens:       10,
		CompletionTokens:   20,
	}

	if chunk.Content != "Hello" {
		t.Errorf("Content = %q", chunk.Content)
	}

	if chunk.Done {
		t.Error("Done should be false")
	}

	if chunk.Model != "qwen2.5:14b" {
		t.Errorf("Model = %q", chunk.Model)
	}

	if chunk.PromptTokens != 10 {
		t.Errorf("PromptTokens = %d", chunk.PromptTokens)
	}

	if chunk.CompletionTokens != 20 {
		t.Errorf("CompletionTokens = %d", chunk.CompletionTokens)
	}
}

func TestStreamChunk_WithToolCalls(t *testing.T) {
	chunk := StreamChunk{
		Content: "",
		ToolCalls: []ToolCall{
			{Function: ToolFunction{Name: "search"}},
		},
		Done: true,
	}

	if len(chunk.ToolCalls) != 1 {
		t.Errorf("ToolCalls length = %d, want 1", len(chunk.ToolCalls))
	}
}

func TestStreamChunk_WithError(t *testing.T) {
	chunk := StreamChunk{
		Error: &testError{},
	}

	if chunk.Error == nil {
		t.Error("Error should not be nil")
	}
}

// =============================================================================
// ERROR TYPE TESTS
// =============================================================================

func TestOllamaError_Fields(t *testing.T) {
	err := OllamaError{
		Error: "model not found",
	}

	if err.Error != "model not found" {
		t.Errorf("Error = %q", err.Error)
	}
}

// =============================================================================
// MODEL DETAILS TESTS
// =============================================================================

func TestModelDetails_Fields(t *testing.T) {
	details := ModelDetails{
		Format:            "gguf",
		Family:            "qwen2.5",
		Families:          []string{"qwen2.5", "qwen2"},
		ParameterSize:     "14B",
		QuantizationLevel: "Q4_K_M",
	}

	if details.Format != "gguf" {
		t.Errorf("Format = %q", details.Format)
	}

	if details.Family != "qwen2.5" {
		t.Errorf("Family = %q", details.Family)
	}

	if len(details.Families) != 2 {
		t.Errorf("Families length = %d", len(details.Families))
	}

	if details.ParameterSize != "14B" {
		t.Errorf("ParameterSize = %q", details.ParameterSize)
	}

	if details.QuantizationLevel != "Q4_K_M" {
		t.Errorf("QuantizationLevel = %q", details.QuantizationLevel)
	}
}

func TestListModelsResponse_Fields(t *testing.T) {
	resp := ListModelsResponse{
		Models: []ModelInfo{
			{Name: "qwen2.5:14b", Size: 8_000_000_000},
			{Name: "llama3:8b", Size: 4_000_000_000},
		},
	}

	if len(resp.Models) != 2 {
		t.Errorf("Models length = %d, want 2", len(resp.Models))
	}

	if resp.Models[0].Name != "qwen2.5:14b" {
		t.Errorf("Models[0].Name = %q", resp.Models[0].Name)
	}
}

func TestShowModelResponse_Fields(t *testing.T) {
	resp := ShowModelResponse{
		License:    "MIT",
		Modelfile:  "FROM qwen2.5:14b",
		Parameters: "temperature 0.7",
		Template:   "{{ .System }}\n{{ .Prompt }}",
		Details: ModelDetails{
			Family: "qwen2.5",
		},
	}

	if resp.License != "MIT" {
		t.Errorf("License = %q", resp.License)
	}

	if resp.Details.Family != "qwen2.5" {
		t.Errorf("Details.Family = %q", resp.Details.Family)
	}
}

// =============================================================================
// TEST HELPERS
// =============================================================================

type testError struct{}

func (e *testError) Error() string { return "test error" }
