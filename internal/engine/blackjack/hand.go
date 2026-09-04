package blackjack

import "github.com/gateway-of-last-resort/felt/internal/deck"

// Hand is one hand of cards with the money staked on it. A player holds more
// than one after a split, which is the reason nothing here assumes a single
// hand exists.
type Hand struct {
	Cards []deck.Card

	// Bet is the stake on this hand alone. Doubling raises it; a split hand
	// carries its own matching stake.
	Bet int64

	Doubled   bool
	Stood     bool
	Surrender bool

	// FromSplit marks a hand that came out of a pair, which cannot make
	// blackjack. SplitAces additionally marks the one-card-only case.
	FromSplit bool
	SplitAces bool

	// The result, filled in when the round settles. They live on the hand so
	// that a snapshot taken after settlement can show what each hand did.
	Outcome Outcome
	Settled bool
	Payout  int64
}

// Value returns the best total the hand can make and whether it is soft. A
// hand is soft when an ace is still counted as eleven, which is what makes it
// safe to hit.
//
// Aces start at eleven and are demoted one at a time while the hand is over
// twenty-one: A,A,A,8 is a soft 21, not a bust.
func (h Hand) Value() (total int, soft bool) {
	aces := 0
	for _, c := range h.Cards {
		total += c.Value()
		if c.IsAce() {
			aces++
		}
	}
	for total > 21 && aces > 0 {
		total -= 10
		aces--
	}
	return total, aces > 0
}

// Total is the hand's value without the softness.
func (h Hand) Total() int {
	total, _ := h.Value()
	return total
}

// IsBlackjack reports an ace with a ten on the opening two cards. A
// twenty-one built from a split is just twenty-one: it pays even money, and
// it does not beat a dealer's twenty-one.
func (h Hand) IsBlackjack() bool {
	return len(h.Cards) == 2 && !h.FromSplit && h.Total() == 21
}

// IsBust reports whether the hand is over twenty-one.
func (h Hand) IsBust() bool { return h.Total() > 21 }

// CanSplit reports whether the hand may be split, given how many times the
// player has already split.
//
// Pairs are matched by value, not by rank, so a king and a queen may be
// split. That is the common table rule, and it is the one the basic strategy
// charts assume.
func (h Hand) CanSplit(r Rules, splitsSoFar int) bool {
	if len(h.Cards) != 2 || h.Doubled || h.Stood {
		return false
	}
	if splitsSoFar >= r.MaxSplits {
		return false
	}
	if h.SplitAces {
		return false
	}
	return h.Cards[0].Value() == h.Cards[1].Value()
}

// CanDouble reports whether the hand may be doubled.
func (h Hand) CanDouble(r Rules) bool {
	if len(h.Cards) != 2 || h.Doubled || h.Stood || h.SplitAces {
		return false
	}
	if h.FromSplit && !r.DoubleAfterSplit {
		return false
	}
	return true
}

// CanSurrender reports whether the hand may be given up for half the stake.
// Late surrender only: opening two cards, before anything else is done to
// the hand, and never after a split.
func (h Hand) CanSurrender(r Rules) bool {
	return r.Surrender && len(h.Cards) == 2 && !h.FromSplit && !h.Doubled && !h.Stood
}

// Done reports whether the hand can take no further action. Split aces are
// dealt one card and then stand, which is why the rules are needed here.
func (h Hand) Done(r Rules) bool {
	if h.SplitAces && r.SplitAcesOneCard && len(h.Cards) >= 2 {
		return true
	}
	return h.Stood || h.Doubled || h.Surrender || h.IsBust() || h.Total() == 21
}

// Add deals a card onto the hand.
func (h *Hand) Add(c deck.Card) { h.Cards = append(h.Cards, c) }

// Split divides a pair into two hands, each keeping the original stake. The
// caller deals the second card to each.
func (h Hand) Split() (Hand, Hand) {
	aces := h.Cards[0].IsAce()

	left := Hand{
		Cards:     []deck.Card{h.Cards[0]},
		Bet:       h.Bet,
		FromSplit: true,
		SplitAces: aces,
	}
	right := Hand{
		Cards:     []deck.Card{h.Cards[1]},
		Bet:       h.Bet,
		FromSplit: true,
		SplitAces: aces,
	}
	return left, right
}
