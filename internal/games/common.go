// Package games holds the presentation layer: the models that draw a table,
// run its animations and turn keystrokes into engine actions.
//
// Nothing here decides an outcome. A game screen asks the driver to apply an
// action and then animates towards the result it is handed — the reels catch
// up with a spin that has already happened. That split is what lets the same
// screen draw a local table and one running on a server.
package games

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

// Options tune the presentation for where it is running.
//
// Locally the animations run at 30 frames a second. Over SSH the same code
// runs at 12, which is the only difference between a local table and a server
// one — and it arrives as a parameter rather than an `if server` branch.
type Options struct {
	FPS        int
	Animations bool
}

// LocalOptions are the defaults for a table in this process.
func LocalOptions() Options { return Options{FPS: 30, Animations: true} }

// Frame is the duration of one animation frame.
func (o Options) Frame() time.Duration {
	fps := o.FPS
	if fps <= 0 {
		fps = 30
	}
	return time.Second / time.Duration(fps)
}

// Compact reports whether a screen has to tighten its layout: cards on one
// line, a narrower roulette field. One definition, so the games cannot
// disagree about where the threshold is.
func Compact(w, h int) bool { return w < 100 || h < 30 }

// Game is what the root needs from a table screen.
type Game interface {
	Init() tea.Cmd

	// Update returns the game rather than a tea.Model so the root can put it
	// straight back into its map without a type assertion.
	Update(msg tea.Msg) (Game, tea.Cmd)

	View() string

	// Title names the game in the top bar.
	Title() string

	// Help lists the bindings that apply right now.
	Help() []key.Binding

	// Busy reports that the table is mid-round or mid-animation. The root
	// refuses to leave a busy game, so a stake cannot be abandoned.
	Busy() bool

	// Modal reports that the screen owns the whole keyboard — a text field, an
	// overlay it must close itself. While it does, the root stops
	// intercepting global keys.
	Modal() bool

	// Stake is what the player has at risk, for the top bar.
	Stake() int64

	// Reset returns the game to its idle state when the player leaves.
	Reset() Game

	// SetTheme rebuilds styles after the terminal reports its background.
	SetTheme(t ui.Theme) Game

	// SetSize gives the game the area the root has left it.
	SetSize(w, h int) Game
}

// ToastMsg asks the root to show a notification. It lives here rather than in
// the root package because the games raise them and the root imports the
// games, not the other way round.
type ToastMsg struct {
	Text  string
	Level ui.Level
}

// Toast returns a command raising a notification.
func Toast(text string, lvl ui.Level) tea.Cmd {
	return func() tea.Msg { return ToastMsg{Text: text, Level: lvl} }
}

// ToggleGlyphsMsg asks the root to switch the slot symbol set. A game cannot
// change a setting itself: preferences live in the settings file, which the
// root owns, so the request goes up and the new value comes back down.
type ToggleGlyphsMsg struct{}

// GlyphsMsg carries the symbol set down to the games.
type GlyphsMsg struct{ Glyphs string }

// Glyph set names, as stored in the settings file.
const (
	GlyphsASCII = "ascii"
	GlyphsEmoji = "emoji"
)
