// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package telemetry provides cost tracking and analytics for rigrun.
package telemetry

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/router"
)

// =============================================================================
// COST TRACKER
// =============================================================================

// sessionIDCounter ensures unique session IDs even when created rapidly
var sessionIDCounter uint64

// CostTracker tracks token usage and costs across sessions.
type CostTracker struct {
	mu        sync.RWMutex
	sessions  map[string]*SessionCost
	currentID string
	storage   *CostStorage
}

// SessionCost tracks costs for a single session.
type SessionCost struct {
	ID        string    `json:"id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`

	// Token counts by tier
	CacheTokens TokenCount `json:"cache_tokens"`
	LocalTokens TokenCount `json:"local_tokens"`
	CloudTokens TokenCount `json:"cloud_tokens"`

	// Costs
	TotalCost float64 `json:"total_cost"` // In dollars
	Savings   float64 `json:"savings"`    // vs all-cloud pricing

	// Top queries
	TopQueries []QueryCost `json:"top_queries"`
}

// TokenCount tracks input/output tokens.
type TokenCount struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// QueryCost tracks cost of individual queries.
type QueryCost struct {
	Timestamp    time.Time `json:"timestamp"`
	Prompt       string    `json:"prompt"`        // First 100 chars
	Tier         string    `json:"tier"`          // cache, local, cloud
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	Cost         float64   `json:"cost"`     // In dollars
	Duration     time.Duration `json:"duration"` // Query duration
}

// CostTrends provides aggregated cost trends over time.
type CostTrends struct {
	Days        int                `json:"days"`
	TotalCost   float64            `json:"total_cost"`
	TotalSaved  float64            `json:"total_saved"`
	DailyBreakdown []DailyCost     `json:"daily_breakdown"`
	TierBreakdown  map[string]float64 `json:"tier_breakdown"`
}

// DailyCost tracks costs for a single day.
type DailyCost struct {
	Date       time.Time `json:"date"`
	Cost       float64   `json:"cost"`
	Saved      float64   `json:"saved"`
	QueryCount int       `json:"query_count"`
}

// =============================================================================
// CONSTRUCTOR
// =============================================================================

// NewCostTracker creates a cost tracker with persistent storage.
func NewCostTracker(storagePath string) (*CostTracker, error) {
	storage, err := NewCostStorage(storagePath)
	if err != nil {
		return nil, err
	}

	ct := &CostTracker{
		sessions:  make(map[string]*SessionCost),
		currentID: generateSessionID(),
		storage:   storage,
	}

	// Initialize current session
	ct.sessions[ct.currentID] = &SessionCost{
		ID:         ct.currentID,
		StartTime:  time.Now(),
		TopQueries: make([]QueryCost, 0),
	}

	return ct, nil
}

// =============================================================================
// RECORDING
// =============================================================================

// RecordQuery records a query's cost and updates session stats.
func (ct *CostTracker) RecordQuery(tier string, inputTokens, outputTokens int, duration time.Duration, prompt string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	session := ct.sessions[ct.currentID]
	if session == nil {
		return
	}

	// Truncate prompt to first 100 chars
	if len(prompt) > 100 {
		prompt = prompt[:100] + "..."
	}

	// Calculate cost based on tier
	var tierEnum router.Tier
	var cost float64

	switch tier {
	case "cache":
		tierEnum = router.TierCache
		session.CacheTokens.Input += inputTokens
		session.CacheTokens.Output += outputTokens
	case "local":
		tierEnum = router.TierLocal
		session.LocalTokens.Input += inputTokens
		session.LocalTokens.Output += outputTokens
	default:
		// Assume cloud for anything else
		tierEnum = router.TierCloud
		session.CloudTokens.Input += inputTokens
		session.CloudTokens.Output += outputTokens
	}

	cost = tierEnum.CalculateCostCents(uint32(inputTokens), uint32(outputTokens)) / 100.0 // Convert cents to dollars
	session.TotalCost += cost

	// Calculate savings vs all-cloud
	opusCost := router.TierOpus.CalculateCostCents(uint32(inputTokens), uint32(outputTokens)) / 100.0
	session.Savings += (opusCost - cost)

	// Record query cost
	queryCost := QueryCost{
		Timestamp:    time.Now(),
		Prompt:       prompt,
		Tier:         tier,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
		Duration:     duration,
	}

	// Add to top queries and maintain top 10
	session.TopQueries = append(session.TopQueries, queryCost)
	ct.updateTopQueries(session)
}

// updateTopQueries maintains the top 10 most expensive queries.
func (ct *CostTracker) updateTopQueries(session *SessionCost) {
	// Sort by cost descending
	queries := session.TopQueries
	for i := 0; i < len(queries); i++ {
		for j := i + 1; j < len(queries); j++ {
			if queries[j].Cost > queries[i].Cost {
				queries[i], queries[j] = queries[j], queries[i]
			}
		}
	}

	// Keep only top 10
	if len(queries) > 10 {
		session.TopQueries = queries[:10]
	}
}

// =============================================================================
// RETRIEVAL
// =============================================================================

// GetCurrentSession returns the current session's cost data.
func (ct *CostTracker) GetCurrentSession() *SessionCost {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	session := ct.sessions[ct.currentID]
	if session == nil {
		return &SessionCost{
			ID:         ct.currentID,
			StartTime:  time.Now(),
			TopQueries: make([]QueryCost, 0),
		}
	}

	// Return a copy to avoid race conditions
	return ct.copySession(session)
}

// GetHistory returns cost history for the specified date range.
func (ct *CostTracker) GetHistory(from, to time.Time) []*SessionCost {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	sessionIDs, err := ct.storage.List(from, to)
	if err != nil {
		return nil
	}

	sessions := make([]*SessionCost, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		session, err := ct.storage.Load(id)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions
}

// GetTrends returns aggregated cost trends over the specified number of days.
func (ct *CostTracker) GetTrends(days int) *CostTrends {
	to := time.Now()
	from := to.AddDate(0, 0, -days)

	sessions := ct.GetHistory(from, to)
	if len(sessions) == 0 {
		return &CostTrends{
			Days:           days,
			DailyBreakdown: make([]DailyCost, 0),
			TierBreakdown:  make(map[string]float64),
		}
	}

	trends := &CostTrends{
		Days:           days,
		DailyBreakdown: make([]DailyCost, 0),
		TierBreakdown: map[string]float64{
			"cache": 0,
			"local": 0,
			"cloud": 0,
		},
	}

	// Aggregate by day
	dailyMap := make(map[string]*DailyCost)
	for _, session := range sessions {
		dateKey := session.StartTime.Format("2006-01-02")
		daily, ok := dailyMap[dateKey]
		if !ok {
			daily = &DailyCost{
				Date: session.StartTime.Truncate(24 * time.Hour),
			}
			dailyMap[dateKey] = daily
		}

		daily.Cost += session.TotalCost
		daily.Saved += session.Savings
		daily.QueryCount += len(session.TopQueries)

		trends.TotalCost += session.TotalCost
		trends.TotalSaved += session.Savings

		// Tier breakdown
		cacheCost := float64(session.CacheTokens.Input+session.CacheTokens.Output) * 0.0 // Free
		localCost := float64(session.LocalTokens.Input+session.LocalTokens.Output) * 0.0 // Free
		cloudCost := session.TotalCost - cacheCost - localCost

		trends.TierBreakdown["cache"] += cacheCost
		trends.TierBreakdown["local"] += localCost
		trends.TierBreakdown["cloud"] += cloudCost
	}

	// Convert map to sorted slice
	for _, daily := range dailyMap {
		trends.DailyBreakdown = append(trends.DailyBreakdown, *daily)
	}

	// Sort by date
	for i := 0; i < len(trends.DailyBreakdown); i++ {
		for j := i + 1; j < len(trends.DailyBreakdown); j++ {
			if trends.DailyBreakdown[j].Date.Before(trends.DailyBreakdown[i].Date) {
				trends.DailyBreakdown[i], trends.DailyBreakdown[j] = trends.DailyBreakdown[j], trends.DailyBreakdown[i]
			}
		}
	}

	return trends
}

// =============================================================================
// SESSION MANAGEMENT
// =============================================================================

// EndSession closes the current session and starts a new one.
func (ct *CostTracker) EndSession() error {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	session := ct.sessions[ct.currentID]
	if session != nil {
		session.EndTime = time.Now()
		if err := ct.storage.Save(session); err != nil {
			return err
		}
	}

	// Start new session
	ct.currentID = generateSessionID()
	ct.sessions[ct.currentID] = &SessionCost{
		ID:         ct.currentID,
		StartTime:  time.Now(),
		TopQueries: make([]QueryCost, 0),
	}

	return nil
}

// SaveCurrentSession saves the current session to disk.
func (ct *CostTracker) SaveCurrentSession() error {
	ct.mu.RLock()
	session := ct.sessions[ct.currentID]
	ct.mu.RUnlock()

	if session == nil {
		return nil
	}

	return ct.storage.Save(session)
}

// =============================================================================
// HELPERS
// =============================================================================

// copySession creates a deep copy of a session.
func (ct *CostTracker) copySession(src *SessionCost) *SessionCost {
	dst := &SessionCost{
		ID:          src.ID,
		StartTime:   src.StartTime,
		EndTime:     src.EndTime,
		CacheTokens: src.CacheTokens,
		LocalTokens: src.LocalTokens,
		CloudTokens: src.CloudTokens,
		TotalCost:   src.TotalCost,
		Savings:     src.Savings,
		TopQueries:  make([]QueryCost, len(src.TopQueries)),
	}
	copy(dst.TopQueries, src.TopQueries)
	return dst
}

// generateSessionID generates a unique session ID.
func generateSessionID() string {
	// Use date format plus atomic counter for guaranteed uniqueness
	now := time.Now()
	counter := atomic.AddUint64(&sessionIDCounter, 1)
	return now.Format("20060102-150405") + "-" + fmt.Sprintf("%d", counter)
}
