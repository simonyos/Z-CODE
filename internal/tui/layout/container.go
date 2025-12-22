package layout

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/simonyos/Z-CODE/internal/tui/theme"
)

// Container wraps content with optional borders and padding
type Container struct {
	// Border on each side
	BorderTop    bool
	BorderRight  bool
	BorderBottom bool
	BorderLeft   bool

	// Padding on each side
	PaddingTop    int
	PaddingRight  int
	PaddingBottom int
	PaddingLeft   int

	// Border style
	BorderStyle lipgloss.Border
	BorderColor lipgloss.Color

	// Sizing
	Width  int
	Height int
}

// ContainerOption configures a container
type ContainerOption func(*Container)

// NewContainer creates a container with options
func NewContainer(opts ...ContainerOption) *Container {
	c := &Container{
		BorderStyle: lipgloss.RoundedBorder(),
		BorderColor: theme.Current.Border,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithBorder sets border on specified sides
func WithBorder(top, right, bottom, left bool) ContainerOption {
	return func(c *Container) {
		c.BorderTop = top
		c.BorderRight = right
		c.BorderBottom = bottom
		c.BorderLeft = left
	}
}

// WithAllBorders enables all borders
func WithAllBorders() ContainerOption {
	return func(c *Container) {
		c.BorderTop = true
		c.BorderRight = true
		c.BorderBottom = true
		c.BorderLeft = true
	}
}

// WithPadding sets padding on all sides
func WithPadding(top, right, bottom, left int) ContainerOption {
	return func(c *Container) {
		c.PaddingTop = top
		c.PaddingRight = right
		c.PaddingBottom = bottom
		c.PaddingLeft = left
	}
}

// WithSize sets the container dimensions
func WithSize(width, height int) ContainerOption {
	return func(c *Container) {
		c.Width = width
		c.Height = height
	}
}

// WithBorderColor sets the border color
func WithBorderColor(color lipgloss.Color) ContainerOption {
	return func(c *Container) {
		c.BorderColor = color
	}
}

// WithBorderStyle sets the border style
func WithBorderStyle(style lipgloss.Border) ContainerOption {
	return func(c *Container) {
		c.BorderStyle = style
	}
}

// Render wraps content with the container styling
func (c *Container) Render(content string) string {
	style := lipgloss.NewStyle()

	// Apply borders
	if c.BorderTop || c.BorderRight || c.BorderBottom || c.BorderLeft {
		style = style.Border(c.BorderStyle, c.BorderTop, c.BorderRight, c.BorderBottom, c.BorderLeft).
			BorderForeground(c.BorderColor)
	}

	// Apply padding
	style = style.
		PaddingTop(c.PaddingTop).
		PaddingRight(c.PaddingRight).
		PaddingBottom(c.PaddingBottom).
		PaddingLeft(c.PaddingLeft)

	// Apply size
	if c.Width > 0 {
		style = style.Width(c.Width)
	}
	if c.Height > 0 {
		style = style.Height(c.Height)
	}

	return style.Render(content)
}

// ThickBorderLeft creates a thick left border style (for message indicators)
func ThickBorderLeft(color lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(color).
		PaddingLeft(1)
}
