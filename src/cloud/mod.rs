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

/// Default OpenRouter API endpoint.
const DEFAULT_OPENROUTER_URL: &str = "https://openrouter.ai/api/v1";

/// Default timeout for API requests (in seconds).
const REQUEST_TIMEOUT_SECS: u64 = 120;

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


/// A chat message for the chat completion API.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Message {
    /// The role of the message sender.
    pub role: String,
    /// The content of the message.
    pub content: String,
}

impl Message {
    /// Create a new message.
    pub fn new(role: impl Into<String>, content: impl Into<String>) -> Self {
        Self {
            role: role.into(),
            content: content.into(),
        }
    }

    /// Create a system message.
    pub fn system(content: impl Into<String>) -> Self {
        Self::new("system", content)
    }

    /// Create a user message.
    pub fn user(content: impl Into<String>) -> Self {
        Self::new("user", content)
    }

    /// Create an assistant message.
    pub fn assistant(content: impl Into<String>) -> Self {
        Self::new("assistant", content)
    }
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
            Self::NotConfigured(msg) => write!(f, "OpenRouter not configured: {}", msg),
            Self::AuthError(msg) => write!(f, "Authentication error: {}", msg),
            Self::RateLimited(msg) => write!(f, "Rate limited: {}", msg),
            Self::ModelNotFound(model) => write!(f, "Model not found: {}", model),
            Self::ApiError(msg) => write!(f, "API error: {}", msg),
            Self::NetworkError(msg) => write!(f, "Network error: {}", msg),
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
    fn with_api_key_option(api_key: Option<String>) -> Self {
        let client = reqwest::Client::builder()
            .timeout(Duration::from_secs(REQUEST_TIMEOUT_SECS))
            .build()
            .expect("Failed to create HTTP client");

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

    /// Perform a chat completion.
    pub async fn chat(&self, model: &str, messages: Vec<Message>) -> Result<OpenRouterResponse> {
        let api_key = self.api_key.as_ref()
            .ok_or_else(|| anyhow!(OpenRouterError::NotConfigured(
                "Set OPENROUTER_API_KEY environment variable or configure via rigrun config".to_string()
            )))?;

        let url = format!("{}/chat/completions", self.base_url);

        let mut request = self.client
            .post(&url)
            .header("Authorization", format!("Bearer {}", api_key))
            .header("Content-Type", "application/json");

        // Add optional headers
        if let Some(ref site_url) = self.site_url {
            request = request.header("HTTP-Referer", site_url);
        }
        if let Some(ref site_name) = self.site_name {
            request = request.header("X-Title", site_name);
        }

        let body = serde_json::json!({
            "model": model,
            "messages": messages,
        });

        let response = request
            .json(&body)
            .timeout(self.timeout)
            .send()
            .await
            .map_err(|e| {
                if e.is_timeout() {
                    anyhow!(OpenRouterError::NetworkError(
                        "Request timed out. Consider using a faster model.".to_string()
                    ))
                } else {
                    anyhow!(OpenRouterError::NetworkError(e.to_string()))
                }
            })?;

        let status = response.status();

        if !status.is_success() {
            let error_text = response.text().await.unwrap_or_default();

            if status.as_u16() == 401 {
                return Err(anyhow!(OpenRouterError::AuthError(
                    "Invalid API key. Check your OPENROUTER_API_KEY.".to_string()
                )));
            }
            if status.as_u16() == 429 {
                return Err(anyhow!(OpenRouterError::RateLimited(
                    "Rate limit exceeded. Wait a moment and try again.".to_string()
                )));
            }
            if status.as_u16() == 404 || error_text.contains("not found") {
                return Err(anyhow!(OpenRouterError::ModelNotFound(model.to_string())));
            }

            return Err(anyhow!(OpenRouterError::ApiError(format!(
                "API error: HTTP {} - {}",
                status, error_text
            ))));
        }

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

        Ok(OpenRouterResponse {
            response: response_text,
            prompt_tokens,
            completion_tokens,
            model: chat_response.model.unwrap_or_else(|| model.to_string()),
            cost_usd: None, // OpenRouter doesn't return cost directly
        })
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

/// Popular models available on OpenRouter.
pub mod models {
    /// Claude 3 Haiku - fast and cheap.
    pub const CLAUDE_3_HAIKU: &str = "anthropic/claude-3-haiku";
    /// Claude 3 Sonnet - balanced.
    pub const CLAUDE_3_SONNET: &str = "anthropic/claude-3-sonnet";
    /// Claude 3 Opus - most capable.
    pub const CLAUDE_3_OPUS: &str = "anthropic/claude-3-opus";
    /// GPT-4o - OpenAI's flagship.
    pub const GPT_4O: &str = "openai/gpt-4o";
    /// GPT-4o-mini - fast and cheap.
    pub const GPT_4O_MINI: &str = "openai/gpt-4o-mini";
    /// Llama 3.1 70B - open source.
    pub const LLAMA_3_1_70B: &str = "meta-llama/llama-3.1-70b-instruct";
    /// Llama 3.1 8B - lightweight open source.
    pub const LLAMA_3_1_8B: &str = "meta-llama/llama-3.1-8b-instruct";
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
        assert!(err.to_string().contains("Rate limited"));
    }
}
