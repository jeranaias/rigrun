// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

//! CLI module for rigrun interactive features.
//!
//! This module provides:
//! - Tab completion for slash commands
//! - Command hints and typeahead
//! - Input history management
//! - Session timeout integration
//!
//! ## Features
//!
//! - **Tab Completion**: Press Tab to complete partial commands
//! - **Hints**: See command suggestions as you type
//! - **History**: Use arrow keys to navigate command history
//! - **Argument Completion**: Complete command arguments (models, modes, etc.)
//!
//! ## Example
//!
//! ```no_run
//! use rigrun::cli::{InteractiveInput, parse_command, show_help};
//!
//! let mut input = InteractiveInput::new()?;
//!
//! // Set available models for /model completion
//! input.set_models(vec!["qwen2.5-coder:14b".to_string()]);
//! input.set_current_model(Some("qwen2.5-coder:14b".to_string()));
//!
//! loop {
//!     match input.read_line("rigrun> ")? {
//!         Some(line) if line.is_empty() => continue,
//!         Some(line) if line == "exit" => break,
//!         Some(line) => {
//!             if let Some((cmd, args)) = parse_command(&line) {
//!                 match cmd {
//!                     "/help" | "/h" | "/?" => show_help(),
//!                     "/model" => { /* handle model change */ }
//!                     _ => println!("Unknown command"),
//!                 }
//!             } else {
//!                 // Regular chat input
//!                 println!("You said: {}", line);
//!             }
//!         }
//!         None => break, // EOF
//!     }
//! }
//! # Ok::<(), anyhow::Error>(())
//! ```

pub mod completer;
pub mod input;

// Re-export commonly used types
pub use completer::{
    CommandInfo, RigrunCompleter, COMMANDS,
    parse_command, is_slash_command, get_canonical_command, show_help,
    ArgValues, DynamicArgType,
};

pub use input::{
    InteractiveInput, SimpleInput,
    show_command_menu, show_filtered_commands,
};
