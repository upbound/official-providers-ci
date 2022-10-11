package config

type AutomatedTest struct {
	ExamplePaths   []string
	HooksDirectory string
	DataSourcePath string
	Composite      bool
	DefaultTimeout int
}

type Example struct {
	Manifest      string
	Namespace     string
	Timeout       int
	WaitCondition string
}

type TestCase struct {
	Timeout int
}
