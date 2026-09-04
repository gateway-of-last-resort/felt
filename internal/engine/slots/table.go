// Package slots is the three-reel slot machine's rules.
//
// The machine has no timers and no turns: a spin is one action in and one
// event out. All the interest is in the maths, which paytable.go computes
// exactly rather than by simulation.
package slots

import (
	"math/rand/v2"
	"time"

	"github.com/gateway-of-last-resort/felt/internal/engine"
)

// LineBets are the selectable stakes per line.
var LineBets = []int64{1, 2, 5, 10}

// MaxLines is how many paylines can be switched on.
var MaxLines = len(Paylines)

// Spin is the only action: stake PerLine on each of Lines paylines.
type Spin struct {
	P       engine.PlayerID
	Lines   int
	PerLine int64
}

// Player satisfies engine.Action.
func (s Spin) Player() engine.PlayerID { return s.P }

// Stake satisfies engine.Stake: the total put at risk, which is what the
// driver debits before the spin is applied.
func (s Spin) Stake() int64 { return s.PerLine * int64(s.Lines) }

// LineWin is one payline that paid.
type LineWin struct {
	Line   int
	Symbol Symbol

	// Count is how many symbols actually made the win: three for a line, two
	// for the cherry consolation. Without it a screen has no way to tell the
	// two apart, and saying "three of a kind" over a pair of cherries makes
	// the machine look broken.
	Count int

	Mult int64
	Pays int64
}

// Spun reports a finished spin: where the reels landed and what it paid. The
// presentation animates towards Stops, which are already decided — the reels
// are catching up with a result, not producing it.
type Spun struct {
	P       engine.PlayerID
	Stops   [Reels]int
	Window  [Reels][3]Symbol
	Wins    []LineWin
	Wagered int64
	Total   int64
}

// Event satisfies engine.Event.
func (Spun) Event() {}

// Payee satisfies engine.Settlement.
func (s Spun) Payee() engine.PlayerID { return s.P }

// Result satisfies engine.Settlement.
func (s Spun) Result() (wagered, won int64) { return s.Wagered, s.Total }

// Game satisfies engine.Settlement.
func (s Spun) Game() engine.Kind { return engine.KindSlots }

// Snapshot is everything a viewer can see. Deadline is always zero: the
// machine waits for a player, never for a clock.
type Snapshot struct {
	Stops    [Reels]int
	Window   [Reels][3]Symbol
	LastWins []LineWin
	LastWin  int64
	Deadline time.Time
}

// Table is one slot machine. It seats one player, which is all a slot machine
// has ever done; the seat exists so the interface is the same as the others.
type Table struct {
	rng    *rand.Rand
	seated engine.PlayerID

	stops    [Reels]int
	lastWins []LineWin
	lastWin  int64
}

// NewTable returns a machine parked on a random stop.
func NewTable(r *rand.Rand) *Table {
	t := &Table{rng: r}
	for i := range t.stops {
		t.stops[i] = r.IntN(StopsPerReel)
	}
	return t
}

// Kind satisfies engine.Game.
func (t *Table) Kind() engine.Kind { return engine.KindSlots }

// Join satisfies engine.Game.
func (t *Table) Join(p engine.PlayerID) (engine.Seat, error) {
	if t.seated != "" && t.seated != p {
		return 0, engine.ErrTableFull
	}
	t.seated = p
	return 0, nil
}

// Leave satisfies engine.Game.
func (t *Table) Leave(p engine.PlayerID) []engine.Event {
	if t.seated == p {
		t.seated = ""
	}
	return nil
}

// Deadline satisfies engine.Game: a slot machine has no clock.
func (t *Table) Deadline() (time.Time, bool) { return time.Time{}, false }

// Tick satisfies engine.Game and does nothing, for the same reason.
func (t *Table) Tick(time.Time) []engine.Event { return nil }

// Apply satisfies engine.Game.
func (t *Table) Apply(a engine.Action, _ time.Time) ([]engine.Event, error) {
	spin, ok := a.(Spin)
	if !ok {
		return nil, engine.ErrNotAllowed
	}
	if spin.Lines < 1 || spin.Lines > MaxLines {
		return nil, engine.ErrInvalidBet
	}
	if !validLineBet(spin.PerLine) {
		return nil, engine.ErrInvalidBet
	}

	for i := range t.stops {
		t.stops[i] = t.rng.IntN(StopsPerReel)
	}

	window := t.window()
	wins, total := score(window, spin.Lines, spin.PerLine)
	t.lastWins, t.lastWin = wins, total

	return []engine.Event{Spun{
		P:       spin.P,
		Stops:   t.stops,
		Window:  window,
		Wins:    wins,
		Wagered: spin.Stake(),
		Total:   total,
	}}, nil
}

// Snapshot satisfies engine.Game.
func (t *Table) Snapshot(engine.PlayerID) any {
	return Snapshot{
		Stops:    t.stops,
		Window:   t.window(),
		LastWins: t.lastWins,
		LastWin:  t.lastWin,
	}
}

// window is the three visible symbols on each reel, top row first.
func (t *Table) window() [Reels][3]Symbol {
	var w [Reels][3]Symbol
	for reel, stop := range t.stops {
		w[reel] = WindowAt(reel, stop)
	}
	return w
}

// WindowAt returns the symbols visible on a reel parked at a stop. The centre
// row is the stop itself, with its neighbours above and below.
func WindowAt(reel, stop int) [3]Symbol {
	at := func(i int) Symbol {
		i %= StopsPerReel
		if i < 0 {
			i += StopsPerReel
		}
		return Strips[reel][i]
	}
	return [3]Symbol{at(stop - 1), at(stop), at(stop + 1)}
}

// score reads the active paylines off a window.
func score(window [Reels][3]Symbol, lines int, perLine int64) ([]LineWin, int64) {
	var (
		wins  []LineWin
		total int64
	)
	for li := 0; li < lines; li++ {
		p := Paylines[li]
		a, b, c := window[0][p[0]], window[1][p[1]], window[2][p[2]]

		mult := Payout(a, b, c)
		if mult == 0 {
			continue
		}
		// A line either matched outright, or it is the two-cherry
		// consolation, which is credited to the cherry.
		sym, matched := lineSymbol(a, b, c)
		count := Reels
		if !matched {
			sym, count = Cherry, 2
		}

		pays := mult * perLine
		wins = append(wins, LineWin{Line: li, Symbol: sym, Count: count, Mult: mult, Pays: pays})
		total += pays
	}
	return wins, total
}

func validLineBet(n int64) bool {
	for _, b := range LineBets {
		if b == n {
			return true
		}
	}
	return false
}
