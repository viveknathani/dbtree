package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type postgresInspector struct{}

func (p *postgresInspector) InspectSchema(ctx context.Context, db *sql.DB) (*Database, error) {
	dbName, err := p.getDatabaseName(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get database name: %w", err)
	}

	tables, err := p.getTables(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	allColumns, err := p.getAllColumns(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get all columns: %w", err)
	}

	allConstraints, err := p.getAllConstraints(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("failed to get all constraints: %w", err)
	}

	for i := range tables {
		tableName := tables[i].Name
		tables[i].Column = allColumns[tableName]
		tables[i].Constraints = allConstraints[tableName]
	}

	return &Database{
		Name:   dbName,
		Tables: tables,
	}, nil
}

func (p *postgresInspector) getDatabaseName(ctx context.Context, db *sql.DB) (string, error) {
	var dbName string
	err := db.QueryRowContext(ctx, "select current_database()").Scan(&dbName)
	return dbName, err
}

func (p *postgresInspector) getTables(ctx context.Context, db *sql.DB) ([]Table, error) {
	query := `
		select table_name 
		from information_schema.tables 
		where table_schema = 'public' 
		and table_type = 'BASE TABLE'
		order by table_name
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

func (p *postgresInspector) getAllColumns(ctx context.Context, db *sql.DB) (map[string][]Column, error) {
	query := `
		select 
			table_name,
			column_name,
			data_type,
			character_maximum_length,
			numeric_precision,
			numeric_scale,
			is_nullable,
			column_default
		from information_schema.columns
		where table_schema = 'public' 
		order by table_name, ordinal_position
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	allColumns := make(map[string][]Column)
	for rows.Next() {
		var (
			tableName        string
			columnName       string
			dataType         string
			charMaxLength    sql.NullInt64
			numericPrecision sql.NullInt64
			numericScale     sql.NullInt64
			isNullable       string
			columnDefault    sql.NullString
		)

		if err := rows.Scan(&tableName, &columnName, &dataType, &charMaxLength, &numericPrecision,
			&numericScale, &isNullable, &columnDefault); err != nil {
			return nil, err
		}

		pgType := formatPostgresType(dataType, charMaxLength, numericPrecision, numericScale)

		column := Column{
			Name:         columnName,
			Type:         DataType(pgType),
			IsNullable:   isNullable == "YES",
			DefaultValue: columnDefault.String,
		}

		allColumns[tableName] = append(allColumns[tableName], column)
	}

	return allColumns, rows.Err()
}

func (p *postgresInspector) getColumns(ctx context.Context, db *sql.DB, tableName string) ([]Column, error) {
	query := `
		select 
			column_name,
			data_type,
			character_maximum_length,
			numeric_precision,
			numeric_scale,
			is_nullable,
			column_default
		from information_schema.columns
		where table_schema = 'public' and table_name = $1
		order by ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var (
			columnName       string
			dataType         string
			charMaxLength    sql.NullInt64
			numericPrecision sql.NullInt64
			numericScale     sql.NullInt64
			isNullable       string
			columnDefault    sql.NullString
		)

		if err := rows.Scan(&columnName, &dataType, &charMaxLength, &numericPrecision,
			&numericScale, &isNullable, &columnDefault); err != nil {
			return nil, err
		}

		pgType := formatPostgresType(dataType, charMaxLength, numericPrecision, numericScale)

		columns = append(columns, Column{
			Name:         columnName,
			Type:         DataType(pgType),
			IsNullable:   isNullable == "YES",
			DefaultValue: columnDefault.String,
		})
	}

	return columns, rows.Err()
}

func (p *postgresInspector) getAllConstraints(ctx context.Context, db *sql.DB) (map[string][]Constraint, error) {
	allConstraints := make(map[string][]Constraint)

	// Get all primary keys
	pkConstraints, err := p.getAllPrimaryKeys(ctx, db)
	if err != nil {
		return nil, err
	}
	for tableName, constraints := range pkConstraints {
		allConstraints[tableName] = append(allConstraints[tableName], constraints...)
	}

	// Get all foreign keys
	fkConstraints, err := p.getAllForeignKeys(ctx, db)
	if err != nil {
		return nil, err
	}
	for tableName, constraints := range fkConstraints {
		allConstraints[tableName] = append(allConstraints[tableName], constraints...)
	}

	// Get all unique constraints
	uniqueConstraints, err := p.getAllUniqueConstraints(ctx, db)
	if err != nil {
		return nil, err
	}
	for tableName, constraints := range uniqueConstraints {
		allConstraints[tableName] = append(allConstraints[tableName], constraints...)
	}

	// Get all check constraints
	checkConstraints, err := p.getAllCheckConstraints(ctx, db)
	if err != nil {
		return nil, err
	}
	for tableName, constraints := range checkConstraints {
		allConstraints[tableName] = append(allConstraints[tableName], constraints...)
	}

	return allConstraints, nil
}

func (p *postgresInspector) getAllPrimaryKeys(ctx context.Context, db *sql.DB) (map[string][]Constraint, error) {
	query := `
		select tc.table_name, kcu.column_name
		from information_schema.table_constraints tc
		join information_schema.key_column_usage kcu 
			on tc.constraint_name = kcu.constraint_name 
			and tc.table_schema = kcu.table_schema
		where tc.constraint_type = 'PRIMARY KEY' 
			and tc.table_schema = 'public'
		order by tc.table_name, kcu.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	constraintMap := make(map[string][]string)
	for rows.Next() {
		var tableName, columnName string
		if err := rows.Scan(&tableName, &columnName); err != nil {
			return nil, err
		}
		constraintMap[tableName] = append(constraintMap[tableName], columnName)
	}

	result := make(map[string][]Constraint)
	for tableName, columns := range constraintMap {
		if len(columns) > 0 {
			result[tableName] = []Constraint{{
				Kind:    PrimaryKey,
				Columns: columns,
			}}
		}
	}

	return result, rows.Err()
}

func (p *postgresInspector) getAllForeignKeys(ctx context.Context, db *sql.DB) (map[string][]Constraint, error) {
	query := `
		select 
			src_rel.relname as table_name,
			con.conname as constraint_name,
			ref_rel.relname as foreign_table_name,
			con.conkey,
			con.confkey
		from pg_constraint con
		join pg_class src_rel on src_rel.oid = con.conrelid
		join pg_class ref_rel on ref_rel.oid = con.confrelid  
		join pg_namespace nsp on nsp.oid = src_rel.relnamespace
		where con.contype = 'f'
			and nsp.nspname = 'public'
		order by src_rel.relname, con.conname
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type fkConstraint struct {
		tableName        string
		constraintName   string
		foreignTableName string
		conkey          []int16
		confkey         []int16
	}

	var fkConstraints []fkConstraint
	for rows.Next() {
		var (
			tableName        string
			constraintName   string
			foreignTableName string
			conkeyBytes      []byte
			confkeyBytes     []byte
		)

		if err := rows.Scan(&tableName, &constraintName, &foreignTableName, &conkeyBytes, &confkeyBytes); err != nil {
			return nil, err
		}

		// Parse PostgreSQL arrays of attribute numbers
		conkey := parseInt16Array(string(conkeyBytes))
		confkey := parseInt16Array(string(confkeyBytes))

		fkConstraints = append(fkConstraints, fkConstraint{
			tableName:        tableName,
			constraintName:   constraintName,
			foreignTableName: foreignTableName,
			conkey:          conkey,
			confkey:         confkey,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Convert attribute numbers to column names
	result := make(map[string][]Constraint)
	for _, fk := range fkConstraints {
		// Get source table column names
		sourceColumns, err := p.getColumnNamesByAttnum(ctx, db, fk.tableName, fk.conkey)
		if err != nil {
			return nil, err
		}

		// Get referenced table column names  
		refColumns, err := p.getColumnNamesByAttnum(ctx, db, fk.foreignTableName, fk.confkey)
		if err != nil {
			return nil, err
		}

		constraint := Constraint{
			Kind:             ForeignKey,
			Columns:          sourceColumns,
			ReferenceTable:   fk.foreignTableName,
			ReferenceColumns: refColumns,
		}

		result[fk.tableName] = append(result[fk.tableName], constraint)
	}

	return result, nil
}

func (p *postgresInspector) getColumnNamesByAttnum(ctx context.Context, db *sql.DB, tableName string, attnums []int16) ([]string, error) {
	if len(attnums) == 0 {
		return nil, nil
	}

	// Get all column names and attnums for the table, then filter and order
	query := `
		select attname, attnum
		from pg_attribute 
		join pg_class on pg_class.oid = pg_attribute.attrelid
		join pg_namespace on pg_namespace.oid = pg_class.relnamespace
		where pg_namespace.nspname = 'public'
			and pg_class.relname = $1
			and attnum > 0
			and not attisdropped
		order by attnum
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Create map of attnum -> column name
	attnumToName := make(map[int16]string)
	for rows.Next() {
		var colName string
		var attnum int16
		if err := rows.Scan(&colName, &attnum); err != nil {
			return nil, err
		}
		attnumToName[attnum] = colName
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build result in the same order as attnums slice
	var columns []string
	for _, attnum := range attnums {
		if colName, exists := attnumToName[attnum]; exists {
			columns = append(columns, colName)
		}
	}

	return columns, nil
}

func parseInt16Array(pgArray string) []int16 {
	// Parse PostgreSQL array format like "{1,2,3}"
	if len(pgArray) < 2 || pgArray[0] != '{' || pgArray[len(pgArray)-1] != '}' {
		return nil
	}

	content := pgArray[1 : len(pgArray)-1]
	if content == "" {
		return nil
	}

	parts := strings.Split(content, ",")
	result := make([]int16, 0, len(parts))
	
	for _, part := range parts {
		if val := parseInt16(strings.TrimSpace(part)); val != 0 {
			result = append(result, val)
		}
	}

	return result
}

func parseInt16(s string) int16 {
	var result int16
	for _, r := range s {
		if r >= '0' && r <= '9' {
			result = result*10 + int16(r-'0')
		} else {
			return 0
		}
	}
	return result
}

func (p *postgresInspector) getAllUniqueConstraints(ctx context.Context, db *sql.DB) (map[string][]Constraint, error) {
	query := `
		select tc.table_name, tc.constraint_name, kcu.column_name
		from information_schema.table_constraints tc
		join information_schema.key_column_usage kcu 
			on tc.constraint_name = kcu.constraint_name 
			and tc.table_schema = kcu.table_schema
		where tc.constraint_type = 'UNIQUE' 
			and tc.table_schema = 'public'
		order by tc.table_name, tc.constraint_name, kcu.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	constraintMap := make(map[string]map[string][]string)
	for rows.Next() {
		var tableName, constraintName, columnName string
		if err := rows.Scan(&tableName, &constraintName, &columnName); err != nil {
			return nil, err
		}

		if constraintMap[tableName] == nil {
			constraintMap[tableName] = make(map[string][]string)
		}
		constraintMap[tableName][constraintName] = append(constraintMap[tableName][constraintName], columnName)
	}

	result := make(map[string][]Constraint)
	for tableName, tableConstraints := range constraintMap {
		for _, columns := range tableConstraints {
			result[tableName] = append(result[tableName], Constraint{
				Kind:    Unique,
				Columns: columns,
			})
		}
	}

	return result, rows.Err()
}

func (p *postgresInspector) getAllCheckConstraints(ctx context.Context, db *sql.DB) (map[string][]Constraint, error) {
	query := `
		select 
			rel.relname as table_name,
			con.conname,
			pg_get_constraintdef(con.oid) as definition
		from pg_constraint con
		join pg_class rel on rel.oid = con.conrelid
		join pg_namespace nsp on nsp.oid = rel.relnamespace
		where nsp.nspname = 'public'
			and con.contype = 'c'
		order by rel.relname
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]Constraint)
	for rows.Next() {
		var tableName, constraintName, definition string
		if err := rows.Scan(&tableName, &constraintName, &definition); err != nil {
			return nil, err
		}

		checkExpr := extractCheckExpression(definition)
		result[tableName] = append(result[tableName], Constraint{
			Kind:            Check,
			CheckExpression: checkExpr,
		})
	}

	return result, rows.Err()
}

func (p *postgresInspector) getPrimaryKeys(ctx context.Context, db *sql.DB, tableName string) ([]Constraint, error) {
	query := `
		select kcu.column_name
		from information_schema.table_constraints tc
		join information_schema.key_column_usage kcu 
			on tc.constraint_name = kcu.constraint_name 
			and tc.table_schema = kcu.table_schema
		where tc.constraint_type = 'PRIMARY KEY' 
			and tc.table_schema = 'public'
			and tc.table_name = $1
		order by kcu.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}

	if len(columns) == 0 {
		return nil, nil
	}

	return []Constraint{{
		Kind:    PrimaryKey,
		Columns: columns,
	}}, rows.Err()
}

func (p *postgresInspector) getForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]Constraint, error) {
	query := `
		select 
			tc.constraint_name,
			kcu.column_name,
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name
		from information_schema.table_constraints AS tc 
		join information_schema.key_column_usage AS kcu
			on tc.constraint_name = kcu.constraint_name
			and tc.table_schema = kcu.table_schema
		join information_schema.constraint_column_usage AS ccu
			on ccu.constraint_name = tc.constraint_name
			and ccu.table_schema = tc.table_schema
		where tc.constraint_type = 'FOREIGN KEY' 
			and tc.table_schema = 'public'
			and tc.table_name = $1
		order by tc.constraint_name, kcu.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	constraintMap := make(map[string]*Constraint)
	for rows.Next() {
		var (
			constraintName    string
			columnName        string
			foreignTableName  string
			foreignColumnName string
		)

		if err := rows.Scan(&constraintName, &columnName, &foreignTableName, &foreignColumnName); err != nil {
			return nil, err
		}

		if _, exists := constraintMap[constraintName]; !exists {
			constraintMap[constraintName] = &Constraint{
				Kind:             ForeignKey,
				Columns:          []string{},
				ReferenceTable:   foreignTableName,
				ReferenceColumns: []string{},
			}
		}

		constraintMap[constraintName].Columns = append(constraintMap[constraintName].Columns, columnName)
		if !contains(constraintMap[constraintName].ReferenceColumns, foreignColumnName) {
			constraintMap[constraintName].ReferenceColumns = append(constraintMap[constraintName].ReferenceColumns, foreignColumnName)
		}
	}

	var constraints []Constraint
	for _, c := range constraintMap {
		constraints = append(constraints, *c)
	}

	return constraints, rows.Err()
}

func (p *postgresInspector) getUniqueConstraints(ctx context.Context, db *sql.DB, tableName string) ([]Constraint, error) {
	query := `
		select tc.constraint_name, kcu.column_name
		from information_schema.table_constraints tc
		join information_schema.key_column_usage kcu 
			on tc.constraint_name = kcu.constraint_name 
			and tc.table_schema = kcu.table_schema
		where tc.constraint_type = 'UNIQUE' 
			and tc.table_schema = 'public'
			and tc.table_name = $1
		order by tc.constraint_name, kcu.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	constraintMap := make(map[string][]string)
	for rows.Next() {
		var constraintName, columnName string
		if err := rows.Scan(&constraintName, &columnName); err != nil {
			return nil, err
		}
		constraintMap[constraintName] = append(constraintMap[constraintName], columnName)
	}

	var constraints []Constraint
	for _, columns := range constraintMap {
		constraints = append(constraints, Constraint{
			Kind:    Unique,
			Columns: columns,
		})
	}

	return constraints, rows.Err()
}

func (p *postgresInspector) getCheckConstraints(ctx context.Context, db *sql.DB, tableName string) ([]Constraint, error) {
	query := `
		select 
			con.conname,
			pg_get_constraintdef(con.oid) as definition
		from pg_constraint con
		join pg_class rel on rel.oid = con.conrelid
		join pg_namespace nsp on nsp.oid = rel.relnamespace
		where nsp.nspname = 'public'
			and rel.relname = $1
			and con.contype = 'c'
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraints []Constraint
	for rows.Next() {
		var constraintName, definition string
		if err := rows.Scan(&constraintName, &definition); err != nil {
			return nil, err
		}

		checkExpr := extractCheckExpression(definition)
		constraints = append(constraints, Constraint{
			Kind:            Check,
			CheckExpression: checkExpr,
		})
	}

	return constraints, rows.Err()
}

func formatPostgresType(dataType string, charMaxLength, numericPrecision, numericScale sql.NullInt64) string {
	switch dataType {
	case "character varying":
		if charMaxLength.Valid {
			return fmt.Sprintf("varchar(%d)", charMaxLength.Int64)
		}
		return "varchar"
	case "character":
		if charMaxLength.Valid {
			return fmt.Sprintf("char(%d)", charMaxLength.Int64)
		}
		return "char"
	case "numeric":
		if numericPrecision.Valid && numericScale.Valid {
			return fmt.Sprintf("numeric(%d,%d)", numericPrecision.Int64, numericScale.Int64)
		} else if numericPrecision.Valid {
			return fmt.Sprintf("numeric(%d)", numericPrecision.Int64)
		}
		return "numeric"
	case "timestamp without time zone":
		return "timestamp"
	case "timestamp with time zone":
		return "timestamptz"
	case "time without time zone":
		return "time"
	case "time with time zone":
		return "timetz"
	default:
		return dataType
	}
}

func extractCheckExpression(definition string) string {
	if strings.HasPrefix(definition, "CHECK (") && strings.HasSuffix(definition, ")") {
		return definition[7 : len(definition)-1]
	}
	return definition
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
