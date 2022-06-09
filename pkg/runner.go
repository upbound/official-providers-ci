package pkg

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	inputKeyword = `TEST_INPUT=`
	defaultCase  = "default"
)

var prDescription = os.Getenv("PR_BODY")
var modifiedFiles = os.Getenv("MODIFIED_FILES")
var providerName = os.Getenv("PROVIDER_NAME")
var workingDirectory = os.Getenv("WORKING_DIRECTORY")
var rootDirectory = os.Getenv("ROOT_DIR")

func RunTest() error {
	testFiles := getTestFiles()

	if len(testFiles) == 0 {
		log.Warn("The file to test was not found. Skipped...")
		os.Exit(0)
	}

	if err := runCommand("bash", true, "-c", filepath.Join("../.github/scripts/install_kubectl_kuttl.sh")); err != nil {
		return err
	}

	if err := os.MkdirAll("/tmp/automated-tests/case", os.ModePerm); err != nil {
		return err
	}

	log.Info("/tmp/automated-tests/case directory was created!")

	if err := createProviderCredsFile(providerName); err != nil {
		return err
	}

	log.Info("Provider credentials were successfully stored.")

	if err := generateTestFiles(testFiles, providerName); err != nil {
		return err
	}

	log.Info("Test files were generated!")

	if err := runCommand("bash", true, "-c", "cat /tmp/automated-tests/case/00-apply.yaml"); err != nil {
		return err
	}
	if err := runCommand("bash", true, "-c", "cat /tmp/automated-tests/case/00-assert.yaml"); err != nil {
		return err
	}
	if err := runCommand("bash", true, "-c", `"${KIND}" create cluster`); err != nil {
		return err
	}
	if err := runCommand("bash", true, "-c", fmt.Sprintf(`"${KUBECTL}" apply -f %s/package/crds`, workingDirectory)); err != nil {
		return err
	}

	time.Sleep(10 * time.Second)

	if err := os.Chdir(workingDirectory); err != nil {
		return err
	}

	log.Infof("Changed directory to %s", workingDirectory)

	if err := runCommand("bash", false, "-c", "make run"); err != nil {
		return err
	}

	if err := runCommand("bash", true, "-c", "kuttl test --start-kind=false /tmp/automated-tests/ --timeout 1200"); err != nil {
		return err
	}
	return nil
}

func getTestFiles() []string {
	if !strings.Contains(prDescription, inputKeyword) {
		log.Warn("TEST_INPUT keyword not found in Pull Request description. Skipped...")
		os.Exit(0)
	}

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

func runCommand(command string, wait bool, args ...string) error {
	cmd := exec.Command(command, args...)

	cmdOutReader, err := cmd.StdoutPipe()
	if err != nil {
		log.Error("Error creating StdoutPipe for Cmd", err)
		return err
	}

	cmdErrReader, err := cmd.StderrPipe()
	if err != nil {
		log.Error("Error creating StderrPipe for Cmd", err)
		return err
	}

	outScanner := bufio.NewScanner(cmdOutReader)
	go func() {
		for outScanner.Scan() {
			log.Infof("\t > %s\n", outScanner.Text())
		}
	}()

	errScanner := bufio.NewScanner(cmdErrReader)
	go func() {
		for errScanner.Scan() {
			log.Infof("\t > %s\n", errScanner.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	if wait {
		if err := cmd.Wait(); err != nil {
			return err
		}
	}

	return nil
}

func createProviderCredsFile(providerName string) error {
	providerCredsEnv := fmt.Sprintf("%s_CREDS", strings.ToUpper(strings.ReplaceAll(providerName, "-", "_")))
	providerCreds := os.Getenv(providerCredsEnv)
	f, err := createFile("/tmp/automated-tests/case/creds.conf", fs.ModePerm)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(providerCreds); err != nil {
		return err
	}
	return nil
}
