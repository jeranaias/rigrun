// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package config provides unified configuration loading and management for rigrun.
//
// Supports both TOML and JSON configuration formats, with sensible defaults,
// environment variable overrides, and validation.
//
// Configuration file locations (in order of precedence):
//   - ~/.rigrun/config.toml
//   - ~/.rigrun/config.json
//   - Built-in defaults
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// CONFIG STRUCTURES
// =============================================================================

// Config represents the complete rigrun configuration.
type Config struct {
	// General settings
	Version      string `toml:"version" json:"version"`
	DefaultModel string `toml:"default_model" json:"default_model"`

	// Routing configuration
	Routing RoutingConfig `toml:"routing" json:"routing"`

	// Local (Ollama) configuration
	Local LocalConfig `toml:"local" json:"local"`

	// Cloud (OpenRouter) configuration
	Cloud CloudConfig `toml:"cloud" json:"cloud"`

	// Security configuration
	Security SecurityConfig `toml:"security" json:"security"`

	// Consent configuration (IL5 AC-8 compliance)
	Consent ConsentConfig `toml:"consent" json:"consent"`

	// Cache configuration
	Cache CacheConfig `toml:"cache" json:"cache"`

	// UI configuration
	UI UIConfig `toml:"ui" json:"ui"`
}

// RoutingConfig contains query routing configuration.
type RoutingConfig struct {
	// DefaultMode is the default routing mode: "auto", "cloud", "local", "hybrid"
	// "auto" (default): Let OpenRouter decide the optimal model/route
	// "local": Force local Ollama only
	// "cloud": Force cloud only
	// "hybrid": Alias for "auto" (deprecated, kept for backwards compatibility)
	DefaultMode string `toml:"default_mode" json:"default_mode"`
	// MaxTier is the maximum tier to route to (e.g., "opus", "sonnet", "local")
	MaxTier string `toml:"max_tier" json:"max_tier"`
	// ParanoidMode blocks all cloud requests when true
	ParanoidMode bool `toml:"paranoid_mode" json:"paranoid_mode"`
	// OfflineMode blocks ALL network except localhost Ollama (IL5 SC-7 compliance)
	// When true: no cloud, no web fetch, no telemetry - only localhost:11434 allowed
	OfflineMode bool `toml:"offline_mode" json:"offline_mode"`

	// Auto mode configuration (used when DefaultMode is "auto" or "hybrid")
	// AutoPreferLocal hints to prefer local models when possible (cost savings)
	AutoPreferLocal bool `toml:"auto_prefer_local" json:"auto_prefer_local"`
	// AutoMaxCost is the maximum cost per query in cents for auto mode (0 = unlimited)
	AutoMaxCost float64 `toml:"auto_max_cost" json:"auto_max_cost"`
	// AutoFallback specifies what to do if OpenRouter is unavailable: "local" or "error"
	AutoFallback string `toml:"auto_fallback" json:"auto_fallback"`
}

// LocalConfig contains local Ollama configuration.
type LocalConfig struct {
	// OllamaURL is the URL of the Ollama server
	OllamaURL string `toml:"ollama_url" json:"ollama_url"`
	// OllamaModel is the default model to use with Ollama
	OllamaModel string `toml:"ollama_model" json:"ollama_model"`
}

// CloudConfig contains cloud provider (OpenRouter) configuration.
type CloudConfig struct {
	// OpenRouterKey is the OpenRouter API key
	OpenRouterKey string `toml:"openrouter_key" json:"openrouter_key"`
	// DefaultModel is the default cloud model to use
	DefaultModel string `toml:"default_model" json:"default_model"`
}

// SecurityConfig contains security-related configuration.
type SecurityConfig struct {
	// SessionTimeoutSecs is the session timeout in seconds.
	// IL5 AC-12 compliance: Valid range is 900-1800 seconds (15-30 minutes) per DoD STIG.
	// Values outside this range will be clamped to valid bounds.
	SessionTimeoutSecs int `toml:"session_timeout_secs" json:"session_timeout_secs"`
	// AuditEnabled enables audit logging
	AuditEnabled bool `toml:"audit_enabled" json:"audit_enabled"`
	// AuditLogPath is the path to the audit log file (empty = default ~/.rigrun/audit.log)
	AuditLogPath string `toml:"audit_log_path" json:"audit_log_path"`
	// BannerEnabled enables classification banner display
	BannerEnabled bool `toml:"banner_enabled" json:"banner_enabled"`
	// Classification is the default classification marking (e.g., "UNCLASSIFIED")
	Classification string `toml:"classification" json:"classification"`
	// ConsentRequired requires user to acknowledge consent banner (AC-8).
	// Default: true (required for IL5 compliance)
	ConsentRequired bool `toml:"consent_required" json:"consent_required"`

	// ==========================================================================
	// NIST 800-53 AC-5/AC-6: Separation of Duties & Least Privilege
	// ==========================================================================
	// UserID is the identifier for the current user (used for RBAC).
	// If not set, will be derived from system username or environment.
	UserID string `toml:"user_id" json:"user_id"`

	// ==========================================================================
	// NIST 800-53 AC-7: Unsuccessful Logon Attempts
	// ==========================================================================
	// MaxLoginAttempts is the maximum number of failed login attempts before lockout.
	// AC-7(a) requires a limit on consecutive invalid logon attempts.
	// Default: 3 (DoD STIG recommendation for IL5)
	MaxLoginAttempts int `toml:"max_login_attempts" json:"max_login_attempts"`
	// LockoutDurationMinutes is how long an account remains locked after max attempts.
	// AC-7(b) requires automatic lockout until released by administrator or timeout.
	// Default: 15 (DoD STIG recommendation for IL5)
	LockoutDurationMinutes int `toml:"lockout_duration_minutes" json:"lockout_duration_minutes"`
	// LockoutDurationSecs is the lockout duration in seconds (for CM-6 baseline).
	// Derived from LockoutDurationMinutes * 60. Default: 900 (15 minutes)
	LockoutDurationSecs int `toml:"lockout_duration_secs" json:"lockout_duration_secs"`
	// LockoutEnabled enables/disables the AC-7 lockout mechanism.
	// When disabled, failed attempts are still logged but lockout is not enforced.
	LockoutEnabled bool `toml:"lockout_enabled" json:"lockout_enabled"`

	// ==========================================================================
	// NIST 800-53 IA-2: Identification and Authentication
	// ==========================================================================
	// AuthSessionDurationHours is how long an authenticated session remains valid.
	// Default: 8 hours (standard workday)
	AuthSessionDurationHours int `toml:"auth_session_duration_hours" json:"auth_session_duration_hours"`
	// MFAEnabled enables multi-factor authentication requirement (IA-2(1) placeholder).
	// When true, MFA verification is required after initial authentication.
	// NOTE: This is a placeholder for future MFA implementation.
	MFAEnabled bool `toml:"mfa_enabled" json:"mfa_enabled"`

	// ==========================================================================
	// NIST 800-53 AU-5: Response to Audit Processing Failures
	// ==========================================================================
	// HaltOnAuditFailure stops operations if audit logging fails (AU-5).
	// When true, operations will fail if audit logs cannot be written.
	HaltOnAuditFailure bool `toml:"halt_on_audit_failure" json:"halt_on_audit_failure"`
	// AuditCapacityWarningMB is the threshold for audit capacity warning (AU-5).
	// Default: 0 (auto-calculate as 80% of available storage)
	AuditCapacityWarningMB int64 `toml:"audit_capacity_warning_mb" json:"audit_capacity_warning_mb"`
	// AuditCapacityCriticalMB is the threshold for audit capacity critical alert (AU-5).
	// Default: 0 (auto-calculate as 90% of available storage)
	AuditCapacityCriticalMB int64 `toml:"audit_capacity_critical_mb" json:"audit_capacity_critical_mb"`

	// ==========================================================================
	// NIST 800-53 SI-7: Software, Firmware, and Information Integrity
	// ==========================================================================
	// IntegrityCheckOnStartup enables integrity verification at application startup (SI-7).
	// When true, rigrun will verify binary and config checksums before starting.
	IntegrityCheckOnStartup bool `toml:"integrity_check_on_startup" json:"integrity_check_on_startup"`
	// IntegrityChecksumFile is the path to the checksums file (SI-7).
	// Default: ~/.rigrun/checksums.json
	IntegrityChecksumFile string `toml:"integrity_checksum_file" json:"integrity_checksum_file"`

	// ==========================================================================
	// NIST 800-53 SC-28: Protection of Information at Rest
	// ==========================================================================
	// EncryptionEnabled indicates whether encryption is initialized and active.
	// When true, sensitive data at rest is encrypted using AES-256-GCM.
	EncryptionEnabled bool `toml:"encryption_enabled" json:"encryption_enabled"`
	// EncryptConfig indicates whether config sensitive fields are encrypted.
	// Encrypted values use ENC: prefix with base64-encoded ciphertext.
	EncryptConfig bool `toml:"encrypt_config" json:"encrypt_config"`
	// EncryptCache indicates whether the cache database is encrypted.
	EncryptCache bool `toml:"encrypt_cache" json:"encrypt_cache"`
	// EncryptAudit indicates whether audit logs are encrypted (optional).
	// For highly sensitive deployments, audit logs can also be encrypted.
	EncryptAudit bool `toml:"encrypt_audit" json:"encrypt_audit"`

	// ==========================================================================
	// NIST 800-53 SC-13: Cryptographic Protection
	// ==========================================================================
	// FIPSMode enables FIPS 140-2/3 compliant cryptographic operations (SC-13).
	// When true, only FIPS-approved algorithms are used for cryptographic operations.
	FIPSMode bool `toml:"fips_mode" json:"fips_mode"`

	// ==========================================================================
	// NIST 800-53 SC-13: Cryptographic Algorithms
	// ==========================================================================
	// AllowedCiphers specifies the allowed cipher suites for encryption (SC-13).
	// Default: "AES-256-GCM" (FIPS 140-2 validated)
	AllowedCiphers string `toml:"allowed_ciphers" json:"allowed_ciphers"`

	// ==========================================================================
	// NIST 800-53 SC-17: PKI Certificates
	// ==========================================================================
	// TLSMinVersion is the minimum TLS version for HTTPS connections (SC-17).
	// Valid values: "1.2" or "1.3". Default: "1.2" (FIPS requires TLS 1.2+).
	TLSMinVersion string `toml:"tls_min_version" json:"tls_min_version"`
	// CertificatePinning enables certificate pinning for HTTPS connections (SC-17).
	// When true, certificates must match pinned fingerprints for known hosts.
	CertificatePinning bool `toml:"certificate_pinning" json:"certificate_pinning"`
	// PinnedCertificates maps hostnames to SHA-256 certificate fingerprints (SC-17).
	// Format: {"openrouter.ai": "abc123..."}
	PinnedCertificates map[string]string `toml:"pinned_certificates" json:"pinned_certificates"`

	// ==========================================================================
	// NIST 800-53 IR-6: Incident Reporting
	// ==========================================================================
	// IncidentWebhook is an optional URL for external incident reporting.
	// When configured, new incidents are also sent to this webhook.
	IncidentWebhook string `toml:"incident_webhook" json:"incident_webhook"`

	// ==========================================================================
	// NIST 800-53 IR-9: Information Spillage Response
	// ==========================================================================
	// SpillageDetection enables scanning for classification markers in content.
	// When true, content is scanned for potential spillage of classified information.
	SpillageDetection bool `toml:"spillage_detection" json:"spillage_detection"`
	// SpillageAction defines the action taken when spillage is detected.
	// Valid values: "warn" (log only), "block" (stop operation), "sanitize" (auto-redact).
	// Default: "warn"
	SpillageAction string `toml:"spillage_action" json:"spillage_action"`

	// ==========================================================================
	// NIST 800-53 AU-9: Protection of Audit Information
	// ==========================================================================
	// PolicyKey is the HMAC key for boundary policy file integrity verification.
	// SECURITY: Must be configured via RIGRUN_POLICY_KEY env var or this field.
	// No silent fallback to hardcoded values - will error if not configured.
	PolicyKey string `toml:"policy_key" json:"policy_key"`
}

// CacheConfig contains cache configuration.
type CacheConfig struct {
	// Enabled controls whether caching is active
	Enabled bool `toml:"enabled" json:"enabled"`
	// TTLHours is the time-to-live for cache entries in hours
	TTLHours int `toml:"ttl_hours" json:"ttl_hours"`
	// MaxSize is the maximum number of cache entries
	MaxSize int `toml:"max_size" json:"max_size"`
	// SemanticEnabled enables semantic (embedding-based) caching
	SemanticEnabled bool `toml:"semantic_enabled" json:"semantic_enabled"`
	// SemanticThreshold is the minimum similarity score for semantic cache hits (0.0-1.0)
	SemanticThreshold float64 `toml:"semantic_threshold" json:"semantic_threshold"`
}

// UIConfig contains UI configuration.
type UIConfig struct {
	// Theme is the UI theme: "dark", "light", "auto"
	Theme string `toml:"theme" json:"theme"`
	// ShowCost displays cost information in the UI
	ShowCost bool `toml:"show_cost" json:"show_cost"`
	// ShowTokens displays token counts in the UI
	ShowTokens bool `toml:"show_tokens" json:"show_tokens"`
	// CompactMode uses a more compact UI layout
	CompactMode bool `toml:"compact_mode" json:"compact_mode"`
	// VimMode enables vim-style modal editing
	VimMode bool `toml:"vim_mode" json:"vim_mode"`
	// TutorialCompleted indicates whether the user has completed the tutorial
	TutorialCompleted bool `toml:"tutorial_completed" json:"tutorial_completed"`
	// TutorialStep tracks the current tutorial step (0-4, or 5 if completed)
	TutorialStep int `toml:"tutorial_step" json:"tutorial_step"`
}

// ConsentConfig contains DoD consent/system use notification settings.
// This supports IL5 compliance with NIST 800-53 AC-8 (System Use Notification).
type ConsentConfig struct {
	// Required indicates whether consent is required before use (IL5 mode)
	Required bool `toml:"required" json:"required"`
	// Accepted indicates whether the user has accepted the consent banner
	Accepted bool `toml:"accepted" json:"accepted"`
	// AcceptedAt is the timestamp when consent was accepted
	AcceptedAt time.Time `toml:"accepted_at" json:"accepted_at,omitempty"`
	// AcceptedBy is the OS username who accepted consent
	AcceptedBy string `toml:"accepted_by" json:"accepted_by,omitempty"`
	// BannerVersion is the version of the consent banner that was accepted
	BannerVersion string `toml:"banner_version" json:"banner_version,omitempty"`
}

// =============================================================================
// DEFAULT CONFIGURATION
// =============================================================================

// Default returns a Config with sensible default values.
func Default() *Config {
	return &Config{
		Version:      "1.0.0",
		DefaultModel: "qwen2.5-coder:14b",

		Routing: RoutingConfig{
			DefaultMode:     "auto",
			MaxTier:         "opus",
			ParanoidMode:    false,
			AutoPreferLocal: false,
			AutoMaxCost:     0, // unlimited
			AutoFallback:    "local",
		},

		Local: LocalConfig{
			OllamaURL:   "http://127.0.0.1:11434",
			OllamaModel: "qwen2.5-coder:14b",
		},

		Cloud: CloudConfig{
			OpenRouterKey: "",
			DefaultModel:  "anthropic/claude-3.5-sonnet",
		},

		Security: SecurityConfig{
			SessionTimeoutSecs:       1800,     // 30 minutes - IL5 AC-12 max per DoD STIG (range: 15-30 min)
			AuditEnabled:             true,
			BannerEnabled:            false,
			Classification:           "UNCLASSIFIED",
			MaxLoginAttempts:         3,        // AC-7: Default 3 attempts before lockout
			LockoutDurationMinutes:   15,       // AC-7: Default 15 minute lockout
			LockoutEnabled:           true,     // AC-7: Lockout enabled by default for IL5
			AuthSessionDurationHours: 8,        // IA-2: 8 hour session duration
			MFAEnabled:               false,    // IA-2(1): MFA disabled (placeholder)
			SpillageDetection:        true,     // IR-9: Spillage detection enabled by default
			SpillageAction:           "warn",    // IR-9: Default to warning only
			TLSMinVersion:            "TLS1.2", // SC-17: Minimum TLS 1.2 for FIPS compliance
			// SC-28: Protection of Information at Rest - SECURITY: Encryption enabled by default for IL5
			EncryptionEnabled:        true,  // SC-28: Encryption enabled by default for API keys
			EncryptConfig:            true,  // SC-28: Encrypt sensitive config fields by default
			EncryptCache:             true,  // SC-28: Cache encryption ENABLED by default for IL5 compliance
			EncryptAudit:             false, // SC-28: Audit encryption optional (for highly sensitive deployments)
		},

		Consent: ConsentConfig{
			Required:      true, // IL5 compliance: consent required by default; disable with 'rigrun consent not-required'
			Accepted:      false,
			AcceptedAt:    time.Time{},
			AcceptedBy:    "",
			BannerVersion: "",
		},

		Cache: CacheConfig{
			Enabled:           true,
			TTLHours:          24,
			MaxSize:           10000,
			SemanticEnabled:   true,
			SemanticThreshold: 0.92,
		},

		UI: UIConfig{
			Theme:             "dark",
			ShowCost:          true,
			ShowTokens:        true,
			CompactMode:       false,
			VimMode:           false, // Vim mode disabled by default
			TutorialCompleted: false,
		},
	}
}

// =============================================================================
// CONFIG PATH HELPERS
// =============================================================================

// ConfigDir returns the rigrun configuration directory path.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".rigrun"), nil
}

// ConfigPathTOML returns the path to the TOML config file.
func ConfigPathTOML() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// ConfigPathJSON returns the path to the JSON config file.
func ConfigPathJSON() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// EnsureConfigDir ensures the config directory exists.
func EnsureConfigDir() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0755)
}

// ensureSecurePermissions checks and fixes permissions on config files.
// SECURITY: Config files should be 0600 (owner read/write only) to protect API keys.
func ensureSecurePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err // File doesn't exist or not accessible
	}

	// Check current permissions
	mode := info.Mode().Perm()

	// If permissions are too permissive (anything other than 0600), fix them
	if mode != 0600 {
		if err := os.Chmod(path, 0600); err != nil {
			return fmt.Errorf("failed to fix insecure permissions (was %o): %w", mode, err)
		}
	}

	return nil
}

// =============================================================================
// LOAD FUNCTIONS
// =============================================================================

// Load loads configuration from the config file(s).
// Tries TOML first, then JSON, and falls back to defaults.
// Environment overrides are applied last.
// CONFIG: Comprehensive validation ensures safe configuration
func Load() (*Config, error) {
	cfg := Default()
	var loadErr error

	// Try TOML first
	tomlPath, err := ConfigPathTOML()
	if err == nil {
		if _, statErr := os.Stat(tomlPath); statErr == nil {
			if err := LoadTOML(cfg, tomlPath); err != nil {
				loadErr = fmt.Errorf("failed to load TOML config: %w", err)
			} else {
				cfg.ApplyEnvOverrides()
				// Apply migration, defaults, and validation
				if err := cfg.Migrate(); err != nil {
					return nil, fmt.Errorf("config migration failed: %w", err)
				}
				cfg.SetDefaults()
				if err := cfg.Validate(); err != nil {
					return nil, fmt.Errorf("invalid config: %w", err)
				}
				return cfg, nil
			}
		}
	}

	// Try JSON as fallback
	jsonPath, err := ConfigPathJSON()
	if err == nil {
		if _, statErr := os.Stat(jsonPath); statErr == nil {
			if err := LoadJSON(cfg, jsonPath); err != nil {
				loadErr = fmt.Errorf("failed to load JSON config: %w", err)
			} else {
				cfg.ApplyEnvOverrides()
				// Apply migration, defaults, and validation
				if err := cfg.Migrate(); err != nil {
					return nil, fmt.Errorf("config migration failed: %w", err)
				}
				cfg.SetDefaults()
				if err := cfg.Validate(); err != nil {
					return nil, fmt.Errorf("invalid config: %w", err)
				}
				return cfg, nil
			}
		}
	}

	// Apply environment overrides to defaults
	cfg.ApplyEnvOverrides()

	// Apply migration, defaults, and validation for default config
	if err := cfg.Migrate(); err != nil {
		return nil, fmt.Errorf("config migration failed: %w", err)
	}
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Return defaults (with any load error for informational purposes)
	return cfg, loadErr
}

// LoadTOML loads configuration from a TOML file.
// SECURITY: Checks and fixes file permissions on load.
func LoadTOML(cfg *Config, path string) error {
	// SECURITY: Check and fix file permissions if needed
	if err := ensureSecurePermissions(path); err != nil {
		// Log warning but don't fail - permissions might not be fixable on all systems
		fmt.Fprintf(os.Stderr, "Warning: could not ensure secure permissions on %s: %v\n", path, err)
	}

	_, err := toml.DecodeFile(path, cfg)
	if err != nil {
		return fmt.Errorf("failed to decode TOML file: %w", err)
	}
	return fillDefaults(cfg)
}

// LoadJSON loads configuration from a JSON file.
// SECURITY: Checks and fixes file permissions on load.
func LoadJSON(cfg *Config, path string) error {
	// SECURITY: Check and fix file permissions if needed
	if err := ensureSecurePermissions(path); err != nil {
		// Log warning but don't fail - permissions might not be fixable on all systems
		fmt.Fprintf(os.Stderr, "Warning: could not ensure secure permissions on %s: %v\n", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to decode JSON file: %w", err)
	}
	return fillDefaults(cfg)
}

// LoadFromPath loads configuration from a specific file path with full validation.
// CONFIG: Comprehensive validation ensures safe configuration
func LoadFromPath(path string) (*Config, error) {
	cfg := &Config{}

	// Determine file type and load accordingly
	if strings.HasSuffix(path, ".json") {
		if err := LoadJSON(cfg, path); err != nil {
			return nil, fmt.Errorf("failed to load JSON config from %s: %w", path, err)
		}
	} else {
		// Default to TOML
		if err := LoadTOML(cfg, path); err != nil {
			return nil, fmt.Errorf("failed to load TOML config from %s: %w", path, err)
		}
	}

	// Apply environment overrides
	cfg.ApplyEnvOverrides()

	// Apply migration, defaults, and validation
	if err := cfg.Migrate(); err != nil {
		return nil, fmt.Errorf("config migration failed: %w", err)
	}
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// fillDefaults fills in any missing values with defaults.
func fillDefaults(cfg *Config) error {
	defaults := Default()

	// General
	if cfg.Version == "" {
		cfg.Version = defaults.Version
	}
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = defaults.DefaultModel
	}

	// Routing
	if cfg.Routing.DefaultMode == "" {
		cfg.Routing.DefaultMode = defaults.Routing.DefaultMode
	}
	if cfg.Routing.MaxTier == "" {
		cfg.Routing.MaxTier = defaults.Routing.MaxTier
	}
	if cfg.Routing.AutoFallback == "" {
		cfg.Routing.AutoFallback = defaults.Routing.AutoFallback
	}

	// Local
	if cfg.Local.OllamaURL == "" {
		cfg.Local.OllamaURL = defaults.Local.OllamaURL
	}
	if cfg.Local.OllamaModel == "" {
		cfg.Local.OllamaModel = defaults.Local.OllamaModel
	}

	// Cloud
	if cfg.Cloud.DefaultModel == "" {
		cfg.Cloud.DefaultModel = defaults.Cloud.DefaultModel
	}

	// Security
	// IL5 AC-12 compliance: Session timeout cannot be 0 (disabled) or below minimum
	// Valid range: 900-1800 seconds (15-30 minutes)
	if cfg.Security.SessionTimeoutSecs == 0 || cfg.Security.SessionTimeoutSecs < 900 {
		cfg.Security.SessionTimeoutSecs = defaults.Security.SessionTimeoutSecs
	}
	if cfg.Security.Classification == "" {
		cfg.Security.Classification = defaults.Security.Classification
	}

	// Cache
	if cfg.Cache.TTLHours == 0 {
		cfg.Cache.TTLHours = defaults.Cache.TTLHours
	}
	if cfg.Cache.MaxSize == 0 {
		cfg.Cache.MaxSize = defaults.Cache.MaxSize
	}
	if cfg.Cache.SemanticThreshold == 0 {
		cfg.Cache.SemanticThreshold = defaults.Cache.SemanticThreshold
	}

	// UI
	if cfg.UI.Theme == "" {
		cfg.UI.Theme = defaults.UI.Theme
	}

	return nil
}

// =============================================================================
// SAVE FUNCTIONS
// =============================================================================

// Save saves the configuration to the default TOML file.
func Save(cfg *Config) error {
	path, err := ConfigPathTOML()
	if err != nil {
		return err
	}
	return SaveTOML(cfg, path)
}

// SaveTOML saves the configuration to a TOML file.
// SECURITY: Creates config files with 0600 permissions (owner read/write only).
func SaveTOML(cfg *Config, path string) error {
	if err := EnsureConfigDir(); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// SECURITY: Create file with restrictive permissions (0600 = owner read/write only)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	// SECURITY: Ensure permissions are correct even if file already existed
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	// Write header comment
	fmt.Fprintln(file, "# rigrun configuration file")
	fmt.Fprintln(file, "# Generated by rigrun - edit with care")
	fmt.Fprintln(file, "#")
	fmt.Fprintln(file, "# Documentation: https://github.com/jeranaias/rigrun")
	fmt.Fprintln(file, "")

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}

// SaveJSON saves the configuration to a JSON file.
// SECURITY: Creates config files with 0600 permissions (owner read/write only).
// RELIABILITY: Atomic write with fsync prevents data loss on crash
func SaveJSON(cfg *Config, path string) error {
	if err := EnsureConfigDir(); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	// RELIABILITY: Atomic write with fsync prevents data loss on crash
	// SECURITY: Write with restrictive permissions (0600 = owner read/write only)
	if err := util.AtomicWriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// =============================================================================
// VALIDATION
// =============================================================================

// CONFIG: Comprehensive validation ensures safe configuration

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateErrors is a collection of validation errors.
type ValidateErrors []ValidationError

func (e ValidateErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// Validate validates the configuration and returns any errors.
// CONFIG: Comprehensive validation ensures safe configuration
func (c *Config) Validate() error {
	var errs ValidateErrors

	// ==========================================================================
	// Routing Settings Validation
	// ==========================================================================

	// Validate routing mode
	validModes := map[string]bool{"auto": true, "cloud": true, "local": true, "hybrid": true}
	if !validModes[strings.ToLower(c.Routing.DefaultMode)] {
		errs = append(errs, ValidationError{
			Field:   "routing.default_mode",
			Message: fmt.Sprintf("invalid mode '%s', must be one of: auto, cloud, local, hybrid", c.Routing.DefaultMode),
		})
	}

	// Validate auto_fallback if specified
	if c.Routing.AutoFallback != "" {
		validFallbacks := map[string]bool{"local": true, "error": true}
		if !validFallbacks[strings.ToLower(c.Routing.AutoFallback)] {
			errs = append(errs, ValidationError{
				Field:   "routing.auto_fallback",
				Message: fmt.Sprintf("invalid fallback '%s', must be one of: local, error", c.Routing.AutoFallback),
			})
		}
	}

	// Validate auto_max_cost (max cost in cents cannot be negative)
	if c.Routing.AutoMaxCost < 0 {
		errs = append(errs, ValidationError{
			Field:   "routing.auto_max_cost",
			Message: "max_cost_cents cannot be negative",
		})
	}

	// Validate max tier
	validTiers := map[string]bool{
		"cache": true, "local": true, "cloud": true,
		"haiku": true, "sonnet": true, "opus": true, "gpt-4o": true,
	}
	if !validTiers[strings.ToLower(c.Routing.MaxTier)] {
		errs = append(errs, ValidationError{
			Field:   "routing.max_tier",
			Message: fmt.Sprintf("invalid tier '%s', must be one of: cache, local, cloud, haiku, sonnet, opus, gpt-4o", c.Routing.MaxTier),
		})
	}

	// Validate Ollama URL
	if c.Local.OllamaURL != "" {
		if _, err := url.Parse(c.Local.OllamaURL); err != nil {
			errs = append(errs, ValidationError{
				Field:   "local.ollama_url",
				Message: fmt.Sprintf("invalid URL: %v", err),
			})
		}
	}

	// ==========================================================================
	// Security Settings Validation
	// ==========================================================================

	// Validate session timeout
	// SECURITY: IL5 AC-12 compliance requires session timeout between 15-30 minutes.
	// Per DoD STIG, sessions must timeout within this window to prevent unauthorized
	// access from unattended terminals. Values outside this range are rejected.
	if c.Security.SessionTimeoutSecs < 900 {
		errs = append(errs, ValidationError{
			Field:   "security.session_timeout_secs",
			Message: fmt.Sprintf("must be at least 900 seconds (15 min) per IL5 AC-12 compliance, got %d", c.Security.SessionTimeoutSecs),
		})
	}
	if c.Security.SessionTimeoutSecs > 1800 {
		errs = append(errs, ValidationError{
			Field:   "security.session_timeout_secs",
			Message: fmt.Sprintf("must be at most 1800 seconds (30 min) per IL5 AC-12 compliance, got %d", c.Security.SessionTimeoutSecs),
		})
	}

	// Validate max login attempts (AC-7 compliance)
	// SECURITY: Must be between 3-10 attempts per DoD STIG guidelines
	if c.Security.MaxLoginAttempts < 3 || c.Security.MaxLoginAttempts > 10 {
		errs = append(errs, ValidationError{
			Field:   "security.max_login_attempts",
			Message: fmt.Sprintf("max_login_attempts must be 3-10, got %d", c.Security.MaxLoginAttempts),
		})
	}

	// Validate lockout duration (AC-7 compliance)
	// SECURITY: Lockout duration must be at least 1 minute, max 60 minutes
	if c.Security.LockoutDurationMinutes < 1 || c.Security.LockoutDurationMinutes > 60 {
		errs = append(errs, ValidationError{
			Field:   "security.lockout_duration_minutes",
			Message: fmt.Sprintf("lockout_duration_minutes must be 1-60, got %d", c.Security.LockoutDurationMinutes),
		})
	}

	// Validate auth session duration (IA-2 compliance)
	// SECURITY: Session duration must be reasonable (1-24 hours)
	if c.Security.AuthSessionDurationHours < 1 || c.Security.AuthSessionDurationHours > 24 {
		errs = append(errs, ValidationError{
			Field:   "security.auth_session_duration_hours",
			Message: fmt.Sprintf("auth_session_duration_hours must be 1-24, got %d", c.Security.AuthSessionDurationHours),
		})
	}

	// Validate TLS minimum version
	// SECURITY: SC-17 PKI Certificates - Only TLS 1.2 and 1.3 are approved for use.
	// TLS 1.0 and 1.1 have known vulnerabilities and are prohibited per NIST SP 800-52 Rev 2.
	// FIPS 140-2/3 compliance also requires TLS 1.2 or higher for cryptographic modules.
	validTLSVersions := map[string]bool{
		"1.2":    true,
		"1.3":    true,
		"TLS1.2": true,
		"TLS1.3": true,
	}
	if !validTLSVersions[c.Security.TLSMinVersion] {
		errs = append(errs, ValidationError{
			Field:   "security.tls_min_version",
			Message: fmt.Sprintf("tls_min_version must be TLS1.2 or TLS1.3, got %s", c.Security.TLSMinVersion),
		})
	}

	// Validate spillage action (IR-9 compliance)
	if c.Security.SpillageAction != "" {
		validSpillageActions := map[string]bool{"warn": true, "block": true, "sanitize": true}
		if !validSpillageActions[strings.ToLower(c.Security.SpillageAction)] {
			errs = append(errs, ValidationError{
				Field:   "security.spillage_action",
				Message: fmt.Sprintf("spillage_action must be warn, block, or sanitize, got %s", c.Security.SpillageAction),
			})
		}
	}

	// Validate classification if provided
	if c.Security.Classification != "" {
		validClassifications := map[string]bool{
			"UNCLASSIFIED": true, "CUI": true, "CONFIDENTIAL": true,
			"SECRET": true, "TOP SECRET": true,
		}
		if !validClassifications[strings.ToUpper(c.Security.Classification)] {
			errs = append(errs, ValidationError{
				Field:   "security.classification",
				Message: fmt.Sprintf("invalid classification '%s'", c.Security.Classification),
			})
		}
	}

	// ==========================================================================
	// Cache Settings Validation
	// ==========================================================================

	// Validate cache TTL
	if c.Cache.TTLHours < 0 {
		errs = append(errs, ValidationError{
			Field:   "cache.ttl_hours",
			Message: "must be non-negative",
		})
	}

	// Validate cache max size (max_entries must be 0-100000)
	if c.Cache.MaxSize < 0 || c.Cache.MaxSize > 100000 {
		errs = append(errs, ValidationError{
			Field:   "cache.max_size",
			Message: fmt.Sprintf("cache max_entries must be 0-100000, got %d", c.Cache.MaxSize),
		})
	}

	// Validate semantic threshold
	if c.Cache.SemanticThreshold < 0 || c.Cache.SemanticThreshold > 1 {
		errs = append(errs, ValidationError{
			Field:   "cache.semantic_threshold",
			Message: "must be between 0.0 and 1.0",
		})
	}

	// ==========================================================================
	// UI Settings Validation
	// ==========================================================================

	// Validate UI theme
	validThemes := map[string]bool{"dark": true, "light": true, "auto": true}
	if !validThemes[strings.ToLower(c.UI.Theme)] {
		errs = append(errs, ValidationError{
			Field:   "ui.theme",
			Message: fmt.Sprintf("invalid theme '%s', must be one of: dark, light, auto", c.UI.Theme),
		})
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// SetDefaults sets default values for any missing or zero-value configuration fields.
// CONFIG: Comprehensive validation ensures safe configuration
func (c *Config) SetDefaults() {
	defaults := Default()

	// General defaults
	if c.Version == "" {
		c.Version = defaults.Version
	}
	if c.DefaultModel == "" {
		c.DefaultModel = defaults.DefaultModel
	}

	// Routing defaults
	if c.Routing.DefaultMode == "" {
		c.Routing.DefaultMode = defaults.Routing.DefaultMode
	}
	if c.Routing.MaxTier == "" {
		c.Routing.MaxTier = "sonnet" // Default to sonnet tier
	}
	if c.Routing.AutoFallback == "" {
		c.Routing.AutoFallback = defaults.Routing.AutoFallback
	}

	// Local defaults
	if c.Local.OllamaURL == "" {
		c.Local.OllamaURL = defaults.Local.OllamaURL
	}
	if c.Local.OllamaModel == "" {
		c.Local.OllamaModel = defaults.Local.OllamaModel
	}

	// Cloud defaults
	if c.Cloud.DefaultModel == "" {
		c.Cloud.DefaultModel = defaults.Cloud.DefaultModel
	}

	// Security defaults
	// IL5 AC-12 compliance: Session timeout cannot be 0 (disabled) or below minimum
	// Valid range: 900-1800 seconds (15-30 minutes). Default: 1800 (30 min)
	if c.Security.SessionTimeoutSecs == 0 || c.Security.SessionTimeoutSecs < 900 {
		c.Security.SessionTimeoutSecs = 1800 // 30 min default
	}
	if c.Security.TLSMinVersion == "" {
		c.Security.TLSMinVersion = "TLS1.2" // Minimum TLS 1.2 for FIPS compliance
	}
	if c.Security.Classification == "" {
		c.Security.Classification = defaults.Security.Classification
	}
	if c.Security.MaxLoginAttempts == 0 {
		c.Security.MaxLoginAttempts = defaults.Security.MaxLoginAttempts
	}
	if c.Security.LockoutDurationMinutes == 0 {
		c.Security.LockoutDurationMinutes = defaults.Security.LockoutDurationMinutes
	}
	if c.Security.AuthSessionDurationHours == 0 {
		c.Security.AuthSessionDurationHours = defaults.Security.AuthSessionDurationHours
	}
	if c.Security.SpillageAction == "" {
		c.Security.SpillageAction = defaults.Security.SpillageAction
	}
	if c.Security.AllowedCiphers == "" {
		c.Security.AllowedCiphers = "AES-256-GCM"
	}

	// Cache defaults
	if c.Cache.TTLHours == 0 {
		c.Cache.TTLHours = defaults.Cache.TTLHours
	}
	if c.Cache.MaxSize == 0 {
		c.Cache.MaxSize = defaults.Cache.MaxSize
	}
	if c.Cache.SemanticThreshold == 0 {
		c.Cache.SemanticThreshold = defaults.Cache.SemanticThreshold
	}

	// UI defaults
	if c.UI.Theme == "" {
		c.UI.Theme = defaults.UI.Theme
	}

	// Derive lockout duration in seconds from minutes
	if c.Security.LockoutDurationSecs == 0 && c.Security.LockoutDurationMinutes > 0 {
		c.Security.LockoutDurationSecs = c.Security.LockoutDurationMinutes * 60
	}
}

// Migrate handles migration from old configuration formats to new ones.
// CONFIG: Comprehensive validation ensures safe configuration
func (c *Config) Migrate() error {
	// Handle "hybrid" mode migration (deprecated, now aliased to "auto")
	if strings.ToLower(c.Routing.DefaultMode) == "hybrid" {
		c.Routing.DefaultMode = "auto"
	}

	// Handle old TLS version format migration
	// Normalize TLS version strings to consistent format
	switch c.Security.TLSMinVersion {
	case "1.2":
		c.Security.TLSMinVersion = "TLS1.2"
	case "1.3":
		c.Security.TLSMinVersion = "TLS1.3"
	}

	// Ensure PinnedCertificates map is initialized if needed
	if c.Security.CertificatePinning && c.Security.PinnedCertificates == nil {
		c.Security.PinnedCertificates = make(map[string]string)
	}

	return nil
}

// =============================================================================
// ENVIRONMENT OVERRIDES
// =============================================================================

// ApplyEnvOverrides applies environment variable overrides to the config.
//
// Supported environment variables:
//   - RIGRUN_MODEL: overrides default_model
//   - RIGRUN_OPENROUTER_KEY: overrides cloud.openrouter_key
//   - RIGRUN_PARANOID: set to "1" or "true" to enable paranoid mode
//   - RIGRUN_OFFLINE: set to "1" or "true" to enable offline mode (IL5 SC-7)
//   - RIGRUN_NO_NETWORK: alias for RIGRUN_OFFLINE
//   - RIGRUN_OLLAMA_URL: overrides local.ollama_url
//   - RIGRUN_MODE: overrides routing.default_mode
//   - RIGRUN_MAX_TIER: overrides routing.max_tier
//   - RIGRUN_CLASSIFICATION: overrides security.classification
func (c *Config) ApplyEnvOverrides() {
	// RIGRUN_MODEL
	if model := os.Getenv("RIGRUN_MODEL"); model != "" {
		c.DefaultModel = model
		c.Local.OllamaModel = model
	}

	// RIGRUN_OPENROUTER_KEY
	if key := os.Getenv("RIGRUN_OPENROUTER_KEY"); key != "" {
		c.Cloud.OpenRouterKey = key
	}

	// RIGRUN_PARANOID
	if paranoid := os.Getenv("RIGRUN_PARANOID"); paranoid != "" {
		c.Routing.ParanoidMode = paranoid == "1" || strings.ToLower(paranoid) == "true"
	}

	// RIGRUN_OFFLINE / RIGRUN_NO_NETWORK (IL5 SC-7 compliance)
	if offline := os.Getenv("RIGRUN_OFFLINE"); offline != "" {
		c.Routing.OfflineMode = offline == "1" || strings.ToLower(offline) == "true"
	}
	if noNetwork := os.Getenv("RIGRUN_NO_NETWORK"); noNetwork != "" {
		c.Routing.OfflineMode = noNetwork == "1" || strings.ToLower(noNetwork) == "true"
	}

	// RIGRUN_OLLAMA_URL
	if url := os.Getenv("RIGRUN_OLLAMA_URL"); url != "" {
		c.Local.OllamaURL = url
	}

	// RIGRUN_MODE
	if mode := os.Getenv("RIGRUN_MODE"); mode != "" {
		c.Routing.DefaultMode = mode
	}

	// RIGRUN_MAX_TIER
	if tier := os.Getenv("RIGRUN_MAX_TIER"); tier != "" {
		c.Routing.MaxTier = tier
	}

	// RIGRUN_CLASSIFICATION
	if classification := os.Getenv("RIGRUN_CLASSIFICATION"); classification != "" {
		c.Security.Classification = classification
	}

	// RIGRUN_POLICY_KEY (AU-9: Protection of Audit Information)
	if policyKey := os.Getenv("RIGRUN_POLICY_KEY"); policyKey != "" {
		c.Security.PolicyKey = policyKey
	}
}

// =============================================================================
// GET/SET HELPERS (DOT NOTATION)
// =============================================================================

// Get retrieves a configuration value using dot notation (e.g., "routing.max_tier").
func (c *Config) Get(key string) (interface{}, error) {
	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return nil, errors.New("empty key")
	}

	v := reflect.ValueOf(c).Elem()
	for i, part := range parts {
		// Normalize the part name
		fieldName := normalizeFieldName(part)

		// Find the field
		field := v.FieldByNameFunc(func(name string) bool {
			return strings.EqualFold(name, fieldName)
		})

		if !field.IsValid() {
			return nil, fmt.Errorf("unknown field: %s", strings.Join(parts[:i+1], "."))
		}

		// If this is the last part, return the value
		if i == len(parts)-1 {
			return field.Interface(), nil
		}

		// Otherwise, navigate into the struct
		if field.Kind() == reflect.Struct {
			v = field
		} else {
			return nil, fmt.Errorf("field '%s' is not a struct", strings.Join(parts[:i+1], "."))
		}
	}

	return nil, fmt.Errorf("invalid key: %s", key)
}

// Set sets a configuration value using dot notation (e.g., "routing.max_tier").
func (c *Config) Set(key string, value interface{}) error {
	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return errors.New("empty key")
	}

	v := reflect.ValueOf(c).Elem()
	for i, part := range parts {
		// Normalize the part name
		fieldName := normalizeFieldName(part)

		// Find the field
		field := v.FieldByNameFunc(func(name string) bool {
			return strings.EqualFold(name, fieldName)
		})

		if !field.IsValid() {
			return fmt.Errorf("unknown field: %s", strings.Join(parts[:i+1], "."))
		}

		// If this is the last part, set the value
		if i == len(parts)-1 {
			if !field.CanSet() {
				return fmt.Errorf("cannot set field: %s", key)
			}
			return setFieldValue(field, value)
		}

		// Otherwise, navigate into the struct
		if field.Kind() == reflect.Struct {
			v = field
		} else {
			return fmt.Errorf("field '%s' is not a struct", strings.Join(parts[:i+1], "."))
		}
	}

	return fmt.Errorf("invalid key: %s", key)
}

// normalizeFieldName converts a snake_case or kebab-case name to its Go field equivalent.
func normalizeFieldName(name string) string {
	// Remove underscores and capitalize following letters
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-'
	})

	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			result.WriteString(strings.ToUpper(string(part[0])))
			result.WriteString(strings.ToLower(part[1:]))
		}
	}

	return result.String()
}

// setFieldValue sets a reflect.Value from an interface{} value with type conversion.
func setFieldValue(field reflect.Value, value interface{}) error {
	// Handle string input with type conversion
	if strVal, ok := value.(string); ok {
		switch field.Kind() {
		case reflect.String:
			field.SetString(strVal)
			return nil
		case reflect.Int, reflect.Int64:
			intVal, err := strconv.ParseInt(strVal, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid integer value: %v", err)
			}
			field.SetInt(intVal)
			return nil
		case reflect.Float64:
			floatVal, err := strconv.ParseFloat(strVal, 64)
			if err != nil {
				return fmt.Errorf("invalid float value: %v", err)
			}
			field.SetFloat(floatVal)
			return nil
		case reflect.Bool:
			boolVal := strVal == "1" || strings.ToLower(strVal) == "true" || strings.ToLower(strVal) == "yes"
			field.SetBool(boolVal)
			return nil
		}
	}

	// Direct assignment for matching types
	val := reflect.ValueOf(value)
	if val.Type().AssignableTo(field.Type()) {
		field.Set(val)
		return nil
	}

	// Type conversion for compatible types
	if val.Type().ConvertibleTo(field.Type()) {
		field.Set(val.Convert(field.Type()))
		return nil
	}

	return fmt.Errorf("cannot assign %T to %s", value, field.Type())
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// GetAllKeys returns all configuration keys in dot notation.
func GetAllKeys() []string {
	return []string{
		"version",
		"default_model",
		"routing.default_mode",
		"routing.max_tier",
		"routing.paranoid_mode",
		"routing.offline_mode",
		"routing.auto_prefer_local",
		"routing.auto_max_cost",
		"routing.auto_fallback",
		"local.ollama_url",
		"local.ollama_model",
		"cloud.openrouter_key",
		"cloud.default_model",
		"security.session_timeout_secs",
		"security.audit_enabled",
		"security.audit_log_path",
		"security.banner_enabled",
		"security.classification",
		// AC-7: Unsuccessful Logon Attempts
		"security.max_login_attempts",
		"security.lockout_duration_minutes",
		"security.lockout_enabled",
		// IA-2: Identification and Authentication
		"security.auth_session_duration_hours",
		"security.mfa_enabled",
		// AU-5: Audit Failure Response
		"security.halt_on_audit_failure",
		"security.audit_capacity_warning_mb",
		"security.audit_capacity_critical_mb",
		// SI-7: Integrity Verification
		"security.integrity_check_on_startup",
		"security.integrity_checksum_file",
		// SC-28: Protection of Information at Rest
		"security.encryption_enabled",
		"security.encrypt_config",
		"security.encrypt_cache",
		"security.encrypt_audit",
		// SC-13: Cryptographic Protection
		"security.fips_mode",
		// SC-17: PKI Certificates
		"security.tls_min_version",
		"security.certificate_pinning",
		"security.pinned_certificates",
		// IR-6: Incident Reporting
		"security.incident_webhook",
		// IR-9: Information Spillage Response
		"security.spillage_detection",
		"security.spillage_action",
		// AU-9: Protection of Audit Information
		"security.policy_key",
		"cache.enabled",
		"cache.ttl_hours",
		"cache.max_size",
		"cache.semantic_enabled",
		"cache.semantic_threshold",
		"ui.theme",
		"ui.show_cost",
		"ui.show_tokens",
		"ui.compact_mode",
		"ui.vim_mode",
		"ui.tutorial_completed",
		"ui.tutorial_step",
	}
}

// Merge merges another config into this one, overwriting only non-zero values.
func (c *Config) Merge(other *Config) {
	if other == nil {
		return
	}

	// General
	if other.Version != "" {
		c.Version = other.Version
	}
	if other.DefaultModel != "" {
		c.DefaultModel = other.DefaultModel
	}

	// Routing
	if other.Routing.DefaultMode != "" {
		c.Routing.DefaultMode = other.Routing.DefaultMode
	}
	if other.Routing.MaxTier != "" {
		c.Routing.MaxTier = other.Routing.MaxTier
	}
	if other.Routing.ParanoidMode {
		c.Routing.ParanoidMode = true
	}
	if other.Routing.AutoPreferLocal {
		c.Routing.AutoPreferLocal = true
	}
	if other.Routing.AutoMaxCost != 0 {
		c.Routing.AutoMaxCost = other.Routing.AutoMaxCost
	}
	if other.Routing.AutoFallback != "" {
		c.Routing.AutoFallback = other.Routing.AutoFallback
	}

	// Local
	if other.Local.OllamaURL != "" {
		c.Local.OllamaURL = other.Local.OllamaURL
	}
	if other.Local.OllamaModel != "" {
		c.Local.OllamaModel = other.Local.OllamaModel
	}

	// Cloud
	if other.Cloud.OpenRouterKey != "" {
		c.Cloud.OpenRouterKey = other.Cloud.OpenRouterKey
	}
	if other.Cloud.DefaultModel != "" {
		c.Cloud.DefaultModel = other.Cloud.DefaultModel
	}

	// Security
	if other.Security.SessionTimeoutSecs != 0 {
		c.Security.SessionTimeoutSecs = other.Security.SessionTimeoutSecs
	}
	if other.Security.AuditEnabled {
		c.Security.AuditEnabled = true
	}
	if other.Security.BannerEnabled {
		c.Security.BannerEnabled = true
	}
	if other.Security.Classification != "" {
		c.Security.Classification = other.Security.Classification
	}

	// Cache
	if other.Cache.Enabled {
		c.Cache.Enabled = true
	}
	if other.Cache.TTLHours != 0 {
		c.Cache.TTLHours = other.Cache.TTLHours
	}
	if other.Cache.MaxSize != 0 {
		c.Cache.MaxSize = other.Cache.MaxSize
	}
	if other.Cache.SemanticEnabled {
		c.Cache.SemanticEnabled = true
	}
	if other.Cache.SemanticThreshold != 0 {
		c.Cache.SemanticThreshold = other.Cache.SemanticThreshold
	}

	// UI
	if other.UI.Theme != "" {
		c.UI.Theme = other.UI.Theme
	}
	if other.UI.ShowCost {
		c.UI.ShowCost = true
	}
	if other.UI.ShowTokens {
		c.UI.ShowTokens = true
	}
	if other.UI.CompactMode {
		c.UI.CompactMode = true
	}
}

// Clone creates a deep copy of the configuration.
// SECURITY: Deep copy is critical to prevent unintended mutation of the original
// config through shared references to maps/slices. A shallow copy would allow
// modifications to PinnedCertificates or other map fields to affect the original,
// potentially bypassing security controls or causing race conditions.
func (c *Config) Clone() *Config {
	// Start with a shallow copy of the struct (copies all value types)
	clone := *c

	// Deep copy the Security.PinnedCertificates map to prevent shared reference
	// SECURITY: PinnedCertificates contains SC-17 certificate fingerprints;
	// shared references could allow unauthorized modification of pinned certs.
	if c.Security.PinnedCertificates != nil {
		clone.Security.PinnedCertificates = make(map[string]string, len(c.Security.PinnedCertificates))
		for k, v := range c.Security.PinnedCertificates {
			clone.Security.PinnedCertificates[k] = v
		}
	}

	return &clone
}

// String returns a string representation of the config for debugging.
// SECURITY: Redacts sensitive fields (API keys, policy keys) to prevent
// accidental exposure in logs, error messages, or debug output.
// Per CWE-532 (Insertion of Sensitive Information into Log File) and
// NIST 800-53 AU-3 (Content of Audit Records), secrets must not appear
// in plaintext in any output that could be logged or displayed.
func (c *Config) String() string {
	// Create a deep copy to safely redact without modifying original
	safe := c.Clone()

	// Redact cloud API keys
	if safe.Cloud.OpenRouterKey != "" {
		safe.Cloud.OpenRouterKey = "[REDACTED]"
	}

	// Redact policy/HMAC key (AU-9: Protection of Audit Information)
	if safe.Security.PolicyKey != "" {
		safe.Security.PolicyKey = "[REDACTED]"
	}

	data, _ := json.MarshalIndent(safe, "", "  ")
	return string(data)
}

// =============================================================================
// SINGLETON PATTERN (THREAD-SAFE)
// =============================================================================

var (
	globalConfig     *Config
	globalConfigOnce sync.Once
	globalConfigMu   sync.RWMutex
)

// Global returns the global configuration instance.
// Loads configuration on first access. Thread-safe.
func Global() *Config {
	// Use sync.Once to ensure initialization happens exactly once
	globalConfigOnce.Do(func() {
		cfg, err := Load()
		if err != nil {
			// Log but don't fail - use defaults
			fmt.Fprintf(os.Stderr, "Warning: %v (using defaults)\n", err)
		}
		globalConfig = cfg
	})

	globalConfigMu.RLock()
	defer globalConfigMu.RUnlock()
	return globalConfig
}

// ReloadGlobal reloads the global configuration from disk. Thread-safe.
func ReloadGlobal() error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	globalConfigMu.Lock()
	defer globalConfigMu.Unlock()
	globalConfig = cfg
	return nil
}

// SetGlobal sets the global configuration instance. Thread-safe.
func SetGlobal(cfg *Config) {
	globalConfigMu.Lock()
	defer globalConfigMu.Unlock()
	globalConfig = cfg
}

// ResetGlobalForTesting resets the global config state for testing.
// This should only be used in tests to reset state between test runs.
func ResetGlobalForTesting() {
	globalConfigMu.Lock()
	defer globalConfigMu.Unlock()
	globalConfig = nil
	globalConfigOnce = sync.Once{}
}
