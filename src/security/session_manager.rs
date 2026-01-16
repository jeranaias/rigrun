// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

//! Session Manager for DoD STIG Compliance
//!
//! Implements session timeout per DoD STIG requirements for IL5 environments.
//!
//! ## DoD STIG Requirements
//!
//! - **AC-12 (Session Termination)**: Sessions MUST terminate after 15 minutes (900 seconds)
//!   of inactivity or absolute timeout.
//! - **AC-11 (Session Lock)**: Session lock with re-authentication required.
//! - **AU-3 (Audit Content)**: Session events must be logged.
//!
//! ## Configuration
//!
//! The maximum session timeout is **15 minutes (900 seconds)** for IL5.
//! This is a HARD LIMIT that cannot be exceeded, only reduced.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};

/// DoD STIG maximum session timeout: 15 minutes (900 seconds)
/// This is the MAXIMUM allowed for IL5 environments - cannot be exceeded.
pub const DOD_STIG_MAX_SESSION_TIMEOUT_SECS: u64 = 900;

/// Default warning time before timeout: 2 minutes (120 seconds)
pub const DOD_STIG_WARNING_BEFORE_TIMEOUT_SECS: u64 = 120;

/// Session state enumeration
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum SessionState {
    /// Session is active and valid
    Active,
    /// Session is in warning period (about to expire)
    Warning,
    /// Session has been locked due to timeout
    Locked,
    /// Session has expired and requires re-authentication
    Expired,
    /// Session was explicitly terminated
    Terminated,
}

impl SessionState {
    /// Returns true if the session allows activity
    pub fn is_active(&self) -> bool {
        matches!(self, SessionState::Active | SessionState::Warning)
    }

    /// Returns true if re-authentication is required
    pub fn requires_reauth(&self) -> bool {
        matches!(self, SessionState::Locked | SessionState::Expired | SessionState::Terminated)
    }
}

impl std::fmt::Display for SessionState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            SessionState::Active => write!(f, "ACTIVE"),
            SessionState::Warning => write!(f, "WARNING"),
            SessionState::Locked => write!(f, "LOCKED"),
            SessionState::Expired => write!(f, "EXPIRED"),
            SessionState::Terminated => write!(f, "TERMINATED"),
        }
    }
}

/// Session events for audit logging
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum SessionEvent {
    /// Session was created
    Created {
        session_id: String,
        user_id: String,
        timestamp: DateTime<Utc>,
    },
    /// Session activity was refreshed
    Refreshed {
        session_id: String,
        timestamp: DateTime<Utc>,
        time_remaining_secs: u64,
    },
    /// Session entered warning period
    WarningIssued {
        session_id: String,
        timestamp: DateTime<Utc>,
        expires_in_secs: u64,
    },
    /// Session was locked
    Locked {
        session_id: String,
        timestamp: DateTime<Utc>,
        reason: String,
    },
    /// Session expired
    Expired {
        session_id: String,
        timestamp: DateTime<Utc>,
        session_duration_secs: u64,
    },
    /// Session was explicitly terminated
    Terminated {
        session_id: String,
        timestamp: DateTime<Utc>,
        reason: String,
    },
    /// Re-authentication was required
    ReauthRequired {
        session_id: String,
        timestamp: DateTime<Utc>,
    },
}

impl SessionEvent {
    /// Format event for audit log
    pub fn to_audit_string(&self) -> String {
        let timestamp = Utc::now().format("%Y-%m-%d %H:%M:%S UTC");
        match self {
            SessionEvent::Created { session_id, user_id, .. } => {
                format!("{} | SESSION_CREATED | session={} user={}", timestamp, session_id, user_id)
            }
            SessionEvent::Refreshed { session_id, time_remaining_secs, .. } => {
                format!("{} | SESSION_REFRESHED | session={} remaining={}s", timestamp, session_id, time_remaining_secs)
            }
            SessionEvent::WarningIssued { session_id, expires_in_secs, .. } => {
                format!("{} | SESSION_WARNING | session={} expires_in={}s", timestamp, session_id, expires_in_secs)
            }
            SessionEvent::Locked { session_id, reason, .. } => {
                format!("{} | SESSION_LOCKED | session={} reason={}", timestamp, session_id, reason)
            }
            SessionEvent::Expired { session_id, session_duration_secs, .. } => {
                format!("{} | SESSION_EXPIRED | session={} duration={}s", timestamp, session_id, session_duration_secs)
            }
            SessionEvent::Terminated { session_id, reason, .. } => {
                format!("{} | SESSION_TERMINATED | session={} reason={}", timestamp, session_id, reason)
            }
            SessionEvent::ReauthRequired { session_id, .. } => {
                format!("{} | REAUTH_REQUIRED | session={}", timestamp, session_id)
            }
        }
    }
}

/// Session configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SessionConfig {
    /// Maximum session timeout in seconds (HARD LIMIT: 900 for IL5)
    /// Any value greater than 900 will be clamped to 900.
    pub max_timeout_secs: u64,

    /// Seconds before timeout to issue warning
    pub warning_before_timeout_secs: u64,

    /// Whether to require consent banner re-acknowledgment after timeout
    pub require_consent_reack: bool,

    /// Session lock message
    pub lock_message: String,

    /// Expiration message
    pub expiration_message: String,

    /// Warning message template ({minutes} and {seconds} will be replaced)
    pub warning_message_template: String,
}

impl Default for SessionConfig {
    fn default() -> Self {
        Self::dod_stig_default()
    }
}

impl SessionConfig {
    /// Create configuration with DoD STIG IL5 defaults
    ///
    /// - 15-minute maximum session timeout
    /// - 2-minute warning before timeout
    /// - Consent banner re-acknowledgment required
    pub fn dod_stig_default() -> Self {
        Self {
            max_timeout_secs: DOD_STIG_MAX_SESSION_TIMEOUT_SECS,
            warning_before_timeout_secs: DOD_STIG_WARNING_BEFORE_TIMEOUT_SECS,
            require_consent_reack: true,
            lock_message: "Session locked due to inactivity. Press any key to re-authenticate.".to_string(),
            expiration_message: "Session expired. Please re-authenticate.".to_string(),
            warning_message_template: "Session expires in {minutes} minute(s) {seconds} second(s). Press any key to continue.".to_string(),
        }
    }

    /// Create a custom configuration with validation
    ///
    /// # Arguments
    /// * `timeout_secs` - Desired timeout (will be clamped to max 900 for IL5)
    /// * `warning_secs` - Seconds before timeout to warn
    ///
    /// # Returns
    /// SessionConfig with validated values
    pub fn custom(timeout_secs: u64, warning_secs: u64) -> Self {
        // HARD LIMIT: Cannot exceed DoD STIG maximum
        let clamped_timeout = timeout_secs.min(DOD_STIG_MAX_SESSION_TIMEOUT_SECS);

        // Warning should be less than timeout
        let clamped_warning = warning_secs.min(clamped_timeout.saturating_sub(60));

        if timeout_secs > DOD_STIG_MAX_SESSION_TIMEOUT_SECS {
            tracing::warn!(
                "SESSION_TIMEOUT: Requested timeout {}s exceeds DoD STIG maximum of {}s. Clamped to {}s.",
                timeout_secs,
                DOD_STIG_MAX_SESSION_TIMEOUT_SECS,
                clamped_timeout
            );
        }

        Self {
            max_timeout_secs: clamped_timeout,
            warning_before_timeout_secs: clamped_warning,
            ..Self::dod_stig_default()
        }
    }
}

/// Session instance representing an authenticated user session
#[derive(Debug, Clone)]
pub struct Session {
    /// Unique session identifier
    pub id: String,

    /// User identifier (for audit logging)
    pub user_id: String,

    /// When the session was created
    pub created_at: Instant,

    /// Timestamp of session creation (for audit)
    pub created_at_utc: DateTime<Utc>,

    /// Last activity timestamp
    pub last_activity: Instant,

    /// Session configuration
    config: SessionConfig,

    /// Current session state
    state: SessionState,

    /// Whether consent banner has been acknowledged
    pub consent_acknowledged: bool,

    /// Whether warning has been issued for this timeout period
    warning_issued: bool,
}

impl Session {
    /// Create a new session
    pub fn new(id: impl Into<String>, user_id: impl Into<String>, config: SessionConfig) -> Self {
        let now = Instant::now();
        let session = Self {
            id: id.into(),
            user_id: user_id.into(),
            created_at: now,
            created_at_utc: Utc::now(),
            last_activity: now,
            config,
            state: SessionState::Active,
            consent_acknowledged: false,
            warning_issued: false,
        };

        // Log session creation
        let event = SessionEvent::Created {
            session_id: session.id.clone(),
            user_id: session.user_id.clone(),
            timestamp: session.created_at_utc,
        };
        tracing::info!("{}", event.to_audit_string());

        session
    }

    /// Check if the session has expired
    ///
    /// A session expires when:
    /// 1. Inactivity timeout exceeded (time since last_activity > max_timeout)
    /// 2. Session is in Expired, Locked, or Terminated state
    pub fn is_expired(&self) -> bool {
        if self.state.requires_reauth() {
            return true;
        }

        let elapsed = self.last_activity.elapsed();
        elapsed.as_secs() >= self.config.max_timeout_secs
    }

    /// Check if the session is in warning period
    pub fn is_in_warning_period(&self) -> bool {
        if self.state.requires_reauth() {
            return false;
        }

        let elapsed = self.last_activity.elapsed();
        let remaining = self.config.max_timeout_secs.saturating_sub(elapsed.as_secs());
        remaining <= self.config.warning_before_timeout_secs && remaining > 0
    }

    /// Get time remaining until session expires (in seconds)
    pub fn time_remaining_secs(&self) -> u64 {
        if self.state.requires_reauth() {
            return 0;
        }

        let elapsed = self.last_activity.elapsed();
        self.config.max_timeout_secs.saturating_sub(elapsed.as_secs())
    }

    /// Get session duration since creation (in seconds)
    pub fn session_duration_secs(&self) -> u64 {
        self.created_at.elapsed().as_secs()
    }

    /// Get inactivity duration since last activity (in seconds)
    pub fn inactivity_duration_secs(&self) -> u64 {
        self.last_activity.elapsed().as_secs()
    }

    /// Refresh session activity timestamp
    ///
    /// Returns the new session state after refresh.
    /// If session has already expired, returns Expired state.
    pub fn refresh(&mut self) -> SessionState {
        // Cannot refresh an expired/locked/terminated session
        if self.state.requires_reauth() {
            let event = SessionEvent::ReauthRequired {
                session_id: self.id.clone(),
                timestamp: Utc::now(),
            };
            tracing::warn!("{}", event.to_audit_string());
            return self.state;
        }

        // Check if session has timed out
        if self.is_expired() {
            self.state = SessionState::Expired;
            let event = SessionEvent::Expired {
                session_id: self.id.clone(),
                timestamp: Utc::now(),
                session_duration_secs: self.session_duration_secs(),
            };
            tracing::info!("{}", event.to_audit_string());
            return SessionState::Expired;
        }

        // Refresh the activity timestamp
        self.last_activity = Instant::now();
        self.warning_issued = false;

        let remaining = self.time_remaining_secs();
        let event = SessionEvent::Refreshed {
            session_id: self.id.clone(),
            timestamp: Utc::now(),
            time_remaining_secs: remaining,
        };
        tracing::debug!("{}", event.to_audit_string());

        // Update state
        self.state = SessionState::Active;
        SessionState::Active
    }

    /// Update session state based on current time
    ///
    /// This should be called periodically to check session status.
    /// Returns the current state and optionally a warning message.
    pub fn update_state(&mut self) -> (SessionState, Option<String>) {
        // Already in terminal state
        if self.state.requires_reauth() {
            return (self.state, None);
        }

        // Check for expiration
        if self.is_expired() {
            self.state = SessionState::Expired;
            let event = SessionEvent::Expired {
                session_id: self.id.clone(),
                timestamp: Utc::now(),
                session_duration_secs: self.session_duration_secs(),
            };
            tracing::info!("{}", event.to_audit_string());
            return (SessionState::Expired, Some(self.config.expiration_message.clone()));
        }

        // Check for warning period
        if self.is_in_warning_period() {
            self.state = SessionState::Warning;
            if !self.warning_issued {
                self.warning_issued = true;
                let remaining = self.time_remaining_secs();
                let minutes = remaining / 60;
                let seconds = remaining % 60;

                let event = SessionEvent::WarningIssued {
                    session_id: self.id.clone(),
                    timestamp: Utc::now(),
                    expires_in_secs: remaining,
                };
                tracing::warn!("{}", event.to_audit_string());

                let message = self.config.warning_message_template
                    .replace("{minutes}", &minutes.to_string())
                    .replace("{seconds}", &seconds.to_string());
                return (SessionState::Warning, Some(message));
            }
            return (SessionState::Warning, None);
        }

        // Session is active
        self.state = SessionState::Active;
        (SessionState::Active, None)
    }

    /// Lock the session (manual lock or due to inactivity)
    pub fn lock(&mut self, reason: &str) {
        self.state = SessionState::Locked;
        let event = SessionEvent::Locked {
            session_id: self.id.clone(),
            timestamp: Utc::now(),
            reason: reason.to_string(),
        };
        tracing::info!("{}", event.to_audit_string());
    }

    /// Terminate the session
    pub fn terminate(&mut self, reason: &str) {
        self.state = SessionState::Terminated;
        let event = SessionEvent::Terminated {
            session_id: self.id.clone(),
            timestamp: Utc::now(),
            reason: reason.to_string(),
        };
        tracing::info!("{}", event.to_audit_string());
    }

    /// Acknowledge consent banner
    pub fn acknowledge_consent(&mut self) {
        self.consent_acknowledged = true;
        tracing::debug!(
            "SESSION_CONSENT_ACKNOWLEDGED | session={} timestamp={}",
            self.id,
            Utc::now().format("%Y-%m-%d %H:%M:%S UTC")
        );
    }

    /// Check if consent needs to be re-acknowledged
    pub fn needs_consent_reack(&self) -> bool {
        self.config.require_consent_reack && !self.consent_acknowledged
    }

    /// Get current session state
    pub fn state(&self) -> SessionState {
        self.state
    }

    /// Get session configuration
    pub fn config(&self) -> &SessionConfig {
        &self.config
    }
}

/// Session manager for handling multiple sessions
pub struct SessionManager {
    /// Active sessions indexed by session ID
    sessions: Arc<RwLock<HashMap<String, Session>>>,

    /// Default session configuration
    config: SessionConfig,

    /// Session ID counter for generating unique IDs
    id_counter: Arc<std::sync::atomic::AtomicU64>,
}

impl SessionManager {
    /// Create a new session manager with the given configuration
    pub fn new(config: SessionConfig) -> Self {
        Self {
            sessions: Arc::new(RwLock::new(HashMap::new())),
            config,
            id_counter: Arc::new(std::sync::atomic::AtomicU64::new(1)),
        }
    }

    /// Create a session manager with DoD STIG IL5 defaults
    pub fn dod_stig_default() -> Self {
        Self::new(SessionConfig::dod_stig_default())
    }

    /// Generate a unique session ID
    fn generate_session_id(&self) -> String {
        use std::sync::atomic::Ordering;

        let counter = self.id_counter.fetch_add(1, Ordering::SeqCst);
        let timestamp = Utc::now().timestamp_millis();

        // Generate a random component
        let random: u32 = rand::random();

        format!("sess_{}_{}_{:08x}", timestamp, counter, random)
    }

    /// Create a new session for a user
    pub fn create_session(&self, user_id: impl Into<String>) -> Session {
        let session_id = self.generate_session_id();
        let session = Session::new(session_id.clone(), user_id, self.config.clone());

        if let Ok(mut sessions) = self.sessions.write() {
            sessions.insert(session_id.clone(), session.clone());
        }

        session
    }

    /// Get a session by ID
    pub fn get_session(&self, session_id: &str) -> Option<Session> {
        if let Ok(sessions) = self.sessions.read() {
            sessions.get(session_id).cloned()
        } else {
            None
        }
    }

    /// Validate and refresh a session
    ///
    /// Returns (is_valid, state, optional_message)
    pub fn validate_session(&self, session_id: &str) -> (bool, SessionState, Option<String>) {
        if let Ok(mut sessions) = self.sessions.write() {
            if let Some(session) = sessions.get_mut(session_id) {
                let (state, message) = session.update_state();
                let is_valid = state.is_active();
                return (is_valid, state, message);
            }
        }
        (false, SessionState::Expired, Some("Session not found".to_string()))
    }

    /// Refresh session activity
    pub fn refresh_session(&self, session_id: &str) -> Option<SessionState> {
        if let Ok(mut sessions) = self.sessions.write() {
            if let Some(session) = sessions.get_mut(session_id) {
                return Some(session.refresh());
            }
        }
        None
    }

    /// Terminate a session
    pub fn terminate_session(&self, session_id: &str, reason: &str) -> bool {
        if let Ok(mut sessions) = self.sessions.write() {
            if let Some(session) = sessions.get_mut(session_id) {
                session.terminate(reason);
                return true;
            }
        }
        false
    }

    /// Remove expired sessions from the manager
    pub fn cleanup_expired(&self) -> usize {
        let mut removed = 0;
        if let Ok(mut sessions) = self.sessions.write() {
            let expired_ids: Vec<String> = sessions
                .iter()
                .filter(|(_, s)| s.is_expired())
                .map(|(id, _)| id.clone())
                .collect();

            for id in expired_ids {
                sessions.remove(&id);
                removed += 1;
            }
        }
        removed
    }

    /// Get count of active sessions
    pub fn active_session_count(&self) -> usize {
        if let Ok(sessions) = self.sessions.read() {
            sessions.values().filter(|s| !s.is_expired()).count()
        } else {
            0
        }
    }

    /// Get session configuration
    pub fn config(&self) -> &SessionConfig {
        &self.config
    }
}

impl Default for SessionManager {
    fn default() -> Self {
        Self::dod_stig_default()
    }
}

// ============================================================================
// TESTS
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;
    use std::thread::sleep;
    use std::time::Duration;

    #[test]
    fn test_session_creation() {
        let config = SessionConfig::dod_stig_default();
        let session = Session::new("test-session-1", "test-user", config);

        assert_eq!(session.state(), SessionState::Active);
        assert!(!session.is_expired());
        assert!(session.time_remaining_secs() > 0);
    }

    #[test]
    fn test_session_config_clamping() {
        // Try to set timeout higher than allowed
        let config = SessionConfig::custom(9999, 60);

        // Should be clamped to DoD STIG maximum
        assert_eq!(config.max_timeout_secs, DOD_STIG_MAX_SESSION_TIMEOUT_SECS);
    }

    #[test]
    fn test_session_state_display() {
        assert_eq!(format!("{}", SessionState::Active), "ACTIVE");
        assert_eq!(format!("{}", SessionState::Warning), "WARNING");
        assert_eq!(format!("{}", SessionState::Expired), "EXPIRED");
        assert_eq!(format!("{}", SessionState::Locked), "LOCKED");
    }

    #[test]
    fn test_session_refresh() {
        let config = SessionConfig::custom(5, 2); // 5 second timeout, 2 second warning
        let mut session = Session::new("test-session-2", "test-user", config);

        // Session should be active
        assert_eq!(session.state(), SessionState::Active);

        // Wait a bit and refresh
        sleep(Duration::from_millis(100));
        let new_state = session.refresh();
        assert_eq!(new_state, SessionState::Active);
    }

    #[test]
    fn test_session_lock() {
        let config = SessionConfig::dod_stig_default();
        let mut session = Session::new("test-session-3", "test-user", config);

        session.lock("Manual lock for testing");

        assert_eq!(session.state(), SessionState::Locked);
        assert!(session.state().requires_reauth());
    }

    #[test]
    fn test_session_terminate() {
        let config = SessionConfig::dod_stig_default();
        let mut session = Session::new("test-session-4", "test-user", config);

        session.terminate("User logout");

        assert_eq!(session.state(), SessionState::Terminated);
        assert!(session.state().requires_reauth());
    }

    #[test]
    fn test_session_manager_create_and_get() {
        let manager = SessionManager::dod_stig_default();

        let session = manager.create_session("test-user");
        let session_id = session.id.clone();

        let retrieved = manager.get_session(&session_id);
        assert!(retrieved.is_some());
        assert_eq!(retrieved.unwrap().user_id, "test-user");
    }

    #[test]
    fn test_session_manager_validate() {
        let manager = SessionManager::dod_stig_default();
        let session = manager.create_session("test-user");
        let session_id = session.id.clone();

        let (is_valid, state, _) = manager.validate_session(&session_id);
        assert!(is_valid);
        assert_eq!(state, SessionState::Active);
    }

    #[test]
    fn test_session_event_audit_string() {
        let event = SessionEvent::Created {
            session_id: "test-123".to_string(),
            user_id: "user-456".to_string(),
            timestamp: Utc::now(),
        };

        let audit_string = event.to_audit_string();
        assert!(audit_string.contains("SESSION_CREATED"));
        assert!(audit_string.contains("test-123"));
        assert!(audit_string.contains("user-456"));
    }

    #[test]
    fn test_dod_stig_timeout_constant() {
        // Verify the constant is exactly 15 minutes
        assert_eq!(DOD_STIG_MAX_SESSION_TIMEOUT_SECS, 900);
        assert_eq!(DOD_STIG_MAX_SESSION_TIMEOUT_SECS / 60, 15);
    }

    #[test]
    fn test_consent_acknowledgment() {
        let config = SessionConfig::dod_stig_default();
        let mut session = Session::new("test-session-5", "test-user", config);

        assert!(session.needs_consent_reack());

        session.acknowledge_consent();

        assert!(!session.needs_consent_reack());
    }
}
