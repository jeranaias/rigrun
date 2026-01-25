// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package model contains the data structures for conversations and messages.
package model

import (
	"fmt"
	"strings"
)

// =============================================================================
// MODEL INFO TYPE
// =============================================================================

// ModelInfo contains detailed information about a model.
// This is used for model selection and display in the UI.
type ModelInfo struct {
	// ID is the model identifier used in API calls
	ID string `json:"id"`

	// Name is the human-readable display name
	Name string `json:"name"`

	// Provider identifies who provides the model (Anthropic, OpenAI, Local)
	Provider string `json:"provider"`

	// Tier categorizes the model's capability level
	Tier string `json:"tier"`

	// CostPer1K is the cost per 1000 tokens in dollars (0 for local models)
	CostPer1K float64 `json:"cost_per_1k"`

	// MaxTokens is the maximum context window size
	MaxTokens int `json:"max_tokens"`

	// Description is a brief explanation of the model's strengths
	Description string `json:"description"`
}

// =============================================================================
// MODEL REGISTRY
// =============================================================================

// Models is the registry of known models with their metadata.
// This includes both cloud API models and well-known local models.
var Models = map[string]ModelInfo{
	// Anthropic Claude models
	"haiku": {
		ID:          "claude-3-haiku-20240307",
		Name:        "Claude 3 Haiku",
		Provider:    "Anthropic",
		Tier:        "Fast",
		CostPer1K:   0.00025,
		MaxTokens:   200000,
		Description: "Fast and efficient for simple tasks",
	},
	"sonnet": {
		ID:          "claude-3-5-sonnet-20241022",
		Name:        "Claude 3.5 Sonnet",
		Provider:    "Anthropic",
		Tier:        "Balanced",
		CostPer1K:   0.003,
		MaxTokens:   200000,
		Description: "Best balance of speed and capability",
	},
	"opus": {
		ID:          "claude-3-opus-20240229",
		Name:        "Claude 3 Opus",
		Provider:    "Anthropic",
		Tier:        "Powerful",
		CostPer1K:   0.015,
		MaxTokens:   200000,
		Description: "Most capable for complex reasoning",
	},

	// OpenAI models
	"gpt-4o": {
		ID:          "gpt-4o",
		Name:        "GPT-4o",
		Provider:    "OpenAI",
		Tier:        "Balanced",
		CostPer1K:   0.0025,
		MaxTokens:   128000,
		Description: "Fast multimodal model with vision",
	},
	"gpt-4o-mini": {
		ID:          "gpt-4o-mini",
		Name:        "GPT-4o Mini",
		Provider:    "OpenAI",
		Tier:        "Fast",
		CostPer1K:   0.00015,
		MaxTokens:   128000,
		Description: "Cost-effective for simple tasks",
	},

	// Local Ollama models (commonly used)
	"llama3": {
		ID:          "llama3",
		Name:        "Llama 3",
		Provider:    "Local",
		Tier:        "Fast",
		CostPer1K:   0.0,
		MaxTokens:   8192,
		Description: "Meta's versatile open-source model",
	},
	"llama3.1": {
		ID:          "llama3.1",
		Name:        "Llama 3.1",
		Provider:    "Local",
		Tier:        "Fast",
		CostPer1K:   0.0,
		MaxTokens:   128000,
		Description: "Extended context Llama 3",
	},
	"qwen2.5-coder": {
		ID:          "qwen2.5-coder",
		Name:        "Qwen 2.5 Coder",
		Provider:    "Local",
		Tier:        "Balanced",
		CostPer1K:   0.0,
		MaxTokens:   32768,
		Description: "Optimized for code generation",
	},
	"codellama": {
		ID:          "codellama",
		Name:        "Code Llama",
		Provider:    "Local",
		Tier:        "Fast",
		CostPer1K:   0.0,
		MaxTokens:   16384,
		Description: "Meta's code-focused model",
	},
	"deepseek-coder": {
		ID:          "deepseek-coder",
		Name:        "DeepSeek Coder",
		Provider:    "Local",
		Tier:        "Balanced",
		CostPer1K:   0.0,
		MaxTokens:   16384,
		Description: "Strong code understanding",
	},
	"mistral": {
		ID:          "mistral",
		Name:        "Mistral",
		Provider:    "Local",
		Tier:        "Fast",
		CostPer1K:   0.0,
		MaxTokens:   32768,
		Description: "Fast and efficient general purpose",
	},
	"mixtral": {
		ID:          "mixtral",
		Name:        "Mixtral 8x7B",
		Provider:    "Local",
		Tier:        "Balanced",
		CostPer1K:   0.0,
		MaxTokens:   32768,
		Description: "MoE for complex reasoning",
	},
	"phi3": {
		ID:          "phi3",
		Name:        "Phi-3",
		Provider:    "Local",
		Tier:        "Fast",
		CostPer1K:   0.0,
		MaxTokens:   4096,
		Description: "Microsoft's compact efficient model",
	},
	"gemma2": {
		ID:          "gemma2",
		Name:        "Gemma 2",
		Provider:    "Local",
		Tier:        "Fast",
		CostPer1K:   0.0,
		MaxTokens:   8192,
		Description: "Google's lightweight model",
	},
}

// =============================================================================
// MODEL INFO METHODS
// =============================================================================

// CapabilitiesString returns a comma-separated list of model capabilities.
// Infers capabilities from model properties like context size and tier.
func (m ModelInfo) CapabilitiesString() string {
	caps := []string{}

	// Context window capability
	if m.MaxTokens >= 100000 {
		caps = append(caps, "Long context")
	} else if m.MaxTokens >= 32000 {
		caps = append(caps, "Extended context")
	}

	// Speed/latency capability
	if m.Tier == "Fast" {
		caps = append(caps, "Low latency")
	}

	// Cost capability
	if m.CostPer1K == 0 {
		caps = append(caps, "Free (local)")
	} else if m.CostPer1K < 0.001 {
		caps = append(caps, "Low cost")
	}

	// Provider-specific capabilities
	if m.Provider == "Local" {
		caps = append(caps, "Offline capable")
	}

	// Code-focused models
	if strings.Contains(strings.ToLower(m.Name), "code") ||
		strings.Contains(strings.ToLower(m.ID), "coder") {
		caps = append(caps, "Code optimized")
	}

	// Reasoning capability
	if m.Tier == "Powerful" || m.Tier == "Balanced" {
		caps = append(caps, "Complex reasoning")
	}

	if len(caps) == 0 {
		return "General purpose"
	}

	return strings.Join(caps, ", ")
}

// TierIcon returns an icon character for the model tier.
func (m ModelInfo) TierIcon() string {
	switch m.Tier {
	case "Fast":
		return "z" // Lightning for fast
	case "Balanced":
		return "~" // Scales for balanced
	case "Powerful":
		return "&" // Star for powerful
	default:
		return "?"
	}
}

// CostString returns a formatted cost string.
// Returns "Free" for local models, otherwise shows cost per 1K tokens.
func (m ModelInfo) CostString() string {
	if m.CostPer1K == 0 {
		return "Free"
	}
	if m.CostPer1K < 0.001 {
		return fmt.Sprintf("$%.5f/1K", m.CostPer1K)
	}
	return fmt.Sprintf("$%.4f/1K", m.CostPer1K)
}

// ContextString returns a formatted context window string.
func (m ModelInfo) ContextString() string {
	if m.MaxTokens >= 1000000 {
		return fmt.Sprintf("%.1fM tokens", float64(m.MaxTokens)/1000000)
	}
	if m.MaxTokens >= 1000 {
		return fmt.Sprintf("%dK tokens", m.MaxTokens/1000)
	}
	return fmt.Sprintf("%d tokens", m.MaxTokens)
}

// =============================================================================
// MODEL LOOKUP FUNCTIONS
// =============================================================================

// GetModelInfo looks up a model by short name or ID.
// Returns the ModelInfo and true if found, otherwise empty ModelInfo and false.
func GetModelInfo(nameOrID string) (ModelInfo, bool) {
	// Try direct lookup by short name
	if info, ok := Models[nameOrID]; ok {
		return info, true
	}

	// Try lookup by ID
	for _, info := range Models {
		if info.ID == nameOrID {
			return info, true
		}
	}

	// Try partial match on name
	lowerName := strings.ToLower(nameOrID)
	for _, info := range Models {
		if strings.Contains(strings.ToLower(info.Name), lowerName) {
			return info, true
		}
		if strings.Contains(strings.ToLower(info.ID), lowerName) {
			return info, true
		}
	}

	return ModelInfo{}, false
}

// GetModelsByProvider returns all models from a specific provider.
func GetModelsByProvider(provider string) []ModelInfo {
	result := []ModelInfo{}
	lowerProvider := strings.ToLower(provider)

	for _, info := range Models {
		if strings.ToLower(info.Provider) == lowerProvider {
			result = append(result, info)
		}
	}

	return result
}

// GetModelsByTier returns all models of a specific tier.
func GetModelsByTier(tier string) []ModelInfo {
	result := []ModelInfo{}
	lowerTier := strings.ToLower(tier)

	for _, info := range Models {
		if strings.ToLower(info.Tier) == lowerTier {
			result = append(result, info)
		}
	}

	return result
}

// GetLocalModels returns all local (free) models.
func GetLocalModels() []ModelInfo {
	return GetModelsByProvider("Local")
}

// GetCloudModels returns all cloud (paid) models.
func GetCloudModels() []ModelInfo {
	result := []ModelInfo{}

	for _, info := range Models {
		if info.Provider != "Local" {
			result = append(result, info)
		}
	}

	return result
}

// ModelShortNames returns a sorted slice of all model short names.
func ModelShortNames() []string {
	names := make([]string, 0, len(Models))
	for name := range Models {
		names = append(names, name)
	}
	return names
}
