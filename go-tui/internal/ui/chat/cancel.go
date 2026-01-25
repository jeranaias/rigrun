// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
//
// This file implements thread-safe cancel function handling to fix the race condition
// where cancelFunc was accessed from both the Update loop and goroutines without synchronization.
package chat

import (
	"context"
	"sync"
)

// =============================================================================
// CANCEL FUNCTION MANAGEMENT (THREAD-SAFE)
// =============================================================================

// cancelManager manages the cancel function with mutex protection.
// This prevents race conditions when accessing cancelFunc from multiple goroutines.
// IMPORTANT: This must be used as a pointer (*cancelManager) in Model structs to prevent
// copying the mutex when Bubble Tea's Update function returns model copies.
type cancelManager struct {
	mu         sync.Mutex
	cancelFunc context.CancelFunc
}

// newCancelManager creates a new cancelManager pointer.
// Always use this constructor to ensure proper initialization.
func newCancelManager() *cancelManager {
	return &cancelManager{}
}

// setCancelFunc stores a new cancel function in a thread-safe manner.
func (cm *cancelManager) setCancelFunc(fn context.CancelFunc) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cancelFunc = fn
}

// cancel invokes the stored cancel function and clears it, in a thread-safe manner.
// Safe to call multiple times or with no cancel function set.
func (cm *cancelManager) cancel() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.cancelFunc != nil {
		cm.cancelFunc()
		cm.cancelFunc = nil
	}
}

// clear cancels the context (if present) and removes the cancel function.
// This ensures contexts are always properly cancelled to prevent resource leaks.
// Safe to call multiple times or with no cancel function set.
func (cm *cancelManager) clear() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.cancelFunc != nil {
		cm.cancelFunc() // Always cancel to prevent context leaks
		cm.cancelFunc = nil
	}
}

// =============================================================================
// MODEL METHODS (CONVENIENCE WRAPPERS)
// =============================================================================

// setCancelFunc stores a new cancel function for the current streaming operation.
func (m *Model) setCancelFunc(fn context.CancelFunc) {
	m.cancelMgr.setCancelFunc(fn)
}

// cancel cancels the current streaming operation if one is in progress.
func (m *Model) cancel() {
	m.cancelMgr.cancel()
}

// clearCancelFunc cancels the context and clears the cancel function.
// This ensures contexts are always properly cancelled to prevent resource leaks.
func (m *Model) clearCancelFunc() {
	m.cancelMgr.clear()
}
