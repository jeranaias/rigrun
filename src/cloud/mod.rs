// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

//! OpenRouter integration
//!
//! Provides cloud LLM inference through OpenRouter API for fallback
//! when local inference is not available or insufficient.
//!
//! # Example
//!
//! ```no_run
//! use rigrun::cloud::OpenRouterClient;
//!
//! # async fn example() -> anyhow::Result<()> {
//! let client = OpenRouterClient::with_api_key("sk-or-...");
//!
//! // List available models
//! let models = client.list_models().await?;
//!
//! // Generate a completion
//! let response = client.generate("anthropic/claude-3-haiku", "Hello!").await?;
//! # Ok(())
//! # }
//! ```

use anyhow::{anyhow, Context, Result};
use serde::{Deserialize, Serialize};
use std::time::Duration;
use tokio::time::sleep;

// Re-export Message from types for API compatibility
pub use crate::types::Message;

/// Default OpenRouter API endpoint.
const DEFAULT_OPENROUTER_URL: &str = "https://openrouter.ai/api/v1";

/// Default timeout for API requests (in seconds).
const REQUEST_TIMEOUT_SECS: u64 = 120;

/// Maximum retry attempts for transient errors.
const MAX_RETRIES: u32 = 3;

/// Base delay for exponential backoff (milliseconds).
const RETRY_BASE_DELAY_MS: u64 = 500;

/// Maximum delay for exponential backoff (milliseconds).
const RETRY_MAX_DELAY_MS: u64 = 10000;

/// Response from OpenRouter generation.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct OpenRouterResponse {
    /// The generated text response.
    pub response: String,
    /// Number of tokens in the prompt.
    pub prompt_tokens: u32,
    /// Number of tokens in the completion.
    pub completion_tokens: u32,
    /// Model used for generation.
    pub model: String,
    /// Total cost in USD (if available).
    pub cost_usd: Option<f64>,
}


/// Error types specific to OpenRouter operations.
#[derive(Debug, Clone)]
pub enum OpenRouterError {
    /// API key not configured.
    NotConfigured(String),
    /// Authentication failed.
    AuthError(String),
    /// Rate limit exceeded.
    RateLimited(String),
    /// Model not found.
    ModelNotFound(String),
    /// API error.
    ApiError(String),
    /// Network error.
    NetworkError(String),
}

impl std::fmt::Display for OpenRouterError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::NotConfigured(msg) => {
                let error = format!(
                    "[✗] OpenRouter not configured\n\n{}\n\nPossible causes:\n  - OPENROUTER_API_KEY environment variable not set\n  - API key not added to rigrun config\n  - API key was deleted or reset\n\nTry these fixes:\n  1. Get an API key: https://openrouter.ai/keys\n  2. Set it: export OPENROUTER_API_KEY=sk-or-...\n  3. Or configure: rigrun config set openrouter_api_key sk-or-...\n  4. Verify: rigrun config show\n\nNeed help? https://github.com/jeranaias/rigrun/issues",
                    msg
                );
                write!(f, "{}", error)
            }
            Self::AuthError(msg) => {
                let error = format!(
                    "[✗] Authentication failed\n\n{}\n\nPossible causes:\n  - Invalid or expired API key\n  - API key was revoked\n  - Incorrect API key format\n  - Account suspended\n\nTry these fixes:\n  1. Verify your API key at: https://openrouter.ai/keys\n  2. Generate a new key if needed\n  3. Update config: rigrun config set openrouter_api_key sk-or-...\n  4. Check account status: https://openrouter.ai/account\n\nNeed help? https://github.com/jeranaias/rigrun/issues",
                    msg
                );
                write!(f, "{}", error)
            }
            Self::RateLimited(msg) => {
                let error = format!(
                    "[✗] Rate limit exceeded\n\n{}\n\nPossible causes:\n  - Too many requests in short time\n  - Free tier limit reached\n  - Account quota exceeded\n  - Shared IP address issue\n\nTry these fixes:\n  1. Wait 60 seconds and try again\n  2. Add credits to account: https://openrouter.ai/credits\n  3. Use a slower model to reduce costs\n  4. Check your usage: https://openrouter.ai/activity\n\nNeed help? https://github.com/jeranaias/rigrun/issues",
                    msg
                );
                write!(f, "{}", error)
            }
            Self::ModelNotFound(model) => {
                let error = format!(
                    "[✗] Model not found: {}\n\nPossible causes:\n  - Model name misspelled\n  - Model was deprecated or removed\n  - Model ID format incorrect\n  - Model not available in your region\n\nTry these fixes:\n  1. List available models: rigrun models\n  2. Check model name spelling\n  3. Browse models: https://openrouter.ai/models\n  4. Try a popular model: anthropic/claude-3-haiku\n\nNeed help? https://github.com/jeranaias/rigrun/issues",
                    model
                );
                write!(f, "{}", error)
            }
            Self::ApiError(msg) => {
                let error = format!(
                    "[✗] OpenRouter API error\n\n{}\n\nPossible causes:\n  - OpenRouter service temporarily down\n  - Invalid request format\n  - Model overloaded or unavailable\n  - Account issue\n\nTry these fixes:\n  1. Check OpenRouter status: https://status.openrouter.ai\n  2. Try a different model\n  3. Wait a moment and retry\n  4. Check account: https://openrouter.ai/account\n\nNeed help? https://github.com/jeranaias/rigrun/issues",
                    msg
                );
                write!(f, "{}", error)
            }
            Self::NetworkError(msg) => {
                let error = format!(
                    "[✗] Network error\n\n{}\n\nPossible causes:\n  - No internet connection\n  - DNS resolution failure\n  - Firewall blocking HTTPS\n  - Proxy or VPN interference\n\nTry these fixes:\n  1. Check internet connection\n  2. Verify DNS: ping openrouter.ai\n  3. Check firewall settings\n  4. Disable VPN temporarily\n\nNeed help? https://github.com/jeranaias/rigrun/issues",
                    msg
                );
                write!(f, "{}", error)
            }
        }
    }
}

impl std::error::Error for OpenRouterError {}

/// Model information from OpenRouter.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelInfo {
    /// Model ID (e.g., "anthropic/claude-3-haiku").
    pub id: String,
    /// Human-readable name.
    pub name: String,
    /// Context window size.
    pub context_length: u32,
    /// Pricing per 1K input tokens (USD).
    pub input_cost_per_1k: f64,
    /// Pricing per 1K output tokens (USD).
    pub output_cost_per_1k: f64,
}

/// Internal response structures for OpenRouter API.
#[derive(Debug, Deserialize)]
struct ChatCompletionResponse {
    #[allow(dead_code)]
    id: Option<String>,
    choices: Vec<ChatChoice>,
    usage: Option<Usage>,
    model: Option<String>,
}

#[derive(Debug, Deserialize)]
struct ChatChoice {
    message: ChatMessage,
    #[allow(dead_code)]
    finish_reason: Option<String>,
}

#[derive(Debug, Deserialize)]
struct ChatMessage {
    content: Option<String>,
}

#[derive(Debug, Deserialize)]
struct Usage {
    prompt_tokens: u32,
    completion_tokens: u32,
    #[allow(dead_code)]
    total_tokens: u32,
}

#[derive(Debug, Deserialize)]
struct ModelsResponse {
    data: Vec<ModelData>,
}

#[derive(Debug, Deserialize)]
struct ModelData {
    id: String,
    name: Option<String>,
    context_length: Option<u32>,
    pricing: Option<Pricing>,
}

#[derive(Debug, Deserialize)]
struct Pricing {
    prompt: Option<String>,
    completion: Option<String>,
}

/// Client for communicating with OpenRouter.
#[derive(Debug, Clone)]
pub struct OpenRouterClient {
    /// API key for authentication.
    api_key: Option<String>,
    /// Base URL for OpenRouter API.
    base_url: String,
    /// HTTP client with configured timeouts.
    client: reqwest::Client,
    /// Request timeout.
    timeout: Duration,
    /// Site URL for OpenRouter (for rate limit categorization).
    site_url: Option<String>,
    /// Site name for OpenRouter.
    site_name: Option<String>,
}

impl Default for OpenRouterClient {
    fn default() -> Self {
        Self::new()
    }
}

impl OpenRouterClient {
    /// Create a new OpenRouter client.
    ///
    /// Attempts to read the API key from the `OPENROUTER_API_KEY` environment variable.
    pub fn new() -> Self {
        let api_key = std::env::var("OPENROUTER_API_KEY").ok();
        Self::with_api_key_option(api_key)
    }

    /// Create a new OpenRouter client with a specific API key.
    pub fn with_api_key(api_key: impl Into<String>) -> Self {
        Self::with_api_key_option(Some(api_key.into()))
    }

    /// Create a new OpenRouter client with an optional API key.
    ///
    /// # Panics
    ///
    /// Panics if the HTTP client cannot be built. This should only happen if the system's
    /// TLS/SSL stack is fundamentally broken. This is acceptable for initialization code.
    fn with_api_key_option(api_key: Option<String>) -> Self {
        let client = reqwest::Client::builder()
            .timeout(Duration::from_secs(REQUEST_TIMEOUT_SECS))
            .build()
            .expect("Failed to create HTTP client for OpenRouter. This indicates a critical system configuration issue (TLS/SSL failure).");

        Self {
            api_key,
            base_url: DEFAULT_OPENROUTER_URL.to_string(),
            client,
            timeout: Duration::from_secs(REQUEST_TIMEOUT_SECS),
            site_url: None,
            site_name: Some("rigrun".to_string()),
        }
    }

    /// Set site URL for OpenRouter (helps with rate limit categorization).
    pub fn with_site_url(mut self, url: impl Into<String>) -> Self {
        self.site_url = Some(url.into());
        self
    }

    /// Set site name for OpenRouter.
    pub fn with_site_name(mut self, name: impl Into<String>) -> Self {
        self.site_name = Some(name.into());
        self
    }

    /// Set request timeout.
    pub fn with_timeout(mut self, timeout: Duration) -> Self {
        self.timeout = timeout;
        self
    }

    /// Check if the client is configured with an API key.
    pub fn is_configured(&self) -> bool {
        self.api_key.is_some()
    }

    /// Get the API key (for display purposes - masked).
    pub fn api_key_masked(&self) -> Option<String> {
        self.api_key.as_ref().map(|k| {
            if k.len() > 8 {
                format!("{}...", &k[..8])
            } else {
                format!("{}...", k)
            }
        })
    }

    /// List available models from OpenRouter.
    pub async fn list_models(&self) -> Result<Vec<ModelInfo>> {
        let url = format!("{}/models", self.base_url);

        let response = self.client
            .get(&url)
            .timeout(self.timeout)
            .send()
            .await
            .map_err(|e| anyhow!(OpenRouterError::NetworkError(e.to_string())))?;

        if !response.status().is_success() {
            return Err(anyhow!(OpenRouterError::ApiError(format!(
                "Failed to list models: HTTP {}",
                response.status()
            ))));
        }

        let models_response: ModelsResponse = response
            .json()
            .await
            .context("Failed to parse models response")?;

        Ok(models_response.data.into_iter().map(|m| {
            let (input_cost, output_cost) = m.pricing
                .map(|p| {
                    let input = p.prompt
                        .and_then(|s| s.parse::<f64>().ok())
                        .unwrap_or(0.0) * 1000.0;
                    let output = p.completion
                        .and_then(|s| s.parse::<f64>().ok())
                        .unwrap_or(0.0) * 1000.0;
                    (input, output)
                })
                .unwrap_or((0.0, 0.0));

            ModelInfo {
                id: m.id.clone(),
                name: m.name.unwrap_or(m.id),
                context_length: m.context_length.unwrap_or(4096),
                input_cost_per_1k: input_cost,
                output_cost_per_1k: output_cost,
            }
        }).collect())
    }

    /// Generate a text completion.
    pub async fn generate(&self, model: &str, prompt: &str) -> Result<OpenRouterResponse> {
        let messages = vec![Message::user(prompt)];
        self.chat(model, messages).await
    }

    /// Perform a chat completion with automatic retry and exponential backoff.
    pub async fn chat(&self, model: &str, messages: Vec<Message>) -> Result<OpenRouterResponse> {
        let api_key = self.api_key.as_ref()
            .ok_or_else(|| anyhow!(OpenRouterError::NotConfigured(
                "API key is not set.".to_string()
            )))?;

        let url = format!("{}/chat/completions", self.base_url);
        let body = serde_json::json!({
            "model": model,
            "messages": messages,
        });

        let mut last_error = None;

        for attempt in 0..MAX_RETRIES {
            if attempt > 0 {
                // Exponential backoff: 500ms, 1000ms, 2000ms, ... capped at 10s
                let delay = std::cmp::min(
                    RETRY_BASE_DELAY_MS * (1 << attempt),
                    RETRY_MAX_DELAY_MS
                );
                tracing::debug!("Retry attempt {} after {}ms delay", attempt + 1, delay);
                sleep(Duration::from_millis(delay)).await;
            }

            let mut request = self.client
                .post(&url)
                .header("Authorization", format!("Bearer {}", api_key))
                .header("Content-Type", "application/json")
                .header("User-Agent", "rigrun/0.2.0");

            // Add optional headers
            if let Some(ref site_url) = self.site_url {
                request = request.header("HTTP-Referer", site_url);
            }
            if let Some(ref site_name) = self.site_name {
                request = request.header("X-Title", site_name);
            }

            let response = match request
                .json(&body)
                .timeout(self.timeout)
                .send()
                .await
            {
                Ok(resp) => resp,
                Err(e) => {
                    let err = if e.is_timeout() {
                        OpenRouterError::NetworkError("Request timed out.".to_string())
                    } else if e.is_connect() {
                        OpenRouterError::NetworkError(format!(
                            "Failed to connect to OpenRouter: {}",
                            e
                        ))
                    } else {
                        OpenRouterError::NetworkError(format!("Network error: {}", e))
                    };
                    last_error = Some(err);
                    continue; // Retry on network errors
                }
            };

            let status = response.status();
            let status_code = status.as_u16();

            // Handle non-success responses
            if !status.is_success() {
                let error_text = response.text().await.unwrap_or_default();

                // Non-retryable errors - fail immediately
                if status_code == 401 {
                    return Err(anyhow!(OpenRouterError::AuthError(
                        "Invalid API key.".to_string()
                    )));
                }
                if status_code == 402 {
                    return Err(anyhow!(OpenRouterError::ApiError(
                        "Insufficient credits. Add funds at https://openrouter.ai/credits".to_string()
                    )));
                }
                if status_code == 404 || error_text.contains("not found") {
                    return Err(anyhow!(OpenRouterError::ModelNotFound(model.to_string())));
                }

                // Retryable errors
                if status_code == 429 {
                    last_error = Some(OpenRouterError::RateLimited("Too many requests.".to_string()));
                    continue; // Retry with backoff
                }
                if status_code >= 500 && status_code < 600 {
                    last_error = Some(OpenRouterError::ApiError(format!(
                        "Server error: HTTP {} - {}",
                        status, error_text
                    )));
                    continue; // Retry on server errors
                }

                // Unknown error - don't retry
                return Err(anyhow!(OpenRouterError::ApiError(format!(
                    "API error: HTTP {} - {}",
                    status, error_text
                ))));
            }

            // Success - parse response
            let chat_response: ChatCompletionResponse = response
                .json()
                .await
                .context("Failed to parse chat response")?;

            let response_text = chat_response.choices
                .first()
                .and_then(|c| c.message.content.clone())
                .unwrap_or_default();

            let (prompt_tokens, completion_tokens) = chat_response.usage
                .map(|u| (u.prompt_tokens, u.completion_tokens))
                .unwrap_or((0, 0));

            return Ok(OpenRouterResponse {
                response: response_text,
                prompt_tokens,
                completion_tokens,
                model: chat_response.model.unwrap_or_else(|| model.to_string()),
                cost_usd: None,
            });
        }

        // All retries exhausted
        Err(anyhow!(last_error.unwrap_or_else(|| OpenRouterError::ApiError(
            "Max retries exceeded".to_string()
        ))))
    }

    /// Get the base URL.
    pub fn base_url(&self) -> &str {
        &self.base_url
    }
}

/// Convenience function to check if OpenRouter is configured.
pub fn is_configured() -> bool {
    std::env::var("OPENROUTER_API_KEY").is_ok()
}

/// Convenience function for chat completion with default client.
pub async fn chat(model: &str, messages: Vec<Message>) -> Result<OpenRouterResponse> {
    OpenRouterClient::new().chat(model, messages).await
}

/// Convenience function for text generation with default client.
pub async fn generate(model: &str, prompt: &str) -> Result<OpenRouterResponse> {
    OpenRouterClient::new().generate(model, prompt).await
}

/// Model constants for OpenRouter.
pub mod models {
    /// Auto-router - OpenRouter picks the best model for the task (RECOMMENDED).
    /// This is the most cost-effective option as OpenRouter intelligently
    /// routes to cheaper models when possible.
    pub const AUTO: &str = "openrouter/auto";
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_message_constructors() {
        let system = Message::system("You are helpful.");
        assert_eq!(system.role, "system");
        assert_eq!(system.content, "You are helpful.");

        let user = Message::user("Hello!");
        assert_eq!(user.role, "user");

        let assistant = Message::assistant("Hi there!");
        assert_eq!(assistant.role, "assistant");
    }

    #[test]
    fn test_client_configuration() {
        let client = OpenRouterClient::new();
        // Client should work without API key (just won't be able to make requests)
        assert_eq!(client.base_url(), DEFAULT_OPENROUTER_URL);
    }

    #[test]
    fn test_error_display() {
        let err = OpenRouterError::NotConfigured("test".to_string());
        assert!(err.to_string().contains("not configured"));

        let err = OpenRouterError::AuthError("test".to_string());
        assert!(err.to_string().contains("Authentication"));

        let err = OpenRouterError::RateLimited("test".to_string());
        assert!(err.to_string().contains("Rate limit"));
    }
}
