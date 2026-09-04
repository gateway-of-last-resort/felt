package ui

import (
	"fmt"
	"image/color"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/lipgloss/v2"
)

// Countdown draws the time left before a deadline.
//
// Offline it never appears: local tables wait for the player, so their
// deadlines are zero. It exists now because the shape of a timed table is
// already in the engine — a room sets a betting window and a turn clock, and
// this is what draws them without the game screens changing.
func Countdown(deadline, now time.Time, width int, t Theme) string {
	if deadline.IsZero() || width < 8 {
		return ""
	}

	left := deadline.Sub(now)
	if left < 0 {
		left = 0
	}

	// The bar is filled by how much time is left, so it empties as the clock
	// runs down rather than filling up towards nothing.
	total := 20 * time.Second
	if left > total {
		total = left
	}
	pct := float64(left) / float64(total)

	bar := progress.New(
		progress.WithoutPercentage(),
		progress.WithWidth(maxInt(width-8, 4)),
		progress.WithColors(urgencyColor(left)),
	)

	return lipgloss.JoinHorizontal(lipgloss.Center,
		bar.ViewAs(pct),
		" ",
		t.Value.Render(fmt.Sprintf("%2ds", int(left.Round(time.Second).Seconds()))),
	)
}

// urgencyColor reddens the bar as the deadline approaches: three seconds left
// should not look like thirty.
func urgencyColor(left time.Duration) color.Color {
	switch {
	case left <= 3*time.Second:
		return lipgloss.Color("#E5383B")
	case left <= 8*time.Second:
		return lipgloss.Color("#D4AF37")
	default:
		return lipgloss.Color("#52B788")
	}
}
