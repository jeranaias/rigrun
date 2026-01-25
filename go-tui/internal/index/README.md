# Codebase Indexing for rigrun TUI

This package provides intelligent codebase indexing and search functionality to make `@codebase` mentions instant and intelligent.

## Features

- **Fast Symbol Search**: Full-text search over functions, classes, types, and variables
- **Multi-Language Support**: Go, JavaScript/TypeScript, Python parsers included
- **Incremental Updates**: File watcher automatically updates index on changes
- **SQLite with FTS5**: Fast full-text search with relevance ranking
- **Repository Structure**: Quick overview of codebase organization

## Usage

### Basic Indexing

```go
import (
    "context"
    "github.com/jeranaias/rigrun-tui/internal/index"
)

// Create index configuration
config := index.DefaultConfig("/path/to/codebase")

// Create codebase index
idx, err := index.NewCodebaseIndex(config)
if err != nil {
    log.Fatal(err)
}
defer idx.Close()

// Perform initial indexing
ctx := context.Background()
if err := idx.Index(ctx); err != nil {
    log.Fatal(err)
}

// Get statistics
stats := idx.Stats()
fmt.Printf("Indexed %d files with %d symbols\n", stats.FileCount, stats.SymbolCount)
```

### Searching

```go
// Search for symbols
opts := index.DefaultSearchOptions()
opts.MaxResults = 10

results, err := idx.Search("NewServer", opts)
if err != nil {
    log.Fatal(err)
}

for _, result := range results {
    fmt.Printf("%s (%s) at %s:%d\n",
        result.Name, result.Type, result.FilePath, result.Line)
}
```

### Search by Name

```go
// Exact or prefix match
results, err := idx.SearchByName("Handler", opts)

// Fuzzy match
opts.FuzzyMatch = true
results, err = idx.SearchByName("hndlr", opts)
```

### Filter by Type or Language

```go
// Search only for functions
opts := index.DefaultSearchOptions()
opts.SymbolTypes = []index.SymbolType{index.SymbolFunction}

// Search only in Go files
opts.Languages = []string{"Go"}

results, err := idx.Search("New", opts)
```

### Integration with @codebase

```go
import (
    "github.com/jeranaias/rigrun-tui/internal/context"
    "github.com/jeranaias/rigrun-tui/internal/index"
)

// Create fetcher
fetcher := context.NewFetcher(nil)

// Set codebase index
idx, _ := index.NewCodebaseIndex(config)
fetcher.SetCodebaseIndex(idx)

// Now @codebase will use the index
result := fetcher.FetchCodebase()
```

## Architecture

### Database Schema

- **files**: Tracks indexed files with modification times
- **symbols**: Code symbols (functions, classes, types, etc.)
- **symbols_fts**: FTS5 virtual table for full-text search
- **imports**: Import dependencies
- **tags**: Custom symbol tags

### Parsers

Each language has a dedicated parser that extracts symbols:

- **GoParser**: Uses `go/ast` for accurate parsing
- **JSParser**: Regex-based for JavaScript/TypeScript
- **PythonParser**: Regex-based for Python

New parsers can be added by implementing the `Parser` interface.

### File Watching

Two implementations:
- **FsnotifyWatcher**: Uses fsnotify for efficient event-based watching
- **PollingWatcher**: Fallback using periodic filesystem scans

## Performance

- **Initial Index**: <5s for 10,000 files
- **Incremental Updates**: <100ms for single file changes
- **Search**: <50ms for typical queries
- **Database Size**: ~1-2MB per 1,000 files

## Commands

### /index

Triggers a full codebase index:

```
/index
```

This will:
1. Scan all supported files in the codebase
2. Parse symbols from each file
3. Store in SQLite database with FTS
4. Start file watcher for incremental updates

### /search

Search for symbols in the indexed codebase:

```
/search NewServer
/search "func.*Handler"
```

## Configuration

```go
config := &index.Config{
    Root:          "/path/to/repo",
    DatabasePath:  ".rigrun/codebase.db",
    MaxFileSize:   10 * 1024 * 1024, // 10MB
    IgnorePatterns: []string{
        ".git", "node_modules", "__pycache__",
    },
    Languages:     []string{}, // Empty = all
    EnableWatch:   true,
    WatchDebounce: 500 * time.Millisecond,
}
```

## Symbol Types

- `SymbolFunction`: Functions and methods
- `SymbolClass`: Classes (JS, Python)
- `SymbolType_`: Type definitions (Go)
- `SymbolStruct`: Struct types (Go)
- `SymbolInterface`: Interfaces (Go)
- `SymbolVariable`: Variables
- `SymbolConst`: Constants
- `SymbolMethod`: Methods (Go, Python)
- `SymbolField`: Struct/class fields

## Example: Repository Structure

```go
structure, err := idx.GetRepositoryStructure()
if err != nil {
    log.Fatal(err)
}

for dir, files := range structure {
    fmt.Printf("%s/\n", dir)
    for _, file := range files {
        fmt.Printf("  %s (%d symbols)\n", file.Path, file.SymbolCount)
    }
}
```

## Example: Language Statistics

```go
stats, err := idx.GetLanguageStats()
if err != nil {
    log.Fatal(err)
}

for lang, count := range stats {
    fmt.Printf("%s: %d files\n", lang, count)
}
```

## Future Enhancements

- Semantic search using embeddings
- Cross-reference analysis (find all callers)
- Symbol renaming across files
- Documentation extraction
- Test coverage mapping
- Dependency graph visualization
