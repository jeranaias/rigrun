// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package router

import (
	"testing"
)

// TestClassifyComplexity tests the query complexity classification logic.
// Verifies that queries are properly categorized based on keywords and word count.
func TestClassifyComplexity(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected QueryComplexity
	}{
		// Trivial queries (< 5 words, no keywords)
		{
			name:     "trivial_single_word",
			query:    "hi",
			expected: ComplexityTrivial,
		},
		{
			name:     "trivial_two_words",
			query:    "hello world",
			expected: ComplexityTrivial,
		},
		{
			name:     "trivial_greeting",
			query:    "hey there",
			expected: ComplexityTrivial,
		},
		{
			name:     "trivial_short_question",
			query:    "thanks",
			expected: ComplexityTrivial,
		},

		// Simple queries (basic lookups)
		{
			name:     "simple_what_is",
			query:    "what is rust",
			expected: ComplexitySimple,
		},
		{
			name:     "simple_where_is",
			query:    "where is main",
			expected: ComplexitySimple,
		},
		{
			name:     "simple_find",
			query:    "find errors",
			expected: ComplexityComplex, // "errors" keyword triggers complex classification
		},
		{
			name:     "simple_list",
			query:    "list files",
			expected: ComplexitySimple,
		},

		// Complex queries (code-related keywords)
		{
			name:     "complex_bug_keyword",
			query:    "how do I fix this bug",
			expected: ComplexityComplex,
		},
		{
			name:     "complex_explain_keyword",
			query:    "explain how async runtime works",
			expected: ComplexityComplex,
		},
		{
			name:     "complex_review_keyword",
			query:    "review this code",
			expected: ComplexityComplex,
		},
		{
			name:     "complex_error_keyword",
			query:    "what is causing this error",
			expected: ComplexityComplex,
		},
		{
			name:     "complex_function_keyword",
			query:    "explain the function",
			expected: ComplexityComplex,
		},
		{
			name:     "complex_refactor_keyword",
			query:    "refactor this module",
			expected: ComplexityComplex,
		},
		{
			name:     "complex_implement_keyword",
			query:    "implement the feature",
			expected: ComplexityComplex,
		},

		// Moderate queries (how/why questions, 5+ words without code keywords)
		{
			name:     "moderate_six_words",
			query:    "tell me about this topic here",
			expected: ComplexityModerate,
		},
		{
			name:     "moderate_how_question",
			query:    "how are you",
			expected: ComplexityModerate,
		},
		{
			name:     "moderate_why_question",
			query:    "why not",
			expected: ComplexityModerate,
		},
		{
			name:     "moderate_five_words",
			query:    "one two three four five",
			expected: ComplexityModerate,
		},

		// Expert queries (architectural decisions, trade-offs)
		{
			name:     "expert_should_i",
			query:    "should I use microservices, what are the trade-offs",
			expected: ComplexityExpert,
		},
		{
			name:     "expert_architect_keyword",
			query:    "architect a new system",
			expected: ComplexityExpert,
		},
		{
			name:     "expert_design_pattern",
			query:    "what design pattern should I use",
			expected: ComplexityExpert,
		},
		{
			name:     "expert_trade_off",
			query:    "trade-off analysis",
			expected: ComplexityExpert,
		},
		{
			name:     "expert_best_approach",
			query:    "what is the best approach",
			expected: ComplexityExpert,
		},
		{
			name:     "expert_pros_and_cons",
			query:    "pros and cons of monorepo",
			expected: ComplexityExpert,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyComplexity(tt.query)
			if result != tt.expected {
				t.Errorf("ClassifyComplexity(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

// TestClassifyType tests the query type classification logic.
// Verifies that queries are categorized by their intent (lookup, debugging, etc.).
func TestClassifyType(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected QueryType
	}{
		// Lookup queries
		{
			name:     "lookup_what_is",
			query:    "what is a mutex",
			expected: QueryTypeLookup,
		},
		{
			name:     "lookup_syntax",
			query:    "syntax for loops",
			expected: QueryTypeLookup,
		},
		{
			name:     "lookup_list",
			query:    "list all commands",
			expected: QueryTypeLookup,
		},

		// Explanation queries
		{
			name:     "explanation_explain",
			query:    "explain async await",
			expected: QueryTypeExplanation,
		},
		{
			name:     "explanation_how_does",
			query:    "how does garbage collection work",
			expected: QueryTypeExplanation,
		},
		{
			name:     "explanation_why",
			query:    "why is this slow",
			expected: QueryTypeExplanation,
		},

		// Code generation queries
		{
			name:     "codegen_write",
			query:    "write a function to sort",
			expected: QueryTypeCodeGeneration,
		},
		{
			name:     "codegen_create",
			query:    "create a new class",
			expected: QueryTypeCodeGeneration,
		},
		{
			name:     "codegen_implement",
			query:    "implement binary search",
			expected: QueryTypeCodeGeneration,
		},
		{
			name:     "codegen_generate",
			query:    "generate test cases",
			expected: QueryTypeCodeGeneration,
		},

		// Debugging queries
		{
			name:     "debugging_fix",
			query:    "fix this bug",
			expected: QueryTypeDebugging,
		},
		{
			name:     "debugging_bug",
			query:    "bug in parser",
			expected: QueryTypeDebugging,
		},
		{
			name:     "debugging_debug",
			query:    "debug the issue",
			expected: QueryTypeDebugging,
		},
		{
			name:     "debugging_error",
			query:    "error handling",
			expected: QueryTypeDebugging,
		},

		// Review queries
		{
			name:     "review_review",
			query:    "review this code",
			expected: QueryTypeReview,
		},
		{
			name:     "review_check",
			query:    "check my implementation",
			expected: QueryTypeCodeGeneration, // "implementation" triggers code gen
		},

		// Refactoring queries
		{
			name:     "refactoring_refactor",
			query:    "refactor this method",
			expected: QueryTypeRefactoring,
		},
		{
			name:     "refactoring_improve",
			query:    "improve performance",
			expected: QueryTypeRefactoring,
		},
		{
			name:     "refactoring_optimize",
			query:    "optimize the algorithm",
			expected: QueryTypeRefactoring,
		},

		// Architecture queries
		{
			name:     "architecture_architect",
			query:    "architect the system",
			expected: QueryTypeArchitecture,
		},
		{
			name:     "architecture_design",
			query:    "design a schema",
			expected: QueryTypeArchitecture,
		},
		{
			name:     "architecture_should_i",
			query:    "should i use redis",
			expected: QueryTypeArchitecture,
		},
		{
			name:     "architecture_trade_off",
			query:    "trade-off between options",
			expected: QueryTypeArchitecture,
		},

		// Planning queries
		{
			name:     "planning_plan",
			query:    "plan the migration",
			expected: QueryTypePlanning,
		},
		{
			name:     "planning_roadmap",
			query:    "roadmap for v2",
			expected: QueryTypePlanning,
		},

		// General queries (default)
		{
			name:     "general_hello",
			query:    "hello there",
			expected: QueryTypeGeneral,
		},
		{
			name:     "general_thanks",
			query:    "thanks for helping",
			expected: QueryTypeGeneral,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyType(tt.query)
			if result != tt.expected {
				t.Errorf("ClassifyType(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

// TestWordCount tests the word counting function.
// Verifies correct handling of empty strings, single words, and whitespace.
func TestWordCount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty_string",
			input:    "",
			expected: 0,
		},
		{
			name:     "single_word",
			input:    "hello",
			expected: 1,
		},
		{
			name:     "two_words",
			input:    "hello world",
			expected: 2,
		},
		{
			name:     "spaced_out",
			input:    "  spaced   out  ",
			expected: 2,
		},
		{
			name:     "multiple_spaces",
			input:    "one    two     three",
			expected: 3,
		},
		{
			name:     "tabs_and_newlines",
			input:    "word1\tword2\nword3",
			expected: 3,
		},
		{
			name:     "only_whitespace",
			input:    "   \t\n   ",
			expected: 0,
		},
		{
			name:     "five_words",
			input:    "one two three four five",
			expected: 5,
		},
		{
			name:     "ten_words",
			input:    "one two three four five six seven eight nine ten",
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wordCount(tt.input)
			if result != tt.expected {
				t.Errorf("wordCount(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestComplexityString tests the String method of QueryComplexity.
func TestComplexityString(t *testing.T) {
	tests := []struct {
		complexity QueryComplexity
		expected   string
	}{
		{ComplexityTrivial, "Trivial"},
		{ComplexitySimple, "Simple"},
		{ComplexityModerate, "Moderate"},
		{ComplexityComplex, "Complex"},
		{ComplexityExpert, "Expert"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.complexity.String()
			if result != tt.expected {
				t.Errorf("QueryComplexity(%d).String() = %q, want %q", tt.complexity, result, tt.expected)
			}
		})
	}
}

// TestQueryTypeString tests the String method of QueryType.
func TestQueryTypeString(t *testing.T) {
	tests := []struct {
		queryType QueryType
		expected  string
	}{
		{QueryTypeLookup, "Lookup"},
		{QueryTypeExplanation, "Explanation"},
		{QueryTypeCodeGeneration, "CodeGeneration"},
		{QueryTypeRefactoring, "Refactoring"},
		{QueryTypeArchitecture, "Architecture"},
		{QueryTypeDebugging, "Debugging"},
		{QueryTypeReview, "Review"},
		{QueryTypePlanning, "Planning"},
		{QueryTypeGeneral, "General"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.queryType.String()
			if result != tt.expected {
				t.Errorf("QueryType(%d).String() = %q, want %q", tt.queryType, result, tt.expected)
			}
		})
	}
}

// TestQueryTypeModelHint tests the ModelHint method of QueryType.
func TestQueryTypeModelHint(t *testing.T) {
	tests := []struct {
		name      string
		queryType QueryType
		expected  string
	}{
		{"lookup_fast", QueryTypeLookup, "fast"},
		{"general_fast", QueryTypeGeneral, "fast"},
		{"explanation_fast", QueryTypeExplanation, "fast"},
		{"codegen_code", QueryTypeCodeGeneration, "code"},
		{"refactoring_code", QueryTypeRefactoring, "code"},
		{"debugging_code", QueryTypeDebugging, "code"},
		{"architecture_reasoning", QueryTypeArchitecture, "reasoning"},
		{"planning_reasoning", QueryTypePlanning, "reasoning"},
		{"review_reasoning", QueryTypeReview, "reasoning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.queryType.ModelHint()
			if result != tt.expected {
				t.Errorf("%s.ModelHint() = %q, want %q", tt.queryType, result, tt.expected)
			}
		})
	}
}

// TestClassifyComplexityCaseInsensitive verifies case-insensitive keyword matching.
func TestClassifyComplexityCaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected QueryComplexity
	}{
		{
			name:     "uppercase_should_i",
			query:    "SHOULD I use microservices",
			expected: ComplexityExpert,
		},
		{
			name:     "mixed_case_explain",
			query:    "ExPlAiN this concept",
			expected: ComplexityComplex,
		},
		{
			name:     "uppercase_what_is",
			query:    "WHAT IS a pointer",
			expected: ComplexitySimple,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyComplexity(tt.query)
			if result != tt.expected {
				t.Errorf("ClassifyComplexity(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

// TestClassifyTypeCaseInsensitive verifies case-insensitive keyword matching.
func TestClassifyTypeCaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected QueryType
	}{
		{
			name:     "uppercase_write",
			query:    "WRITE a function",
			expected: QueryTypeCodeGeneration,
		},
		{
			name:     "mixed_case_explain",
			query:    "ExPlAiN async",
			expected: QueryTypeExplanation,
		},
		{
			name:     "uppercase_review",
			query:    "REVIEW this",
			expected: QueryTypeReview,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyType(tt.query)
			if result != tt.expected {
				t.Errorf("ClassifyType(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

// BenchmarkClassifyComplexity benchmarks the complexity classification function.
func BenchmarkClassifyComplexity(b *testing.B) {
	queries := []string{
		"hi",
		"what is rust",
		"how do I fix this bug in my code",
		"should I use microservices or monolith, what are the trade-offs",
		"explain how the async runtime works in detail with examples",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, q := range queries {
			_ = ClassifyComplexity(q)
		}
	}
}

// BenchmarkClassifyType benchmarks the query type classification function.
func BenchmarkClassifyType(b *testing.B) {
	queries := []string{
		"what is a mutex",
		"explain async await",
		"write a function to sort",
		"fix this bug",
		"review this code",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, q := range queries {
			_ = ClassifyType(q)
		}
	}
}

// BenchmarkWordCount benchmarks the word count function.
func BenchmarkWordCount(b *testing.B) {
	inputs := []string{
		"",
		"hello",
		"hello world",
		"this is a longer sentence with many words in it",
		"  spaced   out   words   here  ",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, s := range inputs {
			_ = wordCount(s)
		}
	}
}
