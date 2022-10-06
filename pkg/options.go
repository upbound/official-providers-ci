package pkg

type AutomatedTestOptions struct {
	ExamplePaths        []string
	ProviderName        string
	RootDirectory       string
	ProviderCredentials string
	DataSourcePath      string
	SkipProviderConfig  bool
}
