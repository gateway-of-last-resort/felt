package app

import (
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestDumpFrames prints each screen for looking at rather than asserting on.
// Run it with FELT_DUMP=1.
func TestDumpFrames(t *testing.T) {
	if os.Getenv("FELT_DUMP") == "" {
		t.Skip("set FELT_DUMP=1")
	}

	m := testModel(t, 1000)
	m, _ = m.update(tea.WindowSizeMsg{Width: 110, Height: 34})

	for _, s := range []Screen{ScreenMenu, ScreenSlots, ScreenRoulette, ScreenBlackjack, ScreenStats} {
		m.screen = s
		if s == ScreenStats {
			m.statsTable.SetRows(m.statsRows())
		}
		t.Logf("\n===== %v =====\n%s\n", s, m.View().Content)
	}
}
