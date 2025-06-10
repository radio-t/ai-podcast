package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/radio-t/ai-podcast/podcast"
)

//go:generate moq -out mocks/article_fetcher.go -pkg mocks -skip-ensure -fmt goimports -stub . ArticleFetcher
//go:generate moq -out mocks/openai_client.go -pkg mocks -skip-ensure -fmt goimports -stub . OpenAIClient
//go:generate moq -out mocks/audio_processor.go -pkg mocks -skip-ensure -fmt goimports -stub . AudioProcessor

// ArticleFetcher defines the interface for fetching articles (consumer side)
type ArticleFetcher interface {
	Fetch(url string) (content, title string, err error)
}

// OpenAIClient defines the interface for OpenAI API interactions (consumer side)
type OpenAIClient interface {
	GenerateDiscussion(params podcast.GenerateDiscussionParams) (podcast.Discussion, error)
	GenerateSpeech(text, voice string) ([]byte, error)
}

// AudioProcessor defines the interface for audio processing operations (consumer side)
type AudioProcessor interface {
	Play(filename string) error
	Concatenate(files []string, outputFile string) error
	StreamToIcecast(inputFile string, config podcast.Config) error
	StreamFromConcat(concatFile string, config podcast.Config) error
}

func main() {
	// parse command line flags
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
		// try to get from environment
		*apiKey = os.Getenv("OPENAI_API_KEY")
		if *apiKey == "" {
			log.Fatal("Please provide an OpenAI API key with -apikey or OPENAI_API_KEY environment variable")
		}
	}

	// define hosts with Russian names and distinct characters
	hosts := []podcast.Host{
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

	config := podcast.Config{
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

	// run the application
	if err := run(config); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func run(config podcast.Config) error {
	// create services
	articleFetcher := NewHTTPArticleFetcher(nil)
	openAI := NewOpenAIService(config.OpenAIAPIKey, nil)
	audioProcessor := NewFFmpegAudioProcessor()

	return runWithDependencies(config, articleFetcher, openAI, audioProcessor)
}

func runWithDependencies(config podcast.Config, articleFetcher ArticleFetcher, openAI OpenAIClient, audioProcessor AudioProcessor) error {
	// 1. Fetch and extract article text
	articleText, title, err := articleFetcher.Fetch(config.ArticleURL)
	if err != nil {
		return fmt.Errorf("error fetching article: %w", err)
	}

	fmt.Printf("Successfully fetched article: %s\n", title)

	// 2. Generate discussion using LLM
	fmt.Printf("Generating a %d-minute podcast discussion...\n", config.TargetDuration)
	discussionParams := podcast.GenerateDiscussionParams{
		ArticleText:    articleText,
		Title:          title,
		Hosts:          config.Hosts,
		TargetDuration: config.TargetDuration,
	}
	discussion, err := openAI.GenerateDiscussion(discussionParams)
	if err != nil {
		return fmt.Errorf("error generating discussion: %w", err)
	}

	fmt.Printf("Generated discussion with %d messages\n", len(discussion.Messages))

	// 3. Generate speech and stream/play/save
	generateParams := podcast.GenerateAndStreamParams{
		Discussion: discussion,
		Config:     config,
	}
	if config.DryRun || config.OutputFile != "" {
		err = generateAndPlayLocally(generateParams, openAI, audioProcessor)
		if err != nil {
			return fmt.Errorf("error playing podcast locally: %w", err)
		}
	} else {
		err = generateAndStreamToIcecast(generateParams, openAI, audioProcessor)
		if err != nil {
			return fmt.Errorf("error streaming podcast: %w", err)
		}
	}

	return nil
}

// generateAndStreamToIcecast generates speech for each message and streams to Icecast
func generateAndStreamToIcecast(params podcast.GenerateAndStreamParams, openAI OpenAIClient, audioProcessor AudioProcessor) error {
	// create text processor
	textProcessor := NewTextProcessor()

	// map host names to their gender and voice
	hostMap := podcast.CreateHostMap(params.Config.Hosts)

	// estimate total discussion duration
	totalEstimatedDuration := textProcessor.EstimateTotalDuration(params.Discussion.Messages)
	fmt.Printf("Estimated podcast duration: %.1f minutes\n", totalEstimatedDuration/60.0)

	// calculate speech speed
	speechSpeed := textProcessor.CalculateSpeechSpeed(totalEstimatedDuration, params.Config.TargetDuration)
	if speechSpeed != 1.0 {
		fmt.Printf("Adjusting speech speed to %.2f to match target duration\n", speechSpeed)
	}

	// create a temporary directory to store the audio segments
	tempDir, err := os.MkdirTemp("", "podcast")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// generate speech for all messages
	segmentsParams := podcast.GenerateSpeechSegmentsParams{
		Messages: params.Discussion.Messages,
		HostMap:  hostMap,
		TempDir:  tempDir,
	}
	audioFiles, err := generateSpeechSegments(segmentsParams, openAI)
	if err != nil {
		return err
	}

	// create concat file for ffmpeg
	concatFile, err := saveToConcatFile(tempDir, audioFiles)
	if err != nil {
		return err
	}

	// stream to Icecast
	fmt.Printf("Streaming to Icecast server at %s%s...\n", params.Config.IcecastURL, params.Config.IcecastMount)
	err = audioProcessor.StreamFromConcat(concatFile, params.Config)
	if err != nil {
		return err
	}

	fmt.Println("Podcast streaming completed successfully!")
	return nil
}

// generateSpeechSegments generates speech for all messages in the discussion
func generateSpeechSegments(params podcast.GenerateSpeechSegmentsParams, openAI OpenAIClient) ([]string, error) {
	audioFiles := make([]string, 0, len(params.Messages))

	for i, msg := range params.Messages {
		// get voice for the host
		voice := "nova" // default
		if info, ok := params.HostMap[msg.Host]; ok {
			voice = info.Voice
		}

		fmt.Printf("Generating speech for %s (message %d/%d)...\n",
			msg.Host, i+1, len(params.Messages))

		// generate speech with OpenAI TTS
		audioData, err := openAI.GenerateSpeech(msg.Content, voice)
		if err != nil {
			return nil, fmt.Errorf("failed to generate speech for message %d: %w", i, err)
		}

		// create a file for the audio
		filename := fmt.Sprintf("%s/segment_%03d.mp3", params.TempDir, i)
		err = os.WriteFile(filename, audioData, 0o600)
		if err != nil {
			return nil, fmt.Errorf("failed to write audio data: %w", err)
		}
		audioFiles = append(audioFiles, filename)
	}

	return audioFiles, nil
}

// generateAndPlayLocally generates speech for each message and plays it locally
func generateAndPlayLocally(params podcast.GenerateAndStreamParams, openAI OpenAIClient, audioProcessor AudioProcessor) error {
	startTime := time.Now()
	fmt.Println("Starting local generation/playback...")

	// create a temporary directory to store the audio segments
	tempDir, err := os.MkdirTemp("", "podcast")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	fmt.Printf("Created temporary directory: %s\n", tempDir)

	// map host names to their gender and voice
	hostMap := podcast.CreateHostMap(params.Config.Hosts)

	// create channels for communication between main thread and background workers
	requestChan := make(chan podcast.SpeechGenerationRequest, len(params.Discussion.Messages))
	resultChan := make(chan podcast.SpeechSegment, len(params.Discussion.Messages))
	stopChan := make(chan struct{})

	// create a buffer for pre-generated segments
	bufferSize := 2
	segmentBuffer := make([]podcast.SpeechSegment, 0, bufferSize)
	bufferMutex := sync.Mutex{}

	// start background worker for speech generation
	fmt.Println("Starting background worker for speech generation...")
	workerParams := podcast.SpeechGenerationWorkerParams{
		RequestChan: requestChan,
		ResultChan:  resultChan,
		StopChan:    stopChan,
	}
	go speechGenerationWorker(workerParams, openAI)

	// start pre-generating segments
	fmt.Println("Starting pre-generation of segments...")
	currentIndex := 0
	for i := 0; i < bufferSize && currentIndex < len(params.Discussion.Messages); i++ {
		msg := params.Discussion.Messages[currentIndex]
		reqParams := podcast.CreateSpeechRequestParams{
			Msg:     msg,
			Index:   currentIndex,
			HostMap: hostMap,
			APIKey:  params.Config.OpenAIAPIKey,
		}
		req := createSpeechRequest(reqParams)
		fmt.Printf("Requesting generation of message %d from %s...\n", currentIndex, msg.Host)
		requestChan <- req
		currentIndex++
	}

	// process messages and play audio
	processParams := podcast.ProcessSegmentsParams{
		Discussion:    params.Discussion,
		Config:        params.Config,
		RequestChan:   requestChan,
		ResultChan:    resultChan,
		StopChan:      stopChan,
		SegmentBuffer: &segmentBuffer,
		BufferMutex:   &bufferMutex,
		CurrentIndex:  &currentIndex,
		TempDir:       tempDir,
	}
	audioFiles, err := processSegments(processParams, audioProcessor)
	if err != nil {
		close(stopChan)
		return err
	}

	close(stopChan)
	fmt.Println("Finished processing all segments")

	// if output file is specified, concatenate all segments
	if params.Config.OutputFile != "" {
		fmt.Printf("\nSaving podcast to %s...\n", params.Config.OutputFile)
		err = audioProcessor.Concatenate(audioFiles, params.Config.OutputFile)
		if err != nil {
			return err
		}
		fmt.Printf("Podcast saved to %s\n", params.Config.OutputFile)
	}

	totalDuration := time.Since(startTime)
	fmt.Printf("\nTotal processing time: %.1f seconds (%.1f minutes)\n", totalDuration.Seconds(), totalDuration.Minutes())

	if params.Config.DryRun {
		fmt.Println("\nPodcast playback completed successfully!")
	} else {
		fmt.Println("\nPodcast generation completed successfully!")
	}
	return nil
}

// speechGenerationWorker processes requests from the request channel and sends results to the result channel
func speechGenerationWorker(params podcast.SpeechGenerationWorkerParams, openAI OpenAIClient) {
	for {
		select {
		case <-params.StopChan:
			fmt.Println("Background worker stopped")
			return
		case req := <-params.RequestChan:
			segmentStartTime := time.Now()
			fmt.Printf("Generating speech for message %d from %s...\n", req.Index, req.Msg.Host)
			audioData, err := openAI.GenerateSpeech(req.Msg.Content, req.Voice)
			if err != nil {
				fmt.Printf("Error generating speech for message %d: %v\n", req.Index, err)
			} else {
				segmentDuration := time.Since(segmentStartTime)
				fmt.Printf("Successfully generated speech for message %d (took %.1f seconds)\n", req.Index, segmentDuration.Seconds())
			}
			params.ResultChan <- podcast.SpeechSegment{
				AudioData: audioData,
				Host:      req.Msg.Host,
				Index:     req.Index,
				Error:     err,
				Msg:       req.Msg,
			}
		}
	}
}

// processSegments handles the main loop of processing speech segments
func processSegments(params podcast.ProcessSegmentsParams, audioProcessor AudioProcessor) ([]string, error) {

	playedIndex := 0
	audioFiles := make([]string, 0, len(params.Discussion.Messages))
	hostMap := podcast.CreateHostMap(params.Config.Hosts)

	fmt.Println("Starting main processing loop...")
	for playedIndex < len(params.Discussion.Messages) {
		// start generating the next segment if we're not at the end
		if *params.CurrentIndex < len(params.Discussion.Messages) {
			msg := params.Discussion.Messages[*params.CurrentIndex]
			reqParams := podcast.CreateSpeechRequestParams{
				Msg:     msg,
				Index:   *params.CurrentIndex,
				HostMap: hostMap,
				APIKey:  params.Config.OpenAIAPIKey,
			}
			req := createSpeechRequest(reqParams)
			fmt.Printf("Requesting generation of message %d from %s...\n", *params.CurrentIndex, msg.Host)
			params.RequestChan <- req
			*params.CurrentIndex++
		}

		// wait for the next segment with a timeout
		fmt.Printf("Waiting for next segment (played %d/%d)...\n", playedIndex, len(params.Discussion.Messages))
		var segment podcast.SpeechSegment
		select {
		case segment = <-params.ResultChan:
			fmt.Printf("Received segment %d from %s\n", segment.Index, segment.Host)
		case <-time.After(30 * time.Second):
			fmt.Println("Timeout waiting for speech generation!")
			return nil, fmt.Errorf("timeout waiting for speech generation")
		}

		if segment.Error != nil {
			fmt.Printf("Error in segment %d: %v\n", segment.Index, segment.Error)
			return nil, fmt.Errorf("failed to generate speech for message %d: %w", segment.Index, segment.Error)
		}

		// add segment to buffer
		params.BufferMutex.Lock()
		*params.SegmentBuffer = append(*params.SegmentBuffer, segment)
		params.BufferMutex.Unlock()

		// process segments in order
		orderedParams := podcast.ProcessOrderedSegmentParams{
			SegmentBuffer: params.SegmentBuffer,
			BufferMutex:   params.BufferMutex,
			PlayedIndex:   playedIndex,
			TempDir:       params.TempDir,
			Config:        params.Config,
		}
		processedSegment, err := processOrderedSegment(orderedParams, audioProcessor)
		if err != nil {
			return nil, err
		}

		if processedSegment != nil {
			audioFiles = append(audioFiles, *processedSegment)
			playedIndex++
		}
	}

	return audioFiles, nil
}

// processOrderedSegment processes a segment in the correct order
func processOrderedSegment(params podcast.ProcessOrderedSegmentParams, audioProcessor AudioProcessor) (*string, error) {

	params.BufferMutex.Lock()
	foundIndex := -1
	for i, seg := range *params.SegmentBuffer {
		if seg.Index == params.PlayedIndex {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		params.BufferMutex.Unlock()
		return nil, nil // wait for the right segment
	}

	// remove the segment from buffer
	nextSegment := (*params.SegmentBuffer)[foundIndex]
	*params.SegmentBuffer = append((*params.SegmentBuffer)[:foundIndex], (*params.SegmentBuffer)[foundIndex+1:]...)
	params.BufferMutex.Unlock()

	// create a temporary file for the audio
	filename := fmt.Sprintf("%s/segment_%03d.mp3", params.TempDir, params.PlayedIndex)
	fmt.Printf("Writing segment %d to file %s...\n", params.PlayedIndex, filename)
	err := os.WriteFile(filename, nextSegment.AudioData, 0o600)
	if err != nil {
		fmt.Printf("Error writing segment %d: %v\n", params.PlayedIndex, err)
		return nil, fmt.Errorf("failed to write audio data: %w", err)
	}

	// play the current segment if dry run is enabled
	if params.Config.DryRun {
		playParams := podcast.PlaySegmentParams{
			Segment:  nextSegment,
			Index:    params.PlayedIndex,
			Filename: filename,
		}
		if err := playSegment(playParams, audioProcessor); err != nil {
			return nil, err
		}
	}

	return &filename, nil
}

// playSegment plays a single audio segment
func playSegment(params podcast.PlaySegmentParams, audioProcessor AudioProcessor) error {
	playStartTime := time.Now()
	fmt.Printf("\nPlaying audio from %s (message %d)...\n", params.Segment.Host, params.Index+1)
	fmt.Printf("Text: %s\n", truncateString(params.Segment.Msg.Content, 50))

	err := audioProcessor.Play(params.Filename)
	if err != nil {
		fmt.Printf("Error playing segment %d: %v\n", params.Index, err)
		return fmt.Errorf("failed to play audio: %w", err)
	}

	playDuration := time.Since(playStartTime)
	fmt.Printf("Segment %d playback completed in %.1f seconds\n", params.Index, playDuration.Seconds())
	return nil
}

// createSpeechRequest creates a speech generation request for the given message
func createSpeechRequest(params podcast.CreateSpeechRequestParams) podcast.SpeechGenerationRequest {
	gender := "female" // default
	voice := "nova"    // default
	if info, ok := params.HostMap[params.Msg.Host]; ok {
		gender = info.Gender
		voice = info.Voice
	}

	return podcast.SpeechGenerationRequest{
		Msg:    params.Msg,
		Index:  params.Index,
		Gender: gender,
		Voice:  voice,
		Speed:  1.0,
		APIKey: params.APIKey,
	}
}
