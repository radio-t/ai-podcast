// file: main.go
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Host represents a podcast host with name, gender, and character traits
type Host struct {
	Name      string
	Gender    string // "male" or "female"
	Character string // Personality traits and perspective
	Voice     string // OpenAI TTS voice to use
}

// Message represents a single utterance in the discussion
type Message struct {
	Host    string
	Content string
}

// Discussion is the complete podcast discussion
type Discussion struct {
	Title    string
	Messages []Message
}

// Config represents the application configuration
type Config struct {
	Hosts          []Host
	ArticleURL     string
	IcecastURL     string
	IcecastMount   string
	IcecastUser    string
	IcecastPass    string
	OpenAIAPIKey   string
	TargetDuration int    // Target duration in minutes
	DryRun         bool   // Play locally instead of streaming
	OutputFile     string // Output MP3 file path
}

// SpeechSegment represents a generated speech segment with its metadata
type SpeechSegment struct {
	AudioData []byte
	Host      string
	Index     int
	Error     error
	msg       Message
}

func main() {
	// Parse command line flags
	articleURL := flag.String("url", "", "URL of the article to discuss")
	icecastURL := flag.String("icecast", "localhost:8000", "Icecast server URL")
	icecastMount := flag.String("mount", "/podcast.mp3", "Icecast mount point")
	icecastUser := flag.String("user", "source", "Icecast username")
	icecastPass := flag.String("pass", "hackme", "Icecast password")
	apiKey := flag.String("apikey", "", "OpenAI API key")
	targetDuration := flag.Int("duration", 10, "Target podcast duration in minutes")
	dryRun := flag.Bool("dry", false, "Dry run: play locally instead of streaming to Icecast")
	outputFile := flag.String("mp3", "", "Output MP3 file path (optional)")
	flag.Parse()

	if *articleURL == "" {
		log.Fatal("Please provide an article URL with -url")
	}

	if *apiKey == "" {
		// Try to get from environment
		*apiKey = os.Getenv("OPENAI_API_KEY")
		if *apiKey == "" {
			log.Fatal("Please provide an OpenAI API key with -apikey or OPENAI_API_KEY environment variable")
		}
	}

	// Define hosts with Russian names and distinct characters
	hosts := []Host{
		{
			Name:      "Алексей",
			Gender:    "male",
			Character: "Молодой энтузиаст технологий, всегда в курсе последних трендов. Говорит быстро и с энтузиазмом, часто использует современные выражения и технические термины. Любит делиться позитивным опытом и верит в силу инноваций. Иногда может быть немного наивным, но его искренность заразительна.",
			Voice:     "onyx",
		},
		{
			Name:      "Мария",
			Gender:    "female",
			Character: "Опытный экономист с глубоким пониманием технологических трендов. Говорит чётко и структурированно, любит подкреплять свои аргументы данными. Часто задаёт провокационные вопросы и умеет находить неочевидные связи. Сохраняет профессиональный подход, но умеет быть ироничной и остроумной.",
			Voice:     "nova",
		},
		{
			Name:      "Дмитрий",
			Gender:    "male",
			Character: "Скептичный технолог с большим опытом в индустрии. Говорит размеренно, с долей здорового цинизма. Любит приводить примеры из прошлого и предупреждает о потенциальных рисках. Его сухой юмор и ирония часто скрывают глубокое понимание ситуации. Не боится спорить, но всегда готов признать свою неправоту.",
			Voice:     "echo",
		},
	}

	config := Config{
		Hosts:          hosts,
		ArticleURL:     *articleURL,
		IcecastURL:     *icecastURL,
		IcecastMount:   *icecastMount,
		IcecastUser:    *icecastUser,
		IcecastPass:    *icecastPass,
		OpenAIAPIKey:   *apiKey,
		TargetDuration: *targetDuration,
		DryRun:         *dryRun,
		OutputFile:     *outputFile,
	}

	// Run the application
	if err := run(config); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func run(config Config) error {
	// 1. Fetch and extract article text
	articleText, title, err := fetchArticle(config.ArticleURL)
	if err != nil {
		return fmt.Errorf("error fetching article: %w", err)
	}

	fmt.Printf("Successfully fetched article: %s\n", title)

	// 2. Generate discussion using LLM
	fmt.Printf("Generating a %d-minute podcast discussion...\n", config.TargetDuration)
	discussion, err := generateDiscussion(articleText, title, config.Hosts, config.TargetDuration, config.OpenAIAPIKey)
	if err != nil {
		return fmt.Errorf("error generating discussion: %w", err)
	}

	fmt.Printf("Generated discussion with %d messages\n", len(discussion.Messages))

	// 3. Generate speech and stream/play/save
	if config.DryRun || config.OutputFile != "" {
		err = generateAndPlayLocally(discussion, config)
		if err != nil {
			return fmt.Errorf("error playing podcast locally: %w", err)
		}
	} else {
		err = generateAndStreamToIcecast(discussion, config)
		if err != nil {
			return fmt.Errorf("error streaming podcast: %w", err)
		}
	}

	return nil
}

// fetchArticle downloads and extracts text from the given URL
func fetchArticle(url string) (string, string, error) {
	// Fetch the page
	resp, err := http.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("failed to fetch article: status code %d", resp.StatusCode)
	}

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", err
	}

	// Extract title
	title := doc.Find("title").Text()

	// Extract article content - this is a simplified approach
	var articleText strings.Builder

	// First try to find article content in common containers
	article := doc.Find("article, .article, .post, .content, main")
	if article.Length() > 0 {
		article.Find("p").Each(func(i int, s *goquery.Selection) {
			articleText.WriteString(s.Text())
			articleText.WriteString("\n\n")
		})
	} else {
		// Fallback to all paragraphs
		doc.Find("p").Each(func(i int, s *goquery.Selection) {
			// Skip very short paragraphs which are likely not article content
			if len(s.Text()) > 50 {
				articleText.WriteString(s.Text())
				articleText.WriteString("\n\n")
			}
		})
	}

	// Limit article length for API calls
	content := articleText.String()
	if len(content) > 8000 {
		content = content[:8000] + "..."
	}

	return content, title, nil
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

// prepareHostDescriptions formats host information for the prompt
func prepareHostDescriptions(hosts []Host) string {
	var hostDescriptions []string
	for _, host := range hosts {
		hostDescriptions = append(hostDescriptions,
			fmt.Sprintf("%s (%s): %s", host.Name, host.Gender, host.Character))
	}
	return strings.Join(hostDescriptions, "\n")
}

// createDiscussionPrompt creates the system prompt for the discussion
func createDiscussionPrompt(hostDescriptions string, targetMessages int, targetDuration int) string {
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

// callOpenAIAPI makes a request to the OpenAI API and returns the response
func callOpenAIAPI(request OpenAIRequest, apiKey string) (string, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the response
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return result.Choices[0].Message.Content, nil
}

// extractAndParseJSON extracts and parses JSON from the OpenAI response
func extractAndParseJSON(content string) ([]Message, error) {
	// The LLM may wrap the JSON in backticks or code block, so remove those
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
	}
	content = strings.TrimSpace(content)

	// Parse the JSON into our structure
	var rawMessages []struct {
		Host    string `json:"host"`
		Content string `json:"content"`
	}

	err := json.Unmarshal([]byte(content), &rawMessages)
	if err != nil {
		// If unmarshaling fails, try to extract JSON from the text
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

	// Convert parsed messages to our Message type
	messages := make([]Message, len(rawMessages))
	for i, msg := range rawMessages {
		messages[i] = Message{
			Host:    msg.Host,
			Content: msg.Content,
		}
	}

	return messages, nil
}

// generateDiscussion uses OpenAI API to create a discussion between hosts
func generateDiscussion(articleText, title string, hosts []Host, targetDuration int, apiKey string) (Discussion, error) {
	// Prepare host descriptions for the prompt
	hostDescriptions := prepareHostDescriptions(hosts)

	// Calculate target number of messages based on duration
	// Assuming average message length of 30 seconds, we need more messages for longer durations
	targetMessages := targetDuration * 2 // 2 messages per minute

	// Create the system prompt
	systemPrompt := createDiscussionPrompt(hostDescriptions, targetMessages, targetDuration)

	// Prepare the API request
	request := OpenAIRequest{
		Model: "gpt-4o",
		Messages: []OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("Article Title: %s\n\nArticle Content: %s\n\nPlease respond in Russian language only.", title, articleText)},
		},
		Temperature: 0.7,
		MaxTokens:   4000, // Increased token limit to accommodate longer discussions
	}

	// Call the OpenAI API
	content, err := callOpenAIAPI(request, apiKey)
	if err != nil {
		return Discussion{}, err
	}

	// Extract and parse the JSON response
	messages, err := extractAndParseJSON(content)
	if err != nil {
		return Discussion{}, err
	}

	// Create and return the discussion
	discussion := Discussion{
		Title:    title,
		Messages: messages,
	}

	return discussion, nil
}

// estimateAudioDuration estimates the spoken duration of text in seconds
func estimateAudioDuration(text string) float64 {
	// Average Russian reading speed is about 150-170 words per minute
	// We'll use 160 words per minute as our baseline
	// Russian has an average of about 5-6 characters per word

	// Count characters excluding spaces
	charCount := 0
	for _, char := range text {
		if char != ' ' && char != '\n' && char != '\t' && char != '\r' {
			charCount++
		}
	}

	// Estimate word count (characters ÷ 5.5)
	estimatedWords := float64(charCount) / 5.5

	// Calculate duration in seconds (words ÷ 160 × 60)
	durationSeconds := estimatedWords / 160.0 * 60.0

	return durationSeconds
}

// estimateTotalDuration estimates the total duration of all messages
func estimateTotalDuration(messages []Message) float64 {
	var totalDuration float64
	for _, msg := range messages {
		totalDuration += estimateAudioDuration(msg.Content)
	}
	return totalDuration
}

// calculateSpeechSpeed determines the speech speed factor to match target duration
func calculateSpeechSpeed(estimatedDuration float64, targetDurationMinutes int) float64 {
	speechSpeed := 1.0
	if estimatedDuration <= 0 {
		return speechSpeed
	}

	// Target duration in seconds
	targetDurationSeconds := float64(targetDurationMinutes * 60)
	
	// If estimated duration is significantly different from target, adjust speed
	// but keep it within reasonable bounds (0.8-1.2)
	speechSpeed = targetDurationSeconds / estimatedDuration
	return math.Max(0.8, math.Min(1.2, speechSpeed))
}

// generateSpeechSegments generates speech for all messages in the discussion
func generateSpeechSegments(messages []Message, hostMap map[string]struct {
	gender string
	voice  string
}, speechSpeed float64, apiKey string, tempDir string) ([]string, error) {
	audioFiles := make([]string, 0, len(messages))
	
	for i, msg := range messages {
		// Get gender and voice for the host
		gender := "female" // default
		voice := "nova"    // default
		if info, ok := hostMap[msg.Host]; ok {
			gender = info.gender
			voice = info.voice
		}

		fmt.Printf("Generating speech for %s (message %d/%d)...\n",
			msg.Host, i+1, len(messages))

		// Generate speech with OpenAI TTS
		audioData, err := generateOpenAITTS(msg.Content, "ru", gender, voice, speechSpeed, apiKey)
		if err != nil {
			return nil, fmt.Errorf("failed to generate speech for message %d: %w", i, err)
		}

		// Create a file for the audio
		filename := fmt.Sprintf("%s/segment_%03d.mp3", tempDir, i)
		err = os.WriteFile(filename, audioData, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to write audio data: %w", err)
		}
		audioFiles = append(audioFiles, filename)
	}
	
	return audioFiles, nil
}

// streamToIcecast streams concatenated audio files to Icecast server
func streamToIcecast(concatFile string, config Config) error {
	fmt.Printf("Streaming to Icecast server at %s%s...\n", config.IcecastURL, config.IcecastMount)

	// Construct Icecast URL with authentication
	icecastURL := fmt.Sprintf("icecast://%s:%s@%s%s",
		config.IcecastUser, config.IcecastPass, config.IcecastURL, config.IcecastMount)

	// Build the ffmpeg command
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
		"-c", "copy",
		"-content_type", "audio/mpeg",
		icecastURL,
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg streaming failed: %w", err)
	}
	
	return nil
}

// generateAndStreamToIcecast generates speech for each message and streams to Icecast using ffmpeg
func generateAndStreamToIcecast(discussion Discussion, config Config) error {
	// Map host names to their gender and voice
	hostMap := createHostMap(config.Hosts)

	// Estimate total discussion duration
	totalEstimatedDuration := estimateTotalDuration(discussion.Messages)
	fmt.Printf("Estimated podcast duration: %.1f minutes\n", totalEstimatedDuration/60.0)

	// Calculate speech speed
	speechSpeed := calculateSpeechSpeed(totalEstimatedDuration, config.TargetDuration)
	if speechSpeed != 1.0 {
		fmt.Printf("Adjusting speech speed to %.2f to match target duration\n", speechSpeed)
	}

	// Create a temporary directory to store the audio segments
	tempDir, err := os.MkdirTemp("", "podcast")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate speech for all messages
	audioFiles, err := generateSpeechSegments(discussion.Messages, hostMap, speechSpeed, config.OpenAIAPIKey, tempDir)
	if err != nil {
		return err
	}

	// Create concat file for ffmpeg
	concatFile, err := saveToConcatFile(tempDir, audioFiles)
	if err != nil {
		return err
	}

	// Stream to Icecast
	err = streamToIcecast(concatFile, config)
	if err != nil {
		return err
	}

	fmt.Println("Podcast streaming completed successfully!")
	return nil
}

// SpeechGenerationRequest contains all parameters needed for TTS generation
type SpeechGenerationRequest struct {
	msg    Message
	index  int
	gender string
	voice  string
	speed  float64
	apiKey string
}

// createHostMap maps host names to their gender and voice settings
func createHostMap(hosts []Host) map[string]struct {
	gender string
	voice  string
} {
	hostMap := make(map[string]struct {
		gender string
		voice  string
	})
	for _, host := range hosts {
		hostMap[host.Name] = struct {
			gender string
			voice  string
		}{
			gender: host.Gender,
			voice:  host.Voice,
		}
	}
	return hostMap
}

// speechGenerationWorker processes requests from the request channel and sends results to the result channel
func speechGenerationWorker(requestChan <-chan SpeechGenerationRequest, resultChan chan<- SpeechSegment, stopChan <-chan struct{}) {
	for {
		select {
		case <-stopChan:
			fmt.Println("Background worker stopped")
			return
		case req := <-requestChan:
			segmentStartTime := time.Now()
			fmt.Printf("Generating speech for message %d from %s...\n", req.index, req.msg.Host)
			audioData, err := generateOpenAITTS(req.msg.Content, "ru", req.gender, req.voice, req.speed, req.apiKey)
			if err != nil {
				fmt.Printf("Error generating speech for message %d: %v\n", req.index, err)
			} else {
				segmentDuration := time.Since(segmentStartTime)
				fmt.Printf("Successfully generated speech for message %d (took %.1f seconds)\n", req.index, segmentDuration.Seconds())
			}
			resultChan <- SpeechSegment{
				AudioData: audioData,
				Host:      req.msg.Host,
				Index:     req.index,
				Error:     err,
				msg:       req.msg,
			}
		}
	}
}

// createSpeechRequest creates a speech generation request for the given message
func createSpeechRequest(msg Message, index int, hostMap map[string]struct {
	gender string
	voice  string
}, apiKey string) SpeechGenerationRequest {
	gender := "female" // default
	voice := "nova"    // default
	if info, ok := hostMap[msg.Host]; ok {
		gender = info.gender
		voice = info.voice
	}

	return SpeechGenerationRequest{
		msg:    msg,
		index:  index,
		gender: gender,
		voice:  voice,
		speed:  1.0,
		apiKey: apiKey,
	}
}

// saveToConcatFile creates a concatenation file for ffmpeg
func saveToConcatFile(tempDir string, audioFiles []string) (string, error) {
	concatFile := fmt.Sprintf("%s/concat.txt", tempDir)
	var concatContent strings.Builder
	for _, file := range audioFiles {
		concatContent.WriteString(fmt.Sprintf("file '%s'\n", file))
	}
	err := os.WriteFile(concatFile, []byte(concatContent.String()), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write concat file: %w", err)
	}
	return concatFile, nil
}

// concatenateAudioFiles uses ffmpeg to concatenate audio files into a single output file
func concatenateAudioFiles(concatFile string, outputFile string) error {
	fmt.Println("Concatenating audio files...")
	concatStartTime := time.Now()
	
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
		"-c", "copy",
		outputFile,
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to concatenate audio files: %w", err)
	}
	
	concatDuration := time.Since(concatStartTime)
	fmt.Printf("Concatenation completed in %.1f seconds\n", concatDuration.Seconds())
	fmt.Printf("Podcast saved to %s\n", outputFile)
	return nil
}

// generateAndPlayLocally generates speech for each message and plays it locally
func generateAndPlayLocally(discussion Discussion, config Config) error {
	startTime := time.Now()
	fmt.Println("Starting local generation/playback...")

	// Create a temporary directory to store the audio segments
	tempDir, err := os.MkdirTemp("", "podcast")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	fmt.Printf("Created temporary directory: %s\n", tempDir)

	// Map host names to their gender and voice
	hostMap := createHostMap(config.Hosts)

	// Create channels for communication between main thread and background workers
	requestChan := make(chan SpeechGenerationRequest, len(discussion.Messages))
	resultChan := make(chan SpeechSegment, len(discussion.Messages))
	stopChan := make(chan struct{})

	// Create a buffer for pre-generated segments
	bufferSize := 2
	segmentBuffer := make([]SpeechSegment, 0, bufferSize)
	bufferMutex := sync.Mutex{}

	// Start background worker for speech generation
	fmt.Println("Starting background worker for speech generation...")
	go speechGenerationWorker(requestChan, resultChan, stopChan)

	// Start pre-generating segments
	fmt.Println("Starting pre-generation of segments...")
	currentIndex := 0
	for i := 0; i < bufferSize && currentIndex < len(discussion.Messages); i++ {
		msg := discussion.Messages[currentIndex]
		req := createSpeechRequest(msg, currentIndex, hostMap, config.OpenAIAPIKey)
		fmt.Printf("Requesting generation of message %d from %s...\n", currentIndex, msg.Host)
		requestChan <- req
		currentIndex++
	}

	// Process messages and play audio
	playedIndex := 0
	audioFiles := make([]string, 0, len(discussion.Messages))

	fmt.Println("Starting main processing loop...")
	for playedIndex < len(discussion.Messages) {
		// Start generating the next segment if we're not at the end
		if currentIndex < len(discussion.Messages) {
			msg := discussion.Messages[currentIndex]
			req := createSpeechRequest(msg, currentIndex, hostMap, config.OpenAIAPIKey)
			fmt.Printf("Requesting generation of message %d from %s...\n", currentIndex, msg.Host)
			requestChan <- req
			currentIndex++
		}

		// Wait for the next segment with a timeout
		fmt.Printf("Waiting for next segment (played %d/%d)...\n", playedIndex, len(discussion.Messages))
		var segment SpeechSegment
		select {
		case segment = <-resultChan:
			fmt.Printf("Received segment %d from %s\n", segment.Index, segment.Host)
		case <-time.After(30 * time.Second):
			fmt.Println("Timeout waiting for speech generation!")
			close(stopChan)
			return fmt.Errorf("timeout waiting for speech generation")
		}

		if segment.Error != nil {
			fmt.Printf("Error in segment %d: %v\n", segment.Index, segment.Error)
			close(stopChan)
			return fmt.Errorf("failed to generate speech for message %d: %w", segment.Index, segment.Error)
		}

		// Add segment to buffer
		bufferMutex.Lock()
		segmentBuffer = append(segmentBuffer, segment)
		bufferMutex.Unlock()

		// Get the next segment to play
		bufferMutex.Lock()
		if len(segmentBuffer) > 0 {
			nextSegment := segmentBuffer[0]
			segmentBuffer = segmentBuffer[1:]
			bufferMutex.Unlock()

			// Create a temporary file for the audio
			filename := fmt.Sprintf("%s/segment_%03d.mp3", tempDir, playedIndex)
			fmt.Printf("Writing segment %d to file %s...\n", playedIndex, filename)
			err := os.WriteFile(filename, nextSegment.AudioData, 0644)
			if err != nil {
				fmt.Printf("Error writing segment %d: %v\n", playedIndex, err)
				close(stopChan)
				return fmt.Errorf("failed to write audio data: %w", err)
			}
			audioFiles = append(audioFiles, filename)

			// Play the current segment if dry run is enabled
			if config.DryRun {
				playStartTime := time.Now()
				fmt.Printf("\nPlaying audio from %s (message %d/%d)...\n",
					nextSegment.Host, playedIndex+1, len(discussion.Messages))
				fmt.Printf("Text: %s\n", truncateString(nextSegment.msg.Content, 50))

				err = playAudioFile(filename)
				if err != nil {
					fmt.Printf("Error playing segment %d: %v\n", playedIndex, err)
					close(stopChan)
					return fmt.Errorf("failed to play audio: %w", err)
				}
				playDuration := time.Since(playStartTime)
				fmt.Printf("Segment %d playback completed in %.1f seconds\n", playedIndex, playDuration.Seconds())
			}

			playedIndex++
		} else {
			bufferMutex.Unlock()
			fmt.Println("No segments in buffer, waiting...")
			// If no segments in buffer, wait a bit and try again
			time.Sleep(100 * time.Millisecond)
		}
	}

	close(stopChan)
	fmt.Println("Finished processing all segments")

	// If output file is specified, concatenate all segments
	if config.OutputFile != "" {
		fmt.Printf("\nSaving podcast to %s...\n", config.OutputFile)

		concatFile, err := saveToConcatFile(tempDir, audioFiles)
		if err != nil {
			return err
		}

		err = concatenateAudioFiles(concatFile, config.OutputFile)
		if err != nil {
			return err
		}
	}

	totalDuration := time.Since(startTime)
	fmt.Printf("\nTotal processing time: %.1f seconds (%.1f minutes)\n", totalDuration.Seconds(), totalDuration.Minutes())

	if config.DryRun {
		fmt.Println("\nPodcast playback completed successfully!")
	} else {
		fmt.Println("\nPodcast generation completed successfully!")
	}
	return nil
}

// playAudioFile plays an audio file using the system's default audio player
func playAudioFile(filename string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("afplay", filename)
	case "windows":
		cmd = exec.Command("cmd", "/C", "start", filename)
	case "linux":
		// Try several common audio players
		players := []string{"mpv", "mplayer", "ffplay", "aplay"}

		for _, player := range players {
			if _, err := exec.LookPath(player); err == nil {
				if player == "aplay" {
					cmd = exec.Command(player, "-q", filename)
				} else {
					cmd = exec.Command(player, filename, "-nodisp", "-autoexit", "-really-quiet")
				}
				break
			}
		}

		if cmd == nil {
			return fmt.Errorf("no suitable audio player found on your system")
		}
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Run the command and wait for it to finish
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error playing audio: %w", err)
	}

	return nil
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
	case "nova": // Мария - analytical and pragmatic
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
	case "echo": // Дмитрий - skeptical and traditionalist
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

// callOpenAITTSAPI makes the API call to OpenAI for text-to-speech
func callOpenAITTSAPI(request OpenAITTSRequest, apiKey string) ([]byte, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// Create the HTTP request
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Make the API call
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the response
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
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no response from API")
	}

	// Decode base64 audio data
	audioData, err := base64.StdEncoding.DecodeString(result.Choices[0].Message.Audio.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode audio data: %w", err)
	}

	return audioData, nil
}

// generateOpenAITTS uses OpenAI's Chat Completions API to generate natural speech
func generateOpenAITTS(text, lang, gender, voice string, speed float64, apiKey string) ([]byte, error) {
	// Get the appropriate speaking style for this voice
	speakingStyle := getSpeakingStyle(voice)
	
	// Create the system prompt
	systemPrompt := createTTSSystemPrompt(speakingStyle)

	// Prepare the API request
	request := OpenAITTSRequest{
		Model:      "gpt-4o-audio-preview",
		Modalities: []string{"text", "audio"},
		Audio: struct {
			Voice  string `json:"voice"`
			Format string `json:"format"`
		}{
			Voice:  voice,
			Format: "mp3",
		},
		Store: true,
		Messages: []OpenAIMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: text,
			},
		},
	}

	// Call the TTS API and return the audio data
	return callOpenAITTSAPI(request, apiKey)
}

// truncateString truncates a string to the specified length and adds "..." if truncated
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}

	return s[:maxLength] + "..."
}
