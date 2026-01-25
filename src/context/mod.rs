// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! Context Mention System for rigrun CLI
//!
//! Provides @ mention support for including context in queries, similar to Continue.dev.
//!
//! # Supported Mentions
//!
//! - `@file:path/to/file.rs` - Include file contents in context
//! - `@codebase` - Include codebase summary/structure
//! - `@git` - Include recent git history/diff
//! - `@git:HEAD~3` - Include specific git range
//! - `@error` - Include last error output
//! - `@clipboard` - Include clipboard contents
//!
//! # Example
//!
//! ```no_run
//! use rigrun::context::{parse_mentions, fetch_context, format_context_header};
//!
//! let input = "@file:src/main.rs Can you explain this code?";
//! let (mentions, query) = parse_mentions(input);
//!
//! for mention in &mentions {
//!     if let Ok(content) = fetch_context(mention) {
//!         println!("{}", format_context_header(mention, content.lines().count()));
//!         println!("{}", content);
//!     }
//! }
//! ```

use std::collections::HashSet;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::LazyLock;
use anyhow::{anyhow, Context, Result};
use regex::Regex;

// IL5: Static regex patterns - compiled once at first use, never panic at runtime
static FILE_PATTERN: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r#"@file:(?:"([^"]+)"|'([^']+)'|(\S+))"#)
        .expect("FILE_PATTERN is a valid regex")
});

static GIT_PATTERN: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"@git(?::(\S+))?")
        .expect("GIT_PATTERN is a valid regex")
});

static CODEBASE_PATTERN: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"@codebase\b")
        .expect("CODEBASE_PATTERN is a valid regex")
});

static ERROR_PATTERN: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"@error\b")
        .expect("ERROR_PATTERN is a valid regex")
});

static CLIPBOARD_PATTERN: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"@clipboard\b")
        .expect("CLIPBOARD_PATTERN is a valid regex")
});

/// Types of context that can be mentioned with @ syntax
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub enum ContextMention {
    /// Include a specific file's contents
    File { path: PathBuf },
    /// Include codebase summary/structure
    Codebase,
    /// Include git history/diff
    Git { range: Option<String> },
    /// Include last error output
    Error,
    /// Include clipboard contents
    Clipboard,
}

impl std::fmt::Display for ContextMention {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ContextMention::File { path } => write!(f, "@file:{}", path.display()),
            ContextMention::Codebase => write!(f, "@codebase"),
            ContextMention::Git { range: None } => write!(f, "@git"),
            ContextMention::Git { range: Some(r) } => write!(f, "@git:{}", r),
            ContextMention::Error => write!(f, "@error"),
            ContextMention::Clipboard => write!(f, "@clipboard"),
        }
    }
}

/// Available mention types for tab completion
pub const MENTION_TYPES: &[&str] = &[
    "@file:",
    "@codebase",
    "@git",
    "@git:",
    "@error",
    "@clipboard",
];

/// Last error storage for @error mention
static LAST_ERROR: std::sync::RwLock<Option<String>> = std::sync::RwLock::new(None);

/// Store the last error for @error mention
pub fn store_last_error(error: &str) {
    if let Ok(mut guard) = LAST_ERROR.write() {
        *guard = Some(error.to_string());
    }
}

/// Clear the last error
pub fn clear_last_error() {
    if let Ok(mut guard) = LAST_ERROR.write() {
        *guard = None;
    }
}

/// Get the last stored error
pub fn get_last_error() -> Option<String> {
    LAST_ERROR.read().ok().and_then(|guard| guard.clone())
}

/// Parse @ mentions from user input
///
/// Returns a tuple of (mentions found, remaining query text)
///
/// # Example
///
/// ```
/// use rigrun::context::parse_mentions;
///
/// let (mentions, query) = parse_mentions("@file:src/main.rs @git Explain this code");
/// assert_eq!(mentions.len(), 2);
/// assert_eq!(query, "Explain this code");
/// ```
pub fn parse_mentions(input: &str) -> (Vec<ContextMention>, String) {
    let mut mentions = Vec::new();
    let mut seen = HashSet::new();
    let mut remaining = input.to_string();

    // IL5: Use static patterns - never panic at runtime

    // Extract @file mentions
    for cap in FILE_PATTERN.captures_iter(input) {
        let path_str = cap.get(1)
            .or_else(|| cap.get(2))
            .or_else(|| cap.get(3))
            .map(|m| m.as_str())
            .unwrap_or("");

        let mention = ContextMention::File {
            path: PathBuf::from(path_str)
        };

        if !seen.contains(&mention) {
            seen.insert(mention.clone());
            mentions.push(mention);
        }

        remaining = FILE_PATTERN.replace(&remaining, "").to_string();
    }

    // Extract @git mentions
    for cap in GIT_PATTERN.captures_iter(input) {
        let range = cap.get(1).map(|m| m.as_str().to_string());
        let mention = ContextMention::Git { range };

        if !seen.contains(&mention) {
            seen.insert(mention.clone());
            mentions.push(mention);
        }
    }
    remaining = GIT_PATTERN.replace_all(&remaining, "").to_string();

    // Extract @codebase mentions
    if CODEBASE_PATTERN.is_match(input) {
        let mention = ContextMention::Codebase;
        if !seen.contains(&mention) {
            seen.insert(mention.clone());
            mentions.push(mention);
        }
        remaining = CODEBASE_PATTERN.replace_all(&remaining, "").to_string();
    }

    // Extract @error mentions
    if ERROR_PATTERN.is_match(input) {
        let mention = ContextMention::Error;
        if !seen.contains(&mention) {
            seen.insert(mention.clone());
            mentions.push(mention);
        }
        remaining = ERROR_PATTERN.replace_all(&remaining, "").to_string();
    }

    // Extract @clipboard mentions
    if CLIPBOARD_PATTERN.is_match(input) {
        let mention = ContextMention::Clipboard;
        if !seen.contains(&mention) {
            seen.insert(mention.clone());
            mentions.push(mention);
        }
        remaining = CLIPBOARD_PATTERN.replace_all(&remaining, "").to_string();
    }

    // Clean up remaining text
    let query = remaining
        .split_whitespace()
        .collect::<Vec<_>>()
        .join(" ");

    (mentions, query)
}

/// Fetch the context content for a mention
///
/// Returns the content string that should be included in the query context.
pub fn fetch_context(mention: &ContextMention) -> Result<String> {
    match mention {
        ContextMention::File { path } => fetch_file_context(path),
        ContextMention::Codebase => fetch_codebase_context(),
        ContextMention::Git { range } => fetch_git_context(range.as_deref()),
        ContextMention::Error => fetch_error_context(),
        ContextMention::Clipboard => fetch_clipboard_context(),
    }
}

/// Fetch file contents
fn fetch_file_context(path: &Path) -> Result<String> {
    // Try to resolve relative paths from current directory
    let resolved_path = if path.is_absolute() {
        path.to_path_buf()
    } else {
        std::env::current_dir()
            .unwrap_or_else(|_| PathBuf::from("."))
            .join(path)
    };

    if !resolved_path.exists() {
        return Err(anyhow!("File not found: {}", resolved_path.display()));
    }

    if !resolved_path.is_file() {
        return Err(anyhow!("Not a file: {}", resolved_path.display()));
    }

    // Check file size (limit to 100KB to avoid memory issues)
    let metadata = std::fs::metadata(&resolved_path)
        .context("Failed to read file metadata")?;

    if metadata.len() > 100 * 1024 {
        return Err(anyhow!(
            "File too large: {} ({} bytes, max 100KB)",
            resolved_path.display(),
            metadata.len()
        ));
    }

    let content = std::fs::read_to_string(&resolved_path)
        .context(format!("Failed to read file: {}", resolved_path.display()))?;

    Ok(content)
}

/// Fetch codebase structure/summary
fn fetch_codebase_context() -> Result<String> {
    let cwd = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));

    let mut summary = String::new();
    summary.push_str(&format!("# Codebase Summary: {}\n\n", cwd.display()));

    // Try to get project type from common files
    let project_type = detect_project_type(&cwd);
    if !project_type.is_empty() {
        summary.push_str(&format!("Project Type: {}\n\n", project_type));
    }

    // Get directory structure (limited depth)
    summary.push_str("## Directory Structure\n\n```\n");
    let tree = get_directory_tree(&cwd, 3)?;
    summary.push_str(&tree);
    summary.push_str("```\n\n");

    // Try to get file counts by type
    let file_stats = get_file_statistics(&cwd);
    if !file_stats.is_empty() {
        summary.push_str("## File Statistics\n\n");
        for (ext, count) in file_stats {
            summary.push_str(&format!("- {}: {} files\n", ext, count));
        }
        summary.push('\n');
    }

    // Include key configuration files if present
    let config_files = ["Cargo.toml", "package.json", "pyproject.toml", "go.mod", "Makefile"];
    for config in config_files {
        let config_path = cwd.join(config);
        if config_path.exists() {
            if let Ok(content) = std::fs::read_to_string(&config_path) {
                // Limit config file content
                let truncated = if content.len() > 2000 {
                    format!("{}...\n(truncated)", &content[..2000])
                } else {
                    content
                };
                summary.push_str(&format!("## {}\n\n```\n{}\n```\n\n", config, truncated));
            }
        }
    }

    Ok(summary)
}

/// Detect project type from common files
fn detect_project_type(dir: &Path) -> String {
    let mut types = Vec::new();

    if dir.join("Cargo.toml").exists() {
        types.push("Rust");
    }
    if dir.join("package.json").exists() {
        types.push("Node.js/JavaScript");
    }
    if dir.join("pyproject.toml").exists() || dir.join("setup.py").exists() {
        types.push("Python");
    }
    if dir.join("go.mod").exists() {
        types.push("Go");
    }
    if dir.join("pom.xml").exists() || dir.join("build.gradle").exists() {
        types.push("Java");
    }
    if dir.join("CMakeLists.txt").exists() {
        types.push("C/C++");
    }

    types.join(", ")
}

/// Get a limited directory tree
fn get_directory_tree(dir: &Path, max_depth: usize) -> Result<String> {
    let mut result = String::new();
    build_tree(&mut result, dir, "", max_depth, 0)?;
    Ok(result)
}

fn build_tree(
    result: &mut String,
    dir: &Path,
    prefix: &str,
    max_depth: usize,
    current_depth: usize,
) -> Result<()> {
    if current_depth >= max_depth {
        return Ok(());
    }

    let mut entries: Vec<_> = std::fs::read_dir(dir)
        .context("Failed to read directory")?
        .filter_map(|e| e.ok())
        .collect();

    // Sort entries: directories first, then files
    entries.sort_by(|a, b| {
        let a_is_dir = a.path().is_dir();
        let b_is_dir = b.path().is_dir();
        match (a_is_dir, b_is_dir) {
            (true, false) => std::cmp::Ordering::Less,
            (false, true) => std::cmp::Ordering::Greater,
            _ => a.file_name().cmp(&b.file_name()),
        }
    });

    // Filter out hidden files and common ignore patterns
    let ignore_patterns = [
        "node_modules", "target", ".git", "__pycache__", ".venv",
        "venv", "dist", "build", ".idea", ".vscode", "coverage",
    ];

    let filtered: Vec<_> = entries
        .into_iter()
        .filter(|e| {
            let name = e.file_name();
            let name_str = name.to_string_lossy();
            !name_str.starts_with('.') && !ignore_patterns.contains(&name_str.as_ref())
        })
        .take(20) // Limit entries per directory
        .collect();

    let count = filtered.len();
    for (i, entry) in filtered.into_iter().enumerate() {
        let is_last = i == count - 1;
        let connector = if is_last { "`-- " } else { "|-- " };
        let child_prefix = if is_last { "    " } else { "|   " };

        let name = entry.file_name();
        let path = entry.path();

        if path.is_dir() {
            result.push_str(&format!("{}{}{}/\n", prefix, connector, name.to_string_lossy()));
            build_tree(
                result,
                &path,
                &format!("{}{}", prefix, child_prefix),
                max_depth,
                current_depth + 1,
            )?;
        } else {
            result.push_str(&format!("{}{}{}\n", prefix, connector, name.to_string_lossy()));
        }
    }

    Ok(())
}

/// Get file statistics by extension
fn get_file_statistics(dir: &Path) -> Vec<(String, usize)> {
    use std::collections::HashMap;
    let mut stats: HashMap<String, usize> = HashMap::new();

    fn walk_dir(dir: &Path, stats: &mut HashMap<String, usize>, depth: usize) {
        if depth > 5 {
            return; // Limit recursion depth
        }

        let ignore_dirs = [
            "node_modules", "target", ".git", "__pycache__", ".venv",
            "venv", "dist", "build", ".idea", ".vscode",
        ];

        if let Ok(entries) = std::fs::read_dir(dir) {
            for entry in entries.filter_map(|e| e.ok()) {
                let path = entry.path();
                let name = entry.file_name();
                let name_str = name.to_string_lossy();

                if name_str.starts_with('.') {
                    continue;
                }

                if path.is_dir() {
                    if !ignore_dirs.contains(&name_str.as_ref()) {
                        walk_dir(&path, stats, depth + 1);
                    }
                } else if let Some(ext) = path.extension() {
                    let ext_str = ext.to_string_lossy().to_lowercase();
                    *stats.entry(ext_str).or_insert(0) += 1;
                }
            }
        }
    }

    walk_dir(dir, &mut stats, 0);

    let mut sorted: Vec<_> = stats.into_iter().collect();
    sorted.sort_by(|a, b| b.1.cmp(&a.1));
    sorted.truncate(10); // Top 10 extensions
    sorted
}

/// Fetch git context (history, diff, status)
fn fetch_git_context(range: Option<&str>) -> Result<String> {
    // Check if we're in a git repo
    let git_check = Command::new("git")
        .args(["rev-parse", "--is-inside-work-tree"])
        .output();

    if git_check.is_err() || !git_check.unwrap().status.success() {
        return Err(anyhow!("Not in a git repository"));
    }

    let mut context = String::new();
    context.push_str("# Git Context\n\n");

    // Get current branch
    if let Ok(output) = Command::new("git")
        .args(["branch", "--show-current"])
        .output()
    {
        if output.status.success() {
            let branch = String::from_utf8_lossy(&output.stdout);
            context.push_str(&format!("**Current Branch:** {}\n\n", branch.trim()));
        }
    }

    // Get git status
    context.push_str("## Status\n\n```\n");
    if let Ok(output) = Command::new("git")
        .args(["status", "--short"])
        .output()
    {
        if output.status.success() {
            let status = String::from_utf8_lossy(&output.stdout);
            if status.is_empty() {
                context.push_str("(working tree clean)\n");
            } else {
                context.push_str(&status);
            }
        }
    }
    context.push_str("```\n\n");

    // Get recent commits or specified range
    let log_args = if let Some(r) = range {
        vec!["log", "--oneline", "-n", "10", r]
    } else {
        vec!["log", "--oneline", "-n", "5"]
    };

    context.push_str("## Recent Commits\n\n```\n");
    if let Ok(output) = Command::new("git").args(&log_args).output() {
        if output.status.success() {
            context.push_str(&String::from_utf8_lossy(&output.stdout));
        }
    }
    context.push_str("```\n\n");

    // Get diff
    let diff_args = if let Some(r) = range {
        vec!["diff", r]
    } else {
        vec!["diff", "HEAD"]
    };

    context.push_str("## Diff\n\n```diff\n");
    if let Ok(output) = Command::new("git").args(&diff_args).output() {
        if output.status.success() {
            let diff = String::from_utf8_lossy(&output.stdout);
            // Limit diff size
            if diff.len() > 5000 {
                context.push_str(&diff[..5000]);
                context.push_str("\n... (truncated)\n");
            } else if diff.is_empty() {
                context.push_str("(no changes)\n");
            } else {
                context.push_str(&diff);
            }
        }
    }
    context.push_str("```\n");

    Ok(context)
}

/// Fetch last error context
fn fetch_error_context() -> Result<String> {
    get_last_error().ok_or_else(|| anyhow!("No error stored. Run a command that produces an error first."))
}

/// Fetch clipboard contents
fn fetch_clipboard_context() -> Result<String> {
    // Try different methods based on platform
    #[cfg(target_os = "windows")]
    {
        // Use PowerShell to get clipboard on Windows
        let output = Command::new("powershell")
            .args(["-Command", "Get-Clipboard"])
            .output()
            .context("Failed to access clipboard via PowerShell")?;

        if output.status.success() {
            let content = String::from_utf8_lossy(&output.stdout).to_string();
            if content.trim().is_empty() {
                return Err(anyhow!("Clipboard is empty"));
            }
            return Ok(content);
        }
    }

    #[cfg(target_os = "macos")]
    {
        let output = Command::new("pbpaste")
            .output()
            .context("Failed to access clipboard via pbpaste")?;

        if output.status.success() {
            let content = String::from_utf8_lossy(&output.stdout).to_string();
            if content.is_empty() {
                return Err(anyhow!("Clipboard is empty"));
            }
            return Ok(content);
        }
    }

    #[cfg(target_os = "linux")]
    {
        // Try xclip first, then xsel
        let output = Command::new("xclip")
            .args(["-selection", "clipboard", "-o"])
            .output()
            .or_else(|_| {
                Command::new("xsel")
                    .args(["--clipboard", "--output"])
                    .output()
            })
            .context("Failed to access clipboard. Install xclip or xsel.")?;

        if output.status.success() {
            let content = String::from_utf8_lossy(&output.stdout).to_string();
            if content.is_empty() {
                return Err(anyhow!("Clipboard is empty"));
            }
            return Ok(content);
        }
    }

    Err(anyhow!("Could not access clipboard on this platform"))
}

/// Format a context header for display
///
/// # Example
///
/// ```
/// use rigrun::context::{ContextMention, format_context_header};
/// use std::path::PathBuf;
///
/// let mention = ContextMention::File { path: PathBuf::from("src/main.rs") };
/// let header = format_context_header(&mention, 142);
/// assert!(header.contains("src/main.rs"));
/// assert!(header.contains("142 lines"));
/// ```
pub fn format_context_header(mention: &ContextMention, line_count: usize) -> String {
    match mention {
        ContextMention::File { path } => {
            format!("[Including {} ({} lines)]", path.display(), line_count)
        }
        ContextMention::Codebase => {
            format!("[Including codebase summary ({} lines)]", line_count)
        }
        ContextMention::Git { range: None } => {
            format!("[Including git context ({} lines)]", line_count)
        }
        ContextMention::Git { range: Some(r) } => {
            format!("[Including git {} ({} lines)]", r, line_count)
        }
        ContextMention::Error => {
            format!("[Including last error ({} lines)]", line_count)
        }
        ContextMention::Clipboard => {
            format!("[Including clipboard ({} lines)]", line_count)
        }
    }
}

/// Build context prefix from mentions for prepending to a query
///
/// This function fetches all context and formats it as a system message prefix.
pub fn build_context_prefix(mentions: &[ContextMention]) -> (String, Vec<String>) {
    let mut context_parts = Vec::new();
    let mut messages = Vec::new();

    for mention in mentions {
        match fetch_context(mention) {
            Ok(content) => {
                let line_count = content.lines().count();
                let header = format_context_header(mention, line_count);
                messages.push(header);

                // Format the context section
                let section = match mention {
                    ContextMention::File { path } => {
                        format!(
                            "--- File: {} ---\n{}\n--- End of {} ---\n",
                            path.display(),
                            content,
                            path.display()
                        )
                    }
                    ContextMention::Codebase => {
                        format!("--- Codebase Summary ---\n{}\n--- End of Codebase Summary ---\n", content)
                    }
                    ContextMention::Git { range } => {
                        let label = range.as_deref().unwrap_or("recent");
                        format!("--- Git Context ({}) ---\n{}\n--- End of Git Context ---\n", label, content)
                    }
                    ContextMention::Error => {
                        format!("--- Last Error ---\n{}\n--- End of Last Error ---\n", content)
                    }
                    ContextMention::Clipboard => {
                        format!("--- Clipboard Contents ---\n{}\n--- End of Clipboard ---\n", content)
                    }
                };
                context_parts.push(section);
            }
            Err(e) => {
                messages.push(format!("[Error fetching {}]: {}", mention, e));
            }
        }
    }

    let prefix = if context_parts.is_empty() {
        String::new()
    } else {
        format!(
            "The user has provided the following context:\n\n{}\n\nPlease use this context to answer the user's question.\n\n",
            context_parts.join("\n")
        )
    };

    (prefix, messages)
}

// ============================================================================
// TAB COMPLETION SUPPORT
// ============================================================================

/// Get tab completion suggestions for a partial @ mention
///
/// # Arguments
///
/// * `partial` - The partial input to complete (e.g., "@f" or "@file:src/")
///
/// # Returns
///
/// A vector of completion suggestions
pub fn get_completions(partial: &str) -> Vec<String> {
    if !partial.starts_with('@') {
        return vec![];
    }

    // If it's a file mention with path, complete the path
    if partial.starts_with("@file:") {
        let path_part = &partial[6..];
        return complete_file_path(path_part);
    }

    // If it's a git mention with range, suggest common ranges
    if partial.starts_with("@git:") {
        return vec![
            "@git:HEAD~1".to_string(),
            "@git:HEAD~3".to_string(),
            "@git:HEAD~5".to_string(),
            "@git:main".to_string(),
            "@git:origin/main".to_string(),
        ];
    }

    // Otherwise, complete the mention type
    MENTION_TYPES
        .iter()
        .filter(|m| m.starts_with(partial))
        .map(|m| m.to_string())
        .collect()
}

/// Complete a file path
fn complete_file_path(partial: &str) -> Vec<String> {
    let cwd = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));

    // Determine the directory to search and the prefix to match
    let (search_dir, prefix) = if partial.is_empty() {
        (cwd.clone(), String::new())
    } else {
        let path = PathBuf::from(partial);
        if partial.ends_with('/') || partial.ends_with('\\') {
            // User typed a directory path ending with separator
            (cwd.join(&path), String::new())
        } else if let Some(parent) = path.parent() {
            // User is typing a file/dir name
            let parent_path = if parent.as_os_str().is_empty() {
                cwd.clone()
            } else {
                cwd.join(parent)
            };
            let file_prefix = path.file_name()
                .map(|s| s.to_string_lossy().to_string())
                .unwrap_or_default();
            (parent_path, file_prefix)
        } else {
            (cwd.clone(), partial.to_string())
        }
    };

    let mut completions = Vec::new();

    if let Ok(entries) = std::fs::read_dir(&search_dir) {
        for entry in entries.filter_map(|e| e.ok()) {
            let name = entry.file_name().to_string_lossy().to_string();

            // Skip hidden files unless user started with a dot
            if name.starts_with('.') && !prefix.starts_with('.') {
                continue;
            }

            if name.to_lowercase().starts_with(&prefix.to_lowercase()) {
                let path = entry.path();

                // Build the completion path relative to what user typed
                let completion = if partial.is_empty() {
                    name.clone()
                } else if partial.ends_with('/') || partial.ends_with('\\') {
                    format!("{}{}", partial, name)
                } else if let Some(parent) = PathBuf::from(partial).parent() {
                    if parent.as_os_str().is_empty() {
                        name.clone()
                    } else {
                        format!("{}/{}", parent.display(), name)
                    }
                } else {
                    name.clone()
                };

                // Add trailing slash for directories
                let display = if path.is_dir() {
                    format!("@file:{}/", completion)
                } else {
                    format!("@file:{}", completion)
                };

                completions.push(display);
            }
        }
    }

    // Sort: directories first, then alphabetically
    completions.sort_by(|a, b| {
        let a_is_dir = a.ends_with('/');
        let b_is_dir = b.ends_with('/');
        match (a_is_dir, b_is_dir) {
            (true, false) => std::cmp::Ordering::Less,
            (false, true) => std::cmp::Ordering::Greater,
            _ => a.cmp(b),
        }
    });

    completions.truncate(20); // Limit suggestions
    completions
}

/// Process input with @ mentions and return the expanded query
///
/// This is a convenience function that combines parsing, fetching, and formatting.
///
/// # Returns
///
/// A tuple of (expanded_query, display_messages)
/// - expanded_query: The query with context prepended
/// - display_messages: Messages to show the user about what was included
pub fn process_mentions(input: &str) -> (String, Vec<String>) {
    let (mentions, query) = parse_mentions(input);

    if mentions.is_empty() {
        return (query, vec![]);
    }

    let (context_prefix, messages) = build_context_prefix(&mentions);

    let expanded = if context_prefix.is_empty() {
        query
    } else {
        format!("{}{}", context_prefix, query)
    };

    (expanded, messages)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_mentions_file() {
        let (mentions, query) = parse_mentions("@file:src/main.rs explain this code");
        assert_eq!(mentions.len(), 1);
        assert!(matches!(&mentions[0], ContextMention::File { path } if path.to_string_lossy() == "src/main.rs"));
        assert_eq!(query, "explain this code");
    }

    #[test]
    fn test_parse_mentions_multiple() {
        let (mentions, query) = parse_mentions("@file:foo.rs @git @codebase what does this do?");
        assert_eq!(mentions.len(), 3);
        assert_eq!(query, "what does this do?");
    }

    #[test]
    fn test_parse_mentions_git_with_range() {
        let (mentions, _) = parse_mentions("@git:HEAD~3 show changes");
        assert!(matches!(&mentions[0], ContextMention::Git { range: Some(r) } if r == "HEAD~3"));
    }

    #[test]
    fn test_parse_mentions_quoted_path() {
        let (mentions, query) = parse_mentions(r#"@file:"path with spaces/file.rs" check this"#);
        assert_eq!(mentions.len(), 1);
        assert!(matches!(&mentions[0], ContextMention::File { path } if path.to_string_lossy() == "path with spaces/file.rs"));
        assert_eq!(query, "check this");
    }

    #[test]
    fn test_parse_mentions_no_duplicates() {
        let (mentions, _) = parse_mentions("@git @git @codebase @codebase");
        assert_eq!(mentions.len(), 2);
    }

    #[test]
    fn test_format_context_header() {
        let mention = ContextMention::File {
            path: PathBuf::from("src/main.rs")
        };
        let header = format_context_header(&mention, 142);
        assert!(header.contains("src/main.rs"));
        assert!(header.contains("142 lines"));
    }

    #[test]
    fn test_get_completions_mention_types() {
        let completions = get_completions("@f");
        assert!(completions.contains(&"@file:".to_string()));
    }

    #[test]
    fn test_get_completions_git_ranges() {
        let completions = get_completions("@git:");
        assert!(completions.iter().any(|c| c.contains("HEAD~")));
    }

    #[test]
    fn test_mention_display() {
        let file = ContextMention::File { path: PathBuf::from("test.rs") };
        assert_eq!(file.to_string(), "@file:test.rs");

        let git = ContextMention::Git { range: Some("HEAD~3".to_string()) };
        assert_eq!(git.to_string(), "@git:HEAD~3");

        let codebase = ContextMention::Codebase;
        assert_eq!(codebase.to_string(), "@codebase");
    }

    #[test]
    fn test_error_storage() {
        store_last_error("Test error message");
        assert_eq!(get_last_error(), Some("Test error message".to_string()));

        clear_last_error();
        assert_eq!(get_last_error(), None);
    }
}
