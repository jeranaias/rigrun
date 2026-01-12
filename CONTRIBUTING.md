# Contributing to rigrun

Welcome!

Thanks for your interest in contributing to **rigrun** - a local-first LLM router written in Rust.
We're excited to collaborate with you on making it faster, more reliable, and easier to use.

This document explains how to get set up, how we work, and what we expect from contributions.

---

## Code of Conduct

We are committed to fostering a welcoming, inclusive, and harassment-free community.

By participating in this project (issues, pull requests, discussions, etc.), you agree to abide by our Code of Conduct.

If you experience or witness unacceptable behavior, please report it to the maintainers.

---

## Development Setup

### Prerequisites

You'll need:

- [Rust](https://www.rust-lang.org/tools/install) (stable channel)
  - Preferably using `rustup`
  - If the repo contains a `rust-toolchain.toml`, it defines the exact supported toolchain
- [Git](https://git-scm.com/)
- A C compiler / build tools (for your platform), if needed by dependencies

Optional but recommended:

- An editor/IDE with good Rust support:
  - [VS Code + rust-analyzer](https://marketplace.visualstudio.com/items?itemName=rust-lang.rust-analyzer)
  - IntelliJ IDEA / CLion with Rust plugin
- [`cargo-watch`](https://crates.io/crates/cargo-watch) for auto-running tests/builds during development

### Clone the repository

```bash
git clone https://github.com/rigrun/rigrun.git
cd rigrun
```

### Install Rust toolchain

If you don't already have Rust:

```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source "$HOME/.cargo/env"
```

Update Rust to the latest stable:

```bash
rustup update stable
rustup default stable
```

If the project specifies a toolchain (via `rust-toolchain` or `rust-toolchain.toml`), `rustup` will automatically use it when you run `cargo` inside the repo.

---

## Building and Testing

### Build the project

From the repository root:

```bash
cargo build
```

For a release build:

```bash
cargo build --release
```

### Run tests

```bash
cargo test
```

To run tests for a specific package (if this is a workspace):

```bash
cargo test -p rigrun
```

To run a specific test (e.g. named `router_works`):

```bash
cargo test router_works
```

Run tests with logs enabled (if using `tracing` or `env_logger`):

```bash
RUST_LOG=debug cargo test -- --nocapture
```

### Lints and formatting

We use `rustfmt` for formatting and `clippy` for lints.

Format the code:

```bash
cargo fmt
```

Run clippy:

```bash
cargo clippy --all-targets --all-features -- -D warnings
```

Please run both `cargo fmt` and `cargo clippy` before opening a PR.

---

## Code Style Guidelines

### General Rust style

- Follow idiomatic Rust practices and the output of `rustfmt`.
- Prefer clarity over cleverness.
- Minimize unsafe code; if absolutely necessary, clearly document the safety invariants.

### Documentation

- Add doc comments (`///`) for:
  - Public types (structs, enums, traits)
  - Public functions and methods
  - Modules with non-trivial behavior
- Explain *why* something exists (not just *what* it does) when it's not obvious.
- For configuration and routing rules, include short usage examples where useful.

### Error handling

- Use expressive error types (e.g. `thiserror`) where appropriate.
- Use `Result<T, E>` rather than panicking, except in truly unrecoverable situations.
- Prefer `?` for propagating errors.
- If you introduce new public-facing error types, document them.

### Logging and observability

- Use structured logging where possible (e.g. via `tracing`).
- Log at appropriate levels:
  - `error` for real failures
  - `warn` for unexpected but non-fatal situations
  - `info` for high-level operational events (startup, shutdown, configuration changes)
  - `debug`/`trace` for detailed internal behavior
- Avoid logging sensitive information (API keys, tokens, secrets, private prompts/responses, etc.).

### Testing

- Add tests for:
  - New features
  - Bug fixes (ideally regression tests demonstrating the original issue)
- Prefer:
  - Unit tests for small, isolated logic
  - Integration tests for end-to-end router behavior (e.g. routing decisions between local and remote LLMs)
- Keep tests deterministic and fast.

---

## Submitting Changes (Pull Request Process)

We use the standard GitHub PR workflow.

### 1. Fork and branch

- Fork the repository on GitHub.
- Clone your fork locally:
  ```bash
  git clone https://github.com/<your-username>/rigrun.git
  cd rigrun
  ```
- Create a feature branch:
  ```bash
  git checkout -b feature/my-change
  ```

Use a descriptive branch name, such as:

- `fix/local-cache-race`
- `feat/add-openai-backend`
- `chore/update-deps-2026-01`

### 2. Make your changes

- Keep changes focused and self-contained.
- Update or add tests where appropriate.
- If you change behavior or defaults, update documentation (README, docs, examples, etc).

Before committing:

```bash
cargo fmt
cargo clippy --all-targets --all-features -- -D warnings
cargo test
```

### 3. Write good commits

- Make commits small and logically coherent.
- Use clear commit messages, e.g.:

  - `fix: avoid routing remote requests when local cache is stale`
  - `feat: add configurable routing weights for local vs remote LLMs`
  - `docs: document environment variables for provider configuration`

### 4. Open a Pull Request

From your fork, open a PR against the main `rigrun` repo's default branch.

In the PR description:

- Explain **what** the change does.
- Explain **why** it's needed (link related issues if applicable).
- Note any breaking changes or migration steps.
- Mention any follow-up work that is intentionally left out.

A helpful template:

- **Motivation / context**
- **What's changed**
- **How to test**
- **Potential risks / tradeoffs**
- **Checklist**

### 5. PR checklist

Before marking your PR as ready for review, please ensure:

- [ ] `cargo fmt` passes
- [ ] `cargo clippy --all-targets --all-features` passes (no new warnings)
- [ ] `cargo test` passes
- [ ] Docs updated (if behavior, config, or public APIs changed)
- [ ] Changelog / release notes updated if the project uses one (follow existing pattern)

### 6. Code review process

- A maintainer will review your PR and may suggest changes.
- Don't hesitate to ask questions or request clarification on review comments.
- It's normal to go through several review iterations, especially for larger changes.
- Once approved, a maintainer will merge the PR (we may use squash or rebase merges depending on project conventions).

---

## Reporting Bugs

If you encounter a bug, please help us by filing an issue.

### Before reporting

- Check existing GitHub issues to see if it's already reported.
- Try updating to the latest version of `rigrun` if possible.

### When reporting

Open a **Bug report** issue on GitHub and include:

- **Environment:**
  - OS and version
  - Rust toolchain version (`rustc --version`)
  - rigrun version/commit hash
- **Steps to reproduce:**
  - Exact commands you ran
  - Relevant configuration (redact secrets!)
  - Any input data needed to reproduce
- **Expected behavior:**
  - What you thought would happen
- **Actual behavior:**
  - What actually happened (error messages, logs, panics, etc.)
- **Additional context:**
  - Are you using local-only backends, remote providers, or a mix?
  - Any special runtime environment (containers, Kubernetes, etc.)

If possible, include:

- A minimal reproducible example (config + command)
- Logs (with sensitive data removed)
- Backtraces (`RUST_BACKTRACE=1`)

---

## Requesting Features

We welcome ideas for improving rigrun.

When opening a **Feature request** issue, please include:

- **Motivation / problem:**
  - What problem does this solve?
  - How does it improve local-first routing or your workflow?
- **Proposed solution:**
  - Rough idea of how you think it might work (API sketch, configuration, UX)
- **Alternatives considered:**
  - Other ways you might solve the problem today
- **Scope:**
  - Is this a small addition or a larger subsystem?

If you're planning to implement the feature yourself:

- Mention that in the issue so we can coordinate.
- For larger changes, please discuss design in the issue before investing significant time - we want to ensure it aligns with the project's direction.

---

## Project Structure Overview

This is a high-level overview; the actual layout may evolve over time. Typical components you may find:

- `Cargo.toml` / `Cargo.lock`
  Crate/workspace definition, dependencies, features, and metadata.

- `src/`
  Core library and router implementation. Common sub-areas might include:
  - Routing logic (deciding between local vs remote LLMs)
  - Configuration parsing and validation
  - Abstractions over different LLM backends (local and remote)
  - Request/response types and middleware
  - Utilities (e.g. caching, rate limiting, metrics)

- `src/bin/` or dedicated binary crates
  CLI and/or server entry points (e.g. launching the router as a service).

- `tests/`
  Integration tests that exercise the router as a whole (end-to-end routing, configuration behavior, etc.).

- `examples/`
  Example configurations and usage patterns (e.g. local-only routing, hybrid local/remote, multi-model routing).

- `docs/` (if present)
  More extensive documentation, design notes, or architecture overviews.

- `.github/`
  CI workflows, issue templates, PR templates, etc.

- `benches/` (if present)
  Benchmarks for performance testing (routing latency, throughput, backend selection performance).

If you're unsure where a new feature or file should live, feel free to ask in an issue or PR.

---

## Contact and Support

Ways to get in touch or seek help:

- **GitHub Issues**
  For bugs, feature requests, and general questions.
  Please label issues appropriately if you can.

- **GitHub Discussions** (if enabled on the repo)
  For broader design discussions, Q&A, and community sharing.

- **Security / Vulnerability reports**
  If you discover a security issue, **do not** open a public issue.
  Instead, follow the instructions in `SECURITY.md` (if present) or contact the maintainers privately as indicated in the repository.

If you're unsure where to start or how to contribute, opening an issue titled something like "Help me find a good first issue" is perfectly fine - we're happy to help.

---

Thank you for contributing to rigrun and helping build a robust, local-first LLM routing ecosystem in Rust!
