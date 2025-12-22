package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines all colors for the TUI
type Theme struct {
	// Primary colors
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color

	// Text colors
	Text        lipgloss.Color
	TextMuted   lipgloss.Color
	TextInverse lipgloss.Color

	// Background colors
	Background          lipgloss.Color
	BackgroundSecondary lipgloss.Color

	// Status colors
	Success lipgloss.Color
	Warning lipgloss.Color
	Error   lipgloss.Color
	Info    lipgloss.Color

	// Border colors
	Border       lipgloss.Color
	BorderFocus  lipgloss.Color
	BorderMuted  lipgloss.Color
}

// Current is the active theme
var Current = DefaultTheme()

// DefaultTheme returns the default z-code theme (Claude-inspired warm aesthetic)
func DefaultTheme() Theme {
	return Theme{
		// Primary colors - warm terracotta/sandy accent like Claude
		Primary:   lipgloss.Color("#D2A679"), // Warm sandy/terracotta
		Secondary: lipgloss.Color("#5A4E40"), // Muted warm brown
		Accent:    lipgloss.Color("#D2A679"), // Same warm accent

		// Text colors
		Text:        lipgloss.Color("#F0F0F0"), // Bright white
		TextMuted:   lipgloss.Color("#888888"), // Medium gray
		TextInverse: lipgloss.Color("#1a1a1a"), // Near black

		// Background colors
		Background:          lipgloss.Color("#1a1a1a"), // Dark background
		BackgroundSecondary: lipgloss.Color("#2d2d2d"), // Slightly lighter

		// Status colors
		Success: lipgloss.Color("#10B981"), // Green
		Warning: lipgloss.Color("#F59E0B"), // Amber
		Error:   lipgloss.Color("#EF4444"), // Red
		Info:    lipgloss.Color("#4D4D4D"), // Neutral gray for user

		// Border colors
		Border:      lipgloss.Color("#3d3d3d"), // Subtle border
		BorderFocus: lipgloss.Color("#D2A679"), // Warm accent on focus
		BorderMuted: lipgloss.Color("#2d2d2d"), // Very subtle
	}
}

// TokyoNight returns a Tokyo Night inspired theme
func TokyoNight() Theme {
	return Theme{
		Primary:             lipgloss.Color("#7AA2F7"),
		Secondary:           lipgloss.Color("#9ECE6A"),
		Accent:              lipgloss.Color("#FF9E64"),
		Text:                lipgloss.Color("#C0CAF5"),
		TextMuted:           lipgloss.Color("#565F89"),
		TextInverse:         lipgloss.Color("#1A1B26"),
		Background:          lipgloss.Color("#1A1B26"),
		BackgroundSecondary: lipgloss.Color("#24283B"),
		Success:             lipgloss.Color("#9ECE6A"),
		Warning:             lipgloss.Color("#E0AF68"),
		Error:               lipgloss.Color("#F7768E"),
		Info:                lipgloss.Color("#7AA2F7"),
		Border:              lipgloss.Color("#3B4261"),
		BorderFocus:         lipgloss.Color("#7AA2F7"),
		BorderMuted:         lipgloss.Color("#24283B"),
	}
}
