package blackjack

import (
	"testing"

	"github.com/gateway-of-last-resort/felt/internal/deck"
	"github.com/gateway-of-last-resort/felt/internal/rng"
)

// hand builds a hand from a compact spelling: "A" ace, "T" ten, "2".."9",
// "J", "Q", "K". Suits do not matter to the rules, so they are dealt round.
func hand(spec string, bet int64) Hand {
	h := Hand{Bet: bet}
	for i, r := range spec {
		var rank deck.Rank
		switch r {
		case 'A':
			rank = deck.Ace
		case 'T':
			rank = deck.Ten
		case 'J':
			rank = deck.Jack
		case 'Q':
			rank = deck.Queen
		case 'K':
			rank = deck.King
		default:
			if r < '2' || r > '9' {
				panic("bad card in hand spec: " + string(r))
			}
			rank = deck.Rank(r - '0')
		}
		h.Add(deck.Card{Rank: rank, Suit: deck.Suit(i % 4)})
	}
	return h
}

// Ace counting is the classic place to get blackjack wrong: aces go in at
// eleven and drop to one only as far as they have to.
func TestHandValue(t *testing.T) {
	cases := []struct {
		spec  string
		total int
		soft  bool
	}{
		{"AAA8", 21, true}, // three aces and an eight: 11+1+1+8
		{"AT", 21, true},   // blackjack is soft
		{"A5T", 16, false}, // the ace had to come down: hard 16
		{"KQA", 21, false}, // ace forced to one
		{"AA", 12, true},   // 11+1
		{"AAAA", 14, true}, // 11+1+1+1
		{"TT2", 22, false}, // bust
		{"96", 15, false},
		{"A9", 20, true},
		{"A9T", 20, false},
		{"AAT9", 21, false},
	}

	for _, c := range cases {
		t.Run(c.spec, func(t *testing.T) {
			total, soft := hand(c.spec, 2).Value()
			if total != c.total || soft != c.soft {
				t.Errorf("Value() = %d soft=%v, want %d soft=%v", total, soft, c.total, c.soft)
			}
		})
	}
}

func TestIsBlackjack(t *testing.T) {
	if !hand("AT", 2).IsBlackjack() {
		t.Error("A,10 is not reported as blackjack")
	}
	if !hand("KA", 2).IsBlackjack() {
		t.Error("K,A is not reported as blackjack")
	}
	if hand("AAT", 2).IsBlackjack() {
		t.Error("a three-card 21 is reported as blackjack")
	}
	if hand("KQ", 2).IsBlackjack() {
		t.Error("K,Q is reported as blackjack")
	}

	// Twenty-one out of a split pays even money, so it must not be blackjack.
	h := hand("AT", 2)
	h.FromSplit = true
	if h.IsBlackjack() {
		t.Error("a split 21 is reported as blackjack")
	}
}

// The full settlement table. Amounts are what returns to the player, stake
// included, on a bet of 10.
func TestSettle(t *testing.T) {
	r := Vegas6()
	const bet = 10

	cases := []struct {
		name    string
		player  string
		dealer  string
		outcome Outcome
		returns int64
	}{
		{"blackjack against blackjack pushes", "AT", "AK", Push, bet},
		{"blackjack pays 3:2", "AT", "KQ", Blackjack, 25},
		{"dealer blackjack beats twenty", "KQ", "AT", Lose, 0},
		{"player bust loses to dealer bust", "TT5", "T96", Lose, 0},
		{"player bust loses", "TT5", "KQ", Lose, 0},
		{"dealer bust pays", "T9", "T96", Win, 2 * bet},
		{"higher total wins", "T9", "T8", Win, 2 * bet},
		{"lower total loses", "T7", "T8", Lose, 0},
		{"equal totals push", "T8", "K8", Push, bet},
		{"twenty-one beats twenty", "T56", "KT", Win, 2 * bet},
		{"three-card 21 does not beat blackjack", "T56", "AK", Lose, 0},
		{"dealer 17 stands and loses to 18", "T8", "T7", Win, 2 * bet},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, ret := Settle(hand(c.player, bet), hand(c.dealer, bet), r)
			if out != c.outcome {
				t.Errorf("outcome = %v, want %v", out, c.outcome)
			}
			if ret != c.returns {
				t.Errorf("returns = %d, want %d", ret, c.returns)
			}
		})
	}
}

// Surrender gives back half the stake, whatever the dealer is holding.
func TestSettleSurrender(t *testing.T) {
	r := Vegas6()
	p := hand("T6", 10)
	p.Surrender = true

	out, ret := Settle(p, hand("AK", 10), r)
	if out != Surrendered {
		t.Errorf("outcome = %v, want Surrendered", out)
	}
	if ret != 5 {
		t.Errorf("returns = %d, want 5", ret)
	}
}

// A doubled hand carries twice the stake, so it returns twice as much.
func TestSettleDoubled(t *testing.T) {
	p := hand("T65", 20) // 21 on three cards, doubled from a stake of 10
	p.Doubled = true

	_, ret := Settle(p, hand("T7", 10), Vegas6())
	if ret != 40 {
		t.Errorf("returns = %d on a doubled winner, want 40", ret)
	}
}

// Every legal bet must settle in whole credits, blackjack included. That is
// the entire reason bets are even.
func TestBlackjackPaysWholeCredits(t *testing.T) {
	r := Vegas6()
	for bet := r.MinBet; bet <= r.MaxBet; bet += r.MinBet {
		_, ret := Settle(hand("AT", bet), hand("KQ", bet), r)
		if want := bet + bet*3/2; ret != want {
			t.Fatalf("bet %d returned %d, want %d", bet, ret, want)
		}
		if (bet*3)%2 != 0 {
			t.Fatalf("bet %d cannot pay 3:2 in whole credits", bet)
		}
		if InsuranceCost(bet)*2 != bet {
			t.Fatalf("insurance on %d is not half the stake", bet)
		}
	}
}

func TestInsurance(t *testing.T) {
	dealerBJ := hand("AK", 10)
	dealerNot := hand("A9", 10)

	// Insuring a 10 bet costs 5 and returns 15 when the dealer has it.
	if got := SettleInsurance(5, dealerBJ); got != 15 {
		t.Errorf("insurance returned %d against a dealer blackjack, want 15", got)
	}
	if got := SettleInsurance(5, dealerNot); got != 0 {
		t.Errorf("insurance returned %d with no dealer blackjack, want 0", got)
	}
	if !DealerShowsAce(dealerNot) {
		t.Error("an ace up is not offering insurance")
	}
	if DealerShowsAce(hand("KA", 10)) {
		t.Error("a king up is offering insurance")
	}
}

// Insuring a blackjack against a dealer blackjack is the break-even case that
// tempts everyone: 10 staked, 5 insurance, push plus 15 back.
func TestInsuredPushBreaksEven(t *testing.T) {
	p, d := hand("AT", 10), hand("AK", 10)

	_, main := Settle(p, d, Vegas6())
	total := main + SettleInsurance(InsuranceCost(10), d)
	if want := int64(10 + 15); total != want {
		t.Errorf("insured blackjack push returned %d, want %d", total, want)
	}
}

func TestDealerHitsToSeventeen(t *testing.T) {
	s17 := Vegas6()
	h17 := Vegas6()
	h17.DealerHitsSoft17 = true

	cases := []struct {
		spec       string
		s17H, h17H bool
	}{
		{"T6", true, true},   // hard 16
		{"T7", false, false}, // hard 17
		{"A6", false, true},  // soft 17 — the rule that differs
		{"A7", false, false}, // soft 18
		{"AA5", false, true}, // soft 17 the long way
		{"96", true, true},   // 15
	}

	for _, c := range cases {
		t.Run(c.spec, func(t *testing.T) {
			if got := ShouldHit(hand(c.spec, 2), s17); got != c.s17H {
				t.Errorf("S17: ShouldHit = %v, want %v", got, c.s17H)
			}
			if got := ShouldHit(hand(c.spec, 2), h17); got != c.h17H {
				t.Errorf("H17: ShouldHit = %v, want %v", got, c.h17H)
			}
		})
	}
}

// The dealer never stops short of seventeen and never draws past it.
func TestPlayDealerLandsInRange(t *testing.T) {
	r := Vegas6()
	shoe := deck.NewShoe(r.Decks, r.Penetration, rng.NewSeeded([32]byte{4}))

	for i := 0; i < 5000; i++ {
		h := Hand{}
		h.Add(shoe.Draw())
		h.Add(shoe.Draw())
		h = PlayDealer(h, shoe, r)

		total, soft := h.Value()
		if total < 17 {
			t.Fatalf("dealer stood on %d", total)
		}
		if total == 17 && soft && r.DealerHitsSoft17 {
			t.Fatal("dealer stood on soft 17 at an H17 table")
		}
		if shoe.NeedsShuffle() {
			shoe.Shuffle()
		}
	}
}

func TestCanSplit(t *testing.T) {
	r := Vegas6()

	if !hand("88", 10).CanSplit(r, 0) {
		t.Error("a pair of eights cannot be split")
	}
	// Pairs go by value, so mixed tens split.
	if !hand("KQ", 10).CanSplit(r, 0) {
		t.Error("king-queen cannot be split")
	}
	if hand("87", 10).CanSplit(r, 0) {
		t.Error("eight-seven can be split")
	}
	if hand("888", 10).CanSplit(r, 0) {
		t.Error("a three-card hand can be split")
	}
	if hand("88", 10).CanSplit(r, r.MaxSplits) {
		t.Error("splitting is allowed past the limit")
	}

	aces := hand("AA", 10)
	l, _ := aces.Split()
	l.Add(deck.Card{Rank: deck.Nine, Suit: deck.Spades})
	if l.CanSplit(r, 1) {
		t.Error("a split-ace hand can be split again")
	}
	if !l.Done(r) {
		t.Error("a split ace with its one card is not done")
	}
}

func TestCanDouble(t *testing.T) {
	r := Vegas6()

	if !hand("65", 10).CanDouble(r) {
		t.Error("a two-card hand cannot double")
	}
	if hand("654", 10).CanDouble(r) {
		t.Error("a three-card hand can double")
	}

	split := hand("65", 10)
	split.FromSplit = true
	if !split.CanDouble(r) {
		t.Error("double after split is refused at a table that allows it")
	}

	noDAS := r
	noDAS.DoubleAfterSplit = false
	if split.CanDouble(noDAS) {
		t.Error("double after split is allowed at a table that forbids it")
	}
}

// Splitting hands out the stake correctly: each half carries the full
// original bet, and neither can make blackjack.
func TestSplitCarriesStake(t *testing.T) {
	l, rt := hand("88", 10).Split()

	for name, h := range map[string]Hand{"left": l, "right": rt} {
		if h.Bet != 10 {
			t.Errorf("%s hand staked %d, want 10", name, h.Bet)
		}
		if len(h.Cards) != 1 {
			t.Errorf("%s hand holds %d cards, want 1", name, len(h.Cards))
		}
		if !h.FromSplit {
			t.Errorf("%s hand is not marked as split", name)
		}
	}
}
