package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/upbound/official-providers/testing"
)

func main() {
	cmd := NewConvertCmd()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type ConvertOptions struct {
	cmd      *cobra.Command
	inputs   []string
	provider string
}

func NewConvertCmd() *cobra.Command {
	co := ConvertOptions{}
	co.cmd = &cobra.Command{
		Use:          "",
		Short:        "",
		Example:      "",
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return testing.GenerateTestFiles(co.inputs, co.provider)
		},
	}

	co.cmd.Flags().StringSliceVarP(&co.inputs, "inputs", "i", nil, "Inputs")
	co.cmd.Flags().StringVarP(&co.provider, "provider", "p", "", "Provider")
	return co.cmd
}
