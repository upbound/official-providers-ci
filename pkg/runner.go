package pkg

import (
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func RunTest(o *AutomatedTestOptions) error {
	var testFilePaths []string
	testInput := strings.Split(strings.Split(o.Description, inputKeyword)[1], `"`)[1]
	if testInput == defaultCase {
		testFilePaths = strings.Split(o.ModifiedFiles, ",")
	} else {
		customInputList := strings.Split(testInput, ",")
		for _, customInput := range customInputList {
			if strings.Contains(customInput, o.ProviderName) {
				testFilePaths = append(testFilePaths, customInput)
			}
		}
	}
	if len(testFilePaths) == 0 {
		log.Warnf("The file to test for %s was not found. Skipped...", o.ProviderName)
		return nil
	}

	// Read examples and inject data source values to manifests
	p := &Preparer{
		testFilePaths:  testFilePaths,
		dataSourcePath: o.DataSourcePath,
	}
	inputs, err := p.PrepareManifests(o.RootDirectory, o.ProviderCredentials)
	if err != nil {
		return errors.Wrap(err, "cannot write manifests")
	}

	// Prepare assert environment and run tests
	t := &Tester{
		inputs: inputs,
	}
	if err := t.ExecuteTests(p.testFilePaths, o.RootDirectory, o.ProviderName); err != nil {
		return errors.Wrap(err, "cannot successfully completed automated tests")
	}

	return nil
}
