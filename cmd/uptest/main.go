// Copyright 2022 Upbound Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// main package for the uptest tooling.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/upbound/uptest/internal"
	"github.com/upbound/uptest/internal/config"
	"github.com/upbound/uptest/internal/crdschema"
)

var (
	app = kingpin.New("uptest", "Automated Test Tool for Upbound Official Providers").DefaultEnvars()
	// e2e command
	e2e = app.Command("e2e", "Run e2e tests for manifests by applying them to a control plane and waiting until a given condition is met.")
	// crddiff command and sub-commands
	cmdCRDDiff  = app.Command("crddiff", "A tool for checking breaking API changes between two CRD OpenAPI v3 schemas. The schemas can come from either two revisions of a CRD, or from the versions declared in a single CRD.")
	cmdRevision = cmdCRDDiff.Command("revision", "Compare the first schema available in a base CRD against the first schema from a revision CRD")
	cmdSelf     = cmdCRDDiff.Command("self", "Use OpenAPI v3 schemas from a single CRD")
)

func main() {
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case e2e.FullCommand():
		e2eTests()
	case cmdRevision.FullCommand():
		crdDiffRevision()
	case cmdSelf.FullCommand():
		crdDiffSelf()
	}
}

var (
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
	defaultConditions = e2e.Flag("default-conditions", "Comma separated list of default conditions to wait for a successful test.\n"+
		"Conditions could be overridden per resource using \"uptest.upbound.io/conditions\" annotation.").Default("Ready").String()

	testDir = e2e.Flag("test-directory", "Directory where kuttl test case will be generated and executed.").Envar("UPTEST_TEST_DIR").Default(filepath.Join(os.TempDir(), "uptest-e2e")).String()
)

func e2eTests() {
	cd, err := os.Getwd()
	if err != nil {
		kingpin.FatalIfError(err, "cannot get current directory")
	}

	list := strings.Split(*manifestList, ",")
	examplePaths := make([]string, 0, len(list))
	for _, e := range list {
		if e == "" {
			continue
		}
		examplePaths = append(examplePaths, filepath.Join(cd, filepath.Clean(e)))
	}
	if len(examplePaths) == 0 {
		kingpin.Fatalf("No manifest to test provided.")
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

var (
	baseCRDPath     = cmdRevision.Arg("base", "The manifest file path of the CRD to be used as the base").Required().ExistingFile()
	revisionCRDPath = cmdRevision.Arg("revision", "The manifest file path of the CRD to be used as a revision to the base").Required().ExistingFile()
)

func crdDiffRevision() {
	crdDiff, err := crdschema.NewRevisionDiff(*baseCRDPath, *revisionCRDPath)
	kingpin.FatalIfError(err, "Failed to load CRDs")
	reportDiff(crdDiff)
}

var (
	crdPath = cmdSelf.Arg("crd", "The manifest file path of the CRD whose versions are to be checked for breaking changes").Required().ExistingFile()
)

func crdDiffSelf() {
	crdDiff, err := crdschema.NewSelfDiff(*crdPath)
	kingpin.FatalIfError(err, "Failed to load CRDs")
	reportDiff(crdDiff)
}

func reportDiff(crdDiff crdschema.SchemaCheck) {
	versionMap, err := crdDiff.GetBreakingChanges()
	kingpin.FatalIfError(err, "Failed to compute CRD breaking API changes")

	l := log.New(os.Stderr, "", 0)
	breakingDetected := false
	for v, d := range versionMap {
		if d.Empty() {
			continue
		}
		breakingDetected = true
		l.Printf("Version %q:\n", v)
		l.Println(crdschema.GetDiffReport(d))
	}
	if breakingDetected {
		syscall.Exit(1)
	}
}
