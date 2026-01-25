# OpenRouter Free Models Reference

This document lists free models available through OpenRouter that can be used with `rigrun intel --model=<model-id>`.

> **Note:** Free model availability changes frequently. Check [OpenRouter Models](https://openrouter.ai/models?q=free) for the latest list.

## Usage

```bash
# Use OpenRouter auto-routing (default - recommended)
rigrun intel "Company Name"

# Force a specific free model
rigrun intel "Company Name" --model=mistralai/devstral-2512:free

# Force a specific paid model
rigrun intel "Company Name" --model=anthropic/claude-sonnet-4
```

## Free Models (as of January 2026)

### Coding & Agentic Tasks

| Model ID | Context | Best For |
|----------|---------|----------|
| `mistralai/devstral-2512:free` | 262K | State-of-the-art agentic coding, SWE tasks |
| `xiaomi/mimo-v2-flash:free` | 262K | #1 open-source on SWE-bench, coding, agents |
| `nvidia/nemotron-3-nano-30b-a3b:free` | 256K | Agentic AI systems, tool use |
| `liquid/lfm-2.5-1.2b-thinking:free` | 32K | Reasoning, data extraction, RAG |

### Long Context & Reasoning

| Model ID | Context | Best For |
|----------|---------|----------|
| `arcee-ai/trinity-mini:free` | 131K | Long context reasoning, 26B params (3B active) |
| `tngtech/tng-r1t-chimera:free` | 164K | Creative reasoning, storytelling |
| `liquid/lfm-2.5-1.2b-instruct:free` | 32K | Fast instruction following |

### Vision & Multimodal

| Model ID | Context | Best For |
|----------|---------|----------|
| `allenai/molmo-2-8b:free` | 37K | Image, video, multi-image understanding |
| `nvidia/nemotron-nano-12b-v2-vl:free` | 128K | Video understanding, document intelligence |

## Model Recommendations

### For Intelligence Reports (rigrun intel)

**Best free option:** `mistralai/devstral-2512:free`
- 262K context handles large research data
- Strong reasoning and synthesis capabilities
- Optimized for agentic workflows

```bash
rigrun intel "Anthropic" --model=mistralai/devstral-2512:free
```

**Alternative:** `xiaomi/mimo-v2-flash:free`
- Top-ranked on benchmarks
- Excellent coding and reasoning

### For Quick Analysis

**Best free option:** `liquid/lfm-2.5-1.2b-thinking:free`
- Fast and lightweight
- Good for data extraction and RAG

### For Vision Tasks

**Best free option:** `nvidia/nemotron-nano-12b-v2-vl:free`
- 128K context
- Video and document understanding

## Auto-Routing (Default)

When you don't specify `--model`, rigrun uses `openrouter/auto` which lets OpenRouter automatically select the optimal model based on:
- Query complexity
- Required capabilities
- Cost optimization
- Current model availability

This is recommended for most use cases as it balances quality and cost.

## Cost Comparison

| Model | Input (per 1M tokens) | Output (per 1M tokens) |
|-------|----------------------|------------------------|
| Free models (`:free`) | $0.00 | $0.00 |
| `openrouter/auto` | ~$0.50-3.00 | ~$1.50-15.00 |
| `anthropic/claude-sonnet-4` | $3.00 | $15.00 |
| `anthropic/claude-opus-4` | $15.00 | $75.00 |
| `openai/gpt-4o` | $2.50 | $10.00 |

## Checking Latest Free Models

Free models change frequently. To see the current list:

1. Visit: https://openrouter.ai/models?q=free
2. Or use the API: `curl https://openrouter.ai/api/v1/models | jq '.data[] | select(.id | endswith(":free"))'`

## Limitations of Free Models

- May have rate limits
- Some have usage logging policies
- Quality varies - test before production use
- May be removed or changed without notice

## See Also

- [OpenRouter Documentation](https://openrouter.ai/docs)
- [OpenRouter Models](https://openrouter.ai/models)
- [rigrun intel command](../README.md)
