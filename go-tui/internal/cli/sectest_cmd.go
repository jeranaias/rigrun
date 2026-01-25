// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// sectest_cmd.go - Security testing CLI commands for NIST 800-53 SA-11.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements CLI interface for developer security testing and evaluation
// controls per NIST 800-53 SA-11 for DoD IL5 compliance.
//
// Command: sectest [subcommand]
// Short:   Security testing and evaluation (IL5 SA-11)
// Aliases: sast
//
// Subcommands:
//   run (default)       Run all security tests
//   static              Run static analysis
//   deps                Check dependency vulnerabilities
//   fuzz [target]       Run fuzz testing
//   report              Generate security test report
//   history             Show test history
//   status              Show last test results
//
// Examples:
//   rigrun sectest                        Run all tests (default)
//   rigrun sectest run                    Run all security tests
//   rigrun sectest run --type static      Run specific test type
//   rigrun sectest static                 Static code analysis
//   rigrun sectest deps                   Dependency vulnerability check
//   rigrun sectest fuzz config            Fuzz test config parser
//   rigrun sectest report                 Generate report
//   rigrun sectest report --format html   HTML report
//   rigrun sectest history                Show test history
//   rigrun sectest status                 Last test results
//   rigrun sectest status --json          JSON format
//
// Test Types:
//   static              SAST (Static Application Security Testing)
//   deps                Software composition analysis
//   fuzz                Fuzz testing for edge cases
//   config              Configuration security testing
//
// Flags:
//   --type TYPE         Specific test type to run
//   --format FORMAT     Report format (text, json, html)
//   --json              Output in JSON format
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// SECTEST COMMAND STYLES
// =============================================================================

var (
	sectestTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	sectestSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	sectestLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Light gray
				Width(18)

	sectestValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")) // White

	sectestGreenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")) // Green

	sectestYellowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")) // Yellow

	sectestRedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")) // Red

	sectestDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Dim

	sectestSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	sectestErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)

	sectestSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// =============================================================================
// SECTEST ARGUMENTS
// =============================================================================

// SecTestArgs holds parsed sectest command arguments.
type SecTestArgs struct {
	Subcommand string
	TestType   string
	Target     string
	Format     string
	Output     string
	JSON       bool
}

// parseSecTestArgs parses sectest command specific arguments.
func parseSecTestArgs(args *Args, remaining []string) SecTestArgs {
	sectestArgs := SecTestArgs{
		Format: "text",
		JSON:   args.JSON,
	}

	if len(remaining) > 0 {
		sectestArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--type", "-t":
			if i+1 < len(remaining) {
				i++
				sectestArgs.TestType = remaining[i]
			}
		case "--target":
			if i+1 < len(remaining) {
				i++
				sectestArgs.Target = remaining[i]
			}
		case "--format", "-f":
			if i+1 < len(remaining) {
				i++
				sectestArgs.Format = remaining[i]
			}
		case "--output", "-o":
			if i+1 < len(remaining) {
				i++
				sectestArgs.Output = remaining[i]
			}
		case "--json":
			sectestArgs.JSON = true
		default:
			if strings.HasPrefix(arg, "--type=") {
				sectestArgs.TestType = strings.TrimPrefix(arg, "--type=")
			} else if strings.HasPrefix(arg, "--target=") {
				sectestArgs.Target = strings.TrimPrefix(arg, "--target=")
			} else if strings.HasPrefix(arg, "--format=") {
				sectestArgs.Format = strings.TrimPrefix(arg, "--format=")
			} else if strings.HasPrefix(arg, "--output=") {
				sectestArgs.Output = strings.TrimPrefix(arg, "--output=")
			} else if !strings.HasPrefix(arg, "-") {
				// Positional argument for target
				sectestArgs.Target = arg
			}
		}
	}

	return sectestArgs
}

// =============================================================================
// HANDLE SECTEST
// =============================================================================

// HandleSecTest handles the "sectest" command with various subcommands.
// Implements SA-11 security testing commands.
func HandleSecTest(args Args) error {
	sectestArgs := parseSecTestArgs(&args, args.Raw)

	switch sectestArgs.Subcommand {
	case "", "run":
		return handleSecTestRun(sectestArgs)
	case "static":
		return handleSecTestStatic(sectestArgs)
	case "deps":
		return handleSecTestDeps(sectestArgs)
	case "fuzz":
		return handleSecTestFuzz(sectestArgs)
	case "report":
		return handleSecTestReport(sectestArgs)
	case "history":
		return handleSecTestHistory(sectestArgs)
	case "status":
		return handleSecTestStatus(sectestArgs)
	default:
		return fmt.Errorf("unknown sectest subcommand: %s\n\nUsage:\n"+
			"  rigrun sectest run [--type TYPE]    Run security tests\n"+
			"  rigrun sectest static               Run static analysis\n"+
			"  rigrun sectest deps                 Check dependencies\n"+
			"  rigrun sectest fuzz [target]        Run fuzz testing\n"+
			"  rigrun sectest report [--format]    Generate test report\n"+
			"  rigrun sectest history              Show test history\n"+
			"  rigrun sectest status               Show last test results\n\n"+
			"Test Types:\n"+
			"  static   - Static code analysis\n"+
			"  deps     - Dependency vulnerability check\n"+
			"  auth     - Authentication testing\n"+
			"  authz    - Authorization testing\n"+
			"  input    - Input validation testing\n"+
			"  crypto   - Cryptographic implementation testing", sectestArgs.Subcommand)
	}
}

// =============================================================================
// SECTEST RUN
// =============================================================================

// handleSecTestRun runs security tests.
func handleSecTestRun(args SecTestArgs) error {
	tester := security.GlobalSecurityTester()

	// Set project root to current directory or go-tui directory
	cwd, _ := os.Getwd()
	if strings.Contains(cwd, "go-tui") {
		tester.SetProjectRoot(cwd)
	} else {
		// Try to find go-tui directory
		goTuiPath := filepath.Join(cwd, "go-tui")
		if _, err := os.Stat(goTuiPath); err == nil {
			tester.SetProjectRoot(goTuiPath)
		}
	}

	if args.TestType != "" {
		// Run specific test type
		testType := security.TestType(args.TestType)
		result, err := tester.RunSpecificTest(testType)
		if err != nil {
			return fmt.Errorf("failed to run test: %w", err)
		}

		if args.JSON {
			return outputTestResultJSON(result)
		}

		displayTestResult(result)
		return nil
	}

	// Run all tests
	if !args.JSON {
		fmt.Println()
		fmt.Println(sectestTitleStyle.Render("SA-11 Security Testing"))
		fmt.Println(sectestSeparatorStyle.Render(strings.Repeat("=", 60)))
		fmt.Println()
		fmt.Println("Running security tests...")
		fmt.Println()
	}

	report, err := tester.RunSecurityTests()
	if err != nil {
		return fmt.Errorf("failed to run security tests: %w", err)
	}

	if args.JSON {
		return outputTestReportJSON(report)
	}

	displayTestReport(report)
	return nil
}

// =============================================================================
// SECTEST STATIC
// =============================================================================

// handleSecTestStatic runs static analysis.
func handleSecTestStatic(args SecTestArgs) error {
	tester := security.GlobalSecurityTester()

	result, err := tester.RunStaticAnalysis()
	if err != nil {
		return fmt.Errorf("failed to run static analysis: %w", err)
	}

	if args.JSON {
		return outputTestResultJSON(result)
	}

	displayTestResult(result)
	return nil
}

// =============================================================================
// SECTEST DEPS
// =============================================================================

// handleSecTestDeps checks dependencies.
func handleSecTestDeps(args SecTestArgs) error {
	tester := security.GlobalSecurityTester()

	result, err := tester.RunDependencyCheck()
	if err != nil {
		return fmt.Errorf("failed to check dependencies: %w", err)
	}

	if args.JSON {
		return outputTestResultJSON(result)
	}

	displayTestResult(result)
	return nil
}

// =============================================================================
// SECTEST FUZZ
// =============================================================================

// handleSecTestFuzz runs fuzz testing.
func handleSecTestFuzz(args SecTestArgs) error {
	tester := security.GlobalSecurityTester()

	target := args.Target
	if target == "" {
		target = "all"
	}

	result, err := tester.RunFuzzTest(target)
	if err != nil {
		return fmt.Errorf("failed to run fuzz test: %w", err)
	}

	if args.JSON {
		return outputTestResultJSON(result)
	}

	displayTestResult(result)
	return nil
}

// =============================================================================
// SECTEST REPORT
// =============================================================================

// handleSecTestReport generates a security test report.
func handleSecTestReport(args SecTestArgs) error {
	tester := security.GlobalSecurityTester()

	report := tester.GetLatestReport()
	if report == nil {
		if args.JSON {
			return outputJSON(map[string]interface{}{
				"error":   "no test results available",
				"message": "Run 'rigrun sectest run' first",
			})
		}
		fmt.Println()
		fmt.Println(sectestYellowStyle.Render("No test results available."))
		fmt.Println("Run 'rigrun sectest run' first to generate test results.")
		fmt.Println()
		return nil
	}

	reportText, err := tester.GenerateTestReport(args.Format)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Output to file if specified
	if args.Output != "" {
		// Validate output path to prevent path traversal attacks
		validatedPath, err := ValidateOutputPath(args.Output)
		if err != nil {
			return fmt.Errorf("invalid output path: %w", err)
		}
		err = os.WriteFile(validatedPath, []byte(reportText), 0600)
		if err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		fmt.Printf("%s Report written to: %s\n",
			sectestSuccessStyle.Render("[OK]"),
			validatedPath)
		return nil
	}

	// Output to stdout
	fmt.Println(reportText)
	return nil
}

// =============================================================================
// SECTEST HISTORY
// =============================================================================

// handleSecTestHistory shows test history.
func handleSecTestHistory(args SecTestArgs) error {
	tester := security.GlobalSecurityTester()
	history := tester.GetTestHistory()

	if len(history) == 0 {
		if args.JSON {
			return outputJSON(map[string]interface{}{
				"history": []interface{}{},
				"message": "No test history available",
			})
		}
		fmt.Println()
		fmt.Println(sectestDimStyle.Render("No test history available."))
		fmt.Println()
		return nil
	}

	if args.JSON {
		return outputJSON(map[string]interface{}{
			"history": history,
			"count":   len(history),
		})
	}

	// Display history
	fmt.Println()
	fmt.Println(sectestTitleStyle.Render("Security Test History"))
	fmt.Println(sectestSeparatorStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	for i, report := range history {
		fmt.Printf("%d. %s - %s\n",
			i+1,
			report.ReportID,
			report.GeneratedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("   Tests: %d | Passed: %s | Failed: %s | Findings: %d\n",
			report.Summary.TotalTests,
			sectestGreenStyle.Render(fmt.Sprintf("%d", report.Summary.Passed)),
			sectestRedStyle.Render(fmt.Sprintf("%d", report.Summary.Failed)),
			report.Summary.TotalFindings)
		fmt.Println()
	}

	return nil
}

// =============================================================================
// SECTEST STATUS
// =============================================================================

// handleSecTestStatus shows the last test results.
func handleSecTestStatus(args SecTestArgs) error {
	tester := security.GlobalSecurityTester()
	report := tester.GetLatestReport()

	if report == nil {
		if args.JSON {
			return outputJSON(map[string]interface{}{
				"error":   "no test results available",
				"message": "Run 'rigrun sectest run' first",
			})
		}
		fmt.Println()
		fmt.Println(sectestYellowStyle.Render("No test results available."))
		fmt.Println("Run 'rigrun sectest run' first to generate test results.")
		fmt.Println()
		return nil
	}

	if args.JSON {
		return outputTestReportJSON(report)
	}

	displayTestReport(report)
	return nil
}

// =============================================================================
// DISPLAY FUNCTIONS
// =============================================================================

// displayTestResult displays a single test result.
func displayTestResult(result *security.TestResult) {
	fmt.Println()
	fmt.Println(sectestTitleStyle.Render(fmt.Sprintf("Test: %s", result.TestName)))
	fmt.Println(sectestSeparatorStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	// Status with color
	var statusText string
	switch result.Status {
	case security.TestStatusPass:
		statusText = sectestGreenStyle.Render("[PASS]")
	case security.TestStatusFail:
		statusText = sectestRedStyle.Render("[FAIL]")
	case security.TestStatusWarn:
		statusText = sectestYellowStyle.Render("[WARN]")
	case security.TestStatusSkip:
		statusText = sectestDimStyle.Render("[SKIP]")
	}

	fmt.Printf("%s%s\n", sectestLabelStyle.Render("Status:"), statusText)
	fmt.Printf("%s%s\n", sectestLabelStyle.Render("Type:"), sectestValueStyle.Render(string(result.TestType)))
	fmt.Printf("%s%dms\n", sectestLabelStyle.Render("Duration:"), result.Duration)

	if result.Error != "" {
		fmt.Printf("%s%s\n", sectestLabelStyle.Render("Error:"), sectestRedStyle.Render(result.Error))
	}

	// Display findings
	if len(result.Findings) > 0 {
		fmt.Println()
		fmt.Println(sectestSectionStyle.Render(fmt.Sprintf("Findings (%d)", len(result.Findings))))
		fmt.Println()

		for i, finding := range result.Findings {
			// Severity with color
			var sevStyle lipgloss.Style
			switch finding.Severity {
			case "HIGH":
				sevStyle = sectestRedStyle
			case "MEDIUM":
				sevStyle = sectestYellowStyle
			case "LOW":
				sevStyle = sectestValueStyle
			default:
				sevStyle = sectestDimStyle
			}

			fmt.Printf("%d. [%s] %s\n",
				i+1,
				sevStyle.Render(finding.Severity),
				finding.Category)
			fmt.Printf("   %s\n", finding.Description)

			if finding.File != "" {
				loc := finding.File
				if finding.Line > 0 {
					loc = fmt.Sprintf("%s:%d", loc, finding.Line)
				}
				fmt.Printf("   %s%s\n", sectestDimStyle.Render("Location: "), loc)
			}

			if finding.Code != "" {
				fmt.Printf("   %s%s\n", sectestDimStyle.Render("Code: "), finding.Code)
			}

			if finding.Remediation != "" {
				fmt.Printf("   %s%s\n", sectestDimStyle.Render("Fix: "), finding.Remediation)
			}

			fmt.Println()
		}
	} else {
		fmt.Println()
		fmt.Println(sectestGreenStyle.Render("No findings - all checks passed!"))
		fmt.Println()
	}
}

// displayTestReport displays a full test report.
func displayTestReport(report *security.TestReport) {
	fmt.Println()
	fmt.Println(sectestTitleStyle.Render("SA-11 Security Test Report"))
	fmt.Println(sectestSeparatorStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	// Summary section
	fmt.Println(sectestSectionStyle.Render("Summary"))
	fmt.Printf("%s%s\n", sectestLabelStyle.Render("Report ID:"), sectestDimStyle.Render(report.ReportID))
	fmt.Printf("%s%s\n", sectestLabelStyle.Render("Generated:"), sectestValueStyle.Render(report.GeneratedAt.Format("2006-01-02 15:04:05")))
	fmt.Println()

	fmt.Printf("%s%d\n", sectestLabelStyle.Render("Total Tests:"), report.Summary.TotalTests)
	fmt.Printf("%s%s\n", sectestLabelStyle.Render("Passed:"), sectestGreenStyle.Render(fmt.Sprintf("%d", report.Summary.Passed)))
	fmt.Printf("%s%s\n", sectestLabelStyle.Render("Failed:"), sectestRedStyle.Render(fmt.Sprintf("%d", report.Summary.Failed)))
	fmt.Printf("%s%s\n", sectestLabelStyle.Render("Warnings:"), sectestYellowStyle.Render(fmt.Sprintf("%d", report.Summary.Warnings)))
	fmt.Printf("%s%d\n", sectestLabelStyle.Render("Skipped:"), report.Summary.Skipped)
	fmt.Println()

	fmt.Printf("%s%d\n", sectestLabelStyle.Render("Total Findings:"), report.Summary.TotalFindings)
	fmt.Printf("%s%s\n", sectestLabelStyle.Render("  High:"), sectestRedStyle.Render(fmt.Sprintf("%d", report.Summary.HighSeverity)))
	fmt.Printf("%s%s\n", sectestLabelStyle.Render("  Medium:"), sectestYellowStyle.Render(fmt.Sprintf("%d", report.Summary.MedSeverity)))
	fmt.Printf("%s%d\n", sectestLabelStyle.Render("  Low:"), report.Summary.LowSeverity)
	fmt.Println()

	// Test results section
	fmt.Println(sectestSectionStyle.Render("Test Results"))
	fmt.Println()

	for _, result := range report.Results {
		var statusText string
		switch result.Status {
		case security.TestStatusPass:
			statusText = sectestGreenStyle.Render("[PASS]")
		case security.TestStatusFail:
			statusText = sectestRedStyle.Render("[FAIL]")
		case security.TestStatusWarn:
			statusText = sectestYellowStyle.Render("[WARN]")
		case security.TestStatusSkip:
			statusText = sectestDimStyle.Render("[SKIP]")
		}

		fmt.Printf("%s %s", statusText, result.TestName)
		if len(result.Findings) > 0 {
			fmt.Printf(" (%d findings)", len(result.Findings))
		}
		fmt.Println()

		if result.Error != "" {
			fmt.Printf("  %s\n", sectestRedStyle.Render("Error: "+result.Error))
		}

		// Show high severity findings inline
		for _, f := range result.Findings {
			if f.Severity == "HIGH" {
				fmt.Printf("  %s %s: %s\n",
					sectestRedStyle.Render("[HIGH]"),
					f.Category,
					f.Description)
			}
		}
	}

	fmt.Println()

	// Final status
	if report.Summary.Failed > 0 {
		fmt.Println(sectestRedStyle.Render("Security tests FAILED - review findings above"))
	} else if report.Summary.Warnings > 0 {
		fmt.Println(sectestYellowStyle.Render("Security tests passed with warnings"))
	} else {
		fmt.Println(sectestGreenStyle.Render("All security tests PASSED"))
	}
	fmt.Println()
}

// =============================================================================
// JSON OUTPUT
// =============================================================================

// outputTestResultJSON outputs a test result as JSON.
func outputTestResultJSON(result *security.TestResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// outputTestReportJSON outputs a test report as JSON.
func outputTestReportJSON(report *security.TestReport) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
