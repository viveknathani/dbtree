package database

import (
	"context"
	"database/sql"
)

type ConstraintKind string

type DataType string

const (
	PrimaryKey ConstraintKind = "PRIMARY_KEY"
	ForeignKey ConstraintKind = "FOREIGN_KEY"
	Unique     ConstraintKind = "UNIQUE"
	Check      ConstraintKind = "CHECK"
)

type Constraint struct {
	Kind             ConstraintKind
	Columns          []string
	ReferenceTable   string
	ReferenceColumns []string
	CheckExpression  string
}

type Column struct {
	Name         string
	Type         DataType
	IsNullable   bool
	DefaultValue string
}

type Table struct {
	Name        string
	Column      []Column
	Constraints []Constraint
}

type Database struct {
	Name   string
	Tables []Table
}

func InspectSchema(ctx context.Context, db *sql.DB) (*Database, error) {
	panic("not implemented")
}
