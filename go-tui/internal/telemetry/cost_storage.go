// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// =============================================================================
// COST STORAGE
// =============================================================================

// CostStorage persists cost data to disk.
type CostStorage struct {
	dir string
}

// NewCostStorage creates a new cost storage manager.
func NewCostStorage(dir string) (*CostStorage, error) {
	// Default to ~/.rigrun/costs/
	if dir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(homeDir, ".rigrun", "costs")
	}

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	return &CostStorage{dir: dir}, nil
}

// =============================================================================
// PERSISTENCE
// =============================================================================

// Save persists a session cost to disk.
func (cs *CostStorage) Save(session *SessionCost) error {
	if session == nil {
		return nil
	}

	// Create filename from session ID
	filename := filepath.Join(cs.dir, session.ID+".json")

	// Marshal to JSON
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(filename, data, 0644)
}

// Load retrieves a session cost from disk.
func (cs *CostStorage) Load(sessionID string) (*SessionCost, error) {
	filename := filepath.Join(cs.dir, sessionID+".json")

	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON
	var session SessionCost
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// List returns all session IDs within the specified date range.
func (cs *CostStorage) List(from, to time.Time) ([]string, error) {
	// Read directory
	entries, err := os.ReadDir(cs.dir)
	if err != nil {
		return nil, err
	}

	var sessionIDs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		// Extract session ID from filename
		sessionID := strings.TrimSuffix(name, ".json")

		// Parse timestamp from session ID (format: 20060102-150405 or 20060102-150405-microseconds)
		// Extract just the date-time part for parsing
		timestampPart := sessionID
		if parts := strings.Split(sessionID, "-"); len(parts) >= 3 {
			// Format: YYYYMMDD-HHMMSS-microseconds
			timestampPart = parts[0] + "-" + parts[1]
		}

		timestamp, err := time.Parse("20060102-150405", timestampPart)
		if err != nil {
			continue // Skip invalid filenames
		}

		// Filter by date range
		if timestamp.Before(from) || timestamp.After(to) {
			continue
		}

		sessionIDs = append(sessionIDs, sessionID)
	}

	// Sort by session ID (which is timestamp-based)
	sort.Strings(sessionIDs)

	return sessionIDs, nil
}

// Delete removes a session cost file from disk.
func (cs *CostStorage) Delete(sessionID string) error {
	filename := filepath.Join(cs.dir, sessionID+".json")
	return os.Remove(filename)
}

// DeleteBefore removes all session cost files older than the specified date.
func (cs *CostStorage) DeleteBefore(before time.Time) error {
	entries, err := os.ReadDir(cs.dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}

		sessionID := strings.TrimSuffix(name, ".json")

		// Parse timestamp from session ID
		timestampPart := sessionID
		if parts := strings.Split(sessionID, "-"); len(parts) >= 3 {
			timestampPart = parts[0] + "-" + parts[1]
		}

		timestamp, err := time.Parse("20060102-150405", timestampPart)
		if err != nil {
			continue
		}

		if timestamp.Before(before) {
			filename := filepath.Join(cs.dir, name)
			os.Remove(filename) // Ignore errors
		}
	}

	return nil
}

// Size returns the total size of stored cost data in bytes.
func (cs *CostStorage) Size() (int64, error) {
	entries, err := os.ReadDir(cs.dir)
	if err != nil {
		return 0, err
	}

	var total int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		total += info.Size()
	}

	return total, nil
}

// Count returns the number of stored sessions.
func (cs *CostStorage) Count() (int, error) {
	entries, err := os.ReadDir(cs.dir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			count++
		}
	}

	return count, nil
}
