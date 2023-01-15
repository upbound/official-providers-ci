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

package crdschema

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pkg/errors"
	"github.com/tufin/oasdiff/diff"
	"github.com/tufin/oasdiff/report"
	"golang.org/x/mod/semver"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiyaml "k8s.io/apimachinery/pkg/util/yaml"
	k8syaml "sigs.k8s.io/yaml"
)

const (
	contentTypeJSON = "application/json"

	errCRDLoad                        = "failed to load the CustomResourceDefinition"
	errBreakingRevisionChangesCompute = "failed to compute breaking changes in base and revision CRD schemas"
	errBreakingSelfVersionsCompute    = "failed to compute breaking changes in the versions of a CRD"
)

// SchemaCheck represents a schema checker that can return the set of breaking
// API changes between schemas.
type SchemaCheck interface {
	GetBreakingChanges() (map[string]*diff.Diff, error)
}

// RevisionDiff can compute schema changes between the base CRD found at `basePath`
// and the revision CRD found at `revisionPath`.
type RevisionDiff struct {
	baseCRD     *v1.CustomResourceDefinition
	revisionCRD *v1.CustomResourceDefinition
}

// NewRevisionDiff returns a new RevisionDiff initialized with
// the base and revision CRDs loaded from the specified
// base and revision CRD paths.
func NewRevisionDiff(basePath, revisionPath string) (*RevisionDiff, error) {
	d := &RevisionDiff{}
	var err error
	d.baseCRD, err = loadCRD(basePath)
	if err != nil {
		return nil, errors.Wrap(err, errCRDLoad)
	}
	d.revisionCRD, err = loadCRD(revisionPath)
	if err != nil {
		return nil, errors.Wrap(err, errCRDLoad)
	}
	return d, nil
}

// SelfDiff can compute schema changes between the consecutive versions
// declared for a CRD.
type SelfDiff struct {
	crd *v1.CustomResourceDefinition
}

// NewSelfDiff returns a new SelfDiff initialized with a CRD loaded
// from the specified path.
func NewSelfDiff(crdPath string) (*SelfDiff, error) {
	d := &SelfDiff{}
	var err error
	d.crd, err = loadCRD(crdPath)
	if err != nil {
		return nil, errors.Wrap(err, errCRDLoad)
	}
	return d, nil
}

func loadCRD(m string) (*v1.CustomResourceDefinition, error) {
	crd := &v1.CustomResourceDefinition{}
	buff, err := os.ReadFile(filepath.Clean(m))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load the CRD manifest from file: %s", m)
	}
	if err := apiyaml.Unmarshal(buff, crd); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal CRD manifest from file: %s", m)
	}
	return crd, nil
}

func getOpenAPIv3Document(crd *v1.CustomResourceDefinition) ([]*openapi3.T, error) {
	schemas := make([]*openapi3.T, 0, len(crd.Spec.Versions))
	for _, v := range crd.Spec.Versions {
		if v.Schema == nil || v.Schema.OpenAPIV3Schema == nil {
			return nil, errors.Errorf("invalid CRD manifest: CRD's .Spec.Versions[%q].Schema.OpenAPIV3Schema cannot be nil", v.Name)
		}
		t := &openapi3.T{
			Info: &openapi3.Info{
				Version: v.Name,
			},
			Paths: make(openapi3.Paths),
		}
		c := make(openapi3.Content)
		t.Paths["/crd"] = &openapi3.PathItem{
			Put: &openapi3.Operation{
				RequestBody: &openapi3.RequestBodyRef{
					Value: &openapi3.RequestBody{
						Content: c,
					},
				},
			},
		}
		s := &openapi3.Schema{}
		c[contentTypeJSON] = &openapi3.MediaType{
			Schema: &openapi3.SchemaRef{
				Value: s,
			},
		}

		// convert from CRD validation schema to openAPI v3 schema
		buff, err := k8syaml.Marshal(v.Schema.OpenAPIV3Schema)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal CRD validation schema")
		}
		if err := k8syaml.Unmarshal(buff, s); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal CRD validation schema into openAPI v3 schema")
		}
		schemas = append(schemas, t)
	}
	return schemas, nil
}

// GetBreakingChanges returns the breaking changes found in the
// consecutive versions of a CRD.
func (d *SelfDiff) GetBreakingChanges() (map[string]*diff.Diff, error) {
	selfDocs, err := getOpenAPIv3Document(d.crd)
	if err != nil {
		return nil, errors.Wrap(err, errBreakingSelfVersionsCompute)
	}
	diffMap := make(map[string]*diff.Diff)
	if len(selfDocs) < 2 {
		return diffMap, nil
	}
	sortVersions(selfDocs)
	prev := 0
	for prev < len(selfDocs)-1 {
		revisionDoc := selfDocs[prev+1]
		sd, err := schemaDiff(selfDocs[prev], revisionDoc)
		if err != nil {
			return nil, errors.Wrap(err, errBreakingSelfVersionsCompute)
		}
		diffMap[revisionDoc.Info.Version] = sd
		prev++
	}
	return diffMap, nil
}

func sortVersions(versions []*openapi3.T) {
	versionNames := make([]string, 0, len(versions))
	for _, t := range versions {
		versionNames = append(versionNames, t.Info.Version)
	}
	semver.Sort(versionNames)
	for i, v := range versionNames {
		for j := range versions {
			if versions[j].Info.Version != v {
				continue
			}
			versions[i], versions[j] = versions[j], versions[i]
			break
		}
	}
}

// GetBreakingChanges returns a diff representing
// the detected breaking schema changes between the base and revision CRDs.
func (d *RevisionDiff) GetBreakingChanges() (map[string]*diff.Diff, error) {
	baseDocs, err := getOpenAPIv3Document(d.baseCRD)
	if err != nil {
		return nil, errors.Wrap(err, errBreakingRevisionChangesCompute)
	}
	revisionDocs, err := getOpenAPIv3Document(d.revisionCRD)
	if err != nil {
		return nil, errors.Wrap(err, errBreakingRevisionChangesCompute)
	}

	diffMap := make(map[string]*diff.Diff, len(baseDocs))
	for i, baseDoc := range baseDocs {
		versionName := baseDoc.Info.Version
		if i >= len(revisionDocs) || revisionDocs[i].Info.Version != versionName {
			// no corresponding version to compare in the revision
			return nil, errors.Errorf("revision has no corresponding version to compare with the base for the version name: %s", versionName)
		}
		sd, err := schemaDiff(baseDoc, revisionDocs[i])
		if err != nil {
			return nil, errors.Wrap(err, errBreakingRevisionChangesCompute)
		}
		diffMap[versionName] = sd
	}
	return filterNonBreaking(diffMap), nil
}

var crdPutEndpoint = diff.Endpoint{
	Method: "PUT",
	Path:   "/crd",
}

func filterNonBreaking(diffMap map[string]*diff.Diff) map[string]*diff.Diff {
	for v, d := range diffMap {
		if d.Empty() {
			continue
		}
		sd := d.EndpointsDiff.Modified[crdPutEndpoint].RequestBodyDiff.ContentDiff.MediaTypeModified[contentTypeJSON].SchemaDiff
		ignoreOptionalNewProperties(sd)
		if sd != nil && empty(sd.PropertiesDiff) {
			sd.PropertiesDiff = nil
		}
		if sd == nil || sd.Empty() {
			delete(diffMap, v)
		}
	}
	return diffMap
}

func ignoreOptionalNewProperties(sd *diff.SchemaDiff) { // nolint:gocyclo
	if sd == nil || sd.Empty() {
		return
	}
	if sd.PropertiesDiff != nil {
		// optional new fields are non-breaking
		filteredAddedProps := make(diff.StringList, 0, len(sd.PropertiesDiff.Added))
		if sd.RequiredDiff != nil {
			for _, f := range sd.PropertiesDiff.Added {
				for _, r := range sd.RequiredDiff.Added {
					if f == r {
						filteredAddedProps = append(filteredAddedProps, f)
						break
					}
				}
			}
		}
		sd.PropertiesDiff.Added = filteredAddedProps
		for n, csd := range sd.PropertiesDiff.Modified {
			ignoreOptionalNewProperties(csd)
			if csd != nil && empty(csd.PropertiesDiff) {
				csd.PropertiesDiff = nil
			}
			if csd == nil || csd.Empty() {
				delete(sd.PropertiesDiff.Modified, n)
			}
		}
		if empty(sd.PropertiesDiff) {
			sd.PropertiesDiff = nil
		}
	}
	ignoreOptionalNewProperties(sd.ItemsDiff)
	if sd.ItemsDiff != nil && sd.ItemsDiff.Empty() {
		sd.ItemsDiff = nil
	}
}

func empty(sd *diff.SchemasDiff) bool {
	if sd == nil || sd.Empty() {
		return true
	}
	if len(sd.Added) != 0 || len(sd.Deleted) != 0 {
		return false
	}
	for _, csd := range sd.Modified {
		if csd != nil && !csd.Empty() {
			return false
		}
	}
	return true
}

func schemaDiff(baseDoc, revisionDoc *openapi3.T) (*diff.Diff, error) {
	config := &diff.Config{
		ExcludeExamples:    true,
		ExcludeDescription: true,
	}
	sd, err := diff.Get(config, baseDoc, revisionDoc)
	return sd, errors.Wrap(err, "failed to compute breaking changes between OpenAPI v3 schemas")
}

// GetDiffReport is a utility function to format the specified diff as a string
func GetDiffReport(d *diff.Diff) string {
	if d.Empty() {
		return ""
	}
	l := strings.Split(report.GetTextReportAsString(d), "\n")
	l = l[12:]
	for i, s := range l {
		l[i] = strings.TrimPrefix(s, "      ")
	}
	return strings.Join(l, "\n")
}
