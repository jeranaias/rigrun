// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// config.go - Config command implementation for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Command: config [subcommand]
// Short:   View and modify configuration
// Aliases: (none)
//
// Subcommands:
//   show (default)      Display current configuration
//   set <key> <value>   Set a configuration value
//   reset               Reset to default configuration
//   path                Show configuration file path
//
// Examples:
//   rigrun config                         Show current config (default)
//   rigrun config show                    Show current configuration
//   rigrun config show --json             Config in JSON format
//   rigrun config set default_model qwen2.5:14b
//   rigrun config set openrouter_key sk-or-xxx
//   rigrun config set paranoid_mode true
//   rigrun config set offline_mode true   Enable IL5 air-gapped mode
//   rigrun config set default_mode local  Set routing mode
//   rigrun config set default_mode hybrid Enable intelligent routing
//   rigrun config set max_tier sonnet     Limit cloud tier
//   rigrun config set audit_enabled true  Enable audit logging
//   rigrun config reset                   Reset to defaults
//   rigrun config path                    Show config file location
//
// Configuration Keys:
//   default_model       Default model name
//   default_mode        Routing mode (local/cloud/hybrid)
//   ollama_url          Ollama server URL
//   ollama_model        Default local model
//   openrouter_key      OpenRouter API key
//   cloud_model         Default cloud model
//   max_tier            Maximum cloud tier (cache/local/haiku/sonnet/opus/gpt-4o)
//   paranoid_mode       Block all cloud requests (true/false)
//   offline_mode        IL5 SC-7: Block ALL network (true/false)
//   session_timeout     Session timeout in seconds
//   audit_enabled       Enable audit logging (true/false)
//   cache_enabled       Enable response caching (true/false)
//   cache_ttl_hours     Cache TTL in hours
//
// Flags:
//   --json              Output in JSON format
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
package cli

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/config"
)

// =============================================================================
// CONFIG STYLES
// =============================================================================

var (
	// Config title style
	configTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	// Config section style
	configSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	// Config key style
	configKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")). // Light gray
			Width(20)

	// Config value style
	configValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")) // Green

	// Config value masked (for secrets)
	configMaskedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("242")) // Dim

	// Success style
	configSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	// Error style
	configErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)

	// Path style
	configPathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true)
)

// =============================================================================
// CONFIG WRAPPER FUNCTIONS (for backward compatibility)
// =============================================================================

// Config is an alias to the main config type for backward compatibility.
type Config = config.Config

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return config.Default()
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	path, err := config.ConfigPathTOML()
	if err != nil {
		return ""
	}
	return path
}

// LoadConfig loads the configuration from the config file.
// Returns default config if file doesn't exist.
func LoadConfig() (*Config, error) {
	return config.Load()
}

// SaveConfig saves the configuration to the config file.
func SaveConfig(cfg *Config) error {
	return config.Save(cfg)
}

// =============================================================================
// HANDLE CONFIG
// =============================================================================

// HandleConfig handles the "config" command.
// Supports JSON output for IL5 SIEM integration (AU-6, SI-4).
func HandleConfig(args Args) error {
	switch args.Subcommand {
	case "", "show":
		if args.JSON {
			return handleConfigShowJSON()
		}
		return handleConfigShow()

	case "set":
		return handleConfigSet(args.ConfigKey, args.ConfigVal)

	case "reset":
		return handleConfigReset()

	case "path":
		if args.JSON {
			return handleConfigPathJSON()
		}
		return handleConfigPath()

	default:
		return fmt.Errorf("unknown config subcommand: %s", args.Subcommand)
	}
}

// handleConfigShowJSON outputs configuration in JSON format for SIEM integration.
func handleConfigShowJSON() error {
	cfg, err := LoadConfig()
	if err != nil {
		// Return error response but still try to show defaults
		cfg = DefaultConfig()
	}

	data := ConfigData{
		General: ConfigGeneralInfo{
			DefaultModel: cfg.DefaultModel,
			DefaultMode:  cfg.Routing.DefaultMode,
		},
		Local: ConfigLocalInfo{
			OllamaURL:   cfg.Local.OllamaURL,
			OllamaModel: cfg.Local.OllamaModel,
		},
		Cloud: ConfigCloudInfo{
			OpenRouterKeySet: cfg.Cloud.OpenRouterKey != "",
			DefaultModel:     cfg.Cloud.DefaultModel,
		},
		Routing: ConfigRoutingInfo{
			DefaultMode:  cfg.Routing.DefaultMode,
			MaxTier:      cfg.Routing.MaxTier,
			ParanoidMode: cfg.Routing.ParanoidMode,
		},
		Security: ConfigSecurityInfo{
			SessionTimeoutSecs: cfg.Security.SessionTimeoutSecs,
			AuditEnabled:       cfg.Security.AuditEnabled,
		},
		Cache: ConfigCacheInfo{
			Enabled:  cfg.Cache.Enabled,
			TTLHours: cfg.Cache.TTLHours,
		},
		Path: ConfigPath(),
	}

	resp := NewJSONResponse("config show", data)
	return resp.Print()
}

// handleConfigPathJSON outputs config path in JSON format.
func handleConfigPathJSON() error {
	path := ConfigPath()
	_, err := os.Stat(path)
	exists := !os.IsNotExist(err)

	data := map[string]interface{}{
		"path":   path,
		"exists": exists,
	}

	resp := NewJSONResponse("config path", data)
	return resp.Print()
}

// handleConfigShow displays the current configuration.
func handleConfigShow() error {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %s (using defaults)\n", err)
	}

	separator := strings.Repeat("=", 41)
	fmt.Println()
	fmt.Println(configTitleStyle.Render("rigrun Configuration"))
	fmt.Println(separatorStyle.Render(separator))
	fmt.Println()

	// General section
	fmt.Println(configSectionStyle.Render("[general]"))
	fmt.Printf("  %s%s\n",
		configKeyStyle.Render("default_model:"),
		configValueStyle.Render(cfg.DefaultModel))
	fmt.Printf("  %s%s\n",
		configKeyStyle.Render("default_mode:"),
		configValueStyle.Render(cfg.Routing.DefaultMode))
	fmt.Println()

	// Local section
	fmt.Println(configSectionStyle.Render("[local]"))
	fmt.Printf("  %s%s\n",
		configKeyStyle.Render("ollama_url:"),
		configValueStyle.Render(cfg.Local.OllamaURL))
	fmt.Printf("  %s%s\n",
		configKeyStyle.Render("ollama_model:"),
		configValueStyle.Render(cfg.Local.OllamaModel))
	fmt.Println()

	// Cloud section
	fmt.Println(configSectionStyle.Render("[cloud]"))
	keyDisplay := maskAPIKey(cfg.Cloud.OpenRouterKey)
	fmt.Printf("  %s%s\n",
		configKeyStyle.Render("openrouter_key:"),
		configMaskedStyle.Render(keyDisplay))
	fmt.Printf("  %s%s\n",
		configKeyStyle.Render("cloud_model:"),
		configValueStyle.Render(cfg.Cloud.DefaultModel))
	fmt.Println()

	// Routing section
	fmt.Println(configSectionStyle.Render("[routing]"))
	fmt.Printf("  %s%s\n",
		configKeyStyle.Render("default_mode:"),
		configValueStyle.Render(cfg.Routing.DefaultMode))
	fmt.Printf("  %s%s\n",
		configKeyStyle.Render("max_tier:"),
		configValueStyle.Render(cfg.Routing.MaxTier))
	paranoidStr := "false"
	if cfg.Routing.ParanoidMode {
		paranoidStr = "true"
	}
	fmt.Printf("  %s%s\n",
		configKeyStyle.Render("paranoid_mode:"),
		configValueStyle.Render(paranoidStr))
	fmt.Println()

	// Security section
	fmt.Println(configSectionStyle.Render("[security]"))
	fmt.Printf("  %s%s\n",
		configKeyStyle.Render("session_timeout:"),
		configValueStyle.Render(fmt.Sprintf("%d seconds", cfg.Security.SessionTimeoutSecs)))
	auditStr := "false"
	if cfg.Security.AuditEnabled {
		auditStr = "true"
	}
	fmt.Printf("  %s%s\n",
		configKeyStyle.Render("audit_enabled:"),
		configValueStyle.Render(auditStr))
	fmt.Println()

	// Cache section
	fmt.Println(configSectionStyle.Render("[cache]"))
	cacheStr := "false"
	if cfg.Cache.Enabled {
		cacheStr = "true"
	}
	fmt.Printf("  %s%s\n",
		configKeyStyle.Render("enabled:"),
		configValueStyle.Render(cacheStr))
	fmt.Printf("  %s%s\n",
		configKeyStyle.Render("ttl_hours:"),
		configValueStyle.Render(fmt.Sprintf("%d", cfg.Cache.TTLHours)))
	fmt.Println()

	// Config file path
	fmt.Println(separatorStyle.Render(strings.Repeat("-", 41)))
	fmt.Printf("Config file: %s\n", configPathStyle.Render(ConfigPath()))
	fmt.Println()

	return nil
}

// handleConfigSet sets a configuration value.
func handleConfigSet(key, value string) error {
	if key == "" {
		return fmt.Errorf("no config key provided\nUsage: rigrun config set <key> <value>")
	}
	if value == "" {
		return fmt.Errorf("no config value provided\nUsage: rigrun config set %s <value>", key)
	}

	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %s (using defaults)\n", err)
		cfg = DefaultConfig()
	}

	// Normalize key (support both dot notation and underscore)
	key = strings.ToLower(key)
	key = strings.ReplaceAll(key, "_", ".")

	// Try using the config's Set method for dot notation
	if err := cfg.Set(key, value); err == nil {
		// Successfully set using dot notation - now validate before saving
		if validateErr := cfg.Validate(); validateErr != nil {
			return fmt.Errorf("invalid configuration value: %w", validateErr)
		}
		if saveErr := SaveConfig(cfg); saveErr != nil {
			return fmt.Errorf("failed to save config: %w", saveErr)
		}
		fmt.Printf("%s %s = %s\n",
			configSuccessStyle.Render("[OK]"),
			key,
			maskIfSecret(key, value))
		return nil
	}

	// Fall back to manual key handling for common shortcuts
	keyNorm := strings.ReplaceAll(key, ".", "_")
	switch keyNorm {
	case "default_model":
		cfg.DefaultModel = value
		cfg.Local.OllamaModel = value // Also set local model

	case "default_mode", "routing_default_mode":
		// Validate mode
		validModes := []string{"local", "cloud", "hybrid"}
		valid := false
		for _, m := range validModes {
			if strings.EqualFold(value, m) {
				valid = true
				value = strings.ToLower(value)
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid mode '%s'. Valid modes: %s", value, strings.Join(validModes, ", "))
		}
		cfg.Routing.DefaultMode = value

	case "ollama_url", "local_ollama_url":
		cfg.Local.OllamaURL = value

	case "ollama_model", "local_ollama_model":
		cfg.Local.OllamaModel = value

	case "openrouter_key", "cloud_openrouter_key":
		// Validate OpenRouter key format
		if !strings.HasPrefix(value, "sk-or-") {
			fmt.Fprintf(os.Stderr, "%s OpenRouter keys should start with 'sk-or-'\n",
				configErrorStyle.Render("[!]"))
			fmt.Fprintf(os.Stderr, "    Get a key at: https://openrouter.ai/keys\n")
		}
		cfg.Cloud.OpenRouterKey = value

	case "cloud_model", "cloud_default_model":
		cfg.Cloud.DefaultModel = value

	case "max_tier", "routing_max_tier":
		// Validate tier
		validTiers := []string{"cache", "local", "cloud", "haiku", "sonnet", "opus", "gpt-4o"}
		valid := false
		for _, t := range validTiers {
			if strings.EqualFold(value, t) {
				valid = true
				value = strings.ToLower(value)
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid tier '%s'. Valid tiers: %s", value, strings.Join(validTiers, ", "))
		}
		cfg.Routing.MaxTier = value

	case "paranoid_mode", "routing_paranoid_mode":
		cfg.Routing.ParanoidMode = parseBool(value)

	case "offline_mode", "routing_offline_mode", "no_network":
		// IL5 SC-7 compliance: Block ALL network except localhost Ollama
		cfg.Routing.OfflineMode = parseBool(value)

	case "session_timeout", "session_timeout_secs", "security_session_timeout_secs":
		var timeout int
		_, err := fmt.Sscanf(value, "%d", &timeout)
		if err != nil || timeout < 0 {
			return fmt.Errorf("invalid session timeout: %s (must be a positive integer)", value)
		}
		// IL5 AC-12 compliance: Session timeout must be 900-1800 seconds (15-30 minutes)
		if timeout < 900 {
			return fmt.Errorf("session timeout must be at least 900 seconds (15 minutes) per IL5 AC-12 compliance, got %d", timeout)
		}
		if timeout > 1800 {
			return fmt.Errorf("session timeout cannot exceed 1800 seconds (30 minutes) per IL5 AC-12 compliance, got %d", timeout)
		}
		cfg.Security.SessionTimeoutSecs = timeout

	case "audit_enabled", "security_audit_enabled":
		cfg.Security.AuditEnabled = parseBool(value)

	case "cache_enabled":
		cfg.Cache.Enabled = parseBool(value)

	case "cache_ttl_hours", "cache_ttl":
		var ttl int
		_, err := fmt.Sscanf(value, "%d", &ttl)
		if err != nil || ttl < 0 {
			return fmt.Errorf("invalid cache TTL: %s (must be a positive integer)", value)
		}
		cfg.Cache.TTLHours = ttl

	default:
		return fmt.Errorf("unknown config key: %s\n\nValid keys:\n"+
			"  default_model      - Default model name\n"+
			"  default_mode       - Default routing mode (local/cloud/hybrid)\n"+
			"  ollama_url         - Ollama server URL\n"+
			"  ollama_model       - Default local model\n"+
			"  openrouter_key     - OpenRouter API key\n"+
			"  cloud_model        - Default cloud model\n"+
			"  max_tier           - Maximum cloud tier to use\n"+
			"  paranoid_mode      - Block all cloud requests (true/false)\n"+
			"  offline_mode       - IL5 SC-7: Block ALL network except localhost (true/false)\n"+
			"  session_timeout    - Session timeout in seconds\n"+
			"  audit_enabled      - Enable audit logging (true/false)\n"+
			"  cache_enabled      - Enable response caching (true/false)\n"+
			"  cache_ttl_hours    - Cache TTL in hours", key)
	}

	// Validate the updated config before saving
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration value: %w", err)
	}

	// Save the updated config
	if err := SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("%s %s = %s\n",
		configSuccessStyle.Render("[OK]"),
		key,
		maskIfSecret(key, value))

	return nil
}

// handleConfigReset resets configuration to defaults.
func handleConfigReset() error {
	config := DefaultConfig()

	if err := SaveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("%s Configuration reset to defaults\n", configSuccessStyle.Render("[OK]"))
	fmt.Printf("Config file: %s\n", configPathStyle.Render(ConfigPath()))

	return nil
}

// handleConfigPath shows the config file path.
func handleConfigPath() error {
	path := ConfigPath()
	fmt.Println(path)

	// Also show if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "%s (file does not exist - will be created on first use)\n",
			configMaskedStyle.Render("Note"))
	}

	return nil
}

// =============================================================================
// HELPERS
// =============================================================================

// maskAPIKey masks an API key for display using a secure SHA-256 fingerprint.
// This prevents API key prefix exposure which could allow attackers to correlate keys.
// NIST 800-53 IA-5(1): Obscure feedback of authentication information.
func maskAPIKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) < 8 {
		return "[invalid key]"
	}
	// Use SHA-256 hash to create a secure fingerprint
	hash := sha256.Sum256([]byte(key))
	// Show first 8 chars of hash as fingerprint (4 bytes = 8 hex chars)
	return fmt.Sprintf("sha256:%x...", hash[:4])
}

// maskIfSecret masks the value if the key is a secret field.
func maskIfSecret(key, value string) string {
	secretKeys := []string{"key", "secret", "token", "password"}
	keyLower := strings.ToLower(key)
	for _, s := range secretKeys {
		if strings.Contains(keyLower, s) {
			return maskAPIKey(value)
		}
	}
	return value
}

// parseBool parses a boolean string value.
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes" || s == "on"
}
