// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package server provides an HTTP API server with OpenAI-compatible endpoints.
//
// This package implements a REST API server that exposes LLM functionality through
// OpenAI-compatible endpoints, enabling integration with external applications.
//
// # Endpoints
//
//   - POST /v1/chat/completions - OpenAI-compatible chat completions
//   - GET  /v1/models          - List available models
//   - GET  /health             - Health check
//   - GET  /stats              - Usage statistics
//   - GET  /cache/stats        - Cache statistics
//   - POST /cache/clear        - Clear cache
//
// # Security Features (DoD STIG Compliant)
//
//   - Bearer token authentication with constant-time comparison
//   - IP allowlist for access control
//   - CORS headers for cross-origin requests
//   - Rate limiting to prevent abuse
//   - Session timeout management (IL5 compliant)
//   - Security headers (X-Content-Type-Options, X-Frame-Options, etc.)
//
// # Key Types
//
//   - Server: HTTP server with router and middleware
//   - Config: Server configuration including auth and rate limits
//
// # Usage
//
//	cfg := server.Config{
//		Addr:      ":8080",
//		AuthToken: "secret-token",
//	}
//	srv := server.New(cfg, router)
//	if err := srv.ListenAndServe(); err != nil {
//		log.Fatal(err)
//	}
package server
