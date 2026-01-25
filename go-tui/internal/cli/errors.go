// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// errors.go - Unified error handling for all CLI commands in rigrun.
//
// This file eliminates inconsistent error handling patterns across
// 23+ command files. Some commands returned errors, some printed and
// returned nil, some did both inconsistently.
//
// STANDARDIZED PATTERN:
//   - ALWAYS return errors (never just print and return nil)
//   - Let the caller decide how to display errors
//   - Use structured error types for better error handling
//   - Provide helpers for common error scenarios
//
// ERROR HANDLING: Errors must not be silently ignored
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// =============================================================================
// EXIT CODES - Specific codes for different error categories
// =============================================================================

const (
	// ExitSuccess indicates successful execution
	ExitSuccess = 0
	// ExitGeneralError indicates a general/unknown error
	ExitGeneralError = 1
	// ExitUsageError indicates invalid command usage or arguments
	ExitUsageError = 2
	// ExitConfigError indicates configuration file or settings error
	ExitConfigError = 3
	// ExitAuthError indicates authentication or authorization failure
	ExitAuthError = 4
	// ExitNetworkError indicates network or connectivity error
	ExitNetworkError = 5
	// ExitSecurityError indicates a security policy violation
	ExitSecurityError = 6
	// ExitNotFoundError indicates a resource was not found
	ExitNotFoundError = 7
	// ExitTimeoutError indicates an operation timed out
	ExitTimeoutError = 8
)

// =============================================================================
// ERROR TYPES FOR STRUCTURED ERROR HANDLING
// =============================================================================

// CommandError represents a CLI command error with context.
type CommandError struct {
	Command string // Command that failed (e.g., "audit", "rbac")
	Action  string // Action being performed (e.g., "show", "delete")
	Reason  string // Human-readable reason
	Err     error  // Underlying error (if any)
}

func (e *CommandError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s %s failed: %s: %v", e.Command, e.Action, e.Reason, e.Err)
	}
	return fmt.Sprintf("%s %s failed: %s", e.Command, e.Action, e.Reason)
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

// ValidationError represents a validation failure for user input.
type ValidationError struct {
	Field   string // Field that failed validation
	Value   string // Value that was provided
	Reason  string // Why validation failed
	Example string // Example of valid value (optional)
}

func (e *ValidationError) Error() string {
	msg := fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
	if e.Value != "" {
		msg += fmt.Sprintf(" (got: %s)", e.Value)
	}
	if e.Example != "" {
		msg += fmt.Sprintf("\nExample: %s", e.Example)
	}
	return msg
}

// PermissionError represents a permission/authorization failure.
type PermissionError struct {
	Action     string // Action that was denied
	UserID     string // User who was denied
	Permission string // Required permission
}

func (e *PermissionError) Error() string {
	return fmt.Sprintf("permission denied: %s requires permission '%s' (user: %s)",
		e.Action, e.Permission, e.UserID)
}

// NotFoundError represents a resource not found error.
type NotFoundError struct {
	Resource string // Type of resource (e.g., "session", "user", "file")
	ID       string // Identifier that was not found
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

// =============================================================================
// ERROR CONSTRUCTION HELPERS
// =============================================================================

// NewCommandError creates a new command error.
func NewCommandError(command, action, reason string, err error) error {
	return &CommandError{
		Command: command,
		Action:  action,
		Reason:  reason,
		Err:     err,
	}
}

// NewValidationError creates a new validation error.
func NewValidationError(field, value, reason string) error {
	return &ValidationError{
		Field:  field,
		Value:  value,
		Reason: reason,
	}
}

// NewValidationErrorWithExample creates a validation error with an example.
func NewValidationErrorWithExample(field, value, reason, example string) error {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Reason:  reason,
		Example: example,
	}
}

// NewPermissionError creates a new permission error.
func NewPermissionError(action, userID, permission string) error {
	return &PermissionError{
		Action:     action,
		UserID:     userID,
		Permission: permission,
	}
}

// NewNotFoundError creates a new not found error.
func NewNotFoundError(resource, id string) error {
	return &NotFoundError{
		Resource: resource,
		ID:       id,
	}
}

// =============================================================================
// ERROR DISPLAY HELPERS
// =============================================================================

// DisplayError displays an error in a consistent format.
// This should be called by command handlers before returning an error.
//
// In JSON mode, outputs structured JSON error.
// In normal mode, displays formatted error message.
func DisplayError(err error, jsonMode bool) {
	if err == nil {
		return
	}

	if jsonMode {
		DisplayErrorJSON(err)
		return
	}

	// Display human-readable error
	fmt.Println()
	fmt.Printf("%s %s\n", ErrorStyle.Render("[ERROR]"), err.Error())
	fmt.Println()
}

// DisplayErrorJSON outputs an error as JSON.
func DisplayErrorJSON(err error) {
	output := map[string]interface{}{
		"error":   err.Error(),
		"success": false,
	}

	// Add structured error details if available
	switch e := err.(type) {
	case *CommandError:
		output["error_type"] = "command_error"
		output["command"] = e.Command
		output["action"] = e.Action
		output["reason"] = e.Reason
		if e.Err != nil {
			output["underlying_error"] = e.Err.Error()
		}

	case *ValidationError:
		output["error_type"] = "validation_error"
		output["field"] = e.Field
		output["value"] = e.Value
		output["reason"] = e.Reason
		if e.Example != "" {
			output["example"] = e.Example
		}

	case *PermissionError:
		output["error_type"] = "permission_error"
		output["action"] = e.Action
		output["user_id"] = e.UserID
		output["required_permission"] = e.Permission

	case *NotFoundError:
		output["error_type"] = "not_found_error"
		output["resource"] = e.Resource
		output["id"] = e.ID

	default:
		output["error_type"] = "generic_error"
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(output)
}

// =============================================================================
// ERROR HANDLING PATTERNS
// =============================================================================

// HandleError is a convenience function that displays and returns an error.
// Use this as the final step in error handling.
//
// Example:
//
//	if err != nil {
//	    return HandleError(err, jsonMode)
//	}
func HandleError(err error, jsonMode bool) error {
	if err == nil {
		return nil
	}

	DisplayError(err, jsonMode)
	return err
}

// HandleErrorAndExit displays an error and exits with an appropriate exit code.
// Use this for fatal errors in main command handlers.
// Exit codes are determined based on the error type:
//   - ExitUsageError (2): ValidationError
//   - ExitConfigError (3): config-related errors
//   - ExitAuthError (4): PermissionError or auth-related errors
//   - ExitSecurityError (6): security policy violations
//   - ExitNotFoundError (7): NotFoundError
//   - ExitGeneralError (1): all other errors
//
// Example:
//
//	if err := validateConfig(); err != nil {
//	    HandleErrorAndExit(err, jsonMode)
//	}
func HandleErrorAndExit(err error, jsonMode bool) {
	if err == nil {
		return
	}

	DisplayError(err, jsonMode)
	os.Exit(GetExitCode(err))
}

// GetExitCode determines the appropriate exit code for an error.
// This enables specific exit codes for different error categories.
func GetExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}

	// Check for specific error types
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return ExitUsageError
	}

	var permissionErr *PermissionError
	if errors.As(err, &permissionErr) {
		return ExitAuthError
	}

	var notFoundErr *NotFoundError
	if errors.As(err, &notFoundErr) {
		return ExitNotFoundError
	}

	// Check error message content for additional categorization
	errMsg := strings.ToLower(err.Error())

	// Config errors
	if strings.Contains(errMsg, "config") ||
		strings.Contains(errMsg, "configuration") ||
		strings.Contains(errMsg, "settings") {
		return ExitConfigError
	}

	// Auth errors
	if strings.Contains(errMsg, "auth") ||
		strings.Contains(errMsg, "permission") ||
		strings.Contains(errMsg, "unauthorized") ||
		strings.Contains(errMsg, "access denied") ||
		strings.Contains(errMsg, "forbidden") {
		return ExitAuthError
	}

	// Network errors
	if strings.Contains(errMsg, "network") ||
		strings.Contains(errMsg, "connection") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "unreachable") ||
		strings.Contains(errMsg, "dial") {
		return ExitNetworkError
	}

	// Security errors
	if strings.Contains(errMsg, "security") ||
		strings.Contains(errMsg, "violation") ||
		strings.Contains(errMsg, "spillage") ||
		strings.Contains(errMsg, "classified") ||
		strings.Contains(errMsg, "audit") {
		return ExitSecurityError
	}

	// Timeout errors
	if strings.Contains(errMsg, "timed out") ||
		strings.Contains(errMsg, "deadline exceeded") {
		return ExitTimeoutError
	}

	return ExitGeneralError
}

// WrapError wraps an error with additional context.
// Use this to add context as errors bubble up the call stack.
//
// Example:
//
//	result, err := doSomething()
//	if err != nil {
//	    return WrapError(err, "failed to do something")
//	}
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// =============================================================================
// COMMON ERROR CONSTRUCTORS
// =============================================================================

// ErrMissingArgument creates an error for missing required arguments.
func ErrMissingArgument(argName, usage string) error {
	return NewValidationErrorWithExample(
		argName,
		"",
		"required argument missing",
		usage,
	)
}

// ErrInvalidFormat creates an error for invalid format.
func ErrInvalidFormat(field, value, expected string) error {
	return NewValidationErrorWithExample(
		field,
		value,
		"invalid format",
		expected,
	)
}

// ErrNotFound creates a not found error.
func ErrNotFound(resource, id string) error {
	return NewNotFoundError(resource, id)
}

// ErrPermissionDenied creates a permission denied error.
func ErrPermissionDenied(action, userID, permission string) error {
	return NewPermissionError(action, userID, permission)
}

// ErrUnsupportedFormat creates an error for unsupported formats.
func ErrUnsupportedFormat(format string, supportedFormats []string) error {
	return NewValidationErrorWithExample(
		"format",
		format,
		"unsupported format",
		fmt.Sprintf("supported formats: %v", supportedFormats),
	)
}

// =============================================================================
// ERROR CHECKING HELPERS
// =============================================================================

// IsPermissionError checks if an error is a permission error.
func IsPermissionError(err error) bool {
	_, ok := err.(*PermissionError)
	return ok
}

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// IsNotFoundError checks if an error is a not found error.
func IsNotFoundError(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

// IsCommandError checks if an error is a command error.
func IsCommandError(err error) bool {
	_, ok := err.(*CommandError)
	return ok
}
