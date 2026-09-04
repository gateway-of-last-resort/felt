package online

import (
	"net"
	"os/exec"

	tea "charm.land/bubbletea/v2"
)

// DoneMsg reports that the ssh session ended and the terminal is ours again.
type DoneMsg struct{ Err error }

// Launch hands the terminal to ssh and takes it back when the session ends.
//
// Bubble Tea suspends itself around the child process, so the server's own
// interface has the terminal to itself — there is no protocol between the two
// programs, and no rendering happening on this side at all.
func Launch(addr, nick string) tea.Cmd {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host, port = addr, "2222"
	}
	if nick == "" {
		nick = "guest"
	}

	keyPath, err := KeyPath()
	if err != nil {
		return func() tea.Msg { return DoneMsg{Err: err} }
	}
	knownHosts, err := KnownHostsPath()
	if err != nil {
		return func() tea.Msg { return DoneMsg{Err: err} }
	}

	c := exec.Command("ssh",
		"-p", port,
		"-i", keyPath,
		"-o", "UserKnownHostsFile="+knownHosts,
		"-o", "StrictHostKeyChecking=accept-new",
		// Without this, ssh offers every key in the agent before ours, which
		// on a server that identifies players by key is the wrong identity.
		"-o", "IdentitiesOnly=yes",
		nick+"@"+host,
	)
	return tea.ExecProcess(c, func(err error) tea.Msg { return DoneMsg{Err: err} })
}

// Command is the equivalent command line, shown when the game cannot run ssh
// itself so the player can connect from another terminal.
func Command(addr, nick string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host, port = addr, "2222"
	}
	key, _ := KeyPath()
	return "ssh -p " + port + " -i " + key + " " + nick + "@" + host
}
