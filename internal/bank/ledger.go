// Package bank owns the money. Everything that can change a balance goes
// through a Ledger, and nothing else in the program is allowed to.
//
// Credits are always int64. Blackjack pays 3:2 and insurance costs half a
// stake, so blackjack bets are kept even rather than rounding in someone's
// favour.
package bank

import (
	"errors"
	"time"

	"github.com/gateway-of-last-resort/felt/internal/engine"
)

// ErrInsufficientFunds is returned when a debit is larger than the balance.
var ErrInsufficientFunds = errors.New("insufficient funds")

// Ledger holds balances and the statistics derived from play.
//
// The single-player JSONLedger ignores the PlayerID; the server's BoltLedger
// keys everything by it. Both satisfy this interface, which is why nothing
// above the bank has to know which one it is talking to.
type Ledger interface {
	Balance(p engine.PlayerID) int64
	Debit(p engine.PlayerID, n int64) error
	Credit(p engine.PlayerID, n int64) error

	// Record files a settled round: what was put at risk and what came back.
	Record(p engine.PlayerID, game engine.Kind, wagered, won int64)

	Stats(p engine.PlayerID) Stats
}

// GameStats is the running tally for one game.
type GameStats struct {
	Rounds  int   `json:"rounds"`
	Wagered int64 `json:"wagered"`
	Won     int64 `json:"won"`
	Best    int64 `json:"best"`
}

// Stats aggregates play across games. The zero value is usable.
type Stats struct {
	ByGame  map[engine.Kind]*GameStats `json:"by_game"`
	Started time.Time                  `json:"started"`
}

// NewStats returns statistics stamped as starting now.
func NewStats(now time.Time) Stats {
	return Stats{ByGame: map[engine.Kind]*GameStats{}, Started: now}
}

// Record adds one settled round. A losing round records won = 0 and a push
// records won equal to wagered, so the turnover is right either way.
func (s *Stats) Record(game engine.Kind, wagered, won int64) {
	if s.ByGame == nil {
		s.ByGame = map[engine.Kind]*GameStats{}
	}
	g, ok := s.ByGame[game]
	if !ok {
		g = &GameStats{}
		s.ByGame[game] = g
	}
	g.Rounds++
	g.Wagered += wagered
	g.Won += won
	if net := won - wagered; net > g.Best {
		g.Best = net
	}
}

// Get returns the tally for a game, zero if it was never played.
func (s Stats) Get(game engine.Kind) GameStats {
	if g, ok := s.ByGame[game]; ok {
		return *g
	}
	return GameStats{}
}

// Rounds is the number of rounds played across all games.
func (s Stats) Rounds() int {
	var n int
	for _, g := range s.ByGame {
		n += g.Rounds
	}
	return n
}

// Wagered is the total turnover across all games.
func (s Stats) Wagered() int64 {
	var n int64
	for _, g := range s.ByGame {
		n += g.Wagered
	}
	return n
}

// Won is the total returned across all games.
func (s Stats) Won() int64 {
	var n int64
	for _, g := range s.ByGame {
		n += g.Won
	}
	return n
}

// Net is the player's overall result; positive means ahead.
func (s Stats) Net() int64 { return s.Won() - s.Wagered() }

// RTP is the realised return for one game as a fraction of what was staked,
// or 0 for a game that has never been played.
func (s Stats) RTP(game engine.Kind) float64 {
	g := s.Get(game)
	if g.Wagered == 0 {
		return 0
	}
	return float64(g.Won) / float64(g.Wagered)
}
