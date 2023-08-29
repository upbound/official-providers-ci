// Copyright 2022 The Crossplane Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package managed contains functions about managed resource management during the experiment
package managed

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"

	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"gopkg.in/yaml.v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/dynamic"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/upbound/uptest/cmd/perf/internal/common"
)

// RunExperiment runs the experiment according to command-line inputs.
// Firstly the input manifests are deployed. After the all MRs are ready, time to readiness metrics are calculated.
// Then, by default, all deployed MRs are deleted.
func RunExperiment(mrTemplatePaths map[string]int, clean bool) ([]common.Result, error) {
	var timeToReadinessResults []common.Result

	client := createDynamicClient()

	tmpFileName, err := applyResources(mrTemplatePaths)
	if err != nil {
		return nil, errors.Wrap(err, "cannot apply resources")
	}

	if err := checkReadiness(client, mrTemplatePaths); err != nil {
		return nil, errors.Wrap(err, "cannot check readiness of resources")
	}

	timeToReadinessResults, err = calculateReadinessDuration(client, mrTemplatePaths)
	if err != nil {
		return nil, errors.Wrap(err, "cannot calculate time to readiness")
	}

	if clean {
		log.Info("Deleting resources...")
		if err := deleteResources(tmpFileName); err != nil {
			return nil, errors.Wrap(err, "cannot delete resources")
		}
	}
	return timeToReadinessResults, nil
}

func applyResources(mrTemplatePaths map[string]int) (string, error) {
	f, err := os.CreateTemp("/tmp", "")
	if err != nil {
		return "", errors.Wrap(err, "cannot create input file")
	}

	for mrPath, count := range mrTemplatePaths {
		m, err := readYamlFile(mrPath)
		if err != nil {
			return "", errors.Wrap(err, "cannot read template file")
		}

		for i := 1; i <= count; i++ {
			m["metadata"].(map[interface{}]interface{})["name"] = fmt.Sprintf("testperfrun%d", i)

			b, err := yaml.Marshal(m)
			if err != nil {
				return "", errors.Wrap(err, "cannot marshal object")
			}

			if _, err := f.Write(b); err != nil {
				return "", errors.Wrap(err, "cannot write manifest")
			}

			if _, err := f.WriteString("\n---\n\n"); err != nil {
				return "", errors.Wrap(err, "cannot write yaml separator")
			}
		}

		if err := f.Close(); err != nil {
			return "", errors.Wrap(err, "cannot close input file")
		}

		if err := runCommand(fmt.Sprintf(`"kubectl" apply -f %s`, f.Name())); err != nil {
			return "", errors.Wrap(err, "cannot execute kubectl apply command")
		}
	}
	return f.Name(), nil
}

func deleteResources(tmpFileName string) error {
	if err := runCommand(fmt.Sprintf(`"kubectl" delete -f %s`, tmpFileName)); err != nil {
		return errors.Wrap(err, "cannot execute kubectl delete command")
	}
	return nil
}

func checkReadiness(client dynamic.Interface, mrTemplatePaths map[string]int) error {
	for mrPath := range mrTemplatePaths {
		m, err := readYamlFile(mrPath)
		if err != nil {
			return errors.Wrap(err, "cannot read template file")
		}

		for {
			log.Info("Checking readiness of resources...")
			list, err := client.Resource(prepareGVR(m)).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				return errors.Wrap(err, "cannot list resources")
			}
			if isReady(list) {
				break
			}
			time.Sleep(10 * time.Second)
		}
	}
	return nil
}

func isReady(list *unstructured.UnstructuredList) bool {
	for _, l := range list.Items {
		if l.Object["status"] == nil {
			return false
		}
		conditions := l.Object["status"].(map[string]interface{})["conditions"].([]interface{})

		status := ""
		for _, condition := range conditions {
			c := condition.(map[string]interface{})
			if c["type"] == "Ready" {
				status = c["status"].(string)
			}
		}

		if status == "False" || status == "" {
			return false
		}
	}
	return true
}

func calculateReadinessDuration(client dynamic.Interface, mrTemplatePaths map[string]int) ([]common.Result, error) {
	var results []common.Result //nolint:prealloc // The size of the slice is not previously known.
	for mrPath := range mrTemplatePaths {
		log.Info("Calculating readiness time of resources...")
		var result common.Result

		m, err := readYamlFile(mrPath)
		if err != nil {
			return nil, errors.Wrap(err, "cannot read template file")
		}

		list, err := client.Resource(prepareGVR(m)).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "cannot list resources")
		}
		for _, l := range list.Items {
			readinessTime := metav1.Time{}
			creationTimestamp := l.GetCreationTimestamp()
			conditions := l.Object["status"].(map[string]interface{})["conditions"].([]interface{})

			for _, condition := range conditions {
				c := condition.(map[string]interface{})

				if c["type"] == "Ready" && c["status"] == "True" {
					t, err := time.Parse(time.RFC3339, c["lastTransitionTime"].(string))
					if err != nil {
						return nil, err
					}
					readinessTime.Time = t

					diff := readinessTime.Sub(creationTimestamp.Time)
					result.Data = append(result.Data, common.Data{Value: diff.Seconds()})
					break
				}
			}
		}
		result.Metric = fmt.Sprintf("Time to Readiness of %s", m["kind"])
		result.MetricUnit = "seconds"
		result.Average, result.Peak = common.CalculateAverageAndPeak(result.Data)
		results = append(results, result)
	}
	return results, nil
}

func prepareGVR(m map[interface{}]interface{}) schema.GroupVersionResource {
	apiVersion := strings.Split(m["apiVersion"].(string), "/")
	kind := strings.ToLower(m["kind"].(string))
	pluralGVR, _ := meta.UnsafeGuessKindToResource(
		schema.GroupVersionKind{
			Group:   apiVersion[0],
			Version: apiVersion[1],
			Kind:    kind,
		},
	)
	return pluralGVR
}

func readYamlFile(pathOrURL string) (map[interface{}]interface{}, error) {
	var content []byte
	var err error

	if strings.HasPrefix(pathOrURL, "http://") || strings.HasPrefix(pathOrURL, "https://") {
		// Download the file if it's an HTTP/HTTPS URL
		resp, err := http.Get(pathOrURL) //nolint:gosec,noctx // This is not a security-sensitive URL.
		if err != nil {
			return nil, errors.Wrap(err, "cannot fetch URL")
		}
		defer func() {
			err := resp.Body.Close()
			if err != nil {
				log.Fatal("cannot close response body")
			}
		}()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to download; HTTP code: %d", resp.StatusCode)
		}

		content, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "cannot read content from URL")
		}
	} else {
		// If it's a local path, read the file directly
		content, err = os.ReadFile(filepath.Clean(pathOrURL))
		if err != nil {
			return nil, errors.Wrap(err, "cannot read file")
		}
	}

	m := make(map[interface{}]interface{})
	err = yaml.Unmarshal(content, &m)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal map")
	}

	return m, nil
}

func createDynamicClient() dynamic.Interface {
	return dynamic.NewForConfigOrDie(ctrl.GetConfigOrDie())
}

func runCommand(command string) error {
	cmd := exec.Command("bash", "-c", command) // #nosec G204
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "cannot start kubectl")
	}
	sc := bufio.NewScanner(stdout)
	sc.Split(bufio.ScanLines)
	for sc.Scan() {
		fmt.Println(sc.Text())
	}
	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "cannot wait for the command exit")
	}
	return nil
}
