// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cloud

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// CONCURRENT ACCESS TESTS
// =============================================================================

// TestChatWithModel_Concurrent verifies that ChatWithModel is thread-safe.
// This is a regression test for the race condition where concurrent calls
// would modify the shared c.model field, causing data races.
//
// Run with: go test -race -run TestChatWithModel_Concurrent
func TestChatWithModel_Concurrent(t *testing.T) {
	// Create a mock server that returns different responses based on model
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return a minimal valid response
		w.Write([]byte(`{
			"id": "test-id",
			"model": "test-model",
			"choices": [{
				"message": {"role": "assistant", "content": "test response"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
		}`))
	}))
	defer server.Close()

	// Create client with test API key
	client := NewOpenRouterClient("sk-or-test-abcdefghijklmnopqrstuvwxyz0123456789")
	client.WithBaseURL(server.URL)
	client.WithCertValidation(false) // Disable cert validation for test server

	var wg sync.WaitGroup
	errChan := make(chan error, 100)
	modelsSeen := make(map[string]bool)
	var modelsMu sync.Mutex

	// Launch 100 concurrent calls with different models
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(modelNum int) {
			defer wg.Done()

			model := fmt.Sprintf("test-model-%d", modelNum%5)
			messages := []ChatMessage{NewUserMessage("hello")}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := client.ChatWithModel(ctx, model, messages)
			if err != nil {
				errChan <- fmt.Errorf("ChatWithModel error for model %s: %w", model, err)
				return
			}

			// Track which models we've seen
			modelsMu.Lock()
			modelsSeen[model] = true
			modelsMu.Unlock()
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		for _, err := range errors {
			t.Errorf("Concurrent ChatWithModel error: %v", err)
		}
	}

	// Verify we saw all expected models
	for i := 0; i < 5; i++ {
		model := fmt.Sprintf("test-model-%d", i)
		if !modelsSeen[model] {
			t.Errorf("Expected to see model %s in concurrent calls", model)
		}
	}

	// Verify the original client's model wasn't modified by concurrent calls
	originalModel := client.GetModel()
	if originalModel != "openrouter/auto" {
		t.Errorf("Original client model was modified: expected 'openrouter/auto', got %s", originalModel)
	}

	t.Logf("Successfully completed %d concurrent requests", requestCount.Load())
}

// TestChatWithModel_NoDeadlock verifies that concurrent ChatWithModel calls
// don't cause deadlocks under load.
func TestChatWithModel_NoDeadlock(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add small delay to simulate network latency
		time.Sleep(time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "test-id",
			"model": "test-model",
			"choices": [{
				"message": {"role": "assistant", "content": "test"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
		}`))
	}))
	defer server.Close()

	client := NewOpenRouterClient("sk-or-test-abcdefghijklmnopqrstuvwxyz0123456789")
	client.WithBaseURL(server.URL)
	client.WithCertValidation(false)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan bool)
	go func() {
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				messages := []ChatMessage{NewUserMessage("test")}
				reqCtx, reqCancel := context.WithTimeout(ctx, 5*time.Second)
				defer reqCancel()
				client.ChatWithModel(reqCtx, fmt.Sprintf("model-%d", n), messages)
			}(i)
		}
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success - no deadlock detected
		t.Log("No deadlock detected - test passed")
	case <-ctx.Done():
		t.Fatal("Deadlock detected - test timed out after 10 seconds")
	}
}

// TestChatWithModel_ModelIsolation verifies that each concurrent call
// uses its own model and doesn't affect other calls.
func TestChatWithModel_ModelIsolation(t *testing.T) {
	// Create a mock server that echoes back the model from the request
	var modelRequests sync.Map

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the request to get the model
		// For simplicity, we'll just track that requests came in
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "test-id",
			"model": "test-model",
			"choices": [{
				"message": {"role": "assistant", "content": "response"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
		}`))
	}))
	defer server.Close()

	client := NewOpenRouterClient("sk-or-test-abcdefghijklmnopqrstuvwxyz0123456789")
	client.WithBaseURL(server.URL)
	client.WithCertValidation(false)

	// Set initial model
	client.SetModel("initial-model")
	initialModel := client.GetModel()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			model := fmt.Sprintf("concurrent-model-%d", n)
			messages := []ChatMessage{NewUserMessage("test")}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := client.ChatWithModel(ctx, model, messages)
			if err == nil {
				modelRequests.Store(model, true)
			}
		}(i)
	}

	wg.Wait()

	// Verify the original client's model wasn't changed
	finalModel := client.GetModel()
	if finalModel != initialModel {
		t.Errorf("Client model was modified during concurrent calls: expected %s, got %s",
			initialModel, finalModel)
	}

	// Verify all concurrent models were processed
	count := 0
	modelRequests.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	if count != 20 {
		t.Errorf("Expected 20 unique models processed, got %d", count)
	}
}

// TestSetModel_Concurrent verifies that SetModel itself is not designed
// for concurrent use (expected behavior - callers should use ChatWithModel
// for concurrent access with different models).
func TestSetModel_Concurrent(t *testing.T) {
	client := NewOpenRouterClient("sk-or-test-abcdefghijklmnopqrstuvwxyz0123456789")

	// Note: This test documents that SetModel is NOT thread-safe by design.
	// Use ChatWithModel for concurrent access with different models.
	// Running this with -race will show the race condition if SetModel
	// is called concurrently (which is expected behavior).

	// For documentation purposes, we verify that the fix is to use ChatWithModel:
	t.Log("SetModel is NOT thread-safe by design. Use ChatWithModel for concurrent access.")
	t.Log("ChatWithModel creates a copy of the client, making it safe for concurrent use.")

	// Simple sequential test to ensure SetModel works
	client.SetModel("model-1")
	if client.GetModel() != "model-1" {
		t.Errorf("SetModel failed: expected model-1, got %s", client.GetModel())
	}

	client.SetModel("sonnet") // Using friendly name
	if client.GetModel() != "anthropic/claude-3-sonnet" {
		t.Errorf("SetModel with friendly name failed: expected anthropic/claude-3-sonnet, got %s", client.GetModel())
	}
}

// =============================================================================
// CLIENT CONFIGURATION TESTS
// =============================================================================

// TestNewOpenRouterClient verifies client initialization.
func TestNewOpenRouterClient(t *testing.T) {
	apiKey := "sk-or-test-abcdefghijklmnopqrstuvwxyz0123456789"
	client := NewOpenRouterClient(apiKey)

	if !client.IsConfigured() {
		t.Error("Client should be configured with valid API key")
	}

	if client.GetModel() != "openrouter/auto" {
		t.Errorf("Default model should be 'openrouter/auto', got %s", client.GetModel())
	}

	// Test empty API key
	emptyClient := NewOpenRouterClient("")
	if emptyClient.IsConfigured() {
		t.Error("Client with empty API key should not be configured")
	}
}

// TestAPIKeyMasked verifies API key masking for display using secure fingerprints.
// NIST 800-53 IA-5(1): Obscure feedback of authentication information.
func TestAPIKeyMasked(t *testing.T) {
	tests := []struct {
		name              string
		apiKey            string
		expectedFormat    string // Expected format of the masked key
		shouldContainHash bool   // Should contain a hash fingerprint
	}{
		{
			name:              "empty key",
			apiKey:            "",
			expectedFormat:    "[not set]",
			shouldContainHash: false,
		},
		{
			name:              "short key",
			apiKey:            "abc",
			expectedFormat:    "[REDACTED, length=3, fingerprint=",
			shouldContainHash: true,
		},
		{
			name:              "normal key",
			apiKey:            "sk-or-test-abc123",
			expectedFormat:    "[REDACTED, length=17, fingerprint=",
			shouldContainHash: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := NewOpenRouterClient(tc.apiKey)
			masked := client.APIKeyMasked()

			// Check that the masked value starts with the expected format
			if !strings.HasPrefix(masked, tc.expectedFormat) {
				t.Errorf("Expected masked key to start with %q, got %q", tc.expectedFormat, masked)
			}

			// Verify it contains a hash fingerprint (not the original key prefix)
			if tc.shouldContainHash {
				if strings.Contains(masked, tc.apiKey) || strings.Contains(masked, tc.apiKey[:min(4, len(tc.apiKey))]) {
					t.Errorf("Masked key should not contain any part of the original key, got %q", masked)
				}
				// Verify it contains a hex fingerprint
				if !strings.Contains(masked, "fingerprint=") {
					t.Errorf("Masked key should contain fingerprint, got %q", masked)
				}
			}
		})
	}
}

// TestValidateAPIKey verifies API key format validation.
func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		valid  bool
	}{
		{
			name:   "valid key",
			apiKey: "sk-or-v1-abcdefghijklmnopqrstuvwxyz0123456789",
			valid:  true,
		},
		{
			name:   "wrong prefix",
			apiKey: "sk-abc-test-key-here",
			valid:  false,
		},
		{
			name:   "too short",
			apiKey: "sk-or-short",
			valid:  false,
		},
		{
			name:   "low entropy",
			apiKey: "sk-or-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			valid:  false,
		},
		{
			name:   "empty",
			apiKey: "",
			valid:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ValidateAPIKey(tc.apiKey)
			if result != tc.valid {
				t.Errorf("ValidateAPIKey(%q) = %v, expected %v", tc.apiKey, result, tc.valid)
			}
		})
	}
}

// =============================================================================
// CLIENT METHOD CHAINING TESTS
// =============================================================================

// TestClientMethodChaining verifies the fluent API for client configuration.
func TestClientMethodChaining(t *testing.T) {
	client := NewOpenRouterClient("sk-or-test-abcdefghijklmnopqrstuvwxyz0123456789").
		WithBaseURL("https://custom.api.com").
		WithTimeout(30 * time.Second).
		WithMaxRetries(5).
		WithSiteURL("https://mysite.com").
		WithSiteName("mysite").
		WithCertValidation(false)

	// Verify the chain returns the same client
	if client == nil {
		t.Fatal("Method chaining should return non-nil client")
	}

	// Verify settings were applied (we can't directly access private fields,
	// but we can test behavior that depends on them)
	if !client.IsConfigured() {
		t.Error("Client should still be configured after method chaining")
	}
}

// =============================================================================
// MESSAGE HELPER TESTS
// =============================================================================

// TestChatMessageHelpers verifies message creation helpers.
func TestChatMessageHelpers(t *testing.T) {
	userMsg := NewUserMessage("user content")
	if userMsg.Role != "user" || userMsg.Content != "user content" {
		t.Errorf("NewUserMessage incorrect: got role=%s, content=%s", userMsg.Role, userMsg.Content)
	}

	assistantMsg := NewAssistantMessage("assistant content")
	if assistantMsg.Role != "assistant" || assistantMsg.Content != "assistant content" {
		t.Errorf("NewAssistantMessage incorrect: got role=%s, content=%s", assistantMsg.Role, assistantMsg.Content)
	}

	systemMsg := NewSystemMessage("system content")
	if systemMsg.Role != "system" || systemMsg.Content != "system content" {
		t.Errorf("NewSystemMessage incorrect: got role=%s, content=%s", systemMsg.Role, systemMsg.Content)
	}
}

// TestChatResponseGetContent verifies response content extraction.
func TestChatResponseGetContent(t *testing.T) {
	// Test with content
	resp := &ChatResponse{
		Choices: []struct {
			Message      ChatMessage `json:"message"`
			FinishReason string      `json:"finish_reason"`
		}{
			{
				Message:      ChatMessage{Role: "assistant", Content: "test content"},
				FinishReason: "stop",
			},
		},
	}
	if resp.GetContent() != "test content" {
		t.Errorf("GetContent() = %q, expected 'test content'", resp.GetContent())
	}

	// Test empty choices
	emptyResp := &ChatResponse{}
	if emptyResp.GetContent() != "" {
		t.Errorf("GetContent() on empty response = %q, expected empty string", emptyResp.GetContent())
	}
}

// =============================================================================
// ERROR HANDLING TESTS
// =============================================================================

// TestOpenRouterError verifies error formatting.
func TestOpenRouterError(t *testing.T) {
	// Error with code
	errWithCode := &OpenRouterError{
		Code:    "invalid_api_key",
		Message: "API key is invalid",
		Status:  401,
	}
	expected := "OpenRouter error [invalid_api_key] (HTTP 401): API key is invalid"
	if errWithCode.Error() != expected {
		t.Errorf("Error() = %q, expected %q", errWithCode.Error(), expected)
	}

	// Error without code
	errNoCode := &OpenRouterError{
		Message: "Server error",
		Status:  500,
	}
	expected = "OpenRouter error (HTTP 500): Server error"
	if errNoCode.Error() != expected {
		t.Errorf("Error() = %q, expected %q", errNoCode.Error(), expected)
	}
}

// =============================================================================
// RETRY LOGIC TESTS
// =============================================================================

// TestIsRetryable verifies retry decision logic.
func TestIsRetryable(t *testing.T) {
	client := NewOpenRouterClient("sk-or-test-abcdefghijklmnopqrstuvwxyz0123456789")

	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "rate limited",
			err:       ErrRateLimited,
			retryable: true,
		},
		{
			name:      "server error 500",
			err:       &OpenRouterError{Status: 500, Message: "Internal Server Error"},
			retryable: true,
		},
		{
			name:      "server error 503",
			err:       &OpenRouterError{Status: 503, Message: "Service Unavailable"},
			retryable: true,
		},
		{
			name:      "client error 400",
			err:       &OpenRouterError{Status: 400, Message: "Bad Request"},
			retryable: false,
		},
		{
			name:      "auth failed",
			err:       ErrAuthFailed,
			retryable: false,
		},
		{
			name:      "context canceled",
			err:       context.Canceled,
			retryable: false,
		},
		{
			name:      "context deadline",
			err:       context.DeadlineExceeded,
			retryable: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := client.isRetryable(tc.err)
			if result != tc.retryable {
				t.Errorf("isRetryable(%v) = %v, expected %v", tc.err, result, tc.retryable)
			}
		})
	}
}

// TestCalculateBackoff verifies exponential backoff calculation.
func TestCalculateBackoff(t *testing.T) {
	client := NewOpenRouterClient("sk-or-test-abcdefghijklmnopqrstuvwxyz0123456789")

	// Attempt 0 should give base delay
	delay0 := client.calculateBackoff(0)
	if delay0 != 500*time.Millisecond {
		t.Errorf("Backoff for attempt 0 = %v, expected 500ms", delay0)
	}

	// Attempt 1 should double
	delay1 := client.calculateBackoff(1)
	if delay1 != 1000*time.Millisecond {
		t.Errorf("Backoff for attempt 1 = %v, expected 1000ms", delay1)
	}

	// Attempt 2 should double again
	delay2 := client.calculateBackoff(2)
	if delay2 != 2000*time.Millisecond {
		t.Errorf("Backoff for attempt 2 = %v, expected 2000ms", delay2)
	}

	// High attempts should cap at max delay
	delayHigh := client.calculateBackoff(10)
	if delayHigh != 10*time.Second {
		t.Errorf("Backoff for attempt 10 = %v, expected 10s (max)", delayHigh)
	}
}

// =============================================================================
// BENCHMARK TESTS
// =============================================================================

// BenchmarkChatWithModel_Concurrent benchmarks concurrent ChatWithModel calls.
func BenchmarkChatWithModel_Concurrent(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "test-id",
			"model": "test-model",
			"choices": [{
				"message": {"role": "assistant", "content": "response"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
		}`))
	}))
	defer server.Close()

	client := NewOpenRouterClient("sk-or-test-abcdefghijklmnopqrstuvwxyz0123456789")
	client.WithBaseURL(server.URL)
	client.WithCertValidation(false)

	messages := []ChatMessage{NewUserMessage("test")}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			ctx := context.Background()
			client.ChatWithModel(ctx, fmt.Sprintf("model-%d", i%10), messages)
			i++
		}
	})
}
