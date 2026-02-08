package database

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestSQLiteInspectSchema tests the complete schema inspection functionality for SQLite.
func TestSQLiteInspectSchema(t *testing.T) {
	// Use in-memory database for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Create test schema
	if err := createSQLiteTestSchema(ctx, db); err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

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

	// Verify constraints
	hasPrimaryKey := false
	hasUniqueConstraint := false
	for _, constraint := range usersTable.Constraints {
		if constraint.Kind == PrimaryKey {
			hasPrimaryKey = true
		}
		if constraint.Kind == Unique {
			hasUniqueConstraint = true
		}
	}

	if !hasPrimaryKey {
		t.Error("Expected primary key constraint on test_users table")
	}

	if !hasUniqueConstraint {
		t.Error("Expected unique constraint on test_users table")
	}

	// Find posts table and verify foreign key
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

	hasForeignKey := false
	for _, constraint := range postsTable.Constraints {
		if constraint.Kind == ForeignKey {
			hasForeignKey = true
			if constraint.ReferenceTable != "test_users" {
				t.Errorf("Expected foreign key to reference test_users, got %s", constraint.ReferenceTable)
			}
		}
	}

	if !hasForeignKey {
		t.Error("Expected foreign key constraint on test_posts table")
	}
}

// TestSQLiteDatabaseDetection tests the database type detection functionality for SQLite.
func TestSQLiteDatabaseDetection(t *testing.T) {
	// Use in-memory database for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Test database type detection
	dbType, err := detectDatabaseType(ctx, db)
	if err != nil {
		t.Fatalf("Failed to detect database type: %v", err)
	}

	if dbType != "sqlite" {
		t.Errorf("Expected database type 'sqlite', got '%s'", dbType)
	}
}

// createSQLiteTestSchema creates test tables with various column types and constraints for testing.
func createSQLiteTestSchema(ctx context.Context, db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS test_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE NOT NULL,
			name TEXT,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS test_posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			title TEXT NOT NULL,
			content TEXT,
			published INTEGER DEFAULT 0,
			FOREIGN KEY (user_id) REFERENCES test_users(id)
		)`,
	}

	for _, query := range queries {
		if _, err := db.ExecContext(ctx, query); err != nil {
			return err
		}
	}

	return nil
}
