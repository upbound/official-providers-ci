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

// Package internal contains methods about quantifying the provider metrics
package internal

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/upbound/uptest/cmd/perf/internal/common"
	"github.com/upbound/uptest/cmd/perf/internal/managed"

	"github.com/prometheus/common/model"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

// QuantifyOptions represents the options of quantify command
type QuantifyOptions struct {
	providerPods      []string
	providerNamespace string
	mrPaths           map[string]int
	cmd               *cobra.Command
	address           string
	startTime         time.Time
	endTime           time.Time
	stepDuration      time.Duration
	clean             bool
	nodeIP            string
	applyInterval     time.Duration
	timeout           time.Duration
	yamlOutput        bool
}

// NewCmdQuantify creates a cobra command
func NewCmdQuantify() *cobra.Command {
	o := QuantifyOptions{}
	o.cmd = &cobra.Command{
		Use: "provider-scale [flags]",
		Short: "This tool collects CPU & Memory Utilization and time to readiness of MRs metrics of providers and " +
			"reports them. When you execute this tool an end-to-end experiment will run.",
		Example: "provider-scale --mrs ./internal/providerScale/manifests/virtualnetwork.yaml=2 " +
			"--mrs https:... OR ./internal/providerScale/manifests/loadbalancer.yaml=2" +
			"--provider-pods crossplane-provider-jet-azure " +
			"--provider-namespace crossplane-system",
		RunE: o.Run,
	}

	o.cmd.Flags().StringSliceVarP(&o.providerPods, "provider-pods", "p", []string{}, "Names of the provider pods. Multiple names can be specified, separated by commas (spaces are ignored).")
	o.cmd.Flags().StringVar(&o.providerNamespace, "provider-namespace", "crossplane-system",
		"Namespace name of provider")
	o.cmd.Flags().StringToIntVar(&o.mrPaths, "mrs", nil, "Managed resource templates that will be deployed, provided as local paths or URLs")
	o.cmd.Flags().StringVar(&o.address, "address", "http://localhost:9090", "Address of Prometheus service")
	o.cmd.Flags().DurationVar(&o.stepDuration, "step-duration", 1*time.Second, "Step duration between two data points")
	o.cmd.Flags().BoolVar(&o.clean, "clean", true, "Delete deployed MRs")
	o.cmd.Flags().StringVar(&o.nodeIP, "node", "", "Node IP")
	o.cmd.Flags().DurationVar(&o.applyInterval, "apply-interval", 0*time.Second, "Elapsed time between applying two manifests to the cluster. Example = 10s. This means that examples will be applied every 10 seconds.")
	o.cmd.Flags().DurationVar(&o.timeout, "timeout", 120*time.Minute, "Timeout for the experiment")
	o.cmd.Flags().BoolVar(&o.yamlOutput, "yaml-output", false, "This option is for exporting experiment results to a yaml file")

	if err := o.cmd.MarkFlagRequired("provider-pods"); err != nil {
		panic(err)
	}
	if err := o.cmd.MarkFlagRequired("mrs"); err != nil {
		panic(err)
	}

	return o.cmd
}

// Run executes the quantify command's tasks.
func (o *QuantifyOptions) Run(_ *cobra.Command, _ []string) error {
	o.startTime = time.Now()
	log.Infof("Experiment Started %v\n\n", o.startTime)

	timeToReadinessResults, err := managed.RunExperiment(o.mrPaths, o.clean, o.applyInterval)
	if err != nil {
		return errors.Wrap(err, "failed to run experiment")
	}

	o.endTime = time.Now()
	log.Infof("\nExperiment Ended %v\n\n", o.endTime)
	log.Infof("Results\n------------------------------------------------------------\n")
	log.Infof("Experiment Duration: %f seconds\n", o.endTime.Sub(o.startTime).Seconds())
	time.Sleep(60 * time.Second)

	err = o.processPods(timeToReadinessResults)
	if err != nil {
		return errors.Wrap(err, "cannot process pods")
	}
	return nil
}

// processPods calculated metrics for provider pods
func (o *QuantifyOptions) processPods(timeToReadinessResults []common.Result) error {
	// Initialize aggregated results
	var aggregatedMemoryResult = &common.Result{Metric: "Memory", MetricUnit: "Bytes"}
	var aggregatedCPURateResult = &common.Result{Metric: "CPU", MetricUnit: "Rate"}

	for _, providerPod := range o.providerPods {
		providerPod = strings.TrimSpace(providerPod)
		queryResultMemory, err := o.CollectData(fmt.Sprintf(`sum(node_namespace_pod_container:container_memory_working_set_bytes{pod="%s", namespace="%s"})`,
			providerPod, o.providerNamespace))
		if err != nil {
			return errors.Wrap(err, "cannot collect memory data")
		}
		memoryResult, err := common.ConstructResult(queryResultMemory, "Memory", "Bytes", providerPod)
		if err != nil {
			return errors.Wrap(err, "cannot construct memory results")
		}
		// Update aggregated memory result
		aggregatedMemoryResult.Average += memoryResult.Average
		if memoryResult.Peak > aggregatedMemoryResult.Peak {
			aggregatedMemoryResult.Peak = memoryResult.Peak
		}

		queryResultCPURate, err := o.CollectData(fmt.Sprintf(`instance:node_cpu_utilisation:rate5m{instance="%s"} * 100`, o.nodeIP))
		if err != nil {
			return errors.Wrap(err, "cannot collect cpu data")
		}
		cpuRateResult, err := common.ConstructResult(queryResultCPURate, "CPU", "Rate", providerPod)
		if err != nil {
			return errors.Wrap(err, "cannot construct cpu results")
		}
		// Update aggregated CPU rate result
		aggregatedCPURateResult.Average += cpuRateResult.Average
		if cpuRateResult.Peak > aggregatedCPURateResult.Peak {
			aggregatedCPURateResult.Peak = cpuRateResult.Peak
		}

		for _, timeToReadinessResult := range timeToReadinessResults {
			timeToReadinessResult.Print()
		}
		memoryResult.Print()
		cpuRateResult.Print()
	}

	if len(o.providerPods) > 1 {
		log.Infof("\nAggregated Results\n------------------------------------------------------------\n")
		aggregatedMemoryResult.Print()
		aggregatedCPURateResult.Print()
	}

	if o.yamlOutput {
		for _, timeToReadinessResult := range timeToReadinessResults {
			b := strings.Builder{}
			timeToReadinessResult.PrintYaml(&b)
			aggregatedMemoryResult.PrintYaml(&b)
			aggregatedCPURateResult.PrintYaml(&b)

			f, err := os.Create("results.yaml")
			if err != nil {
				return errors.Wrap(err, "cannot create results.yaml")
			}

			if _, err = f.WriteString(b.String()); err != nil {
				return errors.Wrap(err, "cannot write results.yaml")
			}
		}
	}

	return nil
}

// CollectData sends query and collect data by using the prometheus client
func (o *QuantifyOptions) CollectData(query string) (model.Value, error) {
	client, err := common.ConstructPrometheusClient(o.address)
	if err != nil {
		return nil, errors.Wrap(err, "cannot construct prometheus client")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r := common.ConstructTimeRange(o.startTime, o.endTime.Add(60*time.Second), o.stepDuration)
	result, warnings, err := client.QueryRange(ctx, query, r)
	if err != nil {
		return nil, errors.Wrap(err, "cannot construct time range for metrics")
	}
	if len(warnings) > 0 {
		log.Infof("Warnings: %v\n", warnings)
	}
	return result, nil
}
