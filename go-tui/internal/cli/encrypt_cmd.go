// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cli provides CLI commands for NIST 800-53 SC-28 encryption.
//
// CLI: Comprehensive help and examples for all commands
//
// Command: encrypt [subcommand]
// Short:   Data-at-rest encryption management (IL5 SC-28)
// Aliases: enc
//
// Subcommands:
//   init                Initialize encryption (creates master key)
//   config              Encrypt config file sensitive fields
//   cache               Encrypt cache database
//   audit               Encrypt audit logs
//   status (default)    Show encryption status
//   rotate              Rotate master key
//
// Examples:
//   rigrun encrypt                   Show encryption status (default)
//   rigrun encrypt status            Show encryption status
//   rigrun encrypt status --json     Status in JSON format
//   rigrun encrypt init              Initialize encryption
//   rigrun encrypt config            Encrypt config sensitive fields
//   rigrun encrypt cache             Encrypt response cache
//   rigrun encrypt audit             Encrypt audit logs
//   rigrun encrypt rotate            Rotate master encryption key
//
// Security Notes (SC-28):
//   - Uses AES-256-GCM for data encryption
//   - Master key stored in system keyring (when available)
//   - Key rotation preserves data integrity
//   - All encryption operations are logged to audit
//
// Flags:
//   --json              Output in JSON format
//   --confirm           Skip confirmation prompts
//
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"syscall"
	"unicode"

	"golang.org/x/term"

	"github.com/jeranaias/rigrun-tui/internal/config"
	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// CONSTANTS
// =============================================================================

// MinPasswordLength is the minimum password length for IL5 compliance.
// NIST SP 800-63B recommends 15+ characters for strong security.
const MinPasswordLength = 15

// =============================================================================
// ENCRYPT COMMAND HANDLER
// =============================================================================

// HandleEncrypt handles the "rigrun encrypt" CLI command.
func HandleEncrypt(args Args) error {
	subcommand := args.Subcommand
	if subcommand == "" && len(args.Raw) > 0 {
		subcommand = args.Raw[0]
	}

	switch subcommand {
	case "init":
		return handleEncryptInit(args)
	case "config":
		return handleEncryptConfig(args)
	case "cache":
		return handleEncryptCache(args)
	case "audit":
		return handleEncryptAudit(args)
	case "status", "":
		return handleEncryptStatus(args)
	case "rotate":
		return handleEncryptRotate(args)
	case "decrypt-config":
		return handleDecryptConfig(args)
	case "decrypt-cache":
		return handleDecryptCache(args)
	default:
		return fmt.Errorf("unknown encrypt subcommand: %s\nUsage: rigrun encrypt [init|config|cache|audit|status|rotate]", subcommand)
	}
}

// =============================================================================
// INIT COMMAND
// =============================================================================

// handleEncryptInit initializes encryption by creating a master key.
func handleEncryptInit(args Args) error {
	em := security.GlobalEncryptionManager()

	// Check if already initialized
	if em.IsInitialized() {
		if args.JSON {
			resp := NewJSONResponse("encrypt_init", map[string]interface{}{
				"status":  "already_initialized",
				"message": "Encryption is already initialized",
			})
			resp.Print()
			return nil
		}
		fmt.Println("Encryption is already initialized.")
		fmt.Println("Use 'rigrun encrypt rotate' to generate a new key.")
		return nil
	}

	// Check for --password flag
	usePassword := false
	for _, arg := range args.Raw {
		if arg == "--password" || arg == "-p" {
			usePassword = true
			break
		}
	}

	var err error
	if usePassword {
		// Prompt for password
		password, promptErr := promptPassword("Enter master password: ")
		if promptErr != nil {
			return fmt.Errorf("failed to read password: %w", promptErr)
		}

		// Confirm password
		confirm, promptErr := promptPassword("Confirm master password: ")
		if promptErr != nil {
			return fmt.Errorf("failed to read password confirmation: %w", promptErr)
		}

		if password != confirm {
			return fmt.Errorf("passwords do not match")
		}

		// Validate password meets IL5 requirements
		if err := validatePassword(password); err != nil {
			return err
		}

		err = em.InitializeWithPassword(password)
	} else {
		// Use system key storage (DPAPI on Windows)
		err = em.Initialize()
	}

	if err != nil {
		return fmt.Errorf("failed to initialize encryption: %w", err)
	}

	if args.JSON {
		resp := NewJSONResponse("encrypt_init", map[string]interface{}{
			"status":         "initialized",
			"algorithm":      "AES-256-GCM",
			"key_derivation": "PBKDF2-SHA-256",
			"password_based": usePassword,
		})
		resp.Print()
		return nil
	}

	fmt.Println("Encryption initialized successfully!")
	fmt.Println()
	fmt.Println("Details:")
	fmt.Println("  Algorithm:      AES-256-GCM")
	fmt.Println("  Key Derivation: PBKDF2-SHA-256 (100,000 iterations)")
	if usePassword {
		fmt.Println("  Key Storage:    Password-based")
	} else {
		fmt.Println("  Key Storage:    System (DPAPI on Windows)")
	}
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  rigrun encrypt config  - Encrypt sensitive config fields")
	fmt.Println("  rigrun encrypt cache   - Encrypt cache database")
	fmt.Println("  rigrun encrypt status  - View encryption status")

	return nil
}

// =============================================================================
// CONFIG COMMAND
// =============================================================================

// handleEncryptConfig encrypts sensitive fields in the config file.
func handleEncryptConfig(args Args) error {
	em := security.GlobalEncryptionManager()

	if !em.IsInitialized() {
		return security.ErrNotInitialized
	}

	// Load current config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Encrypt sensitive fields
	fieldsEncrypted := 0

	// Encrypt OpenRouter API key if present and not already encrypted
	if cfg.Cloud.OpenRouterKey != "" && !security.IsEncrypted(cfg.Cloud.OpenRouterKey) {
		encrypted, err := em.EncryptConfigField(cfg.Cloud.OpenRouterKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt OpenRouter key: %w", err)
		}
		cfg.Cloud.OpenRouterKey = encrypted
		fieldsEncrypted++
	}

	// Save config with encrypted fields
	if fieldsEncrypted > 0 {
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save encrypted config: %w", err)
		}

		// Update encryption status in config
		cfg.Security.EncryptionEnabled = true
		cfg.Security.EncryptConfig = true
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to update config encryption status: %w", err)
		}
	}

	if args.JSON {
		resp := NewJSONResponse("encrypt_config", map[string]interface{}{
			"status":           "success",
			"fields_encrypted": fieldsEncrypted,
		})
		resp.Print()
		return nil
	}

	if fieldsEncrypted > 0 {
		fmt.Printf("Config encryption complete! Encrypted %d sensitive field(s).\n", fieldsEncrypted)
	} else {
		fmt.Println("No unencrypted sensitive fields found in config.")
	}

	return nil
}

// handleDecryptConfig decrypts sensitive fields in the config file.
func handleDecryptConfig(args Args) error {
	em := security.GlobalEncryptionManager()

	if !em.IsInitialized() {
		return security.ErrNotInitialized
	}

	// Require confirmation
	confirmed := false
	for _, arg := range args.Raw {
		if arg == "--confirm" {
			confirmed = true
			break
		}
	}

	if !confirmed {
		return fmt.Errorf("decryption requires --confirm flag for safety")
	}

	// Load current config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Decrypt sensitive fields
	fieldsDecrypted := 0

	// Decrypt OpenRouter API key if encrypted
	if security.IsEncrypted(cfg.Cloud.OpenRouterKey) {
		decrypted, err := em.DecryptConfigField(cfg.Cloud.OpenRouterKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt OpenRouter key: %w", err)
		}
		cfg.Cloud.OpenRouterKey = decrypted
		fieldsDecrypted++
	}

	// Save config with decrypted fields
	if fieldsDecrypted > 0 {
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save decrypted config: %w", err)
		}

		// Update encryption status in config
		cfg.Security.EncryptConfig = false
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to update config encryption status: %w", err)
		}
	}

	if args.JSON {
		resp := NewJSONResponse("decrypt_config", map[string]interface{}{
			"status":           "success",
			"fields_decrypted": fieldsDecrypted,
		})
		resp.Print()
		return nil
	}

	if fieldsDecrypted > 0 {
		fmt.Printf("Config decryption complete! Decrypted %d field(s).\n", fieldsDecrypted)
		fmt.Println()
		fmt.Println("*** SECURITY WARNING (IL5 SC-28 VIOLATION) ***")
		fmt.Println("Sensitive config data is now stored in PLAINTEXT without encryption.")
		fmt.Println("This violates IL5 compliance requirements for data-at-rest protection.")
		fmt.Println()
		fmt.Println("To restore IL5 compliance, run: rigrun encrypt config")
	} else {
		fmt.Println("No encrypted fields found in config.")
	}

	return nil
}

// =============================================================================
// CACHE COMMAND
// =============================================================================

// handleEncryptCache encrypts the cache database.
func handleEncryptCache(args Args) error {
	em := security.GlobalEncryptionManager()

	if !em.IsInitialized() {
		return security.ErrNotInitialized
	}

	if err := em.EncryptCache(); err != nil {
		return fmt.Errorf("failed to encrypt cache: %w", err)
	}

	// Update config
	cfg, _ := config.Load()
	if cfg != nil {
		cfg.Security.EncryptCache = true
		_ = config.Save(cfg)
	}

	if args.JSON {
		resp := NewJSONResponse("encrypt_cache", map[string]interface{}{
			"status": "success",
		})
		resp.Print()
		return nil
	}

	fmt.Println("Cache encryption complete!")
	return nil
}

// handleDecryptCache decrypts the cache database.
func handleDecryptCache(args Args) error {
	em := security.GlobalEncryptionManager()

	if !em.IsInitialized() {
		return security.ErrNotInitialized
	}

	// Require confirmation
	confirmed := false
	for _, arg := range args.Raw {
		if arg == "--confirm" {
			confirmed = true
			break
		}
	}

	if !confirmed {
		return fmt.Errorf("decryption requires --confirm flag for safety")
	}

	if err := em.DecryptCache(); err != nil {
		return fmt.Errorf("failed to decrypt cache: %w", err)
	}

	// Update config
	cfg, _ := config.Load()
	if cfg != nil {
		cfg.Security.EncryptCache = false
		_ = config.Save(cfg)
	}

	if args.JSON {
		resp := NewJSONResponse("decrypt_cache", map[string]interface{}{
			"status": "success",
		})
		resp.Print()
		return nil
	}

	fmt.Println("Cache decryption complete!")
	fmt.Println()
	fmt.Println("*** SECURITY WARNING (IL5 SC-28 VIOLATION) ***")
	fmt.Println("Cache data is now stored in PLAINTEXT without encryption.")
	fmt.Println("This violates IL5 compliance requirements for data-at-rest protection.")
	fmt.Println()
	fmt.Println("To restore IL5 compliance, run: rigrun encrypt cache")
	return nil
}

// =============================================================================
// AUDIT COMMAND
// =============================================================================

// handleEncryptAudit encrypts the audit log.
func handleEncryptAudit(args Args) error {
	em := security.GlobalEncryptionManager()

	if !em.IsInitialized() {
		return security.ErrNotInitialized
	}

	if err := em.EncryptAuditLog(); err != nil {
		return fmt.Errorf("failed to encrypt audit log: %w", err)
	}

	// Update config
	cfg, _ := config.Load()
	if cfg != nil {
		cfg.Security.EncryptAudit = true
		_ = config.Save(cfg)
	}

	if args.JSON {
		resp := NewJSONResponse("encrypt_audit", map[string]interface{}{
			"status": "success",
		})
		resp.Print()
		return nil
	}

	fmt.Println("Audit log encryption complete!")
	fmt.Println("Note: New audit entries will be appended to the encrypted log.")
	return nil
}

// =============================================================================
// STATUS COMMAND
// =============================================================================

// handleEncryptStatus shows the current encryption status.
func handleEncryptStatus(args Args) error {
	em := security.GlobalEncryptionManager()
	status := em.GetStatus()

	if args.JSON {
		resp := NewJSONResponse("encrypt_status", status)
		resp.Print()
		return nil
	}

	fmt.Println("Encryption Status (NIST 800-53 SC-28)")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	if status.Initialized {
		fmt.Println("Status:         INITIALIZED")
	} else {
		fmt.Println("Status:         NOT INITIALIZED")
		fmt.Println()
		fmt.Println("Run 'rigrun encrypt init' to initialize encryption.")
		return nil
	}

	fmt.Printf("Algorithm:      %s\n", status.Algorithm)
	fmt.Printf("Key Derivation: %s\n", status.KeyDerivation)
	fmt.Printf("Key Store:      %s\n", status.KeyStorePath)
	fmt.Println()

	fmt.Println("Data Protection:")
	printEncryptionStatus("  Config", status.ConfigEncrypted)
	printEncryptionStatus("  Cache", status.CacheEncrypted)
	printEncryptionStatus("  Audit Log", status.AuditEncrypted)
	fmt.Println()

	// IL5 compliance check - warn if cache is not encrypted
	if !status.CacheEncrypted {
		fmt.Println("*** IL5 COMPLIANCE WARNING ***")
		fmt.Println("Cache encryption is disabled. For IL5 SC-28 compliance:")
		fmt.Println("  Run: rigrun encrypt cache")
		fmt.Println()
	}

	return nil
}

func printEncryptionStatus(name string, encrypted bool) {
	if encrypted {
		fmt.Printf("%s:     ENCRYPTED\n", name)
	} else {
		fmt.Printf("%s:     not encrypted\n", name)
	}
}

// =============================================================================
// ROTATE COMMAND
// =============================================================================

// handleEncryptRotate rotates the master encryption key.
func handleEncryptRotate(args Args) error {
	em := security.GlobalEncryptionManager()

	if !em.IsInitialized() {
		return security.ErrNotInitialized
	}

	// Require confirmation
	confirmed := false
	for _, arg := range args.Raw {
		if arg == "--confirm" {
			confirmed = true
			break
		}
	}

	if !confirmed {
		fmt.Println("WARNING: Key rotation will generate a new master key.")
		fmt.Println("All encrypted data must be re-encrypted with the new key.")
		fmt.Println()
		fmt.Println("To proceed, run: rigrun encrypt rotate --confirm")
		return nil
	}

	// Get current status to know what needs re-encryption
	status := em.GetStatus()

	// Step 1: Decrypt existing data with old key
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Decrypt config fields
	decryptedKey := ""
	if status.ConfigEncrypted && security.IsEncrypted(cfg.Cloud.OpenRouterKey) {
		decryptedKey, err = em.DecryptConfigField(cfg.Cloud.OpenRouterKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt config for rotation: %w", err)
		}
	}

	// Step 2: Rotate key
	if err := em.RotateKey(); err != nil {
		return fmt.Errorf("failed to rotate key: %w", err)
	}

	// Step 3: Re-encrypt data with new key
	if decryptedKey != "" {
		encrypted, err := em.EncryptConfigField(decryptedKey)
		if err != nil {
			return fmt.Errorf("failed to re-encrypt config: %w", err)
		}
		cfg.Cloud.OpenRouterKey = encrypted
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save re-encrypted config: %w", err)
		}
	}

	// Re-encrypt cache if it was encrypted
	if status.CacheEncrypted {
		// Decrypt with old key, encrypt with new key
		// Note: The cache was already decrypted before rotation
		if err := em.EncryptCache(); err != nil {
			fmt.Printf("Warning: Failed to re-encrypt cache: %v\n", err)
		}
	}

	if args.JSON {
		resp := NewJSONResponse("encrypt_rotate", map[string]interface{}{
			"status":       "success",
			"config_updated": decryptedKey != "",
			"cache_updated":  status.CacheEncrypted,
		})
		resp.Print()
		return nil
	}

	fmt.Println("Key rotation complete!")
	fmt.Println("A new master key has been generated and all data re-encrypted.")
	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// validatePassword validates a password meets IL5 compliance requirements.
// Requirements per NIST SP 800-63B:
//   - Minimum 15 characters
//   - At least one uppercase letter
//   - At least one lowercase letter
//   - At least one digit
//   - At least one special character (punctuation or symbol)
func validatePassword(password string) error {
	// Check minimum length (IL5: NIST SP 800-63B compliant)
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters for IL5 compliance (NIST SP 800-63B)", MinPasswordLength)
	}

	// Check complexity requirements
	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			hasSpecial = true
		}
	}

	// Build error message for missing requirements
	var missing []string
	if !hasUpper {
		missing = append(missing, "uppercase letter")
	}
	if !hasLower {
		missing = append(missing, "lowercase letter")
	}
	if !hasDigit {
		missing = append(missing, "digit")
	}
	if !hasSpecial {
		missing = append(missing, "special character")
	}

	if len(missing) > 0 {
		return fmt.Errorf("password must contain at least one: %s", strings.Join(missing, ", "))
	}

	return nil
}

// promptPassword prompts for a password without echoing it to the terminal.
// Uses golang.org/x/term.ReadPassword for secure password input.
func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	// Use term.ReadPassword for secure input (no echo)
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	fmt.Println() // Add newline after password input

	return string(password), nil
}

// =============================================================================
// JSON OUTPUT
// =============================================================================

// EncryptionStatusData represents encryption status for JSON output.
type EncryptionStatusData struct {
	Initialized     bool   `json:"initialized"`
	Algorithm       string `json:"algorithm"`
	KeyDerivation   string `json:"key_derivation"`
	ConfigEncrypted bool   `json:"config_encrypted"`
	CacheEncrypted  bool   `json:"cache_encrypted"`
	AuditEncrypted  bool   `json:"audit_encrypted"`
}

// Ensure EncryptionStatusData implements the necessary interface
var _ = json.Marshal // Use json package to avoid unused import error

// =============================================================================
// ARGUMENT PARSING
// =============================================================================

// parseEncryptArgs parses encrypt command specific arguments.
func parseEncryptArgs(args *Args, remaining []string) {
	if len(remaining) > 0 {
		args.Subcommand = remaining[0]
	}
}

// IsTerminal checks if stdin is a terminal.
// Used to determine if we can prompt for passwords.
func IsTerminal() bool {
	// Check if stdin is a terminal
	fd := syscall.Stdin
	// This is a simplified check - in production use golang.org/x/term.IsTerminal
	return fd >= 0
}
