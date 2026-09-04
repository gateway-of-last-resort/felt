// Package menu is the game picker.
//
// It knows nothing about screens: an item carries a string key and the root
// maps it. Keeping that dependency one-directional is what lets the root
// import the menu and the games without a cycle.
package menu

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gateway-of-last-resort/felt/internal/ui"
)

// nameWidth is the column the names occupy, so the terms line up down the
// right of the menu.
const nameWidth = 14

// Item is one row.
type Item struct {
	Key    string
	Name   string
	Desc   string
	MinBet int64
	RTP    float64

	// Ready is false for a row that cannot be opened — a game still to be
	// written, or a server that is not answering. The row says so rather than
	// letting the player find out by pressing enter.
	Ready bool

	// Note replaces the usual terms when a row has something else to say,
	// such as why the server is unreachable.
	Note string
}

// FilterValue satisfies list.Item.
func (i Item) FilterValue() string { return i.Name }

type delegate struct{ theme ui.Theme }

// Two lines per row: five rows plus the list title have to fit the body area
// of an 80x24 terminal, which is the smallest size felt supports.
func (d delegate) Height() int                         { return 2 }
func (d delegate) Spacing() int                        { return 1 }
func (d delegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d delegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	it, ok := li.(Item)
	if !ok {
		return
	}
	t := d.theme

	cursor := "  "
	name := lipgloss.NewStyle().Foreground(t.Text).Bold(true).Render(it.Name)
	if index == m.Index() {
		cursor = t.Title.Render("▸ ")
		name = t.Title.Render(it.Name)
	}

	meta := it.Note
	if meta == "" {
		// Screens that are not tables have no stake or return to advertise.
		if it.MinBet > 0 {
			meta = fmt.Sprintf("min %s", ui.Credits(it.MinBet))
			if it.RTP > 0 {
				meta += fmt.Sprintf("  ·  RTP %s", ui.Percent(it.RTP))
			}
		}
		if !it.Ready {
			if meta != "" {
				meta += "  ·  "
			}
			meta += "coming soon"
		}
	}

	_, _ = fmt.Fprintf(w, "%s%s%s\n    %s",
		cursor,
		lipgloss.NewStyle().Width(nameWidth).Render(name),
		t.Label.Render(meta),
		t.Dim.Render(it.Desc),
	)
}

// Model is the menu screen.
type Model struct {
	list  list.Model
	theme ui.Theme
}

// New builds the menu.
func New(items []Item, t ui.Theme) Model {
	li := make([]list.Item, len(items))
	for i, it := range items {
		li[i] = it
	}

	l := list.New(li, delegate{theme: t}, 0, 0)
	l.Title = "Choose your poison"
	l.Styles.Title = t.Title.Padding(0, 1)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)

	return Model{list: l, theme: t}
}

// SetTheme rebuilds the styles after the terminal reports its background.
func (m Model) SetTheme(t ui.Theme) Model {
	m.theme = t
	m.list.SetDelegate(delegate{theme: t})
	m.list.Styles.Title = t.Title.Padding(0, 1)
	return m
}

// SetSize resizes the list to the area the root gives it.
func (m *Model) SetSize(w, h int) { m.list.SetSize(w, h) }

// SetItem replaces one row, which is how the Online row is updated when the
// server check comes back.
func (m *Model) SetItem(key string, it Item) {
	for i, li := range m.list.Items() {
		if existing, ok := li.(Item); ok && existing.Key == key {
			m.list.SetItem(i, it)
			return
		}
	}
}

// Selected returns the item under the cursor.
func (m Model) Selected() (Item, bool) {
	it, ok := m.list.SelectedItem().(Item)
	return it, ok
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update satisfies tea.Model.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View satisfies tea.Model.
func (m Model) View() string { return m.list.View() }
