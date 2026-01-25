// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! Background Download System for rigrun
//!
//! This module provides non-blocking model downloads with:
//! - Progress visibility via `rigrun status`
//! - Resumable downloads that survive restarts
//! - Queue management with priorities
//!
//! # Architecture
//!
//! ```text
//! ┌─────────────────┐     ┌─────────────────┐
//! │ DownloadManager │────▶│ Background      │
//! │                 │     │ Worker (tokio)  │
//! └────────┬────────┘     └────────┬────────┘
//!          │                       │
//!          ▼                       ▼
//! ┌─────────────────┐     ┌─────────────────┐
//! │ DownloadState   │     │ OllamaClient    │
//! │ (persistent)    │     │ (pull_model)    │
//! └─────────────────┘     └─────────────────┘
//! ```
//!
//! # Usage
//!
//! ```rust,no_run
//! use rigrun::download::{DownloadManager, DownloadPriority};
//!
//! # async fn example() -> anyhow::Result<()> {
//! // Create manager (starts background worker)
//! let manager = DownloadManager::new()?;
//!
//! // Queue a download
//! let handle = manager.queue_download("qwen2.5-coder:7b", DownloadPriority::Normal).await?;
//!
//! // Check progress
//! let progress = handle.progress();
//! println!("Status: {:?}", progress.status);
//!
//! // Or wait for completion
//! let final_progress = handle.wait().await;
//! # Ok(())
//! # }
//! ```

pub mod types;
pub mod state;
pub mod manager;

// Re-export commonly used items
pub use types::{DownloadTask, DownloadStatus, DownloadProgress, DownloadPriority};
pub use state::DownloadState;
pub use manager::{DownloadManager, DownloadHandle, DownloadCommand};
