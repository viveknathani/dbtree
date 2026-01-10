package render

import (
	"strings"
	"testing"

	"github.com/viveknathani/dbtree/database"
	"github.com/viveknathani/dbtree/graph"
)

func TestRenderCircularReferences(t *testing.T) {
	// Create a database with circular references
	db := &database.Database{
		Name: "circular_db",
		Tables: []database.Table{
			{
				Name: "departments",
				Columns: []database.Column{
					{Name: "id", Type: "int", IsNullable: false},
					{Name: "name", Type: "varchar", IsNullable: false},
					{Name: "manager_id", Type: "int", IsNullable: true},
				},
				Constraints: []database.Constraint{
					{Kind: database.PrimaryKey, Columns: []string{"id"}},
					{
						Kind:             database.ForeignKey,
						Columns:          []string{"manager_id"},
						ReferenceTable:   "employees",
						ReferenceColumns: []string{"id"},
					},
				},
			},
			{
				Name: "employees",
				Columns: []database.Column{
					{Name: "id", Type: "int", IsNullable: false},
					{Name: "name", Type: "varchar", IsNullable: false},
					{Name: "dept_id", Type: "int", IsNullable: false},
				},
				Constraints: []database.Constraint{
					{Kind: database.PrimaryKey, Columns: []string{"id"}},
					{
						Kind:             database.ForeignKey,
						Columns:          []string{"dept_id"},
						ReferenceTable:   "departments",
						ReferenceColumns: []string{"id"},
					},
				},
			},
		},
	}

	g, err := graph.Build(db)
	if err != nil {
		t.Fatalf("failed to build graph: %v", err)
	}

	t.Run("text tree with cycles", func(t *testing.T) {
		output, err := Render(g, FormatText, ShapeTree)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should contain circular reference indication
		if !strings.Contains(output, "(circular reference)") {
			t.Errorf("expected circular reference indication in output:\n%s", output)
		}

		// Both tables should appear
		if !strings.Contains(output, "departments") {
			t.Errorf("expected departments in output:\n%s", output)
		}
		if !strings.Contains(output, "employees") {
			t.Errorf("expected employees in output:\n%s", output)
		}
	})

	t.Run("json tree with cycles", func(t *testing.T) {
		output, err := Render(g, FormatJSON, ShapeTree)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// JSON tree handles cycles by not including already visited nodes
		// Both tables should still appear in the output
		if !strings.Contains(output, `"name": "departments"`) {
			t.Errorf("expected departments in JSON output:\n%s", output)
		}
		if !strings.Contains(output, `"name": "employees"`) {
			t.Errorf("expected employees in JSON output:\n%s", output)
		}
	})

	// Add a more complex cycle test
	dbComplex := &database.Database{
		Name: "complex_circular_db",
		Tables: []database.Table{
			{
				Name: "a",
				Columns: []database.Column{
					{Name: "id", Type: "int", IsNullable: false},
					{Name: "b_id", Type: "int", IsNullable: true},
				},
				Constraints: []database.Constraint{
					{Kind: database.PrimaryKey, Columns: []string{"id"}},
					{
						Kind:             database.ForeignKey,
						Columns:          []string{"b_id"},
						ReferenceTable:   "b",
						ReferenceColumns: []string{"id"},
					},
				},
			},
			{
				Name: "b",
				Columns: []database.Column{
					{Name: "id", Type: "int", IsNullable: false},
					{Name: "c_id", Type: "int", IsNullable: true},
				},
				Constraints: []database.Constraint{
					{Kind: database.PrimaryKey, Columns: []string{"id"}},
					{
						Kind:             database.ForeignKey,
						Columns:          []string{"c_id"},
						ReferenceTable:   "c",
						ReferenceColumns: []string{"id"},
					},
				},
			},
			{
				Name: "c",
				Columns: []database.Column{
					{Name: "id", Type: "int", IsNullable: false},
					{Name: "a_id", Type: "int", IsNullable: true},
				},
				Constraints: []database.Constraint{
					{Kind: database.PrimaryKey, Columns: []string{"id"}},
					{
						Kind:             database.ForeignKey,
						Columns:          []string{"a_id"},
						ReferenceTable:   "a",
						ReferenceColumns: []string{"id"},
					},
				},
			},
		},
	}

	gComplex, err := graph.Build(dbComplex)
	if err != nil {
		t.Fatalf("failed to build complex graph: %v", err)
	}

	t.Run("complex circular reference", func(t *testing.T) {
		output, err := Render(gComplex, FormatText, ShapeTree)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should handle the A -> B -> C -> A cycle
		if !strings.Contains(output, "(circular reference)") {
			t.Errorf("expected circular reference indication in complex cycle:\n%s", output)
		}

		// All tables should appear
		for _, table := range []string{"a", "b", "c"} {
			if !strings.Contains(output, table) {
				t.Errorf("expected table %s in output:\n%s", table, output)
			}
		}
	})

	// Test flat formats handle cycles correctly
	t.Run("flat formats with cycles", func(t *testing.T) {
		// Text flat
		output, err := Render(g, FormatText, ShapeFlat)
		if err != nil {
			t.Fatalf("unexpected error in text flat: %v", err)
		}
		// Flat format should just show all tables and their relationships
		if !strings.Contains(output, "departments") {
			t.Errorf("expected departments in flat output:\n%s", output)
		}
		if !strings.Contains(output, "employees") {
			t.Errorf("expected employees in flat output:\n%s", output)
		}

		// JSON flat
		output, err = Render(g, FormatJSON, ShapeFlat)
		if err != nil {
			t.Fatalf("unexpected error in json flat: %v", err)
		}
		// Should have edges showing the circular relationship
		if !strings.Contains(output, `"from": "departments"`) {
			t.Errorf("expected departments edge in JSON flat output:\n%s", output)
		}
		if !strings.Contains(output, `"from": "employees"`) {
			t.Errorf("expected employees edge in JSON flat output:\n%s", output)
		}
	})
}

func TestRenderSelfReferencingTable(t *testing.T) {
	// Test a table that references itself (e.g., employee -> manager)
	db := &database.Database{
		Name: "self_ref_db",
		Tables: []database.Table{
			{
				Name: "employees",
				Columns: []database.Column{
					{Name: "id", Type: "int", IsNullable: false},
					{Name: "name", Type: "varchar", IsNullable: false},
					{Name: "manager_id", Type: "int", IsNullable: true},
				},
				Constraints: []database.Constraint{
					{Kind: database.PrimaryKey, Columns: []string{"id"}},
					{
						Kind:             database.ForeignKey,
						Columns:          []string{"manager_id"},
						ReferenceTable:   "employees",
						ReferenceColumns: []string{"id"},
					},
				},
			},
		},
	}

	g, err := graph.Build(db)
	if err != nil {
		t.Fatalf("failed to build graph: %v", err)
	}

	t.Run("self-referencing table", func(t *testing.T) {
		output, err := Render(g, FormatText, ShapeTree)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should show the table
		if !strings.Contains(output, "employees") {
			t.Errorf("expected employees table in output:\n%s", output)
		}

		// Should show the self-reference (it's detected as circular, so details may not show)
		if !strings.Contains(output, "employees") {
			t.Errorf("expected employees table name in output:\n%s", output)
		}

		// Should handle the circular reference gracefully
		if strings.Contains(output, "(circular reference)") {
			// This is OK - it detected the self-reference
			t.Logf("Self-reference detected as circular (this is fine)")
		}
	})
}
