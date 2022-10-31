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
		dataSourcePath = e2e.Flag("data-source", "File path of data source that will be used for injection some values.").Envar("UPTEST_DATASOURCE_PATH").Default("").String()
		setupScript    = e2e.Flag("setup-script", "Script that will be executed before running tests.").Default("").String()
		teardownScript = e2e.Flag("teardown-script", "Script that will be executed after running tests.").Default("").String()

		defaultTimeout = e2e.Flag("default-timeout", "Default timeout in seconds for the test.\n"+
			"Timeout could be overridden per resource using \"uptest.upbound.io/timeout\" annotation.").Default("1200").Int()
		defaultConditions = e2e.Flag("default-conditions", "Comma seperated list of default conditions to wait for a successful test.\n"+
			"Conditions could be overridden per resource using \"uptest.upbound.io/conditions\" annotation.").Default("Ready").String()

		testDir = e2e.Flag("test-directory", "Directory where kuttl test case will be generated and executed.").Envar("UPTEST_TEST_DIR").Default(filepath.Join(os.TempDir(), "uptest-e2e")).String()
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

	setupPath := ""
	if *setupScript != "" {
		setupPath, err = filepath.Abs(*setupScript)
		if err != nil {
			kingpin.FatalIfError(err, "cannot get absolute path of setup script")
		}
	}

	teardownPath := ""
	if *teardownScript != "" {
		teardownPath, err = filepath.Abs(*teardownScript)
		if err != nil {
			kingpin.FatalIfError(err, "cannot get absolute path of teardown script")
		}
	}
	o := &config.AutomatedTest{
		ManifestPaths:      examplePaths,
		DataSourcePath:     *dataSourcePath,
		SetupScriptPath:    setupPath,
		TeardownScriptPath: teardownPath,
		DefaultConditions:  strings.Split(*defaultConditions, ","),
		DefaultTimeout:     *defaultTimeout,
		Directory:          *testDir,
	}

	kingpin.FatalIfError(internal.RunTest(o), "cannot run e2e tests successfully")
}
