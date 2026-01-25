// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides the visual UI components for rigrun TUI.
//
// Classification Banner Component for IL5 Compliance
// Per DoDI 5200.48 (Classification Marking Requirements) and SC-16 (Transmission of Security Attributes)

package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// CLASSIFICATION BANNER COMPONENT - DoD-compliant security classification banner
// =============================================================================

// ClassificationBanner displays the current security classification level
// at the top of the TUI per DoDI 5200.48 marking requirements.
type ClassificationBanner struct {
	classification security.Classification
	width          int
}

// NewClassificationBanner creates a new ClassificationBanner with default UNCLASSIFIED classification.
func NewClassificationBanner() *ClassificationBanner {
	return &ClassificationBanner{
		classification: security.DefaultClassification(),
		width:          80,
	}
}

// NewClassificationBannerFromString creates a ClassificationBanner by parsing a classification string.
// If parsing fails, defaults to UNCLASSIFIED.
func NewClassificationBannerFromString(classificationStr string) *ClassificationBanner {
	classification, err := security.ParseClassification(classificationStr)
	if err != nil {
		classification = security.DefaultClassification()
	}
	return &ClassificationBanner{
		classification: classification,
		width:          80,
	}
}

// SetWidth updates the banner width for full-width rendering.
func (b *ClassificationBanner) SetWidth(width int) {
	b.width = width
}

// SetClassification updates the classification level.
func (b *ClassificationBanner) SetClassification(c security.Classification) {
	b.classification = c
}

// SetClassificationFromString updates the classification level by parsing a string.
// Returns an error if the string cannot be parsed.
func (b *ClassificationBanner) SetClassificationFromString(s string) error {
	c, err := security.ParseClassification(s)
	if err != nil {
		return err
	}
	b.classification = c
	return nil
}

// GetClassification returns the current classification.
func (b *ClassificationBanner) GetClassification() security.Classification {
	return b.classification
}

// View renders the classification banner as a single full-width line.
// Format with decorative blocks:
//
//	(centered block chars) CLASSIFICATION_TEXT (centered block chars)
//
// Example:
//
//	(block chars) UNCLASSIFIED (block chars)
//	(block chars) SECRET // NOFORN (block chars)
func (b *ClassificationBanner) View() string {
	// Get the style based on classification level
	style := security.GetSecurityBannerStyle(b.classification.Level)

	// Build the classification text with caveats
	classText := b.classification.Level.String()
	if len(b.classification.Caveats) > 0 {
		classText = classText + " // " + strings.Join(b.classification.Caveats, " // ")
	}

	// Calculate padding for centering with decorative blocks
	// Format: (block chars) TEXT (block chars)
	// Using Unicode block character for visual emphasis
	blockChar := "\u2588" // Full block character

	// Calculate available space for blocks on each side
	// Minimum 4 blocks on each side, text, and spaces around text
	textWidth := len(classText) + 2 // +2 for spaces around text
	availableForBlocks := b.width - textWidth
	if availableForBlocks < 8 {
		availableForBlocks = 8 // Minimum blocks
	}
	blocksPerSide := availableForBlocks / 2

	// Build the banner line
	leftBlocks := strings.Repeat(blockChar, blocksPerSide)
	rightBlocks := strings.Repeat(blockChar, blocksPerSide)

	// Construct centered content
	content := leftBlocks + " " + classText + " " + rightBlocks

	// Ensure exact width
	contentLen := lipgloss.Width(content)
	if contentLen < b.width {
		// Add extra blocks to fill width
		extra := b.width - contentLen
		extraLeft := extra / 2
		extraRight := extra - extraLeft
		content = strings.Repeat(blockChar, extraLeft) + content + strings.Repeat(blockChar, extraRight)
	} else if contentLen > b.width {
		// Truncate if too wide (unlikely but handle gracefully)
		// Just use centered text without blocks
		content = classText
	}

	// Apply the DoD-standard colors and render full width
	return style.
		Width(b.width).
		MaxWidth(b.width).
		Align(lipgloss.Center).
		Render(content)
}

// ViewCompact renders a compact version of the classification banner.
// Useful for narrower terminals.
func (b *ClassificationBanner) ViewCompact() string {
	style := security.GetSecurityBannerStyle(b.classification.Level)

	// Simpler format: === CLASSIFICATION ===
	classText := b.classification.Level.String()
	if len(b.classification.Caveats) > 0 {
		classText = classText + "//" + strings.Join(b.classification.Caveats, "//")
	}

	// Use simple dashes for compact view
	textWidth := len(classText) + 2
	availableForDashes := b.width - textWidth
	if availableForDashes < 4 {
		availableForDashes = 4
	}
	dashesPerSide := availableForDashes / 2

	leftDashes := strings.Repeat("=", dashesPerSide)
	rightDashes := strings.Repeat("=", dashesPerSide)

	content := leftDashes + " " + classText + " " + rightDashes

	return style.
		Width(b.width).
		MaxWidth(b.width).
		Align(lipgloss.Center).
		Render(content)
}

// Height returns the height of the banner (always 1 line).
func (b *ClassificationBanner) Height() int {
	return 1
}

// IsClassified returns true if the classification level is CONFIDENTIAL or above.
func (b *ClassificationBanner) IsClassified() bool {
	return b.classification.IsClassified()
}

// =============================================================================
// CLASSIFICATION CHANGED MESSAGE
// =============================================================================

// ClassificationChangedMsg is sent when the classification level changes.
// This allows other components to update their display accordingly.
type ClassificationChangedMsg struct {
	Classification security.Classification
}
