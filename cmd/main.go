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

	var (
		app         = kingpin.New("uptest", "Automated Test Tool for Upbound Official Providers").DefaultEnvars()
		exampleList = app.Flag("example-list", "List of example manifests. Value of this option will be used to trigger/configure the tests."+
			"The possible usage:\n"+
			"'provider-aws/examples/s3/bucket.yaml,provider-gcp/examples/storage/bucket.yaml': "+
			"The comma separated resources are used as test inputs.\n"+
			"If this option is not set, 'EXAMPLE_LIST' env var is used as default.").Envar("EXAMPLE_LIST").String()
		providerName = app.Flag("provider", "The provider name to run the tests.\n"+
			"If this option is not set, 'PROVIDER_NAME' env var is used as default.").Envar("PROVIDER_NAME").String()
		dataSourcePath     = app.Flag("data-source", "File path of data source that will be used for injection some values.").Default("").String()
		skipProviderConfig = app.Flag("skip-provider-config", "Skip ProviderConfig creation.").Bool()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	var examplePaths []string
	for _, e := range strings.Split(*exampleList, ",") {
		if e == "" {
			continue
		}
		examplePaths = append(examplePaths, filepath.Join(cd, filepath.Clean(e)))
	}
	if len(examplePaths) == 0 {
		fmt.Println("No example files to test.")
		return
	}

	o := &pkg.AutomatedTestOptions{
		ExamplePaths:       examplePaths,
		ProviderName:       *providerName,
		RootDirectory:      cd,
		DataSourcePath:     *dataSourcePath,
		SkipProviderConfig: *skipProviderConfig,
	}
	providerCredsEnv := fmt.Sprintf("%s_CREDS", strings.ToUpper(strings.ReplaceAll(o.ProviderName, "-", "_")))
	o.ProviderCredentials = os.Getenv(providerCredsEnv)

	if o.DataSourcePath == "" {
		o.DataSourcePath = filepath.Join(o.RootDirectory, "testing/testdata/datasource.yaml")
	}

	kingpin.FatalIfError(pkg.RunTest(o), "cannot run automated tests successfully")
}
