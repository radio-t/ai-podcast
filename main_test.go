package main

import (
	"testing"
)

func TestEstimateAudioDuration(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected float64
	}{
		{
			name:     "empty text",
			text:     "",
			expected: 0,
		},
		{
			name:     "short text",
			text:     "Привет, как дела?",
			expected: 1.02, // ~12 characters (excluding spaces) ÷ 5.5 = ~2.18 words ÷ 160 × 60 = ~0.82 seconds
		},
		{
			name:     "longer text",
			text:     "Искусственный интеллект - это имитация человеческого интеллекта в машинах, запрограммированных для мышления и обучения как люди.",
			expected: 7.7, // ~70 characters (excluding spaces) ÷ 5.5 = ~12.73 words ÷ 160 × 60 = ~4.77 seconds
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := estimateAudioDuration(tc.text)
			// Allow for small floating point differences
			if result < tc.expected*0.9 || result > tc.expected*1.1 {
				t.Errorf("estimateAudioDuration(%q) = %v, want %v (±10%%)", tc.text, result, tc.expected)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxLength int
		expected  string
	}{
		{
			name:      "shorter than max",
			input:     "Hello",
			maxLength: 10,
			expected:  "Hello",
		},
		{
			name:      "equal to max",
			input:     "Hello",
			maxLength: 5,
			expected:  "Hello",
		},
		{
			name:      "longer than max",
			input:     "Hello, world!",
			maxLength: 5,
			expected:  "Hello...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := truncateString(tc.input, tc.maxLength)
			if result != tc.expected {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tc.input, tc.maxLength, result, tc.expected)
			}
		})
	}
}

func TestCreateHostMap(t *testing.T) {
	hosts := []Host{
		{
			Name:   "TestHost1",
			Gender: "male",
			Voice:  "echo",
		},
		{
			Name:   "TestHost2",
			Gender: "female",
			Voice:  "nova",
		},
	}

	hostMap := createHostMap(hosts)

	// Check first host
	if info, ok := hostMap["TestHost1"]; !ok {
		t.Errorf("Expected TestHost1 to be in hostMap")
	} else {
		if info.gender != "male" {
			t.Errorf("Expected gender male, got %s", info.gender)
		}
		if info.voice != "echo" {
			t.Errorf("Expected voice echo, got %s", info.voice)
		}
	}

	// Check second host
	if info, ok := hostMap["TestHost2"]; !ok {
		t.Errorf("Expected TestHost2 to be in hostMap")
	} else {
		if info.gender != "female" {
			t.Errorf("Expected gender female, got %s", info.gender)
		}
		if info.voice != "nova" {
			t.Errorf("Expected voice nova, got %s", info.voice)
		}
	}
}

func TestCalculateSpeechSpeed(t *testing.T) {
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := calculateSpeechSpeed(tc.estimatedDuration, tc.targetDurationMinutes)
			if result != tc.expected {
				t.Errorf("calculateSpeechSpeed(%f, %d) = %f, want %f", 
                  tc.estimatedDuration, tc.targetDurationMinutes, result, tc.expected)
			}
		})
	}
}