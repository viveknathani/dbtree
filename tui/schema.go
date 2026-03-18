package tui

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

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
		switch msg.String() {
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "b":
			m.state = stateMenu
			m.schemaErr = ""
			return m, nil
		case "f":
			m.format = cycleFormat(m.format)
			if m.shape == render.ShapeChart && m.format == render.FormatJSON {
				// chart + json not supported, skip to text
				m.format = render.FormatText
			}
			m.loading = true
			return m, loadSchema(m.currentConn, m.format, m.shape)
		case "s":
			m.shape = cycleShape(m.shape)
			if m.shape == render.ShapeChart && m.format == render.FormatJSON {
				// skip chart when in json format
				m.shape = cycleShape(m.shape)
			}
			m.loading = true
			return m, loadSchema(m.currentConn, m.format, m.shape)
		case "r":
			m.loading = true
			return m, loadSchema(m.currentConn, m.format, m.shape)
		}
	}

	if m.loading {
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m model) viewSchema() string {
	if m.loading {
		s := titleStyle.Render(m.currentConn.Name) + "\n"
		s += subtitleStyle.Render("Loading schema...") + "\n"
		return s
	}

	if m.schemaErr != "" {
		s := titleStyle.Render(m.currentConn.Name) + "\n"
		s += errorStyle.Render("Error: "+m.schemaErr) + "\n\n"
		s += helpStyle.Render("r: Retry  b: Back  q: Quit")
		return s
	}

	header := titleStyle.Render(m.currentConn.Name)
	content := m.viewport.View()
	statusBar := statusBarStyle.Render(
		fmt.Sprintf(" Format: %s  Shape: %s  %d%%",
			m.format, m.shape, int(m.viewport.ScrollPercent()*100)),
	)
	help := helpStyle.Render("↑/↓: Scroll  f: Format  s: Shape  r: Refresh  b: Back  q: Quit")

	return header + "\n" + content + "\n" + statusBar + "\n" + help
}

func loadSchema(conn *store.Connection, format render.Format, shape render.Shape) tea.Cmd {
	return func() tea.Msg {
		connURL := conn.URL

		// Strip protocol prefixes for drivers that need it
		switch conn.Driver {
		case "mysql":
			connURL = strings.TrimPrefix(connURL, "mysql://")
		case "sqlite3":
			connURL = strings.TrimPrefix(connURL, "sqlite://")
		}

		db, err := sql.Open(conn.Driver, connURL)
		if err != nil {
			return schemaLoadedMsg{err: fmt.Errorf("failed to open database: %w", err)}
		}
		defer db.Close()

		ctx := context.Background()
		if err := db.PingContext(ctx); err != nil {
			return schemaLoadedMsg{err: fmt.Errorf("failed to connect: %w", err)}
		}

		schema, err := database.InspectSchema(ctx, db)
		if err != nil {
			return schemaLoadedMsg{err: fmt.Errorf("failed to inspect schema: %w", err)}
		}

		g, err := graph.Build(schema)
		if err != nil {
			return schemaLoadedMsg{err: fmt.Errorf("failed to build graph: %w", err)}
		}

		output, err := render.Render(g, format, shape)
		if err != nil {
			return schemaLoadedMsg{err: fmt.Errorf("failed to render: %w", err)}
		}

		return schemaLoadedMsg{output: output}
	}
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
