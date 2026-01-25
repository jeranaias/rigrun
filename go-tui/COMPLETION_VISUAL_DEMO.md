# Tab Completion Visual Demo

## Command Completion

### Typing `/mo` then Tab

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ rigrun v0.1.0 | Model: qwen2.5-coder:14b | Mode: hybrid | GPU: NVIDIA RTX   │
└──────────────────────────────────────────────────────────────────────────────┘

  System: Welcome to rigrun! Type a message or use / for commands.

────────────────────────────────────────────────────────────────────────────────
> /mo█
  ┌────────────────────────────┐
  │ ▶ /model     Switch model  │
  │   /models    List models   │
  │   /mode      Switch routing│
  └────────────────────────────┘
  3 / 4,096 chars

────────────────────────────────────────────────────────────────────────────────
Ready | Context: 0%
```

### After Second Tab (Cycling)

```
────────────────────────────────────────────────────────────────────────────────
> /mo█
  ┌────────────────────────────┐
  │   /model     Switch model  │
  │ ▶ /models    List models   │  ← Now selected
  │   /mode      Switch routing│
  └────────────────────────────┘
  3 / 4,096 chars
```

## Model Argument Completion

### Typing `/model ` then Tab

```
────────────────────────────────────────────────────────────────────────────────
> /model █
  ┌──────────────────────────────────┐
  │ ▶ qwen2.5-coder:14b   14B params │
  │   qwen2.5-coder:7b    7B params  │
  │   codestral:22b       22B params │
  │   llama3.1:8b         8B params  │
  │   deepseek-coder:6.7b 6.7B       │
  │   mistral:7b          7B params  │
  │   haiku               Cloud fast │
  │   sonnet              Cloud bal. │
  └──────────────────────────────────┘
  7 / 4,096 chars
```

## @Mention Completion

### Typing `@` then Tab

```
────────────────────────────────────────────────────────────────────────────────
> @█
  ┌───────────────────────────────────────┐
  │ ▶ @file:      Include file content    │
  │   @clipboard  Include clipboard       │
  │   @git        Include git info        │
  │   @codebase   Include directory tree  │
  │   @error      Include last error      │
  └───────────────────────────────────────┘
  1 / 4,096 chars
```

### After Selecting `@file:` and Typing Path

```
────────────────────────────────────────────────────────────────────────────────
> @file:src/█
  ┌────────────────────────────────┐
  │ ▶ main.go      4.2 KB          │
  │   config/      directory       │
  │   utils.go     2.1 KB          │
  │   models/      directory       │
  │   handlers.go  6.8 KB          │
  └────────────────────────────────┘
  12 / 4,096 chars
```

## Session Completion

### Typing `/load ` then Tab

```
────────────────────────────────────────────────────────────────────────────────
> /load █
  ┌──────────────────────────────────────────────────────┐
  │ ▶ abc123def4  Debug API error | 12 msgs | 2h ago    │
  │   fed456cba9  Refactor auth  | 8 msgs  | 1d ago    │
  │   789ghi012j  New feature   | 24 msgs | 3d ago    │
  └──────────────────────────────────────────────────────┘
  6 / 4,096 chars
```

## Tool Completion

### Typing `/tool ` then Tab

```
────────────────────────────────────────────────────────────────────────────────
> /tool █
  ┌────────────────────────────────────┐
  │ ▶ Read       Read files           │
  │   Write      Write files          │
  │   Edit       Edit files           │
  │   Glob       Find files           │
  │   Grep       Search in files      │
  │   Bash       Execute commands     │
  └────────────────────────────────────┘
  6 / 4,096 chars
```

## Config Completion

### Typing `/config ` then Tab

```
────────────────────────────────────────────────────────────────────────────────
> /config █
  ┌──────────────────────────────────────┐
  │ ▶ model               Current model  │
  │   mode                Routing mode   │
  │   temperature         LLM temp      │
  │   max_tokens          Token limit   │
  │   timeout             Request timeout│
  │   autosave            Auto-save     │
  │   theme               Color theme   │
  │   routing.default_mode Default route│
  └──────────────────────────────────────┘
  8 / 4,096 chars
```

## Enum Value Completion

### Typing `/mode ` then Tab

```
────────────────────────────────────────────────────────────────────────────────
> /mode █
  ┌───────────────────────────────────────┐
  │ ▶ local   Use local Ollama only      │
  │   cloud   Use cloud APIs only        │
  │   hybrid  Auto-route (recommended)   │
  └───────────────────────────────────────┘
  6 / 4,096 chars
```

## Inline Compact View (Alternative Design)

### When Space is Limited

```
────────────────────────────────────────────────────────────────────────────────
> /mo█  Tab: /model • /models • /mode ...3 completions
  3 / 4,096 chars
```

## With Usage Preview (Future Enhancement)

```
────────────────────────────────────────────────────────────────────────────────
> /model █
  ┌──────────────────────────────────┐
  │ ▶ qwen2.5-coder:14b   14B params │
  │   qwen2.5-coder:7b    7B params  │
  │   codestral:22b       22B params │
  │   llama3.1:8b         8B params  │
  └──────────────────────────────────┘
  ─────────────────────────────────────
  Usage: /model <name>

  Switch the active LLM model. Use /models
  to see full list with capabilities.

  Examples:
    /model qwen2.5-coder:14b
    /model haiku

  7 / 4,096 chars
```

## Multi-Column Layout (Future Enhancement)

```
────────────────────────────────────────────────────────────────────────────────
> /█
  ┌─────────────────────┬─────────────────────┬──────────────────────┐
  │  NAVIGATION         │  CONVERSATION       │  MODEL               │
  │ ▶ /help             │  /new               │  /model              │
  │   /quit             │  /save              │  /models             │
  │                     │  /load              │  /mode               │
  │                     │  /clear             │                      │
  │  TOOLS              │  /copy              │  SETTINGS            │
  │  /tools             │  /export            │  /config             │
  │  /tool              │  /sessions          │  /status             │
  └─────────────────────┴─────────────────────┴──────────────────────┘
  1 / 4,096 chars
```

## Error State (File Not Found)

```
────────────────────────────────────────────────────────────────────────────────
> @file:nonexistent/█
  ┌────────────────────────────────┐
  │  No matches found              │
  └────────────────────────────────┘
  21 / 4,096 chars
```

## Loading State (Async Completion)

```
────────────────────────────────────────────────────────────────────────────────
> /model █
  ┌────────────────────────────────┐
  │  Loading models...  ⣾          │
  └────────────────────────────────┘
  7 / 4,096 chars
```

## Visual Indicators

### Selection Indicator
```
▶  Current selection (cyan background)
   Not selected (normal)
```

### Item Types
```
/command       Slash command
model-name     Model
abc123         Session ID
file.go        File (with size)
directory/     Directory
@mention       Mention type
value          Enum value
```

### Colors (Catppuccin Mocha Theme)
```
▶ Selected:     Cyan background (#89dceb)
  Normal text:  White (#cdd6f4)
  Descriptions: Muted gray (#6c7086)
  Border:       Cyan (#89dceb)
  Background:   Surface (#1e1e2e)
```

## Keyboard Shortcuts

```
Tab         Show completions / Cycle next
Shift+Tab   Cycle previous (future)
↓           Next completion (future)
↑           Previous completion (future)
Enter       Accept completion
Esc         Hide completions
Any letter  Clear completions and type
```

## Completion Popup Positioning

### Above Input (Default)
```
  [Viewport with messages]

  ┌────────────────────┐
  │ ▶ completion 1     │  ← Popup appears here
  │   completion 2     │
  └────────────────────┘
────────────────────────
> input█
  N / 4,096 chars
────────────────────────
  Status Bar
```

### Inline Hint (No Popup)
```
  [Viewport with messages]

────────────────────────
> input█  Tab for 5 completions
  N / 4,096 chars
────────────────────────
  Status Bar
```

## Real-World Example: Debugging Workflow

### 1. Include error context
```
> @error █
  ┌────────────────────────────────┐
  │ ▶ @error  Include last error   │
  └────────────────────────────────┘
  6 / 4,096 chars
```

### 2. Include relevant file
```
> @error @file:█
  ┌────────────────────────────────┐
  │ ▶ src/           directory     │
  │   main.go        4.2 KB        │
  │   config.yaml    1.1 KB        │
  └────────────────────────────────┘
  13 / 4,096 chars
```

### 3. Complete message
```
> @error @file:src/main.go why is this failing?
  146 / 4,096 chars
```

## Full Terminal Example

```
╔══════════════════════════════════════════════════════════════════════════════╗
║ rigrun v0.1.0 | Model: qwen2.5-coder:14b | Mode: hybrid | GPU: NVIDIA RTX   ║
╚══════════════════════════════════════════════════════════════════════════════╝

  System: Welcome! Start chatting or use / for commands.

  You: Can you help me debug this?

  Assistant: Of course! I'd be happy to help. Please share:
  1. The error message or unexpected behavior
  2. Relevant code (use @file:path/to/file)
  3. What you've tried so far

  ┌────────────────────────────────┐
  │ ▶ @error   Include last error  │
  │   @file:   Include file        │
  │   @git     Include git info    │
  └────────────────────────────────┘
────────────────────────────────────────────────────────────────────────────────
> @█
  1 / 4,096 chars

────────────────────────────────────────────────────────────────────────────────
Ready | 2 messages | Context: 12% | Cost: $0.003
```

---

## Notes on Implementation

- Popup uses `lipgloss.JoinVertical` to stack above input
- Completions are limited to 8 visible items with scrolling
- Selection uses cyan background for visual clarity
- Descriptions provide context without cluttering
- Fast prefix matching keeps UI responsive
- Proper width calculation prevents overflow
- Unicode-aware for international file names
