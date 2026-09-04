package blackjack

import "github.com/gateway-of-last-resort/felt/internal/deck"

// ShouldHit reports whether the dealer must take another card. The dealer has
// no choices: hit below seventeen, and on soft seventeen only at a table that
// says so.
func ShouldHit(h Hand, r Rules) bool {
	total, soft := h.Value()
	if total < 17 {
		return true
	}
	if total == 17 && soft && r.DealerHitsSoft17 {
		return true
	}
	return false
}

// PlayDealer draws until the dealer stands or busts.
//
// The model animates the same decision one card at a time through ShouldHit;
// this is the whole-hand version, which is what the tests and any future
// simulation want.
func PlayDealer(h Hand, s *deck.Shoe, r Rules) Hand {
	for ShouldHit(h, r) {
		h.Add(s.Draw())
	}
	return h
}

// DealerPeeks reports whether the dealer checks the hole card before play.
// The check happens on a ten or an ace showing: if there is a blackjack under
// there, the round is over before anyone doubles or splits into a loss.
func DealerPeeks(d Hand) bool {
	if len(d.Cards) == 0 {
		return false
	}
	up := d.Cards[0]
	return up.IsAce() || up.Value() == 10
}
