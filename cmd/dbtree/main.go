package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"github.com/viveknathani/dbtree/database"
	"github.com/viveknathani/dbtree/graph"
	"github.com/viveknathani/dbtree/render"
)

type Configuration struct {
	DatabaseUrl string
	Format      string
	Shape       string
}

// parseFlags parses command-line flags and returns a Configuration struct.
func parseFlags() Configuration {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "dbtree - A tool to visualize database schemas\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(os.Stderr, "  --%s\n", f.Name)
			fmt.Fprintf(os.Stderr, "        %s", f.Usage)
			if f.DefValue != "" && f.DefValue != "false" {
				fmt.Fprintf(os.Stderr, " (default: %s)", f.DefValue)
			}
			fmt.Fprintf(os.Stderr, "\n")
		})
	}

	dbUrl := flag.String("conn", "", "The database connection URL")
	format := flag.String("format", string(render.FormatText), "The output format (text or json)")
	shape := flag.String("shape", string(render.ShapeTree), "The shape of the output (tree, flat, or chart)")
	help := flag.Bool("help", false, "Display help information")

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	return Configuration{
		DatabaseUrl: *dbUrl,
		Format:      *format,
		Shape:       *shape,
	}
}

func main() {
	config := parseFlags()

	if config.DatabaseUrl == "" {
		flag.Usage()
		os.Exit(1)
	}

	if config.Format != string(render.FormatText) && config.Format != string(render.FormatJSON) {
		log.Fatal("error: invalid format specified (use text or json)")
	}

	if config.Shape != string(render.ShapeTree) && config.Shape != string(render.ShapeFlat) && config.Shape != string(render.ShapeChart) {
		log.Fatal("error: invalid shape specified (use tree, flat, or chart)")
	}

	if config.Shape == string(render.ShapeChart) && config.Format == string(render.FormatJSON) {
		log.Fatal("error: chart shape is only supported with text format")
	}

	if !strings.HasPrefix(config.DatabaseUrl, "postgres://") && !strings.HasPrefix(config.DatabaseUrl, "postgresql://") {
		log.Fatal("error: only PostgreSQL is supported currently")
	}

	db, err := sql.Open("postgres", config.DatabaseUrl)
	if err != nil {
		log.Fatalf("error: failed to open database: %v", err)
	}
	defer db.Close()

	// Verify the connection is actually established
	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("error: failed to connect to database: %v", err)
	}

	schema, err := database.InspectSchema(context.Background(), db)

	if err != nil {
		log.Fatalf("error: failed to inspect database schema: %v", err)
	}

	if schema == nil {
		log.Fatal("error: no schema information found")
	}

	graph, err := graph.Build(schema)
	if err != nil {
		log.Fatalf("error: failed to build schema graph: %v", err)
	}

	if graph == nil {
		log.Fatal("error: schema graph is nil")
	}

	renderedOutput, err := render.Render(graph, render.Format(config.Format), render.Shape(config.Shape))
	if err != nil {
		log.Fatalf("error: failed to render output: %v", err)
	}

	fmt.Println(renderedOutput)
}
