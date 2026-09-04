package app

import tea "charm.land/bubbletea/v2"

// NavigateMsg asks the root to switch screens.
type NavigateMsg struct{ To Screen }

// Navigate returns a command switching screens.
func Navigate(s Screen) tea.Cmd {
	return func() tea.Msg { return NavigateMsg{To: s} }
}

// checkedMsg carries the result of the server reachability check, which runs
// off the main loop so opening the menu never blocks on a dead host.
type checkedMsg struct{ status onlineStatus }

// savedMsg reports a wallet write.
type savedMsg struct{ err error }
