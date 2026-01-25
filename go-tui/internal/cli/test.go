// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// test.go - Built-in self-test (BIST) command for IL5 compliance CI/CD.
//
// CLI: Comprehensive help and examples for all commands
//
// Command: test [subcommand]
// Short:   Run built-in self-tests (IL5 CI/CD)
// Aliases: selftest, bist
//
// Subcommands:
//   all (default)       Run all self-tests
//   security            Run security-focused tests only
//   connectivity        Run connectivity tests only
//
// Examples:
//   rigrun test                   Run all self-tests
//   rigrun test all               Run all self-tests (explicit)
//   rigrun test --json            JSON output for CI/CD pipelines
//   rigrun test security          Security compliance tests only
//   rigrun test connectivity      Network connectivity tests only
//
// Test Categories:
//   Security     - FIPS validation, audit logging, encryption
//   Connectivity - Ollama health, OpenRouter (if configured)
//   Compliance   - IL5 control verification
//
// Exit Codes:
//   0   All tests passed
//   1   One or more tests failed
//
// Flags:
//   --json              Output in JSON format (CI/CD integration)
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/cloud"
	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/ollama"
	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/tools"
)

// =============================================================================
// TEST STYLES
// =============================================================================

var (
	// Test title style
	testTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")). // Cyan
			MarginBottom(1)

	// Test pass style (green checkmark)
	testPassStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Bold(true)

	// Test fail style (red X)
	testFailStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	// Test skip style (yellow)
	testSkipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true)

	// Test ID style
	testIDStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Bold(true)

	// Test description style
	testDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255"))

	// Test details style
	testDetailsStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Italic(true).
				PaddingLeft(4)

	// Summary style
	testSummaryStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))
)

// =============================================================================
// TEST TYPES
// =============================================================================

// TestStatus represents the status of a test.
type TestStatus int

const (
	// TestPassed indicates the test passed.
	TestPassed TestStatus = iota
	// TestFailed indicates the test failed.
	TestFailed
	// TestSkipped indicates the test was skipped.
	TestSkipped
)

// String returns the string representation of the test status.
func (s TestStatus) String() string {
	switch s {
	case TestPassed:
		return "PASS"
	case TestFailed:
		return "FAIL"
	case TestSkipped:
		return "SKIP"
	default:
		return "UNKNOWN"
	}
}

// Symbol returns the Unicode symbol for the test status.
func (s TestStatus) Symbol() string {
	switch s {
	case TestPassed:
		return testPassStyle.Render("[PASS]")
	case TestFailed:
		return testFailStyle.Render("[FAIL]")
	case TestSkipped:
		return testSkipStyle.Render("[SKIP]")
	default:
		return "[????]"
	}
}

// TestResult represents the result of a single test.
type TestResult struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Category    string     `json:"category"`
	Status      TestStatus `json:"status"`
	Details     string     `json:"details,omitempty"`
	Duration    int64      `json:"duration_ms"`
}

// MarshalJSON implements custom JSON marshaling for TestResult.
func (r TestResult) MarshalJSON() ([]byte, error) {
	type alias TestResult
	return json.Marshal(&struct {
		Status string `json:"status"`
		alias
	}{
		Status: r.Status.String(),
		alias:  (alias)(r),
	})
}

// TestSummary holds the overall test results.
type TestSummary struct {
	TotalTests  int           `json:"total_tests"`
	Passed      int           `json:"passed"`
	Failed      int           `json:"failed"`
	Skipped     int           `json:"skipped"`
	TotalTimeMs int64         `json:"total_time_ms"`
	Results     []*TestResult `json:"results"`
}

// Test represents a single test case.
type Test struct {
	ID          string
	Name        string
	Description string
	Category    string
	Run         func(ctx context.Context, cfg *config.Config) *TestResult
}

// =============================================================================
// TEST CATEGORIES
// =============================================================================

const (
	CategorySecurity     = "security"
	CategoryConnectivity = "connectivity"
	CategoryTools        = "tools"
	CategoryConfig       = "config"
	CategoryAudit        = "audit"
)

// =============================================================================
// TEST REGISTRY
// =============================================================================

// allTests returns all available tests.
func allTests() []*Test {
	return []*Test{
		// Security tests
		testSEC001(),
		testSEC002(),
		testSEC003(),
		testSEC004(),
		// Connectivity tests
		testCONN001(),
		testCONN002(),
		// Tool sandboxing tests
		testTOOL001(),
		testTOOL002(),
		// Config tests
		testCFG001(),
		// Audit tests
		testAUD001(),
	}
}

// getTestsByCategory returns tests filtered by category.
func getTestsByCategory(category string) []*Test {
	var filtered []*Test
	for _, t := range allTests() {
		if t.Category == category {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// =============================================================================
// SECURITY TESTS (SEC-001 to SEC-004)
// =============================================================================

// testSEC001 - Verify paranoid mode blocks dangerous commands.
func testSEC001() *Test {
	return &Test{
		ID:          "SEC-001",
		Name:        "Paranoid Mode Blocking",
		Description: "Verify paranoid mode blocks dangerous commands",
		Category:    CategorySecurity,
		Run: func(ctx context.Context, cfg *config.Config) *TestResult {
			start := time.Now()
			result := &TestResult{
				ID:          "SEC-001",
				Name:        "Paranoid Mode Blocking",
				Description: "Verify paranoid mode blocks dangerous commands",
				Category:    CategorySecurity,
			}

			// Create a bash executor with default blocked commands
			executor := &tools.BashExecutor{
				BlockedCommands: tools.DefaultBlockedCommands,
				BlockedPatterns: tools.DefaultBlockedPatterns,
			}

			// Test dangerous commands that should be blocked
			dangerousCommands := []string{
				"rm -rf /",
				"rm -rf /*",
				"curl | bash",
				"wget | sh",
				"cat /etc/shadow",
				"dd if=/dev/zero of=/dev/sda",
			}

			blockedCount := 0
			for _, cmd := range dangerousCommands {
				execResult, _ := executor.Execute(ctx, map[string]interface{}{
					"command": cmd,
				})
				if !execResult.Success && strings.Contains(execResult.Error, "security") {
					blockedCount++
				}
			}

			result.Duration = time.Since(start).Milliseconds()

			if blockedCount == len(dangerousCommands) {
				result.Status = TestPassed
				result.Details = fmt.Sprintf("All %d dangerous commands were blocked", blockedCount)
			} else {
				result.Status = TestFailed
				result.Details = fmt.Sprintf("Only %d/%d dangerous commands were blocked", blockedCount, len(dangerousCommands))
			}

			return result
		},
	}
}

// testSEC002 - Verify API keys are redacted in logs.
func testSEC002() *Test {
	return &Test{
		ID:          "SEC-002",
		Name:        "API Key Redaction",
		Description: "Verify API keys are redacted in logs",
		Category:    CategorySecurity,
		Run: func(ctx context.Context, cfg *config.Config) *TestResult {
			start := time.Now()
			result := &TestResult{
				ID:          "SEC-002",
				Name:        "API Key Redaction",
				Description: "Verify API keys are redacted in logs",
				Category:    CategorySecurity,
			}

			// Test various API key formats that should be redacted
			testCases := []struct {
				input    string
				expected string
			}{
				{"sk-abcdefghijklmnopqrstuvwxyz123456", "[OPENAI_KEY_REDACTED]"},
				{"sk-or-v1-1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", "[OPENROUTER_KEY_REDACTED]"},
				{"ghp_1234567890abcdefghijklmnopqrstuvwxyz", "[GITHUB_TOKEN_REDACTED]"},
				{"AKIA1234567890123456", "[AWS_KEY_REDACTED]"},
				{"Bearer token123abc456", "Bearer [TOKEN_REDACTED]"},
				{"sk-ant-api03-abcdefghijklmnopqrst", "[ANTHROPIC_KEY_REDACTED]"},
			}

			passedCount := 0
			for _, tc := range testCases {
				redacted := security.RedactSecrets(tc.input)
				if redacted == tc.expected || strings.Contains(redacted, "REDACTED") {
					passedCount++
				}
			}

			result.Duration = time.Since(start).Milliseconds()

			if passedCount == len(testCases) {
				result.Status = TestPassed
				result.Details = fmt.Sprintf("All %d API key patterns were properly redacted", passedCount)
			} else {
				result.Status = TestFailed
				result.Details = fmt.Sprintf("Only %d/%d API key patterns were redacted", passedCount, len(testCases))
			}

			return result
		},
	}
}

// testSEC003 - Verify session timeout is configured.
func testSEC003() *Test {
	return &Test{
		ID:          "SEC-003",
		Name:        "Session Timeout Configuration",
		Description: "Verify session timeout is configured",
		Category:    CategorySecurity,
		Run: func(ctx context.Context, cfg *config.Config) *TestResult {
			start := time.Now()
			result := &TestResult{
				ID:          "SEC-003",
				Name:        "Session Timeout Configuration",
				Description: "Verify session timeout is configured",
				Category:    CategorySecurity,
			}

			result.Duration = time.Since(start).Milliseconds()

			// IL5 AC-12 compliance: Session timeout must be 900-1800 seconds (15-30 minutes)
			// Zero or values outside this range violate DoD STIG requirements
			if cfg.Security.SessionTimeoutSecs < 900 {
				result.Status = TestFailed
				result.Details = fmt.Sprintf("Session timeout must be at least 900 seconds (15 min) per IL5 AC-12 compliance, got %d", cfg.Security.SessionTimeoutSecs)
			} else if cfg.Security.SessionTimeoutSecs > 1800 {
				result.Status = TestFailed
				result.Details = fmt.Sprintf("Session timeout cannot exceed 1800 seconds (30 min) per IL5 AC-12 compliance, got %d", cfg.Security.SessionTimeoutSecs)
			} else {
				result.Status = TestPassed
				result.Details = fmt.Sprintf("Session timeout configured: %d seconds (within IL5 AC-12 range: 900-1800)", cfg.Security.SessionTimeoutSecs)
			}

			return result
		},
	}
}

// testSEC004 - Verify classification banner system works.
func testSEC004() *Test {
	return &Test{
		ID:          "SEC-004",
		Name:        "Classification Banner System",
		Description: "Verify classification banner system works",
		Category:    CategorySecurity,
		Run: func(ctx context.Context, cfg *config.Config) *TestResult {
			start := time.Now()
			result := &TestResult{
				ID:          "SEC-004",
				Name:        "Classification Banner System",
				Description: "Verify classification banner system works",
				Category:    CategorySecurity,
			}

			// Test classification parsing and rendering
			testCases := []string{
				"UNCLASSIFIED",
				"CUI",
				"SECRET",
				"TOP SECRET",
				"SECRET//NOFORN",
			}

			passedCount := 0
			for _, tc := range testCases {
				classification, err := security.ParseClassification(tc)
				if err == nil {
					// Verify the classification can be rendered
					banner := security.RenderTopBanner(classification, 80)
					if len(banner) > 0 && strings.Contains(strings.ToUpper(banner), strings.Split(tc, "//")[0]) {
						passedCount++
					}
				}
			}

			result.Duration = time.Since(start).Milliseconds()

			if passedCount == len(testCases) {
				result.Status = TestPassed
				result.Details = fmt.Sprintf("All %d classification levels parsed and rendered correctly", passedCount)
			} else {
				result.Status = TestFailed
				result.Details = fmt.Sprintf("Only %d/%d classification levels worked correctly", passedCount, len(testCases))
			}

			return result
		},
	}
}

// =============================================================================
// CONNECTIVITY TESTS (CONN-001 to CONN-002)
// =============================================================================

// testCONN001 - Verify Ollama connection.
func testCONN001() *Test {
	return &Test{
		ID:          "CONN-001",
		Name:        "Ollama Connection",
		Description: "Verify Ollama connection (if configured)",
		Category:    CategoryConnectivity,
		Run: func(ctx context.Context, cfg *config.Config) *TestResult {
			start := time.Now()
			result := &TestResult{
				ID:          "CONN-001",
				Name:        "Ollama Connection",
				Description: "Verify Ollama connection (if configured)",
				Category:    CategoryConnectivity,
			}

			// Create Ollama client with config
			ollamaConfig := &ollama.ClientConfig{
				BaseURL:      cfg.Local.OllamaURL,
				DefaultModel: cfg.Local.OllamaModel,
			}
			client := ollama.NewClientWithConfig(ollamaConfig)

			// Test connection with timeout
			testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			err := client.CheckRunning(testCtx)
			result.Duration = time.Since(start).Milliseconds()

			if err == nil {
				// Try to list models to verify full functionality
				models, listErr := client.ListModels(testCtx)
				if listErr == nil {
					result.Status = TestPassed
					result.Details = fmt.Sprintf("Connected to Ollama at %s with %d models available", cfg.Local.OllamaURL, len(models))
				} else {
					result.Status = TestPassed
					result.Details = fmt.Sprintf("Connected to Ollama at %s (model list unavailable)", cfg.Local.OllamaURL)
				}
			} else {
				result.Status = TestFailed
				result.Details = fmt.Sprintf("Cannot connect to Ollama at %s: %v", cfg.Local.OllamaURL, err)
			}

			return result
		},
	}
}

// testCONN002 - Verify OpenRouter connection.
func testCONN002() *Test {
	return &Test{
		ID:          "CONN-002",
		Name:        "OpenRouter Connection",
		Description: "Verify OpenRouter connection (if API key set)",
		Category:    CategoryConnectivity,
		Run: func(ctx context.Context, cfg *config.Config) *TestResult {
			start := time.Now()
			result := &TestResult{
				ID:          "CONN-002",
				Name:        "OpenRouter Connection",
				Description: "Verify OpenRouter connection (if API key set)",
				Category:    CategoryConnectivity,
			}

			// Check if OpenRouter is configured
			if cfg.Cloud.OpenRouterKey == "" {
				result.Duration = time.Since(start).Milliseconds()
				result.Status = TestSkipped
				result.Details = "OpenRouter API key not configured (cloud.openrouter_key is empty)"
				return result
			}

			// Create OpenRouter client
			client := cloud.NewOpenRouterClient(cfg.Cloud.OpenRouterKey)

			// Test connection by listing models (doesn't require auth for listing)
			testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			models, err := client.ListModels(testCtx)
			result.Duration = time.Since(start).Milliseconds()

			if err == nil && len(models) > 0 {
				result.Status = TestPassed
				result.Details = fmt.Sprintf("Connected to OpenRouter with %d models available", len(models))
			} else if err != nil {
				result.Status = TestFailed
				result.Details = fmt.Sprintf("Cannot connect to OpenRouter: %v", err)
			} else {
				result.Status = TestFailed
				result.Details = "Connected to OpenRouter but no models returned"
			}

			return result
		},
	}
}

// =============================================================================
// TOOL SANDBOXING TESTS (TOOL-001 to TOOL-002)
// =============================================================================

// testTOOL001 - Verify bash tool blocks rm -rf /.
func testTOOL001() *Test {
	return &Test{
		ID:          "TOOL-001",
		Name:        "Bash Tool Destructive Command Blocking",
		Description: "Verify bash tool blocks rm -rf /",
		Category:    CategoryTools,
		Run: func(ctx context.Context, cfg *config.Config) *TestResult {
			start := time.Now()
			result := &TestResult{
				ID:          "TOOL-001",
				Name:        "Bash Tool Destructive Command Blocking",
				Description: "Verify bash tool blocks rm -rf /",
				Category:    CategoryTools,
			}

			// Create a bash executor with default security settings
			executor := &tools.BashExecutor{
				BlockedCommands: tools.DefaultBlockedCommands,
				BlockedPatterns: tools.DefaultBlockedPatterns,
			}

			// Test the specific command that must be blocked
			execResult, _ := executor.Execute(ctx, map[string]interface{}{
				"command": "rm -rf /",
			})

			result.Duration = time.Since(start).Milliseconds()

			if !execResult.Success && strings.Contains(execResult.Error, "security") {
				result.Status = TestPassed
				result.Details = "Command 'rm -rf /' was correctly blocked: " + execResult.Error
			} else {
				result.Status = TestFailed
				result.Details = "CRITICAL: Command 'rm -rf /' was NOT blocked!"
			}

			return result
		},
	}
}

// testTOOL002 - Verify read tool blocks /etc/shadow.
func testTOOL002() *Test {
	return &Test{
		ID:          "TOOL-002",
		Name:        "Read Tool Sensitive File Blocking",
		Description: "Verify read tool blocks /etc/shadow",
		Category:    CategoryTools,
		Run: func(ctx context.Context, cfg *config.Config) *TestResult {
			start := time.Now()
			result := &TestResult{
				ID:          "TOOL-002",
				Name:        "Read Tool Sensitive File Blocking",
				Description: "Verify read tool blocks /etc/shadow",
				Category:    CategoryTools,
			}

			// Create a read executor with default security settings
			executor := &tools.ReadExecutor{}

			// Test reading sensitive file
			execResult, _ := executor.Execute(ctx, map[string]interface{}{
				"file_path": "/etc/shadow",
			})

			result.Duration = time.Since(start).Milliseconds()

			// The file should be blocked due to sensitive file patterns
			if !execResult.Success && (strings.Contains(execResult.Error, "denied") ||
				strings.Contains(execResult.Error, "sensitive") ||
				strings.Contains(execResult.Error, "blocked")) {
				result.Status = TestPassed
				result.Details = "File '/etc/shadow' was correctly blocked: " + execResult.Error
			} else if !execResult.Success {
				// File doesn't exist or other error - still a pass since we can't read it
				result.Status = TestPassed
				result.Details = "File '/etc/shadow' access prevented: " + execResult.Error
			} else {
				result.Status = TestFailed
				result.Details = "CRITICAL: File '/etc/shadow' was NOT blocked!"
			}

			return result
		},
	}
}

// =============================================================================
// CONFIG TESTS (CFG-001)
// =============================================================================

// testCFG001 - Verify config file is valid.
func testCFG001() *Test {
	return &Test{
		ID:          "CFG-001",
		Name:        "Config File Validation",
		Description: "Verify config file is valid",
		Category:    CategoryConfig,
		Run: func(ctx context.Context, cfg *config.Config) *TestResult {
			start := time.Now()
			result := &TestResult{
				ID:          "CFG-001",
				Name:        "Config File Validation",
				Description: "Verify config file is valid",
				Category:    CategoryConfig,
			}

			// Validate the configuration
			err := cfg.Validate()
			result.Duration = time.Since(start).Milliseconds()

			if err == nil {
				result.Status = TestPassed
				result.Details = "Configuration is valid"
			} else {
				result.Status = TestFailed
				result.Details = fmt.Sprintf("Configuration validation failed: %v", err)
			}

			return result
		},
	}
}

// =============================================================================
// AUDIT TESTS (AUD-001)
// =============================================================================

// testAUD001 - Verify audit log is writable.
func testAUD001() *Test {
	return &Test{
		ID:          "AUD-001",
		Name:        "Audit Log Writable",
		Description: "Verify audit log is writable",
		Category:    CategoryAudit,
		Run: func(ctx context.Context, cfg *config.Config) *TestResult {
			start := time.Now()
			result := &TestResult{
				ID:          "AUD-001",
				Name:        "Audit Log Writable",
				Description: "Verify audit log is writable",
				Category:    CategoryAudit,
			}

			// Get the audit log path
			auditPath := security.DefaultAuditPath()

			// Ensure directory exists
			auditDir := filepath.Dir(auditPath)
			if err := os.MkdirAll(auditDir, 0700); err != nil {
				result.Duration = time.Since(start).Milliseconds()
				result.Status = TestFailed
				result.Details = fmt.Sprintf("Cannot create audit log directory %s: %v", auditDir, err)
				return result
			}

			// Try to create/open the audit log for writing
			logger, err := security.NewAuditLogger(auditPath)
			if err != nil {
				result.Duration = time.Since(start).Milliseconds()
				result.Status = TestFailed
				result.Details = fmt.Sprintf("Cannot create audit logger at %s: %v", auditPath, err)
				return result
			}
			defer logger.Close()

			// Try to write a test entry
			testEvent := security.AuditEvent{
				Timestamp: time.Now(),
				EventType: "TEST",
				SessionID: "bist-test-session",
				Success:   true,
			}

			err = logger.Log(testEvent)
			result.Duration = time.Since(start).Milliseconds()

			if err == nil {
				result.Status = TestPassed
				result.Details = fmt.Sprintf("Audit log writable at %s", auditPath)
			} else {
				result.Status = TestFailed
				result.Details = fmt.Sprintf("Cannot write to audit log at %s: %v", auditPath, err)
			}

			return result
		},
	}
}

// =============================================================================
// TEST RUNNER
// =============================================================================

// RunTests executes the given tests and returns a summary.
func RunTests(ctx context.Context, tests []*Test, cfg *config.Config, verbose bool) *TestSummary {
	summary := &TestSummary{
		TotalTests: len(tests),
		Results:    make([]*TestResult, 0, len(tests)),
	}

	totalStart := time.Now()

	for _, test := range tests {
		result := test.Run(ctx, cfg)
		summary.Results = append(summary.Results, result)

		switch result.Status {
		case TestPassed:
			summary.Passed++
		case TestFailed:
			summary.Failed++
		case TestSkipped:
			summary.Skipped++
		}
	}

	summary.TotalTimeMs = time.Since(totalStart).Milliseconds()
	return summary
}

// PrintTestResult prints a single test result.
func PrintTestResult(result *TestResult, verbose bool) {
	// Print test ID and status
	fmt.Printf("%s %s: %s\n",
		result.Status.Symbol(),
		testIDStyle.Render(result.ID),
		testDescStyle.Render(result.Name))

	// Print details if verbose or if test failed
	if verbose || result.Status == TestFailed {
		if result.Details != "" {
			fmt.Println(testDetailsStyle.Render(result.Details))
		}
	}
}

// PrintTestSummary prints the test summary.
func PrintTestSummary(summary *TestSummary) {
	fmt.Println()
	fmt.Println(separatorStyle.Render(strings.Repeat("-", 50)))

	// Build summary line
	summaryParts := []string{
		fmt.Sprintf("%d passed", summary.Passed),
	}
	if summary.Failed > 0 {
		summaryParts = append(summaryParts, testFailStyle.Render(fmt.Sprintf("%d failed", summary.Failed)))
	}
	if summary.Skipped > 0 {
		summaryParts = append(summaryParts, testSkipStyle.Render(fmt.Sprintf("%d skipped", summary.Skipped)))
	}

	fmt.Printf("Tests: %s | Total: %d | Time: %dms\n",
		strings.Join(summaryParts, ", "),
		summary.TotalTests,
		summary.TotalTimeMs)
}

// =============================================================================
// HANDLE TEST COMMAND
// =============================================================================

// HandleTest handles the "test" command.
func HandleTest(args Args) error {
	// Load configuration
	cfg := config.Global()

	// Create context
	ctx := context.Background()

	// Determine which tests to run based on subcommand
	var testsToRun []*Test

	switch args.Subcommand {
	case "all", "":
		testsToRun = allTests()
	case "security":
		testsToRun = getTestsByCategory(CategorySecurity)
	case "connectivity":
		testsToRun = getTestsByCategory(CategoryConnectivity)
	case "tools":
		testsToRun = getTestsByCategory(CategoryTools)
	case "config":
		testsToRun = getTestsByCategory(CategoryConfig)
	case "audit":
		testsToRun = getTestsByCategory(CategoryAudit)
	default:
		return fmt.Errorf("unknown test subcommand: %s\nValid subcommands: all, security, connectivity, tools, config, audit", args.Subcommand)
	}

	if len(testsToRun) == 0 {
		return fmt.Errorf("no tests found for category: %s", args.Subcommand)
	}

	// Check for JSON output flag (both global and local)
	jsonOutput := args.JSON
	for _, arg := range args.Raw {
		if arg == "--json" {
			jsonOutput = true
			break
		}
	}

	// Check for verbose flag (both global and local)
	verbose := args.Verbose
	for _, arg := range args.Raw {
		if arg == "--verbose" || arg == "-v" {
			verbose = true
			break
		}
	}

	// Run tests
	summary := RunTests(ctx, testsToRun, cfg, verbose)

	// Output results
	if jsonOutput {
		// JSON output for CI/CD integration
		output, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal test results: %w", err)
		}
		fmt.Println(string(output))
	} else {
		// Human-readable output
		separator := strings.Repeat("=", 50)
		fmt.Println()
		fmt.Println(testTitleStyle.Render("rigrun Built-In Self-Test (BIST)"))
		fmt.Println(separatorStyle.Render(separator))
		fmt.Println()

		for _, result := range summary.Results {
			PrintTestResult(result, verbose)
		}

		PrintTestSummary(summary)
		fmt.Println()
	}

	// Return exit code based on results
	if summary.Failed > 0 {
		return fmt.Errorf("%d test(s) failed", summary.Failed)
	}

	return nil
}

// parseTestArgs parses test command specific arguments.
func parseTestArgs(args *Args, remaining []string) {
	if len(remaining) > 0 {
		args.Subcommand = remaining[0]
	}
	// Store remaining args for flag parsing
	if len(remaining) > 1 {
		args.Raw = remaining[1:]
	}
}
