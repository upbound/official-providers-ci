package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/upbound/official-providers/testing/common"
	"github.com/upbound/official-providers/testing/pkg"
)

func main() {
	cmd := NewAutoTestCmd()
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func NewAutoTestCmd() *cobra.Command {
	o := &common.AutomatedTestOptions{}
	cmd := &cobra.Command{
		Use:          "Automated Test Tool for Upbound Official Providers",
		Short:        "Automated Test Tool for Upbound Official Providers",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !cmd.Flags().Changed("pr-description") {
				o.Description = os.Getenv("PR_DESCRIPTION")
			}
			if !cmd.Flags().Changed("modified-files") {
				o.ModifiedFiles = os.Getenv("MODIFIED_FILES")
			}
			if !cmd.Flags().Changed("provider") {
				o.ProviderName = os.Getenv("PROVIDER_NAME")
			}
			if !cmd.Flags().Changed("root-dir") {
				o.RootDirectory = os.Getenv("ROOT_DIRECTORY")
			}
			o.WorkingDirectory = fmt.Sprintf("%s/%s", o.RootDirectory, o.ProviderName)

			if err := pkg.RunTest(o); err != nil {
				log.Fatal(err.Error())
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&o.Description, "pr-description", "",
		"Description of Pull Request. Value of this option will be used to trigger/configure the tests."+
			"TEST_INPUT keyword is searched and if found the automated tests is triggered."+
			"The possible usages for TEST_INPUT:\n"+
			"'modified': Modified files in the example directories of providers are used as test inputs.\n"+
			"'provider-aws/examples-generated/s3/bucket.yaml,provider-gcp/examples-generated/storage/bucket.yaml': "+
			"The comma separated resources are used as test inputs.\n"+
			"If this option is not set, 'PR_DESCRIPTION' env var is used as default.")
	cmd.Flags().StringVar(&o.ModifiedFiles, "modified-files", "",
		"Modified Files in the example directories of providers. If the test case is determined as 'modified', "+
			"the value of this option is used as input.\nIf this option is not set, 'MODIFIED_FILES' env var is used as default.")
	cmd.Flags().StringVar(&o.ProviderName, "provider", "", "The provider name to run the tests.\n"+
		"If this option is not set, 'PROVIDER_NAME' env var is used as default.")
	cmd.Flags().StringVar(&o.RootDirectory, "root-dir", "", "Root directory of the official-providers repo\n"+
		"If this option is not set, 'ROOT_DIRECTORY' env var is used as default.")

	return cmd
}
