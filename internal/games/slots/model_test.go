package slots

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gateway-of-last-resort/felt/internal/bank"
	"github.com/gateway-of-last-resort/felt/internal/driver"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	slotsengine "github.com/gateway-of-last-resort/felt/internal/engine/slots"
	"github.com/gateway-of-last-resort/felt/internal/games"
	"github.com/gateway-of-last-resort/felt/internal/rng"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

func testModel(t *testing.T) (Model, *driver.Local) {
	t.Helper()

	ledger, err := bank.OpenJSON(filepath.Join(t.TempDir(), "save.json"))
	if err != nil {
		t.Fatal(err)
	}
	drv := driver.NewLocal(slotsengine.NewTable(rng.NewSeeded([32]byte{4})),
		ledger, engine.LocalPlayer, nil)

	m := New(drv, games.LocalOptions(), ui.Default(), games.GlyphsASCII)
	return m.SetSize(120, 30).(Model), drv
}

// playRound spins and runs the animation to a standstill.
func playRound(t *testing.T, m Model, drv *driver.Local) Model {
	t.Helper()

	g, _ := m.Update(drv.Do(slotsengine.Spin{
		P: engine.LocalPlayer, Lines: m.lines, PerLine: m.LineBet(),
	})())
	m = g.(Model)

	now := m.lastTick
	for i := 0; i < 20*m.opts.FPS && m.state == stateSpinning; i++ {
		now = now.Add(m.opts.Frame())
		g, _ = m.Update(tickMsg(now))
		m = g.(Model)
	}
	for i := 0; i <= slotsengine.MaxLines+1 && m.state == stateResolving; i++ {
		g, _ = m.Update(highlightMsg(m.winIdx + 1))
		m = g.(Model)
	}
	return m
}

// The round total stays on screen until the next spin. A result that clears
// itself is one the player has to be watching to catch.
func TestResultStaysUntilTheNextSpin(t *testing.T) {
	m, drv := testModel(t)
	m = playRound(t, m, drv)

	if m.state != stateResult {
		t.Fatalf("state = %v after the animation, want the result", m.state)
	}

	// No amount of time passing takes it away.
	for i := 0; i < 10; i++ {
		g, _ := m.Update(tickMsg(time.Now().Add(time.Duration(i) * time.Second)))
		m = g.(Model)
	}
	if m.state != stateResult {
		t.Errorf("state = %v after ten seconds, want the result still up", m.state)
	}
	if m.View() == "" {
		t.Error("the result screen renders nothing")
	}

	// Spinning again clears it.
	m2, _ := m.spin()
	if m2.state == stateResult {
		t.Error("the previous result survived into the next spin")
	}
	if len(m2.wins) != 0 || m2.totalWin != 0 {
		t.Error("the previous round's wins survived into the next spin")
	}
}

// Leaving the machine clears the result too, so returning does not show a
// round from ten minutes ago.
func TestLeavingClearsTheResult(t *testing.T) {
	m, drv := testModel(t)
	m = playRound(t, m, drv)

	next := m.Reset().(Model)
	if next.state != stateIdle {
		t.Errorf("state = %v after leaving, want idle", next.state)
	}
}

// A result on screen is not "busy": the player can walk away from it.
func TestResultIsNotBusy(t *testing.T) {
	m, drv := testModel(t)
	m = playRound(t, m, drv)

	if m.Busy() {
		t.Error("Busy() = true with the round over; esc would be refused")
	}
}

// Winning lines are walked one at a time, slowly enough to read. The pace is
// asserted because it is the whole reason the walk exists.
func TestLinesAreWalkedOneAtATime(t *testing.T) {
	if highlightTime < 700*time.Millisecond {
		t.Errorf("highlight time is %v, too fast to read a line", highlightTime)
	}

	m, drv := testModel(t)

	// Spin until a round pays on more than one line, so there is a walk.
	var found Model
	for i := 0; i < 200; i++ {
		next := playRound(t, m, drv)
		if len(next.wins) > 1 {
			found = next
			break
		}
		m = next
	}
	if len(found.wins) < 2 {
		t.Skip("no multi-line win in 200 spins")
	}

	// Replaying the walk should stop on each winning line in turn.
	m = found
	m.state = stateResolving
	m.winIdx = 0
	for i := 1; i < len(m.wins); i++ {
		g, cmd := m.Update(highlightMsg(i))
		m = g.(Model)
		if m.winIdx != i {
			t.Fatalf("highlight %d landed on line index %d", i, m.winIdx)
		}
		if cmd == nil {
			t.Fatal("the walk stopped before the last line")
		}
	}
}

// A rejected spin leaves the reels still rather than spinning for free.
func TestRejectedSpinDoesNotAnimate(t *testing.T) {
	m, _ := testModel(t)

	g, _ := m.Update(driver.ErrorMsg{Err: bank.ErrInsufficientFunds})
	m = g.(Model)

	if m.state != stateIdle {
		t.Errorf("state = %v after a refused spin, want idle", m.state)
	}
	if m.Busy() {
		t.Error("Busy() = true after a refused spin")
	}
}

// The paytable takes the keyboard so it can close itself on esc.
func TestPaytableIsModal(t *testing.T) {
	m, _ := testModel(t)

	g, _ := m.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	m = g.(Model)
	if !m.Modal() {
		t.Fatal("the paytable did not take the keyboard")
	}

	g, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if g.(Model).Modal() {
		t.Error("esc did not close the paytable")
	}
}

// Space skips an animation in progress and lands on the same result the
// reels were travelling towards.
func TestSpaceSkipsTheAnimation(t *testing.T) {
	m, drv := testModel(t)

	g, _ := m.Update(drv.Do(slotsengine.Spin{
		P: engine.LocalPlayer, Lines: m.lines, PerLine: m.LineBet(),
	})())
	m = g.(Model)
	if m.state != stateSpinning {
		t.Fatalf("state = %v, want a spin in progress", m.state)
	}
	wantStops := m.snap.Stops

	g, _ = m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	m = g.(Model)

	if m.state != stateResult {
		t.Fatalf("state = %v after skipping, want the result", m.state)
	}
	for i, r := range m.reels {
		if got := r.Index(); got != wantStops[i] {
			t.Errorf("reel %d skipped to stop %d, want %d", i, got, wantStops[i])
		}
		if r.Spinning() {
			t.Errorf("reel %d is still turning after the skip", i)
		}
	}
	if m.Busy() {
		t.Error("Busy() = true after skipping to the result")
	}
}

// Skipping mid-walk goes straight to the round total rather than to the next
// line.
func TestSpaceSkipsTheLineWalk(t *testing.T) {
	m, drv := testModel(t)

	// Find a round that pays, so there is a walk to skip.
	for i := 0; i < 200; i++ {
		m = playRound(t, m, drv)
		if len(m.wins) > 0 {
			break
		}
	}
	if len(m.wins) == 0 {
		t.Skip("no win in 200 spins")
	}

	m.state = stateResolving
	m.winIdx = 0

	g, _ := m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	m = g.(Model)

	if m.state != stateResult {
		t.Errorf("state = %v after skipping the walk, want the result", m.state)
	}
	if m.winIdx != -1 {
		t.Errorf("still highlighting line index %d after the skip", m.winIdx)
	}
}

// Once the result is up, space starts the next round instead of skipping.
func TestSpaceSpinsAgainAfterTheResult(t *testing.T) {
	m, drv := testModel(t)
	m = playRound(t, m, drv)

	if m.state != stateResult {
		t.Fatalf("state = %v, want the result", m.state)
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	if cmd == nil {
		t.Fatal("space did not start another spin")
	}
	if _, ok := cmd().(driver.EventsMsg); !ok {
		t.Errorf("space produced %T, want another round", cmd())
	}
}

// The line description must match what is on the reels. Two cherries and a
// bell is a win, but calling it "Cherry×3" makes the machine look broken.
func TestWinLineDescribesWhatPaid(t *testing.T) {
	m, _ := testModel(t)

	cases := []struct {
		win  slotsengine.LineWin
		want string
	}{
		{slotsengine.LineWin{Line: 0, Symbol: slotsengine.Cherry, Count: 2, Pays: 40}, "Cherry×2"},
		{slotsengine.LineWin{Line: 2, Symbol: slotsengine.Bell, Count: 3, Pays: 80}, "Bell×3"},
	}
	for _, c := range cases {
		got := m.winLine(c.win)
		if !strings.Contains(got, c.want) {
			t.Errorf("winLine = %q, want it to mention %q", got, c.want)
		}
	}
}

// A single-line win spells out what paid, so the player is not left working
// backwards from a number.
func TestResultExplainsASingleWin(t *testing.T) {
	m, _ := testModel(t)
	m.state = stateResult
	m.totalWin = 40
	m.wins = []slotsengine.LineWin{
		{Line: 0, Symbol: slotsengine.Cherry, Count: 2, Pays: 40},
	}

	got := m.resultView()
	if !strings.Contains(got, "Cherry×2") {
		t.Errorf("result = %q, want it to explain the cherry win", got)
	}
	if !strings.Contains(got, "40") {
		t.Errorf("result = %q, want the amount", got)
	}
}
