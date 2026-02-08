package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// sqliteInspector implements SchemaInspector for SQLite databases.
type sqliteInspector struct{}

// InspectSchema inspects a SQLite database and returns its complete schema.
// It retrieves all tables, columns, and constraints.
func (s *sqliteInspector) InspectSchema(ctx context.Context, db *sql.DB) (*Database, error) {
	dbName, err := s.getDatabaseName(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get database name: %w", err)
	}

	tables, err := s.getTables(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	for i := range tables {
		tableName := tables[i].Name

		columns, err := s.getColumns(ctx, db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get columns for table %s: %w", tableName, err)
		}
		tables[i].Columns = columns

		constraints, err := s.getConstraints(ctx, db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get constraints for table %s: %w", tableName, err)
		}
		tables[i].Constraints = constraints
	}

	return &Database{
		Name:   dbName,
		Tables: tables,
	}, nil
}

// getDatabaseName returns a default name since SQLite doesn't have named databases.
func (s *sqliteInspector) getDatabaseName(ctx context.Context, db *sql.DB) (string, error) {
	// SQLite doesn't have database names in the same way as other DBs
	// We could try to get the filename, but for now we'll use a default
	return "main", nil
}

// getTables retrieves all tables from the SQLite database.
func (s *sqliteInspector) getTables(ctx context.Context, db *sql.DB) ([]Table, error) {
	query := `
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
		  AND name NOT LIKE 'sqlite_%'
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

// getColumns retrieves all columns for a specific table.
func (s *sqliteInspector) getColumns(ctx context.Context, db *sql.DB, tableName string) ([]Column, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var cid int
		var name string
		var typeName string
		var notNull int
		var defaultValue sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}

		column := Column{
			Name:       name,
			Type:       DataType(typeName),
			IsNullable: notNull == 0,
		}

		if defaultValue.Valid {
			column.DefaultValue = defaultValue.String
		}

		columns = append(columns, column)
	}

	return columns, rows.Err()
}

// getConstraints retrieves all constraints for a specific table.
func (s *sqliteInspector) getConstraints(ctx context.Context, db *sql.DB, tableName string) ([]Constraint, error) {
	var constraints []Constraint

	// Get primary keys
	pk, err := s.getPrimaryKey(ctx, db, tableName)
	if err != nil {
		return nil, err
	}
	if pk != nil {
		constraints = append(constraints, *pk)
	}

	// Get foreign keys
	fks, err := s.getForeignKeys(ctx, db, tableName)
	if err != nil {
		return nil, err
	}
	constraints = append(constraints, fks...)

	// Get unique constraints from indexes
	uniques, err := s.getUniqueConstraints(ctx, db, tableName)
	if err != nil {
		return nil, err
	}
	constraints = append(constraints, uniques...)

	return constraints, nil
}

// getPrimaryKey retrieves the primary key constraint for a table.
func (s *sqliteInspector) getPrimaryKey(ctx context.Context, db *sql.DB, tableName string) (*Constraint, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pkColumns []string
	for rows.Next() {
		var cid int
		var name string
		var typeName string
		var notNull int
		var defaultValue sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}

		if pk > 0 {
			pkColumns = append(pkColumns, name)
		}
	}

	if len(pkColumns) == 0 {
		return nil, nil
	}

	return &Constraint{
		Kind:    PrimaryKey,
		Columns: pkColumns,
	}, rows.Err()
}

// getForeignKeys retrieves all foreign key constraints for a table.
func (s *sqliteInspector) getForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]Constraint, error) {
	query := fmt.Sprintf("PRAGMA foreign_key_list(%s)", tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Group foreign keys by id
	fkMap := make(map[int]*Constraint)

	for rows.Next() {
		var id int
		var seq int
		var table string
		var from string
		var to string
		var onUpdate string
		var onDelete string
		var match string

		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}

		if _, exists := fkMap[id]; !exists {
			fkMap[id] = &Constraint{
				Kind:           ForeignKey,
				ReferenceTable: table,
			}
		}

		fkMap[id].Columns = append(fkMap[id].Columns, from)
		fkMap[id].ReferenceColumns = append(fkMap[id].ReferenceColumns, to)
	}

	var constraints []Constraint
	for _, fk := range fkMap {
		constraints = append(constraints, *fk)
	}

	return constraints, rows.Err()
}

// getUniqueConstraints retrieves unique constraints from indexes.
func (s *sqliteInspector) getUniqueConstraints(ctx context.Context, db *sql.DB, tableName string) ([]Constraint, error) {
	query := fmt.Sprintf("PRAGMA index_list(%s)", tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraints []Constraint

	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int

		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, err
		}

		// Only process unique indexes that are not primary keys
		if unique == 1 && !strings.HasPrefix(origin, "pk") {
			columns, err := s.getIndexColumns(ctx, db, name)
			if err != nil {
				return nil, err
			}

			constraints = append(constraints, Constraint{
				Kind:    Unique,
				Columns: columns,
			})
		}
	}

	return constraints, rows.Err()
}

// getIndexColumns retrieves the columns for a specific index.
func (s *sqliteInspector) getIndexColumns(ctx context.Context, db *sql.DB, indexName string) ([]string, error) {
	query := fmt.Sprintf("PRAGMA index_info(%s)", indexName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var seqno int
		var cid int
		var name string

		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil, err
		}

		columns = append(columns, name)
	}

	return columns, rows.Err()
}
