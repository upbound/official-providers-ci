// Copyright 2023 Upbound Inc.
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

// main package for the provider-list tool that can be used to dump the names
// of the required provider family (service-scoped provider) packages
// that satisfy:
//   - All managed resources and Crossplane Compositions observed in a cluster.
//   - All Crossplane Compositions observed in the source tree of
//     a Crossplane Configuration package.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/alecthomas/kong"
	"github.com/pkg/errors"
	"github.com/upbound/upjet/pkg/migration"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	defaultKubeConfig = ".kube/config"
)

// Options represents the available subcommands of provider-list:
// "generate" and "upload".
type Options struct {
	RegistryOrg string `name:"regorg" required:"" default:"xpkg.upbound.io/upbound" help:"<registry host>/<organization> for the provider family packages."`
	Version     string `name:"family-version" required:"" help:"Version of the provider family packages."`
	// sub-commands
	Local struct {
		Path string `name:"path" required:"" help:"Source directory for the Crossplane Configuration package."`
	} `kong:"cmd"`
	Cluster struct {
		KubeConfig string `name:"kubeconfig" optional:"" help:"Path to the kubeconfig to use."`
	} `kong:"cmd"`
}

func main() {
	opts := &Options{}
	kongCtx := kong.Parse(opts, kong.Name("provider-list"),
		kong.Description("Upbound provider families package dependency listing tool"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:   true,
			FlagsLast: true,
			Summary:   true,
		}))

	r := migration.NewRegistry(runtime.NewScheme())
	r.RegisterPreProcessor(migration.CategoryManaged, migration.PreProcessor(GetSSOPNameFromManagedResource))
	r.RegisterPreProcessor(migration.CategoryComposition, migration.PreProcessor(GetSSOPNameFromComposition))
	kongCtx.FatalIfErrorf(r.AddCompositionTypes(), "Failed to register the Crossplane Composition types with the migration registry")

	var source migration.Source
	var err error
	switch kongCtx.Command() {
	case "local":
		source, err = localListing(opts)
	case "cluster":
		source, err = clusterListing(opts, r)
	}
	kongCtx.FatalIfErrorf(err, "Failed to initialize the migration source")

	pg := migration.NewPlanGenerator(r, source, nil, migration.WithSkipGVKs(schema.GroupVersionKind{}))
	kongCtx.FatalIfErrorf(pg.GeneratePlan(), "Failed to list the required provider family packages")
	providers := make([]string, 0, len(SSOPNames))
	for p := range SSOPNames {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	logger := log.New(os.Stdout, "", 0)
	for _, p := range providers {
		logger.Printf("%s", fmt.Sprintf("%s/%s:%s", opts.RegistryOrg, p, opts.Version))
	}
}

func clusterListing(opts *Options, r *migration.Registry) (migration.Source, error) {
	if len(opts.Cluster.KubeConfig) == 0 {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get user's home")
		}
		opts.Cluster.KubeConfig = filepath.Join(homeDir, defaultKubeConfig)
	}
	source, err := migration.NewKubernetesSourceFromKubeConfig(opts.Cluster.KubeConfig,
		migration.WithCategories([]migration.Category{"managed"}), migration.WithRegistry(r))
	return source, errors.Wrap(err, "failed to initialize the migration Kubernetes source")
}

func localListing(opts *Options) (migration.Source, error) {
	source, err := migration.NewFileSystemSource(opts.Local.Path)
	return source, errors.Wrap(err, "failed to initialize the migration FileSystem source")
}
