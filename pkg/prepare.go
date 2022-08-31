package pkg

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

const (
	testDirectory = "/tmp/automated-tests/case"
	credsFile     = "creds.conf"
)

var (
	charset = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	dataSourceRegex = regexp.MustCompile("\\${data\\.(.*?)}")
	randomStrRegex  = regexp.MustCompile("\\${Rand\\.(.*?)}")
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

func (p *Preparer) PrepareManifests(rootDirectory, providerCredentials string) ([]*unstructured.Unstructured, error) {
	if err := os.MkdirAll(testDirectory, os.ModePerm); err != nil {
		return nil, errors.Wrapf(err, "cannot create directory %s", testDirectory)
	}
	if err := os.WriteFile(filepath.Join(testDirectory, credsFile), []byte(providerCredentials), fs.ModePerm); err != nil {
		return nil, errors.Wrap(err, "cannot write credentials file")
	}

	manifestData, err := p.injectVariables(rootDirectory)
	if err != nil {
		return nil, errors.Wrap(err, "cannot inject variables")
	}
	var manifests []*unstructured.Unstructured
	for _, file := range manifestData {
		decoder := kyaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(file), 1024)
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
				manifests = append(manifests, u)
			}
		}
	}
	return manifests, nil
}

func (p *Preparer) injectVariables(rootDirectory string) ([]string, error) {
	dataSourceMap := make(map[string]string)
	if p.dataSourcePath != "" {
		dataSource, err := ioutil.ReadFile(p.dataSourcePath)
		if err != nil {
			return nil, errors.Wrap(err, "cannot read data source file")
		}
		if err := yaml.Unmarshal(dataSource, dataSourceMap); err != nil {
			return nil, errors.Wrap(err, "cannot prepare data source map")
		}
	}

	var inputs []string
	for _, f := range p.testFilePaths {
		manifestData, err := ioutil.ReadFile(filepath.Join(rootDirectory, f))
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read %s", filepath.Join(rootDirectory, f))
		}
		inputData, err := p.injectValues(string(manifestData), dataSourceMap)
		if err != nil {
			return nil, errors.Wrap(err, "cannot inject data source values")
		}
		inputs = append(inputs, inputData)
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
