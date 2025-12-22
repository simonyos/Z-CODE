package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/simonyos/Z-CODE/internal/tui/theme"
)

// Status renders the status bar at the bottom
type Status struct {
	Width      int
	Model      string
	Thinking   bool
	Message    string
	TokenCount int
}

// NewStatus creates a new status bar
func NewStatus(width int) *Status {
	return &Status{
		Width: width,
		Model: "claude",
	}
}

// SetWidth updates the status bar width
func (s *Status) SetWidth(width int) {
	s.Width = width
}

// SetThinking sets the thinking state
func (s *Status) SetThinking(thinking bool) {
	s.Thinking = thinking
}

// SetMessage sets the status message
func (s *Status) SetMessage(msg string) {
	s.Message = msg
}

// SetModel sets the model name
func (s *Status) SetModel(model string) {
	s.Model = model
}

// View renders the status bar
func (s *Status) View() string {
	t := theme.Current

	// Minimalist status - just hints
	hintStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)

	hint := hintStyle.Render("Enter to send · Ctrl+C to quit")

	// Model badge
	modelStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.BackgroundSecondary).
		Padding(0, 1)
	modelBadge := modelStyle.Render(s.Model)

	// Thinking indicator or model
	var rightContent string
	if s.Thinking {
		thinkStyle := lipgloss.NewStyle().
			Foreground(t.Primary)
		rightContent = thinkStyle.Render("● thinking...")
	} else {
		rightContent = modelBadge
	}

	// Calculate spacing
	leftWidth := lipgloss.Width(hint)
	rightWidth := lipgloss.Width(rightContent)
	spacing := s.Width - leftWidth - rightWidth - 2

	if spacing < 0 {
		spacing = 0
	}

	// Build the bar
	bar := lipgloss.JoinHorizontal(
		lipgloss.Center,
		hint,
		lipgloss.NewStyle().Width(spacing).Render(""),
		rightContent,
	)

	return bar
}
