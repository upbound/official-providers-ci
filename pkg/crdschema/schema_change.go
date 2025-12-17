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

import "github.com/tufin/oasdiff/diff"

// ChangeType represents the type of schema change detected
type ChangeType string

const (
	ChangeTypeFieldAdded   ChangeType = "field_added"
	ChangeTypeFieldDeleted ChangeType = "field_deleted"
	ChangeTypeTypeChanged  ChangeType = "type_changed"
)

// SchemaChange represents a single atomic change in a CRD schema.
// It's a flattened representation of changes extracted from oasdiff's
// nested diff structure.
type SchemaChange struct {
	// Path is the JSONPath to the changed field (e.g., "spec.forProvider.instanceName")
	Path string `json:"path"`

	// PathParts is the path split into components for easier processing
	PathParts []string `json:"pathParts"`

	// ChangeType indicates what kind of change occurred
	ChangeType ChangeType `json:"changeType"`

	// OldValue contains the original value (if applicable)
	OldValue interface{} `json:"oldValue,omitempty"`

	// NewValue contains the new value (if applicable)
	NewValue interface{} `json:"newValue,omitempty"`

	// RawSchemaDiff contains the full SchemaDiff object for this change.
	// This is not serialized to JSON but can be used for advanced processing.
	RawSchemaDiff *diff.SchemaDiff `json:"-"`
}

// VersionChanges contains all schema changes for a specific CRD version
type VersionChanges struct {
	// OldVersion Old API version (e.g., "v1alpha1", "v1beta1")
	OldVersion string `json:"oldVersion"`

	// NewVersion New API version (e.g., "v1alpha1", "v1beta1")
	NewVersion string `json:"newVersion"`

	// Changes is the list of all detected changes for this version
	Changes []SchemaChange `json:"changes"`
}

// ChangeReport contains schema changes for all versions in a CRD comparison
type ChangeReport struct {
	// Versions maps version names to their changes
	Versions map[string]*VersionChanges `json:"versions"`
}

// Empty returns true if the report contains no changes
func (r *ChangeReport) Empty() bool {
	if r == nil || len(r.Versions) == 0 {
		return true
	}
	for _, vc := range r.Versions {
		if vc != nil && len(vc.Changes) > 0 {
			return false
		}
	}
	return true
}

// TotalChanges returns the total number of changes across all versions
func (r *ChangeReport) TotalChanges() int {
	if r == nil {
		return 0
	}
	count := 0
	for _, vc := range r.Versions {
		if vc != nil {
			count += len(vc.Changes)
		}
	}
	return count
}
