// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! Utility functions for rigrun.
//!
//! This module provides common utility functions used across the codebase.

/// Mask a sensitive string (like API keys) for logging.
///
/// Shows only the first `visible_prefix` characters and replaces the rest with "...".
/// This ensures API keys are never logged in full.
///
/// # Examples
///
/// ```
/// use rigrun::utils::mask_sensitive;
///
/// let api_key = "sk-or-v1-abcdefghijklmnopqrstuvwxyz123456";
/// let masked = mask_sensitive(&api_key, 8);
/// assert_eq!(masked, "sk-or-v1...");
/// ```
pub fn mask_sensitive(input: &str, visible_prefix: usize) -> String {
    if input.len() <= visible_prefix {
        // If it's shorter than the visible prefix, still mask it to avoid leaking length
        return format!("{}...", input);
    }

    let prefix: String = input.chars().take(visible_prefix).collect();
    format!("{}...", prefix)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_mask_sensitive() {
        assert_eq!(mask_sensitive("sk-or-v1-secret123", 8), "sk-or-v1...");
        assert_eq!(mask_sensitive("short", 8), "short...");
        assert_eq!(mask_sensitive("", 8), "...");
    }
}
