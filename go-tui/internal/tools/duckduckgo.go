// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
// duckduckgo.go implements a DuckDuckGo HTML search tool for web search without API keys.
package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/jeranaias/rigrun-tui/internal/offline"
	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// PERFORMANCE: Pre-compiled regex (compiled once at startup)
// =============================================================================

var (
	// DuckDuckGo HTML parsing patterns
	ddgTitleRegex   = regexp.MustCompile(`(?s)<a[^>]+class="result__a"[^>]+href="([^"]+)"[^>]*>(.+?)</a>`)
	ddgSnippetRegex = regexp.MustCompile(`(?s)<a[^>]+class="result__snippet"[^>]*>(.+?)</a>`)

	// HTML cleaning patterns for DuckDuckGo results
	ddgTagRegex        = regexp.MustCompile(`<[^>]*>`)
	ddgWhitespaceRegex = regexp.MustCompile(`\s+`)
)

// =============================================================================
// DUCKDUCKGO SEARCH EXECUTOR
// =============================================================================

// DuckDuckGoSearchExecutor implements web search using DuckDuckGo HTML.
type DuckDuckGoSearchExecutor struct {
	// BaseURL is the DuckDuckGo HTML search endpoint
	BaseURL string

	// MaxResults is the maximum number of results to return (default: 5, max: 10)
	MaxResults int

	// Timeout is the maximum time for the request (default: 15s)
	Timeout time.Duration

	// UserAgent is the User-Agent header to send
	UserAgent string
}

// SearchResult represents a single search result.
type SearchResult struct {
	Title   string
	URL     string
	Snippet string
}

// Execute performs a DuckDuckGo search and returns formatted results.
func (e *DuckDuckGoSearchExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	// Block WebSearch in offline mode
	if err := offline.CheckWebFetchAllowed(); err != nil {
		return Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Set defaults
	if e.BaseURL == "" {
		e.BaseURL = "https://html.duckduckgo.com/html/"
	}
	if e.MaxResults == 0 {
		e.MaxResults = 5
	}
	if e.Timeout == 0 {
		e.Timeout = 15 * time.Second
	}
	if e.UserAgent == "" {
		e.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	}

	// Extract parameters
	query := getStringParam(params, "query", "")
	maxResults := getIntParam(params, "max_results", e.MaxResults)

	// Validate query
	if query == "" {
		return Result{
			Success: false,
			Error:   "query parameter is required",
		}, nil
	}

	// Validate max_results
	if maxResults < 1 {
		maxResults = 1
	}
	if maxResults > 10 {
		maxResults = 10
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	// Perform search
	results, err := e.search(ctx, query)
	if err != nil {
		return Result{
			Success: false,
			Error:   "search failed: " + err.Error(),
		}, nil
	}

	// Limit results
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	// Format output
	output := e.formatResults(query, results)

	return Result{
		Success:    true,
		Output:     output,
		MatchCount: len(results),
	}, nil
}

// search performs the actual DuckDuckGo search.
func (e *DuckDuckGoSearchExecutor) search(ctx context.Context, query string) ([]SearchResult, error) {
	// Build search URL
	searchURL := e.BaseURL + "?q=" + url.QueryEscape(query)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	// Set headers
	// Note: Don't set Accept-Encoding to gzip/deflate - Go's default http.Client
	// handles this automatically and decompresses. Manual Accept-Encoding breaks this.
	req.Header.Set("User-Agent", e.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")

	// Create HTTP client
	client := &http.Client{
		Timeout: e.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	// Read response body (limit to 5MB)
	limitedReader := io.LimitReader(resp.Body, 5*1024*1024)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}

	// Parse HTML to extract results
	results := e.parseHTML(string(body))

	return results, nil
}

// parseHTML extracts search results from DuckDuckGo HTML.
// Uses pre-compiled ddgTitleRegex and ddgSnippetRegex.
func (e *DuckDuckGoSearchExecutor) parseHTML(html string) []SearchResult {
	var results []SearchResult

	// DuckDuckGo HTML structure (2024+):
	// <div class="result results_links results_links_deep web-result ">
	//   <h2 class="result__title">
	//     <a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=URL">Title</a>
	//   </h2>
	//   <a class="result__snippet" href="...">Snippet text</a>
	// </div>

	// Pattern to match title links using pre-compiled regex
	titleMatches := ddgTitleRegex.FindAllStringSubmatch(html, 30)

	// Pattern to match snippets using pre-compiled regex
	snippetMatches := ddgSnippetRegex.FindAllStringSubmatch(html, 30)

	for i, match := range titleMatches {
		if len(match) < 3 {
			continue
		}

		rawURL := match[1]
		title := match[2]

		// DuckDuckGo uses &amp; for & in HTML - decode it for URL parsing
		rawURL = strings.ReplaceAll(rawURL, "&amp;", "&")

		// Extract actual URL from DuckDuckGo redirect
		// Format: //duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com
		actualURL := extractActualURL(rawURL)
		if actualURL == "" {
			continue
		}

		// Clean title
		title = cleanHTML(title)
		title = strings.TrimSpace(title)

		// Get corresponding snippet
		snippet := ""
		if i < len(snippetMatches) && len(snippetMatches[i]) >= 2 {
			snippet = cleanHTML(snippetMatches[i][1])
			snippet = strings.TrimSpace(snippet)
		}

		// Skip empty results
		if title == "" || actualURL == "" {
			continue
		}

		results = append(results, SearchResult{
			Title:   title,
			URL:     actualURL,
			Snippet: snippet,
		})

		// Stop if we have enough results
		if len(results) >= 20 {
			break
		}
	}

	return results
}

// extractActualURL extracts the real URL from DuckDuckGo's redirect wrapper.
func extractActualURL(ddgURL string) string {
	// Handle //duckduckgo.com/l/?uddg=ENCODED_URL format
	if strings.Contains(ddgURL, "uddg=") {
		// Parse the URL to extract the uddg parameter
		if strings.HasPrefix(ddgURL, "//") {
			ddgURL = "https:" + ddgURL
		}
		parsed, err := url.Parse(ddgURL)
		if err != nil {
			return ""
		}
		encodedURL := parsed.Query().Get("uddg")
		if encodedURL != "" {
			// URL is already decoded by Query().Get()
			return encodedURL
		}
	}

	// If it's already a direct URL
	if strings.HasPrefix(ddgURL, "http://") || strings.HasPrefix(ddgURL, "https://") {
		return ddgURL
	}

	return ""
}

// cleanHTML removes HTML tags and decodes entities.
// Uses pre-compiled ddgTagRegex and ddgWhitespaceRegex.
func cleanHTML(html string) string {
	// Remove HTML tags using pre-compiled regex
	text := ddgTagRegex.ReplaceAllString(html, "")

	// Decode HTML entities
	text = decodeHTMLEntities(text)

	// Clean up whitespace using pre-compiled regex
	text = ddgWhitespaceRegex.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	return text
}

// formatResults formats search results as readable text.
func (e *DuckDuckGoSearchExecutor) formatResults(query string, results []SearchResult) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("DuckDuckGo Search Results for: %s\n", query))
	output.WriteString(fmt.Sprintf("Found %d results\n\n", len(results)))

	if len(results) == 0 {
		output.WriteString("No results found.\n")
		return output.String()
	}

	output.WriteString("=" + strings.Repeat("=", 78) + "\n\n")

	for i, result := range results {
		output.WriteString(fmt.Sprintf("[%d] %s\n", i+1, result.Title))
		output.WriteString(fmt.Sprintf("    URL: %s\n", result.URL))

		if result.Snippet != "" {
			// UNICODE: Rune-aware truncation preserves multi-byte characters
			snippet := util.TruncateRunes(result.Snippet, 300)
			output.WriteString(fmt.Sprintf("    %s\n", snippet))
		}

		output.WriteString("\n")
	}

	output.WriteString("=" + strings.Repeat("=", 78) + "\n")

	return output.String()
}

// =============================================================================
// TOOL DEFINITION
// =============================================================================

// WebSearchTool performs web searches using DuckDuckGo.
var WebSearchTool = &Tool{
	Name:             "WebSearch",
	ShortDescription: "Search the web using DuckDuckGo. Returns titles, URLs, and snippets from search results.",
	Description: `Search the web using DuckDuckGo HTML search (free, no API key required).

USE THIS TOOL WHEN:
- You need to find current information not in your knowledge base
- You need to search for documentation, articles, or resources online
- You need to verify facts or find recent information
- You need to discover websites or resources related to a topic

FEATURES:
- Free web search using DuckDuckGo HTML interface
- No API key or authentication required
- Returns titles, URLs, and snippets for each result
- Configurable number of results (1-10, default 5)
- 15 second timeout to ensure responsiveness

LIMITATIONS:
- Results are parsed from HTML and may vary in quality
- Limited to 10 results maximum
- May be subject to rate limiting if used excessively
- Requires internet connection (blocked in offline mode)

EXAMPLE QUERIES:
- "golang context timeout example"
- "docker compose best practices 2024"
- "react hooks useEffect cleanup"
- "python async await tutorial"`,
	Schema: Schema{
		Parameters: []Parameter{
			{
				Name:        "query",
				Type:        "string",
				Required:    true,
				Description: "The search query to send to DuckDuckGo. Use natural language or keywords. Example: 'golang error handling best practices'",
			},
			{
				Name:        "max_results",
				Type:        "integer",
				Required:    false,
				Description: "Maximum number of results to return (1-10). Default: 5",
				Default:     5,
			},
		},
	},
	RiskLevel:  RiskLow,
	Permission: PermissionAuto,
	Executor:   &DuckDuckGoSearchExecutor{},
}
