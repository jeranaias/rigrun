// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// CUI Classification UI Markers for DoD IL5 Compliance
// Per DoDI 5200.48 and 32 CFR Part 2002

use std::fmt;

/// Classification levels for information marking
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Default)]
pub enum ClassificationLevel {
    /// Unclassified - no special handling required
    #[default]
    Unclassified = 0,
    /// Controlled Unclassified Information - requires CUI markings
    Cui = 1,
    /// CUI with specified category
    CuiSpecified = 2,
}

impl fmt::Display for ClassificationLevel {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ClassificationLevel::Unclassified => write!(f, "UNCLASSIFIED"),
            ClassificationLevel::Cui => write!(f, "CUI"),
            ClassificationLevel::CuiSpecified => write!(f, "CUI//SP"),
        }
    }
}

/// CUI Designation Indicator block (required on first page/screen)
#[derive(Debug, Clone)]
pub struct CuiDesignation {
    /// Organization controlling the CUI
    pub controlled_by: String,
    /// CUI category (e.g., "CTI" for Controlled Technical Information)
    pub category: String,
    /// Distribution/dissemination control marking
    pub distribution: String,
}

impl Default for CuiDesignation {
    fn default() -> Self {
        Self {
            controlled_by: "Department of War".to_string(),
            category: "CTI".to_string(), // Controlled Technical Information
            distribution: "FEDCON".to_string(), // Federal Contractors
        }
    }
}

impl fmt::Display for CuiDesignation {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        writeln!(f, "Controlled by: {}", self.controlled_by)?;
        writeln!(f, "CUI Category: {}", self.category)?;
        write!(f, "Distribution: {}", self.distribution)
    }
}

/// Configuration for classification UI markers
#[derive(Debug, Clone)]
pub struct ClassificationConfig {
    /// Current classification level
    pub level: ClassificationLevel,
    /// CUI designation info (if level is CUI)
    pub designation: Option<CuiDesignation>,
    /// Whether to show banners
    pub show_banners: bool,
    /// Whether to include banners in API responses
    pub include_in_api: bool,
    /// Banner color (ANSI code)
    pub banner_color: &'static str,
}

impl Default for ClassificationConfig {
    fn default() -> Self {
        Self {
            level: ClassificationLevel::Cui, // Default to CUI for IL5
            designation: Some(CuiDesignation::default()),
            show_banners: true,
            include_in_api: true,
            banner_color: "\x1b[33m", // Yellow for CUI
        }
    }
}

// ANSI color codes
const RESET: &str = "\x1b[0m";
const BOLD: &str = "\x1b[1m";
const YELLOW: &str = "\x1b[33m";
const GREEN: &str = "\x1b[32m";

/// Get the appropriate color for a classification level
fn level_color(level: ClassificationLevel) -> &'static str {
    match level {
        ClassificationLevel::Unclassified => GREEN,
        ClassificationLevel::Cui | ClassificationLevel::CuiSpecified => YELLOW,
    }
}

/// Render the top classification banner
pub fn render_top_banner(config: &ClassificationConfig) -> String {
    if !config.show_banners {
        return String::new();
    }

    let color = level_color(config.level);
    let level_str = config.level.to_string();
    let width = 60;
    let _padding = (width - level_str.len()) / 2;

    format!(
        "{color}{BOLD}{:=^width$}{RESET}\n{color}{BOLD}{:^width$}{RESET}\n{color}{BOLD}{:=^width$}{RESET}",
        "",
        level_str,
        "",
        width = width
    )
}

/// Render the bottom classification banner
pub fn render_bottom_banner(config: &ClassificationConfig) -> String {
    if !config.show_banners {
        return String::new();
    }

    let color = level_color(config.level);
    let level_str = config.level.to_string();
    let width = 60;

    format!(
        "{color}{BOLD}{:=^width$}{RESET}\n{color}{BOLD}{:^width$}{RESET}\n{color}{BOLD}{:=^width$}{RESET}",
        "",
        level_str,
        "",
        width = width
    )
}

/// Render the CUI designation indicator block (first screen only)
pub fn render_designation_block(config: &ClassificationConfig) -> String {
    if !config.show_banners {
        return String::new();
    }

    match &config.designation {
        Some(designation) => {
            let color = level_color(config.level);
            format!(
                "{color}┌────────────────────────────────────────────────────────┐{RESET}\n\
                 {color}│ {BOLD}CUI DESIGNATION INDICATOR{RESET}{color}                              │{RESET}\n\
                 {color}├────────────────────────────────────────────────────────┤{RESET}\n\
                 {color}│ Controlled by: {:<40} │{RESET}\n\
                 {color}│ CUI Category:  {:<40} │{RESET}\n\
                 {color}│ Distribution:  {:<40} │{RESET}\n\
                 {color}└────────────────────────────────────────────────────────┘{RESET}",
                designation.controlled_by,
                designation.category,
                designation.distribution,
            )
        }
        None => String::new(),
    }
}

/// Wrap content with classification banners (top and bottom)
pub fn wrap_with_banners(content: &str, config: &ClassificationConfig) -> String {
    if !config.show_banners {
        return content.to_string();
    }

    let top = render_top_banner(config);
    let bottom = render_bottom_banner(config);

    format!("{top}\n\n{content}\n\n{bottom}")
}

/// Wrap content with full classification markings (designation + banners)
pub fn wrap_with_full_markings(content: &str, config: &ClassificationConfig) -> String {
    if !config.show_banners {
        return content.to_string();
    }

    let top = render_top_banner(config);
    let designation = render_designation_block(config);
    let bottom = render_bottom_banner(config);

    if designation.is_empty() {
        format!("{top}\n\n{content}\n\n{bottom}")
    } else {
        format!("{top}\n\n{designation}\n\n{content}\n\n{bottom}")
    }
}

/// Get classification header for API responses
pub fn get_api_headers(config: &ClassificationConfig) -> Vec<(&'static str, String)> {
    if !config.include_in_api {
        return Vec::new();
    }

    let mut headers = vec![
        ("X-Classification-Level", config.level.to_string()),
    ];

    if let Some(ref designation) = config.designation {
        headers.push(("X-CUI-Controlled-By", designation.controlled_by.clone()));
        headers.push(("X-CUI-Category", designation.category.clone()));
        headers.push(("X-CUI-Distribution", designation.distribution.clone()));
    }

    headers
}

/// Render a simple inline classification marker
pub fn inline_marker(config: &ClassificationConfig) -> String {
    if !config.show_banners {
        return String::new();
    }

    let color = level_color(config.level);
    format!("{color}{BOLD}[{}]{RESET}", config.level)
}

/// Print startup classification info
pub fn print_startup_classification(config: &ClassificationConfig) {
    if !config.show_banners {
        return;
    }

    println!("{}", render_top_banner(config));
    println!();

    if config.designation.is_some() {
        println!("{}", render_designation_block(config));
        println!();
    }
}

/// Print shutdown classification banner
pub fn print_shutdown_classification(config: &ClassificationConfig) {
    if !config.show_banners {
        return;
    }

    println!();
    println!("{}", render_bottom_banner(config));
}

/// Create a ClassificationConfig from environment/config
pub fn load_classification_config() -> ClassificationConfig {
    // Check environment variables for configuration
    let level = match std::env::var("RIGRUN_CLASSIFICATION_LEVEL")
        .unwrap_or_default()
        .to_uppercase()
        .as_str()
    {
        "UNCLASSIFIED" => ClassificationLevel::Unclassified,
        "CUI" => ClassificationLevel::Cui,
        "CUI_SPECIFIED" | "CUI//SP" => ClassificationLevel::CuiSpecified,
        _ => ClassificationLevel::Cui, // Default to CUI for IL5
    };

    let show_banners = std::env::var("RIGRUN_SHOW_CLASSIFICATION_BANNERS")
        .map(|v| v != "0" && v.to_lowercase() != "false")
        .unwrap_or(true); // Default to showing banners

    let designation = if level != ClassificationLevel::Unclassified {
        Some(CuiDesignation {
            controlled_by: std::env::var("RIGRUN_CUI_CONTROLLED_BY")
                .unwrap_or_else(|_| "Department of War".to_string()),
            category: std::env::var("RIGRUN_CUI_CATEGORY")
                .unwrap_or_else(|_| "CTI".to_string()),
            distribution: std::env::var("RIGRUN_CUI_DISTRIBUTION")
                .unwrap_or_else(|_| "FEDCON".to_string()),
        })
    } else {
        None
    };

    ClassificationConfig {
        level,
        designation,
        show_banners,
        include_in_api: true,
        banner_color: level_color(level),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_classification_level_display() {
        assert_eq!(ClassificationLevel::Unclassified.to_string(), "UNCLASSIFIED");
        assert_eq!(ClassificationLevel::Cui.to_string(), "CUI");
        assert_eq!(ClassificationLevel::CuiSpecified.to_string(), "CUI//SP");
    }

    #[test]
    fn test_default_config_is_cui() {
        let config = ClassificationConfig::default();
        assert_eq!(config.level, ClassificationLevel::Cui);
        assert!(config.show_banners);
        assert!(config.designation.is_some());
    }

    #[test]
    fn test_banners_disabled() {
        let config = ClassificationConfig {
            show_banners: false,
            ..Default::default()
        };

        assert!(render_top_banner(&config).is_empty());
        assert!(render_bottom_banner(&config).is_empty());
        assert!(render_designation_block(&config).is_empty());
    }

    #[test]
    fn test_wrap_with_banners() {
        let config = ClassificationConfig::default();
        let content = "Test content";
        let wrapped = wrap_with_banners(content, &config);

        assert!(wrapped.contains("CUI"));
        assert!(wrapped.contains(content));
    }

    #[test]
    fn test_api_headers() {
        let config = ClassificationConfig::default();
        let headers = get_api_headers(&config);

        assert!(!headers.is_empty());
        assert!(headers.iter().any(|(k, _)| *k == "X-Classification-Level"));
    }

    #[test]
    fn test_inline_marker() {
        let config = ClassificationConfig::default();
        let marker = inline_marker(&config);

        assert!(marker.contains("CUI"));
    }
}
