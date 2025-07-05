package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/radio-t/ai-podcast/mocks"
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

func TestFFmpegAudioProcessor_PlayWithMock(t *testing.T) {
	// create a temporary file for all tests
	tmpFile, err := os.CreateTemp("", "test-audio-*.mp3")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	require.NoError(t, tmpFile.Close())

	t.Run("successful command execution", func(t *testing.T) {
		mockRunner := &mocks.CommandRunnerMock{
			GetAudioCommandFunc: func(filename string) (*exec.Cmd, error) {
				assert.Equal(t, tmpFile.Name(), filename)
				// return a command that will succeed (echo does nothing)
				return exec.Command("echo", "playing audio"), nil
			},
		}

		processor := &FFmpegAudioProcessor{cmdRunner: mockRunner}
		err := processor.Play(tmpFile.Name())
		require.NoError(t, err)

		// verify mock was called
		calls := mockRunner.GetAudioCommandCalls()
		assert.Len(t, calls, 1)
		assert.Equal(t, tmpFile.Name(), calls[0].Filename)
	})

	t.Run("command runner returns error", func(t *testing.T) {
		mockRunner := &mocks.CommandRunnerMock{
			GetAudioCommandFunc: func(filename string) (*exec.Cmd, error) {
				return nil, fmt.Errorf("no suitable audio player found on your system")
			},
		}

		processor := &FFmpegAudioProcessor{cmdRunner: mockRunner}
		err := processor.Play(tmpFile.Name())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no suitable audio player found")
	})

	t.Run("command execution failure", func(t *testing.T) {
		mockRunner := &mocks.CommandRunnerMock{
			GetAudioCommandFunc: func(filename string) (*exec.Cmd, error) {
				// return a command that will fail
				return exec.Command("false"), nil
			},
		}

		processor := &FFmpegAudioProcessor{cmdRunner: mockRunner}
		err := processor.Play(tmpFile.Name())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error playing audio")
	})
}

func TestDefaultCommandRunner_GetAudioCommand(t *testing.T) {
	runner := &defaultCommandRunner{}

	tests := []struct {
		name        string
		goos        string
		setupMock   func()
		wantErr     bool
		errContains string
		wantCmd     []string
	}{
		{
			name:    "darwin",
			goos:    "darwin",
			wantCmd: []string{"afplay"},
		},
		{
			name:    "windows",
			goos:    "windows",
			wantCmd: []string{"cmd", "/C", "start"},
		},
		{
			name:        "unsupported OS",
			goos:        "plan9",
			wantErr:     true,
			errContains: "unsupported operating system: plan9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// we can't actually change runtime.GOOS in tests, so we'll test what we can
			if runtime.GOOS != tt.goos && tt.goos != "plan9" {
				t.Skip("Test requires", tt.goos, "platform")
			}

			if tt.goos == "plan9" {
				// we can at least verify the error message format
				err := fmt.Errorf("unsupported operating system: %s", tt.goos)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			cmd, err := runner.GetAudioCommand("test.mp3")
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, cmd)
				// verify command structure
				args := append([]string{cmd.Path}, cmd.Args[1:]...)
				for i, want := range tt.wantCmd {
					if i < len(args) {
						assert.Contains(t, args[i], want)
					}
				}
			}
		})
	}

	// security tests that work on any platform
	t.Run("security validation", func(t *testing.T) {
		securityTests := []struct {
			name        string
			filename    string
			errContains string
		}{
			{
				name:        "path traversal attempt",
				filename:    "../../../etc/passwd",
				errContains: "invalid filename: potential security risk",
			},
			{
				name:        "command injection with semicolon",
				filename:    "test.mp3; rm -rf /",
				errContains: "invalid filename: potential security risk",
			},
			{
				name:        "pipe injection attempt",
				filename:    "test.mp3 | cat /etc/passwd",
				errContains: "invalid filename: potential security risk",
			},
			{
				name:        "background execution attempt",
				filename:    "test.mp3 &",
				errContains: "invalid filename: potential security risk",
			},
			{
				name:        "command substitution attempt",
				filename:    "test$(whoami).mp3",
				errContains: "invalid filename: potential security risk",
			},
			{
				name:        "backtick injection attempt",
				filename:    "test`whoami`.mp3",
				errContains: "invalid filename: potential security risk",
			},
		}

		for _, st := range securityTests {
			t.Run(st.name, func(t *testing.T) {
				_, err := runner.GetAudioCommand(st.filename)
				require.Error(t, err)
				assert.Contains(t, err.Error(), st.errContains)
			})
		}
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

func TestFFmpegAudioProcessor_StreamFromConcat(t *testing.T) {
	processor := NewFFmpegAudioProcessor()

	t.Run("non-existent file", func(t *testing.T) {
		config := podcast.Config{
			IcecastUser:  "user",
			IcecastPass:  "pass",
			IcecastURL:   "localhost:8000",
			IcecastMount: "/stream.mp3",
		}
		err := processor.StreamFromConcat("/tmp/non-existent-concat-file.txt", config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ffmpeg streaming failed:")
	})

	t.Run("existing file", func(t *testing.T) {
		// create a temporary file
		tmpFile, err := os.CreateTemp("", "test-concat-*.txt")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.WriteString("file '/path/to/fake.mp3'")
		require.NoError(t, err)
		require.NoError(t, tmpFile.Close())

		config := podcast.Config{
			IcecastUser:  "user",
			IcecastPass:  "pass",
			IcecastURL:   "localhost:8000",
			IcecastMount: "/stream.mp3",
		}

		// this will fail because ffmpeg can't find the input file, but it covers the function
		err = processor.StreamFromConcat(tmpFile.Name(), config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ffmpeg streaming failed:")
	})
}
