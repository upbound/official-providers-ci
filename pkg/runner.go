package pkg

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/upbound/official-providers/testing/common"
)

const (
	inputKeyword = "/test-examples" // followed by a space and comma-separated list.

	defaultCase = "modified"

	testDirectory  = "/tmp/automated-tests/case"
	inputFileName  = "00-apply.yaml"
	assertFileName = "00-assert.yaml"
	deleteFileName = "01-delete.yaml"
	credsFile      = "creds.conf"
)

var inputFilePath = fmt.Sprintf("%s/%s", testDirectory, inputFileName)
var assertFilePath = fmt.Sprintf("%s/%s", testDirectory, assertFileName)
var deleteFilePath = fmt.Sprintf("%s/%s", testDirectory, deleteFileName)

func RunTest(o *common.AutomatedTestOptions) error {
	testFiles := getTestFiles(o.Description, o.ModifiedFiles, o.ProviderName)
	if len(testFiles) == 0 {
		log.Warnf("The file to test for %s was not found. Skipped...", o.ProviderName)
		return nil
	}

	if err := os.MkdirAll(testDirectory, os.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot create directory %s", testDirectory)
	}
	log.Infof("%s directory was created!", testDirectory)

	if err := createProviderCredsFile(o.ProviderName); err != nil {
		return errors.Wrapf(err, "cannot write %s credentials file", o.ProviderName)
	}
	log.Info("Provider credentials were successfully written.")

	if err := generateTestFiles(testFiles, o.WorkingDirectory, o.RootDirectory); err != nil {
		return errors.Wrap(err, "cannot generate test files")
	}
	log.Info("Test files were generated!")

	if err := runCommand("bash", "-c", fmt.Sprintf("cat %s/%s", testDirectory, inputFileName)); err != nil {
		return errors.Wrapf(err, "cannot print %s", inputFileName)
	}
	if err := runCommand("bash", "-c", fmt.Sprintf("cat %s/%s", testDirectory, assertFileName)); err != nil {
		return errors.Wrapf(err, "cannot print %s", assertFileName)
	}
	if err := runCommand("bash", "-c", fmt.Sprintf("cat %s/%s", testDirectory, deleteFileName)); err != nil {
		return errors.Wrapf(err, "cannot print %s", deleteFileName)
	}

	if err := runCommand("bash", "-c", `"${KUTTL}" test --start-kind=false /tmp/automated-tests/ --timeout 1200`); err != nil {
		return errors.Wrap(err, "cannot successfully completed automated tests")
	}
	log.Info("Automated Tests successfully completed!")
	return nil
}

func getTestFiles(prDescription, modifiedFiles, providerName string) []string {
	testInput := strings.Split(strings.Split(prDescription, inputKeyword)[1], `"`)[1]
	if testInput == defaultCase {
		return strings.Split(modifiedFiles, ",")
	}

	customInputList := strings.Split(testInput, ",")
	var filteredCustomInputList []string
	for _, customInput := range customInputList {
		if strings.Contains(customInput, providerName) {
			filteredCustomInputList = append(filteredCustomInputList, customInput)
		}
	}

	return filteredCustomInputList
}

func runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	out, err := cmd.CombinedOutput()
	log.Info(string(out))
	if err != nil {
		return errors.Wrap(err, "an error occurred while running command")
	}

	return nil
}

func createProviderCredsFile(providerName string) error {
	providerCredsEnv := fmt.Sprintf("%s_CREDS", strings.ToUpper(strings.ReplaceAll(providerName, "-", "_")))
	providerCreds := os.Getenv(providerCredsEnv)
	// creds.conf file contains the provider credentials. This file is used for
	// generating the credential secrets of provider.
	// Example aws creds.conf file:
	// > [default]
	// > aws_access_key_id = ***
	// > aws_secret_access_key = ***
	return os.WriteFile(fmt.Sprintf("%s/%s", testDirectory, credsFile), []byte(providerCreds), fs.ModePerm)
}
