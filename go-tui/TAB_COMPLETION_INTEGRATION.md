# Tab Completion Integration Guide

This document describes the enhanced tab completion system for rigrun Go TUI.

## Files Created

1. **`internal/ui/components/completion.go`** - Completion popup UI component
2. **`internal/ui/chat/completion.go`** - Tab completion handlers
3. **`internal/ui/chat/view_completion.go`** - Completion rendering

## Files Modified

1. **`internal/ui/chat/model.go`**
   - Added completion state fields
   - Initialized completer and completion state in `New()`
   - Added Tab key handling in `handleKey()`

2. **`internal/commands/completion.go`** (already existed)
   - Contains the completion engine and logic

## Integration Steps

### 1. Update view.go to render completions

In `internal/ui/chat/view.go`, modify the `renderChat()` function to use the completion-aware input:

```go
// Around line 51, replace:
input := m.renderInput()

// With:
input := m.renderInputWithCompletion()
```

This will show the completion popup when Tab is pressed.

### 2. Initialize completer callbacks

In your main initialization code (likely `main.go` or where the chat model is created), call:

```go
chatModel.SetupCompleterCallbacks()
```

This sets up dynamic completion for models, sessions, tools, etc.

### 3. Set up model/session completers

The completer needs access to actual data sources. Update the callbacks in `completion.go`:

**For Models:**
```go
m.completer.ModelsFn = func() []string {
    if m.ollama == nil {
        return nil
    }
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    models, err := m.ollama.ListModels(ctx)
    if err != nil {
        return nil
    }

    names := make([]string, len(models))
    for i, model := range models {
        names[i] = model.Name
    }
    return names
}
```

**For Sessions:**
```go
m.completer.SessionsFn = func() []commands.SessionInfo {
    if m.storage == nil {
        return nil
    }

    sessions, err := m.storage.List()
    if err != nil {
        return nil
    }

    infos := make([]commands.SessionInfo, len(sessions))
    for i, s := range sessions {
        infos[i] = commands.SessionInfo{
            ID:      s.ID,
            Title:   s.Title,
            Preview: s.GetPreview(),
        }
    }
    return infos
}
```

## Usage

### Basic Tab Completion

**Commands:**
```
You: /mo[Tab]
     ┌────────────────────────────┐
     │ /model     Switch model    │ ◀ selected
     │ /models    List models     │
     │ /mode      Switch routing  │
     └────────────────────────────┘
```

**With Arguments:**
```
You: /model [Tab]
     ┌────────────────────────────┐
     │ llama3.2   Local model     │ ◀
     │ mistral    Local model     │
     │ haiku      Cloud (fast)    │
     │ sonnet     Cloud (balanced)│
     └────────────────────────────┘
```

**File Paths:**
```
You: @file:src/[Tab]
     ┌────────────────────────────┐
     │ main.go    4.2 KB          │ ◀
     │ config/    directory       │
     │ utils.go   2.1 KB          │
     └────────────────────────────┘
```

**@Mentions:**
```
You: @[Tab]
     ┌────────────────────────────┐
     │ @file:      Include file   │ ◀
     │ @clipboard  Clipboard      │
     │ @git        Git info       │
     │ @codebase   Directory tree │
     │ @error      Last error     │
     └────────────────────────────┘
```

### Cycling Through Completions

- **First Tab**: Shows completions (or completes if only one match)
- **Second Tab**: Cycles to next completion
- **Continue Tabbing**: Cycles through all options
- **Esc**: Hides completion popup
- **Any other key**: Clears completions and types normally

## Completion Types Supported

1. **Slash Commands** (`/help`, `/model`, `/quit`, etc.)
2. **Command Arguments** (models, sessions, modes, tools, config keys)
3. **File Paths** (`@file:...`)
4. **@Mentions** (`@file:`, `@clipboard`, `@git`, `@codebase`, `@error`)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    User presses Tab                         │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────┐
│           handleTabCompletion() (completion.go)             │
│  - Gets current input and cursor position                   │
│  - Calls completer.Complete()                               │
│  - Updates completion state                                 │
│  - First Tab: show popup, Second Tab: cycle                 │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────┐
│          Completer.Complete() (commands/completion.go)      │
│  - Parses input to determine completion context             │
│  - Calls appropriate completion function:                   │
│    * completeCommands() for slash commands                  │
│    * completeMentions() for @ mentions                      │
│    * completeArg() for command arguments                    │
│  - Returns sorted, scored completions                       │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────┐
│         CompletionState updated in model.go                 │
│  - Stores completions array                                 │
│  - Tracks selected index                                    │
│  - Sets visible flag                                        │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────┐
│         View renders popup (view_completion.go)             │
│  - renderInputWithCompletion() called                       │
│  - Creates CompletionPopup component                        │
│  - Renders above input with selected item highlighted       │
└─────────────────────────────────────────────────────────────┘
```

## Example Flow

1. User types: `/mo`
2. User presses Tab
3. `handleTabCompletion()` is called
4. `completer.Complete("/mo", 3)` returns: `/model`, `/models`, `/mode`
5. Completion state updated with these 3 options
6. View renders popup with `/model` selected (highest score)
7. User presses Tab again
8. `completionState.Next()` selects `/models`
9. User presses Enter or keeps typing
10. Completion is applied: `/models`

## Performance Considerations

- **File completion** is limited to 20 results to prevent slowdown
- **Model/session lookups** use short timeouts (2-5 seconds)
- **Completions are scored** for relevance (prefix match, length, etc.)
- **No network calls** for basic completions
- **Async operations** for dynamic data (models, sessions)

## Testing

Test the completion system with:

```go
// In your test file
func TestTabCompletion(t *testing.T) {
    completer := commands.NewCompleter(commands.NewRegistry())

    // Test command completion
    completions := completer.Complete("/mo", 3)
    if len(completions) < 1 {
        t.Error("Expected completions for /mo")
    }

    // Test mention completion
    completions = completer.Complete("@f", 2)
    if len(completions) == 0 {
        t.Error("Expected @file completion")
    }
}
```

## Known Limitations

1. File path completion on Windows uses backslashes
2. Session completion requires storage integration
3. Model completion is currently static (needs async refresh)
4. No fuzzy matching (only prefix matching)
5. Popup height is fixed (doesn't scroll for >8 items, wraps instead)

## Future Enhancements

1. **Fuzzy matching** - Match substring anywhere in completion
2. **Usage history** - Boost frequently used completions
3. **Smart suggestions** - Context-aware completions
4. **Preview pane** - Show file contents or command help
5. **Async loading** - Show "Loading..." for slow completions
6. **Completion descriptions** - Fetch from command metadata

## Troubleshooting

**Completions not showing:**
- Check that `m.completer` is initialized
- Verify `SetupCompleterCallbacks()` was called
- Check console for errors in completion engine

**Wrong completions:**
- Verify cursor position is correct
- Check completion parsing logic in `Complete()`
- Debug with breakpoints in `handleTabCompletion()`

**Popup not rendering:**
- Check `m.showCompletions` is true
- Verify `renderInputWithCompletion()` is being called
- Check viewport height calculations

**Performance issues:**
- Limit file completion results
- Add caching for model/session lists
- Use goroutines for slow operations
