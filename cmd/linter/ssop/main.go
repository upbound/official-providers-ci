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

// main package for the ssop-linter tool,
// a linter for checking the packages of a provider family.
package main

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
	admv1 "k8s.io/api/admissionregistration/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	xpkgparser "github.com/crossplane/crossplane-runtime/pkg/parser"
	xpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	pkgmetav1alpha1 "github.com/crossplane/crossplane/apis/pkg/meta/v1alpha1"
)

const (
	streamFile  = "package.yaml"
	labelFamily = "pkg.crossplane.io/provider-family"
)

var (
	metaScheme, objScheme *runtime.Scheme
)

type ssopLinterConfig struct {
	crdDir          *string
	providerName    *string
	providerVersion *string
	packageRepoOrg  *string
}

func main() {
	app := kingpin.New("ssop-linter", "Linter for the official provider families").DefaultEnvars()
	// family command
	familyCmd := app.Command("family", "Checks whether all CRDs generated for a provider family are packaged in the corresponding service-scoped provider and checks the provider metadata.")

	config := &ssopLinterConfig{}
	config.crdDir = familyCmd.Flag("crd-dir", "Directory containing all the generated CRDs for the provider family.").Envar("CRD_DIR").Default("./package/crds").ExistingDir()
	config.providerName = familyCmd.Flag("provider-name", `Provider name such as "aws".`).Envar("PROVIDER_NAME").Required().String()
	config.providerVersion = familyCmd.Flag("provider-version", `Provider family tag to check such as "v0.37.0".`).Envar("PROVIDER_NAME").Required().String()
	config.packageRepoOrg = familyCmd.Flag("package-repo-org", `Package repo organization with the registry host for the provider family.`).Envar("PACKAGE_REPO_ORG").Default("xpkg.upbound.io/upbound-release-candidates").String()

	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))
	if cmd == familyCmd.FullCommand() {
		kingpin.FatalIfError(lint(config), "Linter error reported: ")
	}
}

func lint(config *ssopLinterConfig) error { //nolint:gocyclo // sequential flow easier to follow
	packageURLFormat := *config.packageRepoOrg + "/%s"
	packageURLFormatTagged := packageURLFormat + ":%s"
	familyConfigPackageName := fmt.Sprintf("provider-family-%s", *config.providerName)
	familyConfigPackageRef := fmt.Sprintf(packageURLFormat, familyConfigPackageName)

	entries, err := os.ReadDir(*config.crdDir)
	if err != nil {
		return errors.Wrapf(err, "failed to list CRDs from directory: %s", *config.crdDir)
	}

	metaMap := make(map[string]*xpkgparser.Package)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return errors.Wrap(err, "failed to get directory entry info")
		}
		if info.IsDir() {
			continue
		}

		crd, err := loadCRD(filepath.Join(*config.crdDir, e.Name()))
		if err != nil {
			return errors.Wrapf(err, "failed to load CRD from path: %s", e.Name())
		}
		log.Println("Checking CRD: ", crd.Name)
		group := strings.Split(crd.Spec.Group, ".")[0]

		if _, ok := metaMap[group]; !ok {
			repoName := fmt.Sprintf("provider-%s-%s", *config.providerName, group)
			if group == *config.providerName {
				repoName = familyConfigPackageName
			}

			packageURL := fmt.Sprintf(packageURLFormatTagged, repoName, *config.providerVersion)
			xpkg, err := getPackageMetadata(context.TODO(), packageURL)
			if err != nil {
				return errors.Wrapf(err, "failed to get package metadata for provider package: %s", packageURL)
			}
			metaMap[group] = xpkg
		}

		// check if the provider contains the CRD
		found := false
		for _, o := range metaMap[group].GetObjects() {
			pCRD := o.(*extv1.CustomResourceDefinition)
			if pCRD.Name == crd.Name {
				found = true
				break
			}
		}
		if !found {
			log.Fatalln("CRD not found: ", e.Name())
		}

		// check if the Provider.pkg has a family label
		foundMeta := false
		for _, o := range metaMap[group].GetMeta() {
			m, ok := o.(*pkgmetav1alpha1.Provider)
			if !ok {
				continue
			}
			foundMeta = true

			// check family label
			foundLabel := false
			for k, v := range m.Labels {
				if k == labelFamily && v == familyConfigPackageName {
					foundLabel = true
					break
				}
			}
			if !foundLabel {
				log.Fatalln("Family label not found: ", e.Name())
			}

			// check dependency to family config package
			if group != *config.providerName && (len(m.Spec.DependsOn) != 1 || m.Spec.DependsOn[0].Provider == nil || *m.Spec.DependsOn[0].Provider != familyConfigPackageRef) {
				log.Fatalln("Missing dependency to family config package: ", e.Name())
			}
			break
		}
		if !foundMeta {
			log.Fatalln("Provider package metadata not found: ", e.Name())
		}
	}
	return nil
}

func loadCRD(f string) (*extv1.CustomResourceDefinition, error) {
	do := json.NewSerializerWithOptions(json.DefaultMetaFactory, objScheme, objScheme, json.SerializerOptions{Yaml: true})
	buff, err := os.ReadFile(filepath.Clean(f))
	if err != nil {
		return nil, err
	}
	o, _, err := do.Decode(buff, nil, nil)
	if err != nil {
		return nil, err
	}
	return o.(*extv1.CustomResourceDefinition), nil
}

type readCloser struct {
	reader io.Reader
}

func (r *readCloser) Close() error {
	return nil
}

func (r *readCloser) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func getPackageMetadata(ctx context.Context, packageURL string) (*xpkgparser.Package, error) {
	ref, err := name.ParseReference(packageURL)
	if err != nil {
		return nil, err
	}
	img, err := remote.Image(ref)
	if err != nil {
		return nil, err
	}
	cfgFile, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}
	digest := ""
	for k, v := range cfgFile.Config.Labels {
		if strings.Contains(v, "base") {
			digest = strings.Join(strings.Split(k, ":")[1:], ":")
		}
	}
	layer, err := getBaseLayer(img, digest)
	if err != nil {
		return nil, err
	}
	return extractPackageMetadata(ctx, layer)
}

func getBaseLayer(img v1.Image, digest string) (v1.Layer, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, errors.Wrap(err, "cannot get image layers")
	}
	var layer v1.Layer
	for _, l := range layers {
		d, err := l.Digest()
		if err != nil {
			return nil, errors.Wrap(err, "cannot compute image layer's digest")
		}
		if d.String() == digest {
			layer = l
			break
		}
	}
	if layer == nil {
		return nil, errors.New("cannot find the base layer in the image")
	}
	return layer, nil
}

func extractPackageMetadata(ctx context.Context, layer v1.Layer) (*xpkgparser.Package, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			// End of tar archive
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name != streamFile {
			continue
		}
		parser := xpkgparser.New(metaScheme, objScheme)
		xpkg, err := parser.Parse(ctx, &readCloser{
			reader: tr,
		})
		return xpkg, errors.Wrap(err, "cannot parse package metadata")
	}
	return nil, errors.Errorf("%s not found in the base layer", streamFile)
}

func init() {
	metaScheme = runtime.NewScheme()
	if err := pkgmetav1alpha1.SchemeBuilder.AddToScheme(metaScheme); err != nil {
		kingpin.FatalIfError(err, "Failed to add package metadata v1alpha1 APIs to the runtime scheme: ")
	}
	if err := pkgmetav1.SchemeBuilder.AddToScheme(metaScheme); err != nil {
		kingpin.FatalIfError(err, "Failed to add package metadata v1 APIs to the runtime scheme: ")
	}

	objScheme = runtime.NewScheme()
	if err := xpv1.AddToScheme(objScheme); err != nil {
		kingpin.FatalIfError(err, "Failed to add Crossplane extension v1 APIs to the runtime scheme: ")
	}
	if err := extv1beta1.AddToScheme(objScheme); err != nil {
		kingpin.FatalIfError(err, "Failed to add Crossplane extension v1beta1 APIs to the runtime scheme: ")
	}
	if err := extv1.AddToScheme(objScheme); err != nil {
		kingpin.FatalIfError(err, "Failed to add Kubernetes API Server extension v1 APIs to the runtime scheme: ")
	}
	if err := admv1.AddToScheme(objScheme); err != nil {
		kingpin.FatalIfError(err, "Failed to add Kubernetes admission v1 APIs to the runtime scheme: ")
	}
}
