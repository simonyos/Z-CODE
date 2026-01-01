package components

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/simonyos/Z-CODE/internal/tui/theme"
)

// Header renders the application header
type Header struct {
	Width   int
	Version string
	CWD     string
}

// NewHeader creates a new header component
func NewHeader(width int, version, cwd string) *Header {
	return &Header{
		Width:   width,
		Version: version,
		CWD:     cwd,
	}
}

// SetWidth updates the header width
func (h *Header) SetWidth(width int) {
	h.Width = width
}

// View renders the header
func (h *Header) View() string {
	t := theme.Current

	// Logo/brand with Z icon
	logoStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	logo := logoStyle.Render("⚡ Z-Code")

	// Version badge - subtle pill style
	versionStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.BackgroundSecondary).
		Padding(0, 1).
		Render(fmt.Sprintf("v%s", h.Version))

	// Working directory - show project name prominently with path
	projectName := filepath.Base(h.CWD)
	projectStyle := lipgloss.NewStyle().
		Foreground(t.Text).
		Bold(true)

	// Show shortened path for context
	parentDir := filepath.Dir(h.CWD)
	maxParentLen := 25
	if len(parentDir) > maxParentLen {
		parentDir = "..." + parentDir[len(parentDir)-maxParentLen+3:]
	}

	pathStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)

	cwdDisplay := pathStyle.Render(parentDir+"/") + projectStyle.Render(projectName)

	// Git branch indicator (placeholder - could be enhanced)
	branchStyle := lipgloss.NewStyle().
		Foreground(t.Success)
	gitIndicator := branchStyle.Render("●")

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
		gitIndicator,
		" ",
		cwdDisplay,
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
