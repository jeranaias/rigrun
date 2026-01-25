// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// auth_cmd.go - CLI commands for IA-2 authentication management in rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements NIST 800-53 IA-2 (Identification and Authentication) controls
// for DoD IL5 compliance.
//
// Command: auth [subcommand]
// Short:   Authentication management (IL5 IA-2)
// Aliases: (none)
//
// Subcommands:
//   status (default)    Show authentication status
//   login               Authenticate (API key validation)
//   logout              End authenticated session
//   mfa                 Show MFA status
//   sessions            List active sessions
//
// Examples:
//   rigrun auth                        Show status (default)
//   rigrun auth status                 Show authentication status
//   rigrun auth status --json          Status in JSON format
//   rigrun auth login                  Start authentication
//   rigrun auth logout                 End current session
//   rigrun auth mfa status             Check MFA status
//   rigrun auth sessions               List active sessions
//
// IA-2 Authentication:
//   - API key based authentication for cloud services
//   - Session tracking with timeouts (AC-12)
//   - MFA support (placeholder for future)
//   - All auth events logged to audit
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
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// AUTH COMMAND STYLES
// =============================================================================

var (
	authTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")). // Cyan
			MarginBottom(1)

	authSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	authLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")). // Light gray
			Width(20)

	authValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")) // White

	authGreenStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")) // Green

	authRedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red

	authYellowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")) // Yellow

	authDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Dim

	authSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	authErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
)

// =============================================================================
// AUTH ARGUMENTS
// =============================================================================

// AuthArgs holds parsed auth command arguments.
type AuthArgs struct {
	Subcommand string
	APIKey     string
	SessionID  string
	JSON       bool
}

// parseAuthCmdArgs parses auth command specific arguments.
func parseAuthCmdArgs(args *Args, remaining []string) AuthArgs {
	authArgs := AuthArgs{
		JSON: args.JSON,
	}

	if len(remaining) > 0 {
		authArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--key", "-k":
			if i+1 < len(remaining) {
				i++
				authArgs.APIKey = remaining[i]
			}
		case "--session", "-s":
			if i+1 < len(remaining) {
				i++
				authArgs.SessionID = remaining[i]
			}
		case "--json":
			authArgs.JSON = true
		default:
			if strings.HasPrefix(arg, "--key=") {
				authArgs.APIKey = strings.TrimPrefix(arg, "--key=")
			} else if strings.HasPrefix(arg, "--session=") {
				authArgs.SessionID = strings.TrimPrefix(arg, "--session=")
			}
		}
	}

	return authArgs
}

// =============================================================================
// HANDLE AUTH
// =============================================================================

// HandleAuth handles the "auth" command with various subcommands.
// Implements IA-2 management commands.
func HandleAuth(args Args) error {
	authArgs := parseAuthCmdArgs(&args, args.Raw)

	switch authArgs.Subcommand {
	case "", "status":
		return handleAuthStatus(authArgs)
	case "login":
		return handleAuthLogin(authArgs)
	case "logout":
		return handleAuthLogout(authArgs)
	case "validate":
		return handleAuthValidate(authArgs)
	case "sessions":
		return handleAuthSessions(authArgs)
	case "mfa":
		return handleAuthMFA(authArgs)
	default:
		return fmt.Errorf("unknown auth subcommand: %s\n\nUsage:\n"+
			"  rigrun auth status              Show authentication status\n"+
			"  rigrun auth login [--key KEY]   Authenticate with API key\n"+
			"  rigrun auth logout              End current session\n"+
			"  rigrun auth validate            Validate configured API key\n"+
			"  rigrun auth sessions            List active sessions\n"+
			"  rigrun auth mfa status          Show MFA status (placeholder)", authArgs.Subcommand)
	}
}

// =============================================================================
// AUTH STATUS
// =============================================================================

// AuthStatusOutput is the JSON output format for auth status.
type AuthStatusOutput struct {
	Authenticated   bool   `json:"authenticated"`
	APIKeyConfigured bool   `json:"api_key_configured"`
	APIKeyMasked    string `json:"api_key_masked,omitempty"`
	SessionID       string `json:"session_id,omitempty"`
	UserID          string `json:"user_id,omitempty"`
	AuthMethod      string `json:"auth_method,omitempty"`
	ExpiresIn       string `json:"expires_in,omitempty"`
	MFAEnabled      bool   `json:"mfa_enabled"`
	MFAVerified     bool   `json:"mfa_verified"`
	LockoutEnabled  bool   `json:"lockout_enabled"`
	IA2Compliant    bool   `json:"ia2_compliant"`
}

// handleAuthStatus shows the current authentication status.
func handleAuthStatus(args AuthArgs) error {
	cfg := config.Global()
	authManager := security.GlobalAuthManager()
	stats := authManager.GetStats()

	// Check if API key is configured
	apiKey := cfg.Cloud.OpenRouterKey
	if apiKey == "" {
		apiKey = security.GetAPIKeyFromEnv()
	}

	apiKeyConfigured := apiKey != ""
	apiKeyMasked := ""
	if apiKeyConfigured {
		apiKeyMasked = maskAPIKey(apiKey)
	}

	// Check for active session
	sessions := authManager.ListSessions()
	var currentSession *security.AuthSession
	if len(sessions) > 0 {
		currentSession = sessions[0]
	}

	// Determine IA-2 compliance
	ia2Compliant := apiKeyConfigured && stats.LockoutEnabled

	output := AuthStatusOutput{
		Authenticated:    currentSession != nil && currentSession.IsValid(),
		APIKeyConfigured: apiKeyConfigured,
		APIKeyMasked:     apiKeyMasked,
		MFAEnabled:       stats.MFAEnabled,
		LockoutEnabled:   stats.LockoutEnabled,
		IA2Compliant:     ia2Compliant,
	}

	if currentSession != nil && currentSession.IsValid() {
		output.SessionID = currentSession.SessionID
		output.UserID = currentSession.UserID
		output.AuthMethod = string(currentSession.AuthMethod)
		output.ExpiresIn = currentSession.TimeRemaining().String()
		output.MFAVerified = currentSession.MFAVerified
	}

	if args.JSON {
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Display human-readable status
	separator := strings.Repeat("=", 50)
	fmt.Println()
	fmt.Println(authTitleStyle.Render("IA-2 Authentication Status"))
	fmt.Println(authDimStyle.Render(separator))
	fmt.Println()

	// API Key section
	fmt.Println(authSectionStyle.Render("API Key"))
	if apiKeyConfigured {
		fmt.Printf("  %s%s %s\n",
			authLabelStyle.Render("Status:"),
			authGreenStyle.Render("CONFIGURED"),
			authDimStyle.Render("("+apiKeyMasked+")"))
	} else {
		fmt.Printf("  %s%s\n",
			authLabelStyle.Render("Status:"),
			authRedStyle.Render("NOT CONFIGURED"))
		fmt.Println()
		fmt.Println(authYellowStyle.Render("  Configure API key with:"))
		fmt.Println("    export OPENROUTER_API_KEY=sk-or-...")
		fmt.Println("    or")
		fmt.Println("    rigrun config set cloud.openrouter_key sk-or-...")
	}
	fmt.Println()

	// Session section
	fmt.Println(authSectionStyle.Render("Session"))
	if currentSession != nil && currentSession.IsValid() {
		fmt.Printf("  %s%s\n", authLabelStyle.Render("Authenticated:"), authGreenStyle.Render("YES"))
		fmt.Printf("  %s%s\n", authLabelStyle.Render("Session ID:"), authDimStyle.Render(currentSession.SessionID[:16]+"..."))
		fmt.Printf("  %s%s\n", authLabelStyle.Render("User ID:"), authValueStyle.Render(currentSession.UserID))
		fmt.Printf("  %s%s\n", authLabelStyle.Render("Auth Method:"), authValueStyle.Render(string(currentSession.AuthMethod)))
		fmt.Printf("  %s%s\n", authLabelStyle.Render("Expires In:"), authValueStyle.Render(formatDurationShort(currentSession.TimeRemaining())))
	} else {
		fmt.Printf("  %s%s\n", authLabelStyle.Render("Authenticated:"), authRedStyle.Render("NO"))
		fmt.Println()
		fmt.Println(authDimStyle.Render("  Run 'rigrun auth login' to authenticate"))
	}
	fmt.Println()

	// MFA section (placeholder)
	fmt.Println(authSectionStyle.Render("MFA (IA-2(1) Placeholder)"))
	if stats.MFAEnabled {
		fmt.Printf("  %s%s\n", authLabelStyle.Render("Status:"), authGreenStyle.Render("ENABLED"))
		if currentSession != nil {
			if currentSession.MFAVerified {
				fmt.Printf("  %s%s\n", authLabelStyle.Render("Verified:"), authGreenStyle.Render("YES"))
			} else {
				fmt.Printf("  %s%s\n", authLabelStyle.Render("Verified:"), authYellowStyle.Render("PENDING"))
			}
		}
	} else {
		fmt.Printf("  %s%s\n", authLabelStyle.Render("Status:"), authDimStyle.Render("NOT ENABLED (placeholder)"))
	}
	fmt.Println()

	// Compliance section
	fmt.Println(authSectionStyle.Render("NIST 800-53 IA-2 Compliance"))
	complianceStr := authRedStyle.Render("NON-COMPLIANT")
	if ia2Compliant {
		complianceStr = authGreenStyle.Render("COMPLIANT")
	}
	fmt.Printf("  %s%s\n", authLabelStyle.Render("Status:"), complianceStr)

	if !ia2Compliant {
		fmt.Println()
		if !apiKeyConfigured {
			fmt.Println(authYellowStyle.Render("  Missing: API key configuration"))
		}
		if !stats.LockoutEnabled {
			fmt.Println(authYellowStyle.Render("  Missing: Lockout protection (AC-7)"))
		}
	}

	fmt.Println()
	return nil
}

// =============================================================================
// AUTH LOGIN
// =============================================================================

// handleAuthLogin performs authentication.
func handleAuthLogin(args AuthArgs) error {
	authManager := security.GlobalAuthManager()

	// Get API key from args, config, or prompt
	apiKey := args.APIKey
	if apiKey == "" {
		apiKey = config.Global().Cloud.OpenRouterKey
	}
	if apiKey == "" {
		apiKey = security.GetAPIKeyFromEnv()
	}

	if apiKey == "" {
		// Prompt for API key (secure input without echo)
		fmt.Println()
		fmt.Print("Enter API key: ")
		keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("failed to read API key: %w", err)
		}
		fmt.Println() // Add newline after hidden input
		apiKey = strings.TrimSpace(string(keyBytes))
	}

	if apiKey == "" {
		return fmt.Errorf("API key required")
	}

	// Attempt authentication
	session, err := authManager.Authenticate(security.AuthMethodAPIKey, apiKey)
	if err != nil {
		if args.JSON {
			data, _ := json.MarshalIndent(map[string]interface{}{
				"authenticated": false,
				"error":         err.Error(),
			}, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Println()
		fmt.Printf("%s Authentication failed: %s\n", authErrorStyle.Render("[ERROR]"), err.Error())
		fmt.Println()
		return nil
	}

	if args.JSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"authenticated": true,
			"session_id":    session.SessionID,
			"user_id":       session.UserID,
			"expires_at":    session.ExpiresAt.Format(time.RFC3339),
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("%s Authenticated successfully\n", authSuccessStyle.Render("[OK]"))
	fmt.Printf("  Session ID: %s\n", authDimStyle.Render(session.SessionID[:16]+"..."))
	fmt.Printf("  User ID:    %s\n", session.UserID)
	fmt.Printf("  Expires:    %s\n", session.ExpiresAt.Format(time.RFC1123))
	fmt.Println()

	return nil
}

// =============================================================================
// AUTH LOGOUT
// =============================================================================

// handleAuthLogout ends the current session.
func handleAuthLogout(args AuthArgs) error {
	authManager := security.GlobalAuthManager()

	// Find current session
	sessions := authManager.ListSessions()
	if len(sessions) == 0 {
		if args.JSON {
			data, _ := json.MarshalIndent(map[string]interface{}{
				"logged_out": false,
				"message":    "no active session",
			}, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		fmt.Println()
		fmt.Println(authDimStyle.Render("No active session to log out."))
		fmt.Println()
		return nil
	}

	// Logout the first session (or specified session)
	sessionToLogout := sessions[0]
	if args.SessionID != "" {
		found := false
		for _, s := range sessions {
			if strings.HasPrefix(s.SessionID, args.SessionID) {
				sessionToLogout = s
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("session not found: %s", args.SessionID)
		}
	}

	authManager.Logout(sessionToLogout.SessionID)

	if args.JSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"logged_out": true,
			"session_id": sessionToLogout.SessionID,
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("%s Logged out successfully\n", authSuccessStyle.Render("[OK]"))
	fmt.Printf("  Session: %s\n", authDimStyle.Render(sessionToLogout.SessionID[:16]+"..."))
	fmt.Println()

	return nil
}

// =============================================================================
// AUTH VALIDATE
// =============================================================================

// handleAuthValidate validates the configured API key.
func handleAuthValidate(args AuthArgs) error {
	authManager := security.GlobalAuthManager()

	// Get API key
	apiKey := args.APIKey
	if apiKey == "" {
		apiKey = config.Global().Cloud.OpenRouterKey
	}
	if apiKey == "" {
		apiKey = security.GetAPIKeyFromEnv()
	}

	if apiKey == "" {
		if args.JSON {
			data, _ := json.MarshalIndent(map[string]interface{}{
				"valid":   false,
				"message": "no API key configured",
			}, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		fmt.Println()
		fmt.Printf("%s No API key configured\n", authYellowStyle.Render("[WARN]"))
		fmt.Println()
		return nil
	}

	// Validate with lockout tracking
	valid, err := authManager.ValidateAPIKeyWithLockout(apiKey)

	if args.JSON {
		result := map[string]interface{}{
			"valid": valid,
		}
		if err != nil {
			result["error"] = err.Error()
		}
		if valid {
			result["key_fingerprint"] = maskAPIKey(apiKey)
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	if valid {
		fmt.Printf("%s API key is valid\n", authSuccessStyle.Render("[OK]"))
		fmt.Printf("  Fingerprint: %s\n", authDimStyle.Render(maskAPIKey(apiKey)))
	} else {
		fmt.Printf("%s API key validation failed\n", authErrorStyle.Render("[ERROR]"))
		if err != nil {
			fmt.Printf("  Error: %s\n", err.Error())
		}
	}
	fmt.Println()

	return nil
}

// =============================================================================
// AUTH SESSIONS
// =============================================================================

// handleAuthSessions lists active sessions.
func handleAuthSessions(args AuthArgs) error {
	authManager := security.GlobalAuthManager()
	sessions := authManager.ListSessions()

	if args.JSON {
		data, err := json.MarshalIndent(map[string]interface{}{
			"sessions": sessions,
			"count":    len(sessions),
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(authTitleStyle.Render("Active Sessions"))
	fmt.Println(authDimStyle.Render(strings.Repeat("=", 50)))
	fmt.Println()

	if len(sessions) == 0 {
		fmt.Println(authDimStyle.Render("  No active sessions."))
		fmt.Println()
		return nil
	}

	// Table header
	fmt.Printf("  %-20s %-12s %-15s %-12s\n", "Session ID", "User ID", "Auth Method", "Expires In")
	fmt.Println(authDimStyle.Render("  " + strings.Repeat("-", 60)))

	for _, session := range sessions {
		sessionID := session.SessionID
		if len(sessionID) > 18 {
			sessionID = sessionID[:15] + "..."
		}

		fmt.Printf("  %-20s %-12s %-15s %s\n",
			sessionID,
			session.UserID,
			session.AuthMethod,
			formatDurationShort(session.TimeRemaining()))
	}

	fmt.Println()
	fmt.Printf("  Total: %d active session(s)\n", len(sessions))
	fmt.Println()

	return nil
}

// =============================================================================
// AUTH MFA
// =============================================================================

// handleAuthMFA handles MFA subcommands (placeholder).
func handleAuthMFA(args AuthArgs) error {
	// Parse MFA subcommand
	mfaSubcommand := "status"
	if len(args.Subcommand) > 0 {
		parts := strings.Split(args.Subcommand, " ")
		if len(parts) > 1 {
			mfaSubcommand = parts[1]
		}
	}

	authManager := security.GlobalAuthManager()

	switch mfaSubcommand {
	case "status":
		if args.JSON {
			data, _ := json.MarshalIndent(map[string]interface{}{
				"mfa_enabled":    authManager.IsMFARequired(),
				"mfa_status":     authManager.MFAStatus(),
				"implementation": "placeholder",
				"ia2_1_control":  "IA-2(1) Multi-factor Authentication",
			}, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Println()
		fmt.Println(authTitleStyle.Render("MFA Status (IA-2(1) Placeholder)"))
		fmt.Println(authDimStyle.Render(strings.Repeat("=", 50)))
		fmt.Println()

		fmt.Println(authSectionStyle.Render("Status"))
		fmt.Printf("  %s%s\n", authLabelStyle.Render("MFA Enabled:"), authDimStyle.Render(fmt.Sprintf("%v", authManager.IsMFARequired())))
		fmt.Println()

		fmt.Println(authSectionStyle.Render("IA-2(1) Compliance"))
		fmt.Println(authYellowStyle.Render("  Multi-factor authentication is a placeholder for future implementation."))
		fmt.Println()
		fmt.Println("  IA-2(1) requires network access to privileged accounts to use MFA.")
		fmt.Println("  Supported methods (planned):")
		fmt.Println("    - TOTP (Time-based One-Time Password)")
		fmt.Println("    - Hardware tokens (YubiKey, etc.)")
		fmt.Println("    - Push notifications")
		fmt.Println()

		return nil

	default:
		return fmt.Errorf("unknown MFA subcommand: %s\nUsage: rigrun auth mfa status", mfaSubcommand)
	}
}

// =============================================================================
// HELPER: READ PASSWORD (CROSS-PLATFORM)
// =============================================================================

// readPassword reads a password from stdin without echoing.
// Uses golang.org/x/term for secure cross-platform password input.
func readPassword() (string, error) {
	// Use term.ReadPassword for secure input without echo
	passBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	fmt.Println() // Add newline after hidden input

	return strings.TrimSpace(string(passBytes)), nil
}

// =============================================================================
// PARSE AUTH ARGS (FOR CLI.GO INTEGRATION)
// =============================================================================

// parseAuthArgsFromCLI parses auth command arguments from CLI.
func parseAuthArgsFromCLI(args *Args, remaining []string) {
	if len(remaining) > 0 {
		args.Subcommand = remaining[0]
	}
}

// Ensure syscall is used (for future Windows password masking)
var _ = syscall.Stdin
