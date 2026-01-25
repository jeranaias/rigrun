// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package config provides unified configuration loading and management for rigrun.
//
// Supports both TOML and JSON configuration formats, with sensible defaults,
// environment variable overrides, and validation.
//
// # Key Types
//
//   - Config: Main configuration structure with all settings
//   - ModelConfig: Model-specific configuration (local, cloud)
//   - SecurityConfig: Security settings for IL5 compliance
//   - CacheConfig: Cache behavior configuration
//
// # Configuration Precedence
//
// Configuration is loaded from (in order of precedence):
//   - Environment variables (RIGRUN_*)
//   - ~/.rigrun/config.toml
//   - ~/.rigrun/config.json
//   - Built-in defaults
//
// # Usage
//
// Load configuration:
//
//	cfg, err := config.Load()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Access settings:
//
//	model := cfg.Model.Default
//	timeout := cfg.Security.SessionTimeout
package config
