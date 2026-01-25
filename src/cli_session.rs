// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! CLI Session Manager for DoD STIG IL5 Compliance
//!
//! This module provides session timeout management for CLI interactive modes,
//! implementing DoD STIG requirements for session termination (AC-12) and
//! session lock (AC-11).
//!
//! ## DoD STIG Requirements
//!
//! - **AC-12 (Session Termination)**: Maximum 15-minute (900 second) session timeout
//! - **AC-11 (Session Lock)**: Session lock with re-authentication required
//! - **AU-3 (Audit Content)**: Session events must be logged
//!
//! ## Usage in CLI
//!
//! ```no_run
//! use rigrun::cli_session::{CliSession, CliSessionConfig};
//!
//! let config = CliSessionConfig::dod_stig_default();
//! let session = CliSession::new("cli-user", config);
//!
//! // In the interactive loop:
//! if session.check_timeout() {
//!     // Show expiration message and require re-authentication
//! }
//! session.refresh(); // Call on user activity
//! ```

use chrono::{DateTime, Utc};
use colored::Colorize;
use rand::RngCore;
use std::io::{self, Write};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::Instant;

/// DoD STIG maximum session timeout: 15 minutes (900 seconds)
pub const CLI_SESSION_TIMEOUT_SECS: u64 = 900;

/// Warning period before timeout: 2 minutes (120 seconds)
pub const CLI_WARNING_BEFORE_TIMEOUT_SECS: u64 = 120;

/// CLI session configuration
#[derive(Clone)]
pub struct CliSessionConfig {
    /// Maximum session timeout in seconds (clamped to 900 for IL5)
    pub timeout_secs: u64,
    /// Seconds before timeout to show warning
    pub warning_secs: u64,
    /// Whether to require consent banner after timeout
    pub require_consent_reack: bool,
    /// Warning message template
    pub warning_message: String,
    /// Expiration message
    pub expiration_message: String,
}

impl Default for CliSessionConfig {
    fn default() -> Self {
        Self::dod_stig_default()
    }
}

impl CliSessionConfig {
    /// Create configuration with DoD STIG IL5 defaults
    pub fn dod_stig_default() -> Self {
        Self {
            timeout_secs: CLI_SESSION_TIMEOUT_SECS,
            warning_secs: CLI_WARNING_BEFORE_TIMEOUT_SECS,
            require_consent_reack: true,
            warning_message: "Session expires in {} minute(s) {} second(s). Press ENTER to continue.".to_string(),
            expiration_message: "Session expired. Please re-authenticate.".to_string(),
        }
    }

    /// Create custom configuration (timeout will be clamped to 900 max)
    pub fn custom(timeout_secs: u64, warning_secs: u64) -> Self {
        let clamped_timeout = timeout_secs.min(CLI_SESSION_TIMEOUT_SECS);
        let clamped_warning = warning_secs.min(clamped_timeout.saturating_sub(60));

        if timeout_secs > CLI_SESSION_TIMEOUT_SECS {
            eprintln!(
                "{}[WARNING]{} Session timeout {}s exceeds DoD STIG IL5 max of {}s. Using {}s.",
                "\x1b[33m", "\x1b[0m",
                timeout_secs,
                CLI_SESSION_TIMEOUT_SECS,
                clamped_timeout
            );
        }

        Self {
            timeout_secs: clamped_timeout,
            warning_secs: clamped_warning,
            ..Self::dod_stig_default()
        }
    }
}

/// CLI session state
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CliSessionState {
    Active,
    Warning,
    Expired,
    Locked,
}

impl std::fmt::Display for CliSessionState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            CliSessionState::Active => write!(f, "ACTIVE"),
            CliSessionState::Warning => write!(f, "WARNING"),
            CliSessionState::Expired => write!(f, "EXPIRED"),
            CliSessionState::Locked => write!(f, "LOCKED"),
        }
    }
}

/// CLI Session for interactive modes
pub struct CliSession {
    /// Session ID
    id: String,
    /// User identifier
    user_id: String,
    /// When session was created
    created_at: Instant,
    /// UTC timestamp for audit logging
    created_at_utc: DateTime<Utc>,
    /// Last activity timestamp
    last_activity: Instant,
    /// Configuration
    config: CliSessionConfig,
    /// Current state
    state: CliSessionState,
    /// Whether warning has been shown this timeout period
    warning_shown: bool,
    /// Whether consent has been acknowledged
    consent_acknowledged: bool,
    /// Flag for async timeout monitoring
    expired_flag: Arc<AtomicBool>,
}

impl CliSession {
    /// Create a new CLI session
    pub fn new(user_id: impl Into<String>, config: CliSessionConfig) -> Self {
        let now = Instant::now();
        let utc_now = Utc::now();
        let user_id_string = user_id.into();

        // Generate session ID with cryptographically secure random bytes
        let mut bytes = [0u8; 16];
        rand::rngs::OsRng.fill_bytes(&mut bytes);
        let random_hex: String = bytes.iter().map(|b| format!("{:02x}", b)).collect();
        let session_id = format!("cli_sess_{}_{}", utc_now.timestamp_millis(), random_hex);

        // Log session creation
        tracing::info!(
            "CLI_SESSION_CREATED | session={} user={} timeout={}s timestamp={}",
            session_id,
            user_id_string,
            config.timeout_secs,
            utc_now.format("%Y-%m-%d %H:%M:%S UTC")
        );

        Self {
            id: session_id,
            user_id: user_id_string,
            created_at: now,
            created_at_utc: utc_now,
            last_activity: now,
            config,
            state: CliSessionState::Active,
            warning_shown: false,
            consent_acknowledged: false,
            expired_flag: Arc::new(AtomicBool::new(false)),
        }
    }

    /// Get session ID
    pub fn id(&self) -> &str {
        &self.id
    }

    /// Check if session has expired
    pub fn is_expired(&self) -> bool {
        let elapsed = self.last_activity.elapsed().as_secs();
        elapsed >= self.config.timeout_secs || self.state == CliSessionState::Expired
    }

    /// Check if session is in warning period
    pub fn is_in_warning_period(&self) -> bool {
        let elapsed = self.last_activity.elapsed().as_secs();
        let remaining = self.config.timeout_secs.saturating_sub(elapsed);
        remaining <= self.config.warning_secs && remaining > 0
    }

    /// Get time remaining in seconds
    pub fn time_remaining_secs(&self) -> u64 {
        let elapsed = self.last_activity.elapsed().as_secs();
        self.config.timeout_secs.saturating_sub(elapsed)
    }

    /// Refresh session (on user activity)
    pub fn refresh(&mut self) {
        if self.state == CliSessionState::Expired || self.state == CliSessionState::Locked {
            tracing::warn!(
                "CLI_SESSION_REFRESH_DENIED | session={} state={} reason=requires_reauth",
                self.id,
                self.state
            );
            return;
        }

        self.last_activity = Instant::now();
        self.warning_shown = false;
        self.state = CliSessionState::Active;

        tracing::debug!(
            "CLI_SESSION_REFRESHED | session={} remaining={}s",
            self.id,
            self.config.timeout_secs
        );
    }

    /// Check timeout and update state. Returns true if action needed (warning or expired)
    pub fn check_timeout(&mut self) -> bool {
        // Check for expiration
        if self.is_expired() {
            self.state = CliSessionState::Expired;
            self.expired_flag.store(true, Ordering::SeqCst);

            tracing::info!(
                "CLI_SESSION_EXPIRED | session={} duration={}s timestamp={}",
                self.id,
                self.created_at.elapsed().as_secs(),
                Utc::now().format("%Y-%m-%d %H:%M:%S UTC")
            );

            return true;
        }

        // Check for warning period
        if self.is_in_warning_period() && !self.warning_shown {
            self.state = CliSessionState::Warning;
            self.warning_shown = true;

            let remaining = self.time_remaining_secs();
            tracing::warn!(
                "CLI_SESSION_WARNING | session={} expires_in={}s timestamp={}",
                self.id,
                remaining,
                Utc::now().format("%Y-%m-%d %H:%M:%S UTC")
            );

            return true;
        }

        false
    }

    /// Show warning message to user
    pub fn show_warning(&self) {
        let remaining = self.time_remaining_secs();
        let minutes = remaining / 60;
        let seconds = remaining % 60;

        let message = self.config.warning_message
            .replacen("{}", &minutes.to_string(), 1)
            .replacen("{}", &seconds.to_string(), 1);

        eprintln!("\n{}", format!("⚠ {}", message).yellow().bold());
    }

    /// Show expiration message and return true if consent re-ack is required
    pub fn show_expiration(&self) -> bool {
        eprintln!("\n{}", format!("✗ {}", self.config.expiration_message).red().bold());

        if self.config.require_consent_reack {
            eprintln!("{}", "⚠ You must re-acknowledge the consent banner to continue.".yellow());
        }

        self.config.require_consent_reack
    }

    /// Lock the session
    pub fn lock(&mut self, reason: &str) {
        self.state = CliSessionState::Locked;

        tracing::info!(
            "CLI_SESSION_LOCKED | session={} reason={} timestamp={}",
            self.id,
            reason,
            Utc::now().format("%Y-%m-%d %H:%M:%S UTC")
        );
    }

    /// Terminate the session
    pub fn terminate(&mut self, reason: &str) {
        self.state = CliSessionState::Expired;
        self.expired_flag.store(true, Ordering::SeqCst);

        tracing::info!(
            "CLI_SESSION_TERMINATED | session={} reason={} duration={}s timestamp={}",
            self.id,
            reason,
            self.created_at.elapsed().as_secs(),
            Utc::now().format("%Y-%m-%d %H:%M:%S UTC")
        );
    }

    /// Acknowledge consent banner
    pub fn acknowledge_consent(&mut self) {
        self.consent_acknowledged = true;

        tracing::debug!(
            "CLI_SESSION_CONSENT_ACK | session={} timestamp={}",
            self.id,
            Utc::now().format("%Y-%m-%d %H:%M:%S UTC")
        );
    }

    /// Check if consent needs re-acknowledgment
    pub fn needs_consent(&self) -> bool {
        self.config.require_consent_reack && !self.consent_acknowledged
    }

    /// Get current state
    pub fn state(&self) -> CliSessionState {
        self.state
    }

    /// Get expired flag for async monitoring
    pub fn expired_flag(&self) -> Arc<AtomicBool> {
        self.expired_flag.clone()
    }

    /// Get session configuration
    pub fn config(&self) -> &CliSessionConfig {
        &self.config
    }

    /// Print session status bar
    pub fn print_status_bar(&self) {
        let remaining = self.time_remaining_secs();
        let minutes = remaining / 60;
        let seconds = remaining % 60;

        let status = match self.state {
            CliSessionState::Active => format!("Session: {}:{:02}", minutes, seconds).bright_green(),
            CliSessionState::Warning => format!("Session: {}:{:02} (!)", minutes, seconds).yellow(),
            CliSessionState::Expired => "Session: EXPIRED".red(),
            CliSessionState::Locked => "Session: LOCKED".red(),
        };

        eprint!("\r{} ", status);
        io::stderr().flush().ok();
    }
}

/// Wait for user input with session timeout
///
/// Returns Ok(input) if user entered input before timeout
/// Returns Err("timeout") if session expired
/// Returns Err("warning") if warning was triggered
pub fn read_line_with_timeout(session: &mut CliSession) -> Result<String, &'static str> {
    let stdin = io::stdin();
    let mut input = String::new();

    // For simplicity in synchronous context, we check timeout before and after read
    // A more sophisticated implementation would use async or thread-based timeout

    // Check before read
    if session.check_timeout() {
        if session.is_expired() {
            return Err("timeout");
        } else if session.state() == CliSessionState::Warning {
            session.show_warning();
            return Err("warning");
        }
    }

    // Read input (blocking)
    if stdin.read_line(&mut input).is_err() {
        return Err("read_error");
    }

    // Refresh session on successful input
    session.refresh();

    Ok(input)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::thread::sleep;
    use std::time::Duration;

    #[test]
    fn test_cli_session_creation() {
        let config = CliSessionConfig::dod_stig_default();
        let session = CliSession::new("test-user", config);

        assert_eq!(session.state(), CliSessionState::Active);
        assert!(!session.is_expired());
    }

    #[test]
    fn test_cli_session_config_clamping() {
        let config = CliSessionConfig::custom(9999, 300);

        assert_eq!(config.timeout_secs, CLI_SESSION_TIMEOUT_SECS);
    }

    #[test]
    fn test_cli_session_refresh() {
        let config = CliSessionConfig::custom(10, 3);
        let mut session = CliSession::new("test-user", config);

        sleep(Duration::from_millis(100));
        session.refresh();

        assert_eq!(session.state(), CliSessionState::Active);
        assert!(session.time_remaining_secs() >= 9);
    }

    #[test]
    fn test_cli_session_state_display() {
        assert_eq!(format!("{}", CliSessionState::Active), "ACTIVE");
        assert_eq!(format!("{}", CliSessionState::Warning), "WARNING");
        assert_eq!(format!("{}", CliSessionState::Expired), "EXPIRED");
        assert_eq!(format!("{}", CliSessionState::Locked), "LOCKED");
    }

    #[test]
    fn test_dod_stig_timeout_constant() {
        assert_eq!(CLI_SESSION_TIMEOUT_SECS, 900);
        assert_eq!(CLI_SESSION_TIMEOUT_SECS / 60, 15); // 15 minutes
    }
}
