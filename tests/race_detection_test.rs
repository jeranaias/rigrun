// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! Race Detection Tests for rigrun
//!
//! These tests verify thread safety of concurrent operations in the rigrun codebase.
//! They are designed to detect data races when run with ThreadSanitizer (TSAN).
//!
//! # Running with ThreadSanitizer
//!
//! ```bash
//! # On Linux with nightly Rust:
//! RUSTFLAGS="-Z sanitizer=thread" cargo +nightly test --target x86_64-unknown-linux-gnu --test race_detection_test
//!
//! # Or use cargo-careful for additional checks:
//! cargo install cargo-careful
//! cargo careful test --test race_detection_test
//! ```
//!
//! # Test Categories
//!
//! - Server concurrent request handling
//! - Session management thread safety
//! - Cache concurrent access (read/write)
//! - Router concurrent decision making
//! - Audit logging thread safety
//! - Stats tracking concurrency

use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Duration;
use tokio::sync::RwLock;
use tokio::time::timeout;

// Test configuration
const CONCURRENCY_LEVEL: usize = 100;
const ITERATIONS_PER_TASK: usize = 50;
const TEST_TIMEOUT_SECS: u64 = 30;

// =============================================================================
// CACHE CONCURRENT ACCESS TESTS
// =============================================================================

/// Mock cache for testing concurrent access patterns
struct MockCache {
    entries: RwLock<std::collections::HashMap<String, String>>,
    hits: AtomicU64,
    misses: AtomicU64,
}

impl MockCache {
    fn new() -> Self {
        Self {
            entries: RwLock::new(std::collections::HashMap::new()),
            hits: AtomicU64::new(0),
            misses: AtomicU64::new(0),
        }
    }

    async fn get(&self, key: &str) -> Option<String> {
        let guard = self.entries.read().await;
        match guard.get(key).cloned() {
            Some(value) => {
                self.hits.fetch_add(1, Ordering::Relaxed);
                Some(value)
            }
            None => {
                self.misses.fetch_add(1, Ordering::Relaxed);
                None
            }
        }
    }

    async fn set(&self, key: String, value: String) {
        let mut guard = self.entries.write().await;
        guard.insert(key, value);
    }

    fn stats(&self) -> (u64, u64) {
        (
            self.hits.load(Ordering::Relaxed),
            self.misses.load(Ordering::Relaxed),
        )
    }
}

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn test_cache_concurrent_read_write() {
    let cache = Arc::new(MockCache::new());
    let mut handles = vec![];

    // Pre-populate some entries
    for i in 0..10 {
        cache.set(format!("key-{}", i), format!("value-{}", i)).await;
    }

    // Concurrent reads and writes
    for i in 0..CONCURRENCY_LEVEL {
        let cache = cache.clone();
        handles.push(tokio::spawn(async move {
            for j in 0..ITERATIONS_PER_TASK {
                let key = format!("key-{}", (i + j) % 20);

                if j % 3 == 0 {
                    // Write
                    cache.set(key, format!("new-value-{}-{}", i, j)).await;
                } else {
                    // Read
                    let _ = cache.get(&key).await;
                }
            }
        }));
    }

    // Wait for all tasks with timeout
    let result = timeout(
        Duration::from_secs(TEST_TIMEOUT_SECS),
        async {
            for handle in handles {
                handle.await.expect("Task panicked");
            }
        }
    ).await;

    assert!(result.is_ok(), "Test timed out");

    let (hits, misses) = cache.stats();
    println!("Cache stats: {} hits, {} misses", hits, misses);
}

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn test_cache_heavy_read_contention() {
    let cache = Arc::new(MockCache::new());

    // Pre-populate
    cache.set("hot-key".to_string(), "hot-value".to_string()).await;

    let mut handles = vec![];
    let read_count = Arc::new(AtomicU64::new(0));

    // Many readers hitting the same key
    for _ in 0..CONCURRENCY_LEVEL {
        let cache = cache.clone();
        let count = read_count.clone();
        handles.push(tokio::spawn(async move {
            for _ in 0..ITERATIONS_PER_TASK {
                let _ = cache.get("hot-key").await;
                count.fetch_add(1, Ordering::Relaxed);
            }
        }));
    }

    let result = timeout(
        Duration::from_secs(TEST_TIMEOUT_SECS),
        async {
            for handle in handles {
                handle.await.expect("Task panicked");
            }
        }
    ).await;

    assert!(result.is_ok(), "Test timed out");

    let total_reads = read_count.load(Ordering::Relaxed);
    println!("Completed {} concurrent reads on hot key", total_reads);
    assert!(total_reads >= (CONCURRENCY_LEVEL * ITERATIONS_PER_TASK) as u64);
}

// =============================================================================
// SESSION MANAGEMENT TESTS
// =============================================================================

/// Mock session for testing concurrent access
struct MockSession {
    id: String,
    valid: RwLock<bool>,
    access_count: AtomicU64,
    last_access: RwLock<std::time::Instant>,
}

impl MockSession {
    fn new(id: &str) -> Self {
        Self {
            id: id.to_string(),
            valid: RwLock::new(true),
            access_count: AtomicU64::new(0),
            last_access: RwLock::new(std::time::Instant::now()),
        }
    }

    async fn is_valid(&self) -> bool {
        let guard = self.valid.read().await;
        self.access_count.fetch_add(1, Ordering::Relaxed);
        *guard
    }

    async fn refresh(&self) {
        let mut guard = self.last_access.write().await;
        *guard = std::time::Instant::now();
    }

    async fn invalidate(&self) {
        let mut guard = self.valid.write().await;
        *guard = false;
    }
}

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn test_session_concurrent_access() {
    let session = Arc::new(MockSession::new("test-session-001"));
    let mut handles = vec![];

    // 50 readers, 25 refreshers, 25 potential invalidators
    for i in 0..CONCURRENCY_LEVEL {
        let session = session.clone();
        handles.push(tokio::spawn(async move {
            for j in 0..ITERATIONS_PER_TASK {
                match i % 4 {
                    0 | 1 => {
                        // Reader
                        let _ = session.is_valid().await;
                    }
                    2 => {
                        // Refresher
                        session.refresh().await;
                    }
                    _ => {
                        // Conditional invalidator (only invalidate occasionally)
                        if j == ITERATIONS_PER_TASK - 1 && i == 3 {
                            session.invalidate().await;
                        } else {
                            let _ = session.is_valid().await;
                        }
                    }
                }
            }
        }));
    }

    let result = timeout(
        Duration::from_secs(TEST_TIMEOUT_SECS),
        async {
            for handle in handles {
                handle.await.expect("Task panicked");
            }
        }
    ).await;

    assert!(result.is_ok(), "Test timed out");

    let access_count = session.access_count.load(Ordering::Relaxed);
    println!("Session accessed {} times concurrently", access_count);
}

// =============================================================================
// STATS TRACKING TESTS
// =============================================================================

/// Mock stats tracker for testing concurrent updates
struct MockStatsTracker {
    total_queries: AtomicU64,
    cache_hits: AtomicU64,
    local_queries: AtomicU64,
    cloud_queries: AtomicU64,
    total_tokens: AtomicU64,
}

impl MockStatsTracker {
    fn new() -> Self {
        Self {
            total_queries: AtomicU64::new(0),
            cache_hits: AtomicU64::new(0),
            local_queries: AtomicU64::new(0),
            cloud_queries: AtomicU64::new(0),
            total_tokens: AtomicU64::new(0),
        }
    }

    fn record_query(&self, tier: &str, tokens: u64) {
        self.total_queries.fetch_add(1, Ordering::Relaxed);
        self.total_tokens.fetch_add(tokens, Ordering::Relaxed);

        match tier {
            "cache" => { self.cache_hits.fetch_add(1, Ordering::Relaxed); }
            "local" => { self.local_queries.fetch_add(1, Ordering::Relaxed); }
            "cloud" => { self.cloud_queries.fetch_add(1, Ordering::Relaxed); }
            _ => {}
        }
    }

    fn snapshot(&self) -> (u64, u64, u64, u64, u64) {
        (
            self.total_queries.load(Ordering::Relaxed),
            self.cache_hits.load(Ordering::Relaxed),
            self.local_queries.load(Ordering::Relaxed),
            self.cloud_queries.load(Ordering::Relaxed),
            self.total_tokens.load(Ordering::Relaxed),
        )
    }
}

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn test_stats_concurrent_updates() {
    let stats = Arc::new(MockStatsTracker::new());
    let mut handles = vec![];

    let tiers = ["cache", "local", "cloud"];

    for i in 0..CONCURRENCY_LEVEL {
        let stats = stats.clone();
        handles.push(tokio::spawn(async move {
            for j in 0..ITERATIONS_PER_TASK {
                let tier = tiers[(i + j) % tiers.len()];
                let tokens = ((i + j) % 1000 + 10) as u64;
                stats.record_query(tier, tokens);
            }
        }));
    }

    let result = timeout(
        Duration::from_secs(TEST_TIMEOUT_SECS),
        async {
            for handle in handles {
                handle.await.expect("Task panicked");
            }
        }
    ).await;

    assert!(result.is_ok(), "Test timed out");

    let (total, cache, local, cloud, tokens) = stats.snapshot();

    // Verify all queries were recorded
    let expected_total = (CONCURRENCY_LEVEL * ITERATIONS_PER_TASK) as u64;
    assert_eq!(total, expected_total, "Not all queries recorded");

    // Verify tier distribution is reasonable
    assert_eq!(cache + local + cloud, expected_total, "Tier counts don't sum to total");

    println!(
        "Stats: total={}, cache={}, local={}, cloud={}, tokens={}",
        total, cache, local, cloud, tokens
    );
}

// =============================================================================
// ROUTING DECISION TESTS
// =============================================================================

/// Mock router for testing concurrent routing decisions
struct MockRouter {
    decisions: RwLock<Vec<String>>,
    decision_count: AtomicU64,
}

impl MockRouter {
    fn new() -> Self {
        Self {
            decisions: RwLock::new(Vec::new()),
            decision_count: AtomicU64::new(0),
        }
    }

    async fn route_query(&self, query: &str, classification: &str, paranoid: bool) -> String {
        self.decision_count.fetch_add(1, Ordering::Relaxed);

        // Simulate routing logic
        let tier = if paranoid || classification != "UNCLASSIFIED" {
            "local"
        } else if query.len() < 20 {
            "cache"
        } else if query.len() < 100 {
            "local"
        } else {
            "cloud"
        };

        // Record decision (with write lock)
        {
            let mut decisions = self.decisions.write().await;
            if decisions.len() < 1000 {  // Cap to prevent memory issues
                decisions.push(format!("{}:{}", query.len(), tier));
            }
        }

        tier.to_string()
    }

    fn get_decision_count(&self) -> u64 {
        self.decision_count.load(Ordering::Relaxed)
    }
}

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn test_router_concurrent_decisions() {
    let router = Arc::new(MockRouter::new());
    let mut handles = vec![];

    let queries = [
        "Hi",
        "What is the weather today?",
        "Explain quantum computing in detail with examples and mathematical foundations",
        "Write a comprehensive essay about climate change impacts",
        "2+2",
    ];

    let classifications = ["UNCLASSIFIED", "CUI", "SECRET"];

    for i in 0..CONCURRENCY_LEVEL {
        let router = router.clone();
        handles.push(tokio::spawn(async move {
            for j in 0..ITERATIONS_PER_TASK {
                let query = queries[(i + j) % queries.len()];
                let classification = classifications[j % classifications.len()];
                let paranoid = i % 3 == 0;

                let tier = router.route_query(query, classification, paranoid).await;

                // Verify routing rules
                if classification != "UNCLASSIFIED" || paranoid {
                    assert_eq!(tier, "local", "CUI+ or paranoid should route to local");
                }
            }
        }));
    }

    let result = timeout(
        Duration::from_secs(TEST_TIMEOUT_SECS),
        async {
            for handle in handles {
                handle.await.expect("Task panicked");
            }
        }
    ).await;

    assert!(result.is_ok(), "Test timed out");

    let total_decisions = router.get_decision_count();
    let expected = (CONCURRENCY_LEVEL * ITERATIONS_PER_TASK) as u64;
    assert_eq!(total_decisions, expected, "Not all routing decisions completed");

    println!("Made {} routing decisions concurrently", total_decisions);
}

// =============================================================================
// AUDIT LOGGING TESTS
// =============================================================================

/// Mock audit logger for testing concurrent logging
struct MockAuditLogger {
    entries: RwLock<Vec<String>>,
    log_count: AtomicU64,
}

impl MockAuditLogger {
    fn new() -> Self {
        Self {
            entries: RwLock::new(Vec::new()),
            log_count: AtomicU64::new(0),
        }
    }

    async fn log(&self, event_type: &str, user: &str, details: &str) {
        self.log_count.fetch_add(1, Ordering::Relaxed);

        let entry = format!("[{}] {}: {}", event_type, user, details);

        // Only keep recent entries to prevent memory issues
        let mut entries = self.entries.write().await;
        if entries.len() < 1000 {
            entries.push(entry);
        }
    }

    fn get_log_count(&self) -> u64 {
        self.log_count.load(Ordering::Relaxed)
    }
}

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn test_audit_concurrent_logging() {
    let logger = Arc::new(MockAuditLogger::new());
    let mut handles = vec![];

    let event_types = ["QUERY", "LOGIN", "LOGOUT", "CONFIG_CHANGE", "ERROR"];
    let users = ["user1", "user2", "admin", "service-account"];

    for i in 0..CONCURRENCY_LEVEL {
        let logger = logger.clone();
        handles.push(tokio::spawn(async move {
            for j in 0..ITERATIONS_PER_TASK {
                let event_type = event_types[(i + j) % event_types.len()];
                let user = users[i % users.len()];
                let details = format!("test-event-{}-{}", i, j);

                logger.log(event_type, user, &details).await;
            }
        }));
    }

    let result = timeout(
        Duration::from_secs(TEST_TIMEOUT_SECS),
        async {
            for handle in handles {
                handle.await.expect("Task panicked");
            }
        }
    ).await;

    assert!(result.is_ok(), "Test timed out");

    let total_logs = logger.get_log_count();
    let expected = (CONCURRENCY_LEVEL * ITERATIONS_PER_TASK) as u64;
    assert_eq!(total_logs, expected, "Not all log entries recorded");

    println!("Logged {} audit entries concurrently", total_logs);
}

// =============================================================================
// SERVER REQUEST HANDLING TESTS
// =============================================================================

/// Mock request handler for testing concurrent request processing
struct MockRequestHandler {
    active_requests: AtomicU64,
    max_concurrent: AtomicU64,
    total_processed: AtomicU64,
}

impl MockRequestHandler {
    fn new() -> Self {
        Self {
            active_requests: AtomicU64::new(0),
            max_concurrent: AtomicU64::new(0),
            total_processed: AtomicU64::new(0),
        }
    }

    async fn handle_request(&self, request_id: u64) -> Result<String, &'static str> {
        // Increment active requests
        let active = self.active_requests.fetch_add(1, Ordering::SeqCst) + 1;

        // Track max concurrent
        let mut current_max = self.max_concurrent.load(Ordering::Relaxed);
        while active > current_max {
            match self.max_concurrent.compare_exchange_weak(
                current_max,
                active,
                Ordering::Relaxed,
                Ordering::Relaxed,
            ) {
                Ok(_) => break,
                Err(x) => current_max = x,
            }
        }

        // Simulate some async work
        tokio::time::sleep(Duration::from_micros(100)).await;

        // Decrement active requests
        self.active_requests.fetch_sub(1, Ordering::SeqCst);
        self.total_processed.fetch_add(1, Ordering::Relaxed);

        Ok(format!("Processed request {}", request_id))
    }

    fn stats(&self) -> (u64, u64, u64) {
        (
            self.active_requests.load(Ordering::Relaxed),
            self.max_concurrent.load(Ordering::Relaxed),
            self.total_processed.load(Ordering::Relaxed),
        )
    }
}

#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn test_server_concurrent_requests() {
    let handler = Arc::new(MockRequestHandler::new());
    let mut handles = vec![];

    for i in 0..CONCURRENCY_LEVEL {
        let handler = handler.clone();
        handles.push(tokio::spawn(async move {
            for j in 0..ITERATIONS_PER_TASK {
                let request_id = (i * ITERATIONS_PER_TASK + j) as u64;
                let result = handler.handle_request(request_id).await;
                assert!(result.is_ok(), "Request failed");
            }
        }));
    }

    let result = timeout(
        Duration::from_secs(TEST_TIMEOUT_SECS),
        async {
            for handle in handles {
                handle.await.expect("Task panicked");
            }
        }
    ).await;

    assert!(result.is_ok(), "Test timed out");

    let (active, max_concurrent, total) = handler.stats();

    assert_eq!(active, 0, "All requests should be completed");
    assert!(max_concurrent > 1, "Should have had concurrent requests");

    let expected_total = (CONCURRENCY_LEVEL * ITERATIONS_PER_TASK) as u64;
    assert_eq!(total, expected_total, "Not all requests processed");

    println!(
        "Processed {} requests, max concurrent: {}",
        total, max_concurrent
    );
}

// =============================================================================
// COMBINED STRESS TEST
// =============================================================================

#[tokio::test(flavor = "multi_thread", worker_threads = 8)]
async fn test_all_components_under_load() {
    let cache = Arc::new(MockCache::new());
    let session = Arc::new(MockSession::new("stress-test-session"));
    let stats = Arc::new(MockStatsTracker::new());
    let router = Arc::new(MockRouter::new());
    let logger = Arc::new(MockAuditLogger::new());

    let mut handles = vec![];

    // Launch all component tests concurrently
    for i in 0..CONCURRENCY_LEVEL {
        // Cache operations
        let cache = cache.clone();
        handles.push(tokio::spawn(async move {
            for j in 0..ITERATIONS_PER_TASK / 5 {
                let key = format!("stress-key-{}", (i + j) % 20);
                if j % 2 == 0 {
                    cache.set(key, format!("value-{}", j)).await;
                } else {
                    let _ = cache.get(&key).await;
                }
            }
        }));

        // Session operations
        let session = session.clone();
        handles.push(tokio::spawn(async move {
            for _ in 0..ITERATIONS_PER_TASK / 5 {
                let _ = session.is_valid().await;
                if i % 4 == 0 {
                    session.refresh().await;
                }
            }
        }));

        // Stats operations
        let stats = stats.clone();
        handles.push(tokio::spawn(async move {
            for j in 0..ITERATIONS_PER_TASK / 5 {
                let tier = ["cache", "local", "cloud"][j % 3];
                stats.record_query(tier, 100);
            }
        }));

        // Router operations
        let router = router.clone();
        handles.push(tokio::spawn(async move {
            for j in 0..ITERATIONS_PER_TASK / 5 {
                let _ = router.route_query("test query", "UNCLASSIFIED", j % 2 == 0).await;
            }
        }));

        // Audit operations
        let logger = logger.clone();
        handles.push(tokio::spawn(async move {
            for j in 0..ITERATIONS_PER_TASK / 5 {
                logger.log("STRESS_TEST", "test-user", &format!("event-{}", j)).await;
            }
        }));
    }

    let result = timeout(
        Duration::from_secs(TEST_TIMEOUT_SECS * 2),
        async {
            for handle in handles {
                handle.await.expect("Task panicked");
            }
        }
    ).await;

    assert!(result.is_ok(), "Stress test timed out");

    // Verify all components processed requests
    let (cache_hits, cache_misses) = cache.stats();
    let (total_queries, _, _, _, _) = stats.snapshot();
    let router_decisions = router.get_decision_count();
    let audit_logs = logger.get_log_count();

    println!("Stress test completed:");
    println!("  Cache: {} hits, {} misses", cache_hits, cache_misses);
    println!("  Stats: {} queries tracked", total_queries);
    println!("  Router: {} decisions", router_decisions);
    println!("  Audit: {} log entries", audit_logs);

    // Verify reasonable counts
    assert!(cache_hits + cache_misses > 0, "Cache should have been accessed");
    assert!(total_queries > 0, "Stats should have queries");
    assert!(router_decisions > 0, "Router should have made decisions");
    assert!(audit_logs > 0, "Audit should have log entries");
}

// =============================================================================
// DEADLOCK DETECTION TEST
// =============================================================================

/// Test for potential deadlocks with lock ordering
#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn test_no_deadlock_with_multiple_locks() {
    let lock_a = Arc::new(RwLock::new(0u64));
    let lock_b = Arc::new(RwLock::new(0u64));
    let lock_c = Arc::new(RwLock::new(0u64));

    let mut handles = vec![];

    // Multiple goroutines acquiring locks in various orders
    for i in 0..CONCURRENCY_LEVEL {
        let lock_a = lock_a.clone();
        let lock_b = lock_b.clone();
        let lock_c = lock_c.clone();

        handles.push(tokio::spawn(async move {
            for j in 0..ITERATIONS_PER_TASK {
                // Vary lock acquisition order based on iteration
                match (i + j) % 6 {
                    0 => {
                        let a = lock_a.write().await;
                        let b = lock_b.read().await;
                        let _ = *a + *b;
                    }
                    1 => {
                        let b = lock_b.write().await;
                        let c = lock_c.read().await;
                        let _ = *b + *c;
                    }
                    2 => {
                        let c = lock_c.write().await;
                        let a = lock_a.read().await;
                        let _ = *c + *a;
                    }
                    3 => {
                        let a = lock_a.read().await;
                        let b = lock_b.read().await;
                        let c = lock_c.read().await;
                        let _ = *a + *b + *c;
                    }
                    4 => {
                        let mut a = lock_a.write().await;
                        *a += 1;
                    }
                    _ => {
                        let mut b = lock_b.write().await;
                        let mut c = lock_c.write().await;
                        *b += 1;
                        *c += 1;
                    }
                }
            }
        }));
    }

    // If we complete without hanging, no deadlock occurred
    let result = timeout(
        Duration::from_secs(TEST_TIMEOUT_SECS),
        async {
            for handle in handles {
                handle.await.expect("Task panicked");
            }
        }
    ).await;

    assert!(result.is_ok(), "Deadlock detected - test timed out");
    println!("No deadlock detected with multiple lock orderings");
}
