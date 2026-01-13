# Bearer Token Authentication Implementation for Rigrun

## Summary
This document describes the Bearer token authentication feature added to rigrun's API server.

## Changes Made

### 1. Added API Key Fields to Structs

**AppState** (line ~57):
```rust
pub struct AppState {
    // ... existing fields ...
    /// API key for Bearer token authentication.
    pub api_key: Option<String>,
}
```

**ServerConfig** (line ~74):
```rust
pub struct ServerConfig {
    // ... existing fields ...
    /// API key for Bearer token authentication.
    pub api_key: Option<String>,
}
```

**Server** (line ~87):
Already has:
```rust
pub struct Server {
    // ... existing fields ...
    /// API key for Bearer token authentication.
    api_key: Option<String>,
}
```

### 2. Builder Method

Add after `with_cors_origins()` method (around line 160):
```rust
/// Set the API key for Bearer token authentication.
/// When set, requests must include an "Authorization: Bearer <token>" header.
pub fn with_api_key(mut self, key: impl Into<String>) -> Self {
    self.api_key = Some(key.into());
    self
}
```

### 3. Update AppState Initialization

In `build_router()` method (around line 195):
```rust
let state = Arc::new(AppState {
    config: ServerConfig {
        port: self.port,
        default_model: self.default_model.clone(),
        bind_address: self.bind_address.clone(),
        paranoid_mode: self.paranoid_mode,
        api_key: self.api_key.clone(),  // ADD THIS
    },
    ollama_client: OllamaClient::new(),
    openrouter_client,
    local_model: self.local_model.clone(),
    cache: RwLock::new(semantic_cache),
    paranoid_mode: self.paranoid_mode,
    api_key: self.api_key.clone(),  // ADD THIS
});
```

### 4. Authentication Middleware

Add before the `// Handlers` section (around line 375):
```rust
// =============================================================================
// Authentication Middleware
// =============================================================================

/// Authentication middleware that checks for Bearer token.
///
/// If api_key is configured, requests must include a valid "Authorization: Bearer <token>" header.
/// If api_key is None, all requests are allowed.
async fn require_auth(
    State(state): State<Arc<AppState>>,
    request: axum::extract::Request,
    next: axum::middleware::Next,
) -> Result<Response, (StatusCode, String)> {
    // If no API key is configured, allow all requests
    let api_key = match &state.api_key {
        Some(key) => key,
        None => return Ok(next.run(request).await),
    };

    // Extract Authorization header
    let auth_header = request
        .headers()
        .get(axum::http::header::AUTHORIZATION)
        .and_then(|h| h.to_str().ok());

    // Check for Bearer token
    match auth_header {
        Some(header) if header.starts_with("Bearer ") => {
            let token = &header[7..]; // Skip "Bearer "
            if token == api_key {
                // Valid token, proceed with request
                Ok(next.run(request).await)
            } else {
                // Invalid token
                Err((
                    StatusCode::UNAUTHORIZED,
                    "Unauthorized: Invalid API key. Include 'Authorization: Bearer <your-api-key>' header".to_string(),
                ))
            }
        }
        _ => {
            // Missing or malformed Authorization header
            Err((
                StatusCode::UNAUTHORIZED,
                "Unauthorized: Include 'Authorization: Bearer <your-api-key>' header".to_string(),
            ))
        }
    }
}
```

### 5. Split Router into Public and Protected Routes

Replace the router creation in `build_router()` (around line 220):
```rust
// REPLACE THIS:
Router::new()
    .route("/health", get(health_handler))
    .route("/v1/models", get(models_handler))
    .route("/v1/chat/completions", post(completions_handler))
    .route("/stats", get(stats_handler))
    .route("/cache/stats", get(cache_stats_handler))
    .route("/cache/semantic", get(semantic_cache_stats_handler))
    .layer(DefaultBodyLimit::max(MAX_BODY_SIZE))
    .layer(TimeoutLayer::new(std::time::Duration::from_secs(60)))
    .layer(GovernorLayer {
        config: governor_conf.clone(),
    })
    .layer(RateLimitHeadersLayer::new(governor_conf))
    .with_state(state)

// WITH THIS:
// Public routes (no authentication required)
let public_routes = Router::new()
    .route("/health", get(health_handler))
    .route("/v1/models", get(models_handler));

// Protected routes (require authentication when api_key is set)
let protected_routes = Router::new()
    .route("/v1/chat/completions", post(completions_handler))
    .route("/stats", get(stats_handler))
    .route("/cache/stats", get(cache_stats_handler))
    .route("/cache/semantic", get(semantic_cache_stats_handler))
    .route_layer(axum::middleware::from_fn_with_state(
        state.clone(),
        require_auth,
    ));

Router::new()
    .merge(public_routes)
    .merge(protected_routes)
    .layer(DefaultBodyLimit::max(MAX_BODY_SIZE))
    .layer(TimeoutLayer::new(std::time::Duration::from_secs(60)))
    .layer(GovernorLayer {
        config: governor_conf.clone(),
    })
    .layer(RateLimitHeadersLayer::new(governor_conf))
    .with_state(state)
```

### 6. Enhanced Security Warning

Update the warning in `start()` method (around line 245):
```rust
// Security warning if binding to all interfaces
if self.bind_address == "0.0.0.0" {
    if self.api_key.is_none() {
        tracing::warn!(
            "Server is binding to 0.0.0.0 WITHOUT authentication! \
            This exposes the API to the network without protection. \
            Consider setting an API key with .with_api_key() or use 127.0.0.1 (default) for local-only access."
        );
    } else {
        tracing::warn!(
            "Server is binding to 0.0.0.0 which exposes the API to the network. \
            API key authentication is enabled for protection."
        );
    }
}
```

## Protected vs Public Endpoints

### Public (No Authentication Required)
- `GET /health` - Health check
- `GET /v1/models` - List available models

### Protected (Require Bearer Token when API key is set)
- `POST /v1/chat/completions` - Chat completion requests
- `GET /stats` - Usage statistics
- `GET /cache/stats` - Cache statistics
- `GET /cache/semantic` - Semantic cache statistics

## Usage Example

```rust
use rigrun::server::Server;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let server = Server::new(8787)
        .with_bind_address("0.0.0.0") // Expose to network
        .with_api_key("your-secret-api-key-here"); // Protect with auth

    server.start().await?;
    Ok(())
}
```

## Client Usage

When API key is configured, clients must include the Authorization header:

```bash
curl -X POST http://localhost:8787/v1/chat/completions \
  -H "Authorization: Bearer your-secret-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

## Behavior

1. **When `api_key` is `None`** (default):
   - All requests are allowed
   - No authentication is performed
   - Warning logged if binding to 0.0.0.0

2. **When `api_key` is set**:
   - Protected endpoints require valid Bearer token
   - Public endpoints remain accessible
   - Returns 401 with helpful error message if auth fails
   - Info message logged when binding to 0.0.0.0

## Error Responses

### 401 Unauthorized
```json
"Unauthorized: Include 'Authorization: Bearer <your-api-key>' header"
```

Returned when:
- Authorization header is missing
- Authorization header doesn't start with "Bearer "
- Bearer token doesn't match configured API key
