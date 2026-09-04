package roulette

import "fmt"

// The layout is a graph of betting spots rather than a grid of numbers,
// because half the bets on a roulette table live on the lines *between*
// numbers: a chip on the edge shared by 8 and 11 is a split, one on the
// corner where 8, 9, 11 and 12 meet is a corner bet.
//
// Spots are generated on a half-grid. Even coordinates are the centres of
// number cells; odd ones are the boundaries between them. What a spot covers
// then falls out of which cells it touches — one number is a straight, two a
// split, four a corner — instead of being listed by hand 150 times.
const (
	// Cols and Rows are the shape of the number grid: 12 columns of 3.
	Cols = 12
	Rows = 3

	// Grid coordinates of the furniture around the numbers.
	zeroX     = -2 // the zero box, left of the grid
	streetY   = 2*Rows - 1
	columnX   = 2 * Cols
	dozenY    = streetY + 2
	outsideY  = dozenY + 2
	gridWideX = 2*Cols - 2
)

// Spot is one place a chip can sit.
type Spot struct {
	ID      int
	Label   string
	Type    BetType
	Numbers []int

	// X and Y are half-grid coordinates, used for cursor movement and by the
	// renderer to decide where to draw. They are not terminal columns.
	X, Y int
}

// Spots is the whole layout, built once at start-up.
var Spots = buildSpots()

// SpotByID returns a spot by index.
func SpotByID(id int) (Spot, bool) {
	if id < 0 || id >= len(Spots) {
		return Spot{}, false
	}
	return Spots[id], true
}

// numberAt returns the number in the given grid cell. The grid runs 3 on top
// down to 1 at the bottom, as it does on a real table.
func numberAt(col, row int) (int, bool) {
	if col < 0 || col >= Cols || row < 0 || row >= Rows {
		return 0, false
	}
	return col*Rows + (Rows - row), true
}

// cellOf is the inverse: where a number sits.
func cellOf(n int) (col, row int) {
	col = (n - 1) / Rows
	row = Rows - 1 - (n-1)%Rows
	return col, row
}

func buildSpots() []Spot {
	var spots []Spot
	add := func(s Spot) {
		s.ID = len(spots)
		spots = append(spots, s)
	}

	// Inside bets. Every point of the half-grid within the number field is
	// classified by how many cells it touches.
	for gy := 0; gy <= 2*(Rows-1); gy++ {
		for gx := 0; gx <= 2*(Cols-1); gx++ {
			nums := touching(gx, gy)
			t, ok := insideType(len(nums))
			if !ok {
				continue
			}
			add(Spot{Label: label(t, nums), Type: t, Numbers: nums, X: gx, Y: gy})
		}
	}

	// Streets and six lines sit on the bottom rail: a street under one
	// column, a six line on the boundary between two.
	for gx := 0; gx <= 2*(Cols-1); gx++ {
		nums := columnNumbers(gx)
		switch len(nums) {
		case 3:
			add(Spot{Label: label(Street, nums), Type: Street, Numbers: nums, X: gx, Y: streetY})
		case 6:
			add(Spot{Label: label(SixLine, nums), Type: SixLine, Numbers: nums, X: gx, Y: streetY})
		}
	}

	// Zero, and the basket it shares with the first three numbers.
	add(Spot{Label: "0", Type: Straight, Numbers: []int{0}, X: zeroX, Y: 2})
	add(Spot{Label: "0-1-2-3", Type: Corner, Numbers: []int{0, 1, 2, 3}, X: zeroX + 1, Y: streetY})

	// Column bets, one per row, on the right-hand rail.
	for row := 0; row < Rows; row++ {
		var nums []int
		for col := 0; col < Cols; col++ {
			n, _ := numberAt(col, row)
			nums = append(nums, n)
		}
		sortInts(nums)
		add(Spot{
			Label:   fmt.Sprintf("col %d", Rows-row),
			Type:    Column,
			Numbers: nums,
			X:       columnX,
			Y:       2 * row,
		})
	}

	// Dozens, each under four columns of the grid.
	for d := 0; d < 3; d++ {
		var nums []int
		for n := d*12 + 1; n <= (d+1)*12; n++ {
			nums = append(nums, n)
		}
		add(Spot{
			Label:   []string{"1st 12", "2nd 12", "3rd 12"}[d],
			Type:    Dozen,
			Numbers: nums,
			X:       d*8 + 3,
			Y:       dozenY,
		})
	}

	// The even-money row along the bottom.
	outside := []struct {
		label string
		typ   BetType
		pick  func(int) bool
	}{
		{"1-18", HighLow, func(n int) bool { return n >= 1 && n <= 18 }},
		{"even", OddEven, func(n int) bool { return n%2 == 0 }},
		{"red", RedBlack, func(n int) bool { return ColorOf(n) == Red }},
		{"black", RedBlack, func(n int) bool { return ColorOf(n) == Black }},
		{"odd", OddEven, func(n int) bool { return n%2 == 1 }},
		{"19-36", HighLow, func(n int) bool { return n >= 19 && n <= 36 }},
	}
	for i, o := range outside {
		var nums []int
		for n := 1; n <= 36; n++ {
			if o.pick(n) {
				nums = append(nums, n)
			}
		}
		add(Spot{
			Label:   o.label,
			Type:    o.typ,
			Numbers: nums,
			X:       i*4 + 1,
			Y:       outsideY,
		})
	}

	return spots
}

// touching returns the numbers whose cells meet the given half-grid point.
func touching(gx, gy int) []int {
	var nums []int
	for _, col := range cellsFor(gx) {
		for _, row := range cellsFor(gy) {
			if n, ok := numberAt(col, row); ok {
				nums = append(nums, n)
			}
		}
	}
	sortInts(nums)
	return nums
}

// cellsFor maps one half-grid axis value to the cell indices it covers: an
// even coordinate is a single cell, an odd one is the boundary between two.
func cellsFor(g int) []int {
	if g%2 == 0 {
		return []int{g / 2}
	}
	return []int{g / 2, g/2 + 1}
}

// columnNumbers returns the whole column, or both columns, under a point on
// the bottom rail.
func columnNumbers(gx int) []int {
	var nums []int
	for _, col := range cellsFor(gx) {
		for row := 0; row < Rows; row++ {
			if n, ok := numberAt(col, row); ok {
				nums = append(nums, n)
			}
		}
	}
	sortInts(nums)
	return nums
}

// insideType names a bet by how many numbers it covers.
func insideType(n int) (BetType, bool) {
	switch n {
	case 1:
		return Straight, true
	case 2:
		return Split, true
	case 4:
		return Corner, true
	default:
		return 0, false
	}
}

func label(t BetType, nums []int) string {
	switch t {
	case Straight:
		return fmt.Sprintf("%d", nums[0])
	case Split:
		return fmt.Sprintf("%d-%d", nums[0], nums[1])
	default:
		return fmt.Sprintf("%d-%d", nums[0], nums[len(nums)-1])
	}
}

// Direction is a cursor movement.
type Direction int

// The four directions.
const (
	Up Direction = iota
	Down
	Left
	Right
)

// Neighbour returns the spot the cursor moves to, or the one it is on when
// there is nothing that way.
//
// It picks the nearest spot in the direction of travel, weighting movement
// across the direction heavily, so that pressing right walks along a row
// instead of drifting diagonally into the outside bets.
func Neighbour(from int, dir Direction) int {
	cur, ok := SpotByID(from)
	if !ok {
		return from
	}

	best, bestScore := from, 1<<30
	for _, s := range Spots {
		if s.ID == cur.ID {
			continue
		}
		dx, dy := s.X-cur.X, s.Y-cur.Y

		var along, across int
		switch dir {
		case Up:
			along, across = -dy, abs(dx)
		case Down:
			along, across = dy, abs(dx)
		case Left:
			along, across = -dx, abs(dy)
		case Right:
			along, across = dx, abs(dy)
		}
		if along <= 0 {
			continue
		}

		score := along + across*4
		if score < bestScore {
			best, bestScore = s.ID, score
		}
	}
	return best
}

// DefaultSpot is where the cursor starts: the middle of the number grid.
func DefaultSpot() int {
	for _, s := range Spots {
		if s.Type == Straight && len(s.Numbers) == 1 && s.Numbers[0] == 17 {
			return s.ID
		}
	}
	return 0
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}
