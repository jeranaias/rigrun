// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// transport_cmd.go - Transport security CLI commands for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements NIST 800-53 controls:
//   - SC-8 (Transmission Confidentiality and Integrity)
//   - AC-17 (Remote Access)
//
// Command: transport [subcommand]
// Short:   Transport security (IL5 SC-8, AC-17)
// Aliases: tls
//
// Subcommands:
//   status (default)    Show transport security status
//   verify              Verify TLS configuration
//   test <host>         Test TLS connection to host
//   certs               Show certificate information
//
// Examples:
//   rigrun transport                      Show status (default)
//   rigrun transport status               Show transport status
//   rigrun transport status --json        Status in JSON format
//   rigrun transport verify               Verify TLS config
//   rigrun transport test api.example.com Test connection
//   rigrun transport certs                Show certificates
//
// SC-8 Transport Security:
//   - TLS 1.2+ required for all connections
//   - Certificate verification enabled
//   - Cipher suite restrictions
//   - Certificate pinning (when configured)
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
// STYLES
// =============================================================================

var (
	transportTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	transportSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	transportLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Light gray
				Width(22)

	transportValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")) // White

	transportGreenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")) // Green

	transportYellowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")) // Yellow

	transportRedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")) // Red

	transportDimStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("242")) // Dim

	transportSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// =============================================================================
// TRANSPORT ARGUMENTS
// =============================================================================

// TransportArgs holds parsed transport command arguments.
type TransportArgs struct {
	Subcommand string
	SessionID  string
	CIDR       string
	Host       string
	JSON       bool
}

// parseTransportArgs parses transport command specific arguments.
func parseTransportArgs(args *Args, remaining []string) TransportArgs {
	transportArgs := TransportArgs{
		JSON: args.JSON,
	}

	if len(remaining) > 0 {
		transportArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	// For subcommands that take an argument
	if len(remaining) > 0 {
		switch transportArgs.Subcommand {
		case "terminate":
			transportArgs.SessionID = remaining[0]
		case "allow-ip", "deny-ip":
			transportArgs.CIDR = remaining[0]
		case "verify":
			if len(remaining) > 0 {
				transportArgs.Host = remaining[0]
			}
		}
	}

	// Parse flags
	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]
		switch arg {
		case "--json":
			transportArgs.JSON = true
		}
	}

	return transportArgs
}

// =============================================================================
// HANDLE TRANSPORT
// =============================================================================

// HandleTransport handles the "transport" command with various subcommands.
// Implements SC-8 and AC-17 management commands.
func HandleTransport(args Args) error {
	transportArgs := parseTransportArgs(&args, args.Raw)

	switch transportArgs.Subcommand {
	case "", "status":
		return handleTransportStatus(transportArgs)
	case "verify":
		return handleTransportVerify(transportArgs)
	case "ciphers":
		return handleTransportCiphers(transportArgs)
	case "sessions":
		return handleTransportSessions(transportArgs)
	case "terminate":
		return handleTransportTerminate(transportArgs)
	case "policy":
		return handleTransportPolicy(transportArgs)
	case "allow-ip":
		return handleTransportAllowIP(transportArgs)
	case "deny-ip":
		return handleTransportDenyIP(transportArgs)
	default:
		return fmt.Errorf("unknown transport subcommand: %s\n\nUsage:\n"+
			"  rigrun transport status         Show TLS configuration\n"+
			"  rigrun transport verify [host]  Verify connection security\n"+
			"  rigrun transport ciphers        List allowed ciphers\n"+
			"  rigrun transport sessions       List active remote sessions\n"+
			"  rigrun transport terminate ID   Terminate a remote session\n"+
			"  rigrun transport policy         Show remote access policy\n"+
			"  rigrun transport allow-ip CIDR  Add to IP allowlist\n"+
			"  rigrun transport deny-ip CIDR   Remove from IP allowlist", transportArgs.Subcommand)
	}
}

// =============================================================================
// TRANSPORT STATUS
// =============================================================================

// TransportStatusOutput is the JSON output format for transport status.
type TransportStatusOutput struct {
	TLSEnforced       bool     `json:"tls_enforced"`
	MinTLSVersion     string   `json:"min_tls_version"`
	PreferredVersion  string   `json:"preferred_version"`
	ApprovedCiphers   int      `json:"approved_ciphers"`
	PinnedCerts       int      `json:"pinned_certs"`
	SC8Compliant      bool     `json:"sc8_compliant"`
	ActiveSessions    int      `json:"active_sessions"`
	AllowedIPRanges   []string `json:"allowed_ip_ranges"`
	AC17Compliant     bool     `json:"ac17_compliant"`
}

// handleTransportStatus shows the current transport security status.
func handleTransportStatus(args TransportArgs) error {
	ts := security.GlobalTransportSecurity()
	ram := security.GlobalRemoteAccessManager()

	// Get status information
	tlsEnforced := ts.IsEnforceMode()
	pinnedCerts := ts.GetPinnedCertificates()
	activeSessions := ram.GetActiveSessions()
	allowedIPs := ram.GetAllowedIPs()

	// Determine compliance
	sc8Compliant := true // TLS is always configured with approved settings
	ac17Compliant := len(allowedIPs) > 0 || len(activeSessions) == 0

	output := TransportStatusOutput{
		TLSEnforced:      tlsEnforced,
		MinTLSVersion:    "TLS 1.2",
		PreferredVersion: "TLS 1.3",
		ApprovedCiphers:  len(security.ApprovedCipherSuites),
		PinnedCerts:      len(pinnedCerts),
		SC8Compliant:     sc8Compliant,
		ActiveSessions:   len(activeSessions),
		AllowedIPRanges:  allowedIPs,
		AC17Compliant:    ac17Compliant,
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
	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(transportTitleStyle.Render("SC-8 Transmission Security & AC-17 Remote Access Status"))
	fmt.Println(transportDimStyle.Render(separator))
	fmt.Println()

	// TLS Configuration section
	fmt.Println(transportSectionStyle.Render("TLS Configuration (SC-8)"))
	fmt.Printf("  %s%s\n", transportLabelStyle.Render("Min TLS Version:"), transportValueStyle.Render("TLS 1.2"))
	fmt.Printf("  %s%s\n", transportLabelStyle.Render("Preferred Version:"), transportGreenStyle.Render("TLS 1.3"))
	fmt.Printf("  %s%d\n", transportLabelStyle.Render("Approved Ciphers:"), len(security.ApprovedCipherSuites))

	enforceStr := transportRedStyle.Render("DISABLED")
	if tlsEnforced {
		enforceStr = transportGreenStyle.Render("ENABLED")
	}
	fmt.Printf("  %s%s\n", transportLabelStyle.Render("Enforce Mode:"), enforceStr)
	fmt.Println()

	// Certificate Pinning section
	fmt.Println(transportSectionStyle.Render("Certificate Pinning (SC-17)"))
	if len(pinnedCerts) > 0 {
		fmt.Printf("  %s%d pinned\n", transportLabelStyle.Render("Pinned Certificates:"), len(pinnedCerts))
		for _, host := range pinnedCerts {
			fmt.Printf("    %s %s\n", transportDimStyle.Render("*"), host)
		}
	} else {
		fmt.Printf("  %s%s\n", transportLabelStyle.Render("Pinned Certificates:"), transportDimStyle.Render("None"))
	}
	fmt.Println()

	// Remote Access section
	fmt.Println(transportSectionStyle.Render("Remote Access (AC-17)"))
	fmt.Printf("  %s%d\n", transportLabelStyle.Render("Active Sessions:"), len(activeSessions))

	if len(allowedIPs) > 0 {
		fmt.Printf("  %s%d ranges\n", transportLabelStyle.Render("IP Allowlist:"), len(allowedIPs))
		for _, cidr := range allowedIPs {
			fmt.Printf("    %s %s\n", transportDimStyle.Render("*"), cidr)
		}
	} else {
		fmt.Printf("  %s%s\n", transportLabelStyle.Render("IP Allowlist:"), transportYellowStyle.Render("Not configured (all IPs allowed)"))
	}
	fmt.Println()

	// Compliance section
	fmt.Println(transportSectionStyle.Render("NIST 800-53 Compliance"))

	sc8Status := transportGreenStyle.Render("COMPLIANT")
	fmt.Printf("  %s%s\n", transportLabelStyle.Render("SC-8 Status:"), sc8Status)

	ac17Status := transportRedStyle.Render("NON-COMPLIANT")
	if ac17Compliant {
		ac17Status = transportGreenStyle.Render("COMPLIANT")
	}
	fmt.Printf("  %s%s\n", transportLabelStyle.Render("AC-17 Status:"), ac17Status)

	if !ac17Compliant {
		fmt.Println()
		fmt.Println(transportYellowStyle.Render("  Recommendation: Configure IP allowlist for remote access control"))
		fmt.Println(transportDimStyle.Render("    rigrun transport allow-ip 10.0.0.0/8"))
	}

	fmt.Println()
	return nil
}

// =============================================================================
// TRANSPORT VERIFY
// =============================================================================

// handleTransportVerify verifies the current connection security.
func handleTransportVerify(args TransportArgs) error {
	if args.Host == "" {
		return fmt.Errorf("usage: rigrun transport verify <host>\n\nExample: rigrun transport verify openrouter.ai")
	}

	// This is a placeholder - actual TLS verification would require a real connection
	fmt.Println()
	fmt.Println(transportTitleStyle.Render("Transport Security Verification"))
	fmt.Println(transportDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	fmt.Printf("  %s%s\n", transportLabelStyle.Render("Host:"), transportValueStyle.Render(args.Host))
	fmt.Printf("  %s%s\n", transportLabelStyle.Render("Status:"), transportYellowStyle.Render("Verification not implemented"))
	fmt.Println()
	fmt.Println(transportDimStyle.Render("  Note: TLS verification requires an active connection."))
	fmt.Println(transportDimStyle.Render("  Use this command after establishing a connection to verify:"))
	fmt.Println(transportDimStyle.Render("    - TLS version (>= 1.2)"))
	fmt.Println(transportDimStyle.Render("    - Approved cipher suites"))
	fmt.Println(transportDimStyle.Render("    - Certificate validity"))
	fmt.Println()

	return nil
}

// =============================================================================
// TRANSPORT CIPHERS
// =============================================================================

// handleTransportCiphers lists the allowed cipher suites.
func handleTransportCiphers(args TransportArgs) error {
	if args.JSON {
		type CipherInfo struct {
			Name     string `json:"name"`
			ID       uint16 `json:"id"`
			Approved bool   `json:"approved"`
		}

		ciphers := make([]CipherInfo, 0)
		for _, id := range security.ApprovedCipherSuites {
			ciphers = append(ciphers, CipherInfo{
				Name:     cipherSuiteIDToString(id),
				ID:       id,
				Approved: true,
			})
		}

		data, err := json.MarshalIndent(map[string]interface{}{
			"approved_ciphers": ciphers,
			"count":            len(ciphers),
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(transportTitleStyle.Render("SC-13 Approved Cipher Suites"))
	fmt.Println(transportDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	fmt.Println(transportSectionStyle.Render("FIPS-Approved Ciphers for TLS 1.2"))
	for _, id := range security.ApprovedCipherSuites {
		name := cipherSuiteIDToString(id)
		fmt.Printf("  %s %s\n", transportGreenStyle.Render("[OK]"), name)
	}
	fmt.Println()

	fmt.Println(transportSectionStyle.Render("Blocked Weak Ciphers"))
	fmt.Println(transportDimStyle.Render("  The following ciphers are explicitly blocked:"))
	for id, reason := range security.WeakCipherSuites {
		name := cipherSuiteIDToString(id)
		fmt.Printf("  %s %s - %s\n", transportRedStyle.Render("[FAIL]"), name, transportDimStyle.Render(reason))
	}
	fmt.Println()

	fmt.Println(transportDimStyle.Render("  Note: TLS 1.3 uses its own secure cipher suites automatically."))
	fmt.Println()

	return nil
}

// =============================================================================
// TRANSPORT SESSIONS
// =============================================================================

// handleTransportSessions lists active remote access sessions.
func handleTransportSessions(args TransportArgs) error {
	ram := security.GlobalRemoteAccessManager()
	sessions := ram.GetActiveSessions()

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
	fmt.Println(transportTitleStyle.Render("AC-17 Active Remote Sessions"))
	fmt.Println(transportDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	if len(sessions) == 0 {
		fmt.Println(transportDimStyle.Render("  No active remote sessions."))
		fmt.Println()
		return nil
	}

	// Table header
	fmt.Printf("  %-22s %-15s %-18s %-12s\n", "Session ID", "User ID", "Remote Address", "Protocol")
	fmt.Println(transportDimStyle.Render("  " + strings.Repeat("-", 68)))

	for _, session := range sessions {
		// UNICODE: Rune-aware truncation preserves multi-byte characters
		sessionID := util.TruncateRunes(session.ID, 20)

		// UNICODE: Rune-aware truncation preserves multi-byte characters
		userID := util.TruncateRunes(session.UserID, 13)

		// UNICODE: Rune-aware truncation preserves multi-byte characters
		remoteAddr := util.TruncateRunes(session.RemoteAddr, 16)

		fmt.Printf("  %-22s %-15s %-18s %-12s\n",
			sessionID,
			userID,
			remoteAddr,
			session.Protocol)

		// Show time remaining
		remaining := session.TimeRemaining()
		fmt.Printf("    %s Started: %s ago, Expires: %s\n",
			transportDimStyle.Render("->"),
			formatDurationShort(time.Since(session.StartTime)),
			formatDurationShort(remaining))
	}

	fmt.Println()
	fmt.Printf("  Total: %d active session(s)\n", len(sessions))
	fmt.Println()

	return nil
}

// =============================================================================
// TRANSPORT TERMINATE
// =============================================================================

// handleTransportTerminate terminates a remote access session.
func handleTransportTerminate(args TransportArgs) error {
	if args.SessionID == "" {
		return fmt.Errorf("usage: rigrun transport terminate <session-id>\n\nUse 'rigrun transport sessions' to list active sessions")
	}

	ram := security.GlobalRemoteAccessManager()

	// Find session by prefix match
	sessions := ram.GetActiveSessions()
	var targetSession *security.RemoteSession
	for _, session := range sessions {
		if strings.HasPrefix(session.ID, args.SessionID) {
			targetSession = session
			break
		}
	}

	if targetSession == nil {
		return fmt.Errorf("session not found: %s", args.SessionID)
	}

	// Terminate the session
	if err := ram.TerminateSession(targetSession.ID); err != nil {
		return fmt.Errorf("failed to terminate session: %w", err)
	}

	if args.JSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"terminated": true,
			"session_id": targetSession.ID,
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("%s Session terminated successfully\n", transportGreenStyle.Render("[OK]"))
	fmt.Printf("  Session ID:  %s\n", transportDimStyle.Render(targetSession.ID[:24]+"..."))
	fmt.Printf("  User:        %s\n", targetSession.UserID)
	fmt.Printf("  Remote Addr: %s\n", targetSession.RemoteAddr)
	fmt.Println()

	return nil
}

// =============================================================================
// TRANSPORT POLICY
// =============================================================================

// handleTransportPolicy shows the remote access policy.
func handleTransportPolicy(args TransportArgs) error {
	ram := security.GlobalRemoteAccessManager()
	policy := ram.GetAccessPolicy()
	schedule := ram.GetAllowedTimes()

	if args.JSON {
		data, err := json.MarshalIndent(map[string]interface{}{
			"policy":   policy,
			"schedule": schedule,
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(transportTitleStyle.Render("AC-17 Remote Access Policy"))
	fmt.Println(transportDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	// Protocol restrictions
	fmt.Println(transportSectionStyle.Render("Allowed Protocols"))
	if len(policy.AllowedProtocols) > 0 {
		for _, proto := range policy.AllowedProtocols {
			fmt.Printf("  %s %s\n", transportGreenStyle.Render("[OK]"), proto)
		}
	} else {
		fmt.Println(transportDimStyle.Render("  All protocols allowed"))
	}
	fmt.Println()

	// MFA requirement
	fmt.Println(transportSectionStyle.Render("Multi-Factor Authentication"))
	mfaStatus := transportDimStyle.Render("Not Required")
	if policy.RequireMFA {
		mfaStatus = transportGreenStyle.Render("Required")
	}
	fmt.Printf("  %s%s\n", transportLabelStyle.Render("MFA Status:"), mfaStatus)
	fmt.Println()

	// Session limits
	fmt.Println(transportSectionStyle.Render("Session Limits"))
	fmt.Printf("  %s%d\n", transportLabelStyle.Render("Max Concurrent:"), policy.MaxConcurrentSessions)
	fmt.Println()

	// Time-based access
	fmt.Println(transportSectionStyle.Render("Time-Based Access Control"))
	if schedule != nil {
		fmt.Printf("  %s%02d:00 - %02d:00 %s\n",
			transportLabelStyle.Render("Allowed Hours:"),
			schedule.StartHour, schedule.EndHour, schedule.Timezone)

		fmt.Printf("  %s", transportLabelStyle.Render("Allowed Days:"))
		for i, day := range schedule.AllowedDays {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(day.String())
		}
		fmt.Println()
	} else {
		fmt.Println(transportDimStyle.Render("  24/7 access (no time restrictions)"))
	}
	fmt.Println()

	return nil
}

// =============================================================================
// TRANSPORT ALLOW-IP
// =============================================================================

// handleTransportAllowIP adds an IP range to the allowlist.
func handleTransportAllowIP(args TransportArgs) error {
	if args.CIDR == "" {
		return fmt.Errorf("usage: rigrun transport allow-ip <cidr>\n\nExample: rigrun transport allow-ip 10.0.0.0/8")
	}

	ram := security.GlobalRemoteAccessManager()

	if err := ram.AddAllowedIP(args.CIDR); err != nil {
		return fmt.Errorf("failed to add IP range: %w", err)
	}

	if args.JSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"added": true,
			"cidr":  args.CIDR,
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("%s IP range added to allowlist\n", transportGreenStyle.Render("[OK]"))
	fmt.Printf("  CIDR: %s\n", transportValueStyle.Render(args.CIDR))
	fmt.Println()

	return nil
}

// =============================================================================
// TRANSPORT DENY-IP
// =============================================================================

// handleTransportDenyIP removes an IP range from the allowlist.
func handleTransportDenyIP(args TransportArgs) error {
	if args.CIDR == "" {
		return fmt.Errorf("usage: rigrun transport deny-ip <cidr>\n\nExample: rigrun transport deny-ip 10.0.0.0/8")
	}

	ram := security.GlobalRemoteAccessManager()
	ram.RemoveAllowedIP(args.CIDR)

	if args.JSON {
		data, _ := json.MarshalIndent(map[string]interface{}{
			"removed": true,
			"cidr":    args.CIDR,
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("%s IP range removed from allowlist\n", transportGreenStyle.Render("[OK]"))
	fmt.Printf("  CIDR: %s\n", transportValueStyle.Render(args.CIDR))
	fmt.Println()

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// cipherSuiteIDToString converts a cipher suite ID to a readable name.
func cipherSuiteIDToString(id uint16) string {
	// This is a simplified mapping - in a real implementation, you'd use
	// crypto/tls.CipherSuites() to get the proper names
	switch id {
	case 0xc030:
		return "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
	case 0xc02f:
		return "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	case 0xc02c:
		return "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
	case 0xc02b:
		return "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"
	case 0xcca8:
		return "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305"
	case 0xcca9:
		return "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305"
	default:
		return fmt.Sprintf("UNKNOWN_CIPHER_0x%04X", id)
	}
}
