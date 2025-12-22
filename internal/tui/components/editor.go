package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/simonyos/Z-CODE/internal/tui/theme"
)

// Editor is the message input component
type Editor struct {
	textarea textarea.Model
	width    int
	height   int
	focused  bool
}

// NewEditor creates a new editor component
func NewEditor(width, height int) *Editor {
	ta := textarea.New()
	ta.Placeholder = "Describe your task..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.SetWidth(width - 6) // Account for prompt and padding
	ta.SetHeight(height - 2)
	ta.ShowLineNumbers = false
	ta.CharLimit = 0

	// Style the textarea - Claude aesthetic
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(theme.Current.TextMuted)

	return &Editor{
		textarea: ta,
		width:    width,
		height:   height,
		focused:  true,
	}
}

// SetSize updates the editor dimensions
func (e *Editor) SetSize(width, height int) {
	e.width = width
	e.height = height
	e.textarea.SetWidth(width - 6)
	e.textarea.SetHeight(height - 2)
}

// Focus focuses the editor
func (e *Editor) Focus() {
	e.focused = true
	e.textarea.Focus()
}

// Blur unfocuses the editor
func (e *Editor) Blur() {
	e.focused = false
	e.textarea.Blur()
}

// Value returns the current text (filtering out any escape sequences)
func (e *Editor) Value() string {
	val := e.textarea.Value()
	// Filter out OSC escape sequences that may leak from terminal
	if strings.Contains(val, "\x1b]") || strings.Contains(val, "]11;") {
		// Clean the value
		val = strings.ReplaceAll(val, "\x1b", "")
		// Remove anything that looks like OSC response
		for strings.Contains(val, "]") && strings.Contains(val, ";") {
			start := strings.Index(val, "]")
			end := strings.Index(val[start:], "\x07") // Bell character ends OSC
			if end == -1 {
				end = strings.Index(val[start:], "\x1b\\") // Or ESC backslash
			}
			if end == -1 {
				// Just remove to end of string or next space
				end = strings.IndexAny(val[start:], " \n\t")
				if end == -1 {
					val = val[:start]
					break
				}
			}
			val = val[:start] + val[start+end+1:]
		}
	}
	return strings.TrimSpace(val)
}

// Reset clears the editor
func (e *Editor) Reset() {
	e.textarea.Reset()
}

// SetValue sets the editor content
func (e *Editor) SetValue(value string) {
	e.textarea.SetValue(value)
}

// Update handles textarea updates
func (e *Editor) Update(msg tea.Msg) (*Editor, tea.Cmd) {
	var cmd tea.Cmd
	e.textarea, cmd = e.textarea.Update(msg)
	return e, cmd
}

// View renders the editor
func (e *Editor) View() string {
	t := theme.Current

	// Textarea
	textareaView := e.textarea.View()

	// Container with rounded border - Claude style
	borderColor := t.Border
	if e.focused {
		borderColor = t.BorderFocus
	}

	container := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(e.width - 2).
		Padding(0, 1)

	return container.Render(textareaView)
}
