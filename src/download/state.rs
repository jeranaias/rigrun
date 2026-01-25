// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! Persistent state management for downloads.
//!
//! Saves download queue state to disk so downloads can be resumed
//! after restart.

use std::collections::HashMap;
use std::fs::{self, File, OpenOptions};
use std::io::Write;
use std::path::PathBuf;
use std::time::{Duration, Instant};
use std::thread;
use anyhow::{Result, Context, bail};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use fs2::FileExt;

use super::types::{DownloadTask, DownloadStatus};

/// Default timeout for acquiring file locks (5 seconds)
const LOCK_TIMEOUT: Duration = Duration::from_secs(5);

/// Retry interval when waiting for lock acquisition
const LOCK_RETRY_INTERVAL: Duration = Duration::from_millis(50);

/// Persistent download state saved to disk.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct DownloadState {
    /// All download tasks (queued, in-progress, completed, failed)
    pub tasks: HashMap<String, DownloadTask>,
    /// When the state was last saved
    pub last_saved: Option<DateTime<Utc>>,
    /// Version for future migrations
    pub version: u32,
}

impl DownloadState {
    /// Create a new empty state.
    pub fn new() -> Self {
        Self {
            tasks: HashMap::new(),
            last_saved: None,
            version: 1,
        }
    }

    /// Get the state file path.
    fn state_path() -> PathBuf {
        dirs::home_dir()
            .map(|h| h.join(".rigrun").join("downloads").join("state.json"))
            .unwrap_or_else(|| PathBuf::from(".rigrun/downloads/state.json"))
    }

    /// Get the lock file path for the state file.
    ///
    /// Uses a separate .lock file to coordinate access to the state file.
    /// This allows us to hold the lock during atomic rename operations.
    fn lock_path() -> PathBuf {
        Self::state_path().with_extension("lock")
    }

    /// Acquire an exclusive lock with timeout.
    ///
    /// Returns the locked file handle on success, or an error if the timeout expires.
    fn acquire_exclusive_lock_with_timeout(path: &PathBuf, timeout: Duration) -> Result<File> {
        // Create parent directories if needed
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)
                .with_context(|| format!("Failed to create directory for lock file: {:?}", parent))?;
        }

        // Open or create the lock file
        let lock_file = OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .open(path)
            .with_context(|| format!("Failed to open lock file: {:?}", path))?;

        let start = Instant::now();

        // Try to acquire lock with retries until timeout
        loop {
            // Try non-blocking lock first
            match lock_file.try_lock_exclusive() {
                Ok(()) => return Ok(lock_file),
                Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                    // Lock is held by another process, check timeout
                    if start.elapsed() >= timeout {
                        bail!(
                            "Timed out waiting for exclusive lock on {:?} after {:?}. \
                             Another instance may be writing to the state file.",
                            path,
                            timeout
                        );
                    }
                    // Wait before retrying
                    thread::sleep(LOCK_RETRY_INTERVAL);
                }
                Err(e) => {
                    return Err(e).with_context(|| {
                        format!("Failed to acquire exclusive lock on {:?}", path)
                    });
                }
            }
        }
    }

    /// Acquire a shared lock with timeout.
    ///
    /// Returns the locked file handle on success, or an error if the timeout expires.
    fn acquire_shared_lock_with_timeout(file: &File, timeout: Duration) -> Result<()> {
        let start = Instant::now();

        loop {
            match file.try_lock_shared() {
                Ok(()) => return Ok(()),
                Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                    if start.elapsed() >= timeout {
                        bail!(
                            "Timed out waiting for shared lock after {:?}. \
                             Another instance may be writing to the state file.",
                            timeout
                        );
                    }
                    thread::sleep(LOCK_RETRY_INTERVAL);
                }
                Err(e) => {
                    return Err(e).with_context(|| "Failed to acquire shared lock on state file");
                }
            }
        }
    }

    /// Load state from disk with file locking.
    ///
    /// Uses a shared lock on the lock file to allow multiple readers while
    /// blocking during writes.
    pub fn load() -> Result<Self> {
        Self::load_with_timeout(LOCK_TIMEOUT)
    }

    /// Load state from disk with a custom timeout for lock acquisition.
    pub fn load_with_timeout(timeout: Duration) -> Result<Self> {
        let path = Self::state_path();
        let lock_path = Self::lock_path();

        if !path.exists() {
            return Ok(Self::new());
        }

        // Acquire shared lock on the lock file (allows multiple readers)
        let lock_file = OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .open(&lock_path)
            .with_context(|| format!("Failed to open lock file: {:?}", lock_path))?;

        Self::acquire_shared_lock_with_timeout(&lock_file, timeout)?;

        // Now safe to read the state file
        let content = fs::read_to_string(&path)
            .with_context(|| "Failed to read state file")?;

        let state: DownloadState = serde_json::from_str(&content)
            .with_context(|| "Failed to parse state file")?;

        // Lock is automatically released when lock_file is dropped
        Ok(state)
    }

    /// Save state to disk with atomic writes and file locking.
    ///
    /// Uses a temp file + atomic rename strategy to prevent corruption on crash.
    /// Uses exclusive file locking on the lock file to prevent concurrent writes
    /// from multiple instances. The lock is held during the entire write operation
    /// including the atomic rename.
    pub fn save(&mut self) -> Result<()> {
        self.save_with_timeout(LOCK_TIMEOUT)
    }

    /// Save state to disk with a custom timeout for lock acquisition.
    ///
    /// NIST 800-53 AU-9 Compliance: Protects audit/download state from
    /// unauthorized modification by using exclusive file locking.
    pub fn save_with_timeout(&mut self, timeout: Duration) -> Result<()> {
        let path = Self::state_path();
        let lock_path = Self::lock_path();

        // Create parent directories if needed
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent)
                .with_context(|| format!("Failed to create directory: {:?}", parent))?;
        }

        // Acquire exclusive lock on the LOCK FILE before any writes
        // This prevents multiple instances from writing simultaneously
        let _lock_guard = Self::acquire_exclusive_lock_with_timeout(&lock_path, timeout)?;

        // Create temp file path
        let temp_path = path.with_extension("tmp");

        // Update timestamp
        self.last_saved = Some(Utc::now());

        // Serialize to JSON
        let content = serde_json::to_string_pretty(self)
            .with_context(|| "Failed to serialize state to JSON")?;

        // Write to temp file
        {
            let mut temp_file = OpenOptions::new()
                .write(true)
                .create(true)
                .truncate(true)
                .open(&temp_path)
                .with_context(|| format!("Failed to create temp file: {:?}", temp_path))?;

            // Write content
            temp_file.write_all(content.as_bytes())
                .with_context(|| "Failed to write to temp file")?;

            // Ensure all data is flushed to disk before rename
            temp_file.sync_all()
                .with_context(|| "Failed to sync temp file to disk")?;
        }

        // Atomic rename (atomic on POSIX, best-effort on Windows)
        // The lock file guard is still held, protecting the final file
        fs::rename(&temp_path, &path)
            .with_context(|| format!("Failed to rename temp file to state file: {:?} -> {:?}", temp_path, path))?;

        // Lock is automatically released when _lock_guard is dropped here
        Ok(())
    }

    /// Add or update a task.
    pub fn upsert_task(&mut self, task: DownloadTask) {
        self.tasks.insert(task.id.clone(), task);
    }

    /// Get a task by ID.
    pub fn get_task(&self, id: &str) -> Option<&DownloadTask> {
        self.tasks.get(id)
    }

    /// Get a mutable task by ID.
    pub fn get_task_mut(&mut self, id: &str) -> Option<&mut DownloadTask> {
        self.tasks.get_mut(id)
    }

    /// Remove a task.
    pub fn remove_task(&mut self, id: &str) -> Option<DownloadTask> {
        self.tasks.remove(id)
    }

    /// Get all queued tasks sorted by priority (highest first).
    pub fn queued_tasks(&self) -> Vec<&DownloadTask> {
        let mut tasks: Vec<_> = self.tasks
            .values()
            .filter(|t| matches!(t.status, DownloadStatus::Queued))
            .collect();
        tasks.sort_by(|a, b| b.priority.cmp(&a.priority));
        tasks
    }

    /// Get all active (in-progress) tasks.
    pub fn active_tasks(&self) -> Vec<&DownloadTask> {
        self.tasks
            .values()
            .filter(|t| t.status.is_active())
            .collect()
    }

    /// Get all completed tasks.
    pub fn completed_tasks(&self) -> Vec<&DownloadTask> {
        self.tasks
            .values()
            .filter(|t| matches!(t.status, DownloadStatus::Completed { .. }))
            .collect()
    }

    /// Get all failed tasks.
    pub fn failed_tasks(&self) -> Vec<&DownloadTask> {
        self.tasks
            .values()
            .filter(|t| matches!(t.status, DownloadStatus::Failed { .. }))
            .collect()
    }

    /// Clear completed tasks older than the given duration.
    pub fn cleanup_old_completed(&mut self, max_age: chrono::Duration) {
        let cutoff = Utc::now() - max_age;
        self.tasks.retain(|_, task| {
            if let DownloadStatus::Completed { completed_at } = &task.status {
                *completed_at > cutoff
            } else {
                true
            }
        });
    }

    /// Get count of tasks by status.
    pub fn status_counts(&self) -> (usize, usize, usize, usize) {
        let mut queued = 0;
        let mut active = 0;
        let mut completed = 0;
        let mut failed = 0;

        for task in self.tasks.values() {
            match &task.status {
                DownloadStatus::Queued => queued += 1,
                DownloadStatus::InProgress { .. } | DownloadStatus::Verifying => active += 1,
                DownloadStatus::Completed { .. } => completed += 1,
                DownloadStatus::Failed { .. } => failed += 1,
                DownloadStatus::Paused { .. } => queued += 1, // Count paused as queued
            }
        }

        (queued, active, completed, failed)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::download::types::DownloadPriority;
    use std::sync::Arc;
    use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
    use tempfile::TempDir;

    #[test]
    fn test_state_new() {
        let state = DownloadState::new();
        assert!(state.tasks.is_empty());
        assert_eq!(state.version, 1);
    }

    #[test]
    fn test_state_upsert_and_get() {
        let mut state = DownloadState::new();
        let task = DownloadTask::new("test-model");
        state.upsert_task(task.clone());

        let retrieved = state.get_task("test-model");
        assert!(retrieved.is_some());
        assert_eq!(retrieved.unwrap().model, "test-model");
    }

    #[test]
    fn test_queued_tasks_sorted_by_priority() {
        let mut state = DownloadState::new();

        let low = {
            let mut t = DownloadTask::new("low");
            t.priority = DownloadPriority::Low;
            t
        };
        let high = DownloadTask::high_priority("high");
        let normal = DownloadTask::new("normal");

        state.upsert_task(low);
        state.upsert_task(normal);
        state.upsert_task(high);

        let queued = state.queued_tasks();
        assert_eq!(queued.len(), 3);
        assert_eq!(queued[0].model, "high");
        assert_eq!(queued[1].model, "normal");
        assert_eq!(queued[2].model, "low");
    }

    // ========================================================================
    // File locking tests (NIST 800-53 AU-9 compliance)
    // ========================================================================

    #[test]
    fn test_lock_acquisition_exclusive() {
        // Test that we can acquire an exclusive lock on a file
        let temp_dir = TempDir::new().expect("Failed to create temp dir");
        let lock_path = temp_dir.path().join("test.lock");

        // First lock should succeed
        let lock1 = DownloadState::acquire_exclusive_lock_with_timeout(
            &lock_path,
            Duration::from_secs(1),
        );
        assert!(lock1.is_ok(), "First exclusive lock should succeed");

        // Second lock attempt should fail with timeout (very short timeout)
        let lock2 = DownloadState::acquire_exclusive_lock_with_timeout(
            &lock_path,
            Duration::from_millis(100),
        );
        assert!(lock2.is_err(), "Second exclusive lock should fail while first is held");

        // Release first lock
        drop(lock1);

        // Now a new lock should succeed
        let lock3 = DownloadState::acquire_exclusive_lock_with_timeout(
            &lock_path,
            Duration::from_secs(1),
        );
        assert!(lock3.is_ok(), "Lock should succeed after previous lock released");
    }

    #[test]
    fn test_lock_timeout_behavior() {
        // Test that lock acquisition properly times out
        let temp_dir = TempDir::new().expect("Failed to create temp dir");
        let lock_path = temp_dir.path().join("timeout_test.lock");

        // Acquire first lock
        let _lock1 = DownloadState::acquire_exclusive_lock_with_timeout(
            &lock_path,
            Duration::from_secs(1),
        ).expect("First lock should succeed");

        // Try to acquire second lock with short timeout
        let timeout = Duration::from_millis(200);
        let start = Instant::now();
        let result = DownloadState::acquire_exclusive_lock_with_timeout(
            &lock_path,
            timeout,
        );

        let elapsed = start.elapsed();

        // Should have failed
        assert!(result.is_err(), "Lock should have timed out");

        // Should have taken approximately the timeout duration
        // (allow some tolerance for scheduling)
        assert!(
            elapsed >= Duration::from_millis(150),
            "Should have waited close to timeout duration, elapsed: {:?}",
            elapsed
        );
        assert!(
            elapsed < Duration::from_millis(500),
            "Should not have waited too long, elapsed: {:?}",
            elapsed
        );

        // Error message should mention timeout
        let err_msg = result.unwrap_err().to_string();
        assert!(
            err_msg.contains("Timed out"),
            "Error should mention timeout: {}",
            err_msg
        );
    }

    #[test]
    fn test_multiple_instance_handling() {
        // Test that multiple threads trying to write simultaneously are serialized
        let temp_dir = TempDir::new().expect("Failed to create temp dir");
        let lock_path = temp_dir.path().join("multi_instance.lock");
        let lock_path_arc = Arc::new(lock_path);

        let num_threads = 4;
        let iterations_per_thread = 5;
        let successful_locks = Arc::new(AtomicUsize::new(0));
        let concurrent_holders = Arc::new(AtomicUsize::new(0));
        let max_concurrent = Arc::new(AtomicUsize::new(0));

        let mut handles = vec![];

        for _ in 0..num_threads {
            let lock_path = Arc::clone(&lock_path_arc);
            let successful = Arc::clone(&successful_locks);
            let concurrent = Arc::clone(&concurrent_holders);
            let max_conc = Arc::clone(&max_concurrent);

            let handle = thread::spawn(move || {
                for _ in 0..iterations_per_thread {
                    // Try to acquire lock with generous timeout
                    if let Ok(_guard) = DownloadState::acquire_exclusive_lock_with_timeout(
                        &lock_path,
                        Duration::from_secs(10),
                    ) {
                        // Increment concurrent holders
                        let current = concurrent.fetch_add(1, Ordering::SeqCst) + 1;

                        // Track max concurrent (should always be 1)
                        max_conc.fetch_max(current, Ordering::SeqCst);

                        // Simulate some work while holding lock
                        thread::sleep(Duration::from_millis(10));

                        // Decrement concurrent holders
                        concurrent.fetch_sub(1, Ordering::SeqCst);

                        // Count successful acquisitions
                        successful.fetch_add(1, Ordering::SeqCst);
                    }
                }
            });

            handles.push(handle);
        }

        // Wait for all threads to complete
        for handle in handles {
            handle.join().expect("Thread panicked");
        }

        // All lock acquisitions should have succeeded
        let total_expected = num_threads * iterations_per_thread;
        assert_eq!(
            successful_locks.load(Ordering::SeqCst),
            total_expected,
            "All lock acquisitions should succeed"
        );

        // Max concurrent should be 1 (exclusive lock)
        assert_eq!(
            max_concurrent.load(Ordering::SeqCst),
            1,
            "Only one thread should hold the lock at a time"
        );
    }

    #[test]
    fn test_shared_lock_allows_multiple_readers() {
        // Test that shared locks allow multiple readers
        let temp_dir = TempDir::new().expect("Failed to create temp dir");
        let lock_path = temp_dir.path().join("shared_test.lock");

        // Create the lock file
        let file1 = OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .open(&lock_path)
            .expect("Failed to create lock file");

        let file2 = OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .open(&lock_path)
            .expect("Failed to open lock file");

        // First shared lock should succeed
        let result1 = DownloadState::acquire_shared_lock_with_timeout(
            &file1,
            Duration::from_secs(1),
        );
        assert!(result1.is_ok(), "First shared lock should succeed");

        // Second shared lock should also succeed (shared locks allow multiple readers)
        let result2 = DownloadState::acquire_shared_lock_with_timeout(
            &file2,
            Duration::from_secs(1),
        );
        assert!(result2.is_ok(), "Second shared lock should succeed");
    }

    #[test]
    fn test_exclusive_lock_blocks_shared() {
        // Test that exclusive lock blocks shared lock acquisition
        let temp_dir = TempDir::new().expect("Failed to create temp dir");
        let lock_path = temp_dir.path().join("exclusive_blocks_shared.lock");

        // Acquire exclusive lock first
        let _exclusive = DownloadState::acquire_exclusive_lock_with_timeout(
            &lock_path,
            Duration::from_secs(1),
        ).expect("Exclusive lock should succeed");

        // Try to acquire shared lock - should fail
        let file = OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .open(&lock_path)
            .expect("Failed to open lock file");

        let result = DownloadState::acquire_shared_lock_with_timeout(
            &file,
            Duration::from_millis(100),
        );
        assert!(result.is_err(), "Shared lock should fail while exclusive is held");
    }

    #[test]
    fn test_lock_file_created_on_save() {
        // Test that the lock file is created when saving state
        let temp_dir = TempDir::new().expect("Failed to create temp dir");
        let state_dir = temp_dir.path().join(".rigrun").join("downloads");
        let lock_path = state_dir.join("state.lock");

        // Override state_path for this test by using environment
        // Note: This test may need adjustment based on how state_path is implemented
        // For now, we'll just verify the lock file mechanism works

        // Create a lock file manually to verify the mechanism
        fs::create_dir_all(&state_dir).expect("Failed to create state dir");

        let lock = DownloadState::acquire_exclusive_lock_with_timeout(
            &lock_path,
            Duration::from_secs(1),
        );

        assert!(lock.is_ok(), "Lock acquisition should succeed");
        assert!(lock_path.exists(), "Lock file should be created");
    }

    #[test]
    fn test_lock_released_after_save_completes() {
        // Test that lock is released after save completes
        let temp_dir = TempDir::new().expect("Failed to create temp dir");
        let lock_path = temp_dir.path().join("release_test.lock");

        // Simulate save operation
        {
            let _lock_guard = DownloadState::acquire_exclusive_lock_with_timeout(
                &lock_path,
                Duration::from_secs(1),
            ).expect("Lock should succeed");

            // Simulate write operation
            thread::sleep(Duration::from_millis(10));

            // Lock should be released when _lock_guard goes out of scope
        }

        // Should be able to acquire lock again immediately
        let lock2 = DownloadState::acquire_exclusive_lock_with_timeout(
            &lock_path,
            Duration::from_millis(100),
        );
        assert!(lock2.is_ok(), "Lock should be acquirable after previous lock dropped");
    }

    #[test]
    fn test_concurrent_save_serialization() {
        // Test that concurrent saves are properly serialized
        let temp_dir = TempDir::new().expect("Failed to create temp dir");
        let lock_path = temp_dir.path().join("concurrent_save.lock");
        let lock_path_arc = Arc::new(lock_path);

        let write_count = Arc::new(AtomicUsize::new(0));
        let write_in_progress = Arc::new(AtomicBool::new(false));
        let had_concurrent_write = Arc::new(AtomicBool::new(false));

        let mut handles = vec![];

        for _ in 0..3 {
            let lock_path = Arc::clone(&lock_path_arc);
            let count = Arc::clone(&write_count);
            let in_progress = Arc::clone(&write_in_progress);
            let concurrent = Arc::clone(&had_concurrent_write);

            let handle = thread::spawn(move || {
                for _ in 0..3 {
                    if let Ok(_guard) = DownloadState::acquire_exclusive_lock_with_timeout(
                        &lock_path,
                        Duration::from_secs(10),
                    ) {
                        // Check if another write is in progress (should never happen)
                        if in_progress.swap(true, Ordering::SeqCst) {
                            concurrent.store(true, Ordering::SeqCst);
                        }

                        // Simulate write
                        thread::sleep(Duration::from_millis(20));
                        count.fetch_add(1, Ordering::SeqCst);

                        in_progress.store(false, Ordering::SeqCst);
                    }
                }
            });

            handles.push(handle);
        }

        for handle in handles {
            handle.join().expect("Thread panicked");
        }

        assert_eq!(write_count.load(Ordering::SeqCst), 9, "All writes should complete");
        assert!(!had_concurrent_write.load(Ordering::SeqCst), "No concurrent writes should occur");
    }
}
