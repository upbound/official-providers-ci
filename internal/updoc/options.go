package updoc

// UploadOptions represents the options for the upload subcommand
type UploadOptions struct {
	DocsDir    string `name:"docs-dir" required:"" help:"Root directory to crawl for documents."`
	Name       string `name:"name" required:"" help:"The name of the provider being processed."`
	Version    string `name:"version" required:"" help:"The major and minor version number of the provider branch."`
	BucketName string `name:"bucket-name" required:"" help:"Bucket to put documentation in."`
	CDNDomain  string `name:"cdn-domain" required:"" help:"CDN name to prefix processed urls with."`
}

// Options represents the available subcommands of updoc:
// "generate" and "upload".
type Options struct {
	Generate struct {
		DocsDir string `name:"docs-dir" required:"" help:"Root directory to crawl for documents."`
	} `kong:"cmd"`
	Upload UploadOptions `kong:"cmd"`
}
