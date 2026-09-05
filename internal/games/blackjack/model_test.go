package blackjack

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gateway-of-last-resort/felt/internal/bank"
	"github.com/gateway-of-last-resort/felt/internal/driver"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	bj "github.com/gateway-of-last-resort/felt/internal/engine/blackjack"
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
	table := bj.NewTable(rng.NewSeeded([32]byte{11}), bj.Vegas6(), 1)
	drv := driver.NewLocal(table, ledger, engine.LocalPlayer, nil)
	if err := drv.Join(0); err != nil {
		t.Fatalf("taking a seat: %v", err)
	}

	m := New(drv, games.LocalOptions(), ui.Default())
	return m.SetSize(w, h).(Model), drv
}

// deal starts a round through the model the way the runtime would: the driver
// answers with every card at once, and the model reveals them on a timer.
func deal(t *testing.T, m Model, drv *driver.Local) Model {
	t.Helper()

	cmd := drv.Do(bj.PlaceBet{P: engine.LocalPlayer, Amount: 10})
	g, _ := m.Update(cmd())
	return g.(Model)
}

// Cards arrive together and are shown one at a time. Until the queue drains,
// the screen must show fewer cards than the engine has dealt.
func TestDealIsAnimated(t *testing.T) {
	m, drv := testModel(t, 120, 30)
	m = deal(t, m, drv)

	if len(m.pending) == 0 {
		t.Fatal("the whole round was applied at once; nothing was left to animate")
	}
	if !m.Busy() {
		t.Error("Busy() = false mid-deal; esc would abandon the stake")
	}

	// Walking the queue reveals the rest.
	for i := 0; i < 50 && len(m.pending) > 0; i++ {
		g, _ := m.Update(revealMsg{})
		m = g.(Model)
	}
	if len(m.pending) != 0 {
		t.Fatal("the deal never finished")
	}
	if got := len(m.myHands()); got != 1 {
		t.Fatalf("holding %d hands after the deal, want 1", got)
	}
	if got := len(m.myHands()[0].Cards); got != 2 {
		t.Errorf("player shows %d cards, want 2", got)
	}
}

// The hole card must stay face down on screen for the whole player's turn —
// including mid-deal, when the model is drawing from its own queue rather
// than from the snapshot.
func TestHoleCardStaysDownDuringPlay(t *testing.T) {
	m, drv := testModel(t, 120, 30)
	m = deal(t, m, drv)

	for i := 0; i < 50 && len(m.pending) > 0; i++ {
		if !m.holeDown() && len(m.shown.dealer) > 1 && m.snap.Phase == bj.PhasePlayerTurn {
			t.Fatal("the hole card was face up mid-deal")
		}
		g, _ := m.Update(revealMsg{})
		m = g.(Model)
	}

	if m.snap.Phase == bj.PhasePlayerTurn && !m.holeDown() {
		t.Error("the hole card is face up during the player's turn")
	}
}

// The table refuses input while cards are still being turned over: the player
// would be acting on a hand they cannot see yet.
func TestNoInputMidDeal(t *testing.T) {
	m, drv := testModel(t, 120, 30)
	m = deal(t, m, drv)

	if len(m.pending) == 0 {
		t.Skip("nothing queued to animate")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if cmd != nil {
		t.Error("a key was acted on while the round was still being dealt")
	}
}

// The layout has to fit its area, four split hands included, which is the
// widest the table ever gets.
func TestTableFits(t *testing.T) {
	sizes := []struct{ w, h int }{{160, 33}, {110, 27}, {80, 17}}

	for _, size := range sizes {
		m, drv := testModel(t, size.w, size.h)
		m = deal(t, m, drv)
		for i := 0; i < 60 && len(m.pending) > 0; i++ {
			g, _ := m.Update(revealMsg{})
			m = g.(Model)
		}

		for _, screen := range []string{m.View()} {
			for i, line := range strings.Split(screen, "\n") {
				if got := lipgloss.Width(line); got > size.w {
					t.Errorf("at %dx%d line %d is %d cells wide", size.w, size.h, i+1, got)
					break
				}
			}
			if got := lipgloss.Height(screen); got > size.h {
				t.Errorf("at %dx%d the table is %d rows tall", size.w, size.h, got)
			}
		}
	}
}

// Typed bets are held to the table's limits and to an even number, so 3:2 and
// insurance always settle in whole credits.
func TestTypedBetIsClamped(t *testing.T) {
	m, _ := testModel(t, 120, 30)
	r := m.snap.Rules

	for _, c := range []struct{ in, want int64 }{
		{0, r.MinBet}, {1, r.MinBet}, {7, 6}, {10, 10}, {99999, r.MaxBet},
	} {
		if got := r.ClampBet(c.in); got != c.want {
			t.Errorf("ClampBet(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

// The bet field owns the keyboard while it is open, so esc can cancel it
// instead of dropping the player out to the menu.
func TestBetFieldIsModal(t *testing.T) {
	m, _ := testModel(t, 120, 30)
	if m.Modal() {
		t.Error("Modal() = true with no field open")
	}

	g, _ := m.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	m = g.(Model)
	if !m.Modal() {
		t.Fatal("Modal() = false with the bet field open")
	}

	g, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if g.(Model).Modal() {
		t.Error("esc did not close the bet field")
	}
}

// Enter has to start the next hand once a round is settled. This is what the
// stuck-seat bug looked like from the outside: the table dealt one round and
// then answered every enter with "not now".
func TestEnterDealsTheNextHand(t *testing.T) {
	m, drv := testModel(t, 120, 30)

	for round := 1; round <= 5; round++ {
		m = deal(t, m, drv)
		for i := 0; i < 60 && len(m.pending) > 0; i++ {
			g, _ := m.Update(revealMsg{})
			m = g.(Model)
		}

		// Play whatever landed out to a settled round.
		for i := 0; i < 20 && m.snap.Phase != bj.PhaseSettle; i++ {
			var cmd tea.Cmd
			switch m.snap.Phase {
			case bj.PhaseInsurance:
				g, c := m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
				m, cmd = g.(Model), c
			case bj.PhasePlayerTurn:
				g, c := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
				m, cmd = g.(Model), c
			default:
				i = 20
			}
			if cmd != nil {
				g, _ := m.Update(cmd())
				m = g.(Model)
				for j := 0; j < 60 && len(m.pending) > 0; j++ {
					g, _ = m.Update(revealMsg{})
					m = g.(Model)
				}
			}
		}

		if m.snap.Phase != bj.PhaseSettle {
			t.Fatalf("round %d did not settle, phase = %v", round, m.snap.Phase)
		}

		// Now the key the player actually presses.
		g, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m = g.(Model)
		if cmd == nil {
			t.Fatalf("round %d: enter did nothing", round)
		}

		msg := cmd()
		if err, ok := msg.(driver.ErrorMsg); ok {
			t.Fatalf("round %d: enter was refused with %q", round+1, err.Err)
		}
		g, _ = m.Update(msg)
		m = g.(Model)
	}
}

// While a round is being dealt the screen shows only what it has turned over.
//
// The snapshot already holds the finished round, so reading from it mid-deal
// showed the *previous* hand: the dealer's cards from the round before,
// sitting on the table before they had been dealt a single one.
func TestPreviousHandDoesNotLinger(t *testing.T) {
	m, drv := testModel(t, 120, 30)

	// Play a round out, so there is a hand that could leak into the next.
	m = playOut(t, m, drv)
	if len(m.dealerCards()) == 0 {
		t.Fatal("the first round dealt the dealer nothing")
	}

	// Start the next one. The instant its events arrive the table must be
	// bare, not still showing the last hand.
	g, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = g.(Model)
	if cmd == nil {
		t.Fatal("enter did not deal")
	}
	msg := cmd()
	if err, ok := msg.(driver.ErrorMsg); ok {
		t.Fatalf("the next deal was refused: %v", err.Err)
	}
	g, _ = m.Update(msg)
	m = g.(Model)

	if got := len(m.dealerCards()); got != 0 {
		t.Errorf("the dealer shows %d cards before being dealt any", got)
	}
	if got := len(m.myHands()); got != 0 {
		t.Errorf("the player shows %d hands before being dealt any", got)
	}

	// And they arrive one at a time.
	g, _ = m.Update(revealMsg{})
	m = g.(Model)
	if got := len(m.myHands()); got != 1 {
		t.Fatalf("after one reveal the player has %d hands, want 1", got)
	}
	if got := len(m.myHands()[0].Cards); got != 1 {
		t.Errorf("after one reveal the player shows %d cards, want 1", got)
	}
}

// playOut deals a round and plays it to settlement, standing on everything.
func playOut(t *testing.T, m Model, drv *driver.Local) Model {
	t.Helper()

	m = deal(t, m, drv)
	for i := 0; i < 60 && m.playing; i++ {
		g, _ := m.Update(revealMsg{})
		m = g.(Model)
	}

	for i := 0; i < 30 && m.snap.Phase != bj.PhaseSettle; i++ {
		key := "s"
		if m.snap.Phase == bj.PhaseInsurance {
			key = "n"
		}
		g, cmd := m.Update(tea.KeyPressMsg{Code: rune(key[0]), Text: key})
		m = g.(Model)
		if cmd == nil {
			break
		}
		g, _ = m.Update(cmd())
		m = g.(Model)
		for j := 0; j < 60 && m.playing; j++ {
			g, _ = m.Update(revealMsg{})
			m = g.(Model)
		}
	}

	if m.snap.Phase != bj.PhaseSettle {
		t.Fatalf("the round did not settle, phase = %v", m.snap.Phase)
	}
	return m
}
