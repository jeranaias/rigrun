// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// MESSAGE TYPES
// =============================================================================

// Message represents a conversation message for the agentic loop.
type Message struct {
	// Role is the message role: "user", "assistant", or "tool"
	Role string `json:"role"`

	// Content is the text content of the message
	Content string `json:"content"`

	// ToolCalls contains any tool calls requested by the assistant
	ToolCalls []ToolCallMessage `json:"tool_calls,omitempty"`

	// ToolCallID links a tool result back to its call (for role="tool")
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ToolCallMessage represents a tool call within a message.
type ToolCallMessage struct {
	// ID is a unique identifier for this tool call
	ID string `json:"id"`

	// Name is the name of the tool to call
	Name string `json:"name"`

	// Arguments contains the parameters for the tool
	Arguments map[string]interface{} `json:"arguments"`
}

// NewUserMessage creates a new user message.
func NewUserMessage(content string) Message {
	return Message{
		Role:    "user",
		Content: content,
	}
}

// NewAssistantMessage creates a new assistant message.
func NewAssistantMessage(content string) Message {
	return Message{
		Role:    "assistant",
		Content: content,
	}
}

// NewAssistantMessageWithToolCalls creates an assistant message with tool calls.
func NewAssistantMessageWithToolCalls(content string, calls []ToolCallMessage) Message {
	return Message{
		Role:      "assistant",
		Content:   content,
		ToolCalls: calls,
	}
}

// NewToolResultMessage creates a tool result message.
func NewToolResultMessage(toolCallID, content string) Message {
	return Message{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	}
}

// =============================================================================
// CHAT FUNCTION
// =============================================================================

// ChatFunc is the function signature for calling the LLM.
// It takes the conversation messages and returns:
// - The text response from the model
// - Any tool calls requested by the model
// - An error if the call failed
type ChatFunc func(messages []Message) (string, []ToolCallMessage, error)

// =============================================================================
// AGENTIC LOOP
// =============================================================================

// AgenticLoop manages the iterative conversation with tool execution.
type AgenticLoop struct {
	executor     *Executor
	maxIter      int
	conversation []Message
	onToolCall   func(call ToolCallMessage)
	onToolResult func(result Result)
	mu           sync.Mutex

	// State tracking for safety
	currentIteration    int           // Current iteration count
	consecutiveErrors   int           // Consecutive tool failure count
	maxConsecutiveErrs  int           // Max consecutive errors before stopping (default: 3)
	loopTimeout         time.Duration // Total timeout for entire loop
	loopStartTime       time.Time     // When the current run started
}

// DefaultMaxIterations is the default maximum number of loop iterations.
const DefaultMaxIterations = 25

// DefaultMaxConsecutiveErrors is the default max consecutive tool failures.
const DefaultMaxConsecutiveErrors = 3

// DefaultLoopTimeout is the default total loop timeout.
const DefaultLoopTimeout = 30 * time.Minute

// NewAgenticLoop creates a new agentic loop with the given executor.
// If maxIterations is 0 or negative, DefaultMaxIterations is used.
func NewAgenticLoop(executor *Executor, maxIterations int) *AgenticLoop {
	if maxIterations <= 0 {
		maxIterations = DefaultMaxIterations
	}
	return &AgenticLoop{
		executor:           executor,
		maxIter:            maxIterations,
		conversation:       make([]Message, 0),
		maxConsecutiveErrs: DefaultMaxConsecutiveErrors,
		loopTimeout:        DefaultLoopTimeout,
	}
}

// SetLoopTimeout sets the total timeout for the entire agentic loop.
func (l *AgenticLoop) SetLoopTimeout(timeout time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.loopTimeout = timeout
}

// SetMaxConsecutiveErrors sets the maximum consecutive errors before stopping.
func (l *AgenticLoop) SetMaxConsecutiveErrors(max int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if max > 0 {
		l.maxConsecutiveErrs = max
	}
}

// GetState returns the current loop state for monitoring.
func (l *AgenticLoop) GetState() (iteration int, consecutiveErrs int, elapsed time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	elapsed = time.Duration(0)
	if !l.loopStartTime.IsZero() {
		elapsed = time.Since(l.loopStartTime)
	}
	return l.currentIteration, l.consecutiveErrors, elapsed
}

// resetState resets the loop state for a new run.
func (l *AgenticLoop) resetState() {
	l.currentIteration = 0
	l.consecutiveErrors = 0
	l.loopStartTime = time.Time{}
}

// SetCallbacks sets the callback functions for tool events.
func (l *AgenticLoop) SetCallbacks(onCall func(ToolCallMessage), onResult func(Result)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onToolCall = onCall
	l.onToolResult = onResult
}

// AddMessage adds a message to the conversation history.
func (l *AgenticLoop) AddMessage(msg Message) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.conversation = append(l.conversation, msg)
}

// GetConversation returns a copy of the current conversation.
func (l *AgenticLoop) GetConversation() []Message {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]Message, len(l.conversation))
	copy(result, l.conversation)
	return result
}

// ClearConversation clears the conversation history.
func (l *AgenticLoop) ClearConversation() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.conversation = make([]Message, 0)
}

// SetMaxIterations updates the maximum iterations.
func (l *AgenticLoop) SetMaxIterations(max int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if max > 0 {
		l.maxIter = max
	}
}

// =============================================================================
// RUN LOOP
// =============================================================================

// ErrMaxIterationsReached is returned when the loop exceeds max iterations.
var ErrMaxIterationsReached = errors.New("maximum iterations reached")

// ErrContextCancelled is returned when the context is cancelled.
var ErrContextCancelled = errors.New("context cancelled")

// ErrLoopTimeout is returned when the total loop timeout is exceeded.
var ErrLoopTimeout = errors.New("agentic loop timeout exceeded")

// ErrConsecutiveToolFailures is returned when too many consecutive tools fail.
var ErrConsecutiveToolFailures = errors.New("too many consecutive tool failures")

// Run executes the agentic loop.
//
// The loop:
// 1. Calls chatFunc with the current conversation
// 2. If no tool calls are returned, returns the response (done)
// 3. Executes each tool call
// 4. Adds tool results to the conversation
// 5. Repeats until done or max iterations reached
//
// Safety features:
// - Maximum iteration limit (default: 25)
// - Total loop timeout (default: 30 minutes)
// - Consecutive error detection (default: 3 failures = stop)
//
// Returns the final assistant response.
func (l *AgenticLoop) Run(ctx context.Context, chatFunc ChatFunc) (string, error) {
	// Initialize state at the start of the run
	l.mu.Lock()
	l.currentIteration = 0
	l.consecutiveErrors = 0
	l.loopStartTime = time.Now()
	loopTimeout := l.loopTimeout
	maxConsecErrs := l.maxConsecutiveErrs
	l.mu.Unlock()

	// Ensure state is reset when we exit
	defer l.resetState()

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return "", ErrContextCancelled
		default:
		}

		// SAFETY CHECK: Total loop timeout
		l.mu.Lock()
		elapsed := time.Since(l.loopStartTime)
		l.mu.Unlock()
		if elapsed > loopTimeout {
			return "", fmt.Errorf("%w: exceeded %v", ErrLoopTimeout, loopTimeout)
		}

		// SAFETY CHECK: Iteration limit
		l.mu.Lock()
		l.currentIteration++
		iteration := l.currentIteration
		l.mu.Unlock()

		if iteration > l.maxIter {
			return "", fmt.Errorf("%w: %d", ErrMaxIterationsReached, l.maxIter)
		}

		// Get conversation snapshot
		l.mu.Lock()
		messages := make([]Message, len(l.conversation))
		copy(messages, l.conversation)
		l.mu.Unlock()

		// Call the LLM
		response, toolCalls, err := chatFunc(messages)
		if err != nil {
			return "", fmt.Errorf("chat function error: %w", err)
		}

		// If no tool calls, we're done - loop terminates successfully
		if len(toolCalls) == 0 {
			// Add final assistant message to conversation
			if response != "" {
				l.AddMessage(NewAssistantMessage(response))
			}
			return response, nil
		}

		// Add assistant message with tool calls to conversation
		l.AddMessage(NewAssistantMessageWithToolCalls(response, toolCalls))

		// Execute each tool call and track failures
		allFailed := true
		for _, call := range toolCalls {
			// Check context cancellation before each tool
			select {
			case <-ctx.Done():
				return "", ErrContextCancelled
			default:
			}

			// Notify callback
			l.mu.Lock()
			onCall := l.onToolCall
			l.mu.Unlock()
			if onCall != nil {
				onCall(call)
			}

			// Execute the tool
			toolCall := ToolCall{
				Name:   call.Name,
				Params: call.Arguments,
			}
			result := l.executor.Execute(ctx, toolCall)

			// Track if at least one tool succeeded
			if result.Success {
				allFailed = false
			}

			// Notify callback
			l.mu.Lock()
			onResult := l.onToolResult
			l.mu.Unlock()
			if onResult != nil {
				onResult(result)
			}

			// Format and add result to conversation
			resultContent := FormatToolResult(call, result)
			l.AddMessage(NewToolResultMessage(call.ID, resultContent))
		}

		// SAFETY CHECK: Track consecutive failures
		l.mu.Lock()
		if allFailed && len(toolCalls) > 0 {
			l.consecutiveErrors++
		} else {
			l.consecutiveErrors = 0
		}
		consecErrs := l.consecutiveErrors
		l.mu.Unlock()

		if consecErrs >= maxConsecErrs {
			return "", fmt.Errorf("%w: %d consecutive failures", ErrConsecutiveToolFailures, consecErrs)
		}

		// Continue loop to get next response
	}
}

// RunWithInitialMessage starts the loop with an initial user message.
func (l *AgenticLoop) RunWithInitialMessage(ctx context.Context, chatFunc ChatFunc, userMessage string) (string, error) {
	l.AddMessage(NewUserMessage(userMessage))
	return l.Run(ctx, chatFunc)
}

// =============================================================================
// TOOL CALL PARSING
// =============================================================================

// ParseToolCallsFromResponse extracts tool calls from a model response string.
// Supports multiple formats:
// - JSON format: {"name": "tool", "arguments": {...}}
// - OpenAI function_call format
// - Array of tool calls
func ParseToolCallsFromResponse(response string) ([]ToolCallMessage, error) {
	var calls []ToolCallMessage

	// Try parsing as a JSON array of tool calls
	if strings.HasPrefix(strings.TrimSpace(response), "[") {
		var arrayCalls []ToolCallMessage
		if err := json.Unmarshal([]byte(response), &arrayCalls); err == nil {
			return arrayCalls, nil
		}
	}

	// Try parsing as a single JSON object
	if strings.HasPrefix(strings.TrimSpace(response), "{") {
		// Try direct tool call format: {"name": "...", "arguments": {...}}
		var singleCall struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
			ID        string                 `json:"id,omitempty"`
		}
		if err := json.Unmarshal([]byte(response), &singleCall); err == nil && singleCall.Name != "" {
			id := singleCall.ID
			if id == "" {
				id = generateCallID()
			}
			calls = append(calls, ToolCallMessage{
				ID:        id,
				Name:      singleCall.Name,
				Arguments: singleCall.Arguments,
			})
			return calls, nil
		}

		// Try OpenAI function_call format: {"function_call": {"name": "...", "arguments": "..."}}
		var functionCall struct {
			FunctionCall struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function_call"`
		}
		if err := json.Unmarshal([]byte(response), &functionCall); err == nil && functionCall.FunctionCall.Name != "" {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(functionCall.FunctionCall.Arguments), &args); err != nil {
				args = make(map[string]interface{})
			}
			calls = append(calls, ToolCallMessage{
				ID:        generateCallID(),
				Name:      functionCall.FunctionCall.Name,
				Arguments: args,
			})
			return calls, nil
		}

		// Try tool_calls array format (OpenAI chat completion)
		var toolCallsWrapper struct {
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		}
		if err := json.Unmarshal([]byte(response), &toolCallsWrapper); err == nil && len(toolCallsWrapper.ToolCalls) > 0 {
			for _, tc := range toolCallsWrapper.ToolCalls {
				var args map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					args = make(map[string]interface{})
				}
				id := tc.ID
				if id == "" {
					id = generateCallID()
				}
				calls = append(calls, ToolCallMessage{
					ID:        id,
					Name:      tc.Function.Name,
					Arguments: args,
				})
			}
			return calls, nil
		}
	}

	// Try to find embedded JSON tool calls in the text
	calls = parseEmbeddedToolCalls(response)
	if len(calls) > 0 {
		return calls, nil
	}

	return calls, nil
}

// parseEmbeddedToolCalls finds JSON tool calls embedded in text.
func parseEmbeddedToolCalls(text string) []ToolCallMessage {
	var calls []ToolCallMessage

	// Find JSON objects that look like tool calls
	// Look for patterns like: {"name": "toolname", "arguments": {...}}
	depth := 0
	start := -1

	for i, c := range text {
		if c == '{' {
			if depth == 0 {
				start = i
			}
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 && start >= 0 {
				jsonStr := text[start : i+1]

				// Try to parse as tool call
				var call struct {
					Name      string                 `json:"name"`
					Arguments map[string]interface{} `json:"arguments"`
					Input     map[string]interface{} `json:"input"` // Alternative to arguments
					ID        string                 `json:"id,omitempty"`
				}
				if err := json.Unmarshal([]byte(jsonStr), &call); err == nil && call.Name != "" {
					args := call.Arguments
					if args == nil {
						args = call.Input
					}
					if args == nil {
						args = make(map[string]interface{})
					}
					id := call.ID
					if id == "" {
						id = generateCallID()
					}
					calls = append(calls, ToolCallMessage{
						ID:        id,
						Name:      call.Name,
						Arguments: args,
					})
				}

				start = -1
			}
		}
	}

	return calls
}

// generateCallID creates a unique ID for a tool call.
var callIDCounter int
var callIDMu sync.Mutex

func generateCallID() string {
	callIDMu.Lock()
	defer callIDMu.Unlock()
	callIDCounter++
	return fmt.Sprintf("call_%d", callIDCounter)
}

// =============================================================================
// RESULT FORMATTING
// =============================================================================

// FormatToolResult formats a tool execution result for sending back to the model.
func FormatToolResult(call ToolCallMessage, result Result) string {
	var sb strings.Builder

	if result.Success {
		sb.WriteString(fmt.Sprintf("Tool '%s' (id: %s) completed successfully.\n", call.Name, call.ID))
		if result.Output != "" {
			sb.WriteString("\nOutput:\n")
			sb.WriteString(result.Output)
		} else {
			sb.WriteString("\n(no output)")
		}
	} else {
		sb.WriteString(fmt.Sprintf("Tool '%s' (id: %s) failed.\n", call.Name, call.ID))
		if result.Error != "" {
			sb.WriteString("\nError:\n")
			sb.WriteString(result.Error)
		} else {
			sb.WriteString("\n(unknown error)")
		}
	}

	// Add metadata if available
	if result.Duration > 0 {
		sb.WriteString(fmt.Sprintf("\n\nDuration: %v", result.Duration))
	}
	if result.Truncated {
		sb.WriteString("\n(output was truncated)")
	}

	return sb.String()
}

// FormatToolResultJSON formats a tool result as JSON.
func FormatToolResultJSON(call ToolCallMessage, result Result) (string, error) {
	data := map[string]interface{}{
		"tool_call_id": call.ID,
		"name":         call.Name,
		"success":      result.Success,
	}

	if result.Success {
		data["output"] = result.Output
	} else {
		data["error"] = result.Error
	}

	if result.Truncated {
		data["truncated"] = true
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// =============================================================================
// AGENT EVENTS
// =============================================================================

// AgentEvent represents events that occur during the agentic loop.
type AgentEvent int

const (
	// EventToolCallRequested is emitted when a tool call is requested.
	EventToolCallRequested AgentEvent = iota

	// EventToolCallStarted is emitted when tool execution begins.
	EventToolCallStarted

	// EventToolCallCompleted is emitted when tool execution completes.
	EventToolCallCompleted

	// EventIterationComplete is emitted after each loop iteration.
	EventIterationComplete

	// EventLoopComplete is emitted when the loop finishes.
	EventLoopComplete

	// EventError is emitted when an error occurs.
	EventError
)

// AgentEventData contains data associated with an agent event.
type AgentEventData struct {
	Event     AgentEvent
	ToolCall  *ToolCallMessage
	Result    *Result
	Iteration int
	Error     error
	Response  string
}

// AgentEventCallback is called when agent events occur.
type AgentEventCallback func(data AgentEventData)

// RunWithEvents executes the agentic loop with event callbacks.
// Includes all safety features from Run(): iteration limit, timeout, consecutive error detection.
func (l *AgenticLoop) RunWithEvents(ctx context.Context, chatFunc ChatFunc, eventCb AgentEventCallback) (string, error) {
	// Initialize state at the start of the run
	l.mu.Lock()
	l.currentIteration = 0
	l.consecutiveErrors = 0
	l.loopStartTime = time.Now()
	loopTimeout := l.loopTimeout
	maxConsecErrs := l.maxConsecutiveErrs
	l.mu.Unlock()

	// Ensure state is reset when we exit
	defer l.resetState()

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			if eventCb != nil {
				eventCb(AgentEventData{
					Event: EventError,
					Error: ErrContextCancelled,
				})
			}
			return "", ErrContextCancelled
		default:
		}

		// SAFETY CHECK: Total loop timeout
		l.mu.Lock()
		elapsed := time.Since(l.loopStartTime)
		l.mu.Unlock()
		if elapsed > loopTimeout {
			err := fmt.Errorf("%w: exceeded %v", ErrLoopTimeout, loopTimeout)
			if eventCb != nil {
				eventCb(AgentEventData{
					Event: EventError,
					Error: err,
				})
			}
			return "", err
		}

		// SAFETY CHECK: Iteration limit
		l.mu.Lock()
		l.currentIteration++
		iteration := l.currentIteration
		l.mu.Unlock()

		if iteration > l.maxIter {
			err := fmt.Errorf("%w: %d", ErrMaxIterationsReached, l.maxIter)
			if eventCb != nil {
				eventCb(AgentEventData{
					Event: EventError,
					Error: err,
				})
			}
			return "", err
		}

		// Get conversation snapshot
		l.mu.Lock()
		messages := make([]Message, len(l.conversation))
		copy(messages, l.conversation)
		l.mu.Unlock()

		// Call the LLM
		response, toolCalls, err := chatFunc(messages)
		if err != nil {
			if eventCb != nil {
				eventCb(AgentEventData{
					Event: EventError,
					Error: err,
				})
			}
			return "", fmt.Errorf("chat function error: %w", err)
		}

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			if response != "" {
				l.AddMessage(NewAssistantMessage(response))
			}
			if eventCb != nil {
				eventCb(AgentEventData{
					Event:     EventLoopComplete,
					Iteration: iteration,
					Response:  response,
				})
			}
			return response, nil
		}

		// Add assistant message with tool calls
		l.AddMessage(NewAssistantMessageWithToolCalls(response, toolCalls))

		// Execute each tool call and track failures
		allFailed := true
		for i := range toolCalls {
			call := &toolCalls[i]

			// Check context cancellation
			select {
			case <-ctx.Done():
				if eventCb != nil {
					eventCb(AgentEventData{
						Event: EventError,
						Error: ErrContextCancelled,
					})
				}
				return "", ErrContextCancelled
			default:
			}

			// Emit tool call requested event
			if eventCb != nil {
				eventCb(AgentEventData{
					Event:    EventToolCallRequested,
					ToolCall: call,
				})
			}

			// Emit tool call started event
			if eventCb != nil {
				eventCb(AgentEventData{
					Event:    EventToolCallStarted,
					ToolCall: call,
				})
			}

			// Execute the tool
			toolCall := ToolCall{
				Name:   call.Name,
				Params: call.Arguments,
			}
			result := l.executor.Execute(ctx, toolCall)

			// Track if at least one tool succeeded
			if result.Success {
				allFailed = false
			}

			// Emit tool call completed event
			if eventCb != nil {
				eventCb(AgentEventData{
					Event:    EventToolCallCompleted,
					ToolCall: call,
					Result:   &result,
				})
			}

			// Add result to conversation
			resultContent := FormatToolResult(*call, result)
			l.AddMessage(NewToolResultMessage(call.ID, resultContent))
		}

		// SAFETY CHECK: Track consecutive failures
		l.mu.Lock()
		if allFailed && len(toolCalls) > 0 {
			l.consecutiveErrors++
		} else {
			l.consecutiveErrors = 0
		}
		consecErrs := l.consecutiveErrors
		l.mu.Unlock()

		if consecErrs >= maxConsecErrs {
			err := fmt.Errorf("%w: %d consecutive failures", ErrConsecutiveToolFailures, consecErrs)
			if eventCb != nil {
				eventCb(AgentEventData{
					Event: EventError,
					Error: err,
				})
			}
			return "", err
		}

		// Emit iteration complete event
		if eventCb != nil {
			eventCb(AgentEventData{
				Event:     EventIterationComplete,
				Iteration: iteration,
			})
		}
	}
}

// =============================================================================
// STREAMING SUPPORT
// =============================================================================

// StreamingChatFunc is like ChatFunc but supports streaming responses.
type StreamingChatFunc func(messages []Message, onChunk func(chunk string)) (string, []ToolCallMessage, error)

// RunStreaming executes the agentic loop with streaming text output.
// Includes all safety features from Run(): iteration limit, timeout, consecutive error detection.
func (l *AgenticLoop) RunStreaming(ctx context.Context, chatFunc StreamingChatFunc, onChunk func(chunk string)) (string, error) {
	// Initialize state at the start of the run
	l.mu.Lock()
	l.currentIteration = 0
	l.consecutiveErrors = 0
	l.loopStartTime = time.Now()
	loopTimeout := l.loopTimeout
	maxConsecErrs := l.maxConsecutiveErrs
	l.mu.Unlock()

	// Ensure state is reset when we exit
	defer l.resetState()

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return "", ErrContextCancelled
		default:
		}

		// SAFETY CHECK: Total loop timeout
		l.mu.Lock()
		elapsed := time.Since(l.loopStartTime)
		l.mu.Unlock()
		if elapsed > loopTimeout {
			return "", fmt.Errorf("%w: exceeded %v", ErrLoopTimeout, loopTimeout)
		}

		// SAFETY CHECK: Iteration limit
		l.mu.Lock()
		l.currentIteration++
		iteration := l.currentIteration
		l.mu.Unlock()

		if iteration > l.maxIter {
			return "", fmt.Errorf("%w: %d", ErrMaxIterationsReached, l.maxIter)
		}

		// Get conversation snapshot
		l.mu.Lock()
		messages := make([]Message, len(l.conversation))
		copy(messages, l.conversation)
		l.mu.Unlock()

		// Call the LLM with streaming
		response, toolCalls, err := chatFunc(messages, onChunk)
		if err != nil {
			return "", fmt.Errorf("chat function error: %w", err)
		}

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			if response != "" {
				l.AddMessage(NewAssistantMessage(response))
			}
			return response, nil
		}

		// Add assistant message with tool calls
		l.AddMessage(NewAssistantMessageWithToolCalls(response, toolCalls))

		// Execute each tool call and track failures
		allFailed := true
		for _, call := range toolCalls {
			// Check context cancellation
			select {
			case <-ctx.Done():
				return "", ErrContextCancelled
			default:
			}

			// Notify callback
			l.mu.Lock()
			onCall := l.onToolCall
			l.mu.Unlock()
			if onCall != nil {
				onCall(call)
			}

			// Execute the tool
			toolCall := ToolCall{
				Name:   call.Name,
				Params: call.Arguments,
			}
			result := l.executor.Execute(ctx, toolCall)

			// Track if at least one tool succeeded
			if result.Success {
				allFailed = false
			}

			// Notify callback
			l.mu.Lock()
			onResult := l.onToolResult
			l.mu.Unlock()
			if onResult != nil {
				onResult(result)
			}

			// Add result to conversation
			resultContent := FormatToolResult(call, result)
			l.AddMessage(NewToolResultMessage(call.ID, resultContent))
		}

		// SAFETY CHECK: Track consecutive failures
		l.mu.Lock()
		if allFailed && len(toolCalls) > 0 {
			l.consecutiveErrors++
		} else {
			l.consecutiveErrors = 0
		}
		consecErrs := l.consecutiveErrors
		l.mu.Unlock()

		if consecErrs >= maxConsecErrs {
			return "", fmt.Errorf("%w: %d consecutive failures", ErrConsecutiveToolFailures, consecErrs)
		}
	}
}
