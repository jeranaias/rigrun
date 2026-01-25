// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package index provides codebase indexing for fast symbol search.
package index

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// =============================================================================
// FILE WATCHER INTERFACE
// =============================================================================

// FileWatcher is the interface for file watching implementations
type FileWatcher interface {
	// Watch starts watching for file changes
	Watch() error

	// Close stops watching and releases resources
	Close() error
}

// =============================================================================
// FSNOTIFY WATCHER
// =============================================================================

// FsnotifyWatcher implements FileWatcher using fsnotify
type FsnotifyWatcher struct {
	idx      *CodebaseIndex
	watcher  *fsnotify.Watcher
	debounce time.Duration
	mu       sync.Mutex
	pending  map[string]time.Time // File path -> last change time
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewFsnotifyWatcher creates a new fsnotify-based watcher
func NewFsnotifyWatcher(idx *CodebaseIndex, debounce time.Duration) (*FsnotifyWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	fw := &FsnotifyWatcher{
		idx:      idx,
		watcher:  watcher,
		debounce: debounce,
		pending:  make(map[string]time.Time),
		ctx:      ctx,
		cancel:   cancel,
	}

	return fw, nil
}

// Watch starts watching for file changes
func (fw *FsnotifyWatcher) Watch() error {
	// Add root directory and all subdirectories
	if err := fw.addRecursive(fw.idx.root); err != nil {
		return err
	}

	// Start event processing goroutine
	go fw.processEvents()

	// Start debounce timer goroutine
	go fw.processPending()

	return nil
}

// addRecursive adds a directory and all its subdirectories to the watch list
func (fw *FsnotifyWatcher) addRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if !info.IsDir() {
			return nil
		}

		// Skip ignored directories
		if fw.idx.shouldIgnore(filepath.Base(path)) {
			return filepath.SkipDir
		}

		// Add directory to watcher
		if err := fw.watcher.Add(path); err != nil {
			// Non-fatal, continue
			return nil
		}

		return nil
	})
}

// processEvents processes file system events
func (fw *FsnotifyWatcher) processEvents() {
	// Add panic recovery to prevent crashes
	defer func() {
		if r := recover(); r != nil {
			// Log panic (non-fatal, goroutine exits)
			// In production, this should use proper logging
			_ = r
		}
	}()

	for {
		select {
		case <-fw.ctx.Done():
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// Handle Write and Create events
			if event.Op&fsnotify.Write == fsnotify.Write ||
			   event.Op&fsnotify.Create == fsnotify.Create {
				fw.handleFileChange(event.Name)
			}

			// Handle Rename events (treat as delete of old name)
			if event.Op&fsnotify.Rename == fsnotify.Rename {
				fw.removeFile(event.Name)
			}

			// Handle Remove events
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				fw.removeFile(event.Name)
			}

			// Handle new directories
			if event.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					// Add directory with retry logic
					if err := fw.addRecursive(event.Name); err != nil {
						// Retry once after a short delay
						time.Sleep(100 * time.Millisecond)
						fw.addRecursive(event.Name)
					}
				}
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			// Log error (non-fatal)
			_ = err
		}
	}
}

// handleFileChange handles a file change event
func (fw *FsnotifyWatcher) handleFileChange(path string) {
	// Check if we should index this file
	ext := filepath.Ext(path)
	if _, ok := fw.idx.parsers[ext]; !ok {
		return
	}

	// Add to pending with debounce
	fw.mu.Lock()
	fw.pending[path] = time.Now()
	fw.mu.Unlock()
}

// processPending processes pending file changes with debounce
func (fw *FsnotifyWatcher) processPending() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-fw.ctx.Done():
			return

		case <-ticker.C:
			now := time.Now()

			fw.mu.Lock()
			var toProcess []string

			for path, changeTime := range fw.pending {
				if now.Sub(changeTime) >= fw.debounce {
					toProcess = append(toProcess, path)
					delete(fw.pending, path)
				}
			}
			fw.mu.Unlock()

			// Process the files
			for _, path := range toProcess {
				fw.updateFile(path)
			}
		}
	}
}

// updateFile incrementally updates a single file in the index
func (fw *FsnotifyWatcher) updateFile(path string) error {
	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		// File might have been deleted, remove from index
		return fw.removeFile(path)
	}

	// Check if file is too large
	if info.Size() > fw.idx.config.MaxFileSize {
		return nil
	}

	// Get parser
	ext := filepath.Ext(path)
	parser, ok := fw.idx.parsers[ext]
	if !ok {
		return nil
	}

	// Get relative path
	relPath, err := filepath.Rel(fw.idx.root, path)
	if err != nil {
		relPath = path
	}

	// Begin transaction
	tx, err := fw.idx.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if file exists in database
	var fileID int64
	err = tx.QueryRow("SELECT id FROM files WHERE path = ?", relPath).Scan(&fileID)

	if err == nil {
		// File exists, delete old symbols
		if _, err := tx.Exec("DELETE FROM symbols WHERE file_id = ?", fileID); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM imports WHERE file_id = ?", fileID); err != nil {
			return err
		}

		// Update file record
		language := fw.idx.detectLanguage(ext)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lineCount := strings.Count(string(content), "\n") + 1

		_, err = tx.Exec(`
			UPDATE files
			SET mod_time = ?, size = ?, language = ?, line_count = ?, indexed_at = ?
			WHERE id = ?
		`, info.ModTime().Unix(), info.Size(), language, lineCount, time.Now().Unix(), fileID)
		if err != nil {
			return err
		}
	} else {
		// File doesn't exist, insert it
		if _, err := fw.idx.indexFile(tx, path, info, parser); err != nil {
			// Transaction will be rolled back by deferred Rollback
			return err
		}
	}

	// Commit transaction
	return tx.Commit()
}

// removeFile removes a file from the index
func (fw *FsnotifyWatcher) removeFile(path string) error {
	relPath, err := filepath.Rel(fw.idx.root, path)
	if err != nil {
		relPath = path
	}

	// Delete file (cascade will delete symbols and imports)
	_, err = fw.idx.db.Exec("DELETE FROM files WHERE path = ?", relPath)
	return err
}

// Close stops watching and releases resources
func (fw *FsnotifyWatcher) Close() error {
	fw.cancel()
	if fw.watcher != nil {
		return fw.watcher.Close()
	}
	return nil
}

// =============================================================================
// POLLING WATCHER (FALLBACK)
// =============================================================================

// PollingWatcher implements FileWatcher using periodic polling
type PollingWatcher struct {
	idx      *CodebaseIndex
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	files    map[string]time.Time // File path -> mod time
	mu       sync.Mutex
}

// NewPollingWatcher creates a new polling-based watcher
func NewPollingWatcher(idx *CodebaseIndex, interval time.Duration) *PollingWatcher {
	ctx, cancel := context.WithCancel(context.Background())

	return &PollingWatcher{
		idx:      idx,
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
		files:    make(map[string]time.Time),
	}
}

// Watch starts watching for file changes
func (pw *PollingWatcher) Watch() error {
	// Initial scan
	if err := pw.scan(); err != nil {
		return err
	}

	// Start polling goroutine
	go pw.poll()

	return nil
}

// scan scans the codebase and records file modification times
func (pw *PollingWatcher) scan() error {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	newFiles := make(map[string]time.Time)

	err := filepath.Walk(pw.idx.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if pw.idx.shouldIgnore(filepath.Base(path)) {
				return filepath.SkipDir
			}
			return nil
		}

		// Only track files we can parse
		ext := filepath.Ext(path)
		if _, ok := pw.idx.parsers[ext]; !ok {
			return nil
		}

		newFiles[path] = info.ModTime()
		return nil
	})

	if err != nil {
		return err
	}

	pw.files = newFiles
	return nil
}

// poll periodically checks for file changes
func (pw *PollingWatcher) poll() {
	ticker := time.NewTicker(pw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-pw.ctx.Done():
			return

		case <-ticker.C:
			pw.checkChanges()
		}
	}
}

// checkChanges checks for file changes and updates the index
func (pw *PollingWatcher) checkChanges() {
	pw.mu.Lock()
	oldFiles := make(map[string]time.Time)
	for k, v := range pw.files {
		oldFiles[k] = v
	}
	pw.mu.Unlock()

	// Scan current state
	if err := pw.scan(); err != nil {
		return
	}

	pw.mu.Lock()
	currentFiles := pw.files
	pw.mu.Unlock()

	// Check for changes
	for path, modTime := range currentFiles {
		if oldTime, exists := oldFiles[path]; !exists || !oldTime.Equal(modTime) {
			// File changed or is new
			pw.updateFile(path)
		}
	}

	// Check for deletions
	for path := range oldFiles {
		if _, exists := currentFiles[path]; !exists {
			// File was deleted
			pw.removeFile(path)
		}
	}
}

// updateFile updates a single file in the index
func (pw *PollingWatcher) updateFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	ext := filepath.Ext(path)
	parser, ok := pw.idx.parsers[ext]
	if !ok {
		return nil
	}

	// Get relative path
	relPath, err := filepath.Rel(pw.idx.root, path)
	if err != nil {
		relPath = path
	}

	// Begin transaction
	tx, err := pw.idx.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete old entry if exists
	tx.Exec("DELETE FROM files WHERE path = ?", relPath)

	// Index file
	if _, err := pw.idx.indexFile(tx, path, info, parser); err != nil {
		// Transaction will be rolled back by deferred Rollback
		return err
	}

	return tx.Commit()
}

// removeFile removes a file from the index
func (pw *PollingWatcher) removeFile(path string) error {
	relPath, err := filepath.Rel(pw.idx.root, path)
	if err != nil {
		relPath = path
	}

	_, err = pw.idx.db.Exec("DELETE FROM files WHERE path = ?", relPath)
	return err
}

// Close stops watching
func (pw *PollingWatcher) Close() error {
	pw.cancel()
	return nil
}

// =============================================================================
// WATCHER FACTORY
// =============================================================================

// startWatcher starts the file watcher (fsnotify or polling fallback)
func (idx *CodebaseIndex) startWatcher() error {
	// Try fsnotify first
	fw, err := NewFsnotifyWatcher(idx, idx.config.WatchDebounce)
	if err == nil {
		if err := fw.Watch(); err == nil {
			idx.watcher = fw
			return nil
		}
		fw.Close()
	}

	// Fallback to polling watcher
	pw := NewPollingWatcher(idx, 5*time.Second)
	if err := pw.Watch(); err != nil {
		return err
	}

	idx.watcher = pw
	return nil
}
