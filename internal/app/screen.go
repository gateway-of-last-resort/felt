package app

// Screen identifies a top-level screen.
type Screen int

// The screens the client can show.
const (
	ScreenMenu Screen = iota
	ScreenSlots
	ScreenBlackjack
	ScreenRoulette
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
	return s == ScreenSlots || s == ScreenBlackjack || s == ScreenRoulette
}

func isOverlay(s Screen) bool {
	return s == ScreenStats || s == ScreenHelp || s == ScreenBankrupt
}
