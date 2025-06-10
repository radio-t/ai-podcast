package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/radio-t/ai-podcast/podcast"
)

//go:generate moq -out mocks/http_client.go -pkg mocks -skip-ensure -fmt goimports . HTTPClient

// HTTPClient defines the interface for HTTP client operations
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// OpenAIService implements OpenAI API interactions
type OpenAIService struct {
	apiKey     string
	httpClient HTTPClient
}

// NewOpenAIService creates a new OpenAI service
func NewOpenAIService(apiKey string, httpClient HTTPClient) *OpenAIService {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 2 * time.Minute}
	}
	return &OpenAIService{
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

// OpenAIMessage represents a message in the OpenAI API format
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIRequest represents a request to the OpenAI API
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
	MaxTokens   int             `json:"max_tokens"`
}

// OpenAITTSRequest represents the request structure for OpenAI TTS API
type OpenAITTSRequest struct {
	Model      string          `json:"model"`
	Messages   []OpenAIMessage `json:"messages"`
	Modalities []string        `json:"modalities"`
	Audio      struct {
		Voice  string `json:"voice"`
		Format string `json:"format"`
	} `json:"audio"`
	Store bool `json:"store"`
}

// GenerateDiscussion uses OpenAI API to create a discussion between hosts
func (s *OpenAIService) GenerateDiscussion(params podcast.GenerateDiscussionParams) (podcast.Discussion, error) {
	// calculate target number of messages based on duration
	targetMessages := params.TargetDuration * 2 // 2 messages per minute

	// create the system prompt
	systemPrompt := s.createDiscussionPrompt(params.Hosts, targetMessages, params.TargetDuration)

	// prepare the API request
	request := OpenAIRequest{
		Model: "gpt-4o",
		Messages: []OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("Article Title: %s\n\nArticle Content: %s\n\nPlease respond in Russian language only.", params.Title, params.ArticleText)},
		},
		Temperature: 0.7,
		MaxTokens:   4000,
	}

	// call the OpenAI API
	content, err := s.callChatAPI(request)
	if err != nil {
		return podcast.Discussion{}, fmt.Errorf("failed to generate discussion: %w", err)
	}

	// extract and parse the JSON response
	messages, err := s.extractMessages(content)
	if err != nil {
		return podcast.Discussion{}, fmt.Errorf("failed to parse discussion: %w", err)
	}

	return podcast.Discussion{
		Title:    params.Title,
		Messages: messages,
	}, nil
}

// GenerateSpeech generates speech audio for the given text
func (s *OpenAIService) GenerateSpeech(text, voice string) ([]byte, error) {
	// get the appropriate speaking style for this voice
	speakingStyle := getSpeakingStyle(voice)
	systemPrompt := createTTSSystemPrompt(speakingStyle)

	// prepare the API request
	request := OpenAITTSRequest{
		Model:      "gpt-4o-audio-preview",
		Modalities: []string{"text", "audio"},
		Store:      true,
		Messages: []OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: text},
		},
	}
	request.Audio.Voice = voice
	request.Audio.Format = "mp3"

	// call the TTS API
	return s.callTTSAPI(request)
}

// callChatAPI makes a request to the OpenAI chat completions API
func (s *OpenAIService) callChatAPI(request OpenAIRequest) (string, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// parse the response
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return result.Choices[0].Message.Content, nil
}

// callTTSAPI makes a request to the OpenAI TTS API
func (s *OpenAIService) callTTSAPI(request OpenAITTSRequest) ([]byte, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// parse the response
	var result struct {
		Choices []struct {
			Message struct {
				Audio struct {
					Data string `json:"data"`
				} `json:"audio"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode TTS response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no TTS response from API")
	}

	// decode base64 audio data
	audioData, err := base64.StdEncoding.DecodeString(result.Choices[0].Message.Audio.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode audio data: %w", err)
	}

	return audioData, nil
}

// createDiscussionPrompt creates the system prompt for the discussion
func (s *OpenAIService) createDiscussionPrompt(hosts []podcast.Host, targetMessages, targetDuration int) string {
	hostDescriptions := s.prepareHostDescriptions(hosts)

	return fmt.Sprintf(`Generate a heated and passionate tech podcast discussion in Russian language between these hosts about the following article:

%s

The discussion should be natural, sometimes heated, and reflect real human interactions. Hosts should actively engage with each other, frequently interrupt, strongly disagree, and show genuine emotions.

IMPORTANT RULES:
1. Each host's response should be 2-5 sentences long, with varying lengths
2. Hosts should actively disagree and challenge each other's points with strong language
3. Include frequent interruptions and overlapping speech
4. Show strong emotions - frustration, excitement, anger, skepticism
5. Generate approximately %d messages total (about 2 messages per minute for %d minutes)
6. Each host should speak roughly the same number of times
7. Use casual Russian expressions and strong language naturally
8. Include heated debates and passionate arguments
9. Make it feel like a real, unscripted discussion between passionate tech experts who aren't afraid to get heated

Format the output as a JSON array of messages, where each message has a "host" field (the host's name) and a "content" field (what they say in Russian).

The discussion should flow like this:
1. Brief introduction of the article topic (1-2 messages)
2. Main discussion with active engagement, interruptions, and heated disagreements
3. Short summary of key points (1-2 messages)

Start with a brief introduction of the article topic before jumping into the heated discussion. This introduction should be casual and engaging, giving listeners context about what they're about to hear.

Make it feel like a real tech podcast discussion with passionate experts who aren't afraid to get heated and use strong language when they disagree.
`, hostDescriptions, targetMessages, targetDuration)
}

// prepareHostDescriptions formats host information for the prompt
func (s *OpenAIService) prepareHostDescriptions(hosts []podcast.Host) string {
	descriptions := make([]string, 0, len(hosts))
	for _, host := range hosts {
		descriptions = append(descriptions,
			fmt.Sprintf("%s (%s): %s", host.Name, host.Gender, host.Character))
	}
	return strings.Join(descriptions, "\n")
}

// extractMessages extracts and parses messages from the OpenAI response
func (s *OpenAIService) extractMessages(content string) ([]podcast.Message, error) {
	// the LLM may wrap the JSON in backticks or code block, so remove those
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
	}
	content = strings.TrimSpace(content)

	// parse the JSON into our structure
	var rawMessages []struct {
		Host    string `json:"host"`
		Content string `json:"content"`
	}

	err := json.Unmarshal([]byte(content), &rawMessages)
	if err != nil {
		// if unmarshaling fails, try to extract JSON from the text
		startIdx := strings.Index(content, "[")
		endIdx := strings.LastIndex(content, "]")
		if startIdx >= 0 && endIdx > startIdx {
			content = content[startIdx : endIdx+1]
			err = json.Unmarshal([]byte(content), &rawMessages)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to parse response as JSON: %w", err)
		}
	}

	// convert parsed messages to our Message type
	messages := make([]podcast.Message, len(rawMessages))
	for i, msg := range rawMessages {
		messages[i] = podcast.Message{
			Host:    msg.Host,
			Content: msg.Content,
		}
	}

	return messages, nil
}

// getSpeakingStyle returns the appropriate speaking style based on the voice
func getSpeakingStyle(voice string) string {
	switch voice {
	case "onyx": // Алексей - optimistic and open-minded
		return `You are Алексей, a young tech enthusiast who's always excited about new developments. Your speech style is:
- Super energetic and fast-paced, like you're about to burst with excitement
- Use lots of modern tech slang and casual expressions
- Speak with a bright, enthusiastic tone, like you're sharing something amazing
- Use short, punchy sentences with lots of emphasis
- Add dramatic pauses before exciting news
- Use casual interjections like "охуенно!", "заебись!", "пиздец как круто!"
- Show your personality through your voice - be the tech optimist who sees possibilities everywhere
- Use informal Russian expressions and modern tech slang naturally
- Get genuinely excited and sometimes interrupt others with your enthusiasm`
	case "nova": // мария - analytical and pragmatic
		return `You are Мария, an experienced economist with deep tech knowledge. Your speech style is:
- Confident and direct, but still casual and engaging
- Use data points and statistics naturally, but explain them in simple terms
- Speak with authority but keep it conversational
- Use rhetorical questions and challenge others' views
- Add sarcastic humor and witty comebacks
- Include professional insights but explain them in everyday language
- Show your analytical nature but don't be afraid to get passionate
- Use casual expressions and occasional strong language when appropriate
- Get genuinely frustrated when others don't see the obvious`
	case "echo": // дмитрий - skeptical and traditionalist
		return `You are Дмитрий, a seasoned tech professional with a healthy dose of skepticism. Your speech style is:
- Measured but with strong opinions and emotions
- Use dry humor and sarcasm liberally
- Add frequent "ну..." and "хм..." reactions
- Speak with a world-weary but passionate tone
- Use longer, more complex sentences but with casual expressions
- Include historical parallels and cautionary tales
- Add strong skepticism with phrases like "бля, ну тут же очевидно..."
- Show your personality through passionate, sometimes heated responses
- Get genuinely angry when people ignore obvious risks
- Use casual Russian expressions and occasional strong language`
	default:
		return ""
	}
}

// createTTSSystemPrompt creates the system prompt for TTS generation
func createTTSSystemPrompt(speakingStyle string) string {
	return speakingStyle + "\n\nYou are participating in a heated tech podcast discussion. Speak the following text in Russian language with your unique personality and style. Remember to:\n" +
		"- Use very natural, casual conversational style\n" +
		"- Add strong emotions and emphasis\n" +
		"- Include lots of natural reactions and interjections\n" +
		"- Use informal Russian expressions and modern slang\n" +
		"- Speak like you're having a real argument with friends\n" +
		"- Don't be afraid to use strong language when appropriate\n" +
		"- Show genuine emotions - get excited, frustrated, angry\n" +
		"- Interrupt others when you're passionate about something\n" +
		"- Use casual expressions and modern tech slang naturally"
}
