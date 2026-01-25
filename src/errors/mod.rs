// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

//! IL5-Compliant Error Handling Module
//!
//! This module provides secure error handling that meets NIST 800-53 SI-11 requirements:
//! - No stack traces exposed to users
//! - No implementation details revealed
//! - No database info, IPs, or file paths exposed
//! - No PII or sensitive data leaked
//!
//! Errors include:
//! - User-friendly actionable messages
//! - Unique reference codes for support tracking
//! - Full internal logging with all details for debugging

use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use chrono::Utc;
use rand::Rng;
use regex::Regex;
use serde::Serialize;
use std::sync::LazyLock;
use tracing;

// =============================================================================
// ERROR REFERENCE CODE GENERATION
// =============================================================================

/// Generate a unique error reference code.
/// Format: ERR-YYYYMMDD-XXXXXX (e.g., ERR-20240115-A3F8K2)
pub fn generate_reference_code() -> String {
    let date = Utc::now().format("%Y%m%d");
    let mut rng = rand::thread_rng();
    let chars: Vec<char> = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789".chars().collect();
    let random: String = (0..6)
        .map(|_| chars[rng.gen_range(0..chars.len())])
        .collect();
    format!("ERR-{}-{}", date, random)
}

// =============================================================================
// USER-FACING ERROR TYPES (IL5 COMPLIANT)
// =============================================================================

/// IL5-compliant error types for user-facing responses.
///
/// These errors NEVER expose:
/// - Stack traces
/// - Implementation details
/// - Database information
/// - IP addresses or network details
/// - File system paths
/// - PII or sensitive data
#[derive(Debug, Clone, Serialize)]
#[serde(tag = "error_type", rename_all = "snake_case")]
pub enum UserError {
    /// Service temporarily unavailable (503)
    ServiceUnavailable {
        message: String,
        reference: String,
        retry_after_secs: Option<u64>,
    },

    /// Invalid request from client (400)
    InvalidRequest {
        message: String,
        reference: String,
        field: Option<String>,
    },

    /// Authentication required (401)
    AuthenticationRequired {
        message: String,
        reference: String,
    },

    /// Authorization denied (403)
    AuthorizationDenied {
        message: String,
        reference: String,
    },

    /// Session expired (401)
    SessionExpired {
        message: String,
        reference: String,
    },

    /// Rate limited (429)
    RateLimited {
        message: String,
        reference: String,
        retry_after_secs: u64,
    },

    /// Internal server error (500) - NEVER exposes internal details
    InternalError {
        message: String,
        reference: String,
    },

    /// Gateway timeout (504)
    GatewayTimeout {
        message: String,
        reference: String,
    },

    /// Bad gateway (502)
    BadGateway {
        message: String,
        reference: String,
    },

    /// Resource not found (404)
    NotFound {
        message: String,
        reference: String,
    },

    /// Request entity too large (413)
    PayloadTooLarge {
        message: String,
        reference: String,
        max_size: Option<u64>,
    },
}

impl UserError {
    /// Get the HTTP status code for this error.
    pub fn status_code(&self) -> StatusCode {
        match self {
            UserError::ServiceUnavailable { .. } => StatusCode::SERVICE_UNAVAILABLE,
            UserError::InvalidRequest { .. } => StatusCode::BAD_REQUEST,
            UserError::AuthenticationRequired { .. } => StatusCode::UNAUTHORIZED,
            UserError::AuthorizationDenied { .. } => StatusCode::FORBIDDEN,
            UserError::SessionExpired { .. } => StatusCode::UNAUTHORIZED,
            UserError::RateLimited { .. } => StatusCode::TOO_MANY_REQUESTS,
            UserError::InternalError { .. } => StatusCode::INTERNAL_SERVER_ERROR,
            UserError::GatewayTimeout { .. } => StatusCode::GATEWAY_TIMEOUT,
            UserError::BadGateway { .. } => StatusCode::BAD_GATEWAY,
            UserError::NotFound { .. } => StatusCode::NOT_FOUND,
            UserError::PayloadTooLarge { .. } => StatusCode::PAYLOAD_TOO_LARGE,
        }
    }

    /// Get the reference code for this error.
    pub fn reference(&self) -> &str {
        match self {
            UserError::ServiceUnavailable { reference, .. } => reference,
            UserError::InvalidRequest { reference, .. } => reference,
            UserError::AuthenticationRequired { reference, .. } => reference,
            UserError::AuthorizationDenied { reference, .. } => reference,
            UserError::SessionExpired { reference, .. } => reference,
            UserError::RateLimited { reference, .. } => reference,
            UserError::InternalError { reference, .. } => reference,
            UserError::GatewayTimeout { reference, .. } => reference,
            UserError::BadGateway { reference, .. } => reference,
            UserError::NotFound { reference, .. } => reference,
            UserError::PayloadTooLarge { reference, .. } => reference,
        }
    }

    /// Get the user-facing message.
    pub fn message(&self) -> &str {
        match self {
            UserError::ServiceUnavailable { message, .. } => message,
            UserError::InvalidRequest { message, .. } => message,
            UserError::AuthenticationRequired { message, .. } => message,
            UserError::AuthorizationDenied { message, .. } => message,
            UserError::SessionExpired { message, .. } => message,
            UserError::RateLimited { message, .. } => message,
            UserError::InternalError { message, .. } => message,
            UserError::GatewayTimeout { message, .. } => message,
            UserError::BadGateway { message, .. } => message,
            UserError::NotFound { message, .. } => message,
            UserError::PayloadTooLarge { message, .. } => message,
        }
    }
}

/// User-facing error response structure (JSON format).
#[derive(Debug, Clone, Serialize)]
pub struct ErrorResponse {
    pub error: UserError,
    pub status: u16,
}

impl IntoResponse for UserError {
    fn into_response(self) -> Response {
        let status = self.status_code();
        let response = ErrorResponse {
            status: status.as_u16(),
            error: self,
        };

        let body = serde_json::to_string(&response).unwrap_or_else(|_| {
            r#"{"error":{"error_type":"internal_error","message":"An unexpected error occurred","reference":"ERR-FALLBACK"},"status":500}"#.to_string()
        });

        (status, [("content-type", "application/json")], body).into_response()
    }
}

// =============================================================================
// ERROR CONSTRUCTORS (WITH LOGGING)
// =============================================================================

impl UserError {
    /// Create a ServiceUnavailable error, logging full details internally.
    pub fn service_unavailable(internal_error: &str) -> Self {
        let reference = generate_reference_code();
        let sanitized = sanitize_error_details(internal_error);

        // Log full details internally
        tracing::error!(
            reference = %reference,
            internal_error = %sanitized,
            "Service unavailable"
        );

        Self::ServiceUnavailable {
            message: "Service temporarily unavailable. Please try again later.".to_string(),
            reference,
            retry_after_secs: Some(30),
        }
    }

    /// Create an InvalidRequest error.
    pub fn invalid_request(user_message: &str, field: Option<&str>, internal_details: Option<&str>) -> Self {
        let reference = generate_reference_code();

        // Log internal details if provided
        if let Some(details) = internal_details {
            let sanitized = sanitize_error_details(details);
            tracing::warn!(
                reference = %reference,
                internal_details = %sanitized,
                field = ?field,
                "Invalid request"
            );
        }

        Self::InvalidRequest {
            message: user_message.to_string(),
            reference,
            field: field.map(|s| s.to_string()),
        }
    }

    /// Create an AuthenticationRequired error.
    pub fn authentication_required(internal_reason: Option<&str>) -> Self {
        let reference = generate_reference_code();

        if let Some(reason) = internal_reason {
            let sanitized = sanitize_error_details(reason);
            tracing::warn!(
                reference = %reference,
                internal_reason = %sanitized,
                "Authentication required"
            );
        }

        Self::AuthenticationRequired {
            message: "Authentication required. Please provide valid credentials.".to_string(),
            reference,
        }
    }

    /// Create an AuthorizationDenied error.
    pub fn authorization_denied(internal_reason: Option<&str>) -> Self {
        let reference = generate_reference_code();

        if let Some(reason) = internal_reason {
            let sanitized = sanitize_error_details(reason);
            tracing::warn!(
                reference = %reference,
                internal_reason = %sanitized,
                "Authorization denied"
            );
        }

        Self::AuthorizationDenied {
            message: "Access denied. You do not have permission to perform this action.".to_string(),
            reference,
        }
    }

    /// Create a SessionExpired error.
    pub fn session_expired() -> Self {
        let reference = generate_reference_code();

        tracing::info!(
            reference = %reference,
            "Session expired"
        );

        Self::SessionExpired {
            message: "Your session has expired. Please sign in again.".to_string(),
            reference,
        }
    }

    /// Create a RateLimited error.
    pub fn rate_limited(retry_after_secs: u64) -> Self {
        let reference = generate_reference_code();

        tracing::warn!(
            reference = %reference,
            retry_after_secs = %retry_after_secs,
            "Rate limited"
        );

        Self::RateLimited {
            message: format!("Too many requests. Please wait {} seconds before trying again.", retry_after_secs),
            reference,
            retry_after_secs,
        }
    }

    /// Create an InternalError, logging full details internally.
    /// CRITICAL: This NEVER exposes internal details to the user.
    pub fn internal_error(internal_error: &str) -> Self {
        let reference = generate_reference_code();
        let sanitized = sanitize_error_details(internal_error);

        // Log full details internally (sanitized for logs but complete)
        tracing::error!(
            reference = %reference,
            internal_error = %sanitized,
            "Internal server error"
        );

        Self::InternalError {
            message: format!("An internal error occurred. Reference: {}", reference),
            reference,
        }
    }

    /// Create a GatewayTimeout error.
    pub fn gateway_timeout(internal_details: &str) -> Self {
        let reference = generate_reference_code();
        let sanitized = sanitize_error_details(internal_details);

        tracing::error!(
            reference = %reference,
            internal_details = %sanitized,
            "Gateway timeout"
        );

        Self::GatewayTimeout {
            message: "The request timed out. Please try again.".to_string(),
            reference,
        }
    }

    /// Create a BadGateway error.
    pub fn bad_gateway(internal_details: &str) -> Self {
        let reference = generate_reference_code();
        let sanitized = sanitize_error_details(internal_details);

        tracing::error!(
            reference = %reference,
            internal_details = %sanitized,
            "Bad gateway"
        );

        Self::BadGateway {
            message: "Unable to connect to the backend service. Please try again later.".to_string(),
            reference,
        }
    }

    /// Create a NotFound error.
    pub fn not_found(resource: &str) -> Self {
        let reference = generate_reference_code();

        tracing::info!(
            reference = %reference,
            resource = %resource,
            "Resource not found"
        );

        Self::NotFound {
            message: format!("The requested {} was not found.", resource),
            reference,
        }
    }

    /// Create a PayloadTooLarge error.
    pub fn payload_too_large(max_size: u64) -> Self {
        let reference = generate_reference_code();

        tracing::warn!(
            reference = %reference,
            max_size = %max_size,
            "Payload too large"
        );

        Self::PayloadTooLarge {
            message: format!("Request body too large. Maximum size is {} bytes.", max_size),
            reference,
            max_size: Some(max_size),
        }
    }
}

// =============================================================================
// ERROR SANITIZATION (NIST 800-53 SI-11 COMPLIANT)
// =============================================================================

/// Patterns for sanitizing sensitive information from error messages.
/// These patterns are compiled once at startup.
static SANITIZE_PATTERNS: LazyLock<Vec<(Regex, &'static str)>> = LazyLock::new(|| {
    vec![
        // File paths (Windows and Unix)
        (Regex::new(r"[A-Za-z]:\\[^\s]+").expect("Windows path regex"), "[PATH_REDACTED]"),
        (Regex::new(r"/(?:home|usr|var|etc|opt|tmp|root)/[^\s]+").expect("Unix path regex"), "[PATH_REDACTED]"),
        (Regex::new(r"\\\\[^\s]+").expect("UNC path regex"), "[PATH_REDACTED]"),

        // IP addresses (IPv4 and IPv6)
        (Regex::new(r"\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b").expect("IPv4 regex"), "[IP_REDACTED]"),
        (Regex::new(r"\b(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}\b").expect("IPv6 full regex"), "[IP_REDACTED]"),
        (Regex::new(r"\b(?:[0-9a-fA-F]{1,4}:){1,7}:\b").expect("IPv6 short regex"), "[IP_REDACTED]"),

        // Database connection strings
        (Regex::new(r"(?i)(?:postgres|mysql|mongodb|redis|sqlite)://[^\s]+").expect("DB URL regex"), "[DB_CONN_REDACTED]"),
        (Regex::new(r"(?i)(?:host|server)=[^\s;]+").expect("DB host regex"), "host=[REDACTED]"),
        (Regex::new(r"(?i)(?:user|username)=[^\s;]+").expect("DB user regex"), "user=[REDACTED]"),
        (Regex::new(r"(?i)password=[^\s;]+").expect("Password regex"), "password=[REDACTED]"),
        (Regex::new(r"(?i)(?:database|db)=[^\s;]+").expect("DB name regex"), "database=[REDACTED]"),

        // API keys and tokens (reuse from audit.rs patterns)
        (Regex::new(r"sk-[a-zA-Z0-9]{20,}").expect("OpenAI key regex"), "[API_KEY_REDACTED]"),
        (Regex::new(r"sk-or-[a-zA-Z0-9-]{20,}").expect("OpenRouter key regex"), "[API_KEY_REDACTED]"),
        (Regex::new(r"sk-ant-[a-zA-Z0-9-]{20,}").expect("Anthropic key regex"), "[API_KEY_REDACTED]"),
        (Regex::new(r"AKIA[0-9A-Z]{16}").expect("AWS key regex"), "[AWS_KEY_REDACTED]"),
        (Regex::new(r"ghp_[a-zA-Z0-9]{36}").expect("GitHub token regex"), "[GITHUB_TOKEN_REDACTED]"),
        (Regex::new(r"Bearer [a-zA-Z0-9-._~+/]+=*").expect("Bearer token regex"), "Bearer [TOKEN_REDACTED]"),

        // Stack traces (Rust-specific patterns)
        (Regex::new(r"at [^\s]+\.rs:\d+:\d+").expect("Rust location regex"), "[LOCATION_REDACTED]"),
        (Regex::new(r"thread '[^']+' panicked at").expect("Panic regex"), "[PANIC_REDACTED]"),
        (Regex::new(r"stack backtrace:[\s\S]*").expect("Stack trace regex"), "[STACK_TRACE_REDACTED]"),
        (Regex::new(r"\d+:\s+0x[0-9a-f]+\s+-\s+[^\n]+").expect("Backtrace line regex"), "[BACKTRACE_REDACTED]"),

        // Function names that might reveal implementation
        (Regex::new(r"(?i)fn\s+[a-z_][a-z0-9_]*\s*\(").expect("Function regex"), "fn [FUNC_REDACTED]("),

        // Email addresses (PII)
        (Regex::new(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b").expect("Email regex"), "[EMAIL_REDACTED]"),

        // Phone numbers (PII)
        (Regex::new(r"\b(?:\+?1[-.\s]?)?\(?[0-9]{3}\)?[-.\s]?[0-9]{3}[-.\s]?[0-9]{4}\b").expect("Phone regex"), "[PHONE_REDACTED]"),

        // SSN (PII - US format)
        (Regex::new(r"\b\d{3}-\d{2}-\d{4}\b").expect("SSN regex"), "[SSN_REDACTED]"),

        // Credit card numbers (PII)
        (Regex::new(r"\b(?:\d{4}[-\s]?){3}\d{4}\b").expect("CC regex"), "[CC_REDACTED]"),

        // Generic long alphanumeric strings (potential secrets)
        (Regex::new(r"\b[A-Za-z0-9]{40,}\b").expect("Long secret regex"), "[SECRET_REDACTED]"),
    ]
});

/// Sanitize error details to remove sensitive information.
///
/// Removes:
/// - File paths (Windows and Unix)
/// - IP addresses (IPv4 and IPv6)
/// - Database connection strings and credentials
/// - API keys and tokens
/// - Stack traces
/// - PII (emails, phone numbers, SSN, credit cards)
pub fn sanitize_error_details(error: &str) -> String {
    let mut result = error.to_string();

    for (pattern, replacement) in SANITIZE_PATTERNS.iter() {
        result = pattern.replace_all(&result, *replacement).to_string();
    }

    result
}

/// Check if an error message contains potentially sensitive information.
pub fn contains_sensitive_info(error: &str) -> bool {
    for (pattern, _) in SANITIZE_PATTERNS.iter() {
        if pattern.is_match(error) {
            return true;
        }
    }
    false
}

// =============================================================================
// ERROR MAPPING FROM INTERNAL ERRORS
// =============================================================================

/// Map common error patterns to appropriate UserError types.
pub fn map_error(error: &str) -> UserError {
    let lower = error.to_lowercase();

    // Connection/network errors -> ServiceUnavailable or BadGateway
    if lower.contains("connection refused")
        || lower.contains("connect timeout")
        || lower.contains("dns resolution failed")
        || lower.contains("network unreachable") {
        return UserError::bad_gateway(error);
    }

    // Timeout errors -> GatewayTimeout
    if lower.contains("timeout") || lower.contains("timed out") {
        return UserError::gateway_timeout(error);
    }

    // Rate limiting indicators
    if lower.contains("rate limit") || lower.contains("too many requests") || lower.contains("429") {
        return UserError::rate_limited(60);
    }

    // Authentication errors
    if lower.contains("unauthorized") || lower.contains("invalid api key")
        || lower.contains("authentication failed") || lower.contains("401") {
        return UserError::authentication_required(Some(error));
    }

    // Authorization/permission errors
    if lower.contains("forbidden") || lower.contains("permission denied")
        || lower.contains("access denied") || lower.contains("403") {
        return UserError::authorization_denied(Some(error));
    }

    // Validation errors -> InvalidRequest
    if lower.contains("invalid") || lower.contains("malformed")
        || lower.contains("validation") || lower.contains("missing required") {
        return UserError::invalid_request(
            "Invalid request. Please check your input and try again.",
            None,
            Some(error),
        );
    }

    // Not found errors
    if lower.contains("not found") || lower.contains("404") {
        return UserError::not_found("resource");
    }

    // Service unavailable
    if lower.contains("service unavailable") || lower.contains("503")
        || lower.contains("temporarily unavailable") {
        return UserError::service_unavailable(error);
    }

    // Payload too large
    if lower.contains("too large") || lower.contains("payload")
        || lower.contains("body limit") || lower.contains("413") {
        return UserError::payload_too_large(1024 * 1024); // Default 1MB
    }

    // Default: Internal error (safe fallback)
    UserError::internal_error(error)
}

/// Map an anyhow::Error to a UserError.
pub fn map_anyhow_error(error: &anyhow::Error) -> UserError {
    map_error(&error.to_string())
}

/// Map a std::io::Error to a UserError.
pub fn map_io_error(error: &std::io::Error) -> UserError {
    let kind = error.kind();
    let error_str = error.to_string();

    match kind {
        std::io::ErrorKind::NotFound => UserError::not_found("file or resource"),
        std::io::ErrorKind::PermissionDenied => UserError::authorization_denied(Some(&error_str)),
        std::io::ErrorKind::ConnectionRefused => UserError::bad_gateway(&error_str),
        std::io::ErrorKind::ConnectionReset => UserError::service_unavailable(&error_str),
        std::io::ErrorKind::TimedOut => UserError::gateway_timeout(&error_str),
        _ => UserError::internal_error(&error_str),
    }
}

// =============================================================================
// RESULT TYPE ALIAS
// =============================================================================

/// Result type that uses UserError for the error variant.
pub type ApiResult<T> = Result<T, UserError>;

// =============================================================================
// TESTS
// =============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_generate_reference_code() {
        let code = generate_reference_code();
        assert!(code.starts_with("ERR-"));
        assert_eq!(code.len(), 19); // ERR-YYYYMMDD-XXXXXX = 4+8+1+6

        // Should be unique
        let code2 = generate_reference_code();
        assert_ne!(code, code2);
    }

    #[test]
    fn test_status_codes() {
        assert_eq!(
            UserError::service_unavailable("test").status_code(),
            StatusCode::SERVICE_UNAVAILABLE
        );
        assert_eq!(
            UserError::invalid_request("test", None, None).status_code(),
            StatusCode::BAD_REQUEST
        );
        assert_eq!(
            UserError::authentication_required(None).status_code(),
            StatusCode::UNAUTHORIZED
        );
        assert_eq!(
            UserError::authorization_denied(None).status_code(),
            StatusCode::FORBIDDEN
        );
        assert_eq!(
            UserError::rate_limited(60).status_code(),
            StatusCode::TOO_MANY_REQUESTS
        );
        assert_eq!(
            UserError::internal_error("test").status_code(),
            StatusCode::INTERNAL_SERVER_ERROR
        );
        assert_eq!(
            UserError::gateway_timeout("test").status_code(),
            StatusCode::GATEWAY_TIMEOUT
        );
        assert_eq!(
            UserError::not_found("resource").status_code(),
            StatusCode::NOT_FOUND
        );
    }

    #[test]
    fn test_sanitize_file_paths() {
        let windows_path = r"Error at C:\Users\admin\secret\file.rs:42";
        let sanitized = sanitize_error_details(windows_path);
        assert!(!sanitized.contains("Users"));
        assert!(!sanitized.contains("admin"));
        assert!(sanitized.contains("[PATH_REDACTED]"));

        let unix_path = "Error at /home/user/project/src/main.rs:100";
        let sanitized = sanitize_error_details(unix_path);
        assert!(!sanitized.contains("home"));
        assert!(!sanitized.contains("user"));
        assert!(sanitized.contains("[PATH_REDACTED]"));
    }

    #[test]
    fn test_sanitize_ip_addresses() {
        let ipv4 = "Connection failed to 192.168.1.100:8080";
        let sanitized = sanitize_error_details(ipv4);
        assert!(!sanitized.contains("192.168.1.100"));
        assert!(sanitized.contains("[IP_REDACTED]"));
    }

    #[test]
    fn test_sanitize_database_urls() {
        let db_url = "Failed to connect: postgres://admin:password123@localhost/mydb";
        let sanitized = sanitize_error_details(db_url);
        assert!(!sanitized.contains("password123"));
        assert!(!sanitized.contains("admin"));
        assert!(sanitized.contains("[DB_CONN_REDACTED]"));
    }

    #[test]
    fn test_sanitize_api_keys() {
        let api_key = "API error with key sk-1234567890abcdefghij1234567890";
        let sanitized = sanitize_error_details(api_key);
        assert!(!sanitized.contains("sk-1234567890"));
        assert!(sanitized.contains("[API_KEY_REDACTED]"));
    }

    #[test]
    fn test_sanitize_email() {
        let email = "User test@example.com not found";
        let sanitized = sanitize_error_details(email);
        assert!(!sanitized.contains("test@example.com"));
        assert!(sanitized.contains("[EMAIL_REDACTED]"));
    }

    #[test]
    fn test_sanitize_stack_trace() {
        let stack = "Error at src/main.rs:42:5\nstack backtrace:\n  0: some::function";
        let sanitized = sanitize_error_details(stack);
        assert!(!sanitized.contains("main.rs:42:5"));
        assert!(sanitized.contains("[LOCATION_REDACTED]") || sanitized.contains("[STACK_TRACE_REDACTED]"));
    }

    #[test]
    fn test_error_mapping() {
        // Connection errors
        let err = map_error("Connection refused: localhost:11434");
        assert_eq!(err.status_code(), StatusCode::BAD_GATEWAY);

        // Timeout errors
        let err = map_error("Request timed out after 30s");
        assert_eq!(err.status_code(), StatusCode::GATEWAY_TIMEOUT);

        // Rate limiting
        let err = map_error("Rate limit exceeded");
        assert_eq!(err.status_code(), StatusCode::TOO_MANY_REQUESTS);

        // Auth errors
        let err = map_error("Unauthorized: invalid API key");
        assert_eq!(err.status_code(), StatusCode::UNAUTHORIZED);

        // Permission errors
        let err = map_error("Access denied: forbidden");
        assert_eq!(err.status_code(), StatusCode::FORBIDDEN);

        // Unknown errors -> Internal
        let err = map_error("Some unknown error happened");
        assert_eq!(err.status_code(), StatusCode::INTERNAL_SERVER_ERROR);
    }

    #[test]
    fn test_internal_error_never_exposes_details() {
        let sensitive_error = "Database error at postgres://admin:secret@192.168.1.5/prod connecting from /home/user/app";
        let user_error = UserError::internal_error(sensitive_error);

        let message = user_error.message();
        assert!(!message.contains("postgres"));
        assert!(!message.contains("admin"));
        assert!(!message.contains("secret"));
        assert!(!message.contains("192.168"));
        assert!(!message.contains("/home/user"));
        assert!(message.contains("Reference:"));
    }

    #[test]
    fn test_contains_sensitive_info() {
        assert!(contains_sensitive_info("Error at C:\\Users\\admin\\file.rs"));
        assert!(contains_sensitive_info("Connection to 192.168.1.1 failed"));
        assert!(contains_sensitive_info("API key: sk-1234567890abcdefghij1234"));
        assert!(!contains_sensitive_info("Simple error message"));
    }

    #[test]
    fn test_user_error_serialization() {
        let error = UserError::invalid_request("Bad input", Some("query"), None);
        let json = serde_json::to_string(&error).unwrap();

        assert!(json.contains("invalid_request"));
        assert!(json.contains("Bad input"));
        assert!(json.contains("query"));
        assert!(json.contains("reference"));
    }
}
