// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
// ollama.go provides conversion of tools to Ollama API format and
// model-specific tool calling support information.
package tools

import (
	"fmt"
	"strings"

	"github.com/jeranaias/rigrun-tui/internal/ollama"
)

// =============================================================================
// TOOL-SUPPORTED MODELS
// =============================================================================
// This list is based on Ollama's official documentation and the models page:
// https://ollama.com/search?c=tools
// https://docs.ollama.com/capabilities/tool-calling
//
// Models are listed by their base name (without size/quantization suffix).
// The SupportsTools function handles matching with size variants.

// ToolSupportLevel indicates how well a model supports tool calling.
type ToolSupportLevel int

const (
	// ToolSupportNone - Model does not support tool calling
	ToolSupportNone ToolSupportLevel = iota

	// ToolSupportBasic - Model supports basic tool calling but may have issues
	ToolSupportBasic

	// ToolSupportGood - Model has reliable tool calling support
	ToolSupportGood

	// ToolSupportExcellent - Model is optimized for tool/function calling
	ToolSupportExcellent
)

// String returns the string representation of a tool support level.
func (t ToolSupportLevel) String() string {
	switch t {
	case ToolSupportNone:
		return "None"
	case ToolSupportBasic:
		return "Basic"
	case ToolSupportGood:
		return "Good"
	case ToolSupportExcellent:
		return "Excellent"
	default:
		return "Unknown"
	}
}

// SizeTier categorizes models by parameter count for routing decisions.
type SizeTier int

const (
	SizeTiny   SizeTier = iota // < 3B params
	SizeSmall                  // 3B-7B params
	SizeMedium                 // 7B-14B params
	SizeLarge                  // 14B-32B params
	SizeXLarge                 // 32B+ params
)

// String returns human-readable size tier.
func (s SizeTier) String() string {
	switch s {
	case SizeTiny:
		return "Tiny (<3B)"
	case SizeSmall:
		return "Small (3-7B)"
	case SizeMedium:
		return "Medium (7-14B)"
	case SizeLarge:
		return "Large (14-32B)"
	case SizeXLarge:
		return "XLarge (32B+)"
	default:
		return "Unknown"
	}
}

// MinParamsB returns minimum params in billions for this tier.
func (s SizeTier) MinParamsB() float64 {
	switch s {
	case SizeTiny:
		return 0
	case SizeSmall:
		return 3
	case SizeMedium:
		return 7
	case SizeLarge:
		return 14
	case SizeXLarge:
		return 32
	default:
		return 0
	}
}

// AgenticCapability indicates how well a model handles agentic workflows.
type AgenticCapability int

const (
	AgenticNone   AgenticCapability = iota // Cannot do agentic tasks
	AgenticBasic                           // Simple 1-2 step tasks only
	AgenticGood                            // Multi-step with guidance
	AgenticFull                            // Full autonomous agent capability
)

// String returns human-readable agentic capability.
func (a AgenticCapability) String() string {
	switch a {
	case AgenticNone:
		return "None"
	case AgenticBasic:
		return "Basic (1-2 steps)"
	case AgenticGood:
		return "Good (multi-step)"
	case AgenticFull:
		return "Full (autonomous)"
	default:
		return "Unknown"
	}
}

// ModelToolInfo contains tool-calling information for a specific model.
type ModelToolInfo struct {
	// SupportLevel indicates how well the model supports tools
	SupportLevel ToolSupportLevel

	// RecommendedTemperature for tool calling (lower = more reliable)
	RecommendedTemperature float64

	// RecommendedContextSize for tool calling operations
	RecommendedContextSize int

	// SupportsParallelCalls indicates if model can request multiple tools at once
	SupportsParallelCalls bool

	// SupportsStreaming indicates if model supports streaming with tools
	SupportsStreaming bool

	// MinSizeForAgentic is the minimum size tier recommended for agentic use
	MinSizeForAgentic SizeTier

	// AgenticCap indicates agentic workflow capability at recommended size
	AgenticCap AgenticCapability

	// Notes contains any model-specific considerations
	Notes string
}

// toolSupportedModels maps model family names to their tool support info.
// These are the officially supported models from Ollama's documentation.
// MinSizeForAgentic and AgenticCap are based on empirical testing and research.
var toolSupportedModels = map[string]ModelToolInfo{
	// Llama family - Meta's flagship models with strong tool support
	"llama3.1": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeMedium,
		AgenticCap:             AgenticFull,
		Notes:                  "Best overall choice for function calling. 8B+ recommended for agentic tasks.",
	},
	"llama3.2": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeMedium,
		AgenticCap:             AgenticGood,
		Notes:                  "3B can do basic tools; 8B+ for multi-step agentic. Output format differs from 3.1.",
	},
	"llama3.3": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeLarge,
		AgenticCap:             AgenticFull,
		Notes:                  "70B model with full autonomous agent capability.",
	},

	// Qwen family - Alibaba's models with excellent tool support
	"qwen2": {
		SupportLevel:           ToolSupportGood,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeMedium,
		AgenticCap:             AgenticGood,
		Notes:                  "Good tool support. 7B+ for reliable agentic use.",
	},
	"qwen2.5": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeSmall,
		AgenticCap:             AgenticGood,
		Notes:                  "Excellent tool support. 3B can do basic agentic; 7B+ for complex tasks.",
	},
	"qwen2.5-coder": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeSmall,
		AgenticCap:             AgenticFull,
		Notes:                  "Best for coding agentic tasks. 7B is sweet spot; 14B+ for complex analysis.",
	},
	"qwen3": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeSmall,
		AgenticCap:             AgenticFull,
		Notes:                  "Latest Qwen with streaming tool support. Strong agentic at 7B+.",
	},
	"qwen3-coder": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 65536,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeMedium,
		AgenticCap:             AgenticFull,
		Notes:                  "Optimized for agentic coding. 14B+ recommended for full capability.",
	},

	// Mistral family - Strong tool support with lower resource requirements
	"mistral": {
		SupportLevel:           ToolSupportGood,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeMedium,
		AgenticCap:             AgenticGood,
		Notes:                  "7B model with good function calling. Decent agentic capability.",
	},
	"mistral-nemo": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeMedium,
		AgenticCap:             AgenticFull,
		Notes:                  "12B model optimized for tool use. Great for agentic workflows.",
	},
	"mistral-small": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeLarge,
		AgenticCap:             AgenticFull,
		Notes:                  "22-24B with excellent agentic capability. Recommended for complex tasks.",
	},
	"mixtral": {
		SupportLevel:           ToolSupportGood,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeLarge,
		AgenticCap:             AgenticGood,
		Notes:                  "MoE model - fast inference. Good for moderate agentic tasks.",
	},
	"ministral": {
		SupportLevel:           ToolSupportGood,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeSmall,
		AgenticCap:             AgenticBasic,
		Notes:                  "3B-8B range. Basic agentic only; use larger models for complex tasks.",
	},

	// Command-R family - Cohere's models for RAG and tool use
	"command-r": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeLarge,
		AgenticCap:             AgenticFull,
		Notes:                  "35B optimized for RAG and agentic. CC-BY-NC license.",
	},
	"command-r-plus": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeXLarge,
		AgenticCap:             AgenticFull,
		Notes:                  "104B with excellent agentic capability. CC-BY-NC license.",
	},

	// Specialized function-calling models
	"firefunction-v2": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 8192,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeMedium,
		AgenticCap:             AgenticGood,
		Notes:                  "7B specialized for function calling. Good for simple agentic.",
	},
	"functiongemma": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 8192,
		SupportsParallelCalls:  false,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeTiny,
		AgenticCap:             AgenticBasic,
		Notes:                  "270M specialized for function calling. Single-step only.",
	},

	// Granite family - IBM's models
	"granite3.2": {
		SupportLevel:           ToolSupportGood,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeMedium,
		AgenticCap:             AgenticGood,
		Notes:                  "IBM's 8B model with tool calling. Good for moderate tasks.",
	},
	"granite3.2-vision": {
		SupportLevel:           ToolSupportGood,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeMedium,
		AgenticCap:             AgenticGood,
		Notes:                  "Vision model with tool support for multimodal agentic.",
	},

	// DeepSeek family
	"deepseek-r1": {
		SupportLevel:           ToolSupportGood,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeMedium,
		AgenticCap:             AgenticGood,
		Notes:                  "Reasoning model. 7B+ distill variants good for agentic reasoning.",
	},
	"deepseek-coder-v2": {
		SupportLevel:           ToolSupportGood,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeLarge,
		AgenticCap:             AgenticFull,
		Notes:                  "16B+ code-focused with strong agentic capability.",
	},

	// Other supported models
	"smollm2": {
		SupportLevel:           ToolSupportBasic,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 8192,
		SupportsParallelCalls:  false,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeTiny,
		AgenticCap:             AgenticNone,
		Notes:                  "135M-1.7B. NOT recommended for agentic tasks.",
	},
	"devstral-small-2": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeLarge,
		AgenticCap:             AgenticFull,
		Notes:                  "24B optimized for code exploration. Excellent agentic.",
	},
	"devstral-2": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeXLarge,
		AgenticCap:             AgenticFull,
		Notes:                  "123B software engineering agent. Full autonomous capability.",
	},
	"nemotron-3-nano": {
		SupportLevel:           ToolSupportExcellent,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		MinSizeForAgentic:      SizeLarge,
		AgenticCap:             AgenticFull,
		Notes:                  "NVIDIA's 30B agentic model. Designed for agent workflows.",
	},
	"gpt-oss": {
		SupportLevel:           ToolSupportGood,
		RecommendedTemperature: 0.1,
		RecommendedContextSize: 32768,
		SupportsParallelCalls:  true,
		SupportsStreaming:      true,
		Notes:                  "20B-120B model for reasoning and agentic tasks. May need XML format for some variants.",
	},
}

// =============================================================================
// TOOL SUPPORT CHECKING
// =============================================================================

// SupportsTools checks if a model supports tool calling.
// It handles model names with size suffixes (e.g., "llama3.1:8b", "qwen2.5-coder:14b").
func SupportsTools(modelName string) bool {
	return GetToolSupportLevel(modelName) != ToolSupportNone
}

// GetToolSupportLevel returns the tool support level for a model.
func GetToolSupportLevel(modelName string) ToolSupportLevel {
	info := GetModelToolInfo(modelName)
	return info.SupportLevel
}

// GetModelToolInfo returns detailed tool information for a model.
// Returns default info with ToolSupportNone if model is not in the supported list.
func GetModelToolInfo(modelName string) ModelToolInfo {
	// Normalize model name: lowercase and remove size/quantization suffix
	normalized := normalizeModelName(modelName)

	// Check for exact match first
	if info, ok := toolSupportedModels[normalized]; ok {
		return info
	}

	// Check for prefix match (handles variants like "llama3.1-instruct")
	for family, info := range toolSupportedModels {
		if strings.HasPrefix(normalized, family) {
			return info
		}
	}

	// Return default (no support)
	return ModelToolInfo{
		SupportLevel:           ToolSupportNone,
		RecommendedTemperature: 0.7,
		RecommendedContextSize: 4096,
		SupportsParallelCalls:  false,
		SupportsStreaming:      false,
		Notes:                  "Model not in supported tools list. Tool calling may not work correctly.",
	}
}

// normalizeModelName extracts the base model family name from a full model name.
// Examples:
//   - "llama3.1:8b" -> "llama3.1"
//   - "qwen2.5-coder:14b-instruct-q4_K_M" -> "qwen2.5-coder"
//   - "mistral-nemo:12b" -> "mistral-nemo"
func normalizeModelName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Remove tag/size suffix after colon
	if idx := strings.Index(name, ":"); idx != -1 {
		name = name[:idx]
	}

	return name
}

// GetSupportedModelsList returns a list of all model families that support tools.
func GetSupportedModelsList() []string {
	models := make([]string, 0, len(toolSupportedModels))
	for name := range toolSupportedModels {
		models = append(models, name)
	}
	return models
}

// GetRecommendedModelsForTools returns models with Excellent tool support.
func GetRecommendedModelsForTools() []string {
	var recommended []string
	for name, info := range toolSupportedModels {
		if info.SupportLevel == ToolSupportExcellent {
			recommended = append(recommended, name)
		}
	}
	return recommended
}

// =============================================================================
// MODEL-SPECIFIC OPTIONS
// =============================================================================

// GetToolCallingOptions returns recommended Ollama options for tool calling
// based on the model being used.
func GetToolCallingOptions(modelName string) *ollama.Options {
	info := GetModelToolInfo(modelName)

	return &ollama.Options{
		Temperature: info.RecommendedTemperature,
		NumCtx:      info.RecommendedContextSize,
		// Lower top_p and top_k for more deterministic tool selection
		TopP: 0.9,
		TopK: 40,
	}
}

// ToOllamaTools converts a Registry's tools to Ollama API format.
func (r *Registry) ToOllamaTools() []ollama.Tool {
	tools := r.All()
	result := make([]ollama.Tool, 0, len(tools))

	for _, tool := range tools {
		result = append(result, ToolToOllama(tool))
	}

	return result
}

// ToolToOllama converts a single Tool to Ollama API format.
// The conversion follows the JSON Schema format expected by Ollama's tool calling API:
//
//	{
//	  "type": "function",
//	  "function": {
//	    "name": "tool_name",
//	    "description": "What the tool does",
//	    "parameters": {
//	      "type": "object",
//	      "properties": {
//	        "param_name": {
//	          "type": "string",
//	          "description": "Parameter description",
//	          "default": "optional_default_value"
//	        }
//	      },
//	      "required": ["param_name"]
//	    }
//	  }
//	}
func ToolToOllama(tool *Tool) ollama.Tool {
	properties := make(map[string]ollama.ToolProperty)
	var required []string

	for _, param := range tool.Schema.Parameters {
		prop := ollama.ToolProperty{
			Type:        param.Type,
			Description: param.Description,
		}

		// Include default value if specified (helps models understand optional params)
		if param.Default != nil {
			prop.Default = param.Default
		}

		// Include enum values if specified (for constrained string choices)
		if len(param.Enum) > 0 {
			prop.Enum = param.Enum
		}

		properties[param.Name] = prop

		if param.Required {
			required = append(required, param.Name)
		}
	}

	return ollama.Tool{
		Type: "function",
		Function: ollama.ToolSchema{
			Name:        tool.Name,
			Description: tool.GetShortDescription(), // Use short description for LLM schemas
			Parameters: ollama.ToolParameters{
				Type:       "object",
				Properties: properties,
				Required:   required,
			},
		},
	}
}

// OllamaToolCallToToolCall converts an Ollama tool call to our internal format.
func OllamaToolCallToToolCall(call ollama.ToolCall) *ToolCall {
	return &ToolCall{
		Name:   call.Function.Name,
		Params: call.Function.Arguments,
	}
}

// OllamaToolCallsToToolCalls converts multiple Ollama tool calls.
func OllamaToolCallsToToolCalls(calls []ollama.ToolCall) []*ToolCall {
	result := make([]*ToolCall, len(calls))
	for i, call := range calls {
		result[i] = OllamaToolCallToToolCall(call)
	}
	return result
}

// ToOllamaMessage converts a tools.Message to an ollama.Message.
func MessageToOllama(msg Message) ollama.Message {
	ollamaMsg := ollama.Message{
		Role:    msg.Role,
		Content: msg.Content,
	}

	// Convert tool calls if present
	if len(msg.ToolCalls) > 0 {
		ollamaMsg.ToolCalls = make([]ollama.ToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			ollamaMsg.ToolCalls[i] = ollama.ToolCall{
				Function: ollama.ToolFunction{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			}
		}
	}

	return ollamaMsg
}

// MessagesToOllama converts a slice of tools.Message to ollama.Message.
func MessagesToOllama(msgs []Message) []ollama.Message {
	result := make([]ollama.Message, len(msgs))
	for i, msg := range msgs {
		result[i] = MessageToOllama(msg)
	}
	return result
}

// OllamaMessageToMessage converts an ollama.Message to tools.Message.
func OllamaMessageToMessage(msg ollama.Message) Message {
	toolsMsg := Message{
		Role:    msg.Role,
		Content: msg.Content,
	}

	// Convert tool calls if present
	if len(msg.ToolCalls) > 0 {
		toolsMsg.ToolCalls = make([]ToolCallMessage, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			toolsMsg.ToolCalls[i] = ToolCallMessage{
				ID:        generateCallID(), // Generate ID since Ollama doesn't provide one
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			}
		}
	}

	return toolsMsg
}

// =============================================================================
// TOOL CALLING TROUBLESHOOTING
// =============================================================================
// Common issues and their solutions based on Ollama community feedback:
//
// 1. MODEL NOT CALLING TOOLS
//    - Ensure model is in the supported list (use SupportsTools())
//    - Use lower temperature (0.1 recommended for tool calling)
//    - Use larger context window (32K+ recommended)
//    - Check that tools array is properly formatted
//
// 2. TOOLS WORKING BUT BREAKING NORMAL RESPONSES
//    - Some models always use tools when tools are provided
//    - Consider removing tools array for non-tool conversations
//    - Use model-specific workarounds if needed
//
// 3. THINKING MODELS + TOOL CALLING ISSUES
//    - Models like Qwen3 with "thinking" may lose context between tool calls
//    - Preserve thinking content in conversation history
//    - Consider using non-thinking variants for tool-heavy workloads
//
// 4. EXTENDED SESSION ISSUES (OUTPUT SLIPPING)
//    - Happens with long conversations across multiple models
//    - May be templating issues - ensure proper message history
//    - Consider periodic context summarization
//
// 5. JSON PARSING ISSUES
//    - Llama 3.2 tool outputs are not pure JSON (Ollama handles this)
//    - Some models output tool calls without prefix at start of response
//    - Ollama's parser falls back to JSON detection in these cases

// ValidateToolsForModel checks if tools can be used with the given model
// and returns any warnings or recommendations.
func ValidateToolsForModel(modelName string, tools []ollama.Tool) []string {
	var warnings []string

	info := GetModelToolInfo(modelName)

	if info.SupportLevel == ToolSupportNone {
		warnings = append(warnings,
			"WARNING: Model '"+modelName+"' is not in the supported tools list. Tool calling may not work correctly.")
		warnings = append(warnings,
			"RECOMMENDATION: Consider using one of: llama3.1, llama3.2, qwen2.5, mistral-nemo, command-r")
	}

	if info.SupportLevel == ToolSupportBasic {
		warnings = append(warnings,
			"NOTE: Model '"+modelName+"' has basic tool support. Results may be inconsistent.")
	}

	if len(tools) > 5 && !info.SupportsParallelCalls {
		warnings = append(warnings,
			"WARNING: Model may not handle many tools well. Consider reducing tool count.")
	}

	// Check for overly long descriptions
	for _, tool := range tools {
		if len(tool.Function.Description) > 125 {
			warnings = append(warnings,
				"NOTE: Tool '"+tool.Function.Name+"' has a long description (>125 chars). Consider shortening.")
		}
	}

	if info.Notes != "" {
		warnings = append(warnings, "MODEL NOTE: "+info.Notes)
	}

	return warnings
}

// ToolCallingTips returns general tips for using tool calling effectively.
func ToolCallingTips() []string {
	return []string{
		"Use temperature 0.1 for more reliable tool selection",
		"Use context window of 32K+ for better tool calling performance",
		"Keep tool descriptions concise (<125 characters recommended)",
		"Test with simple single-tool scenarios before complex multi-tool setups",
		"For thinking models, preserve thinking content in conversation history",
		"Enable OLLAMA_DEBUG=1 for detailed logging when troubleshooting",
		"Use 'ollama show <model>' to verify model has tool support template",
	}
}

// =============================================================================
// TOOL-AWARE SYSTEM PROMPT
// =============================================================================

// GenerateToolSystemPrompt creates a system prompt that helps the model understand
// when and how to use available tools. This is especially helpful for models that
// may not have strong native tool support.
func GenerateToolSystemPrompt(registry *Registry) string {
	if registry == nil {
		return ""
	}

	allTools := registry.All()
	if len(allTools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("You are a helpful AI assistant with access to tools.\n\n")
	sb.WriteString("## Available Tools\n\n")
	sb.WriteString("You have access to the following tools that you can use to help answer questions:\n\n")

	for _, tool := range allTools {
		sb.WriteString("### ")
		sb.WriteString(tool.Name)
		sb.WriteString("\n")
		sb.WriteString(tool.GetShortDescription())
		sb.WriteString("\n\n")
	}

	sb.WriteString("## When to Use Tools\n\n")
	sb.WriteString("- Use **WebSearch** when you need current information, documentation, or facts you're uncertain about\n")
	sb.WriteString("- Use **Read** to read file contents when the user asks about files\n")
	sb.WriteString("- Use **Glob** to find files matching patterns\n")
	sb.WriteString("- Use **Grep** to search for text patterns in files\n")
	sb.WriteString("- Use **Bash** to execute shell commands (with caution)\n\n")
	sb.WriteString("## Important Guidelines\n\n")
	sb.WriteString("1. If you're unsure about current information, use WebSearch to verify\n")
	sb.WriteString("2. Always explain what you're doing when using tools\n")
	sb.WriteString("3. After receiving tool results, incorporate them into your response\n")
	sb.WriteString("4. If a tool fails, explain the error and try an alternative approach\n")

	return sb.String()
}

// GenerateMinimalToolPrompt creates a shorter system prompt for models with
// limited context windows or strong native tool support.
func GenerateMinimalToolPrompt() string {
	return `You are a helpful AI assistant with tool access. Use tools when needed:
- WebSearch: Search the web for current information
- Read/Glob/Grep: Read and search files
- Bash: Execute shell commands

Use tools proactively when you need information you don't have.`
}

// =============================================================================
// SMALL MODEL OPTIMIZATIONS
// =============================================================================

// GenerateSmallModelPrompt creates a highly structured system prompt optimized
// for small models (3B-7B parameters) that need explicit guidance.
func GenerateSmallModelPrompt() string {
	return `# ASSISTANT INSTRUCTIONS

You are a precise, helpful assistant. Follow these rules EXACTLY:

## RESPONSE RULES
1. Be CONCISE - no fluff, no filler
2. Use STRUCTURED formats (lists, headers, code blocks)
3. Answer the SPECIFIC question asked
4. If unsure, say "I don't know" - don't guess

## TOOL USAGE
When you need information:
- Files: Use Read, Glob, Grep tools
- Web: Use WebSearch tool
- Commands: Use Bash tool

## OUTPUT FORMAT
- Use markdown formatting
- Code in triple backticks with language
- Lists for multiple items
- Headers for sections

## CONSTRAINTS
- Stay on topic
- One task at a time
- No unnecessary explanations`
}

// GenerateAgenticExplorerPrompt creates a prompt for agentic exploration tasks
// where the model should use tools to discover and analyze.
func GenerateAgenticExplorerPrompt(taskDescription string) string {
	return fmt.Sprintf(`# EXPLORATION TASK

## YOUR MISSION
%s

## HOW TO EXPLORE
1. Use Glob to find relevant files
2. Use Grep to search for patterns
3. Use Read to examine specific files
4. Synthesize what you learn

## TOOL SEQUENCE
Step 1: Glob("**/*.py") or similar to find files
Step 2: Grep("keyword") to find relevant content
Step 3: Read specific files that look important
Step 4: Report your findings

## OUTPUT FORMAT
After exploring, provide:
- What you found (bullet points)
- Key files discovered
- Your analysis

START EXPLORING NOW.`, taskDescription)
}

// GenerateStructuredTaskPrompt creates a prompt for tasks that need specific output format.
func GenerateStructuredTaskPrompt(task string, outputFormat string, constraints []string) string {
	var sb strings.Builder
	sb.WriteString("# TASK\n\n")
	sb.WriteString(task)
	sb.WriteString("\n\n# REQUIRED OUTPUT FORMAT\n\n")
	sb.WriteString(outputFormat)

	if len(constraints) > 0 {
		sb.WriteString("\n\n# CONSTRAINTS\n\n")
		for i, c := range constraints {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, c))
		}
	}

	sb.WriteString("\n\n# BEGIN\n\nProvide your response now:")
	return sb.String()
}

// GenerateAgenticLoopPrompt creates a system prompt optimized for agentic tool loops.
// This prompt teaches small models HOW to use tools and interpret results.
func GenerateAgenticLoopPrompt() string {
	return `# AGENTIC ASSISTANT

You complete tasks using tools. STAY FOCUSED on the user's question.

## TOOLS
- Glob: Find files. {"name":"Glob","arguments":{"pattern":"*.md","path":"/dir"}}
- Grep: Search text. {"name":"Grep","arguments":{"pattern":"word","path":"/dir"}}
- Read: Read file. {"name":"Read","arguments":{"file_path":"/path"}}
- Bash: Run command. {"name":"Bash","arguments":{"command":"ls"}}

## WORKFLOW
1. UNDERSTAND the user's question
2. Call ONE tool to get information
3. ANALYZE the result
4. If you can answer -> WRITE YOUR ANSWER (no JSON)
5. If you need more info -> call another tool

## CRITICAL RULES
- STAY ON TASK - only gather info needed to answer
- When you have the answer -> STOP calling tools and WRITE IT
- Count items? -> Count the lines in tool output
- Looking for files? -> Glob first, then Read if needed
- Your FINAL response must be plain text, NOT a tool call

## ANSWER FORMAT
When ready to answer, write in plain text:
"Based on my search, I found [X]. The answer is [Y]."

DO NOT output JSON in your final answer.`
}

// GenerateREADMEPrompt creates an optimized prompt for README generation.
func GenerateREADMEPrompt(projectName, projectType string, modules []string) string {
	moduleList := strings.Join(modules, ", ")
	return fmt.Sprintf(`# GENERATE README

## PROJECT INFO
- Name: %s
- Type: %s
- Modules: %s

## REQUIRED SECTIONS
1. Title with project name
2. One-paragraph description (2-3 sentences MAX)
3. Key features (5 bullet points MAX)
4. Quick start (installation commands)

## RULES
- Use the EXACT project name provided
- Be CONCISE
- Use markdown formatting
- NO placeholder text
- NO made-up URLs

## OUTPUT
Generate the README now:`, projectName, projectType, moduleList)
}

// =============================================================================
// AGENTIC CAPABILITY HELPERS
// =============================================================================

// ParseModelSize extracts size in billions from model name like "llama3.2:3b" or "qwen2.5-coder:14b".
func ParseModelSize(modelName string) float64 {
	// Extract size suffix like "3b", "7b", "14b", "70b"
	lower := strings.ToLower(modelName)

	// Common patterns: model:3b, model:7b, model-3b, etc.
	patterns := []string{":1b", ":1.5b", ":3b", ":7b", ":8b", ":13b", ":14b", ":22b", ":32b", ":70b", ":72b", ":123b"}
	sizes := []float64{1, 1.5, 3, 7, 8, 13, 14, 22, 32, 70, 72, 123}

	for i, p := range patterns {
		if strings.Contains(lower, p) {
			return sizes[i]
		}
	}

	// Try dash patterns too
	dashPatterns := []string{"-1b", "-1.5b", "-3b", "-7b", "-8b", "-13b", "-14b", "-22b", "-32b", "-70b", "-72b", "-123b"}
	for i, p := range dashPatterns {
		if strings.Contains(lower, p) {
			return sizes[i]
		}
	}

	// Default: assume 7B if can't parse
	return 7.0
}

// GetModelFamily extracts the model family from a full model name.
// E.g., "llama3.2:3b" -> "llama3.2", "qwen2.5-coder:14b" -> "qwen2.5-coder"
func GetModelFamily(modelName string) string {
	// Remove size suffix (e.g., ":3b", ":7b")
	name := strings.ToLower(modelName)

	// Split on colon to remove tag
	if idx := strings.Index(name, ":"); idx != -1 {
		name = name[:idx]
	}

	// Check for known families
	families := []string{
		"llama3.3", "llama3.2", "llama3.1", "llama3",
		"qwen3-coder", "qwen3", "qwen2.5-coder", "qwen2.5", "qwen2",
		"mistral-small", "mistral-nemo", "mistral", "mixtral", "ministral",
		"command-r-plus", "command-r",
		"deepseek-coder-v2", "deepseek-r1",
		"granite3.2-vision", "granite3.2",
		"devstral-small-2", "devstral-2",
		"nemotron-3-nano",
		"firefunction-v2", "functiongemma",
		"smollm2",
	}

	for _, f := range families {
		if strings.HasPrefix(name, f) {
			return f
		}
	}

	// Return the base name
	return name
}

// GetSizeTier returns the size tier for a given model size in billions.
func GetSizeTier(sizeB float64) SizeTier {
	switch {
	case sizeB < 3:
		return SizeTiny
	case sizeB < 7:
		return SizeSmall
	case sizeB < 14:
		return SizeMedium
	case sizeB < 32:
		return SizeLarge
	default:
		return SizeXLarge
	}
}

// AgenticRecommendation contains advice for agentic model selection.
type AgenticRecommendation struct {
	Suitable       bool
	Warning        string
	Recommendation string
	SuggestModel   string
}

// CheckAgenticCapability checks if a model is suitable for agentic tasks.
func CheckAgenticCapability(modelName string) AgenticRecommendation {
	sizeB := ParseModelSize(modelName)
	sizeTier := GetSizeTier(sizeB)

	// Get model family info
	family := GetModelFamily(modelName)
	info, hasInfo := toolSupportedModels[family]

	rec := AgenticRecommendation{
		Suitable: true,
	}

	// Check tool support
	if hasInfo && info.SupportLevel == ToolSupportNone {
		rec.Suitable = false
		rec.Warning = fmt.Sprintf("Model %s does not support tool calling", modelName)
		rec.Recommendation = "Use a tool-capable model like qwen2.5-coder or llama3.2"
		rec.SuggestModel = "qwen2.5-coder:7b"
		return rec
	}

	// Check size requirements
	if hasInfo {
		if sizeTier < info.MinSizeForAgentic {
			rec.Suitable = false
			rec.Warning = fmt.Sprintf("Model %s (%.0fB) is below recommended size for agentic tasks", modelName, sizeB)
			rec.Recommendation = fmt.Sprintf("Use %s or larger for reliable agentic workflows", info.MinSizeForAgentic.String())

			// Suggest specific model
			switch family {
			case "llama3.2":
				rec.SuggestModel = "llama3.1:8b"
			case "qwen2.5", "qwen2.5-coder":
				rec.SuggestModel = "qwen2.5-coder:7b"
			default:
				rec.SuggestModel = "qwen2.5-coder:7b"
			}
			return rec
		}

		// Check agentic capability
		if info.AgenticCap == AgenticNone {
			rec.Suitable = false
			rec.Warning = fmt.Sprintf("Model %s is not capable of agentic tasks", modelName)
			rec.Recommendation = "Use a model with agentic capability"
			rec.SuggestModel = "qwen2.5-coder:7b"
			return rec
		}

		if info.AgenticCap == AgenticBasic {
			rec.Suitable = true
			rec.Warning = fmt.Sprintf("Model %s has basic agentic capability (1-2 steps only)", modelName)
			rec.Recommendation = "For complex multi-step tasks, consider upgrading"
			rec.SuggestModel = "qwen2.5-coder:14b"
		}
	} else {
		// Unknown model family - use size heuristics
		if sizeTier < SizeMedium {
			rec.Suitable = false
			rec.Warning = fmt.Sprintf("Model %s (%.0fB) may struggle with agentic tasks", modelName, sizeB)
			rec.Recommendation = "7B+ recommended for agentic workflows"
			rec.SuggestModel = "qwen2.5-coder:7b"
		}
	}

	return rec
}

// GenerateModelGuide generates a formatted model capability guide.
func GenerateModelGuide() string {
	var sb strings.Builder

	sb.WriteString("# Model Capability Guide\n\n")
	sb.WriteString("## Size Tiers\n")
	sb.WriteString("| Tier | Params | Best For |\n")
	sb.WriteString("|------|--------|----------|\n")
	sb.WriteString("| Tiny | <3B | Simple Q&A, classification |\n")
	sb.WriteString("| Small | 3-7B | Basic tasks, 1-2 tool calls |\n")
	sb.WriteString("| Medium | 7-14B | Multi-step tasks, coding |\n")
	sb.WriteString("| Large | 14-32B | Complex analysis, full agentic |\n")
	sb.WriteString("| XLarge | 32B+ | Expert tasks, long context |\n\n")

	sb.WriteString("## Agentic Capability Levels\n")
	sb.WriteString("| Level | Description |\n")
	sb.WriteString("|-------|-------------|\n")
	sb.WriteString("| None | Cannot use tools reliably |\n")
	sb.WriteString("| Basic | 1-2 step tasks, needs guidance |\n")
	sb.WriteString("| Good | Multi-step with occasional drift |\n")
	sb.WriteString("| Full | Autonomous agent capability |\n\n")

	sb.WriteString("## Recommended Models by Task\n\n")
	sb.WriteString("### Simple Queries (no tools)\n")
	sb.WriteString("- llama3.2:3b - Fast, good quality\n")
	sb.WriteString("- qwen2.5:3b - Excellent instruction following\n\n")

	sb.WriteString("### Basic Tool Use (1-2 calls)\n")
	sb.WriteString("- qwen2.5-coder:7b - Best balance of speed/capability\n")
	sb.WriteString("- mistral:7b - Good general purpose\n\n")

	sb.WriteString("### Complex Agentic Tasks\n")
	sb.WriteString("- qwen2.5-coder:14b - Sweet spot for coding agents\n")
	sb.WriteString("- mistral-small:22b - Excellent multi-step reasoning\n")
	sb.WriteString("- deepseek-coder-v2:16b - Strong code analysis\n\n")

	sb.WriteString("### Full Autonomous Agents\n")
	sb.WriteString("- llama3.3:70b - Maximum capability\n")
	sb.WriteString("- qwen2.5-coder:32b - Expert coding agent\n")
	sb.WriteString("- command-r-plus:104b - RAG specialist\n\n")

	sb.WriteString("## Key Insights\n\n")
	sb.WriteString("1. **Small models (3B) can use tools** but need explicit guidance\n")
	sb.WriteString("2. **7B is the practical minimum** for multi-step agentic\n")
	sb.WriteString("3. **14B+ recommended** for complex analysis tasks\n")
	sb.WriteString("4. **Qwen and Llama families** have best tool support\n")
	sb.WriteString("5. **Task decomposition** helps small models succeed\n")

	return sb.String()
}

// GenerateModelTable generates a concise model comparison table.
func GenerateModelTable() string {
	var sb strings.Builder

	sb.WriteString("Model              | Tools | Agentic | Notes\n")
	sb.WriteString("-------------------|-------|---------|------\n")

	// Key models only
	models := []struct {
		name    string
		tools   string
		agentic string
		notes   string
	}{
		{"llama3.2:3b", "+", "Basic", "Fast, needs guidance"},
		{"qwen2.5:3b", "++", "Basic", "Good tool format"},
		{"qwen2.5-coder:7b", "+++", "Good", "Best for coding"},
		{"mistral:7b", "++", "Good", "General purpose"},
		{"llama3.1:8b", "+++", "Full", "Reliable agentic"},
		{"qwen2.5-coder:14b", "+++", "Full", "Complex tasks"},
		{"mistral-small:22b", "+++", "Full", "Strong reasoning"},
		{"llama3.3:70b", "+++", "Full", "Maximum capability"},
	}

	for _, m := range models {
		sb.WriteString(fmt.Sprintf("%-18s | %-5s | %-7s | %s\n",
			m.name, m.tools, m.agentic, m.notes))
	}

	return sb.String()
}
