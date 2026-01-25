// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package chat provides the chat view component for the TUI.
//
// This file contains tests for vim mode functionality.
package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// VIM HANDLER TESTS
// =============================================================================

func TestVimHandler_Enabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{"enabled", true},
		{"disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vh := NewVimHandler(tt.enabled)
			if vh.Enabled() != tt.enabled {
				t.Errorf("Expected enabled=%v, got %v", tt.enabled, vh.Enabled())
			}
		})
	}
}

func TestVimHandler_ModeTransitions(t *testing.T) {
	vh := NewVimHandler(true)
	vp := viewport.New(80, 20)
	input := textinput.New()

	// Should start in normal mode
	if vh.Mode() != VimModeNormal {
		t.Errorf("Expected VimModeNormal, got %v", vh.Mode())
	}

	// Press 'i' to enter insert mode
	key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}
	consumed, _ := vh.HandleKey(key, &vp, &input)
	if !consumed {
		t.Error("Expected 'i' to be consumed in normal mode")
	}
	if vh.Mode() != VimModeInsert {
		t.Errorf("Expected VimModeInsert after 'i', got %v", vh.Mode())
	}

	// Press 'esc' to return to normal mode
	escKey := tea.KeyMsg{Type: tea.KeyEsc}
	consumed, _ = vh.HandleKey(escKey, &vp, &input)
	if !consumed {
		t.Error("Expected 'esc' to be consumed in insert mode")
	}
	if vh.Mode() != VimModeNormal {
		t.Errorf("Expected VimModeNormal after 'esc', got %v", vh.Mode())
	}
}

func TestVimHandler_Navigation_j_k(t *testing.T) {
	vh := NewVimHandler(true)
	vp := viewport.New(80, 20)
	input := textinput.New()

	// Set some content in viewport
	vp.SetContent(strings.Repeat("line\n", 50))
	vp.GotoTop()
	initialOffset := vp.YOffset

	// Press 'j' to scroll down
	jKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	consumed, _ := vh.HandleKey(jKey, &vp, &input)
	if !consumed {
		t.Error("Expected 'j' to be consumed in normal mode")
	}
	if vp.YOffset <= initialOffset {
		t.Errorf("Expected YOffset to increase after 'j', got %d (was %d)", vp.YOffset, initialOffset)
	}

	// Press 'k' to scroll up
	currentOffset := vp.YOffset
	kKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	consumed, _ = vh.HandleKey(kKey, &vp, &input)
	if !consumed {
		t.Error("Expected 'k' to be consumed in normal mode")
	}
	if vp.YOffset >= currentOffset {
		t.Errorf("Expected YOffset to decrease after 'k', got %d (was %d)", vp.YOffset, currentOffset)
	}
}

func TestVimHandler_Navigation_gg_G(t *testing.T) {
	vh := NewVimHandler(true)
	vp := viewport.New(80, 20)
	input := textinput.New()

	// Set some content in viewport
	vp.SetContent(strings.Repeat("line\n", 50))
	vp.GotoBottom()

	// Press 'g' twice to go to top
	gKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
	consumed, _ := vh.HandleKey(gKey, &vp, &input)
	if !consumed {
		t.Error("Expected first 'g' to be consumed")
	}
	consumed, _ = vh.HandleKey(gKey, &vp, &input)
	if !consumed {
		t.Error("Expected second 'g' to be consumed")
	}
	if vp.YOffset != 0 {
		t.Errorf("Expected YOffset=0 after 'gg', got %d", vp.YOffset)
	}

	// Press 'G' to go to bottom
	GKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}
	consumed, _ = vh.HandleKey(GKey, &vp, &input)
	if !consumed {
		t.Error("Expected 'G' to be consumed")
	}
	if vp.YOffset == 0 {
		t.Error("Expected YOffset>0 after 'G'")
	}
}

func TestVimHandler_NumericPrefix(t *testing.T) {
	vh := NewVimHandler(true)
	vp := viewport.New(80, 20)
	input := textinput.New()

	// Set some content in viewport
	vp.SetContent(strings.Repeat("line\n", 50))
	vp.GotoTop()
	initialOffset := vp.YOffset

	// Press '5j' to scroll down 5 lines
	fiveKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}}
	consumed, _ := vh.HandleKey(fiveKey, &vp, &input)
	if !consumed {
		t.Error("Expected '5' to be consumed")
	}

	jKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	consumed, _ = vh.HandleKey(jKey, &vp, &input)
	if !consumed {
		t.Error("Expected 'j' to be consumed")
	}

	// Should have scrolled more than 1 line
	if vp.YOffset <= initialOffset+1 {
		t.Errorf("Expected YOffset > %d after '5j', got %d", initialOffset+1, vp.YOffset)
	}
}

func TestVimHandler_CommandMode(t *testing.T) {
	vh := NewVimHandler(true)
	vp := viewport.New(80, 20)
	input := textinput.New()

	// Press ':' to enter command mode
	colonKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}}
	consumed, _ := vh.HandleKey(colonKey, &vp, &input)
	if !consumed {
		t.Error("Expected ':' to be consumed")
	}
	if vh.Mode() != VimModeCommand {
		t.Errorf("Expected VimModeCommand, got %v", vh.Mode())
	}

	// Type 'w' to build command
	wKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}}
	consumed, _ = vh.HandleKey(wKey, &vp, &input)
	if !consumed {
		t.Error("Expected 'w' to be consumed in command mode")
	}

	cmdBuffer := vh.GetCommandBuffer()
	if cmdBuffer != ":w" {
		t.Errorf("Expected command buffer ':w', got '%s'", cmdBuffer)
	}

	// Press 'esc' to exit command mode
	escKey := tea.KeyMsg{Type: tea.KeyEsc}
	consumed, _ = vh.HandleKey(escKey, &vp, &input)
	if !consumed {
		t.Error("Expected 'esc' to be consumed")
	}
	if vh.Mode() != VimModeNormal {
		t.Errorf("Expected VimModeNormal after esc, got %v", vh.Mode())
	}
}

func TestVimHandler_VisualMode(t *testing.T) {
	vh := NewVimHandler(true)
	vp := viewport.New(80, 20)
	input := textinput.New()

	// Press 'v' to enter visual mode
	vKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}}
	consumed, _ := vh.HandleKey(vKey, &vp, &input)
	if !consumed {
		t.Error("Expected 'v' to be consumed")
	}
	if vh.Mode() != VimModeVisual {
		t.Errorf("Expected VimModeVisual, got %v", vh.Mode())
	}

	// Press 'esc' to exit visual mode
	escKey := tea.KeyMsg{Type: tea.KeyEsc}
	consumed, _ = vh.HandleKey(escKey, &vp, &input)
	if !consumed {
		t.Error("Expected 'esc' to be consumed")
	}
	if vh.Mode() != VimModeNormal {
		t.Errorf("Expected VimModeNormal after esc, got %v", vh.Mode())
	}
}

func TestVimHandler_DisabledPassthrough(t *testing.T) {
	vh := NewVimHandler(false)
	vp := viewport.New(80, 20)
	input := textinput.New()

	// When disabled, all keys should pass through
	keys := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'j'}},
		{Type: tea.KeyRunes, Runes: []rune{'k'}},
		{Type: tea.KeyRunes, Runes: []rune{'i'}},
		{Type: tea.KeyEsc},
	}

	for _, key := range keys {
		consumed, _ := vh.HandleKey(key, &vp, &input)
		if consumed {
			t.Errorf("Expected key %v to NOT be consumed when vim mode disabled", key)
		}
	}
}

func TestVimHandler_ModeString(t *testing.T) {
	vh := NewVimHandler(true)

	tests := []struct {
		mode     VimMode
		expected string
	}{
		{VimModeNormal, "NORMAL"},
		{VimModeInsert, "INSERT"},
		{VimModeVisual, "VISUAL"},
		{VimModeCommand, "COMMAND"},
	}

	for _, tt := range tests {
		vh.mode = tt.mode
		if vh.ModeString() != tt.expected {
			t.Errorf("Expected mode string '%s', got '%s'", tt.expected, vh.ModeString())
		}
	}

	// When disabled, should return empty string
	vh.SetEnabled(false)
	if vh.ModeString() != "" {
		t.Errorf("Expected empty mode string when disabled, got '%s'", vh.ModeString())
	}
}

func TestVimHandler_SetCommand(t *testing.T) {
	vh := NewVimHandler(true)
	vp := viewport.New(80, 20)
	input := textinput.New()

	// Test :set vim
	vh.mode = VimModeCommand
	vh.commandBuffer = "set vim"

	enterKey := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := vh.HandleKey(enterKey, &vp, &input)
	if cmd == nil {
		t.Error("Expected command to be returned for ':set vim'")
	}

	// Execute the command to get the message
	msg := cmd()
	vimCmd, ok := msg.(VimCommandMsg)
	if !ok {
		t.Errorf("Expected VimCommandMsg, got %T", msg)
	}
	if vimCmd.Command != "set-vim" {
		t.Errorf("Expected command 'set-vim', got '%s'", vimCmd.Command)
	}
	if enabled, ok := vimCmd.Value.(bool); !ok || !enabled {
		t.Error("Expected vim mode to be enabled")
	}

	// Test :set novim
	vh.mode = VimModeCommand
	vh.commandBuffer = "set novim"

	_, cmd = vh.HandleKey(enterKey, &vp, &input)
	if cmd == nil {
		t.Error("Expected command to be returned for ':set novim'")
	}

	msg = cmd()
	vimCmd, ok = msg.(VimCommandMsg)
	if !ok {
		t.Errorf("Expected VimCommandMsg, got %T", msg)
	}
	if vimCmd.Command != "set-vim" {
		t.Errorf("Expected command 'set-vim', got '%s'", vimCmd.Command)
	}
	if enabled, ok := vimCmd.Value.(bool); !ok || enabled {
		t.Error("Expected vim mode to be disabled")
	}
}
