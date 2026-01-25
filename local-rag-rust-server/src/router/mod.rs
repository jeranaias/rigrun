//! Router module for the local RAG Rust server
//!
//! This module handles HTTP routing, request parsing, and audit logging
//! for all API endpoints.

use std::sync::Arc;
use std::io::Write;
use std::fs::{File, OpenOptions};
use std::path::Path;
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use thiserror::Error;

// ============================================================================
// Error Types
// ============================================================================

/// Router-specific errors
#[derive(Debug, Error)]
pub enum RouterError {
    #[error("Route not found: {0}")]
    NotFound(String),

    #[error("Method not allowed: {0}")]
    MethodNotAllowed(String),

    #[error("Bad request: {0}")]
    BadRequest(String),

    #[error("Internal error: {0}")]
    Internal(String),

    #[error("Audit log error: {0}")]
    AuditLogError(String),
}

// ============================================================================
// Audit Logging
// ============================================================================

/// Audit log entry structure
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AuditLogEntry {
    pub timestamp: DateTime<Utc>,
    pub request_id: String,
    pub method: String,
    pub path: String,
    pub user_id: Option<String>,
    pub ip_address: Option<String>,
    pub status_code: u16,
    pub response_time_ms: u64,
    pub error_message: Option<String>,
}

impl AuditLogEntry {
    /// Create a new audit log entry
    pub fn new(
        request_id: String,
        method: String,
        path: String,
    ) -> Self {
        Self {
            timestamp: Utc::now(),
            request_id,
            method,
            path,
            user_id: None,
            ip_address: None,
            status_code: 0,
            response_time_ms: 0,
            error_message: None,
        }
    }

    /// Set the user ID
    pub fn with_user_id(mut self, user_id: String) -> Self {
        self.user_id = Some(user_id);
        self
    }

    /// Set the IP address
    pub fn with_ip_address(mut self, ip_address: String) -> Self {
        self.ip_address = Some(ip_address);
        self
    }

    /// Set the response status code
    pub fn with_status_code(mut self, status_code: u16) -> Self {
        self.status_code = status_code;
        self
    }

    /// Set the response time
    pub fn with_response_time_ms(mut self, response_time_ms: u64) -> Self {
        self.response_time_ms = response_time_ms;
        self
    }

    /// Set the error message
    pub fn with_error(mut self, error_message: String) -> Self {
        self.error_message = Some(error_message);
        self
    }
}

/// Audit log writer trait
pub trait AuditLogWriter: Send + Sync {
    /// Write an audit log entry
    fn write(&self, entry: &AuditLogEntry) -> Result<(), std::io::Error>;
}

/// File-based audit log writer
pub struct FileAuditLog {
    path: std::path::PathBuf,
}

impl FileAuditLog {
    /// Create a new file-based audit log
    pub fn new<P: AsRef<Path>>(path: P) -> Self {
        Self {
            path: path.as_ref().to_path_buf(),
        }
    }
}

impl AuditLogWriter for FileAuditLog {
    fn write(&self, entry: &AuditLogEntry) -> Result<(), std::io::Error> {
        let mut file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&self.path)?;

        let json = serde_json::to_string(entry)
            .map_err(|e| std::io::Error::new(std::io::ErrorKind::InvalidData, e))?;

        writeln!(file, "{}", json)?;

        Ok(())
    }
}

/// In-memory audit log for testing
#[derive(Default)]
pub struct InMemoryAuditLog {
    entries: std::sync::Mutex<Vec<AuditLogEntry>>,
}

impl InMemoryAuditLog {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn entries(&self) -> Vec<AuditLogEntry> {
        self.entries.lock().unwrap().clone()
    }
}

impl AuditLogWriter for InMemoryAuditLog {
    fn write(&self, entry: &AuditLogEntry) -> Result<(), std::io::Error> {
        self.entries.lock().unwrap().push(entry.clone());
        Ok(())
    }
}

// ============================================================================
// Request/Response Types
// ============================================================================

/// HTTP method enum
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Method {
    Get,
    Post,
    Put,
    Delete,
    Options,
}

impl std::fmt::Display for Method {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Method::Get => write!(f, "GET"),
            Method::Post => write!(f, "POST"),
            Method::Put => write!(f, "PUT"),
            Method::Delete => write!(f, "DELETE"),
            Method::Options => write!(f, "OPTIONS"),
        }
    }
}

/// Incoming request structure
#[derive(Debug)]
pub struct Request {
    pub id: String,
    pub method: Method,
    pub path: String,
    pub headers: std::collections::HashMap<String, String>,
    pub body: Option<Vec<u8>>,
    pub user_id: Option<String>,
    pub ip_address: Option<String>,
}

/// Outgoing response structure
#[derive(Debug)]
pub struct Response {
    pub status_code: u16,
    pub headers: std::collections::HashMap<String, String>,
    pub body: Option<Vec<u8>>,
}

impl Response {
    pub fn ok() -> Self {
        Self {
            status_code: 200,
            headers: std::collections::HashMap::new(),
            body: None,
        }
    }

    pub fn with_json<T: Serialize>(mut self, data: &T) -> Result<Self, RouterError> {
        let body = serde_json::to_vec(data)
            .map_err(|e| RouterError::Internal(format!("JSON serialization failed: {}", e)))?;
        self.body = Some(body);
        self.headers.insert("Content-Type".to_string(), "application/json".to_string());
        Ok(self)
    }

    pub fn not_found(message: &str) -> Self {
        Self {
            status_code: 404,
            headers: std::collections::HashMap::new(),
            body: Some(message.as_bytes().to_vec()),
        }
    }

    pub fn bad_request(message: &str) -> Self {
        Self {
            status_code: 400,
            headers: std::collections::HashMap::new(),
            body: Some(message.as_bytes().to_vec()),
        }
    }

    pub fn internal_error(message: &str) -> Self {
        Self {
            status_code: 500,
            headers: std::collections::HashMap::new(),
            body: Some(message.as_bytes().to_vec()),
        }
    }
}

// ============================================================================
// Route Handler Type
// ============================================================================

/// Route handler function type
pub type RouteHandler = Box<dyn Fn(&Request) -> Result<Response, RouterError> + Send + Sync>;

/// Route definition
pub struct Route {
    pub method: Method,
    pub path: String,
    pub handler: RouteHandler,
}

// ============================================================================
// Router Implementation
// ============================================================================

/// Main router structure
pub struct Router<A: AuditLogWriter> {
    routes: Vec<Route>,
    audit_log: Arc<A>,
    /// Whether to fail on audit log errors (for high-security environments)
    fail_on_audit_error: bool,
}

impl<A: AuditLogWriter> Router<A> {
    /// Create a new router with an audit log writer
    pub fn new(audit_log: Arc<A>) -> Self {
        Self {
            routes: Vec::new(),
            audit_log,
            fail_on_audit_error: false,
        }
    }

    /// Create a new router that fails on audit log errors
    ///
    /// Use this in high-security environments where audit log integrity
    /// is critical and requests should fail if logging fails.
    pub fn new_strict(audit_log: Arc<A>) -> Self {
        Self {
            routes: Vec::new(),
            audit_log,
            fail_on_audit_error: true,
        }
    }

    /// Add a route to the router
    pub fn add_route(&mut self, method: Method, path: &str, handler: RouteHandler) {
        self.routes.push(Route {
            method,
            path: path.to_string(),
            handler,
        });
    }

    /// Handle an incoming request
    pub fn handle(&self, request: Request) -> Response {
        let start_time = std::time::Instant::now();

        // Create audit log entry
        let mut audit_entry = AuditLogEntry::new(
            request.id.clone(),
            request.method.to_string(),
            request.path.clone(),
        );

        if let Some(ref user_id) = request.user_id {
            audit_entry = audit_entry.with_user_id(user_id.clone());
        }

        if let Some(ref ip_address) = request.ip_address {
            audit_entry = audit_entry.with_ip_address(ip_address.clone());
        }

        // Find matching route
        let response = self.dispatch(&request);

        let response_time_ms = start_time.elapsed().as_millis() as u64;

        // Update audit entry with response info
        audit_entry = audit_entry
            .with_status_code(response.status_code)
            .with_response_time_ms(response_time_ms);

        if response.status_code >= 400 {
            if let Some(ref body) = response.body {
                if let Ok(error_msg) = String::from_utf8(body.clone()) {
                    audit_entry = audit_entry.with_error(error_msg);
                }
            }
        }

        // RROUTER-2 FIX: Properly handle audit log write failures
        // Previously this was: let _ = self.audit_log.write(&audit_entry);
        // which silently ignored errors. Now we log them and optionally propagate.
        if let Err(e) = self.audit_log.write(&audit_entry) {
            // Log the error so it's visible in monitoring/alerting systems
            tracing::error!(
                "Audit log write failed for request {}: {}",
                request.id,
                e
            );

            // In high-security mode, fail the request if audit logging fails
            // This ensures we never process requests without proper audit trails
            if self.fail_on_audit_error {
                tracing::error!(
                    "Failing request {} due to audit log failure (strict mode enabled)",
                    request.id
                );
                return Response::internal_error(
                    "Request processing failed due to audit logging error"
                );
            }

            // In normal mode, we continue but the error is logged
            // Security teams can monitor for these errors and investigate
        }

        response
    }

    /// Dispatch a request to the appropriate handler
    fn dispatch(&self, request: &Request) -> Response {
        // Find matching route
        for route in &self.routes {
            if route.method == request.method && self.path_matches(&route.path, &request.path) {
                match (route.handler)(request) {
                    Ok(response) => return response,
                    Err(e) => {
                        tracing::error!("Handler error: {}", e);
                        return match e {
                            RouterError::NotFound(msg) => Response::not_found(&msg),
                            RouterError::BadRequest(msg) => Response::bad_request(&msg),
                            RouterError::MethodNotAllowed(msg) => Response {
                                status_code: 405,
                                headers: std::collections::HashMap::new(),
                                body: Some(msg.as_bytes().to_vec()),
                            },
                            _ => Response::internal_error(&e.to_string()),
                        };
                    }
                }
            }
        }

        // No matching route found
        Response::not_found(&format!("No route found for {} {}", request.method, request.path))
    }

    /// Check if a route path matches a request path
    /// Supports simple path parameters like /users/:id
    fn path_matches(&self, route_path: &str, request_path: &str) -> bool {
        let route_parts: Vec<&str> = route_path.split('/').collect();
        let request_parts: Vec<&str> = request_path.split('/').collect();

        if route_parts.len() != request_parts.len() {
            return false;
        }

        for (route_part, request_part) in route_parts.iter().zip(request_parts.iter()) {
            if route_part.starts_with(':') {
                // Path parameter - always matches
                continue;
            }
            if route_part != request_part {
                return false;
            }
        }

        true
    }

    /// Write an audit log entry directly (for custom logging scenarios)
    ///
    /// # Returns
    /// * `Ok(())` if the entry was written successfully
    /// * `Err(RouterError)` if writing failed
    ///
    /// RROUTER-2 FIX: This method now properly returns errors instead of
    /// silently ignoring them, allowing callers to handle failures appropriately.
    pub fn write_audit_log(&self, entry: &AuditLogEntry) -> Result<(), RouterError> {
        self.audit_log.write(entry).map_err(|e| {
            tracing::error!("Audit log write failed: {}", e);
            RouterError::AuditLogError(format!("Failed to write audit log: {}", e))
        })
    }
}

// ============================================================================
// Middleware Support
// ============================================================================

/// Middleware function type
pub type Middleware = Box<dyn Fn(Request, &dyn Fn(Request) -> Response) -> Response + Send + Sync>;

/// Router with middleware support
pub struct MiddlewareRouter<A: AuditLogWriter> {
    router: Router<A>,
    middlewares: Vec<Middleware>,
}

impl<A: AuditLogWriter> MiddlewareRouter<A> {
    pub fn new(audit_log: Arc<A>) -> Self {
        Self {
            router: Router::new(audit_log),
            middlewares: Vec::new(),
        }
    }

    pub fn add_middleware(&mut self, middleware: Middleware) {
        self.middlewares.push(middleware);
    }

    pub fn add_route(&mut self, method: Method, path: &str, handler: RouteHandler) {
        self.router.add_route(method, path, handler);
    }

    pub fn handle(&self, request: Request) -> Response {
        // Apply middlewares in order, then call the router
        let final_handler = |req: Request| self.router.handle(req);

        // For simplicity, we just call the router directly
        // A full implementation would chain middlewares
        self.router.handle(request)
    }
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicBool, Ordering};

    /// Mock audit log that can be configured to fail
    struct FailingAuditLog {
        should_fail: AtomicBool,
        entries: std::sync::Mutex<Vec<AuditLogEntry>>,
    }

    impl FailingAuditLog {
        fn new(should_fail: bool) -> Self {
            Self {
                should_fail: AtomicBool::new(should_fail),
                entries: std::sync::Mutex::new(Vec::new()),
            }
        }

        fn set_should_fail(&self, should_fail: bool) {
            self.should_fail.store(should_fail, Ordering::SeqCst);
        }
    }

    impl AuditLogWriter for FailingAuditLog {
        fn write(&self, entry: &AuditLogEntry) -> Result<(), std::io::Error> {
            if self.should_fail.load(Ordering::SeqCst) {
                return Err(std::io::Error::new(
                    std::io::ErrorKind::Other,
                    "Simulated audit log failure"
                ));
            }
            self.entries.lock().unwrap().push(entry.clone());
            Ok(())
        }
    }

    #[test]
    fn test_router_basic() {
        let audit_log = Arc::new(InMemoryAuditLog::new());
        let mut router = Router::new(audit_log.clone());

        router.add_route(
            Method::Get,
            "/health",
            Box::new(|_| Ok(Response::ok())),
        );

        let request = Request {
            id: "test-1".to_string(),
            method: Method::Get,
            path: "/health".to_string(),
            headers: std::collections::HashMap::new(),
            body: None,
            user_id: None,
            ip_address: None,
        };

        let response = router.handle(request);
        assert_eq!(response.status_code, 200);

        // Verify audit log was written
        let entries = audit_log.entries();
        assert_eq!(entries.len(), 1);
        assert_eq!(entries[0].path, "/health");
    }

    #[test]
    fn test_router_not_found() {
        let audit_log = Arc::new(InMemoryAuditLog::new());
        let router = Router::<InMemoryAuditLog>::new(audit_log);

        let request = Request {
            id: "test-2".to_string(),
            method: Method::Get,
            path: "/nonexistent".to_string(),
            headers: std::collections::HashMap::new(),
            body: None,
            user_id: None,
            ip_address: None,
        };

        let response = router.handle(request);
        assert_eq!(response.status_code, 404);
    }

    #[test]
    fn test_audit_log_failure_logged() {
        // RROUTER-2: Test that audit log failures are properly logged
        // (not silently ignored)
        let audit_log = Arc::new(FailingAuditLog::new(true));
        let router = Router::new(audit_log);

        router.add_route(
            Method::Get,
            "/test",
            Box::new(|_| Ok(Response::ok())),
        );

        let request = Request {
            id: "test-3".to_string(),
            method: Method::Get,
            path: "/test".to_string(),
            headers: std::collections::HashMap::new(),
            body: None,
            user_id: None,
            ip_address: None,
        };

        // In non-strict mode, the request should still succeed
        // but the error should be logged (we can't easily test logging here)
        let response = router.handle(request);
        assert_eq!(response.status_code, 200);
    }

    #[test]
    fn test_audit_log_failure_strict_mode() {
        // RROUTER-2: Test that in strict mode, audit log failures cause request failure
        let audit_log = Arc::new(FailingAuditLog::new(true));
        let router = Router::new_strict(audit_log);

        router.add_route(
            Method::Get,
            "/test",
            Box::new(|_| Ok(Response::ok())),
        );

        let request = Request {
            id: "test-4".to_string(),
            method: Method::Get,
            path: "/test".to_string(),
            headers: std::collections::HashMap::new(),
            body: None,
            user_id: None,
            ip_address: None,
        };

        // In strict mode, the request should fail if audit logging fails
        let response = router.handle(request);
        assert_eq!(response.status_code, 500);
    }

    #[test]
    fn test_write_audit_log_returns_error() {
        // RROUTER-2: Test that write_audit_log properly returns errors
        let audit_log = Arc::new(FailingAuditLog::new(true));
        let router = Router::new(audit_log);

        let entry = AuditLogEntry::new(
            "test-5".to_string(),
            "GET".to_string(),
            "/test".to_string(),
        );

        let result = router.write_audit_log(&entry);
        assert!(result.is_err());
        assert!(matches!(result.unwrap_err(), RouterError::AuditLogError(_)));
    }

    #[test]
    fn test_path_matching() {
        let audit_log = Arc::new(InMemoryAuditLog::new());
        let mut router = Router::new(audit_log);

        router.add_route(
            Method::Get,
            "/users/:id",
            Box::new(|_| Ok(Response::ok())),
        );

        let request = Request {
            id: "test-6".to_string(),
            method: Method::Get,
            path: "/users/123".to_string(),
            headers: std::collections::HashMap::new(),
            body: None,
            user_id: None,
            ip_address: None,
        };

        let response = router.handle(request);
        assert_eq!(response.status_code, 200);
    }
}
