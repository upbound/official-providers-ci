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
		app         = kingpin.New("auto-test", "Automated Test Tool for Upbound Official Providers").DefaultEnvars()
		description = app.Flag("pr-description", "Description of Pull Request. Value of this option will be used to trigger/configure the tests."+
			"TEST_INPUT keyword is searched and if found the automated tests is triggered."+
			"The possible usages for TEST_INPUT:\n"+
			"'modified': Modified files in the example directories of providers are used as test inputs.\n"+
			"'provider-aws/examples-generated/s3/bucket.yaml,provider-gcp/examples-generated/storage/bucket.yaml': "+
			"The comma separated resources are used as test inputs.\n"+
			"If this option is not set, 'PR_DESCRIPTION' env var is used as default.").Envar("PR_DESCRIPTION").String()
		modifiedFiles = app.Flag("modified-files", "Modified Files in the example directories of providers. If the test case is determined as 'modified', "+
			"the value of this option is used as input.\nIf this option is not set, 'MODIFIED_FILES' env var is used as default.").Envar("MODIFIED_FILES").String()
		providerName = app.Flag("provider", "The provider name to run the tests.\n"+
			"If this option is not set, 'PROVIDER_NAME' env var is used as default.").Envar("PROVIDER_NAME").String()
		dataSourcePath = app.Flag("data-source", "File path of data source that will be used for injection some values.").String()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	o := &pkg.AutomatedTestOptions{
		Description:    *description,
		ModifiedFiles:  *modifiedFiles,
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
