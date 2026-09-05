package videopoker

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gateway-of-last-resort/felt/internal/bank"
	"github.com/gateway-of-last-resort/felt/internal/driver"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	"github.com/gateway-of-last-resort/felt/internal/engine/poker/video"
	"github.com/gateway-of-last-resort/felt/internal/games"
	"github.com/gateway-of-last-resort/felt/internal/rng"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

func testModel(t *testing.T, w, h int) (Model, *driver.Local) {
	t.Helper()

	ledger, err := bank.OpenJSON(filepath.Join(t.TempDir(), "save.json"))
	if err != nil {
		t.Fatal(err)
	}
	drv := driver.NewLocal(video.NewTable(rng.NewSeeded([32]byte{19})),
		ledger, engine.LocalPlayer, nil)
	if err := drv.Join(0); err != nil {
		t.Fatalf("taking a seat: %v", err)
	}

	m := New(drv, games.LocalOptions(), ui.Default())
	return m.SetSize(w, h).(Model), drv
}

func press(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}

// settle runs the model until nothing is animating.
func settle(m Model) Model {
	for i := 0; i < 20 && (m.dealing() || m.drawing()); i++ {
		g, _ := m.Update(revealMsg{})
		m = g.(Model)
	}
	return m
}

// deal presses enter and plays the cards out.
func deal(t *testing.T, m Model) Model {
	t.Helper()

	g, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = g.(Model)
	if cmd == nil {
		t.Fatal("enter did not deal")
	}
	msg := cmd()
	if err, ok := msg.(driver.ErrorMsg); ok {
		t.Fatalf("the deal was refused: %v", err.Err)
	}
	g, _ = m.Update(msg)
	return settle(g.(Model))
}

// Enter deals from a standing start.
//
// Before the first hand nothing has been turned over, and treating that as an
// animation in progress locked the machine solid: no key did anything at all.
func TestEnterDealsFromIdle(t *testing.T) {
	m, _ := testModel(t, 120, 34)

	if m.dealing() {
		t.Error("the machine thinks it is dealing before the first hand")
	}
	if m.Busy() {
		t.Error("Busy() = true with no hand out")
	}

	m = deal(t, m)
	if m.shown != 5 {
		t.Fatalf("%d cards are showing after a deal, want 5", m.shown)
	}
	if m.snap.Phase != video.PhaseDraw {
		t.Errorf("phase = %v, want the draw", m.snap.Phase)
	}
}

// Cards arrive one at a time rather than all at once.
func TestDealIsAnimated(t *testing.T) {
	m, _ := testModel(t, 120, 34)

	g, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = g.(Model)
	g, _ = m.Update(cmd())
	m = g.(Model)

	if m.shown != 0 {
		t.Fatalf("%d cards showing immediately, want the deal to be animated", m.shown)
	}
	for i := 1; i <= 5; i++ {
		g, _ = m.Update(revealMsg{})
		m = g.(Model)
		if m.shown != i {
			t.Fatalf("after %d reveals %d cards are showing", i, m.shown)
		}
	}
}

// Held cards stay put; the rest come back replaced.
func TestHoldAndDraw(t *testing.T) {
	m, _ := testModel(t, 120, 34)
	m = deal(t, m)
	before := m.cards

	// Hold the first and third by number.
	for _, k := range []string{"1", "3"} {
		g, _ := m.Update(press(k))
		m = g.(Model)
	}
	if !m.hold[0] || !m.hold[2] || m.hold[1] || m.hold[3] || m.hold[4] {
		t.Fatalf("holds are %v, want the first and third", m.hold)
	}

	g, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = g.(Model)
	g, _ = m.Update(cmd())
	m = settle(g.(Model))

	if m.cards[0] != before[0] || m.cards[2] != before[2] {
		t.Error("a held card was replaced")
	}
	if m.snap.Phase != video.PhaseResult {
		t.Errorf("phase = %v after the draw, want the result", m.snap.Phase)
	}
}

// A number key toggles: pressing it twice puts the card back in the discards.
func TestHoldToggles(t *testing.T) {
	m, _ := testModel(t, 120, 34)
	m = deal(t, m)

	g, _ := m.Update(press("2"))
	m = g.(Model)
	if !m.hold[1] {
		t.Fatal("2 did not hold the second card")
	}
	g, _ = m.Update(press("2"))
	if g.(Model).hold[1] {
		t.Error("2 did not release the second card")
	}
}

// Space holds whatever the cursor is on, and the arrows move it.
func TestCursorAndSpace(t *testing.T) {
	m, _ := testModel(t, 120, 34)
	m = deal(t, m)

	for i := 0; i < 2; i++ {
		g, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
		m = g.(Model)
	}
	if m.cursor != 2 {
		t.Fatalf("cursor is on card %d, want the third", m.cursor+1)
	}

	g, _ := m.Update(press(" "))
	m = g.(Model)
	if !m.hold[2] {
		t.Error("space did not hold the card under the cursor")
	}

	// And it cannot walk off either end.
	for i := 0; i < 10; i++ {
		g, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
		m = g.(Model)
	}
	if m.cursor != 0 {
		t.Errorf("cursor is at %d after walking left, want the first card", m.cursor)
	}
}

// Coins are chosen before the deal, and the fifth one is worth choosing.
func TestCoinSelection(t *testing.T) {
	m, _ := testModel(t, 120, 34)

	g, _ := m.Update(press("-"))
	m = g.(Model)
	if m.coins != video.MaxCoins-1 {
		t.Errorf("coins = %d after one press, want %d", m.coins, video.MaxCoins-1)
	}

	g, _ = m.Update(press("3"))
	m = g.(Model)
	if m.coins != 3 {
		t.Errorf("coins = %d, want 3", m.coins)
	}

	// m bets the maximum and deals in one go.
	g, cmd := m.Update(press("m"))
	m = g.(Model)
	if m.coins != video.MaxCoins {
		t.Errorf("coins = %d after max bet, want %d", m.coins, video.MaxCoins)
	}
	if cmd == nil {
		t.Error("max bet did not deal")
	}
}

// The pay table is always on screen: it is what the player reads to decide
// what to hold, not reference material behind a key.
func TestPaytableIsAlwaysVisible(t *testing.T) {
	m, _ := testModel(t, 120, 34)

	for _, stage := range []string{"idle", "dealt"} {
		view := m.View()
		for _, want := range []string{"Royal flush", "Jacks or better", "4000"} {
			if !strings.Contains(view, want) {
				t.Errorf("%s: the pay table does not show %q", stage, want)
			}
		}
		m = deal(t, m)
	}
}

// The layout fits every supported size.
func TestLayoutFits(t *testing.T) {
	for _, size := range []struct{ w, h int }{{160, 33}, {110, 27}, {80, 17}} {
		m, _ := testModel(t, size.w, size.h)
		m = deal(t, m)

		view := m.View()
		for i, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > size.w {
				t.Errorf("at %dx%d line %d is %d cells wide", size.w, size.h, i+1, got)
				break
			}
		}
		if got := lipgloss.Height(view); got > size.h {
			t.Errorf("at %dx%d the machine is %d rows tall", size.w, size.h, got)
		}
	}
}

// A hand with coins on it cannot be walked away from.
func TestBusyWithAHandOut(t *testing.T) {
	m, _ := testModel(t, 120, 34)
	if m.Busy() {
		t.Error("Busy() = true before anything was staked")
	}

	m = deal(t, m)
	if !m.Busy() {
		t.Error("Busy() = false with a hand waiting to be drawn to")
	}

	g, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = g.(Model)
	g, _ = m.Update(cmd())
	m = settle(g.(Model))

	if m.Busy() {
		t.Error("Busy() = true once the hand is settled")
	}
}

// Hand after hand, which is the path a stuck phase would break.
func TestManyHands(t *testing.T) {
	m, _ := testModel(t, 120, 34)

	for i := 1; i <= 10; i++ {
		m = deal(t, m)

		g, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m = g.(Model)
		if cmd == nil {
			t.Fatalf("hand %d: the draw did nothing", i)
		}
		msg := cmd()
		if err, ok := msg.(driver.ErrorMsg); ok {
			t.Fatalf("hand %d: the draw was refused: %v", i, err.Err)
		}
		g, _ = m.Update(msg)
		m = settle(g.(Model))
	}
}
