// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

//! Tab completion and typeahead for rigrun CLI slash commands and @ mentions.
//!
//! This module provides intelligent command completion for the interactive chat mode,
//! showing available commands as the user types and enabling Tab completion.
//!
//! ## Features
//!
//! - Show all commands when user types `/`
//! - Tab completion for partial commands (e.g., `/mo` + Tab -> `/model`)
//! - Inline command descriptions
//! - Argument completion for commands that take args (e.g., `/model <model_name>`)
//! - @ mention completion for context inclusion (e.g., `@f` + Tab -> `@file:`)
//! - File path completion for `@file:` mentions
//!
//! ## Example UX
//!
//! ```text
//! > /
//!   /help     - Show all commands
//!   /model    - Switch model
//!   /mode     - Switch routing mode
//!   /status   - Show current status
//!   ...
//!
//! > /mo
//!   /model    - Switch model
//!   /mode     - Switch routing mode
//!
//! > /model <Tab>
//!   qwen2.5-coder:14b (current)
//!   qwen2.5-coder:7b
//!   codestral:22b
//!
//! > @f<Tab>
//!   @file:
//!
//! > @file:src/<Tab>
//!   @file:src/main.rs
//!   @file:src/lib.rs
//!   @file:src/context/
//! ```

use std::borrow::Cow;
use rustyline::completion::{Completer, Pair};
use rustyline::highlight::Highlighter;
use rustyline::hint::{Hint, Hinter};
use rustyline::validate::Validator;
use rustyline::{Context, Helper, Result};

/// Information about a slash command.
#[derive(Debug, Clone)]
pub struct CommandInfo {
    /// Primary command name (e.g., "/help")
    pub name: &'static str,
    /// Alternative aliases (e.g., ["/h", "/?"])
    pub aliases: &'static [&'static str],
    /// Short description shown in completion
    pub description: &'static str,
    /// Arguments this command accepts (for display)
    pub args: Option<&'static str>,
    /// Possible argument values for completion
    pub arg_values: ArgValues,
}

/// Possible argument values for a command.
#[derive(Debug, Clone)]
pub enum ArgValues {
    /// No arguments
    None,
    /// Static list of values
    Static(&'static [&'static str]),
    /// Dynamic values (fetched at runtime)
    Dynamic(DynamicArgType),
}

/// Types of dynamic argument values.
#[derive(Debug, Clone, Copy)]
pub enum DynamicArgType {
    /// Model names from Ollama
    Models,
    /// Saved conversation IDs
    ConversationIds,
    /// Config keys
    ConfigKeys,
}

impl CommandInfo {
    /// Create a new command info with no arguments.
    pub const fn new(name: &'static str, description: &'static str) -> Self {
        Self {
            name,
            aliases: &[],
            description,
            args: None,
            arg_values: ArgValues::None,
        }
    }

    /// Create a new command info with aliases.
    pub const fn with_aliases(
        name: &'static str,
        aliases: &'static [&'static str],
        description: &'static str,
    ) -> Self {
        Self {
            name,
            aliases,
            description,
            args: None,
            arg_values: ArgValues::None,
        }
    }

    /// Add static argument values.
    pub const fn with_static_args(
        mut self,
        args: &'static str,
        values: &'static [&'static str],
    ) -> Self {
        self.args = Some(args);
        self.arg_values = ArgValues::Static(values);
        self
    }

    /// Add dynamic argument values.
    pub const fn with_dynamic_args(
        mut self,
        args: &'static str,
        arg_type: DynamicArgType,
    ) -> Self {
        self.args = Some(args);
        self.arg_values = ArgValues::Dynamic(arg_type);
        self
    }

    /// Check if this command matches a given input.
    pub fn matches(&self, input: &str) -> bool {
        let input_lower = input.to_lowercase();
        self.name.to_lowercase().starts_with(&input_lower)
            || self.aliases.iter().any(|a| a.to_lowercase().starts_with(&input_lower))
    }

    /// Check if this command exactly matches a given input.
    pub fn exact_match(&self, input: &str) -> bool {
        let input_lower = input.to_lowercase();
        self.name.to_lowercase() == input_lower
            || self.aliases.iter().any(|a| a.to_lowercase() == input_lower)
    }

    /// Get display string for completion menu.
    pub fn display_string(&self) -> String {
        if let Some(args) = self.args {
            format!("{} {} - {}", self.name, args, self.description)
        } else {
            format!("{} - {}", self.name, self.description)
        }
    }

    /// Get replacement string for completion.
    pub fn replacement(&self) -> &str {
        self.name
    }
}

/// All available slash commands in rigrun.
pub static COMMANDS: &[CommandInfo] = &[
    CommandInfo::with_aliases("/help", &["/h", "/?"], "Show all commands"),
    CommandInfo::new("/model", "Switch model")
        .with_dynamic_args("<model_name>", DynamicArgType::Models),
    CommandInfo::new("/mode", "Switch routing mode")
        .with_static_args("<mode>", &["local", "cloud", "auto", "hybrid"]),
    CommandInfo::new("/save", "Save conversation")
        .with_static_args("[summary]", &[]),
    CommandInfo::new("/resume", "Resume saved conversation")
        .with_dynamic_args("[id]", DynamicArgType::ConversationIds),
    CommandInfo::new("/history", "Show conversation history"),
    CommandInfo::new("/status", "Show current status"),
    CommandInfo::new("/clear", "Clear conversation"),
    CommandInfo::with_aliases("/exit", &["/quit", "/q"], "Exit rigrun"),
    CommandInfo::new("/doctor", "Diagnose system health")
        .with_static_args("[--fix]", &["--fix"]),
    CommandInfo::new("/config", "View or set configuration")
        .with_dynamic_args("[key] [value]", DynamicArgType::ConfigKeys),
];

/// Rigrun CLI completer that provides tab completion and hints for slash commands.
pub struct RigrunCompleter {
    /// Currently available models (updated dynamically)
    models: Vec<String>,
    /// Current model for marking in completion
    current_model: Option<String>,
    /// Saved conversation IDs
    conversation_ids: Vec<String>,
    /// Available config keys
    config_keys: Vec<String>,
}

impl RigrunCompleter {
    /// Create a new completer with empty dynamic values.
    pub fn new() -> Self {
        Self {
            models: Vec::new(),
            current_model: None,
            conversation_ids: Vec::new(),
            config_keys: vec![
                "model".to_string(),
                "port".to_string(),
                "openrouter_key".to_string(),
                "paranoid_mode".to_string(),
                "dod_banner_enabled".to_string(),
                "audit_log_enabled".to_string(),
            ],
        }
    }

    /// Update the list of available models.
    pub fn set_models(&mut self, models: Vec<String>) {
        self.models = models;
    }

    /// Set the current model (for marking in completion).
    pub fn set_current_model(&mut self, model: Option<String>) {
        self.current_model = model;
    }

    /// Update saved conversation IDs.
    pub fn set_conversation_ids(&mut self, ids: Vec<String>) {
        self.conversation_ids = ids;
    }

    /// Get completions for a partial input.
    fn get_completions(&self, line: &str, pos: usize) -> Vec<Pair> {
        let input = &line[..pos];

        // Check for @ mention completion
        if let Some(mention_completions) = self.complete_mention(input) {
            return mention_completions;
        }

        // Only complete slash commands if input starts with /
        if !input.starts_with('/') {
            return Vec::new();
        }

        // Check if we're completing a command or its arguments
        let parts: Vec<&str> = input.split_whitespace().collect();

        if parts.len() <= 1 {
            // Completing command name
            self.complete_command(input)
        } else {
            // Completing command arguments
            let command = parts[0];
            let arg_prefix = if parts.len() > 1 {
                parts.last().copied().unwrap_or("")
            } else {
                ""
            };
            self.complete_arguments(command, arg_prefix, input)
        }
    }

    /// Complete @ mentions for context inclusion.
    fn complete_mention(&self, input: &str) -> Option<Vec<Pair>> {
        // Find the last @ in the input to handle multiple mentions
        let last_at = input.rfind('@')?;
        let mention_part = &input[last_at..];
        let prefix_before_mention = &input[..last_at];

        // Check if we're completing an @ mention
        if !mention_part.starts_with('@') {
            return None;
        }

        // Get completions from the context module
        let completions = crate::context::get_completions(mention_part);

        if completions.is_empty() {
            return None;
        }

        Some(
            completions
                .into_iter()
                .map(|c| {
                    // Build display string with description
                    let display = match c.as_str() {
                        "@file:" => "@file:<path> - Include file contents".to_string(),
                        "@codebase" => "@codebase - Include codebase summary".to_string(),
                        "@git" => "@git - Include recent git history/diff".to_string(),
                        "@git:" => "@git:<range> - Include specific git range".to_string(),
                        "@error" => "@error - Include last error output".to_string(),
                        "@clipboard" => "@clipboard - Include clipboard contents".to_string(),
                        _ if c.starts_with("@file:") => c.clone(),
                        _ if c.starts_with("@git:") => c.clone(),
                        _ => c.clone(),
                    };

                    // For @file: completions, append a space if it's a file (not directory)
                    let replacement = if c.ends_with('/') || c == "@file:" || c == "@git:" {
                        format!("{}{}", prefix_before_mention, c)
                    } else {
                        format!("{}{} ", prefix_before_mention, c)
                    };

                    Pair { display, replacement }
                })
                .collect(),
        )
    }

    /// Highlight @ mentions in the input line.
    fn highlight_mentions(&self, line: &str) -> String {
        use regex::Regex;

        // Pattern to match @ mentions
        // @file:path, @codebase, @git, @git:range, @error, @clipboard
        let pattern = Regex::new(
            r#"(@file:(?:"[^"]+"|'[^']+'|\S+)|@codebase|@git(?::\S+)?|@error|@clipboard)"#
        ).unwrap();

        let mut result = String::new();
        let mut last_end = 0;

        for cap in pattern.captures_iter(line) {
            let m = cap.get(0).unwrap();

            // Add text before the match
            result.push_str(&line[last_end..m.start()]);

            // Add highlighted mention (blue/bright blue: \x1b[94m)
            result.push_str("\x1b[94m");
            result.push_str(m.as_str());
            result.push_str("\x1b[0m");

            last_end = m.end();
        }

        // Add remaining text after last match
        result.push_str(&line[last_end..]);

        result
    }

    /// Complete command names.
    fn complete_command(&self, input: &str) -> Vec<Pair> {
        let input_lower = input.to_lowercase();

        COMMANDS
            .iter()
            .filter(|cmd| {
                cmd.name.to_lowercase().starts_with(&input_lower)
                    || cmd.aliases.iter().any(|a| a.to_lowercase().starts_with(&input_lower))
            })
            .map(|cmd| {
                let display = cmd.display_string();
                Pair {
                    display,
                    replacement: format!("{} ", cmd.name),
                }
            })
            .collect()
    }

    /// Complete command arguments.
    fn complete_arguments(&self, command: &str, prefix: &str, full_input: &str) -> Vec<Pair> {
        let cmd_info = COMMANDS
            .iter()
            .find(|c| c.exact_match(command));

        let cmd_info = match cmd_info {
            Some(c) => c,
            None => return Vec::new(),
        };

        let values: Vec<String> = match &cmd_info.arg_values {
            ArgValues::None => return Vec::new(),
            ArgValues::Static(vals) => vals.iter().map(|s| s.to_string()).collect(),
            ArgValues::Dynamic(dynamic_type) => match dynamic_type {
                DynamicArgType::Models => self.models.clone(),
                DynamicArgType::ConversationIds => self.conversation_ids.clone(),
                DynamicArgType::ConfigKeys => self.config_keys.clone(),
            },
        };

        let prefix_lower = prefix.to_lowercase();
        let command_with_space = format!("{} ", command);

        values
            .iter()
            .filter(|v| v.to_lowercase().starts_with(&prefix_lower))
            .map(|v| {
                let is_current = self.current_model.as_ref().map_or(false, |m| m == v);
                let display = if is_current {
                    format!("{} (current)", v)
                } else {
                    v.clone()
                };

                // Calculate the base of the input before the current argument
                let base = if full_input.ends_with(prefix) && !prefix.is_empty() {
                    &full_input[..full_input.len() - prefix.len()]
                } else if full_input.ends_with(' ') {
                    full_input
                } else {
                    &command_with_space
                };

                Pair {
                    display,
                    replacement: format!("{}{}", base, v),
                }
            })
            .collect()
    }

    /// Get a hint for the current input (shown in dim text).
    fn get_hint(&self, line: &str) -> Option<CommandHint> {
        // Check for @ mention hints
        if let Some(hint) = self.get_mention_hint(line) {
            return Some(hint);
        }

        if !line.starts_with('/') {
            return None;
        }

        let parts: Vec<&str> = line.split_whitespace().collect();

        if parts.len() == 1 {
            // Show command hint if there's exactly one matching command
            let matches: Vec<_> = COMMANDS
                .iter()
                .filter(|cmd| cmd.matches(line))
                .collect();

            if matches.len() == 1 {
                let cmd = matches[0];
                let remaining = &cmd.name[line.len()..];
                let hint = if let Some(args) = cmd.args {
                    format!("{} {} - {}", remaining, args, cmd.description)
                } else {
                    format!("{} - {}", remaining, cmd.description)
                };
                return Some(CommandHint(hint));
            }
        } else if parts.len() == 2 && !line.ends_with(' ') {
            // Show argument hint
            let command = parts[0];
            let arg_prefix = parts[1];

            let cmd_info = COMMANDS.iter().find(|c| c.exact_match(command))?;

            let values: Vec<String> = match &cmd_info.arg_values {
                ArgValues::None => return None,
                ArgValues::Static(vals) => vals.iter().map(|s| s.to_string()).collect(),
                ArgValues::Dynamic(dynamic_type) => match dynamic_type {
                    DynamicArgType::Models => self.models.clone(),
                    DynamicArgType::ConversationIds => self.conversation_ids.clone(),
                    DynamicArgType::ConfigKeys => self.config_keys.clone(),
                },
            };

            let matches: Vec<_> = values
                .iter()
                .filter(|v| v.to_lowercase().starts_with(&arg_prefix.to_lowercase()))
                .collect();

            if matches.len() == 1 {
                let remaining = &matches[0][arg_prefix.len()..];
                return Some(CommandHint(remaining.to_string()));
            }
        }

        None
    }

    /// Get a hint for @ mentions.
    fn get_mention_hint(&self, line: &str) -> Option<CommandHint> {
        // Find the last @ in the input
        let last_at = line.rfind('@')?;
        let mention_part = &line[last_at..];

        // Check if there's a space after the mention (user is done typing it)
        if mention_part.contains(' ') {
            return None;
        }

        // Get possible completions
        let completions = crate::context::get_completions(mention_part);

        if completions.len() == 1 {
            // Single match - show the completion
            let completion = &completions[0];

            // Calculate what to add as hint
            if completion.starts_with(mention_part) {
                let remaining = &completion[mention_part.len()..];
                let hint = match completion.as_str() {
                    "@file:" => format!("{}<path>", remaining),
                    "@codebase" => format!("{} - Include codebase summary", remaining),
                    "@git" => format!("{} - Include git history/diff", remaining),
                    "@git:" => format!("{}<range>", remaining),
                    "@error" => format!("{} - Include last error", remaining),
                    "@clipboard" => format!("{} - Include clipboard", remaining),
                    _ => remaining.to_string(),
                };
                return Some(CommandHint(hint));
            }
        } else if mention_part == "@" {
            // Just @ typed - show available mention types
            return Some(CommandHint("file:|codebase|git|error|clipboard".to_string()));
        }

        None
    }
}

impl Default for RigrunCompleter {
    fn default() -> Self {
        Self::new()
    }
}

/// A hint displayed after the cursor in dim text.
#[derive(Debug, Clone)]
pub struct CommandHint(String);

impl Hint for CommandHint {
    fn display(&self) -> &str {
        &self.0
    }

    fn completion(&self) -> Option<&str> {
        Some(&self.0)
    }
}

impl Completer for RigrunCompleter {
    type Candidate = Pair;

    fn complete(
        &self,
        line: &str,
        pos: usize,
        _ctx: &Context<'_>,
    ) -> Result<(usize, Vec<Pair>)> {
        let completions = self.get_completions(line, pos);

        // Find the start position for replacement
        let start = if line.starts_with('/') {
            0
        } else {
            pos
        };

        Ok((start, completions))
    }
}

impl Hinter for RigrunCompleter {
    type Hint = CommandHint;

    fn hint(&self, line: &str, pos: usize, _ctx: &Context<'_>) -> Option<Self::Hint> {
        // Only show hint if cursor is at end of line
        if pos < line.len() {
            return None;
        }
        self.get_hint(line)
    }
}

impl Highlighter for RigrunCompleter {
    fn highlight_hint<'h>(&self, hint: &'h str) -> Cow<'h, str> {
        // Show hints in dim gray
        Cow::Owned(format!("\x1b[90m{}\x1b[0m", hint))
    }

    fn highlight<'l>(&self, line: &'l str, _pos: usize) -> Cow<'l, str> {
        // Highlight @ mentions in blue
        if line.contains('@') {
            let highlighted = self.highlight_mentions(line);
            if highlighted != line {
                return Cow::Owned(highlighted);
            }
        }

        if line.starts_with('/') {
            // Highlight slash commands in cyan
            let parts: Vec<&str> = line.splitn(2, ' ').collect();
            let command = parts[0];

            // Check if it's a valid command
            let is_valid = COMMANDS.iter().any(|c| c.exact_match(command));

            if is_valid {
                let colored_cmd = format!("\x1b[36m{}\x1b[0m", command);
                if parts.len() > 1 {
                    Cow::Owned(format!("{} {}", colored_cmd, parts[1]))
                } else {
                    Cow::Owned(colored_cmd)
                }
            } else if COMMANDS.iter().any(|c| c.matches(command)) {
                // Partial match - show in yellow
                Cow::Owned(format!("\x1b[33m{}\x1b[0m", line))
            } else {
                // Invalid command - show in red
                Cow::Owned(format!("\x1b[31m{}\x1b[0m", line))
            }
        } else {
            Cow::Borrowed(line)
        }
    }

    fn highlight_char(&self, _line: &str, _pos: usize, _forced: bool) -> bool {
        true
    }
}

impl Validator for RigrunCompleter {}

impl Helper for RigrunCompleter {}

/// Show all available commands (for /help).
pub fn show_help() {
    println!("\nAvailable commands:\n");
    for cmd in COMMANDS {
        let aliases = if cmd.aliases.is_empty() {
            String::new()
        } else {
            format!(" (aliases: {})", cmd.aliases.join(", "))
        };

        let args = cmd.args.map_or(String::new(), |a| format!(" {}", a));

        println!(
            "  \x1b[36m{}{}\x1b[0m{} - {}",
            cmd.name,
            args,
            aliases,
            cmd.description
        );
    }

    // Show @ mention help
    println!("\n@ Mentions (include context in your query):\n");
    println!("  \x1b[94m@file:<path>\x1b[0m      - Include file contents");
    println!("  \x1b[94m@codebase\x1b[0m         - Include codebase summary");
    println!("  \x1b[94m@git\x1b[0m              - Include recent git history/diff");
    println!("  \x1b[94m@git:<range>\x1b[0m      - Include specific git range (e.g., HEAD~3)");
    println!("  \x1b[94m@error\x1b[0m            - Include last error output");
    println!("  \x1b[94m@clipboard\x1b[0m        - Include clipboard contents");

    println!("\nExamples:");
    println!("  > @file:src/main.rs Can you explain this code?");
    println!("  > @git What changed in the last commit?");
    println!("  > @codebase @file:Cargo.toml Give me an overview of this project");
    println!();
}

/// Parse a slash command from user input.
/// Returns (command_name, arguments) if valid, None otherwise.
pub fn parse_command(input: &str) -> Option<(&str, Vec<&str>)> {
    let input = input.trim();
    if !input.starts_with('/') {
        return None;
    }

    let parts: Vec<&str> = input.split_whitespace().collect();
    if parts.is_empty() {
        return None;
    }

    let command = parts[0];
    let args = parts[1..].to_vec();

    // Validate that the command exists
    if COMMANDS.iter().any(|c| c.exact_match(command)) {
        Some((command, args))
    } else {
        None
    }
}

/// Check if input looks like a slash command (even if invalid).
pub fn is_slash_command(input: &str) -> bool {
    input.trim().starts_with('/')
}

/// Get the canonical command name for an alias.
pub fn get_canonical_command(input: &str) -> Option<&'static str> {
    let input_lower = input.to_lowercase();
    COMMANDS
        .iter()
        .find(|c| c.exact_match(&input_lower))
        .map(|c| c.name)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_command_matching() {
        let help = &COMMANDS[0];
        assert!(help.matches("/help"));
        assert!(help.matches("/h"));
        assert!(help.matches("/?"));
        assert!(help.matches("/hel"));
        assert!(!help.matches("/model"));
    }

    #[test]
    fn test_parse_command() {
        assert_eq!(parse_command("/help"), Some(("/help", vec![])));
        assert_eq!(parse_command("/h"), Some(("/h", vec![])));
        assert_eq!(parse_command("/model test"), Some(("/model", vec!["test"])));
        assert_eq!(parse_command("/mode local"), Some(("/mode", vec!["local"])));
        assert_eq!(parse_command("hello"), None);
        assert_eq!(parse_command("/invalid"), None);
    }

    #[test]
    fn test_get_canonical_command() {
        assert_eq!(get_canonical_command("/help"), Some("/help"));
        assert_eq!(get_canonical_command("/h"), Some("/help"));
        assert_eq!(get_canonical_command("/?"), Some("/help"));
        assert_eq!(get_canonical_command("/exit"), Some("/exit"));
        assert_eq!(get_canonical_command("/quit"), Some("/exit"));
        assert_eq!(get_canonical_command("/q"), Some("/exit"));
    }

    #[test]
    fn test_is_slash_command() {
        assert!(is_slash_command("/help"));
        assert!(is_slash_command("/anything"));
        assert!(is_slash_command("  /help"));
        assert!(!is_slash_command("help"));
        assert!(!is_slash_command(""));
    }

    #[test]
    fn test_completer_creation() {
        let completer = RigrunCompleter::new();
        assert!(completer.models.is_empty());
        assert!(completer.current_model.is_none());
    }

    #[test]
    fn test_completer_set_models() {
        let mut completer = RigrunCompleter::new();
        completer.set_models(vec!["model1".to_string(), "model2".to_string()]);
        completer.set_current_model(Some("model1".to_string()));

        assert_eq!(completer.models.len(), 2);
        assert_eq!(completer.current_model, Some("model1".to_string()));
    }

    #[test]
    fn test_mention_hint() {
        let completer = RigrunCompleter::new();

        // Test hint for partial @ mention
        let hint = completer.get_mention_hint("@f");
        assert!(hint.is_some());

        // Test hint for just @
        let hint = completer.get_mention_hint("@");
        assert!(hint.is_some());
        let hint_text = hint.unwrap();
        assert!(hint_text.0.contains("file"));

        // Test no hint after completed mention with space
        let hint = completer.get_mention_hint("@codebase ");
        assert!(hint.is_none());
    }

    #[test]
    fn test_highlight_mentions() {
        let completer = RigrunCompleter::new();

        // Test highlighting file mention
        let highlighted = completer.highlight_mentions("@file:src/main.rs explain this");
        assert!(highlighted.contains("\x1b[94m"));

        // Test highlighting multiple mentions
        let highlighted = completer.highlight_mentions("@git @codebase what is this?");
        // Should contain escape codes for both mentions
        assert!(highlighted.matches("\x1b[94m").count() >= 2);
    }
}
