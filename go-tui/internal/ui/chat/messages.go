// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
//
// This file defines all Bubble Tea message types used by the chat interface.
// Messages are organized into the following categories:
//   - Streaming: Stream start, token delivery, completion, errors, and fallback
//   - Ollama: Health checks, model status, and model switching
//   - Input: User input submission and cancellation
//   - Viewport: Scrolling and navigation
//   - Conversation: Load, save, clear, and session management
//   - UI State: Resize, focus, and blur events
//   - Thinking/Loading: Animation and progress indicators
//   - Errors: Error display and dismissal
//   - Copy: Clipboard operations
//   - Tools: Tool execution, permissions, and agentic loops
//   - Statistics: Token usage and context tracking
//   - Search: Search mode, queries, and navigation
//
// All message types follow Bubble Tea conventions and are immutable.
package chat

import (
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/model"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
	"github.com/jeranaias/rigrun-tui/internal/ui/components"
)

// =============================================================================
// STREAMING MESSAGES
// =============================================================================

// StreamRequestMsg requests the main model to start streaming from Ollama.
// This is sent from the chat model to the main model to initiate LLM calls.
type StreamRequestMsg struct {
	MessageID string
	Messages  []ollama.Message
	// Cloud routing fields
	UseCloud   bool   // If true, use cloud client instead of Ollama
	CloudModel string // Cloud model to use (e.g., "haiku", "sonnet", "opus")
	CloudTier  string // Tier string for display
}

// StreamStartMsg signals that streaming has begun.
type StreamStartMsg struct {
	MessageID string
	StartTime time.Time
}

// StreamTokenMsg delivers a new token from the stream.
type StreamTokenMsg struct {
	MessageID string
	Token     string
	IsFirst   bool // True if this is the first token
}

// StreamCompleteMsg signals that streaming has finished.
type StreamCompleteMsg struct {
	MessageID string
	Stats     *model.Statistics
	Error     error
}

// StreamErrorMsg signals an error during streaming.
type StreamErrorMsg struct {
	MessageID string
	Error     error
}

// RoutingFallbackMsg signals that routing fell back to a different tier.
// Used to update message metadata when cloud fails and falls back to local.
type RoutingFallbackMsg struct {
	MessageID string
	FromTier  string // Original tier that failed (e.g., "Cloud")
	ToTier    string // Tier we fell back to (e.g., "Local")
	Reason    string // Why the fallback happened (e.g., "authentication failed")
}

// =============================================================================
// OLLAMA MESSAGES
// =============================================================================

// OllamaCheckMsg requests an Ollama health check.
type OllamaCheckMsg struct{}

// OllamaStatusMsg reports Ollama connection status.
type OllamaStatusMsg struct {
	Running bool
	Error   error
}

// OllamaModelsMsg delivers the list of available models.
type OllamaModelsMsg struct {
	Models []ollama.ModelInfo
	Error  error
}

// OllamaModelSwitchMsg signals a model switch request.
type OllamaModelSwitchMsg struct {
	Model string
}

// OllamaModelSwitchedMsg confirms a model switch.
type OllamaModelSwitchedMsg struct {
	Model string
	Error error
}

// =============================================================================
// INPUT MESSAGES
// =============================================================================

// SubmitInputMsg signals that the user submitted input.
type SubmitInputMsg struct {
	Content string
}

// CancelInputMsg signals that the user cancelled input (Escape).
type CancelInputMsg struct{}

// ClearInputMsg signals that the input should be cleared.
type ClearInputMsg struct{}

// =============================================================================
// VIEWPORT MESSAGES
// =============================================================================

// ViewportScrollMsg requests a viewport scroll.
type ViewportScrollMsg struct {
	Direction int // -1 for up, +1 for down
	Amount    int // Number of lines
}

// ViewportScrollToBottomMsg scrolls to the bottom.
type ViewportScrollToBottomMsg struct{}

// ViewportScrollToTopMsg scrolls to the top.
type ViewportScrollToTopMsg struct{}

// =============================================================================
// CONVERSATION MESSAGES
// =============================================================================

// NewConversationMsg starts a new conversation.
type NewConversationMsg struct{}

// ClearConversationMsg clears the current conversation.
type ClearConversationMsg struct{}

// LoadConversationMsg loads a conversation by ID.
type LoadConversationMsg struct {
	ID string
}

// ConversationLoadedMsg delivers a loaded conversation.
type ConversationLoadedMsg struct {
	Conversation *model.Conversation
	Error        error
}

// SaveConversationMsg requests saving the current conversation.
type SaveConversationMsg struct {
	Name string // Optional custom name
}

// ConversationSavedMsg confirms a save operation.
type ConversationSavedMsg struct {
	ID    string
	Error error
}

// ListSessionsMsg requests listing available sessions.
type ListSessionsMsg struct{}

// SessionResumeMsg requests resuming a saved session.
type SessionResumeMsg struct {
	SessionID string
}

// SessionResumedMsg confirms a session was resumed successfully.
type SessionResumedMsg struct {
	SessionID    string
	Summary      string
	MessageCount int
	CreatedAt    string
	Error        error
}

// ExportConversationMsg is defined in internal/commands/handlers.go
// DO NOT duplicate - import from there instead

// =============================================================================
// UI STATE MESSAGES
// =============================================================================

// ResizeMsg signals a terminal resize.
type ResizeMsg struct {
	Width  int
	Height int
}

// FocusMsg sets focus to a component.
type FocusMsg struct {
	Component string // "input", "viewport", "command", etc.
}

// BlurMsg removes focus from a component.
type BlurMsg struct {
	Component string
}

// ShowTutorialMsg triggers showing the tutorial overlay.
type ShowTutorialMsg struct{}

// TutorialCompleteMsg signals that the tutorial completed or was skipped.
type TutorialCompleteMsg struct {
	Completed    bool // True if fully completed, false if skipped
	CurrentStep  int  // Last step reached
}

// =============================================================================
// THINKING/LOADING MESSAGES
// =============================================================================

// ThinkingStartMsg starts the thinking animation.
type ThinkingStartMsg struct {
	Message string // e.g., "Thinking", "Processing"
}

// ThinkingUpdateMsg updates the thinking state.
type ThinkingUpdateMsg struct {
	Elapsed time.Duration
	Detail  string // Optional detail text
}

// ThinkingStopMsg stops the thinking animation.
type ThinkingStopMsg struct{}

// SpinnerTickMsg advances the spinner animation.
type SpinnerTickMsg struct {
	Time time.Time
}

// =============================================================================
// ERROR MESSAGES
// =============================================================================

// ErrorMsg displays an error to the user.
type ErrorMsg struct {
	Title       string
	Message     string
	Suggestions []string
	Dismissible bool
}

// ErrorDismissMsg dismisses the current error.
type ErrorDismissMsg struct{}

// =============================================================================
// COPY MESSAGES
// =============================================================================

// CopyToClipboardMsg requests copying content to clipboard.
type CopyToClipboardMsg struct {
	Content string
}

// CopyCompleteMsg confirms a copy operation.
type CopyCompleteMsg struct {
	Success bool
	Error   error
}

// =============================================================================
// TOOL MESSAGES
// =============================================================================

// ToolCallRequestedMsg indicates the LLM wants to call a tool.
type ToolCallRequestedMsg struct {
	MessageID string
	ToolName  string
	ToolID    string
	Arguments map[string]interface{}
}

// DiffPendingMsg indicates a diff is pending approval for Edit/Write tool.
type DiffPendingMsg struct {
	MessageID string
	ToolName  string
	ToolID    string
	FilePath  string
	OldContent string
	NewContent string
}

// DiffApprovedMsg indicates the user approved the diff.
type DiffApprovedMsg struct {
	MessageID string
	ToolID    string
}

// DiffRejectedMsg indicates the user rejected the diff.
type DiffRejectedMsg struct {
	MessageID string
	ToolID    string
	Reason    string
}

// ToolExecutingMsg indicates a tool is being executed.
type ToolExecutingMsg struct {
	MessageID string
	ToolName  string
	ToolID    string
}

// ToolResultMsg delivers the result of a tool execution.
type ToolResultMsg struct {
	MessageID string
	ToolName  string
	ToolID    string
	Success   bool
	Output    string
	Error     string
	Duration  time.Duration
}

// ToolPermissionMsg requests user permission to execute a tool.
type ToolPermissionMsg struct {
	MessageID   string
	ToolName    string
	ToolID      string
	Arguments   map[string]interface{}
	Description string
	RiskLevel   string
}

// ToolPermissionResponseMsg is the user's response to a permission request.
type ToolPermissionResponseMsg struct {
	MessageID   string
	ToolID      string
	Allowed     bool
	AlwaysAllow bool
}

// ToolLoopIterationMsg indicates an agentic loop iteration completed.
type ToolLoopIterationMsg struct {
	MessageID string
	Iteration int
	ToolCalls int
}

// ToolLoopCompleteMsg indicates the agentic loop has finished.
type ToolLoopCompleteMsg struct {
	MessageID       string
	TotalIterations int
	TotalToolCalls  int
	FinalResponse   string
}

// =============================================================================
// STATISTICS MESSAGES
// =============================================================================

// StatsUpdateMsg updates the statistics display.
type StatsUpdateMsg struct {
	TokensUsed     int
	MaxTokens      int
	ContextPercent float64
	Model          string
	GPU            string
	Mode           string
}

// =============================================================================
// SEARCH MESSAGES
// =============================================================================

// SearchToggleMsg toggles search mode on/off.
type SearchToggleMsg struct{}

// SearchQueryMsg updates the search query.
type SearchQueryMsg struct {
	Query string
}

// SearchNextMsg navigates to the next search match.
type SearchNextMsg struct{}

// SearchPrevMsg navigates to the previous search match.
type SearchPrevMsg struct{}

// SearchClearMsg clears the search and exits search mode.
type SearchClearMsg struct{}

// =============================================================================
// PROGRESS MESSAGES (Agentic Loops)
// =============================================================================

// ProgressStartMsg signals the start of a multi-step operation
type ProgressStartMsg struct {
	TotalSteps int
	Title      string
}

// ProgressStepMsg updates the current step being executed
type ProgressStepMsg struct {
	CurrentStep int
	TotalSteps  int
	StepTitle   string
	Tool        string
	ToolArgs    string
}

// ProgressUpdateMsg updates progress without changing the step
type ProgressUpdateMsg struct {
	Message string
}

// ProgressCompleteMsg signals completion of the multi-step operation
type ProgressCompleteMsg struct {
	Success bool
	Message string
}

// ProgressCanceledMsg signals that the operation was canceled
type ProgressCanceledMsg struct {
	AtStep int
}

// ProgressErrorMsg signals an error during the operation
type ProgressErrorMsg struct {
	AtStep int
	Error  error
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// NewStreamStartMsg creates a new StreamStartMsg with the current timestamp.
// This is a convenience constructor for initiating streaming responses.
func NewStreamStartMsg(messageID string) StreamStartMsg {
	return StreamStartMsg{
		MessageID: messageID,
		StartTime: time.Now(),
	}
}

// NewStreamTokenMsg creates a new StreamTokenMsg for delivering streaming content.
// The isFirst flag indicates whether this is the first token in the stream.
func NewStreamTokenMsg(messageID, token string, isFirst bool) StreamTokenMsg {
	return StreamTokenMsg{
		MessageID: messageID,
		Token:     token,
		IsFirst:   isFirst,
	}
}

// NewStreamCompleteMsg creates a new StreamCompleteMsg to signal stream completion.
// Includes optional statistics and error information.
func NewStreamCompleteMsg(messageID string, stats *model.Statistics, err error) StreamCompleteMsg {
	return StreamCompleteMsg{
		MessageID: messageID,
		Stats:     stats,
		Error:     err,
	}
}

// NewErrorMsg creates a new dismissible error message.
// Use this for non-critical errors that users can dismiss.
func NewErrorMsg(title, message string) ErrorMsg {
	return ErrorMsg{
		Title:       title,
		Message:     message,
		Dismissible: true,
	}
}

// NewErrorMsgWithSuggestions creates an error message with actionable suggestions.
// Use this when you can provide helpful guidance for resolving the error.
func NewErrorMsgWithSuggestions(title, message string, suggestions []string) ErrorMsg {
	return ErrorMsg{
		Title:       title,
		Message:     message,
		Suggestions: suggestions,
		Dismissible: true,
	}
}

// SmartErrorMsg creates an error message with auto-detected pattern matching and smart suggestions.
// This analyzes the error message and automatically provides relevant suggestions based on common error patterns.
// Use this as the default error creation method for better user experience.
func SmartErrorMsg(title, message string) ErrorMsg {
	// Use the ErrorPatternMatcher from components package for intelligent error analysis
	matcher := components.GetDefaultMatcher()

	// Try to match the error message against known patterns
	if matched := matcher.Match(message); matched != nil {
		// Pattern matched - use the enhanced error details
		return ErrorMsg{
			Title:       matched.GetTitle(),
			Message:     message,
			Suggestions: matched.GetSuggestions(),
			Dismissible: true,
		}
	}

	// No pattern matched - use the provided title
	return NewErrorMsg(title, message)
}

// detectErrorSuggestions analyzes an error message and returns relevant suggestions.
// This is a simplified version that avoids circular dependencies.
func detectErrorSuggestions(errMsg string) []string {
	errLower := strings.ToLower(errMsg)

	// Network/Connection errors
	if strings.Contains(errLower, "connection refused") ||
		strings.Contains(errLower, "dial tcp") ||
		strings.Contains(errLower, "no such host") {
		return []string{
			"Check your network connection",
			"Verify the service is running",
			"Try using offline mode if available",
		}
	}

	// Ollama-specific
	if strings.Contains(errLower, "ollama") || strings.Contains(errLower, "11434") {
		return []string{
			"Start Ollama: ollama serve",
			"Check if Ollama is installed: ollama --version",
			"Verify Ollama is running on localhost:11434",
		}
	}

	// Model not found
	if strings.Contains(errLower, "model not found") ||
		strings.Contains(errLower, "model does not exist") ||
		strings.Contains(errLower, "404") {
		return []string{
			"List available models: ollama list",
			"Pull the model: ollama pull <model-name>",
			"Check model name spelling",
		}
	}

	// Context exceeded
	if strings.Contains(errLower, "context") &&
		(strings.Contains(errLower, "exceeded") || strings.Contains(errLower, "too long")) {
		return []string{
			"Start new conversation: /new",
			"Clear history: /clear",
			"Use shorter messages",
		}
	}

	// Timeout
	if strings.Contains(errLower, "timeout") || strings.Contains(errLower, "timed out") {
		return []string{
			"Try again",
			"Use a smaller model",
			"Check server load",
		}
	}

	// Permission/Auth
	if strings.Contains(errLower, "permission denied") ||
		strings.Contains(errLower, "unauthorized") ||
		strings.Contains(errLower, "forbidden") {
		return []string{
			"Check file permissions",
			"Verify API key or credentials",
			"Grant necessary access rights",
		}
	}

	// Rate limit
	if strings.Contains(errLower, "rate limit") ||
		strings.Contains(errLower, "too many requests") ||
		strings.Contains(errLower, "429") {
		return []string{
			"Wait a moment and retry",
			"Switch to a local model",
			"Check your API quota",
		}
	}

	// No suggestions found
	return nil
}

// =============================================================================
// STREAMING OPTIMIZATION MESSAGES (Feature 4.2)
// =============================================================================

// StreamTickMsg is sent at 30fps during streaming to batch render tokens.
// This prevents excessive rendering (1000+ fps) which causes flicker and high CPU.
type StreamTickMsg struct {
	Time time.Time
}

// NewStreamTickMsg creates a streaming tick message.
func NewStreamTickMsg() StreamTickMsg {
	return StreamTickMsg{Time: time.Now()}
}
