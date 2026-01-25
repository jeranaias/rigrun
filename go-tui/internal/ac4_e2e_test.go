// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// AC-4 End-to-End Classification Enforcement Tests
// Per NIST SP 800-53 AC-4: Information Flow Enforcement
// Per DoDI 5200.48 and 32 CFR Part 2002
//
// This file contains comprehensive end-to-end tests that verify
// AC-4 information flow enforcement across all layers:
// - Security layer (ClassificationEnforcer)
// - Router layer (RouteQuery with classification)
// - Integration scenarios

package internal

import (
	"errors"
	"strings"
	"testing"

	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/security"
)

// ============================================================================
// AC-4 END-TO-END TEST SUITE
// ============================================================================

// TestAC4_E2E_AllScenarios is the master test function that runs all 10 AC-4 scenarios.
func TestAC4_E2E_AllScenarios(t *testing.T) {
	// Create a fresh enforcer for each test
	enforcer := security.NewClassificationEnforcer(nil, "e2e-test-session")

	// Sample queries of varying complexity
	trivialQuery := "hello"
	simpleQuery := "what is rust"
	complexQuery := "explain how async runtime works with examples"
	expertQuery := "should I use microservices, what are the trade-offs"

	// =========================================================================
	// SCENARIO 1: Unclassified query -> cloud routing: Should SUCCEED
	// =========================================================================
	t.Run("Scenario_1_Unclassified_to_Cloud_SUCCESS", func(t *testing.T) {
		classification := security.ClassificationUnclassified

		// Router layer should allow cloud for complex query
		tier := router.RouteQuery(complexQuery, classification, false, nil)
		if tier != router.TierCloud {
			t.Errorf("Scenario 1 FAILED: Unclassified complex query should route to Cloud, got %v", tier)
		}

		// Security layer should also allow
		if !enforcer.CanRouteToCloud(classification) {
			t.Error("Scenario 1 FAILED: Enforcer should allow cloud for Unclassified")
		}

		t.Log("Scenario 1 PASSED: Unclassified data can flow to cloud")
	})

	// =========================================================================
	// SCENARIO 2: CUI query -> local routing: Should SUCCEED
	// =========================================================================
	t.Run("Scenario_2_CUI_to_Local_SUCCESS", func(t *testing.T) {
		classification := security.ClassificationCUI

		// Router layer should force local for CUI
		tier := router.RouteQuery(complexQuery, classification, false, nil)
		if tier != router.TierLocal {
			t.Errorf("Scenario 2 FAILED: CUI query should route to Local, got %v", tier)
		}

		// Security layer should require local
		if !enforcer.RequiresLocalOnly(classification) {
			t.Error("Scenario 2 FAILED: Enforcer should require local for CUI")
		}

		t.Log("Scenario 2 PASSED: CUI data stays local")
	})

	// =========================================================================
	// SCENARIO 3: CUI query -> cloud routing attempt: Should FAIL/block
	// =========================================================================
	t.Run("Scenario_3_CUI_to_Cloud_BLOCKED", func(t *testing.T) {
		classification := security.ClassificationCUI

		// Router layer should block cloud
		tier := router.RouteQuery(expertQuery, classification, false, nil)
		if tier == router.TierCloud || tier == router.TierHaiku ||
			tier == router.TierSonnet || tier == router.TierOpus {
			t.Errorf("Scenario 3 FAILED: CUI should NOT route to cloud tier, got %v", tier)
		}

		// Security layer should block
		enforcedTier, err := enforcer.EnforceRouting(classification, security.RoutingTierCloud)
		if err == nil {
			t.Error("Scenario 3 FAILED: Enforcer should return error for CUI -> Cloud")
		}
		if enforcedTier != security.RoutingTierLocal {
			t.Errorf("Scenario 3 FAILED: Enforcer should return Local, got %v", enforcedTier)
		}

		t.Log("Scenario 3 PASSED: CUI data blocked from cloud")
	})

	// =========================================================================
	// SCENARIO 4: Secret query -> any cloud tier: Should FAIL/block
	// =========================================================================
	t.Run("Scenario_4_Secret_to_Cloud_BLOCKED", func(t *testing.T) {
		classification := security.ClassificationSecret

		// Router layer should block all cloud tiers
		tier := router.RouteQuery(complexQuery, classification, false, nil)
		if !tier.IsLocal() {
			t.Errorf("Scenario 4 FAILED: Secret should NOT route to cloud, got %v", tier)
		}

		// Security layer should block
		enforcedTier, err := enforcer.EnforceRouting(classification, security.RoutingTierCloud)
		if err == nil {
			t.Error("Scenario 4 FAILED: Enforcer should return error for Secret -> Cloud")
		}
		if enforcedTier != security.RoutingTierLocal {
			t.Errorf("Scenario 4 FAILED: Enforcer should return Local, got %v", enforcedTier)
		}

		// Test specific cloud tiers
		cloudTiers := []router.Tier{router.TierCloud, router.TierHaiku, router.TierSonnet, router.TierOpus}
		for _, cloudTier := range cloudTiers {
			result := router.RouteQuery(expertQuery, classification, false, &cloudTier)
			if !result.IsLocal() {
				t.Errorf("Scenario 4 FAILED: Secret blocked from %v, but got %v", cloudTier, result)
			}
		}

		t.Log("Scenario 4 PASSED: Secret data blocked from all cloud tiers")
	})

	// =========================================================================
	// SCENARIO 5: Top Secret query -> any tier except local: Should FAIL/block
	// =========================================================================
	t.Run("Scenario_5_TopSecret_to_Cloud_BLOCKED", func(t *testing.T) {
		classification := security.ClassificationTopSecret

		// Test all query types
		queries := []string{trivialQuery, simpleQuery, complexQuery, expertQuery}
		for _, query := range queries {
			tier := router.RouteQuery(query, classification, false, nil)
			if tier != router.TierLocal {
				t.Errorf("Scenario 5 FAILED: Top Secret query %q should route to Local, got %v",
					query, tier)
			}
		}

		// Security layer should block
		enforcedTier, err := enforcer.EnforceRouting(classification, security.RoutingTierCloud)
		if err == nil {
			t.Error("Scenario 5 FAILED: Enforcer should return error for TopSecret -> Cloud")
		}
		if enforcedTier != security.RoutingTierLocal {
			t.Error("Scenario 5 FAILED: Top Secret must stay local")
		}

		t.Log("Scenario 5 PASSED: Top Secret data stays local only")
	})

	// =========================================================================
	// SCENARIO 6: Missing classification header: Should default to Unclassified
	// =========================================================================
	t.Run("Scenario_6_Missing_Classification_Defaults_Unclassified", func(t *testing.T) {
		// Empty string should default to Unclassified
		class, err := security.ParseClassification("")
		if err != nil {
			t.Errorf("Scenario 6 FAILED: Empty string should parse without error: %v", err)
		}
		if class.Level != security.ClassificationUnclassified {
			t.Errorf("Scenario 6 FAILED: Empty string should default to Unclassified, got %v",
				class.Level)
		}

		// Default classification should be Unclassified
		def := security.DefaultClassification()
		if def.Level != security.ClassificationUnclassified {
			t.Errorf("Scenario 6 FAILED: DefaultClassification should be Unclassified, got %v",
				def.Level)
		}

		// ClassificationFromEnv with empty should default
		fromEnv := security.ClassificationFromEnv("")
		if fromEnv.Level != security.ClassificationUnclassified {
			t.Errorf("Scenario 6 FAILED: ClassificationFromEnv(\"\") should default to Unclassified")
		}

		t.Log("Scenario 6 PASSED: Missing classification defaults to Unclassified")
	})

	// =========================================================================
	// SCENARIO 7: Invalid classification header: Should return error
	// =========================================================================
	t.Run("Scenario_7_Invalid_Classification_Returns_Error", func(t *testing.T) {
		invalidInputs := []string{
			"INVALID",
			"FOOBAR",
			"SECERT", // Typo
			"TOP",    // Incomplete
			"12345",
			"'; DROP TABLE users;--",
		}

		for _, input := range invalidInputs {
			_, err := security.ParseClassification(input)
			if err == nil {
				t.Errorf("Scenario 7 FAILED: ParseClassification(%q) should return error", input)
			}
		}

		t.Log("Scenario 7 PASSED: Invalid classification headers return errors")
	})

	// =========================================================================
	// SCENARIO 8: Classification enforcement at TUI layer: Verify blocks
	// =========================================================================
	t.Run("Scenario_8_TUI_Layer_Enforcement", func(t *testing.T) {
		// TUI uses RequiresLocalOnly() to check before routing
		if !enforcer.RequiresLocalOnly(security.ClassificationCUI) {
			t.Error("Scenario 8 FAILED: TUI should detect CUI requires local")
		}
		if !enforcer.RequiresLocalOnly(security.ClassificationSecret) {
			t.Error("Scenario 8 FAILED: TUI should detect Secret requires local")
		}
		if !enforcer.RequiresLocalOnly(security.ClassificationTopSecret) {
			t.Error("Scenario 8 FAILED: TUI should detect TopSecret requires local")
		}
		if enforcer.RequiresLocalOnly(security.ClassificationUnclassified) {
			t.Error("Scenario 8 FAILED: TUI should allow Unclassified to route normally")
		}

		t.Log("Scenario 8 PASSED: TUI layer correctly detects classification restrictions")
	})

	// =========================================================================
	// SCENARIO 9: Classification enforcement at router layer: Verify blocks
	// =========================================================================
	t.Run("Scenario_9_Router_Layer_Enforcement", func(t *testing.T) {
		// Router uses classification parameter in RouteQuery
		// Verify all CUI+ classifications force local

		classificationsThatBlockCloud := []security.ClassificationLevel{
			security.ClassificationCUI,
			security.ClassificationConfidential,
			security.ClassificationSecret,
			security.ClassificationTopSecret,
		}

		for _, class := range classificationsThatBlockCloud {
			// Even expert queries should stay local
			tier := router.RouteQuery(expertQuery, class, false, nil)
			if tier != router.TierLocal {
				t.Errorf("Scenario 9 FAILED: Router should force local for %v, got %v",
					class, tier)
			}
		}

		// Detailed routing should also enforce
		for _, class := range classificationsThatBlockCloud {
			decision := router.RouteQueryDetailed(expertQuery, class, nil)
			if decision.Tier != router.TierLocal {
				t.Errorf("Scenario 9 FAILED: Detailed router should force local for %v, got %v",
					class, decision.Tier)
			}
			if !strings.Contains(decision.Reason, "classification blocks cloud") {
				t.Errorf("Scenario 9 FAILED: Reason should explain classification block: %q",
					decision.Reason)
			}
		}

		t.Log("Scenario 9 PASSED: Router layer correctly enforces classification")
	})

	// =========================================================================
	// SCENARIO 10: Fallback blocked for CUI+: When local fails, CUI must not fallback to cloud
	// =========================================================================
	t.Run("Scenario_10_Fallback_Blocked_For_CUI_Plus", func(t *testing.T) {
		classificationsThatBlockFallback := []security.ClassificationLevel{
			security.ClassificationCUI,
			security.ClassificationConfidential,
			security.ClassificationSecret,
			security.ClassificationTopSecret,
		}

		for _, class := range classificationsThatBlockFallback {
			// Simulate local failure and attempt cloud fallback
			// The enforcer should block this
			enforcedTier, err := enforcer.EnforceRouting(class, security.RoutingTierCloud)

			if enforcedTier == security.RoutingTierCloud {
				t.Errorf("Scenario 10 FAILED: %v should not fallback to cloud", class)
			}
			if err == nil {
				t.Errorf("Scenario 10 FAILED: %v fallback attempt should return error", class)
			}
			if !errors.Is(err, security.ErrClassificationBlocksCloud) {
				t.Errorf("Scenario 10 FAILED: Error should be ErrClassificationBlocksCloud for %v", class)
			}
		}

		t.Log("Scenario 10 PASSED: CUI+ cannot fallback to cloud when local fails")
	})
}

// ============================================================================
// AC-4 PARANOID MODE TESTS
// ============================================================================

// TestAC4_E2E_ParanoidMode tests paranoid mode enforcement.
func TestAC4_E2E_ParanoidMode(t *testing.T) {
	complexQuery := "explain how async runtime works with examples"

	t.Run("Paranoid_Mode_Blocks_Cloud_For_Unclassified", func(t *testing.T) {
		// Paranoid mode should block cloud even for unclassified data
		tier := router.RouteQuery(
			complexQuery,
			security.ClassificationUnclassified,
			true, // paranoidMode = true
			nil,
		)

		if tier != router.TierLocal {
			t.Errorf("Paranoid mode should force local, got %v", tier)
		}
	})

	t.Run("Paranoid_Mode_Blocks_All_Cloud_Tiers", func(t *testing.T) {
		queries := []string{
			"hello",
			"what is rust",
			"explain async await",
			"architect the best solution",
		}

		for _, query := range queries {
			tier := router.RouteQuery(
				query,
				security.ClassificationUnclassified,
				true, // paranoidMode
				nil,
			)

			if !tier.IsLocal() {
				t.Errorf("Paranoid mode: query %q should route local, got %v", query, tier)
			}
		}
	})
}

// ============================================================================
// AC-4 QUERY LENGTH VALIDATION TESTS
// ============================================================================

// TestAC4_E2E_QueryLengthValidation tests query length validation.
func TestAC4_E2E_QueryLengthValidation(t *testing.T) {
	t.Run("Excessive_Query_Rejected", func(t *testing.T) {
		// Create a query longer than MaxQueryLength (100KB)
		longQuery := strings.Repeat("a", router.MaxQueryLength+1)

		// Should be rejected and default to local
		tier := router.RouteQuery(
			longQuery,
			security.ClassificationUnclassified,
			false,
			nil,
		)

		if tier != router.TierLocal {
			t.Errorf("Excessive query should return TierLocal, got %v", tier)
		}
	})

	t.Run("Max_Length_Query_Accepted", func(t *testing.T) {
		// Query exactly at max length should be accepted
		maxQuery := strings.Repeat("a", router.MaxQueryLength)

		// Should be processed normally (will be trivial complexity due to repetition)
		tier := router.RouteQuery(
			maxQuery,
			security.ClassificationUnclassified,
			false,
			nil,
		)

		// Should not error - actual tier depends on complexity classification
		if tier != router.TierCache && tier != router.TierLocal && tier != router.TierCloud {
			t.Errorf("Max length query should route to valid tier, got %v", tier)
		}
	})
}

// ============================================================================
// AC-4 CLASSIFICATION BOUNDARY TESTS
// ============================================================================

// TestAC4_E2E_ClassificationBoundaries tests the exact classification boundaries.
func TestAC4_E2E_ClassificationBoundaries(t *testing.T) {
	complexQuery := "explain how async runtime works"

	t.Run("Unclassified_Below_Boundary", func(t *testing.T) {
		// Unclassified is the ONLY level that allows cloud
		tier := router.RouteQuery(
			complexQuery,
			security.ClassificationUnclassified,
			false,
			nil,
		)

		if tier != router.TierCloud {
			t.Errorf("Unclassified should allow cloud, got %v", tier)
		}
	})

	t.Run("CUI_At_Boundary", func(t *testing.T) {
		// CUI is the first level that blocks cloud
		tier := router.RouteQuery(
			complexQuery,
			security.ClassificationCUI,
			false,
			nil,
		)

		if tier != router.TierLocal {
			t.Errorf("CUI should block cloud, got %v", tier)
		}
	})

	t.Run("Classification_Ordering", func(t *testing.T) {
		// Verify classification ordering is correct
		if security.ClassificationUnclassified >= security.ClassificationCUI {
			t.Error("Unclassified should be < CUI")
		}
		if security.ClassificationCUI >= security.ClassificationConfidential {
			t.Error("CUI should be < Confidential")
		}
		if security.ClassificationConfidential >= security.ClassificationSecret {
			t.Error("Confidential should be < Secret")
		}
		if security.ClassificationSecret >= security.ClassificationTopSecret {
			t.Error("Secret should be < TopSecret")
		}
	})
}

// ============================================================================
// AC-4 REALISTIC SCENARIO TESTS
// ============================================================================

// TestAC4_E2E_RealisticScenarios tests realistic usage scenarios.
func TestAC4_E2E_RealisticScenarios(t *testing.T) {
	t.Run("Government_Contractor_CUI_Project", func(t *testing.T) {
		// Contractor working on CUI-marked project
		classification := security.ClassificationCUI

		projectQueries := []string{
			"explain the authentication flow",
			"review the security implementation",
			"architect the DoD integration module",
			"what is the deployment configuration",
			"how do I fix the STIG compliance issue",
		}

		for _, query := range projectQueries {
			tier := router.RouteQuery(query, classification, false, nil)
			if tier != router.TierLocal {
				t.Errorf("CUI project query %q should stay local, got %v", query, tier)
			}
		}
	})

	t.Run("Airgapped_Network", func(t *testing.T) {
		// System operating in airgapped/paranoid mode
		queries := []string{
			"hello",
			"what is rust",
			"explain async await",
			"should I use microservices",
		}

		for _, query := range queries {
			tier := router.RouteQuery(
				query,
				security.ClassificationUnclassified,
				true, // paranoid mode simulates airgap
				nil,
			)

			if tier != router.TierLocal {
				t.Errorf("Airgapped query %q should stay local, got %v", query, tier)
			}
		}
	})

	t.Run("Classification_Upgrade_During_Session", func(t *testing.T) {
		// Session that upgrades from Unclassified to CUI mid-conversation

		// Start unclassified - cloud allowed
		tier1 := router.RouteQuery(
			"explain async await",
			security.ClassificationUnclassified,
			false,
			nil,
		)
		if tier1 != router.TierCloud {
			t.Errorf("Initial unclassified query should allow cloud, got %v", tier1)
		}

		// Upgrade to CUI - cloud blocked
		tier2 := router.RouteQuery(
			"explain async await",
			security.ClassificationCUI,
			false,
			nil,
		)
		if tier2 != router.TierLocal {
			t.Errorf("After CUI upgrade, query should stay local, got %v", tier2)
		}

		// Highest classification applies for rest of session
		// (This would typically be enforced at session layer)
	})
}

// ============================================================================
// BENCHMARK TESTS
// ============================================================================

// BenchmarkAC4_E2E_FullPipeline benchmarks the full AC-4 enforcement pipeline.
func BenchmarkAC4_E2E_FullPipeline(b *testing.B) {
	enforcer := security.NewClassificationEnforcer(nil, "bench-session")
	query := "explain how async runtime works with examples"

	b.Run("Full_Pipeline_Unclassified", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			classification := security.ClassificationUnclassified

			// Security layer check
			enforcer.RequiresLocalOnly(classification)

			// Router layer check
			router.RouteQuery(query, classification, false, nil)
		}
	})

	b.Run("Full_Pipeline_CUI_Blocked", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			classification := security.ClassificationCUI

			// Security layer check
			enforcer.RequiresLocalOnly(classification)

			// Router layer check
			router.RouteQuery(query, classification, false, nil)

			// Enforcement
			enforcer.EnforceRouting(classification, security.RoutingTierCloud)
		}
	})
}
