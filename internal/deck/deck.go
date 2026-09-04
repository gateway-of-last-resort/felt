package deck

import "math/rand/v2"

// CardsPerDeck is the number of cards in one standard pack.
const CardsPerDeck = 52

// Shoe is a multi-deck shoe dealt from the front. Once penetration is reached
// the shoe asks to be shuffled rather than shuffling itself, so the game can
// show the "shuffling" phase at a moment of its own choosing.
type Shoe struct {
	cards       []Card
	pos         int
	decks       int
	penetration float64
	rng         *rand.Rand
}

// NewShoe builds a shuffled shoe of the given size. Penetration is the share
// of the shoe dealt before a reshuffle is due, e.g. 0.75 for three quarters.
func NewShoe(decks int, penetration float64, r *rand.Rand) *Shoe {
	if decks < 1 {
		decks = 1
	}
	if penetration <= 0 || penetration > 1 {
		penetration = 0.75
	}
	s := &Shoe{
		cards:       make([]Card, 0, decks*CardsPerDeck),
		decks:       decks,
		penetration: penetration,
		rng:         r,
	}
	for d := 0; d < decks; d++ {
		for suit := Spades; suit <= Clubs; suit++ {
			for rank := Two; rank <= Ace; rank++ {
				s.cards = append(s.cards, Card{Rank: rank, Suit: suit})
			}
		}
	}
	s.Shuffle()
	return s
}

// Shuffle refills the shoe and reorders it with Fisher-Yates.
func (s *Shoe) Shuffle() {
	for i := len(s.cards) - 1; i > 0; i-- {
		j := s.rng.IntN(i + 1)
		s.cards[i], s.cards[j] = s.cards[j], s.cards[i]
	}
	s.pos = 0
}

// Draw returns the next card, reshuffling first if the shoe has run dry.
func (s *Shoe) Draw() Card {
	if s.pos >= len(s.cards) {
		s.Shuffle()
	}
	c := s.cards[s.pos]
	s.pos++
	return c
}

// NeedsShuffle reports whether the cut card has been reached.
func (s *Shoe) NeedsShuffle() bool {
	return float64(s.pos) >= float64(len(s.cards))*s.penetration
}

// Remaining is the number of cards left before the shoe runs out.
func (s *Shoe) Remaining() int { return len(s.cards) - s.pos }

// Decks is the size of the shoe in packs.
func (s *Shoe) Decks() int { return s.decks }

// Stack puts the given cards at the front of the shoe, in order, so a test
// can deal a known sequence. It is only meant for tests: the cards are placed
// on top of whatever the shuffle produced, which leaves duplicates further
// down the shoe.
func (s *Shoe) Stack(cards []Card) {
	if len(cards) > len(s.cards) {
		cards = cards[:len(s.cards)]
	}
	copy(s.cards, cards)
	s.pos = 0
}
