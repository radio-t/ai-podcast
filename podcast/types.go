package podcast

import "sync"

// Host represents a podcast host with name, gender, and character traits
type Host struct {
	Name      string
	Gender    string // "male" or "female"
	Character string // personality traits and perspective
	Voice     string // openAI TTS voice to use
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
	TargetDuration int    // target duration in minutes
	DryRun         bool   // play locally instead of streaming
	OutputFile     string // output MP3 file path
}

// SpeechSegment represents a generated speech segment with its metadata
type SpeechSegment struct {
	AudioData []byte
	Host      string
	Index     int
	Error     error
	Msg       Message
}

// SpeechGenerationRequest contains all parameters needed for TTS generation
type SpeechGenerationRequest struct {
	Msg    Message
	Index  int
	Gender string
	Voice  string
	Speed  float64
	APIKey string
}

// ProcessSegmentsParams contains parameters for processSegments function
type ProcessSegmentsParams struct {
	Discussion    Discussion
	Config        Config
	RequestChan   chan<- SpeechGenerationRequest
	ResultChan    <-chan SpeechSegment
	StopChan      <-chan struct{}
	SegmentBuffer *[]SpeechSegment
	BufferMutex   *sync.Mutex
	CurrentIndex  *int
	TempDir       string
}

// ProcessOrderedSegmentParams contains parameters for processOrderedSegment function
type ProcessOrderedSegmentParams struct {
	SegmentBuffer *[]SpeechSegment
	BufferMutex   *sync.Mutex
	PlayedIndex   int
	TempDir       string
	Config        Config
}

// GenerateAndStreamParams contains parameters for generateAndStreamToIcecast and generateAndPlayLocally
type GenerateAndStreamParams struct {
	Discussion Discussion
	Config     Config
}

// GenerateSpeechSegmentsParams contains parameters for generateSpeechSegments
type GenerateSpeechSegmentsParams struct {
	Messages []Message
	HostMap  map[string]HostInfo
	TempDir  string
}

// SpeechGenerationWorkerParams contains parameters for speechGenerationWorker
type SpeechGenerationWorkerParams struct {
	RequestChan <-chan SpeechGenerationRequest
	ResultChan  chan<- SpeechSegment
	StopChan    <-chan struct{}
}

// PlaySegmentParams contains parameters for playSegment
type PlaySegmentParams struct {
	Segment  SpeechSegment
	Index    int
	Filename string
}

// CreateSpeechRequestParams contains parameters for createSpeechRequest
type CreateSpeechRequestParams struct {
	Msg     Message
	Index   int
	HostMap map[string]HostInfo
	APIKey  string
}

// GenerateDiscussionParams contains parameters for GenerateDiscussion
type GenerateDiscussionParams struct {
	ArticleText    string
	Title          string
	Hosts          []Host
	TargetDuration int
}

// HostInfo contains gender and voice information for a host
type HostInfo struct {
	Gender string
	Voice  string
}

// CreateHostMap maps host names to their gender and voice settings
func CreateHostMap(hosts []Host) map[string]HostInfo {
	hostMap := make(map[string]HostInfo)
	for _, host := range hosts {
		hostMap[host.Name] = HostInfo{
			Gender: host.Gender,
			Voice:  host.Voice,
		}
	}
	return hostMap
}
