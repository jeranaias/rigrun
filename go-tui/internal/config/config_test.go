// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"sync"
	"testing"
)

// TestConfig_ConcurrentAccess tests that Global(), SetGlobal(), and ReloadGlobal()
// can be safely called concurrently without race conditions.
// Run with: go test -race -v ./internal/config/
func TestConfig_ConcurrentAccess(t *testing.T) {
	// Reset state before test
	ResetGlobalForTesting()

	var wg sync.WaitGroup

	// 50 writers using SetGlobal, 50 readers using Global
	for i := 0; i < 50; i++ {
		wg.Add(2)

		// Writer goroutine
		go func(id int) {
			defer wg.Done()
			c := &Config{
				Version:      "test",
				DefaultModel: "test-model",
				Routing: RoutingConfig{
					DefaultMode: "auto",
					MaxTier:     "opus",
				},
			}
			SetGlobal(c)
		}(i)

		// Reader goroutine
		go func(id int) {
			defer wg.Done()
			cfg := Global()
			if cfg == nil {
				t.Error("Global() returned nil")
			}
		}(i)
	}

	wg.Wait()
}

// TestConfig_ConcurrentReload tests concurrent ReloadGlobal and Global calls.
func TestConfig_ConcurrentReload(t *testing.T) {
	// Reset state before test
	ResetGlobalForTesting()

	// Initialize config first
	_ = Global()

	var wg sync.WaitGroup

	// 20 reloaders, 80 readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// ReloadGlobal may fail if config file doesn't exist, that's ok
			_ = ReloadGlobal()
		}()
	}

	for i := 0; i < 80; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg := Global()
			if cfg == nil {
				t.Error("Global() returned nil")
			}
		}()
	}

	wg.Wait()
}

// TestConfig_GlobalInitialization tests that Global() properly initializes
// the config on first access.
func TestConfig_GlobalInitialization(t *testing.T) {
	// Reset state before test
	ResetGlobalForTesting()

	cfg := Global()
	if cfg == nil {
		t.Fatal("Global() returned nil")
	}

	// Verify defaults are applied
	if cfg.Version == "" {
		t.Error("Config version should not be empty")
	}
	if cfg.Routing.DefaultMode == "" {
		t.Error("Routing default mode should not be empty")
	}
}

// TestConfig_SetGlobalOverwrites tests that SetGlobal properly overwrites
// the existing global config.
func TestConfig_SetGlobalOverwrites(t *testing.T) {
	// Reset state before test
	ResetGlobalForTesting()

	// Initialize with defaults
	_ = Global()

	// Set custom config
	customCfg := &Config{
		Version:      "custom-version",
		DefaultModel: "custom-model",
	}
	SetGlobal(customCfg)

	// Verify the custom config is returned
	result := Global()
	if result.Version != "custom-version" {
		t.Errorf("Expected version 'custom-version', got '%s'", result.Version)
	}
	if result.DefaultModel != "custom-model" {
		t.Errorf("Expected model 'custom-model', got '%s'", result.DefaultModel)
	}
}

// TestConfig_ConcurrentMixedOperations tests a mix of all global operations
// happening concurrently.
func TestConfig_ConcurrentMixedOperations(t *testing.T) {
	// Reset state before test
	ResetGlobalForTesting()

	var wg sync.WaitGroup

	// Mix of operations: Global, SetGlobal, ReloadGlobal
	for i := 0; i < 100; i++ {
		wg.Add(1)
		switch i % 3 {
		case 0:
			// Reader
			go func() {
				defer wg.Done()
				cfg := Global()
				if cfg == nil {
					t.Error("Global() returned nil")
				}
			}()
		case 1:
			// SetGlobal writer
			go func() {
				defer wg.Done()
				c := Default()
				c.Version = "concurrent-test"
				SetGlobal(c)
			}()
		case 2:
			// ReloadGlobal
			go func() {
				defer wg.Done()
				_ = ReloadGlobal()
			}()
		}
	}

	wg.Wait()
}

// TestConfig_Default tests that Default() returns a valid config with defaults.
func TestConfig_Default(t *testing.T) {
	cfg := Default()

	if cfg == nil {
		t.Fatal("Default() returned nil")
	}

	if cfg.Version == "" {
		t.Error("Default config should have a version")
	}

	if cfg.Routing.DefaultMode != "auto" {
		t.Errorf("Expected default routing mode 'auto', got '%s'", cfg.Routing.DefaultMode)
	}

	if cfg.Local.OllamaURL == "" {
		t.Error("Default config should have an Ollama URL")
	}

	if cfg.Security.SessionTimeoutSecs == 0 {
		t.Error("Default config should have a session timeout")
	}
}

// TestConfig_Validate tests configuration validation.
func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			config:  Default(),
			wantErr: false,
		},
		{
			name: "invalid routing mode",
			config: func() *Config {
				c := Default()
				c.Routing.DefaultMode = "invalid"
				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid max tier",
			config: func() *Config {
				c := Default()
				c.Routing.MaxTier = "invalid"
				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid theme",
			config: func() *Config {
				c := Default()
				c.UI.Theme = "invalid"
				return c
			}(),
			wantErr: true,
		},
		{
			name: "negative auto_max_cost",
			config: func() *Config {
				c := Default()
				c.Routing.AutoMaxCost = -1
				return c
			}(),
			wantErr: true,
		},
		{
			name: "invalid semantic threshold",
			config: func() *Config {
				c := Default()
				c.Cache.SemanticThreshold = 1.5
				return c
			}(),
			wantErr: true,
		},
		{
			name: "session timeout disabled (zero)",
			config: func() *Config {
				c := Default()
				c.Security.SessionTimeoutSecs = 0
				return c
			}(),
			wantErr: true,
		},
		{
			name: "session timeout below minimum",
			config: func() *Config {
				c := Default()
				c.Security.SessionTimeoutSecs = 500
				return c
			}(),
			wantErr: true,
		},
		{
			name: "session timeout above maximum",
			config: func() *Config {
				c := Default()
				c.Security.SessionTimeoutSecs = 2000
				return c
			}(),
			wantErr: true,
		},
		{
			name: "session timeout at minimum (900)",
			config: func() *Config {
				c := Default()
				c.Security.SessionTimeoutSecs = 900
				return c
			}(),
			wantErr: false,
		},
		{
			name: "session timeout at maximum (1800)",
			config: func() *Config {
				c := Default()
				c.Security.SessionTimeoutSecs = 1800
				return c
			}(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConfig_GetSet tests Get and Set methods with dot notation.
func TestConfig_GetSet(t *testing.T) {
	cfg := Default()

	// Test Get
	val, err := cfg.Get("routing.default_mode")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if val != "auto" {
		t.Errorf("Get('routing.default_mode') = %v, want 'auto'", val)
	}

	// Test Set
	err = cfg.Set("routing.max_tier", "sonnet")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	val, _ = cfg.Get("routing.max_tier")
	if val != "sonnet" {
		t.Errorf("Get('routing.max_tier') after Set = %v, want 'sonnet'", val)
	}

	// Test Get with invalid key
	_, err = cfg.Get("invalid.key")
	if err == nil {
		t.Error("Get() with invalid key should return error")
	}
}

// TestConfig_Clone tests that Clone creates an independent copy.
func TestConfig_Clone(t *testing.T) {
	original := Default()
	original.Version = "original"

	clone := original.Clone()

	// Modify clone
	clone.Version = "cloned"

	// Verify original unchanged
	if original.Version != "original" {
		t.Error("Clone should create an independent copy")
	}
	if clone.Version != "cloned" {
		t.Error("Clone version should be modified")
	}
}

// TestConfig_Merge tests merging two configs.
func TestConfig_Merge(t *testing.T) {
	base := Default()
	base.Version = "base"

	other := &Config{
		Version:      "merged",
		DefaultModel: "merged-model",
	}

	base.Merge(other)

	if base.Version != "merged" {
		t.Errorf("Merge should overwrite Version, got '%s'", base.Version)
	}
	if base.DefaultModel != "merged-model" {
		t.Errorf("Merge should overwrite DefaultModel, got '%s'", base.DefaultModel)
	}
	// Verify non-overwritten values remain
	if base.Routing.DefaultMode != "auto" {
		t.Error("Merge should not overwrite unset fields")
	}
}
