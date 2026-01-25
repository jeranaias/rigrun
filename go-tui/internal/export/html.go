// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package export

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/storage"
)

// =============================================================================
// HTML EXPORTER
// =============================================================================

// HTMLExporter exports conversations to HTML format with embedded CSS.
type HTMLExporter struct {
	options *Options
}

// NewHTMLExporter creates a new HTML exporter.
func NewHTMLExporter(opts *Options) *HTMLExporter {
	if opts == nil {
		opts = DefaultOptions()
	}
	return &HTMLExporter{options: opts}
}

// Export converts a conversation to HTML format.
func (e *HTMLExporter) Export(conv *storage.StoredConversation) ([]byte, error) {
	// Validate conversation data
	if conv == nil {
		return nil, fmt.Errorf("conversation is nil")
	}
	if len(conv.Messages) == 0 {
		return nil, fmt.Errorf("conversation has no messages")
	}
	if conv.CreatedAt.IsZero() {
		return nil, fmt.Errorf("conversation has invalid creation timestamp")
	}

	var sb strings.Builder

	// HTML header
	sb.WriteString("<!DOCTYPE html>\n")
	sb.WriteString("<html lang=\"en\">\n")
	sb.WriteString("<head>\n")
	sb.WriteString("    <meta charset=\"UTF-8\">\n")
	sb.WriteString("    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	sb.WriteString(fmt.Sprintf("    <title>%s</title>\n", html.EscapeString(conv.Summary)))
	sb.WriteString("    <meta name=\"generator\" content=\"rigrun-tui\">\n")
	sb.WriteString(fmt.Sprintf("    <meta name=\"date\" content=\"%s\">\n", conv.CreatedAt.Format(time.RFC3339)))

	// Embedded CSS
	sb.WriteString(e.getCSS())

	sb.WriteString("</head>\n")
	sb.WriteString(fmt.Sprintf("<body class=\"%s-theme\">\n", e.options.Theme))

	// Container
	sb.WriteString("    <div class=\"container\">\n")

	// Header with metadata
	if e.options.IncludeMetadata {
		sb.WriteString(e.renderHeader(conv))
	}

	// Conversation messages
	sb.WriteString("        <main class=\"conversation\">\n")
	for _, msg := range conv.Messages {
		sb.WriteString(e.renderMessage(&msg))
	}
	sb.WriteString("        </main>\n")

	// Footer
	sb.WriteString("        <footer class=\"footer\">\n")
	sb.WriteString(fmt.Sprintf("            <p>Exported from <strong>rigrun TUI</strong> on %s</p>\n",
		time.Now().Format("January 2, 2006 at 3:04 PM")))
	sb.WriteString("        </footer>\n")

	sb.WriteString("    </div>\n")

	// Theme toggle script
	sb.WriteString(e.getScript())

	sb.WriteString("</body>\n")
	sb.WriteString("</html>\n")

	return []byte(sb.String()), nil
}

// FileExtension returns the file extension for HTML.
func (e *HTMLExporter) FileExtension() string {
	return ".html"
}

// MimeType returns the MIME type for HTML.
func (e *HTMLExporter) MimeType() string {
	return "text/html"
}

// =============================================================================
// RENDERING FUNCTIONS
// =============================================================================

// renderHeader renders the header section with metadata.
func (e *HTMLExporter) renderHeader(conv *storage.StoredConversation) string {
	var sb strings.Builder

	sb.WriteString("        <header class=\"header\">\n")
	sb.WriteString(fmt.Sprintf("            <h1>%s</h1>\n", html.EscapeString(conv.Summary)))
	sb.WriteString("            <div class=\"metadata\">\n")
	sb.WriteString(fmt.Sprintf("                <span class=\"meta-item\"><strong>Model:</strong> %s</span>\n", html.EscapeString(conv.Model)))
	sb.WriteString(fmt.Sprintf("                <span class=\"meta-item\"><strong>Created:</strong> %s</span>\n", formatTimestamp(conv.CreatedAt)))
	sb.WriteString(fmt.Sprintf("                <span class=\"meta-item\"><strong>Messages:</strong> %d</span>\n", len(conv.Messages)))
	if conv.TokensUsed > 0 {
		sb.WriteString(fmt.Sprintf("                <span class=\"meta-item\"><strong>Tokens:</strong> %d</span>\n", conv.TokensUsed))
	}
	sb.WriteString("                <button class=\"theme-toggle\" onclick=\"toggleTheme()\" title=\"Toggle theme\">[Theme]</button>\n")
	sb.WriteString("            </div>\n")
	sb.WriteString("        </header>\n")

	return sb.String()
}

// renderMessage renders a single message.
func (e *HTMLExporter) renderMessage(msg *storage.StoredMessage) string {
	var sb strings.Builder

	roleClass := strings.ToLower(msg.Role)
	sb.WriteString(fmt.Sprintf("            <div class=\"message %s-message\">\n", roleClass))

	// Message header
	sb.WriteString("                <div class=\"message-header\">\n")
	sb.WriteString(fmt.Sprintf("                    <span class=\"role-label\">%s</span>\n", e.getRoleLabel(msg.Role)))
	if e.options.IncludeTimestamps {
		sb.WriteString(fmt.Sprintf("                    <span class=\"timestamp\">%s</span>\n", formatShortTimestamp(msg.Timestamp)))
	}
	sb.WriteString("                </div>\n")

	// Message content
	sb.WriteString("                <div class=\"message-content\">\n")

	content := msg.Content
	if content == "" && msg.Role == "tool" {
		content = e.formatToolMessage(msg)
	} else {
		content = e.formatContent(content)
	}

	sb.WriteString(content)
	sb.WriteString("                </div>\n")

	// Statistics for assistant messages
	if msg.Role == "assistant" && e.options.IncludeMetadata {
		stats := e.renderMessageStats(msg)
		if stats != "" {
			sb.WriteString(stats)
		}
	}

	sb.WriteString("            </div>\n")

	return sb.String()
}

// renderMessageStats renders statistics for a message.
func (e *HTMLExporter) renderMessageStats(msg *storage.StoredMessage) string {
	if msg.TokenCount == 0 && msg.DurationMs == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("                <div class=\"message-stats\">\n")

	if msg.TokenCount > 0 {
		sb.WriteString(fmt.Sprintf("                    <span class=\"stat\">Tokens: %d</span>\n", msg.TokenCount))
	}
	if msg.DurationMs > 0 {
		sb.WriteString(fmt.Sprintf("                    <span class=\"stat\">Time: %s</span>\n", formatDuration(msg.DurationMs)))
	}
	if msg.TTFTMs > 0 {
		sb.WriteString(fmt.Sprintf("                    <span class=\"stat\">TTFT: %s</span>\n", formatDuration(msg.TTFTMs)))
	}
	if msg.TokensPerSec > 0 {
		sb.WriteString(fmt.Sprintf("                    <span class=\"stat\">Speed: %s</span>\n", formatTokensPerSec(msg.TokensPerSec)))
	}

	sb.WriteString("                </div>\n")
	return sb.String()
}

// =============================================================================
// CONTENT FORMATTING
// =============================================================================

// formatContent formats message content with code block syntax highlighting.
func (e *HTMLExporter) formatContent(content string) string {
	// Convert markdown-style code blocks to HTML
	content = html.EscapeString(content)

	// Handle code blocks with language specification
	codeBlockRegex := regexp.MustCompile("```([a-zA-Z0-9_+-]*)\n([\\s\\S]*?)```")
	content = codeBlockRegex.ReplaceAllStringFunc(content, func(match string) string {
		parts := codeBlockRegex.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		lang := parts[1]
		code := parts[2]

		langLabel := ""
		if lang != "" {
			// SECURITY FIX: HTML-escape the language name to prevent XSS
			langLabel = fmt.Sprintf("<div class=\"code-lang\">%s</div>", html.EscapeString(lang))
		}

		return fmt.Sprintf("<div class=\"code-block\">%s<pre><code class=\"language-%s\">%s</code></pre></div>",
			langLabel, html.EscapeString(lang), strings.TrimSpace(code))
	})

	// Handle inline code
	inlineCodeRegex := regexp.MustCompile("`([^`]+)`")
	content = inlineCodeRegex.ReplaceAllString(content, "<code class=\"inline-code\">$1</code>")

	// Convert newlines to <br> for paragraphs
	// But preserve code block formatting
	lines := strings.Split(content, "\n")
	var formatted []string
	inParagraph := false

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Check if line is part of a code block or HTML tag
		if strings.Contains(line, "<div class=\"code-block\">") ||
		   strings.Contains(line, "</div>") ||
		   strings.Contains(line, "<pre>") ||
		   strings.Contains(line, "</pre>") {
			formatted = append(formatted, lines[i]) // Use original line with indentation
			inParagraph = false
			continue
		}

		if line == "" {
			if inParagraph {
				formatted = append(formatted, "</p>")
				inParagraph = false
			}
			formatted = append(formatted, "")
		} else {
			if !inParagraph && !strings.HasPrefix(line, "<") {
				formatted = append(formatted, "<p>"+line)
				inParagraph = true
			} else {
				formatted = append(formatted, line)
			}
		}
	}

	if inParagraph {
		formatted = append(formatted, "</p>")
	}

	return strings.Join(formatted, "\n")
}

// formatToolMessage formats a tool message.
func (e *HTMLExporter) formatToolMessage(msg *storage.StoredMessage) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("<p><strong>Tool:</strong> <code>%s</code></p>\n", html.EscapeString(msg.ToolName)))

	if msg.ToolInput != "" {
		sb.WriteString("<p><strong>Input:</strong></p>\n")
		sb.WriteString(fmt.Sprintf("<pre><code>%s</code></pre>\n", html.EscapeString(msg.ToolInput)))
	}

	if msg.ToolResult != "" {
		status := "[OK] Success"
		statusClass := "success"
		if !msg.IsSuccess {
			status = "[FAIL] Error"
			statusClass = "error"
		}
		sb.WriteString(fmt.Sprintf("<p><strong>Result</strong> <span class=\"%s\">%s</span>:</p>\n", statusClass, status))
		sb.WriteString(fmt.Sprintf("<pre><code>%s</code></pre>\n", html.EscapeString(msg.ToolResult)))
	}

	return sb.String()
}

// getRoleLabel returns a formatted label for the message role.
func (e *HTMLExporter) getRoleLabel(role string) string {
	// Check for empty role
	if role == "" {
		return "Unknown"
	}

	switch role {
	case "user":
		return "[User]"
	case "assistant":
		return "[Assistant]"
	case "system":
		return "[System]"
	case "tool":
		return "[Tool]"
	default:
		// Replace deprecated strings.Title with proper title casing
		if len(role) > 0 {
			runes := []rune(role)
			return strings.ToUpper(string(runes[0])) + strings.ToLower(string(runes[1:]))
		}
		return role
	}
}

// =============================================================================
// EMBEDDED CSS
// =============================================================================

// getCSS returns the embedded CSS for the HTML export.
func (e *HTMLExporter) getCSS() string {
	return `    <style>
        /* Reset and base styles */
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        :root {
            --font-sans: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            --font-mono: "SF Mono", "Monaco", "Inconsolata", "Fira Code", "Dank Mono", "Source Code Pro", monospace;
        }

        /* Dark theme (default) */
        .dark-theme {
            --bg-primary: #1a1b26;
            --bg-secondary: #24283b;
            --bg-tertiary: #414868;
            --text-primary: #c0caf5;
            --text-secondary: #a9b1d6;
            --text-muted: #565f89;
            --border-color: #414868;
            --user-bg: #1f2335;
            --assistant-bg: #24283b;
            --code-bg: #1a1b26;
            --accent-blue: #7aa2f7;
            --accent-green: #9ece6a;
            --accent-purple: #bb9af7;
            --accent-red: #f7768e;
        }

        /* Light theme */
        .light-theme {
            --bg-primary: #ffffff;
            --bg-secondary: #f7f8fa;
            --bg-tertiary: #e1e4e8;
            --text-primary: #24292e;
            --text-secondary: #586069;
            --text-muted: #6a737d;
            --border-color: #e1e4e8;
            --user-bg: #f6f8fa;
            --assistant-bg: #ffffff;
            --code-bg: #f6f8fa;
            --accent-blue: #0366d6;
            --accent-green: #22863a;
            --accent-purple: #6f42c1;
            --accent-red: #d73a49;
        }

        body {
            font-family: var(--font-sans);
            font-size: 16px;
            line-height: 1.6;
            color: var(--text-primary);
            background: var(--bg-primary);
            padding: 20px;
            transition: background 0.3s ease, color 0.3s ease;
        }

        .container {
            max-width: 900px;
            margin: 0 auto;
            background: var(--bg-secondary);
            border-radius: 12px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
            overflow: hidden;
        }

        /* Header */
        .header {
            padding: 32px;
            background: var(--bg-tertiary);
            border-bottom: 2px solid var(--border-color);
        }

        .header h1 {
            font-size: 28px;
            font-weight: 700;
            margin-bottom: 16px;
            color: var(--text-primary);
        }

        .metadata {
            display: flex;
            flex-wrap: wrap;
            gap: 16px;
            font-size: 14px;
            color: var(--text-secondary);
            align-items: center;
        }

        .meta-item {
            display: inline-flex;
            align-items: center;
            gap: 4px;
        }

        .theme-toggle {
            margin-left: auto;
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            padding: 6px 12px;
            cursor: pointer;
            font-size: 18px;
            transition: all 0.2s ease;
        }

        .theme-toggle:hover {
            background: var(--bg-primary);
            transform: scale(1.05);
        }

        /* Conversation */
        .conversation {
            padding: 24px 32px;
        }

        .message {
            margin-bottom: 24px;
            padding: 20px;
            border-radius: 8px;
            border-left: 4px solid transparent;
            transition: all 0.2s ease;
        }

        .message:hover {
            transform: translateX(4px);
        }

        .user-message {
            background: var(--user-bg);
            border-left-color: var(--accent-blue);
        }

        .assistant-message {
            background: var(--assistant-bg);
            border-left-color: var(--accent-green);
        }

        .system-message {
            background: var(--bg-tertiary);
            border-left-color: var(--accent-purple);
        }

        .tool-message {
            background: var(--bg-tertiary);
            border-left-color: var(--accent-purple);
        }

        .message-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 12px;
            font-size: 14px;
        }

        .role-label {
            font-weight: 600;
            color: var(--text-primary);
        }

        .timestamp {
            color: var(--text-muted);
            font-size: 13px;
            font-family: var(--font-mono);
        }

        .message-content {
            color: var(--text-primary);
            line-height: 1.7;
        }

        .message-content p {
            margin-bottom: 12px;
        }

        .message-content p:last-child {
            margin-bottom: 0;
        }

        /* Code blocks */
        .code-block {
            margin: 16px 0;
            border-radius: 8px;
            overflow: hidden;
            background: var(--code-bg);
            border: 1px solid var(--border-color);
        }

        .code-lang {
            padding: 8px 16px;
            background: var(--bg-tertiary);
            font-size: 12px;
            font-weight: 600;
            color: var(--text-secondary);
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .code-block pre {
            margin: 0;
            padding: 16px;
            overflow-x: auto;
        }

        .code-block code {
            font-family: var(--font-mono);
            font-size: 14px;
            line-height: 1.5;
            color: var(--text-primary);
        }

        .inline-code {
            font-family: var(--font-mono);
            font-size: 14px;
            padding: 2px 6px;
            background: var(--code-bg);
            border: 1px solid var(--border-color);
            border-radius: 4px;
            color: var(--accent-purple);
        }

        /* Message stats */
        .message-stats {
            margin-top: 12px;
            padding-top: 12px;
            border-top: 1px solid var(--border-color);
            display: flex;
            flex-wrap: wrap;
            gap: 16px;
            font-size: 13px;
            color: var(--text-muted);
        }

        .stat {
            display: inline-flex;
            align-items: center;
            gap: 4px;
        }

        /* Footer */
        .footer {
            padding: 20px 32px;
            text-align: center;
            font-size: 14px;
            color: var(--text-muted);
            border-top: 1px solid var(--border-color);
        }

        /* Status indicators */
        .success {
            color: var(--accent-green);
        }

        .error {
            color: var(--accent-red);
        }

        /* Print styles */
        @media print {
            body {
                padding: 0;
            }

            .container {
                box-shadow: none;
                border-radius: 0;
            }

            .theme-toggle {
                display: none;
            }

            .message {
                page-break-inside: avoid;
            }
        }

        /* Responsive */
        @media (max-width: 768px) {
            body {
                padding: 10px;
            }

            .header, .conversation, .footer {
                padding: 16px;
            }

            .message {
                padding: 16px;
            }
        }
    </style>
`
}

// =============================================================================
// EMBEDDED JAVASCRIPT
// =============================================================================

// getScript returns the embedded JavaScript for theme toggling.
func (e *HTMLExporter) getScript() string {
	return `    <script>
        function toggleTheme() {
            const body = document.body;
            if (body.classList.contains('dark-theme')) {
                body.classList.remove('dark-theme');
                body.classList.add('light-theme');
                localStorage.setItem('theme', 'light');
            } else {
                body.classList.remove('light-theme');
                body.classList.add('dark-theme');
                localStorage.setItem('theme', 'dark');
            }
        }

        // Load saved theme preference
        document.addEventListener('DOMContentLoaded', function() {
            const savedTheme = localStorage.getItem('theme');
            if (savedTheme) {
                document.body.classList.remove('dark-theme', 'light-theme');
                document.body.classList.add(savedTheme + '-theme');
            }
        });
    </script>
`
}
