package templates

import (
	"strings"
	"text/template"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	"github.com/upbound/uptest/internal/config"
)

var fileTemplates = map[string]string{
	"00-apply.yaml":  inputFileTemplate,
	"00-assert.yaml": assertFileTemplate,
	"01-delete.yaml": deleteFileTemplate,
	"01-assert.yaml": assertDeletedFileTemplate,
}

type TestCaseRenderer struct {
	options *config.AutomatedTest
}

func NewRenderer(opts *config.AutomatedTest) *TestCaseRenderer {
	return &TestCaseRenderer{
		options: opts,
	}
}

func (r *TestCaseRenderer) Render(tc *config.TestCase, examples map[string]config.Example) (map[string]string, error) {
	data := struct {
		Examples map[string]config.Example
		Options  config.AutomatedTest
		TestCase config.TestCase
	}{
		Examples: examples,
		Options:  *r.options,
		TestCase: *tc,
	}

	res := make(map[string]string, len(fileTemplates))
	for name, tmpl := range fileTemplates {
		t, err := template.New(name).Parse(tmpl)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse template %q", name)
		}

		var b strings.Builder
		if err := t.Execute(&b, data); err != nil {
			return nil, errors.Wrapf(err, "cannot execute template %q", name)
		}
		res[name] = b.String()
	}

	return res, nil
}
