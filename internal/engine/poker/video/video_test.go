package video

import (
	"testing"
	"time"

	"github.com/gateway-of-last-resort/felt/internal/deck"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	"github.com/gateway-of-last-resort/felt/internal/engine/poker"
	"github.com/gateway-of-last-resort/felt/internal/rng"
)

const me = engine.PlayerID("p1")

func testTable(t *testing.T) *Table {
	t.Helper()
	tb := NewTable(rng.NewSeeded([32]byte{13}))
	if _, err := tb.Join(me, 0); err != nil {
		t.Fatal(err)
	}
	return tb
}

func rank(spec string) poker.Rank { return poker.Eval5(fiveCards(spec)) }

func fiveCards(spec string) [5]deck.Card {
	var out [5]deck.Card
	i := 0
	for f := 0; f < len(spec); f++ {
		if spec[f] == ' ' {
			continue
		}
		r := parseRank(spec[f])
		f++
		out[i] = deck.Card{Rank: r, Suit: parseSuit(spec[f])}
		i++
	}
	if i != 5 {
		panic("want five cards: " + spec)
	}
	return out
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
	default:
		return deck.Clubs
	}
}

// The 9/6 schedule, one coin at a time. These nine numbers are the game.
func TestPaytable(t *testing.T) {
	cases := []struct {
		spec string
		want int64
	}{
		{"As Ks Qs Js Ts", 250}, // royal, below max coins
		{"9h 8h 7h 6h 5h", 50},
		{"7s 7h 7d 7c 2s", 25},
		{"Kd Kh Ks 4c 4d", 9}, // the nine
		{"Ah Jh 8h 5h 2h", 6}, // the six
		{"9s 8h 7d 6c 5s", 4},
		{"Qs Qh Qd 9c 4s", 3},
		{"Js Jh 6d 6c Ks", 2},
		{"Js Jh 9d 5c 2s", 1}, // jacks
		{"Ts Th 9d 5c 2s", 0}, // tens pay nothing
		{"Ad Qs 9h 6c 3d", 0},
	}

	for _, c := range cases {
		t.Run(c.spec, func(t *testing.T) {
			if got := Payout(rank(c.spec), 1); got != c.want {
				t.Errorf("%s pays %d, want %d", c.spec, got, c.want)
			}
		})
	}
}

// The name of the game: a pair pays only from jacks up.
func TestJacksOrBetter(t *testing.T) {
	paying := []string{"Js Jh 9d 5c 2s", "Qs Qh 9d 5c 2s", "Ks Kh 9d 5c 2s", "As Ah 9d 5c 2s"}
	for _, spec := range paying {
		if got := Payout(rank(spec), 1); got != 1 {
			t.Errorf("%s pays %d, want 1", spec, got)
		}
	}

	dead := []string{"Ts Th 9d 5c 2s", "9s 9h 8d 5c 2s", "2s 2h 9d 5c 3s"}
	for _, spec := range dead {
		if got := Payout(rank(spec), 1); got != 0 {
			t.Errorf("%s pays %d, want nothing", spec, got)
		}
	}
}

// Everything scales with the bet, except the royal, which jumps at five
// coins. That jump is the only reason to bet the maximum.
func TestRoyalBonusAtMaxCoins(t *testing.T) {
	royal := rank("As Ks Qs Js Ts")

	for coins := int64(1); coins < MaxCoins; coins++ {
		if got, want := Payout(royal, coins), RoyalPerCoin*coins; got != want {
			t.Errorf("royal at %d coins pays %d, want %d", coins, got, want)
		}
	}
	if got, want := Payout(royal, MaxCoins), RoyalMaxCoins*MaxCoins; got != want {
		t.Errorf("royal at max coins pays %d, want %d", got, want)
	}

	// The per-coin rate more than triples at the fifth coin.
	fourth := Payout(royal, 4) / 4
	fifth := Payout(royal, MaxCoins) / MaxCoins
	if fifth <= fourth*3 {
		t.Errorf("the max-coin royal pays %d a coin against %d, barely a bonus", fifth, fourth)
	}

	// Nothing else changes shape.
	flush := rank("Ah Jh 8h 5h 2h")
	for coins := int64(1); coins <= MaxCoins; coins++ {
		if got, want := Payout(flush, coins), 6*coins; got != want {
			t.Errorf("a flush at %d coins pays %d, want %d", coins, got, want)
		}
	}
}

// A deal stakes the coins and puts five cards out.
func TestDeal(t *testing.T) {
	tb := testTable(t)

	events, err := tb.Apply(Deal{P: me, Coins: 5}, time.Now())
	if err != nil {
		t.Fatalf("Deal: %v", err)
	}
	dealt, ok := events[0].(Dealt)
	if !ok {
		t.Fatalf("got %T, want Dealt", events[0])
	}
	if dealt.Coins != 5 {
		t.Errorf("staked %d coins, want 5", dealt.Coins)
	}

	seen := map[deck.Card]bool{}
	for _, c := range dealt.Cards {
		if seen[c] {
			t.Errorf("%v was dealt twice", c)
		}
		seen[c] = true
	}

	if got := tb.Snapshot(me).(Snapshot).Phase; got != PhaseDraw {
		t.Errorf("phase = %v after a deal, want the draw", got)
	}
}

// Held cards stay, the rest are replaced, and no replacement is a card the
// player was already holding.
func TestDrawReplacesOnlyWhatIsNotHeld(t *testing.T) {
	tb := testTable(t)
	events, _ := tb.Apply(Deal{P: me, Coins: 1}, time.Now())
	before := events[0].(Dealt).Cards

	hold := [5]bool{true, false, true, false, true}
	events, err := tb.Apply(Draw{P: me, Hold: hold}, time.Now())
	if err != nil {
		t.Fatalf("Draw: %v", err)
	}
	drawn := events[0].(Drawn)

	for i, kept := range hold {
		switch {
		case kept && drawn.Cards[i] != before[i]:
			t.Errorf("card %d was held but changed from %v to %v", i, before[i], drawn.Cards[i])
		case kept && drawn.Replaced[i]:
			t.Errorf("card %d was held but is marked replaced", i)
		case !kept && !drawn.Replaced[i]:
			t.Errorf("card %d was not held but is not marked replaced", i)
		}
	}

	seen := map[deck.Card]bool{}
	for _, c := range drawn.Cards {
		if seen[c] {
			t.Errorf("%v appears twice in the final hand", c)
		}
		seen[c] = true
	}
	// A replacement must not be a card that was discarded and is still known
	// to be out of the deck for this hand.
	for i, c := range drawn.Cards {
		if !drawn.Replaced[i] {
			continue
		}
		for j, old := range before {
			if hold[j] && c == old {
				t.Errorf("card %d was replaced with %v, which is still in the hand", i, c)
			}
		}
	}
}

// Holding everything settles on the hand as dealt.
func TestHoldEverything(t *testing.T) {
	tb := testTable(t)
	events, _ := tb.Apply(Deal{P: me, Coins: 2}, time.Now())
	before := events[0].(Dealt).Cards

	events, _ = tb.Apply(Draw{P: me, Hold: [5]bool{true, true, true, true, true}}, time.Now())
	drawn := events[0].(Drawn)

	if drawn.Cards != before {
		t.Errorf("holding everything changed the hand: %v to %v", before, drawn.Cards)
	}
	if want := Payout(poker.Eval5(before), 2); drawn.Payout != want {
		t.Errorf("paid %d, want %d", drawn.Payout, want)
	}
}

// The settlement reports the stake and the return, which is what the wallet
// and the statistics run on.
func TestDrawSettles(t *testing.T) {
	tb := testTable(t)
	if _, err := tb.Apply(Deal{P: me, Coins: 5}, time.Now()); err != nil {
		t.Fatal(err)
	}
	events, _ := tb.Apply(Draw{P: me}, time.Now())

	settlement, ok := events[0].(engine.Settlement)
	if !ok {
		t.Fatalf("got %T, want a settlement", events[0])
	}
	wagered, won := settlement.Result()
	if wagered != 5 {
		t.Errorf("wagered %d, want 5", wagered)
	}
	if won < 0 {
		t.Errorf("won %d", won)
	}
	if settlement.Game() != engine.KindVideoPoker {
		t.Errorf("filed under %v", settlement.Game())
	}
}

// The order is deal, then draw. Neither can be taken out of turn.
func TestPhaseOrder(t *testing.T) {
	tb := testTable(t)

	if _, err := tb.Apply(Draw{P: me}, time.Now()); err == nil {
		t.Error("a draw was accepted before any cards were dealt")
	}
	if _, err := tb.Apply(Deal{P: me, Coins: 1}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.Apply(Deal{P: me, Coins: 1}, time.Now()); err == nil {
		t.Error("a second deal was accepted with cards on the table")
	}
	if _, err := tb.Apply(Draw{P: me}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.Apply(Draw{P: me}, time.Now()); err == nil {
		t.Error("a second draw was accepted")
	}
	// And the next deal starts a new hand.
	if _, err := tb.Apply(Deal{P: me, Coins: 1}, time.Now()); err != nil {
		t.Errorf("the machine refused the next hand: %v", err)
	}
}

// Coins are limited to one through five.
func TestCoinLimits(t *testing.T) {
	for _, coins := range []int64{0, -1, MaxCoins + 1} {
		tb := testTable(t)
		if _, err := tb.Apply(Deal{P: me, Coins: coins}, time.Now()); err == nil {
			t.Errorf("the machine accepted a bet of %d coins", coins)
		}
	}
	for coins := int64(1); coins <= MaxCoins; coins++ {
		tb := testTable(t)
		if _, err := tb.Apply(Deal{P: me, Coins: coins}, time.Now()); err != nil {
			t.Errorf("the machine refused %d coins: %v", coins, err)
		}
	}
}

// Every hand comes off a full deck, so cards from the last hand are back in
// play. Counting across hands means nothing here, and this is why.
func TestEachHandIsAFreshDeck(t *testing.T) {
	tb := testTable(t)

	for i := 0; i < 50; i++ {
		events, err := tb.Apply(Deal{P: me, Coins: 1}, time.Now())
		if err != nil {
			t.Fatalf("hand %d: %v", i, err)
		}
		seen := map[deck.Card]bool{}
		for _, c := range events[0].(Dealt).Cards {
			if seen[c] {
				t.Fatalf("hand %d dealt %v twice", i, c)
			}
			seen[c] = true
		}
		if _, err := tb.Apply(Draw{P: me}, time.Now()); err != nil {
			t.Fatalf("hand %d draw: %v", i, err)
		}
	}
}

// Buy-ins belong to games with stacks.
func TestBuyInRefused(t *testing.T) {
	tb := NewTable(rng.NewSeeded([32]byte{17}))
	if _, err := tb.Join(me, 10); err == nil {
		t.Fatal("the machine accepted a buy-in")
	}
}

// The famous one: a made flush with four to the royal in it.
//
// At 9/6 the right play is to break the flush and draw at the royal, and it
// is right by a wide margin. If a payout is ever mistyped this is the first
// decision that flips, which is what makes it worth asserting.
func TestBreakingAFlushForTheRoyal(t *testing.T) {
	hand := fiveCards("Ah Kh Qh Jh 9h")

	flush := EV(hand, [5]bool{true, true, true, true, true}, MaxCoins)
	royal := EV(hand, [5]bool{true, true, true, true, false}, MaxCoins)

	if royal <= flush {
		t.Errorf("drawing at the royal is worth %.2f against %.2f for the made flush", royal, flush)
	}
	if best, _ := BestHold(hand, MaxCoins); best != [5]bool{true, true, true, true, false} {
		t.Errorf("best hold is %v, want the four to the royal", best)
	}
}

// A made straight flush is never broken, not even for a royal draw. It pays
// fifty, and the royal is one card in forty-seven.
func TestStraightFlushIsKept(t *testing.T) {
	hand := fiveCards("9h 8h 7h 6h 5h")
	best, ev := BestHold(hand, MaxCoins)

	if best != [5]bool{true, true, true, true, true} {
		t.Errorf("best hold is %v, want to keep the straight flush", best)
	}
	if want := float64(50 * MaxCoins); ev != want {
		t.Errorf("it is worth %.2f, want %.0f", ev, want)
	}
}

// Decisions a strategy card would give the same answers to.
func TestKnownDecisions(t *testing.T) {
	cases := []struct {
		name string
		hand string
		want [5]bool
	}{
		{
			name: "keep the high pair, throw the rest",
			hand: "Ks Kh 2d 7c 4s",
			want: [5]bool{true, true, false, false, false},
		},
		{
			name: "keep a made royal",
			hand: "As Ks Qs Js Ts",
			want: [5]bool{true, true, true, true, true},
		},
		{
			name: "break a low pair for four to the royal",
			hand: "Ah Kh Qh Jh Ac",
			want: [5]bool{true, true, true, true, false},
		},
		{
			name: "keep trips, draw two",
			hand: "8s 8h 8d Kc 3s",
			want: [5]bool{true, true, true, false, false},
		},
		{
			name: "keep the full house",
			hand: "8s 8h 8d Kc Kh",
			want: [5]bool{true, true, true, true, true},
		},
		{
			name: "a low pair beats three to a flush",
			hand: "3h 3s 9h Kh 2c",
			want: [5]bool{true, true, false, false, false},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := BestHold(fiveCards(c.hand), MaxCoins)
			if got != c.want {
				t.Errorf("best hold for %s is %v, want %v", c.hand, got, c.want)
			}
		})
	}
}

// Holding everything is worth exactly what the hand pays — the one case where
// the expectation is not an average over anything.
func TestEVOfAPatHand(t *testing.T) {
	hand := fiveCards("Kd Kh Ks 4c 4d") // full house
	all := [5]bool{true, true, true, true, true}

	if got, want := EV(hand, all, 1), float64(9); got != want {
		t.Errorf("a pat full house is worth %.2f a coin, want %.0f", got, want)
	}
	if got, want := EV(hand, all, MaxCoins), float64(9*MaxCoins); got != want {
		t.Errorf("at max coins it is worth %.2f, want %.0f", got, want)
	}
}

// Drawing five is worth about 0.36 of a coin: a hand dealt at random pays
// that much on average, which is the floor every decision is measured from.
func TestEVOfDrawingEverything(t *testing.T) {
	hand := fiveCards("2c 7d 8s Jh 4h") // nothing worth keeping
	none := [5]bool{}

	ev := EV(hand, none, 1)
	if ev < 0.30 || ev > 0.45 {
		t.Errorf("throwing the hand away is worth %.4f a coin, want about 0.36", ev)
	}
	t.Logf("a random five-card hand returns %.4f of a coin", ev)
}

// The return of the machine to perfect play, sampled.
//
// 9/6 Jacks or Better famously returns 99.54%, and the honest way to confirm
// that is to walk all 2,598,960 deals — days of work. This instead averages
// the *exact* expectation of the best play over a sample of deals, so the
// only noise is which deals were drawn, not how they turned out.
//
// The sample lands a little low because a royal falls in four deals out of
// 2.6 million and contributes about 2% of the return: a sample this size will
// almost never contain one.
func TestReturnToPerfectPlay(t *testing.T) {
	if testing.Short() {
		t.Skip("the strategy sweep is slow")
	}

	r := rng.NewSeeded([32]byte{47})
	shoe := deck.NewShoe(1, 1, r)

	// Sixty deals is enough to land within a percent or two, and keeps the
	// sweep to about ten seconds.
	const deals = 60
	var total float64
	for i := 0; i < deals; i++ {
		shoe.Shuffle()
		var hand [5]deck.Card
		for j := range hand {
			hand[j] = shoe.Draw()
		}
		_, ev := BestHold(hand, MaxCoins)
		total += ev
	}

	// Per coin staked.
	rtp := total / float64(deals) / float64(MaxCoins)
	t.Logf("perfect play returns %.4f over %d deals", rtp, deals)

	if rtp < 0.93 || rtp > 1.02 {
		t.Errorf("return to perfect play is %.4f, want somewhere near 0.995", rtp)
	}
}
