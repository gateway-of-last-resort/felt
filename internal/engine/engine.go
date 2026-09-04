// Package engine holds the game rules as pure state machines.
//
// Nothing here imports Bubble Tea, knows about wallets, or reads the clock:
// time arrives as a parameter. That is what lets the same engine run inside a
// local process and inside a server room, and what lets the rules be tested
// without a terminal or a bank.
package engine

import (
	"errors"
	"time"
)

// PlayerID identifies a player. Locally it is always LocalPlayer; on a server
// it is the SHA256 fingerprint of the player's SSH key.
type PlayerID string

// LocalPlayer is the only player in a single-process game.
const LocalPlayer PlayerID = "local"

// Seat is a position at a table, counted from zero. DealerSeat marks events
// that belong to the house rather than to a player.
type Seat int

// DealerSeat is the seat number used for the dealer's own cards.
const DealerSeat Seat = -1

// Kind names a game, and doubles as the statistics key, so the values are
// part of the save format and must stay stable.
type Kind string

// The games.
const (
	KindSlots     Kind = "slots"
	KindBlackjack Kind = "blackjack"
	KindRoulette  Kind = "roulette"
)

// Errors an engine may return from Apply.
var (
	ErrNotYourTurn  = errors.New("not your turn")
	ErrWrongPhase   = errors.New("not now")
	ErrNotAllowed   = errors.New("not allowed")
	ErrTableFull    = errors.New("table is full")
	ErrUnknownSeat  = errors.New("no seat at this table")
	ErrInvalidBet   = errors.New("invalid bet")
	ErrNothingToBet = errors.New("no bets placed")
)

// Action is something a player asks the table to do. Placing a bet is an
// action like any other: the engine records the amount, and whoever owns the
// money decides beforehand whether it can be staked.
type Action interface {
	Player() PlayerID
}

// Event is something that happened at the table. Events are the only way a
// table reports change, so a room can broadcast exactly what it applied.
//
// The marker method is exported because the implementations live in other
// packages: an unexported one could only ever be satisfied inside this one.
type Event interface {
	Event()
}

// Stake is implemented by an action that puts money at risk. The driver or
// room debits Stake before applying the action and refuses it if the money is
// not there — which is why the engine itself never has to ask.
type Stake interface {
	Action
	Stake() int64
}

// Settlement is implemented by an event that returns money to a player.
// Whoever owns the ledger credits Won and records Wagered against Game.
type Settlement interface {
	Event
	Payee() PlayerID
	Result() (wagered, won int64)
	Game() Kind
}

// Refund is implemented by an event that hands a stake back without the
// round having been played — a chip lifted off the layout before the wheel
// turns. It is deliberately separate from Settlement: a refund must not be
// recorded as turnover, or picking a chip up and putting it down again would
// inflate the statistics.
type Refund interface {
	Event
	Payee() PlayerID
	Refund() int64
}

// Game is one table. Implementations are slots.Table, blackjack.Table and
// roulette.Table.
type Game interface {
	// Kind names the game.
	Kind() Kind

	// Join seats a player, returning the seat they already hold if they are
	// reconnecting.
	Join(p PlayerID) (Seat, error)

	// Leave releases a seat and reports what that changed.
	Leave(p PlayerID) []Event

	// Apply runs one action against the rules.
	Apply(a Action, now time.Time) ([]Event, error)

	// Tick advances anything driven by a deadline — closing the betting
	// window, standing a player who ran out of time — and reports what
	// happened. It is safe to call at any time.
	Tick(now time.Time) []Event

	// Deadline is when Tick next needs calling, or false when the table is
	// waiting on a player rather than a clock. Locally most tables have no
	// deadlines at all.
	Deadline() (time.Time, bool)

	// Snapshot is everything the given viewer may see. Hidden cards are
	// hidden per viewer, so a snapshot can be sent straight to one player.
	Snapshot(viewer PlayerID) any
}
