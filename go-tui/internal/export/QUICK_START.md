# Export Package - Quick Start

## TL;DR

Export rigrun conversations to beautiful Markdown, HTML, or JSON files.

## Basic Usage

### From Command Line (User)

```
/export              # → Markdown (default)
/export markdown     # → .md file
/export html         # → .html file
/export json         # → .json file
```

Files saved to `./exports/` and opened automatically.

### From Code (Developer)

```go
import "github.com/jeranaias/rigrun-tui/internal/export"

// Quick export (one line!)
path, err := export.ExportModelConversation(conversation, "html", nil)

// With options
opts := export.DefaultOptions()
opts.Theme = "light"
opts.OutputDir = "./my-exports"
path, err := export.ExportModelConversation(conversation, "markdown", opts)
```

## Formats

| Format | Extension | Features |
|--------|-----------|----------|
| Markdown | `.md` | YAML frontmatter, code fences, emojis |
| HTML | `.html` | Dark/light themes, interactive, print-friendly |
| JSON | `.json` | Complete data, machine-readable |

## Output Example

Input: `/export html`

Output:
```
✅ Successfully exported to: ./exports/conversation_my_chat_20260124_143052.html
```

File opens in browser with beautiful styling!

## HTML Preview

```html
┌─────────────────────────────────┐
│  My Chat                    🌓  │ ← Click to toggle theme
├─────────────────────────────────┤
│ Model: qwen2.5-coder:14b        │
│ Messages: 5 • Tokens: 1234      │
├─────────────────────────────────┤
│ 👤 User          14:30:52       │
│ How do I write Python?          │
├─────────────────────────────────┤
│ 🤖 Assistant     14:30:54       │
│ Here's how:                     │
│ ┌─ PYTHON ─────────────────┐   │
│ │ print("Hello, World!")    │   │
│ └───────────────────────────┘   │
│ 📊 45 tokens • 1.2s • 37 tok/s │
└─────────────────────────────────┘
```

## Customization

```go
opts := &export.Options{
    OutputDir:         "./exports",    // Where to save
    OpenAfterExport:   true,           // Auto-open?
    IncludeMetadata:   true,           // Show stats?
    IncludeTimestamps: true,           // Show times?
    Theme:             "dark",         // HTML theme
}
```

## That's It!

For more details, see:
- `README.md` - Full API documentation
- `example_test.go` - Code examples
- `../../../EXPORT_INTEGRATION.md` - Integration guide

## Common Tasks

### Export current conversation
```go
path, err := export.ExportModelConversation(m.conversation, "html", nil)
```

### Export stored conversation
```go
conv, _ := store.Load("conv_123")
path, err := export.ExportHTML(conv, nil)
```

### Custom exporter
```go
exporter := export.NewMarkdownExporter(opts)
path, err := export.ExportToFile(conv, exporter, opts)
```

## Tips

💡 HTML exports work offline (no CDN dependencies)
💡 Theme preference saved in browser localStorage
💡 Markdown exports work great in GitHub/Notion
💡 JSON exports preserve all metadata
💡 Filenames are auto-sanitized for safety
💡 Works on Windows, macOS, and Linux

## Help

Error: "conversation is nil"
→ Make sure conversation has messages

Error: "unsupported platform"
→ Auto-open only works on Windows/macOS/Linux

Theme toggle not working?
→ Enable JavaScript in browser
