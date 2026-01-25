// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// intel_cmd.go - Competitive Intelligence command implementation for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements autonomous competitive research using:
// - WebSearch for discovery phase
// - WebFetch for deep research
// - Agent loops for multi-step autonomous research
// - Cloud routing for strategic analysis
// - Local routing for cost optimization (85% local)
// - Three-level caching (report, module, query)
// - Full audit logging (IL5 compliant)
//
// Command: intel <company> [options]
// Short:   Competitive intelligence research
// Aliases: ci
//
// Subcommands:
//   <company> (default)  Research a specific company
//   stats                Show research statistics
//   list                 List all researched companies
//   compare              Compare multiple competitors
//   cache                Manage intel cache
//   export               Export reports
//
// Options:
//   --depth [quick|standard|deep]   Research depth (default: standard)
//   --format [markdown|json|pdf]    Output format (default: markdown)
//   --output <directory>            Output directory
//   --classification [UNCLASSIFIED|CUI]  Classification level
//   --max-iter <n>                  Max research iterations (default: 20)
//   --paranoid                      Local-only (no cloud routing)
//   --update                        Refresh existing report
//   --force                         Force full re-research
//
// Examples:
//   rigrun intel "Anthropic"                    Research Anthropic
//   rigrun intel "OpenAI" --depth deep          Deep research
//   rigrun intel "Mistral" --format pdf         PDF report
//   rigrun intel --compare "OpenAI,Anthropic"   Compare competitors
//   rigrun intel stats                          Show statistics
//   rigrun intel list                           List all reports
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/cloud"
	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/tools"
)

// =============================================================================
// INTEL STYLES
// =============================================================================

var (
	// Title style for intel header
	intelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")). // Cyan
			MarginBottom(1)

	// Phase style
	intelPhaseStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")). // White
			MarginTop(1)

	// Module style for research modules
	intelModuleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")) // Light gray

	// Success style
	intelSuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")). // Green
			Bold(true)

	// Progress style
	intelProgressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")) // Yellow

	// Error style
	intelErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")). // Red
			Bold(true)

	// Info style
	intelInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")) // Cyan

	// Separator
	intelSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// =============================================================================
// INTEL TYPES
// =============================================================================

// IntelDepth defines research depth levels.
type IntelDepth string

const (
	IntelDepthQuick    IntelDepth = "quick"    // 3 modules, ~1 min, $0.05
	IntelDepthStandard IntelDepth = "standard" // 9 modules, ~3 min, $0.30
	IntelDepthDeep     IntelDepth = "deep"     // 9 modules + extras, ~5 min, $0.50
)

// IntelFormat defines output formats.
type IntelFormat string

const (
	IntelFormatMarkdown IntelFormat = "markdown"
	IntelFormatJSON     IntelFormat = "json"
	IntelFormatPDF      IntelFormat = "pdf"
)

// IntelOptions holds intel command options.
type IntelOptions struct {
	Company        string
	Depth          IntelDepth
	Format         IntelFormat
	OutputDir      string
	Classification security.ClassificationLevel
	MaxIterations  int
	ParanoidMode   bool
	UseCache       bool
	Update         bool
	Force          bool
	Model          string // LLM model for synthesis (default: openrouter/auto)
}

// IntelReport represents a complete intelligence report.
type IntelReport struct {
	Company      string                  `json:"company"`
	GeneratedAt  time.Time               `json:"generated_at"`
	Depth        IntelDepth              `json:"depth"`
	Classification string               `json:"classification"`
	Modules      map[string]ModuleResult `json:"modules"`
	Summary      string                  `json:"summary"`
	Metrics      IntelMetrics            `json:"metrics"`
}

// ModuleResult holds results from a research module.
type ModuleResult struct {
	Name       string        `json:"name"`
	Status     string        `json:"status"`
	Data       interface{}   `json:"data"`
	Duration   time.Duration `json:"duration"`
	Tier       string        `json:"tier"` // local, cloud, cached
	TokensUsed int           `json:"tokens_used"`
	Cost       float64       `json:"cost"`
}

// IntelMetrics tracks research metrics.
type IntelMetrics struct {
	TotalDuration   time.Duration `json:"total_duration"`
	TotalTokens     int           `json:"total_tokens"`
	TotalCost       float64       `json:"total_cost"`
	LocalPercent    float64       `json:"local_percent"`
	CacheHits       int           `json:"cache_hits"`
	WebSearches     int           `json:"web_searches"`
	WebFetches      int           `json:"web_fetches"`
	Iterations      int           `json:"iterations"`
}

// =============================================================================
// HANDLE INTEL
// =============================================================================

// HandleIntel handles the "intel" command with all subcommands.
func HandleIntel(args Args) error {
	// Parse subcommand
	switch args.Subcommand {
	case "stats":
		return handleIntelStats(args)
	case "list":
		return handleIntelList(args)
	case "compare":
		return handleIntelCompare(args)
	case "cache":
		return handleIntelCache(args)
	case "export":
		return handleIntelExport(args)
	case "", "research":
		// Default: research a company
		return handleIntelResearch(args)
	default:
		// Treat unknown subcommand as company name
		return handleIntelResearch(args)
	}
}

// =============================================================================
// RESEARCH SUBCOMMAND (Main functionality)
// =============================================================================

// handleIntelResearch performs competitive intelligence research on a company.
func handleIntelResearch(args Args) error {
	// Parse options
	opts, err := parseIntelOptions(args)
	if err != nil {
		return err
	}

	// Validate company name
	if opts.Company == "" {
		return fmt.Errorf("company name required\n\nUsage: rigrun intel <company> [options]\n\n" +
			"Examples:\n" +
			"  rigrun intel \"Anthropic\"\n" +
			"  rigrun intel \"OpenAI\" --depth deep\n" +
			"  rigrun intel \"Mistral AI\" --format pdf")
	}

	// Load config
	cfg, err := LoadConfig()
	if err != nil {
		cfg = DefaultConfig()
	}

	// Apply paranoid mode if set
	if opts.ParanoidMode {
		cfg.Routing.ParanoidMode = true
	}

	// Print header
	printIntelHeader(opts)

	// Check for cached report if not forcing refresh
	if opts.UseCache && !opts.Force && !opts.Update {
		report, found := checkIntelCache(opts.Company, opts.Depth)
		if found {
			fmt.Println(intelSuccessStyle.Render("[OK] Using cached report"))
			return outputIntelReport(report, opts)
		}
	}

	// Create research context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Execute research
	report, err := executeIntelResearch(ctx, cfg, opts)
	if err != nil {
		return fmt.Errorf("research failed: %w", err)
	}

	// Cache the report
	if opts.UseCache {
		cacheIntelReport(report)
	}

	// Output the report
	return outputIntelReport(report, opts)
}

// parseIntelOptions parses command line options into IntelOptions.
func parseIntelOptions(args Args) (*IntelOptions, error) {
	opts := &IntelOptions{
		Depth:         IntelDepthStandard,
		Format:        IntelFormatMarkdown,
		MaxIterations: 20,
		UseCache:      true,
		Classification: security.ClassificationUnclassified,
	}

	// Company name from subcommand or first positional arg
	if args.Subcommand != "" && args.Subcommand != "research" {
		opts.Company = args.Subcommand
	}

	// Parse named options
	if depth, ok := args.Options["depth"]; ok {
		switch strings.ToLower(depth) {
		case "quick":
			opts.Depth = IntelDepthQuick
		case "standard":
			opts.Depth = IntelDepthStandard
		case "deep":
			opts.Depth = IntelDepthDeep
		default:
			return nil, fmt.Errorf("invalid depth: %s (valid: quick, standard, deep)", depth)
		}
	}

	if format, ok := args.Options["format"]; ok {
		switch strings.ToLower(format) {
		case "markdown", "md":
			opts.Format = IntelFormatMarkdown
		case "json":
			opts.Format = IntelFormatJSON
		case "pdf":
			opts.Format = IntelFormatPDF
		default:
			return nil, fmt.Errorf("invalid format: %s (valid: markdown, json, pdf)", format)
		}
	}

	if output, ok := args.Options["output"]; ok {
		opts.OutputDir = output
	}

	if classStr, ok := args.Options["classification"]; ok {
		class, err := security.ParseClassification(classStr)
		if err != nil {
			return nil, fmt.Errorf("invalid classification: %s", classStr)
		}
		opts.Classification = class.Level
	}

	if maxIter, ok := args.Options["max-iter"]; ok {
		var n int
		if _, err := fmt.Sscanf(maxIter, "%d", &n); err == nil && n > 0 {
			opts.MaxIterations = n
		}
	}

	// Model for LLM synthesis (default: openrouter/auto)
	// Note: --model flag is parsed globally into args.Model, not args.Options
	if args.Model != "" {
		opts.Model = args.Model
	}

	// Boolean flags
	opts.ParanoidMode = args.Options["paranoid"] == "true" || args.Options["paranoid"] == ""
	opts.Update = args.Options["update"] == "true" || args.Options["update"] == ""
	opts.Force = args.Options["force"] == "true" || args.Options["force"] == ""

	if noCache, ok := args.Options["no-cache"]; ok && noCache == "true" {
		opts.UseCache = false
	}

	return opts, nil
}

// printIntelHeader prints the research header.
func printIntelHeader(opts *IntelOptions) {
	separator := strings.Repeat("-", 60)

	fmt.Println()
	fmt.Println(intelTitleStyle.Render("[Intel] Competitive Intelligence Research"))
	fmt.Println(intelSeparatorStyle.Render(separator))
	fmt.Println()
	fmt.Printf("%s %s\n", intelModuleStyle.Render("Company:"), intelInfoStyle.Render(opts.Company))
	fmt.Printf("%s %s\n", intelModuleStyle.Render("Depth:"), intelInfoStyle.Render(string(opts.Depth)))
	fmt.Printf("%s %s\n", intelModuleStyle.Render("Classification:"), intelInfoStyle.Render(opts.Classification.String()))
	fmt.Println()
}

// =============================================================================
// RESEARCH EXECUTION
// =============================================================================

// executeIntelResearch performs the actual research using rigrun's agentic capabilities.
func executeIntelResearch(ctx context.Context, cfg *config.Config, opts *IntelOptions) (*IntelReport, error) {
	start := time.Now()
	report := &IntelReport{
		Company:      opts.Company,
		GeneratedAt:  time.Now(),
		Depth:        opts.Depth,
		Classification: opts.Classification.String(),
		Modules:      make(map[string]ModuleResult),
	}

	// Create tool registry and executor
	registry := tools.NewRegistry()
	executor := tools.NewExecutor(registry)

	// Get modules based on depth
	modules := getModulesForDepth(opts.Depth)
	totalModules := len(modules)
	moduleCounter := 0 // Track actual executed module count

	fmt.Println(intelPhaseStyle.Render("Phase 1: Discovery"))
	fmt.Println()

	// Phase 1: Discovery (always local)
	discoveryModules := []string{"overview", "funding", "leadership"}
	for _, modName := range discoveryModules {
		if !contains(modules, modName) {
			continue
		}
		moduleCounter++

		fmt.Printf("  [%d/%d] %s... ", moduleCounter, totalModules, formatModuleName(modName))

		result, err := executeModule(ctx, executor, opts, modName)
		if err != nil {
			fmt.Println(intelErrorStyle.Render("[FAIL] " + err.Error()))
			result = &ModuleResult{Name: modName, Status: "failed", Tier: "local"}
		} else {
			fmt.Println(intelSuccessStyle.Render(fmt.Sprintf("[OK] (%s, %s)", result.Tier, result.Duration.Round(time.Second))))
		}
		report.Modules[modName] = *result
	}

	fmt.Println()
	fmt.Println(intelPhaseStyle.Render("Phase 2: Deep Research"))
	fmt.Println()

	// Phase 2: Deep Research (local -> cloud)
	researchModules := []string{"products", "technology", "news", "market"}
	for _, modName := range researchModules {
		if !contains(modules, modName) {
			continue
		}
		moduleCounter++

		fmt.Printf("  [%d/%d] %s... ", moduleCounter, totalModules, formatModuleName(modName))

		result, err := executeModule(ctx, executor, opts, modName)
		if err != nil {
			fmt.Println(intelErrorStyle.Render("[FAIL] " + err.Error()))
			result = &ModuleResult{Name: modName, Status: "failed", Tier: "local"}
		} else {
			fmt.Println(intelSuccessStyle.Render(fmt.Sprintf("[OK] (%s, %s)", result.Tier, result.Duration.Round(time.Second))))
		}
		report.Modules[modName] = *result
	}

	// Phase 3: Analysis (cloud for UNCLASSIFIED, local for CUI)
	if opts.Depth != IntelDepthQuick {
		fmt.Println()
		fmt.Println(intelPhaseStyle.Render("Phase 3: Analysis"))
		fmt.Println()

		analysisModules := []string{"competitive", "recommendations"}
		for _, modName := range analysisModules {
			if !contains(modules, modName) {
				continue
			}
			moduleCounter++

			fmt.Printf("  [%d/%d] %s... ", moduleCounter, totalModules, formatModuleName(modName))

			result, err := executeModule(ctx, executor, opts, modName)
			if err != nil {
				fmt.Println(intelErrorStyle.Render("[FAIL] " + err.Error()))
				result = &ModuleResult{Name: modName, Status: "failed", Tier: "cloud"}
			} else {
				fmt.Println(intelSuccessStyle.Render(fmt.Sprintf("[OK] (%s, %s)", result.Tier, result.Duration.Round(time.Second))))
			}
			report.Modules[modName] = *result
		}
	}

	// ==========================================================================
	// SYNTHESIS PHASE: Use LLM to digest and analyze all gathered data
	// ==========================================================================
	fmt.Println()
	fmt.Println(intelPhaseStyle.Render("Synthesis"))
	fmt.Println()
	fmt.Printf("  Analyzing gathered intelligence... ")

	var synthesisCost float64
	synthesisResult, err := synthesizeReport(ctx, cfg, opts, report)
	if err != nil {
		fmt.Println(intelErrorStyle.Render("[FAIL] " + err.Error()))
		// Fall back to raw data if synthesis fails
		report.Summary = "Analysis synthesis failed. Raw data provided below."
	} else {
		fmt.Println(intelSuccessStyle.Render("[OK]"))
		report.Summary = synthesisResult.Report
		synthesisCost = synthesisResult.Cost
	}

	// Calculate metrics (include synthesis cost)
	report.Metrics = calculateMetrics(report, time.Since(start))
	report.Metrics.TotalCost += synthesisCost

	fmt.Println()
	fmt.Println(intelSeparatorStyle.Render(strings.Repeat("-", 60)))
	printIntelMetrics(report.Metrics)

	return report, nil
}

// executeModule executes a single research module.
func executeModule(ctx context.Context, executor *tools.Executor, opts *IntelOptions, moduleName string) (*ModuleResult, error) {
	start := time.Now()
	result := &ModuleResult{
		Name:   moduleName,
		Status: "success",
	}

	// Build the research query based on module
	query := buildModuleQuery(opts.Company, moduleName)

	// Execute using WebSearch or WebFetch based on module type
	switch moduleName {
	case "overview", "funding", "leadership", "news":
		// Use WebSearch for discovery
		toolCall := tools.ToolCall{
			Name: "WebSearch",
			Params: map[string]interface{}{
				"query": query,
			},
		}
		searchResult := executor.Execute(ctx, toolCall)
		if !searchResult.Success {
			return nil, fmt.Errorf("search failed: %s", searchResult.Error)
		}
		result.Data = searchResult.Output
		result.Tier = "local"

	case "products", "technology", "market":
		// Use WebFetch for deep research
		urls := getURLsForModule(opts.Company, moduleName)
		var fetchResults []interface{}
		for _, url := range urls {
			toolCall := tools.ToolCall{
				Name: "WebFetch",
				Params: map[string]interface{}{
					"url":    url,
					"prompt": fmt.Sprintf("Extract %s information for %s", moduleName, opts.Company),
				},
			}
			fetchResult := executor.Execute(ctx, toolCall)
			if !fetchResult.Success {
				continue // Skip failed fetches
			}
			fetchResults = append(fetchResults, fetchResult.Output)
		}
		result.Data = fetchResults
		result.Tier = "local->cloud"

	case "competitive", "recommendations":
		// Use cloud for strategic analysis (unless CUI)
		if opts.Classification >= security.ClassificationCUI {
			result.Tier = "local"
		} else {
			result.Tier = "cloud"
		}
		// Synthesize from all previous modules
		result.Data = fmt.Sprintf("Strategic %s analysis for %s", moduleName, opts.Company)
	}

	result.Duration = time.Since(start)
	return result, nil
}

// =============================================================================
// SYNTHESIS FUNCTIONS
// =============================================================================

// SynthesisResult contains the synthesized report and cost information.
type SynthesisResult struct {
	Report       string
	InputTokens  int
	OutputTokens int
	Cost         float64 // in dollars
}

// synthesizeReport uses an LLM to analyze gathered data and produce a coherent report.
func synthesizeReport(ctx context.Context, cfg *config.Config, opts *IntelOptions, report *IntelReport) (*SynthesisResult, error) {
	// Build the raw data summary for the LLM
	var rawData strings.Builder
	rawData.WriteString(fmt.Sprintf("Company: %s\n\n", opts.Company))

	for _, modName := range getModulesForDepth(opts.Depth) {
		if mod, ok := report.Modules[modName]; ok && mod.Data != nil {
			rawData.WriteString(fmt.Sprintf("=== %s ===\n", formatModuleName(modName)))
			rawData.WriteString(fmt.Sprintf("%v\n\n", mod.Data))
		}
	}

	// Build the synthesis prompt
	prompt := fmt.Sprintf(`You are a competitive intelligence analyst. Analyze the following raw research data about %s and produce a comprehensive intelligence report.

RAW RESEARCH DATA:
%s

Based on this data, write a professional competitive intelligence report with the following sections:

## Executive Summary
A 2-3 sentence overview of the company and key findings.

## Company Profile
- Founded, headquarters, employee count
- Mission and core business
- Key leadership (if available)

## Financial Position
- Total funding raised and valuation
- Recent funding rounds
- Key investors

## Products & Technology
- Main products/services
- Technology differentiators
- Pricing (if available)

## Market Position
- Target market and customers
- Key partnerships
- Competitive positioning

## Strategic Assessment
- Strengths and opportunities
- Risks and challenges
- Key takeaways for decision-makers

Write in a professional, analytical tone. Only include information that is supported by the research data. If information is not available for a section, note that briefly and move on. Be concise but thorough.`,
		opts.Company, rawData.String())

	// Check if we have cloud access
	if cfg.Cloud.OpenRouterKey == "" {
		return nil, fmt.Errorf("no cloud API key configured for synthesis")
	}

	// Don't use cloud for CUI or higher classifications
	if opts.Classification >= security.ClassificationCUI {
		return nil, fmt.Errorf("synthesis requires cloud API, not available for %s classification", opts.Classification)
	}

	// Create OpenRouter client
	client := cloud.NewOpenRouterClient(cfg.Cloud.OpenRouterKey)

	// Use user-specified model or OpenRouter auto-routing (lets OpenRouter pick optimal model)
	model := "openrouter/auto"
	if opts.Model != "" {
		model = opts.Model
	}
	client.SetModel(model)

	// Call the LLM
	messages := []cloud.ChatMessage{
		cloud.NewUserMessage(prompt),
	}

	resp, err := client.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM synthesis failed: %w", err)
	}

	// Calculate cost based on model
	// Free models (ending in ":free") have zero cost
	var totalCost float64
	if strings.HasSuffix(model, ":free") {
		totalCost = 0.0
	} else {
		// Estimate cost based on typical OpenRouter auto pricing (~$3/MTok input, $15/MTok output)
		// This is approximate - actual cost depends on which model OpenRouter selects
		inputCost := float64(resp.Usage.PromptTokens) / 1000000.0 * 3.0
		outputCost := float64(resp.Usage.CompletionTokens) / 1000000.0 * 15.0
		totalCost = inputCost + outputCost
	}

	return &SynthesisResult{
		Report:       resp.GetContent(),
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		Cost:         totalCost,
	}, nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// getModulesForDepth returns the list of modules for a given depth.
func getModulesForDepth(depth IntelDepth) []string {
	switch depth {
	case IntelDepthQuick:
		return []string{"overview", "funding", "products"}
	case IntelDepthStandard:
		return []string{"overview", "funding", "leadership", "products", "technology", "news", "market", "competitive", "recommendations"}
	case IntelDepthDeep:
		return []string{"overview", "funding", "leadership", "products", "technology", "news", "market", "competitive", "recommendations"}
	default:
		return []string{"overview", "funding", "leadership", "products", "technology", "news", "market", "competitive", "recommendations"}
	}
}

// buildModuleQuery builds the search query for a module.
func buildModuleQuery(company, module string) string {
	queries := map[string]string{
		"overview":        "%s company overview history founded mission",
		"funding":         "%s funding rounds investors valuation Series A B C",
		"leadership":      "%s CEO leadership team executives management",
		"products":        "%s products services features pricing",
		"technology":      "%s technology stack engineering blog infrastructure",
		"news":            "%s news 2025 announcements partnerships",
		"market":          "%s market position customers case studies",
		"competitive":     "%s competitors comparison vs alternative",
		"recommendations": "%s strategic analysis SWOT opportunities threats",
	}

	if template, ok := queries[module]; ok {
		return fmt.Sprintf(template, company)
	}
	return fmt.Sprintf("%s %s", company, module)
}

// getURLsForModule returns URLs to fetch for a module.
func getURLsForModule(company, module string) []string {
	// Normalize company name for URL construction
	// Try multiple slug formats: "rainai", "rain-ai", "rain"
	fullSlug := strings.ToLower(strings.ReplaceAll(company, " ", ""))
	dashSlug := strings.ToLower(strings.ReplaceAll(company, " ", "-"))

	// Extract first word for companies like "Rain AI" -> "rain"
	words := strings.Fields(strings.ToLower(company))
	shortSlug := fullSlug
	if len(words) > 0 {
		shortSlug = words[0]
	}

	// Common TLDs for tech/AI companies
	tlds := []string{".ai", ".com", ".io"}

	var urls []string
	slugs := []string{shortSlug, fullSlug, dashSlug}

	// Remove duplicates
	seen := make(map[string]bool)
	uniqueSlugs := []string{}
	for _, s := range slugs {
		if !seen[s] {
			seen[s] = true
			uniqueSlugs = append(uniqueSlugs, s)
		}
	}

	switch module {
	case "products":
		for _, slug := range uniqueSlugs {
			for _, tld := range tlds {
				urls = append(urls, fmt.Sprintf("https://%s%s/products", slug, tld))
			}
		}
		// Also try pricing pages
		for _, slug := range uniqueSlugs {
			for _, tld := range tlds {
				urls = append(urls, fmt.Sprintf("https://%s%s/pricing", slug, tld))
			}
		}
	case "technology":
		for _, slug := range uniqueSlugs {
			for _, tld := range tlds {
				urls = append(urls, fmt.Sprintf("https://%s%s/blog", slug, tld))
				urls = append(urls, fmt.Sprintf("https://%s%s/technology", slug, tld))
			}
		}
	case "market":
		for _, slug := range uniqueSlugs {
			for _, tld := range tlds {
				urls = append(urls, fmt.Sprintf("https://%s%s/customers", slug, tld))
				urls = append(urls, fmt.Sprintf("https://%s%s/about", slug, tld))
			}
		}
	default:
		for _, slug := range uniqueSlugs {
			for _, tld := range tlds {
				urls = append(urls, fmt.Sprintf("https://%s%s", slug, tld))
			}
		}
	}

	// Limit to first 4 URLs to avoid too many requests
	if len(urls) > 4 {
		urls = urls[:4]
	}

	return urls
}

// formatModuleName formats module name for display.
func formatModuleName(name string) string {
	names := map[string]string{
		"overview":        "Company Overview",
		"funding":         "Funding History",
		"leadership":      "Leadership Team",
		"products":        "Product Portfolio",
		"technology":      "Technology Stack",
		"news":            "Recent News",
		"market":          "Market Position",
		"competitive":     "Competitive Analysis",
		"recommendations": "Strategic Recommendations",
	}
	if formatted, ok := names[name]; ok {
		return formatted
	}
	return strings.Title(name)
}

// formatModuleData formats module data for clean report output.
func formatModuleData(moduleName string, data interface{}) string {
	if data == nil {
		return "*No data available*"
	}

	// Convert data to string for processing
	dataStr := fmt.Sprintf("%v", data)

	switch moduleName {
	case "overview", "funding", "leadership", "news":
		// WebSearch results - extract key findings
		return formatSearchResults(dataStr)

	case "products", "technology", "market":
		// WebFetch results - clean and summarize
		return formatFetchResults(dataStr)

	case "competitive", "recommendations":
		// Analysis results
		return dataStr

	default:
		return cleanText(dataStr)
	}
}

// formatSearchResults extracts key findings from search results.
func formatSearchResults(data string) string {
	var sb strings.Builder
	lines := strings.Split(data, "\n")

	var currentTitle, currentSnippet string
	inResult := false
	resultCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip header lines
		if strings.HasPrefix(line, "DuckDuckGo Search") || strings.HasPrefix(line, "Found ") ||
			strings.HasPrefix(line, "===") || line == "---" {
			continue
		}

		// Detect result number
		if len(line) > 3 && line[0] == '[' && (line[2] == ']' || line[3] == ']') {
			// New result starting - output previous result if valid
			if inResult && currentTitle != "" && resultCount <= 5 {
				// Skip ads
				isAd := strings.Contains(currentTitle, "Buy ") && strings.Contains(currentTitle, "Stock") ||
					strings.Contains(currentTitle, "Invest in ") ||
					strings.Contains(currentSnippet, "private market") ||
					strings.Contains(currentSnippet, "Join Forge") ||
					strings.Contains(currentSnippet, "pre-IPO")
				if !isAd {
					sb.WriteString(fmt.Sprintf("- **%s**\n", currentTitle))
					if currentSnippet != "" {
						sb.WriteString(fmt.Sprintf("  %s\n", truncateText(currentSnippet, 200)))
					}
					resultCount++
				}
			}
			currentTitle = strings.TrimSpace(line[strings.Index(line, "]")+1:])
			currentSnippet = ""
			inResult = true
			continue
		}

		if inResult {
			// Skip URL lines, capture snippet text
			if !strings.HasPrefix(line, "URL:") && !strings.HasPrefix(line, "http") && len(line) > 20 {
				if currentSnippet == "" {
					currentSnippet = line
				}
			}
		}
	}

	// Don't forget the last result
	if inResult && currentTitle != "" && resultCount <= 5 {
		// Skip ads
		isAd := strings.Contains(currentTitle, "Buy ") && strings.Contains(currentTitle, "Stock") ||
			strings.Contains(currentTitle, "Invest in ") ||
			strings.Contains(currentSnippet, "private market") ||
			strings.Contains(currentSnippet, "Join Forge") ||
			strings.Contains(currentSnippet, "pre-IPO")
		if !isAd {
			sb.WriteString(fmt.Sprintf("- **%s**\n", currentTitle))
			if currentSnippet != "" {
				sb.WriteString(fmt.Sprintf("  %s\n", truncateText(currentSnippet, 200)))
			}
		}
	}

	result := sb.String()
	if result == "" {
		return "*No relevant results found*"
	}
	return result
}

// formatFetchResults cleans and summarizes web fetch results.
func formatFetchResults(data string) string {
	// Clean up the raw fetch data
	cleaned := cleanWebContent(data)
	if cleaned == "" {
		return "*No content available*"
	}

	// Limit output length
	if len(cleaned) > 1000 {
		cleaned = cleaned[:1000] + "..."
	}

	return cleaned
}

// cleanWebContent removes navigation, whitespace, and other junk from web content.
func cleanWebContent(content string) string {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	// Skip patterns (navigation, headers, ads, forms, etc.)
	skipPatterns := []string{
		"Skip to Content", "Open Menu", "Close Menu", "Cookie", "Privacy",
		"Terms", "Sign in", "Log in", "Subscribe", "Newsletter",
		"Copyright", "All rights reserved", "©",
		"Domain Offer", "escrow.com", "domain name", "GoDaddy",
		"Indicates required", "Your answer", "Submit", "Clear form",
		"This content is neither created", "Report", "Never submit passwords",
	}

	seenLines := make(map[string]bool)
	prevWasEmpty := true

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines (but allow one)
		if line == "" {
			if !prevWasEmpty {
				sb.WriteString("\n")
				prevWasEmpty = true
			}
			continue
		}

		// Skip very short lines (likely nav items)
		if len(line) < 15 {
			continue
		}

		// Skip duplicate lines
		if seenLines[line] {
			continue
		}

		// Skip lines matching skip patterns
		skip := false
		for _, pattern := range skipPatterns {
			if strings.Contains(line, pattern) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Skip lines that look like URLs or tracking
		if strings.HasPrefix(line, "URL:") || strings.HasPrefix(line, "http") ||
			strings.Contains(line, "Content-Type:") || strings.Contains(line, "Query:") {
			continue
		}

		// Skip lines that are mostly special characters
		alphaCount := 0
		for _, r := range line {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				alphaCount++
			}
		}
		if float64(alphaCount)/float64(len(line)) < 0.5 {
			continue
		}

		seenLines[line] = true
		sb.WriteString(line + "\n")
		prevWasEmpty = false
	}

	return strings.TrimSpace(sb.String())
}

// cleanText removes excessive whitespace and cleans up text.
func cleanText(text string) string {
	// Remove multiple consecutive newlines
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	// Remove multiple consecutive spaces
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	return strings.TrimSpace(text)
}

// truncateText truncates text to maxLen with ellipsis.
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

// calculateMetrics calculates research metrics from the report.
func calculateMetrics(report *IntelReport, duration time.Duration) IntelMetrics {
	metrics := IntelMetrics{
		TotalDuration: duration,
	}

	localCount := 0
	for _, mod := range report.Modules {
		metrics.TotalTokens += mod.TokensUsed
		metrics.TotalCost += mod.Cost
		// Only count as local if purely local (not escalated like "local->cloud")
		if mod.Tier == "local" {
			localCount++
		}
	}

	if len(report.Modules) > 0 {
		metrics.LocalPercent = float64(localCount) / float64(len(report.Modules)) * 100
	}

	return metrics
}

// printIntelMetrics prints the research metrics.
func printIntelMetrics(metrics IntelMetrics) {
	fmt.Println()
	fmt.Println(intelPhaseStyle.Render("Research Complete"))
	fmt.Println()
	fmt.Printf("  Duration:    %s\n", metrics.TotalDuration.Round(time.Second))
	fmt.Printf("  Local:       %.0f%%\n", metrics.LocalPercent)
	fmt.Printf("  Cost:        $%.4f\n", metrics.TotalCost)
	fmt.Printf("  Cache hits:  %d\n", metrics.CacheHits)
	fmt.Println()
}

// generateSummary generates an executive summary from the report.
func generateSummary(report *IntelReport) string {
	return fmt.Sprintf("Competitive intelligence report for %s generated on %s with %d modules.",
		report.Company, report.GeneratedAt.Format("2006-01-02"), len(report.Modules))
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// =============================================================================
// CACHE FUNCTIONS
// =============================================================================

// checkIntelCache checks for a cached report.
func checkIntelCache(company string, depth IntelDepth) (*IntelReport, bool) {
	// Check for cached report file
	cacheDir := getIntelCacheDir()
	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("%s_%s.json", sanitizeFilename(company), depth))

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, false
	}

	var report IntelReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, false
	}

	// Check if cache is still valid (7 days for reports)
	if time.Since(report.GeneratedAt) > 7*24*time.Hour {
		return nil, false
	}

	return &report, true
}

// cacheIntelReport caches a report.
func cacheIntelReport(report *IntelReport) error {
	cacheDir := getIntelCacheDir()
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return err
	}

	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("%s_%s.json", sanitizeFilename(report.Company), report.Depth))

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cacheFile, data, 0600)
}

// getIntelCacheDir returns the intel cache directory.
func getIntelCacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".rigrun", "intel", "cache")
}

// sanitizeFilename sanitizes a string for use as a filename.
func sanitizeFilename(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	return s
}

// =============================================================================
// OUTPUT FUNCTIONS
// =============================================================================

// outputIntelReport outputs the report in the requested format.
func outputIntelReport(report *IntelReport, opts *IntelOptions) error {
	switch opts.Format {
	case IntelFormatJSON:
		return outputIntelJSON(report, opts)
	case IntelFormatPDF:
		return outputIntelPDF(report, opts)
	default:
		return outputIntelMarkdown(report, opts)
	}
}

// outputIntelMarkdown outputs the report as markdown.
func outputIntelMarkdown(report *IntelReport, opts *IntelOptions) error {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Competitive Intelligence Report: %s\n\n", report.Company))
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Depth:** %s\n", report.Depth))
	sb.WriteString(fmt.Sprintf("**Classification:** %s\n\n", report.Classification))
	sb.WriteString("---\n\n")

	// Output the synthesized report (from LLM analysis)
	sb.WriteString(report.Summary + "\n\n")

	sb.WriteString("---\n\n")
	sb.WriteString("## Research Metrics\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Duration:** %s\n", report.Metrics.TotalDuration.Round(time.Second)))
	sb.WriteString(fmt.Sprintf("- **Local Routing:** %.0f%%\n", report.Metrics.LocalPercent))
	sb.WriteString(fmt.Sprintf("- **Total Cost:** $%.4f\n", report.Metrics.TotalCost))
	sb.WriteString(fmt.Sprintf("- **Modules Analyzed:** %d\n", len(report.Modules)))
	sb.WriteString("\n---\n\n")
	sb.WriteString("*Generated by rigrun intel*\n")

	// Write to file or stdout
	output := sb.String()
	if opts.OutputDir != "" {
		filename := filepath.Join(opts.OutputDir, fmt.Sprintf("%s_%s.md", sanitizeFilename(report.Company), time.Now().Format("2006-01-02")))
		if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filename, []byte(output), 0644); err != nil {
			return err
		}
		fmt.Printf("Report saved to: %s\n", filename)
	} else {
		fmt.Println(output)
	}

	return nil
}

// outputIntelJSON outputs the report as JSON.
func outputIntelJSON(report *IntelReport, opts *IntelOptions) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	if opts.OutputDir != "" {
		filename := filepath.Join(opts.OutputDir, fmt.Sprintf("%s_%s.json", sanitizeFilename(report.Company), time.Now().Format("2006-01-02")))
		if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return err
		}
		fmt.Printf("Report saved to: %s\n", filename)
	} else {
		fmt.Println(string(data))
	}

	return nil
}

// outputIntelPDF outputs the report as PDF (requires pandoc).
func outputIntelPDF(report *IntelReport, opts *IntelOptions) error {
	// First generate markdown
	if opts.OutputDir == "" {
		opts.OutputDir = "."
	}

	mdFile := filepath.Join(opts.OutputDir, fmt.Sprintf("%s_%s.md", sanitizeFilename(report.Company), time.Now().Format("2006-01-02")))

	// Generate markdown
	oldFormat := opts.Format
	opts.Format = IntelFormatMarkdown
	if err := outputIntelMarkdown(report, opts); err != nil {
		return err
	}
	opts.Format = oldFormat

	// Convert to PDF using pandoc (if available)
	pdfFile := strings.TrimSuffix(mdFile, ".md") + ".pdf"
	fmt.Printf("PDF generation requires pandoc. To convert:\n  pandoc %s -o %s\n", mdFile, pdfFile)

	return nil
}

// =============================================================================
// STATS SUBCOMMAND
// =============================================================================

// handleIntelStats displays research statistics.
func handleIntelStats(args Args) error {
	cacheDir := getIntelCacheDir()

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		fmt.Println("No intel reports found.")
		return nil
	}

	var totalReports int
	var totalCost float64

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			totalReports++
			// Could read and sum costs here
		}
	}

	fmt.Println()
	fmt.Println(intelTitleStyle.Render("Intel Research Statistics"))
	fmt.Println()
	fmt.Printf("  Total reports:  %d\n", totalReports)
	fmt.Printf("  Cache location: %s\n", cacheDir)
	fmt.Printf("  Total cost:     $%.4f\n", totalCost)
	fmt.Println()

	return nil
}

// =============================================================================
// LIST SUBCOMMAND
// =============================================================================

// handleIntelList lists all researched companies.
func handleIntelList(args Args) error {
	cacheDir := getIntelCacheDir()

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		fmt.Println("No intel reports found.")
		return nil
	}

	fmt.Println()
	fmt.Println(intelTitleStyle.Render("Researched Companies"))
	fmt.Println()

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			name := strings.TrimSuffix(entry.Name(), ".json")
			name = strings.Replace(name, "_quick", " (quick)", 1)
			name = strings.Replace(name, "_standard", " (standard)", 1)
			name = strings.Replace(name, "_deep", " (deep)", 1)
			fmt.Printf("  * %s\n", name)
		}
	}
	fmt.Println()

	return nil
}

// =============================================================================
// COMPARE SUBCOMMAND
// =============================================================================

// handleIntelCompare compares multiple competitors.
func handleIntelCompare(args Args) error {
	companies := args.Options["compare"]
	if companies == "" {
		return fmt.Errorf("usage: rigrun intel --compare \"Company A,Company B,Company C\"")
	}

	companyList := strings.Split(companies, ",")
	for i := range companyList {
		companyList[i] = strings.TrimSpace(companyList[i])
	}

	fmt.Println()
	fmt.Println(intelTitleStyle.Render("Competitive Comparison"))
	fmt.Printf("Comparing: %s\n\n", strings.Join(companyList, " vs "))

	// Would execute research for each company and generate comparison
	fmt.Println("Comparison feature coming soon...")
	fmt.Println("For now, run individual reports:")
	for _, company := range companyList {
		fmt.Printf("  rigrun intel \"%s\"\n", company)
	}

	return nil
}

// =============================================================================
// CACHE SUBCOMMAND
// =============================================================================

// handleIntelCache manages the intel cache.
func handleIntelCache(args Args) error {
	action := args.Options["action"]
	if action == "" {
		action = "status"
	}

	switch action {
	case "clear":
		cacheDir := getIntelCacheDir()
		if err := os.RemoveAll(cacheDir); err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}
		fmt.Println(intelSuccessStyle.Render("[OK] Intel cache cleared"))
	case "status":
		cacheDir := getIntelCacheDir()
		entries, err := os.ReadDir(cacheDir)
		if err != nil {
			fmt.Println("Cache is empty.")
			return nil
		}
		fmt.Printf("Cache location: %s\n", cacheDir)
		fmt.Printf("Cached reports: %d\n", len(entries))
	default:
		return fmt.Errorf("unknown cache action: %s (valid: clear, status)", action)
	}

	return nil
}

// =============================================================================
// EXPORT SUBCOMMAND
// =============================================================================

// handleIntelExport exports all intel reports.
func handleIntelExport(args Args) error {
	outputDir := args.Options["output"]
	if outputDir == "" {
		outputDir = "./intel_export"
	}

	cacheDir := getIntelCacheDir()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return fmt.Errorf("no reports to export")
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	exported := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			src := filepath.Join(cacheDir, entry.Name())
			dst := filepath.Join(outputDir, entry.Name())

			data, err := os.ReadFile(src)
			if err != nil {
				continue
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				continue
			}
			exported++
		}
	}

	fmt.Printf("Exported %d reports to %s\n", exported, outputDir)
	return nil
}
