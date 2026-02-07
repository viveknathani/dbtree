package database

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
)

const postgresConnStr = "postgres://postgres:postgres@127.0.0.1:35432/testdb?sslmode=disable"

// TestPostgreSQLInspectSchema tests the complete schema inspection functionality for PostgreSQL.
func TestPostgreSQLInspectSchema(t *testing.T) {
	// Connect to dockerized postgres
	db, err := sql.Open("postgres", postgresConnStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		t.Skipf("Skipping test: PostgreSQL not available: %v", err)
	}

	ctx := context.Background()

	// Create test schema
	if err := createPostgreSQLTestSchema(ctx, db); err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}
	defer cleanupPostgreSQLTestSchema(ctx, db)

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

// TestPostgreSQLDatabaseDetection tests the database type detection functionality for PostgreSQL.
func TestPostgreSQLDatabaseDetection(t *testing.T) {
	// Connect to dockerized postgres
	db, err := sql.Open("postgres", postgresConnStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		t.Skipf("Skipping test: PostgreSQL not available: %v", err)
	}

	ctx := context.Background()

	// Test database type detection
	dbType, err := detectDatabaseType(ctx, db)
	if err != nil {
		t.Fatalf("Failed to detect database type: %v", err)
	}

	if dbType != "postgres" {
		t.Errorf("Expected database type 'postgres', got '%s'", dbType)
	}
}

// createPostgreSQLTestSchema creates test tables with various column types and constraints for testing.
func createPostgreSQLTestSchema(ctx context.Context, db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS test_users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			name VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS test_posts (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES test_users(id),
			title VARCHAR(200) NOT NULL,
			content TEXT,
			published BOOLEAN DEFAULT FALSE,
			CONSTRAINT title_length CHECK (LENGTH(title) > 0)
		)`,
	}

	for _, query := range queries {
		if _, err := db.ExecContext(ctx, query); err != nil {
			return err
		}
	}

	return nil
}

// cleanupPostgreSQLTestSchema removes test tables after testing is complete.
func cleanupPostgreSQLTestSchema(ctx context.Context, db *sql.DB) {
	queries := []string{
		`DROP TABLE IF EXISTS test_posts CASCADE`,
		`DROP TABLE IF EXISTS test_users CASCADE`,
	}

	for _, query := range queries {
		db.ExecContext(ctx, query)
	}
}
