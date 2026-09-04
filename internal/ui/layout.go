package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Panel draws a titled box of the given total width around body.
//
// The width passed to the style is the total, borders included: that is what
// Lip Gloss v2 measures.
func Panel(title, body string, w int, t Theme) string {
	inner := maxInt(w, 3)
	var b strings.Builder
	if title != "" {
		b.WriteString(t.Title.Render(title))
		b.WriteString("\n")
	}
	b.WriteString(body)
	return t.Panel.Width(inner).Render(b.String())
}

// Center places s in the middle of a w by h area.
func Center(w, h int, s string) string {
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, s)
}

// Row joins parts side by side, top-aligned, separated by gap spaces.
func Row(gap int, parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	sep := strings.Repeat(" ", maxInt(gap, 0))
	joined := make([]string, 0, len(parts)*2-1)
	for i, p := range parts {
		if i > 0 && sep != "" {
			joined = append(joined, sep)
		}
		joined = append(joined, p)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, joined...)
}

// Stack joins parts vertically, left-aligned.
func Stack(parts ...string) string {
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// Rule returns a horizontal divider of the given width.
func Rule(w int, t Theme) string {
	return t.Dim.Render(strings.Repeat("─", maxInt(w, 0)))
}

// Pad grows s to exactly h lines by appending blank lines, so a screen that
// shrinks between frames does not make the layout jump.
func Pad(s string, h int) string {
	n := lipgloss.Height(s)
	if n >= h {
		return s
	}
	return s + strings.Repeat("\n", h-n)
}
