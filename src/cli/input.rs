// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

//! Interactive input handling for rigrun CLI.
//!
//! This module provides enhanced command-line input with:
//! - Tab completion for slash commands
//! - Command history
//! - Inline hints
//! - Session timeout integration
//!
//! ## Usage
//!
//! ```no_run
//! use rigrun::cli::input::InteractiveInput;
//!
//! let mut input = InteractiveInput::new()?;
//! input.set_models(vec!["qwen2.5-coder:14b".to_string()]);
//! input.set_current_model(Some("qwen2.5-coder:14b".to_string()));
//!
//! loop {
//!     match input.read_line("rigrun> ")? {
//!         Some(line) => println!("Got: {}", line),
//!         None => break, // EOF
//!     }
//! }
//! ```

use anyhow::{Context, Result};
use rustyline::history::{DefaultHistory, History};
use rustyline::{ColorMode, CompletionType, Config, EditMode, Editor};
use std::path::PathBuf;

use super::completer::{RigrunCompleter, COMMANDS};

/// History file name in config directory.
const HISTORY_FILE: &str = "history.txt";

/// Maximum history entries to keep.
const MAX_HISTORY_ENTRIES: usize = 1000;

/// Interactive input handler with tab completion and history.
pub struct InteractiveInput {
    /// Rustyline editor with our completer
    editor: Editor<RigrunCompleter, DefaultHistory>,
    /// Path to history file
    history_path: Option<PathBuf>,
}

impl InteractiveInput {
    /// Create a new interactive input handler.
    pub fn new() -> Result<Self> {
        let config = Config::builder()
            .history_ignore_space(true)
            .history_ignore_dups(true)?
            .completion_type(CompletionType::List)
            .edit_mode(EditMode::Emacs)
            .color_mode(ColorMode::Enabled)
            .auto_add_history(true)
            .max_history_size(MAX_HISTORY_ENTRIES)?
            .build();

        let completer = RigrunCompleter::new();

        let mut editor = Editor::with_config(config)
            .context("Failed to create input editor")?;

        editor.set_helper(Some(completer));

        // Bind Tab to complete
        editor.bind_sequence(
            rustyline::KeyEvent::new('\t', rustyline::Modifiers::NONE),
            rustyline::Cmd::Complete,
        );

        // Try to load history
        let history_path = Self::get_history_path();
        if let Some(ref path) = history_path {
            if path.exists() {
                let _ = editor.load_history(path);
            }
        }

        Ok(Self {
            editor,
            history_path,
        })
    }

    /// Get the path to the history file.
    fn get_history_path() -> Option<PathBuf> {
        dirs::home_dir().map(|home| home.join(".rigrun").join(HISTORY_FILE))
    }

    /// Update the list of available models for completion.
    pub fn set_models(&mut self, models: Vec<String>) {
        if let Some(helper) = self.editor.helper_mut() {
            helper.set_models(models);
        }
    }

    /// Set the current model (marked in completion list).
    pub fn set_current_model(&mut self, model: Option<String>) {
        if let Some(helper) = self.editor.helper_mut() {
            helper.set_current_model(model);
        }
    }

    /// Update saved conversation IDs for completion.
    pub fn set_conversation_ids(&mut self, ids: Vec<String>) {
        if let Some(helper) = self.editor.helper_mut() {
            helper.set_conversation_ids(ids);
        }
    }

    /// Read a line of input with the given prompt.
    ///
    /// Returns `Ok(Some(line))` on successful input, `Ok(None)` on EOF (Ctrl+D),
    /// and `Err` on error.
    pub fn read_line(&mut self, prompt: &str) -> Result<Option<String>> {
        match self.editor.readline(prompt) {
            Ok(line) => {
                // Save history after each line
                self.save_history();
                Ok(Some(line))
            }
            Err(rustyline::error::ReadlineError::Interrupted) => {
                // Ctrl+C - return empty line to let caller handle it
                Ok(Some(String::new()))
            }
            Err(rustyline::error::ReadlineError::Eof) => {
                // Ctrl+D - signal exit
                Ok(None)
            }
            Err(e) => Err(anyhow::anyhow!("Input error: {}", e)),
        }
    }

    /// Read a line with a custom colored prompt.
    ///
    /// The prompt will have session time indicator appended.
    pub fn read_line_with_time(&mut self, base_prompt: &str, remaining_secs: u64) -> Result<Option<String>> {
        let mins = remaining_secs / 60;
        let secs = remaining_secs % 60;

        let time_indicator = if remaining_secs <= 120 {
            format!("\x1b[33m[{}:{:02}]\x1b[0m", mins, secs) // Yellow
        } else {
            format!("\x1b[90m[{}:{:02}]\x1b[0m", mins, secs) // Gray
        };

        let prompt = format!("{} {} ", base_prompt, time_indicator);
        self.read_line(&prompt)
    }

    /// Save history to file.
    fn save_history(&mut self) {
        if let Some(ref path) = self.history_path {
            // Ensure parent directory exists
            if let Some(parent) = path.parent() {
                let _ = std::fs::create_dir_all(parent);
            }
            let _ = self.editor.save_history(path);
        }
    }

    /// Add an entry to history manually.
    pub fn add_history(&mut self, line: &str) {
        let _ = self.editor.add_history_entry(line);
    }

    /// Clear history.
    pub fn clear_history(&mut self) {
        self.editor.clear_history().ok();
        if let Some(ref path) = self.history_path {
            let _ = std::fs::remove_file(path);
        }
    }

    /// Get number of history entries.
    pub fn history_len(&self) -> usize {
        self.editor.history().len()
    }
}

/// Display available commands when user types just `/`.
pub fn show_command_menu() {
    println!();
    println!("\x1b[90m  Available commands:\x1b[0m");
    println!();

    for cmd in COMMANDS.iter() {
        let args_str = cmd.args.map_or(String::new(), |a| format!(" {}", a));
        println!(
            "  \x1b[36m{:<10}\x1b[0m{:<15} \x1b[90m- {}\x1b[0m",
            cmd.name,
            args_str,
            cmd.description
        );
    }
    println!();
}

/// Display filtered command menu based on partial input.
pub fn show_filtered_commands(prefix: &str) {
    let matches: Vec<_> = COMMANDS
        .iter()
        .filter(|cmd| cmd.matches(prefix))
        .collect();

    if matches.is_empty() {
        println!("\x1b[31m  No matching commands for '{}'\x1b[0m", prefix);
        return;
    }

    println!();
    for cmd in matches {
        let args_str = cmd.args.map_or(String::new(), |a| format!(" {}", a));
        println!(
            "  \x1b[36m{:<10}\x1b[0m{:<15} \x1b[90m- {}\x1b[0m",
            cmd.name,
            args_str,
            cmd.description
        );
    }
    println!();
}

/// Simple input for non-interactive environments (fallback).
pub struct SimpleInput {
    reader: std::io::BufReader<std::io::Stdin>,
}

impl SimpleInput {
    /// Create a new simple input handler.
    pub fn new() -> Self {
        Self {
            reader: std::io::BufReader::new(std::io::stdin()),
        }
    }

    /// Read a line without any completion.
    pub fn read_line(&mut self, prompt: &str) -> Result<Option<String>> {
        use std::io::{BufRead, Write};

        print!("{}", prompt);
        std::io::stdout().flush()?;

        let mut line = String::new();
        match self.reader.read_line(&mut line) {
            Ok(0) => Ok(None), // EOF
            Ok(_) => Ok(Some(line.trim_end().to_string())),
            Err(e) => Err(anyhow::anyhow!("Input error: {}", e)),
        }
    }
}

impl Default for SimpleInput {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_history_path() {
        let path = InteractiveInput::get_history_path();
        assert!(path.is_some());
        if let Some(p) = path {
            assert!(p.ends_with("history.txt"));
        }
    }

    #[test]
    fn test_simple_input_creation() {
        let _input = SimpleInput::new();
    }
}
