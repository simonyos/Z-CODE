package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/simonyos/Z-CODE/internal/swarm"
	"github.com/simonyos/Z-CODE/internal/tui/theme"
)

// SwarmHeader renders the header for swarm mode
type SwarmHeader struct {
	Width    int
	Version  string
	RoomCode string
	RoomName string
	Role     swarm.Role
}

// NewSwarmHeader creates a new swarm header component
func NewSwarmHeader(width int, version string, roomCode, roomName string, role swarm.Role) *SwarmHeader {
	return &SwarmHeader{
		Width:    width,
		Version:  version,
		RoomCode: roomCode,
		RoomName: roomName,
		Role:     role,
	}
}

// SetWidth updates the header width
func (h *SwarmHeader) SetWidth(width int) {
	h.Width = width
}

// View renders the header
func (h *SwarmHeader) View() string {
	t := theme.Current

	// Logo with HiveMind branding
	logoStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	logo := logoStyle.Render("🐝 HiveMind")

	// Version badge
	versionStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.BackgroundSecondary).
		Padding(0, 1).
		Render(fmt.Sprintf("v%s", h.Version))

	// Room info - show room code prominently
	roomStyle := lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)

	roomDisplay := labelStyle.Render("Room: ") + roomStyle.Render(h.RoomCode)

	// Role badge
	roleStyle := lipgloss.NewStyle().
		Foreground(t.Background).
		Background(h.roleColor()).
		Padding(0, 1).
		Bold(true)

	roleBadge := roleStyle.Render(string(h.Role))

	// Build left section
	leftPart := lipgloss.JoinHorizontal(
		lipgloss.Center,
		logo,
		"  ",
		versionStyle,
	)

	// Build right section
	rightPart := lipgloss.JoinHorizontal(
		lipgloss.Center,
		roomDisplay,
		"  ",
		roleBadge,
	)

	// Calculate spacing
	spacing := h.Width - lipgloss.Width(leftPart) - lipgloss.Width(rightPart) - 2
	if spacing < 1 {
		spacing = 1
	}

	header := lipgloss.JoinHorizontal(
		lipgloss.Center,
		leftPart,
		lipgloss.NewStyle().Width(spacing).Render(""),
		rightPart,
	)

	// Add a subtle separator line
	separator := lipgloss.NewStyle().
		Foreground(t.Border).
		Width(h.Width).
		Render(strings.Repeat("─", h.Width))

	return header + "\n" + separator
}

// roleColor returns the color for the current role
func (h *SwarmHeader) roleColor() lipgloss.Color {
	t := theme.Current
	switch h.Role {
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
	default:
		return t.TextMuted
	}
}
