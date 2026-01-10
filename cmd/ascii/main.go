package main

import (
	"context"
	"os"

	"github.com/viveknathani/d2/d2graph"
	"github.com/viveknathani/d2/d2layouts/d2elklayout"
	"github.com/viveknathani/d2/d2lib"
	"github.com/viveknathani/d2/d2renderers/d2ascii"
	"github.com/viveknathani/d2/d2renderers/d2ascii/charset"
	"github.com/viveknathani/d2/d2renderers/d2svg"
	"github.com/viveknathani/d2/lib/textmeasure"
	"github.com/viveknathani/dbtree/go2"
)

func main() {
	ruler, err := textmeasure.NewRuler()
	// plugin := &d2plugin.ELKPlugin
	layoutResolver := func(engine string) (d2graph.LayoutGraph, error) {
		return d2elklayout.DefaultLayout, nil
	}

	compileOpts := &d2lib.CompileOptions{
		Ruler:          ruler,
		Layout:         go2.Pointer("elk"),
		LayoutResolver: layoutResolver,
	}

	themeId := int64(0)
	renderOpts := &d2svg.RenderOpts{
		Pad:     go2.Pointer(int64(0)),
		ThemeID: &themeId,
	}

	diagram, _, err := d2lib.Compile(context.Background(), `
	costumes: {
		shape: sql_table
		id: int {constraint: primary_key}
		silliness: int
		monster: int
		last_updated: timestamp
	}

	monsters: {
		shape: sql_table
		id: int {constraint: primary_key}
		movie: string
		weight: int
		last_updated: timestamp
	}

	costumes.monster -> monsters.id

	`, compileOpts, renderOpts)

	os.Setenv("DEBUG_ASCII", "1")

	extendedAsciiArtist := d2ascii.NewASCIIartist()

	extendedBytes, err := extendedAsciiArtist.Render(context.Background(), diagram, &d2ascii.RenderOpts{
		Scale:   go2.Pointer(2.0),
		Charset: charset.Unicode,
	})
	if err != nil {
		panic(err)
	}

	println(string(extendedBytes))
}
