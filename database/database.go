package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

type SchemaInspector interface {
	InspectSchema(ctx context.Context, db *sql.DB) (*Database, error)
}

func detectDatabaseType(ctx context.Context, db *sql.DB) (string, error) {
	var version string
	err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to detect database type: %w", err)
	}

	lowerVersion := strings.ToLower(version)
	switch {
	case strings.Contains(lowerVersion, "postgresql"):
		return "postgres", nil
	case strings.Contains(lowerVersion, "mysql"):
		return "mysql", nil
	case strings.Contains(lowerVersion, "clickhouse"):
		return "clickhouse", nil
	default:
		return "", fmt.Errorf("unsupported database type: %s", version)
	}
}

func InspectSchema(ctx context.Context, db *sql.DB) (*Database, error) {
	dbType, err := detectDatabaseType(ctx, db)
	if err != nil {
		return nil, err
	}

	var inspector SchemaInspector
	switch dbType {
	case "postgres":
		inspector = &postgresInspector{}
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	return inspector.InspectSchema(ctx, db)
}
