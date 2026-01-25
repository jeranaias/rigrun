// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// ROUTER: Accurate cost estimation and complexity analysis
package router

import (
	"strings"
)

// ============================================================================
// CLASSIFICATION FUNCTIONS
// ============================================================================

// wordCount returns the number of words in a string.
// Uses strings.Fields which splits on whitespace.
func wordCount(s string) int {
	return len(strings.Fields(s))
}

// ClassifyComplexity analyzes query text to determine complexity level.
// This drives tier selection for cost optimization.
//
// Classification rules (in order of priority):
//  1. Expert: architectural decisions, trade-offs, best approach questions
//  2. Complex: analysis, implementation, code review, or word count > 15
//  3. Moderate: how/why questions, debugging, or word count > 10
//  4. Simple: basic lookups (what is, where is, find, list)
//  5. Trivial: very short queries (< 5 words) with no keywords
//
// NOTE: Thresholds are set LOW to favor cloud routing for better responses.
// Local models are only used for very simple queries (< 10 words).
// Cloud uses OpenRouter auto-router which picks the cheapest model automatically.
func ClassifyComplexity(query string) QueryComplexity {
	q := strings.ToLower(query)
	wc := wordCount(query)

	// Expert-level indicators (architectural decisions, trade-offs)
	if strings.Contains(q, "architect") ||
		strings.Contains(q, "design pattern") ||
		strings.Contains(q, "trade-off") ||
		strings.Contains(q, "best approach") ||
		strings.Contains(q, "should i") ||
		strings.Contains(q, "pros and cons") {
		return ComplexityExpert
	}

	// Complex indicators (analysis, implementation, code review, any substantial query)
	// LOWERED THRESHOLD: 15+ words or any code-related work goes to cloud
	if strings.Contains(q, "explain") ||
		strings.Contains(q, "compare") ||
		strings.Contains(q, "analyze") ||
		strings.Contains(q, "implement") ||
		strings.Contains(q, "refactor") ||
		strings.Contains(q, "review") ||
		strings.Contains(q, "code") ||
		strings.Contains(q, "function") ||
		strings.Contains(q, "bug") ||
		strings.Contains(q, "error") ||
		wc > 15 {
		return ComplexityComplex
	}

	// Moderate indicators (how/why questions, debugging)
	// LOWERED THRESHOLD: 10+ words goes to cloud
	if strings.Contains(q, "how") ||
		strings.Contains(q, "why") ||
		strings.Contains(q, "debug") ||
		strings.Contains(q, "fix") ||
		wc > 10 {
		return ComplexityModerate
	}

	// Simple indicators - ONLY very basic lookups stay local
	// Anything with more than 10 words goes to cloud
	if strings.Contains(q, "what is") ||
		strings.Contains(q, "where is") ||
		strings.Contains(q, "find") ||
		strings.Contains(q, "list") {
		return ComplexitySimple
	}

	// Default: anything with 5+ words goes to moderate (cloud)
	if wc >= 5 {
		return ComplexityModerate
	}

	return ComplexityTrivial
}

// ClassifyType categorizes a query based on text heuristics.
// This uses keyword matching to quickly categorize queries.
// More sophisticated classification could use embeddings.
//
// Classification rules (in order of priority):
//  1. Lookup: "what is", "syntax", starts with "list " or "first "
//  2. Explanation: "explain", "how does", "why "
//  3. CodeGeneration: "write", "create", "implement", "generate"
//  4. Refactoring: "refactor", "improve", "optimize"
//  5. Architecture: "architect", "design", "should i", "trade-off"
//  6. Debugging: "bug", "fix", "debug", "error"
//  7. Review: "review", "check"
//  8. Planning: "plan", "roadmap"
//  9. General: default fallback
func ClassifyType(query string) QueryType {
	q := strings.ToLower(query)

	// Simple lookups
	if strings.Contains(q, "what is") ||
		strings.Contains(q, "syntax") ||
		strings.HasPrefix(q, "list ") ||
		strings.Contains(q, "first ") {
		return QueryTypeLookup
	}

	// Explanations
	if strings.Contains(q, "explain") ||
		strings.Contains(q, "how does") ||
		strings.Contains(q, "why ") {
		return QueryTypeExplanation
	}

	// Code generation
	if strings.Contains(q, "write") ||
		strings.Contains(q, "create") ||
		strings.Contains(q, "implement") ||
		strings.Contains(q, "generate") {
		return QueryTypeCodeGeneration
	}

	// Refactoring
	if strings.Contains(q, "refactor") ||
		strings.Contains(q, "improve") ||
		strings.Contains(q, "optimize") {
		return QueryTypeRefactoring
	}

	// Architecture
	if strings.Contains(q, "architect") ||
		strings.Contains(q, "design") ||
		strings.Contains(q, "should i") ||
		strings.Contains(q, "trade-off") {
		return QueryTypeArchitecture
	}

	// Debugging
	if strings.Contains(q, "bug") ||
		strings.Contains(q, "fix") ||
		strings.Contains(q, "debug") ||
		strings.Contains(q, "error") {
		return QueryTypeDebugging
	}

	// Review
	if strings.Contains(q, "review") ||
		strings.Contains(q, "check") {
		return QueryTypeReview
	}

	// Planning
	if strings.Contains(q, "plan") ||
		strings.Contains(q, "roadmap") {
		return QueryTypePlanning
	}

	return QueryTypeGeneral
}

// ============================================================================
// QUERY ANALYSIS
// ============================================================================

// Complexity represents the estimated complexity level for query analysis.
type Complexity int

const (
	// ComplexityLow represents simple queries that can be handled quickly.
	ComplexityLow Complexity = iota
	// ComplexityMedium represents moderate queries requiring some processing.
	ComplexityMedium
	// ComplexityHigh represents complex queries requiring significant reasoning.
	ComplexityHigh
)

// String returns the human-readable name of the complexity level.
func (c Complexity) String() string {
	switch c {
	case ComplexityLow:
		return "Low"
	case ComplexityMedium:
		return "Medium"
	case ComplexityHigh:
		return "High"
	default:
		return "Unknown"
	}
}

// QueryAnalysis contains detailed analysis of a query for routing decisions.
type QueryAnalysis struct {
	// TokenCount is the estimated number of tokens in the query.
	TokenCount int
	// HasCode indicates if the query contains code snippets.
	HasCode bool
	// HasMath indicates if the query contains mathematical expressions.
	HasMath bool
	// RequiresReasoning indicates if the query requires multi-step reasoning.
	RequiresReasoning bool
	// EstimatedComplexity is the overall complexity assessment.
	EstimatedComplexity Complexity
}

// AnalyzeQuery performs detailed analysis of a query for routing decisions.
// Examines the query for code, math, reasoning requirements, and overall complexity.
func AnalyzeQuery(query string) QueryAnalysis {
	analysis := QueryAnalysis{
		TokenCount: EstimateTokens(query),
	}

	// Detect code indicators
	codeIndicators := []string{"```", "function", "def ", "class ", "import ", "func ", "var ", "const ", "let "}
	for _, indicator := range codeIndicators {
		if strings.Contains(query, indicator) {
			analysis.HasCode = true
			break
		}
	}

	// Detect math indicators
	mathIndicators := []string{"=", "+", "-", "*", "/", "^", "sqrt", "sum", "integral", "derivative", "equation"}
	queryLower := strings.ToLower(query)
	for _, indicator := range mathIndicators {
		if strings.Contains(queryLower, indicator) {
			analysis.HasMath = true
			break
		}
	}

	// Detect reasoning requirements
	reasoningWords := []string{"why", "how", "explain", "analyze", "compare", "evaluate", "consider", "reason"}
	for _, word := range reasoningWords {
		if strings.Contains(queryLower, word) {
			analysis.RequiresReasoning = true
			break
		}
	}

	// Set complexity based on analysis
	if analysis.TokenCount > 500 || analysis.RequiresReasoning {
		analysis.EstimatedComplexity = ComplexityHigh
	} else if analysis.TokenCount > 100 || analysis.HasCode {
		analysis.EstimatedComplexity = ComplexityMedium
	} else {
		analysis.EstimatedComplexity = ComplexityLow
	}

	return analysis
}

// RecommendedTier returns the recommended tier based on query analysis.
func (a QueryAnalysis) RecommendedTier() Tier {
	switch a.EstimatedComplexity {
	case ComplexityHigh:
		return TierCloud // Let OpenRouter auto-route for complex queries
	case ComplexityMedium:
		if a.HasCode {
			return TierCloud // Code tasks benefit from better models
		}
		return TierLocal
	default:
		return TierLocal
	}
}
