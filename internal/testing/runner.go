package testing

import (
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/upbound/provider-tools/internal/testing/config"
)

func RunTest(o *config.AutomatedTest) error {
	// Read examples and inject data source values to manifests
	manifests, err := NewPreparer(o.ManifestPaths, WithDataSource(o.DataSourcePath)).PrepareManifests()
	if err != nil {
		return errors.Wrap(err, "cannot prepare manifests")
	}

	// Prepare assert environment and run tests
	if err := NewTester(manifests, o).ExecuteTests(); err != nil {
		return errors.Wrap(err, "cannot execute tests")
	}

	return nil
}
