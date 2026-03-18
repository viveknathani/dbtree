package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/viveknathani/dbtree/render"
	"github.com/viveknathani/dbtree/store"
)

type state int

const (
	statePassword state = iota
	stateMenu
	stateNewConn
	stateSchema
)

// model is the main bubbletea model.
type model struct {
	state state

	// password view
	passwordInput textinput.Model
	passwordErr   string
	unlocking     bool

	// menu view
	connections []store.Connection
	menuCursor  int
	menuErr     string

	// new connection view
	newConnInputs [2]textinput.Model // 0=name, 1=url
	newConnFocus  int
	newConnErr    string

	// schema view
	viewport      viewport.Model
	currentConn   *store.Connection
	format        render.Format
	shape         render.Shape
	schemaErr     string
	loading       bool
	schemaReqID   uint64
	commandBuf    string

	// shared
	connStore *store.Store
	width     int
	height    int
	quitting  bool
}

// messages
type (
	connectionsLoadedMsg struct {
		connections []store.Connection
		connStore   *store.Store
		err         error
	}

	schemaLoadedMsg struct {
		output string
		err    error
		reqID  uint64
	}
)

func newModel() model {
	pi := textinput.New()
	pi.Placeholder = "master password"
	pi.EchoMode = textinput.EchoPassword
	pi.EchoCharacter = '*'
	pi.Focus()

	return model{
		state:         statePassword,
		passwordInput: pi,
		format:        render.FormatText,
		shape:         render.ShapeTree,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global quit
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.state == stateSchema {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 3 // room for status + help
		}
		return m, nil

	case connectionsLoadedMsg:
		m.unlocking = false
		if msg.err != nil {
			m.passwordErr = msg.err.Error()
			return m, nil
		}
		m.connStore = msg.connStore
		m.connections = msg.connections
		m.state = stateMenu
		return m, nil

	case schemaLoadedMsg:
		if msg.reqID != m.schemaReqID {
			return m, nil // stale response, discard
		}
		m.loading = false
		if msg.err != nil {
			m.schemaErr = msg.err.Error()
			return m, nil
		}
		m.schemaErr = ""
		m.viewport = viewport.New(m.width, m.height-3)
		m.viewport.SetContent(msg.output)
		return m, nil
	}

	switch m.state {
	case statePassword:
		return m.updatePassword(msg)
	case stateMenu:
		return m.updateMenu(msg)
	case stateNewConn:
		return m.updateNewConn(msg)
	case stateSchema:
		return m.updateSchema(msg)
	}

	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	switch m.state {
	case statePassword:
		return m.viewPassword()
	case stateMenu:
		return m.viewMenu()
	case stateNewConn:
		return m.viewNewConn()
	case stateSchema:
		return m.viewSchema()
	}

	return ""
}

