// Package poker evaluates poker hands.
//
// It is shared: video poker pays out by it, and holdem will award pots by it.
// That is why it is written and tested on its own, ahead of either game — a
// mistake here costs one wrong payout in video poker, but in holdem it hands
// somebody else's money to the wrong player.
package poker

import (
	"fmt"

	"github.com/gateway-of-last-resort/felt/internal/deck"
)

// Category is the kind of hand, ordered so that a higher value beats a lower.
type Category uint8

// The categories, weakest first.
const (
	HighCard Category = iota
	Pair
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush
	RoyalFlush
)

// String names the category.
func (c Category) String() string {
	switch c {
	case Pair:
		return "pair"
	case TwoPair:
		return "two pair"
	case ThreeOfAKind:
		return "three of a kind"
	case Straight:
		return "straight"
	case Flush:
		return "flush"
	case FullHouse:
		return "full house"
	case FourOfAKind:
		return "four of a kind"
	case StraightFlush:
		return "straight flush"
	case RoyalFlush:
		return "royal flush"
	default:
		return "high card"
	}
}

// Rank is a fully comparable hand strength: the category, then the ranks that
// break ties within it, most significant first.
//
// The kickers are ordered by what matters rather than by card: a full house
// lists the trips before the pair, two pair lists the higher pair first, and
// a straight lists only its top card. Unused slots are zero, so two hands of
// the same category compare correctly by walking the list.
type Rank struct {
	Cat     Category
	Kickers [5]deck.Rank
}

// Compare returns -1 if a loses to b, 0 if they tie, and 1 if a wins.
func (a Rank) Compare(b Rank) int {
	switch {
	case a.Cat < b.Cat:
		return -1
	case a.Cat > b.Cat:
		return 1
	}
	for i := range a.Kickers {
		switch {
		case a.Kickers[i] < b.Kickers[i]:
			return -1
		case a.Kickers[i] > b.Kickers[i]:
			return 1
		}
	}
	return 0
}

// Beats reports whether a wins outright. Ties are not wins — in holdem they
// split the pot.
func (a Rank) Beats(b Rank) bool { return a.Compare(b) > 0 }

// String describes the hand, e.g. "two pair, kings and fours".
func (r Rank) String() string {
	k := r.Kickers
	switch r.Cat {
	case RoyalFlush, StraightFlush, Straight:
		return fmt.Sprintf("%s, %s high", r.Cat, name(k[0]))
	case FourOfAKind, ThreeOfAKind, Pair:
		return fmt.Sprintf("%s, %s", r.Cat, plural(k[0]))
	case TwoPair:
		return fmt.Sprintf("%s, %s and %s", r.Cat, plural(k[0]), plural(k[1]))
	case FullHouse:
		return fmt.Sprintf("%s, %s full of %s", r.Cat, plural(k[0]), plural(k[1]))
	default:
		return fmt.Sprintf("%s, %s high", r.Cat, name(k[0]))
	}
}

// plural is the rank in the plural, as a dealer would call it: "sixes", not
// "sixs".
func plural(r deck.Rank) string {
	if r == deck.Six {
		return "sixes"
	}
	return name(r) + "s"
}

func name(r deck.Rank) string {
	switch r {
	case deck.Ace:
		return "ace"
	case deck.King:
		return "king"
	case deck.Queen:
		return "queen"
	case deck.Jack:
		return "jack"
	case deck.Ten:
		return "ten"
	case deck.Nine:
		return "nine"
	case deck.Eight:
		return "eight"
	case deck.Seven:
		return "seven"
	case deck.Six:
		return "six"
	case deck.Five:
		return "five"
	case deck.Four:
		return "four"
	case deck.Three:
		return "three"
	case deck.Two:
		return "two"
	default:
		return "?"
	}
}

// wheelHigh is the top card of the five-high straight, A-2-3-4-5, where the
// ace plays low. It is the one place an ace is not the best card in the deck.
const wheelHigh = deck.Five

// Eval5 rates exactly five cards.
func Eval5(cards [5]deck.Card) Rank {
	var (
		counts [deck.Ace + 1]int
		suits  [4]int
	)
	for _, c := range cards {
		counts[c.Rank]++
		suits[c.Suit]++
	}

	flush := false
	for _, n := range suits {
		if n == 5 {
			flush = true
		}
	}

	// Ranks present, highest first, and the groups by size.
	var (
		ordered []deck.Rank // distinct ranks, high to low
		byCount [5][]deck.Rank
	)
	for r := deck.Ace; r >= deck.Two; r-- {
		if n := counts[r]; n > 0 {
			ordered = append(ordered, r)
			byCount[n] = append(byCount[n], r)
		}
	}

	high, straight := straightHigh(ordered)

	switch {
	case straight && flush && high == deck.Ace:
		return Rank{Cat: RoyalFlush, Kickers: [5]deck.Rank{high}}
	case straight && flush:
		return Rank{Cat: StraightFlush, Kickers: [5]deck.Rank{high}}
	case len(byCount[4]) == 1:
		return Rank{Cat: FourOfAKind, Kickers: [5]deck.Rank{byCount[4][0], byCount[1][0]}}
	case len(byCount[3]) == 1 && len(byCount[2]) == 1:
		return Rank{Cat: FullHouse, Kickers: [5]deck.Rank{byCount[3][0], byCount[2][0]}}
	case flush:
		return Rank{Cat: Flush, Kickers: top5(ordered)}
	case straight:
		return Rank{Cat: Straight, Kickers: [5]deck.Rank{high}}
	case len(byCount[3]) == 1:
		k := [5]deck.Rank{byCount[3][0]}
		copy(k[1:], byCount[1])
		return Rank{Cat: ThreeOfAKind, Kickers: k}
	case len(byCount[2]) == 2:
		k := [5]deck.Rank{byCount[2][0], byCount[2][1], byCount[1][0]}
		return Rank{Cat: TwoPair, Kickers: k}
	case len(byCount[2]) == 1:
		k := [5]deck.Rank{byCount[2][0]}
		copy(k[1:], byCount[1])
		return Rank{Cat: Pair, Kickers: k}
	default:
		return Rank{Cat: HighCard, Kickers: top5(ordered)}
	}
}

// straightHigh reports whether five distinct descending ranks form a run, and
// the rank it tops out at.
func straightHigh(ordered []deck.Rank) (deck.Rank, bool) {
	if len(ordered) != 5 {
		return 0, false
	}
	if ordered[0]-ordered[4] == 4 {
		return ordered[0], true
	}
	// The wheel: an ace playing below the two.
	if ordered[0] == deck.Ace && ordered[1] == deck.Five &&
		ordered[2] == deck.Four && ordered[3] == deck.Three && ordered[4] == deck.Two {
		return wheelHigh, true
	}
	return 0, false
}

func top5(ordered []deck.Rank) [5]deck.Rank {
	var k [5]deck.Rank
	copy(k[:], ordered)
	return k
}

// Best5 finds the strongest five-card hand among the cards given, which may
// be five, six or seven of them — a video poker hand, or a holdem player's
// two with the five on the board.
//
// It walks every five-card subset: 21 of them at seven cards, which is fast
// enough that a cleverer method would only be harder to trust.
func Best5(cards []deck.Card) (Rank, [5]deck.Card) {
	var (
		best  Rank
		hand  [5]deck.Card
		found bool
	)
	if len(cards) < 5 {
		return best, hand
	}

	var pick [5]deck.Card
	n := len(cards)
	for a := 0; a < n-4; a++ {
		for b := a + 1; b < n-3; b++ {
			for c := b + 1; c < n-2; c++ {
				for d := c + 1; d < n-1; d++ {
					for e := d + 1; e < n; e++ {
						pick = [5]deck.Card{cards[a], cards[b], cards[c], cards[d], cards[e]}
						r := Eval5(pick)
						if !found || r.Compare(best) > 0 {
							best, hand, found = r, pick, true
						}
					}
				}
			}
		}
	}
	return best, hand
}
