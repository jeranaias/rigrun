// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package index provides codebase indexing for fast symbol search.
//
// This package creates and maintains a SQLite-based index of code symbols,
// enabling fast full-text search across large codebases.
//
// # Key Types
//
//   - Index: Main indexer with SQLite backend
//   - Symbol: Indexed symbol with type, name, and location
//   - SearchResult: Search result with score and context
//   - Watcher: File system watcher for incremental updates
//
// # Supported Languages
//
//   - Go: Functions, types, methods, interfaces
//   - Python: Functions, classes, methods
//   - JavaScript/TypeScript: Functions, classes, exports
//   - Rust: Functions, structs, traits, impl blocks
//
// # Usage
//
// Create and populate an index:
//
//	idx, err := index.New(dbPath)
//	err = idx.IndexDirectory(ctx, "/path/to/project")
//
// Search the index:
//
//	results, err := idx.Search("handleRequest")
//	for _, r := range results {
//	    fmt.Printf("%s:%d %s\n", r.File, r.Line, r.Name)
//	}
//
// Enable file watching for incremental updates:
//
//	watcher := idx.Watch(ctx, "/path/to/project")
package index
