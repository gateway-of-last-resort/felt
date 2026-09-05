// Package app is the client's root model: it routes between screens, owns the
// wallet and the theme, and hands the terminal to ssh when the player goes
// online.
package app

import (
	"math/rand/v2"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/gateway-of-last-resort/felt/internal/bank"
	"github.com/gateway-of-last-resort/felt/internal/driver"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	bjengine "github.com/gateway-of-last-resort/felt/internal/engine/blackjack"
	videoengine "github.com/gateway-of-last-resort/felt/internal/engine/poker/video"
	rlengine "github.com/gateway-of-last-resort/felt/internal/engine/roulette"
	slotsengine "github.com/gateway-of-last-resort/felt/internal/engine/slots"
	"github.com/gateway-of-last-resort/felt/internal/games"
	bjgame "github.com/gateway-of-last-resort/felt/internal/games/blackjack"
	rlgame "github.com/gateway-of-last-resort/felt/internal/games/roulette"
	slotsgame "github.com/gateway-of-last-resort/felt/internal/games/slots"
	vpgame "github.com/gateway-of-last-resort/felt/internal/games/videopoker"
	"github.com/gateway-of-last-resort/felt/internal/menu"
	"github.com/gateway-of-last-resort/felt/internal/online"
	"github.com/gateway-of-last-resort/felt/internal/store"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

// Terminal size thresholds. Below the minimum the layout is replaced by a
// request to resize, because a sheared table is worse than an honest message.
// Where the layout merely tightens is games.Compact, which the screens share.
const (
	minWidth  = 80
	minHeight = 24
)

// menuHeaderHeight is the title block above the list, which the list has to
// be sized short by or the menu overruns its area.
const menuHeaderHeight = 4

// chromeHeight is what the root reserves around a screen: the balance bar, a
// rule, the toast slot and the help line. The toast slot stays open even when
// empty so a notification does not shove the table upwards mid-spin.
const chromeHeight = 7

// onlineStatus mirrors online.Status without the package having to be
// imported by the message file.
type onlineStatus = online.Status

// Model is the client root.
type Model struct {
	screen Screen
	prev   Screen

	games map[Screen]games.Game
	menu  menu.Model

	ledger   *bank.JSONLedger
	me       engine.PlayerID
	settings store.Settings

	toast ui.Toast
	theme ui.Theme
	help  help.Model
	keys  KeyMap

	statsTable table.Model
	helpVP     viewport.Model

	server online.Status

	width  int
	height int
}

// New builds the root.
func New(ledger *bank.JSONLedger, settings store.Settings, r *rand.Rand) Model {
	// Dark until the terminal answers; a wrong guess is corrected within a
	// frame or two, and dark is what most terminals are.
	t := ui.Default()

	m := Model{
		screen:   ScreenMenu,
		prev:     ScreenMenu,
		ledger:   ledger,
		me:       engine.LocalPlayer,
		settings: settings,
		theme:    t,
		help:     help.New(),
		keys:     DefaultKeyMap(),
		server:   online.Status{Addr: settings.Server},
	}

	opts := games.LocalOptions()
	save := func() { _ = ledger.Save() }

	// Every offline table bets from the wallet, so nobody buys in: the seat
	// is free and the only way Join can fail is a table that is already
	// taken, which cannot happen at a table built for one.
	seat := func(g engine.Game) *driver.Local {
		d := driver.NewLocal(g, ledger, m.me, save)
		_ = d.Join(0)
		return d
	}

	m.games = map[Screen]games.Game{
		ScreenSlots: slotsgame.New(
			seat(slotsengine.NewTable(r)), opts, t, settings.Glyphs),
		ScreenBlackjack: bjgame.New(
			seat(bjengine.NewTable(r, bjengine.Vegas6(), 1)), opts, t),
		ScreenRoulette: rlgame.New(
			seat(rlengine.NewTable(r)), opts, t),
		ScreenVideoPoker: vpgame.New(
			seat(videoengine.NewTable(r)), opts, t),
	}

	m.menu = menu.New(m.menuItems(), t)
	m.statsTable = newStatsTable(t)
	m.helpVP = viewport.New()
	return m
}

func (m Model) menuItems() []menu.Item {
	return []menu.Item{
		{
			Key: "slots", Name: "Slots",
			Desc:   "Three reels, five lines, one spring-loaded stop.",
			MinBet: slotsengine.LineBets[0], RTP: slotsengine.TheoreticalRTP(), Ready: true,
		},
		{
			Key: "blackjack", Name: "Blackjack",
			Desc: "Six decks, dealer stands on soft 17, blackjack pays 3:2.",
			// The textbook figure for these rules played perfectly, not
			// something this package computes.
			MinBet: bjengine.Vegas6().MinBet, RTP: 0.995, Ready: true,
		},
		{
			Key: "roulette", Name: "Roulette",
			Desc:   "Single zero. The house keeps 2.70% and never hurries.",
			MinBet: 1, RTP: 0.973, Ready: true,
		},
		{
			Key: "videopoker", Name: "Video Poker",
			Desc: "Jacks or Better, 9/6. Five cards, hold what you like, draw once.",
			// The textbook return for this schedule played perfectly.
			MinBet: 1, RTP: 0.995, Ready: true,
		},
		m.onlineItem(),
		{Key: "stats", Name: "Statistics", Desc: "What the session has actually cost you.", Ready: true},
		{Key: "help", Name: "Help", Desc: "Keys, rules and house edges.", Ready: true},
	}
}

// onlineItem describes the Online row, which changes with what the check
// found. It is honest about a server being down rather than opening ssh and
// letting it fail in the player's face.
func (m Model) onlineItem() menu.Item {
	it := menu.Item{
		Key:  "online",
		Name: "Online",
		Desc: "Play at a shared table on " + m.settings.Server + ".",
	}
	switch {
	case m.server.Ready():
		it.Ready = true
		it.Note = "server up  ·  as " + m.settings.Nick
	case !m.server.HasSSH:
		it.Note = "no ssh client — press enter for the command"
		it.Ready = true
	default:
		it.Note = "server unreachable"
	}
	return it
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd {
	// Ask the terminal what colour it is. Everything renders dark until the
	// answer arrives, and the same mechanism is what a server session uses to
	// learn about a client's terminal.
	return tea.Batch(tea.RequestBackgroundColor, m.checkServer())
}

func (m Model) checkServer() tea.Cmd {
	addr := m.settings.Server
	return func() tea.Msg {
		return checkedMsg{status: online.Check(addr, online.CheckTimeout)}
	}
}

// Update satisfies tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	return next, cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {

	// 1. Size goes to every screen, not just the visible one: a game resized
	// only when focused would render at a stale width on its first frame.
	case tea.WindowSizeMsg:
		return m.resize(msg)

	// 2. The terminal's background decides the palette.
	case tea.BackgroundColorMsg:
		return m.retheme(msg.IsDark())

	// 3. Global keys, before the active screen sees them.
	case tea.KeyPressMsg:
		if handled, next, cmd := m.globalKey(msg); handled {
			return next, cmd
		}
		return m.forward(msg)

	case NavigateMsg:
		return m.switchTo(msg.To)

	case games.ToastMsg:
		return m, m.toast.Show(msg.Text, msg.Level)

	case ui.ToastExpiredMsg:
		m.toast.Expire(msg)
		return m, nil

	case games.ToggleGlyphsMsg:
		return m.toggleGlyphs()

	case checkedMsg:
		m.server = msg.status
		m.menu.SetItem("online", m.onlineItem())
		return m, nil

	case online.DoneMsg:
		// Back from the server: the terminal is ours again, and the local
		// wallet is untouched — the two balances never mix.
		if msg.Err != nil {
			return m, games.Toast("online session ended: "+msg.Err.Error(), ui.Bad)
		}
		return m, m.checkServer()

	case savedMsg:
		if msg.err != nil {
			return m, games.Toast("could not save: "+msg.err.Error(), ui.Bad)
		}
		return m, nil
	}

	return m.forward(msg)
}

func (m Model) resize(msg tea.WindowSizeMsg) (Model, tea.Cmd) {
	m.width, m.height = msg.Width, msg.Height
	m.help.SetWidth(msg.Width)

	bodyW := msg.Width
	bodyH := maxInt(msg.Height-chromeHeight, 1)

	m.menu.SetSize(bodyW, maxInt(bodyH-menuHeaderHeight, 3))
	m.statsTable.SetWidth(minInt(bodyW-4, statsWidth()))
	m.statsTable.SetHeight(maxInt(minInt(bodyH-6, 12), 3))
	m.helpVP.SetWidth(maxInt(bodyW-4, 20))
	m.helpVP.SetHeight(maxInt(bodyH-2, 3))
	m.helpVP.SetContent(m.helpContent())

	for s, g := range m.games {
		m.games[s] = g.SetSize(bodyW, bodyH)
	}
	return m, nil
}

func (m Model) retheme(isDark bool) (Model, tea.Cmd) {
	m.theme = ui.NewTheme(isDark)
	m.menu = m.menu.SetTheme(m.theme)
	m.statsTable.SetStyles(statsStyles(m.theme))
	m.statsTable.SetRows(m.statsRows())
	m.helpVP.SetContent(m.helpContent())
	for s, g := range m.games {
		m.games[s] = g.SetTheme(m.theme)
	}
	return m, nil
}

// globalKey handles the bindings that work on every screen, reporting whether
// it consumed the key.
func (m Model) globalKey(msg tea.KeyPressMsg) (bool, Model, tea.Cmd) {
	// A game in a modal state — a bet being typed, an overlay open — owns the
	// keyboard until it says otherwise. This is checked before quitting so
	// that typing a "q" into a bet field does not close the program.
	game, atTable := m.games[m.screen]
	if atTable && game.Modal() {
		return false, m, nil
	}

	if key.Matches(msg, m.keys.Quit) {
		// ctrl+c always quits. q quits too, but not with a round in progress,
		// where it is far more likely to be a mis-hit than an instruction.
		if msg.String() == "q" && atTable && game.Busy() {
			return true, m, games.Toast("finish the round first", ui.Info)
		}
		return true, m, tea.Quit
	}

	if m.screen == ScreenBankrupt {
		return m.bankruptKey(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Help):
		next, cmd := m.toggleScreen(ScreenHelp)
		return true, next, cmd

	case key.Matches(msg, m.keys.Stats):
		next, cmd := m.toggleScreen(ScreenStats)
		return true, next, cmd

	case key.Matches(msg, m.keys.Back):
		if m.screen == ScreenMenu {
			return true, m, nil
		}
		if atTable && game.Busy() {
			// Leaving mid-round would strand a stake, so the door stays shut
			// until the table is idle.
			return true, m, games.Toast("finish the round first", ui.Info)
		}
		next, cmd := m.switchTo(ScreenMenu)
		return true, next, cmd

	case key.Matches(msg, m.keys.Confirm):
		if m.screen != ScreenMenu {
			return false, m, nil
		}
		next, cmd := m.openSelected()
		return true, next, cmd
	}
	return false, m, nil
}

func (m Model) toggleScreen(s Screen) (Model, tea.Cmd) {
	if m.screen == s {
		return m.switchTo(m.prev)
	}
	return m.switchTo(s)
}

// openSelected acts on the menu row under the cursor.
func (m Model) openSelected() (Model, tea.Cmd) {
	it, ok := m.menu.Selected()
	if !ok {
		return m, nil
	}

	if it.Key == "online" {
		return m.goOnline()
	}

	target, ok := map[string]Screen{
		"slots":      ScreenSlots,
		"blackjack":  ScreenBlackjack,
		"roulette":   ScreenRoulette,
		"videopoker": ScreenVideoPoker,
		"stats":      ScreenStats,
		"help":       ScreenHelp,
	}[it.Key]
	if !ok {
		return m, nil
	}
	return m.switchTo(target)
}

// goOnline hands the terminal to ssh, or explains why it cannot.
func (m Model) goOnline() (Model, tea.Cmd) {
	if !m.server.HasSSH {
		return m, games.Toast("no ssh here — run: "+online.Command(m.settings.Server, m.settings.Nick), ui.Info)
	}
	if !m.server.Reachable {
		return m, tea.Batch(
			games.Toast(m.settings.Server+" is not answering", ui.Bad),
			m.checkServer(),
		)
	}
	if err := online.EnsureKey(); err != nil {
		return m, games.Toast("could not prepare the key: "+err.Error(), ui.Bad)
	}
	return m, online.Launch(m.settings.Server, m.settings.Nick)
}

func (m Model) bankruptKey(msg tea.KeyPressMsg) (bool, Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		m.ledger.Bailout()
		next, cmd := m.switchTo(ScreenMenu)
		return true, next, tea.Batch(cmd, next.save(),
			games.Toast("staked again with "+ui.Credits(m.balance()), ui.Good))
	case "n", "N", "esc":
		next, cmd := m.switchTo(ScreenMenu)
		return true, next, cmd
	}
	return true, m, nil
}

// switchTo changes screen, resetting the game being left behind.
func (m Model) switchTo(s Screen) (Model, tea.Cmd) {
	if s == m.screen {
		return m, nil
	}

	// A broke player can read the statistics but not sit down at a table.
	if isGame(s) && m.ledger.Broke() {
		s = ScreenBankrupt
	}

	if g, ok := m.games[m.screen]; ok {
		m.games[m.screen] = g.Reset()
	}

	// Help and statistics are overlays: leaving one goes back to the table,
	// not to whichever overlay was open before it.
	if !isOverlay(m.screen) {
		m.prev = m.screen
	}
	m.screen = s

	switch s {
	case ScreenStats:
		m.statsTable.SetRows(m.statsRows())
	case ScreenHelp:
		m.helpVP.SetContent(m.helpContent())
		m.helpVP.GotoTop()
	}

	var cmd tea.Cmd
	if g, ok := m.games[s]; ok {
		cmd = g.Init()
	}
	return m, cmd
}

func (m Model) toggleGlyphs() (Model, tea.Cmd) {
	if m.settings.Glyphs == games.GlyphsEmoji {
		m.settings.Glyphs = games.GlyphsASCII
	} else {
		m.settings.Glyphs = games.GlyphsEmoji
	}

	msg := games.GlyphsMsg{Glyphs: m.settings.Glyphs}
	var cmds []tea.Cmd
	for s, g := range m.games {
		ng, cmd := g.Update(msg)
		m.games[s] = ng
		cmds = append(cmds, cmd)
	}

	settings := m.settings
	cmds = append(cmds,
		func() tea.Msg { return savedMsg{err: settings.Save()} },
		games.Toast("symbols: "+m.settings.Glyphs, ui.Info),
	)
	return m, tea.Batch(cmds...)
}

// forward hands a message to the active screen.
func (m Model) forward(msg tea.Msg) (Model, tea.Cmd) {
	switch m.screen {
	case ScreenMenu:
		var cmd tea.Cmd
		m.menu, cmd = m.menu.Update(msg)
		return m, cmd

	case ScreenStats:
		var cmd tea.Cmd
		m.statsTable, cmd = m.statsTable.Update(msg)
		return m, cmd

	case ScreenHelp:
		var cmd tea.Cmd
		m.helpVP, cmd = m.helpVP.Update(msg)
		return m, cmd

	default:
		g, ok := m.games[m.screen]
		if !ok {
			return m, nil
		}
		ng, cmd := g.Update(msg)
		m.games[m.screen] = ng

		// Losing the last credit sends the player to the bailout rather than
		// leaving them at a table they cannot bet at.
		if m.ledger.Broke() && !ng.Busy() {
			return m, tea.Batch(cmd, Navigate(ScreenBankrupt))
		}
		return m, cmd
	}
}

func (m Model) balance() int64 { return m.ledger.Balance(m.me) }

func (m Model) save() tea.Cmd {
	l := m.ledger
	return func() tea.Msg { return savedMsg{err: l.Save()} }
}

// Persist writes the wallet, which main calls once more on the way out.
func (m Model) Persist() error { return m.ledger.Save() }

func (m Model) tooSmall() bool {
	return m.width > 0 && (m.width < minWidth || m.height < minHeight)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
