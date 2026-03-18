package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/viveknathani/dbtree/store"
)

func (m model) updateNewConn(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.state = stateMenu
			return m, nil
		case tea.KeyTab:
			m.newConnFocus = (m.newConnFocus + 1) % len(m.newConnInputs)
			for i := range m.newConnInputs {
				if i == m.newConnFocus {
					m.newConnInputs[i].Focus()
				} else {
					m.newConnInputs[i].Blur()
				}
			}
			return m, nil
		case tea.KeyShiftTab:
			m.newConnFocus = (m.newConnFocus - 1 + len(m.newConnInputs)) % len(m.newConnInputs)
			for i := range m.newConnInputs {
				if i == m.newConnFocus {
					m.newConnInputs[i].Focus()
				} else {
					m.newConnInputs[i].Blur()
				}
			}
			return m, nil
		case tea.KeyEnter:
			name := strings.TrimSpace(m.newConnInputs[0].Value())
			url := strings.TrimSpace(m.newConnInputs[1].Value())

			if name == "" || url == "" {
				m.newConnErr = "both name and URL are required"
				return m, nil
			}

			driver := detectDriver(url)
			if driver == "" {
				m.newConnErr = "unsupported URL format (use postgres://, mysql://, clickhouse://, sqlite://, or .db/.sqlite/.sqlite3)"
				return m, nil
			}

			conn := store.Connection{
				Name:   name,
				URL:    url,
				Driver: driver,
			}

			if err := m.connStore.Add(conn); err != nil {
				m.newConnErr = fmt.Sprintf("failed to save: %v", err)
				return m, nil
			}

			connections, err := m.connStore.Load()
			if err != nil {
				m.newConnErr = fmt.Sprintf("failed to reload: %v", err)
				return m, nil
			}

			m.connections = connections
			m.menuCursor = len(m.connections) - 1
			m.state = stateMenu
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.newConnInputs[m.newConnFocus], cmd = m.newConnInputs[m.newConnFocus].Update(msg)
	return m, cmd
}

func (m model) viewNewConn() string {
	s := titleStyle.Render("dbtree") + "\n"
	s += subtitleStyle.Render("New Connection") + "\n\n"

	s += inputLabelStyle.Render("Name:") + "\n"
	s += "  " + m.newConnInputs[0].View() + "\n\n"

	s += inputLabelStyle.Render("Connection URL:") + "\n"
	s += "  " + m.newConnInputs[1].View() + "\n\n"

	if m.newConnErr != "" {
		s += errorStyle.Render(m.newConnErr) + "\n\n"
	}

	s += helpStyle.Render("Tab: Next field  Enter: Save  Esc: Cancel")
	return s
}

func detectDriver(url string) string {
	switch {
	case strings.HasPrefix(url, "postgres://"), strings.HasPrefix(url, "postgresql://"):
		return "postgres"
	case strings.HasPrefix(url, "mysql://"):
		return "mysql"
	case strings.HasPrefix(url, "clickhouse://"):
		return "clickhouse"
	case strings.HasPrefix(url, "sqlite://"):
		return "sqlite3"
	case strings.HasSuffix(url, ".db"), strings.HasSuffix(url, ".sqlite"), strings.HasSuffix(url, ".sqlite3"):
		return "sqlite3"
	default:
		return ""
	}
}
