package slots

// Symbol is one reel symbol.
type Symbol uint8

// The symbols, ordered from most common to rarest.
const (
	Cherry Symbol = iota
	Lemon
	Bell
	Bar
	Seven
	Diamond
	Wild
	symbolCount
)

// Name is the symbol's name as printed in the paytable.
func (s Symbol) Name() string {
	switch s {
	case Cherry:
		return "Cherry"
	case Lemon:
		return "Lemon"
	case Bell:
		return "Bell"
	case Bar:
		return "Bar"
	case Seven:
		return "Seven"
	case Diamond:
		return "Diamond"
	case Wild:
		return "Wild"
	default:
		return "?"
	}
}

// Glyph returns the symbol as drawn on a reel. The ASCII set is the default
// because emoji are two cells wide in some terminals and one in others, which
// shears the reel boxes; the emoji set is there for terminals that get it
// right and is toggled from the settings.
func (s Symbol) Glyph(set string) string {
	if set == GlyphsEmoji {
		switch s {
		case Cherry:
			return "🍒"
		case Lemon:
			return "🍋"
		case Bell:
			return "🔔"
		case Bar:
			return "🎰"
		case Seven:
			// A fullwidth digit, not the keycap sequence "7\uFE0F\u20E3":
			// the keycap is three code points, and terminals disagree about
			// how many columns it occupies, which shears the reel border.
			// This is one code point and unambiguously two columns wide.
			return "７"
		case Diamond:
			return "💎"
		case Wild:
			return "⭐"
		}
		return "??"
	}
	switch s {
	case Cherry:
		return "C"
	case Lemon:
		return "L"
	case Bell:
		return "B"
	case Bar:
		return "≡"
	case Seven:
		return "7"
	case Diamond:
		return "◆"
	case Wild:
		return "★"
	}
	return "?"
}

// Glyph set names, as stored in the save file.
const (
	GlyphsASCII = "ascii"
	GlyphsEmoji = "emoji"
)

// Payline is the row index (0 top, 2 bottom) the line reads on each reel.
type Payline [Reels]int

// Paylines are ordered so that line 1 is the centre row: a player betting a
// single line bets the middle, as on a physical machine.
var Paylines = []Payline{
	{1, 1, 1},
	{0, 0, 0},
	{2, 2, 2},
	{0, 1, 2},
	{2, 1, 0},
}

// PaylineName labels a line in the paytable.
func PaylineName(i int) string {
	switch i {
	case 0:
		return "middle"
	case 1:
		return "top"
	case 2:
		return "bottom"
	case 3:
		return "diagonal ↘"
	case 4:
		return "diagonal ↗"
	default:
		return "line"
	}
}

// Reels is the number of reels and StopsPerReel the length of each strip.
const (
	Reels        = 3
	StopsPerReel = 32
)

// These are tuned against TheoreticalRTP, not chosen by eye: the test in this
// package fails if the return leaves the 94-96% band.
var threeOfKind = [symbolCount]int64{
	Cherry:  3,
	Lemon:   5,
	Bell:    8,
	Bar:     16,
	Seven:   48,
	Diamond: 160,
	Wild:    400,
}

// twoCherries is the consolation payout for cherries on the first two reels,
// which is what keeps a cold session from feeling like a wall of nothing.
//
// A single cherry pays nothing on purpose. Paying it would push the hit rate
// to 33% of lines, but most of those hits return less than the line bet, and
// a machine that constantly hands back less than it took reads as broken
// rather than generous.
const TwoCherries int64 = 4

// Payout returns the multiplier applied to the line bet for one payline.
// Wilds substitute for any symbol; three wilds pay the top prize rather than
// standing in for something cheaper.
func Payout(a, b, c Symbol) int64 {
	if s, ok := lineSymbol(a, b, c); ok {
		return threeOfKind[s]
	}
	// Cherries also pay from the left without completing a line.
	if cherryish(a) && cherryish(b) {
		return TwoCherries
	}
	return 0
}

func cherryish(s Symbol) bool { return s == Cherry || s == Wild }

// lineSymbol reports the symbol a line resolves to once wilds are
// substituted, and whether the line matches at all. An all-wild line resolves
// to Wild.
func lineSymbol(cs ...Symbol) (Symbol, bool) {
	var s Symbol
	found := false
	for _, c := range cs {
		if c == Wild {
			continue
		}
		if !found {
			s, found = c, true
			continue
		}
		if c != s {
			return 0, false
		}
	}
	if !found {
		return Wild, true
	}
	return s, true
}

// TheoreticalRTP is the exact return to player, by brute force over every
// combination of reel stops.
//
// The result does not depend on how many lines are active: each line reads an
// independently uniform stop on each reel, so every line has the same
// expectation, and both the stake and the expected win scale with the line
// count.
func TheoreticalRTP() float64 {
	var total int64
	for a := 0; a < StopsPerReel; a++ {
		for b := 0; b < StopsPerReel; b++ {
			for c := 0; c < StopsPerReel; c++ {
				total += Payout(Strips[0][a], Strips[1][b], Strips[2][c])
			}
		}
	}
	combos := float64(StopsPerReel) * float64(StopsPerReel) * float64(StopsPerReel)
	return float64(total) / combos
}

// HitFrequency is the share of lines that pay something, which is the number
// players feel long before they work out the return.
func HitFrequency() float64 {
	var hits int
	for a := 0; a < StopsPerReel; a++ {
		for b := 0; b < StopsPerReel; b++ {
			for c := 0; c < StopsPerReel; c++ {
				if Payout(Strips[0][a], Strips[1][b], Strips[2][c]) > 0 {
					hits++
				}
			}
		}
	}
	combos := StopsPerReel * StopsPerReel * StopsPerReel
	return float64(hits) / float64(combos)
}

// PaytableRow is one line of the paytable screen.
type PaytableRow struct {
	Symbol Symbol
	Pays   int64
}

// PaytableRows lists the three-of-a-kind payouts, richest first.
func PaytableRows() []PaytableRow {
	rows := make([]PaytableRow, 0, symbolCount)
	for s := Symbol(0); s < symbolCount; s++ {
		rows = append(rows, PaytableRow{Symbol: s, Pays: threeOfKind[s]})
	}
	// Richest first.
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows
}
