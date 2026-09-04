package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// ChipDenoms are the chip values, largest last. Bet controls step through
// these, and Chips breaks an amount down into them.
var ChipDenoms = []int64{1, 5, 25, 100, 500}

// chipColor picks the disc colour for a denomination. Colours are resolved
// against the theme rather than being package-level values: Lip Gloss v2 has
// no adaptive colour, so light and dark have to be chosen when the theme is.
func chipColor(denom int64, t Theme) color.Color {
	pick := lipgloss.LightDark(t.Dark)
	switch denom {
	case 1:
		return pick(lipgloss.Color("#495057"), lipgloss.Color("#ADB5BD")) // white
	case 5:
		return pick(lipgloss.Color("#C1121F"), lipgloss.Color("#E5383B")) // red
	case 25:
		return pick(lipgloss.Color("#2D6A4F"), lipgloss.Color("#52B788")) // green
	case 100:
		return pick(lipgloss.Color("#212529"), lipgloss.Color("#868E96")) // black
	case 500:
		return pick(lipgloss.Color("#6A0572"), lipgloss.Color("#C77DFF")) // purple
	default:
		return t.Chip
	}
}

// maxChipGlyphs caps how many discs are drawn before the tail collapses into
// a count, so a large bet cannot overflow the bet track.
const maxChipGlyphs = 10

// ChipStyle returns the style for one denomination.
func ChipStyle(denom int64, t Theme) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(chipColor(denom, t)).Bold(true)
}

// Chip draws a single disc of the given denomination.
func Chip(denom int64, t Theme) string { return ChipStyle(denom, t).Render("●") }

// ChipLabel draws a disc followed by its value, e.g. "● 25".
func ChipLabel(denom int64, t Theme) string {
	return ChipStyle(denom, t).Render("●") + " " + t.Value.Render(fmt.Sprintf("%d", denom))
}

// Chips renders an amount as a stack of coloured discs, largest first.
func Chips(n int64, t Theme) string {
	if n <= 0 {
		return t.Dim.Render("—")
	}
	var glyphs []string
	rest := n
	for i := len(ChipDenoms) - 1; i >= 0; i-- {
		d := ChipDenoms[i]
		for rest >= d && len(glyphs) < maxChipGlyphs {
			glyphs = append(glyphs, Chip(d, t))
			rest -= d
		}
	}
	out := strings.Join(glyphs, "")
	if rest > 0 {
		out += t.Dim.Render(fmt.Sprintf("+%d", rest))
	}
	return out
}
