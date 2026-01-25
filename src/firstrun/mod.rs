// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! First-Run Experience Module for rigrun
//!
//! This module provides the complete first-run experience including:
//! - Interactive setup wizard
//! - Hardware detection
//! - Configuration generation
//! - Model download with progress
//! - Health checks
//!
//! # Usage
//!
//! ```rust,no_run
//! use rigrun::firstrun::{is_first_run, run_wizard};
//!
//! if is_first_run() {
//!     run_wizard().await?;
//! }
//! ```

pub mod wizard;

// Re-export commonly used items
pub use wizard::{
    // Detection functions
    is_first_run,
    mark_wizard_complete,

    // Wizard runners
    run_wizard,
    run_quick_wizard,

    // Configuration types
    WizardConfig,
    DeploymentMode,
    UseCase,
    ModelSelection,

    // Utility functions
    download_model_with_progress,
    run_health_check,
    generate_config,
};
