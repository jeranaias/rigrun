// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package commands provides the slash command system for the TUI.
package commands

import (
	"testing"
)

// =============================================================================
// PARSER TESTS
// =============================================================================

func TestIsCommand(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/help", true},
		{"/model qwen", true},
		{"  /help", true},
		{"hello", false},
		{"hello /help", false},
		{"", false},
		{"/", true},
	}

	for _, tc := range tests {
		got := IsCommand(tc.input)
		if got != tc.want {
			t.Errorf("IsCommand(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestExtractCommandName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/help", "/help"},
		{"/model qwen", "/model"},
		{"/save my-session", "/save"},
		{"  /help  ", "/help"},
		{"hello", ""},
		{"/", "/"},
	}

	for _, tc := range tests {
		got := ExtractCommandName(tc.input)
		if got != tc.want {
			t.Errorf("ExtractCommandName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestGetPartialCommand(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/hel", "/hel"},
		{"/help", "/help"},
		{"/model ", ""},       // Space after command means complete
		{"/model qwen", ""},   // Has arguments
		{"hello", ""},
	}

	for _, tc := range tests {
		got := GetPartialCommand(tc.input)
		if got != tc.want {
			t.Errorf("GetPartialCommand(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestGetPartialArg(t *testing.T) {
	tests := []struct {
		input    string
		wantIdx  int
		wantPart string
	}{
		{"/help", 0, ""},
		{"/model qwe", 0, "qwe"},
		// Note: trailing space is trimmed by the function before checking,
		// so it returns the last part as partial text
		{"/model qwen ", 0, "qwen"},
		{"/save my session", 1, "session"},
	}

	for _, tc := range tests {
		gotIdx, gotPart := GetPartialArg(tc.input)
		if gotIdx != tc.wantIdx || gotPart != tc.wantPart {
			t.Errorf("GetPartialArg(%q) = (%d, %q), want (%d, %q)",
				tc.input, gotIdx, gotPart, tc.wantIdx, tc.wantPart)
		}
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"/help", []string{"/help"}},
		{"/model qwen", []string{"/model", "qwen"}},
		{`/save "my session"`, []string{"/save", "my session"}},
		{`/save 'my session'`, []string{"/save", "my session"}},
		{"/config key value", []string{"/config", "key", "value"}},
		{`/export "file with spaces.md"`, []string{"/export", "file with spaces.md"}},
	}

	for _, tc := range tests {
		got := ParseArgs(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("ParseArgs(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("ParseArgs(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

// =============================================================================
// REGISTRY TESTS
// =============================================================================

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}

	// Should have built-in commands
	if len(r.commands) == 0 {
		t.Error("Registry should have built-in commands")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	cmd := &Command{
		Name:        "/test",
		Aliases:     []string{"/t"},
		Description: "Test command",
	}

	r.Register(cmd)

	if r.Get("/test") == nil {
		t.Error("Should get command by name")
	}

	if r.Get("/t") == nil {
		t.Error("Should get command by alias")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	// Built-in commands
	if r.Get("/help") == nil {
		t.Error("/help command should exist")
	}

	if r.Get("/h") == nil {
		t.Error("/h alias should resolve to /help")
	}

	if r.Get("/?") == nil {
		t.Error("/? alias should resolve to /help")
	}

	if r.Get("/nonexistent") != nil {
		t.Error("/nonexistent should return nil")
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	all := r.All()

	if len(all) == 0 {
		t.Error("All() should return commands")
	}

	// Check that essential commands are present
	found := make(map[string]bool)
	for _, cmd := range all {
		found[cmd.Name] = true
	}

	essentials := []string{"/help", "/quit", "/new", "/model", "/save", "/load"}
	for _, name := range essentials {
		if !found[name] {
			t.Errorf("Essential command %s not found in All()", name)
		}
	}
}

func TestRegistry_ByCategory(t *testing.T) {
	r := NewRegistry()
	byCategory := r.ByCategory()

	if len(byCategory) == 0 {
		t.Error("ByCategory() should return categories")
	}

	// Check that expected categories exist
	expectedCategories := []string{"Navigation", "Conversation", "Model"}
	for _, cat := range expectedCategories {
		if _, ok := byCategory[cat]; !ok {
			t.Errorf("Expected category %q not found", cat)
		}
	}

	// Hidden commands should not appear
	for _, cmds := range byCategory {
		for _, cmd := range cmds {
			if cmd.Hidden {
				t.Errorf("Hidden command %s should not appear in ByCategory()", cmd.Name)
			}
		}
	}
}

// =============================================================================
// PARSER TESTS
// =============================================================================

func TestParser_Parse(t *testing.T) {
	r := NewRegistry()
	p := NewParser(r)

	tests := []struct {
		input     string
		isCommand bool
		cmdName   string
		argsLen   int
	}{
		{"/help", true, "/help", 0},
		{"/model qwen", true, "/model", 1},
		{"hello world", false, "", 0},
		{"/nonexistent", true, "/nonexistent", 0},
		{`/save "my session"`, true, "/save", 1},
	}

	for _, tc := range tests {
		result := p.Parse(tc.input)

		if result.IsCommand != tc.isCommand {
			t.Errorf("Parse(%q).IsCommand = %v, want %v", tc.input, result.IsCommand, tc.isCommand)
		}

		if result.CommandName != tc.cmdName {
			t.Errorf("Parse(%q).CommandName = %q, want %q", tc.input, result.CommandName, tc.cmdName)
		}

		if len(result.Args) != tc.argsLen {
			t.Errorf("Parse(%q) args length = %d, want %d", tc.input, len(result.Args), tc.argsLen)
		}
	}
}

func TestParser_Parse_CommandLookup(t *testing.T) {
	r := NewRegistry()
	p := NewParser(r)

	// Existing command
	result := p.Parse("/help")
	if result.Command == nil {
		t.Error("Parse(/help).Command should not be nil")
	}

	// Alias lookup
	result = p.Parse("/h")
	if result.Command == nil {
		t.Error("Parse(/h).Command should not be nil (alias)")
	}

	// Non-existent command
	result = p.Parse("/nonexistent")
	if result.Command != nil {
		t.Error("Parse(/nonexistent).Command should be nil")
	}
}

// =============================================================================
// VALIDATION TESTS
// =============================================================================

func TestValidateArgs(t *testing.T) {
	// Command with required argument
	cmdWithRequired := &Command{
		Name: "/test",
		Args: []ArgDef{
			{Name: "required_arg", Required: true, Description: "A required argument"},
		},
	}

	// Missing required argument
	err := ValidateArgs(cmdWithRequired, []string{})
	if err == nil {
		t.Error("ValidateArgs should return error for missing required argument")
	}

	// Provided required argument
	err = ValidateArgs(cmdWithRequired, []string{"value"})
	if err != nil {
		t.Errorf("ValidateArgs should not error when required argument provided: %v", err)
	}

	// Command with enum argument
	cmdWithEnum := &Command{
		Name: "/mode",
		Args: []ArgDef{
			{Name: "mode", Required: true, Type: ArgTypeEnum, Values: []string{"local", "cloud", "hybrid"}},
		},
	}

	// Valid enum value
	err = ValidateArgs(cmdWithEnum, []string{"local"})
	if err != nil {
		t.Errorf("ValidateArgs should accept valid enum value: %v", err)
	}

	// Invalid enum value
	err = ValidateArgs(cmdWithEnum, []string{"invalid"})
	if err == nil {
		t.Error("ValidateArgs should reject invalid enum value")
	}

	// Case insensitive enum
	err = ValidateArgs(cmdWithEnum, []string{"LOCAL"})
	if err != nil {
		t.Errorf("ValidateArgs should accept case-insensitive enum: %v", err)
	}

	// Nil command should not error
	err = ValidateArgs(nil, []string{"anything"})
	if err != nil {
		t.Errorf("ValidateArgs(nil) should not error: %v", err)
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Command:  "/test",
		Arg:      "arg1",
		Message:  "invalid value",
		Got:      "bad",
		Expected: "good1, good2",
	}

	errStr := err.Error()

	// Check that all parts are in the error string
	if errStr == "" {
		t.Error("Error() should return non-empty string")
	}

	// Should contain command, argument, message, got, expected
	contains := []string{"/test", "arg1", "invalid value", "bad", "good1, good2"}
	for _, s := range contains {
		if !containsStr(errStr, s) {
			t.Errorf("Error() should contain %q, got: %s", s, errStr)
		}
	}
}

// =============================================================================
// CONTEXT TESTS
// =============================================================================

func TestNewContext(t *testing.T) {
	ctx := NewContext(nil, nil, nil, nil, nil)
	if ctx == nil {
		t.Fatal("NewContext() returned nil")
	}
}

func TestContext_WithHandlerContext(t *testing.T) {
	ctx := NewContext(nil, nil, nil, nil, nil)
	hctx := &HandlerContext{}

	result := ctx.WithHandlerContext(hctx)

	if result != ctx {
		t.Error("WithHandlerContext should return same context")
	}

	if ctx.HandlerCtx != hctx {
		t.Error("HandlerCtx should be set")
	}
}

func TestContext_RecordActivity(t *testing.T) {
	// With nil session, should not panic
	ctx := NewContext(nil, nil, nil, nil, nil)
	ctx.RecordActivity() // Should not panic
}

func TestContext_MarkDirty(t *testing.T) {
	// With nil session, should not panic
	ctx := NewContext(nil, nil, nil, nil, nil)
	ctx.MarkDirty() // Should not panic
}

// =============================================================================
// ARGTYPE TESTS
// =============================================================================

func TestArgType_Values(t *testing.T) {
	// Verify ArgType constants are defined
	types := []ArgType{
		ArgTypeString,
		ArgTypeModel,
		ArgTypeSession,
		ArgTypeFile,
		ArgTypeEnum,
		ArgTypeTool,
		ArgTypeConfig,
	}

	for i, at := range types {
		if int(at) != i {
			t.Errorf("ArgType constant %d has unexpected value %d", i, at)
		}
	}
}

// =============================================================================
// COMPLETION TESTS
// =============================================================================

func TestCompletion_Fields(t *testing.T) {
	c := Completion{
		Value:       "/help",
		Display:     "/help - Show help",
		Description: "Show help and available commands",
		Score:       100,
		IsCurrent:   true,
	}

	if c.Value != "/help" {
		t.Error("Completion.Value not set correctly")
	}

	if c.Score != 100 {
		t.Error("Completion.Score not set correctly")
	}

	if !c.IsCurrent {
		t.Error("Completion.IsCurrent not set correctly")
	}
}

// =============================================================================
// COMMAND DEFINITION TESTS
// =============================================================================

func TestCommand_Fields(t *testing.T) {
	cmd := &Command{
		Name:        "/test",
		Aliases:     []string{"/t", "/tst"},
		Description: "Test command",
		Usage:       "/test <arg>",
		Category:    "Testing",
		Hidden:      false,
		Args: []ArgDef{
			{Name: "arg", Required: true, Type: ArgTypeString, Description: "Test argument"},
		},
	}

	if cmd.Name != "/test" {
		t.Error("Command.Name not set correctly")
	}

	if len(cmd.Aliases) != 2 {
		t.Error("Command.Aliases not set correctly")
	}

	if cmd.Category != "Testing" {
		t.Error("Command.Category not set correctly")
	}

	if len(cmd.Args) != 1 {
		t.Error("Command.Args not set correctly")
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func containsStr(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(haystack) > len(needle) && (haystack[:len(needle)] == needle ||
		containsStr(haystack[1:], needle)))
}
