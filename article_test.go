package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPArticleFetcher_Fetch(t *testing.T) {
	tests := []struct {
		name             string
		html             string
		statusCode       int
		expectedTitle    string
		expectError      bool
		minContentLength int
	}{
		{
			name: "successful fetch with article tag",
			html: `<html>
				<head><title>Test Article</title></head>
				<body>
					<article>
						<p>This is the first paragraph of the article content.</p>
						<p>This is the second paragraph with more detailed content to ensure proper extraction.</p>
					</article>
				</body>
			</html>`,
			statusCode:       http.StatusOK,
			expectedTitle:    "Test Article",
			expectError:      false,
			minContentLength: 50,
		},
		{
			name: "successful fetch with content div",
			html: `<html>
				<head><title>Another Article</title></head>
				<body>
					<div class="content">
						<p>This is a well-structured article with meaningful content that should be extracted properly.</p>
						<p>The second paragraph contains additional information that adds value to the overall article.</p>
					</div>
				</body>
			</html>`,
			statusCode:       http.StatusOK,
			expectedTitle:    "Another Article",
			expectError:      false,
			minContentLength: 80,
		},
		{
			name: "content too short",
			html: `<html>
				<head><title>Short Page</title></head>
				<body>
					<p>Short.</p>
				</body>
			</html>`,
			statusCode:  http.StatusOK,
			expectError: true,
		},
		{
			name:        "error status code",
			html:        "<html><body>Not Found</body></html>",
			statusCode:  http.StatusNotFound,
			expectError: true,
		},
		{
			name: "content length limit",
			html: `<html>
				<head><title>Long Article</title></head>
				<body>
					<article>
						<p>` + strings.Repeat("This is a very long article with repeated content that will exceed the maximum length limit. ", 200) + `</p>
					</article>
				</body>
			</html>`,
			statusCode:       http.StatusOK,
			expectedTitle:    "Long Article",
			expectError:      false,
			minContentLength: 8000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.html))
			}))
			defer server.Close()

			// create fetcher with lower minimum for testing
			fetcher := NewHTTPArticleFetcher(server.Client())
			if !tc.expectError && tc.minContentLength > 0 {
				fetcher.minTextLength = 50 // lower for testing
			}

			// fetch article
			content, title, err := fetcher.Fetch(server.URL)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedTitle, title)
				assert.NotEmpty(t, content)
				if tc.minContentLength > 0 {
					assert.GreaterOrEqual(t, len(content), tc.minContentLength)
				}
				// verify content truncation for long articles
				if tc.name == "content length limit" {
					assert.Contains(t, content, "...")
					assert.LessOrEqual(t, len(content), 8003) // 8000 + "..."
				}
			}
		})
	}
}

func TestHTTPArticleFetcher_FetchWithComplexHTML(t *testing.T) {
	// test with complex HTML containing navigation, ads, and other noise
	html := `<html>
		<head><title>Complex Article</title></head>
		<body>
			<nav>
				<ul><li><a href="#">Home</a></li><li><a href="#">About</a></li></ul>
			</nav>
			<div class="ads">Advertisement content here</div>
			<main>
				<article>
					<h1>The Main Article Title</h1>
					<p>This is the main article content that should be extracted cleanly.</p>
					<p>The second paragraph contains the core information that readers need.</p>
					<p>Additional paragraphs provide more detailed information about the topic.</p>
				</article>
				<aside>
					<h3>Related Articles</h3>
					<ul><li>Link 1</li><li>Link 2</li></ul>
				</aside>
			</main>
			<footer>Copyright notice and other footer content</footer>
		</body>
	</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	fetcher := NewHTTPArticleFetcher(server.Client())
	fetcher.minTextLength = 50 // lower for testing

	content, title, err := fetcher.Fetch(server.URL)

	require.NoError(t, err)
	assert.Equal(t, "The Main Article Title", title) // trafilatura uses H1 as title, which is more accurate
	assert.NotEmpty(t, content)

	// verify that main content is extracted
	assert.Contains(t, content, "main article content")
	assert.Contains(t, content, "core information")

	// verify that navigation and footer noise is excluded (trafilatura should filter this)
	assert.NotContains(t, content, "Advertisement")
	assert.NotContains(t, content, "Copyright notice")
	assert.NotContains(t, content, "Related Articles")
}

func TestHTTPArticleFetcher_FetchWithNetworkError(t *testing.T) {
	// create fetcher with custom client that always fails
	client := &http.Client{
		Transport: &failingTransport{},
	}
	fetcher := NewHTTPArticleFetcher(client)

	content, title, err := fetcher.Fetch("http://example.com")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch URL")
	assert.Empty(t, content)
	assert.Empty(t, title)
}

// failingTransport is a custom transport that always returns an error
type failingTransport struct{}

func (f *failingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, io.ErrUnexpectedEOF
}

func TestHTTPArticleFetcher_URLSchemeValidation(t *testing.T) {
	fetcher := NewHTTPArticleFetcher(&http.Client{})

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid http URL",
			url:         "http://example.com/article",
			expectError: true, // will fail on network, but not on scheme validation
			errorMsg:    "failed to fetch",
		},
		{
			name:        "valid https URL",
			url:         "https://example.com/article",
			expectError: true, // will fail on network, but not on scheme validation
			errorMsg:    "failed to fetch",
		},
		{
			name:        "invalid ftp scheme",
			url:         "ftp://example.com/article",
			expectError: true,
			errorMsg:    "unsupported URL scheme: ftp",
		},
		{
			name:        "invalid file scheme",
			url:         "file:///etc/passwd",
			expectError: true,
			errorMsg:    "unsupported URL scheme: file",
		},
		{
			name:        "invalid javascript scheme",
			url:         "javascript:alert('xss')",
			expectError: true,
			errorMsg:    "unsupported URL scheme: javascript",
		},
		{
			name:        "no scheme",
			url:         "example.com/article",
			expectError: true,
			errorMsg:    "unsupported URL scheme: ",
		},
		{
			name:        "malformed URL",
			url:         "://invalid-url",
			expectError: true,
			errorMsg:    "invalid URL",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := fetcher.Fetch(tc.url)
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				// in this test, all cases should error (either on scheme or network)
				require.Error(t, err)
			}
		})
	}
}
