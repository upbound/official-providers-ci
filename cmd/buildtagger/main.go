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

// main package for the buildtagger application which is used in the
// official provider repositories to generate build tags for the
// provider families. Each family's source modules can be tagged using
// the buildtagger tool.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("buildtagger", "A tool for generating build tags (constraints) for the source modules of the official provider families.").DefaultEnvars()
)

var (
	parentDir = app.Flag("parent-dir", "Parent directory which will be recursively walked to find the Go source files whose relative path to this parent matches the specified regular expression. The files found will be tagged using the specified build tag.").Default("./").String()
	regex     = app.Flag("regex", `The regular expression against which a discovered Go source file's relative path or name will be matched. This expression must contain one and only one group whose value will be substituted in the given tag format string. An example is "(.+)/.+/.+\.go"`).Default(".*").String()
	tagFormat = app.Flag("tag-format", `A Printf format string to construct the build tag. An example is "(%s || all) && !ignore_autogenerated", where the "%s" format specifier can be replaced by a family resource provider group name.`+
		`There should be a string format specifier for each of the capturing groups specified in the "regex".`).Default("!ignore_autogenerated").String()
	mode       = app.Flag("mode", `If "file", the file name of the discovered Go source is matched against the given regular expression. If "dir", the relative path of the source file is matched.`).Default("dir").Enum("file", "dir")
	deleteTags = app.Flag("delete", `If set, the build tags are removed from the discovered Go sources, instead of being added.`).Default("false").Bool()
)

// addOrUpdateBuildTag traverses directories from the parent,
// updating or adding build tags in Go files. If a build tag already exists,
// it's replaced with the computed tags.
func addOrUpdateBuildTag(parent, regex, tagFormat, mode string, deleteTags bool) error {
	re, err := regexp.Compile(regex)
	kingpin.FatalIfError(err, "Failed to compile the given regular expression: %s", regex)

	return filepath.Walk(parent, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			var matchString string
			switch mode {
			case "file":
				// regex is matched against the filename
				matchString = filepath.Base(path)
			case "dir":
				// regex is matched against the relative path
				matchString, err = filepath.Rel(parent, path)
				if err != nil {
					return errors.Wrapf(err, "failed to determine the relative path of %s wrt to %s", path, parent)
				}
			}

			matches := re.FindStringSubmatch(matchString)
			if len(matches) == 0 {
				return nil
			}
			args := make([]any, len(matches)-1)
			for i, a := range matches[1:] {
				args[i] = a
			}
			tag := fmt.Sprintf("//go:build "+strings.TrimSpace(tagFormat), args...)
			err = updateFileWithBuildTag(path, tag, deleteTags)
			if err != nil {
				return errors.Wrap(err, "failed to update the source file")
			}
		}
		return nil
	})
}

// updateFileWithBuildTag reads a Go file and updates or inserts the specified build tag.
func updateFileWithBuildTag(filePath, buildTag string, deleteTag bool) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return errors.Wrapf(err, "failed to read the source file at path %s", filePath)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) < 1 {
		return nil
	}
	var updatedLines []string
	index := -1
	if strings.HasPrefix(lines[0], "//go:build") {
		index++
	}
	emptyLineFollows := len(lines) > 1 && strings.TrimSpace(lines[1]) == ""
	if deleteTag && emptyLineFollows {
		index++
	}
	updatedLines = lines[index+1:]
	if !deleteTag {
		addedLines := [2]string{buildTag}
		trimIndex := 2
		if emptyLineFollows {
			trimIndex = 1
		}
		updatedLines = append(addedLines[:trimIndex], updatedLines...)
	}
	// Write the updated content back to the file
	return errors.Wrapf(os.WriteFile(filePath, []byte(strings.Join(updatedLines, "\n")), 0644), "failed to write the source file at path %s", filePath)
}

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))
	kingpin.FatalIfError(addOrUpdateBuildTag(*parentDir, *regex, *tagFormat, *mode, *deleteTags), "Failed to run the buildtagger...")
}
