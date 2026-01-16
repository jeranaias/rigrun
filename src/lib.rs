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
pub mod detect;
pub mod error;
pub mod local;
pub mod router;
pub mod server;
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
