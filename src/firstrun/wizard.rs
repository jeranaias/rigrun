// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

//! First-Run Wizard for rigrun
//!
//! Provides an interactive setup experience for new users, including:
//! - Deployment mode selection (Local Only, Hybrid, Cloud Primary)
//! - Use case selection (Code assistance, General chat, etc.)
//! - Hardware detection and model recommendations
//! - DoD/IL5 compliance features configuration
//! - Secure configuration generation
//! - Model download with progress tracking
//! - Health check verification

use anyhow::Result;
use inquire::{Confirm, Select};
use std::fs;
use std::io::{self, Write};
use std::path::PathBuf;

use crate::detect::{detect_gpu, recommend_model, GpuInfo, GpuType};
use crate::local::OllamaClient;

mod colors {
    pub const RESET: &str = "\x1b[0m";
    pub const BOLD: &str = "\x1b[1m";
    pub const DIM: &str = "\x1b[2m";
    pub const RED: &str = "\x1b[31m";
    pub const GREEN: &str = "\x1b[32m";
    pub const YELLOW: &str = "\x1b[33m";
    pub const CYAN: &str = "\x1b[36m";
    pub const WHITE: &str = "\x1b[37m";
    pub const BRIGHT_CYAN: &str = "\x1b[96m";
    pub const MAGENTA: &str = "\x1b[35m";
}

use colors::*;

/// Deployment modes for rigrun
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DeploymentMode {
    /// 100% offline, no cloud APIs
    LocalOnly,
    /// Local preferred, cloud fallback
    Hybrid,
    /// Cloud preferred, local fallback
    CloudPrimary,
}

impl std::fmt::Display for DeploymentMode {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            DeploymentMode::LocalOnly => write!(f, "Local Only (100% offline, no cloud APIs)"),
            DeploymentMode::Hybrid => write!(f, "Hybrid (local preferred, cloud fallback)"),
            DeploymentMode::CloudPrimary => write!(f, "Cloud Primary (cloud preferred, local fallback)"),
        }
    }
}

/// Primary use cases for rigrun
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum UseCase {
    CodeAssistance,
    GeneralChat,
    DocumentAnalysis,
    Custom,
}

impl std::fmt::Display for UseCase {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            UseCase::CodeAssistance => write!(f, "Code assistance"),
            UseCase::GeneralChat => write!(f, "General chat"),
            UseCase::DocumentAnalysis => write!(f, "Document analysis"),
            UseCase::Custom => write!(f, "Custom"),
        }
    }
}

/// Model selection result from the wizard
#[derive(Debug, Clone)]
pub enum ModelSelection {
    Accept(String),
    Choose(String),
    Skip,
}

/// Configuration collected from the wizard
#[derive(Debug, Clone)]
pub struct WizardConfig {
    pub deployment_mode: DeploymentMode,
    pub use_case: UseCase,
    pub model: Option<String>,
    pub enable_compliance: bool,
    pub generate_secure_config: bool,
    pub openrouter_key: Option<String>,
}

impl Default for WizardConfig {
    fn default() -> Self {
        Self {
            deployment_mode: DeploymentMode::LocalOnly,
            use_case: UseCase::CodeAssistance,
            model: None,
            enable_compliance: false,
            generate_secure_config: true,
            openrouter_key: None,
        }
    }
}

/// Check if this is the first run (no config file exists)
pub fn is_first_run() -> bool {
    let config_dir = dirs::home_dir()
        .map(|h| h.join(".rigrun"))
        .unwrap_or_else(|| PathBuf::from(".rigrun"));

    let config_file = config_dir.join("config.json");
    let wizard_marker = config_dir.join(".wizard_complete");

    // First run if config doesn't exist OR wizard marker doesn't exist
    !config_file.exists() || !wizard_marker.exists()
}

/// Mark the wizard as completed
pub fn mark_wizard_complete() -> Result<()> {
    let config_dir = dirs::home_dir()
        .map(|h| h.join(".rigrun"))
        .unwrap_or_else(|| PathBuf::from(".rigrun"));

    if !config_dir.exists() {
        fs::create_dir_all(&config_dir)?;
    }

    let marker_path = config_dir.join(".wizard_complete");
    fs::write(marker_path, chrono::Utc::now().to_rfc3339())?;
    Ok(())
}

/// Clear the terminal screen
fn clear_screen() {
    print!("\x1B[2J\x1B[1;1H");
    io::stdout().flush().ok();
}

/// Display the welcome banner
fn show_welcome_banner() {
    println!();
    println!("{BRIGHT_CYAN}{BOLD}+==============================================================+{RESET}");
    println!("{BRIGHT_CYAN}{BOLD}|                                                              |{RESET}");
    println!("{BRIGHT_CYAN}{BOLD}|                    {WHITE}Welcome to rigrun{BRIGHT_CYAN}                        |{RESET}");
    println!("{BRIGHT_CYAN}{BOLD}|            {WHITE}Local-First LLM Router for IL5 Compliance{BRIGHT_CYAN}        |{RESET}");
    println!("{BRIGHT_CYAN}{BOLD}|                                                              |{RESET}");
    println!("{BRIGHT_CYAN}{BOLD}+==============================================================+{RESET}");
    println!();
    println!("{WHITE}Let's get you set up! This will take about 2 minutes.{RESET}");
    println!();
}

/// Display a step indicator
fn show_step(current: u32, total: u32, description: &str) {
    println!("{CYAN}[{current}/{total}]{RESET} {WHITE}{description}{RESET}");
}

/// Prompt for deployment mode selection
fn select_deployment_mode() -> Result<DeploymentMode> {
    let options = vec![
        "Local Only (100% offline, no cloud APIs)",
        "Hybrid (local preferred, cloud fallback)",
        "Cloud Primary (cloud preferred, local fallback)",
    ];

    let selection = Select::new("Select deployment mode:", options)
        .with_help_message("Use arrow keys to navigate, Enter to select")
        .with_vim_mode(true)
        .prompt()?;

    match selection {
        "Local Only (100% offline, no cloud APIs)" => Ok(DeploymentMode::LocalOnly),
        "Hybrid (local preferred, cloud fallback)" => Ok(DeploymentMode::Hybrid),
        "Cloud Primary (cloud preferred, local fallback)" => Ok(DeploymentMode::CloudPrimary),
        _ => Ok(DeploymentMode::LocalOnly),
    }
}

/// Prompt for use case selection
fn select_use_case() -> Result<UseCase> {
    let options = vec![
        "Code assistance",
        "General chat",
        "Document analysis",
        "Custom",
    ];

    let selection = Select::new("Select your primary use case:", options)
        .with_help_message("Use arrow keys to navigate, Enter to select")
        .with_vim_mode(true)
        .prompt()?;

    match selection {
        "Code assistance" => Ok(UseCase::CodeAssistance),
        "General chat" => Ok(UseCase::GeneralChat),
        "Document analysis" => Ok(UseCase::DocumentAnalysis),
        "Custom" => Ok(UseCase::Custom),
        _ => Ok(UseCase::CodeAssistance),
    }
}

/// Display hardware detection results and get model selection
fn hardware_detection_and_model_selection(use_case: UseCase) -> Result<ModelSelection> {
    let gpu = detect_gpu().unwrap_or_default();

    println!();

    // Display detected hardware
    if gpu.gpu_type == GpuType::Cpu {
        println!("{YELLOW}⚠{RESET} {WHITE}No GPU detected - CPU mode{RESET}");
        println!("    {DIM}Performance will be slower without GPU acceleration{RESET}");
    } else {
        println!("{GREEN}✓{RESET} {WHITE}We detected: {BOLD}{}{RESET} ({CYAN}{}GB VRAM{RESET})",
            gpu.name, gpu.vram_gb);
        if let Some(ref driver) = gpu.driver {
            println!("    {DIM}Driver: {}{RESET}", driver);
        }
    }

    // Get recommended model based on hardware and use case
    let recommended = get_recommended_model(&gpu, use_case);

    println!();
    println!("    {WHITE}Recommended model: {CYAN}{BOLD}{}{RESET}", recommended);

    // Show model characteristics
    let (size_info, vram_req) = get_model_info(&recommended);
    println!("    {DIM}Size: {} | VRAM: {}{RESET}", size_info, vram_req);

    println!();

    let options = vec![
        "Accept recommendation",
        "Choose different model",
        "Skip model download",
    ];

    let selection = Select::new("Model selection:", options)
        .with_help_message("Use arrow keys to navigate, Enter to select")
        .prompt()?;

    match selection {
        "Accept recommendation" => Ok(ModelSelection::Accept(recommended)),
        "Choose different model" => {
            let model = choose_model(&gpu)?;
            Ok(ModelSelection::Choose(model))
        }
        "Skip model download" => Ok(ModelSelection::Skip),
        _ => Ok(ModelSelection::Accept(recommended)),
    }
}

/// Get recommended model based on GPU and use case
fn get_recommended_model(gpu: &GpuInfo, use_case: UseCase) -> String {
    let base_recommendation = recommend_model(gpu.vram_gb);

    // Adjust recommendation based on use case
    match use_case {
        UseCase::CodeAssistance => base_recommendation,
        UseCase::GeneralChat => {
            // For general chat, prefer llama or mistral over code-specific models
            match gpu.vram_gb {
                0..=5 => "llama3.2:3b".to_string(),
                6..=9 => "llama3.2:8b".to_string(),
                10..=17 => "llama3.2:8b".to_string(),
                _ => "llama3.2:8b".to_string(),
            }
        }
        UseCase::DocumentAnalysis => {
            // For document analysis, prefer models with longer context
            match gpu.vram_gb {
                0..=9 => "qwen2.5:7b".to_string(),
                10..=17 => "qwen2.5:14b".to_string(),
                _ => "qwen2.5:32b".to_string(),
            }
        }
        UseCase::Custom => base_recommendation,
    }
}

/// Get model size and VRAM requirement info
fn get_model_info(model: &str) -> (&'static str, &'static str) {
    match model {
        m if m.contains("1.5b") || m.contains("1b") => ("~1 GB", "4GB+"),
        m if m.contains("3b") => ("~2 GB", "6GB+"),
        m if m.contains("7b") || m.contains("8b") => ("~4.5 GB", "10GB+"),
        m if m.contains("14b") => ("~8 GB", "16GB+"),
        m if m.contains("16b") => ("~10 GB", "16GB+"),
        m if m.contains("22b") => ("~13 GB", "20GB+"),
        m if m.contains("30b") || m.contains("32b") => ("~18 GB", "24GB+"),
        _ => ("Unknown", "Unknown"),
    }
}

/// Let user choose from a list of available models
fn choose_model(gpu: &GpuInfo) -> Result<String> {
    let models = if gpu.vram_gb >= 16 {
        vec![
            "qwen2.5-coder:32b (Best for 24GB+ VRAM)",
            "qwen2.5-coder:14b (Recommended for 16GB)",
            "codestral:22b (Great for autocomplete)",
            "qwen2.5-coder:7b (Fast, 10GB+)",
            "deepseek-coder-v2:16b (Strong debugging)",
            "qwen2.5-coder:3b (Lightweight)",
        ]
    } else if gpu.vram_gb >= 10 {
        vec![
            "qwen2.5-coder:14b (Recommended for 16GB)",
            "qwen2.5-coder:7b (Fast, balanced)",
            "deepseek-coder-v2:16b (Strong debugging)",
            "codestral:22b (Fits with offloading)",
            "qwen2.5-coder:3b (Lightweight)",
        ]
    } else if gpu.vram_gb >= 6 {
        vec![
            "qwen2.5-coder:7b (Recommended for 8GB)",
            "qwen2.5-coder:3b (Lightweight)",
            "deepseek-coder-v2:lite (Good alternative)",
            "llama3.2:8b (General purpose)",
        ]
    } else {
        vec![
            "qwen2.5-coder:3b (Recommended for limited VRAM)",
            "qwen2.5-coder:1.5b (Minimal footprint)",
            "llama3.2:3b (General purpose)",
        ]
    };

    let selection = Select::new("Choose a model:", models)
        .with_help_message("Models listed by VRAM requirement")
        .prompt()?;

    // Extract model name from selection (before the parenthesis)
    let model = selection.split(" (").next().unwrap_or(selection).trim();
    Ok(model.to_string())
}

/// Prompt for DoD/IL5 compliance features
fn select_compliance_features() -> Result<bool> {
    println!();
    println!("{MAGENTA}{BOLD}DoD/IL5 Compliance Features:{RESET}");
    println!("  {DIM}- Consent banners before data processing{RESET}");
    println!("  {DIM}- Session timeout after inactivity{RESET}");
    println!("  {DIM}- Full audit logging of all queries{RESET}");
    println!("  {DIM}- Data classification awareness{RESET}");
    println!();

    let options = vec![
        "Yes (consent banners, session timeout, audit logging)",
        "No (standard operation)",
    ];

    let selection = Select::new("Enable DoD/IL5 compliance features?", options)
        .with_help_message("Recommended for government/defense environments")
        .prompt()?;

    Ok(selection.starts_with("Yes"))
}

/// Prompt for secure configuration generation
fn select_secure_config() -> Result<bool> {
    println!();

    let options = vec![
        "Yes (recommended)",
        "No (I'll configure manually)",
    ];

    let selection = Select::new("Generate secure configuration?", options)
        .with_help_message("Creates optimized config with security defaults")
        .prompt()?;

    Ok(selection.starts_with("Yes"))
}

/// Prompt for OpenRouter API key if needed
fn prompt_openrouter_key(mode: DeploymentMode) -> Result<Option<String>> {
    if mode == DeploymentMode::LocalOnly {
        return Ok(None);
    }

    println!();
    println!("{CYAN}{BOLD}Cloud Fallback Configuration{RESET}");
    println!("{DIM}For Hybrid and Cloud Primary modes, you need an OpenRouter API key.{RESET}");
    println!("{DIM}Get one at: {WHITE}https://openrouter.ai/keys{RESET}");
    println!();

    let setup_now = Confirm::new("Would you like to set up cloud access now?")
        .with_default(false)
        .prompt()?;

    if !setup_now {
        println!();
        println!("{YELLOW}⚠{RESET} Skipped. You can set this up later with:");
        println!("    {CYAN}rigrun config set-key YOUR_KEY{RESET}");
        return Ok(None);
    }

    // Open browser to OpenRouter
    let _ = open_browser("https://openrouter.ai/keys");

    println!();
    let key = inquire::Text::new("Paste your API key here:")
        .with_help_message("OpenRouter keys start with 'sk-or-'")
        .prompt()?;

    let key = key.trim();
    if key.is_empty() {
        return Ok(None);
    }

    // Validate key format
    if !key.starts_with("sk-or-") {
        println!("{YELLOW}⚠{RESET} Warning: Key doesn't start with 'sk-or-'. It may not be valid.");
    }

    Ok(Some(key.to_string()))
}

/// Open URL in default browser
fn open_browser(url: &str) -> Result<()> {
    #[cfg(target_os = "windows")]
    {
        std::process::Command::new("cmd")
            .args(["/C", "start", "", url])
            .spawn()?;
    }

    #[cfg(target_os = "macos")]
    {
        std::process::Command::new("open")
            .arg(url)
            .spawn()?;
    }

    #[cfg(target_os = "linux")]
    {
        std::process::Command::new("xdg-open")
            .arg(url)
            .spawn()?;
    }

    Ok(())
}

/// Download model with progress bar using indicatif
pub fn download_model_with_progress(model: &str) -> Result<()> {
    use crate::detect::is_model_available;
    use indicatif::{ProgressBar, ProgressStyle};
    use std::time::Duration;

    // Check if already downloaded
    if is_model_available(model) {
        println!("{GREEN}✓{RESET} Model {WHITE}{BOLD}{model}{RESET} already downloaded");
        return Ok(());
    }

    println!();
    println!("{CYAN}⋯{RESET} Downloading {WHITE}{BOLD}{model}{RESET}...");
    println!("{DIM}This is a one-time download. Future starts are instant.{RESET}");
    println!();

    let client = OllamaClient::new();

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
    let mut last_percentage: u64 = 0;
    let mut current_status = String::new();

    let result = client.pull_model_with_progress(model, |progress| {
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
    });

    pb.finish_and_clear();

    match result {
        Ok(()) => {
            println!("{GREEN}✓{RESET} Model {WHITE}{BOLD}{model}{RESET} downloaded successfully!");
            Ok(())
        }
        Err(e) => {
            let err_str = e.to_string();
            if err_str.contains("Cannot connect") || err_str.contains("not running") {
                println!("{RED}✗{RESET} Ollama not running. Start it with: {CYAN}ollama serve{RESET}");
                Err(e)
            } else if err_str.contains("not found") {
                println!("{RED}✗{RESET} Model not found: {WHITE}{model}{RESET}");
                Err(e)
            } else {
                println!("{RED}✗{RESET} Download failed: {}", e);
                Err(e)
            }
        }
    }
}

/// Run health check to verify setup
pub fn run_health_check(model: &Option<String>) -> Result<bool> {
    println!();
    println!("{CYAN}{BOLD}Running health check...{RESET}");
    println!();

    let mut all_passed = true;

    // Check 1: Ollama installed
    print!("  {DIM}Checking Ollama installation...{RESET}");
    io::stdout().flush().ok();

    if crate::detect::check_ollama_available() {
        println!("\r  {GREEN}✓{RESET} Ollama is installed                    ");
    } else {
        println!("\r  {RED}✗{RESET} Ollama not found                        ");
        println!("      {DIM}Install from: https://ollama.ai/download{RESET}");
        all_passed = false;
    }

    // Check 2: Ollama running
    print!("  {DIM}Checking Ollama service...{RESET}");
    io::stdout().flush().ok();

    let client = OllamaClient::new();
    if client.check_ollama_running() {
        println!("\r  {GREEN}✓{RESET} Ollama service is running               ");
    } else {
        println!("\r  {YELLOW}⚠{RESET} Ollama service not running             ");
        println!("      {DIM}Start with: ollama serve{RESET}");
        all_passed = false;
    }

    // Check 3: GPU detection
    print!("  {DIM}Checking GPU...{RESET}");
    io::stdout().flush().ok();

    let gpu = detect_gpu().unwrap_or_default();
    if gpu.gpu_type != GpuType::Cpu {
        println!("\r  {GREEN}✓{RESET} GPU detected: {} ({}GB)              ", gpu.name, gpu.vram_gb);
    } else {
        println!("\r  {YELLOW}⚠{RESET} No GPU detected (CPU mode)             ");
    }

    // Check 4: Model availability
    if let Some(model_name) = model {
        print!("  {DIM}Checking model...{RESET}");
        io::stdout().flush().ok();

        if crate::detect::is_model_available(model_name) {
            println!("\r  {GREEN}✓{RESET} Model {} available               ", model_name);
        } else {
            println!("\r  {YELLOW}⚠{RESET} Model {} not found              ", model_name);
            all_passed = false;
        }
    }

    // Check 5: Configuration
    print!("  {DIM}Checking configuration...{RESET}");
    io::stdout().flush().ok();

    let config_dir = dirs::home_dir()
        .map(|h| h.join(".rigrun"))
        .unwrap_or_else(|| PathBuf::from(".rigrun"));

    if config_dir.join("config.json").exists() {
        println!("\r  {GREEN}✓{RESET} Configuration file exists               ");
    } else {
        println!("\r  {YELLOW}⚠{RESET} No configuration file found            ");
    }

    println!();

    if all_passed {
        println!("{GREEN}{BOLD}✓ All checks passed! rigrun is ready.{RESET}");
    } else {
        println!("{YELLOW}{BOLD}⚠ Some checks failed. See above for details.{RESET}");
    }

    Ok(all_passed)
}

/// Generate and save configuration based on wizard selections
pub fn generate_config(wizard_config: &WizardConfig) -> Result<()> {
    let config_dir = dirs::home_dir()
        .map(|h| h.join(".rigrun"))
        .unwrap_or_else(|| PathBuf::from(".rigrun"));

    if !config_dir.exists() {
        fs::create_dir_all(&config_dir)?;
    }

    // Build config structure
    let paranoid_mode = wizard_config.deployment_mode == DeploymentMode::LocalOnly;

    let config = serde_json::json!({
        "openrouter_key": wizard_config.openrouter_key,
        "model": wizard_config.model,
        "port": 8787,
        "first_run_complete": true,
        "audit_log_enabled": wizard_config.enable_compliance,
        "paranoid_mode": paranoid_mode,
        "deployment_mode": match wizard_config.deployment_mode {
            DeploymentMode::LocalOnly => "local_only",
            DeploymentMode::Hybrid => "hybrid",
            DeploymentMode::CloudPrimary => "cloud_primary",
        },
        "use_case": match wizard_config.use_case {
            UseCase::CodeAssistance => "code_assistance",
            UseCase::GeneralChat => "general_chat",
            UseCase::DocumentAnalysis => "document_analysis",
            UseCase::Custom => "custom",
        },
        "compliance": {
            "enabled": wizard_config.enable_compliance,
            "consent_banner": wizard_config.enable_compliance,
            "session_timeout_minutes": if wizard_config.enable_compliance { 30 } else { 0 },
            "audit_all_queries": wizard_config.enable_compliance,
        }
    });

    let config_path = config_dir.join("config.json");
    let content = serde_json::to_string_pretty(&config)?;
    fs::write(&config_path, content)?;

    println!("{GREEN}✓{RESET} Configuration saved to {}", config_path.display());

    // Generate TOML config if secure config requested
    if wizard_config.generate_secure_config {
        let toml_config = generate_toml_config(wizard_config);
        let toml_path = config_dir.join("config.toml");
        fs::write(&toml_path, toml_config)?;
        println!("{GREEN}✓{RESET} Secure configuration saved to {}", toml_path.display());
    }

    Ok(())
}

/// Generate TOML configuration for advanced users
fn generate_toml_config(wizard_config: &WizardConfig) -> String {
    let paranoid = wizard_config.deployment_mode == DeploymentMode::LocalOnly;
    let model = wizard_config.model.as_deref().unwrap_or("auto");

    format!(r#"# rigrun Configuration
# Generated by first-run wizard
# Location: ~/.rigrun/config.toml

[server]
port = 8787
host = "127.0.0.1"

[routing]
# Deployment mode: "local_only", "hybrid", or "cloud_primary"
mode = "{mode}"

# Paranoid mode blocks ALL cloud requests
paranoid_mode = {paranoid}

# Default model for local inference
default_model = "{model}"

[compliance]
# Enable DoD/IL5 compliance features
enabled = {compliance}

# Show consent banner before processing
consent_banner = {compliance}

# Session timeout in minutes (0 = disabled)
session_timeout_minutes = {timeout}

# Log all queries for audit
audit_all_queries = {compliance}

[cloud]
# OpenRouter API key (optional)
{api_key_line}

# Cloud model preferences
preferred_models = [
    "anthropic/claude-3-5-sonnet",
    "anthropic/claude-3-haiku",
    "openai/gpt-4o",
]

[cache]
# Enable response caching
enabled = true

# Cache TTL in seconds
ttl_seconds = 3600

# Maximum cache size in MB
max_size_mb = 100

[logging]
# Log level: "error", "warn", "info", "debug", "trace"
level = "info"

# Enable structured JSON logging
json_format = false
"#,
        mode = match wizard_config.deployment_mode {
            DeploymentMode::LocalOnly => "local_only",
            DeploymentMode::Hybrid => "hybrid",
            DeploymentMode::CloudPrimary => "cloud_primary",
        },
        paranoid = paranoid,
        model = model,
        compliance = wizard_config.enable_compliance,
        timeout = if wizard_config.enable_compliance { 30 } else { 0 },
        api_key_line = wizard_config.openrouter_key.as_ref()
            .map(|k| format!("api_key = \"{}\"", k))
            .unwrap_or_else(|| "# api_key = \"sk-or-...\"".to_string()),
    )
}

/// Display completion summary
fn show_completion_summary(wizard_config: &WizardConfig, health_passed: bool) {
    println!();
    println!("{BRIGHT_CYAN}{BOLD}+==============================================================+{RESET}");
    println!("{BRIGHT_CYAN}{BOLD}|                                                              |{RESET}");
    println!("{BRIGHT_CYAN}{BOLD}|                    {GREEN}Setup Complete!{BRIGHT_CYAN}                          |{RESET}");
    println!("{BRIGHT_CYAN}{BOLD}|                                                              |{RESET}");
    println!("{BRIGHT_CYAN}{BOLD}+==============================================================+{RESET}");
    println!();

    println!("{WHITE}{BOLD}Configuration Summary:{RESET}");
    println!("  Deployment:  {CYAN}{}{RESET}", wizard_config.deployment_mode);
    println!("  Use Case:    {CYAN}{}{RESET}", wizard_config.use_case);
    if let Some(ref model) = wizard_config.model {
        println!("  Model:       {CYAN}{}{RESET}", model);
    }
    println!("  Compliance:  {}", if wizard_config.enable_compliance {
        format!("{GREEN}Enabled{RESET}")
    } else {
        format!("{DIM}Disabled{RESET}")
    });
    println!("  Cloud:       {}", if wizard_config.openrouter_key.is_some() {
        format!("{GREEN}Configured{RESET}")
    } else {
        format!("{DIM}Not configured{RESET}")
    });

    println!();
    println!("{WHITE}{BOLD}What's Next:{RESET}");
    println!();
    println!("  {GREEN}1.{RESET} Start the server:");
    println!("     {CYAN}rigrun{RESET}");
    println!();
    println!("  {GREEN}2.{RESET} Try a quick question:");
    println!("     {CYAN}rigrun ask \"Explain recursion\"{RESET}");
    println!();
    println!("  {GREEN}3.{RESET} Connect your IDE:");
    println!("     {CYAN}rigrun setup ide{RESET}");
    println!();

    if !health_passed {
        println!("{YELLOW}⚠{RESET} Some health checks failed. Run {CYAN}rigrun doctor{RESET} for details.");
        println!();
    }

    println!("{DIM}Server endpoint: http://localhost:8787{RESET}");
    println!("{DIM}Configuration: ~/.rigrun/config.json{RESET}");
    println!();
}

/// Run the complete first-run wizard
pub async fn run_wizard() -> Result<WizardConfig> {
    clear_screen();
    show_welcome_banner();

    let mut wizard_config = WizardConfig::default();
    let total_steps = 5;

    // Step 1: Deployment mode
    show_step(1, total_steps, "Deployment Mode");
    wizard_config.deployment_mode = select_deployment_mode()?;
    println!();

    // Step 2: Use case
    show_step(2, total_steps, "Primary Use Case");
    wizard_config.use_case = select_use_case()?;
    println!();

    // Step 3: Hardware detection and model selection
    show_step(3, total_steps, "Hardware Detection & Model");
    let model_selection = hardware_detection_and_model_selection(wizard_config.use_case)?;

    let selected_model = match model_selection {
        ModelSelection::Accept(model) | ModelSelection::Choose(model) => Some(model),
        ModelSelection::Skip => None,
    };
    wizard_config.model = selected_model.clone();
    println!();

    // Step 4: Compliance features
    show_step(4, total_steps, "Compliance Configuration");
    wizard_config.enable_compliance = select_compliance_features()?;
    println!();

    // Step 5: Secure config generation
    show_step(5, total_steps, "Configuration Generation");
    wizard_config.generate_secure_config = select_secure_config()?;

    // Optional: OpenRouter key for non-local modes
    wizard_config.openrouter_key = prompt_openrouter_key(wizard_config.deployment_mode)?;

    // Setup phase
    println!();
    println!("{BRIGHT_CYAN}{BOLD}Setting up...{RESET}");
    println!();

    // Generate and save configuration
    generate_config(&wizard_config)?;

    // Download model if selected
    if let Some(ref model) = selected_model {
        if let Err(e) = download_model_with_progress(model) {
            println!("{YELLOW}⚠{RESET} Model download failed: {}", e);
            println!("    {DIM}You can download later with: rigrun pull {}{RESET}", model);
        }
    }

    // Run health check
    let health_passed = run_health_check(&wizard_config.model)?;

    // Mark wizard as complete
    mark_wizard_complete()?;

    // Show summary
    show_completion_summary(&wizard_config, health_passed);

    Ok(wizard_config)
}

/// Simplified wizard for quick setup (skips some options)
pub async fn run_quick_wizard() -> Result<WizardConfig> {
    clear_screen();
    show_welcome_banner();

    println!("{CYAN}Quick Setup Mode{RESET}");
    println!("{DIM}Using recommended defaults for your system.{RESET}");
    println!();

    let mut wizard_config = WizardConfig::default();

    // Auto-detect hardware
    let gpu = detect_gpu().unwrap_or_default();
    if gpu.gpu_type != GpuType::Cpu {
        println!("{GREEN}✓{RESET} Detected: {} ({}GB VRAM)", gpu.name, gpu.vram_gb);
    } else {
        println!("{YELLOW}⚠{RESET} No GPU detected - using CPU mode");
    }

    // Use recommended model
    let model = recommend_model(gpu.vram_gb);
    wizard_config.model = Some(model.clone());
    println!("{GREEN}✓{RESET} Recommended model: {}", model);

    // Use defaults
    wizard_config.deployment_mode = DeploymentMode::LocalOnly;
    wizard_config.enable_compliance = false;
    wizard_config.generate_secure_config = true;

    println!();
    println!("{CYAN}⋯{RESET} Setting up...");

    // Generate config
    generate_config(&wizard_config)?;

    // Download model
    if let Err(e) = download_model_with_progress(&model) {
        println!("{YELLOW}⚠{RESET} Model download failed: {}", e);
    }

    // Mark complete
    mark_wizard_complete()?;

    // Run health check
    let health_passed = run_health_check(&wizard_config.model)?;
    show_completion_summary(&wizard_config, health_passed);

    Ok(wizard_config)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_deployment_mode_display() {
        assert!(format!("{}", DeploymentMode::LocalOnly).contains("Local Only"));
        assert!(format!("{}", DeploymentMode::Hybrid).contains("Hybrid"));
        assert!(format!("{}", DeploymentMode::CloudPrimary).contains("Cloud Primary"));
    }

    #[test]
    fn test_use_case_display() {
        assert!(format!("{}", UseCase::CodeAssistance).contains("Code"));
        assert!(format!("{}", UseCase::GeneralChat).contains("chat"));
    }

    #[test]
    fn test_model_info() {
        let (size, vram) = get_model_info("qwen2.5-coder:7b");
        assert!(size.contains("GB"));
        assert!(vram.contains("GB"));
    }

    #[test]
    fn test_recommended_model_for_use_case() {
        let gpu = GpuInfo {
            name: "Test GPU".to_string(),
            vram_gb: 16,
            driver: None,
            gpu_type: GpuType::Nvidia,
        };

        let code_model = get_recommended_model(&gpu, UseCase::CodeAssistance);
        assert!(code_model.contains("coder") || code_model.contains("code"));

        let chat_model = get_recommended_model(&gpu, UseCase::GeneralChat);
        assert!(chat_model.contains("llama") || chat_model.contains("qwen"));
    }
}
