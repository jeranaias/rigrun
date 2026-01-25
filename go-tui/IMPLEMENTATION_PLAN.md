# rigrun Go TUI - WORLD-CLASS Implementation Plan

## Executive Summary

Building the **most beautiful terminal UI** for rigrun using Go with Bubble Tea + Lip Gloss + Bubbles. This plan enables parallel development by multiple Sonnet agents across 8 implementation waves.

---

## Design Philosophy

- **BEAUTY FIRST** - Stunning, polished, professional aesthetics
- **Lip Gloss for ALL styling** - Gradients, borders, adaptive colors
- **Smooth animations everywhere** - Spinners, transitions, loading states
- **Modern, elegant color scheme** - Deep purples, cyans, emeralds on dark backgrounds

---

## Directory Structure

```
C:\rigrun\go-tui\
├── go.mod
├── go.sum
├── main.go                          # Entry point
├── cmd/
│   └── rigrun/
│       └── main.go                  # CLI entry with flags
├── internal/
│   ├── app/
│   │   ├── app.go                   # Main application model
│   │   ├── state.go                 # Application state machine
│   │   └── config.go                # Configuration loading
│   ├── ollama/
│   │   ├── client.go                # Ollama HTTP client
│   │   ├── streaming.go             # SSE streaming handler
│   │   └── types.go                 # API types
│   ├── model/
│   │   ├── message.go               # Message types (user/assistant/system/tool)
│   │   ├── conversation.go          # Conversation container
│   │   ├── session.go               # Session management
│   │   └── statistics.go            # Token stats, timing
│   ├── ui/
│   │   ├── styles/
│   │   │   ├── theme.go             # Master theme definition
│   │   │   ├── colors.go            # Color palette (adaptive)
│   │   │   ├── borders.go           # Border styles
│   │   │   └── animations.go        # Animation frame definitions
│   │   ├── components/
│   │   │   ├── header.go            # Title bar component
│   │   │   ├── viewport.go          # Chat viewport with scrolling
│   │   │   ├── message_bubble.go    # Styled message bubbles
│   │   │   ├── input.go             # Text input area
│   │   │   ├── status_bar.go        # Bottom status bar
│   │   │   ├── spinner.go           # Braille-style spinner
│   │   │   ├── progress.go          # Progress/context bar
│   │   │   ├── code_block.go        # Syntax-highlighted code
│   │   │   ├── command_palette.go   # Overlay command palette
│   │   │   ├── completion.go        # Tab completion popup
│   │   │   ├── permission.go        # Tool permission prompt
│   │   │   ├── welcome.go           # Welcome screen
│   │   │   └── error_box.go         # Styled error display
│   │   └── layout/
│   │       ├── adaptive.go          # Responsive layout logic
│   │       └── grid.go              # Grid system helpers
│   ├── commands/
│   │   ├── parser.go                # Slash command parsing
│   │   ├── registry.go              # Command registry
│   │   ├── handlers.go              # Command implementations
│   │   └── completion.go            # Tab completion logic
│   ├── context/
│   │   ├── mentions.go              # @ mention parsing
│   │   ├── file.go                  # @file: handler
│   │   ├── clipboard.go             # @clipboard handler
│   │   ├── git.go                   # @git handler
│   │   ├── codebase.go              # @codebase handler
│   │   └── error.go                 # @error handler
│   ├── session/
│   │   ├── store.go                 # Session persistence (JSON)
│   │   ├── autosave.go              # Auto-save functionality
│   │   └── timeout.go               # DoD compliance timeout
│   └── tools/
│       ├── executor.go              # Tool execution engine
│       ├── definitions.go           # Tool schemas
│       ├── read.go                  # Read file tool
│       ├── write.go                 # Write file tool
│       ├── edit.go                  # Edit file tool
│       ├── glob.go                  # Glob pattern tool
│       ├── grep.go                  # Grep search tool
│       └── bash.go                  # Bash execution tool
└── pkg/
    ├── markdown/
    │   └── renderer.go              # Markdown to terminal
    └── syntax/
        └── highlighter.go           # Code syntax highlighting
```

---

## Wave 1: Core Infrastructure

### Objective
Set up the project foundation with Go modules, main entry point, Ollama client, and Bubble Tea scaffolding.

### Files to Create

#### 1. `go.mod`
```go
module github.com/jeranaias/rigrun-tui

go 1.22

require (
    github.com/charmbracelet/bubbletea v0.25.0
    github.com/charmbracelet/lipgloss v0.9.1
    github.com/charmbracelet/bubbles v0.18.0
    github.com/muesli/termenv v0.15.2
    github.com/charmbracelet/glamour v0.6.0  // Markdown rendering
    github.com/alecthomas/chroma/v2 v2.12.0  // Syntax highlighting
)
```

#### 2. `main.go`
```go
package main

import (
    "github.com/jeranaias/rigrun-tui/internal/app"
    tea "github.com/charmbracelet/bubbletea"
)

func main() {
    p := tea.NewProgram(
        app.New(),
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )
    if _, err := p.Run(); err != nil {
        panic(err)
    }
}
```

#### 3. `internal/ollama/client.go`
```go
package ollama

// Client handles communication with Ollama API
type Client struct {
    BaseURL    string
    HTTPClient *http.Client
    Model      string
}

// Methods:
// - NewClient(baseURL string) *Client
// - Chat(ctx context.Context, messages []Message) (*Response, error)
// - ChatStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)
// - ListModels() ([]ModelInfo, error)
// - GenerateEmbedding(text string) ([]float32, error)
```

#### 4. `internal/ollama/streaming.go`
```go
package ollama

// StreamChunk represents a single SSE chunk from Ollama
type StreamChunk struct {
    Content     string
    Done        bool
    TokenCount  int
    Model       string
    Error       error
}

// StreamReader handles Server-Sent Events parsing
type StreamReader struct {
    reader *bufio.Reader
}
```

#### 5. `internal/ollama/types.go`
```go
package ollama

type Message struct {
    Role    string `json:"role"`    // "user", "assistant", "system"
    Content string `json:"content"`
}

type ChatRequest struct {
    Model    string    `json:"model"`
    Messages []Message `json:"messages"`
    Stream   bool      `json:"stream"`
    Options  *Options  `json:"options,omitempty"`
}

type ChatResponse struct {
    Model              string  `json:"model"`
    CreatedAt          string  `json:"created_at"`
    Message            Message `json:"message"`
    Done               bool    `json:"done"`
    TotalDuration      int64   `json:"total_duration"`
    LoadDuration       int64   `json:"load_duration"`
    PromptEvalCount    int     `json:"prompt_eval_count"`
    PromptEvalDuration int64   `json:"prompt_eval_duration"`
    EvalCount          int     `json:"eval_count"`
    EvalDuration       int64   `json:"eval_duration"`
}

type ModelInfo struct {
    Name       string `json:"name"`
    Size       int64  `json:"size"`
    ModifiedAt string `json:"modified_at"`
}
```

#### 6. `internal/app/app.go`
```go
package app

import (
    tea "github.com/charmbracelet/bubbletea"
)

// Model is the main Bubble Tea model
type Model struct {
    state        State
    ollama       *ollama.Client
    conversation *model.Conversation
    viewport     viewport.Model
    input        textinput.Model
    width        int
    height       int
    styles       *styles.Theme
}

// Bubble Tea interface:
// - Init() tea.Cmd
// - Update(msg tea.Msg) (tea.Model, tea.Cmd)
// - View() string
```

### Key Structs/Types

| Type | Purpose |
|------|---------|
| `app.Model` | Root Bubble Tea model |
| `app.State` | State machine enum (Welcome, Chat, Loading, etc.) |
| `ollama.Client` | HTTP client for Ollama API |
| `ollama.StreamChunk` | Individual streaming response token |

### Bubble Tea Messages

```go
// Messages for Wave 1
type WindowSizeMsg struct {
    Width, Height int
}

type OllamaResponseMsg struct {
    Chunk StreamChunk
}

type OllamaCompleteMsg struct {
    Stats Statistics
    Error error
}

type OllamaModelsMsg struct {
    Models []ModelInfo
    Error  error
}
```

### Lip Gloss Styles (Foundation)

```go
// internal/ui/styles/theme.go
type Theme struct {
    // Adaptive color detection
    IsDark bool

    // Base styles
    App       lipgloss.Style
    Container lipgloss.Style
}

// internal/ui/styles/colors.go
var (
    // Primary palette (adaptive)
    Purple     = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}
    Cyan       = lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#22D3EE"}
    Emerald    = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"}
    Rose       = lipgloss.AdaptiveColor{Light: "#E11D48", Dark: "#FB7185"}
    Amber      = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"}

    // Background tones
    Surface    = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1E1E2E"}
    SurfaceDim = lipgloss.AdaptiveColor{Light: "#F5F5F5", Dark: "#181825"}
    Overlay    = lipgloss.AdaptiveColor{Light: "#E5E5E5", Dark: "#313244"}

    // Text
    TextPrimary   = lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#CDD6F4"}
    TextSecondary = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#A6ADC8"}
    TextMuted     = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6C7086"}
)
```

### Dependencies
```
github.com/charmbracelet/bubbletea v0.25.0
github.com/charmbracelet/lipgloss v0.9.1
github.com/muesli/termenv v0.15.2
```

### Success Criteria
- [ ] `go build` succeeds
- [ ] App launches with blank screen
- [ ] Ollama client can list models
- [ ] Ctrl+C exits cleanly

---

## Wave 2: Chat Core

### Objective
Implement the core chat functionality with message models, streaming response display, and token-by-token rendering.

### Files to Create

#### 1. `internal/model/message.go`
```go
package model

import "time"

type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleSystem    Role = "system"
    RoleTool      Role = "tool"
)

type Message struct {
    ID        string
    Role      Role
    Content   string
    Timestamp time.Time

    // For assistant messages
    IsStreaming bool
    TokenCount  int

    // For tool messages
    ToolName   string
    ToolResult string
    IsSuccess  bool

    // Statistics
    TTFT           time.Duration // Time to first token
    TotalDuration  time.Duration
    TokensPerSec   float64
}
```

#### 2. `internal/model/conversation.go`
```go
package model

type Conversation struct {
    ID          string
    Title       string
    Messages    []Message
    Model       string
    CreatedAt   time.Time
    UpdatedAt   time.Time

    // Context tracking
    TokensUsed     int
    MaxTokens      int
    ContextPercent float64
}

// Methods:
// - AddMessage(msg Message)
// - GetHistory() []Message
// - ToOllamaMessages() []ollama.Message
// - EstimateTokens() int
```

#### 3. `internal/model/statistics.go`
```go
package model

type Statistics struct {
    // Timing
    StartTime       time.Time
    FirstTokenTime  time.Time
    EndTime         time.Time

    // Counts
    PromptTokens    int
    CompletionTokens int

    // Derived
    TTFT            time.Duration
    TotalDuration   time.Duration
    TokensPerSecond float64
}

// Methods:
// - RecordFirstToken()
// - RecordComplete(tokenCount int)
// - Format() string  // "2.5s | 128 tokens | 51 tok/s | TTFT 234ms"
```

### Bubble Tea Messages

```go
// Streaming messages
type StreamStartMsg struct {
    MessageID string
}

type StreamTokenMsg struct {
    MessageID string
    Token     string
    Stats     *Statistics
}

type StreamCompleteMsg struct {
    MessageID string
    Stats     Statistics
    Error     error
}
```

### Key Integration Points

```go
// In app.Update()
case tea.KeyMsg:
    if key.String() == "enter" && m.input.Value() != "" {
        // Create user message
        userMsg := model.NewUserMessage(m.input.Value())
        m.conversation.AddMessage(userMsg)

        // Clear input
        m.input.Reset()

        // Start streaming
        return m, m.startStreaming()
    }

case StreamTokenMsg:
    // Append token to current assistant message
    m.conversation.AppendToLast(msg.Token)
    // Update viewport to show new content
    return m, nil

case StreamCompleteMsg:
    // Finalize message with stats
    m.conversation.FinalizeLast(msg.Stats)
    m.state = StateReady
    return m, nil
```

### Lip Gloss Styles

```go
// Statistics bar style
var StatsBar = lipgloss.NewStyle().
    Foreground(TextMuted).
    BorderStyle(lipgloss.NormalBorder()).
    BorderTop(true).
    BorderForeground(Overlay).
    Padding(0, 1)

// Format: "2.5s | 128 tokens | 51 tok/s | TTFT 234ms"
func FormatStats(s Statistics) string {
    return StatsBar.Render(fmt.Sprintf(
        "%s | %d tokens | %.0f tok/s | TTFT %dms",
        s.TotalDuration.Round(time.Millisecond*100),
        s.CompletionTokens,
        s.TokensPerSecond,
        s.TTFT.Milliseconds(),
    ))
}
```

### Dependencies
- Wave 1 complete

### Success Criteria
- [ ] Can type a message and press Enter
- [ ] Response streams token-by-token
- [ ] Statistics display after completion
- [ ] Conversation history maintained

---

## Wave 3: Visual Components

### Objective
Build all the beautiful UI components: header, viewport, message bubbles, input area, and status bar.

### Files to Create

#### 1. `internal/ui/components/header.go`
```go
package components

import "github.com/charmbracelet/lipgloss"

type Header struct {
    Title    string
    Subtitle string
    Width    int
    styles   *styles.Theme
}

func (h Header) View() string {
    // Gradient title bar with brand styling
    // ┌─ rigrun ─────────────────────────────────┐
    // │ Local LLM Router | qwen2.5-coder:14b     │
    // └──────────────────────────────────────────┘
}
```

**Lip Gloss Style:**
```go
var HeaderStyle = lipgloss.NewStyle().
    Bold(true).
    Foreground(Cyan).
    Background(SurfaceDim).
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(Purple).
    Padding(0, 2).
    Align(lipgloss.Center)

var HeaderTitleStyle = lipgloss.NewStyle().
    Bold(true).
    Foreground(lipgloss.Color("#A78BFA")).  // Bright purple
    Background(lipgloss.Color("#1E1E2E"))

var HeaderSubtitleStyle = lipgloss.NewStyle().
    Foreground(TextSecondary).
    Italic(true)
```

#### 2. `internal/ui/components/viewport.go`
```go
package components

import (
    "github.com/charmbracelet/bubbles/viewport"
)

type ChatViewport struct {
    viewport viewport.Model
    messages []Message
    width    int
    height   int
    styles   *styles.Theme
}

// Methods:
// - SetMessages(msgs []Message)
// - ScrollToBottom()
// - ScrollUp(lines int)
// - ScrollDown(lines int)
// - View() string
```

#### 3. `internal/ui/components/message_bubble.go`
```go
package components

type MessageBubble struct {
    Message   *model.Message
    Width     int
    IsLatest  bool
    styles    *styles.Theme
}

func (b MessageBubble) View() string {
    switch b.Message.Role {
    case model.RoleUser:
        return b.renderUserBubble()
    case model.RoleAssistant:
        return b.renderAssistantBubble()
    case model.RoleSystem:
        return b.renderSystemBubble()
    case model.RoleTool:
        return b.renderToolBubble()
    }
}
```

**Lip Gloss Styles:**
```go
// User message bubble - Right aligned, blue tones
var UserBubbleStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("#E0F2FE")).  // Light blue text
    Background(lipgloss.Color("#1D4ED8")).  // Deep blue bg
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("#3B82F6")).
    Padding(0, 2).
    MarginLeft(4).
    Align(lipgloss.Right)

// Assistant message bubble - Left aligned, purple/violet tones
var AssistantBubbleStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("#F3E8FF")).  // Light purple text
    Background(lipgloss.Color("#4C1D95")).  // Deep purple bg
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("#7C3AED")).
    Padding(0, 2).
    MarginRight(4)

// System message - Amber/yellow tones, centered
var SystemBubbleStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("#FEF3C7")).  // Light amber text
    Background(lipgloss.Color("#78350F")).  // Deep amber bg
    BorderStyle(lipgloss.DoubleBorder()).
    BorderForeground(lipgloss.Color("#F59E0B")).
    Padding(0, 2).
    Align(lipgloss.Center).
    Width(60)

// Tool result bubble - Emerald tones for success, rose for error
var ToolSuccessBubbleStyle = lipgloss.NewStyle().
    Foreground(Emerald).
    BorderStyle(lipgloss.NormalBorder()).
    BorderForeground(Emerald).
    BorderLeft(true).
    PaddingLeft(2)

var ToolErrorBubbleStyle = lipgloss.NewStyle().
    Foreground(Rose).
    BorderStyle(lipgloss.NormalBorder()).
    BorderForeground(Rose).
    BorderLeft(true).
    PaddingLeft(2)
```

#### 4. `internal/ui/components/input.go`
```go
package components

import (
    "github.com/charmbracelet/bubbles/textinput"
)

type InputArea struct {
    input        textinput.Model
    charCount    int
    maxChars     int
    width        int
    placeholder  string
    styles       *styles.Theme
}

func (i InputArea) View() string {
    // ─────────────────────────────────────────────
    // > [cursor here]
    // ─────────────────────────────────────────────
    //   1,234 / 4,096 chars
}
```

**Lip Gloss Styles:**
```go
var InputContainerStyle = lipgloss.NewStyle().
    BorderStyle(lipgloss.NormalBorder()).
    BorderTop(true).
    BorderBottom(true).
    BorderForeground(Overlay).
    Padding(0, 1)

var InputPromptStyle = lipgloss.NewStyle().
    Foreground(Cyan).
    Bold(true)

var InputTextStyle = lipgloss.NewStyle().
    Foreground(TextPrimary)

var CharCountStyle = lipgloss.NewStyle().
    Foreground(TextMuted).
    Align(lipgloss.Right)

var CharCountWarningStyle = lipgloss.NewStyle().
    Foreground(Amber).
    Align(lipgloss.Right)

var CharCountDangerStyle = lipgloss.NewStyle().
    Foreground(Rose).
    Align(lipgloss.Right)
```

#### 5. `internal/ui/components/status_bar.go`
```go
package components

type StatusBar struct {
    Mode        string  // "local", "cloud", "hybrid"
    Model       string
    GPU         string
    ContextUsed int
    ContextMax  int
    Width       int
    styles      *styles.Theme
}

func (s StatusBar) View() string {
    // Adaptive based on width:
    // Narrow (<60):  [local|GPU] 45%
    // Medium (60-100): local | qwen2.5:14b | GPU | Ctx: 45% | /help
    // Wide (>100): Full box with shortcuts
}

func (s StatusBar) renderContextBar() string {
    // ████████░░ 45%
    percent := float64(s.ContextUsed) / float64(s.ContextMax) * 100
    filled := int(percent / 10)
    empty := 10 - filled

    color := Cyan
    if percent >= 90 {
        color = Rose
    } else if percent >= 75 {
        color = Amber
    }

    bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
    return lipgloss.NewStyle().Foreground(color).Render(bar) +
           lipgloss.NewStyle().Foreground(TextMuted).Render(fmt.Sprintf(" %d%%", int(percent)))
}
```

**Lip Gloss Styles:**
```go
var StatusBarStyle = lipgloss.NewStyle().
    Background(SurfaceDim).
    Foreground(TextSecondary).
    Padding(0, 1)

var StatusBarWideStyle = lipgloss.NewStyle().
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(Overlay).
    Padding(0, 2)

var ModeLocalStyle = lipgloss.NewStyle().
    Foreground(Emerald).
    Bold(true)

var ModeCloudStyle = lipgloss.NewStyle().
    Foreground(Amber).
    Bold(true)

var ModeHybridStyle = lipgloss.NewStyle().
    Foreground(Purple).
    Bold(true)

var GPUActiveStyle = lipgloss.NewStyle().
    Foreground(Emerald)

var ShortcutKeyStyle = lipgloss.NewStyle().
    Foreground(Cyan).
    Bold(true)

var ShortcutDescStyle = lipgloss.NewStyle().
    Foreground(TextMuted)
```

### Bubble Tea Messages

```go
type ViewportScrollMsg struct {
    Direction int // -1 up, 1 down
    Amount    int
}

type ResizeMsg struct {
    Width  int
    Height int
}
```

### Dependencies
- Wave 1, Wave 2 complete

### Success Criteria
- [ ] Header renders with gradient/border
- [ ] Messages display in styled bubbles (different colors per role)
- [ ] Input area shows character count
- [ ] Status bar adapts to terminal width
- [ ] Viewport scrolls smoothly

---

## Wave 4: Commands System

### Objective
Implement the slash command system with parsing, tab completion, command palette overlay, and all command handlers.

### Files to Create

#### 1. `internal/commands/registry.go`
```go
package commands

type Command struct {
    Name        string
    Aliases     []string
    Description string
    Usage       string
    Args        []ArgDef
    Handler     func(ctx *Context, args []string) tea.Cmd
}

type ArgDef struct {
    Name        string
    Required    bool
    Type        ArgType  // String, Model, File, etc.
    Completer   func() []string
}

type Registry struct {
    commands map[string]*Command
}

// All commands:
var AllCommands = []*Command{
    {Name: "/help", Aliases: []string{"/h", "/?"}, Description: "Show help"},
    {Name: "/new", Description: "Start new conversation"},
    {Name: "/save", Aliases: []string{"/s"}, Description: "Save conversation"},
    {Name: "/load", Args: []ArgDef{{Name: "id", Type: ArgTypeSession}}, Description: "Load conversation"},
    {Name: "/model", Aliases: []string{"/m"}, Args: []ArgDef{{Name: "name", Type: ArgTypeModel}}, Description: "Switch model"},
    {Name: "/mode", Args: []ArgDef{{Name: "mode", Type: ArgTypeEnum, Values: []string{"local", "cloud", "hybrid"}}}, Description: "Switch routing mode"},
    {Name: "/clear", Aliases: []string{"/c"}, Description: "Clear conversation"},
    {Name: "/copy", Description: "Copy last response"},
    {Name: "/export", Args: []ArgDef{{Name: "format", Type: ArgTypeEnum, Values: []string{"json", "md", "txt"}}}, Description: "Export conversation"},
    {Name: "/status", Description: "Show detailed status"},
    {Name: "/config", Description: "Show/edit config"},
    {Name: "/quit", Aliases: []string{"/q", "/exit"}, Description: "Exit rigrun"},
    // Tool commands
    {Name: "/tools", Description: "List available tools"},
    {Name: "/tool", Args: []ArgDef{{Name: "name", Type: ArgTypeTool}}, Description: "Enable/disable tool"},
}
```

#### 2. `internal/commands/parser.go`
```go
package commands

type ParseResult struct {
    IsCommand   bool
    Command     *Command
    Args        []string
    RawInput    string
    Error       error
}

func Parse(input string) ParseResult {
    if !strings.HasPrefix(input, "/") {
        return ParseResult{IsCommand: false, RawInput: input}
    }
    // Parse command and arguments
}

func (r *Registry) GetCompletions(partial string) []Completion {
    // Return matching commands for tab completion
}
```

#### 3. `internal/commands/completion.go`
```go
package commands

type Completion struct {
    Value       string
    Display     string
    Description string
    Score       int
}

type Completer struct {
    registry    *Registry
    ollama      *ollama.Client
    sessions    *session.Store
}

// Methods:
// - Complete(input string, cursorPos int) []Completion
// - CompleteCommand(partial string) []Completion
// - CompleteArg(cmd *Command, argIndex int, partial string) []Completion
```

#### 4. `internal/commands/handlers.go`
```go
package commands

// Context passed to command handlers
type Context struct {
    App          *app.Model
    Conversation *model.Conversation
    Ollama       *ollama.Client
    Sessions     *session.Store
}

// Example handlers:
func handleHelp(ctx *Context, args []string) tea.Cmd {
    return func() tea.Msg {
        return ShowHelpMsg{}
    }
}

func handleModel(ctx *Context, args []string) tea.Cmd {
    if len(args) == 0 {
        return showModelList(ctx)
    }
    return switchModel(ctx, args[0])
}

func handleSave(ctx *Context, args []string) tea.Cmd {
    return func() tea.Msg {
        id, err := ctx.Sessions.Save(ctx.Conversation)
        return SaveCompleteMsg{ID: id, Error: err}
    }
}
```

#### 5. `internal/ui/components/command_palette.go`
```go
package components

type CommandPalette struct {
    visible     bool
    input       textinput.Model
    commands    []commands.Completion
    selected    int
    width       int
    height      int
    styles      *styles.Theme
}

func (c *CommandPalette) View() string {
    // Overlay command palette
    // ┌─ Commands ────────────────────────────────┐
    // │ > /mo                                      │
    // │ ──────────────────────────────────────────│
    // │   /model      Switch model                │
    // │ > /mode       Switch routing mode         │ <- selected
    // │   /models     List available models       │
    // └───────────────────────────────────────────┘
}
```

**Lip Gloss Styles:**
```go
var PaletteOverlayStyle = lipgloss.NewStyle().
    Background(lipgloss.Color("#000000")).
    Foreground(TextPrimary)

var PaletteBoxStyle = lipgloss.NewStyle().
    Background(Surface).
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(Purple).
    Padding(1, 2).
    Width(60)

var PaletteItemStyle = lipgloss.NewStyle().
    Foreground(TextPrimary).
    Padding(0, 1)

var PaletteItemSelectedStyle = lipgloss.NewStyle().
    Background(Purple).
    Foreground(lipgloss.Color("#FFFFFF")).
    Bold(true).
    Padding(0, 1)

var PaletteCommandStyle = lipgloss.NewStyle().
    Foreground(Cyan).
    Width(12)

var PaletteDescStyle = lipgloss.NewStyle().
    Foreground(TextMuted)
```

#### 6. `internal/ui/components/completion.go`
```go
package components

type CompletionPopup struct {
    visible     bool
    completions []commands.Completion
    selected    int
    anchor      Position  // Where to anchor the popup
    styles      *styles.Theme
}

func (c *CompletionPopup) View() string {
    // Small popup above/below input
    // ┌────────────────────────┐
    // │ qwen2.5-coder:14b  ✓  │ <- current
    // │ qwen2.5-coder:7b      │
    // │ codestral:22b         │
    // └────────────────────────┘
}
```

### Bubble Tea Messages

```go
// Command system messages
type ExecuteCommandMsg struct {
    Command *Command
    Args    []string
}

type ShowCommandPaletteMsg struct{}
type HideCommandPaletteMsg struct{}

type ShowCompletionMsg struct {
    Completions []Completion
    Position    Position
}
type HideCompletionMsg struct{}
type SelectCompletionMsg struct {
    Index int
}

type SaveCompleteMsg struct {
    ID    string
    Error error
}

type LoadCompleteMsg struct {
    Conversation *model.Conversation
    Error        error
}

type ModelSwitchMsg struct {
    Model string
    Error error
}

type ShowHelpMsg struct{}
```

### Dependencies
- Waves 1-3 complete

### Success Criteria
- [ ] Typing `/` shows command list
- [ ] Tab completes partial commands
- [ ] Command palette opens with Ctrl+P
- [ ] All commands execute correctly
- [ ] Arguments complete contextually (models, sessions, etc.)

---

## Wave 5: File Mentions

### Objective
Implement the @ mention system for including context from files, clipboard, git, and codebase.

### Files to Create

#### 1. `internal/context/mentions.go`
```go
package context

type MentionType int

const (
    MentionFile MentionType = iota
    MentionClipboard
    MentionGit
    MentionCodebase
    MentionError
)

type Mention struct {
    Type    MentionType
    Raw     string      // Original text (e.g., "@file:src/main.go")
    Path    string      // For file mentions
    Range   string      // For git mentions (e.g., "HEAD~3")
    Content string      // Fetched content
    Error   error
}

// Parse extracts all @ mentions from input
// Returns (mentions, remaining text)
func Parse(input string) ([]Mention, string) {
    // Regex patterns:
    // @file:path/to/file
    // @file:"path with spaces"
    // @clipboard
    // @git
    // @git:HEAD~3
    // @codebase
    // @error
}
```

#### 2. `internal/context/file.go`
```go
package context

type FileContext struct {
    MaxFileSize int64  // Default 100KB
    MaxLines    int    // Default 1000
}

// Fetch reads file content with limits
func (f *FileContext) Fetch(path string) (string, error) {
    // 1. Resolve path (relative to cwd)
    // 2. Check file exists
    // 3. Check size limits
    // 4. Read content
    // 5. Format with line numbers (optional)
}

// Methods for file path completion
func (f *FileContext) Complete(partial string) []string {
    // Return matching file paths for tab completion
}
```

#### 3. `internal/context/clipboard.go`
```go
package context

import "golang.design/x/clipboard"

type ClipboardContext struct{}

func (c *ClipboardContext) Fetch() (string, error) {
    return clipboard.Read(clipboard.FmtText), nil
}
```

#### 4. `internal/context/git.go`
```go
package context

type GitContext struct {
    RepoRoot string
}

func (g *GitContext) Fetch(gitRange string) (string, error) {
    if gitRange == "" {
        // Default: recent commits + uncommitted changes
        return g.fetchDefaultContext()
    }
    // Specific range: git log + diff
    return g.fetchRange(gitRange)
}

func (g *GitContext) fetchDefaultContext() (string, error) {
    // 1. git log --oneline -10
    // 2. git diff --stat
    // 3. git status --short
}
```

#### 5. `internal/context/codebase.go`
```go
package context

type CodebaseContext struct {
    Root       string
    MaxDepth   int
    MaxFiles   int
    Ignores    []string  // .gitignore patterns
}

func (c *CodebaseContext) Fetch() (string, error) {
    // Generate codebase summary:
    // 1. Directory tree structure
    // 2. File counts by type
    // 3. Key files (README, main entry points)
    // 4. Recent changes
}
```

#### 6. `internal/context/error.go`
```go
package context

var lastError string

func StoreError(err string) {
    lastError = err
}

func FetchLastError() (string, error) {
    if lastError == "" {
        return "", fmt.Errorf("no recent error stored")
    }
    return lastError, nil
}
```

### Integration with Input

```go
// In input component, detect @ mentions for:
// 1. Syntax highlighting
// 2. Tab completion
// 3. Preview tooltip

func (i *InputArea) highlightMentions(text string) string {
    // Highlight @file:, @clipboard, etc. with special color
    mentionStyle := lipgloss.NewStyle().
        Foreground(Cyan).
        Bold(true)
    // Apply to mention spans
}
```

### Context Display Component

```go
// internal/ui/components/context_preview.go
type ContextPreview struct {
    mentions []context.Mention
    visible  bool
    styles   *styles.Theme
}

func (c *ContextPreview) View() string {
    // Show fetched context in collapsible sections
    // ┌─ Context ─────────────────────────────────┐
    // │ @file:src/main.go (234 lines)        [-] │
    // │ @git (3 commits, +45/-12)             [+] │
    // └───────────────────────────────────────────┘
}
```

**Lip Gloss Styles:**
```go
var MentionStyle = lipgloss.NewStyle().
    Foreground(Cyan).
    Bold(true)

var ContextPreviewStyle = lipgloss.NewStyle().
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(Overlay).
    Padding(0, 1)

var ContextHeaderStyle = lipgloss.NewStyle().
    Foreground(TextSecondary).
    Bold(true)

var ContextContentStyle = lipgloss.NewStyle().
    Foreground(TextMuted).
    MaxHeight(10)  // Collapsed view
```

### Bubble Tea Messages

```go
type FetchContextMsg struct {
    Mentions []Mention
}

type ContextFetchedMsg struct {
    Mention Mention
    Content string
    Error   error
}

type ToggleContextPreviewMsg struct {
    MentionIndex int
}
```

### Dependencies
- Waves 1-4 complete
- `golang.design/x/clipboard` for clipboard access

### Success Criteria
- [ ] `@file:path` inserts file content
- [ ] `@clipboard` inserts clipboard content
- [ ] `@git` inserts git context
- [ ] `@codebase` generates codebase summary
- [ ] Tab completes file paths
- [ ] Context preview shows fetched content

---

## Wave 6: Session Management

### Objective
Implement conversation persistence with save/load, auto-save, session listing, and DoD compliance timeout.

### Files to Create

#### 1. `internal/session/store.go`
```go
package session

import (
    "encoding/json"
    "path/filepath"
)

type Store struct {
    BaseDir    string  // ~/.rigrun/sessions/
    MaxSessions int    // Default 100
}

type SessionMeta struct {
    ID        string
    Title     string
    Model     string
    CreatedAt time.Time
    UpdatedAt time.Time
    MessageCount int
    Preview   string  // First user message truncated
}

// Methods:
// - Save(conv *model.Conversation) (string, error)
// - Load(id string) (*model.Conversation, error)
// - List() ([]SessionMeta, error)
// - Delete(id string) error
// - Search(query string) ([]SessionMeta, error)

// File format: ~/.rigrun/sessions/{id}.json
```

**Session JSON Schema:**
```json
{
    "id": "sess_abc123",
    "title": "Rust error handling",
    "model": "qwen2.5-coder:14b",
    "created_at": "2025-01-18T10:30:00Z",
    "updated_at": "2025-01-18T11:45:00Z",
    "messages": [
        {
            "role": "user",
            "content": "How do I handle errors in Rust?",
            "timestamp": "2025-01-18T10:30:00Z"
        },
        {
            "role": "assistant",
            "content": "In Rust, error handling is done through...",
            "timestamp": "2025-01-18T10:30:05Z",
            "token_count": 256,
            "duration_ms": 2500
        }
    ],
    "context": {
        "tokens_used": 1024,
        "mentions": ["@file:src/main.rs"]
    }
}
```

#### 2. `internal/session/autosave.go`
```go
package session

type AutoSaver struct {
    store      *Store
    interval   time.Duration  // Default 30 seconds
    enabled    bool
    lastSave   time.Time
    dirty      bool
}

// Methods:
// - Start(conv *model.Conversation)
// - Stop()
// - MarkDirty()
// - SaveNow() error
```

#### 3. `internal/session/timeout.go`
```go
package session

// DoD compliance: sessions must timeout after inactivity
type TimeoutManager struct {
    timeout     time.Duration  // Default 30 minutes
    lastActivity time.Time
    callback    func()  // Called when timeout occurs
}

// Methods:
// - RecordActivity()
// - Start()
// - Stop()
// - IsExpired() bool
```

### Session List Component

```go
// internal/ui/components/session_list.go
type SessionList struct {
    sessions   []SessionMeta
    selected   int
    filter     string
    styles     *styles.Theme
}

func (s *SessionList) View() string {
    // ┌─ Sessions ────────────────────────────────┐
    // │ > Search: rust error                      │
    // │ ──────────────────────────────────────────│
    // │   sess_abc123  "Rust error handling"      │
    // │               qwen2.5:14b | Jan 18, 11:45 │
    // │ > sess_def456  "Go TUI implementation"    │ <- selected
    // │               codestral:22b | Jan 17, 9:30│
    // │   sess_ghi789  "Docker optimization"      │
    // │               qwen2.5:7b | Jan 16, 14:20  │
    // └───────────────────────────────────────────┘
}
```

**Lip Gloss Styles:**
```go
var SessionListStyle = lipgloss.NewStyle().
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(Purple).
    Padding(1, 2)

var SessionItemStyle = lipgloss.NewStyle().
    Foreground(TextPrimary).
    Padding(0, 1)

var SessionItemSelectedStyle = lipgloss.NewStyle().
    Background(Purple).
    Foreground(lipgloss.Color("#FFFFFF")).
    Bold(true).
    Padding(0, 1)

var SessionIDStyle = lipgloss.NewStyle().
    Foreground(TextMuted).
    Width(12)

var SessionTitleStyle = lipgloss.NewStyle().
    Foreground(Cyan).
    Bold(true)

var SessionMetaStyle = lipgloss.NewStyle().
    Foreground(TextMuted).
    Italic(true)
```

### Bubble Tea Messages

```go
type SessionSavedMsg struct {
    ID    string
    Error error
}

type SessionLoadedMsg struct {
    Session *model.Conversation
    Error   error
}

type SessionListMsg struct {
    Sessions []SessionMeta
    Error    error
}

type SessionDeletedMsg struct {
    ID    string
    Error error
}

type SessionTimeoutMsg struct{}

type AutoSaveTriggeredMsg struct{}
```

### Dependencies
- Waves 1-5 complete

### Success Criteria
- [ ] `/save` persists conversation to JSON
- [ ] `/load <id>` restores conversation
- [ ] Session list shows all saved sessions
- [ ] Auto-save triggers every 30 seconds
- [ ] Timeout warning appears after 25 minutes
- [ ] Session locks after 30 minutes inactivity

---

## Wave 7: Tool System

### Objective
Implement the agentic tool system with tool definitions, permission prompts, execution, and result rendering.

### Files to Create

#### 1. `internal/tools/definitions.go`
```go
package tools

type Tool struct {
    Name        string
    Description string
    Schema      ToolSchema
    Executor    ToolExecutor
    Permission  PermissionLevel
}

type ToolSchema struct {
    Parameters []Parameter
}

type Parameter struct {
    Name        string
    Type        string  // "string", "boolean", "number", "array"
    Required    bool
    Description string
}

type PermissionLevel int

const (
    PermissionAuto PermissionLevel = iota   // Always allowed
    PermissionAsk                            // Prompt user
    PermissionNever                          // Never allowed
)

// All available tools
var AllTools = []*Tool{
    ReadFileTool,
    WriteFileTool,
    EditFileTool,
    GlobTool,
    GrepTool,
    BashTool,
}
```

#### 2. `internal/tools/executor.go`
```go
package tools

type ToolExecutor interface {
    Execute(ctx context.Context, params map[string]interface{}) (ToolResult, error)
}

type ToolResult struct {
    Success bool
    Output  string
    Error   string
    Timing  time.Duration
}

type ExecutionContext struct {
    WorkDir     string
    AllowWrite  bool
    AllowBash   bool
    Timeout     time.Duration
}
```

#### 3. `internal/tools/read.go`
```go
package tools

var ReadFileTool = &Tool{
    Name:        "Read",
    Description: "Read file contents from the filesystem",
    Schema: ToolSchema{
        Parameters: []Parameter{
            {Name: "file_path", Type: "string", Required: true, Description: "Absolute path to read"},
            {Name: "offset", Type: "number", Required: false, Description: "Line offset to start from"},
            {Name: "limit", Type: "number", Required: false, Description: "Number of lines to read"},
        },
    },
    Permission: PermissionAuto,
    Executor:   &ReadExecutor{},
}

type ReadExecutor struct{}

func (r *ReadExecutor) Execute(ctx context.Context, params map[string]interface{}) (ToolResult, error) {
    path := params["file_path"].(string)
    // Validate path, read file, return content
}
```

#### 4. `internal/tools/write.go`
```go
package tools

var WriteFileTool = &Tool{
    Name:        "Write",
    Description: "Write content to a file",
    Schema: ToolSchema{
        Parameters: []Parameter{
            {Name: "file_path", Type: "string", Required: true},
            {Name: "content", Type: "string", Required: true},
        },
    },
    Permission: PermissionAsk,  // Requires user confirmation
    Executor:   &WriteExecutor{},
}
```

#### 5. `internal/tools/edit.go`
```go
package tools

var EditFileTool = &Tool{
    Name:        "Edit",
    Description: "Edit file using search and replace",
    Schema: ToolSchema{
        Parameters: []Parameter{
            {Name: "file_path", Type: "string", Required: true},
            {Name: "old_string", Type: "string", Required: true},
            {Name: "new_string", Type: "string", Required: true},
            {Name: "replace_all", Type: "boolean", Required: false},
        },
    },
    Permission: PermissionAsk,
    Executor:   &EditExecutor{},
}
```

#### 6. `internal/tools/glob.go`
```go
package tools

var GlobTool = &Tool{
    Name:        "Glob",
    Description: "Find files matching a pattern",
    Schema: ToolSchema{
        Parameters: []Parameter{
            {Name: "pattern", Type: "string", Required: true},
            {Name: "path", Type: "string", Required: false},
        },
    },
    Permission: PermissionAuto,
    Executor:   &GlobExecutor{},
}
```

#### 7. `internal/tools/grep.go`
```go
package tools

var GrepTool = &Tool{
    Name:        "Grep",
    Description: "Search file contents with regex",
    Schema: ToolSchema{
        Parameters: []Parameter{
            {Name: "pattern", Type: "string", Required: true},
            {Name: "path", Type: "string", Required: false},
            {Name: "glob", Type: "string", Required: false},
            {Name: "output_mode", Type: "string", Required: false},  // content, files_with_matches, count
        },
    },
    Permission: PermissionAuto,
    Executor:   &GrepExecutor{},
}
```

#### 8. `internal/tools/bash.go`
```go
package tools

var BashTool = &Tool{
    Name:        "Bash",
    Description: "Execute shell commands",
    Schema: ToolSchema{
        Parameters: []Parameter{
            {Name: "command", Type: "string", Required: true},
            {Name: "timeout", Type: "number", Required: false},
            {Name: "description", Type: "string", Required: false},
        },
    },
    Permission: PermissionAsk,  // Always ask for shell commands
    Executor:   &BashExecutor{},
}
```

### Permission Prompt Component

```go
// internal/ui/components/permission.go
type PermissionPrompt struct {
    tool       *Tool
    params     map[string]interface{}
    visible    bool
    selected   int  // 0=Allow, 1=Allow Always, 2=Deny
    styles     *styles.Theme
}

func (p *PermissionPrompt) View() string {
    // ┌─ Tool Request ────────────────────────────┐
    // │                                           │
    // │  Bash wants to execute:                   │
    // │                                           │
    // │  ┌───────────────────────────────────────│
    // │  │ npm install                           ││
    // │  └───────────────────────────────────────│
    // │                                           │
    // │  Description: Install dependencies       │
    // │                                           │
    // │  [Allow]  [Allow Always]  [Deny]         │
    // └───────────────────────────────────────────┘
}
```

**Lip Gloss Styles:**
```go
var PermissionPromptStyle = lipgloss.NewStyle().
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(Amber).
    Background(Surface).
    Padding(1, 2).
    Width(50)

var PermissionTitleStyle = lipgloss.NewStyle().
    Foreground(Amber).
    Bold(true)

var PermissionCommandStyle = lipgloss.NewStyle().
    Background(SurfaceDim).
    Foreground(TextPrimary).
    Padding(0, 1)

var PermissionButtonStyle = lipgloss.NewStyle().
    Foreground(TextPrimary).
    Background(Overlay).
    Padding(0, 2).
    MarginRight(1)

var PermissionButtonActiveStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("#FFFFFF")).
    Background(Purple).
    Bold(true).
    Padding(0, 2).
    MarginRight(1)
```

### Tool Result Component

```go
// internal/ui/components/tool_result.go
type ToolResultView struct {
    tool    *Tool
    result  ToolResult
    expanded bool
    styles  *styles.Theme
}

func (t *ToolResultView) View() string {
    // Collapsed:
    // ✓ Read src/main.go (234 lines, 12ms)
    //
    // Expanded:
    // ┌─ Read: src/main.go ────────────────────────┐
    // │ 1│ package main                            │
    // │ 2│                                         │
    // │ 3│ import (                                │
    // │ ...                                        │
    // │ 234│ }                                     │
    // └──────────────────────────────── 12ms ──────┘
}
```

### Bubble Tea Messages

```go
type ToolRequestMsg struct {
    Tool   *Tool
    Params map[string]interface{}
}

type ToolPermissionResponseMsg struct {
    Tool     *Tool
    Allowed  bool
    Remember bool  // "Allow Always"
}

type ToolExecutingMsg struct {
    Tool *Tool
}

type ToolCompleteMsg struct {
    Tool   *Tool
    Result ToolResult
}
```

### Dependencies
- Waves 1-6 complete
- OS package for file operations
- os/exec for bash execution

### Success Criteria
- [ ] Tools are invoked by LLM responses
- [ ] Permission prompt appears for write/bash
- [ ] Tool results display beautifully
- [ ] "Allow Always" remembers preference
- [ ] Tool execution shows timing

---

## Wave 8: Polish & Animations

### Objective
Add the finishing touches: spinners, loading states, smooth transitions, error displays, code highlighting, and welcome screen.

### Files to Create

#### 1. `internal/ui/styles/animations.go`
```go
package styles

// Braille spinner frames (smooth, professional)
var SpinnerFrames = []string{
    "⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
}

// Dot animation frames
var DotFrames = []string{
    "·  ", "·· ", "···", " ··", "  ·", "   ",
}

// Progress bar characters
var ProgressFull = "█"
var ProgressEmpty = "░"
var ProgressPartial = []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉"}

// Fade animation (for transitions)
var FadeChars = []string{"░", "▒", "▓", "█"}
```

#### 2. `internal/ui/components/spinner.go`
```go
package components

import (
    "github.com/charmbracelet/bubbles/spinner"
)

type ThinkingSpinner struct {
    spinner     spinner.Model
    message     string
    elapsed     time.Duration
    showDetails bool
    details     []string  // Context, model, etc.
    styles      *styles.Theme
}

func (t *ThinkingSpinner) View() string {
    // Basic:
    // ⠋ Thinking ··· 2.3s
    //
    // With details (Ctrl+D):
    // ⠋ Thinking ··· 2.3s
    //   ├─ Processing query...
    //   ├─ Context: 2,048 tokens
    //   └─ Model: qwen2.5-coder:14b
}

func NewThinkingSpinner() ThinkingSpinner {
    s := spinner.New()
    s.Spinner = spinner.Spinner{
        Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
        FPS:    time.Second / 12,
    }
    s.Style = lipgloss.NewStyle().Foreground(Purple)
    return ThinkingSpinner{spinner: s}
}
```

**Lip Gloss Styles:**
```go
var SpinnerStyle = lipgloss.NewStyle().
    Foreground(Purple)

var ThinkingTextStyle = lipgloss.NewStyle().
    Foreground(TextSecondary)

var ThinkingDotsStyle = lipgloss.NewStyle().
    Foreground(Purple)

var ThinkingTimeStyle = lipgloss.NewStyle().
    Foreground(TextMuted)

var ThinkingDetailStyle = lipgloss.NewStyle().
    Foreground(TextMuted).
    PaddingLeft(2)

var ThinkingDetailTreeStyle = lipgloss.NewStyle().
    Foreground(Overlay)
```

#### 3. `internal/ui/components/error_box.go`
```go
package components

type ErrorBox struct {
    title       string
    message     string
    suggestions []string
    width       int
    styles      *styles.Theme
}

func (e *ErrorBox) View() string {
    // ┌─ Error ─────────────────────────────────────┐
    // │ ✗ Model 'unknown-model' not found           │
    // │                                             │
    // │   Available models:                         │
    // │     • qwen2.5-coder:14b                    │
    // │     • codestral:22b                        │
    // │                                             │
    // │   Tip: ollama pull unknown-model           │
    // └─────────────────────────────────────────────┘
}
```

**Lip Gloss Styles:**
```go
var ErrorBoxStyle = lipgloss.NewStyle().
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(Rose).
    Padding(1, 2)

var ErrorTitleStyle = lipgloss.NewStyle().
    Foreground(Rose).
    Bold(true)

var ErrorMessageStyle = lipgloss.NewStyle().
    Foreground(TextPrimary)

var ErrorSuggestionStyle = lipgloss.NewStyle().
    Foreground(TextSecondary).
    PaddingLeft(2)

var ErrorTipStyle = lipgloss.NewStyle().
    Foreground(Cyan).
    Italic(true)
```

#### 4. `internal/ui/components/code_block.go`
```go
package components

import (
    "github.com/alecthomas/chroma/v2"
    "github.com/alecthomas/chroma/v2/lexers"
    "github.com/alecthomas/chroma/v2/styles"
)

type CodeBlock struct {
    code      string
    language  string
    showCopy  bool
    width     int
    maxHeight int
    styles    *styles.Theme
}

func (c *CodeBlock) View() string {
    // ```rust
    // fn main() {
    //     println!("Hello, world!");
    // }
    // ```                              [Copy]
}

func (c *CodeBlock) highlight() string {
    lexer := lexers.Get(c.language)
    if lexer == nil {
        lexer = lexers.Fallback
    }
    // Apply syntax highlighting
}
```

**Lip Gloss Styles:**
```go
var CodeBlockStyle = lipgloss.NewStyle().
    Background(lipgloss.Color("#1E1E2E")).
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(Overlay).
    Padding(1, 2)

var CodeLangBadgeStyle = lipgloss.NewStyle().
    Foreground(TextMuted).
    Background(Overlay).
    Padding(0, 1).
    Bold(true)

var CodeCopyButtonStyle = lipgloss.NewStyle().
    Foreground(Cyan).
    Background(Overlay).
    Padding(0, 1)

// Syntax highlighting colors (Catppuccin Mocha inspired)
var SyntaxKeyword = lipgloss.NewStyle().Foreground(lipgloss.Color("#CBA6F7"))   // Mauve
var SyntaxString = lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1"))    // Green
var SyntaxNumber = lipgloss.NewStyle().Foreground(lipgloss.Color("#FAB387"))    // Peach
var SyntaxComment = lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))   // Overlay0
var SyntaxFunction = lipgloss.NewStyle().Foreground(lipgloss.Color("#89B4FA"))  // Blue
var SyntaxType = lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF"))      // Yellow
```

#### 5. `internal/ui/components/welcome.go`
```go
package components

type WelcomeScreen struct {
    width     int
    height    int
    model     string
    gpu       string
    version   string
    styles    *styles.Theme
}

func (w *WelcomeScreen) View() string {
    // ASCII art logo + quick start info
    /*
    ┌─────────────────────────────────────────────────────────────┐
    │                                                             │
    │           ██████╗ ██╗ ██████╗ ██████╗ ██╗   ██╗███╗   ██╗   │
    │           ██╔══██╗██║██╔════╝ ██╔══██╗██║   ██║████╗  ██║   │
    │           ██████╔╝██║██║  ███╗██████╔╝██║   ██║██╔██╗ ██║   │
    │           ██╔══██╗██║██║   ██║██╔══██╗██║   ██║██║╚██╗██║   │
    │           ██║  ██║██║╚██████╔╝██║  ██║╚██████╔╝██║ ╚████║   │
    │           ╚═╝  ╚═╝╚═╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝   │
    │                                                             │
    │                    Local LLM Router v0.2.0                  │
    │                                                             │
    │   Model: qwen2.5-coder:14b                                  │
    │   GPU:   AMD RX 7900 XTX (24GB VRAM)                        │
    │   Mode:  Local                                              │
    │                                                             │
    │   Quick Start:                                              │
    │     • Type a message and press Enter                        │
    │     • Use /help to see all commands                         │
    │     • Use @file:path to include file context                │
    │     • Press Ctrl+C to stop generation                       │
    │                                                             │
    │                  Press any key to continue...               │
    └─────────────────────────────────────────────────────────────┘
    */
}
```

**Lip Gloss Styles:**
```go
var WelcomeBoxStyle = lipgloss.NewStyle().
    BorderStyle(lipgloss.DoubleBorder()).
    BorderForeground(Purple).
    Padding(2, 4).
    Align(lipgloss.Center)

var WelcomeLogoStyle = lipgloss.NewStyle().
    Foreground(Cyan).
    Bold(true)

var WelcomeVersionStyle = lipgloss.NewStyle().
    Foreground(TextMuted).
    Italic(true)

var WelcomeInfoStyle = lipgloss.NewStyle().
    Foreground(TextSecondary)

var WelcomeKeyStyle = lipgloss.NewStyle().
    Foreground(Cyan).
    Bold(true)

var WelcomePressKeyStyle = lipgloss.NewStyle().
    Foreground(Purple).
    Blink(true)
```

#### 6. `pkg/markdown/renderer.go`
```go
package markdown

import (
    "github.com/charmbracelet/glamour"
)

type Renderer struct {
    glamour *glamour.TermRenderer
    width   int
}

func NewRenderer(width int) *Renderer {
    r, _ := glamour.NewTermRenderer(
        glamour.WithAutoStyle(),
        glamour.WithWordWrap(width),
    )
    return &Renderer{glamour: r, width: width}
}

func (r *Renderer) Render(markdown string) string {
    out, _ := r.glamour.Render(markdown)
    return out
}
```

### Transition Animations

```go
// Smooth state transitions
type TransitionType int

const (
    TransitionFade TransitionType = iota
    TransitionSlide
    TransitionNone
)

type Transition struct {
    from       State
    to         State
    ttype      TransitionType
    progress   float64
    duration   time.Duration
}

func (t *Transition) Tick() tea.Msg {
    return TransitionTickMsg{Progress: t.progress}
}
```

### Bubble Tea Messages

```go
type SpinnerTickMsg struct{}

type TransitionStartMsg struct {
    To State
}

type TransitionTickMsg struct {
    Progress float64
}

type TransitionCompleteMsg struct{}

type ShowWelcomeMsg struct{}
type DismissWelcomeMsg struct{}

type ToggleDetailsMsg struct{}

type CopyToClipboardMsg struct {
    Content string
}

type CopyCompleteMsg struct {
    Success bool
}
```

### Dependencies
- All previous waves complete
- `github.com/charmbracelet/glamour` for markdown
- `github.com/alecthomas/chroma/v2` for syntax highlighting

### Success Criteria
- [ ] Thinking spinner animates smoothly
- [ ] Code blocks have syntax highlighting
- [ ] Welcome screen displays on first launch
- [ ] Error messages are styled and helpful
- [ ] Transitions between states feel smooth
- [ ] Copy button works for code blocks

---

## Complete Dependency Graph

```
Wave 1 ─────────────────────────────────────────────────┐
   │                                                    │
   └─> Wave 2 ──────────────────────────────────────────┤
          │                                             │
          └─> Wave 3 ───────────────────────────────────┤
                 │                                      │
                 ├─> Wave 4 (Commands) ─────────────────┤
                 │                                      │
                 └─> Wave 5 (Context) ──────────────────┤
                        │                               │
                        └─> Wave 6 (Sessions) ──────────┤
                               │                        │
                               └─> Wave 7 (Tools) ──────┤
                                      │                 │
                                      └─> Wave 8 ───────┘
```

**Parallelization Opportunities:**
- Wave 4 and Wave 5 can run in parallel after Wave 3
- Multiple components within each wave can be developed simultaneously
- Tests can be written in parallel with implementation

---

## Estimated LOC per Wave

| Wave | Files | Estimated LOC |
|------|-------|---------------|
| Wave 1 | 6 | ~600 |
| Wave 2 | 3 | ~400 |
| Wave 3 | 5 | ~800 |
| Wave 4 | 6 | ~700 |
| Wave 5 | 6 | ~500 |
| Wave 6 | 3 | ~400 |
| Wave 7 | 8 | ~900 |
| Wave 8 | 6 | ~700 |
| **Total** | **43** | **~5,000** |

---

## Testing Strategy

### Unit Tests (per wave)
```
internal/ollama/*_test.go       - API client tests
internal/model/*_test.go        - Data model tests
internal/commands/*_test.go     - Command parsing tests
internal/context/*_test.go      - Mention parsing tests
internal/session/*_test.go      - Persistence tests
internal/tools/*_test.go        - Tool execution tests
```

### Integration Tests
```
tests/integration/chat_test.go      - Full chat flow
tests/integration/commands_test.go  - Command execution
tests/integration/tools_test.go     - Tool system
```

### Visual Tests
```
tests/visual/components_test.go  - Golden file snapshots
```

---

## Color Reference (Quick Lookup)

| Name | Hex (Dark) | Hex (Light) | Usage |
|------|------------|-------------|-------|
| Purple | #A78BFA | #7C3AED | Primary accent, assistant |
| Cyan | #22D3EE | #0891B2 | Brand, info, commands |
| Emerald | #34D399 | #059669 | Success, local mode |
| Rose | #FB7185 | #E11D48 | Errors, critical |
| Amber | #FBBF24 | #D97706 | Warnings, cloud mode |
| Surface | #1E1E2E | #FFFFFF | Background |
| SurfaceDim | #181825 | #F5F5F5 | Header/footer bg |
| Overlay | #313244 | #E5E5E5 | Borders, subtle bg |
| TextPrimary | #CDD6F4 | #1F2937 | Main text |
| TextSecondary | #A6ADC8 | #6B7280 | Labels |
| TextMuted | #6C7086 | #9CA3AF | Hints, timestamps |

---

## Agent Assignment Recommendations

### Agent A: Core Infrastructure (Waves 1-2)
- Focus: Ollama client, streaming, message models
- Skills needed: Go HTTP, SSE parsing, Bubble Tea basics

### Agent B: Visual Components (Wave 3)
- Focus: All UI components
- Skills needed: Lip Gloss mastery, responsive design

### Agent C: Commands & Context (Waves 4-5)
- Focus: Slash commands, @ mentions, tab completion
- Skills needed: Parsing, file I/O, clipboard APIs

### Agent D: Session & Tools (Waves 6-7)
- Focus: Persistence, tool system, permissions
- Skills needed: JSON, exec, security considerations

### Agent E: Polish (Wave 8)
- Focus: Animations, syntax highlighting, welcome screen
- Skills needed: Animation, Chroma, Glamour

---

## Final Notes

This plan creates a **WORLD-CLASS** terminal UI that will set the standard for CLI applications. Every component is designed with beauty and usability in mind.

Key differentiators:
1. **Adaptive colors** - Works beautifully in any terminal
2. **Smooth animations** - Professional, polished feel
3. **Component architecture** - Easy to extend and maintain
4. **Comprehensive tool system** - Full agentic capabilities
5. **Session management** - DoD compliance built-in

**"Your GPU, your data, your interface - beautiful."**
