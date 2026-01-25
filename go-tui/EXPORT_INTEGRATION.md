# Export Feature Integration Guide

This document describes the Session Export feature implementation for rigrun TUI.

## Overview

The export feature allows users to share beautiful conversation logs in multiple formats:
- **Markdown** (.md) - With YAML frontmatter and syntax highlighting
- **HTML** (.html) - Beautiful styled output with dark/light theme support
- **JSON** (.json) - Complete machine-readable data

## Files Created

### Export Package (C:\rigrun\go-tui\internal\export\)

1. **export.go** - Core export interface and utilities
   - `Exporter` interface for format implementations
   - `Options` struct for export configuration
   - `ExportToFile()` - Main export function
   - File sanitization and auto-open functionality

2. **markdown.go** - Markdown exporter
   - YAML frontmatter with metadata
   - Role labels with emojis (👤 User, 🤖 Assistant)
   - Code block preservation
   - Message statistics display

3. **html.go** - HTML exporter with embedded CSS
   - Dark/light theme support with toggle button
   - Syntax-highlighted code blocks
   - Responsive and print-friendly design
   - No external dependencies

4. **json.go** - JSON exporter
   - Pretty-printed JSON output
   - Complete conversation data preservation

5. **converter.go** - Conversion utilities
   - `ConvertModelToStored()` - Converts active conversations to storage format
   - `ExportModelConversation()` - One-stop export function

6. **README.md** - Package documentation

### Integration Files

7. **internal/ui/chat/export.go** - Chat UI export handlers
   - `handleExportConversation()` - Initiates export
   - `handleExportComplete()` - Handles export completion

### Modified Files

8. **internal/commands/handlers.go**
   - Enhanced `HandleExport()` with format validation
   - Added support for format aliases (md → markdown, htm → html)

9. **internal/ui/chat/commands.go**
   - Updated `handleExportCommand()` with new format support
   - Better validation and error messages

10. **internal/ui/chat/model.go**
    - Added message case handlers for `ExportConversationMsg` and `ExportCompleteMsg`

## Usage

### Command Line

Users can export conversations using the `/export` command:

```
/export              # Export to markdown (default)
/export markdown     # Export to .md file
/export html         # Export to .html file
/export json         # Export to .json file
/export md           # Alias for markdown
/export htm          # Alias for html
```

### Flow Diagram

```
User types:  /export html
     ↓
HandleExport (handlers.go)
     ↓ validates format
handleExportCommand (commands.go)
     ↓ creates message
ExportConversationMsg
     ↓
handleExportConversation (export.go)
     ↓ shows "Exporting..." message
ExportModelConversation() [async]
     ↓ converts conversation
     ↓ exports to file
     ↓ opens in default app
ExportCompleteMsg
     ↓
handleExportComplete (export.go)
     ↓
Shows "✅ Successfully exported to: path"
```

## Export Options

The export system uses configurable options:

```go
type Options struct {
    OutputDir         string  // Default: "./exports"
    OpenAfterExport   bool    // Default: true
    IncludeMetadata   bool    // Default: true
    IncludeTimestamps bool    // Default: true
    Theme             string  // Default: "dark" (for HTML)
}
```

Currently hardcoded in `handleExportConversation()` but can be made user-configurable.

## Output Files

Files are automatically named with:
- Sanitized conversation title
- Timestamp (YYYYMMDD_HHMMSS)
- Appropriate extension

Example: `conversation_debugging_rust_code_20260124_143052.html`

## HTML Theme Features

The HTML export includes sophisticated styling:

### Dark Theme (Default)
- Tokyo Night inspired color palette
- Syntax-highlighted code blocks
- Smooth transitions
- Professional appearance

### Light Theme
- GitHub-inspired color scheme
- High contrast for printing
- Clean and bright

### Interactive Features
- Click 🌓 button to toggle theme
- Theme preference saved in localStorage
- Responsive design for mobile
- Print-optimized styles (@media print)

## Code Block Handling

Code blocks are preserved with syntax highlighting:

**Markdown Input:**
````markdown
```python
def hello():
    print("Hello, world!")
```
````

**HTML Output:**
- Language label badge showing "PYTHON"
- Monospace font rendering
- Syntax preservation
- Horizontal scroll for long lines

## Metadata Included

Both Markdown and HTML exports include:

- **Session Info**
  - Conversation ID
  - Model used
  - Creation timestamp
  - Last updated timestamp
  - Total message count
  - Token usage (if available)

- **Per-Message Stats** (for assistant messages)
  - Token count
  - Duration
  - Time to first token (TTFT)
  - Tokens per second

- **Context Information**
  - @mention tracking
  - File inclusions
  - Git context
  - Codebase references

## Security Considerations

- Filename sanitization prevents path traversal
- All HTML content is properly escaped
- No external JavaScript dependencies
- Embedded CSS only (no CDN calls)

## Future Enhancements

Potential improvements for future versions:

1. **PDF Export** (deferred)
   - Convert HTML to PDF using headless browser
   - Requires external dependency or library

2. **Theme Customization**
   - User-configurable color schemes
   - CSS variable overrides

3. **Export Settings**
   - Save export preferences in config
   - Custom output directory per user
   - Default format selection

4. **Batch Export**
   - Export multiple sessions at once
   - Export entire session history

5. **Share Links**
   - Generate shareable links
   - Upload to gist or pastebin

## Testing

To test the export functionality:

1. Start rigrun TUI
2. Have a conversation with the assistant
3. Type `/export markdown` - should create .md file and open it
4. Type `/export html` - should create .html file and open it
5. Type `/export json` - should create .json file and open it

Verify:
- Files are created in `./exports/` directory
- Files open in default application
- Content is properly formatted
- Syntax highlighting works in HTML
- Theme toggle works in HTML
- Metadata is accurate

## Troubleshooting

### Export fails with "conversation is nil"
- Ensure there's an active conversation before exporting
- Check that messages exist in the conversation

### File doesn't open automatically
- Check OS-specific open command support
- Windows: `cmd /c start`
- macOS: `open`
- Linux: `xdg-open`

### HTML theme doesn't toggle
- Check browser localStorage support
- Clear browser cache if needed

### Code blocks not rendering correctly
- Verify markdown code fence syntax
- Check for triple backticks in content

## Package Dependencies

The export package depends on:
- `internal/model` - For conversation data structures
- `internal/storage` - For stored conversation format
- Standard library only (no external deps)

## Acceptance Criteria

✅ `/export markdown` creates styled .md file
✅ `/export html` creates styled .html file
✅ Syntax highlighting in code blocks
✅ Metadata header (date, model, stats)
✅ Opens in default app
✅ Dark/light theme support in HTML
✅ Responsive design
✅ Print-friendly styles
✅ YAML frontmatter in Markdown
✅ Role labels (User, Assistant, System)
✅ Timestamp for each message

## Code Quality

- Well-documented with inline comments
- Follows Go best practices
- Type-safe interfaces
- Error handling throughout
- No panics in production code
- Proper resource cleanup
