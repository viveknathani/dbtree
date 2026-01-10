package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
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
		fmt.Println("dbtree - A tool to visualize database schemas")
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
	shape := flag.String("shape", string(render.ShapeTree), "The shape of the output (tree or flat)")
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
		fmt.Println("error: invalid format specified (use text or json)")
		os.Exit(1)
	}

	if config.Shape != string(render.ShapeTree) && config.Shape != string(render.ShapeFlat) {
		fmt.Println("error: invalid shape specified (use tree or flat)")
		os.Exit(1)
	}

	if !strings.HasPrefix(config.DatabaseUrl, "postgres") {
		fmt.Println("error: only PostgreSQL is supported currently")
		os.Exit(1)
	}

	db, err := sql.Open("postgres", config.DatabaseUrl)
	if err != nil {
		fmt.Println("error: failed to connect to database")
		os.Exit(1)
	}
	defer db.Close()

	schema, err := database.InspectSchema(context.Background(), db)

	if err != nil {
		fmt.Printf("error: failed to inspect database schema: %v\n", err)
		os.Exit(1)
	}

	if schema == nil {
		fmt.Println("error: no schema information found")
		os.Exit(1)
	}

	graph, err := graph.Build(schema)
	if err != nil {
		fmt.Printf("error: failed to build schema graph: %v\n", err)
		os.Exit(1)
	}

	if graph == nil {
		fmt.Println("error: schema graph is nil")
		os.Exit(1)
	}

	renderedOutput, err := render.Render(graph, render.Format(config.Format), render.Shape(config.Shape))
	if err != nil {
		fmt.Printf("error: failed to render output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(renderedOutput)
}
