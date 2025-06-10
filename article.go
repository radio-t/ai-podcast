package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/markusmobius/go-trafilatura"
)

// HTTPArticleFetcher implements article fetching using HTTP and trafilatura
type HTTPArticleFetcher struct {
	client        *http.Client
	timeout       time.Duration
	userAgent     string
	minTextLength int
}

// NewHTTPArticleFetcher creates a new HTTP article fetcher with trafilatura
func NewHTTPArticleFetcher(client *http.Client) *HTTPArticleFetcher {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPArticleFetcher{
		client:        client,
		timeout:       30 * time.Second,
		userAgent:     "AI-Podcast/1.0",
		minTextLength: 100,
	}
}

// Fetch downloads and extracts text from the given URL using trafilatura
func (f *HTTPArticleFetcher) Fetch(urlStr string) (content, title string, err error) {
	// validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}

	// create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()

	// create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, http.NoBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	// set user agent
	req.Header.Set("User-Agent", f.userAgent)

	// perform HTTP request
	resp, err := f.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to fetch article: status code %d", resp.StatusCode)
	}

	// extract content using trafilatura
	options := trafilatura.Options{
		EnableFallback:  true,
		ExcludeComments: true,
		ExcludeTables:   false,
		IncludeImages:   false,
		IncludeLinks:    false,
		Deduplicate:     true,
		OriginalURL:     parsedURL,
	}

	result, err := trafilatura.Extract(resp.Body, options)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract content: %w", err)
	}

	// validate content length
	if len(result.ContentText) < f.minTextLength {
		return "", "", fmt.Errorf("extracted content too short (%d chars, minimum %d)",
			len(result.ContentText), f.minTextLength)
	}

	// get title from trafilatura result metadata
	title = result.Metadata.Title
	if title == "" {
		title = result.Metadata.Sitename
	}
	if title == "" {
		title = "Untitled Article"
	}

	content = result.ContentText

	// limit article length for API calls
	const maxContentLength = 8000
	if len(content) > maxContentLength {
		content = content[:maxContentLength] + "..."
	}

	return content, title, nil
}
