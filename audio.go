package main

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

// FFmpegAudioProcessor implements audio processing using ffmpeg
type FFmpegAudioProcessor struct{}

// NewFFmpegAudioProcessor creates a new FFmpeg audio processor
func NewFFmpegAudioProcessor() *FFmpegAudioProcessor {
	return &FFmpegAudioProcessor{}
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

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("afplay", filename)
	case "windows":
		cmd = exec.Command("cmd", "/C", "start", filename)
	case "linux":
		// try several common audio players
		players := []string{"mpv", "mplayer", "ffplay", "aplay"}
		for _, player := range players {
			if _, err := exec.LookPath(player); err == nil {
				if player == "aplay" {
					// #nosec G204 -- Player is selected from a whitelist of known audio players
					cmd = exec.Command(player, "-q", filename)
				} else {
					// #nosec G204 -- Player is selected from a whitelist of known audio players
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

// createConcatFile creates a concatenation file for ffmpeg
func createConcatFile(tempDir string, audioFiles []string) (string, error) {
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
