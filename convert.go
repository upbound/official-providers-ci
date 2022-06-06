package testing

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"
)

const (
	asyncKey   = "operation"
	asyncValue = "async"

	workDir = "/home/runner/work/official-providers/official-providers/"
)

var syncConditions = map[string]interface{}{
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

var asyncConditions = map[string]interface{}{
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

func GenerateTestFiles(filePaths []string, providerPath string) error {
	inputFile, err := createFile("/tmp/automated-tests/case/00-apply.yaml")
	if err != nil {
		return err
	}
	priorSteps, err := getApplyTemplate(providerPath)
	if err != nil {
		return err
	}
	if err := writeToFile(inputFile, priorSteps); err != nil {
		return err
	}

	assertFile, err := createFile("/tmp/automated-tests/case/00-assert.yaml")
	if err != nil {
		return err
	}

	deleteFile, err := createFile("/tmp/automated-tests/case/01-delete.yaml")
	if err != nil {
		return err
	}
	deleteSteps, err := deleteTemplate()
	if err != nil {
		return err
	}
	if err := writeToFile(deleteFile, deleteSteps); err != nil {
		return err
	}

	for _, f := range filePaths {
		m, yamlData, err := readYamlFile(filepath.Join(workDir, f))
		if err != nil {
			return err
		}

		if err := writeToFile(inputFile, yamlData); err != nil {
			return err
		}

		assertData, err := prepareAssertFile(m)
		if err != nil {
			return err
		}
		if err := writeToFile(assertFile, assertData); err != nil {
			return err
		}
	}

	if err := assertFile.Chmod(os.ModePerm); err != nil {
		return err
	}
	if err := inputFile.Chmod(os.ModePerm); err != nil {
		return err
	}
	if err := deleteFile.Chmod(os.ModePerm); err != nil {
		return err
	}

	return nil
}

func getApplyTemplate(providerPath string) ([]byte, error) {
	m := map[string]interface{}{
		"apiVersion": "kuttl.dev/v1beta1",
		"kind":       "TestStep",
		"commands": []map[string]interface{}{
			{"command": "kubectl create ns crossplane-system"},
			{"command": "kubectl create secret generic provider-creds -n crossplane-system --from-file=creds=/tmp/automated-tests/case/creds.conf"},
			{"command": fmt.Sprintf("kubectl apply -f %s/examples/providerconfig.yaml", providerPath)},
		},
	}
	return yaml.Marshal(m)
}

func deleteTemplate() ([]byte, error) {
	m := map[string]interface{}{
		"apiVersion": "kuttl.dev/v1beta1",
		"kind":       "TestStep",
		"commands": []map[string]interface{}{
			{"command": "kubectl delete managed --all"},
		},
	}
	return yaml.Marshal(m)
}

func createFile(filePath string) (*os.File, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(os.ModeAppend); err != nil {
		return nil, err
	}
	return file, nil
}

func readYamlFile(filePath string) (map[string]interface{}, []byte, error) {
	yamlData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, nil, err
	}

	m := make(map[string]interface{})
	if err := yaml.Unmarshal(yamlData, &m); err != nil {
		return nil, nil, err
	}

	return m, yamlData, nil
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

func writeToFile(file *os.File, data []byte) error {
	if _, err := file.Write(data); err != nil {
		return err
	}
	if _, err := file.WriteString("---\n"); err != nil {
		return err
	}
	return nil
}
