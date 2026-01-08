package graph

import "github.com/viveknathani/dbtree/database"

type TableName string

type ForeignKeyEdge struct {
	FromTable        TableName
	ToTable          TableName
	Columns          []string
	ReferenceColumns []string
}

type SchemaGraph struct {
	DatabaseName string
	Nodes        map[TableName]*database.Table
	Edges        []ForeignKeyEdge
}

func Build(db *database.Database) *SchemaGraph {
	nodes := make(map[TableName]*database.Table)
	edges := []ForeignKeyEdge{}

	for i := range db.Tables {
		table := &db.Tables[i]
		nodes[TableName(table.Name)] = table

		for _, constraint := range table.Constraints {
			if constraint.Kind == database.ForeignKey {
				edge := ForeignKeyEdge{
					FromTable:        TableName(table.Name),
					ToTable:          TableName(constraint.ReferenceTable),
					Columns:          constraint.Columns,
					ReferenceColumns: constraint.ReferenceColumns,
				}
				edges = append(edges, edge)
			}
		}
	}

	return &SchemaGraph{
		DatabaseName: db.Name,
		Nodes:        nodes,
		Edges:        edges,
	}
}
