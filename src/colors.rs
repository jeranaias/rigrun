// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! Unified ANSI color code definitions
//!
//! This module provides a single source of truth for all ANSI color codes used
//! throughout rigrun. This ensures consistent styling across the UI while avoiding
//! duplicate color definitions.
//!
//! Color usage conventions:
//! - RED = Errors/failures, critical alerts
//! - GREEN = Success/confirmations, GPU active states
//! - YELLOW = Warnings, timeout alerts, fallbacks
//! - CYAN = Info messages, prompts, branding
//! - BLUE = Alternative info, secondary messages
//! - MAGENTA = Special highlights, compliance features
//! - WHITE = Primary text content
//! - BRIGHT_CYAN = Enhanced branding, boxes, prominent elements

/// Reset all formatting
pub const RESET: &str = "\x1b[0m";

/// Bold text
pub const BOLD: &str = "\x1b[1m";

/// Dimmed/faint text
pub const DIM: &str = "\x1b[2m";

/// Red text (errors, failures, critical alerts)
pub const RED: &str = "\x1b[31m";

/// Green text (success, confirmations, GPU active)
pub const GREEN: &str = "\x1b[32m";

/// Yellow text (warnings, timeout alerts, fallbacks)
pub const YELLOW: &str = "\x1b[33m";

/// Blue text (alternative info, secondary messages)
pub const BLUE: &str = "\x1b[34m";

/// Cyan text (info messages, prompts, branding)
pub const CYAN: &str = "\x1b[36m";

/// White text (primary text content)
pub const WHITE: &str = "\x1b[37m";

/// Bright cyan text (enhanced branding, boxes, prominent elements)
pub const BRIGHT_CYAN: &str = "\x1b[96m";

/// Magenta text (special highlights, compliance features)
pub const MAGENTA: &str = "\x1b[35m";

/// Bright black (gray) for subtle secondary text
pub const GRAY: &str = "\x1b[90m";

// ============================================================================
// UX 2.0 Design System
// ============================================================================

/// Box drawing characters for visual hierarchy
pub mod box_chars {
    pub const HORIZONTAL: char = '─';
    pub const VERTICAL: char = '│';
    pub const TOP_LEFT: char = '┌';
    pub const TOP_RIGHT: char = '┐';
    pub const BOTTOM_LEFT: char = '└';
    pub const BOTTOM_RIGHT: char = '┘';
    pub const T_DOWN: char = '┬';
    pub const T_UP: char = '┴';
    pub const T_RIGHT: char = '├';
    pub const T_LEFT: char = '┤';
    pub const CROSS: char = '┼';
    pub const HEAVY_HORIZONTAL: char = '━';
}

/// Symbols for status and feedback (Unicode only, no emoji)
pub mod symbols {
    pub const SUCCESS: &str = "[OK]";
    pub const ERROR: &str = "[X]";
    pub const WARNING: &str = "[!]";
    pub const INFO: &str = "[i]";
    pub const THINKING: &str = "::";  // Represents reasoning/processing
    pub const ARROW: &str = "->";
    pub const BULLET: &str = "*";
    pub const PROGRESS_FULL: &str = "#";
    pub const PROGRESS_EMPTY: &str = "-";
    pub const PROGRESS_PARTIAL: &str = "=";
}

/// Animated thinking dots frames
pub const THINKING_FRAMES: &[&str] = &[
    "·  ", "·· ", "···", " ··", "  ·", "   ",
];

/// Render a separator line
pub fn separator(width: usize) -> String {
    format!("{}{}{}", DIM, box_chars::HORIZONTAL.to_string().repeat(width), RESET)
}

/// Render a context/progress bar (uses Unicode block chars for cleaner look)
pub fn progress_bar(percent: usize, width: usize) -> String {
    let filled = (percent * width) / 100;
    let empty = width.saturating_sub(filled);

    let color = if percent >= 90 {
        RED
    } else if percent >= 75 {
        YELLOW
    } else {
        CYAN
    };

    // Use actual Unicode blocks for visual progress bars
    format!("{}{}{}{} {}%{}",
        color,
        "█".repeat(filled),
        "░".repeat(empty),
        RESET,
        percent,
        RESET
    )
}

/// Format a boxed message (error, warning, info)
pub fn boxed_message(title: &str, content: &str, border_color: &str) -> String {
    let width = 50;
    let title_line = format!("{} {} {}",
        box_chars::TOP_LEFT,
        title,
        box_chars::HORIZONTAL.to_string().repeat(width - title.len() - 4)
    );

    format!("{}{}{}{}\n{}{}  {}{}\n{}{}{}{}",
        border_color, title_line, box_chars::TOP_RIGHT, RESET,
        border_color, box_chars::VERTICAL, content, RESET,
        border_color, box_chars::BOTTOM_LEFT,
        box_chars::HORIZONTAL.to_string().repeat(width - 2),
        box_chars::BOTTOM_RIGHT
    )
}
