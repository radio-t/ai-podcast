package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// HTTPArticleFetcher implements article fetching using HTTP
type HTTPArticleFetcher struct {
	client *http.Client
}

// NewHTTPArticleFetcher creates a new HTTP article fetcher
func NewHTTPArticleFetcher(client *http.Client) *HTTPArticleFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPArticleFetcher{client: client}
}

// Fetch downloads and extracts text from the given URL
func (f *HTTPArticleFetcher) Fetch(url string) (content, title string, err error) {
	// #nosec G107 -- URL is provided by command-line flag
	resp, err := f.client.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to fetch article: status code %d", resp.StatusCode)
	}

	// parse the HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// extract title
	title = doc.Find("title").Text()

	// extract article content
	content = f.extractContent(doc)

	// limit article length for API calls
	const maxContentLength = 8000
	if len(content) > maxContentLength {
		content = content[:maxContentLength] + "..."
	}

	return content, title, nil
}

// extractContent extracts the main text content from the HTML document
func (f *HTTPArticleFetcher) extractContent(doc *goquery.Document) string {
	var articleText strings.Builder

	// first try to find article content in common containers
	article := doc.Find("article, .article, .post, .content, main")
	if article.Length() > 0 {
		article.Find("p").Each(func(_ int, s *goquery.Selection) {
			articleText.WriteString(s.Text())
			articleText.WriteString("\n\n")
		})
	} else {
		// fallback to all paragraphs
		doc.Find("p").Each(func(_ int, s *goquery.Selection) {
			// skip very short paragraphs which are likely not article content
			if len(s.Text()) > 50 {
				articleText.WriteString(s.Text())
				articleText.WriteString("\n\n")
			}
		})
	}

	return strings.TrimSpace(articleText.String())
}
