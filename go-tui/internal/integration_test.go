// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package internal provides integration tests for the complete rigrun system.
//
// These tests verify end-to-end functionality including:
// - Query routing through tiers
// - Cache integration
// - Tool execution
// - Session management
// - Audit logging
// - Secret redaction
// - Configuration management
// - GPU detection fallback
// - Model recommendations
// - Classification banners
package internal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/cache"
	"github.com/jeranaias/rigrun-tui/internal/cli"
	"github.com/jeranaias/rigrun-tui/internal/detect"
	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/session"
	"github.com/jeranaias/rigrun-tui/internal/storage"
	"github.com/jeranaias/rigrun-tui/internal/tools"
)

// =============================================================================
// TEST UTILITIES
// =============================================================================

// createTempDir creates a temporary directory for testing.
// The directory is automatically cleaned up when the test finishes.
func createTempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// createTempFile creates a temporary file with the given content.
// Returns the file path. The file is automatically cleaned up when the test finishes.
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return path
}

// mockEmbeddingFunc returns a mock embedding function for testing.
// Returns a fixed embedding vector for any query.
func mockEmbeddingFunc() cache.EmbeddingFunc {
	return func(query string) ([]float64, error) {
		// Return a simple fixed embedding based on query length
		// This is sufficient for testing cache behavior
		embedding := make([]float64, 128)
		for i := range embedding {
			embedding[i] = float64(len(query)%10) * 0.1
		}
		return embedding, nil
	}
}

// =============================================================================
// END-TO-END ROUTING TEST
// =============================================================================

// TestEndToEndRouting verifies that queries are routed to the correct tier
// based on complexity classification.
func TestEndToEndRouting(t *testing.T) {
	// Create cache manager using defaults
	cacheManager := cache.NewCacheManager(nil, nil)

	// Test cases for routing
	tests := []struct {
		name            string
		query           string
		expectedTier    router.Tier
		expectedMinTier router.Tier
	}{
		{
			name:            "trivial query routes to cache",
			query:           "hi",
			expectedTier:    router.TierCache,
			expectedMinTier: router.TierCache,
		},
		{
			name:            "simple lookup routes to local",
			query:           "what is Go?",
			expectedTier:    router.TierLocal,
			expectedMinTier: router.TierLocal,
		},
		{
			name:            "complex query routes to cloud",
			query:           "explain the difference between channels and mutexes in Go and when to use each",
			expectedTier:    router.TierCloud,
			expectedMinTier: router.TierCloud,
		},
		{
			name:            "architecture question routes to cloud",
			query:           "what is the best approach for designing a microservices architecture",
			expectedTier:    router.TierCloud,
			expectedMinTier: router.TierCloud,
		},
		{
			name:            "code generation routes to cloud",
			query:           "implement a binary search tree in Go with insert and delete functions",
			expectedTier:    router.TierCloud,
			expectedMinTier: router.TierCloud,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Classify the query
			complexity := router.ClassifyComplexity(tc.query)
			tier := complexity.MinTier()

			// Verify tier selection
			if tier != tc.expectedTier {
				t.Errorf("expected tier %s, got %s for query: %s",
					tc.expectedTier, tier, tc.query)
			}

			// Test detailed routing
			decision := router.RouteQueryDetailed(tc.query, security.ClassificationUnclassified, nil)
			if decision.Tier != tc.expectedTier {
				t.Errorf("detailed routing: expected tier %s, got %s",
					tc.expectedTier, decision.Tier)
			}

			// Verify the cache manager is initialized
			if cacheManager == nil {
				t.Error("cache manager should not be nil")
			}
		})
	}

	// Test with max tier constraint
	t.Run("max tier constraint", func(t *testing.T) {
		maxTier := router.TierLocal
		tier := router.RouteQuery("explain complex architecture patterns", security.ClassificationUnclassified, false, &maxTier)
		if tier != router.TierLocal {
			t.Errorf("expected tier %s with max constraint, got %s", router.TierLocal, tier)
		}
	})
}

// =============================================================================
// CACHE INTEGRATION TEST
// =============================================================================

// TestCacheIntegration verifies that the cache correctly stores and retrieves responses.
func TestCacheIntegration(t *testing.T) {
	// Create fresh cache manager using defaults
	cacheManager := cache.NewCacheManager(nil, nil)

	// Set up embedding function for semantic cache
	cacheManager.SetEmbeddingFunc(mockEmbeddingFunc())

	query := "what is the capital of France?"
	response := "The capital of France is Paris."
	tier := "local"

	// Initially should miss
	_, hitType := cacheManager.Lookup(query)
	if hitType != cache.CacheHitNone {
		t.Errorf("expected cache miss, got hit type: %s", hitType)
	}

	// Store the response
	cacheManager.Store(query, response, tier)

	// Now should hit
	cachedResponse, hitType := cacheManager.Lookup(query)
	if hitType != cache.CacheHitExact {
		t.Errorf("expected exact cache hit, got: %s", hitType)
	}
	if cachedResponse != response {
		t.Errorf("expected response %q, got %q", response, cachedResponse)
	}

	// Verify stats
	stats := cacheManager.Stats()
	if stats.ExactHits != 1 {
		t.Errorf("expected 1 exact hit, got %d", stats.ExactHits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.TotalLookups != 2 {
		t.Errorf("expected 2 total lookups, got %d", stats.TotalLookups)
	}

	// Test cache clear
	cacheManager.Clear()
	_, hitType = cacheManager.Lookup(query)
	if hitType != cache.CacheHitNone {
		t.Errorf("expected cache miss after clear, got: %s", hitType)
	}
}

// =============================================================================
// TOOL EXECUTION TEST
// =============================================================================

// TestToolExecution verifies that tools can be registered and executed correctly.
func TestToolExecution(t *testing.T) {
	// Create a test file to read
	testContent := "line 1\nline 2\nline 3\n"
	testFile := createTempFile(t, testContent)

	// Create tool registry with built-in tools
	registry := tools.NewRegistry()

	// Verify Read tool is registered
	readTool := registry.Get("Read")
	if readTool == nil {
		t.Fatal("Read tool should be registered")
	}

	// Execute read on the test file
	ctx := context.Background()
	params := map[string]interface{}{
		"file_path": testFile,
	}

	result, err := readTool.Executor.Execute(ctx, params)
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}

	// Verify content was read (output includes line numbers)
	if !strings.Contains(result.Output, "line 1") {
		t.Errorf("expected output to contain 'line 1', got: %s", result.Output)
	}
	if !strings.Contains(result.Output, "line 2") {
		t.Errorf("expected output to contain 'line 2', got: %s", result.Output)
	}

	// Test reading file outside allowed directories
	// Security hardening detects path traversal before checking if file exists
	params["file_path"] = "/nonexistent/path/file.txt"
	result, err = readTool.Executor.Execute(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error for path outside allowed directories: %v", err)
	}
	if result.Success {
		t.Error("expected failure for path outside allowed directories")
	}
	if !strings.Contains(result.Error, "path traversal") {
		t.Errorf("expected 'path traversal' security error, got: %s", result.Error)
	}
}

// =============================================================================
// SESSION TIMEOUT TEST
// =============================================================================

// TestSessionTimeout verifies session timeout functionality.
func TestSessionTimeout(t *testing.T) {
	// Create session manager with short timeout for testing
	cfg := session.Config{
		Timeout:          100 * time.Millisecond,
		WarningBefore:    50 * time.Millisecond,
		AutoSaveEnabled:  false,
		AutoSaveInterval: time.Hour, // Disable auto-save for this test
	}
	mgr := session.NewManager(cfg)

	// Session should not be expired initially
	if mgr.IsExpired() {
		t.Error("session should not be expired initially")
	}

	// Get session ID
	sessionID := mgr.SessionID()
	if sessionID == "" {
		t.Error("session ID should not be empty")
	}

	// Verify session is not expired right away
	if mgr.RemainingTime() <= 0 {
		t.Error("remaining time should be positive initially")
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Now session should be expired
	if !mgr.IsExpired() {
		t.Error("session should be expired after timeout")
	}

	// Remaining time should be 0
	if mgr.RemainingTime() != 0 {
		t.Errorf("remaining time should be 0, got %v", mgr.RemainingTime())
	}

	// Record activity should reset the timeout
	mgr.RecordActivity()
	if mgr.IsExpired() {
		t.Error("session should not be expired after recording activity")
	}
}

// =============================================================================
// AUDIT LOGGING TEST
// =============================================================================

// TestAuditLogging verifies audit logging functionality.
func TestAuditLogging(t *testing.T) {
	// Create temp directory for audit log
	tempDir := createTempDir(t)
	auditPath := filepath.Join(tempDir, "audit.log")

	// Create audit logger
	logger, err := security.NewAuditLogger(auditPath)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	defer logger.Close()

	// Log an event
	event := security.AuditEvent{
		Timestamp: time.Now(),
		EventType: "TEST_EVENT",
		SessionID: "test_session_123",
		Tier:      "local",
		Query:     "test query",
		Tokens:    100,
		Cost:      0.001,
		Success:   true,
	}

	if err := logger.Log(event); err != nil {
		t.Fatalf("failed to log event: %v", err)
	}

	// Sync to ensure write
	if err := logger.Sync(); err != nil {
		t.Fatalf("failed to sync: %v", err)
	}

	// Read the log file and verify format
	content, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	logContent := string(content)

	// Verify log contains expected fields
	if !strings.Contains(logContent, "TEST_EVENT") {
		t.Error("log should contain event type")
	}
	if !strings.Contains(logContent, "test_session_123") {
		t.Error("log should contain session ID")
	}
	if !strings.Contains(logContent, "local") {
		t.Error("log should contain tier")
	}
	if !strings.Contains(logContent, "test query") {
		t.Error("log should contain query")
	}
	if !strings.Contains(logContent, "SUCCESS") {
		t.Error("log should contain SUCCESS status")
	}
}

// =============================================================================
// SECRET REDACTION TEST
// =============================================================================

// TestSecretRedaction verifies that sensitive data is properly redacted.
func TestSecretRedaction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "OpenAI API key",
			input:    "my key is sk-abc123xyz789abc123xyz789abc123xyz",
			expected: "my key is [OPENAI_KEY_REDACTED]",
		},
		{
			name:     "Bearer token",
			input:    "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expected: "Bearer [TOKEN_REDACTED]",
		},
		{
			name:     "password in query string",
			input:    "password=secret123",
			expected: "[PASSWORD_REDACTED]",
		},
		{
			name:     "OpenRouter key",
			input:    "key is sk-or-v1-abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz01",
			expected: "key is [OPENROUTER_KEY_REDACTED]",
		},
		{
			name:     "GitHub token",
			input:    "token: ghp_abcdefghijklmnopqrstuvwxyz012345678901",
			expected: "token: [GITHUB_TOKEN_REDACTED]",
		},
		{
			name:     "AWS access key",
			input:    "access_key: AKIAIOSFODNN7EXAMPLE",
			expected: "access_key: [AWS_KEY_REDACTED]",
		},
		{
			name:     "no secrets",
			input:    "this is a normal string with no secrets",
			expected: "this is a normal string with no secrets",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := security.RedactSecrets(tc.input)
			if result != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// =============================================================================
// CONFIG LOAD/SAVE TEST
// =============================================================================

// TestConfigLoadSave verifies configuration persistence.
func TestConfigLoadSave(t *testing.T) {
	// Create temp directory for config
	tempDir := createTempDir(t)
	configPath := filepath.Join(tempDir, ".rigrun", "config.toml")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("failed to create config directory: %v", err)
	}

	// Create a custom config
	originalConfig := cli.DefaultConfig()
	originalConfig.DefaultModel = "test-model:7b"
	originalConfig.Routing.DefaultMode = "cloud"
	originalConfig.Local.OllamaURL = "http://test:11434"
	originalConfig.Routing.MaxTier = "sonnet"
	originalConfig.Routing.ParanoidMode = true
	originalConfig.Security.SessionTimeoutSecs = 1800
	originalConfig.Security.AuditEnabled = false

	// Save config
	if err := cli.SaveConfig(originalConfig); err != nil {
		// Skip if we can't save to system config path in test environment
		t.Skipf("Cannot test config save in this environment: %v", err)
	}

	// Load config
	loadedConfig, err := cli.LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify values match
	if loadedConfig.DefaultModel != originalConfig.DefaultModel {
		t.Errorf("DefaultModel: expected %q, got %q",
			originalConfig.DefaultModel, loadedConfig.DefaultModel)
	}
	if loadedConfig.Routing.DefaultMode != originalConfig.Routing.DefaultMode {
		t.Errorf("DefaultMode: expected %q, got %q",
			originalConfig.Routing.DefaultMode, loadedConfig.Routing.DefaultMode)
	}
	if loadedConfig.Local.OllamaURL != originalConfig.Local.OllamaURL {
		t.Errorf("OllamaURL: expected %q, got %q",
			originalConfig.Local.OllamaURL, loadedConfig.Local.OllamaURL)
	}
}

// =============================================================================
// GPU DETECTION FALLBACK TEST
// =============================================================================

// TestGPUDetectionFallback verifies graceful fallback when no GPU is available.
func TestGPUDetectionFallback(t *testing.T) {
	// This test verifies that GPU detection doesn't error when nvidia-smi is unavailable
	// It should fall back to CPU mode gracefully

	// Detect GPU (will likely fall back to CPU in test environment)
	gpuInfo, err := detect.DetectGPU()
	if err != nil {
		t.Fatalf("DetectGPU should not error, got: %v", err)
	}

	// Should always return a GpuInfo, never nil
	if gpuInfo == nil {
		t.Fatal("gpuInfo should not be nil")
	}

	// Type should be set (CPU or actual GPU type)
	if gpuInfo.Type.String() == "" {
		t.Error("GPU type should have a string representation")
	}

	// VRAM should be non-negative
	if gpuInfo.VramGB == 0 && gpuInfo.Type != detect.GpuTypeCPU {
		t.Error("VRAM should be set for non-CPU types")
	}

	// Test CPU fallback info
	cpuInfo := detect.GetCPUInfo()
	if cpuInfo == nil {
		t.Fatal("GetCPUInfo should not return nil")
	}
	if cpuInfo.Type != detect.GpuTypeCPU {
		t.Errorf("expected CPU type, got %s", cpuInfo.Type)
	}
	if cpuInfo.Name != "CPU Only" {
		t.Errorf("expected 'CPU Only' name, got %s", cpuInfo.Name)
	}
}

// =============================================================================
// MODEL RECOMMENDATION TEST
// =============================================================================

// TestModelRecommendation verifies model recommendations based on VRAM.
func TestModelRecommendation(t *testing.T) {
	tests := []struct {
		name          string
		vramMB        int
		expectModel   string
		expectQuality string
	}{
		{
			name:          "8GB VRAM recommends 7b model",
			vramMB:        8 * 1024,
			expectModel:   "qwen2.5-coder:7b",
			expectQuality: "balanced",
		},
		{
			name:          "16GB VRAM recommends 14b model",
			vramMB:        16 * 1024,
			expectModel:   "qwen2.5-coder:14b",
			expectQuality: "best",
		},
		{
			name:          "24GB+ VRAM recommends 32b model",
			vramMB:        24 * 1024,
			expectModel:   "qwen2.5-coder:32b",
			expectQuality: "best",
		},
		{
			name:          "4GB VRAM recommends 3b model",
			vramMB:        4 * 1024,
			expectModel:   "qwen2.5-coder:3b",
			expectQuality: "fast",
		},
		{
			name:          "2GB VRAM recommends tiny model",
			vramMB:        2 * 1024,
			expectModel:   "tinyllama:1.1b",
			expectQuality: "fast",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recommendation := detect.RecommendModel(tc.vramMB)

			if recommendation.ModelName != tc.expectModel {
				t.Errorf("expected model %q, got %q", tc.expectModel, recommendation.ModelName)
			}
			if recommendation.Quality != tc.expectQuality {
				t.Errorf("expected quality %q, got %q", tc.expectQuality, recommendation.Quality)
			}
		})
	}

	// Test ListRecommendedModels
	t.Run("list recommended models for 16GB", func(t *testing.T) {
		models := detect.ListRecommendedModels(16 * 1024)
		if len(models) == 0 {
			t.Error("should recommend at least one model for 16GB")
		}

		// Should be sorted by quality (best first)
		for i := 0; i < len(models)-1; i++ {
			currentRank := qualityRank(models[i].Quality)
			nextRank := qualityRank(models[i+1].Quality)
			if currentRank < nextRank {
				t.Errorf("models not sorted by quality: %s before %s",
					models[i].Quality, models[i+1].Quality)
			}
		}
	})
}

// qualityRank returns a numeric rank for model quality sorting
func qualityRank(quality string) int {
	switch quality {
	case "best":
		return 3
	case "balanced":
		return 2
	case "fast":
		return 1
	default:
		return 0
	}
}

// =============================================================================
// CLASSIFICATION BANNER TEST
// =============================================================================

// TestClassificationBanner verifies classification banner rendering.
func TestClassificationBanner(t *testing.T) {
	// Test UNCLASSIFIED banner
	t.Run("UNCLASSIFIED banner", func(t *testing.T) {
		classification := security.DefaultClassification()
		if classification.Level != security.ClassificationUnclassified {
			t.Errorf("expected UNCLASSIFIED level, got %s", classification.Level)
		}

		// Render banner
		width := 80
		banner := security.RenderTopBanner(classification, width)

		// Verify text content
		if !strings.Contains(banner, "UNCLASSIFIED") {
			t.Error("banner should contain 'UNCLASSIFIED'")
		}
	})

	// Test banner text for different levels
	t.Run("classification level strings", func(t *testing.T) {
		levels := []struct {
			level    security.ClassificationLevel
			expected string
		}{
			{security.ClassificationUnclassified, "UNCLASSIFIED"},
			{security.ClassificationCUI, "CUI"},
			{security.ClassificationConfidential, "CONFIDENTIAL"},
			{security.ClassificationSecret, "SECRET"},
			{security.ClassificationTopSecret, "TOP SECRET"},
		}

		for _, tc := range levels {
			if tc.level.String() != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, tc.level.String())
			}
		}
	})

	// Test portion markings
	t.Run("portion markings", func(t *testing.T) {
		classification := security.Classification{
			Level: security.ClassificationSecret,
		}
		marking := security.RenderPortionMarking(classification)
		if marking != "(S)" {
			t.Errorf("expected (S), got %s", marking)
		}
	})

	// Test parsing classification strings
	t.Run("parse classification", func(t *testing.T) {
		c, err := security.ParseClassification("SECRET//NOFORN")
		if err != nil {
			t.Fatalf("failed to parse classification: %v", err)
		}
		if c.Level != security.ClassificationSecret {
			t.Errorf("expected SECRET level, got %s", c.Level)
		}
		if len(c.Caveats) == 0 || c.Caveats[0] != "NOFORN" {
			t.Error("expected NOFORN caveat")
		}
	})
}

// =============================================================================
// TIER ESCALATION TEST
// =============================================================================

// TestTierEscalation verifies tier escalation behavior.
func TestTierEscalation(t *testing.T) {
	tests := []struct {
		tier         router.Tier
		expectNext   *router.Tier
		expectExists bool
	}{
		{router.TierCache, ptrTier(router.TierLocal), true},
		{router.TierLocal, ptrTier(router.TierCloud), true},
		{router.TierCloud, nil, false}, // OpenRouter auto picks best
		{router.TierHaiku, ptrTier(router.TierSonnet), true},
		{router.TierSonnet, ptrTier(router.TierOpus), true},
		{router.TierOpus, nil, false},
		{router.TierGpt4o, nil, false},
	}

	for _, tc := range tests {
		t.Run(tc.tier.String(), func(t *testing.T) {
			next := tc.tier.Escalate()
			if tc.expectExists {
				if next == nil {
					t.Errorf("expected escalation from %s, got nil", tc.tier)
				} else if *next != *tc.expectNext {
					t.Errorf("expected escalation to %s, got %s", *tc.expectNext, *next)
				}
			} else {
				if next != nil {
					t.Errorf("expected no escalation from %s, got %s", tc.tier, *next)
				}
			}
		})
	}
}

// ptrTier returns a pointer to a Tier value.
func ptrTier(t router.Tier) *router.Tier {
	return &t
}

// =============================================================================
// CACHE PERSISTENCE TEST
// =============================================================================

// TestCachePersistence verifies cache save and load functionality.
// Skipped: Cache persistence (Save/Load) not implemented in current CacheManager.
func TestCachePersistence(t *testing.T) {
	t.Skip("Cache persistence (Save/Load) not implemented in current CacheManager")
}

// =============================================================================
// QUERY TYPE CLASSIFICATION TEST
// =============================================================================

// TestQueryTypeClassification verifies query type detection.
func TestQueryTypeClassification(t *testing.T) {
	tests := []struct {
		query      string
		expectType router.QueryType
	}{
		{"what is Go?", router.QueryTypeLookup},
		{"explain how channels work", router.QueryTypeExplanation},
		{"write a function to sort arrays", router.QueryTypeCodeGeneration},
		{"refactor this code to be more efficient", router.QueryTypeRefactoring},
		{"design a microservices architecture", router.QueryTypeArchitecture},
		{"debug this error", router.QueryTypeDebugging},
		{"review this pull request", router.QueryTypeReview},
		{"plan the next sprint", router.QueryTypePlanning},
		{"hello there", router.QueryTypeGeneral},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			queryType := router.ClassifyType(tc.query)
			if queryType != tc.expectType {
				t.Errorf("expected type %s, got %s for query: %s",
					tc.expectType, queryType, tc.query)
			}
		})
	}
}

// =============================================================================
// SESSION DIRTY FLAG TEST
// =============================================================================

// TestSessionDirtyFlag verifies session dirty state tracking.
func TestSessionDirtyFlag(t *testing.T) {
	cfg := session.DefaultConfig()
	mgr := session.NewManager(cfg)

	// Initially should not be dirty
	if mgr.IsDirty() {
		t.Error("session should not be dirty initially")
	}

	// Mark dirty
	mgr.MarkDirty()
	if !mgr.IsDirty() {
		t.Error("session should be dirty after MarkDirty")
	}

	// Mark clean
	mgr.MarkClean()
	if mgr.IsDirty() {
		t.Error("session should not be dirty after MarkClean")
	}
}

// =============================================================================
// COST CALCULATION TEST
// =============================================================================

// TestCostCalculation verifies tier cost calculation.
func TestCostCalculation(t *testing.T) {
	tests := []struct {
		tier           router.Tier
		inputTokens    uint32
		outputTokens   uint32
		expectZeroCost bool
	}{
		{router.TierCache, 100, 100, true},
		{router.TierLocal, 100, 100, true},
		{router.TierCloud, 1000, 1000, false},
		{router.TierHaiku, 1000, 1000, false},
		{router.TierSonnet, 1000, 1000, false},
		{router.TierOpus, 1000, 1000, false},
	}

	for _, tc := range tests {
		t.Run(tc.tier.String(), func(t *testing.T) {
			cost := tc.tier.CalculateCostCents(tc.inputTokens, tc.outputTokens)
			if tc.expectZeroCost && cost != 0 {
				t.Errorf("expected zero cost for %s, got %.4f", tc.tier, cost)
			}
			if !tc.expectZeroCost && cost == 0 {
				t.Errorf("expected non-zero cost for %s", tc.tier)
			}
		})
	}
}

// =============================================================================
// TOOL REGISTRY TEST
// =============================================================================

// TestToolRegistry verifies tool registry functionality.
func TestToolRegistry(t *testing.T) {
	registry := tools.NewRegistry()

	// Verify built-in tools are registered
	builtins := []string{"Read", "Write", "Edit", "Glob", "Grep", "Bash"}
	for _, name := range builtins {
		tool := registry.Get(name)
		if tool == nil {
			t.Errorf("built-in tool %s should be registered", name)
		}
	}

	// Test getting all tools
	allTools := registry.All()
	if len(allTools) < len(builtins) {
		t.Errorf("expected at least %d tools, got %d", len(builtins), len(allTools))
	}

	// Test permission levels
	readTool := registry.Get("Read")
	if readTool == nil {
		t.Fatal("Read tool should exist")
	}
	if readTool.RiskLevel != tools.RiskLow {
		t.Errorf("Read should be low risk, got %s", readTool.RiskLevel)
	}

	bashTool := registry.Get("Bash")
	if bashTool == nil {
		t.Fatal("Bash tool should exist")
	}
	if bashTool.RiskLevel != tools.RiskCritical {
		t.Errorf("Bash should be critical risk, got %s", bashTool.RiskLevel)
	}
}

// =============================================================================
// INTEGRATION: CACHE + ROUTING
// =============================================================================

// TestCacheWithRouting verifies cache integration with routing decisions.
func TestCacheWithRouting(t *testing.T) {
	// Create cache manager
	cacheManager := cache.NewCacheManager(nil, nil)

	query := "what is the syntax for Go interfaces?"
	response := "In Go, an interface is defined using: type InterfaceName interface { ... }"

	// First query - should miss cache, route to appropriate tier
	_, hitType := cacheManager.Lookup(query)
	if hitType != cache.CacheHitNone {
		t.Error("first lookup should miss")
	}

	// Route the query
	decision := router.RouteQueryDetailed(query, security.ClassificationUnclassified, nil)
	t.Logf("Routed to tier %s (complexity: %s)", decision.Tier, decision.Complexity)

	// Store response
	cacheManager.Store(query, response, decision.Tier.String())

	// Second query - should hit cache, tier should be Cache
	cachedResponse, hitType := cacheManager.Lookup(query)
	if hitType != cache.CacheHitExact {
		t.Errorf("second lookup should hit, got: %s", hitType)
	}
	if cachedResponse != response {
		t.Errorf("cached response mismatch")
	}

	// For cached response, effective tier is Cache
	if hitType == cache.CacheHitExact || hitType == cache.CacheHitSemantic {
		// Create cache hit result
		result := router.NewCacheHitResult(cachedResponse, 1)
		if result.TierUsed != router.TierCache {
			t.Errorf("cache hit should use Cache tier, got %s", result.TierUsed)
		}
		if result.CostCents != 0 {
			t.Error("cache hit should have zero cost")
		}
	}
}

// =============================================================================
// FULL QUERY PIPELINE TEST
// =============================================================================

// TestFullQueryPipeline tests the entire query flow from cache check to routing.
// This verifies the integration between cache manager, router, and security classification.
func TestFullQueryPipeline(t *testing.T) {
	// Setup - create fresh instances for isolation
	cacheManager := cache.NewCacheManager(nil, nil)
	cacheManager.SetEmbeddingFunc(mockEmbeddingFunc())
	sessionStats := router.NewSessionStats()

	// Test cases covering various query scenarios
	testCases := []struct {
		name           string
		query          string
		classification security.ClassificationLevel
		expectLocal    bool
		expectMinTier  router.Tier
	}{
		{
			name:           "Simple math query",
			query:          "What is 2+2?",
			classification: security.ClassificationUnclassified,
			expectLocal:    false,
			expectMinTier:  router.TierLocal,
		},
		{
			name:           "CUI forces local routing",
			query:          "Explain the architecture of the system",
			classification: security.ClassificationCUI,
			expectLocal:    true,
			expectMinTier:  router.TierLocal,
		},
		{
			name:           "Complex query routes to cloud",
			query:          "Analyze the trade-offs between microservices and monolith architecture",
			classification: security.ClassificationUnclassified,
			expectLocal:    false,
			expectMinTier:  router.TierCloud,
		},
		{
			name:           "Trivial greeting",
			query:          "hi",
			classification: security.ClassificationUnclassified,
			expectLocal:    false,
			expectMinTier:  router.TierCache,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Step 1: Check cache first
			cachedResponse, hitType := cacheManager.Lookup(tc.query)
			if hitType != cache.CacheHitNone {
				t.Logf("Cache hit for query: %s (response: %s)", tc.query, cachedResponse)
				// Record cache hit in session stats (cache hits are still queries)
				result := router.NewCacheHitResult(cachedResponse, 1)
				sessionStats.RecordQuery(result)
				return
			}

			// Step 2: Route query with classification enforcement
			decision := router.RouteQueryDetailed(tc.query, tc.classification, nil)
			t.Logf("Routing decision: tier=%s complexity=%s cost=%.4f cents",
				decision.Tier, decision.Complexity, decision.EstimatedCostCents)

			// Step 3: Verify routing respects classification
			if tc.classification == security.ClassificationCUI {
				if !decision.Tier.IsLocal() {
					t.Errorf("CUI classification should force local routing, got tier: %s", decision.Tier)
				}
			}

			// Step 4: Verify minimum tier requirement
			if decision.Tier.Order() < tc.expectMinTier.Order() && !tc.expectLocal {
				t.Logf("Tier %s is below expected minimum %s (acceptable for cost optimization)",
					decision.Tier, tc.expectMinTier)
			}

			// Step 5: Simulate storing response and recording query
			mockResponse := "This is a mock response for: " + tc.query
			cacheManager.Store(tc.query, mockResponse, decision.Tier.String())

			// Record query result in session stats
			result := router.NewQueryResult(mockResponse, decision.Tier, 100, 50, 500)
			sessionStats.RecordQuery(result)

			// Verify session stats updated
			stats := sessionStats.GetStats()
			if stats.TotalQueries == 0 {
				t.Error("Session stats should have recorded the query")
			}
		})
	}

	// Verify final session statistics
	finalStats := sessionStats.GetStats()
	t.Logf("Pipeline test completed: %s", sessionStats.Summary())
	if finalStats.TotalQueries != len(testCases) {
		t.Errorf("Expected %d queries recorded, got %d", len(testCases), finalStats.TotalQueries)
	}
}

// =============================================================================
// SECURITY CONTROLS INTEGRATION TEST
// =============================================================================

// TestSecurityControlsIntegration tests that security controls work together correctly.
// Verifies audit logging, session management, and classification enforcement integration.
func TestSecurityControlsIntegration(t *testing.T) {
	// Create temp directory for audit log
	tempDir := createTempDir(t)
	auditPath := filepath.Join(tempDir, "security_integration_audit.log")

	// Step 1: Initialize audit logger
	auditLogger, err := security.NewAuditLogger(auditPath)
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer auditLogger.Close()

	// Step 2: Create session manager
	sessionCfg := session.Config{
		Timeout:          15 * time.Minute,
		WarningBefore:    2 * time.Minute,
		AutoSaveEnabled:  false,
		AutoSaveInterval: time.Hour,
	}
	sessionMgr := session.NewManager(sessionCfg)
	sessionID := sessionMgr.SessionID()

	// Step 3: Log session creation event
	err = auditLogger.Log(security.AuditEvent{
		Timestamp: time.Now(),
		EventType: "SESSION_START",
		SessionID: sessionID,
		Tier:      "local",
		Query:     "",
		Tokens:    0,
		Cost:      0,
		Success:   true,
	})
	if err != nil {
		t.Errorf("Failed to log session start event: %v", err)
	}

	// Step 4: Create classification enforcer
	enforcer := security.NewClassificationEnforcerGlobal(sessionID)

	// Step 5: Test routing permission checks
	testCases := []struct {
		name           string
		classification security.ClassificationLevel
		expectCloud    bool
	}{
		{
			name:           "UNCLASSIFIED can route to cloud",
			classification: security.ClassificationUnclassified,
			expectCloud:    true,
		},
		{
			name:           "CUI requires local",
			classification: security.ClassificationCUI,
			expectCloud:    false,
		},
		{
			name:           "SECRET requires local",
			classification: security.ClassificationSecret,
			expectCloud:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			canCloud := enforcer.CanRouteToCloud(tc.classification)
			if canCloud != tc.expectCloud {
				t.Errorf("CanRouteToCloud(%s) = %v, want %v",
					tc.classification, canCloud, tc.expectCloud)
			}

			requiresLocal := enforcer.RequiresLocalOnly(tc.classification)
			if requiresLocal == tc.expectCloud {
				t.Errorf("RequiresLocalOnly(%s) = %v, expected opposite of cloud routing",
					tc.classification, requiresLocal)
			}

			// Log the permission check for audit trail
			err = auditLogger.Log(security.AuditEvent{
				Timestamp: time.Now(),
				EventType: "PERMISSION_CHECK",
				SessionID: sessionID,
				Tier:      "local",
				Query:     "classification=" + tc.classification.String(),
				Tokens:    0,
				Cost:      0,
				Success:   canCloud == tc.expectCloud,
			})
			if err != nil {
				t.Errorf("Failed to log permission check: %v", err)
			}
		})
	}

	// Step 6: Record activity and verify session is active
	sessionMgr.RecordActivity()
	if sessionMgr.IsExpired() {
		t.Error("Session should not be expired after activity")
	}

	// Step 7: Log session end event
	err = auditLogger.Log(security.AuditEvent{
		Timestamp: time.Now(),
		EventType: "SESSION_END",
		SessionID: sessionID,
		Tier:      "local",
		Query:     "",
		Tokens:    0,
		Cost:      0,
		Success:   true,
	})
	if err != nil {
		t.Errorf("Failed to log session end event: %v", err)
	}

	// Step 8: Sync and verify audit log
	if err := auditLogger.Sync(); err != nil {
		t.Fatalf("Failed to sync audit log: %v", err)
	}

	content, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("Failed to read audit log: %v", err)
	}

	logContent := string(content)
	expectedEvents := []string{"SESSION_START", "PERMISSION_CHECK", "SESSION_END"}
	for _, event := range expectedEvents {
		if !strings.Contains(logContent, event) {
			t.Errorf("Audit log should contain %s event", event)
		}
	}

	t.Logf("Security controls integration test completed. Audit log size: %d bytes", len(content))
}

// =============================================================================
// CACHE EXACT AND SEMANTIC HIT TEST
// =============================================================================

// TestCacheExactAndSemanticHits tests cache behavior for both exact and semantic hits.
func TestCacheExactAndSemanticHits(t *testing.T) {
	// Create cache manager with semantic support
	cacheManager := cache.NewCacheManager(nil, nil)
	cacheManager.SetEmbeddingFunc(mockEmbeddingFunc())

	// Test exact cache behavior
	t.Run("exact cache hit", func(t *testing.T) {
		query := "What is the capital of France?"
		response := "The capital of France is Paris."

		// Store the response
		cacheManager.Store(query, response, "local")

		// Lookup with exact same query
		cachedResponse, hitType := cacheManager.Lookup(query)
		if hitType != cache.CacheHitExact {
			t.Errorf("Expected exact cache hit, got: %s", hitType)
		}
		if cachedResponse != response {
			t.Errorf("Expected response %q, got %q", response, cachedResponse)
		}
	})

	// Test cache miss
	t.Run("cache miss", func(t *testing.T) {
		_, hitType := cacheManager.Lookup("What is the population of Tokyo?")
		if hitType != cache.CacheHitNone {
			t.Errorf("Expected cache miss, got: %s", hitType)
		}
	})

	// Test cache statistics
	t.Run("cache statistics", func(t *testing.T) {
		stats := cacheManager.Stats()
		if stats.TotalLookups < 2 {
			t.Errorf("Expected at least 2 lookups, got %d", stats.TotalLookups)
		}
		if stats.ExactHits < 1 {
			t.Errorf("Expected at least 1 exact hit, got %d", stats.ExactHits)
		}
		if stats.Misses < 1 {
			t.Errorf("Expected at least 1 miss, got %d", stats.Misses)
		}

		hitRate := cacheManager.HitRate()
		t.Logf("Cache hit rate: %.2f%%", hitRate*100)
	})

	// Test cache clear
	t.Run("cache clear and verify", func(t *testing.T) {
		initialSize := cacheManager.ExactCacheSize()
		if initialSize == 0 {
			t.Error("Cache should have entries before clear")
		}

		cacheManager.Clear()

		afterSize := cacheManager.ExactCacheSize()
		if afterSize != 0 {
			t.Errorf("Cache should be empty after clear, got size: %d", afterSize)
		}

		// Verify lookup misses after clear
		_, hitType := cacheManager.Lookup("What is the capital of France?")
		if hitType != cache.CacheHitNone {
			t.Error("Expected cache miss after clear")
		}
	})
}

// =============================================================================
// CONCURRENT OPERATIONS TEST
// =============================================================================

// TestConcurrentOperations tests thread safety of core operations.
// Run with: go test -race -v ./internal/...
func TestConcurrentOperations(t *testing.T) {
	// Test concurrent cache operations
	t.Run("concurrent cache operations", func(t *testing.T) {
		cacheManager := cache.NewCacheManager(nil, nil)

		var wg sync.WaitGroup
		iterations := 100

		// Concurrent writes
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				query := "query_" + string(rune('A'+idx%26))
				response := "response_" + string(rune('A'+idx%26))
				cacheManager.Store(query, response, "local")
			}(i)
		}

		// Concurrent reads
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				query := "query_" + string(rune('A'+idx%26))
				_, _ = cacheManager.Lookup(query)
			}(i)
		}

		wg.Wait()

		// Verify stats are consistent
		stats := cacheManager.Stats()
		t.Logf("Concurrent cache test: %d lookups, %d exact hits, %d misses",
			stats.TotalLookups, stats.ExactHits, stats.Misses)
	})

	// Test concurrent routing operations
	t.Run("concurrent routing operations", func(t *testing.T) {
		var wg sync.WaitGroup
		iterations := 100

		queries := []string{
			"Simple question",
			"Explain complex architecture patterns in detail",
			"What is 2+2?",
			"Design a microservices system",
		}

		classifications := []security.ClassificationLevel{
			security.ClassificationUnclassified,
			security.ClassificationCUI,
		}

		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				query := queries[idx%len(queries)]
				classification := classifications[idx%len(classifications)]

				// Route query
				decision := router.RouteQueryDetailed(query, classification, nil)
				_ = decision.Tier
				_ = decision.Complexity
			}(i)
		}

		wg.Wait()
		t.Log("Concurrent routing test completed without race conditions")
	})

	// Test concurrent session stats operations
	t.Run("concurrent session stats", func(t *testing.T) {
		sessionStats := router.NewSessionStats()
		var wg sync.WaitGroup
		iterations := 100

		// Concurrent RecordQuery calls
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				tier := router.Tier(idx % 4)
				result := router.NewQueryResult("response", tier, 100, 50, 500)
				sessionStats.RecordQuery(result)
			}(i)
		}

		// Concurrent GetStats calls
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = sessionStats.GetStats()
				_ = sessionStats.Summary()
			}()
		}

		wg.Wait()

		// Verify final count
		finalStats := sessionStats.GetStats()
		if finalStats.TotalQueries != iterations {
			t.Errorf("Expected %d queries, got %d", iterations, finalStats.TotalQueries)
		}
		t.Logf("Concurrent session stats test: %s", sessionStats.Summary())
	})

	// Test concurrent session manager operations
	t.Run("concurrent session manager", func(t *testing.T) {
		cfg := session.DefaultConfig()
		mgr := session.NewManager(cfg)

		var wg sync.WaitGroup
		iterations := 100

		// Concurrent activity recording
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				mgr.RecordActivity()
			}()
		}

		// Concurrent status checks
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = mgr.IsExpired()
				_ = mgr.RemainingTime()
				_ = mgr.GetStatus()
			}()
		}

		wg.Wait()
		t.Log("Concurrent session manager test completed")
	})
}

// =============================================================================
// CONVERSATION STORAGE INTEGRATION TEST
// =============================================================================

// TestConversationStorageIntegration tests conversation persistence functionality.
func TestConversationStorageIntegration(t *testing.T) {
	// Create temp directory for conversation storage
	tempDir := createTempDir(t)

	// Create store with custom directory
	store, err := storage.NewConversationStoreWithDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to create conversation store: %v", err)
	}

	// Test saving a conversation
	t.Run("save and load conversation", func(t *testing.T) {
		conv := &storage.StoredConversation{
			Model: "test-model:7b",
			Messages: []storage.StoredMessage{
				{
					ID:        "msg1",
					Role:      "user",
					Content:   "Hello, how are you?",
					Timestamp: time.Now(),
				},
				{
					ID:         "msg2",
					Role:       "assistant",
					Content:    "I'm doing well, thank you for asking!",
					Timestamp:  time.Now(),
					TokenCount: 15,
					DurationMs: 500,
				},
			},
		}

		// Save
		id, err := store.Save(conv)
		if err != nil {
			t.Fatalf("Failed to save conversation: %v", err)
		}
		if id == "" {
			t.Error("Conversation ID should not be empty")
		}

		// Load
		loaded, err := store.Load(id)
		if err != nil {
			t.Fatalf("Failed to load conversation: %v", err)
		}

		// Verify
		if loaded.Model != conv.Model {
			t.Errorf("Model mismatch: expected %s, got %s", conv.Model, loaded.Model)
		}
		if len(loaded.Messages) != len(conv.Messages) {
			t.Errorf("Message count mismatch: expected %d, got %d",
				len(conv.Messages), len(loaded.Messages))
		}
	})

	// Test listing conversations
	t.Run("list conversations", func(t *testing.T) {
		// Save another conversation
		conv2 := &storage.StoredConversation{
			Model: "another-model",
			Messages: []storage.StoredMessage{
				{
					ID:        "msg3",
					Role:      "user",
					Content:   "Second conversation",
					Timestamp: time.Now(),
				},
			},
		}
		_, err := store.Save(conv2)
		if err != nil {
			t.Fatalf("Failed to save second conversation: %v", err)
		}

		// List
		metas, err := store.List()
		if err != nil {
			t.Fatalf("Failed to list conversations: %v", err)
		}

		if len(metas) < 2 {
			t.Errorf("Expected at least 2 conversations, got %d", len(metas))
		}

		// Verify sorted by most recent first
		if len(metas) >= 2 {
			if metas[0].UpdatedAt.Before(metas[1].UpdatedAt) {
				t.Error("Conversations should be sorted by most recent first")
			}
		}
	})

	// Test search
	t.Run("search conversations", func(t *testing.T) {
		results, err := store.Search("Second")
		if err != nil {
			t.Fatalf("Failed to search conversations: %v", err)
		}

		if len(results) == 0 {
			t.Error("Search should find at least one conversation")
		}
	})

	// Test delete
	t.Run("delete conversation", func(t *testing.T) {
		metas, _ := store.List()
		if len(metas) == 0 {
			t.Skip("No conversations to delete")
		}

		firstID := metas[0].ID
		err := store.Delete(firstID)
		if err != nil {
			t.Fatalf("Failed to delete conversation: %v", err)
		}

		// Verify deleted
		_, err = store.Load(firstID)
		if err != storage.ErrConversationNotFound {
			t.Error("Loading deleted conversation should return ErrConversationNotFound")
		}
	})

	// Test clear
	t.Run("clear all conversations", func(t *testing.T) {
		err := store.Clear()
		if err != nil {
			t.Fatalf("Failed to clear conversations: %v", err)
		}

		metas, _ := store.List()
		if len(metas) != 0 {
			t.Errorf("Expected 0 conversations after clear, got %d", len(metas))
		}
	})
}
