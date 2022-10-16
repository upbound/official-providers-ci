package main

import (
	"log"

	"github.com/alecthomas/kong"
	"github.com/spf13/afero"

	"github.com/upbound/provider-tools/internal/docs"
)

func main() {
	opts := docs.UpDocOptions{}

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
		if err := docs.NewIndexer(opts.Generate.DocsDir).Run(); err != nil {
			log.Fatal(err)
		}
	case "upload":
		if err := docs.New().ProcessIndex(opts.Upload, afero.NewOsFs()); err != nil {
			log.Fatal(err)
		}
	}

	// TODO(daren): garbage collection on orphaned docs after updating a version index
}
