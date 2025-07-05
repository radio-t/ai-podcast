package content

import (
	"math"
	"unicode/utf8"

	"github.com/radio-t/ai-podcast/podcast"
)

// TextProcessor handles text-related operations
type TextProcessor struct{}

// NewTextProcessor creates a new text processor
func NewTextProcessor() *TextProcessor {
	return &TextProcessor{}
}

// EstimateAudioDuration estimates the spoken duration of text in seconds
func (tp *TextProcessor) EstimateAudioDuration(text string) float64 {
	// average Russian reading speed is about 150-170 words per minute
	// we'll use 160 words per minute as our baseline
	// russian has an average of about 5-6 characters per word

	// count characters excluding spaces
	charCount := 0
	for _, char := range text {
		if char != ' ' && char != '\n' && char != '\t' && char != '\r' {
			charCount++
		}
	}

	// estimate word count
	estimatedWords := float64(charCount) / avgCharsPerWordRussian

	// calculate duration in seconds
	durationSeconds := estimatedWords / avgWordsPerMinuteRussian * 60.0

	return durationSeconds
}

// EstimateTotalDuration estimates the total duration of all messages
func (tp *TextProcessor) EstimateTotalDuration(messages []podcast.Message) float64 {
	var totalDuration float64
	for _, msg := range messages {
		totalDuration += tp.EstimateAudioDuration(msg.Content)
	}
	return totalDuration
}

// CalculateSpeechSpeed determines the speech speed factor to match target duration
func (tp *TextProcessor) CalculateSpeechSpeed(estimatedDuration float64, targetDurationMinutes int) float64 {
	speechSpeed := 1.0
	if estimatedDuration <= 0 {
		return speechSpeed
	}

	// target duration in seconds
	targetDurationSeconds := float64(targetDurationMinutes * 60)

	// if estimated duration is significantly different from target, adjust speed
	// but keep it within reasonable bounds
	speechSpeed = targetDurationSeconds / estimatedDuration
	return math.Max(minSpeechSpeed, math.Min(maxSpeechSpeed, speechSpeed))
}

// TruncateString truncates a string to the specified length and adds "..." if truncated
// it ensures UTF-8 characters are not broken
func (tp *TextProcessor) TruncateString(s string, maxLength int) string {
	if utf8.RuneCountInString(s) <= maxLength {
		return s
	}

	// convert to runes to handle multi-byte characters properly
	runes := []rune(s)
	if len(runes) <= maxLength {
		return s
	}

	return string(runes[:maxLength]) + "..."
}

// Keep these as package-level functions for backward compatibility
func estimateAudioDuration(text string) float64 {
	tp := NewTextProcessor()
	return tp.EstimateAudioDuration(text)
}

func estimateTotalDuration(messages []podcast.Message) float64 {
	tp := NewTextProcessor()
	return tp.EstimateTotalDuration(messages)
}

func calculateSpeechSpeed(estimatedDuration float64, targetDurationMinutes int) float64 {
	tp := NewTextProcessor()
	return tp.CalculateSpeechSpeed(estimatedDuration, targetDurationMinutes)
}

func truncateString(s string, maxLength int) string {
	tp := NewTextProcessor()
	return tp.TruncateString(s, maxLength)
}
