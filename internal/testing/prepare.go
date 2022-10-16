package testing

import (
	"bytes"
	"fmt"
	"github.com/upbound/provider-tools/internal/testing/config"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sigs.k8s.io/yaml"
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

var (
	testDirectory = filepath.Join(os.TempDir(), "uptest-e2e")
	caseDirectory = filepath.Join(testDirectory, "case")
)

var (
	charset = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	dataSourceRegex = regexp.MustCompile(`\\${data\\.(.*?)}`)
	randomStrRegex  = regexp.MustCompile(`\\${Rand\\.(.*?)}`)
)

func WithDataSource(path string) PreparerOption {
	return func(p *Preparer) {
		p.dataSourcePath = path
	}
}

type PreparerOption func(*Preparer)

func NewPreparer(testFilePaths []string, opts ...PreparerOption) *Preparer {
	p := &Preparer{
		testFilePaths: testFilePaths,
	}
	for _, f := range opts {
		f(p)
	}
	return p
}

type Preparer struct {
	testFilePaths  []string
	dataSourcePath string
}

func (p *Preparer) PrepareManifests() ([]config.Manifest, error) {
	if err := os.MkdirAll(caseDirectory, os.ModePerm); err != nil {
		return nil, errors.Wrapf(err, "cannot create directory %s", caseDirectory)
	}

	injectedFiles, err := p.injectVariables()
	if err != nil {
		return nil, errors.Wrap(err, "cannot inject variables")
	}

	manifests := make([]config.Manifest, 0, len(injectedFiles))
	for path, data := range injectedFiles {
		decoder := kyaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(data), 1024)
		for {
			u := &unstructured.Unstructured{}
			if err := decoder.Decode(&u); err != nil {
				if err == io.EOF {
					break
				}
				return nil, errors.Wrap(err, "cannot decode manifest")
			}
			if u != nil {
				if v, ok := u.GetAnnotations()["upjet.upbound.io/manual-intervention"]; ok {
					fmt.Printf("Skipping %s with name %s since it requires the following manual intervention: %s\n", u.GroupVersionKind().String(), u.GetName(), v)
					continue
				}
				y, err := yaml.Marshal(u)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot marshal manifest for \"%s/%s\"", u.GetObjectKind(), u.GetName())
				}
				manifests = append(manifests, config.Manifest{
					FilePath: path,
					Object:   u,
					YAML:     string(y),
				})
			}
		}
	}
	return manifests, nil
}

func (p *Preparer) injectVariables() (map[string]string, error) {
	dataSourceMap := make(map[string]string)
	if p.dataSourcePath != "" {
		dataSource, err := os.ReadFile(p.dataSourcePath)
		if err != nil {
			return nil, errors.Wrap(err, "cannot read data source file")
		}
		if err := yaml.Unmarshal(dataSource, dataSourceMap); err != nil {
			return nil, errors.Wrap(err, "cannot prepare data source map")
		}
	}

	inputs := make(map[string]string, len(p.testFilePaths))
	for _, f := range p.testFilePaths {
		manifestData, err := os.ReadFile(f)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read %s", f)
		}
		inputData, err := p.injectValues(string(manifestData), dataSourceMap)
		if err != nil {
			return nil, errors.Wrap(err, "cannot inject data source values")
		}
		inputs[f] = inputData
	}
	return inputs, nil
}

func (p *Preparer) injectValues(manifestData string, dataSourceMap map[string]string) (string, error) {
	// Inject data source values such as tenantID, objectID, accountID
	dataSourceKeys := dataSourceRegex.FindAllStringSubmatch(manifestData, -1)
	for _, dataSourceKey := range dataSourceKeys {
		if v, ok := dataSourceMap[dataSourceKey[1]]; ok {
			manifestData = strings.Replace(manifestData, dataSourceKey[0], v, -1)
		}
	}
	// Inject random strings
	randomKeys := randomStrRegex.FindAllStringSubmatch(manifestData, -1)
	for _, randomKey := range randomKeys {
		switch randomKey[1] {
		case "RFC1123Subdomain":
			r := generateRFC1123SubdomainCompatibleString()
			manifestData = strings.Replace(manifestData, randomKey[0], r, 1)
		default:
			continue
		}
	}
	return manifestData, nil
}

func generateRFC1123SubdomainCompatibleString() string {
	rand.Seed(time.Now().UnixNano())
	s := make([]rune, 8)
	for i := range s {
		s[i] = charset[rand.Intn(len(charset))]
	}
	return fmt.Sprintf("op-%s", string(s))
}
