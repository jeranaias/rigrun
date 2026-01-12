//! Ollama Integration Module for rigrun.
//!
//! Provides a complete interface to interact with locally running Ollama instances.
//! Supports model management, text generation, and chat completions.
//!
//! # Example
//!
//! ```no_run
//! use rigrun::local::{OllamaClient, Message};
//!
//! let client = OllamaClient::new();
//!
//! // Check if Ollama is running
//! if client.check_ollama_running() {
//!     // List available models
//!     let models = client.list_models().unwrap();
//!
//!     // Generate a completion
//!     let response = client.generate("llama3.2:latest", "Hello, world!").unwrap();
//!     println!("{}", response.response);
//! }
//! ```

use anyhow::{anyhow, Context, Result};
use serde::{Deserialize, Serialize};
use std::time::Duration;

/// Default Ollama endpoint.
const DEFAULT_OLLAMA_URL: &str = "http://localhost:11434";

/// Default timeout for connection checks (in seconds).
const CONNECTION_TIMEOUT_SECS: u64 = 5;

/// Default timeout for generation requests (in seconds).
const GENERATION_TIMEOUT_SECS: u64 = 300;

/// Default timeout for model pull operations (in seconds).
const PULL_TIMEOUT_SECS: u64 = 3600;

/// Response from Ollama generation or chat endpoints.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct OllamaResponse {
    /// The generated text response.
    pub response: String,
    /// Number of tokens in the prompt.
    pub prompt_tokens: u32,
    /// Number of tokens in the completion.
    pub completion_tokens: u32,
    /// Total duration of the request in milliseconds.
    pub total_duration_ms: u64,
}


/// A chat message for the chat completion API.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Message {
    /// The role of the message sender (e.g., "system", "user", "assistant").
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

/// Error types specific to Ollama operations.
#[derive(Debug, Clone)]
pub enum OllamaError {
    /// Ollama server is not running or unreachable.
    NotRunning(String),
    /// Connection timed out.
    Timeout(String),
    /// The requested model was not found.
    ModelNotFound(String),
    /// API error from Ollama.
    ApiError(String),
    /// Network or HTTP error.
    NetworkError(String),
}

impl std::fmt::Display for OllamaError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::NotRunning(msg) => write!(f, "Ollama is not running: {}", msg),
            Self::Timeout(msg) => write!(f, "Request timed out: {}", msg),
            Self::ModelNotFound(model) => write!(f, "Model not found: {}", model),
            Self::ApiError(msg) => write!(f, "Ollama API error: {}", msg),
            Self::NetworkError(msg) => write!(f, "Network error: {}", msg),
        }
    }
}

impl std::error::Error for OllamaError {}

/// Progress information during model pull operations.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PullProgress {
    /// Current status message.
    pub status: String,
    /// Digest of the layer being downloaded (if applicable).
    pub digest: Option<String>,
    /// Total size in bytes (if applicable).
    pub total: Option<u64>,
    /// Completed size in bytes (if applicable).
    pub completed: Option<u64>,
}

impl PullProgress {
    /// Calculate download progress as a percentage (0-100).
    pub fn percentage(&self) -> Option<f64> {
        match (self.total, self.completed) {
            (Some(total), Some(completed)) if total > 0 => {
                Some((completed as f64 / total as f64) * 100.0)
            }
            _ => None,
        }
    }
}

/// Model information returned from the tags endpoint.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelInfo {
    /// Model name.
    pub name: String,
    /// Model size in bytes.
    pub size: u64,
    /// Model digest.
    pub digest: String,
    /// Model modification time.
    pub modified_at: String,
}

/// Internal response structure for generate API.
#[derive(Debug, Deserialize)]
struct GenerateResponse {
    response: Option<String>,
    #[allow(dead_code)]
    done: bool,
    #[serde(default)]
    prompt_eval_count: u32,
    #[serde(default)]
    eval_count: u32,
    #[serde(default)]
    total_duration: u64,
    error: Option<String>,
}

/// Internal response structure for chat API.
#[derive(Debug, Deserialize)]
struct ChatResponse {
    message: Option<ChatMessage>,
    done: bool,
    #[serde(default)]
    prompt_eval_count: u32,
    #[serde(default)]
    eval_count: u32,
    #[serde(default)]
    total_duration: u64,
    error: Option<String>,
}

#[derive(Debug, Deserialize)]
struct ChatMessage {
    content: String,
}

/// Internal response structure for tags API.
#[derive(Debug, Deserialize)]
struct TagsResponse {
    models: Vec<TagModel>,
}

#[derive(Debug, Deserialize)]
struct TagModel {
    name: String,
    size: u64,
    digest: String,
    modified_at: String,
}

/// Client for interacting with Ollama.
#[derive(Debug, Clone)]
pub struct OllamaClient {
    /// Base URL for the Ollama API.
    base_url: String,
    /// HTTP client with configured timeouts.
    client: reqwest::blocking::Client,
    /// Timeout for generation requests.
    generation_timeout: Duration,
    /// Timeout for pull operations.
    pull_timeout: Duration,
}

impl Default for OllamaClient {
    fn default() -> Self {
        Self::new()
    }
}

impl OllamaClient {
    /// Create a new Ollama client with default settings.
    ///
    /// Connects to `http://localhost:11434` by default.
    pub fn new() -> Self {
        Self::with_url(DEFAULT_OLLAMA_URL)
    }

    /// Create a new Ollama client with a custom URL.
    ///
    /// # Arguments
    ///
    /// * `url` - The base URL for the Ollama API (e.g., "http://localhost:11434")
    pub fn with_url(url: impl Into<String>) -> Self {
        let client = reqwest::blocking::Client::builder()
            .connect_timeout(Duration::from_secs(CONNECTION_TIMEOUT_SECS))
            .build()
            .expect("Failed to create HTTP client");

        Self {
            base_url: url.into().trim_end_matches('/').to_string(),
            client,
            generation_timeout: Duration::from_secs(GENERATION_TIMEOUT_SECS),
            pull_timeout: Duration::from_secs(PULL_TIMEOUT_SECS),
        }
    }

    /// Set a custom timeout for generation requests.
    pub fn with_generation_timeout(mut self, timeout: Duration) -> Self {
        self.generation_timeout = timeout;
        self
    }

    /// Set a custom timeout for model pull operations.
    pub fn with_pull_timeout(mut self, timeout: Duration) -> Self {
        self.pull_timeout = timeout;
        self
    }

    /// Check if Ollama is running and reachable.
    ///
    /// Makes a lightweight request to verify the server is responding.
    ///
    /// # Returns
    ///
    /// `true` if Ollama is running and responding, `false` otherwise.
    ///
    /// # Example
    ///
    /// ```no_run
    /// use rigrun::local::OllamaClient;
    ///
    /// let client = OllamaClient::new();
    /// if client.check_ollama_running() {
    ///     println!("Ollama is ready!");
    /// } else {
    ///     println!("Please start Ollama with: ollama serve");
    /// }
    /// ```
    pub fn check_ollama_running(&self) -> bool {
        let url = format!("{}/api/tags", self.base_url);

        match self.client
            .get(&url)
            .timeout(Duration::from_secs(CONNECTION_TIMEOUT_SECS))
            .send()
        {
            Ok(response) => response.status().is_success(),
            Err(_) => false,
        }
    }

    /// List all available models.
    ///
    /// # Returns
    ///
    /// A vector of model names available locally.
    ///
    /// # Errors
    ///
    /// Returns an error if Ollama is not running or the request fails.
    ///
    /// # Example
    ///
    /// ```no_run
    /// use rigrun::local::OllamaClient;
    ///
    /// let client = OllamaClient::new();
    /// let models = client.list_models()?;
    /// for model in models {
    ///     println!("Available: {}", model);
    /// }
    /// # Ok::<(), anyhow::Error>(())
    /// ```
    pub fn list_models(&self) -> Result<Vec<String>> {
        let url = format!("{}/api/tags", self.base_url);

        let response = self.client
            .get(&url)
            .timeout(Duration::from_secs(CONNECTION_TIMEOUT_SECS))
            .send()
            .map_err(|e| {
                if e.is_connect() {
                    anyhow!(OllamaError::NotRunning(format!(
                        "Cannot connect to Ollama at {}. Please ensure Ollama is running with: ollama serve",
                        self.base_url
                    )))
                } else if e.is_timeout() {
                    anyhow!(OllamaError::Timeout(
                        "Connection timed out while listing models".to_string()
                    ))
                } else {
                    anyhow!(OllamaError::NetworkError(e.to_string()))
                }
            })?;

        if !response.status().is_success() {
            return Err(anyhow!(OllamaError::ApiError(format!(
                "Failed to list models: HTTP {}",
                response.status()
            ))));
        }

        let tags: TagsResponse = response
            .json()
            .context("Failed to parse model list response")?;

        Ok(tags.models.into_iter().map(|m| m.name).collect())
    }

    /// List all available models with detailed information.
    ///
    /// # Returns
    ///
    /// A vector of `ModelInfo` structs containing detailed model information.
    pub fn list_models_detailed(&self) -> Result<Vec<ModelInfo>> {
        let url = format!("{}/api/tags", self.base_url);

        let response = self.client
            .get(&url)
            .timeout(Duration::from_secs(CONNECTION_TIMEOUT_SECS))
            .send()
            .map_err(|e| {
                if e.is_connect() {
                    anyhow!(OllamaError::NotRunning(format!(
                        "Cannot connect to Ollama at {}",
                        self.base_url
                    )))
                } else if e.is_timeout() {
                    anyhow!(OllamaError::Timeout(
                        "Connection timed out while listing models".to_string()
                    ))
                } else {
                    anyhow!(OllamaError::NetworkError(e.to_string()))
                }
            })?;

        if !response.status().is_success() {
            return Err(anyhow!(OllamaError::ApiError(format!(
                "Failed to list models: HTTP {}",
                response.status()
            ))));
        }

        let tags: TagsResponse = response
            .json()
            .context("Failed to parse model list response")?;

        Ok(tags.models.into_iter().map(|m| ModelInfo {
            name: m.name,
            size: m.size,
            digest: m.digest,
            modified_at: m.modified_at,
        }).collect())
    }

    /// Pull (download) a model from the Ollama registry.
    ///
    /// Downloads the model with progress updates via a callback function.
    ///
    /// # Arguments
    ///
    /// * `name` - The model name to pull (e.g., "llama3.2:latest", "qwen2.5-coder:7b")
    ///
    /// # Returns
    ///
    /// `Ok(())` if the model was successfully pulled.
    ///
    /// # Errors
    ///
    /// Returns an error if the pull fails or Ollama is not running.
    ///
    /// # Example
    ///
    /// ```no_run
    /// use rigrun::local::OllamaClient;
    ///
    /// let client = OllamaClient::new();
    /// client.pull_model("llama3.2:latest")?;
    /// # Ok::<(), anyhow::Error>(())
    /// ```
    pub fn pull_model(&self, name: &str) -> Result<()> {
        self.pull_model_with_progress(name, |_| {})
    }

    /// Pull (download) a model with progress callback.
    ///
    /// # Arguments
    ///
    /// * `name` - The model name to pull
    /// * `progress_callback` - A function called with progress updates
    ///
    /// # Example
    ///
    /// ```no_run
    /// use rigrun::local::OllamaClient;
    ///
    /// let client = OllamaClient::new();
    /// client.pull_model_with_progress("llama3.2:latest", |progress| {
    ///     if let Some(pct) = progress.percentage() {
    ///         println!("{}: {:.1}%", progress.status, pct);
    ///     } else {
    ///         println!("{}", progress.status);
    ///     }
    /// })?;
    /// # Ok::<(), anyhow::Error>(())
    /// ```
    pub fn pull_model_with_progress<F>(&self, name: &str, mut progress_callback: F) -> Result<()>
    where
        F: FnMut(PullProgress),
    {
        let url = format!("{}/api/pull", self.base_url);

        let request_body = serde_json::json!({
            "name": name,
            "stream": true
        });

        let response = self.client
            .post(&url)
            .json(&request_body)
            .timeout(self.pull_timeout)
            .send()
            .map_err(|e| {
                if e.is_connect() {
                    anyhow!(OllamaError::NotRunning(format!(
                        "Cannot connect to Ollama at {}. Please ensure Ollama is running.",
                        self.base_url
                    )))
                } else if e.is_timeout() {
                    anyhow!(OllamaError::Timeout(
                        "Pull operation timed out. The model may be very large.".to_string()
                    ))
                } else {
                    anyhow!(OllamaError::NetworkError(e.to_string()))
                }
            })?;

        if !response.status().is_success() {
            let status = response.status();
            let error_text = response.text().unwrap_or_default();

            if status.as_u16() == 404 || error_text.contains("not found") {
                return Err(anyhow!(OllamaError::ModelNotFound(name.to_string())));
            }

            return Err(anyhow!(OllamaError::ApiError(format!(
                "Failed to pull model: HTTP {} - {}",
                status, error_text
            ))));
        }

        // Process streaming response
        let body = response.text().context("Failed to read pull response")?;

        for line in body.lines() {
            if line.is_empty() {
                continue;
            }

            if let Ok(progress) = serde_json::from_str::<PullProgress>(line) {
                progress_callback(progress);
            }
        }

        Ok(())
    }

    /// Generate a text completion.
    ///
    /// # Arguments
    ///
    /// * `model` - The model to use for generation
    /// * `prompt` - The prompt text
    ///
    /// # Returns
    ///
    /// An `OllamaResponse` containing the generated text and token statistics.
    ///
    /// # Errors
    ///
    /// Returns an error if the model is not found, Ollama is not running,
    /// or the request fails.
    ///
    /// # Example
    ///
    /// ```no_run
    /// use rigrun::local::OllamaClient;
    ///
    /// let client = OllamaClient::new();
    /// let response = client.generate("llama3.2:latest", "Explain quantum computing")?;
    /// println!("Response: {}", response.response);
    /// println!("Tokens: {} prompt, {} completion", response.prompt_tokens, response.completion_tokens);
    /// # Ok::<(), anyhow::Error>(())
    /// ```
    pub fn generate(&self, model: &str, prompt: &str) -> Result<OllamaResponse> {
        self.generate_with_options(model, prompt, None)
    }

    /// Generate a text completion with additional options.
    ///
    /// # Arguments
    ///
    /// * `model` - The model to use for generation
    /// * `prompt` - The prompt text
    /// * `options` - Optional generation parameters (temperature, top_p, etc.)
    pub fn generate_with_options(
        &self,
        model: &str,
        prompt: &str,
        options: Option<serde_json::Value>,
    ) -> Result<OllamaResponse> {
        let url = format!("{}/api/generate", self.base_url);

        let mut request_body = serde_json::json!({
            "model": model,
            "prompt": prompt,
            "stream": false
        });

        if let Some(opts) = options {
            if let Some(obj) = request_body.as_object_mut() {
                obj.insert("options".to_string(), opts);
            }
        }

        let response = self.client
            .post(&url)
            .json(&request_body)
            .timeout(self.generation_timeout)
            .send()
            .map_err(|e| {
                if e.is_connect() {
                    anyhow!(OllamaError::NotRunning(format!(
                        "Cannot connect to Ollama at {}. Please ensure Ollama is running with: ollama serve",
                        self.base_url
                    )))
                } else if e.is_timeout() {
                    anyhow!(OllamaError::Timeout(format!(
                        "Generation request timed out after {} seconds. Consider using a smaller model or shorter prompt.",
                        self.generation_timeout.as_secs()
                    )))
                } else {
                    anyhow!(OllamaError::NetworkError(e.to_string()))
                }
            })?;

        let status = response.status();

        if !status.is_success() {
            let error_text = response.text().unwrap_or_default();

            // Check for model not found
            if status.as_u16() == 404 || error_text.contains("not found") || error_text.contains("model") {
                return Err(anyhow!(OllamaError::ModelNotFound(format!(
                    "Model '{}' not found. Pull it first with: ollama pull {}",
                    model, model
                ))));
            }

            return Err(anyhow!(OllamaError::ApiError(format!(
                "Generation failed: HTTP {} - {}",
                status, error_text
            ))));
        }

        let gen_response: GenerateResponse = response
            .json()
            .context("Failed to parse generation response")?;

        if let Some(error) = gen_response.error {
            if error.contains("not found") {
                return Err(anyhow!(OllamaError::ModelNotFound(format!(
                    "Model '{}' not found. Pull it first with: ollama pull {}",
                    model, model
                ))));
            }
            return Err(anyhow!(OllamaError::ApiError(error)));
        }

        Ok(OllamaResponse {
            response: gen_response.response.unwrap_or_default(),
            prompt_tokens: gen_response.prompt_eval_count,
            completion_tokens: gen_response.eval_count,
            total_duration_ms: gen_response.total_duration / 1_000_000, // Convert nanoseconds to milliseconds
        })
    }

    /// Perform a chat completion.
    ///
    /// # Arguments
    ///
    /// * `model` - The model to use for chat
    /// * `messages` - A vector of chat messages
    ///
    /// # Returns
    ///
    /// An `OllamaResponse` containing the assistant's reply and token statistics.
    ///
    /// # Errors
    ///
    /// Returns an error if the model is not found, Ollama is not running,
    /// or the request fails.
    ///
    /// # Example
    ///
    /// ```no_run
    /// use rigrun::local::{OllamaClient, Message};
    ///
    /// let client = OllamaClient::new();
    /// let messages = vec![
    ///     Message::system("You are a helpful assistant."),
    ///     Message::user("What is the capital of France?"),
    /// ];
    /// let response = client.chat("llama3.2:latest", messages)?;
    /// println!("Assistant: {}", response.response);
    /// # Ok::<(), anyhow::Error>(())
    /// ```
    pub fn chat(&self, model: &str, messages: Vec<Message>) -> Result<OllamaResponse> {
        self.chat_with_options(model, messages, None)
    }

    /// Perform a chat completion with additional options.
    ///
    /// # Arguments
    ///
    /// * `model` - The model to use for chat
    /// * `messages` - A vector of chat messages
    /// * `options` - Optional generation parameters
    pub fn chat_with_options(
        &self,
        model: &str,
        messages: Vec<Message>,
        options: Option<serde_json::Value>,
    ) -> Result<OllamaResponse> {
        let url = format!("{}/api/chat", self.base_url);

        let mut request_body = serde_json::json!({
            "model": model,
            "messages": messages,
            "stream": false
        });

        if let Some(opts) = options {
            if let Some(obj) = request_body.as_object_mut() {
                obj.insert("options".to_string(), opts);
            }
        }

        let response = self.client
            .post(&url)
            .json(&request_body)
            .timeout(self.generation_timeout)
            .send()
            .map_err(|e| {
                if e.is_connect() {
                    anyhow!(OllamaError::NotRunning(format!(
                        "Cannot connect to Ollama at {}. Please ensure Ollama is running with: ollama serve",
                        self.base_url
                    )))
                } else if e.is_timeout() {
                    anyhow!(OllamaError::Timeout(format!(
                        "Chat request timed out after {} seconds. Consider using a smaller model or shorter conversation.",
                        self.generation_timeout.as_secs()
                    )))
                } else {
                    anyhow!(OllamaError::NetworkError(e.to_string()))
                }
            })?;

        let status = response.status();

        if !status.is_success() {
            let error_text = response.text().unwrap_or_default();

            if status.as_u16() == 404 || error_text.contains("not found") {
                return Err(anyhow!(OllamaError::ModelNotFound(format!(
                    "Model '{}' not found. Pull it first with: ollama pull {}",
                    model, model
                ))));
            }

            return Err(anyhow!(OllamaError::ApiError(format!(
                "Chat failed: HTTP {} - {}",
                status, error_text
            ))));
        }

        let chat_response: ChatResponse = response
            .json()
            .context("Failed to parse chat response")?;

        if let Some(error) = chat_response.error {
            if error.contains("not found") {
                return Err(anyhow!(OllamaError::ModelNotFound(format!(
                    "Model '{}' not found. Pull it first with: ollama pull {}",
                    model, model
                ))));
            }
            return Err(anyhow!(OllamaError::ApiError(error)));
        }

        let response_text = chat_response
            .message
            .map(|m| m.content)
            .unwrap_or_default();

        Ok(OllamaResponse {
            response: response_text,
            prompt_tokens: chat_response.prompt_eval_count,
            completion_tokens: chat_response.eval_count,
            total_duration_ms: chat_response.total_duration / 1_000_000,
        })
    }

    /// Get the base URL of the Ollama client.
    pub fn base_url(&self) -> &str {
        &self.base_url
    }

    /// Perform a chat completion (async version for server use).
    ///
    /// This is an async wrapper around the blocking chat method,
    /// using tokio's spawn_blocking to avoid blocking the async runtime.
    ///
    /// # Arguments
    ///
    /// * `model` - The model to use for chat
    /// * `messages` - A vector of chat messages
    ///
    /// # Returns
    ///
    /// An `OllamaResponse` containing the assistant's reply and token statistics.
    pub async fn chat_async(&self, model: &str, messages: Vec<Message>) -> Result<OllamaResponse> {
        let client = self.clone();
        let model = model.to_string();

        tokio::task::spawn_blocking(move || {
            client.chat(&model, messages)
        })
        .await
        .map_err(|e| anyhow::anyhow!("Task join error: {}", e))?
    }

    /// Check if a specific model is available locally.
    ///
    /// # Arguments
    ///
    /// * `model` - The model name to check
    ///
    /// # Returns
    ///
    /// `true` if the model is available, `false` otherwise.
    pub fn has_model(&self, model: &str) -> Result<bool> {
        let models = self.list_models()?;
        Ok(models.iter().any(|m| m == model || m.starts_with(&format!("{}:", model))))
    }

    /// Ensure a model is available, pulling it if necessary.
    ///
    /// # Arguments
    ///
    /// * `model` - The model name to ensure
    /// * `progress_callback` - Optional callback for pull progress
    ///
    /// # Returns
    ///
    /// `Ok(true)` if the model was pulled, `Ok(false)` if it was already available.
    pub fn ensure_model<F>(&self, model: &str, progress_callback: F) -> Result<bool>
    where
        F: FnMut(PullProgress),
    {
        if self.has_model(model)? {
            Ok(false)
        } else {
            self.pull_model_with_progress(model, progress_callback)?;
            Ok(true)
        }
    }

    /// Perform a chat completion with streaming response.
    ///
    /// # Arguments
    ///
    /// * `model` - The model to use for chat
    /// * `messages` - A vector of chat messages
    /// * `chunk_callback` - A function called with each chunk of the response
    ///
    /// # Returns
    ///
    /// An `OllamaResponse` containing the full response and token statistics.
    ///
    /// # Example
    ///
    /// ```no_run
    /// use rigrun::local::{OllamaClient, Message};
    ///
    /// let client = OllamaClient::new();
    /// let messages = vec![Message::user("Hello!")];
    /// let response = client.chat_stream("llama3.2:latest", messages, |chunk| {
    ///     print!("{}", chunk);
    /// })?;
    /// # Ok::<(), anyhow::Error>(())
    /// ```
    pub fn chat_stream<F>(&self, model: &str, messages: Vec<Message>, mut chunk_callback: F) -> Result<OllamaResponse>
    where
        F: FnMut(&str),
    {
        let url = format!("{}/api/chat", self.base_url);

        let request_body = serde_json::json!({
            "model": model,
            "messages": messages,
            "stream": true
        });

        let response = self.client
            .post(&url)
            .json(&request_body)
            .timeout(self.generation_timeout)
            .send()
            .map_err(|e| {
                if e.is_connect() {
                    anyhow!(OllamaError::NotRunning(format!(
                        "Cannot connect to Ollama at {}. Please ensure Ollama is running with: ollama serve",
                        self.base_url
                    )))
                } else if e.is_timeout() {
                    anyhow!(OllamaError::Timeout(format!(
                        "Chat request timed out after {} seconds.",
                        self.generation_timeout.as_secs()
                    )))
                } else {
                    anyhow!(OllamaError::NetworkError(e.to_string()))
                }
            })?;

        let status = response.status();

        if !status.is_success() {
            let error_text = response.text().unwrap_or_default();

            if status.as_u16() == 404 || error_text.contains("not found") {
                return Err(anyhow!(OllamaError::ModelNotFound(format!(
                    "Model '{}' not found. Pull it first with: ollama pull {}",
                    model, model
                ))));
            }

            return Err(anyhow!(OllamaError::ApiError(format!(
                "Chat failed: HTTP {} - {}",
                status, error_text
            ))));
        }

        // Process streaming response
        let body = response.text().context("Failed to read chat response")?;

        let mut full_response = String::new();
        let mut prompt_tokens = 0;
        let mut completion_tokens = 0;
        let mut total_duration = 0;

        for line in body.lines() {
            if line.is_empty() {
                continue;
            }

            if let Ok(chunk) = serde_json::from_str::<ChatResponse>(line) {
                if let Some(error) = chunk.error {
                    if error.contains("not found") {
                        return Err(anyhow!(OllamaError::ModelNotFound(format!(
                            "Model '{}' not found. Pull it first with: ollama pull {}",
                            model, model
                        ))));
                    }
                    return Err(anyhow!(OllamaError::ApiError(error)));
                }

                if let Some(msg) = chunk.message {
                    chunk_callback(&msg.content);
                    full_response.push_str(&msg.content);
                }

                if chunk.done {
                    prompt_tokens = chunk.prompt_eval_count;
                    completion_tokens = chunk.eval_count;
                    total_duration = chunk.total_duration;
                }
            }
        }

        Ok(OllamaResponse {
            response: full_response,
            prompt_tokens,
            completion_tokens,
            total_duration_ms: total_duration / 1_000_000,
        })
    }
}

/// Convenience function to check if Ollama is running.
///
/// Uses the default Ollama URL (localhost:11434).
pub fn check_ollama_running() -> bool {
    OllamaClient::new().check_ollama_running()
}

/// Convenience function to list available models.
///
/// Uses the default Ollama URL (localhost:11434).
pub fn list_models() -> Result<Vec<String>> {
    OllamaClient::new().list_models()
}

/// Convenience function to pull a model.
///
/// Uses the default Ollama URL (localhost:11434).
pub fn pull_model(name: &str) -> Result<()> {
    OllamaClient::new().pull_model(name)
}

/// Convenience function to generate text.
///
/// Uses the default Ollama URL (localhost:11434).
pub fn generate(model: &str, prompt: &str) -> Result<OllamaResponse> {
    OllamaClient::new().generate(model, prompt)
}

/// Convenience function for chat completion.
///
/// Uses the default Ollama URL (localhost:11434).
pub fn chat(model: &str, messages: Vec<Message>) -> Result<OllamaResponse> {
    OllamaClient::new().chat(model, messages)
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
        assert_eq!(user.content, "Hello!");

        let assistant = Message::assistant("Hi there!");
        assert_eq!(assistant.role, "assistant");
        assert_eq!(assistant.content, "Hi there!");
    }

    #[test]
    fn test_pull_progress_percentage() {
        let progress = PullProgress {
            status: "downloading".to_string(),
            digest: Some("sha256:abc123".to_string()),
            total: Some(1000),
            completed: Some(500),
        };
        assert_eq!(progress.percentage(), Some(50.0));

        let progress_no_total = PullProgress {
            status: "verifying".to_string(),
            digest: None,
            total: None,
            completed: None,
        };
        assert_eq!(progress_no_total.percentage(), None);
    }

    #[test]
    fn test_ollama_client_url_normalization() {
        let client = OllamaClient::with_url("http://localhost:11434/");
        assert_eq!(client.base_url(), "http://localhost:11434");

        let client2 = OllamaClient::with_url("http://localhost:11434");
        assert_eq!(client2.base_url(), "http://localhost:11434");
    }

    #[test]
    fn test_ollama_error_display() {
        let err = OllamaError::NotRunning("test".to_string());
        assert!(err.to_string().contains("not running"));

        let err = OllamaError::ModelNotFound("llama3".to_string());
        assert!(err.to_string().contains("llama3"));

        let err = OllamaError::Timeout("test".to_string());
        assert!(err.to_string().contains("timed out"));
    }

    #[test]
    fn test_ollama_response_default() {
        let response = OllamaResponse::default();
        assert!(response.response.is_empty());
        assert_eq!(response.prompt_tokens, 0);
        assert_eq!(response.completion_tokens, 0);
        assert_eq!(response.total_duration_ms, 0);
    }
}
