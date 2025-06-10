package main

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/radio-t/ai-podcast/mocks"
	"github.com/radio-t/ai-podcast/podcast"
)

func TestCreateSpeechRequest(t *testing.T) {
	hostMap := map[string]podcast.HostInfo{
		"Host1": {Gender: "male", Voice: "echo"},
		"Host2": {Gender: "female", Voice: "nova"},
	}

	tests := []struct {
		name           string
		msg            podcast.Message
		index          int
		expectedGender string
		expectedVoice  string
	}{
		{
			name:           "host in map",
			msg:            podcast.Message{Host: "Host1", Content: "Test content"},
			index:          0,
			expectedGender: "male",
			expectedVoice:  "echo",
		},
		{
			name:           "host not in map uses defaults",
			msg:            podcast.Message{Host: "UnknownHost", Content: "Test content"},
			index:          1,
			expectedGender: "female",
			expectedVoice:  "nova",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := podcast.CreateSpeechRequestParams{
				Msg:     test.msg,
				Index:   test.index,
				HostMap: hostMap,
				APIKey:  "test-key",
			}

			req := createSpeechRequest(params)

			assert.Equal(t, test.msg, req.Msg)
			assert.Equal(t, test.index, req.Index)
			assert.Equal(t, test.expectedGender, req.Gender)
			assert.Equal(t, test.expectedVoice, req.Voice)
			assert.InEpsilon(t, 1.0, req.Speed, 0.001)
			assert.Equal(t, "test-key", req.APIKey)
		})
	}
}

func TestNewOpenAIService(t *testing.T) {
	t.Run("with nil http client", func(t *testing.T) {
		service := NewOpenAIService("test-key", nil)
		assert.Equal(t, "test-key", service.apiKey)
		assert.NotNil(t, service.httpClient)
	})
}

// Test using generated mocks
func TestGenerateAndStreamWithMocks(t *testing.T) {
	mockOpenAI := &mocks.OpenAIClientMock{}
	mockAudio := &mocks.AudioProcessorMock{}
	mockArticle := &mocks.ArticleFetcherMock{}

	// setup mock responses
	mockOpenAI.GenerateDiscussionFunc = func(params podcast.GenerateDiscussionParams) (podcast.Discussion, error) {
		return podcast.Discussion{
			Title: "Test Discussion",
			Messages: []podcast.Message{
				{Host: "Host1", Content: "Hello"},
				{Host: "Host2", Content: "Hi there"},
			},
		}, nil
	}

	mockOpenAI.GenerateSpeechFunc = func(text, voice string) ([]byte, error) {
		return []byte("audio data"), nil
	}

	mockAudio.PlayFunc = func(filename string) error {
		return nil
	}

	mockAudio.ConcatenateFunc = func(files []string, outputFile string) error {
		return nil
	}

	mockAudio.StreamToIcecastFunc = func(inputFile string, config podcast.Config) error {
		return nil
	}

	mockAudio.StreamFromConcatFunc = func(concatFile string, config podcast.Config) error {
		return nil
	}

	mockArticle.FetchFunc = func(url string) (content, title string, err error) {
		return "article content", "article title", nil
	}

	// test that mocks are working
	discussion, err := mockOpenAI.GenerateDiscussion(podcast.GenerateDiscussionParams{})
	require.NoError(t, err)
	assert.Equal(t, "Test Discussion", discussion.Title)

	audio, err := mockOpenAI.GenerateSpeech("test", "echo")
	require.NoError(t, err)
	assert.Equal(t, []byte("audio data"), audio)

	err = mockAudio.Play("test.mp3")
	require.NoError(t, err)

	err = mockAudio.Concatenate([]string{"file1.mp3", "file2.mp3"}, "output.mp3")
	require.NoError(t, err)

	err = mockAudio.StreamToIcecast("input.mp3", podcast.Config{})
	require.NoError(t, err)

	err = mockAudio.StreamFromConcat("concat.txt", podcast.Config{})
	require.NoError(t, err)

	content, title, err := mockArticle.Fetch("http://example.com")
	require.NoError(t, err)
	assert.Equal(t, "article content", content)
	assert.Equal(t, "article title", title)

	// verify mocks were called
	assert.Len(t, mockOpenAI.GenerateDiscussionCalls(), 1)
	assert.Len(t, mockOpenAI.GenerateSpeechCalls(), 1)
	assert.Len(t, mockAudio.PlayCalls(), 1)
	assert.Len(t, mockAudio.ConcatenateCalls(), 1)
	assert.Len(t, mockAudio.StreamToIcecastCalls(), 1)
	assert.Len(t, mockAudio.StreamFromConcatCalls(), 1)
	assert.Len(t, mockArticle.FetchCalls(), 1)
}

func TestNewFFmpegAudioProcessor(t *testing.T) {
	processor := NewFFmpegAudioProcessor()
	assert.NotNil(t, processor)
}

func TestNewHTTPArticleFetcher(t *testing.T) {
	fetcher := NewHTTPArticleFetcher(nil)
	assert.NotNil(t, fetcher)
}

func TestRunWithDependencies(t *testing.T) {
	tests := []struct {
		name          string
		config        podcast.Config
		fetchError    bool
		discussError  bool
		streamError   bool
		playError     bool
		expectedError string
	}{
		{
			name:   "successful dry run",
			config: podcast.Config{ArticleURL: "http://example.com", DryRun: true, TargetDuration: 5},
		},
		{
			name:   "successful stream to icecast",
			config: podcast.Config{ArticleURL: "http://example.com", DryRun: false, TargetDuration: 5},
		},
		{
			name:   "successful output to file",
			config: podcast.Config{ArticleURL: "http://example.com", OutputFile: "test.mp3", TargetDuration: 5},
		},
		{
			name:          "article fetch error",
			config:        podcast.Config{ArticleURL: "http://example.com", DryRun: true, TargetDuration: 5},
			fetchError:    true,
			expectedError: "error fetching article",
		},
		{
			name:          "discussion generation error",
			config:        podcast.Config{ArticleURL: "http://example.com", DryRun: true, TargetDuration: 5},
			discussError:  true,
			expectedError: "error generating discussion",
		},
		{
			name:          "streaming error",
			config:        podcast.Config{ArticleURL: "http://example.com", DryRun: false, TargetDuration: 5},
			streamError:   true,
			expectedError: "error streaming podcast",
		},
		{
			name:          "local playback error",
			config:        podcast.Config{ArticleURL: "http://example.com", DryRun: true, TargetDuration: 5},
			playError:     true,
			expectedError: "error playing podcast locally",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockArticle := &mocks.ArticleFetcherMock{}
			mockOpenAI := &mocks.OpenAIClientMock{}
			mockAudio := &mocks.AudioProcessorMock{}

			// setup mocks
			if test.fetchError {
				mockArticle.FetchFunc = func(url string) (string, string, error) {
					return "", "", assert.AnError
				}
			} else {
				mockArticle.FetchFunc = func(url string) (string, string, error) {
					return "article content", "article title", nil
				}
			}

			if test.discussError {
				mockOpenAI.GenerateDiscussionFunc = func(params podcast.GenerateDiscussionParams) (podcast.Discussion, error) {
					return podcast.Discussion{}, assert.AnError
				}
			} else {
				mockOpenAI.GenerateDiscussionFunc = func(params podcast.GenerateDiscussionParams) (podcast.Discussion, error) {
					return podcast.Discussion{
						Title: "test discussion",
						Messages: []podcast.Message{
							{Host: "host1", Content: "hello"},
							{Host: "host2", Content: "world"},
						},
					}, nil
				}
			}

			mockOpenAI.GenerateSpeechFunc = func(text, voice string) ([]byte, error) {
				return []byte("audio data"), nil
			}

			if test.streamError {
				mockAudio.StreamFromConcatFunc = func(concatFile string, config podcast.Config) error {
					return assert.AnError
				}
			} else {
				mockAudio.StreamFromConcatFunc = func(concatFile string, config podcast.Config) error {
					return nil
				}
			}

			if test.playError {
				mockAudio.PlayFunc = func(filename string) error {
					return assert.AnError
				}
			} else {
				mockAudio.PlayFunc = func(filename string) error {
					return nil
				}
				mockAudio.ConcatenateFunc = func(files []string, outputFile string) error {
					return nil
				}
			}

			err := runWithDependencies(test.config, mockArticle, mockOpenAI, mockAudio)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGenerateAndStreamToIcecast(t *testing.T) {
	tests := []struct {
		name          string
		speechError   bool
		concatError   bool
		streamError   bool
		expectedError string
	}{
		{
			name: "successful streaming",
		},
		{
			name:          "speech generation error",
			speechError:   true,
			expectedError: "failed to generate speech",
		},
		{
			name:          "streaming error",
			streamError:   true,
			expectedError: "assert.AnError general error for testing",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockOpenAI := &mocks.OpenAIClientMock{}
			mockAudio := &mocks.AudioProcessorMock{}

			hosts := []podcast.Host{
				{Name: "host1", Voice: "nova", Gender: "female"},
				{Name: "host2", Voice: "echo", Gender: "male"},
			}

			params := podcast.GenerateAndStreamParams{
				Discussion: podcast.Discussion{
					Title: "test discussion",
					Messages: []podcast.Message{
						{Host: "host1", Content: "hello"},
						{Host: "host2", Content: "world"},
					},
				},
				Config: podcast.Config{
					Hosts:          hosts,
					TargetDuration: 5,
					IcecastURL:     "localhost:8000",
					IcecastMount:   "/test",
				},
			}

			if test.speechError {
				mockOpenAI.GenerateSpeechFunc = func(text, voice string) ([]byte, error) {
					return nil, assert.AnError
				}
			} else {
				mockOpenAI.GenerateSpeechFunc = func(text, voice string) ([]byte, error) {
					return []byte("audio data"), nil
				}
			}

			if test.streamError {
				mockAudio.StreamFromConcatFunc = func(concatFile string, config podcast.Config) error {
					return assert.AnError
				}
			} else {
				mockAudio.StreamFromConcatFunc = func(concatFile string, config podcast.Config) error {
					return nil
				}
			}

			err := generateAndStreamToIcecast(params, mockOpenAI, mockAudio)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGenerateAndPlayLocally(t *testing.T) {
	tests := []struct {
		name          string
		config        podcast.Config
		speechError   bool
		playError     bool
		concatError   bool
		expectedError string
	}{
		{
			name:   "successful dry run",
			config: podcast.Config{DryRun: true},
		},
		{
			name:   "successful with output file",
			config: podcast.Config{OutputFile: "test.mp3"},
		},
		{
			name:          "speech generation error",
			config:        podcast.Config{DryRun: true},
			speechError:   true,
			expectedError: "failed to generate speech",
		},
		{
			name:          "play error",
			config:        podcast.Config{DryRun: true},
			playError:     true,
			expectedError: "failed to play audio",
		},
		{
			name:          "concatenate error",
			config:        podcast.Config{OutputFile: "test.mp3"},
			concatError:   true,
			expectedError: "assert.AnError",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockOpenAI := &mocks.OpenAIClientMock{}
			mockAudio := &mocks.AudioProcessorMock{}

			hosts := []podcast.Host{
				{Name: "host1", Voice: "nova", Gender: "female"},
			}

			params := podcast.GenerateAndStreamParams{
				Discussion: podcast.Discussion{
					Title: "test discussion",
					Messages: []podcast.Message{
						{Host: "host1", Content: "hello"},
					},
				},
				Config: test.config,
			}
			params.Config.Hosts = hosts
			params.Config.OpenAIAPIKey = "test-key"

			if test.speechError {
				mockOpenAI.GenerateSpeechFunc = func(text, voice string) ([]byte, error) {
					return nil, assert.AnError
				}
			} else {
				mockOpenAI.GenerateSpeechFunc = func(text, voice string) ([]byte, error) {
					return []byte("audio data"), nil
				}
			}

			if test.playError {
				mockAudio.PlayFunc = func(filename string) error {
					return assert.AnError
				}
			} else {
				mockAudio.PlayFunc = func(filename string) error {
					return nil
				}
			}

			if test.concatError {
				mockAudio.ConcatenateFunc = func(files []string, outputFile string) error {
					return assert.AnError
				}
			} else {
				mockAudio.ConcatenateFunc = func(files []string, outputFile string) error {
					return nil
				}
			}

			err := generateAndPlayLocally(params, mockOpenAI, mockAudio)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGenerateSpeechSegments(t *testing.T) {
	tests := []struct {
		name          string
		speechError   bool
		expectedError string
	}{
		{
			name: "successful generation",
		},
		{
			name:          "speech generation error",
			speechError:   true,
			expectedError: "failed to generate speech for message 0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockOpenAI := &mocks.OpenAIClientMock{}

			hostMap := map[string]podcast.HostInfo{
				"host1": {Voice: "nova", Gender: "female"},
				"host2": {Voice: "echo", Gender: "male"},
			}

			params := podcast.GenerateSpeechSegmentsParams{
				Messages: []podcast.Message{
					{Host: "host1", Content: "hello"},
					{Host: "host2", Content: "world"},
				},
				HostMap: hostMap,
				TempDir: t.TempDir(),
			}

			if test.speechError {
				mockOpenAI.GenerateSpeechFunc = func(text, voice string) ([]byte, error) {
					return nil, assert.AnError
				}
			} else {
				mockOpenAI.GenerateSpeechFunc = func(text, voice string) ([]byte, error) {
					return []byte("audio data"), nil
				}
			}

			audioFiles, err := generateSpeechSegments(params, mockOpenAI)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
				assert.Nil(t, audioFiles)
			} else {
				require.NoError(t, err)
				assert.Len(t, audioFiles, 2)
				assert.Contains(t, audioFiles[0], "segment_000.mp3")
				assert.Contains(t, audioFiles[1], "segment_001.mp3")
			}
		})
	}
}

func TestSpeechGenerationWorker(t *testing.T) {
	tests := []struct {
		name        string
		speechError bool
	}{
		{
			name: "successful worker",
		},
		{
			name:        "speech error",
			speechError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockOpenAI := &mocks.OpenAIClientMock{}

			requestChan := make(chan podcast.SpeechGenerationRequest, 1)
			resultChan := make(chan podcast.SpeechSegment, 1)
			stopChan := make(chan struct{})

			params := podcast.SpeechGenerationWorkerParams{
				RequestChan: requestChan,
				ResultChan:  resultChan,
				StopChan:    stopChan,
			}

			if test.speechError {
				mockOpenAI.GenerateSpeechFunc = func(text, voice string) ([]byte, error) {
					return nil, assert.AnError
				}
			} else {
				mockOpenAI.GenerateSpeechFunc = func(text, voice string) ([]byte, error) {
					return []byte("audio data"), nil
				}
			}

			go speechGenerationWorker(params, mockOpenAI)

			// send a request
			req := podcast.SpeechGenerationRequest{
				Msg:   podcast.Message{Host: "host1", Content: "test"},
				Index: 0,
				Voice: "nova",
			}
			requestChan <- req

			// get result
			result := <-resultChan

			assert.Equal(t, 0, result.Index)
			assert.Equal(t, "host1", result.Host)
			if test.speechError {
				require.Error(t, result.Error)
				assert.Nil(t, result.AudioData)
			} else {
				require.NoError(t, result.Error)
				assert.Equal(t, []byte("audio data"), result.AudioData)
			}

			// stop worker
			close(stopChan)
		})
	}
}

func TestProcessSegments(t *testing.T) {
	tests := []struct {
		name          string
		speechError   bool
		playError     bool
		expectedError string
	}{
		{
			name: "successful processing",
		},
		{
			name:          "speech error",
			speechError:   true,
			expectedError: "failed to generate speech for message 0",
		},
		{
			name:          "play error",
			playError:     true,
			expectedError: "failed to play audio",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockAudio := &mocks.AudioProcessorMock{}

			discussion := podcast.Discussion{
				Messages: []podcast.Message{
					{Host: "host1", Content: "hello"},
				},
			}
			config := podcast.Config{
				Hosts:        []podcast.Host{{Name: "host1", Voice: "nova", Gender: "female"}},
				DryRun:       true,
				OpenAIAPIKey: "test-key",
			}

			requestChan := make(chan podcast.SpeechGenerationRequest, 10)
			resultChan := make(chan podcast.SpeechSegment, 10)
			stopChan := make(chan struct{})
			segmentBuffer := make([]podcast.SpeechSegment, 0)
			bufferMutex := sync.Mutex{}
			currentIndex := 0

			params := podcast.ProcessSegmentsParams{
				Discussion:    discussion,
				Config:        config,
				RequestChan:   requestChan,
				ResultChan:    resultChan,
				StopChan:      stopChan,
				SegmentBuffer: &segmentBuffer,
				BufferMutex:   &bufferMutex,
				CurrentIndex:  &currentIndex,
				TempDir:       t.TempDir(),
			}

			if test.playError {
				mockAudio.PlayFunc = func(filename string) error {
					return assert.AnError
				}
			} else {
				mockAudio.PlayFunc = func(filename string) error {
					return nil
				}
			}

			// simulate worker response
			go func() {
				req := <-requestChan
				segment := podcast.SpeechSegment{
					AudioData: []byte("audio data"),
					Host:      req.Msg.Host,
					Index:     req.Index,
					Msg:       req.Msg,
				}
				if test.speechError {
					segment.Error = assert.AnError
				}
				resultChan <- segment
			}()

			audioFiles, err := processSegments(params, mockAudio)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
				assert.Nil(t, audioFiles)
			} else {
				require.NoError(t, err)
				assert.Len(t, audioFiles, 1)
			}
		})
	}
}

func TestProcessOrderedSegment(t *testing.T) {
	tests := []struct {
		name          string
		playedIndex   int
		bufferIndex   int
		isDryRun      bool
		playError     bool
		expectedError string
		shouldReturn  bool
	}{
		{
			name:         "successful processing with dry run",
			playedIndex:  0,
			bufferIndex:  0,
			isDryRun:     true,
			shouldReturn: true,
		},
		{
			name:         "successful processing without dry run",
			playedIndex:  0,
			bufferIndex:  0,
			isDryRun:     false,
			shouldReturn: true,
		},
		{
			name:         "segment not ready",
			playedIndex:  0,
			bufferIndex:  1,
			shouldReturn: false,
		},
		{
			name:          "play error",
			playedIndex:   0,
			bufferIndex:   0,
			isDryRun:      true,
			playError:     true,
			expectedError: "failed to play audio",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockAudio := &mocks.AudioProcessorMock{}

			segmentBuffer := []podcast.SpeechSegment{
				{
					AudioData: []byte("audio data"),
					Host:      "host1",
					Index:     test.bufferIndex,
					Msg:       podcast.Message{Host: "host1", Content: "hello"},
				},
			}
			bufferMutex := sync.Mutex{}

			params := podcast.ProcessOrderedSegmentParams{
				SegmentBuffer: &segmentBuffer,
				BufferMutex:   &bufferMutex,
				PlayedIndex:   test.playedIndex,
				TempDir:       t.TempDir(),
				Config:        podcast.Config{DryRun: test.isDryRun},
			}

			if test.playError {
				mockAudio.PlayFunc = func(filename string) error {
					return assert.AnError
				}
			} else {
				mockAudio.PlayFunc = func(filename string) error {
					return nil
				}
			}

			result, err := processOrderedSegment(params, mockAudio)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				if test.shouldReturn {
					assert.NotNil(t, result)
					assert.Contains(t, *result, "segment_000.mp3")
				} else {
					assert.Nil(t, result)
				}
			}
		})
	}
}

func TestPlaySegment(t *testing.T) {
	tests := []struct {
		name          string
		playError     bool
		expectedError string
	}{
		{
			name: "successful playback",
		},
		{
			name:          "play error",
			playError:     true,
			expectedError: "failed to play audio",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mockAudio := &mocks.AudioProcessorMock{}

			params := podcast.PlaySegmentParams{
				Segment: podcast.SpeechSegment{
					Host: "host1",
					Msg:  podcast.Message{Host: "host1", Content: "hello world"},
				},
				Index:    0,
				Filename: "test.mp3",
			}

			if test.playError {
				mockAudio.PlayFunc = func(filename string) error {
					return assert.AnError
				}
			} else {
				mockAudio.PlayFunc = func(filename string) error {
					return nil
				}
			}

			err := playSegment(params, mockAudio)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				require.NoError(t, err)
			}

			assert.Len(t, mockAudio.PlayCalls(), 1)
			assert.Equal(t, "test.mp3", mockAudio.PlayCalls()[0].Filename)
		})
	}
}
