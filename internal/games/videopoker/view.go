package videopoker

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/gateway-of-last-resort/felt/internal/engine/poker/video"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

// View satisfies games.Game.
func (m Model) View() string {
	parts := []string{
		m.paytableView(),
		"",
		m.handView(),
		"",
		m.promptView(),
	}
	return lipgloss.JoinVertical(lipgloss.Center, parts...)
}

// paytableView is the schedule, with the column for the current stake lit.
//
// In video poker the pay table is not reference material tucked behind a key:
// it is what the player reads while deciding what to hold, so it stays on
// screen at all times.
func (m Model) paytableView() string {
	t := m.theme
	coins := m.Stake()

	rows := video.PaytableRows()
	lines := make([]string, 0, len(rows)+1)

	// On a short terminal only the staked column is shown: the other four are
	// the same numbers multiplied out, and the room is better spent on cards.
	columns := []int64{}
	if m.compact {
		columns = append(columns, coins)
	} else {
		for c := int64(1); c <= video.MaxCoins; c++ {
			columns = append(columns, c)
		}
	}

	header := lipgloss.NewStyle().Width(18).Render("")
	for _, c := range columns {
		st := t.Dim
		if c == coins {
			st = t.Title
		}
		header += st.Render(fmt.Sprintf("%6d", c))
	}
	lines = append(lines, header)

	for _, row := range rows {
		name := lipgloss.NewStyle().Width(18).Render(t.Label.Render(row.Name))
		line := name
		for _, c := range columns {
			pay := row.PerCoin * c
			if row.Name == "Royal flush" && c == video.MaxCoins {
				pay = row.Max
			}

			st := t.Dim
			switch {
			case m.paying(row.Name) && c == coins:
				st = t.Win.Reverse(true)
			case c == coins:
				st = t.Value
			case m.paying(row.Name):
				st = t.Win
			}
			line += st.Render(fmt.Sprintf("%6d", pay))
		}
		lines = append(lines, line)
	}

	table := lipgloss.JoinVertical(lipgloss.Left, lines...)
	if m.compact {
		// The border costs two rows, which a short terminal cannot spare.
		return table
	}
	return t.Panel.Render(table)
}

// paying reports whether a paytable row is the hand just made, so the line
// that paid can be lit up.
func (m Model) paying(name string) bool {
	last := m.snap.Last
	if last == nil || last.Payout == 0 || m.drawing() {
		return false
	}

	switch last.Rank.Cat {
	case 9:
		return name == "Royal flush"
	case 8:
		return name == "Straight flush"
	case 7:
		return name == "Four of a kind"
	case 6:
		return name == "Full house"
	case 5:
		return name == "Flush"
	case 4:
		return name == "Straight"
	case 3:
		return name == "Three of a kind"
	case 2:
		return name == "Two pair"
	case 1:
		return name == "Jacks or better"
	default:
		return false
	}
}

func (m Model) handView() string {
	t := m.theme
	if m.shown == 0 && !m.revealing {
		return t.Dim.Render("press enter to deal")
	}

	if m.compact {
		return m.compactHandView()
	}

	cards := make([]string, 0, 5)
	labels := make([]string, 0, 5)

	for i := 0; i < 5; i++ {
		switch {
		case i >= m.shown:
			cards = append(cards, ui.RenderBack(t))
		case m.turning[i]:
			// Still to be replaced: show the back, so the change is visible
			// when it lands.
			cards = append(cards, ui.RenderBack(t))
		default:
			cards = append(cards, ui.RenderCard(m.cards[i], t))
		}

		labels = append(labels, m.cardLabel(i))
	}

	return lipgloss.JoinVertical(lipgloss.Center,
		ui.Row(1, cards...),
		ui.Row(1, labels...),
	)
}

// compactHandView puts the hand on two lines, for a terminal with no room for
// card boxes.
func (m Model) compactHandView() string {
	t := m.theme

	cards := make([]string, 0, 5)
	labels := make([]string, 0, 5)

	for i := 0; i < 5; i++ {
		label := fmt.Sprintf("%d", i+1)
		st := t.Dim
		switch {
		case m.hold[i]:
			label, st = "held", t.Win
		case m.snap.Phase == video.PhaseDraw && i == m.cursor:
			st = t.Value.Reverse(true)
		}

		text := "▒▒"
		if i < m.shown && !m.turning[i] {
			text = m.cards[i].String()
		}
		cardStyle := lipgloss.NewStyle().Foreground(t.Black)
		if i < m.shown && !m.turning[i] && m.cards[i].Suit.Red() {
			cardStyle = cardStyle.Foreground(t.Red)
		}

		cards = append(cards, lipgloss.NewStyle().Width(5).Align(lipgloss.Center).Render(cardStyle.Render(text)))
		labels = append(labels, lipgloss.NewStyle().Width(5).Align(lipgloss.Center).Render(st.Render(label)))
	}

	return lipgloss.JoinVertical(lipgloss.Center,
		strings.Join(cards, " "),
		strings.Join(labels, " "),
	)
}

// cardLabel is the marker under one card: HELD, or the number that holds it.
func (m Model) cardLabel(i int) string {
	t := m.theme
	width := ui.CardWidth

	label := fmt.Sprintf("%d", i+1)
	st := t.Dim
	switch {
	case m.hold[i]:
		label, st = "HELD", t.Win
	case m.snap.Phase == video.PhaseDraw && i == m.cursor:
		st = t.Value.Reverse(true)
	}

	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(st.Render(label))
}

func (m Model) promptView() string {
	t := m.theme

	if m.dealing() || m.drawing() {
		return t.Dim.Render("…")
	}

	if m.snap.Phase == video.PhaseDraw {
		return lipgloss.JoinVertical(lipgloss.Center,
			t.Label.Render("hold with ")+t.Key.Render("1-5")+
				t.Label.Render(" or ")+t.Key.Render("space")+
				t.Label.Render(", then ")+t.Key.Render("enter")+t.Label.Render(" to draw"),
			t.Dim.Render(fmt.Sprintf("%d coins staked", m.snap.Coins)),
		)
	}

	// Betting, with the result of the last hand still up.
	lines := []string{}
	if last := m.snap.Last; last != nil {
		if last.Payout > 0 {
			lines = append(lines, t.Win.Render(fmt.Sprintf("%s  +%s", last.Rank, ui.Credits(last.Payout))))
		} else {
			lines = append(lines, t.Dim.Render(last.Rank.String()))
		}
	}

	coins := make([]string, 0, video.MaxCoins)
	for c := int64(1); c <= video.MaxCoins; c++ {
		st := t.Dim
		if c <= m.coins {
			st = ui.ChipStyle(1, t)
		}
		coins = append(coins, st.Render("●"))
	}

	lines = append(lines,
		t.Label.Render("coins ")+strings.Join(coins, "")+
			t.Value.Render(fmt.Sprintf(" %d", m.coins))+
			t.Dim.Render("   ·   ")+
			t.Key.Render("enter")+t.Label.Render(" deal  ")+
			t.Key.Render("m")+t.Label.Render(" max bet"),
	)

	if m.coins < video.MaxCoins {
		// Worth saying: the royal pays 800 a coin at five, and 250 below it.
		lines = append(lines, t.Dim.Render("the royal pays 800 a coin at five, 250 below"))
	}

	return lipgloss.JoinVertical(lipgloss.Center, lines...)
}
