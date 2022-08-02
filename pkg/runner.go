package pkg

import (
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func RunTest(o *AutomatedTestOptions) error {
	var testFilePaths []string
	customInputList := strings.Split(o.ExampleList, ",")
	for _, customInput := range customInputList {
		if strings.Contains(customInput, o.ProviderName) {
			testFilePaths = append(testFilePaths, customInput)
		}
	}

	if len(testFilePaths) == 0 {
		log.Warnf("The file to test for %s was not found. Skipped...", o.ProviderName)
		return nil
	}

	// Read examples and inject data source values to manifests
	p := NewPreparer(testFilePaths, WithDataSource(o.DataSourcePath))
	manifests, err := p.PrepareManifests(o.RootDirectory, o.ProviderCredentials)
	if err != nil {
		return errors.Wrap(err, "cannot prepare manifests")
	}

	// Prepare assert environment and run tests
	if err := NewTester(manifests).ExecuteTests(o.RootDirectory, o.ProviderName); err != nil {
		return errors.Wrap(err, "cannot execute tests")
	}

	return nil
}
