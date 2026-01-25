// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// cache_cmd.go - Cache management CLI commands for rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Command: cache [subcommand]
// Short:   Manage response cache for cost savings
// Aliases: (none)
//
// Subcommands:
//   stats (default)     Show cache statistics
//   clear               Clear cache entries
//   export [dir]        Export cache to directory
//
// Cache Types:
//   exact               Exact-match cache (hash-based)
//   semantic            Semantic similarity cache (embedding-based)
//
// Examples:
//   rigrun cache                          Show stats (default)
//   rigrun cache stats                    Show cache statistics
//   rigrun cache stats --json             Stats in JSON format (SIEM)
//   rigrun cache clear                    Clear all cache (interactive)
//   rigrun cache clear --exact            Clear exact-match only
//   rigrun cache clear --semantic         Clear semantic cache only
//   rigrun cache export                   Export to current directory
//   rigrun cache export ./backup/         Export to specific directory
//
// Flags:
//   --exact             Target exact-match cache only
//   --semantic          Target semantic cache only
//   --json              Output in JSON format
//
// Cache Location:
//   ~/.rigrun/cache.json           Exact-match cache
//   ~/.rigrun/semantic_cache.json  Semantic similarity cache
//
// Statistics Explained:
//   Entries     Number of cached responses
//   Hit Rate    Percentage of queries served from cache
//   Hits        Total cache hits
//   Misses      Total cache misses
//   Saved       Estimated cost savings from cache hits
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/cache"
)

// =============================================================================
// CACHE COMMAND HANDLER
// =============================================================================

// HandleCache handles the "cache" command with various subcommands.
// Subcommands:
//   - cache or cache stats: Show cache statistics
//   - cache clear: Clear all cache
//   - cache clear --exact: Clear only exact-match cache
//   - cache clear --semantic: Clear only semantic cache
//   - cache export <dir>: Export cache to directory
// Supports JSON output for IL5 SIEM integration (AU-6, SI-4).
func HandleCache(args Args) error {
	// Parse additional flags from raw args
	clearExact := false
	clearSemantic := false
	exportDir := ""

	for i, arg := range args.Raw {
		switch arg {
		case "--exact":
			clearExact = true
		case "--semantic":
			clearSemantic = true
		default:
			// If subcommand is "export" and this is a path
			if args.Subcommand == "export" && !strings.HasPrefix(arg, "-") {
				exportDir = arg
			}
			// Check for export with path argument
			if i > 0 && args.Raw[i-1] == "export" {
				exportDir = arg
			}
		}
	}

	switch args.Subcommand {
	case "", "stats":
		if args.JSON {
			return showCacheStatsJSON()
		}
		return showCacheStats()
	case "clear":
		// USABILITY: Options struct improves API clarity
		return clearCacheWithOpts(CacheClearOptions{
			ExactOnly:    clearExact,
			SemanticOnly: clearSemantic,
		})
	case "export":
		return exportCache(exportDir)
	default:
		return fmt.Errorf("unknown cache subcommand: %s", args.Subcommand)
	}
}

// =============================================================================
// CACHE STATISTICS
// =============================================================================

// CacheInfo holds cache file information for display.
type CacheInfo struct {
	Path       string
	Entries    int
	SizeBytes  int64
	OldestTime time.Time
}

// showCacheStatsJSON outputs cache statistics in JSON format for SIEM integration.
func showCacheStatsJSON() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		resp := NewJSONErrorResponse("cache stats", err)
		resp.Print()
		return err
	}

	cachePath := filepath.Join(homeDir, ".rigrun", "cache.json")
	semanticPath := filepath.Join(homeDir, ".rigrun", "semantic_cache.json")

	// Gather exact cache stats
	exactStats := CacheTypeStats{}
	exactCache, err := cache.LoadOrCreateDefault()
	if err == nil && exactCache != nil {
		stats := exactCache.Stats()
		exactStats.Entries = stats.Size
		exactStats.HitRate = stats.HitRate
		exactStats.Hits = stats.Hits
		exactStats.Misses = stats.Misses
	}

	// Get file size
	if fileInfo, err := os.Stat(cachePath); err == nil {
		exactStats.SizeBytes = fileInfo.Size()
	}

	// Gather semantic cache stats
	semanticStats := CacheTypeStats{}
	if _, err := os.Stat(semanticPath); err == nil {
		semanticStats.Entries = countSemanticEntries(semanticPath)
	}

	// Calculate savings
	cacheHits := 0
	if exactCache != nil {
		cacheHits = exactCache.Stats().Hits
	}
	savedDollars := float64(cacheHits) * 0.02

	data := CacheStatsData{
		ExactCache:    exactStats,
		SemanticCache: semanticStats,
		Savings: CacheSavings{
			CacheHits:    cacheHits,
			SavedDollars: savedDollars,
		},
		Location: cachePath,
	}

	resp := NewJSONResponse("cache stats", data)
	return resp.Print()
}

// showCacheStats displays cache statistics.
func showCacheStats() error {
	fmt.Println()
	fmt.Println("rigrun Cache Statistics")
	fmt.Println(strings.Repeat("=", 39))
	fmt.Println()

	// Get cache paths
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	cachePath := filepath.Join(homeDir, ".rigrun", "cache.json")

	// Try to load existing cache
	exactCache, err := cache.LoadOrCreateDefault()
	if err != nil && !os.IsNotExist(err) {
		fmt.Printf("  Warning: Could not load cache: %v\n", err)
	}

	// Get cache file info
	fileInfo, statErr := os.Stat(cachePath)

	// Display Exact-Match Cache stats
	fmt.Println("Exact-Match Cache")
	if exactCache != nil {
		stats := exactCache.Stats()
		fmt.Printf("  Entries:    %d\n", stats.Size)

		// File size
		if statErr == nil {
			fmt.Printf("  Size:       %s\n", formatBytes(fileInfo.Size()))
		} else {
			fmt.Printf("  Size:       N/A\n")
		}

		// Hit rate
		fmt.Printf("  Hit Rate:   %.0f%%\n", stats.HitRate)

		// Find oldest entry
		oldestTime := findOldestEntryTime(exactCache)
		if !oldestTime.IsZero() {
			fmt.Printf("  Oldest:     %s\n", formatTimeAgo(oldestTime))
		}
	} else {
		fmt.Println("  No cache data found")
	}
	fmt.Println()

	// Display Semantic Cache stats (if available)
	fmt.Println("Semantic Cache")
	semanticPath := filepath.Join(homeDir, ".rigrun", "semantic_cache.json")
	_, semanticErr := os.Stat(semanticPath)
	if semanticErr == nil {
		entries := countSemanticEntries(semanticPath)
		fmt.Printf("  Entries:    %d\n", entries)
		fmt.Printf("  Embeddings: %d\n", entries)
		fmt.Printf("  Threshold:  0.92\n")
	} else {
		fmt.Println("  Not configured (no embedding model)")
	}
	fmt.Println()

	// Display total savings
	fmt.Println("Total Savings")
	if exactCache != nil {
		stats := exactCache.Stats()
		cacheHits := stats.Hits
		// Estimate savings: ~$0.02 per cloud query saved
		savedDollars := float64(cacheHits) * 0.02
		fmt.Printf("  Cache Hits: %d\n", cacheHits)
		fmt.Printf("  Saved:      $%.2f\n", savedDollars)
	} else {
		fmt.Println("  Cache Hits: 0")
		fmt.Println("  Saved:      $0.00")
	}
	fmt.Println()

	// Show cache location
	fmt.Printf("Cache Location: %s\n", cachePath)
	fmt.Println()

	return nil
}

// =============================================================================
// CACHE CLEAR
// =============================================================================

// USABILITY: Options struct improves API clarity
// CacheClearOptions provides a clear, self-documenting API for cache clear operations.
// This replaces multiple boolean parameters that were unclear at call sites.
//
// Before (unclear):
//
//	clearCache(true, false)  // What do true/false mean?
//
// After (self-documenting):
//
//	clearCacheWithOpts(CacheClearOptions{ExactOnly: true})
type CacheClearOptions struct {
	// ExactOnly clears only the exact-match cache (--exact flag)
	ExactOnly bool
	// SemanticOnly clears only the semantic cache (--semantic flag)
	SemanticOnly bool
}

// clearCacheWithOpts clears cache entries with confirmation using options struct.
// USABILITY: Options struct improves API clarity
func clearCacheWithOpts(opts CacheClearOptions) error {
	return clearCache(opts.ExactOnly, opts.SemanticOnly)
}

// clearCache clears cache entries with confirmation.
//
// Deprecated: Use clearCacheWithOpts for clearer API.
// This function is kept for backward compatibility.
func clearCache(exactOnly bool, semanticOnly bool) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".rigrun")
	exactPath := filepath.Join(cacheDir, "cache.json")
	semanticPath := filepath.Join(cacheDir, "semantic_cache.json")

	// Determine what to clear
	clearExact := !semanticOnly || exactOnly
	clearSemantic := !exactOnly || semanticOnly

	// If neither flag specified, clear both
	if !exactOnly && !semanticOnly {
		clearExact = true
		clearSemantic = true
	}

	// Count entries before clearing
	entryCount := 0
	cacheType := "all cached"

	if clearExact && !clearSemantic {
		cacheType = "exact-match cached"
		entryCount = countCacheEntries(exactPath)
	} else if clearSemantic && !clearExact {
		cacheType = "semantic cached"
		entryCount = countSemanticEntries(semanticPath)
	} else {
		entryCount = countCacheEntries(exactPath) + countSemanticEntries(semanticPath)
	}

	if entryCount == 0 {
		fmt.Println()
		fmt.Println("No cache entries to clear.")
		fmt.Println()
		return nil
	}

	// Confirm with user
	if !confirmClear(cacheType, entryCount) {
		fmt.Println("Cancelled.")
		return nil
	}

	// Clear the caches
	if clearExact {
		if err := clearExactCache(exactPath); err != nil {
			fmt.Printf("Warning: Could not clear exact cache: %v\n", err)
		} else {
			fmt.Println("  Exact-match cache cleared")
		}
	}

	if clearSemantic {
		if err := clearSemanticCache(semanticPath); err != nil {
			if !os.IsNotExist(err) {
				fmt.Printf("Warning: Could not clear semantic cache: %v\n", err)
			}
		} else {
			fmt.Println("  Semantic cache cleared")
		}
	}

	fmt.Println()
	fmt.Println("Cache cleared successfully.")
	fmt.Println()

	return nil
}

// confirmClear prompts user to confirm cache clearing.
func confirmClear(cacheType string, entryCount int) bool {
	fmt.Println()
	fmt.Printf("This will delete %d %s entries. Continue? [y/N]: ", entryCount, cacheType)

	input := promptInput("")
	input = strings.ToLower(strings.TrimSpace(input))

	return input == "y" || input == "yes"
}

// clearExactCache clears the exact-match cache file.
func clearExactCache(path string) error {
	// Load cache, clear it, save empty
	c := cache.NewDefaultExactCache()
	c.Clear()
	return c.SaveToFile(path)
}

// clearSemanticCache clears the semantic cache file.
func clearSemanticCache(path string) error {
	// Simply remove the file
	return os.Remove(path)
}

// =============================================================================
// CACHE EXPORT
// =============================================================================

// exportCache exports the cache to a directory.
func exportCache(dir string) error {
	if dir == "" {
		dir = "."
	}

	// Ensure export directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create export directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".rigrun")
	timestamp := time.Now().Format("20060102_150405")

	fmt.Println()
	fmt.Println("rigrun Cache Export")
	fmt.Println(strings.Repeat("=", 19))
	fmt.Println()

	exported := 0

	// Export exact-match cache
	exactPath := filepath.Join(cacheDir, "cache.json")
	if _, err := os.Stat(exactPath); err == nil {
		exportPath := filepath.Join(dir, fmt.Sprintf("rigrun_cache_%s.json", timestamp))
		if err := copyFile(exactPath, exportPath); err != nil {
			fmt.Printf("  Warning: Could not export exact cache: %v\n", err)
		} else {
			fmt.Printf("  Exported exact-match cache to: %s\n", exportPath)
			exported++
		}
	}

	// Export semantic cache
	semanticPath := filepath.Join(cacheDir, "semantic_cache.json")
	if _, err := os.Stat(semanticPath); err == nil {
		exportPath := filepath.Join(dir, fmt.Sprintf("rigrun_semantic_%s.json", timestamp))
		if err := copyFile(semanticPath, exportPath); err != nil {
			fmt.Printf("  Warning: Could not export semantic cache: %v\n", err)
		} else {
			fmt.Printf("  Exported semantic cache to: %s\n", exportPath)
			exported++
		}
	}

	// Export metadata
	metadataPath := filepath.Join(dir, fmt.Sprintf("rigrun_metadata_%s.json", timestamp))
	if err := exportMetadata(metadataPath); err != nil {
		fmt.Printf("  Warning: Could not export metadata: %v\n", err)
	} else {
		fmt.Printf("  Exported metadata to: %s\n", metadataPath)
		exported++
	}

	fmt.Println()
	if exported > 0 {
		fmt.Printf("Exported %d file(s) to: %s\n", exported, dir)
	} else {
		fmt.Println("No cache data to export.")
	}
	fmt.Println()

	return nil
}

// exportMetadata creates a metadata file for the export.
func exportMetadata(path string) error {
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, ".rigrun")

	metadata := map[string]interface{}{
		"export_time":      time.Now().Format(time.RFC3339),
		"rigrun_version":   Version,
		"source_directory": cacheDir,
	}

	// Add cache stats if available
	if c, err := cache.LoadOrCreateDefault(); err == nil {
		stats := c.Stats()
		metadata["exact_cache"] = map[string]interface{}{
			"entries":  stats.Size,
			"hits":     stats.Hits,
			"misses":   stats.Misses,
			"hit_rate": stats.HitRate,
		}
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// countCacheEntries counts entries in a cache file.
func countCacheEntries(path string) int {
	c := cache.NewDefaultExactCache()
	if err := c.LoadFromFile(path); err != nil {
		return 0
	}
	return c.Len()
}

// countSemanticEntries counts entries in a semantic cache file.
func countSemanticEntries(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	var entries map[string]interface{}
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0
	}

	// Count embeddings key if present
	if embeddings, ok := entries["embeddings"].(map[string]interface{}); ok {
		return len(embeddings)
	}

	return len(entries)
}

// findOldestEntryTime finds the oldest cache entry time.
func findOldestEntryTime(c *cache.ExactCache) time.Time {
	entries := c.Entries()
	var oldest time.Time

	for _, entry := range entries {
		if oldest.IsZero() || entry.CreatedAt.Before(oldest) {
			oldest = entry.CreatedAt
		}
	}

	return oldest
}

// formatTimeAgo formats a time as a relative duration.
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case duration < 7*24*time.Hour:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case duration < 30*24*time.Hour:
		weeks := int(duration.Hours() / 24 / 7)
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		months := int(duration.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
