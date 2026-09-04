package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Level is the severity of a toast.
type Level int

// Toast levels.
const (
	Info Level = iota
	Good
	Bad
)

// ToastLife is how long a toast stays on screen.
const ToastLife = 2500 * time.Millisecond

// ToastExpiredMsg retires a toast. Seq identifies which one: a toast raised
// while an earlier one is still up bumps the sequence, so the older timer
// fires into nothing instead of dismissing the newer message.
type ToastExpiredMsg struct{ Seq int }

// Toast is a self-dismissing notification drawn under the table.
type Toast struct {
	text    string
	level   Level
	visible bool
	seq     int
}

// Show raises a toast and returns the command that will retire it.
func (t *Toast) Show(text string, lvl Level) tea.Cmd {
	t.text, t.level, t.visible = text, lvl, true
	t.seq++
	seq := t.seq
	return tea.Tick(ToastLife, func(time.Time) tea.Msg {
		return ToastExpiredMsg{Seq: seq}
	})
}

// Hide dismisses the toast immediately.
func (t *Toast) Hide() { t.visible = false }

// Expire dismisses the toast if the message belongs to the toast on screen.
func (t *Toast) Expire(msg ToastExpiredMsg) {
	if msg.Seq == t.seq {
		t.visible = false
	}
}

// Visible reports whether anything would be drawn.
func (t Toast) Visible() bool { return t.visible }

// View renders the toast, or an empty string when nothing is showing.
func (t Toast) View(width int, th Theme) string {
	if !t.visible || t.text == "" {
		return ""
	}
	fg := th.Text
	switch t.level {
	case Good:
		fg = th.Gold
	case Bad:
		fg = th.Red
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(fg).
		Foreground(fg).
		Padding(0, 1).
		Render(t.text)
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, box)
}
