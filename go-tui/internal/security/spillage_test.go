// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// spillage_test.go - Tests for spillage detection bypass fixes
package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeForDetection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple uppercase",
			input:    "TOP SECRET",
			expected: "TOPSECRET",
		},
		{
			name:     "Number substitution - zero to O",
			input:    "T0P SECRET",
			expected: "TOPSECRET",
		},
		{
			name:     "Cyrillic T substitution",
			input:    "ТOP SECRET", // Cyrillic T
			expected: "TOPSECRET",
		},
		{
			name:     "Underscore separator bypass",
			input:    "TOP_SECRET",
			expected: "TOPSECRET",
		},
		{
			name:     "Multiple substitutions",
			input:    "T0P_S3CR3T",
			expected: "TOPSECRET",
		},
		{
			name:     "Mixed Cyrillic and Latin",
			input:    "ТОPSECRЕТ", // Multiple Cyrillic chars
			expected: "TOPSECRET",
		},
		{
			name:     "Special character substitutions",
			input:    "T@P $ECRET",
			expected: "TAPSECRET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeForDetection(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeForDetection(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCalculateEntropy(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		minEntropy    float64
		maxEntropy    float64
		expectHighEnt bool
	}{
		{
			name:          "Low entropy - repeated chars",
			input:         "aaaaaaaaaa",
			minEntropy:    0.0,
			maxEntropy:    0.1,
			expectHighEnt: false,
		},
		{
			name:          "Medium entropy - normal text",
			input:         "hello world",
			minEntropy:    2.0,
			maxEntropy:    4.0,
			expectHighEnt: false,
		},
		{
			name:          "High entropy - random key",
			input:         "xK9#mP2$qL5@nR8&vT3!wY4%zU6^aD7",
			minEntropy:    4.5,
			maxEntropy:    6.0,
			expectHighEnt: true,
		},
		{
			name:          "High entropy - API key pattern",
			input:         "test_key_aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2u",
			minEntropy:    4.0,
			maxEntropy:    6.0,
			expectHighEnt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entropy := calculateEntropy(tt.input)
			if entropy < tt.minEntropy || entropy > tt.maxEntropy {
				t.Errorf("calculateEntropy(%q) = %f, want between %f and %f",
					tt.input, entropy, tt.minEntropy, tt.maxEntropy)
			}
			isHigh := entropy > EntropyThreshold
			if isHigh != tt.expectHighEnt {
				t.Errorf("entropy %f (threshold %f): got high=%v, want %v",
					entropy, EntropyThreshold, isHigh, tt.expectHighEnt)
			}
		})
	}
}

func TestDetectBypassAttempts(t *testing.T) {
	manager := NewSpillageManager()

	tests := []struct {
		name          string
		content       string
		shouldDetect  bool
		expectedCount int
	}{
		{
			name:          "Normal TOP SECRET",
			content:       "This is TOP SECRET information",
			shouldDetect:  true,
			expectedCount: 1,
		},
		{
			name:          "Zero substitution bypass attempt",
			content:       "This is T0P SECRET information",
			shouldDetect:  true,
			expectedCount: 1,
		},
		{
			name:          "Cyrillic T bypass attempt",
			content:       "This is ТOP SECRET information", // Cyrillic T
			shouldDetect:  true,
			expectedCount: 1,
		},
		{
			name:          "Underscore separator bypass",
			content:       "This is TOP_SECRET information",
			shouldDetect:  true,
			expectedCount: 1,
		},
		{
			name:          "Multiple bypass techniques",
			content:       "T0P_SECRET and ТОPSECRЕТ data",
			shouldDetect:  true,
			expectedCount: 2,
		},
		{
			name:          "No classification markers",
			content:       "This is unclassified information",
			shouldDetect:  false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := manager.Detect(tt.content)
			// Filter only pattern detections (not entropy)
			patternEvents := 0
			for _, e := range events {
				if e.DetectionType == "pattern" {
					patternEvents++
				}
			}

			if tt.shouldDetect && patternEvents == 0 {
				t.Errorf("Expected to detect spillage in %q but found none", tt.content)
			}
			if !tt.shouldDetect && patternEvents > 0 {
				t.Errorf("Expected no detection in %q but found %d events", tt.content, patternEvents)
			}
			if tt.shouldDetect && patternEvents != tt.expectedCount {
				t.Errorf("Expected %d detections but found %d", tt.expectedCount, patternEvents)
			}
		})
	}
}

func TestEntropyDetection(t *testing.T) {
	manager := NewSpillageManager()

	tests := []struct {
		name         string
		content      string
		shouldDetect bool
	}{
		{
			name:         "API key detection",
			content:      "My key is test_key_aB3cD4eF5gH6iJ7kL8mN9oP0qR1sT2uV3w",
			shouldDetect: true,
		},
		{
			name:         "Random secret detection",
			content:      "Password: xK9#mP2$qL5@nR8&vT3!wY4%zU6",
			shouldDetect: true,
		},
		{
			name:         "Normal text no detection",
			content:      "The quick brown fox jumps over the lazy dog",
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := manager.Detect(tt.content)
			// Check for entropy detections
			entropyDetected := false
			for _, e := range events {
				if e.DetectionType == "entropy" {
					entropyDetected = true
					break
				}
			}

			if tt.shouldDetect && !entropyDetected {
				t.Errorf("Expected entropy detection in %q but found none", tt.content)
			}
			if !tt.shouldDetect && entropyDetected {
				t.Errorf("Expected no entropy detection in %q but found some", tt.content)
			}
		})
	}
}

func TestSanitizeWithBackup(t *testing.T) {
	manager := NewSpillageManager()

	// Create temp directory for test
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	// Write test content with spillage
	content := "This document contains T0P SECRET information that should be redacted."
	err := os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Sanitize the file
	err = manager.SanitizeFile(testFile)
	if err != nil {
		t.Fatalf("SanitizeFile failed: %v", err)
	}

	// Verify sanitized content
	sanitized, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read sanitized file: %v", err)
	}

	sanitizedStr := string(sanitized)
	if strings.Contains(sanitizedStr, "T0P SECRET") {
		t.Errorf("Sanitized file still contains spillage: %q", sanitizedStr)
	}
	if !strings.Contains(sanitizedStr, "[REDACTED-") {
		t.Errorf("Sanitized file does not contain redaction marker: %q", sanitizedStr)
	}

	// Verify backup was created
	backupDir := filepath.Join(tempDir, SpillageBackupDir)
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("Backup directory not created: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("No backup file created")
	}

	// Verify backup contains original content
	backupPath := filepath.Join(backupDir, entries[0].Name())
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup file: %v", err)
	}

	if string(backupContent) != content {
		t.Errorf("Backup content doesn't match original.\nGot: %q\nWant: %q",
			string(backupContent), content)
	}
}

func TestDetectHighEntropyStrings(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		minLength int
		wantCount int
	}{
		{
			name:      "Find one high-entropy string",
			content:   "The password is xK9#mP2$qL5@nR8&vT3!wY4%zU6 for access",
			minLength: 16,
			wantCount: 1,
		},
		{
			name:      "Multiple high-entropy strings",
			content:   "Key1: aB3$dE5&gH7*jK9#mX2pL8qW Key2: mN2@pQ4%rS6&tU8!vF3zY7nC",
			minLength: 16,
			wantCount: 2,
		},
		{
			name:      "No high-entropy strings",
			content:   "This is normal text with common words",
			minLength: 16,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := detectHighEntropyStrings(tt.content, tt.minLength)
			if len(results) != tt.wantCount {
				t.Errorf("detectHighEntropyStrings() found %d strings, want %d",
					len(results), tt.wantCount)
			}
		})
	}
}
