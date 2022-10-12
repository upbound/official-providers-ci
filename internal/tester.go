package internal

import (
	"bufio"
	"fmt"
	"github.com/upbound/uptest/internal/templates"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strconv"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/upbound/uptest/internal/config"
)

func NewTester(manifests []*unstructured.Unstructured, opts *config.AutomatedTest) *Tester {
	return &Tester{
		options:   opts,
		manifests: manifests,
	}
}

type Tester struct {
	options   *config.AutomatedTest
	manifests []*unstructured.Unstructured
}

func (t *Tester) ExecuteTests() error {
	if err := t.writeKuttlFiles(); err != nil {
		return errors.Wrap(err, "cannot write kuttl test files")
	}
	fmt.Println("Running kuttl tests at " + testDirectory)
	cmd := exec.Command("bash", "-c", fmt.Sprintf(`"${KUTTL}" test --start-kind=false --skip-cluster-delete %s --timeout %d 2>&1`, testDirectory, t.options.DefaultTimeout))
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "cannot start kuttl")
	}
	sc := bufio.NewScanner(stdout)
	sc.Split(bufio.ScanLines)
	for sc.Scan() {
		fmt.Println(sc.Text())
	}
	return errors.Wrap(cmd.Wait(), "kuttl failed")
}

func (t *Tester) prepareConfig() (*config.TestCase, []config.Resource, error) {
	tc := &config.TestCase{
		Timeout: t.options.DefaultTimeout,
	}
	examples := make([]config.Resource, len(t.manifests))

	for i, m := range t.manifests {
		if m.GroupVersionKind().String() == "/v1, Kind=Secret" {
			continue
		}

		kg := strings.ToLower(m.GroupVersionKind().Kind + "." + m.GroupVersionKind().Group)
		d, err := yaml.Marshal(m)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "cannot marshal manifest for \"%s/%s\"", kg, m.GetName())
		}

		example := config.Resource{
			Name:         m.GetName(),
			Namespace:    m.GetNamespace(),
			KindGroup:    kg,
			Manifest:     string(d),
			Timeout:      t.options.DefaultTimeout,
			HooksDirPath: t.options.DefaultHooksDirPath,
			Conditions:   t.options.DefaultConditions,
		}

		if v, ok := m.GetAnnotations()[config.AnnotationKeyTimeout]; ok {
			example.Timeout, err = strconv.Atoi(v)
			if err != nil {
				return nil, nil, errors.Wrap(err, "timeout value is not valid")
			}
			if example.Timeout > tc.Timeout {
				tc.Timeout = example.Timeout
			}
		}

		if v, ok := m.GetAnnotations()[config.AnnotationKeyHooksDirectory]; ok {
			example.HooksDirPath, err = filepath.Abs(v)
			if err != nil {
				return nil, nil, errors.Wrap(err, "cannot find absolute path for hooks directory")
			}
		}

		if v, ok := m.GetAnnotations()[config.AnnotationKeyConditions]; ok {
			example.Conditions = strings.Split(v, ",")
		}

		examples[i] = example
	}

	return tc, examples, nil
}

func (t *Tester) writeKuttlFiles() error {
	tc, examples, err := t.prepareConfig()
	if err != nil {
		return errors.Wrap(err, "cannot build examples config")
	}

	files, err := templates.Render(tc, examples)
	if err != nil {
		return errors.Wrap(err, "cannot render kuttl templates")
	}

	for k, v := range files {
		if err := os.WriteFile(filepath.Join(caseDirectory, k), []byte(v), fs.ModePerm); err != nil {
			return errors.Wrapf(err, "cannot write file %q", k)
		}
	}

	return nil
}
