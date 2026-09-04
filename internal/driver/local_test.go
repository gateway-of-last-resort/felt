package driver

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gateway-of-last-resort/felt/internal/bank"
	"github.com/gateway-of-last-resort/felt/internal/engine"
)

const me = engine.LocalPlayer

// fakeGame is a table that does whatever the test tells it to, so the driver
// can be examined on its own: what it debits, when, and what it gives back.
type fakeGame struct {
	err     error
	events  []engine.Event
	applied []engine.Action
	ticks   []engine.Event

	// joinErr and buyIns record how the table was sat down at, which is what
	// the buy-in path is about.
	joinErr     error
	buyIns      []int64
	leaveEvents []engine.Event
}

func (g *fakeGame) Kind() engine.Kind { return engine.KindSlots }
func (g *fakeGame) Join(_ engine.PlayerID, buyIn int64) (engine.Seat, error) {
	g.buyIns = append(g.buyIns, buyIn)
	return 0, g.joinErr
}
func (g *fakeGame) Leave(engine.PlayerID) []engine.Event { return g.leaveEvents }
func (g *fakeGame) Deadline() (time.Time, bool)          { return time.Time{}, false }
func (g *fakeGame) Snapshot(engine.PlayerID) any         { return "snapshot" }

func (g *fakeGame) Tick(time.Time) []engine.Event { return g.ticks }

func (g *fakeGame) Apply(a engine.Action, _ time.Time) ([]engine.Event, error) {
	g.applied = append(g.applied, a)
	if g.err != nil {
		return nil, g.err
	}
	return g.events, nil
}

// stakeAction costs money.
type stakeAction struct{ amount int64 }

func (stakeAction) Player() engine.PlayerID { return me }
func (a stakeAction) Stake() int64          { return a.amount }

// freeAction costs nothing.
type freeAction struct{}

func (freeAction) Player() engine.PlayerID { return me }

// settlement returns money and files turnover.
type settlement struct{ wagered, won int64 }

func (settlement) Event()                   {}
func (settlement) Payee() engine.PlayerID   { return me }
func (s settlement) Result() (int64, int64) { return s.wagered, s.won }
func (settlement) Game() engine.Kind        { return engine.KindSlots }

// refund returns money without it being a result.
type refund struct{ amount int64 }

func (refund) Event()                 {}
func (refund) Payee() engine.PlayerID { return me }
func (r refund) Refund() int64        { return r.amount }

func testLedger(t *testing.T) *bank.JSONLedger {
	t.Helper()
	l, err := bank.OpenJSON(filepath.Join(t.TempDir(), "save.json"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// run executes the command a driver returned and gives back the message.
func run(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("driver returned no command")
	}
	return cmd()
}

// The stake leaves the wallet before the engine ever sees the action: a table
// must not move on money that was never there.
func TestStakeIsTakenBeforeApply(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{events: []engine.Event{settlement{wagered: 25, won: 0}}}
	d := NewLocal(g, l, me, nil)

	start := l.Balance(me)
	msg := run(t, d.Do(stakeAction{amount: 25}))

	if _, ok := msg.(EventsMsg); !ok {
		t.Fatalf("got %T, want EventsMsg", msg)
	}
	if got, want := l.Balance(me), start-25; got != want {
		t.Errorf("balance = %d, want %d", got, want)
	}
	if len(g.applied) != 1 {
		t.Errorf("engine saw %d actions, want 1", len(g.applied))
	}
}

// A stake beyond the balance never reaches the engine at all.
func TestUnaffordableStakeNeverReachesTheEngine(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{}
	d := NewLocal(g, l, me, nil)

	start := l.Balance(me)
	msg := run(t, d.Do(stakeAction{amount: start + 1}))

	errMsg, ok := msg.(ErrorMsg)
	if !ok {
		t.Fatalf("got %T, want ErrorMsg", msg)
	}
	if !errors.Is(errMsg.Err, bank.ErrInsufficientFunds) {
		t.Errorf("error = %v, want ErrInsufficientFunds", errMsg.Err)
	}
	if len(g.applied) != 0 {
		t.Error("the engine was asked to act on money that was not there")
	}
	if got := l.Balance(me); got != start {
		t.Errorf("balance = %d after a refused stake, want %d", got, start)
	}
}

// If the engine refuses after the money was taken, the money comes back.
// Without this, a rejected double would quietly cost the player their stake.
func TestRefusedActionRefundsTheStake(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{err: engine.ErrNotAllowed}
	d := NewLocal(g, l, me, nil)

	start := l.Balance(me)
	msg := run(t, d.Do(stakeAction{amount: 40}))

	if _, ok := msg.(ErrorMsg); !ok {
		t.Fatalf("got %T, want ErrorMsg", msg)
	}
	if got := l.Balance(me); got != start {
		t.Errorf("balance = %d after the engine refused, want the stake back at %d", got, start)
	}
}

// A settlement credits the winnings and records the turnover.
func TestSettlementPaysAndRecords(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{events: []engine.Event{settlement{wagered: 10, won: 30}}}
	d := NewLocal(g, l, me, nil)

	start := l.Balance(me)
	msg := run(t, d.Do(stakeAction{amount: 10}))

	events, ok := msg.(EventsMsg)
	if !ok {
		t.Fatalf("got %T, want EventsMsg", msg)
	}
	if want := start - 10 + 30; events.Balance != want {
		t.Errorf("reported balance %d, want %d", events.Balance, want)
	}
	if got := l.Balance(me); got != start-10+30 {
		t.Errorf("wallet holds %d, want %d", got, start-10+30)
	}

	s := l.Stats(me).Get(engine.KindSlots)
	if s.Rounds != 1 || s.Wagered != 10 || s.Won != 30 {
		t.Errorf("stats = %+v, want one round of 10 for 30", s)
	}
}

// A refund gives money back without counting as a played round — otherwise
// putting a chip down and picking it up again would inflate the turnover.
func TestRefundIsNotTurnover(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{events: []engine.Event{refund{amount: 15}}}
	d := NewLocal(g, l, me, nil)

	start := l.Balance(me)
	run(t, d.Do(freeAction{}))

	if got, want := l.Balance(me), start+15; got != want {
		t.Errorf("balance = %d, want %d", got, want)
	}
	if s := l.Stats(me); s.Rounds() != 0 || s.Wagered() != 0 {
		t.Errorf("a refund was recorded as play: %+v", s)
	}
}

// The wallet is saved once a round settles, and not on every action.
func TestSaveHookRunsOnSettlement(t *testing.T) {
	l := testLedger(t)
	saves := 0

	g := &fakeGame{}
	d := NewLocal(g, l, me, func() { saves++ })

	run(t, d.Do(freeAction{}))
	if saves != 0 {
		t.Errorf("saved %d times on an action that settled nothing", saves)
	}

	g.events = []engine.Event{settlement{wagered: 5, won: 5}}
	run(t, d.Do(stakeAction{amount: 5}))
	if saves != 1 {
		t.Errorf("saved %d times after a settled round, want 1", saves)
	}
}

// An action that costs nothing is applied without touching the wallet.
func TestFreeActionDoesNotTouchTheWallet(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{}
	d := NewLocal(g, l, me, nil)

	start := l.Balance(me)
	run(t, d.Do(freeAction{}))

	if got := l.Balance(me); got != start {
		t.Errorf("balance = %d after a free action, want %d", got, start)
	}
	if len(g.applied) != 1 {
		t.Error("the free action never reached the engine")
	}
}

// Ticks settle the same way actions do, which is how a timed table pays out
// when nobody pressed anything.
func TestTickSettles(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{ticks: []engine.Event{settlement{wagered: 10, won: 20}}}
	d := NewLocal(g, l, me, nil)

	start := l.Balance(me)
	msg := run(t, d.Tick(time.Now()))

	if _, ok := msg.(EventsMsg); !ok {
		t.Fatalf("got %T, want EventsMsg", msg)
	}
	if got, want := l.Balance(me), start+20; got != want {
		t.Errorf("balance = %d, want %d", got, want)
	}
}

// Event picks one kind of event out of a batch, which is how a screen finds
// the thing it animates.
func TestEventFindsByType(t *testing.T) {
	events := []engine.Event{refund{amount: 1}, settlement{wagered: 2, won: 3}}

	s, ok := Event[settlement](events)
	if !ok {
		t.Fatal("Event did not find the settlement")
	}
	if s.won != 3 {
		t.Errorf("found %+v, want the settlement worth 3", s)
	}
	if _, ok := Event[stakeEventNotPresent](events); ok {
		t.Error("Event invented an event that was not there")
	}
}

type stakeEventNotPresent struct{}

func (stakeEventNotPresent) Event() {}

// An action that costs nothing is applied, not refused.
//
// Declining insurance is exactly this: it carries a stake of zero. Treating
// that as an invalid bet left blackjack unable to decline, so a round with a
// dealer ace could never move past the insurance prompt.
func TestZeroStakeIsAppliedNotRefused(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{}
	d := NewLocal(g, l, me, nil)

	start := l.Balance(me)
	msg := run(t, d.Do(stakeAction{amount: 0}))

	if err, ok := msg.(ErrorMsg); ok {
		t.Fatalf("a free action was refused with %v", err.Err)
	}
	if len(g.applied) != 1 {
		t.Error("the free action never reached the engine")
	}
	if got := l.Balance(me); got != start {
		t.Errorf("balance = %d after a free action, want %d", got, start)
	}
}

// A negative stake is still nonsense and is refused.
func TestNegativeStakeRefused(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{}
	d := NewLocal(g, l, me, nil)

	msg := run(t, d.Do(stakeAction{amount: -5}))
	if _, ok := msg.(ErrorMsg); !ok {
		t.Fatalf("got %T, want a negative stake to be refused", msg)
	}
	if len(g.applied) != 0 {
		t.Error("a negative stake reached the engine")
	}
}

// A buy-in is charged once, on sitting down, and handed to the table.
//
// This is the second way money works here: a table with stacks takes a stake
// at the door and then moves chips around by itself, rather than charging for
// every bet. Offline nothing uses it yet — the path exists so that holdem is
// new files rather than a change to this one.
func TestBuyInIsChargedOnJoin(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{}
	d := NewLocal(g, l, me, nil)

	start := l.Balance(me)
	if err := d.Join(200); err != nil {
		t.Fatalf("Join: %v", err)
	}

	if got, want := l.Balance(me), start-200; got != want {
		t.Errorf("balance = %d after buying in, want %d", got, want)
	}
	if len(g.buyIns) != 1 || g.buyIns[0] != 200 {
		t.Errorf("the table was told about buy-ins %v, want one of 200", g.buyIns)
	}

	// And it is not turnover: nothing has been played yet.
	if s := l.Stats(me); s.Rounds() != 0 {
		t.Errorf("buying in recorded %d rounds of play", s.Rounds())
	}
}

// Sitting down at a table that bets from the wallet costs nothing.
func TestFreeSeatCostsNothing(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{}
	d := NewLocal(g, l, me, nil)

	start := l.Balance(me)
	if err := d.Join(0); err != nil {
		t.Fatalf("Join: %v", err)
	}
	if got := l.Balance(me); got != start {
		t.Errorf("balance = %d after taking a free seat, want %d", got, start)
	}
	if len(g.buyIns) != 1 || g.buyIns[0] != 0 {
		t.Errorf("the table was told about buy-ins %v, want one of 0", g.buyIns)
	}
}

// A buy-in beyond the balance leaves the player standing, and their money
// alone.
func TestUnaffordableBuyIn(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{}
	d := NewLocal(g, l, me, nil)

	start := l.Balance(me)
	if err := d.Join(start + 1); err == nil {
		t.Fatal("the player sat down with money they did not have")
	}
	if got := l.Balance(me); got != start {
		t.Errorf("balance = %d after a refused buy-in, want %d", got, start)
	}
	if len(g.buyIns) != 0 {
		t.Error("the table was told about a buy-in that was never paid")
	}
}

// If the table turns the player away, the buy-in comes back. Without this a
// full table would quietly keep the money.
func TestRefusedSeatRefundsTheBuyIn(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{joinErr: engine.ErrTableFull}
	d := NewLocal(g, l, me, nil)

	start := l.Balance(me)
	if err := d.Join(200); err == nil {
		t.Fatal("Join reported success at a full table")
	}
	if got := l.Balance(me); got != start {
		t.Errorf("balance = %d after being turned away, want the buy-in back at %d", got, start)
	}
}

// Whatever is left of a stack comes back as a refund when the player leaves,
// which is the other half of the buy-in.
func TestStackComesBackOnLeave(t *testing.T) {
	l := testLedger(t)
	g := &fakeGame{}
	d := NewLocal(g, l, me, nil)

	if err := d.Join(200); err != nil {
		t.Fatal(err)
	}
	before := l.Balance(me)

	// A table with stacks reports what is left as a refund.
	g.leaveEvents = []engine.Event{refund{amount: 260}}
	run(t, d.Leave())

	if got, want := l.Balance(me), before+260; got != want {
		t.Errorf("balance = %d after leaving with 260 in front of us, want %d", got, want)
	}
	// A stack coming home is not a played round.
	if s := l.Stats(me); s.Rounds() != 0 {
		t.Errorf("returning a stack recorded %d rounds", s.Rounds())
	}
}
