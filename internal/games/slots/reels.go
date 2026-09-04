package slots

import (
	"math"
	"time"

	"github.com/charmbracelet/harmonica"
	"github.com/gateway-of-last-resort/felt/internal/engine/slots"
)

// FPS is the animation rate. The spring is built against it, so changing one
// without the other changes how the reels feel.
const FPS = 30

// Spin timing. Every reel starts together; reel i brakes at
// spinTime + i*reelStagger, which is what produces the left-to-right settle.
const (
	spinTime    = 900 * time.Millisecond
	reelStagger = 450 * time.Millisecond
)

const (
	// freeSpinSpeed is the cruising speed in symbols per second.
	freeSpinSpeed = 26.0

	// brakeLead is how much runway, in symbols, the reel gives itself before
	// the target stop. Too little and the spring cannot absorb the cruising
	// speed, so the reel sails past the stop and crawls back; five symbols is
	// enough at freeSpinSpeed to overshoot by a symbol and bounce home.
	brakeLead = 5.0

	// Settling thresholds: close enough to the stop, slow enough to call it
	// parked.
	restDistance = 0.01
	restSpeed    = 0.05
)

// Reel is one spinning band of symbols.
//
// pos is a continuous, ever-growing coordinate rather than an index modulo
// the strip length. That is what lets the spring pull toward a target that is
// always ahead of the current position: a wrapping coordinate would let it
// take the short way round and reverse the reel.
type Reel struct {
	strip *[slots.StopsPerReel]slots.Symbol

	pos float64
	vel float64

	spinning bool
	braking  bool
	stopAt   time.Time
	stopIdx  int
	target   float64

	spring harmonica.Spring
}

// NewReel returns a reel parked at the given stop.
func NewReel(strip *[slots.StopsPerReel]slots.Symbol, stop int) *Reel {
	return &Reel{
		strip:  strip,
		pos:    float64(stop),
		spring: harmonica.NewSpring(harmonica.FPS(FPS), 6.0, 0.45),
	}
}

// Start sets the reel spinning. It will cruise until stopAt, then spring to
// the strip index stop.
func (r *Reel) Start(stopAt time.Time, stop int) {
	r.spinning = true
	r.braking = false
	r.stopAt = stopAt
	r.stopIdx = ((stop % slots.StopsPerReel) + slots.StopsPerReel) % slots.StopsPerReel
	r.vel = freeSpinSpeed
}

// Advance moves the reel on by dt seconds and reports whether it came to rest
// on this step.
func (r *Reel) Advance(now time.Time, dt float64) bool {
	if !r.spinning {
		return false
	}

	if !r.braking {
		r.pos += r.vel * dt
		if !now.Before(r.stopAt) {
			r.target = r.targetFor(r.stopIdx)
			r.braking = true
		}
		return false
	}

	r.pos, r.vel = r.spring.Update(r.pos, r.vel, r.target)
	if math.Abs(r.pos-r.target) < restDistance && math.Abs(r.vel) < restSpeed {
		r.pos, r.vel = r.target, 0
		r.spinning, r.braking = false, false
		return true
	}
	return false
}

// targetFor returns the smallest coordinate at least brakeLead ahead of the
// current position that lands on the wanted strip index.
func (r Reel) targetFor(stop int) float64 {
	n := float64(slots.StopsPerReel)
	base := math.Ceil(r.pos + brakeLead)
	offset := math.Mod(float64(stop)-math.Mod(base, n)+n, n)
	return base + offset
}

// Spinning reports whether the reel is still moving.
func (r Reel) Spinning() bool { return r.spinning }

// Index is the strip position currently in the centre row.
func (r Reel) Index() int {
	i := int(math.Round(r.pos)) % slots.StopsPerReel
	if i < 0 {
		i += slots.StopsPerReel
	}
	return i
}

// Window returns the three visible symbols, top row first.
func (r Reel) Window() [3]slots.Symbol {
	i := r.Index()
	return [3]slots.Symbol{r.at(i - 1), r.at(i), r.at(i + 1)}
}

func (r Reel) at(i int) slots.Symbol {
	i %= slots.StopsPerReel
	if i < 0 {
		i += slots.StopsPerReel
	}
	return r.strip[i]
}

// Blur reports whether the reel is moving fast enough that a player could not
// read the symbols. The view dims a blurred reel instead of pretending the
// text is legible.
func (r Reel) Blur() bool { return r.spinning && math.Abs(r.vel) > 3 }
