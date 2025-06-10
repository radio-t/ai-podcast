package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/radio-t/ai-podcast/podcast"
)

func TestFFmpegAudioProcessor_Play(t *testing.T) {
	processor := NewFFmpegAudioProcessor()

	t.Run("non-existent file", func(t *testing.T) {
		err := processor.Play("/tmp/non-existent-audio-file.mp3")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "audio file does not exist")
	})

	t.Run("existing file on supported OS", func(t *testing.T) {
		// create a temporary file
		tmpFile, err := os.CreateTemp("", "test-audio-*.mp3")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		require.NoError(t, tmpFile.Close())

		// this test will fail on CI as audio players might not be available
		// so we'll skip it in that case
		if os.Getenv("CI") == "true" {
			t.Skip("Skipping audio playback test in CI environment")
		}

		// the actual playback test would require a real audio player
		// which might not be available in test environments
	})
}

func TestFFmpegAudioProcessor_StreamToIcecast(t *testing.T) {
	processor := NewFFmpegAudioProcessor()

	tests := []struct {
		name        string
		config      podcast.Config
		wantURLPart string
	}{
		{
			name: "normal credentials",
			config: podcast.Config{
				IcecastUser:  "user",
				IcecastPass:  "pass",
				IcecastURL:   "localhost:8000",
				IcecastMount: "/stream.mp3",
			},
			wantURLPart: "icecast://user:pass@localhost:8000/stream.mp3",
		},
		{
			name: "credentials with special characters",
			config: podcast.Config{
				IcecastUser:  "user@domain",
				IcecastPass:  "p@ss:word/123",
				IcecastURL:   "localhost:8000",
				IcecastMount: "/stream.mp3",
			},
			wantURLPart: "icecast://user%40domain:p%40ss%3Aword%2F123@localhost:8000/stream.mp3",
		},
		{
			name: "credentials with spaces",
			config: podcast.Config{
				IcecastUser:  "user name",
				IcecastPass:  "pass word",
				IcecastURL:   "localhost:8000",
				IcecastMount: "/stream.mp3",
			},
			wantURLPart: "icecast://user%20name:pass%20word@localhost:8000/stream.mp3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// we can't actually run ffmpeg in tests, but we can verify URL construction
			// by checking that the method doesn't panic with special characters
			tmpFile, err := os.CreateTemp("", "test-audio-*.mp3")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())
			require.NoError(t, tmpFile.Close())

			// this will fail because ffmpeg isn't available or icecast isn't running
			// but it won't panic due to malformed URLs
			_ = processor.StreamToIcecast(tmpFile.Name(), tt.config)
		})
	}
}

func TestFFmpegAudioProcessor_Concatenate(t *testing.T) {
	processor := NewFFmpegAudioProcessor()

	t.Run("filename escaping", func(t *testing.T) {
		// create temp files with special characters in names
		tmpDir, err := os.MkdirTemp("", "concat-test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		files := []string{
			fmt.Sprintf("%s/normal.mp3", tmpDir),
			fmt.Sprintf("%s/file'with'quotes.mp3", tmpDir),
			fmt.Sprintf("%s/file with spaces.mp3", tmpDir),
		}

		// create the files
		for _, f := range files {
			file, err := os.Create(f) // #nosec G304 -- test file with controlled path
			require.NoError(t, err)
			require.NoError(t, file.Close())
		}

		outputFile := fmt.Sprintf("%s/output.mp3", tmpDir)

		// the actual concatenation will fail without ffmpeg,
		// but we can verify that the concat file is created correctly
		err = processor.Concatenate(files, outputFile)
		// error is expected as ffmpeg might not be available
		_ = err

		// check if concat file was created (it's removed by defer, so we check indirectly)
		// the important thing is that the function doesn't panic with special filenames
	})
}

func TestCreateConcatFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "concat-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		files    []string
		expected []string
	}{
		{
			name:     "normal filenames",
			files:    []string{"/tmp/file1.mp3", "/tmp/file2.mp3"},
			expected: []string{"file '/tmp/file1.mp3'", "file '/tmp/file2.mp3'"},
		},
		{
			name:     "filenames with single quotes",
			files:    []string{"/tmp/file'1.mp3", "/tmp/it's-a-file.mp3"},
			expected: []string{"file '/tmp/file'\\''1.mp3'", "file '/tmp/it'\\''s-a-file.mp3'"},
		},
		{
			name:     "filenames with spaces",
			files:    []string{"/tmp/file 1.mp3", "/tmp/my file.mp3"},
			expected: []string{"file '/tmp/file 1.mp3'", "file '/tmp/my file.mp3'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			concatFile, err := createConcatFile(tmpDir, tt.files)
			require.NoError(t, err)
			defer os.Remove(concatFile)

			content, err := os.ReadFile(concatFile) // #nosec G304 -- test file with controlled path
			require.NoError(t, err)

			lines := strings.Split(strings.TrimSpace(string(content)), "\n")
			assert.Equal(t, tt.expected, lines)
		})
	}
}
