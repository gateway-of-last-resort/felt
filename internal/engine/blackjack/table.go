// Package blackjack holds the rules of the six-deck table.
//
// The engine deals atomically: an action goes in, and every card it turns
// over comes out as a CardDealt event. Nothing here waits 250 milliseconds
// between cards — the presentation does that, by playing the events back on a
// timer. That is what makes the rules testable without a clock, and what lets
// a server room hand the same events to five terminals at once.
package blackjack

import (
	"math/rand/v2"
	"time"

	"github.com/gateway-of-last-resort/felt/internal/deck"
	"github.com/gateway-of-last-resort/felt/internal/engine"
)

// Phase is where a round has got to.
//
// There is no Dealing phase: dealing is instant here, and the pause between
// cards belongs to the screen drawing them.
type Phase int

// The phases of a round.
const (
	PhaseWaiting Phase = iota
	PhaseBetting
	PhaseInsurance
	PhasePlayerTurn
	PhaseDealerTurn
	PhaseSettle
	PhaseShuffle
)

// String names the phase.
func (p Phase) String() string {
	switch p {
	case PhaseBetting:
		return "betting"
	case PhaseInsurance:
		return "insurance"
	case PhasePlayerTurn:
		return "player"
	case PhaseDealerTurn:
		return "dealer"
	case PhaseSettle:
		return "settle"
	case PhaseShuffle:
		return "shuffle"
	default:
		return "waiting"
	}
}

// Actions.
//
// Double, Split and Insure carry the amount they cost. The amount is
// derivable from the table, but it is sent anyway so that whoever holds the
// money can take it before the action is applied; the engine checks it
// against what the hand actually costs and refuses a mismatch, so a wrong or
// forged amount cannot buy anything.
type (
	// PlaceBet stakes a round.
	PlaceBet struct {
		P      engine.PlayerID
		Amount int64
	}

	// Hit draws one card to the active hand.
	Hit struct{ P engine.PlayerID }

	// Stand closes the active hand.
	Stand struct{ P engine.PlayerID }

	// Double matches the stake, draws exactly one card and closes the hand.
	Double struct {
		P      engine.PlayerID
		Amount int64
	}

	// Split turns a pair into two hands, each with its own stake.
	Split struct {
		P      engine.PlayerID
		Amount int64
	}

	// Insure takes or declines insurance against a dealer ace.
	Insure struct {
		P      engine.PlayerID
		Yes    bool
		Amount int64
	}

	// Surrender gives up the hand for half the stake.
	Surrender struct{ P engine.PlayerID }
)

// Player satisfies engine.Action.
func (a PlaceBet) Player() engine.PlayerID  { return a.P }
func (a Hit) Player() engine.PlayerID       { return a.P }
func (a Stand) Player() engine.PlayerID     { return a.P }
func (a Double) Player() engine.PlayerID    { return a.P }
func (a Split) Player() engine.PlayerID     { return a.P }
func (a Insure) Player() engine.PlayerID    { return a.P }
func (a Surrender) Player() engine.PlayerID { return a.P }

// Stake satisfies engine.Stake for the actions that cost money.
func (a PlaceBet) Stake() int64 { return a.Amount }
func (a Double) Stake() int64   { return a.Amount }
func (a Split) Stake() int64    { return a.Amount }

// Stake satisfies engine.Stake. Declining insurance costs nothing.
func (a Insure) Stake() int64 {
	if !a.Yes {
		return 0
	}
	return a.Amount
}

// Events.
type (
	// BetPlaced reports a stake accepted for the coming round.
	BetPlaced struct {
		Seat   engine.Seat
		Amount int64
	}

	// CardDealt reports one card going out. Seat is DealerSeat for the
	// dealer's own cards, and Hidden marks the hole card.
	CardDealt struct {
		Seat   engine.Seat
		Hand   int
		Card   deck.Card
		Hidden bool
	}

	// HoleRevealed reports the dealer turning the hole card face up.
	HoleRevealed struct{ Card deck.Card }

	// TurnStarted reports whose turn it is. Deadline is zero offline.
	TurnStarted struct {
		Seat     engine.Seat
		Hand     int
		Deadline time.Time
	}

	// HandSplit reports a pair becoming two hands.
	HandSplit struct {
		Seat engine.Seat
		Hand int
	}

	// InsuranceTaken reports the side bet.
	InsuranceTaken struct {
		Seat   engine.Seat
		Amount int64
	}

	// HandSettled reports one hand's result and what it returns.
	HandSettled struct {
		P       engine.PlayerID
		Seat    engine.Seat
		Hand    int
		Outcome Outcome
		Wagered int64
		Payout  int64
	}

	// RoundEnded reports the table clearing.
	RoundEnded struct{}

	// ShuffleStarted reports the shoe being replaced.
	ShuffleStarted struct{}
)

// Event satisfies engine.Event.
func (BetPlaced) Event()      {}
func (CardDealt) Event()      {}
func (HoleRevealed) Event()   {}
func (TurnStarted) Event()    {}
func (HandSplit) Event()      {}
func (InsuranceTaken) Event() {}
func (HandSettled) Event()    {}
func (RoundEnded) Event()     {}
func (ShuffleStarted) Event() {}

// Payee satisfies engine.Settlement.
func (h HandSettled) Payee() engine.PlayerID { return h.P }

// Result satisfies engine.Settlement.
func (h HandSettled) Result() (wagered, won int64) { return h.Wagered, h.Payout }

// Game satisfies engine.Settlement.
func (h HandSettled) Game() engine.Kind { return engine.KindBlackjack }

// HandView is one hand as a viewer sees it.
type HandView struct {
	Cards     []deck.Card
	Bet       int64
	Total     int
	Soft      bool
	Blackjack bool
	Bust      bool
	Doubled   bool
	Surrender bool
	FromSplit bool
	Outcome   Outcome
	Settled   bool
	Payout    int64
}

// SeatView is one seat as a viewer sees it.
type SeatView struct {
	Seat      engine.Seat
	Player    engine.PlayerID
	Nick      string
	Present   bool
	Bet       int64
	Insurance int64
	Hands     []HandView
	Splits    int
}

// Snapshot is the table as one viewer sees it. The dealer's hole card is
// omitted until it is turned over, so a snapshot can be sent to a player
// without leaking it.
type Snapshot struct {
	Phase      Phase
	Rules      Rules
	Seats      []SeatView
	Dealer     HandView
	HoleHidden bool
	Active     engine.Seat
	ActiveHand int
	Deadline   time.Time
	Me         engine.Seat
	CardsLeft  int
	NeedsShoe  bool
}

// seat is the engine's own record of a player at the table.
type seat struct {
	player    engine.PlayerID
	nick      string
	present   bool
	bet       int64
	insurance int64
	hands     []Hand
	active    int
	splits    int
	wagered   int64
	inRound   bool
}

// Table is one blackjack table.
type Table struct {
	rules Rules
	shoe  *deck.Shoe
	rng   *rand.Rand

	seats  []*seat
	dealer Hand
	hole   bool // the dealer's second card is still face down

	phase      Phase
	active     engine.Seat
	deadline   time.Time
	needsShoe  bool
	bettingFor time.Duration
	turnFor    time.Duration
}

// NewTable returns a table with a fresh shoe. seats is how many places it
// has; offline that is one.
func NewTable(r *rand.Rand, rules Rules, seats int) *Table {
	if seats < 1 {
		seats = 1
	}
	t := &Table{
		rules: rules,
		rng:   r,
		shoe:  deck.NewShoe(rules.Decks, rules.Penetration, r),
		phase: PhaseBetting,
	}
	t.seats = make([]*seat, seats)
	for i := range t.seats {
		t.seats[i] = &seat{}
	}
	return t
}

// SetTimers gives the table a betting window and a turn clock. Offline both
// are zero, which is what makes Deadline report nothing and Tick do nothing.
func (t *Table) SetTimers(betting, turn time.Duration) {
	t.bettingFor, t.turnFor = betting, turn
}

// Kind satisfies engine.Game.
func (t *Table) Kind() engine.Kind { return engine.KindBlackjack }

// Rules exposes the table configuration.
func (t *Table) Rules() Rules { return t.rules }

// Join satisfies engine.Game, returning an existing seat on reconnect.
func (t *Table) Join(p engine.PlayerID, buyIn int64) (engine.Seat, error) {
	// Bets here come from the wallet one at a time, so there is nothing to
	// buy in with.
	if buyIn != 0 {
		return 0, engine.ErrNoStack
	}
	if s, i := t.find(p); s != nil {
		s.present = true
		return i, nil
	}
	for i, s := range t.seats {
		if s.player == "" {
			s.player = p
			s.present = true
			return engine.Seat(i), nil
		}
	}
	return 0, engine.ErrTableFull
}

// Leave satisfies engine.Game. A seat in the middle of a round keeps its
// cards: the hands are played out and the money settles either way.
func (t *Table) Leave(p engine.PlayerID) []engine.Event {
	s, _ := t.find(p)
	if s == nil {
		return nil
	}
	s.present = false
	if !s.inRound {
		*s = seat{}
	}
	return nil
}

func (t *Table) find(p engine.PlayerID) (*seat, engine.Seat) {
	for i, s := range t.seats {
		if s.player == p && p != "" {
			return s, engine.Seat(i)
		}
	}
	return nil, 0
}

// Deadline satisfies engine.Game.
func (t *Table) Deadline() (time.Time, bool) {
	if t.deadline.IsZero() {
		return time.Time{}, false
	}
	return t.deadline, true
}

// Apply satisfies engine.Game.
func (t *Table) Apply(a engine.Action, now time.Time) ([]engine.Event, error) {
	s, seatNo := t.find(a.Player())
	if s == nil {
		return nil, engine.ErrUnknownSeat
	}

	switch act := a.(type) {
	case PlaceBet:
		return t.placeBet(s, seatNo, act, now)
	case Insure:
		return t.insure(s, seatNo, act, now)
	case Hit:
		return t.hit(s, seatNo, now)
	case Stand:
		return t.stand(s, seatNo, now)
	case Double:
		return t.double(s, seatNo, act, now)
	case Split:
		return t.split(s, seatNo, act, now)
	case Surrender:
		return t.surrender(s, seatNo, now)
	default:
		return nil, engine.ErrNotAllowed
	}
}

func (t *Table) placeBet(s *seat, seatNo engine.Seat, a PlaceBet, now time.Time) ([]engine.Event, error) {
	if t.phase != PhaseBetting && t.phase != PhaseWaiting && t.phase != PhaseSettle {
		return nil, engine.ErrWrongPhase
	}
	if err := t.rules.ValidBet(a.Amount); err != nil {
		return nil, err
	}

	// A bet placed while the last round is still on screen clears it first.
	// This has to happen before the seat is checked: settling leaves the seat
	// marked as in a round, and testing that first would refuse every bet
	// after the first one for the life of the table.
	events := []engine.Event{}
	if t.phase == PhaseSettle {
		events = append(events, t.clear()...)
	}
	if s.inRound {
		return nil, engine.ErrWrongPhase
	}

	s.bet = a.Amount
	s.inRound = true
	s.wagered = a.Amount
	events = append(events, BetPlaced{Seat: seatNo, Amount: a.Amount})

	dealt := t.deal(now)
	return append(events, dealt...), nil
}

// deal puts out the opening four cards and runs the checks that follow.
func (t *Table) deal(now time.Time) []engine.Event {
	events := []engine.Event{}

	if t.shoe.NeedsShuffle() {
		t.shoe.Shuffle()
		events = append(events, ShuffleStarted{})
	}

	t.dealer = Hand{}
	t.hole = true
	for _, s := range t.seats {
		if s.inRound {
			s.hands = []Hand{{Bet: s.bet}}
			s.active = 0
			s.splits = 0
			s.insurance = 0
		}
	}

	// Two rounds of cards: every live seat, then the dealer.
	for round := 0; round < 2; round++ {
		for i, s := range t.seats {
			if !s.inRound {
				continue
			}
			c := t.shoe.Draw()
			s.hands[0].Add(c)
			events = append(events, CardDealt{Seat: engine.Seat(i), Hand: 0, Card: c})
		}
		c := t.shoe.Draw()
		t.dealer.Add(c)
		events = append(events, CardDealt{
			Seat:   engine.DealerSeat,
			Card:   c,
			Hidden: round == 1,
		})
	}

	// Insurance is offered before anything else can happen.
	if t.rules.Insurance && DealerShowsAce(t.dealer) {
		t.phase = PhaseInsurance
		t.setDeadline(now, t.turnFor)
		return events
	}
	return append(events, t.afterInsurance(now)...)
}

// afterInsurance is where a round continues once insurance is settled: the
// dealer peeks, and a natural on either side ends things immediately.
func (t *Table) afterInsurance(now time.Time) []engine.Event {
	// The dealer peeks under a ten or an ace. Settling here is what stops a
	// player doubling into a hand that was already beaten.
	if DealerPeeks(t.dealer) && t.dealer.IsBlackjack() {
		return t.settle(now)
	}
	if t.everyoneHasBlackjack() {
		return t.settle(now)
	}
	return t.beginTurns(now)
}

func (t *Table) everyoneHasBlackjack() bool {
	for _, s := range t.seats {
		if !s.inRound {
			continue
		}
		if !s.hands[0].IsBlackjack() {
			return false
		}
	}
	return true
}

func (t *Table) insure(s *seat, seatNo engine.Seat, a Insure, now time.Time) ([]engine.Event, error) {
	if t.phase != PhaseInsurance {
		return nil, engine.ErrWrongPhase
	}
	events := []engine.Event{}

	if a.Yes {
		cost := InsuranceCost(s.bet)
		if a.Amount != cost {
			return nil, engine.ErrInvalidBet
		}
		s.insurance = cost
		s.wagered += cost
		events = append(events, InsuranceTaken{Seat: seatNo, Amount: cost})
	}

	return append(events, t.afterInsurance(now)...), nil
}

// beginTurns hands play to the first seat that can act.
func (t *Table) beginTurns(now time.Time) []engine.Event {
	t.phase = PhasePlayerTurn
	t.active = 0
	for _, s := range t.seats {
		if s.inRound {
			s.active = 0
		}
	}
	return t.advance(now, nil)
}

// advance moves to the next hand that can act, or to the dealer.
func (t *Table) advance(now time.Time, events []engine.Event) []engine.Event {
	for int(t.active) < len(t.seats) {
		s := t.seats[t.active]
		if !s.inRound {
			t.active++
			continue
		}
		for s.active < len(s.hands) && s.hands[s.active].Done(t.rules) {
			s.active++
		}
		if s.active < len(s.hands) {
			// A disconnected player is not kept waiting for: their hands
			// stand rather than holding up everybody else.
			if !s.present {
				s.hands[s.active].Stood = true
				continue
			}
			t.phase = PhasePlayerTurn
			t.setDeadline(now, t.turnFor)
			return append(events, TurnStarted{
				Seat:     t.active,
				Hand:     s.active,
				Deadline: t.deadline,
			})
		}
		t.active++
	}
	return append(events, t.dealerTurn(now)...)
}

// active hand of the seat whose turn it is.
func (t *Table) turnHand(s *seat, seatNo engine.Seat) (*Hand, error) {
	if t.phase != PhasePlayerTurn {
		return nil, engine.ErrWrongPhase
	}
	if seatNo != t.active {
		return nil, engine.ErrNotYourTurn
	}
	if s.active >= len(s.hands) {
		return nil, engine.ErrWrongPhase
	}
	return &s.hands[s.active], nil
}

func (t *Table) hit(s *seat, seatNo engine.Seat, now time.Time) ([]engine.Event, error) {
	h, err := t.turnHand(s, seatNo)
	if err != nil {
		return nil, err
	}
	c := t.shoe.Draw()
	h.Add(c)
	events := []engine.Event{CardDealt{Seat: seatNo, Hand: s.active, Card: c}}
	return t.advance(now, events), nil
}

func (t *Table) stand(s *seat, seatNo engine.Seat, now time.Time) ([]engine.Event, error) {
	h, err := t.turnHand(s, seatNo)
	if err != nil {
		return nil, err
	}
	h.Stood = true
	return t.advance(now, nil), nil
}

func (t *Table) double(s *seat, seatNo engine.Seat, a Double, now time.Time) ([]engine.Event, error) {
	h, err := t.turnHand(s, seatNo)
	if err != nil {
		return nil, err
	}
	if !h.CanDouble(t.rules) {
		return nil, engine.ErrNotAllowed
	}
	if a.Amount != h.Bet {
		return nil, engine.ErrInvalidBet
	}

	h.Bet += a.Amount
	h.Doubled = true
	s.wagered += a.Amount

	c := t.shoe.Draw()
	h.Add(c)
	events := []engine.Event{CardDealt{Seat: seatNo, Hand: s.active, Card: c}}
	return t.advance(now, events), nil
}

func (t *Table) split(s *seat, seatNo engine.Seat, a Split, now time.Time) ([]engine.Event, error) {
	h, err := t.turnHand(s, seatNo)
	if err != nil {
		return nil, err
	}
	if !h.CanSplit(t.rules, s.splits) {
		return nil, engine.ErrNotAllowed
	}
	if a.Amount != h.Bet {
		return nil, engine.ErrInvalidBet
	}

	left, right := h.Split()
	hands := make([]Hand, 0, len(s.hands)+1)
	hands = append(hands, s.hands[:s.active]...)
	hands = append(hands, left, right)
	hands = append(hands, s.hands[s.active+1:]...)
	s.hands = hands
	s.splits++
	s.wagered += a.Amount

	events := []engine.Event{HandSplit{Seat: seatNo, Hand: s.active}}

	// Each half draws its second card straight away.
	for i := s.active; i <= s.active+1; i++ {
		c := t.shoe.Draw()
		s.hands[i].Add(c)
		events = append(events, CardDealt{Seat: seatNo, Hand: i, Card: c})
	}
	return t.advance(now, events), nil
}

func (t *Table) surrender(s *seat, seatNo engine.Seat, now time.Time) ([]engine.Event, error) {
	h, err := t.turnHand(s, seatNo)
	if err != nil {
		return nil, err
	}
	if !h.CanSurrender(t.rules) {
		return nil, engine.ErrNotAllowed
	}
	h.Surrender = true
	return t.advance(now, nil), nil
}

// dealerTurn turns the hole card over and draws, unless there is nothing left
// to beat.
func (t *Table) dealerTurn(now time.Time) []engine.Event {
	t.phase = PhaseDealerTurn
	t.hole = false
	t.deadline = time.Time{}

	events := []engine.Event{}
	if len(t.dealer.Cards) > 1 {
		events = append(events, HoleRevealed{Card: t.dealer.Cards[1]})
	}

	if t.anyLive() {
		for ShouldHit(t.dealer, t.rules) {
			c := t.shoe.Draw()
			t.dealer.Add(c)
			events = append(events, CardDealt{Seat: engine.DealerSeat, Card: c})
		}
	}
	return append(events, t.settle(now)...)
}

// anyLive reports whether any hand can still win, which decides whether the
// dealer bothers to draw.
func (t *Table) anyLive() bool {
	for _, s := range t.seats {
		if !s.inRound {
			continue
		}
		for _, h := range s.hands {
			if !h.IsBust() && !h.Surrender {
				return true
			}
		}
	}
	return false
}

// settle scores every hand and closes the round.
func (t *Table) settle(now time.Time) []engine.Event {
	t.phase = PhaseSettle
	t.deadline = time.Time{}
	_ = now

	events := []engine.Event{}

	// Show the hole card if it is still down. A round can end without the
	// dealer ever drawing — their blackjack, or the player's — and settling a
	// hand face down leaves the player looking at a loss with no idea what
	// beat them.
	if t.hole && len(t.dealer.Cards) > 1 {
		events = append(events, HoleRevealed{Card: t.dealer.Cards[1]})
	}
	t.hole = false
	for i, s := range t.seats {
		if !s.inRound {
			continue
		}
		insurance := SettleInsurance(s.insurance, t.dealer)

		for hi := range s.hands {
			out, ret := Settle(s.hands[hi], t.dealer, t.rules)
			s.hands[hi].Outcome = out
			s.hands[hi].Settled = true
			s.hands[hi].Payout = ret

			// The insurance return rides on the first hand, so that one
			// settlement event carries the whole seat's wager exactly once.
			wagered := s.hands[hi].Bet
			payout := ret
			if hi == 0 {
				wagered += s.insurance
				payout += insurance
			}

			events = append(events, HandSettled{
				P:       s.player,
				Seat:    engine.Seat(i),
				Hand:    hi,
				Outcome: out,
				Wagered: wagered,
				Payout:  payout,
			})
		}
	}

	t.needsShoe = t.shoe.NeedsShuffle()
	return append(events, RoundEnded{})
}

// clear wipes the table for the next round.
func (t *Table) clear() []engine.Event {
	for _, s := range t.seats {
		s.hands = nil
		s.active = 0
		s.splits = 0
		s.insurance = 0
		s.wagered = 0
		s.inRound = false
		if s.player == "" || !s.present {
			*s = seat{}
		}
	}
	t.dealer = Hand{}
	t.hole = false
	t.active = 0
	t.phase = PhaseBetting
	return nil
}

// Tick satisfies engine.Game: it enforces deadlines, and offline there are
// none, so it does nothing.
func (t *Table) Tick(now time.Time) []engine.Event {
	if t.deadline.IsZero() || now.Before(t.deadline) {
		return nil
	}

	switch t.phase {
	case PhaseInsurance:
		// Time out means declining.
		return t.afterInsurance(now)

	case PhasePlayerTurn:
		s := t.seats[t.active]
		if s.inRound && s.active < len(s.hands) {
			s.hands[s.active].Stood = true
		}
		return t.advance(now, nil)
	}
	return nil
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
	_, me := t.find(viewer)
	if s, _ := t.find(viewer); s == nil {
		me = -1
	}

	snap := Snapshot{
		Phase:      t.phase,
		Rules:      t.rules,
		Active:     t.active,
		Deadline:   t.deadline,
		Me:         me,
		HoleHidden: t.hole,
		CardsLeft:  t.shoe.Remaining(),
		NeedsShoe:  t.needsShoe,
		Dealer:     t.dealerView(),
	}

	if int(t.active) < len(t.seats) {
		snap.ActiveHand = t.seats[t.active].active
	}

	for i, s := range t.seats {
		view := SeatView{
			Seat:      engine.Seat(i),
			Player:    s.player,
			Nick:      s.nick,
			Present:   s.present,
			Bet:       s.bet,
			Insurance: s.insurance,
			Splits:    s.splits,
		}
		for _, h := range s.hands {
			view.Hands = append(view.Hands, handView(h))
		}
		snap.Seats = append(snap.Seats, view)
	}
	return snap
}

// dealerView hides the hole card while it is face down. Trimming it here,
// rather than in the renderer, is what stops it leaking into another player's
// session over the wire.
func (t *Table) dealerView() HandView {
	d := t.dealer
	if t.hole && len(d.Cards) > 1 {
		d.Cards = d.Cards[:1]
	}
	v := handView(d)
	if t.hole {
		// With the hole card withheld, the totals would be misleading.
		v.Blackjack = false
		v.Bust = false
	}
	return v
}

func handView(h Hand) HandView {
	total, soft := h.Value()
	return HandView{
		Cards:     append([]deck.Card(nil), h.Cards...),
		Bet:       h.Bet,
		Total:     total,
		Soft:      soft,
		Blackjack: h.IsBlackjack(),
		Bust:      h.IsBust(),
		Doubled:   h.Doubled,
		Surrender: h.Surrender,
		FromSplit: h.FromSplit,
		Outcome:   h.Outcome,
		Settled:   h.Settled,
		Payout:    h.Payout,
	}
}

// StackForTest puts known cards on top of the shoe. It exists for tests and
// for looking at a table in a particular state; nothing in the game uses it.
func (t *Table) StackForTest(cards []deck.Card) { t.shoe.Stack(cards) }
