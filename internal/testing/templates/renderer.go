package templates

import (
	"github.com/upbound/provider-tools/internal/testing/config"
	"strings"
	"text/template"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

var fileTemplates = map[string]string{
	"00-apply.yaml":  inputFileTemplate,
	"00-assert.yaml": assertFileTemplate,
	"01-delete.yaml": deleteFileTemplate,
	"01-assert.yaml": assertDeletedFileTemplate,
}

func Render(tc *config.TestCase, resources []config.Resource) (map[string]string, error) {
	data := struct {
		Resources []config.Resource
		TestCase  config.TestCase
	}{
		Resources: resources,
		TestCase:  *tc,
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
