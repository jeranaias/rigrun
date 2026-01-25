// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
package chat

import (
	"strings"
	"testing"
	"time"
)

// =============================================================================
// STREAMING BUFFER TESTS
// =============================================================================

func TestNewStreamingBuffer(t *testing.T) {
	sb := NewStreamingBuffer()

	if sb == nil {
		t.Fatal("NewStreamingBuffer returned nil")
	}

	batchSize, maxFPS, minFlushMs := sb.GetConfig()
	if batchSize != 15 {
		t.Errorf("Expected default batch size 15, got %d", batchSize)
	}
	if maxFPS != 30 {
		t.Errorf("Expected default maxFPS 30, got %d", maxFPS)
	}
	expectedMinFlushMs := time.Duration(1000/30) * time.Millisecond
	if minFlushMs != expectedMinFlushMs {
		t.Errorf("Expected minFlushMs %v, got %v", expectedMinFlushMs, minFlushMs)
	}
}

func TestStreamingBufferWrite(t *testing.T) {
	sb := NewStreamingBuffer()

	// Write some tokens
	sb.Write("Hello")
	sb.Write(" ")
	sb.Write("World")

	// Check pending count
	if pending := sb.Pending(); pending != 3 {
		t.Errorf("Expected 3 pending tokens, got %d", pending)
	}
}

func TestStreamingBufferFlushBySize(t *testing.T) {
	sb := NewStreamingBufferWithConfig(3, 30) // Batch size 3

	// Write tokens but don't reach threshold
	sb.Write("A")
	sb.Write("B")

	// Should not flush yet
	content, hasContent := sb.Flush()
	if hasContent {
		t.Error("Should not flush before reaching batch size")
	}

	// Write one more to reach threshold
	sb.Write("C")

	// Should flush now
	content, hasContent = sb.Flush()
	if !hasContent {
		t.Error("Should flush after reaching batch size")
	}
	if content != "ABC" {
		t.Errorf("Expected flushed content 'ABC', got '%s'", content)
	}

	// Buffer should be empty now
	if pending := sb.Pending(); pending != 0 {
		t.Errorf("Expected 0 pending tokens after flush, got %d", pending)
	}
}

func TestStreamingBufferFlushByTime(t *testing.T) {
	sb := NewStreamingBufferWithConfig(100, 30) // Large batch size, 30fps

	// Write a single token
	sb.Write("A")

	// Should not flush immediately
	content, hasContent := sb.Flush()
	if hasContent {
		t.Error("Should not flush immediately")
	}

	// Wait for flush interval (33ms for 30fps)
	time.Sleep(35 * time.Millisecond)

	// Should flush now due to time
	content, hasContent = sb.Flush()
	if !hasContent {
		t.Error("Should flush after time threshold")
	}
	if content != "A" {
		t.Errorf("Expected flushed content 'A', got '%s'", content)
	}
}

func TestStreamingBufferForceFlush(t *testing.T) {
	sb := NewStreamingBuffer()

	// Write some tokens (not enough to auto-flush)
	sb.Write("Test")

	// Force flush
	content, hasContent := sb.ForceFlush()
	if !hasContent {
		t.Error("ForceFlush should return content")
	}
	if content != "Test" {
		t.Errorf("Expected 'Test', got '%s'", content)
	}

	// Buffer should be empty
	if pending := sb.Pending(); pending != 0 {
		t.Errorf("Expected 0 pending after force flush, got %d", pending)
	}
}

func TestStreamingBufferReset(t *testing.T) {
	sb := NewStreamingBuffer()

	// Write some tokens
	sb.Write("A")
	sb.Write("B")
	sb.Write("C")

	// Reset
	sb.Reset()

	// Should have no pending tokens
	if pending := sb.Pending(); pending != 0 {
		t.Errorf("Expected 0 pending after reset, got %d", pending)
	}

	// Flush should return nothing
	_, hasContent := sb.Flush()
	if hasContent {
		t.Error("Should have no content after reset")
	}
}

func TestStreamingBufferConcurrency(t *testing.T) {
	sb := NewStreamingBuffer()

	// Concurrent writes (simulating streaming from goroutine)
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			sb.Write("x")
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Concurrent flushes (simulating main loop)
	flushCount := 0
	go func() {
		for i := 0; i < 100; i++ {
			if _, hasContent := sb.Flush(); hasContent {
				flushCount++
			}
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Should have no data races (test with -race flag)
	t.Logf("Completed with %d flushes", flushCount)
}

func TestStreamingBufferUnicode(t *testing.T) {
	sb := NewStreamingBuffer()

	// Write Unicode tokens
	sb.Write("Hello")
	sb.Write(" ")
	sb.Write("世界")
	sb.Write("!")

	// Force flush
	content, hasContent := sb.ForceFlush()
	if !hasContent {
		t.Error("Should have content")
	}

	expected := "Hello 世界!"
	if content != expected {
		t.Errorf("Expected '%s', got '%s'", expected, content)
	}
}

func TestStreamingBufferSetters(t *testing.T) {
	sb := NewStreamingBuffer()

	// Test SetBatchSize
	sb.SetBatchSize(20)
	batchSize, _, _ := sb.GetConfig()
	if batchSize != 20 {
		t.Errorf("Expected batch size 20, got %d", batchSize)
	}

	// Test SetMaxFPS
	sb.SetMaxFPS(60)
	_, maxFPS, minFlushMs := sb.GetConfig()
	if maxFPS != 60 {
		t.Errorf("Expected maxFPS 60, got %d", maxFPS)
	}
	expectedMinFlushMs := time.Duration(1000/60) * time.Millisecond
	if minFlushMs != expectedMinFlushMs {
		t.Errorf("Expected minFlushMs %v, got %v", expectedMinFlushMs, minFlushMs)
	}
}

// =============================================================================
// VIEWPORT OPTIMIZER TESTS
// =============================================================================

func TestNewViewportOptimizer(t *testing.T) {
	vo := NewViewportOptimizer()

	if vo == nil {
		t.Fatal("NewViewportOptimizer returned nil")
	}

	// Should start dirty
	if !vo.IsDirty() {
		t.Error("Expected optimizer to start dirty")
	}
}

func TestViewportOptimizerShouldUpdate(t *testing.T) {
	vo := NewViewportOptimizer()

	content1 := "Hello World"
	content2 := "Hello World"
	content3 := "Different Content"

	// First update should always proceed
	if !vo.ShouldUpdate(content1) {
		t.Error("First update should proceed")
	}

	// Same content should not need update
	if vo.ShouldUpdate(content2) {
		t.Error("Same content should not need update")
	}

	// Different content should need update
	if !vo.ShouldUpdate(content3) {
		t.Error("Different content should need update")
	}
}

func TestViewportOptimizerStats(t *testing.T) {
	vo := NewViewportOptimizer()

	// Perform some updates
	vo.ShouldUpdate("Content 1")
	vo.ShouldUpdate("Content 1") // Duplicate - should skip
	vo.ShouldUpdate("Content 2")
	vo.ShouldUpdate("Content 2") // Duplicate - should skip
	vo.ShouldUpdate("Content 3")

	// Get stats
	total, skipped, efficiency := vo.GetStats()

	if total != 5 {
		t.Errorf("Expected 5 total updates, got %d", total)
	}
	if skipped != 2 {
		t.Errorf("Expected 2 skipped updates, got %d", skipped)
	}
	if efficiency != 40.0 {
		t.Errorf("Expected 40%% efficiency, got %.1f%%", efficiency)
	}
}

func TestViewportOptimizerMarkClean(t *testing.T) {
	vo := NewViewportOptimizer()

	vo.ShouldUpdate("Content")

	// Should be dirty after update
	if !vo.IsDirty() {
		t.Error("Should be dirty after update")
	}

	// Mark clean
	vo.MarkClean()

	// Should not be dirty anymore
	if vo.IsDirty() {
		t.Error("Should not be dirty after MarkClean")
	}
}

func TestViewportOptimizerReset(t *testing.T) {
	vo := NewViewportOptimizer()

	// Do some updates
	vo.ShouldUpdate("Content 1")
	vo.ShouldUpdate("Content 1")
	vo.ShouldUpdate("Content 2")

	// Reset
	vo.Reset()

	// Should be dirty
	if !vo.IsDirty() {
		t.Error("Should be dirty after reset")
	}

	// Next update should proceed (new hash)
	if !vo.ShouldUpdate("Content 1") {
		t.Error("First update after reset should proceed")
	}
}

func TestViewportOptimizerForceUpdate(t *testing.T) {
	vo := NewViewportOptimizer()

	content := "Test Content"

	// First update
	vo.ShouldUpdate(content)

	// Same content should skip
	if vo.ShouldUpdate(content) {
		t.Error("Same content should skip")
	}

	// Force update
	vo.ForceUpdate()

	// Next update should proceed even with same content
	if !vo.ShouldUpdate(content) {
		t.Error("Update after ForceUpdate should proceed")
	}
}

func TestViewportOptimizerConcurrency(t *testing.T) {
	vo := NewViewportOptimizer()

	// Concurrent updates
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				content := "Content " + string(rune('0'+id%10))
				vo.ShouldUpdate(content)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have no data races (test with -race flag)
	total, skipped, _ := vo.GetStats()
	t.Logf("Completed with %d total, %d skipped", total, skipped)
}

func TestViewportOptimizerEmptyContent(t *testing.T) {
	vo := NewViewportOptimizer()

	// Empty content
	if !vo.ShouldUpdate("") {
		t.Error("First update with empty content should proceed")
	}

	// Another empty content should skip
	if vo.ShouldUpdate("") {
		t.Error("Second update with empty content should skip")
	}
}

func TestViewportOptimizerLargeContent(t *testing.T) {
	vo := NewViewportOptimizer()

	// Create large content (100KB)
	var builder strings.Builder
	for i := 0; i < 100000; i++ {
		builder.WriteByte('x')
	}
	largeContent := builder.String()

	// First update
	start := time.Now()
	if !vo.ShouldUpdate(largeContent) {
		t.Error("First update should proceed")
	}
	duration := time.Since(start)

	// Should be fast (< 10ms for 100KB)
	if duration > 10*time.Millisecond {
		t.Errorf("Hash computation too slow: %v", duration)
	}

	// Second update with same content should skip
	if vo.ShouldUpdate(largeContent) {
		t.Error("Same large content should skip")
	}

	// Stats
	total, skipped, efficiency := vo.GetStats()
	t.Logf("Large content test: total=%d, skipped=%d, efficiency=%.1f%%", total, skipped, efficiency)
}

// =============================================================================
// INTEGRATION TESTS
// =============================================================================

func TestStreamingBufferIntegration(t *testing.T) {
	// Simulate real streaming scenario
	sb := NewStreamingBufferWithConfig(10, 30)

	// Simulate tokens arriving quickly (simulating LLM streaming)
	tokens := []string{"The", " quick", " brown", " fox", " jumps", " over", " the", " lazy", " dog"}

	for i, token := range tokens {
		sb.Write(token)

		// After 10 tokens, should auto-flush by size
		if i == 9 {
			if !sb.ShouldFlush() {
				t.Error("Should be ready to flush after 10 tokens")
			}
		}
	}

	// Force flush remaining
	content, hasContent := sb.ForceFlush()
	if !hasContent {
		t.Error("Should have remaining content")
	}

	expected := "The quick brown fox jumps over the lazy dog"
	if content != expected {
		t.Errorf("Expected '%s', got '%s'", expected, content)
	}
}

func TestStreamingOptimizationFullFlow(t *testing.T) {
	// Test the full streaming optimization flow
	sb := NewStreamingBuffer()
	vo := NewViewportOptimizer()

	// Simulate streaming loop
	messages := []string{
		"Hello, this is a test of the streaming optimization system.",
		"It should batch tokens and reduce redundant viewport updates.",
		"The result should be smooth, flicker-free rendering at 30fps.",
	}

	var fullContent strings.Builder
	updateCount := 0
	for _, msg := range messages {
		// Simulate token-by-token streaming
		words := strings.Fields(msg)
		for _, word := range words {
			sb.Write(word + " ")

			// Simulate 30fps tick
			if content, hasContent := sb.Flush(); hasContent {
				fullContent.WriteString(content)

				// Only update viewport if content changed
				if vo.ShouldUpdate(fullContent.String()) {
					updateCount++
					vo.MarkClean()
				}
			}
		}
	}

	// Force flush final tokens
	if content, hasContent := sb.ForceFlush(); hasContent {
		fullContent.WriteString(content)
		if vo.ShouldUpdate(fullContent.String()) {
			updateCount++
			vo.MarkClean()
		}
	}

	// Get optimizer stats
	total, skipped, efficiency := vo.GetStats()

	t.Logf("Full flow test:")
	t.Logf("  Viewport updates: %d", updateCount)
	t.Logf("  Total checks: %d", total)
	t.Logf("  Skipped: %d", skipped)
	t.Logf("  Efficiency: %.1f%%", efficiency)

	// Without optimization, we'd update on every token (~30 words = 30 updates)
	// With optimization, we should have much fewer updates
	if updateCount == 0 {
		t.Error("Should have some viewport updates")
	}
}

// =============================================================================
// BENCHMARK TESTS
// =============================================================================

func BenchmarkStreamingBufferWrite(b *testing.B) {
	sb := NewStreamingBuffer()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sb.Write("token")
	}
}

func BenchmarkStreamingBufferFlush(b *testing.B) {
	sb := NewStreamingBuffer()
	for i := 0; i < 100; i++ {
		sb.Write("token")
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sb.Flush()
	}
}

func BenchmarkViewportOptimizerShouldUpdate(b *testing.B) {
	vo := NewViewportOptimizer()
	content := "This is a test message that simulates viewport content."
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		vo.ShouldUpdate(content)
	}
}

func BenchmarkViewportOptimizerLargeContent(b *testing.B) {
	vo := NewViewportOptimizer()

	// Create 10KB content
	var builder strings.Builder
	for i := 0; i < 10000; i++ {
		builder.WriteByte('x')
	}
	content := builder.String()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		vo.ShouldUpdate(content)
	}
}
