// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"sort"
	"strings"
	"unicode"
)

// =============================================================================
// FUZZY MATCHING
// =============================================================================

// FuzzyMatch performs fuzzy matching between a query and a target string.
// Returns a score (higher is better) and whether the match succeeded.
//
// Matching rules:
//   - Each character in query must appear in order in target
//   - Consecutive matches get bonus points
//   - Matches at word boundaries get bonus points
//   - Matches at start of string get bonus points
//   - Case-insensitive matching
//
// Examples:
//   - "sv" matches "/save" with high score (start + consecutive)
//   - "sv" matches "/sessions" with lower score (start but not consecutive)
//   - "hlp" matches "/help" (non-consecutive)
//   - "xyz" does not match "/save"
func FuzzyMatch(query, target string) (score int, matched bool) {
	if query == "" {
		return 0, true
	}

	queryRunes := []rune(strings.ToLower(query))
	targetRunes := []rune(strings.ToLower(target))

	if len(queryRunes) > len(targetRunes) {
		return 0, false
	}

	// Pre-convert original strings to runes for case matching (avoid repeated conversions)
	targetOrigRunes := []rune(target)
	queryOrigRunes := []rune(query)

	// Track query position
	queryPos := 0
	score = 0
	lastMatchPos := -1

	for targetPos := 0; targetPos < len(targetRunes) && queryPos < len(queryRunes); targetPos++ {
		if targetRunes[targetPos] == queryRunes[queryPos] {
			// Base score for match
			matchScore := 1

			// Bonus for consecutive matches
			if lastMatchPos == targetPos-1 {
				matchScore += 5
			}

			// Bonus for match at start of target
			if targetPos == 0 {
				matchScore += 10
			}

			// Bonus for match at word boundary
			if isWordBoundary(targetRunes, targetPos) {
				matchScore += 7
			}

			// Bonus for exact case match
			if targetPos < len(targetOrigRunes) && queryPos < len(queryOrigRunes) {
				if targetOrigRunes[targetPos] == queryOrigRunes[queryPos] {
					matchScore += 2
				}
			}

			score += matchScore
			lastMatchPos = targetPos
			queryPos++
		}
	}

	// Did we match all query characters?
	matched = queryPos == len(queryRunes)

	// Penalty for longer targets (shorter strings are better matches)
	if matched {
		score -= len(targetRunes) / 4
	}

	return score, matched
}

// isWordBoundary returns true if the position is at a word boundary.
// A word boundary is:
//   - After a space, slash, dash, or underscore
//   - After a lowercase letter followed by an uppercase letter (camelCase)
func isWordBoundary(runes []rune, pos int) bool {
	if pos == 0 {
		return true
	}

	if pos >= len(runes) {
		return false
	}

	prev := runes[pos-1]

	// After separator characters
	if prev == ' ' || prev == '/' || prev == '-' || prev == '_' {
		return true
	}

	// CamelCase boundary (lowercase -> uppercase)
	if pos > 0 && unicode.IsLower(prev) && unicode.IsUpper(runes[pos]) {
		return true
	}

	return false
}

// FuzzyMatchScore is a convenience wrapper that returns only the score.
// Returns 0 if the match failed.
func FuzzyMatchScore(query, target string) int {
	score, matched := FuzzyMatch(query, target)
	if !matched {
		return 0
	}
	return score
}

// FuzzyMatches returns true if the query fuzzy-matches the target.
func FuzzyMatches(query, target string) bool {
	_, matched := FuzzyMatch(query, target)
	return matched
}

// =============================================================================
// SCORED MATCH
// =============================================================================

// ScoredMatch represents a fuzzy match result with score.
type ScoredMatch struct {
	Target string
	Score  int
	Data   interface{} // Optional associated data
}

// FuzzyFilter filters a list of strings using fuzzy matching.
// Returns matches sorted by score (highest first).
func FuzzyFilter(query string, targets []string) []ScoredMatch {
	var matches []ScoredMatch

	for _, target := range targets {
		score, matched := FuzzyMatch(query, target)
		if matched {
			matches = append(matches, ScoredMatch{
				Target: target,
				Score:  score,
			})
		}
	}

	// Sort by score (highest first)
	sortScoredMatches(matches)

	return matches
}

// sortScoredMatches sorts matches by score in descending order.
func sortScoredMatches(matches []ScoredMatch) {
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})
}

// =============================================================================
// HIGHLIGHTING
// =============================================================================

// HighlightMatch returns the target string with matched characters highlighted.
// Returns the positions of matched characters for styling.
func HighlightMatch(query, target string) (positions []int) {
	if query == "" {
		return nil
	}

	queryRunes := []rune(strings.ToLower(query))
	targetRunes := []rune(strings.ToLower(target))

	queryPos := 0
	for targetPos := 0; targetPos < len(targetRunes) && queryPos < len(queryRunes); targetPos++ {
		if targetRunes[targetPos] == queryRunes[queryPos] {
			positions = append(positions, targetPos)
			queryPos++
		}
	}

	return positions
}
