# Contributing to rigrun

Thank you for your interest in contributing to rigrun! This guide will help you get started with development, testing, and submitting contributions.

---

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Development Workflow](#development-workflow)
- [Testing](#testing)
- [Code Style](#code-style)
- [Submitting Changes](#submitting-changes)
- [Areas for Contribution](#areas-for-contribution)

---

## Getting Started

### Prerequisites

Before contributing, ensure you have:

1. **Rust Toolchain** (stable)
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
rustup update stable
```

2. **Ollama** (for testing local inference)
```bash
# See https://ollama.com/download
```

3. **Git**
```bash
git --version
```

4. **Code Editor** with Rust support
- VS Code + rust-analyzer
- IntelliJ IDEA/CLion + Rust plugin
- Any editor with LSP support

### Quick Setup

```bash
# Fork the repository on GitHub

# Clone your fork
git clone https://github.com/YOUR_USERNAME/rigrun.git
cd rigrun

# Add upstream remote
git remote add upstream https://github.com/rigrun/rigrun.git

# Build and test
cargo build
cargo test
```

---

## Development Setup

### 1. Build rigrun

```bash
# Debug build (fast compilation, slower runtime)
cargo build

# Release build (optimized)
cargo build --release
```

Binaries are in:
- Debug: `target/debug/rigrun`
- Release: `target/release/rigrun`

### 2. Run Tests

```bash
# All tests
cargo test

# Specific test
cargo test test_routing

# With output
cargo test -- --nocapture

# With logging
RUST_LOG=debug cargo test
```

### 3. Run Linters

```bash
# Format code
cargo fmt

# Check formatting (CI check)
cargo fmt -- --check

# Run clippy (linter)
cargo clippy --all-targets --all-features -- -D warnings
```

### 4. Run rigrun Locally

```bash
# Run from source
cargo run

# With arguments
cargo run -- --paranoid

# With debug logging
RUST_LOG=debug cargo run
```

### 5. Clean Build Artifacts

```bash
cargo clean
```

---

## Project Structure

```
rigrun/
├── src/
│   ├── main.rs              # CLI entry point
│   ├── server/              # HTTP server and routing
│   │   └── mod.rs
│   ├── cache/               # Semantic and exact caching
│   │   ├── mod.rs
│   │   ├── semantic.rs
│   │   └── vector_index.rs
│   ├── local/               # Ollama integration
│   │   └── mod.rs
│   ├── cloud/               # OpenRouter integration
│   │   └── mod.rs
│   ├── router/              # Request routing logic
│   │   └── mod.rs
│   ├── config.rs            # Configuration management
│   ├── stats.rs             # Statistics tracking
│   └── audit.rs             # Audit logging
├── tests/
│   └── integration_tests.rs # Integration tests
├── docs/                    # Documentation
├── Cargo.toml               # Dependencies and metadata
├── Cargo.lock               # Dependency lock file
└── README.md                # Main readme
```

### Key Modules

**`src/server/mod.rs`**
- HTTP server (axum)
- Endpoint handlers
- Rate limiting
- Request validation

**`src/cache/semantic.rs`**
- Semantic cache implementation
- Embedding generation (via Ollama)
- Similarity search
- TTL management

**`src/router/mod.rs`**
- Routing decision logic
- Tier selection (cache/local/cloud)
- Fallback handling

**`src/local/mod.rs`**
- Ollama API client
- Model management
- Local inference

**`src/cloud/mod.rs`**
- OpenRouter API client
- Cloud fallback logic
- Model selection

---

## Development Workflow

### 1. Create a Feature Branch

```bash
# Update your fork
git fetch upstream
git checkout main
git merge upstream/main

# Create feature branch
git checkout -b feature/my-feature
```

Branch naming conventions:
- `feature/` - New features
- `fix/` - Bug fixes
- `refactor/` - Code refactoring
- `docs/` - Documentation updates
- `test/` - Test additions/fixes

### 2. Make Changes

**Edit code**:
```bash
# Your favorite editor
code .
```

**Run tests frequently**:
```bash
cargo test
```

**Format code**:
```bash
cargo fmt
```

**Check for issues**:
```bash
cargo clippy
```

### 3. Write Tests

Add tests for new features or bug fixes:

**Unit tests** (in same file):
```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_my_feature() {
        // Test implementation
    }
}
```

**Integration tests** (in `tests/`):
```rust
#[tokio::test]
async fn test_end_to_end() {
    // Test complete workflow
}
```

### 4. Commit Changes

```bash
# Stage changes
git add .

# Commit with clear message
git commit -m "feat: add semantic similarity threshold configuration"
```

**Commit message format**:
```
type: brief description

Longer explanation if needed.

Fixes #123
```

Types:
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation
- `refactor:` - Code refactoring
- `test:` - Test additions
- `chore:` - Maintenance tasks

### 5. Push and Create PR

```bash
# Push to your fork
git push origin feature/my-feature
```

**Create Pull Request on GitHub**:
1. Go to your fork on GitHub
2. Click "New Pull Request"
3. Select your feature branch
4. Fill out PR template
5. Submit

---

## Testing

### Running Tests

**All tests**:
```bash
cargo test
```

**Specific module**:
```bash
cargo test cache
```

**Integration tests only**:
```bash
cargo test --test integration_tests
```

**With coverage** (requires `cargo-tarpaulin`):
```bash
cargo install cargo-tarpaulin
cargo tarpaulin --out Html
```

### Writing Tests

**Example unit test**:
```rust
// In src/cache/semantic.rs
#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_semantic_cache_hit() {
        let cache = SemanticCache::new();

        // Store a response
        cache.store("What is Rust?", "Rust is a programming language").await;

        // Query with similar text
        let result = cache.get("Explain Rust").await;

        assert!(result.is_some());
    }
}
```

**Example integration test**:
```rust
// In tests/integration_tests.rs
#[tokio::test]
async fn test_full_request() {
    // Start server
    let server = start_test_server().await;

    // Make request
    let response = reqwest::Client::new()
        .post(format!("{}/v1/chat/completions", server.url))
        .json(&json!({
            "model": "auto",
            "messages": [{"role": "user", "content": "Hello"}]
        }))
        .send()
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
}
```

### Test Best Practices

1. **Test behavior, not implementation**
2. **Use descriptive test names**
3. **One assertion per test (when possible)**
4. **Clean up resources in tests**
5. **Mock external dependencies**

---

## Code Style

### Rust Style Guidelines

**Follow Rust conventions**:
- Use `cargo fmt` for automatic formatting
- Follow Rust API guidelines: https://rust-lang.github.io/api-guidelines/

**Naming**:
```rust
// Functions and variables: snake_case
fn calculate_similarity(query: &str) -> f32 { }

// Types: PascalCase
struct SemanticCache { }

// Constants: SCREAMING_SNAKE_CASE
const MAX_CACHE_SIZE: usize = 10000;
```

**Error Handling**:
```rust
// Use Result instead of panic
fn process_request() -> Result<Response, Error> {
    // ...
}

// Provide context
.context("Failed to connect to Ollama")?
```

**Documentation**:
```rust
/// Calculates semantic similarity between two queries.
///
/// Uses cosine similarity on embedding vectors.
///
/// # Arguments
///
/// * `query1` - First query text
/// * `query2` - Second query text
///
/// # Returns
///
/// Similarity score between 0.0 and 1.0
///
/// # Examples
///
/// ```
/// let similarity = calculate_similarity("What is Rust?", "Explain Rust");
/// assert!(similarity > 0.8);
/// ```
pub fn calculate_similarity(query1: &str, query2: &str) -> f32 {
    // Implementation
}
```

### Clippy Lints

We enforce clippy warnings:
```bash
cargo clippy --all-targets --all-features -- -D warnings
```

Common issues to avoid:
- Unnecessary clones
- Unused imports
- Redundant pattern matching
- Missing documentation

---

## Submitting Changes

### Pull Request Checklist

Before submitting PR:

- [ ] Code compiles without warnings
- [ ] All tests pass: `cargo test`
- [ ] Code is formatted: `cargo fmt`
- [ ] Clippy passes: `cargo clippy`
- [ ] Documentation updated (if needed)
- [ ] Tests added for new features
- [ ] CHANGELOG.md updated (for significant changes)
- [ ] Commit messages are clear

### PR Description Template

```markdown
## Summary
Brief description of changes

## Motivation
Why this change is needed

## Changes
- List of specific changes
- Another change

## Testing
How this was tested

## Related Issues
Fixes #123
```

### Review Process

1. **Automated checks**:
   - CI runs tests and lints
   - Must pass before review

2. **Code review**:
   - Maintainer reviews code
   - May request changes

3. **Revisions**:
   - Address feedback
   - Push new commits

4. **Merge**:
   - Maintainer merges PR
   - Your contribution is live!

### Getting Feedback

**For large changes**:
1. Open an issue first to discuss approach
2. Get feedback before writing code
3. Submit PR after agreement

**For small changes**:
- PRs welcome directly
- Bug fixes especially appreciated

---

## Areas for Contribution

### Good First Issues

Look for issues tagged `good-first-issue`:
- Documentation improvements
- Test additions
- Small bug fixes
- Error message improvements

### High Priority

**From Remediation Plan**:

**Phase 1: Security**
- API key redaction in logs
- Bearer token authentication
- Rate limiting improvements

**Phase 2: Performance**
- HNSW vector search integration
- Lock contention fixes
- Batch stats persistence

**Phase 3: Code Quality**
- Error handling improvements
- Type deduplication
- Integration test fixes

**Phase 4: UX**
- `rigrun doctor` command
- Better error messages
- CLI simplification

### Feature Ideas

**Caching**:
- Configurable TTL
- Cache warming
- Cache statistics improvements

**Routing**:
- Custom routing rules
- Multi-model support
- A/B testing

**Monitoring**:
- Prometheus metrics
- Grafana dashboard
- Detailed request tracing

**Integration**:
- VS Code extension
- Cursor integration
- CI/CD templates

---

## Development Tips

### Debugging

**Enable debug logging**:
```bash
RUST_LOG=debug cargo run
```

**Specific module**:
```bash
RUST_LOG=rigrun::cache=trace cargo run
```

**Use debugger**:
```bash
# VS Code: Add launch.json configuration
# Or use rust-lldb / rust-gdb
```

### Performance Profiling

**Install tools**:
```bash
cargo install flamegraph
cargo install cargo-instruments  # macOS only
```

**Profile**:
```bash
# Flamegraph
cargo flamegraph

# Instruments (macOS)
cargo instruments --template time
```

### Benchmarking

**Add benchmark** (in `benches/`):
```rust
use criterion::{black_box, criterion_group, criterion_main, Criterion};

fn benchmark_similarity(c: &mut Criterion) {
    c.bench_function("similarity", |b| {
        b.iter(|| calculate_similarity(
            black_box("query1"),
            black_box("query2")
        ))
    });
}

criterion_group!(benches, benchmark_similarity);
criterion_main!(benches);
```

**Run**:
```bash
cargo bench
```

---

## Getting Help

### Communication Channels

- **GitHub Issues**: https://github.com/rigrun/rigrun/issues
- **GitHub Discussions**: https://github.com/rigrun/rigrun/discussions
- **Pull Requests**: Ask questions in PR comments

### Documentation

- [Architecture Overview](../ARCHITECTURE.md) (if exists)
- [API Reference](api-reference.md)
- [Configuration Guide](configuration.md)

### Mentorship

New contributors welcome! Ask for guidance on:
- Where to start
- How to approach a problem
- Code review clarification

---

## Code of Conduct

### Our Standards

**Positive behavior**:
- Be respectful and inclusive
- Give and accept constructive feedback
- Focus on what's best for the project
- Show empathy towards others

**Unacceptable behavior**:
- Harassment or discrimination
- Trolling or insulting comments
- Personal or political attacks
- Publishing others' private information

### Enforcement

Report issues to maintainers. All reports will be reviewed and investigated.

---

## License

By contributing, you agree that your contributions will be licensed under the project's MIT License.

---

## Recognition

Contributors are recognized in:
- GitHub contributors page
- CHANGELOG.md for significant contributions
- README.md acknowledgments section

Thank you for contributing to rigrun!

---

## Additional Resources

- **Rust Book**: https://doc.rust-lang.org/book/
- **Rust API Guidelines**: https://rust-lang.github.io/api-guidelines/
- **Cargo Book**: https://doc.rust-lang.org/cargo/
- **axum Documentation**: https://docs.rs/axum/
- **tokio Documentation**: https://docs.rs/tokio/
