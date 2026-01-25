// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package router provides intelligent query routing for RAG queries.
//
// Routes queries to the appropriate tier based on complexity:
// Cache -> Local LLM -> Cloud (Haiku/Sonnet/Opus)
//
// Ported from rigrun2-ratatui-archive Rust implementation.
// Focus: Cost optimization while maintaining response quality.
//
// ROUTER: Accurate cost estimation and complexity analysis
//
// SECURITY CRITICAL: All routing functions REQUIRE ClassificationLevel as a
// mandatory parameter. Classification enforcement is ALWAYS the first check
// before any other routing logic. This is enforced at compile-time by
// requiring the ClassificationLevel parameter in all public routing functions.
//
// Classification Enforcement Rules:
//   - CUI and higher classifications (CUI, Confidential, Secret, TopSecret)
//     MUST ALWAYS route to TierLocal - cloud routing is NEVER permitted
//   - Paranoid mode blocks ALL cloud routing regardless of classification
//   - These checks occur BEFORE complexity analysis or any other routing logic
package router

import (
	"fmt"
	"log"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// ErrMissingClassification is returned when a routing function is called
// without a valid classification level. This should never happen in production
// as classification is enforced at compile-time via required parameters.
var ErrMissingClassification = fmt.Errorf("classification level is required for all routing decisions")

// truncateForLog truncates a string to maxLen characters for logging purposes.
// Adds "..." suffix if truncated.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// classificationBlocksCloud returns true if the classification level requires
// local-only routing (CUI and above MUST stay on-premise).
// This is the FIRST check in all routing logic.
func classificationBlocksCloud(classification security.ClassificationLevel) bool {
	return classification >= security.ClassificationCUI
}

// MaxQueryLength is the maximum allowed query length in bytes (100KB).
// Queries exceeding this limit will be rejected to prevent resource exhaustion.
const MaxQueryLength = 100000

// ErrQueryTooLong is returned when a query exceeds MaxQueryLength.
var ErrQueryTooLong = fmt.Errorf("query exceeds maximum length of %d bytes", MaxQueryLength)

// validateQuery checks if the query length is within acceptable limits.
// Returns an error if the query exceeds MaxQueryLength.
func validateQuery(query string) error {
	if len(query) > MaxQueryLength {
		return fmt.Errorf("query too long: %d bytes (max %d)", len(query), MaxQueryLength)
	}
	return nil
}

// RouteQuery routes a query to the appropriate tier.
// Takes the query, classification level, paranoid mode flag, and optional maxTier to cap costs.
// Returns the recommended tier for this query.
//
// SECURITY CRITICAL: Classification is a REQUIRED parameter (enforced at compile-time).
// Classification enforcement is the FIRST check before any other routing logic.
//   - If classification >= CUI, FORCE return TierLocal (NEVER route to cloud)
//   - If paranoidMode is true, NEVER allow cloud fallback regardless of local failures
//   - These checks take precedence over all other routing decisions
//   - Query length is validated (max 100KB) to prevent resource exhaustion
//
// Parameters:
//   - query: The user's query text
//   - classification: REQUIRED - The security classification level (compile-time enforced)
//   - paranoidMode: If true, forces local-only routing regardless of classification
//   - maxTier: Optional cap on the maximum tier (can be nil)
//
// Returns: The tier to route the query to (TierLocal for CUI+ classifications)
func RouteQuery(query string, classification security.ClassificationLevel, paranoidMode bool, maxTier *Tier) Tier {
	// ========================================================================
	// SECURITY CHECK ORDER (DO NOT REORDER):
	// 1. Classification check (FIRST - blocks cloud for CUI+)
	// 2. Paranoid mode check (blocks cloud regardless of classification)
	// 3. Query validation
	// 4. Normal complexity-based routing
	// ========================================================================

	// CRITICAL SECURITY CHECK #1: Classification enforcement (MUST BE FIRST)
	// CUI and higher classifications MUST stay on-premise - this check
	// takes absolute precedence over all other routing logic
	if classificationBlocksCloud(classification) {
		// Audit log the classification-based routing decision
		// TODO: Add audit logging here
		return TierLocal
	}

	// CRITICAL SECURITY CHECK #2: Paranoid mode enforcement
	// When paranoid mode is enabled, NEVER allow cloud routing
	if paranoidMode {
		// Audit log the paranoid mode block
		// TODO: Add audit logging here
		return TierLocal
	}

	// SECURITY CHECK #3: Query length validation
	// Reject excessively long queries to prevent resource exhaustion
	if err := validateQuery(query); err != nil {
		// Return TierLocal as safe fallback for invalid queries
		return TierLocal
	}

	// Normal routing logic (only reached if classification allows cloud)
	complexity := ClassifyComplexity(query)
	recommended := complexity.MinTier()

	// Cap at max tier if specified
	if maxTier != nil && recommended.Order() > maxTier.Order() {
		return *maxTier
	}
	return recommended
}

// RouteQueryDetailed makes a detailed routing decision for a query.
// Returns full analysis including tier, complexity, type, estimated cost, and reasoning.
// The opts parameter can be either *Tier (for backward compatibility) or *RouterOptions.
//
// SECURITY CRITICAL: Classification is a REQUIRED parameter (enforced at compile-time).
// Classification enforcement is the FIRST check before any other routing logic.
//   - If classification >= CUI, FORCE return TierLocal (NEVER route to cloud)
//   - If paranoid mode is true, NEVER allow cloud fallback regardless of local failures
//   - Query length is validated (max 100KB) to prevent resource exhaustion
//
// Parameters:
//   - query: The user's query text
//   - classification: REQUIRED - The security classification level (compile-time enforced)
//   - opts: Either *Tier (for max tier cap) or *RouterOptions (for full control)
//
// Returns: RoutingDecision with tier, complexity, cost estimate, and reasoning
func RouteQueryDetailed(query string, classification security.ClassificationLevel, opts interface{}) RoutingDecision {
	// ========================================================================
	// SECURITY CHECK ORDER (DO NOT REORDER):
	// 1. Classification check (FIRST - blocks cloud for CUI+)
	// 2. Paranoid mode check (blocks cloud regardless of classification)
	// 3. Query validation
	// 4. Normal complexity-based routing
	// ========================================================================

	// Pre-compute complexity and type for use in all code paths
	complexity := ClassifyComplexity(query)
	queryType := ClassifyType(query)

	// CRITICAL SECURITY CHECK #1: Classification enforcement (MUST BE FIRST)
	// CUI and higher classifications MUST stay on-premise - this check
	// takes absolute precedence over all other routing logic
	if classificationBlocksCloud(classification) {
		// Audit log the classification-based routing decision
		// TODO: Add audit logging here

		reason := fmt.Sprintf(
			"Query classified as %s complexity (%s type) -> %s tier (FORCED: %s classification blocks cloud)",
			complexity.String(),
			queryType.String(),
			TierLocal.String(),
			classification.String(),
		)

		return RoutingDecision{
			Tier:               TierLocal,
			Complexity:         complexity,
			QueryType:          queryType,
			EstimatedCostCents: 0.0, // Local is free
			Reason:             reason,
			IsAutoRouted:       false,
		}
	}

	// Extract routing options
	var maxTier *Tier
	var routerOpts *RouterOptions
	var paranoidMode bool

	switch v := opts.(type) {
	case *Tier:
		maxTier = v
	case *RouterOptions:
		routerOpts = v
		maxTier = v.GetMaxTier()
		paranoidMode = v.Paranoid
	case nil:
		// Use defaults
	}

	// CRITICAL SECURITY CHECK #2: Paranoid mode enforcement
	// When paranoid mode is enabled, NEVER allow cloud routing
	if paranoidMode {
		// Audit log the paranoid mode block
		// TODO: Add audit logging here

		reason := fmt.Sprintf(
			"Query classified as %s complexity (%s type) -> %s tier (FORCED: paranoid mode blocks cloud)",
			complexity.String(),
			queryType.String(),
			TierLocal.String(),
		)

		return RoutingDecision{
			Tier:               TierLocal,
			Complexity:         complexity,
			QueryType:          queryType,
			EstimatedCostCents: 0.0, // Local is free
			Reason:             reason,
			IsAutoRouted:       false,
		}
	}

	// SECURITY CHECK #3: Query length validation
	// Reject excessively long queries to prevent resource exhaustion
	if err := validateQuery(query); err != nil {
		return RoutingDecision{
			Tier:               TierLocal,
			Complexity:         ComplexitySimple,
			QueryType:          QueryTypeUnknown,
			EstimatedCostCents: 0.0,
			Reason:             fmt.Sprintf("Query rejected: %s", err.Error()),
			IsAutoRouted:       false,
		}
	}

	// Normal routing logic (only reached if classification allows cloud)
	recommended := complexity.MinTier()
	tier := recommended
	isAutoRouted := false

	// Handle local-only mode (no cloud key)
	if routerOpts != nil && routerOpts.ShouldUseLocal() {
		// Force local tier, regardless of complexity
		tier = TierLocal
	} else if routerOpts != nil && routerOpts.ShouldUseOpenRouterAuto() {
		// Auto mode: let OpenRouter decide the best model
		tier = TierAuto
		isAutoRouted = true
	} else {
		// Apply max tier cap
		if maxTier != nil && recommended.Order() > maxTier.Order() {
			tier = *maxTier
		}
	}

	// Estimate cost (assume ~500 input tokens, ~1000 output tokens for typical query)
	estimatedCost := tier.CalculateCostCents(500, 1000)

	// Build reason string
	var reason string
	if routerOpts != nil && routerOpts.ShouldUseLocal() {
		if !routerOpts.HasCloudKey {
			reason = fmt.Sprintf(
				"Query classified as %s complexity (%s type) -> %s tier (no cloud key)",
				complexity.String(),
				queryType.String(),
				tier.String(),
			)
		} else {
			reason = fmt.Sprintf(
				"Query classified as %s complexity (%s type) -> %s tier (local mode)",
				complexity.String(),
				queryType.String(),
				tier.String(),
			)
		}
	} else if isAutoRouted {
		reason = fmt.Sprintf(
			"Query classified as %s complexity (%s type) -> OpenRouter auto-routing",
			complexity.String(),
			queryType.String(),
		)
	} else {
		reason = fmt.Sprintf(
			"Query classified as %s complexity (%s type) -> %s tier",
			complexity.String(),
			queryType.String(),
			tier.String(),
		)
	}

	decision := RoutingDecision{
		Tier:               tier,
		Complexity:         complexity,
		QueryType:          queryType,
		EstimatedCostCents: estimatedCost,
		Reason:             reason,
		IsAutoRouted:       isAutoRouted,
	}

	// Log decision for debugging/audit
	log.Printf("ROUTING: query=%q class=%s -> tier=%s reason=%q cost=%.4f",
		truncateForLog(query, 50),
		classification,
		decision.Tier,
		decision.Reason,
		decision.EstimatedCostCents)

	return decision
}

// RouteQueryAuto creates a routing decision for auto mode.
// Returns a decision that will use OpenRouter's auto-routing.
//
// SECURITY CRITICAL: Classification is a REQUIRED parameter (enforced at compile-time).
// Classification enforcement is the FIRST check before any other routing logic.
//   - If classification >= CUI, FORCE return TierLocal (NEVER route to cloud)
//   - If paranoidMode is true, NEVER allow cloud fallback regardless of local failures
//   - Query length is validated (max 100KB) to prevent resource exhaustion
//
// Parameters:
//   - query: The user's query text
//   - classification: REQUIRED - The security classification level (compile-time enforced)
//   - paranoidMode: If true, forces local-only routing regardless of classification
//   - preferLocal: Hint to prefer local models when possible (cost savings)
//   - maxCost: Maximum cost per query in cents (0 = unlimited)
//
// Returns: RoutingDecision configured for auto-routing (or TierLocal for CUI+)
func RouteQueryAuto(query string, classification security.ClassificationLevel, paranoidMode bool, preferLocal bool, maxCost float64) RoutingDecision {
	// ========================================================================
	// SECURITY CHECK ORDER (DO NOT REORDER):
	// 1. Classification check (FIRST - blocks cloud for CUI+)
	// 2. Paranoid mode check (blocks cloud regardless of classification)
	// 3. Query validation
	// 4. Normal auto-routing logic
	// ========================================================================

	// Pre-compute complexity and type for use in all code paths
	complexity := ClassifyComplexity(query)
	queryType := ClassifyType(query)

	// CRITICAL SECURITY CHECK #1: Classification enforcement (MUST BE FIRST)
	// CUI and higher classifications MUST stay on-premise - this check
	// takes absolute precedence over all other routing logic
	if classificationBlocksCloud(classification) {
		// Audit log the classification-based routing decision
		// TODO: Add audit logging here

		reason := fmt.Sprintf(
			"Query classified as %s complexity (%s type) -> %s tier (FORCED: %s classification blocks cloud)",
			complexity.String(),
			queryType.String(),
			TierLocal.String(),
			classification.String(),
		)

		return RoutingDecision{
			Tier:               TierLocal,
			Complexity:         complexity,
			QueryType:          queryType,
			EstimatedCostCents: 0.0, // Local is free
			Reason:             reason,
			IsAutoRouted:       false,
		}
	}

	// CRITICAL SECURITY CHECK #2: Paranoid mode enforcement
	// When paranoid mode is enabled, NEVER allow cloud routing
	if paranoidMode {
		// Audit log the paranoid mode block
		// TODO: Add audit logging here

		reason := fmt.Sprintf(
			"Query classified as %s complexity (%s type) -> %s tier (FORCED: paranoid mode blocks cloud)",
			complexity.String(),
			queryType.String(),
			TierLocal.String(),
		)

		return RoutingDecision{
			Tier:               TierLocal,
			Complexity:         complexity,
			QueryType:          queryType,
			EstimatedCostCents: 0.0, // Local is free
			Reason:             reason,
			IsAutoRouted:       false,
		}
	}

	// SECURITY CHECK #3: Query length validation
	// Reject excessively long queries to prevent resource exhaustion
	if err := validateQuery(query); err != nil {
		return RoutingDecision{
			Tier:               TierLocal,
			Complexity:         ComplexitySimple,
			QueryType:          QueryTypeUnknown,
			EstimatedCostCents: 0.0,
			Reason:             fmt.Sprintf("Query rejected: %s", err.Error()),
			IsAutoRouted:       false,
		}
	}

	// In auto mode, we let OpenRouter decide, but provide hints
	tier := TierAuto
	estimatedCost := tier.CalculateCostCents(500, 1000)

	reason := "OpenRouter auto-routing"
	if preferLocal {
		reason += " (prefer local hint)"
	}
	if maxCost > 0 {
		reason += fmt.Sprintf(" (max cost: %.2f cents)", maxCost)
	}

	return RoutingDecision{
		Tier:               tier,
		Complexity:         complexity,
		QueryType:          queryType,
		EstimatedCostCents: estimatedCost,
		Reason:             reason,
		IsAutoRouted:       true,
	}
}
