// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package session provides session management with DoD compliance timeout.
package session

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// SESSION MANAGER
// =============================================================================

// Manager tracks session state including timeout for DoD compliance.
type Manager struct {
	mu sync.Mutex

	// Session tracking
	sessionID    string
	startTime    time.Time
	lastActivity time.Time

	// Timeout configuration
	timeout        time.Duration // Default: 15 minutes for DoD compliance
	warningBefore  time.Duration // Default: 2 minutes before timeout
	warningShown   bool

	// Auto-save configuration
	autoSaveEnabled  bool
	autoSaveInterval time.Duration
	lastAutoSave     time.Time
	isDirty          bool

	// Callbacks
	onTimeout    func()
	onWarning    func(remaining time.Duration)
	onAutoSave   func() error
}

// Config holds configuration for the session manager.
type Config struct {
	// Timeout is the session timeout duration (default: 15 minutes)
	Timeout time.Duration

	// WarningBefore is how long before timeout to show warning (default: 2 minutes)
	WarningBefore time.Duration

	// AutoSaveEnabled enables automatic saving
	AutoSaveEnabled bool

	// AutoSaveInterval is how often to auto-save (default: 30 seconds)
	AutoSaveInterval time.Duration
}

// DefaultConfig returns the default session configuration.
func DefaultConfig() Config {
	return Config{
		Timeout:          15 * time.Minute,
		WarningBefore:    2 * time.Minute,
		AutoSaveEnabled:  true,
		AutoSaveInterval: 30 * time.Second,
	}
}

// NewManager creates a new session manager.
func NewManager(cfg Config) *Manager {
	now := time.Now()
	return &Manager{
		sessionID:        generateSessionID(),
		startTime:        now,
		lastActivity:     now,
		timeout:          cfg.Timeout,
		warningBefore:    cfg.WarningBefore,
		autoSaveEnabled:  cfg.AutoSaveEnabled,
		autoSaveInterval: cfg.AutoSaveInterval,
		lastAutoSave:     now,
	}
}

// =============================================================================
// SESSION STATE
// =============================================================================

// SessionID returns the current session ID.
func (m *Manager) SessionID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessionID
}

// StartTime returns when the session started.
func (m *Manager) StartTime() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startTime
}

// Duration returns how long the session has been active.
func (m *Manager) Duration() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return time.Since(m.startTime)
}

// IdleTime returns how long since last activity.
func (m *Manager) IdleTime() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return time.Since(m.lastActivity)
}

// RemainingTime returns time until session timeout.
func (m *Manager) RemainingTime() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	remaining := m.timeout - time.Since(m.lastActivity)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// =============================================================================
// ACTIVITY TRACKING
// =============================================================================

// RecordActivity updates the last activity timestamp.
// This should be called on user input or other activity.
func (m *Manager) RecordActivity() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastActivity = time.Now()
	m.warningShown = false
}

// MarkDirty indicates the session has unsaved changes.
func (m *Manager) MarkDirty() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isDirty = true
}

// MarkClean indicates the session has been saved.
func (m *Manager) MarkClean() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isDirty = false
	m.lastAutoSave = time.Now()
}

// IsDirty returns whether the session has unsaved changes.
func (m *Manager) IsDirty() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isDirty
}

// =============================================================================
// CALLBACKS
// =============================================================================

// SetTimeoutCallback sets the function called when session times out.
func (m *Manager) SetTimeoutCallback(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onTimeout = fn
}

// SetWarningCallback sets the function called when approaching timeout.
func (m *Manager) SetWarningCallback(fn func(remaining time.Duration)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onWarning = fn
}

// SetAutoSaveCallback sets the function called for auto-save.
func (m *Manager) SetAutoSaveCallback(fn func() error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onAutoSave = fn
}

// =============================================================================
// TIMEOUT CHECKING
// =============================================================================

// IsExpired returns true if the session has timed out.
func (m *Manager) IsExpired() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return time.Since(m.lastActivity) >= m.timeout
}

// ShouldShowWarning returns true if timeout warning should be shown.
func (m *Manager) ShouldShowWarning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.warningShown {
		return false
	}

	idle := time.Since(m.lastActivity)
	threshold := m.timeout - m.warningBefore

	return idle >= threshold && idle < m.timeout
}

// ShouldAutoSave returns true if auto-save should trigger.
func (m *Manager) ShouldAutoSave() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.autoSaveEnabled || !m.isDirty {
		return false
	}

	return time.Since(m.lastAutoSave) >= m.autoSaveInterval
}

// Check evaluates session state and triggers appropriate callbacks.
// Returns true if session is still valid, false if expired.
func (m *Manager) Check() bool {
	m.mu.Lock()
	expired := time.Since(m.lastActivity) >= m.timeout

	// Check for warning
	shouldWarn := false
	var remaining time.Duration
	if !m.warningShown && !expired {
		idle := time.Since(m.lastActivity)
		threshold := m.timeout - m.warningBefore
		if idle >= threshold {
			shouldWarn = true
			remaining = m.timeout - idle
			m.warningShown = true
		}
	}

	// Check for auto-save
	shouldSave := m.autoSaveEnabled && m.isDirty &&
		time.Since(m.lastAutoSave) >= m.autoSaveInterval

	// Get callbacks
	onTimeout := m.onTimeout
	onWarning := m.onWarning
	onAutoSave := m.onAutoSave
	m.mu.Unlock()

	// Execute callbacks outside lock
	if shouldWarn && onWarning != nil {
		onWarning(remaining)
	}

	if shouldSave && onAutoSave != nil {
		if err := onAutoSave(); err == nil {
			m.MarkClean()
		}
	}

	if expired && onTimeout != nil {
		onTimeout()
	}

	return !expired
}

// =============================================================================
// BUBBLE TEA INTEGRATION
// =============================================================================

// TickMsg is sent periodically to check session state.
type TickMsg struct {
	Time time.Time
}

// TimeoutWarningMsg indicates session is about to timeout.
type TimeoutWarningMsg struct {
	Remaining time.Duration
}

// TimeoutMsg indicates session has timed out.
type TimeoutMsg struct{}

// AutoSaveMsg indicates auto-save should occur.
type AutoSaveMsg struct{}

// TickCmd returns a command that ticks periodically.
func TickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}

// HandleTick processes a tick and returns appropriate messages.
func (m *Manager) HandleTick() tea.Cmd {
	var cmds []tea.Cmd

	// Check for timeout warning
	if m.ShouldShowWarning() {
		remaining := m.RemainingTime()
		cmds = append(cmds, func() tea.Msg {
			return TimeoutWarningMsg{Remaining: remaining}
		})
		m.mu.Lock()
		m.warningShown = true
		m.mu.Unlock()
	}

	// Check for timeout
	if m.IsExpired() {
		cmds = append(cmds, func() tea.Msg {
			return TimeoutMsg{}
		})
	}

	// Check for auto-save
	if m.ShouldAutoSave() {
		cmds = append(cmds, func() tea.Msg {
			return AutoSaveMsg{}
		})
	}

	// Continue ticking
	cmds = append(cmds, TickCmd())

	return tea.Batch(cmds...)
}

// =============================================================================
// CONFIGURATION
// =============================================================================

// SetTimeout updates the timeout duration.
func (m *Manager) SetTimeout(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeout = d
}

// SetWarningTime updates when to show timeout warning.
func (m *Manager) SetWarningTime(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warningBefore = d
}

// SetAutoSaveEnabled enables or disables auto-save.
func (m *Manager) SetAutoSaveEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autoSaveEnabled = enabled
}

// SetAutoSaveInterval updates the auto-save interval.
func (m *Manager) SetAutoSaveInterval(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autoSaveInterval = d
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// generateSessionID creates a unique session ID.
func generateSessionID() string {
	return "sess_" + formatTimestamp(time.Now())
}

// formatTimestamp formats a time for use in IDs.
func formatTimestamp(t time.Time) string {
	return t.Format("20060102_150405")
}

// =============================================================================
// SESSION STATUS
// =============================================================================

// Status represents the current session status.
type Status struct {
	SessionID     string
	StartTime     time.Time
	Duration      time.Duration
	IdleTime      time.Duration
	RemainingTime time.Duration
	IsDirty       bool
	IsExpired     bool
}

// GetStatus returns the current session status.
func (m *Manager) GetStatus() Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	idle := now.Sub(m.lastActivity)
	remaining := m.timeout - idle
	if remaining < 0 {
		remaining = 0
	}

	return Status{
		SessionID:     m.sessionID,
		StartTime:     m.startTime,
		Duration:      now.Sub(m.startTime),
		IdleTime:      idle,
		RemainingTime: remaining,
		IsDirty:       m.isDirty,
		IsExpired:     idle >= m.timeout,
	}
}

// FormatDuration returns a human-readable duration string.
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		secs := int(d.Seconds())
		return util.IntToString(secs) + "s"
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	if secs == 0 {
		return util.IntToString(mins) + "m"
	}
	return util.IntToString(mins) + "m " + util.IntToString(secs) + "s"
}
