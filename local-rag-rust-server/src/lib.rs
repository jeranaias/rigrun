//! Local RAG Rust Server
//!
//! This library provides secure session management and RAG functionality.
//!
//! Security features:
//! - 128-bit cryptographically secure session IDs (OsRng)
//! - UTC timestamps for session persistence
//! - Proper error propagation for lock operations

pub mod cli_session;
pub mod security;

pub use cli_session::{CliSession, CliSessionError, CliSessionManager};
pub use security::{
    Session, SessionData, SessionError, SessionManager, SessionManagerConfig,
    SessionState, SessionStats, SecurityLevel,
};
