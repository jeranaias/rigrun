// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// spillage.go - NIST 800-53 IR-9: Information Spillage Response implementation.
//
// Provides detection and response capabilities for information spillage events
// where classified information appears in unauthorized locations. Supports
// classification marker detection, spillage response procedures, and
// integration with incident reporting.
//
// SECURITY ENHANCEMENTS (2025-01):
//
// This implementation includes robust bypass prevention mechanisms:
//
// 1. Unicode Normalization (normalizeForDetection):
//    - Converts Unicode text to normalized form (NFKD) to handle accented characters
//    - Removes combining marks and diacritics
//    - Prevents bypasses using Unicode substitution (e.g., Cyrillic T vs Latin T)
//
// 2. Lookalike Character Detection:
//    - Replaces visually similar characters (0→O, 1→I, 3→E, etc.)
//    - Handles Cyrillic lookalikes (А→A, Т→T, О→O, etc.)
//    - Handles Greek lookalikes (Α→A, Τ→T, Ο→O, etc.)
//    - Removes separators that might be used for bypasses (_,-,., )
//
// 3. Entropy Analysis (calculateEntropy):
//    - Implements Shannon entropy calculation
//    - Detects high-entropy strings (>4.5 bits) that may be secrets/keys/tokens
//    - Complements pattern matching with content-based detection
//
// 4. Backup Before Sanitization (createBackup):
//    - Creates timestamped backups in secure directory (.spillage-backup)
//    - Preserves original content before any modifications
//    - Uses restrictive permissions (0600/0700)
//    - Logs backup creation for audit trail
//
// 5. Fuzzy Matching for Sanitization (findAndRedactFuzzy):
//    - Uses character class patterns to catch variations
//    - Handles mixed normal/obfuscated text in sanitization
//
// These enhancements prevent common bypass techniques:
// - "T0P SECRET" (zero instead of O)
// - "ТOP SECRET" (Cyrillic T)
// - "TOP_SECRET" (underscore separator)
// - Mixed character set obfuscation

package security

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// =============================================================================
// CONSTANTS
// =============================================================================

// Spillage actions
const (
	SpillageActionWarn     = "warn"     // Log warning but continue
	SpillageActionBlock    = "block"    // Block the operation
	SpillageActionSanitize = "sanitize" // Auto-sanitize the content
)

// Entropy threshold for high-entropy string detection (likely secrets/keys)
const EntropyThreshold = 4.5

// Backup directory for original content before sanitization
const SpillageBackupDir = ".spillage-backup"

// =============================================================================
// SPILLAGE TYPES
// =============================================================================

// SpillagePattern defines a pattern to detect classification markers.
type SpillagePattern struct {
	Name           string         `json:"name"`           // Human-readable name
	Pattern        *regexp.Regexp `json:"-"`              // Compiled pattern
	PatternString  string         `json:"pattern"`        // Pattern string for serialization
	Classification string         `json:"classification"` // Classification level
	Action         string         `json:"action"`         // warn, block, sanitize
}

// SpillageEvent represents a detected spillage incident.
type SpillageEvent struct {
	Timestamp      time.Time `json:"timestamp"`
	Location       string    `json:"location"`       // File path or "memory"
	PatternName    string    `json:"pattern_name"`   // Which pattern matched
	MatchedText    string    `json:"matched_text"`   // The actual text that matched
	Classification string    `json:"classification"` // Detected classification level
	Action         string    `json:"action"`         // Action taken
	Sanitized      bool      `json:"sanitized"`      // Whether content was sanitized
	LineNumber     int       `json:"line_number"`    // Line number where found (if file)
	DetectionType  string    `json:"detection_type"` // "pattern", "entropy", or "combined"
	Entropy        float64   `json:"entropy"`        // Entropy value if detected by entropy analysis
}

// SpillageReport summarizes spillage scan results.
type SpillageReport struct {
	ScanTime       time.Time       `json:"scan_time"`
	Location       string          `json:"location"` // Path that was scanned
	FilesScanned   int             `json:"files_scanned"`
	EventsFound    int             `json:"events_found"`
	Events         []SpillageEvent `json:"events"`
	IncidentID     string          `json:"incident_id,omitempty"` // Auto-created incident
	ActionsTaken   []string        `json:"actions_taken"`
}

// =============================================================================
// DEFAULT PATTERNS
// =============================================================================

// DefaultSpillagePatterns contains common classification markers to detect.
// These patterns identify potential spillage of classified information.
// NOTE: Patterns will be applied to normalized content (no spaces/separators).
// Patterns are ordered from most specific to least specific to avoid substring issues.
var DefaultSpillagePatterns = []SpillagePattern{
	{
		Name:           "TOP SECRET",
		PatternString:  `TOPSECRET`,
		Classification: "TOP SECRET",
		Action:         SpillageActionBlock,
	},
	{
		Name:           "NATO Markings",
		PatternString:  `NATO(SECRET|CONFIDENTIAL|RESTRICTED)`,
		Classification: "NATO",
		Action:         SpillageActionBlock,
	},
	{
		Name:           "SECRET with Caveats",
		PatternString:  `SECRET.*(NOFORN|RELTO)`,
		Classification: "SECRET",
		Action:         SpillageActionBlock,
	},
	{
		Name:           "Classification Marking",
		PatternString:  `(SECRET|CONFIDENTIAL)[A-Z]+`,
		Classification: "CLASSIFIED",
		Action:         SpillageActionBlock,
	},
	{
		Name:           "CONFIDENTIAL",
		PatternString:  `\bCONFIDENTIAL\b`,
		Classification: "CONFIDENTIAL",
		Action:         SpillageActionWarn,
	},
	{
		Name:           "SECRET",
		PatternString:  `\bSECRET\b`,
		Classification: "SECRET",
		Action:         SpillageActionBlock,
	},
	{
		Name:           "CUI Controlled",
		PatternString:  `\bCUI\b|CONTROLLEDUNCLASSIFIEDINFORMATION`,
		Classification: "CUI",
		Action:         SpillageActionWarn,
	},
	{
		Name:           "SCI Markings",
		PatternString:  `\b(SI|TK|HCS|COMINT|GAMMA)\b`,
		Classification: "SCI",
		Action:         SpillageActionBlock,
	},
}

// =============================================================================
// SPILLAGE MANAGER
// =============================================================================

// SpillageManager handles spillage detection and response per NIST 800-53 IR-9.
type SpillageManager struct {
	patterns        []SpillagePattern
	sanitizer       *DataSanitizer
	incidentManager *IncidentManager
	mu              sync.RWMutex
	enabled         bool
	defaultAction   string // Default action when spillage is detected
}

// ERROR HANDLING: Errors must not be silently ignored

// NewSpillageManager creates a new spillage manager with default patterns.
func NewSpillageManager() *SpillageManager {
	patterns := make([]SpillagePattern, 0, len(DefaultSpillagePatterns))

	// Compile default patterns
	for _, p := range DefaultSpillagePatterns {
		compiled, err := regexp.Compile(p.PatternString)
		if err != nil {
			// Log pattern compilation errors - do not silently skip
			fmt.Fprintf(os.Stderr, "SPILLAGE WARNING: invalid spillage pattern %q (%s): %v\n", p.PatternString, p.Name, err)
			continue
		}
		patterns = append(patterns, SpillagePattern{
			Name:           p.Name,
			Pattern:        compiled,
			PatternString:  p.PatternString,
			Classification: p.Classification,
			Action:         p.Action,
		})
	}

	return &SpillageManager{
		patterns:      patterns,
		sanitizer:     NewDataSanitizer(),
		enabled:       true,
		defaultAction: SpillageActionWarn,
	}
}

// SetIncidentManager sets the incident manager for auto-reporting.
func (s *SpillageManager) SetIncidentManager(m *IncidentManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.incidentManager = m
}

// SetEnabled enables or disables spillage detection.
func (s *SpillageManager) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
}

// SetDefaultAction sets the default action for spillage events.
func (s *SpillageManager) SetDefaultAction(action string) error {
	if action != SpillageActionWarn && action != SpillageActionBlock && action != SpillageActionSanitize {
		return fmt.Errorf("invalid action '%s', must be warn, block, or sanitize", action)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaultAction = action
	return nil
}

// AddPattern adds a custom spillage detection pattern.
func (s *SpillageManager) AddPattern(pattern SpillagePattern) error {
	compiled, err := regexp.Compile(pattern.PatternString)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pattern.Pattern = compiled
	s.patterns = append(s.patterns, pattern)
	return nil
}

// =============================================================================
// DETECTION
// =============================================================================

// Detect scans content for classification markers and returns any spillage events.
// Uses Unicode normalization and lookalike detection to prevent bypass attempts.
func (s *SpillageManager) Detect(content string) []SpillageEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.enabled || content == "" {
		return nil
	}

	var events []SpillageEvent
	now := time.Now()

	// Normalize content to detect bypass attempts using Unicode substitution
	normalizedContent := normalizeForDetection(content)

	// Track already-matched positions to avoid duplicate detections of substrings
	matchedRanges := make([]struct{ start, end int }, 0)

	// Pattern-based detection on normalized content
	for _, pattern := range s.patterns {
		// Find all match indices
		matches := pattern.Pattern.FindAllStringIndex(normalizedContent, -1)
		for _, matchIdx := range matches {
			start, end := matchIdx[0], matchIdx[1]

			// Check if this range overlaps with already-matched ranges
			overlaps := false
			for _, mr := range matchedRanges {
				if (start >= mr.start && start < mr.end) || (end > mr.start && end <= mr.end) {
					overlaps = true
					break
				}
			}

			if overlaps {
				continue // Skip this match as it's part of a more specific pattern
			}

			// Record this match range
			matchedRanges = append(matchedRanges, struct{ start, end int }{start, end})

			match := normalizedContent[start:end]
			event := SpillageEvent{
				Timestamp:      now,
				Location:       "memory",
				PatternName:    pattern.Name,
				MatchedText:    truncateMatch(match, 50),
				Classification: pattern.Classification,
				Action:         pattern.Action,
				Sanitized:      false,
				DetectionType:  "pattern",
			}
			events = append(events, event)
		}
	}

	// Entropy-based detection for potential secrets/keys
	highEntropyStrings := detectHighEntropyStrings(content, 16)
	for _, str := range highEntropyStrings {
		entropy := calculateEntropy(str)
		event := SpillageEvent{
			Timestamp:      now,
			Location:       "memory",
			PatternName:    "High Entropy String",
			MatchedText:    truncateMatch(str, 50),
			Classification: "POTENTIAL SECRET",
			Action:         SpillageActionWarn,
			Sanitized:      false,
			DetectionType:  "entropy",
			Entropy:        entropy,
		}
		events = append(events, event)
	}

	// Log detection if events found
	if len(events) > 0 {
		AuditLogEvent("", "SPILLAGE_DETECTED", map[string]string{
			"location": "memory",
			"count":    fmt.Sprintf("%d", len(events)),
		})
	}

	return events
}

// DetectInFile scans a file for classification markers.
// Uses Unicode normalization and entropy detection to prevent bypasses.
func (s *SpillageManager) DetectInFile(path string) ([]SpillageEvent, error) {
	s.mu.RLock()
	enabled := s.enabled
	patterns := s.patterns
	s.mu.RUnlock()

	if !enabled {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var events []SpillageEvent
	now := time.Now()
	lineNum := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Normalize line for pattern detection
		normalizedLine := normalizeForDetection(line)

		// Pattern-based detection
		for _, pattern := range patterns {
			matches := pattern.Pattern.FindAllString(normalizedLine, -1)
			for _, match := range matches {
				event := SpillageEvent{
					Timestamp:      now,
					Location:       path,
					PatternName:    pattern.Name,
					MatchedText:    truncateMatch(match, 50),
					Classification: pattern.Classification,
					Action:         pattern.Action,
					Sanitized:      false,
					LineNumber:     lineNum,
					DetectionType:  "pattern",
				}
				events = append(events, event)
			}
		}

		// Entropy-based detection on original line
		highEntropyStrings := detectHighEntropyStrings(line, 16)
		for _, str := range highEntropyStrings {
			entropy := calculateEntropy(str)
			event := SpillageEvent{
				Timestamp:      now,
				Location:       path,
				PatternName:    "High Entropy String",
				MatchedText:    truncateMatch(str, 50),
				Classification: "POTENTIAL SECRET",
				Action:         SpillageActionWarn,
				Sanitized:      false,
				LineNumber:     lineNum,
				DetectionType:  "entropy",
				Entropy:        entropy,
			}
			events = append(events, event)
		}
	}

	if err := scanner.Err(); err != nil {
		return events, fmt.Errorf("error reading file: %w", err)
	}

	// Log detection if events found
	if len(events) > 0 {
		AuditLogEvent("", "SPILLAGE_DETECTED", map[string]string{
			"location": path,
			"count":    fmt.Sprintf("%d", len(events)),
		})
	}

	return events, nil
}

// ScanDirectory recursively scans a directory for spillage.
func (s *SpillageManager) ScanDirectory(path string) (*SpillageReport, error) {
	s.mu.RLock()
	enabled := s.enabled
	s.mu.RUnlock()

	if !enabled {
		return nil, nil
	}

	report := &SpillageReport{
		ScanTime: time.Now(),
		Location: path,
		Events:   make([]SpillageEvent, 0),
	}

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories and binary files
		if info.IsDir() {
			return nil
		}

		// Only scan text-like files
		if !isTextFile(filePath) {
			return nil
		}

		report.FilesScanned++

		events, err := s.DetectInFile(filePath)
		if err != nil {
			return nil // Continue scanning despite errors
		}

		report.Events = append(report.Events, events...)
		return nil
	})

	if err != nil {
		return report, err
	}

	report.EventsFound = len(report.Events)

	// Auto-create incident if spillage found
	if report.EventsFound > 0 {
		report.IncidentID = s.createSpillageIncident(report)
	}

	return report, nil
}

// =============================================================================
// RESPONSE
// =============================================================================

// Sanitize removes classified content from a string.
// Note: For memory content, no backup is created. Use SanitizeFile for file-based content with backup.
func (s *SpillageManager) Sanitize(content string) string {
	s.mu.RLock()
	enabled := s.enabled
	patterns := s.patterns
	s.mu.RUnlock()

	if !enabled || content == "" {
		return content
	}

	result := content
	normalizedContent := normalizeForDetection(content)

	// Detect patterns on normalized content
	for _, pattern := range patterns {
		if pattern.Pattern.MatchString(normalizedContent) {
			// Use fuzzy matching to redact in original content
			result = findAndRedactFuzzy(result, pattern.Name, pattern.Classification)
		}
	}

	// Redact high-entropy strings (potential secrets)
	highEntropyStrings := detectHighEntropyStrings(result, 16)
	for _, str := range highEntropyStrings {
		result = strings.ReplaceAll(result, str, "[REDACTED-POTENTIAL-SECRET]")
	}

	// Log sanitization
	if result != content {
		AuditLogEvent("", "SPILLAGE_SANITIZED", map[string]string{
			"location": "memory",
		})
	}

	return result
}

// findAndRedactFuzzy finds text matching a pattern name and redacts it, handling variations.
func findAndRedactFuzzy(content, patternName, classification string) string {
	// Create a regex that matches common variations
	var pattern *regexp.Regexp

	switch patternName {
	case "TOP SECRET":
		// Match TOP SECRET with various separators and substitutions
		pattern = regexp.MustCompile(`(?i)T[O0][P7][\s_\-\.]*S[E3]CR[E3]T`)
	case "SECRET":
		pattern = regexp.MustCompile(`(?i)S[E3]CR[E3]T`)
	case "CONFIDENTIAL":
		pattern = regexp.MustCompile(`(?i)C[O0]NF[I1]D[E3]NT[I1][A4]L`)
	case "CUI Controlled":
		pattern = regexp.MustCompile(`(?i)CU[I1]|C[O0]NTR[O0]LL[E3]D[\s_\-\.]*UNCL[A4]SS[I1]F[I1][E3]D[\s_\-\.]*[I1]NF[O0]RM[A4]T[I1][O0]N`)
	case "NATO Markings":
		pattern = regexp.MustCompile(`(?i)N[A4]T[O0][\s_\-\.]*(S[E3]CR[E3]T|C[O0]NF[I1]D[E3]NT[I1][A4]L|R[E3]STR[I1]CT[E3]D)`)
	case "SCI Markings":
		pattern = regexp.MustCompile(`(?i)(S[I1]|TK|HCS|C[O0]M[I1]NT|G[A4]MM[A4])`)
	default:
		// Generic pattern - match the classification
		pattern = regexp.MustCompile(`(?i)` + regexp.QuoteMeta(classification))
	}

	if pattern != nil {
		return pattern.ReplaceAllString(content, "[REDACTED-"+classification+"]")
	}

	return content
}

// SanitizeFile sanitizes a file in place, creating a backup first.
// The backup is stored in a secure location before any modifications are made.
func (s *SpillageManager) SanitizeFile(path string) error {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Sanitize content
	original := string(data)
	sanitized := s.Sanitize(original)

	// If no changes, nothing to do
	if sanitized == original {
		return nil
	}

	// Create backup before modifying original
	backupPath, err := s.createBackup(path, data)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Write sanitized content
	if err := os.WriteFile(path, []byte(sanitized), 0600); err != nil {
		return fmt.Errorf("failed to write sanitized file: %w", err)
	}

	// Log the file sanitization with backup location
	AuditLogEvent("", "SPILLAGE_SANITIZED", map[string]string{
		"location": path,
		"backup":   backupPath,
	})

	return nil
}

// createBackup creates a secure backup of file content before sanitization.
// Returns the path to the backup file.
func (s *SpillageManager) createBackup(originalPath string, content []byte) (string, error) {
	// Create backup directory if it doesn't exist
	backupDir := filepath.Join(filepath.Dir(originalPath), SpillageBackupDir)
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	baseFilename := filepath.Base(originalPath)
	backupFilename := fmt.Sprintf("%s.%s.backup", baseFilename, timestamp)
	backupPath := filepath.Join(backupDir, backupFilename)

	// Write backup with restricted permissions
	if err := os.WriteFile(backupPath, content, 0600); err != nil {
		return "", fmt.Errorf("failed to write backup file: %w", err)
	}

	// Log backup creation
	AuditLogEvent("", "SPILLAGE_BACKUP_CREATED", map[string]string{
		"original": originalPath,
		"backup":   backupPath,
	})

	return backupPath, nil
}

// SecureDelete securely deletes a file (overwrite + delete).
func (s *SpillageManager) SecureDelete(path string) error {
	if s.sanitizer == nil {
		return fmt.Errorf("sanitizer not initialized")
	}
	return s.sanitizer.SecureDeleteFile(path)
}

// SpillageResponse executes the complete IR-9 spillage response procedure.
// This is the main entry point for responding to a detected spillage event.
func (s *SpillageManager) SpillageResponse(report *SpillageReport) error {
	if report == nil || len(report.Events) == 0 {
		return nil
	}

	// Step 1: Log spillage event with full details
	AuditLogEvent("", "SPILLAGE_RESPONSE_INITIATED", map[string]string{
		"location":    report.Location,
		"event_count": fmt.Sprintf("%d", report.EventsFound),
	})
	report.ActionsTaken = append(report.ActionsTaken, "Spillage response initiated")

	// Step 2: Auto-create incident report if not already done
	if report.IncidentID == "" {
		report.IncidentID = s.createSpillageIncident(report)
	}
	report.ActionsTaken = append(report.ActionsTaken, "Incident created: "+report.IncidentID)

	// Step 3: Sanitize affected data based on action type
	for _, event := range report.Events {
		if event.Location != "memory" && event.Action == SpillageActionSanitize {
			if err := s.SanitizeFile(event.Location); err != nil {
				// Log error but continue
				AuditLogEvent("", "SPILLAGE_SANITIZE_ERROR", map[string]string{
					"location": event.Location,
					"error":    err.Error(),
				})
			} else {
				report.ActionsTaken = append(report.ActionsTaken, "Sanitized: "+event.Location)
			}
		}
	}

	// Step 4: Clear relevant caches and session data
	if s.sanitizer != nil {
		if err := s.sanitizer.SanitizeCache(); err != nil {
			AuditLogEvent("", "SPILLAGE_CACHE_CLEAR_ERROR", map[string]string{
				"error": err.Error(),
			})
		} else {
			report.ActionsTaken = append(report.ActionsTaken, "Cache cleared")
		}
	}

	// Step 5: Generate spillage report for security team
	// (the report itself is returned - caller can export it)
	report.ActionsTaken = append(report.ActionsTaken, "Spillage response completed")

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// normalizeForDetection normalizes text to ASCII and replaces lookalike characters
// to prevent bypass attempts using Unicode substitution or character replacement.
func normalizeForDetection(s string) string {
	// First normalize Unicode to decomposed form (NFKD) and strip accents
	t := transform.Chain(norm.NFKD)
	normalized, _, err := transform.String(t, s)
	if err != nil {
		normalized = s // Fallback to original on error
	}

	// Remove combining marks (accents, diacritics)
	var result strings.Builder
	for _, r := range normalized {
		// Skip combining marks (Unicode category Mn)
		if r < 0x300 || r > 0x36f {
			result.WriteRune(r)
		}
	}
	normalized = result.String()

	// Convert to uppercase for case-insensitive matching
	normalized = strings.ToUpper(normalized)

	// Replace common lookalike characters
	// Numbers that look like letters
	replacer := strings.NewReplacer(
		// Numbers to letters
		"0", "O",
		"1", "I",
		"3", "E",
		"4", "A",
		"5", "S",
		"7", "T",
		"8", "B",
		// Special characters to letters
		"@", "A",
		"$", "S",
		"!", "I",
		"|", "I",
		"(", "C",
		"[", "C",
		// Cyrillic lookalikes (commonly used for bypasses)
		"А", "A", // Cyrillic A
		"В", "B", // Cyrillic Ve
		"С", "C", // Cyrillic Es
		"Е", "E", // Cyrillic Ye
		"К", "K", // Cyrillic Ka
		"М", "M", // Cyrillic Em
		"Н", "H", // Cyrillic En
		"О", "O", // Cyrillic O
		"Р", "P", // Cyrillic Er
		"Т", "T", // Cyrillic Te
		"Х", "X", // Cyrillic Ha
		"Ү", "Y", // Cyrillic U
		// Greek lookalikes
		"Α", "A", // Greek Alpha
		"Β", "B", // Greek Beta
		"Ε", "E", // Greek Epsilon
		"Ι", "I", // Greek Iota
		"Κ", "K", // Greek Kappa
		"Μ", "M", // Greek Mu
		"Ν", "N", // Greek Nu
		"Ο", "O", // Greek Omicron
		"Ρ", "P", // Greek Rho
		"Τ", "T", // Greek Tau
		"Υ", "Y", // Greek Upsilon
		"Χ", "X", // Greek Chi
		// Remove common separators that might be used for bypasses
		"_", "",
		"-", "",
		".", "",
		" ", "",
	)

	return replacer.Replace(normalized)
}

// calculateEntropy calculates the Shannon entropy of a string.
// High entropy (> 4.5) typically indicates random data like keys, tokens, or encrypted content.
func calculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0.0
	}

	freq := make(map[rune]int)
	for _, r := range s {
		freq[r]++
	}

	var entropy float64
	length := float64(len(s))
	for _, count := range freq {
		p := float64(count) / length
		entropy -= p * math.Log2(p)
	}

	return entropy
}

// detectHighEntropyStrings finds potential secrets/keys based on entropy analysis.
// Returns substrings with entropy above threshold.
func detectHighEntropyStrings(content string, minLength int) []string {
	if minLength < 16 {
		minLength = 16 // Minimum length for meaningful entropy analysis
	}

	var highEntropyStrings []string
	seen := make(map[string]bool) // Avoid duplicates

	// Split on whitespace and common separators
	words := strings.FieldsFunc(content, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ',' || r == ';'
	})

	for _, word := range words {
		// Remove common punctuation from ends
		word = strings.Trim(word, "\"'`()[]{},.!?;:")

		// Skip short strings
		if len(word) < minLength {
			continue
		}

		// Skip if already seen
		if seen[word] {
			continue
		}

		entropy := calculateEntropy(word)
		if entropy > EntropyThreshold {
			highEntropyStrings = append(highEntropyStrings, word)
			seen[word] = true
		}
	}

	return highEntropyStrings
}

// createSpillageIncident creates an incident report for detected spillage.
func (s *SpillageManager) createSpillageIncident(report *SpillageReport) string {
	s.mu.RLock()
	incidentManager := s.incidentManager
	s.mu.RUnlock()

	if incidentManager == nil {
		incidentManager = GlobalIncidentManager()
	}

	if incidentManager == nil {
		return ""
	}

	// Determine severity based on highest classification found
	severity := SeverityMedium
	for _, event := range report.Events {
		switch event.Classification {
		case "TOP SECRET", "SCI":
			severity = SeverityCritical
		case "SECRET", "NATO":
			if severity != SeverityCritical {
				severity = SeverityHigh
			}
		case "CONFIDENTIAL", "CUI":
			if severity == SeverityMedium {
				severity = SeverityMedium
			}
		}
	}

	// Build description
	description := fmt.Sprintf(
		"Information spillage detected during scan of %s. Found %d classification markers. Classifications detected: %s",
		report.Location,
		report.EventsFound,
		summarizeClassifications(report.Events),
	)

	incident, err := incidentManager.Report(severity, CategorySpillage, description)
	if err != nil {
		return ""
	}

	return incident.ID
}

// truncateMatch truncates a match string for logging.
func truncateMatch(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// isTextFile checks if a file is likely a text file based on extension.
func isTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	textExtensions := map[string]bool{
		".txt": true, ".md": true, ".log": true, ".json": true,
		".yaml": true, ".yml": true, ".xml": true, ".html": true,
		".css": true, ".js": true, ".ts": true, ".go": true,
		".py": true, ".rs": true, ".c": true, ".h": true,
		".cpp": true, ".hpp": true, ".java": true, ".sh": true,
		".bat": true, ".ps1": true, ".toml": true, ".ini": true,
		".cfg": true, ".conf": true, ".env": true, ".sql": true,
	}
	return textExtensions[ext]
}

// summarizeClassifications returns a comma-separated list of unique classifications.
func summarizeClassifications(events []SpillageEvent) string {
	seen := make(map[string]bool)
	var classifications []string

	for _, e := range events {
		if !seen[e.Classification] {
			seen[e.Classification] = true
			classifications = append(classifications, e.Classification)
		}
	}

	return strings.Join(classifications, ", ")
}

// =============================================================================
// GLOBAL INSTANCE
// =============================================================================

var (
	globalSpillageManager     *SpillageManager
	globalSpillageManagerOnce sync.Once
)

// GlobalSpillageManager returns the global spillage manager instance.
func GlobalSpillageManager() *SpillageManager {
	globalSpillageManagerOnce.Do(func() {
		globalSpillageManager = NewSpillageManager()
	})
	return globalSpillageManager
}
