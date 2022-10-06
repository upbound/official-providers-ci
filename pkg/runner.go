package pkg

import (
	"github.com/pkg/errors"
)

func RunTest(o *AutomatedTestOptions) error {
	// Read examples and inject data source values to manifests
	p := NewPreparer(o.ExamplePaths, WithDataSource(o.DataSourcePath))
	manifests, err := p.PrepareManifests(o.RootDirectory, o.ProviderCredentials)
	if err != nil {
		return errors.Wrap(err, "cannot prepare manifests")
	}

	// Prepare assert environment and run tests
	if err := NewTester(manifests).ExecuteTests(o.RootDirectory, o.ProviderName, o.SkipProviderConfig); err != nil {
		return errors.Wrap(err, "cannot execute tests")
	}

	return nil
}
