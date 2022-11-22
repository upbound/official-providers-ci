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
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pkg/errors"
	"github.com/tufin/oasdiff/diff"
	"github.com/tufin/oasdiff/report"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiyaml "k8s.io/apimachinery/pkg/util/yaml"
	k8syaml "sigs.k8s.io/yaml"
)

const (
	errCRDLoad                = "failed to load the CustomResourceDefinition"
	errBreakingchangesCompute = "failed to compute breaking changes in base and revision CRD schemas"
)

// Diff can compute schema changes between the base CRD found at `basePath`
// and the revision CRD found at `revisionPath`.
type Diff struct {
	baseCRD     *v1.CustomResourceDefinition
	revisionCRD *v1.CustomResourceDefinition
}

// NewDiff returns a new Diff initialized with the base and revision
// CRDs loaded from the specified base and revision CRD paths.
func NewDiff(basePath, revisionPath string) (*Diff, error) {
	d := &Diff{}
	var err error
	d.baseCRD, err = getCRD(basePath)
	if err != nil {
		return nil, errors.Wrap(err, errCRDLoad)
	}
	d.revisionCRD, err = getCRD(revisionPath)
	if err != nil {
		return nil, errors.Wrap(err, errCRDLoad)
	}
	return d, nil
}

func getCRD(m string) (*v1.CustomResourceDefinition, error) {
	crd := &v1.CustomResourceDefinition{}
	buff, err := os.ReadFile(m)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load the CRD manifest from file: %s", m)
	}
	if err := apiyaml.Unmarshal(buff, crd); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal CRD manifest from file: %s", m)
	}
	return crd, nil
}

func getOpenAPIv3Document(crd *v1.CustomResourceDefinition) (*openapi3.T, error) {
	if len(crd.Spec.Versions) != 1 {
		return nil, errors.New("invalid CRD manifest: Only CRDs with exactly one version are supported")
	}
	if crd.Spec.Versions[0].Schema == nil || crd.Spec.Versions[0].Schema.OpenAPIV3Schema == nil {
		return nil, errors.New("invalid CRD manifest: CRD's .Spec.Versions[0].Schema.OpenAPIV3Schema cannot be nil")
	}

	t := &openapi3.T{
		Info:  &openapi3.Info{},
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
	c["application/json"] = &openapi3.MediaType{
		Schema: &openapi3.SchemaRef{
			Value: s,
		},
	}

	// convert from CRD validation schema to openAPI v3 schema
	buff, err := k8syaml.Marshal(crd.Spec.Versions[0].Schema.OpenAPIV3Schema)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal CRD validation schema")
	}
	if err := k8syaml.Unmarshal(buff, s); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal CRD validation schema into openAPI v3 schema")
	}
	return t, nil
}

func (d *Diff) GetBreakingChanges() (*diff.Diff, error) {
	baseDoc, err := getOpenAPIv3Document(d.baseCRD)
	if err != nil {
		return nil, errors.Wrap(err, errBreakingchangesCompute)
	}
	revisionDoc, err := getOpenAPIv3Document(d.revisionCRD)
	if err != nil {
		return nil, errors.Wrap(err, errBreakingchangesCompute)
	}

	config := diff.NewConfig()
	// currently we only need to detect breaking API changes
	config.BreakingOnly = true
	sd, err := diff.Get(config, baseDoc, revisionDoc)
	return sd, errors.Wrap(err, errBreakingchangesCompute)
}

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
