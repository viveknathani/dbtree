package tui

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/viveknathani/dbtree/database"
	"github.com/viveknathani/dbtree/graph"
	"github.com/viveknathani/dbtree/render"
	"github.com/viveknathani/dbtree/store"
)

func (m model) updateSchema(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If command buffer is active, handle it
		if m.commandBuf != "" {
			return m.handleCommandBuf(msg)
		}

		switch msg.String() {
		case "q":
			m.quitting = true
			return m, tea.Quit
		case ":":
			m.commandBuf = ":"
			return m, nil
		case "b":
			m.state = stateMenu
			m.schemaErr = ""
			return m, nil
		case "f":
			m.format = cycleFormat(m.format)
			if m.shape == render.ShapeChart && m.format == render.FormatJSON {
				m.format = render.FormatText
			}
			m.loading = true
			m.schemaReqID++
			return m, loadSchema(m.currentConn, m.format, m.shape, m.schemaReqID)
		case "s":
			m.shape = cycleShape(m.shape)
			if m.shape == render.ShapeChart && m.format == render.FormatJSON {
				m.shape = cycleShape(m.shape)
			}
			m.loading = true
			m.schemaReqID++
			return m, loadSchema(m.currentConn, m.format, m.shape, m.schemaReqID)
		case "r":
			m.loading = true
			m.schemaReqID++
			return m, loadSchema(m.currentConn, m.format, m.shape, m.schemaReqID)
		}
	}

	if m.loading {
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m model) handleCommandBuf(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		cmd := strings.TrimSpace(m.commandBuf)
		m.commandBuf = ""
		if cmd == ":q" || cmd == ":quit" {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	case tea.KeyEsc:
		m.commandBuf = ""
		return m, nil
	case tea.KeyBackspace:
		if len(m.commandBuf) > 1 {
			m.commandBuf = m.commandBuf[:len(m.commandBuf)-1]
		} else {
			m.commandBuf = ""
		}
		return m, nil
	default:
		if msg.Type == tea.KeyRunes {
			m.commandBuf += msg.String()
		}
		return m, nil
	}
}

func (m model) connName() string {
	if m.currentConn != nil {
		return m.currentConn.Name
	}
	return "dbtree"
}

func (m model) viewSchema() string {
	if m.loading {
		s := titleStyle.Render(m.connName()) + "\n"
		s += subtitleStyle.Render("Loading schema...") + "\n"
		return s
	}

	if m.schemaErr != "" {
		s := titleStyle.Render(m.connName()) + "\n"
		s += errorStyle.Render("Error: "+m.schemaErr) + "\n\n"
		s += helpStyle.Render("r: Retry  b: Back  q: Quit")
		return s
	}

	header := titleStyle.Render(m.connName())
	content := m.viewport.View()
	statusBar := statusBarStyle.Render(
		fmt.Sprintf(" Format: %s  Shape: %s  %d%%",
			m.format, m.shape, int(m.viewport.ScrollPercent()*100)),
	)

	helpText := "↑/↓: Scroll  f: Format  s: Shape  r: Refresh  b: Back  q: Quit"
	if m.commandBuf != "" {
		helpText = m.commandBuf
	}
	help := helpStyle.Render(helpText)

	return header + "\n" + content + "\n" + statusBar + "\n" + help
}

func loadSchema(conn *store.Connection, format render.Format, shape render.Shape, reqID uint64) tea.Cmd {
	return func() tea.Msg {
		connURL := conn.URL

		// Format driver-specific connection strings
		switch conn.Driver {
		case "mysql":
			connURL = formatMySQLDSN(connURL)
		case "sqlite3":
			connURL = strings.TrimPrefix(connURL, "sqlite://")
		}

		db, err := sql.Open(conn.Driver, connURL)
		if err != nil {
			return schemaLoadedMsg{err: fmt.Errorf("failed to open database: %w", err), reqID: reqID}
		}
		defer db.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			return schemaLoadedMsg{err: fmt.Errorf("failed to connect: %w", err), reqID: reqID}
		}

		schema, err := database.InspectSchema(ctx, db)
		if err != nil {
			return schemaLoadedMsg{err: fmt.Errorf("failed to inspect schema: %w", err), reqID: reqID}
		}

		g, err := graph.Build(schema)
		if err != nil {
			return schemaLoadedMsg{err: fmt.Errorf("failed to build graph: %w", err), reqID: reqID}
		}

		output, err := render.Render(g, format, shape)
		if err != nil {
			return schemaLoadedMsg{err: fmt.Errorf("failed to render: %w", err), reqID: reqID}
		}

		return schemaLoadedMsg{output: output, reqID: reqID}
	}
}

// formatMySQLDSN converts a mysql:// URL into the go-sql-driver/mysql DSN format
// (user:pass@tcp(host:port)/dbname?params).
func formatMySQLDSN(rawURL string) string {
	if !strings.HasPrefix(rawURL, "mysql://") {
		return rawURL // already a raw DSN
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return strings.TrimPrefix(rawURL, "mysql://")
	}

	user := u.User.Username()
	pass, _ := u.User.Password()
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "3306"
	}
	dbName := strings.TrimPrefix(u.Path, "/")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, pass, host, port, dbName)

	if u.RawQuery != "" {
		dsn += "?" + u.RawQuery
	}

	return dsn
}

func cycleFormat(f render.Format) render.Format {
	if f == render.FormatText {
		return render.FormatJSON
	}
	return render.FormatText
}

func cycleShape(s render.Shape) render.Shape {
	switch s {
	case render.ShapeTree:
		return render.ShapeFlat
	case render.ShapeFlat:
		return render.ShapeChart
	default:
		return render.ShapeTree
	}
}
