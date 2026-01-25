// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package internal contains comprehensive race detection tests for the rigrun TUI.
//
// Run with: go test -race -v ./internal/...
//
// These tests are designed to detect data races under concurrent access patterns
// that match real-world usage scenarios. They should be run as part of CI
// with the -race flag enabled.
package internal

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// TEST CONFIGURATION
// =============================================================================

const (
	// Number of concurrent goroutines for race tests
	raceConcurrency = 100
	// Number of iterations per goroutine
	raceIterations = 50
	// Timeout for race tests
	raceTimeout = 30 * time.Second
)

// =============================================================================
// CONFIG CONCURRENCY TESTS
// =============================================================================

// TestConcurrency_ConfigGlobalAccess tests concurrent access to the global config singleton.
// This is critical as config is accessed from multiple goroutines in the TUI.
func TestConcurrency_ConfigGlobalAccess(t *testing.T) {
	// Reset global config for clean test state
	config.ResetGlobalForTesting()

	ctx, cancel := context.WithTimeout(context.Background(), raceTimeout)
	defer cancel()

	var wg sync.WaitGroup
	errChan := make(chan error, raceConcurrency*2)

	// Launch concurrent readers
	for i := 0; i < raceConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < raceIterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				cfg := config.Global()
				if cfg == nil {
					errChan <- nil // Config can legitimately be nil before init
					continue
				}
				// Read various fields to ensure no race on reads
				_ = cfg.DefaultModel
				_ = cfg.Routing.DefaultMode
				_ = cfg.Routing.ParanoidMode
				_ = cfg.Security.Classification
				_ = cfg.Local.OllamaURL
			}
		}()
	}

	// Launch concurrent writers (SetGlobal)
	for i := 0; i < raceConcurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < raceIterations/10; j++ { // Fewer writes than reads
				select {
				case <-ctx.Done():
					return
				default:
				}
				newCfg := &config.Config{
					DefaultModel: "test-model",
					Routing: config.RoutingConfig{
						DefaultMode:  "auto",
						ParanoidMode: idx%2 == 0,
					},
					Security: config.SecurityConfig{
						Classification: "UNCLASSIFIED",
					},
				}
				config.SetGlobal(newCfg)
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			t.Errorf("Unexpected error during concurrent config access: %v", err)
		}
	}
}

// TestConcurrency_ConfigReload tests concurrent config reloads.
func TestConcurrency_ConfigReload(t *testing.T) {
	config.ResetGlobalForTesting()

	ctx, cancel := context.WithTimeout(context.Background(), raceTimeout)
	defer cancel()

	var wg sync.WaitGroup
	var reloadCount int64

	// Launch concurrent reloads (these may fail if config file doesn't exist, that's OK)
	for i := 0; i < raceConcurrency/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < raceIterations/5; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				_ = config.ReloadGlobal() // Ignore errors, just testing for races
				atomic.AddInt64(&reloadCount, 1)
			}
		}()
	}

	// Concurrent readers while reloading
	for i := 0; i < raceConcurrency/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < raceIterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				_ = config.Global()
			}
		}()
	}

	wg.Wait()
	t.Logf("Completed %d concurrent reloads", atomic.LoadInt64(&reloadCount))
}

// TestConcurrency_ConfigGetSet tests concurrent Get/Set operations on config values.
func TestConcurrency_ConfigGetSet(t *testing.T) {
	config.ResetGlobalForTesting()

	// Initialize with a valid config
	cfg := config.Default()
	config.SetGlobal(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), raceTimeout)
	defer cancel()

	var wg sync.WaitGroup

	keys := []string{
		"routing.default_mode",
		"routing.paranoid_mode",
		"security.classification",
		"local.ollama_url",
	}

	// Concurrent getters
	for i := 0; i < raceConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < raceIterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				cfg := config.Global()
				if cfg == nil {
					continue
				}
				for _, key := range keys {
					_, _ = cfg.Get(key)
				}
			}
		}()
	}

	// Concurrent setters
	for i := 0; i < raceConcurrency/5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < raceIterations/5; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				cfg := config.Global()
				if cfg == nil {
					continue
				}
				_ = cfg.Set("routing.default_mode", "local")
				_ = cfg.Set("routing.paranoid_mode", true)
			}
		}()
	}

	wg.Wait()
}

// =============================================================================
// ROUTER CONCURRENCY TESTS
// =============================================================================

// TestConcurrency_RouteQuery tests concurrent query routing.
func TestConcurrency_RouteQuery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), raceTimeout)
	defer cancel()

	var wg sync.WaitGroup
	queries := []string{
		"Hello, how are you?",
		"Explain quantum computing in detail with examples",
		"What is 2+2?",
		"Write a comprehensive analysis of machine learning algorithms",
		"Simple greeting",
	}

	classifications := []security.ClassificationLevel{
		security.ClassificationUnclassified,
		security.ClassificationCUI,
		security.ClassificationSecret,
	}

	var routedCount int64

	for i := 0; i < raceConcurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < raceIterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				query := queries[idx%len(queries)]
				classification := classifications[j%len(classifications)]
				paranoid := idx%3 == 0

				tier := router.RouteQuery(query, classification, paranoid, nil)
				_ = tier // Use the result to prevent optimization
				atomic.AddInt64(&routedCount, 1)
			}
		}(i)
	}

	wg.Wait()
	t.Logf("Routed %d queries concurrently", atomic.LoadInt64(&routedCount))
}

// TestConcurrency_RouteQueryDetailed tests concurrent detailed query routing.
func TestConcurrency_RouteQueryDetailed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), raceTimeout)
	defer cancel()

	var wg sync.WaitGroup
	var routedCount int64

	for i := 0; i < raceConcurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < raceIterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				query := "Test query for detailed routing analysis"
				classification := security.ClassificationUnclassified

				opts := &router.RouterOptions{
					Paranoid:        idx%2 == 0,
					Mode:            "auto",
					AutoPreferLocal: true,
				}

				decision := router.RouteQueryDetailed(query, classification, opts)
				_ = decision.Tier
				_ = decision.Reason
				atomic.AddInt64(&routedCount, 1)
			}
		}(i)
	}

	wg.Wait()
	t.Logf("Made %d detailed routing decisions concurrently", atomic.LoadInt64(&routedCount))
}

// TestConcurrency_ClassifyComplexity tests concurrent complexity classification.
func TestConcurrency_ClassifyComplexity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), raceTimeout)
	defer cancel()

	var wg sync.WaitGroup
	queries := []string{
		"Hi",
		"What is the weather?",
		"Explain the intricacies of neural network architectures including transformers, CNNs, and RNNs with mathematical foundations",
		"2+2",
		"Write a detailed essay about climate change with references",
	}

	for i := 0; i < raceConcurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < raceIterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				query := queries[(idx+j)%len(queries)]
				complexity := router.ClassifyComplexity(query)
				_ = complexity.String()
			}
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// SECURITY CONCURRENCY TESTS
// =============================================================================

// TestConcurrency_AuditLogger tests concurrent audit logging.
func TestConcurrency_AuditLogger(t *testing.T) {
	// Initialize audit logger (may already be initialized)
	_ = security.InitGlobalAuditLogger("", false) // Use temp path, disabled

	ctx, cancel := context.WithTimeout(context.Background(), raceTimeout)
	defer cancel()

	var wg sync.WaitGroup
	var logCount int64

	// Concurrent logging
	for i := 0; i < raceConcurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			logger := security.GlobalAuditLogger()
			if logger == nil {
				return
			}
			for j := 0; j < raceIterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				// Log various event types
				logger.LogEvent("test-session", "QUERY", map[string]string{
					"query": "test query",
					"tier":  "local",
				})
				atomic.AddInt64(&logCount, 1)
			}
		}(i)
	}

	wg.Wait()
	t.Logf("Logged %d audit events concurrently", atomic.LoadInt64(&logCount))
}

// TestConcurrency_ClassificationEnforcer tests concurrent classification enforcement.
func TestConcurrency_ClassificationEnforcer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), raceTimeout)
	defer cancel()

	var wg sync.WaitGroup

	// Create a shared enforcer
	enforcer := security.NewClassificationEnforcerGlobal("test-session")

	for i := 0; i < raceConcurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < raceIterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Concurrent read/check operations using actual API
				_ = enforcer.IsEnabled()
				_ = enforcer.CanRouteToCloud(security.ClassificationUnclassified)
				_ = enforcer.RequiresLocalOnly(security.ClassificationCUI)
			}
		}(i)
	}

	// Some goroutines check routing
	for i := 0; i < raceConcurrency/10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < raceIterations/10; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				_, _ = enforcer.EnforceRouting(security.ClassificationCUI, security.RoutingTierLocal)
			}
		}()
	}

	wg.Wait()
}

// TestConcurrency_LockoutManager tests concurrent lockout operations.
func TestConcurrency_LockoutManager(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), raceTimeout)
	defer cancel()

	var wg sync.WaitGroup
	manager := security.GlobalLockoutManager()
	if manager == nil {
		t.Skip("LockoutManager not initialized")
	}

	users := []string{"user1", "user2", "user3", "user4", "user5"}

	// Concurrent login attempts
	for i := 0; i < raceConcurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < raceIterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				user := users[idx%len(users)]
				_ = manager.IsLocked(user)
				_ = manager.GetStatus(user)
			}
		}(i)
	}

	// Some goroutines record failures
	for i := 0; i < raceConcurrency/5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < raceIterations/10; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				user := users[idx%len(users)]
				_ = manager.RecordAttempt(user, false)
			}
		}(i)
	}

	// Some goroutines record successes (resets)
	for i := 0; i < raceConcurrency/10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < raceIterations/10; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				user := users[idx%len(users)]
				_ = manager.RecordAttempt(user, true)
			}
		}(i)
	}

	wg.Wait()
}

// TestConcurrency_AuthManager tests concurrent authentication operations.
func TestConcurrency_AuthManager(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), raceTimeout)
	defer cancel()

	var wg sync.WaitGroup
	manager := security.GlobalAuthManager()
	if manager == nil {
		t.Skip("AuthManager not initialized")
	}

	// Concurrent session validations
	for i := 0; i < raceConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < raceIterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				_ = manager.GetStats()
				_ = manager.ListSessions()
			}
		}()
	}

	wg.Wait()
}

// =============================================================================
// COMBINED STRESS TESTS
// =============================================================================

// TestConcurrency_AllComponentsUnderLoad runs all components concurrently to detect
// cross-component race conditions.
func TestConcurrency_AllComponentsUnderLoad(t *testing.T) {
	config.ResetGlobalForTesting()
	cfg := config.Default()
	config.SetGlobal(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), raceTimeout*2)
	defer cancel()

	var wg sync.WaitGroup

	// Config access
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < raceIterations*10; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			cfg := config.Global()
			if cfg != nil {
				_ = cfg.DefaultModel
				_ = cfg.Routing.ParanoidMode
			}
		}
	}()

	// Router operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < raceIterations*10; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			tier := router.RouteQuery("test query", security.ClassificationUnclassified, false, nil)
			_ = tier
		}
	}()

	// Classification operations
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < raceIterations*10; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			complexity := router.ClassifyComplexity("test query")
			_ = complexity
		}
	}()

	// Audit logging
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger := security.GlobalAuditLogger()
		if logger == nil {
			return
		}
		for i := 0; i < raceIterations*5; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			logger.LogEvent("stress-test", "QUERY", nil)
		}
	}()

	wg.Wait()
	t.Log("All components completed under concurrent load")
}

// TestConcurrency_RapidConfigChanges tests the system's behavior under rapid config changes.
func TestConcurrency_RapidConfigChanges(t *testing.T) {
	config.ResetGlobalForTesting()

	ctx, cancel := context.WithTimeout(context.Background(), raceTimeout)
	defer cancel()

	var wg sync.WaitGroup
	var changeCount int64

	// Rapid config changes
	for i := 0; i < raceConcurrency/2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < raceIterations; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				newCfg := config.Default()
				newCfg.Routing.ParanoidMode = j%2 == 0
				newCfg.Routing.DefaultMode = []string{"auto", "local", "cloud"}[j%3]
				config.SetGlobal(newCfg)
				atomic.AddInt64(&changeCount, 1)
			}
		}(i)
	}

	// Concurrent readers relying on config
	for i := 0; i < raceConcurrency/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < raceIterations*2; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				cfg := config.Global()
				if cfg == nil {
					continue
				}
				// Simulate router checking config
				paranoid := cfg.Routing.ParanoidMode
				tier := router.RouteQuery("test", security.ClassificationUnclassified, paranoid, nil)
				_ = tier
			}
		}()
	}

	wg.Wait()
	t.Logf("Completed %d rapid config changes with concurrent readers", atomic.LoadInt64(&changeCount))
}

// =============================================================================
// BENCHMARK TESTS FOR CONCURRENCY OVERHEAD
// =============================================================================

// BenchmarkConcurrent_ConfigGlobal benchmarks concurrent config access.
func BenchmarkConcurrent_ConfigGlobal(b *testing.B) {
	config.ResetGlobalForTesting()
	cfg := config.Default()
	config.SetGlobal(cfg)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c := config.Global()
			_ = c.DefaultModel
		}
	})
}

// BenchmarkConcurrent_RouteQuery benchmarks concurrent query routing.
func BenchmarkConcurrent_RouteQuery(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tier := router.RouteQuery("test query", security.ClassificationUnclassified, false, nil)
			_ = tier
		}
	})
}

// BenchmarkConcurrent_ClassifyComplexity benchmarks concurrent complexity classification.
func BenchmarkConcurrent_ClassifyComplexity(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			complexity := router.ClassifyComplexity("Explain quantum computing")
			_ = complexity
		}
	})
}
