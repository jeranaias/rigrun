// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package router provides intelligent query routing for RAG queries.
//
// Routes queries to the appropriate tier based on complexity and classification:
// Cache -> Local LLM -> Cloud (Haiku/Sonnet/Opus)
//
// # Key Types
//
//   - Router: Main router with configuration and state
//   - Tier: Routing tier enumeration (Cache, Local, Cloud variants)
//   - Complexity: Query complexity level (Simple, Medium, Complex)
//   - RoutingDecision: Decision with tier, reason, and cost estimate
//
// # Security
//
// SECURITY CRITICAL: Classification enforcement is ALWAYS the first check.
// CUI and higher classifications MUST route to TierLocal - cloud is NEVER permitted.
// Paranoid mode blocks ALL cloud routing regardless of classification.
//
// # Usage
//
// Create a router and route a query:
//
//	r := router.New(cfg)
//	decision := r.Route(ctx, query, classification)
//	switch decision.Tier {
//	case router.TierCache:
//	    // Use cached response
//	case router.TierLocal:
//	    // Route to local Ollama
//	case router.TierCloudHaiku:
//	    // Route to Claude Haiku
//	}
//
// # Cost Estimation
//
// The router provides cost estimation for cloud tiers to help
// with budget management and cost optimization.
package router
