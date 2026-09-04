package slots

import (
	"math"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/gateway-of-last-resort/felt/internal/rng"
)

// The house edge is a design decision, not an accident: this test is what
// keeps a casual tweak to a payout from quietly turning the machine into a
// charity or a mugging.
func TestTheoreticalRTP(t *testing.T) {
	got := TheoreticalRTP()
	if got < 0.94 || got > 0.96 {
		t.Errorf("theoretical RTP = %.4f, want between 0.94 and 0.96", got)
	}
	t.Logf("RTP %.4f, hit frequency %.4f", got, HitFrequency())
}

// Every strip must be the advertised length, or the RTP calculation and the
// reel geometry disagree.
func TestStripLengths(t *testing.T) {
	for i, strip := range Strips {
		if len(strip) != StopsPerReel {
			t.Errorf("strip %d has %d stops, want %d", i, len(strip), StopsPerReel)
		}
		for j, s := range strip {
			if s >= symbolCount {
				t.Errorf("strip %d position %d holds symbol %d, out of range", i, j, s)
			}
		}
	}
}

func TestPayout(t *testing.T) {
	cases := []struct {
		name    string
		a, b, c Symbol
		want    int64
	}{
		{"three sevens", Seven, Seven, Seven, threeOfKind[Seven]},
		{"wild completes a line", Bell, Wild, Bell, threeOfKind[Bell]},
		{"two wilds complete a line", Wild, Wild, Bar, threeOfKind[Bar]},
		{"all wilds pay the top prize", Wild, Wild, Wild, threeOfKind[Wild]},
		{"two cherries from the left", Cherry, Cherry, Bell, TwoCherries},
		{"wild stands in for a cherry", Wild, Cherry, Bell, TwoCherries},
		{"one cherry pays nothing", Cherry, Bell, Lemon, 0},
		{"cherries from the right pay nothing", Bell, Cherry, Cherry, 0},
		{"mixed line", Bell, Bar, Seven, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Payout(c.a, c.b, c.c); got != c.want {
				t.Errorf("Payout(%v,%v,%v) = %d, want %d", c.a, c.b, c.c, got, c.want)
			}
		})
	}
}

// The top prize must be strictly the richest, or the paytable is lying about
// what to chase.
func TestPayoutsAreOrdered(t *testing.T) {
	for s := Symbol(0); s < symbolCount-1; s++ {
		if threeOfKind[s] >= threeOfKind[s+1] {
			t.Errorf("%s pays %d, not less than %s at %d",
				s.Name(), threeOfKind[s], (s + 1).Name(), threeOfKind[s+1])
		}
	}
}

// Every symbol needs a glyph in both sets; a missing one shows up as "?" in
// the middle of a spin, which is exactly when nobody is reading carefully.
func TestGlyphsExist(t *testing.T) {
	for s := Symbol(0); s < symbolCount; s++ {
		for _, set := range []string{GlyphsASCII, GlyphsEmoji} {
			g := s.Glyph(set)
			if g == "" || g == "?" || g == "??" {
				t.Errorf("%s has no glyph in the %s set", s.Name(), set)
			}
		}
	}
}

// Realised return over many spins should land near the theoretical figure.
// The band is wide on purpose: this is a smoke test for the wiring between
// the strips, the reels and the resolver, not a second RTP calculation.
func TestSimulatedReturnTracksTheory(t *testing.T) {
	if testing.Short() {
		t.Skip("simulation is slow")
	}

	r := rng.NewSeeded([32]byte{21})
	var wagered, won int64

	const spins = 200000
	for i := 0; i < spins; i++ {
		var window [Reels][3]Symbol
		for reel := 0; reel < Reels; reel++ {
			stop := r.IntN(StopsPerReel)
			window[reel] = [3]Symbol{
				Strips[reel][(stop-1+StopsPerReel)%StopsPerReel],
				Strips[reel][stop],
				Strips[reel][(stop+1)%StopsPerReel],
			}
		}
		for _, p := range Paylines {
			wagered++
			won += Payout(window[0][p[0]], window[1][p[1]], window[2][p[2]])
		}
	}

	got := float64(won) / float64(wagered)
	if math.Abs(got-TheoreticalRTP()) > 0.05 {
		t.Errorf("simulated return %.4f, theoretical %.4f", got, TheoreticalRTP())
	}
	t.Logf("simulated %.4f over %d spins", got, spins)
}

// Every emoji glyph must be a single code point of exactly two columns.
//
// This is not fussiness: the reel window is drawn with borders either side of
// a fixed-width cell, so a glyph the terminal renders wider or narrower than
// we measured pushes the border out of line. Multi-code-point sequences —
// keycaps like "7️⃣", flags, anything with a variation selector — are where
// terminals disagree, so they are refused outright.
func TestEmojiGlyphsAreOneWideCodePoint(t *testing.T) {
	for s := Symbol(0); s < symbolCount; s++ {
		g := s.Glyph(GlyphsEmoji)

		if n := len([]rune(g)); n != 1 {
			t.Errorf("%s is %q, %d code points — terminals will not agree on its width",
				s.Name(), g, n)
		}
		if w := lipgloss.Width(g); w != 2 {
			t.Errorf("%s is %q, %d columns wide, want 2", s.Name(), g, w)
		}
	}
}

// The ASCII set has to be exactly one column, for the same reason.
func TestASCIIGlyphsAreOneColumn(t *testing.T) {
	for s := Symbol(0); s < symbolCount; s++ {
		g := s.Glyph(GlyphsASCII)
		if w := lipgloss.Width(g); w != 1 {
			t.Errorf("%s is %q, %d columns wide, want 1", s.Name(), g, w)
		}
	}
}
