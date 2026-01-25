// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// cli.go - CLI parsing and command handlers for rigrun.
//
// CLI: Comprehensive help and examples for all commands
package cli

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// Version information (can be overridden at build time)
var (
	Version   = "0.1.0"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// Command represents the CLI command to execute.
type Command int

const (
	CmdTUI Command = iota
	CmdAsk
	CmdChat
	CmdStatus
	CmdConfig
	CmdSetup
	CmdCache
	CmdSession
	CmdAudit    // IL5 compliance audit command (AU-5, AU-6, AU-9, AU-11)
	CmdVerify   // NIST 800-53 SI-7: Software/Firmware/Information Integrity
	CmdDoctor
	CmdClassify
	CmdConsent  // IL5 compliance consent command (AC-8)
	CmdTest     // IL5 compliance built-in self-test (BIST)
	CmdEncrypt  // NIST 800-53 SC-28: Protection of Information at Rest
	CmdCrypto   // NIST 800-53 SC-13, IA-7, SC-17: Cryptographic Protection & PKI
	CmdIncident // NIST 800-53 IR-6: Incident Reporting
	CmdData     // NIST 800-53 IR-9: Data Sanitization and Spillage Response
	CmdLockout    // NIST 800-53 AC-7: Unsuccessful Logon Attempts
	CmdAuth       // NIST 800-53 IA-2: Identification and Authentication
	CmdAgreements  // NIST 800-53 PS-6: Access Agreements
	CmdConmon      // NIST 800-53 CA-7: Continuous Monitoring
	CmdRules       // NIST 800-53 PL-4: Rules of Behavior
	CmdRBAC       // NIST 800-53 AC-5 (Separation of Duties) & AC-6 (Least Privilege)
	CmdBackup     // NIST 800-53 CP-9 (System Backup) & CP-10 (System Recovery)
	CmdMaintenance // NIST 800-53 MA-4 (Nonlocal Maintenance) & MA-5 (Maintenance Personnel)
	CmdVersion
	CmdBoundary   // NIST 800-53 SC-7: Boundary Protection
	CmdConfigMgmt // NIST 800-53 CM-5 & CM-6: Configuration Management
	CmdVuln       // NIST 800-53 RA-5: Vulnerability Monitoring and Scanning
	CmdTraining   // NIST 800-53 AT-2: Security Awareness Training
	CmdTransport  // NIST 800-53 SC-8: Transmission Confidentiality and Integrity
	CmdSecTest    // NIST 800-53 SA-11: Developer Security Testing
	CmdIntel      // Competitive Intelligence Research
	CmdHelp
)

// Args holds parsed CLI arguments.
type Args struct {
	// Global flags
	Paranoid   bool
	SkipBanner bool
	Quiet      bool
	Verbose    bool
	Model      string
	JSON       bool // Output in JSON format
	NoNetwork  bool // IL5 compliance (SC-7): Block ALL network except localhost Ollama

	// Command-specific
	Query      string
	File       string
	ConfigKey  string
	ConfigVal  string
	Subcommand string
	Caveat     string // Classification caveat (NOFORN, ORCON, etc.)

	// Agentic mode - enables tool use for ask command
	Agentic    bool
	MaxIter    int // Maximum agentic iterations (default: 10)

	// Raw args (remaining after flag parsing)
	Raw []string

	// Options holds command-specific named options (e.g., --depth, --format)
	Options map[string]string
}

const usageText = `rigrun - AI-powered security-focused CLI assistant

Rigrun is a security-focused AI assistant for the command line.

It provides:
  - Local LLM inference with Ollama (your GPU first)
  - Cloud routing to OpenRouter (when needed)
  - NIST 800-53 compliance features
  - Audit logging and security controls
  - DoD IL5 compatibility

Usage:
  rigrun                     Start TUI (default)
  rigrun ask "question"      Ask a single question
  rigrun chat                Interactive chat
  rigrun status, s           Show system status
  rigrun config [show|set]   Configuration
  rigrun setup               First-run wizard
  rigrun cache [stats|clear] Cache management
  rigrun session, sessions [subcommand] Session management (IL5 AC-12)
  rigrun audit [subcommand]  Audit log management (IL5 AU-5, AU-6, AU-9, AU-11)
  rigrun verify [subcommand]  Integrity verification (SI-7)
  rigrun consent [subcommand] Consent management (IL5 AC-8)
  rigrun classify [show|set] Classification management (IL5)
  rigrun encrypt [subcommand] Encryption management (SC-28)
  rigrun crypto [subcommand]  Cryptographic controls (SC-13, IA-7, SC-17)
  rigrun backup [subcommand]  Backup & recovery management (CP-9, CP-10)
  rigrun conmon [subcommand]  Continuous monitoring (CA-7)
  rigrun lockout [subcommand] Account lockout management (AC-7)
  rigrun auth [subcommand]    Authentication management (IA-2)
  rigrun boundary [subcommand] Network boundary protection (SC-7)
  rigrun sectest [subcommand] Security testing (SA-11)
  rigrun maintenance [subcommand] Maintenance mode management (MA-4, MA-5)
  rigrun test [subcommand]   Built-in self-test (IL5 CI/CD)
  rigrun doctor [--fix]      System diagnostics

Test Commands (IL5 Built-In Self-Test for CI/CD):
  rigrun test all                   Run all self-tests
  rigrun test security              Run security-focused tests only
  rigrun test connectivity          Test Ollama and OpenRouter connections
  rigrun test tools                 Test tool sandboxing is working
  rigrun test config                Validate configuration
  rigrun test audit                 Verify audit logging works
    --json                          Output in JSON for CI/CD integration
    --verbose                       Show detailed test output

Consent Commands (IL5 AC-8 System Use Notification):
  rigrun consent show               Display DoD consent banner text
  rigrun consent status             Show current consent status
  rigrun consent accept             Accept and record consent
  rigrun consent reset              Reset consent (re-acknowledgment)
  rigrun consent require            Re-enable mandatory consent (already default)
  rigrun consent not-required       Disable consent (WARNING: non-compliant with IL5)

  NOTE: Consent is REQUIRED by default for IL5 compliance.

Session Management Commands (IL5 AC-12 Compliance):
  rigrun session list               List all saved sessions
  rigrun session show <id>          Show session details
  rigrun session export <id>        Export session transcript
    --format json|md|txt            Export format (default: txt)
  rigrun session delete <id>        Delete a session
    --confirm                       Required confirmation flag
  rigrun session delete-all         Delete all sessions
    --confirm                       Required confirmation flag
  rigrun session stats              Show session statistics

Audit Commands (IL5 AU-5, AU-6, AU-9, AU-11 Compliance):
  rigrun audit show                 Display recent audit log entries (default: 50)
    --lines N                       Show last N entries
    --since DATE                    Filter by date (YYYY-MM-DD or relative: 1h, 24h, 7d)
    --type TYPE                     Filter by event type (QUERY, SESSION_START, etc.)
  rigrun audit export               Export audit logs for SIEM integration
    --format json|csv|syslog        Export format (default: json)
    --output FILE                   Export to file (default: stdout)
  rigrun audit clear --confirm      Clear audit logs (AU-11: requires confirmation)
  rigrun audit stats                Show audit log statistics
  rigrun audit capacity             (AU-5) Show storage capacity and thresholds
  rigrun audit verify               (AU-5) Verify audit log integrity
  rigrun audit alert                (AU-5) Show alert configuration

Verify Commands (NIST 800-53 SI-7 Integrity Verification):
  rigrun verify all                 Verify all integrity checks (default)
  rigrun verify binary              Verify binary (executable) integrity
  rigrun verify config              Verify config file integrity
  rigrun verify audit               Verify audit log integrity
  rigrun verify checksum            Verify all tracked file checksums
  rigrun verify update              Update baseline checksums (after legitimate changes)
    --json                          Output in JSON format

Classification Commands (IL5 Compliance):
  rigrun classify show              Show current classification level
  rigrun classify set <LEVEL>       Set classification level
  rigrun classify set <LEVEL> --caveat NOFORN
                                    Set with caveats
  rigrun classify banner            Display full classification banner
  rigrun classify validate <text>   Check text for classification markers
  rigrun classify levels            List available classification levels

  Levels: UNCLASSIFIED, CUI, CONFIDENTIAL, SECRET, TOP_SECRET

Encryption Commands (NIST 800-53 SC-28: Protection of Information at Rest):
  rigrun encrypt init             Initialize encryption (creates master key)
    --password                    Use password-based encryption instead of system key
  rigrun encrypt status           Show encryption status
  rigrun encrypt config           Encrypt config file sensitive fields
  rigrun encrypt cache            Encrypt cache database
  rigrun encrypt audit            Encrypt audit logs (optional, for highly sensitive deployments)
  rigrun encrypt rotate           Rotate master encryption key
    --confirm                     Required confirmation flag for key rotation

  Algorithm: AES-256-GCM with PBKDF2-SHA-256 key derivation
  Key Storage: DPAPI (Windows), File-based with 0600 permissions (Unix)

Crypto Commands (NIST 800-53 SC-13/IA-7/SC-17: Cryptographic Protection & PKI):
  rigrun crypto status          Show cryptographic status and FIPS compliance
  rigrun crypto algorithms      List supported algorithms with FIPS status
    --type TYPE                 Filter by type (symmetric, hash, signature, kdf, key_exchange, mac)
  rigrun crypto verify          Verify FIPS 140-2/3 compliance
    --fips                      Strict FIPS mode verification
  rigrun crypto cert check HOST Check TLS certificate for host
  rigrun crypto cert pin HOST   Pin certificate for host
  rigrun crypto cert unpin HOST Remove certificate pin for host
  rigrun crypto cert list       List all pinned certificates
    --json                      Output in JSON format

  FIPS-Approved Algorithms:
    - AES-256-GCM (symmetric)   - SHA-256/384/512 (hash)
    - HMAC-SHA-256 (MAC)        - ECDSA P-256/P-384 (signature)
    - ECDH P-256/P-384 (key exchange)  - RSA-2048+ (signature)
    - PBKDF2-SHA-256 (key derivation)

nVulnerability Scanning Commands (NIST 800-53 RA-5):
  rigrun vuln scan              Run full vulnerability scan
    --deps                      Scan dependencies only
    --config                    Scan configuration only
    --binary                    Scan binary only
  rigrun vuln list              List all vulnerabilities
    --severity LEVEL            Filter by severity (CRITICAL/HIGH/MEDIUM/LOW/INFO)
  rigrun vuln show <id>         Show vulnerability details
  rigrun vuln export            Export vulnerability report
    --format json|csv           Export format (default: json)
    --output FILE               Export to file (default: stdout)
  rigrun vuln schedule INTERVAL Set scan schedule (e.g., 24h, 7d)
  rigrun vuln status            Show last scan time and summary
    --json                      Output in JSON format

  Severity Levels: CRITICAL, HIGH, MEDIUM, LOW, INFO
  Scans for: Outdated dependencies, known CVEs, insecure crypto usage,
             hardcoded credentials, insecure TLS settings

nBackup Commands (NIST 800-53 CP-9/CP-10: System Backup & Recovery):
  rigrun backup create [type]   Create encrypted backup (config/cache/audit/full)
  rigrun backup restore <id>    Restore from backup with integrity verification
    --confirm                   Skip confirmation prompt
  rigrun backup list            List all available backups
  rigrun backup verify <id>     Verify backup integrity (checksum validation)
  rigrun backup delete <id>     Securely delete backup (DoD 5220.22-M)
    --confirm                   Skip confirmation prompt
  rigrun backup schedule <interval> [type]
                                Configure automated backup schedule
                                Interval: 1h, 24h, 7d, 30d
  rigrun backup status          Show backup status and schedule
    --json                      Output in JSON format

  Backup Types:
    - config: Configuration files
    - cache:  Conversation cache
    - audit:  Audit logs
    - full:   Complete backup (all files)

  Features:
    - AES-256-GCM encryption at rest
    - SHA-256 integrity verification
    - Automated retention policy (keep last N backups)
    - Secure deletion (DoD 5220.22-M standard)

RBAC Commands (NIST 800-53 AC-5/AC-6: Separation of Duties & Least Privilege):
  rigrun rbac status              Show current user role and permissions
  rigrun rbac assign USER ROLE    Assign role to user (admin only)
  rigrun rbac revoke USER         Revoke user's role (admin only)
  rigrun rbac list-roles          List all available roles and permissions
  rigrun rbac list-users          List users and their role assignments
  rigrun rbac check PERMISSION    Check if current user has permission
    --json                        Output in JSON format

  Available Roles:
    - admin:    Full access, user management, audit logs
    - operator: Run queries, manage config, view results
    - auditor:  Read-only logs, export reports, read-only config
    - user:     Read-only access, view own history only

  Example Permissions:
    query:run, config:write, audit:view, user:create, session:manage

Security Testing Commands (NIST 800-53 SA-11: Developer Security Testing):
  rigrun sectest run                Run all security tests
  rigrun sectest run --type TYPE    Run specific test type
  rigrun sectest static             Run static code analysis
  rigrun sectest deps               Check for vulnerable dependencies
  rigrun sectest fuzz [target]      Run fuzz testing
  rigrun sectest report             Generate security test report
  rigrun sectest history            Show test history
  rigrun sectest status             Show last test results
    --json                          Output in JSON format

  Test Types:
    static  - Static code analysis (detect security issues in code)
    deps    - Dependency vulnerability check (CVE scanning)
    auth    - Authentication testing (bypass attempts)
    authz   - Authorization testing (privilege escalation)
    input   - Input validation testing (injection, XSS)
    crypto  - Cryptographic implementation testing (FIPS compliance)

Maintenance Commands (NIST 800-53 MA-4/MA-5: Maintenance Management):
  rigrun maintenance start [reason] Enter maintenance mode
    --operator ID             Operator performing maintenance (required)
    --type TYPE               Type: routine, emergency, diagnostic, update (default: routine)
  rigrun maintenance end      Exit maintenance mode
  rigrun maintenance status   Show current maintenance mode status
  rigrun maintenance history  Show maintenance session history
    --limit N                 Limit to N most recent sessions (default: 50)
  rigrun maintenance log <action> Log a maintenance action (must be in maintenance mode)
    --description TEXT        Description of action (required)
    --target PATH             Target file/resource
    --supervisor ID           Supervisor approval (required for sensitive actions)
  rigrun maintenance personnel List authorized maintenance personnel
  rigrun maintenance add-personnel <id> Authorize maintenance personnel
    --name NAME               Full name (required)
    --role ROLE               Role: technician, supervisor, admin (required)
  rigrun maintenance remove-personnel <id> Remove maintenance authorization

  Action Types: config_change, data_access, system_modify*, log_access,
                user_management*, key_management* (* = requires supervisor approval)

  Compliance: MA-4 (sessions time-limited to 4 hours), MA-5 (personnel authorization)

Global Flags:
  --paranoid      Block all cloud requests (local-only operation)
  --no-network    IL5 SC-7: Block ALL network (air-gapped, localhost Ollama only)
                  Aliases: --offline, --airgapped
  --skip-banner   Skip DoD consent banner
  -q, --quiet     Minimal output
  -v, --verbose   Debug output
  --model NAME    Override default model
  --json          Output in JSON format for IL5 SIEM integration (AU-6, SI-4)

Examples:
  # Basic usage
  rigrun                              Start TUI interface
  rigrun ask "What is Rust?"          Ask a single question
  rigrun chat                         Start interactive chat

  # Ask command with options
  rigrun ask "Review this:" --file x.go     Include file with question
  rigrun ask "List processes" --json        Output response as JSON
  rigrun ask --local "Explain this error"   Force local model only
  rigrun ask --agentic "Find all TODO comments"  Enable tool use mode

  # Chat command options
  rigrun chat --model qwen2.5:14b     Start chat with specific model
  rigrun chat --paranoid              Local-only chat mode

  # Configuration and status
  rigrun status                       Check system status (alias: s)
  rigrun config show                  Show current configuration
  rigrun config set openrouter_key YOUR_KEY  Configure cloud access
  rigrun config set paranoid_mode true       Enable paranoid mode

  # Session management (aliases: session, sessions)
  rigrun session list                 List all saved sessions (alias: ls, l)
  rigrun session show 1               View session details
  rigrun session export 1 --format md Export first session as Markdown
  rigrun session delete 1 --confirm   Delete first session
  rigrun session stats                Show session statistics

  # Cache management
  rigrun cache stats                  Show cache statistics
  rigrun cache clear                  Clear all cache
  rigrun cache export ./backup/       Export cache to directory

  # Audit and compliance
  rigrun audit show --lines 100       Show last 100 audit entries
  rigrun audit export --format json   Export for SIEM integration
  rigrun audit stats                  Show audit statistics

  # Backup operations
  rigrun backup create full           Create full backup
  rigrun backup list                  List available backups
  rigrun backup restore <id> --confirm Restore from backup

  # Classification (IL5)
  rigrun classify show                Show current classification level
  rigrun classify set SECRET --caveat NOFORN  Set SECRET//NOFORN

  # System diagnostics
  rigrun doctor                       Run health checks
  rigrun doctor --fix                 Attempt auto-fixes
  rigrun test all --json              Run self-tests for CI/CD

  # Security modes
  rigrun --paranoid                   Block all cloud requests
  rigrun --no-network                 IL5 air-gapped mode (localhost only)
  rigrun --offline ask "prompt"       Offline query

Version: %s
`

// PrintUsage prints the usage/help text.
func PrintUsage() {
	fmt.Printf(usageText, Version)
}

// PrintVersion prints version information.
func PrintVersion() {
	fmt.Printf("rigrun version %s\n", Version)
	fmt.Printf("  Git commit: %s\n", GitCommit)
	fmt.Printf("  Build date: %s\n", BuildDate)
}

// Parse parses command-line arguments and returns the command and args.
func Parse() (Command, Args) {
	args := os.Args[1:]

	// Parse global flags first
	remaining, parsedArgs := parseGlobalFlags(args)

	// If no remaining args, default to TUI
	if len(remaining) == 0 {
		return CmdTUI, parsedArgs
	}

	// Check first argument for command
	cmd := strings.ToLower(remaining[0])
	remaining = remaining[1:]
	parsedArgs.Raw = remaining

	switch cmd {
	case "tui":
		return CmdTUI, parsedArgs

	case "ask":
		// Parse ask-specific flags and query
		parseAskArgs(&parsedArgs, remaining)
		return CmdAsk, parsedArgs

	case "chat":
		// Parse chat-specific flags
		parseChatArgs(&parsedArgs, remaining)
		return CmdChat, parsedArgs

	case "status", "s":
		return CmdStatus, parsedArgs

	case "config":
		parseConfigArgs(&parsedArgs, remaining)
		return CmdConfig, parsedArgs

	case "setup":
		parseSetupArgs(&parsedArgs, remaining)
		return CmdSetup, parsedArgs

	case "cache":
		parseCacheArgs(&parsedArgs, remaining)
		return CmdCache, parsedArgs

	case "session", "sessions":
		parseSessionArgs(&parsedArgs, remaining)
		return CmdSession, parsedArgs

	case "doctor":
		parseDoctorArgs(&parsedArgs, remaining)
		return CmdDoctor, parsedArgs

	case "classify", "classification":
		parseClassifyArgs(&parsedArgs, remaining)
		return CmdClassify, parsedArgs

	case "consent":
		parseConsentArgs(&parsedArgs, remaining)
		return CmdConsent, parsedArgs

	case "audit":
		// Argument parsing is done in audit_cmd.go HandleAudit
		if len(remaining) > 0 {
			parsedArgs.Subcommand = remaining[0]
		}
		return CmdAudit, parsedArgs

	case "verify", "integrity":
		// Argument parsing is done in verify_cmd.go HandleVerify
		if len(remaining) > 0 {
			parsedArgs.Subcommand = remaining[0]
		}
		return CmdVerify, parsedArgs

	case "test", "bist":
		parseTestArgs(&parsedArgs, remaining)
		return CmdTest, parsedArgs

	case "encrypt", "encryption":
		parseEncryptArgs(&parsedArgs, remaining)
		return CmdEncrypt, parsedArgs

	case "crypto", "cryptographic":
		// Argument parsing is done in crypto_cmd.go HandleCrypto
		if len(remaining) > 0 {
			parsedArgs.Subcommand = remaining[0]
		}
		return CmdCrypto, parsedArgs

	case "backup", "backups":
		// NIST 800-53 CP-9 (System Backup) & CP-10 (System Recovery)
		// Argument parsing is done in backup_cmd.go HandleBackup
		if len(remaining) > 0 {
			parsedArgs.Subcommand = remaining[0]
		}
		return CmdBackup, parsedArgs

	case "incident", "incidents":
		// NIST 800-53 IR-6: Incident Reporting
		// Argument parsing is done in incident_cmd.go HandleIncident
		if len(remaining) > 0 {
			parsedArgs.Subcommand = remaining[0]
		}
		return CmdIncident, parsedArgs

	case "data", "sanitize":
		// NIST 800-53 IR-9: Data Sanitization and Spillage Response
		// Argument parsing is done in sanitize_cmd.go HandleData
		if len(remaining) > 0 {
			parsedArgs.Subcommand = remaining[0]
		}
		return CmdData, parsedArgs

	case "vuln", "vulnerability", "vulnerabilities":
		// NIST 800-53 RA-5: Vulnerability Monitoring and Scanning
		// Argument parsing is done in vulnscan_cmd.go HandleVuln
		if len(remaining) > 0 {
			parsedArgs.Subcommand = remaining[0]
		}
		return CmdVuln, parsedArgs

	case "rbac", "roles":
		// NIST 800-53 AC-5 (Separation of Duties) & AC-6 (Least Privilege)
		// Argument parsing is done in rbac_cmd.go HandleRBAC
		if len(remaining) > 0 {
			parsedArgs.Subcommand = remaining[0]
		}
		return CmdRBAC, parsedArgs

	case "config-mgmt", "configmgmt", "cm":
		// NIST 800-53 CM-5 (Access Restrictions for Change) & CM-6 (Configuration Settings)
		// Argument parsing is done in configmgmt_cmd.go HandleConfigMgmt
		parsedArgs.Raw = remaining
		return CmdConfigMgmt, parsedArgs

	case "maintenance", "maint", "ma":
		// NIST 800-53 MA-4 (Nonlocal Maintenance) & MA-5 (Maintenance Personnel)
		// Argument parsing is done in maintenance_cmd.go HandleMaintenance
		parsedArgs.Raw = remaining
		return CmdMaintenance, parsedArgs

	case "boundary", "sc7":
		// NIST 800-53 SC-7: Boundary Protection
		// Argument parsing is done in boundary_cmd.go HandleBoundary
		parsedArgs.Raw = remaining
		return CmdBoundary, parsedArgs

	case "sectest", "sa11":
		// NIST 800-53 SA-11: Developer Security Testing
		// Argument parsing is done in sectest_cmd.go HandleSecTest
		parsedArgs.Raw = remaining
		return CmdSecTest, parsedArgs

	case "conmon", "ca7", "monitoring":
		// NIST 800-53 CA-7: Continuous Monitoring
		// Argument parsing is done in conmon_cmd.go HandleConmon
		parsedArgs.Raw = remaining
		return CmdConmon, parsedArgs

	case "lockout", "ac7":
		// NIST 800-53 AC-7: Unsuccessful Logon Attempts
		// Argument parsing is done in lockout_cmd.go HandleLockout
		parsedArgs.Raw = remaining
		return CmdLockout, parsedArgs

	case "auth", "ia2", "authentication":
		// NIST 800-53 IA-2: Identification and Authentication
		// Argument parsing is done in auth_cmd.go HandleAuth
		parsedArgs.Raw = remaining
		return CmdAuth, parsedArgs

	case "training", "at2":
		// NIST 800-53 AT-2: Security Awareness Training
		// Argument parsing is done in training_cmd.go HandleTraining
		parsedArgs.Raw = remaining
		return CmdTraining, parsedArgs

	case "transport", "sc8":
		// NIST 800-53 SC-8: Transmission Confidentiality and Integrity
		// Argument parsing is done in transport_cmd.go HandleTransport
		parsedArgs.Raw = remaining
		return CmdTransport, parsedArgs

	case "intel", "ci":
		// Competitive Intelligence Research
		// Argument parsing is done in intel_cmd.go HandleIntel
		parseIntelArgs(&parsedArgs, remaining)
		return CmdIntel, parsedArgs

	case "version", "-v", "--version":
		return CmdVersion, parsedArgs

	case "help", "-h", "--help":
		return CmdHelp, parsedArgs

	default:
		// Unknown command - could be a direct prompt, default to TUI
		// Restore the command as it might be part of args
		parsedArgs.Raw = append([]string{cmd}, remaining...)
		return CmdTUI, parsedArgs
	}
}

// parseGlobalFlags extracts global flags from args and returns remaining args.
func parseGlobalFlags(args []string) ([]string, Args) {
	var remaining []string
	parsedArgs := Args{
		Options: make(map[string]string),
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		switch arg {
		case "--paranoid":
			parsedArgs.Paranoid = true
		case "--skip-banner":
			parsedArgs.SkipBanner = true
		case "-q", "--quiet":
			parsedArgs.Quiet = true
		case "-v", "--verbose":
			parsedArgs.Verbose = true
		case "--json":
			parsedArgs.JSON = true
		case "--no-network", "--offline", "--airgapped":
			// IL5 compliance (SC-7): Block ALL network except localhost Ollama
			parsedArgs.NoNetwork = true
		case "--model":
			if i+1 < len(args) {
				i++
				parsedArgs.Model = args[i]
			}
		default:
			// Check for --model=value format
			if strings.HasPrefix(arg, "--model=") {
				parsedArgs.Model = strings.TrimPrefix(arg, "--model=")
			} else {
				remaining = append(remaining, arg)
			}
		}
		i++
	}

	return remaining, parsedArgs
}

// parseAskArgs parses ask command specific arguments.
func parseAskArgs(args *Args, remaining []string) {
	var query []string

	// Default max iterations for agentic mode (higher for complex tasks)
	args.MaxIter = 25

	i := 0
	for i < len(remaining) {
		arg := remaining[i]

		switch arg {
		case "-f", "--file":
			if i+1 < len(remaining) {
				i++
				args.File = remaining[i]
			}
		case "-m", "--model":
			if i+1 < len(remaining) {
				i++
				args.Model = remaining[i]
			}
		case "-a", "--agentic":
			args.Agentic = true
		case "--max-iter":
			if i+1 < len(remaining) {
				i++
				if n, err := strconv.Atoi(remaining[i]); err == nil && n > 0 {
					args.MaxIter = n
				}
			}
		default:
			// Check for --file=value or --model=value format
			if strings.HasPrefix(arg, "--file=") {
				args.File = strings.TrimPrefix(arg, "--file=")
			} else if strings.HasPrefix(arg, "--model=") {
				args.Model = strings.TrimPrefix(arg, "--model=")
			} else if strings.HasPrefix(arg, "--max-iter=") {
				if n, err := strconv.Atoi(strings.TrimPrefix(arg, "--max-iter=")); err == nil && n > 0 {
					args.MaxIter = n
				}
			} else if !strings.HasPrefix(arg, "-") {
				query = append(query, arg)
			}
		}
		i++
	}

	args.Query = strings.Join(query, " ")
}

// parseChatArgs parses chat command specific arguments.
func parseChatArgs(args *Args, remaining []string) {
	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "-m", "--model":
			if i+1 < len(remaining) {
				i++
				args.Model = remaining[i]
			}
		default:
			if strings.HasPrefix(arg, "--model=") {
				args.Model = strings.TrimPrefix(arg, "--model=")
			}
		}
	}
}

// parseConfigArgs parses config command specific arguments.
func parseConfigArgs(args *Args, remaining []string) {
	if len(remaining) > 0 {
		args.Subcommand = remaining[0]
		if len(remaining) > 1 {
			args.ConfigKey = remaining[1]
		}
		if len(remaining) > 2 {
			args.ConfigVal = remaining[2]
		}
	}
}

// parseSetupArgs parses setup command specific arguments.
func parseSetupArgs(args *Args, remaining []string) {
	if len(remaining) > 0 {
		args.Subcommand = remaining[0]
	}
}

// parseCacheArgs parses cache command specific arguments.
func parseCacheArgs(args *Args, remaining []string) {
	if len(remaining) > 0 {
		args.Subcommand = remaining[0]
	}
}

// parseDoctorArgs parses doctor command specific arguments.
func parseDoctorArgs(args *Args, remaining []string) {
	for _, arg := range remaining {
		if arg == "--fix" {
			args.Subcommand = "fix"
		}
	}
}

// parseSessionArgs parses session command specific arguments.
// Detailed argument parsing is done in session_cmd.go.
func parseSessionArgs(args *Args, remaining []string) {
	if len(remaining) > 0 {
		args.Subcommand = remaining[0]
	}
}

// parseClassifyArgs parses classify command specific arguments.
func parseClassifyArgs(args *Args, remaining []string) {
	if len(remaining) > 0 {
		args.Subcommand = remaining[0]
		// For "set" subcommand, capture the level and optional caveat
		if args.Subcommand == "set" && len(remaining) > 1 {
			args.Query = remaining[1] // Level (e.g., SECRET, CUI)
		}
		// For "validate" subcommand, capture the text to validate
		if args.Subcommand == "validate" && len(remaining) > 1 {
			args.Query = strings.Join(remaining[1:], " ")
		}
	}
	// Parse --caveat flag
	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]
		switch arg {
		case "--caveat", "-c":
			if i+1 < len(remaining) {
				args.Caveat = remaining[i+1]
				i++
			}
		default:
			if strings.HasPrefix(arg, "--caveat=") {
				args.Caveat = strings.TrimPrefix(arg, "--caveat=")
			}
		}
	}
}

// parseConsentArgs parses consent command specific arguments.
func parseConsentArgs(args *Args, remaining []string) {
	if len(remaining) > 0 {
		args.Subcommand = remaining[0]
	}
}

// parseIntelArgs parses intel command specific arguments.
func parseIntelArgs(args *Args, remaining []string) {
	// First positional arg is company name or subcommand
	if len(remaining) > 0 {
		args.Subcommand = remaining[0]
	}

	// Parse named options
	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]
		switch {
		case arg == "--depth" && i+1 < len(remaining):
			args.Options["depth"] = remaining[i+1]
			i++
		case strings.HasPrefix(arg, "--depth="):
			args.Options["depth"] = strings.TrimPrefix(arg, "--depth=")
		case arg == "--format" && i+1 < len(remaining):
			args.Options["format"] = remaining[i+1]
			i++
		case strings.HasPrefix(arg, "--format="):
			args.Options["format"] = strings.TrimPrefix(arg, "--format=")
		case arg == "--output" && i+1 < len(remaining):
			args.Options["output"] = remaining[i+1]
			i++
		case strings.HasPrefix(arg, "--output="):
			args.Options["output"] = strings.TrimPrefix(arg, "--output=")
		case arg == "--classification" && i+1 < len(remaining):
			args.Options["classification"] = remaining[i+1]
			i++
		case arg == "--max-iter" && i+1 < len(remaining):
			args.Options["max-iter"] = remaining[i+1]
			i++
		case arg == "--model" && i+1 < len(remaining):
			args.Options["model"] = remaining[i+1]
			i++
		case strings.HasPrefix(arg, "--model="):
			args.Options["model"] = strings.TrimPrefix(arg, "--model=")
		case arg == "--paranoid":
			args.Options["paranoid"] = "true"
		case arg == "--update":
			args.Options["update"] = "true"
		case arg == "--force":
			args.Options["force"] = "true"
		case arg == "--no-cache":
			args.Options["no-cache"] = "true"
		case arg == "--compare" && i+1 < len(remaining):
			args.Options["compare"] = remaining[i+1]
			i++
		case strings.HasPrefix(arg, "--compare="):
			args.Options["compare"] = strings.TrimPrefix(arg, "--compare=")
		}
	}
}

// =============================================================================
// COMMAND HANDLERS
// =============================================================================

// ERROR HANDLING: Errors must not be silently ignored

// HandleAsk handles the "ask" command.
// This delegates to the full implementation in ask.go.
func HandleAsk(args Args) {
	if err := HandleAskCommand(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(GetExitCode(err))
	}
}

// HandleChat handles the "chat" command.
// This delegates to the full implementation in chat.go.
func HandleChat(args Args) {
	if err := HandleChatCommand(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(GetExitCode(err))
	}
}

// NOTE: HandleStatus is implemented in status.go
// NOTE: HandleConfig is implemented in config.go
// NOTE: HandleSetup is implemented in setup.go
// NOTE: HandleCache is implemented in cache_cmd.go
// NOTE: HandleDoctor is implemented in doctor.go

// HandleVersion handles the "version" command.
func HandleVersion() {
	PrintVersion()
}

// HandleVersionWithJSON handles the "version" command with JSON output support.
func HandleVersionWithJSON(args Args) {
	if args.JSON {
		data := VersionData{
			Version:   Version,
			GitCommit: GitCommit,
			BuildDate: BuildDate,
			GoVersion: runtime.Version(),
		}
		resp := NewJSONResponse("version", data)
		resp.Print()
		return
	}
	PrintVersion()
}

// HandleHelp handles the "help" command.
func HandleHelp() {
	PrintUsage()
}
