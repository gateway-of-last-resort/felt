package roulette

import (
	"testing"
	"time"

	"github.com/gateway-of-last-resort/felt/internal/engine"
	"github.com/gateway-of-last-resort/felt/internal/rng"
)

const me = engine.PlayerID("p1")

func testTable(t *testing.T) *Table {
	t.Helper()
	tb := NewTable(rng.NewSeeded([32]byte{3}))
	if _, err := tb.Join(me, 0); err != nil {
		t.Fatalf("join: %v", err)
	}
	return tb
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
		t.Fatal("snapshot is not a roulette.Snapshot")
	}
	return s
}

// straightOn finds the spot that is a single number.
func straightOn(n int) int {
	for _, s := range Spots {
		if s.Type == Straight && s.Numbers[0] == n {
			return s.ID
		}
	}
	panic("no straight spot")
}

func TestPlaceAndStackBets(t *testing.T) {
	tb := testTable(t)
	spot := straightOn(17)

	apply(t, tb, PlaceBet{P: me, Spot: spot, Amount: 5})
	apply(t, tb, PlaceBet{P: me, Spot: spot, Amount: 5})

	s := snap(t, tb)
	if len(s.Bets) != 1 {
		t.Fatalf("got %d bets, want them stacked into 1", len(s.Bets))
	}
	if s.Bets[0].Amount != 10 {
		t.Errorf("stake = %d, want 10 after two fives", s.Bets[0].Amount)
	}
	if s.MyTotal != 10 {
		t.Errorf("my total = %d, want 10", s.MyTotal)
	}
	if !s.Bets[0].Mine {
		t.Error("my own chip is not marked as mine")
	}
}

// Lifting a chip returns it as a refund, which is credited but never counted
// as turnover.
func TestRemoveBetRefunds(t *testing.T) {
	tb := testTable(t)
	spot := straightOn(7)
	apply(t, tb, PlaceBet{P: me, Spot: spot, Amount: 25})

	events := apply(t, tb, RemoveBet{P: me, Spot: spot})
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	ref, ok := events[0].(engine.Refund)
	if !ok {
		t.Fatalf("got %T, want a refund", events[0])
	}
	if ref.Refund() != 25 {
		t.Errorf("refunded %d, want 25", ref.Refund())
	}
	if _, isSettlement := events[0].(engine.Settlement); isSettlement {
		t.Error("a lifted chip settled as a result; it would inflate turnover")
	}
	if got := snap(t, tb).MyTotal; got != 0 {
		t.Errorf("still %d on the table after lifting the chip", got)
	}
}

func TestClearBetsReturnsEverything(t *testing.T) {
	tb := testTable(t)
	apply(t, tb, PlaceBet{P: me, Spot: straightOn(1), Amount: 5})
	apply(t, tb, PlaceBet{P: me, Spot: straightOn(2), Amount: 10})

	events := apply(t, tb, ClearBets{P: me})

	var refunded int64
	for _, e := range events {
		if r, ok := e.(engine.Refund); ok {
			refunded += r.Refund()
		}
	}
	if refunded != 15 {
		t.Errorf("cleared the table for %d, want 15", refunded)
	}
	if got := snap(t, tb).MyTotal; got != 0 {
		t.Errorf("%d left on the table after clearing", got)
	}
}

// A spin settles every stake against one number and files the turnover.
func TestSpinSettles(t *testing.T) {
	tb := testTable(t)
	for n := 0; n < Pockets; n++ {
		apply(t, tb, PlaceBet{P: me, Spot: straightOn(n), Amount: 1})
	}

	events := apply(t, tb, Spin{P: me})

	spun, ok := eventOf[Spun](events)
	if !ok {
		t.Fatal("no Spun event")
	}
	if spun.Number < 0 || spun.Number >= Pockets {
		t.Fatalf("ball landed on %d", spun.Number)
	}

	settled, ok := eventOf[Settled](events)
	if !ok {
		t.Fatal("no Settled event")
	}
	// Every number covered: 37 staked, 36 back.
	if settled.Wagered != 37 || settled.Won != 36 {
		t.Errorf("settled %d staked for %d won, want 37 for 36", settled.Wagered, settled.Won)
	}
	if _, ok := eventOf[RoundEnded](events); !ok {
		t.Error("the round did not end")
	}

	s := snap(t, tb)
	if s.Phase != PhaseResult {
		t.Errorf("phase = %v after a spin, want the result", s.Phase)
	}
	if len(s.History) != 1 || s.History[0] != spun.Number {
		t.Errorf("history = %v, want just %d", s.History, spun.Number)
	}
	if s.MyTotal != 0 {
		t.Errorf("%d still on the layout after settling", s.MyTotal)
	}
}

func eventOf[T engine.Event](events []engine.Event) (T, bool) {
	for _, e := range events {
		if t, ok := e.(T); ok {
			return t, true
		}
	}
	var zero T
	return zero, false
}

// The wheel does not turn for free.
func TestSpinNeedsAStake(t *testing.T) {
	tb := testTable(t)
	if _, err := tb.Apply(Spin{P: me}, time.Now()); err == nil {
		t.Error("the wheel spun with nothing on the table")
	}
}

// Betting again after a result clears the previous round first.
func TestBettingAfterResultClearsTable(t *testing.T) {
	tb := testTable(t)
	apply(t, tb, PlaceBet{P: me, Spot: straightOn(5), Amount: 5})
	apply(t, tb, Spin{P: me})

	apply(t, tb, PlaceBet{P: me, Spot: straightOn(9), Amount: 5})

	s := snap(t, tb)
	if s.Phase != PhaseBetting {
		t.Errorf("phase = %v, want betting again", s.Phase)
	}
	if s.Number != -1 {
		t.Errorf("last number %d still showing during betting", s.Number)
	}
	if s.MyTotal != 5 {
		t.Errorf("my total = %d, want just the new chip", s.MyTotal)
	}
}

// The stakes of the previous round can be read back for the repeat key.
func TestRepeatBetsRemembersLastRound(t *testing.T) {
	tb := testTable(t)
	apply(t, tb, PlaceBet{P: me, Spot: straightOn(3), Amount: 5})
	apply(t, tb, PlaceBet{P: me, Spot: straightOn(4), Amount: 10})
	apply(t, tb, Spin{P: me})

	repeat := tb.RepeatBets(me)
	if len(repeat) != 2 {
		t.Fatalf("remembered %d bets, want 2", len(repeat))
	}
	var total int64
	for _, b := range repeat {
		total += b.Amount
	}
	if total != 15 {
		t.Errorf("remembered %d staked, want 15", total)
	}
}

// Table limits are enforced per spot, including across stacked chips.
func TestLimits(t *testing.T) {
	tb := testTable(t)
	tb.SetLimits(5, 20)
	spot := straightOn(11)

	if _, err := tb.Apply(PlaceBet{P: me, Spot: spot, Amount: 1}, time.Now()); err == nil {
		t.Error("a bet below the table minimum was accepted")
	}
	apply(t, tb, PlaceBet{P: me, Spot: spot, Amount: 15})
	if _, err := tb.Apply(PlaceBet{P: me, Spot: spot, Amount: 10}, time.Now()); err == nil {
		t.Error("stacking past the table maximum was accepted")
	}
	if _, err := tb.Apply(PlaceBet{P: me, Spot: 9999, Amount: 5}, time.Now()); err == nil {
		t.Error("a bet on a spot that does not exist was accepted")
	}
}

// Chips belonging to somebody else are visible but marked as theirs. This is
// the shared-table case the layout was built for.
func TestOtherPlayersChipsAreVisible(t *testing.T) {
	tb := testTable(t)
	const them = engine.PlayerID("p2")
	if _, err := tb.Join(them, 0); err != nil {
		t.Fatal(err)
	}

	apply(t, tb, PlaceBet{P: me, Spot: straightOn(1), Amount: 5})
	apply(t, tb, PlaceBet{P: them, Spot: straightOn(2), Amount: 7})

	s := snap(t, tb)
	if len(s.Bets) != 2 {
		t.Fatalf("see %d chips, want both", len(s.Bets))
	}
	if s.MyTotal != 5 {
		t.Errorf("my total = %d, want only my own 5", s.MyTotal)
	}
	var theirs BetView
	for _, b := range s.Bets {
		if !b.Mine {
			theirs = b
		}
	}
	if theirs.Amount != 7 || theirs.Player != them {
		t.Errorf("their chip = %+v, want 7 from p2", theirs)
	}
}

// Money left on the table by someone who disconnects is still settled.
func TestLeavingDoesNotAbandonChips(t *testing.T) {
	tb := testTable(t)
	const them = engine.PlayerID("p2")
	if _, err := tb.Join(them, 0); err != nil {
		t.Fatal(err)
	}
	apply(t, tb, PlaceBet{P: them, Spot: straightOn(0), Amount: 4})
	apply(t, tb, PlaceBet{P: me, Spot: straightOn(1), Amount: 4})

	tb.Leave(them)
	events := apply(t, tb, Spin{P: me})

	found := false
	for _, e := range events {
		if s, ok := e.(Settled); ok && s.P == them {
			found = true
			if s.Wagered != 4 {
				t.Errorf("the absent player staked %d, want 4", s.Wagered)
			}
		}
	}
	if !found {
		t.Error("the player who left was never settled; their money vanished")
	}
}

// Offline the wheel waits for the player. With a betting window it waits for
// the clock instead, and the player's spin is refused — the path a room takes.
func TestTimedBettingWindow(t *testing.T) {
	tb := testTable(t)
	tb.SetBettingTime(20 * time.Second)

	start := time.Now()
	if _, err := tb.Apply(PlaceBet{P: me, Spot: straightOn(8), Amount: 5}, start); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.Apply(Spin{P: me}, start); err == nil {
		t.Error("a player spun a table that spins on a timer")
	}

	tb.setDeadline(start, 20*time.Second)
	if events := tb.Tick(start.Add(19 * time.Second)); len(events) != 0 {
		t.Fatal("the wheel turned before the window closed")
	}

	events := tb.Tick(start.Add(21 * time.Second))
	if _, ok := eventOf[Spun](events); !ok {
		t.Fatal("the wheel did not turn when the window closed")
	}
	if _, ok := eventOf[BettingClosed](events); !ok {
		t.Error("betting was never announced as closed")
	}
}

// Offline there is no clock at all.
func TestNoDeadlineOffline(t *testing.T) {
	tb := testTable(t)
	if _, ok := tb.Deadline(); ok {
		t.Error("Deadline() reports a deadline with no betting window set")
	}
	if events := tb.Tick(time.Now().Add(time.Hour)); len(events) != 0 {
		t.Errorf("Tick produced %d events offline", len(events))
	}
}

// History is capped and newest-first, as on the board beside a real wheel.
func TestHistoryIsCapped(t *testing.T) {
	tb := testTable(t)
	for i := 0; i < HistoryLength+5; i++ {
		apply(t, tb, PlaceBet{P: me, Spot: straightOn(1), Amount: 1})
		apply(t, tb, Spin{P: me})
	}
	s := snap(t, tb)
	if len(s.History) != HistoryLength {
		t.Errorf("history holds %d numbers, want %d", len(s.History), HistoryLength)
	}
}

// Roulette bets from the wallet, so there is nothing to buy in with.
func TestBuyInRefused(t *testing.T) {
	tb := NewTable(rng.NewSeeded([32]byte{31}))
	if _, err := tb.Join(me, 50); err == nil {
		t.Fatal("the wheel accepted a buy-in")
	}
	if _, err := tb.Join(me, 0); err != nil {
		t.Errorf("a free seat was refused: %v", err)
	}
}
