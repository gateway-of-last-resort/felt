package app

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	slotsengine "github.com/gateway-of-last-resort/felt/internal/engine/slots"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

// View satisfies tea.Model.
//
// Bubble Tea v2 returns a View rather than a string, which is also where the
// alt screen is switched on: the screen mode is part of what is being drawn,
// not a program option set once at start-up.
func (m Model) View() tea.View {
	v := tea.NewView(m.body())
	v.AltScreen = true
	return v
}

func (m Model) body() string {
	// Nothing is drawable until the first size message: painting a guess only
	// to repaint it makes the launch flicker.
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.tooSmall() {
		return m.tinyView()
	}
	return m.chrome(m.screenBody())
}

// chrome wraps a screen in the balance bar, the toast slot and the help line.
func (m Model) chrome(body string) string {
	t := m.theme

	var stake int64
	title := m.screen.String()
	if g, ok := m.games[m.screen]; ok {
		stake = g.Stake()
		title = g.Title()
	}

	bodyHeight := maxInt(m.height-chromeHeight, 1)
	table := lipgloss.Place(m.width, bodyHeight, lipgloss.Center, lipgloss.Center, body)

	// The toast slot is three rows whether or not anything is in it.
	toast := ui.Pad(m.toast.View(m.width, t), 3)

	return lipgloss.JoinVertical(lipgloss.Left,
		ui.Bar(title, m.balance(), stake, m.width, t),
		ui.Rule(m.width, t),
		table,
		toast,
		m.helpLine(),
	)
}

func (m Model) helpLine() string {
	keys := helpKeys{global: m.keys}
	if g, ok := m.games[m.screen]; ok {
		keys.game = g.Help()
	}
	return m.help.ShortHelpView(keys.ShortHelp())
}

// helpKeys merges the active game's bindings with the global ones.
type helpKeys struct {
	global KeyMap
	game   []key.Binding
}

func (h helpKeys) ShortHelp() []key.Binding {
	return append(append([]key.Binding{}, h.game...), h.global.ShortHelp()...)
}

func (h helpKeys) FullHelp() [][]key.Binding {
	rows := h.global.FullHelp()
	if len(h.game) > 0 {
		rows = append([][]key.Binding{h.game}, rows...)
	}
	return rows
}

func (m Model) screenBody() string {
	switch m.screen {
	case ScreenMenu:
		return m.menuView()
	case ScreenStats:
		return m.statsView()
	case ScreenHelp:
		return m.helpScreen()
	case ScreenBankrupt:
		return m.bankruptView()
	default:
		if g, ok := m.games[m.screen]; ok {
			return g.View()
		}
		return ""
	}
}

func (m Model) menuView() string {
	t := m.theme
	width := minInt(m.width-8, 72)
	return lipgloss.JoinVertical(lipgloss.Center,
		t.Title.Render("F E L T"),
		t.Dim.Render("a terminal casino"),
		"",
		lipgloss.NewStyle().Width(width).Render(m.menu.View()),
	)
}

// tinyView replaces the layout on terminals too small to hold it.
func (m Model) tinyView() string {
	t := m.theme
	msg := lipgloss.JoinVertical(lipgloss.Center,
		t.Title.Render("felt needs more room"),
		"",
		t.Label.Render(fmt.Sprintf("terminal is %d×%d, minimum is %d×%d",
			m.width, m.height, minWidth, minHeight)),
		t.Dim.Render("resize the window, or press ctrl+c to fold"),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
}

func (m Model) bankruptView() string {
	t := m.theme
	stats := m.ledger.Stats(m.me)
	return lipgloss.JoinVertical(lipgloss.Center,
		t.Lose.Render("YOU ARE OUT OF CREDITS"),
		"",
		t.Label.Render(fmt.Sprintf("Down %s across %d rounds.",
			ui.Credits(-stats.Net()), stats.Rounds())),
		"",
		t.Value.Render(fmt.Sprintf("Stake another %s and carry on?",
			ui.Credits(m.ledger.Bankroll()))),
		"",
		t.Key.Render("y")+t.Label.Render(" rebuy    ")+
			t.Key.Render("n")+t.Label.Render(" back to the menu    ")+
			t.Key.Render("ctrl+c")+t.Label.Render(" walk away"),
	)
}

// statsRows builds the table, one row per game plus a total.
func (m Model) statsRows() []table.Row {
	stats := m.ledger.Stats(m.me)

	games := []struct {
		kind engine.Kind
		name string
	}{
		{engine.KindSlots, "Slots"},
		{engine.KindBlackjack, "Blackjack"},
		{engine.KindRoulette, "Roulette"},
		{engine.KindVideoPoker, "Video Poker"},
	}

	rows := make([]table.Row, 0, len(games)+1)
	for _, g := range games {
		s := stats.Get(g.kind)
		rtp := "—"
		if s.Wagered > 0 {
			rtp = ui.Percent(stats.RTP(g.kind))
		}
		rows = append(rows, table.Row{
			g.name,
			fmt.Sprintf("%d", s.Rounds),
			ui.Credits(s.Wagered),
			ui.Credits(s.Won),
			ui.Signed(s.Won - s.Wagered),
			rtp,
			ui.Signed(s.Best),
		})
	}

	total := "—"
	if stats.Wagered() > 0 {
		total = ui.Percent(float64(stats.Won()) / float64(stats.Wagered()))
	}
	rows = append(rows, table.Row{
		"All",
		fmt.Sprintf("%d", stats.Rounds()),
		ui.Credits(stats.Wagered()),
		ui.Credits(stats.Won()),
		ui.Signed(stats.Net()),
		total,
		"",
	})
	return rows
}

// statsColumns is the shape of the statistics table.
func statsColumns() []table.Column {
	return []table.Column{
		{Title: "Game", Width: 12}, // "Video Poker" is the longest name
		{Title: "Rounds", Width: 7},
		{Title: "Wagered", Width: 10},
		{Title: "Returned", Width: 10},
		{Title: "Net", Width: 10},
		{Title: "RTP", Width: 8},
		{Title: "Best", Width: 9},
	}
}

// statsWidth is the table's natural width: its columns plus the cell padding
// either side of each.
//
// The table is sized to exactly this. Given anything wider it pads its rows
// out to fill the space while the header stays the width of its columns, and
// the two then centre on different points — the rows visibly slide left of
// their own headings.
func statsWidth() int {
	w := 0
	for _, c := range statsColumns() {
		w += c.Width + 2
	}
	return w
}

func newStatsTable(t ui.Theme) table.Model {
	tbl := table.New(
		table.WithColumns(statsColumns()),
		table.WithFocused(true),
		table.WithHeight(6),
	)
	tbl.SetStyles(statsStyles(t))
	return tbl
}

// statsStyles is separate from the constructor so that a change of theme can
// restyle the existing table.
//
// Rebuilding it instead would throw away the width and height the resize gave
// it — and a table of width zero draws its header and nothing else, which is
// exactly what an empty statistics screen looked like.
func statsStyles(t ui.Theme) table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.Border).
		BorderBottom(true).
		Foreground(t.Gold).
		Bold(true)
	s.Selected = s.Selected.Foreground(t.Text).Background(t.Felt).Bold(true)
	s.Cell = s.Cell.Foreground(t.Text)
	return s
}

func (m Model) statsView() string {
	t := m.theme
	stats := m.ledger.Stats(m.me)

	summary := strings.Join([]string{
		t.Label.Render("ROUNDS ") + t.Value.Render(fmt.Sprintf("%d", stats.Rounds())),
		t.Label.Render("WAGERED ") + t.Value.Render(ui.Credits(stats.Wagered())),
		t.Label.Render("NET ") + netStyle(stats.Net(), t).Render(ui.Signed(stats.Net())),
		t.Label.Render("BALANCE ") + t.Value.Render(ui.Credits(m.balance())),
	}, t.Dim.Render("   ·   "))

	return lipgloss.JoinVertical(lipgloss.Center,
		summary,
		"",
		m.statsTable.View(),
		"",
		t.Dim.Render("This is the local wallet. A server keeps its own, and they never mix."),
	)
}

func netStyle(n int64, t ui.Theme) lipgloss.Style {
	switch {
	case n > 0:
		return t.Win
	case n < 0:
		return t.Lose
	default:
		return t.Push
	}
}

// helpScreen wraps the help text in its scrolling viewport.
func (m Model) helpScreen() string {
	t := m.theme
	body := m.helpVP.View()
	if m.helpVP.TotalLineCount() <= m.helpVP.Height() {
		return body
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, t.Dim.Render("↑↓ scroll"))
}

// helpContent lays the help out for the width available: three columns on a
// wide terminal, folding to one as it narrows.
func (m Model) helpContent() string {
	t := m.theme

	section := func(title string, rows ...[2]string) string {
		lines := []string{t.Title.Render(title), ""}
		for _, r := range rows {
			lines = append(lines, fmt.Sprintf("  %s  %s",
				lipgloss.NewStyle().Width(14).Render(t.Key.Render(r[0])),
				t.Label.Render(r[1])))
		}
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	global := section("Everywhere",
		[2]string{"↑ ↓ ← →", "move"},
		[2]string{"enter", "select"},
		[2]string{"esc", "back to the menu"},
		[2]string{"S", "statistics"},
		[2]string{"?", "this screen"},
		[2]string{"ctrl+c", "quit and save"},
	)

	slotKeys := section("Slots",
		[2]string{"space", "spin"},
		[2]string{"← →", "stake per line"},
		[2]string{"↑ ↓", "active lines"},
		[2]string{"p", "paytable"},
		[2]string{"g", "ascii / emoji symbols"},
	)

	blackjackKeys := section("Blackjack",
		[2]string{"← →", "stake  ·  1-4 quick  ·  b type"},
		[2]string{"enter", "deal, and next hand"},
		[2]string{"h  s", "hit  ·  stand"},
		[2]string{"d  p  u", "double  ·  split  ·  surrender"},
		[2]string{"i  n", "take or decline insurance"},
	)

	rouletteKeys := section("Roulette",
		[2]string{"↑ ↓ ← →", "move between betting spots"},
		[2]string{"space", "place a chip"},
		[2]string{"backspace", "take the chip back"},
		[2]string{"+ -", "chip value"},
		[2]string{"enter", "spin"},
		[2]string{"r  c", "repeat last bets  ·  clear"},
	)

	pokerKeys := section("Video poker",
		[2]string{"enter", "deal, then draw"},
		[2]string{"1 - 5", "hold a card"},
		[2]string{"← → space", "move and hold"},
		[2]string{"+ -", "coins  ·  m for max bet"},
	)

	house := section("House edge",
		[2]string{"Slots", fmt.Sprintf("%s — returns %s over the long run",
			ui.Percent(1-slotsengine.TheoreticalRTP()), ui.Percent(slotsengine.TheoreticalRTP()))},
		[2]string{"Blackjack", "~0.5% with basic strategy; insurance costs 5.9%"},
		[2]string{"Roulette", "2.70% on every bet, the zero included"},
		[2]string{"Video poker", "0.5% at 9/6 played perfectly; bet five coins"},
	)

	var keys string
	switch {
	case m.width >= 140:
		keys = ui.Stack(
			ui.Row(4, global, slotKeys, blackjackKeys), "",
			ui.Row(4, rouletteKeys, pokerKeys))
	case m.width >= 96:
		keys = ui.Stack(ui.Row(4, global, slotKeys), "", blackjackKeys, "", rouletteKeys, "", pokerKeys)
	default:
		keys = ui.Stack(global, "", slotKeys, "", blackjackKeys, "", rouletteKeys, "", pokerKeys)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		keys,
		"",
		house,
		"",
		t.Dim.Render("Credits are imaginary. The maths is not."),
	)
}
