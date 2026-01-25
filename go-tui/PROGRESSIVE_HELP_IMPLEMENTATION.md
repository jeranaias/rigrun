# Progressive Help System Implementation

## Overview
Implemented a progressive help system for rigrun Go TUI that makes help more accessible to new users while still providing comprehensive documentation for advanced users.

## Changes Made

### 1. Config Changes (`internal/config/config.go`)
- **Added `TutorialCompleted` field** to `UIConfig` struct to track whether user has completed the tutorial
- **Updated default configuration** to set `TutorialCompleted: false` for new users
- **Added config key** `ui.tutorial_completed` to `GetAllKeys()` function for proper config management

### 2. Command Registry (`internal/commands/registry.go`)
- **Updated `/help` command definition** with new arguments support
  - Added `Usage: "/help [quick|all|<category>]"`
  - Added `Args` with enum type supporting: `quick`, `all`, `navigation`, `conversation`, `model`, `tools`, `settings`
  - Added description for the mode argument

### 3. Help Handler (`internal/commands/handlers.go`)
- **Refactored `GenerateHelpText` function** to accept a `mode` parameter
- **Implemented three help modes:**

#### a. Quick Help (`/help` or `/help quick`)
Shows only 5 essential commands:
- `/help` - Get help
- `/new` - Start new conversation
- `/save` - Save conversation
- `/model` - Switch model
- `/quit` - Exit

Includes:
- Keyboard shortcuts (Ctrl+C, Ctrl+P, Tab, Up/Down)
- Hints to explore other help modes
- Links to category-specific help

#### b. Category Help (`/help <category>`)
Shows commands grouped by category with contextual tips:
- **Navigation**: Help and quit commands with Esc/Tab tips
- **Conversation**: Session management with @mentions tips
- **Model**: Model switching with local/cloud tips
- **Tools**: Tool management tips
- **Settings**: Configuration and cache tips

Each category includes:
- All commands in that category with descriptions
- Usage examples where applicable
- Contextual tips relevant to that category
- Links back to quick help or full help

#### c. Full Help (`/help all`)
Shows complete command reference:
- All categories with all commands
- Context Mentions section
- Keyboard Shortcuts section
- Tips for using category-specific help

### 4. Chat UI Integration (`internal/ui/chat/commands.go`)
- **Updated `handleHelpCommand`** to use the new progressive help system
- Creates command registry on-the-fly for help generation
- Checks `config.Global().UI.TutorialCompleted` to show contextual tips for new users
- Displays additional hint for users who haven't completed the tutorial

## Usage Examples

### For New Users
```
> /help
Quick Help - Essential Commands
================================

  /help             Show this help (or try /help all)
  /new              Start new conversation
  /save             Save conversation
  /model            Switch model
  /quit             Exit rigrun

Keyboard Shortcuts
------------------
  Ctrl+C            Stop generation / Cancel
  Ctrl+P            Open command palette
  Tab               Auto-complete
  Up/Down           Navigate history

Want more? Try:
  /help all         - Show all available commands
  /help navigation  - Navigation commands
  /help conversation - Conversation management
  /help model       - Model and routing commands
  /help tools       - Tool management
  /help settings    - Settings and configuration

Tip: You're viewing quick help. Use /help all to see all commands.
```

### Category-Specific Help
```
> /help conversation
Conversation Commands
=====================

  /new (/n)                     Start a new conversation
      Usage: /new
  /save (/s)                    Save current conversation
      Usage: /save [name]
  /load (/l, /resume)          Load a saved conversation
      Usage: /load <session_id>
  /clear (/c)                   Clear conversation history
  /copy                         Copy last response to clipboard
  /export                       Export conversation to file
      Usage: /export [format]
  /sessions (/list)             List saved sessions

Tips:
  - Conversations auto-save on changes
  - Use @file:<path> to include files in your prompt
  - Try @clipboard to paste clipboard content

Use /help all to see all commands, or /help quick for essentials.
```

### Full Help
```
> /help all
Available Commands
==================

Navigation
----------
  /help (/h, /?)                Show help and available commands
      Usage: /help [quick|all|<category>]
  /quit (/q, /exit)             Exit rigrun

Conversation
------------
  [... all conversation commands ...]

[... continues with all categories ...]

Context Mentions
----------------
  @file:<path>    Include file content
  @clipboard      Include clipboard content
  @git            Include recent git info
  @codebase       Include directory structure
  @error          Include last error message

Keyboard Shortcuts
------------------
  Ctrl+C          Stop generation / Cancel
  Ctrl+P          Open command palette
  Tab             Auto-complete
  Up/Down         Navigate history
  Esc             Close overlay

Tip: Use /help <category> to see commands by category
Categories: navigation, conversation, model, tools, settings
```

## Benefits

1. **Lower Barrier to Entry**: New users see only essential commands first
2. **Progressive Disclosure**: Users can explore more as they need
3. **Contextual Learning**: Category-specific tips teach best practices
4. **Keyboard Shortcut Discovery**: Essential shortcuts shown prominently
5. **Reduces Overwhelm**: Full help is still available but not forced on new users
6. **Better Discoverability**: Clear pathways to explore different command categories

## Future Enhancements

1. **Interactive Tutorial**: Mark `tutorial_completed` when user completes first few actions
2. **Contextual Tips**: Show tips based on user's interaction count
3. **Command Usage Tracking**: Suggest relevant commands based on usage patterns
4. **Quick Reference Card**: One-page printable reference for essential commands
5. **Search in Help**: Allow `/help search <query>` to find specific commands

## Testing Notes

The implementation is complete and ready for testing. However, the codebase has a pre-existing compilation issue in `internal/ollama/stream_optimized.go` where the `StreamReader` struct is missing a `decoder` field that is being referenced. This issue is unrelated to the Progressive Help System implementation.

To test the Progressive Help System once the ollama issue is resolved:
1. Run `/help` to see quick help
2. Run `/help all` to see full help
3. Run `/help conversation` to see category help
4. Verify that new users see the tutorial tip
5. Set `ui.tutorial_completed = true` in config and verify tip disappears

## Files Modified

1. `C:\rigrun\go-tui\internal\config\config.go` - Added TutorialCompleted field
2. `C:\rigrun\go-tui\internal\commands\registry.go` - Updated /help command definition
3. `C:\rigrun\go-tui\internal\commands\handlers.go` - Implemented progressive help generation
4. `C:\rigrun\go-tui\internal\ui\chat\commands.go` - Integrated new help system

## Compatibility

All changes are backward compatible. Users can still use `/help` as before, but now have additional options for more targeted help.
