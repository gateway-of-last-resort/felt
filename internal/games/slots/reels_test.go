package slots

import (
	"testing"
	"time"

	engine "github.com/gateway-of-last-resort/felt/internal/engine/slots"
)

// A reel must come to rest on the stop it was asked for, whatever the spring
// does on the way there.
func TestReelStopsOnTarget(t *testing.T) {
	strip := &engine.Strips[0]

	for stop := 0; stop < engine.StopsPerReel; stop++ {
		r := NewReel(strip, 0)
		start := time.Now()
		r.Start(start.Add(spinTime), stop)

		now := start
		const step = time.Second / FPS
		// Ten seconds of frames is far more than a spin needs; if the reel
		// has not parked by then the spring is not converging.
		for i := 0; i < 10*FPS; i++ {
			now = now.Add(step)
			if r.Advance(now, step.Seconds()) {
				break
			}
		}

		if r.Spinning() {
			t.Fatalf("reel still spinning after 10s aiming at stop %d", stop)
		}
		if got := r.Index(); got != stop {
			t.Errorf("reel parked on stop %d, want %d", got, stop)
		}
	}
}

// The reel must overshoot and settle back rather than stopping dead: that
// bounce is the whole point of driving it with a spring.
func TestReelOvershoots(t *testing.T) {
	r := NewReel(&engine.Strips[0], 0)
	start := time.Now()
	r.Start(start.Add(spinTime), 10)

	now := start
	const step = time.Second / FPS
	overshot := false
	for i := 0; i < 10*FPS; i++ {
		now = now.Add(step)
		done := r.Advance(now, step.Seconds())
		if r.braking && r.pos > r.target+0.05 {
			overshot = true
		}
		if done {
			break
		}
	}
	if !overshot {
		t.Error("reel never passed its stop; the spring is overdamped")
	}
}

// Window reads three consecutive strip positions, wrapping at the ends.
func TestReelWindow(t *testing.T) {
	strip := &engine.Strips[0]
	r := NewReel(strip, 0)

	got := r.Window()
	want := [3]engine.Symbol{strip[engine.StopsPerReel-1], strip[0], strip[1]}
	if got != want {
		t.Errorf("window at stop 0 = %v, want %v", got, want)
	}
}
