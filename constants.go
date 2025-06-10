package main

import "time"

// http and network timeouts
const (
	defaultHTTPTimeout      = 30 * time.Second
	openAIHTTPTimeout       = 2 * time.Minute
	speechGenerationTimeout = 30 * time.Second
)

// content processing limits
const (
	minArticleTextLength    = 100
	maxArticleContentLength = 8000
	displayTruncateLength   = 50
)

// openai api parameters
const (
	openAITemperature = 0.7
	openAIMaxTokens   = 4000
	messagesPerMinute = 2
)

// text processing constants
const (
	avgCharsPerWordRussian   = 5.5
	avgWordsPerMinuteRussian = 160.0
	minSpeechSpeed           = 0.8
	maxSpeechSpeed           = 1.2
)

// audio processing
const (
	preGeneratedSegmentsBuffer = 2
)
