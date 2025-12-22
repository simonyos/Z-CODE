# Z-Code

A beautiful terminal-based AI coding assistant with multi-provider support.

![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)

## Features

- **Interactive TUI** - Beautiful terminal interface built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Multi-Provider Support** - Use Claude CLI, Gemini CLI, or OpenAI API
- **Streaming Responses** - See AI responses as they're generated
- **Built-in Tools** - File operations, directory listing, and shell commands
- **Slash Commands** - Quick actions with `/help`, `/config`, `/clear`, and more
- **Configuration Management** - Store API keys and defaults securely

## Screenshot

```
┌──────────────────────────────────────────────────────────────┐
│  Z-Code TUI  v0.1.0                              ~/projects  │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│    ███████╗       ██████╗ ██████╗ ██████╗ ███████╗          │
│    ╚══███╔╝      ██╔════╝██╔═══██╗██╔══██╗██╔════╝          │
│      ███╔╝ █████╗██║     ██║   ██║██║  ██║█████╗            │
│     ███╔╝  ╚════╝██║     ██║   ██║██║  ██║██╔══╝            │
│    ███████╗      ╚██████╗╚██████╔╝██████╔╝███████╗          │
│    ╚══════╝       ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝          │
│                                                              │
│  YOU                                                         │
│  What files are in this directory?                           │
│                                                              │
│  CLAUDE                                                      │
│  Let me check that for you...                                │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│ ╭────────────────────────────────────────────────────────╮   │
│ │ Describe your task...                                  │   │
│ ╰────────────────────────────────────────────────────────╯   │
│ Enter to send · Ctrl+C to quit                      claude   │
└──────────────────────────────────────────────────────────────┘
```

## Installation

### Prerequisites

- Go 1.24 or later
- One of the following AI backends:
  - [Claude Code CLI](https://claude.ai/cli) (default)
  - [Gemini CLI](https://github.com/google-gemini/gemini-cli)
  - OpenAI API key

### Build from Source

```bash
# Clone the repository
git clone https://github.com/simonyos/Z-CODE.git
cd z-code

# Build
go build -o zcode .

# Run
./zcode
```

### Install Go Binary

```bash
go install github.com/simonyos/Z-CODE@latest
```

## Usage

### Basic Usage

```bash
# Start with default provider (Claude CLI)
zcode

# Use a specific provider
zcode -p gemini
zcode -p openai

# Specify a model
zcode -p openai -m gpt-4-turbo
```

### Providers

| Provider | Flag | Requirements |
|----------|------|--------------|
| Claude | `-p claude` | [Claude Code CLI](https://claude.ai/cli) installed |
| Gemini | `-p gemini` | [Gemini CLI](https://github.com/google-gemini/gemini-cli) installed |
| OpenAI | `-p openai` | `OPENAI_API_KEY` or configured via `zcode config` |

### Configuration

Z-Code stores configuration in `~/.config/zcode/config.json`.

```bash
# View current configuration
zcode config

# Set OpenAI API key
zcode config set openai sk-your-api-key

# Set default provider
zcode config set provider gemini

# Set default model
zcode config set model gpt-4o

# Remove a configuration
zcode config delete openai

# Show config file path
zcode config path
```

### Slash Commands

Type these commands in the chat:

| Command | Description |
|---------|-------------|
| `/help` | Show keyboard shortcuts and commands |
| `/clear` | Clear chat history |
| `/reset` | Reset conversation and context |
| `/tools` | List available tools |
| `/config` | Show or set configuration |
| `/quit` | Exit Z-Code |

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Send message |
| `Ctrl+C` | Quit |
| `Ctrl+L` | Clear chat |
| `Ctrl+?` | Toggle help |
| `Tab` | Autocomplete command |
| `↑/↓` | Navigate suggestions |
| `Esc` | Close suggestions/help |
| `PgUp/PgDn` | Scroll messages |

## Project Structure

```
z-code/
├── cmd/
│   ├── root.go           # CLI entry point
│   └── config.go         # Config subcommand
├── internal/
│   ├── agent/            # AI agent orchestration
│   ├── config/           # Configuration management
│   ├── llm/              # LLM providers
│   │   ├── provider.go   # Provider interface
│   │   ├── claude_cli.go # Claude CLI implementation
│   │   ├── gemini_cli.go # Gemini CLI implementation
│   │   └── openai.go     # OpenAI API implementation
│   ├── tools/            # Built-in tools
│   │   ├── read_file.go
│   │   ├── write_file.go
│   │   ├── list_dir.go
│   │   └── bash.go
│   └── tui/              # Terminal UI
│       ├── app.go        # Main Bubble Tea model
│       ├── components/   # UI components
│       ├── layout/       # Layout management
│       └── theme/        # Color themes
├── main.go
├── go.mod
└── README.md
```

## Adding a New Provider

1. Create a new file in `internal/llm/` implementing the `Provider` interface:

```go
type Provider interface {
    Generate(ctx context.Context, messages []Message) (string, error)
    GenerateStream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)
}
```

2. Add the provider to the switch statement in `cmd/root.go`
3. Update the help text and documentation

## Built With

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Style definitions
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Glamour](https://github.com/charmbracelet/glamour) - Markdown rendering
- [Cobra](https://github.com/spf13/cobra) - CLI framework

## Contributing

Contributions are welcome! Here's how you can help:

1. **Fork the repository**
2. **Create a feature branch**: `git checkout -b feature/amazing-feature`
3. **Commit your changes**: `git commit -m 'Add amazing feature'`
4. **Push to the branch**: `git push origin feature/amazing-feature`
5. **Open a Pull Request**

### Development Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/z-code.git
cd z-code

# Install dependencies
go mod download

# Run in development
go run .

# Run tests
go test ./...
```

### Ideas for Contributions

- [ ] Add more LLM providers (Anthropic API, Ollama, etc.)
- [ ] Session persistence (save/resume conversations)
- [ ] Custom themes
- [ ] Plugin system for tools
- [ ] Vim keybindings
- [ ] Multi-file context
- [ ] Code syntax highlighting in responses

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by [Claude Code](https://claude.ai/code) and [Aider](https://github.com/paul-gauthier/aider)
- Built with the amazing [Charm](https://charm.sh/) ecosystem

---

Made with :heart: by [Simon Yos](https://github.com/simonyos)
