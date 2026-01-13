# rigrun API Reference

rigrun exposes an OpenAI-compatible HTTP API plus auxiliary endpoints for health checks, stats, and cache inspection.

This document describes **all available endpoints**, request/response formats, and integration examples.

---

## Base URL

- **Default Base URL:** `http://localhost:8787`
- **API Version Prefix:** `/v1` (for OpenAI-compatible endpoints)

Adjust the port based on your rigrun configuration (`rigrun config --port <PORT>`).

---

## Authentication

rigrun runs **locally** and does **not require authentication** by default.

For compatibility with OpenAI client libraries, you can pass a dummy API key:

```http
Authorization: Bearer ANY_STRING
```

When using OpenAI SDKs:
- Set `base_url` to your rigrun address (`http://localhost:8787/v1`)
- Set `api_key` to any non-empty string (it will be ignored)

---

## Rate Limiting

rigrun does **not enforce rate limits** by default. If you deploy rigrun behind a reverse proxy (Nginx, Traefik, Cloudflare), configure rate limiting there.

Best practices:
- Implement client-side exponential backoff on `429` or `5xx` errors
- Keep request concurrency reasonable to avoid overloading your GPU

---

## Error Handling

### HTTP Status Codes

- `200 OK` - Request succeeded
- `400 Bad Request` - Invalid parameters or malformed request
- `404 Not Found` - Unknown endpoint
- `500 Internal Server Error` - Unexpected server error
- `503 Service Unavailable` - Model or provider unavailable

### Error Response Format

Errors follow OpenAI-compatible format:

```json
{
  "error": {
    "message": "Description of what went wrong",
    "type": "invalid_request_error",
    "param": null,
    "code": null
  }
}
```

Fields:
- `message` (string): Human-readable error description
- `type` (string): Error category (`invalid_request_error`, `server_error`, `model_not_found`)
- `param` (string|null): Parameter name related to the error
- `code` (string|null): Internal error code

### Best Practices

- Always check HTTP status code before parsing response
- Retry `5xx` errors with exponential backoff
- Validate required fields (`model`, `messages`) client-side

---

## Model Selection

rigrun routes requests based on the `model` parameter:

| Model ID | Description |
|----------|-------------|
| `auto` | Let rigrun choose optimal routing (cache → local → cloud) |
| `local` | Force local inference via Ollama |
| `cache` | Check cache only (returns cached response or falls back to local) |
| `cloud` | Route to OpenRouter with automatic model selection |
| `haiku` | Anthropic Claude 3 Haiku (fast, cheap) |
| `sonnet` | Anthropic Claude 3.5 Sonnet (balanced) |
| `opus` | Anthropic Claude 3 Opus (most capable) |

You can also specify a specific Ollama model name (e.g., `qwen2.5-coder:14b`).

---

## Endpoints

### 1. GET `/health`

Health check endpoint to verify rigrun is running.

#### Request

```bash
curl http://localhost:8787/health
```

#### Response

```json
{
  "status": "ok",
  "version": "0.1.0"
}
```

Fields:
- `status` (string): `"ok"` when healthy
- `version` (string): rigrun version

---

### 2. GET `/v1/models`

List available models (OpenAI-compatible).

#### Request

```bash
curl http://localhost:8787/v1/models
```

#### Response

```json
{
  "object": "list",
  "data": [
    {
      "id": "auto",
      "object": "model",
      "created": 0,
      "owned_by": "rigrun"
    },
    {
      "id": "local",
      "object": "model",
      "created": 0,
      "owned_by": "ollama"
    },
    {
      "id": "cloud",
      "object": "model",
      "created": 0,
      "owned_by": "openrouter"
    },
    {
      "id": "haiku",
      "object": "model",
      "created": 0,
      "owned_by": "anthropic"
    },
    {
      "id": "sonnet",
      "object": "model",
      "created": 0,
      "owned_by": "anthropic"
    },
    {
      "id": "opus",
      "object": "model",
      "created": 0,
      "owned_by": "anthropic"
    }
  ]
}
```

---

### 3. POST `/v1/chat/completions`

**Primary endpoint.** OpenAI-compatible chat completions.

#### Request Parameters

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `model` | string | yes | Model selector: `auto`, `local`, `cloud`, `haiku`, `sonnet`, `opus`, or model name |
| `messages` | array | yes | Conversation history (role + content) |
| `temperature` | float | no | Sampling temperature (0-2), default varies by model |
| `max_tokens` | int | no | Maximum tokens to generate |
| `stream` | bool | no | Enable streaming responses (default: false) |

**Message format:**

```json
{
  "role": "system | user | assistant",
  "content": "message text"
}
```

#### Non-Streaming Request

```bash
curl -X POST http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Explain HTTP vs HTTPS"}
    ],
    "temperature": 0.7,
    "max_tokens": 256
  }'
```

#### Response

```json
{
  "id": "chatcmpl-1234567890",
  "object": "chat.completion",
  "created": 1700000000,
  "model": "auto",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "HTTP is the Hypertext Transfer Protocol..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 45,
    "completion_tokens": 120,
    "total_tokens": 165
  }
}
```

Fields:
- `id` (string): Unique completion ID
- `object` (string): `"chat.completion"`
- `created` (int): Unix timestamp
- `model` (string): Model used
- `choices` (array):
  - `message.content` (string): Generated response
  - `finish_reason` (string): `"stop"` (completed), `"length"` (hit max_tokens)
- `usage`: Token counts

#### Streaming Request

```bash
curl -N -X POST http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Write a poem"}],
    "stream": true
  }'
```

#### Streaming Response Format

Server-Sent Events (SSE) format:

```text
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1700000000,"model":"auto","choices":[{"index":0,"delta":{"role":"assistant","content":"Local "},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1700000001,"model":"auto","choices":[{"index":0,"delta":{"content":"LLMs..."},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1700000002,"model":"auto","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

Each `delta` contains incremental tokens. Concatenate `delta.content` values to build the full response.

#### Error Responses

**Invalid model:**

```json
{
  "error": {
    "message": "Model 'unknown' is not supported",
    "type": "model_not_found",
    "param": "model",
    "code": null
  }
}
```

**Missing messages:**

```json
{
  "error": {
    "message": "Request must contain at least one message",
    "type": "invalid_request_error",
    "param": "messages",
    "code": null
  }
}
```

---

### 4. GET `/stats`

Usage statistics for session and today.

#### Request

```bash
curl http://localhost:8787/stats
```

#### Response

```json
{
  "session": {
    "queries": 42,
    "local_queries": 30,
    "cloud_queries": 12,
    "tokens_processed": 12345
  },
  "today": {
    "queries": 100,
    "saved_usd": 4.23,
    "spent_usd": 1.10
  }
}
```

Fields:

**session** (since server started):
- `queries` (int): Total queries
- `local_queries` (int): Queries handled locally
- `cloud_queries` (int): Queries routed to cloud
- `tokens_processed` (int): Total tokens (prompt + completion)

**today** (current calendar day):
- `queries` (int): Total queries today
- `saved_usd` (float): Estimated cost saved by using local inference
- `spent_usd` (float): Estimated cloud spend

---

### 5. GET `/cache/stats`

Cache performance statistics.

#### Request

```bash
curl http://localhost:8787/cache/stats
```

#### Response

```json
{
  "entries": 128,
  "total_lookups": 500,
  "hits": 320,
  "misses": 180,
  "expired_skips": 50,
  "total_stores": 200,
  "hit_rate_percent": 64.0,
  "ttl_hours": 24
}
```

Fields:
- `entries` (int): Current cached items
- `total_lookups` (int): Total cache lookups
- `hits` (int): Successful cache hits
- `misses` (int): Cache misses
- `expired_skips` (int): Expired entries skipped
- `total_stores` (int): Total items stored
- `hit_rate_percent` (float): Hit rate percentage
- `ttl_hours` (int): Cache time-to-live in hours

---

## Integration Examples

### Python (with OpenAI SDK)

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8787/v1",
    api_key="unused"
)

response = client.chat.completions.create(
    model="auto",
    messages=[
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Explain rigrun in one sentence."}
    ]
)

print(response.choices[0].message.content)
```

### JavaScript (with OpenAI SDK)

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://localhost:8787/v1',
  apiKey: 'unused',
});

const response = await client.chat.completions.create({
  model: 'auto',
  messages: [
    { role: 'system', content: 'You are a helpful assistant.' },
    { role: 'user', content: 'List benefits of local LLMs.' }
  ]
});

console.log(response.choices[0].message.content);
```

### Rust (with reqwest)

```rust
use reqwest::Client;
use serde::{Deserialize, Serialize};

#[derive(Serialize)]
struct ChatRequest {
    model: String,
    messages: Vec<Message>,
}

#[derive(Serialize)]
struct Message {
    role: String,
    content: String,
}

#[derive(Deserialize)]
struct ChatResponse {
    choices: Vec<Choice>,
}

#[derive(Deserialize)]
struct Choice {
    message: ChatMessage,
}

#[derive(Deserialize)]
struct ChatMessage {
    content: String,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = Client::new();

    let request = ChatRequest {
        model: "auto".to_string(),
        messages: vec![
            Message {
                role: "system".to_string(),
                content: "You are a helpful assistant.".to_string(),
            },
            Message {
                role: "user".to_string(),
                content: "Summarize rigrun.".to_string(),
            },
        ],
    };

    let response = client
        .post("http://localhost:8787/v1/chat/completions")
        .json(&request)
        .send()
        .await?
        .json::<ChatResponse>()
        .await?;

    println!("{}", response.choices[0].message.content);
    Ok(())
}
```

### cURL (with streaming)

```bash
curl -N -X POST http://localhost:8787/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Count to 5"}],
    "stream": true
  }' | while IFS= read -r line; do
    if [[ $line == data:* ]]; then
      echo "${line#data: }"
    fi
  done
```

---

## Notes & Best Practices

1. **Use `model: "auto"`** for most workloads - rigrun will optimize routing
2. **Monitor with `/stats`** to understand local vs cloud usage
3. **Check `/cache/stats`** to verify caching effectiveness
4. **Handle streaming carefully** - read line-by-line, strip `data:` prefix
5. **OpenAI SDK compatibility** - point `base_url` to `http://localhost:8787/v1`
6. **No authentication required** for local use, but compatible with API key headers

---

For more information:
- [Configuration Guide](configuration.md)
- [Quick Start Guide](getting-started.md)
- [Main README](../README.md)
