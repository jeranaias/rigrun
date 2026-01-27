// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeranaias/rigrun-tui/internal/cache"
	"github.com/jeranaias/rigrun-tui/internal/cloud"
	"github.com/jeranaias/rigrun-tui/internal/commands"
	"github.com/jeranaias/rigrun-tui/internal/config"
	ctxmention "github.com/jeranaias/rigrun-tui/internal/context"
	"github.com/jeranaias/rigrun-tui/internal/model"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/tasks"
	"github.com/jeranaias/rigrun-tui/internal/tools"
	"github.com/jeranaias/rigrun-tui/internal/ui/components"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// CHAT STATE
// =============================================================================

// State represents the current state of the chat view.
type State int

const (
	StateReady     State = iota // Ready for input
	StateStreaming              // Receiving streaming response
	StateError                  // Showing an error
)

// =============================================================================
// CHAT MODEL
// =============================================================================

// Model is the Bubble Tea model for the chat view.
type Model struct {
	// State
	state State

	// Styling
	theme *styles.Theme

	// Dimensions
	width  int
	height int

	// Conversation
	conversation *model.Conversation

	// Current streaming message
	streamingMsgID string
	streamingStats *model.Statistics

	// Streaming optimization (Feature 4.2)
	streamingBuffer   *StreamingBuffer   // Batches tokens for efficient rendering
	viewportOptimizer *ViewportOptimizer // Reduces redundant viewport updates
	lastStreamTick    time.Time          // Last time we processed streaming updates

	// Ollama client (local inference)
	ollama    *ollama.Client
	cancelMgr *cancelManager // Pointer to avoid copying mutex during Bubble Tea updates

	// Cloud client (OpenRouter)
	cloudClient *cloud.OpenRouterClient

	// UI Components
	viewport viewport.Model
	input    textinput.Model
	spinner  spinner.Model

	// Key bindings
	keyMap KeyMap // Comprehensive keyboard shortcuts

	// Error state
	lastError *ErrorMsg

	// Status
	modelName   string
	mode        string
	gpu         string
	offlineMode bool   // IL5 SC-7: True when --no-network flag is active
	statusMsg   string // Temporary status message for display

	// Thinking state
	isThinking     bool
	thinkingStart  time.Time
	thinkingDetail string

	// Router integration
	routingMode  string                  // "cloud", "local", "hybrid"
	lastRouting  *router.RoutingDecision // Last routing decision for display
	sessionStats *router.SessionStats    // Cumulative session statistics

	// Current query tracking (for session stats on completion)
	currentQueryTier  router.Tier // Actual tier used for current streaming query
	currentQueryStart time.Time   // Start time of current query for latency tracking

	// Cache integration
	cacheManager *cache.CacheManager // Query response cache manager
	lastCacheHit cache.CacheHitType  // Type of last cache hit (for display)
	pendingQuery string              // Query pending for caching on completion
	pendingMsgID string              // Message ID for pending cache storage

	// Tool system integration
	toolRegistry *tools.Registry    // Available tools
	toolExecutor *tools.Executor    // Tool execution engine
	toolsEnabled bool               // Whether tools are enabled for chat
	agenticLoop  *tools.AgenticLoop // Agentic loop for multi-turn tool use

	// Context mention system (@file, @git, @codebase, @error, @clipboard)
	contextExpander *ctxmention.Expander // Expands @ mentions into context
	lastContextInfo string               // Summary of last expanded context (for display)

	// Context cost display (real-time token estimation)
	contextTokenEstimate int     // Estimated tokens for current @mentions in input
	contextCostEstimate  float64 // Estimated cost in cents for current context

	// Active context tracking (for UI display)
	activeContext     *components.ActiveContext // Tracks active @mentions with token counts
	showContextBar    bool                      // Whether to show expanded context bar
	contextBarExpanded bool                     // Whether context bar is expanded

	// Search functionality (Ctrl+F)
	searchMode       bool            // True when in search mode
	searchQuery      string          // Current search query
	searchInput      textinput.Model // Search input field
	searchMatches    []SearchMatch   // All matches found
	searchMatchIndex int             // Current match index (for navigation)

	// Help overlay
	showHelp bool // True when help overlay is visible

	// Multi-line input mode
	multiLineMode bool // True when in multi-line input mode

	// Vim-like input mode
	inputMode bool // True when focused on text input (vim-like)

	// Vim mode handler
	vimHandler *VimHandler // Vim-style modal editing handler

	// Tab completion system
	completionState   *commands.CompletionState // Completion state tracker
	completer         *commands.Completer       // Completion engine
	showCompletions   bool                      // Whether to show completion popup
	completionCycleCount int                    // Number of times Tab pressed (for cycling)

	// Classification enforcement (NIST 800-53 AC-4)
	classificationLevel    security.ClassificationLevel     // Current session classification level
	classificationEnforcer *security.ClassificationEnforcer // AC-4 routing enforcer

	// Progress tracking (for agentic loops and multi-step operations)
	progressIndicator *components.ProgressIndicator // Current operation progress
	showProgress      bool                          // Whether to show progress indicator

	// Command palette (Ctrl+P)
	commandPalette     *components.CommandPalette // Command palette for fuzzy command search
	commandRegistry    *commands.Registry         // Registry for palette access

	// Tutorial overlay
	tutorial *components.TutorialOverlay // Interactive tutorial overlay

	// Background task system
	taskQueue  *tasks.Queue  // Background task queue
	taskRunner *tasks.Runner // Task runner for background execution

	// Non-blocking error toasts (lazygit-inspired)
	// Toasts appear in bottom-right corner and auto-dismiss without blocking UI
	toastManager *components.ToastManager // Manages error/warning/info toasts
}

// SearchMatch represents a search match location.
type SearchMatch struct {
	MessageIndex int // Index of the message containing the match
	StartPos     int // Start position within the message content
	EndPos       int // End position within the message content
}

// New creates a new chat model.
func New(theme *styles.Theme) Model {
	// Create text input with prompt
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 4096
	ti.Focus()

	// Create viewport
	vp := viewport.New(80, 20)
	vp.SetContent("")

	// Create spinner with ASCII-compatible animation
	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"|", "/", "-", "\\"},
		FPS:    time.Second / 30, // 30fps to match streaming
	}

	// Initialize tool system
	toolRegistry := tools.NewRegistry()
	toolExecutor := tools.NewExecutor(toolRegistry)
	// Auto-approve low-risk tools (Read, Glob, Grep)
	toolExecutor.SetAutoApproveLevel(tools.PermissionAuto)

	// Initialize context expander for @ mentions
	contextExpander := ctxmention.NewExpander(nil)

	// Create search input
	searchInput := textinput.New()
	searchInput.Prompt = "Search: "
	searchInput.Placeholder = "Type to search..."
	searchInput.CharLimit = 256

	// Create conversation with tool-aware system prompt
	conv := model.NewConversation()
	conv.SystemPrompt = tools.GenerateMinimalToolPrompt()

	// Initialize classification enforcer for AC-4 compliance
	// Uses global audit logger for audit trail of blocked requests
	classEnforcer := security.NewClassificationEnforcerGlobal(conv.ID)

	// Initialize completion system
	cmdRegistry := commands.NewRegistry()
	completer := commands.NewCompleter(cmdRegistry)
	completionState := commands.NewCompletionState()

	// Create command palette
	cmdPalette := components.NewCommandPalette(cmdRegistry, theme)

	// Initialize tutorial overlay
	tutorial := components.NewTutorialOverlay()

	// Initialize background task system
	taskQueue := tasks.NewQueue(100) // Keep last 100 completed tasks
	taskRunner := tasks.NewRunner(taskQueue)
	taskRunner.Start() // Start processing tasks immediately

	// Initialize vim handler (check config for vim_mode setting)
	vimEnabled := false
	if cfg := config.Global(); cfg != nil {
		vimEnabled = cfg.UI.VimMode
	}
	vimHandler := NewVimHandler(vimEnabled)

	return Model{
		state:                  StateReady,
		theme:                  theme,
		conversation:           conv,
		viewport:               vp,
		input:                  ti,
		spinner:                sp,
		keyMap:                 DefaultKeyMap(),    // Initialize comprehensive key bindings
		cancelMgr:              newCancelManager(), // Pointer to avoid mutex copy on Bubble Tea updates
		modelName:              "qwen2.5-coder:14b",
		mode:                   "hybrid",
		gpu:                    "", // Set externally via SetGPU()
		routingMode:            "hybrid",
		sessionStats:           router.NewSessionStats(),
		toolRegistry:           toolRegistry,
		toolExecutor:           toolExecutor,
		toolsEnabled:           true, // Enable tools by default
		contextExpander:        contextExpander,
		activeContext:          components.NewActiveContext(),       // Initialize empty active context
		showContextBar:         false,                               // Don't show context bar initially
		contextBarExpanded:     false,                               // Context bar starts collapsed
		searchInput:            searchInput,
		inputMode:              true,                                // Start in input mode (focused on text input)
		completionState:        completionState,                     // Tab completion state
		completer:              completer,                           // Tab completion engine
		showCompletions:        false,                               // Don't show completions initially
		completionCycleCount:   0,                                   // No cycles yet
		classificationLevel:    security.ClassificationUnclassified, // Default to UNCLASSIFIED
		classificationEnforcer: classEnforcer,
		commandPalette:         cmdPalette,
		commandRegistry:        cmdRegistry,
		tutorial:               &tutorial, // Tutorial overlay
		taskQueue:              taskQueue,
		taskRunner:             taskRunner,
		streamingBuffer:        NewStreamingBuffer(),   // Feature 4.2: Token batching for smooth streaming
		viewportOptimizer:     NewViewportOptimizer(),                // Feature 4.2: Reduce redundant viewport updates
		lastStreamTick:        time.Now(),                            // Feature 4.2: Last streaming tick timestamp
		vimHandler:             vimHandler, // Vim mode handler
		toastManager:           components.NewToastManager(), // Non-blocking error toasts
	}
}

// NewWithClient creates a new chat model with an Ollama client.
func NewWithClient(theme *styles.Theme, client *ollama.Client) Model {
	m := New(theme)
	m.ollama = client
	if client != nil {
		m.modelName = client.GetDefaultModel()
	}
	return m
}

// =============================================================================
// BUBBLE TEA INTERFACE
// =============================================================================

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	// Note: Ollama check is handled by the main model (main.go) to avoid
	// duplicate checks with different timeouts causing false errors.
	// The main model's checkOllama() uses a 30-second timeout which is more
	// reliable on Windows where network initialization can be slow.

	var cmds []tea.Cmd
	cmds = append(cmds, textinput.Blink)

	// Check if tutorial should be shown for first-time users
	cfg := config.Global()
	if cfg != nil && !cfg.UI.TutorialCompleted {
		// Show tutorial for first-time users
		cmds = append(cmds, func() tea.Msg {
			return ShowTutorialMsg{}
		})
	}

	return tea.Batch(cmds...)
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg)

	case tea.KeyMsg:
		return m.handleKey(msg)

	case StreamStartMsg:
		return m.handleStreamStart(msg)

	case StreamTokenMsg:
		return m.handleStreamToken(msg)

	case StreamTickMsg:
		return m.handleStreamTick(msg)

	case StreamCompleteMsg:
		return m.handleStreamComplete(msg)

	case StreamErrorMsg:
		return m.handleStreamError(msg)

	case RoutingFallbackMsg:
		return m.handleRoutingFallback(msg)

	case OllamaStatusMsg:
		return m.handleOllamaStatus(msg)

	case OllamaModelsMsg:
		return m.handleOllamaModels(msg)

	case OllamaModelSwitchedMsg:
		return m.handleModelSwitched(msg)

	case ErrorMsg:
		m.lastError = &msg
		m.state = StateError
		return m, nil

	case ErrorDismissMsg:
		m.lastError = nil
		m.state = StateReady
		m.input.Focus()
		return m, textinput.Blink

	case spinner.TickMsg:
		if m.isThinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case ClearConversationMsg:
		m.conversation.ClearHistory()
		m.updateViewport()
		return m, nil

	case NewConversationMsg:
		m.conversation = model.NewConversation()
		if m.toolsEnabled {
			m.conversation.SystemPrompt = tools.GenerateMinimalToolPrompt()
		}
		m.updateViewport()
		return m, nil

	case ToolCallRequestedMsg:
		return m.handleToolCallRequested(msg)

	case ToolResultMsg:
		return m.handleToolResult(msg)

	case ToolPermissionMsg:
		return m.handleToolPermission(msg)

	case ToolPermissionResponseMsg:
		return m.handleToolPermissionResponse(msg)

	case SessionResumeMsg:
		return m.handleSessionResume(msg)

	case SessionResumedMsg:
		return m.handleSessionResumed(msg)

	case SessionSearchMsg:
		return m.handleSessionSearch(msg)

	case SessionSearchResultMsg:
		return m.handleSessionSearchResult(msg)

	case commands.ExportConversationMsg:
		return m.handleExportConversation(msg)

	case commands.ExportCompleteMsg:
		return m.handleExportComplete(msg)

	case ShowTutorialMsg:
		return m.handleShowTutorial(msg)

	case TutorialCompleteMsg:
		return m.handleTutorialComplete(msg)

	case ProgressStartMsg:
		return m.handleProgressStart(msg)

	case ProgressStepMsg:
		return m.handleProgressStep(msg)

	case ProgressUpdateMsg:
		return m.handleProgressUpdate(msg)

	case ProgressCompleteMsg:
		return m.handleProgressComplete(msg)

	case ProgressCanceledMsg:
		return m.handleProgressCanceled(msg)

	case ProgressErrorMsg:
		return m.handleProgressError(msg)

	case components.ExecuteCommandMsg:
		// Execute command selected from palette
		return m.handleCommandExecution(msg)

	case components.TutorialAdvanceMsg:
		// Forward to tutorial overlay
		if m.tutorial != nil {
			var cmd tea.Cmd
			*m.tutorial, cmd = m.tutorial.Update(msg)
			return m, cmd
		}
		return m, nil

	case components.TutorialCompleteMsg:
		// Convert from components.TutorialCompleteMsg to chat.TutorialCompleteMsg
		return m.handleTutorialComplete(TutorialCompleteMsg{
			Completed:   msg.Completed,
			CurrentStep: msg.CurrentStep,
		})

	case TaskCreateMsg:
		return m.handleTaskCreate(msg)

	case TaskListMsg:
		return m.handleTaskList(msg)

	case TaskCancelMsg:
		return m.handleTaskCancel(msg)

	case TaskNotificationMsg:
		return m.handleTaskNotification(msg)

	case VimCommandMsg:
		return m.handleVimCommand(msg)

	case components.ToastTickMsg:
		return m.handleToastTick(msg)

	case components.ToastDismissMsg:
		return m.handleToastDismiss(msg)

	case components.ToastAddMsg:
		return m.handleToastAdd(msg)

	default:
		// For any unhandled messages, update the text input if ready
		// and always update the viewport for scroll events, etc.
		if m.state == StateReady {
			var inputCmd tea.Cmd
			m.input, inputCmd = m.input.Update(msg)
			cmds = append(cmds, inputCmd)
		}

		// Update viewport for scroll and other events
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, vpCmd)

		return m, tea.Batch(cmds...)
	}
}

// View renders the chat view.
func (m Model) View() string {
	return m.renderChat()
}

// =============================================================================
// MESSAGE HANDLERS
// =============================================================================

func (m Model) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	// Apply resize immediately - Bubble Tea handles resize events efficiently
	// and debouncing with time.Sleep causes UI freezes during window drag
	m.width = msg.Width
	m.height = msg.Height

	// Calculate viewport dimensions
	// Layout: header + viewport (dynamic) + input area + status bar
	//
	// IMPORTANT: These constants MUST stay in sync with the actual rendered heights
	// in view.go renderChat(). The renderChat() function measures actual heights using
	// lipgloss.Height() and has a fallback if there's a mismatch, but these values
	// should be conservative (larger) to ensure the viewport is never too tall.
	//
	// If you modify any of these components in view.go, update these constants:
	// - Header: renderHeader() - includes background styling with Padding(0, 1)
	// - Input area: renderInput() - separator + input line + char count
	// - Status bar: renderStatusBar() - includes background styling with Padding(0, 1)
	// - Search bar: renderSearchBar() - includes background styling with Padding(0, 1)
	//
	// Conservative estimates (slightly larger than actual) prevent overflow:
	const (
		headerHeight    = 2 // Was 1 - conservative to account for potential styling/padding
		inputAreaHeight = 4 // Was 3 - separator + input line + char count + buffer
		statusBarHeight = 2 // Was 1 - conservative to account for potential styling/padding
		searchBarHeight = 2 // Was 1 - conservative to account for potential styling/padding
	)

	reservedHeight := headerHeight + inputAreaHeight + statusBarHeight
	if m.searchMode {
		reservedHeight += searchBarHeight
	}

	viewportHeight := m.height - reservedHeight
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	// Viewport width is full terminal width (content handles its own margins)
	viewportWidth := m.width
	if viewportWidth < 1 {
		viewportWidth = 1
	}

	m.viewport.Width = viewportWidth
	m.viewport.Height = viewportHeight

	// Update input width:
	// Layout: inputLine has Width(width-4) with Padding(0,1) giving effective content width of (width-6)
	// The textinput renders as: prompt (2 chars "> ") + input value
	// So textinput.Width should be: (width - 6) - prompt_length(2) = width - 8
	// This ensures the full textinput fits within the padded line without overflow
	const promptLen = 2 // "> "
	inputWidth := m.width - 6 - promptLen
	if inputWidth < 10 {
		inputWidth = 10
	}
	m.input.Width = inputWidth

	// Update theme dimensions
	if m.theme != nil {
		m.theme.SetSize(m.width, m.height)
	}

	// Update command palette dimensions
	if m.commandPalette != nil {
		m.commandPalette.SetSize(m.width, m.height)
	}

	// Update tutorial overlay dimensions
	if m.tutorial != nil {
		m.tutorial.SetSize(m.width, m.height)
	}

	// Re-render viewport content with new dimensions
	m.updateViewport()

	// Also forward the resize to viewport so it can update internal state
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, vpCmd
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Emergency exit - Ctrl+Q always quits regardless of state
	keyStr := msg.String()
	if keyStr == "ctrl+q" {
		return m, tea.Quit
	}

	// Handle tutorial overlay first - it has priority when visible
	if m.tutorial != nil && m.tutorial.IsVisible() {
		var cmd tea.Cmd
		*m.tutorial, cmd = m.tutorial.Update(msg)
		return m, cmd
	}

	// Handle command palette first - it has priority over other modes
	if m.commandPalette != nil && m.commandPalette.IsVisible() {
		var cmd tea.Cmd
		m.commandPalette, cmd = m.commandPalette.Update(msg)
		return m, cmd
	}

	// Handle help overlay first - any key dismisses it except navigation
	if m.showHelp {
		switch keyStr {
		case "?", "esc", "q", "enter":
			m.showHelp = false
			return m, nil
		default:
			// Ignore other keys while help is shown
			return m, nil
		}
	}

	// Handle search mode keys
	if m.searchMode {
		return m.handleSearchKey(msg)
	}

	// Try vim mode handler first (if enabled)
	if m.vimHandler != nil && m.vimHandler.Enabled() {
		consumed, cmd := m.vimHandler.HandleKey(msg, &m.viewport, &m.input)
		if consumed {
			// Update inputMode based on vim mode
			m.inputMode = m.vimHandler.Mode() == VimModeInsert || m.vimHandler.Mode() == VimModeCommand
			return m, cmd
		}
	}

	// ==========================================================================
	// GLOBAL KEYS - work in any state
	// ==========================================================================

	switch keyStr {
	case "ctrl+c":
		if m.state == StateStreaming {
			// Cancel streaming (thread-safe) and clean up all related state
			m.cancel()
			m.state = StateReady
			m.isThinking = false
			m.streamingMsgID = ""
			m.pendingQuery = ""
			m.pendingMsgID = ""
			// Clean up cache and query tracking state
			m.lastCacheHit = 0 // Reset to CacheHitNone
			m.currentQueryTier = 0 // Reset tier
			m.currentQueryStart = time.Time{} // Zero time
			m.input.Focus()
			m.inputMode = true
			// Mark the last message as incomplete if it exists
			if lastMsg := m.conversation.GetLastMessage(); lastMsg != nil && lastMsg.Role == "assistant" && lastMsg.Content != "" {
				m.conversation.AppendToLast(" [incomplete - cancelled]")
			}
			return m, textinput.Blink
		}
		// Ctrl+C only cancels streaming or does nothing (removed quit behavior)
		return m, nil

	case "ctrl+p":
		// Toggle command palette
		if m.commandPalette != nil {
			m.commandPalette.Toggle()
			if m.commandPalette.IsVisible() {
				return m, m.commandPalette.Focus()
			}
		}
		return m, nil

	case "ctrl+f", "/":
		// Enter search mode (/ only when not in input mode or input is empty)
		if keyStr == "/" && m.inputMode && m.input.Value() != "" {
			// Let / be typed into input
			break
		}
		return m.enterSearchMode()

	case "ctrl+l":
		// Clear screen - clear conversation history
		m.conversation.ClearHistory()
		m.updateViewport()
		m.statusMsg = "Screen cleared"
		return m, nil

	case "ctrl+k":
		// Toggle multi-line input mode
		m.multiLineMode = !m.multiLineMode
		if m.multiLineMode {
			m.statusMsg = "Multi-line mode: Ctrl+Enter to submit"
		} else {
			m.statusMsg = "Single-line mode"
		}
		return m, nil

	case "ctrl+r":
		// Cycle through routing modes: local -> cloud -> hybrid -> local
		m.cycleRoutingMode()
		return m, nil

	case "ctrl+y":
		// Copy last assistant response to clipboard
		return m.copyLastResponse()

	case "?":
		// Toggle help overlay (only when not typing)
		if !m.inputMode || m.input.Value() == "" {
			m.showHelp = !m.showHelp
			return m, nil
		}
	}

	// ==========================================================================
	// TOAST DISMISSAL (non-blocking, works in any state)
	// ==========================================================================

	// Handle 'x' key to dismiss the oldest toast (when toasts are visible)
	if keyStr == "x" && m.HasToasts() {
		toasts := m.GetToasts()
		if len(toasts) > 0 {
			// Dismiss the first (oldest) toast
			m.DismissToast(toasts[0].ID)
			return m, nil
		}
	}

	// ==========================================================================
	// ERROR STATE HANDLING (blocking errors - kept for critical errors only)
	// ==========================================================================

	if m.state == StateError {
		switch keyStr {
		case "esc", "enter", " ":
			m.lastError = nil
			m.state = StateReady
			m.input.Focus()
			m.inputMode = true
			return m, textinput.Blink
		}
		return m, nil
	}

	// ==========================================================================
	// STREAMING STATE HANDLING
	// ==========================================================================

	if m.state == StateStreaming {
		switch keyStr {
		case "esc":
			// Cancel streaming (thread-safe)
			m.cancel()
			m.state = StateReady
			m.isThinking = false
			m.streamingMsgID = ""
			m.pendingQuery = ""
			m.pendingMsgID = ""
			m.input.Focus()
			m.inputMode = true
			return m, textinput.Blink
		}
		// Allow scrolling during streaming
		return m.handleNavigationKeys(msg)
	}

	// ==========================================================================
	// READY STATE - VIM-LIKE MODE HANDLING
	// ==========================================================================

	if m.state == StateReady {
		// Vim-like: when not in input mode, handle navigation keys directly
		if !m.inputMode {
			switch keyStr {
			case "i":
				// Enter input mode
				m.inputMode = true
				m.input.Focus()
				return m, textinput.Blink

			case "a":
				// Append mode - enter input mode at end
				m.inputMode = true
				m.input.Focus()
				m.input.CursorEnd()
				return m, textinput.Blink

			case "q":
				// Quit (only in normal mode)
				return m, tea.Quit

			default:
				// Handle navigation in normal mode
				return m.handleNavigationKeys(msg)
			}
		}

		// In input mode
		switch keyStr {
		case "tab":
			// Tab completion
			return m.handleTabCompletion()

		case "esc":
			// Exit input mode (vim-like)
			// Also hide completions if showing
			if m.showCompletions {
				m.clearCompletions()
				return m, nil
			}
			m.inputMode = false
			m.input.Blur()
			return m, nil

		case "enter":
			// Handle enter based on multi-line mode
			if m.multiLineMode {
				// In multi-line mode, enter adds a newline
				// Ctrl+Enter submits (handled below)
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
			// Single-line mode: submit if there's content
			if strings.TrimSpace(m.input.Value()) != "" {
				return m.submitInput()
			}

		case "ctrl+enter":
			// Submit in multi-line mode
			if strings.TrimSpace(m.input.Value()) != "" {
				return m.submitInput()
			}
		}

	// Any other key press clears completion state (user is typing new input)
		if !strings.HasPrefix(keyStr, "ctrl") && !strings.HasPrefix(keyStr, "alt") {
			m.clearCompletions()
		}

		// Forward to text input
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)

		// Update context cost estimate as user types
		m.updateContextCostEstimate()

		return m, cmd
	}

	return m, nil
}

// handleNavigationKeys handles viewport navigation keys.
// These work in both normal mode and during streaming.
func (m Model) handleNavigationKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	// Page navigation
	case "pgup", "ctrl+u":
		m.viewport.HalfViewUp()
		return m, nil

	case "pgdown", "ctrl+d":
		m.viewport.HalfViewDown()
		return m, nil

	// Line-by-line navigation (vim-like)
	case "up", "k":
		m.viewport.LineUp(1)
		return m, nil

	case "down", "j":
		m.viewport.LineDown(1)
		return m, nil

	// Go to top/bottom
	case "home", "g":
		m.viewport.GotoTop()
		return m, nil

	case "end", "G":
		m.viewport.GotoBottom()
		return m, nil
	}

	return m, nil
}

// handleSearchKey handles key presses when in search mode.
func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+f":
		// Exit search mode
		return m.exitSearchMode()

	case "enter", "n":
		// Navigate to next match (vim-like n)
		return m.nextSearchMatch()

	case "N":
		// Navigate to previous match (vim-like N)
		return m.prevSearchMatch()

	case "ctrl+n", "down":
		// Next match
		return m.nextSearchMatch()

	case "ctrl+p", "up":
		// Previous match
		return m.prevSearchMatch()

	default:
		// Forward to search input
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)

		// Update search query and find matches
		newQuery := m.searchInput.Value()
		if newQuery != m.searchQuery {
			m.searchQuery = newQuery
			m.findSearchMatches()
			m.updateViewport() // Re-render with highlights
		}

		return m, cmd
	}
}

// enterSearchMode activates search mode.
func (m Model) enterSearchMode() (tea.Model, tea.Cmd) {
	m.searchMode = true
	m.searchQuery = ""
	m.searchInput.Reset()
	m.searchInput.Focus()
	m.searchMatches = nil
	m.searchMatchIndex = 0
	return m, textinput.Blink
}

// exitSearchMode deactivates search mode.
func (m Model) exitSearchMode() (tea.Model, tea.Cmd) {
	m.searchMode = false
	m.searchQuery = ""
	m.searchMatches = nil
	m.searchMatchIndex = 0
	m.input.Focus()
	m.updateViewport() // Re-render without highlights
	return m, textinput.Blink
}

// findSearchMatches finds all matches of the search query in the conversation.
// Stores RUNE positions (not byte positions) for proper Unicode handling.
func (m *Model) findSearchMatches() {
	m.searchMatches = nil
	m.searchMatchIndex = 0

	if m.searchQuery == "" || m.conversation == nil {
		return
	}

	queryRunes := []rune(strings.ToLower(m.searchQuery))
	queryLen := len(queryRunes)
	if queryLen == 0 {
		return
	}

	messages := m.conversation.GetHistory()

	for msgIdx, msg := range messages {
		content := msg.GetDisplayContent()
		if content == "" {
			continue
		}

		// Convert to lowercase once for efficiency
		contentLower := strings.ToLower(content)
		textRunes := []rune(contentLower)

		// Find all case-insensitive matches by rune comparison
		for i := 0; i <= len(textRunes)-queryLen; i++ {
			matched := true
			for j := 0; j < queryLen; j++ {
				if textRunes[i+j] != queryRunes[j] {
					matched = false
					break
				}
			}
			if matched {
				m.searchMatches = append(m.searchMatches, SearchMatch{
					MessageIndex: msgIdx,
					StartPos:     i,            // RUNE position
					EndPos:       i + queryLen, // RUNE position
				})
				i += queryLen - 1 // Skip past this match
			}
		}
	}
}

// nextSearchMatch navigates to the next search match.
func (m Model) nextSearchMatch() (tea.Model, tea.Cmd) {
	if len(m.searchMatches) == 0 {
		return m, nil
	}

	m.searchMatchIndex = (m.searchMatchIndex + 1) % len(m.searchMatches)
	m.scrollToCurrentMatch()
	m.updateViewport()
	return m, nil
}

// prevSearchMatch navigates to the previous search match.
func (m Model) prevSearchMatch() (tea.Model, tea.Cmd) {
	if len(m.searchMatches) == 0 {
		return m, nil
	}

	m.searchMatchIndex--
	if m.searchMatchIndex < 0 {
		m.searchMatchIndex = len(m.searchMatches) - 1
	}
	m.scrollToCurrentMatch()
	m.updateViewport()
	return m, nil
}

// scrollToCurrentMatch scrolls the viewport to show the current search match.
func (m *Model) scrollToCurrentMatch() {
	if len(m.searchMatches) == 0 {
		return
	}

	// Get the current match
	match := m.searchMatches[m.searchMatchIndex]

	// Calculate approximate line position by estimating lines for each message
	// up to the matched message
	targetLine := 0
	messages := m.conversation.GetHistory()
	termWidth := m.width
	if termWidth <= 0 {
		termWidth = 80 // fallback default
	}
	// Account for message padding/margins (roughly 10 chars on each side)
	contentWidth := termWidth - 20
	if contentWidth < 40 {
		contentWidth = 40
	}

	for i := 0; i < match.MessageIndex && i < len(messages); i++ {
		msg := messages[i]
		content := msg.GetDisplayContent()
		// Estimate lines: header (2 lines) + content lines + spacing (2 lines between messages)
		contentLines := 1
		if len(content) > 0 && contentWidth > 0 {
			// Count actual newlines in content
			newlineCount := strings.Count(content, "\n")
			// Estimate wrapped lines based on content length (use rune count for proper Unicode handling)
			wrappedLines := (len([]rune(content)) + contentWidth - 1) / contentWidth
			// Use the larger of newline-based or wrap-based estimate
			if newlineCount+1 > wrappedLines {
				contentLines = newlineCount + 1
			} else {
				contentLines = wrappedLines
			}
		}
		// Header (role label) + content + message separator spacing
		targetLine += 2 + contentLines + 2
	}

	// Scroll to that position
	m.viewport.SetYOffset(targetLine)
}

// IsSearchMode returns true if search mode is active.
func (m *Model) IsSearchMode() bool {
	return m.searchMode
}

// GetSearchQuery returns the current search query.
func (m *Model) GetSearchQuery() string {
	return m.searchQuery
}

// GetSearchMatches returns the search matches.
func (m *Model) GetSearchMatches() []SearchMatch {
	return m.searchMatches
}

// GetCurrentMatchIndex returns the current match index.
func (m *Model) GetCurrentMatchIndex() int {
	return m.searchMatchIndex
}

func (m Model) handleStreamStart(msg StreamStartMsg) (tea.Model, tea.Cmd) {
	m.streamingMsgID = msg.MessageID
	m.streamingStats = model.NewStatistics()
	m.state = StateStreaming
	m.isThinking = true
	m.thinkingStart = msg.StartTime

	// Feature 4.2: Reset streaming buffer for new stream
	if m.streamingBuffer != nil {
		m.streamingBuffer.Reset()
	}
	m.lastStreamTick = time.Now()

	// Start spinner and 30fps tick for batched rendering
	return m, tea.Batch(m.spinner.Tick, streamTickCmd())
}

func (m Model) handleStreamToken(msg StreamTokenMsg) (tea.Model, tea.Cmd) {
	if msg.MessageID != m.streamingMsgID {
		return m, nil
	}

	// Record first token
	if msg.IsFirst && m.streamingStats != nil {
		m.streamingStats.RecordFirstToken()
		m.isThinking = false
	}

	// Feature 4.2: Add token to streaming buffer instead of immediately appending
	// This batches tokens for smooth rendering at 30fps
	if m.streamingBuffer != nil {
		m.streamingBuffer.Write(msg.Token)
		// Don't update viewport here - let the tick handler do it
		return m, nil
	}

	// Fallback if streaming buffer not initialized
	m.conversation.AppendToLast(msg.Token)
	m.updateViewport()
	m.viewport.GotoBottom()

	return m, nil
}

// handleStreamTick processes buffered tokens at 30fps for smooth rendering.
// This is Feature 4.2: Streaming Optimization.
func (m Model) handleStreamTick(msg StreamTickMsg) (tea.Model, tea.Cmd) {
	// Only process ticks during streaming
	if m.state != StateStreaming {
		return m, nil
	}

	// Feature 4.2: Flush streaming buffer if ready
	if m.streamingBuffer != nil {
		content, hasContent := m.streamingBuffer.Flush()
		if hasContent {
			// Append batched tokens to conversation
			m.conversation.AppendToLast(content)

			// Feature 4.2: Only update viewport if content actually changed
			if m.viewportOptimizer != nil {
				viewportContent := m.renderMessages()
				if m.viewportOptimizer.ShouldUpdate(viewportContent) {
					m.viewport.SetContent(viewportContent)
					m.viewportOptimizer.MarkClean()
				}
			} else {
				// Fallback without optimizer
				m.updateViewport()
			}

			// Auto-scroll to bottom during streaming
			m.viewport.GotoBottom()
		}
	}

	m.lastStreamTick = time.Now()

	// Schedule next tick to maintain 30fps
	return m, streamTickCmd()
}

func (m Model) handleStreamComplete(msg StreamCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.MessageID != m.streamingMsgID {
		return m, nil
	}

	// Feature 4.2: Force flush any remaining buffered tokens
	if m.streamingBuffer != nil {
		content, hasContent := m.streamingBuffer.ForceFlush()
		if hasContent {
			m.conversation.AppendToLast(content)
		}
	}

	// Finalize the message with statistics
	if msg.Stats != nil {
		m.conversation.FinalizeLast(msg.Stats)
	} else if m.streamingStats != nil {
		// Use our tracked stats
		m.conversation.FinalizeLast(m.streamingStats)
	}

	// =========================================================================
	// CACHE STORAGE - Store completed response in cache for future lookups
	// =========================================================================
	if m.pendingQuery != "" && m.pendingMsgID == msg.MessageID {
		// Get the completed response from the last message
		lastMsg := m.conversation.GetLastMessage()
		if lastMsg != nil && lastMsg.Content != "" {
			// Get the tier that was used for this response
			tier := "Local"
			if m.lastRouting != nil {
				tier = m.lastRouting.Tier.String()
			}
			// Store in cache for future lookups
			m.storeInCache(m.pendingQuery, lastMsg.Content, tier)
		}
		// Clear pending state
		m.pendingQuery = ""
		m.pendingMsgID = ""
	}

	// =========================================================================
	// SESSION STATS - Record query result for cost tracking
	// =========================================================================
	if m.sessionStats != nil {
		// Calculate latency
		latencyMs := uint64(time.Since(m.currentQueryStart).Milliseconds())

		// Get token counts from stats if available
		var inputTokens, outputTokens uint32
		if msg.Stats != nil {
			// PromptTokens may not be set by Ollama - estimate from message content
			inputTokens = uint32(msg.Stats.PromptTokens)
			outputTokens = uint32(msg.Stats.CompletionTokens)
		}
		// If input tokens not tracked, estimate from conversation
		if inputTokens == 0 {
			// Rough estimate: sum of all message tokens / 4 chars per token
			for _, convMsg := range m.conversation.GetHistory() {
				inputTokens += uint32((len(convMsg.Content) + 3) / 4)
			}
		}

		// Create and record query result with ACTUAL tier used
		result := router.NewQueryResult(
			"", // Response text not needed for stats
			m.currentQueryTier,
			inputTokens,
			outputTokens,
			latencyMs,
		)
		m.sessionStats.RecordQuery(result)
	}

	// Update state
	m.state = StateReady
	m.isThinking = false
	m.streamingMsgID = ""
	m.streamingStats = nil
	m.clearCancelFunc()

	// Update viewport
	m.updateViewport()

	// Focus input
	m.input.Focus()

	return m, textinput.Blink
}

func (m Model) handleStreamError(msg StreamErrorMsg) (tea.Model, tea.Cmd) {
	// Check if this is a model not found error and we can fallback to cloud
	if ollama.IsModelNotFound(msg.Error) && m.HasCloudClient() {
		// Add a system message about the fallback
		m.conversation.AddSystemMessage(fmt.Sprintf(
			"Local model not found. Routing to cloud instead. To download the model, run: ollama pull %s",
			m.modelName,
		))

		// Re-route to cloud
		m.currentQueryTier = router.TierCloud
		m.state = StateReady
		m.isThinking = false
		m.streamingMsgID = ""
		m.streamingStats = nil
		m.clearCancelFunc()

		// Update viewport and focus input
		m.updateViewport()
		m.input.Focus()

		// The user can re-submit their query - don't auto-retry to avoid confusion
		return m, nil
	}

	// Reset streaming state
	m.isThinking = false
	m.streamingMsgID = ""
	m.streamingStats = nil
	m.clearCancelFunc()

	// Clear pending cache state on error (don't cache failed responses)
	m.pendingQuery = ""
	m.pendingMsgID = ""

	// Store error for @error mention retrieval
	ctxmention.StoreLastError("Streaming Error: " + msg.Error.Error())

	// Use NON-BLOCKING toast instead of blocking StateError
	// This allows user to continue scrolling/reading while error is shown
	if m.toastManager != nil {
		m.toastManager.AddError("Streaming Error: " + msg.Error.Error())
		m.state = StateReady // Stay in ready state, don't block
		m.input.Focus()
		return m, tea.Batch(textinput.Blink, components.ToastTickCmd())
	}

	// Fallback to blocking error for critical errors (toast manager not initialized)
	errMsg := SmartErrorMsg("Streaming Error", msg.Error.Error())
	m.lastError = &errMsg
	m.state = StateError
	m.input.Focus()

	return m, nil
}

// handleRoutingFallback updates the message routing tier when cloud fails and falls back to local.
func (m Model) handleRoutingFallback(msg RoutingFallbackMsg) (tea.Model, tea.Cmd) {
	// Find the message and update its routing tier
	if m.conversation != nil {
		messages := m.conversation.GetHistory()
		for _, histMsg := range messages {
			if histMsg.ID == msg.MessageID {
				// Update tier to show fallback
				histMsg.RoutingTier = msg.ToTier + " (fallback)"
				histMsg.RoutingCost = 0 // Local is free
				break
			}
		}
	}

	// Update current query tier to local (for session stats)
	m.currentQueryTier = router.TierLocal

	// Refresh viewport to show the fallback indicator
	m.updateViewport()

	return m, nil
}

func (m Model) handleOllamaStatus(msg OllamaStatusMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil || !msg.Running {
		// Build error message
		errText := "Cannot connect to Ollama service"
		if msg.Error != nil {
			errText = msg.Error.Error()
		}

		// Store error for @error mention retrieval
		fullErrText := "Ollama Not Running: " + errText
		ctxmention.StoreLastError(fullErrText)

		// Use NON-BLOCKING toast - user can still use cloud mode or read docs
		if m.toastManager != nil {
			m.toastManager.AddWarning("Ollama: " + errText)
			return m, components.ToastTickCmd()
		}

		// Fallback to blocking error
		errMsg := SmartErrorMsg("Ollama Not Running", errText)
		m.lastError = &errMsg
		m.state = StateError
	}
	return m, nil
}

func (m Model) handleOllamaModels(msg OllamaModelsMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		// Store error for @error mention retrieval
		ctxmention.StoreLastError("Failed to List Models: " + msg.Error.Error())

		// Use NON-BLOCKING toast
		if m.toastManager != nil {
			m.toastManager.AddError("Failed to list models: " + msg.Error.Error())
			return m, components.ToastTickCmd()
		}

		// Fallback to blocking error
		errMsg := SmartErrorMsg("Failed to List Models", msg.Error.Error())
		m.lastError = &errMsg
		m.state = StateError
	}
	return m, nil
}

func (m Model) handleModelSwitched(msg OllamaModelSwitchedMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		// Store error for @error mention retrieval
		ctxmention.StoreLastError("Failed to Switch Model: " + msg.Error.Error())

		// Use NON-BLOCKING toast
		if m.toastManager != nil {
			m.toastManager.AddError("Failed to switch model: " + msg.Error.Error())
			return m, components.ToastTickCmd()
		}

		// Fallback to blocking error
		errMsg := SmartErrorMsg("Failed to Switch Model", msg.Error.Error())
		m.lastError = &errMsg
		m.state = StateError
		return m, nil
	}

	m.modelName = msg.Model
	if m.ollama != nil {
		m.ollama.SetModel(msg.Model)
	}
	m.conversation.Model = msg.Model

	// Add system message to provide user feedback
	m.conversation.AddSystemMessage("✓ Switched to model: " + msg.Model)
	m.updateViewport()

	return m, nil
}

// =============================================================================
// INPUT SUBMISSION
// =============================================================================
// Note: submitInput() implementation moved to input.go for better organization

// tierToCloudModel converts a router tier to an OpenRouter model name.
func (m Model) tierToCloudModel(tier router.Tier) string {
	switch tier {
	case router.TierAuto:
		return "auto"
	case router.TierHaiku:
		return "haiku"
	case router.TierSonnet:
		return "sonnet"
	case router.TierOpus:
		return "opus"
	case router.TierGpt4o:
		return "gpt4o"
	case router.TierCloud:
		return "auto"
	default:
		return "auto"
	}
}

// =============================================================================
// COMMAND HANDLING
// =============================================================================
// Note: handleCommand() implementation moved to commands.go using the command
// registry pattern. The 846-line monolithic function has been broken into
// individual, testable handlers.

// =============================================================================
// STREAMING
// =============================================================================

// startStreamingLocal starts streaming from the local Ollama client.
func (m Model) startStreamingLocal(messageID string) tea.Cmd {
	// Get conversation messages to send to the main model
	// The main model handles the actual Ollama streaming via StreamRequestMsg
	messages := m.conversation.ToOllamaMessages()

	return func() tea.Msg {
		// Return a StreamRequestMsg that the main model will handle
		// The main model has access to programRef and can do async streaming
		return StreamRequestMsg{
			MessageID: messageID,
			Messages:  messages,
			UseCloud:  false,
		}
	}
}

// startStreamingCloud starts streaming from the OpenRouter cloud client.
func (m Model) startStreamingCloud(messageID string, cloudModel string, tierName string) tea.Cmd {
	// Get conversation messages to send to the main model
	messages := m.conversation.ToOllamaMessages()

	return func() tea.Msg {
		// Return a StreamRequestMsg with cloud routing info
		return StreamRequestMsg{
			MessageID:  messageID,
			Messages:   messages,
			UseCloud:   true,
			CloudModel: cloudModel,
			CloudTier:  tierName,
		}
	}
}

// startStreamingLocalWithContent starts streaming with custom content for the last user message.
// This is used when @ mentions have been expanded and we need to send the expanded content
// to the LLM while showing the original content in the UI.
func (m Model) startStreamingLocalWithContent(messageID string, expandedContent string) tea.Cmd {
	// Get conversation messages, but replace the last user message content with expanded content
	messages := m.conversation.ToOllamaMessagesWithOverride(expandedContent)

	return func() tea.Msg {
		return StreamRequestMsg{
			MessageID: messageID,
			Messages:  messages,
			UseCloud:  false,
		}
	}
}

// startStreamingCloudWithContent starts cloud streaming with custom content for the last user message.
// This is used when @ mentions have been expanded and we need to send the expanded content
// to the LLM while showing the original content in the UI.
func (m Model) startStreamingCloudWithContent(messageID string, cloudModel string, tierName string, expandedContent string) tea.Cmd {
	// Get conversation messages, but replace the last user message content with expanded content
	messages := m.conversation.ToOllamaMessagesWithOverride(expandedContent)

	return func() tea.Msg {
		return StreamRequestMsg{
			MessageID:  messageID,
			Messages:   messages,
			UseCloud:   true,
			CloudModel: cloudModel,
			CloudTier:  tierName,
		}
	}
}

// StartStreamingCmd creates a command that streams from Ollama.
// This should be called after receiving StreamStartMsg.
func (m *Model) StartStreamingCmd(messageID string) tea.Cmd {
	// Cancel any previous streaming (thread-safe)
	m.cancel()

	// Check ollama client before creating the cmd
	if m.ollama == nil {
		return func() tea.Msg {
			return StreamErrorMsg{
				MessageID: messageID,
				Error:     ollama.ErrNotRunning,
			}
		}
	}

	// Create the cancel context and store it (thread-safe)
	ctx, cancel := context.WithCancel(context.Background())
	m.setCancelFunc(cancel)

	// Capture needed values for the closure to avoid accessing m from goroutine
	ollamaClient := m.ollama
	modelName := m.modelName
	messages := m.conversation.ToOllamaMessages()

	return func() tea.Msg {
		stats := model.NewStatistics()
		isFirst := true

		err := ollamaClient.ChatStream(ctx, modelName, messages, func(chunk ollama.StreamChunk) {
			if chunk.Error != nil {
				return
			}

			if chunk.Content != "" {
				if isFirst {
					stats.RecordFirstToken()
					isFirst = false
				}
			}

			if chunk.Done {
				stats.Finalize(chunk.CompletionTokens)
			}
		})

		if err != nil {
			return StreamErrorMsg{
				MessageID: messageID,
				Error:     err,
			}
		}

		return StreamCompleteMsg{
			MessageID: messageID,
			Stats:     stats,
		}
	}
}

// =============================================================================
// VIEWPORT UPDATE
// =============================================================================

func (m *Model) updateViewport() {
	content := m.renderMessages()
	m.viewport.SetContent(content)
}

// =============================================================================
// OLLAMA CHECK
// =============================================================================

func (m *Model) checkOllama() tea.Cmd {
	client := m.ollama // Capture before closure to avoid race
	return func() tea.Msg {
		if client == nil {
			return OllamaStatusMsg{Running: false, Error: ollama.ErrNotRunning}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := client.CheckRunning(ctx)
		return OllamaStatusMsg{
			Running: err == nil,
			Error:   err,
		}
	}
}

// =============================================================================
// GETTERS AND SETTERS
// =============================================================================

// SetOllamaClient sets the Ollama client.
func (m *Model) SetOllamaClient(client *ollama.Client) {
	m.ollama = client
	if client != nil {
		m.modelName = client.GetDefaultModel()
	}
}

// SetCloudClient sets the OpenRouter cloud client.
func (m *Model) SetCloudClient(client *cloud.OpenRouterClient) {
	m.cloudClient = client
}

// GetCloudClient returns the OpenRouter cloud client.
func (m *Model) GetCloudClient() *cloud.OpenRouterClient {
	return m.cloudClient
}

// HasCloudClient returns true if a cloud client is configured and ready.
func (m *Model) HasCloudClient() bool {
	return m.cloudClient != nil && m.cloudClient.IsConfigured()
}

// GetConversation returns the current conversation.
func (m *Model) GetConversation() *model.Conversation {
	return m.conversation
}

// SetConversation sets the current conversation.
func (m *Model) SetConversation(conv *model.Conversation) {
	if conv == nil {
		conv = model.NewConversation()
	}
	m.conversation = conv
	m.updateViewport()
}

// GetState returns the current state.
func (m *Model) GetState() State {
	return m.state
}

// GetCurrentContext returns the current UI context for context-sensitive help.
// This determines which keybindings are shown in the help overlay, following
// lazygit's pattern of showing only currently-active bindings.
func (m *Model) GetCurrentContext() HelpContext {
	// Priority order: overlays first, then mode-specific contexts

	// Check for overlay states first
	if m.showHelp {
		return ContextHelp
	}
	if m.commandPalette != nil && m.commandPalette.IsVisible() {
		return ContextPalette
	}

	// Check for search mode
	if m.searchMode {
		return ContextSearch
	}

	// Check for error state
	if m.state == StateError {
		return ContextError
	}

	// Check for streaming state
	if m.state == StateStreaming {
		return ContextStreaming
	}

	// Ready state - check input mode
	if m.inputMode {
		return ContextInput
	}

	// Default: normal navigation mode
	return ContextNormal
}

// GetModelName returns the current model name.
func (m *Model) GetModelName() string {
	return m.modelName
}

// SetModelName sets the model name.
func (m *Model) SetModelName(name string) {
	m.modelName = name
}

// GetMode returns the current routing mode.
func (m *Model) GetMode() string {
	return m.mode
}

// SetMode sets the routing mode.
func (m *Model) SetMode(mode string) {
	m.mode = mode
}

// GetGPU returns the GPU name.
func (m *Model) GetGPU() string {
	return m.gpu
}

// SetGPU sets the GPU name.
func (m *Model) SetGPU(gpu string) {
	m.gpu = gpu
}

// GetOfflineMode returns the offline mode state.
func (m *Model) GetOfflineMode() bool {
	return m.offlineMode
}

// SetOfflineMode sets the offline mode state (IL5 SC-7).
func (m *Model) SetOfflineMode(offline bool) {
	m.offlineMode = offline
}

// IsStreaming returns true if currently streaming.
func (m *Model) IsStreaming() bool {
	return m.state == StateStreaming
}

// GetContextPercent returns the context usage percentage.
func (m *Model) GetContextPercent() float64 {
	return m.conversation.GetContextPercent()
}

// =============================================================================
// ROUTING METHODS
// =============================================================================

// cycleRoutingMode cycles through routing modes: local -> cloud -> auto -> local
func (m *Model) cycleRoutingMode() {
	switch m.routingMode {
	case "local":
		m.routingMode = "cloud"
	case "cloud":
		m.routingMode = "auto"
	case "auto", "hybrid": // hybrid is alias for auto
		m.routingMode = "local"
	default:
		m.routingMode = "local"
	}
	// Also update the mode field for status bar display
	m.mode = m.routingMode

	// Update the global config's routing mode
	if cfg := config.Global(); cfg != nil {
		cfg.Routing.DefaultMode = m.routingMode
	}

	// Show status message in conversation
	modeDisplay := m.routingMode
	if m.routingMode == "auto" {
		modeDisplay = "auto (OpenRouter routing)"
	}
	m.conversation.AddSystemMessage("Routing mode: " + modeDisplay)
	m.updateViewport()
}

// copyLastResponse copies the last assistant response to the clipboard.
// Returns the model and a command (nil on success, or error handling).
func (m Model) copyLastResponse() (tea.Model, tea.Cmd) {
	// Find the last assistant message
	if m.conversation == nil {
		m.statusMsg = "No conversation to copy from"
		return m, nil
	}

	messages := m.conversation.GetHistory()
	var lastAssistantMsg *model.Message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == model.RoleAssistant {
			lastAssistantMsg = messages[i]
			break
		}
	}

	if lastAssistantMsg == nil || lastAssistantMsg.Content == "" {
		m.statusMsg = "No response to copy"
		m.conversation.AddSystemMessage("No assistant response to copy")
		m.updateViewport()
		return m, nil
	}

	// Copy to clipboard using atotto/clipboard
	err := copyToClipboard(lastAssistantMsg.Content)
	if err != nil {
		m.statusMsg = "Failed to copy"
		m.conversation.AddSystemMessage("Failed to copy to clipboard: " + err.Error())
		m.updateViewport()
		return m, nil
	}

	// Show success feedback
	contentLen := len(lastAssistantMsg.Content)
	var sizeInfo string
	if contentLen < 1000 {
		sizeInfo = fmt.Sprintf("%d chars", contentLen)
	} else {
		sizeInfo = fmt.Sprintf("%.1fK chars", float64(contentLen)/1000)
	}
	m.statusMsg = "Copied!"
	m.conversation.AddSystemMessage("Copied response to clipboard (" + sizeInfo + ")")
	m.updateViewport()

	return m, nil
}

// GetRoutingMode returns the current routing mode.
func (m *Model) GetRoutingMode() string {
	return m.routingMode
}

// SetRoutingMode sets the routing mode.
func (m *Model) SetRoutingMode(mode string) {
	m.routingMode = mode
}

// GetLastRouting returns the last routing decision.
func (m *Model) GetLastRouting() *router.RoutingDecision {
	return m.lastRouting
}

// GetSessionStats returns the session statistics.
func (m *Model) GetSessionStats() *router.SessionStats {
	return m.sessionStats
}

// recordQuery records a query result in session statistics.
func (m *Model) recordQuery(result router.QueryResult) {
	if m.sessionStats != nil {
		m.sessionStats.RecordQuery(result)
	}
}

// =============================================================================
// CACHE METHODS
// =============================================================================

// SetCacheManager sets the cache manager for query caching.
func (m *Model) SetCacheManager(cm *cache.CacheManager) {
	m.cacheManager = cm
}

// GetCacheManager returns the cache manager.
func (m *Model) GetCacheManager() *cache.CacheManager {
	return m.cacheManager
}

// GetLastCacheHit returns the type of the last cache hit.
func (m *Model) GetLastCacheHit() cache.CacheHitType {
	return m.lastCacheHit
}

// checkCache checks the cache for a response to the given query.
// Returns the cached response and hit type, or empty string and CacheHitNone if not found.
func (m *Model) checkCache(query string) (string, cache.CacheHitType) {
	if m.cacheManager == nil {
		return "", cache.CacheHitNone
	}
	return m.cacheManager.Lookup(query)
}

// storeInCache stores a query-response pair in the cache.
func (m *Model) storeInCache(query, response, tier string) {
	if m.cacheManager == nil {
		return
	}
	m.cacheManager.Store(query, response, tier)
}

// =============================================================================
// TOOL HANDLER METHODS
// =============================================================================

// handleToolCallRequested handles a tool call request from the LLM.
func (m Model) handleToolCallRequested(msg ToolCallRequestedMsg) (tea.Model, tea.Cmd) {
	if !m.toolsEnabled || m.toolExecutor == nil {
		// Tools disabled - just ignore the tool call
		return m, nil
	}

	// Add system message showing the tool call
	m.conversation.AddSystemMessage("Tool call: " + msg.ToolName)
	m.updateViewport()

	// Execute the tool
	return m, m.executeToolCmd(msg)
}

// handleToolResult handles a tool execution result.
func (m Model) handleToolResult(msg ToolResultMsg) (tea.Model, tea.Cmd) {
	// Determine output content and success status
	output := msg.Output
	if msg.Error != "" {
		output = msg.Error

		// Store tool error for @error mention retrieval
		ctxmention.StoreLastError("Tool Error (" + msg.ToolName + "): " + msg.Error)
	}

	// Add tool result to conversation using the proper API method
	// (this also updates timestamps, token estimates, and title)
	m.conversation.AddToolMessage(msg.ToolName, output, msg.Success)
	m.updateViewport()

	// If in agentic loop, continue with the tool result
	if m.agenticLoop != nil {
		// Continue conversation with tool result
		return m, nil
	}

	return m, nil
}

// handleToolPermission handles a tool permission request.
func (m Model) handleToolPermission(msg ToolPermissionMsg) (tea.Model, tea.Cmd) {
	// For now, auto-approve all tool calls (permission system not fully implemented)
	// In the future, this should show a confirmation dialog
	return m, func() tea.Msg {
		return ToolPermissionResponseMsg{
			MessageID: msg.MessageID,
			ToolID:    msg.ToolID,
			Allowed:   true,
		}
	}
}

// handleToolPermissionResponse handles a user's response to a tool permission request.
func (m Model) handleToolPermissionResponse(msg ToolPermissionResponseMsg) (tea.Model, tea.Cmd) {
	if !msg.Allowed {
		// User denied the tool call
		m.conversation.AddSystemMessage("Tool call denied: " + msg.ToolID)
		m.updateViewport()
		return m, nil
	}

	// Permission granted - the tool should already be executing
	return m, nil
}

// executeToolCmd creates a command to execute a tool asynchronously.
func (m *Model) executeToolCmd(msg ToolCallRequestedMsg) tea.Cmd {
	executor := m.toolExecutor // Capture before closure to avoid race
	if executor == nil {
		return func() tea.Msg {
			return ToolResultMsg{
				MessageID: msg.MessageID,
				ToolName:  msg.ToolName,
				ToolID:    msg.ToolID,
				Output:    "",
				Error:     "Tool executor not initialized",
				Success:   false,
			}
		}
	}
	return func() tea.Msg {
		// Create a ToolCall from the message
		call := tools.ToolCall{
			Name:   msg.ToolName,
			Params: msg.Arguments,
		}

		// Execute the tool with a timeout context to prevent hangs
		// 5 minutes should be enough for most tools, prevents infinite execution
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		result := executor.Execute(ctx, call)

		// Check if context was cancelled due to timeout
		if ctx.Err() == context.DeadlineExceeded && !result.Success {
			result.Error = "Tool execution timed out after 5 minutes"
		}

		return ToolResultMsg{
			MessageID: msg.MessageID,
			ToolName:  msg.ToolName,
			ToolID:    msg.ToolID,
			Output:    result.Output,
			Error:     result.Error,
			Success:   result.Success,
			Duration:  result.Duration,
		}
	}
}

// =============================================================================
// CLASSIFICATION METHODS (NIST 800-53 AC-4)
// =============================================================================

// GetClassificationLevel returns the current session classification level.
func (m *Model) GetClassificationLevel() security.ClassificationLevel {
	return m.classificationLevel
}

// SetClassificationLevel sets the session classification level.
// This affects routing decisions - CUI and higher will NEVER route to cloud.
func (m *Model) SetClassificationLevel(level security.ClassificationLevel) {
	m.classificationLevel = level
	// Update enforcer session ID if needed
	if m.classificationEnforcer != nil && m.conversation != nil {
		m.classificationEnforcer.SetSessionID(m.conversation.ID)
	}
}

// GetClassificationEnforcer returns the classification enforcer.
func (m *Model) GetClassificationEnforcer() *security.ClassificationEnforcer {
	return m.classificationEnforcer
}

// CanRouteToCloud returns true if the current classification allows cloud routing.
// This is a convenience method for UI display.
func (m *Model) CanRouteToCloud() bool {
	if m.classificationEnforcer == nil {
		return m.classificationLevel == security.ClassificationUnclassified
	}
	return m.classificationEnforcer.CanRouteToCloud(m.classificationLevel)
}

// GetClassificationRestrictions returns a human-readable description of
// the current classification's routing restrictions.
func (m *Model) GetClassificationRestrictions() string {
	if m.classificationEnforcer == nil {
		if m.classificationLevel == security.ClassificationUnclassified {
			return "UNCLASSIFIED: No routing restrictions"
		}
		return "Cloud routing BLOCKED - local processing only (AC-4)"
	}
	return m.classificationEnforcer.GetClassificationRestrictions(m.classificationLevel)
}

// =============================================================================
// CONTEXT COST ESTIMATION
// =============================================================================

// updateContextCostEstimate updates the context token and cost estimates
// based on @mentions in the current input. Called on every keystroke.
func (m *Model) updateContextCostEstimate() {
	inputText := m.input.Value()

	// Quick check: if no @ mentions, clear estimates and active context
	if !ctxmention.HasMentions(inputText) {
		m.contextTokenEstimate = 0
		m.contextCostEstimate = 0
		if m.activeContext != nil {
			m.activeContext.Clear()
			m.showContextBar = false
		}
		return
	}

	// Don't expand yet - just estimate size for display
	// Show context bar if we have active mentions
	if m.activeContext != nil && m.activeContext.HasItems() {
		m.showContextBar = true
	}
	// This is fast and doesn't fetch actual content
	if m.contextExpander == nil {
		m.contextTokenEstimate = 0
		m.contextCostEstimate = 0
		return
	}

	// Parse mentions to update active context display
	if m.activeContext != nil {
		m.updateActiveContext(inputText)
	}

	// Estimate tokens for the expanded context using fast heuristics
	// This does NOT fetch file content - safe to call on every keystroke
	estimatedTokens := m.contextExpander.EstimateContextSizeFast(inputText)
	m.contextTokenEstimate = estimatedTokens

	// Estimate cost based on current routing decision
	// Use last routing tier as a hint, or default to Auto
	tier := router.TierAuto
	if m.lastRouting != nil {
		tier = m.lastRouting.Tier
	}

	// Estimate cost based on estimated tokens and tier
	m.contextCostEstimate = router.EstimateCost(estimatedTokens, tier)
}

// GetContextTokenEstimate returns the current context token estimate.
func (m *Model) GetContextTokenEstimate() int {
	return m.contextTokenEstimate
}

// GetContextCostEstimate returns the current context cost estimate in cents.
func (m *Model) GetContextCostEstimate() float64 {
	return m.contextCostEstimate
}

// =============================================================================
// SESSION RESUME AND SEARCH HANDLERS
// =============================================================================

// handleSessionResume initiates loading a session for resume with context display.
func (m Model) handleSessionResume(msg SessionResumeMsg) (tea.Model, tea.Cmd) {
	// Return a command that will load the session and return SessionResumedMsg
	sessionID := msg.SessionID
	return m, func() tea.Msg {
		// This will be handled by the main model which has access to storage
		// For now, return a message to indicate we need session loading
		return LoadConversationMsg{ID: sessionID}
	}
}

// handleSessionResumed processes a successfully resumed session.
func (m Model) handleSessionResumed(msg SessionResumedMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.conversation.AddSystemMessage("Failed to resume session: " + msg.Error.Error())
		m.updateViewport()
		return m, nil
	}

	// Display context about the resumed session
	displayID := msg.SessionID
	if len(displayID) > 12 {
		displayID = displayID[:12]
	}
	contextMsg := "Resumed session " + displayID + " (" +
		formatIntSimple(msg.MessageCount) + " messages, created " + msg.CreatedAt + ")"
	m.conversation.AddSystemMessage(contextMsg)
	m.updateViewport()
	return m, nil
}

// handleSessionSearch initiates a search for sessions by message content.
func (m Model) handleSessionSearch(msg SessionSearchMsg) (tea.Model, tea.Cmd) {
	// Return a command that will search and return SessionSearchResultMsg
	query := msg.Query
	return m, func() tea.Msg {
		// This will be handled by the main model which has access to storage
		// For now, add a placeholder message
		return SessionSearchResultMsg{
			Query:    query,
			Sessions: nil,
			Error:    nil,
		}
	}
}

// handleSessionSearchResult processes search results.
func (m Model) handleSessionSearchResult(msg SessionSearchResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.conversation.AddSystemMessage("Search failed: " + msg.Error.Error())
		m.updateViewport()
		return m, nil
	}

	if len(msg.Sessions) == 0 {
		m.conversation.AddSystemMessage("No sessions found matching: " + msg.Query)
		m.updateViewport()
		return m, nil
	}

	// Format search results
	var result strings.Builder
	result.WriteString("Sessions matching \"" + msg.Query + "\":\n")
	result.WriteString("-----------------------------------------------------\n")

	for _, s := range msg.Sessions {
		displayID := s.ID
		if len(displayID) > 12 {
			displayID = displayID[:12]
		}
		result.WriteString(displayID + " | " +
			s.UpdatedAt + " | " +
			formatIntSimple(s.MessageCount) + " msgs | " +
			truncatePreview(s.Preview, 30) + "\n")
	}

	result.WriteString("\nUse /resume <id> to load a session")
	m.conversation.AddSystemMessage(result.String())
	m.updateViewport()
	return m, nil
}

// handleCommandExecution executes a command selected from the command palette.
func (m Model) handleCommandExecution(msg components.ExecuteCommandMsg) (tea.Model, tea.Cmd) {
	if msg.Command == nil {
		return m, nil
	}

	// Insert command into input field
	m.input.SetValue(msg.Command.Name)
	m.input.Focus()
	m.inputMode = true

	// If command has no arguments, execute it immediately
	if len(msg.Command.Args) == 0 {
		return m.submitInput()
	}

	// Otherwise, let user add arguments
	m.input.CursorEnd()
	m.input.SetValue(msg.Command.Name + " ")

	return m, textinput.Blink
}

// formatIntSimple converts an int to string without fmt.
func formatIntSimple(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + formatIntSimple(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// truncatePreview truncates a string to maxLen characters.
func truncatePreview(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// =============================================================================
// ACTIVE CONTEXT MANAGEMENT
// =============================================================================

// updateActiveContext updates the active context based on parsed mentions.
func (m *Model) updateActiveContext(inputText string) {
	if m.contextExpander == nil || m.activeContext == nil {
		return
	}

	// Parse mentions from input
	parser := ctxmention.NewParser()
	mentions, _ := parser.Parse(inputText)

	// Clear non-pinned items
	m.activeContext.Clear()

	// Add each mention as a context item with token estimate
	for _, mention := range mentions {
		// Estimate tokens for this specific mention type
		tokens := m.estimateTokensForMention(mention)

		// Create context item
		item := components.CreateContextItemFromMention(
			mention.Type.String(),
			mention.Path,
			tokens,
		)

		m.activeContext.AddItem(item)
	}
}

// estimateTokensForMention estimates token count for a single mention.
func (m *Model) estimateTokensForMention(mention ctxmention.Mention) int {
	switch mention.Type {
	case ctxmention.MentionFile:
		// Typical source file: ~5KB = ~1250 tokens
		return 1250
	case ctxmention.MentionClipboard:
		// Typical clipboard: ~500 chars = ~125 tokens
		return 125
	case ctxmention.MentionGit:
		// Git context (commits + status + diff): ~2KB = ~500 tokens
		return 500
	case ctxmention.MentionCodebase:
		// Codebase tree summary: ~10KB = ~2500 tokens
		return 2500
	case ctxmention.MentionLastError:
		// Error message: ~200 chars = ~50 tokens
		return 50
	case ctxmention.MentionURL:
		// Web page content: ~3KB = ~750 tokens
		return 750
	default:
		return 100
	}
}

// GetActiveContext returns the active context.
func (m *Model) GetActiveContext() *components.ActiveContext {
	return m.activeContext
}

// ToggleContextBarExpanded toggles the expanded state of the context bar.
func (m *Model) ToggleContextBarExpanded() {
	m.contextBarExpanded = !m.contextBarExpanded
}

// =============================================================================
// PROGRESS TRACKING METHODS (for agentic loops)
// =============================================================================

// handleProgressStart handles the start of a multi-step operation
func (m Model) handleProgressStart(msg ProgressStartMsg) (tea.Model, tea.Cmd) {
	m.progressIndicator = components.NewProgressIndicator(msg.TotalSteps)
	m.progressIndicator.Width = m.width - 4 // Account for margins
	m.showProgress = true
	m.updateViewport()
	return m, nil
}

// handleProgressStep handles updates to the current step
func (m Model) handleProgressStep(msg ProgressStepMsg) (tea.Model, tea.Cmd) {
	if m.progressIndicator != nil {
		m.progressIndicator.StartStep(msg.CurrentStep, msg.StepTitle)
		m.progressIndicator.SetTool(msg.Tool, msg.ToolArgs)
		m.updateViewport()
	}
	return m, nil
}

// handleProgressUpdate handles progress updates without changing step
func (m Model) handleProgressUpdate(msg ProgressUpdateMsg) (tea.Model, tea.Cmd) {
	// Just refresh the viewport to show updated elapsed time
	m.updateViewport()
	return m, nil
}

// handleProgressComplete handles completion of the multi-step operation
func (m Model) handleProgressComplete(msg ProgressCompleteMsg) (tea.Model, tea.Cmd) {
	if m.progressIndicator != nil {
		if msg.Success {
			m.progressIndicator.Complete()
		} else {
			m.progressIndicator.Error()
		}

		// Add a system message with the completion status
		statusMsg := "Operation complete"
		if msg.Message != "" {
			statusMsg = msg.Message
		}
		m.conversation.AddSystemMessage(statusMsg)

		// Hide progress indicator after a brief delay
		m.showProgress = false
		m.updateViewport()
	}
	return m, nil
}

// handleProgressCanceled handles cancellation of the operation
func (m Model) handleProgressCanceled(msg ProgressCanceledMsg) (tea.Model, tea.Cmd) {
	if m.progressIndicator != nil {
		m.progressIndicator.Cancel()
		m.conversation.AddSystemMessage("Operation canceled at step " + formatIntSimple(msg.AtStep))
		m.showProgress = false
		m.updateViewport()
	}
	return m, nil
}

// handleProgressError handles errors during the operation
func (m Model) handleProgressError(msg ProgressErrorMsg) (tea.Model, tea.Cmd) {
	if m.progressIndicator != nil {
		m.progressIndicator.Error()
		m.conversation.AddSystemMessage("Error at step " + formatIntSimple(msg.AtStep) + ": " + msg.Error.Error())
		m.showProgress = false
		m.updateViewport()
	}
	return m, nil
}

// GetProgressIndicator returns the current progress indicator (for rendering)
func (m *Model) GetProgressIndicator() *components.ProgressIndicator {
	return m.progressIndicator
}

// IsShowingProgress returns true if progress indicator is visible
func (m *Model) IsShowingProgress() bool {
	return m.showProgress && m.progressIndicator != nil
}

// =============================================================================
// TUTORIAL HANDLERS
// =============================================================================

// handleShowTutorial handles the ShowTutorialMsg and displays the tutorial overlay.
func (m Model) handleShowTutorial(msg ShowTutorialMsg) (tea.Model, tea.Cmd) {
	if m.tutorial != nil {
		m.tutorial.Show()
		m.tutorial.SetSize(m.width, m.height)
		// Set up tutorial completion callback - this needs access to the Program
		// so we'll handle it via message passing
	}
	return m, nil
}

// handleTutorialComplete handles tutorial completion or skip.
func (m Model) handleTutorialComplete(msg TutorialCompleteMsg) (tea.Model, tea.Cmd) {
	// Save tutorial progress to config
	cfg := config.Global()
	if cfg != nil {
		if msg.Completed {
			cfg.UI.TutorialCompleted = true
			cfg.UI.TutorialStep = msg.CurrentStep
		} else {
			// Skipped - save current step but don't mark as completed
			cfg.UI.TutorialStep = msg.CurrentStep
		}
		// Save config to disk
		if err := config.Save(cfg); err != nil {
			// Log error but don't fail
			m.conversation.AddSystemMessage("Note: Could not save tutorial progress")
		}
	}
	return m, nil
}

// GetTutorial returns the tutorial overlay (for rendering).
func (m *Model) GetTutorial() *components.TutorialOverlay {
	return m.tutorial
}

// IsTutorialVisible returns true if the tutorial is visible.
func (m *Model) IsTutorialVisible() bool {
	return m.tutorial != nil && m.tutorial.IsVisible()
}

// =============================================================================
// BACKGROUND TASK HANDLERS
// =============================================================================

// handleTaskCreate handles task creation requests.
func (m Model) handleTaskCreate(msg TaskCreateMsg) (tea.Model, tea.Cmd) {
	if m.taskQueue == nil {
		m.conversation.AddSystemMessage("Error: Task system not initialized")
		m.updateViewport()
		return m, nil
	}

	// Create new task
	task := tasks.NewTask(msg.Description, msg.Command, msg.Args)
	task.ConversationID = m.conversation.ID

	// Add to queue
	m.taskQueue.Add(task)

	// Notify user
	shortID := task.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	m.conversation.AddSystemMessage(fmt.Sprintf("Task queued: %s [%s]\nUse /tasks to view all tasks", msg.Description, shortID))
	m.updateViewport()

	// Start listening for notifications in background
	return m, m.listenForNotifications()
}

// handleTaskList shows the task list.
func (m Model) handleTaskList(msg TaskListMsg) (tea.Model, tea.Cmd) {
	if m.taskQueue == nil {
		m.conversation.AddSystemMessage("Error: Task system not initialized")
		m.updateViewport()
		return m, nil
	}

	// Create task list component
	taskList := components.NewTaskList(m.taskQueue, m.theme)
	taskList.SetSize(m.width-4, m.height-10)

	// Configure filter
	switch msg.Filter {
	case "all":
		taskList.SetShowCompleted(true)
		taskList.SetShowFailed(true)
		taskList.SetShowCanceled(true)
	case "running":
		taskList.SetShowCompleted(false)
		taskList.SetShowFailed(false)
		taskList.SetShowCanceled(false)
	case "completed":
		taskList.SetShowCompleted(true)
		taskList.SetShowFailed(true)
		taskList.SetShowCanceled(true)
	}

	// Render task list
	listView := taskList.View()
	m.conversation.AddSystemMessage(listView)
	m.updateViewport()

	return m, nil
}

// handleTaskCancel handles task cancellation requests.
func (m Model) handleTaskCancel(msg TaskCancelMsg) (tea.Model, tea.Cmd) {
	if m.taskQueue == nil {
		m.conversation.AddSystemMessage("Error: Task system not initialized")
		m.updateViewport()
		return m, nil
	}

	// Cancel the task
	if m.taskQueue.Cancel(msg.TaskID) {
		m.conversation.AddSystemMessage(fmt.Sprintf("Task canceled: %s", msg.TaskID))
	} else {
		m.conversation.AddSystemMessage(fmt.Sprintf("Error: Could not cancel task %s (not found or already complete)", msg.TaskID))
	}
	m.updateViewport()

	return m, nil
}

// handleTaskNotification handles task completion notifications.
func (m Model) handleTaskNotification(msg TaskNotificationMsg) (tea.Model, tea.Cmd) {
	// Format notification message
	var notifMsg string
	shortID := msg.TaskID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	switch msg.Status {
	case "Complete":
		notifMsg = fmt.Sprintf("[OK] Task complete: %s [%s] (%.1fs)", msg.Description, shortID, msg.Duration.Seconds())
	case "Failed":
		notifMsg = fmt.Sprintf("[FAIL] Task failed: %s [%s] - %s", msg.Description, shortID, msg.Error)
	case "Canceled":
		notifMsg = fmt.Sprintf("[--] Task canceled: %s [%s]", msg.Description, shortID)
	default:
		notifMsg = fmt.Sprintf("Task %s: %s [%s]", msg.Status, msg.Description, shortID)
	}

	// Add notification to conversation
	m.conversation.AddSystemMessage(notifMsg)
	m.updateViewport()

	// Continue listening for more notifications
	return m, m.listenForNotifications()
}

// listenForNotifications creates a command that listens for task notifications.
func (m *Model) listenForNotifications() tea.Cmd {
	if m.taskQueue == nil {
		return nil
	}

	return func() tea.Msg {
		// Non-blocking check for notifications
		select {
		case notif := <-m.taskQueue.Notifications():
			return TaskNotificationMsg{
				TaskID:      notif.TaskID,
				Description: notif.Description,
				Status:      notif.Status.String(),
				Duration:    notif.Duration,
				Error:       notif.Error,
			}
		case <-time.After(100 * time.Millisecond):
			// No notification, return nil
			return nil
		}
	}
}

// GetTaskQueue returns the task queue for external access.
func (m *Model) GetTaskQueue() *tasks.Queue {
	return m.taskQueue
}

// =============================================================================
// VIM MODE HANDLERS
// =============================================================================

// handleVimCommand handles vim command execution messages.
func (m Model) handleVimCommand(msg VimCommandMsg) (tea.Model, tea.Cmd) {
	switch msg.Command {
	case "save":
		// Save conversation - delegate to export command
		return m, func() tea.Msg {
			return commands.ExportConversationMsg{
				Format: "json",
			}
		}
	case "wq":
		// Save and quit
		return m, tea.Sequence(
			func() tea.Msg {
				return commands.ExportConversationMsg{
					Format: "json",
				}
			},
			tea.Quit,
		)
	case "help":
		// Show help overlay
		m.showHelp = true
		return m, nil
	case "set-vim":
		// Toggle vim mode
		if enabled, ok := msg.Value.(bool); ok {
			if m.vimHandler != nil {
				m.vimHandler.SetEnabled(enabled)
				if enabled {
					m.conversation.AddSystemMessage("Vim mode enabled")
				} else {
					m.conversation.AddSystemMessage("Vim mode disabled")
				}
				m.updateViewport()
				// Update config
				if cfg := config.Global(); cfg != nil {
					cfg.UI.VimMode = enabled
					if err := config.Save(cfg); err == nil {
						// Saved successfully
					}
				}
			}
		}
		return m, nil
	}
	return m, nil
}

// GetVimHandler returns the vim handler (for rendering mode indicator)
func (m *Model) GetVimHandler() *VimHandler {
	return m.vimHandler
}

// =============================================================================
// NON-BLOCKING ERROR TOAST HANDLERS (lazygit-inspired)
// =============================================================================

// handleToastTick processes toast tick messages - removes expired toasts.
func (m Model) handleToastTick(msg components.ToastTickMsg) (tea.Model, tea.Cmd) {
	if m.toastManager == nil {
		return m, nil
	}

	// Tick the toast manager to remove expired toasts
	toasts := m.toastManager.TickToasts()

	// If there are still toasts, continue ticking
	if len(toasts) > 0 {
		return m, components.ToastTickCmd()
	}

	return m, nil
}

// handleToastDismiss handles manual toast dismissal (e.g., user pressed 'x').
func (m Model) handleToastDismiss(msg components.ToastDismissMsg) (tea.Model, tea.Cmd) {
	if m.toastManager == nil {
		return m, nil
	}

	m.toastManager.RemoveToast(msg.ID)
	return m, nil
}

// handleToastAdd handles adding a new toast via message.
func (m Model) handleToastAdd(msg components.ToastAddMsg) (tea.Model, tea.Cmd) {
	if m.toastManager == nil {
		return m, nil
	}

	switch msg.Kind {
	case components.ToastKindError:
		m.toastManager.AddError(msg.Message)
	case components.ToastKindWarning:
		m.toastManager.AddWarning(msg.Message)
	case components.ToastKindSuccess:
		m.toastManager.AddSuccess(msg.Message)
	default:
		m.toastManager.AddStatus(msg.Message)
	}

	return m, components.ToastTickCmd()
}

// AddErrorToast adds an error toast (non-blocking).
// Use this instead of setting StateError for recoverable errors.
func (m *Model) AddErrorToast(message string) tea.Cmd {
	if m.toastManager == nil {
		return nil
	}
	m.toastManager.AddError(message)
	return components.ToastTickCmd()
}

// AddWarningToast adds a warning toast (non-blocking).
func (m *Model) AddWarningToast(message string) tea.Cmd {
	if m.toastManager == nil {
		return nil
	}
	m.toastManager.AddWarning(message)
	return components.ToastTickCmd()
}

// AddStatusToast adds a status/info toast (non-blocking).
func (m *Model) AddStatusToast(message string) tea.Cmd {
	if m.toastManager == nil {
		return nil
	}
	m.toastManager.AddStatus(message)
	return components.ToastTickCmd()
}

// AddSuccessToast adds a success toast (non-blocking).
func (m *Model) AddSuccessToast(message string) tea.Cmd {
	if m.toastManager == nil {
		return nil
	}
	m.toastManager.AddSuccess(message)
	return components.ToastTickCmd()
}

// DismissToast dismisses a toast by ID.
func (m *Model) DismissToast(id int) {
	if m.toastManager != nil {
		m.toastManager.RemoveToast(id)
	}
}

// GetToasts returns the current toasts (for rendering).
func (m *Model) GetToasts() []components.ErrorToast {
	if m.toastManager == nil {
		return nil
	}
	return m.toastManager.GetToasts()
}

// HasToasts returns true if there are any active toasts.
func (m *Model) HasToasts() bool {
	if m.toastManager == nil {
		return false
	}
	return m.toastManager.HasToasts()
}

// ClearToasts removes all toasts.
func (m *Model) ClearToasts() {
	if m.toastManager != nil {
		m.toastManager.Clear()
	}
}
