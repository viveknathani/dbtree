package graph

import (
	"reflect"
	"testing"

	"github.com/viveknathani/dbtree/database"
)

func TestBuild(t *testing.T) {
	tests := []struct {
		name     string
		db       *database.Database
		expected *SchemaGraph
	}{
		{
			name: "empty database",
			db: &database.Database{
				Name:   "test_db",
				Tables: []database.Table{},
			},
			expected: &SchemaGraph{
				DatabaseName: "test_db",
				Nodes:        map[TableName]*database.Table{},
				Edges:        []ForeignKeyEdge{},
			},
		},
		{
			name: "single table no constraints",
			db: &database.Database{
				Name: "test_db",
				Tables: []database.Table{
					{
						Name: "users",
						Columns: []database.Column{
							{Name: "id", Type: "integer", IsNullable: false},
							{Name: "name", Type: "varchar", IsNullable: true},
						},
						Constraints: []database.Constraint{},
					},
				},
			},
			expected: &SchemaGraph{
				DatabaseName: "test_db",
				Nodes: map[TableName]*database.Table{
					"users": {
						Name: "users",
						Columns: []database.Column{
							{Name: "id", Type: "integer", IsNullable: false},
							{Name: "name", Type: "varchar", IsNullable: true},
						},
						Constraints: []database.Constraint{},
					},
				},
				Edges: []ForeignKeyEdge{},
			},
		},
		{
			name: "multiple tables with foreign key",
			db: &database.Database{
				Name: "test_db",
				Tables: []database.Table{
					{
						Name: "users",
						Columns: []database.Column{
							{Name: "id", Type: "integer", IsNullable: false},
						},
						Constraints: []database.Constraint{
							{Kind: database.PrimaryKey, Columns: []string{"id"}},
						},
					},
					{
						Name: "orders",
						Columns: []database.Column{
							{Name: "id", Type: "integer", IsNullable: false},
							{Name: "user_id", Type: "integer", IsNullable: false},
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
				},
			},
			expected: &SchemaGraph{
				DatabaseName: "test_db",
				Nodes: map[TableName]*database.Table{
					"users": {
						Name: "users",
						Columns: []database.Column{
							{Name: "id", Type: "integer", IsNullable: false},
						},
						Constraints: []database.Constraint{
							{Kind: database.PrimaryKey, Columns: []string{"id"}},
						},
					},
					"orders": {
						Name: "orders",
						Columns: []database.Column{
							{Name: "id", Type: "integer", IsNullable: false},
							{Name: "user_id", Type: "integer", IsNullable: false},
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
				},
				Edges: []ForeignKeyEdge{
					{
						FromTable:        "orders",
						ToTable:          "users",
						Columns:          []string{"user_id"},
						ReferenceColumns: []string{"id"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Build(tt.db)
			if err != nil {
				t.Errorf("Build failed: %v", err)
				return
			}

			if result.DatabaseName != tt.expected.DatabaseName {
				t.Errorf("DatabaseName = %v, want %v", result.DatabaseName, tt.expected.DatabaseName)
			}
			if len(result.Nodes) != len(tt.expected.Nodes) {
				t.Errorf("Nodes length = %v, want %v", len(result.Nodes), len(tt.expected.Nodes))
			}

			for name, expectedTable := range tt.expected.Nodes {
				actualTable, exists := result.Nodes[name]
				if !exists {
					t.Errorf("Node %v not found", name)
					continue
				}
				if !reflect.DeepEqual(actualTable, expectedTable) {
					t.Errorf("Node %v = %+v, want %+v", name, actualTable, expectedTable)
				}
			}

			if len(result.Edges) != len(tt.expected.Edges) {
				t.Errorf("Edges length = %v, want %v", len(result.Edges), len(tt.expected.Edges))
			} else if len(result.Edges) > 0 && !reflect.DeepEqual(result.Edges, tt.expected.Edges) {
				t.Errorf("Edges = %+v, want %+v", result.Edges, tt.expected.Edges)
			}
		})
	}
}
