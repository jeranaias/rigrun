// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ollama provides the HTTP client for communicating with Ollama API.
//
// This package implements a client for the Ollama local LLM server,
// supporting both streaming and non-streaming chat completions with
// tool calling capabilities.
//
// # Key Types
//
//   - Client: HTTP client for Ollama API communication
//   - Message: Chat message with role, content, and optional tool calls
//   - ChatRequest: Request structure for chat completions
//   - ChatResponse: Response structure with message and metrics
//   - StreamReader: Streaming response reader with pooled resources
//
// # Usage
//
// Create a client and send a chat request:
//
//	client := ollama.NewClient("http://localhost:11434")
//	resp, err := client.Chat(ctx, ollama.ChatRequest{
//	    Model:    "qwen2.5:7b",
//	    Messages: []ollama.Message{{Role: "user", Content: "Hello"}},
//	})
//
// For streaming responses:
//
//	reader, err := client.ChatStream(ctx, request)
//	for {
//	    chunk, err := reader.Next()
//	    if err == io.EOF {
//	        break
//	    }
//	    fmt.Print(chunk.Message.Content)
//	}
//
// # Performance
//
// The package includes optimized streaming with pooled StreamReader
// instances to reduce allocations in the hot path.
package ollama
