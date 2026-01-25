// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cloud provides OpenRouter integration for cloud LLM inference.
//
// OpenRouter provides access to multiple LLM providers through a single API,
// including Claude, GPT-4, and other models. This package implements secure
// communication with OpenRouter's API including retry logic and validation.
//
// # Key Types
//
//   - Client: HTTP client for OpenRouter API with TLS and retry support
//   - Message: Chat message compatible with OpenRouter API format
//   - ChatRequest: Request structure for chat completions
//   - StreamReader: Streaming response reader for real-time output
//
// # Usage
//
// Create a client and send a chat request:
//
//	client := cloud.NewClient(apiKey)
//	resp, err := client.Chat(ctx, cloud.ChatRequest{
//	    Model:    "anthropic/claude-3-sonnet",
//	    Messages: []cloud.Message{{Role: "user", Content: "Hello"}},
//	})
//
// # Security
//
// This package implements secure logging and validation for DoD IL5
// compliance. API keys are never logged, and all requests use TLS 1.2+.
package cloud
