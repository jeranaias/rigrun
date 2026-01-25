# Vim Mode Quick Reference

## Enabling Vim Mode

Add to `~/.rigrun/config.toml`:
```toml
[ui]
vim_mode = true
```

Or toggle at runtime: `:set vim` / `:set novim`

---

## Modes

| Mode | Indicator | Purpose |
|------|-----------|---------|
| **NORMAL** | Cyan | Navigation & commands |
| **INSERT** | Green | Text editing |
| **VISUAL** | Purple | Text selection |
| **COMMAND** | Amber | Execute commands |

---

## Normal Mode (NORMAL)

### Navigation
| Key | Action |
|-----|--------|
| `j` | Scroll down one line |
| `k` | Scroll up one line |
| `h` | Move cursor left |
| `l` | Move cursor right |
| `Ctrl+d` | Scroll half page down |
| `Ctrl+u` | Scroll half page up |
| `Ctrl+f` | Scroll full page down |
| `Ctrl+b` | Scroll full page up |
| `gg` | Go to top (press `g` twice) |
| `G` | Go to bottom |

### Numeric Prefixes
| Key | Action |
|-----|--------|
| `5j` | Scroll down 5 lines |
| `10k` | Scroll up 10 lines |
| Works with most navigation keys |

### Enter Other Modes
| Key | Action |
|-----|--------|
| `i` | Insert at cursor |
| `a` | Insert after cursor |
| `I` | Insert at start of line |
| `A` | Insert at end of line |
| `o` | Open new line below |
| `O` | Open new line above |
| `v` | Visual mode |
| `:` | Command mode |
| `/` | Search mode |

---

## Insert Mode (INSERT)

| Key | Action |
|-----|--------|
| `Esc` | Return to normal mode |
| All other keys work as normal text input |

---

## Visual Mode (VISUAL)

| Key | Action |
|-----|--------|
| `j` | Extend selection down |
| `k` | Extend selection up |
| `y` | Yank (copy) selection |
| `Esc` | Cancel and return to normal |

---

## Command Mode (COMMAND)

Enter by pressing `:` in normal mode.

### Commands
| Command | Action |
|---------|--------|
| `:w` or `:save` | Save conversation |
| `:q` or `:quit` | Quit rigrun |
| `:wq` | Save and quit |
| `:help` | Show help overlay |
| `:set vim` | Enable vim mode |
| `:set novim` | Disable vim mode |

### Keys
| Key | Action |
|-----|--------|
| `Enter` | Execute command |
| `Esc` | Cancel command |
| `Backspace` | Delete character |

---

## Search Mode

| Key | Action |
|-----|--------|
| `/` | Enter search (from normal mode) |
| `n` | Next match |
| `N` | Previous match |
| `Esc` | Exit search mode |

---

## Tips

1. **Start in Normal Mode**: When vim mode is enabled, you start in NORMAL mode. Press `i` to begin typing.

2. **Muscle Memory**: If you're a vim user, the keybindings should feel natural. If not, you can disable vim mode with `:set novim`.

3. **Status Bar**: Check the right side of the status bar to see which mode you're in.

4. **Command Buffer**: When in command mode, you'll see your typed command with a `:` prefix in the input area.

5. **Always Escape**: Press `Esc` from any mode to return to normal mode.

6. **Numeric Multipliers**: Use numeric prefixes for efficient navigation (e.g., `20j` to scroll down 20 lines).

---

## Common Workflows

### Quick Navigation
```
gg      # Jump to top
G       # Jump to bottom
10j     # Scroll down 10 lines
/error  # Search for "error"
n       # Jump to next match
```

### Editing
```
i       # Enter insert mode
(type your message)
Esc     # Back to normal mode
:w      # Save conversation
:q      # Quit
```

### Efficient Scrolling
```
Ctrl+d  # Scroll down half page
Ctrl+u  # Scroll up half page
5j      # Scroll down 5 lines
gg      # Back to top
```

---

## Disabling Vim Mode

If you prefer standard editing:

**Runtime**: `:set novim`

**Config**: Set `vim_mode = false` in `~/.rigrun/config.toml`

When disabled, the interface works like a standard text editor (always in insert mode).
