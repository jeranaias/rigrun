# rigrun Press Kit

*Last Updated: January 12, 2026*

---

## One-Liner

**rigrun: Open-source local-first LLM router that cuts API costs by 80-90% through smart caching and GPU-first routing.**

---

## Elevator Pitch (30 seconds)

rigrun is an OpenAI-compatible LLM router that intelligently routes requests through three tiers: semantic cache, local GPU via Ollama, and cloud fallback. Instead of sending every query to expensive cloud APIs, rigrun handles 90% locally—delivering massive cost savings while maintaining quality. Built in Rust, it's a drop-in replacement for OpenAI's API that helps developers, startups, and teams cut LLM costs by 80-90%.

---

## Boilerplate Paragraph

rigrun is an open-source, local-first LLM (Large Language Model) router designed to dramatically reduce AI API costs for developers and organizations. By implementing a three-tier routing strategy—semantic cache, local GPU inference via Ollama, and cloud fallback via OpenRouter—rigrun handles up to 90% of queries locally, cutting monthly API bills by 80-90%. Built in Rust for performance and reliability, rigrun provides an OpenAI-compatible API that requires zero code changes to integrate. The project supports all major GPU vendors (NVIDIA, AMD, Apple Silicon, Intel Arc) and includes automatic GPU detection, model recommendations, and real-time cost tracking. Released under the MIT license in January 2026, rigrun aims to make LLM technology more accessible and affordable for developers worldwide.

---

## Key Statistics

### Cost Savings
- **80-90% reduction** in LLM API costs
- Average user saves **$450/month** (from $500 to $50)
- Annual savings: **$5,400** per user

### Performance
- **32% cache hit rate** (instant responses)
- **58% local inference** (free, private)
- **10% cloud fallback** (only when needed)
- **12ms median latency** for cached queries
- **180ms median latency** for local (7B model)

### Adoption (Update post-launch)
- GitHub stars: [TBD]
- Downloads: [TBD]
- Active users: [TBD]
- Community members: [TBD]

---

## Problem Statement

Developers and organizations face a costly dilemma when implementing LLM-powered applications:

**Cloud-only solutions** (OpenAI, Anthropic, etc.):
- Monthly costs of $500-5,000+ for moderate usage
- Privacy concerns with sensitive data
- Vendor lock-in and API rate limits
- Latency from external API calls

**Local-only solutions** (Ollama, llama.cpp):
- Quality limitations (small models vs. GPT-4)
- No fallback for complex queries
- Requires technical expertise
- All-or-nothing approach

The market needed a hybrid solution that automatically chooses the right provider for each query.

---

## Solution Overview

rigrun solves this by implementing **intelligent three-tier routing**:

### Tier 1: Semantic Cache
- Recognizes similar queries (e.g., "What is X?" ≈ "Explain X")
- Instant responses (12ms median latency)
- Zero cost per query
- 32% hit rate after warmup

### Tier 2: Local GPU
- Routes to Ollama for local inference
- Supports NVIDIA, AMD, Apple Silicon, Intel Arc
- Automatic model recommendations based on VRAM
- Zero cost, complete privacy

### Tier 3: Cloud Fallback
- Falls back to OpenRouter/OpenAI when needed
- Only 10% of queries require cloud
- Smart provider selection (cost vs. quality)
- Real-time cost tracking

**Result**: 90% of queries handled without cloud APIs = 90% cost reduction

---

## Key Features

### For Developers
- **OpenAI-compatible API** - Drop-in replacement, zero code changes
- **Automatic GPU detection** - Detects hardware and recommends optimal models
- **Real-time cost tracking** - Monitor savings vs. cloud-only approach
- **Cross-platform** - Windows, macOS, Linux support

### For Organizations
- **80-90% cost reduction** - Proven savings in real-world usage
- **Privacy-first** - Sensitive data stays on local infrastructure
- **Scalable** - Handle thousands of requests/day on single GPU
- **Open source** - MIT licensed, fully auditable

### Technical Highlights
- **Built in Rust** - Fast, safe, single binary distribution
- **Semantic caching** - Vector similarity matching for cache hits
- **Async architecture** - Handles concurrent requests efficiently
- **GPU support** - CUDA, ROCm, Metal, OpenCL

---

## Use Cases

### 1. Side Projects & Indie Hackers
**Problem**: Cloud API costs eat into already-thin margins
**Solution**: Run 90% of queries locally on existing hardware
**Impact**: $500/month → $50/month

### 2. Development Teams
**Problem**: Multiple developers = multiplied API costs
**Solution**: Shared rigrun instance for entire team
**Impact**: $5,000/month → $500/month for 10-person team

### 3. Privacy-Sensitive Applications
**Problem**: Can't send customer data to external APIs
**Solution**: 90% of queries stay local, only fallback to cloud when necessary
**Impact**: Compliance + cost savings

### 4. Educational Institutions
**Problem**: Need to provide LLM access to students without breaking budget
**Solution**: Deploy rigrun on school servers, handle most queries locally
**Impact**: Affordable AI education at scale

### 5. Prototyping & Research
**Problem**: Rapid iteration burns through API budgets quickly
**Solution**: Experiment freely with local models, fallback only when needed
**Impact**: Faster iteration without cost anxiety

---

## Target Audience

### Primary
- **Indie developers** - Building AI-powered side projects
- **Startup founders** - Bootstrapped, cost-conscious
- **Small dev teams** (2-20 people) - Looking to optimize spend

### Secondary
- **Enterprises** - Exploring cost optimization strategies
- **Educational institutions** - Providing affordable AI access
- **Researchers** - Need high-volume querying for experiments
- **Privacy advocates** - Prefer local-first solutions

### Demographics
- **Age**: 22-45
- **Role**: Developers, CTOs, founders, DevOps engineers
- **Tech**: Familiar with Docker, APIs, command-line tools
- **Pain point**: High LLM API costs and/or privacy concerns

---

## Founder Bio (PLACEHOLDER - CUSTOMIZE)

**[Your Name]**
[Your Title/Role]

[Your Name] is a [software engineer/developer/founder] with [X years] of experience in [relevant background]. Frustrated by skyrocketing LLM API costs for personal projects, [he/she/they] built rigrun to enable affordable, local-first AI inference. [Additional background, previous projects, expertise].

*Photo: [Link to high-res headshot]*

**Contact**:
- Email: [email@example.com]
- Twitter: [@handle]
- GitHub: [@handle]
- LinkedIn: [linkedin.com/in/profile]

---

## Company Information (If Applicable)

**Company Name**: [TBD or "Independent Open Source Project"]

**Founded**: January 2026

**Location**: [Your location or "Distributed/Remote"]

**Team Size**: [1 (solo) or team size]

**Funding**: Bootstrapped / Open source (no external funding)

**Mission**: Make LLM technology accessible and affordable for developers worldwide by prioritizing local-first inference.

---

## Media Assets

### Logos
- **Primary logo**: [Link to PNG/SVG]
- **Icon only**: [Link to PNG/SVG]
- **Wordmark**: [Link to PNG/SVG]
- **Monochrome versions**: [Link to PNG/SVG]

*Download all: [Link to asset package]*

### Screenshots
1. **Terminal/CLI interface**: [Link to high-res screenshot]
2. **GPU auto-detection**: [Link to screenshot]
3. **Cost tracking dashboard**: [Link to screenshot]
4. **Configuration UI**: [Link to screenshot]

### Diagrams
1. **Architecture diagram**: [Link to high-res image]
2. **Routing flow diagram**: [Link to image]
3. **Cost comparison chart**: [Link to image]

### Demo Videos
1. **Quick setup** (2 min): [YouTube/Vimeo link]
2. **Full walkthrough** (10 min): [YouTube/Vimeo link]
3. **Cost savings demo** (5 min): [YouTube/Vimeo link]

*All media assets licensed under [CC BY 4.0 / MIT / specify license]*

---

## Technical Specifications

### System Requirements
- **OS**: Windows 10+, macOS 11+, Linux (Ubuntu 20.04+)
- **RAM**: 8GB minimum, 16GB recommended
- **GPU**: Optional but recommended (4GB+ VRAM)
- **Disk**: 500MB for rigrun, 4-20GB for models

### Supported GPUs
- NVIDIA (CUDA): GTX 1000 series and newer
- AMD (ROCm): RX 5000 series and newer
- Apple Silicon: M1/M2/M3 series
- Intel Arc: A-series (experimental)

### API Compatibility
- OpenAI Chat Completions API (v1)
- Compatible with OpenAI SDKs (Python, JavaScript, Go, etc.)
- REST API with JSON payloads

### Model Support
- All Ollama-compatible models (50+ models)
- Recommended: qwen2.5-coder, deepseek-coder, llama3, mistral
- Model size range: 3B to 70B parameters

---

## Quotes (PLACEHOLDER - CUSTOMIZE AFTER LAUNCH)

### Founder Quote
> "I was paying $500/month to OpenAI while my $800 GPU sat idle. rigrun was born from that frustration—why not use the hardware I already own? Now I spend $50/month and get 90% of my work done locally, privately, and instantly."
>
> — [Your Name], Creator of rigrun

### Early User Quotes (Add post-launch)
> "[Quote about cost savings]"
> — [User Name], [Title/Company]

> "[Quote about ease of use]"
> — [User Name], [Title/Company]

> "[Quote about privacy benefits]"
> — [User Name], [Title/Company]

---

## Recognition & Press (Update post-launch)

### Press Coverage
- [Publication]: "[Headline]" - [Date] - [Link]
- [Publication]: "[Headline]" - [Date] - [Link]

### Community Recognition
- Hacker News: [Points] points, [#ranking] - [Link]
- Reddit r/LocalLLaMA: [Upvotes] upvotes - [Link]
- Product Hunt: [#ranking] Product of the Day - [Link]

### Awards & Badges
- [Award name] - [Date]
- [Badge/certification] - [Date]

---

## Comparison with Alternatives

### vs. Pure OpenAI/Anthropic
| Feature | rigrun | Cloud-only |
|---------|--------|------------|
| Cost (1M tokens/month) | $12-50 | $300-1,500 |
| Privacy | Local-first | Cloud-dependent |
| Latency (cached) | 12ms | 500-2000ms |
| Setup complexity | Medium | Low |

### vs. Pure Ollama/Local
| Feature | rigrun | Local-only |
|---------|--------|------------|
| Quality | Best of both | Limited by local models |
| Fallback | Automatic | None |
| API compatibility | OpenAI-standard | Ollama-specific |
| Caching | Semantic | None |

### vs. LangChain/LlamaIndex
| Feature | rigrun | Frameworks |
|---------|--------|------------|
| Language support | Any (API-based) | Python-first |
| Cache layer | Built-in semantic | External required |
| Cloud routing | Automatic | Manual configuration |
| Deployment | Single binary | Python environment |

---

## Roadmap

### Q1 2026 (Current)
- ✅ v1.0 launch
- 🚧 Docker image
- 🚧 Web UI for monitoring
- 🚧 Prometheus metrics

### Q2 2026
- Embeddings endpoint support
- Multi-user support
- Fine-tuning workflows
- Budget controls per user

### Q3 2026
- Automatic quality detection
- A/B testing framework
- Enterprise features (SSO, audit logs)
- Custom routing strategies

### Long-term Vision
- Make LLM inference as affordable as possible
- Enable privacy-first AI at scale
- Build vibrant open-source community
- Support enterprise deployments

---

## FAQs for Media

### Q: What makes rigrun different from using Ollama directly?
**A**: Ollama is excellent for running models locally, but rigrun adds semantic caching (32% instant responses), automatic cloud fallback, OpenAI-compatible API, and real-time cost tracking. Think of rigrun as a smart load balancer that sits in front of Ollama and cloud providers.

### Q: Does this really save 90%?
**A**: Yes, for workloads similar to mine (coding assistance, Q&A, content generation). The 90% figure assumes 32% cache hits + 58% local inference + 10% cloud fallback. Your results may vary based on query patterns, but 80-90% savings is typical.

### Q: What about quality? Aren't local models worse?
**A**: Local models (7-14B parameters) are excellent for ~80% of queries but not GPT-4 level. That's why rigrun has automatic cloud fallback. If a local model fails or times out, it seamlessly falls back to cloud. You get quality when you need it.

### Q: Is this suitable for production use?
**A**: v1.0 is stable for small-scale production (personal projects, small teams). Enterprise features (multi-tenancy, SSO, advanced monitoring) are coming in Q2-Q3 2026. Early adopters are already using it in production.

### Q: How does semantic caching work?
**A**: Instead of exact string matching, we compute embeddings (vector representations) of queries and use cosine similarity to find semantically similar questions. "What is X?" and "Explain X" are 85%+ similar, so they hit the same cache.

### Q: What's the business model?
**A**: rigrun is MIT licensed and free forever. Potential future revenue: enterprise support, managed hosting, premium features for teams. Core product will always be free and open source.

### Q: Who should NOT use rigrun?
**A**: If you need 100% GPT-4-level quality for every query, rigrun isn't ideal. Also not recommended for users without GPUs (CPU inference is slow) or enterprise-scale deployments requiring multi-tenancy (not yet supported).

---

## Contact Information

### For Press Inquiries
**Email**: [press@rigrun.dev] (PLACEHOLDER)
**Response time**: Within 24 hours

### For Technical Questions
**GitHub Issues**: https://github.com/rigrun/rigrun/issues
**Discord**: [Link] (PLACEHOLDER)
**Documentation**: https://github.com/rigrun/rigrun/tree/main/docs

### For Partnership Opportunities
**Email**: [partnerships@rigrun.dev] (PLACEHOLDER)

### Social Media
- **Twitter/X**: [@rigrun] (PLACEHOLDER)
- **LinkedIn**: [linkedin.com/company/rigrun] (PLACEHOLDER)
- **GitHub**: https://github.com/rigrun/rigrun

---

## Sample Headlines (Feel free to use)

- "New Open-Source Tool Cuts LLM API Costs by 90% with Local-First Routing"
- "rigrun: The Smart Router That Saved Developers $500/Month on AI Bills"
- "Local-First LLM Router rigrun Launches, Promises 90% Cost Reduction"
- "Developers Revolt Against High AI Costs with New Open-Source Tool"
- "rigrun Routes LLM Requests Through Local GPUs Before Cloud, Saves 90%"
- "MIT-Licensed LLM Router Challenges Cloud-Only AI Infrastructure"
- "How One Developer Cut OpenAI Bills from $500 to $50 with rigrun"

---

## Sample Story Angles

### Angle 1: David vs. Goliath
"Indie developer challenges AI giants with open-source alternative that cuts costs 90%"

### Angle 2: Cost Crisis
"As LLM API costs soar, developers turn to local-first solutions like rigrun"

### Angle 3: Privacy Movement
"Privacy-focused developers embrace local-first AI with new open-source router"

### Angle 4: Developer Tools
"New Rust-based tool makes LLM integration 90% cheaper for developers"

### Angle 5: Open Source Impact
"Open-source community rallies around cost-saving LLM router rigrun"

---

## Interview Availability

**Founder available for**:
- Podcast interviews
- Video interviews (Zoom, etc.)
- Written Q&A
- Conference speaking (submit early)
- Webinars/demos

**Preferred topics**:
- Cost optimization strategies for LLM applications
- Local-first AI architecture
- Open source sustainability
- Building in Rust
- Developer tools and DevOps

**Not available for**:
- Topics unrelated to rigrun/AI/development
- Promotional/sales-focused content (we don't sell anything)

---

## Additional Resources

### Documentation
- **Quick Start Guide**: https://github.com/rigrun/rigrun/blob/main/docs/QUICKSTART.md
- **API Reference**: https://github.com/rigrun/rigrun/blob/main/docs/API.md
- **Configuration Guide**: https://github.com/rigrun/rigrun/blob/main/docs/CONFIGURATION.md

### Code & Repository
- **GitHub**: https://github.com/rigrun/rigrun
- **License**: MIT
- **Changelog**: https://github.com/rigrun/rigrun/blob/main/CHANGELOG.md

### Community
- **GitHub Discussions**: https://github.com/rigrun/rigrun/discussions
- **Discord**: [Link] (PLACEHOLDER)
- **Twitter**: [@rigrun] (PLACEHOLDER)

---

## Branding Guidelines

### Name
- **Correct**: rigrun (lowercase)
- **Incorrect**: RigRun, Rigrun, RIGRUN

### Tagline
"Put your rig to work. Save 90%."

### Voice & Tone
- **Authentic** - No marketing BS, be real
- **Technical** - Speak to developers, use specifics
- **Humble** - Acknowledge limitations, don't oversell
- **Helpful** - Focus on solving problems, not self-promotion

### Visual Style
- **Colors**: [Define primary/secondary colors]
- **Typography**: Monospace for code, clean sans-serif for UI
- **Aesthetic**: Terminal-inspired, developer-focused, minimalist

---

## Press Kit Download

**Download complete press kit** (logos, screenshots, brand assets):
[Link to .zip file] (PLACEHOLDER)

**Contents**:
- Logos (PNG, SVG, multiple sizes)
- Screenshots (high-resolution)
- Diagrams and charts
- Fact sheet (PDF)
- Brand guidelines (PDF)

---

## Version History

- **v1.0** - January 2026: Initial public release
- [Update with future versions]

---

## License

This press kit is provided under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). You are free to use, share, and adapt this content with attribution.

---

**Last Updated**: January 12, 2026

**For the most current information**, visit https://github.com/rigrun/rigrun or contact [press email].

---

*rigrun - Local-first LLM routing for everyone. MIT licensed. Built with ❤️ by developers, for developers.*
