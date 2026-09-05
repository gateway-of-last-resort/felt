// Package videopoker draws the Jacks or Better machine.
//
// The screen is mostly the pay table: in video poker the schedule is the
// game, and a player reads it constantly to decide what to hold. The cards
// themselves are five boxes and a row of HELD markers.
package videopoker

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/gateway-of-last-resort/felt/internal/deck"
	"github.com/gateway-of-last-resort/felt/internal/driver"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	"github.com/gateway-of-last-resort/felt/internal/engine/poker/video"
	"github.com/gateway-of-last-resort/felt/internal/games"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

// Cards land a fifth of a second apart, and replacements turn over a little
// slower so the change is visible.
const (
	dealInterval = 180 * time.Millisecond
	drawInterval = 260 * time.Millisecond
)

type revealMsg struct{}

type keyMap struct {
	Deal   key.Binding
	Hold   key.Binding
	Left   key.Binding
	Right  key.Binding
	CoinUp key.Binding
	CoinDn key.Binding
	MaxBet key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Deal:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "deal/draw")),
		Hold:   key.NewBinding(key.WithKeys(" ", "space"), key.WithHelp("space/1-5", "hold")),
		Left:   key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/→", "card")),
		Right:  key.NewBinding(key.WithKeys("right", "l")),
		CoinUp: key.NewBinding(key.WithKeys("+", "=", "up", "k"), key.WithHelp("+/-", "coins")),
		CoinDn: key.NewBinding(key.WithKeys("-", "_", "down", "j")),
		MaxBet: key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "max bet")),
	}
}

// Model is the video poker screen.
type Model struct {
	drv   driver.Driver
	me    engine.PlayerID
	opts  games.Options
	theme ui.Theme
	keys  keyMap

	snap video.Snapshot

	coins  int64
	cursor int
	hold   [5]bool

	// shown is how many of the opening five have been turned over, and
	// turning is the set still to be replaced on a draw. Both exist only so
	// the cards can arrive one at a time.
	//
	// revealing says a deal is in progress, which shown cannot: it reads zero
	// both before the first hand and in the instant after a deal, and those
	// are opposite situations — one takes keys, the other must not.
	revealing bool
	shown     int
	turning   [5]bool
	cards     [5]deck.Card

	balance int64

	compact bool
	width   int
	height  int
}

// New returns a machine wired to a driver.
func New(drv driver.Driver, opts games.Options, t ui.Theme) Model {
	m := Model{
		drv:     drv,
		me:      drv.Me(),
		opts:    opts,
		theme:   t,
		keys:    defaultKeys(),
		coins:   video.MaxCoins,
		balance: -1,
	}
	if s, ok := currentSnapshot(drv); ok {
		m.snap = s
	}
	return m
}

func currentSnapshot(drv driver.Driver) (video.Snapshot, bool) {
	type snapshotter interface{ Snapshot() any }
	if s, ok := drv.(snapshotter); ok {
		snap, ok := s.Snapshot().(video.Snapshot)
		return snap, ok
	}
	return video.Snapshot{}, false
}

// Title satisfies games.Game.
func (m Model) Title() string { return "Video Poker" }

// Busy satisfies games.Game: a hand waiting to be drawn to has coins on it.
func (m Model) Busy() bool {
	return m.snap.Phase == video.PhaseDraw || m.dealing() || m.drawing()
}

// Modal satisfies games.Game.
func (m Model) Modal() bool { return false }

// Stake satisfies games.Game.
func (m Model) Stake() int64 {
	if m.snap.Phase == video.PhaseDraw {
		return m.snap.Coins
	}
	return m.coins
}

// Help satisfies games.Game.
func (m Model) Help() []key.Binding {
	if m.snap.Phase == video.PhaseDraw {
		return []key.Binding{m.keys.Hold, m.keys.Left, m.keys.Deal}
	}
	return []key.Binding{m.keys.Deal, m.keys.CoinUp, m.keys.MaxBet}
}

// Reset satisfies games.Game.
func (m Model) Reset() games.Game {
	m.turning = [5]bool{}
	return m
}

// SetTheme satisfies games.Game.
func (m Model) SetTheme(t ui.Theme) games.Game {
	m.theme = t
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

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) events(msg driver.EventsMsg) (Model, tea.Cmd) {
	m.balance = msg.Balance
	if s, ok := msg.Snapshot.(video.Snapshot); ok {
		m.snap = s
	}

	if dealt, ok := driver.Event[video.Dealt](msg.Events); ok {
		m.cards = dealt.Cards
		m.hold = [5]bool{}
		m.cursor = 0
		m.turning = [5]bool{}
		if !m.opts.Animations {
			m.shown, m.revealing = 5, false
			return m, nil
		}
		m.shown, m.revealing = 0, true
		return m, tea.Tick(dealInterval, func(time.Time) tea.Msg { return revealMsg{} })
	}

	if drawn, ok := driver.Event[video.Drawn](msg.Events); ok {
		m.turning = drawn.Replaced
		if !m.opts.Animations {
			m.cards = drawn.Cards
			m.turning = [5]bool{}
			return m, m.announce(drawn)
		}
		// The replacements arrive one at a time; the held cards never move.
		return m, tea.Batch(
			tea.Tick(drawInterval, func(time.Time) tea.Msg { return revealMsg{} }),
			m.announce(drawn),
		)
	}
	return m, nil
}

// announce is the toast for a paying hand. A losing hand says nothing: the
// screen already shows what it was.
func (m Model) announce(d video.Drawn) tea.Cmd {
	if d.Payout == 0 {
		return nil
	}
	level := ui.Good
	if d.Rank.Cat >= 8 { // straight flush or better
		level = ui.Good
	}
	return games.Toast(fmt.Sprintf("%s  +%s", d.Rank, ui.Credits(d.Payout)), level)
}

// reveal turns over the next card, dealing or drawing.
func (m Model) reveal() (Model, tea.Cmd) {
	if m.revealing {
		m.shown++
		if m.shown < 5 {
			return m, tea.Tick(dealInterval, func(time.Time) tea.Msg { return revealMsg{} })
		}
		m.revealing = false
		return m, nil
	}

	// A draw: replace the next card still marked as turning.
	for i, turning := range m.turning {
		if !turning {
			continue
		}
		if last := m.snap.Last; last != nil {
			m.cards[i] = last.Cards[i]
		}
		m.turning[i] = false

		for _, more := range m.turning {
			if more {
				return m, tea.Tick(drawInterval, func(time.Time) tea.Msg { return revealMsg{} })
			}
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	// While cards are still arriving the machine takes no input. Before the
	// first deal nothing has arrived yet, which is not the same thing.
	if m.dealing() || m.drawing() {
		return m, nil
	}

	if m.snap.Phase == video.PhaseDraw {
		return m.drawKey(msg)
	}
	return m.betKey(msg)
}

// dealing reports whether the opening five are still being turned over. It is
// false before the first deal, when there is nothing to wait for.
func (m Model) dealing() bool { return m.revealing }

// drawing reports whether replacements are still turning over.
func (m Model) drawing() bool {
	for _, t := range m.turning {
		if t {
			return true
		}
	}
	return false
}

func (m Model) betKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Deal):
		return m, m.drv.Do(video.Deal{P: m.me, Coins: m.coins})

	case key.Matches(msg, m.keys.CoinUp):
		if m.coins < video.MaxCoins {
			m.coins++
		}
		return m, nil

	case key.Matches(msg, m.keys.CoinDn):
		if m.coins > 1 {
			m.coins--
		}
		return m, nil

	case key.Matches(msg, m.keys.MaxBet):
		m.coins = video.MaxCoins
		return m, m.drv.Do(video.Deal{P: m.me, Coins: m.coins})
	}

	// A number picks the stake directly.
	if n := digit(msg); n >= 1 && n <= video.MaxCoins {
		m.coins = n
	}
	return m, nil
}

func (m Model) drawKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Deal):
		return m, m.drv.Do(video.Draw{P: m.me, Hold: m.hold})

	case key.Matches(msg, m.keys.Left):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case key.Matches(msg, m.keys.Right):
		if m.cursor < 4 {
			m.cursor++
		}
		return m, nil

	case key.Matches(msg, m.keys.Hold):
		m.hold[m.cursor] = !m.hold[m.cursor]
		return m, nil
	}

	// A number holds that card, which is how the machines have always worked.
	if n := digit(msg); n >= 1 && n <= 5 {
		m.hold[n-1] = !m.hold[n-1]
		m.cursor = int(n) - 1
	}
	return m, nil
}

// digit reads a number key, returning 0 for anything else.
func digit(msg tea.KeyPressMsg) int64 {
	s := msg.String()
	if len(s) != 1 || s[0] < '1' || s[0] > '9' {
		return 0
	}
	return int64(s[0] - '0')
}
