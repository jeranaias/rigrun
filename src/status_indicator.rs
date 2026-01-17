// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

//! Status Indicator Module for rigrun CLI
//!
//! Provides a persistent status line showing current model, mode, and GPU status
//! in the interactive chat session. Inspired by Claude Code's status display.
//!
//! ## Status Line Styles
//!
//! - **Full**: Box-style status line with detailed info
//! - **Compact**: Single-line inline status
//! - **Minimal**: Ultra-compact indicator
//!
//! ## Example
//!
//! ```no_run
//! use rigrun::status_indicator::{StatusIndicator, StatusConfig, StatusLineStyle, OperatingMode};
//!
//! let config = StatusConfig::default();
//! let mut indicator = StatusIndicator::new(config);
//! indicator.set_model("qwen2.5-coder:14b");
//! indicator.set_mode(OperatingMode::Local);
//! indicator.render();
//! ```

use colored::Colorize;
use serde::{Deserialize, Serialize};
use std::io::{self, Write};

use crate::detect::{
    detect_gpu, get_gpu_memory_usage, get_ollama_loaded_models, GpuInfo, GpuMemoryUsage, GpuType,
    ProcessorType,
};

/// Maximum model name length before truncation.
const MAX_MODEL_NAME_LEN: usize = 20;

/// Operating mode for the CLI.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OperatingMode {
    /// Local-only mode (Ollama)
    Local,
    /// Cloud-only mode (OpenRouter)
    Cloud,
    /// Automatic routing between local and cloud
    Auto,
    /// Hybrid mode with preference settings
    Hybrid,
}

impl Default for OperatingMode {
    fn default() -> Self {
        Self::Local
    }
}

impl std::fmt::Display for OperatingMode {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Local => write!(f, "local"),
            Self::Cloud => write!(f, "cloud"),
            Self::Auto => write!(f, "auto"),
            Self::Hybrid => write!(f, "hybrid"),
        }
    }
}

impl OperatingMode {
    /// Parse mode from string.
    pub fn from_str(s: &str) -> Option<Self> {
        match s.to_lowercase().as_str() {
            "local" => Some(Self::Local),
            "cloud" => Some(Self::Cloud),
            "auto" => Some(Self::Auto),
            "hybrid" => Some(Self::Hybrid),
            _ => None,
        }
    }
}

/// Style for the status line display.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
pub enum StatusLineStyle {
    /// Full box-style status line
    /// ```text
    /// +-  rigrun ----------------------------------------+
    /// | Model: qwen2.5-coder:14b | Mode: local | GPU: RX 9070 XT |
    /// +--------------------------------------------------+
    /// ```
    Full,
    /// Compact inline status (default)
    /// ```text
    /// [qwen2.5:14b | local | GPU]
    /// ```
    #[default]
    Compact,
    /// Minimal status indicator
    /// ```text
    /// [local|GPU]
    /// ```
    Minimal,
}

impl StatusLineStyle {
    /// Parse style from string.
    pub fn from_str(s: &str) -> Option<Self> {
        match s.to_lowercase().as_str() {
            "full" => Some(Self::Full),
            "compact" => Some(Self::Compact),
            "minimal" => Some(Self::Minimal),
            _ => None,
        }
    }
}

/// Configuration for the status indicator.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StatusConfig {
    /// Whether to show the status line.
    pub show_status_line: bool,
    /// Style of the status line.
    pub status_line_style: StatusLineStyle,
    /// Whether to show session time remaining.
    pub show_session_time: bool,
    /// Whether to show VRAM usage.
    pub show_vram_usage: bool,
    /// Whether to show token count.
    pub show_token_count: bool,
}

impl Default for StatusConfig {
    fn default() -> Self {
        Self {
            show_status_line: true,
            status_line_style: StatusLineStyle::Compact,
            show_session_time: true,
            show_vram_usage: false,
            show_token_count: false,
        }
    }
}

/// GPU status for display.
#[derive(Debug, Clone)]
pub struct GpuStatus {
    /// GPU name (may be truncated).
    pub name: String,
    /// Whether GPU is being used.
    pub using_gpu: bool,
    /// GPU type.
    pub gpu_type: GpuType,
    /// VRAM usage if available.
    pub vram_usage: Option<GpuMemoryUsage>,
    /// Processor type for current model.
    pub processor: Option<ProcessorType>,
}

impl GpuStatus {
    /// Create GPU status from current system state.
    pub fn from_system(model_name: Option<&str>) -> Self {
        let gpu_info = detect_gpu().unwrap_or_default();
        let vram_usage = get_gpu_memory_usage(Some(&gpu_info));

        // Check if the current model is using GPU
        let (using_gpu, processor) = if let Some(model) = model_name {
            let loaded_models = get_ollama_loaded_models();
            if let Some(loaded) = loaded_models.iter().find(|m| m.name.starts_with(model)) {
                let is_using = match &loaded.processor {
                    ProcessorType::Gpu(_) => true,
                    ProcessorType::Mixed { gpu_percent, .. } => *gpu_percent > 0,
                    ProcessorType::Cpu => false,
                    ProcessorType::Unknown => gpu_info.gpu_type != GpuType::Cpu,
                };
                (is_using, Some(loaded.processor.clone()))
            } else {
                // Model not loaded yet, assume GPU if available
                (gpu_info.gpu_type != GpuType::Cpu, None)
            }
        } else {
            (gpu_info.gpu_type != GpuType::Cpu, None)
        };

        // Truncate GPU name for display
        let name = if gpu_info.name.len() > 25 {
            format!("{}...", &gpu_info.name[..22])
        } else {
            gpu_info.name.clone()
        };

        Self {
            name,
            using_gpu,
            gpu_type: gpu_info.gpu_type,
            vram_usage,
            processor,
        }
    }

    /// Get a short display string for the GPU.
    pub fn short_display(&self) -> String {
        if self.gpu_type == GpuType::Cpu {
            "CPU".to_string()
        } else if self.using_gpu {
            "GPU".to_string()
        } else {
            "CPU*".to_string() // GPU available but using CPU
        }
    }
}

/// Session statistics for the /status command.
#[derive(Debug, Clone, Default)]
pub struct SessionStats {
    /// Number of messages in current conversation.
    pub message_count: u32,
    /// Approximate tokens used.
    pub tokens_used: u32,
    /// Session start time.
    pub session_start: Option<std::time::Instant>,
    /// Session timeout in seconds.
    pub session_timeout_secs: u64,
}

impl SessionStats {
    /// Get time remaining in session.
    pub fn time_remaining_secs(&self) -> u64 {
        if let Some(start) = self.session_start {
            let elapsed = start.elapsed().as_secs();
            self.session_timeout_secs.saturating_sub(elapsed)
        } else {
            self.session_timeout_secs
        }
    }

    /// Format time remaining as MM:SS.
    pub fn format_time_remaining(&self) -> String {
        let remaining = self.time_remaining_secs();
        let mins = remaining / 60;
        let secs = remaining % 60;
        format!("{}:{:02}", mins, secs)
    }
}

/// The main status indicator that tracks and displays current state.
#[derive(Debug, Clone)]
pub struct StatusIndicator {
    /// Configuration for display.
    pub config: StatusConfig,
    /// Current model name.
    model: String,
    /// Current operating mode.
    mode: OperatingMode,
    /// GPU status (cached).
    gpu_status: Option<GpuStatus>,
    /// Session statistics.
    stats: SessionStats,
    /// Whether auto-routing is enabled.
    auto_routing_enabled: bool,
}

impl StatusIndicator {
    /// Create a new status indicator with the given configuration.
    pub fn new(config: StatusConfig) -> Self {
        Self {
            config,
            model: String::new(),
            mode: OperatingMode::Local,
            gpu_status: None,
            stats: SessionStats::default(),
            auto_routing_enabled: false,
        }
    }

    /// Set the current model name.
    pub fn set_model(&mut self, model: impl Into<String>) {
        self.model = model.into();
        // Refresh GPU status when model changes
        self.refresh_gpu_status();
    }

    /// Get the current model name.
    pub fn model(&self) -> &str {
        &self.model
    }

    /// Set the operating mode.
    pub fn set_mode(&mut self, mode: OperatingMode) {
        self.mode = mode;
    }

    /// Get the current mode.
    pub fn mode(&self) -> OperatingMode {
        self.mode
    }

    /// Set whether auto-routing is enabled.
    pub fn set_auto_routing(&mut self, enabled: bool) {
        self.auto_routing_enabled = enabled;
    }

    /// Update session statistics.
    pub fn update_stats(&mut self, message_count: u32, tokens_used: u32) {
        self.stats.message_count = message_count;
        self.stats.tokens_used = tokens_used;
    }

    /// Set session start time.
    pub fn set_session_start(&mut self, start: std::time::Instant, timeout_secs: u64) {
        self.stats.session_start = Some(start);
        self.stats.session_timeout_secs = timeout_secs;
    }

    /// Refresh GPU status from system.
    pub fn refresh_gpu_status(&mut self) {
        let model = if self.model.is_empty() {
            None
        } else {
            Some(self.model.as_str())
        };
        self.gpu_status = Some(GpuStatus::from_system(model));
    }

    /// Get the GPU status.
    pub fn gpu_status(&self) -> Option<&GpuStatus> {
        self.gpu_status.as_ref()
    }

    /// Truncate model name for display.
    fn truncate_model_name(&self) -> String {
        if self.model.len() > MAX_MODEL_NAME_LEN {
            format!("{}...", &self.model[..MAX_MODEL_NAME_LEN - 3])
        } else {
            self.model.clone()
        }
    }

    /// Render the status line to stdout.
    pub fn render(&self) {
        if !self.config.show_status_line {
            return;
        }

        match self.config.status_line_style {
            StatusLineStyle::Full => self.render_full(),
            StatusLineStyle::Compact => self.render_compact(),
            StatusLineStyle::Minimal => self.render_minimal(),
        }
    }

    /// Render full box-style status line.
    fn render_full(&self) {
        let model_display = self.truncate_model_name();
        let mode_display = format!("{}", self.mode);
        let gpu_display = self
            .gpu_status
            .as_ref()
            .map(|g| {
                if g.using_gpu {
                    format!("{} {}", g.name, "\u{2713}".green())
                } else if g.gpu_type == GpuType::Cpu {
                    "CPU only".to_string()
                } else {
                    format!("{} {}", g.name, "\u{2717}".red())
                }
            })
            .unwrap_or_else(|| "Unknown".to_string());

        // Calculate box width
        let content = format!(
            " Model: {} | Mode: {} | GPU: {} ",
            model_display, mode_display, gpu_display
        );
        let box_width = content.len().max(60);

        // Top border
        println!(
            "{}{}{}",
            "\u{250C}\u{2500} ".bright_cyan(),
            "rigrun".bright_cyan().bold(),
            format!(
                " {}\u{2510}",
                "\u{2500}".repeat(box_width - 10)
            )
            .bright_cyan()
        );

        // Content line
        print!("{}", "\u{2502}".bright_cyan());
        print!(" Model: {}", model_display.bright_white().bold());
        print!(" {} ", "\u{2502}".bright_black());
        print!("Mode: {}", self.format_mode_colored());
        print!(" {} ", "\u{2502}".bright_black());
        print!("GPU: {}", self.format_gpu_colored());

        // Pad to box width
        let current_len = format!(
            " Model: {} | Mode: {} | GPU: {}",
            model_display,
            mode_display,
            gpu_display.trim()
        )
        .len();
        let padding = box_width.saturating_sub(current_len);
        print!("{}", " ".repeat(padding));
        println!("{}", "\u{2502}".bright_cyan());

        // Bottom border
        println!(
            "{}",
            format!(
                "\u{2514}{}\u{2518}",
                "\u{2500}".repeat(box_width)
            )
            .bright_cyan()
        );
    }

    /// Render compact inline status.
    fn render_compact(&self) {
        let model_display = self.truncate_model_name();

        print!("{}", "[".bright_black());
        print!("{}", model_display.bright_white());
        print!("{}", " | ".bright_black());
        print!("{}", self.format_mode_colored());
        print!("{}", " | ".bright_black());
        print!("{}", self.format_gpu_short_colored());
        print!("{}", "]".bright_black());
        print!(" ");
        io::stdout().flush().ok();
    }

    /// Render minimal status indicator.
    fn render_minimal(&self) {
        print!("{}", "[".bright_black());
        print!("{}", self.format_mode_colored());
        print!("{}", "|".bright_black());
        print!("{}", self.format_gpu_short_colored());
        print!("{}", "]".bright_black());
        print!(" ");
        io::stdout().flush().ok();
    }

    /// Format mode with color.
    fn format_mode_colored(&self) -> colored::ColoredString {
        match self.mode {
            OperatingMode::Local => "local".green(),
            OperatingMode::Cloud => "cloud".yellow(),
            OperatingMode::Auto => "auto".cyan(),
            OperatingMode::Hybrid => "hybrid".bright_blue(),
        }
    }

    /// Format GPU status with color (short form).
    fn format_gpu_short_colored(&self) -> colored::ColoredString {
        if let Some(ref gpu) = self.gpu_status {
            if gpu.gpu_type == GpuType::Cpu {
                "CPU".bright_black()
            } else if gpu.using_gpu {
                format!("GPU {}", "\u{2713}").green()
            } else {
                format!("GPU {}", "\u{2717}").red()
            }
            .into()
        } else {
            "?".bright_black()
        }
    }

    /// Format GPU status with color (full form).
    fn format_gpu_colored(&self) -> String {
        if let Some(ref gpu) = self.gpu_status {
            if gpu.gpu_type == GpuType::Cpu {
                "CPU only".bright_black().to_string()
            } else if gpu.using_gpu {
                format!("{} {}", gpu.name, "\u{2713}".green())
            } else {
                format!("{} {}", gpu.name, "\u{2717}".red())
            }
        } else {
            "Unknown".bright_black().to_string()
        }
    }

    /// Render detailed status for /status command.
    pub fn render_detailed_status(&self) {
        println!();
        println!("{}", "Current Status:".bright_cyan().bold());

        // Model
        println!("  Model: {}", self.model.bright_white().bold());

        // Mode
        let mode_detail = if self.auto_routing_enabled {
            format!("{} (auto-routing enabled)", self.mode)
        } else {
            format!("{}", self.mode)
        };
        println!("  Mode: {}", mode_detail.bright_white());

        // GPU
        if let Some(ref gpu) = self.gpu_status {
            let gpu_display = if gpu.gpu_type == GpuType::Cpu {
                "None (CPU mode)".bright_black().to_string()
            } else {
                let status = if gpu.using_gpu {
                    format!("{}", "\u{2713}".green())
                } else {
                    format!("{} (fallback)", "\u{2717}".red())
                };
                format!("{} {}", gpu.name, status)
            };
            println!("  GPU: {}", gpu_display);

            // VRAM
            if let Some(ref vram) = gpu.vram_usage {
                let usage_pct = vram.usage_percent();
                let usage_color = if usage_pct > 90.0 {
                    "red"
                } else if usage_pct > 70.0 {
                    "yellow"
                } else {
                    "green"
                };

                let vram_str = format!(
                    "{:.1}/{} GB ({:.0}%)",
                    vram.used_mb as f64 / 1024.0,
                    vram.total_gb(),
                    usage_pct
                );

                let colored_vram = match usage_color {
                    "red" => vram_str.red(),
                    "yellow" => vram_str.yellow(),
                    _ => vram_str.green(),
                };

                println!("  VRAM: {}", colored_vram);

                if let Some(util) = vram.gpu_utilization {
                    println!("  GPU Utilization: {}%", util);
                }
            }
        }

        // Session info
        if self.stats.session_start.is_some() {
            let time_remaining = self.stats.format_time_remaining();
            println!("  Session: {} remaining", time_remaining.bright_white());
        }

        // Message/token stats
        println!(
            "  Messages: {} in current conversation",
            self.stats.message_count
        );
        if self.stats.tokens_used > 0 {
            println!("  Tokens: ~{} used", self.stats.tokens_used);
        }

        println!();
    }
}

/// Get a prompt string with integrated status for the chat loop.
pub fn get_status_prompt(indicator: &StatusIndicator, session_time: Option<(u64, u64)>) -> String {
    let mut prompt = String::new();

    if indicator.config.show_status_line {
        match indicator.config.status_line_style {
            StatusLineStyle::Full => {
                // Full style renders separately, just return plain prompt
            }
            StatusLineStyle::Compact => {
                let model = if indicator.model.len() > MAX_MODEL_NAME_LEN {
                    format!("{}...", &indicator.model[..MAX_MODEL_NAME_LEN - 3])
                } else {
                    indicator.model.clone()
                };

                let gpu_str = indicator
                    .gpu_status
                    .as_ref()
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

                prompt.push_str(&format!("[{} | {} | {}] ", model, indicator.mode, gpu_str));
            }
            StatusLineStyle::Minimal => {
                let gpu_str = indicator
                    .gpu_status
                    .as_ref()
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

                prompt.push_str(&format!("[{}|{}] ", indicator.mode, gpu_str));
            }
        }
    }

    // Add session time if configured
    if let Some((mins, secs)) = session_time {
        if indicator.config.show_session_time {
            prompt.push_str(&format!("[{}:{:02}] ", mins, secs));
        }
    }

    prompt.push_str("You: ");
    prompt
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_operating_mode_display() {
        assert_eq!(format!("{}", OperatingMode::Local), "local");
        assert_eq!(format!("{}", OperatingMode::Cloud), "cloud");
        assert_eq!(format!("{}", OperatingMode::Auto), "auto");
        assert_eq!(format!("{}", OperatingMode::Hybrid), "hybrid");
    }

    #[test]
    fn test_operating_mode_from_str() {
        assert_eq!(OperatingMode::from_str("local"), Some(OperatingMode::Local));
        assert_eq!(OperatingMode::from_str("CLOUD"), Some(OperatingMode::Cloud));
        assert_eq!(OperatingMode::from_str("Auto"), Some(OperatingMode::Auto));
        assert_eq!(OperatingMode::from_str("invalid"), None);
    }

    #[test]
    fn test_status_line_style_from_str() {
        assert_eq!(
            StatusLineStyle::from_str("full"),
            Some(StatusLineStyle::Full)
        );
        assert_eq!(
            StatusLineStyle::from_str("COMPACT"),
            Some(StatusLineStyle::Compact)
        );
        assert_eq!(
            StatusLineStyle::from_str("minimal"),
            Some(StatusLineStyle::Minimal)
        );
        assert_eq!(StatusLineStyle::from_str("invalid"), None);
    }

    #[test]
    fn test_status_config_default() {
        let config = StatusConfig::default();
        assert!(config.show_status_line);
        assert_eq!(config.status_line_style, StatusLineStyle::Compact);
        assert!(config.show_session_time);
    }

    #[test]
    fn test_status_indicator_new() {
        let config = StatusConfig::default();
        let indicator = StatusIndicator::new(config);
        assert!(indicator.model.is_empty());
        assert_eq!(indicator.mode, OperatingMode::Local);
    }

    #[test]
    fn test_status_indicator_set_model() {
        let config = StatusConfig::default();
        let mut indicator = StatusIndicator::new(config);
        indicator.set_model("qwen2.5-coder:14b");
        assert_eq!(indicator.model(), "qwen2.5-coder:14b");
    }

    #[test]
    fn test_truncate_model_name() {
        let config = StatusConfig::default();
        let mut indicator = StatusIndicator::new(config);

        indicator.set_model("short");
        assert_eq!(indicator.truncate_model_name(), "short");

        indicator.set_model("this-is-a-very-long-model-name-that-should-be-truncated");
        assert!(indicator.truncate_model_name().len() <= MAX_MODEL_NAME_LEN);
        assert!(indicator.truncate_model_name().ends_with("..."));
    }

    #[test]
    fn test_session_stats_time_remaining() {
        let mut stats = SessionStats::default();
        stats.session_timeout_secs = 900;
        stats.session_start = Some(std::time::Instant::now());

        // Should be close to 900 (allowing for test execution time)
        assert!(stats.time_remaining_secs() >= 898);
    }

    #[test]
    fn test_format_time_remaining() {
        let mut stats = SessionStats::default();
        stats.session_timeout_secs = 900;
        stats.session_start = Some(std::time::Instant::now());

        let formatted = stats.format_time_remaining();
        assert!(formatted.contains(":"));
    }
}
