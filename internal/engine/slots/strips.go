package slots

// Strips are the reel bands. They are uneven on purpose: the rare symbols
// appear on fewer stops, and the third reel is tightest, which is what makes
// two-of-a-kind near misses common and three-of-a-kind rare.
//
// Symbol order within a strip does not affect the return — every stop is
// equally likely — but it does affect how the spin reads, so runs of the same
// symbol are broken up.
var Strips = [Reels][StopsPerReel]Symbol{
	{
		Cherry, Lemon, Bell, Lemon, Bar, Cherry, Seven, Lemon,
		Bell, Cherry, Lemon, Bar, Bell, Lemon, Cherry, Diamond,
		Lemon, Bell, Cherry, Bar, Seven, Lemon, Bell, Cherry,
		Wild, Lemon, Bar, Bell, Seven, Cherry, Bar, Wild,
	},
	{
		Lemon, Bell, Cherry, Bar, Lemon, Bell, Seven, Cherry,
		Lemon, Bar, Bell, Lemon, Cherry, Bar, Diamond, Lemon,
		Bell, Cherry, Bar, Lemon, Seven, Bell, Lemon, Cherry,
		Bar, Wild, Bell, Lemon, Seven, Bar, Cherry, Wild,
	},
	{
		Lemon, Cherry, Bell, Lemon, Bar, Bell, Cherry, Lemon,
		Seven, Bell, Lemon, Cherry, Bar, Lemon, Bell, Diamond,
		Cherry, Lemon, Bar, Bell, Seven, Lemon, Cherry, Bell,
		Bar, Lemon, Seven, Cherry, Bell, Lemon, Bar, Wild,
	},
}

// threeOfKind is the multiplier applied to the line bet for three matching
// symbols, wilds substituting.
