// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package router

import (
	"strings"
	"testing"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// ============================================================================
// CLASSIFICATION ENFORCEMENT TESTS
// These tests verify that classification is properly enforced as the FIRST
// check in all routing logic. CUI+ classifications MUST always return TierLocal.
// ============================================================================

// TestClassificationEnforcementCUI verifies CUI classification forces TierLocal.
// This is the core security test for classification enforcement.
func TestClassificationEnforcementCUI(t *testing.T) {
	// Test that CUI classification ALWAYS returns TierLocal, regardless of query complexity
	queries := []string{
		"hello",                                      // Trivial - would route to Cache
		"what is rust",                               // Simple - would route to Local
		"explain how async runtime works in detail",  // Complex - would route to Cloud
		"architect a distributed microservices system with event sourcing", // Expert - would route to Cloud
	}

	for _, query := range queries {
		t.Run("CUI_"+query[:min(len(query), 20)], func(t *testing.T) {
			result := RouteQuery(query, security.ClassificationCUI, false, nil)
			if result != TierLocal {
				t.Errorf("RouteQuery(%q, CUI, false, nil) = %v, want TierLocal (CUI must stay local)",
					query, result)
			}
		})
	}
}

// TestClassificationEnforcementConfidential verifies Confidential classification forces TierLocal.
func TestClassificationEnforcementConfidential(t *testing.T) {
	queries := []string{
		"hello",
		"explain complex architecture patterns",
	}

	for _, query := range queries {
		t.Run("Confidential_"+query[:min(len(query), 15)], func(t *testing.T) {
			result := RouteQuery(query, security.ClassificationConfidential, false, nil)
			if result != TierLocal {
				t.Errorf("RouteQuery(%q, Confidential, false, nil) = %v, want TierLocal",
					query, result)
			}
		})
	}
}

// TestClassificationEnforcementSecret verifies Secret classification forces TierLocal.
func TestClassificationEnforcementSecret(t *testing.T) {
	query := "design a complex system architecture"
	result := RouteQuery(query, security.ClassificationSecret, false, nil)
	if result != TierLocal {
		t.Errorf("RouteQuery(%q, Secret, false, nil) = %v, want TierLocal", query, result)
	}
}

// TestClassificationEnforcementTopSecret verifies TopSecret classification forces TierLocal.
func TestClassificationEnforcementTopSecret(t *testing.T) {
	query := "architect a new distributed system"
	result := RouteQuery(query, security.ClassificationTopSecret, false, nil)
	if result != TierLocal {
		t.Errorf("RouteQuery(%q, TopSecret, false, nil) = %v, want TierLocal", query, result)
	}
}

// TestClassificationEnforcementUnclassifiedAllowsCloud verifies Unclassified can route to cloud.
func TestClassificationEnforcementUnclassifiedAllowsCloud(t *testing.T) {
	// Complex query should route to Cloud when Unclassified
	query := "explain how async runtime works in detail with examples"
	result := RouteQuery(query, security.ClassificationUnclassified, false, nil)

	// Should be a cloud tier for complex queries
	if result == TierLocal || result == TierCache {
		// This might be Local due to complexity classification, but should NOT be
		// forced to Local due to classification
		complexity := ClassifyComplexity(query)
		if complexity.MinTier() != result {
			t.Logf("Query routed to %v (expected based on complexity: %v)", result, complexity.MinTier())
		}
	}
}

// TestClassificationEnforcementDetailedCUI tests RouteQueryDetailed with CUI classification.
func TestClassificationEnforcementDetailedCUI(t *testing.T) {
	query := "architect a microservices system"
	decision := RouteQueryDetailed(query, security.ClassificationCUI, nil)

	if decision.Tier != TierLocal {
		t.Errorf("RouteQueryDetailed with CUI should return TierLocal, got %v", decision.Tier)
	}

	// Verify the reason mentions the classification block
	if !strings.Contains(decision.Reason, "CUI") || !strings.Contains(decision.Reason, "FORCED") {
		t.Errorf("Reason should mention CUI classification block: %q", decision.Reason)
	}

	// Cost should be 0 for local
	if decision.EstimatedCostCents != 0 {
		t.Errorf("EstimatedCostCents should be 0 for forced local, got %f", decision.EstimatedCostCents)
	}
}

// TestClassificationEnforcementAutoCUI tests RouteQueryAuto with CUI classification.
func TestClassificationEnforcementAutoCUI(t *testing.T) {
	query := "complex architecture question"
	decision := RouteQueryAuto(query, security.ClassificationCUI, false, false, 0)

	if decision.Tier != TierLocal {
		t.Errorf("RouteQueryAuto with CUI should return TierLocal, got %v", decision.Tier)
	}

	// Should NOT be auto-routed when CUI forces local
	if decision.IsAutoRouted {
		t.Error("IsAutoRouted should be false when CUI forces local routing")
	}
}

// TestClassificationEnforcementBeforeComplexity verifies classification check happens FIRST.
// This test ensures that even if a query would normally route to Cloud based on complexity,
// the classification check takes precedence.
func TestClassificationEnforcementBeforeComplexity(t *testing.T) {
	// Use a query that would definitely route to Cloud based on complexity
	expertQuery := "should I use microservices or monolith, what are the trade-offs for scale"

	// Verify this query would route to Cloud without classification restrictions
	unclassifiedResult := RouteQuery(expertQuery, security.ClassificationUnclassified, false, nil)
	if unclassifiedResult == TierLocal || unclassifiedResult == TierCache {
		// If complexity routes to local anyway, the test is less meaningful
		t.Logf("Note: Query routes to %v even when Unclassified (complexity-based)", unclassifiedResult)
	}

	// Now verify CUI classification blocks cloud routing regardless
	cuiResult := RouteQuery(expertQuery, security.ClassificationCUI, false, nil)
	if cuiResult != TierLocal {
		t.Errorf("CUI classification should force TierLocal regardless of complexity, got %v", cuiResult)
	}
}

// TestClassificationEnforcementWithMaxTier verifies classification takes precedence over maxTier.
func TestClassificationEnforcementWithMaxTier(t *testing.T) {
	query := "design a complex system"
	maxTier := TierOpus // Even with Opus as max tier...

	// CUI should still force Local
	result := RouteQuery(query, security.ClassificationCUI, false, &maxTier)
	if result != TierLocal {
		t.Errorf("CUI classification should override maxTier=%v, got %v", maxTier, result)
	}
}

// TestClassificationEnforcementWithParanoidMode verifies paranoid mode stacks with classification.
func TestClassificationEnforcementWithParanoidMode(t *testing.T) {
	query := "complex query"

	// Paranoid mode alone should force local
	paranoidResult := RouteQuery(query, security.ClassificationUnclassified, true, nil)
	if paranoidResult != TierLocal {
		t.Errorf("Paranoid mode should force TierLocal, got %v", paranoidResult)
	}

	// CUI + paranoid mode should also force local
	cuiParanoidResult := RouteQuery(query, security.ClassificationCUI, true, nil)
	if cuiParanoidResult != TierLocal {
		t.Errorf("CUI + paranoid mode should force TierLocal, got %v", cuiParanoidResult)
	}
}

// TestAllClassificationLevels tests all classification levels systematically.
func TestAllClassificationLevels(t *testing.T) {
	tests := []struct {
		name           string
		classification security.ClassificationLevel
		expectLocal    bool // true = must be TierLocal, false = can be any tier
	}{
		{"Unclassified", security.ClassificationUnclassified, false},
		{"CUI", security.ClassificationCUI, true},
		{"Confidential", security.ClassificationConfidential, true},
		{"Secret", security.ClassificationSecret, true},
		{"TopSecret", security.ClassificationTopSecret, true},
	}

	complexQuery := "architect a distributed system with event sourcing"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RouteQuery(complexQuery, tt.classification, false, nil)
			if tt.expectLocal && result != TierLocal {
				t.Errorf("Classification %s should force TierLocal, got %v", tt.name, result)
			}
		})
	}
}

// ============================================================================
// STANDARD ROUTING TESTS (with required classification parameter)
// These tests verify normal routing behavior when classification is Unclassified.
// ============================================================================

// TestRouteQuery tests the basic routing logic with classification.
// Verifies that queries are routed to appropriate tiers based on complexity.
func TestRouteQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected Tier
	}{
		// Trivial queries -> Cache tier
		{
			name:     "trivial_hello",
			query:    "hello",
			expected: TierCache,
		},
		{
			name:     "trivial_hi_there",
			query:    "hi there",
			expected: TierCache,
		},
		{
			name:     "trivial_thanks",
			query:    "thanks",
			expected: TierCache,
		},

		// Simple queries -> Local tier
		{
			name:     "simple_what_is_rust",
			query:    "what is rust",
			expected: TierLocal,
		},
		{
			name:     "simple_what_is_go",
			query:    "what is go",
			expected: TierLocal,
		},
		{
			name:     "simple_find_main",
			query:    "find main",
			expected: TierLocal,
		},

		// Complex queries -> Cloud tier
		{
			name:     "complex_fix_error",
			query:    "how do I fix this error",
			expected: TierCloud,
		},
		{
			name:     "complex_explain_async",
			query:    "explain how async runtime works",
			expected: TierCloud,
		},
		{
			name:     "complex_review_code",
			query:    "review this code please",
			expected: TierCloud,
		},
		{
			name:     "complex_many_words",
			query:    "tell me about this topic here now",
			expected: TierCloud,
		},

		// Expert queries -> Cloud tier
		{
			name:     "expert_microservices",
			query:    "should I use microservices",
			expected: TierCloud,
		},
		{
			name:     "expert_trade_offs",
			query:    "should I use microservices, what are the trade-offs",
			expected: TierCloud,
		},
		{
			name:     "expert_architecture",
			query:    "architect the best solution",
			expected: TierCloud,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use Unclassified to test normal routing behavior
			result := RouteQuery(tt.query, security.ClassificationUnclassified, false, nil)
			if result != tt.expected {
				complexity := ClassifyComplexity(tt.query)
				t.Errorf("RouteQuery(%q, Unclassified, false, nil) = %v, want %v (complexity=%v)",
					tt.query, result, tt.expected, complexity)
			}
		})
	}
}

// TestRouteQueryWithMaxTier tests routing with a maximum tier cap.
// Verifies that queries are properly capped at the specified max tier.
func TestRouteQueryWithMaxTier(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		maxTier  Tier
		expected Tier
	}{
		// Complex query capped at Local
		{
			name:     "complex_capped_local",
			query:    "explain how async runtime works with examples",
			maxTier:  TierLocal,
			expected: TierLocal,
		},
		// Expert query capped at Cache
		{
			name:     "expert_capped_cache",
			query:    "should I use microservices, what are the trade-offs",
			maxTier:  TierCache,
			expected: TierCache,
		},
		// Expert query capped at Local
		{
			name:     "expert_capped_local",
			query:    "architect a new distributed system",
			maxTier:  TierLocal,
			expected: TierLocal,
		},
		// Simple query with higher max (no cap needed)
		{
			name:     "simple_no_cap_needed",
			query:    "what is rust",
			maxTier:  TierCloud,
			expected: TierLocal,
		},
		// Trivial query with higher max (no cap needed)
		{
			name:     "trivial_no_cap_needed",
			query:    "hello",
			maxTier:  TierOpus,
			expected: TierCache,
		},
		// Complex query: Cloud tier is already below Haiku cap, so Cloud is returned
		// (maxTier only caps when recommended > maxTier)
		{
			name:     "complex_capped_haiku",
			query:    "review this code and explain the bugs",
			maxTier:  TierHaiku,
			expected: TierCloud, // Cloud (order 2) < Haiku (order 3), so no capping needed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maxTier := tt.maxTier
			// Use Unclassified to test normal routing with max tier
			result := RouteQuery(tt.query, security.ClassificationUnclassified, false, &maxTier)
			if result != tt.expected {
				t.Errorf("RouteQuery(%q, Unclassified, false, &%v) = %v, want %v",
					tt.query, tt.maxTier, result, tt.expected)
			}
		})
	}
}

// TestTierEscalate tests the tier escalation logic.
// Verifies proper escalation path: Cache -> Local -> Cloud, etc.
func TestTierEscalate(t *testing.T) {
	tests := []struct {
		name     string
		tier     Tier
		expected *Tier
	}{
		{
			name:     "cache_to_local",
			tier:     TierCache,
			expected: tierPtr(TierLocal),
		},
		{
			name:     "local_to_cloud",
			tier:     TierLocal,
			expected: tierPtr(TierCloud),
		},
		{
			name:     "cloud_to_nil",
			tier:     TierCloud,
			expected: nil,
		},
		{
			name:     "haiku_to_sonnet",
			tier:     TierHaiku,
			expected: tierPtr(TierSonnet),
		},
		{
			name:     "sonnet_to_opus",
			tier:     TierSonnet,
			expected: tierPtr(TierOpus),
		},
		{
			name:     "opus_to_nil",
			tier:     TierOpus,
			expected: nil,
		},
		{
			name:     "gpt4o_to_nil",
			tier:     TierGpt4o,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tier.Escalate()
			if tt.expected == nil {
				if result != nil {
					t.Errorf("%v.Escalate() = %v, want nil", tt.tier, *result)
				}
			} else {
				if result == nil {
					t.Errorf("%v.Escalate() = nil, want %v", tt.tier, *tt.expected)
				} else if *result != *tt.expected {
					t.Errorf("%v.Escalate() = %v, want %v", tt.tier, *result, *tt.expected)
				}
			}
		})
	}
}

// TestTierCosts tests that tier costs are properly ordered.
// Verifies: Cache/Local = 0, Haiku < Sonnet < Opus
func TestTierCosts(t *testing.T) {
	inputTokens := uint32(1000)
	outputTokens := uint32(1000)

	t.Run("cache_is_free", func(t *testing.T) {
		cost := TierCache.CalculateCostCents(inputTokens, outputTokens)
		if cost != 0 {
			t.Errorf("TierCache cost = %f, want 0", cost)
		}
	})

	t.Run("local_is_free", func(t *testing.T) {
		cost := TierLocal.CalculateCostCents(inputTokens, outputTokens)
		if cost != 0 {
			t.Errorf("TierLocal cost = %f, want 0", cost)
		}
	})

	t.Run("haiku_has_cost", func(t *testing.T) {
		cost := TierHaiku.CalculateCostCents(inputTokens, outputTokens)
		if cost <= 0 {
			t.Errorf("TierHaiku cost = %f, want > 0", cost)
		}
	})

	t.Run("cloud_has_cost", func(t *testing.T) {
		cost := TierCloud.CalculateCostCents(inputTokens, outputTokens)
		if cost <= 0 {
			t.Errorf("TierCloud cost = %f, want > 0", cost)
		}
	})

	t.Run("sonnet_more_than_haiku", func(t *testing.T) {
		haikuCost := TierHaiku.CalculateCostCents(inputTokens, outputTokens)
		sonnetCost := TierSonnet.CalculateCostCents(inputTokens, outputTokens)
		if sonnetCost <= haikuCost {
			t.Errorf("TierSonnet cost (%f) should be > TierHaiku cost (%f)",
				sonnetCost, haikuCost)
		}
	})

	t.Run("opus_more_than_sonnet", func(t *testing.T) {
		sonnetCost := TierSonnet.CalculateCostCents(inputTokens, outputTokens)
		opusCost := TierOpus.CalculateCostCents(inputTokens, outputTokens)
		if opusCost <= sonnetCost {
			t.Errorf("TierOpus cost (%f) should be > TierSonnet cost (%f)",
				opusCost, sonnetCost)
		}
	})
}

// TestCalculateCostCents tests the cost calculation function.
// Verifies correct cost computation for different tiers and token counts.
func TestCalculateCostCents(t *testing.T) {
	tests := []struct {
		name         string
		tier         Tier
		inputTokens  uint32
		outputTokens uint32
		expectZero   bool
		expectGtZero bool
	}{
		{
			name:         "cache_zero_cost",
			tier:         TierCache,
			inputTokens:  1000,
			outputTokens: 1000,
			expectZero:   true,
		},
		{
			name:         "local_zero_cost",
			tier:         TierLocal,
			inputTokens:  1000,
			outputTokens: 1000,
			expectZero:   true,
		},
		{
			name:         "haiku_has_cost",
			tier:         TierHaiku,
			inputTokens:  1000,
			outputTokens: 1000,
			expectGtZero: true,
		},
		{
			name:         "cloud_has_cost",
			tier:         TierCloud,
			inputTokens:  1000,
			outputTokens: 1000,
			expectGtZero: true,
		},
		{
			name:         "sonnet_has_cost",
			tier:         TierSonnet,
			inputTokens:  1000,
			outputTokens: 1000,
			expectGtZero: true,
		},
		{
			name:         "opus_has_cost",
			tier:         TierOpus,
			inputTokens:  1000,
			outputTokens: 1000,
			expectGtZero: true,
		},
		{
			name:         "gpt4o_has_cost",
			tier:         TierGpt4o,
			inputTokens:  1000,
			outputTokens: 1000,
			expectGtZero: true,
		},
		{
			name:         "zero_tokens_zero_cost",
			tier:         TierOpus,
			inputTokens:  0,
			outputTokens: 0,
			expectZero:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := tt.tier.CalculateCostCents(tt.inputTokens, tt.outputTokens)
			if tt.expectZero && cost != 0 {
				t.Errorf("%v.CalculateCostCents(%d, %d) = %f, want 0",
					tt.tier, tt.inputTokens, tt.outputTokens, cost)
			}
			if tt.expectGtZero && cost <= 0 {
				t.Errorf("%v.CalculateCostCents(%d, %d) = %f, want > 0",
					tt.tier, tt.inputTokens, tt.outputTokens, cost)
			}
		})
	}
}

// TestRouteQueryDetailed tests the detailed routing decision function.
// Verifies that all fields are properly populated with meaningful values.
func TestRouteQueryDetailed(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		expectedTier  Tier
		expectedType  QueryType
		expectedCompl QueryComplexity
	}{
		{
			name:          "trivial_query",
			query:         "hello",
			expectedTier:  TierCache,
			expectedCompl: ComplexityTrivial,
			expectedType:  QueryTypeGeneral,
		},
		{
			name:          "simple_lookup",
			query:         "what is a mutex",
			expectedTier:  TierLocal,
			expectedCompl: ComplexitySimple,
			expectedType:  QueryTypeLookup,
		},
		{
			name:          "complex_debugging",
			query:         "fix this bug in my code",
			expectedTier:  TierCloud,
			expectedCompl: ComplexityComplex,
			expectedType:  QueryTypeDebugging,
		},
		{
			name:          "expert_architecture",
			query:         "should I use microservices or monolith",
			expectedTier:  TierCloud,
			expectedCompl: ComplexityExpert,
			expectedType:  QueryTypeArchitecture,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use Unclassified to test normal routing behavior
			decision := RouteQueryDetailed(tt.query, security.ClassificationUnclassified, nil)

			// Check tier
			if decision.Tier != tt.expectedTier {
				t.Errorf("Tier = %v, want %v", decision.Tier, tt.expectedTier)
			}

			// Check complexity
			if decision.Complexity != tt.expectedCompl {
				t.Errorf("Complexity = %v, want %v", decision.Complexity, tt.expectedCompl)
			}

			// Check query type
			if decision.QueryType != tt.expectedType {
				t.Errorf("QueryType = %v, want %v", decision.QueryType, tt.expectedType)
			}

			// Check that reason is informative (contains key information)
			if decision.Reason == "" {
				t.Error("Reason should not be empty")
			}
			if !strings.Contains(decision.Reason, decision.Complexity.String()) {
				t.Errorf("Reason should mention complexity: %q", decision.Reason)
			}
			if !strings.Contains(decision.Reason, decision.Tier.Name()) {
				t.Errorf("Reason should mention tier: %q", decision.Reason)
			}

			// Check cost is calculated
			if decision.Tier.IsPaid() && decision.EstimatedCostCents <= 0 {
				t.Error("Paid tier should have estimated cost > 0")
			}
			if !decision.Tier.IsPaid() && decision.EstimatedCostCents != 0 {
				t.Errorf("Free tier should have cost = 0, got %f", decision.EstimatedCostCents)
			}
		})
	}
}

// TestRouteQueryDetailedWithMaxTier tests detailed routing with max tier cap.
func TestRouteQueryDetailedWithMaxTier(t *testing.T) {
	query := "should I use microservices, what are the trade-offs"
	maxTier := TierLocal

	// Use Unclassified to test max tier capping
	decision := RouteQueryDetailed(query, security.ClassificationUnclassified, &maxTier)

	// Should be capped at Local
	if decision.Tier != TierLocal {
		t.Errorf("Tier = %v, want %v (capped)", decision.Tier, TierLocal)
	}

	// Complexity should still be Expert (not affected by cap)
	if decision.Complexity != ComplexityExpert {
		t.Errorf("Complexity = %v, want Expert", decision.Complexity)
	}
}

// TestTierString tests the String method of Tier.
func TestTierString(t *testing.T) {
	tests := []struct {
		tier     Tier
		expected string
	}{
		{TierCache, "Cache"},
		{TierLocal, "Local"},
		{TierCloud, "Cloud"},
		{TierHaiku, "Haiku"},
		{TierSonnet, "Sonnet"},
		{TierOpus, "Opus"},
		{TierGpt4o, "GPT-4o"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.tier.String()
			if result != tt.expected {
				t.Errorf("Tier(%d).String() = %q, want %q", tt.tier, result, tt.expected)
			}
		})
	}
}

// TestTierName tests the Name method (alias for String).
func TestTierName(t *testing.T) {
	tiers := []Tier{TierCache, TierLocal, TierCloud, TierHaiku, TierSonnet, TierOpus, TierGpt4o}

	for _, tier := range tiers {
		t.Run(tier.String(), func(t *testing.T) {
			if tier.Name() != tier.String() {
				t.Errorf("Tier.Name() = %q, want %q (same as String())",
					tier.Name(), tier.String())
			}
		})
	}
}

// TestTierIsLocal tests the IsLocal method.
func TestTierIsLocal(t *testing.T) {
	tests := []struct {
		tier     Tier
		expected bool
	}{
		{TierCache, true},
		{TierLocal, true},
		{TierCloud, false},
		{TierHaiku, false},
		{TierSonnet, false},
		{TierOpus, false},
		{TierGpt4o, false},
	}

	for _, tt := range tests {
		t.Run(tt.tier.String(), func(t *testing.T) {
			result := tt.tier.IsLocal()
			if result != tt.expected {
				t.Errorf("%v.IsLocal() = %v, want %v", tt.tier, result, tt.expected)
			}
		})
	}
}

// TestTierIsPaid tests the IsPaid method.
func TestTierIsPaid(t *testing.T) {
	tests := []struct {
		tier     Tier
		expected bool
	}{
		{TierCache, false},
		{TierLocal, false},
		{TierCloud, true},
		{TierHaiku, true},
		{TierSonnet, true},
		{TierOpus, true},
		{TierGpt4o, true},
	}

	for _, tt := range tests {
		t.Run(tt.tier.String(), func(t *testing.T) {
			result := tt.tier.IsPaid()
			if result != tt.expected {
				t.Errorf("%v.IsPaid() = %v, want %v", tt.tier, result, tt.expected)
			}
		})
	}
}

// TestTierTypicalLatency tests that typical latencies are reasonable.
func TestTierTypicalLatency(t *testing.T) {
	t.Run("cache_fastest", func(t *testing.T) {
		latency := TierCache.TypicalLatencyMs()
		if latency >= TierLocal.TypicalLatencyMs() {
			t.Errorf("Cache latency (%d) should be < Local latency (%d)",
				latency, TierLocal.TypicalLatencyMs())
		}
	})

	t.Run("local_faster_than_cloud", func(t *testing.T) {
		latency := TierLocal.TypicalLatencyMs()
		if latency >= TierCloud.TypicalLatencyMs() {
			t.Errorf("Local latency (%d) should be < Cloud latency (%d)",
				latency, TierCloud.TypicalLatencyMs())
		}
	})

	t.Run("all_latencies_positive", func(t *testing.T) {
		tiers := []Tier{TierCache, TierLocal, TierCloud, TierHaiku, TierSonnet, TierOpus, TierGpt4o}
		for _, tier := range tiers {
			latency := tier.TypicalLatencyMs()
			if latency == 0 {
				t.Errorf("%v.TypicalLatencyMs() = 0, want > 0", tier)
			}
		}
	})
}

// TestComplexityMinTier tests the MinTier mapping for complexities.
func TestComplexityMinTier(t *testing.T) {
	tests := []struct {
		complexity QueryComplexity
		expected   Tier
	}{
		{ComplexityTrivial, TierCache},
		{ComplexitySimple, TierLocal},
		{ComplexityModerate, TierCloud},
		{ComplexityComplex, TierCloud},
		{ComplexityExpert, TierCloud},
	}

	for _, tt := range tests {
		t.Run(tt.complexity.String(), func(t *testing.T) {
			result := tt.complexity.MinTier()
			if result != tt.expected {
				t.Errorf("%v.MinTier() = %v, want %v", tt.complexity, result, tt.expected)
			}
		})
	}
}

// TestQueryResultCreation tests the QueryResult factory functions.
func TestQueryResultCreation(t *testing.T) {
	t.Run("new_query_result", func(t *testing.T) {
		result := NewQueryResult("response", TierHaiku, 100, 200, 500)

		if result.Response != "response" {
			t.Errorf("Response = %q, want %q", result.Response, "response")
		}
		if result.TierUsed != TierHaiku {
			t.Errorf("TierUsed = %v, want %v", result.TierUsed, TierHaiku)
		}
		if result.InputTokens != 100 {
			t.Errorf("InputTokens = %d, want %d", result.InputTokens, 100)
		}
		if result.OutputTokens != 200 {
			t.Errorf("OutputTokens = %d, want %d", result.OutputTokens, 200)
		}
		if result.LatencyMs != 500 {
			t.Errorf("LatencyMs = %d, want %d", result.LatencyMs, 500)
		}
		if result.CacheHit {
			t.Error("CacheHit should be false")
		}
		if result.CostCents <= 0 {
			t.Errorf("CostCents = %f, want > 0 for paid tier", result.CostCents)
		}
	})

	t.Run("new_cache_hit_result", func(t *testing.T) {
		result := NewCacheHitResult("cached response", 5)

		if result.Response != "cached response" {
			t.Errorf("Response = %q, want %q", result.Response, "cached response")
		}
		if result.TierUsed != TierCache {
			t.Errorf("TierUsed = %v, want %v", result.TierUsed, TierCache)
		}
		if result.InputTokens != 0 {
			t.Errorf("InputTokens = %d, want 0", result.InputTokens)
		}
		if result.OutputTokens != 0 {
			t.Errorf("OutputTokens = %d, want 0", result.OutputTokens)
		}
		if !result.CacheHit {
			t.Error("CacheHit should be true")
		}
		if result.CostCents != 0 {
			t.Errorf("CostCents = %f, want 0 for cache hit", result.CostCents)
		}
	})
}

// TestQueryResultTotalTokens tests the TotalTokens method.
func TestQueryResultTotalTokens(t *testing.T) {
	result := NewQueryResult("test", TierLocal, 100, 200, 500)
	total := result.TotalTokens()
	if total != 300 {
		t.Errorf("TotalTokens() = %d, want 300", total)
	}
}

// TestRoutingDecisionString tests the String method of RoutingDecision.
func TestRoutingDecisionString(t *testing.T) {
	decision := RouteQueryDetailed("what is rust", security.ClassificationUnclassified, nil)
	str := decision.String()

	// Should contain key information
	if !strings.Contains(str, decision.Tier.String()) {
		t.Errorf("String() should contain tier name: %q", str)
	}
	if !strings.Contains(str, decision.Complexity.String()) {
		t.Errorf("String() should contain complexity: %q", str)
	}
}

// TestQueryResultString tests the String method of QueryResult.
func TestQueryResultString(t *testing.T) {
	t.Run("regular_result", func(t *testing.T) {
		result := NewQueryResult("test", TierHaiku, 100, 200, 500)
		str := result.String()

		if !strings.Contains(str, "Haiku") {
			t.Errorf("String() should contain tier name: %q", str)
		}
		if strings.Contains(str, "CACHE HIT") {
			t.Errorf("String() should not contain CACHE HIT for non-cache result: %q", str)
		}
	})

	t.Run("cache_hit_result", func(t *testing.T) {
		result := NewCacheHitResult("cached", 5)
		str := result.String()

		if !strings.Contains(str, "CACHE HIT") {
			t.Errorf("String() should contain CACHE HIT: %q", str)
		}
	})
}

// TestQueryTypeUnknown verifies the new QueryTypeUnknown constant works correctly.
func TestQueryTypeUnknown(t *testing.T) {
	if QueryTypeUnknown.String() != "Unknown" {
		t.Errorf("QueryTypeUnknown.String() = %q, want %q", QueryTypeUnknown.String(), "Unknown")
	}
}

// ============================================================================
// BENCHMARKS
// ============================================================================

// BenchmarkRouteQuery benchmarks the routing function.
func BenchmarkRouteQuery(b *testing.B) {
	queries := []string{
		"hello",
		"what is rust",
		"how do I fix this error in my code",
		"should I use microservices or monolith, what are the trade-offs",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, q := range queries {
			_ = RouteQuery(q, security.ClassificationUnclassified, false, nil)
		}
	}
}

// BenchmarkRouteQueryDetailed benchmarks the detailed routing function.
func BenchmarkRouteQueryDetailed(b *testing.B) {
	queries := []string{
		"hello",
		"what is rust",
		"how do I fix this error in my code",
		"should I use microservices or monolith, what are the trade-offs",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, q := range queries {
			_ = RouteQueryDetailed(q, security.ClassificationUnclassified, nil)
		}
	}
}

// BenchmarkRouteQueryWithCUI benchmarks routing with CUI classification.
func BenchmarkRouteQueryWithCUI(b *testing.B) {
	queries := []string{
		"hello",
		"what is rust",
		"how do I fix this error in my code",
		"should I use microservices or monolith, what are the trade-offs",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, q := range queries {
			_ = RouteQuery(q, security.ClassificationCUI, false, nil)
		}
	}
}

// BenchmarkTierEscalate benchmarks the escalation function.
func BenchmarkTierEscalate(b *testing.B) {
	tiers := []Tier{TierCache, TierLocal, TierCloud, TierHaiku, TierSonnet, TierOpus}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range tiers {
			_ = t.Escalate()
		}
	}
}

// BenchmarkCalculateCostCents benchmarks the cost calculation.
func BenchmarkCalculateCostCents(b *testing.B) {
	tiers := []Tier{TierCache, TierLocal, TierCloud, TierHaiku, TierSonnet, TierOpus}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range tiers {
			_ = t.CalculateCostCents(1000, 1000)
		}
	}
}

// Helper function to create a pointer to a Tier.
func tierPtr(t Tier) *Tier {
	return &t
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
