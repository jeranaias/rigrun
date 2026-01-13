//! Query Router - Intelligent routing for RAG queries.
//!
//! Routes queries to the appropriate tier based on complexity:
//! Cache -> Local LLM -> Cloud (Haiku/Sonnet/Opus)
//!
//! Extracted and simplified from OVERWATCH tier system.
//! Focus: Cost optimization while maintaining response quality.

use serde::{Deserialize, Serialize};
use crate::types::Tier;

// ============================================================================
// TIER EXTENSIONS FOR ROUTING
// ============================================================================

impl Tier {
    /// Human-readable tier name.
    pub fn name(&self) -> &'static str {
        match self {
            Self::Cache => "Cache",
            Self::Local => "Local",
            Self::Cloud => "Cloud",
            Self::Haiku => "Haiku",
            Self::Sonnet => "Sonnet",
            Self::Opus => "Opus",
            Self::Gpt4o => "GPT-4o",
        }
    }

    /// Cost per 1K INPUT tokens (in cents).
    ///
    /// Pricing as of 2024:
    /// - Cloud (OpenRouter auto): $0.3/M average = 0.03 cents/1K (estimated)
    /// - Haiku: $0.25/M input = 0.025 cents/1K
    /// - Sonnet: $3/M input = 0.3 cents/1K
    /// - Opus: $15/M input = 1.5 cents/1K
    /// - GPT-4o: $2.5/M input = 0.25 cents/1K
    pub fn input_cost_per_1k(&self) -> f32 {
        match self {
            Self::Cache => 0.0,
            Self::Local => 0.0,
            Self::Cloud => 0.03, // OpenRouter auto picks optimal model
            Self::Haiku => 0.025,
            Self::Sonnet => 0.3,
            Self::Opus => 1.5,
            Self::Gpt4o => 0.25,
        }
    }

    /// Cost per 1K OUTPUT tokens (in cents).
    ///
    /// Pricing as of 2024:
    /// - Cloud (OpenRouter auto): $1.5/M average = 0.15 cents/1K (estimated)
    /// - Haiku: $1.25/M output = 0.125 cents/1K
    /// - Sonnet: $15/M output = 1.5 cents/1K
    /// - Opus: $75/M output = 7.5 cents/1K
    /// - GPT-4o: $10/M output = 1.0 cents/1K
    pub fn output_cost_per_1k(&self) -> f32 {
        match self {
            Self::Cache => 0.0,
            Self::Local => 0.0,
            Self::Cloud => 0.15, // OpenRouter auto picks optimal model
            Self::Haiku => 0.125,
            Self::Sonnet => 1.5,
            Self::Opus => 7.5,
            Self::Gpt4o => 1.0,
        }
    }

    /// Calculate total cost for a request.
    ///
    /// Returns cost in cents.
    pub fn calculate_cost_cents(&self, input_tokens: u32, output_tokens: u32) -> f32 {
        let input_cost = (input_tokens as f32 / 1000.0) * self.input_cost_per_1k();
        let output_cost = (output_tokens as f32 / 1000.0) * self.output_cost_per_1k();
        input_cost + output_cost
    }

    /// Typical latency in milliseconds (for estimation).
    pub fn typical_latency_ms(&self) -> u32 {
        match self {
            Self::Cache => 1,
            Self::Local => 500,
            Self::Cloud => 1000, // OpenRouter auto-selects based on complexity
            Self::Haiku => 800,
            Self::Sonnet => 1500,
            Self::Opus => 3000,
            Self::Gpt4o => 1200,
        }
    }

    /// Get next tier up (for escalation on failure).
    pub fn escalate(&self) -> Option<Tier> {
        match self {
            Self::Cache => Some(Self::Local),
            Self::Local => Some(Self::Cloud),
            Self::Cloud => None, // OpenRouter auto already picks best model
            Self::Haiku => Some(Self::Sonnet),
            Self::Sonnet => Some(Self::Opus),
            Self::Opus => None,
            Self::Gpt4o => None,
        }
    }
}


// ============================================================================
// QUERY COMPLEXITY
// ============================================================================

/// Query complexity levels - determines minimum tier required.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum QueryComplexity {
    /// Simple lookup, pattern match - cache or local
    Trivial,
    /// Basic question, single-step reasoning - local
    Simple,
    /// Multi-step reasoning, context needed - Haiku
    Moderate,
    /// Complex analysis, synthesis - Sonnet
    Complex,
    /// Novel problems, architectural decisions - Opus
    Expert,
}

impl QueryComplexity {
    /// Minimum tier recommended for this complexity level.
    /// Routes queries based on complexity to optimize cost vs quality:
    /// - Trivial: Cache (semantic cache for instant responses)
    /// - Simple: Local (free, fast, good for basic tasks)
    /// - Moderate/Complex/Expert: Cloud (OpenRouter auto-router picks best model)
    pub fn min_tier(&self) -> Tier {
        match self {
            Self::Trivial => Tier::Cache,
            Self::Simple => Tier::Local,
            Self::Moderate | Self::Complex | Self::Expert => Tier::Cloud,
        }
    }
}

// ============================================================================
// QUERY TYPE CLASSIFICATION
// ============================================================================

/// Query type - helps with model selection and routing.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum QueryType {
    /// Simple fact lookup ("what is X")
    Lookup,
    /// How/why explanations
    Explanation,
    /// Write new code
    CodeGeneration,
    /// Modify existing code
    Refactoring,
    /// Design decisions, architecture
    Architecture,
    /// Find and fix bugs
    Debugging,
    /// Code review
    Review,
    /// Project planning
    Planning,
    /// General conversation
    General,
}

impl QueryType {
    /// Classify a query based on text heuristics.
    ///
    /// This uses keyword matching to quickly categorize queries.
    /// More sophisticated classification could use embeddings.
    pub fn classify(query: &str) -> Self {
        let q = query.to_lowercase();

        // Simple lookups
        if q.contains("what is") || q.contains("syntax") ||
           q.starts_with("list ") || q.contains("first ") {
            return QueryType::Lookup;
        }

        // Explanations
        if q.contains("explain") || q.contains("how does") || q.contains("why ") {
            return QueryType::Explanation;
        }

        // Code generation
        if q.contains("write") || q.contains("create") ||
           q.contains("implement") || q.contains("generate") {
            return QueryType::CodeGeneration;
        }

        // Refactoring
        if q.contains("refactor") || q.contains("improve") || q.contains("optimize") {
            return QueryType::Refactoring;
        }

        // Architecture
        if q.contains("architect") || q.contains("design") ||
           q.contains("should i") || q.contains("trade-off") {
            return QueryType::Architecture;
        }

        // Debugging
        if q.contains("bug") || q.contains("fix") ||
           q.contains("debug") || q.contains("error") {
            return QueryType::Debugging;
        }

        // Review
        if q.contains("review") || q.contains("check") {
            return QueryType::Review;
        }

        // Planning
        if q.contains("plan") || q.contains("roadmap") {
            return QueryType::Planning;
        }

        QueryType::General
    }

    /// Suggest a model capability for this query type.
    ///
    /// Returns a hint for local model selection:
    /// - "fast" = quick responses, simple tasks
    /// - "code" = code-focused model
    /// - "reasoning" = complex reasoning model
    pub fn model_hint(&self) -> &'static str {
        match self {
            QueryType::Lookup | QueryType::General => "fast",
            QueryType::Explanation => "fast",
            QueryType::CodeGeneration | QueryType::Refactoring | QueryType::Debugging => "code",
            QueryType::Architecture | QueryType::Planning | QueryType::Review => "reasoning",
        }
    }
}

// ============================================================================
// QUERY CLASSIFICATION
// ============================================================================

/// Classify query complexity using heuristics.
///
/// Analyzes the query text to determine how complex the response needs to be.
/// This drives tier selection for cost optimization.
pub fn classify_query(query: &str) -> QueryComplexity {
    let q = query.to_lowercase();
    let word_count = query.split_whitespace().count();

    // Expert-level indicators (architectural decisions, trade-offs)
    if q.contains("architect") || q.contains("design pattern") ||
       q.contains("trade-off") || q.contains("best approach") ||
       q.contains("should i") || q.contains("pros and cons") {
        return QueryComplexity::Expert;
    }

    // Complex indicators (analysis, implementation, long queries)
    if q.contains("explain") || q.contains("compare") ||
       q.contains("analyze") || q.contains("implement") ||
       q.contains("refactor") || word_count > 50 {
        return QueryComplexity::Complex;
    }

    // Moderate indicators (how/why questions, debugging)
    if q.contains("how") || q.contains("why") ||
       q.contains("debug") || q.contains("fix") ||
       word_count > 20 {
        return QueryComplexity::Moderate;
    }

    // Simple indicators (basic lookups, short queries)
    if q.contains("what is") || q.contains("where is") ||
       q.contains("find") || q.contains("list") ||
       word_count > 5 {
        return QueryComplexity::Simple;
    }

    QueryComplexity::Trivial
}

// ============================================================================
// QUERY ROUTING
// ============================================================================

/// Route a query to the appropriate tier.
///
/// Takes the query and optional configuration to decide which tier to use.
/// Respects max_tier to cap costs.
pub fn route_query(query: &str, max_tier: Option<Tier>) -> Tier {
    let complexity = classify_query(query);
    let recommended = complexity.min_tier();

    // Cap at max tier if specified
    match max_tier {
        Some(cap) if recommended > cap => cap,
        _ => recommended,
    }
}

/// Routing decision with full details.
#[derive(Debug, Clone)]
pub struct RoutingDecision {
    pub tier: Tier,
    pub complexity: QueryComplexity,
    pub query_type: QueryType,
    pub estimated_cost_cents: f32,
    pub reason: String,
}

/// Make a detailed routing decision for a query.
///
/// Returns full analysis including tier, complexity, type, and reasoning.
pub fn route_query_detailed(query: &str, max_tier: Option<Tier>) -> RoutingDecision {
    let complexity = classify_query(query);
    let query_type = QueryType::classify(query);
    let recommended = complexity.min_tier();

    let tier = match max_tier {
        Some(cap) if recommended > cap => cap,
        _ => recommended,
    };

    // Estimate cost (assume ~500 input tokens, ~1000 output tokens for typical query)
    let estimated_cost = tier.calculate_cost(500, 1000);

    let reason = format!(
        "Query classified as {:?} complexity ({:?} type) -> {} tier",
        complexity, query_type, tier.name()
    );

    RoutingDecision {
        tier,
        complexity,
        query_type,
        estimated_cost_cents: estimated_cost,
        reason,
    }
}

// ============================================================================
// RESULT TYPES
// ============================================================================

/// Result of a routed query execution.
#[derive(Debug, Clone)]
pub struct QueryResult {
    /// The response text
    pub response: String,
    /// Which tier was used
    pub tier_used: Tier,
    /// Input tokens consumed
    pub input_tokens: u32,
    /// Output tokens generated
    pub output_tokens: u32,
    /// Total latency in milliseconds
    pub latency_ms: u64,
    /// Whether this was a cache hit
    pub cache_hit: bool,
    /// Cost in cents
    pub cost_cents: f32,
}

impl QueryResult {
    /// Total tokens used (input + output).
    pub fn total_tokens(&self) -> u32 {
        self.input_tokens + self.output_tokens
    }

    /// Create a cache hit result.
    pub fn cache_hit(response: String, latency_ms: u64) -> Self {
        Self {
            response,
            tier_used: Tier::Cache,
            input_tokens: 0,
            output_tokens: 0,
            latency_ms,
            cache_hit: true,
            cost_cents: 0.0,
        }
    }

    /// Create a new query result.
    pub fn new(
        response: String,
        tier: Tier,
        input_tokens: u32,
        output_tokens: u32,
        latency_ms: u64,
    ) -> Self {
        let cost = tier.calculate_cost_cents(input_tokens, output_tokens);
        Self {
            response,
            tier_used: tier,
            input_tokens,
            output_tokens,
            latency_ms,
            cache_hit: false,
            cost_cents: cost,
        }
    }
}

// ============================================================================
// TESTS
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_tier_costs() {
        // Cache and Local should be free
        assert_eq!(Tier::Cache.calculate_cost_cents(1000, 1000), 0.0);
        assert_eq!(Tier::Local.calculate_cost_cents(1000, 1000), 0.0);

        // Cloud tier should have cost
        assert!(Tier::Cloud.calculate_cost_cents(1000, 1000) > 0.0);

        // Explicit model tiers should have costs
        assert!(Tier::Haiku.calculate_cost_cents(1000, 1000) > 0.0);
        assert!(Tier::Sonnet.calculate_cost_cents(1000, 1000) > Tier::Haiku.calculate_cost_cents(1000, 1000));
        assert!(Tier::Opus.calculate_cost_cents(1000, 1000) > Tier::Sonnet.calculate_cost_cents(1000, 1000));
    }

    #[test]
    fn test_tier_escalation() {
        assert_eq!(Tier::Cache.escalate(), Some(Tier::Local));
        assert_eq!(Tier::Local.escalate(), Some(Tier::Cloud));
        assert_eq!(Tier::Cloud.escalate(), None);
        assert_eq!(Tier::Opus.escalate(), None);
    }

    #[test]
    fn test_complexity_classification() {
        // Trivial queries
        assert_eq!(classify_query("hi"), QueryComplexity::Trivial);

        // Simple queries
        assert_eq!(classify_query("what is rust"), QueryComplexity::Simple);

        // Moderate queries
        assert_eq!(classify_query("how do I fix this bug"), QueryComplexity::Moderate);

        // Expert queries
        assert_eq!(
            classify_query("should I use microservices, what are the trade-offs"),
            QueryComplexity::Expert
        );
    }

    #[test]
    fn test_query_type_classification() {
        assert_eq!(QueryType::classify("what is a mutex"), QueryType::Lookup);
        assert_eq!(QueryType::classify("explain async await"), QueryType::Explanation);
        assert_eq!(QueryType::classify("write a function to sort"), QueryType::CodeGeneration);
        assert_eq!(QueryType::classify("fix this bug"), QueryType::Debugging);
    }

    #[test]
    fn test_routing() {
        // Trivial queries route to Cache tier
        assert_eq!(route_query("hello", None), Tier::Cache);
        // Simple queries route to Local
        assert_eq!(route_query("what is rust", None), Tier::Local);
        // Moderate queries route to Cloud (OpenRouter auto)
        assert_eq!(route_query("how do I fix this error", None), Tier::Cloud);
        // Complex queries route to Cloud (OpenRouter auto)
        assert_eq!(route_query("explain how async runtime works", None), Tier::Cloud);
        // Expert queries route to Cloud (OpenRouter auto)
        assert_eq!(route_query("should I use microservices", None), Tier::Cloud);
    }
}
