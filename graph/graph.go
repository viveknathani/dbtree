package graph

import (
	"fmt"

	"github.com/viveknathani/dbtree/database"
)

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

func Build(db *database.Database) (*SchemaGraph, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}

	nodes := make(map[TableName]*database.Table)
	edges := []ForeignKeyEdge{}

	// First pass: populate nodes map
	for i := range db.Tables {
		table := &db.Tables[i]
		nodes[TableName(table.Name)] = table
	}

	// Second pass: create edges
	for i := range db.Tables {
		table := &db.Tables[i]
		for _, constraint := range table.Constraints {
			if constraint.Kind == database.ForeignKey {
				referencedTable := TableName(constraint.ReferenceTable)
				if _, exists := nodes[referencedTable]; exists {
					edge := ForeignKeyEdge{
						FromTable:        TableName(table.Name),
						ToTable:          referencedTable,
						Columns:          constraint.Columns,
						ReferenceColumns: constraint.ReferenceColumns,
					}
					edges = append(edges, edge)
				}
			}
		}
	}

	return &SchemaGraph{
		DatabaseName: db.Name,
		Nodes:        nodes,
		Edges:        edges,
	}, nil
}
