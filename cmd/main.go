package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/upbound/official-providers/testing/pkg"
)

func main() {
	cd, err := os.Getwd()
	if err != nil {
		kingpin.FatalIfError(err, "cannot get current directory")
	}
	rootDirectory := filepath.Dir(cd)

	var (
		app     = kingpin.New("auto-test", "Automated Test Tool for Upbound Official Providers").DefaultEnvars()
		comment = app.Flag("comment", "Content of Issue Comment. Value of this option will be used to trigger/configure the tests."+
			"/test-examples keyword is searched and if found the automated tests is triggered."+
			"The possible usages for /test-examples:\n"+
			"'provider-aws/examples-generated/s3/bucket.yaml,provider-gcp/examples-generated/storage/bucket.yaml': "+
			"The comma separated resources are used as test inputs.\n"+
			"If this option is not set, 'COMMENT' env var is used as default.").Envar("COMMENT").String()
		providerName = app.Flag("provider", "The provider name to run the tests.\n"+
			"If this option is not set, 'PROVIDER_NAME' env var is used as default.").Envar("PROVIDER_NAME").String()
		dataSourcePath = app.Flag("data-source", "File path of data source that will be used for injection some values.").String()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	o := &pkg.AutomatedTestOptions{
		Description:    *comment,
		ProviderName:   *providerName,
		RootDirectory:  rootDirectory,
		DataSourcePath: *dataSourcePath,
	}
	providerCredsEnv := fmt.Sprintf("%s_CREDS", strings.ToUpper(strings.ReplaceAll(o.ProviderName, "-", "_")))
	o.ProviderCredentials = os.Getenv(providerCredsEnv)

	if o.DataSourcePath == "" {
		o.DataSourcePath = filepath.Join(o.RootDirectory, "testing/testdata/datasource.yaml")
	}

	kingpin.FatalIfError(pkg.RunTest(o), "cannot run automated tests successfully")
}
