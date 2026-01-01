package components

import (
	"strings"

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

	// Separator line
	sepStyle := lipgloss.NewStyle().
		Foreground(t.Border)
	separator := sepStyle.Render(strings.Repeat("─", s.Width))

	// Left side: Keyboard hints with better formatting
	hintKeyStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.BackgroundSecondary).
		Padding(0, 1)
	hintTextStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted)

	hints := []struct {
		key  string
		desc string
	}{
		{"Enter", "send"},
		{"/", "commands"},
		{"Ctrl+L", "clear"},
		{"Ctrl+C", "quit"},
	}

	var hintParts []string
	for _, h := range hints {
		hintParts = append(hintParts, hintKeyStyle.Render(h.key)+hintTextStyle.Render(" "+h.desc))
	}
	hintBar := strings.Join(hintParts, hintTextStyle.Render("  "))

	// Right side: Model and status
	var rightContent string
	if s.Thinking {
		// Animated thinking indicator
		thinkStyle := lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true)
		rightContent = thinkStyle.Render("◐ Processing...")
	} else {
		// Model badge with icon
		modelStyle := lipgloss.NewStyle().
			Foreground(t.Primary).
			Background(t.BackgroundSecondary).
			Padding(0, 1).
			Bold(true)
		rightContent = modelStyle.Render("⚡ " + s.Model)
	}

	// Calculate spacing
	leftWidth := lipgloss.Width(hintBar)
	rightWidth := lipgloss.Width(rightContent)
	spacing := s.Width - leftWidth - rightWidth - 2

	if spacing < 0 {
		spacing = 0
	}

	// Build the status line
	statusLine := lipgloss.JoinHorizontal(
		lipgloss.Center,
		hintBar,
		lipgloss.NewStyle().Width(spacing).Render(""),
		rightContent,
	)

	return separator + "\n" + statusLine
}
