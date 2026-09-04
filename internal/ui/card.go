package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/gateway-of-last-resort/felt/internal/deck"
)

// cardInner is the width of a card's face, borders excluded. Five gives the
// rank and pip room to sit apart from the edges; at three the box read as a
// narrow slot rather than a card.
const cardInner = 6

// CardWidth and CardHeight are the rendered size of one card box, borders
// included. Layout code sizes hands against these rather than measuring.
const (
	CardWidth  = cardInner + 2
	CardHeight = 5
)

// cardBox is the frame around a card.
//
// It deliberately sets no Width or Height. In Lip Gloss v2 those measure the
// whole styled block, borders included, so asking for a width of three left
// one column inside the frame and wrapped every row of the card down the
// screen. The contents are already exactly three cells wide and three rows
// tall, so the border can size itself.
func cardBox(t Theme, fg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Foreground(fg)
}

// RenderCard draws a single face-up card: rank in the top-left corner, pip in
// the middle, rank mirrored bottom-right.
func RenderCard(c deck.Card, t Theme) string {
	fg := t.Black
	if c.Suit.Red() {
		fg = t.Red
	}
	r := c.Rank.Short()
	top := padRight(r, cardInner)
	mid := centerIn(c.Suit.Symbol(), cardInner)
	bot := padLeft(r, cardInner)
	return cardBox(t, fg).Render(top + "\n" + mid + "\n" + bot)
}

// RenderBack draws a face-down card.
func RenderBack(t Theme) string {
	row := strings.Repeat("▒", cardInner)
	body := strings.Repeat(row+"\n", 2) + row
	return cardBox(t, t.Muted).Render(body)
}

// RenderHand lays cards out side by side. When hole is true the second card
// is drawn face down, which is how the dealer's hand is shown mid-round.
func RenderHand(cards []deck.Card, hole bool, t Theme) string {
	if len(cards) == 0 {
		return ""
	}
	boxes := make([]string, 0, len(cards))
	for i, c := range cards {
		if hole && i == 1 {
			boxes = append(boxes, RenderBack(t))
			continue
		}
		boxes = append(boxes, RenderCard(c, t))
	}
	return Row(1, boxes...)
}

// RenderHandCompact draws the same hand as one line of text, for terminals
// too narrow for card boxes.
func RenderHandCompact(cards []deck.Card, hole bool, t Theme) string {
	parts := make([]string, 0, len(cards))
	for i, c := range cards {
		if hole && i == 1 {
			parts = append(parts, t.Dim.Render("▒▒"))
			continue
		}
		st := lipgloss.NewStyle().Foreground(t.Black)
		if c.Suit.Red() {
			st = st.Foreground(t.Red)
		}
		parts = append(parts, st.Render(c.String()))
	}
	return strings.Join(parts, " ")
}

func padRight(s string, w int) string {
	if n := lipgloss.Width(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

func padLeft(s string, w int) string {
	if n := lipgloss.Width(s); n < w {
		return strings.Repeat(" ", w-n) + s
	}
	return s
}

// centerIn pads a string to width w with the extra space split either side,
// leaning left when it cannot be split evenly.
func centerIn(s string, w int) string {
	n := lipgloss.Width(s)
	if n >= w {
		return s
	}
	left := (w - n) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", w-n-left)
}
