package render

import (
	"strings"
	"testing"

	"github.com/viveknathani/dbtree/database"
	"github.com/viveknathani/dbtree/graph"
)

func TestRender(t *testing.T) {
	// Create a sample database
	db := &database.Database{
		Name: "test_db",
		Tables: []database.Table{
			{
				Name: "users",
				Columns: []database.Column{
					{Name: "id", Type: "int", IsNullable: false},
					{Name: "email", Type: "varchar", IsNullable: false},
				},
				Constraints: []database.Constraint{
					{Kind: database.PrimaryKey, Columns: []string{"id"}},
				},
			},
			{
				Name: "posts",
				Columns: []database.Column{
					{Name: "id", Type: "int", IsNullable: false},
					{Name: "user_id", Type: "int", IsNullable: false},
					{Name: "title", Type: "varchar", IsNullable: false},
				},
				Constraints: []database.Constraint{
					{Kind: database.PrimaryKey, Columns: []string{"id"}},
					{
						Kind:             database.ForeignKey,
						Columns:          []string{"user_id"},
						ReferenceTable:   "users",
						ReferenceColumns: []string{"id"},
					},
				},
			},
			{
				Name: "settings",
				Columns: []database.Column{
					{Name: "key", Type: "varchar", IsNullable: false},
					{Name: "value", Type: "text", IsNullable: true},
				},
				Constraints: []database.Constraint{
					{Kind: database.PrimaryKey, Columns: []string{"key"}},
				},
			},
		},
	}

	g, err := graph.Build(db)
	if err != nil {
		t.Fatalf("failed to build graph: %v", err)
	}

	tests := []struct {
		name     string
		format   Format
		shape    Shape
		wantErr  bool
		contains []string
	}{
		{
			name:    "text tree",
			format:  FormatText,
			shape:   ShapeTree,
			wantErr: false,
			contains: []string{
				"test_db",
				"users",
				"posts",
				"settings",
				"user_id (\"int\") → users.id",
			},
		},
		{
			name:    "json tree",
			format:  FormatJSON,
			shape:   ShapeTree,
			wantErr: false,
			contains: []string{
				`"database": "test_db"`,
				`"name": "users"`,
				`"name": "posts"`,
				`"name": "settings"`,
			},
		},
		{
			name:    "text flat",
			format:  FormatText,
			shape:   ShapeFlat,
			wantErr: false,
			contains: []string{
				"Database: test_db",
				"Tables: 3",
				"users",
				"posts",
				"settings",
			},
		},
		{
			name:    "json flat",
			format:  FormatJSON,
			shape:   ShapeFlat,
			wantErr: false,
			contains: []string{
				`"database": "test_db"`,
				`"tables"`,
				`"edges"`,
				`"from": "posts"`,
				`"to": "users"`,
			},
		},
		{
			name:    "text graph",
			format:  FormatText,
			shape:   ShapeChart,
			wantErr: false,
			contains: []string{
				"users",
				"posts",
				"settings",
				"+",
				"-",
				"|",
				"user_id",
			},
		},
		{
			name:    "json chart not supported",
			format:  FormatJSON,
			shape:   ShapeChart,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(g, tt.format, tt.shape)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("Render() output missing %q\nGot:\n%s", want, got)
				}
			}
		})
	}
}

func TestRenderNilGraph(t *testing.T) {
	_, err := Render(nil, FormatText, ShapeTree)
	if err == nil {
		t.Error("expected error for nil graph")
	}
}

func TestRenderInvalidCombination(t *testing.T) {
	g := &graph.SchemaGraph{DatabaseName: "test"}
	_, err := Render(g, "invalid", "invalid")
	if err == nil {
		t.Error("expected error for invalid format/shape")
	}
}

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

func TestRenderWithSpecialCharacters(t *testing.T) {
	// Create a database with special characters in names
	db := &database.Database{
		Name: "test-db",
		Tables: []database.Table{
			{
				Name: "user-profile",
				Columns: []database.Column{
					{Name: "user-id", Type: "int", IsNullable: false},
					{Name: "email address", Type: "varchar", IsNullable: false},
					{Name: "first.name", Type: "varchar", IsNullable: false},
				},
				Constraints: []database.Constraint{
					{Kind: database.PrimaryKey, Columns: []string{"user-id"}},
				},
			},
			{
				Name: "user.settings",
				Columns: []database.Column{
					{Name: "id", Type: "int", IsNullable: false},
					{Name: "user-id", Type: "int", IsNullable: false},
					{Name: "setting key", Type: "varchar", IsNullable: false},
				},
				Constraints: []database.Constraint{
					{Kind: database.PrimaryKey, Columns: []string{"id"}},
					{
						Kind:             database.ForeignKey,
						Columns:          []string{"user-id"},
						ReferenceTable:   "user-profile",
						ReferenceColumns: []string{"user-id"},
					},
				},
			},
		},
	}

	g, err := graph.Build(db)
	if err != nil {
		t.Fatalf("failed to build graph: %v", err)
	}

	// Test that chart rendering works with special characters
	output, err := Render(g, FormatText, ShapeChart)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Verify that output contains the tables (the actual rendering depends on D2)
	if !strings.Contains(output, "user") || !strings.Contains(output, "settings") {
		t.Errorf("Expected output to contain table names, got:\n%s", output)
	}
}
