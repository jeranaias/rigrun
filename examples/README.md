# rigrun Examples

This directory contains example code demonstrating various features of rigrun.

## Contents

### Rust Examples

- **error_demo.rs** - Demonstrates the improved error message formatting across Ollama and OpenRouter integrations. Shows how to use the `ErrorBuilder` for consistent, actionable error messages.

### Go Examples (intel/)

These examples show how to build competitive intelligence research modules using rigrun's agentic tool system.

- **example_module_company_overview.go** - A complete example of implementing a research module (Module 1: Company Overview). Demonstrates:
  - Using WebSearch and WebFetch tools
  - Classification-aware routing
  - Caching with configurable TTL
  - Audit logging for compliance
  - Structured data extraction from unstructured sources

- **example_orchestrator.go** - Shows how to coordinate multiple research modules to produce complete reports. Demonstrates:
  - Multi-phase research workflow (Discovery, Deep Research, Analysis)
  - Classification-based routing decisions
  - Report generation in multiple formats (Markdown, JSON, PDF)
  - Cost tracking and optimization
  - Session management

## Running the Examples

### Rust Example

```bash
# From the rigrun root directory
cargo run --example error_demo
```

### Go Examples

The Go examples are reference implementations showing the architecture patterns. To use them:

```bash
cd go-tui
go build ./examples/...
```

## Related Documentation

- [ARCHITECTURE.md](../ARCHITECTURE.md) - Complete technical architecture documentation
- [Getting Started Guide](../docs/GETTING_STARTED.md) - Quickstart guide
- [API Reference](../docs/api-reference.md) - OpenAI-compatible API documentation
- [Intel System Design](../docs/COMPETITIVE_INTEL_SYSTEM_DESIGN.md) - Competitive intelligence system design

## License

Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
SPDX-License-Identifier: AGPL-3.0-or-later
