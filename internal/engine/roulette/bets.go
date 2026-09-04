package roulette

// BetType is a kind of bet, from the single number outwards.
type BetType int

// The bet types available on a European table.
const (
	Straight BetType = iota // one number,     35:1
	Split                   // two numbers,    17:1
	Street                  // three numbers,  11:1
	Corner                  // four numbers,    8:1
	SixLine                 // six numbers,     5:1
	Column                  // twelve numbers,  2:1
	Dozen                   // twelve numbers,  2:1
	RedBlack                // eighteen,        1:1
	OddEven                 // eighteen,        1:1
	HighLow                 // eighteen,        1:1
)

// Odds is what the bet pays to one. Every one of these is the fair price for
// a 36-pocket wheel, which is exactly how a 37-pocket wheel keeps 2.70%.
func (b BetType) Odds() int64 {
	switch b {
	case Straight:
		return 35
	case Split:
		return 17
	case Street:
		return 11
	case Corner:
		return 8
	case SixLine:
		return 5
	case Column, Dozen:
		return 2
	default:
		return 1
	}
}

// String names the bet type.
func (b BetType) String() string {
	switch b {
	case Straight:
		return "straight"
	case Split:
		return "split"
	case Street:
		return "street"
	case Corner:
		return "corner"
	case SixLine:
		return "six line"
	case Column:
		return "column"
	case Dozen:
		return "dozen"
	case RedBlack:
		return "red/black"
	case OddEven:
		return "odd/even"
	default:
		return "high/low"
	}
}

// Bet is a stake on one spot of the layout.
type Bet struct {
	Spot   int // index into Spots
	Amount int64
}

// Wins reports whether the spot covers the winning number.
func (s Spot) Wins(n int) bool {
	for _, c := range s.Numbers {
		if c == n {
			return true
		}
	}
	return false
}

// Pays is what a winning stake returns, the stake included.
func (s Spot) Pays(amount int64) int64 { return amount * (s.Type.Odds() + 1) }

// Resolve totals what a set of bets returns against a winning number.
func Resolve(bets []Bet, n int) int64 {
	var won int64
	for _, b := range bets {
		s, ok := SpotByID(b.Spot)
		if !ok || !s.Wins(n) {
			continue
		}
		won += s.Pays(b.Amount)
	}
	return won
}

// EdgeOf is the house edge on one bet type: the share of a stake the house
// keeps in the long run. It is 1/37 for every bet on the table, which is the
// single most surprising fact about roulette.
func EdgeOf(b BetType, covered int) float64 {
	p := float64(covered) / float64(Pockets)
	ev := p*float64(b.Odds()+1) - 1
	return -ev
}
