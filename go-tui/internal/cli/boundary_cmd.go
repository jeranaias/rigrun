// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// boundary_cmd.go - Network boundary protection CLI commands for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements NIST 800-53 SC-7: Boundary Protection
// for DoD IL5 compliance.
//
// Command: boundary [subcommand]
// Short:   Network boundary protection (IL5 SC-7)
// Aliases: net
//
// Subcommands:
//   status (default)    Show boundary protection status
//   allow <host>        Add host to allowlist
//   deny <host>         Add host to denylist
//   list                List all rules
//   remove <id>         Remove a rule
//   test <host>         Test if host is allowed
//
// Examples:
//   rigrun boundary                       Show status (default)
//   rigrun boundary status                Show protection status
//   rigrun boundary status --json         Status in JSON format
//   rigrun boundary allow api.openai.com  Allow specific host
//   rigrun boundary deny example.com      Block specific host
//   rigrun boundary list                  Show all rules
//   rigrun boundary remove 3              Remove rule by ID
//   rigrun boundary test api.example.com  Test host access
//
// SC-7 Boundary Features:
//   - Allowlist/denylist based access control
//   - Paranoid mode: block all external connections
//   - Offline mode: localhost only (air-gapped)
//   - All boundary decisions logged to audit
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
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// STYLES
// =============================================================================

var (
	boundaryTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	boundarySectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	boundaryLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Light gray
				Width(20)

	boundaryValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")) // White

	boundaryGreenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")) // Green

	boundaryYellowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")) // Yellow

	boundaryRedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")) // Red

	boundaryDimStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("242")) // Dim

	boundarySeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// =============================================================================
// BOUNDARY ARGUMENTS
// =============================================================================

// BoundaryArgs holds parsed boundary command arguments.
type BoundaryArgs struct {
	Subcommand string
	Host       string
	Reason     string
	Limit      int
	JSON       bool
	Enabled    bool
	Disabled   bool
}

// parseBoundaryArgs parses boundary command specific arguments.
func parseBoundaryArgs(args *Args, remaining []string) BoundaryArgs {
	boundaryArgs := BoundaryArgs{
		Limit: 50, // Default
	}

	if len(remaining) > 0 {
		boundaryArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--json":
			boundaryArgs.JSON = true
		case "--limit", "-n":
			if i+1 < len(remaining) {
				i++
				if n, err := strconv.Atoi(remaining[i]); err == nil && n > 0 {
					boundaryArgs.Limit = n
				}
			}
		case "on", "enable", "enabled":
			boundaryArgs.Enabled = true
		case "off", "disable", "disabled":
			boundaryArgs.Disabled = true
		default:
			// Check for --limit=N format
			if strings.HasPrefix(arg, "--limit=") {
				if n, err := strconv.Atoi(strings.TrimPrefix(arg, "--limit=")); err == nil && n > 0 {
					boundaryArgs.Limit = n
				}
			} else if !strings.HasPrefix(arg, "-") {
				// Positional arguments
				if boundaryArgs.Host == "" {
					boundaryArgs.Host = arg
				} else if boundaryArgs.Reason == "" {
					// Collect remaining as reason
					boundaryArgs.Reason = strings.Join(remaining[i:], " ")
					break
				}
			}
		}
	}

	// Inherit JSON flag
	if args.JSON {
		boundaryArgs.JSON = true
	}

	return boundaryArgs
}

// =============================================================================
// HANDLE BOUNDARY
// =============================================================================

// HandleBoundary handles the "boundary" command with various subcommands.
// Subcommands:
//   - boundary status: Show boundary protection status
//   - boundary policy: Show current network policy
//   - boundary allow <host>: Add host to allowlist
//   - boundary block <host> [reason]: Block a host
//   - boundary unblock <host>: Remove from blocklist
//   - boundary list-allowed: List allowed hosts
//   - boundary list-blocked: List blocked hosts
//   - boundary connections: Show connection log
//   - boundary enforce <on|off>: Enable/disable egress filtering
func HandleBoundary(args Args) error {
	boundaryArgs := parseBoundaryArgs(&args, args.Raw)

	switch boundaryArgs.Subcommand {
	case "", "status":
		return handleBoundaryStatus(boundaryArgs)
	case "policy":
		return handleBoundaryPolicy(boundaryArgs)
	case "allow":
		return handleBoundaryAllow(boundaryArgs)
	case "block":
		return handleBoundaryBlock(boundaryArgs)
	case "unblock":
		return handleBoundaryUnblock(boundaryArgs)
	case "list-allowed", "allowed":
		return handleBoundaryListAllowed(boundaryArgs)
	case "list-blocked", "blocked":
		return handleBoundaryListBlocked(boundaryArgs)
	case "connections", "log":
		return handleBoundaryConnections(boundaryArgs)
	case "enforce":
		return handleBoundaryEnforce(boundaryArgs)
	default:
		return fmt.Errorf("unknown boundary subcommand: %s\n\nUsage:\n"+
			"  rigrun boundary status              Show boundary protection status\n"+
			"  rigrun boundary policy              Show current network policy\n"+
			"  rigrun boundary allow <host>        Add host to allowlist\n"+
			"  rigrun boundary block <host> [reason] Block a host\n"+
			"  rigrun boundary unblock <host>      Remove from blocklist\n"+
			"  rigrun boundary list-allowed        List allowed hosts\n"+
			"  rigrun boundary list-blocked        List blocked hosts\n"+
			"  rigrun boundary connections         Show connection log\n"+
			"  rigrun boundary enforce <on|off>    Enable/disable egress filtering", boundaryArgs.Subcommand)
	}
}

// =============================================================================
// BOUNDARY STATUS
// =============================================================================

// handleBoundaryStatus shows the boundary protection status.
func handleBoundaryStatus(boundaryArgs BoundaryArgs) error {
	bp := security.GlobalBoundaryProtection()
	stats := bp.GetStats()
	policy := bp.GetNetworkPolicy()

	if boundaryArgs.JSON {
		output := map[string]interface{}{
			"stats":  stats,
			"policy": policy,
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	}

	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(boundaryTitleStyle.Render("SC-7 Boundary Protection Status"))
	fmt.Println(boundarySeparatorStyle.Render(separator))
	fmt.Println()

	// Egress status
	egressStatus := boundaryRedStyle.Render("Disabled")
	if stats.EgressEnabled {
		egressStatus = boundaryGreenStyle.Render("Enabled")
	}
	fmt.Printf("  %s%s\n", boundaryLabelStyle.Render("Egress Filtering:"), egressStatus)
	fmt.Println()

	// Policy mode
	policyMode := "Default Deny (IL5)"
	policyStyle := boundaryGreenStyle
	if policy.DefaultAllow {
		policyMode = "Default Allow (Non-Compliant)"
		policyStyle = boundaryYellowStyle
	}
	fmt.Printf("  %s%s\n", boundaryLabelStyle.Render("Policy Mode:"), policyStyle.Render(policyMode))
	fmt.Println()

	// Statistics
	fmt.Println(boundarySectionStyle.Render("Statistics"))
	fmt.Printf("  %s%s\n", boundaryLabelStyle.Render("Allowed Hosts:"), boundaryValueStyle.Render(fmt.Sprintf("%d", stats.AllowedHostsCount)))
	fmt.Printf("  %s%s\n", boundaryLabelStyle.Render("Blocked Hosts:"), boundaryValueStyle.Render(fmt.Sprintf("%d", stats.BlockedHostsCount)))
	fmt.Printf("  %s%s\n", boundaryLabelStyle.Render("Allowed Ports:"), boundaryValueStyle.Render(fmt.Sprintf("%d", stats.AllowedPortsCount)))
	fmt.Printf("  %s%s\n", boundaryLabelStyle.Render("Connections Logged:"), boundaryValueStyle.Render(fmt.Sprintf("%d", stats.ConnectionsLogged)))
	fmt.Printf("  %s%s\n", boundaryLabelStyle.Render("Connections Blocked:"), boundaryValueStyle.Render(fmt.Sprintf("%d", stats.ConnectionsBlocked)))
	fmt.Println()

	// Last update
	if !stats.LastPolicyUpdate.IsZero() {
		fmt.Println(boundarySectionStyle.Render("Policy"))
		fmt.Printf("  %s%s\n", boundaryLabelStyle.Render("Last Updated:"), boundaryValueStyle.Render(stats.LastPolicyUpdate.Format("2006-01-02 15:04:05")))
		fmt.Println()
	}

	// Quick reference
	fmt.Println(boundarySectionStyle.Render("Commands"))
	fmt.Println(boundaryDimStyle.Render("  rigrun boundary policy             View full policy"))
	fmt.Println(boundaryDimStyle.Render("  rigrun boundary allow <host>       Add allowed host"))
	fmt.Println(boundaryDimStyle.Render("  rigrun boundary connections        View connection log"))
	fmt.Println()

	return nil
}

// =============================================================================
// BOUNDARY POLICY
// =============================================================================

// handleBoundaryPolicy shows the current network policy.
func handleBoundaryPolicy(boundaryArgs BoundaryArgs) error {
	bp := security.GlobalBoundaryProtection()
	policy := bp.GetNetworkPolicy()

	if boundaryArgs.JSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(policy)
	}

	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(boundaryTitleStyle.Render("Network Policy (SC-7)"))
	fmt.Println(boundarySeparatorStyle.Render(separator))
	fmt.Println()

	// Policy mode
	fmt.Println(boundarySectionStyle.Render("Policy Mode"))
	if policy.DefaultAllow {
		fmt.Printf("  %s\n", boundaryYellowStyle.Render("Default Allow (WARNING: Non-compliant with IL5)"))
	} else {
		fmt.Printf("  %s\n", boundaryGreenStyle.Render("Default Deny (IL5 Compliant)"))
	}
	fmt.Println()

	// Allowed hosts
	fmt.Println(boundarySectionStyle.Render("Allowed Hosts"))
	if len(policy.AllowedHosts) == 0 {
		fmt.Println(boundaryDimStyle.Render("  No allowed hosts configured"))
	} else {
		for _, host := range policy.AllowedHosts {
			fmt.Printf("  %s %s\n", boundaryGreenStyle.Render("[OK]"), host)
		}
	}
	fmt.Println()

	// Blocked hosts
	fmt.Println(boundarySectionStyle.Render("Blocked Hosts"))
	if len(policy.BlockedHosts) == 0 {
		fmt.Println(boundaryDimStyle.Render("  No blocked hosts configured"))
	} else {
		for _, host := range policy.BlockedHosts {
			fmt.Printf("  %s %s\n", boundaryRedStyle.Render("[FAIL]"), host)
		}
	}
	fmt.Println()

	// Allowed ports
	fmt.Println(boundarySectionStyle.Render("Allowed Ports"))
	if len(policy.AllowedPorts) == 0 {
		fmt.Println(boundaryDimStyle.Render("  All ports allowed"))
	} else {
		portStrs := make([]string, len(policy.AllowedPorts))
		for i, port := range policy.AllowedPorts {
			portName := fmt.Sprintf("%d", port)
			switch port {
			case 443:
				portName = "443 (HTTPS)"
			case 80:
				portName = "80 (HTTP)"
			case 11434:
				portName = "11434 (Ollama)"
			}
			portStrs[i] = portName
		}
		fmt.Printf("  %s\n", strings.Join(portStrs, ", "))
	}
	fmt.Println()

	// Proxy configuration
	if policy.ProxyURL != "" {
		fmt.Println(boundarySectionStyle.Render("Proxy Configuration"))
		fmt.Printf("  %s%s\n", boundaryLabelStyle.Render("Proxy URL:"), boundaryValueStyle.Render(policy.ProxyURL))
		if len(policy.ProxyBypass) > 0 {
			fmt.Printf("  %s%s\n", boundaryLabelStyle.Render("Bypass:"), boundaryValueStyle.Render(strings.Join(policy.ProxyBypass, ", ")))
		}
		fmt.Println()
	}

	return nil
}

// =============================================================================
// BOUNDARY ALLOW
// =============================================================================

// handleBoundaryAllow adds a host to the allowlist.
func handleBoundaryAllow(boundaryArgs BoundaryArgs) error {
	if boundaryArgs.Host == "" {
		return fmt.Errorf("host required\n\nUsage: rigrun boundary allow <host>")
	}

	bp := security.GlobalBoundaryProtection()
	bp.AllowHost(boundaryArgs.Host)

	// Save policy
	if err := bp.SavePolicy(); err != nil {
		return fmt.Errorf("failed to save policy: %w", err)
	}

	if boundaryArgs.JSON {
		return outputJSON(map[string]interface{}{
			"success": true,
			"host":    boundaryArgs.Host,
			"action":  "allowed",
		})
	}

	fmt.Println()
	fmt.Printf("%s Host added to allowlist: %s\n",
		boundaryGreenStyle.Render("[OK]"),
		boundaryValueStyle.Render(boundaryArgs.Host))
	fmt.Println()

	return nil
}

// =============================================================================
// BOUNDARY BLOCK
// =============================================================================

// handleBoundaryBlock blocks a host.
func handleBoundaryBlock(boundaryArgs BoundaryArgs) error {
	if boundaryArgs.Host == "" {
		return fmt.Errorf("host required\n\nUsage: rigrun boundary block <host> [reason]")
	}

	reason := boundaryArgs.Reason
	if reason == "" {
		reason = "manually blocked"
	}

	bp := security.GlobalBoundaryProtection()
	bp.BlockHost(boundaryArgs.Host, reason)

	// Save policy
	if err := bp.SavePolicy(); err != nil {
		return fmt.Errorf("failed to save policy: %w", err)
	}

	if boundaryArgs.JSON {
		return outputJSON(map[string]interface{}{
			"success": true,
			"host":    boundaryArgs.Host,
			"reason":  reason,
			"action":  "blocked",
		})
	}

	fmt.Println()
	fmt.Printf("%s Host blocked: %s\n",
		boundaryRedStyle.Render("[BLOCKED]"),
		boundaryValueStyle.Render(boundaryArgs.Host))
	fmt.Printf("  %s%s\n", boundaryLabelStyle.Render("Reason:"), reason)
	fmt.Println()

	return nil
}

// =============================================================================
// BOUNDARY UNBLOCK
// =============================================================================

// handleBoundaryUnblock removes a host from the blocklist.
func handleBoundaryUnblock(boundaryArgs BoundaryArgs) error {
	if boundaryArgs.Host == "" {
		return fmt.Errorf("host required\n\nUsage: rigrun boundary unblock <host>")
	}

	bp := security.GlobalBoundaryProtection()
	removed := bp.UnblockHost(boundaryArgs.Host)

	// Save policy
	if err := bp.SavePolicy(); err != nil {
		return fmt.Errorf("failed to save policy: %w", err)
	}

	if boundaryArgs.JSON {
		return outputJSON(map[string]interface{}{
			"success": removed,
			"host":    boundaryArgs.Host,
			"action":  "unblocked",
		})
	}

	fmt.Println()
	if removed {
		fmt.Printf("%s Host unblocked: %s\n",
			boundaryGreenStyle.Render("[OK]"),
			boundaryValueStyle.Render(boundaryArgs.Host))
	} else {
		fmt.Printf("%s Host was not in blocklist: %s\n",
			boundaryYellowStyle.Render("[INFO]"),
			boundaryValueStyle.Render(boundaryArgs.Host))
	}
	fmt.Println()

	return nil
}

// =============================================================================
// BOUNDARY LIST ALLOWED
// =============================================================================

// handleBoundaryListAllowed lists all allowed hosts.
func handleBoundaryListAllowed(boundaryArgs BoundaryArgs) error {
	bp := security.GlobalBoundaryProtection()
	hosts := bp.GetAllowedHosts()

	if boundaryArgs.JSON {
		return outputJSON(map[string]interface{}{
			"allowed_hosts": hosts,
			"count":         len(hosts),
		})
	}

	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(boundaryTitleStyle.Render("Allowed Hosts"))
	fmt.Println(boundarySeparatorStyle.Render(separator))
	fmt.Println()

	if len(hosts) == 0 {
		fmt.Println(boundaryDimStyle.Render("  No allowed hosts configured"))
		fmt.Println()
		fmt.Println(boundaryDimStyle.Render("  Add hosts with: rigrun boundary allow <host>"))
	} else {
		// Sort for consistent output
		sort.Strings(hosts)

		for _, host := range hosts {
			fmt.Printf("  %s %s\n", boundaryGreenStyle.Render("[OK]"), host)
		}
		fmt.Println()
		fmt.Printf("  Total: %d host(s)\n", len(hosts))
	}

	fmt.Println()

	return nil
}

// =============================================================================
// BOUNDARY LIST BLOCKED
// =============================================================================

// handleBoundaryListBlocked lists all blocked hosts.
func handleBoundaryListBlocked(boundaryArgs BoundaryArgs) error {
	bp := security.GlobalBoundaryProtection()
	entries := bp.GetBlockedHosts()

	if boundaryArgs.JSON {
		return outputJSON(map[string]interface{}{
			"blocked_hosts": entries,
			"count":         len(entries),
		})
	}

	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(boundaryTitleStyle.Render("Blocked Hosts"))
	fmt.Println(boundarySeparatorStyle.Render(separator))
	fmt.Println()

	if len(entries) == 0 {
		fmt.Println(boundaryDimStyle.Render("  No blocked hosts"))
		fmt.Println()
		fmt.Println(boundaryDimStyle.Render("  Block hosts with: rigrun boundary block <host> [reason]"))
	} else {
		// Sort by host
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Host < entries[j].Host
		})

		for _, entry := range entries {
			fmt.Printf("  %s %s\n", boundaryRedStyle.Render("[FAIL]"), boundaryValueStyle.Render(entry.Host))
			fmt.Printf("    %s%s\n", boundaryLabelStyle.Render("Reason:"), entry.Reason)
			fmt.Printf("    %s%s\n", boundaryLabelStyle.Render("Blocked:"), entry.BlockedAt.Format("2006-01-02 15:04:05"))
			fmt.Println()
		}
		fmt.Printf("  Total: %d host(s)\n", len(entries))
	}

	fmt.Println()

	return nil
}

// =============================================================================
// BOUNDARY CONNECTIONS
// =============================================================================

// handleBoundaryConnections shows the connection log.
func handleBoundaryConnections(boundaryArgs BoundaryArgs) error {
	bp := security.GlobalBoundaryProtection()
	entries := bp.GetConnectionLog(boundaryArgs.Limit)

	if boundaryArgs.JSON {
		return outputJSON(map[string]interface{}{
			"connections": entries,
			"count":       len(entries),
		})
	}

	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(boundaryTitleStyle.Render("Connection Log (SC-7)"))
	fmt.Println(boundarySeparatorStyle.Render(separator))
	fmt.Println()

	if len(entries) == 0 {
		fmt.Println(boundaryDimStyle.Render("  No connections logged yet"))
	} else {
		fmt.Printf("Showing last %d connection(s):\n\n", len(entries))

		for _, entry := range entries {
			timestamp := entry.Timestamp.Format("15:04:05")

			actionStyle := boundaryGreenStyle
			actionText := "ALLOW"
			if entry.Action == "block" {
				actionStyle = boundaryRedStyle
				actionText = "BLOCK"
			}

			fmt.Printf("  %s  %s  %s:%d",
				boundaryDimStyle.Render(timestamp),
				actionStyle.Render(fmt.Sprintf("%-5s", actionText)),
				entry.Destination,
				entry.Port)

			if entry.Reason != "" {
				fmt.Printf("  %s", boundaryDimStyle.Render(fmt.Sprintf("(%s)", entry.Reason)))
			}

			fmt.Println()
		}

		// Count blocked vs allowed
		blocked := 0
		allowed := 0
		for _, entry := range entries {
			if entry.Action == "block" {
				blocked++
			} else {
				allowed++
			}
		}

		fmt.Println()
		fmt.Printf("  Allowed: %s  Blocked: %s\n",
			boundaryGreenStyle.Render(fmt.Sprintf("%d", allowed)),
			boundaryRedStyle.Render(fmt.Sprintf("%d", blocked)))
	}

	fmt.Println()

	return nil
}

// =============================================================================
// BOUNDARY ENFORCE
// =============================================================================

// handleBoundaryEnforce enables or disables egress filtering.
func handleBoundaryEnforce(boundaryArgs BoundaryArgs) error {
	// Determine desired state
	var enabled bool
	if boundaryArgs.Enabled {
		enabled = true
	} else if boundaryArgs.Disabled {
		enabled = false
	} else {
		return fmt.Errorf("specify 'on' or 'off'\n\nUsage: rigrun boundary enforce <on|off>")
	}

	bp := security.GlobalBoundaryProtection()
	bp.EnforceEgress(enabled)

	if boundaryArgs.JSON {
		return outputJSON(map[string]interface{}{
			"success": true,
			"enabled": enabled,
		})
	}

	fmt.Println()
	if enabled {
		fmt.Printf("%s Egress filtering enabled (SC-7 compliant)\n",
			boundaryGreenStyle.Render("[OK]"))
	} else {
		fmt.Printf("%s Egress filtering disabled\n",
			boundaryYellowStyle.Render("[WARNING]"))
		fmt.Println()
		fmt.Println(boundaryRedStyle.Render("  WARNING: Disabling egress filtering may not be compliant with IL5"))
	}
	fmt.Println()

	return nil
}

// =============================================================================
// HELPERS
// =============================================================================
