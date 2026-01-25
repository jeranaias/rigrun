# rigrun TUI Manual Testing Plan

This document provides an exhaustive manual testing plan for the rigrun Go TUI application. Follow each section step-by-step to verify all features work correctly.

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [CLI Commands](#cli-commands)
3. [TUI Slash Commands](#tui-slash-commands)
4. [Keyboard Shortcuts](#keyboard-shortcuts)
5. [Context Mentions (@)](#context-mentions)
6. [OpenRouter Cloud Connection](#openrouter-cloud-connection)
7. [Ollama Local Connection](#ollama-local-connection)
8. [Session Management](#session-management)
9. [Search Functionality](#search-functionality)
10. [Cache System](#cache-system)
11. [Tool System](#tool-system)
12. [Error Handling](#error-handling)
13. [Edge Cases](#edge-cases)

---

## Prerequisites

Before testing, ensure the following:

- [ ] Go is installed and in PATH
- [ ] Build the application: `go build -o rigrun.exe .`
- [ ] Ollama is installed: `ollama --version`
- [ ] Ollama is running: `ollama serve`
- [ ] A model is downloaded: `ollama pull qwen2.5-coder:14b` (or another model)
- [ ] OpenRouter API key available: `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`

---

## CLI Commands

### 1. Default TUI Launch

- [ ] **Test**: Run `rigrun` with no arguments
- [ ] **Expected**: TUI launches with welcome screen showing version, model, GPU info
- [ ] **Verify**: Press any key to enter chat view

### 2. TUI Command

- [ ] **Test**: Run `rigrun tui`
- [ ] **Expected**: Same as running `rigrun` with no arguments

### 3. Help Command

- [ ] **Test**: Run `rigrun help`
- [ ] **Expected**: Displays usage text with all commands and examples
- [ ] **Test**: Run `rigrun --help`
- [ ] **Expected**: Same help output
- [ ] **Test**: Run `rigrun -h`
- [ ] **Expected**: Same help output

### 4. Version Command

- [ ] **Test**: Run `rigrun version`
- [ ] **Expected**: Shows version number, git commit, and build date
- [ ] **Test**: Run `rigrun --version`
- [ ] **Expected**: Same version output
- [ ] **Test**: Run `rigrun -v`
- [ ] **Expected**: Same version output (Note: -v is also verbose flag context-dependent)

### 5. Ask Command (Single Query)

- [ ] **Test**: Run `rigrun ask "What is 2+2?"`
- [ ] **Expected**: Streams response to stdout with routing info and cost summary
- [ ] **Test**: Run `rigrun ask "Review this code:" --file main.go`
- [ ] **Expected**: Includes file content in the prompt, shows "[+] Including file: main.go"
- [ ] **Test**: Run `rigrun ask "Review:" -f main.go`
- [ ] **Expected**: Same as above (short flag)
- [ ] **Test**: Run `rigrun ask "Question" --model qwen2.5:7b`
- [ ] **Expected**: Uses specified model for the query
- [ ] **Test**: Run `rigrun ask "Question" -m qwen2.5:7b`
- [ ] **Expected**: Same as above (short flag)
- [ ] **Test**: Run `rigrun ask "Question" -q`
- [ ] **Expected**: Minimal output (no routing info, no cost summary)
- [ ] **Test**: Run `rigrun ask` (no question)
- [ ] **Expected**: Error message about no question provided

### 6. Chat Command (Interactive)

- [ ] **Test**: Run `rigrun chat`
- [ ] **Expected**: Interactive chat starts with welcome message
- [ ] **Verify**: Prompt shows `rigrun>`
- [ ] **Test**: Run `rigrun chat --model qwen2.5:7b`
- [ ] **Expected**: Chat uses specified model (shown in welcome)
- [ ] **Test**: Run `rigrun chat -m qwen2.5:7b`
- [ ] **Expected**: Same as above (short flag)
- [ ] **Test**: In chat, type a message and press Enter
- [ ] **Expected**: Response streams with routing info and stats
- [ ] **Test**: Type `exit` or `quit`
- [ ] **Expected**: Shows session summary and exits

### 7. Status Command

- [ ] **Test**: Run `rigrun status`
- [ ] **Expected**: Shows system status including GPU, Ollama, Model, Routing, Session, Cache
- [ ] **Test**: Run `rigrun s`
- [ ] **Expected**: Same as `rigrun status` (short alias)
- [ ] **Verify**: GPU section shows detected GPU or "CPU mode"
- [ ] **Verify**: Ollama section shows "Running" with version
- [ ] **Verify**: Model section shows model name and status

### 8. Config Command

- [ ] **Test**: Run `rigrun config`
- [ ] **Expected**: Shows all configuration sections
- [ ] **Test**: Run `rigrun config show`
- [ ] **Expected**: Same as above
- [ ] **Test**: Run `rigrun config path`
- [ ] **Expected**: Shows config file path
- [ ] **Test**: Run `rigrun config set default_model qwen2.5:7b`
- [ ] **Expected**: "[OK] default_model = qwen2.5:7b"
- [ ] **Test**: Run `rigrun config set openrouter_key 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`
- [ ] **Expected**: "[OK] openrouter_key = sk-or-v1****" (masked)
- [ ] **Test**: Run `rigrun config set default_mode hybrid`
- [ ] **Expected**: "[OK] default_mode = hybrid"
- [ ] **Test**: Run `rigrun config set default_mode invalid`
- [ ] **Expected**: Error about invalid mode
- [ ] **Test**: Run `rigrun config set unknown_key value`
- [ ] **Expected**: Error about unknown config key
- [ ] **Test**: Run `rigrun config reset`
- [ ] **Expected**: "[OK] Configuration reset to defaults"

### 9. Setup Command

- [ ] **Test**: Run `rigrun setup`
- [ ] **Expected**: Full interactive wizard starts with hardware detection
- [ ] **Test**: Run `rigrun setup --quick`
- [ ] **Expected**: Quick setup with auto-detection and defaults
- [ ] **Test**: Run `rigrun setup quick`
- [ ] **Expected**: Same as above
- [ ] **Test**: Run `rigrun setup gpu`
- [ ] **Expected**: GPU detection and setup guidance only
- [ ] **Test**: Run `rigrun setup model`
- [ ] **Expected**: Interactive model selection
- [ ] **Test**: Run `rigrun setup wizard`
- [ ] **Expected**: Same as `rigrun setup` (full wizard)

### 10. Cache Command

- [ ] **Test**: Run `rigrun cache`
- [ ] **Expected**: Shows cache statistics
- [ ] **Test**: Run `rigrun cache stats`
- [ ] **Expected**: Same as above
- [ ] **Test**: Run `rigrun cache clear`
- [ ] **Expected**: Prompts for confirmation, clears cache on "y"
- [ ] **Test**: Run `rigrun cache clear --exact`
- [ ] **Expected**: Clears only exact-match cache
- [ ] **Test**: Run `rigrun cache clear --semantic`
- [ ] **Expected**: Clears only semantic cache
- [ ] **Test**: Run `rigrun cache export .`
- [ ] **Expected**: Exports cache files to current directory

### 11. Doctor Command

- [ ] **Test**: Run `rigrun doctor`
- [ ] **Expected**: Runs all health checks with pass/warn/fail indicators
- [ ] **Verify**: Checks include: Ollama Installed, Ollama Running, Model Available, GPU Detected, Config Valid, Cache Writable, OpenRouter Configured
- [ ] **Test**: Run `rigrun doctor --fix`
- [ ] **Expected**: Attempts auto-fix for failed checks

### 12. Global Flags

- [ ] **Test**: Run `rigrun --paranoid`
- [ ] **Expected**: TUI launches in local-only mode (no cloud)
- [ ] **Test**: Run `rigrun --model qwen2.5:7b`
- [ ] **Expected**: TUI uses specified model
- [ ] **Test**: Run `rigrun --skip-banner`
- [ ] **Expected**: Skips DoD consent banner (if enabled)
- [ ] **Test**: Run `rigrun -q ask "test"`
- [ ] **Expected**: Quiet mode - minimal output
- [ ] **Test**: Run `rigrun --quiet ask "test"`
- [ ] **Expected**: Same as above

---

## TUI Slash Commands

Launch the TUI with `rigrun` and test each command:

### Navigation Commands

- [ ] **Test**: Type `/help` and press Enter
- [ ] **Expected**: Shows help with all commands, context mentions, and shortcuts
- [ ] **Test**: Type `/h` and press Enter
- [ ] **Expected**: Same as `/help` (alias)
- [ ] **Test**: Type `/?` and press Enter
- [ ] **Expected**: Same as `/help` (alias)
- [ ] **Test**: Type `/quit` and press Enter
- [ ] **Expected**: Exits the TUI
- [ ] **Test**: Type `/q` and press Enter
- [ ] **Expected**: Same as `/quit` (alias)
- [ ] **Test**: Type `/exit` and press Enter
- [ ] **Expected**: Same as `/quit` (alias)

### Conversation Commands

- [ ] **Test**: Type `/new` and press Enter
- [ ] **Expected**: Starts a new conversation (clears history)
- [ ] **Test**: Type `/n` and press Enter
- [ ] **Expected**: Same as `/new` (alias)
- [ ] **Test**: Type `/clear` and press Enter
- [ ] **Expected**: Clears conversation history
- [ ] **Test**: Type `/c` and press Enter
- [ ] **Expected**: Same as `/clear` (alias)
- [ ] **Test**: Have a conversation, then type `/save` and press Enter
- [ ] **Expected**: Saves conversation with auto-generated name
- [ ] **Test**: Type `/save my-session-name` and press Enter
- [ ] **Expected**: Saves conversation with specified name
- [ ] **Test**: Type `/s my-session` and press Enter
- [ ] **Expected**: Same as `/save` (alias)
- [ ] **Test**: Type `/sessions` and press Enter
- [ ] **Expected**: Lists all saved sessions with IDs, models, message counts
- [ ] **Test**: Type `/list` and press Enter
- [ ] **Expected**: Same as `/sessions` (alias)
- [ ] **Test**: Type `/load 1` and press Enter
- [ ] **Expected**: Loads the first saved session
- [ ] **Test**: Type `/l <session-id>` and press Enter
- [ ] **Expected**: Same as `/load` (alias)
- [ ] **Test**: Type `/resume <session-id>` and press Enter
- [ ] **Expected**: Same as `/load` (alias)
- [ ] **Test**: Type `/copy` and press Enter
- [ ] **Expected**: Copies last response to clipboard
- [ ] **Test**: Type `/export` and press Enter
- [ ] **Expected**: Exports conversation to markdown file
- [ ] **Test**: Type `/export json` and press Enter
- [ ] **Expected**: Exports conversation to JSON file
- [ ] **Test**: Type `/export txt` and press Enter
- [ ] **Expected**: Exports conversation to plain text file

### Model Commands

- [ ] **Test**: Type `/model` and press Enter
- [ ] **Expected**: Shows current model name
- [ ] **Test**: Type `/m` and press Enter
- [ ] **Expected**: Same as `/model` (alias)
- [ ] **Test**: Type `/model qwen2.5:7b` and press Enter
- [ ] **Expected**: Switches to specified model
- [ ] **Test**: Type `/models` and press Enter
- [ ] **Expected**: Lists all available Ollama models
- [ ] **Test**: Type `/mode` and press Enter
- [ ] **Expected**: Shows current routing mode
- [ ] **Test**: Type `/mode local` and press Enter
- [ ] **Expected**: Switches to local-only mode
- [ ] **Test**: Type `/mode cloud` and press Enter
- [ ] **Expected**: Switches to cloud mode
- [ ] **Test**: Type `/mode hybrid` and press Enter
- [ ] **Expected**: Switches to hybrid mode

### Tool Commands

- [ ] **Test**: Type `/tools` and press Enter
- [ ] **Expected**: Lists available tools (Read, Write, Edit, Glob, Grep, Bash, etc.)
- [ ] **Test**: Type `/tool read off` and press Enter
- [ ] **Expected**: Disables the read tool
- [ ] **Test**: Type `/tool read on` and press Enter
- [ ] **Expected**: Enables the read tool

### Settings Commands

- [ ] **Test**: Type `/config` and press Enter
- [ ] **Expected**: Shows current configuration
- [ ] **Test**: Type `/config cache` and press Enter
- [ ] **Expected**: Shows cache status
- [ ] **Test**: Type `/config cache clear` and press Enter
- [ ] **Expected**: Clears the cache
- [ ] **Test**: Type `/config cache on` and press Enter
- [ ] **Expected**: Enables caching
- [ ] **Test**: Type `/config cache off` and press Enter
- [ ] **Expected**: Disables caching
- [ ] **Test**: Type `/status` and press Enter
- [ ] **Expected**: Shows detailed status (model, mode, session, cache)

---

## Keyboard Shortcuts

Test these shortcuts while in the TUI:

### General Navigation

- [ ] **Test**: Press `Ctrl+C` while streaming
- [ ] **Expected**: Cancels the current streaming response
- [ ] **Test**: Press `Ctrl+C` when idle
- [ ] **Expected**: Exits the TUI
- [ ] **Test**: Press `Esc` while streaming
- [ ] **Expected**: Cancels the current streaming response
- [ ] **Test**: Press `Esc` when error is shown
- [ ] **Expected**: Dismisses the error

### Scrolling

- [ ] **Test**: Press `PageUp`
- [ ] **Expected**: Scrolls viewport up by half page
- [ ] **Test**: Press `PageDown`
- [ ] **Expected**: Scrolls viewport down by half page
- [ ] **Test**: Press `Home`
- [ ] **Expected**: Scrolls to top of conversation
- [ ] **Test**: Press `End`
- [ ] **Expected**: Scrolls to bottom of conversation

### Routing Mode

- [ ] **Test**: Press `Ctrl+R`
- [ ] **Expected**: Cycles routing mode: local -> cloud -> hybrid -> local
- [ ] **Verify**: System message shows new mode

### Search

- [ ] **Test**: Press `Ctrl+F`
- [ ] **Expected**: Enters search mode with search input
- [ ] **Test**: In search mode, type a search term
- [ ] **Expected**: Matches are highlighted in conversation
- [ ] **Test**: Press `Enter` or `Ctrl+N` or `Down`
- [ ] **Expected**: Navigates to next match
- [ ] **Test**: Press `Ctrl+P` or `Up`
- [ ] **Expected**: Navigates to previous match
- [ ] **Test**: Press `Esc` or `Ctrl+F`
- [ ] **Expected**: Exits search mode

### Command Palette

- [ ] **Test**: Press `Ctrl+P` (if implemented)
- [ ] **Expected**: Opens command palette overlay
- [ ] **Test**: In palette, use `Up`/`Down` to navigate
- [ ] **Expected**: Selection moves through commands
- [ ] **Test**: Press `Enter` to select
- [ ] **Expected**: Executes selected command
- [ ] **Test**: Press `Esc` to close
- [ ] **Expected**: Closes command palette

### Welcome Screen

- [ ] **Test**: At welcome screen, press any key
- [ ] **Expected**: Transitions to chat view
- [ ] **Test**: At welcome screen, press `q`
- [ ] **Expected**: Exits the TUI
- [ ] **Test**: At welcome screen, press `Ctrl+C`
- [ ] **Expected**: Exits the TUI

---

## Context Mentions

Test @ mentions in the chat input:

### @file Mention

- [ ] **Test**: Type `Explain this file: @file:main.go` and press Enter
- [ ] **Expected**: File content is included in the context
- [ ] **Verify**: System message or context indicator shows "1 file"
- [ ] **Test**: Type `@file:"path with spaces/file.go"` (quoted path)
- [ ] **Expected**: Handles paths with spaces correctly
- [ ] **Test**: Type `@file:nonexistent.txt`
- [ ] **Expected**: Shows error about file not found

### @clipboard Mention

- [ ] **Setup**: Copy some text to clipboard
- [ ] **Test**: Type `@clipboard explain this` and press Enter
- [ ] **Expected**: Clipboard content is included in context
- [ ] **Verify**: Context indicator shows "clipboard"

### @git Mention

- [ ] **Test**: Type `@git what changed recently?` and press Enter
- [ ] **Expected**: Recent git info is included in context
- [ ] **Test**: Type `@git:HEAD~3 what changed?` and press Enter
- [ ] **Expected**: Git info for specified range is included

### @codebase Mention

- [ ] **Test**: Type `@codebase what does this project do?` and press Enter
- [ ] **Expected**: Directory structure is included in context
- [ ] **Verify**: Context indicator shows "codebase"

### @error Mention

- [ ] **Setup**: Trigger an error (e.g., ask about nonexistent model)
- [ ] **Test**: Type `@error what went wrong?` and press Enter
- [ ] **Expected**: Last error message is included in context
- [ ] **Verify**: Context indicator shows "error"

### @url Mention

- [ ] **Test**: Type `@url:https://example.com summarize this` and press Enter
- [ ] **Expected**: URL content is fetched and included in context
- [ ] **Test**: Type `@url:"https://example.com/path"` (quoted)
- [ ] **Expected**: Handles quoted URLs correctly

### Multiple Mentions

- [ ] **Test**: Type `@file:main.go @clipboard compare these` and press Enter
- [ ] **Expected**: Both contexts are included
- [ ] **Verify**: Context indicator shows "1 file, clipboard"

---

## OpenRouter Cloud Connection

### Setup Cloud API Key

- [ ] **Test**: Run `rigrun config set openrouter_key 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`
- [ ] **Expected**: Key is saved successfully
- [ ] **Test**: Run `rigrun status`
- [ ] **Expected**: Cloud Key shows "Configured"
- [ ] **Test**: Run `rigrun doctor`
- [ ] **Expected**: OpenRouter Configured check passes

### Cloud Routing in TUI

- [ ] **Test**: Launch TUI, type `/mode cloud` and press Enter
- [ ] **Expected**: Routing mode changes to cloud
- [ ] **Test**: Ask a complex question: "Explain quantum computing in detail"
- [ ] **Expected**: Response uses cloud tier (shown in routing info)
- [ ] **Verify**: Routing indicator shows tier (Haiku/Sonnet/Opus/GPT-4o)
- [ ] **Test**: Check routing tier selection for simple vs complex queries
  - [ ] Simple: "What is 1+1?" -> Should use lower tier (Haiku or Local)
  - [ ] Complex: "Explain transformer architecture in deep learning" -> Should use higher tier

### Hybrid Mode Routing

- [ ] **Test**: Type `/mode hybrid` and press Enter
- [ ] **Expected**: Routing mode changes to hybrid
- [ ] **Test**: Ask simple question: "What is Python?"
- [ ] **Expected**: May route to local (depends on complexity classification)
- [ ] **Test**: Ask complex question requiring reasoning
- [ ] **Expected**: Routes to cloud tier

### Cloud Fallback

- [ ] **Test**: Set invalid API key: `rigrun config set openrouter_key invalid-key`
- [ ] **Test**: Try cloud mode query
- [ ] **Expected**: Falls back to local with message "Local (cloud unavailable)"

---

## Ollama Local Connection

### Basic Connectivity

- [ ] **Test**: Ensure Ollama is running: `ollama serve`
- [ ] **Test**: Run `rigrun doctor`
- [ ] **Expected**: "Ollama Running" check passes
- [ ] **Test**: Run `rigrun status`
- [ ] **Expected**: Ollama section shows "Running" with version

### Ollama Not Running

- [ ] **Test**: Stop Ollama (close `ollama serve`)
- [ ] **Test**: Launch TUI with `rigrun`
- [ ] **Expected**: Error overlay: "Ollama Not Running"
- [ ] **Verify**: Suggestions show "Run: ollama serve"
- [ ] **Test**: Run `rigrun doctor`
- [ ] **Expected**: "Ollama Running" check fails with fix suggestion

### Auto-Start (if implemented)

- [ ] **Test**: Stop Ollama, then run `rigrun`
- [ ] **Expected**: Attempts to auto-start Ollama (may require 30s timeout)

### Model Not Found

- [ ] **Test**: Type `/model nonexistent-model-xyz` and press Enter
- [ ] **Expected**: Error or warning about model not found

### Local Mode

- [ ] **Test**: Type `/mode local` and press Enter
- [ ] **Expected**: Routing mode changes to local
- [ ] **Test**: Ask any question
- [ ] **Expected**: Uses only Ollama, routing shows "Local"
- [ ] **Verify**: No cloud API calls are made

### Paranoid Mode

- [ ] **Test**: Run `rigrun --paranoid`
- [ ] **Expected**: TUI launches with mode forced to "local"
- [ ] **Verify**: Welcome screen shows "local" mode
- [ ] **Test**: Try to change to cloud mode: `/mode cloud`
- [ ] **Expected**: Mode changes (but paranoid flag only affects startup)

---

## Session Management

### Save Session

- [ ] **Test**: Have a conversation with at least 2 messages
- [ ] **Test**: Type `/save` and press Enter
- [ ] **Expected**: "Conversation saved as: <title> (ID: <id>)"
- [ ] **Test**: Type `/save my-custom-name` and press Enter
- [ ] **Expected**: "Conversation saved as: my-custom-name (ID: <id>)"

### List Sessions

- [ ] **Test**: Type `/sessions` and press Enter
- [ ] **Expected**: Lists saved sessions with:
  - Index number [1], [2], etc.
  - Title/summary
  - Model name
  - Message count
  - Last updated date
  - Preview of content

### Load Session

- [ ] **Test**: Type `/load 1` and press Enter
- [ ] **Expected**: Loads first session, shows "Loaded session: <name>"
- [ ] **Verify**: Previous messages appear in conversation
- [ ] **Test**: Type `/load <session-id>` (full ID)
- [ ] **Expected**: Same result using full ID
- [ ] **Test**: Type `/load nonexistent-id`
- [ ] **Expected**: Error about session not found

### New Conversation After Load

- [ ] **Test**: Load a session, then type `/new`
- [ ] **Expected**: Starts fresh conversation, previous loaded session unchanged
- [ ] **Test**: Save the new conversation
- [ ] **Expected**: Creates new session, doesn't overwrite loaded one

### Empty Session Save

- [ ] **Test**: Start fresh (type `/new`), then immediately type `/save`
- [ ] **Expected**: "Nothing to save - conversation is empty"

---

## Search Functionality

### Enter Search Mode

- [ ] **Test**: Have a conversation with multiple messages
- [ ] **Test**: Press `Ctrl+F`
- [ ] **Expected**: Search bar appears with "Search:" prompt
- [ ] **Verify**: Input focus moves to search field

### Search Term Matching

- [ ] **Test**: Type a word that appears in the conversation
- [ ] **Expected**: Matches are highlighted in the viewport
- [ ] **Verify**: Match count shown (e.g., "1/3 matches")
- [ ] **Test**: Type a term that doesn't exist
- [ ] **Expected**: "No matches found" or similar indicator

### Navigate Matches

- [ ] **Test**: Press `Enter` or `Ctrl+N` or `Down`
- [ ] **Expected**: Moves to next match, viewport scrolls to show it
- [ ] **Test**: Press `Ctrl+P` or `Up`
- [ ] **Expected**: Moves to previous match
- [ ] **Test**: Navigate past last match
- [ ] **Expected**: Wraps to first match

### Exit Search Mode

- [ ] **Test**: Press `Esc`
- [ ] **Expected**: Search bar disappears, highlights removed
- [ ] **Test**: Press `Ctrl+F` again
- [ ] **Expected**: Exits search mode (toggle)

### Case Sensitivity

- [ ] **Test**: Search for "Hello" (capitalized)
- [ ] **Expected**: Matches "hello", "Hello", "HELLO" (case-insensitive)

### Unicode Search

- [ ] **Test**: Have conversation with Unicode characters
- [ ] **Test**: Search for Unicode term
- [ ] **Expected**: Matches Unicode correctly (rune-based)

---

## Cache System

### Cache Hit

- [ ] **Test**: Ask a question: "What is Python?"
- [ ] **Wait**: For response to complete
- [ ] **Test**: Ask the exact same question again
- [ ] **Expected**: Instant response from cache
- [ ] **Verify**: Routing shows "Cache (Exact)" with $0.00 cost

### Semantic Cache (if enabled)

- [ ] **Test**: Ask "What is Python programming language?"
- [ ] **Test**: Ask "Explain Python to me"
- [ ] **Expected**: May hit semantic cache (similar meaning)
- [ ] **Verify**: Routing shows "Cache (Semantic)" if hit

### Cache Stats

- [ ] **Test**: Type `/config cache stats`
- [ ] **Expected**: Shows hit rate, entry counts, etc.
- [ ] **Test**: Type `/status`
- [ ] **Expected**: Session section shows cache hits

### Cache Clear

- [ ] **Test**: Type `/config cache clear`
- [ ] **Expected**: Cache cleared message
- [ ] **Test**: Ask previously cached question
- [ ] **Expected**: Full LLM response (no cache hit)

### Cache Toggle

- [ ] **Test**: Type `/config cache off`
- [ ] **Expected**: Caching disabled
- [ ] **Test**: Ask a question twice
- [ ] **Expected**: No cache hit (always fresh)
- [ ] **Test**: Type `/config cache on`
- [ ] **Expected**: Caching re-enabled

---

## Tool System

The tool system enables agentic capabilities. Test these if tools are enabled:

### Tool Availability

- [ ] **Test**: Type `/tools`
- [ ] **Expected**: Lists available tools:
  - Read - Read file contents
  - Write - Write to files
  - Edit - Edit files
  - Glob - Search for files by pattern
  - Grep - Search file contents
  - Bash - Execute shell commands

### Tool Toggle

- [ ] **Test**: Type `/tool read off`
- [ ] **Expected**: Read tool disabled
- [ ] **Test**: Type `/tool read on`
- [ ] **Expected**: Read tool enabled

### Tool Execution (Agentic Loop)

- [ ] **Test**: Ask "Read the contents of main.go"
- [ ] **Expected**: Model uses Read tool, shows "Executing tool: read"
- [ ] **Verify**: Tool result appears in conversation
- [ ] **Test**: Ask "List all .go files in this directory"
- [ ] **Expected**: Model uses Glob tool

### Tool Permission

- [ ] **Test**: Ask the model to write a file
- [ ] **Expected**: May request permission before executing (based on risk level)

---

## Error Handling

### Ollama Connection Errors

- [ ] **Test**: Stop Ollama while TUI is running
- [ ] **Test**: Send a message
- [ ] **Expected**: Error overlay appears with connection error
- [ ] **Verify**: Press `Esc` or `Enter` dismisses error

### Model Not Found

- [ ] **Test**: Type `/model nonexistent:latest`
- [ ] **Test**: Send a message
- [ ] **Expected**: Model not found error with suggestions

### Timeout

- [ ] **Test**: Send extremely long prompt that might timeout
- [ ] **Expected**: Timeout error with suggestion to try again

### Invalid Command

- [ ] **Test**: Type `/invalid_command` and press Enter
- [ ] **Expected**: "Unknown command" message with help suggestion

### File Not Found (@ mention)

- [ ] **Test**: Type `@file:nonexistent_file.txt explain`
- [ ] **Expected**: Context warning about file not found

### API Key Errors

- [ ] **Test**: Set invalid OpenRouter key
- [ ] **Test**: Force cloud mode and send query
- [ ] **Expected**: Graceful fallback to local with error indication

---

## Edge Cases

### Empty Input

- [ ] **Test**: Press `Enter` with empty input
- [ ] **Expected**: Nothing happens (no empty messages sent)

### Very Long Input

- [ ] **Test**: Type message near 4096 character limit
- [ ] **Verify**: Character counter shows usage (e.g., "4000 / 4096 chars")
- [ ] **Test**: Try to exceed limit
- [ ] **Expected**: Input stops accepting characters

### Rapid Message Sending

- [ ] **Test**: Send multiple messages quickly without waiting for responses
- [ ] **Expected**: Previous streaming cancelled, new request starts

### Resize Terminal

- [ ] **Test**: Resize terminal window while TUI is running
- [ ] **Expected**: UI adapts to new size without crashing
- [ ] **Verify**: Text wraps correctly, no overflow

### Special Characters

- [ ] **Test**: Send message with special characters: `!@#$%^&*()[]{}|;:'"<>,./?\``
- [ ] **Expected**: Characters handled correctly in conversation
- [ ] **Test**: Send message with Unicode:
- [ ] **Expected**: Unicode displayed correctly

### Newlines in Input

- [ ] **Test**: Paste multi-line text
- [ ] **Expected**: Handled as single message (newlines preserved or flattened)

### Command With Extra Spaces

- [ ] **Test**: Type `/model   qwen2.5:7b` (extra spaces)
- [ ] **Expected**: Command parsed correctly

### Quoted Arguments

- [ ] **Test**: Type `/save "My Session Name With Spaces"`
- [ ] **Expected**: Full name preserved including spaces

### Cancel During Tool Execution

- [ ] **Test**: Start a tool-using query
- [ ] **Test**: Press `Ctrl+C` during tool execution
- [ ] **Expected**: Execution cancelled gracefully

### Network Interruption

- [ ] **Test**: Disconnect network during cloud streaming
- [ ] **Expected**: Error shown, fallback behavior

### Session Storage Full

- [ ] **Test**: Save many sessions (20+)
- [ ] **Test**: List and load sessions
- [ ] **Expected**: All sessions accessible, pagination if needed

### Concurrent Operations

- [ ] **Test**: While streaming, try to change model
- [ ] **Expected**: Either queued or rejected gracefully

---

## Performance Checklist

- [ ] **TUI startup**: Less than 2 seconds
- [ ] **First token latency**: Under 500ms for local model
- [ ] **Viewport scrolling**: Smooth with large conversations
- [ ] **Search highlighting**: Responsive even with many matches
- [ ] **Memory usage**: Stable over extended sessions

---

## Regression Checklist

After making changes, verify these core flows still work:

1. [ ] Fresh TUI launch -> Welcome screen -> Chat
2. [ ] Ask question -> Stream response -> Complete
3. [ ] Save session -> List sessions -> Load session
4. [ ] Search in conversation -> Navigate matches -> Exit search
5. [ ] Change routing mode -> Query routes correctly
6. [ ] Cache hit on repeated query
7. [ ] Tool execution in agentic loop
8. [ ] All slash commands respond appropriately
9. [ ] All keyboard shortcuts functional
10. [ ] Error handling for common scenarios

---

## Test Completion Sign-off

| Section | Tester | Date | Status |
|---------|--------|------|--------|
| CLI Commands | | | |
| TUI Slash Commands | | | |
| Keyboard Shortcuts | | | |
| Context Mentions | | | |
| OpenRouter Cloud | | | |
| Ollama Local | | | |
| Session Management | | | |
| Search Functionality | | | |
| Cache System | | | |
| Tool System | | | |
| Error Handling | | | |
| Edge Cases | | | |

---

## Notes

- API Key for OpenRouter testing: `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`
- Config file location: `~/.rigrun/config.toml`
- Cache location: `~/.rigrun/cache.json`
- Sessions location: `~/.rigrun/conversations/`
