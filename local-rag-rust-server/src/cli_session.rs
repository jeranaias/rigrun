//! CLI Session Management
//!
//! This module handles session management for CLI interactions with the RAG server.
//! Security fixes applied:
//! - RSESS-1: Uses 128-bit random session IDs with OsRng for cryptographic security

use chrono::{DateTime, Utc};
use rand::rngs::OsRng;
use rand::RngCore;
use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};
use thiserror::Error;

/// Errors that can occur during CLI session operations
#[derive(Debug, Error)]
pub enum CliSessionError {
    #[error("Session not found: {0}")]
    NotFound(String),

    #[error("Session expired: {0}")]
    Expired(String),

    #[error("Lock poisoned - concurrent access error")]
    LockPoisoned,

    #[error("Invalid session ID format")]
    InvalidFormat,
}

/// Represents a CLI session with security-enhanced properties
#[derive(Debug, Clone)]
pub struct CliSession {
    pub id: String,
    pub user_id: Option<String>,
    pub created_at: Instant,
    pub created_at_utc: DateTime<Utc>,
    pub last_activity: Instant,
    pub last_activity_utc: DateTime<Utc>,
    pub metadata: HashMap<String, String>,
    pub is_authenticated: bool,
}

impl CliSession {
    /// Create a new CLI session with a secure session ID
    pub fn new(user_id: Option<String>) -> Self {
        let now = Instant::now();
        let utc_now = Utc::now();

        CliSession {
            id: generate_session_id(),
            user_id,
            created_at: now,
            created_at_utc: utc_now,
            last_activity: now,
            last_activity_utc: utc_now,
            metadata: HashMap::new(),
            is_authenticated: false,
        }
    }

    /// Refresh the session's last activity timestamp
    pub fn refresh(&mut self) {
        self.last_activity = Instant::now();
        self.last_activity_utc = Utc::now();
    }

    /// Check if the session has expired based on a timeout duration
    pub fn is_expired(&self, timeout: Duration) -> bool {
        self.last_activity.elapsed() > timeout
    }

    /// Get session age in seconds
    pub fn age_seconds(&self) -> u64 {
        self.created_at.elapsed().as_secs()
    }
}

/// Generates a cryptographically secure 128-bit session ID
///
/// Security fix RSESS-1: Changed from 32-bit random to 128-bit random
/// using OsRng for cryptographic security.
///
/// Format: cli_sess_{timestamp_millis}_{32_hex_chars}
/// The timestamp provides ordering while the random bytes provide uniqueness and unpredictability.
fn generate_session_id() -> String {
    let mut bytes = [0u8; 16]; // 128 bits for cryptographic security
    OsRng.fill_bytes(&mut bytes);

    let utc_now = chrono::Utc::now();
    format!(
        "cli_sess_{}_{}",
        utc_now.timestamp_millis(),
        hex::encode(bytes)
    )
}

/// Manager for CLI sessions
pub struct CliSessionManager {
    sessions: Arc<RwLock<HashMap<String, CliSession>>>,
    session_timeout: Duration,
    max_sessions: usize,
}

impl CliSessionManager {
    /// Create a new CLI session manager
    pub fn new(session_timeout: Duration, max_sessions: usize) -> Self {
        CliSessionManager {
            sessions: Arc::new(RwLock::new(HashMap::new())),
            session_timeout,
            max_sessions,
        }
    }

    /// Create a new session for a user
    pub fn create_session(&self, user_id: Option<String>) -> Result<CliSession, CliSessionError> {
        let session = CliSession::new(user_id);
        self.store_session(session.clone())?;
        Ok(session)
    }

    /// Store a session (with proper error propagation - fixes silent failures)
    pub fn store_session(&self, session: CliSession) -> Result<(), CliSessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| CliSessionError::LockPoisoned)?;

        // Check max sessions limit
        if sessions.len() >= self.max_sessions && !sessions.contains_key(&session.id) {
            // Remove oldest expired session or oldest session
            self.cleanup_oldest_session(&mut sessions);
        }

        sessions.insert(session.id.clone(), session);
        Ok(())
    }

    /// Get a session by ID
    pub fn get_session(&self, session_id: &str) -> Result<CliSession, CliSessionError> {
        let sessions = self.sessions.read()
            .map_err(|_| CliSessionError::LockPoisoned)?;

        match sessions.get(session_id) {
            Some(session) => {
                if session.is_expired(self.session_timeout) {
                    Err(CliSessionError::Expired(session_id.to_string()))
                } else {
                    Ok(session.clone())
                }
            }
            None => Err(CliSessionError::NotFound(session_id.to_string())),
        }
    }

    /// Refresh a session's activity timestamp
    pub fn refresh_session(&self, session_id: &str) -> Result<(), CliSessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| CliSessionError::LockPoisoned)?;

        match sessions.get_mut(session_id) {
            Some(session) => {
                if session.is_expired(self.session_timeout) {
                    Err(CliSessionError::Expired(session_id.to_string()))
                } else {
                    session.refresh();
                    Ok(())
                }
            }
            None => Err(CliSessionError::NotFound(session_id.to_string())),
        }
    }

    /// Atomically get a session and refresh it in a single operation.
    ///
    /// This method eliminates the race condition where a session could expire
    /// between get() and refresh() calls by performing both atomically.
    ///
    /// Returns Ok(session) if the session is valid and was refreshed,
    /// or appropriate error if the session has expired or is not found.
    ///
    /// # Concurrency Safety
    /// The session is refreshed before returning, and the returned clone reflects
    /// the refreshed state. No other thread can expire the session between
    /// validation and refresh.
    pub fn get_and_refresh_session(&self, session_id: &str) -> Result<CliSession, CliSessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| CliSessionError::LockPoisoned)?;

        match sessions.get_mut(session_id) {
            Some(session) => {
                if session.is_expired(self.session_timeout) {
                    Err(CliSessionError::Expired(session_id.to_string()))
                } else {
                    // Atomically refresh and return clone
                    session.refresh();
                    Ok(session.clone())
                }
            }
            None => Err(CliSessionError::NotFound(session_id.to_string())),
        }
    }

    /// Remove a session
    pub fn remove_session(&self, session_id: &str) -> Result<Option<CliSession>, CliSessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| CliSessionError::LockPoisoned)?;

        Ok(sessions.remove(session_id))
    }

    /// Clean up expired sessions
    pub fn cleanup_expired(&self) -> Result<usize, CliSessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| CliSessionError::LockPoisoned)?;

        let expired_ids: Vec<String> = sessions
            .iter()
            .filter(|(_, session)| session.is_expired(self.session_timeout))
            .map(|(id, _)| id.clone())
            .collect();

        let count = expired_ids.len();
        for id in expired_ids {
            sessions.remove(&id);
        }

        Ok(count)
    }

    /// Helper to remove the oldest session when at capacity
    fn cleanup_oldest_session(&self, sessions: &mut HashMap<String, CliSession>) {
        if let Some(oldest_id) = sessions
            .iter()
            .min_by_key(|(_, s)| s.created_at_utc)
            .map(|(id, _)| id.clone())
        {
            sessions.remove(&oldest_id);
        }
    }

    /// Get the number of active sessions
    pub fn active_session_count(&self) -> Result<usize, CliSessionError> {
        let sessions = self.sessions.read()
            .map_err(|_| CliSessionError::LockPoisoned)?;

        Ok(sessions
            .values()
            .filter(|s| !s.is_expired(self.session_timeout))
            .count())
    }
}

impl Default for CliSessionManager {
    fn default() -> Self {
        CliSessionManager::new(Duration::from_secs(3600), 1000)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_generate_session_id_format() {
        let id = generate_session_id();
        assert!(id.starts_with("cli_sess_"));
        // Should have timestamp and 32 hex chars (16 bytes = 32 hex)
        let parts: Vec<&str> = id.split('_').collect();
        assert_eq!(parts.len(), 3);
        assert_eq!(parts[2].len(), 32); // 128 bits = 16 bytes = 32 hex chars
    }

    #[test]
    fn test_generate_session_id_uniqueness() {
        let id1 = generate_session_id();
        let id2 = generate_session_id();
        assert_ne!(id1, id2);
    }

    #[test]
    fn test_session_refresh() {
        let mut session = CliSession::new(Some("test_user".to_string()));
        let original_activity = session.last_activity_utc;
        std::thread::sleep(Duration::from_millis(10));
        session.refresh();
        assert!(session.last_activity_utc > original_activity);
    }

    #[test]
    fn test_session_expiry() {
        let session = CliSession::new(None);
        assert!(!session.is_expired(Duration::from_secs(60)));
        assert!(session.is_expired(Duration::from_nanos(1)));
    }
}
