# TUI IL5 Compliance Implementation Plan

**Document Version:** 1.0
**Date:** 2026-01-20
**Status:** Planning
**Author:** Generated for rigrun TUI IL5 compliance effort

---

## Executive Summary

This document outlines the strategic implementation plan for adding IL5 (Impact Level 5) compliance features to the rigrun TUI (Terminal User Interface). The CLI already has full IL5 compliance including audit logging, session management, classification marking, consent banners, and offline mode. This plan details how to bring those same features to the Bubble Tea TUI.

---

## Section 1: Current TUI Architecture

### 1.1 Bubble Tea Model-Update-View Pattern

The TUI follows the Elm Architecture (Model-Update-View) via the Bubble Tea framework:

```
main.go
  |
  +-- Model (struct)           # Application state
  |     |
  |     +-- state              # Current application state (Welcome, Chat, Error)
  |     +-- theme              # Styling/colors
  |     +-- chatModel          # Embedded chat model
  |     +-- welcome            # Welcome screen component
  |     +-- errorDisplay       # Error overlay component
  |     +-- ollamaClient       # Local LLM client
  |     +-- cloudClient        # OpenRouter cloud client
  |     +-- sessionMgr         # Session management
  |     +-- toolRegistry       # Tool system for agentic loop
  |
  +-- Init()                   # Initialize model, return startup commands
  |
  +-- Update(msg)              # Handle messages, return updated model + commands
  |     |
  |     +-- tea.WindowSizeMsg  # Handle resize
  |     +-- tea.KeyMsg         # Handle keyboard input
  |     +-- StreamTokenMsg     # Handle streaming tokens
  |     +-- Custom messages... # Various internal messages
  |
  +-- View()                   # Render current state to string
```

### 1.2 Current Component Structure

| Component | Location | Responsibility |
|-----------|----------|----------------|
| **Main Model** | `main.go:Model` | Top-level state, routing, Ollama integration |
| **Chat Model** | `internal/ui/chat/model.go` | Chat conversation, streaming, routing |
| **Chat View** | `internal/ui/chat/view.go` | Render chat messages, input, status |
| **Welcome** | `internal/ui/components/welcome.go` | Startup screen |
| **StatusBar** | `internal/ui/components/statusbar.go` | Bottom status bar |
| **ErrorDisplay** | `internal/ui/components/error.go` | Error overlay |
| **Header** | `internal/ui/components/header.go` | Top header |

### 1.3 Current Application States

```go
type State int

const (
    StateWelcome State = iota  // Welcome screen - press any key
    StateChat                   // Main chat view
    StateError                  // Error display overlay
)
```

### 1.4 Existing Security Infrastructure

The CLI already has robust IL5 compliance code:

| Package | File | Purpose |
|---------|------|---------|
| `internal/security` | `audit.go` | Audit logging with secret redaction |
| `internal/security` | `classification.go` | DoD classification marking (DoDI 5200.48) |
| `internal/security` | `session.go` | Session timeout management (AC-12) |
| `internal/security` | `banner.go` | DoD consent banner (AC-8) |
| `internal/offline` | `offline.go` | Offline mode / air-gapped operation (SC-7) |
| `internal/cli` | `consent.go` | Consent command implementation |
| `internal/cli` | `audit_cmd.go` | Audit command implementation |
| `internal/cli` | `classify_cmd.go` | Classification command implementation |

---

## Section 2: IL5 Features to Add

### Feature 2.1: Classification Banner (P1)

**What it is:**
A persistent colored banner at the top of the screen showing the current classification level (UNCLASSIFIED, CUI, CONFIDENTIAL, SECRET, TOP SECRET) with DoD-standard colors.

**IL5 Control:**
- DoDI 5200.48 (Classification Marking Requirements)
- SC-16 (Transmission of Security Attributes)

**Where in TUI:**
- Top of every view (Welcome, Chat)
- Must be visible at all times
- Height: 1 line, full terminal width

**Implementation Approach:**
1. Create new `ClassificationBanner` component in `internal/ui/components/classification_banner.go`
2. Use existing `security.Classification` and `security.RenderTopBanner()` functions
3. Add `classificationBanner` field to `Model` struct in `main.go`
4. Modify `View()` to prepend classification banner to all views
5. Read classification level from config at startup
6. Subscribe to classification change events

**Technical Details:**
```go
// New component
type ClassificationBanner struct {
    classification security.Classification
    width          int
}

func (b *ClassificationBanner) View() string {
    return security.RenderTopBanner(b.classification, b.width)
}
```

**Colors per DoDI 5200.48:**
- UNCLASSIFIED: Green (#00FF00) with black text
- CUI: Purple (#800080) with white text
- CONFIDENTIAL: Blue (#0000FF) with white text
- SECRET: Red (#FF0000) with white text
- TOP SECRET: Orange (#FFA500) with black text

---

### Feature 2.2: Consent Banner on Startup (P1)

**What it is:**
DoD System Use Notification that must be displayed and acknowledged before the user can interact with the TUI. Required by AC-8 (System Use Notification).

**IL5 Control:**
- AC-8 (System Use Notification)
- AU-12 (Audit Generation) - log acknowledgment

**Where in TUI:**
- New state: `StateConsent` shown before `StateWelcome`
- Full-screen amber/gold banner
- Requires explicit acknowledgment (press Enter or Y)

**Implementation Approach:**
1. Add new `StateConsent` state to `main.go`
2. Create `ConsentBanner` component in `internal/ui/components/consent_banner.go`
3. Check `config.Consent.Required` at startup
4. If required and not already accepted, show consent banner
5. On acknowledgment, log to audit trail and proceed to welcome
6. Use existing `cli.DoDConsentBanner` text and `security.LogBannerAcknowledgment()`

**Technical Details:**
```go
// Add new state
const (
    StateConsent State = iota  // NEW: Consent banner (before welcome)
    StateWelcome
    StateChat
    StateError
)

// New component
type ConsentBanner struct {
    width, height int
    acknowledged  bool
}

func (c *ConsentBanner) View() string {
    // Render amber banner with DoD text
    // Show "Press Enter to acknowledge" prompt
}
```

**Key Messages:**
- `ConsentAcknowledgedMsg{}` - User pressed Enter
- `ConsentDeclinedMsg{}` - User pressed Ctrl+C

---

### Feature 2.3: Session Timeout Warning/Auto-Logout (P1)

**What it is:**
Visual warning when session is about to expire (2 minutes before timeout), and automatic logout when session expires. Default timeout is 15 minutes per DoD STIG.

**IL5 Control:**
- AC-12 (Session Termination)
- AC-11 (Session Lock)
- AU-3 (Audit Content) - log timeout events

**Where in TUI:**
- Warning overlay appears 2 minutes before timeout
- Auto-logout exits TUI when session expires
- Session timeout resets on any user activity

**Implementation Approach:**
1. Integrate existing `security.SessionManager` into TUI
2. Create `SessionTimeoutOverlay` component
3. Add `sessionManager` field to main Model
4. Start session in `Init()` using `sessionManager.StartSession()`
5. Set callbacks for warning and expiration
6. Refresh session on any `tea.KeyMsg` or `tea.MouseMsg`
7. Send `SessionWarningMsg` at 2 minutes remaining
8. Send `SessionExpiredMsg` at timeout, trigger `tea.Quit`

**Technical Details:**
```go
// In Init()
m.sessionManager = security.NewDefaultSessionManager()
m.sessionManager.SetCallbacks(
    func() { /* warning callback */ },
    func() { /* expired callback */ },
)
session, _ := m.sessionManager.StartSession()

// In Update() for any input
m.sessionManager.RefreshSession()

// New messages
type SessionWarningMsg struct {
    TimeRemaining time.Duration
}

type SessionExpiredMsg struct {}
```

---

### Feature 2.4: Offline Mode Indicator (P1)

**What it is:**
Visual "[OFFLINE]" badge displayed when `--no-network` flag is active, indicating SC-7 boundary protection is enforced.

**IL5 Control:**
- SC-7 (Boundary Protection)
- SC-8 (Transmission Confidentiality)

**Where in TUI:**
- In the header area (next to model name)
- In the status bar
- Styled with distinctive color (red background)

**Implementation Approach:**
1. Check `offline.IsOfflineMode()` in View functions
2. Add offline indicator to header component
3. Add offline badge to status bar
4. Use existing `offline.StatusBadge()` function

**Technical Details:**
```go
// In header rendering
func (h *Header) View() string {
    header := h.renderTitle()
    if offline.IsOfflineMode() {
        offlineBadge := lipgloss.NewStyle().
            Background(lipgloss.Color("#FF0000")).
            Foreground(lipgloss.Color("#FFFFFF")).
            Bold(true).
            Padding(0, 1).
            Render("[OFFLINE]")
        header = header + " " + offlineBadge
    }
    return header
}
```

---

### Feature 2.5: Classification Level in Status Bar (P2)

**What it is:**
Always-visible classification indicator in the bottom status bar, complementing the top banner.

**IL5 Control:**
- DoDI 5200.48 (Classification Marking)

**Where in TUI:**
- Left side of status bar (StatusBar component)
- Shows portion marking format: (U), (CUI), (S), etc.

**Implementation Approach:**
1. Add `classification` field to `StatusBar` struct
2. Add `SetClassification()` method
3. Modify `viewWide()`, `viewMedium()`, `viewNarrow()` to include classification badge
4. Use `security.RenderPortionMarking()` for display

**Technical Details:**
```go
// In StatusBar struct
type StatusBar struct {
    // ... existing fields
    Classification security.Classification
}

// In viewWide()
classificationBadge := security.InlineMarker(s.Classification)
leftParts = append([]string{classificationBadge}, leftParts...)
```

---

### Feature 2.6: Audit Event Viewer (P2)

**What it is:**
TUI command `/audit` to view recent audit log entries inline in the chat, similar to `/status` or `/help`.

**IL5 Control:**
- AU-6 (Audit Review)
- AU-9 (Protection of Audit Information)

**Where in TUI:**
- Slash command `/audit` in chat
- Shows last N entries (default 10) as system message
- Options: `/audit 20` for more entries

**Implementation Approach:**
1. Add `/audit` command handler in `chat/model.go:handleCommand()`
2. Reuse `cli.readAuditEntries()` function
3. Format entries as readable system message
4. Display in conversation

**Technical Details:**
```go
// In handleCommand()
case cmdName == "/audit":
    count := 10
    if len(args) > 0 {
        if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
            count = n
        }
    }
    entries, err := readAuditEntriesForTUI(count)
    if err != nil {
        m.conversation.AddSystemMessage("Error reading audit log: " + err.Error())
    } else {
        m.conversation.AddSystemMessage(formatAuditEntries(entries))
    }
    m.updateViewport()
    return m, nil
```

---

### Feature 2.7: Session Timeout Countdown (P2)

**What it is:**
Visual timer in status bar showing minutes:seconds until session expires.

**IL5 Control:**
- AC-12 (Session Termination)

**Where in TUI:**
- Right side of status bar
- Format: "Session: 12:34"
- Color changes: green (>5min), yellow (2-5min), red (<2min)

**Implementation Approach:**
1. Add `SessionTimeRemaining` field to StatusBar
2. Create `tick` message that fires every second
3. Update display color based on remaining time
4. Get time from `sessionManager.TimeRemaining()`

**Technical Details:**
```go
// Add to StatusBar
SessionTimeout time.Duration

// In viewWide() add:
if s.SessionTimeout > 0 {
    timeStr := formatSessionTime(s.SessionTimeout)
    color := styles.Emerald
    if s.SessionTimeout < 2*time.Minute {
        color = styles.Rose
    } else if s.SessionTimeout < 5*time.Minute {
        color = styles.Amber
    }
    timeStyle := lipgloss.NewStyle().Foreground(color)
    rightParts = append([]string{timeStyle.Render("Session: " + timeStr)}, rightParts...)
}

// New message for ticks
type SessionTickMsg struct{}
```

---

### Feature 2.8: Classify Command in TUI (P2)

**What it is:**
TUI command `/classify` to view and change the current classification level.

**IL5 Control:**
- DoDI 5200.48
- SC-16 (Transmission of Security Attributes)

**Where in TUI:**
- Slash commands: `/classify` (show), `/classify SECRET` (set)

**Implementation Approach:**
1. Add `/classify` command handler
2. Reuse `security.ParseClassification()` for validation
3. Update config and emit `ClassificationChangedMsg`
4. Update all classification banners

**Technical Details:**
```go
case cmdName == "/classify":
    if len(args) == 0 {
        // Show current classification
        cfg := config.Global()
        classification, _ := security.ParseClassification(cfg.Security.Classification)
        msg := fmt.Sprintf("Current classification: %s\n\nTo change: /classify <LEVEL>\nLevels: UNCLASSIFIED, CUI, CONFIDENTIAL, SECRET, TOP_SECRET",
            classification.String())
        m.conversation.AddSystemMessage(msg)
    } else {
        // Set classification
        level := strings.ToUpper(args[0])
        classification, err := security.ParseClassification(level)
        if err != nil {
            m.conversation.AddSystemMessage("Invalid classification: " + err.Error())
        } else {
            // Save to config
            cfg := config.Global()
            cfg.Security.Classification = classification.String()
            config.Save(cfg)
            // Emit change message
            return m, func() tea.Msg {
                return ClassificationChangedMsg{Classification: classification}
            }
        }
    }
    m.updateViewport()
    return m, nil
```

---

### Feature 2.9: Consent Reset in TUI (P2)

**What it is:**
TUI command `/consent` to view current consent status and optionally reset it for re-acknowledgment.

**IL5 Control:**
- AC-8 (System Use Notification)

**Where in TUI:**
- Commands: `/consent` (status), `/consent reset` (force re-acknowledgment)

**Implementation Approach:**
1. Add `/consent` command handler
2. For reset, clear consent in config and transition to `StateConsent`

**Technical Details:**
```go
case cmdName == "/consent":
    if len(args) > 0 && strings.ToLower(args[0]) == "reset" {
        // Reset consent
        cfg := config.Global()
        cfg.Consent.Accepted = false
        cfg.Consent.AcceptedAt = time.Time{}
        config.Save(cfg)
        m.conversation.AddSystemMessage("Consent reset. You will see the banner on next TUI start.")
        m.updateViewport()
        return m, nil
    }
    // Show status
    cfg := config.Global()
    status := "Consent Status:\n"
    status += fmt.Sprintf("  Required: %v\n", cfg.Consent.Required)
    status += fmt.Sprintf("  Accepted: %v\n", cfg.Consent.Accepted)
    if !cfg.Consent.AcceptedAt.IsZero() {
        status += fmt.Sprintf("  Accepted At: %s\n", cfg.Consent.AcceptedAt.Format(time.RFC3339))
    }
    status += "\nUse '/consent reset' to require re-acknowledgment."
    m.conversation.AddSystemMessage(status)
    m.updateViewport()
    return m, nil
```

---

### Feature 2.10: Network Status Indicator (P3)

**What it is:**
Show cloud connectivity status - whether OpenRouter/cloud services are available or blocked.

**IL5 Control:**
- SC-7 (Boundary Protection)

**Where in TUI:**
- Status bar indicator
- Shows: "Cloud: OK" / "Cloud: Blocked" / "Cloud: N/A"

**Implementation Approach:**
1. Add `CloudStatus` field to StatusBar
2. Check `offline.IsOfflineMode()` for blocked
3. Check `cloudClient.IsConfigured()` for availability
4. Periodic ping to verify connectivity (optional)

---

## Section 3: Implementation Waves

### Wave 1: Critical Security Features (Week 1-2)

**Features:** 2.1 (Classification Banner), 2.2 (Consent Banner), 2.3 (Session Timeout), 2.4 (Offline Indicator)

**Complexity:** High
**LOC Estimate:** ~800 lines

**Dependencies:**
- Classification Banner: None (uses existing security package)
- Consent Banner: None (uses existing security package)
- Session Timeout: None (uses existing security package)
- Offline Indicator: None (uses existing offline package)

**Tasks:**
1. Create `ClassificationBanner` component (2.1)
2. Modify `main.go:View()` to include classification banner
3. Create `ConsentBanner` component (2.2)
4. Add `StateConsent` state and handling
5. Integrate `SessionManager` into main model (2.3)
6. Create `SessionTimeoutOverlay` component
7. Add session refresh on user activity
8. Add offline badge to header and status bar (2.4)

**Testing:**
- Manual test all classification levels render correctly
- Test consent banner blocks until acknowledged
- Test session timeout warning appears at 2 minutes
- Test auto-logout at timeout
- Test offline badge appears with `--no-network`

---

### Wave 2: Status Display Enhancements (Week 3)

**Features:** 2.5 (Classification in Status Bar), 2.7 (Session Countdown)

**Complexity:** Medium
**LOC Estimate:** ~200 lines

**Dependencies:**
- Requires Wave 1 completion (classification and session infrastructure)

**Tasks:**
1. Add classification to StatusBar struct
2. Modify StatusBar view methods to include classification badge
3. Add session time remaining to StatusBar
4. Create session tick mechanism for countdown updates
5. Color-code countdown based on remaining time

**Testing:**
- Test classification badge matches top banner
- Test countdown updates every second
- Test color transitions at 5min, 2min thresholds

---

### Wave 3: Commands and Interactivity (Week 4)

**Features:** 2.6 (Audit Viewer), 2.8 (Classify Command), 2.9 (Consent Command)

**Complexity:** Medium
**LOC Estimate:** ~300 lines

**Dependencies:**
- Requires Wave 1 and 2 completion

**Tasks:**
1. Add `/audit` command handler
2. Create audit entry formatter for TUI display
3. Add `/classify` command handler with validation
4. Add `/consent` command handler
5. Handle `ClassificationChangedMsg` to update all banners

**Testing:**
- Test `/audit` shows recent entries
- Test `/audit N` respects count
- Test `/classify LEVEL` changes classification
- Test invalid levels are rejected
- Test `/consent` shows current status
- Test `/consent reset` clears acceptance

---

### Wave 4: Polish and Network Status (Week 5)

**Features:** 2.10 (Network Status Indicator)

**Complexity:** Low
**LOC Estimate:** ~100 lines

**Dependencies:**
- Requires Wave 1-3 completion

**Tasks:**
1. Add cloud status to StatusBar
2. Determine status based on offline mode and client config
3. Optionally add periodic connectivity check

**Testing:**
- Test status shows "Blocked" in offline mode
- Test status shows "N/A" without OpenRouter key
- Test status shows "OK" with configured cloud client

---

## Section 4: File Changes Required

### 4.1 New Files to Create

| File | Purpose |
|------|---------|
| `internal/ui/components/classification_banner.go` | Classification banner component |
| `internal/ui/components/consent_banner.go` | Consent banner component |
| `internal/ui/components/session_timeout_overlay.go` | Session warning/expiry overlay |

### 4.2 Existing Files to Modify

#### `main.go`

**Changes:**
- Add `StateConsent` to State enum
- Add fields to Model:
  - `classificationBanner ClassificationBanner`
  - `consentBanner ConsentBanner`
  - `sessionTimeoutOverlay SessionTimeoutOverlay`
  - `securitySessionMgr *security.SessionManager`
- Modify `Init()`:
  - Check consent requirement
  - Initialize security session
  - Start session timeout ticker
- Modify `Update()`:
  - Handle `ConsentAcknowledgedMsg`
  - Handle `SessionWarningMsg`
  - Handle `SessionExpiredMsg`
  - Handle `SessionTickMsg`
  - Handle `ClassificationChangedMsg`
  - Call `sessionMgr.RefreshSession()` on any user input
- Modify `View()`:
  - Prepend classification banner to all views
  - Show consent banner when in `StateConsent`
  - Show session timeout overlay when warning active

#### `internal/ui/chat/model.go`

**Changes:**
- Add `/audit`, `/classify`, `/consent` to `handleCommand()`
- Add helper functions for audit log reading
- Add helper functions for classification parsing/validation

#### `internal/ui/chat/view.go`

**Changes:**
- Modify `renderHeader()` to include offline badge
- Pass classification to status bar

#### `internal/ui/components/statusbar.go`

**Changes:**
- Add `Classification security.Classification` field
- Add `SessionTimeout time.Duration` field
- Add `CloudStatus string` field
- Add `SetClassification()` method
- Add `SetSessionTimeout()` method
- Modify `viewWide()`, `viewMedium()`, `viewNarrow()` to include:
  - Classification portion mark
  - Session countdown timer
  - Cloud status indicator

#### `internal/ui/components/header.go`

**Changes:**
- Add `OfflineMode bool` field
- Modify `View()` to include "[OFFLINE]" badge when offline

#### `internal/ui/components/welcome.go`

**Changes:**
- Add offline mode indicator to welcome display
- Show classification level

#### `internal/config/config.go`

**Changes:**
- Ensure `Security.Classification` field exists
- Ensure `Consent` struct exists with:
  - `Required bool`
  - `Accepted bool`
  - `AcceptedAt time.Time`
  - `AcceptedBy string`
  - `BannerVersion string`

---

## Section 5: Testing Strategy

### 5.1 Unit Tests

| Component | Test File | Tests |
|-----------|-----------|-------|
| ClassificationBanner | `classification_banner_test.go` | Render all levels, width handling |
| ConsentBanner | `consent_banner_test.go` | Render, key handling |
| SessionTimeoutOverlay | `session_timeout_overlay_test.go` | Warning state, expiry state |
| StatusBar | `statusbar_test.go` (extend) | Classification display, countdown |

### 5.2 Integration Tests

| Test | Description |
|------|-------------|
| `TestConsent_RequiredFlow` | Verify consent blocks until acknowledged |
| `TestConsent_NotRequired` | Verify app starts normally when not required |
| `TestClassification_Persistence` | Verify classification saves to config |
| `TestSession_Timeout` | Verify session expires at configured time |
| `TestSession_Warning` | Verify warning appears 2 minutes before |
| `TestSession_Refresh` | Verify activity resets timeout |
| `TestOffline_Indicator` | Verify badge shows with `--no-network` |

### 5.3 Manual Testing Checklist

#### Classification Banner Tests
- [ ] UNCLASSIFIED shows green banner
- [ ] CUI shows purple banner
- [ ] CONFIDENTIAL shows blue banner
- [ ] SECRET shows red banner
- [ ] TOP SECRET shows orange banner
- [ ] Banner is full width
- [ ] Banner visible on Welcome screen
- [ ] Banner visible on Chat screen
- [ ] `/classify` command works

#### Consent Banner Tests
- [ ] Banner appears when consent required and not accepted
- [ ] Banner has amber/gold styling
- [ ] Press Enter acknowledges and proceeds
- [ ] Press Ctrl+C exits without acknowledging
- [ ] Acknowledgment is logged to audit
- [ ] `/consent` shows status
- [ ] `/consent reset` clears acceptance

#### Session Timeout Tests
- [ ] Session starts on TUI launch
- [ ] Activity resets timeout timer
- [ ] Warning overlay appears at 2 minutes remaining
- [ ] Countdown timer updates every second
- [ ] Timer color changes at thresholds
- [ ] TUI exits when session expires
- [ ] Timeout event is logged to audit

#### Offline Mode Tests
- [ ] `--no-network` shows [OFFLINE] badge in header
- [ ] [OFFLINE] badge shows in status bar
- [ ] Cloud routing is disabled in offline mode
- [ ] `/classify` works in offline mode

#### Audit Viewer Tests
- [ ] `/audit` shows last 10 entries
- [ ] `/audit 20` shows last 20 entries
- [ ] Entries are formatted readably
- [ ] Timestamps are correct
- [ ] Secret data is redacted

### 5.4 Automated Test Commands

```bash
# Run unit tests
go test ./internal/ui/components/... -v

# Run all tests
go test ./... -v

# Test with race detector
go test ./... -race

# Test specific feature
go test ./internal/ui/components -run TestClassification -v
```

### 5.5 IL5 Compliance Verification

| Control | Feature | Verification Method |
|---------|---------|---------------------|
| AC-8 | Consent Banner | Manual: verify banner appears and blocks |
| AC-12 | Session Timeout | Manual: verify timeout and warning |
| AU-3 | Audit Content | Automated: check audit log format |
| AU-6 | Audit Review | Manual: verify `/audit` command |
| SC-7 | Offline Mode | Manual: verify with `--no-network` |
| DoDI 5200.48 | Classification | Manual: verify all levels and colors |

---

## Appendix A: Message Types Reference

### New Messages to Add

```go
// Consent messages
type ConsentAcknowledgedMsg struct{}
type ConsentDeclinedMsg struct{}

// Session messages
type SessionWarningMsg struct {
    TimeRemaining time.Duration
}
type SessionExpiredMsg struct{}
type SessionTickMsg struct{}

// Classification messages
type ClassificationChangedMsg struct {
    Classification security.Classification
}
```

### Existing Messages to Leverage

```go
// From tea package
tea.KeyMsg
tea.WindowSizeMsg
tea.Quit

// From current codebase
OllamaCheckMsg
StreamTokenMsg
StreamCompleteMsg
```

---

## Appendix B: Color Reference

### Classification Colors (DoDI 5200.48)

| Level | Background | Text | Hex Code |
|-------|------------|------|----------|
| UNCLASSIFIED | Green | Black | #00FF00 |
| CUI | Purple | White | #800080 |
| CONFIDENTIAL | Blue | White | #0000FF |
| SECRET | Red | White | #FF0000 |
| TOP SECRET | Orange | Black | #FFA500 |

### Consent Banner Color

| Element | Color | Hex Code |
|---------|-------|----------|
| Banner Text | Amber/Gold | #FFB000 |
| Border | Red | #FF0000 |

### Session Timeout Colors

| Time Remaining | Color | Meaning |
|----------------|-------|---------|
| > 5 minutes | Emerald/Green | Safe |
| 2-5 minutes | Amber/Yellow | Caution |
| < 2 minutes | Rose/Red | Warning |

---

## Appendix C: Configuration Schema

### config.yaml Security Section

```yaml
security:
  classification: "UNCLASSIFIED"  # Current classification level
  banner_enabled: true             # Show classification banners
  audit_enabled: true              # Enable audit logging
  audit_log_path: ""               # Custom audit log path (optional)

consent:
  required: true                   # Require consent acknowledgment
  accepted: false                  # Has user accepted?
  accepted_at: ""                  # When accepted (RFC3339)
  accepted_by: ""                  # Username who accepted
  banner_version: ""               # Version of banner accepted
```

---

## Appendix D: Keyboard Shortcuts

### New Shortcuts

| Key | Action | Context |
|-----|--------|---------|
| Enter | Acknowledge consent | Consent banner |
| Ctrl+C | Decline consent / Exit | Consent banner |
| Escape | Dismiss warning | Session warning overlay |

### Existing Shortcuts (unchanged)

| Key | Action | Context |
|-----|--------|---------|
| Enter | Submit message | Chat input |
| Ctrl+C | Cancel streaming / Exit | Chat |
| Ctrl+R | Cycle routing mode | Chat |
| Ctrl+F | Search | Chat |
| Ctrl+L | Clear conversation | Chat |

---

## Appendix E: Audit Log Format

### Existing Format (audit.log)

```
timestamp | event_type | session_id | tier | query | tokens | cost | status
```

### New Events for TUI

| Event Type | When | Metadata |
|------------|------|----------|
| `TUI_STARTUP` | TUI launches | version, mode |
| `TUI_SHUTDOWN` | TUI exits | duration, queries |
| `CONSENT_ACK_TUI` | Consent acknowledged in TUI | user, version |
| `SESSION_START_TUI` | TUI session starts | timeout_minutes |
| `SESSION_WARNING_TUI` | Warning overlay shown | time_remaining |
| `SESSION_EXPIRED_TUI` | Session times out | duration |
| `CLASSIFICATION_CHANGE_TUI` | Classification changed via /classify | old_level, new_level |

---

*End of Implementation Plan*
