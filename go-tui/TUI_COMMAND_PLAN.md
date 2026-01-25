# TUI Slash Command Plan for rigrun

This document provides a comprehensive inventory of current TUI slash commands, proposed new commands, and an implementation plan for autofill/autocomplete functionality.

---

## 1. Current Commands Inventory

### 1.1 Commands Implemented in `internal/ui/chat/model.go`

These commands are directly handled in the `handleCommand()` function:

| Command | Aliases | Description | Status |
|---------|---------|-------------|--------|
| `/help` | `/h`, `/?` | Show help and available commands | Implemented |
| `/clear` | `/c` | Clear conversation history | Implemented |
| `/new` | `/n` | Start a new conversation | Implemented |
| `/quit` | `/q`, `/exit` | Exit rigrun | Implemented |
| `/model` | `/m` | Switch or show current model | Implemented |
| `/mode` | - | Switch routing mode (local/cloud/hybrid) | Implemented |
| `/status` | - | Show detailed status information | Implemented |
| `/save` | `/s` | Save current conversation | Implemented |
| `/load` | `/l`, `/resume` | Load a saved conversation | Implemented |
| `/list` | `/sessions` | List saved sessions | Implemented |

### 1.2 Commands Defined in Registry (`internal/commands/registry.go`)

The command registry defines additional commands with full metadata:

| Command | Aliases | Category | Description | Handler Status |
|---------|---------|----------|-------------|----------------|
| `/help` | `/h`, `/?` | Navigation | Show help and available commands | Implemented |
| `/quit` | `/q`, `/exit` | Navigation | Exit rigrun | Implemented |
| `/new` | `/n` | Conversation | Start a new conversation | Implemented |
| `/save` | `/s` | Conversation | Save current conversation | Implemented |
| `/load` | `/l`, `/resume` | Conversation | Load a saved conversation | Implemented |
| `/clear` | `/c` | Conversation | Clear conversation history | Implemented |
| `/copy` | - | Conversation | Copy last response to clipboard | Handler exists, not wired |
| `/export` | - | Conversation | Export conversation to file | Handler exists, not wired |
| `/sessions` | `/list` | Conversation | List saved sessions | Implemented |
| `/model` | `/m` | Model | Switch or show current model | Implemented |
| `/models` | - | Model | List available models | Handler exists, not wired |
| `/mode` | - | Model | Switch routing mode | Implemented |
| `/tools` | - | Tools | List available tools | Handler exists, not wired |
| `/tool` | - | Tools | Enable or disable a tool | Handler exists, not wired |
| `/config` | - | Settings | Show or edit configuration | Handler exists, not wired |
| `/status` | - | Settings | Show detailed status information | Implemented |
| `/theme` | - | Settings | Change color theme | Hidden, not implemented |

### 1.3 Keyboard Shortcuts (Non-Command)

| Shortcut | Action |
|----------|--------|
| `Ctrl+C` | Cancel streaming / Exit |
| `Ctrl+F` | Search in conversation |
| `Ctrl+R` | Cycle routing mode |
| `Ctrl+L` | Clear screen |
| `PgUp/PgDown` | Scroll viewport |
| `Home/End` | Go to top/bottom of conversation |

### 1.4 Context Mentions (@ Mentions)

| Mention | Description |
|---------|-------------|
| `@file:<path>` | Include file content |
| `@clipboard` | Include clipboard content |
| `@git` | Include recent git info |
| `@codebase` | Include directory structure |
| `@error` | Include last error message |

---

## 2. CLI Commands Needing TUI Equivalents

### 2.1 Audit Commands (IL5 AU-6, AU-9, AU-11)

CLI: `rigrun audit [show|export|stats|clear]`

| TUI Command | CLI Equivalent | Priority | Notes |
|-------------|----------------|----------|-------|
| `/audit` | `rigrun audit show` | P1 | Show last 10-20 audit entries |
| `/audit <N>` | `rigrun audit show --lines N` | P1 | Show last N entries |
| `/audit stats` | `rigrun audit stats` | P2 | Show audit statistics |
| `/audit export` | `rigrun audit export` | P3 | Export to file (needs path) |

### 2.2 Session Commands (IL5 AC-12)

CLI: `rigrun session [list|show|export|delete]`

| TUI Command | CLI Equivalent | Priority | Notes |
|-------------|----------------|----------|-------|
| `/sessions` | `rigrun session list` | Implemented | Already exists |
| `/load <id>` | `rigrun session show` | Implemented | Already exists |
| `/export` | `rigrun session export` | P1 | Export current session |
| `/delete <id>` | `rigrun session delete` | P2 | Delete session (confirm) |

### 2.3 Classification Commands (IL5 DoDI 5200.48)

CLI: `rigrun classify [show|set|banner|validate]`

| TUI Command | CLI Equivalent | Priority | Notes |
|-------------|----------------|----------|-------|
| `/classify` | `rigrun classify show` | P1 | Show current level |
| `/classify <LEVEL>` | `rigrun classify set` | P1 | Set classification |
| `/classify levels` | `rigrun classify levels` | P2 | List available levels |

### 2.4 Consent Commands (IL5 AC-8)

CLI: `rigrun consent [show|status|accept|reset]`

| TUI Command | CLI Equivalent | Priority | Notes |
|-------------|----------------|----------|-------|
| `/consent` | `rigrun consent status` | P2 | Show consent status |
| `/consent reset` | `rigrun consent reset` | P2 | Force re-acknowledgment |

### 2.5 Test Commands (IL5 BIST)

CLI: `rigrun test [all|security|connectivity]`

| TUI Command | CLI Equivalent | Priority | Notes |
|-------------|----------------|----------|-------|
| `/test` | `rigrun test connectivity` | P3 | Quick connectivity test |
| `/doctor` | `rigrun doctor` | P2 | System diagnostics |

### 2.6 Config Commands

CLI: `rigrun config [show|set]`

| TUI Command | CLI Equivalent | Priority | Notes |
|-------------|----------------|----------|-------|
| `/config` | `rigrun config show` | P1 | Show all config |
| `/config <key>` | `rigrun config show` | P1 | Show specific key |
| `/config <key> <val>` | `rigrun config set` | P1 | Set config value |
| `/config cache stats` | `rigrun cache stats` | P1 | Cache statistics |
| `/config cache clear` | `rigrun cache clear` | P2 | Clear cache |

---

## 3. Proposed New Commands

### 3.1 Conversation Management (Priority 1)

| Command | Aliases | Description | Implementation Notes |
|---------|---------|-------------|---------------------|
| `/export` | `/e` | Export conversation to file | Format: json, md, txt. Default: md |
| `/copy` | `/cp` | Copy last response to clipboard | Use system clipboard API |
| `/retry` | `/r` | Retry last query | Resend last user message |
| `/undo` | - | Undo last message pair | Remove last user+assistant |

### 3.2 Search and Navigation (Priority 1)

| Command | Aliases | Description | Implementation Notes |
|---------|---------|-------------|---------------------|
| `/search` | `/find`, `/f` | Search in conversation | Already have Ctrl+F, add command |
| `/goto` | `/g` | Go to message by index | `/goto 5` jumps to message 5 |

### 3.3 Statistics and Monitoring (Priority 1)

| Command | Aliases | Description | Implementation Notes |
|---------|---------|-------------|---------------------|
| `/tokens` | `/t` | Show token usage for session | Input/output/total tokens |
| `/cost` | - | Show estimated cost so far | Track cloud API costs |
| `/time` | - | Show session duration | Active time, idle time |
| `/stats` | - | Combined tokens/cost/time | All-in-one view |

### 3.4 Security and Compliance (Priority 1)

| Command | Aliases | Description | Implementation Notes |
|---------|---------|-------------|---------------------|
| `/secure` | - | Enter secure mode | Disable cloud, clear history |
| `/classify` | - | View/set classification | IL5 DoDI 5200.48 |
| `/audit` | - | View audit log entries | IL5 AU-6 |
| `/offline` | - | Toggle offline mode | IL5 SC-7 |

### 3.5 Model and Tools (Priority 2)

| Command | Aliases | Description | Implementation Notes |
|---------|---------|-------------|---------------------|
| `/models` | - | List available Ollama models | Fetch from Ollama API |
| `/tools` | - | List available tools | Show enabled/disabled |
| `/tool <name> on/off` | - | Enable/disable specific tool | Per-tool control |
| `/context` | `/ctx` | Show current context window usage | Tokens used / max |

### 3.6 Utility Commands (Priority 2)

| Command | Aliases | Description | Implementation Notes |
|---------|---------|-------------|---------------------|
| `/doctor` | - | Run system diagnostics | Connection, GPU, memory |
| `/reset` | - | Reset all settings to default | Confirm required |
| `/pin` | - | Pin current message | Mark for reference |
| `/share` | - | Share conversation link | If cloud enabled |

### 3.7 Workflow Commands (Priority 3)

| Command | Aliases | Description | Implementation Notes |
|---------|---------|-------------|---------------------|
| `/template` | - | Apply prompt template | `/template code-review` |
| `/persona` | - | Switch AI persona/system prompt | `/persona dod-analyst` |
| `/macro` | - | Define/run command macros | Power user feature |

---

## 4. Command Aliases Summary

### 4.1 Short Aliases (Single Character)

| Alias | Full Command |
|-------|--------------|
| `/c` | `/clear` |
| `/e` | `/export` |
| `/f` | `/find` (search) |
| `/g` | `/goto` |
| `/h` | `/help` |
| `/l` | `/load` |
| `/m` | `/model` |
| `/n` | `/new` |
| `/q` | `/quit` |
| `/r` | `/retry` |
| `/s` | `/save` |
| `/t` | `/tokens` |

### 4.2 Alternative Aliases

| Aliases | Full Command |
|---------|--------------|
| `/?` | `/help` |
| `/exit` | `/quit` |
| `/resume` | `/load` |
| `/list` | `/sessions` |
| `/cp` | `/copy` |
| `/search` | `/find` |
| `/ctx` | `/context` |

---

## 5. Autofill/Autocomplete Implementation Plan

### 5.1 Current State

The `internal/commands/completion.go` already provides:
- `Completer` struct with completion logic
- `CompleteCommands()` for command name completion
- `CompleteArg()` for argument completion
- `CompletionState` for navigation state
- Support for various argument types (Model, Session, File, Enum, Tool, Config)

### 5.2 Implementation Steps

#### Phase 1: Basic Tab Completion (P1)

**Goal**: Press Tab to complete commands and arguments

**Changes to `internal/ui/chat/model.go`**:

```go
// Add to Model struct
completionState *commands.CompletionState
completer       *commands.Completer

// In handleKey(), add Tab handling:
case "tab":
    if m.state == StateReady {
        return m.handleTabCompletion()
    }

// New method:
func (m Model) handleTabCompletion() (tea.Model, tea.Cmd) {
    input := m.input.Value()
    cursorPos := m.input.Position()

    // Get completions
    completions := m.completer.Complete(input, cursorPos)

    if len(completions) == 0 {
        return m, nil
    }

    if len(completions) == 1 {
        // Single completion - apply immediately
        m.input.SetValue(completions[0].Value + " ")
        m.input.SetCursor(len(completions[0].Value) + 1)
        return m, nil
    }

    // Multiple completions - show palette
    m.completionState.Update(input, completions)
    return m, nil
}
```

#### Phase 2: Command Palette UI (P1)

**Goal**: Show visual dropdown when user types `/`

**New file: `internal/ui/components/command_palette.go`**:

```go
type CommandPalette struct {
    completions []commands.Completion
    selected    int
    visible     bool
    width       int
    height      int
    maxItems    int // Max visible items (default: 8)
}

func (p *CommandPalette) View() string {
    if !p.visible || len(p.completions) == 0 {
        return ""
    }

    var sb strings.Builder

    // Render visible completions
    start := max(0, p.selected - p.maxItems/2)
    end := min(len(p.completions), start + p.maxItems)

    for i := start; i < end; i++ {
        c := p.completions[i]
        style := normalStyle
        if i == p.selected {
            style = selectedStyle
        }

        line := fmt.Sprintf("%-20s %s", c.Value, c.Description)
        sb.WriteString(style.Render(line) + "\n")
    }

    return sb.String()
}
```

**Integration in chat view**:
- Render palette above input line when visible
- Navigate with Up/Down arrows
- Enter to select, Escape to cancel
- Filter as user types

#### Phase 3: Real-time Filtering (P2)

**Goal**: Filter command list as user types after `/`

```go
// In handleKey(), modify character input handling:
default:
    // Normal input handling
    var cmd tea.Cmd
    m.input, cmd = m.input.Update(msg)

    // Check if we should show/update completions
    newInput := m.input.Value()
    if strings.HasPrefix(newInput, "/") {
        completions := m.completer.Complete(newInput, len(newInput))
        m.completionState.Update(newInput, completions)
    } else if !strings.Contains(newInput, "@") {
        m.completionState.Clear()
    }

    return m, cmd
```

#### Phase 4: @ Mention Completion (P2)

**Goal**: Autocomplete @ mentions for context

Already partially implemented in `completion.go`:
- `completeMentions()` handles @ prefix
- `completeFiles()` handles `@file:` paths

Integration:
- Trigger on `@` character typed
- Show mention types (@file, @git, @clipboard, etc.)
- For @file:, show file browser

#### Phase 5: Dynamic Argument Completion (P3)

**Goal**: Context-aware argument completion

```go
// Set up dynamic completion callbacks in completer initialization
completer.ModelsFn = func() []string {
    if m.ollama != nil {
        models, _ := m.ollama.ListModels(context.Background())
        names := make([]string, len(models))
        for i, m := range models {
            names[i] = m.Name
        }
        return names
    }
    return nil
}

completer.SessionsFn = func() []commands.SessionInfo {
    if m.convStore != nil {
        metas, _ := m.convStore.List()
        // Convert to SessionInfo
        ...
    }
    return nil
}

completer.ToolsFn = func() []string {
    return m.toolRegistry.List()
}
```

### 5.3 UI/UX Design

#### Command Palette Appearance

```
+------------------------------------------+
| /save                                    |
+------------------------------------------+
| > /save [name]     Save conversation     | <- Selected
|   /sessions        List saved sessions   |
|   /status          Show status info      |
+------------------------------------------+
| Tab: select  Enter: accept  Esc: cancel  |
+------------------------------------------+
```

#### Key Bindings

| Key | Action |
|-----|--------|
| `Tab` | Open completion / Accept top match |
| `Up/Down` | Navigate completions |
| `Enter` | Accept selected completion |
| `Esc` | Close palette |
| `Ctrl+Space` | Force open palette |

#### Visual Styling

- Palette background: subtle dark gray (#1e1e1e)
- Selected item: highlighted with accent color
- Command: bold white
- Description: dimmed gray
- Matching characters: highlighted

---

## 6. Implementation Priority Order

### Phase 1: Core Functionality (Sprint 1)

1. **Wire existing registry commands to chat model**
   - `/copy`, `/export`, `/models`, `/tools`, `/config`

2. **Add new high-priority commands**
   - `/retry`, `/tokens`, `/stats`, `/audit`, `/classify`

3. **Basic tab completion**
   - Single-completion auto-accept
   - Multiple-completion palette display

### Phase 2: Enhanced UX (Sprint 2)

4. **Command palette UI**
   - Visual dropdown component
   - Keyboard navigation
   - Real-time filtering

5. **@ mention completion**
   - File path browser
   - Mention type suggestions

6. **Additional commands**
   - `/undo`, `/search`, `/doctor`, `/consent`

### Phase 3: Advanced Features (Sprint 3)

7. **Dynamic completions**
   - Live model list from Ollama
   - Live session list
   - Config key completion

8. **Utility commands**
   - `/cost`, `/time`, `/secure`, `/offline`

9. **Power user features**
   - `/template`, `/persona`, `/macro`

---

## 7. Testing Plan

### 7.1 Unit Tests

- Command parsing for all new commands
- Completion scoring and ranking
- Argument type validation

### 7.2 Integration Tests

- Tab completion behavior
- Palette navigation
- Command execution

### 7.3 E2E Tests

- Full command workflows
- Session save/load with autocomplete
- IL5 compliance command verification

---

## 8. Files to Modify

| File | Changes |
|------|---------|
| `internal/ui/chat/model.go` | Add completion state, wire registry commands |
| `internal/ui/chat/view.go` | Render command palette |
| `internal/commands/registry.go` | Add new commands to registry |
| `internal/commands/handlers.go` | Implement new command handlers |
| `internal/commands/completion.go` | Add dynamic completion callbacks |
| `internal/ui/components/command_palette.go` | New: palette component |
| `main.go` | Add message handlers for new commands |

---

## 9. Notes for Service Members

### Quick Reference Card

```
CONVERSATION          MODEL/ROUTING         SECURITY (IL5)
------------          -------------         --------------
/new    Start fresh   /model <name>         /classify    Set level
/save   Save session  /mode local/cloud     /audit       View logs
/load   Load session  /models List all      /consent     View status
/clear  Clear history                       /secure      Lock down
/export Export file   TOOLS                 /offline     Air-gap mode
/copy   Copy response /tools List tools
/retry  Retry last    /tool <name> on/off   STATS
                                            -----
SEARCH                CONFIG                /tokens      Token count
------                ------                /cost        API cost
/find   Search text   /config Show all      /time        Session time
/goto   Jump to msg   /config <key> <val>   /stats       All stats

SHORTCUTS: Ctrl+C=Cancel  Ctrl+F=Search  Ctrl+R=Mode  Tab=Complete
```

---

*Document Version: 1.0*
*Created: 2025-01-20*
*Author: rigrun TUI Development Team*
