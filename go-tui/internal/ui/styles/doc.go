// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

/*
Package styles provides the visual styling system for the rigrun TUI application.

This package defines the complete color palette, typography, and animation
system used throughout the application. All colors use Lip Gloss AdaptiveColor
for automatic light/dark terminal detection.

# Color System (colors.go)

## Primary Accent Colors

  - Purple - Primary accent for assistant messages and selections
  - Cyan - Brand color for info, commands, and user highlights
  - Emerald - Success states and local mode indicator
  - Amber - Warnings, cloud mode indicator, and cost displays
  - Rose - Errors and critical warnings

## Semantic Colors

Message bubbles, tool results, and UI elements use semantic color tokens:

	UserBubbleBg      - Background for user messages
	UserBubbleFg      - Text color for user messages
	AssistantBubbleBg - Background for assistant messages
	AssistantBubbleFg - Text color for assistant messages

## Surface Colors

Layered surface system for depth:

	Base       - Main background
	Surface    - Elevated elements
	SurfaceDim - Subtle backgrounds (headers, status bars)
	Overlay    - Overlays and popups

## Text Colors

Hierarchical text color system:

	TextPrimary   - Main content text
	TextSecondary - Supporting text
	TextMuted     - De-emphasized text
	TextInverse   - Text on colored backgrounds

# Theme System (theme.go)

The Theme struct provides runtime color adaptation:

	theme := styles.NewTheme()
	if theme.IsDark {
		// Dark terminal detected
	}
	if theme.HasTrueColor {
		// Terminal supports 16M colors
	}

# Animation System (animations.go)

## Spinner Configurations

Pre-defined spinner styles:

	BrailleSpinner - Smooth 10-frame spinner
	DotsSpinner    - Classic three-dot animation
	LineSpinner    - Simple line rotation

## Status Indicators

Unicode and ASCII indicators for various states:

	StatusIndicators.Success   - Checkmark or [OK]
	StatusIndicators.Error     - X mark or [ERR]
	StatusIndicators.Warning   - Warning triangle or [!]
	StatusIndicators.Info      - Info circle or [i]

## Tier Icons

Icons for routing tier display:

	TierIcons[TierCache]  - Lightning bolt (cached response)
	TierIcons[TierLocal]  - At symbol (local Ollama)
	TierIcons[TierCloud]  - Star (cloud API)

# Usage Example

	import "github.com/jeranaias/rigrun-tui/internal/ui/styles"

	// Use adaptive colors
	headerStyle := lipgloss.NewStyle().
		Background(styles.SurfaceDim).
		Foreground(styles.TextPrimary)

	// Use theme for runtime detection
	theme := styles.NewTheme()
	statusStyle := theme.GetStatusStyle(styles.StatusSuccess)

	// Use spinner configuration
	spinner := styles.BrailleSpinner
	frame := spinner.GetFrame(time.Now())
*/
package styles
