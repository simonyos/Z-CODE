package components

import (
	"fmt"

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

	// Claude-style header with warm background
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D0D0D0")).
		Background(t.Secondary).
		Padding(0, 1).
		Bold(true)

	title := titleStyle.Render(" Z-Code TUI ")

	// Version badge
	versionStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Render(fmt.Sprintf(" v%s", h.Version))

	// Working directory (truncated if too long)
	cwd := h.CWD
	maxCWDLen := h.Width - 30
	if len(cwd) > maxCWDLen && maxCWDLen > 10 {
		cwd = "..." + cwd[len(cwd)-maxCWDLen+3:]
	}

	cwdStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Render(cwd)

	// Build header line
	leftPart := lipgloss.JoinHorizontal(lipgloss.Center, title, versionStyle)

	// Calculate spacing
	spacing := h.Width - lipgloss.Width(leftPart) - lipgloss.Width(cwdStyle) - 2
	if spacing < 1 {
		spacing = 1
	}

	header := lipgloss.JoinHorizontal(
		lipgloss.Center,
		leftPart,
		lipgloss.NewStyle().Width(spacing).Render(""),
		cwdStyle,
	)

	return header
}
