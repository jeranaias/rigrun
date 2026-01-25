// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// session_cmd.go - Session management CLI commands for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements AC-12 (Session Termination) for IL5 compliance.
// Provides session listing, viewing, export, and deletion capabilities.
//
// Command: session [subcommand]
// Short:   Manage saved chat sessions
// Aliases: sessions
//
// Subcommands:
//   list (default)      List all saved sessions (aliases: ls, l)
//   show <id>           Show session details
//   export <id>         Export session transcript
//   delete <id>         Delete a session
//   delete-all          Delete all sessions
//   stats               Show session statistics
//
// Examples:
//   rigrun session                          List all sessions (default)
//   rigrun session list                     List all sessions
//   rigrun session ls                       List sessions (short alias)
//   rigrun session show 1                   Show first session details
//   rigrun session show abc123              Show session by ID
//   rigrun session export 1 --format json   Export as JSON
//   rigrun session export 1 --format md     Export as Markdown
//   rigrun session export 1 --format txt    Export as plain text
//   rigrun session delete 1 --confirm       Delete first session
//   rigrun session delete-all --confirm     Delete all sessions
//   rigrun session stats                    Show statistics
//   rigrun session stats --json             Stats in JSON format
//
// Flags:
//   --format FORMAT     Export format: json, md, txt (default: txt)
//   --confirm           Required for delete operations
//   --json              Output in JSON format
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/storage"
	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// SESSION COMMAND HANDLER
// =============================================================================

// SessionArgs holds parsed session command arguments.
type SessionArgs struct {
	Subcommand string   // list, show, export, delete, delete-all, stats
	SessionID  string   // Session ID for show, export, delete
	Format     string   // Export format: json, md, txt
	Confirm    bool     // Confirmation flag for delete operations
	JSON       bool     // Output in JSON format
	Raw        []string // Raw remaining arguments
}

// HandleSession handles the "session" command with various subcommands.
// Subcommands:
//   - session list: List all saved sessions
//   - session show <id>: Show session details
//   - session export <id> [--format json|md|txt]: Export session transcript
//   - session delete <id> --confirm: Delete a session
//   - session delete-all --confirm: Delete all sessions
//   - session stats: Show session statistics
func HandleSession(args Args) error {
	// Parse session-specific arguments
	sessionArgs := parseSessionCmdArgs(args)

	switch sessionArgs.Subcommand {
	case "", "list":
		return handleSessionList(sessionArgs)
	case "show":
		return handleSessionShow(sessionArgs)
	case "export":
		return handleSessionExport(sessionArgs)
	case "delete":
		return handleSessionDelete(sessionArgs)
	case "delete-all":
		return handleSessionDeleteAll(sessionArgs)
	case "stats":
		return handleSessionStats(sessionArgs)
	default:
		return fmt.Errorf("unknown session subcommand: %s\nUsage: rigrun session [list|show|export|delete|delete-all|stats]", sessionArgs.Subcommand)
	}
}

// parseSessionCmdArgs parses detailed session command arguments from the Args struct.
func parseSessionCmdArgs(args Args) SessionArgs {
	sessionArgs := SessionArgs{
		Subcommand: args.Subcommand,
		Format:     "txt", // Default export format
		Raw:        args.Raw,
		JSON:       args.JSON, // Inherit global JSON flag
	}

	// Parse flags and positional arguments
	for i := 0; i < len(args.Raw); i++ {
		arg := args.Raw[i]

		switch arg {
		case "--format":
			if i+1 < len(args.Raw) {
				i++
				sessionArgs.Format = strings.ToLower(args.Raw[i])
			}
		case "--confirm":
			sessionArgs.Confirm = true
		case "--json":
			sessionArgs.JSON = true
		default:
			// Check for --format=value syntax
			if strings.HasPrefix(arg, "--format=") {
				sessionArgs.Format = strings.ToLower(strings.TrimPrefix(arg, "--format="))
			} else if !strings.HasPrefix(arg, "-") {
				// First non-flag argument after subcommand is the session ID
				if sessionArgs.SessionID == "" && arg != sessionArgs.Subcommand {
					sessionArgs.SessionID = arg
				}
			}
		}
	}

	return sessionArgs
}

// =============================================================================
// SESSION LIST
// =============================================================================

// SessionListOutput is the JSON output format for session list.
type SessionListOutput struct {
	Sessions []SessionInfo `json:"sessions"`
	Count    int           `json:"count"`
}

// SessionInfo is the JSON output format for a single session.
type SessionInfo struct {
	ID           string    `json:"id"`
	Summary      string    `json:"summary"`
	Model        string    `json:"model"`
	MessageCount int       `json:"message_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Preview      string    `json:"preview,omitempty"`
}

// handleSessionList lists all saved sessions.
func handleSessionList(args SessionArgs) error {
	store, err := storage.NewConversationStore()
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	sessions, err := store.List()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	// Audit log the session list action
	logSessionEvent("SESSION_LIST", "", map[string]string{
		"count": strconv.Itoa(len(sessions)),
	})

	if args.JSON {
		return outputSessionListJSON(sessions)
	}

	return outputSessionListText(sessions)
}

// outputSessionListJSON outputs sessions in JSON format.
func outputSessionListJSON(sessions []storage.ConversationMeta) error {
	output := SessionListOutput{
		Sessions: make([]SessionInfo, 0, len(sessions)),
		Count:    len(sessions),
	}

	for _, s := range sessions {
		output.Sessions = append(output.Sessions, SessionInfo{
			ID:           s.ID,
			Summary:      s.Summary,
			Model:        s.Model,
			MessageCount: s.MessageCount,
			CreatedAt:    s.CreatedAt,
			UpdatedAt:    s.UpdatedAt,
			Preview:      s.Preview,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// outputSessionListText outputs sessions in human-readable format.
func outputSessionListText(sessions []storage.ConversationMeta) error {
	if len(sessions) == 0 {
		fmt.Println()
		fmt.Println("No saved sessions found.")
		fmt.Println()
		fmt.Println("Sessions are saved when you use /save in the TUI.")
		fmt.Println()
		return nil
	}

	fmt.Println()
	fmt.Println("Saved Sessions")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// Table header
	fmt.Printf("%-4s %-20s %-20s %-6s %-12s\n", "ID", "Summary", "Model", "Msgs", "Updated")
	fmt.Println(strings.Repeat("-", 60))

	for i, s := range sessions {
		// UNICODE: Rune-aware truncation preserves multi-byte characters
		summary := util.TruncateRunes(s.Summary, 18)

		// UNICODE: Rune-aware truncation preserves multi-byte characters
		model := util.TruncateRunes(s.Model, 18)

		// Format update time
		updated := formatTimeAgo(s.UpdatedAt)
		if len(updated) > 10 {
			updated = s.UpdatedAt.Format("01/02")
		}

		fmt.Printf("%-4d %-20s %-20s %-6d %-12s\n",
			i+1,
			summary,
			model,
			s.MessageCount,
			updated,
		)
	}

	fmt.Println()
	fmt.Printf("Total: %d session(s)\n", len(sessions))
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  rigrun session show <id>              View session details")
	fmt.Println("  rigrun session export <id> --format   Export transcript (json|md|txt)")
	fmt.Println("  rigrun session delete <id> --confirm  Delete session")
	fmt.Println()

	return nil
}

// =============================================================================
// SESSION SHOW
// =============================================================================

// handleSessionShow shows details of a specific session.
func handleSessionShow(args SessionArgs) error {
	if args.SessionID == "" {
		return fmt.Errorf("session ID required\nUsage: rigrun session show <id>")
	}

	store, err := storage.NewConversationStore()
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	conv, err := loadSessionByIDOrIndex(store, args.SessionID)
	if err != nil {
		return err
	}

	// Audit log the session view action
	logSessionEvent("SESSION_VIEW", conv.ID, map[string]string{
		"message_count": strconv.Itoa(len(conv.Messages)),
	})

	if args.JSON {
		return outputSessionShowJSON(conv)
	}

	return outputSessionShowText(conv)
}

// outputSessionShowJSON outputs session details in JSON format.
func outputSessionShowJSON(conv *storage.StoredConversation) error {
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// outputSessionShowText outputs session details in human-readable format.
func outputSessionShowText(conv *storage.StoredConversation) error {
	fmt.Println()
	fmt.Printf("Session: %s\n", conv.Summary)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	fmt.Printf("ID:           %s\n", conv.ID)
	fmt.Printf("Model:        %s\n", conv.Model)
	fmt.Printf("Messages:     %d\n", len(conv.Messages))
	fmt.Printf("Created:      %s\n", conv.CreatedAt.Format(time.RFC1123))
	fmt.Printf("Updated:      %s\n", conv.UpdatedAt.Format(time.RFC1123))
	if conv.TokensUsed > 0 {
		fmt.Printf("Tokens Used:  %d\n", conv.TokensUsed)
	}
	fmt.Println()

	// Show message preview
	fmt.Println("Messages:")
	fmt.Println(strings.Repeat("-", 60))

	for i, msg := range conv.Messages {
		role := strings.ToUpper(msg.Role)
		content := msg.Content

		// Truncate long content for preview
		contentRunes := []rune(content)
		if len(contentRunes) > 100 {
			content = string(contentRunes[:97]) + "..."
		}
		// Replace newlines for single-line display
		content = strings.ReplaceAll(content, "\n", " ")

		fmt.Printf("[%d] %s: %s\n", i+1, role, content)
	}

	fmt.Println()
	fmt.Printf("Use 'rigrun session export %s' to export full transcript.\n", conv.ID)
	fmt.Println()

	return nil
}

// =============================================================================
// SESSION EXPORT
// =============================================================================

// handleSessionExport exports a session transcript.
func handleSessionExport(args SessionArgs) error {
	if args.SessionID == "" {
		return fmt.Errorf("session ID required\nUsage: rigrun session export <id> [--format json|md|txt]")
	}

	// Validate format
	validFormats := map[string]bool{"json": true, "md": true, "txt": true}
	if !validFormats[args.Format] {
		return fmt.Errorf("invalid format '%s', must be one of: json, md, txt", args.Format)
	}

	store, err := storage.NewConversationStore()
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	conv, err := loadSessionByIDOrIndex(store, args.SessionID)
	if err != nil {
		return err
	}

	// Audit log the session export action
	logSessionEvent("SESSION_EXPORT", conv.ID, map[string]string{
		"format":        args.Format,
		"message_count": strconv.Itoa(len(conv.Messages)),
	})

	switch args.Format {
	case "json":
		return exportSessionJSON(conv)
	case "md":
		return exportSessionMarkdown(conv)
	case "txt":
		return exportSessionText(conv)
	default:
		return exportSessionText(conv)
	}
}

// exportSessionJSON exports session as JSON.
func exportSessionJSON(conv *storage.StoredConversation) error {
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// exportSessionMarkdown exports session as Markdown.
func exportSessionMarkdown(conv *storage.StoredConversation) error {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# %s\n\n", conv.Summary))
	sb.WriteString(fmt.Sprintf("**Session ID:** %s  \n", conv.ID))
	sb.WriteString(fmt.Sprintf("**Model:** %s  \n", conv.Model))
	sb.WriteString(fmt.Sprintf("**Messages:** %d  \n", len(conv.Messages)))
	sb.WriteString(fmt.Sprintf("**Created:** %s  \n", conv.CreatedAt.Format(time.RFC1123)))
	sb.WriteString(fmt.Sprintf("**Updated:** %s  \n", conv.UpdatedAt.Format(time.RFC1123)))
	sb.WriteString("\n---\n\n")

	// Messages
	sb.WriteString("## Transcript\n\n")

	for _, msg := range conv.Messages {
		role := formatRole(msg.Role)
		sb.WriteString(fmt.Sprintf("### %s\n\n", role))

		// Handle tool messages specially
		if msg.Role == "tool" && msg.ToolName != "" {
			sb.WriteString(fmt.Sprintf("**Tool:** %s  \n", msg.ToolName))
			if msg.ToolInput != "" {
				sb.WriteString(fmt.Sprintf("**Input:** `%s`  \n", msg.ToolInput))
			}
			sb.WriteString(fmt.Sprintf("**Result:** %s  \n", statusText(msg.IsSuccess)))
			sb.WriteString("\n```\n")
			sb.WriteString(msg.ToolResult)
			sb.WriteString("\n```\n\n")
		} else {
			sb.WriteString(msg.Content)
			sb.WriteString("\n\n")
		}

		// Add statistics for assistant messages
		if msg.Role == "assistant" && msg.TokenCount > 0 {
			sb.WriteString(fmt.Sprintf("*%d tokens | %.1f tok/s | TTFT: %dms*\n\n",
				msg.TokenCount, msg.TokensPerSec, msg.TTFTMs))
		}
	}

	fmt.Print(sb.String())
	return nil
}

// exportSessionText exports session as plain text.
func exportSessionText(conv *storage.StoredConversation) error {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("Session: %s\n", conv.Summary))
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")
	sb.WriteString(fmt.Sprintf("ID:       %s\n", conv.ID))
	sb.WriteString(fmt.Sprintf("Model:    %s\n", conv.Model))
	sb.WriteString(fmt.Sprintf("Messages: %d\n", len(conv.Messages)))
	sb.WriteString(fmt.Sprintf("Created:  %s\n", conv.CreatedAt.Format(time.RFC1123)))
	sb.WriteString(fmt.Sprintf("Updated:  %s\n", conv.UpdatedAt.Format(time.RFC1123)))
	sb.WriteString("\n" + strings.Repeat("-", 60) + "\n\n")

	// Messages
	for i, msg := range conv.Messages {
		role := formatRole(msg.Role)
		sb.WriteString(fmt.Sprintf("[%d] %s:\n", i+1, role))

		// Handle tool messages specially
		if msg.Role == "tool" && msg.ToolName != "" {
			sb.WriteString(fmt.Sprintf("    Tool: %s (%s)\n", msg.ToolName, statusText(msg.IsSuccess)))
			if msg.ToolInput != "" {
				sb.WriteString(fmt.Sprintf("    Input: %s\n", msg.ToolInput))
			}
			sb.WriteString("    Result:\n")
			// Indent tool result
			for _, line := range strings.Split(msg.ToolResult, "\n") {
				sb.WriteString("      " + line + "\n")
			}
		} else {
			sb.WriteString(msg.Content)
		}
		sb.WriteString("\n\n")
	}

	fmt.Print(sb.String())
	return nil
}

// =============================================================================
// SESSION DELETE
// =============================================================================

// handleSessionDelete deletes a specific session.
func handleSessionDelete(args SessionArgs) error {
	if args.SessionID == "" {
		return fmt.Errorf("session ID required\nUsage: rigrun session delete <id> --confirm")
	}

	if !args.Confirm {
		return fmt.Errorf("deletion requires --confirm flag\nUsage: rigrun session delete %s --confirm", args.SessionID)
	}

	store, err := storage.NewConversationStore()
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	// Load session first to verify it exists and get metadata
	conv, err := loadSessionByIDOrIndex(store, args.SessionID)
	if err != nil {
		return err
	}

	// Audit log the session delete action (AU-12: Audit generation)
	logSessionEvent("SESSION_DELETE", conv.ID, map[string]string{
		"summary":       conv.Summary,
		"message_count": strconv.Itoa(len(conv.Messages)),
	})

	// Delete the session
	if err := store.Delete(conv.ID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	if args.JSON {
		output := map[string]interface{}{
			"deleted":    true,
			"session_id": conv.ID,
			"summary":    conv.Summary,
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("Session deleted: %s\n", conv.Summary)
	fmt.Printf("ID: %s\n", conv.ID)
	fmt.Println()

	return nil
}

// =============================================================================
// SESSION DELETE-ALL
// =============================================================================

// handleSessionDeleteAll deletes all sessions.
func handleSessionDeleteAll(args SessionArgs) error {
	if !args.Confirm {
		return fmt.Errorf("deletion requires --confirm flag\nUsage: rigrun session delete-all --confirm")
	}

	store, err := storage.NewConversationStore()
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	// Get count before deletion for audit
	sessions, err := store.List()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	count := len(sessions)
	if count == 0 {
		if args.JSON {
			output := map[string]interface{}{
				"deleted": 0,
				"message": "no sessions to delete",
			}
			data, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(data))
			return nil
		}
		fmt.Println()
		fmt.Println("No sessions to delete.")
		fmt.Println()
		return nil
	}

	// Audit log the session delete-all action (AU-12: Audit generation)
	logSessionEvent("SESSION_DELETE_ALL", "", map[string]string{
		"count": strconv.Itoa(count),
	})

	// Delete all sessions
	if err := store.Clear(); err != nil {
		return fmt.Errorf("failed to delete all sessions: %w", err)
	}

	if args.JSON {
		output := map[string]interface{}{
			"deleted": count,
			"message": "all sessions deleted",
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Printf("Deleted %d session(s).\n", count)
	fmt.Println()

	return nil
}

// =============================================================================
// SESSION STATS
// =============================================================================

// SessionStatsOutput is the JSON output format for session stats.
type SessionStatsOutput struct {
	TotalSessions   int                `json:"total_sessions"`
	TotalMessages   int                `json:"total_messages"`
	AverageLength   float64            `json:"average_messages_per_session"`
	TotalTokens     int                `json:"total_tokens"`
	ModelsUsed      map[string]int     `json:"models_used"`
	OldestSession   *time.Time         `json:"oldest_session,omitempty"`
	NewestSession   *time.Time         `json:"newest_session,omitempty"`
	StorageBytes    int64              `json:"storage_bytes"`
	StorageLocation string             `json:"storage_location"`
}

// handleSessionStats shows session statistics.
func handleSessionStats(args SessionArgs) error {
	store, err := storage.NewConversationStore()
	if err != nil {
		return fmt.Errorf("failed to initialize session storage: %w", err)
	}

	sessions, err := store.List()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	// Calculate statistics
	stats := calculateSessionStats(store, sessions)

	// Audit log the stats view action
	logSessionEvent("SESSION_STATS_VIEW", "", map[string]string{
		"session_count": strconv.Itoa(stats.TotalSessions),
	})

	if args.JSON {
		data, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	return outputSessionStatsText(stats)
}

// calculateSessionStats calculates statistics from sessions.
func calculateSessionStats(store *storage.ConversationStore, sessions []storage.ConversationMeta) SessionStatsOutput {
	stats := SessionStatsOutput{
		TotalSessions:   len(sessions),
		ModelsUsed:      make(map[string]int),
		StorageLocation: store.BaseDir,
	}

	if len(sessions) == 0 {
		return stats
	}

	// Track oldest and newest
	var oldest, newest time.Time

	for _, meta := range sessions {
		stats.TotalMessages += meta.MessageCount
		stats.ModelsUsed[meta.Model]++

		// Track time range
		if oldest.IsZero() || meta.CreatedAt.Before(oldest) {
			oldest = meta.CreatedAt
		}
		if newest.IsZero() || meta.UpdatedAt.After(newest) {
			newest = meta.UpdatedAt
		}

		// Load full conversation for token count
		conv, err := store.Load(meta.ID)
		if err == nil && conv != nil {
			stats.TotalTokens += conv.TokensUsed
		}
	}

	// Calculate average
	if stats.TotalSessions > 0 {
		stats.AverageLength = float64(stats.TotalMessages) / float64(stats.TotalSessions)
	}

	if !oldest.IsZero() {
		stats.OldestSession = &oldest
	}
	if !newest.IsZero() {
		stats.NewestSession = &newest
	}

	// Calculate storage size
	stats.StorageBytes = calculateStorageSize(store.BaseDir)

	return stats
}

// calculateStorageSize calculates total size of session files.
func calculateStorageSize(dir string) int64 {
	var size int64
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".json") {
			size += info.Size()
		}
		return nil
	})
	return size
}

// outputSessionStatsText outputs session stats in human-readable format.
func outputSessionStatsText(stats SessionStatsOutput) error {
	fmt.Println()
	fmt.Println("Session Statistics")
	fmt.Println(strings.Repeat("=", 40))
	fmt.Println()

	fmt.Printf("Total Sessions:    %d\n", stats.TotalSessions)
	fmt.Printf("Total Messages:    %d\n", stats.TotalMessages)
	fmt.Printf("Average Length:    %.1f messages/session\n", stats.AverageLength)

	if stats.TotalTokens > 0 {
		fmt.Printf("Total Tokens:      %d\n", stats.TotalTokens)
	}

	fmt.Printf("Storage Used:      %s\n", formatBytes(stats.StorageBytes))
	fmt.Println()

	// Time range
	if stats.OldestSession != nil {
		fmt.Printf("First Session:     %s\n", stats.OldestSession.Format("2006-01-02 15:04"))
	}
	if stats.NewestSession != nil {
		fmt.Printf("Latest Activity:   %s\n", stats.NewestSession.Format("2006-01-02 15:04"))
	}
	fmt.Println()

	// Models used
	if len(stats.ModelsUsed) > 0 {
		fmt.Println("Models Used:")

		// Sort models by usage count
		type modelCount struct {
			model string
			count int
		}
		var models []modelCount
		for m, c := range stats.ModelsUsed {
			models = append(models, modelCount{m, c})
		}
		sort.Slice(models, func(i, j int) bool {
			return models[i].count > models[j].count
		})

		for _, mc := range models {
			fmt.Printf("  %-30s %d session(s)\n", mc.model, mc.count)
		}
		fmt.Println()
	}

	fmt.Printf("Storage Location:  %s\n", stats.StorageLocation)
	fmt.Println()

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// loadSessionByIDOrIndex loads a session by ID or numeric index.
func loadSessionByIDOrIndex(store *storage.ConversationStore, idOrIndex string) (*storage.StoredConversation, error) {
	// Try parsing as index first
	if idx, err := strconv.Atoi(idOrIndex); err == nil {
		// Load by index (1-based for user friendliness)
		conv, err := store.LoadByIndex(idx - 1)
		if err != nil {
			return nil, fmt.Errorf("session #%d not found", idx)
		}
		return conv, nil
	}

	// Load by ID
	conv, err := store.Load(idOrIndex)
	if err != nil {
		return nil, fmt.Errorf("session '%s' not found", idOrIndex)
	}
	return conv, nil
}

// formatRole formats a message role for display.
func formatRole(role string) string {
	switch role {
	case "user":
		return "User"
	case "assistant":
		return "Assistant"
	case "system":
		return "System"
	case "tool":
		return "Tool"
	default:
		return strings.Title(role)
	}
}

// statusText returns success/failure text.
func statusText(success bool) string {
	if success {
		return "Success"
	}
	return "Failed"
}

// ERROR HANDLING: Errors must not be silently ignored

// logSessionEvent logs a session-related event for IL5 audit compliance (AU-12).
func logSessionEvent(eventType, sessionID string, metadata map[string]string) {
	// Use the global audit logger for IL5 compliance (AU-12: Audit generation)
	logger := security.GlobalAuditLogger()
	if logger == nil || !logger.IsEnabled() {
		return
	}

	event := security.AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		SessionID: sessionID,
		Success:   true,
		Metadata:  metadata,
	}

	if err := logger.Log(event); err != nil {
		// Log to stderr when audit logging fails - per AU-5 requirements
		fmt.Fprintf(os.Stderr, "AUDIT ERROR: failed to log session event %s: %v\n", eventType, err)
	}
}

// Note: promptInput is defined in setup.go and used across CLI commands
