// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! Canonical types used across rigrun.
//!
//! This module provides unified type definitions to avoid duplication.

use serde::{Deserialize, Serialize};

// ============================================================================
// STREAMING CONFIGURATION
// ============================================================================

/// Configuration for streaming responses.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StreamingConfig {
    /// Whether streaming is enabled.
    pub enabled: bool,
    /// Number of tokens to buffer before sending (1 = immediate).
    pub buffer_size: usize,
    /// Timeout for first token in milliseconds (default: 30000).
    pub first_token_timeout_ms: u64,
    /// Timeout between tokens in milliseconds (default: 5000).
    pub token_timeout_ms: u64,
}

impl Default for StreamingConfig {
    fn default() -> Self {
        Self {
            enabled: true,
            buffer_size: 1,  // Immediate token delivery
            first_token_timeout_ms: 30000,  // 30 seconds for first token
            token_timeout_ms: 5000,  // 5 seconds between tokens
        }
    }
}

/// A single token/chunk from a streaming response.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StreamChunk {
    /// The token text.
    pub token: String,
    /// Whether this is the final chunk.
    pub done: bool,
    /// Token count so far (if available).
    pub tokens_so_far: Option<u32>,
}

impl StreamChunk {
    /// Create a new token chunk.
    pub fn token(text: impl Into<String>) -> Self {
        Self {
            token: text.into(),
            done: false,
            tokens_so_far: None,
        }
    }

    /// Create a final/done chunk.
    pub fn done() -> Self {
        Self {
            token: String::new(),
            done: true,
            tokens_so_far: None,
        }
    }

    /// Create a done chunk with token count.
    pub fn done_with_count(tokens: u32) -> Self {
        Self {
            token: String::new(),
            done: true,
            tokens_so_far: Some(tokens),
        }
    }
}

/// Result of a streaming operation that was interrupted.
#[derive(Debug, Clone)]
pub struct PartialResponse {
    /// The partial response text accumulated so far.
    pub text: String,
    /// Number of tokens received before interruption.
    pub tokens_received: u32,
    /// Reason for interruption.
    pub reason: InterruptReason,
}

/// Reason why a streaming response was interrupted.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum InterruptReason {
    /// User pressed Ctrl+C or cancelled.
    UserCancel,
    /// Connection was dropped.
    ConnectionDropped,
    /// Timeout waiting for next token.
    Timeout,
    /// Model returned an error mid-stream.
    ModelError,
}

/// A chat message with role and content.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Message {
    pub role: String,
    pub content: String,
}

impl Message {
    pub fn new(role: impl Into<String>, content: impl Into<String>) -> Self {
        Self {
            role: role.into(),
            content: content.into(),
        }
    }

    pub fn user(content: impl Into<String>) -> Self {
        Self::new("user", content)
    }

    pub fn assistant(content: impl Into<String>) -> Self {
        Self::new("assistant", content)
    }

    pub fn system(content: impl Into<String>) -> Self {
        Self::new("system", content)
    }
}

/// Model tier for routing decisions.
/// Ordered by cost/capability: Cache < Local < Haiku < Cloud < Gpt4o < Sonnet < Opus
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
pub enum Tier {
    /// Cached response (free, instant)
    Cache,
    /// Local Ollama model
    Local,
    /// Claude Haiku (fast, cheap)
    Haiku,
    /// Cloud model (generic)
    Cloud,
    /// OpenAI GPT-4o
    Gpt4o,
    /// Claude Sonnet (balanced)
    Sonnet,
    /// Claude Opus (powerful)
    Opus,
}

impl Tier {
    /// Convert tier to string representation.
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::Cache => "cache",
            Self::Local => "local",
            Self::Cloud => "cloud",
            Self::Haiku => "haiku",
            Self::Sonnet => "sonnet",
            Self::Opus => "opus",
            Self::Gpt4o => "gpt4o",
        }
    }

    /// Calculate cost in millicents (1/1000 of a cent) for this tier.
    pub fn calculate_cost(&self, prompt_tokens: u32, completion_tokens: u32) -> u64 {
        match self {
            Tier::Cache | Tier::Local => 0,
            Tier::Cloud => {
                // OpenRouter auto pricing (estimated)
                let input_cost = (prompt_tokens as u64 * 50) / 1_000_000;
                let output_cost = (completion_tokens as u64 * 150) / 1_000_000;
                input_cost + output_cost
            }
            Tier::Haiku => {
                let input_cost = (prompt_tokens as u64 * 25) / 1_000_000;
                let output_cost = (completion_tokens as u64 * 125) / 1_000_000;
                input_cost + output_cost
            }
            Tier::Sonnet => {
                let input_cost = (prompt_tokens as u64 * 300) / 1_000_000;
                let output_cost = (completion_tokens as u64 * 1500) / 1_000_000;
                input_cost + output_cost
            }
            Tier::Opus => {
                let input_cost = (prompt_tokens as u64 * 1500) / 1_000_000;
                let output_cost = (completion_tokens as u64 * 7500) / 1_000_000;
                input_cost + output_cost
            }
            Tier::Gpt4o => {
                let input_cost = (prompt_tokens as u64 * 250) / 1_000_000;
                let output_cost = (completion_tokens as u64 * 1000) / 1_000_000;
                input_cost + output_cost
            }
        }
    }
}
