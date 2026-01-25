// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

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
    /// Fixed position status bar at top of terminal
    /// ```text
    /// +---------------------------------------------------------------------+
    /// | qwen2.5-coder:7b | [LOCAL] | GPU active | 14:32 remaining | /help   |
    /// +---------------------------------------------------------------------+
    /// ```
    /// Uses ANSI escape codes to render at fixed position without scrolling
    Fixed,
    /// Status bar is hidden
    Off,
}

impl StatusLineStyle {
    /// Parse style from string.
    pub fn from_str(s: &str) -> Option<Self> {
        match s.to_lowercase().as_str() {
            "full" => Some(Self::Full),
            "compact" => Some(Self::Compact),
            "minimal" => Some(Self::Minimal),
            "fixed" => Some(Self::Fixed),
            "off" | "none" | "hidden" => Some(Self::Off),
            _ => None,
        }
    }

    /// Get the next style in the cycle (for toggling)
    pub fn next(&self) -> Self {
        match self {
            Self::Fixed => Self::Compact,
            Self::Compact => Self::Minimal,
            Self::Minimal => Self::Full,
            Self::Full => Self::Off,
            Self::Off => Self::Fixed,
        }
    }

    /// Get a human-readable description of the style
    pub fn description(&self) -> &'static str {
        match self {
            Self::Fixed => "Fixed bar at top of terminal (updates in place)",
            Self::Compact => "Compact inline status in prompt",
            Self::Minimal => "Minimal inline status",
            Self::Full => "Full box-style status line",
            Self::Off => "Status bar hidden",
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
            StatusLineStyle::Fixed => {
                // Fixed rendering is handled separately via render_fixed_status_bar
                // to avoid duplicating the status bar display
            }
            StatusLineStyle::Off => {
                // No rendering when status bar is off
            }
        }
    }

    /// Render full box-style status line.
    fn render_full(&self) {
        let model_display = self.truncate_model_name();
        let mode_display = format!("{}", self.mode);
        // GPU display with text label for accessibility (WCAG 1.4.1)
        let gpu_display = self
            .gpu_status
            .as_ref()
            .map(|g| {
                if g.using_gpu {
                    format!("{} {} (active)", g.name, "\u{2713}".green())
                } else if g.gpu_type == GpuType::Cpu {
                    "CPU only".to_string()
                } else {
                    format!("{} {} (fallback)", g.name, "\u{2193}".yellow())
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

    /// Render fixed status bar at top of terminal.
    /// Uses ANSI escape codes to position at line 1 without scrolling.
    ///
    /// # Arguments
    /// * `session_time` - Optional (minutes, seconds) remaining in session
    ///
    /// # ANSI Escape Codes Used
    /// - `\x1b[s` - Save cursor position
    /// - `\x1b[u` - Restore cursor position
    /// - `\x1b[1;1H` - Move to row 1, col 1
    /// - `\x1b[K` - Clear to end of line
    /// - `\x1b[2K` - Clear entire line
    pub fn render_fixed_status_bar(&self, session_time: Option<(u64, u64)>) {
        if !self.config.show_status_line {
            return;
        }

        // Only render for Fixed style
        if self.config.status_line_style != StatusLineStyle::Fixed {
            return;
        }

        // Get terminal width, default to 80 if unable to detect
        let term_width = crossterm::terminal::size()
            .map(|(w, _)| w as usize)
            .unwrap_or(80);

        // Build content sections
        let model_display = self.truncate_model_name();

        // Mode display
        let mode_str = match self.mode {
            OperatingMode::Local => "[LOCAL]",
            OperatingMode::Cloud => "[CLOUD]",
            OperatingMode::Auto => "[AUTO]",
            OperatingMode::Hybrid => "[HYBRID]",
        };

        // GPU status
        let gpu_str = self
            .gpu_status
            .as_ref()
            .map(|g| {
                if g.gpu_type == GpuType::Cpu {
                    "CPU"
                } else if g.using_gpu {
                    "GPU\u{2713}"
                } else {
                    "CPU\u{2193}"
                }
            })
            .unwrap_or("?");

        // Session time
        let time_str = session_time
            .map(|(mins, secs)| format!("{}:{:02} remaining", mins, secs))
            .unwrap_or_else(|| "".to_string());

        // Help hint
        let help_hint = "/help";

        // Calculate content width (without colors)
        // Format: | model | mode | gpu | time | /help |
        let content_parts = [
            &model_display,
            mode_str,
            gpu_str,
            &time_str,
            help_hint,
        ];
        let separators = " \u{2502} "; // " | "
        let content_width: usize = content_parts.iter().map(|s| s.len()).sum::<usize>()
            + (separators.len() * (content_parts.len() - 1))
            + 4; // Borders and padding

        // Ensure we don't exceed terminal width
        let bar_width = term_width.min(content_width.max(60));

        // ANSI codes
        const SAVE_CURSOR: &str = "\x1b[s";
        const RESTORE_CURSOR: &str = "\x1b[u";
        const MOVE_TO_TOP: &str = "\x1b[1;1H";
        const CLEAR_LINE: &str = "\x1b[2K";

        // Save cursor position
        print!("{}", SAVE_CURSOR);

        // Move to top of terminal and clear the lines we'll use
        print!("{}", MOVE_TO_TOP);
        print!("{}", CLEAR_LINE);

        // Draw top border
        print!(
            "{}",
            format!(
                "\u{250C}{}\u{2510}",
                "\u{2500}".repeat(bar_width.saturating_sub(2))
            )
            .bright_cyan()
        );

        // Move to next line
        print!("\x1b[2;1H");
        print!("{}", CLEAR_LINE);

        // Draw content line
        print!("{}", "\u{2502}".bright_cyan());
        print!(" {}", model_display.bright_white().bold());
        print!(" {} ", "\u{2502}".bright_black());
        print!("{}", self.format_mode_colored());
        print!(" {} ", "\u{2502}".bright_black());
        print!("{}", self.format_gpu_short_colored());

        if !time_str.is_empty() {
            print!(" {} ", "\u{2502}".bright_black());
            // Color time based on urgency
            if let Some((mins, _)) = session_time {
                if mins < 2 {
                    print!("{}", time_str.red().bold());
                } else if mins < 5 {
                    print!("{}", time_str.yellow());
                } else {
                    print!("{}", time_str.bright_black());
                }
            }
        }

        print!(" {} ", "\u{2502}".bright_black());
        print!("{}", help_hint.cyan());

        // Pad and close the box
        let content_so_far = format!(
            " {} | {} | {} | {} | {} ",
            model_display, mode_str, gpu_str, time_str, help_hint
        );
        let padding = bar_width.saturating_sub(content_so_far.len() + 2);
        print!("{}", " ".repeat(padding));
        print!("{}", "\u{2502}".bright_cyan());

        // Move to next line
        print!("\x1b[3;1H");
        print!("{}", CLEAR_LINE);

        // Draw bottom border
        print!(
            "{}",
            format!(
                "\u{2514}{}\u{2518}",
                "\u{2500}".repeat(bar_width.saturating_sub(2))
            )
            .bright_cyan()
        );

        // Restore cursor position
        print!("{}", RESTORE_CURSOR);

        io::stdout().flush().ok();
    }

    /// Clear the fixed status bar (call when switching away from fixed mode)
    pub fn clear_fixed_status_bar(&self) {
        const SAVE_CURSOR: &str = "\x1b[s";
        const RESTORE_CURSOR: &str = "\x1b[u";
        const MOVE_TO_TOP: &str = "\x1b[1;1H";
        const CLEAR_LINE: &str = "\x1b[2K";

        print!("{}", SAVE_CURSOR);
        print!("{}", MOVE_TO_TOP);
        print!("{}", CLEAR_LINE);
        print!("\x1b[2;1H");
        print!("{}", CLEAR_LINE);
        print!("\x1b[3;1H");
        print!("{}", CLEAR_LINE);
        print!("{}", RESTORE_CURSOR);
        io::stdout().flush().ok();
    }

    /// Check if the current style is Fixed
    pub fn is_fixed_style(&self) -> bool {
        self.config.status_line_style == StatusLineStyle::Fixed
    }

    /// Set the status line style
    pub fn set_style(&mut self, style: StatusLineStyle) {
        // If switching away from fixed mode, clear the fixed bar
        if self.config.status_line_style == StatusLineStyle::Fixed && style != StatusLineStyle::Fixed {
            self.clear_fixed_status_bar();
        }
        self.config.status_line_style = style;
        // Update show_status_line based on style
        self.config.show_status_line = style != StatusLineStyle::Off;
    }

    /// Get the current style
    pub fn style(&self) -> StatusLineStyle {
        self.config.status_line_style
    }

    /// Format mode with color and text label for accessibility (WCAG 1.4.1).
    fn format_mode_colored(&self) -> colored::ColoredString {
        match self.mode {
            OperatingMode::Local => "[LOCAL]".green(),
            OperatingMode::Cloud => "[CLOUD]".yellow(),
            OperatingMode::Auto => "[AUTO]".cyan(),
            OperatingMode::Hybrid => "[HYBRID]".bright_blue(),
        }
    }

    /// Format GPU status with color and text label for accessibility (WCAG 1.4.1).
    fn format_gpu_short_colored(&self) -> colored::ColoredString {
        if let Some(ref gpu) = self.gpu_status {
            if gpu.gpu_type == GpuType::Cpu {
                "CPU".bright_black()
            } else if gpu.using_gpu {
                "GPU\u{2713}".green() // GPU checkmark - GPU active
            } else {
                "CPU\u{2193}".yellow() // CPU with down arrow - fallback to CPU
            }
            .into()
        } else {
            "?".bright_black()
        }
    }

    /// Format GPU status with color and text label for accessibility (WCAG 1.4.1).
    fn format_gpu_colored(&self) -> String {
        if let Some(ref gpu) = self.gpu_status {
            if gpu.gpu_type == GpuType::Cpu {
                "CPU only".bright_black().to_string()
            } else if gpu.using_gpu {
                format!("{} {} (active)", gpu.name, "\u{2713}".green())
            } else {
                format!("{} {} (fallback to CPU)", gpu.name, "\u{2193}".yellow())
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

        // GPU - with text labels for accessibility (WCAG 1.4.1)
        if let Some(ref gpu) = self.gpu_status {
            let gpu_display = if gpu.gpu_type == GpuType::Cpu {
                "None (CPU mode)".bright_black().to_string()
            } else {
                let status = if gpu.using_gpu {
                    format!("{} (active)", "\u{2713}".green())
                } else {
                    format!("{} (fallback to CPU)", "\u{2193}".yellow())
                };
                format!("{} {}", gpu.name, status)
            };
            println!("  GPU: {}", gpu_display);

            // VRAM - with text labels for accessibility (WCAG 1.4.1)
            if let Some(ref vram) = gpu.vram_usage {
                let usage_pct = vram.usage_percent();
                let (usage_color, status_text) = if usage_pct > 90.0 {
                    ("red", " ✗")
                } else if usage_pct > 70.0 {
                    ("yellow", " ⚠")
                } else {
                    ("green", " ✓")
                };

                let vram_str = format!(
                    "{:.1}/{} GB ({:.0}%){}",
                    vram.used_mb as f64 / 1024.0,
                    vram.total_gb(),
                    usage_pct,
                    status_text
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
/// Uses text labels for accessibility (WCAG 1.4.1).
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

                // GPU status with text label for accessibility
                let gpu_str = indicator
                    .gpu_status
                    .as_ref()
                    .map(|g| {
                        if g.gpu_type == GpuType::Cpu {
                            "CPU"
                        } else if g.using_gpu {
                            "GPU\u{2713}" // GPU active
                        } else {
                            "CPU\u{2193}" // Fallback to CPU
                        }
                    })
                    .unwrap_or("?");

                // Mode with explicit text label
                let mode_str = match indicator.mode {
                    OperatingMode::Local => "[LOCAL]",
                    OperatingMode::Cloud => "[CLOUD]",
                    OperatingMode::Auto => "[AUTO]",
                    OperatingMode::Hybrid => "[HYBRID]",
                };

                prompt.push_str(&format!("[{} | {} | {}] ", model, mode_str, gpu_str));
            }
            StatusLineStyle::Minimal => {
                // GPU status with text label for accessibility
                let gpu_str = indicator
                    .gpu_status
                    .as_ref()
                    .map(|g| {
                        if g.gpu_type == GpuType::Cpu {
                            "CPU"
                        } else if g.using_gpu {
                            "GPU\u{2713}" // GPU active
                        } else {
                            "CPU\u{2193}" // Fallback to CPU
                        }
                    })
                    .unwrap_or("?");

                // Mode with explicit text label
                let mode_str = match indicator.mode {
                    OperatingMode::Local => "[LOCAL]",
                    OperatingMode::Cloud => "[CLOUD]",
                    OperatingMode::Auto => "[AUTO]",
                    OperatingMode::Hybrid => "[HYBRID]",
                };

                prompt.push_str(&format!("[{}|{}] ", mode_str, gpu_str));
            }
            StatusLineStyle::Fixed => {
                // Fixed style - status bar is rendered separately at top of terminal
                // No inline status in the prompt
            }
            StatusLineStyle::Off => {
                // No status display
            }
        }
    }

    // Add session time if configured (with urgency icon when low)
    if let Some((mins, secs)) = session_time {
        if indicator.config.show_session_time {
            let remaining = mins * 60 + secs;
            if remaining <= 120 {
                prompt.push_str(&format!("\u{23F0}[{}:{:02}] ", mins, secs)); // Clock icon for urgency
            } else {
                prompt.push_str(&format!("[{}:{:02}] ", mins, secs));
            }
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
        assert_eq!(
            StatusLineStyle::from_str("fixed"),
            Some(StatusLineStyle::Fixed)
        );
        assert_eq!(
            StatusLineStyle::from_str("off"),
            Some(StatusLineStyle::Off)
        );
        assert_eq!(
            StatusLineStyle::from_str("none"),
            Some(StatusLineStyle::Off)
        );
        assert_eq!(
            StatusLineStyle::from_str("hidden"),
            Some(StatusLineStyle::Off)
        );
        assert_eq!(StatusLineStyle::from_str("invalid"), None);
    }

    #[test]
    fn test_status_line_style_next() {
        // Test the cycling behavior
        assert_eq!(StatusLineStyle::Fixed.next(), StatusLineStyle::Compact);
        assert_eq!(StatusLineStyle::Compact.next(), StatusLineStyle::Minimal);
        assert_eq!(StatusLineStyle::Minimal.next(), StatusLineStyle::Full);
        assert_eq!(StatusLineStyle::Full.next(), StatusLineStyle::Off);
        assert_eq!(StatusLineStyle::Off.next(), StatusLineStyle::Fixed);
    }

    #[test]
    fn test_status_line_style_description() {
        assert!(StatusLineStyle::Fixed.description().contains("Fixed"));
        assert!(StatusLineStyle::Compact.description().contains("Compact"));
        assert!(StatusLineStyle::Minimal.description().contains("Minimal"));
        assert!(StatusLineStyle::Full.description().contains("Full"));
        assert!(StatusLineStyle::Off.description().contains("hidden"));
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
