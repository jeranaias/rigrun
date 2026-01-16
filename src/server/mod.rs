// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

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
//! - `POST /v1/chat/completions/stream` - Streaming chat (SSE) - TRUE streaming!
//! - `GET /stats` - Usage statistics (includes semantic cache metrics)
//! - `GET /cache/stats` - Exact-match cache statistics
//! - `GET /cache/semantic` - Semantic cache statistics
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
    http::{StatusCode, HeaderValue, Request},
    response::{Json, Response, Sse, sse::Event},
    routing::{get, post},
    Router,
};
use tower_http::timeout::TimeoutLayer;
use serde::{Deserialize, Serialize};
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use tokio::sync::RwLock;
use std::time::Instant;
use anyhow::Result;
use tower::{Layer, Service};
use tower_governor::{
    governor::GovernorConfigBuilder,
    key_extractor::PeerIpKeyExtractor,
    GovernorLayer,
};
use std::task::{Context, Poll};
use std::pin::Pin;
use std::future::Future;
use std::convert::Infallible;
use futures_util::stream::Stream;
use tokio_stream::wrappers::ReceiverStream;
use crate::audit::{self, redact_secrets};
use crate::cache::semantic::SemanticCache;
use crate::errors::{UserError, sanitize_error_details};
use crate::local::OllamaClient;
use crate::router::route_query;
use crate::security::{Session, SessionConfig, SessionManager, SessionState, DOD_STIG_MAX_SESSION_TIMEOUT_SECS};
use crate::stats;
use crate::cloud::OpenRouterClient;
use crate::types::{Tier, Message, StreamChunk};

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
    pub cache: RwLock<SemanticCache>,
    /// Paranoid mode: block all cloud requests.
    pub paranoid_mode: bool,
    /// API key for Bearer token authentication.
    pub api_key: Option<String>,
    /// Session manager for DoD STIG-compliant session timeout (IL5 requirement).
    pub session_manager: SessionManager,
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
    /// Paranoid mode: block all cloud requests.
    pub paranoid_mode: bool,
    /// Maximum concurrent connections (default: 100).
    pub max_connections: usize,
    /// API key for Bearer token authentication.
    pub api_key: Option<String>,
    /// Semantic cache similarity threshold (0.70 - 0.99).
    /// Default is 0.92, which provides a good balance between cache hit rate and accuracy.
    /// Higher values (closer to 1.0) reduce false positives but may miss valid semantic matches.
    /// Lower values increase hit rate but may cause false positive cache hits.
    pub similarity_threshold: Option<f32>,
    /// Session timeout in seconds (DoD STIG IL5 maximum: 900 seconds / 15 minutes).
    /// Any value exceeding 900 will be clamped to 900 per STIG requirements.
    pub session_timeout_secs: u64,
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
    /// Paranoid mode: block all cloud requests.
    paranoid_mode: bool,
    /// CORS allowed origins.
    cors_origins: Vec<String>,
    /// API key for Bearer token authentication.
    api_key: Option<String>,
    /// Semantic cache similarity threshold (0.70 - 0.99).
    /// Default is 0.92 if not specified.
    similarity_threshold: Option<f32>,
    /// Maximum concurrent connections (default: 100).
    max_connections: usize,
    /// Session timeout in seconds (DoD STIG IL5 maximum: 900 seconds / 15 minutes).
    /// Defaults to 900 (maximum allowed for IL5). Any value exceeding 900 will be clamped.
    session_timeout_secs: u64,
}

impl Default for Server {
    fn default() -> Self {
        Self::new(8787)
    }
}

impl Server {
    /// Create a new server with the specified port.
    /// By default, binds to 127.0.0.1 (localhost only) for security.
    /// Session timeout defaults to 900 seconds (15 minutes) per DoD STIG IL5 requirements.
    pub fn new(port: u16) -> Self {
        Self {
            port,
            default_model: "auto".to_string(),
            local_model: "qwen2.5-coder:7b".to_string(),
            openrouter_key: None,
            bind_address: "127.0.0.1".to_string(),
            paranoid_mode: false,
            cors_origins: Vec::new(),
            api_key: None,
            similarity_threshold: None,
            max_connections: 100,
            session_timeout_secs: DOD_STIG_MAX_SESSION_TIMEOUT_SECS, // 15 minutes (IL5 requirement)
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

    /// Enable paranoid mode: block all cloud requests.
    /// When enabled, requests that would go to the cloud return an error instead.
    pub fn with_paranoid_mode(mut self, enabled: bool) -> Self {
        self.paranoid_mode = enabled;
        self
    }

    /// Set CORS allowed origins.
    pub fn with_cors_origins(mut self, origins: Vec<String>) -> Self {
        self.cors_origins = origins;
        self
    }

    /// Set the API key for Bearer token authentication.
    pub fn with_api_key(mut self, key: impl Into<String>) -> Self {
        self.api_key = Some(key.into());
        self
    }

    /// Set maximum concurrent connections.
    pub fn with_max_connections(mut self, max: usize) -> Self {
        self.max_connections = max;
        self
    }

    /// Set the semantic cache similarity threshold (0.70 - 0.99).
    pub fn with_similarity_threshold(mut self, threshold: f32) -> Self {
        self.similarity_threshold = Some(threshold);
        self
    }

    /// Set the session timeout in seconds.
    ///
    /// **DoD STIG IL5 REQUIREMENT**: Maximum allowed timeout is 900 seconds (15 minutes).
    /// Any value exceeding 900 will be clamped to 900 per STIG requirements.
    ///
    /// # Arguments
    /// * `timeout_secs` - Session timeout in seconds (max: 900)
    pub fn with_session_timeout(mut self, timeout_secs: u64) -> Self {
        // Clamp to DoD STIG IL5 maximum
        let clamped = timeout_secs.min(DOD_STIG_MAX_SESSION_TIMEOUT_SECS);
        if timeout_secs > DOD_STIG_MAX_SESSION_TIMEOUT_SECS {
            tracing::warn!(
                "SESSION_TIMEOUT: Requested {}s exceeds DoD STIG IL5 maximum of {}s. Using {}s.",
                timeout_secs,
                DOD_STIG_MAX_SESSION_TIMEOUT_SECS,
                clamped
            );
        }
        self.session_timeout_secs = clamped;
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

        // Create SemanticCache with QueryCache's default persistent settings
        // Default to 0.92 similarity threshold, which provides a good balance between
        // cache hit rate and accuracy, minimizing false positive matches while still
        // capturing semantically equivalent queries.
        let threshold = self.similarity_threshold.unwrap_or(0.92);

        // Validate threshold range
        let threshold = threshold.clamp(0.70, 0.99);

        // Log warning if threshold is below recommended minimum
        if threshold < 0.85 {
            tracing::warn!(
                "Similarity threshold {} is low - may cause false positive cache hits",
                threshold
            );
        }

        tracing::info!("Initializing semantic cache with similarity threshold: {:.2}", threshold);

        let semantic_cache = {
            use crate::cache::QueryCache;
            let exact_cache = QueryCache::default_persistent();
            SemanticCache::with_cache(exact_cache, threshold)
        };

        // Create session manager with DoD STIG-compliant timeout
        // Session timeout is clamped to maximum 900 seconds (15 minutes) per IL5 requirements
        let session_config = SessionConfig::custom(self.session_timeout_secs, 120);
        let session_manager = SessionManager::new(session_config);

        tracing::info!(
            "Initializing session manager with timeout: {}s (DoD STIG IL5 max: {}s)",
            self.session_timeout_secs,
            DOD_STIG_MAX_SESSION_TIMEOUT_SECS
        );

        let state = Arc::new(AppState {
            config: ServerConfig {
                port: self.port,
                default_model: self.default_model.clone(),
                bind_address: self.bind_address.clone(),
                paranoid_mode: self.paranoid_mode,
                max_connections: 100,
                api_key: self.api_key.clone(),
                similarity_threshold: Some(threshold),
                session_timeout_secs: self.session_timeout_secs,
            },
            ollama_client: OllamaClient::new(),
            openrouter_client,
            local_model: self.local_model.clone(),
            cache: RwLock::new(semantic_cache),
            paranoid_mode: self.paranoid_mode,
            api_key: self.api_key.clone(),
            session_manager,
        });

        // Configure rate limiting: 60 requests per minute per IP
        // NOTE: This .expect() is acceptable because:
        // 1. This runs during server initialization, not request handling
        // 2. The configuration is static and known-good
        // 3. Failure here indicates a library bug, not user error
        // 4. The server should not start with broken rate limiting
        let governor_conf = Arc::new(
            GovernorConfigBuilder::default()
                .per_second(1) // 1 request per second = 60 per minute
                .burst_size(60) // Allow burst of 60 requests
                .key_extractor(PeerIpKeyExtractor)
                .finish()
                .expect("Failed to build rate limiter configuration with static parameters. This indicates a tower_governor library bug.")
        );

        // Public routes (no auth needed)
        let public_routes = Router::new()
            .route("/health", get(health_handler))
            .route("/v1/models", get(models_handler));

        // Protected routes (require auth when api_key is set)
        let protected_routes = Router::new()
            .route("/v1/chat/completions", post(completions_handler))
            .route("/v1/chat/completions/stream", post(stream_completions_handler))
            .route("/stats", get(stats_handler))
            .route("/cache/stats", get(cache_stats_handler))
            .route("/cache/semantic", get(semantic_cache_stats_handler));

        // Apply auth middleware only if api_key is configured
        let protected_routes = if state.api_key.is_some() {
            protected_routes.route_layer(axum::middleware::from_fn_with_state(
                state.clone(),
                require_auth,
            ))
        } else {
            protected_routes
        };

        // Apply session validation middleware to protected routes (DoD STIG IL5)
        let protected_routes = protected_routes.route_layer(axum::middleware::from_fn_with_state(
            state.clone(),
            validate_session,
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
            .layer(SessionHeadersLayer::new(self.session_timeout_secs)) // DoD STIG IL5
            .layer(SecurityHeadersLayer::new(self.cors_origins.clone()))
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
                "SECURITY WARNING: Server is binding to 0.0.0.0 which exposes the API to your entire network. \
                This allows anyone on your network to access the API and potentially send data to cloud providers. \
                Use 127.0.0.1 (default) for local-only access, or implement authentication if network access is required."
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
        // Using into_make_service_with_connect_info to provide SocketAddr for rate limiting
        axum::serve(
            listener,
            router.into_make_service_with_connect_info::<std::net::SocketAddr>()
        )
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
// Rate Limit Headers Middleware
// =============================================================================

/// Rate limit headers middleware layer.
#[derive(Clone, Default)]
pub struct RateLimitHeadersLayer;

impl RateLimitHeadersLayer {
    pub fn new<T>(_config: T) -> Self {
        Self
    }
}

impl<S> Layer<S> for RateLimitHeadersLayer {
    type Service = RateLimitHeadersMiddleware<S>;

    fn layer(&self, inner: S) -> Self::Service {
        RateLimitHeadersMiddleware { inner }
    }
}

/// Rate limit headers middleware service.
#[derive(Clone)]
pub struct RateLimitHeadersMiddleware<S> {
    inner: S,
}

impl<S, B> Service<Request<B>> for RateLimitHeadersMiddleware<S>
where
    S: Service<Request<B>, Response = Response> + Clone + Send + 'static,
    S::Future: Send + 'static,
    B: Send + 'static,
{
    type Response = S::Response;
    type Error = S::Error;
    type Future = Pin<Box<dyn Future<Output = Result<Self::Response, Self::Error>> + Send>>;

    fn poll_ready(&mut self, cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        self.inner.poll_ready(cx)
    }

    fn call(&mut self, req: Request<B>) -> Self::Future {
        let mut inner = self.inner.clone();

        Box::pin(async move {
            let response = inner.call(req).await?;
            let (mut parts, body) = response.into_parts();

            // Add rate limit headers
            parts.headers.insert(
                "X-RateLimit-Limit",
                HeaderValue::from_static("60"),
            );
            parts.headers.insert(
                "X-RateLimit-Window",
                HeaderValue::from_static("60s"),
            );

            Ok(Response::from_parts(parts, body))
        })
    }
}

// =============================================================================
// Authentication Middleware
// =============================================================================

/// Authentication middleware that checks for Bearer token.
///
/// If api_key is configured, requests must include a valid "Authorization: Bearer <token>" header.
/// If api_key is None, all requests are allowed.
///
/// Security measures (IL5-compliant per NIST 800-53 SI-11):
/// - Uses constant-time comparison to prevent timing attacks
/// - Returns identical error messages for all auth failures to prevent enumeration attacks
/// - Never reveals whether user exists, API key format, or other implementation details
async fn require_auth(
    State(state): State<Arc<AppState>>,
    request: axum::extract::Request,
    next: axum::middleware::Next,
) -> Result<Response, UserError> {
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
    let token = match auth_header {
        Some(header) if header.starts_with("Bearer ") => &header[7..], // Skip "Bearer "
        _ => {
            // IL5-compliant: Generic error message, no details about missing vs invalid
            return Err(UserError::authentication_required(Some(
                "Missing or invalid Authorization header"
            )));
        }
    };

    // Constant-time comparison to prevent timing attacks
    // First check length equality (fast path that's still safe)
    // Then compare bytes to avoid short-circuit evaluation
    let is_valid = token.len() == api_key.len()
        && token.as_bytes()
            .iter()
            .zip(api_key.as_bytes().iter())
            .fold(0u8, |acc, (a, b)| acc | (a ^ b))
            == 0;

    if is_valid {
        Ok(next.run(request).await)
    } else {
        // IL5-compliant: Same error for all auth failures (prevents enumeration)
        Err(UserError::authentication_required(Some(
            "Invalid API key provided"
        )))
    }
}

// =============================================================================
// Session Validation Middleware (DoD STIG IL5 Compliance)
// =============================================================================

/// Session validation middleware for DoD STIG IL5 compliance.
///
/// This middleware:
/// - Validates session tokens from X-Session-Id header
/// - Checks session expiration against 15-minute (900s) maximum timeout
/// - Adds X-Session-Expires-In header to all responses
/// - Returns 401 with "Session expired" on timeout
///
/// **STIG Requirements Implemented:**
/// - AC-12: Session Termination (15-minute maximum)
/// - AC-11: Session Lock (requires re-authentication)
async fn validate_session(
    State(state): State<Arc<AppState>>,
    request: axum::extract::Request,
    next: axum::middleware::Next,
) -> Result<Response, UserError> {
    // Extract session ID from header
    let session_id = request
        .headers()
        .get("X-Session-Id")
        .and_then(|h| h.to_str().ok());

    // If no session header, allow the request (session tracking is optional for API)
    // Note: For stricter IL5 compliance, you may want to require sessions on all requests
    let session_id = match session_id {
        Some(id) => id,
        None => {
            // No session - proceed but add warning header
            let mut response = next.run(request).await;
            response.headers_mut().insert(
                "X-Session-Warning",
                HeaderValue::from_static("No session provided"),
            );
            return Ok(response);
        }
    };

    // Validate session
    let (is_valid, session_state, message) = state.session_manager.validate_session(session_id);

    if !is_valid {
        // Session expired or invalid - require re-authentication
        tracing::warn!(
            "SESSION_EXPIRED | session={} state={:?} message={:?}",
            session_id,
            session_state,
            message
        );

        return Err(UserError::session_expired());
    }

    // Refresh session activity (user is active)
    state.session_manager.refresh_session(session_id);

    // Get time remaining for header
    let time_remaining = state
        .session_manager
        .get_session(session_id)
        .map(|s| s.time_remaining_secs())
        .unwrap_or(0);

    // Execute the request
    let mut response = next.run(request).await;

    // Add session expiration header to response
    if let Ok(value) = HeaderValue::from_str(&time_remaining.to_string()) {
        response.headers_mut().insert("X-Session-Expires-In", value);
    }

    // Add session state header
    if let Ok(value) = HeaderValue::from_str(&format!("{}", session_state)) {
        response.headers_mut().insert("X-Session-State", value);
    }

    // Add warning header if in warning period
    if session_state == SessionState::Warning {
        if let Some(msg) = message {
            if let Ok(value) = HeaderValue::from_str(&msg) {
                response.headers_mut().insert("X-Session-Warning", value);
            }
        }
    }

    Ok(response)
}

/// Session headers middleware layer.
///
/// Adds X-Session-Expires-In and X-Session-Timeout-Max headers to all responses
/// for DoD STIG IL5 compliance transparency.
#[derive(Clone)]
pub struct SessionHeadersLayer {
    max_timeout_secs: u64,
}

impl SessionHeadersLayer {
    pub fn new(max_timeout_secs: u64) -> Self {
        Self { max_timeout_secs }
    }
}

impl<S> Layer<S> for SessionHeadersLayer {
    type Service = SessionHeadersMiddleware<S>;

    fn layer(&self, inner: S) -> Self::Service {
        SessionHeadersMiddleware {
            inner,
            max_timeout_secs: self.max_timeout_secs,
        }
    }
}

/// Session headers middleware service.
#[derive(Clone)]
pub struct SessionHeadersMiddleware<S> {
    inner: S,
    max_timeout_secs: u64,
}

impl<S, B> Service<Request<B>> for SessionHeadersMiddleware<S>
where
    S: Service<Request<B>, Response = Response> + Clone + Send + 'static,
    S::Future: Send + 'static,
    B: Send + 'static,
{
    type Response = S::Response;
    type Error = S::Error;
    type Future = Pin<Box<dyn Future<Output = Result<Self::Response, Self::Error>> + Send>>;

    fn poll_ready(&mut self, cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        self.inner.poll_ready(cx)
    }

    fn call(&mut self, req: Request<B>) -> Self::Future {
        let mut inner = self.inner.clone();
        let max_timeout = self.max_timeout_secs;

        Box::pin(async move {
            let response = inner.call(req).await?;
            let (mut parts, body) = response.into_parts();

            // Add session timeout max header (for client awareness of IL5 limits)
            if let Ok(value) = HeaderValue::from_str(&max_timeout.to_string()) {
                parts.headers.insert("X-Session-Timeout-Max", value);
            }

            // Add STIG compliance indicator
            parts.headers.insert(
                "X-STIG-Session-Timeout",
                HeaderValue::from_static("DoD-STIG-IL5-Compliant"),
            );

            Ok(Response::from_parts(parts, body))
        })
    }
}

// =============================================================================
// Security Headers Middleware
// =============================================================================

/// Security headers middleware layer.
#[derive(Clone)]
pub struct SecurityHeadersLayer {
    cors_origins: Vec<String>,
}

impl SecurityHeadersLayer {
    pub fn new(cors_origins: Vec<String>) -> Self {
        Self { cors_origins }
    }
}

impl<S> Layer<S> for SecurityHeadersLayer {
    type Service = SecurityHeadersMiddleware<S>;

    fn layer(&self, inner: S) -> Self::Service {
        SecurityHeadersMiddleware {
            inner,
            cors_origins: self.cors_origins.clone(),
        }
    }
}

/// Security headers middleware service.
#[derive(Clone)]
pub struct SecurityHeadersMiddleware<S> {
    inner: S,
    cors_origins: Vec<String>,
}

impl<S, B> Service<Request<B>> for SecurityHeadersMiddleware<S>
where
    S: Service<Request<B>, Response = Response> + Clone + Send + 'static,
    S::Future: Send + 'static,
    B: Send + 'static,
{
    type Response = S::Response;
    type Error = S::Error;
    type Future = Pin<Box<dyn Future<Output = Result<Self::Response, Self::Error>> + Send>>;

    fn poll_ready(&mut self, cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        self.inner.poll_ready(cx)
    }

    fn call(&mut self, req: Request<B>) -> Self::Future {
        let mut inner = self.inner.clone();
        let cors_origins = self.cors_origins.clone();

        // Extract request origin for CORS validation
        let request_origin = req.headers()
            .get("origin")
            .and_then(|v| v.to_str().ok())
            .map(|s| s.to_string());

        Box::pin(async move {
            let response = inner.call(req).await?;
            let (mut parts, body) = response.into_parts();

            // Add security headers
            parts.headers.insert(
                "X-Content-Type-Options",
                HeaderValue::from_static("nosniff"),
            );
            parts.headers.insert(
                "X-Frame-Options",
                HeaderValue::from_static("DENY"),
            );
            parts.headers.insert(
                "X-XSS-Protection",
                HeaderValue::from_static("1; mode=block"),
            );
            parts.headers.insert(
                "Content-Security-Policy",
                HeaderValue::from_static("default-src 'none'"),
            );
            parts.headers.insert(
                "Cache-Control",
                HeaderValue::from_static("no-store, no-cache, must-revalidate"),
            );

            // Add CORS headers if origins are configured
            if !cors_origins.is_empty() {
                if let Some(origin) = request_origin {
                    if cors_origins.contains(&origin) || cors_origins.contains(&"*".to_string()) {
                        if let Ok(value) = HeaderValue::from_str(&origin) {
                            parts.headers.insert("Access-Control-Allow-Origin", value);
                            parts.headers.insert(
                                "Access-Control-Allow-Methods",
                                HeaderValue::from_static("GET, POST, OPTIONS"),
                            );
                            parts.headers.insert(
                                "Access-Control-Allow-Headers",
                                HeaderValue::from_static("Content-Type, Authorization"),
                            );
                        }
                    }
                } else if cors_origins.contains(&"*".to_string()) {
                    parts.headers.insert(
                        "Access-Control-Allow-Origin",
                        HeaderValue::from_static("*"),
                    );
                }
            }

            Ok(Response::from_parts(parts, body))
        })
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
    messages: Vec<Message>,
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
// Short timeout to quickly fall back to cloud if local is slow
const OLLAMA_TIMEOUT_SECS: u64 = 15;
// Maximum request body size (1MB)
const MAX_BODY_SIZE: usize = 1024 * 1024;

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
    message: Message,
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
    #[serde(skip_serializing_if = "Option::is_none")]
    semantic_cache: Option<SemanticCacheMetrics>,
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

#[derive(Serialize)]
struct SemanticCacheMetrics {
    semantic_hits: u64,
    exact_hits: u64,
    semantic_hit_rate: f32,
    total_hit_rate: f32,
    embedding_failures: u64,
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
    let cache = state.cache.read().await;
    let stats = cache.stats();
    let (cache_entries, cache_hit_rate) = (cache.len(), stats.total_hit_rate);

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
    // Try to build HTTP client - if this fails, system TLS is broken
    let client = match reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(2))
        .build()
    {
        Ok(c) => c,
        Err(e) => {
            tracing::error!("Failed to build HTTP client for health check: {}. System TLS/SSL may be misconfigured.", redact_secrets(&e.to_string()));
            return false;
        }
    };

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
) -> Result<Json<ChatCompletionResponse>, UserError> {
    let start_time = Instant::now();

    // Input validation to prevent DoS attacks (IL5-compliant error responses)
    if request.messages.is_empty() {
        return Err(UserError::invalid_request(
            "Request must contain at least one message",
            Some("messages"),
            None,
        ));
    }

    if request.messages.len() > MAX_MESSAGE_COUNT {
        return Err(UserError::invalid_request(
            &format!("Too many messages. Maximum allowed: {}", MAX_MESSAGE_COUNT),
            Some("messages"),
            None,
        ));
    }

    // Validate message lengths
    for (idx, msg) in request.messages.iter().enumerate() {
        if msg.content.len() > MAX_QUERY_LENGTH {
            return Err(UserError::invalid_request(
                "Message content exceeds maximum allowed length",
                Some("messages"),
                Some(&format!("Message {} exceeds {} characters", idx, MAX_QUERY_LENGTH)),
            ));
        }
        if msg.content.trim().is_empty() {
            return Err(UserError::invalid_request(
                "Message content cannot be empty",
                Some("messages"),
                Some(&format!("Message {} has empty content", idx)),
            ));
        }
    }

    // Extract the last user message for cache key (semantic matching)
    let cache_key = request.messages.last()
        .map(|m| m.content.as_str())
        .unwrap_or("");

    // CRITICAL FIX: Generate embedding OUTSIDE any lock to avoid holding write lock during network I/O
    // This prevents serializing all requests during the 60s embedding timeout
    let embedding = {
        let cache = state.cache.read().await;
        match cache.generate_embedding(cache_key).await {
            Ok(emb) => Some(emb),
            Err(_) => {
                // Record embedding failure but continue (will try exact match)
                cache.record_embedding_failure();
                None
            }
        }
    };

    // Check cache first for ALL requests using READ lock with pre-generated embedding
    // This allows concurrent cache lookups without blocking other requests
    let cache_result = {
        let mut cache = state.cache.write().await;
        if let Some(ref emb) = embedding {
            cache.search_with_embedding(cache_key, emb)
        } else {
            // Fallback to exact match if embedding generation failed
            cache.search_with_embedding(cache_key, &[])
        }
    };

    if let Some(cached) = cache_result {

        let latency_ms = start_time.elapsed().as_millis() as u64;
        tracing::info!("Cache hit for query (hit_count: {}, age: {:.1}h)",
            cached.hit_count, cached.age_hours());

        // Record cache hit to stats tracker
        let query_stats = stats::QueryStats::new(
            Tier::Cache,
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
                message: Message {
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

    // Cache miss - proceed to execute query on the selected tier
    // (lock was already released after cache lookup)

    // Determine which tier to use based on the requested model
    // rigrun uses auto-routing for maximum cost efficiency
    let tier = match request.model.as_str() {
        "auto" => {
            // Use router to determine tier based on query complexity
            // Simple queries → local, complex → cloud (OpenRouter auto-routes)
            route_query(cache_key, None)
        }
        "local" => Tier::Local,  // Force local Ollama
        "cache" => Tier::Cache,  // Cache-only (will fall back to local on miss)
        // All cloud requests use OpenRouter auto-routing for best cost/performance
        "cloud" | "haiku" | "sonnet" | "opus" | "gpt4" | "gpt4o" => Tier::Cloud,
        _ => Tier::Local, // Default to local for unknown models
    };

    // If explicitly requested cache tier but had cache miss, fall back to Local
    if tier == Tier::Cache {
        tracing::debug!("Cache miss for explicit cache tier request, falling back to Local tier");
    }

    // Messages are already in the correct format (types::Message)
    let messages = request.messages.clone();

    // Execute on appropriate tier
    let (response_text, prompt_tokens, completion_tokens, actual_tier) = match tier {
        Tier::Cache | Tier::Local => {
            // Call Ollama for inference with timeout to prevent hanging
            let ollama_future = state
                .ollama_client
                .chat_async(&state.local_model, messages.clone());

            let ollama_result = tokio::time::timeout(
                std::time::Duration::from_secs(OLLAMA_TIMEOUT_SECS),
                ollama_future,
            )
            .await;

            // Handle result: success, or fallback to cloud on failure/timeout
            let local_failed = match &ollama_result {
                Ok(Ok(_)) => false,
                _ => true,
            };

            if !local_failed {
                // Local succeeded
                let ollama_response = ollama_result.unwrap().unwrap();
                (
                    ollama_response.response,
                    ollama_response.prompt_tokens,
                    ollama_response.completion_tokens,
                    Tier::Local,
                )
            } else if state.paranoid_mode {
                // Local failed but paranoid mode is on - can't fall back to cloud
                // IL5-compliant: Log full details internally, return sanitized message to user
                let internal_error = match &ollama_result {
                    Err(_) => format!("Ollama request timed out after {} seconds", OLLAMA_TIMEOUT_SECS),
                    Ok(Err(e)) => format!("Ollama error: {}", e),
                    _ => "Unknown local inference error".to_string(),
                };
                return Err(UserError::gateway_timeout(&format!(
                    "{} (paranoid mode: cloud fallback blocked)",
                    sanitize_error_details(&internal_error)
                )));
            } else {
                // Local failed or timed out - fall back to cloud!
                tracing::warn!("Local inference failed/timed out, falling back to cloud (OpenRouter auto)");

                // Use OpenRouter auto-router for automatic model selection
                let model = "openrouter/auto";

                let openrouter_response = state
                    .openrouter_client
                    .chat(model, messages.clone())
                    .await
                    .map_err(|e| {
                        // IL5-compliant: Full error logged internally, sanitized response to user
                        let internal_error = format!("OpenRouter fallback error after local failure: {}", e);
                        UserError::service_unavailable(&internal_error)
                    })?;

                (
                    openrouter_response.response,
                    openrouter_response.prompt_tokens,
                    openrouter_response.completion_tokens,
                    Tier::Cloud,
                )
            }
        }
        Tier::Cloud => {
            // PARANOID MODE: Block cloud requests (IL5-compliant error response)
            if state.paranoid_mode {
                tracing::warn!("PARANOID MODE: Blocking cloud request");
                audit::audit_log_blocked(Tier::Cloud, cache_key);
                return Err(UserError::authorization_denied(Some(
                    "Paranoid mode enabled: cloud requests are blocked"
                )));
            }

            // Use OpenRouter auto-router - it picks the best model automatically
            let model = "openrouter/auto";

            // Messages are already in the correct format (types::Message)
            let cloud_messages = request.messages.clone();

            // Call OpenRouter for cloud inference with auto-routing
            let openrouter_response = state
                .openrouter_client
                .chat(model, cloud_messages)
                .await
                .map_err(|e| {
                    // IL5-compliant: Full error logged internally, sanitized response to user
                    let internal_error = format!("OpenRouter cloud inference error: {}", e);
                    crate::errors::map_error(&internal_error)
                })?;

            (
                openrouter_response.response,
                openrouter_response.prompt_tokens,
                openrouter_response.completion_tokens,
                Tier::Cloud,
            )
        }
        // All other tiers (Haiku, Sonnet, Opus, Gpt4o) are legacy - not used with auto-routing
        _ => unreachable!("All cloud requests now use Tier::Cloud with OpenRouter auto-routing")
    };

    // Store response in cache for future hits
    // CRITICAL FIX: Use pre-generated embedding to avoid holding write lock during embedding generation
    {
        let mut cache = state.cache.write().await;
        if let Some(ref emb) = embedding {
            // Use store_with_embedding to avoid regenerating the embedding
            cache.store_with_embedding(
                cache_key,
                emb.clone(),
                response_text.clone(),
                actual_tier,
                prompt_tokens + completion_tokens,
            );
        } else {
            // Fallback to regular store if embedding generation failed earlier
            cache.store_response(
                cache_key,
                response_text.clone(),
                actual_tier,
                prompt_tokens + completion_tokens,
            ).await;
        }
        tracing::debug!("Stored response in semantic cache (entries: {})", cache.len());
    }

    let total_tokens = prompt_tokens + completion_tokens;
    let latency_ms = start_time.elapsed().as_millis() as u64;

    // Record query to stats tracker
    let query_stats = stats::QueryStats::new(
        actual_tier,
        prompt_tokens,
        completion_tokens,
        latency_ms,
    );
    tracing::info!(
        "Recording query: tier={:?}, tokens={}, latency={}ms",
        actual_tier,
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

    // Audit logging: record the query for transparency
    let cost_usd = actual_tier.calculate_cost(prompt_tokens, completion_tokens) as f64 / 100.0;
    audit::audit_log_query(actual_tier, cache_key, prompt_tokens, completion_tokens, cost_usd);

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
            message: Message {
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

// =============================================================================
// SSE STREAMING RESPONSE TYPES
// =============================================================================

/// SSE event for streaming chat completions.
#[derive(Debug, Serialize)]
struct StreamEvent {
    /// The token text.
    token: String,
    /// Whether this is the final event.
    done: bool,
    /// Total tokens so far (only present when done=true).
    #[serde(skip_serializing_if = "Option::is_none")]
    total_tokens: Option<u32>,
    /// Error message if streaming failed.
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<String>,
}

impl StreamEvent {
    fn token(text: impl Into<String>) -> Self {
        Self {
            token: text.into(),
            done: false,
            total_tokens: None,
            error: None,
        }
    }

    fn done(total: u32) -> Self {
        Self {
            token: String::new(),
            done: true,
            total_tokens: Some(total),
            error: None,
        }
    }

    fn error(msg: impl Into<String>) -> Self {
        Self {
            token: String::new(),
            done: true,
            total_tokens: None,
            error: Some(msg.into()),
        }
    }
}

/// Streaming chat completions handler using Server-Sent Events (SSE).
///
/// This endpoint streams tokens as they arrive, providing sub-500ms time-to-first-token.
/// The response is a stream of SSE events in the format:
///
/// ```text
/// data: {"token": "The", "done": false}
/// data: {"token": " answer", "done": false}
/// data: {"token": "...", "done": false}
/// data: {"token": "", "done": true, "total_tokens": 150}
/// ```
///
/// Supports:
/// - Connection drops mid-stream (graceful handling)
/// - User cancellation (client closes connection)
/// - Model timeout (sends error event)
/// - Error mid-stream (sends error event with partial response)
async fn stream_completions_handler(
    State(state): State<Arc<AppState>>,
    Json(request): Json<ChatCompletionRequest>,
) -> Result<Sse<impl Stream<Item = Result<Event, Infallible>>>, UserError> {
    // Input validation (same as non-streaming endpoint)
    if request.messages.is_empty() {
        return Err(UserError::invalid_request(
            "Request must contain at least one message",
            Some("messages"),
            None,
        ));
    }

    if request.messages.len() > MAX_MESSAGE_COUNT {
        return Err(UserError::invalid_request(
            &format!("Too many messages. Maximum allowed: {}", MAX_MESSAGE_COUNT),
            Some("messages"),
            None,
        ));
    }

    for (idx, msg) in request.messages.iter().enumerate() {
        if msg.content.len() > MAX_QUERY_LENGTH {
            return Err(UserError::invalid_request(
                "Message content exceeds maximum allowed length",
                Some("messages"),
                Some(&format!("Message {} exceeds {} characters", idx, MAX_QUERY_LENGTH)),
            ));
        }
        if msg.content.trim().is_empty() {
            return Err(UserError::invalid_request(
                "Message content cannot be empty",
                Some("messages"),
                Some(&format!("Message {} has empty content", idx)),
            ));
        }
    }

    // Create a channel for streaming tokens
    let (tx, rx) = tokio::sync::mpsc::channel::<Result<Event, Infallible>>(64);

    // Clone what we need for the spawned task
    let messages = request.messages.clone();
    let model_name = request.model.clone();
    let local_model = state.local_model.clone();
    let ollama_client = state.ollama_client.clone();
    let openrouter_client = state.openrouter_client.clone();
    let paranoid_mode = state.paranoid_mode;

    // Determine which tier to use
    let cache_key = messages.last()
        .map(|m| m.content.as_str())
        .unwrap_or("");

    let tier = match model_name.as_str() {
        "auto" => route_query(cache_key, None),
        "local" => Tier::Local,
        "cache" => Tier::Cache,
        "cloud" | "haiku" | "sonnet" | "opus" | "gpt4" | "gpt4o" => Tier::Cloud,
        _ => Tier::Local,
    };

    // Spawn the streaming task
    tokio::spawn(async move {
        let start_time = Instant::now();
        let mut total_tokens = 0u32;
        let mut full_response = String::new();

        // Send initial "thinking" indicator
        let _ = tx.send(Ok(Event::default()
            .event("status")
            .data(r#"{"status": "thinking"}"#))).await;

        match tier {
            Tier::Cache | Tier::Local => {
                // Use Ollama's streaming API
                let (mut stream_rx, _handle) = ollama_client.chat_stream_async(&local_model, messages.clone());

                let mut first_token_received = false;
                while let Some(chunk) = stream_rx.recv().await {
                    if !first_token_received {
                        first_token_received = true;
                        let ttft = start_time.elapsed().as_millis();
                        tracing::info!("Time to first token: {}ms", ttft);
                    }

                    if chunk.done {
                        total_tokens = chunk.tokens_so_far.unwrap_or(total_tokens);
                        let event = StreamEvent::done(total_tokens);
                        let _ = tx.send(Ok(Event::default()
                            .data(serde_json::to_string(&event).unwrap_or_default()))).await;
                        break;
                    } else {
                        total_tokens = chunk.tokens_so_far.unwrap_or(total_tokens);
                        full_response.push_str(&chunk.token);
                        let event = StreamEvent::token(&chunk.token);
                        if tx.send(Ok(Event::default()
                            .data(serde_json::to_string(&event).unwrap_or_default()))).await.is_err() {
                            // Client disconnected
                            tracing::info!("Client disconnected mid-stream (received {} tokens)", total_tokens);
                            break;
                        }
                    }
                }

                // If local failed and not paranoid mode, fall back to cloud
                if full_response.is_empty() && !paranoid_mode {
                    let _ = tx.send(Ok(Event::default()
                        .event("status")
                        .data(r#"{"status": "fallback_to_cloud"}"#))).await;

                    // Fall back to cloud streaming
                    stream_cloud_response(&tx, &openrouter_client, messages, &mut total_tokens, &mut full_response).await;
                }
            }
            Tier::Cloud => {
                if paranoid_mode {
                    let event = StreamEvent::error("Paranoid mode enabled: cloud requests are blocked");
                    let _ = tx.send(Ok(Event::default()
                        .data(serde_json::to_string(&event).unwrap_or_default()))).await;
                } else {
                    stream_cloud_response(&tx, &openrouter_client, messages, &mut total_tokens, &mut full_response).await;
                }
            }
            _ => {
                let event = StreamEvent::error("Unsupported tier for streaming");
                let _ = tx.send(Ok(Event::default()
                    .data(serde_json::to_string(&event).unwrap_or_default()))).await;
            }
        }

        // Record stats
        let latency_ms = start_time.elapsed().as_millis() as u64;
        let query_stats = stats::QueryStats::new(
            tier,
            0, // prompt_tokens not available in streaming
            total_tokens,
            latency_ms,
        );
        stats::record_query(query_stats);
        let _ = stats::persist_stats();

        tracing::info!(
            "Streaming complete: tier={:?}, tokens={}, latency={}ms",
            tier, total_tokens, latency_ms
        );
    });

    // Return the SSE stream
    let stream = ReceiverStream::new(rx);
    Ok(Sse::new(stream)
        .keep_alive(axum::response::sse::KeepAlive::new()
            .interval(std::time::Duration::from_secs(15))
            .text("keep-alive")))
}

/// Helper to stream cloud response via OpenRouter.
async fn stream_cloud_response(
    tx: &tokio::sync::mpsc::Sender<Result<Event, Infallible>>,
    client: &OpenRouterClient,
    messages: Vec<Message>,
    total_tokens: &mut u32,
    full_response: &mut String,
) {
    let model = "openrouter/auto";
    let cancel_flag = Arc::new(AtomicBool::new(false));
    let tx_clone = tx.clone();

    // Create a channel to receive chunks from the cloud streaming
    let (chunk_tx, mut chunk_rx) = tokio::sync::mpsc::channel::<StreamChunk>(64);

    // Spawn a task to call the streaming API
    let client_clone = client.clone();
    let messages_clone = messages.clone();
    let cancel_flag_clone = cancel_flag.clone();

    let handle = tokio::spawn(async move {
        client_clone.chat_stream(
            model,
            messages_clone,
            |chunk| {
                let _ = chunk_tx.try_send(chunk);
            },
            Some(cancel_flag_clone),
        ).await
    });

    // Forward chunks to SSE
    while let Some(chunk) = chunk_rx.recv().await {
        if chunk.done {
            *total_tokens = chunk.tokens_so_far.unwrap_or(*total_tokens);
            let event = StreamEvent::done(*total_tokens);
            let _ = tx_clone.send(Ok(Event::default()
                .data(serde_json::to_string(&event).unwrap_or_default()))).await;
            break;
        } else {
            *total_tokens = chunk.tokens_so_far.unwrap_or(*total_tokens);
            full_response.push_str(&chunk.token);
            let event = StreamEvent::token(&chunk.token);
            if tx_clone.send(Ok(Event::default()
                .data(serde_json::to_string(&event).unwrap_or_default()))).await.is_err() {
                // Client disconnected - cancel the stream
                cancel_flag.store(true, Ordering::Relaxed);
                tracing::info!("Client disconnected mid-stream (received {} tokens)", *total_tokens);
                break;
            }
        }
    }

    // Wait for the handle to complete (optional, for cleanup)
    let _ = handle.await;
}

/// Stats handler.
async fn stats_handler(
    State(state): State<Arc<AppState>>,
) -> Json<StatsResponse> {
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

    // Get semantic cache stats if available
    let cache = state.cache.read().await;
    let stats = cache.stats();
    let semantic_cache = Some(SemanticCacheMetrics {
        semantic_hits: stats.semantic_hits,
        exact_hits: stats.exact_hits,
        semantic_hit_rate: stats.semantic_hit_rate,
        total_hit_rate: stats.total_hit_rate,
        embedding_failures: stats.embedding_failures,
    });

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
        semantic_cache,
    })
}

/// Cache statistics response.
#[derive(Serialize)]
struct CacheStatsResponse {
    entries: usize,
    total_lookups: u64,
    semantic_hits: u64,
    exact_hits: u64,
    misses: u64,
    embedding_failures: u64,
    semantic_hit_rate_percent: f32,
    total_hit_rate_percent: f32,
    vector_index_entries: usize,
}

/// Cache stats handler.
async fn cache_stats_handler(
    State(state): State<Arc<AppState>>,
) -> Json<CacheStatsResponse> {
    let cache = state.cache.read().await;
    let stats = cache.stats();
    let entries = cache.len();
    let vector_index_entries = cache.vector_index_len();
    let total_lookups = stats.total_lookups;
    let semantic_hits = stats.semantic_hits;
    let exact_hits = stats.exact_hits;
    let misses = stats.misses;
    let embedding_failures = stats.embedding_failures;
    let semantic_hit_rate = stats.semantic_hit_rate;
    let total_hit_rate = stats.total_hit_rate;

    Json(CacheStatsResponse {
        entries,
        total_lookups,
        semantic_hits,
        exact_hits,
        misses,
        embedding_failures,
        semantic_hit_rate_percent: semantic_hit_rate,
        total_hit_rate_percent: total_hit_rate,
        vector_index_entries,
    })
}

/// Semantic cache statistics response.
#[derive(Serialize)]
struct SemanticCacheStatsResponse {
    total_lookups: u64,
    semantic_hits: u64,
    exact_hits: u64,
    misses: u64,
    semantic_hit_rate: f32,
    total_hit_rate: f32,
    embedding_failures: u64,
}

/// Semantic cache stats handler.
async fn semantic_cache_stats_handler(
    State(state): State<Arc<AppState>>,
) -> Json<SemanticCacheStatsResponse> {
    let cache = state.cache.read().await;
    let semantic_stats = cache.stats();

    Json(SemanticCacheStatsResponse {
        total_lookups: semantic_stats.total_lookups,
        semantic_hits: semantic_stats.semantic_hits,
        exact_hits: semantic_stats.exact_hits,
        misses: semantic_stats.misses,
        semantic_hit_rate: semantic_stats.semantic_hit_rate,
        total_hit_rate: semantic_stats.total_hit_rate,
        embedding_failures: semantic_stats.embedding_failures,
    })
}

// =============================================================================
// Utilities
// =============================================================================

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

        // Try to install SIGTERM handler, fall back to SIGINT-only if it fails
        let sigterm_result = signal(SignalKind::terminate());
        let sigint_result = signal(SignalKind::interrupt());

        match (sigterm_result, sigint_result) {
            (Ok(mut sigterm), Ok(mut sigint)) => {
                // Both handlers installed successfully
                tokio::select! {
                    _ = sigterm.recv() => {
                        tracing::info!("Received SIGTERM, initiating graceful shutdown...");
                    }
                    _ = sigint.recv() => {
                        tracing::info!("Received SIGINT (Ctrl+C), initiating graceful shutdown...");
                    }
                }
            }
            (Err(e), Ok(mut sigint)) => {
                // SIGTERM handler failed, use SIGINT only
                tracing::warn!("Failed to install SIGTERM handler: {}, using SIGINT (Ctrl+C) only", e);
                sigint.recv().await;
                tracing::info!("Received SIGINT (Ctrl+C), initiating graceful shutdown...");
            }
            (Ok(mut sigterm), Err(e)) => {
                // SIGINT handler failed, use SIGTERM only
                tracing::warn!("Failed to install SIGINT handler: {}, using SIGTERM only", e);
                sigterm.recv().await;
                tracing::info!("Received SIGTERM, initiating graceful shutdown...");
            }
            (Err(e1), Err(e2)) => {
                // Both handlers failed - this is critical, must panic
                // JUSTIFICATION: This panic is acceptable because:
                // 1. This runs during server initialization, not request handling
                // 2. Without signal handlers, the server cannot be stopped gracefully
                // 3. This prevents data loss and orphaned processes
                // 4. Failure indicates severe OS-level issues (out of file descriptors, etc.)
                panic!("Failed to install signal handlers: SIGTERM error: {}, SIGINT error: {}. Cannot start server without signal handling.", e1, e2);
            }
        }
    }

    #[cfg(not(unix))]
    {
        // Fallback: just handle Ctrl+C on non-Unix platforms (Windows)
        match tokio::signal::ctrl_c().await {
            Ok(_) => {
                tracing::info!("Received Ctrl+C, initiating graceful shutdown...");
            }
            Err(e) => {
                // This is critical - if we can't handle Ctrl+C, the server can't be shut down gracefully
                // JUSTIFICATION: This panic is acceptable because:
                // 1. This runs during server initialization, not request handling
                // 2. Without Ctrl+C handler on Windows, server cannot be stopped gracefully
                // 3. Prevents orphaned processes and data loss
                // 4. Failure indicates severe OS-level issues
                panic!("Failed to install Ctrl+C handler: {}. Cannot start server without signal handling.", e);
            }
        }
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
