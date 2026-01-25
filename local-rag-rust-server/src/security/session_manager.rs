//! Session Manager for RAG Server Security
//!
//! This module provides secure session management for the RAG server.
//! Security fixes applied:
//! - RSESS-2: Uses OsRng instead of thread_rng() for cryptographic security
//! - RSESS-3: Added UTC timestamps for last_activity to enable persistence
//! - RSESS-4: Properly propagates lock errors instead of silently failing

use chrono::{DateTime, Utc};
use rand::rngs::OsRng;
use rand::RngCore;
use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};
use thiserror::Error;

/// Session-related errors with proper error propagation (RSESS-4 fix)
#[derive(Debug, Error)]
pub enum SessionError {
    #[error("Session not found: {0}")]
    NotFound(String),

    #[error("Session expired: {0}")]
    Expired(String),

    #[error("Lock poisoned - concurrent access failure")]
    LockPoisoned,

    #[error("Session limit exceeded")]
    LimitExceeded,

    #[error("Invalid session data: {0}")]
    InvalidData(String),

    #[error("Session already exists: {0}")]
    AlreadyExists(String),

    #[error("Authentication required")]
    AuthRequired,

    #[error("Insufficient permissions")]
    InsufficientPermissions,
}

/// Session state enum for tracking lifecycle
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SessionState {
    Active,
    Idle,
    Expired,
    Terminated,
}

/// Security level for session operations
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SecurityLevel {
    Public,
    Authenticated,
    Elevated,
    Admin,
}

/// Session data that can be serialized for persistence
/// RSESS-3 fix: Added last_activity_utc for persistence support
#[derive(Debug, Clone)]
pub struct SessionData {
    pub user_id: Option<String>,
    pub roles: Vec<String>,
    pub permissions: Vec<String>,
    pub security_level: SecurityLevel,
    pub ip_address: Option<String>,
    pub user_agent: Option<String>,
    pub custom_data: HashMap<String, String>,
}

impl Default for SessionData {
    fn default() -> Self {
        SessionData {
            user_id: None,
            roles: Vec::new(),
            permissions: Vec::new(),
            security_level: SecurityLevel::Public,
            ip_address: None,
            user_agent: None,
            custom_data: HashMap::new(),
        }
    }
}

/// Represents a secure session with UTC timestamps for persistence
/// RSESS-3 fix: Added created_at_utc and last_activity_utc fields
#[derive(Debug, Clone)]
pub struct Session {
    pub id: String,
    pub created_at: Instant,
    pub created_at_utc: DateTime<Utc>,
    pub last_activity: Instant,
    pub last_activity_utc: DateTime<Utc>,  // RSESS-3: UTC timestamp for persistence
    pub state: SessionState,
    pub data: SessionData,
    pub access_count: u64,
}

impl Session {
    /// Create a new session with secure ID generation
    pub fn new(data: SessionData) -> Self {
        let now = Instant::now();
        let utc_now = Utc::now();

        Session {
            id: generate_secure_session_id(),
            created_at: now,
            created_at_utc: utc_now,
            last_activity: now,
            last_activity_utc: utc_now,
            state: SessionState::Active,
            data,
            access_count: 0,
        }
    }

    /// Refresh the session's last activity timestamp
    /// RSESS-3 fix: Updates both Instant and UTC timestamps
    pub fn refresh(&mut self) {
        self.last_activity = Instant::now();
        self.last_activity_utc = Utc::now();
        self.access_count += 1;
        if self.state == SessionState::Idle {
            self.state = SessionState::Active;
        }
    }

    /// Check if session is expired based on timeout
    pub fn is_expired(&self, timeout: Duration) -> bool {
        self.last_activity.elapsed() > timeout || self.state == SessionState::Expired
    }

    /// Mark session as expired
    pub fn expire(&mut self) {
        self.state = SessionState::Expired;
    }

    /// Mark session as idle
    pub fn mark_idle(&mut self) {
        if self.state == SessionState::Active {
            self.state = SessionState::Idle;
        }
    }

    /// Terminate the session
    pub fn terminate(&mut self) {
        self.state = SessionState::Terminated;
    }

    /// Check if session has a specific permission
    pub fn has_permission(&self, permission: &str) -> bool {
        self.data.permissions.contains(&permission.to_string())
    }

    /// Check if session has a specific role
    pub fn has_role(&self, role: &str) -> bool {
        self.data.roles.contains(&role.to_string())
    }

    /// Get session age in seconds
    pub fn age_seconds(&self) -> u64 {
        self.created_at.elapsed().as_secs()
    }

    /// Get time since last activity in seconds
    pub fn idle_seconds(&self) -> u64 {
        self.last_activity.elapsed().as_secs()
    }
}

/// Generates a cryptographically secure session ID using OsRng
/// RSESS-2 fix: Uses OsRng instead of thread_rng() for cryptographic security
fn generate_secure_session_id() -> String {
    let mut bytes = [0u8; 16]; // 128 bits for security
    OsRng.fill_bytes(&mut bytes);

    format!("sess_{}", hex::encode(bytes))
}

/// Configuration for the session manager
#[derive(Debug, Clone)]
pub struct SessionManagerConfig {
    pub session_timeout: Duration,
    pub idle_timeout: Duration,
    pub max_sessions: usize,
    pub max_sessions_per_user: usize,
    pub cleanup_interval: Duration,
}

impl Default for SessionManagerConfig {
    fn default() -> Self {
        SessionManagerConfig {
            session_timeout: Duration::from_secs(3600),      // 1 hour
            idle_timeout: Duration::from_secs(900),          // 15 minutes
            max_sessions: 10000,
            max_sessions_per_user: 5,
            cleanup_interval: Duration::from_secs(300),      // 5 minutes
        }
    }
}

/// Secure session manager with proper error handling
/// RSESS-4 fix: All lock operations properly propagate errors
pub struct SessionManager {
    sessions: Arc<RwLock<HashMap<String, Session>>>,
    config: SessionManagerConfig,
    last_cleanup: Arc<RwLock<Instant>>,
}

impl SessionManager {
    /// Create a new session manager with the given configuration
    pub fn new(config: SessionManagerConfig) -> Self {
        SessionManager {
            sessions: Arc::new(RwLock::new(HashMap::new())),
            config,
            last_cleanup: Arc::new(RwLock::new(Instant::now())),
        }
    }

    /// Create a new session
    pub fn create_session(&self, data: SessionData) -> Result<Session, SessionError> {
        // Check user session limit if user_id is present
        if let Some(ref user_id) = data.user_id {
            self.check_user_session_limit(user_id)?;
        }

        let session = Session::new(data);
        self.store_session(session.clone())?;
        Ok(session)
    }

    /// Store a session with proper error propagation
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn store_session(&self, session: Session) -> Result<(), SessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| SessionError::LockPoisoned)?;

        // Check max sessions limit
        if sessions.len() >= self.config.max_sessions && !sessions.contains_key(&session.id) {
            return Err(SessionError::LimitExceeded);
        }

        sessions.insert(session.id.clone(), session);
        Ok(())
    }

    /// Get a session by ID with proper error propagation
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn get_session(&self, session_id: &str) -> Result<Session, SessionError> {
        let sessions = self.sessions.read()
            .map_err(|_| SessionError::LockPoisoned)?;

        match sessions.get(session_id) {
            Some(session) => {
                if session.is_expired(self.config.session_timeout) {
                    Err(SessionError::Expired(session_id.to_string()))
                } else {
                    Ok(session.clone())
                }
            }
            None => Err(SessionError::NotFound(session_id.to_string())),
        }
    }

    /// Get a mutable reference to a session and refresh it
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn refresh_session(&self, session_id: &str) -> Result<Session, SessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| SessionError::LockPoisoned)?;

        match sessions.get_mut(session_id) {
            Some(session) => {
                if session.is_expired(self.config.session_timeout) {
                    Err(SessionError::Expired(session_id.to_string()))
                } else {
                    session.refresh();
                    Ok(session.clone())
                }
            }
            None => Err(SessionError::NotFound(session_id.to_string())),
        }
    }

    /// Remove a session
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn remove_session(&self, session_id: &str) -> Result<Option<Session>, SessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| SessionError::LockPoisoned)?;

        Ok(sessions.remove(session_id))
    }

    /// Terminate a session (marks it as terminated but keeps for audit)
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn terminate_session(&self, session_id: &str) -> Result<(), SessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| SessionError::LockPoisoned)?;

        match sessions.get_mut(session_id) {
            Some(session) => {
                session.terminate();
                Ok(())
            }
            None => Err(SessionError::NotFound(session_id.to_string())),
        }
    }

    /// Validate a session exists and is active
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn validate_session(&self, session_id: &str) -> Result<bool, SessionError> {
        let sessions = self.sessions.read()
            .map_err(|_| SessionError::LockPoisoned)?;

        match sessions.get(session_id) {
            Some(session) => Ok(!session.is_expired(self.config.session_timeout)
                               && session.state == SessionState::Active),
            None => Ok(false),
        }
    }

    /// Atomically validate and refresh a session in a single operation.
    ///
    /// This method eliminates the race condition between validate_session() and
    /// refresh_session() by performing both operations while holding the write lock.
    ///
    /// Returns Ok((session, is_valid)) if the session exists.
    /// - If valid: refreshes the session and returns the updated session with is_valid=true
    /// - If invalid/expired: returns the session without refresh and is_valid=false
    ///
    /// # Concurrency Safety
    /// This operation is atomic - no other thread can modify the session between
    /// validation and refresh, eliminating TOCTOU race conditions.
    pub fn validate_and_refresh_session(&self, session_id: &str) -> Result<(Session, bool), SessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| SessionError::LockPoisoned)?;

        match sessions.get_mut(session_id) {
            Some(session) => {
                let is_valid = !session.is_expired(self.config.session_timeout)
                    && session.state == SessionState::Active;

                if is_valid {
                    // Atomically refresh only if the session is still valid
                    session.refresh();
                }

                Ok((session.clone(), is_valid))
            }
            None => Err(SessionError::NotFound(session_id.to_string())),
        }
    }

    /// Atomically get a session and refresh it in a single operation.
    ///
    /// This method eliminates the race condition where a session could expire
    /// between get() and refresh() calls by performing both atomically.
    ///
    /// Returns Ok(session) if the session is valid and was refreshed,
    /// or Err(SessionError::Expired) if the session has expired.
    ///
    /// # Concurrency Safety
    /// The session is refreshed before returning, and the returned clone reflects
    /// the refreshed state. No other thread can expire the session between
    /// validation and refresh.
    pub fn get_and_refresh_session(&self, session_id: &str) -> Result<Session, SessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| SessionError::LockPoisoned)?;

        match sessions.get_mut(session_id) {
            Some(session) => {
                if session.is_expired(self.config.session_timeout) {
                    // Mark as expired if not already
                    if session.state != SessionState::Expired {
                        session.expire();
                    }
                    Err(SessionError::Expired(session_id.to_string()))
                } else {
                    // Atomically refresh and return clone
                    session.refresh();
                    Ok(session.clone())
                }
            }
            None => Err(SessionError::NotFound(session_id.to_string())),
        }
    }

    /// Check permission for a session
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn check_permission(&self, session_id: &str, permission: &str) -> Result<bool, SessionError> {
        let session = self.get_session(session_id)?;
        Ok(session.has_permission(permission))
    }

    /// Clean up expired sessions
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn cleanup_expired(&self) -> Result<usize, SessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| SessionError::LockPoisoned)?;

        let expired_ids: Vec<String> = sessions
            .iter()
            .filter(|(_, session)| {
                session.is_expired(self.config.session_timeout)
                || session.state == SessionState::Terminated
            })
            .map(|(id, _)| id.clone())
            .collect();

        let count = expired_ids.len();
        for id in expired_ids {
            sessions.remove(&id);
        }

        // Update last cleanup time
        if let Ok(mut last_cleanup) = self.last_cleanup.write() {
            *last_cleanup = Instant::now();
        }

        Ok(count)
    }

    /// Mark idle sessions
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn mark_idle_sessions(&self) -> Result<usize, SessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| SessionError::LockPoisoned)?;

        let mut count = 0;
        for session in sessions.values_mut() {
            if session.state == SessionState::Active
               && session.last_activity.elapsed() > self.config.idle_timeout
            {
                session.mark_idle();
                count += 1;
            }
        }

        Ok(count)
    }

    /// Check if periodic cleanup is needed and perform it
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn maybe_cleanup(&self) -> Result<Option<usize>, SessionError> {
        let needs_cleanup = {
            let last_cleanup = self.last_cleanup.read()
                .map_err(|_| SessionError::LockPoisoned)?;
            last_cleanup.elapsed() > self.config.cleanup_interval
        };

        if needs_cleanup {
            Ok(Some(self.cleanup_expired()?))
        } else {
            Ok(None)
        }
    }

    /// Get session count
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn session_count(&self) -> Result<usize, SessionError> {
        let sessions = self.sessions.read()
            .map_err(|_| SessionError::LockPoisoned)?;
        Ok(sessions.len())
    }

    /// Get active session count
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn active_session_count(&self) -> Result<usize, SessionError> {
        let sessions = self.sessions.read()
            .map_err(|_| SessionError::LockPoisoned)?;

        Ok(sessions
            .values()
            .filter(|s| !s.is_expired(self.config.session_timeout)
                       && s.state == SessionState::Active)
            .count())
    }

    /// Get all sessions for a user
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn get_user_sessions(&self, user_id: &str) -> Result<Vec<Session>, SessionError> {
        let sessions = self.sessions.read()
            .map_err(|_| SessionError::LockPoisoned)?;

        Ok(sessions
            .values()
            .filter(|s| s.data.user_id.as_deref() == Some(user_id))
            .cloned()
            .collect())
    }

    /// Check user session limit
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    fn check_user_session_limit(&self, user_id: &str) -> Result<(), SessionError> {
        let sessions = self.sessions.read()
            .map_err(|_| SessionError::LockPoisoned)?;

        let user_session_count = sessions
            .values()
            .filter(|s| s.data.user_id.as_deref() == Some(user_id)
                       && !s.is_expired(self.config.session_timeout))
            .count();

        if user_session_count >= self.config.max_sessions_per_user {
            Err(SessionError::LimitExceeded)
        } else {
            Ok(())
        }
    }

    /// Terminate all sessions for a user
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn terminate_user_sessions(&self, user_id: &str) -> Result<usize, SessionError> {
        let mut sessions = self.sessions.write()
            .map_err(|_| SessionError::LockPoisoned)?;

        let mut count = 0;
        for session in sessions.values_mut() {
            if session.data.user_id.as_deref() == Some(user_id) {
                session.terminate();
                count += 1;
            }
        }

        Ok(count)
    }

    /// Get session statistics
    /// RSESS-4 fix: Propagates lock errors instead of silently failing
    pub fn get_stats(&self) -> Result<SessionStats, SessionError> {
        let sessions = self.sessions.read()
            .map_err(|_| SessionError::LockPoisoned)?;

        let total = sessions.len();
        let active = sessions.values().filter(|s| s.state == SessionState::Active).count();
        let idle = sessions.values().filter(|s| s.state == SessionState::Idle).count();
        let expired = sessions.values().filter(|s| s.state == SessionState::Expired).count();
        let terminated = sessions.values().filter(|s| s.state == SessionState::Terminated).count();

        Ok(SessionStats {
            total,
            active,
            idle,
            expired,
            terminated,
        })
    }
}

impl Default for SessionManager {
    fn default() -> Self {
        SessionManager::new(SessionManagerConfig::default())
    }
}

/// Session statistics
#[derive(Debug, Clone)]
pub struct SessionStats {
    pub total: usize,
    pub active: usize,
    pub idle: usize,
    pub expired: usize,
    pub terminated: usize,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_generate_secure_session_id() {
        let id = generate_secure_session_id();
        assert!(id.starts_with("sess_"));
        // Should have 32 hex chars (16 bytes = 32 hex)
        let hex_part = &id[5..];
        assert_eq!(hex_part.len(), 32);
    }

    #[test]
    fn test_session_id_uniqueness() {
        let id1 = generate_secure_session_id();
        let id2 = generate_secure_session_id();
        assert_ne!(id1, id2);
    }

    #[test]
    fn test_session_refresh_updates_utc() {
        let mut session = Session::new(SessionData::default());
        let original_utc = session.last_activity_utc;
        std::thread::sleep(Duration::from_millis(10));
        session.refresh();
        assert!(session.last_activity_utc > original_utc);
    }

    #[test]
    fn test_session_manager_store_and_retrieve() {
        let manager = SessionManager::default();
        let session = Session::new(SessionData::default());
        let session_id = session.id.clone();

        manager.store_session(session).unwrap();
        let retrieved = manager.get_session(&session_id).unwrap();

        assert_eq!(retrieved.id, session_id);
    }

    #[test]
    fn test_session_manager_error_on_not_found() {
        let manager = SessionManager::default();
        let result = manager.get_session("nonexistent");

        assert!(matches!(result, Err(SessionError::NotFound(_))));
    }

    #[test]
    fn test_session_expiry() {
        let config = SessionManagerConfig {
            session_timeout: Duration::from_millis(1),
            ..Default::default()
        };
        let manager = SessionManager::new(config);

        let session = Session::new(SessionData::default());
        let session_id = session.id.clone();
        manager.store_session(session).unwrap();

        std::thread::sleep(Duration::from_millis(10));

        let result = manager.get_session(&session_id);
        assert!(matches!(result, Err(SessionError::Expired(_))));
    }

    #[test]
    fn test_session_permissions() {
        let mut data = SessionData::default();
        data.permissions = vec!["read".to_string(), "write".to_string()];

        let session = Session::new(data);

        assert!(session.has_permission("read"));
        assert!(session.has_permission("write"));
        assert!(!session.has_permission("admin"));
    }

    #[test]
    fn test_cleanup_expired_sessions() {
        let config = SessionManagerConfig {
            session_timeout: Duration::from_millis(1),
            ..Default::default()
        };
        let manager = SessionManager::new(config);

        // Create sessions
        for _ in 0..5 {
            let session = Session::new(SessionData::default());
            manager.store_session(session).unwrap();
        }

        assert_eq!(manager.session_count().unwrap(), 5);

        std::thread::sleep(Duration::from_millis(10));

        let cleaned = manager.cleanup_expired().unwrap();
        assert_eq!(cleaned, 5);
        assert_eq!(manager.session_count().unwrap(), 0);
    }

    #[test]
    fn test_validate_and_refresh_session_atomic() {
        let manager = SessionManager::default();
        let session = Session::new(SessionData::default());
        let session_id = session.id.clone();
        manager.store_session(session).unwrap();

        // Atomic validate and refresh should work
        let (refreshed_session, is_valid) =
            manager.validate_and_refresh_session(&session_id).unwrap();

        assert!(is_valid);
        assert_eq!(refreshed_session.id, session_id);
    }

    #[test]
    fn test_get_and_refresh_session_atomic() {
        let manager = SessionManager::default();
        let session = Session::new(SessionData::default());
        let session_id = session.id.clone();
        manager.store_session(session).unwrap();

        // Atomic get and refresh should work
        let refreshed_session = manager.get_and_refresh_session(&session_id).unwrap();
        assert_eq!(refreshed_session.id, session_id);
    }

    #[test]
    fn test_validate_and_refresh_nonexistent_session() {
        let manager = SessionManager::default();

        let result = manager.validate_and_refresh_session("nonexistent-session");
        assert!(matches!(result, Err(SessionError::NotFound(_))));
    }

    #[test]
    fn test_get_and_refresh_nonexistent_session() {
        let manager = SessionManager::default();

        let result = manager.get_and_refresh_session("nonexistent-session");
        assert!(matches!(result, Err(SessionError::NotFound(_))));
    }

    #[test]
    fn test_concurrent_validate_and_refresh() {
        use std::sync::Arc;
        use std::thread;

        let manager = Arc::new(SessionManager::default());
        let session = Session::new(SessionData::default());
        let session_id = session.id.clone();
        manager.store_session(session).unwrap();

        // Spawn multiple threads that concurrently validate and refresh
        let mut handles = vec![];
        for _ in 0..10 {
            let manager_clone = Arc::clone(&manager);
            let session_id_clone = session_id.clone();
            let handle = thread::spawn(move || {
                for _ in 0..100 {
                    let result = manager_clone.validate_and_refresh_session(&session_id_clone);
                    assert!(result.is_ok());
                    let (_, is_valid) = result.unwrap();
                    assert!(is_valid);
                }
            });
            handles.push(handle);
        }

        // Wait for all threads to complete
        for handle in handles {
            handle.join().expect("Thread panicked");
        }

        // Session should still be valid after concurrent access
        let (_, is_valid) = manager.validate_and_refresh_session(&session_id).unwrap();
        assert!(is_valid);
    }

    #[test]
    fn test_concurrent_get_and_refresh() {
        use std::sync::Arc;
        use std::thread;

        let manager = Arc::new(SessionManager::default());
        let session = Session::new(SessionData::default());
        let session_id = session.id.clone();
        manager.store_session(session).unwrap();

        // Spawn multiple threads that concurrently get and refresh
        let mut handles = vec![];
        for _ in 0..10 {
            let manager_clone = Arc::clone(&manager);
            let session_id_clone = session_id.clone();
            let handle = thread::spawn(move || {
                for _ in 0..100 {
                    let result = manager_clone.get_and_refresh_session(&session_id_clone);
                    assert!(result.is_ok());
                }
            });
            handles.push(handle);
        }

        // Wait for all threads to complete
        for handle in handles {
            handle.join().expect("Thread panicked");
        }

        // Session should still be valid after concurrent access
        let result = manager.get_and_refresh_session(&session_id);
        assert!(result.is_ok());
    }

    #[test]
    fn test_no_race_between_refresh_and_get() {
        use std::sync::Arc;
        use std::thread;

        let manager = Arc::new(SessionManager::default());
        let session = Session::new(SessionData::default());
        let session_id = session.id.clone();
        manager.store_session(session).unwrap();

        // Spawn threads that do get operations
        let mut handles = vec![];
        for _ in 0..5 {
            let manager_clone = Arc::clone(&manager);
            let session_id_clone = session_id.clone();
            let handle = thread::spawn(move || {
                for _ in 0..100 {
                    // Use atomic get_and_refresh to prevent race
                    let result = manager_clone.get_and_refresh_session(&session_id_clone);
                    // Session should always be valid since we're refreshing it atomically
                    assert!(result.is_ok(), "Session was unexpectedly invalid during get");
                }
            });
            handles.push(handle);
        }

        // Spawn threads that do validate_and_refresh operations
        for _ in 0..5 {
            let manager_clone = Arc::clone(&manager);
            let session_id_clone = session_id.clone();
            let handle = thread::spawn(move || {
                for _ in 0..100 {
                    // Use atomic validate_and_refresh to prevent race
                    let result = manager_clone.validate_and_refresh_session(&session_id_clone);
                    assert!(result.is_ok(), "Session was unexpectedly invalid during validate");
                    let (_, is_valid) = result.unwrap();
                    assert!(is_valid);
                }
            });
            handles.push(handle);
        }

        // Wait for all threads to complete
        for handle in handles {
            handle.join().expect("Thread panicked");
        }

        // Final check - session should still be valid
        let (_, is_valid) = manager.validate_and_refresh_session(&session_id).unwrap();
        assert!(is_valid);
    }
}
