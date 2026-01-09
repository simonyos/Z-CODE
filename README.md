# Z-Code

A beautiful terminal-based AI coding assistant with multi-provider support.

![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)

## Features

- **Interactive TUI** - Beautiful terminal interface built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Multi-Provider Support** - Use OpenAI API, OpenRouter, or LiteLLM (100+ models)
- **Native Tool Calling** - Reliable structured tool calls via OpenAI-compatible API
- **Self-Healing Loop** - Automatic retry when tool calls fail
- **Streaming Responses** - See AI responses as they're generated
- **Built-in Tools** - File operations, directory listing, and shell commands
- **Custom Agents** - Define specialized AI agents with markdown files
- **Workflows** - Chain agents together with YAML workflow definitions
- **Handoff Mode** - Agents can transfer control to other agents with context
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
  - OpenAI API key
  - OpenRouter API key
  - LiteLLM proxy (unified interface to 100+ models)

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
# Start with default provider (LiteLLM)
zcode

# Use a specific provider
zcode -p openai
zcode -p openrouter
zcode -p litellm

# Specify a model
zcode -p openai -m gpt-4-turbo
zcode -p litellm -m anthropic/claude-3.5-sonnet
zcode -p openrouter -m google/gemini-flash-1.5
```

### Providers

| Provider | Flag | Requirements |
|----------|------|--------------|
| LiteLLM | `-p litellm` | LiteLLM proxy running (default) |
| OpenAI | `-p openai` | `OPENAI_API_KEY` or configured via `zcode config` |
| OpenRouter | `-p openrouter` | `OPENROUTER_API_KEY` or configured via `zcode config` |

### Using Claude and Gemini Models

With v2.0, Claude and Gemini models are accessed through API providers:

```bash
# Claude via LiteLLM
zcode -p litellm -m anthropic/claude-3.5-sonnet

# Claude via OpenRouter
zcode -p openrouter -m anthropic/claude-3.5-sonnet

# Gemini via LiteLLM
zcode -p litellm -m google/gemini-flash-1.5

# Gemini via OpenRouter
zcode -p openrouter -m google/gemini-1.5-flash
```

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
| `/agents` | List custom agents |
| `/skills` | List skills |
| `/workflows` | List available workflows |
| `/config` | Show or set configuration |
| `/quit` | Exit Z-Code |

## Custom Agents

Create specialized AI agents by adding markdown files with YAML frontmatter.

### Creating an Agent

Create a file in `.zcode/agents/` (project-local) or `~/.config/zcode/agents/` (global):

```markdown
---
name: code-reviewer
description: Reviews code for best practices
tools:
  - read_file
  - grep
  - glob
max_iterations: 5
handoff_to: code-fixer
---

You are an expert code reviewer. Your task is to analyze code for:
1. Code Quality
2. Best Practices
3. Potential Bugs
...
```

### Using Custom Agents

```bash
# Invoke by name
/code-reviewer "Review the auth module"

# List available agents
/agents
```

### Agent Configuration

| Field | Description |
|-------|-------------|
| `name` | Unique identifier (used as slash command) |
| `description` | Brief description shown in `/agents` |
| `tools` | List of allowed tools (empty = all tools) |
| `max_iterations` | Max LLM calls per conversation (default: 10) |
| `handoff_to` | Default agent for handoffs |

## Skills

Skills are lightweight reusable prompt templates - simpler than agents.

### Creating a Skill

Create a file in `.zcode/skills/` (project-local) or `~/.config/zcode/skills/` (global):

```markdown
---
name: explain-code
description: Explains code in simple terms
tags:
  - learning
  - documentation
variables:
  - language
---

You are a helpful programming instructor. Explain the following code in simple terms.

Language preference: {language}

Code to explain:
{user_input}
```

### Using Skills

```bash
# Invoke a skill
/skill:explain-code "func main() { fmt.Println(\"Hello\") }"

# With variables
/skill:write-tests framework=jest "function add(a, b) { return a + b }"

# List available skills
/skills
```

### Skill Configuration

| Field | Description |
|-------|-------------|
| `name` | Unique identifier (used with `/skill:` prefix) |
| `description` | Brief description shown in `/skills` |
| `tags` | Categories for organization |
| `variables` | Named placeholders (`{var_name}` in prompt) |

### Built-in Example Skills

- `explain-code` - Explains code in simple terms
- `write-tests` - Generates unit tests
- `refactor` - Suggests refactoring improvements
- `debug` - Helps debug code issues
- `document` - Generates documentation

## Workflows

Chain multiple agents together with YAML workflow definitions.

### Creating a Workflow

Create a file in `.zcode/workflows/` (project-local) or `~/.config/zcode/workflows/` (global):

```yaml
name: review-and-fix
description: Reviews code and fixes issues

steps:
  - name: review
    agent: code-reviewer
    output: review_results
    prompt: "Review: {user_input}"

  - name: fix
    agent: code-fixer
    input: review_results
    condition: "review_results.success"
    prompt: "Fix issues: {review_results.output}"

  - name: test
    agent: test-runner
    loop_until: "test_results.success == true"
    max_loops: 3
```

### Using Workflows

```bash
# Run a workflow
/run:review-and-fix "Fix issues in src/api"

# List available workflows
/workflows
```

### Workflow Step Options

| Field | Description |
|-------|-------------|
| `name` | Step identifier |
| `agent` | Agent to execute |
| `input` | Context key to read from |
| `output` | Context key to write to |
| `prompt` | Custom prompt (supports `{variables}`) |
| `condition` | Expression that must be true to run |
| `loop_until` | Expression to stop looping |
| `max_loops` | Maximum loop iterations |
| `on_success` | Step to jump to on success |
| `on_failure` | Step to jump to on failure |

## HiveMind Swarm Mode

Run multiple AI agents collaboratively in real-time using NATS messaging.

### Starting a Swarm

```bash
# Start NATS server (required for swarm communication)
nats-server

# Create a new swarm room as orchestrator
zcode swarm --create "My Project" --role ORCH

# Join an existing room with a specific role
zcode swarm --room ABC123 --role BE_DEV
```

### Available Roles

| Role | Description |
|------|-------------|
| `ORCH` | Orchestrator - coordinates work between agents |
| `SA` | Solution Architect - designs system architecture |
| `BE_DEV` | Backend Developer - implements server-side code |
| `FE_DEV` | Frontend Developer - implements UI/UX |
| `QA` | Quality Assurance - tests and validates |
| `DEVOPS` | DevOps Engineer - handles deployment and infrastructure |
| `DBA` | Database Administrator - manages data layer |
| `SEC` | Security Engineer - handles security concerns |
| `HUMAN` | Human supervisor with override capabilities |

### Swarm Communication

In swarm mode, use `@ROLE` mentions to communicate with other agents:

```
@SA Can you design the authentication flow?
@BE_DEV Implement the API endpoints from SA's design
@ALL Status update: Phase 1 complete
```

### Human Controls (HUMAN role only)

When joined as HUMAN, you have special commands:

| Command | Description |
|---------|-------------|
| `PAUSE [reason]` | Pause all agent autopilot |
| `RESUME [message]` | Resume agent autopilot |
| `OVERRIDE: @ROLE instruction` | Force an agent to execute |
| `STATUS` | Request status from all agents |

### Keyboard Shortcuts (Swarm Mode)

| Key | Action |
|-----|--------|
| `Ctrl+A` | Toggle autopilot mode |
| `Ctrl+M` | Toggle swarm panel visibility |
| `Tab` | Switch focus between chat and swarm panel |
| `Esc` | Interrupt current LLM response |
| `Ctrl+L` | Clear local chat |

### Swarm Tools

Agents in swarm mode have access to specialized tools:

- `ask_agent` - Send a message to a specific agent
- `broadcast` - Send a message to all agents
- `list_agents` - See who's online in the room

### Connection Recovery

Z-Code automatically handles connection issues:
- Automatic reconnection to NATS server
- Presence re-announcement after reconnect
- Visual indicators for connection state

### Persistence

Swarm conversations are automatically persisted to `~/.zcode/swarm/`:
- Message history for each room
- Agent presence state
- Room metadata

## Handoff Mode

Agents can transfer control to other agents using XML handoff tags:

```xml
<handoff agent="code-fixer" reason="Found issues to fix">
  <context key="issues">List of issues here</context>
</handoff>
```

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
│   ├── config.go         # Config subcommand
│   └── swarm.go          # Swarm mode command
├── internal/
│   ├── agent/            # AI agent orchestration
│   ├── agents/           # Custom agent system
│   │   ├── definition.go # Agent definition types
│   │   ├── loader.go     # Markdown parser
│   │   ├── registry.go   # Agent discovery
│   │   ├── executor.go   # Agent execution
│   │   └── handoff.go    # Handoff parsing
│   ├── skills/           # Skills system
│   │   ├── definition.go # Skill definition types
│   │   ├── loader.go     # Markdown parser
│   │   ├── registry.go   # Skill discovery
│   │   └── executor.go   # Skill execution
│   ├── workflows/        # Workflow engine
│   │   ├── definition.go # Workflow/step types
│   │   ├── loader.go     # YAML parser
│   │   ├── engine.go     # Workflow execution
│   │   ├── context.go    # Shared state
│   │   └── handoff.go    # Handoff management
│   ├── swarm/            # HiveMind swarm system
│   │   ├── client.go     # Main swarm client
│   │   ├── nats.go       # NATS messaging
│   │   ├── room.go       # Room management
│   │   ├── roles.go      # Role definitions
│   │   ├── message.go    # Message types
│   │   ├── presence.go   # Presence tracking
│   │   ├── handler.go    # Message handling
│   │   ├── persistence.go # Room persistence
│   │   └── errors.go     # Error definitions
│   ├── config/           # Configuration management
│   ├── llm/              # LLM providers
│   │   ├── provider.go   # Provider interface
│   │   ├── types.go      # OpenAI-compatible types
│   │   ├── openai.go     # OpenAI API implementation
│   │   ├── openrouter.go # OpenRouter implementation
│   │   └── litellm.go    # LiteLLM implementation (with native tool calling)
│   ├── tools/            # Built-in tools
│   │   ├── read_file.go
│   │   ├── write_file.go
│   │   ├── edit.go
│   │   ├── list_dir.go
│   │   ├── glob.go
│   │   ├── grep.go
│   │   ├── bash.go
│   │   └── swarm.go      # Swarm-specific tools
│   └── tui/              # Terminal UI
│       ├── app.go        # Main Bubble Tea model
│       ├── swarm_app.go  # Swarm mode TUI
│       ├── components/   # UI components
│       ├── layout/       # Layout management
│       └── theme/        # Color themes
├── roles/                # Role definition files
│   ├── ORCH.md
│   ├── SA.md
│   ├── BE_DEV.md
│   └── ...
├── .zcode/               # Project-local agents/skills/workflows
│   ├── agents/
│   ├── skills/
│   └── workflows/
├── main.go
├── Makefile              # Build and install targets
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

2. For native tool calling support, also implement `ToolProvider`:

```go
type ToolProvider interface {
    Provider
    GenerateWithTools(ctx context.Context, messages []Message, tools []OpenAITool) (*ToolCallResponse, error)
    GenerateStreamWithTools(ctx context.Context, messages []Message, tools []OpenAITool) (<-chan ToolStreamChunk, error)
}
```

3. Add the provider to the switch statement in `cmd/root.go`
4. Update the help text and documentation

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

- [ ] Add more LLM providers (Ollama, etc.)
- [ ] Session persistence (save/resume conversations)
- [ ] Custom themes
- [ ] Plugin system for tools
- [ ] Vim keybindings
- [ ] Multi-file context
- [ ] Code syntax highlighting in responses
- [ ] Dual-brain mode (fast/smart model switching)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by [Claude Code](https://claude.ai/code) and [Aider](https://github.com/paul-gauthier/aider)
- Built with the amazing [Charm](https://charm.sh/) ecosystem

---

Made with :heart: by [Simon Yos](https://github.com/simonyos)
