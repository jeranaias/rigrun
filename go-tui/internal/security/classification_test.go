// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// AC-4 Information Flow Enforcement Tests
// Per NIST SP 800-53 AC-4: Information Flow Enforcement
// Per DoDI 5200.48 and 32 CFR Part 2002

package security

import (
	"testing"
)

// ============================================================================
// AC-4 CLASSIFICATION LEVEL TESTS
// ============================================================================

// TestClassificationLevel_String tests the string representation of classification levels.
func TestClassificationLevel_String(t *testing.T) {
	tests := []struct {
		name     string
		level    ClassificationLevel
		expected string
	}{
		{
			name:     "Unclassified",
			level:    ClassificationUnclassified,
			expected: BannerUnclassified,
		},
		{
			name:     "CUI",
			level:    ClassificationCUI,
			expected: BannerCUI,
		},
		{
			name:     "Confidential",
			level:    ClassificationConfidential,
			expected: BannerConfidential,
		},
		{
			name:     "Secret",
			level:    ClassificationSecret,
			expected: BannerSecret,
		},
		{
			name:     "Top Secret",
			level:    ClassificationTopSecret,
			expected: BannerTopSecret,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("ClassificationLevel.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestClassificationLevel_Ordering verifies proper ordering of classification levels.
// NIST AC-4: Classification levels must be strictly ordered for enforcement.
func TestClassificationLevel_Ordering(t *testing.T) {
	tests := []struct {
		name   string
		lower  ClassificationLevel
		higher ClassificationLevel
	}{
		{
			name:   "Unclassified < CUI",
			lower:  ClassificationUnclassified,
			higher: ClassificationCUI,
		},
		{
			name:   "CUI < Confidential",
			lower:  ClassificationCUI,
			higher: ClassificationConfidential,
		},
		{
			name:   "Confidential < Secret",
			lower:  ClassificationConfidential,
			higher: ClassificationSecret,
		},
		{
			name:   "Secret < Top Secret",
			lower:  ClassificationSecret,
			higher: ClassificationTopSecret,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.lower >= tt.higher {
				t.Errorf("Classification ordering violated: %v should be < %v", tt.lower, tt.higher)
			}
		})
	}
}

// TestCompareClassification tests the classification comparison function.
func TestCompareClassification(t *testing.T) {
	tests := []struct {
		name     string
		a        ClassificationLevel
		b        ClassificationLevel
		expected int
	}{
		{
			name:     "Equal levels",
			a:        ClassificationSecret,
			b:        ClassificationSecret,
			expected: 0,
		},
		{
			name:     "Lower < Higher",
			a:        ClassificationCUI,
			b:        ClassificationSecret,
			expected: -1,
		},
		{
			name:     "Higher > Lower",
			a:        ClassificationTopSecret,
			b:        ClassificationUnclassified,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareClassification(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("CompareClassification(%v, %v) = %d, want %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// TestHighestClassification tests selecting the highest classification.
func TestHighestClassification(t *testing.T) {
	tests := []struct {
		name     string
		a        Classification
		b        Classification
		expected ClassificationLevel
	}{
		{
			name:     "Both Unclassified",
			a:        Classification{Level: ClassificationUnclassified},
			b:        Classification{Level: ClassificationUnclassified},
			expected: ClassificationUnclassified,
		},
		{
			name:     "CUI vs Unclassified",
			a:        Classification{Level: ClassificationCUI},
			b:        Classification{Level: ClassificationUnclassified},
			expected: ClassificationCUI,
		},
		{
			name:     "Secret vs CUI",
			a:        Classification{Level: ClassificationSecret},
			b:        Classification{Level: ClassificationCUI},
			expected: ClassificationSecret,
		},
		{
			name:     "Top Secret vs anything",
			a:        Classification{Level: ClassificationTopSecret},
			b:        Classification{Level: ClassificationSecret},
			expected: ClassificationTopSecret,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HighestClassification(tt.a, tt.b)
			if result.Level != tt.expected {
				t.Errorf("HighestClassification() = %v, want %v", result.Level, tt.expected)
			}
		})
	}
}

// ============================================================================
// AC-4 CLASSIFICATION PARSING TESTS
// ============================================================================

// TestParseClassification tests parsing classification strings.
func TestParseClassification(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    ClassificationLevel
		expectError bool
	}{
		{
			name:     "Empty string defaults to Unclassified",
			input:    "",
			expected: ClassificationUnclassified,
		},
		{
			name:     "Parse UNCLASSIFIED",
			input:    "UNCLASSIFIED",
			expected: ClassificationUnclassified,
		},
		{
			name:     "Parse U shorthand",
			input:    "U",
			expected: ClassificationUnclassified,
		},
		{
			name:     "Parse CUI",
			input:    "CUI",
			expected: ClassificationCUI,
		},
		{
			name:     "Parse CONFIDENTIAL",
			input:    "CONFIDENTIAL",
			expected: ClassificationConfidential,
		},
		{
			name:     "Parse C shorthand",
			input:    "C",
			expected: ClassificationConfidential,
		},
		{
			name:     "Parse SECRET",
			input:    "SECRET",
			expected: ClassificationSecret,
		},
		{
			name:     "Parse S shorthand",
			input:    "S",
			expected: ClassificationSecret,
		},
		{
			name:     "Parse TOP SECRET",
			input:    "TOP SECRET",
			expected: ClassificationTopSecret,
		},
		{
			name:     "Parse TS shorthand",
			input:    "TS",
			expected: ClassificationTopSecret,
		},
		{
			name:        "Invalid classification",
			input:       "INVALID_LEVEL",
			expectError: true,
		},
		{
			name:     "Case insensitive - lowercase",
			input:    "secret",
			expected: ClassificationSecret,
		},
		{
			name:     "With caveats",
			input:    "SECRET//NOFORN",
			expected: ClassificationSecret,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseClassification(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseClassification(%q) expected error but got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseClassification(%q) unexpected error: %v", tt.input, err)
				return
			}

			if result.Level != tt.expected {
				t.Errorf("ParseClassification(%q).Level = %v, want %v", tt.input, result.Level, tt.expected)
			}
		})
	}
}

// TestParseClassification_WithCaveats tests parsing classifications with caveats.
func TestParseClassification_WithCaveats(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedLevel  ClassificationLevel
		expectedCaveat string
	}{
		{
			name:           "SECRET with NOFORN",
			input:          "SECRET//NOFORN",
			expectedLevel:  ClassificationSecret,
			expectedCaveat: "NOFORN",
		},
		{
			name:           "TOP SECRET with ORCON",
			input:          "TOP SECRET//ORCON",
			expectedLevel:  ClassificationTopSecret,
			expectedCaveat: "ORCON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseClassification(tt.input)
			if err != nil {
				t.Fatalf("ParseClassification(%q) error: %v", tt.input, err)
			}

			if result.Level != tt.expectedLevel {
				t.Errorf("Level = %v, want %v", result.Level, tt.expectedLevel)
			}

			found := false
			for _, caveat := range result.Caveats {
				if caveat == tt.expectedCaveat {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected caveat %q not found in %v", tt.expectedCaveat, result.Caveats)
			}
		})
	}
}

// ============================================================================
// AC-4 CLASSIFICATION VALIDATION TESTS
// ============================================================================

// TestValidateClassification tests classification validation rules.
func TestValidateClassification(t *testing.T) {
	tests := []struct {
		name        string
		class       Classification
		expectError bool
	}{
		{
			name: "Valid Unclassified",
			class: Classification{
				Level: ClassificationUnclassified,
			},
			expectError: false,
		},
		{
			name: "Valid CUI with Basic",
			class: Classification{
				Level: ClassificationCUI,
				CUI:   []CUIDesignation{CUIBasic},
			},
			expectError: false,
		},
		{
			name: "Valid CUI with NOFORN",
			class: Classification{
				Level: ClassificationCUI,
				CUI:   []CUIDesignation{CUINOFORN},
			},
			expectError: false,
		},
		{
			name: "Invalid: CUI designations on non-CUI level",
			class: Classification{
				Level: ClassificationSecret,
				CUI:   []CUIDesignation{CUINOFORN},
			},
			expectError: true,
		},
		{
			name: "Invalid: CUI BASIC combined with other designations",
			class: Classification{
				Level: ClassificationCUI,
				CUI:   []CUIDesignation{CUIBasic, CUINOFORN},
			},
			expectError: true,
		},
		{
			name: "Invalid: NOFORN on Unclassified",
			class: Classification{
				Level:   ClassificationUnclassified,
				Caveats: []string{"NOFORN"},
			},
			expectError: true,
		},
		{
			name: "Valid: NOFORN on Secret",
			class: Classification{
				Level:   ClassificationSecret,
				Caveats: []string{"NOFORN"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateClassification(tt.class)

			if tt.expectError && err == nil {
				t.Errorf("ValidateClassification() expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("ValidateClassification() unexpected error: %v", err)
			}
		})
	}
}

// ============================================================================
// AC-4 CLASSIFICATION STRING REPRESENTATION TESTS
// ============================================================================

// TestClassification_String tests full classification string output.
func TestClassification_String(t *testing.T) {
	tests := []struct {
		name     string
		class    Classification
		expected string
	}{
		{
			name: "Simple Unclassified",
			class: Classification{
				Level: ClassificationUnclassified,
			},
			expected: "UNCLASSIFIED",
		},
		{
			name: "CUI Basic",
			class: Classification{
				Level: ClassificationCUI,
				CUI:   []CUIDesignation{CUIBasic},
			},
			expected: "CUI", // Basic is not printed
		},
		{
			name: "CUI with NOFORN",
			class: Classification{
				Level: ClassificationCUI,
				CUI:   []CUIDesignation{CUINOFORN},
			},
			expected: "CUI//NOFORN",
		},
		{
			name: "SECRET with caveat",
			class: Classification{
				Level:   ClassificationSecret,
				Caveats: []string{"NOFORN"},
			},
			expected: "SECRET//NOFORN",
		},
		{
			name: "TOP SECRET with multiple caveats",
			class: Classification{
				Level:   ClassificationTopSecret,
				Caveats: []string{"NOFORN", "ORCON"},
			},
			expected: "TOP SECRET//NOFORN//ORCON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.class.String()
			if result != tt.expected {
				t.Errorf("Classification.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// AC-4 CUI DESIGNATION TESTS
// ============================================================================

// TestCUIDesignation_String tests CUI designation string representations.
func TestCUIDesignation_String(t *testing.T) {
	tests := []struct {
		name        string
		designation CUIDesignation
		expected    string
	}{
		{name: "Basic", designation: CUIBasic, expected: "BASIC"},
		{name: "NOFORN", designation: CUINOFORN, expected: "NOFORN"},
		{name: "IMCON", designation: CUIIMCON, expected: "IMCON"},
		{name: "PROPIN", designation: CUIPropin, expected: "PROPIN"},
		{name: "ORCON", designation: CUIORCON, expected: "ORCON"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.designation.String()
			if result != tt.expected {
				t.Errorf("CUIDesignation.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// AC-4 API HEADERS TESTS
// ============================================================================

// TestGetAPIHeaders tests generation of classification headers for API responses.
func TestGetAPIHeaders(t *testing.T) {
	tests := []struct {
		name        string
		class       Classification
		expectLevel string
		expectCUI   bool
	}{
		{
			name: "Unclassified",
			class: Classification{
				Level: ClassificationUnclassified,
			},
			expectLevel: "UNCLASSIFIED",
			expectCUI:   false,
		},
		{
			name: "CUI with designations",
			class: Classification{
				Level: ClassificationCUI,
				CUI:   []CUIDesignation{CUINOFORN},
			},
			expectLevel: "CUI",
			expectCUI:   true,
		},
		{
			name: "Secret",
			class: Classification{
				Level: ClassificationSecret,
			},
			expectLevel: "SECRET",
			expectCUI:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := GetAPIHeaders(tt.class)

			if level := headers["X-Classification-Level"]; level != tt.expectLevel {
				t.Errorf("X-Classification-Level = %q, want %q", level, tt.expectLevel)
			}

			_, hasCUI := headers["X-CUI-Designations"]
			if hasCUI != tt.expectCUI {
				t.Errorf("X-CUI-Designations present = %v, want %v", hasCUI, tt.expectCUI)
			}
		})
	}
}

// ============================================================================
// AC-4 PORTION MARKING TESTS
// ============================================================================

// TestRenderPortionMarking tests inline portion marking output.
func TestRenderPortionMarking(t *testing.T) {
	tests := []struct {
		name         string
		class        Classification
		expectedMark string
	}{
		{
			name: "Unclassified portion",
			class: Classification{
				Level: ClassificationUnclassified,
			},
			expectedMark: "(U)",
		},
		{
			name: "CUI portion",
			class: Classification{
				Level: ClassificationCUI,
			},
			expectedMark: "(CUI)",
		},
		{
			name: "Confidential portion",
			class: Classification{
				Level: ClassificationConfidential,
			},
			expectedMark: "(C)",
		},
		{
			name: "Secret portion",
			class: Classification{
				Level: ClassificationSecret,
			},
			expectedMark: "(S)",
		},
		{
			name: "Top Secret portion",
			class: Classification{
				Level: ClassificationTopSecret,
			},
			expectedMark: "(TS)",
		},
		{
			name: "Secret with NOFORN",
			class: Classification{
				Level:   ClassificationSecret,
				Caveats: []string{"NOFORN"},
			},
			expectedMark: "(S//NF)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderPortionMarking(tt.class)
			if result != tt.expectedMark {
				t.Errorf("RenderPortionMarking() = %q, want %q", result, tt.expectedMark)
			}
		})
	}
}

// ============================================================================
// AC-4 IsClassified TESTS
// ============================================================================

// TestClassification_IsClassified tests the IsClassified method.
func TestClassification_IsClassified(t *testing.T) {
	tests := []struct {
		name       string
		level      ClassificationLevel
		isClassif  bool
	}{
		{
			name:       "Unclassified is not classified",
			level:      ClassificationUnclassified,
			isClassif:  false,
		},
		{
			name:       "CUI is not classified (controlled, but not classified)",
			level:      ClassificationCUI,
			isClassif:  false,
		},
		{
			name:       "Confidential IS classified",
			level:      ClassificationConfidential,
			isClassif:  true,
		},
		{
			name:       "Secret IS classified",
			level:      ClassificationSecret,
			isClassif:  true,
		},
		{
			name:       "Top Secret IS classified",
			level:      ClassificationTopSecret,
			isClassif:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			class := Classification{Level: tt.level}
			result := class.IsClassified()
			if result != tt.isClassif {
				t.Errorf("Classification{Level: %v}.IsClassified() = %v, want %v",
					tt.level, result, tt.isClassif)
			}
		})
	}
}

// ============================================================================
// AC-4 DEFAULT CLASSIFICATION TESTS
// ============================================================================

// TestDefaultClassification tests the default classification factory.
func TestDefaultClassification(t *testing.T) {
	def := DefaultClassification()

	if def.Level != ClassificationUnclassified {
		t.Errorf("DefaultClassification().Level = %v, want %v",
			def.Level, ClassificationUnclassified)
	}

	if len(def.CUI) != 0 {
		t.Errorf("DefaultClassification().CUI should be empty, got %v", def.CUI)
	}

	if len(def.Caveats) != 0 {
		t.Errorf("DefaultClassification().Caveats should be empty, got %v", def.Caveats)
	}

	if def.Portion {
		t.Error("DefaultClassification().Portion should be false")
	}
}

// TestClassificationFromEnv tests environment-based classification creation.
func TestClassificationFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected ClassificationLevel
	}{
		{
			name:     "Valid SECRET",
			envValue: "SECRET",
			expected: ClassificationSecret,
		},
		{
			name:     "Invalid falls back to Unclassified",
			envValue: "INVALID",
			expected: ClassificationUnclassified,
		},
		{
			name:     "Empty falls back to Unclassified",
			envValue: "",
			expected: ClassificationUnclassified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassificationFromEnv(tt.envValue)
			if result.Level != tt.expected {
				t.Errorf("ClassificationFromEnv(%q).Level = %v, want %v",
					tt.envValue, result.Level, tt.expected)
			}
		})
	}
}

// ============================================================================
// BENCHMARK TESTS
// ============================================================================

// BenchmarkParseClassification benchmarks classification parsing.
func BenchmarkParseClassification(b *testing.B) {
	inputs := []string{
		"UNCLASSIFIED",
		"CUI",
		"SECRET//NOFORN",
		"TOP SECRET//ORCON//NOFORN",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			ParseClassification(input)
		}
	}
}

// BenchmarkValidateClassification benchmarks classification validation.
func BenchmarkValidateClassification(b *testing.B) {
	class := Classification{
		Level:   ClassificationSecret,
		Caveats: []string{"NOFORN", "ORCON"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateClassification(class)
	}
}
