package app

import (
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gateway-of-last-resort/felt/internal/bank"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	"github.com/gateway-of-last-resort/felt/internal/games"
	"github.com/gateway-of-last-resort/felt/internal/rng"
	"github.com/gateway-of-last-resort/felt/internal/store"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

func testModel(t *testing.T, balance int64) Model {
	t.Helper()

	l, err := bank.OpenJSON(filepath.Join(t.TempDir(), "save.json"))
	if err != nil {
		t.Fatal(err)
	}
	if spend := bank.DefaultBankroll - balance; spend > 0 {
		if err := l.Debit(engine.LocalPlayer, spend); err != nil {
			t.Fatal(err)
		}
	}

	m := New(l, store.Default(), rng.NewSeeded([32]byte{1}))
	m, _ = m.update(tea.WindowSizeMsg{Width: 160, Height: 40})
	return m
}

func press(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: firstRune(s), Text: s}
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

// The layout must fit the terminal it was given, on every screen and every
// supported size. One over-wide line wraps and shears the whole table.
func TestViewFitsTerminal(t *testing.T) {
	sizes := []struct{ w, h int }{
		{160, 40}, // the target
		{100, 30}, // where the compact layout starts
		{80, 24},  // the smallest supported
		{70, 20},  // below minimum: the resize notice
	}
	screens := []Screen{
		ScreenMenu, ScreenSlots, ScreenBlackjack, ScreenRoulette, ScreenVideoPoker,
		ScreenStats, ScreenHelp, ScreenBankrupt,
	}

	for _, size := range sizes {
		for _, screen := range screens {
			m := testModel(t, 1000)
			m, _ = m.update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
			m.screen = screen
			if screen == ScreenStats {
				m.statsTable.SetRows(m.statsRows())
			}

			view := m.View().Content
			if view == "" {
				t.Errorf("%v at %dx%d rendered nothing", screen, size.w, size.h)
				continue
			}
			for i, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got > size.w {
					t.Errorf("%v at %dx%d: line %d is %d cells wide",
						screen, size.w, size.h, i+1, got)
					break
				}
			}
			if got := lipgloss.Height(view); got > size.h {
				t.Errorf("%v at %dx%d: view is %d rows tall", screen, size.w, size.h, got)
			}
		}
	}
}

// The alt screen is requested through the view, which is where Bubble Tea v2
// puts screen modes.
func TestViewAsksForAltScreen(t *testing.T) {
	m := testModel(t, 1000)
	if !m.View().AltScreen {
		t.Error("the view does not ask for the alt screen")
	}
}

// Nothing is drawn before the first size message: a guessed width would be
// repainted immediately and show as a flicker at launch.
func TestViewBeforeFirstResize(t *testing.T) {
	l, err := bank.OpenJSON(filepath.Join(t.TempDir(), "save.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(l, store.Default(), rng.NewSeeded([32]byte{2}))
	if got := m.View().Content; got != "" {
		t.Errorf("View() before the size message = %q, want empty", got)
	}
}

// The palette follows the terminal's background, which is also how a server
// session will learn about a remote terminal.
func TestThemeFollowsTerminalBackground(t *testing.T) {
	m := testModel(t, 1000)
	if !m.theme.Dark {
		t.Fatal("the default theme is not the dark one")
	}

	m, _ = m.retheme(false)
	if m.theme.Dark {
		t.Error("the theme stayed dark after a light background was reported")
	}

	// And the change reaches the screens, not just the chrome.
	m, _ = m.retheme(true)
	if !m.theme.Dark {
		t.Error("the theme did not switch back")
	}
}

// A broke player is sent to the bailout rather than to a table they cannot
// bet at.
func TestBrokePlayerCannotSitDown(t *testing.T) {
	m := testModel(t, 0)

	m, _ = m.switchTo(ScreenSlots)
	if m.screen != ScreenBankrupt {
		t.Errorf("screen = %v with no credits, want the bailout", m.screen)
	}
}

// The bailout restores the starting bankroll.
func TestBailoutRestoresBankroll(t *testing.T) {
	m := testModel(t, 0)
	m.screen = ScreenBankrupt

	handled, next, _ := m.globalKey(press("y"))
	if !handled {
		t.Fatal("the bailout screen ignored y")
	}
	if got := next.balance(); got != bank.DefaultBankroll {
		t.Errorf("balance = %d after rebuy, want %d", got, bank.DefaultBankroll)
	}
	if next.screen != ScreenMenu {
		t.Errorf("screen = %v after rebuy, want the menu", next.screen)
	}
}

// esc must not release a game mid-round, or a stake is stranded.
func TestEscapeBlockedWhileBusy(t *testing.T) {
	m := testModel(t, 1000)
	m.screen = ScreenSlots
	m.games[ScreenSlots] = busyGame{}

	_, next, _ := m.globalKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if next.screen != ScreenSlots {
		t.Errorf("screen = %v, want to stay at the busy table", next.screen)
	}
}

// A modal game keeps the keyboard, so the overlay can close itself instead of
// the root swallowing the key and dropping out to the menu.
func TestModalGameKeepsTheKeyboard(t *testing.T) {
	m := testModel(t, 1000)
	m.screen = ScreenSlots
	m.games[ScreenSlots] = modalGame{}

	handled, next, _ := m.globalKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if handled {
		t.Error("the root swallowed esc from a modal game")
	}
	if next.screen != ScreenSlots {
		t.Errorf("screen = %v, want to stay put", next.screen)
	}
}

// Statistics and help are overlays: leaving one returns to the table it was
// opened from, not to another overlay.
func TestOverlaysReturnToTheTable(t *testing.T) {
	m := testModel(t, 1000)

	m, _ = m.switchTo(ScreenSlots)
	m, _ = m.toggleScreen(ScreenStats)
	if m.screen != ScreenStats {
		t.Fatalf("screen = %v, want the statistics", m.screen)
	}

	m, _ = m.toggleScreen(ScreenStats)
	if m.screen != ScreenSlots {
		t.Errorf("screen = %v after closing the statistics, want the table", m.screen)
	}
}

// The Online row says what it knows rather than opening ssh and letting it
// fail in the player's face.
func TestOnlineRowReflectsTheCheck(t *testing.T) {
	m := testModel(t, 1000)

	m.server.Reachable, m.server.HasSSH = false, true
	if it := m.onlineItem(); it.Ready {
		t.Error("the Online row is ready with the server down")
	} else if !strings.Contains(it.Note, "unreachable") {
		t.Errorf("note = %q, want it to say the server is unreachable", it.Note)
	}

	m.server.Reachable, m.server.HasSSH = true, true
	if it := m.onlineItem(); !it.Ready {
		t.Error("the Online row is not ready with the server up")
	}

	// No ssh client: the row still opens, to show the command to run by hand.
	m.server.HasSSH = false
	if it := m.onlineItem(); !strings.Contains(it.Note, "no ssh") {
		t.Errorf("note = %q, want it to mention the missing ssh client", it.Note)
	}
}

// Going online without a server does not launch anything.
func TestGoOnlineRefusesWhenUnreachable(t *testing.T) {
	m := testModel(t, 1000)
	m.server.Reachable, m.server.HasSSH = false, true

	_, cmd := m.goOnline()
	if cmd == nil {
		t.Fatal("no feedback when the server is down")
	}
}

// The glyph toggle is a setting, so it goes up to the root and comes back
// down to every screen.
func TestGlyphToggleReachesTheGames(t *testing.T) {
	m := testModel(t, 1000)
	before := m.settings.Glyphs

	m, _ = m.toggleGlyphs()
	if m.settings.Glyphs == before {
		t.Fatalf("glyph set stayed %q", before)
	}

	m, _ = m.toggleGlyphs()
	if m.settings.Glyphs != before {
		t.Errorf("glyph set = %q, want it back at %q", m.settings.Glyphs, before)
	}
}

// Test doubles for the two game states the root treats specially.
type busyGame struct{}

func (busyGame) Init() tea.Cmd                          { return nil }
func (g busyGame) Update(tea.Msg) (games.Game, tea.Cmd) { return g, nil }
func (busyGame) View() string                           { return "busy" }
func (busyGame) Title() string                          { return "Busy" }
func (busyGame) Help() []key.Binding                    { return nil }
func (busyGame) Busy() bool                             { return true }
func (busyGame) Modal() bool                            { return false }
func (busyGame) Stake() int64                           { return 0 }
func (g busyGame) Reset() games.Game                    { return g }
func (g busyGame) SetTheme(ui.Theme) games.Game         { return g }
func (g busyGame) SetSize(int, int) games.Game          { return g }

type modalGame struct{ busyGame }

func (modalGame) Modal() bool                    { return true }
func (g modalGame) Reset() games.Game            { return g }
func (g modalGame) SetTheme(ui.Theme) games.Game { return g }
func (g modalGame) SetSize(int, int) games.Game  { return g }

// The statistics table has to survive the terminal reporting its background.
//
// That message arrives after the first resize, so anything it rebuilds loses
// the size the resize gave it — and a table of width zero renders its header
// and no rows at all, which is what an apparently empty statistics screen
// turned out to be.
func TestStatsSurviveTheThemeMessage(t *testing.T) {
	m := testModel(t, 1000)
	m.ledger.Record(engine.LocalPlayer, engine.KindSlots, 100, 40)

	m, _ = m.update(tea.WindowSizeMsg{Width: 140, Height: 40})
	width, height := m.statsTable.Width(), m.statsTable.Height()
	if width == 0 || height == 0 {
		t.Fatalf("table is %dx%d after a resize", width, height)
	}

	// The order the runtime actually sends them in.
	m, _ = m.update(tea.BackgroundColorMsg{Color: lipgloss.Color("#000000")})

	if got := m.statsTable.Width(); got != width {
		t.Errorf("table width = %d after the theme arrived, want %d", got, width)
	}
	if got := m.statsTable.Height(); got != height {
		t.Errorf("table height = %d after the theme arrived, want %d", got, height)
	}

	m, _ = m.switchTo(ScreenStats)
	view := m.View().Content
	for _, want := range []string{"Slots", "Blackjack", "Roulette", "Video Poker", "All"} {
		if !strings.Contains(view, want) {
			t.Errorf("the statistics screen does not list %q", want)
		}
	}
}

// And the figures on it are the ones the wallet holds.
func TestStatsShowRealFigures(t *testing.T) {
	m := testModel(t, 1000)
	m.ledger.Record(engine.LocalPlayer, engine.KindRoulette, 250, 100)
	m, _ = m.update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m, _ = m.switchTo(ScreenStats)

	view := m.View().Content
	for _, want := range []string{"250", "100", "-150", "40.0%"} {
		if !strings.Contains(view, want) {
			t.Errorf("the statistics screen does not show %q", want)
		}
	}
}

// The rows have to line up under their own headings.
//
// A table given more width than its columns pads the rows to fill it, while
// the header keeps the width of the columns; centring the block then puts the
// two at different offsets and the rows sit visibly left of their headings.
func TestStatsRowsLineUpWithHeadings(t *testing.T) {
	m := testModel(t, 1000)
	m.ledger.Record(engine.LocalPlayer, engine.KindSlots, 100, 40)
	m, _ = m.update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m, _ = m.switchTo(ScreenStats)

	var header, row string
	for _, line := range strings.Split(m.View().Content, "\n") {
		plain := stripANSI(line)
		switch {
		case header == "" && strings.Contains(plain, "Game") && strings.Contains(plain, "Rounds"):
			header = plain
		case row == "" && strings.Contains(plain, "Slots"):
			row = plain
		}
	}
	if header == "" || row == "" {
		t.Fatal("could not find the header and a row on the statistics screen")
	}

	if h, r := indent(header), indent(row); h != r {
		t.Errorf("header starts at column %d and its rows at %d", h, r)
	}
}

func indent(s string) int {
	for i, r := range s {
		if r != ' ' {
			return i
		}
	}
	return len(s)
}

// stripANSI drops the escape sequences so a rendered line can be measured as
// the terminal would show it.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case !inEscape:
			b.WriteRune(r)
		}
	}
	return b.String()
}
