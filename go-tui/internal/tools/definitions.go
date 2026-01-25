// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
// TOOLS: Proper timeout, validation, and resource cleanup
// All tool executors implement proper context handling with timeouts,
// input validation via ValidateToolArgs(), and resource cleanup using defer.
package tools

import (
	"context"
	"strings"
	"time"
)

// =============================================================================
// RISK LEVELS
// =============================================================================

// RiskLevel indicates how dangerous a tool operation is.
type RiskLevel int

const (
	// RiskLow - Read-only operations, no side effects
	RiskLow RiskLevel = iota

	// RiskMedium - May modify files but can be undone
	RiskMedium

	// RiskHigh - Modifies files, harder to undo
	RiskHigh

	// RiskCritical - System commands, potentially destructive
	RiskCritical
)

// String returns the string representation of a risk level.
func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "Low"
	case RiskMedium:
		return "Medium"
	case RiskHigh:
		return "High"
	case RiskCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// Color returns the color associated with a risk level.
func (r RiskLevel) Color() string {
	switch r {
	case RiskLow:
		return "#34D399" // Emerald
	case RiskMedium:
		return "#FBBF24" // Amber
	case RiskHigh:
		return "#FB923C" // Orange
	case RiskCritical:
		return "#FB7185" // Rose
	default:
		return "#A6ADC8" // Text secondary
	}
}

// =============================================================================
// PERMISSION LEVELS
// =============================================================================

// PermissionLevel determines how tool execution is authorized.
// Used in conjunction with RiskLevel to control user interaction during tool execution.
type PermissionLevel int

const (
	// PermissionAuto - Always allowed without prompting.
	// Used for safe, read-only operations like file reads and searches.
	PermissionAuto PermissionLevel = iota

	// PermissionAsk - Prompt user for permission before execution.
	// Used for operations that modify files or execute system commands.
	PermissionAsk

	// PermissionNever - Never allowed, even with user approval.
	// Used for blocked operations that would violate security policies.
	PermissionNever
)

// String returns the string representation of a permission level.
func (p PermissionLevel) String() string {
	switch p {
	case PermissionAuto:
		return "Auto"
	case PermissionAsk:
		return "Ask"
	case PermissionNever:
		return "Never"
	default:
		return "Unknown"
	}
}

// =============================================================================
// TOOL DEFINITION
// =============================================================================

// Tool represents an executable tool.
type Tool struct {
	// Name is the tool identifier (e.g., "Read", "Write", "Bash")
	Name string

	// Description explains what the tool does (full description for documentation)
	Description string

	// ShortDescription is a concise description for LLM tool schemas (<125 chars recommended)
	// If empty, the first line of Description is used
	ShortDescription string

	// Schema defines the tool's parameters
	Schema Schema

	// RiskLevel indicates how dangerous the tool is
	RiskLevel RiskLevel

	// Permission determines how execution is authorized
	Permission PermissionLevel

	// PermissionFunc is an optional function to compute permission dynamically based on parameters
	// If set, this takes precedence over the static Permission field
	PermissionFunc func(params map[string]interface{}) PermissionLevel

	// Executor handles the actual execution
	Executor ToolExecutor
}

// GetShortDescription returns the concise description suitable for LLM tool schemas.
// Returns ShortDescription if set, otherwise returns the first line of Description.
func (t *Tool) GetShortDescription() string {
	if t.ShortDescription != "" {
		return t.ShortDescription
	}
	// Return first line of description
	if idx := strings.Index(t.Description, "\n"); idx != -1 {
		return t.Description[:idx]
	}
	return t.Description
}

// Schema defines a tool's parameters.
type Schema struct {
	Parameters []Parameter
}

// Parameter defines a single tool parameter.
type Parameter struct {
	// Name of the parameter
	Name string

	// Type is the parameter type ("string", "number", "boolean", "array")
	Type string

	// Required indicates if the parameter must be provided
	Required bool

	// Description explains the parameter
	Description string

	// Default is the default value if not provided
	Default interface{}

	// Enum contains allowed values for string type (optional, for Ollama JSON schema)
	Enum []string
}

// =============================================================================
// TOOL EXECUTOR INTERFACE
// =============================================================================

// ToolExecutor is the interface for individual tool execution.
// Each tool implements this to define its execution logic.
type ToolExecutor interface {
	Execute(ctx context.Context, params map[string]interface{}) (Result, error)
}

// Result holds the outcome of a tool execution.
type Result struct {
	// Success indicates if the tool executed successfully
	Success bool

	// Output is the tool's output (for successful execution)
	Output string

	// Error is the error message (for failed execution)
	Error string

	// Duration is how long execution took
	Duration time.Duration

	// Truncated indicates output was truncated
	Truncated bool

	// BytesRead/Written for file operations
	BytesRead    int64
	BytesWritten int64

	// LinesCount for read/grep operations
	LinesCount int

	// MatchCount for grep operations
	MatchCount int

	// FilesMatched for glob operations
	FilesMatched int
}

// =============================================================================
// TOOL REGISTRY
// =============================================================================

// Registry holds all available tools.
type Registry struct {
	tools map[string]*Tool

	// Permission overrides (tool name -> permission)
	overrides map[string]PermissionLevel

	// "Always allow" preferences
	alwaysAllow map[string]bool
}

// NewRegistry creates a new tool registry with built-in tools.
func NewRegistry() *Registry {
	r := &Registry{
		tools:       make(map[string]*Tool),
		overrides:   make(map[string]PermissionLevel),
		alwaysAllow: make(map[string]bool),
	}

	// Register built-in tools
	r.RegisterBuiltins()

	return r
}

// RegisterBuiltins registers all built-in tools.
func (r *Registry) RegisterBuiltins() {
	r.Register(ReadTool)
	r.Register(WriteTool)
	r.Register(EditTool)
	r.Register(GlobTool)
	r.Register(GrepTool)
	r.Register(BashTool)
	r.Register(WebFetchTool)
	r.Register(WebSearchTool)
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool *Tool) {
	r.tools[tool.Name] = tool
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) *Tool {
	return r.tools[name]
}

// All returns all registered tools.
func (r *Registry) All() []*Tool {
	result := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// =============================================================================
// PERMISSION MANAGEMENT
// =============================================================================

// GetPermission returns the effective permission level for a tool.
func (r *Registry) GetPermission(toolName string) PermissionLevel {
	// Check for "always allow"
	if r.alwaysAllow[toolName] {
		return PermissionAuto
	}

	// Check for override
	if override, ok := r.overrides[toolName]; ok {
		return override
	}

	// Return tool's default permission
	if tool := r.Get(toolName); tool != nil {
		return tool.Permission
	}

	return PermissionAsk
}

// GetPermissionWithParams returns the effective permission level for a tool with parameters.
// This allows context-aware permission decisions based on the operation parameters.
//
// SECURITY: PermissionFunc is checked FIRST because it may contain security-critical
// path-based denials that should NOT be bypassed by alwaysAllow preferences.
// The order of checks is:
// 1. PermissionFunc (path-based security) - if returns PermissionAsk/PermissionNever, honor it
// 2. alwaysAllow (user preference) - only applied if PermissionFunc allows auto
// 3. overrides (admin config)
// 4. static Permission (tool default)
func (r *Registry) GetPermissionWithParams(toolName string, params map[string]interface{}) PermissionLevel {
	// Get tool first to access PermissionFunc
	tool := r.Get(toolName)

	// SECURITY: Check PermissionFunc FIRST - path-based denials cannot be bypassed
	// This prevents alwaysAllow from overriding security-critical path checks
	if tool != nil && tool.PermissionFunc != nil {
		funcPermission := tool.PermissionFunc(params)
		// If PermissionFunc requires asking or denies, always honor it (security boundary)
		if funcPermission >= PermissionAsk {
			return funcPermission
		}
		// PermissionFunc returned PermissionAuto, can now consider user preferences
	}

	// Now check "always allow" preference (only reaches here if PermissionFunc allowed auto)
	if r.alwaysAllow[toolName] {
		return PermissionAuto
	}

	// Check for override
	if override, ok := r.overrides[toolName]; ok {
		return override
	}

	// Use tool's static permission if no PermissionFunc
	if tool != nil {
		// If tool has PermissionFunc and we got here, it returned PermissionAuto
		if tool.PermissionFunc != nil {
			return PermissionAuto
		}
		// Otherwise use static permission
		return tool.Permission
	}

	return PermissionAsk
}

// SetPermissionOverride sets a permission override for a tool.
func (r *Registry) SetPermissionOverride(toolName string, perm PermissionLevel) {
	r.overrides[toolName] = perm
}

// SetAlwaysAllow marks a tool as always allowed.
func (r *Registry) SetAlwaysAllow(toolName string, always bool) {
	r.alwaysAllow[toolName] = always
}

// IsAlwaysAllowed returns if a tool is always allowed.
func (r *Registry) IsAlwaysAllowed(toolName string) bool {
	return r.alwaysAllow[toolName]
}

// NeedsPermission returns true if the tool needs user permission.
// NOTE: For context-aware permission checking with params, use NeedsPermissionWithParams.
func (r *Registry) NeedsPermission(toolName string) bool {
	return r.GetPermission(toolName) == PermissionAsk
}

// NeedsPermissionWithParams returns true if the tool needs user permission given the parameters.
// This is the preferred method for permission checks as it considers path-based security rules.
func (r *Registry) NeedsPermissionWithParams(toolName string, params map[string]interface{}) bool {
	return r.GetPermissionWithParams(toolName, params) >= PermissionAsk
}

// =============================================================================
// BUILT-IN TOOL DEFINITIONS
// =============================================================================

// ReadTool reads file contents.
var ReadTool = &Tool{
	Name: "Read",
	Description: `Read the contents of a file from the local filesystem. Use this tool when you need to:
- View source code, configuration files, or documentation
- Examine file contents before making edits
- Understand existing code structure or implementation details
- Check log files or output files

The file contents are returned with line numbers (cat -n style). For large files, use offset and limit parameters to read specific sections. Cannot read directories (use Glob instead) or binary files. Certain sensitive files (credentials, SSH keys, .env files) are protected and cannot be read.`,
	Schema: Schema{
		Parameters: []Parameter{
			{
				Name:        "file_path",
				Type:        "string",
				Required:    true,
				Description: "The absolute path to the file to read. Must be a valid file path, not a directory. Relative paths are resolved from the current working directory.",
			},
			{
				Name:        "offset",
				Type:        "integer",
				Required:    false,
				Description: "The line number to start reading from (1-indexed). Use this to skip to a specific section of a large file. Default: 1 (start from beginning).",
				Default:     1,
			},
			{
				Name:        "limit",
				Type:        "integer",
				Required:    false,
				Description: "Maximum number of lines to read. Use this with offset to read specific portions of large files. Default: 2000 lines.",
				Default:     2000,
			},
		},
	},
	RiskLevel:  RiskLow,
	Permission: PermissionAuto, // Default, overridden by PermissionFunc
	PermissionFunc: func(params map[string]interface{}) PermissionLevel {
		// Extract file_path parameter
		filePath, ok := params["file_path"].(string)
		if !ok || filePath == "" {
			return PermissionAsk // Be conservative if path missing
		}
		// Use context-aware permission check
		return GetPermissionForPath(filePath)
	},
	Executor: &ReadExecutor{},
}

// WriteTool writes content to a file.
var WriteTool = &Tool{
	Name: "Write",
	Description: `Write content to a new file or completely replace an existing file.

USE THIS TOOL FOR:
- Creating new files with specified content
- Completely replacing all content in an existing file
- Writing configuration files, scripts, or code files

DO NOT USE FOR:
- Making small changes to existing files (use Edit tool instead)
- Appending content to files (use Edit or Bash tool instead)

IMPORTANT:
- This tool OVERWRITES the entire file - any existing content will be LOST
- Parent directories are created automatically if they don't exist
- Sensitive paths (.env, credentials, system files) are blocked
- Maximum file size: 10MB`,
	Schema: Schema{
		Parameters: []Parameter{
			{
				Name:        "file_path",
				Type:        "string",
				Required:    true,
				Description: "Absolute path to the file to write. Parent directories will be created automatically if needed. Cannot write to system directories or sensitive files.",
			},
			{
				Name:        "content",
				Type:        "string",
				Required:    true,
				Description: "The complete content to write to the file. This will REPLACE any existing content entirely.",
			},
		},
	},
	RiskLevel:  RiskHigh,
	Permission: PermissionAsk,
	Executor:   &WriteExecutor{},
}

// EditTool edits a file using search and replace.
var EditTool = &Tool{
	Name: "Edit",
	// ShortDescription is used for LLM tool schemas (must be <125 chars)
	ShortDescription: "Edit files by finding and replacing exact text. Supports regex, backup/restore, and dry-run preview.",
	Description: `Edit a file by finding and replacing text. Use this tool when you need to make targeted changes to existing file content without rewriting the entire file.

WHEN TO USE:
- Modifying specific lines or sections of code
- Fixing bugs by changing specific text
- Renaming variables, functions, or identifiers
- Updating configuration values
- Adding or removing specific content

WHEN NOT TO USE:
- Creating new files (use Write instead)
- Completely rewriting a file (use Write instead)
- Reading file contents (use Read instead)

IMPORTANT: The old_string must match EXACTLY including whitespace and newlines. If the match fails, try including more context (surrounding lines) to make the match unique.`,
	Schema: Schema{
		Parameters: []Parameter{
			{
				Name:        "file_path",
				Type:        "string",
				Required:    true,
				Description: "Absolute path to the file to edit. The file must exist.",
			},
			{
				Name:        "old_string",
				Type:        "string",
				Required:    true,
				Description: "The exact text to find and replace. Must match character-for-character including whitespace, indentation, and newlines. Include enough surrounding context to make the match unique.",
			},
			{
				Name:        "new_string",
				Type:        "string",
				Required:    false,
				Description: "The replacement text. Use empty string to delete the matched text. If not provided, defaults to empty string (deletion).",
				Default:     "",
			},
			{
				Name:        "replace_all",
				Type:        "boolean",
				Required:    false,
				Description: "If true, replace ALL occurrences of old_string. If false (default), requires old_string to appear exactly once in the file for safety.",
				Default:     false,
			},
			{
				Name:        "use_regex",
				Type:        "boolean",
				Required:    false,
				Description: "If true, treat old_string as a regular expression pattern. Supports Go regexp syntax. Use capturing groups with $1, $2, etc. in new_string.",
				Default:     false,
			},
			{
				Name:        "create_backup",
				Type:        "boolean",
				Required:    false,
				Description: "If true, create a backup file with .bak extension before editing. Allows recovery via the restore_backup parameter.",
				Default:     false,
			},
			{
				Name:        "restore_backup",
				Type:        "boolean",
				Required:    false,
				Description: "If true, restore the file from its .bak backup instead of editing. Ignores old_string and new_string when set.",
				Default:     false,
			},
			{
				Name:        "dry_run",
				Type:        "boolean",
				Required:    false,
				Description: "If true, show what would be changed without actually modifying the file. Useful for previewing changes.",
				Default:     false,
			},
		},
	},
	RiskLevel:  RiskHigh,
	Permission: PermissionAsk,
	Executor:   &EditExecutor{},
}

// GlobTool finds files matching a pattern.
var GlobTool = &Tool{
	Name:             "Glob",
	ShortDescription: "Find files by name pattern (e.g., **/*.go). Use for locating files, NOT for searching file contents.",
	Description: `Fast file pattern matching tool for finding files by name or path pattern.

USE THIS TOOL WHEN:
- You need to find files by name pattern (e.g., all .go files, all test files)
- You want to explore a directory structure
- You need to locate configuration files, scripts, or specific file types
- You want file paths sorted by modification time (newest first)

DO NOT USE WHEN:
- You need to search FILE CONTENTS (use Grep instead)
- You need to find text within files (use Grep instead)

PATTERN SYNTAX:
- *       matches any sequence within a path segment (e.g., *.go matches foo.go)
- **      matches any sequence including path separators (e.g., **/*.go matches src/pkg/foo.go)
- ?       matches any single character
- [abc]   matches any character in brackets

EXAMPLES:
- "**/*.go"         - all Go files recursively
- "*.json"          - JSON files in current directory only
- "src/**/*.ts"     - TypeScript files under src/ directory
- "**/*_test.go"    - all Go test files
- "cmd/**/main.go"  - main.go files under cmd/

Results are limited to 100 files by default, sorted by modification time (newest first).
Automatically ignores: .git, node_modules, __pycache__, .venv, target, dist, build, .cache`,
	Schema: Schema{
		Parameters: []Parameter{
			{
				Name:        "pattern",
				Type:        "string",
				Required:    true,
				Description: "Glob pattern to match files. Use ** for recursive matching, * for single directory. Examples: '**/*.go', 'src/**/*.ts', '*.json'",
			},
			{
				Name:        "path",
				Type:        "string",
				Required:    false,
				Description: "Base directory to search in. Defaults to current working directory if not specified.",
			},
		},
	},
	RiskLevel:  RiskLow,
	Permission: PermissionAuto, // Default, overridden by PermissionFunc
	PermissionFunc: func(params map[string]interface{}) PermissionLevel {
		// Extract path parameter (base directory)
		path, ok := params["path"].(string)
		if ok && path != "" {
			// Check if searching in sensitive directory
			return GetPermissionForPath(path)
		}
		// Default to auto for searches in current directory
		return PermissionAuto
	},
	Executor: &GlobExecutor{},
}

// GrepTool searches file contents.
var GrepTool = &Tool{
	Name:             "Grep",
	ShortDescription: "Search for text/code patterns INSIDE files using regex. Use for finding function definitions, strings, etc.",
	Description: `Search for text patterns within file contents using regular expressions.

USE THIS TOOL WHEN:
- You need to find specific text, code, or patterns INSIDE files
- You're looking for function definitions, variable usages, or specific strings
- You want to search across multiple files for a pattern
- You need context lines around matches

DO NOT USE WHEN:
- You just need to find files by name (use Glob instead)
- You need to list directory contents (use Glob or Bash instead)

PATTERN SYNTAX (Regular Expressions):
- .        matches any single character
- *        matches zero or more of the preceding element
- +        matches one or more of the preceding element
- ?        matches zero or one of the preceding element
- ^        matches start of line
- $        matches end of line
- [abc]    matches any character in brackets
- \d       matches any digit
- \w       matches any word character
- \s       matches any whitespace
- (?i)     case-insensitive flag (or use case_insensitive parameter)

EXAMPLES:
- "func.*Error"      - find function definitions containing "Error"
- "TODO|FIXME"       - find TODO or FIXME comments
- "import.*\"fmt\""  - find fmt imports
- "^type.*struct"    - find struct definitions at start of line

OUTPUT MODES:
- "content"           - show matching lines with file:line:content format
- "files_with_matches" - show only file paths that contain matches (default)
- "count"             - show count of matches per file

Results limited to 50 matches. Skips binary files and sensitive paths automatically.`,
	Schema: Schema{
		Parameters: []Parameter{
			{
				Name:        "pattern",
				Type:        "string",
				Required:    true,
				Description: "Regular expression pattern to search for in file contents. Examples: 'func.*Error', 'TODO|FIXME', '^import'",
			},
			{
				Name:        "path",
				Type:        "string",
				Required:    false,
				Description: "File or directory to search in. If a file, searches that file only. If a directory, searches recursively. Defaults to current directory.",
			},
			{
				Name:        "glob",
				Type:        "string",
				Required:    false,
				Description: "Glob pattern to filter which files to search. Examples: '*.go' (Go files only), '*.{js,ts}' (JS/TS files), 'test_*.py' (Python test files)",
			},
			{
				Name:        "context",
				Type:        "number",
				Required:    false,
				Description: "Number of context lines to show before and after each match (0-10). Default: 0",
				Default:     0,
			},
			{
				Name:        "output_mode",
				Type:        "string",
				Required:    false,
				Description: "Output format for results",
				Default:     "content",
				Enum:        []string{"content", "files_with_matches", "count"},
			},
			{
				Name:        "case_insensitive",
				Type:        "boolean",
				Required:    false,
				Description: "Enable case-insensitive matching. Default: false",
				Default:     false,
			},
		},
	},
	RiskLevel:  RiskLow,
	Permission: PermissionAuto, // Default, overridden by PermissionFunc
	PermissionFunc: func(params map[string]interface{}) PermissionLevel {
		// Extract path parameter (search directory)
		path, ok := params["path"].(string)
		if ok && path != "" {
			// Check if searching in sensitive directory
			return GetPermissionForPath(path)
		}
		// Default to auto for searches in current directory
		return PermissionAuto
	},
	Executor: &GrepExecutor{},
}

// BashTool executes shell commands.
var BashTool = &Tool{
	Name: "Bash",
	// ShortDescription is used for LLM tool schemas (must be <125 chars)
	ShortDescription: "Execute shell commands for builds, tests, git, and file operations in a sandboxed environment",
	Description: `Execute shell commands in a secure, sandboxed environment.

USE THIS TOOL FOR:
- Running build commands (go build, npm install, cargo build, make)
- Running tests (go test, pytest, npm test)
- Git operations (git status, git diff, git log, git add, git commit)
- File operations (ls, cat, head, tail, cp, mv, mkdir)
- Searching (find, grep, ripgrep)
- System info (uname, hostname, whoami, pwd, env)
- Package managers (apt, brew, pip, npm, cargo)

DO NOT USE FOR:
- Interactive commands (vim, nano, ssh, mysql) - they will fail
- Destructive commands (rm -rf /, format) - they are blocked
- Privileged commands (sudo, su) - they require explicit approval
- Background processes (command &) - they are not allowed
- Remote code execution (curl | bash) - blocked for security

SECURITY RESTRICTIONS:
- Dangerous commands are blocked automatically
- Commands timeout after 30 seconds by default (max 10 minutes)
- Output is limited to 100KB
- No shell backgrounding allowed
- eval/source commands are blocked`,
	Schema: Schema{
		Parameters: []Parameter{
			{
				Name:        "command",
				Type:        "string",
				Required:    true,
				Description: "The shell command to execute. Must be a valid, non-destructive command. Examples: 'ls -la', 'git status', 'go build ./...'",
			},
			{
				Name:        "timeout",
				Type:        "number",
				Required:    false,
				Description: "Timeout in seconds (default: 30, max: 600). Command will be killed if it exceeds this duration.",
				Default:     30,
			},
			{
				Name:        "description",
				Type:        "string",
				Required:    false,
				Description: "Brief description of what this command does and why it is being executed",
			},
		},
	},
	RiskLevel:  RiskCritical,
	Permission: PermissionAsk,
	Executor:   &BashExecutor{},
}

// =============================================================================
// TOOL CALL PARSING
// =============================================================================

// ToolCall represents a parsed tool invocation.
type ToolCall struct {
	Name   string
	Params map[string]interface{}
}

// GetString gets a string parameter with a default value.
func (tc *ToolCall) GetString(name string, defaultVal string) string {
	if val, ok := tc.Params[name]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultVal
}

// GetInt gets an integer parameter with a default value.
func (tc *ToolCall) GetInt(name string, defaultVal int) int {
	if val, ok := tc.Params[name]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return defaultVal
}

// GetBool gets a boolean parameter with a default value.
func (tc *ToolCall) GetBool(name string, defaultVal bool) bool {
	if val, ok := tc.Params[name]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultVal
}
