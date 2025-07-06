package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/radio-t/ai-podcast/internal/content"
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
		httpClient = &http.Client{Timeout: content.OpenAIHTTPTimeout}
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
	targetMessages := params.TargetDuration * content.MessagesPerMinute

	// create the system prompt
	systemPrompt := s.createDiscussionPrompt(params.Hosts, targetMessages, params.TargetDuration)

	// prepare the API request
	request := OpenAIRequest{
		Model: "gpt-4o",
		Messages: []OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{
				Role: "user",
				Content: fmt.Sprintf("Article Title: %s\n\nArticle Content: %s\n\nPlease respond in Russian language only.",
					params.Title, params.ArticleText),
			},
		},
		Temperature: content.OpenAITemperature,
		MaxTokens:   content.OpenAIMaxTokens,
	}

	// call the OpenAI API
	responseContent, err := s.callChatAPI(request)
	if err != nil {
		return podcast.Discussion{}, fmt.Errorf("failed to generate discussion: %w", err)
	}

	// extract and parse the JSON response
	messages, err := s.extractMessages(responseContent)
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
func (s *OpenAIService) createDiscussionPrompt(hosts []podcast.Host, _, targetDuration int) string {
	hostDescriptions := s.prepareHostDescriptions(hosts)

	basePrompt := `You are hosting a Russian tech podcast discussion about this article. The hosts are:

%s

Have a genuine, unscripted conversation about the article. Don't follow any rigid structure - just talk naturally like real people do. Get passionate about things you care about, interrupt each other when excited, disagree when you actually disagree.

Write it as simple dialog format:
Имя: что говорит
Имя: ответ

Just let the conversation flow naturally for about %d minutes worth of talking.`

	return fmt.Sprintf(basePrompt, hostDescriptions, targetDuration)
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
func (s *OpenAIService) extractMessages(responseContent string) ([]podcast.Message, error) {
	lines := strings.Split(strings.TrimSpace(responseContent), "\n")
	var messages []podcast.Message

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// parse format: "Name: content" or "Name : content"
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		host := strings.TrimSpace(parts[0])
		msgContent := strings.TrimSpace(parts[1])

		if host != "" && msgContent != "" {
			messages = append(messages, podcast.Message{
				Host:    host,
				Content: msgContent,
			})
		}
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no valid dialog lines found in response")
	}

	return messages, nil
}

// getSpeakingStyle returns the appropriate speaking style based on the voice
func getSpeakingStyle(voice string) string {
	switch voice {
	case "onyx": // алексей
		return "молодой техно-оптимист"
	case "nova": // мария
		return "аналитик, любит данные"
	case "echo": // дмитрий
		return "скептик, видел всякое"
	default:
		return ""
	}
}

// createTTSSystemPrompt creates the system prompt for TTS generation
func createTTSSystemPrompt(speakingStyle string) string {
	return fmt.Sprintf("Ты %s в подкасте о технологиях. Говори естественно по-русски, как обычный человек.", speakingStyle)
}
