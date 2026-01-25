// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package context

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// TEST HELPERS
// =============================================================================

// createTempFile creates a temporary file with content and returns the path.
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile.txt")
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	return tmpFile
}

// updateTempFile updates the content of a file and ensures modTime changes.
func updateTempFile(t *testing.T, path string, content string) {
	t.Helper()
	// Sleep briefly to ensure modTime is different
	time.Sleep(10 * time.Millisecond)
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to update temp file: %v", err)
	}
}

// =============================================================================
// BASIC OPERATIONS TESTS
// =============================================================================

func TestNewFileCache(t *testing.T) {
	tests := []struct {
		name           string
		maxEntries     int
		maxSize        int64
		expectedEntries int
		expectedSize   int64
	}{
		{
			name:           "default values when zero",
			maxEntries:     0,
			maxSize:        0,
			expectedEntries: 100,
			expectedSize:   100 * 1024 * 1024,
		},
		{
			name:           "default values when negative",
			maxEntries:     -1,
			maxSize:        -1,
			expectedEntries: 100,
			expectedSize:   100 * 1024 * 1024,
		},
		{
			name:           "custom values",
			maxEntries:     50,
			maxSize:        1024,
			expectedEntries: 50,
			expectedSize:   1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewFileCache(tt.maxEntries, tt.maxSize)
			if cache.maxEntries != tt.expectedEntries {
				t.Errorf("Expected maxEntries=%d, got %d", tt.expectedEntries, cache.maxEntries)
			}
			if cache.maxSize != tt.expectedSize {
				t.Errorf("Expected maxSize=%d, got %d", tt.expectedSize, cache.maxSize)
			}
			if cache.cache == nil {
				t.Error("Cache map not initialized")
			}
			if cache.accessOrder == nil {
				t.Error("Access order not initialized")
			}
			if cache.currentSize != 0 {
				t.Errorf("Expected currentSize=0, got %d", cache.currentSize)
			}
		})
	}
}

func TestFileCacheBasicOperations(t *testing.T) {
	cache := NewFileCache(10, 1024*1024)
	tmpFile := createTempFile(t, "test content")

	// Test: Cache miss on empty cache
	content, lineCount, hit := cache.Get(tmpFile)
	if hit {
		t.Error("Expected cache miss on empty cache")
	}
	if content != "" {
		t.Error("Expected empty content on cache miss")
	}
	if lineCount != 0 {
		t.Error("Expected zero line count on cache miss")
	}

	// Test: Put entry into cache
	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	cache.Put(tmpFile, "test content", info.ModTime(), 1)

	// Test: Cache hit after put
	content, lineCount, hit = cache.Get(tmpFile)
	if !hit {
		t.Error("Expected cache hit after put")
	}
	if content != "test content" {
		t.Errorf("Expected content='test content', got '%s'", content)
	}
	if lineCount != 1 {
		t.Errorf("Expected lineCount=1, got %d", lineCount)
	}

	// Test: Clear cache
	cache.Clear()
	content, lineCount, hit = cache.Get(tmpFile)
	if hit {
		t.Error("Expected cache miss after clear")
	}
	if content != "" {
		t.Error("Expected empty content after clear")
	}

	// Verify stats after clear
	stats := cache.Stats()
	if stats.EntryCount != 0 {
		t.Errorf("Expected EntryCount=0 after clear, got %d", stats.EntryCount)
	}
	if stats.TotalSize != 0 {
		t.Errorf("Expected TotalSize=0 after clear, got %d", stats.TotalSize)
	}
}

// =============================================================================
// INVALIDATION TESTS
// =============================================================================

func TestFileCacheInvalidationOnModTime(t *testing.T) {
	cache := NewFileCache(10, 1024*1024)
	tmpFile := createTempFile(t, "original content")

	// Cache the original file
	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	cache.Put(tmpFile, "original content", info.ModTime(), 1)

	// Verify cache hit
	_, _, hit := cache.Get(tmpFile)
	if !hit {
		t.Error("Expected cache hit for original file")
	}

	// Update the file (modTime changes)
	updateTempFile(t, tmpFile, "modified content")

	// Verify cache miss after modification
	content, lineCount, hit := cache.Get(tmpFile)
	if hit {
		t.Error("Expected cache miss after file modification")
	}
	if content != "" {
		t.Error("Expected empty content after invalidation")
	}
	if lineCount != 0 {
		t.Error("Expected zero line count after invalidation")
	}

	// Verify entry was removed from cache
	stats := cache.Stats()
	if stats.EntryCount != 0 {
		t.Errorf("Expected EntryCount=0 after invalidation, got %d", stats.EntryCount)
	}
}

func TestFileCacheInvalidationOnDelete(t *testing.T) {
	cache := NewFileCache(10, 1024*1024)
	tmpFile := createTempFile(t, "test content")

	// Cache the file
	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	cache.Put(tmpFile, "test content", info.ModTime(), 1)

	// Verify cache hit
	_, _, hit := cache.Get(tmpFile)
	if !hit {
		t.Error("Expected cache hit")
	}

	// Delete the file
	err = os.Remove(tmpFile)
	if err != nil {
		t.Fatalf("Failed to delete file: %v", err)
	}

	// Verify cache miss after deletion
	content, lineCount, hit := cache.Get(tmpFile)
	if hit {
		t.Error("Expected cache miss after file deletion")
	}
	if content != "" {
		t.Error("Expected empty content after deletion")
	}
	if lineCount != 0 {
		t.Error("Expected zero line count after deletion")
	}

	// Verify entry was removed from cache
	stats := cache.Stats()
	if stats.EntryCount != 0 {
		t.Errorf("Expected EntryCount=0 after deletion, got %d", stats.EntryCount)
	}
}

func TestFileCacheManualInvalidation(t *testing.T) {
	cache := NewFileCache(10, 1024*1024)
	tmpFile := createTempFile(t, "test content")

	// Cache the file
	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	cache.Put(tmpFile, "test content", info.ModTime(), 1)

	// Verify cache hit
	_, _, hit := cache.Get(tmpFile)
	if !hit {
		t.Error("Expected cache hit")
	}

	// Manually invalidate
	cache.Invalidate(tmpFile)

	// Verify cache miss after invalidation
	_, _, hit = cache.Get(tmpFile)
	if hit {
		t.Error("Expected cache miss after manual invalidation")
	}

	// Verify stats
	stats := cache.Stats()
	if stats.EntryCount != 0 {
		t.Errorf("Expected EntryCount=0 after invalidation, got %d", stats.EntryCount)
	}
	if stats.TotalSize != 0 {
		t.Errorf("Expected TotalSize=0 after invalidation, got %d", stats.TotalSize)
	}
}

// =============================================================================
// LRU EVICTION TESTS
// =============================================================================

func TestFileCacheLRUEviction(t *testing.T) {
	cache := NewFileCache(3, 1024*1024) // Max 3 entries

	// Create 4 temp files
	files := make([]string, 4)
	for i := 0; i < 4; i++ {
		files[i] = createTempFile(t, "content")
	}

	// Add first 3 files to cache
	for i := 0; i < 3; i++ {
		info, err := os.Stat(files[i])
		if err != nil {
			t.Fatalf("Failed to stat file: %v", err)
		}
		cache.Put(files[i], "content", info.ModTime(), 1)
	}

	// Verify all 3 are cached
	stats := cache.Stats()
	if stats.EntryCount != 3 {
		t.Errorf("Expected EntryCount=3, got %d", stats.EntryCount)
	}

	// Add 4th file - should evict the first (LRU)
	info, err := os.Stat(files[3])
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	cache.Put(files[3], "content", info.ModTime(), 1)

	// Verify still 3 entries
	stats = cache.Stats()
	if stats.EntryCount != 3 {
		t.Errorf("Expected EntryCount=3 after eviction, got %d", stats.EntryCount)
	}

	// First file should be evicted
	_, _, hit := cache.Get(files[0])
	if hit {
		t.Error("Expected cache miss for evicted file (files[0])")
	}

	// Other files should still be cached
	for i := 1; i < 4; i++ {
		_, _, hit := cache.Get(files[i])
		if !hit {
			t.Errorf("Expected cache hit for files[%d]", i)
		}
	}
}

func TestFileCacheLRUEvictionWithAccess(t *testing.T) {
	cache := NewFileCache(3, 1024*1024) // Max 3 entries

	// Create 4 temp files
	files := make([]string, 4)
	for i := 0; i < 4; i++ {
		files[i] = createTempFile(t, "content")
	}

	// Add first 3 files to cache
	for i := 0; i < 3; i++ {
		info, err := os.Stat(files[i])
		if err != nil {
			t.Fatalf("Failed to stat file: %v", err)
		}
		cache.Put(files[i], "content", info.ModTime(), 1)
	}

	// Access first file to make it most recently used
	cache.Get(files[0])

	// Add 4th file - should evict files[1] (now LRU), not files[0]
	info, err := os.Stat(files[3])
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	cache.Put(files[3], "content", info.ModTime(), 1)

	// files[1] should be evicted
	_, _, hit := cache.Get(files[1])
	if hit {
		t.Error("Expected cache miss for evicted file (files[1])")
	}

	// files[0], files[2], files[3] should still be cached
	for _, idx := range []int{0, 2, 3} {
		_, _, hit := cache.Get(files[idx])
		if !hit {
			t.Errorf("Expected cache hit for files[%d]", idx)
		}
	}
}

// =============================================================================
// SIZE LIMIT TESTS
// =============================================================================

func TestFileCacheSizeLimit(t *testing.T) {
	// Cache with 10KB limit
	cache := NewFileCache(100, 10*1024)

	// Create a file with 600 bytes (< 10% of 10KB = 1024 bytes)
	content1 := make([]byte, 600)
	for i := range content1 {
		content1[i] = 'A'
	}
	file1 := createTempFile(t, string(content1))

	// Create a file with 500 bytes (< 10% of 10KB)
	content2 := make([]byte, 500)
	for i := range content2 {
		content2[i] = 'B'
	}
	file2 := createTempFile(t, string(content2))

	// Fill cache with multiple 600-byte files to approach size limit
	// Total: 600 * 17 = 10200 bytes (exceeds 10KB limit)
	files := make([]string, 17)
	files[0] = file1
	for i := 0; i < 17; i++ {
		if i == 0 {
			info1, _ := os.Stat(file1)
			cache.Put(file1, string(content1), info1.ModTime(), 1)
		} else {
			tmpFile := createTempFile(t, string(content1))
			info, _ := os.Stat(tmpFile)
			cache.Put(tmpFile, string(content1), info.ModTime(), 1)
			files[i] = tmpFile
		}
	}

	// Cache should have evicted early entries to stay under size limit
	stats := cache.Stats()
	if stats.TotalSize > 10*1024 {
		t.Errorf("Expected TotalSize <= 10240, got %d", stats.TotalSize)
	}

	// Add second file - may evict more entries
	info2, _ := os.Stat(file2)
	cache.Put(file2, string(content2), info2.ModTime(), 1)

	stats = cache.Stats()
	if stats.TotalSize > 10*1024 {
		t.Errorf("Expected TotalSize <= 10240 after adding file2, got %d", stats.TotalSize)
	}

	// Second file should be cached (most recently added)
	_, _, hit := cache.Get(file2)
	if !hit {
		t.Error("Expected cache hit for most recently added file")
	}
}

func TestFileCacheLargeFile(t *testing.T) {
	// Cache with 1KB limit
	cache := NewFileCache(100, 1000)

	// Create a file larger than 10% of maxSize (>100 bytes)
	largeContent := make([]byte, 150)
	for i := range largeContent {
		largeContent[i] = 'X'
	}
	largeFile := createTempFile(t, string(largeContent))

	// Try to cache large file - should be rejected
	info, _ := os.Stat(largeFile)
	cache.Put(largeFile, string(largeContent), info.ModTime(), 1)

	// Verify file was not cached
	stats := cache.Stats()
	if stats.EntryCount != 0 {
		t.Error("Expected large file not to be cached")
	}
	if stats.TotalSize != 0 {
		t.Error("Expected TotalSize=0 when large file rejected")
	}

	// Verify cache miss
	_, _, hit := cache.Get(largeFile)
	if hit {
		t.Error("Expected cache miss for large file")
	}
}

func TestFileCacheEntryLimit(t *testing.T) {
	cache := NewFileCache(2, 1024*1024) // Max 2 entries

	// Create 3 small files
	files := make([]string, 3)
	for i := 0; i < 3; i++ {
		files[i] = createTempFile(t, "x") // 1 byte each
	}

	// Add first 2 files
	for i := 0; i < 2; i++ {
		info, _ := os.Stat(files[i])
		cache.Put(files[i], "x", info.ModTime(), 1)
	}

	stats := cache.Stats()
	if stats.EntryCount != 2 {
		t.Errorf("Expected EntryCount=2, got %d", stats.EntryCount)
	}

	// Add 3rd file - should evict first
	info, _ := os.Stat(files[2])
	cache.Put(files[2], "x", info.ModTime(), 1)

	stats = cache.Stats()
	if stats.EntryCount != 2 {
		t.Errorf("Expected EntryCount=2 after eviction, got %d", stats.EntryCount)
	}

	// First file should be evicted
	_, _, hit := cache.Get(files[0])
	if hit {
		t.Error("Expected cache miss for evicted file")
	}
}

// =============================================================================
// STATISTICS TESTS
// =============================================================================

func TestFileCacheStatistics(t *testing.T) {
	cache := NewFileCache(10, 1024*1024)
	tmpFile := createTempFile(t, "test content")

	// Initial stats
	stats := cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Error("Expected zero hits and misses initially")
	}
	if stats.HitRate != 0.0 {
		t.Error("Expected zero hit rate initially")
	}

	// First get - should be a miss
	cache.Get(tmpFile)
	stats = cache.Stats()
	if stats.Hits != 0 {
		t.Errorf("Expected Hits=0, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected Misses=1, got %d", stats.Misses)
	}
	if stats.HitRate != 0.0 {
		t.Errorf("Expected HitRate=0.0, got %f", stats.HitRate)
	}

	// Put and get - should be a hit
	info, _ := os.Stat(tmpFile)
	cache.Put(tmpFile, "test content", info.ModTime(), 1)
	cache.Get(tmpFile)

	stats = cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected Hits=1, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected Misses=1, got %d", stats.Misses)
	}
	if stats.HitRate != 0.5 {
		t.Errorf("Expected HitRate=0.5, got %f", stats.HitRate)
	}

	// Another hit
	cache.Get(tmpFile)
	stats = cache.Stats()
	if stats.Hits != 2 {
		t.Errorf("Expected Hits=2, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected Misses=1, got %d", stats.Misses)
	}
	expectedHitRate := 2.0 / 3.0
	if stats.HitRate != expectedHitRate {
		t.Errorf("Expected HitRate=%f, got %f", expectedHitRate, stats.HitRate)
	}

	// Verify size stats
	if stats.EntryCount != 1 {
		t.Errorf("Expected EntryCount=1, got %d", stats.EntryCount)
	}
	if stats.TotalSize != int64(len("test content")) {
		t.Errorf("Expected TotalSize=%d, got %d", len("test content"), stats.TotalSize)
	}
	if stats.MaxSize != 1024*1024 {
		t.Errorf("Expected MaxSize=%d, got %d", 1024*1024, stats.MaxSize)
	}
}

func TestFileCacheStatsAfterInvalidation(t *testing.T) {
	cache := NewFileCache(10, 1024*1024)
	tmpFile := createTempFile(t, "original")

	// Cache and access the file
	info, _ := os.Stat(tmpFile)
	cache.Put(tmpFile, "original", info.ModTime(), 1)
	cache.Get(tmpFile) // Hit

	// Update file to invalidate cache
	updateTempFile(t, tmpFile, "modified")
	cache.Get(tmpFile) // Miss due to invalidation

	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected Hits=1, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected Misses=1 after invalidation, got %d", stats.Misses)
	}
}

// =============================================================================
// CONCURRENCY TESTS
// =============================================================================

func TestFileCacheConcurrency(t *testing.T) {
	cache := NewFileCache(100, 1024*1024)
	tmpFile := createTempFile(t, "test content")
	info, _ := os.Stat(tmpFile)

	const numGoroutines = 50
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent reads and writes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				if j%3 == 0 {
					cache.Put(tmpFile, "test content", info.ModTime(), 1)
				} else if j%3 == 1 {
					cache.Get(tmpFile)
				} else {
					cache.Stats()
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify cache is still functional
	stats := cache.Stats()
	if stats.EntryCount < 0 || stats.EntryCount > cache.maxEntries {
		t.Errorf("Invalid EntryCount after concurrent access: %d", stats.EntryCount)
	}
	if stats.TotalSize < 0 || stats.TotalSize > cache.maxSize {
		t.Errorf("Invalid TotalSize after concurrent access: %d", stats.TotalSize)
	}
}

func TestFileCacheConcurrentEviction(t *testing.T) {
	cache := NewFileCache(10, 1024*1024)

	// Create multiple temp files
	const numFiles = 20
	files := make([]string, numFiles)
	for i := 0; i < numFiles; i++ {
		files[i] = createTempFile(t, "content")
	}

	var wg sync.WaitGroup
	wg.Add(numFiles)

	// Concurrently add files that will trigger evictions
	for i := 0; i < numFiles; i++ {
		go func(idx int) {
			defer wg.Done()
			info, err := os.Stat(files[idx])
			if err == nil {
				cache.Put(files[idx], "content", info.ModTime(), 1)
			}
		}(i)
	}

	wg.Wait()

	// Verify cache constraints are maintained
	stats := cache.Stats()
	if stats.EntryCount > cache.maxEntries {
		t.Errorf("EntryCount exceeded limit: %d > %d", stats.EntryCount, cache.maxEntries)
	}
	if stats.TotalSize > cache.maxSize {
		t.Errorf("TotalSize exceeded limit: %d > %d", stats.TotalSize, cache.maxSize)
	}
}

func TestFileCacheConcurrentClear(t *testing.T) {
	cache := NewFileCache(10, 1024*1024)
	tmpFile := createTempFile(t, "test content")
	info, _ := os.Stat(tmpFile)

	var wg sync.WaitGroup
	wg.Add(3)

	// Concurrent operations with clear
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			cache.Put(tmpFile, "test content", info.ModTime(), 1)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			cache.Get(tmpFile)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Millisecond)
			cache.Clear()
		}
	}()

	wg.Wait()

	// Verify cache is in valid state
	stats := cache.Stats()
	if stats.TotalSize < 0 {
		t.Error("TotalSize became negative")
	}
	if stats.EntryCount < 0 {
		t.Error("EntryCount became negative")
	}
}

// =============================================================================
// EDGE CASES
// =============================================================================

func TestFileCacheUpdateExistingEntry(t *testing.T) {
	cache := NewFileCache(10, 1024*1024)
	tmpFile := createTempFile(t, "original")

	// Cache original content
	info, _ := os.Stat(tmpFile)
	cache.Put(tmpFile, "original", info.ModTime(), 1)

	stats := cache.Stats()
	originalSize := stats.TotalSize

	// Update with larger content
	cache.Put(tmpFile, "updated content is longer", info.ModTime(), 2)

	stats = cache.Stats()
	if stats.EntryCount != 1 {
		t.Errorf("Expected EntryCount=1, got %d", stats.EntryCount)
	}
	if stats.TotalSize == originalSize {
		t.Error("Expected TotalSize to change after update")
	}
	if stats.TotalSize != int64(len("updated content is longer")) {
		t.Errorf("Expected TotalSize=%d, got %d", len("updated content is longer"), stats.TotalSize)
	}

	// Verify updated content
	content, lineCount, hit := cache.Get(tmpFile)
	if !hit {
		t.Error("Expected cache hit for updated entry")
	}
	if content != "updated content is longer" {
		t.Errorf("Expected updated content, got '%s'", content)
	}
	if lineCount != 2 {
		t.Errorf("Expected lineCount=2, got %d", lineCount)
	}
}

func TestFileCacheEmptyContent(t *testing.T) {
	cache := NewFileCache(10, 1024*1024)
	tmpFile := createTempFile(t, "")

	// Cache empty file
	info, _ := os.Stat(tmpFile)
	cache.Put(tmpFile, "", info.ModTime(), 0)

	// Verify it's cached
	content, lineCount, hit := cache.Get(tmpFile)
	if !hit {
		t.Error("Expected cache hit for empty file")
	}
	if content != "" {
		t.Errorf("Expected empty content, got '%s'", content)
	}
	if lineCount != 0 {
		t.Errorf("Expected lineCount=0, got %d", lineCount)
	}

	stats := cache.Stats()
	if stats.EntryCount != 1 {
		t.Errorf("Expected EntryCount=1, got %d", stats.EntryCount)
	}
	if stats.TotalSize != 0 {
		t.Errorf("Expected TotalSize=0 for empty file, got %d", stats.TotalSize)
	}
}

func TestFileCacheMultilineContent(t *testing.T) {
	cache := NewFileCache(10, 1024*1024)
	content := "line 1\nline 2\nline 3\n"
	tmpFile := createTempFile(t, content)

	// Cache multiline content
	info, _ := os.Stat(tmpFile)
	cache.Put(tmpFile, content, info.ModTime(), 3)

	// Verify cached content preserves newlines
	cachedContent, lineCount, hit := cache.Get(tmpFile)
	if !hit {
		t.Error("Expected cache hit")
	}
	if cachedContent != content {
		t.Errorf("Expected content to match, got '%s'", cachedContent)
	}
	if lineCount != 3 {
		t.Errorf("Expected lineCount=3, got %d", lineCount)
	}
}

func TestFileCacheInvalidateNonexistent(t *testing.T) {
	cache := NewFileCache(10, 1024*1024)

	// Invalidating nonexistent entry should not panic or error
	cache.Invalidate("/nonexistent/file.txt")

	stats := cache.Stats()
	if stats.EntryCount != 0 {
		t.Error("Expected EntryCount=0 after invalidating nonexistent entry")
	}
}

func TestFileCacheGetNonexistentPath(t *testing.T) {
	cache := NewFileCache(10, 1024*1024)

	// Get on nonexistent path should return cache miss
	content, lineCount, hit := cache.Get("/nonexistent/file.txt")
	if hit {
		t.Error("Expected cache miss for nonexistent path")
	}
	if content != "" {
		t.Error("Expected empty content")
	}
	if lineCount != 0 {
		t.Error("Expected zero line count")
	}

	stats := cache.Stats()
	if stats.Misses != 1 {
		t.Errorf("Expected Misses=1, got %d", stats.Misses)
	}
}
