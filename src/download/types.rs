// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! Download types for background model downloading.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

/// Status of a download task.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum DownloadStatus {
    /// Waiting in queue
    Queued,
    /// Currently downloading
    InProgress {
        bytes_downloaded: u64,
        bytes_total: Option<u64>,
    },
    /// Verifying integrity
    Verifying,
    /// Paused by user
    Paused { bytes_downloaded: u64 },
    /// Successfully completed
    Completed { completed_at: DateTime<Utc> },
    /// Failed with error
    Failed { error: String, retries: u32 },
}

impl DownloadStatus {
    /// Returns true if the download is complete (success or failure).
    pub fn is_terminal(&self) -> bool {
        matches!(self, DownloadStatus::Completed { .. } | DownloadStatus::Failed { .. })
    }

    /// Returns true if the download is actively running.
    pub fn is_active(&self) -> bool {
        matches!(self, DownloadStatus::InProgress { .. } | DownloadStatus::Verifying)
    }

    /// Get progress percentage (0-100) if available.
    pub fn progress_percent(&self) -> Option<f64> {
        match self {
            DownloadStatus::Completed { .. } => Some(100.0),
            DownloadStatus::Queued => Some(0.0),
            DownloadStatus::InProgress { bytes_downloaded, bytes_total } => {
                bytes_total.map(|total| {
                    if total == 0 { 0.0 } else { (*bytes_downloaded as f64 / total as f64) * 100.0 }
                })
            }
            DownloadStatus::Verifying => Some(99.0),
            DownloadStatus::Paused { .. } => None,
            DownloadStatus::Failed { .. } => None,
        }
    }
}

/// Priority level for downloads.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, PartialOrd, Ord)]
pub enum DownloadPriority {
    /// Low priority - can be interrupted
    Low = 0,
    /// Normal priority
    Normal = 1,
    /// High priority - download first
    High = 2,
}

impl Default for DownloadPriority {
    fn default() -> Self {
        DownloadPriority::Normal
    }
}

/// A download task representing a model to be downloaded.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DownloadTask {
    /// Unique identifier (usually the model name)
    pub id: String,
    /// Model name to download (e.g., "qwen2.5-coder:7b")
    pub model: String,
    /// Current status
    pub status: DownloadStatus,
    /// Priority level
    pub priority: DownloadPriority,
    /// When the task was created
    pub created_at: DateTime<Utc>,
    /// When the status was last updated
    pub updated_at: DateTime<Utc>,
    /// Number of retry attempts
    pub retries: u32,
    /// Maximum retries before giving up
    pub max_retries: u32,
}

impl DownloadTask {
    /// Create a new download task for a model.
    pub fn new(model: impl Into<String>) -> Self {
        let model = model.into();
        let now = Utc::now();
        Self {
            id: model.clone(),
            model,
            status: DownloadStatus::Queued,
            priority: DownloadPriority::default(),
            created_at: now,
            updated_at: now,
            retries: 0,
            max_retries: 3,
        }
    }

    /// Create a high-priority download task.
    pub fn high_priority(model: impl Into<String>) -> Self {
        let mut task = Self::new(model);
        task.priority = DownloadPriority::High;
        task
    }

    /// Update the status and timestamp.
    pub fn update_status(&mut self, status: DownloadStatus) {
        self.status = status;
        self.updated_at = Utc::now();
    }
}

/// Progress information for a download.
#[derive(Debug, Clone)]
pub struct DownloadProgress {
    /// Model being downloaded
    pub model: String,
    /// Current status
    pub status: DownloadStatus,
    /// Human-readable status message
    pub message: String,
    /// Estimated time remaining in seconds (if available)
    pub eta_seconds: Option<u64>,
    /// Download speed in bytes per second (if available)
    pub speed_bps: Option<u64>,
}

impl DownloadProgress {
    /// Create a new progress update.
    pub fn new(model: impl Into<String>, status: DownloadStatus, message: impl Into<String>) -> Self {
        Self {
            model: model.into(),
            status,
            message: message.into(),
            eta_seconds: None,
            speed_bps: None,
        }
    }

    /// Get formatted speed string.
    pub fn speed_string(&self) -> Option<String> {
        self.speed_bps.map(|bps| {
            if bps >= 1_073_741_824 {
                format!("{:.1} GB/s", bps as f64 / 1_073_741_824.0)
            } else if bps >= 1_048_576 {
                format!("{:.1} MB/s", bps as f64 / 1_048_576.0)
            } else if bps >= 1024 {
                format!("{:.1} KB/s", bps as f64 / 1024.0)
            } else {
                format!("{} B/s", bps)
            }
        })
    }

    /// Get formatted ETA string.
    pub fn eta_string(&self) -> Option<String> {
        self.eta_seconds.map(|secs| {
            if secs >= 3600 {
                format!("{}h {}m", secs / 3600, (secs % 3600) / 60)
            } else if secs >= 60 {
                format!("{}m {}s", secs / 60, secs % 60)
            } else {
                format!("{}s", secs)
            }
        })
    }
}
