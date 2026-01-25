# Competitive Intelligence System - Design Document

> **Demonstrates rigrun's full capabilities: WebSearch, WebFetch, Agent Loops, Cloud/Local Routing, Caching, All Tools, and CLI Integration**

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [System Architecture](#2-system-architecture)
3. [CLI Commands](#3-cli-commands)
4. [Agent Workflow](#4-agent-workflow)
5. [Research Modules](#5-research-modules)
6. [File Structure](#6-file-structure)
7. [Integration Points](#7-integration-points)
8. [Report Format](#8-report-format)
9. [Caching Strategy](#9-caching-strategy)
10. [Security & Compliance](#10-security--compliance)
11. [Implementation Plan](#11-implementation-plan)
12. [Example Usage](#12-example-usage)

---

## 1. Executive Summary

### What is the Competitive Intelligence System?

A fully autonomous research system that leverages **ALL** rigrun capabilities to gather, analyze, and synthesize comprehensive intelligence on competitor companies. It demonstrates rigrun's power as a complete agentic framework, not just an LLM router.

### Key Differentiators

This system showcases rigrun's unique combination of:

1. **Classification-Aware Research**: Automatically handles CUI-marked competitive data locally
2. **Intelligent Routing**: Complex analysis via cloud models, simple aggregation via local
3. **Autonomous Multi-Step Research**: Agent loops with 15+ research steps per company
4. **Web Integration**: WebSearch for discovery, WebFetch for deep scraping
5. **Full Tool Suite**: Read/Write reports, Grep/Glob local data, Bash for exports
6. **Semantic Caching**: Avoid re-researching companies or duplicate queries
7. **Audit Trail**: Complete provenance of all intelligence gathered (IL5 compliant)

### Business Value

- **Time Savings**: 30 minutes of manual research → 3 minutes autonomous execution
- **Consistency**: Every competitor analyzed with the same depth and methodology
- **Compliance**: All research logged, cached data encrypted, sensitive info redacted
- **Cost Optimization**: 85% of research done locally, only deep analysis routed to cloud

---

## 2. System Architecture

### High-Level Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    rigrun intel <company>                        │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            v
┌─────────────────────────────────────────────────────────────────┐
│                   Intel Agent Orchestrator                       │
│  - Parse target company                                          │
│  - Load intel template                                           │
│  - Initialize research session                                   │
│  - Check cache for existing reports                              │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            v
              ┌─────────────┴─────────────┐
              │                           │
              v                           v
┌──────────────────────┐    ┌──────────────────────┐
│  Discovery Phase     │    │  Deep Research Phase │
│  (Local Routing)     │    │  (Cloud Routing)     │
└──────────┬───────────┘    └──────────┬───────────┘
           │                           │
           │  ┌────────────────────────┤
           v  v                        v
    ┌────────────────┐      ┌────────────────┐
    │  WebSearch     │      │  WebFetch      │
    │  - News        │      │  - Company URL │
    │  - Funding     │      │  - Docs        │
    │  - Team        │      │  - Blog        │
    │  - Tech Stack  │      │  - Pricing     │
    └────────┬───────┘      └────────┬───────┘
             │                       │
             └───────────┬───────────┘
                         v
              ┌─────────────────────┐
              │  Analysis Phase     │
              │  (Cloud: Sonnet/Opus)│
              │  - Synthesis        │
              │  - Competitive Gaps │
              │  - Strategic Insights│
              └──────────┬──────────┘
                         │
                         v
              ┌─────────────────────┐
              │  Report Generation  │
              │  (Local: Formatting)│
              │  - Markdown Report  │
              │  - JSON Export      │
              │  - Cache Results    │
              │  - Audit Log        │
              └─────────────────────┘
```

### Three-Tier Routing Strategy

| Phase | Tier | Model | Rationale |
|-------|------|-------|-----------|
| **Discovery** | Cache → Local | qwen2.5-coder:14b | Simple aggregation, list generation |
| **Deep Research** | Local → Cloud | qwen2.5:32b / Sonnet | URL fetching, content extraction |
| **Analysis** | Cloud | Sonnet / Opus | Strategic synthesis, competitive analysis |
| **Report Generation** | Local | qwen2.5-coder:14b | Markdown formatting, templating |

**Result**: 85% of work done locally, cloud only for high-value strategic analysis.

---

## 3. CLI Commands

### New Commands

```bash
# Primary command: Full competitive intelligence report
rigrun intel <company>

# Alias for brevity
rigrun ci <company>

# Options
rigrun intel <company> --depth [quick|standard|deep]
rigrun intel <company> --format [markdown|json|pdf]
rigrun intel <company> --output <directory>
rigrun intel <company> --classification [UNCLASSIFIED|CUI]
rigrun intel <company> --use-cache              # Default: true
rigrun intel <company> --max-iter <n>           # Max research iterations (default: 20)
rigrun intel <company> --paranoid               # Local-only research (no cloud)

# Batch mode: Research multiple competitors
rigrun intel --batch competitors.txt

# Compare mode: Side-by-side analysis
rigrun intel --compare "Company A,Company B,Company C"

# Update mode: Refresh existing report
rigrun intel <company> --update

# Stats and management
rigrun intel stats                              # Show research statistics
rigrun intel list                               # List all researched companies
rigrun intel cache clear                        # Clear intelligence cache
rigrun intel export --all                       # Export all reports
```

### Examples

```bash
# Quick competitive scan
rigrun intel "Anthropic" --depth quick

# Deep research with CUI classification (stays local)
rigrun intel "OpenAI" --depth deep --classification CUI

# Generate PDF report in specific directory
rigrun intel "Mistral AI" --format pdf --output ./reports/

# Compare three competitors
rigrun intel --compare "OpenAI,Anthropic,Cohere"

# Batch research from file
cat competitors.txt
# OpenAI
# Anthropic
# Cohere
# Mistral AI
rigrun intel --batch competitors.txt

# Update existing report (smart: only fetches new data)
rigrun intel "Anthropic" --update
```

---

## 4. Agent Workflow

### Master Orchestration Loop

The intel agent uses rigrun's agentic capabilities (tools + multi-step reasoning) to autonomously research a company.

```
INITIALIZATION
├─ Parse company name
├─ Check cache for existing report
├─ Load intel template
├─ Initialize research session
└─ Create audit log entry

PHASE 1: DISCOVERY (Local Routing)
├─ Step 1: WebSearch "Company name overview"
├─ Step 2: WebSearch "Company name funding rounds"
├─ Step 3: WebSearch "Company name leadership team"
├─ Step 4: WebSearch "Company name technology stack"
├─ Step 5: WebSearch "Company name recent news"
└─ Cache all search results (semantic cache, 7 day TTL)

PHASE 2: DEEP RESEARCH (Mixed Routing)
├─ Step 6: WebFetch company website (homepage)
├─ Step 7: WebFetch /about page
├─ Step 8: WebFetch /pricing page
├─ Step 9: WebFetch /blog (recent posts)
├─ Step 10: WebFetch /docs (if API product)
├─ Step 11: Bash "gh repo view <company>/<repo>" (GitHub stats)
├─ Step 12: Bash "curl -s https://api.github.com/repos/<company>/<repo>"
└─ Cache fetched content (semantic cache, 24 hour TTL)

PHASE 3: ANALYSIS (Cloud Routing: Sonnet/Opus)
├─ Step 13: Synthesize all gathered data
│   ├─ Prompt: "Analyze competitive positioning..."
│   ├─ Route: TierSonnet (complex analysis)
│   └─ Context: All cached research from Phase 1 & 2
├─ Step 14: Identify competitive gaps
│   ├─ Prompt: "What are this company's weaknesses vs rigrun?"
│   ├─ Route: TierSonnet
│   └─ Context: Product features, pricing, tech stack
├─ Step 15: Strategic recommendations
│   ├─ Prompt: "Recommend strategic responses..."
│   ├─ Route: TierOpus (if available, else Sonnet)
│   └─ Context: Full competitive analysis
└─ Cache analysis results (7 day TTL)

PHASE 4: REPORT GENERATION (Local Routing)
├─ Step 16: Format markdown report
│   ├─ Use Write tool to create report
│   ├─ Route: TierLocal (simple templating)
│   └─ Template: intel_report_template.md
├─ Step 17: Generate JSON export (if requested)
│   ├─ Use Write tool for JSON
│   └─ Route: TierLocal
├─ Step 18: Export to PDF (if requested)
│   ├─ Use Bash tool: "pandoc report.md -o report.pdf"
│   └─ Route: TierLocal
└─ Step 19: Log to audit trail
    ├─ Record all URLs accessed
    ├─ Record routing decisions
    └─ Record total cost and tokens

FINALIZATION
├─ Cache complete report (semantic cache)
├─ Update stats.json
├─ Print summary to user
└─ Return path to report
```

### Tool Usage by Phase

| Phase | Tools Used | Routing Tier | Caching |
|-------|-----------|--------------|---------|
| Discovery | WebSearch (5x) | Local | Semantic, 7 days |
| Deep Research | WebFetch (6x), Bash (2x) | Local → Cloud | Semantic, 24 hours |
| Analysis | None (pure LLM) | Cloud (Sonnet/Opus) | Semantic, 7 days |
| Report Gen | Write (2x), Bash (1x) | Local | Exact, permanent |

### Iteration Control

- **Max iterations**: 20 (configurable via `--max-iter`)
- **Timeout**: 10 minutes total (configurable)
- **Early exit**: If confidence score > 0.95 after analysis phase
- **Fallback**: If cloud unavailable, use local 32B model for analysis

---

## 5. Research Modules

Each research module is a self-contained component that uses specific tools and routing strategies.

### Module 1: Company Overview

**Objective**: Basic facts, founding story, mission, employee count

**Tools**: WebSearch, WebFetch
**Routing**: Local (qwen2.5:14b)
**Queries**:
- WebSearch: `"Company name" overview history founded`
- WebFetch: `https://company.com/about`
- WebFetch: `https://www.crunchbase.com/organization/company-name`

**Output**: Company name, founded year, founders, mission, employee count, HQ location

**Cache TTL**: 30 days (basic facts change rarely)

---

### Module 2: Funding & Financials

**Objective**: Total funding, valuation, investors, revenue estimates

**Tools**: WebSearch, WebFetch
**Routing**: Local (qwen2.5:14b)
**Queries**:
- WebSearch: `"Company name" funding rounds Series A B C`
- WebSearch: `"Company name" valuation 2025`
- WebFetch: `https://www.crunchbase.com/organization/company-name/funding_rounds`
- WebFetch: Company press releases (if available)

**Output**: Total funding, latest round, valuation, key investors, revenue (if public)

**Cache TTL**: 7 days (funding changes frequently)

---

### Module 3: Leadership Team

**Objective**: C-suite executives, advisors, board members

**Tools**: WebSearch, WebFetch
**Routing**: Local (qwen2.5:14b)
**Queries**:
- WebSearch: `"Company name" CEO founder leadership team`
- WebFetch: `https://company.com/about/team`
- WebFetch: LinkedIn profiles (if publicly accessible)

**Output**: CEO, CTO, other executives, notable advisors

**Cache TTL**: 30 days (leadership changes infrequently)

---

### Module 4: Product Portfolio

**Objective**: Core products, features, pricing, target customers

**Tools**: WebFetch, Bash (for API docs)
**Routing**: Local → Cloud (for feature comparison)
**Queries**:
- WebFetch: `https://company.com/products`
- WebFetch: `https://company.com/pricing`
- WebFetch: `https://company.com/docs` (if API product)
- Bash: `curl -s https://api.company.com/docs` (for API schemas)

**Analysis** (Cloud: Sonnet):
- Prompt: "Compare this product's features to rigrun. What do they have that we don't?"

**Output**: Product list, key features, pricing tiers, API capabilities (if applicable)

**Cache TTL**: 7 days (products and pricing change frequently)

---

### Module 5: Technology Stack

**Objective**: Languages, frameworks, infrastructure, dependencies

**Tools**: WebSearch, Bash (GitHub API)
**Routing**: Local (qwen2.5:14b)
**Queries**:
- WebSearch: `"Company name" technology stack engineering blog`
- Bash: `gh repo view company/repo` (if open source)
- Bash: `curl -s https://api.github.com/repos/company/repo/languages`
- WebFetch: Engineering blog posts

**Output**: Programming languages, frameworks, cloud providers, key dependencies

**Cache TTL**: 14 days (tech stack relatively stable)

---

### Module 6: Recent News & Announcements

**Objective**: Latest news, partnerships, product launches

**Tools**: WebSearch, WebFetch
**Routing**: Local (qwen2.5:14b)
**Queries**:
- WebSearch: `"Company name" news 2025`
- WebSearch: `"Company name" partnership announcement`
- WebFetch: `https://company.com/blog` (recent posts)
- WebFetch: `https://company.com/press`

**Output**: Recent news articles, product launches, partnerships (last 90 days)

**Cache TTL**: 1 day (news changes daily)

---

### Module 7: Market Position & Customers

**Objective**: Target market, customer base, case studies, market share

**Tools**: WebSearch, WebFetch
**Routing**: Local → Cloud (for market analysis)
**Queries**:
- WebSearch: `"Company name" customers case studies`
- WebSearch: `"Company name" market share competitors`
- WebFetch: `https://company.com/customers`
- WebFetch: `https://company.com/case-studies`

**Analysis** (Cloud: Sonnet):
- Prompt: "Who are their primary customers? How does their customer base compare to rigrun's target audience?"

**Output**: Target market, notable customers, case studies, estimated market share

**Cache TTL**: 14 days

---

### Module 8: Competitive Analysis

**Objective**: SWOT analysis, competitive positioning, gaps vs rigrun

**Tools**: None (pure LLM analysis)
**Routing**: Cloud (Sonnet/Opus)
**Context**: All data from Modules 1-7

**Prompts**:
1. "Perform a SWOT analysis for this company based on the research gathered."
2. "Compare this company to rigrun. Where do they excel? Where do we excel?"
3. "What competitive gaps exist? What features should rigrun prioritize?"
4. "What are potential partnership or acquisition opportunities?"

**Output**: SWOT matrix, competitive positioning, strategic recommendations

**Cache TTL**: 7 days

---

### Module 9: Strategic Recommendations

**Objective**: Actionable insights for rigrun's product/marketing teams

**Tools**: None (pure LLM synthesis)
**Routing**: Cloud (Opus preferred, Sonnet fallback)
**Context**: Full competitive analysis from Module 8

**Prompts**:
- "Based on this competitive analysis, recommend 3 strategic actions for rigrun."
- "What messaging angles should rigrun emphasize against this competitor?"
- "Are there any product features we should fast-track?"

**Output**: 3-5 strategic recommendations with rationale

**Cache TTL**: 7 days

---

## 6. File Structure

### New Files and Packages

```
rigrun/
├── src/intel/                          # Rust implementation
│   ├── mod.rs                          # Module root
│   ├── orchestrator.rs                 # Main agent orchestrator
│   ├── modules/                        # Research modules
│   │   ├── mod.rs
│   │   ├── company_overview.rs
│   │   ├── funding.rs
│   │   ├── leadership.rs
│   │   ├── product.rs
│   │   ├── tech_stack.rs
│   │   ├── news.rs
│   │   ├── market_position.rs
│   │   ├── competitive_analysis.rs
│   │   └── recommendations.rs
│   ├── report.rs                       # Report generation
│   ├── cache.rs                        # Intel-specific caching
│   ├── stats.rs                        # Research statistics
│   └── templates/                      # Report templates
│       ├── report_template.md
│       ├── comparison_template.md
│       └── batch_summary_template.md
│
├── go-tui/internal/intel/              # Go implementation
│   ├── orchestrator.go                 # Main agent orchestrator
│   ├── modules/                        # Research modules
│   │   ├── company_overview.go
│   │   ├── funding.go
│   │   ├── leadership.go
│   │   ├── product.go
│   │   ├── tech_stack.go
│   │   ├── news.go
│   │   ├── market_position.go
│   │   ├── competitive_analysis.go
│   │   └── recommendations.go
│   ├── report.go                       # Report generation
│   ├── cache.go                        # Intel-specific caching
│   ├── stats.go                        # Research statistics
│   ├── types.go                        # Data structures
│   └── templates/                      # Report templates
│       ├── report.tmpl
│       ├── comparison.tmpl
│       └── batch_summary.tmpl
│
├── docs/
│   ├── COMPETITIVE_INTEL_SYSTEM_DESIGN.md  # This document
│   └── INTEL_USAGE_GUIDE.md                # User guide
│
└── examples/intel/
    ├── anthropic_report.md             # Example output
    ├── openai_report.md
    └── comparison_report.md
```

### Data Storage

```
~/.rigrun/
├── intel/
│   ├── reports/                        # Generated reports
│   │   ├── anthropic_2025-01-23.md
│   │   ├── openai_2025-01-23.md
│   │   └── mistral_2025-01-23.md
│   ├── cache/                          # Cached research data
│   │   ├── anthropic.json
│   │   └── openai.json
│   ├── stats.json                      # Research statistics
│   └── audit.log                       # Audit trail (IL5)
```

---

## 7. Integration Points

### 7.1 WebSearch Integration

**Purpose**: Discovery phase - find news, funding info, team members

**Implementation** (Go):
```go
// go-tui/internal/tools/search.go
func (e *WebSearchExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
    query := params["query"].(string)

    // Use DuckDuckGo search (no API key required)
    results, err := e.searchDuckDuckGo(ctx, query)
    if err != nil {
        return Result{Success: false, Error: err.Error()}, err
    }

    // Format results as markdown
    output := formatSearchResults(results)

    return Result{
        Success: true,
        Output: output,
        MatchCount: len(results),
    }, nil
}
```

**Usage in Intel Agent**:
```go
// Phase 1: Discovery
toolCall := &tools.ToolCall{
    Name: "WebSearch",
    Params: map[string]interface{}{
        "query": fmt.Sprintf("%s funding rounds 2025", companyName),
    },
}

result, err := executor.Execute(ctx, registry.Get("WebSearch"), toolCall)
// Cache result with 7-day TTL
intelCache.Store(ctx, cacheKey, result, 7*24*time.Hour)
```

---

### 7.2 WebFetch Integration

**Purpose**: Deep research - scrape company websites, blogs, docs

**Implementation** (Go):
```go
// go-tui/internal/tools/web.go
func (e *WebFetchExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
    url := params["url"].(string)

    // Fetch page content
    content, err := e.fetchPage(ctx, url)
    if err != nil {
        return Result{Success: false, Error: err.Error()}, err
    }

    // Convert HTML to clean markdown
    markdown := htmlToMarkdown(content)

    return Result{
        Success: true,
        Output: markdown,
        BytesRead: int64(len(content)),
    }, nil
}
```

**Usage in Intel Agent**:
```go
// Phase 2: Deep Research
toolCall := &tools.ToolCall{
    Name: "WebFetch",
    Params: map[string]interface{}{
        "url": "https://www.anthropic.com/about",
    },
}

result, err := executor.Execute(ctx, registry.Get("WebFetch"), toolCall)
// Cache with 24-hour TTL
intelCache.Store(ctx, cacheKey, result, 24*time.Hour)
```

---

### 7.3 Router Integration

**Purpose**: Route research tasks based on complexity

**Strategy**:
- **Discovery queries** → Local (qwen2.5:14b) - simple aggregation
- **Deep research** → Local → Cloud - content extraction, may escalate
- **Analysis** → Cloud (Sonnet/Opus) - strategic synthesis
- **Report generation** → Local - simple templating

**Implementation** (Go):
```go
// Phase determination logic
func (o *Orchestrator) routeResearchPhase(phase ResearchPhase, query string) router.Tier {
    switch phase {
    case PhaseDiscovery:
        // Simple queries, use local
        return router.TierLocal

    case PhaseDeepResearch:
        // Content extraction, try local first
        // Router will auto-escalate to cloud if needed
        return router.RouteQuery(query, security.ClassificationUnclassified, false, &router.TierSonnet)

    case PhaseAnalysis:
        // Complex strategic analysis, use cloud
        return router.TierSonnet

    case PhaseReportGen:
        // Simple templating, use local
        return router.TierLocal

    default:
        return router.TierLocal
    }
}
```

---

### 7.4 Caching Integration

**Purpose**: Avoid re-researching companies, cache expensive API calls

**Strategy**:
- **Exact cache**: Company name → Report path (instant retrieval)
- **Semantic cache**: Similar queries about same company → Reuse results
- **Module-level cache**: Each research module caches its results independently

**Cache TTLs**:
| Data Type | TTL | Rationale |
|-----------|-----|-----------|
| Complete report | Permanent | User can manually refresh with `--update` |
| Company overview | 30 days | Basic facts change rarely |
| Funding data | 7 days | Funding rounds happen frequently |
| News | 1 day | News is time-sensitive |
| Product features | 7 days | Features added frequently |
| Tech stack | 14 days | Tech stack relatively stable |
| Analysis | 7 days | Market conditions change weekly |

**Implementation** (Go):
```go
// Check cache before research
cachedReport, hit := intelCache.Lookup(ctx, companyName)
if hit {
    log.Printf("Cache hit for %s, returning cached report", companyName)
    return cachedReport, nil
}

// After research, cache results
intelCache.Store(ctx, companyName, report, 0) // 0 = permanent
```

---

### 7.5 Audit Logging Integration

**Purpose**: Complete audit trail of all intelligence gathered (IL5 compliance)

**Logged Information**:
- All WebSearch queries and results
- All WebFetch URLs accessed
- All routing decisions (local vs cloud)
- All tool executions (Bash commands, file writes)
- Total tokens consumed and cost
- Research start/end timestamps
- User who initiated research
- Classification level applied

**Implementation** (Go):
```go
// Log each research step
auditLogger.Log(&security.AuditEvent{
    Timestamp: time.Now(),
    EventType: security.EventTypeIntelResearch,
    User:      getUserFromSession(),
    Classification: security.ClassificationUnclassified, // or CUI
    Details: map[string]interface{}{
        "company": companyName,
        "phase": "discovery",
        "tool": "WebSearch",
        "query": query,
        "results_count": len(results),
        "routing_tier": tier.String(),
        "tokens_used": tokensUsed,
        "cost_usd": costUSD,
    },
})
```

**Audit Log Format** (`~/.rigrun/intel/audit.log`):
```
2025-01-23T10:23:45Z | UNCLASSIFIED | intel_research | user=jesse | company=Anthropic | phase=discovery | tool=WebSearch | query="Anthropic funding 2025" | results=12 | tier=local | tokens=0 | cost=$0.00
2025-01-23T10:24:12Z | UNCLASSIFIED | intel_research | user=jesse | company=Anthropic | phase=deep_research | tool=WebFetch | url=https://www.anthropic.com/about | bytes=45231 | tier=local | tokens=0 | cost=$0.00
2025-01-23T10:25:33Z | UNCLASSIFIED | intel_research | user=jesse | company=Anthropic | phase=analysis | tool=llm | prompt="Analyze competitive..." | tier=sonnet | tokens=8472 | cost=$0.042
```

---

### 7.6 CLI Integration

**Purpose**: Add new `rigrun intel` command to existing CLI

**Files Modified**:
- `src/main.rs` (Rust) - Add intel subcommand
- `go-tui/internal/cli/cli.go` (Go) - Add intel command handler

**Implementation** (Rust):
```rust
// src/main.rs
use clap::{Parser, Subcommand};

#[derive(Parser)]
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,
}

#[derive(Subcommand)]
enum Commands {
    Intel {
        /// Company name to research
        company: String,

        /// Research depth (quick/standard/deep)
        #[arg(long, default_value = "standard")]
        depth: String,

        /// Output format (markdown/json/pdf)
        #[arg(long, default_value = "markdown")]
        format: String,

        /// Classification level
        #[arg(long, default_value = "UNCLASSIFIED")]
        classification: String,

        /// Maximum iterations
        #[arg(long, default_value = "20")]
        max_iter: usize,
    },
    // ... existing commands
}

async fn handle_intel_command(company: String, depth: String, format: String, ...) {
    let orchestrator = IntelOrchestrator::new();
    let report = orchestrator.research(company, depth, format).await?;
    println!("Report generated: {}", report.path);
}
```

**Implementation** (Go):
```go
// go-tui/internal/cli/intel.go
func HandleIntel(args []string) error {
    // Parse arguments
    company := args[0]
    depth := getFlag(args, "--depth", "standard")
    format := getFlag(args, "--format", "markdown")
    classification := getFlag(args, "--classification", "UNCLASSIFIED")

    // Create orchestrator
    orch := intel.NewOrchestrator(config)

    // Run research
    report, err := orch.Research(company, depth, format, classification)
    if err != nil {
        return err
    }

    fmt.Printf("Report generated: %s\n", report.Path)
    return nil
}
```

---

### 7.7 Tool Execution Integration

**Purpose**: Use all rigrun tools (Read, Write, Glob, Grep, Bash)

**Tools Used**:

1. **Write** - Generate markdown/JSON reports
   ```go
   executor.Execute(ctx, WriteTool, &ToolCall{
       Name: "Write",
       Params: map[string]interface{}{
           "file_path": reportPath,
           "content": renderedMarkdown,
       },
   })
   ```

2. **Bash** - GitHub stats, PDF export, data exports
   ```go
   // Get GitHub repo stats
   executor.Execute(ctx, BashTool, &ToolCall{
       Name: "Bash",
       Params: map[string]interface{}{
           "command": fmt.Sprintf("gh repo view %s/%s", company, repo),
       },
   })

   // Export to PDF
   executor.Execute(ctx, BashTool, &ToolCall{
       Name: "Bash",
       Params: map[string]interface{}{
           "command": fmt.Sprintf("pandoc %s -o %s", mdPath, pdfPath),
       },
   })
   ```

3. **Read** - Load templates
   ```go
   executor.Execute(ctx, ReadTool, &ToolCall{
       Name: "Read",
       Params: map[string]interface{}{
           "file_path": "~/.rigrun/intel/templates/report_template.md",
       },
   })
   ```

4. **Glob** - Find previous reports
   ```go
   executor.Execute(ctx, GlobTool, &ToolCall{
       Name: "Glob",
       Params: map[string]interface{}{
           "pattern": fmt.Sprintf("~/.rigrun/intel/reports/%s_*.md", company),
       },
   })
   ```

5. **Grep** - Search cached data
   ```go
   // Find all mentions of a competitor in cached reports
   executor.Execute(ctx, GrepTool, &ToolCall{
       Name: "Grep",
       Params: map[string]interface{}{
           "pattern": competitorName,
           "path": "~/.rigrun/intel/reports/",
           "output_mode": "files_with_matches",
       },
   })
   ```

---

## 8. Report Format

### Markdown Report Template

```markdown
# Competitive Intelligence Report: {{company_name}}

**Generated**: {{timestamp}}
**Research Depth**: {{depth}}
**Classification**: {{classification}}
**Researcher**: {{user}}

---

## Executive Summary

{{executive_summary}}

**Key Findings**:
- {{finding_1}}
- {{finding_2}}
- {{finding_3}}

---

## 1. Company Overview

**Founded**: {{founded_year}}
**Founders**: {{founders}}
**Headquarters**: {{hq_location}}
**Employees**: {{employee_count}}
**Mission**: {{mission_statement}}

{{company_description}}

---

## 2. Funding & Financials

**Total Funding**: {{total_funding}}
**Latest Round**: {{latest_round}} ({{round_date}})
**Valuation**: {{valuation}}
**Key Investors**: {{investors}}

### Funding History
{{funding_table}}

---

## 3. Leadership Team

**CEO**: {{ceo_name}} - {{ceo_background}}
**CTO**: {{cto_name}} - {{cto_background}}

### Full Leadership
{{leadership_table}}

---

## 4. Product Portfolio

### Core Products
{{product_list}}

### Pricing
{{pricing_table}}

### Key Features
{{features_comparison}}

**vs rigrun**:
{{rigrun_comparison}}

---

## 5. Technology Stack

**Languages**: {{languages}}
**Frameworks**: {{frameworks}}
**Infrastructure**: {{infrastructure}}
**Key Dependencies**: {{dependencies}}

{{tech_analysis}}

---

## 6. Recent News & Announcements

{{news_items}}

---

## 7. Market Position

**Target Market**: {{target_market}}
**Notable Customers**: {{customer_list}}
**Market Share**: {{market_share_estimate}}

### Case Studies
{{case_studies}}

---

## 8. Competitive Analysis

### SWOT Analysis

**Strengths**:
{{strengths}}

**Weaknesses**:
{{weaknesses}}

**Opportunities**:
{{opportunities}}

**Threats**:
{{threats}}

### Competitive Positioning

{{positioning_analysis}}

---

## 9. Strategic Recommendations

### For rigrun Product Team

1. **{{recommendation_1_title}}**
   {{recommendation_1_detail}}

2. **{{recommendation_2_title}}**
   {{recommendation_2_detail}}

3. **{{recommendation_3_title}}**
   {{recommendation_3_detail}}

---

## Appendix

### Research Metadata

- **Total Research Time**: {{research_duration}}
- **Queries Executed**: {{query_count}}
- **URLs Fetched**: {{url_count}}
- **Tokens Consumed**: {{total_tokens}}
- **Estimated Cost**: {{total_cost}}
- **Cache Hit Rate**: {{cache_hit_rate}}
- **Local/Cloud Split**: {{local_pct}}% local, {{cloud_pct}}% cloud

### Sources

{{source_list}}

---

*Generated by rigrun v{{version}} | Classification: {{classification}}*
```

---

### JSON Export Format

```json
{
  "company": {
    "name": "Anthropic",
    "founded": 2021,
    "founders": ["Dario Amodei", "Daniela Amodei"],
    "hq": "San Francisco, CA",
    "employees": 150,
    "mission": "Build reliable, interpretable, and steerable AI systems"
  },
  "funding": {
    "total": "$7.6B",
    "latest_round": "Series C",
    "valuation": "$18.4B",
    "investors": ["Google", "Salesforce", "Spark Capital"]
  },
  "leadership": [
    {"name": "Dario Amodei", "title": "CEO", "background": "VP Research at OpenAI"},
    {"name": "Daniela Amodei", "title": "President", "background": "VP Operations at OpenAI"}
  ],
  "products": [
    {
      "name": "Claude",
      "type": "LLM API",
      "tiers": ["Free", "Pro ($20/mo)", "Team", "Enterprise"],
      "features": ["200K context", "Vision", "Tool use", "Artifacts"]
    }
  ],
  "tech_stack": {
    "languages": ["Python", "Rust", "TypeScript"],
    "frameworks": ["PyTorch", "JAX"],
    "infrastructure": ["AWS", "GCP"]
  },
  "news": [
    {
      "date": "2025-01-15",
      "title": "Anthropic releases Claude 3.5 Opus",
      "url": "https://www.anthropic.com/news/claude-3-opus"
    }
  ],
  "swot": {
    "strengths": ["Strong safety focus", "Constitutional AI", "200K context"],
    "weaknesses": ["Smaller than OpenAI", "Limited model selection"],
    "opportunities": ["Enterprise AI adoption", "Government contracts"],
    "threats": ["Open source models", "GPT-4 competition"]
  },
  "recommendations": [
    {
      "title": "Emphasize local-first positioning",
      "detail": "Anthropic is cloud-only. rigrun's classification-aware routing is unique.",
      "priority": "High"
    }
  ],
  "metadata": {
    "generated_at": "2025-01-23T10:30:00Z",
    "research_duration": "3m 42s",
    "queries_executed": 18,
    "urls_fetched": 12,
    "tokens_consumed": 42891,
    "cost_usd": 0.21,
    "cache_hit_rate": 0.42,
    "local_cloud_split": {"local": 0.85, "cloud": 0.15}
  }
}
```

---

### PDF Export

- Uses `pandoc` to convert markdown → PDF
- Custom CSS styling for professional appearance
- Includes rigrun branding in footer
- Classification banner at top of every page

```bash
# Command executed by Bash tool
pandoc report.md \
  -o report.pdf \
  --pdf-engine=xelatex \
  --template=~/.rigrun/intel/templates/report_template.tex \
  --metadata title="Competitive Intel: Anthropic" \
  --metadata classification="UNCLASSIFIED"
```

---

## 9. Caching Strategy

### Three-Level Caching

1. **Report-Level Cache** (Exact)
   - Key: Company name
   - Value: Path to completed report
   - TTL: Permanent (until user runs `--update`)
   - Purpose: Instant retrieval of existing reports

2. **Module-Level Cache** (Semantic)
   - Key: Module name + company name
   - Value: Research results for that module
   - TTL: Varies by module (1 day to 30 days)
   - Purpose: Partial refresh - only re-research stale modules

3. **Query-Level Cache** (Semantic)
   - Key: WebSearch/WebFetch query/URL
   - Value: Raw results
   - TTL: Varies by data type (1 day to 30 days)
   - Purpose: Avoid duplicate API calls

### Cache Invalidation

**Manual**:
```bash
# Clear all intel cache
rigrun intel cache clear

# Clear cache for specific company
rigrun intel cache clear "Anthropic"

# Update report (only refreshes stale modules)
rigrun intel "Anthropic" --update
```

**Automatic**:
- Module cache expires based on TTL
- Report cache never expires (user must manually update)

### Smart Update Logic

When `--update` flag is used:
1. Load existing report from cache
2. Check TTL on each module
3. Only re-research modules with expired cache
4. Merge new data with cached data
5. Regenerate report with updated sections
6. Log which modules were refreshed

**Example**:
```bash
rigrun intel "Anthropic" --update

# Output:
# Loading cached report from 2025-01-15...
# Module 1 (Overview): Cached (28 days remaining)
# Module 2 (Funding): Expired, re-researching...
# Module 3 (Leadership): Cached (25 days remaining)
# Module 4 (Products): Expired, re-researching...
# Module 5 (Tech Stack): Cached (10 days remaining)
# Module 6 (News): Expired, re-researching...
# Module 7 (Market): Cached (12 days remaining)
# Module 8 (Analysis): Expired, re-researching...
#
# Updated 4 of 8 modules in 1m 23s
# Report regenerated: ~/.rigrun/intel/reports/anthropic_2025-01-23.md
```

---

## 10. Security & Compliance

### Classification Handling

**UNCLASSIFIED Research**:
- Uses cloud routing for analysis (Sonnet/Opus)
- WebSearch and WebFetch allowed
- Results cached without encryption
- Audit log records UNCLASSIFIED level

**CUI Research** (Controlled Unclassified Information):
- **Forced local-only routing** - classification router blocks cloud
- All research done with local models (qwen2.5:32b)
- Results encrypted at rest
- Audit log records CUI marking with HMAC integrity
- No external API calls allowed (except approved sources)

**Example**:
```bash
# Research on competitor handling CUI data
rigrun intel "Competitor Inc" --classification CUI

# Output:
# Classification: CUI detected
# Routing: FORCED LOCAL (cloud blocked)
# Model: qwen2.5:32b (local)
# All research will stay on-premise
#
# [Research proceeds with local-only routing]
```

### Audit Trail (IL5 Compliance)

Every research action is logged with:
- Timestamp (ISO 8601)
- User identity
- Classification level
- Tool used (WebSearch, WebFetch, Bash, etc.)
- Query/URL accessed
- Routing tier (local vs cloud)
- Tokens consumed and cost
- HMAC signature for audit log integrity

**Audit Log Integrity** (NIST 800-53 AU-9):
- Each log entry signed with HMAC-SHA256
- Secret key stored in `~/.rigrun/audit_key` (600 permissions)
- Tampering detection on log read
- Supports compliance audits for IL5/DoD environments

### Secret Redaction

All URLs, API keys, and sensitive data automatically redacted from logs:
- API keys: `sk-***`
- URLs: `https://***`
- File paths: `/home/***/file.txt`
- Email addresses: `***@example.com`

### Data Sovereignty

- **Local mode**: All research done on-premise, no cloud APIs
- **Paranoid mode**: Blocks all external network access
- **Air-gapped mode**: Can operate fully offline (uses cached data only)

---

## 11. Implementation Plan

### Phase 1: Core Infrastructure (Week 1)

**Goal**: Basic intel command working with 1-2 modules

**Tasks**:
1. Create `src/intel/` and `go-tui/internal/intel/` directories
2. Implement `Orchestrator` struct with agent loop
3. Add Module 1 (Company Overview) - WebSearch integration
4. Add Module 2 (Funding) - WebSearch + WebFetch integration
5. Implement basic markdown report generation
6. Add `rigrun intel <company>` CLI command
7. Wire up routing (local for discovery, cloud for analysis)
8. Add report-level exact caching

**Deliverable**: `rigrun intel "Anthropic"` generates basic 2-section report

---

### Phase 2: Full Research Modules (Week 2)

**Goal**: All 9 research modules implemented

**Tasks**:
1. Module 3: Leadership Team
2. Module 4: Product Portfolio (includes rigrun comparison)
3. Module 5: Technology Stack (includes GitHub integration)
4. Module 6: Recent News
5. Module 7: Market Position
6. Module 8: Competitive Analysis (cloud routing, SWOT)
7. Module 9: Strategic Recommendations (cloud routing, Opus preferred)
8. Add module-level semantic caching with TTLs
9. Implement smart update logic (`--update` flag)

**Deliverable**: Complete competitive intelligence reports

---

### Phase 3: Advanced Features (Week 3)

**Goal**: Batch mode, comparison mode, PDF export

**Tasks**:
1. Batch mode: `rigrun intel --batch competitors.txt`
2. Comparison mode: `rigrun intel --compare "A,B,C"`
3. PDF export: `--format pdf` using pandoc
4. JSON export: `--format json`
5. Add `rigrun intel stats` command
6. Add `rigrun intel list` command
7. Add `rigrun intel export --all`
8. Implement comparison report template

**Deliverable**: Full feature set complete

---

### Phase 4: Polish & Documentation (Week 4)

**Goal**: Production-ready, documented system

**Tasks**:
1. Write user guide: `docs/INTEL_USAGE_GUIDE.md`
2. Create example reports: `examples/intel/`
3. Add comprehensive error handling
4. Optimize caching (reduce duplicate queries)
5. Performance testing (research 10 companies, measure time/cost)
6. Security review (audit log integrity, classification enforcement)
7. Add unit tests for each module
8. Integration tests for full research workflow

**Deliverable**: Production-ready intel system with docs

---

### Timeline Summary

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| Phase 1 | Week 1 | Basic intel command with 2 modules |
| Phase 2 | Week 2 | All 9 modules, full reports |
| Phase 3 | Week 3 | Batch, comparison, PDF export |
| Phase 4 | Week 4 | Production-ready + docs |
| **Total** | **4 weeks** | **Complete intel system** |

---

## 12. Example Usage

### Scenario 1: Quick Competitor Scan

**Objective**: Get a quick overview of a new competitor

```bash
rigrun intel "Mistral AI" --depth quick

# Output:
# Researching: Mistral AI
# Depth: Quick (3 modules)
# Classification: UNCLASSIFIED
#
# [1/3] Company Overview...          ✓ (local, 12s, $0.00)
# [2/3] Funding...                   ✓ (local, 8s, $0.00)
# [3/3] Recent News...               ✓ (local, 6s, $0.00)
#
# Analysis (cloud: Sonnet)...        ✓ (18s, $0.08)
#
# Report generated: ~/.rigrun/intel/reports/mistral-ai_2025-01-23.md
#
# Summary:
# - Researched 3 modules in 44s
# - Tokens: 8,234 (98% local)
# - Cost: $0.08
# - Cache hits: 0
```

**Report Contents** (Quick Mode):
- Company Overview
- Funding & Financials
- Recent News
- Basic SWOT Analysis

---

### Scenario 2: Deep Competitive Analysis

**Objective**: Comprehensive research for strategic planning

```bash
rigrun intel "Anthropic" --depth deep

# Output:
# Researching: Anthropic
# Depth: Deep (all 9 modules)
# Classification: UNCLASSIFIED
#
# Phase 1: Discovery
# [1/9] Company Overview...          ✓ (local, 15s, $0.00)
# [2/9] Funding...                   ✓ (local, 12s, $0.00)
# [3/9] Leadership...                ✓ (local, 10s, $0.00)
#
# Phase 2: Deep Research
# [4/9] Product Portfolio...         ✓ (local→cloud, 28s, $0.04)
# [5/9] Technology Stack...          ✓ (local, 18s, $0.00)
# [6/9] Recent News...               ✓ (local, 9s, $0.00)
# [7/9] Market Position...           ✓ (local→cloud, 22s, $0.03)
#
# Phase 3: Analysis
# [8/9] Competitive Analysis...      ✓ (cloud: Sonnet, 45s, $0.12)
# [9/9] Strategic Recommendations... ✓ (cloud: Opus, 52s, $0.24)
#
# Report generated: ~/.rigrun/intel/reports/anthropic_2025-01-23.md
#
# Summary:
# - Researched 9 modules in 3m 31s
# - Queries: 24 (18 WebSearch, 6 WebFetch)
# - Tokens: 42,891 (85% local, 15% cloud)
# - Cost: $0.43
# - Cache hits: 8/24 (33%)
```

---

### Scenario 3: CUI-Classified Research

**Objective**: Research competitor handling sensitive data (must stay local)

```bash
rigrun intel "Palantir" --classification CUI

# Output:
# Researching: Palantir
# Classification: CUI ⚠️
# Routing: FORCED LOCAL (cloud blocked)
# Model: qwen2.5:32b
#
# ⚠️ CUI CLASSIFICATION ACTIVE ⚠️
# All research will remain on-premise.
# Cloud routing is DISABLED.
#
# Phase 1: Discovery
# [1/9] Company Overview...          ✓ (local, 18s, $0.00)
# [2/9] Funding...                   ✓ (local, 14s, $0.00)
# ...
#
# Phase 3: Analysis
# [8/9] Competitive Analysis...      ✓ (local: qwen2.5:32b, 89s, $0.00)
# [9/9] Strategic Recommendations... ✓ (local: qwen2.5:32b, 76s, $0.00)
#
# Report generated: ~/.rigrun/intel/reports/palantir_2025-01-23.md
# Report encrypted: ~/.rigrun/intel/reports/palantir_2025-01-23.md.enc
#
# Summary:
# - Researched 9 modules in 8m 12s
# - Tokens: 64,238 (100% local)
# - Cost: $0.00
# - Classification: CUI (enforced)
# - Audit log: ~/.rigrun/intel/audit.log
```

**Key Differences**:
- 100% local routing (no cloud API calls)
- Longer research time (local models slower for complex analysis)
- Zero cost (all local inference)
- Report encrypted at rest
- Audit log includes CUI classification markers

---

### Scenario 4: Batch Research

**Objective**: Research multiple competitors at once

```bash
# Create competitors list
cat > competitors.txt <<EOF
OpenAI
Anthropic
Cohere
Mistral AI
Together AI
EOF

rigrun intel --batch competitors.txt

# Output:
# Batch mode: Researching 5 companies
# Depth: Standard (default)
#
# [1/5] OpenAI
#   Research complete in 2m 41s ($0.38)
#   Report: ~/.rigrun/intel/reports/openai_2025-01-23.md
#
# [2/5] Anthropic
#   Cache hit! Using existing report from 2025-01-23
#   Report: ~/.rigrun/intel/reports/anthropic_2025-01-23.md
#
# [3/5] Cohere
#   Research complete in 2m 18s ($0.32)
#   Report: ~/.rigrun/intel/reports/cohere_2025-01-23.md
#
# [4/5] Mistral AI
#   Research complete in 2m 05s ($0.29)
#   Report: ~/.rigrun/intel/reports/mistral-ai_2025-01-23.md
#
# [5/5] Together AI
#   Research complete in 2m 33s ($0.35)
#   Report: ~/.rigrun/intel/reports/together-ai_2025-01-23.md
#
# Batch Summary:
# - Companies researched: 5
# - Cache hits: 1
# - Total time: 9m 37s
# - Total cost: $1.34
# - Average time per company: 1m 55s
# - Average cost per company: $0.27
#
# Batch summary report: ~/.rigrun/intel/reports/batch_summary_2025-01-23.md
```

**Batch Summary Report** includes:
- Comparison table of all competitors
- Aggregate SWOT matrix
- Market positioning map
- Combined strategic recommendations

---

### Scenario 5: Comparison Mode

**Objective**: Side-by-side analysis of 3 competitors

```bash
rigrun intel --compare "OpenAI,Anthropic,Cohere"

# Output:
# Comparison mode: OpenAI vs Anthropic vs Cohere
#
# [1/3] Researching OpenAI...        ✓ (cache hit)
# [2/3] Researching Anthropic...     ✓ (cache hit)
# [3/3] Researching Cohere...        ✓ (cache hit)
#
# Generating comparison analysis (cloud: Opus)...
#   ✓ Complete (68s, $0.31)
#
# Comparison report: ~/.rigrun/intel/reports/comparison_openai-anthropic-cohere_2025-01-23.md
```

**Comparison Report** includes:
- Side-by-side feature matrix
- Pricing comparison table
- Market positioning chart
- Strengths/weaknesses matrix
- "Who wins in what scenario" analysis
- Strategic recommendations for rigrun positioning

---

### Scenario 6: Update Existing Report

**Objective**: Refresh stale data in an existing report

```bash
# Original report generated 10 days ago
rigrun intel "Anthropic" --update

# Output:
# Loading cached report from 2025-01-13...
# Checking module freshness...
#
# Module 1 (Overview):      ✓ Cached (20 days remaining)
# Module 2 (Funding):       ⟳ Expired, refreshing...
# Module 3 (Leadership):    ✓ Cached (20 days remaining)
# Module 4 (Products):      ⟳ Expired, refreshing...
# Module 5 (Tech Stack):    ✓ Cached (4 days remaining)
# Module 6 (News):          ⟳ Expired, refreshing...
# Module 7 (Market):        ✓ Cached (4 days remaining)
# Module 8 (Analysis):      ⟳ Expired, refreshing...
# Module 9 (Recommendations): ⟳ Expired, refreshing...
#
# Refreshing 5 of 9 modules...
#
# [2/9] Funding...                   ✓ (local, 11s, $0.00)
# [4/9] Products...                  ✓ (local→cloud, 24s, $0.03)
# [6/9] News...                      ✓ (local, 8s, $0.00)
# [8/9] Analysis...                  ✓ (cloud: Sonnet, 42s, $0.11)
# [9/9] Recommendations...           ✓ (cloud: Opus, 48s, $0.22)
#
# Report updated: ~/.rigrun/intel/reports/anthropic_2025-01-23.md
#
# Summary:
# - Updated 5 of 9 modules
# - Time: 2m 13s (vs 3m 31s for full research)
# - Cost: $0.36 (vs $0.43 for full research)
# - Savings: 37% time, 16% cost
```

---

## Conclusion

The **Competitive Intelligence System** demonstrates rigrun's full power as an agentic framework:

1. **WebSearch & WebFetch**: Discovery and deep research from the web
2. **Agent Loops**: Autonomous multi-step research (20+ iterations)
3. **Cloud Routing**: Complex analysis via Sonnet/Opus
4. **Local Routing**: Simple aggregation via local models (85% of work)
5. **Caching**: 3-level caching (report, module, query) with smart TTLs
6. **All Tools**: Read/Write reports, Bash for exports, Glob/Grep for data
7. **CLI Integration**: New `rigrun intel` command family
8. **Security**: Classification-aware, audit logging, IL5 compliant

**Business Impact**:
- **30 minutes → 3 minutes**: Manual research automated
- **Consistent methodology**: Every competitor analyzed the same way
- **Cost optimized**: 85% local, only 15% cloud
- **Compliance ready**: Full audit trail, CUI support, encrypted storage

**Technical Achievement**:
- Showcases rigrun as more than an LLM router - a complete agentic platform
- Demonstrates real-world value: competitive intelligence is a universal need
- Production-ready: caching, error handling, security, audit logging
- Extensible: Easy to add new research modules or customize templates

This system positions rigrun as **the definitive local-first agentic AI platform** for organizations that demand both intelligence and compliance.

---

*Document version: 1.0*
*Author: rigrun Engineering Team*
*Date: 2025-01-23*
