package roulette

import (
	"math"
	"testing"
)

// Every bet on a European wheel has exactly the same house edge: 1/37, or
// 2.70%. If a payout or a spot's coverage is ever wrong, this is the test
// that notices.
func TestHouseEdgeIsUniform(t *testing.T) {
	const want = 1.0 / 37.0

	seen := map[BetType]bool{}
	for _, s := range Spots {
		got := EdgeOf(s.Type, len(s.Numbers))
		if math.Abs(got-want) > 1e-12 {
			t.Errorf("%s covering %d numbers has edge %.6f, want %.6f",
				s.Label, len(s.Numbers), got, want)
		}
		seen[s.Type] = true
	}

	// And every bet type is actually reachable on the layout.
	for b := Straight; b <= HighLow; b++ {
		if !seen[b] {
			t.Errorf("no spot on the table offers a %s bet", b)
		}
	}
	if math.Abs(want-0.027027) > 1e-5 {
		t.Fatalf("the edge constant drifted: %.6f", want)
	}
}

// Staking one credit on every number returns 36 against 37 staked, which is
// the same 2.70% seen from the other side.
func TestCoveringTheBoardStillLoses(t *testing.T) {
	var bets []Bet
	for _, s := range Spots {
		if s.Type == Straight {
			bets = append(bets, Bet{Spot: s.ID, Amount: 1})
		}
	}
	if len(bets) != Pockets {
		t.Fatalf("the layout offers %d straight bets, want %d", len(bets), Pockets)
	}

	for n := 0; n < Pockets; n++ {
		if got := Resolve(bets, n); got != 36 {
			t.Fatalf("number %d returned %d on a fully covered board, want 36", n, got)
		}
	}
}

// Each spot must pay on exactly the numbers it covers, and on nothing else.
func TestSpotsWinOnlyOnTheirNumbers(t *testing.T) {
	for _, s := range Spots {
		covered := map[int]bool{}
		for _, n := range s.Numbers {
			covered[n] = true
		}
		for n := 0; n < Pockets; n++ {
			if got := s.Wins(n); got != covered[n] {
				t.Errorf("%s (%s) Wins(%d) = %v, want %v", s.Label, s.Type, n, got, covered[n])
			}
		}
	}
}

// The layout has to contain the right number of each kind of bet. Getting
// this wrong is easy and invisible: a missing split just never gets bet on.
func TestLayoutComposition(t *testing.T) {
	counts := map[BetType]int{}
	for _, s := range Spots {
		counts[s.Type]++
	}

	want := map[BetType]int{
		Straight: 37, // 36 numbers and the zero
		Split:    57, // 24 vertical between rows, 33 horizontal between columns
		Street:   12, // one per column of three
		Corner:   23, // 22 inside the grid, plus the 0-1-2-3 basket
		SixLine:  11, // one per pair of adjacent columns
		Column:   3,
		Dozen:    3,
		RedBlack: 2,
		OddEven:  2,
		HighLow:  2,
	}
	for typ, n := range want {
		if counts[typ] != n {
			t.Errorf("layout has %d %s spots, want %d", counts[typ], typ, n)
		}
	}
}

// Numbers sit where a real table puts them: 3 on top, 1 at the bottom, twelve
// columns across.
func TestGridPlacement(t *testing.T) {
	cases := []struct{ n, col, row int }{
		{1, 0, 2},
		{2, 0, 1},
		{3, 0, 0},
		{4, 1, 2},
		{34, 11, 2},
		{36, 11, 0},
	}
	for _, c := range cases {
		col, row := cellOf(c.n)
		if col != c.col || row != c.row {
			t.Errorf("%d sits at column %d row %d, want %d and %d", c.n, col, row, c.col, c.row)
		}
		if n, _ := numberAt(c.col, c.row); n != c.n {
			t.Errorf("column %d row %d holds %d, want %d", c.col, c.row, n, c.n)
		}
	}
}

// Spot-checks on the layout: the bets a player would actually reach for.
func TestKnownSpotsExist(t *testing.T) {
	find := func(typ BetType, nums ...int) bool {
		for _, s := range Spots {
			if s.Type != typ || len(s.Numbers) != len(nums) {
				continue
			}
			match := true
			for i, n := range nums {
				if s.Numbers[i] != n {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
		return false
	}

	cases := []struct {
		name string
		typ  BetType
		nums []int
	}{
		{"the 1-2 split", Split, []int{1, 2}},
		{"the 8-11 split across columns", Split, []int{8, 11}},
		{"the first street", Street, []int{1, 2, 3}},
		{"the last street", Street, []int{34, 35, 36}},
		{"the 8-9-11-12 corner", Corner, []int{8, 9, 11, 12}},
		{"the first six line", SixLine, []int{1, 2, 3, 4, 5, 6}},
		{"the basket", Corner, []int{0, 1, 2, 3}},
	}
	for _, c := range cases {
		if !find(c.typ, c.nums...) {
			t.Errorf("%s is missing from the layout", c.name)
		}
	}
}

// Colours must match a real wheel, and split evenly.
func TestColours(t *testing.T) {
	var red, black int
	for n := 1; n <= 36; n++ {
		switch ColorOf(n) {
		case Red:
			red++
		case Black:
			black++
		default:
			t.Errorf("%d is green", n)
		}
	}
	if red != 18 || black != 18 {
		t.Errorf("%d red and %d black, want 18 each", red, black)
	}
	if ColorOf(0) != Green {
		t.Error("zero is not green")
	}
	// A few known pockets.
	for n, want := range map[int]Color{1: Red, 2: Black, 19: Red, 20: Black, 36: Red} {
		if got := ColorOf(n); got != want {
			t.Errorf("%d is %v, want %v", n, got, want)
		}
	}
}

// The wheel holds every number exactly once.
func TestWheelOrder(t *testing.T) {
	seen := map[int]bool{}
	for _, n := range EuropeanOrder {
		if seen[n] {
			t.Fatalf("%d appears twice in the wheel order", n)
		}
		seen[n] = true
	}
	if len(seen) != Pockets {
		t.Fatalf("the wheel has %d pockets, want %d", len(seen), Pockets)
	}
	if EuropeanOrder[0] != 0 {
		t.Error("the wheel does not start at zero")
	}
	if got := PocketIndex(32); got != 1 {
		t.Errorf("32 is at index %d, want 1 (clockwise from zero)", got)
	}
}

// The cursor must be able to reach every spot on the table, or some bets are
// unplaceable however carefully the layout was generated.
func TestEverySpotIsReachable(t *testing.T) {
	start := DefaultSpot()
	seen := map[int]bool{start: true}

	// Breadth-first walk of the movement graph.
	queue := []int{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, dir := range []Direction{Up, Down, Left, Right} {
			next := Neighbour(cur, dir)
			if next == cur || seen[next] {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}

	if len(seen) != len(Spots) {
		var missing []string
		for _, s := range Spots {
			if !seen[s.ID] {
				missing = append(missing, s.Label+" ("+s.Type.String()+")")
			}
		}
		t.Errorf("%d of %d spots are unreachable by cursor, e.g. %v",
			len(Spots)-len(seen), len(Spots), missing[:min(5, len(missing))])
	}
}

// Movement must not get stuck: every direction either moves or stays put, and
// no spot points at itself in all four directions unless the table is tiny.
func TestCursorAlwaysSettles(t *testing.T) {
	for _, s := range Spots {
		stuck := 0
		for _, dir := range []Direction{Up, Down, Left, Right} {
			if Neighbour(s.ID, dir) == s.ID {
				stuck++
			}
		}
		if stuck == 4 {
			t.Errorf("%s is isolated: no direction leads anywhere", s.Label)
		}
	}
}
