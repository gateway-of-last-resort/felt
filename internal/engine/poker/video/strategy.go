package video

import (
	"github.com/gateway-of-last-resort/felt/internal/deck"
	"github.com/gateway-of-last-resort/felt/internal/engine/poker"
)

// EV is the exact expected return of holding a given set of cards, in credits
// per hand, counted over every draw the remaining deck can produce.
//
// It is exact rather than sampled: the worst case is holding nothing, which
// is 1,533,939 draws, and that runs in well under a second. Sampling would
// make the interesting cases — the ones turning on a single royal in fifty
// thousand — too noisy to assert on.
func EV(hand [5]deck.Card, hold [5]bool, coins int64) float64 {
	rest := remaining(hand)

	var (
		kept    []deck.Card
		discard int
	)
	for i, keep := range hold {
		if keep {
			kept = append(kept, hand[i])
			continue
		}
		discard++
	}

	if discard == 0 {
		return float64(Payout(poker.Eval5(hand), coins))
	}

	var (
		total float64
		draws int
		final [5]deck.Card
	)
	copy(final[:], kept)

	var walk func(start, need int)
	walk = func(start, need int) {
		if need == 0 {
			total += float64(Payout(poker.Eval5(final), coins))
			draws++
			return
		}
		// Leave enough cards behind to finish the hand.
		for i := start; i <= len(rest)-need; i++ {
			final[5-need] = rest[i]
			walk(i+1, need-1)
		}
	}
	walk(0, discard)

	if draws == 0 {
		return 0
	}
	return total / float64(draws)
}

// BestHold returns the highest-expectation way to play a hand, and what it is
// worth. Ties go to holding more cards, which is how strategy charts are
// written and keeps the answer stable.
func BestHold(hand [5]deck.Card, coins int64) ([5]bool, float64) {
	var (
		best     [5]bool
		bestEV   = -1.0
		bestHeld int
	)

	for mask := 0; mask < 32; mask++ {
		var hold [5]bool
		held := 0
		for i := range hold {
			if mask&(1<<i) != 0 {
				hold[i] = true
				held++
			}
		}

		ev := EV(hand, hold, coins)
		if ev > bestEV || (ev == bestEV && held > bestHeld) {
			best, bestEV, bestHeld = hold, ev, held
		}
	}
	return best, bestEV
}

// remaining is the 47 cards not in the hand.
func remaining(hand [5]deck.Card) []deck.Card {
	inHand := map[deck.Card]bool{}
	for _, c := range hand {
		inHand[c] = true
	}

	rest := make([]deck.Card, 0, 47)
	for suit := deck.Spades; suit <= deck.Clubs; suit++ {
		for rank := deck.Two; rank <= deck.Ace; rank++ {
			c := deck.Card{Rank: rank, Suit: suit}
			if !inHand[c] {
				rest = append(rest, c)
			}
		}
	}
	return rest
}
