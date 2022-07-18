package pkg

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

const (
	manifestSeparator = "\n---\n"

	priorStepsTemplate = `
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} create namespace upbound-system
- command: ${KUBECTL} create secret generic provider-creds -n upbound-system --from-file=creds=/tmp/automated-tests/case/creds.conf
- command: ${KUBECTL} apply -f %s/examples/providerconfig.yaml`

	assertFileBase = `
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 1200
commands:`

	assertStatementTemplate = "- command: ${KUBECTL} wait %s --for=condition=UpToDate --timeout 10s"

	cleanupSteps = `
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} delete managed --all
`

	inputFileName  = "00-apply.yaml"
	assertFileName = "00-assert.yaml"
	deleteFileName = "01-delete.yaml"
)

var (
	inputFilePath  = filepath.Join(testDirectory, inputFileName)
	assertFilePath = filepath.Join(testDirectory, assertFileName)
	deleteFilePath = filepath.Join(testDirectory, deleteFileName)
)

type Tester struct {
	inputs []string
}

func (t *Tester) ExecuteTests(testFilePaths []string, rootDirectory, providerName string) error {
	assertManifest, err := t.generateAssertFiles(testFilePaths, rootDirectory)
	if err != nil {
		return errors.Wrap(err, "cannot generate assert files")
	}
	if err := t.writeKuttlFiles(assertManifest, filepath.Join(rootDirectory, providerName)); err != nil {
		return errors.Wrap(err, "cannot write kuttl test files")
	}
	cmd := exec.Command("bash", "-c", `"${KUTTL}" test --start-kind=false /tmp/automated-tests/ --timeout 1200`)
	out, err := cmd.CombinedOutput()
	log.Printf("%s\n", out)
	if err != nil {
		return errors.Wrap(err, "cannot successfully completed automated tests")
	}
	return nil
}

func (t *Tester) generateAssertFiles(testFilePaths []string, rootDirectory string) ([]string, error) {
	assertManifest := []string{assertFileBase}
	for _, f := range testFilePaths {
		manifestData, err := ioutil.ReadFile(filepath.Join(rootDirectory, f))
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read %s", filepath.Join(rootDirectory, f))
		}
		decoder := kyaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(string(manifestData)), 1024)
		for {
			u := &unstructured.Unstructured{}
			if err := decoder.Decode(&u); err != nil {
				if err == io.EOF {
					break
				}
				return nil, errors.Wrap(err, "cannot decode manifest")
			}
			if u != nil {
				assertManifest = append(assertManifest, fmt.Sprintf(assertStatementTemplate,
					fmt.Sprintf("%s.%s/%s", strings.ToLower(u.GroupVersionKind().Kind),
						strings.ToLower(u.GroupVersionKind().Group), u.GetName())))
			}
		}
	}
	return assertManifest, nil
}

func (t *Tester) writeKuttlFiles(assertManifest []string, workingDirectory string) error {
	priorSteps := fmt.Sprintf(priorStepsTemplate, workingDirectory)
	kuttlInputs := []string{priorSteps}
	kuttlInputs = append(kuttlInputs, t.inputs...)

	if err := os.WriteFile(inputFilePath, []byte(strings.Join(kuttlInputs, manifestSeparator)), fs.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot write input manifests to %s", inputFilePath)
	}
	if err := os.WriteFile(assertFilePath, []byte(strings.Join(assertManifest, "\n")), fs.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot write assertion manifests to %s", assertFilePath)
	}
	if err := os.WriteFile(deleteFilePath, []byte(cleanupSteps), fs.ModePerm); err != nil {
		return errors.Wrapf(err, "cannot write deletion manifest to %s", deleteFilePath)
	}
	return nil
}
