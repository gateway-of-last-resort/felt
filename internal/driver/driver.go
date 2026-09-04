// Package driver connects a presentation model to a game engine.
//
// Locally the driver holds the engine and the ledger in this process. Online
// it will be a channel into a server room. The presentation cannot tell the
// difference: it sends actions and receives EventsMsg either way, which is
// what keeps the whole online step out of the game screens.
package driver

import (
	tea "charm.land/bubbletea/v2"
	"github.com/gateway-of-last-resort/felt/internal/engine"
)

// Driver is the presentation's only route to a game.
type Driver interface {
	// Do submits an action. The outcome arrives as an EventsMsg or an
	// ErrorMsg — never as a return value, because online there is nothing to
	// return yet.
	Do(a engine.Action) tea.Cmd

	// Leave releases the seat.
	Leave() tea.Cmd

	// Me is the player this driver acts for.
	Me() engine.PlayerID
}

// EventsMsg carries what happened, the resulting view, and the balance the
// player now has. The three travel together because a room broadcasts them
// together and the screen needs all three to be consistent.
type EventsMsg struct {
	Events   []engine.Event
	Snapshot any
	Balance  int64
}

// ErrorMsg reports an action that was refused — a bet beyond the balance, an
// action out of turn. It goes only to the player who asked.
type ErrorMsg struct{ Err error }

// Event returns the first event of the given type from a batch, which is how
// a presentation picks out the one thing it animates.
func Event[T engine.Event](msgs []engine.Event) (T, bool) {
	for _, e := range msgs {
		if t, ok := e.(T); ok {
			return t, true
		}
	}
	var zero T
	return zero, false
}
