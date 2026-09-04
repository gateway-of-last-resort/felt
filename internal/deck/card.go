// Package deck models playing cards and a multi-deck shoe.
package deck

// Suit is a card suit, ordered as in a fresh pack.
type Suit uint8

// The four suits.
const (
	Spades Suit = iota
	Hearts
	Diamonds
	Clubs
)

// Rank is a card rank from Two through Ace.
type Rank uint8

// The thirteen ranks.
const (
	Two Rank = iota + 2
	Three
	Four
	Five
	Six
	Seven
	Eight
	Nine
	Ten
	Jack
	Queen
	King
	Ace
)

// Card is a single playing card.
type Card struct {
	Rank Rank
	Suit Suit
}

// Value is the blackjack value of the card: 10 for face cards, 11 for an ace.
// Hand.Value demotes aces to 1 when a hand would otherwise bust.
func (c Card) Value() int {
	switch {
	case c.Rank == Ace:
		return 11
	case c.Rank >= Ten:
		return 10
	default:
		return int(c.Rank)
	}
}

// IsAce reports whether the card is an ace.
func (c Card) IsAce() bool { return c.Rank == Ace }

// Symbol returns the suit pip.
func (s Suit) Symbol() string {
	switch s {
	case Spades:
		return "♠"
	case Hearts:
		return "♥"
	case Diamonds:
		return "♦"
	default:
		return "♣"
	}
}

// Red reports whether the suit is printed in red.
func (s Suit) Red() bool { return s == Hearts || s == Diamonds }

// Short returns the rank label as printed on a card corner.
func (r Rank) Short() string {
	switch r {
	case Ace:
		return "A"
	case King:
		return "K"
	case Queen:
		return "Q"
	case Jack:
		return "J"
	case Ten:
		return "10"
	default:
		return string(rune('0' + r))
	}
}

// String renders the card as rank plus pip, e.g. "A♠".
func (c Card) String() string { return c.Rank.Short() + c.Suit.Symbol() }
