// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

//! DoD Consent Banner Module
//!
//! Implements the legally required DoD consent banner for IL5 compliance.
//! This banner MUST be displayed and acknowledged before system use.
//!
//! # Requirements
//! - Display EXACT DoD consent banner text
//! - User MUST press Enter to acknowledge
//! - Log acknowledgment with timestamp for audit trail
//! - Support --skip-banner flag for automated/CI environments
//! - Configurable enable/disable for non-DoD deployments

use anyhow::Result;
use chrono::{DateTime, Local, Utc};
use std::fs::OpenOptions;
use std::io::{self, BufRead, Write};
use std::path::PathBuf;

/// The exact DoD consent banner text - this is legally required and MUST NOT be modified
pub const DOD_CONSENT_BANNER_TEXT: &str = r#"You are accessing a U.S. Government (USG) Information System (IS) that is provided for USG-authorized use only.

By using this IS (which includes any device attached to this IS), you consent to the following conditions:
- The USG routinely intercepts and monitors communications on this IS for purposes including, but not limited to, penetration testing, COMSEC monitoring, network operations and defense, personnel misconduct (PM), law enforcement (LE), and counterintelligence (CI) investigations.
- At any time, the USG may inspect and seize data stored on this IS.
- Communications using, or data stored on, this IS are not private, are subject to routine monitoring, interception, and search, and may be disclosed or used for any USG-authorized purpose.
- This IS includes security measures (e.g., authentication and access controls) to protect USG interests--not for your personal benefit or privacy.
- Notwithstanding the above, using this IS does not constitute consent to PM, LE or CI investigative searching or monitoring of the content of privileged communications, or work product, related to personal representation or services by attorneys, psychotherapists, or clergy, and their assistants."#;

/// Consent banner acknowledgment log entry
#[derive(Debug, Clone)]
pub struct ConsentAcknowledgment {
    /// Timestamp when consent was acknowledged
    pub timestamp: DateTime<Utc>,
    /// Whether the banner was skipped (--skip-banner flag)
    pub skipped: bool,
    /// Reason for skipping (if skipped)
    pub skip_reason: Option<String>,
    /// Username/identifier if available
    pub user: Option<String>,
    /// Hostname
    pub hostname: Option<String>,
}

impl ConsentAcknowledgment {
    /// Create a new acknowledgment entry for interactive consent
    pub fn acknowledged() -> Self {
        Self {
            timestamp: Utc::now(),
            skipped: false,
            skip_reason: None,
            user: get_username(),
            hostname: get_hostname(),
        }
    }

    /// Create a new acknowledgment entry for skipped consent
    pub fn skipped(reason: &str) -> Self {
        Self {
            timestamp: Utc::now(),
            skipped: true,
            skip_reason: Some(reason.to_string()),
            user: get_username(),
            hostname: get_hostname(),
        }
    }

    /// Format as a log line for the consent audit log
    pub fn to_log_line(&self) -> String {
        let local_time: DateTime<Local> = self.timestamp.into();
        let status = if self.skipped {
            format!("SKIPPED ({})", self.skip_reason.as_deref().unwrap_or("unknown"))
        } else {
            "ACKNOWLEDGED".to_string()
        };
        let user = self.user.as_deref().unwrap_or("unknown");
        let hostname = self.hostname.as_deref().unwrap_or("unknown");

        format!(
            "{} | CONSENT_BANNER | {} | user={} | host={}",
            local_time.format("%Y-%m-%d %H:%M:%S"),
            status,
            user,
            hostname
        )
    }
}

/// Get the current username
fn get_username() -> Option<String> {
    std::env::var("USER")
        .or_else(|_| std::env::var("USERNAME"))
        .ok()
}

/// Get the current hostname
fn get_hostname() -> Option<String> {
    hostname::get()
        .ok()
        .and_then(|h| h.into_string().ok())
}

/// Get the consent log file path
pub fn consent_log_path() -> PathBuf {
    dirs::home_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join(".rigrun")
        .join("consent.log")
}

/// Log a consent acknowledgment to the audit log
pub fn log_consent(ack: &ConsentAcknowledgment) -> Result<()> {
    let log_dir = dirs::home_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join(".rigrun");

    std::fs::create_dir_all(&log_dir)?;

    let log_path = log_dir.join("consent.log");
    let log_line = ack.to_log_line();

    let mut file = OpenOptions::new()
        .create(true)
        .append(true)
        .open(&log_path)?;

    writeln!(file, "{}", log_line)?;

    Ok(())
}

/// Display the DoD consent banner and wait for acknowledgment
///
/// Returns Ok(()) if the user acknowledges the banner by pressing Enter.
/// Returns Err if the user cancels (Ctrl+C) or there's an I/O error.
///
/// # Arguments
/// * `skip` - If true, skip the interactive prompt (for CI/automated environments)
/// * `skip_reason` - Reason for skipping (e.g., "--skip-banner flag", "CI environment")
pub fn display_and_acknowledge(skip: bool, skip_reason: Option<&str>) -> Result<()> {
    if skip {
        // Log that banner was skipped
        let reason = skip_reason.unwrap_or("--skip-banner flag");
        let ack = ConsentAcknowledgment::skipped(reason);

        // Always log skipped acknowledgments for audit trail
        if let Err(e) = log_consent(&ack) {
            eprintln!("Warning: Failed to log consent skip: {}", e);
        }

        // Print a warning that the banner was skipped
        eprintln!("[AUDIT] DoD consent banner skipped: {}", reason);
        eprintln!("[AUDIT] Timestamp: {}", ack.timestamp);

        return Ok(());
    }

    // Display the banner with visual separation
    println!();
    println!("================================================================================");
    println!();
    println!("{}", DOD_CONSENT_BANNER_TEXT);
    println!();
    println!("[Press ENTER to acknowledge and continue]");
    println!("================================================================================");

    // Wait for user to press Enter
    let stdin = io::stdin();
    let mut input = String::new();

    // Flush stdout to ensure the banner is displayed
    io::stdout().flush()?;

    // Read a line (waits for Enter)
    stdin.lock().read_line(&mut input)?;

    // Log the acknowledgment
    let ack = ConsentAcknowledgment::acknowledged();
    if let Err(e) = log_consent(&ack) {
        eprintln!("Warning: Failed to log consent acknowledgment: {}", e);
    }

    println!();
    println!("[ACKNOWLEDGED] Consent recorded at {}", ack.timestamp);
    println!();

    Ok(())
}

/// Check if running in a CI/automated environment
pub fn is_ci_environment() -> bool {
    // Check common CI environment variables
    std::env::var("CI").is_ok()
        || std::env::var("CONTINUOUS_INTEGRATION").is_ok()
        || std::env::var("GITHUB_ACTIONS").is_ok()
        || std::env::var("GITLAB_CI").is_ok()
        || std::env::var("JENKINS_URL").is_ok()
        || std::env::var("TRAVIS").is_ok()
        || std::env::var("CIRCLECI").is_ok()
        || std::env::var("BUILDKITE").is_ok()
        || std::env::var("TF_BUILD").is_ok()  // Azure DevOps
        || std::env::var("TEAMCITY_VERSION").is_ok()
}

/// Check if stdin is a TTY (interactive terminal)
pub fn is_interactive() -> bool {
    use std::io::IsTerminal;
    io::stdin().is_terminal()
}

/// Determine if the consent banner should be displayed
///
/// Returns (should_display, skip_reason) tuple.
/// The banner should be displayed unless:
/// - `skip_banner` flag is true
/// - `dod_banner_enabled` config is false
/// - Running in non-interactive (piped) environment
pub fn should_display_banner(
    skip_banner_flag: bool,
    dod_banner_enabled: bool,
) -> (bool, Option<&'static str>) {
    // Config explicitly disables the banner (non-DoD deployment)
    if !dod_banner_enabled {
        return (false, Some("dod_banner_enabled=false in config"));
    }

    // Skip flag provided
    if skip_banner_flag {
        return (false, Some("--skip-banner flag"));
    }

    // Non-interactive environment (piped input)
    if !is_interactive() {
        return (false, Some("non-interactive environment (stdin not a TTY)"));
    }

    // CI environment - still show but log it
    if is_ci_environment() {
        // In CI, we might want to skip but should log it
        // For now, return true to require explicit --skip-banner in CI
        return (true, None);
    }

    // Default: display the banner
    (true, None)
}

/// Main entry point for consent banner handling
///
/// Call this at application startup before any other operations.
///
/// # Arguments
/// * `skip_banner` - The --skip-banner CLI flag value
/// * `dod_banner_enabled` - The config setting for DoD banner
///
/// # Returns
/// Ok(()) if consent was acknowledged or properly skipped with logging
/// Err if there was an I/O error or user cancelled
pub fn handle_consent_banner(skip_banner: bool, dod_banner_enabled: bool) -> Result<()> {
    let (should_display, skip_reason) = should_display_banner(skip_banner, dod_banner_enabled);

    if should_display {
        display_and_acknowledge(false, None)
    } else {
        display_and_acknowledge(true, skip_reason)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_consent_acknowledgment_acknowledged() {
        let ack = ConsentAcknowledgment::acknowledged();
        assert!(!ack.skipped);
        assert!(ack.skip_reason.is_none());
        let log_line = ack.to_log_line();
        assert!(log_line.contains("ACKNOWLEDGED"));
        assert!(log_line.contains("CONSENT_BANNER"));
    }

    #[test]
    fn test_consent_acknowledgment_skipped() {
        let ack = ConsentAcknowledgment::skipped("--skip-banner flag");
        assert!(ack.skipped);
        assert_eq!(ack.skip_reason, Some("--skip-banner flag".to_string()));
        let log_line = ack.to_log_line();
        assert!(log_line.contains("SKIPPED"));
        assert!(log_line.contains("--skip-banner flag"));
    }

    #[test]
    fn test_should_display_banner_config_disabled() {
        let (should_display, reason) = should_display_banner(false, false);
        assert!(!should_display);
        assert!(reason.is_some());
        assert!(reason.unwrap().contains("dod_banner_enabled"));
    }

    #[test]
    fn test_should_display_banner_skip_flag() {
        let (should_display, reason) = should_display_banner(true, true);
        assert!(!should_display);
        assert!(reason.is_some());
        assert!(reason.unwrap().contains("--skip-banner"));
    }

    #[test]
    fn test_banner_text_contains_required_elements() {
        // Verify the banner contains key legally required phrases
        assert!(DOD_CONSENT_BANNER_TEXT.contains("U.S. Government"));
        assert!(DOD_CONSENT_BANNER_TEXT.contains("USG-authorized use only"));
        assert!(DOD_CONSENT_BANNER_TEXT.contains("consent to the following conditions"));
        assert!(DOD_CONSENT_BANNER_TEXT.contains("intercepts and monitors"));
        assert!(DOD_CONSENT_BANNER_TEXT.contains("may inspect and seize data"));
        assert!(DOD_CONSENT_BANNER_TEXT.contains("not private"));
        assert!(DOD_CONSENT_BANNER_TEXT.contains("security measures"));
        assert!(DOD_CONSENT_BANNER_TEXT.contains("privileged communications"));
    }
}
