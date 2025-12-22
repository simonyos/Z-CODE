package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/simonyos/Z-CODE/internal/tui/theme"
)

// Message represents a chat message
type Message struct {
	Role     string // "user", "assistant", "tool", "system", "error"
	Content  string
	ToolName string
	ToolArgs string
}

// Messages is the scrollable message list component
type Messages struct {
	viewport         viewport.Model
	messages         []Message
	renderer         *glamour.TermRenderer
	width            int
	height           int
	ready            bool
	welcome          string
	streamingContent string // Content being streamed
}

// NewMessages creates a new messages component
func NewMessages(width, height int) *Messages {
	// Use dark style explicitly to avoid terminal color queries
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(width-10),
	)

	// Initialize viewport immediately so content can be set
	vp := viewport.New(width, height)

	return &Messages{
		viewport: vp,
		messages: []Message{},
		renderer: renderer,
		width:    width,
		height:   height,
		ready:    true,
	}
}

// SetSize updates the component dimensions
func (m *Messages) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height

	// Update renderer word wrap - use dark style to avoid terminal queries
	m.renderer, _ = glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(width-10),
	)

	m.updateContent()
}

// AddMessage adds a new message
func (m *Messages) AddMessage(msg Message) {
	m.messages = append(m.messages, msg)
	m.updateContent()
}

// Clear removes all messages
func (m *Messages) Clear() {
	m.messages = []Message{}
	m.updateContent()
}

// GetViewport returns the viewport for handling scroll input
func (m *Messages) GetViewport() *viewport.Model {
	return &m.viewport
}

// SetWelcome sets the welcome message to show when empty
func (m *Messages) SetWelcome(welcome string) {
	m.welcome = welcome
	m.updateContent()
}

// UpdateStreaming updates the streaming content display
func (m *Messages) UpdateStreaming(content string) {
	m.streamingContent = content
	m.updateContent()
}

// ClearStreaming clears the streaming content
func (m *Messages) ClearStreaming() {
	m.streamingContent = ""
	m.updateContent()
}

// UpdateLastToolResult updates the result of the last tool message
func (m *Messages) UpdateLastToolResult(result string) {
	// Find the last tool message and update its content
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "tool" {
			m.messages[i].Content = result
			break
		}
	}
	m.updateContent()
}

// updateContent rebuilds the viewport content
func (m *Messages) updateContent() {
	if !m.ready {
		return
	}

	t := theme.Current
	var sb strings.Builder
	contentWidth := m.width - 4 // Account for borders/padding

	// Show welcome message if no messages
	if len(m.messages) == 0 && m.welcome != "" {
		// Warm welcome header
		welcomeHeader := lipgloss.NewStyle().
			Foreground(t.TextInverse).
			Background(t.Primary).
			Padding(0, 1).
			Bold(true)
		sb.WriteString(welcomeHeader.Render(" Welcome ") + "\n\n")

		// Tips in muted style
		tipsStyle := lipgloss.NewStyle().
			Foreground(t.TextMuted)
		tips := `  I'm Claude, your AI coding assistant.

  • Describe what you'd like to build or fix
  • Ask me to read, write, or run commands
  • Type /help for available commands

  What can I help you with?`
		sb.WriteString(tipsStyle.Render(tips))

		m.viewport.SetContent(sb.String())
		return
	}

	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			// User message - Claude style with header badge
			headerStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E0E0E0")).
				Background(lipgloss.Color("#4D4D4D")).
				Padding(0, 1).
				Bold(true)
			sb.WriteString(headerStyle.Render("YOU") + "\n")

			bodyStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E0E0E0")).
				PaddingLeft(1).
				Width(contentWidth)
			sb.WriteString(bodyStyle.Render(msg.Content) + "\n\n")

		case "assistant":
			// Assistant message - Claude style with warm accent header
			headerStyle := lipgloss.NewStyle().
				Foreground(t.TextInverse).
				Background(t.Primary).
				Padding(0, 1).
				Bold(true)
			sb.WriteString(headerStyle.Render("CLAUDE") + "\n")

			// Render markdown
			rendered := msg.Content
			if m.renderer != nil {
				if r, err := m.renderer.Render(msg.Content); err == nil {
					rendered = strings.TrimSpace(r)
				}
			}

			bodyStyle := lipgloss.NewStyle().
				Foreground(t.Text).
				PaddingLeft(1).
				Width(contentWidth)
			sb.WriteString(bodyStyle.Render(rendered) + "\n\n")

		case "tool":
			// Tool call - subtle styling
			toolHeader := lipgloss.NewStyle().
				Foreground(t.TextMuted).
				Background(t.BackgroundSecondary).
				Padding(0, 1).
				Italic(true)
			sb.WriteString(toolHeader.Render("⚡ "+msg.ToolName) + "\n")

			// Command/args
			if msg.ToolArgs != "" {
				cmdStyle := lipgloss.NewStyle().
					Foreground(t.TextMuted).
					PaddingLeft(2)
				sb.WriteString(cmdStyle.Render("$ "+msg.ToolArgs) + "\n")
			}

			// Result
			if msg.Content != "" {
				result := msg.Content
				if len(result) > 500 {
					result = result[:500] + "\n... (truncated)"
				}

				resultStyle := lipgloss.NewStyle().
					Foreground(t.TextMuted).
					PaddingLeft(2).
					Width(contentWidth - 4)
				sb.WriteString(resultStyle.Render(result) + "\n")
			}
			sb.WriteString("\n")

		case "system":
			// System message - subtle centered
			sysStyle := lipgloss.NewStyle().
				Foreground(t.TextMuted).
				Italic(true).
				PaddingLeft(1)
			sb.WriteString(sysStyle.Render("— "+msg.Content+" —") + "\n\n")

		case "error":
			// Error message
			errStyle := lipgloss.NewStyle().
				Foreground(t.Error).
				Bold(true)
			sb.WriteString(errStyle.Render("✗ "+msg.Content) + "\n\n")
		}
	}

	// Show streaming content if any
	if m.streamingContent != "" {
		// Claude style header for streaming
		headerStyle := lipgloss.NewStyle().
			Foreground(t.TextInverse).
			Background(t.Primary).
			Padding(0, 1).
			Bold(true)
		sb.WriteString(headerStyle.Render("CLAUDE") + "\n")

		// Render streaming content with markdown
		rendered := m.streamingContent
		if m.renderer != nil {
			if r, err := m.renderer.Render(m.streamingContent); err == nil {
				rendered = strings.TrimSpace(r)
			}
		}

		bodyStyle := lipgloss.NewStyle().
			Foreground(t.Text).
			PaddingLeft(1).
			Width(contentWidth)
		sb.WriteString(bodyStyle.Render(rendered+"▌") + "\n\n") // Cursor indicator
	}

	m.viewport.SetContent(sb.String())
	m.viewport.GotoBottom()
}

// View renders the messages
func (m *Messages) View() string {
	if !m.ready {
		return ""
	}
	return m.viewport.View()
}
