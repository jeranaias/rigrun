# Codebase Indexing Integration Guide

This document explains how to integrate the codebase indexing feature into the rigrun TUI.

## Overview

The codebase indexing system provides:
- Fast symbol search (functions, classes, types)
- Intelligent @codebase mentions
- /index and /search commands
- Incremental updates via file watching

## Integration Steps

### 1. Initialize Index on Startup

In your main application initialization (likely in `main.go` or where you set up the TUI):

```go
import (
    "github.com/jeranaias/rigrun-tui/internal/index"
    "github.com/jeranaias/rigrun-tui/internal/context"
    "github.com/jeranaias/rigrun-tui/internal/commands"
)

func initializeApp() {
    // Get current working directory
    wd, err := os.Getwd()
    if err != nil {
        log.Printf("Failed to get working directory: %v", err)
        return nil
    }

    // Create index configuration
    indexConfig := index.DefaultConfig(wd)

    // Optionally customize config
    // indexConfig.MaxFileSize = 20 * 1024 * 1024 // 20MB
    // indexConfig.EnableWatch = true

    // Create codebase index
    codebaseIndex, err := index.NewCodebaseIndex(indexConfig)
    if err != nil {
        log.Printf("Failed to create codebase index: %v", err)
        // Non-fatal - continue without indexing
        codebaseIndex = nil
    }

    // Start background indexing (optional - can also wait for /index command)
    if codebaseIndex != nil {
        go func() {
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
            defer cancel()

            if err := codebaseIndex.Index(ctx); err != nil {
                log.Printf("Background indexing failed: %v", err)
            }
        }()
    }

    // ... rest of initialization
}
```

### 2. Add Index to Command Context

When creating the command context for slash commands:

```go
// Create command context
cmdCtx := commands.NewContext(cfg, ollamaClient, store, sessionMgr, cacheMgr)

// Add codebase index if available
if codebaseIndex != nil {
    cmdCtx.CodebaseIndex = codebaseIndex
}
```

### 3. Add Index to Context Fetcher

For @codebase mentions to use the index:

```go
import "github.com/jeranaias/rigrun-tui/internal/context"

// Create context fetcher
fetcher := context.NewFetcher(fetcherConfig)

// Set codebase index
if codebaseIndex != nil {
    fetcher.SetCodebaseIndex(codebaseIndex)
}

// Create expander
expander := context.NewExpander(fetcher)
```

### 4. Register Index Commands

The `/index` and `/search` commands are defined in `internal/commands/index_cmd.go`. They need to be registered in the command registry:

```go
// In internal/commands/registry.go, in the registerBuiltins() function:

func (r *Registry) registerBuiltins() {
    // ... existing commands ...

    // Codebase commands
    r.Register(IndexCommand)
    r.Register(SearchCommand)
}
```

### 5. Handle Index Messages in UI

The index commands send messages that need to be handled in your main update loop:

```go
// In your bubbletea Update() function:

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {

    case commands.IndexStatusMsg:
        if msg.Success {
            m.statusMessage = fmt.Sprintf(
                "Indexing complete: %d files, %d symbols in %v",
                msg.FileCount, msg.SymbolCount, msg.Duration,
            )
        } else {
            m.statusMessage = fmt.Sprintf("Indexing failed: %v", msg.Error)
        }
        return m, nil

    case commands.SearchResultMsg:
        if msg.Error != nil {
            m.statusMessage = fmt.Sprintf("Search failed: %v", msg.Error)
            return m, nil
        }

        // Display search results
        m.searchResults = msg.Results
        m.showSearchResults = true
        return m, nil

    // ... other cases ...
    }
}
```

### 6. Show Index Status in UI

Optionally show index status in the status bar:

```go
func (m Model) renderStatusBar() string {
    var parts []string

    // ... existing status items ...

    // Show index status
    if m.codebaseIndex != nil {
        stats := m.codebaseIndex.Stats()
        if stats.IsIndexing {
            parts = append(parts, "Indexing...")
        } else if !stats.LastIndexed.IsZero() {
            parts = append(parts, fmt.Sprintf(
                "Indexed: %d files, %d symbols",
                stats.FileCount, stats.SymbolCount,
            ))
        }
    }

    return strings.Join(parts, " | ")
}
```

### 7. Cleanup on Exit

Don't forget to close the index when the application exits:

```go
func cleanup() {
    if codebaseIndex != nil {
        codebaseIndex.Close()
    }
}

// In main():
defer cleanup()
```

## Usage Examples

### User Workflow

1. **Start the TUI**
   - Index initializes in background (if auto-index enabled)
   - Or user can manually trigger: `/index`

2. **Use @codebase**
   ```
   @codebase what is the server implementation?
   ```
   - Returns intelligent summary with statistics and structure
   - Much faster than scanning filesystem

3. **Search for symbols**
   ```
   /search NewServer
   /search Handler
   ```
   - Returns list of matching symbols with locations

4. **Incremental updates**
   - File watcher automatically updates index when files change
   - No manual re-indexing needed

## Configuration Options

Users can customize indexing via config file:

```toml
[index]
enabled = true
max_file_size = 10485760  # 10MB
enable_watch = true
watch_debounce = "500ms"
ignore_patterns = [
    ".git",
    "node_modules",
    "__pycache__",
    ".venv",
    "vendor",
]
languages = []  # Empty = all supported languages
```

## Performance Tuning

### For Large Codebases (>10k files)

```go
config := index.DefaultConfig(wd)
config.MaxFileSize = 5 * 1024 * 1024  // Limit to 5MB files
config.IgnorePatterns = append(config.IgnorePatterns,
    "dist", "build", "out", "*.min.js",
)
```

### Disable Auto-Watch for Very Large Repos

```go
config.EnableWatch = false  // Manual re-index with /index
```

### Background Indexing with Progress

```go
go func() {
    ctx := context.Background()

    // Could emit progress events here
    if err := codebaseIndex.Index(ctx); err != nil {
        log.Printf("Indexing failed: %v", err)
    }

    // Notify UI that indexing is complete
    // p.Send(IndexCompleteMsg{})
}()
```

## Testing

### Unit Tests

```bash
cd internal/index
go test -v
```

### Integration Test

```bash
# Build and run TUI
go build
./rigrun-tui

# In TUI:
/index
/search NewServer
@codebase summarize this codebase
```

### Benchmark

```bash
cd internal/index
go test -bench=. -benchmem
```

## Troubleshooting

### Index not working

1. Check if index initialized:
   ```go
   if codebaseIndex == nil {
       log.Println("Codebase index not available")
   }
   ```

2. Check if indexed:
   ```go
   if !codebaseIndex.IsIndexed() {
       log.Println("Codebase not yet indexed - run /index")
   }
   ```

### Slow indexing

1. Check file count:
   ```bash
   # Too many files?
   find . -type f -name "*.go" | wc -l
   ```

2. Reduce scope:
   ```go
   config.IgnorePatterns = append(config.IgnorePatterns,
       "test", "testdata", "vendor",
   )
   ```

### Database errors

1. Check database path is writable:
   ```bash
   ls -la .rigrun/
   ```

2. Delete and recreate:
   ```bash
   rm .rigrun/codebase.db
   # Then run /index again
   ```

## Future Enhancements

- [ ] Add /goto command to jump to symbol definition
- [ ] Show symbol documentation in search results
- [ ] Cross-reference analysis (find all callers)
- [ ] Semantic search using embeddings
- [ ] Export index to JSON for external tools
- [ ] Multi-repository support
- [ ] Symbol usage statistics
