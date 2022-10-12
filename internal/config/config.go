package config

const (
	AnnotationKeyTimeout        = "uptest.upbound.io/timeout"
	AnnotationKeyHooksDirectory = "uptest.upbound.io/hooks-directory"
	AnnotationKeyConditions     = "uptest.upbound.io/conditions"
)

type AutomatedTest struct {
	ManifestPaths       []string
	DataSourcePath      string
	DefaultTimeout      int
	DefaultHooksDirPath string
	DefaultConditions   []string
}

type Resource struct {
	Name      string
	Namespace string
	KindGroup string
	Manifest  string

	Timeout      int
	HooksDirPath string
	Conditions   []string
}

type TestCase struct {
	Timeout int
}
