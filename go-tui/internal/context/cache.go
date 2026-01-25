// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package context provides the @ mention system for including context in messages.
package context

import (
	"os"
	"sync"
	"time"
)

// =============================================================================
// FILE CACHE
// =============================================================================

// FileCache provides LRU caching for file reads to speed up @file mentions.
// Cache invalidates when file modification time changes.
type FileCache struct {
	mu         sync.RWMutex
	cache      map[string]*FileCacheEntry
	maxEntries int
	maxSize    int64 // Total bytes limit
	currentSize int64
	accessOrder []string // For LRU eviction

	// Statistics
	hits   int
	misses int
}

// FileCacheEntry represents a cached file.
type FileCacheEntry struct {
	Path       string
	Content    string
	ModTime    time.Time
	Size       int64
	CachedAt   time.Time
	AccessedAt time.Time
	LineCount  int
}

// FileCacheStats holds cache statistics.
type FileCacheStats struct {
	Hits        int
	Misses      int
	EntryCount  int
	TotalSize   int64
	MaxSize     int64
	HitRate     float64
}

// NewFileCache creates a new file cache with the given limits.
// maxEntries: maximum number of cached files (default: 100)
// maxSize: maximum total bytes (default: 100MB)
func NewFileCache(maxEntries int, maxSize int64) *FileCache {
	if maxEntries <= 0 {
		maxEntries = 100
	}
	if maxSize <= 0 {
		maxSize = 100 * 1024 * 1024 // 100MB
	}
	return &FileCache{
		cache:      make(map[string]*FileCacheEntry),
		maxEntries: maxEntries,
		maxSize:    maxSize,
		accessOrder: make([]string, 0, maxEntries),
	}
}

// DefaultFileCache is the global file cache instance.
var DefaultFileCache = NewFileCache(100, 100*1024*1024)

// Get retrieves a file from the cache if valid.
// Returns the cached content, line count, and whether it was a cache hit.
func (fc *FileCache) Get(path string) (string, int, bool) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	entry, ok := fc.cache[path]
	if !ok {
		fc.misses++
		return "", 0, false
	}

	// Check if file has been modified
	info, err := os.Stat(path)
	if err != nil {
		// File no longer exists, remove from cache
		fc.removeEntryLocked(path)
		fc.misses++
		return "", 0, false
	}

	if info.ModTime().After(entry.ModTime) {
		// File modified, invalidate cache
		fc.removeEntryLocked(path)
		fc.misses++
		return "", 0, false
	}

	// Cache hit - update access time and order
	entry.AccessedAt = time.Now()
	fc.updateAccessOrderLocked(path)
	fc.hits++

	return entry.Content, entry.LineCount, true
}

// Put adds a file to the cache.
func (fc *FileCache) Put(path string, content string, modTime time.Time, lineCount int) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	contentSize := int64(len(content))

	// Don't cache files larger than 10% of max size
	if contentSize > fc.maxSize/10 {
		return
	}

	// Evict entries if needed
	for fc.currentSize+contentSize > fc.maxSize || len(fc.cache) >= fc.maxEntries {
		if len(fc.accessOrder) == 0 {
			break
		}
		// Evict least recently used
		oldest := fc.accessOrder[0]
		fc.removeEntryLocked(oldest)
	}

	// Remove existing entry if updating
	if existing, ok := fc.cache[path]; ok {
		fc.currentSize -= existing.Size
	}

	// Add new entry
	entry := &FileCacheEntry{
		Path:       path,
		Content:    content,
		ModTime:    modTime,
		Size:       contentSize,
		CachedAt:   time.Now(),
		AccessedAt: time.Now(),
		LineCount:  lineCount,
	}

	fc.cache[path] = entry
	fc.currentSize += contentSize
	fc.updateAccessOrderLocked(path)
}

// Invalidate removes a file from the cache.
func (fc *FileCache) Invalidate(path string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.removeEntryLocked(path)
}

// Clear removes all entries from the cache.
func (fc *FileCache) Clear() {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.cache = make(map[string]*FileCacheEntry)
	fc.accessOrder = make([]string, 0, fc.maxEntries)
	fc.currentSize = 0
}

// Stats returns cache statistics.
func (fc *FileCache) Stats() FileCacheStats {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	hitRate := 0.0
	total := fc.hits + fc.misses
	if total > 0 {
		hitRate = float64(fc.hits) / float64(total)
	}

	return FileCacheStats{
		Hits:       fc.hits,
		Misses:     fc.misses,
		EntryCount: len(fc.cache),
		TotalSize:  fc.currentSize,
		MaxSize:    fc.maxSize,
		HitRate:    hitRate,
	}
}

// removeEntryLocked removes an entry (must hold lock).
func (fc *FileCache) removeEntryLocked(path string) {
	entry, ok := fc.cache[path]
	if !ok {
		return
	}

	fc.currentSize -= entry.Size
	delete(fc.cache, path)

	// Remove from access order
	for i, p := range fc.accessOrder {
		if p == path {
			fc.accessOrder = append(fc.accessOrder[:i], fc.accessOrder[i+1:]...)
			break
		}
	}
}

// updateAccessOrderLocked updates LRU order (must hold lock).
func (fc *FileCache) updateAccessOrderLocked(path string) {
	// Remove existing position
	for i, p := range fc.accessOrder {
		if p == path {
			fc.accessOrder = append(fc.accessOrder[:i], fc.accessOrder[i+1:]...)
			break
		}
	}
	// Add to end (most recently used)
	fc.accessOrder = append(fc.accessOrder, path)
}
