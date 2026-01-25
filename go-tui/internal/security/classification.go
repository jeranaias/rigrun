// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// DoD Classification Marking UI for IL5 Compliance
// Per DoDI 5200.48 and 32 CFR Part 2002

package security

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ClassificationLevel represents DoD information classification levels
type ClassificationLevel int

const (
	ClassificationUnclassified ClassificationLevel = iota
	ClassificationCUI                              // Controlled Unclassified Information
	ClassificationConfidential
	ClassificationSecret
	ClassificationTopSecret
)

// CUIDesignation represents CUI dissemination control markings
type CUIDesignation int

const (
	CUIBasic  CUIDesignation = iota
	CUINOFORN                // Not Releasable to Foreign Nationals
	CUIIMCON                 // Imagery Intelligence Control
	CUIPropin                // Proprietary Information
	CUIORCON                 // Originator Controlled
)

// Banner text constants per DoD marking standards
const (
	BannerUnclassified = "UNCLASSIFIED"
	BannerCUI          = "CUI"
	BannerConfidential = "CONFIDENTIAL"
	BannerSecret       = "SECRET"
	BannerTopSecret    = "TOP SECRET"
)

// Color constants for classification banners
const (
	ColorUnclassified = lipgloss.Color("#00FF00") // Green
	ColorCUI          = lipgloss.Color("#800080") // Purple
	ColorConfidential = lipgloss.Color("#0000FF") // Blue
	ColorSecret       = lipgloss.Color("#FF0000") // Red
	ColorTopSecret    = lipgloss.Color("#FFA500") // Orange
)

// Classification represents a complete security classification marking
type Classification struct {
	Level   ClassificationLevel
	CUI     []CUIDesignation // Multiple can apply
	Caveats []string         // Additional markings (e.g., NOFORN, RELTO)
	Portion bool             // Is this a portion marking?
}

// String returns the string representation of ClassificationLevel
func (c ClassificationLevel) String() string {
	switch c {
	case ClassificationUnclassified:
		return BannerUnclassified
	case ClassificationCUI:
		return BannerCUI
	case ClassificationConfidential:
		return BannerConfidential
	case ClassificationSecret:
		return BannerSecret
	case ClassificationTopSecret:
		return BannerTopSecret
	default:
		return BannerUnclassified
	}
}

// Color returns the appropriate lipgloss color for the classification level
func (c ClassificationLevel) Color() lipgloss.Color {
	switch c {
	case ClassificationUnclassified:
		return ColorUnclassified
	case ClassificationCUI:
		return ColorCUI
	case ClassificationConfidential:
		return ColorConfidential
	case ClassificationSecret:
		return ColorSecret
	case ClassificationTopSecret:
		return ColorTopSecret
	default:
		return ColorUnclassified
	}
}

// String returns the string representation of CUIDesignation
func (c CUIDesignation) String() string {
	switch c {
	case CUIBasic:
		return "BASIC"
	case CUINOFORN:
		return "NOFORN"
	case CUIIMCON:
		return "IMCON"
	case CUIPropin:
		return "PROPIN"
	case CUIORCON:
		return "ORCON"
	default:
		return "BASIC"
	}
}

// String returns the full marking string for a Classification
func (c Classification) String() string {
	var parts []string

	// Add the base classification level
	parts = append(parts, c.Level.String())

	// Add CUI designations if applicable
	if c.Level == ClassificationCUI && len(c.CUI) > 0 {
		for _, cui := range c.CUI {
			if cui != CUIBasic {
				parts = append(parts, cui.String())
			}
		}
	}

	// Add caveats
	parts = append(parts, c.Caveats...)

	return strings.Join(parts, "//")
}

// IsClassified returns true if the classification level is Confidential or above
func (c Classification) IsClassified() bool {
	return c.Level >= ClassificationConfidential
}

// DefaultClassification returns the default UNCLASSIFIED classification
func DefaultClassification() Classification {
	return Classification{
		Level:   ClassificationUnclassified,
		CUI:     nil,
		Caveats: nil,
		Portion: false,
	}
}

// GetSecurityBannerStyle returns the lipgloss style for a classification banner
func GetSecurityBannerStyle(level ClassificationLevel) lipgloss.Style {
	bgColor := level.Color()

	// Use contrasting foreground color for readability per DoDI 5200.48
	var fgColor lipgloss.Color
	switch level {
	case ClassificationUnclassified, ClassificationTopSecret:
		fgColor = lipgloss.Color("#000000") // Black text
	default:
		fgColor = lipgloss.Color("#FFFFFF") // White text
	}

	return lipgloss.NewStyle().
		Bold(true).
		Foreground(fgColor).
		Background(bgColor).
		Padding(0, 1)
}

// RenderTopBanner renders the classification banner for the top of screen
func RenderTopBanner(c Classification, width int) string {
	style := GetSecurityBannerStyle(c.Level)

	// Build the banner text
	bannerText := c.Level.String()

	// Add caveats if present
	if len(c.Caveats) > 0 {
		bannerText = bannerText + "//" + strings.Join(c.Caveats, "//")
	}

	// Center the text within the given width
	padding := (width - len(bannerText)) / 2
	if padding < 0 {
		padding = 0
	}

	// Create full-width banner
	fullBanner := strings.Repeat(" ", padding) + bannerText + strings.Repeat(" ", width-padding-len(bannerText))

	return style.Width(width).Align(lipgloss.Center).Render(bannerText + strings.Repeat(" ", max(0, width-len(fullBanner))))
}

// RenderBottomBanner renders the banner for bottom of screen, including CUI designations
func RenderBottomBanner(c Classification, width int) string {
	style := GetSecurityBannerStyle(c.Level)

	// Build the banner text
	bannerText := c.Level.String()

	// Add CUI designations if applicable
	if c.Level == ClassificationCUI && len(c.CUI) > 0 {
		var cuiParts []string
		for _, cui := range c.CUI {
			if cui != CUIBasic {
				cuiParts = append(cuiParts, cui.String())
			}
		}
		if len(cuiParts) > 0 {
			bannerText = bannerText + "//" + strings.Join(cuiParts, "//")
		}
	}

	// Add caveats if present
	if len(c.Caveats) > 0 {
		bannerText = bannerText + "//" + strings.Join(c.Caveats, "//")
	}

	return style.Width(width).Align(lipgloss.Center).Render(bannerText)
}

// RenderPortionMarking renders short form for inline markings: (U), (C), (S), (TS)
func RenderPortionMarking(c Classification) string {
	var mark string
	switch c.Level {
	case ClassificationUnclassified:
		mark = "U"
	case ClassificationCUI:
		mark = "CUI"
	case ClassificationConfidential:
		mark = "C"
	case ClassificationSecret:
		mark = "S"
	case ClassificationTopSecret:
		mark = "TS"
	default:
		mark = "U"
	}

	// Add NOFORN if present in caveats or CUI designations
	hasNOFORN := false
	for _, caveat := range c.Caveats {
		if strings.ToUpper(caveat) == "NOFORN" {
			hasNOFORN = true
			break
		}
	}
	if c.Level == ClassificationCUI {
		for _, cui := range c.CUI {
			if cui == CUINOFORN {
				hasNOFORN = true
				break
			}
		}
	}

	if hasNOFORN {
		mark = mark + "//NF"
	}

	return "(" + mark + ")"
}

// ValidateClassification validates that a classification combination is valid
func ValidateClassification(c Classification) error {
	// CUI designations are only valid with CUI level
	if c.Level != ClassificationCUI && len(c.CUI) > 0 {
		return errors.New("CUI designations can only be applied to CUI classification level")
	}

	// Check for conflicting CUI designations
	if c.Level == ClassificationCUI {
		hasBasic := false
		hasOther := false
		for _, cui := range c.CUI {
			if cui == CUIBasic {
				hasBasic = true
			} else {
				hasOther = true
			}
		}
		if hasBasic && hasOther {
			return errors.New("CUI BASIC cannot be combined with other CUI designations")
		}
	}

	// Validate caveats are appropriate for classification level
	for _, caveat := range c.Caveats {
		upperCaveat := strings.ToUpper(caveat)
		// NOFORN, ORCON, etc. are only valid for classified information
		if (upperCaveat == "NOFORN" || upperCaveat == "ORCON" || upperCaveat == "IMCON") &&
			c.Level == ClassificationUnclassified {
			return fmt.Errorf("caveat %s is not valid for UNCLASSIFIED information", caveat)
		}
	}

	return nil
}

// ParseClassification parses a marking string like "SECRET//NOFORN" into a Classification
func ParseClassification(s string) (Classification, error) {
	if s == "" {
		return DefaultClassification(), nil
	}

	parts := strings.Split(strings.ToUpper(s), "//")
	if len(parts) == 0 {
		return DefaultClassification(), nil
	}

	c := Classification{}

	// Parse the classification level
	switch strings.TrimSpace(parts[0]) {
	case "UNCLASSIFIED", "U":
		c.Level = ClassificationUnclassified
	case "CUI", "CONTROLLED UNCLASSIFIED INFORMATION":
		c.Level = ClassificationCUI
	case "CONFIDENTIAL", "C":
		c.Level = ClassificationConfidential
	case "SECRET", "S":
		c.Level = ClassificationSecret
	case "TOP SECRET", "TS", "TOPSECRET":
		c.Level = ClassificationTopSecret
	default:
		return Classification{}, fmt.Errorf("unknown classification level: %s", parts[0])
	}

	// Parse additional markings (caveats and CUI designations)
	for i := 1; i < len(parts); i++ {
		part := strings.TrimSpace(parts[i])
		switch part {
		case "NOFORN", "NF":
			if c.Level == ClassificationCUI {
				c.CUI = append(c.CUI, CUINOFORN)
			} else {
				c.Caveats = append(c.Caveats, "NOFORN")
			}
		case "IMCON":
			if c.Level == ClassificationCUI {
				c.CUI = append(c.CUI, CUIIMCON)
			} else {
				c.Caveats = append(c.Caveats, "IMCON")
			}
		case "PROPIN":
			if c.Level == ClassificationCUI {
				c.CUI = append(c.CUI, CUIPropin)
			} else {
				c.Caveats = append(c.Caveats, "PROPIN")
			}
		case "ORCON":
			if c.Level == ClassificationCUI {
				c.CUI = append(c.CUI, CUIORCON)
			} else {
				c.Caveats = append(c.Caveats, "ORCON")
			}
		case "BASIC":
			if c.Level == ClassificationCUI {
				c.CUI = append(c.CUI, CUIBasic)
			}
		case "REL TO", "RELTO":
			c.Caveats = append(c.Caveats, "REL TO")
		case "FOUO":
			c.Caveats = append(c.Caveats, "FOUO")
		default:
			// Treat unknown parts as caveats
			if part != "" {
				c.Caveats = append(c.Caveats, part)
			}
		}
	}

	// Validate the parsed classification
	if err := ValidateClassification(c); err != nil {
		return Classification{}, err
	}

	return c, nil
}

// WrapWithClassification wraps content with top and bottom classification banners
func WrapWithClassification(content string, c Classification, width int) string {
	topBanner := RenderTopBanner(c, width)
	bottomBanner := RenderBottomBanner(c, width)

	return topBanner + "\n" + content + "\n" + bottomBanner
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// CUIDesignationBlock represents the CUI Designation Indicator block
type CUIDesignationBlock struct {
	ControlledBy string // Organization controlling the CUI
	Category     string // CUI category (e.g., "CTI" for Controlled Technical Information)
	Distribution string // Distribution/dissemination control marking
}

// DefaultCUIDesignationBlock returns a default CUI designation block
func DefaultCUIDesignationBlock() CUIDesignationBlock {
	return CUIDesignationBlock{
		ControlledBy: "Department of Defense",
		Category:     "CTI",    // Controlled Technical Information
		Distribution: "FEDCON", // Federal Contractors
	}
}

// RenderCUIDesignationBlock renders the CUI designation indicator block
func RenderCUIDesignationBlock(block CUIDesignationBlock, width int) string {
	style := GetSecurityBannerStyle(ClassificationCUI)

	// Build the designation block
	lines := []string{
		"CUI DESIGNATION INDICATOR",
		strings.Repeat("-", 40),
		fmt.Sprintf("Controlled by: %s", block.ControlledBy),
		fmt.Sprintf("CUI Category:  %s", block.Category),
		fmt.Sprintf("Distribution:  %s", block.Distribution),
	}

	var rendered []string
	for _, line := range lines {
		rendered = append(rendered, style.Width(width).Align(lipgloss.Center).Render(line))
	}

	return strings.Join(rendered, "\n")
}

// GetAPIHeaders returns classification headers for API responses
func GetAPIHeaders(c Classification) map[string]string {
	headers := make(map[string]string)

	headers["X-Classification-Level"] = c.Level.String()

	if c.Level == ClassificationCUI && len(c.CUI) > 0 {
		var cuiDesignations []string
		for _, cui := range c.CUI {
			cuiDesignations = append(cuiDesignations, cui.String())
		}
		headers["X-CUI-Designations"] = strings.Join(cuiDesignations, ",")
	}

	if len(c.Caveats) > 0 {
		headers["X-Classification-Caveats"] = strings.Join(c.Caveats, ",")
	}

	return headers
}

// InlineMarker returns a simple inline classification marker
func InlineMarker(c Classification) string {
	style := GetSecurityBannerStyle(c.Level)
	return style.Render("[" + c.Level.String() + "]")
}

// ClassificationFromEnv creates a Classification from environment configuration
func ClassificationFromEnv(levelStr string) Classification {
	c, err := ParseClassification(levelStr)
	if err != nil {
		return DefaultClassification()
	}
	return c
}

// CompareClassification compares two classification levels
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func CompareClassification(a, b ClassificationLevel) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// HighestClassification returns the higher of two classifications
func HighestClassification(a, b Classification) Classification {
	if a.Level >= b.Level {
		return a
	}
	return b
}

// MustParseClassification parses a classification string and panics on error.
// This is useful for compile-time known strings where failure indicates a programming error.
//
// PANICS: If the classification string is invalid, this function will panic.
// For runtime user input, use ParseClassification instead which returns an error.
//
// Example:
//
//	var DefaultClass = MustParseClassification("SECRET//NOFORN") // OK - compile-time known
//	userClass := MustParseClassification(userInput) // BAD - use ParseClassification instead
func MustParseClassification(s string) Classification {
	c, err := ParseClassification(s)
	if err != nil {
		panic(fmt.Sprintf("invalid classification string: %s", s))
	}
	return c
}

// =============================================================================
// CLASSIFICATION HEADER VALIDATION (AC-4 Information Flow Enforcement)
// =============================================================================

// ClassificationHeaderKey is the HTTP header key for classification level.
const ClassificationHeaderKey = "X-Classification"

// ValidClassificationValues defines the valid values for the X-Classification header.
var ValidClassificationValues = map[string]ClassificationLevel{
	"UNCLASSIFIED": ClassificationUnclassified,
	"CUI":          ClassificationCUI,
	"SECRET":       ClassificationSecret,
	"TOP_SECRET":   ClassificationTopSecret,
}

// ErrInvalidClassificationHeader is returned when the X-Classification header has an invalid value.
var ErrInvalidClassificationHeader = errors.New("invalid X-Classification header value")

// ErrAC4Violation is returned when information flow would violate AC-4 controls.
var ErrAC4Violation = errors.New("AC-4 violation: classified information cannot be routed to cloud tier")

// ValidateClassificationHeaders validates classification headers from an HTTP request.
// Accepts the X-Classification header with valid values: UNCLASSIFIED, CUI, SECRET, TOP_SECRET.
// Returns ClassificationUnclassified if the header is not provided (default behavior).
// Returns an error if the header value is invalid.
//
// This implements NIST 800-53 AC-4: Information Flow Enforcement.
func ValidateClassificationHeaders(headers map[string]string) (ClassificationLevel, error) {
	// Get the X-Classification header value
	classValue, exists := headers[ClassificationHeaderKey]
	if !exists {
		// Also check lowercase variant for case-insensitive lookup
		classValue, exists = headers["x-classification"]
	}

	// Default to Unclassified if header not provided
	if !exists || classValue == "" {
		return ClassificationUnclassified, nil
	}

	// Normalize to uppercase for comparison
	normalizedValue := strings.ToUpper(strings.TrimSpace(classValue))

	// Look up the classification level
	level, valid := ValidClassificationValues[normalizedValue]
	if !valid {
		return ClassificationUnclassified, fmt.Errorf("%w: %s (valid values: UNCLASSIFIED, CUI, SECRET, TOP_SECRET)", ErrInvalidClassificationHeader, classValue)
	}

	return level, nil
}

// =============================================================================
// AC-4 ROUTING ENFORCEMENT
// =============================================================================

// Tier represents a model routing tier for AC-4 enforcement.
// This is a local type to avoid circular imports with the router package.
type Tier int

const (
	// TierCache represents cached responses (local, secure).
	TierCache Tier = iota
	// TierLocal represents local Ollama model inference (local, secure).
	TierLocal
	// TierAuto represents OpenRouter auto-routing (cloud, external).
	TierAuto
	// TierCloud represents cloud model routing (external network).
	TierCloud
	// TierHaiku represents Claude 3 Haiku (cloud, external).
	TierHaiku
	// TierSonnet represents Claude 3 Sonnet (cloud, external).
	TierSonnet
	// TierOpus represents Claude 3 Opus (cloud, external).
	TierOpus
	// TierGpt4o represents OpenAI GPT-4o (cloud, external).
	TierGpt4o
)

// String returns the string representation of a Tier.
func (t Tier) String() string {
	switch t {
	case TierCache:
		return "Cache"
	case TierLocal:
		return "Local"
	case TierAuto:
		return "Auto"
	case TierCloud:
		return "Cloud"
	case TierHaiku:
		return "Haiku"
	case TierSonnet:
		return "Sonnet"
	case TierOpus:
		return "Opus"
	case TierGpt4o:
		return "GPT-4o"
	default:
		return fmt.Sprintf("Tier(%d)", t)
	}
}

// IsCloudTier returns true if the tier routes to external cloud services.
// Cloud tiers are prohibited for CUI and higher classifications per AC-4.
func (t Tier) IsCloudTier() bool {
	return t >= TierAuto
}

// EnforceClassificationForRouting enforces AC-4 information flow controls.
// CUI and higher classifications MUST NOT be routed to cloud tiers.
// Returns an error if the routing would violate AC-4 controls.
//
// Per NIST 800-53 AC-4 (Information Flow Enforcement):
// - CUI (Controlled Unclassified Information) and higher MUST remain on local/on-premise systems
// - Cloud routing is only permitted for UNCLASSIFIED information
// - All AC-4 violations are logged for audit compliance
//
// This function logs AC-4 violations to the audit system with:
// - Classification level
// - Requested tier
// - Timestamp
// - Violation details
func EnforceClassificationForRouting(classification ClassificationLevel, tier Tier) error {
	// UNCLASSIFIED information can be routed anywhere
	if classification == ClassificationUnclassified {
		return nil
	}

	// Check if the tier is a cloud tier
	if tier.IsCloudTier() {
		// AC-4 VIOLATION: CUI or higher attempting cloud routing
		logAC4Violation(classification, tier)
		return fmt.Errorf("%w: %s data cannot be routed to %s tier",
			ErrAC4Violation, classification.String(), tier.String())
	}

	// Local tiers (Cache, Local) are permitted for all classification levels
	return nil
}

// logAC4Violation logs an AC-4 information flow violation to the audit system.
// Per AU-2 and AU-3, security-relevant events must be logged with sufficient detail.
func logAC4Violation(classification ClassificationLevel, tier Tier) {
	logger := GlobalAuditLogger()
	if logger == nil || !logger.IsEnabled() {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "AC4_VIOLATION",
		SessionID: "SYSTEM",
		Success:   false,
		Error:     fmt.Sprintf("Information flow violation: %s data blocked from %s tier", classification.String(), tier.String()),
		Metadata: map[string]string{
			"classification": classification.String(),
			"requested_tier": tier.String(),
			"control":        "AC-4",
			"violation_type": "information_flow",
			"enforcement":    "BLOCKED",
		},
	}

	// ERROR HANDLING: Errors must not be silently ignored
	if err := logger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log AC-4 violation event: %v\n", err)
	}
}

// =============================================================================
// HELPER FUNCTIONS FOR CLASSIFICATION VALIDATION
// =============================================================================

// IsClassificationAllowedForTier checks if a classification level is allowed for a given tier.
// This is a convenience function that wraps EnforceClassificationForRouting.
// Returns true if allowed, false if the combination would violate AC-4.
func IsClassificationAllowedForTier(classification ClassificationLevel, tier Tier) bool {
	return EnforceClassificationForRouting(classification, tier) == nil
}

// GetMaxAllowedTierForClassification returns the maximum tier allowed for a classification level.
// For UNCLASSIFIED: any tier is allowed (returns TierGpt4o as max)
// For CUI and higher: only local tiers are allowed (returns TierLocal as max)
func GetMaxAllowedTierForClassification(classification ClassificationLevel) Tier {
	if classification == ClassificationUnclassified {
		return TierGpt4o // All tiers allowed
	}
	return TierLocal // Only local tiers allowed for CUI+
}

// ValidateAndEnforceClassification is a combined validation and enforcement function.
// It validates the classification headers and enforces AC-4 routing controls in one call.
// Returns the validated classification level and any error (validation or enforcement).
func ValidateAndEnforceClassification(headers map[string]string, tier Tier) (ClassificationLevel, error) {
	// First, validate the classification headers
	classification, err := ValidateClassificationHeaders(headers)
	if err != nil {
		return ClassificationUnclassified, err
	}

	// Then, enforce AC-4 routing controls
	if err := EnforceClassificationForRouting(classification, tier); err != nil {
		return classification, err
	}

	return classification, nil
}

// =============================================================================
// CLASSIFICATION ENFORCER - NIST 800-53 AC-4 ENFORCEMENT
// =============================================================================
// Implements information flow control per NIST 800-53 AC-4.
// CUI and higher classifications MUST NEVER be sent to cloud services.
// This is a HARD security boundary - no exceptions.

// RoutingTier represents a routing tier for enforcement purposes.
// This is a simplified tier type for the enforcer to use.
type RoutingTier int

const (
	// RoutingTierLocal represents local Ollama model inference (safe for all classifications)
	RoutingTierLocal RoutingTier = iota
	// RoutingTierCloud represents cloud services (BLOCKED for CUI and above)
	RoutingTierCloud
)

// String returns the human-readable name of the routing tier.
func (t RoutingTier) String() string {
	switch t {
	case RoutingTierLocal:
		return "Local"
	case RoutingTierCloud:
		return "Cloud"
	default:
		return fmt.Sprintf("RoutingTier(%d)", t)
	}
}

// ErrClassificationBlocksCloud is returned when classification prevents cloud routing.
var ErrClassificationBlocksCloud = errors.New("AC-4: classification level blocks cloud routing")

// ClassificationEnforcer enforces classification-based routing restrictions.
// Implements NIST 800-53 AC-4 (Information Flow Enforcement).
//
// SECURITY CRITICAL: This enforcer ensures that CUI (Controlled Unclassified
// Information) and higher classification levels NEVER leave the local system.
// Cloud routing is ONLY permitted for UNCLASSIFIED data.
type ClassificationEnforcer struct {
	// SECURITY: Mutex protects concurrent access to classification state
	mu sync.RWMutex
	// auditLogger is used for AC-4 audit logging of blocked requests
	auditLogger *AuditLogger
	// sessionID is used for audit logging correlation
	sessionID string
	// enabled controls whether enforcement is active (default: true)
	enabled bool
}

// NewClassificationEnforcer creates a new enforcer with the given audit logger.
func NewClassificationEnforcer(auditLogger *AuditLogger, sessionID string) *ClassificationEnforcer {
	return &ClassificationEnforcer{
		auditLogger: auditLogger,
		sessionID:   sessionID,
		enabled:     true, // Always enabled by default for security
	}
}

// NewClassificationEnforcerGlobal creates an enforcer using the global audit logger.
func NewClassificationEnforcerGlobal(sessionID string) *ClassificationEnforcer {
	return &ClassificationEnforcer{
		auditLogger: GlobalAuditLogger(),
		sessionID:   sessionID,
		enabled:     true,
	}
}

// SetEnabled enables or disables enforcement.
// WARNING: Disabling enforcement violates NIST 800-53 AC-4 and should
// ONLY be done in testing scenarios with explicit acknowledgment.
func (e *ClassificationEnforcer) SetEnabled(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = enabled
}

// IsEnabled returns whether enforcement is active.
func (e *ClassificationEnforcer) IsEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enabled
}

// SetSessionID updates the session ID for audit logging.
func (e *ClassificationEnforcer) SetSessionID(sessionID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessionID = sessionID
}

// CanRouteToCloud determines if data at the given classification level
// can be sent to cloud services.
//
// NIST 800-53 AC-4 Requirement:
//   - UNCLASSIFIED: Allowed to cloud
//   - CUI (Controlled Unclassified Information): BLOCKED - local only
//   - CONFIDENTIAL: BLOCKED - local only
//   - SECRET: BLOCKED - local only
//   - TOP SECRET: BLOCKED - local only
//
// Returns true ONLY for UNCLASSIFIED data.
func (e *ClassificationEnforcer) CanRouteToCloud(classification ClassificationLevel) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.canRouteToCloudLocked(classification)
}

// canRouteToCloudLocked is the internal implementation that requires the lock to be held.
func (e *ClassificationEnforcer) canRouteToCloudLocked(classification ClassificationLevel) bool {
	// If enforcement is disabled (testing only), allow all
	if !e.enabled {
		return true
	}

	// CRITICAL: Only UNCLASSIFIED data can go to cloud
	// CUI and all higher classifications MUST stay local
	return classification == ClassificationUnclassified
}

// EnforceRouting enforces classification-based routing restrictions.
// If the requested tier is Cloud but classification doesn't allow it,
// this method will:
//  1. Log the block to the audit log (AC-4 compliance)
//  2. Return RoutingTierLocal (forced downgrade) with an error
//
// Parameters:
//   - classification: The classification level of the data
//   - requestedTier: The tier that was requested by the router
//
// Returns:
//   - The allowed tier (may be downgraded from Cloud to Local)
//   - An error if the tier was blocked (nil if allowed)
//
// Usage:
//
//	tier, err := enforcer.EnforceRouting(ClassificationCUI, RoutingTierCloud)
//	if err != nil {
//	    // Log warning but continue with local tier
//	    log.Printf("AC-4 block: %v", err)
//	}
//	// tier is guaranteed to be safe for the classification
func (e *ClassificationEnforcer) EnforceRouting(classification ClassificationLevel, requestedTier RoutingTier) (RoutingTier, error) {
	e.mu.RLock()
	enabled := e.enabled
	canRoute := e.canRouteToCloudLocked(classification)
	sessionID := e.sessionID
	auditLogger := e.auditLogger
	e.mu.RUnlock()

	// If enforcement is disabled or local was requested, pass through
	if !enabled || requestedTier == RoutingTierLocal {
		return requestedTier, nil
	}

	// Check if cloud is allowed for this classification
	if canRoute {
		return requestedTier, nil
	}

	// BLOCKED: Cloud requested but classification doesn't allow it
	// This is an AC-4 information flow control enforcement

	// Log the block to audit log (outside lock to avoid deadlock with audit logger)
	e.logAC4BlockWithParams(classification, requestedTier, sessionID, auditLogger)

	// Return forced local tier with error for caller awareness
	return RoutingTierLocal, fmt.Errorf("%w: %s data cannot be sent to %s tier",
		ErrClassificationBlocksCloud,
		classification.String(),
		requestedTier.String(),
	)
}

// logAC4Block logs a classification-based routing block to the audit log.
// This is REQUIRED for NIST 800-53 AC-4 compliance.
func (e *ClassificationEnforcer) logAC4Block(classification ClassificationLevel, requestedTier RoutingTier) {
	e.mu.RLock()
	sessionID := e.sessionID
	auditLogger := e.auditLogger
	e.mu.RUnlock()

	e.logAC4BlockWithParams(classification, requestedTier, sessionID, auditLogger)
}

// logAC4BlockWithParams logs a classification-based routing block with explicit parameters.
// This avoids holding the lock during the potentially slow audit logging operation.
func (e *ClassificationEnforcer) logAC4BlockWithParams(classification ClassificationLevel, requestedTier RoutingTier, sessionID string, auditLogger *AuditLogger) {
	if auditLogger == nil {
		return
	}

	event := AuditEvent{
		Timestamp: time.Now(),
		EventType: "AC4_BLOCK",
		SessionID: sessionID,
		Success:   true, // The block itself is a successful enforcement
		Metadata: map[string]string{
			"classification": classification.String(),
			"requested_tier": requestedTier.String(),
			"enforced_tier":  RoutingTierLocal.String(),
			"control":        "NIST 800-53 AC-4",
			"action":         "BLOCK_CLOUD_ROUTING",
			"reason":         fmt.Sprintf("%s classification blocks cloud routing", classification.String()),
		},
	}

	// ERROR HANDLING: Errors must not be silently ignored
	if err := auditLogger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log AC-4 block event: %v\n", err)
	}
}

// RequiresLocalOnly returns true if the classification requires local-only processing.
// This is a convenience method that returns the inverse of CanRouteToCloud.
func (e *ClassificationEnforcer) RequiresLocalOnly(classification ClassificationLevel) bool {
	return !e.CanRouteToCloud(classification)
}

// ValidateRoutingDecision validates that a routing decision is compatible
// with the given classification level. Returns an error if the routing
// decision would violate AC-4 requirements.
//
// This method does NOT modify the routing decision - use EnforceRouting
// to get a corrected tier.
func (e *ClassificationEnforcer) ValidateRoutingDecision(classification ClassificationLevel, tier RoutingTier) error {
	e.mu.RLock()
	enabled := e.enabled
	canRoute := e.canRouteToCloudLocked(classification)
	e.mu.RUnlock()

	if !enabled {
		return nil
	}

	if tier == RoutingTierCloud && !canRoute {
		return fmt.Errorf("%w: %s data cannot be sent to %s tier",
			ErrClassificationBlocksCloud,
			classification.String(),
			tier.String(),
		)
	}

	return nil
}

// GetClassificationRestrictions returns a human-readable description of
// routing restrictions for the given classification level.
func (e *ClassificationEnforcer) GetClassificationRestrictions(classification ClassificationLevel) string {
	switch classification {
	case ClassificationUnclassified:
		return "UNCLASSIFIED: No routing restrictions - cloud allowed"
	case ClassificationCUI:
		return "CUI: Cloud routing BLOCKED - local processing only (AC-4)"
	case ClassificationConfidential:
		return "CONFIDENTIAL: Cloud routing BLOCKED - local processing only (AC-4)"
	case ClassificationSecret:
		return "SECRET: Cloud routing BLOCKED - local processing only (AC-4)"
	case ClassificationTopSecret:
		return "TOP SECRET: Cloud routing BLOCKED - local processing only (AC-4)"
	default:
		return "UNKNOWN: Cloud routing BLOCKED by default - local processing only (AC-4)"
	}
}
