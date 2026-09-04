// Package roulette holds the rules of the single-zero wheel.
package roulette

// Pockets is the number of pockets on a European wheel: 1 to 36 plus a
// single zero. That one green pocket is the entire house edge — every bet
// pays as though it were not there.
const Pockets = 37

// EuropeanOrder is the real running order of the wheel. Neighbouring numbers
// are deliberately far apart in value, which is why a ball animation cannot
// simply count upwards.
var EuropeanOrder = [Pockets]int{
	0, 32, 15, 19, 4, 21, 2, 25, 17, 34, 6, 27, 13, 36, 11, 30, 8, 23,
	10, 5, 24, 16, 33, 1, 20, 14, 31, 9, 22, 18, 29, 7, 28, 12, 35, 3, 26,
}

// Color is a pocket colour.
type Color int

// The pocket colours.
const (
	Green Color = iota
	Red
	Black
)

// String names the colour.
func (c Color) String() string {
	switch c {
	case Red:
		return "red"
	case Black:
		return "black"
	default:
		return "green"
	}
}

var redNumbers = map[int]bool{
	1: true, 3: true, 5: true, 7: true, 9: true, 12: true, 14: true, 16: true,
	18: true, 19: true, 21: true, 23: true, 25: true, 27: true, 30: true,
	32: true, 34: true, 36: true,
}

// ColorOf returns the colour of a pocket.
func ColorOf(n int) Color {
	switch {
	case n == 0:
		return Green
	case redNumbers[n]:
		return Red
	default:
		return Black
	}
}

// PocketIndex is where a number sits in the wheel's running order, which is
// what an animation needs in order to travel the right distance.
func PocketIndex(n int) int {
	for i, p := range EuropeanOrder {
		if p == n {
			return i
		}
	}
	return 0
}
