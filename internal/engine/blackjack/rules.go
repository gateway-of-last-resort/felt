package blackjack

import "github.com/gateway-of-last-resort/felt/internal/engine"

// Rules is one table's configuration. Everything the house edge depends on
// lives here rather than being scattered through the game logic.
type Rules struct {
	Decks int

	// DealerHitsSoft17 false means the dealer stands on soft 17 (S17), which
	// is the better rule for the player by about 0.2%.
	DealerHitsSoft17 bool

	MaxSplits        int
	DoubleAfterSplit bool
	SplitAcesOneCard bool
	Insurance        bool
	Surrender        bool
	Penetration      float64

	// MinBet is also the betting increment. Blackjack pays 3:2 and insurance
	// costs half the stake, so an odd bet cannot be settled in whole credits;
	// keeping every bet a multiple of two removes the rounding question
	// instead of answering it in someone's favour.
	MinBet int64
	MaxBet int64
}

// Vegas6 is the standard six-deck table: dealer stands on soft 17, blackjack
// pays 3:2, double after split allowed, split aces get one card each.
func Vegas6() Rules {
	return Rules{
		Decks:            6,
		DealerHitsSoft17: false,
		MaxSplits:        3,
		DoubleAfterSplit: true,
		SplitAcesOneCard: true,
		Insurance:        true,
		Surrender:        true,
		Penetration:      0.75,
		MinBet:           2,
		MaxBet:           500,
	}
}

// ValidBet reports whether a stake is one this table accepts: inside the
// limits and even, so that 3:2 and insurance always settle in whole credits.
func (r Rules) ValidBet(n int64) error {
	if n < r.MinBet || n > r.MaxBet || n%2 != 0 {
		return engine.ErrInvalidBet
	}
	return nil
}

// ClampBet rounds a stake into something ValidBet accepts, which is what a
// typed-in amount goes through before it is offered.
func (r Rules) ClampBet(n int64) int64 {
	if n > r.MaxBet {
		n = r.MaxBet
	}
	if n%2 != 0 {
		n--
	}
	if n < r.MinBet {
		return r.MinBet
	}
	return n
}

// Outcome is how one hand finished.
type Outcome int

// The outcomes, ordered from worst to best for the player.
const (
	Lose Outcome = iota
	Surrendered
	Push
	Win
	Blackjack
)

// String names the outcome as shown next to the hand.
func (o Outcome) String() string {
	switch o {
	case Surrendered:
		return "surrender"
	case Push:
		return "push"
	case Win:
		return "win"
	case Blackjack:
		return "blackjack"
	default:
		return "lose"
	}
}

// Settle scores one player hand against the dealer and returns what goes back
// to the player: nothing on a loss, the stake on a push, twice it on a win.
//
// The stake has already left the wallet, so these are returns, not profits. A
// win pays back 2×bet, of which half is the player's own money.
func Settle(p, d Hand, r Rules) (Outcome, int64) {
	if p.Surrender {
		return Surrendered, p.Bet / 2
	}

	// A bust loses immediately, even when the dealer goes on to bust too.
	// That is where the house edge actually comes from.
	if p.IsBust() {
		return Lose, 0
	}

	playerBJ := p.IsBlackjack()
	dealerBJ := d.IsBlackjack()

	switch {
	case playerBJ && dealerBJ:
		return Push, p.Bet
	case playerBJ:
		// 3:2 — the stake back plus half as much again.
		return Blackjack, p.Bet + p.Bet*3/2
	case dealerBJ:
		return Lose, 0
	}

	if d.IsBust() {
		return Win, p.Bet * 2
	}

	switch pt, dt := p.Total(), d.Total(); {
	case pt > dt:
		return Win, p.Bet * 2
	case pt < dt:
		return Lose, 0
	default:
		return Push, p.Bet
	}
}

// SettleInsurance returns what the insurance side bet pays back. Insurance
// costs half the main stake and pays 2:1 when the dealer does have blackjack,
// which returns three times what was put up.
func SettleInsurance(stake int64, dealer Hand) int64 {
	if stake <= 0 || !dealer.IsBlackjack() {
		return 0
	}
	return stake * 3
}

// InsuranceCost is the stake for insuring a hand: half the main bet.
func InsuranceCost(bet int64) int64 { return bet / 2 }

// DealerShowsAce reports whether insurance may be offered.
func DealerShowsAce(d Hand) bool {
	return len(d.Cards) > 0 && d.Cards[0].IsAce()
}
