// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package server provides an HTTP API server with OpenAI-compatible endpoints.
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/router"
)

// =============================================================================
// SERVER STATS TESTS
// =============================================================================

func TestNewServerStats(t *testing.T) {
	stats := NewServerStats()

	if stats == nil {
		t.Fatal("NewServerStats() returned nil")
	}

	if stats.TotalRequests != 0 {
		t.Errorf("TotalRequests = %d, want 0", stats.TotalRequests)
	}

	if stats.StartTime.IsZero() {
		t.Error("StartTime should be set")
	}
}

func TestServerStats_RecordRequest(t *testing.T) {
	stats := NewServerStats()

	// Record cache hit
	stats.RecordRequest(router.TierCache, 0, 0)
	if stats.CacheHits != 1 {
		t.Errorf("CacheHits = %d, want 1", stats.CacheHits)
	}

	// Record local request
	stats.RecordRequest(router.TierLocal, 100, 0)
	if stats.LocalRequests != 1 {
		t.Errorf("LocalRequests = %d, want 1", stats.LocalRequests)
	}

	// Record cloud request
	stats.RecordRequest(router.TierCloud, 200, 0.5)
	if stats.CloudRequests != 1 {
		t.Errorf("CloudRequests = %d, want 1", stats.CloudRequests)
	}

	// Check totals
	if stats.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3", stats.TotalRequests)
	}

	if stats.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300", stats.TotalTokens)
	}
}

func TestServerStats_GetStats(t *testing.T) {
	stats := NewServerStats()
	stats.RecordRequest(router.TierLocal, 100, 0)
	stats.RecordRequest(router.TierCloud, 200, 1.5)

	copy := stats.GetStats()

	if copy.TotalRequests != 2 {
		t.Errorf("GetStats().TotalRequests = %d, want 2", copy.TotalRequests)
	}

	if copy.TotalTokens != 300 {
		t.Errorf("GetStats().TotalTokens = %d, want 300", copy.TotalTokens)
	}

	if copy.TotalCostCents != 1.5 {
		t.Errorf("GetStats().TotalCostCents = %f, want 1.5", copy.TotalCostCents)
	}
}

func TestServerStats_Uptime(t *testing.T) {
	stats := NewServerStats()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	uptime := stats.Uptime()
	if uptime < 10*time.Millisecond {
		t.Errorf("Uptime = %v, expected >= 10ms", uptime)
	}
}

// =============================================================================
// SERVER TESTS
// =============================================================================

func TestNewServer(t *testing.T) {
	s := NewServer(0)

	if s == nil {
		t.Fatal("NewServer(0) returned nil")
	}

	if s.Port() != DefaultPort {
		t.Errorf("Port() = %d, want %d", s.Port(), DefaultPort)
	}
}

func TestNewServer_CustomPort(t *testing.T) {
	s := NewServer(9999)

	if s.Port() != 9999 {
		t.Errorf("Port() = %d, want 9999", s.Port())
	}
}

func TestServer_WithMethods(t *testing.T) {
	s := NewServer(0)

	// Test chaining
	s2 := s.WithOllamaClient(nil)
	if s2 != s {
		t.Error("WithOllamaClient should return same server")
	}

	s3 := s.WithCloudClient(nil)
	if s3 != s {
		t.Error("WithCloudClient should return same server")
	}

	s4 := s.WithCache(nil)
	if s4 != s {
		t.Error("WithCache should return same server")
	}

	s5 := s.WithAuth(nil)
	if s5 != s {
		t.Error("WithAuth should return same server")
	}
}

// =============================================================================
// VALIDATE MESSAGES TESTS
// =============================================================================

func TestValidateMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []ChatMessage
		wantErr  bool
	}{
		{
			name:     "empty",
			messages: []ChatMessage{},
			wantErr:  false,
		},
		{
			name: "valid user",
			messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
			},
			wantErr: false,
		},
		{
			name: "valid assistant",
			messages: []ChatMessage{
				{Role: "assistant", Content: "Hi there!"},
			},
			wantErr: false,
		},
		{
			name: "valid system",
			messages: []ChatMessage{
				{Role: "system", Content: "Be helpful"},
			},
			wantErr: false,
		},
		{
			name: "valid tool",
			messages: []ChatMessage{
				{Role: "tool", Content: "Result"},
			},
			wantErr: false,
		},
		{
			name: "valid conversation",
			messages: []ChatMessage{
				{Role: "system", Content: "Be helpful"},
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi!"},
			},
			wantErr: false,
		},
		{
			name: "invalid role",
			messages: []ChatMessage{
				{Role: "invalid", Content: "Hello"},
			},
			wantErr: true,
		},
		{
			name: "empty role",
			messages: []ChatMessage{
				{Role: "", Content: "Hello"},
			},
			wantErr: true,
		},
		{
			name: "mixed valid and invalid",
			messages: []ChatMessage{
				{Role: "user", Content: "Hello"},
				{Role: "hacker", Content: "Evil"},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMessages(tc.messages)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateMessages() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// =============================================================================
// HANDLER TESTS
// =============================================================================

func TestHandleHealth(t *testing.T) {
	s := NewServer(0)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Version != Version {
		t.Errorf("Version = %q, want %q", resp.Version, Version)
	}
}

func TestHandleStats(t *testing.T) {
	s := NewServer(0)

	// Record some stats
	s.stats.RecordRequest(router.TierLocal, 100, 0)
	s.stats.RecordRequest(router.TierCache, 0, 0)

	req := httptest.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()

	s.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp StatsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.TotalRequests != 2 {
		t.Errorf("TotalRequests = %d, want 2", resp.TotalRequests)
	}

	if resp.CacheHits != 1 {
		t.Errorf("CacheHits = %d, want 1", resp.CacheHits)
	}

	if resp.LocalRequests != 1 {
		t.Errorf("LocalRequests = %d, want 1", resp.LocalRequests)
	}
}

func TestHandleModels(t *testing.T) {
	s := NewServer(0)

	req := httptest.NewRequest("GET", "/v1/models", nil)
	w := httptest.NewRecorder()

	s.handleModels(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp ModelsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Object != "list" {
		t.Errorf("Object = %q, want 'list'", resp.Object)
	}

	if len(resp.Data) == 0 {
		t.Error("Data should contain models")
	}

	// Check for expected models
	hasAuto := false
	hasLocal := false
	for _, m := range resp.Data {
		if m.ID == "auto" {
			hasAuto = true
		}
		if m.ID == "local" {
			hasLocal = true
		}
	}

	if !hasAuto {
		t.Error("Should have 'auto' model")
	}

	if !hasLocal {
		t.Error("Should have 'local' model")
	}
}

func TestHandleCacheStats(t *testing.T) {
	s := NewServer(0)

	req := httptest.NewRequest("GET", "/cache/stats", nil)
	w := httptest.NewRecorder()

	s.handleCacheStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp CacheStatsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
}

func TestHandleCacheClear(t *testing.T) {
	s := NewServer(0)

	req := httptest.NewRequest("POST", "/cache/clear", nil)
	w := httptest.NewRecorder()

	s.handleCacheClear(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("status = %q, want 'ok'", resp["status"])
	}
}

// =============================================================================
// CHAT COMPLETIONS TESTS
// =============================================================================

func TestHandleChatCompletions_EmptyMessages(t *testing.T) {
	s := NewServer(0)

	body := `{"model": "auto", "messages": []}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleChatCompletions_InvalidJSON(t *testing.T) {
	s := NewServer(0)

	body := `{invalid json}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleChatCompletions_InvalidRole(t *testing.T) {
	s := NewServer(0)

	body := `{"model": "auto", "messages": [{"role": "hacker", "content": "test"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleChatCompletions_TooManyMessages(t *testing.T) {
	s := NewServer(0)

	messages := make([]ChatMessage, MaxMessageCount+1)
	for i := range messages {
		messages[i] = ChatMessage{Role: "user", Content: "test"}
	}

	reqBody := ChatCompletionRequest{
		Model:    "auto",
		Messages: messages,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleChatCompletions_MessageTooLong(t *testing.T) {
	s := NewServer(0)

	longContent := strings.Repeat("a", MaxQueryLength+1)
	body := `{"model": "auto", "messages": [{"role": "user", "content": "` + longContent + `"}]}`

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleChatCompletions_InvalidMaxTokens(t *testing.T) {
	s := NewServer(0)

	body := `{"model": "auto", "messages": [{"role": "user", "content": "test"}], "max_tokens": -1}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleChatCompletions_InvalidTemperature(t *testing.T) {
	s := NewServer(0)

	body := `{"model": "auto", "messages": [{"role": "user", "content": "test"}], "temperature": 3.0}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleChatCompletions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// =============================================================================
// ROUTING TESTS
// =============================================================================

func TestRouteRequest(t *testing.T) {
	s := NewServer(0)

	tests := []struct {
		model string
		query string
		want  router.Tier
	}{
		{"local", "test", router.TierLocal},
		{"cache", "test", router.TierLocal},
		{"cloud", "test", router.TierCloud},
		{"haiku", "test", router.TierCloud},
		{"sonnet", "test", router.TierCloud},
		{"opus", "test", router.TierCloud},
		{"gpt4", "test", router.TierCloud},
		{"gpt4o", "test", router.TierCloud},
		{"unknown-model", "test", router.TierLocal},
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			tier := s.routeRequest(tc.model, tc.query)
			if tier != tc.want {
				t.Errorf("routeRequest(%q, %q) = %v, want %v", tc.model, tc.query, tier, tc.want)
			}
		})
	}
}

// =============================================================================
// TYPE TESTS
// =============================================================================

func TestChatCompletionRequest_Fields(t *testing.T) {
	req := ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream:      true,
		Temperature: 0.7,
		MaxTokens:   100,
	}

	if req.Model != "gpt-4" {
		t.Errorf("Model = %q", req.Model)
	}

	if len(req.Messages) != 1 {
		t.Errorf("Messages length = %d", len(req.Messages))
	}

	if !req.Stream {
		t.Error("Stream should be true")
	}

	if req.Temperature != 0.7 {
		t.Errorf("Temperature = %f", req.Temperature)
	}

	if req.MaxTokens != 100 {
		t.Errorf("MaxTokens = %d", req.MaxTokens)
	}
}

func TestChatCompletionResponse_Fields(t *testing.T) {
	resp := ChatCompletionResponse{
		ID:      "test-id",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "gpt-4",
		Choices: []ChatChoice{
			{
				Index:        0,
				Message:      ChatMessage{Role: "assistant", Content: "Hello!"},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	if resp.ID != "test-id" {
		t.Errorf("ID = %q", resp.ID)
	}

	if resp.Object != "chat.completion" {
		t.Errorf("Object = %q", resp.Object)
	}

	if len(resp.Choices) != 1 {
		t.Errorf("Choices length = %d", len(resp.Choices))
	}

	if resp.Usage.TotalTokens != 15 {
		t.Errorf("Usage.TotalTokens = %d", resp.Usage.TotalTokens)
	}
}

func TestUsage_Fields(t *testing.T) {
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	if usage.PromptTokens != 100 {
		t.Errorf("PromptTokens = %d", usage.PromptTokens)
	}

	if usage.CompletionTokens != 50 {
		t.Errorf("CompletionTokens = %d", usage.CompletionTokens)
	}

	if usage.TotalTokens != 150 {
		t.Errorf("TotalTokens = %d", usage.TotalTokens)
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestGenerateResponseID(t *testing.T) {
	id1 := generateResponseID()
	id2 := generateResponseID()

	if id1 == id2 {
		t.Error("generateResponseID should return unique IDs")
	}

	if !strings.HasPrefix(id1, "chatcmpl-") {
		t.Errorf("ID should start with 'chatcmpl-', got: %s", id1)
	}

	// Should be 32 hex chars (16 bytes) plus prefix
	if len(id1) != 9+32 {
		t.Errorf("ID length = %d, expected %d", len(id1), 9+32)
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"abc", 0, "..."},
	}

	for _, tc := range tests {
		got := truncateString(tc.input, tc.maxLen)
		if got != tc.want {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
		}
	}
}

func TestTruncateString_Unicode(t *testing.T) {
	// Unicode handling
	input := "Hello 世界!"
	result := truncateString(input, 7)

	// Should truncate by runes, not bytes
	if result != "Hello 世..." {
		t.Errorf("truncateString with unicode = %q, expected 'Hello 世...'", result)
	}
}

// =============================================================================
// CONSTANT TESTS
// =============================================================================

func TestConstants(t *testing.T) {
	if DefaultPort != 8787 {
		t.Errorf("DefaultPort = %d, want 8787", DefaultPort)
	}

	if MaxQueryLength != 100000 {
		t.Errorf("MaxQueryLength = %d, want 100000", MaxQueryLength)
	}

	if MaxMessageCount != 100 {
		t.Errorf("MaxMessageCount = %d, want 100", MaxMessageCount)
	}

	if MaxRequestBodySize != 1*1024*1024 {
		t.Errorf("MaxRequestBodySize = %d, want 1MB", MaxRequestBodySize)
	}

	if MaxTokensLimit != 128000 {
		t.Errorf("MaxTokensLimit = %d, want 128000", MaxTokensLimit)
	}

	if MinTemperature != 0.0 {
		t.Errorf("MinTemperature = %f, want 0.0", MinTemperature)
	}

	if MaxTemperature != 2.0 {
		t.Errorf("MaxTemperature = %f, want 2.0", MaxTemperature)
	}
}

func TestValidRoles(t *testing.T) {
	expected := []string{"user", "assistant", "system", "tool"}

	for _, role := range expected {
		if !validRoles[role] {
			t.Errorf("validRoles should include %q", role)
		}
	}

	// Invalid roles
	invalid := []string{"hacker", "admin", "root", ""}
	for _, role := range invalid {
		if validRoles[role] {
			t.Errorf("validRoles should not include %q", role)
		}
	}
}
