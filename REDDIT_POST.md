# rigrun v0.2.0: Real semantic caching (60%+ hit rate vs 15% exact-match)

I built rigrun to cut my LLM API costs without sacrificing functionality. v0.2.0 just shipped with semantic caching that actually works.

## The problem with exact-match caching

Most LLM caches use exact string matching. They suck.

```
"What is recursion?" → MISS
"Explain recursion to me" → MISS
"Can you explain recursion?" → MISS
```

Same question, three cache misses. You're paying for the same answer three times.

**Exact-match hit rate in real usage: ~15%**

## How semantic caching works

rigrun uses embeddings to understand meaning, not just match strings:

1. Query comes in → generate embedding
2. Vector similarity search against cached queries
3. Match found above threshold (0.80) → return cached response
4. No match → route to LLM, cache with embedding

```
"What is recursion?" → Cache + index
"Explain recursion to me" → CACHE HIT (similarity: 0.92)
"Can you explain recursion?" → CACHE HIT (similarity: 0.89)
```

**Semantic cache hit rate: 60%+ in typical dev workflows**

## Real cost savings

Running this locally for 2 weeks on my coding workload:

- **Total hit rate:** 81.5%
- **Exact hits:** 55.4%
- **Semantic hits:** 26.2%

**Cost:** $12.35 (vs $145 for 100% OpenAI)
**Savings:** $132.65/month → ~$1,600/year

## Why rigrun vs LiteLLM?

**LiteLLM:** Feature-rich Python router, 100+ providers, complex config
**rigrun:** Single Rust binary, zero config, privacy-first, semantic cache built-in

|  | rigrun | LiteLLM |
|---|---|---|
| Binary size | 15MB | 200MB+ (with deps) |
| Startup time | <1s | 3-5s |
| Setup | `rigrun` | YAML + env vars |
| Semantic cache | ✅ Built-in | ❌ |
| Telemetry | None | Unknown |

Not saying LiteLLM is bad—it's great for enterprise setups needing 100+ providers. rigrun is for devs who want simplicity, speed, and privacy.

## Try it

```bash
# Install Ollama (local LLM runtime)
curl -fsSL https://ollama.com/install.sh | sh  # or download from ollama.com

# Install rigrun
cargo install rigrun  # or download binary from releases

# Start
rigrun
# Auto-detects GPU → downloads model → starts on localhost:8787

# Use with OpenAI SDK
from openai import OpenAI
client = OpenAI(base_url="http://localhost:8787/v1", api_key="unused")
response = client.chat.completions.create(
    model="auto",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

## What's next

v0.3.0 roadmap:
- Web UI for monitoring
- Streaming response support improvements
- Docker image
- Custom cache similarity thresholds

## Links

- GitHub: https://github.com/jeranaias/rigrun
- Docs: https://github.com/jeranaias/rigrun/tree/main/docs

---

Feedback welcome. If you try it and save money (or it doesn't work for your use case), let me know. Working on making local LLM inference actually usable.

⭐ Star if you find it useful—helps others discover it.
