package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/viveknathani/dbtree/store"
)

func (m model) updatePassword(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.unlocking {
				return m, nil
			}
			password := m.passwordInput.Value()
			if password == "" {
				m.passwordErr = "password cannot be empty"
				return m, nil
			}
			m.passwordErr = ""
			m.unlocking = true
			return m, loadConnections(password)
		}
	}

	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	return m, cmd
}

func (m model) viewPassword() string {
	s := titleStyle.Render("dbtree") + "\n"
	s += subtitleStyle.Render("Database Schema Visualizer") + "\n\n"

	if m.unlocking {
		s += subtitleStyle.Render("Unlocking...") + "\n"
		return s
	}

	s += inputLabelStyle.Render("Master Password:") + "\n"
	s += "  " + m.passwordInput.View() + "\n\n"

	if m.passwordErr != "" {
		s += errorStyle.Render(m.passwordErr) + "\n\n"
	}

	s += helpStyle.Render("Enter to unlock, Ctrl+C to quit")
	return s
}

func loadConnections(password string) tea.Cmd {
	return func() tea.Msg {
		s, err := store.NewStore(password)
		if err != nil {
			return connectionsLoadedMsg{err: fmt.Errorf("failed to unlock: %v", err)}
		}

		connections, err := s.Load()
		if err != nil {
			return connectionsLoadedMsg{err: fmt.Errorf("failed to load connections: %v", err)}
		}

		return connectionsLoadedMsg{
			connections: connections,
			connStore:   s,
		}
	}
}

func initNewConnInputs() [2]textinput.Model {
	nameInput := textinput.New()
	nameInput.Placeholder = "e.g. Production DB"
	nameInput.Focus()

	urlInput := textinput.New()
	urlInput.Placeholder = "e.g. postgres://user:pass@host:5432/db"

	return [2]textinput.Model{nameInput, urlInput}
}
