// Package roulette draws the single-zero table.
//
// Two things make this the most involved screen in the project. The layout is
// not a grid but a graph of betting spots — half of them sit on the lines
// between numbers — so the cursor walks spots rather than cells. And the ball
// animates towards a pocket the engine has already chosen.
package roulette

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/gateway-of-last-resort/felt/internal/driver"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	rl "github.com/gateway-of-last-resort/felt/internal/engine/roulette"
	"github.com/gateway-of-last-resort/felt/internal/games"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

// resultHold is how long the winning number stays up before the table is
// ready for new chips.
const resultHold = 2500 * time.Millisecond

type (
	tickMsg   time.Time
	resultMsg struct{}
)

// Chips are the selectable denominations.
var Chips = []int64{1, 5, 25, 100}

type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Place  key.Binding
	Remove key.Binding
	ChipUp key.Binding
	ChipDn key.Binding
	Spin   key.Binding
	Repeat key.Binding
	Clear  key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑↓←→", "move")),
		Down:   key.NewBinding(key.WithKeys("down", "j")),
		Left:   key.NewBinding(key.WithKeys("left", "h")),
		Right:  key.NewBinding(key.WithKeys("right", "l")),
		Place:  key.NewBinding(key.WithKeys(" ", "space"), key.WithHelp("space", "place")),
		Remove: key.NewBinding(key.WithKeys("backspace"), key.WithHelp("bksp", "take back")),
		ChipUp: key.NewBinding(key.WithKeys("+", "="), key.WithHelp("+/-", "chip")),
		ChipDn: key.NewBinding(key.WithKeys("-", "_")),
		Spin:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "spin")),
		Repeat: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "repeat")),
		Clear:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clear")),
	}
}

// Model is the roulette screen.
type Model struct {
	drv   driver.Driver
	me    engine.PlayerID
	opts  games.Options
	theme ui.Theme
	keys  keyMap

	snap rl.Snapshot

	cursor int
	chip   int

	ball     *Ball
	spinning bool
	lastTick time.Time
	result   int
	lastWin  int64
	showWin  bool

	balance int64

	compact bool
	width   int
	height  int
}

// New returns a table wired to a driver.
func New(drv driver.Driver, opts games.Options, t ui.Theme) Model {
	m := Model{
		drv:     drv,
		me:      drv.Me(),
		opts:    opts,
		theme:   t,
		keys:    defaultKeys(),
		cursor:  rl.DefaultSpot(),
		chip:    1,
		result:  -1,
		balance: -1,
		ball:    NewBall(0, opts.FPS),
	}
	if s, ok := currentSnapshot(drv); ok {
		m.snap = s
	}
	return m
}

func currentSnapshot(drv driver.Driver) (rl.Snapshot, bool) {
	type snapshotter interface{ Snapshot() any }
	if s, ok := drv.(snapshotter); ok {
		snap, ok := s.Snapshot().(rl.Snapshot)
		return snap, ok
	}
	return rl.Snapshot{}, false
}

// Title satisfies games.Game.
func (m Model) Title() string { return "Roulette" }

// Busy satisfies games.Game: chips on the layout, or a ball in the air, mean
// the round cannot be walked away from.
func (m Model) Busy() bool { return m.spinning || m.snap.MyTotal > 0 }

// Modal satisfies games.Game.
func (m Model) Modal() bool { return false }

// Stake satisfies games.Game.
func (m Model) Stake() int64 { return m.snap.MyTotal }

// Help satisfies games.Game.
func (m Model) Help() []key.Binding {
	if m.spinning {
		return nil
	}
	keys := []key.Binding{m.keys.Up, m.keys.Place, m.keys.Remove, m.keys.ChipUp}
	if m.snap.MyTotal > 0 {
		keys = append(keys, m.keys.Spin, m.keys.Clear)
	} else {
		keys = append(keys, m.keys.Repeat)
	}
	return keys
}

// Reset satisfies games.Game.
func (m Model) Reset() games.Game {
	m.spinning = false
	m.showWin = false
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

// Chip is the denomination the cursor places.
func (m Model) Chip() int64 { return Chips[m.chip] }

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

	case tickMsg:
		return m.advance(time.Time(msg))

	case resultMsg:
		m.showWin = false
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// events applies the new table state and, if the wheel turned, sends the ball
// after the number it landed on.
func (m Model) events(msg driver.EventsMsg) (Model, tea.Cmd) {
	m.balance = msg.Balance
	if s, ok := msg.Snapshot.(rl.Snapshot); ok {
		m.snap = s
	}

	spun, ok := driver.Event[rl.Spun](msg.Events)
	if !ok {
		return m, nil
	}

	m.result = spun.Number
	if settled, ok := driver.Event[rl.Settled](msg.Events); ok {
		m.lastWin = settled.Won
	}

	if !m.opts.Animations {
		m.ball = NewBall(rl.PocketIndex(spun.Number), m.opts.FPS)
		return m.land()
	}

	now := time.Now()
	m.ball.Start(now, spun.Number)
	m.spinning = true
	m.lastTick = now
	return m, m.tick()
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(m.opts.Frame(), func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) advance(now time.Time) (Model, tea.Cmd) {
	if !m.spinning {
		return m, nil
	}

	dt := now.Sub(m.lastTick).Seconds()
	if frame := m.opts.Frame().Seconds(); dt <= 0 || dt > 4*frame {
		dt = frame
	}
	m.lastTick = now

	if landed := m.ball.Advance(now, dt); !landed {
		return m, m.tick()
	}
	return m.land()
}

// land shows the result. The money moved when the engine settled; this is the
// animation catching up.
func (m Model) land() (Model, tea.Cmd) {
	m.spinning = false
	m.showWin = true

	cmds := []tea.Cmd{
		tea.Tick(resultHold, func(time.Time) tea.Msg { return resultMsg{} }),
	}
	if m.lastWin > 0 {
		cmds = append(cmds, games.Toast(
			fmt.Sprintf("%d %s  ·  +%s", m.result, rl.ColorOf(m.result), ui.Credits(m.lastWin)),
			ui.Good))
	}
	return m, tea.Batch(cmds...)
}

// skipSpin drops the ball straight into its pocket.
func (m Model) skipSpin() (Model, tea.Cmd) {
	m.ball = NewBall(rl.PocketIndex(m.result), m.opts.FPS)
	return m.land()
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.spinning {
		// The pocket was decided before the ball started moving, so skipping
		// the flight loses nothing but the suspense.
		switch {
		case key.Matches(msg, m.keys.Place), key.Matches(msg, m.keys.Spin):
			return m.skipSpin()
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Up):
		m.cursor = rl.Neighbour(m.cursor, rl.Up)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.cursor = rl.Neighbour(m.cursor, rl.Down)
		return m, nil
	case key.Matches(msg, m.keys.Left):
		m.cursor = rl.Neighbour(m.cursor, rl.Left)
		return m, nil
	case key.Matches(msg, m.keys.Right):
		m.cursor = rl.Neighbour(m.cursor, rl.Right)
		return m, nil

	case key.Matches(msg, m.keys.ChipUp):
		if m.chip < len(Chips)-1 {
			m.chip++
		}
		return m, nil
	case key.Matches(msg, m.keys.ChipDn):
		if m.chip > 0 {
			m.chip--
		}
		return m, nil

	case key.Matches(msg, m.keys.Place):
		return m, m.drv.Do(rl.PlaceBet{P: m.me, Spot: m.cursor, Amount: m.Chip()})

	case key.Matches(msg, m.keys.Remove):
		return m, m.drv.Do(rl.RemoveBet{P: m.me, Spot: m.cursor})

	case key.Matches(msg, m.keys.Clear):
		return m, m.drv.Do(rl.ClearBets{P: m.me})

	case key.Matches(msg, m.keys.Repeat):
		return m.repeat()

	case key.Matches(msg, m.keys.Spin):
		return m, m.drv.Do(rl.Spin{P: m.me})
	}
	return m, nil
}

// repeat replays the last round's stakes as ordinary bets, each paid for in
// the usual way — which is what stops a repeat buying chips the player can no
// longer afford.
func (m Model) repeat() (Model, tea.Cmd) {
	type repeater interface {
		RepeatBets(engine.PlayerID) []rl.Bet
	}

	src, ok := m.drv.(interface{ Game() engine.Game })
	if !ok {
		return m, games.Toast("repeat is not available at this table", ui.Info)
	}
	table, ok := src.Game().(repeater)
	if !ok {
		return m, games.Toast("repeat is not available at this table", ui.Info)
	}

	bets := table.RepeatBets(m.me)
	if len(bets) == 0 {
		return m, games.Toast("nothing to repeat", ui.Info)
	}

	cmds := make([]tea.Cmd, 0, len(bets))
	for _, b := range bets {
		cmds = append(cmds, m.drv.Do(rl.PlaceBet{P: m.me, Spot: b.Spot, Amount: b.Amount}))
	}
	return m, tea.Sequence(cmds...)
}
