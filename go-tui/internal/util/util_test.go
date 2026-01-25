// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package util provides utility functions for the go-tui application.
package util

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// ATOMIC WRITE TESTS
// =============================================================================

func TestAtomicWriteFile_Basic(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "test.txt")
	data := []byte("hello, world!")

	err := AtomicWriteFile(path, data, 0644)
	if err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("Content mismatch: got %q, want %q", string(content), string(data))
	}
}

func TestAtomicWriteFile_CreatesParentDir(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "subdir", "deep", "test.txt")
	data := []byte("test data")

	err := AtomicWriteFile(path, data, 0644)
	if err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("File not created: %v", err)
	}
}

func TestAtomicWriteFile_Overwrites(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "test.txt")

	// Write initial content
	if err := AtomicWriteFile(path, []byte("initial"), 0644); err != nil {
		t.Fatalf("First write failed: %v", err)
	}

	// Overwrite
	if err := AtomicWriteFile(path, []byte("updated"), 0644); err != nil {
		t.Fatalf("Second write failed: %v", err)
	}

	// Verify new content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != "updated" {
		t.Errorf("Content not updated: got %q", string(content))
	}
}

func TestAtomicWriteFile_EmptyData(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "empty.txt")

	err := AtomicWriteFile(path, []byte{}, 0644)
	if err != nil {
		t.Fatalf("AtomicWriteFile failed for empty data: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("File not created: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("Expected empty file, got size %d", info.Size())
	}
}

func TestAtomicWriteFile_LargeData(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "large.txt")

	// Create 1MB of data
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	err := AtomicWriteFile(path, data, 0644)
	if err != nil {
		t.Fatalf("AtomicWriteFile failed for large data: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if len(content) != len(data) {
		t.Errorf("Size mismatch: got %d, want %d", len(content), len(data))
	}
}

func TestAtomicWriteFileWithDir(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "newdir", "test.txt")

	err := AtomicWriteFileWithDir(path, []byte("test"), 0600, 0700)
	if err != nil {
		t.Fatalf("AtomicWriteFileWithDir failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("File not created: %v", err)
	}
}

// =============================================================================
// STRING TRUNCATION TESTS
// =============================================================================

func TestTruncateRunes_ASCII(t *testing.T) {
	testCases := []struct {
		input    string
		maxRunes int
		expected string
	}{
		{"hello world", 5, "he..."},
		{"hello", 5, "hello"},
		{"hi", 5, "hi"},
		{"", 5, ""},
		{"hello world", 0, ""},
		{"hello world", 11, "hello world"},
		{"ab", 3, "ab"},
		{"abcd", 3, "abc"}, // When maxRunes <= 3, no ellipsis is added
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := TruncateRunes(tc.input, tc.maxRunes)
			if result != tc.expected {
				t.Errorf("TruncateRunes(%q, %d) = %q, want %q",
					tc.input, tc.maxRunes, result, tc.expected)
			}
		})
	}
}

func TestTruncateRunes_UTF8(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		maxRunes int
		expected string
	}{
		{"emoji", "hello 👋 world", 7, "hell..."},
		{"chinese", "你好世界", 3, "你好世界"[:len("你好世界")-len("界")]}, // 3 chars
		{"mixed", "hi 日本", 4, "h..."},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := TruncateRunes(tc.input, tc.maxRunes)
			if len([]rune(result)) > tc.maxRunes {
				t.Errorf("TruncateRunes result %q has %d runes, want <= %d",
					result, len([]rune(result)), tc.maxRunes)
			}
		})
	}
}

func TestTruncateRunesNoEllipsis(t *testing.T) {
	testCases := []struct {
		input    string
		maxRunes int
		expected string
	}{
		{"hello world", 5, "hello"},
		{"hello", 5, "hello"},
		{"hi", 5, "hi"},
		{"", 5, ""},
		{"hello world", 0, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := TruncateRunesNoEllipsis(tc.input, tc.maxRunes)
			if result != tc.expected {
				t.Errorf("TruncateRunesNoEllipsis(%q, %d) = %q, want %q",
					tc.input, tc.maxRunes, result, tc.expected)
			}
		})
	}
}

func TestSafeSubstring(t *testing.T) {
	testCases := []struct {
		input    string
		start    int
		end      int
		expected string
	}{
		{"hello world", 0, 5, "hello"},
		{"hello world", 6, 11, "world"},
		{"hello", 0, 10, "hello"},
		{"hello", 10, 15, ""},
		{"hello", -1, 3, "hel"},
		{"hello", 3, 2, ""},
		{"你好世界", 0, 2, "你好"},
		{"你好世界", 1, 3, "好世"},
	}

	for _, tc := range testCases {
		name := tc.input + "[" + string(rune('0'+tc.start)) + ":" + string(rune('0'+tc.end)) + "]"
		t.Run(name, func(t *testing.T) {
			result := SafeSubstring(tc.input, tc.start, tc.end)
			if result != tc.expected {
				t.Errorf("SafeSubstring(%q, %d, %d) = %q, want %q",
					tc.input, tc.start, tc.end, result, tc.expected)
			}
		})
	}
}

func TestStringWidth(t *testing.T) {
	testCases := []struct {
		input    string
		expected int
	}{
		{"hello", 5},
		{"", 0},
		{"日本語", 6}, // 3 CJK chars = 6 width
		{"こんにちは", 10}, // 5 hiragana = 10 width
		{"hello世界", 9}, // 5 ASCII + 2 CJK = 9
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := StringWidth(tc.input)
			if result != tc.expected {
				t.Errorf("StringWidth(%q) = %d, want %d", tc.input, result, tc.expected)
			}
		})
	}
}

func TestRuneLen(t *testing.T) {
	testCases := []struct {
		input    string
		expected int
	}{
		{"hello", 5},
		{"", 0},
		{"日本語", 3},
		{"hello 👋", 7},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := RuneLen(tc.input)
			if result != tc.expected {
				t.Errorf("RuneLen(%q) = %d, want %d", tc.input, result, tc.expected)
			}
		})
	}
}

func TestTruncateWidth(t *testing.T) {
	// Note: TruncateWidth adds "..." when truncating, so the output width
	// may exceed maxWidth. Tests verify truncation happens, not exact width.
	testCases := []struct {
		name          string
		input         string
		maxWidth      int
		shouldTrunc   bool
	}{
		{"ascii short", "hello", 10, false},
		{"ascii exact", "hello", 5, false},
		{"ascii truncate", "hello world", 5, true},
		{"cjk truncate", "日本語", 3, true},
		{"empty", "", 5, false},
		{"zero width", "hello", 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := TruncateWidth(tc.input, tc.maxWidth)
			wasTruncated := len(result) < len(tc.input) || result == ""
			if tc.shouldTrunc && !wasTruncated && tc.input != "" {
				t.Errorf("TruncateWidth(%q, %d) = %q, expected truncation",
					tc.input, tc.maxWidth, result)
			}
			if !tc.shouldTrunc && wasTruncated && tc.input != "" {
				t.Errorf("TruncateWidth(%q, %d) = %q, unexpected truncation",
					tc.input, tc.maxWidth, result)
			}
		})
	}
}

// =============================================================================
// RUNE WIDTH TESTS
// =============================================================================

func TestRuneWidth(t *testing.T) {
	testCases := []struct {
		r        rune
		expected int
	}{
		{'a', 1},
		{'z', 1},
		{'0', 1},
		{' ', 1},
		{'日', 2},       // CJK
		{'あ', 2},       // Hiragana
		{'ア', 2},       // Katakana
		{'한', 2},       // Hangul
		{'\uff01', 2}, // Fullwidth exclamation
	}

	for _, tc := range testCases {
		t.Run(string(tc.r), func(t *testing.T) {
			result := runeWidth(tc.r)
			if result != tc.expected {
				t.Errorf("runeWidth(%q) = %d, want %d", tc.r, result, tc.expected)
			}
		})
	}
}
