package render

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"

	"github.com/viveknathani/d2/d2graph"
	"github.com/viveknathani/d2/d2layouts/d2elklayout"
	"github.com/viveknathani/d2/d2lib"
	"github.com/viveknathani/d2/d2renderers/d2ascii"
	"github.com/viveknathani/d2/d2renderers/d2ascii/charset"
	"github.com/viveknathani/d2/d2renderers/d2svg"
	"github.com/viveknathani/d2/lib/log"
	"github.com/viveknathani/d2/lib/textmeasure"
	"github.com/viveknathani/dbtree/database"
	"github.com/viveknathani/dbtree/go2"
	"github.com/viveknathani/dbtree/graph"
)

// Format represents the output serialization format
type Format string

// Shape represents how the schema relationships are structured
type Shape string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
	ShapeTree  Shape  = "tree"
	ShapeFlat  Shape  = "flat"
	ShapeGraph Shape  = "graph"
)

// Render generates a string representation of the schema graph
// based on the specified format and shape.
func Render(g *graph.SchemaGraph, format Format, shape Shape) (string, error) {
	if g == nil {
		return "", fmt.Errorf("schema graph cannot be nil")
	}

	switch {
	case format == FormatText && shape == ShapeTree:
		tree := buildTree(g)
		return renderTreeAsText(tree, g.DatabaseName), nil
	case format == FormatJSON && shape == ShapeTree:
		tree := buildTree(g)
		return renderTreeAsJSON(tree, g.DatabaseName)
	case format == FormatText && shape == ShapeFlat:
		return renderFlatAsText(g)
	case format == FormatJSON && shape == ShapeFlat:
		return renderFlatAsJSON(g)
	case format == FormatText && shape == ShapeGraph:
		return renderGraphAsText(g)
	case format == FormatJSON && shape == ShapeGraph:
		return "", fmt.Errorf("graph shape is only supported with text format")
	default:
		return "", fmt.Errorf("unsupported format/shape combination: %s/%s", format, shape)
	}
}

// TreeNode represents a table in the tree structure
type TreeNode struct {
	TableName    graph.TableName
	Table        *database.Table
	Children     []*TreeNode
	IsCircular   bool
	AlreadyShown bool
}

// buildTree creates a tree structure from the schema graph
func buildTree(g *graph.SchemaGraph) *TreeNode {
	// Build adjacency list
	childrenMap := buildAdjacencyList(g)

	// Find root tables
	roots := findRootTables(g)

	// Track visited nodes
	visited := make(map[graph.TableName]bool)
	processing := make(map[graph.TableName]bool)

	// Build tree starting from roots (or arbitrary node if all cyclic)
	var rootNode *TreeNode

	if len(roots) == 0 && len(g.Nodes) > 0 {
		// All tables in cycles - pick first alphabetically
		var firstTable graph.TableName
		for tableName := range g.Nodes {
			if firstTable == "" || tableName < firstTable {
				firstTable = tableName
			}
		}
		rootNode = buildTreeNode(g, firstTable, childrenMap, visited, processing)
	} else {
		// Create virtual root to hold all actual roots
		rootNode = &TreeNode{
			TableName: graph.TableName(g.DatabaseName),
			Children:  []*TreeNode{},
		}

		for _, root := range roots {
			if child := buildTreeNode(g, root, childrenMap, visited, processing); child != nil {
				rootNode.Children = append(rootNode.Children, child)
			}
		}
	}

	// Add orphan tables
	orphans := findOrphanTables(g, visited)
	if len(orphans) > 0 {
		orphanRoot := &TreeNode{
			TableName: "orphan_tables",
			Children:  []*TreeNode{},
		}

		for _, orphan := range orphans {
			orphanRoot.Children = append(orphanRoot.Children, &TreeNode{
				TableName: orphan,
				Table:     g.Nodes[orphan],
				Children:  []*TreeNode{},
			})
		}

		rootNode.Children = append(rootNode.Children, orphanRoot)
	}

	return rootNode
}

func buildTreeNode(g *graph.SchemaGraph, tableName graph.TableName,
	childrenMap map[graph.TableName][]graph.TableName,
	visited, processing map[graph.TableName]bool) *TreeNode {

	// Handle cycles
	if processing[tableName] {
		return &TreeNode{
			TableName:  tableName,
			Table:      g.Nodes[tableName],
			IsCircular: true,
			Children:   []*TreeNode{},
		}
	}

	// Handle already visited
	if visited[tableName] {
		return &TreeNode{
			TableName:    tableName,
			Table:        g.Nodes[tableName],
			AlreadyShown: true,
			Children:     []*TreeNode{},
		}
	}

	visited[tableName] = true
	processing[tableName] = true

	node := &TreeNode{
		TableName: tableName,
		Table:     g.Nodes[tableName],
		Children:  []*TreeNode{},
	}

	// Add children
	for _, child := range childrenMap[tableName] {
		if childNode := buildTreeNode(g, child, childrenMap, visited, processing); childNode != nil {
			node.Children = append(node.Children, childNode)
		}
	}

	processing[tableName] = false
	return node
}

func renderTreeAsText(root *TreeNode, databaseName string) string {
	var sb strings.Builder
	sb.WriteString(databaseName)
	sb.WriteString("\n")

	for i, child := range root.Children {
		isLast := i == len(root.Children)-1
		renderTextNode(&sb, child, "", isLast)
	}

	return sb.String()
}

func renderTextNode(sb *strings.Builder, node *TreeNode, prefix string, isLast bool) {
	if node.TableName == "orphan_tables" {
		sb.WriteString("\nOrphan tables:\n")
		for _, child := range node.Children {
			sb.WriteString("• ")
			sb.WriteString(string(child.TableName))
			sb.WriteString("\n")
			if child.Table != nil {
				renderTableColumns(sb, child.Table, "  ")
			}
		}
		return
	}

	// Render table name
	sb.WriteString(prefix)
	connector := "├── "
	if isLast {
		connector = "└── "
	}
	sb.WriteString(connector)
	sb.WriteString(string(node.TableName))

	if node.IsCircular {
		sb.WriteString(" (circular reference)")
	} else if node.AlreadyShown {
		sb.WriteString(" (see above)")
	}
	sb.WriteString("\n")

	// Render table details
	if node.Table != nil && !node.IsCircular && !node.AlreadyShown {
		newPrefix := prefix
		if isLast {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
		renderTableColumns(sb, node.Table, newPrefix)
	}

	// Render children
	if !node.IsCircular && !node.AlreadyShown {
		childPrefix := prefix
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}

		for i, child := range node.Children {
			childIsLast := i == len(node.Children)-1
			renderTextNode(sb, child, childPrefix, childIsLast)
		}
	}
}

func renderTableColumns(sb *strings.Builder, table *database.Table, prefix string) {
	if table == nil {
		return
	}

	for i, col := range table.Columns {
		isLast := i == len(table.Columns)-1
		sb.WriteString(prefix)
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		sb.WriteString(connector)

		sb.WriteString(col.Name)
		sb.WriteString(" (")
		sb.WriteString(string(col.Type))
		sb.WriteString(")")

		// Add constraints
		appendConstraints(sb, table, col.Name)
		sb.WriteString("\n")
	}
}

func appendConstraints(sb *strings.Builder, table *database.Table, columnName string) {
	for _, constraint := range table.Constraints {
		if len(constraint.Columns) == 1 && constraint.Columns[0] == columnName {
			switch constraint.Kind {
			case database.PrimaryKey:
				sb.WriteString(" PRIMARY KEY")
			case database.Unique:
				sb.WriteString(" UNIQUE")
			}
		}

		// Foreign keys
		if constraint.Kind == database.ForeignKey {
			for j, fkCol := range constraint.Columns {
				if fkCol == columnName && j < len(constraint.ReferenceColumns) {
					sb.WriteString(" → ")
					sb.WriteString(constraint.ReferenceTable)
					sb.WriteString(".")
					sb.WriteString(constraint.ReferenceColumns[j])
				}
			}
		}
	}
}

func renderTreeAsJSON(root *TreeNode, databaseName string) (string, error) {
	type Column struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Constraint string `json:"constraint,omitempty"`
		Reference  string `json:"reference,omitempty"`
	}

	type Table struct {
		Name     string   `json:"name"`
		Columns  []Column `json:"columns"`
		Children []Table  `json:"children,omitempty"`
	}

	type Result struct {
		Database string  `json:"database"`
		Tables   []Table `json:"tables"`
		Orphans  []Table `json:"orphans,omitempty"`
	}

	var convertNode func(*TreeNode) *Table
	convertNode = func(node *TreeNode) *Table {
		table := &Table{
			Name:    string(node.TableName),
			Columns: []Column{},
		}

		// Only add details and children if not circular/already shown
		if node.Table != nil && !node.IsCircular && !node.AlreadyShown {
			for _, col := range node.Table.Columns {
				column := Column{
					Name: col.Name,
					Type: string(col.Type),
				}

				// Find constraints
				for _, constraint := range node.Table.Constraints {
					if len(constraint.Columns) == 1 && constraint.Columns[0] == col.Name {
						switch constraint.Kind {
						case database.PrimaryKey:
							column.Constraint = "PRIMARY KEY"
						case database.Unique:
							column.Constraint = "UNIQUE"
						}
					}

					// Foreign keys
					if constraint.Kind == database.ForeignKey {
						for j, fkCol := range constraint.Columns {
							if fkCol == col.Name && j < len(constraint.ReferenceColumns) {
								column.Reference = fmt.Sprintf("%s.%s",
									constraint.ReferenceTable,
									constraint.ReferenceColumns[j])
							}
						}
					}
				}

				table.Columns = append(table.Columns, column)
			}
		}

		// Only add children if not circular/already shown
		if !node.IsCircular && !node.AlreadyShown {
			for _, child := range node.Children {
				if childTable := convertNode(child); childTable != nil {
					table.Children = append(table.Children, *childTable)
				}
			}
		}

		return table
	}

	result := Result{
		Database: databaseName,
		Tables:   []Table{},
	}

	// Convert all children, handling orphans specially
	for _, child := range root.Children {
		if child.TableName == "orphan_tables" {
			for _, orphan := range child.Children {
				if table := convertNode(orphan); table != nil {
					result.Orphans = append(result.Orphans, *table)
				}
			}
		} else {
			if table := convertNode(child); table != nil {
				result.Tables = append(result.Tables, *table)
			}
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(data), nil
}

func renderFlatAsText(g *graph.SchemaGraph) (string, error) {
	var sb strings.Builder

	sb.WriteString("Database: ")
	sb.WriteString(g.DatabaseName)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Tables: %d\n\n", len(g.Nodes)))

	// Sort table names
	tableNames := getSortedTableNames(g)

	for _, tableName := range tableNames {
		table := g.Nodes[tableName]
		sb.WriteString(string(tableName))
		sb.WriteString("\n")

		for _, col := range table.Columns {
			sb.WriteString("  - ")
			sb.WriteString(col.Name)
			sb.WriteString(" (")
			sb.WriteString(string(col.Type))
			sb.WriteString(")")

			appendConstraints(&sb, table, col.Name)
			sb.WriteString("\n")
		}

		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func renderFlatAsJSON(g *graph.SchemaGraph) (string, error) {
	type Column struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Constraint string `json:"constraint,omitempty"`
		Reference  string `json:"reference,omitempty"`
	}

	type Table struct {
		Name    string   `json:"name"`
		Columns []Column `json:"columns"`
	}

	type Edge struct {
		From             string   `json:"from"`
		To               string   `json:"to"`
		Columns          []string `json:"columns"`
		ReferenceColumns []string `json:"referenceColumns"`
	}

	type Result struct {
		Database string  `json:"database"`
		Tables   []Table `json:"tables"`
		Edges    []Edge  `json:"edges"`
	}

	result := Result{
		Database: g.DatabaseName,
		Tables:   []Table{},
		Edges:    []Edge{},
	}

	// Build tables
	tableNames := getSortedTableNames(g)
	for _, tableName := range tableNames {
		t := g.Nodes[tableName]
		table := Table{
			Name:    string(tableName),
			Columns: []Column{},
		}

		for _, col := range t.Columns {
			column := Column{
				Name: col.Name,
				Type: string(col.Type),
			}

			// Find constraints
			for _, constraint := range t.Constraints {
				if len(constraint.Columns) == 1 && constraint.Columns[0] == col.Name {
					switch constraint.Kind {
					case database.PrimaryKey:
						column.Constraint = "PRIMARY KEY"
					case database.Unique:
						column.Constraint = "UNIQUE"
					}
				}

				// Foreign keys
				if constraint.Kind == database.ForeignKey {
					for j, fkCol := range constraint.Columns {
						if fkCol == col.Name && j < len(constraint.ReferenceColumns) {
							column.Reference = fmt.Sprintf("%s.%s",
								constraint.ReferenceTable,
								constraint.ReferenceColumns[j])
						}
					}
				}
			}

			table.Columns = append(table.Columns, column)
		}

		result.Tables = append(result.Tables, table)
	}

	// Build edges
	for _, edge := range g.Edges {
		result.Edges = append(result.Edges, Edge{
			From:             string(edge.FromTable),
			To:               string(edge.ToTable),
			Columns:          edge.Columns,
			ReferenceColumns: edge.ReferenceColumns,
		})
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(data), nil
}

func renderGraphAsText(g *graph.SchemaGraph) (string, error) {
	// Generate D2 source
	d2Source := generateD2Diagram(g)

	// Create context with silent logger to suppress D2 warnings
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := log.With(context.Background(), logger)

	// Compile with ELK layout
	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return "", fmt.Errorf("failed to create text ruler: %w", err)
	}

	compileOpts := &d2lib.CompileOptions{
		Ruler:  ruler,
		Layout: go2.Pointer("elk"),
		LayoutResolver: func(engine string) (d2graph.LayoutGraph, error) {
			return d2elklayout.DefaultLayout, nil
		},
	}

	themeId := int64(0)
	renderOpts := &d2svg.RenderOpts{
		Pad:     go2.Pointer(int64(0)),
		ThemeID: &themeId,
	}

	diagram, _, err := d2lib.Compile(ctx, d2Source, compileOpts, renderOpts)
	if err != nil {
		return "", fmt.Errorf("failed to compile D2 diagram: %w", err)
	}

	// Render to ASCII with scale and unicode charset
	artist := d2ascii.NewASCIIartist()
	asciiBytes, err := artist.Render(ctx, diagram, &d2ascii.RenderOpts{
		Scale:   go2.Pointer(1.0),
		Charset: charset.Unicode,
	})
	if err != nil {
		return "", fmt.Errorf("failed to render ASCII: %w", err)
	}

	return string(asciiBytes), nil
}

func generateD2Diagram(g *graph.SchemaGraph) string {
	var sb strings.Builder

	// Set vertical layout direction
	sb.WriteString("direction: left\n\n")

	// Sort tables for deterministic output
	tableNames := getSortedTableNames(g)

	// Define all tables as SQL table shapes
	for _, tableName := range tableNames {
		table := g.Nodes[tableName]
		if table == nil {
			continue
		}

		// Table definition
		sb.WriteString(string(tableName))
		sb.WriteString(": {\n")
		sb.WriteString("  shape: sql_table\n")

		// Columns
		for _, col := range table.Columns {
			sb.WriteString("  ")
			sb.WriteString(col.Name)
			sb.WriteString(": ")
			sb.WriteString(string(col.Type))

			// Add constraints as metadata
			var constraints []string
			for _, constraint := range table.Constraints {
				isInConstraint := false
				for _, constraintCol := range constraint.Columns {
					if constraintCol == col.Name {
						isInConstraint = true
						break
					}
				}

				if isInConstraint {
					switch constraint.Kind {
					case database.PrimaryKey:
						constraints = append(constraints, "PK")
					case database.Unique:
						constraints = append(constraints, "UNIQUE")
					case database.ForeignKey:
						constraints = append(constraints, "FK")
					}
				}
			}

			if len(constraints) > 0 {
				sb.WriteString(" {constraint: ")
				sb.WriteString(strings.Join(constraints, ", "))
				sb.WriteString("}")
			}

			sb.WriteString("\n")
		}

		sb.WriteString("}\n\n")
	}

	// Define all relationships
	for _, edge := range g.Edges {
		// For each column in the FK, create a relationship
		for i, col := range edge.Columns {
			if i < len(edge.ReferenceColumns) {
				sb.WriteString(string(edge.FromTable))
				sb.WriteString(".")
				sb.WriteString(col)
				sb.WriteString(" -> ")
				sb.WriteString(string(edge.ToTable))
				sb.WriteString(".")
				sb.WriteString(edge.ReferenceColumns[i])
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// Old tree rendering helper (still used by tree shape)
func renderTableBox(sb *strings.Builder, table *database.Table, tableName graph.TableName, isCircular bool) {
	// Calculate box width based on table name and columns
	maxWidth := len(table.Name)
	for _, col := range table.Columns {
		colStr := formatColumnForBox(table, col)
		if len(colStr) > maxWidth {
			maxWidth = len(colStr)
		}
	}

	// Add padding
	boxWidth := maxWidth + 2
	if boxWidth < 30 {
		boxWidth = 30
	}

	// Top border
	sb.WriteString("┌")
	sb.WriteString(strings.Repeat("─", boxWidth))
	sb.WriteString("┐\n")

	// Table name (centered)
	padding := (boxWidth - len(table.Name)) / 2
	sb.WriteString("│")
	sb.WriteString(strings.Repeat(" ", padding))
	sb.WriteString(table.Name)
	sb.WriteString(strings.Repeat(" ", boxWidth-padding-len(table.Name)))
	sb.WriteString("│")
	if isCircular {
		sb.WriteString(" (circular)")
	}
	sb.WriteString("\n")

	// Separator
	sb.WriteString("├")
	sb.WriteString(strings.Repeat("─", boxWidth))
	sb.WriteString("┤\n")

	// Columns
	for _, col := range table.Columns {
		colStr := formatColumnForBox(table, col)
		sb.WriteString("│ ")
		sb.WriteString(colStr)
		sb.WriteString(strings.Repeat(" ", boxWidth-len(colStr)-1))
		sb.WriteString("│\n")
	}

	// Bottom border
	sb.WriteString("└")
	sb.WriteString(strings.Repeat("─", boxWidth))
	sb.WriteString("┘\n")
}

func formatColumnForBox(table *database.Table, col database.Column) string {
	var parts []string

	// Check for constraints
	isPK := false
	isFK := false
	isUnique := false

	for _, constraint := range table.Constraints {
		// Check if column is part of this constraint
		isInConstraint := false
		for _, constraintCol := range constraint.Columns {
			if constraintCol == col.Name {
				isInConstraint = true
				break
			}
		}

		if isInConstraint {
			switch constraint.Kind {
			case database.PrimaryKey:
				isPK = true
			case database.Unique:
				isUnique = true
			case database.ForeignKey:
				isFK = true
			}
		}
	}

	// Build column string
	if isPK {
		parts = append(parts, "PK")
	}
	if isFK {
		parts = append(parts, "FK")
	}

	parts = append(parts, col.Name)

	if isUnique && !isPK {
		parts = append(parts, "(unique)")
	}

	return strings.Join(parts, " ")
}

// Helper functions

func buildAdjacencyList(g *graph.SchemaGraph) map[graph.TableName][]graph.TableName {
	childrenMap := make(map[graph.TableName][]graph.TableName)

	for tableName := range g.Nodes {
		childrenMap[tableName] = []graph.TableName{}
	}

	for _, edge := range g.Edges {
		childrenMap[edge.ToTable] = append(childrenMap[edge.ToTable], edge.FromTable)
	}

	return childrenMap
}

func findRootTables(g *graph.SchemaGraph) []graph.TableName {
	hasIncomingEdge := make(map[graph.TableName]bool)

	for _, edge := range g.Edges {
		hasIncomingEdge[edge.FromTable] = true
	}

	var roots []graph.TableName
	for tableName := range g.Nodes {
		if !hasIncomingEdge[tableName] {
			roots = append(roots, tableName)
		}
	}

	sort.Slice(roots, func(i, j int) bool {
		return roots[i] < roots[j]
	})

	return roots
}

func findOrphanTables(g *graph.SchemaGraph, visited map[graph.TableName]bool) []graph.TableName {
	var orphans []graph.TableName

	for tableName := range g.Nodes {
		if !visited[tableName] {
			hasRelationship := false
			for _, edge := range g.Edges {
				if edge.FromTable == tableName || edge.ToTable == tableName {
					hasRelationship = true
					break
				}
			}
			if !hasRelationship {
				orphans = append(orphans, tableName)
			}
		}
	}

	sort.Slice(orphans, func(i, j int) bool {
		return orphans[i] < orphans[j]
	})

	return orphans
}

func getSortedTableNames(g *graph.SchemaGraph) []graph.TableName {
	var tableNames []graph.TableName
	for name := range g.Nodes {
		tableNames = append(tableNames, name)
	}
	sort.Slice(tableNames, func(i, j int) bool {
		return tableNames[i] < tableNames[j]
	})
	return tableNames
}
