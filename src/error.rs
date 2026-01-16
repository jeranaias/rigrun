// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

//! Consistent error formatting for rigrun.
//!
//! Provides utilities to format errors with actionable information including
//! possible causes, suggested fixes, and documentation links.

use std::fmt;

/// GitHub issues URL for support.
pub const GITHUB_ISSUES_URL: &str = "https://github.com/jeranaias/rigrun/issues";

/// Formats an error message with title, causes, fixes, and help link.
///
/// # Arguments
///
/// * `title` - The error title (e.g., "Failed to connect to Ollama")
/// * `causes` - List of possible causes
/// * `fixes` - List of suggested fixes (should be actionable commands or steps)
///
/// # Example
///
/// ```
/// use rigrun::error::format_error;
///
/// let error = format_error(
///     "Failed to connect to Ollama",
///     &[
///         "Ollama service not running",
///         "Ollama installed but not started",
///         "Wrong Ollama URL in config",
///     ],
///     &[
///         "Start Ollama: ollama serve",
///         "Check status: rigrun doctor",
///         "Verify URL: rigrun config show",
///     ],
/// );
/// println!("{}", error);
/// ```
pub fn format_error(title: &str, causes: &[&str], fixes: &[&str]) -> String {
    let mut output = String::new();

    // Error title
    output.push_str(&format!("[✗] {}\n\n", title));

    // Possible causes
    if !causes.is_empty() {
        output.push_str("Possible causes:\n");
        for cause in causes {
            output.push_str(&format!("  - {}\n", cause));
        }
        output.push('\n');
    }

    // Suggested fixes
    if !fixes.is_empty() {
        output.push_str("Try these fixes:\n");
        for (i, fix) in fixes.iter().enumerate() {
            output.push_str(&format!("  {}. {}\n", i + 1, fix));
        }
        output.push('\n');
    }

    // Help link
    output.push_str(&format!("Need help? {}", GITHUB_ISSUES_URL));

    output
}

/// Formats a simple error with just a title and help link.
pub fn format_simple_error(title: &str) -> String {
    format!("[✗] {}\n\nNeed help? {}", title, GITHUB_ISSUES_URL)
}

/// Builder for constructing formatted error messages.
///
/// # Example
///
/// ```
/// use rigrun::error::ErrorBuilder;
///
/// let error = ErrorBuilder::new("Failed to connect to Ollama")
///     .cause("Ollama service not running")
///     .cause("Ollama installed but not started")
///     .fix("Start Ollama: ollama serve")
///     .fix("Check status: rigrun doctor")
///     .build();
/// println!("{}", error);
/// ```
#[derive(Debug, Clone)]
pub struct ErrorBuilder {
    title: String,
    causes: Vec<String>,
    fixes: Vec<String>,
}

impl ErrorBuilder {
    /// Create a new error builder with the given title.
    pub fn new(title: impl Into<String>) -> Self {
        Self {
            title: title.into(),
            causes: Vec::new(),
            fixes: Vec::new(),
        }
    }

    /// Add a possible cause.
    pub fn cause(mut self, cause: impl Into<String>) -> Self {
        self.causes.push(cause.into());
        self
    }

    /// Add a suggested fix.
    pub fn fix(mut self, fix: impl Into<String>) -> Self {
        self.fixes.push(fix.into());
        self
    }

    /// Build the formatted error message.
    pub fn build(self) -> String {
        let causes: Vec<&str> = self.causes.iter().map(|s| s.as_str()).collect();
        let fixes: Vec<&str> = self.fixes.iter().map(|s| s.as_str()).collect();
        format_error(&self.title, &causes, &fixes)
    }
}

impl fmt::Display for ErrorBuilder {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.clone().build())
    }
}

/// Macro to quickly create formatted errors.
///
/// # Examples
///
/// ```
/// use rigrun::error_msg;
///
/// let error = error_msg!(
///     "Failed to connect to Ollama",
///     causes: [
///         "Ollama service not running",
///         "Wrong URL configured",
///     ],
///     fixes: [
///         "Start Ollama: ollama serve",
///         "Check config: rigrun config show",
///     ]
/// );
/// ```
#[macro_export]
macro_rules! error_msg {
    ($title:expr, causes: [$($cause:expr),* $(,)?], fixes: [$($fix:expr),* $(,)?]) => {
        $crate::error::format_error(
            $title,
            &[$($cause),*],
            &[$($fix),*]
        )
    };
    ($title:expr, causes: [$($cause:expr),* $(,)?]) => {
        {
            let causes = vec![$($cause),*];
            let mut output = format!("[✗] {}\n\n", $title);
            if !causes.is_empty() {
                output.push_str("Possible causes:\n");
                for cause in &causes {
                    output.push_str(&format!("  - {}\n", cause));
                }
                output.push('\n');
            }
            output.push_str(&format!("Need help? {}", $crate::error::GITHUB_ISSUES_URL));
            output
        }
    };
    ($title:expr, fixes: [$($fix:expr),* $(,)?]) => {
        {
            let fixes = vec![$($fix),*];
            let mut output = format!("[✗] {}\n\n", $title);
            if !fixes.is_empty() {
                output.push_str("Try these fixes:\n");
                for (i, fix) in fixes.iter().enumerate() {
                    output.push_str(&format!("  {}. {}\n", i + 1, fix));
                }
                output.push('\n');
            }
            output.push_str(&format!("Need help? {}", $crate::error::GITHUB_ISSUES_URL));
            output
        }
    };
    ($title:expr) => {
        $crate::error::format_simple_error($title)
    };
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_format_error() {
        let error = format_error(
            "Test Error",
            &["Cause 1", "Cause 2"],
            &["Fix 1", "Fix 2"],
        );

        assert!(error.contains("[✗] Test Error"));
        assert!(error.contains("Possible causes:"));
        assert!(error.contains("  - Cause 1"));
        assert!(error.contains("  - Cause 2"));
        assert!(error.contains("Try these fixes:"));
        assert!(error.contains("  1. Fix 1"));
        assert!(error.contains("  2. Fix 2"));
        assert!(error.contains(GITHUB_ISSUES_URL));
    }

    #[test]
    fn test_format_simple_error() {
        let error = format_simple_error("Simple error");
        assert!(error.contains("[✗] Simple error"));
        assert!(error.contains(GITHUB_ISSUES_URL));
    }

    #[test]
    fn test_error_builder() {
        let error = ErrorBuilder::new("Builder test")
            .cause("Test cause")
            .fix("Test fix")
            .build();

        assert!(error.contains("[✗] Builder test"));
        assert!(error.contains("Test cause"));
        assert!(error.contains("Test fix"));
    }

    #[test]
    fn test_error_builder_display() {
        let builder = ErrorBuilder::new("Display test")
            .cause("Cause")
            .fix("Fix");

        let error = format!("{}", builder);
        assert!(error.contains("[✗] Display test"));
    }

    #[test]
    fn test_empty_causes_and_fixes() {
        let error = format_error("Empty test", &[], &[]);
        assert!(error.contains("[✗] Empty test"));
        assert!(!error.contains("Possible causes:"));
        assert!(!error.contains("Try these fixes:"));
        assert!(error.contains(GITHUB_ISSUES_URL));
    }
}
