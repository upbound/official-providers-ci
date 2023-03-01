// Copyright 2023 Upbound Inc.
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

// main package for the ttr tool, which reports the time-to-readiness
// measurements for all managed resources in a cluster.

package main

import (
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type filter struct {
	gvk  schema.GroupVersionKind
	name *regexp.Regexp
}

func (f filter) matchGVK(gvk schema.GroupVersionKind) bool {
	return (len(f.gvk.Group) == 0 || f.gvk.Group == gvk.Group) &&
		(len(f.gvk.Version) == 0 || f.gvk.Version == gvk.Version) &&
		(len(f.gvk.Kind) == 0 || f.gvk.Kind == gvk.Kind)
}

type filters []*filter

func (f filters) match(gvk schema.GroupVersionKind, name string) bool {
	if len(f) == 0 {
		return true
	}
	for _, e := range f {
		if e.matchGVK(gvk) && (len(name) == 0 || e.name == nil || e.name.MatchString(name)) {
			return true
		}
	}
	return false
}

func parseFilter(f string) (*filter, error) {
	tokens := strings.Split(f, "/")
	if len(tokens) != 4 {
		return nil, errors.Errorf("invalid filter string: %s", f)
	}
	var re *regexp.Regexp
	if len(tokens[3]) != 0 {
		r, err := regexp.Compile(tokens[3])
		if err != nil {
			return nil, errors.Wrapf(err, "invalid name regex expression: %s", tokens[3])
		}
		re = r
	}
	return &filter{
		gvk: schema.GroupVersionKind{
			Group:   tokens[0],
			Version: tokens[1],
			Kind:    tokens[2],
		},
		name: re,
	}, nil
}

func getFilters(f ...string) (filters, error) {
	result := make(filters, 0, len(f))
	for _, s := range f {
		f, err := parseFilter(s)
		if err != nil {
			return nil, errors.Wrap(err, "failed to prepare filters")
		}
		result = append(result, f)
	}
	return result, nil
}
