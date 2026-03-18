package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.menuCursor > 0 {
				m.menuCursor--
			}
		case "down", "j":
			// +1 for the "new connection" option
			if m.menuCursor < len(m.connections) {
				m.menuCursor++
			}
		case "enter":
			if m.menuCursor == len(m.connections) {
				// "New connection" selected
				m.newConnInputs = initNewConnInputs()
				m.newConnFocus = 0
				m.newConnErr = ""
				m.state = stateNewConn
				return m, nil
			}
			// Connect to selected
			conn := m.connections[m.menuCursor]
			m.currentConn = &conn
			m.loading = true
			m.state = stateSchema
			return m, loadSchema(&conn, m.format, m.shape)
		case "d":
			if len(m.connections) > 0 && m.menuCursor < len(m.connections) {
				name := m.connections[m.menuCursor].Name
				if err := m.connStore.Remove(name); err != nil {
					m.menuErr = fmt.Sprintf("failed to delete: %v", err)
					return m, nil
				}
				connections, err := m.connStore.Load()
				if err != nil {
					m.menuErr = fmt.Sprintf("failed to reload: %v", err)
					return m, nil
				}
				m.connections = connections
				if m.menuCursor >= len(m.connections) && m.menuCursor > 0 {
					m.menuCursor--
				}
				m.menuErr = ""
			}
		case "q":
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m model) viewMenu() string {
	s := titleStyle.Render("dbtree") + "\n"
	s += subtitleStyle.Render("Saved Connections") + "\n\n"

	if len(m.connections) == 0 {
		s += dimStyle.Render("  No saved connections yet.") + "\n\n"
	}

	for i, conn := range m.connections {
		label := fmt.Sprintf("%s (%s)", conn.Name, conn.Driver)
		if i == m.menuCursor {
			s += selectedStyle.Render("> "+label) + "\n"
		} else {
			s += unselectedStyle.Render(label) + "\n"
		}
	}

	// "New connection" option
	newLabel := "[+] Connect to new database"
	if m.menuCursor == len(m.connections) {
		s += selectedStyle.Render("> "+newLabel) + "\n"
	} else {
		s += unselectedStyle.Render(newLabel) + "\n"
	}

	s += "\n"
	if m.menuErr != "" {
		s += errorStyle.Render(m.menuErr) + "\n\n"
	}

	s += helpStyle.Render("↑/↓: Navigate  Enter: Select  d: Delete  q: Quit")
	return s
}
