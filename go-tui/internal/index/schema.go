// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package index provides codebase indexing for fast symbol search.
package index

const (
	// SchemaVersion tracks the database schema version for migrations
	SchemaVersion = 1
)

// SQLite schema for codebase index with FTS (Full Text Search)
const Schema = `
-- Metadata table for schema version and index state
CREATE TABLE IF NOT EXISTS metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
) WITHOUT ROWID;

-- Files table: tracks indexed files with modification times
CREATE TABLE IF NOT EXISTS files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE,
    mod_time INTEGER NOT NULL,  -- Unix timestamp
    size INTEGER NOT NULL,
    language TEXT,              -- Go, JavaScript, Python, etc.
    line_count INTEGER,
    indexed_at INTEGER NOT NULL -- Unix timestamp
);

CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
CREATE INDEX IF NOT EXISTS idx_files_mod_time ON files(mod_time);
CREATE INDEX IF NOT EXISTS idx_files_language ON files(language);

-- Symbols table: code symbols (functions, classes, types, etc.)
CREATE TABLE IF NOT EXISTS symbols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,         -- Function, Class, Type, Variable, Const, Interface, Struct, Method
    file_id INTEGER NOT NULL,
    line INTEGER NOT NULL,
    end_line INTEGER,           -- End line for multi-line symbols
    signature TEXT,             -- Function signature, type definition, etc.
    doc TEXT,                   -- Documentation string
    parent TEXT,                -- Parent symbol (for methods, nested functions)
    visibility TEXT,            -- public, private, exported
    FOREIGN KEY(file_id) REFERENCES files(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name);
CREATE INDEX IF NOT EXISTS idx_symbols_type ON symbols(type);
CREATE INDEX IF NOT EXISTS idx_symbols_file_id ON symbols(file_id);
CREATE INDEX IF NOT EXISTS idx_symbols_visibility ON symbols(visibility);

-- Full-text search virtual table for symbols
CREATE VIRTUAL TABLE IF NOT EXISTS symbols_fts USING fts5(
    name,
    signature,
    doc,
    content='symbols',
    content_rowid='id',
    tokenize='porter unicode61'
);

-- Triggers to keep FTS table in sync
CREATE TRIGGER IF NOT EXISTS symbols_ai AFTER INSERT ON symbols BEGIN
    INSERT INTO symbols_fts(rowid, name, signature, doc)
    VALUES (new.id, new.name, new.signature, new.doc);
END;

CREATE TRIGGER IF NOT EXISTS symbols_ad AFTER DELETE ON symbols BEGIN
    DELETE FROM symbols_fts WHERE rowid = old.id;
END;

CREATE TRIGGER IF NOT EXISTS symbols_au AFTER UPDATE ON symbols BEGIN
    DELETE FROM symbols_fts WHERE rowid = old.id;
    INSERT INTO symbols_fts(rowid, name, signature, doc)
    VALUES (new.id, new.name, new.signature, new.doc);
END;

-- Imports table: track file dependencies
CREATE TABLE IF NOT EXISTS imports (
    file_id INTEGER NOT NULL,
    import_path TEXT NOT NULL,
    alias TEXT,
    line INTEGER NOT NULL,
    FOREIGN KEY(file_id) REFERENCES files(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_imports_file_id ON imports(file_id);
CREATE INDEX IF NOT EXISTS idx_imports_path ON imports(import_path);

-- Tags table: custom tags for categorization
CREATE TABLE IF NOT EXISTS tags (
    symbol_id INTEGER NOT NULL,
    tag TEXT NOT NULL,
    FOREIGN KEY(symbol_id) REFERENCES symbols(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tags_symbol_id ON tags(symbol_id);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);
`

// InitMetadata initializes the metadata table with default values
const InitMetadata = `
INSERT OR IGNORE INTO metadata (key, value) VALUES ('schema_version', '1');
INSERT OR IGNORE INTO metadata (key, value) VALUES ('created_at', strftime('%s', 'now'));
INSERT OR IGNORE INTO metadata (key, value) VALUES ('last_full_index', '0');
INSERT OR IGNORE INTO metadata (key, value) VALUES ('root_path', '');
`

// =============================================================================
// SYMBOL TYPES
// =============================================================================

// SymbolType represents the type of a code symbol
type SymbolType string

const (
	SymbolFunction   SymbolType = "Function"
	SymbolClass      SymbolType = "Class"
	SymbolType_      SymbolType = "Type"
	SymbolVariable   SymbolType = "Variable"
	SymbolConst      SymbolType = "Const"
	SymbolInterface  SymbolType = "Interface"
	SymbolStruct     SymbolType = "Struct"
	SymbolMethod     SymbolType = "Method"
	SymbolEnum       SymbolType = "Enum"
	SymbolField      SymbolType = "Field"
	SymbolImport     SymbolType = "Import"
	SymbolPackage    SymbolType = "Package"
)

// String returns the string representation of SymbolType
func (s SymbolType) String() string {
	return string(s)
}

// IsValid checks if the symbol type is valid
func (s SymbolType) IsValid() bool {
	switch s {
	case SymbolFunction, SymbolClass, SymbolType_, SymbolVariable,
		SymbolConst, SymbolInterface, SymbolStruct, SymbolMethod,
		SymbolEnum, SymbolField, SymbolImport, SymbolPackage:
		return true
	}
	return false
}

// =============================================================================
// VISIBILITY LEVELS
// =============================================================================

// Visibility represents symbol visibility
type Visibility string

const (
	VisibilityPublic   Visibility = "public"   // Exported, can be used anywhere
	VisibilityPrivate  Visibility = "private"  // Internal to file/package
	VisibilityExported Visibility = "exported" // Go exported (capitalized)
)

// String returns the string representation
func (v Visibility) String() string {
	return string(v)
}
