// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package commands provides the slash command system for the TUI.
package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeranaias/rigrun-tui/internal/model"
)

// =============================================================================
// HANDLER CONTEXT
// =============================================================================

// HandlerContext provides access to application state for command handlers.
// This is populated by the main application when executing commands.
type HandlerContext struct {
	// CurrentModel is the currently selected model
	CurrentModel string

	// CurrentMode is the current routing mode (local, cloud, hybrid)
	CurrentMode string

	// ConversationID is the current conversation ID
	ConversationID string

	// Messages in the current conversation
	Messages []interface{}

	// LastResponse is the last assistant response (for /copy)
	LastResponse string

	// LastError is the last error message (for @error)
	LastError string

	// WorkingDirectory for file operations
	WorkingDirectory string

	// AvailableModels from Ollama
	AvailableModels []string

	// SavedSessions list
	SavedSessions []SessionInfo
}

// SessionInfo contains metadata about a saved session.
type SessionInfo struct {
	ID        string
	Title     string
	Model     string
	Preview   string
	UpdatedAt string
	MsgCount  int
}

// =============================================================================
// MESSAGE TYPES
// =============================================================================

// These messages are sent by command handlers to update the application state.

// ShowHelpMsg triggers the help display.
type ShowHelpMsg struct {
	Topic string // Optional topic for specific help
}

// ModelSwitchMsg indicates a model switch request.
type ModelSwitchMsg struct {
	Model   string // The model ID to switch to
	Message string // Optional message to display after switching
	Error   error
}

// ModeSwitchMsg indicates a mode switch request.
type ModeSwitchMsg struct {
	Mode  string // "local", "cloud", "hybrid"
	Error error
}

// SaveConversationMsg triggers saving the current conversation.
type SaveConversationMsg struct {
	Name string // Optional name/summary
}

// SaveCompleteMsg indicates save completion.
type SaveCompleteMsg struct {
	ID    string
	Name  string
	Error error
}

// LoadConversationMsg triggers loading a conversation.
type LoadConversationMsg struct {
	ID string
}

// LoadCompleteMsg indicates load completion.
type LoadCompleteMsg struct {
	ID    string
	Error error
}

// ListSessionsMsg triggers showing the session list.
type ListSessionsMsg struct{}

// ClearConversationMsg triggers clearing the conversation.
type ClearConversationMsg struct{}

// ShowStatusMsg triggers showing detailed status.
type ShowStatusMsg struct{}

// CopyToClipboardMsg triggers copying to clipboard.
type CopyToClipboardMsg struct {
	Content string
}

// CopyCompleteMsg indicates copy completion.
type CopyCompleteMsg struct {
	Success bool
	Error   error
}

// ExportConversationMsg triggers exporting the conversation.
type ExportConversationMsg struct {
	Format string // "json", "md", "txt"
}

// ExportCompleteMsg indicates export completion.
type ExportCompleteMsg struct {
	Path  string
	Error error
}

// ShowModelsMsg triggers showing the model list.
type ShowModelsMsg struct {
	Models []string
}

// ShowToolsMsg triggers showing the tools list.
type ShowToolsMsg struct{}

// ToggleToolMsg toggles a tool on/off.
type ToggleToolMsg struct {
	Tool  string
	State bool // true = on, false = off
}

// ShowTutorialMsg triggers showing the tutorial overlay.
type ShowTutorialMsg struct{}

// CreatePlanMsg triggers creating a new plan from a task description.
type CreatePlanMsg struct {
	Task string
}

// PlanCreatedMsg indicates a plan was created and is ready for approval.
type PlanCreatedMsg struct {
	Plan  interface{} // *plan.Plan (using interface{} to avoid import cycle)
	Error error
}

// ApprovePlanMsg triggers plan approval and execution start.
type ApprovePlanMsg struct {
	PlanID string
}

// PausePlanMsg triggers pausing plan execution.
type PausePlanMsg struct {
	PlanID string
}

// ResumePlanMsg triggers resuming plan execution.
type ResumePlanMsg struct {
	PlanID string
}

// CancelPlanMsg triggers cancelling plan execution.
type CancelPlanMsg struct {
	PlanID string
}

// EditPlanMsg triggers showing the plan editor.
type EditPlanMsg struct {
	PlanID string
}

// PlanProgressMsg updates plan execution progress.
type PlanProgressMsg struct {
	PlanID      string
	CurrentStep int
	TotalSteps  int
	Status      string
}

// PlanCompleteMsg indicates plan execution completed.
type PlanCompleteMsg struct {
	PlanID  string
	Success bool
	Error   error
}

// ShowConfigMsg triggers showing configuration.
type ShowConfigMsg struct {
	Key   string // Optional specific key
	Value string // For setting
}

// ConfigUpdateMsg indicates a config value was updated.
type ConfigUpdateMsg struct {
	Key      string
	Value    interface{}
	OldValue interface{}
	Error    error
}

// CacheStatsMsg contains cache statistics.
type CacheStatsMsg struct {
	Enabled       bool
	ExactHits     int
	SemanticHits  int
	Misses        int
	TotalLookups  int
	HitRate       float64
	ExactSize     int
	SemanticSize  int
}

// CacheClearMsg triggers clearing the cache.
type CacheClearMsg struct{}

// CacheClearCompleteMsg indicates cache was cleared.
type CacheClearCompleteMsg struct {
	Error error
}

// CacheToggleMsg toggles cache on/off.
type CacheToggleMsg struct {
	Enabled bool
}

// ErrorMsg indicates an error occurred.
type ErrorMsg struct {
	Title   string
	Message string
	Tip     string
}

// SystemMessageMsg adds a system message to the chat.
type SystemMessageMsg struct {
	Content string
}

// =============================================================================
// HANDLER IMPLEMENTATIONS
// =============================================================================

// InitializeHandlers sets up all command handlers in the registry.
// This should be called after NewRegistry() to wire up the actual handlers.
func InitializeHandlers(r *Registry) {
	// Navigation
	if cmd := r.Get("/help"); cmd != nil {
		cmd.Handler = HandleHelp
	}
	if cmd := r.Get("/quit"); cmd != nil {
		cmd.Handler = HandleQuit
	}

	// Conversation
	if cmd := r.Get("/new"); cmd != nil {
		cmd.Handler = HandleNew
	}
	if cmd := r.Get("/save"); cmd != nil {
		cmd.Handler = HandleSave
	}
	if cmd := r.Get("/load"); cmd != nil {
		cmd.Handler = HandleLoad
	}
	if cmd := r.Get("/clear"); cmd != nil {
		cmd.Handler = HandleClear
	}
	if cmd := r.Get("/copy"); cmd != nil {
		cmd.Handler = HandleCopy
	}
	if cmd := r.Get("/export"); cmd != nil {
		cmd.Handler = HandleExport
	}
	if cmd := r.Get("/sessions"); cmd != nil {
		cmd.Handler = HandleSessions
	}

	// Model
	if cmd := r.Get("/model"); cmd != nil {
		cmd.Handler = HandleModel
	}
	if cmd := r.Get("/models"); cmd != nil {
		cmd.Handler = HandleModels
	}
	if cmd := r.Get("/mode"); cmd != nil {
		cmd.Handler = HandleMode
	}

	// Tools
	if cmd := r.Get("/tools"); cmd != nil {
		cmd.Handler = HandleTools
	}
	if cmd := r.Get("/tool"); cmd != nil {
		cmd.Handler = HandleTool
	}

	// Settings
	if cmd := r.Get("/config"); cmd != nil {
		cmd.Handler = HandleConfig
	}
	if cmd := r.Get("/status"); cmd != nil {
		cmd.Handler = HandleStatus
	}
}

// HandleHelp shows help information.
func HandleHelp(ctx *Context, args []string) tea.Cmd {
	topic := ""
	if len(args) > 0 {
		topic = args[0]
	}
	return func() tea.Msg {
		return ShowHelpMsg{Topic: topic}
	}
}

// HandleQuit exits the application.
func HandleQuit(ctx *Context, args []string) tea.Cmd {
	return tea.Quit
}

// HandleNew starts a new conversation.
func HandleNew(ctx *Context, args []string) tea.Cmd {
	return func() tea.Msg {
		return ClearConversationMsg{}
	}
}

// HandleSave saves the current conversation.
func HandleSave(ctx *Context, args []string) tea.Cmd {
	name := ""
	if len(args) > 0 {
		name = strings.Join(args, " ")
	}
	return func() tea.Msg {
		return SaveConversationMsg{Name: name}
	}
}

// ConversationLoadedMsg contains the loaded conversation data.
type ConversationLoadedMsg struct {
	ID       string
	Summary  string
	Model    string
	Messages []StoredMessage
	Error    error
}

// StoredMessage mirrors storage.StoredMessage for handler output.
type StoredMessage struct {
	Role      string
	Content   string
	Timestamp string
}

// HandleLoad loads a saved conversation.
func HandleLoad(ctx *Context, args []string) tea.Cmd {
	if len(args) == 0 {
		// Show session list instead
		return HandleSessions(ctx, args)
	}

	sessionID := args[0]

	if ctx != nil && ctx.Storage != nil {
		store := ctx.Storage
		return func() tea.Msg {
			conv, err := store.Load(sessionID)
			if err != nil {
				return ConversationLoadedMsg{ID: sessionID, Error: err}
			}

			// Convert messages
			messages := make([]StoredMessage, len(conv.Messages))
			for i, m := range conv.Messages {
				messages[i] = StoredMessage{
					Role:      m.Role,
					Content:   m.Content,
					Timestamp: m.Timestamp.Format("15:04:05"),
				}
			}

			return ConversationLoadedMsg{
				ID:       conv.ID,
				Summary:  conv.Summary,
				Model:    conv.Model,
				Messages: messages,
			}
		}
	}

	return func() tea.Msg {
		return LoadConversationMsg{ID: sessionID}
	}
}

// HandleClear clears the conversation history.
func HandleClear(ctx *Context, args []string) tea.Cmd {
	return func() tea.Msg {
		return ClearConversationMsg{}
	}
}

// HandleCopy copies the last response to clipboard.
func HandleCopy(ctx *Context, args []string) tea.Cmd {
	return func() tea.Msg {
		// The actual content will be filled by the app
		return CopyToClipboardMsg{}
	}
}

// HandleExport exports the conversation.
func HandleExport(ctx *Context, args []string) tea.Cmd {
	format := "markdown" // Default to markdown
	if len(args) > 0 {
		format = strings.ToLower(args[0])
		// Support aliases
		if format == "md" {
			format = "markdown"
		} else if format == "htm" {
			format = "html"
		}
	}

	// Validate format
	switch format {
	case "markdown", "html", "json":
		// Valid formats
	default:
		return func() tea.Msg {
			return ErrorMsg{
				Title:   "Invalid export format",
				Message: fmt.Sprintf("Unknown format: %s", format),
				Tip:     "Supported formats: markdown, html, json",
			}
		}
	}

	return func() tea.Msg {
		return ExportConversationMsg{Format: format}
	}
}

// SessionListMsg contains the list of available sessions.
type SessionListMsg struct {
	Sessions []SessionInfo
	Error    error
}

// HandleSessions shows the session list.
func HandleSessions(ctx *Context, args []string) tea.Cmd {
	if ctx != nil && ctx.Storage != nil {
		store := ctx.Storage
		return func() tea.Msg {
			metas, err := store.List()
			if err != nil {
				return SessionListMsg{Error: err}
			}

			// Convert to SessionInfo
			sessions := make([]SessionInfo, len(metas))
			for i, m := range metas {
				sessions[i] = SessionInfo{
					ID:        m.ID,
					Title:     m.Summary,
					Model:     m.Model,
					Preview:   m.Preview,
					UpdatedAt: m.UpdatedAt.Format("2006-01-02 15:04"),
					MsgCount:  m.MessageCount,
				}
			}

			return SessionListMsg{Sessions: sessions}
		}
	}
	return func() tea.Msg {
		return ListSessionsMsg{}
	}
}

// HandleModel switches or shows the current model.
// When called without arguments, lists available models with their descriptions.
// When called with a model name, switches to that model.
func HandleModel(ctx *Context, args []string) tea.Cmd {
	if len(args) == 0 {
		// Show available models with descriptions
		var sb strings.Builder
		sb.WriteString("Available models:\n\n")

		// Get current model for marking
		var currentModel string
		if ctx != nil && ctx.Ollama != nil {
			currentModel = ctx.Ollama.GetDefaultModel()
		}

		// Group models by provider
		providers := []string{"Anthropic", "OpenAI", "Local"}
		for _, provider := range providers {
			providerModels := []string{}
			for shortName, info := range model.Models {
				if info.Provider == provider {
					providerModels = append(providerModels, shortName)
				}
			}

			if len(providerModels) == 0 {
				continue
			}

			sb.WriteString(fmt.Sprintf("  %s:\n", provider))
			for _, shortName := range providerModels {
				info := model.Models[shortName]
				current := ""
				// Check if this is the current model (by short name or ID)
				if shortName == currentModel || info.ID == currentModel ||
					strings.Contains(currentModel, shortName) {
					current = " (current)"
				}
				sb.WriteString(fmt.Sprintf("    %s - %s [%s]%s\n",
					shortName,
					info.Description,
					info.Tier,
					current,
				))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("Usage: /model <name> to switch models")

		return func() tea.Msg {
			return SystemMessageMsg{Content: sb.String()}
		}
	}

	// Switch to specified model
	modelName := args[0]

	// Look up model info if available
	var switchMessage string
	if info, ok := model.GetModelInfo(modelName); ok {
		// Use the actual model ID for Ollama
		if ctx != nil && ctx.Ollama != nil {
			ctx.Ollama.SetModel(info.ID)
			if ctx.Config != nil {
				ctx.Config.Local.OllamaModel = info.ID
				ctx.Config.DefaultModel = info.ID
			}
		}
		switchMessage = fmt.Sprintf("Switched to %s (%s)\n%s\nCapabilities: %s",
			info.Name, info.Tier, info.Description, info.CapabilitiesString())
		modelName = info.ID
	} else {
		// Unknown model - use as-is (may be a local Ollama model not in registry)
		if ctx != nil && ctx.Ollama != nil {
			ctx.Ollama.SetModel(modelName)
			if ctx.Config != nil {
				ctx.Config.Local.OllamaModel = modelName
				ctx.Config.DefaultModel = modelName
			}
		}
		switchMessage = fmt.Sprintf("Switched to %s", modelName)
	}

	return func() tea.Msg {
		return ModelSwitchMsg{Model: modelName, Message: switchMessage}
	}
}

// HandleModels lists available models.
// Shows both cloud/known models and locally installed Ollama models.
func HandleModels(ctx *Context, args []string) tea.Cmd {
	ollamaClient := ctx.Ollama
	return func() tea.Msg {
		var sb strings.Builder
		sb.WriteString("Available Models\n")
		sb.WriteString("================\n\n")

		// Show known models by tier
		sb.WriteString("Cloud Models (API):\n")
		for _, tier := range []string{"Fast", "Balanced", "Powerful"} {
			tierModels := model.GetModelsByTier(tier)
			if len(tierModels) == 0 {
				continue
			}

			for _, info := range tierModels {
				if info.Provider == "Local" {
					continue // Skip local models in cloud section
				}
				sb.WriteString(fmt.Sprintf("  %s %-18s %s - %s\n",
					info.TierIcon(),
					info.Name,
					info.CostString(),
					info.Description,
				))
			}
		}
		sb.WriteString("\n")

		// Show local Ollama models if available
		if ollamaClient != nil {
			reqCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			models, err := ollamaClient.ListModels(reqCtx)
			if err == nil && len(models) > 0 {
				sb.WriteString("Local Models (Ollama):\n")
				for _, m := range models {
					// Check if we have info about this model
					if info, ok := model.GetModelInfo(m.Name); ok {
						sb.WriteString(fmt.Sprintf("  @ %-18s Free - %s\n",
							m.Name, info.Description))
					} else {
						sb.WriteString(fmt.Sprintf("  @ %-18s Free - Installed locally\n", m.Name))
					}
				}
				sb.WriteString("\n")
			} else if err != nil {
				sb.WriteString("Local Models: (Ollama not available)\n\n")
			}
		}

		sb.WriteString("Use /model <name> to switch models\n")
		sb.WriteString("Tier Icons: z=Fast ~=Balanced &=Powerful @=Local")

		return SystemMessageMsg{Content: sb.String()}
	}
}

// HandleMode switches the routing mode.
func HandleMode(ctx *Context, args []string) tea.Cmd {
	if len(args) == 0 {
		// Show current mode if no args
		if ctx != nil && ctx.Config != nil {
			currentMode := ctx.Config.Routing.DefaultMode
			return func() tea.Msg {
				return SystemMessageMsg{Content: fmt.Sprintf("Current mode: %s\nAvailable: local, cloud, hybrid", currentMode)}
			}
		}
		return func() tea.Msg {
			return ErrorMsg{
				Title:   "Missing argument",
				Message: "/mode requires a mode argument",
				Tip:     "Usage: /mode <local|cloud|hybrid>",
			}
		}
	}

	mode := strings.ToLower(args[0])
	switch mode {
	case "local", "cloud", "hybrid":
		// Update config if available
		if ctx != nil && ctx.Config != nil {
			ctx.Config.Routing.DefaultMode = mode
		}
		return func() tea.Msg {
			return ModeSwitchMsg{Mode: mode}
		}
	default:
		return func() tea.Msg {
			return ErrorMsg{
				Title:   "Invalid mode",
				Message: fmt.Sprintf("Unknown mode: %s", mode),
				Tip:     "Valid modes: local, cloud, hybrid",
			}
		}
	}
}

// HandleTools lists available tools.
func HandleTools(ctx *Context, args []string) tea.Cmd {
	return func() tea.Msg {
		return ShowToolsMsg{}
	}
}

// HandleTool toggles a tool on/off.
func HandleTool(ctx *Context, args []string) tea.Cmd {
	if len(args) == 0 {
		return func() tea.Msg {
			return ErrorMsg{
				Title:   "Missing argument",
				Message: "/tool requires a tool name",
				Tip:     "Usage: /tool <name> [on|off]",
			}
		}
	}

	tool := args[0]
	state := true // Default to on
	if len(args) > 1 {
		switch strings.ToLower(args[1]) {
		case "off", "disable", "false", "0":
			state = false
		}
	}

	return func() tea.Msg {
		return ToggleToolMsg{Tool: tool, State: state}
	}
}

// HandleConfig shows or sets configuration.
func HandleConfig(ctx *Context, args []string) tea.Cmd {
	// No args - show all config
	if len(args) == 0 {
		return func() tea.Msg {
			return ShowConfigMsg{}
		}
	}

	key := args[0]

	// Special cache commands
	if key == "cache" {
		if len(args) > 1 {
			switch strings.ToLower(args[1]) {
			case "clear":
				if ctx != nil && ctx.Cache != nil {
					ctx.Cache.Clear()
				}
				return func() tea.Msg {
					return CacheClearCompleteMsg{}
				}
			case "on", "enable", "true", "1":
				if ctx != nil && ctx.Cache != nil {
					ctx.Cache.Enable()
				}
				return func() tea.Msg {
					return CacheToggleMsg{Enabled: true}
				}
			case "off", "disable", "false", "0":
				if ctx != nil && ctx.Cache != nil {
					ctx.Cache.Disable()
				}
				return func() tea.Msg {
					return CacheToggleMsg{Enabled: false}
				}
			case "stats", "status":
				if ctx != nil && ctx.Cache != nil {
					stats := ctx.Cache.Stats()
					return func() tea.Msg {
						return CacheStatsMsg{
							Enabled:      ctx.Cache.IsEnabled(),
							ExactHits:    stats.ExactHits,
							SemanticHits: stats.SemanticHits,
							Misses:       stats.Misses,
							TotalLookups: stats.TotalLookups,
							HitRate:      ctx.Cache.HitRate(),
							ExactSize:    ctx.Cache.ExactCacheSize(),
							SemanticSize: ctx.Cache.SemanticCacheSize(),
						}
					}
				}
				return func() tea.Msg {
					return CacheStatsMsg{Enabled: false}
				}
			}
		}
		// Show cache status by default
		if ctx != nil && ctx.Cache != nil {
			stats := ctx.Cache.Stats()
			return func() tea.Msg {
				return CacheStatsMsg{
					Enabled:      ctx.Cache.IsEnabled(),
					ExactHits:    stats.ExactHits,
					SemanticHits: stats.SemanticHits,
					Misses:       stats.Misses,
					TotalLookups: stats.TotalLookups,
					HitRate:      ctx.Cache.HitRate(),
					ExactSize:    ctx.Cache.ExactCacheSize(),
					SemanticSize: ctx.Cache.SemanticCacheSize(),
				}
			}
		}
		return func() tea.Msg {
			return CacheStatsMsg{Enabled: false}
		}
	}

	// Single arg - get config value
	if len(args) == 1 {
		if ctx != nil && ctx.Config != nil {
			val, err := ctx.Config.Get(key)
			if err != nil {
				return func() tea.Msg {
					return ErrorMsg{
						Title:   "Config error",
						Message: err.Error(),
						Tip:     "Use /config to see all available keys",
					}
				}
			}
			return func() tea.Msg {
				return ShowConfigMsg{Key: key, Value: fmt.Sprintf("%v", val)}
			}
		}
		return func() tea.Msg {
			return ShowConfigMsg{Key: key}
		}
	}

	// Two or more args - set config value
	value := strings.Join(args[1:], " ")
	if ctx != nil && ctx.Config != nil {
		oldVal, _ := ctx.Config.Get(key)
		if err := ctx.Config.Set(key, value); err != nil {
			return func() tea.Msg {
				return ConfigUpdateMsg{Key: key, Error: err}
			}
		}
		newVal, _ := ctx.Config.Get(key)
		return func() tea.Msg {
			return ConfigUpdateMsg{Key: key, Value: newVal, OldValue: oldVal}
		}
	}
	return func() tea.Msg {
		return ShowConfigMsg{Key: key, Value: value}
	}
}

// StatusInfoMsg contains detailed status information.
type StatusInfoMsg struct {
	Model         string
	Mode          string
	SessionID     string
	SessionStart  string
	IdleTime      string
	CacheEnabled  bool
	CacheHitRate  float64
	OllamaStatus  string
}

// HandleStatus shows detailed status information.
func HandleStatus(ctx *Context, args []string) tea.Cmd {
	if ctx == nil {
		return func() tea.Msg {
			return ShowStatusMsg{}
		}
	}

	// Gather status info
	return func() tea.Msg {
		info := StatusInfoMsg{}

		// Model info
		if ctx.Ollama != nil {
			info.Model = ctx.Ollama.GetDefaultModel()

			// Check Ollama connectivity
			reqCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := ctx.Ollama.CheckRunning(reqCtx); err != nil {
				info.OllamaStatus = "disconnected"
			} else {
				info.OllamaStatus = "connected"
			}
		}

		// Config info
		if ctx.Config != nil {
			info.Mode = ctx.Config.Routing.DefaultMode
		}

		// Session info
		if ctx.Session != nil {
			status := ctx.Session.GetStatus()
			info.SessionID = status.SessionID
			info.SessionStart = status.StartTime.Format("15:04:05")
			info.IdleTime = formatDuration(status.IdleTime)
		}

		// Cache info
		if ctx.Cache != nil {
			info.CacheEnabled = ctx.Cache.IsEnabled()
			info.CacheHitRate = ctx.Cache.HitRate()
		}

		return info
	}
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// HandleTheme changes the color theme.
func HandleTheme(ctx *Context, args []string) tea.Cmd {
	if len(args) == 0 {
		// Show current theme
		if ctx != nil && ctx.Config != nil {
			return func() tea.Msg {
				return SystemMessageMsg{Content: "Current theme: " + ctx.Config.UI.Theme}
			}
		}
		return func() tea.Msg {
			return SystemMessageMsg{Content: "Theme: dark (default)"}
		}
	}

	theme := strings.ToLower(args[0])
	switch theme {
	case "dark", "light", "auto":
		if ctx != nil && ctx.Config != nil {
			ctx.Config.UI.Theme = theme
		}
		return func() tea.Msg {
			return SystemMessageMsg{Content: "Theme changed to: " + theme}
		}
	default:
		return func() tea.Msg {
			return ErrorMsg{
				Title:   "Invalid theme",
				Message: fmt.Sprintf("Unknown theme: %s", theme),
				Tip:     "Valid themes: dark, light, auto",
			}
		}
	}
}

// =============================================================================
// HELP TEXT GENERATION
// =============================================================================

// GenerateHelpText generates the help text for all commands.
// mode can be "quick", "all", or a category name (Navigation, Conversation, Model, Tools, Settings)
func GenerateHelpText(r *Registry, mode string) string {
	mode = strings.ToLower(mode)

	// Default to quick mode
	if mode == "" {
		mode = "quick"
	}

	// Quick help - show only essential commands
	if mode == "quick" {
		return generateQuickHelp()
	}

	// Category-specific help
	categoryMap := map[string]string{
		"navigation":   "Navigation",
		"conversation": "Conversation",
		"model":        "Model",
		"tools":        "Tools",
		"settings":     "Settings",
	}
	if canonical, ok := categoryMap[mode]; ok {
		return generateCategoryHelp(r, canonical)
	}

	// Full help (default for "all" or unknown modes)
	return generateFullHelp(r)
}

// generateQuickHelp shows only the 5 most essential commands
func generateQuickHelp() string {
	var sb strings.Builder

	sb.WriteString("Quick Help - Essential Commands\n")
	sb.WriteString("================================\n\n")

	// Essential commands with keyboard shortcuts
	sb.WriteString("  /help             Show this help (or try /help all)\n")
	sb.WriteString("  /new              Start new conversation\n")
	sb.WriteString("  /save             Save conversation\n")
	sb.WriteString("  /model            Switch model\n")
	sb.WriteString("  /quit             Exit rigrun\n\n")

	sb.WriteString("Keyboard Shortcuts\n")
	sb.WriteString("------------------\n")
	sb.WriteString("  Ctrl+C            Stop generation / Cancel\n")
	sb.WriteString("  Ctrl+P            Open command palette\n")
	sb.WriteString("  Tab               Auto-complete\n")
	sb.WriteString("  Up/Down           Navigate history\n\n")

	sb.WriteString("Want more? Try:\n")
	sb.WriteString("  /help all         - Show all available commands\n")
	sb.WriteString("  /help navigation  - Navigation commands\n")
	sb.WriteString("  /help conversation - Conversation management\n")
	sb.WriteString("  /help model       - Model and routing commands\n")
	sb.WriteString("  /help tools       - Tool management\n")
	sb.WriteString("  /help settings    - Settings and configuration\n")

	return sb.String()
}

// generateCategoryHelp generates help for a specific category
func generateCategoryHelp(r *Registry, category string) string {
	var sb strings.Builder

	categories := r.ByCategory()
	cmds, ok := categories[category]
	if !ok || len(cmds) == 0 {
		return fmt.Sprintf("No commands found in category: %s\n\nTry /help all to see all categories.", category)
	}

	sb.WriteString(fmt.Sprintf("%s Commands\n", category))
	sb.WriteString(strings.Repeat("=", len(category)+9) + "\n\n")

	for _, cmd := range cmds {
		if cmd.Hidden {
			continue
		}

		// Command name and aliases
		line := "  " + cmd.Name
		if len(cmd.Aliases) > 0 {
			line += " (" + strings.Join(cmd.Aliases, ", ") + ")"
		}

		// Pad to align descriptions
		for len(line) < 30 {
			line += " "
		}

		line += cmd.Description
		sb.WriteString(line + "\n")

		// Usage if specified
		if cmd.Usage != "" {
			sb.WriteString("      Usage: " + cmd.Usage + "\n")
		}
	}

	sb.WriteString("\n")

	// Add relevant tips based on category
	switch category {
	case "Navigation":
		sb.WriteString("Tips:\n")
		sb.WriteString("  - Press Esc to close any overlay\n")
		sb.WriteString("  - Use Tab for command auto-completion\n")
	case "Conversation":
		sb.WriteString("Tips:\n")
		sb.WriteString("  - Conversations auto-save on changes\n")
		sb.WriteString("  - Use @file:<path> to include files in your prompt\n")
		sb.WriteString("  - Try @clipboard to paste clipboard content\n")
	case "Model":
		sb.WriteString("Tips:\n")
		sb.WriteString("  - Local models are free but require Ollama\n")
		sb.WriteString("  - Cloud models require OpenRouter API key\n")
		sb.WriteString("  - Use /mode to switch between local/cloud/auto\n")
	case "Tools":
		sb.WriteString("Tips:\n")
		sb.WriteString("  - Tools extend rigrun's capabilities\n")
		sb.WriteString("  - Toggle tools on/off as needed\n")
	case "Settings":
		sb.WriteString("Tips:\n")
		sb.WriteString("  - Config changes persist automatically\n")
		sb.WriteString("  - Use /status to see current settings\n")
		sb.WriteString("  - Cache improves response time and reduces costs\n")
	}

	sb.WriteString("\nUse /help all to see all commands, or /help quick for essentials.\n")

	return sb.String()
}

// generateFullHelp generates the complete help text with all commands
func generateFullHelp(r *Registry) string {
	var sb strings.Builder

	sb.WriteString("Available Commands\n")
	sb.WriteString("==================\n\n")

	categories := r.ByCategory()
	categoryOrder := []string{"Navigation", "Conversation", "Model", "Tools", "Settings"}

	for _, category := range categoryOrder {
		cmds, ok := categories[category]
		if !ok || len(cmds) == 0 {
			continue
		}

		sb.WriteString(category + "\n")
		sb.WriteString(strings.Repeat("-", len(category)) + "\n")

		for _, cmd := range cmds {
			if cmd.Hidden {
				continue
			}

			// Command name and aliases
			line := "  " + cmd.Name
			if len(cmd.Aliases) > 0 {
				line += " (" + strings.Join(cmd.Aliases, ", ") + ")"
			}

			// Pad to align descriptions
			for len(line) < 30 {
				line += " "
			}

			line += cmd.Description
			sb.WriteString(line + "\n")

			// Usage if specified
			if cmd.Usage != "" {
				sb.WriteString("      Usage: " + cmd.Usage + "\n")
			}
		}

		sb.WriteString("\n")
	}

	sb.WriteString("Context Mentions\n")
	sb.WriteString("----------------\n")
	sb.WriteString("  @file:<path>    Include file content\n")
	sb.WriteString("  @clipboard      Include clipboard content\n")
	sb.WriteString("  @git            Include recent git info\n")
	sb.WriteString("  @codebase       Include directory structure\n")
	sb.WriteString("  @error          Include last error message\n")
	sb.WriteString("\n")

	sb.WriteString("Keyboard Shortcuts\n")
	sb.WriteString("------------------\n")
	sb.WriteString("  Ctrl+C          Stop generation / Cancel\n")
	sb.WriteString("  Ctrl+P          Open command palette\n")
	sb.WriteString("  Tab             Auto-complete\n")
	sb.WriteString("  Up/Down         Navigate history\n")
	sb.WriteString("  Esc             Close overlay\n\n")

	sb.WriteString("Tip: Use /help <category> to see commands by category\n")
	sb.WriteString("Categories: navigation, conversation, model, tools, settings\n")

	return sb.String()
}

// HandleTutorial shows the interactive tutorial.
func HandleTutorial(ctx *Context, args []string) tea.Cmd {
	return func() tea.Msg {
		return ShowTutorialMsg{}
	}
}

// HandlePlan creates and shows an execution plan for a task.
func HandlePlan(ctx *Context, args []string) tea.Cmd {
	if len(args) == 0 {
		return func() tea.Msg {
			return ErrorMsg{
				Title:   "Missing argument",
				Message: "/plan requires a task description",
				Tip:     "Usage: /plan <task description>",
			}
		}
	}

	// Join all args to get the full task description
	task := strings.Join(args, " ")

	// Validate task length
	const maxTaskLength = 10000
	if len(task) > maxTaskLength {
		return func() tea.Msg {
			return ErrorMsg{
				Title:   "Task too long",
				Message: fmt.Sprintf("Task description is too long: %d characters (max: %d)", len(task), maxTaskLength),
				Tip:     "Please provide a shorter task description",
			}
		}
	}

	// Validate task is not empty after trimming
	if strings.TrimSpace(task) == "" {
		return func() tea.Msg {
			return ErrorMsg{
				Title:   "Empty task",
				Message: "Task description cannot be empty",
				Tip:     "Usage: /plan <task description>",
			}
		}
	}

	return func() tea.Msg {
		return CreatePlanMsg{Task: task}
	}
}

// GenerateStatusText generates the status text.
func GenerateStatusText(hctx *HandlerContext) string {
	var sb strings.Builder

	sb.WriteString("Status Information\n")
	sb.WriteString("==================\n\n")

	sb.WriteString("Model: " + hctx.CurrentModel + "\n")
	sb.WriteString("Mode:  " + hctx.CurrentMode + "\n")

	if hctx.ConversationID != "" {
		sb.WriteString("Session: " + hctx.ConversationID + "\n")
	}

	sb.WriteString(fmt.Sprintf("Messages: %d\n", len(hctx.Messages)))
	sb.WriteString("Directory: " + hctx.WorkingDirectory + "\n")

	return sb.String()
}

// =============================================================================
// COST TRACKING
// =============================================================================

// ShowCostDashboardMsg triggers showing the cost dashboard.
type ShowCostDashboardMsg struct {
	View string // "summary", "history", "breakdown"
}

// HandleCost shows cost information and analytics.
func HandleCost(ctx *Context, args []string) tea.Cmd {
	view := "summary" // Default view

	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "history":
			view = "history"
		case "breakdown":
			view = "breakdown"
		case "summary":
			view = "summary"
		default:
			return func() tea.Msg {
				return ErrorMsg{
					Title:   "Invalid view",
					Message: fmt.Sprintf("Unknown cost view: %s", args[0]),
					Tip:     "Valid views: summary, history, breakdown",
				}
			}
		}
	}

	return func() tea.Msg {
		return ShowCostDashboardMsg{View: view}
	}
}
