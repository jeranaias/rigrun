// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// +build ignore

// This file demonstrates how to integrate cost tracking into the main application.
// It is not compiled but serves as documentation.

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/commands"
	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/telemetry"
	"github.com/jeranaias/rigrun-tui/internal/ui/components"
)

// Example 1: Initialize cost tracker on application startup
func initializeCostTracking() (*telemetry.CostTracker, error) {
	// Create cost tracker with default storage location (~/.rigrun/costs/)
	tracker, err := telemetry.NewCostTracker("")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cost tracker: %w", err)
	}

	// Start auto-save goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if err := tracker.SaveCurrentSession(); err != nil {
				log.Printf("Failed to auto-save cost session: %v", err)
			}
		}
	}()

	return tracker, nil
}

// Example 2: Add cost tracker to command context
func createCommandContext(tracker *telemetry.CostTracker) *commands.Context {
	// Initialize other dependencies (config, ollama, storage, etc.)
	// ...

	ctx := commands.NewContext(cfg, ollama, store, session, cache)
	ctx.CostTracker = tracker

	return ctx
}

// Example 3: Record query costs after LLM inference
func executeQueryWithCostTracking(tracker *telemetry.CostTracker, query string) {
	// Execute query
	startTime := time.Now()

	// Simulate query execution
	result := router.QueryResult{
		TierUsed:     router.TierCloud,
		InputTokens:  500,
		OutputTokens: 1500,
		LatencyMs:    2000,
		Response:     "Generated response...",
		CostCents:    0.234,
	}

	duration := time.Since(startTime)

	// Record cost
	tracker.RecordQuery(
		result.TierUsed.String(),
		int(result.InputTokens),
		int(result.OutputTokens),
		duration,
		query, // First 100 chars will be stored
	)
}

// Example 4: Handle /cost command
func handleCostCommand(tracker *telemetry.CostTracker, view string) {
	dashboard := components.NewCostDashboard(tracker)
	dashboard.SetSize(80, 40) // Terminal width/height

	switch view {
	case "history":
		dashboard.SetView(components.ViewHistory)
	case "breakdown":
		dashboard.SetView(components.ViewBreakdown)
	default:
		dashboard.SetView(components.ViewSummary)
	}

	output := dashboard.View()
	fmt.Println(output)
}

// Example 5: Display real-time cost updates in status bar
func getStatusBarCostInfo(tracker *telemetry.CostTracker) string {
	session := tracker.GetCurrentSession()
	if session == nil {
		return "Cost: $0.0000"
	}

	return fmt.Sprintf("Cost: $%.4f | Saved: $%.4f", session.TotalCost, session.Savings)
}

// Example 6: Session management - end session on application exit
func shutdownCostTracking(tracker *telemetry.CostTracker) {
	// End current session and save
	if err := tracker.EndSession(); err != nil {
		log.Printf("Failed to end cost session: %v", err)
	}

	log.Println("Cost tracking shut down successfully")
}

// Example 7: Cleanup old cost data
func cleanupOldCostData(tracker *telemetry.CostTracker, daysToKeep int) error {
	cutoffDate := time.Now().AddDate(0, 0, -daysToKeep)

	// Access storage directly for cleanup
	// Note: This is internal implementation detail
	// In production, you'd expose this through CostTracker API
	return tracker.storage.DeleteBefore(cutoffDate)
}

// Example 8: Export cost report
func exportCostReport(tracker *telemetry.CostTracker, days int) {
	trends := tracker.GetTrends(days)

	fmt.Printf("Cost Report - Last %d Days\n", days)
	fmt.Println("=" + string(make([]byte, 50)))
	fmt.Printf("Total Cost:   $%.4f\n", trends.TotalCost)
	fmt.Printf("Total Saved:  $%.4f\n", trends.TotalSaved)
	fmt.Println()

	fmt.Println("Daily Breakdown:")
	for _, daily := range trends.DailyBreakdown {
		fmt.Printf("  %s: $%.4f (%d queries)\n",
			daily.Date.Format("2006-01-02"),
			daily.Cost,
			daily.QueryCount,
		)
	}

	fmt.Println()
	fmt.Println("Tier Breakdown:")
	for tier, cost := range trends.TierBreakdown {
		percent := 0.0
		if trends.TotalCost > 0 {
			percent = (cost / trends.TotalCost) * 100
		}
		fmt.Printf("  %-6s: $%.4f (%.1f%%)\n", tier, cost, percent)
	}
}

// Example 9: Cost alerts
func checkCostAlerts(tracker *telemetry.CostTracker, dailyLimit float64) bool {
	session := tracker.GetCurrentSession()
	if session == nil {
		return false
	}

	if session.TotalCost > dailyLimit {
		fmt.Printf("[!!] Cost Alert: Session cost ($%.4f) exceeds daily limit ($%.4f)\n",
			session.TotalCost, dailyLimit)
		return true
	}

	return false
}

// Example 10: Complete integration example
func main() {
	// 1. Initialize cost tracking
	tracker, err := initializeCostTracking()
	if err != nil {
		log.Fatal(err)
	}
	defer shutdownCostTracking(tracker)

	// 2. Create command context
	ctx := createCommandContext(tracker)

	// 3. Execute queries with cost tracking
	queries := []string{
		"What is the capital of France?",
		"Explain quantum computing",
		"Write a sorting algorithm in Go",
	}

	for _, query := range queries {
		executeQueryWithCostTracking(tracker, query)
	}

	// 4. Display cost dashboard
	fmt.Println("\n=== Cost Summary ===")
	handleCostCommand(tracker, "summary")

	fmt.Println("\n=== Cost History ===")
	handleCostCommand(tracker, "history")

	fmt.Println("\n=== Tier Breakdown ===")
	handleCostCommand(tracker, "breakdown")

	// 5. Check cost alerts
	checkCostAlerts(tracker, 1.0) // $1 daily limit

	// 6. Export report
	fmt.Println("\n=== Cost Report ===")
	exportCostReport(tracker, 7)

	// 7. Cleanup old data (keep last 90 days)
	if err := cleanupOldCostData(tracker, 90); err != nil {
		log.Printf("Failed to cleanup old data: %v", err)
	}

	// 8. Show status bar info
	fmt.Printf("\nStatus Bar: %s\n", getStatusBarCostInfo(tracker))
}
