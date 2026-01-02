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
	"github.com/simonyos/Z-CODE/internal/agents"
	"github.com/simonyos/Z-CODE/internal/config"
	"github.com/simonyos/Z-CODE/internal/llm"
	"github.com/simonyos/Z-CODE/internal/skills"
	"github.com/simonyos/Z-CODE/internal/tui/components"
	"github.com/simonyos/Z-CODE/internal/tui/layout"
	"github.com/simonyos/Z-CODE/internal/tui/theme"
	"github.com/simonyos/Z-CODE/internal/workflows"
)

const version = "0.1.0"

// Layout constants for consistent height calculations
const (
	layoutHeaderHeight = 2 // Header row + separator line
	layoutStatusHeight = 2 // Separator line + status bar
	layoutEditorHeight = 5 // Input editor area
	layoutPadding      = 1 // Extra padding for separators
)

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

	// Custom agents, skills, and workflows
	agentRegistry    *agents.Registry
	workflowRegistry *workflows.Registry
	skillRegistry    *skills.Registry
	agentExecutor    *agents.Executor
	skillExecutor    *skills.Executor
	workflowEngine   *workflows.Engine
	provider         llm.Provider

	// State
	width            int
	height           int
	ready            bool
	thinking         bool
	showHelp         bool
	streamingContent string                    // Accumulates streaming response
	eventChan        <-chan agent.StreamEvent  // Channel for streaming events
	customEventChan  <-chan agents.StreamEvent // Channel for custom agent streaming
	skillEventChan   <-chan skills.StreamEvent // Channel for skill streaming
}

// New creates a new TUI model
func New(ag *agent.Agent, modelName string) Model {
	cwd, _ := os.Getwd()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	status := components.NewStatus(80)
	status.SetModel(modelName)

	// Initialize custom agent, skill, and workflow registries
	agentReg := agents.NewRegistry()
	_ = agentReg.Refresh() // Load agents from disk

	workflowReg := workflows.NewRegistry()
	_ = workflowReg.Refresh() // Load workflows from disk

	skillLoader := skills.NewLoader(config.GetSkillPaths())
	skillReg := skills.NewRegistry(skillLoader)
	_ = skillReg.Refresh() // Load skills from disk

	suggestions := components.NewSuggestions()

	m := Model{
		agent:            ag,
		header:           components.NewHeader(80, version, cwd),
		status:           status,
		help:             components.NewHelpDialog(),
		suggestions:      suggestions,
		spinner:          sp,
		agentRegistry:    agentReg,
		workflowRegistry: workflowReg,
		skillRegistry:    skillReg,
		provider:         ag.Provider(),
	}

	// Set up command provider for dynamic suggestions
	suggestions.SetCommandProvider(&m)

	return m
}

// NewWithProvider creates a TUI model with explicit provider for custom agents
func NewWithProvider(ag *agent.Agent, modelName string, provider llm.Provider) Model {
	m := New(ag, modelName)
	m.provider = provider
	m.agentExecutor = agents.NewExecutor(provider, ConfirmAction)
	m.skillExecutor = skills.NewExecutor(provider, ConfirmAction)
	m.workflowEngine = workflows.NewEngine(m.agentRegistry, m.workflowRegistry, provider, ConfirmAction)
	return m
}

// GetAgentCommands returns commands for custom agents (implements CommandProvider)
func (m *Model) GetAgentCommands() []components.Command {
	var cmds []components.Command
	for _, ag := range m.agentRegistry.List() {
		cmds = append(cmds, components.Command{
			Name:        "/" + ag.Name,
			Description: ag.Description,
			IsCustom:    true,
			AgentName:   ag.Name,
		})
	}
	return cmds
}

// GetSkillCommands returns commands for skills (implements CommandProvider)
func (m *Model) GetSkillCommands() []components.Command {
	var cmds []components.Command
	for _, sk := range m.skillRegistry.List() {
		cmds = append(cmds, components.Command{
			Name:        "/skill:" + sk.Name,
			Description: "Skill: " + sk.Description,
			IsCustom:    true,
		})
	}
	return cmds
}

// GetWorkflowCommands returns commands for workflows (implements CommandProvider)
func (m *Model) GetWorkflowCommands() []components.Command {
	var cmds []components.Command
	for _, wf := range m.workflowRegistry.List() {
		cmds = append(cmds, components.Command{
			Name:        "/run:" + wf.Name,
			Description: "Workflow: " + wf.Description,
			IsCustom:    true,
		})
	}
	return cmds
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

		// Calculate messages area height using layout constants
		messagesHeight := msg.Height - layoutHeaderHeight - layoutStatusHeight - layoutEditorHeight - layoutPadding

		if !m.ready {
			m.layout = layout.NewSplitPane(msg.Width, msg.Height)
			m.messages = components.NewMessages(msg.Width, messagesHeight)
			m.messages.SetWelcome(welcomeMessage())
			m.editor = components.NewEditor(msg.Width, layoutEditorHeight)
			// Clear any garbage that may have accumulated before init
			m.editor.Reset()
			m.ready = true
		} else {
			m.layout.SetSize(msg.Width, msg.Height)
			m.messages.SetSize(msg.Width, messagesHeight)
			m.editor.SetSize(msg.Width, layoutEditorHeight)
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

	// Custom agent event handlers
	case customAgentEventChanMsg:
		m.customEventChan = msg.events
		m.streamingContent = ""
		cmds = append(cmds, readNextCustomAgentEvent(m.customEventChan))

	case customAgentContinueMsg:
		// Continue reading custom agent events after unknown event type
		cmds = append(cmds, readNextCustomAgentEvent(msg.events))

	// Skill event handlers
	case skillEventChanMsg:
		m.skillEventChan = msg.events
		m.streamingContent = ""
		cmds = append(cmds, readNextSkillEvent(m.skillEventChan))

	case skillContinueMsg:
		// Continue reading skill events after unknown event type
		cmds = append(cmds, readNextSkillEvent(msg.events))

	// Workflow result handler
	case workflowResultMsg:
		m.thinking = false
		m.status.SetThinking(false)

		if msg.err != nil {
			m.messages.AddMessage(components.Message{
				Role:    "error",
				Content: "Workflow error: " + msg.err.Error(),
			})
		} else if msg.result != nil {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Workflow completed: %s\n", msg.result.WorkflowName))
			sb.WriteString(fmt.Sprintf("Success: %v\n", msg.result.Success))
			sb.WriteString(fmt.Sprintf("Steps executed: %d\n", len(msg.result.StepResults)))
			if msg.result.FinalOutput != "" {
				sb.WriteString("\nFinal output:\n")
				sb.WriteString(msg.result.FinalOutput)
			}
			m.messages.AddMessage(components.Message{
				Role:    "assistant",
				Content: sb.String(),
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

// customAgentContinueMsg signals to continue reading custom agent events
type customAgentContinueMsg struct {
	events <-chan agents.StreamEvent
}

// readNextCustomAgentEvent reads the next event from a custom agent channel
func readNextCustomAgentEvent(events <-chan agents.StreamEvent) tea.Cmd {
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
		case "handoff":
			// Handle handoff by showing a message
			if event.Handoff != nil {
				return streamDoneMsg{
					finalResponse: fmt.Sprintf("Handoff requested to agent: %s\nReason: %s",
						event.Handoff.TargetAgent, event.Handoff.Reason),
				}
			}
			// If handoff is nil, continue reading
			return customAgentContinueMsg{events: events}
		default:
			// Unknown event type, continue reading
			return customAgentContinueMsg{events: events}
		}
	}
}

// skillContinueMsg signals to continue reading skill events
type skillContinueMsg struct {
	events <-chan skills.StreamEvent
}

// readNextSkillEvent reads the next event from a skill channel
func readNextSkillEvent(events <-chan skills.StreamEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			// Channel closed
			return streamDoneMsg{}
		}

		switch event.Type {
		case skills.StreamEventText:
			return streamChunkMsg{text: event.Content}
		case skills.StreamEventToolCall:
			return streamToolStartMsg{name: "skill", args: event.Content}
		case skills.StreamEventToolResult:
			return streamToolResultMsg{name: "skill", result: event.Content}
		case skills.StreamEventDone:
			return streamDoneMsg{finalResponse: event.Content}
		case skills.StreamEventError:
			return responseMsg{err: event.Error}
		default:
			// Unknown event type, continue reading
			return skillContinueMsg{events: events}
		}
	}
}

// handleCommand processes slash commands
func (m Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m, nil
	}

	cmd := strings.ToLower(parts[0])

	// Check for skill command (e.g., /skill:explain-code)
	if strings.HasPrefix(cmd, "/skill:") {
		skillName := strings.TrimPrefix(cmd, "/skill:")
		prompt := strings.Join(parts[1:], " ")
		return m.executeSkill(skillName, prompt)
	}

	// Check for custom agent command (e.g., /code-reviewer)
	if strings.HasPrefix(cmd, "/") && !strings.HasPrefix(cmd, "/run:") {
		agentName := strings.TrimPrefix(cmd, "/")
		if agentDef, ok := m.agentRegistry.Get(agentName); ok {
			prompt := strings.Join(parts[1:], " ")
			if prompt == "" {
				prompt = "Help me with my task."
			}
			return m.executeCustomAgent(agentDef, prompt)
		}
	}

	// Check for workflow command (e.g., /run:review-fix)
	if strings.HasPrefix(cmd, "/run:") {
		workflowName := strings.TrimPrefix(cmd, "/run:")
		prompt := strings.Join(parts[1:], " ")
		if prompt == "" {
			prompt = "Execute the workflow."
		}
		return m.executeWorkflow(workflowName, prompt)
	}

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
  read_file   - Read file contents
  write_file  - Create or modify files
  edit_file   - Edit files with find/replace
  list_dir    - List directory contents
  run_command - Execute shell commands
  glob        - Find files by pattern
  grep        - Search file contents`,
		})
		return m, nil

	case "/agents":
		return m.listAgents()

	case "/skills":
		return m.listSkills()

	case "/workflows":
		return m.listWorkflows()

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

// listAgents displays available custom agents
func (m Model) listAgents() (tea.Model, tea.Cmd) {
	agentList := m.agentRegistry.List()

	if len(agentList) == 0 {
		m.messages.AddMessage(components.Message{
			Role:    "system",
			Content: "No custom agents found.\n\nTo create agents, add markdown files to:\n  .zcode/agents/       (project-local)\n  ~/.config/zcode/agents/  (global)",
		})
		return m, nil
	}

	var sb strings.Builder
	sb.WriteString("Custom Agents:\n\n")
	for _, ag := range agentList {
		location := "local"
		if ag.IsGlobal {
			location = "global"
		}
		sb.WriteString(fmt.Sprintf("  /%s - %s (%s)\n", ag.Name, ag.Description, location))
		if len(ag.Tools) > 0 {
			sb.WriteString(fmt.Sprintf("    Tools: %s\n", strings.Join(ag.Tools, ", ")))
		}
	}
	sb.WriteString("\nUsage: /<agent-name> <prompt>")

	m.messages.AddMessage(components.Message{
		Role:    "system",
		Content: sb.String(),
	})
	return m, nil
}

// listWorkflows displays available workflows
func (m Model) listWorkflows() (tea.Model, tea.Cmd) {
	workflowList := m.workflowRegistry.List()

	if len(workflowList) == 0 {
		m.messages.AddMessage(components.Message{
			Role:    "system",
			Content: "No workflows found.\n\nTo create workflows, add YAML files to:\n  .zcode/workflows/       (project-local)\n  ~/.config/zcode/workflows/  (global)",
		})
		return m, nil
	}

	var sb strings.Builder
	sb.WriteString("Workflows:\n\n")
	for _, wf := range workflowList {
		location := "local"
		if wf.IsGlobal {
			location = "global"
		}
		sb.WriteString(fmt.Sprintf("  /run:%s - %s (%s)\n", wf.Name, wf.Description, location))
		sb.WriteString(fmt.Sprintf("    Steps: %d\n", len(wf.Steps)))
	}
	sb.WriteString("\nUsage: /run:<workflow-name> <prompt>")

	m.messages.AddMessage(components.Message{
		Role:    "system",
		Content: sb.String(),
	})
	return m, nil
}

// listSkills displays available skills
func (m Model) listSkills() (tea.Model, tea.Cmd) {
	skillList := m.skillRegistry.List()

	if len(skillList) == 0 {
		m.messages.AddMessage(components.Message{
			Role:    "system",
			Content: "No skills found.\n\nTo create skills, add markdown files to:\n  .zcode/skills/       (project-local)\n  ~/.config/zcode/skills/  (global)",
		})
		return m, nil
	}

	var sb strings.Builder
	sb.WriteString("Skills:\n\n")
	for _, sk := range skillList {
		location := "local"
		if sk.IsGlobal {
			location = "global"
		}
		sb.WriteString(fmt.Sprintf("  /skill:%s - %s (%s)\n", sk.Name, sk.Description, location))
		if len(sk.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("    Tags: %s\n", strings.Join(sk.Tags, ", ")))
		}
		if len(sk.Variables) > 0 {
			sb.WriteString(fmt.Sprintf("    Variables: %s\n", strings.Join(sk.Variables, ", ")))
		}
	}
	sb.WriteString("\nUsage: /skill:<skill-name> <input>")

	m.messages.AddMessage(components.Message{
		Role:    "system",
		Content: sb.String(),
	})
	return m, nil
}

// executeSkill runs a skill
func (m Model) executeSkill(skillName string, userInput string) (tea.Model, tea.Cmd) {
	sk, ok := m.skillRegistry.Get(skillName)
	if !ok {
		m.messages.AddMessage(components.Message{
			Role:    "error",
			Content: "Unknown skill: " + skillName + "\nType /skills to see available skills.",
		})
		return m, nil
	}

	// Ensure executor is initialized
	if m.skillExecutor == nil {
		if m.provider == nil {
			m.messages.AddMessage(components.Message{
				Role:    "error",
				Content: "Cannot execute skill: no LLM provider available",
			})
			return m, nil
		}
		m.skillExecutor = skills.NewExecutor(m.provider, ConfirmAction)
	}

	m.messages.AddMessage(components.Message{
		Role:    "system",
		Content: fmt.Sprintf("Running skill: %s", sk.Name),
	})

	m.messages.AddMessage(components.Message{
		Role:    "user",
		Content: userInput,
	})

	m.thinking = true
	m.status.SetThinking(true)

	return m, tea.Batch(m.spinner.Tick, m.sendSkillMessage(sk, userInput))
}

// sendSkillMessage sends a message using a skill
func (m *Model) sendSkillMessage(sk *skills.SkillDefinition, userInput string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		events := m.skillExecutor.ExecuteStream(ctx, sk, userInput, nil)
		return skillEventChanMsg{events: events}
	}
}

// skillEventChanMsg carries the skill event channel
type skillEventChanMsg struct {
	events <-chan skills.StreamEvent
}

// executeCustomAgent runs a custom agent
func (m Model) executeCustomAgent(agentDef *agents.AgentDefinition, prompt string) (tea.Model, tea.Cmd) {
	// Ensure executor is initialized
	if m.agentExecutor == nil {
		if m.provider == nil {
			m.messages.AddMessage(components.Message{
				Role:    "error",
				Content: "Cannot execute custom agent: no LLM provider available",
			})
			return m, nil
		}
		m.agentExecutor = agents.NewExecutor(m.provider, ConfirmAction)
	}

	m.messages.AddMessage(components.Message{
		Role:    "system",
		Content: fmt.Sprintf("Running agent: %s", agentDef.Name),
	})

	m.messages.AddMessage(components.Message{
		Role:    "user",
		Content: prompt,
	})

	m.thinking = true
	m.status.SetThinking(true)

	return m, tea.Batch(m.spinner.Tick, m.sendCustomAgentMessage(agentDef, prompt))
}

// sendCustomAgentMessage sends a message to a custom agent
func (m *Model) sendCustomAgentMessage(agentDef *agents.AgentDefinition, prompt string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		events := m.agentExecutor.ExecuteStream(ctx, agentDef, prompt)
		return customAgentEventChanMsg{events: events}
	}
}

// customAgentEventChanMsg carries the custom agent event channel
type customAgentEventChanMsg struct {
	events <-chan agents.StreamEvent
}

// executeWorkflow runs a workflow
func (m Model) executeWorkflow(workflowName string, prompt string) (tea.Model, tea.Cmd) {
	wf, ok := m.workflowRegistry.Get(workflowName)
	if !ok {
		m.messages.AddMessage(components.Message{
			Role:    "error",
			Content: "Unknown workflow: " + workflowName + "\nType /workflows to see available workflows.",
		})
		return m, nil
	}

	// Ensure engine is initialized
	if m.workflowEngine == nil {
		if m.provider == nil {
			m.messages.AddMessage(components.Message{
				Role:    "error",
				Content: "Cannot execute workflow: no LLM provider available",
			})
			return m, nil
		}
		m.workflowEngine = workflows.NewEngine(m.agentRegistry, m.workflowRegistry, m.provider, ConfirmAction)
	}

	m.messages.AddMessage(components.Message{
		Role:    "system",
		Content: fmt.Sprintf("Running workflow: %s (%d steps)", wf.Name, len(wf.Steps)),
	})

	m.messages.AddMessage(components.Message{
		Role:    "user",
		Content: prompt,
	})

	m.thinking = true
	m.status.SetThinking(true)

	return m, tea.Batch(m.spinner.Tick, m.executeWorkflowAsync(wf, prompt))
}

// executeWorkflowAsync executes a workflow asynchronously
func (m *Model) executeWorkflowAsync(wf *workflows.WorkflowDefinition, prompt string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		result, err := m.workflowEngine.Execute(ctx, wf.Name, prompt)
		return workflowResultMsg{result: result, err: err}
	}
}

// workflowResultMsg carries workflow execution result
type workflowResultMsg struct {
	result *workflows.WorkflowResult
	err    error
}

// View renders the TUI
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	t := theme.Current

	// Calculate messages area height using layout constants
	messagesHeight := m.height - layoutHeaderHeight - layoutStatusHeight - layoutEditorHeight - layoutPadding

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
