# Vim Mode for rigrun Go TUI

## Overview

Vim Mode provides vim-style modal editing for the rigrun chat interface. This feature allows vim users to navigate and interact with the chat using familiar keybindings.

## Modes

### Normal Mode (NORMAL)

The default mode for navigation and commands. Press `Esc` from any other mode to return to Normal mode.

**Navigation Keys:**
- `j` / `k` - Scroll down/up one line
- `Ctrl+d` / `Ctrl+u` - Scroll half page down/up
- `Ctrl+f` / `Ctrl+b` - Scroll full page down/up
- `gg` - Go to top (press `g` twice)
- `G` - Go to bottom
- `h` / `l` - Move cursor left/right (when in input)

**Numeric Prefixes:**
- `5j` - Scroll down 5 lines
- `10k` - Scroll up 10 lines
- Works with most navigation commands

**Mode Transitions:**
- `i` - Enter insert mode at cursor
- `a` - Enter insert mode after cursor
- `I` - Enter insert mode at start of line
- `A` - Enter insert mode at end of line
- `o` - Open new line below (enter insert mode)
- `O` - Open new line above (enter insert mode)
- `v` - Enter visual mode
- `:` - Enter command mode
- `/` - Enter search mode

### Insert Mode (INSERT)

Text editing mode where you can type normally.

**Keys:**
- `Esc` - Return to normal mode
- All other keys work as normal text input

### Visual Mode (VISUAL)

Visual selection mode for copying text.

**Keys:**
- `j` / `k` - Extend selection down/up
- `y` - Yank (copy) selection and return to normal mode
- `Esc` - Cancel selection and return to normal mode

### Command Mode (COMMAND)

Execute vim-style commands by typing `:` in normal mode.

**Available Commands:**
- `:w` or `:save` - Save conversation to file
- `:q` or `:quit` - Quit rigrun
- `:wq` - Save and quit
- `:help` - Show help overlay
- `:set vim` - Enable vim mode
- `:set novim` - Disable vim mode

**Keys:**
- `Esc` - Cancel command and return to normal mode
- `Enter` - Execute command
- `Backspace` - Delete character from command buffer

## Configuration

### Enable/Disable Vim Mode

Vim mode can be configured in `~/.rigrun/config.toml`:

```toml
[ui]
vim_mode = true  # Enable vim mode (default: false)
```

### Runtime Toggle

You can toggle vim mode at runtime using:
- Command mode: `:set vim` or `:set novim`
- This will update the config file to persist the setting

## Visual Indicators

### Mode Indicator in Status Bar

The current vim mode is displayed in the status bar (right side):
- **NORMAL** (cyan, bold) - Normal mode
- **INSERT** (green, bold) - Insert mode
- **VISUAL** (purple, bold) - Visual mode
- **COMMAND** (amber, bold) - Command mode

### Command Buffer

When in command mode, the command buffer is shown in the input area with a `:` prefix.

## Integration with Existing Features

### Search Mode

Vim mode integrates with the existing search functionality:
- Press `/` in normal mode to enter search
- `n` - Next match (vim-style)
- `N` - Previous match (vim-style)
- `Esc` - Exit search mode

### Input Mode vs. Vim Mode

- When vim mode is **disabled**: always in insert mode (normal behavior)
- When vim mode is **enabled**: toggle between normal and insert modes
- The `inputMode` field tracks whether the text input is focused

## Implementation Details

### VimHandler

The `VimHandler` struct manages vim mode state and key handling:

```go
type VimHandler struct {
    mode          VimMode  // Current mode (Normal, Insert, Visual, Command)
    enabled       bool     // Whether vim mode is active
    commandBuffer string   // For : commands
    searchBuffer  string   // For / search
    visualStart   int      // Start position for visual selection
    visualEnd     int      // End position for visual selection
    count         int      // Numeric prefix (e.g., 5j for 5 lines down)
    lastG         bool     // Track if g was just pressed (for gg)
}
```

### Key Handling Flow

1. Tutorial overlay (highest priority)
2. Command palette
3. Help overlay
4. Search mode
5. **Vim mode handler** (if enabled)
6. Global keys (Ctrl+C, Ctrl+Q, etc.)
7. Standard input handling

### State Management

- Vim mode state is stored in `Model.vimHandler`
- Config is updated when toggling via `:set vim`/`:set novim`
- Mode indicator is rendered in the status bar
- Input mode is synchronized with vim mode

## Testing

Comprehensive tests are provided in `vim_test.go`:

```bash
go test ./internal/ui/chat -run TestVimHandler
```

**Test Coverage:**
- Mode transitions (Normal ↔ Insert ↔ Visual ↔ Command)
- Navigation keys (j, k, gg, G, Ctrl+d, Ctrl+u)
- Numeric prefixes (5j, 10k)
- Command execution (:w, :q, :wq, :help)
- Set commands (:set vim, :set novim)
- Disabled mode passthrough

## Acceptance Criteria

- [x] j/k scrolls viewport
- [x] i enters insert mode
- [x] Esc returns to normal mode
- [x] / activates search
- [x] :w saves conversation
- [x] Can disable vim mode in config
- [x] Mode shown in status bar (NORMAL, INSERT, VISUAL, COMMAND)
- [x] gg goes to top
- [x] G goes to bottom
- [x] Numeric prefixes work (5j, 10k)
- [x] Visual mode for selection
- [x] Command mode for : commands
- [x] :set vim/:set novim to toggle
- [x] Configurable via config.toml

## Future Enhancements

Potential improvements for future versions:

1. **Clipboard Integration**: Implement `y` (yank) with system clipboard
2. **Search Integration**: Better integration with / search (highlight matches)
3. **Marks**: Vim-style marks for quick navigation (`ma`, `'a`)
4. **Registers**: Multiple copy/paste registers
5. **Macros**: Record and replay command sequences
6. **More Commands**: `:e` (edit), `:n` (next), etc.
7. **Insert Mode Shortcuts**: Ctrl+W (delete word), Ctrl+U (delete line)
8. **Replace Mode**: `R` for replace mode
9. **Change Commands**: `cw` (change word), `cc` (change line)
10. **Delete Commands**: `dd` (delete line), `dw` (delete word)

## Known Limitations

1. Yank (copy) does not currently integrate with system clipboard
2. Visual mode selection is based on viewport offset, not actual text
3. No undo/redo support within vim mode
4. Limited command set compared to full vim
5. No ex commands beyond the basic set

## Credits

Inspired by vim's modal editing paradigm and designed to feel natural for vim users while maintaining rigrun's existing functionality.
