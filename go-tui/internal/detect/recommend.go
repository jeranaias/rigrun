// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package detect provides GPU detection and model recommendation functionality.
// It retrieves GPU specifications and recommends appropriate AI models based on available VRAM.
package detect

import (
	"regexp"
	"sort"
	"strings"
)

// =============================================================================
// PERFORMANCE: Pre-compiled regex (compiled once at startup)
// =============================================================================

var (
	// Model parameter extraction patterns
	modelParamRegex = regexp.MustCompile(`(\d+(?:\.\d+)?)(b)`)
	numericBRegex   = regexp.MustCompile(`(\d+(?:\.\d+)?)b`)

	// Suffix patterns for model parameter extraction
	suffixRegexes = map[string]*regexp.Regexp{
		":1.1b": regexp.MustCompile(`:1\.1b`),
		":1.5b": regexp.MustCompile(`:1\.5b`),
		":1b":   regexp.MustCompile(`:1b`),
		":3b":   regexp.MustCompile(`:3b`),
		":7b":   regexp.MustCompile(`:7b`),
		":8b":   regexp.MustCompile(`:8b`),
		":13b":  regexp.MustCompile(`:13b`),
		":14b":  regexp.MustCompile(`:14b`),
		":16b":  regexp.MustCompile(`:16b`),
		":22b":  regexp.MustCompile(`:22b`),
		":30b":  regexp.MustCompile(`:30b`),
		":32b":  regexp.MustCompile(`:32b`),
		":33b":  regexp.MustCompile(`:33b`),
		":34b":  regexp.MustCompile(`:34b`),
		":70b":  regexp.MustCompile(`:70b`),
		":72b":  regexp.MustCompile(`:72b`),
	}

	// General patterns for model parameter extraction
	generalRegexes = map[string]*regexp.Regexp{
		"1.1b": regexp.MustCompile(`1\.1b`),
		"1.5b": regexp.MustCompile(`1\.5b`),
	}

	// Suffix values for parameter count extraction
	suffixValues = map[string]float64{
		":1.1b": 1.1,
		":1.5b": 1.5,
		":1b":   1.0,
		":3b":   3.0,
		":7b":   7.0,
		":8b":   8.0,
		":13b":  13.0,
		":14b":  14.0,
		":16b":  16.0,
		":22b":  22.0,
		":30b":  30.0,
		":32b":  32.0,
		":33b":  33.0,
		":34b":  34.0,
		":70b":  70.0,
		":72b":  72.0,
	}

	generalValues = map[string]float64{
		"1.1b": 1.1,
		"1.5b": 1.5,
	}
)

// ModelRecommendation represents a model recommendation with metadata.
type ModelRecommendation struct {
	ModelName   string `json:"model_name"`
	Description string `json:"description"`
	VRAMNeeded  int    `json:"vram_needed_mb"`
	Quality     string `json:"quality"` // "fast", "balanced", "best"
}

// ModelTiers maps model names to their quality tier.
// Quality tiers indicate the trade-off between speed and capability:
// - "fast": Optimized for speed, suitable for limited VRAM
// - "balanced": Good balance of speed and capability
// - "best": Highest capability, requires more VRAM
var ModelTiers = map[string]string{
	"tinyllama:1.1b":         "fast",
	"qwen2.5-coder:3b":       "fast",
	"qwen2.5-coder:7b":       "balanced",
	"qwen2.5-coder:14b":      "best",
	"qwen2.5-coder:32b":      "best",
	"qwen3-coder:30b":        "best",
	"deepseek-coder:33b":     "best",
	"deepseek-coder-v2:lite": "fast",
	"codestral:22b":          "balanced",
}

// qualityRank returns a numeric rank for sorting (higher = better quality)
func qualityRank(quality string) int {
	switch quality {
	case "best":
		return 3
	case "balanced":
		return 2
	case "fast":
		return 1
	default:
		return 0
	}
}

// RecommendModel recommends the optimal coding model based on available VRAM.
//
// Model recommendations are based on 2025 benchmarks:
// - Qwen 2.5 Coder excels on code repair tasks
// - 14B model is the sweet spot for 12-16GB GPUs
// - Qwen 3 Coder (MoE) offers best performance for high-VRAM systems
//
// Parameters:
//   - vramMB: Available VRAM in megabytes
//
// Returns:
//   - ModelRecommendation with the recommended model details
func RecommendModel(vramMB int) ModelRecommendation {
	vramGB := vramMB / 1024

	switch {
	case vramGB < 4:
		// Very limited VRAM: smallest capable model
		return ModelRecommendation{
			ModelName:   "tinyllama:1.1b",
			Description: "Minimal model for very limited VRAM",
			VRAMNeeded:  2000,
			Quality:     "fast",
		}
	case vramGB >= 4 && vramGB < 6:
		// 4-6GB class: 3B model works well
		return ModelRecommendation{
			ModelName:   "qwen2.5-coder:3b",
			Description: "Best for limited VRAM, fast inference",
			VRAMNeeded:  3500,
			Quality:     "fast",
		}
	case vramGB >= 6 && vramGB < 8:
		// 6-8GB class: 7B model fits
		return ModelRecommendation{
			ModelName:   "qwen2.5-coder:7b",
			Description: "Excellent all-rounder, near GPT-4o level",
			VRAMNeeded:  6000,
			Quality:     "balanced",
		}
	case vramGB >= 8 && vramGB < 12:
		// 8-12GB class: 7B model with comfortable headroom
		return ModelRecommendation{
			ModelName:   "qwen2.5-coder:7b",
			Description: "Balanced performance with comfortable VRAM headroom",
			VRAMNeeded:  6000,
			Quality:     "balanced",
		}
	case vramGB >= 12 && vramGB < 16:
		// 12-16GB class: 14B is the sweet spot
		return ModelRecommendation{
			ModelName:   "qwen2.5-coder:14b",
			Description: "Fastest (13s on 16GB GPU), excellent quality",
			VRAMNeeded:  10000,
			Quality:     "best",
		}
	case vramGB >= 16 && vramGB < 24:
		// 16-24GB class: 14B with headroom or 32B tight fit
		return ModelRecommendation{
			ModelName:   "qwen2.5-coder:14b",
			Description: "Best performance per VRAM with headroom",
			VRAMNeeded:  10000,
			Quality:     "best",
		}
	default:
		// 24GB+ class: Premium models
		return ModelRecommendation{
			ModelName:   "qwen2.5-coder:32b",
			Description: "Competitive with GPT-4o on code repair",
			VRAMNeeded:  20000,
			Quality:     "best",
		}
	}
}

// EstimateModelVRAM estimates the VRAM required for a model based on its parameter count.
//
// This is a rough estimate using the following heuristics:
// - Q4_K_M quantization uses approximately 0.5-0.6 bytes per parameter
// - Add ~1-2GB overhead for KV cache and CUDA context
//
// VRAM estimates are based on Q4_K_M quantization (most common in Ollama):
// - Q4_K_M uses roughly 4.5 bits per parameter = 0.5625 bytes per parameter
//
// Parameters:
//   - modelName: The model name (e.g., "qwen2.5-coder:14b")
//
// Returns:
//   - Estimated VRAM in MB
func EstimateModelVRAM(modelName string) int {
	nameLower := strings.ToLower(modelName)

	// Extract parameter count from model name using regex
	paramBillions := extractParamCount(nameLower)

	if paramBillions == 0 {
		// Default estimate for unknown models
		return 6000 // ~6GB default
	}

	// Estimate VRAM based on Q4_K_M quantization
	// Q4_K_M uses roughly 4.5 bits per parameter = 0.5625 bytes per parameter
	// Add ~1.5GB overhead for KV cache and CUDA context
	bytesPerParam := 0.56
	overheadGB := 1.5

	vramGB := (paramBillions * bytesPerParam) + overheadGB
	vramMB := int(vramGB * 1024)

	return vramMB
}

// extractParamCount extracts the parameter count in billions from a model name.
// Uses pre-compiled suffixRegexes, generalRegexes, and numericBRegex.
func extractParamCount(modelName string) float64 {
	// Check for specific suffixes first using pre-compiled regexes
	// Order matters - check more specific patterns first
	suffixOrder := []string{":1.1b", ":1.5b", ":1b", ":3b", ":7b", ":8b", ":13b", ":14b", ":16b", ":22b", ":30b", ":32b", ":33b", ":34b", ":70b", ":72b"}
	for _, suffix := range suffixOrder {
		if re, ok := suffixRegexes[suffix]; ok {
			if re.MatchString(modelName) {
				return suffixValues[suffix]
			}
		}
	}

	// Also check without colon prefix for names like "tinyllama1.1b"
	for key, re := range generalRegexes {
		if re.MatchString(modelName) {
			return generalValues[key]
		}
	}

	// Try to extract numeric pattern using pre-compiled regex
	matches := numericBRegex.FindStringSubmatch(modelName)
	if len(matches) >= 2 {
		// Try to parse as float
		var val float64
		if n, _ := strings.CutSuffix(matches[0], "b"); n != "" {
			// Simple parsing
			switch n {
			case "1", "1.0":
				val = 1.0
			case "1.1":
				val = 1.1
			case "1.5":
				val = 1.5
			case "3":
				val = 3.0
			case "7":
				val = 7.0
			case "8":
				val = 8.0
			case "13":
				val = 13.0
			case "14":
				val = 14.0
			case "16":
				val = 16.0
			case "22":
				val = 22.0
			case "30":
				val = 30.0
			case "32":
				val = 32.0
			case "33":
				val = 33.0
			case "34":
				val = 34.0
			case "70":
				val = 70.0
			case "72":
				val = 72.0
			}
			if val > 0 {
				return val
			}
		}
	}

	return 0
}

// WillModelFit checks if a model will likely fit in the available VRAM.
//
// Parameters:
//   - modelName: The model name (e.g., "qwen2.5-coder:14b")
//   - availableVRAM: Available VRAM in MB
//
// Returns:
//   - true if the model should fit (with 20% safety buffer), false otherwise
func WillModelFit(modelName string, availableVRAM int) bool {
	estimatedVRAM := EstimateModelVRAM(modelName)

	// Add 20% buffer for safety
	requiredWithBuffer := int(float64(estimatedVRAM) * 1.2)

	return requiredWithBuffer <= availableVRAM
}

// GetModelSizeFromName parses a model name to extract the parameter count.
// Uses pre-compiled modelParamRegex.
//
// Parameters:
//   - modelName: The model name (e.g., "qwen2.5-coder:7b")
//
// Returns:
//   - billions: The parameter count in billions (e.g., 7.0)
//   - suffix: The size suffix (e.g., "b")
func GetModelSizeFromName(modelName string) (float64, string) {
	nameLower := strings.ToLower(modelName)

	// Use pre-compiled regex to find parameter count pattern
	matches := modelParamRegex.FindStringSubmatch(nameLower)

	if len(matches) >= 3 {
		// Parse the number part
		numStr := matches[1]
		suffix := matches[2]

		// Map common values
		valueMap := map[string]float64{
			"1":   1.0,
			"1.0": 1.0,
			"1.1": 1.1,
			"1.5": 1.5,
			"3":   3.0,
			"7":   7.0,
			"8":   8.0,
			"13":  13.0,
			"14":  14.0,
			"16":  16.0,
			"22":  22.0,
			"30":  30.0,
			"32":  32.0,
			"33":  33.0,
			"34":  34.0,
			"70":  70.0,
			"72":  72.0,
		}

		if val, ok := valueMap[numStr]; ok {
			return val, suffix
		}
	}

	return 0, ""
}

// ListRecommendedModels returns all models that would fit in the available VRAM.
// Models are sorted by quality (best first).
//
// Parameters:
//   - vramMB: Available VRAM in megabytes
//
// Returns:
//   - A slice of ModelRecommendation sorted by quality (best first)
func ListRecommendedModels(vramMB int) []ModelRecommendation {
	// Define all available models with their VRAM requirements
	allModels := []ModelRecommendation{
		{
			ModelName:   "tinyllama:1.1b",
			Description: "Minimal model for very limited VRAM",
			VRAMNeeded:  2000,
			Quality:     "fast",
		},
		{
			ModelName:   "qwen2.5-coder:3b",
			Description: "Best for limited VRAM, fast inference",
			VRAMNeeded:  3500,
			Quality:     "fast",
		},
		{
			ModelName:   "deepseek-coder-v2:lite",
			Description: "Good alternative for limited VRAM",
			VRAMNeeded:  4000,
			Quality:     "fast",
		},
		{
			ModelName:   "qwen2.5-coder:7b",
			Description: "Excellent all-rounder, near GPT-4o level",
			VRAMNeeded:  6000,
			Quality:     "balanced",
		},
		{
			ModelName:   "qwen2.5-coder:14b",
			Description: "Fastest (13s on 16GB GPU), excellent quality",
			VRAMNeeded:  10000,
			Quality:     "best",
		},
		{
			ModelName:   "codestral:22b",
			Description: "22B sweet spot for code, best for autocomplete (FIM)",
			VRAMNeeded:  14000,
			Quality:     "balanced",
		},
		{
			ModelName:   "deepseek-coder-v2:16b",
			Description: "Strong debugging partner",
			VRAMNeeded:  11000,
			Quality:     "balanced",
		},
		{
			ModelName:   "qwen2.5-coder:32b",
			Description: "Competitive with GPT-4o on code repair",
			VRAMNeeded:  20000,
			Quality:     "best",
		},
		{
			ModelName:   "deepseek-coder:33b",
			Description: "Premium model for high-VRAM systems",
			VRAMNeeded:  20000,
			Quality:     "best",
		},
		{
			ModelName:   "qwen3-coder:30b",
			Description: "Latest MoE, 256K context",
			VRAMNeeded:  19000,
			Quality:     "best",
		},
	}

	// Filter models that fit with 20% buffer
	var fittingModels []ModelRecommendation
	for _, model := range allModels {
		requiredWithBuffer := int(float64(model.VRAMNeeded) * 1.2)
		if requiredWithBuffer <= vramMB {
			fittingModels = append(fittingModels, model)
		}
	}

	// Sort by quality (best first), then by VRAM needed (larger models first within same tier)
	sort.Slice(fittingModels, func(i, j int) bool {
		rankI := qualityRank(fittingModels[i].Quality)
		rankJ := qualityRank(fittingModels[j].Quality)
		if rankI != rankJ {
			return rankI > rankJ // Higher rank (better quality) first
		}
		// Within same quality tier, prefer larger models (more capable)
		return fittingModels[i].VRAMNeeded > fittingModels[j].VRAMNeeded
	})

	return fittingModels
}

// GetModelTier returns the quality tier for a given model name.
//
// Parameters:
//   - modelName: The model name (e.g., "qwen2.5-coder:14b")
//
// Returns:
//   - The quality tier ("fast", "balanced", or "best"), or "unknown" if not found
func GetModelTier(modelName string) string {
	// First check exact match
	if tier, ok := ModelTiers[modelName]; ok {
		return tier
	}

	// Check with lowercase
	nameLower := strings.ToLower(modelName)
	if tier, ok := ModelTiers[nameLower]; ok {
		return tier
	}

	// Try to infer from model size
	billions, _ := GetModelSizeFromName(modelName)
	switch {
	case billions <= 3:
		return "fast"
	case billions <= 8:
		return "balanced"
	case billions > 8:
		return "best"
	default:
		return "unknown"
	}
}

// GetAlternativeModels returns alternative model recommendations for the given VRAM.
// Provides options optimized for different use cases:
// - Fill-in-the-middle (autocomplete): Codestral
// - General coding: Qwen 2.5 Coder
// - Long context: Qwen 3 Coder
//
// Parameters:
//   - vramMB: Available VRAM in megabytes
//
// Returns:
//   - A slice of alternative ModelRecommendation (excluding the primary recommendation)
func GetAlternativeModels(vramMB int) []ModelRecommendation {
	allModels := ListRecommendedModels(vramMB)
	primary := RecommendModel(vramMB)

	var alternatives []ModelRecommendation
	for _, model := range allModels {
		if model.ModelName != primary.ModelName {
			alternatives = append(alternatives, model)
		}
	}

	return alternatives
}
