package content

import "time"

// http and network timeouts
const (
	defaultHTTPTimeout      = 30 * time.Second
	OpenAIHTTPTimeout       = 2 * time.Minute
	SpeechGenerationTimeout = 30 * time.Second
)

// content processing limits
const (
	minArticleTextLength    = 100
	maxArticleContentLength = 8000
	DisplayTruncateLength   = 50
)

// openai api parameters
const (
	OpenAITemperature = 0.7
	OpenAIMaxTokens   = 4000
	MessagesPerMinute = 2
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
	PreGeneratedSegmentsBuffer = 2
)
