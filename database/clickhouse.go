package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// clickhouseInspector implements SchemaInspector for ClickHouse databases.
type clickhouseInspector struct{}

// InspectSchema inspects a ClickHouse database and returns its complete schema.
// It retrieves all tables, columns, and primary keys from the current database.
// Note: ClickHouse does not enforce foreign keys, so they are not included.
func (c *clickhouseInspector) InspectSchema(ctx context.Context, db *sql.DB) (*Database, error) {
	dbName, err := c.getDatabaseName(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get database name: %w", err)
	}

	tables, err := c.getTables(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	allColumns, err := c.getAllColumns(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get all columns: %w", err)
	}

	allConstraints, err := c.getAllConstraints(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get all constraints: %w", err)
	}

	for i := range tables {
		tableName := tables[i].Name
		tables[i].Columns = allColumns[tableName]
		tables[i].Constraints = allConstraints[tableName]
	}

	return &Database{
		Name:   dbName,
		Tables: tables,
	}, nil
}

// getDatabaseName retrieves the current database name from ClickHouse.
func (c *clickhouseInspector) getDatabaseName(ctx context.Context, db *sql.DB) (string, error) {
	var dbName string
	err := db.QueryRowContext(ctx, "SELECT currentDatabase()").Scan(&dbName)
	return dbName, err
}

// getTables retrieves all tables from the current database.
// Excludes views and dictionary tables.
func (c *clickhouseInspector) getTables(ctx context.Context, db *sql.DB) ([]Table, error) {
	query := `
		SELECT name
		FROM system.tables
		WHERE database = currentDatabase()
		  AND engine NOT LIKE '%View%'
		  AND engine NOT LIKE 'Dictionary%'
		ORDER BY name
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, Table{Name: tableName})
	}

	return tables, rows.Err()
}

// getAllColumns retrieves all column information for all tables in the current database.
// Returns a map of table name to column list for efficient lookup.
func (c *clickhouseInspector) getAllColumns(ctx context.Context, db *sql.DB) (map[string][]Column, error) {
	query := `
		SELECT
		  table,
		  name,
		  type,
		  default_kind,
		  default_expression
		FROM system.columns
		WHERE database = currentDatabase()
		ORDER BY table, position
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string][]Column)
	for rows.Next() {
		var tableName string
		var columnName string
		var columnType string
		var defaultKind string
		var defaultExpression string

		if err := rows.Scan(&tableName, &columnName, &columnType, &defaultKind, &defaultExpression); err != nil {
			return nil, err
		}

		column := Column{
			Name:       columnName,
			Type:       DataType(c.formatClickHouseType(columnType)),
			IsNullable: strings.HasPrefix(columnType, "Nullable("),
		}

		if defaultKind != "" && defaultExpression != "" {
			column.DefaultValue = fmt.Sprintf("%s: %s", defaultKind, defaultExpression)
		}

		columns[tableName] = append(columns[tableName], column)
	}

	return columns, rows.Err()
}

// getAllConstraints retrieves all constraints for all tables.
// For ClickHouse, this primarily means primary keys.
// ClickHouse does not enforce foreign keys or unique constraints.
func (c *clickhouseInspector) getAllConstraints(ctx context.Context, db *sql.DB) (map[string][]Constraint, error) {
	constraints := make(map[string][]Constraint)

	pks, err := c.getPrimaryKeys(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary keys: %w", err)
	}
	for tableName, constraint := range pks {
		constraints[tableName] = append(constraints[tableName], constraint)
	}

	return constraints, nil
}

// getPrimaryKeys retrieves all primary key constraints.
// In ClickHouse, primary keys are indicated by the is_in_primary_key column.
func (c *clickhouseInspector) getPrimaryKeys(ctx context.Context, db *sql.DB) (map[string]Constraint, error) {
	query := `
		SELECT
		  table,
		  name
		FROM system.columns
		WHERE database = currentDatabase()
		  AND is_in_primary_key = 1
		ORDER BY table, position
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pks := make(map[string]Constraint)
	for rows.Next() {
		var tableName string
		var columnName string
		if err := rows.Scan(&tableName, &columnName); err != nil {
			return nil, err
		}

		constraint, exists := pks[tableName]
		if !exists {
			constraint = Constraint{Kind: PrimaryKey}
		}
		constraint.Columns = append(constraint.Columns, columnName)
		pks[tableName] = constraint
	}

	return pks, rows.Err()
}

// formatClickHouseType formats ClickHouse types for display.
// ClickHouse has types like: UInt64, String, Nullable(String), DateTime64(3), Array(String), etc.
func (c *clickhouseInspector) formatClickHouseType(columnType string) string {
	// ClickHouse types are already fairly readable, just return as-is
	// We could potentially simplify Nullable(Type) to Type? but keeping it explicit for now
	return columnType
}
