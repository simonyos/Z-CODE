package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SplitPane manages a layout with multiple panels
type SplitPane struct {
	Width  int
	Height int

	// Vertical split ratio (top vs bottom)
	TopRatio float64

	// Horizontal split ratio (left vs right) for top section
	LeftRatio float64

	// Whether right panel is visible
	ShowRight bool
}

// NewSplitPane creates a split pane layout
func NewSplitPane(width, height int) *SplitPane {
	return &SplitPane{
		Width:     width,
		Height:    height,
		TopRatio:  0.85, // 85% for messages, 15% for editor
		LeftRatio: 1.0,  // 100% left (no right panel by default)
		ShowRight: false,
	}
}

// SetSize updates the pane dimensions
func (s *SplitPane) SetSize(width, height int) {
	s.Width = width
	s.Height = height
}

// GetTopHeight returns the height of the top section
func (s *SplitPane) GetTopHeight() int {
	return int(float64(s.Height) * s.TopRatio)
}

// GetBottomHeight returns the height of the bottom section
func (s *SplitPane) GetBottomHeight() int {
	return s.Height - s.GetTopHeight()
}

// GetLeftWidth returns the width of the left panel
func (s *SplitPane) GetLeftWidth() int {
	if !s.ShowRight {
		return s.Width
	}
	return int(float64(s.Width) * s.LeftRatio)
}

// GetRightWidth returns the width of the right panel
func (s *SplitPane) GetRightWidth() int {
	if !s.ShowRight {
		return 0
	}
	return s.Width - s.GetLeftWidth()
}

// Render combines the panels into the final layout
func (s *SplitPane) Render(topLeft, topRight, bottom string) string {
	var result strings.Builder

	topHeight := s.GetTopHeight()
	bottomHeight := s.GetBottomHeight()

	// Build top section
	var topSection string
	if s.ShowRight && topRight != "" {
		leftWidth := s.GetLeftWidth()
		rightWidth := s.GetRightWidth()

		leftStyle := lipgloss.NewStyle().Width(leftWidth).Height(topHeight)
		rightStyle := lipgloss.NewStyle().Width(rightWidth).Height(topHeight)

		topSection = lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftStyle.Render(topLeft),
			rightStyle.Render(topRight),
		)
	} else {
		topStyle := lipgloss.NewStyle().Width(s.Width).Height(topHeight)
		topSection = topStyle.Render(topLeft)
	}

	result.WriteString(topSection)
	result.WriteString("\n")

	// Build bottom section
	bottomStyle := lipgloss.NewStyle().Width(s.Width).Height(bottomHeight)
	result.WriteString(bottomStyle.Render(bottom))

	return result.String()
}

// RenderSimple creates a simple top/bottom split
func (s *SplitPane) RenderSimple(top, bottom string) string {
	return s.Render(top, "", bottom)
}
