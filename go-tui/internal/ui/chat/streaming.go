// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
//
// This file implements streaming optimization to provide smooth, flicker-free
// rendering during LLM response streaming. The StreamingBuffer batches tokens
// for efficient rendering at a capped frame rate to balance responsiveness
// with CPU efficiency.
package chat

import (
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// STREAMING BUFFER
// =============================================================================

// StreamingBuffer batches tokens for efficient rendering.
// Tokens are accumulated in a buffer and flushed either when:
// 1. The batch size threshold is reached (e.g., 15 tokens)
// 2. Enough time has passed since the last flush (e.g., 33ms for 30fps)
//
// This prevents excessive rendering (>1000fps) which causes flicker and
// high CPU usage, while maintaining smooth visual updates.
//
// Thread-safety: All operations are protected by a mutex since streaming
// happens in a goroutine while rendering happens in the main Bubble Tea loop.
type StreamingBuffer struct {
	mu         sync.Mutex
	buffer     strings.Builder
	tokenCount int
	lastFlush  time.Time

	// Configuration
	batchSize  int           // Tokens per batch (default: 15)
	maxFPS     int           // Max frames per second (default: 30)
	minFlushMs time.Duration // Min time between flushes (1000/maxFPS)
}

// NewStreamingBuffer creates an optimized streaming buffer with default settings.
// Default configuration:
// - Batch size: 15 tokens (balances latency vs throughput)
// - Max FPS: 30 (smooth but not wasteful)
// - Min flush interval: ~33ms (1000ms / 30fps)
func NewStreamingBuffer() *StreamingBuffer {
	const (
		defaultBatchSize = 15
		defaultMaxFPS    = 30
	)

	return &StreamingBuffer{
		batchSize:  defaultBatchSize,
		maxFPS:     defaultMaxFPS,
		minFlushMs: time.Duration(1000/defaultMaxFPS) * time.Millisecond,
		lastFlush:  time.Now(),
	}
}

// NewStreamingBufferWithConfig creates a streaming buffer with custom settings.
// Use this when you need different batch sizes or frame rates for specific use cases.
func NewStreamingBufferWithConfig(batchSize, maxFPS int) *StreamingBuffer {
	if batchSize <= 0 {
		batchSize = 15
	}
	if maxFPS <= 0 || maxFPS > 60 {
		maxFPS = 30
	}

	return &StreamingBuffer{
		batchSize:  batchSize,
		maxFPS:     maxFPS,
		minFlushMs: time.Duration(1000/maxFPS) * time.Millisecond,
		lastFlush:  time.Now(),
	}
}

// Write adds a token to the buffer.
// This is called from the streaming goroutine, so it's thread-safe.
// The token is accumulated in the buffer and will be flushed when
// either the batch size or time threshold is reached.
func (sb *StreamingBuffer) Write(token string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	sb.buffer.WriteString(token)
	sb.tokenCount++
}

// Flush returns accumulated content if the buffer should be flushed.
// Returns (content, hasContent) where:
// - content: the accumulated tokens since last flush
// - hasContent: true if there was content to flush
//
// The buffer is flushed if either:
// 1. Batch size threshold reached (e.g., 15 tokens accumulated)
// 2. Time threshold reached (e.g., 33ms since last flush)
//
// This is called from the main Bubble Tea loop, so it's thread-safe.
func (sb *StreamingBuffer) Flush() (string, bool) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	// Nothing to flush
	if sb.buffer.Len() == 0 {
		return "", false
	}

	// Check if we should flush based on size or time
	shouldFlush := sb.shouldFlushLocked()
	if !shouldFlush {
		return "", false
	}

	// Extract content and reset buffer
	content := sb.buffer.String()
	sb.buffer.Reset()
	sb.tokenCount = 0
	sb.lastFlush = time.Now()

	return content, true
}

// ShouldFlush checks if the buffer should be flushed (time or size based).
// This is a public method for external callers to check if a flush is needed.
// Thread-safe.
func (sb *StreamingBuffer) ShouldFlush() bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.shouldFlushLocked()
}

// shouldFlushLocked checks flush conditions without locking (caller must hold lock).
// Flush triggers when:
// 1. Token count >= batch size (e.g., accumulated 15+ tokens), OR
// 2. Time since last flush >= min flush interval (e.g., 33ms for 30fps)
func (sb *StreamingBuffer) shouldFlushLocked() bool {
	// Empty buffer never needs flushing
	if sb.buffer.Len() == 0 {
		return false
	}

	// Flush if batch size reached
	if sb.tokenCount >= sb.batchSize {
		return true
	}

	// Flush if enough time has passed (for smooth animation even with slow streams)
	timeSinceFlush := time.Since(sb.lastFlush)
	if timeSinceFlush >= sb.minFlushMs {
		return true
	}

	return false
}

// Reset clears the buffer without flushing.
// Use this when canceling a stream or starting a new message.
// Thread-safe.
func (sb *StreamingBuffer) Reset() {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	sb.buffer.Reset()
	sb.tokenCount = 0
	sb.lastFlush = time.Now()
}

// Pending returns the number of tokens waiting to be flushed.
// Useful for debugging and metrics.
// Thread-safe.
func (sb *StreamingBuffer) Pending() int {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.tokenCount
}

// ForceFlush immediately flushes all buffered content regardless of thresholds.
// Use this when a stream completes to ensure all tokens are rendered.
// Thread-safe.
func (sb *StreamingBuffer) ForceFlush() (string, bool) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.buffer.Len() == 0 {
		return "", false
	}

	content := sb.buffer.String()
	sb.buffer.Reset()
	sb.tokenCount = 0
	sb.lastFlush = time.Now()

	return content, true
}

// GetConfig returns the current buffer configuration.
// Thread-safe.
func (sb *StreamingBuffer) GetConfig() (batchSize, maxFPS int, minFlushMs time.Duration) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.batchSize, sb.maxFPS, sb.minFlushMs
}

// SetBatchSize updates the batch size threshold.
// Thread-safe.
func (sb *StreamingBuffer) SetBatchSize(size int) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if size > 0 {
		sb.batchSize = size
	}
}

// SetMaxFPS updates the maximum frame rate.
// Thread-safe.
func (sb *StreamingBuffer) SetMaxFPS(fps int) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if fps > 0 && fps <= 60 {
		sb.maxFPS = fps
		sb.minFlushMs = time.Duration(1000/fps) * time.Millisecond
	}
}

// =============================================================================
// STREAMING TICK COMMAND (Feature 4.2)
// =============================================================================

// streamTickCmd creates a tea.Cmd that sends StreamTickMsg at 30fps.
// This enables smooth, flicker-free streaming by batching token updates.
func streamTickCmd() tea.Cmd {
	return tea.Tick(33*time.Millisecond, func(t time.Time) tea.Msg {
		return StreamTickMsg{Time: t}
	})
}
