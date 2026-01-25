// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// ROUTER: Accurate cost estimation and complexity analysis
package router

import (
	"fmt"
	"strings"
	"sync"
)

// ============================================================================
// TOKEN ESTIMATION
// ============================================================================

// EstimateTokens provides more accurate token counting for cost estimation.
// GPT-style: ~4 chars per token on average.
// Uses a blend of word and character estimates for better accuracy.
func EstimateTokens(text string) int {
	// GPT-style: ~4 chars per token on average
	// More accurate: use tiktoken or similar
	words := len(strings.Fields(text))
	chars := len(text)

	// Blend of word and char estimates
	return (words + chars/4) / 2
}

// TierPricing holds input and output pricing per 1K tokens in cents.
type TierPricing struct {
	Input  float64 // Cost per 1K input tokens in cents
	Output float64 // Cost per 1K output tokens in cents
}

// GetTierPricing returns the pricing for a given tier.
// Returns nil if the tier is not recognized.
func GetTierPricing(tier Tier) *TierPricing {
	// Per-model pricing (per 1K tokens in cents)
	// Updated pricing as of 2024
	pricing := map[Tier]TierPricing{
		TierCache:  {0.0, 0.0},
		TierLocal:  {0.0, 0.0},
		TierHaiku:  {0.025, 0.125},   // $0.25/M input, $1.25/M output
		TierSonnet: {0.3, 1.5},       // $3/M input, $15/M output
		TierOpus:   {1.5, 7.5},       // $15/M input, $75/M output
		TierGpt4o:  {0.25, 1.0},      // $2.5/M input, $10/M output
		TierAuto:   {0.03, 0.15},     // OpenRouter auto average
		TierCloud:  {0.03, 0.15},     // Legacy alias for auto
	}

	if p, ok := pricing[tier]; ok {
		return &p
	}
	return nil
}

// EstimateCost calculates estimated cost for a given token count and tier.
// Assumes a 3:1 output:input ratio for typical queries.
func EstimateCost(inputTokens int, tier Tier) float64 {
	pricing := GetTierPricing(tier)
	if pricing == nil {
		return 0
	}

	// Assume 3:1 output:input ratio for typical queries
	outputTokens := inputTokens * 3

	// Calculate cost in cents
	inputCost := (float64(inputTokens) * pricing.Input) / 1000
	outputCost := (float64(outputTokens) * pricing.Output) / 1000

	return inputCost + outputCost
}

// EstimateCostWithRatio calculates estimated cost with a custom output:input ratio.
func EstimateCostWithRatio(inputTokens int, tier Tier, outputRatio float64) float64 {
	pricing := GetTierPricing(tier)
	if pricing == nil {
		return 0
	}

	outputTokens := int(float64(inputTokens) * outputRatio)

	// Calculate cost in cents
	inputCost := (float64(inputTokens) * pricing.Input) / 1000
	outputCost := (float64(outputTokens) * pricing.Output) / 1000

	return inputCost + outputCost
}

// ============================================================================
// TIER COST EXTENSIONS
// ============================================================================

// Name returns the human-readable name of the tier.
// This is an alias for String() for API compatibility.
func (t Tier) Name() string {
	return t.String()
}

// CalculateSavingsVsOpus calculates how much was saved by using this tier
// instead of Opus for the same query.
//
// Returns the savings in cents (positive means money saved).
//
// Pricing reference: 2024 API pricing
//   - Opus input: $15/M = 1.5 cents/1K tokens
//   - Opus output: $75/M = 7.5 cents/1K tokens
func CalculateSavingsVsOpus(tier Tier, inputTokens, outputTokens int) float64 {
	opusCost := TierOpus.CalculateCostCents(uint32(inputTokens), uint32(outputTokens))
	actualCost := tier.CalculateCostCents(uint32(inputTokens), uint32(outputTokens))
	return opusCost - actualCost
}

// ============================================================================
// SESSION STATISTICS
// ============================================================================

// SessionStats tracks cumulative statistics for a routing session.
// All fields are safe for concurrent access.
type SessionStats struct {
	mu sync.RWMutex

	// TotalQueries is the total number of queries processed.
	TotalQueries int `json:"total_queries"`
	// LocalQueries is the number of queries handled by local tier.
	LocalQueries int `json:"local_queries"`
	// CacheHits is the number of queries served from cache.
	CacheHits int `json:"cache_hits"`
	// CloudQueries is the number of queries sent to cloud tiers.
	CloudQueries int `json:"cloud_queries"`
	// TotalCostCents is the cumulative cost in cents.
	TotalCostCents float64 `json:"total_cost_cents"`
	// TotalSavedCents is the cumulative savings vs Opus in cents.
	TotalSavedCents float64 `json:"total_saved_cents"`
	// TotalInputTokens is the cumulative input tokens used.
	TotalInputTokens int `json:"total_input_tokens"`
	// TotalOutputTokens is the cumulative output tokens generated.
	TotalOutputTokens int `json:"total_output_tokens"`
}

// NewSessionStats creates a new SessionStats instance.
func NewSessionStats() *SessionStats {
	return &SessionStats{}
}

// RecordQuery updates session statistics with a new query result.
// Thread-safe for concurrent access.
func (s *SessionStats) RecordQuery(result QueryResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalQueries++
	s.TotalCostCents += result.CostCents
	s.TotalInputTokens += int(result.InputTokens)
	s.TotalOutputTokens += int(result.OutputTokens)

	// Calculate savings vs using Opus for this query
	savings := CalculateSavingsVsOpus(result.TierUsed, int(result.InputTokens), int(result.OutputTokens))
	s.TotalSavedCents += savings

	// Track query distribution by tier type
	// Note: CacheHit flag OR TierCache tier indicates a cache hit
	switch result.TierUsed {
	case TierCache:
		s.CacheHits++
	case TierLocal:
		// Also count explicit CacheHit flag for Local tier (semantic cache)
		if result.CacheHit {
			s.CacheHits++
		}
		s.LocalQueries++
	default:
		// All other tiers are cloud-based (Cloud, Haiku, Sonnet, Opus, Gpt4o)
		s.CloudQueries++
	}
}

// Summary returns a human-readable summary of the session statistics.
func (s *SessionStats) Summary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.TotalQueries == 0 {
		return "No queries processed yet"
	}

	// Calculate percentages
	cachePercent := 0.0
	localPercent := 0.0
	cloudPercent := 0.0

	if s.TotalQueries > 0 {
		cachePercent = float64(s.CacheHits) / float64(s.TotalQueries) * 100
		localPercent = float64(s.LocalQueries) / float64(s.TotalQueries) * 100
		cloudPercent = float64(s.CloudQueries) / float64(s.TotalQueries) * 100
	}

	return fmt.Sprintf(
		"Session Stats: %d queries (%.0f%% cache, %.0f%% local, %.0f%% cloud) | Cost: %.4f cents | Saved: %.4f cents vs Opus",
		s.TotalQueries,
		cachePercent,
		localPercent,
		cloudPercent,
		s.TotalCostCents,
		s.TotalSavedCents,
	)
}

// Reset clears all session statistics.
func (s *SessionStats) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalQueries = 0
	s.LocalQueries = 0
	s.CacheHits = 0
	s.CloudQueries = 0
	s.TotalCostCents = 0.0
	s.TotalSavedCents = 0.0
	s.TotalInputTokens = 0
	s.TotalOutputTokens = 0
}

// GetStats returns a copy of the current statistics (thread-safe snapshot).
func (s *SessionStats) GetStats() SessionStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return SessionStats{
		TotalQueries:      s.TotalQueries,
		LocalQueries:      s.LocalQueries,
		CacheHits:         s.CacheHits,
		CloudQueries:      s.CloudQueries,
		TotalCostCents:    s.TotalCostCents,
		TotalSavedCents:   s.TotalSavedCents,
		TotalInputTokens:  s.TotalInputTokens,
		TotalOutputTokens: s.TotalOutputTokens,
	}
}

// TotalTokens returns the total tokens (input + output) used in the session.
func (s *SessionStats) TotalTokens() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TotalInputTokens + s.TotalOutputTokens
}

// CostEfficiencyPercent returns what percentage of Opus cost was actually spent.
// Returns 0 if no queries have been processed.
// 100% means same cost as Opus, lower is better.
func (s *SessionStats) CostEfficiencyPercent() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalOpusCost := s.TotalCostCents + s.TotalSavedCents
	if totalOpusCost == 0 {
		return 0.0
	}
	return (s.TotalCostCents / totalOpusCost) * 100
}
