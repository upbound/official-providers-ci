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
	"strings"

	kinoapi "github.com/getkin/kin-openapi/openapi3"
	"github.com/oasdiff/oasdiff/diff"
)

// FlattenDiff converts a nested diff.Diff structure into a flat list of SchemaChanges.
// This makes it easier to process and analyze schema changes programmatically.
func FlattenDiff(d *diff.Diff) []SchemaChange {
	if d == nil || d.Empty() {
		return nil
	}

	// Navigate to the SchemaDiff within the complex diff structure
	sd := extractSchemaDiff(d)
	if sd == nil {
		return nil
	}

	// Recursively walk the schema diff tree and extract all changes
	return walkSchemaDiff("", sd)
}

// extractSchemaDiff navigates the oasdiff structure to find the SchemaDiff
// that contains CRD schema changes. The path is:
// Diff → EndpointsDiff → Modified["/crd"] → RequestBodyDiff → ContentDiff → MediaTypeModified["application/json"] → SchemaDiff
func extractSchemaDiff(d *diff.Diff) *diff.SchemaDiff {
	if d.EndpointsDiff == nil {
		return nil
	}

	methodDiff, ok := d.EndpointsDiff.Modified[crdPutEndpoint]
	if !ok || methodDiff.RequestBodyDiff == nil {
		return nil
	}

	if methodDiff.RequestBodyDiff.ContentDiff == nil {
		return nil
	}

	mediaType, ok := methodDiff.RequestBodyDiff.ContentDiff.MediaTypeModified[contentTypeJSON]
	if !ok {
		return nil
	}

	return mediaType.SchemaDiff
}

// walkSchemaDiff recursively traverses a SchemaDiff tree and extracts all changes
func walkSchemaDiff(path string, sd *diff.SchemaDiff) []SchemaChange {
	if sd == nil {
		return nil
	}

	var changes []SchemaChange //nolint:prealloc // Cannot pre-allocate: size depends on recursive walkSchemaDiff calls

	// Handle schema lifecycle (added/deleted)
	if sd.SchemaAdded {
		changes = append(changes, SchemaChange{
			Path:          path,
			PathParts:     parsePath(path),
			ChangeType:    ChangeTypeFieldAdded,
			RawSchemaDiff: sd,
		})
		return changes // Don't process further if entire schema added
	}

	if sd.SchemaDeleted {
		changes = append(changes, SchemaChange{
			Path:          path,
			PathParts:     parsePath(path),
			ChangeType:    ChangeTypeFieldDeleted,
			RawSchemaDiff: sd,
		})
		return changes // Don't process further if entire schema deleted
	}

	// Extract different types of changes
	changes = append(changes, extractPropertyChanges(path, sd)...)
	changes = append(changes, extractTypeChanges(path, sd)...)
	changes = append(changes, extractItemsChanges(path, sd)...)

	return changes
}

// extractPropertyChanges handles changes to object properties (PropertiesDiff)
func extractPropertyChanges(path string, sd *diff.SchemaDiff) []SchemaChange {
	if sd.PropertiesDiff == nil {
		return nil
	}

	var changes []SchemaChange //nolint:prealloc // Cannot pre-allocate: size depends on recursive walkSchemaDiff calls

	// Added properties
	for _, propName := range sd.PropertiesDiff.Added {
		// Skip properties that appear "added" when they're actually part of
		// an array->object type conversion. The type change itself will be
		// captured by extractTypeChanges, and extractItemsChanges will handle
		// the array item schema deletion.
		if shouldSkipDueToArrayObjectConversion(sd) {
			continue
		}
		propPath := joinPath(path, propName)
		changes = append(changes, SchemaChange{
			Path:          propPath,
			PathParts:     parsePath(propPath),
			ChangeType:    ChangeTypeFieldAdded,
			RawSchemaDiff: sd,
		})
	}

	// Deleted properties
	for _, propName := range sd.PropertiesDiff.Deleted {
		propPath := joinPath(path, propName)
		changes = append(changes, SchemaChange{
			Path:          propPath,
			PathParts:     parsePath(propPath),
			ChangeType:    ChangeTypeFieldDeleted,
			RawSchemaDiff: sd,
		})
	}

	// Modified properties - recurse into nested changes
	for propName, propDiff := range sd.PropertiesDiff.Modified {
		propPath := joinPath(path, propName)
		changes = append(changes, walkSchemaDiff(propPath, propDiff)...)
	}

	return changes
}

// extractTypeChanges handles type and format changes
func extractTypeChanges(path string, sd *diff.SchemaDiff) []SchemaChange {
	// Pre-allocate: maximum 2 changes (type + format)
	changes := make([]SchemaChange, 0, 2)

	// Skip array<->object conversions as they are handled independently
	// (the conversion is a special case that shouldn't be reported as a simple type change)
	if shouldSkipDueToArrayObjectConversion(sd) {
		return changes
	}

	// Type change (e.g., string → integer)
	if sd.TypeDiff != nil && !sd.TypeDiff.Empty() {
		schChange := SchemaChange{
			Path:       path,
			PathParts:  parsePath(path),
			ChangeType: ChangeTypeTypeChanged,
			TypeChangeDetails: &TypeChangeDetails{
				Added:   sd.TypeDiff.Added,
				Deleted: sd.TypeDiff.Deleted,
			},
			RawSchemaDiff: sd,
		}
		// set original types for display purposes
		if sd.Base != nil {
			schChange.TypeChangeDetails.OldType = sd.Base.Type
		}
		if sd.Revision != nil {
			schChange.TypeChangeDetails.NewType = sd.Revision.Type
		}
		changes = append(changes, schChange)
	}

	return changes
}

// Helper functions

// joinPath concatenates path segments with dot notation
func joinPath(base, part string) string {
	if base == "" {
		return part
	}
	return base + "." + part
}

// parsePath splits a dot-notation path into components
func parsePath(path string) []string {
	if path == "" {
		return []string{}
	}
	return strings.Split(path, ".")
}

// shouldSkipDueToArrayObjectConversion checks if we should skip reporting changes
// because they're part of an array->object type conversion.
func shouldSkipDueToArrayObjectConversion(sd *diff.SchemaDiff) bool {
	if sd.ItemsDiff == nil || sd.TypeDiff == nil {
		return false
	}
	if !sd.ItemsDiff.SchemaDeleted {
		return false
	}
	// Check for array->object conversions
	return sd.TypeDiff.Deleted.Is(kinoapi.TypeArray) && sd.TypeDiff.Added.Is(kinoapi.TypeObject)
}

// extractItemsChanges handles changes to array item schemas (ItemsDiff).
// This captures changes like adding/removing fields in array item objects,
// or changing the type of array items.
func extractItemsChanges(path string, sd *diff.SchemaDiff) []SchemaChange {
	if sd.ItemsDiff == nil {
		return nil
	}

	// Skip processing ItemsDiff when the field's type is changing FROM or TO array.
	// This prevents reporting redundant changes:
	// - array → string: Type change is the real change, item schema deletion is side effect
	// - string → array: Type change is the real change, item schema addition is side effect
	// - array → object: Special case handled by shouldSkipDueToArrayObjectConversion
	// We only process ItemsDiff when the type stays as array but the item schema changes
	// (e.g., array items gain/lose fields, or item type changes).
	if sd.TypeDiff != nil && !sd.TypeDiff.Empty() {
		// If type is changing FROM array or TO array, skip ItemsDiff processing
		if sd.TypeDiff.Added.Is(kinoapi.TypeArray) || sd.TypeDiff.Deleted.Is(kinoapi.TypeArray) {
			return nil
		}
	}

	// Recursively process changes to the array item schema.
	// Use [*] notation to indicate this is the schema for array items.
	itemPath := path + "[*]"
	return walkSchemaDiff(itemPath, sd.ItemsDiff)
}
