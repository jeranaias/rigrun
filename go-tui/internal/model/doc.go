// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package model contains the data structures for conversations and messages.
//
// This package defines the core domain types used throughout the application
// for representing chat conversations, messages, and model information.
//
// # Key Types
//
//   - Conversation: Container for a chat session with messages and metadata
//   - Message: Single message with role, content, timestamp, and optional tool calls
//   - ModelInfo: Information about an LLM model (ID, provider, cost)
//   - Role: Message role enumeration (user, assistant, system, tool)
//
// # Usage
//
// Create a new conversation:
//
//	conv := model.NewConversation()
//	conv.AddMessage(model.Message{
//	    Role:    model.RoleUser,
//	    Content: "Hello!",
//	})
//
// Work with model information:
//
//	info := model.GetModelInfo("qwen2.5:7b")
//	fmt.Printf("Model: %s, Cost: $%.4f/1K\n", info.Name, info.CostPer1K)
package model
