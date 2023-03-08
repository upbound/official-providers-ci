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

package managed

import (
	"bufio"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

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
func RunExperiment(mrTemplatePaths map[string]int, clean bool) ([]common.Result, error) { //nolint:gocyclo
	var timeToReadinessResults []common.Result //nolint:prealloc

	client := createDynamicClient()

	if err := applyResources(client, mrTemplatePaths); err != nil {
		return nil, err
	}

	if err := checkReadiness(client, mrTemplatePaths); err != nil {
		return nil, err
	}

	timeToReadinessResults, err := calculateReadinessDuration(client, mrTemplatePaths)
	if err != nil {
		return nil, err
	}

	if clean {
		log.Info("Deleting resources...")
		if err := deleteResources(client, mrTemplatePaths); err != nil {
			return nil, err
		}
		log.Info("Checking deletion of resources...")
		if err := checkDeletion(client, mrTemplatePaths); err != nil {
			return nil, err
		}
	}
	return timeToReadinessResults, nil
}

func applyResources(client dynamic.Interface, mrTemplatePaths map[string]int) error {
	file, err := os.Create(fmt.Sprintf("/tmp/test.yaml"))
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	for mrPath, count := range mrTemplatePaths {
		m, err := readYamlFile(mrPath)
		if err != nil {
			return err
		}
		o := prepareUnstructuredObject(m)

		f, err := os.OpenFile("/tmp/test.yaml", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			return err
		}

		for i := 1; i <= count; i++ {
			o["metadata"].(map[string]interface{})["name"] = fmt.Sprintf("test-%d", i)

			b, err := yaml.Marshal(o)
			if err != nil {
				return err
			}

			if _, err := f.Write(b); err != nil {
				return err
			}

			if _, err := f.WriteString("\n---\n\n"); err != nil {
				return err
			}

			log.Info(fmt.Sprintf("%s/%s was successfully created!\n", m["kind"], o["metadata"].(map[string]interface{})["name"]))
		}

		cmd := exec.Command("bash", "-c", fmt.Sprintf(`"kubectl" apply -f /tmp/test.yaml`)) // #nosec G204
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
			return err
		}
	}
	return nil
}

func deleteResources(client dynamic.Interface, mrTemplatePaths map[string]int) error {
	for mrPath := range mrTemplatePaths {
		m, err := readYamlFile(mrPath)
		if err != nil {
			return err
		}

		background := metav1.DeletePropagationBackground
		if err := client.Resource(prepareGvk(m)).DeleteCollection(context.TODO(),
			metav1.DeleteOptions{PropagationPolicy: &background}, metav1.ListOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func checkReadiness(client dynamic.Interface, mrTemplatePaths map[string]int) error {
	for mrPath := range mrTemplatePaths {
		m, err := readYamlFile(mrPath)
		if err != nil {
			return err
		}

		for {
			log.Info("Checking readiness of resources...")
			list, err := client.Resource(prepareGvk(m)).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				return err
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
	var results []common.Result //nolint:prealloc
	for mrPath := range mrTemplatePaths {
		log.Info("Calculating readiness time of resources...")
		var result common.Result

		m, err := readYamlFile(mrPath)
		if err != nil {
			return nil, err
		}

		list, err := client.Resource(prepareGvk(m)).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
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
				}

				diff := readinessTime.Sub(creationTimestamp.Time)
				result.Data = append(result.Data, common.Data{Value: diff.Seconds()})
				break
			}
		}
		result.Metric = fmt.Sprintf("Time to Readiness of %s", m["kind"])
		result.MetricUnit = "seconds"
		result.Average, result.Peak = common.CalculateAverageAndPeak(result.Data)
		results = append(results, result)
	}
	return results, nil
}

func checkDeletion(client dynamic.Interface, mrTemplatePaths map[string]int) error {
	for mrPath := range mrTemplatePaths {
		m, err := readYamlFile(mrPath)
		if err != nil {
			return err
		}

		for {
			log.Info("Checking deletion of resources...")
			list, err := client.Resource(prepareGvk(m)).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				return err
			}
			if len(list.Items) == 0 {
				break
			}
			time.Sleep(10 * time.Second)
		}
	}
	return nil
}

func prepareGvk(m map[interface{}]interface{}) schema.GroupVersionResource {
	suffix := "s"
	apiVersion := strings.Split(m["apiVersion"].(string), "/")
	kind := strings.ToLower(m["kind"].(string))

	if kind[len(kind)-1] == 'y' {
		kind = kind[:len(kind)-1]
		suffix = "ies"
	}
	return schema.GroupVersionResource{
		Group:    apiVersion[0],
		Version:  apiVersion[1],
		Resource: fmt.Sprintf("%s%s", kind, suffix),
	}
}

func prepareUnstructuredObject(m map[interface{}]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v := range m {
		t, ok := v.(map[interface{}]interface{})
		if ok {
			result[k.(string)] = prepareUnstructuredObject(t)
		} else {
			result[k.(string)] = v
		}
	}
	return result
}

func readYamlFile(fileName string) (map[interface{}]interface{}, error) {
	yamlFile, err := ioutil.ReadFile(fileName) //nolint:gosec
	if err != nil {
		return nil, err
	}

	m := make(map[interface{}]interface{})
	err = yaml.Unmarshal(yamlFile, m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func createDynamicClient() dynamic.Interface {
	return dynamic.NewForConfigOrDie(ctrl.GetConfigOrDie())
}
