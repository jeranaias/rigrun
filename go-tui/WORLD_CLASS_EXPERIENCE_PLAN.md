# World-Class LLM TUI Experience Plan for rigrun

**Author**: Claude Opus 4.5
**Date**: 2026-01-24
**Version**: 1.0
**Purpose**: Comprehensive roadmap to make rigrun the best DoD-compliant LLM TUI available

---

## Table of Contents

1. [Vision Statement](#1-vision-statement)
2. [Research Summary](#2-research-summary)
3. [Current State Analysis](#3-current-state-analysis)
4. [Gap Analysis](#4-gap-analysis)
5. [Feature Roadmap](#5-feature-roadmap)
6. [Implementation Plan](#6-implementation-plan)
7. [Technical Architecture](#7-technical-architecture)
8. [Success Metrics](#8-success-metrics)

---

## 1. Vision Statement

### What "World-Class" Means for rigrun

**rigrun will be the gold standard for DoD-compliant LLM terminal interfaces**, combining:

1. **Exceptional User Experience**
   - Instant, intuitive interactions that feel magical
   - Zero cognitive friction between thought and execution
   - Beautiful, informative UI that respects the terminal aesthetic
   - Responsive feedback for every user action

2. **Best-in-Class Agentic Capabilities**
   - Autonomous multi-step task execution with safety guardrails
   - Intelligent context awareness (@file, @git, @codebase, @error)
   - Tool execution that "just works" with minimal prompting
   - Self-healing error recovery and adaptive planning

3. **DoD IL5 Compliance Without Compromise**
   - Full NIST 800-53 controls that enhance rather than hinder UX
   - Transparent security indicators integrated into the flow
   - Air-gapped operation that feels as good as cloud
   - Audit trails that provide value, not just compliance checkboxes

4. **Developer Productivity Multiplier**
   - Faster than switching to IDE or browser
   - Smarter than manual copy-paste workflows
   - More reliable than remembering commands
   - More powerful than individual CLI tools

### Core Principles

1. **Speed Over Features**: Every interaction should feel instant
2. **Clarity Over Cleverness**: Users should never be confused
3. **Safety Over Convenience**: Security should be invisible but uncompromising
4. **Intelligence Over Automation**: Be smart about context, not just fast

---

## 2. Research Summary

### What Makes Leading LLM TUIs Exceptional

Based on research of Claude Code, Aider, Cursor, and other tools, the key differentiators are:

#### A. Claude Code Excellence

**Source**: [GitHub - anthropics/claude-code](https://github.com/anthropics/claude-code), [Anthropic Engineering Blog](https://www.anthropic.com/engineering/claude-code-best-practices)

**Key Strengths**:
- Multi-agent architecture (general, explore, plan, guide, statusline)
- Agentic search discovers context without manual selection
- Natural language for all operations (commits, PRs, troubleshooting)
- Background task execution with notification system
- Seamless IDE integration with terminal-first philosophy
- Task chaining and iterative debugging

**UX Patterns**:
- Always show what's happening (streaming status updates)
- Clear permission boundaries (auto vs ask vs never)
- Recoverable errors with suggested actions
- Context that persists across conversations (CLAUDE.md)

#### B. Aider Excellence

**Source**: [Aider Documentation](https://aider.chat/docs/), [Agentic CLI Comparison](https://research.aimultiple.com/agentic-cli/)

**Key Strengths**:
- Repository map for efficient context (function signatures + structure)
- Git-native workflow (every change is a commit)
- Model flexibility (works with almost any LLM)
- Multimodal input (images, web pages, voice)
- Automatic linting and testing after changes
- Token cost transparency

**UX Patterns**:
- X of Y progress for multi-step processes
- Clear visual separation of AI vs user content
- Instant access to diffs and undo via git
- Proactive context suggestions ("Did you mean to include X?")

#### C. Cursor/Continue Excellence

**Source**: [Cursor Context Management](https://stevekinning.com/courses/ai-development/cursor-context), [How Cursor Remembers](https://medium.com/@bswa006/how-cursor-remembers-your-codebase-so-you-dont-have-to-72c04583ccb7)

**Key Strengths**:
- @ symbol system for surgical context injection
- Codebase indexing with semantic search (400k-500k files)
- Automatic context awareness without manual @mentions
- Multiple context files (.cursorrules, AI-CONTEXT.md, AI-PATTERNS.md)
- Incremental updates to context (don't re-process unchanged files)

**UX Patterns**:
- Tab completion for @mentions
- Visual indicators showing what context is active
- Smart truncation for large files (summaries with expansion)
- Context cost awareness (token estimates before sending)

#### D. Terminal TUI Best Practices

**Source**: [Evil Martians CLI UX](https://evilmartians.com/chronicles/cli-ux-best-practices-3-patterns-for-improving-progress-displays), [Terminal.Gui](https://github.com/gui-cs/Terminal.Gui)

**Key Patterns**:
1. **Progress Indicators**
   - Use X of Y when you know total steps
   - Use progress bars when you can measure completion
   - Use spinners only for indeterminate waits
   - Always clean up indicators when complete

2. **Streaming Output**
   - Show first token ASAP (time-to-first-token matters)
   - Update smoothly (10-60fps for animations)
   - Preserve terminal history (no full-screen takeover unless needed)
   - Support Ctrl+C at any point

3. **Error Handling**
   - Show errors inline when possible
   - Suggest recovery actions
   - Preserve user input on error
   - Log errors for debugging without cluttering UI

4. **Keyboard Shortcuts**
   - Vim-like navigation (h/j/k/l)
   - Ctrl+R for history search
   - Ctrl+F for in-conversation search
   - Tab for completion
   - Esc for cancel/back

---

## 3. Current State Analysis

### Strengths (What's Already Great)

rigrun already has a strong foundation:

#### Security & Compliance (Best-in-Class)
- **Comprehensive IL5 Controls**: Full NIST 800-53 implementation (50+ controls)
- **Audit System**: HMAC-protected audit logs with AU-9 protection
- **Encryption**: SC-28 data-at-rest with keystore integration
- **Classification Banners**: DoDI 5200.48 compliance with AC-4 enforcement
- **Session Management**: AC-12 timeout with warning/extension
- **Consent Banner**: AC-8 system use notification
- **Offline Mode**: SC-7 air-gapped operation with localhost-only networking

#### Architecture (Solid Foundation)
- **Tool System**: 8 agentic tools (Read, Glob, Grep, Write, Edit, Bash, WebFetch, WebSearch)
- **Permission Model**: Auto/Ask/Never with user confirmation
- **Agentic Loop**: Safety limits (25 iterations, 30min timeout, consecutive error tracking)
- **Slash Commands**: 16+ commands with category organization
- **Context Mentions**: @file, @git, @codebase, @error, @clipboard
- **Routing System**: Cache -> Local -> Cloud with intelligent tier selection
- **Session Persistence**: Save/load conversations with metadata

#### UI Components (Good Start)
- **Bubble Tea Framework**: Modern, reactive TUI
- **Spinner Components**: Multiple styles (braille, dots, line, pulse, block)
- **Classification Banner**: Top-level security indicator
- **Error Display**: Full-screen error overlays
- **Viewport**: Scrollable chat history
- **Status Bar**: Model/mode/GPU indicators

#### Developer Experience
- **50+ CLI Commands**: Comprehensive command suite
- **JSON Output**: Machine-readable for automation
- **Doctor Command**: Built-in diagnostics
- **Config Management**: TOML with environment variable overrides

### Weaknesses (What Needs Work)

#### UX Gaps

1. **No Progressive Disclosure**
   - Help system dumps all commands at once (overwhelming)
   - No onboarding flow for new users
   - No contextual help (e.g., "Looks like you want to load a file, try @file")

2. **Limited Visual Feedback**
   - No progress indication for multi-step agentic loops
   - Tool execution happens in background without visibility
   - No indication of what context is being used
   - Streaming doesn't show partial context (e.g., which files are being read)

3. **Command Discovery**
   - Tab completion exists but not discoverable
   - No fuzzy search for commands
   - No recent command history
   - No command palette (Ctrl+P pattern from Claude Code)

4. **Error Recovery**
   - Errors don't suggest fixes
   - No "retry with different model" option
   - No automatic fallback hints
   - Input is lost on error

5. **Context Management**
   - @mentions work but don't show token cost estimates
   - No visual indicator of active context
   - No codebase indexing (re-reads files every time)
   - No smart file suggestions based on current query

#### Performance Gaps

1. **No Caching of File Reads**
   - @file re-reads every time
   - @git re-executes git commands
   - @codebase rescans on every mention

2. **No Incremental Context Updates**
   - Conversation grows unbounded (no summarization)
   - Old messages stay in context (not truncated intelligently)

3. **Blocking Operations**
   - Some tools block the UI thread
   - No background task queue

#### Feature Gaps

1. **No Multi-Turn Planning**
   - Agentic loop is reactive (responds to tool calls)
   - No explicit "plan then execute" mode
   - No progress tracking across plan steps

2. **No Collaboration Features**
   - Can't share sessions easily (only local save/load)
   - No session export to markdown/HTML
   - No diff view for tool-modified files

3. **No Advanced Search**
   - No Ctrl+F in-conversation search
   - No semantic search over history
   - No regex search in messages

4. **Limited Model Management**
   - No model download UI
   - No model benchmarking
   - No model recommendation based on query

5. **No Telemetry Dashboard**
   - Can't see cost over time
   - Can't see token usage trends
   - Can't see which tools are most used

---

## 4. Gap Analysis

### UX Gaps vs World-Class

| Feature | rigrun Current | World-Class Standard | Priority | Effort |
|---------|---------------|---------------------|----------|--------|
| **Command Palette** | None | Ctrl+P fuzzy search (Claude Code) | HIGH | M |
| **Progressive Help** | All at once | Contextual, just-in-time | HIGH | S |
| **Progress Indicators** | Basic spinner | X of Y for multi-step (Aider) | HIGH | M |
| **Context Visibility** | Hidden | Shows active context + cost (Cursor) | HIGH | M |
| **Error Recovery** | Generic errors | Suggested actions + retry (Claude) | HIGH | M |
| **History Search** | None | Ctrl+F with highlighting | MEDIUM | S |
| **Tab Completion** | Basic | Rich with previews (Cursor) | MEDIUM | M |
| **Onboarding** | None | Interactive tutorial | MEDIUM | M |
| **Background Tasks** | None | Task queue with notifications (Claude) | LOW | L |
| **Diff View** | None | Inline diffs for changes (Aider) | LOW | L |

### Performance Gaps vs World-Class

| Feature | rigrun Current | World-Class Standard | Priority | Effort |
|---------|---------------|---------------------|----------|--------|
| **Context Caching** | None | Indexed codebase (Cursor) | HIGH | L |
| **File Read Cache** | None | LRU cache with invalidation | HIGH | S |
| **Smart Truncation** | None | Summarize old messages | MEDIUM | M |
| **Async Tool Execution** | Partial | Full async with progress | MEDIUM | M |
| **Model Preloading** | None | Warm model pool | LOW | M |
| **Streaming Optimization** | Good | Token batching for smoothness | LOW | S |

### Feature Gaps vs World-Class

| Feature | rigrun Current | World-Class Standard | Priority | Effort |
|---------|---------------|---------------------|----------|--------|
| **Plan Mode** | None | Explicit plan-then-execute (Claude) | HIGH | L |
| **Repository Map** | None | Function signatures + structure (Aider) | HIGH | XL |
| **Session Export** | JSON only | MD/HTML with styling | MEDIUM | S |
| **Model Benchmarking** | None | Auto-benchmark on new models | MEDIUM | M |
| **Cost Dashboard** | Basic stats | Trends + breakdowns | LOW | M |
| **Voice Input** | None | Speech-to-text (Aider) | LOW | L |
| **Multimodal** | None | Image context (Aider) | LOW | XL |

### IL5/Security Gaps (Minimal)

rigrun is already best-in-class for DoD compliance. Minor enhancements:

| Feature | Current | Enhancement | Priority | Effort |
|---------|---------|-------------|----------|--------|
| **Audit Visualization** | CLI only | TUI dashboard | LOW | M |
| **RBAC UI** | CLI only | Visual permission viewer | LOW | M |
| **Spillage Detection** | Basic | Real-time highlighting | MEDIUM | S |
| **Classification Hints** | Banner only | Inline warnings in context | MEDIUM | S |

**Priority**: HIGH (must-have), MEDIUM (should-have), LOW (nice-to-have)
**Effort**: S (1-2 days), M (3-5 days), L (1-2 weeks), XL (2-4 weeks)

---

## 5. Feature Roadmap

### Priority 1: Quick Wins (Immediate Impact, Low Effort)

**Goal**: Ship visible improvements within 1 week

#### 1.1 Progressive Help System (S)
**User Impact**: New users can get started without being overwhelmed
**Implementation**:
- Welcome screen shows top 5 commands
- `/help quick` shows essentials
- `/help all` shows full list
- Contextual tips ("Press Tab to complete")

**Acceptance Criteria**:
- [ ] Welcome screen shows max 5 starter commands
- [ ] Help is categorized (Basic, Advanced, Security)
- [ ] Tips appear based on user actions (no tips after 10 sessions)

#### 1.2 File Read Cache (S)
**User Impact**: @file mentions 10x faster on repeated use
**Implementation**:
- LRU cache with 100MB limit
- Invalidate on file modification time change
- Show "cached" indicator in verbose mode

**Acceptance Criteria**:
- [ ] Second @file mention is instant (<50ms)
- [ ] Cache invalidates on file change
- [ ] Cache respects memory limit

#### 1.3 History Search (Ctrl+F) (S)
**User Impact**: Find previous responses without scrolling
**Implementation**:
- Ctrl+F enters search mode
- Type to search, Enter to jump to next match
- Esc to exit search
- Highlight matches in viewport

**Acceptance Criteria**:
- [ ] Ctrl+F activates search mode
- [ ] Search highlights all matches
- [ ] N/Shift+N to navigate matches
- [ ] Case-insensitive by default (Ctrl+C for case-sensitive)

#### 1.4 Error Recovery Suggestions (S)
**User Impact**: Users know what to do when things fail
**Implementation**:
- Pattern-match common errors
- Show 1-3 suggested actions
- Make suggestions clickable (or show command to run)

**Acceptance Criteria**:
- [ ] Network errors suggest "Check connection or try offline mode"
- [ ] Model not found suggests "Run: ollama pull <model>"
- [ ] Permission denied suggests "Check file permissions"
- [ ] Suggestions shown inline in error display

#### 1.5 Context Cost Estimates (S)
**User Impact**: Users know token impact before sending
**Implementation**:
- Count tokens in @mentions before sending
- Show "Adding ~5,000 tokens" when @file is used
- Warn if over 100k tokens
- Show cost estimate for cloud routing

**Acceptance Criteria**:
- [ ] @file shows token count estimate
- [ ] Warning at 100k tokens
- [ ] Cloud cost shown before sending (if routing to cloud)
- [ ] Can cancel before sending if cost too high

### Priority 2: Core UX Improvements (High Impact, Medium Effort)

**Goal**: Ship professional-grade UX within 2-3 weeks

#### 2.1 Command Palette (Ctrl+P) (M)
**User Impact**: Instant access to any command without memorizing
**Implementation**:
- Ctrl+P opens fuzzy search over all commands
- Type to filter (fuzzy match)
- Up/Down to select, Enter to execute
- Show command description in preview
- Recent commands bubble to top

**Acceptance Criteria**:
- [ ] Ctrl+P opens palette
- [ ] Fuzzy search works (e.g., "sv" matches "/save")
- [ ] Recent commands shown first
- [ ] Preview shows command description + args
- [ ] Esc to close without executing

#### 2.2 Progress Indicators for Agentic Loops (M)
**User Impact**: Users see what's happening during multi-step tasks
**Implementation**:
- Show "Step X of Y" for plan execution
- Show current tool being executed
- Show elapsed time per step
- Show overall progress bar

**Acceptance Criteria**:
- [ ] Multi-step tasks show "Step 2 of 5"
- [ ] Current tool execution visible ("Running: Bash ls -la")
- [ ] Progress bar shows completion %
- [ ] Can cancel at any step with Ctrl+C

#### 2.3 Active Context Indicator (M)
**User Impact**: Users see what context is being sent with each message
**Implementation**:
- Status bar shows active @mentions
- Hover/expand to see token counts
- Click to remove context before sending
- Persist context across messages (optional)

**Acceptance Criteria**:
- [ ] Status bar shows "@file:main.go (+2.5k tokens)"
- [ ] Can click to remove context
- [ ] Can pin context to persist across messages
- [ ] Shows total context size before sending

#### 2.4 Enhanced Tab Completion (M)
**User Impact**: Faster command entry with rich previews
**Implementation**:
- Tab shows completions with descriptions
- Tab twice cycles through options
- Preview shows example usage
- Complete paths, models, sessions, tools

**Acceptance Criteria**:
- [ ] Tab on `/` shows all commands
- [ ] Tab on `/model ` shows available models
- [ ] Tab on `@file:` shows file paths
- [ ] Descriptions shown inline
- [ ] Tab twice cycles options

#### 2.5 Onboarding Tutorial (M)
**User Impact**: New users productive in 60 seconds
**Implementation**:
- First run shows interactive tutorial
- Teaches: basic chat, slash commands, @mentions, tools
- Can skip or restart via `/tutorial`
- Progress saved (don't repeat completed steps)

**Acceptance Criteria**:
- [ ] First run shows "Welcome! Let's learn rigrun"
- [ ] 5 interactive steps (type to advance)
- [ ] Can skip with Esc
- [ ] Can restart with `/tutorial`
- [ ] Progress saved to config

#### 2.6 Improved Error Display (M)
**User Impact**: Errors are informative, not scary
**Implementation**:
- Categorize errors (network, model, tool, config)
- Show error + context + suggested actions
- Link to docs for complex errors
- Show logs path for debugging

**Acceptance Criteria**:
- [ ] Error category shown (e.g., "Network Error")
- [ ] Context shown ("While connecting to Ollama")
- [ ] 1-3 suggested actions shown
- [ ] Link to docs if applicable
- [ ] Logs path shown for debugging

### Priority 3: Advanced Features (High Impact, High Effort)

**Goal**: Ship over 4-6 weeks, one feature at a time

#### 3.1 Plan Mode (L)
**User Impact**: Complex tasks execute reliably with transparency
**Implementation**:
- `/plan <task>` creates execution plan
- LLM breaks task into steps
- User approves plan before execution
- Shows progress through steps
- Can pause/resume/modify plan

**Acceptance Criteria**:
- [ ] `/plan "Refactor auth to use RBAC"` generates 5-step plan
- [ ] User approves plan before execution
- [ ] Progress shown (Step 2/5: Running tests)
- [ ] Can pause and resume
- [ ] Can edit steps before execution

**Technical Design**:
```go
// Plan represents an execution plan
type Plan struct {
    ID          string
    Description string
    Steps       []PlanStep
    Status      PlanStatus // Draft, Approved, Running, Paused, Complete
    CurrentStep int
}

// PlanStep is a single step in the plan
type PlanStep struct {
    ID          string
    Description string
    ToolCalls   []ToolCall
    Status      StepStatus // Pending, Running, Complete, Failed
    Result      string
    Error       error
}

// PlanExecutor executes a plan step-by-step
type PlanExecutor struct {
    plan     *Plan
    executor *Executor
    onProgress func(step int, total int, status string)
}
```

#### 3.2 Repository Map / Codebase Indexing (XL)
**User Impact**: @codebase becomes instant and intelligent
**Implementation**:
- Scan codebase on startup (or demand)
- Build index: files -> functions/classes/types
- Store in SQLite with FTS
- Incremental updates (watch for changes)
- Semantic search over code

**Acceptance Criteria**:
- [ ] Initial index <5s for 10k files
- [ ] Incremental updates <100ms
- [ ] @codebase returns relevant files (not all)
- [ ] Search finds functions by name
- [ ] Shows repository structure

**Technical Design**:
```go
// CodebaseIndex indexes a repository for fast search
type CodebaseIndex struct {
    db        *sql.DB
    watcher   *fsnotify.Watcher
    root      string
}

// Symbol represents a code symbol (function, class, type)
type Symbol struct {
    Name       string
    Type       SymbolType // Function, Class, Type, Variable
    File       string
    Line       int
    Signature  string
    Doc        string
}

// Search finds symbols matching query
func (idx *CodebaseIndex) Search(query string) []Symbol
```

#### 3.3 Background Task System (L)
**User Impact**: Long tasks don't block the UI
**Implementation**:
- Queue for long-running tasks
- Notifications when complete
- Can switch conversations while task runs
- Task history and logs

**Acceptance Criteria**:
- [ ] `/task "Run tests" bash "go test ./..."` queues task
- [ ] Notification when task completes
- [ ] `/tasks` shows running/completed tasks
- [ ] Can view task output
- [ ] Can cancel running tasks

**Technical Design**:
```go
// TaskQueue manages background tasks
type TaskQueue struct {
    tasks   []*Task
    running map[string]*Task
    mu      sync.Mutex
}

// Task represents a background task
type Task struct {
    ID          string
    Description string
    Command     string
    Status      TaskStatus // Queued, Running, Complete, Failed
    StartTime   time.Time
    EndTime     time.Time
    Output      string
    Error       error
}
```

#### 3.4 Session Export with Styling (S-M)
**User Impact**: Share beautiful conversation logs
**Implementation**:
- Export to Markdown with syntax highlighting
- Export to HTML with CSS styling
- Export to PDF via HTML -> PDF
- Include metadata (model, timestamp, tokens)

**Acceptance Criteria**:
- [ ] `/export markdown` creates styled .md file
- [ ] `/export html` creates styled .html file
- [ ] Syntax highlighting in code blocks
- [ ] Metadata header (date, model, stats)
- [ ] Opens in default app

#### 3.5 Model Benchmarking (M)
**User Impact**: Know which model is best for your use case
**Implementation**:
- `/benchmark` runs standard test suite
- Measures: speed, quality, cost
- Compares models on same prompts
- Saves results for future reference

**Acceptance Criteria**:
- [ ] `/benchmark qwen2.5-coder:14b` runs tests
- [ ] Reports: tokens/sec, TTFT, accuracy score
- [ ] Can compare multiple models
- [ ] Results saved to ~/.rigrun/benchmarks.json

### Priority 4: Polish & Optimization (Medium Impact, Low-Medium Effort)

**Goal**: Ship continuously over 6-8 weeks

#### 4.1 Smart Context Truncation (M)
**User Impact**: Long conversations don't slow down or fail
**Implementation**:
- Summarize old messages
- Keep recent N messages in full
- Keep system prompt + first message
- Show "Conversation summarized" indicator

**Acceptance Criteria**:
- [ ] After 50 messages, old messages summarized
- [ ] Summary preserves key facts
- [ ] User can expand summary if needed
- [ ] Performance stays constant as conversation grows

#### 4.2 Streaming Optimization (S)
**User Impact**: Smoother, faster streaming
**Implementation**:
- Batch tokens for rendering (10-20 tokens/frame)
- Render at 30fps max
- Use efficient string building
- Optimize viewport updates

**Acceptance Criteria**:
- [ ] Streaming feels smooth (no flicker)
- [ ] UI responsive during streaming
- [ ] Can scroll while streaming
- [ ] Memory usage stays constant

#### 4.3 Vim Mode (M)
**User Impact**: Vim users feel at home
**Implementation**:
- Normal mode: j/k for scroll, / for search, : for command
- Insert mode: standard text input
- Visual mode: select text for copying
- Configurable via config.toml

**Acceptance Criteria**:
- [ ] j/k scrolls viewport
- [ ] i enters insert mode
- [ ] Esc returns to normal mode
- [ ] :w saves conversation
- [ ] Can disable vim mode in config

#### 4.4 Inline Diffs (L)
**User Impact**: See exactly what tools changed
**Implementation**:
- When Edit/Write tool runs, show diff
- Highlight added (green) and removed (red)
- Can approve/reject changes
- Integrates with git for commits

**Acceptance Criteria**:
- [ ] Edit tool shows diff before applying
- [ ] User can approve/reject
- [ ] Changes highlighted in color
- [ ] Diffs saved to session

#### 4.5 Cost Dashboard (M)
**User Impact**: Understand spending and optimize usage
**Implementation**:
- Track costs per session
- Show trends over time
- Break down by tier (cache/local/cloud)
- Show most expensive queries

**Acceptance Criteria**:
- [ ] `/cost` shows current session cost
- [ ] `/cost history` shows trends
- [ ] Chart shows cache/local/cloud breakdown
- [ ] Can filter by date range

---

## 6. Implementation Plan

### Phase 1: Quick Wins (Week 1-2)

**Goal**: Ship 5 high-impact, low-effort improvements

**Week 1**:
- Day 1-2: Progressive Help System
  - Refactor help display to categorize commands
  - Add contextual tips system
  - Update welcome screen
- Day 3: File Read Cache
  - Implement LRU cache with mtime invalidation
  - Add cache hit/miss metrics
- Day 4-5: History Search (Ctrl+F)
  - Add search mode to chat model
  - Implement match highlighting
  - Add navigation (N/Shift+N)

**Week 2**:
- Day 1-2: Error Recovery Suggestions
  - Pattern-match common errors
  - Add suggestion engine
  - Update error display component
- Day 3-4: Context Cost Estimates
  - Add token counter for @mentions
  - Show estimates before sending
  - Add cost warning for large context
- Day 5: Testing & Polish
  - Integration tests for new features
  - Documentation updates
  - User testing feedback

**Deliverables**:
- Release v0.4.0 with "Quick Wins"
- Updated docs with new features
- Blog post: "5 UX Improvements in rigrun v0.4"

### Phase 2: Core UX (Week 3-5)

**Goal**: Ship professional-grade command interface

**Week 3**:
- Day 1-3: Command Palette (Ctrl+P)
  - Fuzzy search implementation
  - Command preview component
  - Recent command tracking
- Day 4-5: Progress Indicators
  - Step counter for agentic loops
  - Current tool display
  - Progress bar component

**Week 4**:
- Day 1-2: Active Context Indicator
  - Status bar enhancement
  - Context management UI
  - Pin/unpin context
- Day 3-5: Enhanced Tab Completion
  - Rich completion engine
  - Preview component
  - Path/model/session completers

**Week 5**:
- Day 1-3: Onboarding Tutorial
  - Interactive tutorial system
  - Progress tracking
  - Skip/restart logic
- Day 4-5: Improved Error Display
  - Error categorization
  - Contextual information
  - Doc links

**Deliverables**:
- Release v0.5.0 "Professional UI"
- Video demo of new features
- User guide updates

### Phase 3: Advanced Features (Week 6-11)

**Goal**: Ship game-changing capabilities

**Week 6-7: Plan Mode**
- Plan data structures
- Plan generation via LLM
- User approval UI
- Step execution engine
- Pause/resume logic

**Week 8-10: Repository Map**
- Code indexing engine
- SQLite schema design
- File watcher integration
- Semantic search
- Incremental updates

**Week 11: Background Tasks**
- Task queue implementation
- Notification system
- Task history UI
- Task output viewer

**Deliverables**:
- Release v0.6.0 "Advanced Agent"
- Case studies of complex tasks
- Architecture documentation

### Phase 4: Polish (Week 12-16)

**Goal**: Refine and optimize every interaction

**Week 12**: Smart truncation, streaming optimization
**Week 13**: Vim mode, inline diffs
**Week 14**: Cost dashboard, model benchmarking
**Week 15**: Session export, multimodal prep
**Week 16**: Testing, docs, release

**Deliverables**:
- Release v1.0.0 "World-Class"
- Comprehensive user guide
- Video tutorial series
- Benchmark report vs competitors

---

## 7. Technical Architecture

### 7.1 Command Palette Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Command Palette                      │
│  ┌───────────────────────────────────────────────────┐ │
│  │ > sv                                              │ │
│  └───────────────────────────────────────────────────┘ │
│                                                          │
│  ┌─ Search Results ──────────────────────────────────┐ │
│  │ > /save [name]          Save conversation        │ │
│  │   /service status       Security service status  │ │
│  │   /verify software      Verify software          │ │
│  └──────────────────────────────────────────────────┘ │
│                                                          │
│  ┌─ Preview ─────────────────────────────────────────┐ │
│  │ /save [name]                                      │ │
│  │                                                    │ │
│  │ Save current conversation to storage.             │ │
│  │ If no name provided, auto-generates title.        │ │
│  │                                                    │ │
│  │ Examples:                                         │ │
│  │   /save my-session                                │ │
│  │   /save                                           │ │
│  └───────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

**Components**:
- `CommandPalette` (Bubble Tea model)
- `FuzzyMatcher` (scoring algorithm)
- `CommandRegistry` (command metadata)
- `RecentTracker` (usage history)

**Key Files**:
- `internal/ui/components/command_palette.go`
- `internal/commands/fuzzy.go`
- `internal/commands/recent.go`

### 7.2 Progress Indicator Architecture

```
┌─────────────────────────────────────────────────────────┐
│  ⠙ Executing Plan: "Refactor auth system"              │
│  ───────────────────────────────────────────────────    │
│  Step 3 of 5: Running tests                             │
│  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░░░░░░░ 60% (1m 23s)│
│                                                          │
│  ✓ Step 1: Read auth files (2.3s)                       │
│  ✓ Step 2: Create RBAC tests (15.7s)                    │
│  ⠿ Step 3: Run test suite (running...)                  │
│    ├─ TestRBACEnforcer (0.5s)                           │
│    ├─ TestPermissionCheck (0.3s)                        │
│    └─ TestRoleHierarchy (running...)                    │
│  ○ Step 4: Update documentation                         │
│  ○ Step 5: Commit changes                               │
└─────────────────────────────────────────────────────────┘
```

**Components**:
- `ProgressTracker` (state manager)
- `StepIndicator` (X of Y display)
- `ProgressBar` (visual % complete)
- `StepTree` (nested step display)

**Key Files**:
- `internal/ui/components/progress.go`
- `internal/tools/plan_executor.go`

### 7.3 Context Indicator Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Active Context:                                        │
│  ┌────────────────────────────────────────────────────┐│
│  │ @file:main.go                           +2.5k [×]  ││
│  │ @file:auth.go                           +1.8k [×]  ││
│  │ @git:recent                             +0.5k [×]  ││
│  │ @clipboard                              +0.2k [×]  ││
│  └────────────────────────────────────────────────────┘│
│  Total: ~5,000 tokens (~$0.02 cloud cost)              │
│                                                          │
│  [Pin for next message]  [Clear all]                    │
└─────────────────────────────────────────────────────────┘
```

**Components**:
- `ContextTracker` (active context state)
- `TokenCounter` (estimate context size)
- `ContextDisplay` (visual representation)
- `CostEstimator` (cloud cost calculation)

**Key Files**:
- `internal/context/tracker.go`
- `internal/ui/components/context_indicator.go`

### 7.4 Plan Mode Architecture

```
┌─────────────────────────────────────────────────────────┐
│                         Plan                            │
│  "Refactor auth to use RBAC"                            │
│                                                          │
│  Generated Plan (5 steps):                              │
│                                                          │
│  1. [Read] Read current auth implementation             │
│     Files: auth/*.go                                    │
│     Estimated: 30s                                      │
│                                                          │
│  2. [Write] Create RBAC types and interfaces            │
│     Files: internal/security/rbac.go                    │
│     Estimated: 2m                                       │
│                                                          │
│  3. [Edit] Refactor auth to use RBAC                    │
│     Files: auth/*.go (5 files)                          │
│     Estimated: 5m                                       │
│                                                          │
│  4. [Bash] Run tests to verify changes                  │
│     Command: go test ./internal/security/...            │
│     Estimated: 1m                                       │
│                                                          │
│  5. [Bash] Commit changes                               │
│     Command: git commit -m "Refactor auth to use RBAC"  │
│     Estimated: 5s                                       │
│                                                          │
│  Total Estimated Time: ~8 minutes                       │
│                                                          │
│  [Approve] [Edit] [Cancel]                              │
└─────────────────────────────────────────────────────────┘
```

**Data Flow**:
```
User Input ("/plan <task>")
    ↓
PlanGenerator (LLM)
    ↓
Plan (struct with steps)
    ↓
User Approval UI
    ↓
PlanExecutor
    ↓
ToolExecutor (step by step)
    ↓
ProgressTracker (updates UI)
    ↓
Completion / Error
```

**Key Structures**:
```go
// Plan represents an execution plan
type Plan struct {
    ID          string
    Task        string
    Steps       []PlanStep
    Status      PlanStatus
    CurrentStep int
    CreatedAt   time.Time
    ApprovedAt  time.Time
    StartedAt   time.Time
    CompletedAt time.Time
}

// PlanStep is a single step
type PlanStep struct {
    ID          string
    Order       int
    Description string
    Tool        string
    Args        map[string]interface{}
    Estimate    time.Duration
    Status      StepStatus
    StartTime   time.Time
    EndTime     time.Time
    Result      *ToolResult
}

// PlanGenerator creates plans from task descriptions
type PlanGenerator struct {
    llm          ChatFunc
    toolRegistry *Registry
}

// GeneratePlan creates a plan for a task
func (pg *PlanGenerator) GeneratePlan(ctx context.Context, task string) (*Plan, error)

// PlanExecutor executes approved plans
type PlanExecutor struct {
    executor   *Executor
    tracker    *ProgressTracker
    onProgress func(step int, status string)
}

// Execute runs a plan step by step
func (pe *PlanExecutor) Execute(ctx context.Context, plan *Plan) error
```

### 7.5 Repository Map Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Repository Map                       │
│                                                          │
│  Structure:                                             │
│  ├─ cmd/                        (2 files)               │
│  ├─ internal/                   (87 files)              │
│  │  ├─ security/               (23 files)               │
│  │  │  ├─ auth.go              (15 functions)           │
│  │  │  ├─ rbac.go              (22 functions)           │
│  │  │  └─ audit.go             (18 functions)           │
│  │  ├─ tools/                  (12 files)               │
│  │  └─ ui/                     (32 files)               │
│  └─ main.go                     (3 functions)           │
│                                                          │
│  Recent Changes:                                        │
│  • internal/security/rbac.go (2 hours ago)              │
│  • internal/tools/executor.go (5 hours ago)             │
│                                                          │
│  Top Symbols:                                           │
│  • func NewExecutor(registry *Registry) *Executor       │
│  • type RBACEnforcer struct                             │
│  • func AuditLogEvent(sessionID, action string, ...)    │
└─────────────────────────────────────────────────────────┘
```

**Index Schema**:
```sql
CREATE TABLE files (
    id INTEGER PRIMARY KEY,
    path TEXT UNIQUE NOT NULL,
    size INTEGER,
    modified INTEGER,
    hash TEXT,
    indexed_at INTEGER
);

CREATE TABLE symbols (
    id INTEGER PRIMARY KEY,
    file_id INTEGER,
    name TEXT NOT NULL,
    type TEXT,  -- function, struct, interface, const, var
    line INTEGER,
    signature TEXT,
    doc TEXT,
    FOREIGN KEY (file_id) REFERENCES files(id)
);

CREATE VIRTUAL TABLE symbols_fts USING fts5(
    name, signature, doc,
    content='symbols',
    content_rowid='id'
);

CREATE TABLE imports (
    file_id INTEGER,
    import_path TEXT,
    FOREIGN KEY (file_id) REFERENCES files(id)
);
```

**Key Components**:
```go
// CodebaseIndex manages the repository index
type CodebaseIndex struct {
    db      *sql.DB
    root    string
    watcher *fsnotify.Watcher
    parser  *Parser
}

// Symbol represents a code symbol
type Symbol struct {
    Name      string
    Type      SymbolType
    File      string
    Line      int
    Signature string
    Doc       string
}

// Parser extracts symbols from source files
type Parser interface {
    Parse(path string) ([]Symbol, error)
}

// GoParser parses Go source files
type GoParser struct{}

func (p *GoParser) Parse(path string) ([]Symbol, error) {
    // Use go/parser and go/ast
}

// Search finds relevant symbols
func (idx *CodebaseIndex) Search(query string) ([]Symbol, error)

// Update incrementally updates the index
func (idx *CodebaseIndex) Update(path string) error

// Watch monitors file changes
func (idx *CodebaseIndex) Watch() error
```

**Index Build Performance**:
- Goal: <5s for 10,000 files
- Strategy: Parallel parsing (worker pool)
- Incremental updates: <100ms per file

### 7.6 Background Task System Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Background Tasks                                       │
│                                                          │
│  Running (1):                                           │
│  ⠿ Run full test suite                   [1m 23s]      │
│    └─ go test ./... -v                                  │
│                                                          │
│  Queued (2):                                            │
│  ○ Build release binary                                 │
│  ○ Generate documentation                               │
│                                                          │
│  Recent (3):                                            │
│  ✓ Lint codebase                        [45s] (2m ago)  │
│  ✗ Deploy to staging                    [1m 12s] (5m ago)│
│  ✓ Update dependencies                  [2m 3s] (10m ago)│
│                                                          │
│  [View Output] [Cancel All] [Clear History]             │
└─────────────────────────────────────────────────────────┘
```

**Architecture**:
```go
// TaskQueue manages background tasks
type TaskQueue struct {
    tasks    []*Task
    running  map[string]*Task
    maxRunning int
    notifier Notifier
    mu       sync.RWMutex
}

// Task represents a background task
type Task struct {
    ID          string
    Description string
    Type        TaskType // Bash, Tool, Plan
    Command     interface{}
    Status      TaskStatus
    Progress    float64
    StartTime   time.Time
    EndTime     time.Time
    Output      []byte
    Error       error
}

// Enqueue adds a task to the queue
func (q *TaskQueue) Enqueue(task *Task) error

// Run processes tasks from the queue
func (q *TaskQueue) Run(ctx context.Context)

// Cancel stops a running task
func (q *TaskQueue) Cancel(taskID string) error

// Notifier sends notifications
type Notifier interface {
    Notify(task *Task, event TaskEvent)
}

// TerminalNotifier shows in-TUI notifications
type TerminalNotifier struct {
    program *tea.Program
}

func (tn *TerminalNotifier) Notify(task *Task, event TaskEvent) {
    // Send Bubble Tea message
    tn.program.Send(TaskNotificationMsg{
        TaskID: task.ID,
        Event:  event,
    })
}
```

### 7.7 File Structure

**New Files** (to be created):

```
internal/
├── ui/
│   └── components/
│       ├── command_palette.go     # Ctrl+P fuzzy command search
│       ├── progress.go             # Multi-step progress indicators
│       ├── context_indicator.go   # Active context display
│       ├── onboarding.go          # Interactive tutorial
│       └── diff_viewer.go         # Inline diff display
│
├── commands/
│   ├── fuzzy.go                   # Fuzzy matching algorithm
│   ├── recent.go                  # Recent command tracking
│   └── palette.go                 # Command palette integration
│
├── context/
│   ├── tracker.go                 # Active context management
│   ├── cache.go                   # File read cache (LRU)
│   └── cost.go                    # Token/cost estimation
│
├── search/
│   ├── history.go                 # Ctrl+F in-conversation search
│   └── highlight.go               # Match highlighting
│
├── plan/
│   ├── generator.go               # Plan generation via LLM
│   ├── executor.go                # Plan execution engine
│   ├── types.go                   # Plan data structures
│   └── progress.go                # Plan progress tracking
│
├── index/
│   ├── codebase.go                # Repository indexing
│   ├── parser.go                  # AST parsing interface
│   ├── go_parser.go               # Go-specific parser
│   ├── watcher.go                 # File system watcher
│   └── schema.sql                 # SQLite schema
│
├── tasks/
│   ├── queue.go                   # Background task queue
│   ├── notifier.go                # Task notifications
│   └── types.go                   # Task data structures
│
└── telemetry/
    ├── dashboard.go               # Cost/usage dashboard
    ├── exporter.go                # Session export (MD/HTML)
    └── benchmark.go               # Model benchmarking
```

**Modified Files** (enhancements):

```
internal/
├── ui/
│   ├── chat/
│   │   ├── model.go               # + search mode, context tracking
│   │   ├── input.go               # + enhanced tab completion
│   │   └── help.go                # + progressive help
│   └── components/
│       ├── error.go               # + recovery suggestions
│       └── statusbar.go           # + context indicator
│
├── commands/
│   └── handlers.go                # + plan, search, export commands
│
├── context/
│   └── expander.go                # + cost estimation, caching
│
└── tools/
    └── executor.go                # + progress callbacks
```

---

## 8. Success Metrics

### 8.1 User Satisfaction Indicators

**Measurement**: Monthly user survey (NPS style)

| Metric | Current Baseline | Target (v1.0) | How to Measure |
|--------|------------------|---------------|----------------|
| **Net Promoter Score** | N/A (new) | 50+ | Survey: "How likely to recommend?" (0-10) |
| **Task Completion Rate** | N/A | 90%+ | Track slash command success vs errors |
| **Time to First Value** | N/A | <60s | Time from install to first successful query |
| **Daily Active Users** | N/A (new) | Track growth | Telemetry (opt-in) |
| **Retention (7-day)** | N/A | 60%+ | % users who return within 7 days |

**Qualitative**:
- GitHub stars: Target 1,000+ in 3 months
- Community feedback: Monitor Discord/Reddit sentiment
- Issue tracker: <5 open UX bugs at any time

### 8.2 Performance Benchmarks

**Measurement**: Automated benchmark suite

| Metric | Current | Target | How to Measure |
|--------|---------|--------|----------------|
| **Time to First Token** | ~500ms | <200ms | Median TTFT for local queries |
| **Command Palette Load** | N/A | <50ms | Time to open + render |
| **Tab Completion** | ~100ms | <50ms | Time to show completions |
| **History Search** | N/A | <100ms | Time to search 1000 messages |
| **@file Expansion (cached)** | ~200ms | <50ms | Time to expand cached file |
| **@codebase Index** | N/A | <5s | Time to index 10k files |
| **Memory Usage** | ~50MB | <100MB | RSS after 1hr session |
| **CPU Usage (idle)** | <1% | <1% | When waiting for input |

**Load Testing**:
- 1000+ message conversations: No degradation
- 100+ @file mentions: Caching prevents re-reads
- 10+ concurrent agentic loops: Stable performance

### 8.3 Feature Completeness Checklist

**Must-Have (v1.0)**:
- [ ] Command Palette (Ctrl+P) with fuzzy search
- [ ] Progressive Help (quick/all/contextual)
- [ ] File Read Cache (LRU with invalidation)
- [ ] History Search (Ctrl+F with highlighting)
- [ ] Error Recovery Suggestions
- [ ] Context Cost Estimates
- [ ] Progress Indicators (X of Y)
- [ ] Active Context Indicator
- [ ] Enhanced Tab Completion
- [ ] Onboarding Tutorial
- [ ] Plan Mode (basic)

**Should-Have (v1.1)**:
- [ ] Repository Map / Codebase Indexing
- [ ] Background Task System
- [ ] Smart Context Truncation
- [ ] Vim Mode
- [ ] Inline Diffs
- [ ] Cost Dashboard
- [ ] Model Benchmarking
- [ ] Session Export (MD/HTML)

**Nice-to-Have (v1.2+)**:
- [ ] Voice Input
- [ ] Multimodal (images)
- [ ] Collaboration (shared sessions)
- [ ] Plugin System
- [ ] Custom Themes

### 8.4 Quality Metrics

**Code Quality**:
- [ ] Test Coverage: 80%+ for new code
- [ ] Linting: 0 golangci-lint errors
- [ ] Documentation: All public APIs documented
- [ ] Examples: Each feature has usage example

**Security / Compliance**:
- [ ] IL5 Compliance: All controls passing
- [ ] Audit Coverage: 100% of security events logged
- [ ] Vulnerability Scan: 0 high/critical issues
- [ ] Dependency Audit: 0 known vulnerabilities

**User Experience**:
- [ ] Accessibility: Keyboard-only navigation works
- [ ] Error Messages: All errors have recovery suggestions
- [ ] Help Text: Every command has clear description
- [ ] Performance: No operation >500ms without progress indicator

### 8.5 Competitive Benchmarks

**Comparison to Claude Code**:
- [ ] Agentic Loop: Comparable safety limits
- [ ] Context Management: Richer @mention system
- [ ] Speed: Faster due to local-first architecture
- [ ] Compliance: Better (IL5 certified vs standard)

**Comparison to Aider**:
- [ ] Git Integration: Comparable workflow
- [ ] Repository Map: Similar indexing capability
- [ ] Model Support: Equal flexibility
- [ ] UX: More polished (command palette, progress)

**Comparison to Cursor**:
- [ ] Context Injection: Comparable @ system
- [ ] Indexing Scale: Smaller (10k vs 500k files) but sufficient
- [ ] Performance: Faster for small-medium codebases
- [ ] Compliance: Much better (Cursor not DoD-ready)

**Unique Strengths** (what makes rigrun the best):
1. Only IL5-compliant LLM TUI on the market
2. Best-in-class local-first architecture (privacy + speed)
3. Intelligent routing (cache -> local -> cloud)
4. Terminal-native with modern UX patterns
5. Comprehensive CLI for automation (50+ commands)

---

## Appendix A: Research Sources

### Claude Code
- [GitHub - anthropics/claude-code](https://github.com/anthropics/claude-code)
- [Claude Code: Best practices for agentic coding](https://www.anthropic.com/engineering/claude-code-best-practices)
- [Agentic CLI Tools Compared](https://research.aimultiple.com/agentic-cli/)

### Aider
- [Aider Documentation](https://aider.chat/docs/)
- [GitHub - Aider-AI/aider](https://github.com/Aider-AI/aider)
- [Aider Review: A Developer's Month](https://www.blott.com/blog/post/aider-review-a-developers-month-with-this-terminal-based-code-assistant)

### Cursor / Continue
- [Mastering Context Management in Cursor](https://stevekinney.com/courses/ai-development/cursor-context)
- [How Cursor Remembers Your Codebase](https://medium.com/@bswa006/how-cursor-remembers-your-codebase-so-you-dont-have-to-72c04583ccb7)

### Terminal TUI Best Practices
- [CLI UX Best Practices: Progress Displays](https://evilmartians.com/chronicles/cli-ux-best-practices-3-patterns-for-improving-progress-displays)
- [GitHub - rothgar/awesome-tuis](https://github.com/rothgar/awesome-tuis)
- [GitHub - gui-cs/Terminal.Gui](https://github.com/gui-cs/Terminal.Gui)

---

## Appendix B: Implementation Templates

### Command Palette Template

```go
// internal/ui/components/command_palette.go

package components

import (
    "strings"

    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"

    "github.com/jeranaias/rigrun-tui/internal/commands"
)

// CommandPalette is a fuzzy searchable command picker
type CommandPalette struct {
    visible    bool
    input      textinput.Model
    registry   *commands.Registry
    matches    []*commands.Command
    selected   int
    width      int
    height     int
}

// NewCommandPalette creates a new command palette
func NewCommandPalette(registry *commands.Registry) *CommandPalette {
    input := textinput.New()
    input.Placeholder = "Type to search commands..."
    input.Focus()

    return &CommandPalette{
        input:    input,
        registry: registry,
        matches:  registry.AllCommands(),
    }
}

// Show opens the palette
func (cp *CommandPalette) Show() {
    cp.visible = true
    cp.input.SetValue("")
    cp.selected = 0
    cp.updateMatches()
}

// Hide closes the palette
func (cp *CommandPalette) Hide() {
    cp.visible = false
}

// Update handles messages
func (cp *CommandPalette) Update(msg tea.Msg) (*CommandPalette, tea.Cmd) {
    if !cp.visible {
        return cp, nil
    }

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "esc":
            cp.Hide()
            return cp, nil
        case "enter":
            if cp.selected < len(cp.matches) {
                cmd := cp.matches[cp.selected]
                cp.Hide()
                return cp, func() tea.Msg {
                    return CommandSelectedMsg{Command: cmd}
                }
            }
        case "up", "ctrl+k":
            if cp.selected > 0 {
                cp.selected--
            }
        case "down", "ctrl+j":
            if cp.selected < len(cp.matches)-1 {
                cp.selected++
            }
        default:
            var cmd tea.Cmd
            cp.input, cmd = cp.input.Update(msg)
            cp.updateMatches()
            cp.selected = 0
            return cp, cmd
        }
    }

    return cp, nil
}

// updateMatches filters commands based on input
func (cp *CommandPalette) updateMatches() {
    query := cp.input.Value()
    if query == "" {
        cp.matches = cp.registry.AllCommands()
        return
    }

    cp.matches = fuzzyMatch(query, cp.registry.AllCommands())
}

// View renders the palette
func (cp *CommandPalette) View() string {
    if !cp.visible {
        return ""
    }

    // Implementation here...
    return ""
}

// CommandSelectedMsg is sent when a command is selected
type CommandSelectedMsg struct {
    Command *commands.Command
}
```

### Progress Indicator Template

```go
// internal/ui/components/progress.go

package components

import (
    "fmt"
    "strings"
    "time"

    "github.com/charmbracelet/lipgloss"
)

// ProgressTracker tracks multi-step progress
type ProgressTracker struct {
    steps       []ProgressStep
    currentStep int
    startTime   time.Time
}

// ProgressStep represents a single step
type ProgressStep struct {
    Description string
    Status      StepStatus
    StartTime   time.Time
    EndTime     time.Time
    Detail      string
}

type StepStatus int

const (
    StepPending StepStatus = iota
    StepRunning
    StepComplete
    StepFailed
)

// NewProgressTracker creates a progress tracker
func NewProgressTracker(steps []string) *ProgressTracker {
    progressSteps := make([]ProgressStep, len(steps))
    for i, desc := range steps {
        progressSteps[i] = ProgressStep{
            Description: desc,
            Status:      StepPending,
        }
    }

    return &ProgressTracker{
        steps:     progressSteps,
        startTime: time.Now(),
    }
}

// StartStep marks a step as running
func (pt *ProgressTracker) StartStep(index int) {
    if index >= 0 && index < len(pt.steps) {
        pt.steps[index].Status = StepRunning
        pt.steps[index].StartTime = time.Now()
        pt.currentStep = index
    }
}

// CompleteStep marks a step as complete
func (pt *ProgressTracker) CompleteStep(index int) {
    if index >= 0 && index < len(pt.steps) {
        pt.steps[index].Status = StepComplete
        pt.steps[index].EndTime = time.Now()
    }
}

// FailStep marks a step as failed
func (pt *ProgressTracker) FailStep(index int, detail string) {
    if index >= 0 && index < len(pt.steps) {
        pt.steps[index].Status = StepFailed
        pt.steps[index].EndTime = time.Now()
        pt.steps[index].Detail = detail
    }
}

// View renders the progress display
func (pt *ProgressTracker) View() string {
    var sb strings.Builder

    // Header
    completed := 0
    for _, step := range pt.steps {
        if step.Status == StepComplete {
            completed++
        }
    }

    sb.WriteString(fmt.Sprintf("Step %d of %d\n", pt.currentStep+1, len(pt.steps)))

    // Progress bar
    pct := float64(completed) / float64(len(pt.steps))
    barWidth := 40
    filled := int(pct * float64(barWidth))
    bar := strings.Repeat("▓", filled) + strings.Repeat("░", barWidth-filled)
    sb.WriteString(fmt.Sprintf("%s %.0f%%\n\n", bar, pct*100))

    // Steps
    for i, step := range pt.steps {
        icon := "○"
        color := lipgloss.Color("240")

        switch step.Status {
        case StepRunning:
            icon = "⠿"
            color = lipgloss.Color("12")
        case StepComplete:
            icon = "✓"
            color = lipgloss.Color("10")
        case StepFailed:
            icon = "✗"
            color = lipgloss.Color("9")
        }

        style := lipgloss.NewStyle().Foreground(color)
        sb.WriteString(style.Render(fmt.Sprintf("%s %s", icon, step.Description)))

        if step.Status == StepRunning && !step.StartTime.IsZero() {
            elapsed := time.Since(step.StartTime)
            sb.WriteString(fmt.Sprintf(" (%s)", formatDuration(elapsed)))
        }

        if step.Detail != "" {
            sb.WriteString(fmt.Sprintf("\n  %s", step.Detail))
        }

        sb.WriteString("\n")
    }

    return sb.String()
}
```

---

**End of Document**

This comprehensive plan provides a clear roadmap to make rigrun a world-class LLM TUI with exceptional user experience, best-in-class agentic capabilities, and uncompromising DoD IL5 compliance.
