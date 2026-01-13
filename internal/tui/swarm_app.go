package tui

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/simonyos/Z-CODE/internal/agent"
	"github.com/simonyos/Z-CODE/internal/swarm"
	"github.com/simonyos/Z-CODE/internal/tui/components"
	"github.com/simonyos/Z-CODE/internal/tui/layout"
	"github.com/simonyos/Z-CODE/internal/tui/theme"
)

// debugEnabled checks if debug mode is enabled
func debugEnabled() bool {
	return os.Getenv("ZCODE_DEBUG") != ""
}

// debugLog writes debug output to stderr
func debugLog(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[DEBUG] "+format, args...)
}

// Swarm-specific message types
type swarmMessageMsg struct {
	msg *swarm.Message
}

type swarmPresenceMsg struct {
	event *swarm.PresenceEvent
}

type swarmErrorMsg struct {
	err error
}

type swarmConnectedMsg struct{}

type swarmConnectionEventMsg struct {
	event *swarm.ConnectionEvent
}

// SwarmModel is the TUI model for swarm mode
type SwarmModel struct {
	agent       *agent.Agent
	swarmClient *swarm.Client

	// Components
	header     *components.SwarmHeader
	messages   *components.Messages
	swarmPanel *components.SwarmPanel
	editor     *components.Editor
	status     *components.Status
	help       *components.HelpDialog
	spinner    spinner.Model

	// Layout
	layout     *layout.SplitPane
	showSwarm  bool
	focusSwarm bool

	// State
	width            int
	height           int
	ready            bool
	thinking         bool
	showHelp         bool
	streamingContent string
	eventChan        <-chan agent.StreamEvent

	// Swarm message channels
	swarmMsgChan        <-chan *swarm.Message
	swarmPresenceChan   <-chan *swarm.PresenceEvent
	swarmConnChan       <-chan *swarm.ConnectionEvent
	swarmErrorChan      <-chan error

	// Autopilot mode - when false, agents don't auto-respond to messages
	autopilot bool

	// Connection state
	connectionState swarm.ConnectionState

	// Context for swarm operations
	ctx    context.Context
	cancel context.CancelFunc
}

// NewSwarmModel creates a new swarm TUI model
func NewSwarmModel(ag *agent.Agent, client *swarm.Client, modelName string) SwarmModel {
	ctx, cancel := context.WithCancel(context.Background())

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	status := components.NewStatus(80)
	status.SetModel(modelName)

	// role and roomCode will be used during initComponents
	_ = client.Role()
	if client.Room() != nil {
		_ = client.Room().Code
	}

	return SwarmModel{
		agent:       ag,
		swarmClient: client,
		status:      status,
		help:        components.NewHelpDialog(),
		spinner:     sp,
		showSwarm:   true,
		autopilot:   true, // Agents auto-respond by default
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Init initializes the swarm TUI
func (m SwarmModel) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.startSwarmListener(),
	)
}

// startSwarmListener starts listening for swarm events
func (m *SwarmModel) startSwarmListener() tea.Cmd {
	return func() tea.Msg {
		return swarmConnectedMsg{}
	}
}

// waitForSwarmMessage waits for the next swarm message from the client
func waitForSwarmMessage(client *swarm.Client, msgChan <-chan *swarm.Message) tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-msgChan:
			if !ok {
				return nil // Channel closed
			}
			return swarmMessageMsg{msg: msg}
		}
	}
}

// waitForSwarmPresence waits for presence updates
func waitForSwarmPresence(presenceChan <-chan *swarm.PresenceEvent) tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-presenceChan:
			if !ok {
				return nil
			}
			return swarmPresenceMsg{event: event}
		}
	}
}

// waitForConnectionEvent waits for connection state changes
func waitForConnectionEvent(connChan <-chan *swarm.ConnectionEvent) tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-connChan:
			if !ok {
				return nil
			}
			return swarmConnectionEventMsg{event: event}
		}
	}
}

// waitForSwarmError waits for error events
func waitForSwarmError(errorChan <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case err, ok := <-errorChan:
			if !ok {
				return nil
			}
			return swarmErrorMsg{err: err}
		}
	}
}

// Update handles messages for swarm mode
func (m SwarmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.cancel()
			m.swarmClient.Close()
			return m, tea.Quit

		case "ctrl+?", "ctrl+h":
			m.showHelp = !m.showHelp
			return m, nil

		case "ctrl+l":
			m.messages.Clear()
			return m, nil

		case "ctrl+m":
			// Toggle swarm panel
			m.showSwarm = !m.showSwarm
			m.updateLayout()
			return m, nil

		case "tab":
			// Switch focus between local chat and swarm panel
			if m.showSwarm {
				m.focusSwarm = !m.focusSwarm
				if m.swarmPanel != nil {
					m.swarmPanel.SetFocused(m.focusSwarm)
					if m.focusSwarm {
						m.swarmPanel.ClearUnread()
					}
				}
			}
			return m, nil

		case "ctrl+a":
			// Toggle autopilot mode
			m.autopilot = !m.autopilot
			status := "ON"
			if !m.autopilot {
				status = "OFF"
			}
			if m.messages != nil {
				m.messages.AddMessage(components.Message{
					Role:    "system",
					Content: fmt.Sprintf("Autopilot %s", status),
				})
			}
			return m, nil

		case "esc":
			if m.showHelp {
				m.showHelp = false
				return m, nil
			}
			// Interrupt current LLM response if thinking
			if m.thinking {
				m.thinking = false
				m.status.SetThinking(false)
				m.streamingContent = ""
				m.eventChan = nil
				if m.messages != nil {
					m.messages.ClearStreaming()
					m.messages.AddMessage(components.Message{
						Role:    "system",
						Content: "⛔ Interrupted",
					})
				}
				return m, nil
			}
			return m, nil

		case "up", "down":
			// If swarm panel focused, handle navigation there
			if m.focusSwarm && m.swarmPanel != nil {
				if msg.String() == "up" {
					m.swarmPanel.MoveUp()
				} else {
					m.swarmPanel.MoveDown()
				}
				return m, nil
			}

		case "enter":
			if !m.thinking && strings.TrimSpace(m.editor.Value()) != "" {
				userMsg := strings.TrimSpace(m.editor.Value())
				m.editor.Reset()

				// Check for special HUMAN commands (only if we are HUMAN role)
				if m.swarmClient.Role() == swarm.RoleHuman {
					upperMsg := strings.ToUpper(userMsg)

					// PAUSE command
					if upperMsg == "PAUSE" || strings.HasPrefix(upperMsg, "PAUSE ") {
						reason := "Human requested pause"
						if len(userMsg) > 6 {
							reason = strings.TrimSpace(userMsg[6:])
						}
						msg := swarm.NewPause(m.swarmClient.Room().Code, swarm.RoleHuman, reason)
						m.swarmClient.Send(msg)
						if m.messages != nil {
							m.messages.AddMessage(components.Message{
								Role:    "system",
								Content: fmt.Sprintf("⏸ PAUSE sent to all agents: %s", reason),
							})
						}
						return m, nil
					}

					// RESUME command
					if upperMsg == "RESUME" || strings.HasPrefix(upperMsg, "RESUME ") {
						message := "Resuming operations"
						if len(userMsg) > 7 {
							message = strings.TrimSpace(userMsg[7:])
						}
						msg := swarm.NewResume(m.swarmClient.Room().Code, swarm.RoleHuman, message)
						m.swarmClient.Send(msg)
						if m.messages != nil {
							m.messages.AddMessage(components.Message{
								Role:    "system",
								Content: fmt.Sprintf("▶ RESUME sent to all agents: %s", message),
							})
						}
						return m, nil
					}

					// OVERRIDE command
					if strings.HasPrefix(upperMsg, "OVERRIDE:") || strings.HasPrefix(upperMsg, "OVERRIDE ") {
						instruction := strings.TrimSpace(userMsg[9:])
						if instruction != "" {
							// Check for @ROLE target in override
							if target, content := parseSwarmMention(instruction); target != "" {
								msg := swarm.NewOverride(m.swarmClient.Room().Code, swarm.RoleHuman, target, content)
								m.swarmClient.Send(msg)
								if m.messages != nil {
									m.messages.AddMessage(components.Message{
										Role:    "system",
										Content: fmt.Sprintf("⚠ OVERRIDE sent to %s: %s", target, content),
									})
								}
							} else {
								// Broadcast override to all
								msg := swarm.NewOverride(m.swarmClient.Room().Code, swarm.RoleHuman, swarm.RoleAll, instruction)
								m.swarmClient.Send(msg)
								if m.messages != nil {
									m.messages.AddMessage(components.Message{
										Role:    "system",
										Content: fmt.Sprintf("⚠ OVERRIDE sent to ALL: %s", instruction),
									})
								}
							}
							return m, nil
						}
					}

					// STATUS command
					if upperMsg == "STATUS" {
						m.swarmClient.Broadcast("STATUS? Please report your current status.")
						if m.messages != nil {
							m.messages.AddMessage(components.Message{
								Role:    "system",
								Content: "📊 Status request sent to all agents",
							})
						}
						return m, nil
					}
				}

				// Check for @ROLE mentions (swarm messages)
				if target, content := parseSwarmMention(userMsg); target != "" {
					return m.sendSwarmMessage(target, content)
				}

				// Regular chat message
				m.messages.AddMessage(components.Message{
					Role:    "user",
					Content: userMsg,
				})
				m.thinking = true
				m.status.SetThinking(true)
				return m, tea.Batch(m.spinner.Tick, m.sendMessage(userMsg))
			}

		case "pgup", "pgdown":
			if m.focusSwarm && m.swarmPanel != nil {
				vp := m.swarmPanel.GetViewport()
				var cmd tea.Cmd
				*vp, cmd = vp.Update(msg)
				cmds = append(cmds, cmd)
			} else {
				vp := m.messages.GetViewport()
				var cmd tea.Cmd
				*vp, cmd = vp.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

		if !m.ready {
			m.initComponents()
			m.ready = true
		}

	case spinner.TickMsg:
		if m.thinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case swarmConnectedMsg:
		m.connectionState = swarm.StateConnected
		if m.messages != nil {
			m.messages.AddMessage(components.Message{
				Role:    "system",
				Content: fmt.Sprintf("Connected to swarm as %s", m.swarmClient.Role()),
			})
		}
		// Start listening for messages, presence, connection events, and errors
		m.swarmMsgChan = m.swarmClient.Messages()
		m.swarmPresenceChan = m.swarmClient.PresenceUpdates()
		m.swarmConnChan = m.swarmClient.ConnectionEvents()
		m.swarmErrorChan = m.swarmClient.Errors()

		if m.swarmMsgChan != nil {
			cmds = append(cmds, waitForSwarmMessage(m.swarmClient, m.swarmMsgChan))
		}
		if m.swarmPresenceChan != nil {
			cmds = append(cmds, waitForSwarmPresence(m.swarmPresenceChan))
		}
		if m.swarmConnChan != nil {
			cmds = append(cmds, waitForConnectionEvent(m.swarmConnChan))
		}
		if m.swarmErrorChan != nil {
			cmds = append(cmds, waitForSwarmError(m.swarmErrorChan))
		}

	case swarmMessageMsg:
		// Add to swarm panel
		if m.swarmPanel != nil {
			m.swarmPanel.AddMessage(msg.msg)
		}

		// Handle control messages (PAUSE/RESUME/OVERRIDE) from HUMAN
		if msg.msg.IsControlMessage() {
			switch msg.msg.Type {
			case swarm.MsgPause:
				m.autopilot = false
				if m.messages != nil {
					m.messages.AddMessage(components.Message{
						Role:    "system",
						Content: fmt.Sprintf("⏸ PAUSED by %s: %s", msg.msg.From, msg.msg.Content),
					})
				}
				if m.swarmMsgChan != nil {
					cmds = append(cmds, waitForSwarmMessage(m.swarmClient, m.swarmMsgChan))
				}
				return m, tea.Batch(cmds...)

			case swarm.MsgResume:
				m.autopilot = true
				if m.messages != nil {
					m.messages.AddMessage(components.Message{
						Role:    "system",
						Content: fmt.Sprintf("▶ RESUMED by %s: %s", msg.msg.From, msg.msg.Content),
					})
				}
				if m.swarmMsgChan != nil {
					cmds = append(cmds, waitForSwarmMessage(m.swarmClient, m.swarmMsgChan))
				}
				return m, tea.Batch(cmds...)

			case swarm.MsgHumanOverride:
				// Override messages always get processed, even when paused
				if m.messages != nil {
					m.messages.AddMessage(components.Message{
						Role:    "system",
						Content: fmt.Sprintf("⚠ OVERRIDE from %s: %s", msg.msg.From, msg.msg.Content),
					})
				}
				// Force process the override
				if !m.thinking {
					m.thinking = true
					m.status.SetThinking(true)
					llmPrompt := fmt.Sprintf("[URGENT OVERRIDE from %s]: %s\n\nYou MUST follow this instruction immediately.", msg.msg.From, msg.msg.Content)
					cmds = append(cmds, m.spinner.Tick, m.sendMessage(llmPrompt))
				}
				if m.swarmMsgChan != nil {
					cmds = append(cmds, waitForSwarmMessage(m.swarmClient, m.swarmMsgChan))
				}
				return m, tea.Batch(cmds...)
			}
		}

		// If message is to us (and not from us), process it
		isToUs := msg.msg.IsToRole(m.swarmClient.Role()) || msg.msg.To == swarm.RoleAll
		isFromUs := msg.msg.IsFromRole(m.swarmClient.Role())

		if isToUs && !isFromUs {
			// Show in local chat
			if m.messages != nil {
				m.messages.AddMessage(components.Message{
					Role:    "system",
					Content: fmt.Sprintf("[%s → %s] %s", msg.msg.From, msg.msg.To, msg.msg.Content),
				})
			}

			// Auto-respond: Send to LLM for response (if autopilot is ON and not already thinking)
			// Messages from HUMAN always get processed regardless of autopilot state
			shouldProcess := m.autopilot || msg.msg.IsFromHuman()
			if shouldProcess && !m.thinking {
				m.thinking = true
				m.status.SetThinking(true)
				// Format the incoming message with context for the LLM
				prefix := "[Swarm message from %s]: %s"
				if msg.msg.IsFromHuman() {
					prefix = "[HUMAN instruction from %s]: %s\n\nThis is a message from a human team member. Prioritize this request."
				}
				llmPrompt := fmt.Sprintf(prefix, msg.msg.From, msg.msg.Content)
				cmds = append(cmds, m.spinner.Tick, m.sendMessage(llmPrompt))
			} else if !m.autopilot && !msg.msg.IsFromHuman() {
				// Show that we're paused
				if m.messages != nil {
					m.messages.AddMessage(components.Message{
						Role:    "system",
						Content: "⏸ (Autopilot OFF - not auto-responding. Press Ctrl+A to toggle.)",
					})
				}
			}
		}

		// Continue listening for more messages
		if m.swarmMsgChan != nil {
			cmds = append(cmds, waitForSwarmMessage(m.swarmClient, m.swarmMsgChan))
		}

	case swarmPresenceMsg:
		if m.swarmPanel != nil {
			m.swarmPanel.SetPresenceFromEvent(msg.event)
		}
		// Continue listening for more presence updates
		if m.swarmPresenceChan != nil {
			cmds = append(cmds, waitForSwarmPresence(m.swarmPresenceChan))
		}

	case swarmConnectionEventMsg:
		m.connectionState = msg.event.State
		var icon string
		var msgRole string
		switch msg.event.State {
		case swarm.StateConnected:
			icon = "✓"
			msgRole = "system"
		case swarm.StateReconnecting:
			icon = "⟳"
			msgRole = "system"
		case swarm.StateDisconnected, swarm.StateClosed:
			icon = "✗"
			msgRole = "error"
		default:
			icon = "ℹ"
			msgRole = "system"
		}
		if m.messages != nil {
			content := fmt.Sprintf("%s %s", icon, msg.event.Message)
			if msg.event.Error != nil {
				content = fmt.Sprintf("%s %s: %v", icon, msg.event.Message, msg.event.Error)
			}
			m.messages.AddMessage(components.Message{
				Role:    msgRole,
				Content: content,
			})
		}
		// Continue listening for connection events
		if m.swarmConnChan != nil {
			cmds = append(cmds, waitForConnectionEvent(m.swarmConnChan))
		}

	case swarmErrorMsg:
		if m.messages != nil {
			m.messages.AddMessage(components.Message{
				Role:    "error",
				Content: fmt.Sprintf("Swarm error: %v", msg.err),
			})
		}
		// Continue listening for errors
		if m.swarmErrorChan != nil {
			cmds = append(cmds, waitForSwarmError(m.swarmErrorChan))
		}

	// Standard streaming message handlers
	case streamEventChanMsg:
		m.eventChan = msg.events
		m.streamingContent = ""
		cmds = append(cmds, readNextEvent(m.eventChan))

	case streamStartMsg:
		if m.eventChan != nil {
			cmds = append(cmds, readNextEvent(m.eventChan))
		}

	case streamChunkMsg:
		m.streamingContent += msg.text
		if m.messages != nil {
			m.messages.UpdateStreaming(m.streamingContent)
		}
		if m.eventChan != nil {
			cmds = append(cmds, readNextEvent(m.eventChan))
		}

	case streamToolStartMsg:
		if debugEnabled() {
			debugLog("TUI streamToolStartMsg: name=%s, args=%s\n", msg.name, msg.args)
		}
		m.streamingContent = ""
		if m.messages != nil {
			m.messages.ClearStreaming()
			m.messages.AddMessage(components.Message{
				Role:     "tool",
				ToolName: msg.name,
				ToolArgs: msg.args,
				Content:  "Running...",
			})
		}
		if m.eventChan != nil {
			cmds = append(cmds, readNextEvent(m.eventChan))
		}

	case streamToolResultMsg:
		if debugEnabled() {
			debugLog("TUI streamToolResultMsg: name=%s, isError=%v, result_len=%d\n", msg.name, msg.isError, len(msg.result))
		}
		result := msg.result
		if msg.isError {
			result = "Error: " + msg.result
		}
		if m.messages != nil {
			m.messages.UpdateLastToolResult(result)
		}
		if m.eventChan != nil {
			cmds = append(cmds, readNextEvent(m.eventChan))
		}

	case streamBatchStartMsg:
		// Start of a batch of tool calls - clear streaming content
		if debugEnabled() {
			debugLog("TUI streamBatchStartMsg: batchSize=%d\n", msg.batchSize)
		}
		m.streamingContent = ""
		if m.messages != nil {
			m.messages.ClearStreaming()
		}
		if m.eventChan != nil {
			cmds = append(cmds, readNextEvent(m.eventChan))
		}

	case streamBatchEndMsg:
		// End of a batch - continue reading for potential next iteration
		if debugEnabled() {
			debugLog("TUI streamBatchEndMsg: batchSize=%d\n", msg.batchSize)
		}
		if m.eventChan != nil {
			cmds = append(cmds, readNextEvent(m.eventChan))
		}

	case streamDoneMsg:
		m.thinking = false
		m.status.SetThinking(false)
		m.eventChan = nil
		if m.messages != nil {
			m.messages.ClearStreaming()
			if msg.finalResponse != "" {
				m.messages.AddMessage(components.Message{
					Role:    "assistant",
					Content: msg.finalResponse,
				})

				// Check if response contains @ROLE mentions and send to swarm
				swarmCmds := m.parseAndSendSwarmMentions(msg.finalResponse)
				cmds = append(cmds, swarmCmds...)
			}
		}

	case responseMsg:
		m.thinking = false
		m.status.SetThinking(false)
		m.eventChan = nil

		if msg.err != nil && m.messages != nil {
			m.messages.AddMessage(components.Message{
				Role:    "error",
				Content: msg.err.Error(),
			})
		}
	}

	// Update editor if not thinking and not focused on swarm
	if !m.thinking && m.editor != nil && !m.focusSwarm {
		if _, ok := msg.(tea.KeyMsg); ok {
			var cmd tea.Cmd
			m.editor, cmd = m.editor.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Update viewports
	if m.messages != nil && !m.focusSwarm {
		vp := m.messages.GetViewport()
		var cmd tea.Cmd
		*vp, cmd = vp.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// initComponents initializes all components with current dimensions
func (m *SwarmModel) initComponents() {
	role := m.swarmClient.Role()
	roomCode := ""
	if m.swarmClient.Room() != nil {
		roomCode = m.swarmClient.Room().Code
	}

	// Calculate dimensions
	leftWidth := m.width
	rightWidth := 0
	if m.showSwarm {
		leftWidth = int(float64(m.width) * 0.6)
		rightWidth = m.width - leftWidth
	}

	messagesHeight := m.height - layoutHeaderHeight - layoutStatusHeight - layoutEditorHeight - layoutPadding

	m.layout = layout.NewSplitPane(m.width, m.height)
	m.layout.ShowRight = m.showSwarm
	m.layout.LeftRatio = 0.6

	roomName := roomCode
	if m.swarmClient.Room() != nil && m.swarmClient.Room().Name != "" {
		roomName = m.swarmClient.Room().Name
	}

	m.header = components.NewSwarmHeader(m.width, version, roomCode, roomName, role)
	m.messages = components.NewMessages(leftWidth, messagesHeight)
	// Don't set the regular welcome - we'll add our swarm welcome as a system message
	m.editor = components.NewEditor(leftWidth, layoutEditorHeight)

	// Add welcome message
	m.messages.AddMessage(components.Message{
		Role: "system",
		Content: fmt.Sprintf(`🐝 HiveMind Swarm Mode Active

Room: %s
Code: %s
Role: %s

Use @ROLE to message agents (e.g., @SA, @BE_DEV, @ALL)
Press Ctrl+M to toggle swarm panel, Tab to switch focus`, roomName, roomCode, role),
	})

	if m.showSwarm {
		m.swarmPanel = components.NewSwarmPanel(rightWidth, m.height-layoutStatusHeight, role, roomCode)
		// Set our presence as online
		m.swarmPanel.UpdatePresence(role, swarm.PresenceOnline)
	}
}

// updateLayout updates component sizes when layout changes
func (m *SwarmModel) updateLayout() {
	if !m.ready {
		return
	}

	leftWidth := m.width
	rightWidth := 0
	if m.showSwarm {
		leftWidth = int(float64(m.width) * 0.6)
		rightWidth = m.width - leftWidth
	}

	messagesHeight := m.height - layoutHeaderHeight - layoutStatusHeight - layoutEditorHeight - layoutPadding

	if m.layout != nil {
		m.layout.SetSize(m.width, m.height)
		m.layout.ShowRight = m.showSwarm
	}
	if m.header != nil {
		m.header.SetWidth(m.width)
	}
	if m.messages != nil {
		m.messages.SetSize(leftWidth, messagesHeight)
	}
	if m.editor != nil {
		m.editor.SetSize(leftWidth, layoutEditorHeight)
	}
	if m.status != nil {
		m.status.SetWidth(m.width)
	}

	if m.swarmPanel != nil {
		if m.showSwarm {
			m.swarmPanel.SetSize(rightWidth, m.height-layoutStatusHeight)
		}
	} else if m.showSwarm {
		role := m.swarmClient.Role()
		roomCode := ""
		if m.swarmClient.Room() != nil {
			roomCode = m.swarmClient.Room().Code
		}
		m.swarmPanel = components.NewSwarmPanel(rightWidth, m.height-layoutStatusHeight, role, roomCode)
	}
}

// sendMessage sends a chat message to the agent
func (m *SwarmModel) sendMessage(content string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		events := m.agent.ChatStream(ctx, content)
		return streamEventChanMsg{events: events}
	}
}

// sendSwarmMessage sends a message to another agent via swarm
func (m SwarmModel) sendSwarmMessage(target swarm.Role, content string) (tea.Model, tea.Cmd) {
	// Show in local chat
	if m.messages != nil {
		m.messages.AddMessage(components.Message{
			Role:    "user",
			Content: fmt.Sprintf("@%s %s", target, content),
		})
	}

	// Send via swarm
	var err error
	if target == swarm.RoleAll {
		err = m.swarmClient.Broadcast(content)
	} else {
		err = m.swarmClient.SendTo(target, content)
	}

	if err != nil {
		if m.messages != nil {
			m.messages.AddMessage(components.Message{
				Role:    "error",
				Content: fmt.Sprintf("Failed to send: %v", err),
			})
		}
	} else {
		// Add own message to swarm panel
		if m.swarmPanel != nil {
			ownMsg := swarm.NewMessage(
				m.swarmClient.Room().Code,
				m.swarmClient.Role(),
				target,
				swarm.MsgBroadcast,
				content,
			)
			m.swarmPanel.AddMessage(ownMsg)
		}
	}

	return m, nil
}

// parseSwarmMention extracts @ROLE from message
func parseSwarmMention(content string) (swarm.Role, string) {
	re := regexp.MustCompile(`^@(\w+)\s*(.*)$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(content))

	if len(matches) == 3 {
		roleStr := strings.ToUpper(matches[1])
		role, err := swarm.ParseRole(roleStr)
		if err == nil {
			return role, matches[2]
		}
	}

	return "", content
}

// parseAndSendSwarmMentions finds all @ROLE mentions in LLM output and sends them to swarm
func (m *SwarmModel) parseAndSendSwarmMentions(content string) []tea.Cmd {
	var cmds []tea.Cmd

	// Find all @ROLE mentions (with optional punctuation after)
	// Pattern matches @ROLE at word boundaries
	re := regexp.MustCompile(`@(\w+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	// Track which roles we've already sent to (avoid duplicates)
	sentTo := make(map[swarm.Role]bool)

	for _, match := range matches {
		if len(match) == 2 {
			roleStr := strings.ToUpper(match[1])

			role, err := swarm.ParseRole(roleStr)
			if err == nil && !sentTo[role] {
				sentTo[role] = true

				// Send the full response content to the mentioned role
				var sendErr error
				if role == swarm.RoleAll {
					sendErr = m.swarmClient.Broadcast(content)
				} else {
					sendErr = m.swarmClient.SendTo(role, content)
				}

				if sendErr != nil {
					if m.messages != nil {
						m.messages.AddMessage(components.Message{
							Role:    "error",
							Content: fmt.Sprintf("Failed to send to %s: %v", role, sendErr),
						})
					}
				} else {
					// Add to swarm panel
					if m.swarmPanel != nil {
						// Truncate for display in panel
						displayContent := content
						if len(displayContent) > 100 {
							displayContent = displayContent[:100] + "..."
						}
						ownMsg := swarm.NewMessage(
							m.swarmClient.Room().Code,
							m.swarmClient.Role(),
							role,
							swarm.MsgRequest,
							displayContent,
						)
						m.swarmPanel.AddMessage(ownMsg)
					}
					// Notify in local chat
					if m.messages != nil {
						m.messages.AddMessage(components.Message{
							Role:    "system",
							Content: fmt.Sprintf("→ Sent response to %s", role),
						})
					}
				}
			}
		}
	}

	return cmds
}

// View renders the swarm TUI
func (m SwarmModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	t := theme.Current

	messagesHeight := m.height - layoutHeaderHeight - layoutStatusHeight - layoutEditorHeight - layoutPadding

	// Header
	header := m.header.View()

	// Messages area (left panel)
	messagesView := m.messages.View()
	if m.thinking {
		thinkingStyle := lipgloss.NewStyle().Foreground(t.Primary)
		messagesView = lipgloss.NewStyle().
			Height(messagesHeight).
			Render(messagesView + "\n" + thinkingStyle.Render(m.spinner.View()+" Thinking..."))
	} else {
		messagesView = lipgloss.NewStyle().
			Height(messagesHeight).
			Render(messagesView)
	}

	// Editor
	editor := m.editor.View()

	// Left panel (local chat)
	leftPanel := lipgloss.JoinVertical(
		lipgloss.Left,
		messagesView,
		editor,
	)

	// Build main content area
	var mainContent string
	if m.showSwarm && m.swarmPanel != nil {
		// Split view with swarm panel on right
		rightPanel := m.swarmPanel.View()
		mainContent = lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftPanel,
			rightPanel,
		)
	} else {
		mainContent = leftPanel
	}

	// Status bar
	status := m.status.View()

	// Stack everything
	view := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		mainContent,
		status,
	)

	// Overlay help dialog if shown
	if m.showHelp {
		overlay := m.help.View()
		view = components.PlaceOverlay(overlay, view, m.width, m.height)
	}

	// Apply background
	return lipgloss.NewStyle().
		Background(t.Background).
		Width(m.width).
		Height(m.height).
		Render(view)
}
