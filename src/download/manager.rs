// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! Download manager for background model downloading.

use std::sync::{Arc, RwLock};
use std::time::Instant;
use anyhow::Result;
use chrono::Utc;
use tokio::sync::{mpsc, watch};

use crate::local::OllamaClient;
use crate::security::{resilient_read, resilient_write};
use super::types::{DownloadTask, DownloadStatus, DownloadProgress, DownloadPriority};
use super::state::DownloadState;

/// Commands that can be sent to the download worker.
#[derive(Debug)]
pub enum DownloadCommand {
    /// Queue a new download
    Queue { model: String, priority: DownloadPriority },
    /// Cancel a download
    Cancel { model: String },
    /// Pause a download
    Pause { model: String },
    /// Resume a paused download
    Resume { model: String },
    /// Shutdown the worker
    Shutdown,
}

/// Handle to a download, allowing status checks.
#[derive(Debug, Clone)]
pub struct DownloadHandle {
    /// Model being downloaded
    pub model: String,
    /// Receiver for progress updates
    progress_rx: watch::Receiver<DownloadProgress>,
}

impl DownloadHandle {
    /// Get the current progress.
    pub fn progress(&self) -> DownloadProgress {
        self.progress_rx.borrow().clone()
    }

    /// Check if the download is complete.
    pub fn is_complete(&self) -> bool {
        self.progress_rx.borrow().status.is_terminal()
    }

    /// Wait for the download to complete.
    pub async fn wait(&mut self) -> DownloadProgress {
        loop {
            if self.progress_rx.borrow().status.is_terminal() {
                return self.progress_rx.borrow().clone();
            }
            if self.progress_rx.changed().await.is_err() {
                // Channel closed
                return self.progress_rx.borrow().clone();
            }
        }
    }
}

/// Manager for background downloads.
///
/// Handles queueing, cancellation, and progress tracking for model downloads.
pub struct DownloadManager {
    /// Channel to send commands to the worker
    command_tx: mpsc::Sender<DownloadCommand>,
    /// Shared state for downloads
    state: Arc<RwLock<DownloadState>>,
    /// Progress watchers for active downloads
    progress_watchers: Arc<RwLock<std::collections::HashMap<String, watch::Sender<DownloadProgress>>>>,
}

impl DownloadManager {
    /// Create a new download manager.
    ///
    /// Loads any existing state from disk and starts the background worker.
    pub fn new() -> Result<Self> {
        let state = DownloadState::load().unwrap_or_default();
        let state = Arc::new(RwLock::new(state));
        let progress_watchers = Arc::new(RwLock::new(std::collections::HashMap::new()));

        let (command_tx, command_rx) = mpsc::channel(100);

        // Start background worker
        let worker_state = state.clone();
        let worker_watchers = progress_watchers.clone();
        tokio::spawn(async move {
            Self::worker_loop(command_rx, worker_state, worker_watchers).await;
        });

        Ok(Self {
            command_tx,
            state,
            progress_watchers,
        })
    }

    /// Queue a model for download.
    ///
    /// Returns a handle that can be used to track progress.
    pub async fn queue_download(&self, model: impl Into<String>, priority: DownloadPriority) -> Result<DownloadHandle> {
        let model = model.into();

        // Create progress watcher
        let initial_progress = DownloadProgress::new(
            &model,
            DownloadStatus::Queued,
            "Queued for download",
        );
        let (progress_tx, progress_rx) = watch::channel(initial_progress);

        // Store watcher
        {
            let mut watchers = resilient_write(&self.progress_watchers);
            watchers.insert(model.clone(), progress_tx);
        }

        // Create and store task
        {
            let mut state = resilient_write(&self.state);
            let mut task = DownloadTask::new(&model);
            task.priority = priority;
            state.upsert_task(task);
            let _ = state.save(); // Best effort save
        }

        // Send command to worker
        self.command_tx.send(DownloadCommand::Queue {
            model: model.clone(),
            priority,
        }).await?;

        Ok(DownloadHandle {
            model,
            progress_rx,
        })
    }

    /// Get the current progress for a download.
    pub fn get_progress(&self, model: &str) -> Option<DownloadProgress> {
        let watchers = resilient_read(&self.progress_watchers);
        watchers.get(model).map(|tx| tx.borrow().clone())
    }

    /// Cancel a download.
    pub async fn cancel(&self, model: impl Into<String>) -> Result<()> {
        let model = model.into();
        self.command_tx.send(DownloadCommand::Cancel { model }).await?;
        Ok(())
    }

    /// Get all active downloads.
    pub fn active_downloads(&self) -> Vec<DownloadProgress> {
        let watchers = resilient_read(&self.progress_watchers);
        watchers.values()
            .map(|tx| tx.borrow().clone())
            .filter(|p| p.status.is_active())
            .collect()
    }

    /// Get download statistics.
    pub fn stats(&self) -> (usize, usize, usize, usize) {
        let state = resilient_read(&self.state);
        state.status_counts()
    }

    /// Shutdown the download manager.
    pub async fn shutdown(&self) -> Result<()> {
        self.command_tx.send(DownloadCommand::Shutdown).await?;
        Ok(())
    }

    /// Background worker loop that processes download commands.
    async fn worker_loop(
        mut command_rx: mpsc::Receiver<DownloadCommand>,
        state: Arc<RwLock<DownloadState>>,
        watchers: Arc<RwLock<std::collections::HashMap<String, watch::Sender<DownloadProgress>>>>,
    ) {
        let client = OllamaClient::new();

        loop {
            // Check for pending downloads
            let next_download = {
                let state = resilient_read(&state);
                state.queued_tasks().first().map(|t| t.model.clone())
            };

            // Process next download if available
            if let Some(model) = next_download {
                // Update status to in-progress
                {
                    let mut state = resilient_write(&state);
                    if let Some(task) = state.get_task_mut(&model) {
                        task.update_status(DownloadStatus::InProgress {
                            bytes_downloaded: 0,
                            bytes_total: None,
                        });
                    }
                    let _ = state.save();
                }

                // Update progress watcher
                if let Some(tx) = resilient_read(&watchers).get(&model) {
                    let _ = tx.send(DownloadProgress::new(
                        &model,
                        DownloadStatus::InProgress { bytes_downloaded: 0, bytes_total: None },
                        "Starting download...",
                    ));
                }

                // Track download start time and last update for speed calculation
                let download_start = Instant::now();
                let mut _last_update = Instant::now();
                let mut _last_bytes = 0u64;

                // Perform download
                let result = client.pull_model_with_progress(&model, |progress| {
                    // Update progress
                    let status = DownloadStatus::InProgress {
                        bytes_downloaded: progress.completed.unwrap_or(0),
                        bytes_total: progress.total,
                    };

                    if let Some(tx) = resilient_read(&watchers).get(&model) {
                        let mut prog = DownloadProgress::new(&model, status.clone(), &progress.status);

                        // Calculate actual download speed
                        if let Some(completed) = progress.completed {
                            let now = Instant::now();
                            let elapsed = now.duration_since(download_start).as_secs_f64();

                            if elapsed > 0.0 {
                                // Overall average speed
                                prog.speed_bps = Some((completed as f64 / elapsed) as u64);

                                // Calculate ETA if we have total size
                                if let Some(total) = progress.total {
                                    let remaining_bytes = total.saturating_sub(completed);
                                    // IL5: Use if-let instead of unwrap_or + unwrap pattern
                                    if let Some(speed) = prog.speed_bps {
                                        if speed > 0 {
                                            let eta_secs = remaining_bytes as f64 / speed as f64;
                                            prog.eta_seconds = Some(eta_secs as u64);
                                        }
                                    }
                                }
                            }

                            _last_bytes = completed;
                            _last_update = now;
                        }

                        let _ = tx.send(prog);
                    }

                    // Update state
                    {
                        let mut state_guard = resilient_write(&state);
                        if let Some(task) = state_guard.get_task_mut(&model) {
                            task.update_status(status);
                        }
                    }
                });

                // Handle result with proper retry checking
                let final_status = match result {
                    Ok(()) => DownloadStatus::Completed { completed_at: Utc::now() },
                    Err(e) => {
                        let mut state_guard = resilient_write(&state);
                        let should_retry = if let Some(task) = state_guard.get_task_mut(&model) {
                            // Increment retry counter
                            task.retries += 1;

                            // Check if we should retry
                            if task.retries < task.max_retries {
                                // Re-queue for retry
                                task.update_status(DownloadStatus::Queued);
                                true
                            } else {
                                // Max retries exceeded
                                false
                            }
                        } else {
                            false
                        };

                        if should_retry {
                            let retries = state_guard.get_task(&model)
                                .map(|t| t.retries)
                                .unwrap_or(0);
                            drop(state_guard); // Release lock before continuing

                            // Update progress watcher for retry
                            if let Some(tx) = resilient_read(&watchers).get(&model) {
                                let _ = tx.send(DownloadProgress::new(
                                    &model,
                                    DownloadStatus::Queued,
                                    &format!("Retrying download (attempt {})...", retries + 1),
                                ));
                            }

                            // Continue to next iteration to retry
                            continue;
                        }

                        let retries = state_guard.get_task(&model)
                            .map(|t| t.retries)
                            .unwrap_or(0);

                        DownloadStatus::Failed {
                            error: e.to_string(),
                            retries,
                        }
                    }
                };

                // Update final status
                {
                    let mut state_guard = resilient_write(&state);
                    if let Some(task) = state_guard.get_task_mut(&model) {
                        task.update_status(final_status.clone());
                    }
                    let _ = state_guard.save();
                }

                // Update progress watcher
                if let Some(tx) = resilient_read(&watchers).get(&model) {
                    let message = match &final_status {
                        DownloadStatus::Completed { .. } => "Download complete!".to_string(),
                        DownloadStatus::Failed { error, .. } => format!("Download failed: {}", error),
                        _ => "Unknown status".to_string(),
                    };
                    let _ = tx.send(DownloadProgress::new(&model, final_status, message));
                }
            }

            // Process commands (non-blocking)
            tokio::select! {
                Some(cmd) = command_rx.recv() => {
                    match cmd {
                        DownloadCommand::Queue { model, priority } => {
                            // Already handled by queue_download, just ensure task exists
                            let mut state_guard = resilient_write(&state);
                            if state_guard.get_task(&model).is_none() {
                                let mut task = DownloadTask::new(&model);
                                task.priority = priority;
                                state_guard.upsert_task(task);
                                let _ = state_guard.save();
                            }
                        }
                        DownloadCommand::Cancel { model } => {
                            let mut state_guard = resilient_write(&state);
                            state_guard.remove_task(&model);
                            let _ = state_guard.save();

                            // Remove watcher
                            resilient_write(&watchers).remove(&model);
                        }
                        DownloadCommand::Pause { model } => {
                            let mut state_guard = resilient_write(&state);
                            if let Some(task) = state_guard.get_task_mut(&model) {
                                if let DownloadStatus::InProgress { bytes_downloaded, .. } = task.status {
                                    task.update_status(DownloadStatus::Paused { bytes_downloaded });
                                }
                            }
                            let _ = state_guard.save();
                        }
                        DownloadCommand::Resume { model } => {
                            let mut state_guard = resilient_write(&state);
                            if let Some(task) = state_guard.get_task_mut(&model) {
                                if matches!(task.status, DownloadStatus::Paused { .. }) {
                                    task.update_status(DownloadStatus::Queued);
                                }
                            }
                            let _ = state_guard.save();
                        }
                        DownloadCommand::Shutdown => {
                            // Cancel all in-progress downloads
                            let mut state_guard = resilient_write(&state);

                            // Mark all in-progress downloads as paused
                            for task in state_guard.tasks.values_mut() {
                                if let DownloadStatus::InProgress { bytes_downloaded, .. } = task.status {
                                    task.update_status(DownloadStatus::Paused { bytes_downloaded });
                                }
                            }

                            // Save state before exiting
                            if let Err(e) = state_guard.save() {
                                tracing::error!("Failed to save state during shutdown: {}", e);
                            }

                            // Clean up partial download files would go here
                            // (Ollama handles its own partial downloads, so we just mark as paused)

                            break;
                        }
                    }
                }
                // Small delay to prevent busy loop
                _ = tokio::time::sleep(std::time::Duration::from_millis(100)) => {}
            }
        }
    }
}

impl Default for DownloadManager {
    /// IL5 Note: This expect() is acceptable because new() only fails on
    /// fundamental system errors (channel creation failure). Callers needing
    /// error handling should use DownloadManager::new() directly.
    fn default() -> Self {
        Self::new().expect("Failed to create DownloadManager")
    }
}
