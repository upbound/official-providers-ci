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

func processYAML(yamlData []byte) ([]byte, error) {
	yamlData = bytes.Replace(yamlData, []byte("${Rand.RFC1123Subdomain}"), []byte("random"), -1)

	decoder := kyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlData), 1024)
	var modifiedYAMLs []byte
	separator := []byte("---\n")
	var first bool = true

	for {
		u := &unstructured.Unstructured{}
		if err := decoder.Decode(&u); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("cannot decode the YAML file: %w", err)
		}

		annotations := u.GetAnnotations()
		if annotations != nil {
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

func processDirectory(inputDir string, outputDir string) error {
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
				err = os.MkdirAll(newDir, 0755)
				if err != nil {
					return fmt.Errorf("cannot create directory %s: %w", newDir, err)
				}
			}
			return nil
		}

		// Only process YAML files
		if filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
			yamlFile, err := os.ReadFile(path)
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
			err = os.WriteFile(outputPath, modifiedYAMLs, 0644)
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
