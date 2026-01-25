// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// agreements_cmd.go - CLI commands for PS-6 and PL-4 management in rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements NIST 800-53 PS-6 (Access Agreements) and PL-4 (Rules of Behavior)
// controls for DoD IL5 compliance.
//
// Command: agreements [subcommand]
// Short:   Access agreements management (IL5 PS-6, PL-4)
// Aliases: agree
//
// Subcommands:
//   list (default)      List all agreements
//   show <id>           Show agreement text
//   sign <id>           Sign an agreement
//   status              Show user's agreement status
//   check               Verify all required agreements signed
//
// Rules of Behavior Subcommands:
//   rules show          Show rules of behavior
//   rules acknowledge   Acknowledge rules
//   rules status        Show acknowledgment status
//
// Examples:
//   rigrun agreements                      List agreements (default)
//   rigrun agreements list                 List all agreements
//   rigrun agreements show EULA-001        Show agreement text
//   rigrun agreements sign EULA-001        Sign agreement
//   rigrun agreements status               Show your status
//   rigrun agreements status --json        Status in JSON format
//   rigrun agreements check                Verify compliance
//   rigrun rules show                      Show rules of behavior
//   rigrun rules acknowledge               Acknowledge rules
//   rigrun rules status                    Check acknowledgment
//
// PS-6/PL-4 Requirements:
//   - Access agreements required before system use
//   - Rules of behavior acknowledgment required
//   - All signatures are logged and timestamped
//   - Periodic re-acknowledgment may be required
//
// Flags:
//   --json              Output in JSON format
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// AGREEMENT COMMAND STYLES
// =============================================================================

var (
	agreementTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	agreementSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	agreementLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Light gray
				Width(20)

	agreementValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")) // White

	agreementGreenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")) // Green

	agreementRedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red

	agreementYellowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")) // Yellow

	agreementDimStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("242")) // Dim

	agreementSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	agreementErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)

	agreementBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(1, 2)
)

// =============================================================================
// AGREEMENTS ARGUMENTS
// =============================================================================

// AgreementsArgs holds parsed agreements command arguments.
type AgreementsArgs struct {
	Subcommand  string
	AgreementID string
	UserID      string
	JSON        bool
}

// parseAgreementsCmdArgs parses agreements command specific arguments.
func parseAgreementsCmdArgs(args *Args, remaining []string) AgreementsArgs {
	agreementArgs := AgreementsArgs{
		JSON: args.JSON,
	}

	if len(remaining) > 0 {
		agreementArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	if len(remaining) > 0 {
		agreementArgs.AgreementID = remaining[0]
		remaining = remaining[1:]
	}

	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--user", "-u":
			if i+1 < len(remaining) {
				i++
				agreementArgs.UserID = remaining[i]
			}
		case "--json":
			agreementArgs.JSON = true
		default:
			if strings.HasPrefix(arg, "--user=") {
				agreementArgs.UserID = strings.TrimPrefix(arg, "--user=")
			}
		}
	}

	return agreementArgs
}

// =============================================================================
// HANDLE AGREEMENTS
// =============================================================================

// HandleAgreements handles the "agreements" command with various subcommands.
// Implements PS-6 management commands.
func HandleAgreements(args Args) error {
	agreementArgs := parseAgreementsCmdArgs(&args, args.Raw)

	switch agreementArgs.Subcommand {
	case "", "list":
		return handleAgreementsList(agreementArgs)
	case "show":
		return handleAgreementsShow(agreementArgs)
	case "sign":
		return handleAgreementsSign(agreementArgs)
	case "status":
		return handleAgreementsStatus(agreementArgs)
	case "check":
		return handleAgreementsCheck(agreementArgs)
	default:
		return fmt.Errorf("unknown agreements subcommand: %s\n\nUsage:\n"+
			"  rigrun agreements list          List all agreements\n"+
			"  rigrun agreements show [id]     Show agreement text\n"+
			"  rigrun agreements sign [id]     Sign agreement\n"+
			"  rigrun agreements status        Show user's agreement status\n"+
			"  rigrun agreements check         Verify all required agreements signed", agreementArgs.Subcommand)
	}
}

// =============================================================================
// AGREEMENTS LIST
// =============================================================================

// handleAgreementsList lists all available agreements.
func handleAgreementsList(args AgreementsArgs) error {
	manager := security.GlobalAgreementManager()
	agreements := manager.ListAgreements()

	if args.JSON {
		data, err := json.MarshalIndent(agreements, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(agreementTitleStyle.Render("Available Agreements (PS-6)"))
	fmt.Println(agreementDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	if len(agreements) == 0 {
		fmt.Println(agreementDimStyle.Render("  No agreements found."))
		fmt.Println()
		return nil
	}

	for _, agreement := range agreements {
		status := agreementGreenStyle.Render("ACTIVE")
		if !agreement.Active {
			status = agreementRedStyle.Render("INACTIVE")
		}

		fmt.Printf("  %s [%s] %s\n",
			agreementValueStyle.Render(agreement.ID),
			status,
			agreementDimStyle.Render(strings.ToUpper(string(agreement.Type))))

		fmt.Printf("    Version: %s | Created: %s\n",
			agreement.Version,
			agreement.CreatedAt.Format("2006-01-02"))

		if agreement.ExpiresAfter > 0 {
			fmt.Printf("    Expires after: %s\n", formatDurationShort(agreement.ExpiresAfter))
		} else {
			fmt.Printf("    Expires: Never\n")
		}

		// UNICODE: Rune-aware truncation preserves multi-byte characters
		preview := util.TruncateRunes(agreement.Content, 100)
		preview = strings.ReplaceAll(preview, "\n", " ")
		fmt.Printf("    %s\n", agreementDimStyle.Render(preview))

		fmt.Println()
	}

	fmt.Printf("  Total: %d agreement(s)\n", len(agreements))
	fmt.Println()

	return nil
}

// =============================================================================
// AGREEMENTS SHOW
// =============================================================================

// handleAgreementsShow shows the full text of an agreement.
func handleAgreementsShow(args AgreementsArgs) error {
	if args.AgreementID == "" {
		return fmt.Errorf("agreement ID required\n\nUsage: rigrun agreements show <id>")
	}

	manager := security.GlobalAgreementManager()
	agreement, err := manager.GetAgreement(args.AgreementID)
	if err != nil {
		return err
	}

	if args.JSON {
		data, err := json.MarshalIndent(agreement, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(agreementTitleStyle.Render(fmt.Sprintf("Agreement: %s", strings.ToUpper(args.AgreementID))))
	fmt.Println(agreementDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	// Show metadata
	fmt.Println(agreementSectionStyle.Render("Details"))
	fmt.Printf("  %s%s\n", agreementLabelStyle.Render("Type:"), strings.ToUpper(string(agreement.Type)))
	fmt.Printf("  %s%s\n", agreementLabelStyle.Render("Version:"), agreement.Version)
	fmt.Printf("  %s%s\n", agreementLabelStyle.Render("Created:"), agreement.CreatedAt.Format(time.RFC1123))

	if agreement.ExpiresAfter > 0 {
		fmt.Printf("  %s%s\n", agreementLabelStyle.Render("Signature Expires:"), formatDurationShort(agreement.ExpiresAfter))
	} else {
		fmt.Printf("  %s%s\n", agreementLabelStyle.Render("Signature Expires:"), "Never")
	}

	status := agreementGreenStyle.Render("ACTIVE")
	if !agreement.Active {
		status = agreementRedStyle.Render("INACTIVE")
	}
	fmt.Printf("  %s%s\n", agreementLabelStyle.Render("Status:"), status)

	fmt.Println()

	// Show content in a box
	fmt.Println(agreementSectionStyle.Render("Content"))
	fmt.Println()
	fmt.Println(agreementBoxStyle.Render(agreement.Content))
	fmt.Println()

	return nil
}

// =============================================================================
// AGREEMENTS SIGN
// =============================================================================

// handleAgreementsSign signs an agreement for the current user.
func handleAgreementsSign(args AgreementsArgs) error {
	if args.AgreementID == "" {
		return fmt.Errorf("agreement ID required\n\nUsage: rigrun agreements sign <id>")
	}

	manager := security.GlobalAgreementManager()

	// Get the agreement first
	agreement, err := manager.GetAgreement(args.AgreementID)
	if err != nil {
		return err
	}

	// Show the agreement content
	fmt.Println()
	fmt.Println(agreementTitleStyle.Render(fmt.Sprintf("Sign Agreement: %s", strings.ToUpper(args.AgreementID))))
	fmt.Println(agreementDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()
	fmt.Println(agreementBoxStyle.Render(agreement.Content))
	fmt.Println()

	// Prompt for confirmation
	fmt.Print("Do you agree to the above terms? (yes/no): ")
	var response string
	fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))
	if response != "yes" && response != "y" {
		fmt.Println()
		fmt.Println(agreementYellowStyle.Render("Agreement not signed."))
		fmt.Println()
		return nil
	}

	// Get user ID (in production, this would come from authentication)
	userID := args.UserID
	if userID == "" {
		userID = getCurrentUserID()
	}

	// Sign the agreement
	if err := manager.SignAgreement(userID, args.AgreementID); err != nil {
		return err
	}

	if args.JSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"signed":       true,
			"user_id":      userID,
			"agreement_id": args.AgreementID,
			"signed_at":    time.Now().Format(time.RFC3339),
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("%s Agreement signed successfully\n", agreementSuccessStyle.Render("[OK]"))
	fmt.Printf("  User ID:     %s\n", userID)
	fmt.Printf("  Agreement:   %s\n", args.AgreementID)
	fmt.Printf("  Signed at:   %s\n", time.Now().Format(time.RFC1123))
	fmt.Println()

	return nil
}

// =============================================================================
// AGREEMENTS STATUS
// =============================================================================

// handleAgreementsStatus shows the user's agreement status.
func handleAgreementsStatus(args AgreementsArgs) error {
	manager := security.GlobalAgreementManager()

	// Get user ID
	userID := args.UserID
	if userID == "" {
		userID = getCurrentUserID()
	}

	// Get user's agreements
	userAgreements := manager.GetUserAgreements(userID)

	if args.JSON {
		data, err := json.MarshalIndent(map[string]interface{}{
			"user_id":           userID,
			"signed_agreements": userAgreements,
			"total":             len(userAgreements),
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(agreementTitleStyle.Render("Agreement Status (PS-6)"))
	fmt.Println(agreementDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	fmt.Printf("  User ID: %s\n", userID)
	fmt.Println()

	if len(userAgreements) == 0 {
		fmt.Println(agreementYellowStyle.Render("  No agreements signed yet."))
		fmt.Println()
		return nil
	}

	// Show signed agreements
	fmt.Println(agreementSectionStyle.Render("Signed Agreements"))
	fmt.Println()

	for _, ua := range userAgreements {
		status := agreementGreenStyle.Render("VALID")
		if !ua.IsValid() {
			if ua.Revoked {
				status = agreementRedStyle.Render("REVOKED")
			} else {
				status = agreementRedStyle.Render("EXPIRED")
			}
		}

		fmt.Printf("  %s [%s]\n",
			agreementValueStyle.Render(ua.AgreementID),
			status)

		fmt.Printf("    Signed:  %s\n", ua.SignedAt.Format("2006-01-02 15:04"))
		fmt.Printf("    Version: %s\n", ua.Version)

		if !ua.ExpiresAt.IsZero() {
			if ua.IsValid() {
				remaining := ua.TimeRemaining()
				fmt.Printf("    Expires: %s (in %s)\n",
					ua.ExpiresAt.Format("2006-01-02"),
					formatDurationShort(remaining))
			} else {
				fmt.Printf("    Expired: %s\n", ua.ExpiresAt.Format("2006-01-02"))
			}
		} else {
			fmt.Printf("    Expires: Never\n")
		}

		if ua.Revoked {
			fmt.Printf("    Revoked: %s\n", ua.RevokedAt.Format("2006-01-02 15:04"))
			if ua.RevokedReason != "" {
				fmt.Printf("    Reason:  %s\n", ua.RevokedReason)
			}
		}

		fmt.Println()
	}

	return nil
}

// =============================================================================
// AGREEMENTS CHECK
// =============================================================================

// handleAgreementsCheck verifies all required agreements are signed.
func handleAgreementsCheck(args AgreementsArgs) error {
	manager := security.GlobalAgreementManager()

	// Get user ID
	userID := args.UserID
	if userID == "" {
		userID = getCurrentUserID()
	}

	// Check agreements
	valid, missing := manager.CheckAgreementsValid(userID)

	if args.JSON {
		data, err := json.MarshalIndent(map[string]interface{}{
			"user_id":             userID,
			"all_signed":          valid,
			"missing_agreements":  missing,
			"ps6_compliant":       valid,
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(agreementTitleStyle.Render("PS-6 Compliance Check"))
	fmt.Println(agreementDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	fmt.Printf("  User ID: %s\n", userID)
	fmt.Println()

	if valid {
		fmt.Printf("  %s All required agreements are signed\n", agreementSuccessStyle.Render("[OK]"))
		fmt.Println()
		fmt.Println(agreementGreenStyle.Render("  PS-6 Compliance: COMPLIANT"))
	} else {
		fmt.Printf("  %s Missing required agreements\n", agreementErrorStyle.Render("[FAIL]"))
		fmt.Println()
		fmt.Println(agreementYellowStyle.Render("  Missing Agreements:"))
		for _, id := range missing {
			fmt.Printf("    - %s\n", id)
		}
		fmt.Println()
		fmt.Println(agreementRedStyle.Render("  PS-6 Compliance: NON-COMPLIANT"))
		fmt.Println()
		fmt.Println(agreementDimStyle.Render("  Sign missing agreements with: rigrun agreements sign <id>"))
	}

	fmt.Println()

	return nil
}

// =============================================================================
// HANDLE RULES
// =============================================================================

// RulesArgs holds parsed rules command arguments.
type RulesArgs struct {
	Subcommand string
	Category   string
	UserID     string
	JSON       bool
}

// parseRulesCmdArgs parses rules command specific arguments.
func parseRulesCmdArgs(args *Args, remaining []string) RulesArgs {
	rulesArgs := RulesArgs{
		JSON: args.JSON,
	}

	if len(remaining) > 0 {
		rulesArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--category", "-c":
			if i+1 < len(remaining) {
				i++
				rulesArgs.Category = remaining[i]
			}
		case "--user", "-u":
			if i+1 < len(remaining) {
				i++
				rulesArgs.UserID = remaining[i]
			}
		case "--json":
			rulesArgs.JSON = true
		}
	}

	return rulesArgs
}

// HandleRules handles the "rules" command with various subcommands.
// Implements PL-4 management commands.
func HandleRules(args Args) error {
	rulesArgs := parseRulesCmdArgs(&args, args.Raw)

	switch rulesArgs.Subcommand {
	case "", "show":
		return handleRulesShow(rulesArgs)
	case "acknowledge":
		return handleRulesAcknowledge(rulesArgs)
	case "status":
		return handleRulesStatus(rulesArgs)
	default:
		return fmt.Errorf("unknown rules subcommand: %s\n\nUsage:\n"+
			"  rigrun rules show               Show rules of behavior\n"+
			"  rigrun rules acknowledge        Acknowledge rules\n"+
			"  rigrun rules status             Show acknowledgment status", rulesArgs.Subcommand)
	}
}

// =============================================================================
// RULES SHOW
// =============================================================================

// handleRulesShow shows the rules of behavior.
func handleRulesShow(args RulesArgs) error {
	manager := security.GlobalRulesManager()

	var rules []*security.Rule
	if args.Category != "" {
		category := security.RuleCategory(args.Category)
		rules = manager.GetRulesByCategory(category)
	} else {
		rules = manager.GetRulesOfBehavior()
	}

	if args.JSON {
		data, err := json.MarshalIndent(rules, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(agreementTitleStyle.Render("Rules of Behavior (PL-4)"))
	fmt.Println(agreementDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	if len(rules) == 0 {
		fmt.Println(agreementDimStyle.Render("  No rules found."))
		fmt.Println()
		return nil
	}

	// Group by category
	categoryRules := make(map[security.RuleCategory][]*security.Rule)
	for _, rule := range rules {
		categoryRules[rule.Category] = append(categoryRules[rule.Category], rule)
	}

	// Display each category
	categories := []security.RuleCategory{
		security.RulesCategoryGeneral,
		security.RulesCategorySecurity,
		security.RulesCategoryData,
		security.RulesCategoryNetwork,
		security.RulesCategoryIncident,
	}

	for _, category := range categories {
		catRules := categoryRules[category]
		if len(catRules) == 0 {
			continue
		}

		fmt.Println(agreementSectionStyle.Render(strings.ToUpper(string(category)) + " RULES"))
		fmt.Println()

		for _, rule := range catRules {
			marker := "*"
			if rule.Required {
				marker = agreementRedStyle.Render("[!]")
			}

			fmt.Printf("  %s %s\n", marker, rule.Description)
		}

		fmt.Println()
	}

	fmt.Println(agreementDimStyle.Render("  * Required rules must be acknowledged"))
	fmt.Println()

	return nil
}

// =============================================================================
// RULES ACKNOWLEDGE
// =============================================================================

// handleRulesAcknowledge records acknowledgment of rules.
func handleRulesAcknowledge(args RulesArgs) error {
	manager := security.GlobalRulesManager()

	// Show rules first
	rules := manager.GetRulesOfBehavior()

	fmt.Println()
	fmt.Println(agreementTitleStyle.Render("Acknowledge Rules of Behavior (PL-4)"))
	fmt.Println(agreementDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	// Display rules by category
	categoryRules := make(map[security.RuleCategory][]*security.Rule)
	for _, rule := range rules {
		categoryRules[rule.Category] = append(categoryRules[rule.Category], rule)
	}

	categories := []security.RuleCategory{
		security.RulesCategoryGeneral,
		security.RulesCategorySecurity,
		security.RulesCategoryData,
		security.RulesCategoryNetwork,
		security.RulesCategoryIncident,
	}

	for _, category := range categories {
		catRules := categoryRules[category]
		if len(catRules) == 0 {
			continue
		}

		fmt.Println(agreementSectionStyle.Render(strings.ToUpper(string(category))))
		for _, rule := range catRules {
			fmt.Printf("  * %s\n", rule.Description)
		}
		fmt.Println()
	}

	// Prompt for acknowledgment
	fmt.Print("Do you acknowledge and agree to follow these rules? (yes/no): ")
	var response string
	fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))
	if response != "yes" && response != "y" {
		fmt.Println()
		fmt.Println(agreementYellowStyle.Render("Rules not acknowledged."))
		fmt.Println()
		return nil
	}

	// Get user ID
	userID := args.UserID
	if userID == "" {
		userID = getCurrentUserID()
	}

	// Acknowledge rules
	if err := manager.AcknowledgeRules(userID); err != nil {
		return err
	}

	if args.JSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"acknowledged": true,
			"user_id":      userID,
			"acknowledged_at": time.Now().Format(time.RFC3339),
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("%s Rules acknowledged successfully\n", agreementSuccessStyle.Render("[OK]"))
	fmt.Printf("  User ID:        %s\n", userID)
	fmt.Printf("  Acknowledged:   %s\n", time.Now().Format(time.RFC1123))
	fmt.Println()

	return nil
}

// =============================================================================
// RULES STATUS
// =============================================================================

// handleRulesStatus shows the user's rules acknowledgment status.
func handleRulesStatus(args RulesArgs) error {
	manager := security.GlobalRulesManager()

	// Get user ID
	userID := args.UserID
	if userID == "" {
		userID = getCurrentUserID()
	}

	// Check acknowledgment
	acknowledged := manager.HasAcknowledgedRules(userID)

	// Get acknowledgment details if exists
	var ack *security.RulesAcknowledgment
	if acknowledged {
		ack, _ = manager.GetUserAcknowledgment(userID)
	}

	if args.JSON {
		result := map[string]interface{}{
			"user_id":       userID,
			"acknowledged":  acknowledged,
			"pl4_compliant": acknowledged,
		}
		if ack != nil {
			result["acknowledged_at"] = ack.AcknowledgedAt.Format(time.RFC3339)
			result["rules_version"] = ack.RulesVersion
		}
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(agreementTitleStyle.Render("Rules Acknowledgment Status (PL-4)"))
	fmt.Println(agreementDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	fmt.Printf("  User ID: %s\n", userID)
	fmt.Println()

	if acknowledged && ack != nil {
		fmt.Printf("  %s Rules acknowledged\n", agreementSuccessStyle.Render("[OK]"))
		fmt.Printf("    Date:    %s\n", ack.AcknowledgedAt.Format(time.RFC1123))
		fmt.Printf("    Version: %s\n", ack.RulesVersion)
		fmt.Println()
		fmt.Println(agreementGreenStyle.Render("  PL-4 Compliance: COMPLIANT"))
	} else {
		fmt.Printf("  %s Rules not acknowledged\n", agreementErrorStyle.Render("[FAIL]"))
		fmt.Println()
		fmt.Println(agreementRedStyle.Render("  PL-4 Compliance: NON-COMPLIANT"))
		fmt.Println()
		fmt.Println(agreementDimStyle.Render("  Acknowledge rules with: rigrun rules acknowledge"))
	}

	fmt.Println()

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================
