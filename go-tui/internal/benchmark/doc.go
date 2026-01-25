// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package benchmark provides model benchmarking capabilities for rigrun.
//
// This package runs standardized tests against local LLM models to measure
// performance characteristics like tokens per second, latency, and accuracy.
//
// # Key Types
//
//   - Runner: Benchmark runner with configurable tests
//   - Result: Single benchmark result with timing and metrics
//   - Summary: Aggregated results across multiple runs
//   - Test: Individual test case definition
//
// # Usage
//
// Run benchmarks:
//
//	runner := benchmark.NewRunner(client)
//	results, err := runner.Run(ctx, "qwen2.5:7b")
//
// Compare models:
//
//	summary := benchmark.Compare(results1, results2)
//	fmt.Printf("Speed: %.1f vs %.1f tok/s\n",
//	    summary.Model1.TokensPerSecond,
//	    summary.Model2.TokensPerSecond)
//
// # Test Categories
//
//   - Speed: Raw token generation speed
//   - Quality: Response quality for various tasks
//   - Tool Use: Function/tool calling accuracy
package benchmark
