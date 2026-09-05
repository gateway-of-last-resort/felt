package app

// Screen identifies a top-level screen.
type Screen int

// The screens the client can show.
const (
	ScreenMenu Screen = iota
	ScreenSlots
	ScreenBlackjack
	ScreenRoulette
	ScreenVideoPoker
	ScreenStats
	ScreenHelp
	ScreenBankrupt
)

// String is the screen's name in the top bar.
func (s Screen) String() string {
	switch s {
	case ScreenSlots:
		return "Slots"
	case ScreenBlackjack:
		return "Blackjack"
	case ScreenRoulette:
		return "Roulette"
	case ScreenVideoPoker:
		return "Video Poker"
	case ScreenStats:
		return "Statistics"
	case ScreenHelp:
		return "Help"
	case ScreenBankrupt:
		return "Out of credits"
	default:
		return "Felt"
	}
}

func isGame(s Screen) bool {
	return s == ScreenSlots || s == ScreenBlackjack ||
		s == ScreenRoulette || s == ScreenVideoPoker
}

func isOverlay(s Screen) bool {
	return s == ScreenStats || s == ScreenHelp || s == ScreenBankrupt
}
