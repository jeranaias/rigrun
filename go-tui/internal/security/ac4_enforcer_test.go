// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// AC-4 Classification Enforcer Tests
// Per NIST SP 800-53 AC-4: Information Flow Enforcement
//
// These tests verify the ClassificationEnforcer correctly blocks unauthorized
// information flow from classified systems to cloud tiers.

package security

import (
	"errors"
	"strings"
	"testing"
)

// ============================================================================
// AC-4 CLASSIFICATION ENFORCER - CORE TESTS
// ============================================================================

// TestAC4_ClassificationEnforcer tests the core enforcement logic.
func TestAC4_ClassificationEnforcer(t *testing.T) {
	enforcer := NewClassificationEnforcer(nil, "test-session")

	tests := []struct {
		name           string
		classification ClassificationLevel
		requestedTier  RoutingTier
		expectTier     RoutingTier
		expectError    bool
		description    string
	}{
		// =====================================================================
		// Scenario 1: Unclassified -> Cloud: ALLOWED
		// =====================================================================
		{
			name:           "AC4_1_Unclassified_to_Cloud_ALLOWED",
			classification: ClassificationUnclassified,
			requestedTier:  RoutingTierCloud,
			expectTier:     RoutingTierCloud,
			expectError:    false,
			description:    "Unclassified data CAN flow to cloud",
		},
		{
			name:           "AC4_1_Unclassified_to_Local_ALLOWED",
			classification: ClassificationUnclassified,
			requestedTier:  RoutingTierLocal,
			expectTier:     RoutingTierLocal,
			expectError:    false,
			description:    "Unclassified data can stay local",
		},

		// =====================================================================
		// Scenario 2: CUI -> Local: ALLOWED
		// =====================================================================
		{
			name:           "AC4_2_CUI_to_Local_ALLOWED",
			classification: ClassificationCUI,
			requestedTier:  RoutingTierLocal,
			expectTier:     RoutingTierLocal,
			expectError:    false,
			description:    "CUI data CAN stay local",
		},

		// =====================================================================
		// Scenario 3: CUI -> Cloud: BLOCKED
		// =====================================================================
		{
			name:           "AC4_3_CUI_to_Cloud_BLOCKED",
			classification: ClassificationCUI,
			requestedTier:  RoutingTierCloud,
			expectTier:     RoutingTierLocal, // Forced downgrade
			expectError:    true,             // Error indicates block
			description:    "CUI data MUST NOT flow to cloud",
		},

		// =====================================================================
		// Scenario 4: Secret -> Cloud: BLOCKED
		// =====================================================================
		{
			name:           "AC4_4_Secret_to_Cloud_BLOCKED",
			classification: ClassificationSecret,
			requestedTier:  RoutingTierCloud,
			expectTier:     RoutingTierLocal,
			expectError:    true,
			description:    "Secret data MUST NOT flow to cloud",
		},
		{
			name:           "AC4_4_Secret_to_Local_ALLOWED",
			classification: ClassificationSecret,
			requestedTier:  RoutingTierLocal,
			expectTier:     RoutingTierLocal,
			expectError:    false,
			description:    "Secret data CAN stay local",
		},

		// =====================================================================
		// Scenario 5: Top Secret -> Cloud: BLOCKED
		// =====================================================================
		{
			name:           "AC4_5_TopSecret_to_Cloud_BLOCKED",
			classification: ClassificationTopSecret,
			requestedTier:  RoutingTierCloud,
			expectTier:     RoutingTierLocal,
			expectError:    true,
			description:    "Top Secret data MUST NOT flow to cloud",
		},
		{
			name:           "AC4_5_TopSecret_to_Local_ALLOWED",
			classification: ClassificationTopSecret,
			requestedTier:  RoutingTierLocal,
			expectTier:     RoutingTierLocal,
			expectError:    false,
			description:    "Top Secret data CAN stay local",
		},

		// =====================================================================
		// Scenario: Confidential -> Cloud: BLOCKED
		// =====================================================================
		{
			name:           "AC4_Confidential_to_Cloud_BLOCKED",
			classification: ClassificationConfidential,
			requestedTier:  RoutingTierCloud,
			expectTier:     RoutingTierLocal,
			expectError:    true,
			description:    "Confidential data MUST NOT flow to cloud",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier, err := enforcer.EnforceRouting(tt.classification, tt.requestedTier)

			if tt.expectError {
				if err == nil {
					t.Errorf("[%s] EnforceRouting() expected error but got none\n"+
						"  Classification: %v\n"+
						"  RequestedTier: %v\n"+
						"  Description: %s",
						tt.name, tt.classification, tt.requestedTier, tt.description)
				} else if !errors.Is(err, ErrClassificationBlocksCloud) {
					t.Errorf("[%s] Expected ErrClassificationBlocksCloud, got: %v",
						tt.name, err)
				}
			} else {
				if err != nil {
					t.Errorf("[%s] EnforceRouting() unexpected error: %v",
						tt.name, err)
				}
			}

			if tier != tt.expectTier {
				t.Errorf("[%s] EnforceRouting() tier = %v, want %v",
					tt.name, tier, tt.expectTier)
			}
		})
	}
}

// ============================================================================
// AC-4 CAN ROUTE TO CLOUD TESTS
// ============================================================================

// TestAC4_CanRouteToCloud tests the CanRouteToCloud method.
func TestAC4_CanRouteToCloud(t *testing.T) {
	enforcer := NewClassificationEnforcer(nil, "test-session")

	tests := []struct {
		name           string
		classification ClassificationLevel
		canRoute       bool
	}{
		{
			name:           "Unclassified_can_route",
			classification: ClassificationUnclassified,
			canRoute:       true,
		},
		{
			name:           "CUI_cannot_route",
			classification: ClassificationCUI,
			canRoute:       false,
		},
		{
			name:           "Confidential_cannot_route",
			classification: ClassificationConfidential,
			canRoute:       false,
		},
		{
			name:           "Secret_cannot_route",
			classification: ClassificationSecret,
			canRoute:       false,
		},
		{
			name:           "TopSecret_cannot_route",
			classification: ClassificationTopSecret,
			canRoute:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := enforcer.CanRouteToCloud(tt.classification)
			if result != tt.canRoute {
				t.Errorf("CanRouteToCloud(%v) = %v, want %v",
					tt.classification, result, tt.canRoute)
			}
		})
	}
}

// ============================================================================
// AC-4 REQUIRES LOCAL ONLY TESTS
// ============================================================================

// TestAC4_RequiresLocalOnly tests the RequiresLocalOnly convenience method.
func TestAC4_RequiresLocalOnly(t *testing.T) {
	enforcer := NewClassificationEnforcer(nil, "test-session")

	tests := []struct {
		name           string
		classification ClassificationLevel
		requiresLocal  bool
	}{
		{
			name:           "Unclassified_does_not_require_local",
			classification: ClassificationUnclassified,
			requiresLocal:  false,
		},
		{
			name:           "CUI_requires_local",
			classification: ClassificationCUI,
			requiresLocal:  true,
		},
		{
			name:           "Confidential_requires_local",
			classification: ClassificationConfidential,
			requiresLocal:  true,
		},
		{
			name:           "Secret_requires_local",
			classification: ClassificationSecret,
			requiresLocal:  true,
		},
		{
			name:           "TopSecret_requires_local",
			classification: ClassificationTopSecret,
			requiresLocal:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := enforcer.RequiresLocalOnly(tt.classification)
			if result != tt.requiresLocal {
				t.Errorf("RequiresLocalOnly(%v) = %v, want %v",
					tt.classification, result, tt.requiresLocal)
			}
		})
	}
}

// ============================================================================
// AC-4 VALIDATE ROUTING DECISION TESTS
// ============================================================================

// TestAC4_ValidateRoutingDecision tests routing decision validation.
func TestAC4_ValidateRoutingDecision(t *testing.T) {
	enforcer := NewClassificationEnforcer(nil, "test-session")

	tests := []struct {
		name           string
		classification ClassificationLevel
		tier           RoutingTier
		expectError    bool
	}{
		{
			name:           "Valid_Unclassified_Cloud",
			classification: ClassificationUnclassified,
			tier:           RoutingTierCloud,
			expectError:    false,
		},
		{
			name:           "Valid_Unclassified_Local",
			classification: ClassificationUnclassified,
			tier:           RoutingTierLocal,
			expectError:    false,
		},
		{
			name:           "Valid_CUI_Local",
			classification: ClassificationCUI,
			tier:           RoutingTierLocal,
			expectError:    false,
		},
		{
			name:           "Invalid_CUI_Cloud",
			classification: ClassificationCUI,
			tier:           RoutingTierCloud,
			expectError:    true,
		},
		{
			name:           "Invalid_Secret_Cloud",
			classification: ClassificationSecret,
			tier:           RoutingTierCloud,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforcer.ValidateRoutingDecision(tt.classification, tt.tier)

			if tt.expectError && err == nil {
				t.Errorf("ValidateRoutingDecision() expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateRoutingDecision() unexpected error: %v", err)
			}
		})
	}
}

// ============================================================================
// AC-4 ENFORCER ENABLED/DISABLED TESTS
// ============================================================================

// TestAC4_EnforcerEnabled tests the enabled flag behavior.
func TestAC4_EnforcerEnabled(t *testing.T) {
	enforcer := NewClassificationEnforcer(nil, "test-session")

	t.Run("Enforcer_Default_Enabled", func(t *testing.T) {
		if !enforcer.IsEnabled() {
			t.Error("Enforcer should be enabled by default for security")
		}
	})

	t.Run("Enabled_Blocks_CUI_Cloud", func(t *testing.T) {
		enforcer.SetEnabled(true)

		tier, err := enforcer.EnforceRouting(ClassificationCUI, RoutingTierCloud)
		if err == nil {
			t.Error("Enabled enforcer should block CUI -> Cloud")
		}
		if tier != RoutingTierLocal {
			t.Errorf("Enabled enforcer should return Local, got %v", tier)
		}
	})

	t.Run("Disabled_Allows_CUI_Cloud_TESTING_ONLY", func(t *testing.T) {
		// WARNING: This is ONLY for testing scenarios
		enforcer.SetEnabled(false)

		tier, err := enforcer.EnforceRouting(ClassificationCUI, RoutingTierCloud)
		if err != nil {
			t.Errorf("Disabled enforcer should not return error, got %v", err)
		}
		if tier != RoutingTierCloud {
			t.Errorf("Disabled enforcer should allow Cloud, got %v", tier)
		}

		// Re-enable for safety
		enforcer.SetEnabled(true)
	})
}

// ============================================================================
// AC-4 ROUTING TIER STRING TESTS
// ============================================================================

// TestAC4_RoutingTierString tests the RoutingTier String method.
func TestAC4_RoutingTierString(t *testing.T) {
	tests := []struct {
		tier     RoutingTier
		expected string
	}{
		{RoutingTierLocal, "Local"},
		{RoutingTierCloud, "Cloud"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.tier.String()
			if result != tt.expected {
				t.Errorf("RoutingTier.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// AC-4 CLASSIFICATION RESTRICTIONS TEXT TESTS
// ============================================================================

// TestAC4_GetClassificationRestrictions tests the restriction descriptions.
func TestAC4_GetClassificationRestrictions(t *testing.T) {
	enforcer := NewClassificationEnforcer(nil, "test-session")

	tests := []struct {
		classification ClassificationLevel
		mustContain    string
	}{
		{ClassificationUnclassified, "cloud allowed"},
		{ClassificationCUI, "BLOCKED"},
		{ClassificationSecret, "BLOCKED"},
		{ClassificationTopSecret, "BLOCKED"},
	}

	for _, tt := range tests {
		t.Run(tt.classification.String(), func(t *testing.T) {
			result := enforcer.GetClassificationRestrictions(tt.classification)
			if !strings.Contains(strings.ToLower(result), strings.ToLower(tt.mustContain)) {
				t.Errorf("GetClassificationRestrictions(%v) = %q, should contain %q",
					tt.classification, result, tt.mustContain)
			}
		})
	}
}

// ============================================================================
// AC-4 ERROR TYPE TESTS
// ============================================================================

// TestAC4_ErrorType verifies the error returned is the correct type.
func TestAC4_ErrorType(t *testing.T) {
	enforcer := NewClassificationEnforcer(nil, "test-session")

	_, err := enforcer.EnforceRouting(ClassificationSecret, RoutingTierCloud)

	if err == nil {
		t.Fatal("Expected error for Secret -> Cloud routing")
	}

	if !errors.Is(err, ErrClassificationBlocksCloud) {
		t.Errorf("Error should be ErrClassificationBlocksCloud, got %v", err)
	}

	// Verify error message contains useful information
	errMsg := err.Error()
	if !strings.Contains(errMsg, "SECRET") {
		t.Errorf("Error message should contain classification level, got %q", errMsg)
	}
	if !strings.Contains(errMsg, "Cloud") {
		t.Errorf("Error message should contain tier name, got %q", errMsg)
	}
}

// ============================================================================
// AC-4 SESSION ID TESTS
// ============================================================================

// TestAC4_SessionID tests session ID management.
func TestAC4_SessionID(t *testing.T) {
	enforcer := NewClassificationEnforcer(nil, "initial-session")

	t.Run("Initial_Session_ID", func(t *testing.T) {
		// We can't directly access sessionID, but we can verify behavior
		// The session ID is used for audit logging correlation
	})

	t.Run("Update_Session_ID", func(t *testing.T) {
		enforcer.SetSessionID("updated-session")
		// Session ID update should not affect enforcement
		tier, _ := enforcer.EnforceRouting(ClassificationCUI, RoutingTierCloud)
		if tier != RoutingTierLocal {
			t.Error("Session ID change should not affect enforcement")
		}
	})
}

// ============================================================================
// AC-4 GLOBAL ENFORCER TESTS
// ============================================================================

// TestAC4_NewClassificationEnforcerGlobal tests the global factory.
func TestAC4_NewClassificationEnforcerGlobal(t *testing.T) {
	enforcer := NewClassificationEnforcerGlobal("global-test-session")

	if enforcer == nil {
		t.Fatal("NewClassificationEnforcerGlobal should not return nil")
	}

	// Verify it enforces correctly
	if enforcer.CanRouteToCloud(ClassificationCUI) {
		t.Error("Global enforcer should block CUI from cloud")
	}
}

// ============================================================================
// AC-4 SCENARIO 7: INVALID CLASSIFICATION HEADER
// ============================================================================

// TestAC4_7_InvalidClassificationHeader tests handling of invalid classification headers.
// Scenario 7: Invalid classification header: Should return error
func TestAC4_7_InvalidClassificationHeader(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "Invalid_Classification_String",
			input:       "INVALID_LEVEL",
			expectError: true,
		},
		{
			name:        "Random_String",
			input:       "FOOBAR",
			expectError: true,
		},
		{
			name:        "Typo_in_Classification",
			input:       "SECERT",
			expectError: true,
		},
		{
			name:        "Partial_Classification",
			input:       "TOP",
			expectError: true,
		},
		{
			name:        "Numbers_Only",
			input:       "12345",
			expectError: true,
		},
		{
			name:        "SQL_Injection_Attempt",
			input:       "SECRET'; DROP TABLE users;--",
			expectError: true,
		},
		{
			name:        "Valid_Uppercase",
			input:       "SECRET",
			expectError: false,
		},
		{
			name:        "Valid_Lowercase",
			input:       "secret",
			expectError: false,
		},
		{
			name:        "Valid_With_Caveats",
			input:       "SECRET//NOFORN",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseClassification(tt.input)

			if tt.expectError && err == nil {
				t.Errorf("ParseClassification(%q) expected error but got none", tt.input)
			}
			if !tt.expectError && err != nil {
				t.Errorf("ParseClassification(%q) unexpected error: %v", tt.input, err)
			}
		})
	}
}

// ============================================================================
// AC-4 SCENARIO 6: MISSING CLASSIFICATION HEADER
// ============================================================================

// TestAC4_6_MissingClassificationHeader tests default behavior for missing headers.
// Scenario 6: Missing classification header: Should default to Unclassified
func TestAC4_6_MissingClassificationHeader(t *testing.T) {
	t.Run("Empty_String_Defaults_To_Unclassified", func(t *testing.T) {
		class, err := ParseClassification("")
		if err != nil {
			t.Fatalf("ParseClassification(\"\") returned error: %v", err)
		}
		if class.Level != ClassificationUnclassified {
			t.Errorf("Empty string should default to Unclassified, got %v", class.Level)
		}
	})

	t.Run("Default_Classification_Is_Unclassified", func(t *testing.T) {
		def := DefaultClassification()
		if def.Level != ClassificationUnclassified {
			t.Errorf("DefaultClassification().Level = %v, want Unclassified", def.Level)
		}
	})

	t.Run("ClassificationFromEnv_Empty_Defaults", func(t *testing.T) {
		class := ClassificationFromEnv("")
		if class.Level != ClassificationUnclassified {
			t.Errorf("ClassificationFromEnv(\"\") should default to Unclassified, got %v",
				class.Level)
		}
	})
}

// ============================================================================
// AC-4 INTEGRATION SCENARIO TESTS
// ============================================================================

// TestAC4_8_TUILayerEnforcement tests classification enforcement at TUI layer.
// Scenario 8: Classification enforcement at TUI layer: Verify blocks
func TestAC4_8_TUILayerEnforcement(t *testing.T) {
	// The TUI layer uses ClassificationEnforcer.RequiresLocalOnly() to check
	// if cloud routing should be blocked before making routing decisions.

	enforcer := NewClassificationEnforcer(nil, "test-session")

	t.Run("TUI_Blocks_CUI_Before_Routing", func(t *testing.T) {
		// Simulate TUI check before routing decision
		classification := ClassificationCUI

		if enforcer.RequiresLocalOnly(classification) {
			// TUI should force local tier
			tier, err := enforcer.EnforceRouting(classification, RoutingTierCloud)
			if tier != RoutingTierLocal {
				t.Error("TUI layer should force local for CUI")
			}
			if err == nil {
				t.Error("Enforcement should return error to indicate block")
			}
		} else {
			t.Error("RequiresLocalOnly should return true for CUI")
		}
	})

	t.Run("TUI_Allows_Unclassified_Cloud", func(t *testing.T) {
		classification := ClassificationUnclassified

		if enforcer.RequiresLocalOnly(classification) {
			t.Error("RequiresLocalOnly should return false for Unclassified")
		}

		// TUI can proceed with normal routing
		tier, err := enforcer.EnforceRouting(classification, RoutingTierCloud)
		if tier != RoutingTierCloud {
			t.Error("Unclassified should be allowed to route to Cloud")
		}
		if err != nil {
			t.Errorf("Unclassified -> Cloud should not return error: %v", err)
		}
	})
}

// TestAC4_9_RouterLayerEnforcement tests classification enforcement at router layer.
// Scenario 9: Classification enforcement at router layer: Verify blocks
func TestAC4_9_RouterLayerEnforcement(t *testing.T) {
	// The router layer uses classification parameter directly in RouteQuery
	// to enforce classification before any routing logic.

	enforcer := NewClassificationEnforcer(nil, "test-session")

	t.Run("Router_Validates_Before_Decision", func(t *testing.T) {
		// Validate the routing decision after it's made
		classification := ClassificationSecret

		// Simulate a routing decision that would go to cloud
		proposedTier := RoutingTierCloud

		// Validate it
		err := enforcer.ValidateRoutingDecision(classification, proposedTier)
		if err == nil {
			t.Error("Router should reject Secret -> Cloud decision")
		}

		// Enforce correction
		correctedTier, _ := enforcer.EnforceRouting(classification, proposedTier)
		if correctedTier != RoutingTierLocal {
			t.Error("Router should correct to Local")
		}
	})
}

// TestAC4_10_FallbackBlockedForCUIPlus tests fallback prevention.
// Scenario 10: Fallback blocked for CUI+: When local fails, CUI must not fallback to cloud
func TestAC4_10_FallbackBlockedForCUIPlus(t *testing.T) {
	enforcer := NewClassificationEnforcer(nil, "test-session")

	// Simulate local failure scenario
	localFailed := true

	classifications := []ClassificationLevel{
		ClassificationCUI,
		ClassificationConfidential,
		ClassificationSecret,
		ClassificationTopSecret,
	}

	for _, class := range classifications {
		t.Run("No_Fallback_"+class.String(), func(t *testing.T) {
			if localFailed {
				// Try to fallback to cloud
				tier, err := enforcer.EnforceRouting(class, RoutingTierCloud)

				// Should be blocked
				if tier == RoutingTierCloud {
					t.Errorf("%s: Fallback to cloud should be blocked", class.String())
				}
				if err == nil {
					t.Errorf("%s: Fallback attempt should return error", class.String())
				}

				// The only option is to stay local (and handle the failure there)
				if tier != RoutingTierLocal {
					t.Errorf("%s: Must stay on local tier, got %v", class.String(), tier)
				}
			}
		})
	}
}

// ============================================================================
// BENCHMARK TESTS
// ============================================================================

// BenchmarkAC4_EnforceRouting benchmarks the enforcement check.
func BenchmarkAC4_EnforceRouting(b *testing.B) {
	enforcer := NewClassificationEnforcer(nil, "bench-session")

	b.Run("Unclassified_Cloud", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			enforcer.EnforceRouting(ClassificationUnclassified, RoutingTierCloud)
		}
	})

	b.Run("CUI_Cloud_Blocked", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			enforcer.EnforceRouting(ClassificationCUI, RoutingTierCloud)
		}
	})

	b.Run("Secret_Local", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			enforcer.EnforceRouting(ClassificationSecret, RoutingTierLocal)
		}
	})
}

// BenchmarkAC4_CanRouteToCloud benchmarks the permission check.
func BenchmarkAC4_CanRouteToCloud(b *testing.B) {
	enforcer := NewClassificationEnforcer(nil, "bench-session")

	classifications := []ClassificationLevel{
		ClassificationUnclassified,
		ClassificationCUI,
		ClassificationSecret,
		ClassificationTopSecret,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, class := range classifications {
			enforcer.CanRouteToCloud(class)
		}
	}
}
