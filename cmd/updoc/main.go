// Package main is the main package for updoc,
// the tool for publishing official provider docs.
package main

import (
	"log"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	internal "github.com/upbound/uptest/internal/updoc"
)

func main() {
	opts := internal.Options{}

	ctx := kong.Parse(&opts, kong.Name("updoc"),
		kong.Description("Upbound enhanced document processor"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:   true,
			FlagsLast: true,
			Summary:   true,
		}))

	switch ctx.Command() {
	case "generate":
		if err := internal.NewIndexer(opts.Generate.DocsDir).Run(); err != nil {
			log.Fatal(err)
		}
	case "upload":
		if err := internal.New().ProcessIndex(opts.Upload, afero.NewOsFs()); err != nil {
			log.Fatal(err)
		}
	}

	// TODO(daren): garbage collection on orphaned docs after updating a version index
}
