# Diff Viewer Component

The `DiffViewer` component provides a visual, interactive diff display for the rigrun TUI. It shows file changes with syntax highlighting and allows users to approve or reject changes before they're applied.

## Features

- **Visual diff display**: Shows added (green), removed (red), and context (gray) lines
- **Syntax highlighting**: Color-coded diff with line numbers
- **Interactive approval**: Users can approve (y/Enter) or reject (n/Esc) changes
- **Scrollable**: Navigate large diffs with arrow keys
- **Statistics**: Shows file mode and change counts (+X, -Y lines)
- **Unified diff format**: Displays hunks with @@ headers
- **Help text**: Built-in keyboard shortcuts guide

## Usage

### Basic Example

```go
import (
    "github.com/jeranaias/rigrun-tui/internal/diff"
    "github.com/jeranaias/rigrun-tui/internal/ui/components"
)

// Compute a diff
oldContent := "line1\nline2\nline3"
newContent := "line1\nmodified\nline3"
d := diff.ComputeDiff("myfile.txt", oldContent, newContent)

// Create viewer
viewer := components.NewDiffViewer(d)
viewer.SetSize(80, 24)

// Render the view
output := viewer.View()
fmt.Println(output)
```

### Approval Flow

```go
// Create viewer
viewer := components.NewDiffViewer(d)

// User approves the changes
viewer.Approve()

if viewer.IsApproved() {
    // Apply the changes
    applyDiff(d)
}

// Or user rejects
viewer.Reject()

if viewer.IsRejected() {
    // Cancel the operation
    cancelDiff()
}
```

### Scrolling

```go
viewer := components.NewDiffViewer(d)

// Scroll up 5 lines
viewer.ScrollUp(5)

// Scroll down 10 lines
viewer.ScrollDown(10)
```

## Integration with Bubble Tea

### Message Types

Add these message types to your chat messages:

```go
// DiffPendingMsg indicates a diff is pending approval
type DiffPendingMsg struct {
    MessageID  string
    ToolName   string
    ToolID     string
    FilePath   string
    OldContent string
    NewContent string
}

// DiffApprovedMsg indicates user approved the diff
type DiffApprovedMsg struct {
    MessageID string
    ToolID    string
}

// DiffRejectedMsg indicates user rejected the diff
type DiffRejectedMsg struct {
    MessageID string
    ToolID    string
    Reason    string
}
```

### Update Handler

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case DiffPendingMsg:
        // Compute diff
        d := diff.ComputeDiff(msg.FilePath, msg.OldContent, msg.NewContent)

        // Create viewer
        m.diffViewer = components.NewDiffViewer(d)
        m.diffViewer.SetSize(m.width, m.height)
        m.showingDiff = true

        return m, nil

    case tea.KeyMsg:
        if m.showingDiff {
            switch msg.String() {
            case "y", "enter":
                m.diffViewer.Approve()
                m.showingDiff = false
                return m, func() tea.Msg {
                    return DiffApprovedMsg{
                        MessageID: m.currentMessageID,
                        ToolID:    m.currentToolID,
                    }
                }

            case "n", "esc":
                m.diffViewer.Reject()
                m.showingDiff = false
                return m, func() tea.Msg {
                    return DiffRejectedMsg{
                        MessageID: m.currentMessageID,
                        ToolID:    m.currentToolID,
                        Reason:    "User rejected changes",
                    }
                }

            case "up", "k":
                m.diffViewer.ScrollUp(1)
                return m, nil

            case "down", "j":
                m.diffViewer.ScrollDown(1)
                return m, nil
            }
        }
    }

    return m, nil
}
```

### View Rendering

```go
func (m Model) View() string {
    if m.showingDiff {
        // Show diff viewer
        return m.diffViewer.View()
    }

    // Normal view
    return m.renderChat()
}
```

## Visual Layout

The diff viewer displays:

```
┌─────────────────────────────────────────┐
│        File Diff Preview                │
│                                         │
│ Modified +2 -1 lines                    │
│                                         │
│ /path/to/myfile.txt                     │
│ ─────────────────────────────────────── │
│                                         │
│ @@ -1,3 +1,3 @@                         │
│    1    1  line1                        │
│    2       -line2                       │
│       2   +modified                     │
│    3    3  line3                        │
│                                         │
│ Review the changes above:               │
│                                         │
│   y / Enter  - Approve and apply        │
│   n / Esc    - Reject changes           │
└─────────────────────────────────────────┘
```

## Color Scheme

- **Header**: Primary color, bold, underlined
- **File path**: Primary color, bold
- **New file**: Text muted, italic
- **Modified**: Text muted, italic
- **Additions (+)**: Success green with dark green background
- **Deletions (-)**: Danger red with dark red background
- **Context**: Text muted (gray)
- **Hunk headers (@@ ...)**: Info blue with dim background
- **Line numbers**: Text muted
- **Approved status**: Success green, bold
- **Rejected status**: Danger red, bold

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `y` | Approve changes |
| `Enter` | Approve changes |
| `n` | Reject changes |
| `Esc` | Reject changes |
| `↑` / `k` | Scroll up |
| `↓` / `j` | Scroll down |
| `PageUp` | Scroll up one page |
| `PageDown` | Scroll down one page |

## Configuration

```go
// Create with custom size
viewer := components.NewDiffViewer(d)
viewer.SetSize(120, 40)  // wider view

// Hide help text
viewer.showHelp = false

// Start at specific scroll position
viewer.scrollPos = 10
```

## Best Practices

1. **Always show diff for Edit/Write**: Let users see what will change
2. **Require approval**: Don't auto-apply changes
3. **Save diffs to session**: Keep history of all changes
4. **Handle rejections gracefully**: Explain why operation was cancelled
5. **Provide context**: Show enough lines around changes
6. **Limit size**: For very large diffs, consider pagination or summary

## Example: Complete Edit Flow

```go
// 1. Tool requests Edit
toolCall := ToolCallRequestedMsg{
    ToolName: "Edit",
    Arguments: map[string]interface{}{
        "file_path": "main.go",
        "old_string": "oldFunc()",
        "new_string": "newFunc()",
    },
}

// 2. Get diff preview
preview, err := tools.GetToolDiffPreview("Edit", toolCall.Arguments)
if err != nil {
    return handleError(err)
}

// 3. Show diff viewer
return showDiffApproval(preview)

// 4. Wait for user response
// (handled in Update via DiffApprovedMsg or DiffRejectedMsg)

// 5. If approved, execute the tool
if approved {
    result := executeEdit(toolCall.Arguments)
    saveDiffToSession(preview, true)
} else {
    saveDiffToSession(preview, false)
}
```

## Testing

Test the diff viewer:

```bash
go test -v ./internal/ui/components -run TestDiffViewer
```

Tests cover:
- Creation and initialization
- Approval/rejection state
- Scrolling behavior
- Size configuration
- View rendering for different diff types
- State transitions

## Performance

The diff viewer is optimized for:
- **Small to medium diffs**: < 1000 lines
- **Typical file edits**: < 100 changes
- **Fast rendering**: Lipgloss-based rendering

For very large diffs (> 10,000 lines), consider:
- Pagination or virtual scrolling
- Summary view with expandable hunks
- Separate diff file viewer

## Troubleshooting

### Diff not showing colors

Ensure terminal supports 256 colors:
```bash
echo $TERM  # should be xterm-256color or similar
```

### Layout issues

Adjust viewer size to match terminal:
```go
viewer.SetSize(termWidth, termHeight)
```

### Long lines wrapping

For files with very long lines, consider:
- Horizontal scrolling
- Line truncation with ellipsis
- Word wrap mode
