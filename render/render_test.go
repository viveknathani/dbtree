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

	g := graph.Build(db)

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
				"user_id (int) → users.id",
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
