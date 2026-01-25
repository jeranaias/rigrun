// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package model contains the data structures for conversations and messages.
package model

import (
	"strings"
	"testing"
)

// =============================================================================
// MODEL INFO TESTS
// =============================================================================

func TestModelInfo_CapabilitiesString(t *testing.T) {
	tests := []struct {
		name     string
		model    ModelInfo
		contains []string
	}{
		{
			name:     "long context model",
			model:    ModelInfo{MaxTokens: 200000, Tier: "Balanced"},
			contains: []string{"Long context"},
		},
		{
			name:     "extended context model",
			model:    ModelInfo{MaxTokens: 64000, Tier: "Balanced"},
			contains: []string{"Extended context"},
		},
		{
			name:     "fast tier model",
			model:    ModelInfo{MaxTokens: 8000, Tier: "Fast"},
			contains: []string{"Low latency"},
		},
		{
			name:     "free local model",
			model:    ModelInfo{MaxTokens: 8000, CostPer1K: 0, Provider: "Local"},
			contains: []string{"Free"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caps := tc.model.CapabilitiesString()
			for _, want := range tc.contains {
				if !strings.Contains(caps, want) {
					t.Errorf("CapabilitiesString() = %q, want to contain %q", caps, want)
				}
			}
		})
	}
}

// =============================================================================
// MODEL REGISTRY TESTS
// =============================================================================

func TestModels_Registry(t *testing.T) {
	// Verify essential models are in the registry
	essentialModels := []string{"haiku", "sonnet", "opus", "gpt-4o", "llama3", "mistral"}

	for _, id := range essentialModels {
		if _, ok := Models[id]; !ok {
			t.Errorf("Essential model %q missing from registry", id)
		}
	}
}

func TestModels_HaveRequiredFields(t *testing.T) {
	for id, model := range Models {
		t.Run(id, func(t *testing.T) {
			if model.ID == "" {
				t.Error("Model.ID should not be empty")
			}
			if model.Name == "" {
				t.Error("Model.Name should not be empty")
			}
			if model.Provider == "" {
				t.Error("Model.Provider should not be empty")
			}
			if model.MaxTokens <= 0 {
				t.Error("Model.MaxTokens should be positive")
			}
		})
	}
}

func TestModels_LocalModelsAreFree(t *testing.T) {
	for id, model := range Models {
		if model.Provider == "Local" && model.CostPer1K != 0 {
			t.Errorf("Local model %q should have CostPer1K = 0, got %f", id, model.CostPer1K)
		}
	}
}

func TestModels_CloudModelsHaveCost(t *testing.T) {
	cloudProviders := map[string]bool{"Anthropic": true, "OpenAI": true}

	for id, model := range Models {
		if cloudProviders[model.Provider] && model.CostPer1K <= 0 {
			t.Errorf("Cloud model %q should have positive CostPer1K, got %f", id, model.CostPer1K)
		}
	}
}

// =============================================================================
// LOOKUP TESTS
// =============================================================================

func TestGetModelInfo(t *testing.T) {
	// Test existing model by short name
	model, ok := GetModelInfo("sonnet")
	if !ok {
		t.Error("GetModelInfo(sonnet) should return true")
	}
	if model.Name != "Claude 3.5 Sonnet" {
		t.Errorf("GetModelInfo(sonnet).Name = %q, want 'Claude 3.5 Sonnet'", model.Name)
	}

	// Test by full API ID
	model, ok = GetModelInfo("claude-3-5-sonnet-20241022")
	if !ok {
		t.Error("GetModelInfo should find model by API ID")
	}
	if model.Provider != "Anthropic" {
		t.Error("Found model should be Anthropic")
	}

	// Test non-existent model
	_, ok = GetModelInfo("nonexistent-model")
	if ok {
		t.Error("GetModelInfo(nonexistent-model) should return false")
	}
}

func TestGetLocalModels(t *testing.T) {
	models := GetLocalModels()

	for _, m := range models {
		if m.Provider != "Local" {
			t.Errorf("GetLocalModels returned non-local model: %s (provider: %s)", m.Name, m.Provider)
		}
		if m.CostPer1K != 0 {
			t.Errorf("Local model %s should be free", m.Name)
		}
	}
}

func TestGetCloudModels(t *testing.T) {
	models := GetCloudModels()

	for _, m := range models {
		if m.Provider == "Local" {
			t.Errorf("GetCloudModels returned local model: %s", m.Name)
		}
	}

	// Should have at least Anthropic and OpenAI models
	if len(models) < 3 {
		t.Errorf("Expected at least 3 cloud models, got %d", len(models))
	}
}

func TestGetModelsByProvider(t *testing.T) {
	anthropicModels := GetModelsByProvider("Anthropic")
	if len(anthropicModels) == 0 {
		t.Error("Should have Anthropic models")
	}
	for _, m := range anthropicModels {
		if m.Provider != "Anthropic" {
			t.Errorf("GetModelsByProvider(Anthropic) returned %s model", m.Provider)
		}
	}

	localModels := GetModelsByProvider("Local")
	if len(localModels) == 0 {
		t.Error("Should have Local models")
	}
}

func TestGetModelsByTier(t *testing.T) {
	fastModels := GetModelsByTier("Fast")
	for _, m := range fastModels {
		if m.Tier != "Fast" {
			t.Errorf("GetModelsByTier(Fast) returned %s tier model", m.Tier)
		}
	}

	balancedModels := GetModelsByTier("Balanced")
	if len(balancedModels) == 0 {
		t.Error("Should have Balanced tier models")
	}
}
