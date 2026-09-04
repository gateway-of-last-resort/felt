// Package blackjack draws the six-deck table.
//
// The engine deals a whole round the moment a bet is placed and hands back
// every card as an event. This model puts them on screen a quarter of a
// second apart: it holds a queue of pending events and reveals them on a
// timer, so the round the player watches is the round that already happened.
package blackjack

import (
	"fmt"
	"strconv"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/gateway-of-last-resort/felt/internal/deck"
	"github.com/gateway-of-last-resort/felt/internal/driver"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	bj "github.com/gateway-of-last-resort/felt/internal/engine/blackjack"
	"github.com/gateway-of-last-resort/felt/internal/games"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

// Timings. Cards land a quarter of a second apart, about the pace of a real
// deal; the dealer draws a little slower so the totals can be read.
const (
	dealInterval   = 250 * time.Millisecond
	dealerInterval = 450 * time.Millisecond
	settleHold     = 400 * time.Millisecond
)

// QuickBets are the one-key stakes. Every one is even, because 3:2 and
// insurance both halve.
var QuickBets = []int64{2, 10, 50, 100}

type revealMsg struct{}

type keyMap struct {
	Deal      key.Binding
	BetUp     key.Binding
	BetDown   key.Binding
	BetCustom key.Binding
	Hit       key.Binding
	Stand     key.Binding
	Double    key.Binding
	Split     key.Binding
	Surrender key.Binding
	Yes       key.Binding
	No        key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Deal:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "deal")),
		BetUp:     key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("←/→", "bet")),
		BetDown:   key.NewBinding(key.WithKeys("left", "h")),
		BetCustom: key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "type a bet")),
		Hit:       key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "hit")),
		Stand:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stand")),
		Double:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "double")),
		Split:     key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "split")),
		Surrender: key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "surrender")),
		Yes:       key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "insure")),
		No:        key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "no insurance")),
	}
}

// Model is the blackjack screen.
type Model struct {
	drv   driver.Driver
	me    engine.PlayerID
	opts  games.Options
	theme ui.Theme
	keys  keyMap

	snap bj.Snapshot

	// pending holds the events still to be revealed, and shown is what the
	// table looks like so far. Dealing is an animation over a result that is
	// already decided.
	pending   []engine.Event
	shown     dealState
	finalSnap bj.Snapshot

	bet     int64
	betIn   textinput.Model
	editing bool

	balance int64

	spin spinner.Model

	compact bool
	width   int
	height  int
}

// dealState is what has been turned over so far.
type dealState struct {
	seats     map[engine.Seat][][]deck.Card
	dealer    []deck.Card
	holeShown bool
	settled   map[[2]int]bj.HandSettled
	roundOver bool
	shuffling bool
}

// New returns a table wired to a driver.
func New(drv driver.Driver, opts games.Options, t ui.Theme) Model {
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = 6
	in.SetWidth(8)

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))

	m := Model{
		drv:     drv,
		me:      drv.Me(),
		opts:    opts,
		theme:   t,
		keys:    defaultKeys(),
		bet:     QuickBets[0],
		betIn:   in,
		spin:    sp,
		balance: -1,
	}
	if s, ok := currentSnapshot(drv); ok {
		m.snap = s
		m.bet = s.Rules.ClampBet(m.bet)
	}
	m.shown = newDealState()
	return m
}

func newDealState() dealState {
	return dealState{
		seats:   map[engine.Seat][][]deck.Card{},
		settled: map[[2]int]bj.HandSettled{},
	}
}

func currentSnapshot(drv driver.Driver) (bj.Snapshot, bool) {
	type snapshotter interface{ Snapshot() any }
	if s, ok := drv.(snapshotter); ok {
		snap, ok := s.Snapshot().(bj.Snapshot)
		return snap, ok
	}
	return bj.Snapshot{}, false
}

// Title satisfies games.Game.
func (m Model) Title() string { return "Blackjack" }

// Busy satisfies games.Game. Anything past betting has money on the table, so
// the root will not let the player walk away from it — not only during an
// animation, but for the whole round.
func (m Model) Busy() bool {
	if len(m.pending) > 0 {
		return true
	}
	switch m.snap.Phase {
	case bj.PhaseBetting, bj.PhaseWaiting, bj.PhaseSettle:
		return false
	default:
		return true
	}
}

// Modal satisfies games.Game: while a bet is being typed the table keeps every
// key, esc included, so the field can cancel itself.
func (m Model) Modal() bool { return m.editing }

// Stake satisfies games.Game.
func (m Model) Stake() int64 {
	if m.inBetting() {
		return m.bet
	}
	var total int64
	for _, s := range m.snap.Seats {
		if s.Player != m.me {
			continue
		}
		total += s.Insurance
		for _, h := range s.Hands {
			total += h.Bet
		}
	}
	return total
}

func (m Model) inBetting() bool {
	return m.snap.Phase == bj.PhaseBetting || m.snap.Phase == bj.PhaseWaiting
}

// Help satisfies games.Game, listing only what can be pressed right now.
func (m Model) Help() []key.Binding {
	if len(m.pending) > 0 {
		return nil
	}
	switch m.snap.Phase {
	case bj.PhaseBetting, bj.PhaseWaiting, bj.PhaseSettle:
		return []key.Binding{m.keys.Deal, m.keys.BetUp, m.keys.BetCustom}
	case bj.PhaseInsurance:
		return []key.Binding{m.keys.Yes, m.keys.No}
	case bj.PhasePlayerTurn:
		keys := []key.Binding{m.keys.Hit, m.keys.Stand}
		if h, ok := m.activeHand(); ok {
			if m.canDouble(h) {
				keys = append(keys, m.keys.Double)
			}
			if m.canSplit(h) {
				keys = append(keys, m.keys.Split)
			}
			if m.canSurrender(h) {
				keys = append(keys, m.keys.Surrender)
			}
		}
		return keys
	default:
		return nil
	}
}

// Reset satisfies games.Game.
func (m Model) Reset() games.Game {
	m.pending = nil
	m.shown = newDealState()
	m.editing = false
	m.betIn.Blur()
	return m
}

// SetTheme satisfies games.Game.
func (m Model) SetTheme(t ui.Theme) games.Game {
	m.theme = t
	m.spin.Style = t.Dim
	return m
}

// SetSize satisfies games.Game.
func (m Model) SetSize(w, h int) games.Game {
	m.width, m.height = w, h
	m.compact = games.Compact(w, h)
	return m
}

// Init satisfies games.Game.
func (m Model) Init() tea.Cmd { return nil }

// Update satisfies games.Game.
func (m Model) Update(msg tea.Msg) (games.Game, tea.Cmd) {
	next, cmd := m.update(msg)
	return next, cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case driver.EventsMsg:
		return m.events(msg)

	case driver.ErrorMsg:
		return m, games.Toast(msg.Err.Error(), ui.Bad)

	case revealMsg:
		return m.reveal()

	case spinner.TickMsg:
		if !m.shown.shuffling {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// events queues a round for playback. The snapshot is not applied yet: it
// already shows the finished round, and applying it now would skip the deal.
func (m Model) events(msg driver.EventsMsg) (Model, tea.Cmd) {
	m.balance = msg.Balance

	if !m.opts.Animations {
		if s, ok := msg.Snapshot.(bj.Snapshot); ok {
			m.snap = s
		}
		m.shown = newDealState()
		return m, nil
	}

	m.pending = append(m.pending, msg.Events...)
	if s, ok := msg.Snapshot.(bj.Snapshot); ok {
		m.finalSnap = s
	}
	if len(m.pending) == 0 {
		return m, nil
	}
	return m.reveal()
}

// reveal turns over the next event in the queue and schedules the one after.
func (m Model) reveal() (Model, tea.Cmd) {
	if len(m.pending) == 0 {
		// Everything has been shown, so the real snapshot can take over.
		m.snap = m.finalSnap
		return m, nil
	}

	e := m.pending[0]
	m.pending = m.pending[1:]

	delay := dealInterval
	switch ev := e.(type) {
	case bj.ShuffleStarted:
		m.shown = newDealState()
		m.shown.shuffling = true
		delay = 600 * time.Millisecond

	case bj.BetPlaced:
		m.shown = newDealState()

	case bj.CardDealt:
		m.shown.shuffling = false
		if ev.Seat == engine.DealerSeat {
			m.shown.dealer = append(m.shown.dealer, ev.Card)
			if !ev.Hidden {
				m.shown.holeShown = len(m.shown.dealer) > 1
			}
			if m.dealerDrawing() {
				delay = dealerInterval
			}
			break
		}
		m.addCard(ev.Seat, ev.Hand, ev.Card)

	case bj.HandSplit:
		// The split itself is instant; its two new cards follow as events.
		m.splitShown(ev.Seat, ev.Hand)
		delay = dealInterval

	case bj.HoleRevealed:
		m.shown.holeShown = true
		delay = dealerInterval

	case bj.InsuranceTaken:
		delay = dealInterval

	case bj.TurnStarted:
		// Nothing to reveal; the turn is whatever the snapshot says.
		delay = 0

	case bj.HandSettled:
		m.shown.settled[[2]int{int(ev.Seat), ev.Hand}] = ev
		delay = settleHold

	case bj.RoundEnded:
		m.shown.roundOver = true
		delay = 0
	}

	if len(m.pending) == 0 {
		m.snap = m.finalSnap
		return m, nil
	}
	if delay <= 0 {
		return m.reveal()
	}

	cmds := []tea.Cmd{tea.Tick(delay, func(time.Time) tea.Msg { return revealMsg{} })}
	if m.shown.shuffling {
		cmds = append(cmds, m.spin.Tick)
	}
	return m, tea.Batch(cmds...)
}

// dealerDrawing reports whether the dealer is past the opening two cards, so
// the extra ones land at the slower pace.
func (m Model) dealerDrawing() bool { return len(m.shown.dealer) > 2 }

func (m Model) addCard(seat engine.Seat, hand int, c deck.Card) {
	hands := m.shown.seats[seat]
	for len(hands) <= hand {
		hands = append(hands, nil)
	}
	hands[hand] = append(hands[hand], c)
	m.shown.seats[seat] = hands
}

func (m Model) splitShown(seat engine.Seat, hand int) {
	hands := m.shown.seats[seat]
	if hand >= len(hands) || len(hands[hand]) != 2 {
		return
	}
	left := []deck.Card{hands[hand][0]}
	right := []deck.Card{hands[hand][1]}

	next := make([][]deck.Card, 0, len(hands)+1)
	next = append(next, hands[:hand]...)
	next = append(next, left, right)
	next = append(next, hands[hand+1:]...)
	m.shown.seats[seat] = next
}

// activeHand is the hand the player is acting on.
func (m Model) activeHand() (bj.HandView, bool) {
	seat, ok := m.mySeat()
	if !ok || m.snap.Phase != bj.PhasePlayerTurn || m.snap.Active != seat.Seat {
		return bj.HandView{}, false
	}
	if m.snap.ActiveHand >= len(seat.Hands) {
		return bj.HandView{}, false
	}
	return seat.Hands[m.snap.ActiveHand], true
}

func (m Model) mySeat() (bj.SeatView, bool) {
	for _, s := range m.snap.Seats {
		if s.Player == m.me {
			return s, true
		}
	}
	return bj.SeatView{}, false
}

// The three conditional actions. They are worked out from the snapshot rather
// than from the engine's Hand type, because online the snapshot is all there
// is — the rules live on the server.
func (m Model) canDouble(h bj.HandView) bool {
	r := m.snap.Rules
	if len(h.Cards) != 2 || h.Doubled || h.Settled {
		return false
	}
	if h.FromSplit && !r.DoubleAfterSplit {
		return false
	}
	return true
}

func (m Model) canSplit(h bj.HandView) bool {
	seat, ok := m.mySeat()
	if !ok || len(h.Cards) != 2 || h.Doubled {
		return false
	}
	if seat.Splits >= m.snap.Rules.MaxSplits {
		return false
	}
	return h.Cards[0].Value() == h.Cards[1].Value()
}

func (m Model) canSurrender(h bj.HandView) bool {
	return m.snap.Rules.Surrender && len(h.Cards) == 2 && !h.FromSplit && !h.Doubled
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.editing {
		return m.editBet(msg)
	}
	// While a round is being dealt the table takes no input: the player would
	// be acting on cards they cannot see yet.
	if len(m.pending) > 0 {
		return m, nil
	}

	switch m.snap.Phase {
	case bj.PhaseBetting, bj.PhaseWaiting, bj.PhaseSettle:
		return m.bettingKey(msg)
	case bj.PhaseInsurance:
		return m.insuranceKey(msg)
	case bj.PhasePlayerTurn:
		return m.playerKey(msg)
	}
	return m, nil
}

func (m Model) bettingKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	r := m.snap.Rules
	switch {
	case key.Matches(msg, m.keys.Deal):
		return m, m.drv.Do(bj.PlaceBet{P: m.me, Amount: m.bet})

	case key.Matches(msg, m.keys.BetUp):
		m.bet = r.ClampBet(m.bet + r.MinBet)
		return m, nil

	case key.Matches(msg, m.keys.BetDown):
		m.bet = r.ClampBet(m.bet - r.MinBet)
		return m, nil

	case key.Matches(msg, m.keys.BetCustom):
		m.editing = true
		m.betIn.SetValue("")
		return m, m.betIn.Focus()
	}

	// Number keys pick a stake outright.
	if n, err := strconv.Atoi(msg.String()); err == nil && n >= 1 && n <= len(QuickBets) {
		m.bet = r.ClampBet(QuickBets[n-1])
	}
	return m, nil
}

func (m Model) editBet(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editing = false
		m.betIn.Blur()
		return m, nil

	case "enter":
		m.editing = false
		m.betIn.Blur()

		n, err := strconv.ParseInt(m.betIn.Value(), 10, 64)
		if err != nil {
			return m, games.Toast("that is not a number", ui.Bad)
		}
		r := m.snap.Rules
		rounded := r.ClampBet(n)
		m.bet = rounded
		if rounded != n {
			// Say so rather than quietly taking a different bet than asked.
			return m, games.Toast(fmt.Sprintf("bets are even, between %s and %s — taking %s",
				ui.Credits(r.MinBet), ui.Credits(r.MaxBet), ui.Credits(rounded)), ui.Info)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.betIn, cmd = m.betIn.Update(msg)
	return m, cmd
}

func (m Model) insuranceKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	seat, ok := m.mySeat()
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.Yes):
		return m, m.drv.Do(bj.Insure{P: m.me, Yes: true, Amount: bj.InsuranceCost(seat.Bet)})
	case key.Matches(msg, m.keys.No):
		return m, m.drv.Do(bj.Insure{P: m.me, Yes: false})
	}
	return m, nil
}

func (m Model) playerKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	h, ok := m.activeHand()
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Hit):
		return m, m.drv.Do(bj.Hit{P: m.me})

	case key.Matches(msg, m.keys.Stand):
		return m, m.drv.Do(bj.Stand{P: m.me})

	case key.Matches(msg, m.keys.Double):
		if !m.canDouble(h) {
			return m, nil
		}
		return m, m.drv.Do(bj.Double{P: m.me, Amount: h.Bet})

	case key.Matches(msg, m.keys.Split):
		if !m.canSplit(h) {
			return m, nil
		}
		return m, m.drv.Do(bj.Split{P: m.me, Amount: h.Bet})

	case key.Matches(msg, m.keys.Surrender):
		if !m.canSurrender(h) {
			return m, nil
		}
		return m, m.drv.Do(bj.Surrender{P: m.me})
	}
	return m, nil
}
