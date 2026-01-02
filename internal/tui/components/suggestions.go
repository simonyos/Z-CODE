package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/simonyos/Z-CODE/internal/tui/theme"
)

// Command represents a slash command
type Command struct {
	Name        string
	Description string
	IsCustom    bool   // True for custom agent/workflow commands
	AgentName   string // For custom agent commands
}

// BuiltinCommands lists all built-in slash commands
var BuiltinCommands = []Command{
	{Name: "/help", Description: "Show keyboard shortcuts and commands"},
	{Name: "/clear", Description: "Clear chat history"},
	{Name: "/reset", Description: "Reset conversation and context"},
	{Name: "/tools", Description: "List available tools"},
	{Name: "/config", Description: "Show or set configuration"},
	{Name: "/agents", Description: "List custom agents"},
	{Name: "/skills", Description: "List skills"},
	{Name: "/workflows", Description: "List workflows"},
	{Name: "/quit", Description: "Exit Z-Code"},
}

// AvailableCommands is kept for backward compatibility
var AvailableCommands = BuiltinCommands

// CommandProvider provides dynamic commands
type CommandProvider interface {
	GetAgentCommands() []Command
	GetSkillCommands() []Command
	GetWorkflowCommands() []Command
}

// Suggestions shows command autocomplete suggestions
type Suggestions struct {
	visible         bool
	commands        []Command
	selected        int
	width           int
	commandProvider CommandProvider
}

// NewSuggestions creates a new suggestions component
func NewSuggestions() *Suggestions {
	return &Suggestions{
		commands: BuiltinCommands,
		selected: 0,
	}
}

// SetCommandProvider sets the provider for dynamic commands
func (s *Suggestions) SetCommandProvider(provider CommandProvider) {
	s.commandProvider = provider
}

// SetWidth sets the component width
func (s *Suggestions) SetWidth(width int) {
	s.width = width
}

// Filter filters commands based on input
func (s *Suggestions) Filter(input string) {
	if !strings.HasPrefix(input, "/") {
		s.visible = false
		return
	}

	s.visible = true
	s.commands = []Command{}

	// Add built-in commands
	for _, cmd := range BuiltinCommands {
		if strings.HasPrefix(cmd.Name, input) {
			s.commands = append(s.commands, cmd)
		}
	}

	// Add dynamic commands from provider
	if s.commandProvider != nil {
		// Add custom agent commands
		for _, cmd := range s.commandProvider.GetAgentCommands() {
			if strings.HasPrefix(cmd.Name, input) {
				s.commands = append(s.commands, cmd)
			}
		}

		// Add skill commands
		for _, cmd := range s.commandProvider.GetSkillCommands() {
			if strings.HasPrefix(cmd.Name, input) {
				s.commands = append(s.commands, cmd)
			}
		}

		// Add workflow commands
		for _, cmd := range s.commandProvider.GetWorkflowCommands() {
			if strings.HasPrefix(cmd.Name, input) {
				s.commands = append(s.commands, cmd)
			}
		}
	}

	// Reset selection if out of bounds
	if s.selected >= len(s.commands) {
		s.selected = 0
	}
}

// IsVisible returns whether suggestions are showing
func (s *Suggestions) IsVisible() bool {
	return s.visible && len(s.commands) > 0
}

// Hide hides the suggestions
func (s *Suggestions) Hide() {
	s.visible = false
}

// MoveUp moves selection up
func (s *Suggestions) MoveUp() {
	if s.selected > 0 {
		s.selected--
	}
}

// MoveDown moves selection down
func (s *Suggestions) MoveDown() {
	if s.selected < len(s.commands)-1 {
		s.selected++
	}
}

// GetSelected returns the currently selected command
func (s *Suggestions) GetSelected() string {
	if len(s.commands) > 0 && s.selected < len(s.commands) {
		return s.commands[s.selected].Name
	}
	return ""
}

// View renders the suggestions
func (s *Suggestions) View() string {
	if !s.visible || len(s.commands) == 0 {
		return ""
	}

	t := theme.Current

	var sb strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Italic(true)
	sb.WriteString(headerStyle.Render("Commands") + "\n")

	for i, cmd := range s.commands {
		// Command name with icon
		iconStyle := lipgloss.NewStyle().
			Foreground(t.Primary)

		nameStyle := lipgloss.NewStyle().
			Foreground(t.Accent).
			Bold(true).
			Width(12)

		descStyle := lipgloss.NewStyle().
			Foreground(t.TextMuted)

		icon := "  "
		if i == s.selected {
			icon = "› "
		}

		row := iconStyle.Render(icon) + nameStyle.Render(cmd.Name) + descStyle.Render(cmd.Description)

		if i == s.selected {
			row = lipgloss.NewStyle().
				Background(t.BackgroundSecondary).
				Foreground(t.Text).
				Width(s.width - 6).
				Render(row)
		}

		sb.WriteString(row + "\n")
	}

	// Footer hint
	footerStyle := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Italic(true)
	sb.WriteString(footerStyle.Render("↑↓ navigate • Tab to complete • Esc to cancel"))

	// Container
	container := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Background(t.Background).
		Padding(0, 1).
		Width(s.width - 2)

	return container.Render(sb.String())
}
