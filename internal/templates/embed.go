package templates

import _ "embed" // nolint:golint

// inputFileTemplate is the template for the input file.
//
//go:embed 00-apply.yaml.tmpl
var inputFileTemplate string

// assertFileTemplate is the template for the assert file.
//
//go:embed 00-assert.yaml.tmpl
var assertFileTemplate string

// deleteFileTemplate is the template for the delete file.
//
//go:embed 01-delete.yaml.tmpl
var deleteFileTemplate string

// assertDeletedFileTemplate is the template for delete assert file.
//
//go:embed 01-assert.yaml.tmpl
var assertDeletedFileTemplate string
