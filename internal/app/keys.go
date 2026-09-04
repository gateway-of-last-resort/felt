package app

import "charm.land/bubbles/v2/key"

// KeyMap is the set of bindings the root handles on every screen. Games add
// their own on top.
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Confirm key.Binding
	Back    key.Binding
	Help    key.Binding
	Stats   key.Binding
	Quit    key.Binding
}

// DefaultKeyMap returns the standard bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
		Right:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
		Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Stats:   key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "stats")),
		Quit:    key.NewBinding(key.WithKeys("ctrl+c", "q"), key.WithHelp("q", "quit")),
	}
}

// ShortHelp satisfies help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Back, k.Help, k.Quit}
}

// FullHelp satisfies help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Confirm, k.Back},
		{k.Stats, k.Help, k.Quit},
	}
}
