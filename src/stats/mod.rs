//! Cost Tracking and Statistics Module for Rigrun
//!
//! Tracks what users SAVE by running locally instead of paying for cloud APIs.
//! This is the dopamine hit that keeps users coming back.
//!
//! Key insight: Local inference is FREE. Every query run locally is money saved.

use anyhow::Result;
use chrono::{DateTime, Duration, Utc};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs;
use std::path::PathBuf;
use std::sync::{Arc, RwLock};

// ============================================================================
// COST CONSTANTS (per 1K tokens, in dollars)
// ============================================================================

/// Cost constants for different model tiers (per 1K tokens, in USD)
pub mod costs {
    // OpenRouter Auto - intelligent routing (estimated average)
    pub const CLOUD_INPUT_PER_1K: f64 = 0.0003;
    pub const CLOUD_OUTPUT_PER_1K: f64 = 0.0015;

    // Claude Haiku - fast and cheap
    pub const HAIKU_INPUT_PER_1K: f64 = 0.00025;
    pub const HAIKU_OUTPUT_PER_1K: f64 = 0.00125;

    // Claude Sonnet - balanced
    pub const SONNET_INPUT_PER_1K: f64 = 0.003;
    pub const SONNET_OUTPUT_PER_1K: f64 = 0.015;

    // Claude Opus - most capable
    pub const OPUS_INPUT_PER_1K: f64 = 0.015;
    pub const OPUS_OUTPUT_PER_1K: f64 = 0.075;

    // OpenAI GPT-4o
    pub const GPT4O_INPUT_PER_1K: f64 = 0.0025;
    pub const GPT4O_OUTPUT_PER_1K: f64 = 0.01;

    // Local inference - FREE!
    pub const LOCAL_INPUT_PER_1K: f64 = 0.0;
    pub const LOCAL_OUTPUT_PER_1K: f64 = 0.0;
}

// ============================================================================
// TIER DEFINITION
// ============================================================================

/// Execution tier for queries
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Tier {
    /// Local cache hit - instant, free
    Cache,
    /// Local LLM (Ollama, etc.) - free!
    Local,
    /// Cloud inference via OpenRouter auto-router
    Cloud,
    /// Claude Haiku - fast, cheap
    Haiku,
    /// Claude Sonnet - balanced
    Sonnet,
    /// Claude Opus - most capable, most expensive
    Opus,
    /// OpenAI GPT-4o
    Gpt4o,
}

impl Tier {
    /// Get the display name for the tier
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

    /// Get a symbol/emoji for the tier
    pub fn symbol(&self) -> &'static str {
        match self {
            Self::Cache => "[CACHE]",
            Self::Local => "[LOCAL]",
            Self::Cloud => "[CLOUD]",
            Self::Haiku => "[HAIKU]",
            Self::Sonnet => "[SONNET]",
            Self::Opus => "[OPUS]",
            Self::Gpt4o => "[GPT4O]",
        }
    }

    /// Input cost per 1K tokens (USD)
    pub fn input_cost_per_1k(&self) -> f64 {
        match self {
            Self::Cache | Self::Local => costs::LOCAL_INPUT_PER_1K,
            Self::Cloud => costs::CLOUD_INPUT_PER_1K,
            Self::Haiku => costs::HAIKU_INPUT_PER_1K,
            Self::Sonnet => costs::SONNET_INPUT_PER_1K,
            Self::Opus => costs::OPUS_INPUT_PER_1K,
            Self::Gpt4o => costs::GPT4O_INPUT_PER_1K,
        }
    }

    /// Output cost per 1K tokens (USD)
    pub fn output_cost_per_1k(&self) -> f64 {
        match self {
            Self::Cache | Self::Local => costs::LOCAL_OUTPUT_PER_1K,
            Self::Cloud => costs::CLOUD_OUTPUT_PER_1K,
            Self::Haiku => costs::HAIKU_OUTPUT_PER_1K,
            Self::Sonnet => costs::SONNET_OUTPUT_PER_1K,
            Self::Opus => costs::OPUS_OUTPUT_PER_1K,
            Self::Gpt4o => costs::GPT4O_OUTPUT_PER_1K,
        }
    }

    /// Calculate cost for given input and output tokens (USD)
    pub fn calculate_cost(&self, input_tokens: u32, output_tokens: u32) -> f64 {
        let input_cost = (input_tokens as f64 / 1000.0) * self.input_cost_per_1k();
        let output_cost = (output_tokens as f64 / 1000.0) * self.output_cost_per_1k();
        input_cost + output_cost
    }

    /// Is this tier free (local)?
    pub fn is_free(&self) -> bool {
        matches!(self, Self::Cache | Self::Local)
    }
}

// ============================================================================
// QUERY STATS
// ============================================================================

/// Statistics for a single query
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QueryStats {
    /// Which tier executed this query
    pub tier: Tier,
    /// Number of input tokens
    pub input_tokens: u32,
    /// Number of output tokens
    pub output_tokens: u32,
    /// Latency in milliseconds
    pub latency_ms: u64,
    /// When the query was executed
    pub timestamp: DateTime<Utc>,
    /// What this would have cost on Opus (USD)
    pub saved_vs_opus: f64,
    /// Actual cost of this query (USD)
    pub actual_cost: f64,
}

impl QueryStats {
    /// Create a new QueryStats
    pub fn new(
        tier: Tier,
        input_tokens: u32,
        output_tokens: u32,
        latency_ms: u64,
    ) -> Self {
        let actual_cost = tier.calculate_cost(input_tokens, output_tokens);
        let opus_cost = Tier::Opus.calculate_cost(input_tokens, output_tokens);
        let saved_vs_opus = opus_cost - actual_cost;

        Self {
            tier,
            input_tokens,
            output_tokens,
            latency_ms,
            timestamp: Utc::now(),
            saved_vs_opus,
            actual_cost,
        }
    }

    /// Total tokens used
    pub fn total_tokens(&self) -> u32 {
        self.input_tokens + self.output_tokens
    }
}

// ============================================================================
// SESSION STATS
// ============================================================================

/// Statistics for the current session (in-memory)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SessionStats {
    /// Total number of queries this session
    pub total_queries: u32,
    /// Queries handled by local models (free!)
    pub local_queries: u32,
    /// Cache hits (instant, free!)
    pub cache_hits: u32,
    /// Queries sent to cloud APIs (paid)
    pub cloud_queries: u32,
    /// Total tokens processed
    pub total_tokens: u64,
    /// Total money saved vs all-Opus (USD)
    pub total_saved: f64,
    /// Total actual cost (USD)
    pub total_cost: f64,
    /// Session start time
    pub session_start: DateTime<Utc>,
    /// Per-tier breakdown
    pub by_tier: HashMap<String, TierStats>,
    /// Recent queries (for display)
    #[serde(skip)]
    pub recent_queries: Vec<QueryStats>,
}

impl Default for SessionStats {
    fn default() -> Self {
        Self {
            total_queries: 0,
            local_queries: 0,
            cache_hits: 0,
            cloud_queries: 0,
            total_tokens: 0,
            total_saved: 0.0,
            total_cost: 0.0,
            session_start: Utc::now(),
            by_tier: HashMap::new(),
            recent_queries: Vec::new(),
        }
    }
}

impl SessionStats {
    /// Create a new session
    pub fn new() -> Self {
        Self::default()
    }

    /// Record a query
    pub fn record(&mut self, query: &QueryStats) {
        self.total_queries += 1;
        self.total_tokens += query.total_tokens() as u64;
        self.total_saved += query.saved_vs_opus;
        self.total_cost += query.actual_cost;

        match query.tier {
            Tier::Cache => self.cache_hits += 1,
            Tier::Local => self.local_queries += 1,
            _ => self.cloud_queries += 1,
        }

        // Update tier stats
        let tier_name = query.tier.name().to_string();
        let tier_stats = self.by_tier.entry(tier_name).or_default();
        tier_stats.queries += 1;
        tier_stats.tokens += query.total_tokens() as u64;
        tier_stats.cost += query.actual_cost;
        tier_stats.saved += query.saved_vs_opus;
        tier_stats.total_latency_ms += query.latency_ms;

        // Keep last 100 queries
        self.recent_queries.push(query.clone());
        if self.recent_queries.len() > 100 {
            self.recent_queries.remove(0);
        }
    }

    /// Get cache hit rate as percentage
    pub fn cache_hit_rate(&self) -> f64 {
        if self.total_queries == 0 {
            0.0
        } else {
            (self.cache_hits as f64 / self.total_queries as f64) * 100.0
        }
    }

    /// Get local query rate as percentage
    pub fn local_rate(&self) -> f64 {
        if self.total_queries == 0 {
            0.0
        } else {
            ((self.cache_hits + self.local_queries) as f64 / self.total_queries as f64) * 100.0
        }
    }

    /// Session duration
    pub fn duration(&self) -> Duration {
        Utc::now() - self.session_start
    }
}

/// Per-tier statistics
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TierStats {
    pub queries: u64,
    pub tokens: u64,
    pub cost: f64,
    pub saved: f64,
    pub total_latency_ms: u64,
}

impl TierStats {
    /// Average latency in milliseconds
    pub fn avg_latency_ms(&self) -> u64 {
        if self.queries == 0 {
            0
        } else {
            self.total_latency_ms / self.queries
        }
    }
}

// ============================================================================
// ALL-TIME STATS (Persisted)
// ============================================================================

/// All-time statistics, persisted to disk
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AllTimeStats {
    /// Total queries ever
    pub total_queries: u64,
    /// Total local queries
    pub local_queries: u64,
    /// Total cache hits
    pub cache_hits: u64,
    /// Total cloud queries
    pub cloud_queries: u64,
    /// Total tokens processed
    pub total_tokens: u64,
    /// Total money saved (USD)
    pub total_saved: f64,
    /// Total money spent (USD)
    pub total_spent: f64,
    /// First query timestamp
    pub first_query: Option<DateTime<Utc>>,
    /// Last query timestamp
    pub last_query: Option<DateTime<Utc>>,
    /// Per-tier breakdown
    pub by_tier: HashMap<String, TierStats>,
    /// Daily savings history (for sparklines/graphs)
    pub daily_savings: Vec<DailySavings>,
}

impl Default for AllTimeStats {
    fn default() -> Self {
        Self {
            total_queries: 0,
            local_queries: 0,
            cache_hits: 0,
            cloud_queries: 0,
            total_tokens: 0,
            total_saved: 0.0,
            total_spent: 0.0,
            first_query: None,
            last_query: None,
            by_tier: HashMap::new(),
            daily_savings: Vec::new(),
        }
    }
}

/// Daily savings record
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DailySavings {
    pub date: String, // YYYY-MM-DD
    pub queries: u32,
    pub saved: f64,
    pub spent: f64,
}

impl AllTimeStats {
    /// Record a query
    pub fn record(&mut self, query: &QueryStats) {
        self.total_queries += 1;
        self.total_tokens += query.total_tokens() as u64;
        self.total_saved += query.saved_vs_opus;
        self.total_spent += query.actual_cost;

        if self.first_query.is_none() {
            self.first_query = Some(query.timestamp);
        }
        self.last_query = Some(query.timestamp);

        match query.tier {
            Tier::Cache => self.cache_hits += 1,
            Tier::Local => self.local_queries += 1,
            _ => self.cloud_queries += 1,
        }

        // Update tier stats
        let tier_name = query.tier.name().to_string();
        let tier_stats = self.by_tier.entry(tier_name).or_default();
        tier_stats.queries += 1;
        tier_stats.tokens += query.total_tokens() as u64;
        tier_stats.cost += query.actual_cost;
        tier_stats.saved += query.saved_vs_opus;
        tier_stats.total_latency_ms += query.latency_ms;

        // Update daily savings
        let today = query.timestamp.format("%Y-%m-%d").to_string();
        if let Some(daily) = self.daily_savings.iter_mut().find(|d| d.date == today) {
            daily.queries += 1;
            daily.saved += query.saved_vs_opus;
            daily.spent += query.actual_cost;
        } else {
            self.daily_savings.push(DailySavings {
                date: today,
                queries: 1,
                saved: query.saved_vs_opus,
                spent: query.actual_cost,
            });
            // Keep last 365 days
            if self.daily_savings.len() > 365 {
                self.daily_savings.remove(0);
            }
        }
    }

    /// Get savings for today
    pub fn today_savings(&self) -> f64 {
        let today = Utc::now().format("%Y-%m-%d").to_string();
        self.daily_savings
            .iter()
            .find(|d| d.date == today)
            .map(|d| d.saved)
            .unwrap_or(0.0)
    }

    /// Get savings for this week
    pub fn week_savings(&self) -> f64 {
        let week_ago = (Utc::now() - Duration::days(7)).format("%Y-%m-%d").to_string();
        self.daily_savings
            .iter()
            .filter(|d| d.date >= week_ago)
            .map(|d| d.saved)
            .sum()
    }

    /// Get savings for this month
    pub fn month_savings(&self) -> f64 {
        let month_ago = (Utc::now() - Duration::days(30)).format("%Y-%m-%d").to_string();
        self.daily_savings
            .iter()
            .filter(|d| d.date >= month_ago)
            .map(|d| d.saved)
            .sum()
    }
}

// ============================================================================
// SAVINGS SUMMARY
// ============================================================================

/// A summary of savings for display
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SavingsSummary {
    /// Savings today
    pub today: f64,
    /// Savings this week
    pub this_week: f64,
    /// Savings this month
    pub this_month: f64,
    /// All-time savings
    pub all_time: f64,
    /// Percentage run locally (free)
    pub local_percentage: f64,
    /// Cache hit rate
    pub cache_hit_rate: f64,
    /// Total queries
    pub total_queries: u64,
    /// Equivalent Opus queries worth
    pub opus_equivalent: u64,
}

impl SavingsSummary {
    /// Create a summary from all-time stats
    pub fn from_stats(all_time: &AllTimeStats, _session: &SessionStats) -> Self {
        let total_queries = all_time.total_queries;
        let local_and_cache = all_time.local_queries + all_time.cache_hits;

        Self {
            today: all_time.today_savings(),
            this_week: all_time.week_savings(),
            this_month: all_time.month_savings(),
            all_time: all_time.total_saved,
            local_percentage: if total_queries > 0 {
                (local_and_cache as f64 / total_queries as f64) * 100.0
            } else {
                0.0
            },
            cache_hit_rate: if total_queries > 0 {
                (all_time.cache_hits as f64 / total_queries as f64) * 100.0
            } else {
                0.0
            },
            total_queries,
            // Estimate how many Opus queries the savings would have bought
            opus_equivalent: (all_time.total_saved / 0.09).round() as u64, // ~$0.09 per typical Opus query
        }
    }
}

// ============================================================================
// STATS TRACKER
// ============================================================================

/// The main stats tracker - thread-safe, handles persistence
pub struct StatsTracker {
    session: Arc<RwLock<SessionStats>>,
    all_time: Arc<RwLock<AllTimeStats>>,
    stats_path: PathBuf,
}

impl StatsTracker {
    /// Create a new stats tracker, loading existing stats from disk
    pub fn new() -> Result<Self> {
        let stats_dir = Self::stats_dir();
        fs::create_dir_all(&stats_dir)?;

        let stats_path = stats_dir.join("stats.json");
        let all_time = if stats_path.exists() {
            let content = fs::read_to_string(&stats_path)?;
            serde_json::from_str(&content).unwrap_or_default()
        } else {
            AllTimeStats::default()
        };

        Ok(Self {
            session: Arc::new(RwLock::new(SessionStats::new())),
            all_time: Arc::new(RwLock::new(all_time)),
            stats_path,
        })
    }

    /// Get the stats directory path
    pub fn stats_dir() -> PathBuf {
        dirs::home_dir()
            .unwrap_or_else(|| PathBuf::from("."))
            .join(".rigrun")
    }

    /// Record a query
    pub fn record_query(&self, stats: QueryStats) -> Result<()> {
        // Update session stats
        if let Ok(mut session) = self.session.write() {
            session.record(&stats);
        }

        // Update all-time stats
        if let Ok(mut all_time) = self.all_time.write() {
            all_time.record(&stats);
        }

        Ok(())
    }

    /// Get current session stats
    pub fn get_session_stats(&self) -> SessionStats {
        self.session
            .read()
            .map(|s| s.clone())
            .unwrap_or_default()
    }

    /// Get all-time stats
    pub fn get_all_time_stats(&self) -> AllTimeStats {
        self.all_time
            .read()
            .map(|s| s.clone())
            .unwrap_or_default()
    }

    /// Get a savings summary
    pub fn get_savings_summary(&self) -> SavingsSummary {
        let all_time = self.get_all_time_stats();
        let session = self.get_session_stats();
        SavingsSummary::from_stats(&all_time, &session)
    }

    /// Persist stats to disk
    pub fn persist_stats(&self) -> Result<()> {
        if let Ok(all_time) = self.all_time.read() {
            let content = serde_json::to_string_pretty(&*all_time)?;
            fs::write(&self.stats_path, content)?;
        }
        Ok(())
    }

    /// Load stats from disk (called internally by new())
    pub fn load_stats() -> Result<AllTimeStats> {
        let stats_path = Self::stats_dir().join("stats.json");
        if stats_path.exists() {
            let content = fs::read_to_string(&stats_path)?;
            Ok(serde_json::from_str(&content)?)
        } else {
            Ok(AllTimeStats::default())
        }
    }
}

impl Drop for StatsTracker {
    fn drop(&mut self) {
        // Auto-save on drop
        let _ = self.persist_stats();
    }
}

// ============================================================================
// PRETTY PRINTING
// ============================================================================

/// Format session stats for CLI display
pub fn format_stats(session: &SessionStats, all_time: &AllTimeStats) -> String {
    let mut output = String::new();

    output.push('\n');
    output.push_str("================================================================================\n");
    output.push_str("                           RIGRUN SAVINGS REPORT                               \n");
    output.push_str("================================================================================\n\n");

    // Session summary
    output.push_str("--- THIS SESSION ---\n\n");
    output.push_str(&format!("  Queries:     {}\n", session.total_queries));
    output.push_str(&format!("  Local:       {} ({:.1}% free!)\n",
        session.local_queries + session.cache_hits,
        session.local_rate()
    ));
    output.push_str(&format!("  Cache Hits:  {} ({:.1}%)\n",
        session.cache_hits,
        session.cache_hit_rate()
    ));
    output.push_str(&format!("  Cloud:       {}\n", session.cloud_queries));
    output.push_str(&format!("  Tokens:      {}\n", format_number(session.total_tokens)));
    output.push('\n');
    output.push_str(&format!("  SAVED:       ${:.2}\n", session.total_saved));
    output.push_str(&format!("  Spent:       ${:.4}\n", session.total_cost));

    // All-time summary
    output.push_str("\n--- ALL TIME ---\n\n");
    output.push_str(&format!("  Total Queries: {}\n", format_number(all_time.total_queries)));
    output.push_str(&format!("  Total Tokens:  {}\n", format_number(all_time.total_tokens)));
    output.push('\n');

    // The dopamine hit - BIG savings numbers
    output.push_str("  +------------------------------------------+\n");
    output.push_str("  |                                          |\n");
    output.push_str(&format!("  |   TODAY:      ${:>10.2}               |\n", all_time.today_savings()));
    output.push_str(&format!("  |   THIS WEEK:  ${:>10.2}               |\n", all_time.week_savings()));
    output.push_str(&format!("  |   THIS MONTH: ${:>10.2}               |\n", all_time.month_savings()));
    output.push_str("  |                                          |\n");
    output.push_str(&format!("  |   ALL TIME:   ${:>10.2}  SAVED!       |\n", all_time.total_saved));
    output.push_str("  |                                          |\n");
    output.push_str("  +------------------------------------------+\n");

    // Tier breakdown
    output.push_str("\n--- BY TIER ---\n\n");
    for tier in [Tier::Cache, Tier::Local, Tier::Cloud, Tier::Haiku, Tier::Sonnet, Tier::Opus, Tier::Gpt4o] {
        if let Some(stats) = all_time.by_tier.get(tier.name()) {
            if stats.queries > 0 {
                output.push_str(&format!(
                    "  {:8} {:>8} queries | {:>10} tokens | ${:>8.4} spent | ${:>8.2} saved | {:>6}ms avg\n",
                    tier.symbol(),
                    format_number(stats.queries),
                    format_number(stats.tokens),
                    stats.cost,
                    stats.saved,
                    stats.avg_latency_ms()
                ));
            }
        }
    }

    // Motivational message based on savings
    output.push('\n');
    output.push_str(&get_motivation_message(all_time.total_saved));

    output.push_str("\n================================================================================\n");

    output
}

/// Format a savings summary for quick display
pub fn format_savings_quick(summary: &SavingsSummary) -> String {
    format!(
        "Saved: ${:.2} today | ${:.2} this week | ${:.2} all time | {:.0}% local",
        summary.today,
        summary.this_week,
        summary.all_time,
        summary.local_percentage
    )
}

/// Format session stats for quick display
pub fn format_session_quick(session: &SessionStats) -> String {
    format!(
        "{} queries | {} tokens | ${:.2} saved | ${:.4} spent",
        session.total_queries,
        format_number(session.total_tokens),
        session.total_saved,
        session.total_cost
    )
}

/// Convert stats to JSON for API/dashboard
pub fn to_json(session: &SessionStats, all_time: &AllTimeStats) -> Result<String> {
    let combined = serde_json::json!({
        "session": session,
        "all_time": all_time,
        "summary": {
            "today_saved": all_time.today_savings(),
            "week_saved": all_time.week_savings(),
            "month_saved": all_time.month_savings(),
            "all_time_saved": all_time.total_saved,
            "local_percentage": if all_time.total_queries > 0 {
                ((all_time.local_queries + all_time.cache_hits) as f64 / all_time.total_queries as f64) * 100.0
            } else {
                0.0
            }
        }
    });
    Ok(serde_json::to_string_pretty(&combined)?)
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

/// Format large numbers with commas
fn format_number(n: u64) -> String {
    let s = n.to_string();
    let mut result = String::new();
    for (i, c) in s.chars().rev().enumerate() {
        if i > 0 && i % 3 == 0 {
            result.push(',');
        }
        result.push(c);
    }
    result.chars().rev().collect()
}

/// Get a motivational message based on savings
fn get_motivation_message(total_saved: f64) -> String {
    if total_saved < 1.0 {
        "  Keep going! Every query run locally is money saved.\n".to_string()
    } else if total_saved < 10.0 {
        format!("  Nice! You've saved ${:.2} - that's a fancy coffee!\n", total_saved)
    } else if total_saved < 50.0 {
        format!("  Awesome! ${:.2} saved - that's a nice lunch!\n", total_saved)
    } else if total_saved < 100.0 {
        format!("  Impressive! ${:.2} saved - treat yourself to dinner!\n", total_saved)
    } else if total_saved < 500.0 {
        format!("  Amazing! ${:.2} saved - that's real money!\n", total_saved)
    } else if total_saved < 1000.0 {
        format!("  Incredible! ${:.2} saved - you're a local inference pro!\n", total_saved)
    } else {
        format!("  LEGENDARY! ${:.2} saved - you're basically printing money!\n", total_saved)
    }
}

/// Calculate what a query would cost on Opus
pub fn calculate_opus_cost(input_tokens: u32, output_tokens: u32) -> f64 {
    Tier::Opus.calculate_cost(input_tokens, output_tokens)
}

/// Calculate savings compared to Opus
pub fn calculate_savings(tier: Tier, input_tokens: u32, output_tokens: u32) -> f64 {
    let actual = tier.calculate_cost(input_tokens, output_tokens);
    let opus = calculate_opus_cost(input_tokens, output_tokens);
    opus - actual
}

// ============================================================================
// GLOBAL TRACKER
// ============================================================================

use std::sync::OnceLock;

static GLOBAL_TRACKER: OnceLock<StatsTracker> = OnceLock::new();

/// Get or initialize the global stats tracker
pub fn global_tracker() -> &'static StatsTracker {
    GLOBAL_TRACKER.get_or_init(|| {
        StatsTracker::new().expect("Failed to initialize stats tracker")
    })
}

/// Record a query to the global tracker
pub fn record_query(stats: QueryStats) {
    let _ = global_tracker().record_query(stats);
}

/// Get session stats from the global tracker
pub fn get_session_stats() -> SessionStats {
    global_tracker().get_session_stats()
}

/// Get savings summary from the global tracker
pub fn get_savings_summary() -> SavingsSummary {
    global_tracker().get_savings_summary()
}

/// Persist global stats to disk
pub fn persist_stats() -> Result<()> {
    global_tracker().persist_stats()
}

/// Load stats from disk
pub fn load_stats() -> Result<AllTimeStats> {
    StatsTracker::load_stats()
}

// ============================================================================
// TESTS
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_tier_costs() {
        // Local is free
        assert_eq!(Tier::Local.calculate_cost(1000, 1000), 0.0);
        assert_eq!(Tier::Cache.calculate_cost(1000, 1000), 0.0);

        // Haiku costs
        let haiku_cost = Tier::Haiku.calculate_cost(1000, 1000);
        assert!((haiku_cost - 0.0015).abs() < 0.0001); // 0.00025 + 0.00125

        // Opus costs
        let opus_cost = Tier::Opus.calculate_cost(1000, 1000);
        assert!((opus_cost - 0.09).abs() < 0.0001); // 0.015 + 0.075
    }

    #[test]
    fn test_savings_calculation() {
        let savings = calculate_savings(Tier::Local, 1000, 1000);
        let opus_cost = Tier::Opus.calculate_cost(1000, 1000);
        assert!((savings - opus_cost).abs() < 0.0001);
    }

    #[test]
    fn test_query_stats() {
        let stats = QueryStats::new(Tier::Local, 1000, 500, 100);

        assert_eq!(stats.tier, Tier::Local);
        assert_eq!(stats.total_tokens(), 1500);
        assert_eq!(stats.actual_cost, 0.0);
        assert!(stats.saved_vs_opus > 0.0);
    }

    #[test]
    fn test_session_stats() {
        let mut session = SessionStats::new();

        let query1 = QueryStats::new(Tier::Local, 1000, 500, 100);
        let query2 = QueryStats::new(Tier::Cache, 500, 0, 5);
        let query3 = QueryStats::new(Tier::Haiku, 2000, 1000, 800);

        session.record(&query1);
        session.record(&query2);
        session.record(&query3);

        assert_eq!(session.total_queries, 3);
        assert_eq!(session.local_queries, 1);
        assert_eq!(session.cache_hits, 1);
        assert_eq!(session.cloud_queries, 1);
        assert!(session.total_saved > 0.0);
    }

    #[test]
    fn test_format_number() {
        assert_eq!(format_number(1000), "1,000");
        assert_eq!(format_number(1000000), "1,000,000");
        assert_eq!(format_number(123), "123");
    }

    #[test]
    fn test_motivation_messages() {
        assert!(get_motivation_message(0.5).contains("Keep going"));
        assert!(get_motivation_message(5.0).contains("coffee"));
        assert!(get_motivation_message(25.0).contains("lunch"));
        assert!(get_motivation_message(75.0).contains("dinner"));
        assert!(get_motivation_message(200.0).contains("real money"));
        assert!(get_motivation_message(750.0).contains("pro"));
        assert!(get_motivation_message(2000.0).contains("LEGENDARY"));
    }
}
