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
}

// Duration returns the time difference from startTime to endTime.
func (o *QuantifyOptions) Duration() time.Duration {
	return o.endTime.Sub(o.startTime)
}

// NewCmdQuantify creates a cobra command
func NewCmdQuantify() *cobra.Command {
	o := QuantifyOptions{}
	o.cmd = &cobra.Command{
		Use: "provider-scale [flags]",
		Short: "This tool collects CPU & Memory Utilization and time to readiness of MRs metrics of providers and " +
			"reports them. When you execute this tool an end-to-end experiment will run.",
		Example: "provider-scale --mrs ./internal/providerScale/manifests/virtualnetwork.yaml=2 " +
			"--mrs https:... OR ./internal/providerScale/manifests/loadbalancer.yaml=2 " +
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

	results := make(chan []common.Result, 5)
	errChan := make(chan error, 1)
	go func() {
		timeToReadinessResults, err := managed.RunExperiment(o.mrPaths, o.clean, o.applyInterval)
		if err != nil {
			errChan <- errors.Wrap(err, "cannot run experiment")
			return
		}
		errChan <- nil
		results <- timeToReadinessResults
	}()

	var timeToReadinessResults []common.Result
	select {
	case res := <-results:
		if err := <-errChan; err != nil {
			return errors.Wrap(err, "")
		}
		timeToReadinessResults = res
	case <-time.After(o.timeout):
		fmt.Println("Experiment duration exceeded")
	}

	o.endTime = time.Now()
	log.Infof("\nExperiment Ended %v\n\n", o.endTime)
	log.Infof("Results\n------------------------------------------------------------\n")
	log.Infof("Experiment Duration: %f seconds\n", o.Duration().Seconds())

	// Sleeping here allows Prometheus to scrape at least one more
	// sample, which might be used in case there weren't enough samples
	// collected during the experiment. See
	// getInstantVectorQueryResultSampleByHandlingEmptyResults.
	time.Sleep(60 * time.Second)

	err := o.processPods(timeToReadinessResults)
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
		queryResultMemory, err := o.CollectRangeQueryData(fmt.Sprintf(`sum(node_namespace_pod_container:container_memory_working_set_bytes{pod="%s", namespace="%s"})`,
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

		// Following matchers for image, job, and metrics_path are copied
		// from
		// node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate
		// recording rule, definition of which can be found at
		// /api/v1/rules endpoint of Prometheus instance, e.g.,
		// http://localhost:9090/api/v1/rules.
		labelMatcher := `image!="", job="kubelet", metrics_path="/metrics/cadvisor", container=~"provider-.*"`
		avgCPURateQueryFormat := fmt.Sprintf(`100 * sum(rate(container_cpu_usage_seconds_total{%s}%s))`, labelMatcher, "[%.0fs] @ %d")
		avgCPURate, err := o.getInstantVectorQueryResultSampleByHandlingEmptyResults(avgCPURateQueryFormat)
		if err != nil {
			return errors.Wrap(err, "cannot get average cpu rate")
		}

		peakCPURateQueryFormat := fmt.Sprintf(`100 * max_over_time(irate(container_cpu_usage_seconds_total{%s}[5m])%s)`, labelMatcher, "[%.0fs:200ms] @ %d")
		peakCPURate, err := o.getInstantVectorQueryResultSampleByHandlingEmptyResults(peakCPURateQueryFormat)
		if err != nil {
			return errors.Wrap(err, "cannot get peak cpu rate ")
		}

		cpuRateResult := &common.Result{
			Metric:     "CPU",
			MetricUnit: "Rate",
			Average:    float64(avgCPURate),
			Peak:       float64(peakCPURate),
			PodName:    providerPod,
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
	return nil
}

// CollectRangeQueryData sends a range query and collects data by using the prometheus client
func (o *QuantifyOptions) CollectRangeQueryData(query string) (model.Value, error) {
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

// CollectInstantQueryData sends an instant query and collects data by using the prometheus client
func (o *QuantifyOptions) CollectInstantQueryData(query string) (model.Value, error) {
	client, err := common.ConstructPrometheusClient(o.address)
	if err != nil {
		return nil, errors.Wrap(err, "cannot construct prometheus client")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Infof("Executing query: %s", query)
	result, warnings, err := client.Query(ctx, query, time.Now())
	if err != nil {
		return nil, errors.Wrap(err, "cannot construct time range for metrics")
	}
	if len(warnings) > 0 {
		log.Infof("Warnings: %v\n", warnings)
	}

	return result, nil
}

type emptyQueryResultError struct {
	query string
}

func (e *emptyQueryResultError) Error() string {
	var query string
	if e != nil {
		query = e.query
	}

	return fmt.Sprintf(`empty result returned from query: "%s"`, query)
}

// getInstantVectorQueryResultSample extracts the value from a
// queryResult that holds a single value. It returns an error in case
// queryResult is empty.
func getInstantVectorQueryResultSample(queryResult model.Value) (model.SampleValue, error) {
	var zero model.SampleValue

	queryResultVector, ok := queryResult.(model.Vector)
	if !ok {
		return zero, errors.New("type assertion to vector failed")
	}

	if queryResultVector.Len() == 0 {
		return zero, &emptyQueryResultError{}
	}

	return queryResultVector[0].Value, nil
}

// getInstantVectorQueryResultSample sends a query, which is
// expected to return a single value inside an instant vector, and
// extracts resulting value.
func (o *QuantifyOptions) getInstantVectorQueryResultSample(query string) (model.SampleValue, error) {
	var zero model.SampleValue

	queryResult, err := o.CollectInstantQueryData(query)
	if err != nil {
		return zero, errors.Wrap(err, "cannot collect query data")
	}

	sampleValue, err := getInstantVectorQueryResultSample(queryResult)
	if err != nil {
		return zero, errors.Wrap(err, "cannot process query result")
	}

	return sampleValue, nil
}

// getInstantVectorQueryResultSampleByHandlingEmptyResults prepares
// and executes a query, built from queryFormat. queryFormat should
// contain two format verbs: one for a float durationn and one for
// integer timestamp, respectively, values of which are calculated
// from o.
//
// If a query returns empty result, it is likely to be because the
// queried interval doesn't contain enough samples. Query interval is
// expanded, up to a limit, to get a non-empty result. Doing so
// reduces precision of the query, but having an imprecise result is
// better than having no results at all.
func (o *QuantifyOptions) getInstantVectorQueryResultSampleByHandlingEmptyResults(queryFormat string) (model.SampleValue, error) {
	var result model.SampleValue
	var errEmptyQueryResult *emptyQueryResultError

	// Each empty query result causes query duration to be increased by
	// durationIncrementStepSeconds, so that query duration is more
	// likely to include samples.
	durationIncrementStepSeconds := 30
	i := 0
	maxTries := 10
	for ; i < maxTries; i++ {
		// Query interval is expanded from both ends, beginning and end.
		time := o.endTime.Add(time.Duration(i*durationIncrementStepSeconds/2) * time.Second).Unix()
		duration := o.Duration().Seconds() + float64(i*durationIncrementStepSeconds)
		query := fmt.Sprintf(queryFormat, duration, time)

		var err error
		result, err = o.getInstantVectorQueryResultSample(query)
		if err == nil {
			break
		}

		errEmptyQueryResult = nil
		if errors.As(err, &errEmptyQueryResult) {
			errEmptyQueryResult.query = query
			log.Warnf("Got empty query result. Retrying with a wider query interval.")
			continue
		}

		return result, errors.Wrapf(err, `cannot get result for query: "%s"`, query)
	}

	if i == maxTries {
		return result, errEmptyQueryResult
	}
	return result, nil
}
