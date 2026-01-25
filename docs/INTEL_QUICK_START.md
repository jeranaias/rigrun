# Competitive Intelligence System - Quick Start Guide

> **Get started with rigrun's intel system in 5 minutes**

---

## Installation

The intel system is part of rigrun core. No additional installation needed.

```bash
# Verify rigrun is installed
rigrun --version

# Check if intel command is available
rigrun intel --help
```

---

## Basic Usage

### Research a Single Company

```bash
# Quick scan (3 modules, ~1 minute)
rigrun intel "Anthropic" --depth quick

# Standard research (9 modules, ~3 minutes)
rigrun intel "Anthropic"

# Deep dive (9 modules + extra analysis, ~5 minutes)
rigrun intel "Anthropic" --depth deep
```

**Output**: Markdown report at `~/.rigrun/intel/reports/anthropic_YYYY-MM-DD.md`

---

### View Generated Report

```bash
# Open in default markdown viewer
cat ~/.rigrun/intel/reports/anthropic_2025-01-23.md

# Open in browser (if pandoc installed)
pandoc ~/.rigrun/intel/reports/anthropic_2025-01-23.md -o /tmp/report.html
open /tmp/report.html  # macOS
xdg-open /tmp/report.html  # Linux
```

---

### Export to PDF

```bash
# Requires pandoc
rigrun intel "Anthropic" --format pdf

# Output: ~/.rigrun/intel/reports/anthropic_2025-01-23.pdf
```

**Install pandoc**:
```bash
# macOS
brew install pandoc

# Ubuntu/Debian
sudo apt install pandoc

# Windows
choco install pandoc
```

---

### Research Multiple Competitors

```bash
# Create a list of competitors
cat > competitors.txt <<EOF
OpenAI
Anthropic
Cohere
Mistral AI
EOF

# Run batch research
rigrun intel --batch competitors.txt

# Output: 4 individual reports + 1 batch summary report
```

---

### Compare Competitors

```bash
# Side-by-side comparison
rigrun intel --compare "OpenAI,Anthropic,Cohere"

# Output: Comparison report with feature matrix and positioning analysis
```

---

### Update Existing Report

```bash
# Refresh only stale data (smart update)
rigrun intel "Anthropic" --update

# Forces full re-research
rigrun intel "Anthropic" --update --force
```

---

## Understanding Research Depth

| Depth | Modules | Analysis | Time | Cost | Best For |
|-------|---------|----------|------|------|----------|
| **Quick** | 3 (Overview, Funding, News) | Basic SWOT | ~1 min | ~$0.05 | Initial discovery |
| **Standard** | 9 (All modules) | Full SWOT + Recommendations | ~3 min | ~$0.30 | Regular monitoring |
| **Deep** | 9 + Extra analysis | Extended strategic analysis | ~5 min | ~$0.50 | Strategic planning |

---

## Classification Levels

### UNCLASSIFIED Research (Default)

```bash
# Uses cloud routing for analysis
rigrun intel "Anthropic"

# Result: Fast, uses Sonnet/Opus for strategic analysis
```

### CUI Research (Local-Only)

```bash
# All research stays on-premise
rigrun intel "Palantir" --classification CUI

# Result:
# - 100% local routing (no cloud APIs)
# - Report encrypted at rest
# - Full audit trail
# - Zero cost (all local inference)
```

**When to use CUI**:
- Researching competitors handling sensitive government data
- Compliance requirements mandate local-only processing
- Paranoid mode for maximum security

---

## Managing Reports

### List All Reports

```bash
rigrun intel list

# Output:
# Anthropic       2025-01-23  Standard  UNCLASSIFIED  $0.43
# OpenAI          2025-01-22  Deep      UNCLASSIFIED  $0.51
# Palantir        2025-01-20  Standard  CUI           $0.00
```

### Show Statistics

```bash
rigrun intel stats

# Output:
# Total companies researched: 12
# Total reports generated: 15
# Total research time: 47m 23s
# Total tokens consumed: 523,891
# Total cost: $4.21
# Cache hit rate: 38%
# Local/Cloud split: 84% local, 16% cloud
```

### Clear Cache

```bash
# Clear all intel cache
rigrun intel cache clear

# Clear specific company
rigrun intel cache clear "Anthropic"

# Clear only expired cache entries
rigrun intel cache prune
```

### Export All Reports

```bash
# Export all reports to directory
rigrun intel export --all --output ./intel-export/

# Export as ZIP archive
rigrun intel export --all --archive intel-reports.zip
```

---

## Advanced Options

### Custom Output Directory

```bash
rigrun intel "Anthropic" --output ./reports/competitors/
```

### JSON Export

```bash
rigrun intel "Anthropic" --format json

# Output: JSON file with structured data
# Use for programmatic analysis or integration with other tools
```

### Control Research Iterations

```bash
# Limit max iterations (faster, less thorough)
rigrun intel "Anthropic" --max-iter 10

# Allow more iterations (slower, more thorough)
rigrun intel "Anthropic" --max-iter 30
```

### Paranoid Mode (Air-Gapped)

```bash
# No external network access
rigrun intel "Anthropic" --paranoid

# Uses only cached data + local models
# Fails if required data not in cache
```

---

## Customization

### Custom Report Template

```bash
# Copy default template
cp ~/.rigrun/intel/templates/report_template.md ./my_template.md

# Edit template (uses Go text/template syntax)
vim ./my_template.md

# Use custom template
rigrun intel "Anthropic" --template ./my_template.md
```

### Custom Research Modules

Create a custom module in `~/.rigrun/intel/custom_modules/`:

```go
// ~/.rigrun/intel/custom_modules/custom_analysis.go
package custom_modules

import (
    "context"
    "github.com/jeranaias/rigrun-tui/internal/intel"
)

type CustomAnalysisModule struct{}

func (m *CustomAnalysisModule) Research(ctx context.Context, company string) (*intel.ModuleResult, error) {
    // Your custom research logic
    return &intel.ModuleResult{
        Name: "Custom Analysis",
        Data: map[string]interface{}{
            "custom_metric": "value",
        },
    }, nil
}
```

Enable in config:
```toml
# ~/.rigrun/config.toml
[intel]
custom_modules = ["~/.rigrun/intel/custom_modules/custom_analysis.go"]
```

---

## Troubleshooting

### Ollama Not Running

```
Error: Failed to connect to Ollama at http://localhost:11434

Solution:
1. Start Ollama: `ollama serve`
2. Or install: https://ollama.com/download
```

### Model Not Found

```
Error: Model qwen2.5-coder:14b not found

Solution:
rigrun pull qwen2.5-coder:14b
```

### Cloud API Key Missing

```
Warning: OpenRouter API key not configured. Cloud routing disabled.

Solution:
rigrun config set-key sk-or-v1-xxx
```

### Rate Limited by Website

```
Error: Rate limited by https://example.com (429 Too Many Requests)

Solution:
1. Wait 60 seconds and retry
2. Use --update to leverage cached data
3. Reduce --depth to minimize requests
```

### Out of Memory (Large Model)

```
Error: Out of memory loading qwen2.5:32b

Solution:
1. Use smaller model: rigrun config set-model qwen2.5-coder:14b
2. Or add more RAM/VRAM
```

---

## Best Practices

### 1. Start with Quick Depth

```bash
# Get initial overview quickly
rigrun intel "NewCompetitor" --depth quick

# If interesting, do deep dive
rigrun intel "NewCompetitor" --depth deep
```

### 2. Leverage Caching

```bash
# Research once
rigrun intel "Anthropic"

# Update only when needed (smart refresh)
rigrun intel "Anthropic" --update
```

### 3. Batch Research Weekly

```bash
# Create weekly research script
cat > weekly_intel.sh <<'EOF'
#!/bin/bash
rigrun intel --batch competitors.txt --update
EOF

chmod +x weekly_intel.sh

# Run via cron every Monday
# 0 9 * * 1 /path/to/weekly_intel.sh
```

### 4. Use CUI for Sensitive Research

```bash
# Always use CUI for government/defense competitors
rigrun intel "Palantir" --classification CUI
rigrun intel "BoozAllen" --classification CUI
```

### 5. Export Reports for Sharing

```bash
# Generate PDF for stakeholders
rigrun intel "Anthropic" --format pdf

# Share via email/Slack
```

---

## Integration Examples

### Integrate with CI/CD

```yaml
# .github/workflows/competitive-intel.yml
name: Weekly Competitive Intelligence

on:
  schedule:
    - cron: '0 9 * * 1'  # Every Monday at 9am

jobs:
  research:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2

      - name: Install rigrun
        run: |
          curl -fsSL https://install.rigrun.com | sh

      - name: Research competitors
        run: |
          rigrun intel --batch competitors.txt --output ./reports/
        env:
          RIGRUN_OPENROUTER_KEY: ${{ secrets.OPENROUTER_KEY }}

      - name: Upload reports
        uses: actions/upload-artifact@v2
        with:
          name: intel-reports
          path: ./reports/
```

### Slack Integration

```bash
# Post report to Slack
rigrun intel "Anthropic" --format json | jq -r '.executive_summary' | \
  curl -X POST https://slack.com/api/chat.postMessage \
    -H "Authorization: Bearer $SLACK_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"channel\":\"#competitive-intel\",\"text\":\"$(cat -)\"}"
```

### Python Integration

```python
import subprocess
import json

def research_competitor(company, depth="standard"):
    """Research a competitor using rigrun intel."""
    result = subprocess.run(
        ["rigrun", "intel", company, "--depth", depth, "--format", "json"],
        capture_output=True,
        text=True
    )

    if result.returncode == 0:
        return json.loads(result.stdout)
    else:
        raise Exception(f"Research failed: {result.stderr}")

# Example usage
anthropic_intel = research_competitor("Anthropic", depth="deep")
print(f"Funding: {anthropic_intel['funding']['total']}")
print(f"Strengths: {', '.join(anthropic_intel['swot']['strengths'])}")
```

---

## FAQ

### Q: How much does research cost?

**A**: Depends on depth and routing:
- Quick (UNCLASSIFIED): ~$0.05 (mostly local)
- Standard (UNCLASSIFIED): ~$0.30 (85% local, 15% cloud)
- Deep (UNCLASSIFIED): ~$0.50 (80% local, 20% cloud)
- Any depth (CUI): $0.00 (100% local)

### Q: How long does research take?

**A**: Depends on depth:
- Quick: ~1 minute
- Standard: ~3 minutes
- Deep: ~5 minutes

### Q: Can I research private companies?

**A**: Yes, but results depend on publicly available data. rigrun cannot access:
- Non-public financial data
- Internal documents
- Authenticated APIs

### Q: Is research data saved?

**A**: Yes, everything is cached:
- Reports: `~/.rigrun/intel/reports/`
- Cached data: `~/.rigrun/intel/cache/`
- Audit log: `~/.rigrun/intel/audit.log`

### Q: Can I customize what's researched?

**A**: Yes, via custom modules (see "Customization" section above).

### Q: Is this legal?

**A**: Yes. rigrun only accesses publicly available information via:
- Public search engines (DuckDuckGo)
- Public websites (company homepages, blogs, docs)
- Public APIs (GitHub, Crunchbase when available)

No scraping of paywalled content or authenticated resources.

---

## Next Steps

1. **Try it now**: `rigrun intel "Anthropic"`
2. **Read full design doc**: `docs/COMPETITIVE_INTEL_SYSTEM_DESIGN.md`
3. **Explore examples**: `examples/intel/`
4. **Join discussions**: [GitHub Discussions](https://github.com/rigrun/rigrun/discussions)

---

*Last updated: 2025-01-23*
*rigrun version: 0.2.0+*
