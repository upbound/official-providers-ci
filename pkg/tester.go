package pkg

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const (
	manifestSeparator = "\n---\n"

	priorStepsTemplate = `
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} get namespace upbound-system > /dev/null 2>&1 || ${KUBECTL} create namespace upbound-system
- command: ${KUBECTL} create secret generic provider-creds -n upbound-system --from-file=creds=/tmp/automated-tests/case/creds.conf
- command: ${KUBECTL} apply -f %s/examples/providerconfig.yaml`

	assertFileBase = `
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: %s
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

	timeout = 1200
)

func NewTester(manifests []*unstructured.Unstructured) *Tester {
	return &Tester{
		manifests: manifests,
	}
}

type Tester struct {
	manifests []*unstructured.Unstructured
}

func (t *Tester) ExecuteTests(rootDirectory, providerName string, skipProviderConfig bool) error {
	assertManifest, err := t.generateAssertFiles()
	if err != nil {
		return errors.Wrap(err, "cannot generate assert files")
	}
	if err := t.writeKuttlFiles(assertManifest, filepath.Join(rootDirectory, providerName), skipProviderConfig); err != nil {
		return errors.Wrap(err, "cannot write kuttl test files")
	}
	cmd := exec.Command("bash", "-c", fmt.Sprintf(`"${KUTTL}" test --start-kind=false --skip-cluster-delete /tmp/automated-tests/ --timeout %d 2>&1`, timeout))
	stdout, _ := cmd.StdoutPipe()
	err = cmd.Start()
	if err != nil {
		return errors.Wrap(err, "cannot start kuttl")
	}
	sc := bufio.NewScanner(stdout)
	sc.Split(bufio.ScanLines)
	for sc.Scan() {
		fmt.Println(sc.Text())
	}
	return errors.Wrap(cmd.Wait(), "kuttl failed")
}

func (t *Tester) generateAssertFiles() ([]string, error) {
	assertManifest := []string{assertFileBase}
	for _, m := range t.manifests {
		if m.GroupVersionKind().String() == "/v1, Kind=Secret" {
			continue
		}
		assertManifest = append(assertManifest, fmt.Sprintf(assertStatementTemplate,
			fmt.Sprintf("%s.%s/%s", strings.ToLower(m.GroupVersionKind().Kind),
				strings.ToLower(m.GroupVersionKind().Group), m.GetName())))
		if v, ok := m.GetAnnotations()["upjet.upbound.io/timeout"]; ok {
			vint, err := strconv.Atoi(v)
			if err != nil {
				return nil, errors.Wrap(err, "timeout value is not valid")
			}
			if vint > timeout {
				timeout = vint
			}
		}
	}
	assertManifest[0] = fmt.Sprintf(assertManifest[0], strconv.Itoa(timeout))
	return assertManifest, nil
}

func (t *Tester) writeKuttlFiles(assertManifest []string, workingDirectory string, skipProviderConfig bool) error {
	kuttlInputs := make([]string, len(t.manifests)+1)
	if !skipProviderConfig {
		priorSteps := fmt.Sprintf(priorStepsTemplate, workingDirectory)
		kuttlInputs[0] = priorSteps
	}
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
