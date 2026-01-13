package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/simonyos/Z-CODE/internal/swarm"
	"github.com/simonyos/Z-CODE/internal/tui/theme"
)

// SwarmMessage represents a message in the swarm panel
type SwarmMessage struct {
	ID        string
	Timestamp time.Time
	From      swarm.Role
	To        swarm.Role
	Type      swarm.MessageType
	Content   string
	IsToMe    bool   // Highlight if addressed to current role
	IsFromMe  bool   // Show as "[YOU SENT]"
	Expanded  bool   // Show full content
}

// SwarmPanel displays swarm messages and presence
type SwarmPanel struct {
	viewport    viewport.Model
	messages    []SwarmMessage
	presence    map[swarm.Role]swarm.PresenceStatus
	currentRole swarm.Role
	roomCode    string
	width       int
	height      int
	ready       bool
	focused     bool
	selected    int
	unread      int
}

// NewSwarmPanel creates a new swarm panel
func NewSwarmPanel(width, height int, role swarm.Role, roomCode string) *SwarmPanel {
	vp := viewport.New(width, height-3) // Reserve space for header and presence bar

	return &SwarmPanel{
		viewport:    vp,
		messages:    []SwarmMessage{},
		presence:    make(map[swarm.Role]swarm.PresenceStatus),
		currentRole: role,
		roomCode:    roomCode,
		width:       width,
		height:      height,
		ready:       true,
	}
}

// SetSize updates the panel dimensions
func (p *SwarmPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
	p.viewport.Width = width
	p.viewport.Height = height - 3 // Header + presence bar
	p.updateContent()
}

// SetFocused sets whether the panel is focused
func (p *SwarmPanel) SetFocused(focused bool) {
	p.focused = focused
}

// IsFocused returns whether the panel is focused
func (p *SwarmPanel) IsFocused() bool {
	return p.focused
}

// AddMessage adds a new swarm message
func (p *SwarmPanel) AddMessage(msg *swarm.Message) {
	sm := SwarmMessage{
		ID:        msg.ID,
		Timestamp: msg.Timestamp,
		From:      msg.From,
		To:        msg.To,
		Type:      msg.Type,
		Content:   msg.Content,
		IsToMe:    msg.IsToRole(p.currentRole) && !msg.IsFromRole(p.currentRole),
		IsFromMe:  msg.IsFromRole(p.currentRole),
	}

	p.messages = append(p.messages, sm)

	// Increment unread if not focused and message is to us
	if !p.focused && sm.IsToMe {
		p.unread++
	}

	p.updateContent()
}

// ClearUnread clears the unread count
func (p *SwarmPanel) ClearUnread() {
	p.unread = 0
}

// GetUnread returns the unread message count
func (p *SwarmPanel) GetUnread() int {
	return p.unread
}

// UpdatePresence updates the presence status for a role
func (p *SwarmPanel) UpdatePresence(role swarm.Role, status swarm.PresenceStatus) {
	p.presence[role] = status
	p.updateContent()
}

// SetPresenceFromEvent updates presence from a presence event
func (p *SwarmPanel) SetPresenceFromEvent(event *swarm.PresenceEvent) {
	p.presence[event.Role] = event.Status
	p.updateContent()
}

// MoveUp scrolls up or selects previous message
func (p *SwarmPanel) MoveUp() {
	if p.selected > 0 {
		p.selected--
		p.updateContent()
	}
}

// MoveDown scrolls down or selects next message
func (p *SwarmPanel) MoveDown() {
	if p.selected < len(p.messages)-1 {
		p.selected++
		p.updateContent()
	}
}

// ToggleExpand toggles expansion of selected message
func (p *SwarmPanel) ToggleExpand() {
	if p.selected >= 0 && p.selected < len(p.messages) {
		p.messages[p.selected].Expanded = !p.messages[p.selected].Expanded
		p.updateContent()
	}
}

// GetViewport returns the viewport for handling scroll input
func (p *SwarmPanel) GetViewport() *viewport.Model {
	return &p.viewport
}

// updateContent rebuilds the viewport content
func (p *SwarmPanel) updateContent() {
	if !p.ready {
		return
	}

	t := theme.Current
	contentWidth := p.width - 4

	var sb strings.Builder

	// Render each message
	for i, msg := range p.messages {
		// Time and route header
		timeStr := msg.Timestamp.Format("15:04")

		headerStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
		fromStyle := lipgloss.NewStyle().Foreground(p.roleColor(msg.From)).Bold(true)
		toStyle := lipgloss.NewStyle().Foreground(p.roleColor(msg.To))

		header := headerStyle.Render(timeStr+" ") +
			fromStyle.Render(string(msg.From)) +
			headerStyle.Render(" → ") +
			toStyle.Render(string(msg.To))

		// Add indicator for messages to me
		if msg.IsToMe {
			indicatorStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
			header += indicatorStyle.Render(" ◀──")
		}

		// Selection highlight
		if p.focused && i == p.selected {
			header = lipgloss.NewStyle().
				Background(t.Primary).
				Foreground(t.Background).
				Render(header)
		}

		sb.WriteString(header + "\n")

		// Content box
		content := msg.Content
		maxLen := 100
		if !msg.Expanded && len(content) > maxLen {
			content = content[:maxLen] + "..."
		}

		// Mark as sent by us
		if msg.IsFromMe {
			content = "[YOU SENT] " + content
		}

		// Message box style
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.messageTypeColor(msg.Type)).
			Padding(0, 1).
			Width(contentWidth - 2)

		sb.WriteString(boxStyle.Render(content) + "\n\n")
	}

	// If no messages, show empty state
	if len(p.messages) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Italic(true)
		sb.WriteString(emptyStyle.Render("No swarm messages yet.\n"))
		sb.WriteString(emptyStyle.Render("Use @ROLE to send messages."))
	}

	p.viewport.SetContent(sb.String())
	p.viewport.GotoBottom()
}

// roleColor returns the color for a role
func (p *SwarmPanel) roleColor(role swarm.Role) lipgloss.Color {
	t := theme.Current
	switch role {
	case swarm.RoleOrchestrator:
		return t.Primary
	case swarm.RoleSA:
		return t.Info
	case swarm.RoleBEDev:
		return t.Success
	case swarm.RoleFEDev:
		return t.Warning
	case swarm.RoleQA:
		return t.Accent
	case swarm.RoleHuman:
		return t.Error
	case swarm.RoleAll:
		return t.Text
	default:
		return t.TextMuted
	}
}

// messageTypeColor returns border color based on message type
func (p *SwarmPanel) messageTypeColor(msgType swarm.MessageType) lipgloss.Color {
	t := theme.Current
	switch msgType {
	case swarm.MsgApproval:
		return t.Success
	case swarm.MsgRejection:
		return t.Error
	case swarm.MsgRequest:
		return t.Info
	case swarm.MsgStatus:
		return t.Warning
	case swarm.MsgHumanOverride:
		return t.Error
	default:
		return t.Border
	}
}

// renderPresence renders the presence bar
func (p *SwarmPanel) renderPresence() string {
	t := theme.Current

	var parts []string
	for _, role := range swarm.AllRoles() {
		status, exists := p.presence[role]
		if !exists {
			status = swarm.PresenceOffline
		}

		var icon string
		var color lipgloss.Color
		switch status {
		case swarm.PresenceOnline:
			icon = "●"
			color = t.Success
		case swarm.PresenceBusy, swarm.PresenceTyping:
			icon = "◐"
			color = t.Warning
		default:
			icon = "○"
			color = t.TextMuted
		}

		iconStyle := lipgloss.NewStyle().Foreground(color)
		roleStyle := lipgloss.NewStyle().Foreground(t.TextMuted)

		// Highlight current role
		if role == p.currentRole {
			roleStyle = roleStyle.Bold(true).Foreground(t.Primary)
		}

		parts = append(parts, iconStyle.Render(icon)+" "+roleStyle.Render(string(role)))
	}

	return strings.Join(parts, "  ")
}

// View renders the swarm panel
func (p *SwarmPanel) View() string {
	if !p.ready {
		return ""
	}

	t := theme.Current

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Width(p.width).
		Align(lipgloss.Center)

	title := "SWARM MESSAGES"
	if p.unread > 0 {
		title = fmt.Sprintf("SWARM MESSAGES (%d new) 🔴", p.unread)
	}
	header := headerStyle.Render(title)

	// Separator
	sepStyle := lipgloss.NewStyle().Foreground(t.Border)
	separator := sepStyle.Render(strings.Repeat("─", p.width))

	// Messages viewport
	messages := p.viewport.View()

	// Presence bar
	presence := p.renderPresence()
	presenceStyle := lipgloss.NewStyle().
		Width(p.width).
		Align(lipgloss.Center)
	presenceBar := presenceStyle.Render(presence)

	// Border style for the panel
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Width(p.width - 2).
		Height(p.height - 2)

	if p.focused {
		borderStyle = borderStyle.BorderForeground(t.Primary)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		separator,
		messages,
		presenceBar,
	)

	return borderStyle.Render(content)
}
