# Diff Package

The `diff` package provides file diff computation and formatting for the rigrun TUI. It implements a line-based diff algorithm similar to the classic Myers diff algorithm.

## Features

- **Line-based diffing**: Computes differences between file contents line by line
- **Unified diff format**: Generates standard unified diff output
- **Diff statistics**: Tracks additions, deletions, and file mode (new/modified/deleted)
- **Hunk grouping**: Organizes changes into hunks with context lines
- **LCS algorithm**: Uses Longest Common Subsequence for accurate diff computation

## Usage

### Basic Diff Computation

```go
import "github.com/jeranaias/rigrun-tui/internal/diff"

// Compute a diff between old and new content
oldContent := "line1\nline2\nline3"
newContent := "line1\nmodified\nline3\nline4"

d := diff.ComputeDiff("myfile.txt", oldContent, newContent)

// Access statistics
fmt.Printf("Additions: %d\n", d.Stats.Additions)
fmt.Printf("Deletions: %d\n", d.Stats.Deletions)
fmt.Printf("Mode: %s\n", d.Stats.FileMode)

// Get summary
fmt.Println(d.Summary()) // "Modified +2 -1"
```

### Unified Diff Format

```go
// Generate unified diff format
unified := diff.FormatUnifiedDiff(d)
fmt.Println(unified)

// Output:
// --- a/myfile.txt
// +++ b/myfile.txt
// @@ -1,3 +1,4 @@
//  line1
// -line2
// +modified
//  line3
// +line4
```

### Working with Diff Lines

```go
// Iterate through hunks and lines
for _, hunk := range d.Hunks {
    fmt.Printf("@@ -%d,%d +%d,%d @@\n",
        hunk.OldStart, hunk.OldCount,
        hunk.NewStart, hunk.NewCount)

    for _, line := range hunk.Lines {
        prefix := line.Type.Prefix() // " ", "+", or "-"
        fmt.Printf("%s%s\n", prefix, line.Content)
    }
}
```

### File Modes

The diff package automatically detects the file mode:

- **`new`**: Empty old content → new file
- **`deleted`**: Empty new content → file deleted
- **`modified`**: Both have content → file modified

```go
d := diff.ComputeDiff("file.txt", "", "new content")
fmt.Println(d.Stats.FileMode) // "new"

d = diff.ComputeDiff("file.txt", "old content", "")
fmt.Println(d.Stats.FileMode) // "deleted"

d = diff.ComputeDiff("file.txt", "old", "new")
fmt.Println(d.Stats.FileMode) // "modified"
```

## Types

### Diff

The main diff structure containing all diff information:

```go
type Diff struct {
    FilePath   string     // Path to the file
    OldContent string     // Original content
    NewContent string     // New content
    Hunks      []DiffHunk // The diff hunks
    Stats      DiffStats  // Statistics
}
```

### DiffHunk

A contiguous section of changes:

```go
type DiffHunk struct {
    OldStart int        // Starting line in old file
    OldCount int        // Number of lines in old file
    NewStart int        // Starting line in new file
    NewCount int        // Number of lines in new file
    Lines    []DiffLine // The diff lines
}
```

### DiffLine

A single line in a diff:

```go
type DiffLine struct {
    Type    DiffLineType // Added, Removed, or Context
    Content string       // The line content
    OldLine int          // Line number in old file (0 if added)
    NewLine int          // Line number in new file (0 if removed)
}
```

### DiffLineType

The type of a diff line:

```go
const (
    DiffLineContext  // Unchanged context line
    DiffLineAdded    // Added line
    DiffLineRemoved  // Removed line
)
```

## Algorithm

The diff implementation uses a simplified LCS (Longest Common Subsequence) algorithm:

1. **Split content into lines**: Handle empty lines and trailing newlines correctly
2. **Compute LCS**: Find the longest sequence of common lines
3. **Generate diff lines**: Mark lines as added, removed, or context
4. **Group into hunks**: Create hunks with configurable context (default: 3 lines)

## Integration with Tools

The diff package integrates with the tool system via `internal/tools/diff_integration.go`:

```go
import "github.com/jeranaias/rigrun-tui/internal/tools"

// Get diff preview for Edit tool
preview, err := tools.GetToolDiffPreview("Edit", params)
if err != nil {
    // handle error
}

// Show the diff to the user
fmt.Println(diff.FormatUnifiedDiff(preview.Diff))
```

## Testing

Run tests with:

```bash
go test -v ./internal/diff
```

Tests cover:
- New file creation
- File deletion
- File modification
- No changes
- LCS computation
- Hunk grouping
- Unified diff formatting
- Edge cases (empty files, single lines, etc.)

## Performance

The diff algorithm has:
- **Time complexity**: O(n*m) where n and m are the number of lines
- **Space complexity**: O(n*m) for the DP table
- **Optimized for**: Small to medium files (< 10,000 lines)

For very large files, consider using streaming or incremental diff approaches.

## Future Enhancements

Possible improvements:
- Word-level diffing for better granularity
- Patience diff algorithm for better results
- Syntax-aware diffing for code
- Binary file detection and handling
- Performance optimization for large files
- Configurable context line count
