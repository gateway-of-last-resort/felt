package driver

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gateway-of-last-resort/felt/internal/bank"
	"github.com/gateway-of-last-resort/felt/internal/engine"
)

// Local runs a game in this process against the local wallet.
//
// It is the only place offline where money moves: a stake is debited before
// the action reaches the engine, and a settlement credits what comes back.
// The engine itself never sees a balance.
type Local struct {
	game   engine.Game
	ledger bank.Ledger
	me     engine.PlayerID

	// onSettle is called after a round settles, so the client can persist the
	// wallet without the driver knowing what persistence means.
	onSettle func()
}

// NewLocal seats a player at a game.
func NewLocal(g engine.Game, l bank.Ledger, me engine.PlayerID, onSettle func()) *Local {
	_, _ = g.Join(me)
	return &Local{game: g, ledger: l, me: me, onSettle: onSettle}
}

// Me satisfies Driver.
func (d *Local) Me() engine.PlayerID { return d.me }

// Do satisfies Driver: debit, apply, credit, report.
func (d *Local) Do(a engine.Action) tea.Cmd {
	now := time.Now()

	// Take the stake first. An action that cannot be paid for must not reach
	// the engine at all, or the table would move without the money.
	staked := int64(0)
	if s, ok := a.(engine.Stake); ok {
		amount := s.Stake()

		// A stake of zero is not a bad bet, it is an action that happens to
		// cost nothing — declining insurance is the one that matters. Only a
		// negative stake is nonsense.
		if amount < 0 {
			return d.fail(engine.ErrInvalidBet)
		}
		if amount > 0 {
			if err := d.ledger.Debit(d.me, amount); err != nil {
				return d.fail(err)
			}
			staked = amount
		}
	}

	events, err := d.game.Apply(a, now)
	if err != nil {
		// The engine refused after the money was taken, so give it back.
		if staked > 0 {
			_ = d.ledger.Credit(d.me, staked)
		}
		return d.fail(err)
	}

	return d.settle(events)
}

// Tick advances the engine's clock and reports anything it produced. The
// presentation calls it when the driver asks; offline most tables never do.
func (d *Local) Tick(now time.Time) tea.Cmd {
	events := d.game.Tick(now)
	if len(events) == 0 {
		return nil
	}
	return d.settle(events)
}

// Deadline is when Tick next needs calling.
func (d *Local) Deadline() (time.Time, bool) { return d.game.Deadline() }

// settle credits every settlement in a batch and packages the result.
func (d *Local) settle(events []engine.Event) tea.Cmd {
	settled := false
	for _, e := range events {
		// A refund is money coming back off the table, not a result: it is
		// credited but never recorded as turnover.
		if r, ok := e.(engine.Refund); ok {
			if n := r.Refund(); n > 0 {
				_ = d.ledger.Credit(r.Payee(), n)
			}
			continue
		}

		s, ok := e.(engine.Settlement)
		if !ok {
			continue
		}
		wagered, won := s.Result()
		if won > 0 {
			_ = d.ledger.Credit(s.Payee(), won)
		}
		d.ledger.Record(s.Payee(), s.Game(), wagered, won)
		settled = true
	}
	if settled && d.onSettle != nil {
		d.onSettle()
	}

	msg := EventsMsg{
		Events:   events,
		Snapshot: d.game.Snapshot(d.me),
		Balance:  d.ledger.Balance(d.me),
	}
	return func() tea.Msg { return msg }
}

// Leave satisfies Driver.
func (d *Local) Leave() tea.Cmd {
	events := d.game.Leave(d.me)
	if len(events) == 0 {
		return nil
	}
	return d.settle(events)
}

// Game exposes the engine, for the few presentation conveniences that need
// to ask it something rather than act on it — repeating last round's bets,
// for one. It is deliberately not part of the Driver interface: a screen that
// reached for it online would be reaching across the network.
func (d *Local) Game() engine.Game { return d.game }

// Snapshot is the current view, for a screen that is being opened rather than
// reacting to an action.
func (d *Local) Snapshot() any { return d.game.Snapshot(d.me) }

// Balance is the player's current balance.
func (d *Local) Balance() int64 { return d.ledger.Balance(d.me) }

func (d *Local) fail(err error) tea.Cmd {
	return func() tea.Msg { return ErrorMsg{Err: err} }
}
