package config

const (
	AnnotationKeyTimeout        = "uptest.upbound.io/timeout"
	AnnotationKeyConditions     = "uptest.upbound.io/conditions"
	AnnotationKeyPreAssertHook  = "uptest.upbound.io/pre-assert-hook"
	AnnotationKeyPostAssertHook = "uptest.upbound.io/post-assert-hook"
)

type AutomatedTest struct {
	ManifestPaths  []string
	DataSourcePath string

	SetupScriptPath    string
	TeardownScriptPath string

	DefaultTimeout    int
	DefaultConditions []string
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
	Manifest  string

	Timeout              int
	Conditions           []string
	PreAssertScriptPath  string
	PostAssertScriptPath string
}
