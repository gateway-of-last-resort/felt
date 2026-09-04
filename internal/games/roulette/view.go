package roulette

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	rl "github.com/gateway-of-last-resort/felt/internal/engine/roulette"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

// The layout is drawn into a character grid rather than composed from boxes,
// because half the betting spots sit *on* the lines between numbers. A chip
// on the border between 8 and 11 has to appear on that border, and a cursor
// there has to light it up — neither is expressible as a stack of panels.
type cell struct {
	r  rune
	st lipgloss.Style
}

// cellWidth is the inside width of one number cell.
func (m Model) cellWidth() int {
	if m.compact {
		return 3
	}
	return 4
}

// zeroWidth is the width of the zero box on the left.
func (m Model) zeroWidth() int { return m.cellWidth() + 1 }

// colX maps a half-grid x to a character column: even values are cell
// centres, odd ones the borders between them.
func (m Model) colX(gx int) int {
	cw := m.cellWidth()
	if gx%2 == 0 {
		c := gx / 2
		return m.zeroWidth() + c*(cw+1) + 1 + cw/2
	}
	c := gx / 2
	return m.zeroWidth() + (c+1)*(cw+1)
}

// rowY maps a half-grid y to a character row.
func (m Model) rowY(gy int) int { return gy + 1 }

// View satisfies games.Game.
func (m Model) View() string {
	parts := []string{
		m.wheelView(),
		"",
		m.layoutView(),
		"",
		m.statusView(),
	}
	return lipgloss.JoinVertical(lipgloss.Center, parts...)
}

// wheelView shows where the ball is, with its neighbours either side so the
// wheel reads as a wheel rather than a lone number.
func (m Model) wheelView() string {
	t := m.theme

	if !m.spinning && !m.showWin {
		return m.historyView()
	}

	span := 3
	if m.compact {
		span = 2
	}
	nums := m.ball.Neighbours(span)

	cells := make([]string, 0, len(nums))
	for i, n := range nums {
		st := m.numberStyle(n)
		label := fmt.Sprintf(" %2d ", n)
		switch {
		case i == span && !m.ball.Blur():
			st = st.Reverse(true)
		case m.ball.Blur():
			st = t.Dim
		}
		cells = append(cells, st.Render(label))
	}

	line := strings.Join(cells, t.Dim.Render("·"))
	caption := t.Dim.Render("the ball is running…")
	if !m.spinning {
		caption = t.Win.Render(fmt.Sprintf("%d %s", m.result, rl.ColorOf(m.result)))
		if m.lastWin > 0 {
			caption += t.Win.Render("   +" + ui.Credits(m.lastWin))
		} else {
			caption = t.Lose.Render(fmt.Sprintf("%d %s", m.result, rl.ColorOf(m.result)))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Center, line, caption)
}

// historyView is the board of recent numbers beside a real wheel.
func (m Model) historyView() string {
	t := m.theme
	if len(m.snap.History) == 0 {
		return t.Dim.Render("no spins yet")
	}

	cells := make([]string, 0, len(m.snap.History))
	for _, n := range m.snap.History {
		cells = append(cells, m.numberStyle(n).Render(fmt.Sprintf(" %d ", n)))
	}
	return t.Label.Render("last  ") + strings.Join(cells, "")
}

func (m Model) numberColor(n int) color.Color {
	pick := lipgloss.LightDark(m.theme.Dark)
	switch rl.ColorOf(n) {
	case rl.Red:
		return pick(lipgloss.Color("#C1121F"), lipgloss.Color("#E5383B"))
	case rl.Green:
		return pick(lipgloss.Color("#2D6A4F"), lipgloss.Color("#52B788"))
	default:
		return m.theme.Text
	}
}

func (m Model) numberStyle(n int) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(m.numberColor(n)).Bold(true)
}

// layoutView draws the betting layout with the chips and the cursor on it.
func (m Model) layoutView() string {
	t := m.theme
	cw := m.cellWidth()

	width := m.zeroWidth() + rl.Cols*(cw+1) + 1 + 6
	height := m.rowY(9) + 1

	grid := make([][]cell, height)
	blank := lipgloss.NewStyle()
	for y := range grid {
		grid[y] = make([]cell, width)
		for x := range grid[y] {
			grid[y][x] = cell{r: ' ', st: blank}
		}
	}

	border := lipgloss.NewStyle().Foreground(t.Border)
	put := func(x, y int, r rune, st lipgloss.Style) {
		if y < 0 || y >= len(grid) || x < 0 || x >= len(grid[y]) {
			return
		}
		grid[y][x] = cell{r: r, st: st}
	}
	write := func(x, y int, s string, st lipgloss.Style) {
		for i, r := range s {
			put(x+i, y, r, st)
		}
	}

	// The frame around the numbers. Corners, tees and crossings are picked
	// per position rather than drawn as one box, because the inner lines have
	// to be there for chips and the cursor to sit on.
	left, right := m.zeroWidth(), m.zeroWidth()+rl.Cols*(cw+1)
	top, bottom := m.rowY(0)-1, m.rowY(4)+1
	for y := top; y <= bottom; y++ {
		for x := left; x <= right; x++ {
			onRail := (x-left)%(cw+1) == 0
			onEdge := y == top || y == bottom
			onInner := y%2 == 0 && !onEdge

			switch {
			case onEdge && onRail:
				put(x, y, corner(x == left, x == right, y == top, y == bottom), border)
			case onEdge:
				put(x, y, '─', border)
			case onRail && onInner:
				put(x, y, junction(x == left, x == right), border)
			case onRail:
				put(x, y, '│', border)
			case onInner:
				put(x, y, '─', border)
			}
		}
	}

	// The numbers themselves.
	for col := 0; col < rl.Cols; col++ {
		for row := 0; row < rl.Rows; row++ {
			n := col*rl.Rows + (rl.Rows - row)
			label := fmt.Sprintf("%*d", cw-1, n)
			write(left+col*(cw+1)+1, m.rowY(2*row), label, m.numberStyle(n))
		}
	}

	// The zero box, and the outside bets below.
	write(0, m.rowY(2), fmt.Sprintf("%*d", cw-1, 0), m.numberStyle(0))

	// Chips first, then the cursor on top: a chip under the cursor should
	// still be visible.
	staked := m.stakes()
	for _, s := range rl.Spots {
		amount, ok := staked[s.ID]
		if !ok {
			continue
		}
		x, y := m.spotXY(s)
		st := ui.ChipStyle(chipFor(amount), t)
		put(x, y, '●', st)
	}

	// Outside bets are labelled rather than drawn as boxes; there is no room
	// for both a border and the words.
	for _, s := range rl.Spots {
		switch s.Type {
		case rl.Column:
			write(right+2, m.rowY(s.Y), "2:1", t.Label)
		case rl.Dozen, rl.RedBlack, rl.OddEven, rl.HighLow:
			x, y := m.spotXY(s)
			label := s.Label
			st := t.Label
			if s.Label == "red" {
				st = lipgloss.NewStyle().Foreground(m.numberColor(1))
			}
			write(x-len(label)/2, y, label, st)
		}
	}

	// The cursor, drawn last so a chip underneath it stays visible. A bet on
	// a number lights the whole cell; a bet on a line lights the line, which
	// is the only way to tell a split from the numbers it sits between.
	if cur, ok := rl.SpotByID(m.cursor); ok {
		x, y := m.spotXY(cur)
		width := 1
		if cur.Type == rl.Straight && cur.X >= 0 {
			x = left + (cur.X/2)*(cw+1) + 1
			width = cw - 1
		}
		for i := 0; i < width; i++ {
			if y < 0 || y >= len(grid) || x+i < 0 || x+i >= len(grid[y]) {
				continue
			}
			under := grid[y][x+i]
			r := under.r
			if r == ' ' || r == '│' || r == '─' || r == '┼' {
				r = '◆'
			}
			grid[y][x+i] = cell{r: r, st: t.Win.Reverse(true)}
		}
	}

	lines := make([]string, 0, len(grid))
	for _, row := range grid {
		var b strings.Builder
		for _, c := range row {
			b.WriteString(c.st.Render(string(c.r)))
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// corner picks the right box-drawing character for a point on the outer edge.
func corner(atLeft, atRight, atTop, atBottom bool) rune {
	switch {
	case atTop && atLeft:
		return '┌'
	case atTop && atRight:
		return '┐'
	case atBottom && atLeft:
		return '└'
	case atBottom && atRight:
		return '┘'
	case atTop:
		return '┬'
	default:
		return '┴'
	}
}

// junction is where an inner row line meets a column rail.
func junction(atLeft, atRight bool) rune {
	switch {
	case atLeft:
		return '├'
	case atRight:
		return '┤'
	default:
		return '┼'
	}
}

// spotXY is where a betting spot is drawn.
func (m Model) spotXY(s rl.Spot) (int, int) {
	switch s.Type {
	case rl.Column:
		return m.zeroWidth() + rl.Cols*(m.cellWidth()+1) + 2, m.rowY(s.Y)
	}
	if s.X < 0 {
		// The zero box and the basket beside it.
		return m.cellWidth() / 2, m.rowY(s.Y)
	}
	return m.colX(s.X), m.rowY(s.Y)
}

// stakes totals what is on each spot, mine and other players' together.
func (m Model) stakes() map[int]int64 {
	out := map[int]int64{}
	for _, b := range m.snap.Bets {
		out[b.Spot] += b.Amount
	}
	return out
}

// chipFor picks the disc denomination that best represents a stake.
func chipFor(amount int64) int64 {
	d := ui.ChipDenoms[0]
	for _, c := range ui.ChipDenoms {
		if amount >= c {
			d = c
		}
	}
	return d
}

// statusView is the line under the layout: the cursor's bet, the chip in
// hand, and what is on the table.
func (m Model) statusView() string {
	t := m.theme

	if m.spinning {
		return t.Dim.Render("no more bets") + t.Dim.Render("   ·   ") +
			t.Label.Render("press ") + t.Key.Render("space") + t.Label.Render(" to skip")
	}

	cur, _ := rl.SpotByID(m.cursor)
	staked := m.stakes()[m.cursor]

	spot := t.Label.Render("on ") + t.Value.Render(cur.Label) +
		t.Dim.Render(fmt.Sprintf("  %s  %d:1", cur.Type, cur.Type.Odds()))
	if staked > 0 {
		spot += t.Win.Render("  " + ui.Credits(staked))
	}

	chips := make([]string, 0, len(Chips))
	for i, c := range Chips {
		st := t.Dim
		if i == m.chip {
			st = ui.ChipStyle(c, t).Reverse(true)
		}
		chips = append(chips, st.Render(fmt.Sprintf(" %d ", c)))
	}

	table := t.Label.Render("on the table ") + t.Value.Render(ui.Credits(m.snap.MyTotal))

	lines := []string{
		spot,
		t.Label.Render("chip ") + strings.Join(chips, "") + t.Dim.Render("   ·   ") + table,
	}

	if m.snap.MyTotal > 0 {
		lines = append(lines, t.Label.Render("press ")+t.Key.Render("enter")+t.Label.Render(" to spin"))
	} else {
		lines = append(lines, t.Dim.Render("space places a chip · r repeats the last round"))
	}
	return lipgloss.JoinVertical(lipgloss.Center, lines...)
}
