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
		// Centered welcome with ASCII art logo
		logoStyle := lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)

		logo := `
   ███████╗       ██████╗ ██████╗ ██████╗ ███████╗
   ╚══███╔╝      ██╔════╝██╔═══██╗██╔══██╗██╔════╝
     ███╔╝ █████╗██║     ██║   ██║██║  ██║█████╗
    ███╔╝  ╚════╝██║     ██║   ██║██║  ██║██╔══╝
   ███████╗      ╚██████╗╚██████╔╝██████╔╝███████╗
   ╚══════╝       ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝`

		sb.WriteString(logoStyle.Render(logo) + "\n\n")

		// Tagline
		taglineStyle := lipgloss.NewStyle().
			Foreground(t.Text).
			Bold(true)
		sb.WriteString(taglineStyle.Render("   AI-Powered Coding Assistant") + "\n\n")

		// Separator
		sepStyle := lipgloss.NewStyle().
			Foreground(t.Border)
		sb.WriteString(sepStyle.Render("   " + strings.Repeat("─", 40)) + "\n\n")

		// Quick start tips with icons
		tipHeaderStyle := lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
		sb.WriteString(tipHeaderStyle.Render("   Quick Start") + "\n\n")

		tipStyle := lipgloss.NewStyle().
			Foreground(t.TextMuted)
		iconStyle := lipgloss.NewStyle().
			Foreground(t.Accent)

		tips := []struct {
			icon string
			text string
		}{
			{"📝", "Describe what you want to build or fix"},
			{"📖", "Ask me to read and explain code"},
			{"⚡", "Let me run commands and edit files"},
			{"🔍", "Search the codebase with glob and grep"},
		}

		for _, tip := range tips {
			sb.WriteString("   " + iconStyle.Render(tip.icon) + " " + tipStyle.Render(tip.text) + "\n")
		}

		sb.WriteString("\n")

		// Commands hint
		cmdStyle := lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Italic(true)
		sb.WriteString(cmdStyle.Render("   Type /help for commands • Enter to send") + "\n")

		m.viewport.SetContent(sb.String())
		return
	}

	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			// User message with avatar-style icon
			iconStyle := lipgloss.NewStyle().
				Foreground(t.Info).
				Bold(true)
			headerStyle := lipgloss.NewStyle().
				Foreground(t.Text).
				Bold(true)
			sb.WriteString(iconStyle.Render("◉") + " " + headerStyle.Render("You") + "\n")

			bodyStyle := lipgloss.NewStyle().
				Foreground(t.Text).
				PaddingLeft(2).
				Width(contentWidth)
			sb.WriteString(bodyStyle.Render(msg.Content) + "\n\n")

		case "assistant":
			// Assistant message with Z-Code branding
			iconStyle := lipgloss.NewStyle().
				Foreground(t.Primary).
				Bold(true)
			headerStyle := lipgloss.NewStyle().
				Foreground(t.Primary).
				Bold(true)
			sb.WriteString(iconStyle.Render("⚡") + " " + headerStyle.Render("Z-Code") + "\n")

			// Render markdown
			rendered := msg.Content
			if m.renderer != nil {
				if r, err := m.renderer.Render(msg.Content); err == nil {
					rendered = strings.TrimSpace(r)
				}
			}

			bodyStyle := lipgloss.NewStyle().
				Foreground(t.Text).
				PaddingLeft(2).
				Width(contentWidth)
			sb.WriteString(bodyStyle.Render(rendered) + "\n\n")

		case "tool":
			// Tool execution with progress-style indicator
			isRunning := msg.Content == "Running..."
			isError := strings.HasPrefix(msg.Content, "Error:")

			var statusIcon string
			var statusColor lipgloss.Color
			if isRunning {
				statusIcon = "◐"
				statusColor = t.Warning
			} else if isError {
				statusIcon = "✗"
				statusColor = t.Error
			} else {
				statusIcon = "✓"
				statusColor = t.Success
			}

			// Tool header with status
			iconStyle := lipgloss.NewStyle().
				Foreground(statusColor).
				Bold(true)
			toolNameStyle := lipgloss.NewStyle().
				Foreground(t.TextMuted).
				Bold(true)

			sb.WriteString("  " + iconStyle.Render(statusIcon) + " " + toolNameStyle.Render(msg.ToolName))

			// Command/args inline
			if msg.ToolArgs != "" {
				argsStyle := lipgloss.NewStyle().
					Foreground(t.TextMuted)
				sb.WriteString(argsStyle.Render(" → " + msg.ToolArgs))
			}
			sb.WriteString("\n")

			// Result (if not running and has content)
			if !isRunning && msg.Content != "" {
				result := msg.Content
				maxResultLen := 300
				if len(result) > maxResultLen {
					result = result[:maxResultLen] + "\n    ⋯ (truncated)"
				}

				resultStyle := lipgloss.NewStyle().
					Foreground(t.TextMuted).
					PaddingLeft(4).
					Width(contentWidth - 6)

				// Add a subtle box for output
				boxStyle := lipgloss.NewStyle().
					Foreground(t.Border).
					PaddingLeft(4)
				sb.WriteString(boxStyle.Render("│") + "\n")
				sb.WriteString(resultStyle.Render(result) + "\n")
			}
			sb.WriteString("\n")

		case "system":
			// System message with info icon
			iconStyle := lipgloss.NewStyle().
				Foreground(t.Info)
			sysStyle := lipgloss.NewStyle().
				Foreground(t.TextMuted).
				Italic(true)
			sb.WriteString(iconStyle.Render("ℹ") + " " + sysStyle.Render(msg.Content) + "\n\n")

		case "error":
			// Error message with clear visual treatment
			iconStyle := lipgloss.NewStyle().
				Foreground(t.Error).
				Bold(true)
			errStyle := lipgloss.NewStyle().
				Foreground(t.Error)
			sb.WriteString(iconStyle.Render("✗") + " " + errStyle.Render(msg.Content) + "\n\n")
		}
	}

	// Show streaming content if any
	if m.streamingContent != "" {
		// Z-Code style header for streaming
		iconStyle := lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
		headerStyle := lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
		sb.WriteString(iconStyle.Render("⚡") + " " + headerStyle.Render("Z-Code") + "\n")

		// Render streaming content with markdown
		rendered := m.streamingContent
		if m.renderer != nil {
			if r, err := m.renderer.Render(m.streamingContent); err == nil {
				rendered = strings.TrimSpace(r)
			}
		}

		bodyStyle := lipgloss.NewStyle().
			Foreground(t.Text).
			PaddingLeft(2).
			Width(contentWidth)

		// Blinking cursor effect
		cursorStyle := lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
		sb.WriteString(bodyStyle.Render(rendered) + cursorStyle.Render("▌") + "\n\n")
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
