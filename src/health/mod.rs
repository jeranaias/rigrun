// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

//! Health Check Module for rigrun
//!
//! Provides comprehensive system health diagnostics including:
//! - Ollama server status and version checking
//! - Model availability and loading status
//! - GPU detection and utilization
//! - Configuration validation
//! - Disk space checking
//! - Network connectivity (for hybrid mode)
//!
//! # Example
//!
//! ```no_run
//! use rigrun::health::{HealthChecker, run_health_check, Severity};
//!
//! let checker = HealthChecker::new();
//! let status = checker.run_full_check();
//!
//! if status.has_critical_issues() {
//!     eprintln!("Critical issues found! Cannot start.");
//!     for issue in status.critical_issues() {
//!         eprintln!("  - {}", issue.message);
//!     }
//! }
//! ```

use std::path::PathBuf;
use std::time::Duration;
use serde::{Deserialize, Serialize};

use crate::detect::{
    detect_gpu, get_gpu_memory_usage, get_ollama_loaded_models, is_model_available,
    list_ollama_models, recommend_model, GpuInfo, GpuType, ProcessorType,
    check_rocm_installed, check_nvidia_smi_available, get_nvidia_driver_version,
    detect_amd_architecture, AmdArchitecture, get_hsa_override_version,
};
use crate::local::OllamaClient;

/// Minimum supported Ollama version (major.minor)
const MIN_OLLAMA_VERSION: (u32, u32) = (0, 3);

/// Recommended Ollama version (major.minor.patch)
const RECOMMENDED_OLLAMA_VERSION: &str = "0.5.0";

/// Minimum disk space required (in GB)
const MIN_DISK_SPACE_GB: u64 = 10;

/// Warning threshold for VRAM usage (percentage)
const VRAM_WARNING_THRESHOLD: f64 = 85.0;

/// Critical threshold for VRAM usage (percentage)
const VRAM_CRITICAL_THRESHOLD: f64 = 95.0;

/// Severity level for health issues
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
pub enum Severity {
    /// Informational - not a problem, just FYI
    Info,
    /// Warning - system will work but may have degraded performance
    Warning,
    /// Critical - system cannot function properly
    Critical,
}

impl std::fmt::Display for Severity {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Severity::Info => write!(f, "INFO"),
            Severity::Warning => write!(f, "WARN"),
            Severity::Critical => write!(f, "CRITICAL"),
        }
    }
}

/// Represents a health issue with actionable fix instructions
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HealthIssue {
    /// Severity level of the issue
    pub severity: Severity,
    /// Component affected (e.g., "Ollama", "GPU", "Config")
    pub component: String,
    /// Human-readable description of the issue
    pub message: String,
    /// Actionable fix instruction
    pub fix: String,
    /// Optional command to run for auto-fix
    pub auto_fix_command: Option<String>,
}

impl HealthIssue {
    /// Creates a new health issue
    pub fn new(severity: Severity, component: &str, message: &str, fix: &str) -> Self {
        Self {
            severity,
            component: component.to_string(),
            message: message.to_string(),
            fix: fix.to_string(),
            auto_fix_command: None,
        }
    }

    /// Creates a health issue with an auto-fix command
    pub fn with_auto_fix(mut self, command: &str) -> Self {
        self.auto_fix_command = Some(command.to_string());
        self
    }

    /// Creates a critical issue
    pub fn critical(component: &str, message: &str, fix: &str) -> Self {
        Self::new(Severity::Critical, component, message, fix)
    }

    /// Creates a warning issue
    pub fn warning(component: &str, message: &str, fix: &str) -> Self {
        Self::new(Severity::Warning, component, message, fix)
    }

    /// Creates an info issue
    pub fn info(component: &str, message: &str, fix: &str) -> Self {
        Self::new(Severity::Info, component, message, fix)
    }
}

impl std::fmt::Display for HealthIssue {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let icon = match self.severity {
            Severity::Critical => "[X]",
            Severity::Warning => "[!]",
            Severity::Info => "[i]",
        };
        write!(f, "{} {}: {}", icon, self.component, self.message)
    }
}

/// Comprehensive health status of the system
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HealthStatus {
    /// Whether Ollama server is running and reachable
    pub ollama_running: bool,
    /// Ollama version if available
    pub ollama_version: Option<String>,
    /// Whether Ollama version is compatible
    pub ollama_version_ok: bool,
    /// Whether a model is currently loaded in Ollama
    pub model_loaded: bool,
    /// Name of the configured/default model
    pub model_name: Option<String>,
    /// Whether the configured model is downloaded
    pub model_downloaded: bool,
    /// Whether a GPU was detected
    pub gpu_detected: bool,
    /// GPU information if detected
    pub gpu_info: Option<GpuInfoStatus>,
    /// Whether GPU is being used (not CPU fallback)
    pub gpu_in_use: bool,
    /// Available VRAM in MB (if GPU detected)
    pub vram_available_mb: Option<u64>,
    /// Used VRAM in MB (if GPU detected)
    pub vram_used_mb: Option<u64>,
    /// VRAM usage percentage
    pub vram_usage_percent: Option<f64>,
    /// Whether configuration is valid
    pub config_valid: bool,
    /// Configuration issues (if any)
    pub config_issues: Vec<String>,
    /// Available disk space in GB
    pub disk_space_gb: Option<u64>,
    /// Whether network is available (for hybrid mode)
    pub network_available: Option<bool>,
    /// All detected health issues
    pub issues: Vec<HealthIssue>,
}

/// GPU information for health status
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GpuInfoStatus {
    /// GPU name
    pub name: String,
    /// GPU type (NVIDIA, AMD, etc.)
    pub gpu_type: String,
    /// VRAM in GB
    pub vram_gb: u32,
    /// Driver version if available
    pub driver: Option<String>,
}

impl From<&GpuInfo> for GpuInfoStatus {
    fn from(info: &GpuInfo) -> Self {
        Self {
            name: info.name.clone(),
            gpu_type: format!("{}", info.gpu_type),
            vram_gb: info.vram_gb,
            driver: info.driver.clone(),
        }
    }
}

impl Default for HealthStatus {
    fn default() -> Self {
        Self {
            ollama_running: false,
            ollama_version: None,
            ollama_version_ok: false,
            model_loaded: false,
            model_name: None,
            model_downloaded: false,
            gpu_detected: false,
            gpu_info: None,
            gpu_in_use: false,
            vram_available_mb: None,
            vram_used_mb: None,
            vram_usage_percent: None,
            config_valid: true,
            config_issues: Vec::new(),
            disk_space_gb: None,
            network_available: None,
            issues: Vec::new(),
        }
    }
}

impl HealthStatus {
    /// Returns true if there are any critical issues
    pub fn has_critical_issues(&self) -> bool {
        self.issues.iter().any(|i| i.severity == Severity::Critical)
    }

    /// Returns true if there are any warnings
    pub fn has_warnings(&self) -> bool {
        self.issues.iter().any(|i| i.severity == Severity::Warning)
    }

    /// Returns only critical issues
    pub fn critical_issues(&self) -> Vec<&HealthIssue> {
        self.issues
            .iter()
            .filter(|i| i.severity == Severity::Critical)
            .collect()
    }

    /// Returns only warning issues
    pub fn warning_issues(&self) -> Vec<&HealthIssue> {
        self.issues
            .iter()
            .filter(|i| i.severity == Severity::Warning)
            .collect()
    }

    /// Returns only info issues
    pub fn info_issues(&self) -> Vec<&HealthIssue> {
        self.issues
            .iter()
            .filter(|i| i.severity == Severity::Info)
            .collect()
    }

    /// Returns issues that can be auto-fixed
    pub fn fixable_issues(&self) -> Vec<&HealthIssue> {
        self.issues
            .iter()
            .filter(|i| i.auto_fix_command.is_some())
            .collect()
    }

    /// Returns the overall health as a simple status string
    pub fn overall_status(&self) -> &'static str {
        if self.has_critical_issues() {
            "CRITICAL"
        } else if self.has_warnings() {
            "WARNING"
        } else {
            "HEALTHY"
        }
    }

    /// Returns count of issues by severity
    pub fn issue_counts(&self) -> (usize, usize, usize) {
        let critical = self.issues.iter().filter(|i| i.severity == Severity::Critical).count();
        let warnings = self.issues.iter().filter(|i| i.severity == Severity::Warning).count();
        let info = self.issues.iter().filter(|i| i.severity == Severity::Info).count();
        (critical, warnings, info)
    }
}

/// Health checker that performs system diagnostics
#[derive(Debug, Clone)]
pub struct HealthChecker {
    /// Ollama client for API checks
    ollama_client: OllamaClient,
    /// Model name to check (optional)
    model_name: Option<String>,
    /// Configuration directory
    config_dir: Option<PathBuf>,
    /// Whether to check network connectivity
    check_network: bool,
}

impl Default for HealthChecker {
    fn default() -> Self {
        Self::new()
    }
}

impl HealthChecker {
    /// Creates a new health checker with default settings
    pub fn new() -> Self {
        Self {
            ollama_client: OllamaClient::new(),
            model_name: None,
            config_dir: dirs::home_dir().map(|h| h.join(".rigrun")),
            check_network: false,
        }
    }

    /// Sets a specific model to check
    pub fn with_model(mut self, model: &str) -> Self {
        self.model_name = Some(model.to_string());
        self
    }

    /// Sets the configuration directory
    pub fn with_config_dir(mut self, dir: PathBuf) -> Self {
        self.config_dir = Some(dir);
        self
    }

    /// Enables network connectivity checking
    pub fn with_network_check(mut self, enabled: bool) -> Self {
        self.check_network = enabled;
        self
    }

    /// Runs a full health check
    pub fn run_full_check(&self) -> HealthStatus {
        let mut status = HealthStatus::default();

        // Check Ollama
        self.check_ollama(&mut status);

        // Check GPU
        self.check_gpu(&mut status);

        // Check model
        self.check_model(&mut status);

        // Check config
        self.check_config(&mut status);

        // Check disk space
        self.check_disk_space(&mut status);

        // Check network (if enabled)
        if self.check_network {
            self.check_network_connectivity(&mut status);
        }

        status
    }

    /// Runs a quick health check (just Ollama and model)
    pub fn run_quick_check(&self) -> HealthStatus {
        let mut status = HealthStatus::default();
        self.check_ollama(&mut status);
        self.check_model(&mut status);
        self.check_gpu(&mut status);
        status
    }

    /// Checks Ollama server status
    fn check_ollama(&self, status: &mut HealthStatus) {
        // Check if Ollama is running
        status.ollama_running = self.ollama_client.check_ollama_running();

        if !status.ollama_running {
            status.issues.push(
                HealthIssue::critical(
                    "Ollama",
                    "Ollama server is not running",
                    "Start Ollama with: ollama serve"
                ).with_auto_fix("ollama serve")
            );
            return;
        }

        // Get Ollama version
        if let Some(version) = get_ollama_version() {
            status.ollama_version = Some(version.clone());
            status.ollama_version_ok = check_ollama_version_compatible(&version);

            if !status.ollama_version_ok {
                status.issues.push(
                    HealthIssue::warning(
                        "Ollama",
                        &format!("Ollama version {} may be outdated (recommended: {}+)", version, RECOMMENDED_OLLAMA_VERSION),
                        "Update Ollama from https://ollama.ai/download"
                    )
                );
            }
        }
    }

    /// Checks GPU status
    fn check_gpu(&self, status: &mut HealthStatus) {
        match detect_gpu() {
            Ok(gpu_info) => {
                if gpu_info.gpu_type == GpuType::Cpu {
                    status.gpu_detected = false;
                    status.issues.push(
                        HealthIssue::warning(
                            "GPU",
                            "No GPU detected - running in CPU-only mode",
                            "Install a supported GPU or check GPU drivers"
                        )
                    );
                } else {
                    status.gpu_detected = true;
                    status.gpu_info = Some(GpuInfoStatus::from(&gpu_info));

                    // Check GPU-specific issues
                    self.check_gpu_specific_issues(&gpu_info, status);

                    // Check VRAM usage
                    if let Some(usage) = get_gpu_memory_usage(Some(&gpu_info)) {
                        status.vram_available_mb = Some(usage.free_mb);
                        status.vram_used_mb = Some(usage.used_mb);
                        status.vram_usage_percent = Some(usage.usage_percent());

                        if usage.usage_percent() >= VRAM_CRITICAL_THRESHOLD {
                            status.issues.push(
                                HealthIssue::critical(
                                    "VRAM",
                                    &format!("VRAM critically low: {:.1}% used ({}/{}MB)",
                                        usage.usage_percent(), usage.used_mb, usage.total_mb),
                                    "Close other GPU applications or use a smaller model"
                                )
                            );
                        } else if usage.usage_percent() >= VRAM_WARNING_THRESHOLD {
                            status.issues.push(
                                HealthIssue::warning(
                                    "VRAM",
                                    &format!("VRAM running high: {:.1}% used ({}/{}MB)",
                                        usage.usage_percent(), usage.used_mb, usage.total_mb),
                                    "Consider using a smaller model if experiencing slowdowns"
                                )
                            );
                        }
                    }

                    // Check if loaded models are using GPU
                    let loaded_models = get_ollama_loaded_models();
                    let any_on_gpu = loaded_models.iter().any(|m| {
                        matches!(m.processor, ProcessorType::Gpu(_))
                            || matches!(m.processor, ProcessorType::Mixed { gpu_percent, .. } if gpu_percent > 0)
                    });

                    let any_on_cpu = loaded_models.iter().any(|m| {
                        matches!(m.processor, ProcessorType::Cpu)
                    });

                    status.gpu_in_use = any_on_gpu;

                    if any_on_cpu && !loaded_models.is_empty() {
                        for model in &loaded_models {
                            if matches!(model.processor, ProcessorType::Cpu) {
                                let suggested = recommend_model(gpu_info.vram_gb);
                                status.issues.push(
                                    HealthIssue::warning(
                                        "GPU",
                                        &format!("Model '{}' is running on CPU - inference will be slow", model.name),
                                        &format!("Consider using a smaller model like '{}'", suggested)
                                    )
                                );
                            }
                        }
                    }
                }
            }
            Err(e) => {
                status.gpu_detected = false;
                status.issues.push(
                    HealthIssue::warning(
                        "GPU",
                        &format!("GPU detection failed: {}", e),
                        "Check GPU drivers are installed correctly"
                    )
                );
            }
        }
    }

    /// Checks GPU-specific issues (AMD RDNA 4, NVIDIA drivers, etc.)
    fn check_gpu_specific_issues(&self, gpu_info: &GpuInfo, status: &mut HealthStatus) {
        match gpu_info.gpu_type {
            GpuType::Nvidia => {
                // Check nvidia-smi availability
                if !check_nvidia_smi_available() {
                    status.issues.push(
                        HealthIssue::critical(
                            "GPU",
                            "NVIDIA GPU detected but nvidia-smi is not available",
                            "Install or update NVIDIA drivers"
                        )
                    );
                } else if let Some(driver) = get_nvidia_driver_version() {
                    // Check driver version
                    if let Some(major_str) = driver.split('.').next() {
                        if let Ok(major) = major_str.parse::<u32>() {
                            if major < 470 {
                                status.issues.push(
                                    HealthIssue::warning(
                                        "GPU",
                                        &format!("NVIDIA driver {} may be outdated", driver),
                                        "Update to driver version 470+ for best performance"
                                    )
                                );
                            }
                        }
                    }
                }
            }
            GpuType::Amd => {
                let arch = detect_amd_architecture(&gpu_info.name);

                // RDNA 4 needs Vulkan backend
                if arch == AmdArchitecture::Rdna4 {
                    // Check if OLLAMA_VULKAN is set
                    if std::env::var("OLLAMA_VULKAN").ok().as_deref() != Some("1") {
                        status.issues.push(
                            HealthIssue::warning(
                                "GPU",
                                &format!("{} (RDNA 4) requires Vulkan backend", gpu_info.name),
                                "Set OLLAMA_VULKAN=1 before starting Ollama"
                            ).with_auto_fix(if cfg!(target_os = "windows") {
                                "setx OLLAMA_VULKAN 1"
                            } else {
                                "export OLLAMA_VULKAN=1"
                            })
                        );
                    }
                } else if !check_rocm_installed() {
                    // Non-RDNA 4 AMD needs ROCm
                    status.issues.push(
                        HealthIssue::warning(
                            "GPU",
                            &format!("{} detected but ROCm/HIP is not installed", gpu_info.name),
                            "Install ROCm (Linux) or HIP SDK (Windows) for GPU acceleration"
                        )
                    );
                }

                // Check if HSA override might help
                if let Some(hsa_version) = get_hsa_override_version(&gpu_info.name) {
                    if std::env::var("HSA_OVERRIDE_GFX_VERSION").is_err() {
                        status.issues.push(
                            HealthIssue::info(
                                "GPU",
                                &format!("AMD GPU may benefit from HSA override"),
                                &format!("Try: HSA_OVERRIDE_GFX_VERSION={}", hsa_version)
                            )
                        );
                    }
                }
            }
            GpuType::Intel => {
                status.issues.push(
                    HealthIssue::info(
                        "GPU",
                        "Intel Arc GPU support is experimental in Ollama",
                        "See https://github.com/ollama/ollama/blob/main/docs/gpu.md"
                    )
                );
            }
            _ => {}
        }
    }

    /// Checks model status
    fn check_model(&self, status: &mut HealthStatus) {
        // Determine which model to check
        let model_to_check = self.model_name.clone().or_else(|| {
            // Try to get from loaded models
            let loaded = get_ollama_loaded_models();
            loaded.first().map(|m| m.name.clone())
        });

        if let Some(model) = model_to_check {
            status.model_name = Some(model.clone());

            // Check if model is downloaded
            status.model_downloaded = is_model_available(&model);

            if !status.model_downloaded {
                status.issues.push(
                    HealthIssue::critical(
                        "Model",
                        &format!("Model '{}' is not downloaded", model),
                        &format!("Download with: ollama pull {}", model)
                    ).with_auto_fix(&format!("ollama pull {}", model))
                );
            }

            // Check if model is loaded
            let loaded = get_ollama_loaded_models();
            status.model_loaded = loaded.iter().any(|m| {
                m.name == model || m.name.starts_with(&format!("{}:", model))
            });
        } else {
            // No model specified, check if any models are available
            let models = list_ollama_models();
            if models.is_empty() {
                status.issues.push(
                    HealthIssue::warning(
                        "Model",
                        "No models downloaded",
                        "Download a model with: ollama pull qwen2.5-coder:7b"
                    ).with_auto_fix("ollama pull qwen2.5-coder:7b")
                );
            }
        }
    }

    /// Checks configuration
    fn check_config(&self, status: &mut HealthStatus) {
        if let Some(ref config_dir) = self.config_dir {
            let config_file = config_dir.join("config.json");

            if !config_file.exists() {
                // Config doesn't exist - not necessarily an error for first run
                status.issues.push(
                    HealthIssue::info(
                        "Config",
                        "No configuration file found",
                        "Run 'rigrun' to create default configuration"
                    )
                );
                return;
            }

            // Try to read and validate config
            match std::fs::read_to_string(&config_file) {
                Ok(content) => {
                    match serde_json::from_str::<serde_json::Value>(&content) {
                        Ok(config) => {
                            // Check for required/recommended fields
                            if config.get("model").is_none() {
                                status.issues.push(
                                    HealthIssue::info(
                                        "Config",
                                        "No default model configured",
                                        "Set with: rigrun config set-model qwen2.5-coder:7b"
                                    )
                                );
                            }

                            // Check OpenRouter key format if present
                            if let Some(key) = config.get("openrouter_key").and_then(|k| k.as_str()) {
                                if !key.starts_with("sk-or-") {
                                    status.config_valid = false;
                                    status.config_issues.push("Invalid OpenRouter key format".to_string());
                                    status.issues.push(
                                        HealthIssue::warning(
                                            "Config",
                                            "OpenRouter API key doesn't start with 'sk-or-'",
                                            "Get a valid key from https://openrouter.ai/keys"
                                        )
                                    );
                                }
                            }
                        }
                        Err(e) => {
                            status.config_valid = false;
                            status.config_issues.push(format!("JSON parse error: {}", e));
                            status.issues.push(
                                HealthIssue::critical(
                                    "Config",
                                    &format!("Configuration file is invalid JSON: {}", e),
                                    "Fix or delete the config file and run 'rigrun' to recreate"
                                )
                            );
                        }
                    }
                }
                Err(e) => {
                    status.config_valid = false;
                    status.config_issues.push(format!("Read error: {}", e));
                    status.issues.push(
                        HealthIssue::warning(
                            "Config",
                            &format!("Could not read configuration: {}", e),
                            "Check file permissions or delete and recreate"
                        )
                    );
                }
            }
        }
    }

    /// Checks disk space
    fn check_disk_space(&self, status: &mut HealthStatus) {
        // Try to get disk space from the home directory
        if let Some(home) = dirs::home_dir() {
            if let Ok(space) = get_available_disk_space(&home) {
                status.disk_space_gb = Some(space);

                if space < MIN_DISK_SPACE_GB {
                    status.issues.push(
                        HealthIssue::warning(
                            "Disk",
                            &format!("Low disk space: {}GB available", space),
                            "Free up disk space for model storage and caching"
                        )
                    );
                }
            }
        }
    }

    /// Checks network connectivity (for hybrid cloud mode)
    fn check_network_connectivity(&self, status: &mut HealthStatus) {
        // Try to connect to OpenRouter API
        let client = reqwest::blocking::Client::builder()
            .timeout(Duration::from_secs(5))
            .build();

        match client {
            Ok(c) => {
                match c.head("https://openrouter.ai/api/v1").send() {
                    Ok(resp) => {
                        status.network_available = Some(resp.status().is_success() || resp.status().as_u16() == 401);
                        if !status.network_available.unwrap_or(false) {
                            status.issues.push(
                                HealthIssue::warning(
                                    "Network",
                                    "Cannot reach OpenRouter API",
                                    "Check internet connection for cloud fallback support"
                                )
                            );
                        }
                    }
                    Err(_) => {
                        status.network_available = Some(false);
                        status.issues.push(
                            HealthIssue::info(
                                "Network",
                                "No network connectivity to cloud APIs",
                                "Local-only mode will be used"
                            )
                        );
                    }
                }
            }
            Err(_) => {
                status.network_available = None;
            }
        }
    }
}

/// Gets the Ollama version from the CLI
fn get_ollama_version() -> Option<String> {
    let output = std::process::Command::new("ollama")
        .arg("--version")
        .output()
        .ok()?;

    if output.status.success() {
        let version_str = String::from_utf8_lossy(&output.stdout);
        // Parse version string - typically "ollama version 0.5.4"
        version_str
            .split_whitespace()
            .last()
            .map(|s| s.trim().to_string())
    } else {
        None
    }
}

/// Checks if an Ollama version string is compatible
fn check_ollama_version_compatible(version: &str) -> bool {
    // Parse version string (e.g., "0.5.4")
    let parts: Vec<&str> = version.split('.').collect();
    if parts.len() < 2 {
        return false;
    }

    let major: u32 = parts[0].parse().unwrap_or(0);
    let minor: u32 = parts[1].parse().unwrap_or(0);

    (major, minor) >= MIN_OLLAMA_VERSION
}

/// Gets available disk space in GB for the given path
fn get_available_disk_space(path: &std::path::Path) -> Result<u64, std::io::Error> {
    #[cfg(target_os = "windows")]
    {
        use std::os::windows::ffi::OsStrExt;
        use std::ffi::OsStr;

        let path_wide: Vec<u16> = OsStr::new(path)
            .encode_wide()
            .chain(std::iter::once(0))
            .collect();

        let mut free_bytes: u64 = 0;
        let mut total_bytes: u64 = 0;
        let mut total_free_bytes: u64 = 0;

        unsafe {
            extern "system" {
                fn GetDiskFreeSpaceExW(
                    lpDirectoryName: *const u16,
                    lpFreeBytesAvailableToCaller: *mut u64,
                    lpTotalNumberOfBytes: *mut u64,
                    lpTotalNumberOfFreeBytes: *mut u64,
                ) -> i32;
            }

            if GetDiskFreeSpaceExW(
                path_wide.as_ptr(),
                &mut free_bytes,
                &mut total_bytes,
                &mut total_free_bytes,
            ) != 0
            {
                Ok(free_bytes / (1024 * 1024 * 1024)) // Convert to GB
            } else {
                Err(std::io::Error::last_os_error())
            }
        }
    }

    #[cfg(not(target_os = "windows"))]
    {
        use std::os::unix::fs::MetadataExt;

        // Use statvfs on Unix-like systems
        let output = std::process::Command::new("df")
            .args(["-B1", path.to_string_lossy().as_ref()])
            .output()?;

        if output.status.success() {
            let stdout = String::from_utf8_lossy(&output.stdout);
            // Parse df output to get available space
            for line in stdout.lines().skip(1) {
                let parts: Vec<&str> = line.split_whitespace().collect();
                if parts.len() >= 4 {
                    if let Ok(avail) = parts[3].parse::<u64>() {
                        return Ok(avail / (1024 * 1024 * 1024));
                    }
                }
            }
        }

        Err(std::io::Error::new(
            std::io::ErrorKind::Other,
            "Could not determine disk space",
        ))
    }
}

/// Convenience function to run a full health check
pub fn run_health_check() -> HealthStatus {
    HealthChecker::new().run_full_check()
}

/// Convenience function to run a quick health check
pub fn run_quick_health_check() -> HealthStatus {
    HealthChecker::new().run_quick_check()
}

/// Attempts to auto-fix issues that have fix commands
pub fn auto_fix_issues(issues: &[&HealthIssue]) -> Vec<(String, Result<(), String>)> {
    let mut results = Vec::new();

    for issue in issues {
        if let Some(ref cmd) = issue.auto_fix_command {
            let result = run_fix_command(cmd);
            results.push((cmd.clone(), result));
        }
    }

    results
}

/// Runs a fix command
fn run_fix_command(cmd: &str) -> Result<(), String> {
    // Split command into program and args
    let parts: Vec<&str> = cmd.split_whitespace().collect();
    if parts.is_empty() {
        return Err("Empty command".to_string());
    }

    let program = parts[0];
    let args = &parts[1..];

    let output = std::process::Command::new(program)
        .args(args)
        .output()
        .map_err(|e| format!("Failed to run '{}': {}", cmd, e))?;

    if output.status.success() {
        Ok(())
    } else {
        let stderr = String::from_utf8_lossy(&output.stderr);
        Err(format!("Command failed: {}", stderr.trim()))
    }
}

/// Formats health status for terminal display
pub fn format_health_status(status: &HealthStatus) -> String {
    let mut output = String::new();

    // Ollama status
    if status.ollama_running {
        let version = status.ollama_version.as_deref().unwrap_or("unknown");
        output.push_str(&format!("[OK] Ollama: Running (v{})\n", version));
    } else {
        output.push_str("[X] Ollama: Not running\n");
    }

    // Model status
    if let Some(ref model) = status.model_name {
        if status.model_loaded {
            output.push_str(&format!("[OK] Model: {} loaded\n", model));
        } else if status.model_downloaded {
            output.push_str(&format!("[OK] Model: {} available (not loaded)\n", model));
        } else {
            output.push_str(&format!("[X] Model: {} not downloaded\n", model));
        }
    } else {
        output.push_str("[!] Model: None configured\n");
    }

    // GPU status
    if let Some(ref gpu) = status.gpu_info {
        if status.gpu_in_use {
            output.push_str(&format!("[OK] GPU: {} ({}GB)\n", gpu.name, gpu.vram_gb));
        } else {
            output.push_str(&format!("[!] GPU: {} (not in use)\n", gpu.name));
        }

        if let (Some(used), Some(total)) = (status.vram_used_mb, status.vram_available_mb.map(|a| a + status.vram_used_mb.unwrap_or(0))) {
            let percent = status.vram_usage_percent.unwrap_or(0.0);
            output.push_str(&format!("    VRAM: {:.1}% used ({}MB/{}MB)\n", percent, used, total));
        }
    } else if !status.gpu_detected {
        output.push_str("[!] GPU: None detected (CPU mode)\n");
    }

    // Config status
    if status.config_valid {
        output.push_str("[OK] Config: Valid\n");
    } else {
        output.push_str("[X] Config: Invalid\n");
    }

    // Disk space
    if let Some(space) = status.disk_space_gb {
        if space >= MIN_DISK_SPACE_GB {
            output.push_str(&format!("[OK] Disk: {}GB available\n", space));
        } else {
            output.push_str(&format!("[!] Disk: {}GB available (low)\n", space));
        }
    }

    // Network
    if let Some(available) = status.network_available {
        if available {
            output.push_str("[OK] Network: Cloud APIs reachable\n");
        } else {
            output.push_str("[!] Network: Cloud APIs unreachable\n");
        }
    }

    // Summary
    let (critical, warnings, _info) = status.issue_counts();
    output.push_str(&format!(
        "\nOverall: {} critical, {} warnings\n",
        critical, warnings
    ));

    // List issues with fixes
    if !status.issues.is_empty() {
        output.push_str("\nIssues:\n");
        for issue in &status.issues {
            let icon = match issue.severity {
                Severity::Critical => "[X]",
                Severity::Warning => "[!]",
                Severity::Info => "[i]",
            };
            output.push_str(&format!("{} {}: {}\n", icon, issue.component, issue.message));
            output.push_str(&format!("    Fix: {}\n", issue.fix));
        }
    }

    output
}

/// Result of a startup health check
pub enum StartupHealthResult {
    /// All checks passed, system is ready
    Ready,
    /// Warnings present but system can start
    Warnings(Vec<HealthIssue>),
    /// Critical issues prevent startup
    Critical(Vec<HealthIssue>),
}

/// Runs a quick health check on startup and returns whether the system can proceed.
///
/// This function is designed to be called during rigrun startup to ensure
/// the system is properly configured before accepting queries.
///
/// # Returns
/// - `StartupHealthResult::Ready` if all checks pass
/// - `StartupHealthResult::Warnings` if there are warnings but system can operate
/// - `StartupHealthResult::Critical` if there are critical issues that prevent operation
///
/// # Example
///
/// ```no_run
/// use rigrun::health::{check_startup_health, StartupHealthResult};
///
/// match check_startup_health() {
///     StartupHealthResult::Ready => {
///         println!("System ready!");
///     }
///     StartupHealthResult::Warnings(warnings) => {
///         for w in &warnings {
///             eprintln!("[!] {}: {}", w.component, w.message);
///         }
///         println!("Starting with warnings...");
///     }
///     StartupHealthResult::Critical(errors) => {
///         for e in &errors {
///             eprintln!("[X] {}: {}", e.component, e.message);
///             eprintln!("    Fix: {}", e.fix);
///         }
///         std::process::exit(1);
///     }
/// }
/// ```
pub fn check_startup_health() -> StartupHealthResult {
    let status = HealthChecker::new().run_quick_check();

    let critical: Vec<HealthIssue> = status.issues.iter()
        .filter(|i| i.severity == Severity::Critical)
        .cloned()
        .collect();

    let warnings: Vec<HealthIssue> = status.issues.iter()
        .filter(|i| i.severity == Severity::Warning)
        .cloned()
        .collect();

    if !critical.is_empty() {
        StartupHealthResult::Critical(critical)
    } else if !warnings.is_empty() {
        StartupHealthResult::Warnings(warnings)
    } else {
        StartupHealthResult::Ready
    }
}

/// Prints startup health warnings to stderr with colored output.
///
/// This is a convenience function to display startup warnings in a consistent format.
pub fn print_startup_warnings(warnings: &[HealthIssue]) {
    if warnings.is_empty() {
        return;
    }

    eprintln!();
    eprintln!("Startup warnings:");
    for w in warnings {
        eprintln!("  [!] {}: {}", w.component, w.message);
        eprintln!("      Fix: {}", w.fix);
    }
    eprintln!();
}

/// Prints critical errors and exits.
///
/// This function displays critical errors that prevent rigrun from starting
/// and then exits with code 1.
pub fn print_critical_and_exit(errors: &[HealthIssue]) -> ! {
    eprintln!();
    eprintln!("Critical errors - cannot start rigrun:");
    eprintln!();
    for e in errors {
        eprintln!("  [X] {}: {}", e.component, e.message);
        eprintln!("      Fix: {}", e.fix);
        eprintln!();
    }
    eprintln!("Run 'rigrun doctor' for detailed diagnostics.");
    eprintln!();
    std::process::exit(1);
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_health_issue_creation() {
        let issue = HealthIssue::critical("Test", "Test message", "Test fix");
        assert_eq!(issue.severity, Severity::Critical);
        assert_eq!(issue.component, "Test");
        assert_eq!(issue.message, "Test message");
        assert_eq!(issue.fix, "Test fix");
        assert!(issue.auto_fix_command.is_none());
    }

    #[test]
    fn test_health_issue_with_auto_fix() {
        let issue = HealthIssue::warning("Test", "Message", "Fix")
            .with_auto_fix("some command");
        assert_eq!(issue.auto_fix_command, Some("some command".to_string()));
    }

    #[test]
    fn test_health_status_defaults() {
        let status = HealthStatus::default();
        assert!(!status.ollama_running);
        assert!(!status.gpu_detected);
        assert!(status.config_valid);
        assert!(status.issues.is_empty());
    }

    #[test]
    fn test_health_status_issue_counts() {
        let mut status = HealthStatus::default();
        status.issues.push(HealthIssue::critical("A", "B", "C"));
        status.issues.push(HealthIssue::warning("A", "B", "C"));
        status.issues.push(HealthIssue::warning("A", "B", "C"));
        status.issues.push(HealthIssue::info("A", "B", "C"));

        let (critical, warnings, info) = status.issue_counts();
        assert_eq!(critical, 1);
        assert_eq!(warnings, 2);
        assert_eq!(info, 1);
    }

    #[test]
    fn test_ollama_version_check() {
        assert!(check_ollama_version_compatible("0.5.4"));
        assert!(check_ollama_version_compatible("0.3.0"));
        assert!(check_ollama_version_compatible("1.0.0"));
        assert!(!check_ollama_version_compatible("0.2.9"));
        assert!(!check_ollama_version_compatible("invalid"));
    }

    #[test]
    fn test_severity_ordering() {
        assert!(Severity::Info < Severity::Warning);
        assert!(Severity::Warning < Severity::Critical);
    }
}
