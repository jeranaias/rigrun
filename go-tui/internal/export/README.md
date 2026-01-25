# Export Package

Session export functionality for rigrun TUI with beautiful styling.

## Features

- **Multiple Formats**: Export to Markdown, HTML, or JSON
- **Styled Output**: Beautiful CSS styling for HTML exports with dark/light theme support
- **Syntax Highlighting**: Code blocks with language detection
- **Metadata**: Include session metadata (model, timestamps, token stats)
- **Auto-Open**: Automatically opens exported files in default application
- **Print-Friendly**: HTML exports include print-optimized styles

## Usage

### Exporting Active Conversations

```go
import (
    "github.com/jeranaias/rigrun-tui/internal/export"
    "github.com/jeranaias/rigrun-tui/internal/model"
)

// Export current conversation to Markdown
opts := export.DefaultOptions()
opts.OutputDir = "./exports"
opts.Theme = "dark" // or "light" for HTML exports

path, err := export.ExportModelConversation(conversation, "markdown", opts)
if err != nil {
    // Handle error
}
fmt.Printf("Exported to: %s\n", path)
```

### Exporting Stored Conversations

```go
import (
    "github.com/jeranaias/rigrun-tui/internal/export"
    "github.com/jeranaias/rigrun-tui/internal/storage"
)

// Load a stored conversation
store, _ := storage.NewConversationStore()
conv, _ := store.Load("conv_abc123")

// Export to HTML
opts := export.DefaultOptions()
path, err := export.ExportHTML(conv, opts)
```

## Formats

### Markdown (.md)

- YAML frontmatter with metadata
- Proper code fence languages
- Role labels with emojis (👤 User, 🤖 Assistant)
- Timestamp for each message
- Message statistics (tokens, duration, speed)

### HTML (.html)

- Clean, readable CSS styling
- Dark/light theme support with toggle button
- Syntax-highlighted code blocks
- Responsive design
- Print-friendly styles
- Embedded CSS (no external dependencies)

### JSON (.json)

- Complete conversation data
- Preserves all metadata and statistics
- Machine-readable format

## Export Options

```go
type Options struct {
    // OutputDir is the directory where files will be saved
    OutputDir string

    // OpenAfterExport opens the file in the default application
    OpenAfterExport bool

    // IncludeMetadata includes metadata header
    IncludeMetadata bool

    // IncludeTimestamps includes per-message timestamps
    IncludeTimestamps bool

    // Theme for HTML export ("light" or "dark")
    Theme string
}
```

## Integration with Chat UI

To handle export in the chat model's Update function:

```go
case commands.ExportConversationMsg:
    format := msg.Format

    // Convert current conversation
    opts := export.DefaultOptions()
    opts.OutputDir = "./exports"

    // Export asynchronously
    return m, func() tea.Msg {
        path, err := export.ExportModelConversation(m.conversation, format, opts)
        return commands.ExportCompleteMsg{
            Path:  path,
            Error: err,
        }
    }

case commands.ExportCompleteMsg:
    if msg.Error != nil {
        m.conversation.AddSystemMessage(fmt.Sprintf("Export failed: %v", msg.Error))
    } else {
        m.conversation.AddSystemMessage(fmt.Sprintf("✓ Exported to: %s", msg.Path))
    }
    m.updateViewport()
    return m, nil
```

## Command Usage

Users can export conversations using the `/export` command:

```
/export markdown    # Export to .md file
/export html        # Export to .html file
/export json        # Export to .json file
/export md          # Alias for markdown
/export htm         # Alias for html
```

## File Naming

Exported files are automatically named with:
- Sanitized conversation summary
- Timestamp (YYYYMMDD_HHMMSS)
- Appropriate extension

Example: `conversation_my_first_chat_20260124_143052.html`

## HTML Theme Features

The HTML export includes:

### Dark Theme (default)
- Tokyo Night inspired color scheme
- Easy on the eyes for long reading
- Professional appearance

### Light Theme
- GitHub-inspired color scheme
- Clean and bright
- Print-optimized

### Theme Toggle
- Click the 🌓 button to toggle
- Preference saved in localStorage
- Smooth transitions

## Code Block Highlighting

Code blocks preserve the language from markdown:

````markdown
```python
def hello():
    print("Hello, world!")
```
````

Becomes a styled code block with:
- Language label badge
- Monospace font
- Proper syntax preservation
- Scroll for long code
