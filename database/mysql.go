package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// mysqlInspector implements SchemaInspector for MySQL databases.
type mysqlInspector struct{}

// InspectSchema inspects a MySQL database and returns its complete schema.
// It retrieves all tables, columns, and constraints from the current database.
func (m *mysqlInspector) InspectSchema(ctx context.Context, db *sql.DB) (*Database, error) {
	dbName, err := m.getDatabaseName(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get database name: %w", err)
	}

	tables, err := m.getTables(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	allColumns, err := m.getAllColumns(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get all columns: %w", err)
	}

	allConstraints, err := m.getAllConstraints(ctx, db)
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

// getDatabaseName retrieves the current database name from MySQL.
func (m *mysqlInspector) getDatabaseName(ctx context.Context, db *sql.DB) (string, error) {
	var dbName string
	err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&dbName)
	return dbName, err
}

// getTables retrieves all tables from the current database.
func (m *mysqlInspector) getTables(ctx context.Context, db *sql.DB) ([]Table, error) {
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = DATABASE()
		  AND table_type = 'BASE TABLE'
		ORDER BY table_name
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
func (m *mysqlInspector) getAllColumns(ctx context.Context, db *sql.DB) (map[string][]Column, error) {
	query := `
		SELECT
		  table_name,
		  column_name,
		  column_type,
		  is_nullable,
		  column_default
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		ORDER BY table_name, ordinal_position
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
		var isNullable string
		var columnDefault sql.NullString

		if err := rows.Scan(&tableName, &columnName, &columnType, &isNullable, &columnDefault); err != nil {
			return nil, err
		}

		column := Column{
			Name:       columnName,
			Type:       DataType(columnType),
			IsNullable: isNullable == "YES",
		}

		if columnDefault.Valid {
			column.DefaultValue = columnDefault.String
		}

		columns[tableName] = append(columns[tableName], column)
	}

	return columns, rows.Err()
}

// getAllConstraints retrieves all constraints (primary keys, foreign keys, unique, check) for all tables.
// Returns a map of table name to constraint list.
func (m *mysqlInspector) getAllConstraints(ctx context.Context, db *sql.DB) (map[string][]Constraint, error) {
	constraints := make(map[string][]Constraint)

	pks, err := m.getPrimaryKeys(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary keys: %w", err)
	}
	for tableName, constraint := range pks {
		constraints[tableName] = append(constraints[tableName], constraint)
	}

	fks, err := m.getForeignKeys(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %w", err)
	}
	for tableName, tableConstraints := range fks {
		constraints[tableName] = append(constraints[tableName], tableConstraints...)
	}

	uniques, err := m.getUniqueConstraints(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique constraints: %w", err)
	}
	for tableName, tableConstraints := range uniques {
		constraints[tableName] = append(constraints[tableName], tableConstraints...)
	}

	// Get check constraints (MySQL 8.0.16+)
	checks, err := m.getCheckConstraints(ctx, db)
	if err != nil {
		// Check constraints might not be available in older MySQL versions
		// Log but don't fail
	} else {
		for tableName, tableConstraints := range checks {
			constraints[tableName] = append(constraints[tableName], tableConstraints...)
		}
	}

	return constraints, nil
}

// getPrimaryKeys retrieves all primary key constraints.
func (m *mysqlInspector) getPrimaryKeys(ctx context.Context, db *sql.DB) (map[string]Constraint, error) {
	query := `
		SELECT
		  table_name,
		  column_name
		FROM information_schema.key_column_usage
		WHERE table_schema = DATABASE()
		  AND constraint_name = 'PRIMARY'
		ORDER BY table_name, ordinal_position
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

// getForeignKeys retrieves all foreign key constraints.
func (m *mysqlInspector) getForeignKeys(ctx context.Context, db *sql.DB) (map[string][]Constraint, error) {
	query := `
		SELECT
		  kcu.table_name,
		  kcu.column_name,
		  kcu.referenced_table_name,
		  kcu.referenced_column_name,
		  kcu.constraint_name
		FROM information_schema.key_column_usage kcu
		WHERE kcu.table_schema = DATABASE()
		  AND kcu.referenced_table_name IS NOT NULL
		ORDER BY kcu.table_name, kcu.constraint_name, kcu.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type constraintKey struct {
		tableName      string
		constraintName string
	}
	fkMap := make(map[constraintKey]Constraint)

	for rows.Next() {
		var tableName string
		var columnName string
		var refTableName string
		var refColumnName string
		var constraintName string

		if err := rows.Scan(&tableName, &columnName, &refTableName, &refColumnName, &constraintName); err != nil {
			return nil, err
		}

		key := constraintKey{tableName: tableName, constraintName: constraintName}
		constraint, exists := fkMap[key]
		if !exists {
			constraint = Constraint{
				Kind:           ForeignKey,
				ReferenceTable: refTableName,
			}
		}
		constraint.Columns = append(constraint.Columns, columnName)
		constraint.ReferenceColumns = append(constraint.ReferenceColumns, refColumnName)
		fkMap[key] = constraint
	}

	result := make(map[string][]Constraint)
	for key, constraint := range fkMap {
		result[key.tableName] = append(result[key.tableName], constraint)
	}

	return result, rows.Err()
}

// getUniqueConstraints retrieves all unique constraints.
func (m *mysqlInspector) getUniqueConstraints(ctx context.Context, db *sql.DB) (map[string][]Constraint, error) {
	query := `
		SELECT
		  tc.table_name,
		  tc.constraint_name,
		  kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name
		  AND tc.table_schema = kcu.table_schema
		WHERE tc.table_schema = DATABASE()
		  AND tc.constraint_type = 'UNIQUE'
		ORDER BY tc.table_name, tc.constraint_name, kcu.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type constraintKey struct {
		tableName      string
		constraintName string
	}
	uniqueMap := make(map[constraintKey]Constraint)

	for rows.Next() {
		var tableName string
		var constraintName string
		var columnName string

		if err := rows.Scan(&tableName, &constraintName, &columnName); err != nil {
			return nil, err
		}

		key := constraintKey{tableName: tableName, constraintName: constraintName}
		constraint, exists := uniqueMap[key]
		if !exists {
			constraint = Constraint{Kind: Unique}
		}
		constraint.Columns = append(constraint.Columns, columnName)
		uniqueMap[key] = constraint
	}

	// Convert to map[tableName][]Constraint
	result := make(map[string][]Constraint)
	for key, constraint := range uniqueMap {
		result[key.tableName] = append(result[key.tableName], constraint)
	}

	return result, rows.Err()
}

// getCheckConstraints retrieves all check constraints (MySQL 8.0.16+).
func (m *mysqlInspector) getCheckConstraints(ctx context.Context, db *sql.DB) (map[string][]Constraint, error) {
	query := `
		SELECT
		  cc.table_name,
		  cc.constraint_name,
		  cc.check_clause
		FROM information_schema.check_constraints cc
		JOIN information_schema.table_constraints tc
		  ON cc.constraint_name = tc.constraint_name
		  AND cc.constraint_schema = tc.table_schema
		WHERE cc.constraint_schema = DATABASE()
		ORDER BY cc.table_name
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		// Check constraints might not be available
		return make(map[string][]Constraint), nil
	}
	defer rows.Close()

	result := make(map[string][]Constraint)
	for rows.Next() {
		var tableName string
		var constraintName string
		var checkClause string

		if err := rows.Scan(&tableName, &constraintName, &checkClause); err != nil {
			return nil, err
		}

		// MySQL's check_clause includes the parentheses, so we'll keep it as-is
		// Example: "(length(`title`) > 0)"
		// Remove extra parentheses and backticks for cleaner display
		checkClause = strings.TrimPrefix(checkClause, "(")
		checkClause = strings.TrimSuffix(checkClause, ")")
		checkClause = strings.ReplaceAll(checkClause, "`", "")

		constraint := Constraint{
			Kind:            Check,
			CheckExpression: checkClause,
		}
		result[tableName] = append(result[tableName], constraint)
	}

	return result, rows.Err()
}
