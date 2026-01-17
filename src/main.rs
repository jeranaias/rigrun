// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

use anyhow::{Context, Result};
use chrono;
use clap::{Parser, Subcommand};
use colored::Colorize;
use serde::{Deserialize, Serialize};
use std::fs;
use std::io::{self, BufRead, IsTerminal, Read, Write};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::cell::RefCell;
use std::path::PathBuf;
use std::time::Instant;
use crossterm::{
    cursor,
    terminal::{Clear, ClearType},
    ExecutableCommand,
};
use unicode_width::UnicodeWidthStr;

// Use the library's detect module
use rigrun::detect::{
    detect_gpu, recommend_model, GpuInfo, GpuType, is_model_available, list_ollama_models,
    check_gpu_utilization, ProcessorType, get_gpu_status_report,
    get_gpu_setup_status, AmdArchitecture, detect_amd_architecture,
};
use rigrun::local::{OllamaClient, Message};
use rigrun::cli::{InteractiveInput, show_help, show_command_menu};

mod background;
mod consent_banner;
mod conversation_store;

// Use firstrun from the library crate
use rigrun::firstrun;

const VERSION: &str = env!("CARGO_PKG_VERSION");
const DEFAULT_PORT: u16 = 8787;

// ANSI color codes for terminal output
mod colors {
    pub const RESET: &str = "\x1b[0m";
    pub const BOLD: &str = "\x1b[1m";
    pub const DIM: &str = "\x1b[2m";
    pub const RED: &str = "\x1b[31m";
    pub const GREEN: &str = "\x1b[32m";
    pub const YELLOW: &str = "\x1b[33m";
    pub const BLUE: &str = "\x1b[34m";
    pub const CYAN: &str = "\x1b[36m";
    pub const WHITE: &str = "\x1b[37m";
    pub const BRIGHT_CYAN: &str = "\x1b[96m";
}

use colors::*;

/// Exit codes following sysexits.h conventions
/// These provide meaningful exit status to calling processes and scripts
mod exit_codes {
    /// Success - operation completed successfully
    pub const SUCCESS: i32 = 0;
    /// General error - unspecified error
    pub const ERROR: i32 = 1;
    /// Usage error - invalid command line arguments
    pub const USAGE: i32 = 64;
    /// Data error - invalid input data format
    pub const DATA_ERR: i32 = 65;
    /// Service unavailable - required service (Ollama) not running
    pub const SERVICE_UNAVAILABLE: i32 = 69;
    /// Internal software error - unexpected condition
    pub const SOFTWARE: i32 = 70;
    /// I/O error - network or file operation failed
    pub const IO_ERR: i32 = 74;
    /// Temporary failure - try again later
    pub const TEMP_FAIL: i32 = 75;
    /// Configuration error - invalid or missing config
    pub const CONFIG: i32 = 78;
}

use exit_codes::*;

/// Spinner helpers for consistent progress indicators
mod spinner {
    use indicatif::{ProgressBar, ProgressStyle};
    use std::time::Duration;

    /// Create a spinner with consistent styling
    pub fn create(message: &str) -> ProgressBar {
        let spinner = ProgressBar::new_spinner();
        spinner.set_style(
            ProgressStyle::default_spinner()
                .tick_chars("\u{28FB}\u{28F9}\u{28FC}\u{28F8}\u{28FE}\u{28F6}\u{28F7}\u{28E7}\u{28CF}\u{28DF} ")
                .template("{spinner:.cyan} {msg}")
                .unwrap()
        );
        spinner.set_message(message.to_string());
        spinner.enable_steady_tick(Duration::from_millis(80));
        spinner
    }

    /// Finish spinner with success message
    pub fn finish_success(spinner: &ProgressBar, message: &str) {
        spinner.finish_and_clear();
        println!("\x1b[32m[OK]\x1b[0m {}", message);
    }

    /// Finish spinner with warning message
    pub fn finish_warning(spinner: &ProgressBar, message: &str) {
        spinner.finish_and_clear();
        println!("\x1b[33m[!]\x1b[0m {}", message);
    }

    /// Finish spinner with error message
    pub fn finish_error(spinner: &ProgressBar, message: &str) {
        spinner.finish_and_clear();
        println!("\x1b[31m[X]\x1b[0m {}", message);
    }

    /// Clear spinner silently (for use when transitioning to streaming output)
    pub fn clear(spinner: &ProgressBar) {
        spinner.finish_and_clear();
    }
}

/// Clear the terminal screen
fn clear_screen() {
    print!("\x1B[2J\x1B[1;1H");
    std::io::Write::flush(&mut std::io::stdout()).ok();
}

/// Strip ANSI escape codes from a string for accurate display width calculation
fn strip_ansi_codes(s: &str) -> String {
    let re = regex::Regex::new(r"\x1b\[[0-9;]*m").unwrap();
    re.replace_all(s, "").to_string()
}

/// Calculate the display width of a string, accounting for ANSI escape codes and unicode
fn display_width(s: &str) -> usize {
    let stripped = strip_ansi_codes(s);
    UnicodeWidthStr::width(stripped.as_str())
}

/// Pad a string to a target width, correctly accounting for ANSI escape codes
/// This ensures columns align properly even when strings contain color codes
fn pad_display(s: &str, target_width: usize) -> String {
    let current_width = display_width(s);
    if current_width >= target_width {
        s.to_string()
    } else {
        format!("{}{}", s, " ".repeat(target_width - current_width))
    }
}

/// Tracks code block state during streaming for syntax highlighting
/// Handles token-by-token streaming where ``` may be split across chunks
struct CodeBlockTracker {
    in_code_block: bool,
    pending_backticks: String,  // Buffer for partial ``` sequences
    language: Option<String>,   // Language hint after opening ```
    collecting_language: bool,  // True right after opening ``` to collect language
}

impl CodeBlockTracker {
    fn new() -> Self {
        Self {
            in_code_block: false,
            pending_backticks: String::new(),
            language: None,
            collecting_language: false,
        }
    }

    /// Process a streaming token and print with appropriate styling
    fn process_token(&mut self, token: &str) {
        for ch in token.chars() {
            if ch == '`' {
                self.pending_backticks.push(ch);

                // Check if we have a complete ``` sequence
                if self.pending_backticks == "```" {
                    self.toggle_code_block();
                    self.pending_backticks.clear();
                }
            } else {
                // Non-backtick character
                if !self.pending_backticks.is_empty() {
                    // We had partial backticks that didn't complete - flush them
                    let pending = self.pending_backticks.clone();
                    self.pending_backticks.clear();
                    self.print_text(&pending);
                }

                if self.collecting_language {
                    // After opening ```, collect language until newline
                    if ch == '\n' {
                        self.collecting_language = false;
                        // Print the code block header with styling
                        let lang_display = self.language.as_deref().unwrap_or("");
                        print!("{}", format!("```{}\n", lang_display).cyan());
                    } else if !ch.is_whitespace() {
                        // Accumulate language identifier
                        if self.language.is_none() {
                            self.language = Some(String::new());
                        }
                        if let Some(ref mut lang) = self.language {
                            lang.push(ch);
                        }
                    }
                } else {
                    self.print_char(ch);
                }
            }
        }
    }

    fn toggle_code_block(&mut self) {
        if self.in_code_block {
            // Closing code block
            print!("{}", "```".cyan());
            self.in_code_block = false;
            self.language = None;
        } else {
            // Opening code block - start collecting language
            self.in_code_block = true;
            self.collecting_language = true;
            self.language = None;
            // Don't print ``` yet - wait until we have the full header line
        }
    }

    fn print_char(&self, ch: char) {
        if self.in_code_block {
            // Code content - use distinct styling (bright cyan)
            print!("{}", format!("{}", ch).bright_cyan());
        } else {
            // Normal text
            print!("{}", ch);
        }
    }

    fn print_text(&self, text: &str) {
        if self.in_code_block {
            print!("{}", text.bright_cyan());
        } else {
            print!("{}", text);
        }
    }

    /// Flush any pending state at end of stream
    fn flush(&mut self) {
        if !self.pending_backticks.is_empty() {
            let pending = self.pending_backticks.clone();
            self.pending_backticks.clear();
            self.print_text(&pending);
        }
    }
}

/// RigRun - Local-first LLM router. Your GPU first, cloud when needed.
#[derive(Parser)]
#[command(name = "rigrun")]
#[command(version = VERSION)]
#[command(about = "Local-first LLM router. Your GPU first, cloud when needed.")]
#[command(long_about = "RigRun - Local-first LLM router\n\n\
    Start the server:    rigrun\n\
    Quick question:      rigrun ask \"What is Rust?\"\n\
    Interactive chat:    rigrun chat\n\
    Check status:        rigrun status (or: rigrun s)\n\
    Configure:           rigrun config show\n\
    Get help:            rigrun doctor\n\n\
    Your GPU runs local models first. Cloud fallback only when needed.")]
#[command(propagate_version = true)]
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,

    /// Direct prompt to send to the model (or start server if not provided)
    prompt: Option<String>,

    /// Paranoid mode: block ALL cloud requests (local-only operation)
    #[arg(long, global = true)]
    paranoid: bool,

    /// Skip DoD consent banner (for CI/automated environments - will be logged)
    #[arg(long, global = true)]
    skip_banner: bool,

    /// Skip the first-run wizard (use defaults)
    #[arg(long, global = true)]
    no_wizard: bool,

    /// Run quick setup wizard (minimal prompts, recommended defaults)
    #[arg(long, global = true)]
    quick_setup: bool,

    /// Quiet mode: minimal output, only essential information
    #[arg(short = 'q', long, global = true)]
    quiet: bool,

    /// Verbose mode: detailed output for debugging
    #[arg(short = 'v', long, global = true)]
    verbose: bool,
}

#[derive(Subcommand)]
enum Commands {
    /// Ask a single question (simplest way to use rigrun)
    ///
    /// Examples:
    ///   rigrun ask "What is Rust?"
    ///   rigrun ask "Explain closures" --model qwen2.5-coder:7b
    ///   rigrun ask "Review this:" --file code.rs
    Ask {
        /// The question to ask
        question: Option<String>,
        /// Model to use (defaults to local)
        #[arg(short, long)]
        model: Option<String>,
        /// File to include with the question (content appended after question)
        #[arg(short, long)]
        file: Option<String>,
    },

    /// Start interactive chat session
    ///
    /// Examples:
    ///   rigrun chat
    ///   rigrun chat --model qwen2.5-coder:14b
    Chat {
        /// Model to use (defaults to configured model)
        #[arg(short, long)]
        model: Option<String>,
    },

    /// Show current stats and server status
    ///
    /// Examples:
    ///   rigrun status
    ///   rigrun s
    #[command(alias = "s")]
    Status,

    /// Configure settings
    ///
    /// Examples:
    ///   rigrun config show
    ///   rigrun config set-key sk-or-v1-xxx
    ///   rigrun config set-model qwen2.5-coder:7b
    ///   rigrun config set-port 8080
    Config {
        #[command(subcommand)]
        command: Option<ConfigCommands>,
    },

    /// Unified setup wizard - ONE command to rule them all
    ///
    /// Replaces 6 conflicting docs and 20+ manual steps with a single command.
    /// Auto-detects hardware, downloads optimal model, generates secure config.
    ///
    /// Examples:
    ///   rigrun setup              # Auto-detect everything
    ///   rigrun setup --quick      # Essential setup only
    ///   rigrun setup --full       # Full setup with all features
    ///   rigrun setup --hardware nvidia   # Force NVIDIA mode
    ///   rigrun setup --hardware amd      # Force AMD mode
    ///   rigrun setup --hardware cpu      # Force CPU-only mode
    ///   rigrun setup ide          # Legacy: IDE setup only
    ///   rigrun setup gpu          # Legacy: GPU setup only
    Setup {
        /// Quick setup - just the essentials to get running
        #[arg(long, conflicts_with = "full")]
        quick: bool,

        /// Full setup with all features and optimizations
        #[arg(long, conflicts_with = "quick")]
        full: bool,

        /// Hardware mode: auto, nvidia, amd, or cpu
        #[arg(long, value_name = "MODE")]
        hardware: Option<String>,

        /// Legacy subcommands (ide, gpu)
        #[command(subcommand)]
        command: Option<SetupCommands>,
    },

    /// Cache operations
    ///
    /// Examples:
    ///   rigrun cache         (shows stats)
    ///   rigrun cache stats
    ///   rigrun cache clear
    ///   rigrun cache export
    Cache {
        #[command(subcommand)]
        command: Option<CacheCommands>,
    },

    /// Diagnose system health and configuration
    ///
    /// Examples:
    ///   rigrun doctor
    ///   rigrun doctor --fix
    Doctor {
        /// Auto-fix issues where possible
        #[arg(long)]
        fix: bool,
        /// Check network connectivity for hybrid cloud mode
        #[arg(long)]
        check_network: bool,
    },

    /// List available and downloaded models
    ///
    /// Examples:
    ///   rigrun models
    #[command(alias = "m")]
    Models,

    /// Download a specific model
    ///
    /// Examples:
    ///   rigrun pull qwen2.5-coder:7b
    ///   rigrun pull deepseek-coder-v2:16b
    Pull {
        /// Model name to download (e.g., qwen2.5-coder:14b)
        model: String,
    },

    // Legacy commands (kept for backward compatibility but hidden)
    #[command(hide = true)]
    Examples,

    #[command(hide = true)]
    Background,

    #[command(hide = true)]
    Stop,

    #[command(hide = true)]
    IdeSetup,

    #[command(hide = true)]
    GpuSetup,

    #[command(hide = true)]
    Export {
        #[arg(short, long)]
        output: Option<PathBuf>,
    },
}

#[derive(Subcommand)]
enum ConfigCommands {
    /// Show current configuration
    ///
    /// Example:
    ///   rigrun config show
    Show,

    /// Set OpenRouter API key for cloud fallback
    ///
    /// Example:
    ///   rigrun config set-key sk-or-v1-xxx
    SetKey {
        /// OpenRouter API key
        key: String,
    },

    /// Override default model
    ///
    /// Example:
    ///   rigrun config set-model qwen2.5-coder:7b
    SetModel {
        /// Model name
        model: String,
    },

    /// Change server port
    ///
    /// Example:
    ///   rigrun config set-port 8080
    SetPort {
        /// Port number
        port: u16,
    },
}

#[derive(Subcommand)]
enum SetupCommands {
    /// Set up IDE integration with rigrun
    ///
    /// Example:
    ///   rigrun setup ide
    Ide,

    /// Interactive GPU setup wizard (legacy - use 'rigrun setup' instead)
    ///
    /// Example:
    ///   rigrun setup gpu
    Gpu,

    /// Run unified setup wizard (same as 'rigrun setup')
    ///
    /// Example:
    ///   rigrun setup wizard
    #[command(hide = true)]
    Wizard {
        /// Quick setup - just the essentials
        #[arg(long)]
        quick: bool,
        /// Full setup with all features
        #[arg(long)]
        full: bool,
        /// Hardware mode: auto, nvidia, amd, or cpu
        #[arg(long)]
        hardware: Option<String>,
    },
}

#[derive(Subcommand)]
enum CacheCommands {
    /// Show cache statistics
    ///
    /// Example:
    ///   rigrun cache stats
    Stats,

    /// Clear the cache
    ///
    /// Example:
    ///   rigrun cache clear
    Clear,

    /// Export cache and audit log for backup
    ///
    /// Example:
    ///   rigrun cache export
    ///   rigrun cache export --output ./backups
    Export {
        /// Output directory for exported data (defaults to current directory)
        #[arg(short, long)]
        output: Option<PathBuf>,
    },
}

#[derive(Serialize, Deserialize, Default, Clone)]
pub struct Config {
    pub openrouter_key: Option<String>,
    pub model: Option<String>,
    pub port: Option<u16>,
    #[serde(default)]
    pub first_run_complete: bool,
    /// Enable audit logging (default: true)
    #[serde(default = "default_audit_log_enabled")]
    pub audit_log_enabled: bool,
    /// Paranoid mode: block all cloud requests (default: false)
    #[serde(default)]
    pub paranoid_mode: bool,
    /// Enable DoD consent banner on startup (default: true for IL5 compliance)
    /// Set to false for non-DoD deployments
    #[serde(default = "default_dod_banner_enabled")]
    pub dod_banner_enabled: bool,
    /// Show status line in interactive chat (default: true)
    #[serde(default = "default_show_status_line")]
    pub show_status_line: bool,
    /// Status line style: "full", "compact", or "minimal" (default: "compact")
    #[serde(default = "default_status_line_style")]
    pub status_line_style: String,
}

fn default_audit_log_enabled() -> bool {
    true
}

fn default_dod_banner_enabled() -> bool {
    true
}

fn default_show_status_line() -> bool {
    true
}

fn default_status_line_style() -> String {
    "compact".to_string()
}

#[derive(Serialize, Deserialize, Default)]
struct Stats {
    queries_today: u64,
    local_queries: u64,
    cloud_queries: u64,
    money_saved: f64,
    last_reset: Option<String>,
}

fn get_config_dir() -> Result<PathBuf> {
    let home = dirs::home_dir().context("Could not find home directory")?;
    let config_dir = home.join(".rigrun");
    if !config_dir.exists() {
        fs::create_dir_all(&config_dir)?;
    }
    Ok(config_dir)
}

/// Validate OpenRouter API key format and warn about common mistakes.
/// Returns true if key looks valid, false otherwise.
fn validate_openrouter_key(key: &str) -> bool {
    let key = key.trim();

    // Check for empty key
    if key.is_empty() {
        return false;
    }

    // Check for common wrong key formats
    if key.starts_with("sk-ant-") {
        eprintln!(
            "{YELLOW}[!]{RESET} Warning: This looks like an Anthropic API key (starts with 'sk-ant-')."
        );
        eprintln!(
            "{YELLOW}[!]{RESET}          OpenRouter keys start with 'sk-or-'. Get one at: https://openrouter.ai/keys"
        );
        return false;
    }

    if key.starts_with("sk-") && !key.starts_with("sk-or-") {
        eprintln!(
            "{YELLOW}[!]{RESET} Warning: This looks like an OpenAI API key (starts with 'sk-')."
        );
        eprintln!(
            "{YELLOW}[!]{RESET}          OpenRouter keys start with 'sk-or-'. Get one at: https://openrouter.ai/keys"
        );
        return false;
    }

    // Check for correct OpenRouter format
    if !key.starts_with("sk-or-") {
        eprintln!(
            "{YELLOW}[!]{RESET} Warning: OpenRouter API key doesn't start with 'sk-or-'."
        );
        eprintln!(
            "{YELLOW}[!]{RESET}          This may not be a valid OpenRouter key. Get one at: https://openrouter.ai/keys"
        );
        return false;
    }

    true
}

pub fn load_config() -> Result<Config> {
    let config_path = get_config_dir()?.join("config.json");
    let config = if config_path.exists() {
        let content = fs::read_to_string(&config_path)?;
        serde_json::from_str(&content)?
    } else {
        Config::default()
    };

    // Validate OpenRouter key if present
    if let Some(ref key) = config.openrouter_key {
        validate_openrouter_key(key);
    }

    Ok(config)
}

pub fn save_config(config: &Config) -> Result<()> {
    let config_path = get_config_dir()?.join("config.json");
    let content = serde_json::to_string_pretty(config)?;
    fs::write(config_path, content)?;
    Ok(())
}

fn load_stats() -> Result<Stats> {
    let stats_path = get_config_dir()?.join("stats.json");
    if stats_path.exists() {
        let content = fs::read_to_string(&stats_path)?;
        Ok(serde_json::from_str(&content)?)
    } else {
        Ok(Stats::default())
    }
}

/// Spawn a background task that updates the stats display in real-time
fn spawn_stats_updater() -> tokio::task::JoinHandle<()> {
    tokio::spawn(async {
        // Wait a moment for the initial display to render
        tokio::time::sleep(tokio::time::Duration::from_millis(500)).await;

        loop {
            // Update every 2 seconds
            tokio::time::sleep(tokio::time::Duration::from_secs(2)).await;

            // Get current stats from the global tracker
            let all_time = rigrun::stats::global_tracker().get_all_time_stats();
            let _session = rigrun::stats::get_session_stats();

            // Get today's data
            let today = chrono::Utc::now().format("%Y-%m-%d").to_string();
            let today_data = all_time.daily_savings
                .iter()
                .find(|d| d.date == today);

            let (today_queries, today_saved, today_spent) = today_data
                .map(|d| (d.queries, d.saved, d.spent))
                .unwrap_or((0, 0.0, 0.0));

            // Calculate local queries from all-time cumulative stats
            // This shows total counts across all time, which is better than 0
            let local_queries = all_time.local_queries + all_time.cache_hits;
            let cloud_queries = all_time.cloud_queries;

            // Calculate savings percentage
            let total_cost_if_cloud = today_saved + today_spent;
            let savings_pct = if total_cost_if_cloud > 0.0 {
                (today_saved / total_cost_if_cloud) * 100.0
            } else {
                0.0
            };

            // Move cursor up 5 lines to overwrite the stats area
            // Use \r to return to start of line for each update
            let mut stdout = io::stdout();

            // Move up 5 lines (past the blank line, "Press Ctrl+C", separator, 2 stat lines)
            let _ = stdout.execute(cursor::MoveUp(5));

            // Clear and write the separator
            let _ = stdout.execute(Clear(ClearType::CurrentLine));
            println!("  {DIM}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━{RESET}");

            // Clear and write queries line
            let _ = stdout.execute(Clear(ClearType::CurrentLine));
            println!("  Queries: {WHITE}{BOLD}{}{RESET}    Local: {GREEN}{}{RESET}    Cloud: {YELLOW}{}{RESET}",
                today_queries, local_queries, cloud_queries);

            // Clear and write savings line
            let _ = stdout.execute(Clear(ClearType::CurrentLine));
            println!("  Saved: {GREEN}{BOLD}${:.2}{RESET}   Spent: {YELLOW}${:.2}{RESET}  ({GREEN}{:.0}% savings{RESET})",
                today_saved, today_spent, savings_pct);

            // Clear and write bottom separator
            let _ = stdout.execute(Clear(ClearType::CurrentLine));
            println!("  {DIM}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━{RESET}");

            // Move down to original position
            let _ = stdout.execute(Clear(ClearType::CurrentLine));
            println!("{DIM}Press Ctrl+C to stop{RESET}");

            let _ = stdout.flush();
        }
    })
}

fn print_banner() {
    println!(
        "{BRIGHT_CYAN}
   ____  _       ____
  |  _ \\(_) __ _|  _ \\ _   _ _ __
  | |_) | |/ _` | |_) | | | | '_ \\
  |  _ <| | (_| |  _ <| |_| | | | |
  |_| \\_\\_|\\__, |_| \\_\\\\__,_|_| |_|
           |___/{RESET}  v{VERSION}
{DIM}Local-first LLM router • Your GPU first, cloud when needed{RESET}"
    );
    println!();
}

#[allow(dead_code)]
fn format_gpu_info(gpu: &GpuInfo) -> String {
    if gpu.gpu_type == GpuType::Cpu {
        format!("{DIM}None detected{RESET}")
    } else {
        format!("{WHITE}{BOLD}{}{RESET} ({}GB)", gpu.name, gpu.vram_gb)
    }
}

fn check_model_downloaded(model: &str) -> bool {
    is_model_available(model)
}

async fn download_model(model: &str) -> Result<()> {
    use indicatif::{ProgressBar, ProgressStyle};
    use std::time::Duration;

    // Check if already downloaded
    if is_model_available(model) {
        println!("  {GREEN}[✓]{RESET} Model {model} already downloaded");
        return Ok(());
    }

    println!("  {YELLOW}[↓]{RESET} Downloading {model}...");
    println!("      {DIM}This is a one-time download. Future starts are instant.{RESET}");

    let client = OllamaClient::new();
    let model_name = model.to_string();

    // Create a progress bar with a nice style
    let pb = ProgressBar::new(100);
    pb.set_style(
        ProgressStyle::default_bar()
            .template("      {spinner:.green} [{bar:40.cyan/blue}] {pos:>3}% | {msg}")
            .unwrap()
            .progress_chars("█▓░")
    );
    pb.enable_steady_tick(Duration::from_millis(100));

    // Track state for progress updates
    let pb_clone = pb.clone();
    let mut last_percentage: u64 = 0;
    let mut current_status = String::new();

    // Run the download in a blocking task since OllamaClient uses blocking reqwest
    let result = tokio::task::spawn_blocking(move || {
        client.pull_model_with_progress(&model_name, |progress| {
            // Update status message if changed
            if progress.status != current_status {
                current_status = progress.status.clone();
                let status_msg = match current_status.as_str() {
                    s if s.contains("pulling manifest") => "Fetching manifest...",
                    s if s.contains("pulling") => "Downloading layers...",
                    s if s.contains("verifying") => "Verifying...",
                    s if s.contains("writing") => "Writing to disk...",
                    s if s.contains("success") => "Complete!",
                    _ => &current_status,
                };
                pb_clone.set_message(status_msg.to_string());
            }

            // Update progress bar position
            if let Some(pct) = progress.percentage() {
                let pct_int = pct as u64;
                if pct_int != last_percentage {
                    last_percentage = pct_int;
                    pb_clone.set_position(pct_int);

                    // Show size info if available
                    if let (Some(completed), Some(total)) = (progress.completed, progress.total) {
                        let completed_gb = completed as f64 / 1_073_741_824.0;
                        let total_gb = total as f64 / 1_073_741_824.0;
                        if total_gb >= 0.1 {
                            pb_clone.set_message(format!("{:.1} GB / {:.1} GB", completed_gb, total_gb));
                        }
                    }
                }
            }
        })
    })
    .await
    .map_err(|e| anyhow::anyhow!("Task join error: {}", e))?;

    pb.finish_and_clear();

    match result {
        Ok(()) => {
            println!("  {GREEN}[✓]{RESET} Model {model} ready");
            Ok(())
        }
        Err(e) => {
            let err_str = e.to_string();
            if err_str.contains("Cannot connect") || err_str.contains("not running") {
                println!("  {RED}[✗]{RESET} Ollama not running. Start it with: ollama serve");
                anyhow::bail!("Ollama not running")
            } else if err_str.contains("not found") {
                println!("  {RED}[✗]{RESET} Model not found: {model}");
                anyhow::bail!("Model not found: {}", model)
            } else {
                println!("  {RED}[✗]{RESET} Failed to download model: {}", e);
                anyhow::bail!("Failed to download model: {}", e)
            }
        }
    }
}

fn check_server_running(port: u16) -> bool {
    // Check if server is already running on the port
    std::net::TcpListener::bind(format!("127.0.0.1:{}", port)).is_err()
}

#[cfg(target_os = "windows")]
fn find_process_on_port(port: u16) -> Option<u32> {
    // Use netstat to find the process ID using the port
    let output = std::process::Command::new("netstat")
        .args(["-ano"])
        .output()
        .ok()?;

    let output_str = String::from_utf8_lossy(&output.stdout);
    let port_str = format!(":{}", port);

    for line in output_str.lines() {
        if line.contains(&port_str) && line.contains("LISTENING") {
            // Extract PID from the last column
            if let Some(pid_str) = line.split_whitespace().last() {
                if let Ok(pid) = pid_str.parse::<u32>() {
                    return Some(pid);
                }
            }
        }
    }
    None
}

#[cfg(not(target_os = "windows"))]
fn find_process_on_port(port: u16) -> Option<u32> {
    // Use lsof on Unix-like systems
    let output = std::process::Command::new("lsof")
        .args(["-ti", &format!(":{}", port)])
        .output()
        .ok()?;

    let output_str = String::from_utf8_lossy(&output.stdout);
    output_str.trim().parse::<u32>().ok()
}

#[cfg(target_os = "windows")]
fn kill_process(pid: u32) -> Result<()> {
    let status = std::process::Command::new("taskkill")
        .args(["/F", "/PID", &pid.to_string()])
        .status()?;

    if status.success() {
        Ok(())
    } else {
        anyhow::bail!("Failed to kill process {}", pid)
    }
}

#[cfg(not(target_os = "windows"))]
fn kill_process(pid: u32) -> Result<()> {
    let status = std::process::Command::new("kill")
        .args(["-9", &pid.to_string()])
        .status()?;

    if status.success() {
        Ok(())
    } else {
        anyhow::bail!("Failed to kill process {}", pid)
    }
}

fn is_rigrun_process(pid: u32) -> bool {
    #[cfg(target_os = "windows")]
    {
        if let Ok(output) = std::process::Command::new("tasklist")
            .args(["/FI", &format!("PID eq {}", pid), "/FO", "CSV", "/NH"])
            .output()
        {
            let output_str = String::from_utf8_lossy(&output.stdout);
            return output_str.contains("rigrun.exe");
        }
        false
    }

    #[cfg(not(target_os = "windows"))]
    {
        if let Ok(output) = std::process::Command::new("ps")
            .args(["-p", &pid.to_string(), "-o", "comm="])
            .output()
        {
            let output_str = String::from_utf8_lossy(&output.stdout);
            return output_str.contains("rigrun");
        }
        false
    }
}

async fn start_server(config: &Config) -> Result<()> {
    let mut port = config.port.unwrap_or(DEFAULT_PORT);

    // Start server immediately - GPU detection happens in background
    println!("{BLUE}[i]{RESET} Starting server...");

    // Spawn GPU detection in background (non-blocking) with animated spinner
    let gpu_spinner = spinner::create("Detecting GPU...");
    let gpu_handle = tokio::spawn(async {
        tokio::time::timeout(
            tokio::time::Duration::from_secs(5),
            tokio::task::spawn_blocking(detect_gpu)
        ).await
    });

    // Use default GPU info initially (conservative - CPU mode)
    let mut gpu = GpuInfo::default();

    // Ensure Ollama is running with default settings first
    // (will work for most cases; RDNA 4 users may need manual Vulkan setup)
    ensure_ollama_running(&gpu).await?;

    // Wait briefly for GPU detection - if it's fast, we get the real info
    // If slow, we continue with defaults and log when detection completes
    tokio::select! {
        result = gpu_handle => {
            match result {
                Ok(Ok(Ok(Ok(gpu_info)))) => {
                    spinner::finish_success(&gpu_spinner, &format!("GPU: {} ({} GB VRAM)", gpu_info.name, gpu_info.vram_gb));
                    gpu = gpu_info;
                }
                _ => {
                    spinner::finish_warning(&gpu_spinner, "GPU: using defaults");
                }
            }
        }
        _ = tokio::time::sleep(tokio::time::Duration::from_millis(500)) => {
            // Timeout - GPU detection still running, continue with defaults
            spinner::finish_warning(&gpu_spinner, "GPU: detection in background, using defaults");
        }
    }

    let model = config
        .model
        .clone()
        .unwrap_or_else(|| recommend_model(gpu.vram_gb));

    // Model status - download if needed
    if !check_model_downloaded(&model) {
        download_model(&model).await?;
    }

    // Handle port conflicts
    if check_server_running(port) {
        println!(
            "{YELLOW}[!]{RESET} Port {port} is already in use"
        );

        // Try to find the process using the port
        if let Some(pid) = find_process_on_port(port) {
            if is_rigrun_process(pid) {
                println!(
                    "{YELLOW}[!]{RESET} Found existing rigrun server (PID: {pid})"
                );
                println!(
                    "{YELLOW}[!]{RESET} Attempting to stop old server..."
                );

                if let Err(e) = kill_process(pid) {
                    println!(
                        "{RED}[✗]{RESET} Failed to stop old server: {}",
                        e
                    );
                    println!(
                        "{YELLOW}[!]{RESET} Searching for next available port..."
                    );
                } else {
                    println!("{GREEN}[✓]{RESET} Old server stopped");
                    // Wait a moment for the port to be released
                    tokio::time::sleep(tokio::time::Duration::from_millis(500)).await;

                    // Verify port is now free
                    if check_server_running(port) {
                        println!(
                            "{YELLOW}[!]{RESET} Port still in use, searching for next available port..."
                        );
                    } else {
                        println!("{GREEN}[✓]{RESET} Port {port} is now available");
                    }
                }
            } else {
                println!(
                    "{YELLOW}[!]{RESET} Port is used by another process (PID: {pid})"
                );
                println!(
                    "{YELLOW}[!]{RESET} Searching for next available port..."
                );
            }
        } else {
            println!(
                "{YELLOW}[!]{RESET} Searching for next available port..."
            );
        }

        // Find next available port if still in use
        let original_port = port;
        while check_server_running(port) {
            port += 1;
            if port > original_port + 20 {
                anyhow::bail!("Could not find available port in range {}-{}", original_port, port);
            }
        }

        if port != original_port {
            println!(
                "{GREEN}[✓]{RESET} Found available port: {port}"
            );
        }
    }

    // Display clean server dashboard
    println!();
    println!("  Server: {CYAN}http://localhost:{port}{RESET}");
    println!("  Model:  {WHITE}{model}{RESET}");
    println!();
    println!("  {DIM}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━{RESET}");

    // Reserve space for live stats (will be updated by background task)
    println!("  Queries: 0    Local: 0    Cloud: 0");
    println!("  Saved: $0.00   Spent: $0.00  (0% savings)");

    println!("  {DIM}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━{RESET}");
    println!();
    println!("{DIM}Press Ctrl+C to stop{RESET}");
    println!();

    // Normal startup - spawn the stats updater task
    let _stats_updater = spawn_stats_updater();

    // Start the actual server
    let mut server = rigrun::Server::new(port)
        .with_default_model(model)
        .with_paranoid_mode(config.paranoid_mode);
    if let Some(ref key) = config.openrouter_key {
        server = server.with_openrouter_key(key.clone());
    }
    server.start().await?;

    Ok(())
}

fn show_status() -> Result<()> {
    let config = load_config()?;
    let stats = load_stats()?;
    let port = config.port.unwrap_or(DEFAULT_PORT);

    println!();
    println!("{BRIGHT_CYAN}{BOLD}=== RigRun Status ==={RESET}");
    println!();

    // Server status
    let server_running = check_server_running(port);
    if server_running {
        println!(
            "{GREEN}[✓]{RESET} Server: {GREEN}{BOLD}Running{RESET} on port {port}"
        );
    } else {
        println!("{RED}[✗]{RESET} Server: {RED}Not running{RESET}");
    }

    // Get comprehensive GPU status report
    let gpu_report = get_gpu_status_report();
    let gpu = &gpu_report.gpu_info;

    // Current model
    let model = config
        .model
        .clone()
        .unwrap_or_else(|| recommend_model(gpu.vram_gb));
    println!("{BLUE}[i]{RESET} Model: {WHITE}{BOLD}{model}{RESET}");

    // GPU
    if gpu.gpu_type == GpuType::Cpu {
        println!("{BLUE}[i]{RESET} GPU: {DIM}None (CPU mode){RESET}");
    } else {
        println!(
            "{BLUE}[i]{RESET} GPU: {} ({}GB)",
            gpu.name, gpu.vram_gb
        );
    }

    // Real-time VRAM usage
    if let Some(ref usage) = gpu_report.vram_usage {
        let usage_pct = usage.usage_percent();
        let usage_color = if usage_pct > 90.0 {
            RED
        } else if usage_pct > 70.0 {
            YELLOW
        } else {
            GREEN
        };
        println!(
            "{BLUE}[i]{RESET} VRAM: {usage_color}{}MB{RESET} / {}MB ({usage_color}{:.1}%{RESET} used)",
            usage.used_mb, usage.total_mb, usage_pct
        );
        if let Some(util) = usage.gpu_utilization {
            println!("{BLUE}[i]{RESET} GPU Utilization: {}%", util);
        }
    }

    // GPU Utilization - check loaded models
    println!();
    println!("{BRIGHT_CYAN}{BOLD}=== GPU Utilization ==={RESET}");
    println!();

    if gpu_report.loaded_models.is_empty() {
        println!("  {DIM}No models currently loaded in Ollama{RESET}");
    } else {
        for loaded in &gpu_report.loaded_models {
            let processor_color = match &loaded.processor {
                ProcessorType::Gpu(_) => GREEN,
                ProcessorType::Mixed { gpu_percent, .. } if *gpu_percent > 50 => YELLOW,
                ProcessorType::Cpu => RED,
                _ => YELLOW,
            };
            println!(
                "  {WHITE}{BOLD}{}{RESET} ({}) - {processor_color}{}{RESET}",
                loaded.name, loaded.size, loaded.processor
            );
        }
    }

    // Show any warnings from the GPU report
    if !gpu_report.warnings.is_empty() {
        println!();
        for warning in &gpu_report.warnings {
            println!("{YELLOW}[!]{RESET} {}", warning);
        }
    }

    // Check if current model will fit in VRAM
    if gpu.gpu_type != GpuType::Cpu {
        let gpu_status = check_gpu_utilization(&model, gpu);
        if let Some(ref warning) = gpu_status.warning {
            // Only show if not already in warnings
            if !gpu_report.warnings.iter().any(|w| w.contains(&model)) {
                println!();
                println!("{YELLOW}[!]{RESET} {}", warning);
                if let Some(ref suggested) = gpu_status.suggested_model {
                    println!(
                        "    Recommended: {CYAN}rigrun config --model {}{RESET}",
                        suggested
                    );
                }
            }
        }
    }

    println!();
    println!("{BRIGHT_CYAN}{BOLD}=== Today's Stats ==={RESET}");
    println!();
    println!(
        "  Total queries:  {WHITE}{BOLD}{}{RESET}",
        stats.queries_today
    );
    println!("  Local queries:  {GREEN}{}{RESET}", stats.local_queries);
    println!("  Cloud queries:  {YELLOW}{}{RESET}", stats.cloud_queries);
    println!(
        "  Money saved:    {GREEN}{BOLD}${:.2}{RESET}",
        stats.money_saved
    );
    println!();

    Ok(())
}

fn handle_config(command: Option<ConfigCommands>) -> Result<()> {
    let mut config = load_config()?;

    match command {
        None | Some(ConfigCommands::Show) => {
            println!();
            println!("{BRIGHT_CYAN}{BOLD}=== RigRun Configuration ==={RESET}");
            println!();

            let key_display = config
                .openrouter_key
                .as_ref()
                .map(|k| {
                    if k.len() > 8 {
                        format!("{}...", &k[..8])
                    } else {
                        format!("{}...", k)
                    }
                })
                .unwrap_or_else(|| format!("{DIM}(not set){RESET}"));
            println!("  OpenRouter Key: {}", key_display);

            let model_display = config
                .model.as_deref()
                .unwrap_or("(auto)");
            println!("  Model:          {}", model_display);

            println!("  Port:           {}", config.port.unwrap_or(DEFAULT_PORT));
            println!();

            if let Ok(config_dir) = get_config_dir() {
                println!(
                    "Config file: {}",
                    config_dir.join("config.json").display()
                );
            }
            println!();
        }
        Some(ConfigCommands::SetKey { key }) => {
            if !validate_openrouter_key(&key) {
                return Ok(());
            }
            config.openrouter_key = Some(key);
            save_config(&config)?;
            println!("{GREEN}[✓]{RESET} OpenRouter API key set");
            println!();
        }
        Some(ConfigCommands::SetModel { model }) => {
            config.model = Some(model.clone());
            save_config(&config)?;
            println!("{GREEN}[✓]{RESET} Model set to: {}", model);
            println!();
        }
        Some(ConfigCommands::SetPort { port }) => {
            config.port = Some(port);
            save_config(&config)?;
            println!("{GREEN}[✓]{RESET} Port set to: {}", port);
            println!();
        }
    }

    Ok(())
}

fn list_models() -> Result<()> {
    println!();
    println!("{BRIGHT_CYAN}{BOLD}=== Available Models ==={RESET}");
    println!();

    let models = [
        ("qwen2.5-coder:1.5b", "1.5B", "~1GB", "4GB+", "Fast, lightweight"),
        ("qwen2.5-coder:3b", "3B", "~2GB", "6GB+", "Good balance"),
        ("qwen2.5-coder:7b", "7B", "~4GB", "10GB+", "Recommended"),
        ("qwen2.5-coder:14b", "14B", "~8GB", "16GB+", "Best quality"),
        ("qwen2.5-coder:32b", "32B", "~18GB", "24GB+", "Maximum capability"),
        ("deepseek-coder-v2:16b", "16B", "~10GB", "16GB+", "Great for code"),
    ];

    // Get list of downloaded models from Ollama
    let downloaded_models = list_ollama_models();

    // Column widths for consistent alignment
    const COL_MODEL: usize = 25;
    const COL_SIZE: usize = 8;
    const COL_DISK: usize = 10;
    const COL_VRAM: usize = 10;

    println!(
        "  {DIM}{} {} {} {} NOTES{RESET}",
        pad_display("MODEL", COL_MODEL),
        pad_display("SIZE", COL_SIZE),
        pad_display("DISK", COL_DISK),
        pad_display("VRAM", COL_VRAM)
    );
    println!("  {DIM}{}{RESET}", "-".repeat(75));

    for (name, size, disk, vram, notes) in models {
        let is_downloaded = downloaded_models.iter().any(|m| m.starts_with(name.split(':').next().unwrap_or(name)));
        let status = if is_downloaded {
            format!("{GREEN}[✓]{RESET}")
        } else {
            "   ".to_string()
        };
        // Use pad_display to correctly handle colored text alignment
        let colored_name = format!("{WHITE}{}{RESET}", name);
        let colored_notes = format!("{DIM}{}{RESET}", notes);
        println!(
            "{} {} {} {} {} {}",
            pad_display(&status, 3),  // Status column (checkmark or spaces)
            pad_display(&colored_name, COL_MODEL),
            pad_display(size, COL_SIZE),
            pad_display(disk, COL_DISK),
            pad_display(vram, COL_VRAM),
            colored_notes
        );
    }

    println!();

    // Show recommendation based on detected GPU
    let gpu = detect_gpu().unwrap_or_default();
    let recommended = recommend_model(gpu.vram_gb);
    if gpu.gpu_type == GpuType::Cpu {
        println!(
            "{BLUE}[i]{RESET} No GPU detected. Recommended: {GREEN}{BOLD}{}{RESET}",
            recommended
        );
    } else {
        println!(
            "{BLUE}[i]{RESET} Recommended for your {} ({}GB): {GREEN}{BOLD}{}{RESET}",
            gpu.name, gpu.vram_gb, recommended
        );
    }

    println!();
    println!("To download a model: {CYAN}rigrun pull <model>{RESET}");
    println!();

    Ok(())
}

/// Ensure Ollama is running, auto-starting if needed with correct GPU settings
async fn ensure_ollama_running(gpu: &GpuInfo) -> Result<()> {
    // Check if Ollama is already running
    if check_ollama_running_quick() {
        println!("{GREEN}[✓]{RESET} Ollama: Running");
        return Ok(());
    }

    // Ollama not running - auto-start it
    println!("{YELLOW}[!]{RESET} Ollama not running, starting automatically...");

    // Determine if we need Vulkan for RDNA 4
    let needs_vulkan = gpu.gpu_type == GpuType::Amd && {
        let arch = detect_amd_architecture(&gpu.name);
        arch == AmdArchitecture::Rdna4
    };

    // Build the command with appropriate environment
    let mut cmd = std::process::Command::new("ollama");
    cmd.arg("serve");

    if needs_vulkan {
        cmd.env("OLLAMA_VULKAN", "1");
        println!("{BLUE}[i]{RESET} RDNA 4 detected - enabling Vulkan backend");
    }

    // Start Ollama in background (detached)
    #[cfg(target_os = "windows")]
    {
        use std::os::windows::process::CommandExt;
        const CREATE_NO_WINDOW: u32 = 0x08000000;
        const DETACHED_PROCESS: u32 = 0x00000008;
        cmd.creation_flags(CREATE_NO_WINDOW | DETACHED_PROCESS);
    }

    #[cfg(not(target_os = "windows"))]
    {
        cmd.stdout(std::process::Stdio::null());
        cmd.stderr(std::process::Stdio::null());
    }

    match cmd.spawn() {
        Ok(_) => {
            // Wait for Ollama to be ready (up to 10 seconds)
            let ollama_spinner = spinner::create("Waiting for Ollama to start...");

            for _ in 0..20 {
                tokio::time::sleep(tokio::time::Duration::from_millis(500)).await;
                if check_ollama_running_quick() {
                    spinner::finish_success(&ollama_spinner, "Ollama: Started successfully");
                    return Ok(());
                }
            }

            // Timeout - Ollama didn't start in time
            spinner::finish_error(&ollama_spinner, "Ollama failed to start within 10 seconds");
            anyhow::bail!(
                "Ollama failed to start. Please start manually:\n  \
                 Windows: {}\n  \
                 Linux/Mac: ollama serve",
                if needs_vulkan {
                    "set OLLAMA_VULKAN=1 && ollama serve"
                } else {
                    "ollama serve"
                }
            );
        }
        Err(e) => {
            println!("{RED}[X]{RESET} Failed to start Ollama: {}", e);
            anyhow::bail!(
                "Failed to start Ollama. Is it installed?\n  \
                 Download from: https://ollama.ai/download"
            );
        }
    }
}

/// Quick check if Ollama is running (no timeout, fast)
fn check_ollama_running_quick() -> bool {
    std::process::Command::new("ollama")
        .arg("list")
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null())
        .status()
        .map(|s| s.success())
        .unwrap_or(false)
}

async fn pull_model(model: String) -> Result<()> {
    println!();
    println!("{CYAN}[↓]{RESET} Pulling {WHITE}{BOLD}{model}{RESET}...");
    println!();

    use indicatif::{ProgressBar, ProgressStyle};
    use std::time::Duration;

    // Validate model name (allow any model that follows the pattern)
    let valid_prefixes = [
        "qwen2.5-coder",
        "deepseek-coder",
        "codellama",
        "llama",
        "mistral",
        "phi",
    ];

    let is_valid = valid_prefixes.iter().any(|prefix| model.starts_with(prefix))
        || model.contains(':'); // Allow any model with a tag

    if !is_valid {
        println!("{YELLOW}[!]{RESET} Warning: '{model}' may not be a known model.");
        println!("    Attempting to pull anyway...");
        println!();
    }

    let client = OllamaClient::new();

    // Check if Ollama is running first
    if !client.check_ollama_running() {
        println!("{RED}[✗]{RESET} Ollama is not running.");
        println!();
        println!("Make sure:");
        println!("  1. Ollama is installed (https://ollama.ai/download)");
        println!("  2. Ollama service is running: {CYAN}ollama serve{RESET}");
        println!();
        anyhow::bail!("Ollama not running");
    }

    // Create a progress bar with indicatif
    let pb = ProgressBar::new(100);
    pb.set_style(
        ProgressStyle::default_bar()
            .template("  {spinner:.green} [{bar:40.cyan/blue}] {pos:>3}% | {msg}")
            .unwrap()
            .progress_chars("█▓░")
    );
    pb.enable_steady_tick(Duration::from_millis(100));
    pb.set_message("Starting download...");

    // Track state for progress updates
    let pb_clone = pb.clone();
    let mut last_percentage: u64 = 0;
    let mut current_status = String::new();
    let model_clone = model.clone();

    // Run the download in a blocking task since OllamaClient uses blocking reqwest
    let result = tokio::task::spawn_blocking(move || {
        client.pull_model_with_progress(&model_clone, |progress| {
            // Update status message if changed
            if progress.status != current_status {
                current_status = progress.status.clone();
                let status_msg = match current_status.as_str() {
                    s if s.contains("pulling manifest") => "Fetching manifest...",
                    s if s.contains("pulling") => "Downloading layers...",
                    s if s.contains("verifying") => "Verifying...",
                    s if s.contains("writing") => "Writing to disk...",
                    s if s.contains("success") => "Complete!",
                    _ => &current_status,
                };
                pb_clone.set_message(status_msg.to_string());
            }

            // Update progress bar position
            if let Some(pct) = progress.percentage() {
                let pct_int = pct as u64;
                if pct_int != last_percentage {
                    last_percentage = pct_int;
                    pb_clone.set_position(pct_int);

                    // Show size info if available
                    if let (Some(completed), Some(total)) = (progress.completed, progress.total) {
                        let completed_gb = completed as f64 / 1_073_741_824.0;
                        let total_gb = total as f64 / 1_073_741_824.0;
                        if total_gb >= 0.1 {
                            pb_clone.set_message(format!("{:.1} GB / {:.1} GB", completed_gb, total_gb));
                        }
                    }
                }
            }
        })
    })
    .await
    .map_err(|e| anyhow::anyhow!("Task join error: {}", e))?;

    pb.finish_and_clear();

    println!();

    match result {
        Ok(()) => {
            println!(
                "{GREEN}[✓]{RESET} Model {WHITE}{BOLD}{model}{RESET} downloaded successfully!"
            );
            println!();
            println!("Start the server with: {CYAN}rigrun{RESET}");
        }
        Err(e) => {
            let err_str = e.to_string();
            if err_str.contains("not found") {
                println!("{RED}[✗]{RESET} Model not found: {model}");
            } else {
                println!("{RED}[✗]{RESET} Failed to download model: {model}");
                println!("    Error: {}", e);
            }
            println!();
            println!("Make sure:");
            println!("  1. Ollama service is running: {CYAN}ollama serve{RESET}");
            println!("  2. Model name is correct");
        }
    }

    println!();

    Ok(())
}

fn get_model_from_config() -> String {
    let config = load_config().ok();
    let gpu = detect_gpu().unwrap_or_default();

    config
        .and_then(|c| c.model)
        .unwrap_or_else(|| recommend_model(gpu.vram_gb))
}

fn ensure_model_available(client: &OllamaClient, model: &str) -> Result<()> {
    use indicatif::{ProgressBar, ProgressStyle};
    use std::time::Duration;

    if !client.check_ollama_running() {
        anyhow::bail!(
            "Ollama is not running. Please start it with: {}", "ollama serve".bright_cyan()
        );
    }

    if !client.has_model(model)? {
        println!(
            "{} Model {} not found. Downloading...",
            "[↓]".bright_yellow(),
            model.bright_white().bold()
        );

        // Create a progress bar with indicatif
        let pb = ProgressBar::new(100);
        pb.set_style(
            ProgressStyle::default_bar()
                .template("  {spinner:.green} [{bar:40.cyan/blue}] {pos:>3}% | {msg}")
                .unwrap()
                .progress_chars("█▓░")
        );
        pb.enable_steady_tick(Duration::from_millis(100));

        let mut last_percentage: u64 = 0;
        let mut current_status = String::new();

        client.pull_model_with_progress(model, |progress| {
            // Update status message if changed
            if progress.status != current_status {
                current_status = progress.status.clone();
                let status_msg = match current_status.as_str() {
                    s if s.contains("pulling manifest") => "Fetching manifest...",
                    s if s.contains("pulling") => "Downloading layers...",
                    s if s.contains("verifying") => "Verifying...",
                    s if s.contains("writing") => "Writing to disk...",
                    s if s.contains("success") => "Complete!",
                    _ => &current_status,
                };
                pb.set_message(status_msg.to_string());
            }

            // Update progress bar position
            if let Some(pct) = progress.percentage() {
                let pct_int = pct as u64;
                if pct_int != last_percentage {
                    last_percentage = pct_int;
                    pb.set_position(pct_int);

                    // Show size info if available
                    if let (Some(completed), Some(total)) = (progress.completed, progress.total) {
                        let completed_gb = completed as f64 / 1_073_741_824.0;
                        let total_gb = total as f64 / 1_073_741_824.0;
                        if total_gb >= 0.1 {
                            pb.set_message(format!("{:.1} GB / {:.1} GB", completed_gb, total_gb));
                        }
                    }
                }
            }
        })?;

        pb.finish_and_clear();
        println!("{} Model ready", "[✓]".bright_green());
    }

    Ok(())
}

fn direct_prompt(prompt: &str, model: Option<String>) -> Result<()> {
    let model = model.unwrap_or_else(get_model_from_config);
    let client = OllamaClient::new();

    ensure_model_available(&client, &model)?;

    let messages = vec![Message::user(prompt)];

    let start = Instant::now();
    let mut first_token_received = false;
    let mut time_to_first_token = 0u128;

    // Show thinking indicator with animated spinner
    let thinking_spinner = RefCell::new(Some(spinner::create("Thinking...")));

    // Set up Ctrl+C handling for cancellation
    let cancel_flag = Arc::new(AtomicBool::new(false));
    let cancel_flag_clone = cancel_flag.clone();
    let _ = ctrlc::set_handler(move || {
        cancel_flag_clone.store(true, Ordering::Relaxed);
    });

    // Use CodeBlockTracker for syntax highlighting of code blocks
    let code_block_tracker = RefCell::new(CodeBlockTracker::new());

    let response = client.chat_stream_cancellable(&model, messages, |chunk| {
        if !first_token_received {
            first_token_received = true;
            time_to_first_token = start.elapsed().as_millis();
            // Clear thinking spinner before showing first token
            if let Some(s) = thinking_spinner.borrow_mut().take() {
                spinner::clear(&s);
            }
        }
        if !chunk.done {
            // Use the code block tracker for syntax-highlighted output
            code_block_tracker.borrow_mut().process_token(&chunk.token);
            io::stdout().flush().ok();
        }
    }, Some(cancel_flag.clone()));

    // Flush any pending code block state
    code_block_tracker.borrow_mut().flush();

    // Handle cancellation or completion
    let (response, was_cancelled) = match response {
        Ok(r) => (r, false),
        Err(e) => {
            if e.to_string().contains("cancelled") {
                println!("\n{}", "[Interrupted]".yellow());
                // Return a partial response struct
                (rigrun::local::OllamaResponse {
                    response: String::new(),
                    prompt_tokens: 0,
                    completion_tokens: 0,
                    total_duration_ms: 0,
                }, true)
            } else {
                return Err(e);
            }
        }
    };

    println!(); // newline after response

    let elapsed = start.elapsed();
    let tokens_per_sec = if elapsed.as_secs_f64() > 0.0 && response.completion_tokens > 0 {
        response.completion_tokens as f64 / elapsed.as_secs_f64()
    } else {
        0.0
    };

    if !was_cancelled {
        println!(
            "\n{} {} tokens ({} prompt + {} completion) in {:.1}s ({:.1} tok/s) | TTFT: {}ms",
            "───".bright_black(),
            (response.prompt_tokens + response.completion_tokens).to_string().bright_black(),
            response.prompt_tokens.to_string().bright_black(),
            response.completion_tokens.to_string().bright_black(),
            elapsed.as_secs_f64(),
            tokens_per_sec,
            time_to_first_token.to_string().bright_green()
        );
    }

    Ok(())
}

pub fn interactive_chat(model: Option<String>) -> Result<()> {
    use rigrun::cli_session::{CliSession, CliSessionConfig, CliSessionState};
    use rigrun::status_indicator::{StatusIndicator, StatusConfig, StatusLineStyle, OperatingMode};
    use rigrun::detect::GpuType;
    use conversation_store::{ConversationStore, CommandResult, handle_slash_command};

    let mut model = model.unwrap_or_else(get_model_from_config);
    let client = OllamaClient::new();

    ensure_model_available(&client, &model)?;

    // Load config for status line settings
    let config = load_config().unwrap_or_default();

    // Initialize status indicator
    let status_config = StatusConfig {
        show_status_line: config.show_status_line,
        status_line_style: StatusLineStyle::from_str(&config.status_line_style)
            .unwrap_or(StatusLineStyle::Compact),
        show_session_time: true,
        show_vram_usage: false,
        show_token_count: false,
    };
    let mut status_indicator = StatusIndicator::new(status_config);
    status_indicator.set_model(&model);
    status_indicator.set_mode(OperatingMode::Local);
    status_indicator.refresh_gpu_status();
    status_indicator.set_session_start(Instant::now(), rigrun::CLI_SESSION_TIMEOUT_SECS);

    // Initialize conversation storage
    let store = ConversationStore::new()
        .context("Failed to initialize conversation storage")?;

    // Create CLI session with DoD STIG IL5 defaults (15-minute timeout)
    let session_config = CliSessionConfig::dod_stig_default();
    let mut session = CliSession::new("cli-user", session_config);
    session.acknowledge_consent(); // Consent was already shown at startup

    // Initialize interactive input with tab completion
    let mut input_handler = InteractiveInput::new()
        .context("Failed to initialize interactive input")?;

    // Set up models for completion
    let available_models = list_ollama_models();
    input_handler.set_models(available_models);
    input_handler.set_current_model(Some(model.clone()));

    // Set up saved conversation IDs for /resume completion
    if let Ok(conversations) = store.list() {
        let ids: Vec<String> = conversations.iter().map(|c| c.id.clone()).collect();
        input_handler.set_conversation_ids(ids);
    }

    // Show startup banner with optional full status line
    println!();
    if config.show_status_line && matches!(status_indicator.config.status_line_style, StatusLineStyle::Full) {
        status_indicator.render();
    }
    println!(
        "{} Interactive chat mode | Type 'exit' or Ctrl+C to quit",
        "rigrun".bright_cyan().bold()
    );
    println!(
        "{} Model: {} | Mode: {} | Session: {} min",
        "[i]".bright_blue(),
        model.bright_white(),
        "local".green(),
        rigrun::CLI_SESSION_TIMEOUT_SECS / 60
    );
    println!(
        "{} Type {} for commands (/status, /model), Tab for completion\n",
        "[i]".bright_blue(),
        "/help".bright_cyan()
    );

    let mut conversation: Vec<Message> = Vec::new();
    let mut auto_save = false;
    let mut current_conversation_id: Option<String> = None;
    let mut last_failed_message: Option<String> = None; // For /retry command
    // Keep a fallback stdin for session warning acknowledgment
    let stdin = io::stdin();

    loop {
        // Check session timeout before each prompt
        if session.check_timeout() {
            match session.state() {
                CliSessionState::Expired => {
                    let needs_reauth = session.show_expiration();
                    if needs_reauth {
                        // Require consent banner re-acknowledgment
                        println!("\n{} Re-authenticating session...\n", "[!]".yellow());

                        // Show consent banner again
                        if let Err(e) = consent_banner::handle_consent_banner(false, true) {
                            eprintln!("{}{RESET} Failed to re-authenticate: {}", RED, e);
                            break;
                        }

                        // Create new session
                        let session_config = CliSessionConfig::dod_stig_default();
                        session = CliSession::new("cli-user", session_config);
                        session.acknowledge_consent();

                        println!(
                            "\n{} Session renewed. Timeout: {} minutes\n",
                            "[+]".green(),
                            rigrun::CLI_SESSION_TIMEOUT_SECS / 60
                        );
                    }
                }
                CliSessionState::Warning => {
                    session.show_warning();
                    // Wait for user acknowledgment
                    print!("{} Press ENTER to continue... ", "[!]".yellow());
                    io::stdout().flush()?;
                    let mut ack = String::new();
                    stdin.read_line(&mut ack)?;
                    session.refresh();
                    println!("{} Session extended.\n", "[+]".green());
                    continue;
                }
                _ => {}
            }
        }

        // Build prompt with status indicator and session time
        let remaining = session.time_remaining_secs();
        let mins = remaining / 60;
        let secs = remaining % 60;

        // Build the prompt based on status line style
        let prompt = if config.show_status_line {
            match status_indicator.config.status_line_style {
                StatusLineStyle::Full => {
                    // Full style shows status line above prompt
                    status_indicator.render();
                    let time_indicator = if remaining <= 120 {
                        format!("\x1b[33m[{}:{:02}]\x1b[0m", mins, secs)
                    } else {
                        format!("\x1b[90m[{}:{:02}]\x1b[0m", mins, secs)
                    };
                    format!("\x1b[96m\x1b[1mYou:\x1b[0m {} ", time_indicator)
                }
                StatusLineStyle::Compact => {
                    // Compact style: [model | mode | GPU] [time] You:
                    let model_short = if model.len() > 15 {
                        format!("{}...", &model[..12])
                    } else {
                        model.clone()
                    };
                    let gpu_str = status_indicator.gpu_status()
                        .map(|g| {
                            if g.gpu_type == GpuType::Cpu {
                                "CPU"
                            } else if g.using_gpu {
                                "GPU\u{2713}"
                            } else {
                                "GPU\u{2717}"
                            }
                        })
                        .unwrap_or("?");
                    let time_indicator = if remaining <= 120 {
                        format!("\x1b[33m[{}:{:02}]\x1b[0m", mins, secs)
                    } else {
                        format!("\x1b[90m[{}:{:02}]\x1b[0m", mins, secs)
                    };
                    format!("\x1b[90m[{} | {} | {}]\x1b[0m {} \x1b[96m\x1b[1mYou:\x1b[0m ",
                        model_short, status_indicator.mode(), gpu_str, time_indicator)
                }
                StatusLineStyle::Minimal => {
                    // Minimal style: [mode|GPU] [time] You:
                    let gpu_str = status_indicator.gpu_status()
                        .map(|g| {
                            if g.gpu_type == GpuType::Cpu {
                                "CPU"
                            } else if g.using_gpu {
                                "GPU"
                            } else {
                                "cpu"
                            }
                        })
                        .unwrap_or("?");
                    let time_indicator = if remaining <= 120 {
                        format!("\x1b[33m[{}:{:02}]\x1b[0m", mins, secs)
                    } else {
                        format!("\x1b[90m[{}:{:02}]\x1b[0m", mins, secs)
                    };
                    format!("\x1b[90m[{}|{}]\x1b[0m {} \x1b[96m\x1b[1mYou:\x1b[0m ",
                        status_indicator.mode(), gpu_str, time_indicator)
                }
            }
        } else {
            // No status line - simple prompt with time
            let time_indicator = if remaining <= 120 {
                format!("\x1b[33m[{}:{:02}]\x1b[0m", mins, secs)
            } else {
                format!("\x1b[90m[{}:{:02}]\x1b[0m", mins, secs)
            };
            format!("\x1b[96m\x1b[1mrigrun>\x1b[0m {} ", time_indicator)
        };

        // Read user input with tab completion
        let input = match input_handler.read_line(&prompt) {
            Ok(Some(line)) => line,
            Ok(None) => {
                // EOF (Ctrl+D) - exit
                break;
            }
            Err(e) => {
                eprintln!("{} Input error: {}", "[!]".red(), e);
                continue;
            }
        };
        let mut owned_input = input.trim().to_string();

        // Refresh session on user activity
        session.refresh();

        // Check for empty input - provide subtle feedback
        if owned_input.is_empty() {
            println!("{}", "(empty - type a message or /help for commands)".bright_black());
            continue;
        }

        // Check for exit commands (without slash)
        if owned_input.eq_ignore_ascii_case("exit") || owned_input.eq_ignore_ascii_case("quit") {
            // Handle auto-save on exit
            if auto_save && !conversation.is_empty() {
                println!("{} Auto-saving conversation...", "[i]".bright_blue());
                if let Some(ref id) = current_conversation_id {
                    if let Err(e) = store.update(id, conversation.clone()) {
                        eprintln!("{} Failed to auto-save: {}", "[!]".yellow(), e);
                    } else {
                        println!("{} Conversation saved.", "[+]".green());
                    }
                } else {
                    let saved = conversation_store::SavedConversation::new(&model, conversation.clone());
                    if let Err(e) = store.save(&saved) {
                        eprintln!("{} Failed to auto-save: {}", "[!]".yellow(), e);
                    } else {
                        println!("{} Saved: \"{}\"", "[+]".green(), saved.summary);
                    }
                }
            }
            session.terminate("User requested exit");
            println!("Goodbye!");
            break;
        }

        // Handle slash commands
        if owned_input.starts_with('/') {
            // Special case: show help for just "/"
            if owned_input == "/" {
                show_command_menu();
                continue;
            }

            // Special case: /help command
            if owned_input == "/help" || owned_input == "/h" || owned_input == "/?" {
                show_help();
                continue;
            }

            // Special case: /model command to change model with completion
            if let Some(model_arg) = owned_input.strip_prefix("/model ") {
                let new_model = model_arg.trim();
                if !new_model.is_empty() {
                    // Check if model is available
                    if is_model_available(new_model) {
                        model = new_model.to_string();
                        input_handler.set_current_model(Some(model.clone()));
                        // Update status indicator with new model
                        status_indicator.set_model(&model);
                        status_indicator.refresh_gpu_status();
                        println!("{} Switched to model: {}", "[+]".green(), model.bright_white());
                    } else {
                        println!("{} Model '{}' not found.", "[!]".yellow(), new_model);
                        println!("    {}", "Tip: Use Tab to see available models, or pull with:".bright_black());
                        println!("    {}", format!("ollama pull {}", new_model).bright_cyan());
                    }
                    continue;
                }
            }

            // Special case: /mode command to change routing mode
            if let Some(mode_arg) = owned_input.strip_prefix("/mode ") {
                let new_mode = mode_arg.trim().to_lowercase();
                match OperatingMode::from_str(&new_mode) {
                    Some(mode) => {
                        status_indicator.set_mode(mode);
                        let mode_desc = match mode {
                            OperatingMode::Local => "Local only - all queries processed locally via Ollama",
                            OperatingMode::Cloud => "Cloud only - all queries routed to cloud providers",
                            OperatingMode::Auto => "Auto - local first, cloud fallback if needed",
                            OperatingMode::Hybrid => "Hybrid - intelligent routing based on query complexity",
                        };
                        println!("{} Mode: {} - {}", "[+]".green(), new_mode.bright_white(), mode_desc.bright_black());
                    }
                    None => {
                        println!("{} Invalid mode '{}'. Available: local, cloud, auto, hybrid", "[!]".yellow(), new_mode);
                    }
                }
                continue;
            }

            // Special case: /retry command to resend last failed message
            if owned_input == "/retry" || owned_input == "/r!" {
                if let Some(failed_msg) = last_failed_message.take() {
                    println!("{} Retrying last message...", "[i]".bright_blue());
                    // Set input to the failed message
                    owned_input = failed_msg;
                    // Fall through to process the message (not a slash command anymore)
                } else {
                    println!("{} No failed message to retry.", "[!]".yellow());
                    println!("    {}", "Tip: /retry resends your last message that failed to send.".bright_black());
                    continue;
                }
            }

            // Update status indicator stats before handling command
            status_indicator.update_stats(conversation.len() as u32, 0);

            let result = handle_slash_command(
                &owned_input,
                &conversation,
                &model,
                &store,
                &mut auto_save,
                &mut current_conversation_id,
                Some(&status_indicator),
            )?;

            match result {
                CommandResult::Continue => {
                    // Not a command, continue to send to model
                }
                CommandResult::Handled => {
                    // Command was handled, continue loop
                    continue;
                }
                CommandResult::Exit => {
                    // Handle auto-save on exit
                    if auto_save && !conversation.is_empty() {
                        println!("{} Auto-saving conversation...", "[i]".bright_blue());
                        if let Some(ref id) = current_conversation_id {
                            if let Err(e) = store.update(id, conversation.clone()) {
                                eprintln!("{} Failed to auto-save: {}", "[!]".yellow(), e);
                            } else {
                                println!("{} Conversation saved.", "[+]".green());
                            }
                        } else {
                            let saved = conversation_store::SavedConversation::new(&model, conversation.clone());
                            if let Err(e) = store.save(&saved) {
                                eprintln!("{} Failed to auto-save: {}", "[!]".yellow(), e);
                            } else {
                                println!("{} Saved: \"{}\"", "[+]".green(), saved.summary);
                            }
                        }
                    }
                    session.terminate("User requested exit via command");
                    println!("Goodbye!");
                    break;
                }
                CommandResult::Resume(conv) => {
                    // Resume the conversation
                    println!(
                        "\n{} Resuming \"{}\"...",
                        "[+]".green(),
                        conv.summary
                    );
                    println!(
                        "{} Loading {} messages from previous session.\n",
                        "[i]".bright_blue(),
                        conv.message_count
                    );

                    // Load conversation history
                    conversation = conv.messages.clone();
                    current_conversation_id = Some(conv.id.clone());

                    // Show last user message for context
                    if let Some(last_msg) = conv.last_user_message() {
                        let display_msg = if last_msg.len() > 80 {
                            format!("{}...", &last_msg[..77])
                        } else {
                            last_msg.to_string()
                        };
                        println!(
                            "{} Your last message was:\n  \"{}\"\n",
                            "[i]".bright_blue(),
                            display_msg.bright_white()
                        );
                    }

                    println!("{} Ready to continue.\n", "[+]".green());
                    continue;
                }
            }
        }

        // Handle /new and /clear - clear the conversation
        if owned_input.eq_ignore_ascii_case("/new") || owned_input.eq_ignore_ascii_case("/clear") {
            conversation.clear();
            current_conversation_id = None;
            println!("\n  Starting new conversation. Previous messages cleared.\n");
            continue;
        }

        // Process @ mentions for context inclusion
        let (expanded_input, context_messages) = rigrun::process_mentions(&owned_input);

        // Display what context was included
        for msg in &context_messages {
            println!("{}", msg.bright_blue());
        }
        if !context_messages.is_empty() {
            println!(); // Extra line after context messages
        }

        // Add user message to conversation (with expanded context)
        conversation.push(Message::user(&expanded_input));

        let start = Instant::now();
        let mut first_token_received = false;
        let mut time_to_first_token = 0u128;
        let mut accumulated_response = String::new();

        // Show thinking indicator with animated spinner
        let thinking_spinner = RefCell::new(Some(spinner::create("Thinking...")));

        // Set up Ctrl+C handling for cancellation
        let cancel_flag = Arc::new(AtomicBool::new(false));
        let cancel_flag_clone = cancel_flag.clone();
        let _ = ctrlc::set_handler(move || {
            cancel_flag_clone.store(true, Ordering::Relaxed);
        });

        // Get response with TRUE streaming and cancellation support
        // Use CodeBlockTracker for syntax highlighting of code blocks
        let code_block_tracker = RefCell::new(CodeBlockTracker::new());

        let response_result = client.chat_stream_cancellable(&model, conversation.clone(), |chunk| {
            if !first_token_received {
                first_token_received = true;
                time_to_first_token = start.elapsed().as_millis();
                // Clear thinking spinner before showing first token
                if let Some(s) = thinking_spinner.borrow_mut().take() {
                    spinner::clear(&s);
                }
            }
            accumulated_response.push_str(&chunk.token);
            // Use the code block tracker for syntax-highlighted output
            code_block_tracker.borrow_mut().process_token(&chunk.token);
            io::stdout().flush().ok();
        }, Some(cancel_flag.clone()));

        // Flush any pending code block state
        code_block_tracker.borrow_mut().flush();

        // Handle cancellation or completion
        let (response, was_cancelled) = match response_result {
            Ok(r) => (r, false),
            Err(e) => {
                if e.to_string().contains("cancelled") {
                    println!("\n{}", "[Interrupted]".yellow());
                    // Create partial response with accumulated text
                    (rigrun::local::OllamaResponse {
                        response: accumulated_response.clone(),
                        prompt_tokens: 0,
                        completion_tokens: 0,
                        total_duration_ms: 0,
                    }, true)
                } else {
                    // Store error for @error mention
                    rigrun::store_last_error(&format!("{}", e));
                    println!("\r{}\r", " ".repeat(12)); // Clear thinking indicator
                    eprintln!("{} {}", "[Error]".red(), e);
                    // Store the failed message for /retry
                    last_failed_message = Some(owned_input.clone());
                    println!("    {}", "Tip: Use /retry to resend this message.".bright_black());
                    // Remove the failed user message from conversation
                    conversation.pop();
                    continue;
                }
            }
        };

        println!(); // newline after response

        // Refresh session after receiving response (user is active)
        session.refresh();

        // Add assistant response to conversation (even partial if interrupted)
        if !response.response.is_empty() {
            conversation.push(Message::assistant(response.response.clone()));
            // Clear any failed message on success
            last_failed_message = None;
        } else if !accumulated_response.is_empty() {
            // Use accumulated response for interrupted stream
            conversation.push(Message::assistant(accumulated_response.clone()));
            last_failed_message = None;
        } else {
            // Empty response, remove the user message
            conversation.pop();
        }

        // Show stats
        let elapsed = start.elapsed();
        if !was_cancelled {
            let tokens_per_sec = if elapsed.as_secs_f64() > 0.0 && response.completion_tokens > 0 {
                response.completion_tokens as f64 / elapsed.as_secs_f64()
            } else {
                0.0
            };

            println!(
                "{} {:.1}s | {} tok/s | TTFT: {}ms\n",
                "───".bright_black(),
                elapsed.as_secs_f64(),
                format!("{:.1}", tokens_per_sec).bright_black(),
                time_to_first_token.to_string().bright_green()
            );
        } else {
            // Show partial stats for interrupted stream
            let word_count = accumulated_response.split_whitespace().count();
            println!(
                "{} {} | ~{} words | TTFT: {}ms\n",
                "─↴─".bright_black(),
                "partial".yellow(),
                word_count,
                if first_token_received { time_to_first_token.to_string() } else { "-".to_string() }
            );
        }
    }

    Ok(())
}

fn read_stdin() -> Result<String> {
    let stdin = io::stdin();
    let mut buffer = String::new();
    stdin.lock().read_to_string(&mut buffer)?;
    Ok(buffer)
}

/// Represents a detected IDE with its name and configuration path
#[derive(Debug, Clone)]
struct DetectedIDE {
    name: String,
    display_name: String,
    config_path: Option<PathBuf>,
    config_type: IDEConfigType,
}

#[derive(Debug, Clone)]
enum IDEConfigType {
    VSCode,
    Cursor,
    JetBrains(String), // Product name
    Neovim,
}

/// Check if a command is available in PATH
fn is_command_in_path(command: &str) -> bool {
    std::process::Command::new(command)
        .arg("--version")
        .output()
        .is_ok()
}

/// Check if any of the given paths exist and return the first one found
#[cfg(target_os = "windows")]
fn find_existing_path(paths: &[PathBuf]) -> Option<PathBuf> {
    paths.iter().find(|p| p.exists()).cloned()
}

/// Get Windows-specific IDE installation paths
#[cfg(target_os = "windows")]
fn get_windows_ide_paths() -> (Vec<PathBuf>, Vec<PathBuf>, Vec<PathBuf>) {
    let home = dirs::home_dir();
    let program_files = std::env::var("ProgramFiles").ok().map(PathBuf::from);
    let local_app_data = std::env::var("LOCALAPPDATA").ok().map(PathBuf::from);

    // VS Code installation paths
    let mut vscode_paths = Vec::new();
    if let Some(ref pf) = program_files {
        vscode_paths.push(pf.join("Microsoft VS Code").join("Code.exe"));
    }
    if let Some(ref local) = local_app_data {
        vscode_paths.push(local.join("Programs").join("Microsoft VS Code").join("Code.exe"));
    }
    if let Some(ref h) = home {
        vscode_paths.push(h.join("AppData").join("Local").join("Programs").join("Microsoft VS Code").join("Code.exe"));
    }

    // Cursor installation paths
    let mut cursor_paths = Vec::new();
    if let Some(ref local) = local_app_data {
        cursor_paths.push(local.join("Programs").join("Cursor").join("Cursor.exe"));
        cursor_paths.push(local.join("Programs").join("cursor").join("Cursor.exe"));
    }
    if let Some(ref h) = home {
        cursor_paths.push(h.join("AppData").join("Local").join("Programs").join("Cursor").join("Cursor.exe"));
        cursor_paths.push(h.join("AppData").join("Local").join("Programs").join("cursor").join("Cursor.exe"));
    }

    // Neovim installation paths
    let mut neovim_paths = Vec::new();
    if let Some(ref pf) = program_files {
        neovim_paths.push(pf.join("Neovim").join("bin").join("nvim.exe"));
    }
    if let Some(ref local) = local_app_data {
        neovim_paths.push(local.join("Programs").join("Neovim").join("bin").join("nvim.exe"));
    }
    // Also check scoop installation
    if let Some(ref h) = home {
        neovim_paths.push(h.join("scoop").join("apps").join("neovim").join("current").join("bin").join("nvim.exe"));
    }

    (vscode_paths, cursor_paths, neovim_paths)
}

/// Get JetBrains IDE installation paths on Windows
#[cfg(target_os = "windows")]
fn get_jetbrains_paths() -> Vec<PathBuf> {
    let mut paths = Vec::new();

    if let Some(pf) = std::env::var("ProgramFiles").ok().map(PathBuf::from) {
        paths.push(pf.join("JetBrains"));
    }

    if let Some(home) = dirs::home_dir() {
        // JetBrains Toolbox installations
        paths.push(home.join("AppData").join("Local").join("JetBrains").join("Toolbox").join("apps"));
    }

    paths
}

/// Detect installed IDEs on the system
fn detect_ides() -> Vec<DetectedIDE> {
    let mut ides = Vec::new();

    // Detect VS Code
    #[cfg(target_os = "windows")]
    {
        // First try PATH-based detection
        let vscode_found = if is_command_in_path("code") {
            true
        } else {
            // Fall back to checking common installation directories
            let (vscode_paths, _, _) = get_windows_ide_paths();
            find_existing_path(&vscode_paths).is_some()
        };

        if vscode_found {
            let appdata = std::env::var("APPDATA").ok();
            let config_path = appdata.map(|p| PathBuf::from(p).join("Code").join("User").join("settings.json"));
            ides.push(DetectedIDE {
                name: "vscode".to_string(),
                display_name: "VS Code".to_string(),
                config_path,
                config_type: IDEConfigType::VSCode,
            });
        }

        // Check for Cursor - first try PATH, then installation directories
        let cursor_found = if is_command_in_path("cursor") {
            true
        } else {
            let (_, cursor_paths, _) = get_windows_ide_paths();
            find_existing_path(&cursor_paths).is_some()
        };

        if cursor_found {
            let appdata = std::env::var("APPDATA").ok();
            let config_path = appdata.map(|p| PathBuf::from(p).join("Cursor").join("User").join("settings.json"));
            ides.push(DetectedIDE {
                name: "cursor".to_string(),
                display_name: "Cursor".to_string(),
                config_path,
                config_type: IDEConfigType::Cursor,
            });
        }

        // Check for JetBrains IDEs in common installation directories
        let jetbrains_search_paths = get_jetbrains_paths();
        let jetbrains_ides = vec![
            ("IntelliJ IDEA", "IntelliJ IDEA"),
            ("PyCharm", "PyCharm"),
            ("WebStorm", "WebStorm"),
            ("PhpStorm", "PhpStorm"),
            ("RustRover", "RustRover"),
            ("CLion", "CLion"),
            ("GoLand", "GoLand"),
            ("Rider", "Rider"),
            ("DataGrip", "DataGrip"),
        ];

        for search_path in &jetbrains_search_paths {
            if !search_path.exists() {
                continue;
            }

            for (product, display) in &jetbrains_ides {
                // Check if this IDE was already found
                let ide_name = product.to_lowercase().replace(" ", "_");
                if ides.iter().any(|i| i.name == ide_name) {
                    continue;
                }

                if let Ok(entries) = fs::read_dir(search_path) {
                    for entry in entries.flatten() {
                        let entry_name = entry.file_name().to_string_lossy().to_string();
                        if entry_name.contains(product) {
                            // Verify there's an executable in the bin folder
                            let bin_path = entry.path().join("bin");
                            let has_executable = if bin_path.exists() {
                                fs::read_dir(&bin_path)
                                    .map(|entries| {
                                        entries.flatten().any(|e| {
                                            let name = e.file_name().to_string_lossy().to_string();
                                            name.ends_with("64.exe") || name.ends_with(".exe")
                                        })
                                    })
                                    .unwrap_or(false)
                            } else {
                                // For Toolbox apps, the structure might be different
                                entry.path().is_dir()
                            };

                            if has_executable {
                                ides.push(DetectedIDE {
                                    name: ide_name.clone(),
                                    display_name: display.to_string(),
                                    config_path: None,
                                    config_type: IDEConfigType::JetBrains(product.to_string()),
                                });
                                break;
                            }
                        }
                    }
                }
            }
        }
    }

    #[cfg(not(target_os = "windows"))]
    {
        // Check for VS Code on Unix-like systems
        if is_command_in_path("code") {
            let home = dirs::home_dir();
            let config_path = home.map(|h| {
                if cfg!(target_os = "macos") {
                    h.join("Library").join("Application Support").join("Code").join("User").join("settings.json")
                } else {
                    h.join(".config").join("Code").join("User").join("settings.json")
                }
            });
            ides.push(DetectedIDE {
                name: "vscode".to_string(),
                display_name: "VS Code".to_string(),
                config_path,
                config_type: IDEConfigType::VSCode,
            });
        }

        // Check for Cursor
        if is_command_in_path("cursor") {
            let home = dirs::home_dir();
            let config_path = home.map(|h| {
                if cfg!(target_os = "macos") {
                    h.join("Library").join("Application Support").join("Cursor").join("User").join("settings.json")
                } else {
                    h.join(".config").join("Cursor").join("User").join("settings.json")
                }
            });
            ides.push(DetectedIDE {
                name: "cursor".to_string(),
                display_name: "Cursor".to_string(),
                config_path,
                config_type: IDEConfigType::Cursor,
            });
        }
    }

    // Check for Neovim (cross-platform)
    #[cfg(target_os = "windows")]
    let neovim_found = if is_command_in_path("nvim") {
        true
    } else {
        let (_, _, neovim_paths) = get_windows_ide_paths();
        find_existing_path(&neovim_paths).is_some()
    };

    #[cfg(not(target_os = "windows"))]
    let neovim_found = is_command_in_path("nvim");

    if neovim_found {
        let home = dirs::home_dir();
        let config_path = home.map(|h| {
            #[cfg(target_os = "windows")]
            {
                h.join("AppData").join("Local").join("nvim").join("init.lua")
            }
            #[cfg(not(target_os = "windows"))]
            {
                h.join(".config").join("nvim").join("init.lua")
            }
        });
        ides.push(DetectedIDE {
            name: "neovim".to_string(),
            display_name: "Neovim".to_string(),
            config_path,
            config_type: IDEConfigType::Neovim,
        });
    }

    ides
}

/// Generate configuration for an IDE using the local AI model
async fn generate_ide_config(ide: &DetectedIDE, port: u16) -> Result<String> {
    let config = load_config()?;
    let server_port = config.port.unwrap_or(port);

    // Create the prompt for the AI to generate the config
    let prompt = match &ide.config_type {
        IDEConfigType::VSCode | IDEConfigType::Cursor => {
            format!(
                "Generate a JSON configuration snippet for {} that configures it to use an OpenAI-compatible API endpoint. \
                The API endpoint is http://localhost:{}/v1. \
                Include settings for:\n\
                - The API base URL\n\
                - A default model name (use 'auto' to let rigrun choose)\n\
                - Any other relevant settings for AI code completion\n\n\
                Return ONLY the JSON object that should be added to settings.json, without any markdown formatting or explanation.",
                ide.display_name, server_port
            )
        }
        IDEConfigType::JetBrains(product) => {
            format!(
                "Generate configuration instructions for {} to use an OpenAI-compatible API endpoint at http://localhost:{}/v1. \
                Explain step-by-step how to:\n\
                1. Open the AI Assistant settings\n\
                2. Configure a custom OpenAI provider\n\
                3. Set the API endpoint URL\n\
                4. Set the model name (use 'auto')\n\n\
                Keep it concise and actionable.",
                product, server_port
            )
        }
        IDEConfigType::Neovim => {
            format!(
                "Generate a Lua configuration snippet for Neovim that configures an AI completion plugin (like copilot.lua or codecompanion.nvim) \
                to use an OpenAI-compatible API endpoint at http://localhost:{}/v1. \
                Use model name 'auto'. \
                Return ONLY the Lua code without markdown formatting or explanation.",
                server_port
            )
        }
    };

    // Make request to the local rigrun server
    let client = reqwest::Client::new();
    let request_body = serde_json::json!({
        "model": "auto",
        "messages": [
            {
                "role": "user",
                "content": prompt
            }
        ]
    });

    let response = client
        .post(format!("http://localhost:{}/v1/chat/completions", server_port))
        .json(&request_body)
        .send()
        .await
        .context("Failed to connect to rigrun server. Is it running?")?;

    if !response.status().is_success() {
        anyhow::bail!("Server returned error: {}", response.status());
    }

    let response_body: serde_json::Value = response.json().await?;

    let config_text = response_body["choices"][0]["message"]["content"]
        .as_str()
        .context("Invalid response format from server")?
        .to_string();

    Ok(config_text)
}

/// Handle IDE setup command
async fn handle_ide_setup() -> Result<()> {
    println!();
    println!("{BRIGHT_CYAN}{BOLD}=== IDE Setup ==={RESET}");
    println!();
    println!("{DIM}Detecting installed IDEs...{RESET}");
    println!();

    let ides = detect_ides();

    if ides.is_empty() {
        println!("{YELLOW}[!]{RESET} No supported IDEs detected.");
        println!();
        println!("Supported IDEs:");
        println!("  - VS Code (install from https://code.visualstudio.com)");
        println!("  - Cursor (install from https://cursor.sh)");
        println!("  - JetBrains IDEs (IntelliJ IDEA, PyCharm, WebStorm, etc.)");
        println!("  - Neovim (install from https://neovim.io)");
        println!();
        return Ok(());
    }

    println!("{GREEN}[✓]{RESET} Detected {} IDE(s):", ides.len());
    for ide in &ides {
        println!("  - {}", ide.display_name);
    }
    println!();

    // Check if server is running
    let config = load_config()?;
    let port = config.port.unwrap_or(DEFAULT_PORT);

    if !check_server_running(port) {
        println!("{YELLOW}[!]{RESET} rigrun server is not running on port {}", port);
        println!();
        println!("To use this feature, start the server first:");
        println!("  {CYAN}rigrun{RESET}");
        println!();
        println!("Or in another terminal, run:");
        println!("  {CYAN}rigrun{RESET}");
        println!();
        return Ok(());
    }

    println!("{GREEN}[✓]{RESET} rigrun server is running on port {}", port);
    println!();

    // Let user select which IDEs to configure
    let options: Vec<String> = ides.iter().map(|ide| ide.display_name.clone()).collect();

    let selected = inquire::MultiSelect::new("Select IDEs to configure:", options)
        .prompt();

    let selected = match selected {
        Ok(s) => s,
        Err(_) => {
            println!();
            println!("{YELLOW}[!]{RESET} Selection cancelled.");
            return Ok(());
        }
    };

    if selected.is_empty() {
        println!();
        println!("{YELLOW}[!]{RESET} No IDEs selected.");
        return Ok(());
    }

    println!();
    println!("{CYAN}[...]{RESET} Generating configurations with your local AI...");
    println!();

    // Generate configurations for selected IDEs
    for display_name in selected {
        let Some(ide) = ides.iter().find(|i| i.display_name == display_name) else {
            eprintln!("{RED}Error: IDE '{}' not found{RESET}", display_name);
            continue;
        };

        println!("{BRIGHT_CYAN}{BOLD}Configuration for {}:{RESET}", ide.display_name);
        println!();

        match generate_ide_config(ide, port).await {
            Ok(config_text) => {
                // Clean up markdown code blocks if present
                let cleaned_config = config_text
                    .trim()
                    .trim_start_matches("```json")
                    .trim_start_matches("```lua")
                    .trim_start_matches("```")
                    .trim_end_matches("```")
                    .trim();

                println!("{}", cleaned_config);
                println!();

                // Offer to write the config automatically
                if let Some(ref config_path) = ide.config_path {
                    if matches!(ide.config_type, IDEConfigType::VSCode | IDEConfigType::Cursor) {
                        println!("{CYAN}Would you like to add this to {} automatically? [y/n]{RESET}", config_path.display());
                        print!("{CYAN}>{RESET} ");
                        io::stdout().flush()?;

                        let stdin = io::stdin();
                        let mut input = String::new();
                        stdin.lock().read_line(&mut input)?;

                        if input.trim().eq_ignore_ascii_case("y") {
                            // Read existing settings or create new
                            let mut settings: serde_json::Value = if config_path.exists() {
                                let content = fs::read_to_string(config_path)?;
                                serde_json::from_str(&content).unwrap_or(serde_json::json!({}))
                            } else {
                                // Create parent directory if it doesn't exist
                                if let Some(parent) = config_path.parent() {
                                    fs::create_dir_all(parent)?;
                                }
                                serde_json::json!({})
                            };

                            // Parse the generated config
                            let new_config: serde_json::Value = serde_json::from_str(cleaned_config)
                                .context("Failed to parse generated configuration")?;

                            // Merge the configurations
                            if let Some(settings_obj) = settings.as_object_mut() {
                                if let Some(new_obj) = new_config.as_object() {
                                    for (key, value) in new_obj {
                                        settings_obj.insert(key.clone(), value.clone());
                                    }
                                }
                            }

                            // Write back to file
                            let content = serde_json::to_string_pretty(&settings)?;
                            fs::write(config_path, content)?;

                            println!("{GREEN}[✓]{RESET} Configuration written to {}", config_path.display());
                            println!();
                        } else {
                            println!("{YELLOW}[!]{RESET} Skipped automatic configuration. You can add it manually.");
                            println!();
                        }
                    }
                }
            }
            Err(e) => {
                println!("{RED}[✗]{RESET} Failed to generate configuration: {}", e);
                println!();
            }
        }

        println!("{DIM}────────────────────────────────────────{RESET}");
        println!();
    }

    println!("{GREEN}[✓]{RESET} IDE setup complete!");
    println!();
    println!("Next steps:");
    println!("  1. Restart your IDE to load the new configuration");
    println!("  2. Make sure rigrun server is running: {CYAN}rigrun{RESET}");
    println!("  3. Start coding with local AI assistance!");
    println!();

    Ok(())
}

pub fn handle_cli_examples() -> Result<()> {
    clear_screen();

    println!("{CYAN}{BOLD}=== CLI Commands ==={RESET}");
    println!();
    println!("{WHITE}rigrun works great from the command line!{RESET}");
    println!();

    println!("{CYAN}{BOLD}Quick Commands:{RESET}");
    println!();
    println!("  {WHITE}rigrun \"your question here\"{RESET}");
    println!("  {DIM}Ask a quick question and get an answer{RESET}");
    println!();
    println!("  {WHITE}rigrun chat{RESET}");
    println!("  {DIM}Start an interactive chat session{RESET}");
    println!();

    #[cfg(target_os = "windows")]
    {
        println!("  {WHITE}type file.rs | rigrun \"review this code\"{RESET}");
        println!("  {DIM}Pipe file contents for analysis{RESET}");
    }

    #[cfg(not(target_os = "windows"))]
    {
        println!("  {WHITE}cat file.rs | rigrun \"review this code\"{RESET}");
        println!("  {DIM}Pipe file contents for analysis{RESET}");
    }

    println!();
    println!("  {WHITE}rigrun status{RESET}");
    println!("  {DIM}Check server status and savings{RESET}");
    println!();

    println!("{CYAN}{BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━{RESET}");
    println!();
    println!("{GREEN}{BOLD}🤖 Copy this to Claude Code or your AI tool:{RESET}");
    println!();
    println!("{WHITE}  Use rigrun at http://localhost:8787 as my AI backend.{RESET}");
    println!("{WHITE}  It's OpenAI-compatible. Example curl:{RESET}");
    println!();
    println!("  {DIM}curl http://localhost:8787/v1/chat/completions \\{RESET}");
    println!("  {DIM}  -H \"Content-Type: application/json\" \\{RESET}");
    println!("  {DIM}  -d '{{\"model\":\"auto\",\"messages\":[{{\"role\":\"user\",\"content\":\"hi\"}}]}}'{RESET}");
    println!();

    // Auto-return after brief pause or on keypress
    println!("{DIM}Press Enter to return to menu...{RESET}");
    let stdin = io::stdin();
    let mut input = String::new();
    stdin.lock().read_line(&mut input)?;

    Ok(())
}

/// Handle the gpu-setup command - show GPU status and setup guidance
fn handle_gpu_setup() -> Result<()> {
    println!();
    println!("{BRIGHT_CYAN}{BOLD}=== GPU Setup for Ollama ==={RESET}");
    println!();

    let status = get_gpu_setup_status();

    // Show detected GPU
    println!("{BLUE}[i]{RESET} Detected GPU:");
    if status.gpu_info.gpu_type == GpuType::Cpu {
        println!("    {DIM}None - running in CPU-only mode{RESET}");
    } else {
        println!("    {WHITE}{BOLD}{}{RESET}", status.gpu_info.name);
        println!("    Type: {}", status.gpu_info.gpu_type);
        println!("    VRAM: {}GB", status.gpu_info.vram_gb);
        if let Some(ref driver) = status.gpu_info.driver {
            println!("    Driver: {}", driver);
        }
    }
    println!();

    // Show GPU-specific details
    match status.gpu_info.gpu_type {
        GpuType::Nvidia => {
            if let Some(ref driver) = status.nvidia_driver {
                println!("{BLUE}[i]{RESET} NVIDIA Driver: {}", driver);
                if rigrun::detect::is_nvidia_driver_recent(driver) {
                    println!("    {GREEN}[OK]{RESET} Driver version is recent enough for Ollama");
                } else {
                    println!("    {YELLOW}[!]{RESET} Driver may be outdated - consider updating to 470+");
                }
            }
            if rigrun::detect::check_nvidia_smi_available() {
                println!("    {GREEN}[OK]{RESET} nvidia-smi is available");
            } else {
                println!("    {RED}[X]{RESET} nvidia-smi is not available");
            }
        }
        GpuType::Amd => {
            if let Some(ref arch) = status.amd_architecture {
                println!("{BLUE}[i]{RESET} AMD Architecture: {}", arch);

                // Special note for RDNA 4
                if *arch == AmdArchitecture::Rdna4 {
                    println!("    {GREEN}[i]{RESET} RDNA 4 works with Vulkan backend (set OLLAMA_VULKAN=1)");
                }
            }

            if status.rocm_installed {
                println!("    {GREEN}[OK]{RESET} ROCm/HIP is installed");
            } else {
                println!("    {YELLOW}[!]{RESET} ROCm/HIP not detected");
            }

            // Show HSA override hint if applicable
            if let Some(hsa_version) = rigrun::detect::get_hsa_override_version(&status.gpu_info.name) {
                println!("    {BLUE}[i]{RESET} Suggested HSA_OVERRIDE_GFX_VERSION: {}", hsa_version);
            }
        }
        _ => {}
    }
    println!();

    // Show VRAM usage if available
    if let Some(ref usage) = status.vram_usage {
        let usage_pct = usage.usage_percent();
        let usage_color = if usage_pct > 90.0 {
            RED
        } else if usage_pct > 70.0 {
            YELLOW
        } else {
            GREEN
        };
        println!("{BLUE}[i]{RESET} VRAM Usage:");
        println!(
            "    {usage_color}{}MB{RESET} / {}MB ({usage_color}{:.1}%{RESET})",
            usage.used_mb, usage.total_mb, usage_pct
        );
        if let Some(util) = usage.gpu_utilization {
            println!("    GPU Utilization: {}%", util);
        }
        println!();
    }

    // Show GPU acceleration status
    println!("{BRIGHT_CYAN}{BOLD}=== GPU Acceleration Status ==={RESET}");
    println!();

    if status.gpu_working {
        println!("{GREEN}[OK]{RESET} GPU acceleration appears to be working!");
    } else if status.gpu_info.gpu_type == GpuType::Cpu {
        println!("{YELLOW}[!]{RESET} No GPU detected - Ollama will use CPU only");
        println!("    {DIM}This will be slower than GPU inference{RESET}");
    } else {
        println!("{YELLOW}[!]{RESET} GPU detected but acceleration may not be working");
    }
    println!();

    // Show loaded models and their GPU status
    if !status.loaded_models.is_empty() {
        println!("{BRIGHT_CYAN}{BOLD}=== Loaded Models ==={RESET}");
        println!();
        for model in &status.loaded_models {
            let processor_color = match &model.processor {
                ProcessorType::Gpu(_) => GREEN,
                ProcessorType::Mixed { gpu_percent, .. } if *gpu_percent > 50 => YELLOW,
                ProcessorType::Cpu => RED,
                _ => YELLOW,
            };
            println!(
                "  {WHITE}{BOLD}{}{RESET} ({}) - {processor_color}{}{RESET}",
                model.name, model.size, model.processor
            );
        }
        println!();
    }

    // Show guidance if there are issues
    if let Some(ref guidance) = status.guidance {
        println!("{BRIGHT_CYAN}{BOLD}=== Setup Guidance ==={RESET}");
        println!();
        println!("{YELLOW}Issue:{RESET} {}", guidance.issue);
        println!();
        println!("{GREEN}Solution:{RESET} {}", guidance.solution);
        println!();

        if !guidance.commands.is_empty() {
            println!("{CYAN}Commands to run:{RESET}");
            for cmd in &guidance.commands {
                if cmd.starts_with('#') {
                    println!("  {DIM}{}{RESET}", cmd);
                } else {
                    println!("  {WHITE}{}{RESET}", cmd);
                }
            }
            println!();
        }

        if !guidance.links.is_empty() {
            println!("{CYAN}Helpful links:{RESET}");
            for link in &guidance.links {
                println!("  {BLUE}{}{RESET}", link);
            }
            println!();
        }
    } else {
        println!("{GREEN}[OK]{RESET} No setup issues detected!");
        println!();
    }

    // Show recommended model for this GPU
    if status.gpu_info.vram_gb > 0 {
        let recommended = recommend_model(status.gpu_info.vram_gb);
        println!("{BLUE}[i]{RESET} Recommended model for {}GB VRAM: {CYAN}{}{RESET}", status.gpu_info.vram_gb, recommended);
        println!("    To use: {CYAN}rigrun config --model {}{RESET}", recommended);
        println!();
    }

    Ok(())
}

/// Handle the ask command - send a single question to rigrun
/// Supports file input: `rigrun ask "Review this:" --file code.rs`
/// Supports @ mentions: `rigrun ask "@file:src/main.rs explain this code"`
async fn handle_ask(question: Option<&str>, model: Option<&str>, file: Option<&str>) -> Result<()> {
    use std::fs;
    use colored::Colorize;

    // Build the initial question from arg + file content
    let initial_question = {
        let file_content = if let Some(path) = file {
            match fs::read_to_string(path) {
                Ok(content) => Some(content),
                Err(e) => {
                    eprintln!("Error reading file '{}': {}", path, e);
                    return Ok(());
                }
            }
        } else {
            None
        };

        match (question, file_content) {
            (Some(q), Some(content)) => format!("{}\n\n{}", q, content),  // Question + file
            (Some(q), None) => q.to_string(),  // Just the question
            (None, Some(content)) => format!("Please analyze this:\n\n{}", content),  // Just file
            (None, None) => {
                eprintln!("Error: No question provided. Use: rigrun ask \"your question\" or --file <path>");
                return Ok(());
            }
        }
    };

    // Process @ mentions for context inclusion
    let (final_question, context_messages) = rigrun::process_mentions(&initial_question);

    // Display what context was included
    for msg in &context_messages {
        eprintln!("{}", msg.bright_blue());
    }
    if !context_messages.is_empty() {
        eprintln!(); // Extra line after context messages
    }

    let model = model.unwrap_or("local");
    let url = "http://localhost:8787/v1/chat/completions";

    let client = reqwest::Client::new();
    let body = serde_json::json!({
        "model": model,
        "messages": [{"role": "user", "content": final_question}]
    });

    let response = client.post(url)
        .json(&body)
        .timeout(std::time::Duration::from_secs(300))
        .send()
        .await;

    match response {
        Ok(res) => {
            if res.status().is_success() {
                let json: serde_json::Value = res.json().await?;
                if let Some(content) = json["choices"][0]["message"]["content"].as_str() {
                    println!("{}", content);
                } else {
                    eprintln!("Error: No response content");
                }
            } else {
                eprintln!("Error: {} - {}", res.status(), res.text().await.unwrap_or_default());
            }
        }
        Err(e) => {
            eprintln!("Error connecting to rigrun: {}", e);
            eprintln!("Make sure rigrun server is running (rigrun &)");
        }
    }

    Ok(())
}

/// Handle cache commands
fn handle_cache(command: CacheCommands) -> Result<()> {
    match command {
        CacheCommands::Stats => {
            println!();
            println!("{BRIGHT_CYAN}{BOLD}=== Cache Statistics ==={RESET}");
            println!();

            let cache_dir = dirs::data_dir()
                .unwrap_or_else(|| PathBuf::from("."))
                .join("rigrun")
                .join("cache");
            let cache_file = cache_dir.join("query_cache.json");

            if cache_file.exists() {
                let cache_content = fs::read_to_string(&cache_file)?;
                let entry_count = cache_content.matches("query_hash").count();
                let file_size = cache_content.len();

                println!("  Cache entries:  {WHITE}{BOLD}{}{RESET}", entry_count);
                println!("  Cache size:     {WHITE}{}{RESET} bytes ({:.2} KB)", file_size, file_size as f64 / 1024.0);
                println!("  Cache location: {DIM}{}{RESET}", cache_file.display());
            } else {
                println!("  {DIM}No cache data found{RESET}");
            }

            println!();
        }
        CacheCommands::Clear => {
            println!();
            println!("{BRIGHT_CYAN}{BOLD}=== Clear Cache ==={RESET}");
            println!();

            let cache_dir = dirs::data_dir()
                .unwrap_or_else(|| PathBuf::from("."))
                .join("rigrun")
                .join("cache");
            let cache_file = cache_dir.join("query_cache.json");

            if cache_file.exists() {
                fs::remove_file(&cache_file)?;
                println!("{GREEN}[✓]{RESET} Cache cleared successfully");
            } else {
                println!("{YELLOW}[!]{RESET} No cache to clear");
            }

            println!();
        }
        CacheCommands::Export { output } => {
            handle_export(output)?;
        }
    }

    Ok(())
}

/// Handle the export command - export cached data and audit log
fn handle_export(output_dir: Option<PathBuf>) -> Result<()> {
    println!();
    println!("{BRIGHT_CYAN}{BOLD}=== RigRun Data Export ==={RESET}");
    println!();

    let output_dir = output_dir.unwrap_or_else(|| PathBuf::from("."));

    // Ensure output directory exists
    if !output_dir.exists() {
        fs::create_dir_all(&output_dir)?;
    }

    let timestamp = chrono::Utc::now().format("%Y%m%d_%H%M%S");
    let mut files_exported = Vec::new();
    let mut total_size: u64 = 0;

    // 1. Export cache data
    println!("{CYAN}[1/3]{RESET} Exporting cache data...");
    let cache_dir = dirs::data_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join("rigrun")
        .join("cache");
    let cache_file = cache_dir.join("query_cache.json");

    if cache_file.exists() {
        let cache_content = fs::read_to_string(&cache_file)?;
        let export_cache_path = output_dir.join(format!("rigrun_cache_{}.json", timestamp));
        fs::write(&export_cache_path, &cache_content)?;
        let size = cache_content.len() as u64;
        total_size += size;
        files_exported.push((export_cache_path.display().to_string(), size));
        println!("  {GREEN}[OK]{RESET} Cache exported ({} entries)",
            cache_content.matches("query_hash").count());
    } else {
        println!("  {DIM}No cache data found{RESET}");
    }

    // 2. Export audit log
    println!("{CYAN}[2/3]{RESET} Exporting audit log...");
    let audit_log_path = dirs::home_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join(".rigrun")
        .join("audit.log");

    if audit_log_path.exists() {
        let audit_content = fs::read_to_string(&audit_log_path)?;
        let export_audit_path = output_dir.join(format!("rigrun_audit_{}.log", timestamp));
        fs::write(&export_audit_path, &audit_content)?;
        let size = audit_content.len() as u64;
        total_size += size;
        let line_count = audit_content.lines().count();
        files_exported.push((export_audit_path.display().to_string(), size));
        println!("  {GREEN}[OK]{RESET} Audit log exported ({} entries)", line_count);
    } else {
        println!("  {DIM}No audit log found{RESET}");
    }

    // 3. Export stats
    println!("{CYAN}[3/3]{RESET} Exporting statistics...");
    let stats_path = dirs::home_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join(".rigrun")
        .join("stats.json");

    if stats_path.exists() {
        let stats_content = fs::read_to_string(&stats_path)?;
        let export_stats_path = output_dir.join(format!("rigrun_stats_{}.json", timestamp));
        fs::write(&export_stats_path, &stats_content)?;
        let size = stats_content.len() as u64;
        total_size += size;
        files_exported.push((export_stats_path.display().to_string(), size));
        println!("  {GREEN}[OK]{RESET} Statistics exported");
    } else {
        println!("  {DIM}No statistics found{RESET}");
    }

    // Summary
    println!();
    println!("{BRIGHT_CYAN}{BOLD}=== Export Summary ==={RESET}");
    println!();

    if files_exported.is_empty() {
        println!("{YELLOW}[!]{RESET} No data to export. Use rigrun to generate some data first.");
    } else {
        println!("  Files exported:");
        for (path, size) in &files_exported {
            println!("    {GREEN}[OK]{RESET} {} ({} bytes)", path, size);
        }
        println!();
        println!("  Total size: {} bytes", total_size);
        println!();
        println!("{GREEN}[OK]{RESET} Export complete!");
        println!();
        println!("{DIM}Your data is yours. These files can be used for:");
        println!("  - Backup and restoration");
        println!("  - Privacy auditing");
        println!("  - Migration to another machine{RESET}");
    }

    println!();

    Ok(())
}

/// Print paranoid mode warning banner
fn print_paranoid_banner() {
    println!();
    println!("{RED}{BOLD}============================================{RESET}");
    println!("{RED}{BOLD}          PARANOID MODE ENABLED            {RESET}");
    println!("{RED}{BOLD}============================================{RESET}");
    println!("{YELLOW}[!]{RESET} All cloud requests are BLOCKED");
    println!("{YELLOW}[!]{RESET} Only local inference and cache will be used");
    println!("{YELLOW}[!]{RESET} Your data NEVER leaves your machine");
    println!("{RED}{BOLD}============================================{RESET}");
    println!();
}

/// Run comprehensive diagnostic checks on the system.
async fn run_doctor(auto_fix: bool, check_network: bool) -> anyhow::Result<()> {
    use colored::Colorize;
    use rigrun::health::{HealthChecker, Severity, auto_fix_issues};

    println!();
    println!("{}", "Running diagnostics...".bold());
    println!();

    // Build health checker with options
    let mut checker = HealthChecker::new();
    if check_network {
        checker = checker.with_network_check(true);
    }

    // Run the comprehensive health check
    let status = checker.run_full_check();

    // Display Ollama status
    if status.ollama_running {
        let version = status.ollama_version.as_deref().unwrap_or("unknown");
        println!("{} Ollama: Running (v{})", "[OK]".green(), version);
    } else {
        println!("{} Ollama: Not running", "[X]".red());
        println!("   {} Start Ollama: ollama serve", "Fix:".yellow());
    }

    // Display Model status
    if let Some(ref model) = status.model_name {
        if status.model_loaded {
            println!("{} Model: {} loaded", "[OK]".green(), model);
        } else if status.model_downloaded {
            println!("{} Model: {} available (not loaded)", "[OK]".green(), model);
        } else {
            println!("{} Model: {} not downloaded", "[X]".red(), model);
            println!("   {} ollama pull {}", "Fix:".yellow(), model);
        }
    } else {
        let available_models = rigrun::detect::list_ollama_models();
        if available_models.is_empty() {
            println!("{} Model: No models downloaded", "[!]".yellow());
            println!("   {} ollama pull qwen2.5-coder:7b", "Fix:".yellow());
        } else {
            println!("{} Model: {} models available", "[OK]".green(), available_models.len());
        }
    }

    // Display GPU status
    if let Some(ref gpu) = status.gpu_info {
        let vram_info = format!("{}GB VRAM", gpu.vram_gb);
        if status.gpu_in_use {
            println!("{} GPU: {} ({}, {})", "[OK]".green(), gpu.name, gpu.gpu_type, vram_info);
        } else if status.gpu_detected {
            println!("{} GPU: {} (detected but not in use)", "[!]".yellow(), gpu.name);
        } else {
            println!("{} GPU: {} ({})", "[OK]".green(), gpu.name, vram_info);
        }
    } else if !status.gpu_detected {
        println!("{} GPU: None detected (CPU mode)", "[!]".yellow());
        println!("   {} Install GPU drivers or run 'rigrun setup gpu'", "Fix:".yellow());
    }

    // Display VRAM usage if available
    if let (Some(used), Some(percent)) = (status.vram_used_mb, status.vram_usage_percent) {
        let total = status.vram_used_mb.unwrap_or(0) + status.vram_available_mb.unwrap_or(0);
        if percent >= 95.0 {
            println!("{} VRAM: {:.1}% used ({}/{}MB) - critically low!", "[X]".red(), percent, used, total);
            println!("   {} Consider using a smaller model", "Fix:".yellow());
        } else if percent >= 85.0 {
            println!("{} VRAM: {:.1}% used ({}/{}MB)", "[!]".yellow(), percent, used, total);
            println!("   {} Consider using a smaller model if experiencing slowdowns", "Tip:".cyan());
        } else {
            println!("{} VRAM: {:.1}% used ({}/{}MB)", "[OK]".green(), percent, used, total);
        }
    }

    // Display Config status
    if status.config_valid {
        println!("{} Config: Valid", "[OK]".green());
    } else {
        println!("{} Config: Invalid", "[X]".red());
        for issue in &status.config_issues {
            println!("   {} {}", "Issue:".yellow(), issue);
        }
    }

    // Display Disk space status
    if let Some(space) = status.disk_space_gb {
        if space >= 10 {
            println!("{} Disk: {}GB available", "[OK]".green(), space);
        } else {
            println!("{} Disk: {}GB available (low)", "[!]".yellow(), space);
            println!("   {} Free up disk space for model storage", "Fix:".yellow());
        }
    }

    // Display Network status (if checked)
    if let Some(available) = status.network_available {
        if available {
            println!("{} Network: Cloud APIs reachable", "[OK]".green());
        } else {
            println!("{} Network: Cloud APIs unreachable", "[!]".yellow());
            println!("   {} Check internet connection for cloud fallback", "Note:".cyan());
        }
    }

    // Display port availability
    print!("Checking port 8787... ");
    match check_port_available(8787) {
        Ok(_) => println!("{}", "[OK]".green()),
        Err(e) => {
            println!("{} {}", "[!]".yellow(), e);
        }
    }

    // Summary
    println!();
    let (critical, warnings, _info) = status.issue_counts();

    if critical == 0 && warnings == 0 {
        println!("{}", "All checks passed! rigrun is ready.".green().bold());
    } else {
        // Show detailed issues
        if critical > 0 || warnings > 0 {
            println!("{}", "Issues found:".yellow().bold());
            println!();

            for issue in &status.issues {
                let icon = match issue.severity {
                    Severity::Critical => "[X]".red(),
                    Severity::Warning => "[!]".yellow(),
                    Severity::Info => "[i]".cyan(),
                };
                println!("{} {}: {}", icon, issue.component.bold(), issue.message);
                println!("   {} {}", "Fix:".yellow(), issue.fix);
                println!();
            }
        }

        println!(
            "Overall: {} critical, {} warning{}",
            if critical > 0 { format!("{}", critical).red().bold() } else { format!("{}", critical).green() },
            if warnings > 0 { format!("{}", warnings).yellow().bold() } else { format!("{}", warnings).green() },
            if warnings == 1 { "" } else { "s" }
        );

        // Auto-fix if requested
        if auto_fix {
            let fixable: Vec<_> = status.fixable_issues();
            if fixable.is_empty() {
                println!();
                println!("{}", "No auto-fixable issues found.".dimmed());
            } else {
                println!();
                println!("{}", "Attempting to auto-fix issues...".bold());
                println!();

                let results = auto_fix_issues(&fixable);
                for (cmd, result) in results {
                    match result {
                        Ok(_) => println!("{} Fixed: {}", "[OK]".green(), cmd),
                        Err(e) => println!("{} Failed: {} - {}", "[X]".red(), cmd, e),
                    }
                }
            }
        } else if !status.fixable_issues().is_empty() {
            println!();
            println!("Run '{}' to auto-fix {} issue(s) where possible.",
                "rigrun doctor --fix".cyan(),
                status.fixable_issues().len()
            );
        }
    }

    println!();

    Ok(())
}

fn check_ollama_installed() -> anyhow::Result<String> {
    let output = std::process::Command::new("ollama")
        .arg("--version")
        .output()
        .map_err(|_| anyhow::anyhow!("Ollama not found. Install from https://ollama.ai"))?;

    let version = String::from_utf8_lossy(&output.stdout);
    Ok(version.trim().to_string())
}

async fn check_ollama_running() -> anyhow::Result<()> {
    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(2))
        .build()?;

    client.get("http://localhost:11434/api/tags")
        .send()
        .await
        .map_err(|_| anyhow::anyhow!("Ollama not running. Start with: ollama serve"))?;

    Ok(())
}

fn check_config() -> anyhow::Result<()> {
    let config_dir = dirs::config_dir()
        .ok_or_else(|| anyhow::anyhow!("Cannot find config directory"))?
        .join("rigrun");

    if !config_dir.exists() {
        return Err(anyhow::anyhow!("Config not initialized. Run: rigrun"));
    }
    Ok(())
}

fn check_port_available(port: u16) -> anyhow::Result<()> {
    use std::net::TcpListener;
    TcpListener::bind(format!("127.0.0.1:{}", port))
        .map_err(|_| anyhow::anyhow!("Port {} in use", port))?;
    Ok(())
}


/// Async operations that require the Tokio runtime.
/// This is separated from main() to avoid runtime conflicts with blocking code.
async fn run_async_command(command: AsyncCommand, config: &mut Config, paranoid_mode: bool) -> Result<()> {
    match command {
        AsyncCommand::StartServer { no_wizard, quick_setup } => {
            // Use the new wizard module for first run detection
            use rigrun::firstrun::{is_first_run, run_wizard, run_quick_wizard};

            // Check if this is first run (and wizard not skipped)
            let should_run_wizard = is_first_run() && !config.first_run_complete && !no_wizard;

            if should_run_wizard {
                // FIRST RUN: Show new wizard
                if quick_setup {
                    // Quick setup mode - minimal prompts
                    if let Err(e) = run_quick_wizard().await {
                        eprintln!("{YELLOW}[!]{RESET} Quick setup error: {}", e);
                        eprintln!("{YELLOW}[!]{RESET} You can run 'rigrun setup' manually to configure.");
                    }
                } else {
                    // Full interactive wizard
                    if let Err(e) = run_wizard().await {
                        eprintln!("{YELLOW}[!]{RESET} Wizard error: {}", e);
                        eprintln!("{YELLOW}[!]{RESET} You can run 'rigrun setup' manually to configure.");
                    }
                }

                // Reload config after wizard completes
                if let Ok(new_config) = load_config() {
                    *config = new_config;
                }

                // After wizard completes, clear screen and start clean server
                clear_screen();
                print_banner();
                if paranoid_mode {
                    print_paranoid_banner();
                }
                start_server(config).await?;
            } else if no_wizard && is_first_run() {
                // First run but wizard skipped - use defaults
                println!("{YELLOW}[!]{RESET} First run wizard skipped (--no-wizard flag)");
                println!("{YELLOW}[!]{RESET} Using default configuration. Run {CYAN}rigrun setup{RESET} later to configure.");
                println!();

                // Mark first run complete with defaults
                config.first_run_complete = true;
                if let Err(e) = save_config(config) {
                    eprintln!("{YELLOW}[!]{RESET} Failed to save config: {}", e);
                }

                clear_screen();
                print_banner();
                if paranoid_mode {
                    print_paranoid_banner();
                }
                start_server(config).await?;
            } else {
                // SUBSEQUENT RUNS: Skip wizard, go straight to clean server
                clear_screen();
                print_banner();
                if paranoid_mode {
                    print_paranoid_banner();
                }
                start_server(config).await?;
            }
        }
        AsyncCommand::Ask { question, model, file } => {
            if paranoid_mode {
                print_paranoid_banner();
            }
            handle_ask(question.as_deref(), model.as_deref(), file.as_deref()).await?;
        }
        AsyncCommand::IdeSetup => {
            handle_ide_setup().await?;
        }
        AsyncCommand::Doctor { fix, check_network } => {
            run_doctor(fix, check_network).await?;
        }
        AsyncCommand::Pull { model } => {
            pull_model(model).await?;
        }
        AsyncCommand::UnifiedSetup { quick, full, hardware } => {
            // Run unified setup wizard
            match rigrun::run_setup(quick, full, hardware) {
                Ok(result) => {
                    if !result.success {
                        std::process::exit(CONFIG);
                    }
                }
                Err(e) => {
                    eprintln!("{RED}[!]{RESET} Setup failed: {}", e);
                    std::process::exit(CONFIG);
                }
            }
        }
        AsyncCommand::RunWizard { quick } => {
            // Explicitly run the new wizard
            use rigrun::firstrun::{run_wizard, run_quick_wizard};

            if quick {
                if let Err(e) = run_quick_wizard().await {
                    eprintln!("{RED}[!]{RESET} Quick wizard failed: {}", e);
                    std::process::exit(CONFIG);
                }
            } else {
                if let Err(e) = run_wizard().await {
                    eprintln!("{RED}[!]{RESET} Wizard failed: {}", e);
                    std::process::exit(CONFIG);
                }
            }
        }
    }
    Ok(())
}

/// Commands that require async runtime
enum AsyncCommand {
    StartServer { no_wizard: bool, quick_setup: bool },
    Ask { question: Option<String>, model: Option<String>, file: Option<String> },
    IdeSetup,
    Doctor { fix: bool, check_network: bool },
    Pull { model: String },
    UnifiedSetup { quick: bool, full: bool, hardware: Option<String> },
    RunWizard { quick: bool },
}

fn main() -> Result<()> {
    let cli = Cli::parse();

    // Check if paranoid mode is enabled via CLI or config
    let mut config = load_config()?;
    let paranoid_mode = cli.paranoid || config.paranoid_mode;

    // Update config with CLI override for paranoid mode
    // This ensures the server gets the correct setting
    if cli.paranoid {
        config.paranoid_mode = true;
    }

    // Initialize audit logging based on config
    if let Err(e) = rigrun::init_audit_logger(config.audit_log_enabled) {
        eprintln!("{YELLOW}[!]{RESET} Failed to initialize audit logging: {}", e);
    }

    // DoD Consent Banner - IL5 REQUIREMENT
    // Must be displayed and acknowledged before system use (unless explicitly skipped)
    if let Err(e) = consent_banner::handle_consent_banner(cli.skip_banner, config.dod_banner_enabled) {
        eprintln!("{RED}[!]{RESET} Failed to handle DoD consent banner: {}", e);
        eprintln!("{RED}[!]{RESET} Cannot proceed without consent acknowledgment.");
        std::process::exit(ERROR);
    }

    // Check if we have a prompt argument (either direct prompt or stdin)
    let stdin_is_piped = !io::stdin().is_terminal();

    // Handle direct prompts (these use blocking client, no async needed)
    if let Some(prompt_text) = cli.prompt {
        if paranoid_mode {
            print_paranoid_banner();
        }
        return direct_prompt(&prompt_text, None);
    } else if stdin_is_piped && cli.command.is_none() {
        let input = read_stdin()?;
        if !input.trim().is_empty() {
            if paranoid_mode {
                print_paranoid_banner();
            }
            return direct_prompt(&input, None);
        }
    }

    // Determine if we need async runtime or can run synchronously
    let async_command = match &cli.command {
        None => Some(AsyncCommand::StartServer {
            no_wizard: cli.no_wizard,
            quick_setup: cli.quick_setup,
        }),
        Some(Commands::Ask { question, model, file }) => Some(AsyncCommand::Ask {
            question: question.clone(),
            model: model.clone(),
            file: file.clone(),
        }),
        Some(Commands::Setup { quick, full, hardware, command }) => match command {
            // Unified setup wizard (default when no subcommand specified)
            None => Some(AsyncCommand::UnifiedSetup {
                quick: *quick,
                full: *full,
                hardware: hardware.clone(),
            }),
            Some(SetupCommands::Ide) => Some(AsyncCommand::IdeSetup),
            Some(SetupCommands::Gpu) => None, // Sync
            Some(SetupCommands::Wizard { quick, full, hardware }) => Some(AsyncCommand::UnifiedSetup {
                quick: *quick,
                full: *full,
                hardware: hardware.clone(),
            }),
        },
        Some(Commands::Doctor { fix, check_network }) => Some(AsyncCommand::Doctor { fix: *fix, check_network: *check_network }),
        Some(Commands::Pull { model }) => Some(AsyncCommand::Pull { model: model.clone() }),
        Some(Commands::IdeSetup) => Some(AsyncCommand::IdeSetup), // Legacy
        _ => None, // All other commands are synchronous
    };

    // If we need async, create runtime and run
    if let Some(cmd) = async_command {
        let runtime = tokio::runtime::Runtime::new()
            .expect("Failed to create Tokio runtime");
        return runtime.block_on(run_async_command(cmd, &mut config, paranoid_mode));
    }

    // Otherwise, run synchronous commands directly (no runtime conflict)
    match cli.command {
        Some(Commands::Chat { model }) => {
            if paranoid_mode {
                print_paranoid_banner();
            }
            interactive_chat(model)?;
        }
        Some(Commands::Status) => {
            if paranoid_mode {
                print_paranoid_banner();
            }
            show_status()?;
        }
        Some(Commands::Config { command }) => {
            handle_config(command)?;
        }
        Some(Commands::Setup { quick: _, full: _, hardware: _, command }) => {
            match command {
                Some(SetupCommands::Gpu) => {
                    handle_gpu_setup()?;
                }
                _ => unreachable!(), // UnifiedSetup, Ide, and Wizard handled in async
            }
        }
        Some(Commands::Cache { command }) => {
            // Default to Stats if no subcommand provided
            handle_cache(command.unwrap_or(CacheCommands::Stats))?;
        }
        Some(Commands::Models) => {
            list_models()?;
        }
        Some(Commands::Examples) => {
            handle_cli_examples()?;
        }
        Some(Commands::Background) => {
            background::handle_background_server()?;
        }
        Some(Commands::Stop) => {
            background::handle_stop_server()?;
        }
        Some(Commands::GpuSetup) => {
            handle_gpu_setup()?;
        }
        Some(Commands::Export { output }) => {
            handle_export(output)?;
        }
        _ => unreachable!(), // Async commands handled above
    }

    Ok(())
}
