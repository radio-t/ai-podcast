package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
			assert.Equal(t, 1.0, req.Speed)
			assert.Equal(t, "test-key", req.APIKey)
		})
	}
}

func TestNewOpenAIService(t *testing.T) {
	t.Run("with nil http client", func(t *testing.T) {
		service := NewOpenAIService("test-key", nil)
		assert.Equal(t, "test-key", service.apiKey)
		assert.NotNil(t, service.httpClient)
		assert.Equal(t, 2*time.Minute, service.httpClient.Timeout)
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
	assert.NoError(t, err)
	assert.Equal(t, "Test Discussion", discussion.Title)

	audio, err := mockOpenAI.GenerateSpeech("test", "echo")
	assert.NoError(t, err)
	assert.Equal(t, []byte("audio data"), audio)

	err = mockAudio.Play("test.mp3")
	assert.NoError(t, err)

	err = mockAudio.Concatenate([]string{"file1.mp3", "file2.mp3"}, "output.mp3")
	assert.NoError(t, err)

	err = mockAudio.StreamToIcecast("input.mp3", podcast.Config{})
	assert.NoError(t, err)

	err = mockAudio.StreamFromConcat("concat.txt", podcast.Config{})
	assert.NoError(t, err)

	content, title, err := mockArticle.Fetch("http://example.com")
	assert.NoError(t, err)
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
