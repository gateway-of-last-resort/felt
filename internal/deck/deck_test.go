package deck

import (
	"math"
	"testing"

	"github.com/gateway-of-last-resort/felt/internal/rng"
)

func seeded() *Shoe {
	var seed [32]byte
	seed[0] = 7
	return NewShoe(6, 0.75, rng.NewSeeded(seed))
}

// A shuffle may reorder the shoe but must not change what is in it.
func TestShuffleKeepsComposition(t *testing.T) {
	s := seeded()

	count := func() map[Card]int {
		m := make(map[Card]int, CardsPerDeck)
		for _, c := range s.cards {
			m[c]++
		}
		return m
	}

	before := count()
	if got, want := len(s.cards), 6*CardsPerDeck; got != want {
		t.Fatalf("shoe has %d cards, want %d", got, want)
	}
	if got, want := len(before), CardsPerDeck; got != want {
		t.Fatalf("shoe has %d distinct cards, want %d", got, want)
	}
	for c, n := range before {
		if n != 6 {
			t.Fatalf("%v appears %d times, want 6", c, n)
		}
	}

	s.Shuffle()
	after := count()
	for c, n := range before {
		if after[c] != n {
			t.Errorf("%v: %d before, %d after shuffle", c, n, after[c])
		}
	}
}

// Drawing the whole shoe and one more must not run off the end.
func TestDrawStaysInBounds(t *testing.T) {
	s := seeded()
	total := 6 * CardsPerDeck

	for i := 0; i < total; i++ {
		s.Draw()
	}
	if got := s.Remaining(); got != 0 {
		t.Fatalf("remaining = %d after draining the shoe, want 0", got)
	}

	// The next draw reshuffles rather than panicking.
	s.Draw()
	if got := s.Remaining(); got != total-1 {
		t.Fatalf("remaining = %d after the wrap-around draw, want %d", got, total-1)
	}
}

func TestNeedsShuffleAtPenetration(t *testing.T) {
	s := NewShoe(1, 0.75, rng.NewSeeded([32]byte{3}))

	// At 0.75 penetration the cut card sits at 39 of 52.
	for i := 0; i < 39; i++ {
		if s.NeedsShuffle() {
			t.Fatalf("cut card reached after %d of 52 cards, want 39", i)
		}
		s.Draw()
	}
	if !s.NeedsShuffle() {
		t.Fatal("cut card not reached after 39 of 52 cards at 0.75 penetration")
	}
}

// Every card should land in every position about equally often. With 20k
// shuffles of a single deck, each of the 52 cards lands in position 0 about
// 385 times; a bias big enough to matter shows up far outside 15%.
func TestShuffleIsUniform(t *testing.T) {
	if testing.Short() {
		t.Skip("distribution test is slow")
	}

	const rounds = 20000
	s := NewShoe(1, 1, rng.NewSeeded([32]byte{11}))

	first := make(map[Card]int, CardsPerDeck)
	for i := 0; i < rounds; i++ {
		s.Shuffle()
		first[s.cards[0]]++
	}

	want := float64(rounds) / CardsPerDeck
	for c, n := range first {
		if dev := math.Abs(float64(n)-want) / want; dev > 0.15 {
			t.Errorf("%v led %d times, want ~%.0f (off by %.1f%%)", c, n, want, dev*100)
		}
	}
}

func TestCardValues(t *testing.T) {
	cases := []struct {
		card Card
		want int
	}{
		{Card{Ace, Spades}, 11},
		{Card{King, Hearts}, 10},
		{Card{Ten, Clubs}, 10},
		{Card{Two, Diamonds}, 2},
		{Card{Nine, Spades}, 9},
	}
	for _, c := range cases {
		if got := c.card.Value(); got != c.want {
			t.Errorf("%v.Value() = %d, want %d", c.card, got, c.want)
		}
	}
}

func TestRankShort(t *testing.T) {
	cases := map[Rank]string{Two: "2", Nine: "9", Ten: "10", Jack: "J", Queen: "Q", King: "K", Ace: "A"}
	for r, want := range cases {
		if got := r.Short(); got != want {
			t.Errorf("Rank(%d).Short() = %q, want %q", r, got, want)
		}
	}
}
