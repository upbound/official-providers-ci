package config

type AutomatedTest struct {
	ExamplePaths          []string
	DataSourcePath        string
	DefaultHooksDirectory string
	DefaultConditions     []string
	DefaultTimeout        int
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
