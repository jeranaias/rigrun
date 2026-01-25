// vulnscan.go - Vulnerability scanning for NIST 800-53 RA-5 compliance.
//
// Implements NIST 800-53 RA-5 (Vulnerability Monitoring and Scanning)
// for DoD IL5 compliance.
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package security

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// vulnScanTimeout is the default timeout for vulnerability scanning operations.
const vulnScanTimeout = 5 * time.Minute

// =============================================================================
// SEVERITY LEVELS
// =============================================================================

// Note: Severity types are defined in incident.go and shared across security package
// SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical, SeverityInfo

// =============================================================================
// VULNERABILITY STRUCTURE
// =============================================================================

// Vulnerability represents a discovered security vulnerability.
type Vulnerability struct {
	ID           string    `json:"id"`
	Severity     string    `json:"severity"`
	Component    string    `json:"component"`
	Description  string    `json:"description"`
	Remediation  string    `json:"remediation"`
	DiscoveredAt time.Time `json:"discovered_at"`
	CVE          string    `json:"cve,omitempty"`          // CVE identifier if applicable
	CVSS         float64   `json:"cvss,omitempty"`         // CVSS score if available
	References   []string  `json:"references,omitempty"`   // External references
}

// =============================================================================
// VULNERABILITY SCANNER
// =============================================================================

// VulnScanner performs vulnerability scanning and management.
type VulnScanner struct {
	mu             sync.RWMutex
	vulnerabilities []Vulnerability
	lastScanTime    time.Time
	scanSchedule    time.Duration
	stopSchedule    chan struct{}
}

// NewVulnScanner creates a new vulnerability scanner.
func NewVulnScanner() *VulnScanner {
	return &VulnScanner{
		vulnerabilities: make([]Vulnerability, 0),
		stopSchedule:    make(chan struct{}),
	}
}

// =============================================================================
// SCANNING METHODS
// =============================================================================

// ScanDependencies scans go.mod dependencies for known vulnerabilities.
// Uses govulncheck if available, falls back to pattern-based scanning.
func (vs *VulnScanner) ScanDependencies() ([]Vulnerability, error) {
	return vs.ScanDependenciesWithContext(context.Background())
}

// ScanDependenciesWithContext scans go.mod dependencies with context support.
// CANCELLATION: Context enables timeout and cancellation
func (vs *VulnScanner) ScanDependenciesWithContext(ctx context.Context) ([]Vulnerability, error) {
	// Create timeout context if none is set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, vulnScanTimeout)
		defer cancel()
	}

	vs.mu.Lock()
	defer vs.mu.Unlock()

	var vulns []Vulnerability

	// Find go.mod file
	goModPath, err := findGoMod()
	if err != nil {
		return nil, fmt.Errorf("failed to find go.mod: %w", err)
	}

	// Try govulncheck first (if available)
	govulnVulns, err := vs.scanWithGovulncheckWithContext(ctx, goModPath)
	if err == nil {
		vulns = append(vulns, govulnVulns...)
	}

	// Fallback: scan for known vulnerable versions
	patternVulns, err := vs.scanDependencyPatterns(goModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to scan dependencies: %w", err)
	}
	vulns = append(vulns, patternVulns...)

	// Add to tracked vulnerabilities
	vs.vulnerabilities = append(vs.vulnerabilities, vulns...)

	return vulns, nil
}

// scanWithGovulncheck uses the govulncheck tool if available.
func (vs *VulnScanner) scanWithGovulncheck(goModPath string) ([]Vulnerability, error) {
	return vs.scanWithGovulncheckWithContext(context.Background(), goModPath)
}

// scanWithGovulncheckWithContext uses the govulncheck tool with context support.
// CANCELLATION: Context enables timeout and cancellation
func (vs *VulnScanner) scanWithGovulncheckWithContext(ctx context.Context, goModPath string) ([]Vulnerability, error) {
	// Check if govulncheck is available
	_, err := exec.LookPath("govulncheck")
	if err != nil {
		return nil, fmt.Errorf("govulncheck not available")
	}

	// Run govulncheck with context
	workDir := filepath.Dir(goModPath)
	cmd := exec.CommandContext(ctx, "govulncheck", "-json", "./...")
	cmd.Dir = workDir

	output, err := cmd.Output()
	if err != nil {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		// govulncheck returns non-zero if vulnerabilities found
		// This is expected, so we continue parsing
	}

	return vs.parseGovulncheckOutput(output)
}

// parseGovulncheckOutput parses govulncheck JSON output.
func (vs *VulnScanner) parseGovulncheckOutput(output []byte) ([]Vulnerability, error) {
	var vulns []Vulnerability

	// Parse govulncheck JSON output (line-delimited JSON)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// Look for vulnerability entries
		if entry["vulncheck"] != nil {
			vuln := vs.parseGovulncheckEntry(entry)
			if vuln != nil {
				vulns = append(vulns, *vuln)
			}
		}
	}

	return vulns, nil
}

// parseGovulncheckEntry parses a single govulncheck entry.
func (vs *VulnScanner) parseGovulncheckEntry(entry map[string]interface{}) *Vulnerability {
	vulnData, ok := entry["vulncheck"].(map[string]interface{})
	if !ok {
		return nil
	}

	id := getStringField(vulnData, "id")
	if id == "" {
		return nil
	}

	severity := vs.cvssToSeverity(getFloatField(vulnData, "cvss"))

	return &Vulnerability{
		ID:           id,
		Severity:     severity,
		Component:    getStringField(vulnData, "package"),
		Description:  getStringField(vulnData, "details"),
		Remediation:  fmt.Sprintf("Update to fixed version: %s", getStringField(vulnData, "fixed")),
		DiscoveredAt: time.Now(),
		CVE:          id,
		CVSS:         getFloatField(vulnData, "cvss"),
	}
}

// scanDependencyPatterns scans go.mod for known vulnerable dependency patterns.
func (vs *VulnScanner) scanDependencyPatterns(goModPath string) ([]Vulnerability, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var vulns []Vulnerability
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Check for outdated dependencies
		if strings.Contains(line, "v0.0.") {
			vulns = append(vulns, Vulnerability{
				ID:           fmt.Sprintf("DEP-%d", lineNum),
				Severity:     SeverityLow,
				Component:    extractPackageName(line),
				Description:  "Dependency using v0.0.x version (unstable)",
				Remediation:  "Update to stable version",
				DiscoveredAt: time.Now(),
			})
		}

		// Check for known vulnerable versions (example patterns)
		// In production, this would check against a CVE database
		if strings.Contains(line, "golang.org/x/crypto") && strings.Contains(line, "v0.0.0-") {
			vulns = append(vulns, Vulnerability{
				ID:           fmt.Sprintf("CVE-CHECK-%d", lineNum),
				Severity:     SeverityMedium,
				Component:    "golang.org/x/crypto",
				Description:  "Using potentially outdated crypto library",
				Remediation:  "Update to latest stable version",
				DiscoveredAt: time.Now(),
			})
		}
	}

	return vulns, scanner.Err()
}

// ScanBinaries checks for unsafe patterns in compiled binaries.
// Note: This is a basic implementation. Production systems should use
// specialized tools like checksec, LIEF, or radare2.
func (vs *VulnScanner) ScanBinaries() ([]Vulnerability, error) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	var vulns []Vulnerability

	// Get executable path
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Check file permissions
	info, err := os.Stat(exePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat binary: %w", err)
	}

	// Check if binary is writable by others (security issue)
	mode := info.Mode()
	if mode.Perm()&0022 != 0 {
		vulns = append(vulns, Vulnerability{
			ID:           "BIN-001",
			Severity:     SeverityHigh,
			Component:    "Binary",
			Description:  "Executable has insecure permissions (writable by group/others)",
			Remediation:  "Run: chmod 755 " + exePath,
			DiscoveredAt: time.Now(),
		})
	}

	// Check for debug symbols (information disclosure)
	// This is a simplified check - production would parse ELF/PE headers
	if info.Size() > 50*1024*1024 { // Unusually large binary
		vulns = append(vulns, Vulnerability{
			ID:           "BIN-002",
			Severity:     SeverityLow,
			Component:    "Binary",
			Description:  "Binary may contain debug symbols (large size)",
			Remediation:  "Strip binary: go build -ldflags='-s -w'",
			DiscoveredAt: time.Now(),
		})
	}

	vs.vulnerabilities = append(vs.vulnerabilities, vulns...)
	return vulns, nil
}

// ScanConfig checks configuration for insecure settings.
func (vs *VulnScanner) ScanConfig() ([]Vulnerability, error) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	var vulns []Vulnerability

	// Find config file
	configPath, err := findConfigFile()
	if err != nil {
		// Config file not found - not necessarily an error
		return nil, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	content := string(data)

	// Check for hardcoded credentials
	if vs.containsHardcodedCredentials(content) {
		vulns = append(vulns, Vulnerability{
			ID:           "CFG-001",
			Severity:     SeverityCritical,
			Component:    "Configuration",
			Description:  "Hardcoded credentials detected in configuration",
			Remediation:  "Remove hardcoded credentials, use environment variables or keystore",
			DiscoveredAt: time.Now(),
		})
	}

	// Check for insecure crypto usage
	if strings.Contains(content, "des") || strings.Contains(content, "DES") ||
		strings.Contains(content, "md5") || strings.Contains(content, "MD5") {
		vulns = append(vulns, Vulnerability{
			ID:           "CFG-002",
			Severity:     SeverityHigh,
			Component:    "Configuration",
			Description:  "Weak cryptographic algorithm configured (DES/MD5)",
			Remediation:  "Use FIPS 140-2 approved algorithms (AES-256, SHA-256)",
			DiscoveredAt: time.Now(),
		})
	}

	// Check for insecure TLS settings
	if strings.Contains(content, "tls_min_version") {
		if strings.Contains(content, "1.0") || strings.Contains(content, "1.1") {
			vulns = append(vulns, Vulnerability{
				ID:           "CFG-003",
				Severity:     SeverityHigh,
				Component:    "TLS Configuration",
				Description:  "Insecure TLS version configured (TLS 1.0/1.1)",
				Remediation:  "Set tls_min_version to 1.2 or higher",
				DiscoveredAt: time.Now(),
				References:   []string{"NIST SP 800-52r2"},
			})
		}
	}

	// Check for disabled security features
	if strings.Contains(content, "audit_enabled = false") ||
		strings.Contains(content, "audit_enabled=false") {
		vulns = append(vulns, Vulnerability{
			ID:           "CFG-004",
			Severity:     SeverityMedium,
			Component:    "Audit Logging",
			Description:  "Audit logging is disabled",
			Remediation:  "Enable audit logging for IL5 compliance (AU-2, AU-3)",
			DiscoveredAt: time.Now(),
			References:   []string{"NIST 800-53 AU-2", "NIST 800-53 AU-3"},
		})
	}

	vs.vulnerabilities = append(vs.vulnerabilities, vulns...)
	return vulns, nil
}

// containsHardcodedCredentials checks for common credential patterns.
func (vs *VulnScanner) containsHardcodedCredentials(content string) bool {
	// Patterns for common credential formats
	patterns := []string{
		`password\s*=\s*["'][^"']{4,}["']`,
		`api_key\s*=\s*["'][^"']{16,}["']`,
		`secret\s*=\s*["'][^"']{16,}["']`,
		`token\s*=\s*["'][^"']{16,}["']`,
		`sk-[a-zA-Z0-9]{20,}`, // OpenAI/Anthropic key pattern
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			// Exclude common placeholders
			if !strings.Contains(content, "your_password_here") &&
				!strings.Contains(content, "YOUR_API_KEY") &&
				!strings.Contains(content, "${") { // Environment variable
				return true
			}
		}
	}

	return false
}

// =============================================================================
// VULNERABILITY MANAGEMENT
// =============================================================================

// GetVulnerabilities returns all discovered vulnerabilities.
func (vs *VulnScanner) GetVulnerabilities() []Vulnerability {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	// Return a copy
	vulns := make([]Vulnerability, len(vs.vulnerabilities))
	copy(vulns, vs.vulnerabilities)
	return vulns
}

// GetVulnBySeverity filters vulnerabilities by severity level.
func (vs *VulnScanner) GetVulnBySeverity(severity string) []Vulnerability {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	var filtered []Vulnerability
	for _, vuln := range vs.vulnerabilities {
		if vuln.Severity == severity {
			filtered = append(filtered, vuln)
		}
	}
	return filtered
}

// GetVulnByComponent filters vulnerabilities by component.
func (vs *VulnScanner) GetVulnByComponent(component string) []Vulnerability {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	var filtered []Vulnerability
	for _, vuln := range vs.vulnerabilities {
		if strings.Contains(strings.ToLower(vuln.Component), strings.ToLower(component)) {
			filtered = append(filtered, vuln)
		}
	}
	return filtered
}

// ClearVulnerabilities clears all tracked vulnerabilities.
func (vs *VulnScanner) ClearVulnerabilities() {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vs.vulnerabilities = make([]Vulnerability, 0)
}

// GetLastScanTime returns the time of the last scan.
func (vs *VulnScanner) GetLastScanTime() time.Time {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.lastScanTime
}

// GetScanSummary returns a summary of vulnerabilities by severity.
func (vs *VulnScanner) GetScanSummary() map[string]int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	summary := make(map[string]int)
	for _, vuln := range vs.vulnerabilities {
		summary[vuln.Severity]++
	}
	return summary
}

// =============================================================================
// EXPORT METHODS
// =============================================================================

// ExportReport exports vulnerabilities in the specified format.
func (vs *VulnScanner) ExportReport(format string, output io.Writer) error {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	switch strings.ToLower(format) {
	case "json":
		return vs.exportJSON(output)
	case "csv":
		return vs.exportCSV(output)
	default:
		return fmt.Errorf("unsupported format: %s (supported: json, csv)", format)
	}
}

// exportJSON exports vulnerabilities as JSON.
func (vs *VulnScanner) exportJSON(output io.Writer) error {
	report := map[string]interface{}{
		"scan_time":        vs.lastScanTime.Format(time.RFC3339),
		"total_count":      len(vs.vulnerabilities),
		"summary":          vs.GetScanSummary(),
		"vulnerabilities":  vs.vulnerabilities,
	}

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// exportCSV exports vulnerabilities as CSV.
func (vs *VulnScanner) exportCSV(output io.Writer) error {
	writer := csv.NewWriter(output)
	defer writer.Flush()

	// Write header
	header := []string{"ID", "Severity", "Component", "Description", "Remediation", "CVE", "CVSS", "Discovered At"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write vulnerabilities
	for _, vuln := range vs.vulnerabilities {
		record := []string{
			vuln.ID,
			string(vuln.Severity),
			vuln.Component,
			vuln.Description,
			vuln.Remediation,
			vuln.CVE,
			fmt.Sprintf("%.1f", vuln.CVSS),
			vuln.Timestamp(),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// =============================================================================
// SCHEDULED SCANNING
// =============================================================================

// ScheduleScan schedules periodic vulnerability scans.
func (vs *VulnScanner) ScheduleScan(interval time.Duration) error {
	vs.mu.Lock()
	vs.scanSchedule = interval
	vs.mu.Unlock()

	// Start background scheduler
	go vs.runScheduler()

	return nil
}

// StopSchedule stops the scheduled scanning.
func (vs *VulnScanner) StopSchedule() {
	// BUGFIX: Add mutex to prevent double-close panic
	vs.mu.Lock()
	defer vs.mu.Unlock()

	select {
	case <-vs.stopSchedule:
		// Already closed
		return
	default:
		close(vs.stopSchedule)
	}
}

// runScheduler runs the scheduled scan loop.
func (vs *VulnScanner) runScheduler() {
	ticker := time.NewTicker(vs.scanSchedule)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Run full scan
			vs.ScanAll()
		case <-vs.stopSchedule:
			return
		}
	}
}

// ScanAll runs all vulnerability scans.
func (vs *VulnScanner) ScanAll() error {
	vs.mu.Lock()
	vs.lastScanTime = time.Now()
	vs.vulnerabilities = make([]Vulnerability, 0) // Clear old results
	vs.mu.Unlock()

	// Run all scans
	var allErrors []error

	if _, err := vs.ScanDependencies(); err != nil {
		allErrors = append(allErrors, fmt.Errorf("dependency scan failed: %w", err))
	}

	if _, err := vs.ScanBinaries(); err != nil {
		allErrors = append(allErrors, fmt.Errorf("binary scan failed: %w", err))
	}

	if _, err := vs.ScanConfig(); err != nil {
		allErrors = append(allErrors, fmt.Errorf("config scan failed: %w", err))
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("scan completed with errors: %v", allErrors)
	}

	return nil
}

// =============================================================================
// HELPER METHODS
// =============================================================================

// cvssToSeverity converts CVSS score to severity level.
func (vs *VulnScanner) cvssToSeverity(cvss float64) string {
	switch {
	case cvss >= 9.0:
		return SeverityCritical
	case cvss >= 7.0:
		return SeverityHigh
	case cvss >= 4.0:
		return SeverityMedium
	case cvss > 0:
		return SeverityLow
	default:
		return "info"
	}
}

// findGoMod finds the go.mod file in the project.
func findGoMod() (string, error) {
	// Start from current directory and walk up
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath, nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("go.mod not found")
}

// findConfigFile finds the rigrun configuration file.
func findConfigFile() (string, error) {
	// Check common locations
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	paths := []string{
		filepath.Join(home, ".rigrun", "config.toml"),
		filepath.Join(home, ".config", "rigrun", "config.toml"),
		"config.toml",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("config file not found")
}

// extractPackageName extracts package name from go.mod line.
func extractPackageName(line string) string {
	fields := strings.Fields(line)
	if len(fields) > 0 {
		return fields[0]
	}
	return "unknown"
}

// getStringField safely extracts string field from map.
func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// getFloatField safely extracts float field from map.
func getFloatField(m map[string]interface{}, key string) float64 {
	if val, ok := m[key].(float64); ok {
		return val
	}
	return 0
}

// Timestamp returns formatted timestamp string.
func (v *Vulnerability) Timestamp() string {
	return v.DiscoveredAt.Format("2006-01-02 15:04:05")
}

// SortBySeverity sorts vulnerabilities by severity (Critical -> Info).
func SortBySeverity(vulns []Vulnerability) {
	severityOrder := map[string]int{
		SeverityCritical: 0,
		SeverityHigh:     1,
		SeverityMedium:   2,
		SeverityLow:      3,
		"info":           4,
	}

	sort.Slice(vulns, func(i, j int) bool {
		return severityOrder[vulns[i].Severity] < severityOrder[vulns[j].Severity]
	})
}

// =============================================================================
// GLOBAL SCANNER
// =============================================================================

var (
	globalScanner     *VulnScanner
	globalScannerOnce sync.Once
)

// GlobalVulnScanner returns the global vulnerability scanner instance.
func GlobalVulnScanner() *VulnScanner {
	globalScannerOnce.Do(func() {
		globalScanner = NewVulnScanner()
	})
	return globalScanner
}

// =============================================================================
// TLS CONFIGURATION SCANNER
// =============================================================================

// ScanTLSConfig checks TLS configuration for security issues.
func (vs *VulnScanner) ScanTLSConfig() ([]Vulnerability, error) {
	var vulns []Vulnerability

	// Check system TLS configuration
	defaultConfig := &tls.Config{}

	// Check minimum TLS version
	if defaultConfig.MinVersion < tls.VersionTLS12 {
		vulns = append(vulns, Vulnerability{
			ID:           "TLS-001",
			Severity:     SeverityHigh,
			Component:    "TLS Configuration",
			Description:  "System default TLS version is below 1.2",
			Remediation:  "Configure MinVersion to tls.VersionTLS12 or higher",
			DiscoveredAt: time.Now(),
			References:   []string{"NIST SP 800-52r2"},
		})
	}

	return vulns, nil
}
