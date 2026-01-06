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
	Nodes map[TableName]*database.Table
	Edges []ForeignKeyEdge
}

func Build(database *database.Database) *SchemaGraph {
	panic("not implemented")
}
