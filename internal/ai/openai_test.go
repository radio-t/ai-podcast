package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/radio-t/ai-podcast/internal/ai/mocks"
	"github.com/radio-t/ai-podcast/podcast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// processTTSResponse extracts audio data from TTS API response
func processTTSResponse(resp *http.Response) ([]byte, error) {
	var apiResult struct {
		Choices []struct {
			Message struct {
				Audio struct {
					Data string `json:"data"`
				} `json:"audio"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
		return nil, fmt.Errorf("failed to decode TTS response: %w", err)
	}

	if len(apiResult.Choices) == 0 {
		return nil, fmt.Errorf("no TTS response from API")
	}

	// decode base64 audio data
	result, err := base64.StdEncoding.DecodeString(apiResult.Choices[0].Message.Audio.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode audio data: %w", err)
	}

	return result, nil
}

func TestOpenAIService_PrepareHostDescriptions(t *testing.T) {
	service := NewOpenAIService("test-key", nil)

	hosts := []podcast.Host{
		{Name: "Alice", Gender: "female", Character: "Tech expert"},
		{Name: "Bob", Gender: "male", Character: "Economist"},
	}

	result := service.prepareHostDescriptions(hosts)
	expected := "Alice (female): Tech expert\nBob (male): Economist"
	assert.Equal(t, expected, result)
}

func TestOpenAIService_ExtractMessages(t *testing.T) {
	service := NewOpenAIService("test-key", nil)

	t.Run("valid json", func(t *testing.T) {
		content := `[{"host": "Alice", "content": "Hello"}, {"host": "Bob", "content": "Hi there"}]`
		messages, err := service.extractMessages(content)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
		assert.Equal(t, "Alice", messages[0].Host)
		assert.Equal(t, "Hello", messages[0].Content)
		assert.Equal(t, "Bob", messages[1].Host)
		assert.Equal(t, "Hi there", messages[1].Content)
	})

	t.Run("json with code blocks", func(t *testing.T) {
		content := "```json\n[{\"host\": \"Alice\", \"content\": \"Hello\"}]\n```"
		messages, err := service.extractMessages(content)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "Alice", messages[0].Host)
		assert.Equal(t, "Hello", messages[0].Content)
	})

	t.Run("json with simple code blocks", func(t *testing.T) {
		content := "```\n[{\"host\": \"Alice\", \"content\": \"Hello\"}]\n```"
		messages, err := service.extractMessages(content)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "Alice", messages[0].Host)
		assert.Equal(t, "Hello", messages[0].Content)
	})

	t.Run("json embedded in text", func(t *testing.T) {
		content := "Here is the discussion: [{\"host\": \"Alice\", \"content\": \"Hello\"}] and that's it."
		messages, err := service.extractMessages(content)
		require.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "Alice", messages[0].Host)
		assert.Equal(t, "Hello", messages[0].Content)
	})

	t.Run("invalid json", func(t *testing.T) {
		content := "not valid json at all"
		_, err := service.extractMessages(content)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse response as JSON")
	})
}

func TestGetSpeakingStyle(t *testing.T) {
	tests := []struct {
		voice    string
		contains []string
	}{
		{
			voice:    "onyx",
			contains: []string{"Алексей", "energetic", "enthusiastic"},
		},
		{
			voice:    "nova",
			contains: []string{"Мария", "analytical", "economist"},
		},
		{
			voice:    "echo",
			contains: []string{"Дмитрий", "skepticism", "seasoned"},
		},
		{
			voice:    "unknown",
			contains: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.voice, func(t *testing.T) {
			style := getSpeakingStyle(test.voice)
			if test.contains == nil {
				assert.Empty(t, style)
			} else {
				for _, keyword := range test.contains {
					assert.Contains(t, style, keyword)
				}
			}
		})
	}
}

func TestCreateTTSSystemPrompt(t *testing.T) {
	speakingStyle := "Test speaking style"
	result := createTTSSystemPrompt(speakingStyle)
	assert.Contains(t, result, "Test speaking style")
	assert.Contains(t, result, "tech podcast discussion")
	assert.Contains(t, result, "Russian language")
	assert.Contains(t, result, "natural reactions")
}

func TestOpenAIService_CreateDiscussionPrompt(t *testing.T) {
	service := NewOpenAIService("test-key", nil)
	hosts := []podcast.Host{
		{Name: "Alice", Gender: "female", Character: "Tech expert"},
		{Name: "Bob", Gender: "male", Character: "Economist"},
	}

	prompt := service.createDiscussionPrompt(hosts, 10, 5)
	assert.Contains(t, prompt, "Alice (female): Tech expert")
	assert.Contains(t, prompt, "Bob (male): Economist")
	assert.Contains(t, prompt, "10 messages total")
	assert.Contains(t, prompt, "5 minutes")
	assert.Contains(t, prompt, "Russian language")
	assert.Contains(t, prompt, "heated")
}

func TestOpenAIService_CallChatAPI(t *testing.T) {
	tests := []struct {
		name            string
		responseBody    string
		statusCode      int
		expectedError   string
		expectedContent string
	}{
		{
			name:       "successful api call",
			statusCode: 200,
			responseBody: `{
				"choices": [{
					"message": {
						"content": "test response content"
					}
				}]
			}`,
			expectedContent: "test response content",
		},
		{
			name:          "api error response",
			statusCode:    400,
			responseBody:  `{"error": "bad request"}`,
			expectedError: "API request failed with status 400",
		},
		{
			name:       "empty choices response",
			statusCode: 200,
			responseBody: `{
				"choices": []
			}`,
			expectedError: "no response from API",
		},
		{
			name:          "invalid json response",
			statusCode:    200,
			responseBody:  `invalid json`,
			expectedError: "failed to decode response",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.statusCode)
				_, _ = w.Write([]byte(test.responseBody))
			}))
			defer server.Close()

			service := &OpenAIService{
				apiKey:     "test-key",
				httpClient: &http.Client{},
			}

			request := OpenAIRequest{
				Model: "gpt-4o",
				Messages: []OpenAIMessage{
					{Role: "user", Content: "test message"},
				},
			}

			// temporarily override URL by creating custom request
			requestBody, _ := json.Marshal(request)
			req, _ := http.NewRequest("POST", server.URL, bytes.NewBuffer(requestBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-key")

			resp, err := service.httpClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				err = fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
			} else {
				var result struct {
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
					} `json:"choices"`
				}

				if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
					err = fmt.Errorf("failed to decode response: %w", decodeErr)
				} else if len(result.Choices) == 0 {
					err = fmt.Errorf("no response from API")
				}
			}

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOpenAIService_CallTTSAPI(t *testing.T) {
	tests := []struct {
		name          string
		responseBody  string
		statusCode    int
		expectedError string
	}{
		{
			name:       "successful tts call",
			statusCode: 200,
			responseBody: `{
				"choices": [{
					"message": {
						"audio": {
							"data": "dGVzdCBhdWRpbyBkYXRh"
						}
					}
				}]
			}`,
		},
		{
			name:          "tts error response",
			statusCode:    400,
			responseBody:  `{"error": "bad request"}`,
			expectedError: "TTS request failed with status 400",
		},
		{
			name:       "empty choices response",
			statusCode: 200,
			responseBody: `{
				"choices": []
			}`,
			expectedError: "no TTS response from API",
		},
		{
			name:       "invalid base64 audio data",
			statusCode: 200,
			responseBody: `{
				"choices": [{
					"message": {
						"audio": {
							"data": "invalid-base64!"
						}
					}
				}]
			}`,
			expectedError: "failed to decode audio data",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.statusCode)
				_, _ = w.Write([]byte(test.responseBody))
			}))
			defer server.Close()

			service := &OpenAIService{
				apiKey:     "test-key",
				httpClient: &http.Client{},
			}

			request := OpenAITTSRequest{
				Model:      "gpt-4o-audio-preview",
				Modalities: []string{"text", "audio"},
				Messages: []OpenAIMessage{
					{Role: "user", Content: "test message"},
				},
			}

			// simulate the TTS API call logic
			requestBody, _ := json.Marshal(request)
			req, _ := http.NewRequest("POST", server.URL, bytes.NewBuffer(requestBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-key")

			resp, err := service.httpClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			var result []byte
			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				err = fmt.Errorf("TTS request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
			} else {
				result, err = processTTSResponse(resp)
			}

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestOpenAIService_GenerateDiscussion(t *testing.T) {
	tests := []struct {
		name             string
		mockResponse     *http.Response
		mockError        error
		expectedError    string
		expectedTitle    string
		expectedMsgCount int
	}{
		{
			name: "successful discussion generation",
			mockResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"choices": [{"message": {"content": "[{\"host\": \"Alice\", \"content\": \"Hello world\"}, {\"host\": \"Bob\", \"content\": \"Hi there\"}]"}}]}`)),
				Header:     make(http.Header),
			},
			expectedTitle:    "test article",
			expectedMsgCount: 2,
		},
		{
			name: "api error response",
			mockResponse: &http.Response{
				StatusCode: 400,
				Body:       io.NopCloser(strings.NewReader(`{"error": "bad request"}`)),
				Header:     make(http.Header),
			},
			expectedError: "failed to generate discussion",
		},
		{
			name:          "http client error",
			mockError:     fmt.Errorf("network error"),
			expectedError: "failed to generate discussion",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockClient := &mocks.HTTPClientMock{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, "POST", req.Method)
					assert.Equal(t, "https://api.openai.com/v1/chat/completions", req.URL.String())
					assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
					assert.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))

					if test.mockError != nil {
						return nil, test.mockError
					}
					return test.mockResponse, nil
				},
			}

			service := NewOpenAIService("test-key", mockClient)
			params := podcast.GenerateDiscussionParams{
				ArticleText:    "test article content",
				Title:          "test article",
				Hosts:          []podcast.Host{{Name: "Alice"}, {Name: "Bob"}},
				TargetDuration: 5,
			}

			discussion, err := service.GenerateDiscussion(params)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedTitle, discussion.Title)
				assert.Len(t, discussion.Messages, test.expectedMsgCount)
			}
		})
	}
}

func TestOpenAIService_GenerateSpeech(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  *http.Response
		mockError     error
		expectedError string
		expectedAudio []byte
	}{
		{
			name: "successful speech generation",
			mockResponse: &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"choices": [{"message": {"audio": {"data": "dGVzdCBhdWRpbyBkYXRh"}}}]}`)),
				Header:     make(http.Header),
			},
			expectedAudio: []byte("test audio data"),
		},
		{
			name: "api error response",
			mockResponse: &http.Response{
				StatusCode: 400,
				Body:       io.NopCloser(strings.NewReader(`{"error": "bad request"}`)),
				Header:     make(http.Header),
			},
			expectedError: "TTS request failed with status 400",
		},
		{
			name:          "http client error",
			mockError:     fmt.Errorf("network error"),
			expectedError: "TTS request failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockClient := &mocks.HTTPClientMock{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, "POST", req.Method)
					assert.Equal(t, "https://api.openai.com/v1/chat/completions", req.URL.String())
					assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
					assert.Equal(t, "Bearer test-key", req.Header.Get("Authorization"))

					if test.mockError != nil {
						return nil, test.mockError
					}
					return test.mockResponse, nil
				},
			}

			service := NewOpenAIService("test-key", mockClient)

			audioData, err := service.GenerateSpeech("test text", "echo")

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedAudio, audioData)
			}
		})
	}
}

func TestOpenAIService_CallAPIErrorCases(t *testing.T) {
	t.Run("empty choices in chat response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"choices": []}`))
		}))
		defer server.Close()

		mockClient := &mocks.HTTPClientMock{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				testReq, _ := http.NewRequest(req.Method, server.URL, req.Body)
				testReq.Header = req.Header
				return http.DefaultClient.Do(testReq)
			},
		}

		service := &OpenAIService{
			apiKey:     "test-key",
			httpClient: mockClient,
		}

		req := OpenAIRequest{
			Model: "gpt-4o",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "test"},
			},
		}

		_, err := service.callChatAPI(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no response from API")
	})

	t.Run("malformed json in chat response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"choices": [{"message": {"content": }}}]}`)) // missing value
		}))
		defer server.Close()

		mockClient := &mocks.HTTPClientMock{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				testReq, _ := http.NewRequest(req.Method, server.URL, req.Body)
				testReq.Header = req.Header
				return http.DefaultClient.Do(testReq)
			},
		}

		service := &OpenAIService{
			apiKey:     "test-key",
			httpClient: mockClient,
		}

		req := OpenAIRequest{
			Model: "gpt-4o",
			Messages: []OpenAIMessage{
				{Role: "user", Content: "test"},
			},
		}

		_, err := service.callChatAPI(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode response")
	})

	t.Run("empty choices in TTS response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"choices": []}`))
		}))
		defer server.Close()

		mockClient := &mocks.HTTPClientMock{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				testReq, _ := http.NewRequest(req.Method, server.URL, req.Body)
				testReq.Header = req.Header
				return http.DefaultClient.Do(testReq)
			},
		}

		service := NewOpenAIService("test-key", mockClient)
		_, err := service.GenerateSpeech("test", "echo")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no TTS response from API")
	})

	t.Run("malformed json in TTS response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"choices": [{"message": {"audio": {"data": }}}]}`)) // missing value
		}))
		defer server.Close()

		mockClient := &mocks.HTTPClientMock{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				testReq, _ := http.NewRequest(req.Method, server.URL, req.Body)
				testReq.Header = req.Header
				return http.DefaultClient.Do(testReq)
			},
		}

		service := NewOpenAIService("test-key", mockClient)
		_, err := service.GenerateSpeech("test", "echo")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode TTS response")
	})

	t.Run("invalid base64 in TTS response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"choices": [{"message": {"audio": {"data": "!!!invalid-base64!!!"}}}]}`))
		}))
		defer server.Close()

		mockClient := &mocks.HTTPClientMock{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				testReq, _ := http.NewRequest(req.Method, server.URL, req.Body)
				testReq.Header = req.Header
				return http.DefaultClient.Do(testReq)
			},
		}

		service := NewOpenAIService("test-key", mockClient)
		_, err := service.GenerateSpeech("test", "echo")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode audio data")
	})
}
