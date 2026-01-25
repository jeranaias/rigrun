// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package router provides intelligent query routing for the rigrun RAG system.
//
// Routes queries to the appropriate tier based on complexity:
// Cache -> Local LLM -> Cloud (Haiku/Sonnet/Opus/GPT-4o)
//
// Focus: Cost optimization while maintaining response quality.
package router

import "fmt"

// ============================================================================
// TIER TYPE
// ============================================================================

// Tier represents a model tier for routing decisions.
// Ordered by cost/capability: Cache < Local < Cloud < Haiku < Sonnet < Opus < GPT-4o
type Tier int

const (
	// TierCache represents cached responses (free, instant).
	TierCache Tier = iota
	// TierLocal represents local Ollama model inference.
	TierLocal
	// TierAuto represents OpenRouter auto-routing (OpenRouter decides the model).
	TierAuto
	// TierCloud represents OpenRouter auto-selection (legacy, alias for TierAuto).
	TierCloud
	// TierHaiku represents Claude 3 Haiku (fast, cheap).
	TierHaiku
	// TierSonnet represents Claude 3 Sonnet (balanced).
	TierSonnet
	// TierOpus represents Claude 3 Opus (powerful).
	TierOpus
	// TierGpt4o represents OpenAI GPT-4o.
	TierGpt4o
)

// String returns the human-readable name of the tier.
func (t Tier) String() string {
	switch t {
	case TierCache:
		return "Cache"
	case TierLocal:
		return "Local"
	case TierAuto:
		return "Auto"
	case TierCloud:
		return "Cloud"
	case TierHaiku:
		return "Haiku"
	case TierSonnet:
		return "Sonnet"
	case TierOpus:
		return "Opus"
	case TierGpt4o:
		return "GPT-4o"
	default:
		return fmt.Sprintf("Tier(%d)", t)
	}
}

// IsLocal returns true if the tier is a local/free tier (Cache or Local).
func (t Tier) IsLocal() bool {
	return t == TierCache || t == TierLocal
}

// IsAuto returns true if the tier is auto-routed by OpenRouter.
func (t Tier) IsAuto() bool {
	return t == TierAuto || t == TierCloud
}

// IsPaid returns true if the tier incurs API costs (cloud tiers).
func (t Tier) IsPaid() bool {
	return t >= TierAuto
}

// Order returns the numeric order of the tier for comparison.
// Lower values mean cheaper/faster tiers.
func (t Tier) Order() int {
	return int(t)
}

// InputCostPer1K returns the cost per 1K input tokens in cents.
//
// Pricing as of 2024:
//   - Cache/Local: Free
//   - Cloud (OpenRouter auto): $0.3/M average = 0.03 cents/1K
//   - Haiku: $0.25/M input = 0.025 cents/1K
//   - Sonnet: $3/M input = 0.3 cents/1K
//   - Opus: $15/M input = 1.5 cents/1K
//   - GPT-4o: $2.5/M input = 0.25 cents/1K
func (t Tier) InputCostPer1K() float64 {
	switch t {
	case TierCache, TierLocal:
		return 0.0
	case TierAuto, TierCloud:
		return 0.03 // OpenRouter auto picks optimal model
	case TierHaiku:
		return 0.025
	case TierSonnet:
		return 0.3
	case TierOpus:
		return 1.5
	case TierGpt4o:
		return 0.25
	default:
		return 0.0
	}
}

// OutputCostPer1K returns the cost per 1K output tokens in cents.
//
// Pricing as of 2024:
//   - Cache/Local: Free
//   - Cloud (OpenRouter auto): $1.5/M average = 0.15 cents/1K
//   - Haiku: $1.25/M output = 0.125 cents/1K
//   - Sonnet: $15/M output = 1.5 cents/1K
//   - Opus: $75/M output = 7.5 cents/1K
//   - GPT-4o: $10/M output = 1.0 cents/1K
func (t Tier) OutputCostPer1K() float64 {
	switch t {
	case TierCache, TierLocal:
		return 0.0
	case TierAuto, TierCloud:
		return 0.15
	case TierHaiku:
		return 0.125
	case TierSonnet:
		return 1.5
	case TierOpus:
		return 7.5
	case TierGpt4o:
		return 1.0
	default:
		return 0.0
	}
}

// CalculateCostCents calculates the total cost for a request in cents.
func (t Tier) CalculateCostCents(inputTokens, outputTokens uint32) float64 {
	inputCost := (float64(inputTokens) / 1000.0) * t.InputCostPer1K()
	outputCost := (float64(outputTokens) / 1000.0) * t.OutputCostPer1K()
	return inputCost + outputCost
}

// TypicalLatencyMs returns the typical latency in milliseconds for this tier.
func (t Tier) TypicalLatencyMs() uint32 {
	switch t {
	case TierCache:
		return 1
	case TierLocal:
		return 500
	case TierCloud:
		return 1000
	case TierHaiku:
		return 800
	case TierSonnet:
		return 1500
	case TierOpus:
		return 3000
	case TierGpt4o:
		return 1200
	default:
		return 1000
	}
}

// Escalate returns the next tier up for escalation on failure.
// Returns nil if there is no higher tier to escalate to.
func (t Tier) Escalate() *Tier {
	var next Tier
	switch t {
	case TierCache:
		next = TierLocal
	case TierLocal:
		next = TierCloud
	case TierHaiku:
		next = TierSonnet
	case TierSonnet:
		next = TierOpus
	default:
		return nil // Cloud, Opus, Gpt4o have no escalation
	}
	return &next
}

// ============================================================================
// QUERY COMPLEXITY
// ============================================================================

// QueryComplexity represents the complexity level of a query.
// Determines the minimum tier required for adequate response.
type QueryComplexity int

const (
	// ComplexityTrivial represents simple lookup, pattern match (< 5 words).
	// Routes to cache or local tier.
	ComplexityTrivial QueryComplexity = iota
	// ComplexitySimple represents basic questions, single-step reasoning.
	// Routes to local tier.
	ComplexitySimple
	// ComplexityModerate represents multi-step reasoning, context needed.
	// Routes to cloud tier (Haiku+).
	ComplexityModerate
	// ComplexityComplex represents complex analysis, synthesis, code review.
	// Routes to cloud tier (Sonnet+).
	ComplexityComplex
	// ComplexityExpert represents novel problems, architectural decisions.
	// Routes to cloud tier (Opus).
	ComplexityExpert
)

// String returns the human-readable name of the complexity level.
func (c QueryComplexity) String() string {
	switch c {
	case ComplexityTrivial:
		return "Trivial"
	case ComplexitySimple:
		return "Simple"
	case ComplexityModerate:
		return "Moderate"
	case ComplexityComplex:
		return "Complex"
	case ComplexityExpert:
		return "Expert"
	default:
		return fmt.Sprintf("QueryComplexity(%d)", c)
	}
}

// MinTier returns the minimum tier recommended for this complexity level.
// Routes queries based on complexity to optimize cost vs quality:
//   - Trivial: Cache (semantic cache for instant responses)
//   - Simple: Local (free, fast, good for basic tasks)
//   - Moderate/Complex/Expert: Cloud (OpenRouter auto-router picks best model)
func (c QueryComplexity) MinTier() Tier {
	switch c {
	case ComplexityTrivial:
		return TierCache
	case ComplexitySimple:
		return TierLocal
	default:
		return TierCloud
	}
}

// ============================================================================
// QUERY TYPE
// ============================================================================

// QueryType represents the type/category of a query.
// Helps with model selection and routing decisions.
type QueryType int

const (
	// QueryTypeUnknown represents an unknown or unclassified query type.
	// Used when query type cannot be determined or for error conditions.
	QueryTypeUnknown QueryType = iota
	// QueryTypeLookup represents simple fact lookup ("what is X").
	QueryTypeLookup
	// QueryTypeExplanation represents how/why explanations.
	QueryTypeExplanation
	// QueryTypeCodeGeneration represents writing new code.
	QueryTypeCodeGeneration
	// QueryTypeRefactoring represents modifying existing code.
	QueryTypeRefactoring
	// QueryTypeArchitecture represents design decisions and architecture.
	QueryTypeArchitecture
	// QueryTypeDebugging represents finding and fixing bugs.
	QueryTypeDebugging
	// QueryTypeReview represents code review tasks.
	QueryTypeReview
	// QueryTypePlanning represents project planning tasks.
	QueryTypePlanning
	// QueryTypeGeneral represents general conversation.
	QueryTypeGeneral
)

// String returns the human-readable name of the query type.
func (q QueryType) String() string {
	switch q {
	case QueryTypeUnknown:
		return "Unknown"
	case QueryTypeLookup:
		return "Lookup"
	case QueryTypeExplanation:
		return "Explanation"
	case QueryTypeCodeGeneration:
		return "CodeGeneration"
	case QueryTypeRefactoring:
		return "Refactoring"
	case QueryTypeArchitecture:
		return "Architecture"
	case QueryTypeDebugging:
		return "Debugging"
	case QueryTypeReview:
		return "Review"
	case QueryTypePlanning:
		return "Planning"
	case QueryTypeGeneral:
		return "General"
	default:
		return fmt.Sprintf("QueryType(%d)", q)
	}
}

// ModelHint returns a hint for local model selection.
// Returns:
//   - "fast" for quick responses, simple tasks
//   - "code" for code-focused model
//   - "reasoning" for complex reasoning model
func (q QueryType) ModelHint() string {
	switch q {
	case QueryTypeLookup, QueryTypeGeneral, QueryTypeExplanation:
		return "fast"
	case QueryTypeCodeGeneration, QueryTypeRefactoring, QueryTypeDebugging:
		return "code"
	case QueryTypeArchitecture, QueryTypePlanning, QueryTypeReview:
		return "reasoning"
	default:
		return "fast"
	}
}

// ============================================================================
// ROUTER OPTIONS
// ============================================================================

// RouterOptions contains configuration for routing decisions.
type RouterOptions struct {
	// Mode is the default routing mode: "auto", "local", "cloud", "hybrid"
	// "auto" (default): Let OpenRouter decide the optimal model/route
	// "local": Force local Ollama only
	// "cloud": Force cloud only
	// "hybrid": Alias for "auto" (deprecated)
	Mode string
	// MaxTier caps the maximum tier that can be selected
	MaxTier string
	// Paranoid blocks all cloud requests when true
	Paranoid bool
	// HasCloudKey indicates if an OpenRouter API key is configured
	HasCloudKey bool

	// Auto mode configuration
	// AutoPreferLocal hints to prefer local models when possible (cost savings)
	AutoPreferLocal bool
	// AutoMaxCost is the maximum cost per query in cents for auto mode (0 = unlimited)
	AutoMaxCost float64
	// AutoFallback specifies what to do if OpenRouter is unavailable: "local" or "error"
	AutoFallback string
}

// GetMaxTier returns the Tier corresponding to the MaxTier string.
// Returns nil if no cap should be applied.
func (o *RouterOptions) GetMaxTier() *Tier {
	if o == nil || o.MaxTier == "" {
		return nil
	}

	var tier Tier
	switch o.MaxTier {
	case "cache":
		tier = TierCache
	case "local":
		tier = TierLocal
	case "cloud":
		tier = TierCloud
	case "haiku":
		tier = TierHaiku
	case "sonnet":
		tier = TierSonnet
	case "opus":
		tier = TierOpus
	case "gpt-4o":
		tier = TierGpt4o
	default:
		return nil
	}
	return &tier
}

// ShouldUseLocal returns true if routing should prefer local-only.
func (o *RouterOptions) ShouldUseLocal() bool {
	if o == nil {
		return false
	}
	return o.Paranoid || o.Mode == "local" || (o.Mode == "cloud" && !o.HasCloudKey)
}

// IsAutoMode returns true if the routing mode is "auto" or "hybrid" (alias for auto).
func (o *RouterOptions) IsAutoMode() bool {
	if o == nil {
		return false
	}
	return o.Mode == "auto" || o.Mode == "hybrid"
}

// ShouldUseOpenRouterAuto returns true if OpenRouter auto-routing should be used.
// This is true when mode is "auto" or "hybrid", cloud key is available, and not in paranoid mode.
func (o *RouterOptions) ShouldUseOpenRouterAuto() bool {
	if o == nil {
		return false
	}
	return o.IsAutoMode() && o.HasCloudKey && !o.Paranoid
}

// ============================================================================
// ROUTING DECISION
// ============================================================================

// RoutingDecision contains the full routing analysis for a query.
type RoutingDecision struct {
	// Tier is the selected model tier for this query.
	Tier Tier `json:"tier"`
	// Complexity is the assessed complexity level.
	Complexity QueryComplexity `json:"complexity"`
	// QueryType is the classified type of the query.
	QueryType QueryType `json:"query_type"`
	// EstimatedCostCents is the estimated cost in cents.
	EstimatedCostCents float64 `json:"estimated_cost_cents"`
	// Reason explains why this routing decision was made.
	Reason string `json:"reason"`
	// SelectedModel is the model OpenRouter selected (set after response for auto mode).
	SelectedModel string `json:"selected_model,omitempty"`
	// IsAutoRouted indicates if OpenRouter auto-routing was used.
	IsAutoRouted bool `json:"is_auto_routed,omitempty"`
}

// String returns a human-readable summary of the routing decision.
func (r RoutingDecision) String() string {
	return fmt.Sprintf("%s (complexity=%s, type=%s, est_cost=%.4f cents): %s",
		r.Tier, r.Complexity, r.QueryType, r.EstimatedCostCents, r.Reason)
}

// ============================================================================
// QUERY RESULT
// ============================================================================

// QueryResult contains the result of a routed query execution.
type QueryResult struct {
	// Response is the generated response text.
	Response string `json:"response"`
	// TierUsed is which tier was actually used.
	TierUsed Tier `json:"tier_used"`
	// InputTokens is the number of input tokens consumed.
	InputTokens uint32 `json:"input_tokens"`
	// OutputTokens is the number of output tokens generated.
	OutputTokens uint32 `json:"output_tokens"`
	// LatencyMs is the total latency in milliseconds.
	LatencyMs uint64 `json:"latency_ms"`
	// CacheHit indicates whether this was a cache hit.
	CacheHit bool `json:"cache_hit"`
	// CostCents is the actual cost in cents.
	CostCents float64 `json:"cost_cents"`
}

// TotalTokens returns the total tokens used (input + output).
func (r QueryResult) TotalTokens() uint32 {
	return r.InputTokens + r.OutputTokens
}

// String returns a human-readable summary of the query result.
func (r QueryResult) String() string {
	cacheStr := ""
	if r.CacheHit {
		cacheStr = " [CACHE HIT]"
	}
	return fmt.Sprintf("%s%s: %d tokens (%d in, %d out), %dms, %.4f cents",
		r.TierUsed, cacheStr, r.TotalTokens(), r.InputTokens, r.OutputTokens, r.LatencyMs, r.CostCents)
}

// NewQueryResult creates a new query result with cost calculated automatically.
func NewQueryResult(response string, tier Tier, inputTokens, outputTokens uint32, latencyMs uint64) QueryResult {
	return QueryResult{
		Response:     response,
		TierUsed:     tier,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		LatencyMs:    latencyMs,
		CacheHit:     false,
		CostCents:    tier.CalculateCostCents(inputTokens, outputTokens),
	}
}

// NewCacheHitResult creates a query result for a cache hit.
func NewCacheHitResult(response string, latencyMs uint64) QueryResult {
	return QueryResult{
		Response:     response,
		TierUsed:     TierCache,
		InputTokens:  0,
		OutputTokens: 0,
		LatencyMs:    latencyMs,
		CacheHit:     true,
		CostCents:    0.0,
	}
}
