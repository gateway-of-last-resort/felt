// Package rng provides the single random source shared by every game.
//
// One instance is created in main and passed down, so a test can swap in a
// seeded source and replay an exact sequence of spins and deals.
package rng

import (
	crand "crypto/rand"
	"math/rand/v2"
)

// New returns a ChaCha8 source seeded from crypto/rand.
func New() *rand.Rand {
	var seed [32]byte
	_, _ = crand.Read(seed[:])
	return rand.New(rand.NewChaCha8(seed))
}

// NewSeeded returns a deterministic source for tests.
func NewSeeded(seed [32]byte) *rand.Rand {
	return rand.New(rand.NewChaCha8(seed))
}
