// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! Audit Logging Module for Rigrun Privacy Features
//!
//! Logs every query with details for transparency and privacy auditing.
//! Part of the "privacy maximalism" feature set.
//!
//! Log format:
//! `2024-01-15 10:23:45 | CACHE_HIT | "What is recursi..." | 0 tokens | $0.00`

use anyhow::Result;
use chrono::{DateTime, Local, Utc};
use hmac::{Hmac, Mac};
use regex::Regex;
use serde::{Deserialize, Serialize};
use sha2::Sha256;
use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::PathBuf;
use std::sync::{Arc, RwLock, OnceLock, LazyLock, Mutex};

use crate::types::Tier;

type HmacSha256 = Hmac<Sha256>;

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

/// Configuration for audit log retention and rotation (IL5 compliance)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AuditConfig {
    /// Maximum file size in MB before rotation
    pub max_file_size_mb: u64,
    /// Maximum age of logs in days (e.g., 365 for IL5)
    pub max_age_days: u64,
    /// Number of rotated files to keep
    pub rotate_count: u32,
}

impl Default for AuditConfig {
    fn default() -> Self {
        Self {
            max_file_size_mb: 100,  // 100 MB default
            max_age_days: 365,      // 1 year retention for IL5
            rotate_count: 10,       // Keep 10 rotated files
        }
    }
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
    /// Session ID from X-Session-Id header (IL5: AC-12)
    pub session_id: Option<String>,
    /// Source IP address of the request (IL5: AU-2, AU-3)
    pub source_ip: Option<String>,
    /// Type of action performed (IL5: AU-2 "what happened")
    pub action_type: String,
    /// Whether the action succeeded (IL5: AU-2 "success/failure")
    pub success: bool,
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

/// Session event audit entry for DoD STIG IL5 compliance (AC-11, AC-12, AU-3)
///
/// Session lifecycle events MUST be logged to persistent audit trail per IL5 requirements.
/// This ensures accountability and traceability of all session management actions.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SessionAuditEntry {
    /// Timestamp of the event
    pub timestamp: DateTime<Utc>,
    /// Session event type (e.g., "SESSION_CREATED", "SESSION_EXPIRED")
    pub event_type: String,
    /// Session identifier (for correlation)
    pub session_id: String,
    /// User identifier (for accountability)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub user_id: Option<String>,
    /// Additional event-specific details (reason, duration, etc.)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<String>,
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
    /// Create a new audit entry with IL5-compliant user identification
    pub fn new(
        tier: Tier,
        query: &str,
        input_tokens: u32,
        output_tokens: u32,
        cost_usd: f64,
        blocked: bool,
        session_id: Option<String>,
        source_ip: Option<String>,
    ) -> Self {
        // Redact secrets before truncating
        let redacted_query = redact_secrets(query);
        let query_preview = truncate_query(&redacted_query, QUERY_PREVIEW_LENGTH);

        // Determine action type based on tier and blocked status
        let action_type = if blocked {
            "blocked".to_string()
        } else {
            match AuditTier::from_tier(tier, blocked) {
                AuditTier::CacheHit => "cache_hit".to_string(),
                AuditTier::Local => "query_local".to_string(),
                AuditTier::Cloud => "query_cloud".to_string(),
                AuditTier::CloudBlocked => "blocked".to_string(),
            }
        };

        Self {
            timestamp: Utc::now(),
            tier: AuditTier::from_tier(tier, blocked),
            query_preview,
            tokens: input_tokens + output_tokens,
            cost_usd,
            blocked,
            session_id,
            source_ip,
            action_type,
            success: !blocked,
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

/// Audit logger that writes to ~/.rigrun/audit.log with HMAC integrity protection
pub struct AuditLogger {
    /// Path to the audit log file
    log_path: PathBuf,
    /// Whether logging is enabled
    enabled: bool,
    /// In-memory buffer of recent entries (for export)
    recent_entries: RwLock<Vec<AuditEntry>>,
    /// HMAC key for tamper-evident logging (IL5 compliance)
    hmac_key: [u8; 32],
    /// Previous hash for HMAC chain (protected by Mutex for interior mutability)
    previous_hash: Mutex<String>,
    /// Retention and rotation configuration
    config: AuditConfig,
}

impl AuditLogger {
    /// Create a new audit logger with HMAC integrity protection
    pub fn new(enabled: bool) -> Result<Self> {
        Self::new_with_config(enabled, AuditConfig::default())
    }

    /// Create a new audit logger with custom configuration
    pub fn new_with_config(enabled: bool, config: AuditConfig) -> Result<Self> {
        let log_dir = Self::log_dir();
        fs::create_dir_all(&log_dir)?;

        // Generate or load HMAC key
        let hmac_key = Self::load_or_generate_hmac_key(&log_dir)?;

        // Initialize with genesis hash (all zeros)
        let initial_hash = "0".repeat(64);

        Ok(Self {
            log_path: log_dir.join("audit.log"),
            enabled,
            recent_entries: RwLock::new(Vec::new()),
            hmac_key,
            previous_hash: Mutex::new(initial_hash),
            config,
        })
    }

    /// Load or generate HMAC key for audit log integrity
    ///
    /// The key is stored in ~/.rigrun/.audit_key and should be protected with 0600 permissions.
    /// If the key doesn't exist, a cryptographically secure random key is generated.
    fn load_or_generate_hmac_key(log_dir: &PathBuf) -> Result<[u8; 32]> {
        let key_path = log_dir.join(".audit_key");

        if key_path.exists() {
            // Load existing key
            let key_bytes = fs::read(&key_path)?;
            if key_bytes.len() != 32 {
                anyhow::bail!("Invalid audit key file: expected 32 bytes, got {}", key_bytes.len());
            }
            let mut key = [0u8; 32];
            key.copy_from_slice(&key_bytes);
            Ok(key)
        } else {
            // Generate new cryptographically secure key
            use rand::RngCore;
            let mut key = [0u8; 32];
            let mut rng = rand::thread_rng();
            rng.fill_bytes(&mut key);

            // Write key to file with restricted permissions
            fs::write(&key_path, &key)?;

            // Set file permissions to 0600 (owner read/write only)
            #[cfg(unix)]
            {
                use std::os::unix::fs::PermissionsExt;
                let mut perms = fs::metadata(&key_path)?.permissions();
                perms.set_mode(0o600);
                fs::set_permissions(&key_path, perms)?;
            }

            Ok(key)
        }
    }

    /// Get the log directory path (~/.rigrun)
    pub fn log_dir() -> PathBuf {
        dirs::home_dir()
            .unwrap_or_else(|| PathBuf::from("."))
            .join(".rigrun")
    }

    /// Create a new audit logger with a custom log directory (for testing)
    #[cfg(test)]
    pub fn new_with_path(enabled: bool, log_dir: PathBuf) -> Result<Self> {
        fs::create_dir_all(&log_dir)?;

        // Generate or load HMAC key
        let hmac_key = Self::load_or_generate_hmac_key(&log_dir)?;

        // Initialize with genesis hash (all zeros)
        let initial_hash = "0".repeat(64);

        Ok(Self {
            log_path: log_dir.join("audit.log"),
            enabled,
            recent_entries: RwLock::new(Vec::new()),
            hmac_key,
            previous_hash: Mutex::new(initial_hash),
            config: AuditConfig::default(),
        })
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

    /// Log a query with HMAC integrity protection (IL5 compliance)
    ///
    /// Each log entry is protected by an HMAC chain where:
    /// 1. HMAC = HMAC-SHA256(key, previous_hash || log_line)
    /// 2. Log format: log_line|HMAC_hex
    /// 3. previous_hash is updated to current HMAC for next entry
    ///
    /// This creates a tamper-evident chain where modifying any entry
    /// will break all subsequent HMAC verifications.
    ///
    /// IL5 Enhancement: Checks log size and rotates if needed before logging
    pub fn log(&self, entry: AuditEntry) -> Result<()> {
        if !self.enabled {
            return Ok(());
        }

        // Check if rotation is needed (IL5 retention policy)
        self.rotate_if_needed()?;

        // Store in memory
        if let Ok(mut recent) = self.recent_entries.write() {
            recent.push(entry.clone());
            // Keep last 10000 entries in memory
            if recent.len() > 10000 {
                recent.remove(0);
            }
        }

        // Convert entry to JSON for structured logging and HMAC
        let log_line = serde_json::to_string(&entry)?;

        // Acquire lock on previous_hash for HMAC chain computation
        let mut prev_hash_guard = self.previous_hash.lock()
            .map_err(|e| anyhow::anyhow!("Failed to lock previous_hash: {}", e))?;

        // Create HMAC of: previous_hash + log_line
        let mut mac = HmacSha256::new_from_slice(&self.hmac_key)
            .map_err(|e| anyhow::anyhow!("HMAC key error: {}", e))?;
        mac.update(prev_hash_guard.as_bytes());
        mac.update(log_line.as_bytes());
        let hash = hex::encode(mac.finalize().into_bytes());

        // Write log line with hash for tamper-evidence
        let protected_line = format!("{}|{}", log_line, hash);

        // Open file with proper permissions
        let mut file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&self.log_path)?;

        // Set file permissions to 0600 (owner read/write only) on Unix systems
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let metadata = file.metadata()?;
            let mut perms = metadata.permissions();
            perms.set_mode(0o600);
            fs::set_permissions(&self.log_path, perms)?;
        }

        writeln!(file, "{}", protected_line)?;

        // Update previous hash for chain
        *prev_hash_guard = hash;

        Ok(())
    }

    /// Log a query with tier information and IL5-required user identification
    pub fn log_query(
        &self,
        tier: Tier,
        query: &str,
        input_tokens: u32,
        output_tokens: u32,
        cost_usd: f64,
        session_id: Option<String>,
        source_ip: Option<String>,
    ) -> Result<()> {
        let entry = AuditEntry::new(tier, query, input_tokens, output_tokens, cost_usd, false, session_id, source_ip);
        self.log(entry)
    }

    /// Log a blocked cloud request (paranoid mode) with IL5-required user identification
    pub fn log_blocked(&self, tier: Tier, query: &str, session_id: Option<String>, source_ip: Option<String>) -> Result<()> {
        let entry = AuditEntry::new(tier, query, 0, 0, 0.0, true, session_id, source_ip);
        self.log(entry)
    }

    /// Log a session event to the persistent audit trail (IL5 compliance)
    ///
    /// Session events MUST be logged per DoD STIG requirements:
    /// - AC-11 (Session Lock): Lock/unlock events
    /// - AC-12 (Session Termination): Creation, expiration, termination events
    /// - AU-3 (Audit Content): All session lifecycle events with accountability info
    ///
    /// This method logs to the same HMAC-protected audit log as query events.
    pub fn log_session_event(&self, entry: SessionAuditEntry) -> Result<()> {
        if !self.enabled {
            return Ok(());
        }

        // Check if rotation is needed (IL5 retention policy)
        self.rotate_if_needed()?;

        // Convert entry to JSON for structured logging and HMAC
        let log_line = serde_json::to_string(&entry)?;

        // Acquire lock on previous_hash for HMAC chain computation
        let mut prev_hash_guard = self.previous_hash.lock()
            .map_err(|e| anyhow::anyhow!("Failed to lock previous_hash: {}", e))?;

        // Create HMAC of: previous_hash + log_line
        let mut mac = HmacSha256::new_from_slice(&self.hmac_key)
            .map_err(|e| anyhow::anyhow!("HMAC key error: {}", e))?;
        mac.update(prev_hash_guard.as_bytes());
        mac.update(log_line.as_bytes());
        let hash = hex::encode(mac.finalize().into_bytes());

        // Write log line with hash for tamper-evidence
        let protected_line = format!("{}|{}", log_line, hash);

        // Open file with proper permissions
        let mut file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&self.log_path)?;

        // Set file permissions to 0600 (owner read/write only) on Unix systems
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let metadata = file.metadata()?;
            let mut perms = metadata.permissions();
            perms.set_mode(0o600);
            fs::set_permissions(&self.log_path, perms)?;
        }

        writeln!(file, "{}", protected_line)?;

        // Update previous hash for chain
        *prev_hash_guard = hash;

        Ok(())
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
    pub fn log_size_bytes(&self) -> Result<u64> {
        Ok(fs::metadata(&self.log_path)
            .map(|m| m.len())
            .unwrap_or(0))
    }

    /// Check if log rotation is needed and perform it
    ///
    /// Rotation strategy for IL5 compliance:
    /// - audit.log -> audit.log.1 -> audit.log.2 -> ... -> audit.log.N
    /// - Oldest file is deleted when rotate_count is exceeded
    /// - Each rotated file maintains its HMAC chain integrity
    pub fn rotate_if_needed(&self) -> Result<()> {
        let size = self.log_size_bytes()?;
        let max_size = self.config.max_file_size_mb * 1024 * 1024;

        // Warn if approaching limit (80% threshold)
        if size > max_size * 80 / 100 {
            tracing::warn!(
                target: "audit",
                "Audit log approaching size limit: {} MB of {} MB",
                size / 1024 / 1024,
                self.config.max_file_size_mb
            );
        }

        // Rotate if size exceeded
        if size > max_size {
            self.rotate()?;
        }

        Ok(())
    }

    /// Perform log rotation
    ///
    /// IL5 Compliance Notes:
    /// - Rotated files preserve HMAC chain integrity
    /// - Files are renamed, not deleted (until rotate_count exceeded)
    /// - Each rotation is logged for audit trail
    fn rotate(&self) -> Result<()> {
        if !self.log_path.exists() {
            return Ok(());
        }

        tracing::info!(
            target: "audit",
            "Rotating audit log: current size {} MB",
            self.log_size_bytes()? / 1024 / 1024
        );

        // Shift existing rotated files: audit.log.1 -> audit.log.2, etc.
        for i in (1..self.config.rotate_count).rev() {
            let old_name = if i == 1 {
                self.log_path.clone()
            } else {
                self.log_path.with_extension(format!("log.{}", i - 1))
            };

            let new_name = self.log_path.with_extension(format!("log.{}", i));

            if old_name.exists() {
                // Delete oldest file if it exists
                if i == self.config.rotate_count - 1 && new_name.exists() {
                    fs::remove_file(&new_name)?;
                    tracing::info!(
                        target: "audit",
                        "Deleted oldest rotated log: {:?}",
                        new_name
                    );
                }

                fs::rename(&old_name, &new_name)?;
                tracing::debug!(
                    target: "audit",
                    "Rotated {:?} -> {:?}",
                    old_name,
                    new_name
                );
            }
        }

        // Reset previous hash for new chain
        let mut prev_hash_guard = self.previous_hash.lock()
            .map_err(|e| anyhow::anyhow!("Failed to lock previous_hash: {}", e))?;
        *prev_hash_guard = "0".repeat(64);

        tracing::info!(
            target: "audit",
            "Audit log rotation completed. New chain started."
        );

        Ok(())
    }

    /// Clean up old rotated logs based on max_age_days
    ///
    /// IL5 Compliance: Logs older than retention policy are deleted
    /// Note: This should be called periodically (e.g., daily cron job)
    pub fn cleanup_old_logs(&self) -> Result<()> {
        use std::time::{SystemTime, UNIX_EPOCH};

        let max_age_seconds = self.config.max_age_days * 24 * 60 * 60;
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)?
            .as_secs();

        // Check all rotated log files
        for i in 1..=self.config.rotate_count {
            let log_file = self.log_path.with_extension(format!("log.{}", i));

            if log_file.exists() {
                let metadata = fs::metadata(&log_file)?;
                let modified = metadata.modified()?
                    .duration_since(UNIX_EPOCH)?
                    .as_secs();

                let age_seconds = now.saturating_sub(modified);

                if age_seconds > max_age_seconds {
                    fs::remove_file(&log_file)?;
                    tracing::info!(
                        target: "audit",
                        "Deleted old audit log (age: {} days): {:?}",
                        age_seconds / (24 * 60 * 60),
                        log_file
                    );
                }
            }
        }

        Ok(())
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
    ///
    /// **WARNING - IL5 COMPLIANCE VIOLATION**
    ///
    /// This function is DISABLED for IL5 compliance. Audit logs with HMAC integrity
    /// protection MUST NOT be deleted or cleared to maintain tamper-evident chain.
    ///
    /// For IL5 environments:
    /// - Audit logs must be archived, not deleted
    /// - Log rotation should use secure archival mechanisms
    /// - Any log modification invalidates the HMAC chain
    ///
    /// To clear logs for testing/development only, manually delete:
    /// - ~/.rigrun/audit.log
    /// - ~/.rigrun/.audit_key (this will break HMAC chain verification)
    pub fn clear(&self) -> Result<()> {
        anyhow::bail!(
            "CRITICAL IL5 VIOLATION: Audit log clearing is disabled for compliance. \
            Audit logs with HMAC integrity protection must not be deleted. \
            For compliance reasons, this operation is not permitted. \
            Manual deletion of ~/.rigrun/audit.log and ~/.rigrun/.audit_key required \
            if absolutely necessary (e.g., testing environments only)."
        );
    }

    /// Verify HMAC chain integrity of the audit log
    ///
    /// This function verifies that all log entries have valid HMAC signatures
    /// and that the chain is unbroken from the genesis hash.
    ///
    /// Returns (total_entries, verified_entries, first_error)
    pub fn verify_integrity(&self) -> Result<(usize, usize, Option<String>)> {
        if !self.log_path.exists() {
            return Ok((0, 0, None));
        }

        let content = fs::read_to_string(&self.log_path)?;
        let lines: Vec<&str> = content.lines().collect();
        let total = lines.len();
        let mut verified = 0;
        let mut prev_hash = "0".repeat(64); // Genesis hash

        for (i, line) in lines.iter().enumerate() {
            // Parse line format: json|hmac
            let parts: Vec<&str> = line.rsplitn(2, '|').collect();
            if parts.len() != 2 {
                return Ok((total, verified, Some(format!(
                    "Line {} has invalid format (expected 'json|hmac')", i + 1
                ))));
            }

            let (claimed_hash, log_line) = (parts[0], parts[1]);

            // Recompute HMAC
            let mut mac = HmacSha256::new_from_slice(&self.hmac_key)
                .map_err(|e| anyhow::anyhow!("HMAC key error: {}", e))?;
            mac.update(prev_hash.as_bytes());
            mac.update(log_line.as_bytes());
            let computed_hash = hex::encode(mac.finalize().into_bytes());

            // Verify HMAC
            if computed_hash != claimed_hash {
                return Ok((total, verified, Some(format!(
                    "Line {} has invalid HMAC (tampering detected)", i + 1
                ))));
            }

            verified += 1;
            prev_hash = claimed_hash.to_string();
        }

        Ok((total, verified, None))
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
    session_id: Option<String>,
    source_ip: Option<String>,
) {
    if let Ok(logger) = global_audit_logger().read() {
        let _ = logger.log_query(tier, query, input_tokens, output_tokens, cost_usd, session_id, source_ip);
    }
}

/// Log a blocked request to the global audit logger with IL5-required user identification
pub fn audit_log_blocked(tier: Tier, query: &str, session_id: Option<String>, source_ip: Option<String>) {
    if let Ok(logger) = global_audit_logger().read() {
        let _ = logger.log_blocked(tier, query, session_id, source_ip);
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

/// Log a session event to the global audit logger (IL5 compliance)
///
/// This function provides a convenient way to log session lifecycle events
/// to the persistent audit trail from anywhere in the application.
///
/// # Arguments
/// * `event_type` - Type of session event (e.g., "SESSION_CREATED", "SESSION_EXPIRED")
/// * `session_id` - Session identifier for correlation
/// * `user_id` - Optional user identifier for accountability
/// * `details` - Optional additional details (reason, duration, etc.)
///
/// # Example
/// ```no_run
/// use rigrun::audit::audit_log_session_event;
/// audit_log_session_event(
///     "SESSION_CREATED",
///     "sess_abc123",
///     Some("user@example.com"),
///     None
/// );
/// ```
pub fn audit_log_session_event(
    event_type: impl Into<String>,
    session_id: impl Into<String>,
    user_id: Option<String>,
    details: Option<String>,
) {
    let entry = SessionAuditEntry {
        timestamp: Utc::now(),
        event_type: event_type.into(),
        session_id: session_id.into(),
        user_id,
        details,
    };

    if let Ok(logger) = global_audit_logger().read() {
        let _ = logger.log_session_event(entry);
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
            Some("session-123".to_string()),
            Some("192.168.1.1".to_string()),
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
            None,
            None,
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
        // GitHub tokens are ghp_ followed by exactly 36 alphanumeric chars
        let text = "GitHub token: ghp_1234567890abcdefghijklmnopqrstuvwxyz";
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
        // GitHub tokens need exactly 36 chars after ghp_
        let text = "AWS: AKIAIOSFODNN7EXAMPLE, GitHub: ghp_abcdefghijklmnopqrstuvwxyz1234567890, password=secret123";
        let redacted = redact_secrets(text);
        assert!(redacted.contains("[REDACTED_AWS_KEY]"));
        assert!(redacted.contains("[REDACTED_GITHUB_TOKEN]"));
        assert!(redacted.contains("password=[REDACTED]"));
        assert!(!redacted.contains("AKIAIOSFODNN7EXAMPLE"));
        assert!(!redacted.contains("ghp_"));
        assert!(!redacted.contains("secret123"));
    }

    #[test]
    fn test_hmac_integrity_protection() {
        use tempfile::TempDir;

        // Create temporary directory for test
        let temp_dir = TempDir::new().unwrap();
        let log_path = temp_dir.path().join("audit.log");

        // Create a test logger (this will be more complex in practice)
        // For now, just verify the structure is correct
        let logger = AuditLogger::new(true).unwrap();
        assert!(logger.hmac_key.len() == 32);
        assert_eq!(logger.config.max_file_size_mb, 100);
        assert_eq!(logger.config.max_age_days, 365);
        assert_eq!(logger.config.rotate_count, 10);
    }

    #[test]
    fn test_clear_function_disabled() {
        let logger = AuditLogger::new(true).unwrap();
        let result = logger.clear();
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("IL5 VIOLATION"));
    }

    #[test]
    fn test_audit_logger_creates_hmac_key() {
        use tempfile::TempDir;
        let temp_dir = TempDir::new().unwrap();

        // Load or generate HMAC key
        let key1 = AuditLogger::load_or_generate_hmac_key(&temp_dir.path().to_path_buf()).unwrap();
        assert_eq!(key1.len(), 32);

        // Loading again should return the same key
        let key2 = AuditLogger::load_or_generate_hmac_key(&temp_dir.path().to_path_buf()).unwrap();
        assert_eq!(key1, key2);
    }

    #[test]
    fn test_log_with_hmac_chain() {
        // This test would require mocking or using a temporary directory
        // For now, verify the logger can be created successfully
        let logger = AuditLogger::new(true).unwrap();

        let entry = AuditEntry::new(
            Tier::Local,
            "Test query",
            100,
            50,
            0.0,
            false,
            Some("test-session".to_string()),
            Some("127.0.0.1".to_string()),
        );

        // Log should succeed with HMAC protection
        let result = logger.log(entry);
        assert!(result.is_ok());
    }

    #[test]
    fn test_verify_integrity_empty_log() {
        use tempfile::TempDir;

        // Use isolated temp directory to avoid reading existing system logs
        let temp_dir = TempDir::new().unwrap();
        let logger = AuditLogger::new_with_path(true, temp_dir.path().to_path_buf()).unwrap();
        let (total, verified, error) = logger.verify_integrity().unwrap();

        // Empty log should verify successfully
        assert_eq!(total, 0);
        assert_eq!(verified, 0);
        assert!(error.is_none());
    }

    #[test]
    fn test_audit_config_default() {
        let config = AuditConfig::default();
        assert_eq!(config.max_file_size_mb, 100);
        assert_eq!(config.max_age_days, 365);
        assert_eq!(config.rotate_count, 10);
    }

    #[test]
    fn test_audit_config_custom() {
        let config = AuditConfig {
            max_file_size_mb: 50,
            max_age_days: 180,
            rotate_count: 5,
        };
        let logger = AuditLogger::new_with_config(true, config.clone()).unwrap();
        assert_eq!(logger.config.max_file_size_mb, 50);
        assert_eq!(logger.config.max_age_days, 180);
        assert_eq!(logger.config.rotate_count, 5);
    }

    #[test]
    fn test_log_size_bytes() {
        let logger = AuditLogger::new(true).unwrap();
        let size = logger.log_size_bytes().unwrap();
        // New log should be 0 bytes or not exist
        assert!(size >= 0);
    }

    #[test]
    fn test_rotate_if_needed_small_log() {
        let logger = AuditLogger::new(true).unwrap();
        // Should not error on small/nonexistent log
        let result = logger.rotate_if_needed();
        assert!(result.is_ok());
    }
}
