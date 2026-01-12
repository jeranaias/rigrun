//! API server
//!
//! Provides an HTTP API for LLM inference requests.
//! Implements an OpenAI-compatible API for easy integration.
//!
//! # Endpoints
//!
//! - `GET /health` - Health check
//! - `GET /v1/models` - List available models
//! - `POST /v1/chat/completions` - Chat completion (OpenAI-compatible)
//! - `GET /stats` - Usage statistics
//!
//! # Example
//!
//! ```no_run
//! use rigrun::server::Server;
//!
//! # async fn example() -> anyhow::Result<()> {
//! let server = Server::new(8787);
//! server.start().await?;
//! # Ok(())
//! # }
//! ```

use axum::{
    extract::{DefaultBodyLimit, State},
    http::StatusCode,
    response::Json,
    routing::{get, post},
    Router,
};
use serde::{Deserialize, Serialize};
use std::sync::{Arc, RwLock};
use std::time::Instant;
use anyhow::Result;
use tower_governor::{
    governor::GovernorConfigBuilder,
    key_extractor::SmartIpKeyExtractor,
    GovernorLayer,
};
use crate::cache::QueryCache;
use crate::local::{OllamaClient, Message};
use crate::router::{route_query, Tier};
use crate::stats;
use crate::cloud::{OpenRouterClient, Message as CloudMessage};

/// Server state shared across handlers.
pub struct AppState {
    /// Server configuration.
    pub config: ServerConfig,
    /// Ollama client for local inference.
    pub ollama_client: OllamaClient,
    /// OpenRouter client for cloud inference.
    pub openrouter_client: OpenRouterClient,
    /// Default local model name.
    pub local_model: String,
    /// Query cache for semantic caching.
    pub cache: RwLock<QueryCache>,
}

/// Server configuration.
#[derive(Clone)]
pub struct ServerConfig {
    /// Port to listen on.
    pub port: u16,
    /// Default model to use.
    pub default_model: String,
    /// Address to bind to (defaults to 127.0.0.1 for security).
    pub bind_address: String,
}

/// API server configuration.
#[derive(Debug)]
pub struct Server {
    /// Port to listen on.
    port: u16,
    /// Default model.
    default_model: String,
    /// Default local model for Ollama.
    local_model: String,
    /// OpenRouter API key.
    openrouter_key: Option<String>,
    /// Address to bind to (defaults to 127.0.0.1 for security).
    bind_address: String,
}

impl Default for Server {
    fn default() -> Self {
        Self::new(8787)
    }
}

impl Server {
    /// Create a new server with the specified port.
    /// By default, binds to 127.0.0.1 (localhost only) for security.
    pub fn new(port: u16) -> Self {
        Self {
            port,
            default_model: "auto".to_string(),
            local_model: "qwen2.5-coder:7b".to_string(),
            openrouter_key: None,
            bind_address: "127.0.0.1".to_string(),
        }
    }

    /// Set the default model.
    pub fn with_default_model(mut self, model: impl Into<String>) -> Self {
        self.default_model = model.into();
        self
    }

    /// Set the local model for Ollama.
    pub fn with_local_model(mut self, model: impl Into<String>) -> Self {
        self.local_model = model.into();
        self
    }

    /// Set the OpenRouter API key.
    pub fn with_openrouter_key(mut self, key: impl Into<String>) -> Self {
        self.openrouter_key = Some(key.into());
        self
    }

    /// Set the bind address.
    /// Use "0.0.0.0" to allow network access, "127.0.0.1" (default) for localhost only.
    pub fn with_bind_address(mut self, addr: impl Into<String>) -> Self {
        self.bind_address = addr.into();
        self
    }

    /// Build the router with all routes.
    pub fn build_router(&self) -> Router {
        // Initialize OpenRouter client with API key from config or environment
        let openrouter_client = if let Some(ref key) = self.openrouter_key {
            OpenRouterClient::with_api_key(key.clone())
        } else {
            OpenRouterClient::new() // Will try OPENROUTER_API_KEY env var
        };

        let state = Arc::new(AppState {
            config: ServerConfig {
                port: self.port,
                default_model: self.default_model.clone(),
                bind_address: self.bind_address.clone(),
            },
            ollama_client: OllamaClient::new(),
            openrouter_client,
            local_model: self.local_model.clone(),
            cache: RwLock::new(QueryCache::default_persistent()),
        });

        // Configure rate limiting: 60 requests per minute per IP
        let governor_conf = Arc::new(
            GovernorConfigBuilder::default()
                .per_second(1) // 1 request per second = 60 per minute
                .burst_size(60) // Allow burst of 60 requests
                .key_extractor(SmartIpKeyExtractor)
                .finish()
                .expect("Failed to build governor config")
        );

        Router::new()
            .route("/health", get(health_handler))
            .route("/v1/models", get(models_handler))
            .route("/v1/chat/completions", post(completions_handler))
            .route("/stats", get(stats_handler))
            .route("/cache/stats", get(cache_stats_handler))
            .layer(DefaultBodyLimit::max(MAX_BODY_SIZE))
            .layer(GovernorLayer {
                config: governor_conf,
            })
            .with_state(state)
    }

    /// Start the server with graceful shutdown.
    pub async fn start(&self) -> Result<()> {
        let router = self.build_router();
        let addr = format!("{}:{}", self.bind_address, self.port);

        tracing::info!("Starting server on {}", addr);

        // Security warning if binding to all interfaces
        if self.bind_address == "0.0.0.0" {
            tracing::warn!(
                "Server is binding to 0.0.0.0 which exposes the API to the network. \
                Use 127.0.0.1 (default) for local-only access."
            );
        }

        let listener = tokio::net::TcpListener::bind(&addr).await.map_err(|e| {
            if e.kind() == std::io::ErrorKind::AddrInUse {
                anyhow::anyhow!(
                    "Port {} is already in use (os error 10048). \
                    This usually means another rigrun server is running. \
                    Try stopping other instances or use a different port with: rigrun config --port <PORT>",
                    self.port
                )
            } else {
                anyhow::anyhow!("Failed to bind to {}: {}", addr, e)
            }
        })?;

        // Start server with graceful shutdown on signal
        axum::serve(listener, router)
            .with_graceful_shutdown(shutdown_signal())
            .await?;

        Ok(())
    }

    /// Get the port.
    pub fn port(&self) -> u16 {
        self.port
    }
}

// =============================================================================
// Request/Response Types
// =============================================================================

/// Health check response.
#[derive(Serialize)]
struct HealthResponse {
    status: String,
    version: &'static str,
    ollama_status: String,
    cache_entries: usize,
    cache_hit_rate: f32,
}

/// Model information.
#[derive(Serialize)]
struct ModelInfo {
    id: String,
    object: &'static str,
    created: u64,
    owned_by: String,
}

/// Models list response.
#[derive(Serialize)]
struct ModelsResponse {
    object: &'static str,
    data: Vec<ModelInfo>,
}

/// Chat completion request.
#[derive(Deserialize)]
struct ChatCompletionRequest {
    model: String,
    messages: Vec<ChatMessage>,
    #[serde(default)]
    #[allow(dead_code)]
    temperature: Option<f32>,
    #[serde(default)]
    #[allow(dead_code)]
    max_tokens: Option<u32>,
    #[serde(default)]
    #[allow(dead_code)]
    stream: Option<bool>,
}

// Maximum query length to prevent DoS attacks
const MAX_QUERY_LENGTH: usize = 100_000; // 100KB
const MAX_MESSAGE_COUNT: usize = 100;
// Default timeout for Ollama calls (in seconds)
const OLLAMA_TIMEOUT_SECS: u64 = 120;
// Maximum request body size (1MB)
const MAX_BODY_SIZE: usize = 1024 * 1024;

/// Chat message.
#[derive(Deserialize, Serialize, Clone)]
struct ChatMessage {
    role: String,
    content: String,
}

/// Chat completion response.
#[derive(Serialize)]
struct ChatCompletionResponse {
    id: String,
    object: &'static str,
    created: u64,
    model: String,
    choices: Vec<ChatChoice>,
    usage: UsageInfo,
}

/// Chat completion choice.
#[derive(Serialize)]
struct ChatChoice {
    index: u32,
    message: ChatMessage,
    finish_reason: String,
}

/// Token usage information.
#[derive(Serialize)]
struct UsageInfo {
    prompt_tokens: u32,
    completion_tokens: u32,
    total_tokens: u32,
}

/// Stats response.
#[derive(Serialize)]
struct StatsResponse {
    session: SessionStatsInfo,
    today: TodayStats,
}

#[derive(Serialize)]
struct SessionStatsInfo {
    queries: u64,
    local_queries: u64,
    cloud_queries: u64,
    tokens_processed: u64,
}

#[derive(Serialize)]
struct TodayStats {
    queries: u64,
    saved_usd: f64,
    spent_usd: f64,
}

// =============================================================================
// Handlers
// =============================================================================

/// Health check handler.
///
/// Checks if Ollama is reachable and returns degraded status if not.
async fn health_handler(
    State(state): State<Arc<AppState>>,
) -> Json<HealthResponse> {
    // Check Ollama availability with a quick HTTP ping
    let ollama_status = match check_ollama_health().await {
        true => "ok".to_string(),
        false => "unavailable".to_string(),
    };

    // Get cache stats
    let (cache_entries, cache_hit_rate) = match state.cache.read() {
        Ok(cache) => (cache.len(), cache.stats().hit_rate()),
        Err(_) => (0, 0.0),
    };

    // Determine overall status
    let status = if ollama_status == "ok" {
        "ok".to_string()
    } else {
        "degraded".to_string()
    };

    Json(HealthResponse {
        status,
        version: env!("CARGO_PKG_VERSION"),
        ollama_status,
        cache_entries,
        cache_hit_rate,
    })
}

/// Check if Ollama is reachable with a quick HTTP ping.
async fn check_ollama_health() -> bool {
    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(2))
        .build()
        .unwrap_or_default();

    match client.get("http://localhost:11434/api/tags").send().await {
        Ok(response) => response.status().is_success(),
        Err(_) => false,
    }
}

/// List models handler.
async fn models_handler(
    State(_state): State<Arc<AppState>>,
) -> Json<ModelsResponse> {
    let models = vec![
        ModelInfo {
            id: "auto".to_string(),
            object: "model",
            created: 0,
            owned_by: "rigrun".to_string(),
        },
        ModelInfo {
            id: "local".to_string(),
            object: "model",
            created: 0,
            owned_by: "ollama".to_string(),
        },
        ModelInfo {
            id: "cloud".to_string(),
            object: "model",
            created: 0,
            owned_by: "openrouter".to_string(),
        },
        ModelInfo {
            id: "haiku".to_string(),
            object: "model",
            created: 0,
            owned_by: "anthropic".to_string(),
        },
        ModelInfo {
            id: "sonnet".to_string(),
            object: "model",
            created: 0,
            owned_by: "anthropic".to_string(),
        },
        ModelInfo {
            id: "opus".to_string(),
            object: "model",
            created: 0,
            owned_by: "anthropic".to_string(),
        },
    ];

    Json(ModelsResponse {
        object: "list",
        data: models,
    })
}

/// Chat completions handler.
async fn completions_handler(
    State(state): State<Arc<AppState>>,
    Json(request): Json<ChatCompletionRequest>,
) -> Result<Json<ChatCompletionResponse>, (StatusCode, String)> {
    let start_time = Instant::now();

    // Input validation to prevent DoS attacks
    if request.messages.is_empty() {
        return Err((StatusCode::BAD_REQUEST, "Request must contain at least one message".to_string()));
    }

    if request.messages.len() > MAX_MESSAGE_COUNT {
        return Err((StatusCode::BAD_REQUEST, format!("Too many messages (max: {})", MAX_MESSAGE_COUNT)));
    }

    // Validate message lengths
    for (idx, msg) in request.messages.iter().enumerate() {
        if msg.content.len() > MAX_QUERY_LENGTH {
            return Err((StatusCode::BAD_REQUEST,
                format!("Message {} exceeds maximum length of {} characters", idx, MAX_QUERY_LENGTH)));
        }
        if msg.content.trim().is_empty() {
            return Err((StatusCode::BAD_REQUEST,
                format!("Message {} has empty content", idx)));
        }
    }

    // Extract the last user message for cache key (semantic matching)
    let cache_key = request.messages.last()
        .map(|m| m.content.as_str())
        .unwrap_or("");

    // Determine which tier to use based on the requested model
    let tier = match request.model.as_str() {
        "auto" => {
            // Use router to determine tier based on query complexity
            route_query(cache_key, None)
        }
        "local" => Tier::Local,
        "cache" => Tier::Cache,
        "cloud" => Tier::Cloud,
        "haiku" => Tier::Haiku,
        "sonnet" => Tier::Sonnet,
        "opus" => Tier::Opus,
        _ => Tier::Local, // Default to local for unknown models
    };

    // Check cache first for Cache tier or auto-routed Cache tier
    if tier == Tier::Cache || request.model == "auto" {
        // Gracefully handle poisoned lock - recover and continue rather than crashing
        let cache_result = match state.cache.write() {
            Ok(mut cache) => cache.check_and_record_hit(cache_key),
            Err(poisoned) => {
                tracing::warn!("Cache lock was poisoned, recovering and continuing");
                // Recover by getting the inner data despite the poison
                let mut cache = poisoned.into_inner();
                cache.check_and_record_hit(cache_key)
            }
        };

        if let Some(cached) = cache_result {
            let latency_ms = start_time.elapsed().as_millis() as u64;
            tracing::info!("Cache hit for query (hit_count: {}, age: {:.1}h)",
                cached.hit_count, cached.age_hours());

            // Record cache hit to stats tracker
            let query_stats = stats::QueryStats::new(
                stats::Tier::Cache,
                0, // No tokens for cache hit
                0,
                latency_ms,
            );
            tracing::info!("Recording cache hit, latency={}ms", latency_ms);
            stats::record_query(query_stats);

            // Persist stats to disk after recording
            if let Err(e) = stats::persist_stats() {
                tracing::warn!("Failed to persist stats: {}", e);
            } else {
                let session_stats = stats::get_session_stats();
                tracing::debug!(
                    "Stats persisted. Session: {} queries (cache hits: {})",
                    session_stats.total_queries,
                    session_stats.cache_hits
                );
            }

            return Ok(Json(ChatCompletionResponse {
                id: format!("chatcmpl-{}", uuid_v4()),
                object: "chat.completion",
                created: std::time::SystemTime::now()
                    .duration_since(std::time::UNIX_EPOCH)
                    .unwrap_or_default()
                    .as_secs(),
                model: "cache".to_string(),
                choices: vec![ChatChoice {
                    index: 0,
                    message: ChatMessage {
                        role: "assistant".to_string(),
                        content: cached.response,
                    },
                    finish_reason: "stop".to_string(),
                }],
                usage: UsageInfo {
                    prompt_tokens: 0,
                    completion_tokens: 0,
                    total_tokens: 0,
                },
            }));
        }

        // If explicitly requested cache tier but no hit, fall back to Local
        if tier == Tier::Cache {
            tracing::debug!("Cache miss, falling back to Local tier");
        }
    }

    // Convert request messages to the format expected by OllamaClient
    let messages: Vec<Message> = request
        .messages
        .iter()
        .map(|msg| Message::new(msg.role.clone(), msg.content.clone()))
        .collect();

    // Execute on appropriate tier
    let (response_text, prompt_tokens, completion_tokens, actual_tier) = match tier {
        Tier::Cache | Tier::Local => {
            // Call Ollama for inference with timeout to prevent hanging
            let ollama_future = state
                .ollama_client
                .chat_async(&state.local_model, messages);

            let ollama_response = tokio::time::timeout(
                std::time::Duration::from_secs(OLLAMA_TIMEOUT_SECS),
                ollama_future,
            )
            .await
            .map_err(|_| {
                tracing::error!("Ollama request timed out after {}s", OLLAMA_TIMEOUT_SECS);
                (
                    StatusCode::GATEWAY_TIMEOUT,
                    format!("Ollama request timed out after {} seconds", OLLAMA_TIMEOUT_SECS),
                )
            })?
            .map_err(|e| {
                tracing::error!("Ollama error: {}", e);
                (
                    StatusCode::INTERNAL_SERVER_ERROR,
                    format!("Ollama error: {}", e),
                )
            })?;

            (
                ollama_response.response,
                ollama_response.prompt_tokens,
                ollama_response.completion_tokens,
                Tier::Local,
            )
        }
        Tier::Cloud => {
            // Use OpenRouter auto-router - it picks the best model automatically
            let model = "openrouter/auto";

            // Convert messages to OpenRouter format
            let cloud_messages: Vec<CloudMessage> = request
                .messages
                .iter()
                .map(|msg| CloudMessage::new(msg.role.clone(), msg.content.clone()))
                .collect();

            // Call OpenRouter for cloud inference with auto-routing
            let openrouter_response = state
                .openrouter_client
                .chat(model, cloud_messages)
                .await
                .map_err(|e| {
                    tracing::error!("OpenRouter error: {}", e);
                    (
                        StatusCode::INTERNAL_SERVER_ERROR,
                        format!("OpenRouter error: {}", e),
                    )
                })?;

            (
                openrouter_response.response,
                openrouter_response.prompt_tokens,
                openrouter_response.completion_tokens,
                Tier::Cloud,
            )
        }
        Tier::Haiku | Tier::Sonnet | Tier::Opus => {
            // Map tier to OpenRouter model name (explicit model selection)
            let model = match tier {
                Tier::Haiku => "anthropic/claude-3-haiku",
                Tier::Sonnet => "anthropic/claude-3.5-sonnet",
                Tier::Opus => "anthropic/claude-3-opus",
                _ => unreachable!(),
            };

            // Convert messages to OpenRouter format
            let cloud_messages: Vec<CloudMessage> = request
                .messages
                .iter()
                .map(|msg| CloudMessage::new(msg.role.clone(), msg.content.clone()))
                .collect();

            // Call OpenRouter for cloud inference
            let openrouter_response = state
                .openrouter_client
                .chat(model, cloud_messages)
                .await
                .map_err(|e| {
                    tracing::error!("OpenRouter error: {}", e);
                    (
                        StatusCode::INTERNAL_SERVER_ERROR,
                        format!("OpenRouter error: {}", e),
                    )
                })?;

            (
                openrouter_response.response,
                openrouter_response.prompt_tokens,
                openrouter_response.completion_tokens,
                tier,
            )
        }
    };

    // Store response in cache for future hits
    // Gracefully handle poisoned lock - skip caching rather than crashing
    match state.cache.write() {
        Ok(mut cache) => {
            cache.store(
                cache_key,
                response_text.clone(),
                actual_tier,
                prompt_tokens + completion_tokens,
            );
            tracing::debug!("Stored response in cache (entries: {})", cache.len());
        }
        Err(poisoned) => {
            tracing::warn!("Cache lock was poisoned, recovering and storing response");
            let mut cache = poisoned.into_inner();
            cache.store(
                cache_key,
                response_text.clone(),
                actual_tier,
                prompt_tokens + completion_tokens,
            );
            tracing::debug!("Stored response in cache after recovery (entries: {})", cache.len());
        }
    }

    let total_tokens = prompt_tokens + completion_tokens;
    let latency_ms = start_time.elapsed().as_millis() as u64;

    // Record query to stats tracker
    let stats_tier = map_tier_to_stats(actual_tier);
    let query_stats = stats::QueryStats::new(
        stats_tier,
        prompt_tokens,
        completion_tokens,
        latency_ms,
    );
    tracing::info!(
        "Recording query: tier={:?}, tokens={}, latency={}ms",
        stats_tier,
        prompt_tokens + completion_tokens,
        latency_ms
    );
    stats::record_query(query_stats);

    // Persist stats to disk after recording
    if let Err(e) = stats::persist_stats() {
        tracing::warn!("Failed to persist stats: {}", e);
    } else {
        let session_stats = stats::get_session_stats();
        tracing::debug!(
            "Stats persisted. Session: {} queries, All-time: {} queries",
            session_stats.total_queries,
            stats::global_tracker().get_all_time_stats().total_queries
        );
    }

    let response = ChatCompletionResponse {
        id: format!("chatcmpl-{}", uuid_v4()),
        object: "chat.completion",
        created: std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs(),
        model: request.model,
        choices: vec![ChatChoice {
            index: 0,
            message: ChatMessage {
                role: "assistant".to_string(),
                content: response_text,
            },
            finish_reason: "stop".to_string(),
        }],
        usage: UsageInfo {
            prompt_tokens,
            completion_tokens,
            total_tokens,
        },
    };

    Ok(Json(response))
}

/// Stats handler.
async fn stats_handler() -> Json<StatsResponse> {
    // Get session stats (current session, in-memory)
    let session = stats::get_session_stats();

    // Get all-time stats from the global tracker (includes persisted + current session)
    let all_time = stats::global_tracker().get_all_time_stats();

    // Calculate today's savings
    let today_saved = all_time.today_savings();

    // Get today's queries and spending from daily_savings
    let today = chrono::Utc::now().format("%Y-%m-%d").to_string();
    let today_data = all_time.daily_savings
        .iter()
        .find(|d| d.date == today);

    let (today_queries, today_spent) = today_data
        .map(|d| (d.queries as u64, d.spent))
        .unwrap_or((0, 0.0));

    Json(StatsResponse {
        session: SessionStatsInfo {
            queries: session.total_queries as u64,
            local_queries: (session.local_queries + session.cache_hits) as u64,
            cloud_queries: session.cloud_queries as u64,
            tokens_processed: session.total_tokens,
        },
        today: TodayStats {
            queries: today_queries,
            saved_usd: today_saved,
            spent_usd: today_spent,
        },
    })
}

/// Cache statistics response.
#[derive(Serialize)]
struct CacheStatsResponse {
    entries: usize,
    total_lookups: u64,
    hits: u64,
    misses: u64,
    expired_skips: u64,
    total_stores: u64,
    hit_rate_percent: f32,
    ttl_hours: u32,
}

/// Cache stats handler.
async fn cache_stats_handler(
    State(state): State<Arc<AppState>>,
) -> Json<CacheStatsResponse> {
    // Gracefully handle poisoned lock - return default stats rather than error
    let (entries, total_lookups, hits, misses, expired_skips, total_stores, hit_rate, ttl_hours) = match state.cache.read() {
        Ok(cache) => {
            let stats = cache.stats();
            (cache.len(), stats.total_lookups, stats.hits, stats.misses, stats.expired_skips, stats.total_stores, stats.hit_rate(), cache.ttl_hours())
        }
        Err(poisoned) => {
            tracing::warn!("Cache lock was poisoned, recovering to read stats");
            let cache = poisoned.into_inner();
            let stats = cache.stats();
            (cache.len(), stats.total_lookups, stats.hits, stats.misses, stats.expired_skips, stats.total_stores, stats.hit_rate(), cache.ttl_hours())
        }
    };

    Json(CacheStatsResponse {
        entries,
        total_lookups,
        hits,
        misses,
        expired_skips,
        total_stores,
        hit_rate_percent: hit_rate,
        ttl_hours,
    })
}

// =============================================================================
// Utilities
// =============================================================================

/// Map router::Tier to stats::Tier for stats tracking.
fn map_tier_to_stats(tier: Tier) -> stats::Tier {
    match tier {
        Tier::Cache => stats::Tier::Cache,
        Tier::Local => stats::Tier::Local,
        Tier::Cloud => stats::Tier::Cloud,
        Tier::Haiku => stats::Tier::Haiku,
        Tier::Sonnet => stats::Tier::Sonnet,
        Tier::Opus => stats::Tier::Opus,
    }
}

/// Generate a proper random UUID v4 for response IDs.
fn uuid_v4() -> String {
    use rand::Rng;

    let mut rng = rand::thread_rng();

    // Generate 16 random bytes
    let mut bytes = [0u8; 16];
    rng.fill(&mut bytes);

    // Set version (4) and variant (RFC 4122) bits
    bytes[6] = (bytes[6] & 0x0f) | 0x40; // Version 4
    bytes[8] = (bytes[8] & 0x3f) | 0x80; // Variant RFC 4122

    // Format as UUID string (without hyphens for compact response IDs)
    format!(
        "{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}",
        bytes[0], bytes[1], bytes[2], bytes[3],
        bytes[4], bytes[5], bytes[6], bytes[7],
        bytes[8], bytes[9], bytes[10], bytes[11],
        bytes[12], bytes[13], bytes[14], bytes[15]
    )
}

/// Graceful shutdown signal handler.
///
/// Waits for SIGINT/SIGTERM, then persists stats before allowing the server to shut down.
async fn shutdown_signal() {
    // On Unix, listen for SIGINT and SIGTERM
    // On Windows, fall back to Ctrl+C only
    #[cfg(unix)]
    {
        use tokio::signal::unix::{signal, SignalKind};

        let mut sigterm = signal(SignalKind::terminate())
            .expect("failed to install SIGTERM handler");
        let mut sigint = signal(SignalKind::interrupt())
            .expect("failed to install SIGINT handler");

        tokio::select! {
            _ = sigterm.recv() => {
                tracing::info!("Received SIGTERM, initiating graceful shutdown...");
            }
            _ = sigint.recv() => {
                tracing::info!("Received SIGINT (Ctrl+C), initiating graceful shutdown...");
            }
        }
    }

    #[cfg(not(unix))]
    {
        // Fallback: just handle Ctrl+C on non-Unix platforms (Windows)
        tokio::signal::ctrl_c()
            .await
            .expect("failed to install Ctrl+C handler");
        tracing::info!("Received Ctrl+C, initiating graceful shutdown...");
    }

    // Persist stats before shutdown
    if let Err(e) = stats::persist_stats() {
        tracing::error!("Failed to persist stats during shutdown: {}", e);
    } else {
        tracing::info!("Stats persisted successfully");
    }

    tracing::info!("Cleanup complete, shutting down server");
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_server_creation() {
        let server = Server::new(3000);
        assert_eq!(server.port(), 3000);
    }

    #[test]
    fn test_server_default() {
        let server = Server::default();
        assert_eq!(server.port(), 8787);
    }

    #[test]
    fn test_server_with_model() {
        let server = Server::new(8080).with_default_model("qwen2.5-coder:7b");
        assert_eq!(server.default_model, "qwen2.5-coder:7b");
    }

    #[test]
    fn test_uuid_generation() {
        let id1 = uuid_v4();
        let id2 = uuid_v4();
        // UUIDs should be different (or at least not always the same)
        assert_eq!(id1.len(), 32);
        assert_eq!(id2.len(), 32);
    }
}
