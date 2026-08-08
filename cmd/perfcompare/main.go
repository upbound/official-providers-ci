// Copyright 2024 The Crossplane Authors
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

// Package main
package main

import (
	"math"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
)

type applicationPerformance struct {
	AverageTimeToReadiness float64 `yaml:"average_time_to_readiness"`
	PeakTimeToReadiness    float64 `yaml:"peak_time_to_readiness"`
	AverageMemory          float64 `yaml:"average_memory"`
	PeakMemory             float64 `yaml:"peak_memory"`
	AverageCPU             float64 `yaml:"average_cpu"`
	PeakCPU                float64 `yaml:"peak_cpu"`
}

// Compare two applicationPerformance structs and print the differences with a threshold check
func (ap applicationPerformance) Compare(newData applicationPerformance, threshold float64) {
	log.Infoln("Comparing Performance:")
	compareField("Average Time to Readiness", ap.AverageTimeToReadiness, newData.AverageTimeToReadiness, threshold)
	compareField("Peak Time to Readiness", ap.PeakTimeToReadiness, newData.PeakTimeToReadiness, threshold)
	compareField("Average Memory", ap.AverageMemory, newData.AverageMemory, threshold)
	compareField("Peak Memory", ap.PeakMemory, newData.PeakMemory, threshold)
	compareField("Average CPU", ap.AverageCPU, newData.AverageCPU, threshold)
	compareField("Peak CPU", ap.PeakCPU, newData.PeakCPU, threshold)
}

func compareField(fieldName string, oldData, newData float64, threshold float64) {
	diff := newData - oldData
	percentChange := diff / oldData * 100
	diff = math.Round(diff*100) / 100
	percentChange = math.Round(percentChange*100) / 100

	// Print the comparison result with 2 decimal places
	log.Infof("%s: Old Data = %.2f, New Data = %.2f, Difference = %.2f (%.2f%% change)\n", fieldName, oldData, newData, diff, percentChange)

	// Check if the percent change exceeds the threshold
	if percentChange > threshold*100 {
		log.Warnf("Attention: %s increased by more than %.0f%% -> %f\n", fieldName, threshold*100, percentChange)
	}
}

// Read YAML file and unmarshal into applicationPerformance struct
func readYAMLFile(filename string) (applicationPerformance, error) {
	data, err := os.ReadFile(filepath.Clean(filename))
	if err != nil {
		return applicationPerformance{}, err
	}

	var performance applicationPerformance
	err = yaml.Unmarshal(data, &performance)
	if err != nil {
		return applicationPerformance{}, err
	}

	return performance, nil
}

func main() {
	// Command-line argument parsing
	app := kingpin.New("perf-compare", "A tool to compare application performance data from YAML files.")
	old := app.Flag("old", "Path to the old results as YAML file.").Short('o').Required().String()
	current := app.Flag("new", "Path to the new results as YAML file.").Short('n').Required().String()
	threshold := app.Flag("threshold", "The threshold for the performance comparison like 0.10 for %10.").Short('t').Required().Float64()

	if _, err := app.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}

	// Read data from the YAML files
	oldData, err := readYAMLFile(*old)
	if err != nil {
		log.Fatalf("Failed to read old data: %v", err)
	}

	newData, err := readYAMLFile(*current)
	if err != nil {
		log.Fatalf("Failed to read new data: %v", err)
	}

	// Compare the two datasets
	oldData.Compare(newData, *threshold)
}
