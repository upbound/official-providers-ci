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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestNewDiff(t *testing.T) {
	type args struct {
		basePath     string
		revisionPath string
	}
	tests := map[string]struct {
		reason  string
		args    args
		wantErr bool
	}{
		"NonExistentBaseCRDPath": {
			reason: "Base CRD does not exist at the specified path",
			args: args{
				basePath:     "non-existent",
				revisionPath: "testdata/base.yaml",
			},
			wantErr: true,
		},
		"NonExistentRevisionCRDPath": {
			reason: "Base CRD does not exist at the specified path",
			args: args{
				basePath:     "testdata/base.yaml",
				revisionPath: "non-existent",
			},
			wantErr: true,
		},
		"NonExistentBaseAndRevisionCRDPaths": {
			reason: "Base and revision CRDs do not exist at the specified paths",
			args: args{
				basePath:     "non-existent",
				revisionPath: "non-existent",
			},
			wantErr: true,
		},
		"ValidBaseAndRevisionCRDPaths": {
			reason: "Base and revision CRDs both exist",
			args: args{
				basePath:     "testdata/base.yaml",
				revisionPath: "testdata/base.yaml",
			},
		},
		"InvalidBaseCRD": {
			reason: "Invalid base CRD",
			args: args{
				basePath:     "testdata/invalid.yaml",
				revisionPath: "testdata/base.yaml",
			},
			wantErr: true,
		},
		"InvalidRevisionCRD": {
			reason: "Invalid base CRD",
			args: args{
				basePath:     "testdata/base.yaml",
				revisionPath: "testdata/invalid.yaml",
			},
			wantErr: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := NewDiff(tt.args.basePath, tt.args.revisionPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("\n%s\nNewDiff(...): error = %v, wantErr = %v", tt.reason, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if got.baseCRD == nil {
				t.Errorf("\n%s\nNewDiff(...): Loaded base CRD is not expected to be nil", tt.reason)
			}
			if got.revisionCRD == nil {
				t.Errorf("\n%s\nNewDiff(...): Loaded revision CRD is not expected to be nil", tt.reason)
			}
		})
	}
}

func Test_GetBreakingChanges(t *testing.T) {
	valTrue := true
	valFalse := false
	type want struct {
		errExpected     bool
		breakingChanges string
	}
	type args struct {
		basePath          string
		revisionModifiers []crdModifier
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"IdenticalBaseAndRevision": {
			reason: "No diff should be reported if base and revision CRDs are identical",
			args: args{
				basePath: "testdata/base.yaml",
			},
		},
		"NewOptionalFieldInRevision": {
			reason: "No diff should be reported if the revision acquires a new optional field",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						addSpecForProviderProperty(r, "newField", v1.JSONSchemaProps{
							Type: "string",
						}, &valFalse)
					},
				},
			},
		},
		"NewRequiredFieldInRevision": {
			reason: "A new required field in the revision is a breaking API change",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						addSpecForProviderProperty(r, "newField", v1.JSONSchemaProps{
							Type: "string",
						}, &valTrue)
					},
				},
			},
			want: want{
				breakingChanges: `
- Schema changed
  - Properties changed
    - Modified property: spec
      - Properties changed
        - Modified property: forProvider
          - Required changed
            - New required property: newField
          - Properties changed
            - New property: newField`,
			},
		},
		"NewEnumConstantInRevision": {
			reason: "No diff should be reported if a new enum constant is added to a field",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						p := getSpecForProviderProperty(r, "testEnum")
						p.Enum = append(p.Enum, v1.JSON{Raw: []byte(`"Const3"`)})
						addSpecForProviderProperty(r, "testEnum", p, nil)
					},
				},
			},
		},
		"ExistingEnumConstantRemovedInRevision": {
			reason: "Removing an existing enum constant is a breaking API change",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						p := getSpecForProviderProperty(r, "testEnum")
						p.Enum = p.Enum[1:]
						addSpecForProviderProperty(r, "testEnum", p, nil)
					},
				},
			},
			want: want{
				breakingChanges: `
- Schema changed
  - Properties changed
    - Modified property: spec
      - Properties changed
        - Modified property: forProvider
          - Properties changed
            - Modified property: testEnum
              - Deleted enum values: [Const1]`,
			},
		},
		"ExistingFieldTypeChangeInRevision": {
			reason: "Changing the type of an existing field is a breaking API change",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						p := getSpecForProviderProperty(r, "certificateChain")
						p.Type = "int"
						addSpecForProviderProperty(r, "certificateChain", p, nil)
					},
				},
			},
			want: want{
				breakingChanges: `
- Schema changed
  - Properties changed
    - Modified property: spec
      - Properties changed
        - Modified property: forProvider
          - Properties changed
            - Modified property: certificateChain
              - Type changed from 'string' to 'int'`,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			diff, err := newDiffWithModifiers(tt.args.basePath, tt.args.basePath, tt.args.revisionModifiers...)
			if err != nil {
				t.Errorf("\n%s\nnewDiffWithModifiers(...): failed to load base or revision CRD:\n%v", tt.reason, err)
				return
			}
			d, err := diff.GetBreakingChanges()
			if (err != nil) != tt.want.errExpected {
				t.Errorf("\n%s\nGetBreakingChanges(): error = %v, wantErr = %v", tt.reason, err, tt.want.errExpected)
				return
			}
			if err != nil {
				return
			}
			if (len(tt.want.breakingChanges) == 0) != d.Empty() {
				t.Errorf("\n%s\nGetBreakingChanges(): (len(breakingChanges) == 0) = %v, isEmpty = %v, diff = \n%s", tt.reason, len(tt.want.breakingChanges) == 0, d.Empty(), GetDiffReport(d))
				return
			}
			got := GetDiffReport(d)
			if diff := cmp.Diff(strings.TrimSpace(tt.want.breakingChanges), strings.TrimSpace(got)); diff != "" {
				t.Errorf("\n%s\nGetDiffReport(...): -want, +got:\n%s", tt.reason, diff)
			}
		})
	}
}

type crdModifier func(crd *v1.CustomResourceDefinition)

func newDiffWithModifiers(basePath, revisionPath string, revisionModifiers ...crdModifier) (*Diff, error) {
	d, err := NewDiff(basePath, revisionPath)
	if err != nil {
		return nil, err
	}
	for _, m := range revisionModifiers {
		m(d.revisionCRD)
	}
	return d, nil
}

func getSpecForProviderProperty(crd *v1.CustomResourceDefinition, fieldName string) v1.JSONSchemaProps {
	return crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["forProvider"].Properties[fieldName]
}

func addSpecForProviderProperty(crd *v1.CustomResourceDefinition, fieldName string, p v1.JSONSchemaProps, isRequired *bool) {
	forProvider := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["forProvider"]
	forProvider.Properties[fieldName] = p
	if isRequired != nil && *isRequired {
		required := append(forProvider.Required, fieldName)
		forProvider.Required = required
	}
	crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["forProvider"] = forProvider
}
