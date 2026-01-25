// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package index_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/index"
)

// Example demonstrates basic indexing and search
func Example() {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "codebase-index-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some sample Go files
	createSampleFiles(tmpDir)

	// Create index configuration
	config := index.DefaultConfig(tmpDir)
	config.DatabasePath = filepath.Join(tmpDir, "test.db")

	// Create codebase index
	idx, err := index.NewCodebaseIndex(config)
	if err != nil {
		panic(err)
	}
	defer idx.Close()

	// Perform initial indexing
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := idx.Index(ctx); err != nil {
		panic(err)
	}

	// Get statistics
	stats := idx.Stats()
	fmt.Printf("Indexed %d files with %d symbols\n", stats.FileCount, stats.SymbolCount)

	// Search for symbols
	opts := index.DefaultSearchOptions()
	opts.MaxResults = 5

	results, err := idx.SearchByName("NewServer", opts)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Found %d results for 'NewServer'\n", len(results))
	for _, result := range results {
		fmt.Printf("  %s (%s) at %s:%d\n",
			result.Name, result.Type, result.FilePath, result.Line)
	}

	// Output:
	// Indexed 2 files with 8 symbols
	// Found 1 results for 'NewServer'
	//   NewServer (Function) at server.go:12
}

// createSampleFiles creates sample Go files for testing
func createSampleFiles(dir string) {
	files := map[string]string{
		"server.go": `package main

import "net/http"

// Server represents an HTTP server
type Server struct {
	addr string
	handler http.Handler
}

// NewServer creates a new server
func NewServer(addr string) *Server {
	return &Server{addr: addr}
}

// Start starts the server
func (s *Server) Start() error {
	return http.ListenAndServe(s.addr, s.handler)
}
`,
		"handler.go": `package main

import "net/http"

// Handler handles HTTP requests
type Handler struct {
	routes map[string]http.HandlerFunc
}

// NewHandler creates a new handler
func NewHandler() *Handler {
	return &Handler{
		routes: make(map[string]http.HandlerFunc),
	}
}

// ServeHTTP implements http.Handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if fn, ok := h.routes[r.URL.Path]; ok {
		fn(w, r)
	}
}
`,
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			panic(err)
		}
	}
}

// Example_search demonstrates searching the index
func Example_search() {
	// Setup (omitted for brevity)
	tmpDir, _ := os.MkdirTemp("", "test")
	defer os.RemoveAll(tmpDir)

	createSampleFiles(tmpDir)

	config := index.DefaultConfig(tmpDir)
	config.DatabasePath = filepath.Join(tmpDir, "test.db")

	idx, _ := index.NewCodebaseIndex(config)
	defer idx.Close()

	ctx := context.Background()
	idx.Index(ctx)

	// Search examples
	opts := index.DefaultSearchOptions()

	// Search for all functions
	opts.SymbolTypes = []index.SymbolType{index.SymbolFunction}
	results, _ := idx.SearchByName("New", opts)
	fmt.Printf("Found %d constructor functions\n", len(results))

	// Search for structs
	opts.SymbolTypes = []index.SymbolType{index.SymbolStruct}
	results, _ = idx.SearchByName("", opts)
	fmt.Printf("Found %d structs\n", len(results))

	// Full-text search
	opts = index.DefaultSearchOptions()
	results, _ = idx.Search("Server", opts)
	fmt.Printf("Found %d symbols matching 'Server'\n", len(results))

	// Output:
	// Found 2 constructor functions
	// Found 2 structs
	// Found 1 symbols matching 'Server'
}

// Example_structure demonstrates repository structure queries
func Example_structure() {
	// Setup
	tmpDir, _ := os.MkdirTemp("", "test")
	defer os.RemoveAll(tmpDir)

	createSampleFiles(tmpDir)

	config := index.DefaultConfig(tmpDir)
	config.DatabasePath = filepath.Join(tmpDir, "test.db")

	idx, _ := index.NewCodebaseIndex(config)
	defer idx.Close()

	ctx := context.Background()
	idx.Index(ctx)

	// Get repository structure
	structure, err := idx.GetRepositoryStructure()
	if err != nil {
		panic(err)
	}

	for dir, files := range structure {
		fmt.Printf("%s/\n", dir)
		for _, file := range files {
			fmt.Printf("  %s (%d symbols)\n", filepath.Base(file.Path), file.SymbolCount)
		}
	}

	// Get language statistics
	langStats, err := idx.GetLanguageStats()
	if err != nil {
		panic(err)
	}

	fmt.Println("\nLanguage Statistics:")
	for lang, count := range langStats {
		fmt.Printf("  %s: %d files\n", lang, count)
	}

	// Output:
	// ./
	//   handler.go (4 symbols)
	//   server.go (4 symbols)
	//
	// Language Statistics:
	//   Go: 2 files
}
