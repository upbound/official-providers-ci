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

package common

import (
	"fmt"
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/prometheus/common/model"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// Data represents a collected data
type Data struct {
	Timestamp time.Time
	Value     float64
}

// Result represents all collected data for a metric
type Result struct {
	Data          []Data
	Metric        string
	MetricUnit    string
	Peak, Average float64
}

// ConstructPrometheusClient creates a Prometheus API Client
func ConstructPrometheusClient(address string) v1.API {
	client, err := api.NewClient(api.Config{
		Address: address,
	})

	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	return v1.NewAPI(client)
}

// ConstructTimeRange creates a Range object that consists the start time, end time and step duration
func ConstructTimeRange(startTime, endTime time.Time, stepDuration time.Duration) v1.Range {
	return v1.Range{
		Start: startTime,
		End:   endTime,
		Step:  stepDuration,
	}
}

// ConstructResult creates a Result object from collected data
func ConstructResult(value model.Value, metric, unit string) (*Result, error) {
	result := &Result{}
	matrix := value.(model.Matrix)

	for _, m := range matrix {
		for _, v := range m.Values {
			valueNum, err := strconv.ParseFloat(v.Value.String(), 64)
			if err != nil {
				return nil, err
			}
			result.Data = append(result.Data, Data{Timestamp: v.Timestamp.Time(), Value: valueNum})
		}
	}

	result.Average, result.Peak = CalculateAverageAndPeak(result.Data)
	result.Metric = metric
	result.MetricUnit = unit
	return result, nil
}

// CalculateAverageAndPeak calculates the average and peak values of related metric
func CalculateAverageAndPeak(data []Data) (float64, float64) {
	var sum, peak float64
	for _, d := range data {
		sum += d.Value

		if d.Value > peak {
			peak = d.Value
		}
	}
	return sum / float64(len(data)), peak
}

func (r Result) String() {
	log.Info(fmt.Sprintf("Average %s: %f %s \n", r.Metric, r.Average, r.MetricUnit))
	log.Info(fmt.Sprintf("Peak %s: %f %s \n", r.Metric, r.Peak, r.MetricUnit))
}
