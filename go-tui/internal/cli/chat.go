// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// chat.go - Interactive chat command handler for rigrun CLI.
//
// CLI: Comprehensive help and examples for all commands
// USABILITY: Markdown rendering and history for better CLI experience
//
// Handles the "rigrun chat" command which provides an interactive REPL
// for conversing with the LLM.
//
// Command: chat
// Short:   Start an interactive chat session
// Aliases: (none)
//
// Examples:
//   rigrun chat                       Start interactive chat (default model)
//   rigrun chat --model qwen2.5:14b   Use specific model
//   rigrun chat --local               Force local-only mode
//   rigrun chat --paranoid            Block all cloud requests
//
// Flags:
//   -m, --model NAME    Use specific model (overrides config)
//   --local             Force local model (alias for --paranoid)
//   --paranoid          Block all cloud requests
//   -v, --verbose       Verbose output
//   -q, --quiet         Minimal output
//
// Interactive Commands (during chat):
//   /help, /h           Show available commands
//   /clear, /c          Clear conversation history
//   /model [name]       Show or switch model
//   /status, /s         Show session statistics
//   /history            Show conversation history
//   /quit, /q           Exit chat
//   Ctrl+C              Cancel current generation
//   Ctrl+D              Exit chat
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/peterh/liner"

	"github.com/jeranaias/rigrun-tui/internal/cloud"
	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/offline"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// STYLES
// =============================================================================

var (
	// Prompt style
	promptStyle = lipgloss.NewStyle().
			Foreground(styles.Cyan).
			Bold(true)

	// Welcome banner style
	welcomeStyle = lipgloss.NewStyle().
			Foreground(styles.Purple).
			Bold(true)

	// Info style
	infoStyle = lipgloss.NewStyle().
			Foreground(styles.TextSecondary)

	// Command style
	commandStyle = lipgloss.NewStyle().
			Foreground(styles.Emerald)

	// Warning style
	warningStyle = lipgloss.NewStyle().
			Foreground(styles.Amber)

	// Session summary header
	summaryHeaderStyle = lipgloss.NewStyle().
				Foreground(styles.Cyan).
				Bold(true)
)

// =============================================================================
// INPUT HISTORY
// =============================================================================

// ChatCLI provides input history and line editing for interactive chat.
// USABILITY: Supports arrow keys for history navigation and line editing.
type ChatCLI struct {
	line        *liner.State
	historyFile string
}

// NewChatCLI creates a new ChatCLI with input history support.
func NewChatCLI() *ChatCLI {
	line := liner.NewLiner()
	line.SetCtrlCAborts(true)

	// Get history file path in config directory
	configDir, err := config.ConfigDir()
	if err != nil {
		// Fallback to temp directory if config dir unavailable
		configDir = os.TempDir()
	}
	historyFile := filepath.Join(configDir, "chat_history")

	cli := &ChatCLI{
		line:        line,
		historyFile: historyFile,
	}

	// Load existing history
	cli.LoadHistory()

	return cli
}

// LoadHistory loads command history from file.
func (c *ChatCLI) LoadHistory() {
	if f, err := os.Open(c.historyFile); err == nil {
		c.line.ReadHistory(f)
		f.Close()
	}
}

// ReadInput reads a line of input with the given prompt.
// Supports history navigation with arrow keys.
func (c *ChatCLI) ReadInput(prompt string) (string, error) {
	input, err := c.line.Prompt(prompt)
	if err != nil {
		return "", err
	}

	// Add non-empty input to history
	if strings.TrimSpace(input) != "" {
		c.line.AppendHistory(input)
	}

	return input, nil
}

// SaveHistory persists command history to file with secure permissions.
func (c *ChatCLI) SaveHistory() {
	// Ensure config directory exists
	if err := config.EnsureConfigDir(); err != nil {
		return
	}

	// Create file with secure permissions (0600 - owner read/write only)
	f, err := os.OpenFile(c.historyFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	c.line.WriteHistory(f)
}

// Close saves history and closes the liner.
func (c *ChatCLI) Close() {
	c.SaveHistory()
	c.line.Close()
}

// =============================================================================
// SESSION STATE
// =============================================================================

// ChatSession holds the state for an interactive chat session.
type ChatSession struct {
	// Conversation history
	Messages []ollama.Message

	// Cloud message history (separate format for OpenRouter)
	CloudMessages []cloud.ChatMessage

	// Session statistics
	Stats *router.SessionStats

	// Configuration
	Config     *config.Config
	Model      string
	CloudModel string // Model for cloud (openrouter/auto by default)
	Quiet      bool
	Paranoid   bool

	// Tracking
	StartTime   time.Time
	TotalTokens int
	TotalCost   float64 // Cloud cost in dollars

	// Clients
	Client      *ollama.Client
	CloudClient *cloud.OpenRouterClient

	// Cancel function for current stream
	CancelFunc context.CancelFunc

	// Input history handler
	// USABILITY: Provides readline-like input with history
	InputCLI *ChatCLI
}

// NewChatSession creates a new chat session.
func NewChatSession(args Args) *ChatSession {
	// Load configuration
	cfg := config.Global()

	// ==========================================================================
	// IL5 SC-7: Offline Mode Setup
	// Block ALL network except localhost Ollama when --no-network or config set
	// ==========================================================================
	if args.NoNetwork || cfg.Routing.OfflineMode {
		offline.SetOfflineMode(true)
	}

	// Create Ollama client with config
	ollamaConfig := &ollama.ClientConfig{
		BaseURL:      cfg.Local.OllamaURL,
		DefaultModel: cfg.Local.OllamaModel,
	}
	client := ollama.NewClientWithConfig(ollamaConfig)

	// Determine model to use (CLI arg > config > client default)
	model := args.Model
	if model == "" {
		model = cfg.Local.OllamaModel
	}
	if model == "" {
		model = cfg.DefaultModel
	}
	if model == "" {
		model = client.GetDefaultModel()
	}

	// Determine paranoid mode (CLI flag OR config setting OR offline mode)
	// In offline mode, always set paranoid to block cloud
	paranoid := args.Paranoid || cfg.Routing.ParanoidMode || offline.IsOfflineMode()

	// Cloud model: use openrouter/auto by default unless user specifies a model
	// If user specifies a model that looks like an OpenRouter model, use it for cloud
	cloudModel := "openrouter/auto"
	if args.Model != "" && (strings.Contains(args.Model, "/") || strings.HasSuffix(args.Model, ":free")) {
		cloudModel = args.Model
	}

	// Create cloud client if API key is available
	var cloudClient *cloud.OpenRouterClient
	if cfg.Cloud.OpenRouterKey != "" && !paranoid {
		cloudClient = cloud.NewOpenRouterClient(cfg.Cloud.OpenRouterKey)
		cloudClient.SetModel(cloudModel)
	}

	return &ChatSession{
		Messages:      make([]ollama.Message, 0),
		CloudMessages: make([]cloud.ChatMessage, 0),
		Stats:         router.NewSessionStats(),
		Config:        cfg,
		Model:         model,
		CloudModel:    cloudModel,
		Quiet:         args.Quiet,
		Paranoid:      paranoid,
		StartTime:     time.Now(),
		Client:        client,
		CloudClient:   cloudClient,
		InputCLI:      NewChatCLI(),
	}
}

// =============================================================================
// CHAT HANDLER
// =============================================================================

// HandleChatCommand handles the "chat" command with full interactive support.
// This replaces the stub implementation in cli.go.
func HandleChatCommand(args Args) error {
	session := NewChatSession(args)

	// ==========================================================================
	// IL5 SC-7: Validate Ollama URL in offline mode
	// Only localhost connections are allowed
	// ==========================================================================
	if err := offline.ValidateOllamaURL(session.Config.Local.OllamaURL); err != nil {
		return err
	}

	// Display offline mode indicator
	if offline.IsOfflineMode() && !session.Quiet {
		fmt.Fprintf(os.Stderr, "%s %s\n",
			lipgloss.NewStyle().Foreground(styles.Rose).Bold(true).Render("[OFFLINE MODE]"),
			"IL5 SC-7: Network restricted to localhost only")
		fmt.Println()
	}

	// Check if Ollama is running
	ctx := context.Background()
	if err := session.Client.CheckRunning(ctx); err != nil {
		return fmt.Errorf("Ollama is not running. Start it with: ollama serve")
	}

	// Show welcome message
	if !session.Quiet {
		printWelcome(session)
	}

	// Ensure input history is saved on exit
	// USABILITY: Save history for future sessions
	defer session.InputCLI.Close()

	// Set up signal handling for graceful Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Handle signals in a goroutine
	go func() {
		for sig := range sigChan {
			if sig == os.Interrupt || sig == syscall.SIGTERM {
				// First Ctrl+C cancels current operation
				if session.CancelFunc != nil {
					session.CancelFunc()
					session.CancelFunc = nil
					fmt.Fprintln(os.Stderr, "\n"+warningStyle.Render("[Cancelled]"))
				}
			}
		}
	}()

	// Main REPL loop using liner for input history
	// USABILITY: Provides readline-like line editing and history navigation
	for {
		// Read input with history support
		input, err := session.InputCLI.ReadInput(promptStyle.Render("rigrun> "))
		if err != nil {
			if err == liner.ErrPromptAborted {
				// Ctrl+C pressed - exit gracefully
				fmt.Println()
				printExitSummary(session)
				return nil
			}
			// EOF (Ctrl+D) or other error - exit gracefully
			fmt.Println()
			printExitSummary(session)
			return nil
		}

		input = strings.TrimSpace(input)

		// Skip empty input
		if input == "" {
			continue
		}

		// Handle slash commands
		if strings.HasPrefix(input, "/") {
			shouldContinue, err := handleSlashCommand(input, session)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s %v\n",
					errorStyle.Render("[Error]"),
					err)
			}
			if !shouldContinue {
				printExitSummary(session)
				return nil
			}
			continue
		}

		// Handle exit/quit without slash
		if strings.EqualFold(input, "exit") || strings.EqualFold(input, "quit") {
			printExitSummary(session)
			return nil
		}

		// Process the message
		if err := processMessage(session, input); err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n",
				errorStyle.Render("[Error]"),
				err)
		}
	}
}

// =============================================================================
// MESSAGE PROCESSING
// =============================================================================

// processMessage sends a message through the router and streams the response.
func processMessage(session *ChatSession, input string) error {
	// Route the query with config-aware options
	// In offline mode, block cloud key and force paranoid
	routerOpts := &router.RouterOptions{
		Mode:        session.Config.Routing.DefaultMode,
		MaxTier:     session.Config.Routing.MaxTier,
		Paranoid:    session.Paranoid || offline.IsOfflineMode(),
		HasCloudKey: session.Config.Cloud.OpenRouterKey != "" && !offline.IsOfflineMode(),
	}
	// Default to Unclassified for CLI chat (TUI uses interactive classification)
	// Classification enforcement ensures CUI+ data stays on-premise (NIST AC-4)
	decision := router.RouteQueryDetailed(input, security.ClassificationUnclassified, routerOpts)

	// Determine if we should use cloud based on routing decision
	useCloud := !decision.Tier.IsLocal() && session.CloudClient != nil

	// Show routing decision (unless quiet)
	if !session.Quiet {
		showRoutingInfo(decision)
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	session.CancelFunc = cancel
	defer func() {
		session.CancelFunc = nil
		cancel()
	}()

	// Track stats
	var inputTokens, outputTokens int
	var iterationCost float64
	startTime := time.Now()

	// Determine if we should use markdown rendering
	// USABILITY: Render markdown on TTY for better formatting
	useMarkdown := IsStdoutTTY()

	// Stream the response
	fmt.Println() // Space before response

	var responseContent string

	if useCloud {
		// Use cloud (OpenRouter) for this query
		// Add user message to cloud history
		session.CloudMessages = append(session.CloudMessages, cloud.NewUserMessage(input))

		// Also keep local history in sync for context
		session.Messages = append(session.Messages, ollama.NewUserMessage(input))

		// Call cloud API
		resp, err := session.CloudClient.Chat(ctx, session.CloudMessages)
		if err != nil {
			// Remove messages on error
			if len(session.CloudMessages) > 0 {
				session.CloudMessages = session.CloudMessages[:len(session.CloudMessages)-1]
			}
			if len(session.Messages) > 0 {
				session.Messages = session.Messages[:len(session.Messages)-1]
			}
			return fmt.Errorf("cloud API failed: %w", err)
		}

		responseContent = resp.GetContent()
		inputTokens = resp.Usage.PromptTokens
		outputTokens = resp.Usage.CompletionTokens

		// Calculate cost (check if free model)
		if !strings.HasSuffix(session.CloudModel, ":free") {
			// Estimate cost using openrouter/auto pricing (varies, use conservative estimate)
			inputCost := float64(inputTokens) / 1000000.0 * 3.0
			outputCost := float64(outputTokens) / 1000000.0 * 15.0
			iterationCost = inputCost + outputCost
			session.TotalCost += iterationCost
		}

		// Display response
		if useMarkdown {
			displayResponse(responseContent)
		} else {
			streamToStdout(responseContent)
		}

		// Add assistant response to cloud history
		session.CloudMessages = append(session.CloudMessages, cloud.NewAssistantMessage(responseContent))
		session.Messages = append(session.Messages, ollama.NewAssistantMessage(responseContent))
	} else {
		// Use local (Ollama) for this query
		// Add user message to history
		session.Messages = append(session.Messages, ollama.NewUserMessage(input))

		// Create accumulator
		accumulator := ollama.NewStreamAccumulator()

		err := session.Client.ChatStream(ctx, session.Model, session.Messages, func(chunk ollama.StreamChunk) {
			if chunk.Error != nil {
				fmt.Fprintf(os.Stderr, "\n%s %v\n",
					errorStyle.Render("[Error]"),
					chunk.Error)
				return
			}

			// Stream output when not using markdown
			// When using markdown, we collect and render at the end for proper formatting
			if !useMarkdown {
				streamToStdout(chunk.Content)
			}

			// Update accumulator
			accumulator.Add(chunk)

			// Capture final stats
			if chunk.Done {
				inputTokens = chunk.PromptTokens
				outputTokens = chunk.CompletionTokens
			}
		})

		if err != nil {
			// Remove the user message on error
			if len(session.Messages) > 0 {
				session.Messages = session.Messages[:len(session.Messages)-1]
			}
			return fmt.Errorf("streaming failed: %w", err)
		}

		responseContent = accumulator.GetContent()

		// USABILITY: Display response with markdown rendering when on TTY
		if useMarkdown {
			displayResponse(responseContent)
		}

		// Add assistant message to history
		session.Messages = append(session.Messages, ollama.NewAssistantMessage(responseContent))
	}

	// Ensure newline after response
	fmt.Println()
	fmt.Println() // Extra space after response

	// Update session stats
	result := router.NewQueryResult(
		responseContent,
		decision.Tier,
		uint32(inputTokens),
		uint32(outputTokens),
		uint64(time.Since(startTime).Milliseconds()),
	)
	session.Stats.RecordQuery(result)
	session.TotalTokens += inputTokens + outputTokens

	// Show brief stats (unless quiet)
	if !session.Quiet {
		showBriefStats(decision.Tier, inputTokens+outputTokens, time.Since(startTime), iterationCost)
	}

	return nil
}

// showRoutingInfo displays routing decision inline.
func showRoutingInfo(decision router.RoutingDecision) {
	tierName := decision.Tier.Name()
	if decision.Tier.IsLocal() {
		tierName = lipgloss.NewStyle().Foreground(styles.Emerald).Render(tierName)
	} else {
		tierName = lipgloss.NewStyle().Foreground(styles.Amber).Render(tierName)
	}

	fmt.Fprintf(os.Stderr, "%s %s (%s)\n",
		infoStyle.Render("[Routing]"),
		tierName,
		decision.Complexity.String())
}

// showBriefStats shows brief stats after response.
func showBriefStats(tier router.Tier, tokens int, duration time.Duration, cost float64) {
	tierName := tier.Name()
	if tier.IsLocal() {
		tierName = lipgloss.NewStyle().Foreground(styles.Emerald).Render(tierName)
	} else {
		tierName = lipgloss.NewStyle().Foreground(styles.Amber).Render(tierName)
	}

	if cost > 0 {
		fmt.Fprintf(os.Stderr, "%s %s | %d tokens | %s | $%.4f\n",
			infoStyle.Render("[Stats]"),
			tierName,
			tokens,
			duration.Round(time.Millisecond),
			cost)
	} else {
		fmt.Fprintf(os.Stderr, "%s %s | %d tokens | %s\n",
			infoStyle.Render("[Stats]"),
			tierName,
			tokens,
			duration.Round(time.Millisecond))
	}
}

// =============================================================================
// SLASH COMMANDS
// =============================================================================

// handleSlashCommand processes slash commands.
// Returns (shouldContinue, error) where shouldContinue=false means exit.
func handleSlashCommand(cmd string, session *ChatSession) (bool, error) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return true, nil
	}

	command := strings.ToLower(parts[0])
	args := parts[1:]

	switch command {
	case "/help", "/h", "/?":
		printHelp()
		return true, nil

	case "/clear", "/c":
		session.Messages = session.Messages[:0]
		session.CloudMessages = session.CloudMessages[:0]
		fmt.Println(commandStyle.Render("[Conversation cleared]"))
		return true, nil

	case "/model", "/m":
		return handleModelCommand(session, args)

	case "/status", "/s":
		printStatus(session)
		return true, nil

	case "/quit", "/q", "/exit":
		return false, nil

	case "/history":
		printHistory(session)
		return true, nil

	case "/":
		// Just "/" shows help
		printHelp()
		return true, nil

	default:
		return true, fmt.Errorf("unknown command: %s (type /help for commands)", command)
	}
}

// handleModelCommand handles the /model command.
func handleModelCommand(session *ChatSession, args []string) (bool, error) {
	if len(args) == 0 {
		fmt.Printf("%s Current model: %s\n",
			infoStyle.Render("[Model]"),
			commandStyle.Render(session.Model))
		return true, nil
	}

	newModel := args[0]

	// Verify model exists (optional - could skip for cloud models)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.Client.GetModel(ctx, newModel)
	if err != nil {
		// Just warn, don't fail - might be a cloud model
		fmt.Fprintf(os.Stderr, "%s Model '%s' not found locally, will attempt to use anyway\n",
			warningStyle.Render("[Warning]"),
			newModel)
	}

	session.Model = newModel
	fmt.Printf("%s Switched to model: %s\n",
		commandStyle.Render("[OK]"),
		newModel)

	return true, nil
}

// =============================================================================
// DISPLAY FUNCTIONS
// =============================================================================

// printWelcome prints the welcome banner.
func printWelcome(session *ChatSession) {
	fmt.Println()
	fmt.Println(welcomeStyle.Render("rigrun interactive chat"))
	fmt.Println(infoStyle.Render(strings.Repeat("\u2500", 30)))
	fmt.Printf("%s %s\n",
		infoStyle.Render("Model:"),
		commandStyle.Render(session.Model))

	// Show routing mode from config
	mode := session.Config.Routing.DefaultMode
	if session.Paranoid {
		fmt.Printf("%s %s\n",
			infoStyle.Render("Mode:"),
			warningStyle.Render("Local only (paranoid)"))
	} else if mode == "hybrid" {
		fmt.Printf("%s %s\n",
			infoStyle.Render("Mode:"),
			commandStyle.Render("Hybrid (intelligent routing)"))
	} else if mode == "cloud" {
		fmt.Printf("%s %s\n",
			infoStyle.Render("Mode:"),
			commandStyle.Render("Cloud (OpenRouter)"))
	} else {
		fmt.Printf("%s %s\n",
			infoStyle.Render("Mode:"),
			commandStyle.Render("Local (Ollama)"))
	}

	// Show cloud key status if relevant
	if mode != "local" && !session.Paranoid {
		if session.Config.Cloud.OpenRouterKey != "" {
			fmt.Printf("%s %s\n",
				infoStyle.Render("Cloud:"),
				commandStyle.Render("Configured"))
		} else {
			fmt.Printf("%s %s\n",
				infoStyle.Render("Cloud:"),
				warningStyle.Render("No API key (local fallback)"))
		}
	}

	fmt.Println()
	fmt.Println(infoStyle.Render("Type your message and press Enter. Commands: /help, /quit"))
	fmt.Println()
}

// printHelp prints available commands.
func printHelp() {
	fmt.Println()
	fmt.Println(summaryHeaderStyle.Render("Available Commands"))
	fmt.Println(infoStyle.Render(strings.Repeat("\u2500", 20)))
	fmt.Println()

	commands := []struct {
		cmd  string
		desc string
	}{
		{"/help, /h", "Show this help"},
		{"/clear, /c", "Clear conversation history"},
		{"/model [name]", "Show or switch model"},
		{"/status, /s", "Show session statistics"},
		{"/history", "Show conversation history"},
		{"/quit, /q", "Exit chat"},
	}

	for _, c := range commands {
		fmt.Printf("  %s  %s\n",
			commandStyle.Render(fmt.Sprintf("%-15s", c.cmd)),
			infoStyle.Render(c.desc))
	}

	fmt.Println()
	fmt.Println(infoStyle.Render("Tip: Ctrl+C cancels current generation, Ctrl+D exits"))
	fmt.Println()
}

// printStatus prints session statistics.
func printStatus(session *ChatSession) {
	stats := session.Stats.GetStats()
	elapsed := time.Since(session.StartTime).Round(time.Second)

	fmt.Println()
	fmt.Println(summaryHeaderStyle.Render("Session Status"))
	fmt.Println(infoStyle.Render(strings.Repeat("\u2500", 20)))
	fmt.Println()

	fmt.Printf("  %s %s\n",
		infoStyle.Render("Local Model:"),
		commandStyle.Render(session.Model))
	if session.CloudClient != nil {
		fmt.Printf("  %s %s\n",
			infoStyle.Render("Cloud Model:"),
			commandStyle.Render(session.CloudModel))
	}
	fmt.Printf("  %s %s\n",
		infoStyle.Render("Duration:"),
		elapsed.String())
	fmt.Printf("  %s %d messages\n",
		infoStyle.Render("History:"),
		len(session.Messages))

	fmt.Println()
	fmt.Println(infoStyle.Render("Statistics:"))
	fmt.Printf("  %s %d (%d local, %d cloud)\n",
		infoStyle.Render("Queries:"),
		stats.TotalQueries,
		stats.LocalQueries,
		stats.CloudQueries)
	fmt.Printf("  %s %s\n",
		infoStyle.Render("Tokens:"),
		formatNumber(session.TotalTokens))

	// Show cloud cost if any
	if session.TotalCost > 0 {
		fmt.Printf("  %s $%.4f\n",
			infoStyle.Render("Cloud Cost:"),
			session.TotalCost)
	} else {
		fmt.Printf("  %s $0.00 (local or free models)\n",
			infoStyle.Render("Cloud Cost:"))
	}
	fmt.Printf("  %s %.2f cents vs Opus\n",
		infoStyle.Render("Saved:"),
		stats.TotalSavedCents)

	fmt.Println()
}

// printHistory prints conversation history.
func printHistory(session *ChatSession) {
	if len(session.Messages) == 0 {
		fmt.Println(infoStyle.Render("[No messages yet]"))
		return
	}

	fmt.Println()
	fmt.Println(summaryHeaderStyle.Render("Conversation History"))
	fmt.Println(infoStyle.Render(strings.Repeat("\u2500", 25)))
	fmt.Println()

	for i, msg := range session.Messages {
		role := msg.Role
		switch role {
		case "user":
			role = lipgloss.NewStyle().Foreground(styles.Cyan).Render("You")
		case "assistant":
			role = lipgloss.NewStyle().Foreground(styles.Purple).Render("AI")
		case "system":
			role = lipgloss.NewStyle().Foreground(styles.Amber).Render("System")
		}

		// Truncate long messages using rune-based truncation for Unicode safety
		content := msg.Content
		runes := []rune(content)
		if len(runes) > 100 {
			content = string(runes[:100]) + "..."
		}
		content = strings.ReplaceAll(content, "\n", " ")

		fmt.Printf("  %d. %s: %s\n", i+1, role, content)
	}

	fmt.Println()
}

// printExitSummary prints the session summary on exit.
func printExitSummary(session *ChatSession) {
	stats := session.Stats.GetStats()

	// Skip if no queries
	if stats.TotalQueries == 0 {
		fmt.Println(infoStyle.Render("Goodbye!"))
		return
	}

	elapsed := time.Since(session.StartTime).Round(time.Second)

	fmt.Println()
	fmt.Println(summaryHeaderStyle.Render("Session Summary"))
	fmt.Println(infoStyle.Render(strings.Repeat("\u2500", 15)))

	fmt.Printf("  %s %d (%d local, %d cloud)\n",
		infoStyle.Render("Queries:"),
		stats.TotalQueries,
		stats.LocalQueries,
		stats.CloudQueries)
	fmt.Printf("  %s %s\n",
		infoStyle.Render("Tokens:"),
		formatNumber(session.TotalTokens))

	// Show cloud cost
	if session.TotalCost > 0 {
		fmt.Printf("  %s $%.4f\n",
			infoStyle.Render("Cost:"),
			session.TotalCost)
	} else if stats.CloudQueries > 0 {
		fmt.Printf("  %s $0.00 (free models)\n",
			infoStyle.Render("Cost:"))
	}

	fmt.Printf("  %s %.2f cents vs Opus\n",
		infoStyle.Render("Saved:"),
		stats.TotalSavedCents)
	fmt.Printf("  %s %s\n",
		infoStyle.Render("Duration:"),
		elapsed.String())

	fmt.Println()
	fmt.Println(infoStyle.Render("Goodbye!"))
}
