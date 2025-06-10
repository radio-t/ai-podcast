package main

import (
	"testing"

	"github.com/radio-t/ai-podcast/podcast"
	"github.com/stretchr/testify/assert"
)

func TestTextProcessor_EstimateAudioDuration(t *testing.T) {
	tp := NewTextProcessor()

	tests := []struct {
		name     string
		text     string
		expected float64
		delta    float64
	}{
		{
			name:     "empty text",
			text:     "",
			expected: 0,
			delta:    0.01,
		},
		{
			name:     "short text",
			text:     "Привет, как дела?",
			expected: 1.02, // ~14 chars ÷ 5.5 = ~2.55 words ÷ 160 × 60 = ~0.96 seconds
			delta:    0.1,
		},
		{
			name:     "longer text",
			text:     "Искусственный интеллект - это имитация человеческого интеллекта в машинах, запрограммированных для мышления и обучения как люди.",
			expected: 7.7, // ~118 chars ÷ 5.5 = ~21.45 words ÷ 160 × 60 = ~8.04 seconds
			delta:    1.0,
		},
		{
			name:     "text with spaces and newlines",
			text:     "Привет,   как   дела?\n\n\nВсе   хорошо!",
			expected: 1.8, // ~22 chars (without spaces) ÷ 5.5 = ~4 words ÷ 160 × 60 = ~1.5 seconds
			delta:    0.5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tp.EstimateAudioDuration(tc.text)
			assert.InDelta(t, tc.expected, result, tc.delta)
		})
	}
}

func TestTextProcessor_TruncateString(t *testing.T) {
	tp := NewTextProcessor()

	tests := []struct {
		name      string
		input     string
		maxLength int
		expected  string
	}{
		{name: "shorter than max", input: "Hello", maxLength: 10, expected: "Hello"},
		{name: "equal to max", input: "Hello", maxLength: 5, expected: "Hello"},
		{name: "longer than max", input: "Hello, world!", maxLength: 5, expected: "Hello..."},
		{name: "empty string", input: "", maxLength: 5, expected: ""},
		{name: "zero max length", input: "Hello", maxLength: 0, expected: "..."},
		{name: "utf-8 russian text", input: "Привет, мир!", maxLength: 5, expected: "Приве..."},
		{name: "utf-8 emoji", input: "Hello 👋 World 🌍", maxLength: 8, expected: "Hello 👋 ..."},
		{name: "utf-8 chinese", input: "你好世界", maxLength: 2, expected: "你好..."},
		{name: "mixed utf-8", input: "Hi 👋 Привет", maxLength: 7, expected: "Hi 👋 Пр..."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tp.TruncateString(tc.input, tc.maxLength)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTextProcessor_EstimateTotalDuration(t *testing.T) {
	tp := NewTextProcessor()

	messages := []podcast.Message{
		{Host: "Host1", Content: "Привет, как дела?"},
		{Host: "Host2", Content: "Все хорошо, спасибо!"},
		{Host: "Host1", Content: "Отлично!"},
	}

	result := tp.EstimateTotalDuration(messages)
	assert.Greater(t, result, 0.0)
	assert.Less(t, result, 10.0) // should be a few seconds for these short messages
}

func TestTextProcessor_CalculateSpeechSpeed(t *testing.T) {
	tp := NewTextProcessor()

	tests := []struct {
		name                  string
		estimatedDuration     float64
		targetDurationMinutes int
		expected              float64
	}{
		{
			name:                  "zero estimated duration",
			estimatedDuration:     0,
			targetDurationMinutes: 10,
			expected:              1.0,
		},
		{
			name:                  "estimated equals target",
			estimatedDuration:     600, // 10 minutes
			targetDurationMinutes: 10,
			expected:              1.0,
		},
		{
			name:                  "estimated shorter than target",
			estimatedDuration:     300, // 5 minutes
			targetDurationMinutes: 10,
			expected:              1.2, // would be 2.0 but capped at 1.2
		},
		{
			name:                  "estimated longer than target",
			estimatedDuration:     1200, // 20 minutes
			targetDurationMinutes: 10,
			expected:              0.8, // would be 0.5 but capped at 0.8
		},
		{
			name:                  "very short estimated",
			estimatedDuration:     60, // 1 minute
			targetDurationMinutes: 10,
			expected:              1.2, // would be 10.0 but capped at 1.2
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tp.CalculateSpeechSpeed(tc.estimatedDuration, tc.targetDurationMinutes)
			assert.InEpsilon(t, tc.expected, result, 0.001)
		})
	}
}

// Test backward compatibility functions
func TestBackwardCompatibilityFunctions(t *testing.T) {
	t.Run("estimateAudioDuration", func(t *testing.T) {
		result := estimateAudioDuration("Привет")
		assert.Greater(t, result, 0.0)
	})

	t.Run("truncateString", func(t *testing.T) {
		result := truncateString("Hello, world!", 5)
		assert.Equal(t, "Hello...", result)
	})

	t.Run("calculateSpeechSpeed", func(t *testing.T) {
		result := calculateSpeechSpeed(600, 10)
		assert.InEpsilon(t, 1.0, result, 0.001)
	})

	t.Run("estimateTotalDuration", func(t *testing.T) {
		messages := []podcast.Message{{Host: "Test", Content: "Hello"}}
		result := estimateTotalDuration(messages)
		assert.Greater(t, result, 0.0)
	})
}
