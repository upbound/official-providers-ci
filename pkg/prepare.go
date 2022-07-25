package pkg

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

const (
	inputKeyword = "/test-examples" // followed by a space and comma-separated list.

	testDirectory = "/tmp/automated-tests/case"
	credsFile     = "creds.conf"
)

var (
	charset = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

	dataSourceRegex = regexp.MustCompile("\\${data\\.(.*?)}")
	randomStrRegex  = regexp.MustCompile("\\${Rand\\.(.*?)}")
)

type Preparer struct {
	testFilePaths  []string
	dataSourcePath string
}

func (p *Preparer) PrepareManifests(rootDirectory, providerCredentials string) ([]string, error) {
	if err := os.MkdirAll(testDirectory, os.ModePerm); err != nil {
		return nil, errors.Wrapf(err, "cannot create directory %s", testDirectory)
	}
	if err := os.WriteFile(filepath.Join(testDirectory, credsFile), []byte(providerCredentials), fs.ModePerm); err != nil {
		return nil, errors.Wrap(err, "cannot write credentials file")
	}

	return p.prepareInputs(rootDirectory)
}

func (p *Preparer) prepareInputs(rootDirectory string) ([]string, error) {
	var inputs []string
	for _, f := range p.testFilePaths {
		manifestData, err := ioutil.ReadFile(filepath.Join(rootDirectory, f))
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read %s", filepath.Join(rootDirectory, f))
		}
		inputData, err := p.injectValues(string(manifestData))
		if err != nil {
			return nil, errors.Wrap(err, "cannot inject data source values")
		}
		inputs = append(inputs, inputData)
	}
	return inputs, nil
}

func (p *Preparer) injectValues(manifestData string) (string, error) {
	// Inject data source values such as tenantID, objectID, accountID
	dataSourceMap := make(map[string]string)
	dataSource, err := ioutil.ReadFile(p.dataSourcePath)
	if err != nil {
		return "", errors.Wrap(err, "cannot read data source file")
	}
	if err := yaml.Unmarshal(dataSource, dataSourceMap); err != nil {
		return "", errors.Wrap(err, "cannot prepare data source map")
	}
	dataSourceKeys := dataSourceRegex.FindAllStringSubmatch(manifestData, -1)
	for _, dataSourceKey := range dataSourceKeys {
		manifestData = strings.Replace(manifestData, dataSourceKey[0], dataSourceMap[dataSourceKey[1]], -1)
	}

	// Inject random strings
	randomKeys := randomStrRegex.FindAllStringSubmatch(manifestData, -1)
	for _, randomKey := range randomKeys {
		switch randomKey[1] {
		case "RFC1123Subdomain":
			r := generateRFC1123SubdomainCompatibleString()
			manifestData = strings.Replace(manifestData, randomKey[0], r, -1)
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
