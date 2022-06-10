package pkg

import (
	"bytes"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

const (
	asyncKey   = "operation"
	asyncValue = "async"

	manifestSeparator = "\n---\n"

	priorSteps   = "apiVersion: kuttl.dev/v1beta1\nkind: TestStep\ncommands:\n- command: ${KUBECTL} create secret generic provider-creds -n crossplane-system --from-file=creds=/tmp/automated-tests/case/creds.conf\n- command: ${KUBECTL} apply -f {{.}}/examples/providerconfig.yaml"
	cleanupSteps = "apiVersion: kuttl.dev/v1beta1\nkind: TestStep\ncommands:\n- command: ${KUBECTL} delete managed --all"
)

var (
	syncConditions = map[string]interface{}{
		"conditions": []map[string]interface{}{
			{
				"status": "True",
				"type":   "Ready",
			},
			{
				"status": "True",
				"type":   "Synced",
			},
		},
	}

	asyncConditions = map[string]interface{}{
		"conditions": []map[string]interface{}{
			{
				"status": "True",
				"type":   "Ready",
			},
			{
				"status": "True",
				"type":   "Synced",
			},
			{
				"status": "True",
				"type":   "AsyncOperation",
			},
			{
				"status": "True",
				"type":   "LastAsyncOperation",
			},
		},
	}
)

func generateTestFiles(filePaths []string, workingDirectory, rootDirectory string) error {
	var inputFileData, assertFileData, deletionFileData []string

	t, err := template.New("PriorStepsTemplate").Parse(priorSteps)
	if err != nil {
		return errors.Wrap(err, "cannot create a template object for prior steps")
	}
	var priorStepsBuff bytes.Buffer
	if err = t.Execute(&priorStepsBuff, workingDirectory); err != nil {
		return errors.Wrap(err, "cannot execute template operation for prior steps")
	}

	inputFileData = append(inputFileData, priorStepsBuff.String())
	deletionFileData = append(deletionFileData, cleanupSteps)

	for _, f := range filePaths {
		m, err := readYamlFile(filepath.Join(rootDirectory, "/", f))
		if err != nil {
			return errors.Wrapf(err, "cannot read %s", filepath.Join(rootDirectory, "/", f))
		}

		yamlData, err := yaml.Marshal(m)
		if err != nil {
			return errors.Wrap(err, "cannot marshal data")
		}

		inputFileData = append(inputFileData, string(yamlData))

		assertData, err := prepareAssertFile(m)
		if err != nil {
			return errors.Wrap(err, "cannot prepare manifest of assert file")
		}
		assertFileData = append(assertFileData, string(assertData))
	}

	if err := os.WriteFile(inputFilePath, []byte(strings.Join(inputFileData, manifestSeparator)), fs.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot write input manifests to %s", fmt.Sprintf("%s/%s", testDirectory, inputFileName))
	}
	if err := os.WriteFile(assertFilePath, []byte(strings.Join(assertFileData, manifestSeparator)), fs.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot write assertion manifests to %s", fmt.Sprintf("%s/%s", testDirectory, assertFileName))
	}
	if err := os.WriteFile(deleteFilePath, []byte(strings.Join(deletionFileData, manifestSeparator)), fs.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot write deletion manifest to %s", fmt.Sprintf("%s/%s", testDirectory, assertFileName))
	}

	return nil
}

func readYamlFile(filePath string) (map[string]interface{}, error) {
	yamlData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read %s", filePath)
	}

	m := make(map[string]interface{})
	if err := yaml.Unmarshal(yamlData, &m); err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal data of %s", filePath)
	}

	return m, nil
}

func prepareAssertFile(m map[string]interface{}) ([]byte, error) {
	delete(m, "spec")

	m["status"] = syncConditions
	metadata := m["metadata"].(map[string]interface{})
	if metadata["annotations"] != nil {
		if metadata["annotations"].(map[string]interface{})[asyncKey] == asyncValue {
			m["status"] = asyncConditions
		}
	}
	return yaml.Marshal(m)
}
