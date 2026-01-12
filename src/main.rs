use anyhow::{Context, Result};
use clap::{Parser, Subcommand};
use colored::Colorize;
use serde::{Deserialize, Serialize};
use std::fs;
use std::io::{self, BufRead, IsTerminal, Read, Write};
use std::path::PathBuf;
use std::time::Instant;
use crossterm::{
    cursor,
    terminal::{Clear, ClearType},
    ExecutableCommand,
};

// Use the library's detect module
use rigrun::detect::{
    detect_gpu, recommend_model, GpuInfo, GpuType, is_model_available, list_ollama_models,
    check_gpu_utilization, ProcessorType, get_gpu_status_report,
    get_gpu_setup_status, AmdArchitecture,
};
use rigrun::local::{OllamaClient, Message};

mod background;
mod firstrun;

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

/// Clear the terminal screen
fn clear_screen() {
    print!("\x1B[2J\x1B[1;1H");
    std::io::Write::flush(&mut std::io::stdout()).ok();
}

/// RigRun - Local-first LLM router. Your GPU first, cloud when needed.
#[derive(Parser)]
#[command(name = "rigrun")]
#[command(version = VERSION)]
#[command(about = "Local-first LLM router. Your GPU first, cloud when needed.", long_about = None)]
#[command(propagate_version = true)]
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,

    /// Direct prompt to send to the model (or start server if not provided)
    prompt: Option<String>,
}

#[derive(Subcommand)]
enum Commands {
    /// Show current stats and server status
    Status,

    /// Configure settings
    Config {
        /// Set OpenRouter API key for cloud fallback
        #[arg(long, value_name = "KEY")]
        openrouter_key: Option<String>,

        /// Override default model
        #[arg(long, value_name = "NAME")]
        model: Option<String>,

        /// Change server port
        #[arg(long, value_name = "PORT")]
        port: Option<u16>,

        /// Show current configuration
        #[arg(long)]
        show: bool,
    },

    /// List available and downloaded models
    Models,

    /// Download a specific model
    Pull {
        /// Model name to download (e.g., qwen2.5-coder:14b)
        model: String,
    },

    /// Start interactive chat session
    Chat {
        /// Model to use (defaults to configured model)
        #[arg(short, long)]
        model: Option<String>,
    },

    /// Show practical CLI usage examples
    Examples,

    /// Run server as background process
    Background,

    /// Stop running background server
    Stop,

    /// Set up IDE integration with rigrun
    IdeSetup,

    /// Interactive GPU setup wizard
    GpuSetup,
}

#[derive(Serialize, Deserialize, Default, Clone)]
pub struct Config {
    pub openrouter_key: Option<String>,
    pub model: Option<String>,
    pub port: Option<u16>,
    #[serde(default)]
    pub first_run_complete: bool,
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
           |___/{RESET}  v{VERSION}"
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
    println!("  {YELLOW}[↓]{RESET} Downloading {model}...");

    // Use ollama pull to download the model
    let status = tokio::process::Command::new("ollama")
        .args(["pull", model])
        .status()
        .await;

    match status {
        Ok(s) if s.success() => {
            println!("  {GREEN}[✓]{RESET} Model ready");
            Ok(())
        }
        Ok(_) => {
            println!("  {RED}[✗]{RESET} Failed to download model");
            anyhow::bail!("Failed to download model")
        }
        Err(e) => {
            println!("  {RED}[✗]{RESET} Ollama not found. Please install Ollama first.");
            println!("      Visit: https://ollama.ai/download");
            anyhow::bail!("Ollama not available: {}", e)
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
    let gpu = detect_gpu().unwrap_or_default();
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
    let mut server = rigrun::Server::new(port).with_default_model(model);
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

fn handle_config(
    openrouter_key: Option<String>,
    model: Option<String>,
    port: Option<u16>,
    show: bool,
) -> Result<()> {
    let mut config = load_config()?;

    if show {
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
        return Ok(());
    }

    let mut updated = false;

    if let Some(key) = openrouter_key {
        config.openrouter_key = Some(key);
        println!("{GREEN}[✓]{RESET} OpenRouter API key set");
        updated = true;
    }

    if let Some(m) = model {
        println!("{GREEN}[✓]{RESET} Model set to: {}", m);
        config.model = Some(m);
        updated = true;
    }

    if let Some(p) = port {
        config.port = Some(p);
        println!("{GREEN}[✓]{RESET} Port set to: {}", p);
        updated = true;
    }

    if updated {
        save_config(&config)?;
        println!();
        println!("{GREEN}Configuration saved!{RESET}");
    } else {
        println!("No changes. Use --show to view current config or provide options to set.");
        println!();
        println!("Examples:");
        println!("  rigrun config --openrouter-key sk-or-xxx");
        println!("  rigrun config --model qwen2.5-coder:7b");
        println!("  rigrun config --port 8080");
        println!("  rigrun config --show");
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

    println!(
        "  {DIM}{:<25} {:<8} {:<10} {:<10} NOTES{RESET}",
        "MODEL", "SIZE", "DISK", "VRAM"
    );
    println!("  {DIM}{}{RESET}", "-".repeat(75));

    for (name, size, disk, vram, notes) in models {
        let is_downloaded = downloaded_models.iter().any(|m| m.starts_with(name.split(':').next().unwrap_or(name)));
        let status = if is_downloaded {
            format!("{GREEN}[✓]{RESET}")
        } else {
            "   ".to_string()
        };
        println!(
            "{} {WHITE}{:<25}{RESET} {:<8} {:<10} {:<10} {DIM}{}{RESET}",
            status, name, size, disk, vram, notes
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

async fn pull_model(model: String) -> Result<()> {
    println!();
    println!("{CYAN}[↓]{RESET} Pulling {WHITE}{BOLD}{model}{RESET}...");
    println!();

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

    // Use ollama pull to download the model
    println!("  Connecting to Ollama...");

    let mut child = tokio::process::Command::new("ollama")
        .args(["pull", &model])
        .stdout(std::process::Stdio::inherit())
        .stderr(std::process::Stdio::inherit())
        .spawn()
        .context("Failed to start ollama. Is Ollama installed?")?;

    let status = child.wait().await?;

    println!();

    if status.success() {
        println!(
            "{GREEN}[✓]{RESET} Model {WHITE}{BOLD}{model}{RESET} downloaded successfully!"
        );
        println!();
        println!("Start the server with: {CYAN}rigrun{RESET}");
    } else {
        println!("{RED}[✗]{RESET} Failed to download model: {model}");
        println!();
        println!("Make sure:");
        println!("  1. Ollama is installed (https://ollama.ai/download)");
        println!("  2. Ollama service is running (ollama serve)");
        println!("  3. Model name is correct");
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

        client.pull_model_with_progress(model, |progress| {
            if let Some(pct) = progress.percentage() {
                print!("\r{} {}: {:.1}%", "[↓]".bright_yellow(), progress.status, pct);
                io::stdout().flush().ok();
            }
        })?;

        println!("\n{} Model ready", "[✓]".bright_green());
    }

    Ok(())
}

fn direct_prompt(prompt: &str, model: Option<String>) -> Result<()> {
    let model = model.unwrap_or_else(get_model_from_config);
    let client = OllamaClient::new();

    ensure_model_available(&client, &model)?;

    let messages = vec![Message::user(prompt)];

    let start = Instant::now();

    let response = client.chat_stream(&model, messages, |chunk| {
        print!("{}", chunk);
        io::stdout().flush().ok();
    })?;

    println!(); // newline after response

    let elapsed = start.elapsed();
    let tokens_per_sec = if elapsed.as_secs_f64() > 0.0 {
        response.completion_tokens as f64 / elapsed.as_secs_f64()
    } else {
        0.0
    };

    println!(
        "\n{} {} tokens ({} prompt + {} completion) in {:.1}s ({:.1} tok/s)",
        "───".bright_black(),
        (response.prompt_tokens + response.completion_tokens).to_string().bright_black(),
        response.prompt_tokens.to_string().bright_black(),
        response.completion_tokens.to_string().bright_black(),
        elapsed.as_secs_f64(),
        tokens_per_sec
    );

    Ok(())
}

pub fn interactive_chat(model: Option<String>) -> Result<()> {
    let model = model.unwrap_or_else(get_model_from_config);
    let client = OllamaClient::new();

    ensure_model_available(&client, &model)?;

    println!(
        "\n{} Interactive chat mode | Model: {} | Type 'exit' or Ctrl+C to quit\n",
        "rigrun".bright_cyan().bold(),
        model.bright_white()
    );

    let mut conversation: Vec<Message> = Vec::new();
    let stdin = io::stdin();
    let mut reader = stdin.lock();

    loop {
        // Show prompt
        print!("{} ", "rigrun>".bright_cyan().bold());
        io::stdout().flush()?;

        // Read user input
        let mut input = String::new();
        reader.read_line(&mut input)?;
        let input = input.trim();

        // Check for exit
        if input.is_empty() {
            continue;
        }
        if input.eq_ignore_ascii_case("exit") || input.eq_ignore_ascii_case("quit") {
            println!("Goodbye!");
            break;
        }

        // Add user message to conversation
        conversation.push(Message::user(input));

        let start = Instant::now();

        // Get response with streaming
        let response = client.chat_stream(&model, conversation.clone(), |chunk| {
            print!("{}", chunk);
            io::stdout().flush().ok();
        })?;

        println!(); // newline after response

        // Add assistant response to conversation
        conversation.push(Message::assistant(response.response.clone()));

        // Show stats
        let elapsed = start.elapsed();
        let tokens_per_sec = if elapsed.as_secs_f64() > 0.0 {
            response.completion_tokens as f64 / elapsed.as_secs_f64()
        } else {
            0.0
        };

        println!(
            "{} {:.1}s | {} tok/s\n",
            "───".bright_black(),
            elapsed.as_secs_f64(),
            format!("{:.1}", tokens_per_sec).bright_black()
        );
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

                // Special warning for RDNA 4
                if *arch == AmdArchitecture::Rdna4 {
                    println!("    {YELLOW}[!]{RESET} RDNA 4 requires ollama-for-amd fork");
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


#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    // Check if we have a prompt argument (either direct prompt or stdin)
    let stdin_is_piped = !io::stdin().is_terminal();

    if let Some(prompt_text) = cli.prompt {
        // Direct prompt: rigrun "prompt text"
        return direct_prompt(&prompt_text, None);
    } else if stdin_is_piped && cli.command.is_none() {
        // Piped input: echo "prompt" | rigrun
        let input = read_stdin()?;
        if !input.trim().is_empty() {
            return direct_prompt(&input, None);
        }
    }

    match cli.command {
        None => {
            // Default: start the server
            let mut config = load_config()?;

            // Check if this is first run
            if !config.first_run_complete {
                // FIRST RUN: Show wizard ONLY, start server AFTER wizard completes
                clear_screen();
                if let Err(e) = firstrun::show_first_run_menu(&mut config) {
                    eprintln!("{YELLOW}[!]{RESET} Setup error: {}", e);
                }

                // After wizard completes, clear screen and start clean server
                clear_screen();
                print_banner();
                start_server(&config).await?;
            } else {
                // SUBSEQUENT RUNS: Skip wizard, go straight to clean server
                clear_screen();
                print_banner();
                start_server(&config).await?;
            }
        }
        Some(Commands::Status) => {
            show_status()?;
        }
        Some(Commands::Config {
            openrouter_key,
            model,
            port,
            show,
        }) => {
            handle_config(openrouter_key, model, port, show)?;
        }
        Some(Commands::Models) => {
            list_models()?;
        }
        Some(Commands::Pull { model }) => {
            pull_model(model).await?;
        }
        Some(Commands::Chat { model }) => {
            interactive_chat(model)?;
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
        Some(Commands::IdeSetup) => {
            handle_ide_setup().await?;
        }
        Some(Commands::GpuSetup) => {
            handle_gpu_setup()?;
        }
    }

    Ok(())
}
