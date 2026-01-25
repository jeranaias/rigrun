// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tools provides the agentic tool system for rigrun TUI.
// web.go implements a secure WebFetch tool for fetching and processing web content.
package tools

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/jeranaias/rigrun-tui/internal/offline"
	"github.com/jeranaias/rigrun-tui/internal/security"
)

// =============================================================================
// SSRF PROTECTION - BLOCKED IP RANGES
// =============================================================================

// blockedCIDRs contains IP ranges that should be blocked to prevent SSRF attacks.
// Based on RFC1918 and other private/reserved address spaces.
var blockedCIDRs = []string{
	// IPv4 Private networks (RFC1918)
	"10.0.0.0/8",     // Class A private
	"172.16.0.0/12",  // Class B private
	"192.168.0.0/16", // Class C private

	// IPv4 Loopback
	"127.0.0.0/8", // Loopback

	// IPv4 Link-local
	"169.254.0.0/16", // Link-local

	// IPv4 Special purpose
	"0.0.0.0/8",          // "This" network
	"100.64.0.0/10",      // Shared address space (CGN)
	"192.0.0.0/24",       // IETF Protocol Assignments
	"192.0.2.0/24",       // Documentation (TEST-NET-1)
	"192.88.99.0/24",     // 6to4 relay anycast
	"198.18.0.0/15",      // Benchmarking
	"198.51.100.0/24",    // Documentation (TEST-NET-2)
	"203.0.113.0/24",     // Documentation (TEST-NET-3)
	"224.0.0.0/4",        // Multicast
	"240.0.0.0/4",        // Reserved for future use
	"255.255.255.255/32", // Broadcast

	// IPv6 Special addresses
	"::1/128",      // Loopback
	"::/128",       // Unspecified
	"64:ff9b::/96", // IPv4/IPv6 translation
	// NOTE: ::ffff:0:0/96 (IPv4-mapped) intentionally omitted - Go's net.ParseCIDR
	// incorrectly normalizes it to 0.0.0.0/0, blocking ALL IPv4 addresses.
	// IPv4-mapped bypass attacks are already prevented because Go normalizes
	// ::ffff:X.X.X.X to X.X.X.X, so IPv4 blocklists catch them.
	"100::/64",      // Discard prefix
	"2001::/32",     // Teredo
	"2001:10::/28",  // ORCHID
	"2001:20::/28",  // ORCHIDv2
	"2001:db8::/32", // Documentation
	"2002::/16",     // 6to4
	"fc00::/7",      // Unique local
	"fe80::/10",     // Link-local
	"ff00::/8",      // Multicast
}

// Cloud metadata endpoints that should be blocked
var blockedHosts = []string{
	"metadata.google.internal",
	"metadata.google.com",
	"169.254.169.254", // AWS/GCP/Azure metadata
	"metadata",
	"instance-data",
	"localhost",
}

// blockedNetworks is the parsed list of blocked CIDR ranges.
var blockedNetworks []*net.IPNet

// =============================================================================
// PERFORMANCE: Pre-compiled regex (compiled once at startup)
// =============================================================================

var (
	// HTML tag removal patterns
	htmlCommentRegex = regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlTagRegex     = regexp.MustCompile(`<[^>]*>`)

	// Block element patterns for plain text conversion
	blockEndTagRegexes   = make(map[string]*regexp.Regexp)
	blockStartTagRegexes = make(map[string]*regexp.Regexp)

	// Heading patterns (h1-h6)
	headingRegexes = make(map[int]*regexp.Regexp)

	// Link pattern
	linkRegex = regexp.MustCompile(`(?is)<a[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)

	// List patterns
	listItemStartRegex = regexp.MustCompile(`(?i)<li[^>]*>`)
	listItemEndRegex   = regexp.MustCompile(`(?i)</li>`)
	listContainerRegex = regexp.MustCompile(`(?i)</?[ou]l[^>]*>`)

	// Emphasis patterns
	boldRegex   = regexp.MustCompile(`(?i)<(strong|b)[^>]*>(.*?)</(strong|b)>`)
	italicRegex = regexp.MustCompile(`(?i)<(em|i)[^>]*>(.*?)</(em|i)>`)

	// Code block patterns
	preCodeRegex    = regexp.MustCompile(`(?is)<pre[^>]*><code[^>]*>(.*?)</code></pre>`)
	inlineCodeRegex = regexp.MustCompile(`(?i)<code[^>]*>(.*?)</code>`)
	preBlockRegex   = regexp.MustCompile(`(?is)<pre[^>]*>(.*?)</pre>`)

	// Paragraph patterns
	paragraphEndRegex   = regexp.MustCompile(`(?i)</p>`)
	paragraphStartRegex = regexp.MustCompile(`(?i)<p[^>]*>`)

	// BR tag pattern
	brTagRegex = regexp.MustCompile(`(?i)<br\s*/?>`)

	// Entity decoding patterns
	numericEntityRegex = regexp.MustCompile(`&#(\d+);`)

	// Whitespace cleanup patterns
	multiSpaceRegex   = regexp.MustCompile(`[ \t]+`)
	multiNewlineRegex = regexp.MustCompile(`\n{3,}`)

	// Tag content removal patterns (script, style, etc.)
	tagContentRegexes = make(map[string]*regexp.Regexp)
)

func init() {
	// Initialize block element regexes
	blockElements := []string{"p", "div", "br", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr", "td", "th"}
	for _, tag := range blockElements {
		blockEndTagRegexes[tag] = regexp.MustCompile(`(?i)</` + tag + `>`)
		blockStartTagRegexes[tag] = regexp.MustCompile(`(?i)<` + tag + `[^>]*>`)
	}

	// Initialize heading regexes
	for i := 1; i <= 6; i++ {
		tag := "h" + string(rune('0'+i))
		pattern := `(?i)<` + tag + `[^>]*>(.*?)</` + tag + `>`
		headingRegexes[i] = regexp.MustCompile(pattern)
	}

	// Initialize tag content removal regexes
	tagsToRemove := []string{"script", "style", "noscript", "iframe", "svg"}
	for _, tag := range tagsToRemove {
		pattern := `(?is)<` + tag + `[^>]*>.*?</` + tag + `>`
		tagContentRegexes[tag] = regexp.MustCompile(pattern)
	}

	// Initialize blocked networks for SSRF protection
	blockedNetworks = make([]*net.IPNet, 0, len(blockedCIDRs))
	for _, cidr := range blockedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			blockedNetworks = append(blockedNetworks, network)
		}
	}
}

// =============================================================================
// WEB FETCH EXECUTOR
// =============================================================================

// WebFetchExecutor implements secure web content fetching.
type WebFetchExecutor struct {
	// MaxResponseSize is the maximum response body size to read (default: 5MB)
	MaxResponseSize int64

	// Timeout is the maximum time for the entire request (default: 30s)
	Timeout time.Duration

	// MaxRedirects is the maximum number of redirects to follow (default: 5)
	MaxRedirects int

	// UserAgent is the User-Agent header to send
	UserAgent string
}

// SSRF protection errors
var (
	ErrBlockedIP        = errors.New("IP address is blocked (private/internal range)")
	ErrBlockedHost      = errors.New("hostname is blocked")
	ErrInvalidScheme    = errors.New("only http and https schemes are allowed")
	ErrInvalidURL       = errors.New("invalid URL")
	ErrTooManyRedirects = errors.New("too many redirects")
	ErrResponseTooLarge = errors.New("response body too large")
	ErrDNSRebinding     = errors.New("DNS rebinding detected")
)

// Execute fetches content from a URL and returns processed text.
func (e *WebFetchExecutor) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	// IL5 SC-7: Block WebFetch in offline mode
	if err := offline.CheckWebFetchAllowed(); err != nil {
		return Result{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Set defaults
	if e.MaxResponseSize == 0 {
		e.MaxResponseSize = 5 * 1024 * 1024 // 5MB
	}
	if e.Timeout == 0 {
		e.Timeout = 30 * time.Second
	}
	if e.MaxRedirects == 0 {
		e.MaxRedirects = 5
	}
	if e.UserAgent == "" {
		e.UserAgent = "RigrunBot/1.0 (LLM Agent; +https://github.com/jeranaias/rigrun-tui)"
	}

	// Extract parameters
	rawURL := getStringParam(params, "url", "")
	prompt := getStringParam(params, "prompt", "")
	outputFormat := getStringParam(params, "output_format", "text")

	// Validate URL
	if rawURL == "" {
		return Result{
			Success: false,
			Error:   "url parameter is required",
		}, nil
	}

	// Parse and validate URL
	parsedURL, err := e.validateURL(rawURL)
	if err != nil {
		return Result{
			Success: false,
			Error:   "URL validation failed: " + err.Error(),
		}, nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, e.Timeout)
	defer cancel()

	// Create secure HTTP client
	client, err := e.createSecureClient()
	if err != nil {
		return Result{
			Success: false,
			Error:   "failed to create HTTP client: " + err.Error(),
		}, nil
	}

	// Fetch the URL
	content, contentType, finalURL, err := e.fetch(ctx, client, parsedURL)
	if err != nil {
		return Result{
			Success: false,
			Error:   "fetch failed: " + err.Error(),
		}, nil
	}

	// Process content based on type
	var processedContent string
	if strings.Contains(contentType, "text/html") {
		processedContent = e.htmlToReadableText(content, outputFormat == "markdown")
	} else if strings.Contains(contentType, "text/") || strings.Contains(contentType, "application/json") {
		processedContent = content
	} else {
		processedContent = "[Binary or unsupported content type: " + contentType + "]"
	}

	// Build output
	var output strings.Builder
	output.WriteString("URL: ")
	output.WriteString(finalURL)
	output.WriteString("\n")
	output.WriteString("Content-Type: ")
	output.WriteString(contentType)
	output.WriteString("\n\n")

	if prompt != "" {
		output.WriteString("Query: ")
		output.WriteString(prompt)
		output.WriteString("\n\n")
	}

	output.WriteString("--- Content ---\n")
	output.WriteString(processedContent)

	return Result{
		Success:   true,
		Output:    output.String(),
		BytesRead: int64(len(content)),
	}, nil
}

// =============================================================================
// URL VALIDATION
// =============================================================================

// validateURL validates and normalizes a URL, checking for SSRF vulnerabilities.
func (e *WebFetchExecutor) validateURL(rawURL string) (*url.URL, error) {
	// Parse URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, ErrInvalidURL
	}

	// Check scheme
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, ErrInvalidScheme
	}

	// Upgrade HTTP to HTTPS for security
	if scheme == "http" {
		parsedURL.Scheme = "https"
	}

	// Get hostname (without port)
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return nil, ErrInvalidURL
	}

	// Check for blocked hostnames
	lowerHostname := strings.ToLower(hostname)
	for _, blocked := range blockedHosts {
		if lowerHostname == blocked || strings.HasSuffix(lowerHostname, "."+blocked) {
			return nil, ErrBlockedHost
		}
	}

	// If hostname is an IP address, validate it directly
	if ip := net.ParseIP(hostname); ip != nil {
		if e.isBlockedIP(ip) {
			return nil, ErrBlockedIP
		}
	}

	return parsedURL, nil
}

// isBlockedIP checks if an IP address is in a blocked range.
func (e *WebFetchExecutor) isBlockedIP(ip net.IP) bool {
	for _, network := range blockedNetworks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// =============================================================================
// SECURE HTTP CLIENT
// =============================================================================

// createSecureClient creates an HTTP client with SSRF protections.
func (e *WebFetchExecutor) createSecureClient() (*http.Client, error) {
	// Create a custom dialer that validates resolved IPs
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	// SECURITY: TLS verification required for production
	// Get secure TLS configuration from global transport security manager
	secureTLSConfig := security.GlobalTransportSecurity().GetTLSConfig()

	// Create transport with custom dial context for DNS rebinding protection
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Extract host from address
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			// Resolve hostname to IP addresses
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}

			// Check all resolved IPs against blocklist
			for _, ip := range ips {
				if e.isBlockedIP(ip) {
					return nil, ErrBlockedIP
				}
			}

			// Use first valid IP
			if len(ips) == 0 {
				return nil, errors.New("no IP addresses resolved")
			}

			// Connect to resolved IP
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
		// SECURITY: Use secure TLS config with MinVersion TLS 1.2 and approved cipher suites
		TLSClientConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			CipherSuites: security.ApprovedCipherSuites,
			// SECURITY: TLS verification required for production - never skip verification
			InsecureSkipVerify: false,
			// Use secure config from PKI manager if available
			RootCAs: secureTLSConfig.RootCAs,
		},
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		DisableCompression:    false,
	}

	// TOOLS: Proper timeout, validation, and resource cleanup
	// Create redirect checker - uses via slice length instead of closure variable
	// to avoid closure capturing bug where redirectCount could be shared across requests
	maxRedirects := e.MaxRedirects

	client := &http.Client{
		Transport: transport,
		Timeout:   e.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Use len(via) instead of closure-captured counter to track redirects
			// via contains the previous requests, so len(via) >= maxRedirects means too many
			if len(via) >= maxRedirects {
				return ErrTooManyRedirects
			}

			// Validate redirect URL
			_, err := e.validateURL(req.URL.String())
			if err != nil {
				return err
			}

			return nil
		},
	}

	return client, nil
}

// =============================================================================
// FETCH IMPLEMENTATION
// =============================================================================

// fetch performs the HTTP request and returns the content.
func (e *WebFetchExecutor) fetch(ctx context.Context, client *http.Client, u *url.URL) (content string, contentType string, finalURL string, err error) {
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return "", "", "", err
	}

	// Set headers
	req.Header.Set("User-Agent", e.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	// Get content type
	contentType = resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/plain"
	}

	// Get final URL after redirects
	finalURL = resp.Request.URL.String()

	// Limit response size
	limitedReader := io.LimitReader(resp.Body, e.MaxResponseSize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", "", "", err
	}

	if int64(len(body)) > e.MaxResponseSize {
		return "", "", "", ErrResponseTooLarge
	}

	return string(body), contentType, finalURL, nil
}

// =============================================================================
// HTML TO TEXT CONVERSION
// =============================================================================

// htmlToReadableText converts HTML to readable text or markdown.
func (e *WebFetchExecutor) htmlToReadableText(html string, asMarkdown bool) string {
	// Remove script and style tags
	html = removeTagContent(html, "script")
	html = removeTagContent(html, "style")
	html = removeTagContent(html, "noscript")
	html = removeTagContent(html, "iframe")
	html = removeTagContent(html, "svg")

	// Remove HTML comments
	html = removeHTMLComments(html)

	if asMarkdown {
		return htmlToMarkdown(html)
	}
	return htmlToPlainText(html)
}

// removeTagContent removes tags and their content.
// Uses pre-compiled regex from tagContentRegexes map.
func removeTagContent(html, tagName string) string {
	if re, ok := tagContentRegexes[tagName]; ok {
		return re.ReplaceAllString(html, "")
	}
	return html
}

// removeHTMLComments removes HTML comments.
// Uses pre-compiled htmlCommentRegex.
func removeHTMLComments(html string) string {
	return htmlCommentRegex.ReplaceAllString(html, "")
}

// htmlToMarkdown converts HTML to markdown format.
func htmlToMarkdown(html string) string {
	var result strings.Builder

	// Process common elements
	html = processHeadings(html)
	html = processLinks(html)
	html = processLists(html)
	html = processEmphasis(html)
	html = processCodeBlocks(html)
	html = processParagraphs(html)
	html = processBR(html)

	// Strip remaining tags
	html = stripHTMLTags(html)

	// Decode entities
	html = decodeHTMLEntities(html)

	// Clean up whitespace
	html = cleanWhitespace(html)

	result.WriteString(html)
	return result.String()
}

// htmlToPlainText converts HTML to plain text.
// Uses pre-compiled blockEndTagRegexes and blockStartTagRegexes.
func htmlToPlainText(html string) string {
	// Convert block elements to newlines using pre-compiled regexes
	blockElements := []string{"p", "div", "br", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr", "td", "th"}
	for _, tag := range blockElements {
		if endRe, ok := blockEndTagRegexes[tag]; ok {
			html = endRe.ReplaceAllString(html, "\n")
		}
		if startRe, ok := blockStartTagRegexes[tag]; ok {
			html = startRe.ReplaceAllString(html, "\n")
		}
	}

	// Strip all remaining tags
	html = stripHTMLTags(html)

	// Decode entities
	html = decodeHTMLEntities(html)

	// Clean up whitespace
	html = cleanWhitespace(html)

	return html
}

// processHeadings converts heading tags to markdown.
// Uses pre-compiled headingRegexes map.
func processHeadings(html string) string {
	for i := 1; i <= 6; i++ {
		prefix := strings.Repeat("#", i) + " "
		if re, ok := headingRegexes[i]; ok {
			html = re.ReplaceAllString(html, "\n\n"+prefix+"$1\n\n")
		}
	}
	return html
}

// processLinks converts anchor tags to markdown links.
// Uses pre-compiled linkRegex.
func processLinks(html string) string {
	// Pattern: <a href="url">text</a> -> [text](url)
	return linkRegex.ReplaceAllString(html, "[$2]($1)")
}

// processLists converts list tags to markdown.
// Uses pre-compiled listItemStartRegex, listItemEndRegex, listContainerRegex.
func processLists(html string) string {
	// Convert list items using pre-compiled regexes
	html = listItemStartRegex.ReplaceAllString(html, "\n- ")
	html = listItemEndRegex.ReplaceAllString(html, "")

	// Remove list containers
	html = listContainerRegex.ReplaceAllString(html, "\n")

	return html
}

// processEmphasis converts emphasis tags.
// Uses pre-compiled boldRegex and italicRegex.
func processEmphasis(html string) string {
	// Bold: <strong>, <b>
	html = boldRegex.ReplaceAllString(html, "**$2**")

	// Italic: <em>, <i>
	html = italicRegex.ReplaceAllString(html, "*$2*")

	return html
}

// processCodeBlocks converts code tags.
// Uses pre-compiled preCodeRegex, inlineCodeRegex, preBlockRegex.
func processCodeBlocks(html string) string {
	// Code blocks: <pre><code>
	html = preCodeRegex.ReplaceAllString(html, "\n```\n$1\n```\n")

	// Inline code: <code>
	html = inlineCodeRegex.ReplaceAllString(html, "`$1`")

	// Pre blocks without code
	html = preBlockRegex.ReplaceAllString(html, "\n```\n$1\n```\n")

	return html
}

// processParagraphs adds spacing around paragraphs.
// Uses pre-compiled paragraphEndRegex and paragraphStartRegex.
func processParagraphs(html string) string {
	html = paragraphEndRegex.ReplaceAllString(html, "\n\n")
	html = paragraphStartRegex.ReplaceAllString(html, "")
	return html
}

// processBR converts <br> tags to newlines.
// Uses pre-compiled brTagRegex.
func processBR(html string) string {
	return brTagRegex.ReplaceAllString(html, "\n")
}

// stripHTMLTags removes all HTML tags.
// Uses pre-compiled htmlTagRegex.
func stripHTMLTags(html string) string {
	return htmlTagRegex.ReplaceAllString(html, "")
}

// decodeHTMLEntities decodes common HTML entities.
func decodeHTMLEntities(html string) string {
	entities := map[string]string{
		"&nbsp;":   " ",
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   "\"",
		"&#39;":    "'",
		"&apos;":   "'",
		"&copy;":   "(c)",
		"&reg;":    "(R)",
		"&trade;":  "(TM)",
		"&mdash;":  "--",
		"&ndash;":  "-",
		"&hellip;": "...",
		"&lsquo;":  "'",
		"&rsquo;":  "'",
		"&ldquo;":  "\"",
		"&rdquo;":  "\"",
		"&bull;":   "*",
	}

	for entity, replacement := range entities {
		html = strings.ReplaceAll(html, entity, replacement)
	}

	// Decode numeric entities using pre-compiled numericEntityRegex
	html = numericEntityRegex.ReplaceAllStringFunc(html, func(s string) string {
		matches := numericEntityRegex.FindStringSubmatch(s)
		if len(matches) == 2 {
			var code int
			for _, c := range matches[1] {
				code = code*10 + int(c-'0')
			}
			if code > 0 && code < 0x10FFFF {
				return string(rune(code))
			}
		}
		return s
	})

	return html
}

// cleanWhitespace normalizes whitespace in the output.
// Uses pre-compiled multiSpaceRegex and multiNewlineRegex.
func cleanWhitespace(text string) string {
	// Replace multiple spaces with single space
	text = multiSpaceRegex.ReplaceAllString(text, " ")

	// Replace multiple newlines with double newline
	text = multiNewlineRegex.ReplaceAllString(text, "\n\n")

	// Trim lines
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimRightFunc(line, unicode.IsSpace)
		cleanLines = append(cleanLines, trimmed)
	}
	text = strings.Join(cleanLines, "\n")

	// Trim overall
	text = strings.TrimSpace(text)

	return text
}

// =============================================================================
// TOOL DEFINITION
// =============================================================================

// WebFetchTool fetches and processes web content securely.
var WebFetchTool = &Tool{
	Name:        "WebFetch",
	Description: "Fetch content from a URL and convert it to readable text. Includes SSRF protection to block internal/private IPs. Use this to retrieve web pages, documentation, or API responses.",
	Schema: Schema{
		Parameters: []Parameter{
			{
				Name:        "url",
				Type:        "string",
				Required:    true,
				Description: "The URL to fetch (must be http or https, will be upgraded to https)",
			},
			{
				Name:        "prompt",
				Type:        "string",
				Required:    false,
				Description: "Optional query or context about what information to look for in the content",
			},
			{
				Name:        "output_format",
				Type:        "string",
				Required:    false,
				Description: "Output format: 'text' (plain text) or 'markdown' (default: 'text')",
				Default:     "text",
			},
		},
	},
	RiskLevel:  RiskLow,
	Permission: PermissionAuto,
	Executor:   &WebFetchExecutor{},
}
