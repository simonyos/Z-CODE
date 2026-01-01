package components

import (
	"strings"

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
		Width:  55,
		Height: 24,
	}
}

// View renders the help dialog
func (h *HelpDialog) View() string {
	t := theme.Current

	// Header with icon
	headerStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	title := headerStyle.Render("⚡ Z-Code Help")

	// Separator
	sepStyle := lipgloss.NewStyle().
		Foreground(t.Border)
	separator := sepStyle.Render(strings.Repeat("─", h.Width-6))

	// Keyboard shortcuts section
	sectionStyle := lipgloss.NewStyle().
		Foreground(t.Text).
		Bold(true)
	keyboardSection := sectionStyle.Render("Keyboard Shortcuts")

	keyStyle := lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(t.BackgroundSecondary).
		Padding(0, 1)
	descStyle := lipgloss.NewStyle().
		Foreground(t.Text)

	keyboardShortcuts := []struct {
		key  string
		desc string
	}{
		{"Enter", "Send message"},
		{"Ctrl+C", "Quit Z-Code"},
		{"Ctrl+L", "Clear chat"},
		{"Esc", "Cancel/Close"},
		{"PgUp/PgDn", "Scroll messages"},
	}

	var keyContent string
	for _, s := range keyboardShortcuts {
		keyContent += keyStyle.Render(s.key) + " " + descStyle.Render(s.desc) + "\n"
	}

	// Commands section
	commandSection := sectionStyle.Render("Slash Commands")

	cmdStyle := lipgloss.NewStyle().
		Foreground(t.Accent).
		Bold(true)

	commands := []struct {
		cmd  string
		desc string
	}{
		{"/help", "Show this help dialog"},
		{"/clear", "Clear chat history"},
		{"/reset", "Reset conversation context"},
		{"/tools", "List available tools"},
		{"/config", "View or set configuration"},
		{"/quit", "Exit Z-Code"},
	}

	var cmdContent string
	for _, c := range commands {
		cmdContent += cmdStyle.Render(c.cmd) + " " + descStyle.Render(c.desc) + "\n"
	}

	// Footer
	footerStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Italic(true)
	footer := footerStyle.Render("Press any key to close")

	// Build content
	content := title + "\n" +
		separator + "\n\n" +
		keyboardSection + "\n" +
		keyContent + "\n" +
		commandSection + "\n" +
		cmdContent + "\n" +
		footer

	// Container with subtle shadow effect
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Background(t.Background).
		Padding(1, 2).
		Width(h.Width)

	return box.Render(content)
}

// PlaceOverlay places the dialog centered on the screen
func PlaceOverlay(overlay, _ string, bgWidth, bgHeight int) string {
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
