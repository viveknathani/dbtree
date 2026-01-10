package render

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/viveknathani/dbtree/database"
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
	TableName graph.TableName
	Table     *database.Table
	Children  []*TreeNode
	IsCircular bool
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
			TableName:     tableName,
			Table:         g.Nodes[tableName],
			AlreadyShown:  true,
			Children:      []*TreeNode{},
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
		Database string   `json:"database"`
		Tables   []Table  `json:"tables"`
		Orphans  []Table  `json:"orphans,omitempty"`
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
	// Find connected components
	components := findConnectedComponents(g)

	var results []string
	for _, component := range components {
		componentStr := renderComponentAsGraph(g, component)
		results = append(results, componentStr)
	}

	return strings.Join(results, "\n\n"), nil
}

type tableLayout struct {
	name       graph.TableName
	table      *database.Table
	y          int // Y position where table starts
	height     int // Number of lines tall
	boxWidth   int // Width of the box
	isCircular bool
	// Map column name to its line index within the table box
	columnLines map[string]int
}

type canvas struct {
	grid   [][]rune
	width  int
	height int
}

func newCanvas(width, height int) *canvas {
	grid := make([][]rune, height)
	for i := range grid {
		grid[i] = make([]rune, width)
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}
	return &canvas{grid: grid, width: width, height: height}
}

func (c *canvas) set(x, y int, ch rune) {
	if y >= 0 && y < c.height && x >= 0 && x < c.width {
		c.grid[y][x] = ch
	}
}

func (c *canvas) get(x, y int) rune {
	if y >= 0 && y < c.height && x >= 0 && x < c.width {
		return c.grid[y][x]
	}
	return ' '
}

func (c *canvas) String() string {
	var sb strings.Builder
	for y := 0; y < c.height; y++ {
		line := string(c.grid[y])
		// Trim trailing spaces
		line = strings.TrimRight(line, " ")
		sb.WriteString(line)
		if y < c.height-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func renderComponentAsGraph(g *graph.SchemaGraph, component []graph.TableName) string {
	// Order tables within component
	orderedTables := orderTablesInComponent(g, component)
	circular := findCircularReferences(g, component)

	// Calculate layouts for all tables
	layouts := make(map[graph.TableName]*tableLayout)
	currentY := 0
	maxBoxWidth := 26 // Minimum box width

	for _, tableName := range orderedTables {
		table := g.Nodes[tableName]
		if table == nil {
			continue
		}

		layout := &tableLayout{
			name:        tableName,
			table:       table,
			y:           currentY,
			isCircular:  circular[tableName],
			columnLines: make(map[string]int),
		}

		// Calculate box width
		maxWidth := len(table.Name)
		for _, col := range table.Columns {
			colStr := formatColumnForBox(table, col)
			if len(colStr) > maxWidth {
				maxWidth = len(colStr)
			}
		}
		boxWidth := maxWidth + 2
		if boxWidth < maxBoxWidth {
			boxWidth = maxBoxWidth
		}
		layout.boxWidth = boxWidth

		if boxWidth > maxBoxWidth {
			maxBoxWidth = boxWidth
		}

		// Calculate height: top border + title + separator + columns + bottom border
		layout.height = 4 + len(table.Columns)

		// Map column names to line numbers (relative to table start)
		for i, col := range table.Columns {
			lineNum := 3 + i // 0=top border, 1=title, 2=separator, 3+=columns
			layout.columnLines[col.Name] = lineNum
		}

		layouts[tableName] = layout
		currentY += layout.height + 1 // +1 for spacing between tables
	}

	// Normalize box widths to maximum
	for _, layout := range layouts {
		layout.boxWidth = maxBoxWidth
	}

	// Calculate canvas size (add extra width for arrows on the right)
	canvasWidth := maxBoxWidth + 40 // Extra space for arrows
	canvasHeight := currentY

	canvas := newCanvas(canvasWidth, canvasHeight)

	// Draw all tables
	for _, tableName := range orderedTables {
		layout := layouts[tableName]
		if layout == nil {
			continue
		}
		drawTableOnCanvas(canvas, layout)
	}

	// Draw all FK relationships as arrows
	drawForeignKeyArrows(canvas, g, component, layouts)

	return canvas.String()
}

func drawTableOnCanvas(canvas *canvas, layout *tableLayout) {
	table := layout.table
	y := layout.y
	boxWidth := layout.boxWidth

	// Top border
	canvas.set(0, y, '┌')
	for x := 1; x < boxWidth-1; x++ {
		canvas.set(x, y, '─')
	}
	canvas.set(boxWidth-1, y, '┐')

	// Table name (centered)
	y++
	padding := (boxWidth - 2 - len(table.Name)) / 2
	canvas.set(0, y, '│')
	x := 1
	for i := 0; i < padding; i++ {
		canvas.set(x, y, ' ')
		x++
	}
	for _, ch := range table.Name {
		canvas.set(x, y, ch)
		x++
	}
	for x < boxWidth-1 {
		canvas.set(x, y, ' ')
		x++
	}
	canvas.set(boxWidth-1, y, '│')

	// Separator
	y++
	canvas.set(0, y, '├')
	for x := 1; x < boxWidth-1; x++ {
		canvas.set(x, y, '─')
	}
	canvas.set(boxWidth-1, y, '┤')

	// Columns
	for _, col := range table.Columns {
		y++
		colStr := formatColumnForBox(table, col)
		canvas.set(0, y, '│')
		canvas.set(1, y, ' ')
		x := 2
		for _, ch := range colStr {
			canvas.set(x, y, ch)
			x++
		}
		for x < boxWidth-1 {
			canvas.set(x, y, ' ')
			x++
		}
		canvas.set(boxWidth-1, y, '│')
	}

	// Bottom border
	y++
	canvas.set(0, y, '└')
	for x := 1; x < boxWidth-1; x++ {
		canvas.set(x, y, '─')
	}
	canvas.set(boxWidth-1, y, '┘')
}

func drawForeignKeyArrows(canvas *canvas, g *graph.SchemaGraph, component []graph.TableName, layouts map[graph.TableName]*tableLayout) {
	// Build set of tables in component
	inComponent := make(map[graph.TableName]bool)
	for _, t := range component {
		inComponent[t] = true
	}

	// Collect all FK relationships within this component
	type fkRelation struct {
		fromTable  graph.TableName
		fromColumn string
		toTable    graph.TableName
	}
	var relations []fkRelation

	for _, tableName := range component {
		table := g.Nodes[tableName]
		if table == nil {
			continue
		}

		for _, constraint := range table.Constraints {
			if constraint.Kind != database.ForeignKey {
				continue
			}
			if !inComponent[graph.TableName(constraint.ReferenceTable)] {
				continue
			}

			// For each column in the FK
			for _, col := range constraint.Columns {
				relations = append(relations, fkRelation{
					fromTable:  tableName,
					fromColumn: col,
					toTable:    graph.TableName(constraint.ReferenceTable),
				})
			}
		}
	}

	// Draw each relationship
	arrowX := layouts[component[0]].boxWidth + 2 // Start arrows 2 spaces after box edge
	for i, rel := range relations {
		fromLayout := layouts[rel.fromTable]
		toLayout := layouts[rel.toTable]
		if fromLayout == nil || toLayout == nil {
			continue
		}

		// Find the line where the FK column is
		fromLineOffset, ok := fromLayout.columnLines[rel.fromColumn]
		if !ok {
			continue
		}
		fromY := fromLayout.y + fromLineOffset

		// Determine target Y (middle of target table)
		toY := toLayout.y + toLayout.height/2

		// Use different X offsets for different arrows to avoid overlap
		currentArrowX := arrowX + (i * 2)

		// Draw horizontal line from box edge to arrow column
		for x := fromLayout.boxWidth; x <= currentArrowX; x++ {
			if canvas.get(x, fromY) == ' ' {
				if x == fromLayout.boxWidth {
					canvas.set(x, fromY, '─')
				} else if x == currentArrowX {
					if fromY < toY {
						canvas.set(x, fromY, '┐')
					} else if fromY > toY {
						canvas.set(x, fromY, '┘')
					} else {
						canvas.set(x, fromY, '►')
					}
				} else {
					canvas.set(x, fromY, '─')
				}
			}
		}

		// Draw vertical line
		if fromY != toY {
			startY := fromY
			endY := toY
			if startY > endY {
				startY, endY = endY, startY
			}
			for y := startY; y <= endY; y++ {
				if y == fromY || y == toY {
					continue // Already handled by corners
				}
				if canvas.get(currentArrowX, y) == ' ' || canvas.get(currentArrowX, y) == '│' {
					canvas.set(currentArrowX, y, '│')
				}
			}

			// Draw corner at target end and arrow pointing to target
			if fromY < toY {
				canvas.set(currentArrowX, toY, '└')
			} else {
				canvas.set(currentArrowX, toY, '┌')
			}

			// Draw arrow pointing left to target
			for x := currentArrowX - 1; x >= 0; x-- {
				ch := canvas.get(x, toY)
				if ch == '│' || ch == '┤' || ch == '├' {
					// Hit the target box
					canvas.set(x, toY, '◄')
					break
				} else if ch == ' ' {
					canvas.set(x, toY, '─')
				}
			}
		}
	}
}

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

func findConnectedComponents(g *graph.SchemaGraph) [][]graph.TableName {
	visited := make(map[graph.TableName]bool)
	var components [][]graph.TableName

	// Build adjacency list (undirected)
	neighbors := make(map[graph.TableName][]graph.TableName)
	for tableName := range g.Nodes {
		neighbors[tableName] = []graph.TableName{}
	}

	for _, edge := range g.Edges {
		neighbors[edge.FromTable] = append(neighbors[edge.FromTable], edge.ToTable)
		neighbors[edge.ToTable] = append(neighbors[edge.ToTable], edge.FromTable)
	}

	// DFS to find components
	var dfs func(graph.TableName, *[]graph.TableName)
	dfs = func(table graph.TableName, component *[]graph.TableName) {
		visited[table] = true
		*component = append(*component, table)

		for _, neighbor := range neighbors[table] {
			if !visited[neighbor] {
				dfs(neighbor, component)
			}
		}
	}

	// Sort table names for deterministic order
	sortedTables := getSortedTableNames(g)

	for _, tableName := range sortedTables {
		if !visited[tableName] {
			component := []graph.TableName{}
			dfs(tableName, &component)
			components = append(components, component)
		}
	}

	return components
}

func orderTablesInComponent(g *graph.SchemaGraph, component []graph.TableName) []graph.TableName {
	// Build component set for quick lookup
	inComponent := make(map[graph.TableName]bool)
	for _, t := range component {
		inComponent[t] = true
	}

	// Count incoming edges within component
	inDegree := make(map[graph.TableName]int)
	adjList := make(map[graph.TableName][]graph.TableName)

	for _, t := range component {
		inDegree[t] = 0
		adjList[t] = []graph.TableName{}
	}

	for _, edge := range g.Edges {
		if inComponent[edge.FromTable] && inComponent[edge.ToTable] {
			inDegree[edge.FromTable]++
			adjList[edge.ToTable] = append(adjList[edge.ToTable], edge.FromTable)
		}
	}

	// Topological sort (Kahn's algorithm)
	var queue []graph.TableName
	for _, t := range component {
		if inDegree[t] == 0 {
			queue = append(queue, t)
		}
	}

	// Sort queue for deterministic order
	sort.Slice(queue, func(i, j int) bool {
		return queue[i] < queue[j]
	})

	var result []graph.TableName
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		for _, neighbor := range adjList[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
				// Keep queue sorted
				sort.Slice(queue, func(i, j int) bool {
					return queue[i] < queue[j]
				})
			}
		}
	}

	// If we have a cycle, add remaining tables
	if len(result) < len(component) {
		for _, t := range component {
			found := false
			for _, r := range result {
				if r == t {
					found = true
					break
				}
			}
			if !found {
				result = append(result, t)
			}
		}
	}

	return result
}

func findCircularReferences(g *graph.SchemaGraph, component []graph.TableName) map[graph.TableName]bool {
	circular := make(map[graph.TableName]bool)
	inComponent := make(map[graph.TableName]bool)

	for _, t := range component {
		inComponent[t] = true
	}

	// Use DFS to detect cycles
	visited := make(map[graph.TableName]bool)
	recStack := make(map[graph.TableName]bool)

	var dfs func(graph.TableName) bool
	dfs = func(table graph.TableName) bool {
		visited[table] = true
		recStack[table] = true

		// Check all outgoing edges
		for _, edge := range g.Edges {
			if edge.ToTable == table && inComponent[edge.FromTable] {
				if !visited[edge.FromTable] {
					if dfs(edge.FromTable) {
						circular[edge.FromTable] = true
						return true
					}
				} else if recStack[edge.FromTable] {
					circular[edge.FromTable] = true
					return true
				}
			}
		}

		recStack[table] = false
		return false
	}

	for _, t := range component {
		if !visited[t] {
			dfs(t)
		}
	}

	return circular
}

func getOutgoingReferences(g *graph.SchemaGraph, tableName graph.TableName, component []graph.TableName) []graph.TableName {
	inComponent := make(map[graph.TableName]bool)
	for _, t := range component {
		inComponent[t] = true
	}

	var refs []graph.TableName
	for _, edge := range g.Edges {
		if edge.ToTable == tableName && inComponent[edge.FromTable] {
			refs = append(refs, edge.FromTable)
		}
	}

	return refs
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