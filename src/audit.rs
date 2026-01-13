//! Audit Logging Module for Rigrun Privacy Features
//!
//! Logs every query with details for transparency and privacy auditing.
//! Part of the "privacy maximalism" feature set.
//!
//! Log format:
//! `2024-01-15 10:23:45 | CACHE_HIT | "What is recursi..." | 0 tokens | $0.00`

use anyhow::Result;
use chrono::{DateTime, Local, Utc};
use regex::Regex;
use serde::{Deserialize, Serialize};
use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::PathBuf;
use std::sync::{Arc, RwLock, OnceLock, LazyLock};

use crate::types::Tier;

/// Maximum length of query preview in audit log
const QUERY_PREVIEW_LENGTH: usize = 50;

/// Redaction patterns for sensitive data
/// JUSTIFICATION for .unwrap(): These are static, compile-time-validated regex patterns.
/// If any of these fail to compile, it's a programmer error that should be caught in testing.
/// This initialization happens once at startup, not during request handling.
static REDACTION_PATTERNS: LazyLock<Vec<(Regex, &'static str)>> = LazyLock::new(|| {
    vec![
        (Regex::new(r"sk-[a-zA-Z0-9]{20,}").expect("OpenAI key regex is valid"), "[REDACTED_API_KEY]"),
        (Regex::new(r"sk-or-[a-zA-Z0-9-]{20,}").expect("OpenRouter key regex is valid"), "[REDACTED_API_KEY]"),
        (Regex::new(r"sk-ant-[a-zA-Z0-9-]{20,}").expect("Anthropic key regex is valid"), "[REDACTED_API_KEY]"),
        (Regex::new(r"AKIA[0-9A-Z]{16}").expect("AWS key regex is valid"), "[REDACTED_AWS_KEY]"),
        (Regex::new(r"ghp_[a-zA-Z0-9]{36}").expect("GitHub token regex is valid"), "[REDACTED_GITHUB_TOKEN]"),
        (Regex::new(r"password[=:]\s*\S+").expect("Password regex is valid"), "password=[REDACTED]"),
        (Regex::new(r"Bearer [a-zA-Z0-9-._~+/]+=*").expect("Bearer token regex is valid"), "Bearer [REDACTED]"),
        (Regex::new(r"\b[A-Za-z0-9]{32,}\b").expect("Generic key regex is valid"), "[REDACTED_KEY]"),
    ]
});

/// Redact secrets from text before logging
pub fn redact_secrets(text: &str) -> String {
    let mut result = text.to_string();
    for (pattern, replacement) in REDACTION_PATTERNS.iter() {
        result = pattern.replace_all(&result, *replacement).to_string();
    }
    result
}

/// Audit log entry
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AuditEntry {
    /// Timestamp of the query
    pub timestamp: DateTime<Utc>,
    /// Which tier handled the request
    pub tier: AuditTier,
    /// Preview of the query (first N chars)
    pub query_preview: String,
    /// Total tokens used (input + output)
    pub tokens: u32,
    /// Cost in USD
    pub cost_usd: f64,
    /// Whether this was blocked (paranoid mode)
    pub blocked: bool,
}

/// Simplified tier enum for audit logging
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum AuditTier {
    CacheHit,
    Local,
    Cloud,
    CloudBlocked,
}

impl AuditTier {
    pub fn from_tier(tier: Tier, blocked: bool) -> Self {
        if blocked {
            return Self::CloudBlocked;
        }
        match tier {
            Tier::Cache => Self::CacheHit,
            Tier::Local => Self::Local,
            Tier::Cloud | Tier::Haiku | Tier::Sonnet | Tier::Opus | Tier::Gpt4o => Self::Cloud,
        }
    }

    pub fn as_str(&self) -> &'static str {
        match self {
            Self::CacheHit => "CACHE_HIT",
            Self::Local => "LOCAL",
            Self::Cloud => "CLOUD",
            Self::CloudBlocked => "CLOUD_BLOCKED",
        }
    }
}

impl std::fmt::Display for AuditTier {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.as_str())
    }
}

impl AuditEntry {
    /// Create a new audit entry
    pub fn new(
        tier: Tier,
        query: &str,
        input_tokens: u32,
        output_tokens: u32,
        cost_usd: f64,
        blocked: bool,
    ) -> Self {
        // Redact secrets before truncating
        let redacted_query = redact_secrets(query);
        let query_preview = truncate_query(&redacted_query, QUERY_PREVIEW_LENGTH);

        Self {
            timestamp: Utc::now(),
            tier: AuditTier::from_tier(tier, blocked),
            query_preview,
            tokens: input_tokens + output_tokens,
            cost_usd,
            blocked,
        }
    }

    /// Format as a log line
    pub fn to_log_line(&self) -> String {
        let local_time: DateTime<Local> = self.timestamp.into();
        format!(
            "{} | {:>13} | \"{}\" | {} tokens | ${:.2}",
            local_time.format("%Y-%m-%d %H:%M:%S"),
            self.tier.as_str(),
            self.query_preview,
            self.tokens,
            self.cost_usd
        )
    }
}

/// Truncate query to a preview length, adding ellipsis if needed
fn truncate_query(query: &str, max_len: usize) -> String {
    // Remove newlines and excessive whitespace for cleaner log
    let cleaned: String = query
        .chars()
        .map(|c| if c.is_whitespace() { ' ' } else { c })
        .collect::<String>()
        .split_whitespace()
        .collect::<Vec<_>>()
        .join(" ");

    if cleaned.len() <= max_len {
        cleaned
    } else {
        // Find a good break point (word boundary if possible)
        let truncated = &cleaned[..max_len.saturating_sub(3)];
        format!("{}...", truncated)
    }
}

/// Audit logger that writes to ~/.rigrun/audit.log
pub struct AuditLogger {
    /// Path to the audit log file
    log_path: PathBuf,
    /// Whether logging is enabled
    enabled: bool,
    /// In-memory buffer of recent entries (for export)
    recent_entries: RwLock<Vec<AuditEntry>>,
}

impl AuditLogger {
    /// Create a new audit logger
    pub fn new(enabled: bool) -> Result<Self> {
        let log_dir = Self::log_dir();
        fs::create_dir_all(&log_dir)?;

        Ok(Self {
            log_path: log_dir.join("audit.log"),
            enabled,
            recent_entries: RwLock::new(Vec::new()),
        })
    }

    /// Get the log directory path (~/.rigrun)
    pub fn log_dir() -> PathBuf {
        dirs::home_dir()
            .unwrap_or_else(|| PathBuf::from("."))
            .join(".rigrun")
    }

    /// Get the audit log file path
    pub fn log_path(&self) -> &PathBuf {
        &self.log_path
    }

    /// Check if logging is enabled
    pub fn is_enabled(&self) -> bool {
        self.enabled
    }

    /// Enable or disable logging
    pub fn set_enabled(&mut self, enabled: bool) {
        self.enabled = enabled;
    }

    /// Log a query
    pub fn log(&self, entry: AuditEntry) -> Result<()> {
        if !self.enabled {
            return Ok(());
        }

        // Store in memory
        if let Ok(mut recent) = self.recent_entries.write() {
            recent.push(entry.clone());
            // Keep last 10000 entries in memory
            if recent.len() > 10000 {
                recent.remove(0);
            }
        }

        // Append to file
        let log_line = entry.to_log_line();
        let mut file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&self.log_path)?;

        writeln!(file, "{}", log_line)?;

        Ok(())
    }

    /// Log a query with tier information
    pub fn log_query(
        &self,
        tier: Tier,
        query: &str,
        input_tokens: u32,
        output_tokens: u32,
        cost_usd: f64,
    ) -> Result<()> {
        let entry = AuditEntry::new(tier, query, input_tokens, output_tokens, cost_usd, false);
        self.log(entry)
    }

    /// Log a blocked cloud request (paranoid mode)
    pub fn log_blocked(&self, tier: Tier, query: &str) -> Result<()> {
        let entry = AuditEntry::new(tier, query, 0, 0, 0.0, true);
        self.log(entry)
    }

    /// Get recent entries from memory
    pub fn get_recent_entries(&self) -> Vec<AuditEntry> {
        self.recent_entries
            .read()
            .map(|r| r.clone())
            .unwrap_or_default()
    }

    /// Read all entries from the log file
    pub fn read_all_entries(&self) -> Result<Vec<String>> {
        if !self.log_path.exists() {
            return Ok(Vec::new());
        }

        let content = fs::read_to_string(&self.log_path)?;
        Ok(content.lines().map(String::from).collect())
    }

    /// Get log file size in bytes
    pub fn log_size_bytes(&self) -> u64 {
        fs::metadata(&self.log_path)
            .map(|m| m.len())
            .unwrap_or(0)
    }

    /// Count total log entries
    pub fn entry_count(&self) -> usize {
        self.read_all_entries()
            .map(|entries| entries.len())
            .unwrap_or(0)
    }

    /// Export log to JSON format
    pub fn export_to_json(&self) -> Result<String> {
        let entries = self.read_all_entries()?;

        // Parse log lines back into structured data
        let parsed_entries: Vec<serde_json::Value> = entries
            .iter()
            .map(|line| {
                serde_json::json!({
                    "raw_log": line
                })
            })
            .collect();

        Ok(serde_json::to_string_pretty(&parsed_entries)?)
    }

    /// Clear the audit log
    pub fn clear(&self) -> Result<()> {
        if self.log_path.exists() {
            fs::remove_file(&self.log_path)?;
        }
        if let Ok(mut recent) = self.recent_entries.write() {
            recent.clear();
        }
        Ok(())
    }
}

// ============================================================================
// GLOBAL AUDIT LOGGER
// ============================================================================

static GLOBAL_AUDIT_LOGGER: OnceLock<Arc<RwLock<AuditLogger>>> = OnceLock::new();

/// Get or initialize the global audit logger
/// JUSTIFICATION for .expect(): This is global initialization code that runs once at startup.
/// If creating the audit log directory fails, it indicates severe filesystem issues
/// (no home directory, no write permissions, disk full, etc.) that should prevent startup.
pub fn global_audit_logger() -> &'static Arc<RwLock<AuditLogger>> {
    GLOBAL_AUDIT_LOGGER.get_or_init(|| {
        Arc::new(RwLock::new(
            AuditLogger::new(true).expect("Failed to initialize audit logger. Cannot create ~/.rigrun directory. Check filesystem permissions and disk space.")
        ))
    })
}

/// Initialize the global audit logger with specific settings
pub fn init_audit_logger(enabled: bool) -> Result<()> {
    let logger = AuditLogger::new(enabled)?;
    let _ = GLOBAL_AUDIT_LOGGER.set(Arc::new(RwLock::new(logger)));
    Ok(())
}

/// Log a query to the global audit logger
pub fn audit_log_query(
    tier: Tier,
    query: &str,
    input_tokens: u32,
    output_tokens: u32,
    cost_usd: f64,
) {
    if let Ok(logger) = global_audit_logger().read() {
        let _ = logger.log_query(tier, query, input_tokens, output_tokens, cost_usd);
    }
}

/// Log a blocked request to the global audit logger
pub fn audit_log_blocked(tier: Tier, query: &str) {
    if let Ok(logger) = global_audit_logger().read() {
        let _ = logger.log_blocked(tier, query);
    }
}

/// Check if audit logging is enabled
pub fn is_audit_enabled() -> bool {
    global_audit_logger()
        .read()
        .map(|l| l.is_enabled())
        .unwrap_or(false)
}

/// Set audit logging enabled/disabled
pub fn set_audit_enabled(enabled: bool) {
    if let Ok(mut logger) = global_audit_logger().write() {
        logger.set_enabled(enabled);
    }
}

// ============================================================================
// TESTS
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_truncate_query() {
        // Short query
        assert_eq!(truncate_query("hello", 50), "hello");

        // Long query
        let long = "a".repeat(100);
        let truncated = truncate_query(&long, 50);
        assert!(truncated.ends_with("..."));
        assert!(truncated.len() <= 50);

        // Query with newlines
        let with_newlines = "hello\nworld\ntest";
        let result = truncate_query(with_newlines, 50);
        assert!(!result.contains('\n'));
    }

    #[test]
    fn test_audit_tier_conversion() {
        assert_eq!(AuditTier::from_tier(Tier::Cache, false), AuditTier::CacheHit);
        assert_eq!(AuditTier::from_tier(Tier::Local, false), AuditTier::Local);
        assert_eq!(AuditTier::from_tier(Tier::Cloud, false), AuditTier::Cloud);
        assert_eq!(AuditTier::from_tier(Tier::Haiku, false), AuditTier::Cloud);
        assert_eq!(AuditTier::from_tier(Tier::Cloud, true), AuditTier::CloudBlocked);
    }

    #[test]
    fn test_audit_entry_log_line() {
        let entry = AuditEntry::new(
            Tier::Cache,
            "What is recursion in programming?",
            100,
            200,
            0.0,
            false,
        );

        let log_line = entry.to_log_line();
        assert!(log_line.contains("CACHE_HIT"));
        assert!(log_line.contains("What is recursion"));
        assert!(log_line.contains("300 tokens"));
        assert!(log_line.contains("$0.00"));
    }

    #[test]
    fn test_audit_tier_display() {
        assert_eq!(AuditTier::CacheHit.as_str(), "CACHE_HIT");
        assert_eq!(AuditTier::Local.as_str(), "LOCAL");
        assert_eq!(AuditTier::Cloud.as_str(), "CLOUD");
        assert_eq!(AuditTier::CloudBlocked.as_str(), "CLOUD_BLOCKED");
    }

    #[test]
    fn test_redact_secrets_openai_key() {
        let text = "Use this key: sk-1234567890abcdefghij1234567890";
        let redacted = redact_secrets(text);
        assert_eq!(redacted, "Use this key: [REDACTED_API_KEY]");
    }

    #[test]
    fn test_redact_secrets_openrouter_key() {
        let text = "OpenRouter key: sk-or-v1-1234567890abcdefghij1234567890";
        let redacted = redact_secrets(text);
        assert_eq!(redacted, "OpenRouter key: [REDACTED_API_KEY]");
    }

    #[test]
    fn test_redact_secrets_anthropic_key() {
        let text = "Anthropic key: sk-ant-api03-1234567890abcdefghij1234567890";
        let redacted = redact_secrets(text);
        assert_eq!(redacted, "Anthropic key: [REDACTED_API_KEY]");
    }

    #[test]
    fn test_redact_secrets_bearer_token() {
        let text = "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9";
        let redacted = redact_secrets(text);
        assert_eq!(redacted, "Authorization: Bearer [REDACTED]");
    }

    #[test]
    fn test_redact_secrets_long_alphanumeric() {
        let text = "Secret: abcdefghij1234567890abcdefghij1234567890";
        let redacted = redact_secrets(text);
        assert_eq!(redacted, "Secret: [REDACTED_KEY]");
    }

    #[test]
    fn test_redact_secrets_multiple_patterns() {
        let text = "OpenAI: sk-1234567890abcdefghij1234 and Bearer token123456789012345678901234567890";
        let redacted = redact_secrets(text);
        assert!(redacted.contains("[REDACTED_API_KEY]"));
        assert!(redacted.contains("Bearer [REDACTED]"));
    }

    #[test]
    fn test_redact_secrets_in_audit_entry() {
        let entry = AuditEntry::new(
            Tier::Cloud,
            "Query with key sk-1234567890abcdefghij1234567890",
            100,
            200,
            0.01,
            false,
        );
        assert!(entry.query_preview.contains("[REDACTED_API_KEY]"));
        assert!(!entry.query_preview.contains("sk-1234567890"));
    }

    #[test]
    fn test_redact_secrets_preserves_safe_text() {
        let text = "What is the capital of France?";
        let redacted = redact_secrets(text);
        assert_eq!(redacted, text);
    }

    #[test]
    fn test_redact_secrets_aws_key() {
        let text = "AWS key: AKIAIOSFODNN7EXAMPLE";
        let redacted = redact_secrets(text);
        assert_eq!(redacted, "AWS key: [REDACTED_AWS_KEY]");
    }

    #[test]
    fn test_redact_secrets_github_token() {
        let text = "GitHub token: ghp_1234567890abcdefghijklmnopqrstuvw";
        let redacted = redact_secrets(text);
        assert_eq!(redacted, "GitHub token: [REDACTED_GITHUB_TOKEN]");
    }

    #[test]
    fn test_redact_secrets_password() {
        let text = "Connect with password=secretpass123";
        let redacted = redact_secrets(text);
        assert_eq!(redacted, "Connect with password=[REDACTED]");
    }

    #[test]
    fn test_redact_secrets_password_colon() {
        let text = "Config password: mypassword123";
        let redacted = redact_secrets(text);
        assert_eq!(redacted, "Config password=[REDACTED]");
    }

    #[test]
    fn test_redact_secrets_multiple_new_patterns() {
        let text = "AWS: AKIAIOSFODNN7EXAMPLE, GitHub: ghp_abcdefghijklmnopqrstuvwxyz123456, password=secret123";
        let redacted = redact_secrets(text);
        assert!(redacted.contains("[REDACTED_AWS_KEY]"));
        assert!(redacted.contains("[REDACTED_GITHUB_TOKEN]"));
        assert!(redacted.contains("password=[REDACTED]"));
        assert!(!redacted.contains("AKIAIOSFODNN7EXAMPLE"));
        assert!(!redacted.contains("ghp_"));
        assert!(!redacted.contains("secret123"));
    }
}
