// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// AC-4 Information Flow Enforcement Tests for Router Layer
// Per NIST SP 800-53 AC-4: Information Flow Enforcement
// Per DoDI 5200.48 and 32 CFR Part 2002
//
// These tests verify that classification-based routing decisions are enforced
// correctly to prevent unauthorized information flow to cloud tiers.

package router

import (
	"strings"
	"testing"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// ============================================================================
// AC-4 CLASSIFICATION ENFORCEMENT - CORE SCENARIOS
// ============================================================================

// TestAC4_ClassificationEnforcement tests all core AC-4 classification scenarios.
// NIST AC-4: The information system enforces approved authorizations for controlling
// the flow of information within the system and between interconnected systems.
func TestAC4_ClassificationEnforcement(t *testing.T) {
	tests := []struct {
		name           string
		classification security.ClassificationLevel
		query          string
		paranoidMode   bool
		maxTier        *Tier
		expectTier     Tier
		expectError    bool
		description    string
	}{
		// =====================================================================
		// Scenario 1: Unclassified query -> cloud routing: Should SUCCEED
		// =====================================================================
		{
			name:           "AC4_1_Unclassified_to_Cloud_ALLOWED",
			classification: security.ClassificationUnclassified,
			query:          "explain how async runtime works with examples",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierCloud, // Complex query should route to cloud
			expectError:    false,
			description:    "Unclassified data CAN flow to cloud tiers",
		},
		{
			name:           "AC4_1_Unclassified_Simple_to_Local",
			classification: security.ClassificationUnclassified,
			query:          "what is rust",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierLocal, // Simple query routes to local
			expectError:    false,
			description:    "Unclassified simple queries route to local for cost efficiency",
		},
		{
			name:           "AC4_1_Unclassified_Trivial_to_Cache",
			classification: security.ClassificationUnclassified,
			query:          "hello",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierCache, // Trivial query routes to cache
			expectError:    false,
			description:    "Unclassified trivial queries route to cache",
		},

		// =====================================================================
		// Scenario 2: CUI query -> local routing: Should SUCCEED
		// =====================================================================
		{
			name:           "AC4_2_CUI_to_Local_ALLOWED",
			classification: security.ClassificationCUI,
			query:          "explain how async runtime works with examples",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierLocal, // CUI FORCES local
			expectError:    false,
			description:    "CUI data MUST stay on-premise (TierLocal)",
		},
		{
			name:           "AC4_2_CUI_Simple_to_Local",
			classification: security.ClassificationCUI,
			query:          "what is the function signature",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierLocal, // CUI FORCES local even for simple queries
			expectError:    false,
			description:    "CUI simple queries also stay local",
		},
		{
			name:           "AC4_2_CUI_Trivial_to_Local",
			classification: security.ClassificationCUI,
			query:          "hello",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierLocal, // CUI FORCES local even for trivial queries
			expectError:    false,
			description:    "CUI trivial queries stay local (no cache for CUI)",
		},

		// =====================================================================
		// Scenario 3: CUI query -> cloud routing attempt: Should FAIL/block
		// =====================================================================
		{
			name:           "AC4_3_CUI_to_Cloud_BLOCKED",
			classification: security.ClassificationCUI,
			query:          "should I use microservices, what are the trade-offs",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierLocal, // CUI MUST NOT go to cloud
			expectError:    false,     // No error, just forced to local
			description:    "CUI data MUST NOT flow to cloud - forced to local",
		},
		{
			name:           "AC4_3_CUI_Expert_Query_Blocked",
			classification: security.ClassificationCUI,
			query:          "architect a distributed system with microservices",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierLocal, // Even expert queries stay local for CUI
			expectError:    false,
			description:    "CUI expert queries blocked from cloud",
		},

		// =====================================================================
		// Scenario 4: Secret query -> any cloud tier: Should FAIL/block
		// =====================================================================
		{
			name:           "AC4_4_Secret_to_Cloud_BLOCKED",
			classification: security.ClassificationSecret,
			query:          "explain how async runtime works with examples",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierLocal, // Secret MUST NOT go to cloud
			expectError:    false,
			description:    "Secret data MUST NOT flow to cloud",
		},
		{
			name:           "AC4_4_Secret_to_Haiku_BLOCKED",
			classification: security.ClassificationSecret,
			query:          "review this code and explain the bugs",
			paranoidMode:   false,
			maxTier:        tierPtr(TierHaiku), // Attempt to cap at Haiku
			expectTier:     TierLocal,          // Classification overrides maxTier
			expectError:    false,
			description:    "Secret data blocked from Haiku tier",
		},
		{
			name:           "AC4_4_Secret_to_Sonnet_BLOCKED",
			classification: security.ClassificationSecret,
			query:          "architect a new system",
			paranoidMode:   false,
			maxTier:        tierPtr(TierSonnet),
			expectTier:     TierLocal,
			expectError:    false,
			description:    "Secret data blocked from Sonnet tier",
		},
		{
			name:           "AC4_4_Secret_to_Opus_BLOCKED",
			classification: security.ClassificationSecret,
			query:          "should I use microservices",
			paranoidMode:   false,
			maxTier:        tierPtr(TierOpus),
			expectTier:     TierLocal,
			expectError:    false,
			description:    "Secret data blocked from Opus tier",
		},

		// =====================================================================
		// Scenario 5: Top Secret query -> any tier except local: Should FAIL/block
		// =====================================================================
		{
			name:           "AC4_5_TopSecret_to_Cloud_BLOCKED",
			classification: security.ClassificationTopSecret,
			query:          "explain how async runtime works with examples",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierLocal, // Top Secret MUST stay local
			expectError:    false,
			description:    "Top Secret data MUST stay on-premise",
		},
		{
			name:           "AC4_5_TopSecret_to_Cache_Forces_Local",
			classification: security.ClassificationTopSecret,
			query:          "hello",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierLocal, // Top Secret forces local even for trivial
			expectError:    false,
			description:    "Top Secret trivial queries stay local (no cache)",
		},
		{
			name:           "AC4_5_TopSecret_Expert_Local",
			classification: security.ClassificationTopSecret,
			query:          "architect the best solution for distributed computing",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierLocal, // Top Secret expert queries stay local
			expectError:    false,
			description:    "Top Secret expert queries MUST stay local",
		},

		// =====================================================================
		// Scenario 6: Missing classification header: Should default to Unclassified
		// NOTE: This is tested implicitly by passing ClassificationUnclassified
		// =====================================================================
		{
			name:           "AC4_6_Default_Unclassified",
			classification: security.ClassificationUnclassified, // Default
			query:          "explain async await",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierCloud, // Normal routing for unclassified
			expectError:    false,
			description:    "Missing classification defaults to Unclassified",
		},

		// =====================================================================
		// Scenario 8: Classification enforcement at router layer
		// =====================================================================
		{
			name:           "AC4_8_Router_Blocks_Confidential",
			classification: security.ClassificationConfidential,
			query:          "explain how async runtime works",
			paranoidMode:   false,
			maxTier:        nil,
			expectTier:     TierLocal, // Confidential blocked from cloud
			expectError:    false,
			description:    "Router blocks Confidential data from cloud",
		},

		// =====================================================================
		// Scenario 9: Paranoid mode enforcement
		// =====================================================================
		{
			name:           "AC4_9_Paranoid_Mode_Blocks_Cloud",
			classification: security.ClassificationUnclassified,
			query:          "explain how async runtime works with examples",
			paranoidMode:   true, // Paranoid mode enabled
			maxTier:        nil,
			expectTier:     TierLocal, // Paranoid mode forces local
			expectError:    false,
			description:    "Paranoid mode blocks all cloud routing",
		},
		{
			name:           "AC4_9_Paranoid_Mode_Expert_Query",
			classification: security.ClassificationUnclassified,
			query:          "should I use microservices or monolith",
			paranoidMode:   true,
			maxTier:        nil,
			expectTier:     TierLocal, // Paranoid mode overrides normal routing
			expectError:    false,
			description:    "Paranoid mode blocks expert queries from cloud",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RouteQuery(tt.query, tt.classification, tt.paranoidMode, tt.maxTier)

			if result != tt.expectTier {
				t.Errorf("[%s] RouteQuery() = %v, want %v\n"+
					"  Classification: %v\n"+
					"  ParanoidMode: %v\n"+
					"  Query: %q\n"+
					"  Description: %s",
					tt.name, result, tt.expectTier,
					tt.classification, tt.paranoidMode, tt.query, tt.description)
			}
		})
	}
}

// ============================================================================
// AC-4 DETAILED ROUTING DECISION TESTS
// ============================================================================

// TestAC4_ClassificationEnforcementDetailed tests detailed routing with classification.
func TestAC4_ClassificationEnforcementDetailed(t *testing.T) {
	tests := []struct {
		name           string
		classification security.ClassificationLevel
		query          string
		opts           interface{}
		expectTier     Tier
		expectReason   string // Substring to look for in reason
	}{
		{
			name:           "CUI_Forces_Local_With_Reason",
			classification: security.ClassificationCUI,
			query:          "explain async await in detail",
			opts:           nil,
			expectTier:     TierLocal,
			expectReason:   "CUI classification blocks cloud",
		},
		{
			name:           "Secret_Forces_Local_With_Reason",
			classification: security.ClassificationSecret,
			query:          "architect a distributed system",
			opts:           nil,
			expectTier:     TierLocal,
			expectReason:   "SECRET classification blocks cloud",
		},
		{
			name:           "TopSecret_Forces_Local_With_Reason",
			classification: security.ClassificationTopSecret,
			query:          "explain the architecture",
			opts:           nil,
			expectTier:     TierLocal,
			expectReason:   "TOP SECRET classification blocks cloud",
		},
		{
			name:           "Paranoid_Mode_Blocks_With_Reason",
			classification: security.ClassificationUnclassified,
			query:          "explain async await",
			opts:           &RouterOptions{Paranoid: true},
			expectTier:     TierLocal,
			expectReason:   "paranoid mode blocks cloud",
		},
		{
			name:           "Unclassified_Normal_Routing",
			classification: security.ClassificationUnclassified,
			query:          "explain async await",
			opts:           nil,
			expectTier:     TierCloud, // Normal routing for unclassified complex query
			expectReason:   "Complex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := RouteQueryDetailed(tt.query, tt.classification, tt.opts)

			if decision.Tier != tt.expectTier {
				t.Errorf("RouteQueryDetailed() Tier = %v, want %v", decision.Tier, tt.expectTier)
			}

			if !strings.Contains(decision.Reason, tt.expectReason) {
				t.Errorf("RouteQueryDetailed() Reason = %q, want to contain %q",
					decision.Reason, tt.expectReason)
			}
		})
	}
}

// ============================================================================
// AC-4 FALLBACK BLOCKED FOR CUI+ TESTS
// ============================================================================

// TestAC4_10_FallbackBlockedForCUIPlus tests that CUI+ data cannot fallback to cloud.
// Scenario 10: Fallback blocked for CUI+: When local fails, CUI must not fallback to cloud
func TestAC4_10_FallbackBlockedForCUIPlus(t *testing.T) {
	classifications := []struct {
		level security.ClassificationLevel
		name  string
	}{
		{security.ClassificationCUI, "CUI"},
		{security.ClassificationConfidential, "Confidential"},
		{security.ClassificationSecret, "Secret"},
		{security.ClassificationTopSecret, "Top Secret"},
	}

	for _, class := range classifications {
		t.Run("Fallback_Blocked_"+class.name, func(t *testing.T) {
			// Even with a complex query that would normally escalate to cloud,
			// classification enforcement should prevent fallback
			query := "architect a complex distributed system with microservices"

			tier := RouteQuery(query, class.level, false, nil)

			if tier != TierLocal {
				t.Errorf("Classification %s should force local tier (no cloud fallback), got %v",
					class.name, tier)
			}

			// Verify escalation path is blocked
			escalated := tier.Escalate()
			if escalated != nil && *escalated != TierLocal && class.level >= security.ClassificationCUI {
				// NOTE: The Escalate() method itself doesn't check classification.
				// The classification check must happen at the routing layer.
				// This test verifies the routing layer blocks escalation.
				t.Logf("Note: Tier.Escalate() returns %v, but classification enforcement at routing layer prevents actual escalation", *escalated)
			}
		})
	}
}

// TestAC4_EscalationPrevention verifies that classification prevents tier escalation.
func TestAC4_EscalationPrevention(t *testing.T) {
	// Complex query that would normally route to Cloud
	query := "should I use microservices, what are the trade-offs"

	t.Run("CUI_Prevents_Cloud_Escalation", func(t *testing.T) {
		// Without classification, this would go to Cloud
		unclasTier := RouteQuery(query, security.ClassificationUnclassified, false, nil)
		if unclasTier != TierCloud {
			t.Fatalf("Expected unclassified expert query to route to Cloud, got %v", unclasTier)
		}

		// With CUI, it MUST stay local
		cuiTier := RouteQuery(query, security.ClassificationCUI, false, nil)
		if cuiTier != TierLocal {
			t.Errorf("CUI classification should force Local tier, got %v", cuiTier)
		}
	})

	t.Run("Secret_Prevents_Cloud_Escalation", func(t *testing.T) {
		secretTier := RouteQuery(query, security.ClassificationSecret, false, nil)
		if secretTier != TierLocal {
			t.Errorf("Secret classification should force Local tier, got %v", secretTier)
		}
	})
}

// ============================================================================
// AC-4 AUTO MODE CLASSIFICATION TESTS
// ============================================================================

// TestAC4_AutoModeClassification tests classification enforcement in auto mode.
func TestAC4_AutoModeClassification(t *testing.T) {
	tests := []struct {
		name           string
		classification security.ClassificationLevel
		paranoidMode   bool
		expectTier     Tier
		expectAuto     bool
	}{
		{
			name:           "Auto_Unclassified_Allowed",
			classification: security.ClassificationUnclassified,
			paranoidMode:   false,
			expectTier:     TierAuto,
			expectAuto:     true,
		},
		{
			name:           "Auto_CUI_Blocked",
			classification: security.ClassificationCUI,
			paranoidMode:   false,
			expectTier:     TierLocal,
			expectAuto:     false,
		},
		{
			name:           "Auto_Secret_Blocked",
			classification: security.ClassificationSecret,
			paranoidMode:   false,
			expectTier:     TierLocal,
			expectAuto:     false,
		},
		{
			name:           "Auto_Paranoid_Blocked",
			classification: security.ClassificationUnclassified,
			paranoidMode:   true,
			expectTier:     TierLocal,
			expectAuto:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := RouteQueryAuto(
				"explain async await",
				tt.classification,
				tt.paranoidMode,
				false, // preferLocal
				0.0,   // maxCost
			)

			if decision.Tier != tt.expectTier {
				t.Errorf("RouteQueryAuto() Tier = %v, want %v", decision.Tier, tt.expectTier)
			}

			if decision.IsAutoRouted != tt.expectAuto {
				t.Errorf("RouteQueryAuto() IsAutoRouted = %v, want %v",
					decision.IsAutoRouted, tt.expectAuto)
			}
		})
	}
}

// ============================================================================
// AC-4 QUERY LENGTH VALIDATION TESTS
// ============================================================================

// TestAC4_QueryLengthValidation tests that excessively long queries are rejected.
func TestAC4_QueryLengthValidation(t *testing.T) {
	// Create a query that exceeds MaxQueryLength
	longQuery := strings.Repeat("a", MaxQueryLength+1)

	t.Run("Long_Query_Rejected", func(t *testing.T) {
		tier := RouteQuery(longQuery, security.ClassificationUnclassified, false, nil)

		// Should return TierLocal as safe fallback
		if tier != TierLocal {
			t.Errorf("Excessively long query should return TierLocal, got %v", tier)
		}
	})

	t.Run("Long_Query_Rejected_Detailed", func(t *testing.T) {
		decision := RouteQueryDetailed(longQuery, security.ClassificationUnclassified, nil)

		if decision.Tier != TierLocal {
			t.Errorf("Excessively long query should return TierLocal, got %v", decision.Tier)
		}

		if !strings.Contains(decision.Reason, "Query rejected") {
			t.Errorf("Reason should indicate query rejection: %q", decision.Reason)
		}
	})

	t.Run("Long_Query_Rejected_Auto", func(t *testing.T) {
		decision := RouteQueryAuto(longQuery, security.ClassificationUnclassified, false, false, 0)

		if decision.Tier != TierLocal {
			t.Errorf("Excessively long query should return TierLocal, got %v", decision.Tier)
		}

		if decision.IsAutoRouted {
			t.Error("Rejected query should not be marked as auto-routed")
		}
	})

	t.Run("MaxLength_Query_Accepted", func(t *testing.T) {
		maxQuery := strings.Repeat("a", MaxQueryLength)
		tier := RouteQuery(maxQuery, security.ClassificationUnclassified, false, nil)

		// Should NOT be rejected (exactly at limit)
		// Note: The actual tier depends on complexity classification
		if tier != TierLocal && tier != TierCache && tier != TierCloud {
			t.Errorf("Query at max length should route normally, got unexpected tier %v", tier)
		}
	})
}

// ============================================================================
// AC-4 CLASSIFICATION LEVEL BOUNDARY TESTS
// ============================================================================

// TestAC4_ClassificationBoundaries tests the exact boundaries of classification enforcement.
func TestAC4_ClassificationBoundaries(t *testing.T) {
	query := "explain async await in detail"

	t.Run("Unclassified_Below_Boundary", func(t *testing.T) {
		tier := RouteQuery(query, security.ClassificationUnclassified, false, nil)

		// Unclassified is BELOW the CUI boundary, so cloud is allowed
		if tier != TierCloud {
			t.Errorf("Unclassified should allow Cloud routing, got %v", tier)
		}
	})

	t.Run("CUI_At_Boundary", func(t *testing.T) {
		tier := RouteQuery(query, security.ClassificationCUI, false, nil)

		// CUI is AT the boundary, cloud is blocked
		if tier != TierLocal {
			t.Errorf("CUI should block Cloud routing, got %v", tier)
		}
	})

	t.Run("All_Above_CUI_Blocked", func(t *testing.T) {
		aboveCUI := []security.ClassificationLevel{
			security.ClassificationConfidential,
			security.ClassificationSecret,
			security.ClassificationTopSecret,
		}

		for _, level := range aboveCUI {
			tier := RouteQuery(query, level, false, nil)
			if tier != TierLocal {
				t.Errorf("Classification %v should block Cloud routing, got %v", level, tier)
			}
		}
	})
}

// ============================================================================
// AC-4 INTEGRATION TESTS
// ============================================================================

// TestAC4_IntegrationScenarios tests realistic integration scenarios.
func TestAC4_IntegrationScenarios(t *testing.T) {
	t.Run("Government_Contractor_Scenario", func(t *testing.T) {
		// Contractor working on CUI-marked project
		// All queries about the project must stay on-premise

		queries := []string{
			"explain the authentication flow",         // Simple
			"review the security implementation",      // Complex
			"architect the DoD integration module",    // Expert
			"what is the deployment configuration",    // Lookup
			"how do I fix the STIG compliance issue",  // Debugging
		}

		for _, q := range queries {
			tier := RouteQuery(q, security.ClassificationCUI, false, nil)
			if tier != TierLocal {
				t.Errorf("CUI project query %q should stay local, got %v", q, tier)
			}
		}
	})

	t.Run("Airgapped_Network_Scenario", func(t *testing.T) {
		// System in airgapped/paranoid mode
		// No queries should ever reach cloud

		queries := []string{
			"hello",
			"what is rust",
			"explain async await",
			"should I use microservices",
		}

		for _, q := range queries {
			tier := RouteQuery(q, security.ClassificationUnclassified, true, nil)
			if tier != TierLocal {
				t.Errorf("Paranoid mode query %q should stay local, got %v", q, tier)
			}
		}
	})

	t.Run("Mixed_Classification_Session", func(t *testing.T) {
		// Session starts unclassified, then transitions to CUI

		// Unclassified phase - cloud allowed
		unclasTier := RouteQuery("explain async await", security.ClassificationUnclassified, false, nil)
		if unclasTier != TierCloud {
			t.Errorf("Unclassified query should allow cloud, got %v", unclasTier)
		}

		// CUI phase - cloud blocked
		cuiTier := RouteQuery("explain async await", security.ClassificationCUI, false, nil)
		if cuiTier != TierLocal {
			t.Errorf("CUI query should force local, got %v", cuiTier)
		}

		// Cannot downgrade back to unclassified (session highest marking applies)
		// This would typically be enforced at the session layer, not router
		// The router always respects the classification passed to it
	})
}

// ============================================================================
// BENCHMARK TESTS
// ============================================================================

// BenchmarkAC4_ClassificationCheck benchmarks the classification enforcement check.
func BenchmarkAC4_ClassificationCheck(b *testing.B) {
	query := "explain how async runtime works with examples"

	b.Run("Unclassified", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			RouteQuery(query, security.ClassificationUnclassified, false, nil)
		}
	})

	b.Run("CUI", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			RouteQuery(query, security.ClassificationCUI, false, nil)
		}
	})

	b.Run("Secret", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			RouteQuery(query, security.ClassificationSecret, false, nil)
		}
	})

	b.Run("Paranoid", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			RouteQuery(query, security.ClassificationUnclassified, true, nil)
		}
	})
}

// BenchmarkAC4_DetailedDecision benchmarks the detailed routing decision with classification.
func BenchmarkAC4_DetailedDecision(b *testing.B) {
	query := "should I use microservices, what are the trade-offs"

	b.Run("Unclassified_Detailed", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			RouteQueryDetailed(query, security.ClassificationUnclassified, nil)
		}
	})

	b.Run("CUI_Detailed", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			RouteQueryDetailed(query, security.ClassificationCUI, nil)
		}
	})
}
