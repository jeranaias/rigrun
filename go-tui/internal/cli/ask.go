// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// ask.go - Single query command handler for rigrun CLI.
//
// CLI: Comprehensive help and examples for all commands
// USABILITY: Markdown rendering and history for better CLI experience
//
// Handles the "rigrun ask" command which sends a single question to the LLM
// and streams the response to stdout.
//
// Command: ask [question]
// Short:   Ask a single question
// Aliases: (none)
//
// Examples:
//   rigrun ask "What is the capital of France?"
//   rigrun ask --json "List all running processes"
//   rigrun ask --local "Explain this error"
//   rigrun ask "Review this code:" --file main.go
//   rigrun ask --agentic "Find all TODO comments in this project"
//   rigrun ask --agentic --max-iter 50 "Refactor the config module"
//
// Flags:
//   -f, --file FILE     Include file content with the question
//   -m, --model NAME    Use specific model (overrides config)
//   -a, --agentic       Enable agentic mode (tool use)
//   --max-iter N        Max iterations in agentic mode (default: 25)
//   --json              Output response as JSON
//   --local             Force local model (alias for --paranoid)
//   -v, --verbose       Verbose output
//   -q, --quiet         Minimal output
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/cloud"
	"github.com/jeranaias/rigrun-tui/internal/config"
	ctxmention "github.com/jeranaias/rigrun-tui/internal/context"
	"github.com/jeranaias/rigrun-tui/internal/offline"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
	"github.com/jeranaias/rigrun-tui/internal/router"
	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/tools"
	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// CONSTANTS
// =============================================================================

const (
	// MaxFileSize is the maximum file size to include (50KB).
	MaxFileSize = 50 * 1024
)

// =============================================================================
// PERFORMANCE: Pre-compiled regex (compiled once at startup)
// =============================================================================

var (
	// Tool call parsing patterns
	askJsonToolCallRegex = regexp.MustCompile(`(?s)\{[^{}]*"name"\s*:\s*"[^"]+"\s*,\s*"(?:parameters|arguments)"\s*:\s*\{[^{}]*\}[^{}]*\}`)
	askCodeBlockRegex    = regexp.MustCompile("(?s)```(?:json)?\\s*({.+?})\\s*```")
)

// =============================================================================
// MARKDOWN RENDERING
// =============================================================================

// markdownRenderer is the global glamour renderer for markdown output.
// USABILITY: Renders markdown responses with syntax highlighting and formatting.
var markdownRenderer *glamour.TermRenderer

func init() {
	var err error
	markdownRenderer, err = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		// Fallback to plain text if renderer initialization fails
		markdownRenderer = nil
	}
}

// renderMarkdown renders markdown content for terminal display.
// Returns the original content if rendering fails or renderer is unavailable.
func renderMarkdown(content string) string {
	if markdownRenderer == nil {
		return content
	}

	rendered, err := markdownRenderer.Render(content)
	if err != nil {
		return content
	}
	return rendered
}

// displayResponse displays a response with markdown rendering when appropriate.
// Only renders markdown when stdout is a TTY to avoid corrupting piped output.
func displayResponse(response string) {
	if IsStdoutTTY() {
		fmt.Print(renderMarkdown(response))
	} else {
		fmt.Print(response)
	}
}

// =============================================================================
// STYLES
// =============================================================================

var (
	// Routing info style
	routingStyle = lipgloss.NewStyle().
			Foreground(styles.Cyan)

	// Cost estimate style
	costStyle = lipgloss.NewStyle().
			Foreground(styles.TextMuted)

	// Separator style
	separatorStyle = lipgloss.NewStyle().
			Foreground(styles.Overlay)

	// Summary label style
	summaryLabelStyle = lipgloss.NewStyle().
				Foreground(styles.TextSecondary)

	// Summary value style
	summaryValueStyle = lipgloss.NewStyle().
				Foreground(styles.Emerald)

	// Error style
	errorStyle = lipgloss.NewStyle().
			Foreground(styles.Rose)
)

// =============================================================================
// STREAMING CALLBACK
// =============================================================================

// StreamCallback is a function that receives streamed tokens.
type StreamCallback func(token string)

// streamToStdout prints tokens directly to stdout.
func streamToStdout(token string) {
	fmt.Print(token)
}

// =============================================================================
// FILE READING
// =============================================================================

// readFileForContext reads a file and formats it for inclusion in a prompt.
// Returns the formatted content or an error.
// Files larger than MaxFileSize are rejected.
func readFileForContext(path string) (string, error) {
	// Check file info
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", fmt.Errorf("cannot access file: %w", err)
	}

	// Check size
	if info.Size() > MaxFileSize {
		return "", fmt.Errorf("file too large: %d bytes (max %d bytes)", info.Size(), MaxFileSize)
	}

	// Read content
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Format with header
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("\n--- File: %s ---\n", path))
	builder.Write(content)
	builder.WriteString("\n--- End of file ---\n")

	return builder.String(), nil
}

// =============================================================================
// ASK HANDLER
// =============================================================================

// HandleAskCommand handles the "ask" command with full routing and streaming support.
// This replaces the stub implementation in cli.go.
// Supports JSON output for IL5 SIEM integration (AU-6, SI-4).
func HandleAskCommand(args Args) error {
	// Load configuration
	cfg := config.Global()

	// ==========================================================================
	// IL5 SC-7: Offline Mode Setup
	// Block ALL network except localhost Ollama when --no-network or config set
	// ==========================================================================
	if args.NoNetwork || cfg.Routing.OfflineMode {
		offline.SetOfflineMode(true)
		if !args.Quiet && !args.JSON {
			fmt.Fprintf(os.Stderr, "%s %s\n",
				lipgloss.NewStyle().Foreground(styles.Rose).Bold(true).Render("[OFFLINE MODE]"),
				"IL5 SC-7: Network restricted to localhost only")
		}
	}

	// Get the question from args.Query (built by parseAskArgs from positional args)
	// NOTE: Do NOT use args.Raw here - it contains unparsed flags like "--agentic"
	question := args.Query

	// If no question from args, try reading from stdin (for piped input)
	if question == "" {
		// Check if stdin has data (is a pipe, not a terminal)
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Stdin is a pipe, read from it
			reader := bufio.NewReader(os.Stdin)
			stdinData, err := io.ReadAll(reader)
			if err == nil && len(stdinData) > 0 {
				question = strings.TrimSpace(string(stdinData))
				if !args.Quiet {
					fmt.Fprintf(os.Stderr, "%s Read question from stdin (%d bytes)\n",
						lipgloss.NewStyle().Foreground(styles.Cyan).Render("[+]"),
						len(stdinData))
				}
			}
		}
	}

	if question == "" {
		err := fmt.Errorf("no question provided. Usage: rigrun ask \"your question\"")
		if args.JSON {
			resp := NewJSONErrorResponse("ask", err)
			resp.Print()
		}
		return err
	}

	// If file is specified, read and append to question
	if args.File != "" {
		fileContent, err := readFileForContext(args.File)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		question = question + "\n" + fileContent

		if !args.Quiet {
			fmt.Fprintf(os.Stderr, "%s Including file: %s\n",
				lipgloss.NewStyle().Foreground(styles.Cyan).Render("[+]"),
				args.File)
		}
	}

	// ==========================================================================
	// CONTEXT EXPANSION - Process @ mentions before sending to LLM
	// Supports: @file:path, @git, @codebase, @error, @clipboard
	// ==========================================================================
	if ctxmention.HasMentions(question) {
		expander := ctxmention.NewExpander(nil)
		result := expander.Expand(question)

		// Display what context was included (unless --quiet)
		if !args.Quiet && result.Summary.TotalCount > 0 {
			fmt.Fprintf(os.Stderr, "%s Context: %s\n",
				lipgloss.NewStyle().Foreground(styles.Cyan).Render("[+]"),
				result.Summary.FormatSummary())
		}

		// Report any errors fetching context
		if result.HasErrors() && !args.Quiet {
			fmt.Fprintf(os.Stderr, "%s Context errors: %s\n",
				lipgloss.NewStyle().Foreground(styles.Rose).Render("[!]"),
				result.ErrorSummary())
		}

		// Use the expanded message (with context prepended) for the LLM
		question = result.ExpandedMessage
	}

	// Route the query (passing config for routing decisions)
	// In offline mode, always route to local and force paranoid mode
	routerOpts := &router.RouterOptions{
		Mode:            cfg.Routing.DefaultMode,
		MaxTier:         cfg.Routing.MaxTier,
		Paranoid:        args.Paranoid || cfg.Routing.ParanoidMode || offline.IsOfflineMode(),
		HasCloudKey:     cfg.Cloud.OpenRouterKey != "" && !offline.IsOfflineMode(),
		AutoPreferLocal: cfg.Routing.AutoPreferLocal,
		AutoMaxCost:     cfg.Routing.AutoMaxCost,
		AutoFallback:    cfg.Routing.AutoFallback,
	}

	// AGENTIC MODE: Force OpenRouter auto-routing for tool-use tasks when available
	// Agentic tasks require capable models that support function calling - cache/local tiers won't work
	// We use "auto" mode (not "cloud") because ShouldUseOpenRouterAuto() only returns true for "auto"/"hybrid"
	if args.Agentic && routerOpts.HasCloudKey && !routerOpts.Paranoid {
		routerOpts.Mode = "auto"
		routerOpts.AutoPreferLocal = false // Don't prefer local for agentic tasks
	}

	// Default to Unclassified for CLI (TUI uses interactive classification)
	// Classification enforcement ensures CUI+ data stays on-premise (NIST AC-4)
	decision := router.RouteQueryDetailed(question, security.ClassificationUnclassified, routerOpts)

	// Display routing decision (unless --quiet)
	if !args.Quiet && !args.JSON {
		displayRoutingDecision(decision)
	}

	// ==========================================================================
	// IL5 SC-7: Validate Ollama URL in offline mode
	// Only localhost connections are allowed
	// ==========================================================================
	if err := offline.ValidateOllamaURL(cfg.Local.OllamaURL); err != nil {
		if args.JSON {
			resp := NewJSONErrorResponse("ask", err)
			resp.Print()
		}
		return err
	}

	// Create Ollama client with config
	ollamaConfig := &ollama.ClientConfig{
		BaseURL:      cfg.Local.OllamaURL,
		DefaultModel: cfg.Local.OllamaModel,
	}
	client := ollama.NewClientWithConfig(ollamaConfig)

	// Check if Ollama is running
	ctx := context.Background()
	if err := client.CheckRunning(ctx); err != nil {
		err := fmt.Errorf("Ollama is not running. Start it with: ollama serve")
		if args.JSON {
			resp := NewJSONErrorResponse("ask", err)
			resp.Print()
		}
		return err
	}

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

	// ==========================================================================
	// AGENTIC MODE - Enable tool use with execution loop
	// ==========================================================================
	if args.Agentic {
		// Check if routing decision says to use cloud
		useCloud := decision.Tier == router.TierAuto || decision.Tier == router.TierCloud ||
			decision.Tier == router.TierHaiku || decision.Tier == router.TierSonnet ||
			decision.Tier == router.TierOpus || decision.Tier == router.TierGpt4o

		// Use cloud for agentic tasks when available (much better tool support)
		if useCloud && cfg.Cloud.OpenRouterKey != "" && !args.Paranoid {
			cloudModel := args.Model
			if cloudModel == "" {
				cloudModel = "openrouter/auto" // Let OpenRouter pick optimal model
			}

			if !args.Quiet {
				fmt.Fprintf(os.Stderr, "%s Using cloud model: %s\n",
					lipgloss.NewStyle().Foreground(styles.Cyan).Render("[AGENTIC]"),
					cloudModel)
			}

			return runCloudAgenticLoop(ctx, cfg, cloudModel, question, args)
		}

		// Fallback to local Ollama for agentic mode
		// Upgrade to a more capable model if available
		agenticModel := model
		rec := tools.CheckAgenticCapability(model)

		// If current model isn't suitable, try to upgrade
		if !rec.Suitable {
			// Try progressively better local models
			betterModels := []string{
				"qwen2.5:32b",    // Best for agentic tasks
				"qwen2.5:14b",    // Good balance
				"mistral:latest", // Decent fallback
				"llama3.2:latest", // Another option
			}

			for _, betterModel := range betterModels {
				// Check if model exists
				available := client.ModelExists(ctx, betterModel)
				if available {
					betterRec := tools.CheckAgenticCapability(betterModel)
					if betterRec.Suitable || betterRec.Warning == "" {
						agenticModel = betterModel
						if !args.Quiet {
							fmt.Fprintf(os.Stderr, "%s Upgrading to %s for better tool support\n",
								lipgloss.NewStyle().Foreground(styles.Cyan).Render("[AGENTIC]"),
								agenticModel)
						}
						break
					}
				}
			}
		}

		// Re-check capability with possibly upgraded model
		rec = tools.CheckAgenticCapability(agenticModel)
		if !rec.Suitable && !args.Quiet {
			fmt.Fprintf(os.Stderr, "%s %s\n",
				lipgloss.NewStyle().Foreground(styles.Amber).Bold(true).Render("[AGENTIC WARNING]"),
				rec.Warning)
			fmt.Fprintf(os.Stderr, "%s %s\n",
				lipgloss.NewStyle().Foreground(styles.Amber).Render("[RECOMMENDATION]"),
				rec.Recommendation)
			if rec.SuggestModel != "" {
				fmt.Fprintf(os.Stderr, "%s Consider using: %s\n",
					lipgloss.NewStyle().Foreground(styles.Cyan).Render("[SUGGESTED]"),
					rec.SuggestModel)
			}
			fmt.Fprintln(os.Stderr) // Blank line
		}

		// Use specialized agentic prompt with platform awareness
		cwd, _ := os.Getwd()
		agenticPrompt := tools.GenerateAgenticLoopPromptWithContext(runtime.GOOS, cwd)
		agenticMessages := []ollama.Message{
			ollama.NewSystemMessage(agenticPrompt),
			ollama.NewUserMessage(question),
		}
		return runAgenticLoop(ctx, client, agenticModel, agenticMessages, args)
	}

	// Build messages with system prompt optimized for small models
	systemPrompt := tools.GenerateSmallModelPrompt()
	messages := []ollama.Message{
		ollama.NewSystemMessage(systemPrompt),
		ollama.NewUserMessage(question),
	}

	// Track timing and tokens
	startTime := time.Now()
	var totalTokens int
	var inputTokens int
	var outputTokens int

	// Create accumulator for stats
	accumulator := ollama.NewStreamAccumulator()

	// Collect full response for JSON mode and markdown rendering
	var fullResponse strings.Builder
	var streamErr error

	// Determine if we should use markdown rendering
	// USABILITY: Render markdown on TTY for better formatting, stream plain for pipes
	useMarkdown := IsStdoutTTY() && !args.JSON

	// Stream the response
	if !args.Quiet && !args.JSON {
		fmt.Println() // Space before response
	}

	err := client.ChatStream(ctx, model, messages, func(chunk ollama.StreamChunk) {
		if chunk.Error != nil {
			if !args.JSON {
				fmt.Fprintf(os.Stderr, "\n%s %v\n",
					errorStyle.Render("[Error]"),
					chunk.Error)
			}
			// Store error for @error mention retrieval
			ctxmention.StoreLastError("Streaming Error: " + chunk.Error.Error())
			streamErr = chunk.Error
			return
		}

		// Collect the content
		fullResponse.WriteString(chunk.Content)

		// Stream output in non-JSON mode when not using markdown
		// When using markdown, we collect and render at the end for proper formatting
		if !args.JSON && !useMarkdown {
			streamToStdout(chunk.Content)
		}

		// Update accumulator
		accumulator.Add(chunk)

		// Capture final stats
		if chunk.Done {
			inputTokens = chunk.PromptTokens
			outputTokens = chunk.CompletionTokens
			totalTokens = inputTokens + outputTokens
		}
	})

	duration := time.Since(startTime)

	if err != nil {
		// Store error for @error mention retrieval
		ctxmention.StoreLastError("Streaming failed: " + err.Error())
		if args.JSON {
			resp := NewJSONErrorResponse("ask", err)
			resp.Print()
		}
		return fmt.Errorf("streaming failed: %w", err)
	}

	if streamErr != nil {
		if args.JSON {
			resp := NewJSONErrorResponse("ask", streamErr)
			resp.Print()
		}
		return streamErr
	}

	// Calculate actual cost
	cost := decision.Tier.CalculateCostCents(uint32(inputTokens), uint32(outputTokens))

	// JSON output mode
	if args.JSON {
		data := AskData{
			Response:     fullResponse.String(),
			Tier:         decision.Tier.Name(),
			Model:        model,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  totalTokens,
			CostCents:    cost,
			DurationMs:   duration.Milliseconds(),
			Complexity:   decision.Complexity.String(),
		}

		resp := NewJSONResponse("ask", data)
		return resp.Print()
	}

	// USABILITY: Display response with markdown rendering when on TTY
	if useMarkdown {
		displayResponse(fullResponse.String())
	}

	// Ensure newline after response
	fmt.Println()

	// Show cost summary (unless --quiet)
	if !args.Quiet {
		displayCostSummary(decision.Tier, inputTokens, outputTokens, totalTokens, duration)
	}

	return nil
}

// displayRoutingDecision shows the routing decision to the user.
func displayRoutingDecision(decision router.RoutingDecision) {
	tierName := decision.Tier.Name()
	complexityName := decision.Complexity.String()

	// Format the routing message
	var tierDesc string
	switch decision.Tier {
	case router.TierCache:
		tierDesc = "Cache (instant)"
	case router.TierLocal:
		tierDesc = "Local (Ollama)"
	case router.TierAuto:
		tierDesc = "Auto (OpenRouter routing)"
	case router.TierCloud:
		tierDesc = "Cloud (OpenRouter auto)"
	case router.TierHaiku:
		tierDesc = "Cloud (Claude Haiku)"
	case router.TierSonnet:
		tierDesc = "Cloud (Claude Sonnet)"
	case router.TierOpus:
		tierDesc = "Cloud (Claude Opus)"
	case router.TierGpt4o:
		tierDesc = "Cloud (GPT-4o)"
	default:
		tierDesc = tierName
	}

	// For auto mode, show special indicator
	if decision.IsAutoRouted {
		fmt.Fprintf(os.Stderr, "%s %s -> %s\n",
			routingStyle.Render("Routing:"),
			complexityName,
			lipgloss.NewStyle().Foreground(styles.Purple).Render(tierDesc))
	} else {
		fmt.Fprintf(os.Stderr, "%s %s -> %s\n",
			routingStyle.Render("Routing:"),
			complexityName,
			tierDesc)
	}

	// Show estimated cost
	if decision.EstimatedCostCents > 0 {
		fmt.Fprintf(os.Stderr, "%s ~%.2f cents\n",
			costStyle.Render("Estimated cost:"),
			decision.EstimatedCostCents)
	} else {
		fmt.Fprintf(os.Stderr, "%s free (local)\n",
			costStyle.Render("Estimated cost:"))
	}

	fmt.Fprintln(os.Stderr) // Blank line before response
}

// displayCostSummary shows the final cost summary after response.
func displayCostSummary(tier router.Tier, inputTokens, outputTokens, totalTokens int, duration time.Duration) {
	// Print separator
	separator := strings.Repeat("\u2500", 45)
	fmt.Fprintln(os.Stderr, separatorStyle.Render(separator))

	// Calculate actual cost
	cost := tier.CalculateCostCents(uint32(inputTokens), uint32(outputTokens))

	// Calculate savings vs Opus
	savings := router.CalculateSavingsVsOpus(tier, inputTokens, outputTokens)

	// Format tier display
	tierDisplay := tier.Name()
	if tier.IsLocal() {
		tierDisplay = lipgloss.NewStyle().Foreground(styles.Emerald).Render(tierDisplay)
	} else if tier.IsAuto() {
		tierDisplay = lipgloss.NewStyle().Foreground(styles.Purple).Render(tierDisplay)
	} else {
		tierDisplay = lipgloss.NewStyle().Foreground(styles.Amber).Render(tierDisplay)
	}

	// Build summary line
	fmt.Fprintf(os.Stderr, "%s %s | %s %s | %s %.3f cents",
		summaryLabelStyle.Render("Tier:"),
		tierDisplay,
		summaryLabelStyle.Render("Tokens:"),
		summaryValueStyle.Render(formatNumber(totalTokens)),
		summaryLabelStyle.Render("Cost:"),
		cost)

	// Show savings if any
	if savings > 0 {
		fmt.Fprintf(os.Stderr, " | %s %.2f cents vs Opus",
			summaryLabelStyle.Render("Saved:"),
			savings)
	}

	fmt.Fprintln(os.Stderr) // Final newline
}

// formatNumber formats an integer with commas for thousands.
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Simple comma formatting
	s := fmt.Sprintf("%d", n)
	result := make([]byte, 0, len(s)+len(s)/3)

	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}

	return string(result)
}

// =============================================================================
// AGENTIC MODE
// =============================================================================

// runAgenticLoop executes the agentic tool-use loop for CLI mode.
// This allows the model to use tools (Read, Glob, Grep, Bash, WebSearch, etc.)
// and iteratively explore/act until the task is complete.
func runAgenticLoop(ctx context.Context, client *ollama.Client, model string, messages []ollama.Message, args Args) error {
	// Create tool registry with all available tools
	registry := tools.NewRegistry()

	// Convert tools to Ollama format
	ollamaTools := registry.ToOllamaTools()

	if !args.Quiet {
		fmt.Fprintf(os.Stderr, "%s Agentic mode enabled with %d tools\n",
			lipgloss.NewStyle().Foreground(styles.Cyan).Render("[AGENTIC]"),
			len(ollamaTools))
		fmt.Fprintf(os.Stderr, "%s Max iterations: %d\n",
			lipgloss.NewStyle().Foreground(styles.Cyan).Render("[AGENTIC]"),
			args.MaxIter)
		fmt.Println()
	}

	startTime := time.Now()
	var totalTokens int
	iteration := 0

	for iteration < args.MaxIter {
		iteration++

		if !args.Quiet && iteration > 1 {
			fmt.Fprintf(os.Stderr, "\n%s Iteration %d/%d\n",
				lipgloss.NewStyle().Foreground(styles.Purple).Render("[LOOP]"),
				iteration, args.MaxIter)
		}

		// Accumulate response and detect tool calls
		var responseContent strings.Builder
		var detectedToolCalls []ollama.ToolCall
		var iterTokens int

		// Stream with tools
		err := client.ChatStreamWithTools(ctx, model, messages, ollamaTools, func(chunk ollama.StreamChunk) {
			if chunk.Error != nil {
				fmt.Fprintf(os.Stderr, "\n%s %v\n", errorStyle.Render("[Error]"), chunk.Error)
				return
			}

			// Accumulate content
			if chunk.Content != "" {
				responseContent.WriteString(chunk.Content)
				if !args.JSON {
					fmt.Print(chunk.Content)
				}
			}

			// Detect tool calls
			if len(chunk.ToolCalls) > 0 {
				detectedToolCalls = append(detectedToolCalls, chunk.ToolCalls...)
			}

			// Track tokens
			if chunk.Done {
				iterTokens = chunk.PromptTokens + chunk.CompletionTokens
			}
		})

		if err != nil {
			return fmt.Errorf("agentic streaming failed: %w", err)
		}

		totalTokens += iterTokens

		// If no structured tool calls detected, try to parse from JSON text output
		// (Many small models output tool calls as JSON text rather than structured calls)
		if len(detectedToolCalls) == 0 {
			parsedCalls := parseToolCallsFromText(responseContent.String())
			if len(parsedCalls) > 0 {
				detectedToolCalls = parsedCalls
				if !args.Quiet {
					fmt.Fprintf(os.Stderr, "\n%s Parsed %d tool call(s) from text output\n",
						lipgloss.NewStyle().Foreground(styles.Purple).Render("[PARSE]"),
						len(parsedCalls))
				}
			}
		}

		// If still no tool calls detected, we're done
		if len(detectedToolCalls) == 0 {
			if !args.Quiet {
				fmt.Println() // Ensure newline
				fmt.Fprintf(os.Stderr, "\n%s Task complete after %d iteration(s)\n",
					lipgloss.NewStyle().Foreground(styles.Emerald).Render("[DONE]"),
					iteration)
			}
			break
		}

		// Execute tool calls
		if !args.Quiet {
			fmt.Fprintf(os.Stderr, "\n%s Executing %d tool call(s)...\n",
				lipgloss.NewStyle().Foreground(styles.Amber).Render("[TOOLS]"),
				len(detectedToolCalls))
		}

		// Add assistant message with tool calls
		assistantMsg := ollama.Message{
			Role:      "assistant",
			Content:   responseContent.String(),
			ToolCalls: detectedToolCalls,
		}
		messages = append(messages, assistantMsg)

		// Execute each tool and collect results
		for _, tc := range detectedToolCalls {
			toolName := tc.Function.Name
			toolArgs := tc.Function.Arguments

			if !args.Quiet {
				fmt.Fprintf(os.Stderr, "  %s %s\n",
					lipgloss.NewStyle().Foreground(styles.Cyan).Render("->"),
					toolName)
			}

			// Execute the tool
			result, err := executeToolForCLI(ctx, registry, toolName, toolArgs)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}

			// UNICODE: Rune-aware truncation preserves multi-byte characters
			if util.RuneLen(result) > 4000 {
				result = util.TruncateRunesNoEllipsis(result, 4000) + "\n... (truncated)"
			}

			// Add tool result message
			toolResultMsg := ollama.Message{
				Role:    "tool",
				Content: fmt.Sprintf("[%s result]\n%s", toolName, result),
			}
			messages = append(messages, toolResultMsg)

			if args.Verbose {
				fmt.Fprintf(os.Stderr, "    Result: %s\n",
					lipgloss.NewStyle().Foreground(styles.TextMuted).Render(
						truncateString(result, 100)))
			}
		}
	}

	// Show final summary
	duration := time.Since(startTime)
	if !args.Quiet {
		separator := strings.Repeat("\u2500", 45)
		fmt.Fprintln(os.Stderr, separatorStyle.Render(separator))
		fmt.Fprintf(os.Stderr, "%s Agentic | %s %d | %s %d | %s %v\n",
			summaryLabelStyle.Render("Mode:"),
			summaryLabelStyle.Render("Iterations:"),
			iteration,
			summaryLabelStyle.Render("Tokens:"),
			totalTokens,
			summaryLabelStyle.Render("Time:"),
			duration.Round(time.Millisecond))
	}

	return nil
}

// executeToolForCLI executes a single tool in CLI context.
func executeToolForCLI(ctx context.Context, registry *tools.Registry, toolName string, args map[string]interface{}) (string, error) {
	tool := registry.Get(toolName)
	if tool == nil {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	if tool.Executor == nil {
		return "", fmt.Errorf("tool %s has no executor", toolName)
	}

	result, err := tool.Executor.Execute(ctx, args)
	if err != nil {
		return "", err
	}

	if !result.Success {
		return fmt.Sprintf("Tool error: %s", result.Error), nil
	}

	return result.Output, nil
}

// truncateString truncates a string to maxLen runes (characters).
// UNICODE: Rune-aware truncation preserves multi-byte characters.
func truncateString(s string, maxLen int) string {
	return util.TruncateRunes(s, maxLen)
}

// =============================================================================
// TOOL CALL PARSING FROM TEXT
// =============================================================================

// jsonToolCall represents a tool call as JSON output by smaller models.
// Supports both "parameters" and "arguments" field names.
type jsonToolCall struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"`
	Arguments  map[string]interface{} `json:"arguments"`
}

// getArgs returns either Parameters or Arguments (whichever is set).
func (tc *jsonToolCall) getArgs() map[string]interface{} {
	if tc.Parameters != nil {
		return tc.Parameters
	}
	if tc.Arguments != nil {
		return tc.Arguments
	}
	return make(map[string]interface{})
}

// parseToolCallsFromText attempts to parse tool calls from JSON text output.
// Uses pre-compiled askJsonToolCallRegex and askCodeBlockRegex.
// Many small models output tool calls as JSON text like:
// ```json
// {"name": "Glob", "parameters": {"pattern": "*.py"}}
// ```
// Also supports "arguments" instead of "parameters".
func parseToolCallsFromText(text string) []ollama.ToolCall {
	var calls []ollama.ToolCall

	// Pattern to find JSON objects with "name" and either "parameters" or "arguments"
	// Using pre-compiled regex
	matches := askJsonToolCallRegex.FindAllString(text, -1)
	for _, match := range matches {
		var tc jsonToolCall
		if err := json.Unmarshal([]byte(match), &tc); err == nil {
			if tc.Name != "" {
				args := tc.getArgs()
				calls = append(calls, ollama.ToolCall{
					Function: ollama.ToolFunction{
						Name:      tc.Name,
						Arguments: args,
					},
				})
			}
		}
	}

	// Try finding JSON in code blocks if no matches found using pre-compiled regex
	if len(calls) == 0 {
		codeMatches := askCodeBlockRegex.FindAllStringSubmatch(text, -1)
		for _, cm := range codeMatches {
			if len(cm) > 1 {
				var tc jsonToolCall
				if err := json.Unmarshal([]byte(cm[1]), &tc); err == nil {
					if tc.Name != "" {
						args := tc.getArgs()
						calls = append(calls, ollama.ToolCall{
							Function: ollama.ToolFunction{
								Name:      tc.Name,
								Arguments: args,
							},
						})
					}
				}
			}
		}
	}

	// Final fallback: try to find any valid JSON object with "name" field
	if len(calls) == 0 {
		// Try parsing the whole text as JSON
		var tc jsonToolCall
		if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &tc); err == nil {
			if tc.Name != "" {
				args := tc.getArgs()
				calls = append(calls, ollama.ToolCall{
					Function: ollama.ToolFunction{
						Name:      tc.Name,
						Arguments: args,
					},
				})
			}
		}
	}

	return calls
}

// =============================================================================
// CLOUD AGENTIC MODE
// =============================================================================

// runCloudAgenticLoop executes the agentic tool-use loop using OpenRouter cloud API.
// This provides better tool support than local models and uses openrouter/auto by default.
func runCloudAgenticLoop(ctx context.Context, cfg *config.Config, model string, question string, args Args) error {
	// Create OpenRouter client
	cloudClient := cloud.NewOpenRouterClient(cfg.Cloud.OpenRouterKey)
	cloudClient.SetModel(model)

	// Create tool registry
	registry := tools.NewRegistry()
	toolsList := registry.All()

	if !args.Quiet {
		fmt.Fprintf(os.Stderr, "%s Agentic mode enabled with %d tools (cloud)\n",
			lipgloss.NewStyle().Foreground(styles.Cyan).Render("[AGENTIC]"),
			len(toolsList))
		fmt.Fprintf(os.Stderr, "%s Max iterations: %d\n",
			lipgloss.NewStyle().Foreground(styles.Cyan).Render("[AGENTIC]"),
			args.MaxIter)
		fmt.Println()
	}

	// Build agentic system prompt with platform awareness
	cwd, _ := os.Getwd()
	agenticPrompt := tools.GenerateAgenticLoopPromptWithContext(runtime.GOOS, cwd)

	// Build messages for cloud API
	messages := []cloud.ChatMessage{
		cloud.NewSystemMessage(agenticPrompt),
		cloud.NewUserMessage(question),
	}

	startTime := time.Now()
	var totalTokens int
	var totalCost float64
	iteration := 0

	for iteration < args.MaxIter {
		iteration++

		if !args.Quiet && iteration > 1 {
			fmt.Fprintf(os.Stderr, "\n%s Iteration %d/%d\n",
				lipgloss.NewStyle().Foreground(styles.Purple).Render("[LOOP]"),
				iteration, args.MaxIter)
		}

		// Call cloud API
		resp, err := cloudClient.Chat(ctx, messages)
		if err != nil {
			return fmt.Errorf("cloud API call failed: %w", err)
		}

		// Track tokens and cost
		iterTokens := resp.Usage.PromptTokens + resp.Usage.CompletionTokens
		totalTokens += iterTokens

		// Calculate cost (check if free model)
		if !strings.HasSuffix(model, ":free") {
			inputCost := float64(resp.Usage.PromptTokens) / 1000000.0 * 3.0
			outputCost := float64(resp.Usage.CompletionTokens) / 1000000.0 * 15.0
			totalCost += inputCost + outputCost
		}

		// Get response content
		responseContent := resp.GetContent()

		// Print response
		if !args.JSON {
			fmt.Println(responseContent)
		}

		// Add assistant response to messages
		messages = append(messages, cloud.NewAssistantMessage(responseContent))

		// Try to parse tool calls from response
		parsedCalls := parseToolCallsFromText(responseContent)

		// If no tool calls detected, we're done
		if len(parsedCalls) == 0 {
			if !args.Quiet {
				fmt.Fprintf(os.Stderr, "\n%s Task complete after %d iteration(s)\n",
					lipgloss.NewStyle().Foreground(styles.Emerald).Render("[DONE]"),
					iteration)
			}
			break
		}

		if !args.Quiet {
			fmt.Fprintf(os.Stderr, "%s Parsed %d tool call(s)\n",
				lipgloss.NewStyle().Foreground(styles.Purple).Render("[PARSE]"),
				len(parsedCalls))
			fmt.Fprintf(os.Stderr, "%s Executing %d tool call(s)...\n",
				lipgloss.NewStyle().Foreground(styles.Cyan).Render("[TOOLS]"),
				len(parsedCalls))
		}

		// Execute tool calls and collect results
		var toolResults strings.Builder
		for _, tc := range parsedCalls {
			toolName := tc.Function.Name
			toolArgs := tc.Function.Arguments

			if !args.Quiet {
				fmt.Fprintf(os.Stderr, "  -> %s\n", toolName)
			}

			result, err := executeToolForCLI(ctx, registry, toolName, toolArgs)
			if err != nil {
				result = fmt.Sprintf("Error: %v", err)
			}

			// Truncate long results
			if util.RuneLen(result) > 4000 {
				result = util.TruncateRunesNoEllipsis(result, 4000) + "\n... (truncated)"
			}

			toolResults.WriteString(fmt.Sprintf("[%s result]\n%s\n\n", toolName, result))
		}

		// Add tool results as user message for next iteration
		messages = append(messages, cloud.NewUserMessage(toolResults.String()))
	}

	// Show final summary
	duration := time.Since(startTime)
	if !args.Quiet {
		separator := strings.Repeat("\u2500", 45)
		fmt.Fprintln(os.Stderr, separatorStyle.Render(separator))
		fmt.Fprintf(os.Stderr, "%s Agentic | %s %d | %s %d | %s %v",
			summaryLabelStyle.Render("Mode:"),
			summaryLabelStyle.Render("Iterations:"),
			iteration,
			summaryLabelStyle.Render("Tokens:"),
			totalTokens,
			summaryLabelStyle.Render("Time:"),
			duration.Round(time.Millisecond))

		// Show cost for non-free models
		if totalCost > 0 {
			fmt.Fprintf(os.Stderr, " | %s $%.4f",
				summaryLabelStyle.Render("Cost:"),
				totalCost)
		}
		fmt.Fprintln(os.Stderr)
	}

	return nil
}
