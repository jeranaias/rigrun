// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package context provides the @ mention system for including context in messages.
package context

import (
	"strings"
	"testing"
)

// =============================================================================
// MENTION TYPE TESTS
// =============================================================================

func TestMentionType_String(t *testing.T) {
	tests := []struct {
		mentionType MentionType
		want        string
	}{
		{MentionFile, "file"},
		{MentionClipboard, "clipboard"},
		{MentionGit, "git"},
		{MentionCodebase, "codebase"},
		{MentionLastError, "error"},
		{MentionURL, "url"},
		{MentionType(99), "unknown"},
	}

	for _, tc := range tests {
		got := tc.mentionType.String()
		if got != tc.want {
			t.Errorf("MentionType(%d).String() = %q, want %q", tc.mentionType, got, tc.want)
		}
	}
}

// =============================================================================
// MENTION STRUCT TESTS
// =============================================================================

func TestMention_IsResolved(t *testing.T) {
	tests := []struct {
		name    string
		mention Mention
		want    bool
	}{
		{"unresolved", Mention{}, false},
		{"with content", Mention{Content: "some content"}, true},
		{"with error", Mention{Error: errTestError}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.mention.IsResolved()
			if got != tc.want {
				t.Errorf("IsResolved() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMention_HasError(t *testing.T) {
	tests := []struct {
		name    string
		mention Mention
		want    bool
	}{
		{"no error", Mention{}, false},
		{"with error", Mention{Error: errTestError}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.mention.HasError()
			if got != tc.want {
				t.Errorf("HasError() = %v, want %v", got, tc.want)
			}
		})
	}
}

// =============================================================================
// PARSER TESTS
// =============================================================================

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("NewParser() returned nil")
	}

	if len(p.patterns) == 0 {
		t.Error("Parser should have patterns")
	}
}

func TestParser_Parse_FileMention(t *testing.T) {
	p := NewParser()

	tests := []struct {
		input     string
		wantPath  string
		wantClean string
	}{
		{"@file:src/main.go", "src/main.go", ""},
		{`@file:"path with spaces.go"`, "path with spaces.go", ""},
		{`@file:'another path.go'`, "another path.go", ""},
		{"Check @file:main.go please", "main.go", "Check please"},
	}

	for _, tc := range tests {
		mentions, clean := p.Parse(tc.input)

		if len(mentions) != 1 {
			t.Errorf("Parse(%q) expected 1 mention, got %d", tc.input, len(mentions))
			continue
		}

		if mentions[0].Type != MentionFile {
			t.Errorf("Parse(%q) type = %v, want MentionFile", tc.input, mentions[0].Type)
		}

		if mentions[0].Path != tc.wantPath {
			t.Errorf("Parse(%q) path = %q, want %q", tc.input, mentions[0].Path, tc.wantPath)
		}

		if clean != tc.wantClean {
			t.Errorf("Parse(%q) clean = %q, want %q", tc.input, clean, tc.wantClean)
		}
	}
}

func TestParser_Parse_ClipboardMention(t *testing.T) {
	p := NewParser()

	mentions, clean := p.Parse("Use @clipboard content")

	if len(mentions) != 1 {
		t.Fatalf("Expected 1 mention, got %d", len(mentions))
	}

	if mentions[0].Type != MentionClipboard {
		t.Errorf("Type = %v, want MentionClipboard", mentions[0].Type)
	}

	if clean != "Use content" {
		t.Errorf("Clean = %q, want 'Use content'", clean)
	}
}

func TestParser_Parse_GitMention(t *testing.T) {
	p := NewParser()

	tests := []struct {
		input     string
		wantRange string
	}{
		{"@git", ""},
		{"@git:HEAD~3", "HEAD~3"},
		{"@git:main..feature", "main..feature"},
	}

	for _, tc := range tests {
		mentions, _ := p.Parse(tc.input)

		if len(mentions) != 1 {
			t.Errorf("Parse(%q) expected 1 mention, got %d", tc.input, len(mentions))
			continue
		}

		if mentions[0].Type != MentionGit {
			t.Errorf("Parse(%q) type = %v, want MentionGit", tc.input, mentions[0].Type)
		}

		if mentions[0].Range != tc.wantRange {
			t.Errorf("Parse(%q) range = %q, want %q", tc.input, mentions[0].Range, tc.wantRange)
		}
	}
}

func TestParser_Parse_URLMention(t *testing.T) {
	p := NewParser()

	tests := []struct {
		input   string
		wantURL string
	}{
		{"@url:https://example.com", "https://example.com"},
		{`@url:"https://example.com/path with spaces"`, "https://example.com/path with spaces"},
	}

	for _, tc := range tests {
		mentions, _ := p.Parse(tc.input)

		if len(mentions) != 1 {
			t.Errorf("Parse(%q) expected 1 mention, got %d", tc.input, len(mentions))
			continue
		}

		if mentions[0].Type != MentionURL {
			t.Errorf("Parse(%q) type = %v, want MentionURL", tc.input, mentions[0].Type)
		}

		if mentions[0].URL != tc.wantURL {
			t.Errorf("Parse(%q) URL = %q, want %q", tc.input, mentions[0].URL, tc.wantURL)
		}
	}
}

func TestParser_Parse_MultipleMentions(t *testing.T) {
	p := NewParser()

	input := "Check @file:main.go and @clipboard"
	mentions, clean := p.Parse(input)

	if len(mentions) != 2 {
		t.Fatalf("Expected 2 mentions, got %d", len(mentions))
	}

	// Verify types
	hasFile := false
	hasClipboard := false
	for _, m := range mentions {
		if m.Type == MentionFile {
			hasFile = true
		}
		if m.Type == MentionClipboard {
			hasClipboard = true
		}
	}

	if !hasFile || !hasClipboard {
		t.Error("Expected both file and clipboard mentions")
	}

	if clean != "Check and" {
		t.Errorf("Clean = %q, want 'Check and'", clean)
	}
}

func TestParser_Parse_NoMentions(t *testing.T) {
	p := NewParser()

	mentions, clean := p.Parse("Hello world")

	if len(mentions) != 0 {
		t.Errorf("Expected 0 mentions, got %d", len(mentions))
	}

	if clean != "Hello world" {
		t.Errorf("Clean = %q, want 'Hello world'", clean)
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestHasMentions(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"@file:test.go", true},
		{"@clipboard", true},
		{"@git", true},
		{"@codebase", true},
		{"@error", true},
		{"@url:https://example.com", true},
		{"hello world", false},
		{"email@example.com", false}, // Should not match
	}

	for _, tc := range tests {
		got := HasMentions(tc.input)
		if got != tc.want {
			t.Errorf("HasMentions(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseMentionTypes(t *testing.T) {
	types := ParseMentionTypes()

	expected := []string{"@file:", "@clipboard", "@git", "@codebase", "@error", "@url:"}

	if len(types) != len(expected) {
		t.Errorf("ParseMentionTypes() length = %d, want %d", len(types), len(expected))
	}

	for i, want := range expected {
		if i < len(types) && types[i] != want {
			t.Errorf("ParseMentionTypes()[%d] = %q, want %q", i, types[i], want)
		}
	}
}

func TestGetMentionAtPosition(t *testing.T) {
	input := "Check @file:main.go please"

	// Position inside the mention
	m := GetMentionAtPosition(input, 10)
	if m == nil {
		t.Error("GetMentionAtPosition should return mention at position 10")
	}
	if m != nil && m.Type != MentionFile {
		t.Error("Mention at position 10 should be file type")
	}

	// Position outside any mention
	m = GetMentionAtPosition(input, 0)
	if m != nil {
		t.Error("GetMentionAtPosition should return nil at position 0")
	}
}

func TestHighlightMentions(t *testing.T) {
	input := "Check @file:main.go"

	highlighter := func(mention string) string {
		return "[" + mention + "]"
	}

	result := HighlightMentions(input, highlighter)

	if !strings.Contains(result, "[@file:main.go]") {
		t.Errorf("HighlightMentions should wrap mention, got: %s", result)
	}

	// Nil highlighter should return original
	result = HighlightMentions(input, nil)
	if result != input {
		t.Errorf("HighlightMentions with nil should return original, got: %s", result)
	}
}

// =============================================================================
// SUMMARY TESTS
// =============================================================================

func TestSummarize(t *testing.T) {
	mentions := []Mention{
		{Type: MentionFile, Path: "main.go"},
		{Type: MentionFile, Path: "test.go"},
		{Type: MentionClipboard},
		{Type: MentionGit, Range: "HEAD~3"},
		{Type: MentionURL, URL: "https://example.com"},
	}

	summary := Summarize(mentions)

	if summary.TotalCount != 5 {
		t.Errorf("TotalCount = %d, want 5", summary.TotalCount)
	}

	if summary.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", summary.FileCount)
	}

	if len(summary.Files) != 2 {
		t.Errorf("Files length = %d, want 2", len(summary.Files))
	}

	if !summary.HasClipboard {
		t.Error("HasClipboard should be true")
	}

	if !summary.HasGit {
		t.Error("HasGit should be true")
	}

	if summary.GitRange != "HEAD~3" {
		t.Errorf("GitRange = %q, want 'HEAD~3'", summary.GitRange)
	}

	if !summary.HasURL {
		t.Error("HasURL should be true")
	}
}

func TestMentionSummary_FormatSummary(t *testing.T) {
	tests := []struct {
		name    string
		summary MentionSummary
		want    string
	}{
		{
			"empty",
			MentionSummary{},
			"",
		},
		{
			"one file",
			MentionSummary{TotalCount: 1, FileCount: 1, Files: []string{"main.go"}},
			"1 file",
		},
		{
			"multiple files",
			MentionSummary{TotalCount: 2, FileCount: 2, Files: []string{"a.go", "b.go"}},
			"2 files",
		},
		{
			"clipboard",
			MentionSummary{TotalCount: 1, HasClipboard: true},
			"clipboard",
		},
		{
			"git with range",
			MentionSummary{TotalCount: 1, HasGit: true, GitRange: "HEAD~3"},
			"git:HEAD~3",
		},
		{
			"multiple types",
			MentionSummary{TotalCount: 3, FileCount: 1, Files: []string{"a.go"}, HasClipboard: true, HasGit: true},
			"1 file, clipboard, git",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.summary.FormatSummary()
			if got != tc.want {
				t.Errorf("FormatSummary() = %q, want %q", got, tc.want)
			}
		})
	}
}

// =============================================================================
// EXPANDER TESTS
// =============================================================================

func TestNewExpander(t *testing.T) {
	// With nil fetcher
	e := NewExpander(nil)
	if e == nil {
		t.Fatal("NewExpander(nil) returned nil")
	}

	// With custom fetcher
	f := NewFetcher(nil)
	e = NewExpander(f)
	if e == nil {
		t.Fatal("NewExpander(fetcher) returned nil")
	}
}

func TestExpander_Expand_NoMentions(t *testing.T) {
	e := NewExpander(nil)

	result := e.Expand("Hello world")

	if result.OriginalMessage != "Hello world" {
		t.Errorf("OriginalMessage = %q, want 'Hello world'", result.OriginalMessage)
	}

	if result.ExpandedMessage != "Hello world" {
		t.Errorf("ExpandedMessage = %q, want 'Hello world'", result.ExpandedMessage)
	}

	if len(result.Mentions) != 0 {
		t.Errorf("Mentions length = %d, want 0", len(result.Mentions))
	}
}

func TestExpander_Expand_WithMentions(t *testing.T) {
	e := NewExpander(nil)

	result := e.Expand("Check @clipboard please")

	if result.OriginalMessage != "Check @clipboard please" {
		t.Errorf("OriginalMessage = %q", result.OriginalMessage)
	}

	if len(result.Mentions) != 1 {
		t.Errorf("Mentions length = %d, want 1", len(result.Mentions))
	}

	if result.CleanMessage != "Check please" {
		t.Errorf("CleanMessage = %q, want 'Check please'", result.CleanMessage)
	}
}

func TestExpansionResult_HasErrors(t *testing.T) {
	result := &ExpansionResult{}

	if result.HasErrors() {
		t.Error("HasErrors should be false for empty errors")
	}

	result.Errors = append(result.Errors, MentionError{
		Mention: Mention{Raw: "@test"},
		Error:   errTestError,
	})

	if !result.HasErrors() {
		t.Error("HasErrors should be true with errors")
	}
}

func TestExpansionResult_ErrorSummary(t *testing.T) {
	result := &ExpansionResult{}

	if result.ErrorSummary() != "" {
		t.Error("ErrorSummary should be empty for no errors")
	}

	result.Errors = append(result.Errors, MentionError{
		Mention: Mention{Raw: "@test"},
		Error:   errTestError,
	})

	summary := result.ErrorSummary()
	if summary == "" {
		t.Error("ErrorSummary should not be empty with errors")
	}

	if !strings.Contains(summary, "@test") {
		t.Errorf("ErrorSummary should contain mention, got: %s", summary)
	}
}

// =============================================================================
// CONVENIENCE FUNCTION TESTS
// =============================================================================

func TestQuickExpand(t *testing.T) {
	result := QuickExpand("Hello world")
	if result == nil {
		t.Fatal("QuickExpand returned nil")
	}

	if result.OriginalMessage != "Hello world" {
		t.Errorf("OriginalMessage = %q", result.OriginalMessage)
	}
}

func TestQuickExpandWithConfig(t *testing.T) {
	config := &FetcherConfig{
		MaxFileSize: 1024,
	}

	result := QuickExpandWithConfig("Hello world", config)
	if result == nil {
		t.Fatal("QuickExpandWithConfig returned nil")
	}
}

// =============================================================================
// FORMAT OPTIONS TESTS
// =============================================================================

func TestDefaultFormatOptions(t *testing.T) {
	opts := DefaultFormatOptions()

	if opts == nil {
		t.Fatal("DefaultFormatOptions returned nil")
	}

	if !opts.UseXMLTags {
		t.Error("UseXMLTags should be true by default")
	}

	if !opts.IncludeLineNumbers {
		t.Error("IncludeLineNumbers should be true by default")
	}

	if opts.MaxContentLength != 0 {
		t.Errorf("MaxContentLength = %d, want 0", opts.MaxContentLength)
	}
}

func TestExpander_ExpandWithOptions(t *testing.T) {
	e := NewExpander(nil)

	opts := &FormatOptions{
		MaxContentLength: 10,
	}

	result := e.ExpandWithOptions("Hello world, this is a long message", opts)
	if result == nil {
		t.Fatal("ExpandWithOptions returned nil")
	}

	// Message should be truncated
	if len(result.ExpandedMessage) > 13 { // 10 + "..."
		t.Errorf("ExpandedMessage should be truncated, got length %d", len(result.ExpandedMessage))
	}
}

// =============================================================================
// CONTEXT SIZE TESTS
// =============================================================================

func TestExpander_EstimateContextSize(t *testing.T) {
	e := NewExpander(nil)

	size := e.EstimateContextSize("Hello world")

	// "Hello world" = 11 chars, roughly 3 tokens
	if size < 2 || size > 5 {
		t.Errorf("EstimateContextSize = %d, expected ~3", size)
	}
}

func TestExpander_GetContextSizeInfo(t *testing.T) {
	e := NewExpander(nil)

	info := e.GetContextSizeInfo("Hello world")

	if info.TotalChars != 11 {
		t.Errorf("TotalChars = %d, want 11", info.TotalChars)
	}

	if info.MessageChars != 11 {
		t.Errorf("MessageChars = %d, want 11", info.MessageChars)
	}

	if info.EstimatedTokens < 1 {
		t.Errorf("EstimatedTokens = %d, expected >= 1", info.EstimatedTokens)
	}
}

// =============================================================================
// PREVIEW TESTS
// =============================================================================

func TestExpander_GeneratePreview_NoMentions(t *testing.T) {
	e := NewExpander(nil)

	preview := e.GeneratePreview("Hello world")

	if preview != "No context mentions found" {
		t.Errorf("Preview = %q, expected 'No context mentions found'", preview)
	}
}

func TestExpander_GeneratePreview_WithMentions(t *testing.T) {
	e := NewExpander(nil)

	preview := e.GeneratePreview("@file:main.go")

	if !strings.Contains(preview, "Context to include") {
		t.Errorf("Preview should contain header, got: %s", preview)
	}

	if !strings.Contains(preview, "@file:main.go") {
		t.Errorf("Preview should contain mention, got: %s", preview)
	}
}

// =============================================================================
// STREAMING EXPANDER TESTS
// =============================================================================

func TestNewStreamingExpander(t *testing.T) {
	se := NewStreamingExpander(nil)
	if se == nil {
		t.Fatal("NewStreamingExpander returned nil")
	}
}

func TestStreamingExpander_Start(t *testing.T) {
	se := NewStreamingExpander(nil)

	mentions := se.Start("@file:main.go @clipboard")

	if len(mentions) != 2 {
		t.Errorf("Start returned %d mentions, want 2", len(mentions))
	}
}

func TestStreamingExpander_Progress(t *testing.T) {
	se := NewStreamingExpander(nil)

	// No mentions
	if se.Progress() != 1.0 {
		t.Error("Progress should be 1.0 with no mentions")
	}

	se.Start("@file:main.go @clipboard")

	if se.Progress() != 0.0 {
		t.Errorf("Progress = %f, want 0.0 after Start", se.Progress())
	}
}

func TestStreamingExpander_FetchNext(t *testing.T) {
	se := NewStreamingExpander(nil)
	se.Start("@file:main.go")

	m, hasMore := se.FetchNext()

	if m == nil {
		t.Error("FetchNext should return mention")
	}

	if hasMore {
		t.Error("hasMore should be false with single mention")
	}

	// Second call should return nil
	m, hasMore = se.FetchNext()
	if m != nil || hasMore {
		t.Error("FetchNext should return nil, false when done")
	}
}

func TestStreamingExpander_Complete(t *testing.T) {
	se := NewStreamingExpander(nil)
	se.Start("@file:main.go")
	se.FetchNext()

	result := se.Complete("@file:main.go")

	if result == nil {
		t.Fatal("Complete returned nil")
	}

	if result.OriginalMessage != "@file:main.go" {
		t.Errorf("OriginalMessage = %q", result.OriginalMessage)
	}
}

// =============================================================================
// TEST HELPERS
// =============================================================================

type testError struct{}

func (e testError) Error() string { return "test error" }

var errTestError = testError{}
