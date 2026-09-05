package video

import (
	"math/rand/v2"
	"time"

	"github.com/gateway-of-last-resort/felt/internal/deck"
	"github.com/gateway-of-last-resort/felt/internal/engine"
	"github.com/gateway-of-last-resort/felt/internal/engine/poker"
)

// Phase is where a hand has got to.
type Phase int

// The phases of a hand.
const (
	PhaseBetting Phase = iota // choosing a stake, no cards out
	PhaseDraw                 // five cards dealt, choosing what to hold
	PhaseResult               // the draw is in
)

// String names the phase.
func (p Phase) String() string {
	switch p {
	case PhaseDraw:
		return "draw"
	case PhaseResult:
		return "result"
	default:
		return "betting"
	}
}

// Actions.
type (
	// Deal stakes coins and puts five cards out.
	Deal struct {
		P     engine.PlayerID
		Coins int64
	}

	// Draw replaces the cards not held.
	Draw struct {
		P    engine.PlayerID
		Hold [5]bool
	}
)

// Player satisfies engine.Action.
func (a Deal) Player() engine.PlayerID { return a.P }
func (a Draw) Player() engine.PlayerID { return a.P }

// Stake satisfies engine.Stake: coins are credits, one for one.
func (a Deal) Stake() int64 { return a.Coins }

// Events.
type (
	// Dealt reports the opening five.
	Dealt struct {
		P     engine.PlayerID
		Cards [5]deck.Card
		Coins int64
	}

	// Drawn reports the final hand and what it paid. Replaced marks which
	// cards changed, so the screen can turn those over and leave the rest.
	Drawn struct {
		P        engine.PlayerID
		Cards    [5]deck.Card
		Replaced [5]bool
		Rank     poker.Rank
		Wagered  int64
		Payout   int64
	}
)

// Event satisfies engine.Event.
func (Dealt) Event() {}
func (Drawn) Event() {}

// Payee satisfies engine.Settlement.
func (d Drawn) Payee() engine.PlayerID { return d.P }

// Result satisfies engine.Settlement.
func (d Drawn) Result() (wagered, won int64) { return d.Wagered, d.Payout }

// Game satisfies engine.Settlement.
func (d Drawn) Game() engine.Kind { return engine.KindVideoPoker }

// Snapshot is the machine as the player sees it.
type Snapshot struct {
	Phase Phase
	Cards [5]deck.Card
	Held  [5]bool
	Coins int64

	// Last is the result of the hand just finished, kept until the next deal
	// so the player can read what they got.
	Last *Drawn

	Deadline time.Time
}

// Table is one video poker machine: one player, one deck per hand, no clock.
type Table struct {
	rng    *rand.Rand
	seated engine.PlayerID

	phase Phase
	cards [5]deck.Card
	held  [5]bool
	coins int64
	last  *Drawn

	// shoe is a single deck, reshuffled for every hand. Video poker deals
	// from a fresh deck each time, which is why counting cards across hands
	// means nothing here.
	shoe *deck.Shoe
}

// NewTable returns an idle machine.
func NewTable(r *rand.Rand) *Table {
	return &Table{
		rng:   r,
		phase: PhaseBetting,
		shoe:  deck.NewShoe(1, 1, r),
	}
}

// Kind satisfies engine.Game.
func (t *Table) Kind() engine.Kind { return engine.KindVideoPoker }

// Join satisfies engine.Game.
func (t *Table) Join(p engine.PlayerID, buyIn int64) (engine.Seat, error) {
	// Coins are staked hand by hand from the wallet, so there is nothing to
	// buy in with.
	if buyIn != 0 {
		return 0, engine.ErrNoStack
	}
	if t.seated != "" && t.seated != p {
		return 0, engine.ErrTableFull
	}
	t.seated = p
	return 0, nil
}

// Leave satisfies engine.Game.
func (t *Table) Leave(p engine.PlayerID) []engine.Event {
	if t.seated == p {
		t.seated = ""
	}
	return nil
}

// Deadline satisfies engine.Game: the machine waits for the player.
func (t *Table) Deadline() (time.Time, bool) { return time.Time{}, false }

// Tick satisfies engine.Game and does nothing, for the same reason.
func (t *Table) Tick(time.Time) []engine.Event { return nil }

// Apply satisfies engine.Game.
func (t *Table) Apply(a engine.Action, _ time.Time) ([]engine.Event, error) {
	switch act := a.(type) {
	case Deal:
		return t.deal(act)
	case Draw:
		return t.draw(act)
	default:
		return nil, engine.ErrNotAllowed
	}
}

func (t *Table) deal(a Deal) ([]engine.Event, error) {
	if t.phase == PhaseDraw {
		// There are cards on the table waiting to be drawn to.
		return nil, engine.ErrWrongPhase
	}
	if a.Coins < 1 || a.Coins > MaxCoins {
		return nil, engine.ErrInvalidBet
	}

	// Every hand comes off a full deck.
	t.shoe.Shuffle()
	for i := range t.cards {
		t.cards[i] = t.shoe.Draw()
	}
	t.held = [5]bool{}
	t.coins = a.Coins
	t.last = nil
	t.phase = PhaseDraw

	return []engine.Event{Dealt{P: a.P, Cards: t.cards, Coins: a.Coins}}, nil
}

func (t *Table) draw(a Draw) ([]engine.Event, error) {
	if t.phase != PhaseDraw {
		return nil, engine.ErrWrongPhase
	}

	var replaced [5]bool
	for i, keep := range a.Hold {
		if keep {
			continue
		}
		// The replacements come off the same deck the hand was dealt from, so
		// a card already in the player's hand cannot come back.
		t.cards[i] = t.shoe.Draw()
		replaced[i] = true
	}

	rank := poker.Eval5(t.cards)
	drawn := Drawn{
		P:        a.P,
		Cards:    t.cards,
		Replaced: replaced,
		Rank:     rank,
		Wagered:  t.coins,
		Payout:   Payout(rank, t.coins),
	}

	t.held = a.Hold
	t.last = &drawn
	t.phase = PhaseResult

	return []engine.Event{drawn}, nil
}

// Snapshot satisfies engine.Game.
func (t *Table) Snapshot(engine.PlayerID) any {
	return Snapshot{
		Phase: t.phase,
		Cards: t.cards,
		Held:  t.held,
		Coins: t.coins,
		Last:  t.last,
	}
}
