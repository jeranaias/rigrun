# Race Detection Guide

This document describes how to run race detection tests for the rigrun codebase to ensure thread safety and prevent data races.

## Overview

Race detection is critical for concurrent applications. Both Go and Rust components of rigrun include comprehensive race detection tests that verify thread safety under high concurrency.

## Go Race Detection

Go's built-in race detector is one of the best tools for finding data races. It instruments memory accesses at compile time and detects unsynchronized access.

### Running Go Race Tests Locally

```bash
# Navigate to the Go TUI directory
cd go-tui

# Run all tests with race detector
go test -race -v ./internal/...

# Run specific concurrency tests
go test -race -v -run TestConcurrency ./internal/...

# Run with coverage
go test -race -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out -o coverage.html

# Run with timeout (recommended for CI)
go test -race -v -timeout 5m ./internal/...

# Run benchmarks with race detection (slower but thorough)
go test -race -bench=BenchmarkConcurrent ./internal/...
```

### Go Race Detector Notes

- The race detector adds ~5-10x overhead to execution time
- Memory usage increases ~5-10x as well
- Race detection is only active during test execution
- The `-race` flag must be used at both compile and run time

### What the Go Tests Cover

1. **Config Concurrency** (`TestConcurrency_ConfigGlobalAccess`)
   - Concurrent reads/writes to global config singleton
   - Config reload during active readers
   - Get/Set operations on config values

2. **Router Concurrency** (`TestConcurrency_RouteQuery`)
   - Concurrent query routing decisions
   - Classification-based routing under load
   - Detailed routing with multiple options

3. **Security Concurrency**
   - Audit logger concurrent access
   - Classification enforcer thread safety
   - Lockout manager concurrent operations
   - Auth manager session validation

4. **Stress Tests** (`TestConcurrency_AllComponentsUnderLoad`)
   - All components accessed simultaneously
   - Rapid config changes with concurrent readers

## Rust Thread Safety Testing

Rust's ownership system prevents most data races at compile time, but concurrent access to shared state through `Arc<Mutex<T>>` or `Arc<RwLock<T>>` can still have issues.

### Running Rust Thread Safety Tests Locally

```bash
# Run race detection tests with multiple threads
cargo test --test race_detection_test -- --test-threads=4

# Run with verbose output
cargo test --test race_detection_test -- --test-threads=4 --nocapture

# Run all tests with multi-threading
SKIP_INTEGRATION_TESTS=1 cargo test --all-features -- --test-threads=4
```

### Using ThreadSanitizer (Linux Only)

ThreadSanitizer (TSAN) can detect race conditions that Rust's type system doesn't catch:

```bash
# Install nightly Rust with rust-src component
rustup toolchain install nightly
rustup component add rust-src --toolchain nightly

# Run with ThreadSanitizer
RUSTFLAGS="-Z sanitizer=thread" cargo +nightly test \
    --target x86_64-unknown-linux-gnu \
    --test race_detection_test \
    -- --test-threads=1
```

### Using cargo-careful

`cargo-careful` runs tests with extra checks enabled:

```bash
# Install cargo-careful
cargo install cargo-careful

# Run tests with careful checks
cargo careful test --test race_detection_test
```

### What the Rust Tests Cover

1. **Cache Concurrency** (`test_cache_concurrent_read_write`)
   - Concurrent cache reads and writes
   - Heavy read contention on hot keys

2. **Session Management** (`test_session_concurrent_access`)
   - Concurrent session validation
   - Session refresh/invalidation under load

3. **Stats Tracking** (`test_stats_concurrent_updates`)
   - Atomic counter updates
   - Concurrent tier recording

4. **Routing Decisions** (`test_router_concurrent_decisions`)
   - Concurrent routing logic execution
   - Classification enforcement verification

5. **Audit Logging** (`test_audit_concurrent_logging`)
   - Concurrent log entry creation
   - Write lock contention handling

6. **Server Requests** (`test_server_concurrent_requests`)
   - Simulated request handling
   - Max concurrent tracking

7. **Deadlock Detection** (`test_no_deadlock_with_multiple_locks`)
   - Multiple lock acquisition patterns
   - Lock ordering variations

## CI Integration

Race detection is integrated into the CI pipeline via GitHub Actions:

### Standard Race Detection Job

Runs on every push and PR:
- Go race detection tests
- Rust thread safety tests with multiple threads

### ThreadSanitizer Job (Nightly)

Runs on main branch only:
- Uses Rust nightly with TSAN
- More thorough but slower
- Results are advisory (non-blocking)

## Best Practices for Writing Race-Safe Code

### Go

```go
// Use sync.RWMutex for read-heavy shared state
var mu sync.RWMutex
var data map[string]string

func Read(key string) string {
    mu.RLock()
    defer mu.RUnlock()
    return data[key]
}

func Write(key, value string) {
    mu.Lock()
    defer mu.Unlock()
    data[key] = value
}

// Use sync/atomic for simple counters
var counter int64
atomic.AddInt64(&counter, 1)

// Use sync.Once for one-time initialization
var once sync.Once
var instance *Config

func GetInstance() *Config {
    once.Do(func() {
        instance = loadConfig()
    })
    return instance
}
```

### Rust

```rust
// Use RwLock for read-heavy shared state
let data: Arc<RwLock<HashMap<String, String>>> = Arc::new(RwLock::new(HashMap::new()));

// Reading
{
    let guard = data.read().await;
    let value = guard.get("key");
}

// Writing
{
    let mut guard = data.write().await;
    guard.insert("key".to_string(), "value".to_string());
}

// Use AtomicU64 for counters
let counter = AtomicU64::new(0);
counter.fetch_add(1, Ordering::Relaxed);

// Use once_cell for lazy initialization
use once_cell::sync::Lazy;
static CONFIG: Lazy<Config> = Lazy::new(|| load_config());
```

## Troubleshooting

### "race detected" in Go tests

1. Check the stack trace to identify the racing goroutines
2. Look for shared state access without proper synchronization
3. Consider using channels instead of shared memory
4. Add appropriate mutex/RWMutex protection

### Deadlock in Rust tests

1. Ensure consistent lock ordering across all code paths
2. Avoid holding multiple locks simultaneously when possible
3. Use `try_lock()` with timeout for debugging
4. Consider using lock-free data structures

### Tests timing out

1. Reduce CONCURRENCY_LEVEL or ITERATIONS_PER_TASK
2. Check for deadlocks using RUST_BACKTRACE=1
3. Increase TEST_TIMEOUT_SECS if tests are legitimately slow
4. Profile to find performance bottlenecks

## Adding New Race Tests

When adding new concurrent functionality, follow this pattern:

1. Create a test that exercises the concurrent behavior
2. Use many goroutines/tasks (100+) with multiple iterations
3. Mix read and write operations
4. Include timeout protection
5. Verify expected outcomes after concurrent execution

Example template for Go:

```go
func TestConcurrency_NewFeature(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            for j := 0; j < 50; j++ {
                select {
                case <-ctx.Done():
                    return
                default:
                }
                // Test concurrent operations here
            }
        }(i)
    }
    wg.Wait()
}
```

Example template for Rust:

```rust
#[tokio::test(flavor = "multi_thread", worker_threads = 4)]
async fn test_new_feature_concurrent() {
    let shared = Arc::new(SharedState::new());
    let mut handles = vec![];

    for i in 0..100 {
        let shared = shared.clone();
        handles.push(tokio::spawn(async move {
            for j in 0..50 {
                // Test concurrent operations here
            }
        }));
    }

    for handle in handles {
        handle.await.expect("Task panicked");
    }
}
```
