package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPArticleFetcher_Fetch(t *testing.T) {
	tests := []struct {
		name            string
		html            string
		statusCode      int
		expectedTitle   string
		expectedContent string
		expectError     bool
	}{
		{
			name: "successful fetch with article tag",
			html: `<html>
				<head><title>Test Article</title></head>
				<body>
					<article>
						<p>This is the first paragraph of the article.</p>
						<p>This is the second paragraph with more content.</p>
					</article>
				</body>
			</html>`,
			statusCode:      http.StatusOK,
			expectedTitle:   "Test Article",
			expectedContent: "This is the first paragraph of the article.\n\nThis is the second paragraph with more content.",
			expectError:     false,
		},
		{
			name: "successful fetch with class content",
			html: `<html>
				<head><title>Another Article</title></head>
				<body>
					<div class="content">
						<p>Content paragraph one.</p>
						<p>Content paragraph two with enough text to be included.</p>
					</div>
				</body>
			</html>`,
			statusCode:      http.StatusOK,
			expectedTitle:   "Another Article",
			expectedContent: "Content paragraph one.\n\nContent paragraph two with enough text to be included.",
			expectError:     false,
		},
		{
			name: "fallback to all paragraphs",
			html: `<html>
				<head><title>Simple Page</title></head>
				<body>
					<p>Short.</p>
					<p>This is a longer paragraph that should be included in the content extraction.</p>
					<p>Another long paragraph with sufficient content to pass the length filter.</p>
				</body>
			</html>`,
			statusCode:      http.StatusOK,
			expectedTitle:   "Simple Page",
			expectedContent: "This is a longer paragraph that should be included in the content extraction.\n\nAnother long paragraph with sufficient content to pass the length filter.",
			expectError:     false,
		},
		{
			name:            "error status code",
			html:            "<html><body>Not Found</body></html>",
			statusCode:      http.StatusNotFound,
			expectedTitle:   "",
			expectedContent: "",
			expectError:     true,
		},
		{
			name: "content length limit",
			html: `<html>
				<head><title>Long Article</title></head>
				<body>
					<article>
						<p>` + strings.Repeat("A", 9000) + `</p>
					</article>
				</body>
			</html>`,
			statusCode:      http.StatusOK,
			expectedTitle:   "Long Article",
			expectedContent: strings.Repeat("A", 8000) + "...",
			expectError:     false,
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

			// create fetcher
			fetcher := NewHTTPArticleFetcher(server.Client())

			// fetch article
			content, title, err := fetcher.Fetch(server.URL)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedTitle, title)
				assert.Equal(t, tc.expectedContent, strings.TrimSpace(content))
			}
		})
	}
}

func TestHTTPArticleFetcher_FetchWithNetworkError(t *testing.T) {
	// create fetcher with custom client that always fails
	client := &http.Client{
		Transport: &failingTransport{},
	}
	fetcher := NewHTTPArticleFetcher(client)

	content, title, err := fetcher.Fetch("http://example.com")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch URL")
	assert.Empty(t, content)
	assert.Empty(t, title)
}

// failingTransport is a custom transport that always returns an error
type failingTransport struct{}

func (f *failingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, io.ErrUnexpectedEOF
}
