// Package ui holds the palette and the drawing helpers shared by every
// screen: cards, chips, the balance bar, toasts and layout primitives.
package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme is the whole palette in one place.
//
// Lip Gloss v2 dropped AdaptiveColor: a style now holds one concrete colour.
// The light and dark choices are therefore made once, when the theme is
// built, from the terminal's actual background — which the root learns
// through tea.RequestBackgroundColor. The same mechanism works over SSH,
// where the server cannot otherwise know what the client's terminal looks
// like.
type Theme struct {
	Dark bool

	Felt   color.Color
	Gold   color.Color
	Chip   color.Color
	Red    color.Color
	Black  color.Color
	Muted  color.Color
	Text   color.Color
	Border color.Color

	Panel lipgloss.Style
	Title lipgloss.Style
	Label lipgloss.Style
	Value lipgloss.Style
	Win   lipgloss.Style
	Lose  lipgloss.Style
	Push  lipgloss.Style
	Key   lipgloss.Style
	Dim   lipgloss.Style
}

// NewTheme builds the green-felt-and-gold palette for a light or dark
// terminal.
func NewTheme(isDark bool) Theme {
	pick := lipgloss.LightDark(isDark)

	t := Theme{
		Dark:   isDark,
		Felt:   pick(lipgloss.Color("#2D6A4F"), lipgloss.Color("#1B4332")),
		Gold:   pick(lipgloss.Color("#B8860B"), lipgloss.Color("#D4AF37")),
		Chip:   pick(lipgloss.Color("#1D3557"), lipgloss.Color("#A8DADC")),
		Red:    pick(lipgloss.Color("#C1121F"), lipgloss.Color("#E5383B")),
		Black:  pick(lipgloss.Color("#212529"), lipgloss.Color("#DEE2E6")),
		Muted:  lipgloss.Color("#6C757D"),
		Text:   pick(lipgloss.Color("#1B1B1B"), lipgloss.Color("#F1F1F1")),
		Border: pick(lipgloss.Color("#95A5A6"), lipgloss.Color("#40916C")),
	}

	t.Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1)
	t.Title = lipgloss.NewStyle().Foreground(t.Gold).Bold(true)
	t.Label = lipgloss.NewStyle().Foreground(t.Muted)
	t.Value = lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	t.Win = lipgloss.NewStyle().Foreground(t.Gold).Bold(true)
	t.Lose = lipgloss.NewStyle().Foreground(t.Red)
	t.Push = lipgloss.NewStyle().Foreground(t.Muted)
	t.Key = lipgloss.NewStyle().Foreground(t.Chip).Bold(true)
	t.Dim = lipgloss.NewStyle().Foreground(t.Muted)
	return t
}

// Default is the dark theme, used until the terminal answers the background
// colour request. Dark is the right guess: it is what most terminals are, and
// a wrong guess is corrected within a frame or two.
func Default() Theme { return NewTheme(true) }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
