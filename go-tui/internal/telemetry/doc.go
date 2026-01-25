// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package telemetry provides cost tracking and analytics for rigrun.
//
// This package tracks token usage and costs across different LLM tiers,
// providing insights for budget management and cost optimization.
//
// # Key Types
//
//   - Tracker: Main cost tracking interface
//   - CostRecord: Single cost event with tokens, tier, and timestamp
//   - Summary: Aggregated cost statistics for a time period
//   - Storage: Persistent storage for cost records
//
// # Usage
//
// Track a query cost:
//
//	tracker := telemetry.NewTracker(storage)
//	tracker.Record(telemetry.CostRecord{
//	    Tier:         router.TierLocal,
//	    InputTokens:  100,
//	    OutputTokens: 200,
//	    Cost:         0.0,
//	})
//
// Get cost summary:
//
//	summary := tracker.Summary(time.Now().AddDate(0, 0, -7), time.Now())
//	fmt.Printf("Weekly cost: $%.2f\n", summary.TotalCost)
//
// # Privacy
//
// Cost tracking is local-only and does not transmit any data.
// Query content is never stored - only token counts and costs.
package telemetry
