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
		dataSourcePath = app.Flag("data-source", "File path of data source that will be used for injection some values.").Default("").String()
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
		ExamplePaths:   examplePaths,
		DataSourcePath: *dataSourcePath,
	}
	kingpin.FatalIfError(pkg.RunTest(o), "cannot run automated tests successfully")
}
