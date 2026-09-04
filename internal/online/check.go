package online

import (
	"errors"
	"net"
	"os/exec"
	"time"
)

// CheckTimeout is how long the menu waits before calling a server
// unreachable. Short, because it runs every time the menu opens.
const CheckTimeout = 900 * time.Millisecond

// ErrNoSSH means the machine has no ssh client, so the game cannot hand the
// terminal over even though the server may be up.
var ErrNoSSH = errors.New("no ssh client on this machine")

// Status is what the menu knows about the server.
type Status struct {
	Addr      string
	Reachable bool
	HasSSH    bool
	Err       error
}

// Ready reports whether the Online item can actually be opened.
func (s Status) Ready() bool { return s.Reachable && s.HasSSH }

// Check dials the server and looks for an ssh client. It answers only
// "something is listening": whether it is felt, and whether this key is
// welcome, is the server's business.
func Check(addr string, timeout time.Duration) Status {
	st := Status{Addr: addr}

	if _, err := exec.LookPath("ssh"); err == nil {
		st.HasSSH = true
	} else {
		st.Err = ErrNoSSH
	}

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		if st.Err == nil {
			st.Err = err
		}
		return st
	}
	_ = conn.Close()

	st.Reachable = true
	return st
}
