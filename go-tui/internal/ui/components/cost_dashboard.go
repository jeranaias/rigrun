// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/telemetry"
)

// =============================================================================
// COST DASHBOARD
// =============================================================================

// CostDashboard displays cost information and analytics.
type CostDashboard struct {
	tracker *telemetry.CostTracker
	view    DashboardView
	width   int
	height  int
}

// DashboardView determines what the dashboard displays.
type DashboardView int

const (
	ViewSummary DashboardView = iota
	ViewHistory
	ViewBreakdown
)

// NewCostDashboard creates a new cost dashboard.
func NewCostDashboard(tracker *telemetry.CostTracker) *CostDashboard {
	return &CostDashboard{
		tracker: tracker,
		view:    ViewSummary,
	}
}

// SetView changes the dashboard view.
func (cd *CostDashboard) SetView(view DashboardView) {
	cd.view = view
}

// SetSize updates the dashboard dimensions.
func (cd *CostDashboard) SetSize(width, height int) {
	cd.width = width
	cd.height = height
}

// =============================================================================
// RENDERING
// =============================================================================

// View renders the dashboard based on the current view mode.
func (cd *CostDashboard) View() string {
	switch cd.view {
	case ViewHistory:
		return cd.renderHistory()
	case ViewBreakdown:
		return cd.renderBreakdown()
	default:
		return cd.renderSummary()
	}
}

// renderSummary shows the current session cost summary.
func (cd *CostDashboard) renderSummary() string {
	session := cd.tracker.GetCurrentSession()
	if session == nil {
		return "No session data available"
	}

	var b strings.Builder

	// Header
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	b.WriteString(titleStyle.Render("Cost Dashboard - Current Session"))
	b.WriteString("\n\n")

	// Session info
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	b.WriteString(labelStyle.Render("Session ID: "))
	b.WriteString(infoStyle.Render(session.ID))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Started: "))
	b.WriteString(infoStyle.Render(session.StartTime.Format("15:04:05")))
	b.WriteString("\n")

	duration := time.Since(session.StartTime)
	b.WriteString(labelStyle.Render("Duration: "))
	b.WriteString(infoStyle.Render(formatCostDuration(duration)))
	b.WriteString("\n\n")

	// Cost summary
	b.WriteString(cd.renderCostSummary(session))
	b.WriteString("\n\n")

	// Token breakdown
	b.WriteString(cd.renderTokenBreakdown(session))
	b.WriteString("\n\n")

	// Top queries
	b.WriteString(cd.renderTopQueries(session))

	return b.String()
}

// renderCostSummary renders the cost summary section.
func (cd *CostDashboard) renderCostSummary(session *telemetry.SessionCost) string {
	var b strings.Builder

	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	b.WriteString(sectionStyle.Render("Cost Summary"))
	b.WriteString("\n")

	// Total cost
	costStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	savingsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	b.WriteString(fmt.Sprintf("  Total Cost:    %s\n", costStyle.Render(fmt.Sprintf("$%.4f", session.TotalCost))))
	b.WriteString(fmt.Sprintf("  Savings:       %s (vs Opus all-cloud)\n", savingsStyle.Render(fmt.Sprintf("$%.4f", session.Savings))))

	// Efficiency
	efficiency := 0.0
	if session.TotalCost+session.Savings > 0 {
		efficiency = (session.Savings / (session.TotalCost + session.Savings)) * 100
	}
	efficiencyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	b.WriteString(fmt.Sprintf("  Efficiency:    %s\n", efficiencyStyle.Render(fmt.Sprintf("%.1f%%", efficiency))))

	return b.String()
}

// renderTokenBreakdown renders the token usage breakdown by tier.
func (cd *CostDashboard) renderTokenBreakdown(session *telemetry.SessionCost) string {
	var b strings.Builder

	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	b.WriteString(sectionStyle.Render("Token Usage by Tier"))
	b.WriteString("\n")

	totalTokens := session.CacheTokens.Input + session.CacheTokens.Output +
		session.LocalTokens.Input + session.LocalTokens.Output +
		session.CloudTokens.Input + session.CloudTokens.Output

	if totalTokens == 0 {
		b.WriteString("  No tokens used yet\n")
		return b.String()
	}

	// Cache tier
	cacheTotal := session.CacheTokens.Input + session.CacheTokens.Output
	cachePercent := float64(cacheTotal) / float64(totalTokens) * 100
	b.WriteString(fmt.Sprintf("  Cache:  %s %s\n",
		cd.renderBar(int(cachePercent), 20, "green"),
		fmt.Sprintf("%d tokens (%.1f%%)", cacheTotal, cachePercent)))

	// Local tier
	localTotal := session.LocalTokens.Input + session.LocalTokens.Output
	localPercent := float64(localTotal) / float64(totalTokens) * 100
	b.WriteString(fmt.Sprintf("  Local:  %s %s\n",
		cd.renderBar(int(localPercent), 20, "blue"),
		fmt.Sprintf("%d tokens (%.1f%%)", localTotal, localPercent)))

	// Cloud tier
	cloudTotal := session.CloudTokens.Input + session.CloudTokens.Output
	cloudPercent := float64(cloudTotal) / float64(totalTokens) * 100
	b.WriteString(fmt.Sprintf("  Cloud:  %s %s\n",
		cd.renderBar(int(cloudPercent), 20, "yellow"),
		fmt.Sprintf("%d tokens (%.1f%%)", cloudTotal, cloudPercent)))

	return b.String()
}

// renderTopQueries renders the top 5 most expensive queries.
func (cd *CostDashboard) renderTopQueries(session *telemetry.SessionCost) string {
	var b strings.Builder

	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	b.WriteString(sectionStyle.Render("Top 5 Most Expensive Queries"))
	b.WriteString("\n")

	if len(session.TopQueries) == 0 {
		b.WriteString("  No queries yet\n")
		return b.String()
	}

	// Show top 5
	count := len(session.TopQueries)
	if count > 5 {
		count = 5
	}

	for i := 0; i < count; i++ {
		query := session.TopQueries[i]
		b.WriteString(fmt.Sprintf("  %d. [%s] $%.4f - %s\n",
			i+1,
			query.Tier,
			query.Cost,
			query.Prompt))
	}

	return b.String()
}

// renderHistory shows cost trends over time.
func (cd *CostDashboard) renderHistory() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	b.WriteString(titleStyle.Render("Cost History - Last 7 Days"))
	b.WriteString("\n\n")

	trends := cd.tracker.GetTrends(7)
	if trends == nil || len(trends.DailyBreakdown) == 0 {
		b.WriteString("No historical data available\n")
		return b.String()
	}

	// Overall stats
	b.WriteString(fmt.Sprintf("Total Cost:   $%.4f\n", trends.TotalCost))
	b.WriteString(fmt.Sprintf("Total Saved:  $%.4f\n", trends.TotalSaved))
	b.WriteString("\n")

	// Daily chart
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	b.WriteString(sectionStyle.Render("Daily Breakdown"))
	b.WriteString("\n")

	// Find max cost for scaling
	maxCost := 0.0
	for _, daily := range trends.DailyBreakdown {
		if daily.Cost > maxCost {
			maxCost = daily.Cost
		}
	}

	for _, daily := range trends.DailyBreakdown {
		dateStr := daily.Date.Format("Mon Jan 2")
		barWidth := 0
		if maxCost > 0 {
			barWidth = int((daily.Cost / maxCost) * 30)
		}

		b.WriteString(fmt.Sprintf("  %-12s %s $%.4f (%d queries)\n",
			dateStr,
			cd.renderBar(barWidth, 30, "blue"),
			daily.Cost,
			daily.QueryCount))
	}

	return b.String()
}

// renderBreakdown shows detailed cost breakdown by tier.
func (cd *CostDashboard) renderBreakdown() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	b.WriteString(titleStyle.Render("Cost Breakdown by Tier"))
	b.WriteString("\n\n")

	trends := cd.tracker.GetTrends(30)
	if trends == nil {
		b.WriteString("No data available\n")
		return b.String()
	}

	totalCost := trends.TierBreakdown["cache"] + trends.TierBreakdown["local"] + trends.TierBreakdown["cloud"]

	if totalCost == 0 {
		b.WriteString("No costs recorded\n")
		return b.String()
	}

	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	b.WriteString(sectionStyle.Render("Last 30 Days"))
	b.WriteString("\n")

	// Tier breakdown
	for _, tier := range []string{"cache", "local", "cloud"} {
		cost := trends.TierBreakdown[tier]
		percent := (cost / totalCost) * 100

		b.WriteString(fmt.Sprintf("  %-6s %s $%.4f (%.1f%%)\n",
			strings.Title(tier),
			cd.renderBar(int(percent), 25, cd.tierColor(tier)),
			cost,
			percent))
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Total: $%.4f\n", totalCost))
	b.WriteString(fmt.Sprintf("Saved: $%.4f (vs all Opus)\n", trends.TotalSaved))

	return b.String()
}

// =============================================================================
// HELPERS
// =============================================================================

// renderBar renders a horizontal bar chart.
func (cd *CostDashboard) renderBar(value, maxWidth int, color string) string {
	if value < 0 {
		value = 0
	}
	if value > maxWidth {
		value = maxWidth
	}

	filled := strings.Repeat("#", value)
	empty := strings.Repeat("-", maxWidth-value)

	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	return barStyle.Render(filled) + emptyStyle.Render(empty)
}

// tierColor returns the color for a tier.
func (cd *CostDashboard) tierColor(tier string) string {
	switch tier {
	case "cache":
		return "2" // Green
	case "local":
		return "12" // Blue
	case "cloud":
		return "11" // Yellow
	default:
		return "7" // White
	}
}

// formatCostDuration formats a duration for display.
func formatCostDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}
