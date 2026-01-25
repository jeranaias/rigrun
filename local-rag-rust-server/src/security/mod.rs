//! Security module for the RAG server
//!
//! This module contains security-related functionality including:
//! - Session management with cryptographically secure IDs
//! - Proper error handling for lock operations

pub mod session_manager;

pub use session_manager::{
    Session, SessionData, SessionError, SessionManager, SessionManagerConfig,
    SessionState, SessionStats, SecurityLevel,
};
