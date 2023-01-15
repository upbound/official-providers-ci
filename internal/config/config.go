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

package config

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

const (
	AnnotationKeyTimeout        = "uptest.upbound.io/timeout"
	AnnotationKeyConditions     = "uptest.upbound.io/conditions"
	AnnotationKeyPreAssertHook  = "uptest.upbound.io/pre-assert-hook"
	AnnotationKeyPostAssertHook = "uptest.upbound.io/post-assert-hook"
	AnnotationKeyPreDeleteHook  = "uptest.upbound.io/pre-delete-hook"
	AnnotationKeyPostDeleteHook = "uptest.upbound.io/post-delete-hook"
)

type AutomatedTest struct {
	Directory string

	ManifestPaths  []string
	DataSourcePath string

	SetupScriptPath    string
	TeardownScriptPath string

	DefaultTimeout    int
	DefaultConditions []string
}

type Manifest struct {
	FilePath string
	Object   *unstructured.Unstructured
	YAML     string
}

type TestCase struct {
	Timeout            int
	SetupScriptPath    string
	TeardownScriptPath string
}

type Resource struct {
	Name      string
	Namespace string
	KindGroup string
	YAML      string

	Timeout              int
	Conditions           []string
	PreAssertScriptPath  string
	PostAssertScriptPath string
	PreDeleteScriptPath  string
	PostDeleteScriptPath string
}
