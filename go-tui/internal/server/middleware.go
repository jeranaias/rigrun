// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package server provides HTTP API server middleware for security, logging, and rate limiting.
//
// Implements DoD STIG-compliant security features including:
//   - Bearer token authentication with constant-time comparison
//   - IP allowlist for access control
//   - CORS headers for cross-origin requests
//   - Rate limiting to prevent abuse
//   - Session timeout management (IL5 compliant)
//   - Security headers (X-Content-Type-Options, X-Frame-Options, etc.)
//   - Request logging with timing information
//   - Panic recovery with stack trace logging
package server

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// ============================================================================
// Auth Configuration and Middleware
// ============================================================================

// AuthConfig contains authentication configuration options.
type AuthConfig struct {
	// Enabled indicates whether authentication is required.
	Enabled bool

	// BearerToken is the expected bearer token for API authentication.
	// If empty and Enabled is true, all requests will be rejected.
	BearerToken string

	// AllowedIPs is a list of IP addresses or CIDR ranges that are allowed access.
	// If empty, all IPs are allowed (subject to token authentication).
	AllowedIPs []string

	// parsedCIDRs caches parsed CIDR networks for efficient lookup.
	parsedCIDRs []*net.IPNet

	// parsedOnce ensures CIDR parsing happens only once.
	parsedOnce sync.Once
}

// DefaultAuthConfig returns a default AuthConfig with authentication disabled.
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		Enabled:     false,
		BearerToken: "",
		AllowedIPs:  []string{},
	}
}

// parseCIDRs parses the AllowedIPs into net.IPNet for efficient matching.
func (c *AuthConfig) parseCIDRs() {
	c.parsedOnce.Do(func() {
		c.parsedCIDRs = make([]*net.IPNet, 0, len(c.AllowedIPs))
		for _, ipStr := range c.AllowedIPs {
			// Check if it's a CIDR notation
			if strings.Contains(ipStr, "/") {
				_, ipNet, err := net.ParseCIDR(ipStr)
				if err == nil {
					c.parsedCIDRs = append(c.parsedCIDRs, ipNet)
				} else {
					log.Printf("AUTH_CONFIG: Invalid CIDR notation: %s", ipStr)
				}
			} else {
				// Single IP - convert to /32 (IPv4) or /128 (IPv6) CIDR
				ip := net.ParseIP(ipStr)
				if ip != nil {
					var mask net.IPMask
					if ip.To4() != nil {
						mask = net.CIDRMask(32, 32)
					} else {
						mask = net.CIDRMask(128, 128)
					}
					c.parsedCIDRs = append(c.parsedCIDRs, &net.IPNet{IP: ip, Mask: mask})
				} else {
					log.Printf("AUTH_CONFIG: Invalid IP address: %s", ipStr)
				}
			}
		}
	})
}

// isIPAllowed checks if the given IP address is in the allowlist.
func (c *AuthConfig) isIPAllowed(ipStr string) bool {
	// If no IPs are specified, allow all
	if len(c.AllowedIPs) == 0 {
		return true
	}

	c.parseCIDRs()

	// Parse the client IP
	ip := net.ParseIP(ipStr)
	if ip == nil {
		log.Printf("AUTH: Could not parse client IP: %s", ipStr)
		return false
	}

	// Check against all allowed CIDRs
	for _, cidr := range c.parsedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}

// AuthMiddleware returns HTTP middleware that authenticates requests.
//
// Authentication checks (in order):
//  1. If authentication is disabled, allow all requests
//  2. Check client IP against allowlist (if configured)
//  3. Check Authorization header for Bearer token
//
// Returns 401 Unauthorized if authentication fails.
// Uses constant-time comparison for token validation to prevent timing attacks.
func AuthMiddleware(config *AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If auth is disabled, pass through
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Check IP allowlist first
			clientIP := GetClientIP(r)
			if !config.isIPAllowed(clientIP) {
				log.Printf("AUTH_DENIED | ip=%s reason=ip_not_allowed", clientIP)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Check Bearer token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Printf("AUTH_DENIED | ip=%s reason=missing_auth_header", clientIP)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Extract Bearer token
			if !strings.HasPrefix(authHeader, "Bearer ") {
				log.Printf("AUTH_DENIED | ip=%s reason=invalid_auth_format", clientIP)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")

			// Validate token using constant-time comparison
			if !ValidateBearerToken(token, config.BearerToken) {
				log.Printf("AUTH_DENIED | ip=%s reason=invalid_token", clientIP)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Authentication successful
			next.ServeHTTP(w, r)
		})
	}
}

// ValidateBearerToken compares tokens using constant-time comparison.
// This prevents timing attacks that could be used to guess the token.
// Returns false if either token is empty.
func ValidateBearerToken(token, expected string) bool {
	// Reject empty tokens
	if token == "" || expected == "" {
		return false
	}

	// Use constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

// ============================================================================
// CORS Configuration and Middleware
// ============================================================================

// CORSConfig contains CORS (Cross-Origin Resource Sharing) configuration.
type CORSConfig struct {
	// AllowedOrigins is a list of allowed origins for CORS requests.
	// Use "*" to allow all origins (not recommended for production).
	AllowedOrigins []string

	// AllowedMethods is a list of allowed HTTP methods.
	AllowedMethods []string

	// AllowedHeaders is a list of allowed request headers.
	AllowedHeaders []string

	// MaxAge is the max age (in seconds) for preflight cache.
	MaxAge int
}

// DefaultCORSConfig returns a default CORS configuration allowing localhost origins.
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowedOrigins: []string{
			"http://localhost",
			"http://localhost:3000",
			"http://localhost:8080",
			"http://127.0.0.1",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:8080",
		},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization", "X-Session-Id"},
		MaxAge:         86400, // 24 hours
	}
}

// isOriginAllowed checks if the origin is in the allowlist.
func (c *CORSConfig) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range c.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
		// Support wildcard subdomain matching (e.g., "*.example.com")
		if strings.HasPrefix(allowed, "*.") {
			domain := strings.TrimPrefix(allowed, "*")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}
	return false
}

// CORSMiddleware returns HTTP middleware that handles CORS headers.
//
// Features:
//   - Validates origin against allowlist
//   - Handles preflight OPTIONS requests
//   - Sets appropriate Access-Control-* headers
func CORSMiddleware(config *CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if config.isOriginAllowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else if len(config.AllowedOrigins) > 0 {
				// Check for wildcard
				for _, allowed := range config.AllowedOrigins {
					if allowed == "*" {
						w.Header().Set("Access-Control-Allow-Origin", "*")
						w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
						w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
						w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))
						break
					}
				}
			}

			// Handle preflight OPTIONS request
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ============================================================================
// Rate Limiter
// ============================================================================

// RateLimiter implements a sliding window rate limiter per IP address.
type RateLimiter struct {
	// requests maps IP addresses to their request timestamps.
	requests map[string][]time.Time

	// limit is the maximum number of requests per window.
	limit int

	// window is the time window for rate limiting.
	window time.Duration

	// mu protects concurrent access to the requests map.
	mu sync.Mutex
}

// NewRateLimiter creates a new RateLimiter with the specified limit and window.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}

	// Start background cleanup goroutine
	go rl.cleanup()

	return rl
}

// DefaultRateLimiter returns a RateLimiter with default settings: 100 requests per minute.
func DefaultRateLimiter() *RateLimiter {
	return NewRateLimiter(100, time.Minute)
}

// Allow checks if a request from the given IP should be allowed.
// Returns true if the request is allowed, false if rate limit is exceeded.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	// Get existing timestamps for this IP
	timestamps := rl.requests[ip]

	// Filter out timestamps outside the current window
	validTimestamps := make([]time.Time, 0, len(timestamps))
	for _, ts := range timestamps {
		if ts.After(windowStart) {
			validTimestamps = append(validTimestamps, ts)
		}
	}

	// Check if we've exceeded the limit
	if len(validTimestamps) >= rl.limit {
		rl.requests[ip] = validTimestamps
		return false
	}

	// Add current request timestamp
	validTimestamps = append(validTimestamps, now)
	rl.requests[ip] = validTimestamps

	return true
}

// cleanup periodically removes old entries from the rate limiter.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		windowStart := now.Add(-rl.window)

		for ip, timestamps := range rl.requests {
			// Filter out old timestamps
			validTimestamps := make([]time.Time, 0, len(timestamps))
			for _, ts := range timestamps {
				if ts.After(windowStart) {
					validTimestamps = append(validTimestamps, ts)
				}
			}

			if len(validTimestamps) == 0 {
				delete(rl.requests, ip)
			} else {
				rl.requests[ip] = validTimestamps
			}
		}
		rl.mu.Unlock()
	}
}

// GetRemaining returns the number of requests remaining for the given IP.
func (rl *RateLimiter) GetRemaining(ip string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.window)

	timestamps := rl.requests[ip]
	count := 0
	for _, ts := range timestamps {
		if ts.After(windowStart) {
			count++
		}
	}

	remaining := rl.limit - count
	if remaining < 0 {
		remaining = 0
	}
	return remaining
}

// RateLimitMiddleware returns HTTP middleware that enforces rate limiting.
//
// Returns 429 Too Many Requests if the rate limit is exceeded.
// Adds X-RateLimit-* headers to all responses.
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := GetClientIP(r)

			// Add rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limiter.limit))
			w.Header().Set("X-RateLimit-Window", limiter.window.String())

			// Check rate limit
			if !limiter.Allow(clientIP) {
				remaining := limiter.GetRemaining(clientIP)
				w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(limiter.window.Seconds())))

				log.Printf("RATE_LIMIT_EXCEEDED | ip=%s limit=%d window=%v", clientIP, limiter.limit, limiter.window)
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			remaining := limiter.GetRemaining(clientIP)
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

			next.ServeHTTP(w, r)
		})
	}
}

// ============================================================================
// Request Logging Middleware
// ============================================================================

// RequestLogger wraps a logger for request logging.
type RequestLogger struct {
	logger *log.Logger
}

// NewRequestLogger creates a new RequestLogger with the given logger.
func NewRequestLogger(logger *log.Logger) *RequestLogger {
	return &RequestLogger{logger: logger}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// newResponseWriter creates a wrapped response writer.
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

// WriteHeader captures the status code before writing it.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware returns HTTP middleware that logs all requests.
//
// Log format: "2024-01-15 14:30:45 | POST /v1/chat/completions | 200 | 1.234s"
func LoggingMiddleware(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap the response writer to capture status code
			wrapped := newResponseWriter(w)

			// Process the request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)

			// Format timestamp
			timestamp := start.Format("2006-01-02 15:04:05")

			// Log the request
			logger.Printf("%s | %s %s | %d | %.3fs",
				timestamp,
				r.Method,
				r.URL.Path,
				wrapped.statusCode,
				duration.Seconds(),
			)
		})
	}
}

// ============================================================================
// Session Timeout Middleware
// ============================================================================

// SessionTimeoutMiddleware returns HTTP middleware that enforces session timeouts.
//
// Implements DoD STIG AC-12 (Session Termination) requirements.
// Adds session-related headers to all responses:
//   - X-Session-Id: The session identifier
//   - X-Session-Expires-In: Seconds until session expires
//   - X-Session-State: Current session state (ACTIVE, WARNING, EXPIRED)
//
// Returns 401 Unauthorized if the session has expired.
func SessionTimeoutMiddleware(manager *security.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract session ID from header
			sessionID := r.Header.Get("X-Session-Id")

			// If no session header provided, allow request but add warning
			if sessionID == "" {
				w.Header().Set("X-Session-Warning", "No session provided")
				next.ServeHTTP(w, r)
				return
			}

			// Get current session
			session := manager.GetSession()

			// Check if session exists and matches
			if session == nil {
				log.Printf("SESSION_INVALID | session=%s reason=no_active_session", sessionID)
				http.Error(w, "Session expired - re-authentication required", http.StatusUnauthorized)
				return
			}

			if session.ID != sessionID {
				log.Printf("SESSION_INVALID | provided=%s expected=%s", sessionID, session.ID)
				http.Error(w, "Invalid session - re-authentication required", http.StatusUnauthorized)
				return
			}

			// Check if session is expired
			if manager.IsExpired() {
				log.Printf("SESSION_EXPIRED | session=%s", sessionID)
				w.Header().Set("X-Session-State", "EXPIRED")
				http.Error(w, "Session expired - re-authentication required", http.StatusUnauthorized)
				return
			}

			// Refresh session activity
			if err := manager.RefreshSession(); err != nil {
				log.Printf("SESSION_REFRESH_FAILED | session=%s error=%v", sessionID, err)
				http.Error(w, "Session expired - re-authentication required", http.StatusUnauthorized)
				return
			}

			// Add session headers to response
			w.Header().Set("X-Session-Id", session.ID)
			w.Header().Set("X-Session-Expires-In", fmt.Sprintf("%d", int(manager.TimeRemaining().Seconds())))
			w.Header().Set("X-Session-State", manager.GetState().String())
			w.Header().Set("X-Session-Timeout-Max", fmt.Sprintf("%d", int(manager.GetTimeout().Seconds())))
			w.Header().Set("X-STIG-Session-Timeout", "DoD-STIG-IL5-Compliant")

			// Add warning header if in warning state
			if manager.GetState() == security.SessionWarning {
				w.Header().Set("X-Session-Warning", fmt.Sprintf("Session expires in %d seconds", int(manager.TimeRemaining().Seconds())))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ============================================================================
// Security Headers Middleware
// ============================================================================

// SecurityHeadersMiddleware returns HTTP middleware that adds security headers.
//
// Headers set:
//   - X-Content-Type-Options: nosniff
//   - X-Frame-Options: DENY
//   - X-XSS-Protection: 1; mode=block
//   - Content-Security-Policy: default-src 'self'
//   - Cache-Control: no-store, no-cache, must-revalidate
func SecurityHeadersMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// Enable XSS filter
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Content Security Policy
			w.Header().Set("Content-Security-Policy", "default-src 'self'")

			// Prevent caching of sensitive responses
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")

			// Strict Transport Security (for HTTPS)
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

			// Referrer Policy
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			next.ServeHTTP(w, r)
		})
	}
}

// ============================================================================
// Recovery Middleware
// ============================================================================

// RecoveryMiddleware returns HTTP middleware that recovers from panics.
//
// Features:
//   - Catches panics in downstream handlers
//   - Logs stack trace for debugging
//   - Returns 500 Internal Server Error to client
//   - Prevents server crash from unhandled panics
func RecoveryMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Get stack trace
					stack := debug.Stack()

					// Log the panic and stack trace
					log.Printf("PANIC_RECOVERED | method=%s path=%s error=%v\n%s",
						r.Method,
						r.URL.Path,
						err,
						string(stack),
					)

					// Return 500 error to client
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// ============================================================================
// Middleware Chain Helper
// ============================================================================

// Chain composes multiple middleware functions into a single middleware.
// Middlewares are applied in the order provided.
//
// Example:
//
//	chain := Chain(
//	    LoggingMiddleware(logger),
//	    AuthMiddleware(authConfig),
//	    RateLimitMiddleware(rateLimiter),
//	)
//	http.Handle("/api", chain(handler))
func Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		// Apply middlewares in reverse order so they execute in order
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

// ============================================================================
// IP Extraction Helper
// ============================================================================

// trustedProxies defines CIDR ranges of trusted proxies that are allowed to set
// X-Forwarded-For and X-Real-IP headers. Only trust these headers when the
// request comes from one of these trusted proxy IPs.
//
// DoD STIG Security: IL5-compliant protection against header spoofing attacks.
// Attackers cannot bypass rate limiting or IP allowlists by setting fake headers.
var trustedProxies = []string{
	"127.0.0.1/32",      // IPv4 localhost
	"::1/128",           // IPv6 localhost
	"10.0.0.0/8",        // Private network (RFC 1918)
	"172.16.0.0/12",     // Private network (RFC 1918)
	"192.168.0.0/16",    // Private network (RFC 1918)
	"fc00::/7",          // IPv6 Unique Local Addresses (RFC 4193)
}

// parsedTrustedProxies caches the parsed CIDR networks for performance.
var parsedTrustedProxies []*net.IPNet
var trustedProxiesOnce sync.Once

// parseTrustedProxies parses the trusted proxy CIDR ranges once.
func parseTrustedProxies() {
	trustedProxiesOnce.Do(func() {
		parsedTrustedProxies = make([]*net.IPNet, 0, len(trustedProxies))
		for _, cidr := range trustedProxies {
			_, ipNet, err := net.ParseCIDR(cidr)
			if err == nil {
				parsedTrustedProxies = append(parsedTrustedProxies, ipNet)
			} else {
				log.Printf("TRUSTED_PROXIES: Invalid CIDR notation: %s", cidr)
			}
		}
	})
}

// isTrustedProxy checks if the given IP address is in the trusted proxy list.
func isTrustedProxy(ipStr string) bool {
	parseTrustedProxies()

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, cidr := range parsedTrustedProxies {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}

// getRemoteIP extracts the IP address from r.RemoteAddr.
// RemoteAddr is in the format "IP:port" or "[IPv6]:port".
func getRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// RemoteAddr might not have a port
		return remoteAddr
	}
	return host
}

// GetClientIP extracts the client IP address from an HTTP request.
//
// Security: Only trusts X-Forwarded-For and X-Real-IP headers when the request
// comes from a trusted proxy (localhost or private network ranges). This prevents
// header spoofing attacks that could bypass rate limiting or IP allowlists.
//
// DoD STIG IL5 Compliant: Protects against header injection attacks.
//
// Process:
//  1. Extract the direct connection IP from RemoteAddr
//  2. If the connection is from a trusted proxy, check forwarded headers:
//     a. X-Forwarded-For (validate IP format, use first IP in list)
//     b. X-Real-IP (validate IP format)
//  3. Fall back to connection IP (RemoteAddr) if no valid forwarded header
//
// Returns: The validated client IP address.
func GetClientIP(r *http.Request) string {
	// Always get the direct connection IP first
	connIP := getRemoteIP(r.RemoteAddr)

	// Only trust forwarded headers if the connection is from a trusted proxy
	if !isTrustedProxy(connIP) {
		// Direct connection from untrusted source - use connection IP only
		return connIP
	}

	// Connection is from trusted proxy - check forwarded headers

	// Check X-Forwarded-For header (may contain multiple IPs)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// The first IP is the original client
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			// Validate that it's a valid IP address to prevent injection
			if net.ParseIP(clientIP) != nil {
				return clientIP
			}
		}
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		realIP := strings.TrimSpace(xri)
		// Validate that it's a valid IP address to prevent injection
		if net.ParseIP(realIP) != nil {
			return realIP
		}
	}

	// Fall back to connection IP if no valid forwarded headers
	return connIP
}

// ============================================================================
// Response Recorder for Testing
// ============================================================================

// ResponseRecorder is a response writer that records the response for testing.
type ResponseRecorder struct {
	http.ResponseWriter
	StatusCode int
	Body       []byte
}

// NewResponseRecorder creates a new ResponseRecorder.
func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{
		ResponseWriter: w,
		StatusCode:     http.StatusOK,
	}
}

// WriteHeader records the status code.
func (r *ResponseRecorder) WriteHeader(code int) {
	r.StatusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Write records the body.
func (r *ResponseRecorder) Write(b []byte) (int, error) {
	r.Body = append(r.Body, b...)
	return r.ResponseWriter.Write(b)
}
