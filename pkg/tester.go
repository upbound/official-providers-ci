package pkg

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
commands:
- command: ${KUBECTL} annotate managed --all upjet.upbound.io/test=true --overwrite`

	assertStatementTemplate = "- command: ${KUBECTL} wait %s --for=condition=Test --timeout 10s"

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

func NewTester(manifests []*unstructured.Unstructured) *Tester {
	return &Tester{
		manifests: manifests,
	}
}

type Tester struct {
	manifests []*unstructured.Unstructured
}

func (t *Tester) ExecuteTests(rootDirectory, providerName string) error {
	assertManifest, err := t.generateAssertFiles()
	if err != nil {
		return errors.Wrap(err, "cannot generate assert files")
	}
	if err := t.writeKuttlFiles(assertManifest, filepath.Join(rootDirectory, providerName)); err != nil {
		return errors.Wrap(err, "cannot write kuttl test files")
	}
	cmd := exec.Command("bash", "-c", `"${KUTTL}" test --start-kind=false /tmp/automated-tests/ --timeout 1200`)
	out, err := cmd.CombinedOutput()
	log.Printf("%s\n", out)
	return errors.Wrap(err, "cannot successfully completed automated tests")
}

func (t *Tester) generateAssertFiles() ([]string, error) {
	assertManifest := []string{assertFileBase}
	for _, m := range t.manifests {
		assertManifest = append(assertManifest, fmt.Sprintf(assertStatementTemplate,
			fmt.Sprintf("%s.%s/%s", strings.ToLower(m.GroupVersionKind().Kind),
				strings.ToLower(m.GroupVersionKind().Group), m.GetName())))
	}
	return assertManifest, nil
}

func (t *Tester) writeKuttlFiles(assertManifest []string, workingDirectory string) error {
	priorSteps := fmt.Sprintf(priorStepsTemplate, workingDirectory)
	kuttlInputs := make([]string, len(t.manifests)+1)
	kuttlInputs[0] = priorSteps
	for i, m := range t.manifests {
		d, err := yaml.Marshal(m)
		if err != nil {
			return errors.Wrapf(err, "cannot marshal manifest %s with name %s", m.GroupVersionKind().String(), m.GetName())
		}
		kuttlInputs[i+1] = string(d)
	}

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
