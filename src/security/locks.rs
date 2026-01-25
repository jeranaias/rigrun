// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! Resilient Lock Helpers for IL5 Compliance
//!
//! This module provides lock helper functions that recover from lock poisoning
//! instead of panicking. Lock poisoning occurs when a thread panics while
//! holding a lock, marking the lock as "poisoned" to prevent other threads
//! from accessing potentially corrupted data.
//!
//! ## IL5 Requirement
//!
//! Per NIST 800-53 SI-11 (Error Handling), applications must not terminate
//! unexpectedly due to recoverable errors. Lock poisoning is a recoverable
//! condition where we can choose to accept the potentially-stale data rather
//! than causing a denial of service.
//!
//! ## Usage
//!
//! ```no_run
//! use std::sync::RwLock;
//! use rigrun::security::locks::{resilient_read, resilient_write};
//!
//! let lock = RwLock::new(42);
//!
//! // Read with recovery
//! let guard = resilient_read(&lock);
//! println!("Value: {}", *guard);
//!
//! // Write with recovery
//! let mut guard = resilient_write(&lock);
//! *guard = 100;
//! ```
//!
//! ## Security Considerations
//!
//! When a lock is poisoned, we log a CRITICAL security event and recover
//! the guard anyway. The data may be in an inconsistent state, but:
//! 1. We avoid denial of service from the panic
//! 2. We log the event for security audit
//! 3. In most cases, stale data is preferable to service unavailability

use std::sync::{RwLock, RwLockReadGuard, RwLockWriteGuard};

/// Acquire a read lock, recovering from poisoning if necessary.
///
/// If the lock is poisoned (a thread panicked while holding the write lock),
/// this function logs a security event and recovers the guard anyway.
///
/// # Arguments
///
/// * `lock` - The RwLock to acquire
///
/// # Returns
///
/// A read guard for the lock.
///
/// # IL5 Compliance
///
/// This function satisfies SI-11 by never panicking on lock acquisition.
#[inline]
pub fn resilient_read<T>(lock: &RwLock<T>) -> RwLockReadGuard<'_, T> {
    match lock.read() {
        Ok(guard) => guard,
        Err(poisoned) => {
            tracing::error!(
                target: "security::locks",
                event = "LOCK_POISONED_READ",
                "CRITICAL: RwLock was poisoned during read acquisition. Recovering data. \
                 A thread previously panicked while holding this lock. \
                 Data may be inconsistent. Investigate panic cause in logs."
            );
            poisoned.into_inner()
        }
    }
}

/// Acquire a write lock, recovering from poisoning if necessary.
///
/// If the lock is poisoned (a thread panicked while holding the lock),
/// this function logs a security event and recovers the guard anyway.
///
/// # Arguments
///
/// * `lock` - The RwLock to acquire
///
/// # Returns
///
/// A write guard for the lock.
///
/// # IL5 Compliance
///
/// This function satisfies SI-11 by never panicking on lock acquisition.
#[inline]
pub fn resilient_write<T>(lock: &RwLock<T>) -> RwLockWriteGuard<'_, T> {
    match lock.write() {
        Ok(guard) => guard,
        Err(poisoned) => {
            tracing::error!(
                target: "security::locks",
                event = "LOCK_POISONED_WRITE",
                "CRITICAL: RwLock was poisoned during write acquisition. Recovering data. \
                 A thread previously panicked while holding this lock. \
                 Data may be inconsistent. Investigate panic cause in logs."
            );
            poisoned.into_inner()
        }
    }
}

/// Try to acquire a read lock without blocking.
///
/// Returns `Some(guard)` if the lock can be acquired immediately,
/// `None` if it would block. Recovers from poisoning.
#[inline]
pub fn try_resilient_read<T>(lock: &RwLock<T>) -> Option<RwLockReadGuard<'_, T>> {
    match lock.try_read() {
        Ok(guard) => Some(guard),
        Err(std::sync::TryLockError::Poisoned(poisoned)) => {
            tracing::error!(
                target: "security::locks",
                event = "LOCK_POISONED_TRY_READ",
                "CRITICAL: RwLock was poisoned during try_read. Recovering data."
            );
            Some(poisoned.into_inner())
        }
        Err(std::sync::TryLockError::WouldBlock) => None,
    }
}

/// Try to acquire a write lock without blocking.
///
/// Returns `Some(guard)` if the lock can be acquired immediately,
/// `None` if it would block. Recovers from poisoning.
#[inline]
pub fn try_resilient_write<T>(lock: &RwLock<T>) -> Option<RwLockWriteGuard<'_, T>> {
    match lock.try_write() {
        Ok(guard) => Some(guard),
        Err(std::sync::TryLockError::Poisoned(poisoned)) => {
            tracing::error!(
                target: "security::locks",
                event = "LOCK_POISONED_TRY_WRITE",
                "CRITICAL: RwLock was poisoned during try_write. Recovering data."
            );
            Some(poisoned.into_inner())
        }
        Err(std::sync::TryLockError::WouldBlock) => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;
    use std::thread;

    #[test]
    fn test_resilient_read_normal() {
        let lock = RwLock::new(42);
        let guard = resilient_read(&lock);
        assert_eq!(*guard, 42);
    }

    #[test]
    fn test_resilient_write_normal() {
        let lock = RwLock::new(42);
        {
            let mut guard = resilient_write(&lock);
            *guard = 100;
        }
        let guard = resilient_read(&lock);
        assert_eq!(*guard, 100);
    }

    #[test]
    fn test_resilient_read_poisoned() {
        let lock = Arc::new(RwLock::new(42));
        let lock_clone = Arc::clone(&lock);

        // Poison the lock by panicking while holding it
        let handle = thread::spawn(move || {
            let _guard = lock_clone.write().unwrap();
            panic!("intentional panic to poison lock");
        });
        let _ = handle.join(); // Ignore the panic

        // Should recover instead of panicking
        let guard = resilient_read(&lock);
        assert_eq!(*guard, 42);
    }

    #[test]
    fn test_resilient_write_poisoned() {
        let lock = Arc::new(RwLock::new(42));
        let lock_clone = Arc::clone(&lock);

        // Poison the lock
        let handle = thread::spawn(move || {
            let _guard = lock_clone.write().unwrap();
            panic!("intentional panic to poison lock");
        });
        let _ = handle.join();

        // Should recover and allow writes
        let mut guard = resilient_write(&lock);
        *guard = 100;
        drop(guard);

        let guard = resilient_read(&lock);
        assert_eq!(*guard, 100);
    }

    #[test]
    fn test_try_resilient_read() {
        let lock = RwLock::new(42);
        let guard = try_resilient_read(&lock);
        assert!(guard.is_some());
        assert_eq!(*guard.unwrap(), 42);
    }

    #[test]
    fn test_try_resilient_write() {
        let lock = RwLock::new(42);
        let guard = try_resilient_write(&lock);
        assert!(guard.is_some());
    }
}
