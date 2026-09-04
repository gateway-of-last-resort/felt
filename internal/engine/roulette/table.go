package roulette

import (
	"math/rand/v2"
	"time"

	"github.com/gateway-of-last-resort/felt/internal/engine"
)

// Phase is where a round has got to.
type Phase int

// The phases of a round.
const (
	PhaseBetting Phase = iota
	PhaseSpinning
	PhaseResult
)

// String names the phase.
func (p Phase) String() string {
	switch p {
	case PhaseSpinning:
		return "spinning"
	case PhaseResult:
		return "result"
	default:
		return "betting"
	}
}

// HistoryLength is how many past numbers the table remembers, which is what
// the board beside a real wheel shows.
const HistoryLength = 12

// Actions.
type (
	// PlaceBet puts a chip on a spot.
	PlaceBet struct {
		P      engine.PlayerID
		Spot   int
		Amount int64
	}

	// RemoveBet lifts the player's whole stake off one spot.
	RemoveBet struct {
		P    engine.PlayerID
		Spot int
	}

	// ClearBets lifts everything the player has on the table.
	ClearBets struct{ P engine.PlayerID }

	// Spin starts the wheel. Offline the player spins; in a room the clock
	// does, and this action is ignored.
	Spin struct{ P engine.PlayerID }
)

// Player satisfies engine.Action.
func (a PlaceBet) Player() engine.PlayerID  { return a.P }
func (a RemoveBet) Player() engine.PlayerID { return a.P }
func (a ClearBets) Player() engine.PlayerID { return a.P }
func (a Spin) Player() engine.PlayerID      { return a.P }

// Stake satisfies engine.Stake.
func (a PlaceBet) Stake() int64 { return a.Amount }

// Events.
type (
	// BettingOpened reports a new round taking bets.
	BettingOpened struct{ Deadline time.Time }

	// BetPlaced reports a chip landing.
	BetPlaced struct {
		P      engine.PlayerID
		Spot   int
		Amount int64
	}

	// BetReturned reports chips coming back off the layout.
	BetReturned struct {
		P      engine.PlayerID
		Spot   int
		Amount int64
	}

	// BettingClosed reports the table refusing further chips.
	BettingClosed struct{}

	// Spun reports where the ball landed. The presentation animates towards
	// this number, which is already decided.
	Spun struct{ Number int }

	// Settled reports one player's result for the round.
	Settled struct {
		P       engine.PlayerID
		Wagered int64
		Won     int64
	}

	// RoundEnded reports the table clearing.
	RoundEnded struct{}
)

// Event satisfies engine.Event.
func (BettingOpened) Event() {}
func (BetPlaced) Event()     {}
func (BetReturned) Event()   {}
func (BettingClosed) Event() {}
func (Spun) Event()          {}
func (Settled) Event()       {}
func (RoundEnded) Event()    {}

// Payee satisfies engine.Refund.
func (b BetReturned) Payee() engine.PlayerID { return b.P }

// Refund satisfies engine.Refund.
func (b BetReturned) Refund() int64 { return b.Amount }

// Payee satisfies engine.Settlement.
func (s Settled) Payee() engine.PlayerID { return s.P }

// Result satisfies engine.Settlement.
func (s Settled) Result() (wagered, won int64) { return s.Wagered, s.Won }

// Game satisfies engine.Settlement.
func (s Settled) Game() engine.Kind { return engine.KindRoulette }

// BetView is one stake as a viewer sees it. Other players' chips are drawn
// dimmed rather than hidden, which is most of the point of a shared table.
type BetView struct {
	Player engine.PlayerID
	Spot   int
	Amount int64
	Mine   bool
}

// Snapshot is the table as one viewer sees it.
type Snapshot struct {
	Phase    Phase
	Bets     []BetView
	MyTotal  int64
	Number   int // -1 until the ball lands
	LastWin  int64
	History  []int
	Deadline time.Time
	Me       engine.PlayerID
	CanSpin  bool
}

// Table is one roulette wheel.
type Table struct {
	rng *rand.Rand

	players []engine.PlayerID
	bets    map[engine.PlayerID]map[int]int64
	last    map[engine.PlayerID]map[int]int64

	phase    Phase
	number   int
	lastWin  map[engine.PlayerID]int64
	history  []int
	deadline time.Time

	bettingFor time.Duration

	minBet, maxBet int64
}

// NewTable returns a wheel taking bets.
func NewTable(r *rand.Rand) *Table {
	return &Table{
		rng:     r,
		bets:    map[engine.PlayerID]map[int]int64{},
		last:    map[engine.PlayerID]map[int]int64{},
		lastWin: map[engine.PlayerID]int64{},
		phase:   PhaseBetting,
		number:  -1,
		minBet:  1,
		maxBet:  500,
	}
}

// SetBettingTime gives the table a betting window. Offline it is zero, so the
// wheel waits for the player instead of the clock.
func (t *Table) SetBettingTime(d time.Duration) { t.bettingFor = d }

// SetLimits sets the table minimum and maximum for a single spot.
func (t *Table) SetLimits(minBet, maxBet int64) { t.minBet, t.maxBet = minBet, maxBet }

// Kind satisfies engine.Game.
func (t *Table) Kind() engine.Kind { return engine.KindRoulette }

// Join satisfies engine.Game. Roulette has no seats: everyone bets at once,
// so the seat number is only ever an index into who is present.
func (t *Table) Join(p engine.PlayerID, buyIn int64) (engine.Seat, error) {
	// Bets here come from the wallet one at a time, so there is nothing to
	// buy in with.
	if buyIn != 0 {
		return 0, engine.ErrNoStack
	}
	for i, existing := range t.players {
		if existing == p {
			return engine.Seat(i), nil
		}
	}
	t.players = append(t.players, p)
	if t.bets[p] == nil {
		t.bets[p] = map[int]int64{}
	}
	return engine.Seat(len(t.players) - 1), nil
}

// Leave satisfies engine.Game. Chips already on the layout stay there and are
// settled with everyone else's: money on the table is not abandoned.
func (t *Table) Leave(p engine.PlayerID) []engine.Event {
	for i, existing := range t.players {
		if existing == p {
			t.players = append(t.players[:i], t.players[i+1:]...)
			break
		}
	}
	return nil
}

// Deadline satisfies engine.Game.
func (t *Table) Deadline() (time.Time, bool) {
	if t.deadline.IsZero() {
		return time.Time{}, false
	}
	return t.deadline, true
}

// Tick satisfies engine.Game: it closes the betting window when one is set.
func (t *Table) Tick(now time.Time) []engine.Event {
	if t.deadline.IsZero() || now.Before(t.deadline) {
		return nil
	}
	switch t.phase {
	case PhaseBetting:
		return t.spin(now)
	case PhaseResult:
		return t.openBetting(now)
	}
	return nil
}

// Apply satisfies engine.Game.
func (t *Table) Apply(a engine.Action, now time.Time) ([]engine.Event, error) {
	switch act := a.(type) {
	case PlaceBet:
		return t.placeBet(act)
	case RemoveBet:
		return t.removeBet(act)
	case ClearBets:
		return t.clearBets(act.P)
	case Spin:
		return t.playerSpin(act, now)
	default:
		return nil, engine.ErrNotAllowed
	}
}

func (t *Table) placeBet(a PlaceBet) ([]engine.Event, error) {
	if t.phase != PhaseBetting && t.phase != PhaseResult {
		return nil, engine.ErrWrongPhase
	}
	if _, ok := SpotByID(a.Spot); !ok {
		return nil, engine.ErrInvalidBet
	}
	if a.Amount < t.minBet {
		return nil, engine.ErrInvalidBet
	}

	events := []engine.Event{}
	if t.phase == PhaseResult {
		// Betting again clears the previous result off the table first.
		events = append(events, t.clearRound()...)
	}

	if t.bets[a.P] == nil {
		t.bets[a.P] = map[int]int64{}
	}
	if t.bets[a.P][a.Spot]+a.Amount > t.maxBet {
		return nil, engine.ErrInvalidBet
	}
	t.bets[a.P][a.Spot] += a.Amount

	// The action and the event carry the same three fields, which is not a
	// coincidence: the event is the accepted form of the request.
	return append(events, BetPlaced(a)), nil
}

func (t *Table) removeBet(a RemoveBet) ([]engine.Event, error) {
	if t.phase != PhaseBetting {
		return nil, engine.ErrWrongPhase
	}
	amount := t.bets[a.P][a.Spot]
	if amount == 0 {
		return nil, engine.ErrNothingToBet
	}
	delete(t.bets[a.P], a.Spot)
	return []engine.Event{BetReturned{P: a.P, Spot: a.Spot, Amount: amount}}, nil
}

func (t *Table) clearBets(p engine.PlayerID) ([]engine.Event, error) {
	if t.phase != PhaseBetting {
		return nil, engine.ErrWrongPhase
	}
	if len(t.bets[p]) == 0 {
		return nil, engine.ErrNothingToBet
	}

	var events []engine.Event
	for spot, amount := range t.bets[p] {
		events = append(events, BetReturned{P: p, Spot: spot, Amount: amount})
	}
	t.bets[p] = map[int]int64{}
	return events, nil
}

// RepeatBets returns the previous round's stakes so the presentation can
// replay them as ordinary PlaceBet actions, each paid for in the usual way.
//
// Repeating is a convenience, not a rule: doing it as real bets is what keeps
// a player from repeating stakes they can no longer afford.
func (t *Table) RepeatBets(p engine.PlayerID) []Bet {
	var out []Bet
	for spot, amount := range t.last[p] {
		out = append(out, Bet{Spot: spot, Amount: amount})
	}
	sortBets(out)
	return out
}

func (t *Table) playerSpin(a Spin, now time.Time) ([]engine.Event, error) {
	// In a room the clock spins the wheel and this action is ignored, which
	// is why the offline path is the special case rather than the rule.
	if t.bettingFor > 0 {
		return nil, engine.ErrNotAllowed
	}
	if t.phase != PhaseBetting {
		return nil, engine.ErrWrongPhase
	}
	if t.totalStaked() == 0 {
		return nil, engine.ErrNothingToBet
	}
	if len(t.bets[a.P]) == 0 {
		// Spinning is the spinner's call, so it has to be their money.
		return nil, engine.ErrNothingToBet
	}
	return t.spin(now), nil
}

func (t *Table) totalStaked() int64 {
	var n int64
	for _, bets := range t.bets {
		for _, amount := range bets {
			n += amount
		}
	}
	return n
}

// spin closes betting, turns the wheel and settles everyone.
func (t *Table) spin(now time.Time) []engine.Event {
	events := []engine.Event{BettingClosed{}}

	n := t.rng.IntN(Pockets)
	t.number = n
	t.phase = PhaseResult
	events = append(events, Spun{Number: n})

	// Remember the stakes for the repeat key before they are cleared.
	t.last = map[engine.PlayerID]map[int]int64{}
	for p, bets := range t.bets {
		if len(bets) == 0 {
			continue
		}
		copied := make(map[int]int64, len(bets))
		for spot, amount := range bets {
			copied[spot] = amount
		}
		t.last[p] = copied
	}

	for _, p := range t.settlementOrder() {
		bets := t.bets[p]
		if len(bets) == 0 {
			continue
		}
		var wagered, won int64
		for spot, amount := range bets {
			wagered += amount
			if s, ok := SpotByID(spot); ok && s.Wins(n) {
				won += s.Pays(amount)
			}
		}
		t.lastWin[p] = won
		events = append(events, Settled{P: p, Wagered: wagered, Won: won})
	}

	t.history = append([]int{n}, t.history...)
	if len(t.history) > HistoryLength {
		t.history = t.history[:HistoryLength]
	}

	t.bets = map[engine.PlayerID]map[int]int64{}
	t.setDeadline(now, t.bettingFor)
	return append(events, RoundEnded{})
}

// settlementOrder keeps events in a stable order, so two sessions replaying
// the same round see the same thing.
func (t *Table) settlementOrder() []engine.PlayerID {
	order := make([]engine.PlayerID, 0, len(t.bets))
	for _, p := range t.players {
		if len(t.bets[p]) > 0 {
			order = append(order, p)
		}
	}
	// Anyone who left mid-round still has chips down and still gets paid.
	for p := range t.bets {
		if !contains(order, p) && len(t.bets[p]) > 0 {
			order = append(order, p)
		}
	}
	return order
}

func contains(list []engine.PlayerID, p engine.PlayerID) bool {
	for _, x := range list {
		if x == p {
			return true
		}
	}
	return false
}

// clearRound wipes the last result so a new round can take bets.
func (t *Table) clearRound() []engine.Event {
	t.number = -1
	t.phase = PhaseBetting
	t.lastWin = map[engine.PlayerID]int64{}
	return nil
}

func (t *Table) openBetting(now time.Time) []engine.Event {
	t.clearRound()
	t.setDeadline(now, t.bettingFor)
	return []engine.Event{BettingOpened{Deadline: t.deadline}}
}

func (t *Table) setDeadline(now time.Time, d time.Duration) {
	if d <= 0 {
		t.deadline = time.Time{}
		return
	}
	t.deadline = now.Add(d)
}

// Snapshot satisfies engine.Game.
func (t *Table) Snapshot(viewer engine.PlayerID) any {
	snap := Snapshot{
		Phase:    t.phase,
		Number:   t.number,
		LastWin:  t.lastWin[viewer],
		Deadline: t.deadline,
		Me:       viewer,
		History:  append([]int(nil), t.history...),
		CanSpin:  t.bettingFor == 0 && t.phase == PhaseBetting && t.totalStaked() > 0,
	}

	for p, bets := range t.bets {
		for spot, amount := range bets {
			mine := p == viewer
			if mine {
				snap.MyTotal += amount
			}
			snap.Bets = append(snap.Bets, BetView{
				Player: p,
				Spot:   spot,
				Amount: amount,
				Mine:   mine,
			})
		}
	}
	sortBetViews(snap.Bets)
	return snap
}

func sortBets(a []Bet) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j].Spot < a[j-1].Spot; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

// sortBetViews keeps the rendering order stable across snapshots; map order
// alone would make chips flicker between frames.
func sortBetViews(a []BetView) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && less(a[j], a[j-1]); j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

func less(x, y BetView) bool {
	if x.Spot != y.Spot {
		return x.Spot < y.Spot
	}
	return x.Player < y.Player
}
