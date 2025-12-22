package components

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/simonyos/Z-CODE/internal/tui/theme"
)

// HelpDialog shows available keyboard shortcuts
type HelpDialog struct {
	Width  int
	Height int
}

// NewHelpDialog creates a help dialog
func NewHelpDialog() *HelpDialog {
	return &HelpDialog{
		Width:  50,
		Height: 20,
	}
}

// View renders the help dialog
func (h *HelpDialog) View() string {
	t := theme.Current

	// Title
	title := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Render("Keyboard Shortcuts")

	// Shortcuts
	shortcuts := []struct {
		key  string
		desc string
	}{
		{"enter", "Send message"},
		{"ctrl+c", "Quit"},
		{"ctrl+l", "Clear chat"},
		{"esc", "Cancel/Close"},
		{"page up/down", "Scroll messages"},
		{"", ""},
		{"/help", "Show commands"},
		{"/clear", "Clear history"},
		{"/reset", "Reset conversation"},
		{"/tools", "List tools"},
		{"/quit", "Exit"},
	}

	var content string
	for _, s := range shortcuts {
		if s.key == "" {
			content += "\n"
			continue
		}

		key := lipgloss.NewStyle().
			Foreground(t.Accent).
			Bold(true).
			Width(15).
			Render(s.key)

		desc := lipgloss.NewStyle().
			Foreground(t.Text).
			Render(s.desc)

		content += key + desc + "\n"
	}

	// Footer
	footer := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Render("\nPress any key to close")

	// Container
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2).
		Width(h.Width)

	return box.Render(title + "\n\n" + content + footer)
}

// PlaceOverlay places the dialog centered on the screen
func PlaceOverlay(overlay, background string, bgWidth, bgHeight int) string {
	// Simple center placement
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := lipgloss.Height(overlay)

	x := (bgWidth - overlayWidth) / 2
	y := (bgHeight - overlayHeight) / 2

	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	return lipgloss.Place(
		bgWidth,
		bgHeight,
		lipgloss.Center,
		lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(theme.Current.Background),
	)
}
