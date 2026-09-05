package slots

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	slotsengine "github.com/gateway-of-last-resort/felt/internal/engine/slots"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

// cellWidth is the width of one reel cell. Emoji are two cells wide in most
// terminals, so the window widens with the glyph set rather than letting the
// symbols shove the borders around.
func (m Model) cellWidth() int {
	w := 7
	if m.compact {
		w = 5
	}
	if m.glyphs == games_GlyphsEmoji {
		w++
	}
	return w
}

// games_GlyphsEmoji mirrors games.GlyphsEmoji without importing the parent
// package, which would be a cycle.
const games_GlyphsEmoji = "emoji"

func (m Model) symbolColor(s slotsengine.Symbol) color.Color {
	pick := lipgloss.LightDark(m.theme.Dark)
	switch s {
	case slotsengine.Cherry:
		return pick(lipgloss.Color("#C1121F"), lipgloss.Color("#E5383B"))
	case slotsengine.Lemon:
		return pick(lipgloss.Color("#B8860B"), lipgloss.Color("#F4D35E"))
	case slotsengine.Bell:
		return pick(lipgloss.Color("#B8860B"), lipgloss.Color("#D4AF37"))
	case slotsengine.Bar:
		return pick(lipgloss.Color("#1D3557"), lipgloss.Color("#8ECAE6"))
	case slotsengine.Seven:
		return pick(lipgloss.Color("#7B2CBF"), lipgloss.Color("#C77DFF"))
	case slotsengine.Diamond:
		return pick(lipgloss.Color("#0077B6"), lipgloss.Color("#48CAE4"))
	case slotsengine.Wild:
		return pick(lipgloss.Color("#2D6A4F"), lipgloss.Color("#52B788"))
	default:
		return m.theme.Text
	}
}

func (m Model) symbolStyle(s slotsengine.Symbol) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(m.symbolColor(s)).Bold(true)
}

// View satisfies games.Game.
func (m Model) View() string {
	if m.showPaytable {
		return m.paytableView()
	}

	parts := []string{
		m.reelsView(),
		"",
		m.betView(),
	}
	if line := m.resultView(); line != "" {
		parts = append(parts, "", line)
	}
	parts = append(parts, "", m.hintView())

	return lipgloss.JoinVertical(lipgloss.Center, parts...)
}

// highlightedRows returns the row each reel contributes to the line currently
// being walked.
func (m Model) highlightedRows() (slotsengine.Payline, bool) {
	if m.state != stateResolving || m.winIdx < 0 || m.winIdx >= len(m.wins) {
		return slotsengine.Payline{}, false
	}
	return slotsengine.Paylines[m.wins[m.winIdx].Line], true
}

func (m Model) reelsView() string {
	cw := m.cellWidth()
	t := m.theme
	border := lipgloss.NewStyle().Foreground(t.Border)

	var windows [slotsengine.Reels][3]slotsengine.Symbol
	for i, r := range m.reels {
		windows[i] = r.Window()
	}
	hl, highlighting := m.highlightedRows()

	seg := strings.Repeat("─", cw)
	top := border.Render("╭" + seg + "┬" + seg + "┬" + seg + "╮")
	bottom := border.Render("╰" + seg + "┴" + seg + "┴" + seg + "╯")
	bar := border.Render("│")

	rows := make([]string, 0, 3)
	for row := 0; row < 3; row++ {
		cells := make([]string, 0, slotsengine.Reels)
		for reel := 0; reel < slotsengine.Reels; reel++ {
			lit := highlighting && hl[reel] == row
			cells = append(cells, m.cell(windows[reel][row], cw, lit, m.reels[reel].Blur()))
		}
		rows = append(rows, bar+strings.Join(cells, bar)+bar)
	}

	grid := lipgloss.JoinVertical(lipgloss.Left, append([]string{top}, append(rows, bottom)...)...)
	return lipgloss.JoinVertical(lipgloss.Center, grid, m.lineStrip())
}

// cell renders one symbol inside the reel window.
func (m Model) cell(s slotsengine.Symbol, w int, lit, blur bool) string {
	st := m.symbolStyle(s)
	switch {
	case lit:
		st = lipgloss.NewStyle().
			Foreground(lipgloss.LightDark(m.theme.Dark)(lipgloss.Color("#FFFFFF"), lipgloss.Color("#101010"))).
			Background(m.theme.Gold).
			Bold(true)
	case blur:
		// A reel at speed is not readable; say so with dimming rather than
		// pretending the symbols underneath mean anything.
		st = lipgloss.NewStyle().Foreground(m.theme.Muted)
	}
	return st.Width(w).Align(lipgloss.Center).Render(s.Glyph(m.glyphs))
}

// lineStrip draws the payline numbers under the reels. They sit below rather
// than beside the window on purpose: a payline is not tied to one row, so
// numbers down the side would imply a correspondence that does not exist.
func (m Model) lineStrip() string {
	t := m.theme

	current := -1
	if m.state == stateResolving && m.winIdx >= 0 && m.winIdx < len(m.wins) {
		current = m.wins[m.winIdx].Line
	}
	won := map[int]bool{}
	if m.state == stateResolving || m.state == stateResult {
		for _, w := range m.wins {
			won[w.Line] = true
		}
	}

	cells := make([]string, 0, slotsengine.MaxLines)
	for i := 0; i < slotsengine.MaxLines; i++ {
		label := fmt.Sprintf(" %d ", i+1)
		switch {
		case i == current:
			cells = append(cells, t.Win.Reverse(true).Render(label))
		case i < m.lines && won[i]:
			cells = append(cells, t.Win.Render(label))
		case i < m.lines:
			cells = append(cells, t.Value.Render(label))
		default:
			cells = append(cells, t.Dim.Render(" · "))
		}
	}
	return t.Label.Render("lines ") + strings.Join(cells, "")
}

func (m Model) betView() string {
	t := m.theme

	betCells := make([]string, 0, len(slotsengine.LineBets))
	for i, b := range slotsengine.LineBets {
		label := fmt.Sprintf(" %d ", b)
		if i == m.lineBet {
			betCells = append(betCells, ui.ChipStyle(b, t).Reverse(true).Render(label))
			continue
		}
		betCells = append(betCells, t.Dim.Render(label))
	}

	// The active line count is not repeated here: the strip under the reels
	// already shows which lines are live.
	fields := []string{
		t.Label.Render("LINE BET ") + strings.Join(betCells, ""),
		t.Label.Render("TOTAL ") + t.Value.Render(ui.Credits(m.TotalBet())),
	}
	return strings.Join(fields, t.Dim.Render("   ·   "))
}

func (m Model) resultView() string {
	t := m.theme
	switch m.state {
	case stateSpinning:
		return t.Dim.Render("spinning…")

	case stateResolving:
		if m.winIdx < 0 || m.winIdx >= len(m.wins) {
			return ""
		}
		return t.Win.Render(m.winLine(m.wins[m.winIdx]))

	case stateResult:
		if m.totalWin == 0 {
			return t.Dim.Render("no win")
		}

		summary := ui.WinBox(fmt.Sprintf("+%s win", ui.Credits(m.totalWin)), m.compact, t)

		// A total on its own leaves the player working out what paid, which
		// matters most for the two-cherry win: it does not look like a win
		// until you know the rule. One or two lines are spelled out; more
		// than that is a wall of text, so only the count is given.
		if len(m.wins) == 1 {
			return lipgloss.JoinVertical(lipgloss.Center,
				summary, t.Label.Render(m.winLine(m.wins[0])))
		}
		if len(m.wins) == 2 {
			return lipgloss.JoinVertical(lipgloss.Center,
				summary,
				t.Label.Render(m.winLine(m.wins[0])),
				t.Label.Render(m.winLine(m.wins[1])))
		}
		return lipgloss.JoinVertical(lipgloss.Center,
			summary,
			t.Label.Render(fmt.Sprintf("on %d lines", len(m.wins))))
	}
	return ""
}

// winLine describes one winning line in the machine's own terms: which line,
// which symbol, and how many of them it took.
func (m Model) winLine(w slotsengine.LineWin) string {
	return fmt.Sprintf("line %d  %s×%d  +%s",
		w.Line+1, w.Symbol.Name(), w.Count, ui.Credits(w.Pays))
}

func (m Model) hintView() string {
	t := m.theme
	if m.Busy() {
		return t.Label.Render("press ") + t.Key.Render("space") + t.Label.Render(" to skip")
	}
	return t.Label.Render("press ") + t.Key.Render("space") + t.Label.Render(" to spin")
}

// paytableContent is the body of the paytable overlay.
func (m Model) paytableContent() string {
	t := m.theme
	var b strings.Builder

	b.WriteString(t.Title.Render("Three of a kind, left to right"))
	b.WriteString("\n\n")
	for _, r := range slotsengine.PaytableRows() {
		glyph := m.symbolStyle(r.Symbol).Render(strings.TrimSpace(r.Symbol.Glyph(m.glyphs)))
		fmt.Fprintf(&b, "  %s %s %s\n",
			lipgloss.NewStyle().Width(4).Render(glyph),
			lipgloss.NewStyle().Width(10).Render(t.Value.Render(r.Symbol.Name())),
			t.Win.Render(fmt.Sprintf("×%d", r.Pays)),
		)
	}

	b.WriteString("\n  ")
	b.WriteString(t.Dim.Render(fmt.Sprintf(
		"%s on reels 1-2 pays ×%d.  %s substitutes for any symbol.",
		slotsengine.Cherry.Name(), slotsengine.TwoCherries, slotsengine.Wild.Name())))
	b.WriteString("\n\n")

	b.WriteString(t.Title.Render("Paylines"))
	b.WriteString("\n\n")
	for i, p := range slotsengine.Paylines {
		active := t.Dim
		if i < m.lines {
			active = t.Value
		}
		fmt.Fprintf(&b, "  %s  %s  %s\n",
			active.Render(fmt.Sprintf("%d", i+1)),
			t.Label.Render(slotsengine.PaylineName(i)),
			t.Dim.Render(paylineSketch(p)),
		)
	}

	b.WriteString("\n  ")
	b.WriteString(t.Label.Render("Payouts are multiples of the line bet."))
	b.WriteString("\n  ")
	b.WriteString(t.Label.Render(fmt.Sprintf(
		"Theoretical return %s, %s of lines pay.",
		ui.Percent(slotsengine.TheoreticalRTP()), ui.Percent(slotsengine.HitFrequency()))))

	return b.String()
}

// paylineSketch draws the shape of a line as three row groups, top row first,
// so the diagonals are recognisable on a single text line.
func paylineSketch(p slotsengine.Payline) string {
	var rows [3]string
	for row := range rows {
		var b strings.Builder
		for reel := 0; reel < slotsengine.Reels; reel++ {
			if p[reel] == row {
				b.WriteString("●")
				continue
			}
			b.WriteString("·")
		}
		rows[row] = b.String()
	}
	return strings.Join(rows[:], "│")
}

func (m Model) paytableView() string {
	t := m.theme
	footer := t.Label.Render("press ") + t.Key.Render("p") + t.Label.Render(" or ") +
		t.Key.Render("esc") + t.Label.Render(" to close")
	return lipgloss.JoinVertical(lipgloss.Left, m.vp.View(), "", footer)
}
