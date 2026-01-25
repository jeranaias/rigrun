# Competitive Intelligence System - Capabilities Matrix

> **How the intel system demonstrates EVERY rigrun capability**

---

## Capability Coverage

This matrix shows how the Competitive Intelligence System uses each of rigrun's core capabilities, with specific examples from the workflow.

---

## 1. WebSearch Integration

### Capability Description
Search the web using DuckDuckGo (no API key required) to find information across the internet.

### How Intel System Uses It

| Research Phase | Usage | Example Query | Output |
|----------------|-------|---------------|--------|
| Discovery | Company overview | `"Anthropic company overview history founded"` | Name, founded year, founders, mission |
| Discovery | Funding info | `"Anthropic funding rounds Series A B C valuation"` | Total funding, investors, valuation |
| Discovery | Leadership | `"Anthropic CEO Dario Amodei leadership team"` | C-suite executives, backgrounds |
| Research | Tech stack | `"Anthropic engineering blog technology stack"` | Languages, frameworks, infrastructure |
| Research | News | `"Anthropic news 2025 partnerships announcements"` | Recent news, product launches |
| Research | Market | `"Anthropic customers case studies enterprise"` | Customer list, market position |

**Total WebSearch Calls**: 5-10 per company
**Routing Tier**: Local (qwen2.5:14b)
**Cache TTL**: 7 days
**Cost**: $0.00 (all local)

---

## 2. WebFetch Integration

### Capability Description
Fetch and parse web pages, converting HTML to clean markdown for LLM processing.

### How Intel System Uses It

| Research Phase | Target URL | Purpose | Output |
|----------------|-----------|---------|--------|
| Discovery | `https://company.com/about` | Company background | Mission, history, team size |
| Research | `https://company.com/pricing` | Pricing tiers | Plans, features, costs |
| Research | `https://company.com/docs` | API documentation | Endpoints, capabilities |
| Research | `https://company.com/blog` | Recent posts | News, announcements |
| Research | `https://company.com/customers` | Case studies | Notable customers, use cases |
| Research | `https://crunchbase.com/organization/...` | Funding data | Detailed funding history |

**Total WebFetch Calls**: 6-12 per company
**Routing Tier**: Local → Cloud (escalate for complex extraction)
**Cache TTL**: 24 hours
**Cost**: $0.03-0.05 (mostly local)

---

## 3. Agent Loop (Autonomous Multi-Step Research)

### Capability Description
Agent autonomously decides what to research next based on previous findings, without human intervention.

### How Intel System Uses It

```
Iteration 1: WebSearch "Anthropic overview" → Found: AI safety company, 2021
Iteration 2: WebSearch "Anthropic funding" → Found: $7.6B raised
Iteration 3: WebFetch https://anthropic.com/about → Found: Team of 150
Iteration 4: WebSearch "Anthropic Claude product" → Found: LLM API
Iteration 5: WebFetch https://anthropic.com/pricing → Found: Pricing tiers
Iteration 6: Agent decides: Need tech stack info
Iteration 7: WebSearch "Anthropic engineering blog" → Found: Tech articles
Iteration 8: Bash "gh repo view anthropic/anthropic-sdk-python" → Found: GitHub stats
Iteration 9: Agent decides: Need recent news
Iteration 10: WebSearch "Anthropic news 2025" → Found: Recent announcements
...
[Continues for 15-20 iterations]
```

**Key Features**:
- Agent decides which URLs to fetch based on search results
- Agent identifies gaps in data and fills them
- Agent knows when to stop (sufficient data collected)
- Agent handles errors (retry with different query if tool fails)

**Max Iterations**: 20 (configurable via `--max-iter`)
**Decision Making**: LLM determines next action at each step
**Early Exit**: Stops when confidence score > 0.95

---

## 4. Cloud Routing (Complex Analysis)

### Capability Description
Route complex strategic analysis to powerful cloud models (Claude Sonnet/Opus) for high-quality synthesis.

### How Intel System Uses It

| Task | Model | Input | Output | Cost |
|------|-------|-------|--------|------|
| SWOT Analysis | Sonnet | All research data (7 modules) | Strengths, Weaknesses, Opportunities, Threats | $0.12 |
| Competitive Positioning | Sonnet | Company data + rigrun's features | Positioning analysis, differentiation | $0.08 |
| Strategic Recommendations | Opus | Full competitive analysis | 3-5 actionable recommendations | $0.24 |
| Feature Comparison | Sonnet | Product data from both companies | Feature-by-feature comparison matrix | $0.06 |

**When Cloud is Used**:
- Module 8: Competitive Analysis (SWOT, positioning)
- Module 9: Strategic Recommendations (action items)
- Module 4: Product comparison (when requested)

**When Cloud is NOT Used** (Classification = CUI):
- All analysis done with local qwen2.5:32b
- Longer execution time (2-3x slower)
- Zero cost
- 100% on-premise

**Routing Logic**:
```
IF classification >= CUI THEN
  FORCE LOCAL (cloud blocked)
ELSE
  IF task = "strategic analysis" THEN
    TierSonnet
  ELSEIF task = "recommendations" THEN
    TierOpus (if available) ELSE TierSonnet
  ELSE
    TierLocal
  END IF
END IF
```

---

## 5. Local Routing (Cost Optimization)

### Capability Description
Route 85% of work to local GPU models for zero marginal cost.

### How Intel System Uses It

| Phase | Tasks | Model | Routing % | Cost |
|-------|-------|-------|-----------|------|
| Discovery | WebSearch aggregation, basic extraction | qwen2.5:14b | 100% local | $0.00 |
| Research | WebFetch parsing, content extraction | qwen2.5:14b | 90% local | $0.00 |
| Analysis (CUI) | SWOT, recommendations | qwen2.5:32b | 100% local | $0.00 |
| Report Gen | Markdown formatting, templating | qwen2.5:14b | 100% local | $0.00 |

**Overall Split**:
- Discovery: 100% local (30s, $0.00)
- Research: 90% local (1m, $0.03)
- Analysis: 0% local / 100% cloud UNLESS CUI (1.5m, $0.36)
- Report Gen: 100% local (10s, $0.00)

**Total**: 85% local, 15% cloud (for UNCLASSIFIED)
**Total**: 100% local (for CUI)

---

## 6. Caching (Three Levels)

### Capability Description
Cache results at three levels to avoid re-researching companies or re-fetching URLs.

### How Intel System Uses It

#### Level 1: Report Cache (Exact Match)
```
Key: "intel:report:anthropic"
Value: Complete ResearchReport object
TTL: Permanent (until user runs --update)
Hit Rate: 10-20%

Example:
First run: 3m 31s, $0.43 → Cache MISS
Second run: Instant → Cache HIT
```

#### Level 2: Module Cache (Semantic)
```
Key: "intel:module:overview:anthropic"
Value: CompanyOverviewResult object
TTL: 30 days (basic facts change rarely)
Hit Rate: 30-40%

Example:
Day 1: Research overview → Cache MISS
Day 7: Update report → Overview cache HIT (reuse)
        Only re-research funding, news
```

#### Level 3: Query Cache (Semantic)
```
Key: Embedding of "Anthropic funding rounds 2025"
Value: WebSearch results
TTL: 7 days
Hit Rate: 40-50%

Example:
Query 1: "Anthropic funding rounds" → Cache MISS
Query 2: "Anthropic funding and investors" → Cache HIT (semantic similarity)
```

**Cache Impact**:
- Without cache: 3m 31s, $0.43
- With cache (33% hit): 2m 18s, $0.29
- **Savings**: 34% time, 33% cost

**Smart Update Logic**:
```bash
rigrun intel "Anthropic" --update

# Output:
# Module 1 (Overview): Cached (20 days remaining) ✓
# Module 2 (Funding): Expired, refreshing... ⟳
# Module 3 (Leadership): Cached (25 days remaining) ✓
# Module 4 (Products): Expired, refreshing... ⟳
# ...
# Updated 4 of 9 modules in 1m 23s
```

---

## 7. All Tools Demonstrated

### Capability Description
Use every tool in rigrun's toolkit for different tasks.

### How Intel System Uses Each Tool

#### Tool: Read
**Purpose**: Load report templates
**Usage**:
```
Tool: Read
Params: file_path="~/.rigrun/intel/templates/report_template.md"
Output: Template content with {{placeholders}}
Phase: Report Generation
Routing: Local
```

#### Tool: Write
**Purpose**: Generate markdown/JSON reports
**Usage**:
```
Tool: Write
Params:
  file_path="~/.rigrun/intel/reports/anthropic_2025-01-23.md"
  content="# Competitive Intelligence Report: Anthropic\n\n..."
Output: File written successfully
Phase: Report Generation
Routing: Local
```

#### Tool: Bash
**Purpose**: GitHub stats, PDF export
**Usage 1** (GitHub stats):
```
Tool: Bash
Params: command="gh repo view anthropic/anthropic-sdk-python"
Output: Stars, forks, language breakdown
Phase: Research (Tech Stack)
Routing: Local
```

**Usage 2** (PDF export):
```
Tool: Bash
Params: command="pandoc report.md -o report.pdf"
Output: PDF generated
Phase: Report Generation
Routing: Local
```

#### Tool: Glob
**Purpose**: Find previous reports
**Usage**:
```
Tool: Glob
Params: pattern="~/.rigrun/intel/reports/anthropic_*.md"
Output: List of all Anthropic reports
Phase: Stats/List commands
Routing: Local
```

#### Tool: Grep
**Purpose**: Search cached data
**Usage**:
```
Tool: Grep
Params:
  pattern="Dario Amodei"
  path="~/.rigrun/intel/reports/"
  output_mode="files_with_matches"
Output: List of reports mentioning Dario Amodei
Phase: Search across reports
Routing: Local
```

#### Tool: WebSearch
**Purpose**: Discovery phase
**Usage**: See "WebSearch Integration" section above

#### Tool: WebFetch
**Purpose**: Deep research phase
**Usage**: See "WebFetch Integration" section above

---

## 8. CLI Integration

### Capability Description
Add new commands to rigrun's CLI that feel native and consistent.

### How Intel System Uses It

#### New Command Family
```bash
rigrun intel <company>              # Research a company
rigrun intel --batch <file>         # Batch research
rigrun intel --compare "A,B,C"      # Compare competitors
rigrun intel stats                  # Show statistics
rigrun intel list                   # List all reports
rigrun intel cache clear            # Clear cache
rigrun intel export --all           # Export all reports
```

#### Command Options
```bash
--depth [quick|standard|deep]       # Research depth
--format [markdown|json|pdf]        # Output format
--output <directory>                # Custom output directory
--classification [UNCLASSIFIED|CUI] # Classification level
--use-cache                         # Use cached data (default: true)
--max-iter <n>                      # Max iterations (default: 20)
--paranoid                          # Local-only (no cloud)
--update                            # Smart update (only stale modules)
--force                             # Force full re-research
```

#### CLI Output
```
Researching: Anthropic
Depth: Standard (9 modules)
Classification: UNCLASSIFIED

Phase 1: Discovery
[1/9] Company Overview...          ✓ (local, 15s, $0.00)
[2/9] Funding...                   ✓ (local, 12s, $0.00)
[3/9] Leadership...                ✓ (local, 10s, $0.00)

Phase 2: Deep Research
[4/9] Product Portfolio...         ✓ (local→cloud, 28s, $0.04)
[5/9] Technology Stack...          ✓ (local, 18s, $0.00)
[6/9] Recent News...               ✓ (local, 9s, $0.00)
[7/9] Market Position...           ✓ (local→cloud, 22s, $0.03)

Phase 3: Analysis
[8/9] Competitive Analysis...      ✓ (cloud: Sonnet, 45s, $0.12)
[9/9] Strategic Recommendations... ✓ (cloud: Opus, 52s, $0.24)

Report generated: ~/.rigrun/intel/reports/anthropic_2025-01-23.md

Summary:
- Researched 9 modules in 3m 31s
- Queries: 24 (18 WebSearch, 6 WebFetch)
- Tokens: 42,891 (85% local, 15% cloud)
- Cost: $0.43
- Cache hits: 8/24 (33%)
```

---

## 9. Classification-Aware Routing

### Capability Description
UNIQUE TO RIGRUN: Automatic detection and enforcement of data classification levels.

### How Intel System Uses It

#### UNCLASSIFIED Research
```bash
rigrun intel "Anthropic"

# Routing:
# - Discovery: Local
# - Research: Local → Cloud (if needed)
# - Analysis: Cloud (Sonnet/Opus)
# - Report: Local

# Result:
# - Time: 3m 31s
# - Cost: $0.43
# - Cloud usage: 15%
```

#### CUI Research (Controlled Unclassified Information)
```bash
rigrun intel "Palantir" --classification CUI

# Output:
# ⚠️ CUI CLASSIFICATION ACTIVE ⚠️
# All research will remain on-premise.
# Cloud routing is DISABLED.

# Routing:
# - Discovery: Local (forced)
# - Research: Local (forced)
# - Analysis: Local qwen2.5:32b (forced)
# - Report: Local (forced)

# Result:
# - Time: 8m 12s (slower, local models)
# - Cost: $0.00 (100% local)
# - Cloud usage: 0%
# - Report encrypted at rest
# - Full audit trail
```

#### Classification Enforcement
```go
// CRITICAL: Classification check is FIRST, before any routing logic
if classification >= security.ClassificationCUI {
    // FORCE LOCAL - cloud is completely blocked
    return router.TierLocal
}

// Only if UNCLASSIFIED, proceed with complexity-based routing
tier := router.RouteQuery(query, classification, paranoidMode, maxTier)
```

**Why This Matters**:
- Government/defense contractors can't send CUI to cloud APIs
- rigrun is the ONLY LLM router with automatic classification enforcement
- Competitors (LiteLLM, OpenRouter) have no classification awareness
- This enables AI in environments where it wasn't possible before

---

## 10. Audit Logging (IL5 Compliant)

### Capability Description
Complete audit trail of all research actions for compliance and accountability.

### How Intel System Uses It

#### What Gets Logged
```
2025-01-23T10:23:45Z | UNCLASSIFIED | intel_research | user=jesse | company=Anthropic | phase=discovery | tool=WebSearch | query="Anthropic overview" | results=12 | tier=local | tokens=0 | cost=$0.00
2025-01-23T10:24:12Z | UNCLASSIFIED | intel_research | user=jesse | company=Anthropic | phase=research | tool=WebFetch | url=https://anthropic.com/about | bytes=45231 | tier=local | tokens=0 | cost=$0.00
2025-01-23T10:25:33Z | UNCLASSIFIED | intel_research | user=jesse | company=Anthropic | phase=analysis | tool=llm | prompt="Analyze competitive..." | tier=sonnet | tokens=8472 | cost=$0.042
```

#### Audit Trail Contents
- **Timestamp**: ISO 8601 format
- **Classification**: UNCLASSIFIED, CUI, SECRET, TOP_SECRET
- **Event Type**: intel_research, tool_execution, report_generated
- **User**: Who initiated the research
- **Company**: Target of research
- **Phase**: discovery, research, analysis, report
- **Tool**: WebSearch, WebFetch, Bash, Write, etc.
- **Query/URL**: What was searched or fetched
- **Routing Tier**: local, cloud, sonnet, opus
- **Tokens/Cost**: Resource consumption
- **HMAC Signature**: Tamper detection

#### Compliance Features
- **NIST 800-53 AU-6**: Audit review and analysis
- **NIST 800-53 AU-9**: Protection of audit information (HMAC integrity)
- **DoD IL5**: Complete provenance for all queries
- **Secret Redaction**: API keys automatically masked
- **Tamper Detection**: HMAC signature verifies log integrity

#### Usage
```bash
# View audit log
cat ~/.rigrun/intel/audit.log

# Export audit log
rigrun intel export --audit --output ./audit-export/

# Verify integrity
rigrun intel audit verify
```

---

## Capability Summary Matrix

| Capability | Used? | How | Impact |
|-----------|-------|-----|--------|
| **WebSearch** | ✅ | Discovery phase (5-10 searches) | Find company info |
| **WebFetch** | ✅ | Research phase (6-12 URLs) | Deep data extraction |
| **Agent Loop** | ✅ | 15-20 autonomous iterations | No human intervention |
| **Cloud Routing** | ✅ | Strategic analysis (Sonnet/Opus) | High-quality synthesis |
| **Local Routing** | ✅ | 85% of work (qwen2.5) | Cost optimization |
| **Caching** | ✅ | 3 levels (report, module, query) | 34% time savings |
| **Read Tool** | ✅ | Load templates | Report generation |
| **Write Tool** | ✅ | Generate reports (MD, JSON) | Output creation |
| **Bash Tool** | ✅ | GitHub stats, PDF export | Data enrichment |
| **Glob Tool** | ✅ | Find previous reports | Stats/list commands |
| **Grep Tool** | ✅ | Search cached data | Cross-report analysis |
| **CLI Integration** | ✅ | `rigrun intel` command family | User interface |
| **Classification** | ✅ | UNCLASSIFIED vs CUI routing | Security enforcement |
| **Audit Logging** | ✅ | Every action logged (IL5) | Compliance |

**Result**: ALL rigrun capabilities demonstrated in a single, coherent system.

---

## Why This Matters

### 1. Comprehensive Demonstration
Every rigrun capability is used in a **real, production-ready system**:
- Not toy examples
- Not isolated demos
- Not contrived use cases

**Real business value**: Research competitors 10x faster, 88% cheaper.

---

### 2. Unique Positioning
The intel system showcases what **only rigrun can do**:
- Classification-aware routing (no competitor has this)
- Local-first with cloud fallback (optimal cost/quality)
- IL5 compliant audit logging (government/defense ready)

**This is not just "better" - it's "uniquely capable."**

---

### 3. Platform Power
Shows rigrun as a **platform, not just a router**:
- Agent loops (autonomous multi-step research)
- Web integration (search + fetch)
- Tool ecosystem (Read, Write, Bash, Glob, Grep)
- Extensible (add custom modules, templates)

**This is an AI platform that can build complex applications.**

---

## Conclusion

The Competitive Intelligence System uses **ALL** rigrun capabilities:
- ✅ WebSearch (discovery)
- ✅ WebFetch (deep research)
- ✅ Agent Loops (autonomous 20+ iterations)
- ✅ Cloud Routing (strategic analysis)
- ✅ Local Routing (85% cost optimization)
- ✅ 3-Level Caching (34% time savings)
- ✅ All Tools (Read, Write, Bash, Glob, Grep)
- ✅ CLI Integration (native commands)
- ✅ Classification-Aware (CUI enforcement)
- ✅ Audit Logging (IL5 compliant)

**This is not a feature demo. This is a killer app that showcases rigrun's full power as an agentic AI platform.**

---

*Last updated: 2025-01-23*
*rigrun Engineering Team*
