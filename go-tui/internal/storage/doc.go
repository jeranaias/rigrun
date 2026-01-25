// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package storage provides conversation persistence for rigrun TUI.
//
// This package handles saving and loading conversations to/from disk,
// with support for search, listing, and session management.
//
// # Key Types
//
//   - Store: Main storage interface for conversations
//   - StoredConversation: Serializable conversation with metadata
//   - ConversationMeta: Lightweight metadata for listing
//
// # Usage
//
// Create a store and save a conversation:
//
//	store := storage.NewStore(dataDir)
//	err := store.Save(conversation)
//
// List and load conversations:
//
//	metas, err := store.List()
//	conv, err := store.Load(metas[0].ID)
//
// Search conversations:
//
//	results, err := store.Search("query text")
//
// # Storage Location
//
// Conversations are stored in ~/.rigrun/sessions/ as JSON files.
package storage
