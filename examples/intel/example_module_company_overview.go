// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package examples demonstrates how to implement a competitive intelligence module
// This example shows Module 1: Company Overview
package examples

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/cache"
	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/tools"
)

// =============================================================================
// MODULE 1: COMPANY OVERVIEW
// =============================================================================
//
// Objective: Gather basic company information
//   - Company name, founded year, founders
//   - Mission statement, employee count
//   - Headquarters location, website
//
// Tools Used: WebSearch, WebFetch
// Routing: Local (qwen2.5:14b) - simple aggregation
// Cache TTL: 30 days (basic facts change rarely)
// =============================================================================

// CompanyOverviewModule researches basic company information.
type CompanyOverviewModule struct {
	executor  *tools.Executor
	registry  *tools.Registry
	cache     *cache.CacheManager
	router    *router.Router
	auditLog  *security.AuditLogger
}

// NewCompanyOverviewModule creates a new company overview research module.
func NewCompanyOverviewModule(
	executor *tools.Executor,
	registry *tools.Registry,
	cache *cache.CacheManager,
	routerCfg *router.Router,
	auditLog *security.AuditLogger,
) *CompanyOverviewModule {
	return &CompanyOverviewModule{
		executor:  executor,
		registry:  registry,
		cache:     cache,
		router:    routerCfg,
		auditLog:  auditLog,
	}
}

// CompanyOverviewResult holds the research results.
type CompanyOverviewResult struct {
	Name          string            `json:"name"`
	Founded       int               `json:"founded"`
	Founders      []string          `json:"founders"`
	Mission       string            `json:"mission"`
	Employees     int               `json:"employees"`
	Headquarters  string            `json:"headquarters"`
	Website       string            `json:"website"`
	Description   string            `json:"description"`
	RawData       map[string]string `json:"raw_data"`       // Raw search/fetch results
	Sources       []string          `json:"sources"`        // URLs accessed
	ResearchTime  time.Duration     `json:"research_time"`
	TokensUsed    int               `json:"tokens_used"`
	Cost          float64           `json:"cost"`
	CacheHit      bool              `json:"cache_hit"`
}

// Research executes the company overview research module.
func (m *CompanyOverviewModule) Research(ctx context.Context, companyName string, classification security.ClassificationLevel) (*CompanyOverviewResult, error) {
	startTime := time.Now()

	// Step 1: Check cache
	cacheKey := fmt.Sprintf("intel:overview:%s", strings.ToLower(companyName))
	if cached, hit := m.cache.Lookup(ctx, cacheKey); hit {
		m.auditLog.Log(&security.AuditEvent{
			Timestamp:      time.Now(),
			EventType:      security.EventTypeIntelResearch,
			Classification: classification,
			Details: map[string]interface{}{
				"module":     "company_overview",
				"company":    companyName,
				"cache_hit":  true,
			},
		})

		// Type assert cached result
		if result, ok := cached.(*CompanyOverviewResult); ok {
			result.CacheHit = true
			return result, nil
		}
	}

	result := &CompanyOverviewResult{
		Name:     companyName,
		RawData:  make(map[string]string),
		Sources:  make([]string, 0),
		CacheHit: false,
	}

	// Step 2: WebSearch for company overview
	searchQuery := fmt.Sprintf("%s company overview history founded", companyName)
	searchResult, err := m.executeWebSearch(ctx, searchQuery, classification)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}
	result.RawData["search_overview"] = searchResult.Output
	result.TokensUsed += searchResult.TokensUsed
	result.Cost += searchResult.Cost

	// Step 3: WebFetch company website /about page
	aboutURL := m.guessAboutURL(companyName)
	fetchResult, err := m.executeWebFetch(ctx, aboutURL, classification)
	if err == nil {
		result.RawData["about_page"] = fetchResult.Output
		result.Sources = append(result.Sources, aboutURL)
		result.TokensUsed += fetchResult.TokensUsed
		result.Cost += fetchResult.Cost
	}

	// Step 4: Extract structured data from raw results
	// Use local LLM to parse unstructured data into structured format
	extractedData, err := m.extractStructuredData(ctx, result.RawData, classification)
	if err != nil {
		return nil, fmt.Errorf("data extraction failed: %w", err)
	}

	// Populate structured fields
	result.Founded = extractedData.Founded
	result.Founders = extractedData.Founders
	result.Mission = extractedData.Mission
	result.Employees = extractedData.Employees
	result.Headquarters = extractedData.Headquarters
	result.Website = extractedData.Website
	result.Description = extractedData.Description

	result.ResearchTime = time.Since(startTime)

	// Step 5: Cache result (30 day TTL)
	m.cache.Store(ctx, cacheKey, result, 30*24*time.Hour)

	// Step 6: Audit log
	m.auditLog.Log(&security.AuditEvent{
		Timestamp:      time.Now(),
		EventType:      security.EventTypeIntelResearch,
		Classification: classification,
		Details: map[string]interface{}{
			"module":        "company_overview",
			"company":       companyName,
			"cache_hit":     false,
			"sources_count": len(result.Sources),
			"tokens_used":   result.TokensUsed,
			"cost_usd":      result.Cost,
			"duration_ms":   result.ResearchTime.Milliseconds(),
		},
	})

	return result, nil
}

// executeWebSearch performs a web search using the WebSearch tool.
func (m *CompanyOverviewModule) executeWebSearch(ctx context.Context, query string, classification security.ClassificationLevel) (*ToolExecutionResult, error) {
	// Create tool call
	toolCall := &tools.ToolCall{
		Name: "WebSearch",
		Params: map[string]interface{}{
			"query": query,
		},
	}

	// Execute tool
	toolResult, err := m.executor.Execute(ctx, m.registry.Get("WebSearch"), toolCall)
	if err != nil {
		return nil, err
	}

	if !toolResult.Success {
		return nil, fmt.Errorf("web search failed: %s", toolResult.Error)
	}

	// Route to local for simple parsing
	// (In full implementation, would call LLM to summarize results)
	return &ToolExecutionResult{
		Output:     toolResult.Output,
		TokensUsed: 0, // No LLM call for raw search
		Cost:       0,
	}, nil
}

// executeWebFetch fetches a URL using the WebFetch tool.
func (m *CompanyOverviewModule) executeWebFetch(ctx context.Context, url string, classification security.ClassificationLevel) (*ToolExecutionResult, error) {
	toolCall := &tools.ToolCall{
		Name: "WebFetch",
		Params: map[string]interface{}{
			"url": url,
		},
	}

	toolResult, err := m.executor.Execute(ctx, m.registry.Get("WebFetch"), toolCall)
	if err != nil {
		return nil, err
	}

	if !toolResult.Success {
		return nil, fmt.Errorf("web fetch failed: %s", toolResult.Error)
	}

	return &ToolExecutionResult{
		Output:     toolResult.Output,
		TokensUsed: 0,
		Cost:       0,
	}, nil
}

// guessAboutURL attempts to guess the company's about page URL.
func (m *CompanyOverviewModule) guessAboutURL(companyName string) string {
	// Simple heuristic - in production would use more sophisticated logic
	domain := strings.ToLower(strings.ReplaceAll(companyName, " ", ""))
	return fmt.Sprintf("https://www.%s.com/about", domain)
}

// extractStructuredData uses a local LLM to extract structured data from raw text.
func (m *CompanyOverviewModule) extractStructuredData(ctx context.Context, rawData map[string]string, classification security.ClassificationLevel) (*ExtractedData, error) {
	// Combine all raw data into a single context
	var contextBuilder strings.Builder
	for source, data := range rawData {
		contextBuilder.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", source, data))
	}

	// Create extraction prompt
	prompt := fmt.Sprintf(`Extract company information from the following data:

%s

Please extract and return ONLY a JSON object with these fields:
{
  "founded": <year as integer>,
  "founders": ["founder1", "founder2"],
  "mission": "mission statement",
  "employees": <count as integer>,
  "headquarters": "city, state/country",
  "website": "https://...",
  "description": "2-3 sentence company description"
}

If a field is unknown, use null. Return ONLY valid JSON, no explanation.`, contextBuilder.String())

	// Route this query - extraction is simple, use local
	tier := router.TierLocal
	if classification >= security.ClassificationCUI {
		// CUI classification forces local
		tier = router.TierLocal
	}

	// In full implementation, would call ollama.Chat() here
	// For this example, returning mock data
	return &ExtractedData{
		Founded:      2021,
		Founders:     []string{"Dario Amodei", "Daniela Amodei"},
		Mission:      "Build reliable, interpretable, and steerable AI systems",
		Employees:    150,
		Headquarters: "San Francisco, CA",
		Website:      "https://www.anthropic.com",
		Description:  "Anthropic is an AI safety company focused on building reliable, interpretable, and steerable AI systems.",
	}, nil
}

// ToolExecutionResult holds the result of a tool execution.
type ToolExecutionResult struct {
	Output     string
	TokensUsed int
	Cost       float64
}

// ExtractedData holds structured data extracted from raw text.
type ExtractedData struct {
	Founded      int
	Founders     []string
	Mission      string
	Employees    int
	Headquarters string
	Website      string
	Description  string
}

// =============================================================================
// USAGE EXAMPLE
// =============================================================================

/*
func main() {
	// Initialize dependencies
	executor := tools.NewExecutor(registry, config)
	registry := tools.NewRegistry()
	cache := cache.NewCacheManager()
	router := router.NewRouter(config)
	auditLog := security.NewAuditLogger(config)

	// Create module
	module := NewCompanyOverviewModule(executor, registry, cache, router, auditLog)

	// Research company
	result, err := module.Research(
		context.Background(),
		"Anthropic",
		security.ClassificationUnclassified,
	)

	if err != nil {
		log.Fatalf("Research failed: %v", err)
	}

	// Print results
	fmt.Printf("Company: %s\n", result.Name)
	fmt.Printf("Founded: %d by %s\n", result.Founded, strings.Join(result.Founders, ", "))
	fmt.Printf("Mission: %s\n", result.Mission)
	fmt.Printf("Employees: %d\n", result.Employees)
	fmt.Printf("HQ: %s\n", result.Headquarters)
	fmt.Printf("\nResearch Stats:\n")
	fmt.Printf("  Time: %v\n", result.ResearchTime)
	fmt.Printf("  Tokens: %d\n", result.TokensUsed)
	fmt.Printf("  Cost: $%.4f\n", result.Cost)
	fmt.Printf("  Cache Hit: %v\n", result.CacheHit)
}
*/
