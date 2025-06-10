package main

import (
	"testing"

	"github.com/radio-t/ai-podcast/podcast"
	"github.com/stretchr/testify/assert"
)

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
		assert.NoError(t, err)
		assert.Len(t, messages, 2)
		assert.Equal(t, "Alice", messages[0].Host)
		assert.Equal(t, "Hello", messages[0].Content)
		assert.Equal(t, "Bob", messages[1].Host)
		assert.Equal(t, "Hi there", messages[1].Content)
	})

	t.Run("json with code blocks", func(t *testing.T) {
		content := "```json\n[{\"host\": \"Alice\", \"content\": \"Hello\"}]\n```"
		messages, err := service.extractMessages(content)
		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "Alice", messages[0].Host)
		assert.Equal(t, "Hello", messages[0].Content)
	})

	t.Run("json with simple code blocks", func(t *testing.T) {
		content := "```\n[{\"host\": \"Alice\", \"content\": \"Hello\"}]\n```"
		messages, err := service.extractMessages(content)
		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "Alice", messages[0].Host)
		assert.Equal(t, "Hello", messages[0].Content)
	})

	t.Run("json embedded in text", func(t *testing.T) {
		content := "Here is the discussion: [{\"host\": \"Alice\", \"content\": \"Hello\"}] and that's it."
		messages, err := service.extractMessages(content)
		assert.NoError(t, err)
		assert.Len(t, messages, 1)
		assert.Equal(t, "Alice", messages[0].Host)
		assert.Equal(t, "Hello", messages[0].Content)
	})

	t.Run("invalid json", func(t *testing.T) {
		content := "not valid json at all"
		_, err := service.extractMessages(content)
		assert.Error(t, err)
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
