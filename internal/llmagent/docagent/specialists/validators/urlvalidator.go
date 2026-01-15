// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"context"
	"fmt"
	"io"
	"iter"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/elastic/elastic-package/internal/logger"
)

const (
	urlValidatorAgentName        = "url_validator"
	urlValidatorAgentDescription = "Validates URLs in documentation content for accessibility"

	// HTTP client configuration
	httpTimeout       = 10 * time.Second
	maxConcurrentReqs = 5

	// Soft 404 detection configuration
	maxBodyReadSize = 64 * 1024 // Read up to 64KB to check for soft 404
)

// URLValidatorAgent validates URLs in documentation content using HTTP requests.
type URLValidatorAgent struct{}

// NewURLValidatorAgent creates a new URL validator agent.
func NewURLValidatorAgent() *URLValidatorAgent {
	return &URLValidatorAgent{}
}

// Name returns the agent name.
func (u *URLValidatorAgent) Name() string {
	return urlValidatorAgentName
}

// Description returns the agent description.
func (u *URLValidatorAgent) Description() string {
	return urlValidatorAgentDescription
}

// URLCheckResult represents the result of URL validation
type URLCheckResult struct {
	TotalURLs   int      `json:"total_urls"`
	ValidURLs   []string `json:"valid_urls"`
	InvalidURLs []string `json:"invalid_urls"`
	Soft404URLs []string `json:"soft_404_urls"` // URLs that return 200 but show error content
	Warnings    []string `json:"warnings"`
}

// Build creates the underlying ADK agent.
// This agent always uses programmatic HTTP validation, not LLM calls.
func (u *URLValidatorAgent) Build(ctx context.Context, cfg AgentConfig) (agent.Agent, error) {
	return agent.New(agent.Config{
		Name:        urlValidatorAgentName,
		Description: urlValidatorAgentDescription,
		Run:         u.run,
	})
}

// run implements the agent logic using HTTP requests to validate URLs
func (u *URLValidatorAgent) run(invCtx agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		state := invCtx.Session().State()

		// Read content from state
		content, err := state.Get(StateKeyContent)
		if err != nil {
			logger.Debugf("URLValidator: no content found in state")
			event := session.NewEvent(invCtx.InvocationID())
			event.Content = genai.NewContentFromText("No content to check for URLs", genai.RoleModel)
			event.Author = urlValidatorAgentName
			event.Actions.StateDelta = map[string]any{
				StateKeyURLCheck: URLCheckResult{
					TotalURLs: 0,
					Warnings:  []string{"No content provided for URL validation"},
				},
			}
			yield(event, nil)
			return
		}

		contentStr, _ := content.(string)

		// Create context with timeout for HTTP operations.
		// Note: ADK InvocationContext doesn't expose parent context, so we use Background.
		// The 2-minute timeout provides reasonable protection against hanging requests.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Validate URLs using HTTP requests
		result := u.validateURLs(ctx, contentStr)

		// Create event with state update
		event := session.NewEvent(invCtx.InvocationID())
		event.Content = genai.NewContentFromText("URL validation complete", genai.RoleModel)
		event.Author = urlValidatorAgentName
		event.Actions.StateDelta = map[string]any{
			StateKeyURLCheck: result,
		}

		yield(event, nil)
	}
}

// urlPattern matches URLs in text
var urlPattern = regexp.MustCompile(`https?://[^\s\)>\]"']+`)

// markdownLinkPattern matches markdown links [text](url)
var markdownLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// internalLinkPattern matches internal docs-content:// links that should not appear in public docs
var internalLinkPattern = regexp.MustCompile(`docs-content://[^\s\)>\]"']+`)

// urlCheckResult holds the result of checking a single URL
type urlCheckResult struct {
	url       string
	valid     bool
	isSoft404 bool
	warning   string
}

// validateURLs extracts and validates URLs from content using HTTP requests
func (u *URLValidatorAgent) validateURLs(ctx context.Context, content string) URLCheckResult {
	// First, check for internal docs-content:// links that should not appear in public docs
	internalLinks := u.extractInternalLinks(content)
	var warnings []string
	var invalidURLs []string
	for _, link := range internalLinks {
		warning := fmt.Sprintf("Internal link found (should use public URL): %s - Replace with https://www.elastic.co/guide/... equivalent", link)
		warnings = append(warnings, warning)
		invalidURLs = append(invalidURLs, link)
	}

	// Extract all URLs
	urls := u.extractURLs(content)
	if len(urls) == 0 && len(internalLinks) == 0 {
		return URLCheckResult{TotalURLs: 0}
	}
	if len(urls) == 0 {
		// Only internal links found
		return URLCheckResult{
			TotalURLs:   len(internalLinks),
			InvalidURLs: invalidURLs,
			Warnings:    warnings,
		}
	}

	// Deduplicate URLs
	uniqueURLs := deduplicate(urls)

	// Create HTTP client
	client := &http.Client{
		Timeout: httpTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow redirects but track them
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// Check URLs concurrently with bounded parallelism
	results := u.checkURLsConcurrently(ctx, client, uniqueURLs)

	// Aggregate results (start with any internal link issues found earlier)
	var validURLs, soft404URLs []string
	for _, r := range results {
		if r.valid {
			validURLs = append(validURLs, r.url)
		} else if r.isSoft404 {
			soft404URLs = append(soft404URLs, r.url)
			invalidURLs = append(invalidURLs, r.url) // Also add to invalid for backwards compatibility
		} else {
			invalidURLs = append(invalidURLs, r.url)
		}
		if r.warning != "" {
			warnings = append(warnings, r.warning)
		}
	}

	return URLCheckResult{
		TotalURLs:   len(uniqueURLs) + len(internalLinks),
		ValidURLs:   validURLs,
		InvalidURLs: invalidURLs,
		Soft404URLs: soft404URLs,
		Warnings:    warnings,
	}
}

// extractURLs extracts all URLs from content
func (u *URLValidatorAgent) extractURLs(content string) []string {
	var urls []string

	// Extract plain URLs
	urls = append(urls, urlPattern.FindAllString(content, -1)...)

	// Extract markdown links
	mdLinks := markdownLinkPattern.FindAllStringSubmatch(content, -1)
	for _, match := range mdLinks {
		if len(match) > 2 && strings.HasPrefix(match[2], "http") {
			urls = append(urls, match[2])
		}
	}

	return urls
}

// extractInternalLinks extracts docs-content:// internal links that should not appear in public docs
func (u *URLValidatorAgent) extractInternalLinks(content string) []string {
	var links []string

	// Extract plain internal links
	links = append(links, internalLinkPattern.FindAllString(content, -1)...)

	// Extract internal links from markdown
	mdLinks := markdownLinkPattern.FindAllStringSubmatch(content, -1)
	for _, match := range mdLinks {
		if len(match) > 2 && strings.HasPrefix(match[2], "docs-content://") {
			links = append(links, match[2])
		}
	}

	return deduplicate(links)
}

// checkURLsConcurrently checks multiple URLs with bounded parallelism
func (u *URLValidatorAgent) checkURLsConcurrently(ctx context.Context, client *http.Client, urls []string) []urlCheckResult {
	results := make([]urlCheckResult, len(urls))
	sem := make(chan struct{}, maxConcurrentReqs)
	var wg sync.WaitGroup

	for i, url := range urls {
		wg.Add(1)
		go func(idx int, urlToCheck string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = urlCheckResult{url: urlToCheck, valid: false, warning: "validation cancelled"}
				return
			}

			// Check the URL
			valid, isSoft404, warning := u.checkSingleURL(ctx, client, urlToCheck)
			results[idx] = urlCheckResult{url: urlToCheck, valid: valid, isSoft404: isSoft404, warning: warning}
		}(i, url)
	}

	wg.Wait()
	return results
}

// checkSingleURL performs HTTP request to validate a URL and checks for soft 404s
func (u *URLValidatorAgent) checkSingleURL(ctx context.Context, client *http.Client, url string) (valid bool, isSoft404 bool, warning string) {
	// Pre-validation checks
	if containsPlaceholder(url) {
		return false, false, fmt.Sprintf("URL contains placeholder: %s", url)
	}
	if isLocalhostURL(url) {
		return false, false, fmt.Sprintf("URL points to localhost: %s", url)
	}

	// Use GET request to be able to check response body for soft 404s
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, false, fmt.Sprintf("invalid URL format: %s", url)
	}

	// Set a user agent to avoid being blocked
	req.Header.Set("User-Agent", "elastic-package-url-validator/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return false, false, fmt.Sprintf("URL unreachable: %s (%v)", url, err)
	}
	defer resp.Body.Close()

	// Check status code first
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		// Even with 200 OK, check for soft 404 in body
		soft404Detected, soft404Reason := u.detectSoft404(resp)
		if soft404Detected {
			return false, true, fmt.Sprintf("URL returns 200 but appears to be a soft 404: %s (%s)", url, soft404Reason)
		}
		return true, false, ""
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		// Redirect - consider valid but warn
		return true, false, fmt.Sprintf("URL redirects (HTTP %d): %s", resp.StatusCode, url)
	case resp.StatusCode == 403:
		// Forbidden - might be valid but access restricted
		return true, false, fmt.Sprintf("URL access forbidden (HTTP 403): %s", url)
	case resp.StatusCode == 404:
		return false, false, fmt.Sprintf("URL not found (HTTP 404): %s", url)
	default:
		return false, false, fmt.Sprintf("URL returned HTTP %d: %s", resp.StatusCode, url)
	}
}

// detectSoft404 checks the response body for indicators of a soft 404 page
func (u *URLValidatorAgent) detectSoft404(resp *http.Response) (isSoft404 bool, reason string) {
	// Read limited portion of body
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyReadSize))
	if err != nil {
		// Can't read body, assume valid
		return false, ""
	}

	bodyLower := strings.ToLower(string(bodyBytes))

	// Check for common soft 404 patterns in HTML content
	// These are strong indicators that the page is showing an error
	soft404Patterns := []struct {
		pattern string
		reason  string
	}{
		// Common 404/error page titles
		{"<title>404", "page title contains 404"},
		{"<title>page not found", "page title indicates not found"},
		{"<title>error 404", "page title indicates error 404"},
		{"<title>not found", "page title indicates not found"},
		{"<title>page does not exist", "page title indicates non-existent"},
		{"<title>page has been removed", "page title indicates removed"},
		{"<title>page has moved", "page title indicates moved"},
		{"<title>content not found", "page title indicates content not found"},
		{"<title>resource not found", "page title indicates resource not found"},
		{"<title>oops", "page title suggests error page"},

		// Heading-based indicators (more specific to avoid false positives)
		{"<h1>404</h1>", "heading indicates 404"},
		{"<h1>page not found</h1>", "heading indicates not found"},
		{"<h1>not found</h1>", "heading indicates not found"},
		{">404 - ", "content contains 404 error pattern"},
		{">error 404<", "content contains error 404"},

		// Common error page text patterns (more specific)
		{"the page you requested could not be found", "error message found"},
		{"the page you are looking for doesn't exist", "error message found"},
		{"the page you are looking for does not exist", "error message found"},
		{"this page doesn't exist", "error message found"},
		{"this page does not exist", "error message found"},
		{"page you're looking for can't be found", "error message found"},
		{"page you were looking for doesn't exist", "error message found"},
		{"sorry, we couldn't find that page", "error message found"},
		{"we couldn't find the page", "error message found"},
		{"the requested url was not found", "error message found"},
		{"the requested page was not found", "error message found"},
		{"this content has been removed", "content removed message"},
		{"this article has been removed", "content removed message"},
		{"this document has been removed", "content removed message"},
		{"has been discontinued", "discontinued message"},
		{"no longer available", "content unavailable message"},
		{"page has been archived", "archived message"},

		// Technical error indicators
		{"http error 404", "HTTP error 404 text found"},
		{"http status 404", "HTTP status 404 text found"},
		{"status code: 404", "status code 404 found"},

		// Common CMS/framework error pages
		{"wp-content/themes/.*404", "WordPress 404 theme detected"},
		{"drupal-404", "Drupal 404 page detected"},
	}

	for _, p := range soft404Patterns {
		if strings.Contains(bodyLower, p.pattern) {
			return true, p.reason
		}
	}

	// Check for meta refresh to error page
	if strings.Contains(bodyLower, `http-equiv="refresh"`) &&
		(strings.Contains(bodyLower, "error") || strings.Contains(bodyLower, "404") ||
			strings.Contains(bodyLower, "not-found") || strings.Contains(bodyLower, "notfound")) {
		return true, "meta refresh to error page detected"
	}

	// Check response headers for soft 404 indicators
	if xRobotsTag := resp.Header.Get("X-Robots-Tag"); xRobotsTag != "" {
		if strings.Contains(strings.ToLower(xRobotsTag), "noindex") {
			// noindex alone is not a soft 404, but combined with error-like content
			if strings.Contains(bodyLower, "not found") || strings.Contains(bodyLower, "doesn't exist") {
				return true, "noindex with error-like content"
			}
		}
	}

	return false, ""
}

// deduplicate removes duplicate URLs from a slice
func deduplicate(urls []string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, url := range urls {
		if _, ok := seen[url]; !ok {
			seen[url] = struct{}{}
			result = append(result, url)
		}
	}
	return result
}

// containsPlaceholder checks if URL contains placeholder text
func containsPlaceholder(url string) bool {
	placeholders := []string{"example.com", "placeholder", "your-", "xxx", "PLACEHOLDER", "YOUR_"}
	lowerURL := strings.ToLower(url)
	for _, p := range placeholders {
		if strings.Contains(lowerURL, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// isLocalhostURL checks if URL points to localhost
func isLocalhostURL(url string) bool {
	localhostPatterns := []string{"localhost", "127.0.0.1", "0.0.0.0"}
	lowerURL := strings.ToLower(url)
	for _, p := range localhostPatterns {
		if strings.Contains(lowerURL, p) {
			return true
		}
	}
	return false
}
