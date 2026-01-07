package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/viveknathani/dbtree/database"
	_ "github.com/lib/pq"
)

func main() {
	var connStr string
	flag.StringVar(&connStr, "conn", "", "PostgreSQL connection string (e.g., postgresql://user:pass@localhost/dbname)")
	flag.Parse()

	if connStr == "" {
		connStr = os.Getenv("DATABASE_URL")
		if connStr == "" {
			fmt.Println("usage: dbtree -conn='postgresql://user:pass@localhost/dbname'")
			fmt.Println("or set DATABASE_URL environment variable")
			os.Exit(1)
		}
	}

	// Connect to database
	fmt.Print("connecting to database...")
	connectStart := time.Now()
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	connectDuration := time.Since(connectStart)
	fmt.Printf(" ✓ (took %v)\n", connectDuration)

	// Inspect schema
	fmt.Print("inspecting database schema...")
	inspectStart := time.Now()
	ctx := context.Background()
	schema, err := database.InspectSchema(ctx, db)
	if err != nil {
		log.Fatalf("failed to inspect schema: %v", err)
	}
	inspectDuration := time.Since(inspectStart)
	fmt.Printf(" ✓ (took %v)\n", inspectDuration)

	// Print results
	fmt.Print("generating output...")
	outputStart := time.Now()
	
	fmt.Printf("\n\ndatabase: %s\n", schema.Name)
	fmt.Printf("found %d tables\n\n", len(schema.Tables))

	for _, table := range schema.Tables {
		fmt.Printf("table: %s\n", table.Name)
		fmt.Printf("  columns (%d):\n", len(table.Column))
		for _, col := range table.Column {
			nullable := "NOT NULL"
			if col.IsNullable {
				nullable = "NULL"
			}
			defaultVal := ""
			if col.DefaultValue != "" {
				defaultVal = fmt.Sprintf(" DEFAULT %s", col.DefaultValue)
			}
			fmt.Printf("    - %s %s %s%s\n", col.Name, col.Type, nullable, defaultVal)
		}

		if len(table.Constraints) > 0 {
			fmt.Printf("  constraints (%d):\n", len(table.Constraints))
			for _, constraint := range table.Constraints {
				switch constraint.Kind {
				case database.PrimaryKey:
					fmt.Printf("    - PRIMARY KEY (%s)\n", joinStrings(constraint.Columns))
				case database.ForeignKey:
					fmt.Printf("    - FOREIGN KEY (%s) REFERENCES %s(%s)\n", 
						joinStrings(constraint.Columns), 
						constraint.ReferenceTable, 
						joinStrings(constraint.ReferenceColumns))
				case database.Unique:
					fmt.Printf("    - UNIQUE (%s)\n", joinStrings(constraint.Columns))
				case database.Check:
					fmt.Printf("    - CHECK (%s)\n", constraint.CheckExpression)
				}
			}
		}
		fmt.Println()
	}

	outputDuration := time.Since(outputStart)
	fmt.Printf("output generated ✓ (took %v)\n", outputDuration)

	// Also output as JSON for programmatic use
	if jsonOutput := os.Getenv("JSON_OUTPUT"); jsonOutput == "true" {
		fmt.Print("generating json output...")
		jsonStart := time.Now()
		jsonData, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal schema to json: %v", err)
		}
		jsonDuration := time.Since(jsonStart)
		fmt.Printf(" ✓ (took %v)\n", jsonDuration)
		fmt.Println("\njson output:")
		fmt.Println(string(jsonData))
	}

	totalDuration := time.Since(connectStart)
	fmt.Printf("\ntotal execution time: %v\n", totalDuration)
}

func joinStrings(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += ", " + strs[i]
	}
	return result
}
