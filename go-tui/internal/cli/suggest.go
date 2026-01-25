// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// suggest.go - Command suggestion for typo correction.
package cli

import (
	"strings"
)

// validCommands is the list of all valid rigrun commands.
// This includes primary commands and their aliases.
var validCommands = []string{
	// Primary commands
	"tui",
	"ask",
	"chat",
	"status",
	"config",
	"setup",
	"cache",
	"session",
	"doctor",
	"classify",
	"consent",
	"audit",
	"verify",
	"test",
	"encrypt",
	"crypto",
	"backup",
	"incident",
	"data",
	"vuln",
	"rbac",
	"maintenance",
	"boundary",
	"sectest",
	"conmon",
	"lockout",
	"auth",
	"training",
	"transport",
	"intel",
	"version",
	"help",
	// Aliases
	"s",           // status
	"sessions",    // session
	"backups",     // backup
	"incidents",   // incident
	"sanitize",    // data
	"roles",       // rbac
	"maint",       // maintenance
	"ma",          // maintenance
	"sc7",         // boundary
	"sa11",        // sectest
	"ca7",         // conmon
	"monitoring",  // conmon
	"ac7",         // lockout
	"ia2",         // auth
	"authentication", // auth
	"at2",         // training
	"sc8",         // transport
	"ci",          // intel
	"bist",        // test
	"integrity",   // verify
	"encryption",  // encrypt
	"cryptographic", // crypto
	"classification", // classify
	"vulnerability",  // vuln
	"vulnerabilities", // vuln
	"config-mgmt", // configmgmt
	"configmgmt",  // configmgmt
	"cm",          // configmgmt
}

// SuggestCommand returns a suggested command if the input is close to a valid command.
// Returns empty string if no good match is found.
// Uses Levenshtein distance with a threshold based on command length.
func SuggestCommand(input string) string {
	input = strings.ToLower(input)

	// Don't suggest for very short inputs (likely intentional)
	if len(input) < 2 {
		return ""
	}

	bestMatch := ""
	bestDistance := -1

	// Calculate maximum acceptable distance based on input length
	// For very short commands (<=3 chars): allow 1 edit
	// For short commands (4-5 chars): allow 2 edits (catches transpositions like "hepl" -> "help")
	// For medium commands (6-8 chars): allow 2 edits
	// For longer commands: allow 3 edits
	maxDistance := 1
	if len(input) >= 4 {
		maxDistance = 2
	}
	if len(input) > 8 {
		maxDistance = 3
	}

	for _, cmd := range validCommands {
		distance := levenshteinDistance(input, cmd)

		// Skip exact matches (shouldn't happen if called correctly)
		if distance == 0 {
			return ""
		}

		// Update best match if this is closer
		if distance <= maxDistance && (bestDistance == -1 || distance < bestDistance) {
			bestDistance = distance
			bestMatch = cmd
		}
	}

	return bestMatch
}

// levenshteinDistance calculates the edit distance between two strings.
// This is the minimum number of single-character edits (insertions, deletions,
// or substitutions) required to change one string into the other.
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	rows := len(s1) + 1
	cols := len(s2) + 1

	// Use two rows instead of full matrix for memory efficiency
	prev := make([]int, cols)
	curr := make([]int, cols)

	// Initialize first row
	for j := 0; j < cols; j++ {
		prev[j] = j
	}

	// Fill in the rest
	for i := 1; i < rows; i++ {
		curr[0] = i

		for j := 1; j < cols; j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			// Minimum of: delete, insert, substitute
			curr[j] = min3(
				prev[j]+1,      // deletion
				curr[j-1]+1,    // insertion
				prev[j-1]+cost, // substitution
			)
		}

		// Swap rows
		prev, curr = curr, prev
	}

	return prev[cols-1]
}

// min3 returns the minimum of three integers.
func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}
