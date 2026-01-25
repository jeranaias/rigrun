// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! Security Module for DoD STIG Compliance
//!
//! This module implements security controls required for IL5 (Impact Level 5)
//! environments per DoD Security Technical Implementation Guide (STIG) requirements.
//!
//! ## Key Requirements Implemented
//!
//! - **Session Timeout (STIG ID: AC-12)**: Maximum 15-minute (900 second) session timeout
//! - **Session Lock (STIG ID: AC-11)**: Screen/session lock after inactivity
//! - **Re-authentication**: Required after session timeout
//! - **Audit Logging**: All session events are logged for security audit
//!
//! ## Usage
//!
//! ```no_run
//! use rigrun::security::{Session, SessionManager, SessionConfig};
//!
//! // Create a session manager with DoD STIG defaults
//! let config = SessionConfig::dod_stig_default();
//! let manager = SessionManager::new(config);
//!
//! // Create a new session
//! let session = manager.create_session("user_id");
//!
//! // Check if session is valid
//! if session.is_expired() {
//!     // Require re-authentication
//! }
//! ```

pub mod locks;
pub mod session_manager;

pub use locks::{resilient_read, resilient_write, try_resilient_read, try_resilient_write};
pub use session_manager::{
    Session, SessionConfig, SessionManager, SessionState, SessionEvent, PrivilegeLevel,
    DOD_STIG_MAX_SESSION_TIMEOUT_SECS, DOD_STIG_WARNING_BEFORE_TIMEOUT_SECS,
};
