// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// crypto_cmd.go - Cryptographic control CLI commands for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements NIST 800-53 controls:
//   - SC-13 (Cryptographic Protection)
//   - IA-7 (Cryptographic Module Authentication)
//   - SC-17 (PKI Certificates)
//
// Command: crypto [subcommand]
// Short:   Cryptographic controls (IL5 SC-13, IA-7, SC-17)
// Aliases: (none)
//
// Subcommands:
//   status (default)    Show cryptographic status
//   fips                Check FIPS 140-2 compliance status
//   certs               Show certificate information
//   validate            Validate cryptographic configuration
//
// Examples:
//   rigrun crypto                     Show crypto status (default)
//   rigrun crypto status              Show cryptographic status
//   rigrun crypto status --json       Status in JSON format
//   rigrun crypto fips                Check FIPS compliance
//   rigrun crypto certs               Show PKI certificate info
//   rigrun crypto validate            Validate crypto configuration
//
// FIPS 140-2 Notes:
//   - Validates crypto module compliance
//   - Checks for approved algorithms
//   - Verifies key lengths meet requirements
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
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// STYLES
// =============================================================================

var (
	cryptoTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")). // Cyan
			MarginBottom(1)

	cryptoSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	cryptoLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Light gray
				Width(20)

	cryptoValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")) // White

	cryptoGreenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")) // Green

	cryptoYellowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")) // Yellow

	cryptoRedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // Red

	cryptoDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242")) // Dim

	cryptoSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// =============================================================================
// CRYPTO ARGUMENTS
// =============================================================================

// CryptoArgs holds parsed crypto command arguments.
type CryptoArgs struct {
	Subcommand string
	Host       string
	JSON       bool
	FIPS       bool
	Type       string
}

// parseCryptoArgs parses crypto command specific arguments.
func parseCryptoArgs(args *Args, remaining []string) CryptoArgs {
	cryptoArgs := CryptoArgs{}

	if len(remaining) > 0 {
		cryptoArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--json":
			cryptoArgs.JSON = true
		case "--fips":
			cryptoArgs.FIPS = true
		case "--type", "-t":
			if i+1 < len(remaining) {
				i++
				cryptoArgs.Type = remaining[i]
			}
		default:
			// Check for --type=value format
			if strings.HasPrefix(arg, "--type=") {
				cryptoArgs.Type = strings.TrimPrefix(arg, "--type=")
			} else if !strings.HasPrefix(arg, "-") && cryptoArgs.Host == "" {
				cryptoArgs.Host = arg
			}
		}
	}

	// Inherit JSON flag from global args
	if args.JSON {
		cryptoArgs.JSON = true
	}

	return cryptoArgs
}

// =============================================================================
// HANDLE CRYPTO
// =============================================================================

// HandleCrypto handles the "crypto" command with various subcommands.
// Subcommands:
//   - crypto status: Show crypto status and FIPS compliance
//   - crypto algorithms: List algorithms with FIPS status
//   - crypto verify --fips: Verify FIPS compliance
//   - crypto cert check <host>: Check certificate for host
//   - crypto cert pin <host>: Pin certificate for host
//   - crypto cert unpin <host>: Remove certificate pin for host
//   - crypto cert list: List pinned certificates
func HandleCrypto(args Args) error {
	cryptoArgs := parseCryptoArgs(&args, args.Raw)

	switch cryptoArgs.Subcommand {
	case "", "status":
		return handleCryptoStatus(cryptoArgs)
	case "algorithms", "algs":
		return handleCryptoAlgorithms(cryptoArgs)
	case "verify":
		return handleCryptoVerify(cryptoArgs)
	case "cert":
		return handleCryptoCert(cryptoArgs, args.Raw)
	default:
		return fmt.Errorf("unknown crypto subcommand: %s\n\nUsage:\n"+
			"  rigrun crypto status              Show crypto status\n"+
			"  rigrun crypto algorithms          List supported algorithms\n"+
			"  rigrun crypto verify --fips       Verify FIPS compliance\n"+
			"  rigrun crypto cert check <host>   Check certificate\n"+
			"  rigrun crypto cert pin <host>     Pin certificate\n"+
			"  rigrun crypto cert unpin <host>   Unpin certificate\n"+
			"  rigrun crypto cert list           List pinned certs", cryptoArgs.Subcommand)
	}
}

// =============================================================================
// CRYPTO STATUS
// =============================================================================

// handleCryptoStatus displays the cryptographic status of the system.
func handleCryptoStatus(cryptoArgs CryptoArgs) error {
	status := security.GetCryptoStatus()

	if cryptoArgs.JSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(status)
	}

	// Display status
	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(cryptoTitleStyle.Render("Cryptographic Status (SC-13/IA-7)"))
	fmt.Println(cryptoSeparatorStyle.Render(separator))
	fmt.Println()

	// FIPS Status
	fmt.Println(cryptoSectionStyle.Render("FIPS 140-2/3 Status"))

	fipsModeStr := "Disabled"
	fipsModeStyle := cryptoYellowStyle
	if status.FIPSMode {
		fipsModeStr = "Enabled"
		fipsModeStyle = cryptoGreenStyle
	}
	fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("FIPS Mode:"), fipsModeStyle.Render(fipsModeStr))

	fipsAvailStr := "Not Available"
	fipsAvailStyle := cryptoRedStyle
	if status.FIPSAvailable {
		fipsAvailStr = "Available"
		fipsAvailStyle = cryptoGreenStyle
	}
	fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("FIPS Crypto:"), fipsAvailStyle.Render(fipsAvailStr))

	fmt.Println()

	// TLS Status
	fmt.Println(cryptoSectionStyle.Render("TLS Configuration"))
	fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("TLS Version:"), cryptoValueStyle.Render(status.TLSVersion))
	fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Min Version:"), cryptoValueStyle.Render(status.TLSMinVersion))

	fmt.Println()

	// Platform Info
	fmt.Println(cryptoSectionStyle.Render("Platform"))
	fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("OS/Arch:"), cryptoValueStyle.Render(status.Platform))
	fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Go Version:"), cryptoValueStyle.Render(status.GoVersion))

	// Issues
	if len(status.Issues) > 0 {
		fmt.Println()
		fmt.Println(cryptoSectionStyle.Render("Issues"))
		for _, issue := range status.Issues {
			fmt.Printf("  %s %s\n", cryptoRedStyle.Render("[!]"), issue)
		}
	}

	fmt.Println()

	// Algorithm summary
	fmt.Println(cryptoSectionStyle.Render("Supported Algorithms"))
	fmt.Printf("  %s%d FIPS-approved algorithms\n",
		cryptoLabelStyle.Render("Total:"),
		len(status.Algorithms))
	fmt.Println()
	fmt.Println(cryptoDimStyle.Render("  Run 'rigrun crypto algorithms' for details"))
	fmt.Println()

	return nil
}

// =============================================================================
// CRYPTO ALGORITHMS
// =============================================================================

// handleCryptoAlgorithms lists supported cryptographic algorithms.
func handleCryptoAlgorithms(cryptoArgs CryptoArgs) error {
	algorithms := security.GetSupportedAlgorithms()

	// Filter by type if specified
	if cryptoArgs.Type != "" {
		var filtered []security.AlgorithmInfo
		for _, alg := range algorithms {
			if strings.EqualFold(string(alg.Type), cryptoArgs.Type) {
				filtered = append(filtered, alg)
			}
		}
		algorithms = filtered
	}

	if cryptoArgs.JSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]interface{}{
			"algorithms": algorithms,
			"count":      len(algorithms),
		})
	}

	// Group by type
	byType := make(map[security.AlgorithmType][]security.AlgorithmInfo)
	for _, alg := range algorithms {
		byType[alg.Type] = append(byType[alg.Type], alg)
	}

	// Sort types for consistent output
	var types []security.AlgorithmType
	for t := range byType {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool {
		return string(types[i]) < string(types[j])
	})

	// Display
	separator := strings.Repeat("=", 70)
	fmt.Println()
	fmt.Println(cryptoTitleStyle.Render("Cryptographic Algorithms"))
	fmt.Println(cryptoSeparatorStyle.Render(separator))

	for _, algType := range types {
		algs := byType[algType]
		fmt.Println()
		fmt.Println(cryptoSectionStyle.Render(formatAlgorithmType(algType)))

		for _, alg := range algs {
			fipsStatus := cryptoRedStyle.Render("[NOT FIPS]")
			if alg.FIPSApproved {
				fipsStatus = cryptoGreenStyle.Render("[FIPS]")
			}

			keyInfo := ""
			if alg.KeySize > 0 {
				keyInfo = cryptoDimStyle.Render(fmt.Sprintf(" (%d-bit)", alg.KeySize))
			}

			fmt.Printf("  %-20s %s%s\n", alg.Name, fipsStatus, keyInfo)

			if alg.Standard != "" {
				fmt.Printf("    %s\n", cryptoDimStyle.Render(alg.Standard))
			}
		}
	}

	fmt.Println()
	fmt.Printf("Total: %d algorithms (%d FIPS-approved)\n",
		len(algorithms),
		countFIPSApproved(algorithms))
	fmt.Println()

	return nil
}

func formatAlgorithmType(t security.AlgorithmType) string {
	switch t {
	case security.AlgorithmTypeSymmetric:
		return "Symmetric Encryption"
	case security.AlgorithmTypeHash:
		return "Hash Functions"
	case security.AlgorithmTypeSignature:
		return "Digital Signatures"
	case security.AlgorithmTypeKDF:
		return "Key Derivation"
	case security.AlgorithmTypeKeyExch:
		return "Key Exchange"
	case security.AlgorithmTypeMAC:
		return "Message Authentication"
	default:
		return string(t)
	}
}

func countFIPSApproved(algs []security.AlgorithmInfo) int {
	count := 0
	for _, alg := range algs {
		if alg.FIPSApproved {
			count++
		}
	}
	return count
}

// =============================================================================
// CRYPTO VERIFY
// =============================================================================

// handleCryptoVerify verifies FIPS compliance.
func handleCryptoVerify(cryptoArgs CryptoArgs) error {
	result := security.VerifyFIPSCompliance()

	// Log the check
	security.LogCryptoFIPSCheck("CLI", result.Compliant, result.Issues)

	if cryptoArgs.JSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}

	// Display result
	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(cryptoTitleStyle.Render("FIPS Compliance Verification"))
	fmt.Println(cryptoSeparatorStyle.Render(separator))
	fmt.Println()

	// Overall status
	if result.Compliant {
		fmt.Printf("  %s FIPS Compliance: %s\n",
			cryptoGreenStyle.Render("[PASS]"),
			cryptoGreenStyle.Render("Compliant"))
	} else {
		fmt.Printf("  %s FIPS Compliance: %s\n",
			cryptoRedStyle.Render("[FAIL]"),
			cryptoRedStyle.Render("Non-Compliant"))
	}
	fmt.Println()

	// Issues
	if len(result.Issues) > 0 {
		fmt.Println(cryptoSectionStyle.Render("Issues"))
		for _, issue := range result.Issues {
			fmt.Printf("  %s %s\n", cryptoRedStyle.Render("[ERROR]"), issue)
		}
		fmt.Println()
	}

	// Warnings
	if len(result.Warnings) > 0 {
		fmt.Println(cryptoSectionStyle.Render("Warnings"))
		for _, warning := range result.Warnings {
			fmt.Printf("  %s %s\n", cryptoYellowStyle.Render("[WARN]"), warning)
		}
		fmt.Println()
	}

	// Recommendations
	if !result.Compliant || len(result.Warnings) > 0 {
		fmt.Println(cryptoSectionStyle.Render("Recommendations"))
		fmt.Println("  1. Enable FIPS mode: rigrun config set security.fips_mode true")
		fmt.Println("  2. Use FIPS-validated Go build (GOEXPERIMENT=boringcrypto)")
		fmt.Println("  3. Ensure TLS 1.2+ for all connections")
		fmt.Println("  4. Enable certificate pinning for sensitive endpoints")
		fmt.Println()
	}

	return nil
}

// =============================================================================
// CRYPTO CERT
// =============================================================================

// handleCryptoCert handles certificate-related subcommands.
func handleCryptoCert(cryptoArgs CryptoArgs, rawArgs []string) error {
	// Parse cert subcommand
	certSubcmd := ""
	host := ""

	// Look for cert subcommand in raw args
	inCert := false
	for i, arg := range rawArgs {
		if arg == "cert" {
			inCert = true
			continue
		}
		if inCert && !strings.HasPrefix(arg, "-") {
			if certSubcmd == "" {
				certSubcmd = arg
			} else if host == "" {
				host = arg
			}
		}
		// Handle --json appearing anywhere
		if arg == "--json" {
			cryptoArgs.JSON = true
		}
		_ = i
	}

	if host == "" {
		host = cryptoArgs.Host
	}

	switch certSubcmd {
	case "check":
		if host == "" {
			return fmt.Errorf("host required for cert check\n\nUsage: rigrun crypto cert check <host>")
		}
		return handleCertCheck(host, cryptoArgs.JSON)
	case "pin":
		if host == "" {
			return fmt.Errorf("host required for cert pin\n\nUsage: rigrun crypto cert pin <host>")
		}
		return handleCertPin(host, cryptoArgs.JSON)
	case "unpin":
		if host == "" {
			return fmt.Errorf("host required for cert unpin\n\nUsage: rigrun crypto cert unpin <host>")
		}
		return handleCertUnpin(host, cryptoArgs.JSON)
	case "list":
		return handleCertList(cryptoArgs.JSON)
	default:
		return fmt.Errorf("unknown cert subcommand: %s\n\nUsage:\n"+
			"  rigrun crypto cert check <host>   Check certificate\n"+
			"  rigrun crypto cert pin <host>     Pin certificate\n"+
			"  rigrun crypto cert unpin <host>   Unpin certificate\n"+
			"  rigrun crypto cert list           List pinned certs", certSubcmd)
	}
}

// handleCertCheck checks the certificate for a host.
func handleCertCheck(host string, jsonOutput bool) error {
	pm := security.GlobalPKIManager()

	status, err := pm.ValidateCertificate(host)

	// Log the validation
	valid := err == nil
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	security.LogCertValidation("CLI", host, valid, reason)

	if jsonOutput {
		result := map[string]interface{}{
			"host":   host,
			"valid":  err == nil,
			"status": status,
		}
		if err != nil {
			result["error"] = err.Error()
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}

	// Display result
	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(cryptoTitleStyle.Render("Certificate Check (SC-17)"))
	fmt.Println(cryptoSeparatorStyle.Render(separator))
	fmt.Println()

	fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Host:"), cryptoValueStyle.Render(host))

	if err != nil {
		fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Status:"), cryptoRedStyle.Render("INVALID"))
		fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Error:"), cryptoRedStyle.Render(err.Error()))
	} else {
		fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Status:"), cryptoGreenStyle.Render("VALID"))
	}

	if status != nil {
		fmt.Println()
		fmt.Println(cryptoSectionStyle.Render("Certificate Details"))
		fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Subject:"), cryptoValueStyle.Render(status.Subject))
		fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Issuer:"), cryptoValueStyle.Render(status.Issuer))
		fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Valid From:"), cryptoValueStyle.Render(status.ValidFrom.Format("2006-01-02 15:04:05")))
		fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Valid Until:"), cryptoValueStyle.Render(status.ValidUntil.Format("2006-01-02 15:04:05")))

		// Days until expiry with color coding
		expiryStyle := cryptoGreenStyle
		if status.DaysUntilExpiry < 30 {
			expiryStyle = cryptoYellowStyle
		}
		if status.DaysUntilExpiry < 7 {
			expiryStyle = cryptoRedStyle
		}
		fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Days to Expiry:"), expiryStyle.Render(fmt.Sprintf("%d", status.DaysUntilExpiry)))

		chainStatus := cryptoGreenStyle.Render("Valid")
		if !status.ChainValid {
			chainStatus = cryptoRedStyle.Render("Invalid")
		}
		fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Chain:"), chainStatus)

		pinnedStatus := cryptoDimStyle.Render("No")
		if status.Pinned {
			pinnedStatus = cryptoGreenStyle.Render("Yes")
		}
		fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Pinned:"), pinnedStatus)

		if status.Fingerprint != "" {
			fmt.Println()
			fmt.Println(cryptoSectionStyle.Render("Fingerprint (SHA-256)"))
			fmt.Printf("  %s\n", cryptoDimStyle.Render(security.FormatFingerprint(status.Fingerprint)))
		}
	}

	fmt.Println()

	return nil
}

// handleCertPin pins the certificate for a host.
func handleCertPin(host string, jsonOutput bool) error {
	pm := security.GlobalPKIManager()

	// First, validate and get the certificate
	status, err := pm.ValidateCertificate(host)
	if err != nil {
		if jsonOutput {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(map[string]interface{}{
				"success": false,
				"host":    host,
				"error":   err.Error(),
			})
		}
		return fmt.Errorf("failed to get certificate for %s: %w", host, err)
	}

	// Pin the certificate
	if err := pm.PinCertificate(host, status.Fingerprint); err != nil {
		if jsonOutput {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(map[string]interface{}{
				"success": false,
				"host":    host,
				"error":   err.Error(),
			})
		}
		return fmt.Errorf("failed to pin certificate: %w", err)
	}

	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]interface{}{
			"success":     true,
			"host":        host,
			"fingerprint": status.Fingerprint,
			"subject":     status.Subject,
		})
	}

	fmt.Println()
	fmt.Printf("%s Certificate pinned for %s\n",
		cryptoGreenStyle.Render("[OK]"),
		cryptoValueStyle.Render(host))
	fmt.Println()
	fmt.Println(cryptoSectionStyle.Render("Pinned Certificate"))
	fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Subject:"), status.Subject)
	fmt.Printf("  %s%s\n", cryptoLabelStyle.Render("Fingerprint:"),
		cryptoDimStyle.Render(security.FormatFingerprint(status.Fingerprint)))
	fmt.Println()
	fmt.Println(cryptoDimStyle.Render("Note: Add to config for persistence:"))
	fmt.Printf("  [security.pinned_certificates]\n")
	fmt.Printf("  \"%s\" = \"%s\"\n", host, status.Fingerprint)
	fmt.Println()

	return nil
}

// handleCertUnpin removes the certificate pin for a host.
func handleCertUnpin(host string, jsonOutput bool) error {
	pm := security.GlobalPKIManager()

	wasPinned := pm.IsPinned(host)
	pm.UnpinCertificate(host)

	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]interface{}{
			"success":    true,
			"host":       host,
			"was_pinned": wasPinned,
		})
	}

	if wasPinned {
		fmt.Println()
		fmt.Printf("%s Certificate pin removed for %s\n",
			cryptoGreenStyle.Render("[OK]"),
			cryptoValueStyle.Render(host))
	} else {
		fmt.Println()
		fmt.Printf("%s No certificate pin found for %s\n",
			cryptoYellowStyle.Render("[INFO]"),
			cryptoValueStyle.Render(host))
	}
	fmt.Println()

	return nil
}

// handleCertList lists all pinned certificates.
func handleCertList(jsonOutput bool) error {
	pm := security.GlobalPKIManager()
	pinnedHosts := pm.GetPinnedHosts()

	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]interface{}{
			"pinned_certificates": pinnedHosts,
			"count":               len(pinnedHosts),
		})
	}

	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(cryptoTitleStyle.Render("Pinned Certificates"))
	fmt.Println(cryptoSeparatorStyle.Render(separator))
	fmt.Println()

	if len(pinnedHosts) == 0 {
		fmt.Println(cryptoDimStyle.Render("  No certificates are currently pinned."))
		fmt.Println()
		fmt.Println(cryptoDimStyle.Render("  Pin a certificate with: rigrun crypto cert pin <host>"))
	} else {
		// Sort hosts for consistent output
		var hosts []string
		for host := range pinnedHosts {
			hosts = append(hosts, host)
		}
		sort.Strings(hosts)

		for _, host := range hosts {
			fingerprint := pinnedHosts[host]
			fmt.Printf("  %s\n", cryptoValueStyle.Render(host))
			fmt.Printf("    %s\n", cryptoDimStyle.Render(security.FormatFingerprint(fingerprint)))
		}
		fmt.Println()
		fmt.Printf("  Total: %d pinned certificate(s)\n", len(pinnedHosts))
	}

	fmt.Println()

	return nil
}
