// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

//! Conversation Storage Module for rigrun
//!
//! Provides persistence for chat conversations, enabling users to save,
//! resume, and manage conversation history across sessions.
//!
//! ## Features
//!
//! - Save conversations with automatic summary generation
//! - Resume previous conversations
//! - List saved conversations with timestamps and summaries
//! - Delete old conversations
//! - Auto-save option on exit

use anyhow::{Context, Result};
use chrono::{DateTime, Utc};
use colored::Colorize;
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

use rigrun::types::Message;
use rigrun::status_indicator::{StatusIndicator, OperatingMode};

/// Maximum length for conversation summaries
const MAX_SUMMARY_LENGTH: usize = 60;

/// Saved conversation metadata and messages
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SavedConversation {
    /// Unique identifier for the conversation
    pub id: String,
    /// Human-readable summary/title of the conversation
    pub summary: String,
    /// Model used for the conversation
    pub model: String,
    /// When the conversation was created
    pub created_at: DateTime<Utc>,
    /// When the conversation was last updated
    pub updated_at: DateTime<Utc>,
    /// Total number of messages
    pub message_count: usize,
    /// The conversation messages
    pub messages: Vec<Message>,
}

impl SavedConversation {
    /// Create a new saved conversation from messages
    pub fn new(model: &str, messages: Vec<Message>) -> Self {
        let now = Utc::now();
        let id = format!("conv_{}", now.timestamp_millis());
        let summary = Self::generate_summary(&messages);
        let message_count = messages.len();

        Self {
            id,
            summary,
            model: model.to_string(),
            created_at: now,
            updated_at: now,
            message_count,
            messages,
        }
    }

    /// Generate a summary from the first user message
    fn generate_summary(messages: &[Message]) -> String {
        // Find the first user message
        let first_user_msg = messages
            .iter()
            .find(|m| m.role == "user")
            .map(|m| m.content.as_str())
            .unwrap_or("Empty conversation");

        // Truncate and clean up the summary
        let summary = first_user_msg
            .lines()
            .next()
            .unwrap_or(first_user_msg)
            .trim();

        if summary.len() > MAX_SUMMARY_LENGTH {
            format!("{}...", &summary[..MAX_SUMMARY_LENGTH - 3])
        } else {
            summary.to_string()
        }
    }

    /// Get a formatted time description (e.g., "2 hours ago", "Yesterday")
    pub fn time_ago(&self) -> String {
        let now = Utc::now();
        let duration = now.signed_duration_since(self.updated_at);

        if duration.num_minutes() < 1 {
            "Just now".to_string()
        } else if duration.num_minutes() < 60 {
            let mins = duration.num_minutes();
            if mins == 1 {
                "1 minute ago".to_string()
            } else {
                format!("{} minutes ago", mins)
            }
        } else if duration.num_hours() < 24 {
            let hours = duration.num_hours();
            if hours == 1 {
                "1 hour ago".to_string()
            } else {
                format!("{} hours ago", hours)
            }
        } else if duration.num_days() == 1 {
            "Yesterday".to_string()
        } else if duration.num_days() < 7 {
            format!("{} days ago", duration.num_days())
        } else if duration.num_weeks() == 1 {
            "1 week ago".to_string()
        } else {
            self.updated_at.format("%Y-%m-%d").to_string()
        }
    }

    /// Get the last user message in the conversation
    pub fn last_user_message(&self) -> Option<&str> {
        self.messages
            .iter()
            .rev()
            .find(|m| m.role == "user")
            .map(|m| m.content.as_str())
    }
}

/// Manages conversation storage on disk
pub struct ConversationStore {
    /// Directory where conversations are stored
    store_dir: PathBuf,
}

impl ConversationStore {
    /// Create a new conversation store
    pub fn new() -> Result<Self> {
        let home = dirs::home_dir().context("Could not find home directory")?;
        let store_dir = home.join(".rigrun").join("conversations");

        if !store_dir.exists() {
            fs::create_dir_all(&store_dir)
                .context("Failed to create conversations directory")?;
        }

        Ok(Self { store_dir })
    }

    /// Get the path for a conversation file
    fn conversation_path(&self, id: &str) -> PathBuf {
        self.store_dir.join(format!("{}.json", id))
    }

    /// Save a conversation
    pub fn save(&self, conversation: &SavedConversation) -> Result<()> {
        let path = self.conversation_path(&conversation.id);
        let content = serde_json::to_string_pretty(conversation)
            .context("Failed to serialize conversation")?;
        fs::write(&path, content)
            .context("Failed to write conversation file")?;

        tracing::info!(
            "CONVERSATION_SAVED | id={} messages={} summary=\"{}\"",
            conversation.id,
            conversation.message_count,
            conversation.summary
        );

        Ok(())
    }

    /// Load a conversation by ID
    pub fn load(&self, id: &str) -> Result<SavedConversation> {
        let path = self.conversation_path(id);
        let content = fs::read_to_string(&path)
            .context(format!("Failed to read conversation file: {}", id))?;
        let conversation: SavedConversation = serde_json::from_str(&content)
            .context("Failed to parse conversation file")?;

        tracing::info!(
            "CONVERSATION_LOADED | id={} messages={}",
            conversation.id,
            conversation.message_count
        );

        Ok(conversation)
    }

    /// List all saved conversations, sorted by most recent first
    pub fn list(&self) -> Result<Vec<SavedConversation>> {
        let mut conversations = Vec::new();

        if let Ok(entries) = fs::read_dir(&self.store_dir) {
            for entry in entries.flatten() {
                let path = entry.path();
                if path.extension().map(|e| e == "json").unwrap_or(false) {
                    if let Ok(content) = fs::read_to_string(&path) {
                        if let Ok(conv) = serde_json::from_str::<SavedConversation>(&content) {
                            conversations.push(conv);
                        }
                    }
                }
            }
        }

        // Sort by most recent first
        conversations.sort_by(|a, b| b.updated_at.cmp(&a.updated_at));

        Ok(conversations)
    }

    /// Delete a conversation by ID
    pub fn delete(&self, id: &str) -> Result<bool> {
        let path = self.conversation_path(id);
        if path.exists() {
            fs::remove_file(&path)
                .context(format!("Failed to delete conversation: {}", id))?;

            tracing::info!("CONVERSATION_DELETED | id={}", id);
            Ok(true)
        } else {
            Ok(false)
        }
    }

    /// Delete a conversation by index (1-based, as shown to user)
    pub fn delete_by_index(&self, index: usize) -> Result<Option<String>> {
        let conversations = self.list()?;
        if index == 0 || index > conversations.len() {
            return Ok(None);
        }

        let conv = &conversations[index - 1];
        let id = conv.id.clone();
        let summary = conv.summary.clone();

        self.delete(&id)?;
        Ok(Some(summary))
    }

    /// Get a conversation by index (1-based, as shown to user)
    pub fn get_by_index(&self, index: usize) -> Result<Option<SavedConversation>> {
        let conversations = self.list()?;
        if index == 0 || index > conversations.len() {
            return Ok(None);
        }

        Ok(Some(conversations[index - 1].clone()))
    }

    /// Update an existing conversation with new messages
    pub fn update(&self, id: &str, messages: Vec<Message>) -> Result<()> {
        let mut conversation = self.load(id)?;
        conversation.messages = messages;
        conversation.message_count = conversation.messages.len();
        conversation.updated_at = Utc::now();
        conversation.summary = SavedConversation::generate_summary(&conversation.messages);
        self.save(&conversation)
    }
}

impl Default for ConversationStore {
    fn default() -> Self {
        Self::new().expect("Failed to create conversation store")
    }
}

/// Result of a slash command execution
#[derive(Debug)]
pub enum CommandResult {
    /// Continue the chat loop normally
    Continue,
    /// Command was handled, skip sending to model
    Handled,
    /// Exit the chat session
    Exit,
    /// Resume a conversation (returns the conversation)
    Resume(SavedConversation),
}

/// Parse and handle slash commands in the chat
pub fn handle_slash_command(
    input: &str,
    conversation: &[Message],
    model: &str,
    store: &ConversationStore,
    auto_save: &mut bool,
    current_conversation_id: &mut Option<String>,
    status_indicator: Option<&StatusIndicator>,
) -> Result<CommandResult> {
    let input = input.trim();

    // Check if it's a slash command
    if !input.starts_with('/') {
        return Ok(CommandResult::Continue);
    }

    let parts: Vec<&str> = input[1..].split_whitespace().collect();
    if parts.is_empty() {
        return Ok(CommandResult::Continue);
    }

    let command = parts[0].to_lowercase();
    let args: Vec<&str> = parts[1..].to_vec();

    match command.as_str() {
        "help" | "h" | "?" => {
            print_help();
            Ok(CommandResult::Handled)
        }

        "save" | "s" => {
            if conversation.is_empty() {
                println!("\n  No messages to save.\n");
                return Ok(CommandResult::Handled);
            }

            // Check if we're updating an existing conversation
            if let Some(id) = current_conversation_id.as_ref() {
                store.update(id, conversation.to_vec())?;
                println!("\n  Conversation updated.\n");
            } else {
                let saved = SavedConversation::new(model, conversation.to_vec());
                let id = saved.id.clone();
                let summary = saved.summary.clone();
                store.save(&saved)?;
                *current_conversation_id = Some(id);
                println!("\n  Saved: \"{}\"\n", summary);
            }
            Ok(CommandResult::Handled)
        }

        "resume" | "r" => {
            let conversations = store.list()?;
            if conversations.is_empty() {
                println!("\n  No saved conversations found.\n");
                return Ok(CommandResult::Handled);
            }

            // If an index was provided, resume directly
            if !args.is_empty() {
                if let Ok(index) = args[0].parse::<usize>() {
                    if let Some(conv) = store.get_by_index(index)? {
                        return Ok(CommandResult::Resume(conv));
                    } else {
                        println!("\n  Invalid conversation number.\n");
                        return Ok(CommandResult::Handled);
                    }
                }
            }

            // Show the list and prompt for selection
            println!("\n  Saved Conversations:");
            println!("  {}", "-".repeat(60));

            for (i, conv) in conversations.iter().enumerate() {
                println!(
                    "    [{}] {} - \"{}\"",
                    i + 1,
                    conv.time_ago(),
                    conv.summary
                );
            }

            println!("\n  Select conversation (1-{}) or 'c' to cancel: ", conversations.len());

            // Read user selection
            let mut selection = String::new();
            std::io::stdin().read_line(&mut selection)?;
            let selection = selection.trim();

            if selection.eq_ignore_ascii_case("c") || selection.is_empty() {
                println!("  Cancelled.\n");
                return Ok(CommandResult::Handled);
            }

            if let Ok(index) = selection.parse::<usize>() {
                if let Some(conv) = store.get_by_index(index)? {
                    return Ok(CommandResult::Resume(conv));
                }
            }

            println!("  Invalid selection.\n");
            Ok(CommandResult::Handled)
        }

        "history" | "list" | "ls" => {
            let conversations = store.list()?;
            if conversations.is_empty() {
                println!("\n  No saved conversations found.\n");
                return Ok(CommandResult::Handled);
            }

            println!("\n  Saved Conversations:");
            println!("  {}", "-".repeat(60));

            for (i, conv) in conversations.iter().enumerate() {
                println!(
                    "    [{}] {} - \"{}\" ({} messages)",
                    i + 1,
                    conv.time_ago(),
                    conv.summary,
                    conv.message_count
                );
            }
            println!();

            Ok(CommandResult::Handled)
        }

        "delete" | "del" | "rm" => {
            if args.is_empty() {
                // Show list and prompt for selection
                let conversations = store.list()?;
                if conversations.is_empty() {
                    println!("\n  No saved conversations to delete.\n");
                    return Ok(CommandResult::Handled);
                }

                println!("\n  Select conversation to delete:");
                println!("  {}", "-".repeat(60));

                for (i, conv) in conversations.iter().enumerate() {
                    println!(
                        "    [{}] {} - \"{}\"",
                        i + 1,
                        conv.time_ago(),
                        conv.summary
                    );
                }

                println!("\n  Enter number (1-{}) or 'c' to cancel: ", conversations.len());

                let mut selection = String::new();
                std::io::stdin().read_line(&mut selection)?;
                let selection = selection.trim();

                if selection.eq_ignore_ascii_case("c") || selection.is_empty() {
                    println!("  Cancelled.\n");
                    return Ok(CommandResult::Handled);
                }

                if let Ok(index) = selection.parse::<usize>() {
                    if let Some(summary) = store.delete_by_index(index)? {
                        println!("  Deleted: \"{}\"\n", summary);
                    } else {
                        println!("  Invalid selection.\n");
                    }
                } else {
                    println!("  Invalid selection.\n");
                }
            } else if let Ok(index) = args[0].parse::<usize>() {
                if let Some(summary) = store.delete_by_index(index)? {
                    println!("\n  Deleted: \"{}\"\n", summary);
                } else {
                    println!("\n  Invalid conversation number.\n");
                }
            } else {
                println!("\n  Usage: /delete <number>\n");
            }

            Ok(CommandResult::Handled)
        }

        "autosave" | "auto" => {
            *auto_save = !*auto_save;
            if *auto_save {
                println!("\n  Auto-save enabled. Conversation will be saved on exit.\n");
            } else {
                println!("\n  Auto-save disabled.\n");
            }
            Ok(CommandResult::Handled)
        }

        "new" | "clear" => {
            *current_conversation_id = None;
            println!("\n  Starting new conversation. Previous messages cleared.\n");
            Ok(CommandResult::Handled)
        }

        "exit" | "quit" | "q" => {
            Ok(CommandResult::Exit)
        }

        "status" | "stat" => {
            if let Some(indicator) = status_indicator {
                indicator.render_detailed_status();
            } else {
                // Fallback: create a temporary status indicator
                let mut temp_indicator = StatusIndicator::new(Default::default());
                temp_indicator.set_model(model);
                temp_indicator.set_mode(OperatingMode::Local);
                temp_indicator.refresh_gpu_status();
                temp_indicator.update_stats(conversation.len() as u32, 0);
                temp_indicator.render_detailed_status();
            }
            Ok(CommandResult::Handled)
        }

        "model" | "m" => {
            if args.is_empty() {
                // Show current model
                println!();
                println!("  Current model: {}", model.bright_white().bold());
                println!();
                println!("  To change model, use: /model <model_name>");
                println!("  Example: /model qwen2.5-coder:7b");
                println!();

                // List available models
                if let Ok(models) = rigrun::local::list_models() {
                    if !models.is_empty() {
                        println!("  Available models:");
                        for m in models.iter().take(10) {
                            let indicator = if m == model { " (active)" } else { "" };
                            println!("    - {}{}", m, indicator.bright_green());
                        }
                        if models.len() > 10 {
                            println!("    ... and {} more", models.len() - 10);
                        }
                        println!();
                    }
                }
            } else {
                // Note: Actual model switching would need to return a signal
                // to the main loop to change the model variable
                println!();
                println!("  {}", "[!] Model switching during session is not yet supported.".yellow());
                println!("  Exit and restart with: rigrun chat --model {}", args[0]);
                println!();
            }
            Ok(CommandResult::Handled)
        }

        "mode" => {
            // Show current mode and available modes
            let current_mode = if let Some(indicator) = status_indicator {
                format!("{}", indicator.mode())
            } else {
                "local".to_string()
            };

            println!();
            println!("  Current mode: {}", current_mode.bright_white().bold());
            println!();
            println!("  Available modes:");
            println!("    {} - All queries processed locally via Ollama", "local".green());
            println!("    {} - All queries routed to cloud providers", "cloud".yellow());
            println!("    {}  - Local first, cloud fallback if needed", "auto".cyan());
            println!("    {} - Intelligent routing based on query complexity", "hybrid".bright_blue());
            println!();
            println!("  To change mode, use: /mode <mode_name>");
            println!("  Example: /mode auto");
            println!();
            Ok(CommandResult::Handled)
        }

        _ => {
            println!("\n  Unknown command: /{}", command);
            println!("  Type /help for available commands.\n");
            Ok(CommandResult::Handled)
        }
    }
}

/// Print help for slash commands
fn print_help() {
    println!();
    println!("  Available Commands:");
    println!("  {}", "-".repeat(50));
    println!("    /save, /s          Save current conversation");
    println!("    /resume, /r [n]    Resume a saved conversation");
    println!("    /history, /ls      List all saved conversations");
    println!("    /delete, /rm [n]   Delete a saved conversation");
    println!("    /new, /clear       Start a new conversation");
    println!("    /autosave, /auto   Toggle auto-save on exit");
    println!("    /status, /stat     Show current status (model, GPU, session)");
    println!("    /model, /m         Show/change current model");
    println!("    /mode              Show/change routing mode (local/cloud/auto/hybrid)");
    println!("    /help, /h, /?      Show this help message");
    println!("    /exit, /quit, /q   Exit the chat");
    println!();
    println!("  Tips:");
    println!("    - Type 'exit' or 'quit' to leave chat mode");
    println!("    - Use /resume to continue where you left off");
    println!("    - Enable /autosave to never lose your work");
    println!("    - Use /status to check GPU and session info");
    println!("    - Press Ctrl+C during response to interrupt");
    println!();
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::tempdir;

    #[test]
    fn test_saved_conversation_new() {
        let messages = vec![
            Message::user("Hello, how are you?"),
            Message::assistant("I'm doing well, thank you!"),
        ];

        let conv = SavedConversation::new("test-model", messages);

        assert!(conv.id.starts_with("conv_"));
        assert_eq!(conv.summary, "Hello, how are you?");
        assert_eq!(conv.model, "test-model");
        assert_eq!(conv.message_count, 2);
    }

    #[test]
    fn test_summary_truncation() {
        let long_message = "A".repeat(100);
        let messages = vec![Message::user(&long_message)];

        let conv = SavedConversation::new("test-model", messages);

        assert!(conv.summary.len() <= 60);
        assert!(conv.summary.ends_with("..."));
    }

    #[test]
    fn test_time_ago_formatting() {
        let mut conv = SavedConversation::new("test", vec![Message::user("test")]);

        // Just now
        assert_eq!(conv.time_ago(), "Just now");

        // Modify to 2 hours ago
        conv.updated_at = Utc::now() - chrono::Duration::hours(2);
        assert_eq!(conv.time_ago(), "2 hours ago");

        // Yesterday
        conv.updated_at = Utc::now() - chrono::Duration::days(1);
        assert_eq!(conv.time_ago(), "Yesterday");
    }
}
