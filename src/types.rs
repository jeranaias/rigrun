// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

//! Canonical types used across rigrun.
//!
//! This module provides unified type definitions to avoid duplication.

use serde::{Deserialize, Serialize};

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
