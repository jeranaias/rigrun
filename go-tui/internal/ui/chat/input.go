// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
//
// This file contains input submission logic, broken down into smaller,
// testable functions from the original giant submitInput() function.
package chat

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeranaias/rigrun-tui/internal/cache"
	"github.com/jeranaias/rigrun-tui/internal/config"
	ctxmention "github.com/jeranaias/rigrun-tui/internal/context"
	"github.com/jeranaias/rigrun-tui/internal/model"
	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/tools"
)

// =============================================================================
// INPUT SUBMISSION - REFACTORED
// =============================================================================

// submitInput is the main entry point for input submission.
// It coordinates the entire pipeline: validation -> command check -> context expansion ->
// cache check -> routing -> streaming.
func (m Model) submitInput() (tea.Model, tea.Cmd) {
	content := strings.TrimSpace(m.input.Value())
	if content == "" {
		return m, nil
	}

	// Check for commands first
	if strings.HasPrefix(content, "/") {
		return m.handleCommand(content)
	}

	// Record tutorial action for regular messages
	var tutorialCmd tea.Cmd
	if m.tutorial != nil && m.tutorial.IsVisible() {
		tutorialCmd = m.tutorial.RecordAction("message")
	}

	// Clear input
	m.input.Reset()

	// Process context expansion (@mentions)
	displayContent, expandedContent, contextInfo := m.expandContextMentions(content)

	// Check cache before routing
	if cachedResponse, hitType := m.checkCache(displayContent); hitType != cache.CacheHitNone {
		return m.handleCacheHit(displayContent, cachedResponse, hitType)
	}

	// Cache miss - proceed with routing
	m.lastCacheHit = cache.CacheHitNone

	// Get routing decision
	decision := m.makeRoutingDecision(expandedContent)
	m.lastRouting = &decision

	// Add user message to conversation
	m.conversation.AddUserMessage(displayContent)

	// Create assistant message for streaming
	assistantMsg := m.conversation.AddAssistantMessage()
	assistantMsg.RoutingTier = decision.Tier.String()
	assistantMsg.RoutingCost = decision.EstimatedCostCents
	if contextInfo != "" {
		assistantMsg.ContextInfo = contextInfo
	}

	// Store pending query for caching on completion
	m.pendingQuery = displayContent
	m.pendingMsgID = assistantMsg.ID

	// Update viewport
	m.updateViewport()
	m.viewport.GotoBottom()

	// Track query start time for session stats
	m.currentQueryStart = time.Now()

	// Route to appropriate backend
	updatedModel, routeCmd := m.routeQuery(assistantMsg, decision, expandedContent)

	// Batch with tutorial command if present
	if tutorialCmd != nil {
		return updatedModel, tea.Batch(routeCmd, tutorialCmd)
	}
	return updatedModel, routeCmd
}

// =============================================================================
// CONTEXT EXPANSION
// =============================================================================

// expandContextMentions processes @ mentions in the user input.
// Returns: (displayContent, expandedContent, contextInfo)
// - displayContent: what to show in the UI (clean message)
// - expandedContent: what to send to the LLM (with context)
// - contextInfo: summary of expanded context for display
func (m *Model) expandContextMentions(content string) (string, string, string) {
	displayContent := content
	expandedContent := content
	contextInfo := ""

	if !ctxmention.HasMentions(content) || m.contextExpander == nil {
		return displayContent, expandedContent, contextInfo
	}

	result := m.contextExpander.Expand(content)

	// Store context summary for display
	if result.Summary.TotalCount > 0 {
		contextInfo = result.Summary.FormatSummary()
	}

	// Use clean message for display (mentions removed)
	displayContent = result.CleanMessage
	if displayContent == "" {
		displayContent = "(context only)"
	}

	// Use expanded message for LLM (with context prepended)
	expandedContent = result.ExpandedMessage

	// Log any errors fetching context as system message
	if result.HasErrors() {
		m.conversation.AddSystemMessage("Context warning: " + result.ErrorSummary())
		m.updateViewport()
	}

	return displayContent, expandedContent, contextInfo
}

// =============================================================================
// CACHE HANDLING
// =============================================================================

// handleCacheHit handles a successful cache lookup.
// It adds the cached response to the conversation without streaming.
func (m Model) handleCacheHit(query, cachedResponse string, hitType cache.CacheHitType) (tea.Model, tea.Cmd) {
	m.lastCacheHit = hitType

	// Add user message (display version for UI)
	m.conversation.AddUserMessage(query)

	// Add assistant message with cached response (not streaming)
	assistantMsg := m.conversation.AddAssistantMessage()
	assistantMsg.Content = cachedResponse
	assistantMsg.IsStreaming = false

	// Set routing info to show cache hit
	if hitType == cache.CacheHitExact {
		assistantMsg.RoutingTier = "Cache (Exact)"
	} else {
		assistantMsg.RoutingTier = "Cache (Semantic)"
	}
	assistantMsg.RoutingCost = 0 // Cache hits are free!
	assistantMsg.CacheHitType = hitType.String()

	// Create routing decision for display
	m.lastRouting = &router.RoutingDecision{
		Tier:               router.TierCache,
		Complexity:         router.ComplexityTrivial,
		EstimatedCostCents: 0,
		Reason:             "Cache " + hitType.String() + " hit - instant response",
	}

	// Record cache hit in session stats
	if m.sessionStats != nil {
		result := router.NewCacheHitResult(cachedResponse, 1) // ~1ms latency for cache
		m.sessionStats.RecordQuery(result)
	}

	// Update viewport and scroll to bottom
	m.updateViewport()
	m.viewport.GotoBottom()

	// Focus input for next query
	m.input.Focus()

	return m, textinput.Blink
}

// =============================================================================
// ROUTING DECISION
// =============================================================================

// makeRoutingDecision determines which tier to use based on routing mode and content.
// Classification enforcement ensures CUI+ data stays on-premise (NIST AC-4).
//
// SECURITY CRITICAL (AC-4): This function enforces information flow control.
// CUI and higher classifications MUST NEVER be routed to cloud services.
// The ClassificationEnforcer provides the hard security boundary.
func (m *Model) makeRoutingDecision(content string) router.RoutingDecision {
	cfg := config.Global()

	// Get classification level from the model (set via SetClassificationLevel)
	// This integrates with the classification UI when available
	classification := m.classificationLevel

	// ==========================================================================
	// CRITICAL AC-4 ENFORCEMENT: Check if classification blocks cloud routing
	// ==========================================================================
	// If classification is CUI or higher, we MUST force local routing.
	// This check happens BEFORE any routing mode logic to ensure it cannot be bypassed.
	if m.classificationEnforcer != nil && m.classificationEnforcer.RequiresLocalOnly(classification) {
		// Classification blocks cloud - force local tier with AC-4 enforcement
		localTier := router.TierLocal
		decision := router.RouteQueryDetailed(content, classification, &localTier)

		// Log the AC-4 enforcement (audit trail)
		// The enforcer logs via EnforceRouting, but we also update the reason
		decision.Reason = "AC-4 ENFORCED: " + classification.String() + " classification blocks cloud routing - " + decision.Reason

		return decision
	}

	// ==========================================================================
	// Standard routing logic (only reached for UNCLASSIFIED data)
	// ==========================================================================
	switch m.routingMode {
	case "local":
		// Force local tier
		localTier := router.TierLocal
		return router.RouteQueryDetailed(content, classification, &localTier)

	case "cloud":
		// Force cloud tier (no cap) - only allowed for UNCLASSIFIED
		decision := router.RouteQueryDetailed(content, classification, nil)
		// Ensure at least cloud tier for cloud mode
		if decision.Tier.IsLocal() {
			decision.Tier = router.TierCloud
		}

		// Double-check enforcement (defense in depth)
		decision = m.enforceClassificationOnDecision(decision, classification)
		return decision

	case "auto", "hybrid":
		// Auto mode: let OpenRouter decide the best model
		routerOpts := &router.RouterOptions{
			Mode:            m.routingMode,
			MaxTier:         cfg.Routing.MaxTier,
			Paranoid:        cfg.Routing.ParanoidMode || m.offlineMode,
			HasCloudKey:     m.HasCloudClient(),
			AutoPreferLocal: cfg.Routing.AutoPreferLocal,
			AutoMaxCost:     cfg.Routing.AutoMaxCost,
			AutoFallback:    cfg.Routing.AutoFallback,
		}
		decision := router.RouteQueryDetailed(content, classification, routerOpts)

		// Double-check enforcement (defense in depth)
		decision = m.enforceClassificationOnDecision(decision, classification)
		return decision

	default:
		// Unknown mode - use auto routing if cloud available, else local
		decision := router.RouteQueryDetailed(content, classification, nil)

		// Double-check enforcement (defense in depth)
		decision = m.enforceClassificationOnDecision(decision, classification)
		return decision
	}
}

// enforceClassificationOnDecision applies AC-4 classification enforcement to a routing decision.
// This is a defense-in-depth check that ensures no cloud routing for CUI+ data.
// Returns the modified decision (forced to local if classification blocks cloud).
func (m *Model) enforceClassificationOnDecision(decision router.RoutingDecision, classification security.ClassificationLevel) router.RoutingDecision {
	if m.classificationEnforcer == nil {
		return decision
	}

	// Check if the decision's tier is a cloud tier
	isCloudTier := !decision.Tier.IsLocal()

	if isCloudTier {
		// Convert router.Tier to security.RoutingTier for enforcement
		var routingTier security.RoutingTier
		if decision.Tier.IsLocal() {
			routingTier = security.RoutingTierLocal
		} else {
			routingTier = security.RoutingTierCloud
		}

		// Enforce classification restrictions
		enforcedTier, err := m.classificationEnforcer.EnforceRouting(classification, routingTier)
		if err != nil {
			// Classification blocked cloud - force local
			decision.Tier = router.TierLocal
			decision.EstimatedCostCents = 0 // Local is free
			decision.Reason = "AC-4 ENFORCED: " + decision.Reason + " (blocked: " + err.Error() + ")"
		} else if enforcedTier == security.RoutingTierLocal && routingTier == security.RoutingTierCloud {
			// Enforcer downgraded to local
			decision.Tier = router.TierLocal
			decision.EstimatedCostCents = 0
			decision.Reason = "AC-4 ENFORCED: " + decision.Reason
		}
	}

	return decision
}

// =============================================================================
// QUERY ROUTING
// =============================================================================

// routeQuery routes the query to the appropriate backend based on the routing decision.
func (m Model) routeQuery(assistantMsg *model.Message, decision router.RoutingDecision, expandedContent string) (tea.Model, tea.Cmd) {
	cfg := config.Global()

	switch decision.Tier {
	case router.TierCache:
		// Cache tier selected but we had a cache miss above, fallback to local
		assistantMsg.RoutingTier = "Local (cache miss)"
		assistantMsg.RoutingCost = 0
		m.currentQueryTier = router.TierLocal
		return m, m.startStreamingLocalWithContent(assistantMsg.ID, expandedContent)

	case router.TierLocal:
		// Use Ollama for local inference
		m.currentQueryTier = router.TierLocal
		return m, m.startStreamingLocalWithContent(assistantMsg.ID, expandedContent)

	case router.TierAuto:
		return m.routeAutoTier(assistantMsg, expandedContent, cfg)

	case router.TierCloud, router.TierHaiku, router.TierSonnet, router.TierOpus, router.TierGpt4o:
		return m.routeCloudTier(assistantMsg, decision, expandedContent)

	default:
		// Unknown tier - fallback to local
		assistantMsg.RoutingTier = "Local"
		assistantMsg.RoutingCost = 0
		m.currentQueryTier = router.TierLocal
		return m, m.startStreamingLocalWithContent(assistantMsg.ID, expandedContent)
	}
}

// routeAutoTier handles routing for auto tier.
func (m Model) routeAutoTier(assistantMsg *model.Message, expandedContent string, cfg *config.Config) (tea.Model, tea.Cmd) {
	// Auto mode: let OpenRouter decide the best model
	if m.HasCloudClient() {
		assistantMsg.RoutingTier = "Auto (OpenRouter)"
		m.currentQueryTier = router.TierAuto
		return m, m.startStreamingCloudWithContent(assistantMsg.ID, "auto", "Auto", expandedContent)
	}

	// No cloud client - check fallback setting
	if cfg.Routing.AutoFallback == "error" {
		assistantMsg.RoutingTier = "Error (no cloud key)"
		m.conversation.AddSystemMessage("Error: Auto mode requires OpenRouter API key. Set with: rigrun config cloud.openrouter_key <key>")
		m.updateViewport()
		return m, nil
	}

	// Fallback to local
	assistantMsg.RoutingTier = "Local (auto fallback)"
	assistantMsg.RoutingCost = 0
	m.currentQueryTier = router.TierLocal
	return m, m.startStreamingLocalWithContent(assistantMsg.ID, expandedContent)
}

// routeCloudTier handles routing for cloud tiers.
func (m Model) routeCloudTier(assistantMsg *model.Message, decision router.RoutingDecision, expandedContent string) (tea.Model, tea.Cmd) {
	// Cloud tiers - use OpenRouter if configured, fallback to local
	if m.HasCloudClient() {
		cloudModel := m.tierToCloudModel(decision.Tier)
		m.currentQueryTier = decision.Tier
		return m, m.startStreamingCloudWithContent(assistantMsg.ID, cloudModel, decision.Tier.String(), expandedContent)
	}

	// No cloud client - fallback to local with warning
	assistantMsg.RoutingTier = "Local (cloud unavailable)"
	assistantMsg.RoutingCost = 0
	m.currentQueryTier = router.TierLocal
	return m, m.startStreamingLocalWithContent(assistantMsg.ID, expandedContent)
}

// =============================================================================
// CONVERSATION HELPER
// =============================================================================

// NewConversation creates a new conversation with proper initialization.
func NewConversation(m *Model) *model.Conversation {
	conv := model.NewConversation()
	if m.toolsEnabled {
		conv.SystemPrompt = tools.GenerateMinimalToolPrompt()
	}
	return conv
}
