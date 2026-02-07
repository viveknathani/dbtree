package database

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

const clickhouseConnStr = "clickhouse://default:@localhost:9000/testdb"

// TestClickHouseInspectSchema tests the complete schema inspection functionality for ClickHouse.
func TestClickHouseInspectSchema(t *testing.T) {
	// Connect to dockerized ClickHouse
	db, err := sql.Open("clickhouse", clickhouseConnStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		t.Skipf("Skipping test: ClickHouse not available: %v", err)
	}

	ctx := context.Background()

	// Create test schema
	if err := createClickHouseTestSchema(ctx, db); err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}
	defer cleanupClickHouseTestSchema(ctx, db)

	// Test InspectSchema
	result, err := InspectSchema(ctx, db)
	if err != nil {
		t.Fatalf("InspectSchema failed: %v", err)
	}

	// Verify database name
	if result.Name == "" {
		t.Error("Expected database name to be set")
	}

	// Verify tables
	if len(result.Tables) < 2 {
		t.Errorf("Expected at least 2 tables, got %d", len(result.Tables))
	}

	// Find users table
	var usersTable *Table
	for i := range result.Tables {
		if result.Tables[i].Name == "test_users" {
			usersTable = &result.Tables[i]
			break
		}
	}

	if usersTable == nil {
		t.Fatal("test_users table not found")
	}

	// Verify columns
	expectedColumns := []string{"id", "email", "name", "created_at"}
	if len(usersTable.Columns) != len(expectedColumns) {
		t.Errorf("Expected %d columns, got %d", len(expectedColumns), len(usersTable.Columns))
	}

	// Verify primary key constraint
	hasPrimaryKey := false
	for _, constraint := range usersTable.Constraints {
		if constraint.Kind == PrimaryKey {
			hasPrimaryKey = true
			// ClickHouse primary key should be on id column
			if len(constraint.Columns) != 1 || constraint.Columns[0] != "id" {
				t.Errorf("Expected primary key on 'id' column, got %v", constraint.Columns)
			}
		}
	}

	if !hasPrimaryKey {
		t.Error("Expected primary key constraint on test_users table")
	}

	// Find posts table
	var postsTable *Table
	for i := range result.Tables {
		if result.Tables[i].Name == "test_posts" {
			postsTable = &result.Tables[i]
			break
		}
	}

	if postsTable == nil {
		t.Fatal("test_posts table not found")
	}

	// Note: ClickHouse does not enforce foreign keys, so we don't test for them
}

// TestClickHouseDatabaseDetection tests the database type detection functionality for ClickHouse.
func TestClickHouseDatabaseDetection(t *testing.T) {
	// Connect to dockerized ClickHouse
	db, err := sql.Open("clickhouse", clickhouseConnStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		t.Skipf("Skipping test: ClickHouse not available: %v", err)
	}

	ctx := context.Background()

	// Test database type detection
	dbType, err := detectDatabaseType(ctx, db)
	if err != nil {
		t.Fatalf("Failed to detect database type: %v", err)
	}

	if dbType != "clickhouse" {
		t.Errorf("Expected database type 'clickhouse', got '%s'", dbType)
	}
}

// createClickHouseTestSchema creates test tables with various column types and constraints for testing.
func createClickHouseTestSchema(ctx context.Context, db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS test_users (
			id UInt64,
			email String,
			name String,
			created_at DateTime DEFAULT now()
		) ENGINE = MergeTree()
		PRIMARY KEY id
		ORDER BY id`,
		`CREATE TABLE IF NOT EXISTS test_posts (
			id UInt64,
			user_id UInt64,
			title String,
			content String,
			published UInt8 DEFAULT 0
		) ENGINE = MergeTree()
		PRIMARY KEY id
		ORDER BY id`,
	}

	for _, query := range queries {
		if _, err := db.ExecContext(ctx, query); err != nil {
			return err
		}
	}

	return nil
}

// cleanupClickHouseTestSchema removes test tables after testing is complete.
func cleanupClickHouseTestSchema(ctx context.Context, db *sql.DB) {
	queries := []string{
		`DROP TABLE IF EXISTS test_posts`,
		`DROP TABLE IF EXISTS test_users`,
	}

	for _, query := range queries {
		db.ExecContext(ctx, query)
	}
}
