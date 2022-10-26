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
