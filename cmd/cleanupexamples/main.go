// Copyright 2024 Upbound Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// main package for the cleanupexamples tooling, the tool to remove
// uptest-specific code from published examples on the marketplace
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// filterAnnotations removes specific annotations from a Kubernetes
// unstructured object. It looks for annotations with prefixes
// "upjet.upbound.io/" and "uptest.upbound.io/" and removes
// them if they are present.
func filterAnnotations(u *unstructured.Unstructured) {
	annotations := u.GetAnnotations()
	annotationsToRemove := []string{
		"upjet.upbound.io/",
		"uptest.upbound.io/",
	}

	for key := range annotations {
		for _, prefix := range annotationsToRemove {
			if strings.HasPrefix(key, prefix) {
				delete(annotations, key)
				break
			}
		}
	}

	if len(annotations) == 0 {
		u.SetAnnotations(nil)
	} else {
		u.SetAnnotations(annotations)
	}
}

// processYAML processes a YAML file by replacing specific placeholders
// and removing certain annotations from the Kubernetes objects within it.
// It returns the modified YAML content as a byte slice with YAML document
// separators ("---") between each object.
func processYAML(yamlData []byte) ([]byte, error) {
	// TODO(turkenf): Handle replacing UPTEST_DATASOURCE placeholders like ${data.aws_account_id}
	yamlData = bytes.ReplaceAll(yamlData, []byte("${Rand.RFC1123Subdomain}"), []byte("random"))

	// Create a YAML or JSON decoder to read and decode the input YAML data
	decoder := kyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlData), 1024)
	var modifiedYAMLs []byte
	separator := []byte("---\n")
	first := true

	for {
		u := &unstructured.Unstructured{}
		if err := decoder.Decode(&u); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("cannot decode the YAML file: %w", err)
		}

		if u == nil {
			continue
		}

		// Remove specific annotations from the decoded Kubernetes object.
		filterAnnotations(u)

		modifiedYAML, err := yaml.Marshal(u.Object)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal the YAML file: %w", err)
		}

		if !first {
			modifiedYAMLs = append(modifiedYAMLs, separator...)
		}
		modifiedYAMLs = append(modifiedYAMLs, modifiedYAML...)
		first = false
	}

	return modifiedYAMLs, nil
}

// processDirectory walks through a directory structure, processes all YAML
// files  within it by calling `processYAML`, and saves the modified files
// to a specified  output directory while preserving the original
// directory structure.
func processDirectory(inputDir string, outputDir string) error { //nolint:gocyclo // sequential flow easier to follow
	// Walk through the input directory
	err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		// Check if it's a directory, skip processing but ensure the directory exists in outputDir
		if info.IsDir() {
			relativePath, err := filepath.Rel(inputDir, path)
			if err != nil {
				return fmt.Errorf("error finding relative path: %w", err)
			}
			newDir := filepath.Join(outputDir, relativePath)
			if _, err := os.Stat(newDir); os.IsNotExist(err) {
				err = os.MkdirAll(newDir, 0750)
				if err != nil {
					return fmt.Errorf("cannot create directory %s: %w", newDir, err)
				}
			}
			return nil
		}

		// Only process YAML files
		if filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
			yamlFile, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				return fmt.Errorf("cannot read the YAML file %s: %w", path, err)
			}

			modifiedYAMLs, err := processYAML(yamlFile)
			if err != nil {
				return fmt.Errorf("error processing YAML file %s: %w", path, err)
			}

			// Create the output path, preserving directory structure
			relativePath, err := filepath.Rel(inputDir, path)
			if err != nil {
				return fmt.Errorf("error finding relative path: %w", err)
			}
			outputPath := filepath.Join(outputDir, relativePath)
			err = os.WriteFile(outputPath, modifiedYAMLs, 0600)
			if err != nil {
				return fmt.Errorf("cannot write the YAML file %s: %w", outputPath, err)
			}
		}
		return nil
	})
	return err
}

func main() {
	inputDir := kingpin.Arg("inputDir", "Directory containing the input YAML files.").Required().String()
	outputDir := kingpin.Arg("outputDir", "Directory to save the processed YAML files.").Required().String()

	kingpin.Parse()

	err := processDirectory(*inputDir, *outputDir)
	if err != nil {
		fmt.Printf("error processing directory: %v\n", err)
		return
	}

	fmt.Printf("All YAML files processed and saved to: %s\n", *outputDir)
}
