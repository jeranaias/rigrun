// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package components provides UI components for the rigrun TUI.
package components

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	chromaStyles "github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/ui/styles"
)

// =============================================================================
// CODE BLOCK RENDERER
// =============================================================================

// CodeBlock represents a rendered code block.
type CodeBlock struct {
	Language string
	Code     string
	MaxWidth int
}

// NewCodeBlock creates a new code block.
func NewCodeBlock(language, code string) CodeBlock {
	return CodeBlock{
		Language: language,
		Code:     code,
		MaxWidth: 80,
	}
}

// SetMaxWidth sets the maximum width for the code block.
func (c *CodeBlock) SetMaxWidth(width int) {
	c.MaxWidth = width
}

// Render renders the code block with styling.
// USABILITY: Syntax highlighting for better code readability
func (c CodeBlock) Render() string {
	// Clean the code
	code := strings.TrimSpace(c.Code)

	// Apply syntax highlighting if language is specified or can be detected
	language := c.Language
	if language == "" {
		language = detectLanguage(code)
	}

	// Get highlighted code (returns original if highlighting fails)
	highlightedCode := highlightCode(code, language)
	lines := strings.Split(highlightedCode, "\n")

	// Build the rendered lines with line numbers
	var renderedLines []string

	lineNumStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Width(4).
		Align(lipgloss.Right).
		MarginRight(1)

	for i, line := range lines {
		lineNum := lineNumStyle.Render(formatCodeInt(i + 1))
		// Line already has syntax highlighting from chroma, don't apply additional styling
		renderedLines = append(renderedLines, lineNum+line)
	}

	codeContent := strings.Join(renderedLines, "\n")

	// Create the header with language badge
	var header string
	if c.Language != "" {
		langBadge := lipgloss.NewStyle().
			Foreground(styles.TextMuted).
			Background(styles.OverlayDim).
			Padding(0, 1).
			Bold(true).
			Render(c.Language)
		header = langBadge + "\n"
	}

	// Create the code block container
	maxWidth := c.MaxWidth - 4
	if maxWidth < 20 {
		maxWidth = 20
	}

	block := lipgloss.NewStyle().
		Background(styles.SurfaceDim).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Overlay).
		Padding(1, 2).
		MaxWidth(maxWidth).
		Render(header + codeContent)

	return block
}

// =============================================================================
// MARKDOWN CODE BLOCK PARSER
// =============================================================================

// ParseCodeBlocks extracts code blocks from markdown text.
// Returns the text with code blocks replaced by rendered versions.
func ParseCodeBlocks(text string, maxWidth int) string {
	lines := strings.Split(text, "\n")
	var result []string
	var inCodeBlock bool
	var codeLines []string
	var language string

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block
				code := strings.Join(codeLines, "\n")
				cb := NewCodeBlock(language, code)
				cb.SetMaxWidth(maxWidth)
				result = append(result, cb.Render())
				codeLines = nil
				language = ""
				inCodeBlock = false
			} else {
				// Start of code block
				language = strings.TrimPrefix(line, "```")
				language = strings.TrimSpace(language)
				inCodeBlock = true
			}
		} else if inCodeBlock {
			codeLines = append(codeLines, line)
		} else {
			result = append(result, line)
		}
	}

	// Handle unclosed code block
	if inCodeBlock && len(codeLines) > 0 {
		code := strings.Join(codeLines, "\n")
		cb := NewCodeBlock(language, code)
		cb.SetMaxWidth(maxWidth)
		result = append(result, cb.Render())
	}

	return strings.Join(result, "\n")
}

// =============================================================================
// INLINE CODE RENDERER
// =============================================================================

// RenderInlineCode renders inline code with a subtle background.
func RenderInlineCode(code string) string {
	return lipgloss.NewStyle().
		Background(styles.SurfaceDim).
		Foreground(styles.Cyan).
		Padding(0, 1).
		Render(code)
}

// ParseInlineCode replaces `code` with styled inline code.
func ParseInlineCode(text string) string {
	var result strings.Builder
	var inCode bool
	var codeBuffer strings.Builder

	for _, r := range text {
		if r == '`' {
			if inCode {
				// End inline code
				result.WriteString(RenderInlineCode(codeBuffer.String()))
				codeBuffer.Reset()
				inCode = false
			} else {
				// Start inline code
				inCode = true
			}
		} else if inCode {
			codeBuffer.WriteRune(r)
		} else {
			result.WriteRune(r)
		}
	}

	// Handle unclosed inline code
	if inCode {
		result.WriteString("`")
		result.WriteString(codeBuffer.String())
	}

	return result.String()
}

// =============================================================================
// SYNTAX HIGHLIGHTING (Chroma-based)
// =============================================================================

// USABILITY: Syntax highlighting for better code readability

// highlightCode applies syntax highlighting to code using the chroma library.
// This provides proper ANSI-safe syntax highlighting for terminal output.
func highlightCode(code, language string) string {
	// Get lexer for language
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// Get style (use terminal-friendly style)
	style := chromaStyles.Get("monokai")
	if style == nil {
		style = chromaStyles.Fallback
	}

	// Get terminal formatter
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	// Tokenize and format
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code // Fallback to plain text
	}

	var buf strings.Builder
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return code
	}

	return buf.String()
}

// detectLanguage attempts to detect the programming language of the given code.
func detectLanguage(code string) string {
	lexer := lexers.Analyse(code)
	if lexer != nil {
		return lexer.Config().Name
	}
	return ""
}

// HighlightGo applies Go syntax highlighting using chroma.
func HighlightGo(code string) string {
	return highlightCode(code, "go")
}

// HighlightPython applies Python syntax highlighting using chroma.
func HighlightPython(code string) string {
	return highlightCode(code, "python")
}

// HighlightJavaScript applies JavaScript syntax highlighting using chroma.
func HighlightJavaScript(code string) string {
	return highlightCode(code, "javascript")
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// formatCodeInt converts int to string without fmt.
func formatCodeInt(n int) string {
	if n == 0 {
		return "0"
	}
	if n == -9223372036854775808 { // math.MinInt64
		return "-9223372036854775808"
	}
	if n < 0 {
		return "-" + formatCodeInt(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
