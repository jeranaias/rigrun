// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package examples demonstrates the intel system orchestrator
// This shows how all modules are coordinated to produce a complete report
package examples

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/cache"
	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/tools"
)

// =============================================================================
// INTEL ORCHESTRATOR
// =============================================================================
//
// The orchestrator coordinates all research modules to produce a complete
// competitive intelligence report. It manages:
//   - Multi-step agent workflow (20+ iterations)
//   - Module execution in phases (Discovery → Research → Analysis → Report)
//   - Routing decisions (local vs cloud based on classification and complexity)
//   - Caching at report, module, and query levels
//   - Audit logging for compliance
//   - Cost tracking and optimization
// =============================================================================

// Orchestrator coordinates the entire intel research process.
type Orchestrator struct {
	// Core dependencies
	config     *config.Config
	executor   *tools.Executor
	registry   *tools.Registry
	cache      *cache.CacheManager
	router     *router.Router
	ollamaClient *ollama.Client
	auditLog   *security.AuditLogger

	// Research modules
	modules map[string]ResearchModule

	// Session tracking
	sessionID     string
	startTime     time.Time
	totalTokens   int
	totalCost     float64
	queriesExecuted int
}

// ResearchModule is the interface all intel modules implement.
type ResearchModule interface {
	// Name returns the module name
	Name() string

	// Research executes the module's research
	Research(ctx context.Context, company string, classification security.ClassificationLevel) (interface{}, error)

	// CacheTTL returns how long results should be cached
	CacheTTL() time.Duration
}

// NewOrchestrator creates a new intel orchestrator.
func NewOrchestrator(cfg *config.Config) (*Orchestrator, error) {
	// Initialize core components
	registry := tools.NewRegistry()
	executor := tools.NewExecutor(registry, cfg)
	cacheManager := cache.NewCacheManager()
	routerInstance := router.NewRouter(cfg)
	ollamaClient := ollama.NewClient(ollama.DefaultConfig())
	auditLog := security.NewAuditLogger(cfg)

	o := &Orchestrator{
		config:       cfg,
		executor:     executor,
		registry:     registry,
		cache:        cacheManager,
		router:       routerInstance,
		ollamaClient: ollamaClient,
		auditLog:     auditLog,
		modules:      make(map[string]ResearchModule),
		sessionID:    generateSessionID(),
		startTime:    time.Now(),
	}

	// Register all research modules
	o.registerModules()

	return o, nil
}

// registerModules registers all built-in research modules.
func (o *Orchestrator) registerModules() {
	// In full implementation, would register all 9 modules:
	// o.modules["overview"] = NewCompanyOverviewModule(...)
	// o.modules["funding"] = NewFundingModule(...)
	// o.modules["leadership"] = NewLeadershipModule(...)
	// o.modules["product"] = NewProductModule(...)
	// o.modules["tech_stack"] = NewTechStackModule(...)
	// o.modules["news"] = NewNewsModule(...)
	// o.modules["market"] = NewMarketPositionModule(...)
	// o.modules["competitive_analysis"] = NewCompetitiveAnalysisModule(...)
	// o.modules["recommendations"] = NewRecommendationsModule(...)
}

// ResearchOptions configures the research process.
type ResearchOptions struct {
	// Depth controls how thorough the research is
	Depth ResearchDepth

	// Classification controls routing and security
	Classification security.ClassificationLevel

	// MaxIterations limits the agent loop
	MaxIterations int

	// OutputFormat specifies the report format
	OutputFormat string

	// OutputPath specifies where to save the report
	OutputPath string

	// UseCache controls whether to use cached data
	UseCache bool

	// ForceUpdate forces re-research even if cached
	ForceUpdate bool
}

// ResearchDepth specifies how thorough the research should be.
type ResearchDepth int

const (
	// DepthQuick runs 3 modules (Overview, Funding, News)
	DepthQuick ResearchDepth = iota

	// DepthStandard runs all 9 modules
	DepthStandard

	// DepthDeep runs all 9 modules with extended analysis
	DepthDeep
)

// ResearchReport holds the complete research results.
type ResearchReport struct {
	Company        string
	GeneratedAt    time.Time
	Classification security.ClassificationLevel
	Depth          ResearchDepth

	// Module results
	Overview           *CompanyOverviewResult
	Funding            interface{} // Would be *FundingResult in full implementation
	Leadership         interface{}
	Product            interface{}
	TechStack          interface{}
	News               interface{}
	MarketPosition     interface{}
	CompetitiveAnalysis interface{}
	Recommendations    interface{}

	// Metadata
	ResearchTime     time.Duration
	TotalTokens      int
	TotalCost        float64
	QueriesExecuted  int
	CacheHitRate     float64
	LocalCloudSplit  map[string]float64
	Sources          []string
	AuditTrailPath   string
}

// Research conducts complete competitive intelligence research on a company.
func (o *Orchestrator) Research(ctx context.Context, companyName string, opts ResearchOptions) (*ResearchReport, error) {
	fmt.Printf("Researching: %s\n", companyName)
	fmt.Printf("Depth: %s\n", opts.Depth)
	fmt.Printf("Classification: %s\n", opts.Classification)
	fmt.Println()

	// Step 1: Check for complete cached report
	if opts.UseCache && !opts.ForceUpdate {
		if cached := o.checkReportCache(ctx, companyName); cached != nil {
			fmt.Println("Cache hit! Using existing report.")
			return cached, nil
		}
	}

	// Step 2: Initialize report
	report := &ResearchReport{
		Company:        companyName,
		GeneratedAt:    time.Now(),
		Classification: opts.Classification,
		Depth:          opts.Depth,
		LocalCloudSplit: make(map[string]float64),
		Sources:        make([]string, 0),
	}

	// Step 3: Execute research phases
	if err := o.executePhases(ctx, companyName, opts, report); err != nil {
		return nil, err
	}

	// Step 4: Generate report file
	if err := o.generateReportFile(ctx, report, opts); err != nil {
		return nil, err
	}

	// Step 5: Cache complete report
	if opts.UseCache {
		o.cacheReport(ctx, companyName, report)
	}

	// Step 6: Update stats
	report.ResearchTime = time.Since(o.startTime)
	report.TotalTokens = o.totalTokens
	report.TotalCost = o.totalCost
	report.QueriesExecuted = o.queriesExecuted

	// Step 7: Print summary
	o.printSummary(report)

	return report, nil
}

// executePhases runs all research phases.
func (o *Orchestrator) executePhases(ctx context.Context, companyName string, opts ResearchOptions, report *ResearchReport) error {
	// PHASE 1: DISCOVERY (Local Routing)
	fmt.Println("Phase 1: Discovery")
	if err := o.phaseDiscovery(ctx, companyName, opts, report); err != nil {
		return fmt.Errorf("discovery phase failed: %w", err)
	}

	// PHASE 2: DEEP RESEARCH (Mixed Routing)
	fmt.Println("\nPhase 2: Deep Research")
	if err := o.phaseDeepResearch(ctx, companyName, opts, report); err != nil {
		return fmt.Errorf("deep research phase failed: %w", err)
	}

	// PHASE 3: ANALYSIS (Cloud Routing, unless CUI)
	if opts.Depth >= DepthStandard {
		fmt.Println("\nPhase 3: Analysis")
		if err := o.phaseAnalysis(ctx, companyName, opts, report); err != nil {
			return fmt.Errorf("analysis phase failed: %w", err)
		}
	}

	return nil
}

// phaseDiscovery runs the discovery phase (modules 1-3).
func (o *Orchestrator) phaseDiscovery(ctx context.Context, companyName string, opts ResearchOptions, report *ResearchReport) error {
	// Module 1: Company Overview
	fmt.Print("[1/9] Company Overview...          ")
	start := time.Now()

	overviewModule := o.modules["overview"]
	result, err := overviewModule.Research(ctx, companyName, opts.Classification)
	if err != nil {
		return err
	}

	report.Overview = result.(*CompanyOverviewResult)
	duration := time.Since(start)
	cost := report.Overview.Cost

	fmt.Printf("✓ (local, %ds, $%.2f)\n", int(duration.Seconds()), cost)

	o.totalTokens += report.Overview.TokensUsed
	o.totalCost += cost
	o.queriesExecuted++

	// Module 2 & 3 would follow same pattern...

	return nil
}

// phaseDeepResearch runs the deep research phase (modules 4-7).
func (o *Orchestrator) phaseDeepResearch(ctx context.Context, companyName string, opts ResearchOptions, report *ResearchReport) error {
	// Module 4: Product Portfolio
	fmt.Print("[4/9] Product Portfolio...         ")
	// ... implementation similar to phaseDiscovery

	return nil
}

// phaseAnalysis runs the analysis phase (modules 8-9).
func (o *Orchestrator) phaseAnalysis(ctx context.Context, companyName string, opts ResearchOptions, report *ResearchReport) error {
	// Module 8: Competitive Analysis
	fmt.Print("[8/9] Competitive Analysis...      ")

	// This module uses cloud routing (Sonnet) for complex strategic analysis
	// Unless classification is CUI, then forced to local

	// Prompt for analysis
	prompt := o.buildCompetitiveAnalysisPrompt(report)

	// Determine routing tier
	tier := router.TierSonnet
	if opts.Classification >= security.ClassificationCUI {
		// CUI classification forces local
		tier = router.TierLocal
		fmt.Print("(local: qwen2.5:32b, ")
	} else {
		fmt.Print("(cloud: Sonnet, ")
	}

	start := time.Now()

	// Execute LLM call
	// In full implementation would call ollama.Chat() or cloud.Chat()
	_ = prompt // Use prompt
	duration := time.Since(start)
	cost := 0.12 // Would calculate from actual tokens

	fmt.Printf("%ds, $%.2f)\n", int(duration.Seconds()), cost)

	o.totalCost += cost
	o.queriesExecuted++

	return nil
}

// buildCompetitiveAnalysisPrompt creates the analysis prompt.
func (o *Orchestrator) buildCompetitiveAnalysisPrompt(report *ResearchReport) string {
	var promptBuilder strings.Builder

	promptBuilder.WriteString("Perform a comprehensive competitive analysis based on the following research:\n\n")

	promptBuilder.WriteString(fmt.Sprintf("=== COMPANY OVERVIEW ===\n"))
	if report.Overview != nil {
		promptBuilder.WriteString(fmt.Sprintf("Name: %s\n", report.Overview.Name))
		promptBuilder.WriteString(fmt.Sprintf("Founded: %d by %s\n", report.Overview.Founded, strings.Join(report.Overview.Founders, ", ")))
		promptBuilder.WriteString(fmt.Sprintf("Mission: %s\n", report.Overview.Mission))
		promptBuilder.WriteString(fmt.Sprintf("Employees: %d\n", report.Overview.Employees))
		promptBuilder.WriteString(fmt.Sprintf("HQ: %s\n\n", report.Overview.Headquarters))
	}

	// In full implementation, would include all module data

	promptBuilder.WriteString(`
Please provide a SWOT analysis with the following structure:

STRENGTHS:
- [List 3-5 key strengths]

WEAKNESSES:
- [List 3-5 key weaknesses]

OPPORTUNITIES:
- [List 3-5 opportunities]

THREATS:
- [List 3-5 threats]

COMPETITIVE POSITIONING:
[2-3 paragraphs analyzing their market position compared to rigrun]

Return ONLY the analysis, no preamble.`)

	return promptBuilder.String()
}

// generateReportFile creates the markdown/JSON/PDF report file.
func (o *Orchestrator) generateReportFile(ctx context.Context, report *ResearchReport, opts ResearchOptions) error {
	// Use Write tool to generate report
	reportPath := o.getReportPath(report.Company, opts.OutputFormat)

	var content string
	switch opts.OutputFormat {
	case "json":
		content = o.renderJSON(report)
	case "pdf":
		// First generate markdown, then convert to PDF using pandoc
		mdContent := o.renderMarkdown(report)
		// Use Write tool for markdown
		o.writeFile(ctx, reportPath+".md", mdContent)
		// Use Bash tool to convert to PDF
		o.convertToPDF(ctx, reportPath+".md", reportPath)
		return nil
	default: // markdown
		content = o.renderMarkdown(report)
	}

	// Write report using Write tool
	return o.writeFile(ctx, reportPath, content)
}

// renderMarkdown generates the markdown report.
func (o *Orchestrator) renderMarkdown(report *ResearchReport) string {
	var md strings.Builder

	md.WriteString(fmt.Sprintf("# Competitive Intelligence Report: %s\n\n", report.Company))
	md.WriteString(fmt.Sprintf("**Generated**: %s\n", report.GeneratedAt.Format(time.RFC3339)))
	md.WriteString(fmt.Sprintf("**Classification**: %s\n", report.Classification))
	md.WriteString(fmt.Sprintf("**Depth**: %s\n\n", report.Depth))

	md.WriteString("---\n\n")

	md.WriteString("## 1. Company Overview\n\n")
	if report.Overview != nil {
		md.WriteString(fmt.Sprintf("**Founded**: %d by %s\n", report.Overview.Founded, strings.Join(report.Overview.Founders, ", ")))
		md.WriteString(fmt.Sprintf("**Headquarters**: %s\n", report.Overview.Headquarters))
		md.WriteString(fmt.Sprintf("**Employees**: %d\n", report.Overview.Employees))
		md.WriteString(fmt.Sprintf("**Mission**: %s\n\n", report.Overview.Mission))
		md.WriteString(fmt.Sprintf("%s\n\n", report.Overview.Description))
	}

	// Would continue with all other sections...

	md.WriteString("---\n\n")
	md.WriteString("## Appendix\n\n")
	md.WriteString("### Research Metadata\n\n")
	md.WriteString(fmt.Sprintf("- **Total Research Time**: %v\n", report.ResearchTime))
	md.WriteString(fmt.Sprintf("- **Queries Executed**: %d\n", report.QueriesExecuted))
	md.WriteString(fmt.Sprintf("- **Tokens Consumed**: %d\n", report.TotalTokens))
	md.WriteString(fmt.Sprintf("- **Estimated Cost**: $%.4f\n", report.TotalCost))

	return md.String()
}

// renderJSON generates the JSON export.
func (o *Orchestrator) renderJSON(report *ResearchReport) string {
	// Would use json.Marshal in full implementation
	return `{"company": "..."}`
}

// writeFile uses the Write tool to write a file.
func (o *Orchestrator) writeFile(ctx context.Context, path string, content string) error {
	toolCall := &tools.ToolCall{
		Name: "Write",
		Params: map[string]interface{}{
			"file_path": path,
			"content":   content,
		},
	}

	result, err := o.executor.Execute(ctx, o.registry.Get("Write"), toolCall)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("write failed: %s", result.Error)
	}

	return nil
}

// convertToPDF uses pandoc to convert markdown to PDF.
func (o *Orchestrator) convertToPDF(ctx context.Context, mdPath string, pdfPath string) error {
	toolCall := &tools.ToolCall{
		Name: "Bash",
		Params: map[string]interface{}{
			"command": fmt.Sprintf("pandoc %s -o %s", mdPath, pdfPath),
		},
	}

	result, err := o.executor.Execute(ctx, o.registry.Get("Bash"), toolCall)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("PDF conversion failed: %s", result.Error)
	}

	return nil
}

// getReportPath generates the report file path.
func (o *Orchestrator) getReportPath(companyName string, format string) string {
	sanitized := strings.ToLower(strings.ReplaceAll(companyName, " ", "-"))
	date := time.Now().Format("2006-01-02")
	extension := format
	if format == "markdown" {
		extension = "md"
	}
	return fmt.Sprintf("~/.rigrun/intel/reports/%s_%s.%s", sanitized, date, extension)
}

// checkReportCache checks if a complete report exists in cache.
func (o *Orchestrator) checkReportCache(ctx context.Context, companyName string) *ResearchReport {
	cacheKey := fmt.Sprintf("intel:report:%s", strings.ToLower(companyName))
	if cached, hit := o.cache.Lookup(ctx, cacheKey); hit {
		if report, ok := cached.(*ResearchReport); ok {
			return report
		}
	}
	return nil
}

// cacheReport stores the complete report in cache.
func (o *Orchestrator) cacheReport(ctx context.Context, companyName string, report *ResearchReport) {
	cacheKey := fmt.Sprintf("intel:report:%s", strings.ToLower(companyName))
	o.cache.Store(ctx, cacheKey, report, 0) // 0 = permanent (until user updates)
}

// printSummary prints the research summary.
func (o *Orchestrator) printSummary(report *ResearchReport) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Summary:")
	fmt.Printf("- Researched %d modules in %v\n", o.queriesExecuted, report.ResearchTime.Round(time.Second))
	fmt.Printf("- Tokens: %d\n", report.TotalTokens)
	fmt.Printf("- Cost: $%.4f\n", report.TotalCost)
	if report.CacheHitRate > 0 {
		fmt.Printf("- Cache hit rate: %.0f%%\n", report.CacheHitRate*100)
	}
	fmt.Println(strings.Repeat("=", 60))
}

// generateSessionID creates a unique session ID.
func generateSessionID() string {
	return fmt.Sprintf("intel-%d", time.Now().Unix())
}

// String returns string representation of ResearchDepth.
func (d ResearchDepth) String() string {
	switch d {
	case DepthQuick:
		return "Quick"
	case DepthStandard:
		return "Standard"
	case DepthDeep:
		return "Deep"
	default:
		return "Unknown"
	}
}

// =============================================================================
// USAGE EXAMPLE
// =============================================================================

/*
func main() {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create orchestrator
	orch, err := NewOrchestrator(cfg)
	if err != nil {
		log.Fatalf("Failed to create orchestrator: %v", err)
	}

	// Research a company
	report, err := orch.Research(context.Background(), "Anthropic", ResearchOptions{
		Depth:          DepthStandard,
		Classification: security.ClassificationUnclassified,
		MaxIterations:  20,
		OutputFormat:   "markdown",
		UseCache:       true,
		ForceUpdate:    false,
	})

	if err != nil {
		log.Fatalf("Research failed: %v", err)
	}

	fmt.Printf("\nReport generated: %s\n", report.Company)
}
*/
