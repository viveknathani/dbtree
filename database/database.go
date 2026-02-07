// Package database provides schema inspection functionality for PostgreSQL and other databases.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ConstraintKind represents the type of database constraint.
type ConstraintKind string

// DataType represents a database column data type.
type DataType string

const (
	PrimaryKey ConstraintKind = "PRIMARY_KEY"
	ForeignKey ConstraintKind = "FOREIGN_KEY"
	Unique     ConstraintKind = "UNIQUE"
	Check      ConstraintKind = "CHECK"
)

// Constraint represents a database table constraint including primary keys, foreign keys,
// unique constraints, and check constraints.
type Constraint struct {
	Kind             ConstraintKind
	Columns          []string
	ReferenceTable   string
	ReferenceColumns []string
	CheckExpression  string
}

// Column represents a database table column with its properties.
type Column struct {
	Name         string
	Type         DataType
	IsNullable   bool
	DefaultValue string
}

// Table represents a database table with its columns and constraints.
type Table struct {
	Name        string
	Columns     []Column
	Constraints []Constraint
}

// Database represents a database schema with all its tables.
type Database struct {
	Name   string
	Tables []Table
}

// SchemaInspector defines the interface for database schema inspection implementations.
type SchemaInspector interface {
	InspectSchema(ctx context.Context, db *sql.DB) (*Database, error)
}

// detectDatabaseType determines the database type by querying the version string.
// It supports PostgreSQL, MySQL, and ClickHouse detection.
func detectDatabaseType(ctx context.Context, db *sql.DB) (string, error) {
	// Try PostgreSQL version query first
	var version string
	err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err == nil {
		lowerVersion := strings.ToLower(version)
		if strings.Contains(lowerVersion, "postgresql") || strings.Contains(lowerVersion, "postgres") {
			return "postgres", nil
		}
		if strings.Contains(lowerVersion, "clickhouse") {
			return "clickhouse", nil
		}
	}

	// Try MySQL specific query
	err = db.QueryRowContext(ctx, "SELECT @@version_comment").Scan(&version)
	if err == nil {
		lowerVersion := strings.ToLower(version)
		if strings.Contains(lowerVersion, "mysql") {
			return "mysql", nil
		}
	}

	// Try ClickHouse specific query
	err = db.QueryRowContext(ctx, "SELECT name FROM system.databases LIMIT 1").Scan(&version)
	if err == nil {
		return "clickhouse", nil
	}

	return "", fmt.Errorf("could not determine database type")
}

// InspectSchema analyzes a database connection and returns a complete schema representation.
// It automatically detects the database type and uses the appropriate inspector implementation.
func InspectSchema(ctx context.Context, db *sql.DB) (*Database, error) {
	dbType, err := detectDatabaseType(ctx, db)
	if err != nil {
		return nil, err
	}

	var inspector SchemaInspector
	switch dbType {
	case "postgres":
		inspector = &postgresInspector{}
	case "mysql":
		inspector = &mysqlInspector{}
	case "clickhouse":
		inspector = &clickhouseInspector{}
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	return inspector.InspectSchema(ctx, db)
}
