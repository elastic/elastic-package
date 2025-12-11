// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package specialists

import (
	"context"
	"fmt"
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

		// Create context with timeout for HTTP operations
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

// urlCheckResult holds the result of checking a single URL
type urlCheckResult struct {
	url     string
	valid   bool
	warning string
}

// validateURLs extracts and validates URLs from content using HTTP requests
func (u *URLValidatorAgent) validateURLs(ctx context.Context, content string) URLCheckResult {
	// Extract all URLs
	urls := u.extractURLs(content)
	if len(urls) == 0 {
		return URLCheckResult{TotalURLs: 0}
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

	// Aggregate results
	var validURLs, invalidURLs, warnings []string
	for _, r := range results {
		if r.valid {
			validURLs = append(validURLs, r.url)
		} else {
			invalidURLs = append(invalidURLs, r.url)
		}
		if r.warning != "" {
			warnings = append(warnings, r.warning)
		}
	}

	return URLCheckResult{
		TotalURLs:   len(uniqueURLs),
		ValidURLs:   validURLs,
		InvalidURLs: invalidURLs,
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
			valid, warning := u.checkSingleURL(ctx, client, urlToCheck)
			results[idx] = urlCheckResult{url: urlToCheck, valid: valid, warning: warning}
		}(i, url)
	}

	wg.Wait()
	return results
}

// checkSingleURL performs HTTP HEAD request to validate a URL
func (u *URLValidatorAgent) checkSingleURL(ctx context.Context, client *http.Client, url string) (valid bool, warning string) {
	// Pre-validation checks
	if containsPlaceholder(url) {
		return false, fmt.Sprintf("URL contains placeholder: %s", url)
	}
	if isLocalhostURL(url) {
		return false, fmt.Sprintf("URL points to localhost: %s", url)
	}

	// Create HEAD request
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, fmt.Sprintf("invalid URL format: %s", url)
	}

	// Set a user agent to avoid being blocked
	req.Header.Set("User-Agent", "elastic-package-url-validator/1.0")

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		// Some servers don't support HEAD, try GET
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req.Header.Set("User-Agent", "elastic-package-url-validator/1.0")
		resp, err = client.Do(req)
		if err != nil {
			return false, fmt.Sprintf("URL unreachable: %s (%v)", url, err)
		}
	}
	defer resp.Body.Close()

	// Check status code
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return true, ""
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		// Redirect - consider valid but warn
		return true, fmt.Sprintf("URL redirects (HTTP %d): %s", resp.StatusCode, url)
	case resp.StatusCode == 403:
		// Forbidden - might be valid but access restricted
		return true, fmt.Sprintf("URL access forbidden (HTTP 403): %s", url)
	case resp.StatusCode == 404:
		return false, fmt.Sprintf("URL not found (HTTP 404): %s", url)
	default:
		return false, fmt.Sprintf("URL returned HTTP %d: %s", resp.StatusCode, url)
	}
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
