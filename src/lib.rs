// Copyright (c) 2024-2025 Jesse Morgan
// Licensed under the MIT License. See LICENSE file for details.

//! rigrun - Local-first LLM router library
//!
//! Your GPU first, cloud when needed.
//!
//! RigRun provides intelligent query routing to optimize costs while maintaining
//! response quality. It automatically routes queries through a tiered system:
//!
//! **Cache** -> **Local LLM** -> **Cloud (Haiku/Sonnet/Opus)**
//!
//! # Core Modules
//!
//! - [`router`] - Query routing, tier management, and complexity classification
//! - [`cache`] - Response caching with TTL
//! - [`local`] - Ollama integration for local inference
//! - [`detect`] - GPU detection and model recommendations
//! - [`stats`] - Cost tracking and savings analytics
//! - [`cloud`] - Cloud API integration (OpenRouter)
//! - [`server`] - HTTP server for API compatibility
//! - [`audit`] - Privacy audit logging for transparency
//! - [`error`] - Consistent error formatting utilities

pub mod audit;
pub mod cache;
pub mod cloud;
pub mod consent_banner;
pub mod detect;
pub mod error;
pub mod errors;
pub mod health;
pub mod firstrun;
pub mod local;
pub mod router;
pub mod security;
pub mod server;
pub mod setup;
pub mod stats;
pub mod types;
pub mod utils;

// Re-export commonly used types from types module
pub use types::Tier;

// Re-export commonly used types from router
pub use router::{
    classify_query, route_query, route_query_detailed,
    QueryComplexity, QueryResult, QueryType, RoutingDecision,
};

// Re-export cache types
pub use cache::{CachedResponse, QueryCache, CacheStats};

// Re-export from other modules
pub use detect::{
    detect_gpu, recommend_model, recommend_models_all, ModelRecommendation, GpuInfo, GpuType,
    // GPU utilization checking
    check_gpu_utilization, check_loaded_model_gpu_usage, get_gpu_status_report,
    GpuUtilizationStatus, PostLoadGpuCheck, GpuStatusReport,
    // Ollama model status
    get_ollama_loaded_models, get_model_gpu_status, is_model_using_gpu,
    LoadedModelInfo, ProcessorType,
    // VRAM estimation and checking
    estimate_model_vram, will_model_fit_in_vram, format_cpu_fallback_warning,
    // Real-time GPU memory usage
    get_gpu_memory_usage, get_nvidia_gpu_memory_usage, get_amd_gpu_memory_usage,
    GpuMemoryUsage,
    // GPU setup guidance
    get_gpu_setup_guidance, get_gpu_setup_status, GpuSetupGuidance, GpuSetupStatus,
    // AMD-specific detection
    detect_amd_architecture, AmdArchitecture, check_rocm_installed, get_hsa_override_version,
    // NVIDIA-specific detection
    check_nvidia_smi_available, get_nvidia_driver_version, is_nvidia_driver_recent,
    // CPU fallback diagnosis
    diagnose_cpu_fallback, CpuFallbackCause, CpuFallbackDiagnosis,
};
pub use local::{OllamaClient, OllamaResponse};
pub use types::Message;
pub use stats::{QueryStats, SessionStats, StatsTracker, SavingsSummary};
pub use cloud::OpenRouterClient;
pub use server::Server;
pub use utils::mask_sensitive;

// Re-export audit types
pub use audit::{
    AuditEntry, AuditLogger, AuditTier,
    audit_log_query, audit_log_blocked, is_audit_enabled, set_audit_enabled,
    init_audit_logger, global_audit_logger, redact_secrets,
};

// Re-export error utilities
pub use error::{format_error, format_simple_error, ErrorBuilder, GITHUB_ISSUES_URL};

// Re-export IL5-compliant error handling (NIST 800-53 SI-11)
pub use errors::{
    UserError, ErrorResponse, ApiResult,
    generate_reference_code, sanitize_error_details, contains_sensitive_info,
    map_error, map_anyhow_error, map_io_error,
};

// Re-export security types (DoD STIG compliance)
pub use security::{
    Session, SessionConfig, SessionManager, SessionState, SessionEvent,
    DOD_STIG_MAX_SESSION_TIMEOUT_SECS, DOD_STIG_WARNING_BEFORE_TIMEOUT_SECS,
};

// Re-export setup types
pub use setup::{
    run_setup, SetupWizard, SetupMode, SetupResult, HardwareMode,
    RigrunConfig, GeneralConfig, HardwareConfig, ModelConfig, ServerConfig, SecurityConfig,
};

// Re-export consent banner types (DoD IL5 compliance)
pub use consent_banner::{
    handle_consent_banner, display_and_acknowledge, should_display_banner,
    is_ci_environment, is_interactive, consent_log_path, log_consent,
    ConsentAcknowledgment, DOD_CONSENT_BANNER_TEXT,
};

// Re-export first-run wizard types
pub use firstrun::{
    is_first_run, mark_wizard_complete, run_wizard, run_quick_wizard,
    WizardConfig, DeploymentMode, UseCase, ModelSelection,
    download_model_with_progress, run_health_check, generate_config,
};

// Re-export health check types
pub use health::{
    HealthChecker, HealthStatus, HealthIssue, Severity,
    run_health_check as health_check, run_quick_health_check, auto_fix_issues,
    format_health_status, GpuInfoStatus,
};
