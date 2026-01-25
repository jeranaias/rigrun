// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
// bash.go implements shell command execution with security restrictions.
package tools

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// SECURITY: Unicode Normalization
// =============================================================================

// normalizeCommand normalizes unicode to NFKC form to prevent homoglyph attacks.
// NFKC (Compatibility Decomposition + Canonical Composition) converts lookalike
// characters to their canonical forms, preventing bypasses using unicode homoglyphs.
func normalizeCommand(cmd string) string {
	// Normalize to NFKC form (compatibility decomposition + canonical composition)
	normalized := norm.NFKC.String(cmd)
	return normalized
}

// =============================================================================
// SECURITY: Wrapped Shell Detection
// =============================================================================

// wrappedShellPatterns detect attempts to bypass security by wrapping commands
// in another shell invocation (e.g., sh -c "malicious command").
var wrappedShellPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)sh\s+-c`),
	regexp.MustCompile(`(?i)bash\s+-c`),
	regexp.MustCompile(`(?i)zsh\s+-c`),
	regexp.MustCompile(`(?i)ksh\s+-c`),
	regexp.MustCompile(`(?i)dash\s+-c`),
	regexp.MustCompile(`(?i)/bin/sh\s+-c`),
	regexp.MustCompile(`(?i)/bin/bash\s+-c`),
	regexp.MustCompile(`(?i)/bin/zsh\s+-c`),
	regexp.MustCompile(`(?i)/usr/bin/sh\s+-c`),
	regexp.MustCompile(`(?i)/usr/bin/bash\s+-c`),
	regexp.MustCompile(`(?i)/usr/bin/env\s+sh\s+-c`),
	regexp.MustCompile(`(?i)/usr/bin/env\s+bash\s+-c`),
}

// detectWrappedShell checks if a command attempts to bypass security by
// wrapping the payload in another shell invocation.
func detectWrappedShell(cmd string) bool {
	for _, pattern := range wrappedShellPatterns {
		if pattern.MatchString(cmd) {
			return true
		}
	}
	return false
}

// =============================================================================
// SECURITY: Backtick Detection (Quote-Aware)
// =============================================================================

// containsBacktickAnywhere checks if a command contains backticks ANYWHERE.
// Backticks are dangerous in ANY context in shell - even inside quotes they
// can execute commands. This is a critical security check.
func containsBacktickAnywhere(cmd string) bool {
	// Backticks are dangerous in ANY context in shell
	return strings.Contains(cmd, "`")
}

// =============================================================================
// SECURITY: Additional Eval Patterns
// =============================================================================

// evalPatterns detect eval-like commands that can execute arbitrary code.
// These patterns catch various ways to dynamically execute commands.
var evalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\beval\b`),
	regexp.MustCompile(`(?i)\bbuiltin\b`),
	regexp.MustCompile(`(?i)\bcommand\b`),
	regexp.MustCompile(`(?i)\bsource\b`),
	regexp.MustCompile(`(?i)^\s*\.\s+`), // . (dot command) at start
	regexp.MustCompile(`(?i);\s*\.\s+`), // . (dot command) after semicolon
	regexp.MustCompile(`(?i)\|\s*\.\s+`), // . (dot command) after pipe
}

// detectEvalPattern checks if a command contains eval-like patterns.
func detectEvalPattern(cmd string) (bool, string) {
	for _, pattern := range evalPatterns {
		if pattern.MatchString(cmd) {
			return true, pattern.String()
		}
	}
	return false, ""
}

// =============================================================================
// SECURITY: Additional Data Exfiltration Patterns
// =============================================================================

// additionalExfilPatterns detect additional data exfiltration attempts
// beyond the basic curl/wget checks.
var additionalExfilPatterns = []*regexp.Regexp{
	// wget with body/post data
	regexp.MustCompile(`(?i)wget\s+.*--body-data`),
	regexp.MustCompile(`(?i)wget\s+.*--post-data`),
	regexp.MustCompile(`(?i)wget\s+.*--post-file`),
	// netcat/nc sending data
	regexp.MustCompile(`(?i)\bnc\s+.*<`),
	regexp.MustCompile(`(?i)\bnetcat\s+.*<`),
	regexp.MustCompile(`(?i)\bnc\s+-.*\|`),
	regexp.MustCompile(`(?i)\bnetcat\s+-.*\|`),
	// base64 encoding piped to network tools
	regexp.MustCompile(`(?i)base64\s+.*\|.*curl`),
	regexp.MustCompile(`(?i)base64\s+.*\|.*wget`),
	regexp.MustCompile(`(?i)base64\s+.*\|.*nc\b`),
	regexp.MustCompile(`(?i)base64\s+.*\|.*netcat`),
	// xxd encoding piped to network tools
	regexp.MustCompile(`(?i)xxd\s+.*\|.*curl`),
	regexp.MustCompile(`(?i)xxd\s+.*\|.*wget`),
	// openssl encoding piped to network tools
	regexp.MustCompile(`(?i)openssl\s+.*\|.*curl`),
	regexp.MustCompile(`(?i)openssl\s+.*\|.*wget`),
}

// detectAdditionalExfiltration checks for additional data exfiltration patterns.
func detectAdditionalExfiltration(cmd string) (bool, string) {
	for _, pattern := range additionalExfilPatterns {
		if pattern.MatchString(cmd) {
			return true, pattern.String()
		}
	}
	return false, ""
}

// =============================================================================
// CONSTANTS
// =============================================================================

// Security limits for command validation
const (
	// maxNewlines is the maximum allowed newlines in a command.
	// Reduced from 50 to 3 to prevent multi-command injection attacks.
	maxNewlines = 3

	// maxHexEscapes is the maximum allowed hex escape sequences (\x) in a command.
	// Reduced from 5 to 1 to prevent obfuscation attacks.
	maxHexEscapes = 1

	// maxOctalEscapes is the maximum allowed octal escape sequences (\0) in a command.
	// Set to 1 to prevent obfuscation attacks.
	maxOctalEscapes = 1
)

// DefaultBlockedCommands are commands that are always blocked for safety.
// SECURITY CRITICAL: This list protects military systems from destructive operations.
var DefaultBlockedCommands = []string{
	// ==========================================================================
	// DESTRUCTIVE FILE OPERATIONS
	// ==========================================================================
	// Root/home directory destruction
	"rm -rf /", "rm -rf /*", "rm -rf .", "rm -rf ..",
	"rm -rf ~", "rm -rf ~/*", "rm -rf ~/",
	"rm -rf $HOME", "rm -rf %USERPROFILE%",
	"rm -fr /", "rm -fr /*", // Alternative flag order
	"rmdir /s /q c:", "rmdir /s /q c:\\",
	// Dangerous rm variations
	"rm --no-preserve-root",

	// ==========================================================================
	// DISK AND PARTITION OPERATIONS
	// ==========================================================================
	"dd if=", "dd of=/dev",
	"mkfs", "mkfs.", "mke2fs", "mkswap",
	"fdisk", "gdisk", "parted", "cfdisk", "sfdisk",
	"wipefs", "shred", "badblocks",
	"hdparm", "smartctl --all /dev",

	// ==========================================================================
	// FORK BOMBS AND RESOURCE EXHAUSTION
	// ==========================================================================
	":(){:|:&};:", ":(){ :|:& };:",
	"bomb()", "fork()", // Common fork bomb patterns
	"while true; do", "while :; do",  // Infinite loops
	"yes |", "yes|",

	// ==========================================================================
	// DANGEROUS PERMISSIONS
	// ==========================================================================
	"chmod -R 777 /", "chmod 777 /",
	"chmod -R 000", "chmod 000 /",
	"chown -R", // Recursive ownership change on system dirs
	"chattr +i /", "chattr -i /",

	// ==========================================================================
	// DEVICE AND KERNEL OPERATIONS
	// ==========================================================================
	"> /dev/sda", "> /dev/nvme", "> /dev/hd",
	"echo > /dev/sd", "cat > /dev/sd",
	"/dev/null >", // Redirecting to critical files
	"insmod", "rmmod", "modprobe -r",
	"sysctl -w", // Kernel parameter modification

	// ==========================================================================
	// REMOTE CODE EXECUTION (CRITICAL FOR MILITARY SECURITY)
	// ==========================================================================
	"curl | bash", "curl | sh", "curl | python",
	"wget | bash", "wget | sh", "wget | python",
	"curl|bash", "curl|sh", "curl|python",
	"wget|bash", "wget|sh", "wget|python",
	"curl -s | bash", "wget -q | bash",
	"curl -sSL | bash", "curl -fsSL | bash",
	"bash <(curl", "sh <(curl", "bash <(wget", "sh <(wget",
	"python -c \"import urllib", // Python remote execution
	"python3 -c \"import urllib",

	// ==========================================================================
	// WINDOWS DESTRUCTIVE COMMANDS
	// ==========================================================================
	"format c:", "format d:", "format e:",
	"del /f /s /q c:\\", "del /f /s /q c:/",
	"rd /s /q c:\\", "rd /s /q c:/",
	"rmdir /s /q c:", "rmdir /s /q c:\\",
	"cipher /w:", // Secure wipe
	"diskpart", "clean all",
	"bcdedit", "bootrec",

	// ==========================================================================
	// SYSTEM CONTROL
	// ==========================================================================
	"shutdown -h now", "shutdown /s", "shutdown -r now",
	"poweroff", "init 0", "init 6",
	"reboot", "halt",
	"systemctl poweroff", "systemctl reboot", "systemctl halt",

	// ==========================================================================
	// NETWORK SECURITY (CRITICAL FOR MILITARY)
	// ==========================================================================
	"iptables -F", "iptables --flush", // Flush firewall rules
	"ufw disable", "firewall-cmd --disable",
	"netsh advfirewall set", "netsh firewall",
	"nc -e", "nc -c", "ncat -e", // Reverse shells
	"bash -i >& /dev/tcp", // Bash reverse shell
	"/bin/bash -i >& /dev/tcp",
	"python -c 'import socket", // Python reverse shell
	"perl -e 'use Socket", // Perl reverse shell
	"ruby -rsocket", // Ruby reverse shell

	// ==========================================================================
	// CREDENTIAL AND SENSITIVE DATA ACCESS
	// ==========================================================================
	"cat /etc/shadow", "cat /etc/passwd",
	"cat /etc/sudoers", "visudo",
	"/etc/ssh/sshd_config",
	".ssh/id_rsa", ".ssh/id_ed25519", ".ssh/authorized_keys",
	".gnupg/", ".gpg/",
	".aws/credentials", ".azure/", ".kube/config",
	"type %USERPROFILE%\\.ssh",

	// ==========================================================================
	// HISTORY AND LOG TAMPERING
	// ==========================================================================
	"history -c", "history -w /dev/null",
	"rm ~/.bash_history", "rm ~/.zsh_history",
	"> ~/.bash_history", "> ~/.zsh_history",
	"shred ~/.bash_history",
	"rm /var/log", "truncate /var/log", "> /var/log",

	// ==========================================================================
	// PACKAGE MANAGER ABUSE
	// ==========================================================================
	"pip install --user -e", // Editable installs can be dangerous
	"npm install -g", // Global installs without audit
	"gem install", // Ruby gem installs
}

// DefaultBlockedPatterns are patterns that are always blocked.
// SECURITY CRITICAL: Pattern matching catches obfuscation attempts.
var DefaultBlockedPatterns = []string{
	// ==========================================================================
	// DEVICE ACCESS PATTERNS
	// ==========================================================================
	"> /dev/sd",   // Writing to SATA/SAS disks
	"> /dev/nvme", // Writing to NVMe drives
	"> /dev/hd",   // Writing to IDE disks
	"> /dev/vd",   // Writing to virtual disks
	">/dev/sd",    // No space variant
	">/dev/nvme",
	"of=/dev/",    // dd output to device

	// ==========================================================================
	// COMMAND CHAINING WITH DESTRUCTIVE OPERATIONS
	// ==========================================================================
	"| rm ",       // Piping to rm
	"|rm ",        // No space variant
	"; rm -rf",    // Chained destructive
	"&& rm -rf",   // Chained destructive
	"|| rm -rf",   // Conditional destructive
	"; rm -r",     // Less aggressive but still dangerous
	"&& rm -r",

	// ==========================================================================
	// SHELL EXECUTION PATTERNS
	// ==========================================================================
	"| bash",      // Piping to bash
	"| sh",        // Piping to sh
	"|bash",       // No space variants
	"|sh",
	"| eval",      // Piping to eval
	"|eval",
	"| python -c", // Piping to python exec
	"| python3 -c",
	"| perl -e",   // Piping to perl exec
	"| ruby -e",   // Piping to ruby exec
	"; exec ",     // Shell exec
	"&& exec ",
	"`",           // Backtick command substitution (often used for injection)
	"$(",          // Command substitution

	// ==========================================================================
	// REVERSE SHELL PATTERNS
	// ==========================================================================
	"/dev/tcp/",   // Bash TCP redirects
	"/dev/udp/",   // Bash UDP redirects
	"mkfifo",      // Named pipe (often used for reverse shells)
	"mknod",       // Device node creation

	// ==========================================================================
	// ENCODING/OBFUSCATION DETECTION
	// ==========================================================================
	"base64 -d |",  // Decode and execute
	"base64 --decode |",
	"| base64 -d",
	"xxd -r |",     // Hex decode and execute
	"printf '\\x",  // Hex escape sequences

	// ==========================================================================
	// ENVIRONMENT MANIPULATION
	// ==========================================================================
	"export PATH=",      // PATH manipulation
	"PATH=",             // Inline PATH override
	"LD_PRELOAD=",       // Library injection
	"LD_LIBRARY_PATH=",  // Library path manipulation
	"DYLD_INSERT_LIBRARIES=", // macOS library injection

	// ==========================================================================
	// SENSITIVE FILE ACCESS PATTERNS
	// ==========================================================================
	"/etc/shadow",
	"/etc/sudoers",
	"/etc/gshadow",
	"/.ssh/",
	"\\.ssh\\",
	"/id_rsa",
	"/id_ed25519",
	"/id_ecdsa",
	"/id_dsa",

	// ==========================================================================
	// WINDOWS-SPECIFIC PATTERNS
	// ==========================================================================
	"\\system32\\",
	"\\syswow64\\",
	"/system32/",
	"reg add",        // Registry modification
	"reg delete",
	"reg import",
	"schtasks /create", // Scheduled task creation
	"at ",              // Legacy task scheduler
}

// InteractiveCommands are commands that require TTY and should be blocked.
var InteractiveCommands = []string{
	"vim", "vi", "nano", "emacs", "pico",
	"less", "more",
	"top", "htop", "btop",
	"ssh", "telnet", "ftp",
	"mysql", "psql", "sqlite3",
	"python -i", "python3 -i", "node -i",
	"irb", "ghci",
	"bash -i", "sh -i", "zsh -i",
}

// DangerousFlags are flags that trigger warnings.
var DangerousFlags = []string{
	"--force",
	"-f",
	"--no-preserve-root",
	"--recursive",
	"-r",
	"-rf",
}

// =============================================================================
// BASH EXECUTOR
// =============================================================================

// BashExecutor implements shell command execution with security restrictions.
type BashExecutor struct {
	// WorkDir is the working directory for commands
	WorkDir string

	// DefaultTimeout is the default command timeout (default: 30 seconds)
	DefaultTimeout time.Duration

	// MaxTimeout is the maximum allowed timeout (default: 10 minutes)
	MaxTimeout time.Duration

	// MaxOutputSize is the maximum output size (default: 100KB)
	MaxOutputSize int

	// AllowedCommands - if set, only these commands are allowed (whitelist mode)
	AllowedCommands []string

	// BlockedCommands are commands that are never allowed
	BlockedCommands []string

	// BlockedPatterns are patterns to block
	BlockedPatterns []string
}

// Execute runs a shell command with security restrictions.
func (e *BashExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	start := time.Now()

	// Set defaults
	if e.DefaultTimeout == 0 {
		e.DefaultTimeout = 30 * time.Second
	}
	if e.MaxTimeout == 0 {
		e.MaxTimeout = 10 * time.Minute
	}
	if e.MaxOutputSize == 0 {
		e.MaxOutputSize = 100000 // 100KB
	}
	if len(e.BlockedCommands) == 0 {
		e.BlockedCommands = DefaultBlockedCommands
	}
	if len(e.BlockedPatterns) == 0 {
		e.BlockedPatterns = DefaultBlockedPatterns
	}

	// Extract parameters
	command, _ := params["command"].(string)
	timeoutSec := getIntParam(params, "timeout", int(e.DefaultTimeout.Seconds()))

	// Validate command
	if command == "" {
		return Result{
			Success:  false,
			Error:    "command is required",
			Duration: time.Since(start),
		}, nil
	}

	// Security validation
	if err := e.validateCommand(command); err != nil {
		return Result{
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(start),
		}, nil
	}

	// Calculate timeout
	timeout := time.Duration(timeoutSec) * time.Second
	if timeout > e.MaxTimeout {
		timeout = e.MaxTimeout
	}
	if timeout < time.Second {
		timeout = time.Second
	}

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create command based on platform
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(cmdCtx, "bash", "-c", command)
	}

	// Set working directory
	if e.WorkDir != "" {
		cmd.Dir = e.WorkDir
	}

	// SECURITY: Sanitize environment to prevent injection attacks
	cmd.Env = sanitizeEnvironment()

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()
	duration := time.Since(start)

	// Check for context cancellation (timeout)
	select {
	case <-cmdCtx.Done():
		if cmdCtx.Err() == context.DeadlineExceeded {
			return Result{
				Success:  false,
				Error:    "command timed out after " + formatDuration(timeout),
				Duration: duration,
			}, nil
		}
		if cmdCtx.Err() == context.Canceled {
			return Result{
				Success:  false,
				Error:    "command cancelled",
				Duration: duration,
			}, nil
		}
	default:
	}

	// Build output
	output, truncated := e.buildOutput(&stdout, &stderr)

	// Check for error
	if err != nil {
		errorMsg := "command failed"
		if exitErr, ok := err.(*exec.ExitError); ok {
			errorMsg = "command exited with code " + util.IntToStr(exitErr.ExitCode())
		}

		// Include output even on failure
		return Result{
			Success:   false,
			Error:     errorMsg,
			Output:    output,
			Duration:  duration,
			Truncated: truncated,
		}, nil
	}

	return Result{
		Success:   true,
		Output:    output,
		Duration:  duration,
		Truncated: truncated,
	}, nil
}

// =============================================================================
// COMMAND VALIDATION
// =============================================================================

// validateCommand checks a command against security rules.
func (e *BashExecutor) validateCommand(command string) error {
	// ==========================================================================
	// STEP 1: Unicode Normalization (BASH-3 fix)
	// Normalize unicode to NFKC to prevent homoglyph bypass attacks.
	// This MUST happen first, before any other checks.
	// ==========================================================================
	command = normalizeCommand(command)

	// Use enhanced validation from security.go
	if err := ValidateCommandSecure(command); err != nil {
		return err
	}

	// ==========================================================================
	// STEP 2: Wrapped Shell Detection (BASH-1 fix)
	// Detect attempts to bypass security by wrapping commands in sh -c, bash -c, etc.
	// ==========================================================================
	if detectWrappedShell(command) {
		return &BashSecurityError{
			Command: command,
			Reason:  "wrapped shell execution (sh -c, bash -c, etc.) is blocked for security",
		}
	}

	// ==========================================================================
	// STEP 3: Backtick Detection Anywhere (BASH-2 fix)
	// Backticks are dangerous in ANY context - even inside quotes they execute commands.
	// ==========================================================================
	if containsBacktickAnywhere(command) {
		return &BashSecurityError{
			Command: command,
			Reason:  "backtick command substitution is blocked for security (use $() instead for legitimate needs)",
		}
	}

	normalizedCmd := strings.ToLower(strings.TrimSpace(command))
	normalizedCmd = strings.ReplaceAll(normalizedCmd, "\t", " ")

	// Whitelist mode: only allow specific commands
	if len(e.AllowedCommands) > 0 {
		// Parse command tokens for proper validation
		tokens, err := parseCommandTokens(command)
		if err != nil {
			return &BashSecurityError{
				Command: command,
				Reason:  "failed to parse command: " + err.Error(),
			}
		}

		allowed := false
		if len(tokens) > 0 {
			baseCmd := strings.ToLower(filepath.Base(tokens[0]))
			for _, allow := range e.AllowedCommands {
				if baseCmd == strings.ToLower(allow) {
					allowed = true
					break
				}
			}
		}

		if !allowed {
			return &BashSecurityError{
				Command: command,
				Reason:  "command not in allowed list",
			}
		}
	}

	// Check blocked commands - use token-based matching instead of substring
	tokens, _ := parseCommandTokens(command)
	if len(tokens) > 0 {
		baseCmd := strings.ToLower(filepath.Base(tokens[0]))
		for _, blocked := range e.BlockedCommands {
			if baseCmd == strings.ToLower(blocked) || strings.Contains(normalizedCmd, strings.ToLower(blocked)) {
				return &BashSecurityError{
					Command: command,
					Reason:  "command contains blocked operation: " + blocked,
				}
			}
		}
	}

	// Check blocked patterns
	for _, pattern := range e.BlockedPatterns {
		if strings.Contains(normalizedCmd, strings.ToLower(pattern)) {
			return &BashSecurityError{
				Command: command,
				Reason:  "command contains dangerous pattern",
			}
		}
	}

	// Check for interactive commands
	for _, interactive := range InteractiveCommands {
		interactiveLower := strings.ToLower(interactive)
		if strings.HasPrefix(normalizedCmd, interactiveLower+" ") ||
			normalizedCmd == interactiveLower ||
			strings.Contains(normalizedCmd, "|"+interactiveLower) ||
			strings.Contains(normalizedCmd, "| "+interactiveLower) ||
			strings.Contains(normalizedCmd, ";"+interactiveLower) ||
			strings.Contains(normalizedCmd, "; "+interactiveLower) {
			return &BashSecurityError{
				Command: command,
				Reason:  "interactive command '" + interactive + "' cannot run in non-TTY environment",
			}
		}
	}

	// Check for backgrounding with & (but allow &&)
	if containsBackgroundOperator(command) {
		return &BashSecurityError{
			Command: command,
			Reason:  "backgrounding commands with '&' is not allowed",
		}
	}

	// Check for sudo/su
	if strings.HasPrefix(normalizedCmd, "sudo ") || strings.HasPrefix(normalizedCmd, "su ") {
		return &BashSecurityError{
			Command: command,
			Reason:  "privileged commands require explicit approval",
		}
	}

	// ==========================================================================
	// STEP 4: Enhanced Eval Pattern Detection (BASH-6 fix)
	// Check for eval-like commands including builtin, command, and dot (.) command.
	// These patterns are checked using regex for more robust detection.
	// ==========================================================================
	if matched, pattern := detectEvalPattern(command); matched {
		return &BashSecurityError{
			Command: command,
			Reason:  "eval-like command pattern is blocked for security: " + pattern,
		}
	}

	// ==========================================================================
	// ADDITIONAL SECURITY CHECKS FOR MILITARY-GRADE PROTECTION
	// ==========================================================================

	// Check command length (prevent DoS via extremely long commands)
	if len(command) > 10000 {
		return &BashSecurityError{
			Command: "[command too long to display]",
			Reason:  "command exceeds maximum length of 10000 characters",
		}
	}

	// Check for null bytes (command injection via null byte)
	if strings.Contains(command, "\x00") {
		return &BashSecurityError{
			Command: command,
			Reason:  "command contains null bytes (potential injection)",
		}
	}

	// ==========================================================================
	// STEP 5: Newline Injection Check (BASH-4 fix)
	// Reduced from 50 to 3 max newlines to prevent multi-command injection.
	// ==========================================================================
	newlineCount := strings.Count(command, "\n")
	if newlineCount > maxNewlines {
		return &BashSecurityError{
			Command: command,
			Reason:  "command contains too many newlines (max " + util.IntToStr(maxNewlines) + ") - potential multi-command injection",
		}
	}

	// ==========================================================================
	// STEP 6: Hex/Octal Escape Check (BASH-5 fix)
	// Reduced thresholds from 5/3 to 1/1 to prevent obfuscation attacks.
	// ==========================================================================
	hexCount := strings.Count(command, "\\x")
	octalCount := strings.Count(command, "\\0")
	if hexCount > maxHexEscapes {
		return &BashSecurityError{
			Command: command,
			Reason:  "command contains too many hex escape sequences (max " + util.IntToStr(maxHexEscapes) + ") - potential obfuscation",
		}
	}
	if octalCount > maxOctalEscapes {
		return &BashSecurityError{
			Command: command,
			Reason:  "command contains too many octal escape sequences (max " + util.IntToStr(maxOctalEscapes) + ") - potential obfuscation",
		}
	}

	// Check for runas on Windows (privilege escalation)
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(normalizedCmd, "runas ") {
			return &BashSecurityError{
				Command: command,
				Reason:  "runas command requires explicit approval",
			}
		}
		// PowerShell execution policy bypass
		if strings.Contains(normalizedCmd, "-executionpolicy bypass") ||
			strings.Contains(normalizedCmd, "-ep bypass") ||
			strings.Contains(normalizedCmd, "-exec bypass") {
			return &BashSecurityError{
				Command: command,
				Reason:  "PowerShell execution policy bypass is blocked",
			}
		}
		// PowerShell encoded commands
		if strings.Contains(normalizedCmd, "-encodedcommand") ||
			strings.Contains(normalizedCmd, "-enc ") ||
			strings.Contains(normalizedCmd, "-e ") && strings.Contains(normalizedCmd, "powershell") {
			return &BashSecurityError{
				Command: command,
				Reason:  "PowerShell encoded commands are blocked for security",
			}
		}
	}

	// Check for doas (OpenBSD privilege escalation)
	if strings.HasPrefix(normalizedCmd, "doas ") {
		return &BashSecurityError{
			Command: command,
			Reason:  "doas command requires explicit approval",
		}
	}

	// Check for pkexec (Polkit privilege escalation)
	if strings.HasPrefix(normalizedCmd, "pkexec ") {
		return &BashSecurityError{
			Command: command,
			Reason:  "pkexec command requires explicit approval",
		}
	}

	// Check for network tools that could exfiltrate data
	if containsDataExfiltrationRisk(normalizedCmd) {
		return &BashSecurityError{
			Command: command,
			Reason:  "command may pose data exfiltration risk",
		}
	}

	// ==========================================================================
	// STEP 7: Additional Data Exfiltration Patterns (BASH-7 fix)
	// Check for additional exfiltration patterns including wget, nc, base64, etc.
	// ==========================================================================
	if matched, pattern := detectAdditionalExfiltration(command); matched {
		return &BashSecurityError{
			Command: command,
			Reason:  "command matches data exfiltration pattern: " + pattern,
		}
	}

	return nil
}

// containsDataExfiltrationRisk checks for commands that could exfiltrate sensitive data.
func containsDataExfiltrationRisk(cmd string) bool {
	// Check for curl/wget posting data
	if (strings.Contains(cmd, "curl") || strings.Contains(cmd, "wget")) &&
		(strings.Contains(cmd, "-d ") || strings.Contains(cmd, "--data") ||
			strings.Contains(cmd, "-X POST") || strings.Contains(cmd, "-X PUT") ||
			strings.Contains(cmd, "--post-data") || strings.Contains(cmd, "--post-file")) {
		return true
	}

	// Check for netcat sending data
	if (strings.Contains(cmd, "nc ") || strings.Contains(cmd, "netcat ") ||
		strings.Contains(cmd, "ncat ")) &&
		(strings.Contains(cmd, ">") || strings.Contains(cmd, "|") ||
			strings.Contains(cmd, "-e") || strings.Contains(cmd, "-c")) {
		return true
	}

	// Check for scp/sftp/rsync to external hosts
	if (strings.Contains(cmd, "scp ") || strings.Contains(cmd, "sftp ") ||
		strings.Contains(cmd, "rsync ")) && strings.Contains(cmd, "@") {
		return true
	}

	// Check for ftp uploads
	if strings.Contains(cmd, "ftp ") &&
		(strings.Contains(cmd, "put ") || strings.Contains(cmd, "mput ")) {
		return true
	}

	return false
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// buildOutput combines stdout and stderr with truncation.
func (e *BashExecutor) buildOutput(stdout, stderr *bytes.Buffer) (string, bool) {
	var output strings.Builder
	truncated := false

	// Add stdout
	if stdout.Len() > 0 {
		outStr := stdout.String()
		if len(outStr) > e.MaxOutputSize {
			outStr = outStr[:e.MaxOutputSize]
			truncated = true
		}
		output.WriteString(outStr)
	}

	// Add stderr
	if stderr.Len() > 0 {
		if output.Len() > 0 {
			output.WriteString("\n\nSTDERR:\n")
		}
		errStr := stderr.String()
		remaining := e.MaxOutputSize - output.Len()
		if remaining > 0 {
			if len(errStr) > remaining {
				errStr = errStr[:remaining]
				truncated = true
			}
			output.WriteString(errStr)
		}
	}

	result := output.String()
	if result == "" {
		result = "(no output)"
	}

	if truncated {
		result = result + "\n\n[Output truncated at " + util.IntToStr(e.MaxOutputSize) + " bytes]"
	}

	return result, truncated
}

// containsBackgroundOperator checks for standalone & (not &&).
func containsBackgroundOperator(command string) bool {
	chars := []rune(command)
	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < len(chars); i++ {
		c := chars[i]

		// Track quote state
		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		// Skip if inside quotes
		if inSingleQuote || inDoubleQuote {
			continue
		}

		// Check for standalone &
		if c == '&' {
			var prev, next rune
			if i > 0 {
				prev = chars[i-1]
			}
			if i < len(chars)-1 {
				next = chars[i+1]
			}

			// Skip if part of && (command chaining)
			if prev == '&' || next == '&' {
				continue
			}

			// This is a standalone & (backgrounding)
			return true
		}
	}

	return false
}

// isWordPresent checks if a word appears as a standalone word.
func isWordPresent(text, word string) bool {
	wordLower := strings.ToLower(word)
	// Split on non-alphanumeric characters
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_')
	})
	for _, part := range parts {
		if strings.ToLower(part) == wordLower {
			return true
		}
	}
	return false
}

// BashSecurityError represents a command security violation.
type BashSecurityError struct {
	Command string
	Reason  string
}

func (e *BashSecurityError) Error() string {
	return "security check failed: " + e.Reason
}

// =============================================================================
// ENVIRONMENT SANITIZATION
// =============================================================================

// DangerousEnvVars are environment variables that can be used for injection attacks.
var DangerousEnvVars = []string{
	// Library injection
	"LD_PRELOAD",
	"LD_LIBRARY_PATH",
	"LD_AUDIT",
	"LD_DEBUG",
	"LD_DYNAMIC_WEAK",
	"DYLD_INSERT_LIBRARIES",
	"DYLD_LIBRARY_PATH",
	"DYLD_FRAMEWORK_PATH",

	// Shell behavior modification
	"BASH_ENV",
	"ENV",
	"SHELLOPTS",
	"BASHOPTS",
	"CDPATH",
	"GLOBIGNORE",
	"BASH_FUNC_",

	// Dangerous executables
	"EDITOR",
	"VISUAL",
	"PAGER",

	// IFS injection
	"IFS",

	// Proxy settings (could redirect traffic)
	"http_proxy",
	"https_proxy",
	"HTTP_PROXY",
	"HTTPS_PROXY",
	"ALL_PROXY",
	"all_proxy",
	"ftp_proxy",
	"FTP_PROXY",

	// SSH/GPG agent hijacking
	"SSH_AUTH_SOCK",
	"GPG_AGENT_INFO",

	// Python injection
	"PYTHONSTARTUP",
	"PYTHONPATH",
	"PYTHONHOME",

	// Ruby injection
	"RUBYOPT",
	"RUBYLIB",

	// Perl injection
	"PERL5OPT",
	"PERL5LIB",
	"PERLLIB",

	// Node.js injection
	"NODE_OPTIONS",
	"NODE_PATH",

	// Java injection
	"JAVA_TOOL_OPTIONS",
	"_JAVA_OPTIONS",
	"CLASSPATH",

	// Git hooks (could execute arbitrary code)
	"GIT_SSH",
	"GIT_SSH_COMMAND",
	"GIT_EXEC_PATH",

	// Prompt injection
	"PS1",
	"PS2",
	"PS4",
	"PROMPT_COMMAND",
}

// SafeEnvVars are environment variables that are safe to pass through.
var SafeEnvVars = []string{
	"PATH",
	"HOME",
	"USER",
	"LOGNAME",
	"SHELL",
	"TERM",
	"LANG",
	"LC_ALL",
	"LC_CTYPE",
	"TZ",
	"TMPDIR",
	"TEMP",
	"TMP",
	// Windows essentials
	"USERPROFILE",
	"HOMEDRIVE",
	"HOMEPATH",
	"SYSTEMROOT",
	"COMSPEC",
	"PATHEXT",
	"WINDIR",
	"APPDATA",
	"LOCALAPPDATA",
	"PROGRAMFILES",
	"PROGRAMFILES(X86)",
	"COMMONPROGRAMFILES",
	// Development tools (safe read-only vars)
	"GOPATH",
	"GOROOT",
	"GOPROXY",
	"GOMODCACHE",
	"CARGO_HOME",
	"RUSTUP_HOME",
	"NVM_DIR",
	"SDKMAN_DIR",
}

// sanitizeEnvironment creates a sanitized environment for command execution.
// It filters out dangerous environment variables that could be used for injection.
func sanitizeEnvironment() []string {
	// Build a set of safe vars for quick lookup
	safeSet := make(map[string]bool)
	for _, v := range SafeEnvVars {
		safeSet[strings.ToUpper(v)] = true
	}

	// Build a set of dangerous vars for quick lookup
	dangerousSet := make(map[string]bool)
	for _, v := range DangerousEnvVars {
		dangerousSet[strings.ToUpper(v)] = true
	}

	// Get current environment
	currentEnv := getEnviron()
	result := make([]string, 0, len(currentEnv))

	for _, env := range currentEnv {
		// Parse key=value
		idx := strings.Index(env, "=")
		if idx <= 0 {
			continue
		}
		key := env[:idx]
		keyUpper := strings.ToUpper(key)

		// Skip if explicitly dangerous
		if dangerousSet[keyUpper] {
			continue
		}

		// Skip if starts with dangerous prefix
		if strings.HasPrefix(keyUpper, "BASH_FUNC_") {
			continue
		}
		if strings.HasPrefix(keyUpper, "LD_") {
			continue
		}
		if strings.HasPrefix(keyUpper, "DYLD_") {
			continue
		}

		// Include if safe or not on the dangerous list
		if safeSet[keyUpper] || !dangerousSet[keyUpper] {
			result = append(result, env)
		}
	}

	return result
}

// getEnviron returns the current environment (abstracted for testing).
var getEnviron = func() []string {
	return os.Environ()
}

// formatDuration formats a duration as a string.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return util.IntToStr(int(d.Milliseconds())) + "ms"
	}
	if d < time.Minute {
		secs := int(d.Seconds())
		return util.IntToStr(secs) + "s"
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	if secs == 0 {
		return util.IntToStr(mins) + "m"
	}
	return util.IntToStr(mins) + "m" + util.IntToStr(secs) + "s"
}

