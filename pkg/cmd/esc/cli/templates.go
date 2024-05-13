// Copyright 2023, Pulumi Corporation.

package cli

import (
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates
var templatesFS embed.FS

var (
	envGetTemplate  *template.Template
	envDiffTemplate *template.Template
)

func init() {
	mustLookup := func(t *template.Template, name string) *template.Template {
		tt := t.Lookup(name)
		if tt == nil {
			panic(fmt.Errorf("missing template %v", name))
		}
		return tt
	}

	root := template.New("templates")

	templates := template.Must(root.ParseFS(templatesFS, "templates/*.tmpl"))
	envGetTemplate = mustLookup(templates, "env-get.tmpl")
	envDiffTemplate = mustLookup(templates, "env-diff.tmpl")
}
