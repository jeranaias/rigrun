// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package commands provides the slash command system for the TUI.
package commands

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeranaias/rigrun-tui/internal/cache"
	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/index"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
	"github.com/jeranaias/rigrun-tui/internal/session"
	"github.com/jeranaias/rigrun-tui/internal/storage"
	"github.com/jeranaias/rigrun-tui/internal/telemetry"
)

// =============================================================================
// COMMAND DEFINITION
// =============================================================================

// Command represents a slash command that can be executed.
type Command struct {
	// Name is the primary command name (e.g., "/help")
	Name string

	// Aliases are alternative names (e.g., "/h", "/?")
	Aliases []string

	// Description is shown in help and completion
	Description string

	// Usage shows argument syntax (e.g., "/model <name>")
	Usage string

	// Args defines the expected arguments
	Args []ArgDef

	// Handler is the function that executes the command
	Handler func(ctx *Context, args []string) tea.Cmd

	// Hidden commands don't appear in help
	Hidden bool

	// Category for grouping in help display
	Category string
}

// ArgDef defines an argument for a command.
type ArgDef struct {
	// Name of the argument
	Name string

	// Required indicates if the argument must be provided
	Required bool

	// Type determines completion behavior
	Type ArgType

	// Description explains the argument
	Description string

	// Values for enum types
	Values []string

	// Completer for custom completion
	Completer func() []string
}

// ArgType indicates what kind of completion to provide.
type ArgType int

const (
	ArgTypeString ArgType = iota // Free-form string
	ArgTypeModel                 // Model name from Ollama
	ArgTypeSession               // Session ID from saved sessions
	ArgTypeFile                  // File path
	ArgTypeEnum                  // One of predefined values
	ArgTypeTool                  // Tool name
	ArgTypeConfig                // Config key
)

// =============================================================================
// COMMAND REGISTRY
// =============================================================================

// Registry holds all registered commands.
type Registry struct {
	commands map[string]*Command
	aliases  map[string]*Command
}

// NewRegistry creates a new command registry with all built-in commands.
func NewRegistry() *Registry {
	r := &Registry{
		commands: make(map[string]*Command),
		aliases:  make(map[string]*Command),
	}
	r.registerBuiltins()
	return r
}

// Register adds a command to the registry.
func (r *Registry) Register(cmd *Command) {
	r.commands[cmd.Name] = cmd
	for _, alias := range cmd.Aliases {
		r.aliases[alias] = cmd
	}
}

// Get retrieves a command by name or alias.
func (r *Registry) Get(name string) *Command {
	if cmd, ok := r.commands[name]; ok {
		return cmd
	}
	if cmd, ok := r.aliases[name]; ok {
		return cmd
	}
	return nil
}

// All returns all registered commands.
func (r *Registry) All() []*Command {
	cmds := make([]*Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// ByCategory returns commands grouped by category.
func (r *Registry) ByCategory() map[string][]*Command {
	result := make(map[string][]*Command)
	for _, cmd := range r.commands {
		if cmd.Hidden {
			continue
		}
		category := cmd.Category
		if category == "" {
			category = "General"
		}
		result[category] = append(result[category], cmd)
	}
	return result
}

// =============================================================================
// BUILT-IN COMMANDS
// =============================================================================

func (r *Registry) registerBuiltins() {
	// Navigation commands
	r.Register(&Command{
		Name:        "/help",
		Aliases:     []string{"/h", "/?"},
		Description: "Show help and available commands",
		Usage:       "/help [quick|all|<category>]",
		Args: []ArgDef{
			{
				Name:        "mode",
				Required:    false,
				Type:        ArgTypeEnum,
				Values:      []string{"quick", "all", "navigation", "conversation", "model", "tools", "settings"},
				Description: "Help mode or category",
			},
		},
		Category: "Navigation",
		Handler:  handleHelp,
	})

	r.Register(&Command{
		Name:        "/quit",
		Aliases:     []string{"/q", "/exit"},
		Description: "Exit rigrun",
		Category:    "Navigation",
		Handler:     handleQuit,
	})

	// Conversation commands
	r.Register(&Command{
		Name:        "/new",
		Aliases:     []string{"/n"},
		Description: "Start a new conversation",
		Category:    "Conversation",
		Handler:     handleNew,
	})

	r.Register(&Command{
		Name:        "/save",
		Aliases:     []string{"/s"},
		Description: "Save current conversation",
		Usage:       "/save [name]",
		Args: []ArgDef{
			{Name: "name", Required: false, Type: ArgTypeString, Description: "Optional name for the session"},
		},
		Category: "Conversation",
		Handler:  handleSave,
	})

	r.Register(&Command{
		Name:        "/load",
		Aliases:     []string{"/l", "/resume"},
		Description: "Load a saved conversation",
		Usage:       "/load <session_id>",
		Args: []ArgDef{
			{Name: "session_id", Required: true, Type: ArgTypeSession, Description: "ID of the session to load"},
		},
		Category: "Conversation",
		Handler:  handleLoad,
	})

	r.Register(&Command{
		Name:        "/clear",
		Aliases:     []string{"/c"},
		Description: "Clear conversation history",
		Category:    "Conversation",
		Handler:     handleClear,
	})

	r.Register(&Command{
		Name:        "/copy",
		Description: "Copy last response to clipboard",
		Category:    "Conversation",
		Handler:     handleCopy,
	})

	r.Register(&Command{
		Name:        "/export",
		Description: "Export conversation to file",
		Usage:       "/export [format]",
		Args: []ArgDef{
			{Name: "format", Required: false, Type: ArgTypeEnum, Values: []string{"json", "md", "txt"}, Description: "Export format"},
		},
		Category: "Conversation",
		Handler:  handleExport,
	})

	r.Register(&Command{
		Name:        "/sessions",
		Aliases:     []string{"/list"},
		Description: "List saved sessions",
		Category:    "Conversation",
		Handler:     handleSessions,
	})

	// Model commands
	r.Register(&Command{
		Name:        "/model",
		Aliases:     []string{"/m"},
		Description: "Switch or show current model",
		Usage:       "/model [name]",
		Args: []ArgDef{
			{Name: "name", Required: false, Type: ArgTypeModel, Description: "Model to switch to"},
		},
		Category: "Model",
		Handler:  handleModel,
	})

	r.Register(&Command{
		Name:        "/models",
		Description: "List available models",
		Category:    "Model",
		Handler:     handleModels,
	})

	r.Register(&Command{
		Name:        "/mode",
		Description: "Switch routing mode",
		Usage:       "/mode <local|cloud|hybrid>",
		Args: []ArgDef{
			{Name: "mode", Required: true, Type: ArgTypeEnum, Values: []string{"local", "cloud", "hybrid"}, Description: "Routing mode"},
		},
		Category: "Model",
		Handler:  handleMode,
	})

	// Tool commands
	r.Register(&Command{
		Name:        "/tools",
		Description: "List available tools",
		Category:    "Tools",
		Handler:     handleTools,
	})

	r.Register(&Command{
		Name:        "/tool",
		Description: "Enable or disable a tool",
		Usage:       "/tool <name> [on|off]",
		Args: []ArgDef{
			{Name: "name", Required: true, Type: ArgTypeTool, Description: "Tool name"},
			{Name: "state", Required: false, Type: ArgTypeEnum, Values: []string{"on", "off"}, Description: "Enable or disable"},
		},
		Category: "Tools",
		Handler:  handleTool,
	})

	// Settings commands
	r.Register(&Command{
		Name:        "/config",
		Description: "Show or edit configuration",
		Usage:       "/config [key] [value]",
		Args: []ArgDef{
			{Name: "key", Required: false, Type: ArgTypeConfig, Description: "Config key to show/set"},
			{Name: "value", Required: false, Type: ArgTypeString, Description: "New value"},
		},
		Category: "Settings",
		Handler:  handleConfig,
	})

	r.Register(&Command{
		Name:        "/status",
		Description: "Show detailed status information",
		Category:    "Settings",
		Handler:     handleStatus,
	})

	r.Register(&Command{
		Name:        "/theme",
		Description: "Change color theme",
		Usage:       "/theme [name]",
		Args: []ArgDef{
			{Name: "name", Required: false, Type: ArgTypeEnum, Values: []string{"dark", "light", "auto"}, Description: "Theme name"},
		},
		Category: "Settings",
		Hidden:   true, // Not yet implemented
		Handler:  handleTheme,
	})

	r.Register(&Command{
		Name:        "/tutorial",
		Aliases:     []string{"/tut"},
		Description: "Restart the interactive tutorial",
		Category:    "Navigation",
		Handler:     handleTutorial,
	})

	// Plan mode
	r.Register(&Command{
		Name:        "/plan",
		Description: "Create and execute a multi-step plan",
		Usage:       "/plan <task>",
		Args: []ArgDef{
			{Name: "task", Required: true, Type: ArgTypeString, Description: "Task description to plan"},
		},
		Category: "Tools",
		Handler:  handlePlan,
	})

	// Benchmark command
	r.Register(&Command{
		Name:        "/benchmark",
		Aliases:     []string{"/bench"},
		Description: "Run benchmark tests on a model",
		Usage:       "/benchmark <model> [model2 model3...]",
		Args: []ArgDef{
			{Name: "model", Required: true, Type: ArgTypeModel, Description: "Model to benchmark"},
			{Name: "models", Required: false, Type: ArgTypeModel, Description: "Additional models for comparison"},
		},
		Category: "Model",
		Handler:  handleBenchmark,
	})

	// Cost tracking command
	r.Register(&Command{
		Name:        "/cost",
		Description: "Show cost dashboard and analytics",
		Usage:       "/cost [summary|history|breakdown]",
		Args: []ArgDef{
			{Name: "view", Required: false, Type: ArgTypeEnum, Values: []string{"summary", "history", "breakdown"}, Description: "Dashboard view"},
		},
		Category: "Settings",
		Handler:  handleCost,
	})
}

// =============================================================================
// HANDLER IMPLEMENTATIONS
// =============================================================================

func handleHelp(ctx *Context, args []string) tea.Cmd {
	return HandleHelp(ctx, args)
}

func handleQuit(ctx *Context, args []string) tea.Cmd {
	return HandleQuit(ctx, args)
}

func handleNew(ctx *Context, args []string) tea.Cmd {
	return HandleNew(ctx, args)
}

func handleSave(ctx *Context, args []string) tea.Cmd {
	return HandleSave(ctx, args)
}

func handleLoad(ctx *Context, args []string) tea.Cmd {
	return HandleLoad(ctx, args)
}

func handleClear(ctx *Context, args []string) tea.Cmd {
	return HandleClear(ctx, args)
}

func handleCopy(ctx *Context, args []string) tea.Cmd {
	return HandleCopy(ctx, args)
}

func handleExport(ctx *Context, args []string) tea.Cmd {
	return HandleExport(ctx, args)
}

func handleSessions(ctx *Context, args []string) tea.Cmd {
	return HandleSessions(ctx, args)
}

func handleModel(ctx *Context, args []string) tea.Cmd {
	return HandleModel(ctx, args)
}

func handleModels(ctx *Context, args []string) tea.Cmd {
	return HandleModels(ctx, args)
}

func handleMode(ctx *Context, args []string) tea.Cmd {
	return HandleMode(ctx, args)
}

func handleTools(ctx *Context, args []string) tea.Cmd {
	return HandleTools(ctx, args)
}

func handleTool(ctx *Context, args []string) tea.Cmd {
	return HandleTool(ctx, args)
}

func handleConfig(ctx *Context, args []string) tea.Cmd {
	return HandleConfig(ctx, args)
}

func handleStatus(ctx *Context, args []string) tea.Cmd {
	return HandleStatus(ctx, args)
}

func handleTheme(ctx *Context, args []string) tea.Cmd {
	return HandleTheme(ctx, args)
}

func handleTutorial(ctx *Context, args []string) tea.Cmd {
	return HandleTutorial(ctx, args)
}

func handlePlan(ctx *Context, args []string) tea.Cmd {
	return HandlePlan(ctx, args)
}

func handleBenchmark(ctx *Context, args []string) tea.Cmd {
	return HandleBenchmark(ctx, args)
}

func handleCost(ctx *Context, args []string) tea.Cmd {
	return HandleCost(ctx, args)
}

// =============================================================================
// CONTEXT TYPE
// =============================================================================

// Context provides access to application state for command handlers.
// It follows the dependency injection pattern, allowing handlers to access
// services without direct coupling to the application structure.
//
// All fields are optional and may be nil - handlers should check before use.
//
// Example usage in a handler:
//
//	func HandleStatus(ctx *Context, args []string) tea.Cmd {
//	    if ctx.Ollama != nil {
//	        model := ctx.Ollama.GetDefaultModel()
//	        // ...
//	    }
//	}
type Context struct {
	// Config provides access to application configuration
	Config *config.Config

	// Ollama is the client for local model operations
	Ollama *ollama.Client

	// Storage handles conversation persistence
	Storage *storage.ConversationStore

	// Session manages the current session state
	Session *session.Manager

	// Cache provides query caching functionality
	Cache *cache.CacheManager

	// CodebaseIndex provides intelligent code search (optional)
	CodebaseIndex *index.CodebaseIndex

	// CostTracker tracks token usage and costs
	CostTracker *telemetry.CostTracker

	// HandlerCtx provides additional runtime context
	HandlerCtx *HandlerContext
}

// NewContext creates a new command context with the given dependencies.
// All parameters are optional and can be nil.
func NewContext(cfg *config.Config, ollamaClient *ollama.Client, store *storage.ConversationStore, sess *session.Manager, cacheManager *cache.CacheManager) *Context {
	return &Context{
		Config:  cfg,
		Ollama:  ollamaClient,
		Storage: store,
		Session: sess,
		Cache:   cacheManager,
	}
}

// WithHandlerContext attaches a HandlerContext to the Context.
func (c *Context) WithHandlerContext(hctx *HandlerContext) *Context {
	c.HandlerCtx = hctx
	return c
}

// RecordActivity records user activity in the session manager if available.
func (c *Context) RecordActivity() {
	if c.Session != nil {
		c.Session.RecordActivity()
	}
}

// MarkDirty marks the session as having unsaved changes.
func (c *Context) MarkDirty() {
	if c.Session != nil {
		c.Session.MarkDirty()
	}
}

// =============================================================================
// COMPLETION TYPE
// =============================================================================

// Completion represents a completion suggestion.
type Completion struct {
	// Value to insert
	Value string

	// Display text (may include formatting)
	Display string

	// Description shown alongside
	Description string

	// Score for ranking (higher = better match)
	Score int

	// IsCurrent indicates this is the current selection
	IsCurrent bool
}
