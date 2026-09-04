package blackjack

import (
	"testing"
	"time"

	"github.com/gateway-of-last-resort/felt/internal/deck"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	"github.com/gateway-of-last-resort/felt/internal/rng"
)

const me = engine.PlayerID("p1")

// newStacked builds a one-seat table whose shoe deals the given cards in
// order, so a test can set up the exact hands it is about.
func newStacked(t *testing.T, spec string) *Table {
	t.Helper()
	tb := NewTable(rng.NewSeeded([32]byte{7}), Vegas6(), 1)
	tb.shoe.Stack(cards(spec))
	if _, err := tb.Join(me, 0); err != nil {
		t.Fatalf("join: %v", err)
	}
	return tb
}

func cards(spec string) []deck.Card {
	out := make([]deck.Card, 0, len(spec))
	i := 0
	for _, r := range spec {
		if r == ' ' {
			continue
		}
		var rank deck.Rank
		switch r {
		case 'A':
			rank = deck.Ace
		case 'K':
			rank = deck.King
		case 'Q':
			rank = deck.Queen
		case 'J':
			rank = deck.Jack
		case 'T':
			rank = deck.Ten
		default:
			if r < '2' || r > '9' {
				panic("bad card in spec: " + string(r))
			}
			rank = deck.Rank(r - '0')
		}
		out = append(out, deck.Card{Rank: rank, Suit: deck.Suit(i % 4)})
		i++
	}
	return out
}

func apply(t *testing.T, tb *Table, a engine.Action) []engine.Event {
	t.Helper()
	events, err := tb.Apply(a, time.Now())
	if err != nil {
		t.Fatalf("%T: %v", a, err)
	}
	return events
}

func snap(t *testing.T, tb *Table) Snapshot {
	t.Helper()
	s, ok := tb.Snapshot(me).(Snapshot)
	if !ok {
		t.Fatal("snapshot is not a blackjack.Snapshot")
	}
	return s
}

func countDealt(events []engine.Event) (player, dealer int) {
	for _, e := range events {
		c, ok := e.(CardDealt)
		if !ok {
			continue
		}
		if c.Seat == engine.DealerSeat {
			dealer++
			continue
		}
		player++
	}
	return
}

// Placing a bet deals the whole opening round in one go: four cards, the
// dealer's second one hidden.
func TestBetDealsOpeningRound(t *testing.T) {
	tb := newStacked(t, "9 8 7 6")
	events := apply(t, tb, PlaceBet{P: me, Amount: 10})

	player, dealer := countDealt(events)
	if player != 2 || dealer != 2 {
		t.Fatalf("dealt %d to the player and %d to the dealer, want 2 and 2", player, dealer)
	}

	hidden := 0
	for _, e := range events {
		if c, ok := e.(CardDealt); ok && c.Hidden {
			hidden++
		}
	}
	if hidden != 1 {
		t.Errorf("%d cards dealt face down, want exactly the hole card", hidden)
	}

	s := snap(t, tb)
	if s.Phase != PhasePlayerTurn {
		t.Errorf("phase = %v, want the player's turn", s.Phase)
	}
	// 9 and 7 to the player, 8 and 6 to the dealer.
	if got := s.Seats[0].Hands[0].Total; got != 16 {
		t.Errorf("player total = %d, want 16", got)
	}
}

// The hole card must not appear in a snapshot until it is turned over — this
// is the leak that matters once snapshots travel to other terminals.
func TestHoleCardStaysHidden(t *testing.T) {
	tb := newStacked(t, "9 8 7 6 5")
	apply(t, tb, PlaceBet{P: me, Amount: 10})

	s := snap(t, tb)
	if !s.HoleHidden {
		t.Fatal("snapshot does not report a hidden hole card")
	}
	if got := len(s.Dealer.Cards); got != 1 {
		t.Fatalf("snapshot exposes %d dealer cards during the player's turn, want 1", got)
	}

	apply(t, tb, Stand{P: me})

	s = snap(t, tb)
	if s.HoleHidden {
		t.Error("hole card still hidden after the dealer's turn")
	}
	if got := len(s.Dealer.Cards); got < 2 {
		t.Errorf("dealer shows %d cards after standing, want at least 2", got)
	}
}

// A player blackjack settles at once, with no dealer draw.
func TestPlayerBlackjackSettlesImmediately(t *testing.T) {
	tb := newStacked(t, "A 9 K 7")
	events := apply(t, tb, PlaceBet{P: me, Amount: 10})

	settled := settlements(events)
	if len(settled) != 1 {
		t.Fatalf("got %d settlements, want 1", len(settled))
	}
	if settled[0].Outcome != Blackjack {
		t.Errorf("outcome = %v, want Blackjack", settled[0].Outcome)
	}
	if settled[0].Payout != 25 {
		t.Errorf("payout = %d on a 10 blackjack, want 25", settled[0].Payout)
	}
	if snap(t, tb).Phase != PhaseSettle {
		t.Error("round did not end")
	}
}

// The dealer peeks under a ten, so the round is over before the player can
// double into a hand that was already lost.
func TestDealerPeekEndsRound(t *testing.T) {
	tb := newStacked(t, "9 K 8 A")
	events := apply(t, tb, PlaceBet{P: me, Amount: 10})

	settled := settlements(events)
	if len(settled) != 1 || settled[0].Outcome != Lose {
		t.Fatalf("settlements = %+v, want a single loss", settled)
	}
	if snap(t, tb).Phase != PhaseSettle {
		t.Error("round continued past a dealer blackjack")
	}
}

func settlements(events []engine.Event) []HandSettled {
	var out []HandSettled
	for _, e := range events {
		if s, ok := e.(HandSettled); ok {
			out = append(out, s)
		}
	}
	return out
}

// An ace up offers insurance, and taking it costs exactly half the stake.
func TestInsuranceAtTheTable(t *testing.T) {
	tb := newStacked(t, "9 A 8 K")
	apply(t, tb, PlaceBet{P: me, Amount: 10})

	if got := snap(t, tb).Phase; got != PhaseInsurance {
		t.Fatalf("phase = %v with an ace up, want insurance", got)
	}

	// The amount is checked against the table, so a wrong one buys nothing.
	if _, err := tb.Apply(Insure{P: me, Yes: true, Amount: 1}, time.Now()); err == nil {
		t.Error("the table accepted insurance at the wrong price")
	}

	events := apply(t, tb, Insure{P: me, Yes: true, Amount: 5})

	settled := settlements(events)
	if len(settled) != 1 {
		t.Fatalf("got %d settlements, want 1", len(settled))
	}
	// Dealer had blackjack: the main bet loses, insurance returns 15, and the
	// seat staked 15 in total. A wash.
	if settled[0].Wagered != 15 || settled[0].Payout != 15 {
		t.Errorf("settled %d staked for %d returned, want 15 for 15",
			settled[0].Wagered, settled[0].Payout)
	}
}

// Declining insurance carries on into the same peek.
func TestInsuranceDeclined(t *testing.T) {
	tb := newStacked(t, "9 A 8 9 5")
	apply(t, tb, PlaceBet{P: me, Amount: 10})
	apply(t, tb, Insure{P: me, Yes: false})

	if got := snap(t, tb).Phase; got != PhasePlayerTurn {
		t.Errorf("phase = %v after declining, want the player's turn", got)
	}
	if got := snap(t, tb).Seats[0].Insurance; got != 0 {
		t.Errorf("insurance = %d after declining, want 0", got)
	}
}

// Splitting produces two hands, each with the original stake and a fresh
// second card.
func TestSplit(t *testing.T) {
	tb := newStacked(t, "8 9 8 7 3 2 6 6")
	apply(t, tb, PlaceBet{P: me, Amount: 10})

	if _, err := tb.Apply(Split{P: me, Amount: 4}, time.Now()); err == nil {
		t.Error("the table accepted a split at the wrong price")
	}

	events := apply(t, tb, Split{P: me, Amount: 10})
	if _, ok := engineEvent[HandSplit](events); !ok {
		t.Error("no HandSplit event")
	}

	s := snap(t, tb)
	hands := s.Seats[0].Hands
	if len(hands) != 2 {
		t.Fatalf("holding %d hands after a split, want 2", len(hands))
	}
	for i, h := range hands {
		if len(h.Cards) != 2 {
			t.Errorf("hand %d holds %d cards, want 2", i, len(h.Cards))
		}
		if h.Bet != 10 {
			t.Errorf("hand %d staked %d, want 10", i, h.Bet)
		}
		if !h.FromSplit {
			t.Errorf("hand %d is not marked as split", i)
		}
	}
}

func engineEvent[T engine.Event](events []engine.Event) (T, bool) {
	for _, e := range events {
		if t, ok := e.(T); ok {
			return t, true
		}
	}
	var zero T
	return zero, false
}

// Split aces take one card each and play passes on.
func TestSplitAcesGetOneCard(t *testing.T) {
	tb := newStacked(t, "A 9 A 7 K Q 5 5")
	apply(t, tb, PlaceBet{P: me, Amount: 10})
	apply(t, tb, Split{P: me, Amount: 10})

	s := snap(t, tb)
	for i, h := range s.Seats[0].Hands {
		if len(h.Cards) != 2 {
			t.Errorf("split ace hand %d holds %d cards, want 2", i, len(h.Cards))
		}
		if h.Blackjack {
			t.Errorf("split ace hand %d counts as blackjack", i)
		}
	}
	if s.Phase == PhasePlayerTurn {
		t.Error("still the player's turn after splitting aces")
	}
}

// Doubling takes a matching stake, draws one card and closes the hand.
func TestDouble(t *testing.T) {
	tb := newStacked(t, "6 9 5 7 K 4")
	apply(t, tb, PlaceBet{P: me, Amount: 10})
	events := apply(t, tb, Double{P: me, Amount: 10})

	player, _ := countDealt(events)
	if player != 1 {
		t.Errorf("doubling drew %d cards, want exactly 1", player)
	}

	s := snap(t, tb)
	h := s.Seats[0].Hands[0]
	if !h.Doubled || h.Bet != 20 {
		t.Errorf("hand after doubling: doubled=%v bet=%d, want true and 20", h.Doubled, h.Bet)
	}
	if s.Phase == PhasePlayerTurn {
		t.Error("still acting after a double")
	}
}

// Busting out skips the dealer's draw: there is nothing left to beat.
func TestBustSkipsDealerDraw(t *testing.T) {
	tb := newStacked(t, "K 6 Q 5 5 4 3")
	apply(t, tb, PlaceBet{P: me, Amount: 10})

	events := apply(t, tb, Hit{P: me})
	_, dealerDrew := countDealt(events)
	if dealerDrew != 0 {
		t.Errorf("dealer drew %d cards with nothing to beat", dealerDrew)
	}

	settled := settlements(events)
	if len(settled) != 1 || settled[0].Payout != 0 {
		t.Errorf("settlements = %+v, want a single loss", settled)
	}
}

// Standing hands over to the dealer, who draws to seventeen.
func TestDealerDrawsToSeventeen(t *testing.T) {
	tb := newStacked(t, "K 6 Q 5 4 3")
	apply(t, tb, PlaceBet{P: me, Amount: 10})
	apply(t, tb, Stand{P: me})

	s := snap(t, tb)
	if got := s.Dealer.Total; got != 18 {
		t.Errorf("dealer stopped on %d, want 18", got)
	}
	if got := s.Seats[0].Hands[0].Outcome; got != Win {
		t.Errorf("outcome = %v with 20 against 18, want Win", got)
	}
}

// Every settlement carries the wager exactly once, or the statistics would
// double-count a split.
func TestWagerCountedOnce(t *testing.T) {
	tb := newStacked(t, "8 9 8 7 3 2 6 6 6 6")
	apply(t, tb, PlaceBet{P: me, Amount: 10})
	events := apply(t, tb, Split{P: me, Amount: 10})
	events = append(events, apply(t, tb, Stand{P: me})...)
	events = append(events, apply(t, tb, Stand{P: me})...)

	var wagered int64
	for _, s := range settlements(events) {
		wagered += s.Wagered
	}
	if wagered != 20 {
		t.Errorf("settlements report %d staked across a split, want 20", wagered)
	}
}

// Offline the table has no clock at all.
func TestNoDeadlinesWithoutTimers(t *testing.T) {
	tb := newStacked(t, "9 8 7 6")
	apply(t, tb, PlaceBet{P: me, Amount: 10})

	if _, ok := tb.Deadline(); ok {
		t.Error("Deadline() reports a deadline with no timers set")
	}
	if events := tb.Tick(time.Now().Add(time.Hour)); len(events) != 0 {
		t.Errorf("Tick produced %d events with no timers set", len(events))
	}
}

// With a turn clock, running out of time stands the hand. This is the path a
// server room takes; offline it never runs.
func TestTurnTimeoutStands(t *testing.T) {
	tb := newStacked(t, "9 8 7 6 5")
	tb.SetTimers(15*time.Second, 20*time.Second)

	start := time.Now()
	if _, err := tb.Apply(PlaceBet{P: me, Amount: 10}, start); err != nil {
		t.Fatal(err)
	}

	deadline, ok := tb.Deadline()
	if !ok {
		t.Fatal("no deadline on the player's turn")
	}
	if want := start.Add(20 * time.Second); !deadline.Equal(want) {
		t.Errorf("deadline = %v, want %v", deadline, want)
	}

	if events := tb.Tick(start.Add(19 * time.Second)); len(events) != 0 {
		t.Fatal("Tick acted before the deadline")
	}

	tb.Tick(start.Add(21 * time.Second))
	if got := snap(t, tb).Phase; got != PhaseSettle {
		t.Errorf("phase = %v after the turn timed out, want the round settled", got)
	}
}

// Bets are checked against the table limits, and oddness is a limit here.
func TestBetValidation(t *testing.T) {
	tb := newStacked(t, "9 8 7 6")
	r := tb.Rules()

	for _, bad := range []int64{0, -2, 1, r.MinBet + 1, r.MaxBet + 2} {
		if _, err := tb.Apply(PlaceBet{P: me, Amount: bad}, time.Now()); err == nil {
			t.Errorf("the table accepted a bet of %d", bad)
		}
	}
	if _, err := tb.Apply(PlaceBet{P: me, Amount: 10}, time.Now()); err != nil {
		t.Errorf("the table refused a legal bet: %v", err)
	}
}

// Acting out of turn is refused rather than applied to somebody's hand.
func TestActionsOutsideTurnRefused(t *testing.T) {
	tb := newStacked(t, "9 8 7 6")

	if _, err := tb.Apply(Hit{P: me}, time.Now()); err == nil {
		t.Error("hit was accepted before the round started")
	}
	if _, err := tb.Apply(Hit{P: "stranger"}, time.Now()); err == nil {
		t.Error("a player with no seat was allowed to act")
	}
}

// A seat is kept across a reconnect, and its hands are not thrown away.
func TestRejoinKeepsSeat(t *testing.T) {
	tb := newStacked(t, "9 8 7 6 5")
	apply(t, tb, PlaceBet{P: me, Amount: 10})

	tb.Leave(me)
	seat, err := tb.Join(me, 0)
	if err != nil {
		t.Fatalf("rejoin refused: %v", err)
	}
	if seat != 0 {
		t.Errorf("rejoined at seat %d, want 0", seat)
	}
	if got := len(snap(t, tb).Seats[0].Hands); got == 0 {
		t.Error("hands were discarded when the player dropped mid-round")
	}
}

// Round after round, at the same table.
//
// Every other test here plays one round, which is exactly how a table that
// accepts a single bet and then refuses forever went unnoticed: settling
// leaves the seat marked as playing, and the next bet was tested against
// that mark before the table had been cleared.
func TestManyRoundsInARow(t *testing.T) {
	tb := NewTable(rng.NewSeeded([32]byte{23}), Vegas6(), 1)
	if _, err := tb.Join(me, 0); err != nil {
		t.Fatal(err)
	}

	for round := 1; round <= 25; round++ {
		events, err := tb.Apply(PlaceBet{P: me, Amount: 10}, time.Now())
		if err != nil {
			t.Fatalf("round %d: the table refused a bet: %v", round, err)
		}
		if _, ok := engineEvent[BetPlaced](events); !ok {
			t.Fatalf("round %d: the bet was never placed", round)
		}

		// Play it out however it lands.
		for i := 0; i < 20; i++ {
			s := snap(t, tb)
			switch s.Phase {
			case bjPhaseInsurance:
				if _, err := tb.Apply(Insure{P: me, Yes: false}, time.Now()); err != nil {
					t.Fatalf("round %d: declining insurance failed: %v", round, err)
				}
			case bjPhasePlayerTurn:
				if _, err := tb.Apply(Stand{P: me}, time.Now()); err != nil {
					t.Fatalf("round %d: standing failed: %v", round, err)
				}
			}
			if snap(t, tb).Phase == PhaseSettle {
				break
			}
		}

		if got := snap(t, tb).Phase; got != PhaseSettle {
			t.Fatalf("round %d ended in phase %v, want it settled", round, got)
		}
	}
}

// Aliases that keep the loop above readable.
const (
	bjPhaseInsurance  = PhaseInsurance
	bjPhasePlayerTurn = PhasePlayerTurn
)

// Settling leaves the seat ready for the next bet rather than stuck in the
// round that just finished.
func TestSeatIsFreeAfterSettling(t *testing.T) {
	tb := newStacked(t, "A 9 K 7 5 5 5 5")
	apply(t, tb, PlaceBet{P: me, Amount: 10})

	if got := snap(t, tb).Phase; got != PhaseSettle {
		t.Fatalf("phase = %v, want the round settled", got)
	}
	if _, err := tb.Apply(PlaceBet{P: me, Amount: 10}, time.Now()); err != nil {
		t.Fatalf("the table refused the next bet: %v", err)
	}
}

// Blackjack bets from the wallet: no stacks, no buy-in.
func TestBuyInRefused(t *testing.T) {
	tb := NewTable(rng.NewSeeded([32]byte{29}), Vegas6(), 1)
	if _, err := tb.Join(me, 20); err == nil {
		t.Fatal("the table accepted a buy-in")
	}
	if _, err := tb.Join(me, 0); err != nil {
		t.Errorf("a free seat was refused: %v", err)
	}
}
