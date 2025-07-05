package audio

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/radio-t/ai-podcast/podcast"
)

//go:generate moq -out mocks/command_runner.go -pkg mocks -skip-ensure -fmt goimports . CommandRunner

// CommandRunner provides OS-specific command creation for audio playback
type CommandRunner interface {
	GetAudioCommand(filename string) (*exec.Cmd, error)
}

// FFmpegAudioProcessor implements audio processing using ffmpeg
type FFmpegAudioProcessor struct {
	cmdRunner CommandRunner
}

// NewFFmpegAudioProcessor creates a new FFmpeg audio processor
func NewFFmpegAudioProcessor() *FFmpegAudioProcessor {
	return &FFmpegAudioProcessor{
		cmdRunner: &DefaultCommandRunner{},
	}
}

// Play plays an audio file using the system's default audio player
func (p *FFmpegAudioProcessor) Play(filename string) error {
	// check if file exists before attempting to play
	if _, err := os.Stat(filename); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("audio file does not exist: %s", filename)
		}
		return fmt.Errorf("failed to check audio file: %w", err)
	}

	cmd, err := p.cmdRunner.GetAudioCommand(filename)
	if err != nil {
		return fmt.Errorf("failed to get audio command: %w", err)
	}

	// run the command and wait for it to finish
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error playing audio: %w", err)
	}

	return nil
}

// Concatenate uses ffmpeg to concatenate audio files into a single output file
func (p *FFmpegAudioProcessor) Concatenate(files []string, outputFile string) error {
	// create a temporary concat file
	tempDir := os.TempDir()
	concatFile := fmt.Sprintf("%s/concat_%d.txt", tempDir, time.Now().Unix())
	defer os.Remove(concatFile)

	// write the concat file
	var concatContent strings.Builder
	for _, file := range files {
		// escape single quotes in filenames for ffmpeg concat format
		safeFile := strings.ReplaceAll(file, "'", "'\\''")
		concatContent.WriteString(fmt.Sprintf("file '%s'\n", safeFile))
	}
	if err := os.WriteFile(concatFile, []byte(concatContent.String()), 0o600); err != nil {
		return fmt.Errorf("failed to write concat file: %w", err)
	}

	// run ffmpeg to concatenate
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
		"-c", "copy",
		outputFile,
	}

	// #nosec G204 -- Arguments are constructed internally, not from external input
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to concatenate audio files: %w", err)
	}

	return nil
}

// StreamToIcecast streams audio to an Icecast server
func (p *FFmpegAudioProcessor) StreamToIcecast(inputFile string, config podcast.Config) error {
	// construct Icecast URL with authentication
	u := url.URL{
		Scheme: "icecast",
		User:   url.UserPassword(config.IcecastUser, config.IcecastPass),
		Host:   config.IcecastURL,
		Path:   config.IcecastMount,
	}
	icecastURL := u.String()

	// build the ffmpeg command
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-re", // read input at native frame rate
		"-i", inputFile,
		"-c", "copy",
		"-content_type", "audio/mpeg",
		icecastURL,
	}

	// #nosec G204 -- Arguments are constructed internally, not from external input
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg streaming failed: %w", err)
	}

	return nil
}

// StreamFromConcat streams audio files listed in a concat file to Icecast
func (p *FFmpegAudioProcessor) StreamFromConcat(concatFile string, config podcast.Config) error {
	// construct Icecast URL with authentication
	u := url.URL{
		Scheme: "icecast",
		User:   url.UserPassword(config.IcecastUser, config.IcecastPass),
		Host:   config.IcecastURL,
		Path:   config.IcecastMount,
	}
	icecastURL := u.String()

	// build the ffmpeg command
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

	// #nosec G204 -- Arguments are constructed internally, not from external input
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg streaming failed: %w", err)
	}

	return nil
}

// CreateConcatFile creates a concatenation file for ffmpeg
func CreateConcatFile(tempDir string, audioFiles []string) (string, error) {
	concatFile := fmt.Sprintf("%s/concat.txt", tempDir)
	var concatContent strings.Builder
	for _, file := range audioFiles {
		// escape single quotes in filenames for ffmpeg concat format
		safeFile := strings.ReplaceAll(file, "'", "'\\''")
		concatContent.WriteString(fmt.Sprintf("file '%s'\n", safeFile))
	}
	err := os.WriteFile(concatFile, []byte(concatContent.String()), 0o600)
	if err != nil {
		return "", fmt.Errorf("failed to write concat file: %w", err)
	}
	return concatFile, nil
}

// DefaultCommandRunner is the default implementation of CommandRunner
type DefaultCommandRunner struct{}

// GetAudioCommand returns the appropriate audio command for the current OS
func (r *DefaultCommandRunner) GetAudioCommand(filename string) (*exec.Cmd, error) {
	// validate filename to prevent potential security issues
	if strings.Contains(filename, "..") || strings.ContainsAny(filename, ";|&$`") {
		return nil, fmt.Errorf("invalid filename: potential security risk")
	}

	switch runtime.GOOS {
	case "darwin": // macOS
		return exec.Command("afplay", filename), nil
	case "windows":
		return exec.Command("cmd", "/C", "start", filename), nil
	case "linux":
		// try several common audio players
		players := []string{"mpv", "mplayer", "ffplay", "aplay"}
		for _, player := range players {
			if _, err := exec.LookPath(player); err == nil {
				if player == "aplay" {
					// #nosec G204 -- Player is selected from a whitelist of known audio players
					return exec.Command(player, "-q", filename), nil
				}
				// #nosec G204 -- Player is selected from a whitelist of known audio players
				// note: options must come before filename for mpv/mplayer/ffplay
				return exec.Command(player, "-nodisp", "-autoexit", "-really-quiet", filename), nil
			}
		}
		return nil, fmt.Errorf("no suitable audio player found on your system")
	default:
		return nil, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}
