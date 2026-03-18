package tui

import tea "github.com/charmbracelet/bubbletea"

// Run launches the interactive TUI.
func Run() error {
	p := tea.NewProgram(
		newModel(),
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}
