// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
package tools

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// PERFORMANCE: Pre-compiled regex (compiled once at startup)
// =============================================================================

var (
	// Tool call parsing patterns
	jsonToolCallRegex  = regexp.MustCompile(`\{[^{}]*"name"\s*:\s*"([^"]+)"[^{}]*(?:"parameters"|"input")\s*:\s*(\{[^{}]*\})[^{}]*\}`)
	functionCallRegex  = regexp.MustCompile(`\b([A-Z][a-zA-Z]*)\s*\(\s*([^)]*)\s*\)`)
	keyValuePairsRegex = regexp.MustCompile(`(\w+)\s*=\s*(?:"([^"]*)"|\x60([^\x60]*)\x60|'([^']*)'|(\S+))`)
)

// =============================================================================
// PERMISSION CALLBACK
// =============================================================================

// PermissionCallback is called to check if a tool execution should be allowed.
// Returns true if the tool call is approved.
type PermissionCallback func(tool *Tool, params map[string]interface{}) bool

// AllowAllCallback returns a permission callback that allows all executions.
func AllowAllCallback() PermissionCallback {
	return func(tool *Tool, params map[string]interface{}) bool {
		return true
	}
}

// DenyAllCallback returns a permission callback that denies all executions.
func DenyAllCallback() PermissionCallback {
	return func(tool *Tool, params map[string]interface{}) bool {
		return false
	}
}

// ConfirmHighRiskCallback returns a callback that denies high/critical risk tools.
func ConfirmHighRiskCallback() PermissionCallback {
	return func(tool *Tool, params map[string]interface{}) bool {
		return tool.RiskLevel < RiskHigh
	}
}

// =============================================================================
// EXECUTION RECORD
// =============================================================================

// ExecutionRecord tracks the result of a tool execution for audit purposes.
type ExecutionRecord struct {
	// ToolName is the name of the executed tool
	ToolName string

	// Params are the parameters passed to the tool
	Params map[string]interface{}

	// Result is the outcome of the execution
	Result Result

	// Timestamp is when the execution started
	Timestamp time.Time

	// Duration is how long the execution took
	Duration time.Duration

	// Approved indicates whether the execution was approved
	Approved bool
}

// =============================================================================
// EXECUTOR
// =============================================================================

// Executor orchestrates tool execution with permission handling and audit logging.
type Executor struct {
	registry     *Registry
	permissionCb PermissionCallback
	autoApprove  PermissionLevel // Auto-approve up to this level
	history      []ExecutionRecord
	mu           sync.Mutex

	// Configuration
	workDir       string
	maxOutputSize int           // Max output size in bytes (default: 30000)
	maxTimeout    time.Duration // Maximum execution timeout
}

// NewExecutor creates a new tool executor with the given registry.
func NewExecutor(registry *Registry) *Executor {
	return &Executor{
		registry:      registry,
		permissionCb:  ConfirmHighRiskCallback(), // IL5: Deny high/critical risk tools by default
		autoApprove:   PermissionAsk,             // Don't auto-approve anything by default
		history:       make([]ExecutionRecord, 0),
		workDir:       ".",
		maxOutputSize: 30000,
		maxTimeout:    10 * time.Minute,
	}
}

// SetPermissionCallback sets the callback function for permission checks.
func (e *Executor) SetPermissionCallback(cb PermissionCallback) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.permissionCb = cb
}

// SetAutoApproveLevel sets the permission level up to which tools are auto-approved.
// Tools with permission level <= this level will be auto-approved.
func (e *Executor) SetAutoApproveLevel(level PermissionLevel) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.autoApprove = level
}

// SetWorkDir updates the working directory for tool execution.
func (e *Executor) SetWorkDir(dir string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.workDir = dir
}

// GetWorkDir returns the current working directory.
func (e *Executor) GetWorkDir() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.workDir
}

// History returns a copy of the execution history.
func (e *Executor) History() []ExecutionRecord {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Return a copy to prevent external modification
	result := make([]ExecutionRecord, len(e.history))
	copy(result, e.history)
	return result
}

// ClearHistory clears the execution history.
func (e *Executor) ClearHistory() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = make([]ExecutionRecord, 0)
}

// Registry returns the tool registry.
func (e *Executor) Registry() *Registry {
	return e.registry
}

// =============================================================================
// EXECUTION
// =============================================================================

// TOOLS: Proper timeout, validation, and resource cleanup
// DefaultToolTimeout is the default timeout applied when context has no deadline
const DefaultToolTimeout = 30 * time.Second

// Execute runs a tool call and returns the result.
// It handles permission checking, execution, timeout handling, and history recording.
func (e *Executor) Execute(ctx context.Context, call ToolCall) Result {
	start := time.Now()

	// Look up the tool in the registry
	tool := e.registry.Get(call.Name)
	if tool == nil {
		return Result{
			Success:  false,
			Error:    "unknown tool: " + call.Name,
			Duration: time.Since(start),
		}
	}

	// Check permission level
	approved := e.checkPermission(tool, call.Params)

	// Record the execution attempt
	record := ExecutionRecord{
		ToolName:  call.Name,
		Params:    call.Params,
		Timestamp: start,
		Approved:  approved,
	}

	// If not approved, record and return early
	if !approved {
		record.Duration = time.Since(start)
		record.Result = Result{
			Success:  false,
			Error:    "permission denied for tool: " + call.Name,
			Duration: record.Duration,
		}

		e.addToHistory(record)
		return record.Result
	}

	// Validate parameters using comprehensive validation
	if err := e.validateParams(tool, call.Params); err != nil {
		result := Result{
			Success:  false,
			Error:    "parameter validation failed: " + err.Error(),
			Duration: time.Since(start),
		}
		record.Duration = result.Duration
		record.Result = result
		e.addToHistory(record)
		return result
	}

	// TOOLS: Add timeout if not in context
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultToolTimeout)
		defer cancel()
	}

	// Execute with timeout using goroutine pattern to ensure cleanup
	resultCh := make(chan Result, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := tool.Executor.Execute(ctx, call.Params)
		if err != nil {
			errCh <- err
		} else {
			resultCh <- result
		}
	}()

	var result Result
	select {
	case result = <-resultCh:
		// Execution completed successfully
	case err := <-errCh:
		result = Result{
			Success: false,
			Error:   err.Error(),
		}
	case <-ctx.Done():
		result = Result{
			Success: false,
			Error:   "tool execution timed out: " + ctx.Err().Error(),
		}
	}

	result.Duration = time.Since(start)

	// Truncate output if too large
	if len(result.Output) > e.maxOutputSize {
		result.Output = result.Output[:e.maxOutputSize]
		result.Truncated = true
	}

	// Record the execution
	record.Duration = result.Duration
	record.Result = result
	e.addToHistory(record)

	return result
}

// ExecuteBatch executes multiple tool calls and returns their results.
func (e *Executor) ExecuteBatch(ctx context.Context, calls []ToolCall) []Result {
	results := make([]Result, len(calls))
	for i, call := range calls {
		results[i] = e.Execute(ctx, call)
	}
	return results
}

// checkPermission determines if a tool execution should be allowed.
// Uses GetPermissionWithParams for context-aware permission checking that considers
// path-based security rules in addition to tool-level permissions.
func (e *Executor) checkPermission(tool *Tool, params map[string]interface{}) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Use GetPermissionWithParams for context-aware permission checking
	// This ensures PermissionFunc (path-based security) is evaluated with actual params
	toolPermission := e.registry.GetPermissionWithParams(tool.Name, params)

	// If tool permission is <= autoApprove level, auto-approve
	if toolPermission <= e.autoApprove {
		return true
	}

	// Otherwise, call the permission callback
	if e.permissionCb != nil {
		return e.permissionCb(tool, params)
	}

	return false
}

// addToHistory adds an execution record to the history.
func (e *Executor) addToHistory(record ExecutionRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Limit history size to prevent unbounded growth
	const maxHistorySize = 1000
	if len(e.history) >= maxHistorySize {
		// Remove oldest entries
		e.history = e.history[len(e.history)-maxHistorySize+1:]
	}

	e.history = append(e.history, record)
}

// validateParams validates tool parameters against the schema.
func (e *Executor) validateParams(tool *Tool, params map[string]interface{}) error {
	for _, param := range tool.Schema.Parameters {
		val, exists := params[param.Name]

		// Check required parameters
		if param.Required && (!exists || val == nil) {
			return &ValidationError{
				Param:   param.Name,
				Message: "required parameter is missing",
			}
		}

		// Skip validation for optional parameters that aren't provided
		if !exists || val == nil {
			continue
		}

		// Type validation
		if err := e.validateType(param, val); err != nil {
			return err
		}
	}

	return nil
}

// validateType validates a parameter value against its expected type.
func (e *Executor) validateType(param Parameter, val interface{}) error {
	switch param.Type {
	case "string":
		if _, ok := val.(string); !ok {
			return &ValidationError{
				Param:   param.Name,
				Message: "expected string",
			}
		}
	case "number":
		switch val.(type) {
		case int, int64, float64:
			// OK
		default:
			return &ValidationError{
				Param:   param.Name,
				Message: "expected number",
			}
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return &ValidationError{
				Param:   param.Name,
				Message: "expected boolean",
			}
		}
	case "array":
		if _, ok := val.([]interface{}); !ok {
			return &ValidationError{
				Param:   param.Name,
				Message: "expected array",
			}
		}
	}

	return nil
}

// =============================================================================
// STANDALONE VALIDATION FUNCTION
// =============================================================================

// TOOLS: Proper timeout, validation, and resource cleanup
// ValidateToolArgs validates tool arguments against a schema before execution.
// This is a standalone function that can be used by any tool for input validation.
// It performs:
// - Required parameter checking
// - Type validation
// - Bounds checking for numeric values
// - String length validation
func ValidateToolArgs(schema *Schema, args map[string]interface{}) error {
	if schema == nil {
		return nil
	}

	// Check required parameters
	for _, param := range schema.Parameters {
		val, exists := args[param.Name]

		// Required parameter check
		if param.Required && (!exists || val == nil) {
			return &ValidationError{
				Param:   param.Name,
				Message: "missing required argument",
			}
		}

		// Skip validation for optional parameters that aren't provided
		if !exists || val == nil {
			continue
		}

		// Type checking
		if err := validateArgType(param, val); err != nil {
			return err
		}

		// Bounds checking for numbers
		if param.Type == "number" {
			if err := validateNumericBounds(param, val); err != nil {
				return err
			}
		}

		// String length validation
		if param.Type == "string" {
			if str, ok := val.(string); ok {
				if err := validateStringLength(param, str); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// validateArgType validates the type of an argument.
func validateArgType(param Parameter, val interface{}) error {
	switch param.Type {
	case "string":
		if _, ok := val.(string); !ok {
			return &ValidationError{
				Param:   param.Name,
				Message: "expected string type",
			}
		}
	case "number":
		switch val.(type) {
		case int, int64, int32, float64, float32:
			// OK
		default:
			return &ValidationError{
				Param:   param.Name,
				Message: "expected number type",
			}
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return &ValidationError{
				Param:   param.Name,
				Message: "expected boolean type",
			}
		}
	case "array":
		if _, ok := val.([]interface{}); !ok {
			return &ValidationError{
				Param:   param.Name,
				Message: "expected array type",
			}
		}
	case "object":
		if _, ok := val.(map[string]interface{}); !ok {
			return &ValidationError{
				Param:   param.Name,
				Message: "expected object type",
			}
		}
	}
	return nil
}

// validateNumericBounds checks if a numeric value is within acceptable bounds.
func validateNumericBounds(param Parameter, val interface{}) error {
	var numVal float64

	switch v := val.(type) {
	case int:
		numVal = float64(v)
	case int32:
		numVal = float64(v)
	case int64:
		numVal = float64(v)
	case float32:
		numVal = float64(v)
	case float64:
		numVal = v
	default:
		return nil // Already validated type
	}

	// Check for reasonable bounds to prevent DoS
	const maxReasonableValue = 1e15
	const minReasonableValue = -1e15

	if numVal > maxReasonableValue || numVal < minReasonableValue {
		return &ValidationError{
			Param:   param.Name,
			Message: "numeric value out of reasonable bounds",
		}
	}

	return nil
}

// validateStringLength checks if a string is within acceptable length bounds.
func validateStringLength(param Parameter, val string) error {
	// Prevent extremely long strings that could cause memory issues
	const maxStringLength = 10 * 1024 * 1024 // 10MB

	if len(val) > maxStringLength {
		return &ValidationError{
			Param:   param.Name,
			Message: "string value exceeds maximum length",
		}
	}

	return nil
}

// =============================================================================
// BUBBLE TEA INTEGRATION
// =============================================================================

// ExecuteWithPermission executes a tool, prompting for permission if needed.
// Returns a tea.Cmd that handles the execution flow.
// Uses NeedsPermissionWithParams for context-aware permission checking that considers
// path-based security rules in addition to tool-level permissions.
func (e *Executor) ExecuteWithPermission(call *ToolCall) tea.Cmd {
	tool := e.registry.Get(call.Name)
	if tool == nil {
		return func() tea.Msg {
			return ToolCompleteMsg{
				Call: call,
				Result: Result{
					Success: false,
					Error:   "unknown tool: " + call.Name,
				},
			}
		}
	}

	// Check if permission is needed using context-aware method with params
	// This ensures path-based security rules are evaluated (e.g., sensitive file access)
	if e.registry.NeedsPermissionWithParams(call.Name, call.Params) {
		return func() tea.Msg {
			return ToolPermissionRequestMsg{
				Call: call,
				Tool: tool,
			}
		}
	}

	// Execute directly
	return e.executeAsync(call)
}

// executeAsync runs the tool asynchronously.
func (e *Executor) executeAsync(call *ToolCall) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), e.maxTimeout)
		defer cancel()

		result := e.Execute(ctx, ToolCall{Name: call.Name, Params: call.Params})
		return ToolCompleteMsg{
			Call:   call,
			Result: result,
		}
	}
}

// =============================================================================
// TOOL CALL PARSING
// =============================================================================

// ParseToolCalls extracts tool calls from LLM response text.
// Supports various formats:
// - JSON tool_use blocks
// - Markdown code blocks with tool invocations
// - Direct function call syntax
func ParseToolCalls(text string) []*ToolCall {
	var calls []*ToolCall

	// Try parsing JSON tool_use format
	if jsonCalls := parseJSONToolCalls(text); len(jsonCalls) > 0 {
		calls = append(calls, jsonCalls...)
	}

	// Try parsing function call format: toolName(param1="value1", param2="value2")
	if fnCalls := parseFunctionCalls(text); len(fnCalls) > 0 {
		calls = append(calls, fnCalls...)
	}

	return calls
}

// parseJSONToolCalls parses JSON-formatted tool calls.
// Uses pre-compiled jsonToolCallRegex.
func parseJSONToolCalls(text string) []*ToolCall {
	var calls []*ToolCall

	// Look for JSON objects with "name" and "parameters" or "input"
	// Pattern: {"name": "...", "parameters": {...}}
	matches := jsonToolCallRegex.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			name := match[1]
			paramsJSON := match[2]

			var params map[string]interface{}
			if err := json.Unmarshal([]byte(paramsJSON), &params); err == nil {
				calls = append(calls, &ToolCall{
					Name:   name,
					Params: params,
				})
			}
		}
	}

	return calls
}

// parseFunctionCalls parses function-style tool calls.
// Uses pre-compiled functionCallRegex.
func parseFunctionCalls(text string) []*ToolCall {
	var calls []*ToolCall

	// Pattern: ToolName(key="value", ...)
	matches := functionCallRegex.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			name := match[1]
			argsStr := match[2]

			// Parse key="value" pairs
			params := parseKeyValuePairs(argsStr)
			if len(params) > 0 {
				calls = append(calls, &ToolCall{
					Name:   name,
					Params: params,
				})
			}
		}
	}

	return calls
}

// parseKeyValuePairs parses key="value" or key=value pairs.
// Uses pre-compiled keyValuePairsRegex.
func parseKeyValuePairs(s string) map[string]interface{} {
	params := make(map[string]interface{})

	// Match key="value" or key=value
	matches := keyValuePairsRegex.FindAllStringSubmatch(s, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			key := match[1]
			var value string

			// Find the non-empty capture group
			for i := 2; i < len(match); i++ {
				if match[i] != "" {
					value = match[i]
					break
				}
			}

			// Try to parse as number or boolean
			params[key] = parseValue(value)
		}
	}

	return params
}

// parseValue converts a string value to its appropriate type.
func parseValue(s string) interface{} {
	s = strings.TrimSpace(s)

	// Boolean
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Number (integer)
	if isInteger(s) {
		var n int64
		for _, c := range s {
			n = n*10 + int64(c-'0')
		}
		return n
	}

	// String
	return s
}

// isInteger checks if a string represents an integer.
func isInteger(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// =============================================================================
// BUBBLE TEA MESSAGES
// =============================================================================

// ToolPermissionRequestMsg requests user permission for a tool.
type ToolPermissionRequestMsg struct {
	Call *ToolCall
	Tool *Tool
}

// ToolPermissionResponseMsg is the user's response to a permission request.
type ToolPermissionResponseMsg struct {
	Call        *ToolCall
	Allowed     bool
	AlwaysAllow bool // "Allow Always" was selected
}

// ToolExecutingMsg indicates a tool is currently executing.
type ToolExecutingMsg struct {
	Call *ToolCall
	Tool *Tool
}

// ToolCompleteMsg indicates a tool has finished executing.
type ToolCompleteMsg struct {
	Call   *ToolCall
	Result Result
}

// =============================================================================
// VALIDATION ERRORS
// =============================================================================

// ValidationError represents a parameter validation error.
type ValidationError struct {
	Param   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Param + ": " + e.Message
}

// =============================================================================
// EXECUTION STATISTICS
// =============================================================================

// ExecutionStats provides statistics about tool executions.
type ExecutionStats struct {
	TotalExecutions int
	Successful      int
	Failed          int
	Denied          int
	TotalDuration   time.Duration
	AvgDuration     time.Duration
}

// Stats returns statistics about the execution history.
func (e *Executor) Stats() ExecutionStats {
	e.mu.Lock()
	defer e.mu.Unlock()

	stats := ExecutionStats{}
	stats.TotalExecutions = len(e.history)

	for _, record := range e.history {
		if !record.Approved {
			stats.Denied++
		} else if record.Result.Success {
			stats.Successful++
		} else {
			stats.Failed++
		}
		stats.TotalDuration += record.Duration
	}

	if stats.TotalExecutions > 0 {
		stats.AvgDuration = stats.TotalDuration / time.Duration(stats.TotalExecutions)
	}

	return stats
}
