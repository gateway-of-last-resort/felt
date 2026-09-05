package blackjack

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/gateway-of-last-resort/felt/internal/deck"
	bj "github.com/gateway-of-last-resort/felt/internal/engine/blackjack"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

// View satisfies games.Game.
func (m Model) View() string {
	if m.shown.shuffling {
		return m.shuffleView()
	}
	if m.inBetting() && len(m.pending) == 0 && len(m.shown.dealer) == 0 {
		return m.bettingView()
	}
	return m.tableView()
}

func (m Model) shuffleView() string {
	t := m.theme
	return lipgloss.JoinVertical(lipgloss.Center,
		t.Title.Render("Shuffling the shoe"),
		"",
		m.spin.View()+t.Dim.Render(fmt.Sprintf(" %d decks, cut at three quarters", m.snap.Rules.Decks)),
	)
}

func (m Model) bettingView() string {
	t := m.theme
	r := m.snap.Rules

	// Each quick stake is labelled with the key that picks it, key first, so
	// the number pressed is not mistaken for the amount.
	quick := make([]string, 0, len(QuickBets))
	for i, b := range QuickBets {
		st := t.Dim
		if b == m.bet {
			st = ui.ChipStyle(b, t).Reverse(true)
		}
		quick = append(quick,
			t.Key.Render(fmt.Sprintf("%d", i+1))+st.Render(fmt.Sprintf(" %d ", b)))
	}

	body := []string{
		t.Title.Render("Place your bet"),
		"",
		ui.Chips(m.bet, t) + "  " + t.Value.Render(ui.Credits(m.bet)),
		"",
		strings.Join(quick, "  "),
		"",
	}

	if m.editing {
		body = append(body,
			t.Label.Render("bet  ")+m.betIn.View(),
			t.Dim.Render("enter to accept · esc to cancel"),
		)
	} else {
		body = append(body,
			t.Label.Render("←/→ adjust  ·  1-4 quick  ·  ")+
				t.Key.Render("b")+t.Label.Render(" type  ·  ")+
				t.Key.Render("enter")+t.Label.Render(" deal"),
		)
	}

	body = append(body,
		"",
		t.Dim.Render(fmt.Sprintf(
			"%d decks · dealer stands on soft 17 · blackjack pays 3:2 · %d cards left",
			r.Decks, m.snap.CardsLeft)),
		t.Dim.Render("bets are even numbers, so 3:2 and insurance pay in whole credits"),
	)

	return lipgloss.JoinVertical(lipgloss.Center, body...)
}

func (m Model) tableView() string {
	t := m.theme

	sections := []string{
		m.dealerView(),
		"",
		m.handsView(),
	}

	if seat, ok := m.mySeat(); ok && seat.Insurance > 0 {
		sections = append(sections, "",
			t.Label.Render("insurance ")+t.Value.Render(ui.Credits(seat.Insurance)))
	}

	sections = append(sections, "", m.promptView())
	return lipgloss.JoinVertical(lipgloss.Center, sections...)
}

// holeDown reports whether the dealer's second card is still face down on
// screen. During playback that is what has been revealed so far, not what the
// engine knows.
func (m Model) holeDown() bool {
	if len(m.pending) > 0 || len(m.shown.dealer) > 0 {
		return !m.shown.holeShown && len(m.shown.dealer) > 1
	}
	return m.snap.HoleHidden
}

// dealerCards is what the dealer has on screen right now.
func (m Model) dealerCards() []deck.Card {
	if len(m.shown.dealer) > 0 {
		return m.shown.dealer
	}
	return m.snap.Dealer.Cards
}

func (m Model) dealerView() string {
	t := m.theme
	cards := m.dealerCards()
	if len(cards) == 0 {
		return ""
	}

	hole := m.holeDown()
	rendered := m.renderCards(cards, hole)

	// While the hole card is down, only the up card can honestly be totalled.
	var total string
	if hole {
		up := cards[0]
		if up.IsAce() {
			// An ace is shown as an ace: calling it eleven would imply the
			// dealer is committed to counting it that way.
			total = t.Dim.Render("shows A")
		} else {
			total = t.Dim.Render(fmt.Sprintf("shows %d", up.Value()))
		}
	} else {
		total = m.totalLabel(handOf(cards), false)
	}

	return lipgloss.JoinVertical(lipgloss.Center, t.Label.Render("DEALER"), rendered, total)
}

// handOf makes a value view out of raw cards, for totalling what is on screen
// mid-deal rather than what the snapshot holds.
func handOf(cards []deck.Card) bj.HandView {
	h := bj.Hand{Cards: cards}
	total, soft := h.Value()
	return bj.HandView{
		Cards:     cards,
		Total:     total,
		Soft:      soft,
		Blackjack: h.IsBlackjack(),
		Bust:      h.IsBust(),
	}
}

// myHands is what this player has on screen: the revealed cards during a
// deal, the snapshot once it has caught up.
func (m Model) myHands() []bj.HandView {
	seat, ok := m.mySeat()
	if !ok {
		return nil
	}

	shown := m.shown.seats[seat.Seat]
	if len(m.pending) == 0 && len(shown) == 0 {
		return seat.Hands
	}

	views := make([]bj.HandView, 0, len(shown))
	for i, cards := range shown {
		v := handOf(cards)
		v.Bet = seat.Bet
		if i < len(seat.Hands) {
			v.Bet = seat.Hands[i].Bet
			v.Doubled = seat.Hands[i].Doubled
			v.FromSplit = seat.Hands[i].FromSplit
			v.Surrender = seat.Hands[i].Surrender
		}
		if s, ok := m.shown.settled[[2]int{int(seat.Seat), i}]; ok {
			v.Settled = true
			v.Outcome = s.Outcome
			v.Payout = s.Payout
		}
		views = append(views, v)
	}
	return views
}

func (m Model) handsView() string {
	hands := m.myHands()
	if len(hands) == 0 {
		return ""
	}

	// Four hands side by side do not fit a narrow terminal, and stacking the
	// full blocks does not fit either. Past two hands on a compact screen
	// each one collapses to a single line.
	if m.compact && len(hands) > 2 {
		lines := make([]string, 0, len(hands))
		for i, h := range hands {
			lines = append(lines, m.handLine(h, i, len(hands)))
		}
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	views := make([]string, 0, len(hands))
	for i, h := range hands {
		views = append(views, m.handView(h, i, len(hands)))
	}
	return ui.Row(3, views...)
}

func (m Model) handView(h bj.HandView, i, n int) string {
	t := m.theme

	tag := t.Label.Render(tagText(i, n))
	if m.isActive(i) {
		tag = t.Title.Render("▸ " + tagText(i, n))
	}

	stake := t.Dim.Render("bet " + ui.Credits(h.Bet))
	if h.Doubled {
		stake = t.Value.Render("bet"+" "+ui.Credits(h.Bet)) + t.Dim.Render(" doubled")
	}

	lines := []string{
		tag + "  " + stake,
		m.renderCards(h.Cards, false),
		m.totalLabel(h, h.Surrender),
	}
	if h.Settled {
		lines = append(lines, m.outcomeLabel(h))
	}
	return lipgloss.JoinVertical(lipgloss.Center, lines...)
}

// handLine is one hand on one row, for when there is no room for card boxes.
func (m Model) handLine(h bj.HandView, i, n int) string {
	t := m.theme

	marker := "  "
	if m.isActive(i) {
		marker = t.Title.Render("▸ ")
	}

	parts := []string{
		marker + lipgloss.NewStyle().Width(8).Render(t.Label.Render(tagText(i, n))),
		lipgloss.NewStyle().Width(9).Render(t.Dim.Render("bet " + ui.Credits(h.Bet))),
		lipgloss.NewStyle().Width(20).Render(ui.RenderHandCompact(h.Cards, false, t)),
		m.totalLabel(h, h.Surrender),
	}
	if h.Settled {
		parts = append(parts, "  "+m.outcomeLabel(h))
	}
	return strings.Join(parts, " ")
}

func (m Model) isActive(hand int) bool {
	if m.snap.Phase != bj.PhasePlayerTurn || len(m.pending) > 0 {
		return false
	}
	seat, ok := m.mySeat()
	return ok && m.snap.Active == seat.Seat && m.snap.ActiveHand == hand
}

func tagText(i, n int) string {
	if n > 1 {
		return fmt.Sprintf("HAND %d", i+1)
	}
	return "YOU"
}

// renderCards picks between boxed cards and one-line notation by how much
// room is left: four split hands of boxes do not fit anywhere.
func (m Model) renderCards(cards []deck.Card, hole bool) string {
	if len(cards) == 0 {
		return ""
	}
	if m.compact || len(m.myHands()) > 2 {
		return ui.RenderHandCompact(cards, hole, m.theme)
	}
	return ui.RenderHand(cards, hole, m.theme)
}

func (m Model) totalLabel(h bj.HandView, surrendered bool) string {
	t := m.theme
	if len(h.Cards) == 0 {
		return ""
	}
	if surrendered {
		return t.Push.Render("surrendered")
	}

	switch {
	case h.Blackjack:
		return t.Win.Render("BLACKJACK")
	case h.Bust:
		return t.Lose.Render(fmt.Sprintf("%d bust", h.Total))
	case h.Soft:
		return t.Value.Render(fmt.Sprintf("%d", h.Total)) + t.Dim.Render(" soft")
	default:
		return t.Value.Render(fmt.Sprintf("%d", h.Total))
	}
}

func (m Model) outcomeLabel(h bj.HandView) string {
	t := m.theme
	net := h.Payout - h.Bet
	switch h.Outcome {
	case bj.Blackjack, bj.Win:
		return t.Win.Render(fmt.Sprintf("%s +%s", h.Outcome, ui.Credits(net)))
	case bj.Push:
		return t.Push.Render("push")
	case bj.Surrendered:
		return t.Push.Render(fmt.Sprintf("surrender %s", ui.Credits(net)))
	default:
		return t.Lose.Render(fmt.Sprintf("lose %s", ui.Credits(net)))
	}
}

// promptView is the line under the table: what the player can do now.
func (m Model) promptView() string {
	t := m.theme

	if len(m.pending) > 0 {
		return t.Dim.Render("dealing…")
	}

	switch m.snap.Phase {
	case bj.PhaseInsurance:
		seat, _ := m.mySeat()
		cost := bj.InsuranceCost(seat.Bet)
		return lipgloss.JoinVertical(lipgloss.Center,
			t.Title.Render("Dealer shows an ace"),
			t.Label.Render(fmt.Sprintf("insurance costs %s and pays 2:1  ·  ", ui.Credits(cost)))+
				t.Key.Render("i")+t.Label.Render(" take it  ")+
				t.Key.Render("n")+t.Label.Render(" decline"),
			t.Dim.Render("the house edge on insurance is 5.9% — declining is usually right"),
		)

	case bj.PhaseDealerTurn:
		return t.Dim.Render("dealer draws…")

	case bj.PhaseSettle, bj.PhaseBetting, bj.PhaseWaiting:
		return lipgloss.JoinVertical(lipgloss.Center,
			m.roundLine(),
			t.Label.Render("press ")+t.Key.Render("enter")+t.Label.Render(" for the next hand"),
		)

	case bj.PhasePlayerTurn:
		return m.actionBar()
	}
	return ""
}

// roundLine is the net result of the round just settled.
func (m Model) roundLine() string {
	t := m.theme

	var staked, back int64
	for _, h := range m.myHands() {
		if !h.Settled {
			return ""
		}
		staked += h.Bet
		back += h.Payout
	}
	if staked == 0 {
		return ""
	}

	switch net := back - staked; {
	case net > 0:
		return ui.WinBox(fmt.Sprintf("+%s win", ui.Credits(net)), m.compact, t)
	case net < 0:
		return t.Lose.Render(fmt.Sprintf("round  %s", ui.Credits(net)))
	default:
		return t.Push.Render("round  even")
	}
}

// actionBar shows every action the table offers, dimming the ones this hand
// cannot take. Removing them would make the row jump between hands.
func (m Model) actionBar() string {
	t := m.theme
	h, ok := m.activeHand()

	action := func(k, label string, enabled bool) string {
		if !enabled {
			return t.Dim.Render(fmt.Sprintf(" %s %s ", k, label))
		}
		return t.Key.Render(" "+k) + t.Value.Render(" "+label+" ")
	}

	return strings.Join([]string{
		action("h", "hit", ok),
		action("s", "stand", ok),
		action("d", "double", ok && m.canDouble(h)),
		action("p", "split", ok && m.canSplit(h)),
		action("u", "surrender", ok && m.canSurrender(h)),
	}, t.Dim.Render("·"))
}
