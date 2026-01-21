// Copyright 2024 Upbound Inc.
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
	"testing"

	kinoapi "github.com/getkin/kin-openapi/openapi3"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/oasdiff/oasdiff/utils"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestFlattenDiff_RevisionDiff(t *testing.T) {
	type args struct {
		basePath          string
		revisionModifiers []crdModifier
	}
	type want struct {
		changes []SchemaChange
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoChanges": {
			reason: "No changes between base and revision should result in empty changes",
			args: args{
				basePath:          "testdata/base.yaml",
				revisionModifiers: []crdModifier{},
			},
			want: want{
				changes: nil,
			},
		},
		"AddObjectField": {
			reason: "Adding a new field to an object should be detected",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						addSpecForProviderProperty(r, 0, "newField", v1.JSONSchemaProps{
							Type:        "string",
							Description: "A new field",
						}, nil)
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:       "spec.forProvider.newField",
						PathParts:  []string{"spec", "forProvider", "newField"},
						ChangeType: ChangeTypeFieldAdded,
					},
				},
			},
		},
		"DeleteObjectField": {
			reason: "Deleting a field from an object should be detected",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						removeSpecForProviderProperty(r, 0, "tags")
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:       "spec.forProvider.tags",
						PathParts:  []string{"spec", "forProvider", "tags"},
						ChangeType: ChangeTypeFieldDeleted,
					},
				},
			},
		},
		"ChangeFieldType": {
			reason: "Changing the type of a field should be detected",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						p := getSpecForProviderProperty(r, 0, "certificateChain")
						p.Type = "integer"
						addSpecForProviderProperty(r, 0, "certificateChain", p, nil)
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:       "spec.forProvider.certificateChain",
						PathParts:  []string{"spec", "forProvider", "certificateChain"},
						ChangeType: ChangeTypeTypeChanged,
						TypeChangeDetails: &TypeChangeDetails{
							OldType: &kinoapi.Types{"string"},
							NewType: &kinoapi.Types{"integer"},
							Added:   utils.StringList{"integer"},
							Deleted: utils.StringList{"string"},
						},
					},
				},
			},
		},
		"AddFieldToArrayItems": {
			reason: "Adding a field to array item schema should be detected",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						validationOption := getSpecForProviderProperty(r, 0, "validationOption")
						if validationOption.Items.Schema.Properties == nil {
							validationOption.Items.Schema.Properties = make(map[string]v1.JSONSchemaProps)
						}
						validationOption.Items.Schema.Properties["newField"] = v1.JSONSchemaProps{
							Type: "string",
						}
						addSpecForProviderProperty(r, 0, "validationOption", validationOption, nil)
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:       "spec.forProvider.validationOption[*].newField",
						PathParts:  []string{"spec", "forProvider", "validationOption[*]", "newField"},
						ChangeType: ChangeTypeFieldAdded,
					},
				},
			},
		},
		"DeleteFieldFromArrayItems": {
			reason: "Deleting a field from array item schema should be detected",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						validationOption := getSpecForProviderProperty(r, 0, "validationOption")
						if validationOption.Items != nil && validationOption.Items.Schema != nil && validationOption.Items.Schema.Properties != nil {
							delete(validationOption.Items.Schema.Properties, "domainName")
						}
						addSpecForProviderProperty(r, 0, "validationOption", validationOption, nil)
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:       "spec.forProvider.validationOption[*].domainName",
						PathParts:  []string{"spec", "forProvider", "validationOption[*]", "domainName"},
						ChangeType: ChangeTypeFieldDeleted,
					},
				},
			},
		},
		"ChangeArrayItemFieldType": {
			reason: "Changing type of array item field should be detected",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						validationOption := getSpecForProviderProperty(r, 0, "validationOption")
						if validationOption.Items != nil && validationOption.Items.Schema != nil && validationOption.Items.Schema.Properties != nil {
							domainProp := validationOption.Items.Schema.Properties["domainName"]
							domainProp.Type = "integer"
							validationOption.Items.Schema.Properties["domainName"] = domainProp
						}
						addSpecForProviderProperty(r, 0, "validationOption", validationOption, nil)
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:       "spec.forProvider.validationOption[*].domainName",
						PathParts:  []string{"spec", "forProvider", "validationOption[*]", "domainName"},
						ChangeType: ChangeTypeTypeChanged,
						TypeChangeDetails: &TypeChangeDetails{
							OldType: &kinoapi.Types{"string"},
							NewType: &kinoapi.Types{"integer"},
							Added:   utils.StringList{"integer"},
							Deleted: utils.StringList{"string"},
						},
					},
				},
			},
		},
		"ChangeArrayToString": {
			reason: "Changing array to string should only report type change, not item deletion",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						addSpecForProviderProperty(r, 0, "subjectAlternativeNames", v1.JSONSchemaProps{
							Type: "string",
						}, nil)
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:       "spec.forProvider.subjectAlternativeNames",
						PathParts:  []string{"spec", "forProvider", "subjectAlternativeNames"},
						ChangeType: ChangeTypeTypeChanged,
						TypeChangeDetails: &TypeChangeDetails{
							OldType: &kinoapi.Types{"array"},
							NewType: &kinoapi.Types{"string"},
							Added:   utils.StringList{"string"},
							Deleted: utils.StringList{"array"},
						},
					},
				},
			},
		},
		"ChangeStringToArray": {
			reason: "Changing string to array should only report type change, not item addition",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						addSpecForProviderProperty(r, 0, "certificateChain", v1.JSONSchemaProps{
							Type: "array",
							Items: &v1.JSONSchemaPropsOrArray{
								Schema: &v1.JSONSchemaProps{
									Type: "string",
								},
							},
						}, nil)
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:       "spec.forProvider.certificateChain",
						PathParts:  []string{"spec", "forProvider", "certificateChain"},
						ChangeType: ChangeTypeTypeChanged,
						TypeChangeDetails: &TypeChangeDetails{
							OldType: &kinoapi.Types{"string"},
							NewType: &kinoapi.Types{"array"},
							Added:   utils.StringList{"array"},
							Deleted: utils.StringList{"string"},
						},
					},
				},
			},
		},
		"NestedObjectFieldAddition": {
			reason: "Adding a nested object with fields should be detected",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						// Add a nested object with multiple fields
						addSpecForProviderProperty(r, 0, "metadata", v1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]v1.JSONSchemaProps{
								"name":    {Type: "string"},
								"version": {Type: "string"},
							},
						}, nil)
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:       "spec.forProvider.metadata",
						PathParts:  []string{"spec", "forProvider", "metadata"},
						ChangeType: ChangeTypeFieldAdded,
					},
				},
			},
		},
		"MultipleChanges": {
			reason: "Multiple changes should all be detected",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						// Add a field
						addSpecForProviderProperty(r, 0, "newField", v1.JSONSchemaProps{
							Type: "string",
						}, nil)
						// Delete a field
						removeSpecForProviderProperty(r, 0, "tags")
						// Change a type
						p := getSpecForProviderProperty(r, 0, "certificateChain")
						p.Type = "integer"
						addSpecForProviderProperty(r, 0, "certificateChain", p, nil)
					},
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:       "spec.forProvider.certificateChain",
						PathParts:  []string{"spec", "forProvider", "certificateChain"},
						ChangeType: ChangeTypeTypeChanged,
						TypeChangeDetails: &TypeChangeDetails{
							OldType: &kinoapi.Types{"string"},
							NewType: &kinoapi.Types{"integer"},
							Added:   utils.StringList{"integer"},
							Deleted: utils.StringList{"string"},
						},
					},
					{
						Path:       "spec.forProvider.newField",
						PathParts:  []string{"spec", "forProvider", "newField"},
						ChangeType: ChangeTypeFieldAdded,
					},
					{
						Path:       "spec.forProvider.tags",
						PathParts:  []string{"spec", "forProvider", "tags"},
						ChangeType: ChangeTypeFieldDeleted,
					},
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			diff, err := newRevisionDiffWithModifiers(tt.args.basePath, tt.args.basePath, nil, tt.args.revisionModifiers...)
			if err != nil {
				t.Errorf("\n%s\nnewRevisionDiffWithModifiers(...): failed to load CRDs:\n%v", tt.reason, err)
				return
			}

			rawDiff, err := diff.GetRawDiff()
			if err != nil {
				t.Errorf("\n%s\nGetRawDiff(): error = %v", tt.reason, err)
				return
			}

			report, err := GetChangesAsStructured(rawDiff, true)
			if err != nil {
				t.Errorf("\n%s\nGetChangesAsStructured(): error = %v", tt.reason, err)
				return
			}

			var got []SchemaChange
			for _, versionChanges := range report.Versions {
				got = append(got, versionChanges.Changes...)
			}

			// Sort for comparison (order doesn't matter)
			sortChanges := cmpopts.SortSlices(func(a, b SchemaChange) bool {
				if a.Path != b.Path {
					return a.Path < b.Path
				}
				return a.ChangeType < b.ChangeType
			})

			// Ignore RawSchemaDiff in comparison
			ignoreRaw := cmpopts.IgnoreFields(SchemaChange{}, "RawSchemaDiff")

			if diff := cmp.Diff(tt.want.changes, got, sortChanges, ignoreRaw); diff != "" {
				t.Errorf("\n%s\nGetChangesAsStructured(): -want, +got:\n%s", tt.reason, diff)
			}
		})
	}
}

func TestFlattenDiff_SelfDiff(t *testing.T) {
	type args struct {
		crdPath     string
		crdModifier crdModifier
	}
	type want struct {
		changes []SchemaChange
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AddFieldInV1Beta2": {
			reason: "Adding a field in v1beta2 version should be detected",
			args: args{
				crdPath: "testdata/base.yaml",
				crdModifier: func(r *v1.CustomResourceDefinition) {
					addSpecForProviderProperty(r, 1, "newFieldV1Beta2", v1.JSONSchemaProps{
						Type: "string",
					}, nil)
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:       "spec.forProvider.newFieldV1Beta2",
						PathParts:  []string{"spec", "forProvider", "newFieldV1Beta2"},
						ChangeType: ChangeTypeFieldAdded,
					},
				},
			},
		},
		"TypeChangeAcrossVersions": {
			reason: "Type change between consecutive versions should be detected",
			args: args{
				crdPath: "testdata/base.yaml",
				crdModifier: func(r *v1.CustomResourceDefinition) {
					p := getSpecForProviderProperty(r, 1, "certificateChain")
					p.Type = "integer"
					addSpecForProviderProperty(r, 1, "certificateChain", p, nil)
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:       "spec.forProvider.certificateChain",
						PathParts:  []string{"spec", "forProvider", "certificateChain"},
						ChangeType: ChangeTypeTypeChanged,
						TypeChangeDetails: &TypeChangeDetails{
							OldType: &kinoapi.Types{"string"},
							NewType: &kinoapi.Types{"integer"},
							Added:   utils.StringList{"integer"},
							Deleted: utils.StringList{"string"},
						},
					},
				},
			},
		},
		"ArrayItemChangeInV1Beta2": {
			reason: "Changes to array items in v1beta2 should be detected",
			args: args{
				crdPath: "testdata/base.yaml",
				crdModifier: func(r *v1.CustomResourceDefinition) {
					validationOption := getSpecForProviderProperty(r, 1, "validationOption")
					if validationOption.Items.Schema.Properties == nil {
						validationOption.Items.Schema.Properties = make(map[string]v1.JSONSchemaProps)
					}
					validationOption.Items.Schema.Properties["newField"] = v1.JSONSchemaProps{
						Type: "string",
					}
					addSpecForProviderProperty(r, 1, "validationOption", validationOption, nil)
				},
			},
			want: want{
				changes: []SchemaChange{
					{
						Path:       "spec.forProvider.validationOption[*].newField",
						PathParts:  []string{"spec", "forProvider", "validationOption[*]", "newField"},
						ChangeType: ChangeTypeFieldAdded,
					},
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			diff, err := newSelfDiffWithModifiers(tt.args.crdPath, nil, tt.args.crdModifier)
			if err != nil {
				t.Errorf("\n%s\nnewSelfDiffWithModifiers(...): failed to load CRD:\n%v", tt.reason, err)
				return
			}

			rawDiff, err := diff.GetRawDiff()
			if err != nil {
				t.Errorf("\n%s\nGetRawDiff(): error = %v", tt.reason, err)
				return
			}

			report, err := GetChangesAsStructured(rawDiff, true)
			if err != nil {
				t.Errorf("\n%s\nGetChangesAsStructured(): error = %v", tt.reason, err)
				return
			}

			var got []SchemaChange
			for _, versionChanges := range report.Versions {
				got = append(got, versionChanges.Changes...)
			}

			sortChanges := cmpopts.SortSlices(func(a, b SchemaChange) bool {
				if a.Path != b.Path {
					return a.Path < b.Path
				}
				return a.ChangeType < b.ChangeType
			})

			ignoreRaw := cmpopts.IgnoreFields(SchemaChange{}, "RawSchemaDiff")

			if diff := cmp.Diff(tt.want.changes, got, sortChanges, ignoreRaw); diff != "" {
				t.Errorf("\n%s\nGetChangesAsStructured(): -want, +got:\n%s", tt.reason, diff)
			}
		})
	}
}

func TestChangeReport_Empty(t *testing.T) {
	tests := map[string]struct {
		reason string
		report *ChangeReport
		want   bool
	}{
		"NilReport": {
			reason: "Nil report should be considered empty",
			report: nil,
			want:   true,
		},
		"EmptyVersions": {
			reason: "Report with no versions should be empty",
			report: &ChangeReport{
				Versions: map[string]*VersionChanges{},
			},
			want: true,
		},
		"VersionWithNoChanges": {
			reason: "Version with empty changes list should be empty",
			report: &ChangeReport{
				Versions: map[string]*VersionChanges{
					"v1alpha1": {
						Changes: []SchemaChange{},
					},
				},
			},
			want: true,
		},
		"VersionWithChanges": {
			reason: "Version with changes should not be empty",
			report: &ChangeReport{
				Versions: map[string]*VersionChanges{
					"v1alpha1": {
						Changes: []SchemaChange{
							{Path: "test", ChangeType: ChangeTypeFieldAdded},
						},
					},
				},
			},
			want: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := tt.report.Empty()
			if got != tt.want {
				t.Errorf("\n%s\nEmpty(): got = %v, want %v", tt.reason, got, tt.want)
			}
		})
	}
}

func TestChangeReport_TotalChanges(t *testing.T) {
	tests := map[string]struct {
		reason string
		report *ChangeReport
		want   int
	}{
		"NilReport": {
			reason: "Nil report should have 0 changes",
			report: nil,
			want:   0,
		},
		"EmptyVersions": {
			reason: "Report with no versions should have 0 changes",
			report: &ChangeReport{
				Versions: map[string]*VersionChanges{},
			},
			want: 0,
		},
		"SingleVersionSingleChange": {
			reason: "Single version with one change",
			report: &ChangeReport{
				Versions: map[string]*VersionChanges{
					"v1alpha1": {
						Changes: []SchemaChange{
							{Path: "test1"},
						},
					},
				},
			},
			want: 1,
		},
		"SingleVersionMultipleChanges": {
			reason: "Single version with multiple changes",
			report: &ChangeReport{
				Versions: map[string]*VersionChanges{
					"v1alpha1": {
						Changes: []SchemaChange{
							{Path: "test1"},
							{Path: "test2"},
							{Path: "test3"},
						},
					},
				},
			},
			want: 3,
		},
		"MultipleVersions": {
			reason: "Multiple versions with changes",
			report: &ChangeReport{
				Versions: map[string]*VersionChanges{
					"v1alpha1": {
						Changes: []SchemaChange{
							{Path: "test1"},
							{Path: "test2"},
						},
					},
					"v1beta1": {
						Changes: []SchemaChange{
							{Path: "test3"},
						},
					},
				},
			},
			want: 3,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := tt.report.TotalChanges()
			if got != tt.want {
				t.Errorf("\n%s\nTotalChanges(): got = %v, want %v", tt.reason, got, tt.want)
			}
		})
	}
}

func TestGetChangesAsStructured_KeepAllChanges(t *testing.T) {
	type args struct {
		basePath          string
		revisionModifiers []crdModifier
		keepAllChanges    bool
	}
	type want struct {
		hasChanges bool
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"KeepAllChangesTrue_NonBreaking": {
			reason: "Non-breaking changes should be included when keepAllChanges is true",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						// Add optional field - non-breaking
						addSpecForProviderProperty(r, 0, "optionalField", v1.JSONSchemaProps{
							Type: "string",
						}, nil)
					},
				},
				keepAllChanges: true,
			},
			want: want{
				hasChanges: true,
			},
		},
		"KeepAllChangesFalse_NonBreaking": {
			reason: "Non-breaking changes might be filtered when keepAllChanges is false",
			args: args{
				basePath: "testdata/base.yaml",
				revisionModifiers: []crdModifier{
					func(r *v1.CustomResourceDefinition) {
						// Add optional field - non-breaking
						addSpecForProviderProperty(r, 0, "optionalField", v1.JSONSchemaProps{
							Type: "string",
						}, nil)
					},
				},
				keepAllChanges: false,
			},
			want: want{
				hasChanges: false,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			diff, err := newRevisionDiffWithModifiers(tt.args.basePath, tt.args.basePath, nil, tt.args.revisionModifiers...)
			if err != nil {
				t.Errorf("\n%s\nnewRevisionDiffWithModifiers(...): failed to load CRDs:\n%v", tt.reason, err)
				return
			}

			rawDiff, err := diff.GetRawDiff()
			if err != nil {
				t.Errorf("\n%s\nGetRawDiff(): error = %v", tt.reason, err)
				return
			}

			report, err := GetChangesAsStructured(rawDiff, tt.args.keepAllChanges)
			if err != nil {
				t.Errorf("\n%s\nGetChangesAsStructured(): error = %v", tt.reason, err)
				return
			}

			hasChanges := !report.Empty()
			if hasChanges != tt.want.hasChanges {
				t.Errorf("\n%s\nGetChangesAsStructured(): hasChanges = %v, want %v", tt.reason, hasChanges, tt.want.hasChanges)
			}
		})
	}
}
