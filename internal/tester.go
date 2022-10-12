package internal

import (
	"bufio"
	"fmt"
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
	"github.com/upbound/uptest/internal/templates"
)

type Renderer interface {
	Render(*config.TestCase, map[string]config.Example) (map[string]string, error)
}

func NewTester(manifests []*unstructured.Unstructured, opts *config.AutomatedTest) *Tester {
	return &Tester{
		options:   opts,
		manifests: manifests,
		renderer:  templates.NewRenderer(opts),
	}
}

type Tester struct {
	options   *config.AutomatedTest
	manifests []*unstructured.Unstructured
	renderer  Renderer
}

func (t *Tester) ExecuteTests() error {
	if err := t.writeKuttlFiles(); err != nil {
		return errors.Wrap(err, "cannot write kuttl test files")
	}
	cmd := exec.Command("bash", "-c", fmt.Sprintf(`"${KUTTL}" test --start-kind=false --skip-cluster-delete /tmp/automated-tests/ --timeout %d 2>&1`, t.options.DefaultTimeout))
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

func (t *Tester) prepareConfig() (*config.TestCase, map[string]config.Example, error) {
	tc := &config.TestCase{
		Timeout: t.options.DefaultTimeout,
	}
	examples := make(map[string]config.Example, len(t.manifests))

	for _, m := range t.manifests {
		if m.GroupVersionKind().String() == "/v1, Kind=Secret" {
			continue
		}

		key := fmt.Sprintf("%s.%s/%s", strings.ToLower(m.GroupVersionKind().Kind),
			strings.ToLower(m.GroupVersionKind().Group), m.GetName())

		d, err := yaml.Marshal(m)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "cannot marshal manifest for %q", key)
		}

		example := config.Example{
			Manifest:      string(d),
			Namespace:     m.GetNamespace(),
			WaitCondition: "Test",
		}

		if v, ok := m.GetAnnotations()["upjet.upbound.io/timeout"]; ok {
			example.Timeout, err = strconv.Atoi(v)
			if err != nil {
				return nil, nil, errors.Wrap(err, "timeout value is not valid")
			}
			if example.Timeout > tc.Timeout {
				tc.Timeout = example.Timeout
			}
		}

		examples[key] = example
	}

	return tc, examples, nil
}

func (t *Tester) writeKuttlFiles() error {
	tc, examples, err := t.prepareConfig()
	if err != nil {
		return errors.Wrap(err, "cannot build examples config")
	}

	files, err := t.renderer.Render(tc, examples)
	if err != nil {
		return errors.Wrap(err, "cannot render kuttl templates")
	}

	for k, v := range files {
		if err := os.WriteFile(filepath.Join(testDirectory, k), []byte(v), fs.ModePerm); err != nil {
			return errors.Wrapf(err, "cannot write file %q", k)
		}
	}

	return nil
}
