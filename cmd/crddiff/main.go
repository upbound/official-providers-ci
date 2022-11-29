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

	"github.com/upbound/uptest/internal/crdschema"
)

func main() {
	var (
		app = kingpin.New("crddiff", "A tool for checking breaking API changes between two CRD OpenAPI v3 schemas. The schemas can come from either two revisions of a CRD, or from the versions declared in a single CRD.").DefaultEnvars()

		cmdSelf = app.Command("self", "Use OpenAPI v3 schemas from a single CRD")
		crdPath = cmdSelf.Arg("crd", "The manifest file path of the CRD whose versions are to be checked for breaking changes").Required().ExistingFile()

		cmdRevision     = app.Command("revision", "Compare the first schema available in a base CRD against the first schema from a revision CRD")
		baseCRDPath     = cmdRevision.Arg("base", "The manifest file path of the CRD to be used as the base").Required().ExistingFile()
		revisionCRDPath = cmdRevision.Arg("revision", "The manifest file path of the CRD to be used as a revision to the base").Required().ExistingFile()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	var crdDiff crdschema.SchemaCheck
	var err error
	if baseCRDPath != nil && revisionCRDPath != nil {
		crdDiff, err = crdschema.NewRevisionDiff(*baseCRDPath, *revisionCRDPath)
	} else {
		crdDiff, err = crdschema.NewSelfDiff(*crdPath)
	}
	kingpin.FatalIfError(err, "Failed to load CRDs")
	versionMap, err := crdDiff.GetBreakingChanges()
	kingpin.FatalIfError(err, "Failed to compute CRD breaking API changes")

	l := log.New(os.Stderr, "", 0)
	breakingDetected := false
	for v, d := range versionMap {
		if d.Empty() {
			continue
		}
		breakingDetected = true
		l.Printf("Version %q:\n", v)
		l.Println(crdschema.GetDiffReport(d))
	}
	if breakingDetected {
		syscall.Exit(1)
	}
}
