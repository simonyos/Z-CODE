package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/simonyos/Z-CODE/internal/agent"
	"github.com/simonyos/Z-CODE/internal/config"
	"github.com/simonyos/Z-CODE/internal/tui/components"
	"github.com/simonyos/Z-CODE/internal/tui/layout"
	"github.com/simonyos/Z-CODE/internal/tui/theme"
)

const version = "0.1.0"

// Message types for Bubble Tea
type responseMsg struct {
	result *agent.ChatResult
	err    error
}

// Streaming message types
type streamStartMsg struct{}

type streamChunkMsg struct {
	text string
}

type streamToolStartMsg struct {
	name string
	args string
}

type streamToolResultMsg struct {
	name    string
	result  string
	isError bool
}

type streamDoneMsg struct {
	finalResponse string
}

// Model is the main TUI model
type Model struct {
	agent *agent.Agent

	// Components
	header      *components.Header
	messages    *components.Messages
	editor      *components.Editor
	status      *components.Status
	help        *components.HelpDialog
	suggestions *components.Suggestions
	spinner     spinner.Model

	// Layout
	layout *layout.SplitPane

	// State
	width            int
	height           int
	ready            bool
	thinking         bool
	showHelp         bool
	streamingContent string                    // Accumulates streaming response
	eventChan        <-chan agent.StreamEvent  // Channel for streaming events
}

// New creates a new TUI model
func New(ag *agent.Agent, modelName string) Model {
	cwd, _ := os.Getwd()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	status := components.NewStatus(80)
	status.SetModel(modelName)

	return Model{
		agent:       ag,
		header:      components.NewHeader(80, version, cwd),
		status:      status,
		help:        components.NewHelpDialog(),
		suggestions: components.NewSuggestions(),
		spinner:     sp,
	}
}

// welcomeMessage returns the initial welcome content
func welcomeMessage() string {
	return `
    ███████╗       ██████╗ ██████╗ ██████╗ ███████╗
    ╚══███╔╝      ██╔════╝██╔═══██╗██╔══██╗██╔════╝
      ███╔╝ █████╗██║     ██║   ██║██║  ██║█████╗
     ███╔╝  ╚════╝██║     ██║   ██║██║  ██║██╔══╝
    ███████╗      ╚██████╗╚██████╔╝██████╔╝███████╗
    ╚══════╝       ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝
`
}

// Init initializes the TUI
func (m Model) Init() tea.Cmd {
	return tea.EnterAltScreen
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle help dialog
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "ctrl+?", "ctrl+h":
			m.showHelp = !m.showHelp
			return m, nil

		case "ctrl+l":
			// Clear chat
			m.messages.Clear()
			return m, nil

		case "esc":
			if m.showHelp {
				m.showHelp = false
			}
			if m.suggestions.IsVisible() {
				m.suggestions.Hide()
			}
			return m, nil

		case "tab":
			// Autocomplete command
			if m.suggestions.IsVisible() {
				selected := m.suggestions.GetSelected()
				if selected != "" {
					m.editor.SetValue(selected)
					m.suggestions.Hide()
				}
				return m, nil
			}

		case "up":
			if m.suggestions.IsVisible() {
				m.suggestions.MoveUp()
				return m, nil
			}

		case "down":
			if m.suggestions.IsVisible() {
				m.suggestions.MoveDown()
				return m, nil
			}

		case "enter":
			// If suggestions visible and selected, use that command
			if m.suggestions.IsVisible() {
				selected := m.suggestions.GetSelected()
				if selected != "" {
					m.editor.Reset()
					m.suggestions.Hide()
					return m.handleCommand(selected)
				}
			}

			if !m.thinking && strings.TrimSpace(m.editor.Value()) != "" {
				userMsg := strings.TrimSpace(m.editor.Value())
				m.editor.Reset()
				m.suggestions.Hide()

				// Check for slash commands
				if strings.HasPrefix(userMsg, "/") {
					return m.handleCommand(userMsg)
				}

				m.messages.AddMessage(components.Message{
					Role:    "user",
					Content: userMsg,
				})
				m.thinking = true
				m.status.SetThinking(true)
				return m, tea.Batch(m.spinner.Tick, m.sendMessage(userMsg))
			}

		case "pgup", "pgdown":
			// Pass to messages viewport
			vp := m.messages.GetViewport()
			var cmd tea.Cmd
			*vp, cmd = vp.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Layout dimensions
		headerHeight := 2
		statusHeight := 2
		editorHeight := 5
		messagesHeight := msg.Height - headerHeight - statusHeight - editorHeight

		if !m.ready {
			m.layout = layout.NewSplitPane(msg.Width, msg.Height)
			m.messages = components.NewMessages(msg.Width, messagesHeight)
			m.messages.SetWelcome(welcomeMessage())
			m.editor = components.NewEditor(msg.Width, editorHeight)
			// Clear any garbage that may have accumulated before init
			m.editor.Reset()
			m.ready = true
		} else {
			m.layout.SetSize(msg.Width, msg.Height)
			m.messages.SetSize(msg.Width, messagesHeight)
			m.editor.SetSize(msg.Width, editorHeight)
		}

		m.header.SetWidth(msg.Width)
		m.status.SetWidth(msg.Width)

	case spinner.TickMsg:
		if m.thinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case responseMsg:
		m.thinking = false
		m.status.SetThinking(false)
		m.eventChan = nil

		if msg.err != nil {
			m.messages.AddMessage(components.Message{
				Role:    "error",
				Content: msg.err.Error(),
			})
		} else if msg.result != nil {
			// Add tool executions first
			for _, tool := range msg.result.ToolCalls {
				result := tool.Result
				if tool.Error != "" {
					result = "Error: " + tool.Error
				}
				m.messages.AddMessage(components.Message{
					Role:     "tool",
					ToolName: tool.Name,
					ToolArgs: tool.Args,
					Content:  result,
				})
			}
			// Then add the final response
			if msg.result.Response != "" {
				m.messages.AddMessage(components.Message{
					Role:    "assistant",
					Content: msg.result.Response,
				})
			}
		}

	// Streaming message handlers
	case streamEventChanMsg:
		m.eventChan = msg.events
		m.streamingContent = ""
		cmds = append(cmds, readNextEvent(m.eventChan))

	case streamStartMsg:
		// Stream started, continue reading
		if m.eventChan != nil {
			cmds = append(cmds, readNextEvent(m.eventChan))
		}

	case streamChunkMsg:
		// Accumulate streaming content and update display
		m.streamingContent += msg.text
		m.messages.UpdateStreaming(m.streamingContent)
		if m.eventChan != nil {
			cmds = append(cmds, readNextEvent(m.eventChan))
		}

	case streamToolStartMsg:
		// Clear streaming content (it was a tool call, not final response)
		m.streamingContent = ""
		m.messages.ClearStreaming()
		// Add tool start message
		m.messages.AddMessage(components.Message{
			Role:     "tool",
			ToolName: msg.name,
			ToolArgs: msg.args,
			Content:  "Running...",
		})
		if m.eventChan != nil {
			cmds = append(cmds, readNextEvent(m.eventChan))
		}

	case streamToolResultMsg:
		// Update the last tool message with result
		result := msg.result
		if msg.isError {
			result = "Error: " + msg.result
		}
		m.messages.UpdateLastToolResult(result)
		if m.eventChan != nil {
			cmds = append(cmds, readNextEvent(m.eventChan))
		}

	case streamDoneMsg:
		m.thinking = false
		m.status.SetThinking(false)
		m.eventChan = nil
		m.messages.ClearStreaming()

		// Add final response if not empty
		if msg.finalResponse != "" {
			m.messages.AddMessage(components.Message{
				Role:    "assistant",
				Content: msg.finalResponse,
			})
		}
	}

	// Update editor if not thinking - only pass key messages
	if !m.thinking && m.editor != nil {
		if _, ok := msg.(tea.KeyMsg); ok {
			var cmd tea.Cmd
			m.editor, cmd = m.editor.Update(msg)
			cmds = append(cmds, cmd)

			// Update suggestions based on editor content
			m.suggestions.Filter(m.editor.Value())
		}
	}

	// Update messages viewport for scrolling
	if m.messages != nil {
		vp := m.messages.GetViewport()
		var cmd tea.Cmd
		*vp, cmd = vp.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) sendMessage(content string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		events := m.agent.ChatStream(ctx, content)
		return streamEventChanMsg{events: events}
	}
}

// streamEventChanMsg carries the event channel
type streamEventChanMsg struct {
	events <-chan agent.StreamEvent
}

// readNextEvent reads the next event from the channel
func readNextEvent(events <-chan agent.StreamEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			// Channel closed
			return streamDoneMsg{}
		}

		switch event.Type {
		case "start":
			return streamStartMsg{}
		case "chunk":
			return streamChunkMsg{text: event.Text}
		case "tool_start":
			return streamToolStartMsg{name: event.ToolName, args: event.ToolArgs}
		case "tool_result":
			return streamToolResultMsg{
				name:    event.ToolName,
				result:  event.ToolResult,
				isError: event.ToolError,
			}
		case "done":
			return streamDoneMsg{finalResponse: event.FinalResponse}
		case "error":
			return responseMsg{err: event.Error}
		}
		return nil
	}
}

// handleCommand processes slash commands
func (m Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/help":
		m.showHelp = true
		return m, nil

	case "/clear":
		m.messages.Clear()
		return m, nil

	case "/reset":
		m.messages.Clear()
		m.agent.Reset()
		m.messages.AddMessage(components.Message{
			Role:    "system",
			Content: "Conversation reset.",
		})
		return m, nil

	case "/tools":
		m.messages.AddMessage(components.Message{
			Role: "system",
			Content: `Available tools:
  📖 read_file   - Read file contents
  📝 write_file  - Create or modify files
  📁 list_dir    - List directory contents
  ⚡ run_command - Execute shell commands`,
		})
		return m, nil

	case "/quit", "/exit", "/q":
		return m, tea.Quit

	case "/config":
		// Handle /config command
		if len(parts) == 1 {
			// Show current config
			keys := config.ListKeys()
			var sb strings.Builder
			sb.WriteString("Configuration:\n")
			sb.WriteString(fmt.Sprintf("  Config file: %s\n\n", config.ConfigPath()))

			if len(keys) == 0 {
				sb.WriteString("  No keys configured.\n")
			} else {
				for k, v := range keys {
					sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
				}
			}
			sb.WriteString("\nUsage:\n")
			sb.WriteString("  /config set <key> <value>  - Set a config value\n")
			sb.WriteString("  /config delete <key>       - Delete a config value\n")
			sb.WriteString("\nKeys: openai, anthropic, provider, model")

			m.messages.AddMessage(components.Message{
				Role:    "system",
				Content: sb.String(),
			})
			return m, nil
		}

		subCmd := strings.ToLower(parts[1])
		switch subCmd {
		case "set":
			if len(parts) < 4 {
				m.messages.AddMessage(components.Message{
					Role:    "error",
					Content: "Usage: /config set <key> <value>",
				})
				return m, nil
			}
			key := parts[2]
			value := strings.Join(parts[3:], " ")
			if err := config.Set(key, value); err != nil {
				m.messages.AddMessage(components.Message{
					Role:    "error",
					Content: fmt.Sprintf("Failed to set config: %v", err),
				})
			} else {
				m.messages.AddMessage(components.Message{
					Role:    "system",
					Content: fmt.Sprintf("Set %s successfully.", key),
				})
			}
			return m, nil

		case "delete", "remove", "unset":
			if len(parts) < 3 {
				m.messages.AddMessage(components.Message{
					Role:    "error",
					Content: "Usage: /config delete <key>",
				})
				return m, nil
			}
			key := parts[2]
			if err := config.Delete(key); err != nil {
				m.messages.AddMessage(components.Message{
					Role:    "error",
					Content: fmt.Sprintf("Failed to delete config: %v", err),
				})
			} else {
				m.messages.AddMessage(components.Message{
					Role:    "system",
					Content: fmt.Sprintf("Deleted %s.", key),
				})
			}
			return m, nil

		default:
			m.messages.AddMessage(components.Message{
				Role:    "error",
				Content: "Unknown config subcommand: " + subCmd + "\nUse: set, delete",
			})
			return m, nil
		}

	default:
		m.messages.AddMessage(components.Message{
			Role:    "error",
			Content: "Unknown command: " + cmd + "\nType /help for available commands.",
		})
		return m, nil
	}
}

// View renders the TUI
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	t := theme.Current

	// Calculate heights
	headerHeight := 2
	statusHeight := 2
	editorHeight := 5
	messagesHeight := m.height - headerHeight - statusHeight - editorHeight

	// Header (fixed at top)
	header := m.header.View()

	// Messages area (fills middle)
	messagesView := m.messages.View()
	if m.thinking {
		// Add thinking indicator at bottom of messages
		thinkingStyle := lipgloss.NewStyle().Foreground(t.Primary)
		messagesView = lipgloss.NewStyle().
			Height(messagesHeight).
			Render(messagesView + "\n" + thinkingStyle.Render(m.spinner.View()+" Thinking..."))
	} else {
		messagesView = lipgloss.NewStyle().
			Height(messagesHeight).
			Render(messagesView)
	}

	// Suggestions (shown above editor when typing /)
	suggestions := ""
	if m.suggestions.IsVisible() {
		m.suggestions.SetWidth(m.width)
		suggestions = m.suggestions.View()
	}

	// Editor (fixed height)
	editor := m.editor.View()

	// Status bar (fixed at bottom)
	status := m.status.View()

	// Stack all sections vertically
	var view string
	if suggestions != "" {
		view = lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			messagesView,
			suggestions,
			editor,
			status,
		)
	} else {
		view = lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			messagesView,
			editor,
			status,
		)
	}

	// Overlay help dialog if shown
	if m.showHelp {
		overlay := m.help.View()
		view = components.PlaceOverlay(overlay, view, m.width, m.height)
	}

	// Apply background and ensure full height
	return lipgloss.NewStyle().
		Background(t.Background).
		Width(m.width).
		Height(m.height).
		Render(view)
}

// ConfirmAction creates a confirmation function for tools
func ConfirmAction(prompt string) bool {
	// In TUI mode, we auto-approve for now
	// TODO: Implement proper confirmation dialog
	return true
}
