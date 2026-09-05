// Package slots draws the three-reel machine.
//
// The engine decides where the reels land; this package makes them look like
// they got there by spinning. A round arrives as one Spun event carrying the
// stops and the wins, and the model then spends two seconds catching up with
// it: reels cruise, brake on a spring, and the winning lines are walked one
// at a time so the player can see what paid.
package slots

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/gateway-of-last-resort/felt/internal/driver"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	slotsengine "github.com/gateway-of-last-resort/felt/internal/engine/slots"
	"github.com/gateway-of-last-resort/felt/internal/games"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

type state int

const (
	stateIdle      state = iota // waiting for a spin
	stateSpinning               // reels turning towards a known result
	stateResolving              // walking the winning lines
	stateResult                 // showing the round total
)

// highlightTime is how long each winning line is held on screen. It is slow
// on purpose: the point of walking the lines one at a time is that the player
// can see which one paid and why, and at a third of a second that reads as a
// flicker rather than an explanation.
const highlightTime = 900 * time.Millisecond

type (
	tickMsg      time.Time
	highlightMsg int
)

type keyMap struct {
	Spin     key.Binding
	BetUp    key.Binding
	BetDown  key.Binding
	LinesUp  key.Binding
	LinesDn  key.Binding
	Paytable key.Binding
	Glyphs   key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Spin:     key.NewBinding(key.WithKeys(" ", "space"), key.WithHelp("space", "spin")),
		BetUp:    key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("←/→", "line bet")),
		BetDown:  key.NewBinding(key.WithKeys("left", "h")),
		LinesUp:  key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/↓", "lines")),
		LinesDn:  key.NewBinding(key.WithKeys("down", "j")),
		Paytable: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "paytable")),
		Glyphs:   key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "symbols")),
	}
}

// Model is the slot machine screen.
type Model struct {
	drv   driver.Driver
	me    engine.PlayerID
	opts  games.Options
	theme ui.Theme
	keys  keyMap

	snap  slotsengine.Snapshot
	reels [slotsengine.Reels]*Reel
	state state

	lineBet int
	lines   int
	balance int64

	wins     []slotsengine.LineWin
	winIdx   int
	totalWin int64
	wagered  int64

	lastTick time.Time

	showPaytable bool
	vp           viewport.Model

	glyphs  string
	compact bool
	width   int
	height  int
}

// New returns a machine wired to a driver.
func New(drv driver.Driver, opts games.Options, t ui.Theme, glyphs string) Model {
	m := Model{
		drv:     drv,
		me:      drv.Me(),
		opts:    opts,
		theme:   t,
		keys:    defaultKeys(),
		state:   stateIdle,
		lines:   slotsengine.MaxLines,
		glyphs:  glyphs,
		winIdx:  -1,
		balance: -1,
	}

	// Park the reels wherever the engine says they are, so the screen opens
	// showing the machine's real position rather than a guess.
	if s, ok := currentSnapshot(drv); ok {
		m.snap = s
	}
	for i := range m.reels {
		m.reels[i] = NewReel(&slotsengine.Strips[i], m.snap.Stops[i])
	}
	m.vp = viewport.New()
	return m
}

func currentSnapshot(drv driver.Driver) (slotsengine.Snapshot, bool) {
	type snapshotter interface{ Snapshot() any }
	if s, ok := drv.(snapshotter); ok {
		snap, ok := s.Snapshot().(slotsengine.Snapshot)
		return snap, ok
	}
	return slotsengine.Snapshot{}, false
}

// Title satisfies games.Game.
func (m Model) Title() string { return "Slots" }

// Busy satisfies games.Game: the root will not let a spin be abandoned.
func (m Model) Busy() bool { return m.state == stateSpinning || m.state == stateResolving }

// Modal satisfies games.Game: with the paytable up the machine keeps the
// keyboard so it can close the overlay on esc itself.
func (m Model) Modal() bool { return m.showPaytable }

// Stake satisfies games.Game.
func (m Model) Stake() int64 { return m.TotalBet() }

// Help satisfies games.Game.
func (m Model) Help() []key.Binding {
	return []key.Binding{m.keys.Spin, m.keys.BetUp, m.keys.LinesUp, m.keys.Paytable, m.keys.Glyphs}
}

// Reset satisfies games.Game. The stake survives; the round does not.
func (m Model) Reset() games.Game {
	m.state = stateIdle
	m.wins = nil
	m.winIdx = -1
	m.totalWin = 0
	m.showPaytable = false
	return m
}

// SetTheme satisfies games.Game.
func (m Model) SetTheme(t ui.Theme) games.Game {
	m.theme = t
	m.vp.SetContent(m.paytableContent())
	return m
}

// SetSize satisfies games.Game.
func (m Model) SetSize(w, h int) games.Game {
	m.width, m.height = w, h
	m.compact = games.Compact(w, h)
	m.vp.SetWidth(maxInt(w-8, 20))
	m.vp.SetHeight(maxInt(h-6, 5))
	m.vp.SetContent(m.paytableContent())
	return m
}

// Init satisfies games.Game.
func (m Model) Init() tea.Cmd { return nil }

// LineBet is the current stake per line.
func (m Model) LineBet() int64 { return slotsengine.LineBets[m.lineBet] }

// TotalBet is the stake for one spin.
func (m Model) TotalBet() int64 { return m.LineBet() * int64(m.lines) }

// Update satisfies games.Game.
func (m Model) Update(msg tea.Msg) (games.Game, tea.Cmd) {
	next, cmd := m.update(msg)
	return next, cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case games.GlyphsMsg:
		m.glyphs = msg.Glyphs
		m.vp.SetContent(m.paytableContent())
		return m, nil

	case driver.EventsMsg:
		return m.events(msg)

	case driver.ErrorMsg:
		m.state = stateIdle
		return m, games.Toast(msg.Err.Error(), ui.Bad)

	case tickMsg:
		return m.advance(time.Time(msg))

	case highlightMsg:
		return m.highlight(int(msg))

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// events takes the outcome of a spin and starts the animation that will
// arrive at it.
func (m Model) events(msg driver.EventsMsg) (Model, tea.Cmd) {
	m.balance = msg.Balance
	if s, ok := msg.Snapshot.(slotsengine.Snapshot); ok {
		m.snap = s
	}

	spun, ok := driver.Event[slotsengine.Spun](msg.Events)
	if !ok {
		return m, nil
	}

	m.wins = spun.Wins
	m.totalWin = spun.Total
	m.wagered = spun.Wagered
	m.winIdx = -1

	if !m.opts.Animations {
		// Without animation the reels are simply already there.
		for i := range m.reels {
			m.reels[i] = NewReel(&slotsengine.Strips[i], spun.Stops[i])
		}
		return m.finish()
	}

	now := time.Now()
	for i := range m.reels {
		m.reels[i].Start(now.Add(spinTime+time.Duration(i)*reelStagger), spun.Stops[i])
	}
	m.state = stateSpinning
	m.lastTick = now
	return m, m.tick()
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(m.opts.Frame(), func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) advance(now time.Time) (Model, tea.Cmd) {
	if m.state != stateSpinning {
		return m, nil
	}

	dt := now.Sub(m.lastTick).Seconds()
	// A dropped frame, or a laptop waking from sleep, must not teleport the
	// reels: clamp to one nominal frame.
	if frame := m.opts.Frame().Seconds(); dt <= 0 || dt > 4*frame {
		dt = frame
	}
	m.lastTick = now

	spinning := false
	for _, r := range m.reels {
		r.Advance(now, dt)
		if r.Spinning() {
			spinning = true
		}
	}
	if spinning {
		return m, m.tick()
	}
	return m.startResolving()
}

// startResolving walks the winning lines, or goes straight to the total when
// nothing paid.
func (m Model) startResolving() (Model, tea.Cmd) {
	if len(m.wins) == 0 {
		return m.finish()
	}
	m.state = stateResolving
	m.winIdx = 0
	return m, tea.Tick(highlightTime, func(time.Time) tea.Msg { return highlightMsg(1) })
}

func (m Model) highlight(i int) (Model, tea.Cmd) {
	if m.state != stateResolving {
		return m, nil
	}
	if i >= len(m.wins) {
		return m.finish()
	}
	m.winIdx = i
	return m, tea.Tick(highlightTime, func(time.Time) tea.Msg { return highlightMsg(i + 1) })
}

// finish shows the round total. The money has already moved — the driver
// settled it before this screen ever saw the event.
//
// The total stays on screen until the next spin rather than timing out. A
// result that clears itself after a second is one the player has to catch;
// leaving it up means they can look away and still find out what happened.
func (m Model) finish() (Model, tea.Cmd) {
	m.winIdx = -1
	m.state = stateResult

	// No toast for an ordinary win: the plate under the reels already says
	// it, and saying it twice on one screen reads as a glitch. A jackpot
	// still gets one, because that is worth interrupting for.
	if m.totalWin >= m.wagered*20 && m.totalWin > 0 {
		return m, games.Toast(fmt.Sprintf("JACKPOT  +%s", ui.Credits(m.totalWin)), ui.Good)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	// The paytable is a modal overlay: while it is up it eats the keys.
	if m.showPaytable {
		switch msg.String() {
		case "p", "esc":
			m.showPaytable = false
			return m, nil
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(msg, m.keys.Paytable):
		m.showPaytable = true
		m.vp.SetContent(m.paytableContent())
		m.vp.GotoTop()
		return m, nil

	case key.Matches(msg, m.keys.Glyphs):
		return m, func() tea.Msg { return games.ToggleGlyphsMsg{} }

	case key.Matches(msg, m.keys.Spin):
		// Space does double duty: it skips an animation in progress, and
		// starts the next spin once there is nothing to skip. Waiting out a
		// walk you have already read is the one thing a slot machine can do
		// that is genuinely annoying.
		if m.Busy() {
			return m.skipAnimation()
		}
		return m.spin()

	case key.Matches(msg, m.keys.BetUp):
		if !m.Busy() && m.lineBet < len(slotsengine.LineBets)-1 {
			m.lineBet++
		}
		return m, nil

	case key.Matches(msg, m.keys.BetDown):
		if !m.Busy() && m.lineBet > 0 {
			m.lineBet--
		}
		return m, nil

	case key.Matches(msg, m.keys.LinesUp):
		if !m.Busy() && m.lines < slotsengine.MaxLines {
			m.lines++
		}
		return m, nil

	case key.Matches(msg, m.keys.LinesDn):
		if !m.Busy() && m.lines > 1 {
			m.lines--
		}
		return m, nil
	}
	return m, nil
}

// skipAnimation jumps to the end of the round in progress. The result is
// already decided — the engine settled it before the reels started turning —
// so nothing is lost by arriving early.
func (m Model) skipAnimation() (Model, tea.Cmd) {
	switch m.state {
	case stateSpinning:
		for i := range m.reels {
			m.reels[i] = NewReel(&slotsengine.Strips[i], m.snap.Stops[i])
		}
		return m.finish()
	case stateResolving:
		return m.finish()
	}
	return m, nil
}

// spin asks the driver for a round. Nothing moves until the answer comes
// back, so a stake that cannot be paid leaves the reels still.
func (m Model) spin() (Model, tea.Cmd) {
	if m.Busy() {
		return m, nil
	}
	m.wins = nil
	m.winIdx = -1
	m.totalWin = 0
	m.state = stateIdle

	return m, m.drv.Do(slotsengine.Spin{
		P:       m.me,
		Lines:   m.lines,
		PerLine: m.LineBet(),
	})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
