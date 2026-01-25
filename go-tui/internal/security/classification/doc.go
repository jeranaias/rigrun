// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package classification provides DoD classification marking and enforcement.
//
// This package implements NIST 800-53 AC-4 (Information Flow Enforcement)
// and supports DoD classification marking per DoDI 5200.48 and 32 CFR Part 2002.
//
// # Classification Levels
//
// Supported classification levels (highest to lowest):
//   - TOP SECRET: Grave damage to national security
//   - SECRET: Serious damage to national security
//   - CONFIDENTIAL: Damage to national security
//   - CUI: Controlled Unclassified Information
//   - UNCLASSIFIED: No classification
//
// # Classification Marking
//
// Classification banners are displayed per DoD requirements:
//
//	level := classification.ParseLevel("SECRET")
//	banner := classification.RenderTopBanner(level)
//	// Returns: "SECRET//NOFORN"
//
// # Information Flow Enforcement
//
// The enforcer prevents unauthorized data flows:
//
//	enforcer := classification.NewEnforcer()
//	err := enforcer.ValidateFlow(sourceLevel, destLevel)
//	if err != nil {
//	    // Block the flow
//	}
//
// # CUI Designations
//
// CUI categories are supported:
//
//	classification.CUIBasic      // Basic CUI
//	classification.CUISpecified  // Specified CUI with handling requirements
//
// # Routing Tier Integration
//
// Classification levels map to routing tiers:
//
//	tier := classification.GetRoutingTier(level)
//	// Returns: classification.TierIL5 for SECRET
package classification
