// Copyright 2022 Upbound Inc.
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

package main

import (
	"log"
	"os"
	"syscall"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/upbound/uptest/crdschema"
)

func main() {
	var (
		app             = kingpin.New("crddiff", "A tool for checking breaking API changes between two CRD OpenAPI v3 schemas").DefaultEnvars()
		baseCRDPath     = app.Arg("base", "The manifest file path of the CRD to be used as the base").Required().ExistingFile()
		revisionCRDPath = app.Arg("revision", "The manifest file path of the CRD to be used as a revision to the base").Required().ExistingFile()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	crdDiff, err := crdschema.NewDiff(*baseCRDPath, *revisionCRDPath)
	kingpin.FatalIfError(err, "Failed to load CRDs")
	d, err := crdDiff.GetBreakingChanges()
	kingpin.FatalIfError(err, "Failed to compute CRD breaking API changes")
	if d.Empty() {
		return
	}
	l := log.New(os.Stderr, "", 0)
	l.Println(crdschema.GetDiffReport(d))
	syscall.Exit(1)
}
