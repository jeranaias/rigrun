// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package index provides codebase indexing for fast symbol search.
package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// =============================================================================
// ERRORS
// =============================================================================

var (
	ErrNotIndexed    = errors.New("codebase not indexed")
	ErrIndexing      = errors.New("indexing in progress")
	ErrDatabaseError = errors.New("database error")
	ErrInvalidPath   = errors.New("invalid path")
)

// =============================================================================
// CODEBASE INDEX
// =============================================================================

// CodebaseIndex indexes a repository for fast search
type CodebaseIndex struct {
	db      *sql.DB
	watcher FileWatcher // Interface for file watching (fsnotify or polling)
	root    string
	mu      sync.RWMutex

	// Indexing state
	indexing     bool
	indexingMu   sync.Mutex
	lastIndexed  time.Time
	symbolCount  int
	fileCount    int

	// Configuration
	config       *Config

	// Parser registry
	parsers      map[string]Parser
}

// Config holds index configuration
type Config struct {
	// Root is the codebase root directory
	Root string

	// DatabasePath is where to store the SQLite database
	DatabasePath string

	// MaxFileSize is the maximum file size to index (bytes)
	MaxFileSize int64

	// IgnorePatterns are glob patterns to ignore
	IgnorePatterns []string

	// Languages to index (empty = all supported)
	Languages []string

	// EnableWatch enables file watching for incremental updates
	EnableWatch bool

	// WatchDebounce is the debounce duration for file change events
	WatchDebounce time.Duration
}

// DefaultConfig returns default configuration
func DefaultConfig(root string) *Config {
	return &Config{
		Root:          root,
		DatabasePath:  filepath.Join(root, ".rigrun", "codebase.db"),
		MaxFileSize:   10 * 1024 * 1024, // 10MB
		IgnorePatterns: []string{
			".git", ".svn", ".hg",
			"node_modules", "__pycache__", ".venv", "venv",
			"vendor", "target", "dist", "build",
			".idea", ".vscode", ".vs",
			"*.exe", "*.dll", "*.so", "*.dylib",
			"*.zip", "*.tar", "*.gz",
			"*.jpg", "*.png", "*.gif", "*.pdf",
		},
		Languages:     []string{}, // All supported
		EnableWatch:   true,
		WatchDebounce: 500 * time.Millisecond,
	}
}

// NewCodebaseIndex creates a new codebase index
func NewCodebaseIndex(config *Config) (*CodebaseIndex, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	// Validate root path
	info, err := os.Stat(config.Root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: not a directory", ErrInvalidPath)
	}

	// Create database directory if needed
	dbDir := filepath.Dir(config.DatabasePath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite", config.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for SQLite
	// SQLite only supports one writer at a time, so limit connections
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // No lifetime limit

	// Configure SQLite for better performance
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-64000",      // 64MB cache
		"PRAGMA temp_store=MEMORY",
		"PRAGMA mmap_size=268435456",    // 256MB mmap
		"PRAGMA foreign_keys=ON",        // Enable foreign key constraints
		"PRAGMA wal_autocheckpoint=1000", // Checkpoint every 1000 pages
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma: %w", err)
		}
	}

	idx := &CodebaseIndex{
		db:      db,
		root:    config.Root,
		config:  config,
		parsers: make(map[string]Parser),
	}

	// Initialize schema
	if err := idx.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Register language parsers
	idx.registerParsers()

	// Load statistics
	if err := idx.loadStats(); err != nil {
		// Non-fatal, continue
	}

	return idx, nil
}

// initSchema creates the database schema
func (idx *CodebaseIndex) initSchema() error {
	// Create tables
	if _, err := idx.db.Exec(Schema); err != nil {
		return err
	}

	// Initialize metadata
	if _, err := idx.db.Exec(InitMetadata); err != nil {
		return err
	}

	// Set root path in metadata
	_, err := idx.db.Exec("UPDATE metadata SET value = ? WHERE key = 'root_path'", idx.root)
	return err
}

// registerParsers registers language parsers
func (idx *CodebaseIndex) registerParsers() {
	// Register Go parser
	idx.parsers[".go"] = &GoParser{}

	// Register JavaScript/TypeScript parser
	idx.parsers[".js"] = &JSParser{}
	idx.parsers[".ts"] = &JSParser{}
	idx.parsers[".jsx"] = &JSParser{}
	idx.parsers[".tsx"] = &JSParser{}

	// Register Python parser
	idx.parsers[".py"] = &PythonParser{}

	// More parsers can be added as needed
}

// Close closes the index and releases resources
func (idx *CodebaseIndex) Close() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.watcher != nil {
		idx.watcher.Close()
	}

	if idx.db != nil {
		return idx.db.Close()
	}

	return nil
}

// =============================================================================
// INDEXING
// =============================================================================

// Index performs a full index of the codebase
func (idx *CodebaseIndex) Index(ctx context.Context) error {
	idx.indexingMu.Lock()
	if idx.indexing {
		idx.indexingMu.Unlock()
		return ErrIndexing
	}
	idx.indexing = true
	idx.indexingMu.Unlock()

	defer func() {
		idx.indexingMu.Lock()
		idx.indexing = false
		idx.indexingMu.Unlock()
	}()

	startTime := time.Now()

	// Begin transaction
	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}
	defer tx.Rollback()

	// Clear existing data
	if _, err := tx.Exec("DELETE FROM symbols"); err != nil {
		return fmt.Errorf("failed to clear symbols: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM files"); err != nil {
		return fmt.Errorf("failed to clear files: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM imports"); err != nil {
		return fmt.Errorf("failed to clear imports: %w", err)
	}

	// Walk the codebase
	var fileCount, symbolCount int
	err = filepath.Walk(idx.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if info.IsDir() {
			// Check if should ignore
			if idx.shouldIgnore(filepath.Base(path)) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip ignored files
		if idx.shouldIgnore(filepath.Base(path)) {
			return nil
		}

		// Skip large files
		if info.Size() > idx.config.MaxFileSize {
			return nil
		}

		// Check if we have a parser for this file
		ext := filepath.Ext(path)
		parser, ok := idx.parsers[ext]
		if !ok {
			return nil
		}

		// Index the file
		numSymbols, err := idx.indexFile(tx, path, info, parser)
		if err != nil {
			// Log error but continue
			return nil
		}

		fileCount++
		symbolCount += numSymbols

		return nil
	})

	if err != nil && err != context.Canceled {
		return fmt.Errorf("failed to walk codebase: %w", err)
	}

	// Update metadata
	now := time.Now().Unix()
	if _, err := tx.Exec("UPDATE metadata SET value = ? WHERE key = 'last_full_index'", now); err != nil {
		return err
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Update statistics with proper mutex protection
	idx.mu.Lock()
	idx.lastIndexed = startTime
	idx.fileCount = fileCount
	idx.symbolCount = symbolCount
	idx.mu.Unlock()

	// Start file watcher if enabled
	if idx.config.EnableWatch && idx.watcher == nil {
		if err := idx.startWatcher(); err != nil {
			// Non-fatal, log and continue
		}
	}

	return nil
}

// indexFile indexes a single file and returns the number of symbols indexed
func (idx *CodebaseIndex) indexFile(tx *sql.Tx, path string, info os.FileInfo, parser Parser) (int, error) {
	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	// Parse symbols
	symbols, imports, err := parser.Parse(string(content), path)
	if err != nil {
		return 0, err
	}

	// Get relative path
	relPath, err := filepath.Rel(idx.root, path)
	if err != nil {
		relPath = path
	}

	// Detect language
	language := idx.detectLanguage(filepath.Ext(path))

	// Count lines
	lineCount := strings.Count(string(content), "\n") + 1

	// Insert file record
	result, err := tx.Exec(`
		INSERT INTO files (path, mod_time, size, language, line_count, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, relPath, info.ModTime().Unix(), info.Size(), language, lineCount, time.Now().Unix())
	if err != nil {
		return 0, err
	}

	fileID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Insert symbols
	for _, sym := range symbols {
		_, err := tx.Exec(`
			INSERT INTO symbols (name, type, file_id, line, end_line, signature, doc, parent, visibility)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, sym.Name, sym.Type, fileID, sym.Line, sym.EndLine, sym.Signature, sym.Doc, sym.Parent, sym.Visibility)
		if err != nil {
			return 0, err
		}
	}

	// Insert imports
	for _, imp := range imports {
		_, err := tx.Exec(`
			INSERT INTO imports (file_id, import_path, alias, line)
			VALUES (?, ?, ?, ?)
		`, fileID, imp.Path, imp.Alias, imp.Line)
		if err != nil {
			return 0, err
		}
	}

	return len(symbols), nil
}

// shouldIgnore checks if a file/directory should be ignored
func (idx *CodebaseIndex) shouldIgnore(name string) bool {
	for _, pattern := range idx.config.IgnorePatterns {
		matched, _ := filepath.Match(pattern, name)
		if matched {
			return true
		}
	}
	return false
}

// detectLanguage detects the language from file extension
func (idx *CodebaseIndex) detectLanguage(ext string) string {
	switch ext {
	case ".go":
		return "Go"
	case ".js", ".jsx":
		return "JavaScript"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".py":
		return "Python"
	case ".java":
		return "Java"
	case ".c", ".h":
		return "C"
	case ".cpp", ".hpp", ".cc":
		return "C++"
	case ".rs":
		return "Rust"
	case ".rb":
		return "Ruby"
	case ".php":
		return "PHP"
	case ".cs":
		return "C#"
	default:
		return "Unknown"
	}
}

// loadStats loads statistics from the database
func (idx *CodebaseIndex) loadStats() error {
	var lastIndexed int64
	err := idx.db.QueryRow("SELECT value FROM metadata WHERE key = 'last_full_index'").Scan(&lastIndexed)
	if err != nil {
		return err
	}

	if lastIndexed > 0 {
		idx.lastIndexed = time.Unix(lastIndexed, 0)
	}

	// Count files
	err = idx.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&idx.fileCount)
	if err != nil {
		return err
	}

	// Count symbols
	err = idx.db.QueryRow("SELECT COUNT(*) FROM symbols").Scan(&idx.symbolCount)
	if err != nil {
		return err
	}

	return nil
}

// =============================================================================
// STATISTICS
// =============================================================================

// Stats returns index statistics
type Stats struct {
	FileCount    int
	SymbolCount  int
	LastIndexed  time.Time
	IsIndexing   bool
	DatabaseSize int64
}

// Stats returns current index statistics
func (idx *CodebaseIndex) Stats() Stats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	idx.indexingMu.Lock()
	indexing := idx.indexing
	idx.indexingMu.Unlock()

	// Get database file size
	var dbSize int64
	if info, err := os.Stat(idx.config.DatabasePath); err == nil {
		dbSize = info.Size()
	}

	return Stats{
		FileCount:    idx.fileCount,
		SymbolCount:  idx.symbolCount,
		LastIndexed:  idx.lastIndexed,
		IsIndexing:   indexing,
		DatabaseSize: dbSize,
	}
}

// IsIndexed returns true if the codebase has been indexed
func (idx *CodebaseIndex) IsIndexed() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return !idx.lastIndexed.IsZero()
}
