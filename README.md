# AI Podcast Generator

An automated tool that generates podcast discussions in Russian language from web articles using AI. The application fetches an article, creates a natural-sounding discussion between multiple hosts, generates speech, and can stream to an Icecast server or save locally.

## Features

- Generates natural-sounding discussions from web articles
- Supports multiple hosts with distinct personalities
- Uses OpenAI GPT-4o for content generation
- Uses OpenAI TTS for realistic speech synthesis
- Streams to Icecast server or saves locally
- Customizable podcast duration

## Requirements

- Go 1.24+
- FFmpeg
- OpenAI API key

## Installation

```bash
git clone https://github.com/radio-t/ai-podcast.git
cd ai-podcast
go build
```

## Usage

```bash
# Stream to Icecast server
./ai-podcast -url "https://example.com/article" -apikey "your-openai-api-key" -icecast "localhost:8000" -mount "/podcast.mp3" -user "source" -pass "hackme" -duration 15

# Generate and save locally
./ai-podcast -url "https://example.com/article" -apikey "your-openai-api-key" -mp3 "output.mp3" -duration 10

# Play locally without saving
./ai-podcast -url "https://example.com/article" -apikey "your-openai-api-key" -dry -duration 5
```

### Command Line Options

- `-url`: URL of the article to discuss (required)
- `-apikey`: OpenAI API key (or set OPENAI_API_KEY environment variable)
- `-icecast`: Icecast server URL (default: "localhost:8000")
- `-mount`: Icecast mount point (default: "/podcast.mp3")
- `-user`: Icecast username (default: "source")
- `-pass`: Icecast password (default: "hackme")
- `-duration`: Target podcast duration in minutes (default: 10)
- `-dry`: Play locally instead of streaming
- `-mp3`: Output MP3 file path (optional)

## License

MIT License - see the [LICENSE](LICENSE) file for details.