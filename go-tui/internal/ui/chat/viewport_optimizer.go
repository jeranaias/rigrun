// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
//
// This file implements viewport optimization to reduce redundant viewport updates
// during streaming. The ViewportOptimizer tracks content changes and only triggers
// redraws when the content actually changes, preventing unnecessary CPU usage.
package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// =============================================================================
// VIEWPORT OPTIMIZER
// =============================================================================

// ViewportOptimizer reduces redundant viewport updates by tracking content changes.
// During streaming, we may attempt to update the viewport hundreds of times per second.
// Many of these updates are redundant (same content). This optimizer uses a fast
// content hash to detect actual changes and skip redundant updates.
//
// Thread-safety: All operations are protected by a mutex.
type ViewportOptimizer struct {
	mu             sync.RWMutex
	lastContentHash string        // SHA-256 hash of last rendered content
	lastUpdateTime time.Time      // Time of last update
	dirty          bool           // Whether content has changed since last render
	updateCount    uint64         // Total update attempts
	skipCount      uint64         // Updates skipped due to no change
}

// NewViewportOptimizer creates a new viewport optimizer.
func NewViewportOptimizer() *ViewportOptimizer {
	return &ViewportOptimizer{
		lastUpdateTime: time.Now(),
		dirty:          true, // Start dirty to force initial render
	}
}

// ShouldUpdate returns true if the viewport needs to be redrawn.
// This performs a fast hash comparison to detect actual content changes.
//
// The optimizer uses SHA-256 hashing because:
// 1. It's fast enough for typical message sizes (<1ms for 100KB)
// 2. Collision probability is negligible for our use case
// 3. It's more reliable than length-based checks (content can change with same length)
//
// Thread-safe.
func (vo *ViewportOptimizer) ShouldUpdate(newContent string) bool {
	vo.mu.Lock()
	defer vo.mu.Unlock()

	vo.updateCount++

	// First update always proceeds
	if vo.updateCount == 1 {
		vo.lastContentHash = hashContent(newContent)
		vo.lastUpdateTime = time.Now()
		vo.dirty = true
		return true
	}

	// Compute hash of new content
	newHash := hashContent(newContent)

	// Compare with last hash
	if newHash == vo.lastContentHash {
		// Content unchanged - skip update
		vo.skipCount++
		return false
	}

	// Content changed - update required
	vo.lastContentHash = newHash
	vo.lastUpdateTime = time.Now()
	vo.dirty = true

	return true
}

// MarkClean marks the viewport as up-to-date after a render.
// Call this after successfully rendering the viewport.
// Thread-safe.
func (vo *ViewportOptimizer) MarkClean() {
	vo.mu.Lock()
	defer vo.mu.Unlock()
	vo.dirty = false
}

// IsDirty returns true if the viewport has pending changes.
// Thread-safe.
func (vo *ViewportOptimizer) IsDirty() bool {
	vo.mu.RLock()
	defer vo.mu.RUnlock()
	return vo.dirty
}

// Reset clears the optimizer state.
// Use this when starting a new conversation or clearing history.
// Thread-safe.
func (vo *ViewportOptimizer) Reset() {
	vo.mu.Lock()
	defer vo.mu.Unlock()

	vo.lastContentHash = ""
	vo.lastUpdateTime = time.Now()
	vo.dirty = true
	// Don't reset counters - keep them for metrics
}

// GetStats returns optimizer statistics.
// Returns (totalUpdates, skippedUpdates, efficiency%)
// Thread-safe.
func (vo *ViewportOptimizer) GetStats() (total, skipped uint64, efficiency float64) {
	vo.mu.RLock()
	defer vo.mu.RUnlock()

	total = vo.updateCount
	skipped = vo.skipCount

	if total > 0 {
		efficiency = float64(skipped) / float64(total) * 100.0
	}

	return
}

// ForceUpdate forces the next update to proceed regardless of content changes.
// Use this when you need to guarantee a viewport update (e.g., after resize).
// Thread-safe.
func (vo *ViewportOptimizer) ForceUpdate() {
	vo.mu.Lock()
	defer vo.mu.Unlock()

	vo.lastContentHash = ""
	vo.dirty = true
}

// GetLastUpdateTime returns the time of the last viewport update.
// Thread-safe.
func (vo *ViewportOptimizer) GetLastUpdateTime() time.Time {
	vo.mu.RLock()
	defer vo.mu.RUnlock()
	return vo.lastUpdateTime
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// hashContent computes a SHA-256 hash of the content for change detection.
// This is fast enough for real-time use (~0.5ms for 100KB) and provides
// reliable content comparison without false positives.
func hashContent(content string) string {
	if content == "" {
		return ""
	}

	// Use SHA-256 for reliable, fast hashing
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// =============================================================================
// BATCH VIEWPORT UPDATER
// =============================================================================

// BatchViewportUpdater combines multiple viewport update requests into batches.
// This is useful when multiple components want to update the viewport
// (e.g., streaming + scroll + resize) - we batch them into a single update.
type BatchViewportUpdater struct {
	mu            sync.Mutex
	pendingUpdate bool
	lastBatchTime time.Time
	batchInterval time.Duration
}

// NewBatchViewportUpdater creates a new batch updater.
func NewBatchViewportUpdater(batchInterval time.Duration) *BatchViewportUpdater {
	if batchInterval <= 0 {
		batchInterval = 16 * time.Millisecond // ~60fps default
	}

	return &BatchViewportUpdater{
		batchInterval: batchInterval,
		lastBatchTime: time.Now(),
	}
}

// RequestUpdate requests a viewport update.
// Returns true if the update should proceed now, false if it should be batched.
// Thread-safe.
func (bvu *BatchViewportUpdater) RequestUpdate() bool {
	bvu.mu.Lock()
	defer bvu.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(bvu.lastBatchTime)

	// If enough time has passed, allow the update
	if elapsed >= bvu.batchInterval {
		bvu.lastBatchTime = now
		bvu.pendingUpdate = false
		return true
	}

	// Otherwise, mark as pending and batch it
	bvu.pendingUpdate = true
	return false
}

// HasPending returns true if there's a pending update.
// Thread-safe.
func (bvu *BatchViewportUpdater) HasPending() bool {
	bvu.mu.Lock()
	defer bvu.mu.Unlock()
	return bvu.pendingUpdate
}

// Reset clears pending updates.
// Thread-safe.
func (bvu *BatchViewportUpdater) Reset() {
	bvu.mu.Lock()
	defer bvu.mu.Unlock()
	bvu.pendingUpdate = false
	bvu.lastBatchTime = time.Now()
}
