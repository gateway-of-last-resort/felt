package poker

import (
	"testing"

	"github.com/gateway-of-last-resort/felt/internal/deck"
)

// hand parses "As Kd 7h 7c 2s" — rank letter then suit letter. Suits matter
// here in a way they never did for blackjack, so they are spelled out.
func hand(spec string) []deck.Card {
	var out []deck.Card
	for _, f := range fields(spec) {
		if len(f) != 2 {
			panic("bad card: " + f)
		}
		out = append(out, deck.Card{Rank: parseRank(f[0]), Suit: parseSuit(f[1])})
	}
	return out
}

func five(spec string) [5]deck.Card {
	cards := hand(spec)
	if len(cards) != 5 {
		panic("want exactly five cards: " + spec)
	}
	return [5]deck.Card{cards[0], cards[1], cards[2], cards[3], cards[4]}
}

func parseRank(b byte) deck.Rank {
	switch b {
	case 'A':
		return deck.Ace
	case 'K':
		return deck.King
	case 'Q':
		return deck.Queen
	case 'J':
		return deck.Jack
	case 'T':
		return deck.Ten
	default:
		if b < '2' || b > '9' {
			panic("bad rank: " + string(b))
		}
		return deck.Rank(b - '0')
	}
}

func parseSuit(b byte) deck.Suit {
	switch b {
	case 's':
		return deck.Spades
	case 'h':
		return deck.Hearts
	case 'd':
		return deck.Diamonds
	case 'c':
		return deck.Clubs
	default:
		panic("bad suit: " + string(b))
	}
}

func fields(s string) []string {
	var out []string
	cur := ""
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(s[i])
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// Every category, recognised from a hand that is unmistakably it.
func TestCategories(t *testing.T) {
	cases := []struct {
		spec string
		want Category
	}{
		{"As Ks Qs Js Ts", RoyalFlush},
		{"9h 8h 7h 6h 5h", StraightFlush},
		{"5c 4c 3c 2c Ac", StraightFlush}, // the wheel, suited
		{"7s 7h 7d 7c 2s", FourOfAKind},
		{"Kd Kh Ks 4c 4d", FullHouse},
		{"Ah Jh 8h 5h 2h", Flush},
		{"9s 8h 7d 6c 5s", Straight},
		{"5s 4h 3d 2c Ah", Straight}, // the wheel
		{"Qs Qh Qd 9c 4s", ThreeOfAKind},
		{"Js Jh 6d 6c Ks", TwoPair},
		{"Ts Th 9d 5c 2s", Pair},
		{"Ad Qs 9h 6c 3d", HighCard},
	}

	for _, c := range cases {
		t.Run(c.spec, func(t *testing.T) {
			if got := Eval5(five(c.spec)).Cat; got != c.want {
				t.Errorf("Eval5(%s) = %v, want %v", c.spec, got, c.want)
			}
		})
	}
}

// The wheel is the one hand where an ace is the lowest card. It must rank as
// a five-high straight, and lose to every other straight.
func TestWheelIsFiveHigh(t *testing.T) {
	wheel := Eval5(five("5s 4h 3d 2c Ah"))
	if wheel.Cat != Straight {
		t.Fatalf("the wheel is %v, want a straight", wheel.Cat)
	}
	if wheel.Kickers[0] != deck.Five {
		t.Errorf("the wheel tops out at %v, want a five", wheel.Kickers[0])
	}

	sixHigh := Eval5(five("6s 5h 4d 3c 2h"))
	if !sixHigh.Beats(wheel) {
		t.Error("a six-high straight does not beat the wheel")
	}

	// And an ace-high straight is the best of them.
	broadway := Eval5(five("As Kh Qd Jc Ts"))
	if !broadway.Beats(sixHigh) {
		t.Error("broadway does not beat a six-high straight")
	}
}

// Hands of the same category are separated by their kickers, in the right
// order of significance.
func TestKickers(t *testing.T) {
	cases := []struct {
		name          string
		better, worse string
	}{
		{"higher pair", "Ks Kh 5d 3c 2s", "Qs Qh Jd 9c 8s"},
		{"pair kicker", "8s 8h Ad 3c 2s", "8d 8c Kd 3h 2h"},
		{"pair second kicker", "8s 8h Ad Qc 2s", "8d 8c Ah Jc 2h"},
		{"higher top pair", "Ks Kh 2d 2c 9s", "Qs Qh Jd Jc As"},
		{"two pair second pair", "Ks Kh 9d 9c 2s", "Kd Kc 8d 8h As"},
		{"two pair kicker", "Ks Kh 9d 9c As", "Kd Kc 9h 9s Qd"},
		{"higher trips", "Qs Qh Qd 3c 2s", "Js Jh Jd Ac Ks"},
		{"trips kicker", "7s 7h 7d Ac 2s", "7c 7d 7h Kc Qs"},
		{"higher flush", "Ah 9h 7h 5h 3h", "Ks Qs Js 9s 7s"},
		{"flush second card", "Ah Kh 7h 5h 3h", "Ad Qd Jd 9d 7d"},
		{"higher full house trips", "Ks Kh Kd 2c 2s", "Qs Qh Qd Ac As"},
		{"full house pair", "Ks Kh Kd Ac As", "Kc Kd Ks 2h 2d"},
		{"higher quads", "As Ah Ad Ac 2s", "Ks Kh Kd Kc As"},
		{"quads kicker", "9s 9h 9d 9c As", "9s 9h 9d 9c Ks"},
		{"higher straight", "Ts 9h 8d 7c 6s", "9s 8h 7d 6c 5s"},
		{"high card", "As Qh 9d 6c 3s", "Ks Qh 9d 6c 3s"},
		{"high card last kicker", "As Qh 9d 6c 4s", "Ad Qs 9h 6d 3c"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			better, worse := Eval5(five(c.better)), Eval5(five(c.worse))
			if !better.Beats(worse) {
				t.Errorf("%s (%v) does not beat %s (%v)", c.better, better, c.worse, worse)
			}
			if worse.Beats(better) {
				t.Errorf("%s beats %s in both directions", c.worse, c.better)
			}
		})
	}
}

// Hands that differ only by suit tie. In holdem that means a split pot, so a
// spurious winner here is money moved wrongly.
func TestTies(t *testing.T) {
	cases := [][2]string{
		{"As Ks Qs Js Ts", "Ah Kh Qh Jh Th"}, // royal flushes
		{"9s 9h 4d 4c As", "9d 9c 4h 4s Ad"}, // two pair with the same kicker
		{"As Qh 9d 6c 3s", "Ad Qs 9h 6d 3c"}, // high card
		{"5s 4h 3d 2c As", "5d 4c 3h 2s Ah"}, // wheels
		{"Ks Kh Kd 2c 2s", "Kc Kd Kh 2h 2d"}, // full houses
	}

	for _, c := range cases {
		a, b := Eval5(five(c[0])), Eval5(five(c[1]))
		if got := a.Compare(b); got != 0 {
			t.Errorf("%s vs %s = %d, want a tie (%v vs %v)", c[0], c[1], got, a, b)
		}
	}
}

// The whole ordering, from the bottom up. Each hand must beat every hand
// below it and lose to every hand above.
func TestCategoryOrdering(t *testing.T) {
	ladder := []string{
		"Ad Qs 9h 6c 3d", // high card
		"Ts Th 9d 5c 2s", // pair
		"Js Jh 6d 6c Ks", // two pair
		"Qs Qh Qd 9c 4s", // trips
		"9s 8h 7d 6c 5s", // straight
		"Ah Jh 8h 5h 2h", // flush
		"Kd Kh Ks 4c 4d", // full house
		"7s 7h 7d 7c 2s", // quads
		"9h 8h 7h 6h 5h", // straight flush
		"As Ks Qs Js Ts", // royal flush
	}

	for i := range ladder {
		for j := range ladder {
			a, b := Eval5(five(ladder[i])), Eval5(five(ladder[j]))
			switch {
			case i > j && !a.Beats(b):
				t.Errorf("%s does not beat %s", ladder[i], ladder[j])
			case i < j && a.Beats(b):
				t.Errorf("%s beats %s, but should not", ladder[i], ladder[j])
			case i == j && a.Compare(b) != 0:
				t.Errorf("%s does not tie itself", ladder[i])
			}
		}
	}
}

// Best5 picks the strongest five out of six or seven — a holdem hand read
// against the board.
func TestBest5(t *testing.T) {
	cases := []struct {
		name  string
		cards string
		want  Category
		hand  string
	}{
		{
			name:  "seven cards, the flush is not the first five",
			cards: "2c 7d Ah 9h 4h Kh 3h",
			want:  Flush,
			hand:  "Ah Kh 9h 4h 3h",
		},
		{
			name:  "trips on the board plus a pocket pair is a full house",
			cards: "8s 8h 8d Kc Kh 2s 3d",
			want:  FullHouse,
			hand:  "8s 8h 8d Kc Kh",
		},
		{
			name:  "a straight hiding among seven",
			cards: "As 2h 3d 4c 5s Kh Qd",
			want:  Straight,
			hand:  "As 2h 3d 4c 5s",
		},
		{
			name:  "six cards",
			cards: "9s 9h 9d 9c Ks 2h",
			want:  FourOfAKind,
			hand:  "9s 9h 9d 9c Ks",
		},
		{
			name:  "the best straight, not the first one found",
			cards: "9h 8s 7d 6c 5h 4s 3d",
			want:  Straight,
			hand:  "9h 8s 7d 6c 5h",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, best := Best5(hand(c.cards))
			if got.Cat != c.want {
				t.Fatalf("Best5(%s) = %v, want %v", c.cards, got, c.want)
			}
			if want := Eval5(five(c.hand)); got.Compare(want) != 0 {
				t.Errorf("Best5 found %v (%v), want the strength of %s (%v)",
					got, best, c.hand, want)
			}
		})
	}
}

// Best5 must return the cards it rated, not just the rating.
func TestBest5ReturnsItsCards(t *testing.T) {
	r, cards := Best5(hand("2c 7d Ah 9h 4h Kh 3h"))
	if again := Eval5(cards); again.Compare(r) != 0 {
		t.Errorf("the returned cards rate %v, but Best5 reported %v", again, r)
	}
	for _, c := range cards {
		if c.Suit != deck.Hearts {
			t.Errorf("the flush it returned contains %v", c)
		}
	}
}

// Fewer than five cards has no answer, and must not panic looking for one.
func TestBest5NeedsFiveCards(t *testing.T) {
	for _, spec := range []string{"", "As", "As Ks Qs Js"} {
		r, _ := Best5(hand(spec))
		if r.Cat != HighCard || r.Kickers[0] != 0 {
			t.Errorf("Best5(%q) = %v, want the zero rank", spec, r)
		}
	}
}

// A quick description, since it ends up on screen at a showdown.
func TestRankDescriptions(t *testing.T) {
	cases := map[string]string{
		"As Ks Qs Js Ts": "royal flush, ace high",
		"9h 8h 7h 6h 5h": "straight flush, nine high",
		"7s 7h 7d 7c 2s": "four of a kind, sevens",
		"Kd Kh Ks 4c 4d": "full house, kings full of fours",
		"Js Jh 6d 6c Ks": "two pair, jacks and sixes",
		"Ts Th 9d 5c 2s": "pair, tens",
		"Ad Qs 9h 6c 3d": "high card, ace high",
	}
	for spec, want := range cases {
		if got := Eval5(five(spec)).String(); got != want {
			t.Errorf("%s reads as %q, want %q", spec, got, want)
		}
	}
}

// Every five-card hand in the deck, counted by category.
//
// The frequencies of poker hands are known exactly, and this walks all
// 2,598,960 of them and checks the tally against that table. Spot checks can
// miss a whole class of hand — a straight that is also a flush counted twice,
// an ace-low run misread — and this cannot: one misclassified hand anywhere
// in the deck moves two of these numbers.
func TestExhaustiveHandFrequencies(t *testing.T) {
	if testing.Short() {
		t.Skip("the full deck walk is slow")
	}

	var cards []deck.Card
	for suit := deck.Spades; suit <= deck.Clubs; suit++ {
		for rank := deck.Two; rank <= deck.Ace; rank++ {
			cards = append(cards, deck.Card{Rank: rank, Suit: suit})
		}
	}
	if len(cards) != 52 {
		t.Fatalf("built a deck of %d cards", len(cards))
	}

	counts := map[Category]int{}
	total := 0
	for a := 0; a < 48; a++ {
		for b := a + 1; b < 49; b++ {
			for c := b + 1; c < 50; c++ {
				for d := c + 1; d < 51; d++ {
					for e := d + 1; e < 52; e++ {
						r := Eval5([5]deck.Card{cards[a], cards[b], cards[c], cards[d], cards[e]})
						counts[r.Cat]++
						total++
					}
				}
			}
		}
	}

	want := map[Category]int{
		RoyalFlush:    4,
		StraightFlush: 36,
		FourOfAKind:   624,
		FullHouse:     3744,
		Flush:         5108,
		Straight:      10200,
		ThreeOfAKind:  54912,
		TwoPair:       123552,
		Pair:          1098240,
		HighCard:      1302540,
	}

	if total != 2598960 {
		t.Fatalf("walked %d hands, want 2,598,960", total)
	}
	for cat, n := range want {
		if got := counts[cat]; got != n {
			t.Errorf("%v: %d hands, want %d", cat, got, n)
		}
	}
}
