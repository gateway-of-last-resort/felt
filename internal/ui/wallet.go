package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// Bar renders the full-width top strip: the current screen on the left, the
// balance and the staked amount on the right.
//
// The signature carries a title as well as the money because the strip is the
// only chrome above the table; giving each screen its own heading line would
// cost a row that the narrow layouts cannot spare.
func Bar(title string, balance, bet int64, width int, t Theme) string {
	if width < 4 {
		return ""
	}

	left := t.Title.Render(strings.ToUpper(title))

	right := t.Label.Render("BALANCE ") + t.Value.Render(Credits(balance))
	if bet > 0 {
		right += t.Label.Render("   BET ") + ChipStyle(betChip(bet), t).Render(Credits(bet))
	}

	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		// Too narrow for both halves: money wins, it is the live number.
		return lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Render(right)
	}
	return left + strings.Repeat(" ", gap) + right
}

// betChip picks the colour of the largest chip that fits the bet, so the bet
// figure is tinted the same as the stack that would pay it.
func betChip(bet int64) int64 {
	d := ChipDenoms[0]
	for _, c := range ChipDenoms {
		if bet >= c {
			d = c
		}
	}
	return d
}

// Credits formats an amount with thousands separators.
func Credits(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// Signed formats a net result with an explicit sign, as shown on the stats
// screen where the direction matters more than the magnitude.
func Signed(n int64) string {
	if n > 0 {
		return "+" + Credits(n)
	}
	return Credits(n)
}

// Percent formats a ratio as a percentage with one decimal.
func Percent(f float64) string { return fmt.Sprintf("%.1f%%", f*100) }
