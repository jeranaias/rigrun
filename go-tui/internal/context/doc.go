// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package context provides the @ mention system and context management for messages.
//
// This package handles parsing and expanding @ mentions in user input,
// fetching referenced content, and managing conversation context including
// truncation and summarization for long conversations.
//
// # Key Types
//
//   - MentionType: Enumeration of supported mention types (@file, @clipboard, etc.)
//   - Mention: Parsed mention with type and value
//   - ExpandedContext: Fetched and processed context content
//   - Truncator: Conversation truncation with configurable strategies
//   - Summarizer: Conversation summarization using LLM
//
// # Mention Types
//
//   - @file:path - Include file contents
//   - @clipboard - Include clipboard contents
//   - @git or @git:range - Include git diff
//   - @codebase - Include codebase context
//   - @error - Include last error
//   - @url:https://... - Fetch URL content
//
// # Usage
//
// Parse and expand mentions:
//
//	mentions := context.ParseMentions(input)
//	expanded, err := context.ExpandMentions(ctx, mentions)
//
// Truncate long conversations:
//
//	truncator := context.NewTruncator(maxTokens, summarizer)
//	truncated, err := truncator.Truncate(ctx, messages)
package context
