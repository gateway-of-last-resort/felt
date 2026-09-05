// Package video is Jacks or Better, the 9/6 pay table.
//
// "9/6" names the two lines that matter: a full house pays 9 and a flush 6,
// per coin. Those two numbers are what casinos shave to set the return —
// 9/6 pays 99.5% to perfect play, 8/5 barely 97%.
package video

import (
	"github.com/gateway-of-last-resort/felt/internal/deck"
	"github.com/gateway-of-last-resort/felt/internal/engine/poker"
)

// MaxCoins is the largest bet, and the only one that pays the royal bonus.
const MaxCoins = 5

// Payouts per coin. A royal at less than max coins pays RoyalPerCoin; at max
// coins it pays RoyalMaxCoins per coin instead, which is the whole reason
// anyone bets five.
const (
	RoyalPerCoin  int64 = 250
	RoyalMaxCoins int64 = 800
	minPayingPair       = deck.Jack
)

// perCoin is the 9/6 schedule, in coins won per coin staked.
var perCoin = map[poker.Category]int64{
	poker.RoyalFlush:    RoyalPerCoin,
	poker.StraightFlush: 50,
	poker.FourOfAKind:   25,
	poker.FullHouse:     9,
	poker.Flush:         6,
	poker.Straight:      4,
	poker.ThreeOfAKind:  3,
	poker.TwoPair:       2,
	// A bare pair pays only from jacks up; see Payout.
}

// Payout is what a hand returns for a bet of the given coins, stake included.
//
// Nothing below two pair pays unless the pair is jacks or better — that is
// the game's name and its whole shape: most hands lose, and the ones that
// scrape back the stake are high pairs.
func Payout(r poker.Rank, coins int64) int64 {
	if coins <= 0 {
		return 0
	}

	if r.Cat == poker.RoyalFlush && coins >= MaxCoins {
		return RoyalMaxCoins * coins
	}
	if mult, ok := perCoin[r.Cat]; ok {
		return mult * coins
	}
	if r.Cat == poker.Pair && r.Kickers[0] >= minPayingPair {
		return coins
	}
	return 0
}

// PaytableRow is one line of the paytable screen.
type PaytableRow struct {
	Name string
	// PerCoin is what one coin returns; Max is what the fifth coin returns,
	// which differs only for the royal.
	PerCoin int64
	Max     int64
}

// PaytableRows lists the schedule, richest first.
func PaytableRows() []PaytableRow {
	return []PaytableRow{
		{"Royal flush", RoyalPerCoin, RoyalMaxCoins * MaxCoins},
		{"Straight flush", perCoin[poker.StraightFlush], perCoin[poker.StraightFlush] * MaxCoins},
		{"Four of a kind", perCoin[poker.FourOfAKind], perCoin[poker.FourOfAKind] * MaxCoins},
		{"Full house", perCoin[poker.FullHouse], perCoin[poker.FullHouse] * MaxCoins},
		{"Flush", perCoin[poker.Flush], perCoin[poker.Flush] * MaxCoins},
		{"Straight", perCoin[poker.Straight], perCoin[poker.Straight] * MaxCoins},
		{"Three of a kind", perCoin[poker.ThreeOfAKind], perCoin[poker.ThreeOfAKind] * MaxCoins},
		{"Two pair", perCoin[poker.TwoPair], perCoin[poker.TwoPair] * MaxCoins},
		{"Jacks or better", 1, MaxCoins},
	}
}

// Pays reports whether a hand wins anything at all, which is what the screen
// uses to decide whether to celebrate.
func Pays(r poker.Rank) bool { return Payout(r, 1) > 0 }
