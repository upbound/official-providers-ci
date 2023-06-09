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

// Package main has generic functions to get the new provider names
// from compositions and managed resources.
package main

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/upbound/upjet/pkg/migration"
)

// SSOPNames is a global map for collecting the new provider names
var SSOPNames = map[string]struct{}{}

// GetSSOPNameFromManagedResource collects the new provider name from MR
func GetSSOPNameFromManagedResource(u migration.UnstructuredWithMetadata) error { //nolint:unparam // Because of the signature of preprocessor interface
	newProviderName := getProviderAndServiceName(u.Object.GroupVersionKind().Group)
	if newProviderName != "" {
		SSOPNames[newProviderName] = struct{}{}
	}
	return nil
}

// GetSSOPNameFromComposition collects the new provider name from Composition
func GetSSOPNameFromComposition(u migration.UnstructuredWithMetadata) error { //nolint:deadcode // It will be used in the main of this tool
	composition, err := migration.ToComposition(u.Object)
	if err != nil {
		return errors.Wrap(err, "unstructured object cannot be converted to composition")
	}
	for _, composedTemplate := range composition.Spec.Resources {
		composedUnstructured, err := migration.FromRawExtension(composedTemplate.Base)
		if err != nil {
			return errors.Wrap(err, "resource raw cannot convert to unstructured")
		}
		newProviderName := getProviderAndServiceName(composedUnstructured.GroupVersionKind().Group)
		if newProviderName != "" {
			SSOPNames[newProviderName] = struct{}{}
		}
	}
	return nil
}

func getProviderAndServiceName(name string) string {
	parts := strings.Split(name, ".")
	if len(parts) > 3 {
		provider := ""
		switch parts[1] {
		case "aws":
			provider = "provider-aws"
		case "gcp":
			provider = "provider-gcp"
		case "azure":
			provider = "provider-azure"
		default:
			return ""
		}
		service := parts[0]
		return fmt.Sprintf("%s-%s", provider, service)
	}
	return ""
}
