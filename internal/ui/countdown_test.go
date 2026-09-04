package ui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
)

// A zero deadline draws nothing, which is what every offline table has.
func TestCountdownIsInvisibleWithoutADeadline(t *testing.T) {
	if got := Countdown(time.Time{}, time.Now(), 40, Default()); got != "" {
		t.Errorf("Countdown with no deadline = %q, want empty", got)
	}
}

// The bar fits the width it is given and shows the seconds remaining.
func TestCountdownFitsAndCounts(t *testing.T) {
	now := time.Now()

	for _, width := range []int{20, 40, 80} {
		out := Countdown(now.Add(12*time.Second), now, width, Default())
		if got := lipgloss.Width(out); got > width {
			t.Errorf("at width %d the countdown is %d cells", width, got)
		}
		if !strings.Contains(out, "12s") {
			t.Errorf("at width %d the countdown does not show the time left: %q", width, out)
		}
	}
}

// A deadline in the past reads as zero rather than as a negative time.
func TestCountdownFloorsAtZero(t *testing.T) {
	now := time.Now()
	out := Countdown(now.Add(-5*time.Second), now, 40, Default())
	if !strings.Contains(out, "0s") {
		t.Errorf("a passed deadline shows %q, want zero seconds", out)
	}
	if strings.Contains(out, "-") {
		t.Errorf("a passed deadline shows negative time: %q", out)
	}
}
