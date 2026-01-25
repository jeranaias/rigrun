// rigrun TUI - A beautiful terminal interface for local LLM chat.
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeranaias/rigrun-tui/internal/cache"
	"github.com/jeranaias/rigrun-tui/internal/cli"
	"github.com/jeranaias/rigrun-tui/internal/cloud"
	"github.com/jeranaias/rigrun-tui/internal/commands"
	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/detect"
	"github.com/jeranaias/rigrun-tui/internal/model"
	"github.com/jeranaias/rigrun-tui/internal/offline"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/session"
	"github.com/jeranaias/rigrun-tui/internal/storage"
	"github.com/jeranaias/rigrun-tui/internal/tools"
	"github.com/jeranaias/rigrun-tui/internal/ui/chat"
	"github.com/jeranaias/rigrun-tui/internal/ui/components"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// Version information (set at build time)
var (
	Version   = "0.3.0-wired"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// Global program reference for async streaming
var (
	programRef *tea.Program
	programMu  sync.Mutex
)

func init() {
	// Sync version info with cli package
	cli.Version = Version
	cli.GitCommit = GitCommit
	cli.BuildDate = BuildDate
}

func main() {
	// Parse CLI arguments
	cmd, args := cli.Parse()

	// Route to appropriate handler
	switch cmd {
	case cli.CmdTUI:
		runTUI(args)
	case cli.CmdAsk:
		cli.HandleAsk(args)
	case cli.CmdChat:
		cli.HandleChat(args)
	case cli.CmdStatus:
		cli.HandleStatus(args)
	case cli.CmdConfig:
		cli.HandleConfig(args)
	case cli.CmdSetup:
		cli.HandleSetup(args)
	case cli.CmdCache:
		cli.HandleCache(args)
	case cli.CmdSession:
		handleSession(args)
	case cli.CmdAudit:
		if err := cli.HandleAudit(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdVerify:
		if err := cli.HandleVerify(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdDoctor:
		cli.HandleDoctor(args)
	case cli.CmdClassify:
		if err := cli.HandleClassify(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdConsent:
		if err := cli.HandleConsent(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdTest:
		if err := cli.HandleTest(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdEncrypt:
		if err := cli.HandleEncrypt(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdCrypto:
		if err := cli.HandleCrypto(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdBackup:
		if err := cli.HandleBackup(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdIncident:
		if err := cli.HandleIncident(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdData:
		if err := cli.HandleData(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdConmon:
		if err := cli.HandleConmon(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdLockout:
		if err := cli.HandleLockout(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdAuth:
		if err := cli.HandleAuth(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdRBAC:
		if err := cli.HandleRBAC(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdBoundary:
		if err := cli.HandleBoundary(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdVuln:
		if err := cli.HandleVuln(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdTraining:
		if err := cli.HandleTraining(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdTransport:
		if err := cli.HandleTransport(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdSecTest:
		if err := cli.HandleSecTest(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdConfigMgmt:
		if err := cli.HandleConfigMgmt(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdMaintenance:
		if err := cli.HandleMaintenance(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdIntel:
		if err := cli.HandleIntel(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case cli.CmdVersion:
		cli.HandleVersionWithJSON(args)
	case cli.CmdHelp:
		cli.HandleHelp()
	default:
		runTUI(args) // Default to TUI
	}
}

// runTUI starts the TUI interface.
func runTUI(args cli.Args) {
	// Load configuration at startup
	cfg := config.Global()

	// ==========================================================================
	// SI-7: Startup Integrity Check
	// Verify binary and config integrity before starting (when enabled)
	// ==========================================================================
	if cfg.Security.IntegrityCheckOnStartup {
		im := security.GlobalIntegrityManager()
		allValid, results := im.PerformStartupCheck()
		if !allValid {
			fmt.Fprintf(os.Stderr, "[SI-7 INTEGRITY] Startup integrity check detected issues:\n")
			for _, r := range results {
				if !r.Valid && r.Error != "" && !strings.Contains(r.Error, "no checksum record") && !strings.Contains(r.Error, "no baseline") {
					fmt.Fprintf(os.Stderr, "  [FAIL] %s: %s\n", r.Path, r.Error)
				}
			}
			fmt.Fprintf(os.Stderr, "[SI-7 INTEGRITY] Continuing startup. Run 'rigrun verify' for details.\n")
		}
		// Log the startup check
		security.AuditLogEvent("STARTUP", "INTEGRITY_CHECK", map[string]string{
			"passed": fmt.Sprintf("%t", allValid),
		})
	}

	// ==========================================================================
	// SC-28: Cache Encryption Check
	// Warn if cache encryption is disabled (IL5 compliance requirement)
	// ==========================================================================
	if !cfg.Security.EncryptCache {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "*** SECURITY WARNING (IL5 SC-28 VIOLATION) ***\n")
		fmt.Fprintf(os.Stderr, "Cache encryption is DISABLED. Cached data will be stored in plaintext.\n")
		fmt.Fprintf(os.Stderr, "This violates IL5 compliance requirements for data-at-rest protection.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "To enable cache encryption:\n")
		fmt.Fprintf(os.Stderr, "  1. Run: rigrun encrypt init\n")
		fmt.Fprintf(os.Stderr, "  2. Run: rigrun encrypt cache\n")
		fmt.Fprintf(os.Stderr, "  3. Or set security.encrypt_cache = true in config\n")
		fmt.Fprintf(os.Stderr, "\n")

		// Log the security warning
		security.AuditLogEvent("STARTUP", "CACHE_ENCRYPTION_DISABLED", map[string]string{
			"compliance_impact": "IL5_SC-28_VIOLATION",
			"severity":          "HIGH",
		})
	}

	// ==========================================================================
	// IL5 SC-7: Offline Mode Setup
	// Block ALL network except localhost Ollama when --no-network or config set
	// ==========================================================================
	if args.NoNetwork || cfg.Routing.OfflineMode {
		offline.SetOfflineMode(true)
		fmt.Fprintf(os.Stderr, "[OFFLINE MODE] IL5 SC-7: Network restricted to localhost only\n")
	}

	// Validate Ollama URL in offline mode
	if err := offline.ValidateOllamaURL(cfg.Local.OllamaURL); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Initialize the theme
	theme := styles.NewTheme()

	// Create Ollama client with config values
	ollamaConfig := &ollama.ClientConfig{
		BaseURL:      cfg.Local.OllamaURL,
		DefaultModel: cfg.Local.OllamaModel,
	}
	ollamaClient := ollama.NewClientWithConfig(ollamaConfig)

	// Create the application model with config
	m := NewModelWithConfig(theme, ollamaClient, cfg)

	// Ensure cleanup of cache goroutine when TUI exits
	defer func() {
		if m.stopCleanup != nil {
			m.stopCleanup()
		}
	}()

	// Apply CLI args to model (CLI args override config)
	if args.Model != "" {
		m.modelName = args.Model
		if m.ollamaClient != nil {
			m.ollamaClient.SetModel(args.Model)
		}
		m.welcome.SetModelName(args.Model)
	}

	// Apply paranoid mode from CLI (overrides config)
	if args.Paranoid {
		m.mode = "local"
		m.welcome.SetMode("local")
	}

	// Create the Bubble Tea program
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Store program reference for async operations
	programMu.Lock()
	programRef = p
	programMu.Unlock()

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running rigrun: %v\n", err)
		os.Exit(1)
	}
}

// =============================================================================
// APPLICATION MODEL
// =============================================================================

// State represents the current application state.
type State int

const (
	StateConsent State = iota // Consent banner (IL5 AC-8) - shown BEFORE welcome
	StateWelcome              // Welcome screen
	StateChat                 // Chat view
	StateError                // Error display
)

// Model is the main Bubble Tea model for the application.
type Model struct {
	// State
	state State

	// Theme and styling
	theme *styles.Theme

	// Dimensions
	width  int
	height int

	// Chat model (embedded for chat functionality)
	chatModel chat.Model

	// Welcome component
	welcome components.Welcome

	// Error display
	errorDisplay components.ErrorDisplay

	// Classification banner (IL5 compliance - DoDI 5200.48)
	classificationBanner *components.ClassificationBanner

	// Ollama client (local inference)
	ollamaClient *ollama.Client

	// Cloud client (OpenRouter)
	cloudClient *cloud.OpenRouterClient

	// Cache manager for query caching
	cacheManager *cache.CacheManager
	stopCleanup  func() // Function to stop cache cleanup goroutine

	// Application configuration
	config *config.Config

	// Session management
	sessionMgr *session.Manager
	convStore  *storage.ConversationStore

	// Streaming state
	streamingMsgID string
	cancelStream   context.CancelFunc
	streamStats    *model.Statistics

	// Status display
	modelName   string
	mode        string
	gpu         string
	offlineMode bool // IL5 SC-7: True when --no-network flag is active

	// Tool system for agentic loop
	toolRegistry *tools.Registry
	toolExecutor *tools.Executor
	toolsEnabled bool

	// Agentic loop state - tracks pending tool calls during streaming
	pendingToolCalls []ollama.ToolCall
	agenticMessages  []ollama.Message // Accumulated messages for agentic loop

	// Agentic loop safety limits
	agenticIteration       int       // Current iteration count
	agenticMaxIterations   int       // Maximum iterations (default: 25)
	agenticConsecutiveErrs int       // Consecutive tool failures
	agenticLoopStartTime   time.Time // When the current agentic loop started
	agenticLoopTimeout     time.Duration // Total timeout for entire loop (default: 30 min)

	// Session timeout (IL5 AC-12 compliance)
	sessionTimeoutOverlay components.SessionTimeoutOverlay
	sessionTimeout        time.Duration // Configured timeout (15-30 min per DoD STIG)
	sessionWarningShown   bool          // Whether warning is currently displayed
	sessionLastActivity   time.Time     // Last user activity timestamp

	// Consent banner (IL5 AC-8 compliance - System Use Notification)
	consentBanner   components.ConsentBanner
	consentRequired bool // Whether consent is required at startup
}

// NewModel creates a new application model (uses default config).
func NewModel(theme *styles.Theme, ollamaClient *ollama.Client) *Model {
	return NewModelWithConfig(theme, ollamaClient, config.Global())
}

// NewModelWithConfig creates a new application model with explicit configuration.
func NewModelWithConfig(theme *styles.Theme, ollamaClient *ollama.Client, cfg *config.Config) *Model {
	chatModel := chat.NewWithClient(theme, ollamaClient)

	welcome := components.NewWelcome(theme)
	welcome.SetVersion(Version)

	// Use config for default model, with fallback
	modelName := cfg.Local.OllamaModel
	if modelName == "" {
		modelName = cfg.DefaultModel
	}
	if modelName == "" && ollamaClient != nil {
		modelName = ollamaClient.GetDefaultModel()
	}
	if modelName == "" {
		modelName = "qwen2.5-coder:14b" // Ultimate fallback
	}

	welcome.SetModelName(modelName)

	// Detect GPU instead of hardcoding
	gpuName := "No GPU detected"
	if gpuInfo, err := detect.DetectGPUCached(); err == nil && gpuInfo != nil && gpuInfo.Type != detect.GpuTypeCPU {
		gpuName = gpuInfo.Name
		if gpuInfo.VramGB > 0 {
			gpuName = fmt.Sprintf("%s (%dGB)", gpuInfo.Name, gpuInfo.VramGB)
		}
	}
	welcome.SetGPUName(gpuName)

	// Use routing mode from config
	mode := cfg.Routing.DefaultMode
	if mode == "" {
		mode = "local" // Safe default
	}
	// Handle paranoid mode override
	if cfg.Routing.ParanoidMode {
		mode = "local"
	}
	// IL5 SC-7: Force local mode when offline
	isOffline := offline.IsOfflineMode()
	if isOffline {
		mode = "local-only" // Display as "local-only" to indicate IL5 SC-7 compliance
	}
	welcome.SetMode(mode)
	welcome.SetOfflineMode(isOffline)

	// Initialize error display
	errorDisplay := components.NewErrorDisplay()

	// Initialize classification banner (IL5 compliance - DoDI 5200.48)
	// Read classification from config; banner is shown on ALL screens when enabled
	classificationBanner := components.NewClassificationBannerFromString(cfg.Security.Classification)

	// Initialize session manager with default config
	sessionMgr := session.NewManager(session.DefaultConfig())

	// Initialize conversation store (creates ~/.rigrun/conversations/ directory)
	convStore, err := storage.NewConversationStore()
	if err != nil {
		// Log error but continue - sessions won't persist but app will work
		fmt.Fprintf(os.Stderr, "Warning: Could not initialize session storage: %v\n", err)
	}

	// Initialize cache manager with exact and semantic caching
	cacheManager := cache.NewCacheManager(nil, nil)
	// Set the embedding function for semantic caching (using simple hash-based embedding for now)
	cacheManager.SetEmbeddingFunc(cache.SimpleHashEmbedding)
	// Start periodic cleanup of expired cache entries (every 10 minutes)
	stopCleanup := cacheManager.StartCleanup(10 * time.Minute)

	// Pass cache manager to chat model for cache-first routing
	chatModel.SetCacheManager(cacheManager)

	// Initialize OpenRouter cloud client if API key is configured
	// IL5 SC-7: Cloud client is DISABLED in offline mode
	var cloudClient *cloud.OpenRouterClient
	if cfg.Cloud.OpenRouterKey != "" && !cfg.Routing.ParanoidMode && !offline.IsOfflineMode() {
		cloudClient = cloud.NewOpenRouterClient(cfg.Cloud.OpenRouterKey)
		// Set default cloud model from config
		if cfg.Cloud.DefaultModel != "" {
			cloudClient.SetModel(cfg.Cloud.DefaultModel)
		}
		// Pass cloud client to chat model for cloud routing
		chatModel.SetCloudClient(cloudClient)
	}

	// Initialize tool system for agentic loop
	toolRegistry := tools.NewRegistry()
	toolExecutor := tools.NewExecutor(toolRegistry)
	// Auto-approve low-risk read-only tools
	toolExecutor.SetAutoApproveLevel(tools.PermissionAuto)

	// ==========================================================================
	// IL5 AC-12: Session Timeout Configuration
	// ==========================================================================
	// Default: 30 minutes, configurable via config.Security.SessionTimeoutSecs
	// Valid range: 15-30 minutes per DoD STIG
	sessionTimeoutDuration := components.DefaultSessionTimeout
	if cfg.Security.SessionTimeoutSecs > 0 {
		sessionTimeoutDuration = time.Duration(cfg.Security.SessionTimeoutSecs) * time.Second
	}
	sessionTimeoutDuration = components.ValidateSessionTimeout(sessionTimeoutDuration)

	// Initialize session timeout overlay
	sessionTimeoutOverlay := components.NewSessionTimeoutOverlay()

	// ==========================================================================
	// IL5 AC-8: Consent Banner Check
	// ==========================================================================
	// Check if consent is required and not yet accepted
	// If Required=true and Accepted=false, show consent screen
	// If Required=false, skip consent (non-DoD mode)
	consentBanner := components.NewConsentBanner()
	consentRequired, _ := components.CheckConsentStatus()

	// Determine initial state based on consent requirement
	initialState := StateWelcome
	if consentRequired {
		initialState = StateConsent
	}

	return &Model{
		state:                initialState,
		theme:                theme,
		chatModel:            chatModel,
		welcome:              welcome,
		errorDisplay:         errorDisplay,
		classificationBanner: classificationBanner,
		ollamaClient:         ollamaClient,
		cloudClient:          cloudClient,
		cacheManager:         cacheManager,
		stopCleanup:          stopCleanup,
		config:               cfg,
		sessionMgr:           sessionMgr,
		convStore:            convStore,
		modelName:            modelName,
		mode:                 mode,
		gpu:                  gpuName,
		offlineMode:          isOffline, // IL5 SC-7: Offline mode state
		toolRegistry:         toolRegistry,
		toolExecutor:         toolExecutor,
		toolsEnabled:         true, // Enabled - tools now have proper Ollama schema and model compatibility checking
		// Agentic loop safety defaults
		agenticMaxIterations: 25,               // Reasonable limit for complex tasks
		agenticLoopTimeout:   30 * time.Minute, // Total loop timeout
		// Session timeout (IL5 AC-12 compliance)
		sessionTimeoutOverlay: sessionTimeoutOverlay,
		sessionTimeout:        sessionTimeoutDuration,
		sessionLastActivity:   time.Now(),
		// Consent banner (IL5 AC-8 compliance)
		consentBanner:   consentBanner,
		consentRequired: consentRequired,
	}
}

// =============================================================================
// BUBBLE TEA INTERFACE
// =============================================================================

// Init initializes the model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.welcome.Init(),
		m.chatModel.Init(),
		m.checkOllama(),
		m.startSessionTimeoutTick(), // IL5 AC-12: Start session timeout monitoring
	)
}

// Update handles messages and updates the model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.theme.SetSize(msg.Width, msg.Height)
		m.welcome.SetSize(msg.Width, msg.Height)
		m.errorDisplay.SetSize(msg.Width, msg.Height)

		// Update session timeout overlay dimensions (IL5 AC-12)
		m.sessionTimeoutOverlay.SetSize(msg.Width, msg.Height)

		// Update consent banner dimensions (IL5 AC-8)
		m.consentBanner.SetSize(msg.Width, msg.Height)

		// Update classification banner width (IL5 DoDI 5200.48)
		if m.classificationBanner != nil {
			m.classificationBanner.SetWidth(msg.Width)
		}

		// Forward to chat model with adjusted height for banner
		// Banner takes 1 line when visible
		adjustedHeight := msg.Height
		if m.classificationBanner != nil && m.config.Security.BannerEnabled {
			adjustedHeight = msg.Height - m.classificationBanner.Height()
		}
		adjustedMsg := tea.WindowSizeMsg{
			Width:  msg.Width,
			Height: adjustedHeight,
		}
		newChatModel, cmd := m.chatModel.Update(adjustedMsg)
		m.chatModel = newChatModel.(chat.Model)
		cmds = append(cmds, cmd)

		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case OllamaCheckMsg:
		if msg.Error != nil {
			// Use the detailed error message from our improved error handling
			m.errorDisplay = components.ConnectionErrorWithDetails(msg.Error.Error())
			m.errorDisplay.SetSize(m.width, m.height)
			m.errorDisplay.Show()
		} else {
			// Connection successful - hide any error display
			m.errorDisplay.Hide()
		}
		return m, nil

	case chat.StreamRequestMsg:
		// Convert chat.StreamRequestMsg to local StreamRequestMsg, preserving cloud routing fields
		return m.startStreaming(StreamRequestMsg{
			MessageID:  msg.MessageID,
			Messages:   msg.Messages,
			UseCloud:   msg.UseCloud,
			CloudModel: msg.CloudModel,
			CloudTier:  msg.CloudTier,
		})

	case StreamRequestMsg:
		return m.startStreaming(msg)

	case StreamTokenMsg:
		return m.handleStreamToken(msg)

	case StreamCompleteMsg:
		return m.handleStreamComplete(msg)

	case StreamErrorMsg:
		return m.handleStreamError(msg)

	case RoutingFallbackMsg:
		return m.handleRoutingFallback(msg)

	case ToolCallsDetectedMsg:
		return m.handleToolCallsDetected(msg)

	case ToolExecutionCompleteMsg:
		return m.handleToolExecutionComplete(msg)

	// Session management messages from /save, /load, /list commands
	// Handle both commands package and chat package message types
	case commands.SaveConversationMsg:
		return m.handleSaveConversation(commands.SaveConversationMsg{Name: msg.Name})

	case chat.SaveConversationMsg:
		return m.handleSaveConversation(commands.SaveConversationMsg{Name: msg.Name})

	case commands.LoadConversationMsg:
		return m.handleLoadConversation(commands.LoadConversationMsg{ID: msg.ID})

	case chat.LoadConversationMsg:
		return m.handleLoadConversation(commands.LoadConversationMsg{ID: msg.ID})

	case commands.ListSessionsMsg:
		return m.handleListSessions()

	case chat.ListSessionsMsg:
		return m.handleListSessions()

	case commands.SaveCompleteMsg:
		return m.handleSaveComplete(msg)

	case commands.LoadCompleteMsg:
		return m.handleLoadComplete(msg)

	case SessionLoadedMsg:
		return m.handleSessionLoaded(msg)

	case SessionListMsg:
		return m.handleSessionList(msg)

	// Classification change message (IL5 DoDI 5200.48)
	// Allows /classify command to update the banner
	case components.ClassificationChangedMsg:
		m.SetClassification(msg.Classification)
		return m, nil

	// ==========================================================================
	// IL5 AC-12: Session Timeout Messages
	// ==========================================================================
	case components.SessionTimeoutTickMsg:
		return m.handleSessionTimeoutTick(msg)

	case components.SessionTimeoutWarningMsg:
		return m.handleSessionTimeoutWarning(msg)

	case components.SessionExpiredMsg:
		return m.handleSessionExpired()

	case components.SessionExtendedMsg:
		return m.handleSessionExtended()
	}

	// Forward messages to chat model when in chat state
	if m.state == StateChat {
		newChatModel, cmd := m.chatModel.Update(msg)
		m.chatModel = newChatModel.(chat.Model)
		cmds = append(cmds, cmd)
	}

	// Update error display if visible
	if m.errorDisplay.IsVisible() {
		newError, cmd := m.errorDisplay.Update(msg)
		m.errorDisplay = newError
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleKeyPress processes keyboard input.
func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ==========================================================================
	// IL5 AC-12: Reset session timeout on any user activity
	// ==========================================================================
	m.sessionLastActivity = time.Now()

	// Handle session timeout overlay first - any key extends the session
	if m.sessionTimeoutOverlay.IsVisible() && !m.sessionTimeoutOverlay.IsExpired() {
		m.sessionTimeoutOverlay.Hide()
		m.sessionWarningShown = false
		// Log session extended
		security.AuditLogEvent(m.sessionMgr.SessionID(), "SESSION_EXTENDED", map[string]string{
			"remaining": m.sessionTimeout.String(),
		})
		return m, nil
	}

	// Handle error state first
	if m.errorDisplay.IsVisible() {
		switch msg.String() {
		case "esc", "enter", "q":
			m.errorDisplay.Hide()
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
	}

	// ==========================================================================
	// IL5 AC-8: Handle consent state
	// ==========================================================================
	if m.state == StateConsent {
		switch msg.String() {
		case "ctrl+c", "esc":
			// Exit without acknowledging - user declined consent
			return m, tea.Quit
		case "enter", "y", "Y":
			// User acknowledged consent - record it and proceed
			if err := cli.RecordConsentAcceptance(); err != nil {
				// Log error but proceed anyway - don't block user
				fmt.Fprintf(os.Stderr, "Warning: Could not record consent: %v\n", err)
			}
			m.consentBanner.Acknowledge()
			m.consentRequired = false
			m.state = StateWelcome
			return m, nil
		case "up", "down", "k", "j", "pgup", "pgdown", "pgdn", "home", "end":
			// Allow scroll keys to pass through to consent banner
			var cmd tea.Cmd
			m.consentBanner, cmd = m.consentBanner.Update(msg)
			return m, cmd
		}
		// Ignore other keys in consent state - must explicitly acknowledge
		return m, nil
	}

	// Handle welcome state
	if m.state == StateWelcome {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			return m, tea.Quit
		default:
			// Any key transitions to chat
			m.state = StateChat
			// Forward the window size to chat model
			newChatModel, cmd := m.chatModel.Update(tea.WindowSizeMsg{
				Width:  m.width,
				Height: m.height,
			})
			m.chatModel = newChatModel.(chat.Model)
			return m, cmd
		}
	}

	// In chat state
	if m.state == StateChat {
		switch msg.String() {
		case "ctrl+c":
			// If streaming, cancel it
			if m.cancelStream != nil {
				m.cancelStream()
				m.cancelStream = nil
				return m, nil
			}
			// Otherwise quit
			return m, tea.Quit

		case "ctrl+l":
			// Clear screen - send message to chat model
			return m, func() tea.Msg { return chat.ClearConversationMsg{} }
		}

		// Forward to chat model
		newChatModel, cmd := m.chatModel.Update(msg)
		m.chatModel = newChatModel.(chat.Model)
		return m, cmd
	}

	return m, nil
}

// View renders the current state.
func (m *Model) View() string {
	// ==========================================================================
	// IL5 AC-12: Session timeout overlay takes precedence when visible
	// ==========================================================================
	if m.sessionTimeoutOverlay.IsVisible() {
		m.sessionTimeoutOverlay.SetSize(m.width, m.height)
		return m.sessionTimeoutOverlay.View()
	}

	// Show error overlay if visible (error overlay takes full screen, no banner)
	if m.errorDisplay.IsVisible() {
		return m.errorDisplay.View()
	}

	// ==========================================================================
	// IL5 AC-8: Consent banner takes full screen when displayed
	// ==========================================================================
	if m.state == StateConsent {
		// Consent banner is full-screen, no classification banner overlay
		m.consentBanner.SetSize(m.width, m.height)
		return m.consentBanner.View()
	}

	// Build the main content based on state
	var content string
	switch m.state {
	case StateWelcome:
		content = m.welcome.View()
	case StateChat:
		content = m.chatModel.View()
	default:
		content = m.welcome.View()
	}

	// Prepend classification banner if enabled (IL5 DoDI 5200.48 compliance)
	// The banner appears at the TOP of ALL screens when security.banner_enabled is true
	if m.config.Security.BannerEnabled && m.classificationBanner != nil {
		return m.classificationBanner.View() + "\n" + content
	}

	return content
}

// SetClassification updates the classification level on the banner.
// This method allows changing classification via commands (e.g., /classify).
func (m *Model) SetClassification(c security.Classification) {
	if m.classificationBanner != nil {
		m.classificationBanner.SetClassification(c)
	}
}

// =============================================================================
// OLLAMA INTEGRATION
// =============================================================================

// OllamaCheckMsg is sent when Ollama status is checked.
type OllamaCheckMsg struct {
	Running bool
	Error   error
}

// checkOllama returns a command that checks Ollama status and auto-starts if needed.
func (m *Model) checkOllama() tea.Cmd {
	// Capture the client BEFORE returning the closure to avoid race conditions
	client := m.ollamaClient

	return func() tea.Msg {
		if client == nil {
			return OllamaCheckMsg{Running: false, Error: ollama.ErrNotRunning}
		}

		// Use EnsureRunning which will auto-start Ollama if not running
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := client.EnsureRunning(ctx)
		return OllamaCheckMsg{
			Running: err == nil,
			Error:   err,
		}
	}
}

// =============================================================================
// STREAMING INTEGRATION
// =============================================================================

// StreamRequestMsg requests starting a stream.
type StreamRequestMsg struct {
	MessageID string
	Messages  []ollama.Message
	// Cloud routing fields
	UseCloud   bool   // If true, use cloud client instead of Ollama
	CloudModel string // Cloud model to use (e.g., "haiku", "sonnet", "opus")
	CloudTier  string // Tier string for display
}

// StreamTokenMsg delivers a token from the stream.
type StreamTokenMsg struct {
	MessageID string
	Token     string
	IsFirst   bool
}

// StreamCompleteMsg signals stream completion.
type StreamCompleteMsg struct {
	MessageID string
	Stats     *model.Statistics
}

// StreamErrorMsg signals a stream error.
type StreamErrorMsg struct {
	MessageID string
	Error     error
}

// RoutingFallbackMsg signals that routing fell back to a different tier.
// Used to update message metadata when cloud fails and falls back to local.
type RoutingFallbackMsg struct {
	MessageID string
	FromTier  string // Original tier that failed
	ToTier    string // Tier we fell back to
	Reason    string // Why the fallback happened
}

// ToolCallsDetectedMsg signals that tool calls were detected in the LLM response.
// This triggers the agentic loop: execute tools and call LLM again.
type ToolCallsDetectedMsg struct {
	MessageID      string
	ToolCalls      []ollama.ToolCall
	ResponseText   string           // Any text content before tool calls
	Messages       []ollama.Message // Current conversation for continuation
	Stats          *model.Statistics
}

// ToolExecutionCompleteMsg signals that tool execution has finished.
// The results are added to the conversation and LLM is called again.
type ToolExecutionCompleteMsg struct {
	MessageID    string
	ToolResults  []ToolResultEntry
	Messages     []ollama.Message // Updated messages including tool results
}

// ToolResultEntry holds a single tool execution result.
type ToolResultEntry struct {
	ToolName string
	Result   string
	Success  bool
}

// startStreaming begins a streaming request, routing to cloud or local as requested.
func (m *Model) startStreaming(msg StreamRequestMsg) (tea.Model, tea.Cmd) {
	m.streamingMsgID = msg.MessageID
	m.streamStats = model.NewStatistics()

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelStream = cancel

	// Notify chat model that streaming has started
	startMsg := chat.StreamStartMsg{
		MessageID: msg.MessageID,
		StartTime: time.Now(),
	}
	newChatModel, _ := m.chatModel.Update(startMsg)
	m.chatModel = newChatModel.(chat.Model)

	// Route to cloud or local based on request
	if msg.UseCloud && m.cloudClient != nil && m.cloudClient.IsConfigured() {
		return m, m.startCloudStreaming(ctx, msg)
	}
	return m, m.startLocalStreaming(ctx, msg)
}

// startLocalStreaming performs streaming via local Ollama client with tool support.
func (m *Model) startLocalStreaming(ctx context.Context, msg StreamRequestMsg) tea.Cmd {
	// Capture model fields before returning closure to avoid race conditions
	ollamaClient := m.ollamaClient
	toolsEnabled := m.toolsEnabled
	toolRegistry := m.toolRegistry
	modelName := m.modelName
	cancelStream := m.cancelStream

	return func() tea.Msg {
		if ollamaClient == nil {
			if cancelStream != nil {
				cancelStream()
			}
			return StreamErrorMsg{
				MessageID: msg.MessageID,
				Error:     ollama.ErrNotRunning,
			}
		}

		isFirst := true
		var accumulatedContent string
		var detectedToolCalls []ollama.ToolCall

		// Get tool definitions if tools are enabled
		var ollamaTools []ollama.Tool
		if toolsEnabled && toolRegistry != nil {
			ollamaTools = toolRegistry.ToOllamaTools()
		}

		// Use ChatStreamWithTools if tools are available, otherwise regular ChatStream
		var streamErr error
		if len(ollamaTools) > 0 {
			streamErr = ollamaClient.ChatStreamWithTools(ctx, modelName, msg.Messages, ollamaTools, func(chunk ollama.StreamChunk) {
				if chunk.Error != nil {
					programMu.Lock()
					p := programRef
					programMu.Unlock()
					if p != nil {
						p.Send(StreamErrorMsg{
							MessageID: msg.MessageID,
							Error:     chunk.Error,
						})
					}
					return
				}

				// Send token message for text content
				if chunk.Content != "" {
					accumulatedContent += chunk.Content
					programMu.Lock()
					p := programRef
					programMu.Unlock()
					if p != nil {
						p.Send(StreamTokenMsg{
							MessageID: msg.MessageID,
							Token:     chunk.Content,
							IsFirst:   isFirst,
						})
					}
					isFirst = false
				}

				// Capture tool calls from the response
				if len(chunk.ToolCalls) > 0 {
					detectedToolCalls = append(detectedToolCalls, chunk.ToolCalls...)
				}

				// Handle completion
				if chunk.Done {
					stats := model.NewStatistics()
					stats.Finalize(chunk.CompletionTokens)

					programMu.Lock()
					p := programRef
					programMu.Unlock()
					// Check if we have tool calls - if so, trigger agentic loop
					if len(detectedToolCalls) > 0 {
						if p != nil {
							p.Send(ToolCallsDetectedMsg{
								MessageID:    msg.MessageID,
								ToolCalls:    detectedToolCalls,
								ResponseText: accumulatedContent,
								Messages:     msg.Messages,
								Stats:        stats,
							})
						}
					} else {
						// No tool calls - regular completion
						if p != nil {
							p.Send(StreamCompleteMsg{
								MessageID: msg.MessageID,
								Stats:     stats,
							})
						}
					}
				}
			})
		} else {
			// No tools - use regular streaming
			streamErr = ollamaClient.ChatStream(ctx, modelName, msg.Messages, func(chunk ollama.StreamChunk) {
				if chunk.Error != nil {
					programMu.Lock()
					p := programRef
					programMu.Unlock()
					if p != nil {
						p.Send(StreamErrorMsg{
							MessageID: msg.MessageID,
							Error:     chunk.Error,
						})
					}
					return
				}

				// Send token message
				if chunk.Content != "" {
					programMu.Lock()
					p := programRef
					programMu.Unlock()
					if p != nil {
						p.Send(StreamTokenMsg{
							MessageID: msg.MessageID,
							Token:     chunk.Content,
							IsFirst:   isFirst,
						})
					}
					isFirst = false
				}

				// Handle completion
				if chunk.Done {
					stats := model.NewStatistics()
					stats.Finalize(chunk.CompletionTokens)
					programMu.Lock()
					p := programRef
					programMu.Unlock()
					if p != nil {
						p.Send(StreamCompleteMsg{
							MessageID: msg.MessageID,
							Stats:     stats,
						})
					}
				}
			})
		}

		if streamErr != nil && streamErr != context.Canceled {
			return StreamErrorMsg{
				MessageID: msg.MessageID,
				Error:     streamErr,
			}
		}

		return nil
	}
}

// startCloudStreaming performs streaming via OpenRouter cloud client.
func (m *Model) startCloudStreaming(ctx context.Context, msg StreamRequestMsg) tea.Cmd {
	// Capture model fields before returning closure to avoid race conditions
	cloudClient := m.cloudClient
	ollamaClient := m.ollamaClient
	modelName := m.modelName
	cancelStream := m.cancelStream

	// Set the model for this request BEFORE returning closure to avoid race condition
	// (cloudClient is a shared object, so SetModel must run on main thread)
	if msg.CloudModel != "" && cloudClient != nil {
		cloudClient.SetModel(msg.CloudModel)
	}

	return func() tea.Msg {
		if cloudClient == nil || !cloudClient.IsConfigured() {
			// Fallback to local if cloud not configured
			return fallbackToLocalWithCaptures(ctx, msg, ollamaClient, modelName, cancelStream)
		}

		// Convert ollama.Message to cloud.ChatMessage
		cloudMessages := make([]cloud.ChatMessage, 0, len(msg.Messages))
		for _, ollamaMsg := range msg.Messages {
			cloudMessages = append(cloudMessages, cloud.ChatMessage{
				Role:    ollamaMsg.Role,
				Content: ollamaMsg.Content,
			})
		}

		// Use streaming API for OpenRouter
		isFirst := true
		var accumulatedContent string
		var tokenCount int

		streamErr := cloudClient.ChatStream(ctx, cloudMessages, func(chunk cloud.StreamChunk) {
			content := chunk.GetContent()
			if content != "" {
				accumulatedContent += content
				tokenCount++

				programMu.Lock()
				p := programRef
				programMu.Unlock()
				if p != nil {
					p.Send(StreamTokenMsg{
						MessageID: msg.MessageID,
						Token:     content,
						IsFirst:   isFirst,
					})
				}
				isFirst = false
			}

			// Handle completion (finish_reason is set)
			if chunk.IsDone() {
				// Create stats
				stats := model.NewStatistics()
				stats.RecordFirstToken()
				stats.Finalize(tokenCount)

				// Session stats are recorded in handleStreamComplete (chat/model.go)
				// to avoid double-counting

				programMu.Lock()
				p := programRef
				programMu.Unlock()
				if p != nil {
					p.Send(StreamCompleteMsg{
						MessageID: msg.MessageID,
						Stats:     stats,
					})
				}
			}
		})

		if streamErr != nil && streamErr != context.Canceled {
			// On cloud failure, attempt fallback to local
			// Notify via the message system that we're falling back
			programMu.Lock()
			p := programRef
			programMu.Unlock()
			if p != nil {
				p.Send(RoutingFallbackMsg{
					MessageID: msg.MessageID,
					FromTier:  msg.CloudTier,
					ToTier:    "Local",
					Reason:    streamErr.Error(),
				})
			}
			return fallbackToLocalWithCaptures(ctx, msg, ollamaClient, modelName, cancelStream)
		}

		return nil
	}
}

// fallbackToLocal attempts to handle a request locally after cloud failure.
// This method captures model fields at call time for safe use in non-goroutine contexts.
func (m *Model) fallbackToLocal(ctx context.Context, msg StreamRequestMsg) tea.Msg {
	return fallbackToLocalWithCaptures(ctx, msg, m.ollamaClient, m.modelName, m.cancelStream)
}

// fallbackToLocalWithCaptures performs fallback with pre-captured values for goroutine safety.
func fallbackToLocalWithCaptures(ctx context.Context, msg StreamRequestMsg, ollamaClient *ollama.Client, modelName string, cancelStream context.CancelFunc) tea.Msg {
	if ollamaClient == nil {
		if cancelStream != nil {
			cancelStream()
		}
		return StreamErrorMsg{
			MessageID: msg.MessageID,
			Error:     fmt.Errorf("cloud request failed and local Ollama not available"),
		}
	}

	isFirst := true
	err := ollamaClient.ChatStream(ctx, modelName, msg.Messages, func(chunk ollama.StreamChunk) {
		if chunk.Error != nil {
			programMu.Lock()
			p := programRef
			programMu.Unlock()
			if p != nil {
				p.Send(StreamErrorMsg{
					MessageID: msg.MessageID,
					Error:     chunk.Error,
				})
			}
			return
		}

		if chunk.Content != "" {
			programMu.Lock()
			p := programRef
			programMu.Unlock()
			if p != nil {
				p.Send(StreamTokenMsg{
					MessageID: msg.MessageID,
					Token:     chunk.Content,
					IsFirst:   isFirst,
				})
			}
			isFirst = false
		}

		if chunk.Done {
			stats := model.NewStatistics()
			stats.Finalize(chunk.CompletionTokens)
			programMu.Lock()
			p := programRef
			programMu.Unlock()
			if p != nil {
				p.Send(StreamCompleteMsg{
					MessageID: msg.MessageID,
					Stats:     stats,
				})
			}
		}
	})

	if err != nil && err != context.Canceled {
		return StreamErrorMsg{
			MessageID: msg.MessageID,
			Error:     err,
		}
	}

	return nil
}

// cloudModelToTier converts a cloud model name to a router tier.
func (m *Model) cloudModelToTier(cloudModel string) router.Tier {
	switch cloudModel {
	case "haiku":
		return router.TierHaiku
	case "sonnet":
		return router.TierSonnet
	case "opus":
		return router.TierOpus
	case "gpt4o":
		return router.TierGpt4o
	case "auto":
		return router.TierCloud
	default:
		return router.TierCloud
	}
}

// handleStreamToken processes a stream token.
func (m *Model) handleStreamToken(msg StreamTokenMsg) (tea.Model, tea.Cmd) {
	if msg.MessageID != m.streamingMsgID {
		return m, nil
	}

	// Record first token timing
	if msg.IsFirst && m.streamStats != nil {
		m.streamStats.RecordFirstToken()
	}

	// Forward to chat model
	chatMsg := chat.StreamTokenMsg{
		MessageID: msg.MessageID,
		Token:     msg.Token,
		IsFirst:   msg.IsFirst,
	}
	newChatModel, cmd := m.chatModel.Update(chatMsg)
	m.chatModel = newChatModel.(chat.Model)

	return m, cmd
}

// handleStreamComplete processes stream completion.
func (m *Model) handleStreamComplete(msg StreamCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.MessageID != m.streamingMsgID {
		return m, nil
	}

	// Clean up streaming state
	m.streamingMsgID = ""
	m.cancelStream = nil
	m.streamStats = nil

	// Reset agentic loop state - stream completion means the loop is done
	// (either naturally finished or was stopped by safety checks)
	m.resetAgenticState()

	// Forward to chat model
	chatMsg := chat.StreamCompleteMsg{
		MessageID: msg.MessageID,
		Stats:     msg.Stats,
	}
	newChatModel, cmd := m.chatModel.Update(chatMsg)
	m.chatModel = newChatModel.(chat.Model)

	return m, cmd
}

// handleStreamError processes a stream error.
func (m *Model) handleStreamError(msg StreamErrorMsg) (tea.Model, tea.Cmd) {
	// Clean up streaming state
	m.streamingMsgID = ""
	m.cancelStream = nil
	m.streamStats = nil

	// Reset agentic loop state on error
	m.resetAgenticState()

	// Show error
	if ollama.IsNotRunning(msg.Error) {
		m.errorDisplay = components.ConnectionError()
	} else if ollama.IsModelNotFound(msg.Error) {
		m.errorDisplay = components.ModelNotFoundError(m.modelName)
	} else if ollama.IsTimeout(msg.Error) {
		m.errorDisplay = components.TimeoutError()
	} else {
		m.errorDisplay = components.NewError("Streaming Error", msg.Error.Error())
	}
	m.errorDisplay.SetSize(m.width, m.height)
	m.errorDisplay.Show()

	// Forward to chat model
	chatMsg := chat.StreamErrorMsg{
		MessageID: msg.MessageID,
		Error:     msg.Error,
	}
	newChatModel, cmd := m.chatModel.Update(chatMsg)
	m.chatModel = newChatModel.(chat.Model)

	return m, cmd
}

// handleRoutingFallback processes a routing fallback notification.
// Updates the message's routing tier to reflect that we fell back to local.
func (m *Model) handleRoutingFallback(msg RoutingFallbackMsg) (tea.Model, tea.Cmd) {
	// Forward to chat model to update the message metadata
	chatMsg := chat.RoutingFallbackMsg{
		MessageID: msg.MessageID,
		FromTier:  msg.FromTier,
		ToTier:    msg.ToTier,
		Reason:    msg.Reason,
	}
	newChatModel, cmd := m.chatModel.Update(chatMsg)
	m.chatModel = newChatModel.(chat.Model)

	return m, cmd
}

// =============================================================================
// AGENTIC TOOL LOOP
// =============================================================================

// handleToolCallsDetected processes detected tool calls from the LLM.
// This is the entry point of the agentic loop.
func (m *Model) handleToolCallsDetected(msg ToolCallsDetectedMsg) (tea.Model, tea.Cmd) {
	if msg.MessageID != m.streamingMsgID {
		return m, nil
	}

	// Initialize agentic loop tracking on first iteration
	if m.agenticIteration == 0 {
		m.agenticIteration = 1
		m.agenticConsecutiveErrs = 0
		m.agenticLoopStartTime = time.Now()
	} else {
		m.agenticIteration++
	}

	// SAFETY CHECK: Maximum iterations limit
	if m.agenticIteration > m.agenticMaxIterations {
		m.chatModel.GetConversation().AddSystemMessage(
			fmt.Sprintf("Agentic loop stopped: maximum iterations (%d) reached. The task may be incomplete.", m.agenticMaxIterations))
		m.resetAgenticState()
		return m, func() tea.Msg {
			return StreamCompleteMsg{MessageID: msg.MessageID, Stats: msg.Stats}
		}
	}

	// SAFETY CHECK: Total loop timeout
	if time.Since(m.agenticLoopStartTime) > m.agenticLoopTimeout {
		m.chatModel.GetConversation().AddSystemMessage(
			fmt.Sprintf("Agentic loop stopped: total timeout (%v) exceeded. The task may be incomplete.", m.agenticLoopTimeout))
		m.resetAgenticState()
		return m, func() tea.Msg {
			return StreamCompleteMsg{MessageID: msg.MessageID, Stats: msg.Stats}
		}
	}

	// Store the pending tool calls
	m.pendingToolCalls = msg.ToolCalls
	m.agenticMessages = msg.Messages

	// Add a system message to the chat showing tool calls are being executed
	// This provides visual feedback to the user
	for _, tc := range msg.ToolCalls {
		toolName := tc.Function.Name
		m.chatModel.GetConversation().AddSystemMessage(
			fmt.Sprintf("Executing tool: %s (iteration %d/%d)", toolName, m.agenticIteration, m.agenticMaxIterations))
	}

	// Create a cancellable context for tool execution
	// This allows the user to cancel tool execution with Ctrl+C
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelStream = cancel

	// Execute tools asynchronously
	return m, m.executeToolsAsync(ctx, msg.MessageID, msg.ToolCalls, msg.Messages, msg.ResponseText)
}

// resetAgenticState resets the agentic loop state after completion or error.
func (m *Model) resetAgenticState() {
	m.agenticIteration = 0
	m.agenticConsecutiveErrs = 0
	m.agenticLoopStartTime = time.Time{}
	m.pendingToolCalls = nil
	m.agenticMessages = nil
}

// executeToolsAsync runs tool execution in the background.
// The parentCtx allows cancellation from outside (e.g., user pressing Ctrl+C).
func (m *Model) executeToolsAsync(parentCtx context.Context, messageID string, toolCalls []ollama.ToolCall, messages []ollama.Message, assistantText string) tea.Cmd {
	// Capture toolExecutor before closure to avoid race conditions
	toolExecutor := m.toolExecutor

	return func() tea.Msg {
		if toolExecutor == nil {
			return StreamErrorMsg{
				MessageID: messageID,
				Error:     fmt.Errorf("tool executor not initialized"),
			}
		}

		// Execute each tool call
		var results []ToolResultEntry
		for _, tc := range toolCalls {
			// Check if context was cancelled before starting next tool
			if parentCtx.Err() != nil {
				return StreamErrorMsg{
					MessageID: messageID,
					Error:     fmt.Errorf("tool execution cancelled"),
				}
			}

			// Convert to our internal tool call format
			call := tools.ToolCall{
				Name:   tc.Function.Name,
				Params: tc.Function.Arguments,
			}

			// Execute with a timeout context derived from the parent context
			// This ensures cancellation propagates from user's Ctrl+C
			ctx, cancel := context.WithTimeout(parentCtx, 2*time.Minute)
			result := toolExecutor.Execute(ctx, call)
			cancel()

			// Collect result
			output := result.Output
			if !result.Success {
				output = result.Error
			}

			results = append(results, ToolResultEntry{
				ToolName: tc.Function.Name,
				Result:   output,
				Success:  result.Success,
			})
		}

		// Build updated messages with assistant response and tool results
		updatedMessages := make([]ollama.Message, len(messages))
		copy(updatedMessages, messages)

		// Add the assistant message with tool calls
		updatedMessages = append(updatedMessages, ollama.NewAssistantMessageWithTools(assistantText, toolCalls))

		// Add tool result messages
		for _, r := range results {
			// Format the tool result
			resultContent := r.Result
			if r.Result == "" {
				if r.Success {
					resultContent = "(success, no output)"
				} else {
					resultContent = "(failed, no error message)"
				}
			}
			updatedMessages = append(updatedMessages, ollama.NewToolResultMessage(resultContent))
		}

		return ToolExecutionCompleteMsg{
			MessageID:   messageID,
			ToolResults: results,
			Messages:    updatedMessages,
		}
	}
}

// handleToolExecutionComplete processes completed tool executions and continues the agentic loop.
func (m *Model) handleToolExecutionComplete(msg ToolExecutionCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.MessageID != m.streamingMsgID {
		return m, nil
	}

	// Track consecutive errors - if all tools failed, increment counter
	allFailed := true
	for _, r := range msg.ToolResults {
		if r.Success {
			allFailed = false
			break
		}
	}

	if allFailed && len(msg.ToolResults) > 0 {
		m.agenticConsecutiveErrs++
	} else {
		m.agenticConsecutiveErrs = 0
	}

	// SAFETY CHECK: Too many consecutive failures indicates the LLM is stuck
	const maxConsecutiveErrors = 3
	if m.agenticConsecutiveErrs >= maxConsecutiveErrors {
		m.chatModel.GetConversation().AddSystemMessage(
			fmt.Sprintf("Agentic loop stopped: %d consecutive tool failures. The LLM may be stuck in an error loop.", maxConsecutiveErrors))
		m.resetAgenticState()
		// Create a completion message to finalize the stream
		return m, func() tea.Msg {
			return StreamCompleteMsg{MessageID: msg.MessageID}
		}
	}

	// SAFETY CHECK: Verify we haven't exceeded timeout during tool execution
	if !m.agenticLoopStartTime.IsZero() && time.Since(m.agenticLoopStartTime) > m.agenticLoopTimeout {
		m.chatModel.GetConversation().AddSystemMessage(
			fmt.Sprintf("Agentic loop stopped: total timeout (%v) exceeded during tool execution.", m.agenticLoopTimeout))
		m.resetAgenticState()
		return m, func() tea.Msg {
			return StreamCompleteMsg{MessageID: msg.MessageID}
		}
	}

	// Add tool results to the conversation for display
	for _, r := range msg.ToolResults {
		// Use the model package's NewToolMessage for proper display
		toolMsg := model.NewToolMessage(r.ToolName, r.Result, r.Success)
		m.chatModel.GetConversation().Messages = append(m.chatModel.GetConversation().Messages, toolMsg)
	}

	// Clear pending state
	m.pendingToolCalls = nil

	// Store messages for agentic continuation
	m.agenticMessages = msg.Messages

	// Add a new assistant message for the continuation response
	assistantMsg := m.chatModel.GetConversation().AddAssistantMessage()

	// Continue the conversation - call LLM again with updated messages including tool results
	// This creates a new StreamRequestMsg which will flow through startStreaming again
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelStream = cancel

	return m, m.continueAgenticLoop(ctx, assistantMsg.ID, msg.Messages)
}

// continueAgenticLoop sends updated messages back to the LLM after tool execution.
func (m *Model) continueAgenticLoop(ctx context.Context, messageID string, messages []ollama.Message) tea.Cmd {
	// Update the streaming message ID for the continuation
	m.streamingMsgID = messageID

	// Create a new stream request with updated messages
	return m.startLocalStreaming(ctx, StreamRequestMsg{
		MessageID: messageID,
		Messages:  messages,
		UseCloud:  false,
	})
}

// =============================================================================
// SESSION MANAGEMENT
// =============================================================================

// handleSaveConversation saves the current conversation to storage.
func (m *Model) handleSaveConversation(msg commands.SaveConversationMsg) (tea.Model, tea.Cmd) {
	if m.convStore == nil {
		// Show error - storage not available
		m.chatModel.GetConversation().AddSystemMessage("Error: Session storage not available")
		m.chatModel.SetConversation(m.chatModel.GetConversation())
		return m, nil
	}

	conv := m.chatModel.GetConversation()
	if conv == nil {
		return m, nil
	}
	if conv.IsEmpty() {
		conv.AddSystemMessage("Nothing to save - conversation is empty")
		m.chatModel.SetConversation(m.chatModel.GetConversation())
		return m, nil
	}

	// Capture fields before closure to avoid race conditions
	convStore := m.convStore
	sessionMgr := m.sessionMgr
	modelName := m.modelName
	sessionStats := m.chatModel.GetSessionStats()

	return m, func() tea.Msg {
		// Convert model.Conversation to storage.StoredConversation
		storedConv := convertToStoredConversation(conv, modelName, sessionStats)

		// Use custom name if provided
		if msg.Name != "" {
			storedConv.Summary = msg.Name
		}

		// Save to storage
		id, err := convStore.Save(storedConv)
		if err != nil {
			return commands.SaveCompleteMsg{Error: err}
		}

		// Mark session as clean
		if sessionMgr != nil {
			sessionMgr.MarkClean()
		}

		return commands.SaveCompleteMsg{
			ID:   id,
			Name: storedConv.Summary,
		}
	}
}

// handleSaveComplete processes save completion.
func (m *Model) handleSaveComplete(msg commands.SaveCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.chatModel.GetConversation().AddSystemMessage("Error saving conversation: " + msg.Error.Error())
	} else {
		m.chatModel.GetConversation().AddSystemMessage("Conversation saved as: " + msg.Name + " (ID: " + msg.ID + ")")
	}
	m.chatModel.SetConversation(m.chatModel.GetConversation())
	return m, nil
}

// handleLoadConversation loads a conversation from storage.
func (m *Model) handleLoadConversation(msg commands.LoadConversationMsg) (tea.Model, tea.Cmd) {
	if m.convStore == nil {
		m.chatModel.GetConversation().AddSystemMessage("Error: Session storage not available")
		m.chatModel.SetConversation(m.chatModel.GetConversation())
		return m, nil
	}

	// Capture convStore before closure to avoid race conditions
	convStore := m.convStore

	return m, func() tea.Msg {
		var storedConv *storage.StoredConversation
		var err error

		// Check if ID is a number (index) or an actual ID
		if idx, parseErr := strconv.Atoi(msg.ID); parseErr == nil {
			// Load by index (1-based for user friendliness)
			storedConv, err = convStore.LoadByIndex(idx - 1)
		} else {
			// Load by ID
			storedConv, err = convStore.Load(msg.ID)
		}

		if err != nil {
			return commands.LoadCompleteMsg{Error: err}
		}

		return SessionLoadedMsg{
			Conversation: storedConv,
		}
	}
}

// SessionLoadedMsg contains the loaded conversation data.
type SessionLoadedMsg struct {
	Conversation *storage.StoredConversation
}

// handleLoadComplete processes load completion.
func (m *Model) handleLoadComplete(msg commands.LoadCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.chatModel.GetConversation().AddSystemMessage("Error loading conversation: " + msg.Error.Error())
		m.chatModel.SetConversation(m.chatModel.GetConversation())
	}
	return m, nil
}

// handleListSessions shows available saved sessions.
func (m *Model) handleListSessions() (tea.Model, tea.Cmd) {
	if m.convStore == nil {
		m.chatModel.GetConversation().AddSystemMessage("Error: Session storage not available")
		m.chatModel.SetConversation(m.chatModel.GetConversation())
		return m, nil
	}

	// Capture convStore before closure to avoid race conditions
	convStore := m.convStore

	return m, func() tea.Msg {
		sessions, err := convStore.List()
		if err != nil {
			return commands.ErrorMsg{
				Title:   "Error listing sessions",
				Message: err.Error(),
			}
		}

		return SessionListMsg{Sessions: sessions}
	}
}

// SessionListMsg contains the list of available sessions.
type SessionListMsg struct {
	Sessions []storage.ConversationMeta
}

// convertToStoredConversation converts a model.Conversation to storage.StoredConversation.
func convertToStoredConversation(conv *model.Conversation, modelName string, stats *router.SessionStats) *storage.StoredConversation {
	messages := conv.GetHistory()
	stored := &storage.StoredConversation{
		ID:        conv.ID,
		Summary:   conv.GetTitle(),
		Model:     modelName,
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
		Messages:  make([]storage.StoredMessage, 0, len(messages)),
	}

	// Track routing cost from session stats
	if stats != nil {
		snapshot := stats.GetStats()
		stored.TokensUsed = snapshot.TotalQueries * 500 // Rough estimate
	}

	// Convert messages
	for _, msg := range messages {
		storedMsg := storage.StoredMessage{
			ID:           msg.ID,
			Role:         string(msg.Role),
			Content:      msg.GetDisplayContent(),
			Timestamp:    msg.Timestamp,
			TokenCount:   msg.TokenCount,
			DurationMs:   msg.TotalDuration.Milliseconds(),
			TokensPerSec: msg.TokensPerSec,
			TTFTMs:       msg.TTFT.Milliseconds(),
		}

		// Include tool info if present
		if msg.Role == model.RoleTool {
			storedMsg.ToolName = msg.ToolName
			storedMsg.ToolInput = msg.ToolInput
			storedMsg.ToolResult = msg.ToolResult
			storedMsg.IsSuccess = msg.IsSuccess
		}

		stored.Messages = append(stored.Messages, storedMsg)
	}

	return stored
}

// convertFromStoredConversation converts a storage.StoredConversation to model.Conversation.
func convertFromStoredConversation(stored *storage.StoredConversation) *model.Conversation {
	conv := model.NewConversation()
	conv.ID = stored.ID
	conv.Title = stored.Summary
	conv.Model = stored.Model
	conv.CreatedAt = stored.CreatedAt
	conv.UpdatedAt = stored.UpdatedAt

	// Convert messages
	for _, storedMsg := range stored.Messages {
		var msg *model.Message

		switch storedMsg.Role {
		case "user":
			msg = model.NewUserMessage(storedMsg.Content)
		case "assistant":
			msg = model.NewMessage(model.RoleAssistant, storedMsg.Content)
			msg.TokenCount = storedMsg.TokenCount
			msg.TotalDuration = time.Duration(storedMsg.DurationMs) * time.Millisecond
			msg.TokensPerSec = storedMsg.TokensPerSec
			msg.TTFT = time.Duration(storedMsg.TTFTMs) * time.Millisecond
		case "system":
			msg = model.NewSystemMessage(storedMsg.Content)
		case "tool":
			msg = model.NewToolMessage(storedMsg.ToolName, storedMsg.ToolResult, storedMsg.IsSuccess)
		default:
			continue
		}

		msg.ID = storedMsg.ID
		msg.Timestamp = storedMsg.Timestamp
		conv.Messages = append(conv.Messages, msg)
	}

	return conv
}

// handleSessionLoaded processes a successfully loaded session.
func (m *Model) handleSessionLoaded(msg SessionLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Conversation == nil {
		m.chatModel.GetConversation().AddSystemMessage("Error: Empty session data")
		m.chatModel.SetConversation(m.chatModel.GetConversation())
		return m, nil
	}

	// Convert stored conversation to model conversation
	conv := convertFromStoredConversation(msg.Conversation)

	// Set the conversation on the chat model
	m.chatModel.SetConversation(conv)

	// Update model name if different
	if msg.Conversation.Model != "" && msg.Conversation.Model != m.modelName {
		m.modelName = msg.Conversation.Model
		if m.ollamaClient != nil {
			m.ollamaClient.SetModel(msg.Conversation.Model)
		}
	}

	// Show success message
	m.chatModel.GetConversation().AddSystemMessage("Loaded session: " + msg.Conversation.Summary)
	m.chatModel.SetConversation(m.chatModel.GetConversation())

	return m, nil
}

// handleSessionList processes the session list and displays it.
func (m *Model) handleSessionList(msg SessionListMsg) (tea.Model, tea.Cmd) {
	if len(msg.Sessions) == 0 {
		m.chatModel.GetConversation().AddSystemMessage("No saved sessions found")
		m.chatModel.SetConversation(m.chatModel.GetConversation())
		return m, nil
	}

	// Build the session list display
	var sb strings.Builder
	sb.WriteString("Saved Sessions:\n")
	sb.WriteString(strings.Repeat("-", 50) + "\n")

	for i, session := range msg.Sessions {
		// Format: [index] Title (Model) - Messages - Date
		sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, session.Summary))
		sb.WriteString(fmt.Sprintf("    Model: %s | Messages: %d | Updated: %s\n",
			session.Model,
			session.MessageCount,
			session.UpdatedAt.Format("2006-01-02 15:04")))
		if session.Preview != "" {
			preview := session.Preview
			// Use rune-based truncation for Unicode safety
			previewRunes := []rune(preview)
			if len(previewRunes) > 60 {
				preview = string(previewRunes[:57]) + "..."
			}
			sb.WriteString(fmt.Sprintf("    Preview: %s\n", preview))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Use /load <number> to load a session")

	m.chatModel.GetConversation().AddSystemMessage(sb.String())
	m.chatModel.SetConversation(m.chatModel.GetConversation())

	return m, nil
}

// =============================================================================
// SESSION CLI COMMAND HANDLER
// =============================================================================

// handleSession handles the "rigrun session" CLI command.
// This provides IL5 AC-12 session termination capabilities.
func handleSession(args cli.Args) {
	if err := cli.HandleSession(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// =============================================================================
// IL5 AC-12: SESSION TIMEOUT HANDLERS
// =============================================================================

// startSessionTimeoutTick starts the periodic tick for session timeout checking.
func (m *Model) startSessionTimeoutTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return components.SessionTimeoutTickMsg{Time: t}
	})
}

// handleSessionTimeoutTick processes the periodic session timeout tick.
// Checks for warning threshold and expiration.
func (m *Model) handleSessionTimeoutTick(msg components.SessionTimeoutTickMsg) (tea.Model, tea.Cmd) {
	// Calculate time remaining
	elapsed := time.Since(m.sessionLastActivity)
	remaining := m.sessionTimeout - elapsed

	// Check for session expiration
	if remaining <= 0 {
		// Log session timeout
		security.AuditLogEvent(m.sessionMgr.SessionID(), "SESSION_TIMEOUT", map[string]string{
			"idle_duration": elapsed.String(),
		})
		// Show expired state and trigger quit
		m.sessionTimeoutOverlay.Show(0)
		m.sessionTimeoutOverlay.SetSize(m.width, m.height)
		// Brief delay to show expiration message before quitting
		return m, tea.Sequence(
			tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return components.SessionExpiredMsg{}
			}),
		)
	}

	// Check for warning threshold (2 minutes before timeout)
	warningThreshold := components.DefaultWarningThreshold
	if remaining <= warningThreshold && !m.sessionWarningShown {
		// Show warning overlay
		m.sessionWarningShown = true
		m.sessionTimeoutOverlay.Show(remaining)
		m.sessionTimeoutOverlay.SetSize(m.width, m.height)
		// Log warning
		security.AuditLogEvent(m.sessionMgr.SessionID(), "SESSION_TIMEOUT_WARNING", map[string]string{
			"remaining": remaining.String(),
		})
	} else if m.sessionTimeoutOverlay.IsVisible() {
		// Update countdown while warning is visible
		m.sessionTimeoutOverlay.UpdateTime(remaining)
	}

	// Continue ticking
	return m, m.startSessionTimeoutTick()
}

// handleSessionTimeoutWarning processes the session timeout warning message.
func (m *Model) handleSessionTimeoutWarning(msg components.SessionTimeoutWarningMsg) (tea.Model, tea.Cmd) {
	// Show warning overlay
	m.sessionWarningShown = true
	m.sessionTimeoutOverlay.Show(msg.TimeRemaining)
	m.sessionTimeoutOverlay.SetSize(m.width, m.height)
	return m, nil
}

// handleSessionExpired processes the session expiration and exits gracefully.
func (m *Model) handleSessionExpired() (tea.Model, tea.Cmd) {
	// Save any unsaved work before exiting
	if m.convStore != nil && m.chatModel.GetConversation() != nil && !m.chatModel.GetConversation().IsEmpty() {
		// Auto-save conversation on timeout
		conv := m.chatModel.GetConversation()
		storedConv := convertToStoredConversation(conv, m.modelName, m.chatModel.GetSessionStats())
		storedConv.Summary = "[Timeout] " + storedConv.Summary
		_, _ = m.convStore.Save(storedConv) // Best effort save
	}

	// Log shutdown due to timeout
	security.AuditLogEvent(m.sessionMgr.SessionID(), "SESSION_EXPIRED_SHUTDOWN", nil)

	// Exit gracefully
	return m, tea.Quit
}

// handleSessionExtended processes the session extension when user presses a key.
func (m *Model) handleSessionExtended() (tea.Model, tea.Cmd) {
	// Reset activity timer (already done in handleKeyPress)
	// Hide overlay and resume normal operation
	m.sessionTimeoutOverlay.Hide()
	m.sessionWarningShown = false
	return m, nil
}
