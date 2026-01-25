// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
//
// This file implements the command handler registry pattern, breaking up the
// monolithic handleCommand() function into individual, testable command handlers.
package chat

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeranaias/rigrun-tui/internal/commands"
	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/tools"
)

// =============================================================================
// COMMAND HANDLER REGISTRY
// =============================================================================

// CommandHandler is a function that handles a specific command.
// It receives the model and command arguments, and returns an updated model and command.
type CommandHandler func(m *Model, args []string) (tea.Model, tea.Cmd)

// commandHandlers maps command names to their handler functions.
var commandHandlers = map[string]CommandHandler{
	// Help & Meta
	"help": handleHelpCommand,
	"h":    handleHelpCommand,
	"?":    handleHelpCommand,
	"quit": handleQuitCommand,
	"q":    handleQuitCommand,
	"exit": handleQuitCommand,

	// Session Management
	"clear":    handleClearCommand,
	"c":        handleClearCommand,
	"new":      handleNewCommand,
	"n":        handleNewCommand,
	"save":     handleSaveCommand,
	"s":        handleSaveCommand,
	"load":     handleLoadCommand,
	"l":        handleLoadCommand,
	"resume":   handleResumeCommand,
	"r":        handleResumeCommand,
	"list":     handleListCommand,
	"sessions": handleListCommand,
	"search":   handleSearchSessionsCommand,
	"export":   handleExportCommand,
	"e":        handleExportCommand,
	"history":  handleHistoryCommand,
	"hist":     handleHistoryCommand,

	// Security & Compliance
	"audit":    handleAuditCommand,
	"security": handleSecurityCommand,
	"sec":      handleSecurityCommand,
	"classify": handleClassifyCommand,
	"cls":      handleClassifyCommand,
	"consent":  handleConsentCommand,

	// Configuration
	"config": handleConfigCommand,
	"cfg":    handleConfigCommand,
	"model":  handleModelCommand,
	"m":      handleModelCommand,
	"mode":   handleModeCommand,

	// Tools & System
	"tools":     handleToolsCommand,
	"t":         handleToolsCommand,
	"doctor":    handleDoctorCommand,
	"diag":      handleDoctorCommand,
	"models":    handleModelsCommand,
	"streaming": handleStreamingCommand,
	"stream":    handleStreamingCommand,

	// Status & Information
	"status":  handleStatusCommand,
	"cache":   handleCacheCommand,
	"gpu":     handleGpuCommand,
	"version": handleVersionCommand,
	"ver":     handleVersionCommand,
	"tokens":  handleTokensCommand,
	"tok":     handleTokensCommand,
	"context": handleContextCommand,
	"ctx":     handleContextCommand,

	// Background Tasks
	"task":   handleTaskCommand,
	"tasks":  handleTasksCommand,
	"cancel": handleCancelTaskCommand,
}

// handleCommand processes slash commands using the command registry pattern.
func (m Model) handleCommand(content string) (tea.Model, tea.Cmd) {
	m.input.Reset()

	// Parse command and arguments
	parts := strings.Fields(content)
	if len(parts) == 0 {
		return m, nil
	}

	cmdName := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	args := parts[1:]

	// Record tutorial action for specific commands
	var tutorialCmd tea.Cmd
	if m.tutorial != nil && m.tutorial.IsVisible() {
		// Map specific commands to tutorial actions
		switch cmdName {
		case "help":
			tutorialCmd = m.tutorial.RecordAction("help")
		case "models", "model":
			tutorialCmd = m.tutorial.RecordAction("models")
		}
	}

	// Look up handler in registry
	var resultModel tea.Model
	var resultCmd tea.Cmd
	if handler, ok := commandHandlers[cmdName]; ok {
		resultModel, resultCmd = handler(&m, args)
	} else {
		// Unknown command
		m.conversation.AddSystemMessage("Error: Unknown command '" + content + "'\nType /help for available commands")
		m.updateViewport()
		resultModel = m
		resultCmd = nil
	}

	// Batch result command with tutorial command if present
	if tutorialCmd != nil {
		return resultModel, tea.Batch(resultCmd, tutorialCmd)
	}
	return resultModel, resultCmd
}

// =============================================================================
// HELP AND META COMMANDS
// =============================================================================

func handleHelpCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	// Get the mode argument (quick, all, or category name)
	mode := ""
	if len(args) > 0 {
		mode = strings.ToLower(args[0])
	}

	// Create command registry for help generation
	registry := commands.NewRegistry()
	commands.InitializeHandlers(registry)

	// Generate help text using the progressive help system
	helpText := commands.GenerateHelpText(registry, mode)

	// Add contextual tip for new users
	cfg := config.Global()
	if cfg != nil && !cfg.UI.TutorialCompleted {
		helpText += "\n\nTip: You're viewing quick help. Use /help all to see all commands."
	}

	m.conversation.AddSystemMessage(helpText)
	m.updateViewport()
	return m, nil
}

func handleQuitCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	return m, tea.Quit
}

// =============================================================================
// SESSION MANAGEMENT COMMANDS
// =============================================================================

func handleClearCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	m.conversation.ClearHistory()
	m.updateViewport()
	return m, nil
}

func handleNewCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	m.conversation = NewConversation(m)
	m.updateViewport()
	return m, nil
}

func handleSaveCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	name := ""
	if len(args) > 0 {
		name = strings.Join(args, " ")
	}
	return m, func() tea.Msg {
		return SaveConversationMsg{Name: name}
	}
}

func handleLoadCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		return m, func() tea.Msg {
			return ListSessionsMsg{}
		}
	}
	return m, func() tea.Msg {
		return LoadConversationMsg{ID: args[0]}
	}
}

func handleListCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	return m, func() tea.Msg {
		return ListSessionsMsg{}
	}
}

func handleExportCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	format := "markdown"
	if len(args) > 0 {
		format = strings.ToLower(args[0])
		// Support aliases
		if format == "md" {
			format = "markdown"
		} else if format == "htm" {
			format = "html"
		} else if format == "txt" {
			format = "markdown" // txt is just markdown without frontmatter
		}

		// Validate format
		if format != "json" && format != "markdown" && format != "html" {
			m.conversation.AddSystemMessage("Error: Invalid format '" + format + "'\nUsage: /export [markdown|html|json]")
			m.updateViewport()
			return m, nil
		}
	}
	return m, func() tea.Msg {
		return commands.ExportConversationMsg{Format: format}
	}
}

func handleHistoryCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	n := 10
	if len(args) > 0 {
		parsed, err := strconv.Atoi(args[0])
		if err != nil {
			m.conversation.AddSystemMessage("Error: Invalid number '" + args[0] + "'\nUsage: /history [n]")
			m.updateViewport()
			return m, nil
		}
		if parsed <= 0 {
			m.conversation.AddSystemMessage("Error: Number must be positive\nUsage: /history [n]")
			m.updateViewport()
			return m, nil
		}
		if parsed > 10000 {
			m.conversation.AddSystemMessage("Error: Number too large (max 10000)\nUsage: /history [n]")
			m.updateViewport()
			return m, nil
		}
		n = parsed
	}

	historyText := formatHistory(m.conversation, n)
	m.conversation.AddSystemMessage(historyText)
	m.updateViewport()
	return m, nil
}

// =============================================================================
// SECURITY AND COMPLIANCE COMMANDS
// =============================================================================

func handleAuditCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	lines := 10
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			m.conversation.AddSystemMessage("Error: Invalid number '" + args[0] + "'\nUsage: /audit [lines]")
			m.updateViewport()
			return m, nil
		}
		if n <= 0 {
			m.conversation.AddSystemMessage("Error: Number must be positive\nUsage: /audit [lines]")
			m.updateViewport()
			return m, nil
		}
		if n > 1000 {
			m.conversation.AddSystemMessage("Error: Number too large (max 1000)\nUsage: /audit [lines]")
			m.updateViewport()
			return m, nil
		}
		lines = n
	}

	result := getRecentAuditEntries(lines)
	m.conversation.AddSystemMessage(result)
	m.updateViewport()
	return m, nil
}

func handleSecurityCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	result := getSecurityStatus()
	m.conversation.AddSystemMessage(result)
	m.updateViewport()
	return m, nil
}

func handleClassifyCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		cfg := config.Global()
		classLevel := "UNCLASSIFIED"
		if cfg != nil && cfg.Security.Classification != "" {
			classLevel = cfg.Security.Classification
		}
		result := fmt.Sprintf("Current classification level: %s\n\nNote: Classification levels can only be set via CLI:\n  rigrun config security.classification <level>", classLevel)
		m.conversation.AddSystemMessage(result)
	} else {
		result := "Classification levels must be set via CLI for security:\n  rigrun config security.classification <level>\n\nValid levels: UNCLASSIFIED, CUI, CONFIDENTIAL, SECRET, TOP SECRET"
		m.conversation.AddSystemMessage(result)
	}
	m.updateViewport()
	return m, nil
}

func handleConsentCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	result := getConsentStatus()
	m.conversation.AddSystemMessage(result)
	m.updateViewport()
	return m, nil
}

// =============================================================================
// CONFIGURATION COMMANDS
// =============================================================================

func handleConfigCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	cfg := config.Global()
	if len(args) == 0 {
		// PERFORMANCE: strings.Builder avoids quadratic allocations
		var sb strings.Builder
		sb.WriteString("Configuration:\n")
		sb.WriteString("  Model: ")
		sb.WriteString(cfg.DefaultModel)
		sb.WriteString("\n  Mode: ")
		sb.WriteString(cfg.Routing.DefaultMode)
		sb.WriteString("\n  Ollama URL: ")
		sb.WriteString(cfg.Local.OllamaURL)
		sb.WriteByte('\n')
		if cfg.Cloud.OpenRouterKey != "" {
			sb.WriteString("  OpenRouter: configured\n")
		} else {
			sb.WriteString("  OpenRouter: not configured\n")
		}
		sb.WriteString("  Max Tier: ")
		sb.WriteString(cfg.Routing.MaxTier)
		sb.WriteByte('\n')
		if cfg.Routing.ParanoidMode {
			sb.WriteString("  Paranoid Mode: enabled\n")
		}
		if cfg.Routing.OfflineMode {
			sb.WriteString("  Offline Mode: enabled (SC-7)\n")
		}
		sb.WriteString("  Session Timeout: ")
		sb.WriteString(formatInt(cfg.Security.SessionTimeoutSecs))
		sb.WriteString("s\n  Audit Logging: ")
		sb.WriteString(formatBool(cfg.Security.AuditEnabled))
		sb.WriteByte('\n')
		m.conversation.AddSystemMessage(sb.String())
		m.updateViewport()
		return m, nil
	}

	key := strings.ToLower(args[0])
	var value string
	switch key {
	case "model":
		value = cfg.DefaultModel
	case "mode":
		value = cfg.Routing.DefaultMode
	case "url", "ollama_url":
		value = cfg.Local.OllamaURL
	case "max_tier":
		value = cfg.Routing.MaxTier
	case "paranoid":
		value = formatBool(cfg.Routing.ParanoidMode)
	case "offline":
		value = formatBool(cfg.Routing.OfflineMode)
	case "timeout":
		value = formatInt(cfg.Security.SessionTimeoutSecs) + "s"
	case "audit":
		value = formatBool(cfg.Security.AuditEnabled)
	default:
		m.conversation.AddSystemMessage("Error: Unknown config key '" + key + "'")
		m.updateViewport()
		return m, nil
	}
	m.conversation.AddSystemMessage(key + ": " + value)
	m.updateViewport()
	return m, nil
}

func handleModelCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		m.conversation.AddSystemMessage("Current model: " + m.modelName + "\nUsage: /model <name>")
		m.updateViewport()
		return m, nil
	}
	modelName := strings.TrimSpace(args[0])
	return m, func() tea.Msg {
		return OllamaModelSwitchedMsg{Model: modelName}
	}
}

func handleModeCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		modeDisplay := m.routingMode
		if m.routingMode == "auto" {
			modeDisplay = "auto (OpenRouter routing)"
		}
		modeMsg := "Current mode: " + modeDisplay + "\nUsage: /mode <auto|local|cloud>"
		if m.offlineMode {
			modeMsg = "Current mode: local-only (OFFLINE)\nCloud modes disabled - SC-7 Boundary Protection active"
		}
		m.conversation.AddSystemMessage(modeMsg)
		m.updateViewport()
		return m, nil
	}

	mode := strings.ToLower(args[0])
	switch mode {
	case "local":
		m.routingMode = mode
		m.mode = mode
		m.conversation.AddSystemMessage("Routing mode changed to: " + mode)
		m.updateViewport()
		return m, nil

	case "auto", "hybrid":
		if m.offlineMode {
			m.conversation.AddSystemMessage("Error: Cannot switch to '" + mode + "' mode - OFFLINE MODE ACTIVE\n\nSC-7 Boundary Protection is enabled. Network access is restricted to localhost only.\nTo enable cloud features, restart without --no-network flag.")
			m.updateViewport()
			return m, nil
		}
		m.routingMode = "auto"
		m.mode = "auto"
		m.conversation.AddSystemMessage("Routing mode changed to: auto (OpenRouter routing)")
		m.updateViewport()
		return m, nil

	case "cloud":
		if m.offlineMode {
			m.conversation.AddSystemMessage("Error: Cannot switch to '" + mode + "' mode - OFFLINE MODE ACTIVE\n\nSC-7 Boundary Protection is enabled. Network access is restricted to localhost only.\nTo enable cloud features, restart without --no-network flag.")
			m.updateViewport()
			return m, nil
		}
		m.routingMode = mode
		m.mode = mode
		m.conversation.AddSystemMessage("Routing mode changed to: " + mode)
		m.updateViewport()
		return m, nil

	default:
		m.conversation.AddSystemMessage("Error: Invalid mode '" + mode + "'\nUsage: /mode <auto|local|cloud>")
		m.updateViewport()
		return m, nil
	}
}

// =============================================================================
// TOOLS AND SYSTEM COMMANDS
// =============================================================================

func handleToolsCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 || args[0] == "list" {
		allTools := m.toolRegistry.All()
		// PERFORMANCE: strings.Builder avoids quadratic allocations
		var sb strings.Builder
		sb.WriteString("Available Tools:\n")
		for _, tool := range allTools {
			perm := m.toolRegistry.GetPermission(tool.Name)
			permStr := "ask"
			switch perm {
			case 0:
				permStr = "enabled"
			case 2:
				permStr = "disabled"
			}
			fmt.Fprintf(&sb, "  %s - %s (permission: %s, risk: %s)\n",
				tool.Name, tool.GetShortDescription(), permStr, tool.RiskLevel.String())
		}
		m.conversation.AddSystemMessage(sb.String())
		m.updateViewport()
		return m, nil
	}

	action := strings.ToLower(args[0])
	switch action {
	case "enable":
		if len(args) < 2 {
			m.conversation.AddSystemMessage("Error: Missing tool name\nUsage: /tools enable <tool-name>")
			m.updateViewport()
			return m, nil
		}
		toolName := args[1]
		if m.toolRegistry.Get(toolName) == nil {
			m.conversation.AddSystemMessage("Error: Unknown tool '" + toolName + "'")
			m.updateViewport()
			return m, nil
		}
		m.toolRegistry.SetPermissionOverride(toolName, 0)
		m.conversation.AddSystemMessage("Tool '" + toolName + "' enabled (auto-approved)")
		m.updateViewport()
		return m, nil

	case "disable":
		if len(args) < 2 {
			m.conversation.AddSystemMessage("Error: Missing tool name\nUsage: /tools disable <tool-name>")
			m.updateViewport()
			return m, nil
		}
		toolName := args[1]
		if m.toolRegistry.Get(toolName) == nil {
			m.conversation.AddSystemMessage("Error: Unknown tool '" + toolName + "'")
			m.updateViewport()
			return m, nil
		}
		m.toolRegistry.SetPermissionOverride(toolName, 2)
		m.conversation.AddSystemMessage("Tool '" + toolName + "' disabled")
		m.updateViewport()
		return m, nil

	default:
		m.conversation.AddSystemMessage("Error: Invalid action '" + action + "'\nUsage: /tools [list|enable|disable] <tool-name>")
		m.updateViewport()
		return m, nil
	}
}

func handleDoctorCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	// PERFORMANCE: strings.Builder avoids quadratic allocations
	var sb strings.Builder
	sb.WriteString("System Diagnostics:\n\n")

	sb.WriteString("Ollama Connection:\n")
	if m.ollama != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := m.ollama.CheckRunning(ctx)
		cancel()
		if err != nil {
			fmt.Fprintf(&sb, "  Status: OFFLINE\n  Error: %v\n", err)
		} else {
			sb.WriteString("  Status: ONLINE\n")
			sb.WriteString("  URL: http://127.0.0.1:11434\n")
		}
	} else {
		sb.WriteString("  Status: NOT INITIALIZED\n")
	}

	sb.WriteString("\nCloud API Configuration:\n")
	if m.offlineMode {
		sb.WriteString("  Status: OFFLINE MODE (SC-7 Boundary Protection)\n")
		sb.WriteString("  Cloud access is disabled\n")
	} else if m.cloudClient != nil {
		sb.WriteString("  Status: CONFIGURED\n")
		sb.WriteString("  Provider: OpenRouter\n")
	} else {
		sb.WriteString("  Status: NOT CONFIGURED\n")
		sb.WriteString("  Hint: Set RIGRUN_OPENROUTER_KEY environment variable\n")
	}

	sb.WriteString("\nConfiguration:\n")
	cfg := config.Global()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(&sb, "  Status: INVALID\n  Error: %v\n", err)
	} else {
		sb.WriteString("  Status: VALID\n")
		fmt.Fprintf(&sb, "  Default Model: %s\n", cfg.DefaultModel)
		fmt.Fprintf(&sb, "  Routing Mode: %s\n", cfg.Routing.DefaultMode)
	}

	sb.WriteString("\nTool System:\n")
	fmt.Fprintf(&sb, "  Registered Tools: %d\n", len(m.toolRegistry.All()))
	fmt.Fprintf(&sb, "  Tools Enabled: %v\n", m.toolsEnabled)

	m.conversation.AddSystemMessage(sb.String())
	m.updateViewport()
	return m, nil
}

func handleModelsCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 || args[0] == "list" {
		return handleModelsListCommand(m)
	}

	action := strings.ToLower(args[0])
	switch action {
	case "guide", "capabilities", "cap":
		return handleModelsGuideCommand(m)
	case "table", "compare":
		return handleModelsTableCommand(m)
	case "check":
		return handleModelsCheckCommand(m, args)
	case "info":
		return handleModelsInfoCommand(m, args)
	default:
		m.conversation.AddSystemMessage("Error: Invalid action '" + action + "'\nUsage: /models [list|info|guide|table|check] <model-name>")
		m.updateViewport()
		return m, nil
	}
}

func handleStreamingCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		statusMsg := "Streaming is always enabled in the TUI.\n"
		statusMsg += "This provides real-time responses as they are generated.\n"
		statusMsg += "\nNote: Streaming cannot be disabled in interactive mode."
		m.conversation.AddSystemMessage(statusMsg)
		m.updateViewport()
		return m, nil
	}

	action := strings.ToLower(args[0])
	switch action {
	case "on", "enable":
		m.conversation.AddSystemMessage("Streaming is already enabled (default mode)")
		m.updateViewport()
		return m, nil
	case "off", "disable":
		m.conversation.AddSystemMessage("Streaming cannot be disabled in TUI mode\nFor non-streaming mode, use CLI: rigrun ask --no-stream")
		m.updateViewport()
		return m, nil
	default:
		m.conversation.AddSystemMessage("Error: Invalid action '" + action + "'\nUsage: /streaming [on|off]")
		m.updateViewport()
		return m, nil
	}
}

// =============================================================================
// STATUS AND INFORMATION COMMANDS
// =============================================================================

func handleStatusCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	// PERFORMANCE: strings.Builder avoids quadratic allocations
	var sb strings.Builder
	sb.WriteString("Current Status:\n")
	sb.WriteString("  Model: ")
	sb.WriteString(m.modelName)
	sb.WriteByte('\n')
	if m.offlineMode {
		sb.WriteString("  Mode: local-only (OFFLINE - SC-7 Boundary Protection)\n")
		sb.WriteString("  Cloud: disabled\n")
	} else {
		sb.WriteString("  Mode: ")
		sb.WriteString(m.routingMode)
		sb.WriteByte('\n')
	}
	sb.WriteString("  Messages: ")
	sb.WriteString(formatInt(len(m.conversation.GetHistory())))
	sb.WriteByte('\n')
	if m.sessionStats != nil {
		snapshot := m.sessionStats.GetStats()
		sb.WriteString("  Queries: ")
		sb.WriteString(formatInt(snapshot.TotalQueries))
		sb.WriteString("\n  Cache hits: ")
		sb.WriteString(formatInt(snapshot.CacheHits))
		sb.WriteByte('\n')
	}
	m.conversation.AddSystemMessage(sb.String())
	m.updateViewport()
	return m, nil
}

func handleCacheCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	action := "stats"
	if len(args) > 0 {
		action = strings.ToLower(args[0])
	}
	switch action {
	case "stats", "status", "":
		if m.cacheManager == nil {
			m.conversation.AddSystemMessage("Cache: not initialized")
			m.updateViewport()
			return m, nil
		}
		stats := m.cacheManager.Stats()
		// PERFORMANCE: strings.Builder avoids quadratic allocations
		var sb strings.Builder
		sb.WriteString("Cache Statistics:\n")
		sb.WriteString("  Exact Hits: ")
		sb.WriteString(formatInt(stats.ExactHits))
		sb.WriteString("\n  Semantic Hits: ")
		sb.WriteString(formatInt(stats.SemanticHits))
		sb.WriteString("\n  Misses: ")
		sb.WriteString(formatInt(stats.Misses))
		sb.WriteString("\n  Total Lookups: ")
		sb.WriteString(formatInt(stats.TotalLookups))
		hitRate := m.cacheManager.HitRate() * 100
		fmt.Fprintf(&sb, "\n  Hit Rate: %.1f%%\n", hitRate)
		sb.WriteString("  Exact Cache Size: ")
		sb.WriteString(formatInt(m.cacheManager.ExactCacheSize()))
		sb.WriteString(" entries\n  Semantic Cache Size: ")
		sb.WriteString(formatInt(m.cacheManager.SemanticCacheSize()))
		sb.WriteString(" entries\n  Enabled: ")
		sb.WriteString(formatBool(m.cacheManager.IsEnabled()))
		sb.WriteByte('\n')
		m.conversation.AddSystemMessage(sb.String())
		m.updateViewport()
		return m, nil

	case "clear":
		if m.cacheManager == nil {
			m.conversation.AddSystemMessage("Cache: not initialized")
			m.updateViewport()
			return m, nil
		}
		m.cacheManager.Clear()
		m.conversation.AddSystemMessage("Cache cleared successfully")
		m.updateViewport()
		return m, nil

	default:
		m.conversation.AddSystemMessage("Error: Invalid action '" + action + "'\nUsage: /cache [stats|clear]")
		m.updateViewport()
		return m, nil
	}
}

func handleGpuCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	// PERFORMANCE: strings.Builder avoids quadratic allocations
	var sb strings.Builder
	sb.WriteString("GPU Status:\n")
	if m.gpu != "" {
		sb.WriteString("  GPU: ")
		sb.WriteString(m.gpu)
		sb.WriteByte('\n')
	} else {
		sb.WriteString("  GPU: not detected\n")
	}
	if m.ollama != nil {
		sb.WriteString("  Ollama: connected\n")
	} else {
		sb.WriteString("  Ollama: not connected\n")
	}
	m.conversation.AddSystemMessage(sb.String())
	m.updateViewport()
	return m, nil
}

func handleVersionCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	// PERFORMANCE: strings.Builder avoids quadratic allocations
	var sb strings.Builder
	sb.WriteString("Version Information:\n")
	sb.WriteString("  rigrun: 0.1.0\n")
	sb.WriteString("  Go: ")
	sb.WriteString(runtime.Version())
	sb.WriteString("\n  Platform: ")
	sb.WriteString(runtime.GOOS)
	sb.WriteByte('/')
	sb.WriteString(runtime.GOARCH)
	sb.WriteByte('\n')
	m.conversation.AddSystemMessage(sb.String())
	m.updateViewport()
	return m, nil
}

func handleTokensCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	tokensUsed := m.conversation.TokensUsed
	maxTokens := m.conversation.MaxTokens
	contextPercent := m.conversation.ContextPercent

	var tokenInfo strings.Builder
	tokenInfo.WriteString("Token Usage:\n")
	tokenInfo.WriteString("  Total tokens: ")
	tokenInfo.WriteString(formatInt(tokensUsed))
	tokenInfo.WriteString("\n  Max tokens: ")
	tokenInfo.WriteString(formatInt(maxTokens))
	tokenInfo.WriteString("\n  Context used: ")
	tokenInfo.WriteString(formatFloat64(contextPercent))
	tokenInfo.WriteString("%\n")

	if contextPercent >= 90 {
		tokenInfo.WriteString("\n  Status: CRITICAL - Context near limit!")
	} else if contextPercent >= 75 {
		tokenInfo.WriteString("\n  Status: WARNING - Consider starting new conversation")
	} else {
		tokenInfo.WriteString("\n  Status: OK")
	}

	m.conversation.AddSystemMessage(tokenInfo.String())
	m.updateViewport()
	return m, nil
}

func handleContextCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	tokensUsed := m.conversation.TokensUsed
	maxTokens := m.conversation.MaxTokens
	contextPercent := m.conversation.ContextPercent
	remaining := maxTokens - tokensUsed

	var contextInfo strings.Builder
	contextInfo.WriteString("Context Window Information:\n")
	contextInfo.WriteString("  Model: ")
	contextInfo.WriteString(m.modelName)
	contextInfo.WriteString("\n  Current size: ")
	contextInfo.WriteString(formatInt(tokensUsed))
	contextInfo.WriteString(" tokens\n  Max context: ")
	contextInfo.WriteString(formatInt(maxTokens))
	contextInfo.WriteString(" tokens\n  Remaining: ")
	contextInfo.WriteString(formatInt(remaining))
	contextInfo.WriteString(" tokens\n  Usage: ")
	contextInfo.WriteString(formatFloat64(contextPercent))
	contextInfo.WriteString("%\n")

	contextInfo.WriteString("\n  [")
	barWidth := 40
	filled := int(contextPercent * float64(barWidth) / 100)
	if filled > barWidth {
		filled = barWidth
	}
	for i := 0; i < barWidth; i++ {
		if i < filled {
			contextInfo.WriteString("=")
		} else {
			contextInfo.WriteString(" ")
		}
	}
	contextInfo.WriteString("]")

	m.conversation.AddSystemMessage(contextInfo.String())
	m.updateViewport()
	return m, nil
}

// =============================================================================
// MODELS COMMAND HELPERS
// =============================================================================

func handleModelsListCommand(m *Model) (tea.Model, tea.Cmd) {
	// PERFORMANCE: strings.Builder avoids quadratic allocations
	var sb strings.Builder
	sb.WriteString("Available Models:\n\n")

	sb.WriteString("Local Models (Ollama):\n")
	if m.ollama != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		models, err := m.ollama.ListModels(ctx)
		cancel()
		if err != nil {
			fmt.Fprintf(&sb, "  Error: %v\n", err)
		} else if len(models) == 0 {
			sb.WriteString("  No models found\n")
		} else {
			for _, model := range models {
				sizeGB := float64(model.Size) / (1024 * 1024 * 1024)
				rec := tools.CheckAgenticCapability(model.Name)
				agenticStatus := "[OK]"
				if !rec.Suitable {
					agenticStatus = "[!]"
				}
				fmt.Fprintf(&sb, "  - %s (%.1fGB) [Agentic: %s]\n", model.Name, sizeGB, agenticStatus)
			}
		}
	} else {
		sb.WriteString("  Ollama not initialized\n")
	}

	if !m.offlineMode {
		sb.WriteString("\nCloud Models:\n")
		sb.WriteString("  - anthropic/claude-3.5-sonnet (via OpenRouter)\n")
		sb.WriteString("  - anthropic/claude-3-opus (via OpenRouter)\n")
		sb.WriteString("  - anthropic/claude-3-haiku (via OpenRouter)\n")
		sb.WriteString("  - openai/gpt-4o (via OpenRouter)\n")
		sb.WriteString("\nNote: Use 'auto' mode to let OpenRouter select optimal model\n")
	} else {
		sb.WriteString("\nCloud Models: DISABLED (Offline Mode)\n")
	}

	sb.WriteString("\nTip: Use /models guide for capability recommendations")
	m.conversation.AddSystemMessage(sb.String())
	m.updateViewport()
	return m, nil
}

func handleModelsGuideCommand(m *Model) (tea.Model, tea.Cmd) {
	guideMsg := tools.GenerateModelGuide()
	m.conversation.AddSystemMessage(guideMsg)
	m.updateViewport()
	return m, nil
}

func handleModelsTableCommand(m *Model) (tea.Model, tea.Cmd) {
	tableMsg := "Model Capability Comparison:\n\n"
	tableMsg += tools.GenerateModelTable()
	tableMsg += "\nLegend: +=Basic ++=Good +++=Excellent"
	m.conversation.AddSystemMessage(tableMsg)
	m.updateViewport()
	return m, nil
}

func handleModelsCheckCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	if len(args) < 2 {
		m.conversation.AddSystemMessage("Error: Missing model name\nUsage: /models check <model-name>")
		m.updateViewport()
		return m, nil
	}
	modelName := args[1]
	rec := tools.CheckAgenticCapability(modelName)

	// PERFORMANCE: strings.Builder avoids quadratic allocations
	var sb strings.Builder
	fmt.Fprintf(&sb, "Agentic Capability Check: %s\n\n", modelName)
	if rec.Suitable {
		sb.WriteString("[OK] Suitable for agentic tasks\n")
	} else {
		sb.WriteString("[!] WARNING: ")
		sb.WriteString(rec.Warning)
		sb.WriteByte('\n')
	}
	if rec.Recommendation != "" {
		sb.WriteString("\nRecommendation: ")
		sb.WriteString(rec.Recommendation)
		sb.WriteByte('\n')
	}
	if rec.SuggestModel != "" && !rec.Suitable {
		sb.WriteString("Suggested model: ")
		sb.WriteString(rec.SuggestModel)
		sb.WriteByte('\n')
	}
	m.conversation.AddSystemMessage(sb.String())
	m.updateViewport()
	return m, nil
}

func handleModelsInfoCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	if len(args) < 2 {
		m.conversation.AddSystemMessage("Error: Missing model name\nUsage: /models info <model-name>")
		m.updateViewport()
		return m, nil
	}
	modelName := args[1]

	if m.ollama != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		modelInfo, err := m.ollama.GetModel(ctx, modelName)
		cancel()
		if err != nil {
			m.conversation.AddSystemMessage("Error: Failed to get model info - " + err.Error())
		} else {
			// PERFORMANCE: strings.Builder avoids quadratic allocations
			var sb strings.Builder
			fmt.Fprintf(&sb, "Model Information: %s\n\n", modelName)
			fmt.Fprintf(&sb, "Family: %s\n", modelInfo.Details.Family)
			fmt.Fprintf(&sb, "Parameter Size: %s\n", modelInfo.Details.ParameterSize)
			fmt.Fprintf(&sb, "Quantization: %s\n", modelInfo.Details.QuantizationLevel)
			fmt.Fprintf(&sb, "Format: %s\n", modelInfo.Details.Format)

			// Add agentic capability info
			rec := tools.CheckAgenticCapability(modelName)
			sb.WriteString("\nAgentic Capability:\n")
			if rec.Suitable {
				sb.WriteString("  [OK] Suitable for agentic tasks\n")
			} else {
				sb.WriteString("  [!] ")
				sb.WriteString(rec.Warning)
				sb.WriteByte('\n')
				if rec.SuggestModel != "" {
					sb.WriteString("  Suggested: ")
					sb.WriteString(rec.SuggestModel)
					sb.WriteByte('\n')
				}
			}
			m.conversation.AddSystemMessage(sb.String())
		}
	} else {
		m.conversation.AddSystemMessage("Error: Ollama not initialized")
	}
	m.updateViewport()
	return m, nil
}

// =============================================================================
// SESSION RESUME AND SEARCH COMMANDS
// =============================================================================

// handleResumeCommand resumes a saved session with context display.
func handleResumeCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		m.conversation.AddSystemMessage("Error: Session ID required\nUsage: /resume <session-id or #index>\nUse /list to see available sessions")
		m.updateViewport()
		return m, nil
	}
	sessionID := args[0]
	return m, func() tea.Msg {
		return SessionResumeMsg{SessionID: sessionID}
	}
}

// handleSearchSessionsCommand searches sessions by message content.
func handleSearchSessionsCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		m.conversation.AddSystemMessage("Error: Search query required\nUsage: /search <query>")
		m.updateViewport()
		return m, nil
	}
	query := strings.Join(args, " ")
	return m, func() tea.Msg {
		return SessionSearchMsg{Query: query}
	}
}

// SessionSearchMsg requests searching sessions by message content.
type SessionSearchMsg struct {
	Query string
}

// SessionSearchResultMsg contains search results.
type SessionSearchResultMsg struct {
	Query    string
	Sessions []SessionSearchResult
	Error    error
}

// SessionSearchResult contains a single search result.
type SessionSearchResult struct {
	ID           string
	Summary      string
	MessageCount int
	Preview      string
	UpdatedAt    string
}

// =============================================================================
// BACKGROUND TASK COMMANDS
// =============================================================================

// handleTaskCommand creates a new background task.
// Usage: /task "Description" command args...
func handleTaskCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	if len(args) < 2 {
		m.conversation.AddSystemMessage("Error: Task requires description and command\nUsage: /task \"Description\" <command> <args...>\nExample: /task \"Run tests\" bash \"go test ./...\"")
		m.updateViewport()
		return m, nil
	}

	// Parse description (first argument, may be quoted)
	description := args[0]
	command := args[1]
	commandArgs := args[2:]

	return m, func() tea.Msg {
		return TaskCreateMsg{
			Description: description,
			Command:     command,
			Args:        commandArgs,
		}
	}
}

// handleTasksCommand shows the task list.
// Usage: /tasks [--all | --running | --completed]
func handleTasksCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	filter := "default"
	if len(args) > 0 {
		filter = strings.TrimPrefix(args[0], "--")
	}

	return m, func() tea.Msg {
		return TaskListMsg{Filter: filter}
	}
}

// handleCancelTaskCommand cancels a running task.
// Usage: /cancel <task-id>
func handleCancelTaskCommand(m *Model, args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		m.conversation.AddSystemMessage("Error: Task ID required\nUsage: /cancel <task-id>\nUse /tasks to see running tasks")
		m.updateViewport()
		return m, nil
	}

	taskID := args[0]
	return m, func() tea.Msg {
		return TaskCancelMsg{TaskID: taskID}
	}
}

// =============================================================================
// BACKGROUND TASK MESSAGE TYPES
// =============================================================================

// TaskCreateMsg requests creating a new background task.
type TaskCreateMsg struct {
	Description string
	Command     string
	Args        []string
}

// TaskListMsg requests showing the task list.
type TaskListMsg struct {
	Filter string // "default", "all", "running", "completed"
}

// TaskCancelMsg requests canceling a task.
type TaskCancelMsg struct {
	TaskID string
}

// TaskNotificationMsg notifies about a task state change.
type TaskNotificationMsg struct {
	TaskID      string
	Description string
	Status      string
	Duration    time.Duration
	Error       string
}
