package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/upbound/uptest/internal"
	"github.com/upbound/uptest/internal/config"
)

func main() {
	cd, err := os.Getwd()
	if err != nil {
		kingpin.FatalIfError(err, "cannot get current directory")
	}

	var (
		app = kingpin.New("uptest", "Automated Test Tool for Upbound Official Providers").DefaultEnvars()

		e2e          = app.Command("e2e", "Run e2e tests for manifests by applying them to a control plane and waiting until a given condition is met.")
		manifestList = e2e.Arg("manifest-list", "List of manifests. Value of this option will be used to trigger/configure the tests."+
			"The possible usage:\n"+
			"'provider-aws/examples/s3/bucket.yaml,provider-gcp/examples/storage/bucket.yaml': "+
			"The comma separated resources are used as test inputs.\n"+
			"If this option is not set, 'MANIFEST_LIST' env var is used as default.").Envar("MANIFEST_LIST").String()
		dataSourcePath        = e2e.Flag("data-source", "File path of data source that will be used for injection some values.").Default("").String()
		defaultHooksDirectory = e2e.Flag("default-hooks-directory", "Path to hooks directory for default hooks to run for all examples.\n"+
			"This could be overridden per resource using \"upjet.upbound.io/hooks-directory\" annotation.").String()
		defaultTimeout = e2e.Flag("default-timeout", "Default timeout in seconds for the test.\n"+
			"Timeout could be overridden per resource using \"upjet.upbound.io/timeout\" annotation.").Default("1200").Int()
		defaultConditions = e2e.Flag("default-conditions", "Comma seperated list of default conditions to wait for a successful test.\n"+
			"Conditions could be overridden per resource using \"upjet.upbound.io/conditions\" annotation.").Default("Ready").String()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	var examplePaths []string
	for _, e := range strings.Split(*manifestList, ",") {
		if e == "" {
			continue
		}
		examplePaths = append(examplePaths, filepath.Join(cd, filepath.Clean(e)))
	}
	if len(examplePaths) == 0 {
		fmt.Println("No example files to test.")
		return
	}

	o := &config.AutomatedTest{
		ExamplePaths:          examplePaths,
		DataSourcePath:        *dataSourcePath,
		DefaultHooksDirectory: *defaultHooksDirectory,
		DefaultConditions:     strings.Split(*defaultConditions, ","),
		DefaultTimeout:        *defaultTimeout,
	}

	kingpin.FatalIfError(internal.RunTest(o), "cannot run automated tests successfully")
}
