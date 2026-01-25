// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package index provides codebase indexing for fast symbol search.
package index

import (
	"database/sql"
	"fmt"
	"strings"
)

// =============================================================================
// SEARCH RESULT
// =============================================================================

// SearchResult represents a single search result
type SearchResult struct {
	Symbol
	FilePath string
	Language string
	Rank     float64 // Search relevance rank
}

// SearchOptions configures search behavior
type SearchOptions struct {
	// MaxResults limits the number of results (0 = unlimited)
	MaxResults int

	// SymbolTypes filters by symbol types (empty = all types)
	SymbolTypes []SymbolType

	// Languages filters by programming language (empty = all)
	Languages []string

	// ExportedOnly returns only exported/public symbols
	ExportedOnly bool

	// IncludeDoc includes documentation in results
	IncludeDoc bool

	// FuzzyMatch enables fuzzy matching
	FuzzyMatch bool
}

// DefaultSearchOptions returns default search options
func DefaultSearchOptions() *SearchOptions {
	return &SearchOptions{
		MaxResults:   50,
		SymbolTypes:  []SymbolType{},
		Languages:    []string{},
		ExportedOnly: false,
		IncludeDoc:   true,
		FuzzyMatch:   false,
	}
}

// =============================================================================
// SEARCH METHODS
// =============================================================================

// Search finds symbols matching the query using full-text search
func (idx *CodebaseIndex) Search(query string, options *SearchOptions) ([]SearchResult, error) {
	if !idx.IsIndexed() {
		return nil, ErrNotIndexed
	}

	if options == nil {
		options = DefaultSearchOptions()
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Build FTS query
	ftsQuery := idx.buildFTSQuery(query, options)
	if ftsQuery == "" {
		// Empty query not allowed for FTS search
		return []SearchResult{}, nil
	}

	// Build SQL query
	sqlQuery := `
		SELECT
			s.id, s.name, s.type, s.line, s.end_line, s.signature, s.doc, s.parent, s.visibility,
			f.path, f.language,
			fts.rank
		FROM symbols_fts fts
		JOIN symbols s ON s.id = fts.rowid
		JOIN files f ON f.id = s.file_id
		WHERE symbols_fts MATCH ?
	`

	var args []interface{}
	args = append(args, ftsQuery)

	// Add filters
	var conditions []string

	if len(options.SymbolTypes) > 0 {
		placeholders := make([]string, len(options.SymbolTypes))
		for i, t := range options.SymbolTypes {
			placeholders[i] = "?"
			args = append(args, t.String())
		}
		conditions = append(conditions, "s.type IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(options.Languages) > 0 {
		placeholders := make([]string, len(options.Languages))
		for i, lang := range options.Languages {
			placeholders[i] = "?"
			args = append(args, lang)
		}
		conditions = append(conditions, "f.language IN ("+strings.Join(placeholders, ",")+")")
	}

	if options.ExportedOnly {
		conditions = append(conditions, "s.visibility IN ('exported', 'public')")
	}

	if len(conditions) > 0 {
		sqlQuery += " AND " + strings.Join(conditions, " AND ")
	}

	// Order by rank
	sqlQuery += " ORDER BY fts.rank DESC"

	// Limit results
	if options.MaxResults > 0 {
		sqlQuery += " LIMIT ?"
		args = append(args, options.MaxResults)
	}

	// Execute query
	rows, err := idx.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var symType, visibility, doc, parent string
		var endLine sql.NullInt64
		var signature sql.NullString
		var symbolID int64 // id column from SELECT

		err := rows.Scan(
			&symbolID, // id from SELECT
			&result.Name,
			&symType,
			&result.Line,
			&endLine,
			&signature,
			&doc,
			&parent,
			&visibility,
			&result.FilePath,
			&result.Language,
			&result.Rank,
		)
		if err != nil {
			continue
		}

		result.Type = SymbolType(symType)
		result.Visibility = Visibility(visibility)
		if endLine.Valid {
			result.EndLine = int(endLine.Int64)
		}
		if signature.Valid {
			result.Signature = signature.String
		}
		if options.IncludeDoc {
			result.Doc = doc
		}
		result.Parent = parent

		results = append(results, result)
	}

	return results, nil
}

// SearchByName searches for symbols by exact or prefix name match
func (idx *CodebaseIndex) SearchByName(name string, options *SearchOptions) ([]SearchResult, error) {
	if !idx.IsIndexed() {
		return nil, ErrNotIndexed
	}

	if options == nil {
		options = DefaultSearchOptions()
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	sqlQuery := `
		SELECT
			s.name, s.type, s.line, s.end_line, s.signature, s.doc, s.parent, s.visibility,
			f.path, f.language
		FROM symbols s
		JOIN files f ON f.id = s.file_id
		WHERE s.name LIKE ?
	`

	var args []interface{}
	if options.FuzzyMatch {
		args = append(args, "%"+name+"%")
	} else {
		args = append(args, name+"%")
	}

	// Add filters
	var conditions []string

	if len(options.SymbolTypes) > 0 {
		placeholders := make([]string, len(options.SymbolTypes))
		for i, t := range options.SymbolTypes {
			placeholders[i] = "?"
			args = append(args, t.String())
		}
		conditions = append(conditions, "s.type IN ("+strings.Join(placeholders, ",")+")")
	}

	if len(options.Languages) > 0 {
		placeholders := make([]string, len(options.Languages))
		for i, lang := range options.Languages {
			placeholders[i] = "?"
			args = append(args, lang)
		}
		conditions = append(conditions, "f.language IN ("+strings.Join(placeholders, ",")+")")
	}

	if options.ExportedOnly {
		conditions = append(conditions, "s.visibility IN ('exported', 'public')")
	}

	if len(conditions) > 0 {
		sqlQuery += " AND " + strings.Join(conditions, " AND ")
	}

	sqlQuery += " ORDER BY s.name"

	if options.MaxResults > 0 {
		sqlQuery += " LIMIT ?"
		args = append(args, options.MaxResults)
	}

	rows, err := idx.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var symType, visibility, doc, parent string
		var endLine sql.NullInt64
		var signature sql.NullString

		err := rows.Scan(
			&result.Name,
			&symType,
			&result.Line,
			&endLine,
			&signature,
			&doc,
			&parent,
			&visibility,
			&result.FilePath,
			&result.Language,
		)
		if err != nil {
			continue
		}

		result.Type = SymbolType(symType)
		result.Visibility = Visibility(visibility)
		if endLine.Valid {
			result.EndLine = int(endLine.Int64)
		}
		if signature.Valid {
			result.Signature = signature.String
		}
		if options.IncludeDoc {
			result.Doc = doc
		}
		result.Parent = parent

		results = append(results, result)
	}

	return results, nil
}

// GetFileSymbols returns all symbols in a file
func (idx *CodebaseIndex) GetFileSymbols(filePath string) ([]Symbol, error) {
	if !idx.IsIndexed() {
		return nil, ErrNotIndexed
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	rows, err := idx.db.Query(`
		SELECT s.name, s.type, s.line, s.end_line, s.signature, s.doc, s.parent, s.visibility
		FROM symbols s
		JOIN files f ON f.id = s.file_id
		WHERE f.path = ?
		ORDER BY s.line
	`, filePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}
	defer rows.Close()

	var symbols []Symbol
	for rows.Next() {
		var sym Symbol
		var symType, visibility, doc, parent string
		var endLine sql.NullInt64
		var signature sql.NullString

		err := rows.Scan(
			&sym.Name,
			&symType,
			&sym.Line,
			&endLine,
			&signature,
			&doc,
			&parent,
			&visibility,
		)
		if err != nil {
			continue
		}

		sym.Type = SymbolType(symType)
		sym.Visibility = Visibility(visibility)
		if endLine.Valid {
			sym.EndLine = int(endLine.Int64)
		}
		if signature.Valid {
			sym.Signature = signature.String
		}
		sym.Doc = doc
		sym.Parent = parent

		symbols = append(symbols, sym)
	}

	return symbols, nil
}

// GetFiles returns all indexed files
func (idx *CodebaseIndex) GetFiles() ([]string, error) {
	if !idx.IsIndexed() {
		return nil, ErrNotIndexed
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	rows, err := idx.db.Query("SELECT path FROM files ORDER BY path")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}
	defer rows.Close()

	var files []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err == nil {
			files = append(files, path)
		}
	}

	return files, nil
}

// buildFTSQuery builds an FTS5 query from user input
func (idx *CodebaseIndex) buildFTSQuery(query string, options *SearchOptions) string {
	// Clean query
	query = strings.TrimSpace(query)
	if query == "" {
		// Return empty string for empty query - caller should handle this
		return ""
	}

	// Sanitize query by escaping FTS5 special characters
	query = sanitizeFTSQuery(query)

	// For simple queries, search in name field with prefix match
	if !strings.ContainsAny(query, " \"*") {
		return "name:" + query + "*"
	}

	// For complex queries, use the query as-is
	return query
}

// sanitizeFTSQuery escapes FTS5 special characters to prevent injection
func sanitizeFTSQuery(query string) string {
	// FTS5 special characters: " * ( ) { } [ ] : ^ - ~
	specialChars := []string{"\"", "*", "(", ")", "{", "}", "[", "]", ":", "^", "-", "~"}

	for _, char := range specialChars {
		query = strings.ReplaceAll(query, char, "\\"+char)
	}

	return query
}

// =============================================================================
// REPOSITORY STRUCTURE
// =============================================================================

// FileInfo represents file information
type FileInfo struct {
	Path       string
	Language   string
	Size       int64
	Lines      int
	SymbolCount int
	ModTime    int64
}

// GetRepositoryStructure returns a summary of the repository structure
func (idx *CodebaseIndex) GetRepositoryStructure() (map[string][]FileInfo, error) {
	if !idx.IsIndexed() {
		return nil, ErrNotIndexed
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	rows, err := idx.db.Query(`
		SELECT
			f.path, f.language, f.size, f.line_count, f.mod_time,
			COUNT(s.id) as symbol_count
		FROM files f
		LEFT JOIN symbols s ON s.file_id = f.id
		GROUP BY f.id
		ORDER BY f.path
	`)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}
	defer rows.Close()

	structure := make(map[string][]FileInfo)

	for rows.Next() {
		var info FileInfo
		var symbolCount sql.NullInt64

		err := rows.Scan(
			&info.Path,
			&info.Language,
			&info.Size,
			&info.Lines,
			&info.ModTime,
			&symbolCount,
		)
		if err != nil {
			continue
		}

		if symbolCount.Valid {
			info.SymbolCount = int(symbolCount.Int64)
		}

		// Group by directory
		dir := "."
		if idx := strings.LastIndex(info.Path, "/"); idx != -1 {
			dir = info.Path[:idx]
		}

		structure[dir] = append(structure[dir], info)
	}

	return structure, nil
}

// GetLanguageStats returns statistics by programming language
func (idx *CodebaseIndex) GetLanguageStats() (map[string]int, error) {
	if !idx.IsIndexed() {
		return nil, ErrNotIndexed
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	rows, err := idx.db.Query(`
		SELECT language, COUNT(*) as count
		FROM files
		GROUP BY language
		ORDER BY count DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var lang string
		var count int
		if err := rows.Scan(&lang, &count); err == nil {
			stats[lang] = count
		}
	}

	return stats, nil
}
