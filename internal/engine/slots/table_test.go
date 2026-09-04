package slots

import (
	"testing"
	"time"

	"github.com/gateway-of-last-resort/felt/internal/engine"
	"github.com/gateway-of-last-resort/felt/internal/rng"
)

func testTable() *Table { return NewTable(rng.NewSeeded([32]byte{5})) }

func spin(t *testing.T, tb *Table, lines int, perLine int64) Spun {
	t.Helper()
	events, err := tb.Apply(Spin{P: engine.LocalPlayer, Lines: lines, PerLine: perLine}, time.Now())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	s, ok := events[0].(Spun)
	if !ok {
		t.Fatalf("got %T, want Spun", events[0])
	}
	return s
}

// The stake a spin puts at risk is what the driver debits, so it has to be
// the line bet times the lines and nothing else.
func TestSpinStake(t *testing.T) {
	s := Spin{P: engine.LocalPlayer, Lines: 5, PerLine: 10}
	if got := s.Stake(); got != 50 {
		t.Errorf("Stake() = %d, want 50", got)
	}
}

// A spin reports where the reels landed, and the window must agree with the
// stops: the presentation animates to the stops and then reads the window.
func TestSpinWindowMatchesStops(t *testing.T) {
	tb := testTable()
	s := spin(t, tb, 5, 1)

	for reel, stop := range s.Stops {
		if got, want := s.Window[reel], WindowAt(reel, stop); got != want {
			t.Errorf("reel %d parked on %d shows %v, want %v", reel, stop, got, want)
		}
	}
}

// Only active lines are scored.
func TestOnlyActiveLinesPay(t *testing.T) {
	// Every row the same symbol, so all five lines are three bells.
	window := [Reels][3]Symbol{
		{Bell, Bell, Bell},
		{Bell, Bell, Bell},
		{Bell, Bell, Bell},
	}

	for lines := 1; lines <= MaxLines; lines++ {
		wins, total := score(window, lines, 2)
		if len(wins) != lines {
			t.Errorf("%d active lines produced %d wins", lines, len(wins))
		}
		if want := threeOfKind[Bell] * 2 * int64(lines); total != want {
			t.Errorf("%d active lines paid %d, want %d", lines, total, want)
		}
	}
}

// A dead spin settles at zero rather than producing no event: the turnover
// still belongs in the statistics.
func TestLosingSpinStillSettles(t *testing.T) {
	window := [Reels][3]Symbol{
		{Bell, Bar, Lemon},
		{Seven, Lemon, Bar},
		{Bar, Bell, Seven},
	}
	wins, total := score(window, MaxLines, 5)
	if len(wins) != 0 || total != 0 {
		t.Fatalf("expected a dead window, got %d wins worth %d", len(wins), total)
	}

	// And the event carries the wager regardless.
	s := Spun{P: engine.LocalPlayer, Wagered: 25, Total: 0}
	wagered, won := s.Result()
	if wagered != 25 || won != 0 {
		t.Errorf("Result() = %d/%d, want 25/0", wagered, won)
	}
	if s.Game() != engine.KindSlots {
		t.Errorf("Game() = %v, want slots", s.Game())
	}
}

// The engine refuses a bet it does not offer, rather than quietly rounding it
// into one that it does.
func TestInvalidBetsRefused(t *testing.T) {
	cases := []struct {
		name    string
		lines   int
		perLine int64
	}{
		{"no lines", 0, 1},
		{"too many lines", MaxLines + 1, 1},
		{"stake off the ladder", 5, 3},
		{"zero stake", 5, 0},
		{"negative stake", 5, -5},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tb := testTable()
			_, err := tb.Apply(Spin{P: engine.LocalPlayer, Lines: c.lines, PerLine: c.perLine}, time.Now())
			if err == nil {
				t.Fatal("the engine accepted an invalid spin")
			}
		})
	}
}

// A slot machine waits for a player, never for a clock.
func TestNoDeadlines(t *testing.T) {
	tb := testTable()
	if _, ok := tb.Deadline(); ok {
		t.Error("Deadline() reports a deadline")
	}
	if events := tb.Tick(time.Now()); len(events) != 0 {
		t.Errorf("Tick produced %d events", len(events))
	}
}

// The snapshot follows the last spin, which is what a reconnecting screen
// draws before anything else happens.
func TestSnapshotTracksLastSpin(t *testing.T) {
	tb := testTable()
	s := spin(t, tb, 5, 2)

	snap, ok := tb.Snapshot(engine.LocalPlayer).(Snapshot)
	if !ok {
		t.Fatal("Snapshot is not a slots.Snapshot")
	}
	if snap.Stops != s.Stops {
		t.Errorf("snapshot stops %v, spin stops %v", snap.Stops, s.Stops)
	}
	if snap.LastWin != s.Total {
		t.Errorf("snapshot win %d, spin total %d", snap.LastWin, s.Total)
	}
	if !snap.Deadline.IsZero() {
		t.Error("snapshot carries a deadline")
	}
}

// One machine, one player.
func TestSeatingIsExclusive(t *testing.T) {
	tb := testTable()
	if _, err := tb.Join("a"); err != nil {
		t.Fatalf("first player refused: %v", err)
	}
	if _, err := tb.Join("b"); err == nil {
		t.Error("a second player was seated at a slot machine")
	}
	tb.Leave("a")
	if _, err := tb.Join("b"); err != nil {
		t.Errorf("seat not released on leave: %v", err)
	}
}

// A win has to say how many symbols made it. Three of a kind and the
// two-cherry consolation pay differently and look different on the reels, so
// a screen that cannot tell them apart ends up claiming three cherries over a
// picture of two.
func TestWinsReportTheirSymbolCount(t *testing.T) {
	cases := []struct {
		name   string
		window [Reels][3]Symbol
		symbol Symbol
		count  int
	}{
		{
			name:   "three of a kind",
			window: [Reels][3]Symbol{{Bell, Bell, Bell}, {Bell, Bell, Bell}, {Bell, Bell, Bell}},
			symbol: Bell,
			count:  3,
		},
		{
			name:   "two cherries and something else",
			window: [Reels][3]Symbol{{Cherry, Cherry, Cherry}, {Cherry, Cherry, Cherry}, {Bell, Bell, Bell}},
			symbol: Cherry,
			count:  2,
		},
		{
			name:   "wild standing in for a cherry",
			window: [Reels][3]Symbol{{Wild, Wild, Wild}, {Cherry, Cherry, Cherry}, {Bell, Bell, Bell}},
			symbol: Cherry,
			count:  2,
		},
		{
			name:   "three cherries",
			window: [Reels][3]Symbol{{Cherry, Cherry, Cherry}, {Cherry, Cherry, Cherry}, {Cherry, Cherry, Cherry}},
			symbol: Cherry,
			count:  3,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wins, _ := score(c.window, 1, 10)
			if len(wins) != 1 {
				t.Fatalf("got %d wins, want 1", len(wins))
			}
			if wins[0].Symbol != c.symbol {
				t.Errorf("symbol = %v, want %v", wins[0].Symbol, c.symbol)
			}
			if wins[0].Count != c.count {
				t.Errorf("count = %d, want %d", wins[0].Count, c.count)
			}
			// And the count has to agree with what was paid.
			if c.count == 2 && wins[0].Mult != TwoCherries {
				t.Errorf("a two-symbol win paid ×%d, want ×%d", wins[0].Mult, TwoCherries)
			}
			if c.count == 3 && wins[0].Mult != threeOfKind[c.symbol] {
				t.Errorf("a three-symbol win paid ×%d, want ×%d",
					wins[0].Mult, threeOfKind[c.symbol])
			}
		})
	}
}
