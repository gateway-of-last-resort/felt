package roulette

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gateway-of-last-resort/felt/internal/bank"
	"github.com/gateway-of-last-resort/felt/internal/driver"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	rl "github.com/gateway-of-last-resort/felt/internal/engine/roulette"
	"github.com/gateway-of-last-resort/felt/internal/games"
	"github.com/gateway-of-last-resort/felt/internal/rng"
	"github.com/gateway-of-last-resort/felt/internal/ui"
	"path/filepath"
)

func testModel(t *testing.T, w, h int) Model {
	t.Helper()

	ledger, err := bank.OpenJSON(filepath.Join(t.TempDir(), "save.json"))
	if err != nil {
		t.Fatal(err)
	}
	drv := driver.NewLocal(rl.NewTable(rng.NewSeeded([32]byte{9})), ledger, engine.LocalPlayer, nil)

	m := New(drv, games.LocalOptions(), ui.Default())
	g := m.SetSize(w, h)
	return g.(Model)
}

// The table has to fit its area at every supported size. The layout is drawn
// into a character grid, so an off-by-one in the coordinate maths shows up
// here rather than as a sheared board in front of a player.
func TestLayoutFits(t *testing.T) {
	// The body area a root leaves a game: the terminal less its chrome.
	sizes := []struct{ w, h int }{
		{160, 33},
		{110, 27},
		{100, 23},
		{80, 17},
	}

	for _, size := range sizes {
		m := testModel(t, size.w, size.h)
		view := m.View()

		for i, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > size.w {
				t.Errorf("at %dx%d line %d is %d cells wide", size.w, size.h, i+1, got)
				break
			}
		}
		if got := lipgloss.Height(view); got > size.h {
			t.Errorf("at %dx%d the table is %d rows tall", size.w, size.h, got)
		}
	}
}

// Every spot on the layout must be drawable with the cursor on it. With
// around 150 spots, most of them on the lines between numbers, this is where
// a coordinate mistake would otherwise go unnoticed.
func TestCursorOnEverySpotRenders(t *testing.T) {
	for _, size := range []struct{ w, h int }{{160, 33}, {80, 17}} {
		m := testModel(t, size.w, size.h)

		for _, s := range rl.Spots {
			m.cursor = s.ID
			view := m.View()
			if view == "" {
				t.Fatalf("cursor on %s (%s) rendered nothing", s.Label, s.Type)
			}
			for _, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got > size.w {
					t.Fatalf("cursor on %s (%s) at %dx%d overflows: %d cells",
						s.Label, s.Type, size.w, size.h, got)
				}
			}
		}
	}
}

// Chips on the layout must render wherever they are placed, including on the
// lines between numbers.
func TestChipsRenderOnEverySpot(t *testing.T) {
	m := testModel(t, 160, 33)

	for _, s := range rl.Spots {
		m.snap = rl.Snapshot{
			Bets:    []rl.BetView{{Spot: s.ID, Amount: 25, Mine: true}},
			MyTotal: 25,
			Number:  -1,
		}
		if view := m.View(); view == "" {
			t.Fatalf("a chip on %s rendered nothing", s.Label)
		}
	}
}

// Moving the cursor with the arrow keys never leaves the layout.
func TestCursorStaysOnTheTable(t *testing.T) {
	m := testModel(t, 160, 33)

	keys := []tea.KeyPressMsg{
		{Code: tea.KeyUp}, {Code: tea.KeyDown}, {Code: tea.KeyLeft}, {Code: tea.KeyRight},
	}
	for i := 0; i < 400; i++ {
		g, _ := m.Update(keys[i%len(keys)])
		m = g.(Model)
		if m.cursor < 0 || m.cursor >= len(rl.Spots) {
			t.Fatalf("cursor left the table: %d", m.cursor)
		}
	}
}

// Placing a chip goes through the driver, and the driver takes the money.
func TestPlacingAChipCostsCredits(t *testing.T) {
	m := testModel(t, 160, 33)

	g, cmd := m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	m = g.(Model)
	if cmd == nil {
		t.Fatal("placing a chip issued no command")
	}

	msg := cmd()
	events, ok := msg.(driver.EventsMsg)
	if !ok {
		t.Fatalf("got %T, want EventsMsg", msg)
	}
	if want := bank.DefaultBankroll - m.Chip(); events.Balance != want {
		t.Errorf("balance = %d after a chip of %d, want %d", events.Balance, m.Chip(), want)
	}
}

// The wheel does not turn on an empty layout.
func TestSpinWithNoChipsIsRefused(t *testing.T) {
	m := testModel(t, 160, 33)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("spinning an empty table issued no command")
	}
	if _, ok := cmd().(driver.ErrorMsg); !ok {
		t.Errorf("got %T, want the spin to be refused", cmd())
	}
}

// Space drops the ball into its pocket rather than waiting out the flight.
// The number was decided before the ball moved, so nothing changes but the
// suspense.
func TestSpaceSkipsTheBall(t *testing.T) {
	m := testModel(t, 160, 33)

	// Put a chip down and spin.
	g, cmd := m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	m = g.(Model)
	g, _ = m.Update(cmd())
	m = g.(Model)

	g, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = g.(Model)
	g, _ = m.Update(cmd())
	m = g.(Model)

	if !m.spinning {
		t.Fatal("the ball is not in the air")
	}
	want := m.result

	g, _ = m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	m = g.(Model)

	if m.spinning {
		t.Error("the ball is still travelling after the skip")
	}
	if m.result != want {
		t.Errorf("result changed from %d to %d when skipped", want, m.result)
	}
	if got := m.ball.Number(); got != want {
		t.Errorf("the ball landed on %d, want the drawn number %d", got, want)
	}
	if !m.showWin {
		t.Error("the result is not being shown after the skip")
	}
}

// Chips cannot be placed while the wheel is turning: space is the skip then,
// not a bet.
func TestNoBetsWhileSpinning(t *testing.T) {
	m := testModel(t, 160, 33)
	m.spinning = true
	m.result = 17

	before := m.snap.MyTotal
	g, _ := m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	m = g.(Model)

	if m.snap.MyTotal != before {
		t.Error("a chip was placed while the wheel was turning")
	}
}
