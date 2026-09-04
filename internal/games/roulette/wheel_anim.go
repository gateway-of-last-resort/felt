package roulette

import (
	"math"
	"time"

	"github.com/charmbracelet/harmonica"
	"github.com/gateway-of-last-resort/felt/internal/engine/roulette"
)

// Spin timing: the ball runs freely for a while, then a spring pulls it into
// the pocket the engine already chose.
const (
	freeSpinTime = 1600 * time.Millisecond
	ballSpeed    = 14.0 // pockets per second
	brakeLead    = 6.0  // pockets of runway before the target
	restDistance = 0.01
	restSpeed    = 0.05
)

// Ball animates the ball travelling to a known pocket.
//
// Position is measured in pockets around the wheel and grows without
// wrapping, so the spring always pulls forwards: a wrapping coordinate would
// let the ball take the short way round and appear to run backwards.
type Ball struct {
	pos float64
	vel float64

	spinning bool
	braking  bool
	stopAt   time.Time
	target   float64
	targetIx int

	spring harmonica.Spring
}

// NewBall returns a ball resting at a pocket index.
func NewBall(index int, fps int) *Ball {
	if fps <= 0 {
		fps = 30
	}
	return &Ball{
		pos:    float64(index),
		spring: harmonica.NewSpring(harmonica.FPS(fps), 5.0, 0.5),
	}
}

// Start sends the ball to the pocket holding number n.
func (b *Ball) Start(now time.Time, n int) {
	b.spinning = true
	b.braking = false
	b.stopAt = now.Add(freeSpinTime)
	b.targetIx = roulette.PocketIndex(n)
	b.vel = ballSpeed
}

// Advance moves the ball on by dt seconds, reporting whether it just landed.
func (b *Ball) Advance(now time.Time, dt float64) bool {
	if !b.spinning {
		return false
	}

	if !b.braking {
		b.pos += b.vel * dt
		if !now.Before(b.stopAt) {
			b.target = b.targetFor(b.targetIx)
			b.braking = true
		}
		return false
	}

	b.pos, b.vel = b.spring.Update(b.pos, b.vel, b.target)
	if math.Abs(b.pos-b.target) < restDistance && math.Abs(b.vel) < restSpeed {
		b.pos, b.vel = b.target, 0
		b.spinning, b.braking = false, false
		return true
	}
	return false
}

// targetFor is the nearest coordinate at least brakeLead ahead that lands on
// the wanted pocket.
func (b Ball) targetFor(index int) float64 {
	n := float64(roulette.Pockets)
	base := math.Ceil(b.pos + brakeLead)
	offset := math.Mod(float64(index)-math.Mod(base, n)+n, n)
	return base + offset
}

// Spinning reports whether the ball is still travelling.
func (b Ball) Spinning() bool { return b.spinning }

// Index is the pocket the ball is over.
func (b Ball) Index() int {
	i := int(math.Round(b.pos)) % roulette.Pockets
	if i < 0 {
		i += roulette.Pockets
	}
	return i
}

// Number is the number the ball is over.
func (b Ball) Number() int { return roulette.EuropeanOrder[b.Index()] }

// Blur reports whether the ball is moving too fast to read, which the view
// says with dimming rather than pretending the number means anything.
func (b Ball) Blur() bool { return b.spinning && math.Abs(b.vel) > 3 }

// Neighbours returns the numbers either side of the ball, which is what makes
// the wheel read as a wheel rather than a single number.
func (b Ball) Neighbours(n int) []int {
	out := make([]int, 0, 2*n+1)
	for d := -n; d <= n; d++ {
		i := (b.Index() + d) % roulette.Pockets
		if i < 0 {
			i += roulette.Pockets
		}
		out = append(out, roulette.EuropeanOrder[i])
	}
	return out
}
