# rigrun UX 2.0 Design Specification

**Goal**: Claude Code-level polish with rigrun's unique identity.

---

## Design Principles (Extracted from Claude Code)

1. **Context First** - Always show what matters (model, mode, context usage)
2. **Progressive Disclosure** - Details on demand, not by default
3. **Semantic Colors** - Green=success, Red=error, Yellow=warning, Cyan=brand
4. **Adaptive Layout** - Respond to terminal width gracefully
5. **Keyboard-First** - Rich shortcuts with visible hints
6. **Accessibility** - Text labels with every color/symbol

---

## Color Palette (rigrun branded)

```rust
// Primary brand colors
pub const RIGRUN_CYAN: &str = "\x1b[96m";      // Brand identity, prompts
pub const RIGRUN_PURPLE: &str = "\x1b[35m";    // Thinking, processing

// Semantic colors
pub const SUCCESS: &str = "\x1b[32m";          // Green - success, local mode
pub const WARNING: &str = "\x1b[33m";          // Yellow - warnings, cloud mode
pub const ERROR: &str = "\x1b[31m";            // Red - errors, critical
pub const INFO: &str = "\x1b[36m";             // Cyan - info messages

// Typography
pub const BOLD: &str = "\x1b[1m";
pub const DIM: &str = "\x1b[2m";
pub const RESET: &str = "\x1b[0m";
```

---

## 1. Input/Prompt Experience

### Current
```
rigrun>
```

### New Design
```
────────────────────────────────────────────────────────────
> [cursor here]
────────────────────────────────────────────────────────────
  local | qwen2.5:14b | GPU | Ctx: ████████░░ 45%
```

### Components
- **Separator line**: `─` repeated to terminal width
- **Prompt**: Bold cyan `>` with blinking cursor
- **Footer bar**: Mode | Model | GPU | Context usage

### Multi-line Input
```
> def calculate_total(items):
    return sum(item.price for item in items)
```
- First line: `> ` prefix
- Continuation: `  ` (2-space indent)

---

## 2. Thinking Indicator

### Current
```
◐ Thinking... 0:02.345s
```

### New Design
```
∴ Thinking ··· 2.3s
```

### Animation Frames
```rust
const THINKING_DOTS: &[&str] = &["·  ", "·· ", "···", " ··", "  ·", "   "];
```
- Symbol: `∴` (therefore) in purple - represents reasoning
- Dots animate at 150ms intervals
- Time shown in compact format (2.3s not 0:02.345s)

### With Details (Ctrl+D toggle)
```
∴ Thinking ··· 2.3s
  ├─ Processing query...
  ├─ Context: 2,048 tokens
  └─ Model: qwen2.5-coder:14b
```

---

## 3. Context Usage Bar

### Design
```
Ctx: ████████░░ 45%
```

### Implementation
```rust
fn render_context_bar(used: usize, total: usize) -> String {
    let percent = (used as f64 / total as f64 * 100.0) as usize;
    let filled = percent / 10;  // 10 chars total
    let empty = 10 - filled;

    let color = if percent >= 90 {
        ERROR
    } else if percent >= 75 {
        WARNING
    } else {
        RIGRUN_CYAN
    };

    format!("{}Ctx: {}{}{}░{} {}%{}",
        DIM, color,
        "█".repeat(filled),
        "░".repeat(empty),
        RESET, percent, RESET
    )
}
```

---

## 4. Status Bar (Adaptive)

### Narrow (<60 cols)
```
[local|GPU] 45%
```

### Medium (60-100 cols)
```
local | qwen2.5:14b | GPU | Ctx: 45% | /help
```

### Wide (>100 cols)
```
┌─ rigrun ──────────────────────────────────────────────────────────────┐
│ local | qwen2.5-coder:14b | GPU: RX 9070 XT | Ctx: ████████░░ 45%    │
│ Ctrl+C=stop · /help · Ctrl+D=details                                  │
└───────────────────────────────────────────────────────────────────────┘
```

### Implementation
```rust
fn render_status_bar(&self, width: usize) -> String {
    if width < 60 {
        self.render_narrow()
    } else if width < 100 {
        self.render_medium()
    } else {
        self.render_wide()
    }
}
```

---

## 5. Response Streaming

### First Token Arrival
```
∴ Thinking ··· 2.3s

[clears thinking indicator, starts streaming response]

Here's how you can implement...
```

### Code Block Treatment
```
```rust
fn example() {
    println!("Hello, world!");
}
```                                              [Copy]
```

- Language badge in top-left
- Copy button (optional, for interactive terminals)
- Syntax highlighting via syntect

### Response Footer
```
━━━ 2.5s │ 128 tokens │ 51 tok/s │ TTFT 234ms
```

---

## 6. Error Messages

### Current
```
✗ Model not found
```

### New Design
```
┌─ Error ─────────────────────────────────────────┐
│ ✗ Model 'unknown-model' not found               │
│                                                 │
│   Available models:                             │
│     • qwen2.5-coder:14b                        │
│     • codestral:22b                            │
│                                                 │
│   Tip: ollama pull unknown-model               │
└─────────────────────────────────────────────────┘
```

### Pattern
```rust
fn format_error(title: &str, details: &str, suggestions: &[&str]) -> String {
    let mut output = format!("{}┌─ {} ─{}┐{}\n", ERROR, title, "─".repeat(40), RESET);
    output += &format!("{}│ ✗ {}{}\n", ERROR, details, RESET);

    if !suggestions.is_empty() {
        output += &format!("{}│{}\n", DIM, RESET);
        for suggestion in suggestions {
            output += &format!("{}│   • {}{}\n", DIM, suggestion, RESET);
        }
    }

    output += &format!("{}└{}┘{}\n", ERROR, "─".repeat(45), RESET);
    output
}
```

---

## 7. Keyboard Shortcuts

### Always Visible (in status bar footer)
```
Ctrl+C=stop · /help · Tab=complete
```

### Help Menu (/help)
```
┌─ rigrun Commands ───────────────────────────────┐
│                                                 │
│  Navigation                                     │
│    /help, /?      Show this help               │
│    /exit, /q      Exit rigrun                  │
│                                                 │
│  Conversation                                   │
│    /new           Start new conversation       │
│    /save          Save current conversation    │
│    /resume <id>   Resume saved conversation    │
│                                                 │
│  Settings                                       │
│    /model <name>  Switch model                 │
│    /mode <mode>   Switch routing mode          │
│    /statusbar     Cycle status bar style       │
│                                                 │
│  Keyboard Shortcuts                             │
│    Ctrl+C         Stop generation              │
│    Ctrl+D         Toggle thinking details      │
│    Tab            Auto-complete                │
│    ↑/↓            History navigation           │
│                                                 │
└─────────────────────────────────────────────────┘
```

---

## 8. Visual Hierarchy

### Spacing Rules
```
Between messages: 1 blank line
Inside code blocks: 1rem padding equivalent
After response: stats footer (no extra line)
Between sections: separator line
```

### Typography
```
Headers:     BOLD + CYAN
Primary:     Normal + foreground
Secondary:   DIM
Emphasis:    BOLD
Code:        Monospace (terminal default)
```

---

## Implementation Priority

1. **Phase 1**: Thinking indicator + context bar (highest impact)
2. **Phase 2**: Adaptive status bar + separator lines
3. **Phase 3**: Error message boxes + help menu
4. **Phase 4**: Code block enhancements

---

## Files to Modify

| File | Changes |
|------|---------|
| `src/colors.rs` | Add new color constants |
| `src/main.rs` | New thinking indicator, response formatting |
| `src/status_indicator.rs` | Adaptive layout, context bar |
| `src/cli/mod.rs` | Input separator rendering |
| `src/error.rs` | Boxed error formatting |

---

**"Professional polish. Local-first power. Your GPU, your data, your interface."**
