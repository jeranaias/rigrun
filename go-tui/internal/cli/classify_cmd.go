// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// classify_cmd.go - Classification command implementation for rigrun IL5 compliance.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements runtime classification level management per:
// - DoDI 5200.48 (Classification marking requirements)
// - 32 CFR Part 2002 (CUI marking standards)
// - AC-3 (Access enforcement based on classification)
// - SC-16 (Transmission of security attributes)
//
// Command: classify [subcommand]
// Short:   Classification level management (IL5)
// Aliases: class
//
// Subcommands:
//   show (default)      Show current classification level
//   set <level>         Set classification level
//
// Classification Levels:
//   UNCLASSIFIED        Public data
//   CUI                 Controlled Unclassified Information
//   CONFIDENTIAL        Confidential (requires clearance)
//   SECRET              Secret (requires clearance)
//   TOP_SECRET          Top Secret (requires clearance)
//
// Examples:
//   rigrun classify                          Show current level
//   rigrun classify show                     Show classification status
//   rigrun classify show --json              Status in JSON format
//   rigrun classify set UNCLASSIFIED         Set to unclassified
//   rigrun classify set CUI                  Set to CUI
//   rigrun classify set SECRET               Set to Secret
//   rigrun classify set SECRET --caveat NOFORN
//                                            Set SECRET//NOFORN
//   rigrun classify set TOP_SECRET --caveat SI --caveat TK
//                                            Set TS//SI/TK
//
// Caveats:
//   NOFORN              Not releasable to foreign nationals
//   ORCON               Originator controlled
//   SI                  Special Intelligence
//   TK                  Talent Keyhole
//   SAR                 Special Access Required
//
// Flags:
//   --caveat CAVEAT     Add caveat to classification (repeatable)
//   --json              Output in JSON format
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/util"
)

// Ensure config package is imported for side effects
var _ = config.Default

// =============================================================================
// CLASSIFY STYLES
// =============================================================================

var (
	// Title style for classify header
	classifyTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	// Section style
	classifySectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	// Label style for field names
	classifyLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Light gray
				Width(18)

	// Value style
	classifyValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")) // Green

	// Warning style
	classifyWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")). // Yellow
				Bold(true)

	// Error style
	classifyErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")). // Red
				Bold(true)

	// Success style
	classifySuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")). // Green
				Bold(true)

	// Separator style
	classifySeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// =============================================================================
// JSON OUTPUT TYPES
// =============================================================================

// ClassificationInfo represents classification information for JSON output.
type ClassificationInfo struct {
	Level       string   `json:"level"`
	Caveats     []string `json:"caveats,omitempty"`
	Banner      string   `json:"banner"`
	Color       string   `json:"color"`
	IsClassified bool    `json:"is_classified"`
	Timestamp   string   `json:"timestamp"`
}

// ValidationResult represents text validation results for JSON output.
type ValidationResult struct {
	Text          string         `json:"text"`
	HasMarkers    bool           `json:"has_markers"`
	MarkersFound  []MarkerMatch  `json:"markers_found,omitempty"`
	HighestLevel  string         `json:"highest_level,omitempty"`
	Timestamp     string         `json:"timestamp"`
}

// MarkerMatch represents a found classification marker.
type MarkerMatch struct {
	Marker   string `json:"marker"`
	Level    string `json:"level"`
	Position int    `json:"position"`
}

// LevelsInfo represents available levels for JSON output.
type LevelsInfo struct {
	Levels    []LevelDetail `json:"levels"`
	Current   string        `json:"current"`
	Timestamp string        `json:"timestamp"`
}

// LevelDetail provides details about a classification level.
type LevelDetail struct {
	Name        string `json:"name"`
	ShortForm   string `json:"short_form"`
	Color       string `json:"color"`
	ColorCode   string `json:"color_code"`
	Description string `json:"description"`
}

// =============================================================================
// HANDLE CLASSIFY
// =============================================================================

// HandleClassify handles the "classify" command with all subcommands.
func HandleClassify(args Args) error {
	switch args.Subcommand {
	case "", "show":
		return handleClassifyShow(args)
	case "set":
		return handleClassifySet(args)
	case "banner":
		return handleClassifyBanner(args)
	case "validate":
		return handleClassifyValidate(args)
	case "levels":
		return handleClassifyLevels(args)
	default:
		return fmt.Errorf("unknown classify subcommand: %s\n\nValid subcommands:\n"+
			"  show      Show current classification level\n"+
			"  set       Set classification level\n"+
			"  banner    Display full classification banner\n"+
			"  validate  Check text for classification markers\n"+
			"  levels    List available classification levels", args.Subcommand)
	}
}

// =============================================================================
// SHOW SUBCOMMAND
// =============================================================================

// handleClassifyShow displays the current classification level and banner.
func handleClassifyShow(args Args) error {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = DefaultConfig()
	}

	// Parse current classification from config
	classification, parseErr := security.ParseClassification(cfg.Security.Classification)
	if parseErr != nil {
		classification = security.DefaultClassification()
	}

	// JSON output
	if args.JSON {
		info := ClassificationInfo{
			Level:        classification.Level.String(),
			Caveats:      classification.Caveats,
			Banner:       classification.String(),
			Color:        getColorName(classification.Level),
			IsClassified: classification.IsClassified(),
			Timestamp:    time.Now().UTC().Format(time.RFC3339),
		}
		return classifyOutputJSON(info)
	}

	// Human-readable output
	separator := strings.Repeat("=", 50)
	fmt.Println()
	fmt.Println(classifyTitleStyle.Render("Classification Status"))
	fmt.Println(classifySeparatorStyle.Render(separator))
	fmt.Println()

	// Current level section
	fmt.Println(classifySectionStyle.Render("Current Classification"))

	// Level with colored indicator
	levelStyle := security.GetSecurityBannerStyle(classification.Level)
	fmt.Printf("  %s%s\n",
		classifyLabelStyle.Render("Level:"),
		levelStyle.Render(" "+classification.Level.String()+" "))

	// Caveats
	if len(classification.Caveats) > 0 {
		fmt.Printf("  %s%s\n",
			classifyLabelStyle.Render("Caveats:"),
			classifyValueStyle.Render(strings.Join(classification.Caveats, ", ")))
	} else {
		fmt.Printf("  %s%s\n",
			classifyLabelStyle.Render("Caveats:"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("None"))
	}

	// Full banner string
	fmt.Printf("  %s%s\n",
		classifyLabelStyle.Render("Banner:"),
		classifyValueStyle.Render(classification.String()))

	// Classification status
	fmt.Printf("  %s%s\n",
		classifyLabelStyle.Render("Is Classified:"),
		formatBool(classification.IsClassified()))

	fmt.Println()

	// Configuration section
	fmt.Println(classifySectionStyle.Render("Configuration"))
	fmt.Printf("  %s%s\n",
		classifyLabelStyle.Render("Banner Enabled:"),
		formatBool(cfg.Security.BannerEnabled))
	fmt.Printf("  %s%s\n",
		classifyLabelStyle.Render("Audit Enabled:"),
		formatBool(cfg.Security.AuditEnabled))

	fmt.Println()

	// IL5 compliance reminder
	if classification.IsClassified() {
		fmt.Println(classifyWarningStyle.Render("WARNING: Classified information handling in effect"))
		fmt.Println("  Per AC-3: Access control enforcement is active")
		fmt.Println("  Per SC-16: Security attributes transmitted with data")
	}

	fmt.Println()
	return nil
}

// =============================================================================
// SET SUBCOMMAND
// =============================================================================

// handleClassifySet sets the classification level.
func handleClassifySet(args Args) error {
	if args.Query == "" {
		return fmt.Errorf("no classification level provided\n\nUsage: rigrun classify set <LEVEL> [--caveat <CAVEAT>]\n\n"+
			"Available levels:\n"+
			"  UNCLASSIFIED\n"+
			"  CUI           (Controlled Unclassified Information)\n"+
			"  CONFIDENTIAL\n"+
			"  SECRET\n"+
			"  TOP_SECRET\n\n"+
			"Available caveats:\n"+
			"  NOFORN        Not Releasable to Foreign Nationals\n"+
			"  ORCON         Originator Controlled\n"+
			"  IMCON         Imagery Intelligence Control\n"+
			"  PROPIN        Proprietary Information\n"+
			"  REL TO        Releasable To\n"+
			"  FOUO          For Official Use Only")
	}

	// Normalize level input
	levelStr := strings.ToUpper(strings.TrimSpace(args.Query))
	levelStr = strings.ReplaceAll(levelStr, " ", "_")

	// Build classification string
	classificationStr := levelStr
	if args.Caveat != "" {
		caveat := strings.ToUpper(strings.TrimSpace(args.Caveat))
		classificationStr = levelStr + "//" + caveat
	}

	// Parse and validate the classification
	classification, err := security.ParseClassification(classificationStr)
	if err != nil {
		return fmt.Errorf("invalid classification: %w\n\n"+
			"Valid levels: UNCLASSIFIED, CUI, CONFIDENTIAL, SECRET, TOP_SECRET\n"+
			"Valid caveats: NOFORN, ORCON, IMCON, PROPIN, REL TO, FOUO", err)
	}

	// Validate the classification combination
	if err := security.ValidateClassification(classification); err != nil {
		return fmt.Errorf("invalid classification combination: %w", err)
	}

	// Load config and update
	cfg, loadErr := LoadConfig()
	if loadErr != nil {
		cfg = DefaultConfig()
	}

	// Store the full classification string
	cfg.Security.Classification = classification.String()

	// If setting to a classified level, enable banner by default
	if classification.IsClassified() {
		cfg.Security.BannerEnabled = true
	}

	// Save config
	if err := SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Log the classification change for audit (SC-16 compliance)
	// ERROR HANDLING: Errors must not be silently ignored
	auditLogger := security.GlobalAuditLogger()
	if auditLogger != nil && auditLogger.IsEnabled() {
		metadata := map[string]string{
			"new_level":  classification.Level.String(),
			"old_level":  cfg.Security.Classification,
			"caveats":    strings.Join(classification.Caveats, ","),
		}
		if err := auditLogger.LogEvent("CLI", "CLASSIFICATION_CHANGE", metadata); err != nil {
			// Log to stderr when audit logging fails - per AU-5 requirements
			fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log classification change: %v\n", err)
		}
	}

	// JSON output
	if args.JSON {
		info := ClassificationInfo{
			Level:        classification.Level.String(),
			Caveats:      classification.Caveats,
			Banner:       classification.String(),
			Color:        getColorName(classification.Level),
			IsClassified: classification.IsClassified(),
			Timestamp:    time.Now().UTC().Format(time.RFC3339),
		}
		return classifyOutputJSON(info)
	}

	// Human-readable output
	levelStyle := security.GetSecurityBannerStyle(classification.Level)
	fmt.Printf("%s Classification set to: %s\n",
		classifySuccessStyle.Render("[OK]"),
		levelStyle.Render(" "+classification.String()+" "))

	if classification.IsClassified() {
		fmt.Println()
		fmt.Println(classifyWarningStyle.Render("NOTICE: Classified information handling is now in effect"))
		fmt.Println("  - Classification banner will be displayed in TUI")
		fmt.Println("  - Audit logging includes classification metadata")
		fmt.Println("  - Session data includes security attributes (SC-16)")
	}

	return nil
}

// =============================================================================
// BANNER SUBCOMMAND
// =============================================================================

// handleClassifyBanner displays the full classification banner.
func handleClassifyBanner(args Args) error {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = DefaultConfig()
	}

	// Parse current classification
	classification, parseErr := security.ParseClassification(cfg.Security.Classification)
	if parseErr != nil {
		classification = security.DefaultClassification()
	}

	// JSON output
	if args.JSON {
		info := ClassificationInfo{
			Level:        classification.Level.String(),
			Caveats:      classification.Caveats,
			Banner:       classification.String(),
			Color:        getColorName(classification.Level),
			IsClassified: classification.IsClassified(),
			Timestamp:    time.Now().UTC().Format(time.RFC3339),
		}
		return classifyOutputJSON(info)
	}

	// Get terminal width (default to 80)
	width := 80

	// Render top banner
	fmt.Println()
	fmt.Println(security.RenderTopBanner(classification, width))
	fmt.Println()

	// Banner information per DoDI 5200.48
	fmt.Println("Classification Banner Information")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("  Level:          %s\n", classification.Level.String())
	fmt.Printf("  Portion Mark:   %s\n", security.RenderPortionMarking(classification))
	fmt.Printf("  Full Banner:    %s\n", classification.String())
	if len(classification.Caveats) > 0 {
		fmt.Printf("  Caveats:        %s\n", strings.Join(classification.Caveats, ", "))
	}

	fmt.Println()

	// Render bottom banner
	fmt.Println(security.RenderBottomBanner(classification, width))
	fmt.Println()

	// Compliance information
	fmt.Println("Per DoDI 5200.48:")
	fmt.Println("  - Classification banners must appear at top and bottom of displays")
	fmt.Println("  - Portion markings must appear on all classified portions")
	fmt.Println("  - Banner colors follow DoD standard (Green/Purple/Blue/Red/Orange)")

	return nil
}

// =============================================================================
// VALIDATE SUBCOMMAND
// =============================================================================

// classificationMarkerPatterns defines regex patterns for classification markers.
var classificationMarkerPatterns = []struct {
	pattern *regexp.Regexp
	level   string
}{
	{regexp.MustCompile(`(?i)\bTOP\s*SECRET\b`), "TOP_SECRET"},
	{regexp.MustCompile(`(?i)\bTS\b`), "TOP_SECRET"},
	{regexp.MustCompile(`(?i)\bTS//[A-Z]+\b`), "TOP_SECRET"},
	{regexp.MustCompile(`(?i)\bSECRET\b`), "SECRET"},
	{regexp.MustCompile(`(?i)\bS//[A-Z]+\b`), "SECRET"},
	{regexp.MustCompile(`(?i)\bCONFIDENTIAL\b`), "CONFIDENTIAL"},
	{regexp.MustCompile(`(?i)\bC//[A-Z]+\b`), "CONFIDENTIAL"},
	{regexp.MustCompile(`(?i)\bCUI\b`), "CUI"},
	{regexp.MustCompile(`(?i)\bCONTROLLED\s+UNCLASSIFIED\b`), "CUI"},
	{regexp.MustCompile(`(?i)\bUNCLASSIFIED\b`), "UNCLASSIFIED"},
	{regexp.MustCompile(`(?i)\bU//FOUO\b`), "UNCLASSIFIED"},
	{regexp.MustCompile(`(?i)\bNOFORN\b`), "CAVEAT"},
	{regexp.MustCompile(`(?i)\bORCON\b`), "CAVEAT"},
	{regexp.MustCompile(`(?i)\bREL\s*TO\b`), "CAVEAT"},
	{regexp.MustCompile(`(?i)\bFOUO\b`), "CAVEAT"},
	{regexp.MustCompile(`(?i)\(TS\)`), "TOP_SECRET"},
	{regexp.MustCompile(`(?i)\(S\)`), "SECRET"},
	{regexp.MustCompile(`(?i)\(C\)`), "CONFIDENTIAL"},
	{regexp.MustCompile(`(?i)\(U\)`), "UNCLASSIFIED"},
	{regexp.MustCompile(`(?i)\(CUI\)`), "CUI"},
}

// handleClassifyValidate checks text for classification markers.
func handleClassifyValidate(args Args) error {
	text := args.Query
	if text == "" {
		return fmt.Errorf("no text provided to validate\n\nUsage: rigrun classify validate \"<text>\"\n\n"+
			"Example: rigrun classify validate \"This document is SECRET//NOFORN\"")
	}

	var markers []MarkerMatch
	highestLevel := ""
	levelPriority := map[string]int{
		"UNCLASSIFIED": 0,
		"CUI":          1,
		"CONFIDENTIAL": 2,
		"SECRET":       3,
		"TOP_SECRET":   4,
		"CAVEAT":       -1, // Caveats don't count for priority
	}

	// Find all classification markers
	for _, mp := range classificationMarkerPatterns {
		matches := mp.pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			marker := text[match[0]:match[1]]
			markers = append(markers, MarkerMatch{
				Marker:   marker,
				Level:    mp.level,
				Position: match[0],
			})

			// Track highest classification level found
			if mp.level != "CAVEAT" {
				if priority, ok := levelPriority[mp.level]; ok {
					if highestPriority, hok := levelPriority[highestLevel]; !hok || priority > highestPriority {
						highestLevel = mp.level
					}
				}
			}
		}
	}

	hasMarkers := len(markers) > 0

	// JSON output
	if args.JSON {
		result := ValidationResult{
			Text:         text,
			HasMarkers:   hasMarkers,
			MarkersFound: markers,
			HighestLevel: highestLevel,
			Timestamp:    time.Now().UTC().Format(time.RFC3339),
		}
		return classifyOutputJSON(result)
	}

	// Human-readable output
	fmt.Println()
	fmt.Println(classifyTitleStyle.Render("Classification Marker Validation"))
	fmt.Println(classifySeparatorStyle.Render(strings.Repeat("=", 50)))
	fmt.Println()

	// UNICODE: Rune-aware truncation preserves multi-byte characters
	displayText := util.TruncateRunes(text, 100)
	fmt.Printf("  %s%s\n",
		classifyLabelStyle.Render("Input:"),
		fmt.Sprintf("\"%s\"", displayText))
	fmt.Println()

	if hasMarkers {
		fmt.Printf("  %s%s\n",
			classifyLabelStyle.Render("Markers Found:"),
			classifyWarningStyle.Render("YES"))

		fmt.Println()
		fmt.Println(classifySectionStyle.Render("Detected Markers"))

		for _, m := range markers {
			levelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
			if m.Level == "TOP_SECRET" || m.Level == "SECRET" {
				levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
			} else if m.Level == "CONFIDENTIAL" {
				levelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("21"))
			}
			fmt.Printf("    - %s (Level: %s, Position: %d)\n",
				levelStyle.Render(m.Marker),
				m.Level,
				m.Position)
		}

		fmt.Println()
		if highestLevel != "" {
			fmt.Printf("  %s%s\n",
				classifyLabelStyle.Render("Highest Level:"),
				classifyErrorStyle.Render(highestLevel))
		}

		fmt.Println()
		fmt.Println(classifyWarningStyle.Render("WARNING: Text contains classification markers"))
		fmt.Println("  Ensure proper handling per DoDI 5200.48")
	} else {
		fmt.Printf("  %s%s\n",
			classifyLabelStyle.Render("Markers Found:"),
			classifySuccessStyle.Render("None detected"))
		fmt.Println()
		fmt.Println("  No classification markers found in the provided text.")
		fmt.Println("  Note: This does not guarantee the text is unclassified.")
	}

	fmt.Println()
	return nil
}

// =============================================================================
// LEVELS SUBCOMMAND
// =============================================================================

// handleClassifyLevels lists all available classification levels.
func handleClassifyLevels(args Args) error {
	cfg, err := LoadConfig()
	if err != nil {
		cfg = DefaultConfig()
	}

	levels := []LevelDetail{
		{
			Name:        "UNCLASSIFIED",
			ShortForm:   "(U)",
			Color:       "Green",
			ColorCode:   "#00FF00",
			Description: "Information that has been reviewed and determined not to require classification",
		},
		{
			Name:        "CUI",
			ShortForm:   "(CUI)",
			Color:       "Purple",
			ColorCode:   "#800080",
			Description: "Controlled Unclassified Information requiring safeguarding per 32 CFR Part 2002",
		},
		{
			Name:        "CONFIDENTIAL",
			ShortForm:   "(C)",
			Color:       "Blue",
			ColorCode:   "#0000FF",
			Description: "Information that could cause damage to national security if disclosed",
		},
		{
			Name:        "SECRET",
			ShortForm:   "(S)",
			Color:       "Red",
			ColorCode:   "#FF0000",
			Description: "Information that could cause serious damage to national security if disclosed",
		},
		{
			Name:        "TOP_SECRET",
			ShortForm:   "(TS)",
			Color:       "Orange",
			ColorCode:   "#FFA500",
			Description: "Information that could cause exceptionally grave damage to national security if disclosed",
		},
	}

	// JSON output
	if args.JSON {
		info := LevelsInfo{
			Levels:    levels,
			Current:   cfg.Security.Classification,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		return classifyOutputJSON(info)
	}

	// Human-readable output
	fmt.Println()
	fmt.Println(classifyTitleStyle.Render("Available Classification Levels"))
	fmt.Println(classifySeparatorStyle.Render(strings.Repeat("=", 70)))
	fmt.Println()

	fmt.Printf("  Current Level: %s\n\n", classifyValueStyle.Render(cfg.Security.Classification))

	for _, level := range levels {
		// Get appropriate style for this level
		levelColor := lipgloss.Color(level.ColorCode)
		levelStyle := lipgloss.NewStyle().
			Bold(true).
			Background(levelColor)

		// Use contrasting foreground
		if level.Name == "UNCLASSIFIED" || level.Name == "TOP_SECRET" {
			levelStyle = levelStyle.Foreground(lipgloss.Color("#000000"))
		} else {
			levelStyle = levelStyle.Foreground(lipgloss.Color("#FFFFFF"))
		}

		fmt.Printf("  %s %s\n",
			levelStyle.Render(" "+level.Name+" "),
			lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render(level.ShortForm))
		fmt.Printf("    Color: %s\n", level.Color)
		fmt.Printf("    %s\n\n", level.Description)
	}

	// Available caveats section
	fmt.Println(classifySectionStyle.Render("Available Caveats"))
	fmt.Println()

	caveats := []struct {
		name        string
		description string
	}{
		{"NOFORN", "Not Releasable to Foreign Nationals"},
		{"ORCON", "Originator Controlled - dissemination requires originator approval"},
		{"IMCON", "Imagery Intelligence Control"},
		{"PROPIN", "Proprietary Information Involved"},
		{"REL TO", "Releasable To (followed by country codes)"},
		{"FOUO", "For Official Use Only (legacy, use CUI instead)"},
	}

	for _, c := range caveats {
		fmt.Printf("  %-10s %s\n", c.name, c.description)
	}

	fmt.Println()
	fmt.Println("Per DoDI 5200.48:")
	fmt.Println("  - Classification banners use standard DoD colors")
	fmt.Println("  - Caveats restrict further dissemination of classified information")
	fmt.Println("  - CUI follows 32 CFR Part 2002 marking requirements")
	fmt.Println()

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// getColorName returns the human-readable color name for a classification level.
func getColorName(level security.ClassificationLevel) string {
	switch level {
	case security.ClassificationUnclassified:
		return "Green"
	case security.ClassificationCUI:
		return "Purple"
	case security.ClassificationConfidential:
		return "Blue"
	case security.ClassificationSecret:
		return "Red"
	case security.ClassificationTopSecret:
		return "Orange"
	default:
		return "Green"
	}
}

// formatBool returns a styled yes/no string for classification display.
// NOTE: This differs from view.go's formatBool which returns "enabled"/"disabled".
// This function is specifically for yes/no question answers in classification output.
// TODO: Rename to formatBoolYesNo() or consolidate into shared utility package.
func formatBool(b bool) string {
	if b {
		return classifyValueStyle.Render("Yes")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("No")
}

// classifyOutputJSON marshals data to JSON and prints it.
func classifyOutputJSON(data interface{}) error {
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(output))
	return nil
}
