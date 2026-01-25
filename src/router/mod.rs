// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

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

    /// Internal escalation logic - returns next tier without classification checks.
    ///
    /// SECURITY WARNING: This is PRIVATE. Do NOT make this public.
    /// Use `try_escalate()` which enforces classification-based routing restrictions.
    fn escalate_unchecked(&self) -> Option<Tier> {
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

    /// Check if a tier is a cloud tier (Haiku, Sonnet, Opus, Cloud, GPT-4o).
    ///
    /// Cloud tiers send data off-premise and are BLOCKED for CUI+ classifications.
    pub fn is_cloud_tier(&self) -> bool {
        matches!(self, Self::Cloud | Self::Haiku | Self::Sonnet | Self::Opus | Self::Gpt4o)
    }

    /// Attempt to escalate to the next tier with classification enforcement.
    ///
    /// # Security
    ///
    /// CRITICAL: This function enforces classification-based routing restrictions.
    /// - If classification >= CUI, escalation to ANY cloud tier is BLOCKED
    /// - Blocked escalation attempts are audit logged
    /// - Returns `EscalationResult` indicating success, blocked, or no escalation available
    ///
    /// # Parameters
    ///
    /// - `classification`: The classification level of the query
    /// - `query_preview`: Optional query preview for audit logging (first 50 chars)
    ///
    /// # Returns
    ///
    /// - `EscalationResult::Success(Tier)` - Escalation allowed, use this tier
    /// - `EscalationResult::Blocked` - Escalation blocked due to classification
    /// - `EscalationResult::NotAvailable` - No higher tier available
    pub fn try_escalate(
        &self,
        classification: crate::ClassificationLevel,
        query_preview: Option<&str>,
    ) -> EscalationResult {
        use crate::ClassificationLevel;

        // Get the next tier (if any)
        let Some(next_tier) = self.escalate_unchecked() else {
            return EscalationResult::NotAvailable;
        };

        // CRITICAL SECURITY CHECK: Block cloud escalation for CUI+ classifications
        if classification >= ClassificationLevel::Cui && next_tier.is_cloud_tier() {
            // Audit log the blocked escalation attempt
            if let Some(logger) = crate::audit::global_audit_logger().read().ok() {
                let query = query_preview.unwrap_or("[no query provided]");
                let _ = logger.log_blocked(next_tier, query, None, None);
            }

            // Log to stderr for immediate visibility in debug scenarios
            eprintln!(
                "[SECURITY] Escalation to {} BLOCKED: Classification {:?} prohibits cloud routing",
                next_tier.name(),
                classification
            );

            return EscalationResult::Blocked {
                requested_tier: next_tier,
                classification,
            };
        }

        EscalationResult::Success(next_tier)
    }
}

/// Result of an escalation attempt.
///
/// Returned by `Tier::try_escalate()` to indicate whether escalation
/// was successful, blocked by classification, or not available.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum EscalationResult {
    /// Escalation allowed - use this tier
    Success(Tier),
    /// Escalation blocked due to classification restrictions
    Blocked {
        /// The tier that was requested but blocked
        requested_tier: Tier,
        /// The classification that caused the block
        classification: crate::ClassificationLevel,
    },
    /// No higher tier available (already at max)
    NotAvailable,
}

impl EscalationResult {
    /// Returns `true` if escalation was successful.
    pub fn is_success(&self) -> bool {
        matches!(self, Self::Success(_))
    }

    /// Returns `true` if escalation was blocked due to classification.
    pub fn is_blocked(&self) -> bool {
        matches!(self, Self::Blocked { .. })
    }

    /// Returns the new tier if escalation was successful.
    pub fn tier(&self) -> Option<Tier> {
        match self {
            Self::Success(tier) => Some(*tier),
            _ => None,
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
///
/// NOTE: Threshold is set LOW to favor cloud routing for better responses.
/// Local models are only used for very simple queries (< 10 words).
/// Cloud uses OpenRouter auto-router which picks the cheapest model automatically.
pub fn classify_query(query: &str) -> QueryComplexity {
    let q = query.to_lowercase();
    let word_count = query.split_whitespace().count();

    // Expert-level indicators (architectural decisions, trade-offs)
    if q.contains("architect") || q.contains("design pattern") ||
       q.contains("trade-off") || q.contains("best approach") ||
       q.contains("should i") || q.contains("pros and cons") {
        return QueryComplexity::Expert;
    }

    // Complex indicators (analysis, implementation, code review, any substantial query)
    // LOWERED THRESHOLD: 15+ words or any code-related work goes to cloud
    if q.contains("explain") || q.contains("compare") ||
       q.contains("analyze") || q.contains("implement") ||
       q.contains("refactor") || q.contains("review") ||
       q.contains("code") || q.contains("function") ||
       q.contains("bug") || q.contains("error") ||
       word_count > 15 {
        return QueryComplexity::Complex;
    }

    // Moderate indicators (how/why questions, debugging)
    // LOWERED THRESHOLD: 10+ words goes to cloud
    if q.contains("how") || q.contains("why") ||
       q.contains("debug") || q.contains("fix") ||
       word_count > 10 {
        return QueryComplexity::Moderate;
    }

    // Simple indicators - ONLY very basic lookups stay local
    // Anything with more than 10 words goes to cloud
    if q.contains("what is") || q.contains("where is") ||
       q.contains("find") || q.contains("list") {
        return QueryComplexity::Simple;
    }

    // Default: anything with 5+ words goes to moderate (cloud)
    if word_count >= 5 {
        return QueryComplexity::Moderate;
    }

    QueryComplexity::Trivial
}

// ============================================================================
// QUERY ROUTING
// ============================================================================

/// Route a query to the appropriate tier.
///
/// Takes the query, classification level, paranoid mode flag, and optional configuration
/// to decide which tier to use. Enforces classification-based routing restrictions.
///
/// # Security
///
/// CRITICAL: Classification enforcement is the FIRST check before any other routing logic.
/// - If classification >= CUI, FORCE return Tier::Local (NEVER route to cloud)
/// - If paranoid_mode is true, NEVER allow cloud fallback regardless of local failures
/// - These checks take precedence over all other routing decisions
///
/// # Parameters
///
/// - `query`: The query text to route
/// - `classification`: The classification level of the query (UNCLASSIFIED, CUI, etc.)
/// - `paranoid_mode`: If true, block all cloud routing regardless of classification
/// - `max_tier`: Optional tier cap for cost control
pub fn route_query(
    query: &str,
    classification: crate::ClassificationLevel,
    paranoid_mode: bool,
    max_tier: Option<Tier>,
) -> Tier {
    use crate::ClassificationLevel;

    // CRITICAL SECURITY CHECK #1: Classification enforcement
    // CUI and higher classifications MUST stay on-premise
    if classification >= ClassificationLevel::Cui {
        // Audit log the classification-based routing decision
        if let Ok(logger) = crate::audit::global_audit_logger().read() {
            let _ = logger.log_blocked(Tier::Cloud, query, None, None);
        }
        return Tier::Local;
    }

    // CRITICAL SECURITY CHECK #2: Paranoid mode enforcement
    // When paranoid mode is enabled, NEVER allow cloud routing
    if paranoid_mode {
        // Audit log the paranoid mode block
        if let Ok(logger) = crate::audit::global_audit_logger().read() {
            let _ = logger.log_blocked(Tier::Cloud, query, None, None);
        }
        return Tier::Local;
    }

    // Normal routing logic (only reached if classification allows cloud)
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
///
/// # Security
///
/// CRITICAL: Classification enforcement is the FIRST check before any other routing logic.
/// - If classification >= CUI, FORCE return Tier::Local (NEVER route to cloud)
/// - If paranoid_mode is true, NEVER allow cloud fallback regardless of local failures
pub fn route_query_detailed(
    query: &str,
    classification: crate::ClassificationLevel,
    paranoid_mode: bool,
    max_tier: Option<Tier>,
) -> RoutingDecision {
    use crate::ClassificationLevel;

    let complexity = classify_query(query);
    let query_type = QueryType::classify(query);
    let recommended = complexity.min_tier();

    // CRITICAL SECURITY CHECK #1: Classification enforcement
    // CUI and higher classifications MUST stay on-premise
    if classification >= ClassificationLevel::Cui {
        // Audit log the classification-based routing decision
        if let Ok(logger) = crate::audit::global_audit_logger().read() {
            let _ = logger.log_blocked(Tier::Cloud, query, None, None);
        }

        let reason = format!(
            "Query classified as {:?} complexity ({:?} type) -> Local tier (FORCED: {} classification blocks cloud)",
            complexity, query_type, classification
        );

        return RoutingDecision {
            tier: Tier::Local,
            complexity,
            query_type,
            estimated_cost_cents: 0.0, // Local is free
            reason,
        };
    }

    // CRITICAL SECURITY CHECK #2: Paranoid mode enforcement
    // When paranoid mode is enabled, NEVER allow cloud routing
    if paranoid_mode {
        // Audit log the paranoid mode block
        if let Ok(logger) = crate::audit::global_audit_logger().read() {
            let _ = logger.log_blocked(Tier::Cloud, query, None, None);
        }

        let reason = format!(
            "Query classified as {:?} complexity ({:?} type) -> Local tier (FORCED: paranoid mode blocks cloud)",
            complexity, query_type
        );

        return RoutingDecision {
            tier: Tier::Local,
            complexity,
            query_type,
            estimated_cost_cents: 0.0, // Local is free
            reason,
        };
    }

    // Normal routing logic (only reached if classification allows cloud)
    let tier = match max_tier {
        Some(cap) if recommended > cap => cap,
        _ => recommended,
    };

    // Estimate cost (assume ~500 input tokens, ~1000 output tokens for typical query)
    let estimated_cost = tier.calculate_cost_cents(500, 1000);

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
    fn test_tier_escalation_unclassified() {
        use crate::ClassificationLevel;

        // UNCLASSIFIED queries can escalate freely through all tiers
        let unclass = ClassificationLevel::Unclassified;

        // Cache -> Local (allowed)
        assert_eq!(
            Tier::Cache.try_escalate(unclass, None),
            EscalationResult::Success(Tier::Local)
        );

        // Local -> Cloud (allowed for UNCLASSIFIED)
        assert_eq!(
            Tier::Local.try_escalate(unclass, None),
            EscalationResult::Success(Tier::Cloud)
        );

        // Cloud has no escalation path
        assert_eq!(
            Tier::Cloud.try_escalate(unclass, None),
            EscalationResult::NotAvailable
        );

        // Opus has no escalation path
        assert_eq!(
            Tier::Opus.try_escalate(unclass, None),
            EscalationResult::NotAvailable
        );

        // Haiku -> Sonnet (allowed for UNCLASSIFIED)
        assert_eq!(
            Tier::Haiku.try_escalate(unclass, None),
            EscalationResult::Success(Tier::Sonnet)
        );

        // Sonnet -> Opus (allowed for UNCLASSIFIED)
        assert_eq!(
            Tier::Sonnet.try_escalate(unclass, None),
            EscalationResult::Success(Tier::Opus)
        );
    }

    #[test]
    fn test_tier_escalation_cui_blocked() {
        use crate::ClassificationLevel;

        // CUI classifications MUST block escalation to ANY cloud tier
        let cui = ClassificationLevel::Cui;

        // Cache -> Local (allowed even for CUI - Local is on-premise)
        assert_eq!(
            Tier::Cache.try_escalate(cui, None),
            EscalationResult::Success(Tier::Local)
        );

        // Local -> Cloud (BLOCKED for CUI)
        let result = Tier::Local.try_escalate(cui, Some("test CUI query"));
        assert!(result.is_blocked());
        assert_eq!(
            result,
            EscalationResult::Blocked {
                requested_tier: Tier::Cloud,
                classification: cui,
            }
        );

        // Haiku -> Sonnet (BLOCKED for CUI - both are cloud tiers, but if somehow
        // we ended up on Haiku with CUI, escalation must still be blocked)
        let result = Tier::Haiku.try_escalate(cui, Some("test CUI query"));
        assert!(result.is_blocked());

        // Sonnet -> Opus (BLOCKED for CUI)
        let result = Tier::Sonnet.try_escalate(cui, Some("test CUI query"));
        assert!(result.is_blocked());
    }

    #[test]
    fn test_tier_escalation_cui_specified_blocked() {
        use crate::ClassificationLevel;

        // CUI//SP (specified) is higher than CUI, must also block cloud
        let cui_sp = ClassificationLevel::CuiSpecified;

        // Local -> Cloud (BLOCKED for CUI//SP)
        let result = Tier::Local.try_escalate(cui_sp, Some("test CUI//SP query"));
        assert!(result.is_blocked());
        assert_eq!(
            result,
            EscalationResult::Blocked {
                requested_tier: Tier::Cloud,
                classification: cui_sp,
            }
        );
    }

    #[test]
    fn test_is_cloud_tier() {
        // Cloud tiers - these send data off-premise
        assert!(Tier::Cloud.is_cloud_tier());
        assert!(Tier::Haiku.is_cloud_tier());
        assert!(Tier::Sonnet.is_cloud_tier());
        assert!(Tier::Opus.is_cloud_tier());
        assert!(Tier::Gpt4o.is_cloud_tier());

        // Non-cloud tiers - these are on-premise
        assert!(!Tier::Cache.is_cloud_tier());
        assert!(!Tier::Local.is_cloud_tier());
    }

    #[test]
    fn test_escalation_result_helpers() {
        use crate::ClassificationLevel;

        let success = EscalationResult::Success(Tier::Cloud);
        assert!(success.is_success());
        assert!(!success.is_blocked());
        assert_eq!(success.tier(), Some(Tier::Cloud));

        let blocked = EscalationResult::Blocked {
            requested_tier: Tier::Cloud,
            classification: ClassificationLevel::Cui,
        };
        assert!(!blocked.is_success());
        assert!(blocked.is_blocked());
        assert_eq!(blocked.tier(), None);

        let not_available = EscalationResult::NotAvailable;
        assert!(!not_available.is_success());
        assert!(!not_available.is_blocked());
        assert_eq!(not_available.tier(), None);
    }

    #[test]
    fn test_complexity_classification() {
        // Trivial queries (< 5 words, no keywords)
        assert_eq!(classify_query("hi"), QueryComplexity::Trivial);
        assert_eq!(classify_query("hello world"), QueryComplexity::Trivial);

        // Simple queries (basic lookups only)
        assert_eq!(classify_query("what is rust"), QueryComplexity::Simple);

        // Moderate queries (how/why or 5+ words)
        assert_eq!(classify_query("how do I fix this bug"), QueryComplexity::Complex); // "bug" keyword
        assert_eq!(classify_query("tell me about this topic here"), QueryComplexity::Moderate); // 6 words

        // Complex queries (code-related keywords)
        assert_eq!(classify_query("review this code"), QueryComplexity::Complex);
        assert_eq!(classify_query("explain the function"), QueryComplexity::Complex);

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
        use crate::ClassificationLevel;

        // UNCLASSIFIED queries with normal routing (no paranoid mode)
        let unclass = ClassificationLevel::Unclassified;

        // Trivial queries route to Cache tier (< 5 words, no keywords)
        assert_eq!(route_query("hello", unclass, false, None), Tier::Cache);
        assert_eq!(route_query("hi there", unclass, false, None), Tier::Cache);

        // Simple queries route to Local (basic lookups only)
        assert_eq!(route_query("what is rust", unclass, false, None), Tier::Local);

        // Moderate+ queries route to Cloud (OpenRouter auto) - lowered threshold!
        // 5+ words or code-related keywords go to cloud
        assert_eq!(route_query("how do I fix this error", unclass, false, None), Tier::Cloud); // "error" keyword
        assert_eq!(route_query("explain how async runtime works", unclass, false, None), Tier::Cloud);
        assert_eq!(route_query("should I use microservices", unclass, false, None), Tier::Cloud);
        assert_eq!(route_query("review this code please", unclass, false, None), Tier::Cloud); // "review" + "code"
        assert_eq!(route_query("tell me about this topic here now", unclass, false, None), Tier::Cloud); // 7 words
    }

    #[test]
    fn test_classification_enforcement() {
        use crate::ClassificationLevel;

        // CUI classification FORCES local routing, regardless of complexity
        let cui = ClassificationLevel::Cui;
        assert_eq!(route_query("simple query", cui, false, None), Tier::Local);
        assert_eq!(route_query("complex expert level architectural decision", cui, false, None), Tier::Local);
        assert_eq!(route_query("review this code please", cui, false, None), Tier::Local);

        // Even if max_tier is Cloud, CUI forces Local
        assert_eq!(route_query("any query", cui, false, Some(Tier::Cloud)), Tier::Local);
    }

    #[test]
    fn test_paranoid_mode_enforcement() {
        use crate::ClassificationLevel;

        // Paranoid mode FORCES local routing for UNCLASSIFIED queries
        let unclass = ClassificationLevel::Unclassified;
        assert_eq!(route_query("simple query", unclass, true, None), Tier::Local);
        assert_eq!(route_query("complex query with many words", unclass, true, None), Tier::Local);
        assert_eq!(route_query("expert architectural decision", unclass, true, None), Tier::Local);

        // Even if max_tier is Cloud, paranoid mode forces Local
        assert_eq!(route_query("any query", unclass, true, Some(Tier::Cloud)), Tier::Local);
    }

    #[test]
    fn test_classification_takes_precedence() {
        use crate::ClassificationLevel;

        // CUI + paranoid mode (both enforcing local)
        let cui = ClassificationLevel::Cui;
        assert_eq!(route_query("any query", cui, true, None), Tier::Local);

        // CUI takes precedence even with Cloud max_tier
        assert_eq!(route_query("any query", cui, false, Some(Tier::Cloud)), Tier::Local);
    }
}
