package template

import (
	"bytes"
	"path/filepath"
	"text/template"

	"github.com/stakater/konfigurator/pkg/reconcileerr"
	"github.com/stakater/konfigurator/pkg/template/funcs"
)

func newTemplate(name string) *template.Template {
	return template.New(name).Funcs(funcs.TemplateFuncs)
}

// ExecuteFile executes a template file located at path with the specified data
func ExecuteFile(path string, data interface{}) ([]byte, error) {
	template, err := newTemplate(filepath.Base(path)).ParseFiles(path)
	if err != nil {
		return nil, err
	}

	return executeTemplate(template, data)
}

// ExecuteString executes a template string with the specified data.
//
// A parse error is permanent: only the author can fix it. An execute error is
// next-cycle: it depends on the data, which is live cluster state (an empty Pods
// list during a rollout makes `index .Pods 0` fail, then succeed a minute later).
func ExecuteString(text string, data interface{}) ([]byte, error) {
	template, err := newTemplate("stdin").Parse(text)
	if err != nil {
		return nil, reconcileerr.Permanent(err)
	}

	out, err := executeTemplate(template, data)
	return out, reconcileerr.NextCycle(err)
}

// Helper for ExecuteFile and ExecuteString - actually executes the template
func executeTemplate(template *template.Template, data interface{}) ([]byte, error) {
	var buffer bytes.Buffer
	if err := template.Execute(&buffer, data); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
