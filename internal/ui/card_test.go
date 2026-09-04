package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/gateway-of-last-resort/felt/internal/deck"
)

// Every card is the same size, face up or face down.
//
// This is the test that would have caught the face-down card unrolling into a
// column eleven rows tall: Lip Gloss v2 measures a styled block including its
// border, so a width meant for the contents left one column inside the frame
// and wrapped every row.
func TestCardsAreAllTheSameSize(t *testing.T) {
	th := Default()

	cards := []deck.Card{
		{Rank: deck.Ace, Suit: deck.Spades},
		{Rank: deck.Seven, Suit: deck.Clubs},
		{Rank: deck.Ten, Suit: deck.Hearts}, // the two-character rank
		{Rank: deck.King, Suit: deck.Diamonds},
	}

	for _, c := range cards {
		got := RenderCard(c, th)
		if w, h := lipgloss.Width(got), lipgloss.Height(got); w != CardWidth || h != CardHeight {
			t.Errorf("%v renders %dx%d, want %dx%d", c, w, h, CardWidth, CardHeight)
		}
	}

	back := RenderBack(th)
	if w, h := lipgloss.Width(back), lipgloss.Height(back); w != CardWidth || h != CardHeight {
		t.Errorf("the card back renders %dx%d, want %dx%d", w, h, CardWidth, CardHeight)
	}
}

// A hand is a single row of cards, however many it holds and whichever of
// them are face down.
func TestHandIsOneRowOfCards(t *testing.T) {
	th := Default()
	hand := []deck.Card{
		{Rank: deck.Ace, Suit: deck.Spades},
		{Rank: deck.Ten, Suit: deck.Hearts},
		{Rank: deck.Two, Suit: deck.Clubs},
	}

	for _, hole := range []bool{false, true} {
		got := RenderHand(hand, hole, th)
		if h := lipgloss.Height(got); h != CardHeight {
			t.Errorf("hole=%v: hand is %d rows tall, want %d", hole, h, CardHeight)
		}
		// Three cards and two single-column gaps.
		if w, want := lipgloss.Width(got), 3*CardWidth+2; w != want {
			t.Errorf("hole=%v: hand is %d cells wide, want %d", hole, w, want)
		}
		for i, line := range strings.Split(got, "\n") {
			if w := lipgloss.Width(line); w != 3*CardWidth+2 {
				t.Errorf("hole=%v: line %d is %d cells wide", hole, i+1, w)
			}
		}
	}
}

// The compact form is one line, whatever it holds.
func TestCompactHandIsOneLine(t *testing.T) {
	th := Default()
	hand := []deck.Card{
		{Rank: deck.Ace, Suit: deck.Spades},
		{Rank: deck.Ten, Suit: deck.Hearts},
	}

	got := RenderHandCompact(hand, false, th)
	if h := lipgloss.Height(got); h != 1 {
		t.Errorf("compact hand is %d rows tall, want 1", h)
	}
	if strings.Contains(got, "\n") {
		t.Error("compact hand contains a newline")
	}
}
