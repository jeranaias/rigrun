// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
package chat

import (
	"context"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeranaias/rigrun-tui/internal/model"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
)

// =============================================================================
// COMMAND CREATORS
// =============================================================================

// StreamToModel creates a command that streams from Ollama and sends messages.
// This is the main entry point for starting a streaming chat.
func StreamToModel(client *ollama.Client, modelName string, messages []ollama.Message, messageID string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return StreamErrorMsg{
				MessageID: messageID,
				Error:     ollama.ErrNotRunning,
			}
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stats := model.NewStatistics()
		isFirst := true
		// PERFORMANCE: strings.Builder avoids quadratic allocations
		var lastContent strings.Builder

		err := client.ChatStream(ctx, modelName, messages, func(chunk ollama.StreamChunk) {
			if chunk.Error != nil {
				return
			}

			// Accumulate content
			if chunk.Content != "" {
				lastContent.WriteString(chunk.Content)
				if isFirst {
					stats.RecordFirstToken()
					isFirst = false
				}
			}

			// Check for completion
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

// CheckOllamaCmd creates a command that checks if Ollama is running.
func CheckOllamaCmd(client *ollama.Client) tea.Cmd {
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

// ListModelsCmd creates a command that lists available Ollama models.
func ListModelsCmd(client *ollama.Client) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return OllamaModelsMsg{Error: ollama.ErrNotRunning}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		models, err := client.ListModels(ctx)
		return OllamaModelsMsg{
			Models: models,
			Error:  err,
		}
	}
}

// SwitchModelCmd creates a command to switch the model.
func SwitchModelCmd(client *ollama.Client, modelName string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return OllamaModelSwitchedMsg{Model: modelName, Error: ollama.ErrNotRunning}
		}

		// Verify model exists
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := client.GetModel(ctx, modelName)
		if err != nil {
			return OllamaModelSwitchedMsg{Model: modelName, Error: err}
		}

		return OllamaModelSwitchedMsg{Model: modelName, Error: nil}
	}
}

// =============================================================================
// STREAMING STATE MACHINE
// =============================================================================

// StreamingState tracks the state of a streaming operation.
// All fields are protected by mu for thread-safe access from multiple goroutines.
// IMPORTANT: This struct should be used as a pointer (*StreamingState) to prevent
// copying the mutex when passed between goroutines.
type StreamingState struct {
	mu         sync.RWMutex
	messageID  string
	startTime  time.Time
	firstToken time.Time
	tokenCount int
	// PERFORMANCE: strings.Builder avoids quadratic allocations
	content    strings.Builder
	isComplete bool
	err        error
	cancelFunc context.CancelFunc
}

// NewStreamingState creates a new streaming state.
// Always returns a pointer to ensure the mutex is not copied.
func NewStreamingState(messageID string) *StreamingState {
	return &StreamingState{
		messageID: messageID,
		startTime: time.Now(),
	}
}

// GetMessageID returns the message ID in a thread-safe manner.
func (s *StreamingState) GetMessageID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.messageID
}

// GetContent returns the accumulated content in a thread-safe manner.
func (s *StreamingState) GetContent() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.content.String()
}

// AppendContent appends content to the stream in a thread-safe manner.
func (s *StreamingState) AppendContent(content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.content.WriteString(content)
	s.tokenCount++
}

// GetTokenCount returns the token count in a thread-safe manner.
func (s *StreamingState) GetTokenCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tokenCount
}

// IsComplete returns whether streaming is complete in a thread-safe manner.
func (s *StreamingState) IsComplete() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isComplete
}

// SetComplete marks the stream as complete in a thread-safe manner.
func (s *StreamingState) SetComplete() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isComplete = true
}

// GetError returns the error in a thread-safe manner.
func (s *StreamingState) GetError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

// SetError sets the error in a thread-safe manner.
func (s *StreamingState) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

// RecordFirstToken records the time of the first token in a thread-safe manner.
func (s *StreamingState) RecordFirstToken() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.firstToken.IsZero() {
		s.firstToken = time.Now()
	}
}

// TTFT returns the time to first token in a thread-safe manner.
func (s *StreamingState) TTFT() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.firstToken.IsZero() {
		return 0
	}
	return s.firstToken.Sub(s.startTime)
}

// Elapsed returns the elapsed time since start in a thread-safe manner.
func (s *StreamingState) Elapsed() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.startTime)
}

// SetCancelFunc sets the cancel function in a thread-safe manner.
func (s *StreamingState) SetCancelFunc(fn context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelFunc = fn
}

// Cancel cancels the streaming operation in a thread-safe manner.
// Safe to call multiple times or with no cancel function set.
func (s *StreamingState) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil // Prevent double-cancel
	}
}

// =============================================================================
// ASYNC STREAMING
// =============================================================================

// AsyncStreamResult is returned by the async streaming command.
type AsyncStreamResult struct {
	MessageID string
	Chunk     *StreamTokenMsg
	Complete  *StreamCompleteMsg
	Error     *StreamErrorMsg
}

// AsyncStreamCmd creates an async streaming command that sends tokens via a channel.
// This is more suitable for Bubble Tea's architecture.
func AsyncStreamCmd(client *ollama.Client, modelName string, messages []ollama.Message, messageID string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return StreamErrorMsg{
				MessageID: messageID,
				Error:     ollama.ErrNotRunning,
			}
		}

		// For Bubble Tea, we need to return immediately and then send subsequent
		// messages through the program. This requires a different approach.
		// We'll return the start message and handle the rest via a separate mechanism.

		return StreamStartMsg{
			MessageID: messageID,
			StartTime: time.Now(),
		}
	}
}

// =============================================================================
// PROGRAM RUNNER FOR STREAMING
// =============================================================================

// StreamRunner manages streaming for a Bubble Tea program.
type StreamRunner struct {
	program *tea.Program
	client  *ollama.Client
}

// NewStreamRunner creates a new stream runner.
func NewStreamRunner(program *tea.Program, client *ollama.Client) *StreamRunner {
	return &StreamRunner{
		program: program,
		client:  client,
	}
}

// Run executes a streaming chat and sends messages to the program.
func (r *StreamRunner) Run(ctx context.Context, modelName string, messages []ollama.Message, messageID string) {
	r.RunWithTools(ctx, modelName, messages, nil, messageID)
}

// RunWithTools executes a streaming chat with optional tool support.
func (r *StreamRunner) RunWithTools(ctx context.Context, modelName string, messages []ollama.Message, tools []ollama.Tool, messageID string) {
	if r.client == nil || r.program == nil {
		r.program.Send(StreamErrorMsg{
			MessageID: messageID,
			Error:     ollama.ErrNotRunning,
		})
		return
	}

	// Send start message
	r.program.Send(StreamStartMsg{
		MessageID: messageID,
		StartTime: time.Now(),
	})

	stats := model.NewStatistics()
	isFirst := true
	var accumulatedToolCalls []ollama.ToolCall
	completeSent := false

	// Choose streaming method based on whether tools are provided
	var streamErr error
	if len(tools) > 0 {
		streamErr = r.client.ChatStreamWithTools(ctx, modelName, messages, tools, func(chunk ollama.StreamChunk) {
			if chunk.Error != nil {
				r.program.Send(StreamErrorMsg{
					MessageID: messageID,
					Error:     chunk.Error,
				})
				return
			}

			// Handle tool calls
			if len(chunk.ToolCalls) > 0 {
				accumulatedToolCalls = append(accumulatedToolCalls, chunk.ToolCalls...)
				for _, tc := range chunk.ToolCalls {
					r.program.Send(ToolCallRequestedMsg{
						MessageID: messageID,
						ToolName:  tc.Function.Name,
						ToolID:    messageID + "-" + tc.Function.Name, // Generate a unique ID
						Arguments: tc.Function.Arguments,
					})
				}
			}

			// Send token message
			if chunk.Content != "" {
				r.program.Send(StreamTokenMsg{
					MessageID: messageID,
					Token:     chunk.Content,
					IsFirst:   isFirst,
				})

				if isFirst {
					stats.RecordFirstToken()
					isFirst = false
				}
			}

			// Handle completion
			if chunk.Done {
				stats.Finalize(chunk.CompletionTokens)
				r.program.Send(StreamCompleteMsg{
					MessageID: messageID,
					Stats:     stats,
				})
				completeSent = true
			}
		})
	} else {
		streamErr = r.client.ChatStream(ctx, modelName, messages, func(chunk ollama.StreamChunk) {
			if chunk.Error != nil {
				r.program.Send(StreamErrorMsg{
					MessageID: messageID,
					Error:     chunk.Error,
				})
				return
			}

			// Send token message
			if chunk.Content != "" {
				r.program.Send(StreamTokenMsg{
					MessageID: messageID,
					Token:     chunk.Content,
					IsFirst:   isFirst,
				})

				if isFirst {
					stats.RecordFirstToken()
					isFirst = false
				}
			}

			// Handle completion
			if chunk.Done {
				stats.Finalize(chunk.CompletionTokens)
				r.program.Send(StreamCompleteMsg{
					MessageID: messageID,
					Stats:     stats,
				})
				completeSent = true
			}
		})
	}

	// Only send error if completion wasn't already sent
	if streamErr != nil && !completeSent {
		r.program.Send(StreamErrorMsg{
			MessageID: messageID,
			Error:     streamErr,
		})
	}
}

// =============================================================================
// SPINNER TICK
// =============================================================================

// SpinnerTickCmd creates a command that ticks the spinner.
func SpinnerTickCmd() tea.Cmd {
	return tea.Tick(time.Second/12, func(t time.Time) tea.Msg {
		return SpinnerTickMsg{Time: t}
	})
}

// =============================================================================
// THINKING ANIMATION
// =============================================================================

// ThinkingTickCmd creates a command that updates the thinking animation.
func ThinkingTickCmd(startTime time.Time) tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return ThinkingUpdateMsg{
			Elapsed: t.Sub(startTime),
		}
	})
}

// =============================================================================
// VIEWPORT COMMANDS
// =============================================================================

// ScrollToBottomCmd scrolls the viewport to the bottom.
func ScrollToBottomCmd() tea.Cmd {
	return func() tea.Msg {
		return ViewportScrollToBottomMsg{}
	}
}

// ScrollToTopCmd scrolls the viewport to the top.
func ScrollToTopCmd() tea.Cmd {
	return func() tea.Msg {
		return ViewportScrollToTopMsg{}
	}
}

// =============================================================================
// ERROR HANDLING
// =============================================================================

// HandleOllamaError converts an Ollama error to an appropriate message.
// Uses smart error pattern matching to provide contextual suggestions.
func HandleOllamaError(err error) tea.Msg {
	if ollama.IsNotRunning(err) {
		return SmartErrorMsg("Ollama Not Running", "Cannot connect to Ollama service.")
	}

	if ollama.IsModelNotFound(err) {
		return SmartErrorMsg("Model Not Found", err.Error())
	}

	if ollama.IsTimeout(err) {
		return SmartErrorMsg("Request Timeout", "The request took too long to complete.")
	}

	// Generic error - use smart pattern matching
	return SmartErrorMsg("Error", err.Error())
}

// =============================================================================
// BATCH COMMANDS
// =============================================================================

// InitCommands returns the commands to run on initialization.
func InitCommands(client *ollama.Client) tea.Cmd {
	return tea.Batch(
		CheckOllamaCmd(client),
	)
}
