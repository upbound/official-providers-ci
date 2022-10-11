package internal

import (
	"github.com/pkg/errors"
)

func RunTest(o *AutomatedTestOptions) error {
	// Read examples and inject data source values to manifests
	manifests, err := NewPreparer(o.ExamplePaths, WithDataSource(o.DataSourcePath)).PrepareManifests()
	if err != nil {
		return errors.Wrap(err, "cannot prepare manifests")
	}

	// Prepare assert environment and run tests
	if err := NewTester(manifests).ExecuteTests(); err != nil {
		return errors.Wrap(err, "cannot execute tests")
	}

	return nil
}
